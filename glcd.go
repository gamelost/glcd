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
	Name       string
	PlayerName string
	ClientId   string
	Type       string // better way to persist type info?
	Command    string
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

type PlayerState struct {
	X float64 `mapstructure:"px"`
	Y float64 `mapstructure:"py"`
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

type WallMessage string

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

	// game state channels
	HeartbeatChan chan *Heartbeat

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

	// set up channels
	glcd.HeartbeatChan = make(chan *Heartbeat)

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

	// Spawn goroutine to clear out clients who don't send hearbeats
	// anymore.
	go glcd.CleanupClients()
	go glcd.HandleHeartbeatChannel()

	return nil
}

func (glcd *GLCD) Publish(msg *Message) {
	encodedRequest, _ := json.Marshal(*msg)
	glcd.NSQWriter.Publish(glcd.GLCGameStateTopicName, encodedRequest)
}

func (glcd *GLCD) HandleHeartbeatChannel() {
	for {
		hb := <-glcd.HeartbeatChan
		fmt.Printf("HandleHeartbeatChannel: Received heartbeat: %+v\n", hb)

		// see if key and client exists in the map
		c, exists := glcd.Clients[hb.ClientId]

		if exists {
			fmt.Printf("Client %s exists.  Updating heartbeat.\n", hb.ClientId)
			c.Heartbeat = time.Now()
		} else {
			fmt.Printf("Adding client %s to client list\n", hb.ClientId)
			client := &GLCClient{ClientId: hb.ClientId, Heartbeat: time.Now()}
			glcd.Clients[hb.ClientId] = client
		}
	}
}

func (glcd *GLCD) CleanupClients() error {
	for {
		exp := time.Now().Unix()
		<-time.After(time.Second * 10)
		fmt.Println("Doing client clean up")
		// Expire any clients who haven't sent a heartbeat in the last 10 seconds.
		for k, v := range glcd.Clients {
			if v.Heartbeat.Unix() < exp {
				fmt.Printf("Deleting client %s due to inactivity.\n", v.ClientId)
				delete(glcd.Clients, k)
				//glcd.Publish(&Message{Type: "playerPassport", Data: PlayerPassport{Action: "playerGone"}}) // somehow add k to this
			} else {
				fmt.Printf("Client has not expired.")
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
				glcd.Publish(&Message{Type: "zone", Data: res.(string)}) // dump res as a JSON string
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
	if msg.Data != nil {
		dataMap = msg.Data.(map[string]interface{})
	} else {
		dataMap = make(map[string]interface{})
	}

	if msg.Command == "playerPassport" {
		//		HandlePassport(msg.Data)
	} else if msg.Command == "playerState" {
		var ps PlayerState
		//fmt.Printf("Data is: %v, %v\n", psmap["px"], psmap["py"])
		err := ms.Decode(dataMap, &ps)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Printf("Player state: %+v\n", ps)
	} else if msg.Command == "zone" {
		//		HandleZoneUpdate(msg.Data)
	} else if msg.Command == "wall" {
		//		HandleWallMessage(msg.Data)
	} else if msg.Command == "heartbeat" {
		hb := &Heartbeat{}
		hb.ClientId = msg.Name
		glcd.HeartbeatChan <- hb
	} else if msg.Command == "error" {
		//		HandleError(msg.Data)
	} else {
		// log.Printf("Unknown Message Type: %s", msg.Type)
	}

	return nil
}
