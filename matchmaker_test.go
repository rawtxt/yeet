package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMatchmakerEndpoints(t *testing.T) {
	mm := NewMatchmaker()
	mm.Silent = true
	if err := mm.Start("127.0.0.1:0", "127.0.0.1:0"); err != nil {
		t.Fatalf("failed to start matchmaker: %v", err)
	}
	defer mm.Close()

	sigURL := "http://" + mm.SignallingAddr

	resp, err := http.Get(sigURL + "/health")
	if err != nil {
		t.Fatalf("failed to get /health: %v", err)
	}
	defer resp.Body.Close()

	var hres struct {
		Status     string `json:"status"`
		StunServer string `json:"stun_server"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&hres); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if hres.Status != "OK" {
		t.Errorf("expected status OK, got %q", hres.Status)
	}
	if !strings.HasPrefix(hres.StunServer, "stun:") {
		t.Errorf("expected stun_server starting with 'stun:', got %q", hres.StunServer)
	}
}

func TestMatchmakerE2EFileTransfer(t *testing.T) {
	mm := NewMatchmaker()
	mm.Silent = true
	if err := mm.Start("127.0.0.1:0", "127.0.0.1:0"); err != nil {
		t.Fatalf("failed to start matchmaker: %v", err)
	}
	defer mm.Close()

	matchmakerURL := "http://" + mm.SignallingAddr

	tmpDir, err := os.MkdirTemp("", "yeet-mm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "matchmaker_source.bin")
	content := []byte(`Hello from self-hosted Matchmaker server containing both Signalling and STUN!`)
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	receiver, err := NewReceiver(matchmakerURL)
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiver.Close()

	recvDone := make(chan error, 1)
	go func() {
		select {
		case senderName := <-receiver.SenderRequest():
			_ = senderName
		case <-time.After(5 * time.Second):
			recvDone <- fmt.Errorf("timeout waiting for sender request")
			return
		}

		if err := receiver.ApproveConnection(); err != nil {
			recvDone <- fmt.Errorf("failed to approve connection: %w", err)
			return
		}

		var senderToken string
		select {
		case senderToken = <-receiver.SenderAnswer():
		case <-time.After(5 * time.Second):
			recvDone <- fmt.Errorf("timeout waiting for sender answer")
			return
		}

		if err := receiver.Connect(senderToken); err != nil {
			recvDone <- fmt.Errorf("failed to connect receiver: %w", err)
			return
		}

		var tr TransferRequest
		select {
		case tr = <-receiver.TransferRequest():
		case <-time.After(5 * time.Second):
			recvDone <- fmt.Errorf("timeout waiting for transfer request")
			return
		}

		if err := receiver.Accept(tr); err != nil {
			recvDone <- fmt.Errorf("failed to accept transfer: %w", err)
			return
		}

		select {
		case err := <-receiver.Done():
			recvDone <- err
		case <-time.After(10 * time.Second):
			recvDone <- fmt.Errorf("timeout waiting for transfer completion")
		}
	}()

	sendErrChan := make(chan error, 1)
	go func() {
		sender, err := NewSender(matchmakerURL, receiver.SessionID)
		if err != nil {
			sendErrChan <- fmt.Errorf("failed to create sender: %w", err)
			return
		}
		defer sender.Close()

		sendErrChan <- sender.Send(srcPath)
	}()

	select {
	case err := <-sendErrChan:
		if err != nil {
			t.Fatalf("sender failed: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("sender timed out")
	}

	select {
	case err := <-recvDone:
		if err != nil {
			t.Fatalf("receiver failed: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("receiver timed out")
	}

	destPath := "matchmaker_source.bin"
	defer os.Remove(destPath)

	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if string(destContent) != string(content) {
		t.Errorf("content mismatch!\nExpected: %q\nGot:      %q", content, destContent)
	}

	h1 := sha256.Sum256(content)
	h2 := sha256.Sum256(destContent)
	if h1 != h2 {
		t.Errorf("SHA256 hashes do not match!")
	}
}
