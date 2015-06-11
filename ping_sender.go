package ringpop

import (
	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
)

func sendPing(ringpop *Ringpop, target string) (*pingBody, error) {
	// TODO
	ringpop.stat("increment", "ping.send", 1)

	pingsender := newPingSender(ringpop, target)
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

type pingSender struct {
	ringpop *Ringpop
	address string
}

func newPingSender(ringpop *Ringpop, address string) *pingSender {
	pingsender := &pingSender{
		ringpop: ringpop,
		address: address,
	}

	return pingsender
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (ps *pingSender) send() (*pingBody, error) {
	// changes := ps.ringpop.dissemination.issueChanges()
	var changes []Change

	body := pingBody{
		Checksum: ps.ringpop.membership.checksum,
		Changes:  changes,
		Source:   ps.ringpop.WhoAmI(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), ps.ringpop.pingTimeout)
	defer cancel()

	call, err := ps.ringpop.channel.BeginCall(ctx, ps.address, "protocol", "ping", nil)
	if err != nil {
		ps.ringpop.logger.WithFields(log.Fields{
			"local":  ps.ringpop.WhoAmI(),
			"remote": ps.address,
		}).Errorf("Could not begin call to remote ping service: %v", err)
		return nil, err
	}

	// Make call
	if err := call.WriteArg2(tchannel.NewJSONOutput(headers{})); err != nil {
		log.Errorf("Could not write headers: %v", err)
		return nil, err
	}

	if err := call.WriteArg3(tchannel.NewJSONOutput(body)); err != nil {
		log.Errorf("Could not write ping body: %v", err)
		return nil, err
	}

	// Get response
	var resHeaders headers
	if err := call.Response().ReadArg2(tchannel.NewJSONInput(&resHeaders)); err != nil {
		log.Errorf("Could not read response headers: %v", err)
		return nil, err
	}

	var res pingBody
	if err := call.Response().ReadArg3(tchannel.NewJSONInput(&res)); err != nil {
		log.Errorf("Could not read response body: %v", err)
		return nil, err
	}

	return &res, nil
}
