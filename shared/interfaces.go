package shared

import "github.com/uber/tchannel-go"

// The TChannel interface defines the dependencies for TChannel in Ringpop.
type TChannel interface {
	Register(h tchannel.Handler, methodName string)
	PeerInfo() tchannel.LocalPeerInfo
	GetSubChannel(string, ...tchannel.SubChannelOption) *tchannel.SubChannel
}

// SubChannel represents a TChannel SubChannel as used in Ringpop.
type SubChannel interface {
	tchannel.Registrar
}
