package main

import (
	"fmt"
	"log"

	"github.com/pion/webrtc/v4"
)

type Sender struct {
	SessionID SessionID
	pc        *webrtc.PeerConnection
	dc        *webrtc.DataChannel
}

func NewSender(sessionID SessionID) (*Sender, error) {
	pc, err := webrtc.NewPeerConnection(WebRTCConfig())
	if err != nil {
		return nil, fmt.Errorf("NewSender: %w", err)
	}

	dataChannelWaiter := make(chan *webrtc.DataChannel, 1)
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Println("Received Data Channel", dc.Label())
		dataChannelWaiter <- dc
	})

	receiverSD := getReceiverSD(sessionID)
	if err := pc.SetRemoteDescription(receiverSD); err != nil {
		return nil, fmt.Errorf("NewSender: %w", err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return nil, fmt.Errorf("NewSender: %w", err)
	}

	if err = pc.SetLocalDescription(answer); err != nil {
		return nil, fmt.Errorf("NewSender: %w", err)
	}

	fmt.Printf("Sender token: %s\n\n", encodeSDP(*pc.LocalDescription()))

	<-webrtc.GatheringCompletePromise(pc)

	dc := <-dataChannelWaiter
	return &Sender{SessionID: sessionID, pc: pc, dc: dc}, nil
}

func (s *Sender) Close() {
	s.dc.Close()
	s.pc.Close()
}

func (s *Sender) Send(filename string) error {
	log.Printf("Sending %q\n", filename)

	acceptanceWaiter := make(chan struct{})
	s.dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString && string(msg.Data) == fmt.Sprintf("accept %q", filename) {
			// once receiver accepts, we can then send
			log.Println("got 'accept' from receiver")
			close(acceptanceWaiter)
		}
	})

	tr := TransferRequest{
		FileName: filename,
		Size:     67,
	}

	bytes, err := tr.Marshal()
	if err != nil {
		return fmt.Errorf("Send: %w", err)
	}

	log.Println("Sending transfer request to receiver")
	if err := s.dc.Send(bytes); err != nil {
		return fmt.Errorf("Send: %w", err)
	}

	<-acceptanceWaiter
	log.Printf("Got approval from receiver to send %q\n", filename)
	// we can now send after getting approval from receiver

	return nil
}

func getReceiverSD(_ SessionID) webrtc.SessionDescription {
	fmt.Printf("Enter receiver token: ")
	receiverToken := readLine()
	return decodeSDP(receiverToken)
}
