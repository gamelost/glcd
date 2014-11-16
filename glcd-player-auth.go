package main

import (
  "fmt"
)

type PlayerAuthInfo struct {
	Name     string `bson:"user"`
	Password string `bson:"password"`
}

type PlayerAuthService struct {
  glcd *GLCD
}

func (pas *PlayerAuthService) HandlePlayerAuthChannel() error {
	for {
		authInfo := <-pas.glcd.AuthChan
		fmt.Printf("Received auth for user %s\n", authInfo.Name)

		_, ok := pas.glcd.Clients[authInfo.Name]

		if ok {
			authed, err := pas.glcd.isPasswordCorrect(authInfo.Name, authInfo.Password)

			if err != nil {
				fmt.Printf("User %s %s\n", authInfo.Name, err)
			}

			if authed {
				fmt.Printf("Auth successful for user %s\n", authInfo.Name)
				// ALLOW PLAYERS DO ANYTHING
				// UPDATE pas.glcd.Clients.AUthenticated = true
				pas.glcd.Clients[authInfo.Name].Authenticated = true
			} else {
				fmt.Printf("Auth failed for user %s\n", authInfo.Name)
			}
		} else {
			fmt.Printf("User %s does not exist!\n", authInfo.Name)
		}
	}
}

func (pas *PlayerAuthService) Serve() {
	pas.HandlePlayerAuthChannel();
}

func (pas *PlayerAuthService) Stop() {
	// Do something.
}
