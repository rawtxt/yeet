package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/pion/webrtc/v4"
)

type Receiver struct {
	SessionID           SessionID
	SecretToken         string
	serverURL           string
	transferRequestChan chan TransferRequest
	pc                  *webrtc.PeerConnection
	dc                  *webrtc.DataChannel
	mu                  sync.Mutex
	hasAccepted         bool
	activeFile          *os.File
	bytesRemaining      int64
	totalBytes          int64
	doneChan            chan error

	senderRequestChan chan string
	senderAnswerChan  chan string

	localServer    *SignallingServer
	mdnsServer     *mdns.Server
	LocalServerURL string
}

func NewReceiver(serverURL string) (*Receiver, error) {
	useSTUN := true
	client := http.Client{Timeout: 200 * time.Millisecond}
	if _, err := client.Get(serverURL); err != nil {
		useSTUN = false
	}

	pc, err := webrtc.NewPeerConnection(WebRTCConfig(useSTUN))
	if err != nil {
		return nil, fmt.Errorf("NewReceiver: %w", err)
	}

	dc, err := pc.CreateDataChannel(DataChannelLabel, &webrtc.DataChannelInit{Ordered: new(true)})
	if err != nil {
		return nil, fmt.Errorf("NewReceiver: %w", err)
	}

	r := &Receiver{
		serverURL:           serverURL,
		transferRequestChan: make(chan TransferRequest, 1),
		pc:                  pc,
		dc:                  dc,
		doneChan:            make(chan error, 1),
		senderRequestChan:   make(chan string, 1),
		senderAnswerChan:    make(chan string, 1),
	}

	if err := r.setupDataChannel(); err != nil {
		return nil, fmt.Errorf("NewReceiver: %w", err)
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return nil, fmt.Errorf("NewReceiver: %w", err)
	}

	if err := pc.SetLocalDescription(offer); err != nil {
		return nil, fmt.Errorf("NewReceiver: %w", err)
	}

	select {
	case <-webrtc.GatheringCompletePromise(pc):
	case <-time.After(2 * time.Second):
	}

	localServer := NewSignallingServer()
	localServer.Silent = true
	actualAddr, err := localServer.Start("0.0.0.0:0")
	if err != nil {
		return nil, fmt.Errorf("NewReceiver: failed to start local signalling server: %w", err)
	}
	r.localServer = localServer

	_, portStr, err := net.SplitHostPort(actualAddr)
	if err != nil {
		return nil, fmt.Errorf("NewReceiver: failed to parse local server port: %w", err)
	}
	r.LocalServerURL = "http://127.0.0.1:" + portStr

	if err := r.registerSession(); err != nil {
		r.SessionID = SessionID(generateSessionID())
		r.SecretToken = generateSecretToken()
		r.serverURL = "http://127.0.0.1:" + portStr
	}

	localServer.AddSession(string(r.SessionID), r.SecretToken, r.LocalToken())

	go r.listenToEvents()

	if port, err := strconv.Atoi(portStr); err == nil {
		host, _ := os.Hostname()
		service, err := mdns.NewMDNSService(
			string(r.SessionID),
			"_yeet._tcp",
			"",
			host+".",
			port,
			nil,
			[]string{"yeet local signalling"},
		)
		if err == nil {
			mdnsServer, err := mdns.NewServer(&mdns.Config{
				Zone:   service,
				Logger: log.New(io.Discard, "", 0),
			})
			if err == nil {
				r.mdnsServer = mdnsServer
				// log.Printf("dbg: Started local mDNS server advertising %s._yeet._tcp on port %d\n", r.SessionID, port)
			} else {
				// log.Printf("dbg: Failed to start local mDNS server: %v\n", err)
			}
		}
	}

	return r, nil
}

func (r *Receiver) LocalToken() string {
	return encodeSDP(*r.pc.LocalDescription())
}

func (r *Receiver) Connect(senderToken string) error {
	return r.pc.SetRemoteDescription(decodeSDP(senderToken))
}

func (r *Receiver) Close() {
	r.mu.Lock()
	if r.activeFile != nil {
		r.activeFile.Close()
	}
	r.mu.Unlock()

	if r.dc != nil {
		r.dc.Close()
	}

	if r.pc != nil {
		r.pc.Close()
	}

	if r.mdnsServer != nil {
		r.mdnsServer.Shutdown()
	}
}

func (r *Receiver) TransferRequest() <-chan TransferRequest {
	return r.transferRequestChan
}

func (r *Receiver) Done() <-chan error {
	return r.doneChan
}

func (r *Receiver) SenderRequest() <-chan string {
	return r.senderRequestChan
}

func (r *Receiver) SenderAnswer() <-chan string {
	return r.senderAnswerChan
}

