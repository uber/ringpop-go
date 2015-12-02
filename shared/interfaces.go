package shared

import "github.com/uber/tchannel-go"

type TChannel interface {
	tchannel.Registrar
	PeerInfo() tchannel.LocalPeerInfo
	GetSubChannel(string, ...tchannel.SubChannelOption) *tchannel.SubChannel
}

type SubChannel interface {
	tchannel.Registrar
}
