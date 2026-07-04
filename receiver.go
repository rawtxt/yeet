package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/pion/webrtc/v4"
)

type Receiver struct {
	SessionID           SessionID
	transferRequestChan chan TransferRequest
	pc                  *webrtc.PeerConnection
	dc                  *webrtc.DataChannel
	mu                  sync.Mutex
	hasAccepted         bool
	activeFile          *os.File
	bytesRemaining      int64
	doneChan            chan error
}

func NewReceiver() (*Receiver, error) {
	pc, err := webrtc.NewPeerConnection(WebRTCConfig())
	if err != nil {
		return nil, fmt.Errorf("NewReceiver: %w", err)
	}

	dc, err := pc.CreateDataChannel(DataChannelLabel, &webrtc.DataChannelInit{Ordered: new(true)})
	if err != nil {
		return nil, fmt.Errorf("NewReceiver: %w", err)
	}

	r := &Receiver{
		SessionID:           "SessionID-123",
		transferRequestChan: make(chan TransferRequest, 1),
		pc:                  pc,
		dc:                  dc,
		doneChan:            make(chan error, 1),
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

	<-webrtc.GatheringCompletePromise(pc)

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
}

func (r *Receiver) TransferRequest() <-chan TransferRequest {
	return r.transferRequestChan
}

func (r *Receiver) Done() <-chan error {
	return r.doneChan
}

func (r *Receiver) Accept(tr TransferRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.hasAccepted {
		return fmt.Errorf("Accept: already accepted a transfer request in this session")
	}

	outName := tr.FileName + ".download"
	file, err := os.Create(outName)
	if err != nil {
		return fmt.Errorf("Accept: failed to create output file: %w", err)
	}
	r.activeFile = file
	r.bytesRemaining = int64(tr.Size)
	r.hasAccepted = true

	log.Printf("Accepting transfer of %q (%d bytes) as %q\n", tr.FileName, tr.Size, outName)
	return r.dc.SendText(fmt.Sprintf("accept %q", tr.FileName))
}

func (r *Receiver) setupDataChannel() error {
	r.dc.OnOpen(func() {
		log.Println("Data channel opened")
	})

	r.dc.OnClose(func() {
		log.Println("Data channel closed")
	})

	r.dc.OnBufferedAmountLow(func() {
		log.Println("Data channel buffered amount low")
	})

	r.dc.OnDial(func() {
		log.Println("Data channel onDial")
	})

	r.dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString {
			r.mu.Lock()
			alreadyAccepted := r.hasAccepted
			r.mu.Unlock()

			if alreadyAccepted {
				log.Printf("receiver received metadata message, but a transfer has already been accepted; ignoring subsequent request.\n")
				return
			}

			log.Printf("receiver received metadata message\n")
			tr, err := UnmarshalTransferRequest(msg)
			if err != nil {
				log.Printf("unmarshalling transfer request failed: %s", err)
				return
			}

			r.transferRequestChan <- tr
		} else {
			r.mu.Lock()
			file := r.activeFile
			r.mu.Unlock()

			if file == nil {
				log.Printf("receiver received binary data chunk but no active file download!\n")
				return
			}
			n, err := file.Write(msg.Data)
			if err != nil {
				log.Printf("failed to write to file: %s", err)
				r.doneChan <- err
				return
			}

			r.mu.Lock()
			r.bytesRemaining -= int64(n)
			remaining := r.bytesRemaining
			if remaining <= 0 {
				r.activeFile.Close()
				r.activeFile = nil
			}
			r.mu.Unlock()

			if remaining <= 0 {
				log.Printf("Transfer complete! Received all bytes.\n")
				r.doneChan <- nil
			}
		}
	})

	r.dc.OnError(func(err error) {
		log.Printf("Data channel error: %s", err)
	})

	return nil
}
