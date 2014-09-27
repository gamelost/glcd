package main

import (
	"fmt"
	ms "github.com/mitchellh/mapstructure"
	"time"
)

func (glcd *GLCD) HandleHeartbeat(msg *Message, dataMap map[string]interface{}) {
	hb := &Heartbeat{}
	hb.ClientId = msg.ClientId
	glcd.HeartbeatChan <- hb
}

func (glcd *GLCD) HandleKnock(msg *Message, dataMap map[string]interface{}) {
	glcd.KnockChan <- glcd.Clients[msg.ClientId]
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

func (glcd *GLCD) HandleHeartbeatChannel() {
	for {
		hb := <-glcd.HeartbeatChan
		//fmt.Printf("HandleHeartbeatChannel: Received heartbeat: %+v\n", hb)

		hb.Timestamp = time.Now()

		// see if key and client exists in the map
		c, exists := glcd.Clients[hb.ClientId]

		if !exists {
			c := &GLCClient{ClientId: hb.ClientId, Heartbeat: hb, Authenticated: false}
			glcd.Clients[hb.ClientId] = c
		}

		if (!exists) || c.Heartbeat.Status != hb.Status {
			glcd.Publish(&Message{ClientId: c.ClientId, Type: "playerHeartbeat", Data: hb})
		}
		if hb.Status == "QUIT" {
			if exists {
				delete(glcd.Clients, hb.ClientId)
			}
			// If it doesn't exist, then there's nothing to clear?
		} else {
			c.Heartbeat = hb
		}
	}
}

func (glcd *GLCD) HandleKnockChannel() error {
	for {
		client := <-glcd.KnockChan
		fmt.Printf("Received knock from %s\n", client.ClientId)
		players := make(Players, len(glcd.Clients))

		i := 0
		for _, c := range glcd.Clients {
			players[i] = PlayerInfo{ClientId: c.ClientId}
			i++
		}

		glcd.Publish(&Message{ClientId: client.ClientId, Type: "knock", Data: players})
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
