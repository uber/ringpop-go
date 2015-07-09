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
		// "/health":            ringpopServer.healthHandler,
		// "/admin/stats/":      ringpopServer.adminStatsHandler,
		"/admin/debugSet":   ringpopServer.adminDebugSetHandler,
		"/admin/debugClear": ringpopServer.adminDebugClearHandler,
		// "/admin/gossip":      ringpopServer.adminGossipHandler,
		// "/admin/leave":       ringpopServer.adminLeaveHandler,
		// "/admin/join":        ringpopServer.adminJoinHandler,
		// "/admin/reload":      ringpopServer.adminReloadHandler,
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

func (s *ringpopServer) listenAndServe() error {
	return s.channel.ListenAndServe(s.ringpop.WhoAmI())
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// PROTOCOL HANDLERS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (s *ringpopServer) protocolJoinHandler(ctx context.Context, call *tchannel.InboundCall) {
	if s.channel.Closed() {
		s.ringpop.logger.WithField("local", s.ringpop.WhoAmI()).
			Error("[ringpop] got call while channel closed!")
	}

	// receive request
	var reqHeaders headers
	if err := tchannel.NewArgReader(call.Arg2Reader()).ReadJSON(&reqHeaders); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "join-recv",
		}).Debug("[ringpop] could not read request headers")
		return
	}

	var reqBody joinBody
	if err := tchannel.NewArgReader(call.Arg3Reader()).ReadJSON(&reqBody); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "join-recv",
		}).Debug("[ringpop] could not read request body")
		return
	}

	// handle request and send back resposne
	var resHeaders headers
	if err := tchannel.NewArgWriter(call.Response().Arg2Writer()).WriteJSON(resHeaders); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "join-recv",
		}).Debug("[ringpop] could not write response headers")
		return
	}

	resBody, err := handleJoin(s.ringpop, reqBody)
	if err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "join-recv",
		}).Debug("[ringpop] could not complete join")
		return
	}
	if err := tchannel.NewArgWriter(call.Response().Arg3Writer()).WriteJSON(resBody); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "join-recv",
		}).Debug("[ringpop] could not write response headers")
		return
	}
}

func (s *ringpopServer) protocolPingHandler(ctx context.Context, call *tchannel.InboundCall) {
	if s.channel.Closed() {
		s.ringpop.logger.WithField("local", s.ringpop.WhoAmI()).
			Error("[ringpop] got call while channel closed!")
	}

	// receive request
	var reqHeaders headers
	if err := tchannel.NewArgReader(call.Arg2Reader()).ReadJSON(&reqHeaders); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-recv",
		}).Debug("[ringpop] could not read request headers")
		return
	}

	var reqBody pingBody
	if err := tchannel.NewArgReader(call.Arg3Reader()).ReadJSON(&reqBody); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-recv",
		}).Debug("[ringpop] could not read request body")
		return
	}

	// handle request and send back response
	var resHeaders headers
	if err := tchannel.NewArgWriter(call.Response().Arg2Writer()).WriteJSON(resHeaders); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-recv",
		}).Debug("[ringpop] could not write response headers")

		return
	}

	resBody := handlePing(s.ringpop, reqBody)
	if err := tchannel.NewArgWriter(call.Response().Arg3Writer()).WriteJSON(resBody); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-recv",
		}).Debug("[ringpop] could not write response body")
		return
	}
}

func (s *ringpopServer) protocolPingReqHandler(ctx context.Context, call *tchannel.InboundCall) {
	if s.channel.Closed() {
		s.ringpop.logger.WithField("local", s.ringpop.WhoAmI()).
			Error("[ringpop] got call while channel closed!")
	}

	// receive request
	var reqHeaders headers
	if err := tchannel.NewArgReader(call.Arg2Reader()).ReadJSON(&reqHeaders); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-req-recv",
		}).Debug("[ringpop] could not read request headers")
		return
	}

	var reqBody pingReqBody
	if err := tchannel.NewArgReader(call.Arg3Reader()).ReadJSON(&reqBody); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-req-recv",
		}).Debug("[ringpop] could not read request body")
		return
	}

	// handle request and send back response
	var resHeaders headers
	if err := tchannel.NewArgWriter(call.Response().Arg2Writer()).WriteJSON(resHeaders); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-req-recv",
		}).Debug("[ringpop] could not write response headers")
		return
	}

	resBody := handlePingReq(s.ringpop, reqBody)
	if err := tchannel.NewArgWriter(call.Response().Arg3Writer()).WriteJSON(resBody); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-req-recv",
		}).Debug("[ringpop] could not write response body")
		return
	}
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// ADMIN HANDLERS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func receiveCallNoArgs(s *ringpopServer, call *tchannel.InboundCall, endpoint string, f func()) {
	var reqHeaders []byte
	if err := tchannel.NewArgReader(call.Arg2Reader()).ReadJSON(&reqHeaders); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": endpoint,
		}).Debug("[ringpop] could not read request headers")
		return
	}

	var reqBody []byte
	if err := tchannel.NewArgReader(call.Arg3Reader()).Read(&reqBody); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": endpoint,
		}).Debug("[ringpop] could not read request body")
		return
	}

	// do whatever is in f
	f()

	var resHeaders []byte
	if err := tchannel.NewArgWriter(call.Response().Arg2Writer()).Write(resHeaders); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": endpoint,
		}).Debug("[ringpop] could not write response headers")
		return
	}

	var resBody []byte
	if err := tchannel.NewArgWriter(call.Response().Arg3Writer()).Write(resBody); err != nil {
		s.ringpop.logger.WithFields(log.Fields{
			"local":    s.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": endpoint,
		}).Debug("[ringpop] could not write response body")
		return
	}
}

func (s *ringpopServer) adminDebugSetHandler(ctx context.Context, call *tchannel.InboundCall) {
	if s.channel.Closed() {
		s.ringpop.logger.WithField("local", s.ringpop.WhoAmI()).
			Error("[ringpop] got call while channel closed!")
	}

	receiveCallNoArgs(s, call, "admin-debug-set", func() {
		s.ringpop.logger.Level = log.DebugLevel
	})
}

func (s *ringpopServer) adminDebugClearHandler(ctx context.Context, call *tchannel.InboundCall) {
	if s.channel.Closed() {
		s.ringpop.logger.WithField("local", s.ringpop.WhoAmI()).
			Error("[ringpop] got call while channel closed!")
	}

	receiveCallNoArgs(s, call, "admin-debug-set", func() {
		s.ringpop.logger.Level = log.InfoLevel
	})
}
