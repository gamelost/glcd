package main

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

/* Players coming in and out */
type PlayerPassport struct {
  Action string
}

type ErrorMessage string

type ChatMessage struct {
  Sender  string
  Message string
}
