package ringpop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	REQUEST PROXY
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type proxy struct {
	ringpop       *Ringpop
	retrySchedule []time.Duration
	maxRetries    int
	keys          []string
	endpoint      string
}

func newProxy(ringpop *Ringpop, retrySchedule []time.Duration, maxRetries int) *proxy {
	p := &proxy{
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

func (p *proxy) proxyRequest(opts map[string]interface{}) error {
	var ringpop = p.ringpop
	var dest = opts["dest"].(string)
	var req = opts["req"].(*proxyReq)
	var keys = opts["keys"].([]string)

	var endpoint string
	if opts["endpoint"] != nil {
		endpoint = opts["endpoint"].(string)
	} else {
		endpoint = "/proxy/req"
	}

	var timeout time.Duration
	if opts["timeout"] != nil {
		temp, _ := strconv.Atoi(opts["timeout"].(string))
		timeout = time.Duration(temp)
	} else {
		timeout = ringpop.proxyReqTimeout
	}

	cOpts := newChannelOpts(dest, keys, endpoint, timeout)

	ringpop.logger.WithFields(log.Fields{
		"local":  ringpop.WhoAmI(),
		"target": cOpts.host,
	}).Debug("[ringpop] proxy-req recieve")

	_, err := sendProxyRequest(ringpop, cOpts, req)

	ringpop.logger.WithFields(log.Fields{
		"local": ringpop.WhoAmI(),
		"isOK":  err == nil,
	}).Debug("[ringpop] proxy-req complete")

	return err
}

// TODO: Revisit and Fix this once we have some tests
func handleProxyRequest(ringpop *Ringpop, headers *proxyReqHeader, pReq *proxyReq) proxyReqRes {
	ringpop.stat("increment", "proxy-req", 1)

	var res proxyReqRes
	var err error
	var resp *http.Response

	checksum := headers.Checksum

	if checksum != ringpop.membership.checksum {
		ringpop.logger.WithFields(log.Fields{
			"local": ringpop.WhoAmI(),
		}).Debug("[ringpop] proxy-req checksums differ")
		return res
	}

	// send the http request
	hclient := &http.Client{}

	resp, err = hclient.Get(pReq.Header.URL)
	if err != nil {
		return res
	}

	bodyReader := bytes.NewBuffer(pReq.Body.body)
	req, err := http.NewRequest("GET", pReq.Header.URL, bodyReader)

	if err != nil {
		return res
	}

	resp, err = hclient.Do(req)
	if err != nil {
		return res
	}

	// Make sure the conneciton is closed
	defer resp.Body.Close()

	ringpop.logger.WithFields(log.Fields{
		"local": ringpop.WhoAmI(),
		"isOK":  err == nil,
	}).Debug("[ringpop] proxy-req complete")

	jHeaders, _ := json.Marshal(headers)
	resBody := proxyReqRes{
		StatusCode: resp.StatusCode,
		Headers:    string(jHeaders),
	}

	return resBody
}
