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
//	REPLICATE TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
// ReplicateTestSuite has a recording server used by all the tests
type ReplicateTestSuite struct {
	suite.Suite
	ringpop    *Ringpop
	testServer *xhttptest.RecordingServer
}

/*
 * SetupSuite instantiates a test server with a valid route to /test/myReplicate
 * This also sets up a testPop to be used by the tests
 */
func (suite *ReplicateTestSuite) SetupSuite() {
	suite.testServer = xhttptest.NewRecordingServer()
	suite.testServer.Router.AddPatternRoute("/test/myReplicate",
		xhttp.ServeContentHandler("", time.Time{}, strings.NewReader("My Text")))
	suite.testServer.Router.AddPatternRoute("/error", xhttp.ErrorHandler("no such topic", 420))
	suite.ringpop = testPop("127.0.0.1:3000", unixMilliseconds(time.Now()), nil)
}

// TearDownSuite is expected to close the test server
func (suite *ReplicateTestSuite) TearDownSuite() {
	suite.ringpop.Destroy()
	suite.testServer.Close()
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
		URL:      "127.0.0.1:3001",
		Checksum: ringpop.membership.checksum,
		Keys:     []string{"key"},
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
		URL:      "127.0.0.1:3001",
		Checksum: ringpop.membership.checksum,
		Keys:     []string{"key"},
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
