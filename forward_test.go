package ringpop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	FORWARD TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
func TestBasicForward(t *testing.T) {
	incarnation := unixMilliseconds(time.Now())

	// testPop makes the local member alive
	ringpop := testPop("127.0.0.1:3000", incarnation, nil)
	defer ringpop.Destroy()

	ringpop.ring.addServer("server1")

	var forwardOpts map[string]interface{}

	header := forwardReqHeader{
		URL:      "127.0.0.1:3000",
		Checksum: ringpop.ring.checksum,
		Keys:     []string{"key"},
	}
	body := forwardReqBody{
		body: []byte("hello"),
	}

	req := forwardReq{
		Header: header,
		Body:   body,
	}

	var res forwardReqRes
	test := ringpop.handleOrForward("key", &req, &res, forwardOpts)

	assert.True(t, test, "Expected to be handled and not proxied")
}

func TestForwardMaxRetries(t *testing.T) {
	incarnation := unixMilliseconds(time.Now())

	// testPop makes the local member alive
	ringpop := testPop("127.0.0.1:3000", incarnation, nil)
	defer ringpop.Destroy()

	var forwardOpts map[string]interface{}

	ringpop.membership.makeAlive("127.0.0.1:3001", incarnation)
	ringpop.membership.makeAlive("127.0.0.1:3002", incarnation)

	ringpop.ring.addServer("server1")
	ringpop.ring.addServer("server2")
	ringpop.ring.addServer("server3")

	header := forwardReqHeader{
		URL:      "127.0.0.1:3000",
		Checksum: ringpop.ring.checksum,
		Keys:     []string{"key"},
	}
	body := forwardReqBody{
		body: []byte("hello"),
	}

	req := forwardReq{
		Header: header,
		Body:   body,
	}

	var res forwardReqRes
	test := ringpop.handleOrForward("key", &req, &res, forwardOpts)

	//err := ringpop.forwardReq(forwardOpts)
	assert.False(t, test, "Expected to be proxied")
}
