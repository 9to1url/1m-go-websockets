package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"io"
	"log"
	"net/url"
	"os"
	"sync"
	"time"
)

const MYSELF string = "1002"

var (
	ip          = flag.String("ip", "127.0.0.1", "server IP")
	connections = flag.Int("conn", 1, "number of websocket connections")
)

type IncomingMessage struct {
	Caller  string `json:"caller"`
	Callee  string `json:"callee"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

// please add a func to signalSdp
func signalSdp(conn *websocket.Conn, sdp webrtc.SessionDescription) error {
	var incomingMsg IncomingMessage
	incomingMsg.Type = "sdp"
	incomingMsg.Caller = "1002"
	incomingMsg.Callee = "1001"
	incomingMsg.Message = sdp.SDP
	msg, err := json.Marshal(incomingMsg)
	if err != nil {
		log.Printf("Error parsing message: %v", err)
		panic(err)
	}
	log.Printf("client msg: %s", msg)

	return conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

func signalCandidate(conn *websocket.Conn, c *webrtc.ICECandidate) error {
	if c == nil {
		return nil
	}
	var incomingMsg IncomingMessage
	incomingMsg.Type = "candidate"
	incomingMsg.Caller = "1002"
	incomingMsg.Callee = "1001"
	incomingMsg.Message = c.ToJSON().Candidate
	msg, err := json.Marshal(incomingMsg)

	if err != nil {
		return err
	}
	log.Printf("client msg: %s", msg)

	return conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

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

	wsConn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println("Failed to connect", err)
		os.Exit(1)
	}
	defer func() {
		wsConn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		time.Sleep(time.Second)
		wsConn.Close()
	}()

	var incomingMsg IncomingMessage
	incomingMsg.Type = "register"
	incomingMsg.Caller = "1002"
	incomingMsg.Callee = "1002"
	incomingMsg.Message = "login"
	msg, err := json.Marshal(incomingMsg)
	if err != nil {
		log.Printf("Error parsing message: %v", err)
		panic(err)
	}
	log.Printf("client msg: %s", msg)
	if err := wsConn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		log.Printf("Failed to send message: %v", err)
		os.Exit(1)
	}

	var candidatesMux sync.Mutex
	pendingCandidates := make([]*webrtc.ICECandidate, 0)

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := peerConnection.Close(); err != nil {
			fmt.Printf("cannot close peerConnection: %v\n", err)
		}
	}()

	// When an ICE candidate is available send to the other Pion instance
	// the other Pion instance will add this candidate by calling AddICECandidate
	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		candidatesMux.Lock()
		defer candidatesMux.Unlock()

		desc := peerConnection.RemoteDescription()
		if desc == nil {
			pendingCandidates = append(pendingCandidates, c)
		} else if onICECandidateErr := signalCandidate(wsConn, c); onICECandidateErr != nil {
			panic(onICECandidateErr)
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}

		if s == webrtc.PeerConnectionStateClosed {
			// PeerConnection was explicitly closed. This usually happens from a DTLS CloseNotify
			fmt.Println("Peer Connection has gone to closed exiting")
			os.Exit(0)
		}
	})

	// Register data channel creation handling
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

		// Register channel opening handling
		d.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", d.Label(), d.ID())

			for range time.NewTicker(5 * time.Second).C {
				message := RandSeq(15)
				fmt.Printf("Sending '%s'\n", message)

				// Send the message as text
				sendTextErr := d.SendText(message)
				if sendTextErr != nil {
					panic(sendTextErr)
				}
			}
		})

		// Register text message handling
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
		})
	})

	for {

		_, response, err := wsConn.ReadMessage()
		if err != nil {
			log.Printf("Failed to read message: %v", err)
			continue
		}

		log.Printf("server msg: %s", string(response))

		var incomingMsg IncomingMessage
		err = json.Unmarshal(response, &incomingMsg)
		if err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}
		// Check if the message type is "register"
		if incomingMsg.Type == "candidate" {

			candidate := webrtc.ICECandidateInit{}
			if err := json.Unmarshal([]byte(incomingMsg.Message), &candidate); err != nil {
				panic(err)
			}

			if candidateErr := peerConnection.AddICECandidate(candidate); candidateErr != nil {
				panic(candidateErr)
			}

		} else if incomingMsg.Type == "sdp" {
			sdp := webrtc.SessionDescription{}
			sdp.Type = webrtc.SDPTypeOffer
			sdp.SDP = incomingMsg.Message

			if err := peerConnection.SetRemoteDescription(sdp); err != nil {
				panic(err)
			}

			// Create an answer to send to the other process
			answer, err := peerConnection.CreateAnswer(nil)
			if err != nil {
				panic(err)
			}

			// Send our answer to the HTTP server listening in the other process
			// send back the sdp
			signalSdp(wsConn, answer)

			// Sets the LocalDescription, and starts our UDP listeners
			err = peerConnection.SetLocalDescription(answer)
			if err != nil {
				panic(err)
			}

			candidatesMux.Lock()
			for _, c := range pendingCandidates {
				onICECandidateErr := signalCandidate(wsConn, c)
				if onICECandidateErr != nil {
					panic(onICECandidateErr)
				}
			}
			candidatesMux.Unlock()
		}
	}
}
