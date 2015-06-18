package ringpop

import (
	"fmt"

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
		// 	"tick":       tick,
		// },
		"protocol": {
			"join": ringpopTChannel.protocolJoinHandler,
			"ping": ringpopTChannel.protocolPingHandler,
			// "ping-req": protocolPingReq,
		},
		// "proxy": {
		// 	"req": proxyReq,
		// },
	}

	// ringpopTChannel.channel.Register(ringpopTChannel.protocolPing(), "protocol", "ping")

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
	var reqHeaders headers
	if err := tchannel.NewArgReader(call.Arg2Reader()).ReadJSON(&reqHeaders); err != nil {
		rc.ringpop.logger.Debugf("could not read request headers: %v", err)
		return
	}

	var reqBody joinBody
	if err := tchannel.NewArgReader(call.Arg3Reader()).ReadJSON(&reqBody); err != nil {
		rc.ringpop.logger.Debugf("could not read request body: %v", err)
		return
	}

	resBody, err := receiveJoin(rc.ringpop, reqBody)
	if err != nil {
		rc.ringpop.logger.Debugf("ringpop unable to receive join: %v", err)
		return
	}

	var resHeaders headers
	if err := tchannel.NewArgWriter(call.Response().Arg2Writer()).WriteJSON(resHeaders); err != nil {
		rc.ringpop.logger.Debugf("could not write response headers: %v", err)
		return
	}

	if err := tchannel.NewArgWriter(call.Response().Arg3Writer()).WriteJSON(resBody); err != nil {
		rc.ringpop.logger.Debugf("could not write response body: %v", err)
		return
	}
}

func (rc *ringpopTChannel) protocolPingHandler(ctx context.Context, call *tchannel.InboundCall) {
	var reqHeaders headers

	if err := tchannel.NewArgReader(call.Arg2Reader()).ReadJSON(&reqHeaders); err != nil {
		rc.ringpop.logger.Debugf("could not read request headers: %v", err)
		return
	}

	var reqBody pingBody
	if err := tchannel.NewArgReader(call.Arg3Reader()).ReadJSON(&reqBody); err != nil {
		rc.ringpop.logger.Debugf("could not read request body: %v", err)
		return
	}

	changes := receivePing(rc.ringpop, reqBody)
	resBody := pingBody{
		Checksum: rc.ringpop.membership.checksum,
		Changes:  changes,
		Source:   rc.ringpop.WhoAmI(),
	}

	var resHeaders headers
	if err := tchannel.NewArgWriter(call.Response().Arg2Writer()).WriteJSON(resHeaders); err != nil {
		rc.ringpop.logger.Debugf("Could not write response headers: %v", err)
		return
	}

	if err := tchannel.NewArgWriter(call.Response().Arg3Writer()).WriteJSON(resBody); err != nil {
		rc.ringpop.logger.Debugf("Could not write response body: %v", err)
		return
	}
}
