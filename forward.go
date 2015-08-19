package ringpop

import (
	"errors"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang/json"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	REQUEST FORWARD
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type forwarder struct {
	ringpop       *Ringpop
	retrySchedule []time.Duration
	maxRetries    int
}

func newForwarder(ringpop *Ringpop, retrySchedule []time.Duration, maxRetries int) *forwarder {
	p := &forwarder{
		ringpop:       ringpop,
		retrySchedule: retrySchedule,
		maxRetries:    maxRetries,
	}

	return p
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (p *forwarder) forwardRequest(opts map[string]interface{}) error {
	var ringpop = p.ringpop
	var dest = opts["dest"].(string)
	var req = opts["req"].(*forwardReq)
	var keys = opts["keys"].([]string)
	var res = opts["res"].(*forwardReqRes)
	var err error
	var endpoint string
	var destService string

	// If the destination endpoint and the service are given, use them to relay
	// over tchannel directly
	if opts["endpoint"] != nil {
		endpoint = opts["endpoint"].(string)
	} else {
		endpoint = "/forward/req"
	}

	if opts["service"] != nil {
		destService = opts["service"].(string)
	} else {
		destService = "ringpop"
	}

	var timeout time.Duration
	if opts["timeout"] != nil {
		temp, _ := strconv.Atoi(opts["timeout"].(string))
		timeout = time.Duration(temp)
	} else {
		timeout = ringpop.forwardReqTimeout
	}

	cOpts := newChannelOpts(dest, destService, keys, endpoint, timeout)

	ringpop.logger.WithFields(log.Fields{
		"local":  ringpop.WhoAmI(),
		"target": cOpts.host,
	}).Debug("[ringpop] forward-req recieve")

	*res, err = forwardRequest(ringpop, cOpts, req)

	ringpop.logger.WithFields(log.Fields{
		"local": ringpop.WhoAmI(),
		"isOK":  err == nil,
	}).Debug("[ringpop] forward-req complete")

	return err
}

func handleForwardRequest(ringpop *Ringpop, headers *forwardReqHeader, pReq *forwardReq) (forwardReqRes, error) {
	ringpop.stat("increment", "forward-req", 1)

	var res forwardReqRes
	var err error

	checksum := headers.Checksum

	if checksum != ringpop.membership.checksum {
		ringpop.logger.WithFields(log.Fields{
			"local":             ringpop.WhoAmI(),
			"expected checksum": ringpop.membership.checksum,
			"actual":            checksum,
		}).Debug("[ringpop] forward-req checksums differ")

		ringpop.emit("forwardRequest.checksumsDiffer")
		res.StatusCode = 500
		err = errors.New("checksums differ")
		return res, err
	}

	// Relay the request over tchanne to a local handlerl
	ctx, cancel := json.NewContext(ringpop.forwardReqTimeout)
	defer cancel()

	// Relay this directly via tchannel to the appropriate end point
	peer := ringpop.GetOrAddPeer(pReq.Header.HostPort)

	if err := json.CallPeer(ctx, peer, pReq.Header.Service, pReq.Header.Operation, pReq,
		&res); err != nil {
		log.Fatalf("json.Call failed: %v", err)
	}

	ringpop.logger.WithFields(log.Fields{
		"local": ringpop.WhoAmI(),
		"isOK":  err == nil,
	}).Debug("[ringpop] forward-req complete")

	return res, err
}
