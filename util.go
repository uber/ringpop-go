package ringpop

import (
	"math/rand"
	"regexp"
	"strings"
	"time"
)

var hostPortPattern = regexp.MustCompile("^([0-9]+.[0-9]+.[0-9]+.[0-9]+):[0-9]+$")

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
func takeNode(nodes *[]string) string {
	if len(*nodes) == 0 {
		return ""
	}

	i := rand.Intn(len(*nodes))
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

func testPop(hostport string) *Ringpop {
	ringpop := NewRingpop("test", hostport, Options{})
	ringpop.testBootstrapper()

	return ringpop
}
