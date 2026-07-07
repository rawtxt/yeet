package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
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

func NewSender(serverURL string, sessionID SessionID) (*Sender, error) {
	u, err := user.Current()
	var username string
	if err == nil {
		username = u.Username
	} else {
		username = "unknown"
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	senderName := fmt.Sprintf("%s@%s", username, hostname)

	// log.Printf("Connecting to signalling server, requesting connection to receiver %s as '%s'...\n", sessionID, senderName)
	reqBody, err := json.Marshal(map[string]string{
		"sender_name": senderName,
	})
	if err != nil {
		return nil, fmt.Errorf("NewSender: %w", err)
	}

	resp, err := http.Post(serverURL+"/connect?session_id="+string(sessionID), "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("NewSender: server connect request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("NewSender: connection rejected by the receiver")
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NewSender: server returned status: %s", resp.Status)
	}

	var res struct {
		ReceiverToken string `json:"receiver_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("NewSender: failed to parse response: %w", err)
	}

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
		// log.Println("Received Data Channel", dc.Label())
		s.dc = dc
		close(s.dataChannelReady)
	})

	receiverSD := decodeSDP(res.ReceiverToken)
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

	// log.Println("Submitting connection answer to signalling server...")
	localAnswer := s.LocalToken()
	answerReqBody, err := json.Marshal(map[string]string{
		"sender_token": localAnswer,
	})
	if err != nil {
		return nil, fmt.Errorf("NewSender: failed to marshal answer: %w", err)
	}

	answerResp, err := http.Post(serverURL+"/answer?session_id="+string(sessionID), "application/json", bytes.NewReader(answerReqBody))
	if err != nil {
		return nil, fmt.Errorf("NewSender: failed to submit answer to server: %w", err)
	}
	defer answerResp.Body.Close()

	if answerResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NewSender: failed to submit answer: server returned status %s", answerResp.Status)
	}

	return s, nil
}

func (s *Sender) LocalToken() string {
	return encodeSDP(*s.pc.LocalDescription())
}

func (s *Sender) Close() {
	if s.dc != nil {
		s.dc.Close()
	}
	if s.pc != nil {
		s.pc.Close()
	}
}

func (s *Sender) Send(filename string) error {
	// log.Println("Waiting for PeerConnection to establish data channel...")
	select {
	case <-s.dataChannelReady:
		// log.Println("Data channel established successfully!")
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
	// log.Printf("Preparing to send %q (%d bytes)\n", baseName, stat.Size())

	acceptanceWaiter := make(chan struct{})
	s.dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString && string(msg.Data) == fmt.Sprintf("accept %q", baseName) {
			// once receiver accepts, we can then send
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

	// log.Println("Sending transfer request to receiver")
	if err := s.dc.SendText(string(bytes)); err != nil {
		return fmt.Errorf("Send: %w", err)
	}

	<-acceptanceWaiter
	// log.Printf("Got approval from receiver to send %q\n", baseName)

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
			percent := float64(totalSent) / float64(stat.Size()) * 100
			fmt.Printf("\r📤 Yeeting... %.1f%% (%s / %s)", percent, formatSize(totalSent), formatSize(stat.Size()))
			// log.Printf("Progress: %d / %d bytes sent (%.2f%%)\n", totalSent, stat.Size(), float64(totalSent)/float64(stat.Size())*100)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("Send: failed reading file: %w", err)
		}
	}
	fmt.Println()

	// log.Println("File sent completely! Waiting for receiver confirmation...")

	doneWaiter := make(chan struct{})
	s.dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString && string(msg.Data) == "done" {
			// log.Println("Receiver confirmed receipt of all bytes.")
			close(doneWaiter)
		}
	})

	select {
	case <-doneWaiter:
		// log.Println("Receiver confirmed successful receipt of all bytes. Transfer complete!")
	case <-time.After(15 * time.Second):
		return fmt.Errorf("Send: timed out waiting for receiver completion acknowledgment")
	}

	return nil
}
