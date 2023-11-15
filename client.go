package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/url"
	"os"
	"time"
)

var (
	ip          = flag.String("ip", "127.0.0.1", "server IP")
	connections = flag.Int("conn", 1, "number of websocket connections")
)

func main() {
	flag.Usage = func() {
		io.WriteString(os.Stderr, `Websockets client generator
Example usage: ./client -ip=172.17.0.1 -conn=10
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	u := url.URL{Scheme: "ws", Host: *ip + ":8000", Path: "/"}
	log.Printf("Connecting to %s", u.String())

	startTime := time.Now()
	var conns []*websocket.Conn
	for i := 0; i < *connections; i++ {
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			fmt.Println("Failed to connect", i, err)
			break
		}
		conns = append(conns, c)
		defer func() {
			c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
			time.Sleep(time.Second)
			c.Close()
		}()
	}

	finishTimeNeeded := time.Since(startTime)
	log.Printf("Setup %v connections time needed: %v", *connections, finishTimeNeeded)

	log.Printf("Finished initializing %d connections", len(conns))
	tts := time.Second
	if *connections > 100 {
		tts = time.Millisecond * 5
	}
	for {
		for _, conn := range conns {
			time.Sleep(tts)
			sendTime := time.Now()
			msg := fmt.Sprintf("Hello from client, sent at %s", sendTime.Format(time.RFC3339Nano))
			log.Printf("client msg: %s", msg)
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				log.Printf("Failed to send message: %v", err)
				continue
			}

			_, response, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Failed to read message: %v", err)
				continue
			}

			//serverTime, _ := time.Parse(time.RFC3339Nano, string(response))
			log.Printf("server msg: %s", string(response))
			latency := time.Since(sendTime)
			log.Printf("Round-trip latency: %v", latency)
		}
	}
}
