package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
)

func main() {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	switch role := os.Args[1]; role {
	case "send":
		runSender(pc)
	case "receive":
		runReceiver(pc)
	}
}

func runSender(pc *webrtc.PeerConnection) {
	dc, err := pc.CreateDataChannel("pingpong-channel", &webrtc.DataChannelInit{
		Ordered: new(true),
	})
	if err != nil {
		panic(err)
	}

	dc.OnOpen(func() {
		fmt.Printf("Data Channel %s is open\n", dc.Label())
		err := dc.SendText("ping")
		if err != nil {
			log.Println("Send error:", err)
		}
	})

	doneChan := make(chan struct{})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString && strings.ToLower(string(msg.Data)) == "pong" {
			fmt.Println("Received pong!")
			close(doneChan)
		}
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	err = pc.SetLocalDescription(offer)
	if err != nil {
		panic(err)
	}

	<-webrtc.GatheringCompletePromise(pc)

	fmt.Printf("Sender token: %s\n", encodeSDP(*pc.LocalDescription()))

	fmt.Printf("\nEnter receiver token: ")
	receiverToken := readLine()
	fmt.Println()
	answer := decodeSDP(receiverToken)

	err = pc.SetRemoteDescription(answer)
	if err != nil {
		panic(err)
	}

	<-doneChan
}

func runReceiver(pc *webrtc.PeerConnection) {
	var once sync.Once
	doneChan := make(chan struct{})

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		fmt.Println("Received Data Channel", dc.Label())
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			if msg.IsString && strings.ToLower(string(msg.Data)) == "ping" {
				fmt.Println("Received ping, sending pong...")
				dc.SendText("pong")

				go func() {
					for dc.BufferedAmount() > 0 {
						time.Sleep(1 * time.Millisecond)
					}
					once.Do(func() { close(doneChan) })
				}()
			}
		})
	})

	pc.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		if pcs == webrtc.PeerConnectionStateClosed {
			once.Do(func() { close(doneChan) })
		}
	})

	fmt.Printf("Enter sender token: ")
	senderToken := readLine()
	fmt.Println()
	offer := decodeSDP(senderToken)

	if err := pc.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	if err = pc.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	<-webrtc.GatheringCompletePromise(pc)

	fmt.Printf("Receiver token: %s\n\n", encodeSDP(*pc.LocalDescription()))

	<-doneChan
}

func encodeSDP(desc webrtc.SessionDescription) string {
	b, err := json.Marshal(desc)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(b); err != nil {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func decodeSDP(str string) webrtc.SessionDescription {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(str))
	if err != nil {
		panic(err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	defer gz.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, gz); err != nil {
		panic(err)
	}

	var desc webrtc.SessionDescription
	err = json.Unmarshal(buf.Bytes(), &desc)
	if err != nil {
		panic(err)
	}

	return desc
}

func readLine() string {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(line)
}
