package main

type BroadcastService struct {
  glcd *GLCD
}

func (service *BroadcastService) HandleBroadcastChannel() error {
  for {
    msg := <-service.glcd.BroadcastChan
    service.glcd.Publish(msg)
  }
}

func (service *BroadcastService) Serve() {
  service.HandleBroadcastChannel()
}

func (service *BroadcastService) Stop() {
  // Do something.
}
