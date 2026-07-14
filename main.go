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
		fmt.Fprintf(flag.CommandLine.Output(), "To send files:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [-server <url>] <filename1> [<filename2> ...]\n\n", filepath.Base(os.Args[0]))
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

	runSend(*server, args)
}

func runSignalling(addr string) {
	server := NewSignallingServer()
	_, err := server.Start(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error starting signalling server: %v\n", err)
		os.Exit(1)
	}
	select {}
}

func runSend(serverURL string, filenames []string) {
	fmt.Printf("Enter 6-digit Session ID: ")
	sessionID := readLine()

	sender, err := NewSender(serverURL, SessionID(sessionID))
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "no such host") || strings.Contains(errStr, "unreachable") {
			fmt.Fprintf(os.Stderr, "❌ Error: Signalling server unreachable (offline or DNS failed).\n")
			fmt.Fprintf(os.Stderr, "💡 Tip: Check your internet connection, or make sure the receiver is running on the same local network (LAN) and has generated Session ID %s.\n", sessionID)
		} else {
			fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		}
		os.Exit(1)
	}
	defer sender.Close()

	fmt.Println("🔗 Connected to signalling server! Handshaking with receiver...")

	for _, filename := range filenames {
		if err := sender.Send(filename); err != nil {
			fmt.Fprintf(os.Stderr, "\n❌ Error sending file %s: %v\n", filename, err)
			os.Exit(1)
		}
	}
}

func runReceive(serverURL string) {
	receiver, err := NewReceiver(serverURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error starting receiver: %v\n", err)
		os.Exit(1)
	}
	defer receiver.Close()

	fmt.Printf("Your 6-digit Session ID: %s\n", receiver.SessionID)
	fmt.Println("Waiting for a sender to connect...")

	senderName := <-receiver.SenderRequest()
	fmt.Printf("\nConnection request from '%s'. Accept? (y/n): ", senderName)
	answer := readLine()
	if strings.ToLower(answer) != "y" && strings.ToLower(answer) != "yes" {
		if err := receiver.RejectConnection(); err != nil {
			fmt.Printf("❌ Error rejecting connection: %v\n", err)
		}
		fmt.Println("❌ Connection rejected.")
		return
	}

	fmt.Println("Connection accepted! Establishing direct P2P link...")
	if err := receiver.ApproveConnection(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error accepting connection: %v\n", err)
		os.Exit(1)
	}

	senderToken := <-receiver.SenderAnswer()
	if err := receiver.Connect(senderToken); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error establishing connection: %v\n", err)
		os.Exit(1)
	}

	for tr := range receiver.TransferRequest() {
		if err := receiver.Accept(tr); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error accepting file transfer: %v\n", err)
			os.Exit(1)
		}

		if err := <-receiver.Done(); err != nil {
			fmt.Printf("❌ Error during transfer: %v\n", err)
			os.Exit(1)
		}
	}
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
		fmt.Fprintf(os.Stderr, "❌ Error reading input: %v\n", err)
		os.Exit(1)
	}
	return strings.TrimSpace(line)
}
