package ringpop

import (
	"errors"
	"time"

	log "github.com/uber/bark"
	"github.com/uber/tchannel/golang/json"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// HELPER STRUCTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type pingBody struct {
	Checksum          uint32   `json:"checksum"`
	Changes           []Change `json:"changes"`
	Source            string   `json:"source"`
	SourceIncarnation int64    `json:"sourceIncarnation"`
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
	// ctx, cancel := tchannel.NewContext(p.timeout)
	// defer cancel()

	ctx, cancel := json.NewContext(p.timeout)
	defer cancel()

	errC := make(chan error)

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

func (p *pingSender) send(ctx json.Context, resBody *pingBody, errC chan<- error) {
	defer close(errC)
	peer := p.ringpop.channel.Peers().GetOrAdd(p.address)

	reqBody := pingBody{
		Checksum:          p.ringpop.membership.checksum,
		Changes:           p.ringpop.dissemination.issueChangesAsSender(),
		Source:            p.ringpop.WhoAmI(),
		SourceIncarnation: p.ringpop.membership.localMember.Incarnation,
	}

	err := json.CallPeer(ctx, peer, "ringpop", "/protocol/ping", reqBody, resBody)
	if err != nil {
		p.ringpop.logger.WithFields(log.Fields{
			"local":   p.ringpop.WhoAmI(),
			"remote":  p.address,
			"service": "ping-send",
			"error":   err,
		}).Debug("[ringpop] ping failed")
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
