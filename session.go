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

func encodeSDP(desc webrtc.SessionDescription) string {
	b, err := json.Marshal(desc)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(b); err != nil {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func decodeSDP(str string) webrtc.SessionDescription {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(str))
	if err != nil {
		panic(err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	defer gz.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, gz); err != nil {
		panic(err)
	}

	var desc webrtc.SessionDescription
	err = json.Unmarshal(buf.Bytes(), &desc)
	if err != nil {
		panic(err)
	}

	return desc
}
