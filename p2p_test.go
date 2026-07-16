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

func TestP2PFileTransferRemote(t *testing.T) {
	server := NewSignallingServer()
	go func() {
		_, _ = server.Start(":18080")
	}()
	// Give the server a few milliseconds to start up
	time.Sleep(50 * time.Millisecond)

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

	receiver, err := NewReceiver("http://localhost:18080")
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiver.Close()

	recvDone := make(chan error, 1)
	go func() {
		var senderName string
		select {
		case senderName = <-receiver.SenderRequest():
			log.Printf("Test Receiver: Received request from '%s'\n", senderName)
		case <-time.After(5 * time.Second):
			recvDone <- fmt.Errorf("timeout waiting for sender request")
			return
		}

		if err := receiver.ApproveConnection(); err != nil {
			recvDone <- fmt.Errorf("failed to approve: %w", err)
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
			log.Printf("Test Receiver: Received transfer request for %s (%d bytes)\n", tr.FileName, tr.Size)
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
		sender, err := NewSender("http://localhost:18080", receiver.SessionID)
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

	destPath := "source.bin"
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
	} else {
		log.Println("Go Integration Test: Success! File transferred completely via mock signalling server.")
	}
}

func TestP2PFileTransferLocal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yeet-test-local-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "source.bin")
	content := []byte(`P2P Local Offline mDNS Transfer Content. Foobar quzbar....`)
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	receiver, err := NewReceiver("http://127.0.0.1:55555")
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiver.Close()

	recvDone := make(chan error, 1)
	go func() {
		var senderName string
		select {
		case senderName = <-receiver.SenderRequest():
			log.Printf("Test Local Receiver: Received request from '%s'\n", senderName)
		case <-time.After(5 * time.Second):
			recvDone <- fmt.Errorf("timeout waiting for sender request")
			return
		}

		if err := receiver.ApproveConnection(); err != nil {
			recvDone <- fmt.Errorf("failed to approve: %w", err)
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
			log.Printf("Test Local Receiver: Received transfer request for %s (%d bytes)\n", tr.FileName, tr.Size)
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
		sender, err := NewSender(receiver.LocalServerURL, receiver.SessionID)
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

	destPath := "source.bin"
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
	} else {
		log.Println("Go Integration Test: Success! File transferred completely via local mDNS offline fallback mode.")
	}
}

func TestP2PFileTransferE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "yeet-test-e2e-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "source.bin")
	content := []byte(`P2P WAN Real-World E2E handshaking over Fly.io. Consectetur adipiscing elit.`)
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	receiver, err := NewReceiver(YeetSignallingServer)
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiver.Close()

	recvDone := make(chan error, 1)
	go func() {
		var senderName string
		select {
		case senderName = <-receiver.SenderRequest():
			log.Printf("Test E2E Receiver: Received request from '%s'\n", senderName)
		case <-time.After(15 * time.Second):
			recvDone <- fmt.Errorf("timeout waiting for sender request")
			return
		}

		if err := receiver.ApproveConnection(); err != nil {
			recvDone <- fmt.Errorf("failed to approve: %w", err)
			return
		}

		var senderToken string
		select {
		case senderToken = <-receiver.SenderAnswer():
		case <-time.After(15 * time.Second):
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
			log.Printf("Test E2E Receiver: Received transfer request for %s (%d bytes)\n", tr.FileName, tr.Size)
		case <-time.After(15 * time.Second):
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
		case <-time.After(25 * time.Second):
			recvDone <- fmt.Errorf("timeout waiting for transfer completion")
		}
	}()

	sendErrChan := make(chan error, 1)
	go func() {
		sender, err := NewSender(YeetSignallingServer, receiver.SessionID)
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
	case <-time.After(35 * time.Second):
		t.Fatalf("sender timed out")
	}

	select {
	case err := <-recvDone:
		if err != nil {
			t.Fatalf("receiver failed: %v", err)
		}
	case <-time.After(35 * time.Second):
		t.Fatalf("receiver timed out")
	}

	destPath := "source.bin"
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
	} else {
		log.Println("Go Integration Test: Success! File transferred completely via deployed E2E signalling server.")
	}
}

