package tchannel

import "golang.org/x/net/context"

// SubChannel allows calling a specific service on a channel.
// TODO(prashant): Allow creating a subchannel with default call options.
// TODO(prashant): Allow registering handlers on a subchannel.
type SubChannel struct {
	serviceName        string
	defaultCallOptions *CallOptions
	peers              *PeerList
}

func newSubChannel(serviceName string, peers *PeerList) *SubChannel {
	return &SubChannel{
		serviceName: serviceName,
		peers:       peers,
	}
}

// ServiceName returns the service name that this subchannel is for.
func (c *SubChannel) ServiceName() string {
	return c.serviceName
}

// BeginCall starts a new call to a remote peer, returning an OutboundCall that can
// be used to write the arguments of the call.
func (c *SubChannel) BeginCall(ctx context.Context, operationName string, callOptions *CallOptions) (*OutboundCall, error) {
	if callOptions == nil {
		callOptions = defaultCallOptions
	}
	return c.peers.Get().BeginCall(ctx, c.serviceName, operationName, callOptions)
}

// Peers returns the PeerList for this subchannel.
func (c *SubChannel) Peers() *PeerList {
	return c.peers
}
