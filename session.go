package main

import (
	"fmt"
	mrand "math/rand/v2"
	"time"
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
	n := len(sessionWords)
	word1 := sessionWords[mrand.IntN(n)]
	word2 := sessionWords[mrand.IntN(n)]
	word3 := sessionWords[mrand.IntN(n)]
	return SessionID(fmt.Sprintf("%s-%s-%s", word1, word2, word3))
}
