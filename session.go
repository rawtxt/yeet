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
	"about", "above", "actor", "acute", "adapt", "admit", "adopt", "agent",
	"agree", "ahead", "alarm", "album", "alert", "alike", "alive", "allow",
	"alone", "along", "alpha", "alter", "among", "anger", "angle", "angry",
	"apart", "apple", "apply", "arena", "argue", "arise", "array", "arrow",
	"aside", "asset", "audio", "audit", "avoid", "award", "aware", "awful",
	"bacon", "badge", "baker", "basic", "basin", "basis", "basket", "beach",
	"beard", "beast", "begin", "bench", "berry", "bible", "birth", "black",
	"blade", "blame", "blank", "blast", "blend", "blind", "blink", "block",
	"blood", "board", "boast", "bonus", "boost", "bound", "brain", "brand",
	"brave", "bravo", "bread", "break", "breed", "brick", "bride", "brief",
	"bring", "broad", "broke", "brown", "brush", "buyer", "cabin", "cable",
	"camel", "camera", "candy", "cargo", "carve", "casey", "castle", "catch",
	"cater", "cause", "chain", "chair", "chalk", "chaos", "charlie", "charm",
	"chart", "chase", "cheap", "check", "cheek", "cheer", "chef", "chest",
	"chief", "child", "chime", "china", "chips", "choir", "chunk", "cigar",
	"circus", "civet", "claim", "clash", "clasp", "class", "clean", "clear",
	"clerk", "click", "cliff", "climb", "clock", "close", "cloth", "cloud",
	"clown", "coach", "coast", "cobra", "cocoa", "color", "comet", "comic",
	"coral", "couch", "cough", "count", "court", "cover", "craft", "crane",
	"crash", "crate", "crawl", "crazy", "cream", "creed", "creek", "crest",
	"cried", "crime", "crisp", "crook", "cross", "crowd", "crown", "crude",
	"cruel", "crush", "crust", "crypt", "cubic", "curry", "curve", "cycle",
	"daily", "dairy", "daisy", "dance", "danger", "decor", "delay", "delta",
	"demon", "dense", "depth", "derby", "diary", "digit", "dirty", "ditch",
	"diver", "divot", "donor", "donut", "doubt", "dough", "draft", "drama",
	"drank", "drawl", "dream", "dress", "dried", "drift", "drill", "drink",
	"drive", "drone", "droop", "drown", "druid", "drunk", "dryer", "dwarf",
	"dwell", "eagle", "early", "earth", "easel", "ebony", "echo", "elbow",
	"elder", "elect", "elite", "elope", "elude", "email", "ember", "empty",
	"enact", "endow", "enjoy", "enter", "entry", "envoy", "epoch", "equal",
	"equip", "erase", "error", "erupt", "essay", "ether", "foxtrot", "golf",
	"hotel", "india", "juliet", "kilo", "lima", "mike", "november", "oscar",
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
