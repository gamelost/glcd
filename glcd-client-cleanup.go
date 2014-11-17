package main

import (
  "fmt"
  "time"
)

type GLCClient struct {
	Name          string
	ClientId      string
	Authenticated bool
	State         *PlayerState
	Heartbeat     *Heartbeat
}

type ClientCleanupService struct {
  glcd *GLCD
}

func (service *ClientCleanupService) CleanupClients() error {
	for {
		exp := time.Now().Unix()
		<-time.After(time.Second * 10)
		//fmt.Println("Doing client clean up")
		// Expire any clients who haven't sent a heartbeat in the last 10 seconds.
		for k, v := range service.glcd.Clients {
			if v.Heartbeat.Timestamp < exp {
				fmt.Printf("Deleting client %s due to inactivity.\n", v.ClientId)
				delete(service.glcd.Clients, k)
				v.Heartbeat.Status = "QUIT"
				service.glcd.Publish(&Message{ClientId: v.ClientId, Type: "playerHeartbeat", Data: v.Heartbeat})
			}
		}
	}
}

func (service *ClientCleanupService) Serve() {
	service.CleanupClients();
}

func (service *ClientCleanupService) Stop() {
	// Do something.
}
