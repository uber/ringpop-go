package remoteservice

import (
	"testing"
	"time"

	"github.com/uber/ringpop-go/forward"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/uber/ringpop-go/router"
	"github.com/uber/ringpop-go/test/mocks"
	. "github.com/uber/ringpop-go/test/remoteservice/.gen/go/remoteservice"
	shared "github.com/uber/ringpop-go/test/remoteservice/.gen/go/shared"
	servicemocks "github.com/uber/ringpop-go/test/remoteservice/mocks"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

var _ = shared.GoUnusedProtection__

//go:generate mkdir -p .gen/go
//go:generate thrift-gen --generateThrift --outputDir .gen/go --inputFile remoteservice.thrift --template github.com/uber/ringpop-go/ringpop.thrift-gen -packagePrefix github.com/uber/ringpop-go/test/remoteservice/.gen/go/
//go:generate mockery -dir=.gen/go/remoteservice -name=TChanRemoteService

func TestNewRingpopRemoteServiceAdapter(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("AddListener", mock.Anything).Return(false)
	serviceImpl := &servicemocks.TChanRemoteService{}

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, nil, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name shared.Name) (string, error) {
				return string(name), nil
			},
		},
	})
	assert.Equal(t, err, nil, "creation of adator gave an error")
	assert.NotEqual(t, adapter, nil, "adapter not created")
}

func TestNewRingpopRemoteServiceAdapterInputValidation(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("AddListener", mock.Anything).Return(false)
	serviceImpl := &servicemocks.TChanRemoteService{}

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, nil, RemoteServiceConfiguration{})
	assert.Equal(t, err, nil, "creation of adator gave an error")
	assert.NotEqual(t, adapter, nil, "adapter not created")

	adapter, err = NewRingpopRemoteServiceAdapter(serviceImpl, rp, nil, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: nil,
		},
	})
	assert.NotEqual(t, err, nil, "adapter creation should have given an error without a Key closure defined")
	assert.Equal(t, adapter, nil, "adapter should not be creaeted on error")
}

func TestRingpopRemoteServiceAdapterCallLocal(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("AddListener", mock.Anything).Return(false)
	rp.On("Lookup", "hello").Return("127.0.0.1:3000", nil)
	rp.On("WhoAmI").Return("127.0.0.1:3000", nil)

	serviceImpl := &servicemocks.TChanRemoteService{}
	serviceImpl.On("RemoteCall", mock.Anything, shared.Name("hello")).Run(func(args mock.Arguments) {
		ctx := args[0].(thrift.Context)
		headers := ctx.Headers()

		_, has := headers[forward.ForwardedHeaderName]
		assert.False(t, has, "expected the forwarding header %q to not be present in the call to the local implementation of the service when calling the local client", forward.ForwardedHeaderName)
	}).Return(nil)

	ctx, _ := thrift.NewContext(0 * time.Second)

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, nil, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name shared.Name) (string, error) {
				return string(name), nil
			},
		},
	})
	assert.Equal(t, err, nil, "creation of adator gave an error")

	err = adapter.RemoteCall(ctx, shared.Name("hello"))
	assert.Equal(t, err, nil, "calling RemoteCall gave an error")

	serviceImpl.AssertCalled(t, "RemoteCall", mock.Anything, shared.Name("hello"))
}

func TestRingpopRemoteServiceAdapterCallLocalPreservingHeaders(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("AddListener", mock.Anything).Return(false)
	rp.On("Lookup", "hello").Return("127.0.0.1:3000", nil)
	rp.On("WhoAmI").Return("127.0.0.1:3000", nil)

	serviceImpl := &servicemocks.TChanRemoteService{}
	serviceImpl.On("RemoteCall", mock.Anything, shared.Name("hello")).Run(func(args mock.Arguments) {
		ctx := args[0].(thrift.Context)
		headers := ctx.Headers()

		_, has := headers[forward.ForwardedHeaderName]
		assert.False(t, has, "expected the forwarding header %q to not be present in the call to the local implementation of the service when calling the local client", forward.ForwardedHeaderName)

		assert.Equal(t, map[string]string{
			"hello": "world",
			"foo":   "bar",
			"baz":   "42",
		}, headers, "expected existing headers to be preserved")
	}).Return(nil)

	ctx, _ := thrift.NewContext(0 * time.Second)
	ctx = thrift.WithHeaders(ctx, map[string]string{
		"hello": "world",
		"foo":   "bar",
		"baz":   "42",
	})

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, nil, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name shared.Name) (string, error) {
				return string(name), nil
			},
		},
	})
	assert.Equal(t, err, nil, "creation of adator gave an error")

	err = adapter.RemoteCall(ctx, shared.Name("hello"))
	assert.Equal(t, err, nil, "calling RemoteCall gave an error")

	serviceImpl.AssertCalled(t, "RemoteCall", mock.Anything, shared.Name("hello"))
}

