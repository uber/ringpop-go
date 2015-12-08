package pingpong

import (
	"errors"
	"fmt"

	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/router"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

// ------ Generated code for PingPong ------
type RingpopPingPongAdapter struct {
	impl    TChanPingPong
	ringpop ringpop.Interface
	ch      *tchannel.Channel
	config  PingPongConfiguration
	router  router.Router
}

type PingPongConfiguration struct {
	// structs for all service endpoints containing forwarding behavior
	Ping *PingPongPingConfiguration
}

func (c *PingPongConfiguration) validate() error {
	if c.Ping != nil {
		if c.Ping.Key == nil {
			return errors.New("configuration for endpoint Ping is missing a Key function")
		}
	}
	return nil
}

func NewRingpopPingPongAdapter(
	impl TChanPingPong,
	rp ringpop.Interface,
	ch *tchannel.Channel,
	config PingPongConfiguration,
) (TChanPingPong, error) {
	err := config.validate()
	if err != nil {
		return nil, err
	}

	adapter := &RingpopPingPongAdapter{
		impl:    impl,
		ringpop: rp,
		ch:      ch,
		config:  config,
	}
	// create ringpop router for routing based on ring membership
	adapter.router = router.NewRouter(rp, adapter, ch)

	return adapter, nil
}

func (a *RingpopPingPongAdapter) GetLocalClient() interface{} {
	return a.impl
}

func (a *RingpopPingPongAdapter) MakeRemoteClient(client thrift.TChanClient) interface{} {
	return NewTChanPingPongClient(client)
}

// ------ 'Generated' code for Ping ------ config.Ping
type PingPongPingConfiguration struct {
	Key func(ctx thrift.Context, request *Ping) (string, error)
}

func (a *RingpopPingPongAdapter) Ping(ctx thrift.Context, request *Ping) (r *Pong, err error) {
	// check if the function should be called locally
	if a.config.Ping == nil || forward.HasForwardedHeader(ctx) {
		return a.impl.Ping(ctx, request)
	}

	// find the key to shard on
	ringpopKey, err := a.config.Ping.Key(ctx, request)
	if err != nil {
		return r, fmt.Errorf("could not get key: %q", err)
	}

	clientInterface, err := a.router.GetClient(ringpopKey)
	if err != nil {
		return r, err
	}

	client := clientInterface.(TChanPingPong)
	ctx = forward.SetForwardedHeader(ctx)
	return client.Ping(ctx, request)
}

// ------ End of PingPong ------
