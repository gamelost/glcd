What is this?
====

GameLostCrash server daemon for world state. In cooperation with glc-client (https://github.com/gamelost/glc-client), this provides a platform

The code here is volatile and experimental, just like its developers. While the goal of glc-client is to create a 2d metaverse, glcd's aim is to be agnostic of all UI and worlds, instead acting strictly as a platform to let clients communicate state to each other.

Installation
====
glcd relies on two other services: nsq, for communicating with clients, and mongo, for storing world state.

Apart from nsq and mongo, glcd should be installable using only "go get github.com/gamelost/glcd"

Configuration
====
the glcd.config.default file exists and should be copied to glcd.config for your own installation. It contains two sections: nsq and mongo, for configuring access to their servers.

Of import: server-topic, server-channel and glcd-topic must be identical with those in glc-client.

Running
====
Simply: when glcd is installed, run it in a directory with an existing glcd.config

Design
====

glcd and glc-client communicate over nsq, using JSON for player state.

The player state structure is currently (and constantly) in flux.

Channels
====
glcd and glc-client communicate via channels and namespaces. All clients send messages to glcd via the configured "glcd-topic" channel, which only glcd should listen on.

glcd broadcasts over the "server-topic" channel, which all glc-clients listen to, creating their own namespaces within the channel.

Messages
====
Messages are received and sent over channels. Of import:

* Heartbeats are expected from all clients, or they'll be cleared.
* Knock: Responds to single client with information on all connected clients.
* chat: Broadcast to everyone.
* playerauth: in progress
