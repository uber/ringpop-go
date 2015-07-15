package ringpop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	PROXY TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
func TestBasicProxy(t *testing.T) {
	incarnation := unixMilliseconds(time.Now())

	// testPop makes the local member alive
	ringpop := testPop("127.0.0.1:3000", incarnation, nil)
	defer ringpop.Destroy()

	ringpop.ring.addServer("server1")

	var proxyOpts map[string]interface{}

	header := proxyReqHeader{
		URL:      "127.0.0.1:3000",
		Checksum: ringpop.ring.checksum,
		Keys:     []string{"key"},
	}
	body := proxyReqBody{
		body: []byte("hello"),
	}

	req := proxyReq{
		Header: header,
		Body:   body,
	}

	var res proxyReqRes
	test := ringpop.handleOrProxy("key", &req, &res, proxyOpts)

	assert.True(t, test, "Expected to be handled and not proxied")
}

func TestProxyMaxRetries(t *testing.T) {
	incarnation := unixMilliseconds(time.Now())

	// testPop makes the local member alive
	ringpop := testPop("127.0.0.1:3000", incarnation, nil)
	defer ringpop.Destroy()

	var proxyOpts map[string]interface{}

	ringpop.membership.makeAlive("127.0.0.1:3001", incarnation)
	ringpop.membership.makeAlive("127.0.0.1:3002", incarnation)

	ringpop.ring.addServer("server1")
	ringpop.ring.addServer("server2")
	ringpop.ring.addServer("server3")

	header := proxyReqHeader{
		URL:      "127.0.0.1:3000",
		Checksum: ringpop.ring.checksum,
		Keys:     []string{"key"},
	}
	body := proxyReqBody{
		body: []byte("hello"),
	}

	req := proxyReq{
		Header: header,
		Body:   body,
	}

	var res proxyReqRes
	test := ringpop.handleOrProxy("key", &req, &res, proxyOpts)

	//err := ringpop.proxyReq(proxyOpts)
	assert.False(t, test, "Expected to be proxied")
}
