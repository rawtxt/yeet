package main

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/pion/webrtc/v4"
)

const DataChannelLabel = "yeet-channel"

const YeetMatchmakerServer = "https://yeet-server.fly.dev"
const YeetSignallingServer = YeetMatchmakerServer
const DefaultReceiverPort = "8338"

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
	if strings.HasPrefix(stunURLStr, "stun:") {
		target := strings.TrimPrefix(stunURLStr, "stun:")
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
