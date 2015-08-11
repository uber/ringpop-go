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
//	REPLICATE TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
// ReplicateTestSuite has a recording server used by all the tests
type ReplicateTestSuite struct {
	suite.Suite
	ringpop *Ringpop
	ch      *tchannel.Channel
}

func testReplicateHandler(ctx json.Context, req *forwardReq) (*forwardReqRes, error) {
	// Make sure you received the expected value
	if string(req.Body) != "hello" {
		log.Fatalf("Expected \"hello\" but receieved: %v", string(req.Body))
	}

	return &forwardReqRes{
		Body:       []byte("replicateTestResp"),
		StatusCode: 200,
	}, nil
}

func onReplicateError(ctx context.Context, err error) {
	log.Fatalf("onError: %v", err)
}

func replicaListenAndHandle(s *tchannel.Channel, hostPort string) {
	log.Infof("Service %s", hostPort)

	// If no error is returned, the listen was successful. Serving happens in the background.
	if err := s.ListenAndServe(hostPort); err != nil {
		log.Fatalf("Could not listen on %s: %v", hostPort, err)
	}
}

/*
* SetupSuite creates a new Tchannel for handling incoming requests sent to "replicatorService"
* and registers a handler for an endpoint called "test"
 */
func (suite *ReplicateTestSuite) SetupSuite() {
	suite.ringpop = testPop("127.0.0.1:3000", unixMilliseconds(time.Now()), nil)
	var err error
	suite.ch, err = tchannel.NewChannel("replicatorService", &tchannel.ChannelOptions{Logger: tchannel.SimpleLogger})
	if err != nil {
		log.Fatalf("Could not create new channel: %v", err)
	}

	// Register a handler for the test message on the ForwardService
	json.Register(suite.ch, json.Handlers{
		"/test": testReplicateHandler,
	}, onReplicateError)

	// Listen for incoming requests
	replicaListenAndHandle(suite.ch, "127.0.0.1:10501")
}

// TearDownSuite is expected to cleanup after test
func (suite *ReplicateTestSuite) TearDownSuite() {
	suite.ringpop.Destroy()
	suite.ch.Close()
}

func (suite *ReplicateTestSuite) TestBasicReplicate() {
	t := suite.T()
	incarnation := unixMilliseconds(time.Now())

	ringpop := suite.ringpop
	ringpop.membership.makeAlive("127.0.0.1:3001", incarnation)
	ringpop.membership.makeAlive("127.0.0.1:3001", incarnation)

	ringpop.ring.addServer("server1")
	ringpop.ring.addServer("server2")
	ringpop.ring.addServer("server3")

	// Instantiate the replicator
	repl := newReplicator(ringpop, 3, 1, 2)

	// Setup the replica options
	header := forwardReqHeader{
		HostPort:  "127.0.0.1:10501",
		Service:   "replicatorService",
		Operation: "/test",
		Checksum:  ringpop.membership.checksum,
		Keys:      []string{"key"},
	}

	req := forwardReq{
		Header: header,
		Body:   []byte("hello"),
	}

	keys := []string{"key"}
	rOpts := newReplicaOpts(keys, &req, 1, 2, 3, 30)
	resp, _ := repl.read(rOpts)
	assert.Equal(t, len(resp.keys), rOpts.nReplicas)
}

func (suite *ReplicateTestSuite) TestReplicateInvalid() {
	t := suite.T()
	incarnation := unixMilliseconds(time.Now())

	ringpop := suite.ringpop
	ringpop.membership.makeAlive("127.0.0.1:3001", incarnation)
	ringpop.membership.makeAlive("127.0.0.1:3001", incarnation)

	// Instantiate the replicator
	repl := newReplicator(ringpop, 3, 1, 2)

	// Setup the invalid replica options
	header := forwardReqHeader{
		HostPort:  "127.0.0.1:3001",
		Service:   "replicatorService",
		Operation: "/test",
		Checksum:  ringpop.membership.checksum,
		Keys:      []string{"key"},
	}

	req := forwardReq{
		Header: header,
		Body:   []byte("hello"),
	}

	// No keys => not enough replicas
	var keys []string
	rOpts := newReplicaOpts(keys, &req, 1, 2, 3, 30)
	resp, err := repl.read(rOpts)
	assert.Nil(t, resp)
	assert.NotNil(t, err)

	// Invalid rwvalue
	rOpts.wValue = 6
	rOpts.rValue = 7
	resp, err = repl.write(rOpts)
	assert.Nil(t, resp)
	assert.NotNil(t, err)

	// the number of avaliable replicas less than nValue
	ringpop.ring.removeServer("server1")
	ringpop.ring.removeServer("server2")
	ringpop.ring.removeServer("server3")

	rOpts.nReplicas = 0
	rOpts.wValue = 2
	rOpts.rValue = 1

	resp, err = repl.read(rOpts)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestRunReplicatorSuite(t *testing.T) {
	suiteReplicateTester := new(ReplicateTestSuite)
	suite.Run(t, suiteReplicateTester)
}
