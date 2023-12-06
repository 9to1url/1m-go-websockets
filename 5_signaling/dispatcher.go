package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"sync"
)

type Message struct {
	Sender    string
	Recipient string
	Content   string
	Typ       string
}

type Dispatcher struct {
	routes map[string]chan Message
	mu     sync.Mutex
}

var (
	dispatcher *Dispatcher
	once       sync.Once
)

func GetDispatcher() *Dispatcher {
	once.Do(func() {
		dispatcher = &Dispatcher{
			routes: make(map[string]chan Message),
		}
	})
	return dispatcher
}

func (d *Dispatcher) Register(name string, ch chan Message) {
	d.mu.Lock()
	d.routes[name] = ch
	d.mu.Unlock()
}

func (d *Dispatcher) Send(msg Message) {
	d.mu.Lock()
	if ch, ok := d.routes[msg.Recipient]; ok {
		d.mu.Unlock()
		ch <- msg
	} else {
		d.mu.Unlock()
		fmt.Printf("No recipient found for %s\n", msg.Recipient)
	}
}

func worker(myself string, ch chan Message, conn *websocket.Conn) {
	for msg := range ch {
		//fmt.Printf("%s received message: %s\n", name, msg.Content)

		// wait on channel for response, if received the ch message, then use websocket send the message to client
		// create a new server.go 's IncomingMessage struct, and then marshal the struct to json string, and then send the json string to client
		var incomingMsg IncomingMessage
		incomingMsg.Type = msg.Typ
		incomingMsg.Caller = msg.Sender
		incomingMsg.Callee = msg.Recipient
		incomingMsg.Message = msg.Content
		jsonMsg, err := json.Marshal(incomingMsg)
		if err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, []byte(jsonMsg)); err != nil {
			log.Printf("Write error: %v", err)
			return
		}
	}
}