func TestRingpopRemoteServiceAdapterCallRemote(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("AddListener", mock.Anything).Return(false)
	rp.On("Lookup", "hello").Return("127.0.0.1:3001", nil)
	rp.On("WhoAmI").Return("127.0.0.1:3000", nil)

	serviceImpl := &servicemocks.TChanRemoteService{}
	// THIS IS NOT CALLED AS IT IS THE LOCAL IMPLEMENTATION, NEED TO FIND A WAY TO MOCK THE REMOTE CLIENT TO TEST THIS
	serviceImpl.On("RemoteCall", mock.Anything, shared.Name("hello")).Run(func(args mock.Arguments) {
		t.Fail()

		ctx := args[0].(thrift.Context)
		headers := ctx.Headers()

		_, has := headers[forward.ForwardedHeaderName]
		assert.True(t, has, "expected the forwarding header %q to be present in the call to the remote implementation of the service", forward.ForwardedHeaderName)
	}).Return(nil)
	ctx, _ := thrift.NewContext(0 * time.Second)

	ch, err := tchannel.NewChannel("remote", nil)
	assert.Equal(t, err, nil, "could not create tchannel")

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, ch, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name shared.Name) (string, error) {
				return string(name), nil
			},
		},
	})
	assert.Equal(t, err, nil, "creation of adator gave an error")

	// Because it is not easily possible to stub a remote call we assert that the remote call failed.
	// If it didn't fail it is likely that the serviceImpl was called, which we assert that it isn't called either
	err = adapter.RemoteCall(ctx, "hello")
	assert.NotEqual(t, err, nil, "we expected an error from the remote call since it could not reach anything over the network")
	serviceImpl.AssertNotCalled(t, "RemoteCall", mock.Anything, "hello")
}

func TestRingpopRemoteServiceAdapterCallRemotePreservingHeaders(t *testing.T) {
	t.Skip("The data passed to the remote call is not verified in any test at this moment, can't verify the preserving of the headers")

	rp := &mocks.Ringpop{}
	rp.On("AddListener", mock.Anything).Return(false)
	rp.On("Lookup", "hello").Return("127.0.0.1:3001", nil)
	rp.On("WhoAmI").Return("127.0.0.1:3000", nil)

	serviceImpl := &servicemocks.TChanRemoteService{}
	// THIS IS NOT CALLED AS IT IS THE LOCAL IMPLEMENTATION, NEED TO FIND A WAY TO MOCK THE REMOTE CLIENT TO TEST THIS
	serviceImpl.On("RemoteCall", mock.Anything, shared.Name("hello")).Run(func(args mock.Arguments) {
		t.Fail()

		ctx := args[0].(thrift.Context)
		headers := ctx.Headers()

		_, has := headers[forward.ForwardedHeaderName]
		assert.True(t, has, "expected the forwarding header %q to be present in the call to the remote implementation of the service", forward.ForwardedHeaderName)

		assert.Equal(t, map[string]string{
			// the forwarded header is expected here because it will be sent
			// over the wire and will be picked up on the other side at the
			// adapter level again
			forward.ForwardedHeaderName: "true",

			"hello": "world",
			"foo":   "bar",
			"baz":   "42",
		}, headers, "expected existing headers to be preserved")
	}).Return(nil)

	ctx, _ := thrift.NewContext(0 * time.Second)
	ctx = thrift.WithHeaders(ctx, map[string]string{
		"hello": "world",
		"foo":   "bar",
		"baz":   "42",
	})

	ch, err := tchannel.NewChannel("remote", nil)
	assert.Equal(t, err, nil, "could not create tchannel")

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, ch, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name shared.Name) (string, error) {
				return string(name), nil
			},
		},
	})
	assert.Equal(t, err, nil, "creation of adator gave an error")

	// Because it is not easily possible to stub a remote call we assert that the remote call failed.
	// If it didn't fail it is likely that the serviceImpl was called, which we assert that it isn't called either
	err = adapter.RemoteCall(ctx, "hello")
	assert.NotEqual(t, err, nil, "we expected an error from the remote call since it could not reach anything over the network")
	serviceImpl.AssertNotCalled(t, "RemoteCall", mock.Anything, "hello")
}

func TestRingpopRemoteServiceAdapterReceivingForwardedCall(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("AddListener", mock.Anything).Return(false)
	rp.On("Lookup", "hello").Return("127.0.0.1:3000", nil)
	rp.On("WhoAmI").Return("127.0.0.1:3000", nil)

	serviceImpl := &servicemocks.TChanRemoteService{}
	serviceImpl.On("RemoteCall", mock.Anything, shared.Name("hello")).Run(func(args mock.Arguments) {
		ctx := args[0].(thrift.Context)
		headers := ctx.Headers()

		_, has := headers[forward.ForwardedHeaderName]
		assert.False(t, has, "expected the forwarding header %q to not be present in dispatching after being forwarded", forward.ForwardedHeaderName)
	}).Return(nil)

	ctx, _ := thrift.NewContext(0 * time.Second)
	ctx = forward.SetForwardedHeader(ctx, []string{"hello"})

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, nil, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name shared.Name) (string, error) {
				return string(name), nil
			},
		},
	})
	assert.Equal(t, err, nil, "creation of adator gave an error")

	err = adapter.RemoteCall(ctx, shared.Name("hello"))
	assert.Equal(t, err, nil, "calling RemoteCall gave an error")

	serviceImpl.AssertCalled(t, "RemoteCall", mock.Anything, shared.Name("hello"))
}

