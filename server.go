package ringpop

import (
	"errors"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
	"golang.org/x/net/context"
)

type headers map[string]string

type ringpopServer struct {
	ringpop *Ringpop
	channel *tchannel.Channel
}

func newRingpopServer(ringpop *Ringpop) (*ringpopServer, error) {
	if ringpop.channel == nil {
		return nil, errors.New("ringpop channel cannot be nil")
	}

	ringpopServer := &ringpopServer{
		ringpop: ringpop,
		channel: ringpop.channel,
	}

	var commands = map[string]tchannel.HandlerFunc{
		// "/health":            ringpopServer.health,
		// "/admin/stats/":      ringpopServer.adminStats,
		// "/admin/debugSet":    ringpopServer.adminDebugSet,
		// "/admin/debugClear":  ringpopServer.adminDebugClear,
		// "/admin/gossip":      ringpopServer.adminGossip,
		// "/admin/leave":       ringpopServer.adminLeave,
		// "/admin/join":        ringpopServer.adminJoin,
		// "/admin/reload":      ringpopServer.adminReload,
		"/protocol/join":     ringpopServer.protocolJoinHandler,
		"/protocol/ping":     ringpopServer.protocolPingHandler,
		"/protocol/ping-req": ringpopServer.protocolPingReqHandler,
	}

	// Register endpoints with channel
	for operation, handler := range commands {
		ringpopServer.channel.Register(handler, operation)
	}

	return ringpopServer, nil
}

func (rc *ringpopServer) listenAndServe() error {
	return rc.channel.ListenAndServe(rc.ringpop.WhoAmI())
}

func (rc *ringpopServer) protocolJoinHandler(ctx context.Context, call *tchannel.InboundCall) {
	if rc.channel.Closed() {
		rc.ringpop.logger.WithField("local", rc.ringpop.WhoAmI()).
			Error("[ringpop] got call while channel closed!")
	}

	// receive request
	var reqHeaders headers
	if err := tchannel.NewArgReader(call.Arg2Reader()).ReadJSON(&reqHeaders); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "join-recv",
		}).Debug("[ringpop] could not read request headers")
		return
	}

	var reqBody joinBody
	if err := tchannel.NewArgReader(call.Arg3Reader()).ReadJSON(&reqBody); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "join-recv",
		}).Debug("[ringpop] could not read request body")
		return
	}

	// handle request and send back resposne
	var resHeaders headers
	if err := tchannel.NewArgWriter(call.Response().Arg2Writer()).WriteJSON(resHeaders); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "join-recv",
		}).Debug("[ringpop] could not write response headers")
		return
	}

	resBody, err := handleJoin(rc.ringpop, reqBody)
	if err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "join-recv",
		}).Debug("[ringpop] could not complete join")
		return
	}
	if err := tchannel.NewArgWriter(call.Response().Arg3Writer()).WriteJSON(resBody); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "join-recv",
		}).Debug("[ringpop] could not write response headers")
		return
	}
}

func (rc *ringpopServer) protocolPingHandler(ctx context.Context, call *tchannel.InboundCall) {
	if rc.channel.Closed() {
		rc.ringpop.logger.WithField("local", rc.ringpop.WhoAmI()).
			Error("[ringpop] got call while channel closed!")
	}

	// receive request
	var reqHeaders headers
	if err := tchannel.NewArgReader(call.Arg2Reader()).ReadJSON(&reqHeaders); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-recv",
		}).Debug("[ringpop] could not read request headers")
		return
	}

	var reqBody pingBody
	if err := tchannel.NewArgReader(call.Arg3Reader()).ReadJSON(&reqBody); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-recv",
		}).Debug("[ringpop] could not read request body")
		return
	}

	// handle request and send back response
	var resHeaders headers
	if err := tchannel.NewArgWriter(call.Response().Arg2Writer()).WriteJSON(resHeaders); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-recv",
		}).Debug("[ringpop] could not write response headers")

		return
	}

	resBody := handlePing(rc.ringpop, reqBody)
	if err := tchannel.NewArgWriter(call.Response().Arg3Writer()).WriteJSON(resBody); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-recv",
		}).Debug("[ringpop] could not write response body")
		return
	}
}

func (rc *ringpopServer) protocolPingReqHandler(ctx context.Context, call *tchannel.InboundCall) {
	if rc.channel.Closed() {
		rc.ringpop.logger.WithField("local", rc.ringpop.WhoAmI()).
			Error("[ringpop] got call while channel closed!")
	}

	// receive request
	var reqHeaders headers
	if err := tchannel.NewArgReader(call.Arg2Reader()).ReadJSON(&reqHeaders); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-req-recv",
		}).Debug("[ringpop] could not read request headers")
		return
	}

	var reqBody pingReqBody
	if err := tchannel.NewArgReader(call.Arg3Reader()).ReadJSON(&reqBody); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-req-recv",
		}).Debug("[ringpop] could not read request body")
		return
	}

	// handle request and send back response
	var resHeaders headers
	if err := tchannel.NewArgWriter(call.Response().Arg2Writer()).WriteJSON(resHeaders); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-req-recv",
		}).Debug("[ringpop] could not write response headers")
		return
	}

	resBody := handlePingReq(rc.ringpop, reqBody)
	if err := tchannel.NewArgWriter(call.Response().Arg3Writer()).WriteJSON(resBody); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-req-recv",
		}).Debug("[ringpop] could not write response body")
		return
	}
}
