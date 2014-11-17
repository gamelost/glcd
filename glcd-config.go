package main

import (
  iniconf "code.google.com/p/goconf/conf"
  "flag"
  "fmt"
)

const (
  GLCD_CONFIG = "glcd.config"
)

type GLCConfig struct {
  NSQ struct {
    Address        string
    LookupdAddress string
    ReadTopic      string
    PublishTopic   string
  }
  Mongo struct {
    Servers string
    DB      string
  }
}

func setString(category, name string, msg string, ptr *string, conf *iniconf.ConfigFile) {
  // If it's in the config file, read it in.
  if conf != nil {
    str, err := conf.GetString(category, name)
    if err == nil {
      *ptr = str
    }
  }

  // Add it to flag for later parsing.
  flag.StringVar(ptr, fmt.Sprintf("%s-%s", category, name), *ptr, msg)
}

// Configuration priority:
//
// 1) Defaults.
// 2) Config file.
// 3) Flags.
//
// (Each overwrites any such settings in the prior)
func ReadConfiguration() *GLCConfig {
  // Sensible defaults.
  ret := &GLCConfig{}
  ret.NSQ.Address = "localhost:4150"
  ret.NSQ.LookupdAddress = "localhost:4161"
  ret.NSQ.ReadTopic = "glc-daemon"
  ret.NSQ.PublishTopic = "glc-gamestate"

  ret.Mongo.Servers = "localhost"
  ret.Mongo.DB = "test"

  configFile, err := iniconf.ReadConfigFile(GLCD_CONFIG)
  // It's okay to not have a config file.
  if err != nil {
    configFile = nil
  }

  // NSQ configuration, and set up flags for it as well.
  setString("nsq", "nsqd-address", "host:port of the NSQ daemon (nsq)",
    &ret.NSQ.Address, configFile)
  setString("nsq", "lookupd-address", "host:port of the NSQ Lookup Daemon (http)",
    &ret.NSQ.LookupdAddress, configFile)
  setString("nsq", "server-topic", "Topic GLCD listens on for clients",
    &ret.NSQ.ReadTopic, configFile)
  setString("nsq", "glcd-topic", "Topic GLCD broadcasts to clients via",
    &ret.NSQ.PublishTopic, configFile)

  // Mongo configuration
  setString("mongo", "servers", "Server(s) of Mongo daemons", &ret.Mongo.Servers, configFile)
  setString("mongo", "db", "DB that GLCD uses within Mongo", &ret.Mongo.DB, configFile)

  // Parse all flags.
  flag.Parse()

  return ret
}

func (conf *GLCConfig) PrintConfiguration() {
  fmt.Printf("[%s]\n", "nsq")
  fmt.Printf("%s = %s\n", "nsqd-address", conf.NSQ.Address)
  fmt.Printf("%s = %s\n", "lookupd-address", conf.NSQ.LookupdAddress)
  fmt.Printf("%s = %s\n", "server-topic", conf.NSQ.ReadTopic)
  fmt.Printf("%s = %s\n", "glcd-topic", conf.NSQ.PublishTopic)

  fmt.Printf("\n")
  fmt.Printf("[%s]\n", "mongo")
  fmt.Printf("%s: %s\n", "servers", conf.Mongo.Servers)
  fmt.Printf("%s: %s\n", "db", conf.Mongo.DB)
}
