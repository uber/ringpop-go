// Autogenerated by thrift-gen. Do not modify.
package pingpong

import (
	"fmt"

	athrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/uber/tchannel/golang/thrift"
)

// Interfaces for the service and client for the services defined in the IDL.

type TChanPingPong interface {
	Ping(ctx thrift.Context, request *Ping) (*Pong, error)
}

// Implementation of a client and service handler.

type tchanPingPongClient struct {
	client thrift.TChanClient
}

func NewTChanPingPongClient(client thrift.TChanClient) TChanPingPong {
	return &tchanPingPongClient{client: client}
}

func (c *tchanPingPongClient) Ping(ctx thrift.Context, request *Ping) (*Pong, error) {
	var resp PingResult
	args := PingArgs{
		Request: request,
	}
	success, err := c.client.Call(ctx, "PingPong", "Ping", &args, &resp)
	if err == nil && !success {
	}

	return resp.GetSuccess(), err
}

type tchanPingPongServer struct {
	handler TChanPingPong
}

func NewTChanPingPongServer(handler TChanPingPong) thrift.TChanServer {
	return &tchanPingPongServer{handler}
}

func (s *tchanPingPongServer) Service() string {
	return "PingPong"
}

func (s *tchanPingPongServer) Methods() []string {
	return []string{
		"Ping",
	}
}

func (s *tchanPingPongServer) Handle(ctx thrift.Context, methodName string, protocol athrift.TProtocol) (bool, athrift.TStruct, error) {
	switch methodName {
	case "Ping":
		return s.handlePing(ctx, protocol)
	default:
		return false, nil, fmt.Errorf("method %v not found in service %v", methodName, s.Service())
	}
}

func (s *tchanPingPongServer) handlePing(ctx thrift.Context, protocol athrift.TProtocol) (bool, athrift.TStruct, error) {
	var req PingArgs
	var res PingResult

	if err := req.Read(protocol); err != nil {
		return false, nil, err
	}

	r, err :=
		s.handler.Ping(ctx, req.Request)

	if err != nil {
		return false, nil, err
	} else {
		res.Success = r
	}

	return err == nil, &res, nil
}