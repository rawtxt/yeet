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

		fmt.Printf("Enter session ID: ")
		sessionID := readLine()

		fmt.Printf("Enter receiver token: ")
		receiverToken := readLine()

		sender, err := NewSender(SessionID(sessionID), receiverToken)
		if err != nil {
			panic(err)
		}
		defer sender.Close()

		fmt.Printf("Sender token: %s\n\n", sender.LocalToken())

		if err := sender.Send(filename); err != nil {
			panic(err)
		}
		fmt.Println("Sender done!")

	case "receive":
		receiver, err := NewReceiver()
		if err != nil {
			panic(err)
		}
		defer receiver.Close()

		fmt.Println("receiver token:", receiver.LocalToken())

		fmt.Printf("\nEnter sender token: ")
		senderToken := readLine()
		if err := receiver.Connect(senderToken); err != nil {
			panic(err)
		}

		tr := <-receiver.TransferRequest()
		fmt.Printf("Received transfer request for %s (%d bytes)\n", tr.FileName, tr.Size)

		fmt.Println("Accepting request...")
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
