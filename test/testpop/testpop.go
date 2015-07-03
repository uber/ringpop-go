package main

import (
	"os"
	"regexp"
	"ringpop"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
)

var hostPortPattern = regexp.MustCompile(`^(\d+.\d+.\d+.\d+):\d+$`)

func main() {

	logger := log.StandardLogger()
	logger.Formatter = &log.TextFormatter{}
	logger.Level = log.DebugLevel

	args := os.Args
	if len(args) != 2 {
		log.Fatal("too many/not enough arguments")
	}

	hostport := args[1]
	if !hostPortPattern.Match([]byte(hostport)) {
		log.Fatal("bad host:port")
	}

	ch, err := tchannel.NewChannel("ringpop", &tchannel.ChannelOptions{
		Logger: logger,
	})
	if err != nil {
		log.Fatalf("could not create channel: %v", err)
	}

	rp := ringpop.NewRingpop("test-service", hostport, ch, &ringpop.Options{
		Logger: logger,
	})

	// nodesJoined, err := rp.Bootstrap(ringpop.BootstrapOptions{File: "./testpop/hosts.json"})
	nodesJoined, err := rp.Bootstrap(ringpop.BootstrapOptions{Hosts: []string{rp.WhoAmI()}})
	if err != nil {
		return
	}
	log.WithFields(log.Fields{
		"hostport":  rp.WhoAmI(),
		"operation": rp.App(),
		"numJoined": len(nodesJoined),
	}).Info("[ringpop] bootstrap successful, listening")

	for {
		// do nothing FOREVER
	}
}
