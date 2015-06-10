package ringpop

import "time"

func TimeNow() int64 {
	return time.Now().UnixNano() / 1000000
}

// Testing functions

func testPop(hostport string) *Ringpop {
	ringpop := NewRingpop("test", hostport, nil)
	ringpop.membership.makeAlive(hostport, 1, "")

	return ringpop
}
