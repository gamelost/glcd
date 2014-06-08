package main

import (
	iniconf "code.google.com/p/goconf/conf"
	"encoding/json"
	"fmt"
	// "github.com/gamelost/bot3server/server"
	nsq "github.com/gamelost/go-nsq"
	// irc "github.com/gamelost/goirc/client"
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
	PlayerName string
	ClientId   string
	Type       string // better way to persist type info?
	Data       string // Json Representation of any of the structs or strings or time.Time
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

type PlayerState struct {
	x int
	y int
}

/* Players coming in and out */
type PlayerPassport struct {
	Action string
	Avatar string
}

type ErrorMessage string

type WallMessage string

// type HeartbeatMessage time.Time  //eh?

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
	Name      string
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
	}

	glcd.MongoDB = glcd.MongoSession.DB(db)

	nsqdAddress, _ := conf.GetString("nsq", "nsqd-address")
	lookupdAddress, _ := conf.GetString("nsq", "lookupd-address")
	glcd.GLCGameStateTopicName, _ = conf.GetString("nsq", "server-topic")

	glcdTopic, _ := conf.GetString("nsq", "glcd-topic")

	// Create the channel, by connecting to lookupd. (TODO; if it doesn't
	// exist. Also do it the right way with a Register command?)
	glcd.NSQWriter = nsq.NewWriter(nsqdAddress)
	glcd.NSQWriter.Publish(glcd.GLCGameStateTopicName, []byte("{\"client\":\"server\"}"))

	// set up listener for heartbeat from bot3server
	reader, err := nsq.NewReader(glcdTopic, "main")
	if err != nil {
		glcd.QuitChan <- syscall.SIGINT
	}
	glcd.GLCDaemonTopic = reader
	glcd.GLCDaemonTopic.AddHandler(glcd)
	glcd.GLCDaemonTopic.ConnectToLookupd(lookupdAddress)

	// Spawn goroutine to clear out clients who don't send hearbeats
	// anymore.
	go glcd.CleanupClients()

	return nil
}

func (glcd *GLCD) Publish(msg *Message) {
	encodedRequest, _ := json.Marshal(*msg)
	glcd.NSQWriter.Publish(glcd.GLCGameStateTopicName, encodedRequest)
}

func (glcd *GLCD) CleanupClients() error {
	for {
		exp := time.Now().Unix()
		<-time.After(time.Second * 10)
		// Expire any clients who haven't sent a heartbeat in the last 10 seconds.
		for k, v := range glcd.Clients {
			if v.Heartbeat.Unix() < exp {
				delete(glcd.Clients, k)
				glcd.Publish(&Message{Type: "playerPassport", Data: PlayerPassport{Action: "playerGone"}}) // somehow add k to this
			}
		}
	}
}

// Send a zone file update.
func (glcd *GLCD) SendZones() {
	c := glcd.MongoDB.C("zones")
	q := c.Find(nil)

	if q == nil {
		glcd.Publish(&Message{Type: "error", Data: fmt.Sprintf("No zones found")})
	} else {
		var results []interface{}
		err := q.All(&results)
		if err == nil {
			for _, res := range results {
				glcd.Publish(&Message{Type: "zone", Data: res}) // dump res as a JSON string
			}
		} else {
			glcd.Publish(&Message{Type: "error", Data: fmt.Sprintf("Unable to fetch zones: %v", err)})
		}
	}
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
			glcd.Publish(&Message{Type: "zone", Data: res})
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

func (glcd *GLCD) HandleMessage(message *nsq.Message) error {

	msg := Message{}

	err := json.Unmarshal(message.Body, &msg)

	if err != nil {
		return fmt.Errorf("Not a JSON interface")
	}

	if msg.Type == "playerPassport" {
		//		HandlePassport(msg.Data)
	} else if msg.Type == "playerState" {
		//		HandlePlayerState(msg.Data)
	} else if msg.Type == "zone" {
		//		HandleZoneUpdate(msg.Data)
	} else if msg.Type == "wall" {
		//		HandleWallMessage(msg.Data)
	} else if msg.Type == "heartbeat" {
		//		HandleHeartbeat(msg.Data)
	} else if msg.Type == "error" {
		//		HandleError(msg.Data)
	} else {
		// log.Printf("Unknown Message Type: %s", msg.Type)
	}

	return nil
}
