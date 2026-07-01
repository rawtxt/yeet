package main

import (
	"fmt"
	"log"

	"github.com/pion/webrtc/v4"
)

type Receiver struct {
	SessionID           SessionID
	transferRequestChan chan TransferRequest
	pc                  *webrtc.PeerConnection
	dc                  *webrtc.DataChannel
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

	transferReqChan := make(chan TransferRequest, 1) // each session only supports one transfer
	if err := setupDataChannel(dc, transferReqChan); err != nil {
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

	sessionID, err := initSession(pc)
	if err != nil {
		return nil, fmt.Errorf("NewReceiver: %w", err)
	}

	return &Receiver{
		SessionID:           sessionID,
		transferRequestChan: transferReqChan,
		pc:                  pc,
		dc:                  dc,
	}, nil
}

func (r *Receiver) Close() {
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

func (r *Receiver) Accept(tr TransferRequest) error {
	return r.dc.SendText(fmt.Sprintf("accept %q", tr.FileName))
}

func initSession(pc *webrtc.PeerConnection) (SessionID, error) {
	desc := encodeSDP(*pc.LocalDescription())
	fmt.Println("receiver token:", desc)

	// TODO: get a session id from the signalling server

	return "SessionID-123", nil
}

func setupDataChannel(dc *webrtc.DataChannel, transferRefChan chan<- TransferRequest) error {
	dc.OnOpen(func() {
		log.Println("Data channel opened")
	})

	dc.OnClose(func() {
		log.Println("Data channel closed")
	})

	dc.OnBufferedAmountLow(func() {
		log.Println("Data channel buffered amount low")
	})

	dc.OnDial(func() {
		log.Println("Data channel onDial")
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Printf("receiver receive message\n")
		tr, err := UnmarshalTransferRequest(msg)
		if err != nil {
			log.Printf("unmarshalling transfer request failed: %s", err)
			return
		}

		transferRefChan <- tr
	})

	dc.OnError(func(err error) {
		log.Printf("Data channel error: %s", err)
	})

	return nil
}
