package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"

	"github.com/pion/webrtc/v4"
)

type SessionID string

func encodeSDP(desc webrtc.SessionDescription) (string, error) {
	b, err := json.Marshal(desc)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(b); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func decodeSDP(str string) (webrtc.SessionDescription, error) {
	var desc webrtc.SessionDescription
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(str))
	if err != nil {
		return desc, err
	}

	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return desc, err
	}
	defer gz.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, gz); err != nil {
		return desc, err
	}

	err = json.Unmarshal(buf.Bytes(), &desc)
	if err != nil {
		return desc, err
	}

	return desc, nil
}
