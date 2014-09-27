package main

import (
	"time"
)

type Message struct {
	ClientId string
	Type     string // better way to persist type info?
	Data     interface{}
}

type ZoneInfo struct {
	x int
	y int
}

type Zone struct {
	Id    int
	Name  string
	State *ZoneInfo
}

type PlayerInfo struct {
	Name     string
	ClientId string
}

type Players []PlayerInfo

type PlayerState struct {
	ClientId    string
	Name        string `json:",omitempty"`
	X           float64
	Y           float64
	AvatarId    string `json:",omitempty"`
	AvatarState int64  `json:",omitempty"`
}

type PlayerAuthInfo struct {
	Name     string `bson:"user"`
	Password string `bson:"password"`
}

// Hearbeat includes Status: idle, away, typing, etc. "QUIT" is a special
// status
type Heartbeat struct {
	ClientId  string
	Timestamp time.Time
	Status    string `json:",omitempty"`
}

/* Players coming in and out */
type PlayerPassport struct {
	Action string
	Avatar string
}

type ErrorMessage string

type ChatMessage struct {
	Sender  string
	Message string
}
