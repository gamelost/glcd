package main

type PlayerState struct {
  ClientId string `json:",omitempty"`
  Name     string `json:",omitempty"`
}

type PlayerStateService struct {
  glcd *GLCD
}

func (service *PlayerStateService) HandlePlayerStateChannel() {
  for {
    ps := <-service.glcd.PlayerStateChan
    service.glcd.Publish(&Message{Type: "playerState", Data: ps})
  }
}

func (service *PlayerStateService) Serve() {
  service.HandlePlayerStateChannel()
}

func (service *PlayerStateService) Stop() {
  // Do something.
}
