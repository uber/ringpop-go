package ringpop

import (
	"strings"
	"testing"
	"time"

	"code.uber.internal/go-common.git/x/net/xhttp"
	"code.uber.internal/go-common.git/x/net/xhttp/xhttptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	FORWARD TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
// ForwardTestSuite has a recording server used by all the tests
type ForwardTestSuite struct {
	suite.Suite
	testServer *xhttptest.RecordingServer
}

/*
 * SetupSuite instantiates a test server with a valid route to /topics/mytopic
  * and instantiates a ForwardClient object as well
*/
func (suite *ForwardTestSuite) SetupSuite() {
	suite.testServer = xhttptest.NewRecordingServer()
	suite.testServer.Router.AddPatternRoute("/test/myforward",
		xhttp.ServeContentHandler("", time.Time{}, strings.NewReader("My Text")))
	suite.testServer.Router.AddPatternRoute("/error", xhttp.ErrorHandler("no such topic", 420))
}

// TearDownSuite is expected to close the test server
func (suite *ForwardTestSuite) TearDownSuite() {
	suite.testServer.Close()
}

func (suite *ForwardTestSuite) TestForwardBasic() {
	t := suite.T()
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

	req := forwardReq{
		Header: header,
		Body:   []byte("hello"),
	}

	var res forwardReqRes
	test := ringpop.handleOrForward("key", &req, &res, forwardOpts)

	assert.True(t, test, "Expected to be handled and not proxied")
}

func (suite *ForwardTestSuite) TestForwardMaxRetries() {
	t := suite.T()
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

	req := forwardReq{
		Header: header,
		Body:   []byte("hello"),
	}

	var res forwardReqRes
	test := ringpop.handleOrForward("key", &req, &res, forwardOpts)

	assert.False(t, test, "Expected to be proxied")
}

func (suite *ForwardTestSuite) TestForwardInvalid() {
	t := suite.T()
	incarnation := unixMilliseconds(time.Now())

	// testPop makes the local member alive
	ringpop1 := testPop("127.0.0.1:3000", incarnation, nil)
	defer ringpop1.Destroy()

	incarnation1 := unixMilliseconds(time.Now())
	ringpop2 := testPop("127.0.0.1:4000", incarnation1, nil)
	defer ringpop2.Destroy()

	incarnation2 := unixMilliseconds(time.Now())
	ringpop3 := testPop("127.0.0.1:5000", incarnation2, nil)
	defer ringpop3.Destroy()

	var forwardOpts map[string]interface{}

	ringpop1.membership.makeAlive("127.0.0.1:3001", incarnation)
	ringpop1.membership.makeAlive("127.0.0.1:3002", incarnation)

	ringpop1.ring.addServer("server1")
	ringpop2.ring.addServer("server2")
	ringpop3.ring.addServer("server3")

	header := forwardReqHeader{
		URL:      "127.0.0.1:3000",
		Checksum: ringpop1.ring.checksum,
		Keys:     []string{"key", "key2"},
	}

	req := forwardReq{
		Header: header,
		Body:   []byte("hello"),
	}

	dest, _ := ringpop1.ring.lookup("key")

	var res forwardReqRes
	forwardOpts = map[string]interface{}{
		"dest": dest,
		"keys": []string{"key"},
		"req":  &req,
		"res":  &res,
	}
	err := ringpop1.forwardReq(forwardOpts)
	assert.NotNil(t, err)
	//assert.False(t, test, "Expected to be proxied")
}

func (suite *ForwardTestSuite) TestForwardSuccess() {
	t := suite.T()
	hostport1 := "127.0.0.1:3001"
	hostport2 := "127.0.0.1:3002"

	var hosts []string

	hosts = append(hosts, hostport1)
	ringpop1 := newServerRingpop(t, hostport1)
	defer ringpop1.Destroy()

	nodesJoined, err := ringpop1.Bootstrap(&BootstrapOptions{
		Hosts:   hosts,
		Stopped: true,
	})

	assert.NoError(t, err, "expected bootstrap to complete successfully")
	assert.Empty(t, nodesJoined, "expected join of size zero (self only)")
	assert.True(t, ringpop1.Ready(), "expected ringpop to be ready")
	assert.True(t, ringpop1.gossip.Stopped(), "expected gossip to be stopped")

	hosts = append(hosts, hostport2)
	ringpop2 := newServerRingpop(t, hostport2)
	defer ringpop2.Destroy()

	nodesJoined, err = ringpop2.Bootstrap(&BootstrapOptions{
		Hosts:   hosts,
		Stopped: true,
	})

	assert.NoError(t, err, "expected bootstrap to complete successfully")
	assert.Len(t, nodesJoined, 1, "expected join of size one")
	assert.True(t, ringpop2.Ready(), "expected ringpop to be ready")
	assert.True(t, ringpop2.gossip.Stopped(), "expected gossip to be stopped")

	var forwardOpts map[string]interface{}

	header := forwardReqHeader{
		URL:      suite.testServer.HTTPServer.URL + "/test/myforward",
		Checksum: ringpop2.membership.checksum,
		Keys:     []string{"key"},
	}

	req := forwardReq{
		Header: header,
		Body:   []byte("hello"),
	}

	dest, _ := ringpop2.ring.lookup("key")

	var res forwardReqRes
	forwardOpts = map[string]interface{}{
		"dest": dest,
		"keys": []string{"key"},
		"req":  &req,
		"res":  &res,
	}

	err = ringpop2.forwardReq(forwardOpts)
	assert.Nil(t, err)
	assert.Equal(t, res.StatusCode, 200)
}

func (suite *ForwardTestSuite) TestForwardFailure() {
	t := suite.T()
	hostport1 := "127.0.0.1:3001"
	hostport2 := "127.0.0.1:3002"

	var hosts []string

	hosts = append(hosts, hostport1)
	ringpop1 := newServerRingpop(t, hostport1)
	defer ringpop1.Destroy()

	nodesJoined, err := ringpop1.Bootstrap(&BootstrapOptions{
		Hosts:   hosts,
		Stopped: true,
	})

	assert.NoError(t, err, "expected bootstrap to complete successfully")
	assert.Empty(t, nodesJoined, "expected join of size zero (self only)")
	assert.True(t, ringpop1.Ready(), "expected ringpop to be ready")
	assert.True(t, ringpop1.gossip.Stopped(), "expected gossip to be stopped")

	hosts = append(hosts, hostport2)
	ringpop2 := newServerRingpop(t, hostport2)
	defer ringpop2.Destroy()

	nodesJoined, err = ringpop2.Bootstrap(&BootstrapOptions{
		Hosts:   hosts,
		Stopped: true,
	})

	assert.NoError(t, err, "expected bootstrap to complete successfully")
	assert.Len(t, nodesJoined, 1, "expected join of size one")
	assert.True(t, ringpop2.Ready(), "expected ringpop to be ready")
	assert.True(t, ringpop2.gossip.Stopped(), "expected gossip to be stopped")

	var forwardOpts map[string]interface{}

	header := forwardReqHeader{
		URL:      "127.0.0.1:3001",
		Checksum: ringpop2.membership.checksum,
		Keys:     []string{"key"},
	}

	req := forwardReq{
		Header: header,
		Body:   []byte("hello"),
	}

	dest, _ := ringpop2.ring.lookup("key")

	var res forwardReqRes
	forwardOpts = map[string]interface{}{
		"dest": dest,
		"keys": []string{"key"},
		"req":  &req,
		"res":  &res,
	}

	err = ringpop2.forwardReq(forwardOpts)
	assert.NotNil(t, err)
}

func TestRunSuite(t *testing.T) {
	suiteForwardTester := new(ForwardTestSuite)
	suite.Run(t, suiteForwardTester)
}
