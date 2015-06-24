package ringpop

import (
	"errors"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
)

func sendPing(ringpop *Ringpop, target string) (*pingBody, error) {
	ringpop.stat("increment", "ping.send", 1)
	pingsender := newPinger(ringpop, target)
	return pingsender.sendPing()
}

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

type pinger struct {
	ringpop *Ringpop
	address string
	timeout time.Duration
}

func newPinger(ringpop *Ringpop, address string) *pinger {
	pinger := &pinger{
		ringpop: ringpop,
		address: address,
		timeout: ringpop.pingTimeout,
	}

	return pinger
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (p *pinger) sendPing() (*pingBody, error) {
	ctx, cancel := context.WithTimeout(tchannel.NewRootContext(context.Background()),
		p.timeout)
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

func (p *pinger) send(ctx context.Context, resBody *pingBody, errC chan error) {
	// begin call
	call, err := p.ringpop.channel.BeginCall(ctx, p.address, "ringpop", "/protocol/ping", nil)
	if err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":  p.ringpop.WhoAmI(),
			"remote": p.address,
		}).Debugf("could not begin call to remote ping service: %v", err)
		errC <- err
		return
	}

	// send request
	var reqHeaders headers
	if err := tchannel.NewArgWriter(call.Arg2Writer()).WriteJSON(reqHeaders); err != nil {
		log.Debugf("could not write headers: %v", err)
		errC <- err
		return
	}

	reqBody := pingBody{
		Checksum: p.ringpop.membership.checksum,
		Changes:  p.ringpop.dissemination.issueChanges(0, ""),
		Source:   p.ringpop.WhoAmI(),
	}

	if err := tchannel.NewArgWriter(call.Arg3Writer()).WriteJSON(reqBody); err != nil {
		log.Debugf("could not write ping body: %v", err)
		errC <- err
		return
	}

	// get response
	var resHeaders headers
	if err := tchannel.NewArgReader(call.Response().Arg2Reader()).ReadJSON(&resHeaders); err != nil {
		log.Debugf("could not read response headers: %v", err)
		errC <- err
		return
	}

	if err := tchannel.NewArgReader(call.Response().Arg3Reader()).ReadJSON(resBody); err != nil {
		log.Debugf("could not read response body: %v", err)
		errC <- err
		return
	}

	errC <- nil
}
