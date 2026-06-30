package main

import (
	"bufio"
	"fmt"
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
		// runReceiver(pc)
		receiver, err := NewReceiver()
		if err != nil {
			panic(err)
		}
		defer receiver.Close()

		// TODO: this should be done automatically in initSession
		fmt.Printf("\nEnter sender token: ")
		senderToken := readLine()
		if err := receiver.pc.SetRemoteDescription(decodeSDP(senderToken)); err != nil {
			panic(err)
		}

		tr := <-receiver.TransferRequest()
		fmt.Printf("%#v\n", tr)
	}
}

// func runSender(pc *webrtc.PeerConnection) {
// 	dc, err := pc.CreateDataChannel("pingpong-channel", &webrtc.DataChannelInit{
// 		Ordered: new(true),
// 	})
// 	if err != nil {
// 		panic(err)
// 	}

// 	dc.OnOpen(func() {
// 		fmt.Printf("Data Channel %s is open\n", dc.Label())
// 		err := dc.SendText("ping")
// 		if err != nil {
// 			log.Println("Send error:", err)
// 		}
// 	})

// 	doneChan := make(chan struct{})
// 	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
// 		if msg.IsString && strings.ToLower(string(msg.Data)) == "pong" {
// 			fmt.Println("Received pong!")
// 			close(doneChan)
// 		}
// 	})

// 	offer, err := pc.CreateOffer(nil)
// 	if err != nil {
// 		panic(err)
// 	}

// 	err = pc.SetLocalDescription(offer)
// 	if err != nil {
// 		panic(err)
// 	}

// 	<-webrtc.GatheringCompletePromise(pc)

// 	fmt.Printf("Sender token: %s\n", encodeSDP(*pc.LocalDescription()))

// 	fmt.Printf("\nEnter receiver token: ")
// 	receiverToken := readLine()
// 	fmt.Println()
// 	answer := decodeSDP(receiverToken)

// 	err = pc.SetRemoteDescription(answer)
// 	if err != nil {
// 		panic(err)
// 	}

// 	<-doneChan
// }

func runSender(pc *webrtc.PeerConnection) {
	var once sync.Once
	doneChan := make(chan struct{})

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		fmt.Println("Received Data Channel", dc.Label())
		tr := TransferRequest{
			FileName: "Hello.txt",
			Size:     67,
		}
		bytes, err := tr.Marshal()
		if err != nil {
			panic(err)
		}

		go func() {
			fmt.Println("sender sending transfer request...")
			time.Sleep(1 * time.Second)
			dc.Send(bytes)
		}()

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Println("sender receives data")
			// if msg.IsString && strings.ToLower(string(msg.Data)) == "ping" {
			// 	fmt.Println("Received ping, sending pong...")
			// 	dc.SendText("pong")

			// 	go func() {
			// 		for dc.BufferedAmount() > 0 {
			// 			time.Sleep(1 * time.Millisecond)
			// 		}
			// 		once.Do(func() { close(doneChan) })
			// 	}()
			// }
		})
	})

	pc.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		if pcs == webrtc.PeerConnectionStateClosed {
			once.Do(func() { close(doneChan) })
		}
	})

	fmt.Printf("Enter receiver token: ")
	receiverToken := readLine()
	fmt.Println()
	offer := decodeSDP(receiverToken)

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

	fmt.Printf("Sender token: %s\n\n", encodeSDP(*pc.LocalDescription()))

	<-doneChan
}

func readLine() string {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(line)
}
