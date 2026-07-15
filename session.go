package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	mrand "math/rand/v2"
	"strings"
	"time"

	"github.com/pion/webrtc/v4"
)

type SessionID string

type Session struct {
	ID            SessionID
	SecretToken   string
	ReceiverToken string
	EventChan     chan string
	ApprovedChan  chan bool
	ExpiresAt     time.Time
}

var sessionWords = []string{
	"acid", "actor", "acute", "adapt", "admit", "agony", "ahead", "alert", "alike", "alive",
	"allow", "alone", "along", "alpha", "angry", "ankle", "arena", "argue", "arrow", "asset",
	"atlas", "audio", "audit", "avoid", "awake", "aware", "bacon", "badge", "baker", "basic",
	"basil", "basin", "beach", "beard", "beast", "begin", "bench", "berry", "bingo", "birch",
	"birth", "bison", "black", "blade", "blame", "blank", "blast", "blend", "blind", "block",
	"blond", "blood", "bloom", "board", "boast", "bonus", "boost", "bound", "brain", "brass",
	"brave", "bread", "break", "brick", "bride", "brief", "broad", "broke", "brown", "brush",
	"buddy", "build", "bunch", "buyer", "cabin", "cable", "camel", "candy", "cargo", "carve",
	"catch", "cedar", "chain", "chair", "chalk", "champ", "chaos", "charm", "chart", "chase",
	"cheap", "check", "cheek", "cheer", "chess", "chest", "chief", "child", "chili", "chill",
	"chime", "choir", "cigar", "civic", "claim", "clamp", "clank", "clash", "clasp", "class",
	"clean", "clear", "clerk", "click", "cliff", "climb", "cling", "clock", "close", "cloth",
	"cloud", "clown", "clump", "coach", "coast", "cocoa", "color", "comet", "coral", "couch",
	"cough", "count", "court", "cover", "crack", "craft", "crane", "crank", "crash", "crawl",
	"crazy", "cream", "creek", "crest", "crisp", "cross", "crowd", "crown", "crude", "cruel",
	"crush", "crust", "cubic", "curly", "cycle", "daily", "dairy", "daisy", "dance", "denim",
	"depot", "depth", "derby", "devil", "diary", "digit", "diner", "dirty", "disco", "ditch",
	"diver", "dizzy", "dodge", "donor", "donut", "draft", "drain", "drama", "drawl", "dream",
	"dress", "dried", "drift", "drill", "drink", "drive", "droll", "drone", "drool", "drove",
	"drown", "drum", "dryer", "ducky", "dusty", "dwarf", "dwell", "eager", "eagle", "early",
	"earth", "easel", "echo", "elbow", "elder", "elect", "elite", "ember", "empty", "enemy",
	"enjoy", "enter", "entry", "envoy", "epoch", "equal", "error", "erupt", "essay", "ethic",
	"event", "every", "evict", "exact", "excel", "exert", "exile", "exist", "extra", "fable",
	"facet", "fairy", "faith", "false", "fancy", "fatal", "fault", "favor", "feast", "fiber",
	"field", "fiery", "fifth", "fifty", "fight", "final", "first", "fixed", "flame", "flash",
	"flask", "flat", "fleet", "flesh", "flight", "flint",
}

func generateSessionID() SessionID {
	word1 := sessionWords[mrand.IntN(256)]
	word2 := sessionWords[mrand.IntN(256)]
	word3 := sessionWords[mrand.IntN(256)]
	return SessionID(fmt.Sprintf("%s-%s-%s", word1, word2, word3))
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
