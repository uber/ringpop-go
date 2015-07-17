package tchannel

// Copyright (c) 2015 Uber Technologies, Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/net/context"
)

var (
	errAlreadyListening = errors.New("channel already listening")
)

const (
	ephemeralHostPort = "0.0.0.0:0"
)

// ChannelOptions are used to control parameters on a create a TChannel
type ChannelOptions struct {
	// Default Connection options
	DefaultConnectionOptions ConnectionOptions

	// The name of the process, for logging and reporting to peers
	ProcessName string

	// The logger to use for this channel
	Logger Logger

	// The reporter to use for reporting stats for this channel.
	StatsReporter StatsReporter
}

// A Channel is a bi-directional connection to the peering and routing network.
// Applications can use a Channel to make service calls to remote peers via
// BeginCall, or to listen for incoming calls from peers.  Applications that
// want to receive requests should call one of Serve or ListenAndServe
// TODO(prashant): Shutdown all subchannels + peers when channel is closed.
type Channel struct {
	log               Logger
	commonStatsTags   map[string]string
	statsReporter     StatsReporter
	connectionOptions ConnectionOptions
	handlers          *handlerMap
	peers             *PeerList

	// mutable contains all the members of Channel which are mutable.
	mutable struct {
		mut         sync.RWMutex // protects members of the mutable struct.
		closed      bool
		peerInfo    LocalPeerInfo // May be ephemeral if this is a client only channel
		l           net.Listener  // May be nil if this is a client only channel
		subChannels map[string]*SubChannel
	}
}

// NewChannel creates a new Channel.  The new channel can be used to send outbound requests
// to peers, but will not listen or handling incoming requests until one of ListenAndServe
// or Serve is called. The local service name should be passed to serviceName.
func NewChannel(serviceName string, opts *ChannelOptions) (*Channel, error) {
	if opts == nil {
		opts = &ChannelOptions{}
	}

	logger := opts.Logger
	if logger == nil {
		logger = NullLogger
	}

	processName := opts.ProcessName
	if processName == "" {
		processName = fmt.Sprintf("%s[%d]", filepath.Base(os.Args[0]), os.Getpid())
	}

	statsReporter := opts.StatsReporter
	if statsReporter == nil {
		statsReporter = NullStatsReporter
	}

	ch := &Channel{
		connectionOptions: opts.DefaultConnectionOptions,
		log:               logger,
		statsReporter:     statsReporter,
		handlers:          &handlerMap{},
	}
	ch.mutable.peerInfo = LocalPeerInfo{
		PeerInfo: PeerInfo{
			ProcessName: processName,
			HostPort:    ephemeralHostPort,
		},
		ServiceName: serviceName,
	}
	ch.mutable.subChannels = make(map[string]*SubChannel)
	ch.peers = newPeerList(ch)
	ch.createCommonStats()
	return ch, nil
}

// Serve serves incoming requests using the provided listener.
// The local peer info is set synchronously, but the actual socket listening is done in
// a separate goroutine.
func (ch *Channel) Serve(l net.Listener) error {
	mutable := &ch.mutable
	mutable.mut.Lock()
	defer mutable.mut.Unlock()

	if mutable.l != nil {
		return errAlreadyListening
	}

	mutable.l = l
	mutable.peerInfo.HostPort = l.Addr().String()

	peerInfo := mutable.peerInfo
	ch.log.Debugf("%v (%v) listening on %v", peerInfo.ProcessName, peerInfo.ServiceName, peerInfo.HostPort)
	go ch.serve()
	return nil
}

// ListenAndServe listens on the given address and serves incoming requests.
// The port may be 0, in which case the channel will use an OS assigned port
// This method does not block as the handling of connections is done in a goroutine.
func (ch *Channel) ListenAndServe(hostPort string) error {
	mutable := &ch.mutable
	mutable.mut.RLock()

	if mutable.l != nil {
		mutable.mut.RUnlock()
		return errAlreadyListening
	}

	l, err := net.Listen("tcp", hostPort)
	if err != nil {
		mutable.mut.RUnlock()
		return err
	}

	mutable.mut.RUnlock()
	return ch.Serve(l)
}

// Register registers a handler for a service+operation pair
func (ch *Channel) Register(h Handler, operationName string) {
	ch.handlers.register(h, ch.PeerInfo().ServiceName, operationName)
}

// PeerInfo returns the current peer info for the channel
func (ch *Channel) PeerInfo() LocalPeerInfo {
	ch.mutable.mut.RLock()
	defer ch.mutable.mut.RUnlock()

	return ch.mutable.peerInfo
}

