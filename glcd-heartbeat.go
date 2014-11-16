package main

import (
       "time"
)

type HeartbeatWatcher struct {
       //hb chan Heartbeat
       glcd *GLCD
}

func (hbw *HeartbeatWatcher) HandleHeartbeatChannel() {
       for {
               hb := <-hbw.glcd.HeartbeatChan
               //fmt.Printf("HandleHeartbeatChannel: Received hb: %+v\n", heartbeat)

               hb.Timestamp = time.Now().Unix()

               // see if key and client exists in the map
               c, exists := hbw.glcd.Clients[hb.ClientId]

               if !exists {
                       c = &GLCClient{ClientId: hb.ClientId, Heartbeat: hb, Authenticated: false}
                       hbw.glcd.Clients[hb.ClientId] = c
               }

               if (!exists) || c.Heartbeat.Status != hb.Status {
                       hbw.glcd.Publish(&Message{ClientId: hb.ClientId, Type: "playerHeartbeat", Data: hb})
               }
               if hb.Status == "QUIT" {
                       delete(hbw.glcd.Clients, hb.ClientId)
               } else {
                       c.Heartbeat = hb
               }
       }
}

func (hbw *HeartbeatWatcher) Serve() {
	hbw.HandleHeartbeatChannel();
}

func (hbw *HeartbeatWatcher) Stop() {
	// Do something.
}
