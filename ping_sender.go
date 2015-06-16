package ringpop

import (
	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
)

func sendPing(ringpop *Ringpop, target string) (*pingBody, error) {
	// TODO
	ringpop.stat("increment", "ping.send", 1)

	pingsender := newPinger(ringpop, target)
	return pingsender.send()
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
}

func newPinger(ringpop *Ringpop, address string) *pinger {
	pinger := &pinger{
		ringpop: ringpop,
		address: address,
	}

	return pinger
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (p *pinger) send() (*pingBody, error) {
	// changes := p.ringpop.dissemination.issueChanges()
	var changes []Change

	ctx, cancel := context.WithTimeout(context.Background(), p.ringpop.pingTimeout)
	defer cancel()

	// begin call
	call, err := p.ringpop.channel.BeginCall(ctx, p.address, "ringpop", "/protocol/ping", nil)
	if err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":  p.ringpop.WhoAmI(),
			"remote": p.address,
		}).Errorf("Could not begin call to remote ping service: %v", err)
		return nil, err
	}

	// send request
	var reqHeaders headers
	if err := tchannel.NewArgWriter(call.Arg2Writer()).WriteJSON(reqHeaders); err != nil {
		log.Errorf("Could not write headers: %v", err)
		return nil, err
	}

	changes = p.ringpop.dissemination.issueChanges(0, "")
	reqBody := pingBody{
		Checksum: p.ringpop.membership.checksum,
		Changes:  changes,
		Source:   p.ringpop.WhoAmI(),
	}
	if err := tchannel.NewArgWriter(call.Arg3Writer()).WriteJSON(reqBody); err != nil {
		log.Errorf("Could not write ping body: %v", err)
		return nil, err
	}

	// get response
	var resHeaders headers
	if err := tchannel.NewArgReader(call.Response().Arg2Reader()).ReadJSON(&resHeaders); err != nil {
		log.Errorf("Could not read response headers: %v", err)
		return nil, err
	}

	var resBody pingBody
	if err := tchannel.NewArgReader(call.Response().Arg3Reader()).ReadJSON(&resBody); err != nil {
		log.Errorf("Could not read response body: %v", err)
		return nil, err
	}

	return &resBody, nil
}