func TestP2PMultiFileTransferLocal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yeet-test-multi-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := []struct {
		name    string
		content []byte
	}{
		{"file1.bin", []byte("First file contents. Yay!")},
		{"file2.bin", []byte("Second file contents. Dynamic, cool!")},
		{"file3.bin", []byte("Third file contents. Extremely fast transfer, hopefully!")},
	}

	var srcPaths []string
	for _, f := range files {
		srcPath := filepath.Join(tmpDir, f.name)
		if err := os.WriteFile(srcPath, f.content, 0644); err != nil {
			t.Fatalf("failed to write test file %s: %v", f.name, err)
		}
		srcPaths = append(srcPaths, srcPath)
	}

	receiver, err := NewReceiver("http://127.0.0.1:55556")
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiver.Close()

	recvDone := make(chan error, 1)
	go func() {
		var senderName string
		select {
		case senderName = <-receiver.SenderRequest():
			log.Printf("Test Multi-file Receiver: Received request from '%s'\n", senderName)
		case <-time.After(5 * time.Second):
			recvDone <- fmt.Errorf("timeout waiting for sender request")
			return
		}

		if err := receiver.ApproveConnection(); err != nil {
			recvDone <- fmt.Errorf("failed to approve: %w", err)
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

		for range files {
			var tr TransferRequest
			select {
			case tr = <-receiver.TransferRequest():
				log.Printf("Test Multi-file Receiver: Received transfer request for %s (%d bytes)\n", tr.FileName, tr.Size)
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
				if err != nil {
					recvDone <- err
					return
				}
			case <-time.After(10 * time.Second):
				recvDone <- fmt.Errorf("timeout waiting for transfer completion")
				return
			}
		}

		recvDone <- nil
	}()

	sendErrChan := make(chan error, 1)
	go func() {
		sender, err := NewSender(receiver.LocalServerURL, receiver.SessionID)
		if err != nil {
			sendErrChan <- fmt.Errorf("failed to create sender: %w", err)
			return
		}
		defer sender.Close()

		for _, srcPath := range srcPaths {
			if err := sender.Send(srcPath); err != nil {
				sendErrChan <- err
				return
			}
		}
		sendErrChan <- nil
	}()

	select {
	case err := <-sendErrChan:
		if err != nil {
			t.Fatalf("sender failed: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("sender timed out")
	}

	select {
	case err := <-recvDone:
		if err != nil {
			t.Fatalf("receiver failed: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("receiver timed out")
	}

	for _, f := range files {
		destPath := f.name
		defer os.Remove(destPath)

		destContent, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("failed to read downloaded file %s: %v", destPath, err)
		}

		if string(destContent) != string(f.content) {
			t.Errorf("content mismatch for %s!\nExpected: %q\nGot:      %q", f.name, f.content, destContent)
		}

		h1 := sha256.Sum256(f.content)
		h2 := sha256.Sum256(destContent)
		if h1 != h2 {
			t.Errorf("SHA256 hashes do not match for %s!", f.name)
		}
	}
	log.Println("Go Integration Test: Success! Multi-file local transfer completed.")
}

func TestUniqueFilename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yeet-resolve-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	basePath := filepath.Join(tmpDir, "testfile.txt")

	// 1. Path doesn't exist yet: should return original name
	got1 := uniqueFilename(basePath)
	if got1 != basePath {
		t.Errorf("expected original path %q, got %q", basePath, got1)
	}

	// 2. Create the file: should return "testfile (1).txt"
	if err := os.WriteFile(basePath, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	expected1 := filepath.Join(tmpDir, "testfile (1).txt")
	got2 := uniqueFilename(basePath)
	if got2 != expected1 {
		t.Errorf("expected %q, got %q", expected1, got2)
	}

	// 3. Create the "testfile (1).txt" file: should return "testfile (2).txt"
	if err := os.WriteFile(expected1, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	expected2 := filepath.Join(tmpDir, "testfile (2).txt")
	got3 := uniqueFilename(basePath)
	if got3 != expected2 {
		t.Errorf("expected %q, got %q", expected2, got3)
	}
}
