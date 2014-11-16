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

func (service *PlayerAuthService) HandlePlayerAuthChannel() error {
	for {
		authInfo := <-service.glcd.AuthChan
		fmt.Printf("Received auth for user %s\n", authInfo.Name)

		_, ok := service.glcd.Clients[authInfo.Name]

		if ok {
			authed, err := service.glcd.isPasswordCorrect(authInfo.Name, authInfo.Password)

			if err != nil {
				fmt.Printf("User %s %s\n", authInfo.Name, err)
			}

			if authed {
				fmt.Printf("Auth successful for user %s\n", authInfo.Name)
				// ALLOW PLAYERS DO ANYTHING
				// UPDATE service.glcd.Clients.AUthenticated = true
				service.glcd.Clients[authInfo.Name].Authenticated = true
			} else {
				fmt.Printf("Auth failed for user %s\n", authInfo.Name)
			}
		} else {
			fmt.Printf("User %s does not exist!\n", authInfo.Name)
		}
	}
}

func (service *PlayerAuthService) Serve() {
	service.HandlePlayerAuthChannel();
}

func (service *PlayerAuthService) Stop() {
	// Do something.
}
