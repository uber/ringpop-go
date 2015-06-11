package ringpop

import (
	"time"

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
			// "join":     protocolJoin,
			"ping": ringpopTChannel.protocolPing(),
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
			ringpopTChannel.channel.Register(handler, service, operation)
		}
	}

	return ringpopTChannel
}

func (rc *ringpopTChannel) protocolPing() tchannel.HandlerFunc {
	handler := func(ctx context.Context, call *tchannel.InboundCall) {
		var headers headers
		if err := call.ReadArg2(tchannel.NewJSONInput(&headers)); err != nil {
			rc.ringpop.logger.Warnf("Could not read request headers: %v", err)
			return
		}

		var body pingBody
		if err := call.ReadArg3(tchannel.NewJSONInput(&body)); err != nil {
			rc.ringpop.logger.Warnf("Could not read request body: %v", err)
			return
		}

		// TODO: do something with body

		// changes := receivePing(rc.ringpop)

		// TODO: Real version ...
		resBody := pingBody{
			Checksum: rc.ringpop.membership.checksum,
			Changes: []Change{Change{
				Address:     "127.0.0.1:9001",
				Status:      ALIVE,
				Incarnation: time.Now().UnixNano(),
				Source:      rc.ringpop.WhoAmI(),
			}, Change{
				Address:     "127.0.0.1:9002",
				Status:      ALIVE,
				Incarnation: time.Now().UnixNano(),
				Source:      rc.ringpop.WhoAmI(),
			}},
			Source: rc.ringpop.WhoAmI(),
		}

		if err := call.Response().WriteArg2(tchannel.NewJSONOutput(headers)); err != nil {
			rc.ringpop.logger.Warnf("Could not write response headers: %v", err)
			return
		}

		if err := call.Response().WriteArg3(tchannel.NewJSONOutput(resBody)); err != nil {
			rc.ringpop.logger.Warnf("Could not write response body: %v", err)
			return
		}
	}
	return tchannel.HandlerFunc(handler)
}
