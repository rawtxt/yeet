package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestP2PFileTransfer(t *testing.T) {
	// 1. Create a temporary file with some content to transfer
	tmpDir, err := os.MkdirTemp("", "yeet-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "source.bin")
	content := []byte(`Lorem ipsum dolor sit amet, consectetur adipiscing elit.
Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.
Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.`)
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// 2. Initialize the receiver
	receiver, err := NewReceiver()
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiver.Close()

	receiverToken := receiver.LocalToken()

	// 3. Initialize the sender using the receiver's token
	sender, err := NewSender("test-session", receiverToken)
	if err != nil {
		t.Fatalf("failed to create sender: %v", err)
	}
	defer sender.Close()

	senderToken := sender.LocalToken()

	// 4. Connect the receiver to the sender's token
	if err := receiver.Connect(senderToken); err != nil {
		t.Fatalf("failed to connect receiver to sender: %v", err)
	}

	// 5. In a separate goroutine, run the receiver wait logic
	recvDone := make(chan error, 1)
	go func() {
		// Wait for transfer request
		var tr TransferRequest
		select {
		case tr = <-receiver.TransferRequest():
			// Accept request
			if err := receiver.Accept(tr); err != nil {
				recvDone <- err
				return
			}
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for transfer request")
			recvDone <- fmt.Errorf("timeout")
			return
		}

		// Wait for completion
		select {
		case err := <-receiver.Done():
			recvDone <- err
		case <-time.After(10 * time.Second):
			t.Error("timed out waiting for transfer completion")
			recvDone <- fmt.Errorf("timeout")
		}
	}()

	// 6. Send the file on the main goroutine (in a background goroutine to avoid any blocking/deadlocks)
	sendErrChan := make(chan error, 1)
	go func() {
		sendErrChan <- sender.Send(srcPath)
	}()

	// 7. Check for sender errors
	select {
	case err := <-sendErrChan:
		if err != nil {
			t.Fatalf("sender failed: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("sender timed out")
	}

	// 8. Check for receiver errors
	select {
	case err := <-recvDone:
		if err != nil {
			t.Fatalf("receiver failed: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("receiver timed out")
	}

	// 9. Verify the transferred file
	destPath := "source.bin.download"
	defer os.Remove(destPath) // clean up after ourselves

	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if string(destContent) != string(content) {
		t.Errorf("content mismatch!\nExpected: %q\nGot:      %q", content, destContent)
	}

	// Double check with SHA256 hashes
	h1 := sha256.Sum256(content)
	h2 := sha256.Sum256(destContent)
	if h1 != h2 {
		t.Errorf("SHA256 hashes do not match!")
	} else {
		log.Println("Go Integration Test: Success! File transferred correctly with verified integrity.")
	}
}
