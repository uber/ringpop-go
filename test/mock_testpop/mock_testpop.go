package main

import (
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

var hostPortPattern = regexp.MustCompile(`^(\d+.\d+.\d+.\d+):\d+$`)

func main() {
	log := logrus.StandardLogger()

	args := os.Args
	if len(args) < 2 {
		log.Fatal("not enough arguments")
	}

	hostport := args[1]
	if !hostPortPattern.Match([]byte(hostport)) {
		println("??")
		log.Fatal("bad host:port")
	}

	log.WithField("hostport", hostport).Info("started")

	// fake goroutine doing something
	ticker := time.NewTicker(time.Second)
	go func() {
		<-ticker.C
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait() // block forever and let ringpop do its thing
}
