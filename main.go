package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	switch role := os.Args[1]; role {
	case "send":
		fmt.Printf("Enter session ID: ")
		sessionID := readLine()
		sender, err := NewSender(SessionID(sessionID))
		if err != nil {
			panic(err)
		}
		defer sender.Close()

		sender.Send("foobar.txt")
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
