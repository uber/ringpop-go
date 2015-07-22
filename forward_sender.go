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

type forwardReqErr struct {
	badForward bool
	err        error
}

type forwardReqHeader struct {
	URL      string   `json:"url"`
	Checksum uint32   `json:"checksum"`
	Keys     []string `json:"keys"`
}

type forwardReqBody struct {
	body []byte
}

type forwardReq struct {
	Header forwardReqHeader
	Body   forwardReqBody
}

type forwardReqRes struct {
	StatusCode int    `json:"statusCode"`
	Headers    string `json:"headers"`
}

func (p *forwardReqErr) Error() string {
	return p.err.Error()
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// FORWARD REQ SENDER
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
type channelOpts struct {
	keys     []string
	host     string
	timeout  time.Duration
	endpoint string
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

func newChannelOpts(host string, keys []string, endpoint string, timeout time.Duration) *channelOpts {
	p := &channelOpts{
		host:     host,
		endpoint: endpoint,
		timeout:  timeout,
	}
	p.keys = make([]string, len(keys))
	copy(p.keys, keys)

	return p
}

func newForwardReqHeader(host string, keys []string, checksum uint32) *forwardReqHeader {
	p := &forwardReqHeader{
		URL:      host,
		Checksum: checksum,
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
		return handleForwardRequest(
			p.ringpop,
			newForwardReqHeader(p.cOpts.host, p.cOpts.keys, p.ringpop.membership.checksum),
			p.req), nil
	}

	p.ringpop.stat("increment", "requestForward.retry.reroute.remote", 1)

	pnew := newChannelOpts(dest, p.cOpts.keys, p.cOpts.endpoint, p.cOpts.timeout)

	return forwardRequest(p.ringpop, pnew, p.req)
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
		_, err = p.forwardReq()
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

func (p *forwardReqSender) forwardReq() (forwardReqRes, error) {
	ctx, cancel := json.NewContext(p.timeout)
	defer cancel()

	headers := newForwardReqHeader(p.cOpts.host, p.cOpts.keys, p.ringpop.membership.checksum)
	ctx = json.WithHeaders(ctx, headers)

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
		return resBody, err

	case <-time.After(p.timeout): // forward-req timed out
		return resBody, errors.New("forward-req timed out")
	}
}

func (p *forwardReqSender) send(ctx json.Context, resBody *forwardReqRes, errC chan<- error) {
	defer close(errC)
	peer := p.ringpop.channel.Peers().GetOrAdd(p.cOpts.host)

	err := json.CallPeer(ctx, peer, "ringpop", "/forward/req", p.req, resBody)
	if err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":   p.ringpop.WhoAmI(),
			"host":    p.cOpts.host,
			"service": "forward-req",
			"error":   err,
		}).Debug("[ringpop] forward-req failed")

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

	go func() {
		p := newForwardReqSender(ringpop, opts, ringpop.forwardReqTimeout, reqBody)

		p.ringpop.logger.WithFields(log.Fields{
			"local": p.ringpop.WhoAmI(),
			"opts":  p.cOpts,
		}).Debug("[ringpop] forward-req send")

		res, err = p.forwardReq()
	}()
	if err != nil {
		ringpop.logger.WithFields(log.Fields{
			"statusCode": res.StatusCode,
			"err":        err,
		}).Warn("[ringpop] forward-req response received")
	}

	return res, err
}
