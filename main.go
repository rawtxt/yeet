package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// `yeet receive` or just `yeet` to receive
	if len(os.Args) < 2 || os.Args[1] == "receive" {
		runReceive()
		return
	}

	// `./yeet signalling` to start custom signalling server
	if os.Args[1] == "signalling" {
		addr := ":8080"
		if len(os.Args) >= 3 {
			addr = os.Args[2]
		}
		runSignalling(addr)
		return
	}

	// `yeet send <filename>` or just `yeet <filename>` to send
	filename := os.Args[1]
	if os.Args[1] == "send" && len(os.Args) >= 3 {
		filename = os.Args[2]
	}

	runSend(filename)
}

func runSignalling(addr string) {
	server := NewSignallingServer()
	if err := server.Start(addr); err != nil {
		panic(err)
	}
}

func runSend(filename string) {
	fmt.Printf("Enter 6-digit Session ID: ")
	sessionID := readLine()

	sender, err := NewSender(YeetSignallingServer, SessionID(sessionID))
	if err != nil {
		panic(err)
	}
	defer sender.Close()

	fmt.Println("🔗 Connected to signalling server! Handshaking with receiver...")

	if err := sender.Send(filename); err != nil {
		panic(err)
	}
	fmt.Printf("\n✨ %s yeeted successfully!\n", filepath.Base(filename))
}

func runReceive() {
	receiver, err := NewReceiver(YeetSignallingServer)
	if err != nil {
		panic(err)
	}
	defer receiver.Close()

	fmt.Printf("🚀 Your 6-digit Session ID: %s\n", receiver.SessionID)
	fmt.Println("⏳ Waiting for a sender to connect...")

	senderName := <-receiver.SenderRequest()
	fmt.Printf("\n🔔 Connection request from '%s'. Accept? (y/n): ", senderName)
	answer := readLine()
	if strings.ToLower(answer) != "y" && strings.ToLower(answer) != "yes" {
		if err := receiver.RejectConnection(); err != nil {
			fmt.Printf("❌ Error rejecting connection: %v\n", err)
		}
		fmt.Println("❌ Connection rejected.")
		return
	}

	fmt.Println("🔗 Connection accepted! Establishing direct P2P link...")
	if err := receiver.ApproveConnection(); err != nil {
		panic(err)
	}

	senderToken := <-receiver.SenderAnswer()
	if err := receiver.Connect(senderToken); err != nil {
		panic(err)
	}

	tr := <-receiver.TransferRequest()
	fmt.Printf("📦 Incoming file: %s (%s)\n", tr.FileName, formatSize(int64(tr.Size)))

	if err := receiver.Accept(tr); err != nil {
		panic(err)
	}

	fmt.Println("📥 Downloading...")

	if err := <-receiver.Done(); err != nil {
		fmt.Printf("❌ Error during transfer: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\n🎉 %s received successfully! Saved as %s.yeeted\n", tr.FileName, tr.FileName)
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func readLine() string {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(line)
}
