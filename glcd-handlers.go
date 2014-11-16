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

	var ps PlayerState
	err := ms.Decode(dataMap, &ps)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		ps.ClientId = msg.ClientId
		//fmt.Printf("Player state: %+v\n", ps)
		glcd.PlayerStateChan <- &ps
	}
}

func (glcd *GLCD) HandleConnected(msg *Message, dataMap map[string]interface{}) {
	fmt.Println("Received connected from client")
	glcd.SendZones()
}

func (glcd *GLCD) HandlePlayerStateChannel() {
	for {
		ps := <-glcd.PlayerStateChan
		glcd.Publish(&Message{Type: "playerState", Data: ps})
	}
}

func (glcd *GLCD) HandleBroadcastChannel() error {
	for {
		msg := <-glcd.BroadcastChan
		glcd.Publish(msg)
	}
}

func (glcd *GLCD) HandlePlayerAuthChannel() error {
	for {
		authInfo := <-glcd.AuthChan
		fmt.Printf("Received auth for user %s\n", authInfo.Name)

		_, ok := glcd.Clients[authInfo.Name]

		if ok {
			authed, err := glcd.isPasswordCorrect(authInfo.Name, authInfo.Password)

			if err != nil {
				fmt.Printf("User %s %s\n", authInfo.Name, err)
			}

			if authed {
				fmt.Printf("Auth successful for user %s\n", authInfo.Name)
				// ALLOW PLAYERS DO ANYTHING
				// UPDATE glcd.Clients.AUthenticated = true
				glcd.Clients[authInfo.Name].Authenticated = true
			} else {
				fmt.Printf("Auth failed for user %s\n", authInfo.Name)
			}
		} else {
			fmt.Printf("User %s does not exist!\n", authInfo.Name)
		}
	}
}
