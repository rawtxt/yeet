package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pion/webrtc/v4"
)

type Sender struct {
	SessionID        SessionID
	pc               *webrtc.PeerConnection
	dc               *webrtc.DataChannel
	dataChannelReady chan struct{}
}

func NewSender(sessionID SessionID, receiverToken string) (*Sender, error) {
	pc, err := webrtc.NewPeerConnection(WebRTCConfig())
	if err != nil {
		return nil, fmt.Errorf("NewSender: %w", err)
	}

	s := &Sender{
		SessionID:        sessionID,
		pc:               pc,
		dataChannelReady: make(chan struct{}),
	}

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Println("Received Data Channel", dc.Label())
		s.dc = dc
		close(s.dataChannelReady)
	})

	receiverSD := decodeSDP(receiverToken)
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

	<-webrtc.GatheringCompletePromise(pc)

	return s, nil
}

func (s *Sender) LocalToken() string {
	return encodeSDP(*s.pc.LocalDescription())
}

func (s *Sender) Close() {
	s.dc.Close()
	s.pc.Close()
}

func (s *Sender) Send(filename string) error {
	log.Println("Waiting for PeerConnection to establish data channel...")
	select {
	case <-s.dataChannelReady:
		log.Println("Data channel established successfully!")
	case <-time.After(30 * time.Second):
		return fmt.Errorf("Send: timed out waiting for connection from receiver")
	}

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Send: failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Send: failed to stat file: %w", err)
	}

	baseName := filepath.Base(filename)
	log.Printf("Preparing to send %q (%d bytes)\n", baseName, stat.Size())

	acceptanceWaiter := make(chan struct{})
	s.dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString && string(msg.Data) == fmt.Sprintf("accept %q", baseName) {
			// once receiver accepts, we can then send
			log.Println("got 'accept' from receiver")
			close(acceptanceWaiter)
		}
	})

	tr := TransferRequest{
		FileName: baseName,
		Size:     int(stat.Size()),
	}

	bytes, err := tr.Marshal()
	if err != nil {
		return fmt.Errorf("Send: %w", err)
	}

	log.Println("Sending transfer request to receiver")
	if err := s.dc.SendText(string(bytes)); err != nil {
		return fmt.Errorf("Send: %w", err)
	}

	<-acceptanceWaiter
	log.Printf("Got approval from receiver to send %q\n", baseName)

	buffer := make([]byte, 16*1024)
	totalSent := int64(0)
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			// WebRTC flow control: ensure we don't overwhelm the sending buffer.
			// If BufferedAmount is too high (e.g. > 1MB), wait for it to clear.
			for s.dc.BufferedAmount() > 1024*1024 {
				time.Sleep(10 * time.Millisecond)
			}

			if err := s.dc.Send(buffer[:n]); err != nil {
				return fmt.Errorf("Send: failed to send chunk: %w", err)
			}
			totalSent += int64(n)
			log.Printf("Progress: %d / %d bytes sent (%.2f%%)\n", totalSent, stat.Size(), float64(totalSent)/float64(stat.Size())*100)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("Send: failed reading file: %w", err)
		}
	}

	log.Println("File sent completely!")
	return nil
}
