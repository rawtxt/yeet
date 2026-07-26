package main

import (
	"fmt"
	"net"
)

// Matchmaker orchestrates both the HTTP Signalling server and UDP STUN server in a unified self-hosted service.
type Matchmaker struct {
	SignallingServer *SignallingServer
	STUNServer       *STUNServer
	SignallingAddr   string
	STUNAddr         string
	BehindProxy      bool
	Silent           bool
}

// NewMatchmaker initializes a new Matchmaker containing a Signalling server and STUN server.
func NewMatchmaker() *Matchmaker {
	return &Matchmaker{
		SignallingServer: NewSignallingServer(),
		STUNServer:       NewSTUNServer(),
	}
}

// Start launches both the STUN server and HTTP Signalling server.
func (m *Matchmaker) Start(signallingAddr, stunAddr string) error {
	m.SignallingServer.Silent = m.Silent
	m.SignallingServer.BehindProxy = m.BehindProxy
	m.STUNServer.Silent = m.Silent

	stunActualAddr, err := m.STUNServer.Start(stunAddr)
	if err != nil {
		return fmt.Errorf("Matchmaker: failed to start STUN server: %w", err)
	}
	m.STUNAddr = stunActualAddr

	host, portStr, err := net.SplitHostPort(stunActualAddr)
	if err == nil {
		if host == "0.0.0.0" || host == "::" || host == "" {
			m.SignallingServer.StunURL = fmt.Sprintf("stun:%s", portStr)
		} else {
			m.SignallingServer.StunURL = fmt.Sprintf("stun:%s:%s", host, portStr)
		}
	} else {
		m.SignallingServer.StunURL = fmt.Sprintf("stun:%s", stunAddr)
	}

	sigActualAddr, err := m.SignallingServer.Start(signallingAddr)
	if err != nil {
		_ = m.STUNServer.Close()
		return fmt.Errorf("Matchmaker: failed to start Signalling server: %w", err)
	}
	m.SignallingAddr = sigActualAddr

	return nil
}

func (m *Matchmaker) Close() error {
	var errStun, errSig error
	if m.STUNServer != nil {
		errStun = m.STUNServer.Close()
	}
	if m.SignallingServer != nil {
		errSig = m.SignallingServer.Close()
	}
	if errStun != nil {
		return errStun
	}
	return errSig
}
