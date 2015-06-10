package ringpop

import (
	"fmt"
	"time"

	"github.com/uber/tchannel/golang"
	"golang.org/x/net/context"
)

// var commands = map[string]map[string]tchannel.HandlerFunc{
// 	// "health": {
// 	// 	"health": health,
// 	// },
// 	// "admin": {
// 	// 	"stats":      adminStats,
// 	// 	"debugSet":   debugSet,
// 	// 	"debugClear": debugClear,
// 	// 	"gossip":     gossip,
// 	// 	"leave":      leave,
// 	// 	"join":       join,
// 	// 	"reload":     reload,
// 	// 	"tick":       tick,
// 	// },
// 	"protocol": {
// 		// "join":     protocolJoin,
// 		"ping": protocolPing,
// 		// "ping-req": protocolPingReq,
// 	},
// 	// "proxy": {
// 	// 	"req": proxyReq,
// 	// },
// }

type RingpopTChannel struct {
	ringpop *Ringpop
	channel *tchannel.Channel
}

func NewRingpopTChannel(ringpop *Ringpop, channel *tchannel.Channel) {
	ringpopTChannel := &RingpopTChannel{
		ringpop: ringpop,
		channel: channel,
	}

	ringpopTChannel.channel.Register(ringpopTChannel.protocolPing(), "protocol", "ping")

	// Register endpoint`s with TChannel
	// for service, operations := range commands {
	// 	for operation, handler := range operations {
	// 		ringpopTChannel.channel.Register(handler(ringpopTChannel), service, operation)
	// 	}
	// }
}

type Headers map[string]string

type Ping struct {
	Message string `json:"message"`
}

func (this *RingpopTChannel) protocolPing() tchannel.HandlerFunc {
	handler := func(ctx context.Context, call *tchannel.InboundCall) {
		var headers Headers
		var inArg2 tchannel.BytesInput
		if err := call.ReadArg2(&inArg2); err != nil {
			this.ringpop.logger.Warnf("Could not read request headers: %v", err)
			return
		}

		var inArg3 tchannel.BytesInput
		if err := call.ReadArg3(&inArg3); err != nil {
			this.ringpop.logger.Warnf("Could not read request body: %v", err)
			return
		}

		// TODO: Real version ...
		res := PingBody{
			Checksum: this.ringpop.membership.checksum,
			Changes: []Change{Change{
				Address:     "127.0.0.1:9001",
				Status:      ALIVE,
				Incarnation: time.Now().UnixNano(),
				Source:      this.ringpop.WhoAmI(),
			}, Change{
				Address:     "127.0.0.1:9002",
				Status:      ALIVE,
				Incarnation: time.Now().UnixNano(),
				Source:      this.ringpop.WhoAmI(),
			}},
			Source: this.ringpop.WhoAmI(),
		}

		if err := call.Response().WriteArg2(tchannel.NewJSONOutput(headers)); err != nil {
			this.ringpop.logger.Warnf("Could not write response headers: %v", err)
			return
		}

		if err := call.Response().WriteArg3(tchannel.NewJSONOutput(res)); err != nil {
			this.ringpop.logger.Warnf("Could not write response body: %v", err)
			return
		}
	}
	return tchannel.HandlerFunc(handler)
}

func protocolPing(ringpopTCh *RingpopTChannel) tchannel.HandlerFunc {
	// this := ringpopTCh

	handler := func(ctx context.Context, call *tchannel.InboundCall) {
		var headers Headers

		var inArg2 tchannel.BytesInput
		if err := call.ReadArg2(&inArg2); err != nil {
			return
		}

		var inArg3 tchannel.BytesInput
		if err := call.ReadArg3(&inArg3); err != nil {
			return
		}

		ping := Ping{
			Message: fmt.Sprintf("ping %s", inArg3),
		}

		if err := call.Response().WriteArg2(tchannel.NewJSONOutput(headers)); err != nil {
			return
		}

		if err := call.Response().WriteArg3(tchannel.NewJSONOutput(ping)); err != nil {
			return
		}
	}
	return tchannel.HandlerFunc(handler)
}
