package main

import (
	"net"

	"github.com/pion/webrtc/v4"
)

const DataChannelLabel = "yeet-channel"

const YeetSignallingServer = "https://yeet-signalling.fly.dev/"

func WebRTCConfig(useSTUN bool) webrtc.Configuration {
	if !useSTUN {
		return webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{},
		}
	}
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			webrtc.ICEServer{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
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
