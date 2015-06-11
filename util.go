package ringpop

import (
	"regexp"
	"strings"
	"time"
)

var hostPortPattern = regexp.MustCompile("^([0-9]+.[0-9]+.[0-9]+.[0-9]+):[0-9]+$")

func unixMilliseconds() int64 {
	return time.Now().UnixNano() / 1000000
}

// Testing functions

// takes x.x.x.x:y and returns x.x.x.x
func captureHost(hostport string) string {
	if hostPortPattern.Match([]byte(hostport)) {
		parts := strings.Split(hostport, ":")
		return parts[0]
	}
	return ""
}

func selectNumDefault(opt, def int) int {
	if opt == 0 {
		return def
	}
	return opt
}

func selectDurationDefault(opt, def time.Duration) time.Duration {
	if opt == time.Duration(0) {
		return def
	}
	return opt
}

func testPop(hostport string) *Ringpop {
	ringpop := NewRingpop("test", hostport, nil)
	ringpop.membership.makeAlive(hostport, 1, "")

	return ringpop
}
