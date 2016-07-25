package role

import (
	"errors"
	"fmt"

	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/router"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

type RingpopRoleServiceAdapter struct {
	impl    TChanRoleService
	ringpop ringpop.Interface
	ch      *tchannel.Channel
	config  RoleServiceConfiguration
	router  router.Router
}

// RoleServiceConfiguration contains the forwarding configuration for the RoleService service. It has a field for every endpoint defined in the service. In this field the endpoint specific forward configuration can be stored. Populating these fields is optional, default behaviour is to call the service implementation locally to the process where the call came in.
type RoleServiceConfiguration struct {
	// GetMembers holds the forwarding configuration for the GetMembers endpoint defined in the service
	GetMembers *RoleServiceGetMembersConfiguration
	// SetRole holds the forwarding configuration for the SetRole endpoint defined in the service
	SetRole *RoleServiceSetRoleConfiguration
}

func (c *RoleServiceConfiguration) validate() error {
	if c.GetMembers != nil {
		if c.GetMembers.Key == nil {
			return errors.New("configuration for endpoint GetMembers is missing a Key function")
		}
	}
	if c.SetRole != nil {
		if c.SetRole.Key == nil {
			return errors.New("configuration for endpoint SetRole is missing a Key function")
		}
	}
	return nil
}

// NewRingpopRoleServiceAdapter creates an implementation of the TChanRoleService interface. This specific implementation will use to configuration provided during construction to deterministically route calls to nodes from a ringpop cluster. The channel should be the channel on which the service exposes its endpoints. Forwarded calls, calls to unconfigured endpoints and calls that already were executed on the right machine will be passed on the the implementation passed in during construction.
//
// Example usage:
//  import "github.com/uber/tchannel-go/thrift"
//
//  var server thrift.Server
//  server = ...
//
//  var handler TChanRoleService
//  handler = &YourImplementation{}
//
//  adapter, _ := NewRingpopRoleServiceAdapter(handler, ringpop, channel,
//    RoleServiceConfiguration{
//      GetMembers: &RoleServiceGetMembersConfiguration: {
//        Key: func(ctx thrift.Context, role string) (shardKey string, err error) {
//          return "calculated-shard-key", nil
//        },

//      SetRole: &RoleServiceSetRoleConfiguration: {
//        Key: func(ctx thrift.Context, role string) (shardKey string, err error) {
//          return "calculated-shard-key", nil
//        },
//      },
//    },
//  )
//  server.Register(NewTChanRoleServiceServer(adapter))
func NewRingpopRoleServiceAdapter(
	impl TChanRoleService,
	rp ringpop.Interface,
	ch *tchannel.Channel,
	config RoleServiceConfiguration,
) (TChanRoleService, error) {
	err := config.validate()
	if err != nil {
		return nil, err
	}

	adapter := &RingpopRoleServiceAdapter{
		impl:    impl,
		ringpop: rp,
		ch:      ch,
		config:  config,
	}
	// create ringpop router for routing based on ring membership
	adapter.router = router.New(rp, adapter, ch)

	return adapter, nil
}

// GetLocalClient satisfies the ClientFactory interface of ringpop-go/router
func (a *RingpopRoleServiceAdapter) GetLocalClient() interface{} {
	return a.impl
}

// MakeRemoteClient satisfies the ClientFactory interface of ringpop-go/router
func (a *RingpopRoleServiceAdapter) MakeRemoteClient(client thrift.TChanClient) interface{} {
	return NewTChanRoleServiceClient(client)
}

// RoleServiceGetMembersConfiguration contains the configuration on how to route calls to the thrift endpoint RoleService::GetMembers.
type RoleServiceGetMembersConfiguration struct {
	// Key is a closure that generates a routable key based on the parameters of the incomming request.
	Key func(ctx thrift.Context, role string) (string, error)
}

// GetMembers satisfies the TChanRoleService interface. This function uses the configuration for GetMembers to determine the host to execute the call on. When it decides the call needs to be executed in the current process it will forward the invocation to its local implementation.
func (a *RingpopRoleServiceAdapter) GetMembers(ctx thrift.Context, role string) (r []string, err error) {
	// check if the function should be called locally
	if a.config.GetMembers == nil || forward.HasForwardedHeader(ctx) {
		return a.impl.GetMembers(ctx, role)
	}

	// find the key to shard on
	ringpopKey, err := a.config.GetMembers.Key(ctx, role)
	if err != nil {
		return r, fmt.Errorf("could not get key: %q", err)
	}

	clientInterface, err := a.router.GetClient(ringpopKey)
	if err != nil {
		return r, err
	}

	client := clientInterface.(TChanRoleService)
	ctx = forward.SetForwardedHeader(ctx)
	return client.GetMembers(ctx, role)
}

// RoleServiceSetRoleConfiguration contains the configuration on how to route calls to the thrift endpoint RoleService::SetRole.
type RoleServiceSetRoleConfiguration struct {
	// Key is a closure that generates a routable key based on the parameters of the incomming request.
	Key func(ctx thrift.Context, role string) (string, error)
}

// SetRole satisfies the TChanRoleService interface. This function uses the configuration for SetRole to determine the host to execute the call on. When it decides the call needs to be executed in the current process it will forward the invocation to its local implementation.
func (a *RingpopRoleServiceAdapter) SetRole(ctx thrift.Context, role string) (err error) {
	// check if the function should be called locally
	if a.config.SetRole == nil || forward.HasForwardedHeader(ctx) {
		return a.impl.SetRole(ctx, role)
	}

	// find the key to shard on
	ringpopKey, err := a.config.SetRole.Key(ctx, role)
	if err != nil {
		return fmt.Errorf("could not get key: %q", err)
	}

	clientInterface, err := a.router.GetClient(ringpopKey)
	if err != nil {
		return err
	}

	client := clientInterface.(TChanRoleService)
	ctx = forward.SetForwardedHeader(ctx)
	return client.SetRole(ctx, role)
}
