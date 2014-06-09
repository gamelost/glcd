package main

import (
	"bytes"
	iniconf "code.google.com/p/goconf/conf"
	"crypto/sha512"
	"encoding/json"
	"errors"
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
	Name          string
	ClientId      string
	Authenticated bool
	State         *PlayerState
	Heartbeat     time.Time
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
	AuthChan        chan *PlayerAuthInfo
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
	glcd.setupMongoDBConnection()

	// set up channels
	glcd.setupTopicChannels()

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
	go glcd.HandlePlayerAuthChannel()
	go glcd.HandleHeartbeatChannel()
	go glcd.HandleKnockChannel()
	go glcd.HandlePlayerStateChannel()

	return nil
}

func (glcd *GLCD) setupTopicChannels() {
	// set up channels
	glcd.HeartbeatChan = make(chan *Heartbeat)
	glcd.KnockChan = make(chan *GLCClient)
	glcd.AuthChan = make(chan *PlayerAuthInfo)
	glcd.PlayerStateChan = make(chan *PlayerState)
}

func (glcd *GLCD) setupMongoDBConnection() error {

	// Connect to Mongo.
	servers, err := glcd.ConfigFile.GetString("mongo", "servers")
	if err != nil {
		return err
	}

	glcd.MongoSession, err = mgo.Dial(servers)
	if err != nil {
		return err
	}

	db, err := glcd.ConfigFile.GetString("mongo", "db")
	if err != nil {
		return err
	} else {
		fmt.Println("Successfully obtained config from mongo")
	}

	glcd.MongoDB = glcd.MongoSession.DB(db)
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

	// UNIMPLEMENTED TYPES: playerPassport, sendZones, error
	// add new/future handler functions in glcd-handlers.go
	if msg.Type == "playerState" {
		glcd.HandlePlayerState(msg, dataMap)
	} else if msg.Type == "connected" {
		glcd.HandleConnected(msg, dataMap)
	} else if msg.Type == "chat" {
		glcd.HandleChat(msg, msg.Data)
	} else if msg.Type == "heartbeat" {
		glcd.HandleHeartbeat(msg, dataMap)
	} else if msg.Type == "knock" {
		glcd.HandleKnock(msg, dataMap)
	} else if msg.Type == "playerAuth" {
		glcd.HandlePlayerAuth(msg, dataMap)
	} else {
		fmt.Printf("Unable to determine handler for message: %+v\n", msg)
	}

	return nil
}

func (glcd *GLCD) isPasswordCorrect(name string, password string) (bool, error) {
	c := glcd.MongoDB.C("users")
	authInfo := PlayerAuthInfo{}
	query := bson.M{"user": name}
	err := c.Find(query).One(&authInfo)

	if err != nil {
		return false, err
	}

	return password == authInfo.Password, nil
}

func generateSaltedPasswordHash(password string, salt []byte) ([]byte, error) {
	hash := sha512.New()
	//hash.Write(server_salt)
	hash.Write(salt)
	hash.Write([]byte(password))
	return hash.Sum(salt), nil
}

func (glcd *GLCD) getUserPasswordHash(name string) ([]byte, error) {
	return nil, nil
}

func (glcd *GLCD) isPasswordCorrectWithHash(name string, password string, salt []byte) (bool, error) {
	expectedHash, err := glcd.getUserPasswordHash(name)

	if err != nil {
		return false, err
	}

	if len(expectedHash) != 32+sha512.Size {
		return false, errors.New("Wrong size")
	}

	actualHash := sha512.New()
	actualHash.Write(salt)
	actualHash.Write([]byte(password))

	return bytes.Equal(actualHash.Sum(nil), expectedHash[32:]), nil
}
