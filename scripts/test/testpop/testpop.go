package main

import (
	"flag"
	"regexp"
	"ringpop"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
)

var hostport = flag.String("hostport", "127.0.0.1:3000", "hostport to start ringpop on")

var hostPortPattern = regexp.MustCompile(`^(\d+.\d+.\d+.\d+):\d+$`)

func main() {
	logger := log.StandardLogger()
	logger.Formatter = &log.TextFormatter{}
	// logger.Level = log.DebugLevel

	flag.Parse()

	if !hostPortPattern.Match([]byte(*hostport)) {
		log.Fatal("bad host:port")
	}

	ch, err := tchannel.NewChannel("ringpop", nil)
	if err != nil {
		log.Fatalf("could not create channel: %v", err)
	}

	rp := ringpop.NewRingpop("test-service", *hostport, ch, &ringpop.Options{
		Logger: logger,
	})

	nodesJoined, err := rp.Bootstrap(&ringpop.BootstrapOptions{File: "./testpop/hosts.json"})
	if err != nil {
		return
	}
	log.WithFields(log.Fields{
		"hostport":  rp.WhoAmI(),
		"service":   rp.App(),
		"numJoined": len(nodesJoined),
	}).Info("[ringpop] bootstrap successful, listening")

	select {}
}
