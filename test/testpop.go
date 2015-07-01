package main

import (
	"os"
	"regexp"
	"ringpop"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
)

var hostPortPattern = regexp.MustCompile(`^(\d+.\d+.\d+.\d+):\d+$`)

func main() {
	args := os.Args
	if len(args) != 2 {
		log.Fatal("too many/not enough arguments")
	}

	hostport := args[1]
	if !hostPortPattern.Match([]byte(hostport)) {
		log.Fatal("bad host:port")
	}

	ch, err := tchannel.NewChannel("ringpop", nil)
	if err != nil {
		log.Fatalf("could not create channel: %v", err)
	}
	rp := ringpop.NewRingpop("test-service", hostport, ch, nil)

	nodesJoined, err := rp.Bootstrap(ringpop.BootstrapOptions{File: "./hosts.json"})
	if err != nil {
		log.Fatalf("could not bootstrap ringpop: %v", err)
	}
	log.WithFields(log.Fields{
		"hostport":  rp.WhoAmI(),
		"service":   rp.App(),
		"numJoined": len(nodesJoined),
	}).Info("ringpop bootstrap successful, listening")

	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait() // block forever and let ringpop do its thing
}
