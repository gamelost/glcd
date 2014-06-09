package main

import (
	iniconf "code.google.com/p/goconf/conf"
	"encoding/json"
	"fmt"
	// "github.com/gamelost/bot3server/server"
	nsq "github.com/gamelost/go-nsq"
	// irc "github.com/gamelost/goirc/client"
	ms "github.com/mitchellh/mapstructure"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	GLCD_CONFIG = "glcd.config"
)

var gamestateTopic = ""

type Message struct {
	ClientId   string
	Type       string // better way to persist type info?
	Data       interface{}
}

type ZoneInfo struct {
	x int
	y int
}

type Zone struct {
	Id    int
	Name  string
	State *ZoneInfo
}

type PlayerInfo struct {
	Name     string
	ClientId string
}

type Players []PlayerInfo

type PlayerState struct {
	ClientId string
	X        float64
	Y        float64
	AvatarId string `json:",omitempty"`
}

type Heartbeat struct {
	ClientId  string
	Timestamp time.Time
}

/* Players coming in and out */
type PlayerPassport struct {
	Action string
	Avatar string
}

type ErrorMessage string

type ChatMessage struct {
	Sender string
	Message string
}

func main() {
	// the quit channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// read in necessary configuration
	configFile, err := iniconf.ReadConfigFile(GLCD_CONFIG)
	if err != nil {
		log.Fatal("Unable to read configuration file. Exiting now.")
	}

	glcd := &GLCD{QuitChan: sigChan}
	glcd.init(configFile)

	// receiving quit shuts down
	<-glcd.QuitChan
}

type GLCClient struct {
	ClientId  string
	State     *PlayerState
	Heartbeat time.Time
}

// struct type for Bot3
type GLCD struct {
	Online     bool
	ConfigFile *iniconf.ConfigFile

	// NSQ input/output
	NSQWriter             *nsq.Writer
	GLCDaemonTopic        *nsq.Reader
	GLCGameStateTopicName string
	GLCDaemonTopicChannel string
	Clients               map[string]*GLCClient

	// game state channels
	HeartbeatChan   chan *Heartbeat
	KnockChan       chan *GLCClient
	PlayerStateChan chan *PlayerState

	QuitChan chan os.Signal

	MongoSession *mgo.Session
	MongoDB      *mgo.Database
}

func (glcd *GLCD) init(conf *iniconf.ConfigFile) error {

	glcd.ConfigFile = conf
	glcd.Online = false

	glcd.Clients = map[string]*GLCClient{}

	// Connect to Mongo.
	servers, err := glcd.ConfigFile.GetString("mongo", "servers")

	if err != nil {
		return fmt.Errorf("Mongo: No server configured.")
	}

	glcd.MongoSession, err = mgo.Dial(servers)

	if err != nil {
	}

	db, err := glcd.ConfigFile.GetString("mongo", "db")

	if err != nil {
		return fmt.Errorf("Mongo: No database configured.")
	} else {
		fmt.Println("Successfully obtained config from mongo")
	}

	glcd.MongoDB = glcd.MongoSession.DB(db)

	// set up channels
	glcd.HeartbeatChan = make(chan *Heartbeat)
	glcd.KnockChan = make(chan *GLCClient)
	glcd.PlayerStateChan = make(chan *PlayerState)

	nsqdAddress, _ := conf.GetString("nsq", "nsqd-address")
	lookupdAddress, _ := conf.GetString("nsq", "lookupd-address")
	glcd.GLCGameStateTopicName, _ = conf.GetString("nsq", "server-topic")

	glcdTopic, _ := conf.GetString("nsq", "glcd-topic")

	// Create the channel, by connecting to lookupd. (TODO; if it doesn't
	// exist. Also do it the right way with a Register command?)
	glcd.NSQWriter = nsq.NewWriter(nsqdAddress)
	glcd.NSQWriter.Publish(glcd.GLCGameStateTopicName, []byte("{\"client\":\"server\"}"))

	// set up reader for glcdTopic
	reader, err := nsq.NewReader(glcdTopic, "main")
	if err != nil {
		glcd.QuitChan <- syscall.SIGINT
	}
	glcd.GLCDaemonTopic = reader
	glcd.GLCDaemonTopic.AddHandler(glcd)
	glcd.GLCDaemonTopic.ConnectToLookupd(lookupdAddress)

	// goroutines to handle concurrent events
	go glcd.CleanupClients()
	go glcd.HandleHeartbeatChannel()
	go glcd.HandleKnockChannel()
	go glcd.HandlePlayerStateChannel()

	return nil
}

func (glcd *GLCD) Publish(msg *Message) {
	encodedRequest, _ := json.Marshal(*msg)
	glcd.NSQWriter.Publish(glcd.GLCGameStateTopicName, encodedRequest)
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

		// see if key and client exists in the map
		c, exists := glcd.Clients[hb.ClientId]

		if exists {
			//fmt.Printf("Client %s exists.  Updating heartbeat.\n", hb.ClientId)
			c.Heartbeat = time.Now()
		} else {
			//fmt.Printf("Adding client %s to client list\n", hb.ClientId)
			client := &GLCClient{ClientId: hb.ClientId, Heartbeat: time.Now()}
			glcd.Clients[hb.ClientId] = client
		}
	}
}

