package ringpop

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
	"golang.org/x/net/context"
)

type headers map[string]string

type ringpopTChannel struct {
	ringpop *Ringpop
	channel *tchannel.Channel
}

func newRingpopTChannel(ringpop *Ringpop, channel *tchannel.Channel) *ringpopTChannel {
	ringpopTChannel := &ringpopTChannel{
		ringpop: ringpop,
		channel: channel,
	}

	var commands = map[string]map[string]tchannel.HandlerFunc{
		// "health": {
		// 	"health": health,
		// },
		// "admin": {
		// 	"stats":      adminStats,
		// 	"debugSet":   debugSet,
		// 	"debugClear": debugClear,
		// 	"gossip":     gossip,
		// 	"leave":      leave,
		// 	"join":       join,
		// 	"reload":     reload,
		// },
		"protocol": {
			"join":     ringpopTChannel.protocolJoinHandler,
			"ping":     ringpopTChannel.protocolPingHandler,
			"ping-req": ringpopTChannel.protocolPingReqHandler,
		},
		// "proxy": {
		// 	"req": proxyReq,
		// },
	}

	// Register endpoints with channel
	for service, operations := range commands {
		for operation, handler := range operations {
			handlerName := fmt.Sprintf("/%s/%s", service, operation)
			ringpopTChannel.channel.Register(handler, handlerName)
		}
	}

	return ringpopTChannel
}

func (rc *ringpopTChannel) protocolJoinHandler(ctx context.Context, call *tchannel.InboundCall) {
	if rc.channel.Closed() {
		rc.ringpop.logger.WithField("local", rc.ringpop.WhoAmI()).Error("[ringpop] got call while channel closed!")
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

	resBody, err := receiveJoin(rc.ringpop, reqBody)
	if err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "join-recv",
		}).Debug("[ringpop] could not read request headers")
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

func (rc *ringpopTChannel) protocolPingHandler(ctx context.Context, call *tchannel.InboundCall) {
	if rc.channel.Closed() {
		rc.ringpop.logger.WithField("local", rc.ringpop.WhoAmI()).Error("[ringpop] got call while channel closed!")
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

	resBody := receivePing(rc.ringpop, reqBody)
	if err := tchannel.NewArgWriter(call.Response().Arg3Writer()).WriteJSON(resBody); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-recv",
		}).Debug("[ringpop] could not write response body")
		return
	}
}

func (rc *ringpopTChannel) protocolPingReqHandler(ctx context.Context, call *tchannel.InboundCall) {
	if rc.channel.Closed() {
		rc.ringpop.logger.WithField("local", rc.ringpop.WhoAmI()).Error("[ringpop] got call while channel closed!")
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

	resBody := receivePingReq(rc.ringpop, reqBody)
	if err := tchannel.NewArgWriter(call.Response().Arg3Writer()).WriteJSON(resBody); err != nil {
		rc.ringpop.logger.WithFields(log.Fields{
			"local":    rc.ringpop.WhoAmI(),
			"error":    err,
			"endpoint": "ping-req-recv",
		}).Debug("[ringpop] could not write response body")
		return
	}
}
