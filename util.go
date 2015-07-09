package ringpop

import (
	"io/ioutil"
	"math/rand"
	"regexp"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
)

var hostPortPattern = regexp.MustCompile(`^(\d+.\d+.\d+.\d+):\d+$`)

func unixMilliseconds(t time.Time) int64 {
	return t.UnixNano() / 1000000
}

func milliseconds(d time.Duration) int64 {
	return d.Nanoseconds() / 1000000
}

func indexOf(slice []string, element string) int {
	for i, e := range slice {
		if e == element {
			return i
		}
	}

	return -1
}

// mutates nodes
func takeNode(nodes *[]string, index int) string {
	if len(*nodes) == 0 {
		return ""
	}

	var i int
	if index >= 0 {
		if index >= len(*nodes) {
			return ""
		}
		i = index
	} else {
		i = rand.Intn(len(*nodes))
	}

	node := (*nodes)[i]

	*nodes = append((*nodes)[:i], (*nodes)[i+1:]...)

	return node
}

// takes x.x.x.x:y and returns x.x.x.x
func captureHost(hostport string) string {
	if hostPortPattern.Match([]byte(hostport)) {
		parts := strings.Split(hostport, ":")
		return parts[0]
	}
	return ""
}

func selectNumOrDefault(opt, def int) int {
	if opt == 0 {
		return def
	}
	return opt
}

func selectDurationOrDefault(opt, def time.Duration) time.Duration {
	if opt == time.Duration(0) {
		return def
	}
	return opt
}

func testPop(hostport string, incarnation int64) *Ringpop {
	logger := log.New()
	logger.Out = ioutil.Discard

	testCh, _ := tchannel.NewChannel("test-service", nil)
	ringpop := NewRingpop("test", hostport, testCh, &Options{
		Logger: logger,
	})

	ringpop.testBootstrapper()
	if incarnation != 0 {
		ringpop.membership.localMember.Incarnation = incarnation
	}

	return ringpop
}
