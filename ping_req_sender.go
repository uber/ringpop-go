package ringpop

import (
	"errors"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang/json"
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
	Source            string   `json:"source"`
	SourceIncarnation int64    `json:"sourceIncarnation"`
	Target            string   `json:"target"`
	Checksum          uint32   `json:"checksum"`
	Changes           []Change `json:"changes"`
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
	ctx, cancel := json.NewContext(p.timeout)
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

func (p *pingReqSender) send(ctx json.Context, resBody *pingReqRes, errC chan<- error) {
	defer close(errC)
	peer := p.ringpop.channel.Peers().GetOrAdd(p.address)

	reqBody := pingReqBody{
		Checksum:          p.ringpop.membership.checksum,
		Changes:           p.ringpop.dissemination.issueChangesAsSender(),
		Source:            p.ringpop.WhoAmI(),
		SourceIncarnation: p.ringpop.membership.localMember.Incarnation,
		Target:            p.target,
	}

	err := json.CallPeer(ctx, peer, "ringpop", "/protocol/ping-req", reqBody, resBody)
	if err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":   p.ringpop.WhoAmI(),
			"remote":  p.address,
			"service": "ping-req-send",
			"error":   err,
		}).Debug("[ringpop] ping-req failed")
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
