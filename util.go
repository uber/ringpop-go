package ringpop

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/require"
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

func genNodes(from, to int) []string {
	var hosts []string
	for i := from; i <= to; i++ {
		hosts = append(hosts, fmt.Sprintf("127.0.0.1:%v", 3000+i))
	}

	return hosts
}

func genNodesDiffHosts(from, to int) []string {
	var hosts []string
	for i := from; i <= to; i++ {
		hosts = append(hosts, fmt.Sprintf("127.0.0.%v:3001", i))
	}

	return hosts
}

func genRingpops(t *testing.T, hostports []string) []*Ringpop {
	var ringpops []*Ringpop

	for _, hostport := range hostports {
		ringpop := newServerRingpop(t, hostport)
		ringpops = append(ringpops, ringpop)
	}

	return ringpops
}

func destroyRingpops(ringpops []*Ringpop) {
	for _, ringpop := range ringpops {
		ringpop.Destroy()
	}
}

func newServerRingpop(t *testing.T, hostport string) *Ringpop {
	channel, err := tchannel.NewChannel("ringpop", nil)
	require.NoError(t, err, "error must be nil")

	logger := log.New()
	logger.Out = ioutil.Discard

	ringpop := NewRingpop("test", hostport, channel, &Options{
	// Logger: logger,
	})

	return ringpop
}

func newServerRingpopSub(t *testing.T, hostport string, ch *tchannel.Channel) *Ringpop {
	subCh := ch.GetSubChannel("ringpop")

	logger := log.New()
	logger.Out = ioutil.Discard

	ringpop := NewRingpop("test", hostport, subCh, &Options{
	// Logger: logger,
	})

	return ringpop
}

func testPop(hostport string, incarnation int64, opts *Options) *Ringpop {
	logger := log.New()
	logger.Out = ioutil.Discard

	if opts == nil {
		opts = &Options{}
	}
	opts.Logger = logger

	testCh, _ := tchannel.NewChannel("test-service", nil)
	ringpop := NewRingpop("test", hostport, testCh, opts)

	ringpop.testBootstrapper()
	if incarnation != 0 {
		ringpop.membership.localMember.Incarnation = incarnation
	}

	return ringpop
}
