// @generated Code generated by thrift-gen. Do not modify.

// Package keyvalue is generated code used to make or handle TChannel calls using Thrift.
package keyvalue

import (
	"fmt"

	athrift "github.com/uber/tchannel-go/thirdparty/github.com/apache/thrift/lib/go/thrift"
	"github.com/uber/tchannel-go/thrift"
)

// Interfaces for the service and client for the services defined in the IDL.

// TChanKeyValueService is the interface that defines the server handler and client interface.
type TChanKeyValueService interface {
	Get(ctx thrift.Context, key string) (string, error)
	GetAll(ctx thrift.Context, keys []string) ([]string, error)
	Set(ctx thrift.Context, key string, value string) error
}

// Implementation of a client and service handler.

type tchanKeyValueServiceClient struct {
	thriftService string
	client        thrift.TChanClient
}

func NewTChanKeyValueServiceInheritedClient(thriftService string, client thrift.TChanClient) *tchanKeyValueServiceClient {
	return &tchanKeyValueServiceClient{
		thriftService,
		client,
	}
}

// NewTChanKeyValueServiceClient creates a client that can be used to make remote calls.
func NewTChanKeyValueServiceClient(client thrift.TChanClient) TChanKeyValueService {
	return NewTChanKeyValueServiceInheritedClient("KeyValueService", client)
}

func (c *tchanKeyValueServiceClient) Get(ctx thrift.Context, key string) (string, error) {
	var resp KeyValueServiceGetResult
	args := KeyValueServiceGetArgs{
		Key: key,
	}
	success, err := c.client.Call(ctx, c.thriftService, "Get", &args, &resp)
	if err == nil && !success {
	}

	return resp.GetSuccess(), err
}

func (c *tchanKeyValueServiceClient) GetAll(ctx thrift.Context, keys []string) ([]string, error) {
	var resp KeyValueServiceGetAllResult
	args := KeyValueServiceGetAllArgs{
		Keys: keys,
	}
	success, err := c.client.Call(ctx, c.thriftService, "GetAll", &args, &resp)
	if err == nil && !success {
	}

	return resp.GetSuccess(), err
}

func (c *tchanKeyValueServiceClient) Set(ctx thrift.Context, key string, value string) error {
	var resp KeyValueServiceSetResult
	args := KeyValueServiceSetArgs{
		Key:   key,
		Value: value,
	}
	success, err := c.client.Call(ctx, c.thriftService, "Set", &args, &resp)
	if err == nil && !success {
	}

	return err
}

type tchanKeyValueServiceServer struct {
	handler TChanKeyValueService
}

// NewTChanKeyValueServiceServer wraps a handler for TChanKeyValueService so it can be
// registered with a thrift.Server.
func NewTChanKeyValueServiceServer(handler TChanKeyValueService) thrift.TChanServer {
	return &tchanKeyValueServiceServer{
		handler,
	}
}

func (s *tchanKeyValueServiceServer) Service() string {
	return "KeyValueService"
}

func (s *tchanKeyValueServiceServer) Methods() []string {
	return []string{
		"Get",
		"GetAll",
		"Set",
	}
}

func (s *tchanKeyValueServiceServer) Handle(ctx thrift.Context, methodName string, protocol athrift.TProtocol) (bool, athrift.TStruct, error) {
	switch methodName {
	case "Get":
		return s.handleGet(ctx, protocol)
	case "GetAll":
		return s.handleGetAll(ctx, protocol)
	case "Set":
		return s.handleSet(ctx, protocol)

	default:
		return false, nil, fmt.Errorf("method %v not found in service %v", methodName, s.Service())
	}
}

func (s *tchanKeyValueServiceServer) handleGet(ctx thrift.Context, protocol athrift.TProtocol) (bool, athrift.TStruct, error) {
	var req KeyValueServiceGetArgs
	var res KeyValueServiceGetResult

	if err := req.Read(protocol); err != nil {
		return false, nil, err
	}

	r, err :=
		s.handler.Get(ctx, req.Key)

	if err != nil {
		return false, nil, err
	} else {
		res.Success = &r
	}

	return err == nil, &res, nil
}

func (s *tchanKeyValueServiceServer) handleGetAll(ctx thrift.Context, protocol athrift.TProtocol) (bool, athrift.TStruct, error) {
	var req KeyValueServiceGetAllArgs
	var res KeyValueServiceGetAllResult

	if err := req.Read(protocol); err != nil {
		return false, nil, err
	}

	r, err :=
		s.handler.GetAll(ctx, req.Keys)

	if err != nil {
		return false, nil, err
	} else {
		res.Success = r
	}

	return err == nil, &res, nil
}

func (s *tchanKeyValueServiceServer) handleSet(ctx thrift.Context, protocol athrift.TProtocol) (bool, athrift.TStruct, error) {
	var req KeyValueServiceSetArgs
	var res KeyValueServiceSetResult

	if err := req.Read(protocol); err != nil {
		return false, nil, err
	}

	err :=
		s.handler.Set(ctx, req.Key, req.Value)

	if err != nil {
		return false, nil, err
	} else {
	}

	return err == nil, &res, nil
}