func TestRingpopRemoteServiceAdapterReceivingForwardedCallPreservingHeaders(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("AddListener", mock.Anything).Return(false)
	rp.On("Lookup", "hello").Return("127.0.0.1:3000", nil)
	rp.On("WhoAmI").Return("127.0.0.1:3000", nil)

	serviceImpl := &servicemocks.TChanRemoteService{}
	serviceImpl.On("RemoteCall", mock.Anything, shared.Name("hello")).Run(func(args mock.Arguments) {
		ctx := args[0].(thrift.Context)
		headers := ctx.Headers()

		_, has := headers[forward.ForwardedHeaderName]
		assert.False(t, has, "expected the forwarding header %q to not be present in dispatching after being forwarded", forward.ForwardedHeaderName)

		assert.Equal(t, map[string]string{
			"hello": "world",
			"foo":   "bar",
			"baz":   "42",
		}, headers, "expected existing headers to be preserved")
	}).Return(nil)

	ctx, _ := thrift.NewContext(0 * time.Second)
	// set headers for the call
	ctx = thrift.WithHeaders(ctx, map[string]string{
		// headers that should be preserved
		"hello": "world",
		"foo":   "bar",
		"baz":   "42",
	})
	// add the forwarding header
	ctx = forward.SetForwardedHeader(ctx, []string{"hello"})

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, nil, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name shared.Name) (string, error) {
				return string(name), nil
			},
		},
	})
	assert.Equal(t, err, nil, "creation of adator gave an error")

	err = adapter.RemoteCall(ctx, shared.Name("hello"))
	assert.Equal(t, err, nil, "calling RemoteCall gave an error")

	serviceImpl.AssertCalled(t, "RemoteCall", mock.Anything, shared.Name("hello"))
}

func TestGetLocalClient(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("AddListener", mock.Anything).Return(false)
	rp.On("Lookup", "hello").Return("127.0.0.1:3001")
	rp.On("WhoAmI").Return("127.0.0.1:3000")

	serviceImpl := &servicemocks.TChanRemoteService{}
	serviceImpl.On("RemoteCall", mock.Anything, shared.Name("hello")).Return(nil)
	ctx, _ := thrift.NewContext(0 * time.Second)

	ch, err := tchannel.NewChannel("remote", nil)
	assert.Equal(t, err, nil, "could not create tchannel")

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, ch, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name shared.Name) (string, error) {
				return string(name), nil
			},
		},
	})

	cf := adapter.(router.ClientFactory)
	localClient := cf.GetLocalClient().(TChanRemoteService)
	err = localClient.RemoteCall(ctx, shared.Name("hello"))
	assert.Equal(t, err, nil, "calling the local client gave an error")
	serviceImpl.AssertCalled(t, "RemoteCall", mock.Anything, shared.Name("hello"))
}

func TestMakeRemoteClient(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("AddListener", mock.Anything).Return(false)
	rp.On("Lookup", "hello").Return("127.0.0.1:3001")
	rp.On("WhoAmI").Return("127.0.0.1:3000")

	serviceImpl := &servicemocks.TChanRemoteService{}
	serviceImpl.On("RemoteCall", mock.Anything, shared.Name("hello")).Return(nil)
	ctx, _ := thrift.NewContext(0 * time.Second)

	ch, err := tchannel.NewChannel("remote", nil)
	assert.Equal(t, err, nil, "could not create tchannel")

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, ch, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name shared.Name) (string, error) {
				return string(name), nil
			},
		},
	})

	tchanClient := &mocks.TChanClient{}
	tchanClient.On("Call", mock.Anything, "RemoteService", "RemoteCall", &RemoteServiceRemoteCallArgs{Name: shared.Name("hello")}, mock.Anything).Return(true, nil)

	cf := adapter.(router.ClientFactory)
	remoteClient := cf.MakeRemoteClient(tchanClient).(TChanRemoteService)
	err = remoteClient.RemoteCall(ctx, shared.Name("hello"))
	assert.Equal(t, err, nil, "calling the remote client gave an error")

	tchanClient.AssertCalled(t, "Call", mock.Anything, "RemoteService", "RemoteCall", &RemoteServiceRemoteCallArgs{Name: shared.Name("hello")}, mock.Anything)
}
