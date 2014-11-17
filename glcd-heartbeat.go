package main

import (
  "time"
)

// Hearbeat includes Status: idle, away, typing, etc. "QUIT" is a special
// status
type Heartbeat struct {
  ClientId string
  Timestamp int64
  Status string `json:",omitempty"`
}

type HeartbeatService struct {
  glcd *GLCD
}

func (service *HeartbeatService) HandleHeartbeatChannel() {
  for {
    hb := <-service.glcd.HeartbeatChan
    //fmt.Printf("HandleHeartbeatChannel: Received hb: %+v\n", heartbeat)

    hb.Timestamp = time.Now().Unix()

    // see if key and client exists in the map
    c, exists := service.glcd.Clients[hb.ClientId]

    if !exists {
      c = &GLCClient{ClientId: hb.ClientId, Heartbeat: hb, Authenticated: false}
      service.glcd.Clients[hb.ClientId] = c
    }

    if !exists || c.Heartbeat.Status != hb.Status {
      service.glcd.Publish(&Message{ClientId: hb.ClientId, Type: "playerHeartbeat", Data: hb})
    }

    if hb.Status == "QUIT" {
      delete(service.glcd.Clients, hb.ClientId)
    } else {
      c.Heartbeat = hb
    }
  }
}

func (service *HeartbeatService) Serve() {
  service.HandleHeartbeatChannel();
}

func (service *HeartbeatService) Stop() {
  // Do something.
}
