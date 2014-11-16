package main

type HandleBroadcastService struct {
  glcd *GLCD
}

func (service *HandleBroadcastService) HandleBroadcastChannel() error {
	for {
		msg := <-service.glcd.BroadcastChan
		service.glcd.Publish(msg)
	}
}

func (service *HandleBroadcastService) Serve() {
	service.HandleBroadcastChannel();
}

func (service *HandleBroadcastService) Stop() {
	// Do something.
}
