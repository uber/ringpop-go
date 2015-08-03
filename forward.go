package ringpop

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
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
	if opts["endpoint"] != nil {
		endpoint = opts["endpoint"].(string)
	} else {
		endpoint = "/forward/req"
	}

	var timeout time.Duration
	if opts["timeout"] != nil {
		temp, _ := strconv.Atoi(opts["timeout"].(string))
		timeout = time.Duration(temp)
	} else {
		timeout = ringpop.forwardReqTimeout
	}

	cOpts := newChannelOpts(dest, keys, endpoint, timeout)

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
	var resp *http.Response

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

	// send the http request
	hclient := &http.Client{}

	resp, err = hclient.Get(pReq.Header.URL)
	if err != nil {
		res.StatusCode = 500
		return res, err
	}
	// Make sure the conneciton is closed
	defer resp.Body.Close()

	ringpop.logger.WithFields(log.Fields{
		"local": ringpop.WhoAmI(),
		"isOK":  err == nil,
	}).Debug("[ringpop] forward-req complete")

	jHeaders, _ := json.Marshal(headers)
	resBody := forwardReqRes{
		StatusCode: resp.StatusCode,
		Headers:    string(jHeaders),
	}

	return resBody, err
}
