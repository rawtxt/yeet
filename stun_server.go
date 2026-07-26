package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"github.com/pion/stun/v3"
)

type STUNServer struct {
	addr      string
	conn      *net.UDPConn
	closed    atomic.Bool
	closeChan chan struct{}
	mu        sync.Mutex
	Silent    bool
}

func NewSTUNServer() *STUNServer {
	return &STUNServer{
		closeChan: make(chan struct{}),
	}
}

func (s *STUNServer) logf(format string, v ...any) {
	if !s.Silent {
		log.Printf(format, v...)
	}
}

func (s *STUNServer) Start(addr string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return "", fmt.Errorf("STUNServer: failed to resolve address %s: %w", addr, err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return "", fmt.Errorf("STUNServer: failed to listen on %s: %w", addr, err)
	}
	s.conn = conn
	s.addr = conn.LocalAddr().String()

	s.logf("STUN server listening on %s (UDP)\n", s.addr)

	go s.serve()

	return s.addr, nil
}

func (s *STUNServer) serve() {
	buf := make([]byte, 1500)
	for {
		n, clientAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if s.closed.Load() {
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			s.logf("STUN server read error: %v\n", err)
			return
		}

		if n < 20 || !stun.IsMessage(buf[:n]) {
			continue
		}

		var req stun.Message
		req.Raw = make([]byte, n)
		copy(req.Raw, buf[:n])
		if err := req.Decode(); err != nil {
			continue
		}

		if req.Type.Class == stun.ClassRequest && req.Type.Method == stun.MethodBinding {
			res := stun.New()
			err := res.Build(
				stun.NewTransactionIDSetter(req.TransactionID),
				stun.BindingSuccess,
				&stun.XORMappedAddress{
					IP:   clientAddr.IP,
					Port: clientAddr.Port,
				},
				stun.Fingerprint,
			)
			if err != nil {
				s.logf("STUN server failed to build response: %v\n", err)
				continue
			}

			_, _ = s.conn.WriteToUDP(res.Raw, clientAddr)
		}
	}
}

func (s *STUNServer) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	close(s.closeChan)
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