func (glcd *GLCD) HandleKnockChannel() error {
	for {
		client := <-glcd.KnockChan
		fmt.Printf("Received knock from %s", client.ClientId)
		players := make(Players, len(glcd.Clients))

		i := 0
		for _, c := range glcd.Clients {
			players[i] = PlayerInfo{ClientId: c.ClientId}
			i++
		}

		glcd.Publish(&Message{ClientId: client.ClientId, Type: "knock", Data: players})
	}
}

func (glcd *GLCD) CleanupClients() error {
	for {
		exp := time.Now().Unix()
		<-time.After(time.Second * 10)
		//fmt.Println("Doing client clean up")
		// Expire any clients who haven't sent a heartbeat in the last 10 seconds.
		for k, v := range glcd.Clients {
			if v.Heartbeat.Unix() < exp {
				fmt.Printf("Deleting client %s due to inactivity.\n", v.ClientId)
				delete(glcd.Clients, k)
				//glcd.Publish(&Message{Type: "playerPassport", Data: PlayerPassport{Action: "playerGone"}}) // somehow add k to this
			} else {
				//fmt.Printf("Client has not expired.")
			}
		}
	}
}

// Send a zone file update.
func (glcd *GLCD) SendZones() {
	fmt.Println("SendZones --")
	c := glcd.MongoDB.C("zones")
	q := c.Find(nil)

	if q == nil {
		glcd.Publish(&Message{Type: "error", Data: fmt.Sprintf("No zones found")})
	} else {
		fmt.Println("Publishing zones to clients")
		var results []interface{}
		err := q.All(&results)
		if err == nil {
			for _, res := range results {
				fmt.Printf("Res: is %+v", res)
				glcd.Publish(&Message{Type: "updateZone", Data: res.(bson.M)}) // dump res as a JSON string
			}
		} else {
			glcd.Publish(&Message{Type: "error", Data: fmt.Sprintf("Unable to fetch zones: %v", err)})
		}
	}
}

func (glcd *GLCD) HandleChatMessage(msg *Message, data interface{}) {
	glcd.Publish(msg)
}

func (glcd *GLCD) SendZone(zone *Zone) {
	c := glcd.MongoDB.C("zones")
	query := bson.M{"zone": zone.Name}
	results := c.Find(query)

	if results == nil {
		glcd.Publish(&Message{Type: "error", Data: fmt.Sprintf("No such zone '%s'", zone.Name)})
	} else {
		var res interface{}
		err := results.One(&res)
		if err == nil {
			glcd.Publish(&Message{Type: "zone", Data: res.(string)})
		} else {
			glcd.Publish(&Message{Type: "error", Data: fmt.Sprintf("Unable to fetch zone: %v", err)})
		}
	}
}

// Send a zone file update.
func (glcd *GLCD) UpdateZone(zone *Zone) {
	query := bson.M{"zone": zone.Name}
	zdata := ZoneInfo{}
	c := glcd.MongoDB.C("zones")
	val := bson.M{"type": "zone", "zdata": zdata, "timestamp": time.Now()}
	change := bson.M{"$set": val}

	err := c.Update(query, change)

	if err == mgo.ErrNotFound {
		val["id"], _ = c.Count()
		change = bson.M{"$set": val}
		err = c.Update(query, change)
	}

	if err != nil {
		glcd.Publish(&Message{Type: "error", Data: fmt.Sprintf("Unable to update zone: %v", err)})
	} else {
		glcd.Publish(&Message{Type: "error", Data: fmt.Sprintf("Updated zone '%s'", zone.Name)})
	}
}

func (glcd *GLCD) HandleMessage(nsqMessage *nsq.Message) error {

	// fmt.Println("-------")
	// fmt.Printf("Received message %s\n\n", nsqMessage.Body)
	// fmt.Println("-------")
	msg := &Message{}

	err := json.Unmarshal(nsqMessage.Body, &msg)

	if err != nil {
		fmt.Printf(err.Error())
	}

	var dataMap map[string]interface{}
	var ok bool

	if msg.Data != nil {
		dataMap, ok = msg.Data.(map[string]interface{})
	} else {
		dataMap = make(map[string]interface{})
		ok = true
	}

	if !ok {
		return nil
	}

	if msg.Type == "playerPassport" {
		//		HandlePassport(msg.Data)
	} else if msg.Type == "playerState" {
		var ps PlayerState
		err := ms.Decode(dataMap, &ps)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			ps.ClientId = msg.ClientId
			log.Printf("Player state: %+v\n", ps)
			glcd.PlayerStateChan <- &ps
		}
	} else if msg.Type == "connected" {
		fmt.Println("Received connected from client")
		glcd.SendZones()
	} else if msg.Type == "sendZones" {
		fmt.Println("Received sendZones from client")
		//		HandleZoneUpdate(msg.Data)
	} else if msg.Type == "chat" {
		glcd.HandleChatMessage(msg, msg.Data)
	} else if msg.Type == "heartbeat" {
		hb := &Heartbeat{}
		hb.ClientId = msg.ClientId
		glcd.HeartbeatChan <- hb
	} else if msg.Type == "knock" {
		glcd.KnockChan <- glcd.Clients[msg.ClientId]
	} else if msg.Type == "error" {
		//		HandleError(msg.Data)
	} else {
		// log.Printf("Unknown Message Type: %s", msg.Type)
	}

	return nil
}
