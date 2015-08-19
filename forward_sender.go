package ringpop

import (
	"errors"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang/json"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// HELPER STRUCTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type forwardReqHeader struct {
	HostPort  string   `json:"HostPort"`
	Service   string   `json:"Service"`
	Operation string   `json:"Operation"`
	Checksum  uint32   `json:"Checksum"`
	Keys      []string `json:"Keys"`
}

type forwardReq struct {
	Header forwardReqHeader
	Body   []byte
}

type forwardReqRes struct {
	StatusCode int    `json:"statusCode"`
	Headers    string `json:"headers"`
	Body       []byte `json:"body"`
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// FORWARD REQ SENDER
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
type channelOpts struct {
	keys        []string
	host        string
	destService string
	timeout     time.Duration
	endpoint    string
}

type forwardReqSender struct {
	ringpop        *Ringpop
	cOpts          *channelOpts
	req            *forwardReq
	numRetries     int
	reqStartTime   time.Duration
	retryStartTime time.Time
	timeout        time.Duration
}

func newForwardReqSender(ringpop *Ringpop, opts *channelOpts, timeout time.Duration, req *forwardReq) *forwardReqSender {
	p := &forwardReqSender{
		ringpop: ringpop,
		cOpts:   opts,
		req:     req,
		timeout: timeout,
	}
	return p
}

func newChannelOpts(host string, destService string, keys []string, endpoint string, timeout time.Duration) *channelOpts {
	p := &channelOpts{
		host:        host,
		destService: destService,
		endpoint:    endpoint,
		timeout:     timeout,
	}
	p.keys = make([]string, len(keys))
	copy(p.keys, keys)

	return p
}

func newForwardReqHeader(hostport string, service string, opName string, keys []string, checksum uint32) *forwardReqHeader {
	p := &forwardReqHeader{
		HostPort:  hostport,
		Service:   service,
		Operation: opName,
		Checksum:  checksum,
	}
	p.Keys = make([]string, len(keys))
	copy(p.Keys, keys)

	return p
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// FORWARD REQ SENDER METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (p *forwardReqSender) rerouteRetry(dest string) (forwardReqRes, error) {
	p.ringpop.logger.WithFields(log.Fields{
		"local":  p.ringpop.WhoAmI(),
		"remote": dest,
	}).Debug("[ringpop] request forward rerouted")

	p.ringpop.emit("requestForward.retryRerouted")

	if dest == p.ringpop.WhoAmI() {
		p.ringpop.stat("increment", "requestForward.retry.reroute.local", 1)
		resp, err := handleForwardRequest(
			p.ringpop,
			&p.req.Header,
			p.req)
		return resp, err
	}

	p.ringpop.stat("increment", "requestForward.retry.reroute.remote", 1)

	// The channel options should be updated
	newCopts := newChannelOpts(dest, p.cOpts.destService, p.cOpts.keys, p.cOpts.endpoint, p.cOpts.timeout)

	var res forwardReqRes
	var err error
	p.cOpts = newCopts
	doneC := make(chan bool)
	go func() {
		res, err = p.forwardReq(doneC)
	}()

	<-doneC
	return res, err
}

func (p *forwardReqSender) lookupKeys(keys []string) []string {
	var dests []string

	for _, key := range keys {
		val, _ := p.ringpop.ring.lookup(key)
		dests = append(dests, val)
	}
	return dests
}

func (p *forwardReqSender) attemptRetry(errC chan<- error) {
	p.numRetries++

	var err error
	dests := p.lookupKeys(p.cOpts.keys)
	if len(dests) > 1 || len(dests) == 0 {
		p.ringpop.logger.WithFields(log.Fields{
			"local": p.ringpop.WhoAmI(),
			"host":  p.cOpts.host,
		}).Debug("[ringpop] request forward retry aborted")
		p.ringpop.emit("requestForward.retryAborted")
		err = errors.New("retry aborted")
		errC <- err
		return
	}
	p.ringpop.stat("increment", "requestForward.retry.attempted", 1)
	p.ringpop.emit("requestForward.retryAttempted")

	newDest := dests[0]
	// If nothing rebalanced, just try sending once again
	if newDest == p.cOpts.host {
		done := make(chan bool)
		go func() {
			_, err = p.forwardReq(done)
		}()

		<-done
		errC <- err
		return
	}

	// the looked up key didn't match with the host => reroute
	_, err = p.rerouteRetry(newDest)
	errC <- err
	return
}

func (p *forwardReqSender) scheduleRetry(errC chan<- error) {
	if p.numRetries == 0 {
		p.retryStartTime = time.Now()
	}
	attemptErrC := make(chan error)

	// sleep for the specified delay time
	delay := p.ringpop.forwardRetrySchedule[p.numRetries]

	time.Sleep(delay)

	go p.attemptRetry(attemptErrC)
	p.ringpop.emit("requestForward.retryScheduled")

	select {
	case err := <-attemptErrC: // forward-req succeeded or failed
		errC <- err
		return
	}
}

func (p *forwardReqSender) forwardReq(doneC chan<- bool) (forwardReqRes, error) {
	ctx, cancel := json.NewContext(p.timeout)
	defer cancel()

	ctx = json.WithHeaders(ctx, p.req.Header)

	var err error
	errC := make(chan error)

	var resBody forwardReqRes

	// send forward-req
	go p.send(ctx, &resBody, errC)

	// wait for response
	select {
	case err = <-errC: // forward-req succeeded or failed
		if err != nil {
			if p.numRetries < p.ringpop.forwardMaxRetries {
				retryErr := make(chan error)
				go p.scheduleRetry(retryErr)

				select {
				case err = <-retryErr:
					break
				}
			} else {
				p.ringpop.logger.WithFields(log.Fields{
					"local": p.ringpop.WhoAmI(),
					"host":  p.cOpts.host,
				}).Warn("[ringpop] Max retries exceeded")
			}
		}
		doneC <- true
		return resBody, err

	case <-time.After(p.timeout): // forward-req timed out
		doneC <- true
		return resBody, errors.New("forward-req timed out")
	}
}

func (p *forwardReqSender) send(ctx json.Context, resBody *forwardReqRes, errC chan<- error) {
	defer close(errC)

	peer := p.ringpop.GetOrAddPeer(p.cOpts.host)

	// send over tchannel to the appropriate service and endpoint
	err := json.CallPeer(ctx, peer, p.cOpts.destService, p.cOpts.endpoint, p.req, resBody)
	if err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":   p.ringpop.WhoAmI(),
			"host":    p.cOpts.host,
			"service": p.cOpts.destService,
			"error":   err,
		}).Debug("[ringpop] forward-req failed")

		resBody.StatusCode = 500
		errC <- err
		return
	}

	errC <- nil
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func forwardRequest(ringpop *Ringpop, opts *channelOpts, reqBody *forwardReq) (forwardReqRes, error) {
	var err error
	var res forwardReqRes
	doneC := make(chan bool)

	go func() {
		p := newForwardReqSender(ringpop, opts, ringpop.forwardReqTimeout, reqBody)

		p.ringpop.logger.WithFields(log.Fields{
			"local": p.ringpop.WhoAmI(),
			"opts":  p.cOpts,
		}).Debug("[ringpop] forward-req send")

		res, err = p.forwardReq(doneC)
	}()

	// Wait for the forward request to complete
	<-doneC
	if err != nil {
		ringpop.logger.WithFields(log.Fields{
			"statusCode": res.StatusCode,
			"err":        err,
		}).Warn("[ringpop] forward-req response received")
	}

	return res, err
}
