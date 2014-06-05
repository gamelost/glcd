package main

import (
	iniconf "code.google.com/p/goconf/conf"
	"encoding/json"
	"fmt"
	// "github.com/gamelost/bot3server/server"
	nsq "github.com/gamelost/go-nsq"
	// irc "github.com/gamelost/goirc/client"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	GLCD_CONFIG = "glcd.config"
)

type Message map[string]interface{}

func main() {
	// the quit channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// read in necessary configuration
	configFile, err := iniconf.ReadConfigFile(GLCD_CONFIG)
	if err != nil {
		log.Fatal("Unable to read configuration file. Exiting now.")
	}

	glcd := &GLCD{}
	glcd.QuitChan = sigChan
	glcd.init(configFile)

	// receiving quit shuts down
	<-sigChan
}

type GLCClient struct {
	Name		string
	Topic		string
	Writer		*nsq.Writer
	Heartbeat	time.Time
	Clientid	string
}

// struct type for Bot3
type GLCD struct {
	Online			 bool
	ConfigFile		 *iniconf.ConfigFile

	// NSQ input/output to bot3Server
	GLCInput		 *nsq.Reader
	Clients			 map[string] *GLCClient

	QuitChan                 chan os.Signal
}

func (glcd *GLCD) init(conf *iniconf.ConfigFile) error {

	glcd.ConfigFile = conf
	glcd.Online = false

	lookupdAddress, _ := conf.GetString("nsq", "lookupd-address")
	nsqdAddress, _ := conf.GetString("nsq", "nsqd-address")
	serverTopic, _ := conf.GetString("nsq", "server-topic")
	serverChannel, _ := conf.GetString("nsq", "server-channel")

	// Create the channel, by connecting to lookupd. (TODO; if it doesn't
	// exist. Also do it the right way with a Register command?)
	writer := nsq.NewWriter(nsqdAddress)
	writer.Publish(serverTopic, []byte("{\"client\":\"server\"}"))

	// set up listener for heartbeat from bot3server
	reader, err := nsq.NewReader(serverTopic, serverChannel)
	if err != nil {
		panic(err)
		glcd.QuitChan <- syscall.SIGINT
	}
	glcd.GLCInput = reader

	glcd.GLCInput.AddHandler(glcd)
	glcd.GLCInput.ConnectToLookupd(lookupdAddress)

	// Spawn goroutine to clear out clients who don't send hearbeats
	// anymore.
	go glcd.CleanupClients()

	// hardcoded kill command just in case

	return nil
}

func (cl *GLCClient) Publish(msg *Message) {
	if cl.Writer == nil {
		args := strings.SplitN(cl.Clientid, ":", 3)
		host, port, topic := args[0], args[1], args[2]
		cl.Writer = nsq.NewWriter(host + ":" + port)
		cl.Topic = topic
	}
	encodedRequest, _ := json.Marshal(*msg)
	cl.Writer.Publish(cl.Topic, encodedRequest)
}

func (glcd *GLCD) CleanupClients() error {
	for {
		exp := time.Now().Unix()
		<-time.After(time.Second * 10)
		// Expire any clients who haven't sent a heartbeat in the last 10 seconds.
		for k, v := range glcd.Clients {
			if v.Heartbeat.Unix() < exp {
				delete(glcd.Clients, k)
			}
		}
	}
}

// Coming up next. COFFEE?
func (glcd *GLCD) HandleMessage(message *nsq.Message) error {
	msg := Message{}

	err := json.Unmarshal(message.Body, &msg)

	if err != nil { return fmt.Errorf("Not a JSON interface") }

	// If "client" is not in the JSON received, dump it.
	cdata, ok := msg["client"]
	if !ok { return fmt.Errorf("No client provided") }

	// If "client" from JSON is 
	clientid, ok := cdata.(string)
	if !ok { return fmt.Errorf("Invalid format: client is not a string.") }

	// Ignore our silly "create a topic" message.
	if clientid == "server" {
		return nil
	}

	// Make sure client exists in glcd.Clients
	cl, exists := glcd.Clients[clientid];
	if !exists {
		cl = &GLCClient{}
		cl.Clientid = clientid
		glcd.Clients[clientid] = cl
		cl.Heartbeat = time.Now()
	}

	// Now perform the client's action.
	cmddata, ok := msg["command"]
	if !ok {
		// Lacking a command is okay - It's a heartbeat.
		return nil
	}

	// If "client" from JSON is 
	command, ok := cmddata.(string)
	if !ok { return fmt.Errorf("Invalid format: command is not a string.") }

	switch command {
	case "ping":
		cl.Publish(&Message{"pong": fmt.Sprint(time.Now())})
		break
	case "wall":
		for _, v := range(glcd.Clients) {
			v.Publish(&msg)
		}
	}
	return nil
}
