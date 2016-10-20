package shared

import "github.com/uber/tchannel-go"

// The TChannel interface defines the dependencies for TChannel in Ringpop.
type TChannel interface {
	GetSubChannel(string, ...tchannel.SubChannelOption) *tchannel.SubChannel
	PeerInfo() tchannel.LocalPeerInfo
	Register(h tchannel.Handler, methodName string)
	State() tchannel.ChannelState
}

// SubChannel represents a TChannel SubChannel as used in Ringpop.
type SubChannel interface {
	tchannel.Registrar
}
