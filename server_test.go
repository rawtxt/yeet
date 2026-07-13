package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignallingRegister(t *testing.T) {
	server := NewSignallingServer()

	reqBody := []byte(`{"receiver_token":"test_offer_sdp"}`)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(reqBody))
	rr := httptest.NewRecorder()

	server.handleRegister(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp struct {
		SessionID   string `json:"session_id"`
		SecretToken string `json:"secret_token"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.SessionID) != 6 {
		t.Errorf("expected 6-digit session ID, got %q", resp.SessionID)
	}
	if len(resp.SecretToken) != 32 {
		t.Errorf("expected 32-character secret token, got %q", resp.SecretToken)
	}
}

func TestSignallingSecurity(t *testing.T) {
	server := NewSignallingServer()

	// 1. Register a valid session
	reqBody := []byte(`{"receiver_token":"test_offer_sdp"}`)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(reqBody))
	rr := httptest.NewRecorder()
	server.handleRegister(rr, req)

	var registered struct {
		SessionID   string `json:"session_id"`
		SecretToken string `json:"secret_token"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&registered)

	// 2. Test /events with invalid token
	req = httptest.NewRequest(http.MethodGet, "/events?session_id="+registered.SessionID+"&token=invalid_token", nil)
	rr = httptest.NewRecorder()
	server.handleEvents(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for invalid token, got %d", rr.Code)
	}

	// 3. Test /events with invalid session_id
	req = httptest.NewRequest(http.MethodGet, "/events?session_id=999999&token="+registered.SecretToken, nil)
	rr = httptest.NewRecorder()
	server.handleEvents(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for invalid session_id, got %d", rr.Code)
	}

	// 4. Test /approve with invalid token
	req = httptest.NewRequest(http.MethodPost, "/approve?session_id="+registered.SessionID+"&status=accept&token=invalid_token", nil)
	rr = httptest.NewRecorder()
	server.handleApprove(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for invalid token, got %d", rr.Code)
	}

	// 5. Test /approve with invalid session_id
	req = httptest.NewRequest(http.MethodPost, "/approve?session_id=999999&status=accept&token="+registered.SecretToken, nil)
	rr = httptest.NewRecorder()
	server.handleApprove(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for invalid session_id, got %d", rr.Code)
	}
}

func TestSignallingConnectAndApprove(t *testing.T) {
	server := NewSignallingServer()

	// Register session
	reqBody := []byte(`{"receiver_token":"test_offer"}`)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(reqBody))
	rr := httptest.NewRecorder()
	server.handleRegister(rr, req)

	var registered struct {
		SessionID   string `json:"session_id"`
		SecretToken string `json:"secret_token"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&registered)

	// We start a goroutine to connect
	connectDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		connectReqBody := []byte(`{"sender_name":"alice"}`)
		connectReq := httptest.NewRequest(http.MethodPost, "/connect?session_id="+registered.SessionID, bytes.NewReader(connectReqBody))
		connectRR := httptest.NewRecorder()
		server.handleConnect(connectRR, connectReq)
		connectDone <- connectRR
	}()

	// Read event channel to make sure "sender_request alice" event is received
	session := server.sessions[registered.SessionID]
	select {
	case event := <-session.EventChan:
		if event != "sender_request alice" {
			t.Errorf("expected event 'sender_request alice', got %q", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sender_request event")
	}

	// Approve connection
	approveReq := httptest.NewRequest(http.MethodPost, "/approve?session_id="+registered.SessionID+"&status=accept&token="+registered.SecretToken, nil)
	approveRR := httptest.NewRecorder()
	server.handleApprove(approveRR, approveReq)
	if approveRR.Code != http.StatusOK {
		t.Fatalf("expected approve status 200, got %d", approveRR.Code)
	}

	// Connect should now complete and return the receiver token!
	var connectRR *httptest.ResponseRecorder
	select {
	case connectRR = <-connectDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for connect response")
	}

	if connectRR.Code != http.StatusOK {
		t.Fatalf("expected connect status 200, got %d", connectRR.Code)
	}

	var connectResp struct {
		ReceiverToken string `json:"receiver_token"`
	}
	if err := json.NewDecoder(connectRR.Body).Decode(&connectResp); err != nil {
		t.Fatalf("failed to decode connect response: %v", err)
	}
	if connectResp.ReceiverToken != "test_offer" {
		t.Errorf("expected receiver_token 'test_offer', got %q", connectResp.ReceiverToken)
	}
}

func TestSessionExpiration(t *testing.T) {
	server := NewSignallingServer()

	// Register a session
	reqBody := []byte(`{"receiver_token":"test_offer"}`)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(reqBody))
	rr := httptest.NewRecorder()
	server.handleRegister(rr, req)

	var registered struct {
		SessionID string `json:"session_id"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&registered)

	// Verify session is active
	server.mu.Lock()
	session, exists := server.sessions[registered.SessionID]
	server.mu.Unlock()
	if !exists {
		t.Fatal("session was not registered")
	}

	// Set the expiration to the past to simulate expiration
	server.mu.Lock()
	session.ExpiresAt = time.Now().Add(-1 * time.Second)
	server.mu.Unlock()

	// Run Reap() and verify the session was cleaned up
	reapedCount := server.Reap()
	if reapedCount != 1 {
		t.Errorf("expected 1 session to be reaped, got %d", reapedCount)
	}

	// Verify session is gone from the database
	server.mu.Lock()
	_, exists = server.sessions[registered.SessionID]
	server.mu.Unlock()
	if exists {
		t.Error("expected session to be deleted after reap")
	}

	// Verify event channel was closed
	select {
	case _, ok := <-session.EventChan:
		if ok {
			t.Error("expected event channel to be closed by reaper")
		}
	default:
		t.Error("expected event channel to be closed and readable")
	}
}

func TestSignallingCapacityLimit(t *testing.T) {
	server := NewSignallingServer()
	server.MaxSessions = 2

	for i := range 2 {
		reqBody := []byte(`{"receiver_token":"token"}`)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(reqBody))
		rr := httptest.NewRecorder()
		server.handleRegister(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d on step %d", rr.Code, i)
		}
	}

	reqBody := []byte(`{"receiver_token":"token"}`)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(reqBody))
	rr := httptest.NewRecorder()
	server.handleRegister(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 Service Unavailable, got %d", rr.Code)
	}
}

func TestSanitizeSenderName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"alice", "alice"},
		{"bob@localhost", "bob@localhost"},
		{"malicious\x1b[31mname", "malicious[31mname"},
		{"佐藤", "佐藤"},
		{"Anaïs", "Anaïs"},
		{"name_with_spaces and_dots.1-2_3", "name_with_spaces and_dots.1-2_3"},
		{"", ""},
		{strings.Repeat("a", 100), strings.Repeat("a", 64)},
	}

	for _, tc := range tests {
		actual := sanitizeSenderName(tc.input)
		if actual != tc.expected {
			t.Errorf("sanitizeSenderName(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}
