package ringpop

import (
	"errors"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// HELPER STRUCTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type pingBody struct {
	Checksum uint32   `json:"checksum"`
	Changes  []Change `json:"changes"`
	Source   string   `json:"source"`
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// PING SENDER
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type pingSender struct {
	ringpop *Ringpop
	address string
	timeout time.Duration
}

func newPingSender(ringpop *Ringpop, address string, timeout time.Duration) *pingSender {
	pingSender := &pingSender{
		ringpop: ringpop,
		address: address,
		timeout: timeout,
	}

	return pingSender
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// PING SENDER METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (p *pingSender) sendPing() (*pingBody, error) {
	ctx, cancel := tchannel.NewContext(p.timeout)
	defer cancel()

	errC := make(chan error)
	defer close(errC)

	var resBody pingBody

	// send ping
	go p.send(ctx, &resBody, errC)
	// wait for response
	select {
	case err := <-errC: // ping succeeded or failed
		if err != nil {
			return nil, err
		}
		return &resBody, nil

	case <-ctx.Done(): // ping timed out
		return nil, errors.New("ping timed out")
	}
}

func (p *pingSender) send(ctx context.Context, resBody *pingBody, errC chan error) {
	// begin call
	call, err := p.ringpop.channel.BeginCall(ctx, p.address, "ringpop", "/protocol/ping", nil)
	if err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":   p.ringpop.WhoAmI(),
			"remote":  p.address,
			"service": "ping-send",
			"error":   err,
		}).Debug("[ringpop] could not begin call")
		errC <- err
		return
	}

	// send request
	var reqHeaders headers
	if err := tchannel.NewArgWriter(call.Arg2Writer()).WriteJSON(reqHeaders); err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":   p.ringpop.WhoAmI(),
			"error":   err,
			"service": "ping-send",
		}).Debug("[ringpop] could not write request headers")
		errC <- err
		return
	}

	reqBody := pingBody{
		Checksum: p.ringpop.membership.checksum,
		Changes:  p.ringpop.dissemination.issueChanges(0, ""),
		Source:   p.ringpop.WhoAmI(),
	}

	if err := tchannel.NewArgWriter(call.Arg3Writer()).WriteJSON(reqBody); err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":   p.ringpop.WhoAmI(),
			"error":   err,
			"service": "ping-send",
		}).Debug("[ringpop] could not write request body")
		errC <- err
		return
	}

	// get response
	var resHeaders headers
	if err := tchannel.NewArgReader(call.Response().Arg2Reader()).ReadJSON(&resHeaders); err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":   p.ringpop.WhoAmI(),
			"error":   err,
			"service": "ping-send",
		}).Debug("[ringpop] could not read response body")
		errC <- err
		return
	}

	if err := tchannel.NewArgReader(call.Response().Arg3Reader()).ReadJSON(&resBody); err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":   p.ringpop.WhoAmI(),
			"error":   err,
			"service": "ping-send",
		}).Debug("[ringpop] could not read response body")
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

func sendPing(ringpop *Ringpop, target string, timeout time.Duration) (*pingBody, error) {
	ringpop.stat("increment", "ping.send", 1)
	ringpop.logger.WithFields(log.Fields{
		"local":  ringpop.WhoAmI(),
		"target": target,
	}).Debug("[ringpop] ping send")

	pingsender := newPingSender(ringpop, target, timeout)
	return pingsender.sendPing()
}
