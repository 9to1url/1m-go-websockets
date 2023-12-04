package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"sync"
)

type Message struct {
	Recipient string
	Content   string
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

func worker(name string, ch chan Message, conn *websocket.Conn) {
	for msg := range ch {
		fmt.Printf("%s received message: %s\n", name, msg.Content)

		// wait on channel for response, if received the ch message, then use websocket send the message to client

		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Content)); err != nil {
			log.Printf("Write error: %v", err)
			return
		}
	}
	fmt.Printf("%s exiting\n", name)
}
