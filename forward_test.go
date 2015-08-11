package ringpop

import (
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/uber/tchannel/golang"
	"github.com/uber/tchannel/golang/json"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	FORWARD TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
// ForwardTestSuite has a recording server used by all the tests
type ForwardTestSuite struct {
	suite.Suite
	testCh *tchannel.Channel
}

func testHandler(ctx json.Context, req *forwardReq) (*forwardReqRes, error) {
	// Make sure you received the expected value
	if string(req.Body) != "hello" {
		log.Fatalf("Expected \"hello\" but receieved: %v", string(req.Body))
	}

	return &forwardReqRes{
		Body:       []byte("forwardTestResp"),
		StatusCode: 200,
	}, nil
}

func onError(ctx context.Context, err error) {
	log.Fatalf("onError: %v", err)
}

func listenAndHandle(s *tchannel.Channel, hostPort string) {
	log.Infof("Service %s", hostPort)

	// If no error is returned, the listen was successful. Serving happens in the background.
	if err := s.ListenAndServe(hostPort); err != nil {
		log.Fatalf("Could not listen on %s: %v", hostPort, err)
	}
}

/*
* SetupSuite creates a new Tchannel for handling incoming requests sent to "forwardService"
* and registers a handler for an endpoint called "test"
 */
func (suite *ForwardTestSuite) SetupSuite() {
	var err error
	suite.testCh, err = tchannel.NewChannel("forwardService", &tchannel.ChannelOptions{Logger: tchannel.SimpleLogger})
	if err != nil {
		log.Fatalf("Could not create new channel: %v", err)
	}

	// Register a handler for the test message on the ForwardService
	json.Register(suite.testCh, json.Handlers{
		"/test": testHandler,
	}, onError)

	// Listen for incoming requests
	listenAndHandle(suite.testCh, "127.0.0.1:10500")
}

func (suite *ForwardTestSuite) TearDownSuite() {
	// Close the channel
	suite.testCh.Close()
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
		HostPort:  "127.0.0.1:10500",
		Service:   "forwardService",
		Operation: "/test",
		Checksum:  ringpop.ring.checksum,
		Keys:      []string{"key"},
	}

	req := forwardReq{
		Header: header,
		Body:   []byte("hello"),
	}

	var res forwardReqRes
	test := ringpop.handleOrForward("key", &req, &res, forwardOpts)

	assert.True(t, test, "Expected to be handled and not forwarded")
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
		Service:   "forwardService",
		Operation: "/test",
		Checksum:  ringpop.ring.checksum,
		Keys:      []string{"key"},
	}

	req := forwardReq{
		Header: header,
		Body:   []byte("hello"),
	}

	var res forwardReqRes
	test := ringpop.handleOrForward("key", &req, &res, forwardOpts)

	assert.False(t, test, "Expected to be forwarded")
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
		Service:   "forwardService",
		Operation: "/test",
		Checksum:  ringpop1.ring.checksum,
		Keys:      []string{"key", "key2"},
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
		HostPort:  "127.0.0.1:10500",
		Service:   "forwardService",
		Operation: "/test",
		Checksum:  ringpop2.membership.checksum,
		Keys:      []string{"key"},
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
	assert.Equal(t, string(res.Body), "forwardTestResp")
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
		HostPort:  "127.0.0.1:10500",
		Service:   "forwardService",
		Operation: "/test",
		Checksum:  ringpop2.membership.checksum,
		Keys:      []string{"key"},
	}

	req := forwardReq{
		Header: header,
		Body:   []byte("hello"),
	}

	dest, _ := ringpop2.ring.lookup("key")

	var res forwardReqRes
	forwardOpts = map[string]interface{}{
		"dest":     dest,
		"keys":     []string{"key"},
		"req":      &req,
		"res":      &res,
		"service":  "dead",
		"endpoint": "beef",
	}

	err = ringpop2.forwardReq(forwardOpts)
	assert.NotNil(t, err)
}

func TestRunSuite(t *testing.T) {
	suiteForwardTester := new(ForwardTestSuite)
	suite.Run(t, suiteForwardTester)
}
