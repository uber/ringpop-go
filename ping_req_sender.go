package ringpop

import (
	"errors"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"

	"golang.org/x/net/context"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// HELPER STRUCTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type pingReqErr struct {
	badPing bool
	err     error
}

type pingReqBody struct {
	Checksum uint32   `json:"checksum"`
	Source   string   `json:"source"`
	Target   string   `json:"target"`
	Changes  []Change `json:"changes"`
}

type pingReqRes struct {
	// Checksum   uint32   `json:"checksum"`
	// Source     string   `json:"source"`
	Target     string   `json:"target"`
	Changes    []Change `json:"changes"`
	PingStatus bool     `json:"pingStatus"`
}

func (p *pingReqErr) Error() string {
	return p.err.Error()
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// PING REQ SENDER
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type pingReqSender struct {
	ringpop *Ringpop
	address string
	target  string
	timeout time.Duration
}

func newPingReqSender(ringpop *Ringpop, address, target string, timeout time.Duration) *pingReqSender {
	p := &pingReqSender{
		ringpop: ringpop,
		address: address,
		target:  target,
		timeout: timeout,
	}

	return p
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// PING REQ SENDER METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (p *pingReqSender) sendPingReq() (pingReqRes, error) {
	ctx, cancel := tchannel.NewContext(p.timeout)
	defer cancel()

	errC := make(chan error)
	// defer close(errC)

	var resBody pingReqRes

	// send ping-req
	go p.send(ctx, &resBody, errC)
	// wait for response
	select {
	case err := <-errC: // ping-req succeeded or failed
		if err != nil {
			return resBody, err
		}
		return resBody, nil

	case <-ctx.Done(): // ping-req timed out
		return resBody, errors.New("ping-req timed out")
	}
}

func (p *pingReqSender) send(ctx context.Context, resBody *pingReqRes, errC chan<- error) {
	// begin call
	call, err := p.ringpop.channel.BeginCall(ctx, p.address, "ringpop", "/protocol/ping-req", nil)
	if err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":  p.ringpop.WhoAmI(),
			"remote": p.address,
			"error":  err,
		}).Debug("[ringpop] could not begins call to remote ping-req service")
		errC <- err
		return
	}

	// send request
	var reqHeaders headers
	if err := tchannel.NewArgWriter(call.Arg2Writer()).WriteJSON(reqHeaders); err != nil {
		log.WithField("error", err).Debugf("[ringpop] could not write request headers")
		errC <- err
		return
	}

	reqBody := pingReqBody{
		Checksum: p.ringpop.membership.checksum,
		Changes:  p.ringpop.dissemination.issueChanges(0, ""),
		Source:   p.ringpop.WhoAmI(),
		Target:   p.target,
	}
	if err := tchannel.NewArgWriter(call.Arg3Writer()).WriteJSON(reqBody); err != nil {
		log.WithField("error", err).Debugf("[ringpop] could not write request body")
		errC <- err
		return
	}

	// get response
	var resHeaders headers
	if err := tchannel.NewArgReader(call.Response().Arg2Reader()).ReadJSON(&resHeaders); err != nil {
		log.WithField("error", err).Debugf("[ringpop] could not read response headers")
		errC <- err
		return
	}

	if err := tchannel.NewArgReader(call.Response().Arg3Reader()).ReadJSON(resBody); err != nil {
		log.WithField("error", err).Debugf("[ringpop] could not read response body")
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

func sendPingReq(ringpop *Ringpop, peer, target string) {
	resC := make(chan *pingReqErr)

	go func() {
		p := newPingReqSender(ringpop, peer, target, ringpop.pingReqTimeout)
		// startingStatus := peer.Status

		p.ringpop.logger.WithFields(log.Fields{
			"local":  p.ringpop.WhoAmI(),
			"peer":   peer,
			"target": p.target,
		}).Debug("[ringpop] ping-req send")

		res, err := p.sendPingReq()
		if err != nil {
			// something interesting happened, but it wasn't the ping to remote member failing
			resC <- &pingReqErr{badPing: false, err: err}

		} else if res.PingStatus {
			p.ringpop.membership.update(res.Changes)
			resC <- nil
		} else {
			resC <- &pingReqErr{badPing: true, err: errors.New("remote ping failed")}
		}
	}()
	res := <-resC

	if res != nil {
		ringpop.logger.WithFields(log.Fields{
			"badPing": res.badPing,
			"err":     res.Error(),
		}).Warn("[ringpop] ping-req response received")
	}
}

func sendPingReqs(ringpop *Ringpop, target Member, size int) {
	pingReqMembers := ringpop.membership.randomPingablemembers(size, []string{target.Address})
	pingReqAddresses := make([]string, len(pingReqMembers))
	for i, member := range pingReqMembers {
		pingReqAddresses[i] = member.Address
	}

	if len(pingReqMembers) == 0 {
		return //errors.New("no selectable ping-req members")
	}

	var wg sync.WaitGroup
	resC := make(chan *pingReqErr, size)

	startTime := time.Now()

	// fan out ping-reqs
	for _, member := range pingReqMembers {
		wg.Add(1)
		go func(peer *Member) {
			p := newPingReqSender(ringpop, peer.Address, target.Address, ringpop.pingReqTimeout)
			// startingStatus := peer.Status

			p.ringpop.logger.WithFields(log.Fields{
				"local":  p.ringpop.WhoAmI(),
				"peer":   peer.Address,
				"target": p.target,
			}).Debug("[ringpop] ping-req send")

			res, err := p.sendPingReq()
			if err != nil {
				// something interesting happened, but it wasn't the ping to remote member failing
				resC <- &pingReqErr{badPing: false, err: err}

			} else if res.PingStatus {
				p.ringpop.membership.update(res.Changes)
				resC <- nil
			} else {
				resC <- &pingReqErr{badPing: true, err: errors.New("remote ping failed")}
			}
			wg.Done()
		}(member)
	}

	// wait for all ping-reqs to finish before closing response channel
	go func() {
		wg.Wait()
		close(resC)
	}()

	var errs []pingReqErr

	// wait for response(s)
	for err := range resC {
		// NOTE: if member is reachable it does not need to be explicitely
		// marked as alive, this already happens through the implicit exchange
		// of membership update on the ping-reqs and responses.
		if err == nil {
			ringpop.logger.WithFields(log.Fields{
				"local":             ringpop.WhoAmI(),
				"errors":            errs,
				"numErrors":         len(errs),
				"numPingReqMembers": len(pingReqMembers),
				"pingReqAddrs":      pingReqAddresses,
				"pingReqTime":       time.Now().Sub(startTime),
			}).Info("[ringpop] ping-req determined member is reachable")

			return
		}

		// wait for more responses
		errs = append(errs, *err)
	}

	pingReqBadPings := 0
	for _, err := range errs {
		if err.badPing {
			pingReqBadPings++
		}
	}

	// ping-req got a response from peer, wasn't a good one
	if pingReqBadPings > 0 {
		ringpop.logger.WithFields(log.Fields{
			"local":             ringpop.WhoAmI(),
			"target":            target.Address,
			"errors":            errs,
			"numErrors":         len(errs),
			"numPingReqMembers": len(pingReqMembers),
			"pingReqAddrs":      pingReqAddresses,
		}).Info("[ringpop] ping-req determined member is unreachable")

		ringpop.membership.makeSuspect(target.Address, target.Incarnation)
		return
	}

	ringpop.logger.WithFields(log.Fields{
		"local":              ringpop.WhoAmI(),
		"target":             target.Address,
		"errors":             errs,
		"numErrors":          len(errs),
		"numPingReqMemebers": len(pingReqMembers),
	}).Warn("[ringpop] ping-req inconclusive due to errors")

	return
}
