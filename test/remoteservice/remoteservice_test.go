package remoteservice

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/uber/ringpop-go/router"
	"github.com/uber/ringpop-go/test/mocks"
	servicemocks "github.com/uber/ringpop-go/test/remoteservice/mocks"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

//go:generate mkdir -p .gen/go
//go:generate thrift-gen --generateThrift --outputDir .gen/go --inputFile remoteservice.thrift --template github.com/uber/ringpop-go/ringpop.thrift-gen
//go:generate cp -R .gen/go/remoteservice/ .
//go:generate rm -rf .gen
//go:generate mockery --name=TChanRemoteService

func TestNewRingpopRemoteServiceAdapter(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	serviceImpl := &servicemocks.TChanRemoteService{}

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, nil, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name string) (string, error) {
				return name, nil
			},
		},
	})
	assert.Equal(t, err, nil, "creation of adator gave an error")
	assert.NotEqual(t, adapter, nil, "adapter not created")
}

func TestNewRingpopRemoteServiceAdapterInputValidation(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
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
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3000")
	rp.On("WhoAmI").Return("127.0.0.1:3000")

	serviceImpl := &servicemocks.TChanRemoteService{}
	serviceImpl.On("RemoteCall", mock.Anything, "hello").Return(nil)
	ctx, _ := thrift.NewContext(0 * time.Second)

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, nil, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name string) (string, error) {
				return name, nil
			},
		},
	})
	assert.Equal(t, err, nil, "creation of adator gave an error")

	err = adapter.RemoteCall(ctx, "hello")
	assert.Equal(t, err, nil, "calling RemoteCall gave an error")

	serviceImpl.AssertCalled(t, "RemoteCall", mock.Anything, "hello")
}

func TestRingpopRemoteServiceAdapterCallRemote(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3001")
	rp.On("WhoAmI").Return("127.0.0.1:3000")

	serviceImpl := &servicemocks.TChanRemoteService{}
	serviceImpl.On("RemoteCall", mock.Anything, "hello").Return(nil)
	ctx, _ := thrift.NewContext(0 * time.Second)

	ch, err := tchannel.NewChannel("remote", nil)
	assert.Equal(t, err, nil, "could not create tchannel")

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, ch, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name string) (string, error) {
				return name, nil
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

func TestGetLocalClient(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3001")
	rp.On("WhoAmI").Return("127.0.0.1:3000")

	serviceImpl := &servicemocks.TChanRemoteService{}
	serviceImpl.On("RemoteCall", mock.Anything, "hello").Return(nil)
	ctx, _ := thrift.NewContext(0 * time.Second)

	ch, err := tchannel.NewChannel("remote", nil)
	assert.Equal(t, err, nil, "could not create tchannel")

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, ch, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name string) (string, error) {
				return name, nil
			},
		},
	})

	cf := adapter.(router.ClientFactory)
	localClient := cf.GetLocalClient().(TChanRemoteService)
	err = localClient.RemoteCall(ctx, "hello")
	assert.Equal(t, err, nil, "calling the local client gave an error")
	serviceImpl.AssertCalled(t, "RemoteCall", mock.Anything, "hello")

}

func TestMakeRemoteClient(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3001")
	rp.On("WhoAmI").Return("127.0.0.1:3000")

	serviceImpl := &servicemocks.TChanRemoteService{}
	serviceImpl.On("RemoteCall", mock.Anything, "hello").Return(nil)
	ctx, _ := thrift.NewContext(0 * time.Second)

	ch, err := tchannel.NewChannel("remote", nil)
	assert.Equal(t, err, nil, "could not create tchannel")

	adapter, err := NewRingpopRemoteServiceAdapter(serviceImpl, rp, ch, RemoteServiceConfiguration{
		RemoteCall: &RemoteServiceRemoteCallConfiguration{
			Key: func(ctx thrift.Context, name string) (string, error) {
				return name, nil
			},
		},
	})

	tchanClient := &mocks.TChanClient{}
	tchanClient.On("Call", mock.Anything, "RemoteService", "RemoteCall", &RemoteServiceRemoteCallArgs{Name: "hello"}, mock.Anything).Return(true, nil)

	cf := adapter.(router.ClientFactory)
	remoteClient := cf.MakeRemoteClient(tchanClient).(TChanRemoteService)
	err = remoteClient.RemoteCall(ctx, "hello")
	assert.Equal(t, err, nil, "calling the remote client gave an error")

	tchanClient.AssertCalled(t, "Call", mock.Anything, "RemoteService", "RemoteCall", &RemoteServiceRemoteCallArgs{Name: "hello"}, mock.Anything)
}
