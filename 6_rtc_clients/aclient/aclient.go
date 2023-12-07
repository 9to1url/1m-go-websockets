package main

import (
	"log"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	host := os.Getenv("WEBSOCKET_HOST")
	if host == "" {
		log.Fatal("Environment variable WEBSOCKET_HOST not set")
	}
	u := url.URL{Scheme: "ws", Host: host, Path: "/"}

	for {
		conn, err := establishConnection(u)
		if err != nil {
			log.Printf("Error establishing connection: %v", err)
			time.Sleep(5 * time.Second) // Wait before retrying
			continue
		}

		handleMessages(conn)

		// Close the connection gracefully
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("write close:", err)
		}
		time.Sleep(time.Second) // Wait for the close message to be sent
	}
}

func establishConnection(u url.URL) (*websocket.Conn, error) {
	log.Printf("Connecting to %s", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	conn.SetPongHandler(func(appData string) error {
		log.Printf("Pong received: %s", appData)
		return nil
	})

	go func() {
		pingTicker := time.NewTicker(5 * time.Second)
		defer pingTicker.Stop()

		for {
			select {
			case <-pingTicker.C:
				if err := conn.WriteControl(websocket.PingMessage, []byte("ping from client, are you echo?"), time.Now().Add(time.Second)); err != nil {
					log.Printf("Failed to send ping: %v", err)
					conn.Close()
					return
				}
			}
		}
	}()

	return conn, nil
}

func handleMessages(conn *websocket.Conn) {
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("Error reading message: %v", err)
			}
			return
		}

		if messageType == websocket.CloseMessage {
			log.Println("Close message received")
			return
		}

		log.Printf("Received: %s", message)
	}
}
