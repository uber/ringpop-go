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

type proxyReqErr struct {
	badProxy bool
	err      error
}

type proxyReqHeader struct {
	URL      string   `json:"url"`
	Checksum uint32   `json:"checksum"`
	Keys     []string `json:"keys"`
}

type proxyReqBody struct {
	body []byte
}

type proxyReq struct {
	Header proxyReqHeader
	Body   proxyReqBody
}

type proxyReqRes struct {
	StatusCode int    `json:"statusCode"`
	Headers    string `json:"headers"`
}

func (p *proxyReqErr) Error() string {
	return p.err.Error()
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// PROXY REQ SENDER
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
type channelOpts struct {
	keys     []string
	host     string
	timeout  time.Duration
	endpoint string
}

type proxyOpts struct {
	channelOpts *channelOpts
	req         *proxyReq
	res         *proxyReqRes
}

type proxyReqSender struct {
	ringpop        *Ringpop
	cOpts          *channelOpts
	req            *proxyReq
	numRetries     int
	reqStartTime   time.Duration
	retryStartTime time.Time
	timeout        time.Duration
}

func newProxyReqSender(ringpop *Ringpop, opts *channelOpts, timeout time.Duration, req *proxyReq) *proxyReqSender {
	p := &proxyReqSender{
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

func newProxyReqHeader(host string, keys []string, checksum uint32) *proxyReqHeader {
	p := &proxyReqHeader{
		URL:      host,
		Checksum: checksum,
	}
	p.Keys = make([]string, len(keys))
	copy(p.Keys, keys)

	return p
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// PROXY REQ SENDER METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (p *proxyReqSender) rerouteRetry(dest string) (proxyReqRes, error) {
	p.ringpop.logger.WithFields(log.Fields{
		"local":  p.ringpop.WhoAmI(),
		"remote": dest,
	}).Debug("[ringpop] request proxy rerouted")

	p.ringpop.emit("requestProxy.retryRerouted")

	if dest == p.ringpop.WhoAmI() {
		p.ringpop.stat("increment", "requestProxy.retry.reroute.local", 1)
		return handleProxyRequest(
			p.ringpop,
			newProxyReqHeader(p.cOpts.host, p.cOpts.keys, p.ringpop.membership.checksum),
			p.req), nil
	}

	p.ringpop.stat("increment", "requestProxy.retry.reroute.remote", 1)

	pnew := newChannelOpts(dest, p.cOpts.keys, p.cOpts.endpoint, p.cOpts.timeout)

	return sendProxyRequest(p.ringpop, pnew, p.req)
}

func (p *proxyReqSender) lookupKeys(keys []string) []string {
	var dests []string

	for _, key := range keys {
		val, _ := p.ringpop.ring.lookup(key)
		dests = append(dests, val)
	}
	return dests
}

func (p *proxyReqSender) attemptRetry(errC chan<- error) {
	p.numRetries++

	var err error
	dests := p.lookupKeys(p.cOpts.keys)
	if len(dests) > 1 || len(dests) == 0 {
		p.ringpop.logger.WithFields(log.Fields{
			"local": p.ringpop.WhoAmI(),
			"host":  p.cOpts.host,
		}).Debug("[ringpop] request proxy retry aborted")
		p.ringpop.emit("requestProxy.retryAborted")
		err = errors.New("retry aborted")
		errC <- err
		return
	}
	p.ringpop.stat("increment", "requestProxy.retry.attempted", 1)
	p.ringpop.emit("requestProxy.retryAttempted")

	newDest := dests[0]

	// If nothing rebalanced, just try sending once again
	if newDest == p.cOpts.host {
		_, err = p.sendProxyReq()
		errC <- err
		return
	}

	// the looked up key didn't match with the host => reroute
	_, err = p.rerouteRetry(newDest)
	errC <- err
	return
}

func (p *proxyReqSender) scheduleRetry(errC chan<- error) {
	if p.numRetries == 0 {
		p.retryStartTime = time.Now()
	}
	attemptErrC := make(chan error)

	// sleep for the specified delay time
	delay := p.ringpop.proxyRetrySchedule[p.numRetries]

	time.Sleep(delay)

	go p.attemptRetry(attemptErrC)
	p.ringpop.emit("requestProxy.retryScheduled")

	select {
	case err := <-attemptErrC: // proxy-req succeeded or failed
		errC <- err
		return
	}
}

func (p *proxyReqSender) sendProxyReq() (proxyReqRes, error) {
	ctx, cancel := json.NewContext(p.timeout)
	defer cancel()

	headers := newProxyReqHeader(p.cOpts.host, p.cOpts.keys, p.ringpop.membership.checksum)
	ctx = json.WithHeaders(ctx, headers)

	var err error
	errC := make(chan error)

	var resBody proxyReqRes

	// send proxy-req
	go p.send(ctx, &resBody, errC)
	// wait for response
	select {
	case err = <-errC: // proxy-req succeeded or failed
		if err != nil {
			if p.numRetries < p.ringpop.proxyMaxRetries {
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

	case <-ctx.Done(): // proxy-req timed out
		return resBody, errors.New("proxy-req timed out")
	}
}

func (p *proxyReqSender) send(ctx json.Context, resBody *proxyReqRes, errC chan<- error) {
	defer close(errC)
	peer := p.ringpop.channel.Peers().GetOrAdd(p.cOpts.host)

	err := json.CallPeer(ctx, peer, "ringpop", "/proxy/req", p.req, resBody)
	if err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":   p.ringpop.WhoAmI(),
			"host":    p.cOpts.host,
			"service": "proxy-req",
			"error":   err,
		}).Debug("[ringpop] proxy-req failed")

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

func sendProxyRequest(ringpop *Ringpop, opts *channelOpts, reqBody *proxyReq) (proxyReqRes, error) {
	var err error
	var res proxyReqRes

	go func() {
		p := newProxyReqSender(ringpop, opts, ringpop.proxyReqTimeout, reqBody)

		p.ringpop.logger.WithFields(log.Fields{
			"local": p.ringpop.WhoAmI(),
			"opts":  p.cOpts,
		}).Debug("[ringpop] proxy-req send")

		res, err = p.sendProxyReq()
	}()
	if err != nil {
		ringpop.logger.WithFields(log.Fields{
			"statusCode": res.StatusCode,
			"err":        err,
		}).Warn("[ringpop] proxy-req response received")
	}

	return res, err
}
