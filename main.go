package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: yeet send <filename> OR yeet receive OR yeet signalling [addr]")
		os.Exit(1)
	}

	switch role := os.Args[1]; role {
	case "signalling":
		addr := ":8080"
		if len(os.Args) >= 3 {
			addr = os.Args[2]
		}
		server := NewSignallingServer()
		if err := server.Start(addr); err != nil {
			panic(err)
		}
	case "send":
		if len(os.Args) < 3 {
			fmt.Println("Usage: yeet send <filename>")
			os.Exit(1)
		}
		filename := os.Args[2]

		fmt.Printf("Enter 6-digit Session ID: ")
		sessionID := readLine()

		sender, err := NewSender(YeetSignallingServer, SessionID(sessionID))
		if err != nil {
			panic(err)
		}
		defer sender.Close()

		if err := sender.Send(filename); err != nil {
			panic(err)
		}
		fmt.Println("Sender done!")

	case "receive":
		receiver, err := NewReceiver(YeetSignallingServer)
		if err != nil {
			panic(err)
		}
		defer receiver.Close()

		fmt.Printf("Your 6-digit Session ID: %s\n", receiver.SessionID)
		fmt.Println("Waiting for a sender to connect...")

		senderName := <-receiver.SenderRequest()
		fmt.Printf("\nIncoming connection request from '%s'. Accept? (y/n): ", senderName)
		answer := readLine()
		if strings.ToLower(answer) != "y" && strings.ToLower(answer) != "yes" {
			if err := receiver.RejectConnection(); err != nil {
				fmt.Printf("Error rejecting connection: %v\n", err)
			}
			fmt.Println("Connection rejected.")
			return
		}

		fmt.Println("Accepting connection, establishing P2P link...")
		if err := receiver.ApproveConnection(); err != nil {
			panic(err)
		}

		senderToken := <-receiver.SenderAnswer()
		if err := receiver.Connect(senderToken); err != nil {
			panic(err)
		}

		tr := <-receiver.TransferRequest()
		fmt.Printf("Received transfer request for %s (%d bytes)\n", tr.FileName, tr.Size)

		fmt.Println("Accepting transfer request...")
		if err := receiver.Accept(tr); err != nil {
			panic(err)
		}

		if err := <-receiver.Done(); err != nil {
			fmt.Printf("Error during transfer: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Receiver done! File successfully saved as %s.download\n", tr.FileName)
	}
}

func readLine() string {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(line)
}
