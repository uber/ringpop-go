// @generated Code generated by thrift-gen. Do not modify.

package keyvalue

import (
	"errors"
	"fmt"

	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/router"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

type RingpopKeyValueServiceAdapter struct {
	impl    TChanKeyValueService
	ringpop ringpop.Interface
	ch      *tchannel.Channel
	config  KeyValueServiceConfiguration
	router  router.Router
}

// KeyValueServiceConfiguration contains the forwarding configuration for the KeyValueService service. It has a field for every endpoint defined in the service. In this field the endpoint specific forward configuration can be stored. Populating these fields is optional, default behaviour is to call the service implementation locally to the process where the call came in.
type KeyValueServiceConfiguration struct {
	// Get holds the forwarding configuration for the Get endpoint defined in the service
	Get *KeyValueServiceGetConfiguration
	// GetAll holds the forwarding configuration for the GetAll endpoint defined in the service
	GetAll *KeyValueServiceGetAllConfiguration
	// Set holds the forwarding configuration for the Set endpoint defined in the service
	Set *KeyValueServiceSetConfiguration
}

func (c *KeyValueServiceConfiguration) validate() error {
	if c.Get != nil {
		if c.Get.Key == nil {
			return errors.New("configuration for endpoint Get is missing a Key function")
		}
	}
	if c.GetAll != nil {
		if c.GetAll.Key == nil {
			return errors.New("configuration for endpoint GetAll is missing a Key function")
		}
	}
	if c.Set != nil {
		if c.Set.Key == nil {
			return errors.New("configuration for endpoint Set is missing a Key function")
		}
	}
	return nil
}

// NewRingpopKeyValueServiceAdapter creates an implementation of the TChanKeyValueService interface. This specific implementation will use to configuration provided during construction to deterministically route calls to nodes from a ringpop cluster. The channel should be the channel on which the service exposes its endpoints. Forwarded calls, calls to unconfigured endpoints and calls that already were executed on the right machine will be passed on the the implementation passed in during construction.
//
// Example usage:
//  import "github.com/uber/tchannel-go/thrift"
//
//  var server thrift.Server
//  server = ...
//
//  var handler TChanKeyValueService
//  handler = &YourImplementation{}
//
//  adapter, _ := NewRingpopKeyValueServiceAdapter(handler, ringpop, channel,
//    KeyValueServiceConfiguration{
//      Get: &KeyValueServiceGetConfiguration: {
//        Key: func(ctx thrift.Context, key string) (shardKey string, err error) {
//          return "calculated-shard-key", nil
//        },

//      GetAll: &KeyValueServiceGetAllConfiguration: {
//        Key: func(ctx thrift.Context, keys []string) (shardKey string, err error) {
//          return "calculated-shard-key", nil
//        },

//      Set: &KeyValueServiceSetConfiguration: {
//        Key: func(ctx thrift.Context, key string, value string) (shardKey string, err error) {
//          return "calculated-shard-key", nil
//        },
//      },
//    },
//  )
//  server.Register(NewTChanKeyValueServiceServer(adapter))
func NewRingpopKeyValueServiceAdapter(
	impl TChanKeyValueService,
	rp ringpop.Interface,
	ch *tchannel.Channel,
	config KeyValueServiceConfiguration,
) (TChanKeyValueService, error) {
	err := config.validate()
	if err != nil {
		return nil, err
	}

	adapter := &RingpopKeyValueServiceAdapter{
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
func (a *RingpopKeyValueServiceAdapter) GetLocalClient() interface{} {
	return a.impl
}

// MakeRemoteClient satisfies the ClientFactory interface of ringpop-go/router
func (a *RingpopKeyValueServiceAdapter) MakeRemoteClient(client thrift.TChanClient) interface{} {
	return NewTChanKeyValueServiceClient(client)
}

// KeyValueServiceGetConfiguration contains the configuration on how to route calls to the thrift endpoint KeyValueService::Get.
type KeyValueServiceGetConfiguration struct {
	// Key is a closure that generates a routable key based on the parameters of the incomming request.
	Key func(ctx thrift.Context, key string) (string, error)
}

// Get satisfies the TChanKeyValueService interface. This function uses the configuration for Get to determine the host to execute the call on. When it decides the call needs to be executed in the current process it will forward the invocation to its local implementation.
func (a *RingpopKeyValueServiceAdapter) Get(ctx thrift.Context, key string) (r string, err error) {
	// check if the function should be called locally
	if a.config.Get == nil || forward.HasForwardedHeader(ctx) {
		return a.impl.Get(ctx, key)
	}

	// find the key to shard on
	ringpopKey, err := a.config.Get.Key(ctx, key)
	if err != nil {
		return r, fmt.Errorf("could not get key: %q", err)
	}

	clientInterface, isRemote, err := a.router.GetClient(ringpopKey)
	if err != nil {
		return r, err
	}

	client := clientInterface.(TChanKeyValueService)
	if isRemote {
		ctx = forward.SetForwardedHeader(ctx, []string{ringpopKey})
	}
	return client.Get(ctx, key)
}

// KeyValueServiceGetAllConfiguration contains the configuration on how to route calls to the thrift endpoint KeyValueService::GetAll.
type KeyValueServiceGetAllConfiguration struct {
	// Key is a closure that generates a routable key based on the parameters of the incomming request.
	Key func(ctx thrift.Context, keys []string) (string, error)
}

// GetAll satisfies the TChanKeyValueService interface. This function uses the configuration for GetAll to determine the host to execute the call on. When it decides the call needs to be executed in the current process it will forward the invocation to its local implementation.
func (a *RingpopKeyValueServiceAdapter) GetAll(ctx thrift.Context, keys []string) (r []string, err error) {
	// check if the function should be called locally
	if a.config.GetAll == nil || forward.HasForwardedHeader(ctx) {
		return a.impl.GetAll(ctx, keys)
	}

	// find the key to shard on
	ringpopKey, err := a.config.GetAll.Key(ctx, keys)
	if err != nil {
		return r, fmt.Errorf("could not get key: %q", err)
	}

	clientInterface, isRemote, err := a.router.GetClient(ringpopKey)
	if err != nil {
		return r, err
	}

	client := clientInterface.(TChanKeyValueService)
	if isRemote {
		ctx = forward.SetForwardedHeader(ctx, []string{ringpopKey})
	}
	return client.GetAll(ctx, keys)
}

// KeyValueServiceSetConfiguration contains the configuration on how to route calls to the thrift endpoint KeyValueService::Set.
type KeyValueServiceSetConfiguration struct {
	// Key is a closure that generates a routable key based on the parameters of the incomming request.
	Key func(ctx thrift.Context, key string, value string) (string, error)
}

// Set satisfies the TChanKeyValueService interface. This function uses the configuration for Set to determine the host to execute the call on. When it decides the call needs to be executed in the current process it will forward the invocation to its local implementation.
func (a *RingpopKeyValueServiceAdapter) Set(ctx thrift.Context, key string, value string) (err error) {
	// check if the function should be called locally
	if a.config.Set == nil || forward.HasForwardedHeader(ctx) {
		return a.impl.Set(ctx, key, value)
	}

	// find the key to shard on
	ringpopKey, err := a.config.Set.Key(ctx, key, value)
	if err != nil {
		return fmt.Errorf("could not get key: %q", err)
	}

	clientInterface, isRemote, err := a.router.GetClient(ringpopKey)
	if err != nil {
		return err
	}

	client := clientInterface.(TChanKeyValueService)
	if isRemote {
		ctx = forward.SetForwardedHeader(ctx, []string{ringpopKey})
	}
	return client.Set(ctx, key, value)
}