func (ch *Channel) createCommonStats() {
	ch.commonStatsTags = map[string]string{
		"app":     ch.mutable.peerInfo.ProcessName,
		"service": ch.mutable.peerInfo.ServiceName,
	}
	host, err := os.Hostname()
	if err != nil {
		ch.log.Infof("channel failed to get host: %v", err)
		return
	}
	ch.commonStatsTags["host"] = host
	// TODO(prashant): Allow user to pass extra tags (such as cluster, version).
}

func (ch *Channel) registerNewSubChannel(serviceName string) *SubChannel {
	mutable := &ch.mutable
	mutable.mut.Lock()
	defer mutable.mut.Unlock()

	// Recheck for the subchannel under the write lock.
	if sc, ok := mutable.subChannels[serviceName]; ok {
		return sc
	}

	sc := newSubChannel(serviceName, ch.peers)
	mutable.subChannels[serviceName] = sc
	return sc
}

// GetSubChannel returns a SubChannel for the given service name. If the subchannel does not
// exist, it is created.
func (ch *Channel) GetSubChannel(serviceName string) *SubChannel {
	mutable := &ch.mutable
	mutable.mut.RLock()

	if sc, ok := mutable.subChannels[serviceName]; ok {
		mutable.mut.RUnlock()
		return sc
	}

	mutable.mut.RUnlock()
	return ch.registerNewSubChannel(serviceName)
}

// Peers returns the PeerList for the channel.
func (ch *Channel) Peers() *PeerList {
	return ch.peers
}

// BeginCall starts a new call to a remote peer, returning an OutboundCall that can
// be used to write the arguments of the call.
func (ch *Channel) BeginCall(ctx context.Context, hostPort, serviceName, operationName string, callOptions *CallOptions) (*OutboundCall, error) {
	p := ch.peers.GetOrAdd(hostPort)
	return p.BeginCall(ctx, serviceName, operationName, callOptions)
}

// serve runs the listener to accept and manage new incoming connections, blocking
// until the channel is closed.
func (ch *Channel) serve() {
	acceptBackoff := 0 * time.Millisecond

	for {
		netConn, err := ch.mutable.l.Accept()
		if err != nil {
			// Backoff from new accepts if this is a temporary error
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if acceptBackoff == 0 {
					acceptBackoff = 5 * time.Millisecond
				} else {
					acceptBackoff *= 2
				}
				if max := 1 * time.Second; acceptBackoff > max {
					acceptBackoff = max
				}
				ch.log.Warnf("accept error: %v; retrying in %v", err, acceptBackoff)
				time.Sleep(acceptBackoff)
				continue
			} else {
				// Only log an error if we are not shutdown.
				if ch.Closed() {
					return
				}
				ch.log.Fatalf("unrecoverable accept error: %v; closing server", err)
				return
			}
		}

		acceptBackoff = 0

		// Register the connection in the peer once the channel is set up.
		onActive := func(c *Connection) {
			ch.log.Debugf("Add connection as an active peer for %v", c.remotePeerInfo.HostPort)
			p := ch.peers.GetOrAdd(c.remotePeerInfo.HostPort)
			p.AddConnection(c)
		}
		_, err = ch.newInboundConnection(netConn, onActive, &ch.connectionOptions)
		if err != nil {
			// Server is getting overloaded - begin rejecting new connections
			ch.log.Errorf("could not create new TChannelConnection for incoming conn: %v", err)
			netConn.Close()
			continue
		}
	}
}

// Ping sends a ping message to the given hostPort and waits for a response.
func (ch *Channel) Ping(ctx context.Context, hostPort string) error {
	peer := ch.Peers().GetOrAdd(hostPort)
	conn, err := peer.GetConnection(ctx)
	if err != nil {
		return err
	}

	return conn.ping(ctx)
}

// Logger returns the logger for this channel.
func (ch *Channel) Logger() Logger {
	return ch.log
}

// Closed returns whether this channel has been closed with .Close()
func (ch *Channel) Closed() bool {
	ch.mutable.mut.RLock()
	defer ch.mutable.mut.RUnlock()
	return ch.mutable.closed
}

// Close closes the channel including all connections to any active peers.
func (ch *Channel) Close() {
	ch.mutable.mut.Lock()
	defer ch.mutable.mut.Unlock()

	ch.mutable.closed = true
	if ch.mutable.l != nil {
		ch.mutable.l.Close()
	}
	ch.peers.Close()
}
