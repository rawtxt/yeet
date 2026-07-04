package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: yeet send <filename> OR yeet receive")
		os.Exit(1)
	}

	switch role := os.Args[1]; role {
	case "send":
		if len(os.Args) < 3 {
			fmt.Println("Usage: yeet send <filename>")
			os.Exit(1)
		}
		filename := os.Args[2]

		fmt.Printf("Enter session ID: ")
		sessionID := readLine()
		sender, err := NewSender(SessionID(sessionID))
		if err != nil {
			panic(err)
		}
		defer sender.Close()

		if err := sender.Send(filename); err != nil {
			panic(err)
		}
		select {}

	case "receive":
		// runReceiver(pc)
		receiver, err := NewReceiver()
		if err != nil {
			panic(err)
		}
		defer receiver.Close()

		// TODO: this should be done automatically in initSession
		fmt.Printf("\nEnter sender token: ")
		senderToken := readLine()
		if err := receiver.pc.SetRemoteDescription(decodeSDP(senderToken)); err != nil {
			panic(err)
		}

		tr := <-receiver.TransferRequest()
		fmt.Printf("Received %#v\n", tr)

		fmt.Println("Accepting request")
		receiver.Accept(tr)
		select {}
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
