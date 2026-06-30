package main

import "github.com/pion/webrtc/v4"

const DataChannelLabel = "yeet-channel"

func WebRTCConfig() webrtc.Configuration {
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			webrtc.ICEServer{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
}
