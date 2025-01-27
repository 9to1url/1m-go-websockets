package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"syscall"
)

var count int64

type IncomingMessage struct {
	Caller  string `json:"caller"`
	Callee  string `json:"callee"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

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

	var myself string = "unknown"

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			return
		}

		log.Printf("msg: %s ", string(msg))
		// deal with msg , the msg is a json string, like this:{
		//    "to": "1001",
		//    "message": "login",
		//    "type": "register"
		//}
		// and then parse the msg, get the to and message and type.
		// if type in "register" then call Dispatcher Register function
		var incomingMsg IncomingMessage
		err = json.Unmarshal(msg, &incomingMsg)
		if err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}
		// Check if the message type is "register"
		if incomingMsg.Type == "register" {
			// Create a new channel for the recipient
			ch := make(chan Message)

			myself = incomingMsg.Caller

			// Get the dispatcher instance
			dispatcher := GetDispatcher()

			// Register the new channel with the dispatcher
			dispatcher.Register(incomingMsg.Caller, ch)

			// Optionally, start a worker goroutine for the new recipient
			// add the websocket connection to worker and listen the ch, if received the ch message, then use websocket send the message to client
			go worker(myself, ch, conn)

			log.Printf("Registered %s with the dispatcher", incomingMsg.Caller)
		} else if incomingMsg.Type == "sdp" {

			if myself == "unknown" {
				log.Printf("myself is unknown")
				// exit websocket connection
				return
			}

			// Create a new message with the recipient and content
			msg := Message{
				Sender:    myself,
				Recipient: incomingMsg.Callee,
				Content:   incomingMsg.Message,
				Typ:       "sdp",
			}

			// Get the dispatcher instance
			dispatcher := GetDispatcher()

			// Send the message to the recipient
			dispatcher.Send(msg)
		} else if incomingMsg.Type == "candidate" {
			if myself == "unknown" {
				log.Printf("myself is unknown")
				// exit websocket connection
				return
			}

			// Create a new message with the recipient and content
			msg := Message{
				Sender:    myself,
				Recipient: incomingMsg.Callee,
				Content:   incomingMsg.Message,
				Typ:       "candidate",
			}

			// Get the dispatcher instance
			dispatcher := GetDispatcher()

			// Send the message to the recipient
			dispatcher.Send(msg)
		}

	}
}

func main() {
	previousLimit := SetMemoryLimit(11 * 1024 * 1024 * 1024) // 11 GB
	println("Previous memory limit:", previousLimit)

	// Increase resources limitations
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	log.Printf("NOFILE limit: %v", rLimit.Max)
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	// Enable pprof hooks
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

// SetMemoryLimit sets a limit on the maximum memory usage of the Go program.
// The limit is specified in bytes. Returns the previous limit.
func SetMemoryLimit(limit int64) int64 {
	// Estimate the current limit based on total memory and the current GC target percentage.
	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)
	previousLimit := int64(memStats.HeapAlloc) * 100 / int64(debug.SetGCPercent(-1))

	// Calculate the new GC target percentage based on the desired memory limit.
	// If the limit is zero, reset to default GC behavior.
	if limit > 0 {
		newGCTarget := int(int64(100*memStats.HeapAlloc) / limit)
		debug.SetGCPercent(newGCTarget)
	} else {
		debug.SetGCPercent(100) // Reset to default
	}

	// Return the previous limit for reference.
	return previousLimit
}
