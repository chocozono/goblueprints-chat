package main

import (
	"log"
	"net/http"

	"github.com/chocozono/trace"
	"github.com/gorilla/websocket"
)

type room struct {
	// forward represents the channel which has some messages for transporting to another clients
	foward chan []byte

	// join represents the channel for the client who wanna join to chat room
	join chan *client

	// leave represents the channel for the client who wanna leave from chat room
	leave chan *client

	// clients holds *client
	// treating this map exepts using channel is to be deprecated because
	// there is a possibility that some goroutines change at the same time
	clients map[*client]bool

	//tracer receves operation log from chat room
	tracer trace.Tracer
}

//new room generates and returns the chat room
func newRoom() *room {
	return &room{
		foward:  make(chan []byte),
		join:    make(chan *client),
		leave:   make(chan *client),
		clients: make(map[*client]bool),
		//default trace sets as Off
		tracer: trace.Off(),
	}
}

func (room *room) run() {
	for {
		select {
		case client := <-room.join:
			// join
			room.clients[client] = true
			room.tracer.Trace("Joined new client")
		case client := <-room.leave:
			// leave
			delete(room.clients, client)
			close(client.send)
			room.tracer.Trace("Leaved the client")
		case msg := <-room.foward:
			room.tracer.Trace("Recived message: ", string(msg))
			// send messages to all clients
			for client := range room.clients {
				select {
				case client.send <- msg:
					// send a message
					room.tracer.Trace(" -- Success -- Send message")
				default:
					// if failed sending message
					delete(room.clients, client)
					close(client.send)
					room.tracer.Trace(" -- Fail -- Send message ")
				}
			}
		}
	}
}

const (
	socketBufferSize  = 1024
	messageBufferSize = 256
)

var upgrader = &websocket.Upgrader{
	ReadBufferSize:  socketBufferSize,
	WriteBufferSize: socketBufferSize,
}

func (room *room) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal("ServeHTTP: ", err)
		return
	}
	client := &client{
		socket: socket,
		send:   make(chan []byte, messageBufferSize),
		room:   room,
	}
	room.join <- client
	defer func() { room.leave <- client }()
	go client.write()
	client.read()
}
