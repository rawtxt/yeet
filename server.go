package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	mrand "math/rand/v2"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
)

type Session struct {
	ID            string
	SecretToken   string
	ReceiverToken string
	EventChan     chan string
	ApprovedChan  chan bool
	ExpiresAt     time.Time
}

type SignallingServer struct {
	mu          sync.Mutex
	sessions    map[string]*Session
	Silent      bool
	MaxSessions int
}

func NewSignallingServer() *SignallingServer {
	return &SignallingServer{
		sessions:    make(map[string]*Session),
		MaxSessions: 10000,
	}
}

func (s *SignallingServer) logf(format string, v ...any) {
	if !s.Silent {
		log.Printf(format, v...)
	}
}

func (s *SignallingServer) AddSession(sessionID, secretToken, receiverToken string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addSessionLocked(sessionID, secretToken, receiverToken)
}

func (s *SignallingServer) addSessionLocked(sessionID, secretToken, receiverToken string) {
	session := &Session{
		ID:            sessionID,
		SecretToken:   secretToken,
		ReceiverToken: receiverToken,
		EventChan:     make(chan string, 10),
		ApprovedChan:  make(chan bool, 1),
		ExpiresAt:     time.Now().Add(5 * time.Minute),
	}
	s.sessions[sessionID] = session
}

func (s *SignallingServer) Start(addr string) (string, error) {
	// Start background janitor to clean up expired sessions
	go s.reapExpiredSessions()

	mux := http.NewServeMux()
	mux.HandleFunc("/register", s.handleRegister)
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/connect", s.handleConnect)
	mux.HandleFunc("/approve", s.handleApprove)
	mux.HandleFunc("/answer", s.handleAnswer)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}

	actualAddr := listener.Addr().String()
	s.logf("Signalling server starting on %s\n", actualAddr)

	go func() {
		_ = http.Serve(listener, mux)
	}()

	return actualAddr, nil
}

func (s *SignallingServer) Reap() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	reapedCount := 0
	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			close(session.EventChan)
			delete(s.sessions, id)
			reapedCount++
		}
	}
	return reapedCount
}

func (s *SignallingServer) reapExpiredSessions() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		reapedCount := s.Reap()
		if reapedCount > 0 {
			s.logf("[Server] Cleaned up %d expired sessions\n", reapedCount)
		}
	}
}

func generateSecretToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand shouldn't fail
	}
	return hex.EncodeToString(b)
}

func generateSessionID() string {
	return fmt.Sprintf("%06d", mrand.IntN(1000000))
}

func sanitizeSenderName(name string) string {
	var sb strings.Builder
	for _, r := range name {
		if unicode.IsPrint(r) {
			sb.WriteRune(r)
		}
		if sb.Len() >= 64 {
			break
		}
	}
	return sb.String()
}

func (s *SignallingServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10240)

	var req struct {
		ReceiverToken string `json:"receiver_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if len(s.sessions) >= s.MaxSessions {
		s.mu.Unlock()
		http.Error(w, "Server is at full capacity", http.StatusServiceUnavailable)
		return
	}

	var sessionID string
	for {
		sessionID = generateSessionID()
		if _, exists := s.sessions[sessionID]; !exists {
			break
		}
	}

	secretToken := generateSecretToken()
	s.addSessionLocked(sessionID, secretToken, req.ReceiverToken)
	s.mu.Unlock()

	s.logf("[Server] Registered session %s (secure token generated)\n", sessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"session_id":   sessionID,
		"secret_token": secretToken,
	})
}

func (s *SignallingServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	token := r.URL.Query().Get("token")
	if sessionID == "" || token == "" {
		http.Error(w, "Missing session_id or token", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	session, exists := s.sessions[sessionID]
	s.mu.Unlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Validate secret token
	if session.SecretToken != token {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Set headers for Server-Sent Events (SSE)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Notify receiver that the stream is open
	fmt.Fprintf(w, "data: connected\n\n")
	flusher.Flush()

	s.logf("[Server] Session %s opened SSE stream\n", sessionID)

	for {
		select {
		case event, ok := <-session.EventChan:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", event)
			flusher.Flush()
		case <-r.Context().Done():
			s.logf("[Server] Session %s closed SSE stream\n", sessionID)
			s.mu.Lock()
			delete(s.sessions, sessionID)
			s.mu.Unlock()
			return
		}
	}
}

func (s *SignallingServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "Missing session_id", http.StatusBadRequest)
		return
	}

	var req struct {
		SenderName string `json:"sender_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.SenderName = sanitizeSenderName(req.SenderName)
	if req.SenderName == "" {
		req.SenderName = "Unknown Sender"
	}

	s.mu.Lock()
	session, exists := s.sessions[sessionID]
	s.mu.Unlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	s.logf("[Server] Sender '%s' requesting connection to session %s\n", req.SenderName, sessionID)

	// Notify receiver that a sender wants to connect (include sender name)
	select {
	case session.EventChan <- fmt.Sprintf("sender_request %s", req.SenderName):
	default:
		s.logf("[Server] Warning: session %s event channel full\n", sessionID)
	}

	// Wait for receiver's approval (timeout after 30 seconds)
	select {
	case approved := <-session.ApprovedChan:
		if approved {
			s.logf("[Server] Connection approved for session %s\n", sessionID)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"receiver_token": session.ReceiverToken,
			})
		} else {
			s.logf("[Server] Connection rejected for session %s\n", sessionID)
			http.Error(w, "Connection rejected by receiver", http.StatusForbidden)
		}
	case <-time.After(30 * time.Second):
		s.logf("[Server] Connection request timed out for session %s\n", sessionID)
		http.Error(w, "Request timed out waiting for approval", http.StatusRequestTimeout)
	}
}

func (s *SignallingServer) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	status := r.URL.Query().Get("status")
	token := r.URL.Query().Get("token")
	if sessionID == "" || status == "" || token == "" {
		http.Error(w, "Missing session_id, status, or token", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	session, exists := s.sessions[sessionID]
	s.mu.Unlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Validate secret token
	if session.SecretToken != token {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	approved := status == "accept"
	select {
	case session.ApprovedChan <- approved:
	default:
		// already handled or channel full
	}

	w.WriteHeader(http.StatusOK)
}

func (s *SignallingServer) handleAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10240)

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "Missing session_id", http.StatusBadRequest)
		return
	}

	var req struct {
		SenderToken string `json:"sender_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	session, exists := s.sessions[sessionID]
	s.mu.Unlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	s.logf("[Server] Forwarding sender's answer token to session %s\n", sessionID)

	select {
	case session.EventChan <- fmt.Sprintf("sender_answer %s", req.SenderToken):
	default:
		s.logf("[Server] Warning: session %s event channel full\n", sessionID)
	}

	w.WriteHeader(http.StatusOK)
}
