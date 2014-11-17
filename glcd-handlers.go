package main

import (
  "fmt"
  ms "github.com/mitchellh/mapstructure"
  "time"
)

func (glcd *GLCD) HandleHeartbeat(msg *Message, dataMap map[string]interface{}) {
  var hb Heartbeat

  err := ms.Decode(dataMap, &hb)
  if err != nil {
    fmt.Println(err.Error())
    // Old style client.
    hb.ClientId = msg.ClientId
    hb.Timestamp = time.Now().Unix()
    hb.Status = "ACTIVE"
  }
  fmt.Println("Got a heartbeat.")
  glcd.HeartbeatChan <- &hb
}

func (glcd *GLCD) HandleBroadcast(msg *Message, dataMap map[string]interface{}) {
  glcd.BroadcastChan <- msg
}

func (glcd *GLCD) HandlePlayerAuth(msg *Message, dataMap map[string]interface{}) {
  var pai PlayerAuthInfo
  err := ms.Decode(dataMap, &pai)
  if err != nil {
    fmt.Println(err.Error())
  } else {
    glcd.AuthChan <- &pai
  }
}

func (glcd *GLCD) HandleChat(msg *Message, data interface{}) {
  glcd.Publish(msg)
}

func (glcd *GLCD) HandlePlayerState(msg *Message, dataMap map[string]interface{}) {

  // make sure ClientId and Name are extant
  var ps PlayerState
  err := ms.Decode(dataMap, &ps)
  if err != nil {
    fmt.Println(err.Error())
  } else {
    ps.ClientId = msg.ClientId
    glcd.PlayerStateChan <- msg
  }
}

func (glcd *GLCD) HandleConnected(msg *Message, dataMap map[string]interface{}) {
  fmt.Println("Received connected from client")
  glcd.SendZones()
}
