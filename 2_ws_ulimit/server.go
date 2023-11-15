package main

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"runtime/debug"
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
		_ = msg
		//log.Printf("msg: %s received at: %s", string(msg), receivedTime.Format(time.RFC3339Nano))

		if err := conn.WriteMessage(websocket.TextMessage, []byte(receivedTime.Format(time.RFC3339Nano))); err != nil {
			log.Printf("Write error: %v", err)
			return
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
