package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/pion/webrtc/v4"
)

// TransferRequest represents metadata sent over the WebRTC data channel prior to file payload.
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

const DataChannelLabel = "yeet-channel"

const YeetMatchmakerServer = "https://yeet-server.fly.dev"
const YeetSignallingServer = YeetMatchmakerServer
const DefaultReceiverPort = "8338"

type ProgressFunc func(fileName string, currentBytes, totalBytes int64)

const (
	EventConnected           = "connected"
	EventSenderRequestPrefix = "sender_request "
	EventSenderAnswerPrefix  = "sender_answer "
	ControlDone              = "done"
)

func FormatReceiverURL(receiverIP string) string {
	receiverIP = strings.TrimSpace(receiverIP)
	if strings.HasPrefix(receiverIP, "http://") || strings.HasPrefix(receiverIP, "https://") {
		return receiverIP
	}
	host, port, err := net.SplitHostPort(receiverIP)
	if err == nil && host != "" && port != "" {
		return "http://" + receiverIP
	}
	return "http://" + net.JoinHostPort(receiverIP, DefaultReceiverPort)
}

func WebRTCConfig(useSTUN bool, stunURLs ...string) webrtc.Configuration {
	if !useSTUN {
		return webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{},
		}
	}
	var validURLs []string
	for _, u := range stunURLs {
		if u != "" {
			validURLs = append(validURLs, u)
		}
	}
	if len(validURLs) == 0 {
		validURLs = []string{"stun:stun.l.google.com:19302"}
	}
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: validURLs},
		},
	}
}

func resolveStunURL(stunURLStr, serverURL string) string {
	if stunURLStr == "" {
		return ""
	}
	if after, ok := strings.CutPrefix(stunURLStr, "stun:"); ok {
		target := after
		if !strings.Contains(target, ":") {
			u, err := url.Parse(serverURL)
			if err == nil {
				host := u.Hostname()
				if host != "" {
					return fmt.Sprintf("stun:%s:%s", host, target)
				}
			}
		}
	}
	return stunURLStr
}

func GetLocalIP() (string, error) {
	conn, err := net.Dial("udp", "1.1.1.1:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String(), nil
	}

	conn, err = net.Dial("udp", "224.0.0.1:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String(), nil
	}

	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String(), nil
				}
			}
		}
	}

	return "127.0.0.1", nil
}
