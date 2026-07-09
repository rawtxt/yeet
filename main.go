package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	signalling := flag.Bool("signalling", false, "Start custom signalling server")
	addr := flag.String("addr", ":8080", "Address for signalling server to listen on")
	server := flag.String("server", YeetSignallingServer, "Custom signalling server URL")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(flag.CommandLine.Output(), "To receive a file:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [-server <url>]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(flag.CommandLine.Output(), "To send a file:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [-server <url>] <filename>\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(flag.CommandLine.Output(), "To start a custom signalling node:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s -signalling [-addr <addr>]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(flag.CommandLine.Output(), "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *signalling {
		runSignalling(*addr)
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		runReceive(*server)
		return
	}

	runSend(*server, args[0])
}

func runSignalling(addr string) {
	server := NewSignallingServer()
	if err := server.Start(addr); err != nil {
		panic(err)
	}
}

func runSend(serverURL, filename string) {
	fmt.Printf("Enter 6-digit Session ID: ")
	sessionID := readLine()

	sender, err := NewSender(serverURL, SessionID(sessionID))
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

func runReceive(serverURL string) {
	receiver, err := NewReceiver(serverURL)
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