func (r *Receiver) Accept(tr TransferRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.hasAccepted {
		return fmt.Errorf("Accept: already accepted a transfer request in this session")
	}

	outName := tr.FileName + ".yeeted"
	file, err := os.Create(outName)
	if err != nil {
		return fmt.Errorf("Accept: failed to create output file: %w", err)
	}
	r.activeFile = file
	r.bytesRemaining = int64(tr.Size)
	r.totalBytes = int64(tr.Size)
	r.hasAccepted = true

	// log.Printf("Accepting transfer of %q (%d bytes) as %q\n", tr.FileName, tr.Size, outName)
	return r.dc.SendText(fmt.Sprintf("accept %q", tr.FileName))
}

func (r *Receiver) registerSession() error {
	localToken := r.LocalToken()
	reqBody, err := json.Marshal(map[string]string{
		"receiver_token": localToken,
	})
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(r.serverURL+"/register", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %s", resp.Status)
	}

	var res struct {
		SessionID   string `json:"session_id"`
		SecretToken string `json:"secret_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	r.SessionID = SessionID(res.SessionID)
	r.SecretToken = res.SecretToken
	return nil
}

func (r *Receiver) listenToEvents() {
	url := fmt.Sprintf("%s/events?session_id=%s&token=%s", r.serverURL, r.SessionID, r.SecretToken)
	resp, err := http.Get(url)
	if err != nil {
		// log.Printf("SSE: connection failed: %v\n", err)
		r.doneChan <- fmt.Errorf("SSE connection failed: %w", err)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if after, ok := strings.CutPrefix(line, "data: "); ok {
			data := after
			if data == "connected" {
				continue
			}

			if after, ok := strings.CutPrefix(data, "sender_request "); ok {
				senderName := after
				r.senderRequestChan <- senderName
			} else if after, ok := strings.CutPrefix(data, "sender_answer "); ok {
				senderToken := after
				r.senderAnswerChan <- senderToken
			}
		}
	}
	if err := scanner.Err(); err != nil {
		// log.Printf("SSE: stream read error: %v\n", err)
		r.doneChan <- fmt.Errorf("SSE stream read error: %w", err)
	}
}

func (r *Receiver) ApproveConnection() error {
	url := fmt.Sprintf("%s/approve?session_id=%s&status=accept&token=%s", r.serverURL, r.SessionID, r.SecretToken)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("approve connection failed: server status %s", resp.Status)
	}
	return nil
}

func (r *Receiver) RejectConnection() error {
	url := fmt.Sprintf("%s/approve?session_id=%s&status=reject&token=%s", r.serverURL, r.SessionID, r.SecretToken)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reject connection failed: server status %s", resp.Status)
	}
	return nil
}

func (r *Receiver) setupDataChannel() error {
	r.dc.OnOpen(func() {
		// log.Println("Data channel opened")
	})
	r.dc.OnClose(func() {
		// log.Println("Data channel closed")
	})
	r.dc.OnBufferedAmountLow(func() {
		// log.Println("Data channel buffered amount low")
	})
	r.dc.OnDial(func() {
		// log.Println("Data channel onDial")
	})

	r.dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString {
			r.mu.Lock()
			alreadyAccepted := r.hasAccepted
			r.mu.Unlock()

			if alreadyAccepted {
				// log.Printf("receiver received metadata message, but a transfer has already been accepted; ignoring subsequent request.\n")
				return
			}

			// log.Printf("receiver received metadata message\n")
			tr, err := UnmarshalTransferRequest(msg)
			if err != nil {
				// log.Printf("unmarshalling transfer request failed: %s", err)
				r.doneChan <- fmt.Errorf("unmarshalling transfer request failed: %w", err)
				return
			}

			r.transferRequestChan <- tr
		} else {
			r.mu.Lock()
			file := r.activeFile
			r.mu.Unlock()

			if file == nil {
				// log.Printf("receiver received binary data chunk but no active file download!\n")
				return
			}
			n, err := file.Write(msg.Data)
			if err != nil {
				// log.Printf("failed to write to file: %s", err)
				r.doneChan <- fmt.Errorf("failed to write to file: %w", err)
				return
			}

			r.mu.Lock()
			r.bytesRemaining -= int64(n)
			remaining := r.bytesRemaining
			total := r.totalBytes
			if remaining <= 0 {
				r.activeFile.Close()
				r.activeFile = nil
			}
			r.mu.Unlock()

			written := total - remaining
			percent := float64(written) / float64(total) * 100
			fmt.Printf("\r📥 Downloading... %.1f%% (%s / %s)", percent, formatSize(written), formatSize(total))

			if remaining <= 0 {
				fmt.Println()
				// log.Printf("Transfer complete! Received all bytes. Sending completion acknowledgment...\n")
				if err := r.dc.SendText("done"); err != nil {
					// log.Printf("Warning: failed to send completion acknowledgment: %v\n", err)
				}

				// Wait 200ms before triggering Done to ensure the "done" packet is flushed out of WebRTC network queues
				go func() {
					time.Sleep(200 * time.Millisecond)
					r.doneChan <- nil
				}()
			}
		}
	})

	r.dc.OnError(func(err error) {
		// log.Printf("Data channel error: %s", err)
	})

	return nil
}
