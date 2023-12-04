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

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println("Failed to connect", err)
		os.Exit(1)
	}
	defer func() {
		c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		time.Sleep(time.Second)
		c.Close()
	}()

	msg := `{
    "caller": "1002",
    "callee": "1002",
    "message": "login",
    "type": "register"
}`
	log.Printf("client msg: %s", msg)
	if err := c.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		log.Printf("Failed to send message: %v", err)
		os.Exit(1)
	}

	for {

		_, response, err := c.ReadMessage()
		if err != nil {
			log.Printf("Failed to read message: %v", err)
			continue
		}

		log.Printf("server msg: %s", string(response))
	}
}
