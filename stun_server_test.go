package main

import (
	"net"
	"testing"
	"time"

	"github.com/pion/stun/v3"
)

func TestSTUNServerBinding(t *testing.T) {
	stunServer := NewSTUNServer()
	stunServer.Silent = true
	actualAddr, err := stunServer.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start STUN server: %v", err)
	}
	defer stunServer.Close()

	conn, err := net.Dial("udp", actualAddr)
	if err != nil {
		t.Fatalf("failed to dial STUN server: %v", err)
	}
	defer conn.Close()

	client, err := stun.NewClient(conn)
	if err != nil {
		t.Fatalf("failed to create STUN client: %v", err)
	}
	defer client.Close()

	var mappedAddr stun.XORMappedAddress
	done := make(chan error, 1)

	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	err = client.Do(message, func(res stun.Event) {
		if res.Error != nil {
			done <- res.Error
			return
		}
		if getErr := mappedAddr.GetFrom(res.Message); getErr != nil {
			done <- getErr
			return
		}
		done <- nil
	})

	if err != nil {
		t.Fatalf("STUN request failed: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("STUN response error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for STUN response")
	}

	if mappedAddr.IP == nil || mappedAddr.Port == 0 {
		t.Fatalf("expected valid mapped address, got IP=%v, Port=%d", mappedAddr.IP, mappedAddr.Port)
	}

	if !mappedAddr.IP.IsLoopback() {
		t.Errorf("expected loopback IP, got %v", mappedAddr.IP)
	}
}
