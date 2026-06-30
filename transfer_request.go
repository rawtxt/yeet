package main

import (
	"encoding/json"

	"github.com/pion/webrtc/v4"
)

type TransferRequest struct {
	FileName string `json:"filename"`
	Size     int    `json:"size"`
}

func (tr TransferRequest) Marshal() ([]byte, error) {
	return json.Marshal(tr)
}

func UnmarshalTransferRequest(msg webrtc.DataChannelMessage) (TransferRequest, error) {
	var tr TransferRequest
	err := json.Unmarshal(msg.Data, &tr)
	return tr, err
}
