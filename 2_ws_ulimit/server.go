package main

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync/atomic"
	"syscall"
	"time"
)

var count int64

func ws(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}

	n := atomic.AddInt64(&count, 1)
	if n%100 == 0 {
		log.Printf("Total number of connections: %v", n)
	}
	defer func() {
		n := atomic.AddInt64(&count, -1)
		if n%100 == 0 {
			log.Printf("Total number of connections: %v", n)
		}
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			return
		}

		receivedTime := time.Now()
		log.Printf("msg: %s received at: %s", string(msg), receivedTime.Format(time.RFC3339Nano))

		if err := conn.WriteMessage(websocket.TextMessage, []byte(receivedTime.Format(time.RFC3339Nano))); err != nil {
			log.Printf("Write error: %v", err)
			return
		}
	}
}

func main() {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	log.Printf("NOFILE limit: %v", rLimit.Max)
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Fatalf("Pprof failed: %v", err)
		}
	}()

	http.HandleFunc("/", ws)
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatal(err)
	}
}
