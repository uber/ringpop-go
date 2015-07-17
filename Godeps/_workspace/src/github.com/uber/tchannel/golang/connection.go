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
	"sync"
	"sync/atomic"

	"github.com/uber/tchannel/golang/typed"
	"golang.org/x/net/context"
)

// PeerInfo contains information about a TChannel peer
type PeerInfo struct {
	// The host and port that can be used to contact the peer, as encoded by net.JoinHostPort
	HostPort string

	// The logical process name for the peer, used for only for logging / debugging
	ProcessName string
}

func (p PeerInfo) String() string {
	return fmt.Sprintf("%s(%s)", p.HostPort, p.ProcessName)
}

// IsEphemeral returns if hostPort is the default ephemeral hostPort.
func (p PeerInfo) IsEphemeral() bool {
	return p.HostPort == "" || p.HostPort == ephemeralHostPort
}

// LocalPeerInfo adds service name to the peer info, only required for the local peer.
type LocalPeerInfo struct {
	PeerInfo

	// ServiceName is the service name for the local peer.
	ServiceName string
}

func (p LocalPeerInfo) String() string {
	return fmt.Sprintf("%v: %v", p.ServiceName, p.PeerInfo)
}

// CurrentProtocolVersion is the current version of the TChannel protocol
// supported by this stack
const CurrentProtocolVersion = 0x02

var (
	// ErrConnectionClosed is returned when a caller performs an operation
	// on a closed connection
	ErrConnectionClosed = errors.New("connection is closed")

	// ErrConnectionNotReady is returned when a caller attempts to send a
	// request through a connection which has not yet been initialized
	ErrConnectionNotReady = errors.New("connection is not yet ready")

	// ErrSendBufferFull is returned when a message cannot be sent to the
	// peer because the frame sending buffer has become full.  Typically
	// this indicates that the connection is stuck and writes have become
	// backed up
	ErrSendBufferFull = errors.New("connection send buffer is full, cannot send frame")

	errConnectionAlreadyActive     = errors.New("connection is already active")
	errConnectionWaitingOnPeerInit = errors.New("connection is waiting for the peer to sent init")
	errCannotHandleInitRes         = errors.New("could not return init-res to handshake thread")
)

// ConnectionOptions are options that control the behavior of a Connection
type ConnectionOptions struct {
	// The frame pool, allowing better management of frame buffers.  Defaults to using raw heap
	FramePool FramePool

	// The size of receive channel buffers.  Defaults to 512
	RecvBufferSize int

	// The size of send channel buffers.  Defaults to 512
	SendBufferSize int

	// The type of checksum to use when sending messages
	ChecksumType ChecksumType
}

// OnActiveHandler is the event handler for when a connection becomes active.
type OnActiveHandler func(c *Connection)

// Connection represents a connection to a remote peer.
type Connection struct {
	log             Logger
	statsReporter   StatsReporter
	checksumType    ChecksumType
	framePool       FramePool
	conn            net.Conn
	localPeerInfo   LocalPeerInfo
	remotePeerInfo  PeerInfo
	sendCh          chan *Frame
	state           connectionState
	stateMut        sync.RWMutex
	inbound         messageExchangeSet
	outbound        messageExchangeSet
	handlers        *handlerMap
	nextMessageID   uint32
	onActive        OnActiveHandler
	commonStatsTags map[string]string
}

// nextConnID gives an ID for each connection for debugging purposes.
var nextConnID uint32

type connectionState int

const (
	// Connection initiated by peer is waiting to recv init-req from peer
	connectionWaitingToRecvInitReq connectionState = iota

	// Connection initated by current process is waiting to send init-req to peer
	connectionWaitingToSendInitReq

	// Connection initiated by current process has sent init-req, and is
	// waiting for init-req
	connectionWaitingToRecvInitRes

	// Connection is fully active
	connectionActive

	// Connection is starting to close; new incoming requests are rejected, outbound
	// requests are allowed to proceed
	connectionStartClose

	// Connection has finished processing all active inbound, and is
	// waiting for outbound requests to complete or timeout
	connectionInboundClosed

	// Connection is fully closed
	connectionClosed
)

// Creates a new Connection around an outbound connection initiated to a peer
func (ch *Channel) newOutboundConnection(hostPort string, opts *ConnectionOptions) (*Connection, error) {
	conn, err := net.Dial("tcp", hostPort)
	if err != nil {
		return nil, err
	}

	return ch.newConnection(conn, connectionWaitingToSendInitReq, nil, opts), nil
}

// Creates a new Connection based on an incoming connection from a peer
func (ch *Channel) newInboundConnection(conn net.Conn, onActive OnActiveHandler, opts *ConnectionOptions) (*Connection, error) {
	return ch.newConnection(conn, connectionWaitingToRecvInitReq, onActive, opts), nil
}

// Creates a new connection in a given initial state
func (ch *Channel) newConnection(conn net.Conn, initialState connectionState, onActive OnActiveHandler, opts *ConnectionOptions) *Connection {
	if opts == nil {
		opts = &ConnectionOptions{}
	}

	checksumType := opts.ChecksumType
	if checksumType == ChecksumTypeNone {
		checksumType = ChecksumTypeCrc32C
	}

	sendBufferSize := opts.SendBufferSize
	if sendBufferSize <= 0 {
		sendBufferSize = 512
	}

	recvBufferSize := opts.RecvBufferSize
	if recvBufferSize <= 0 {
		recvBufferSize = 512
	}

	framePool := opts.FramePool
	if framePool == nil {
		framePool = DefaultFramePool
	}

	connID := atomic.AddUint32(&nextConnID, 1)
	log := PrefixedLogger(fmt.Sprintf("C%v ", connID), ch.log)
	peerInfo := ch.PeerInfo()
	log.Debugf("created for %v (%v) local: %v remote: %v",
		peerInfo.ServiceName, peerInfo.ProcessName, conn.LocalAddr(), conn.RemoteAddr())
	c := &Connection{
		log:           log,
		statsReporter: ch.statsReporter,
		conn:          conn,
		framePool:     framePool,
		state:         initialState,
		sendCh:        make(chan *Frame, sendBufferSize),
		localPeerInfo: peerInfo,
		checksumType:  checksumType,
		inbound: messageExchangeSet{
			name:      messageExchangeSetInbound,
			log:       log,
			exchanges: make(map[uint32]*messageExchange),
		},
		outbound: messageExchangeSet{
			name:      messageExchangeSetOutbound,
			log:       log,
			exchanges: make(map[uint32]*messageExchange),
		},
		handlers:        ch.handlers,
		onActive:        onActive,
		commonStatsTags: ch.commonStatsTags,
	}

	go c.readFrames()
	go c.writeFrames()
	return c
}

// IsActive returns whether this connection is in an active state.
func (c *Connection) IsActive() bool {
	var isActive bool
	c.withStateRLock(func() error {
		isActive = (c.state == connectionActive)
		return nil
	})
	return isActive
}

func (c *Connection) callOnActive() {
	if f := c.onActive; f != nil {
		f(c)
	}
}

// Initiates a handshake with a peer.
func (c *Connection) sendInit(ctx context.Context) error {
	err := c.withStateLock(func() error {
		switch c.state {
		case connectionWaitingToSendInitReq:
			c.state = connectionWaitingToRecvInitRes
			return nil
		case connectionWaitingToRecvInitReq:
			return errConnectionWaitingOnPeerInit
		case connectionClosed, connectionStartClose, connectionInboundClosed:
			return ErrConnectionClosed
		case connectionActive, connectionWaitingToRecvInitRes:
			return errConnectionAlreadyActive
		default:
			return fmt.Errorf("connection in unknown state %d", c.state)
		}
	})
	if err != nil {
		return err
	}

	initMsgID := c.NextMessageID()
	req := initReq{initMessage{id: initMsgID}}
	req.Version = CurrentProtocolVersion
	req.initParams = initParams{
		InitParamHostPort:    c.localPeerInfo.HostPort,
		InitParamProcessName: c.localPeerInfo.ProcessName,
	}

	mex, err := c.outbound.newExchange(ctx, c.framePool, req.messageType(), req.ID(), 1)
	if err != nil {
		return c.connectionError(err)
	}

	defer c.outbound.removeExchange(req.ID())

	if err := c.sendMessage(&req); err != nil {
		return c.connectionError(err)
	}

	res := initRes{initMessage{id: initMsgID}}
	err = c.recvMessage(ctx, &res, mex.recvCh)
	if err != nil {
		return c.connectionError(err)
	}

	if res.Version != CurrentProtocolVersion {
		return c.connectionError(fmt.Errorf("Unsupported protocol version %d from peer", res.Version))
	}

	c.remotePeerInfo.HostPort = res.initParams[InitParamHostPort]
	if c.remotePeerInfo.IsEphemeral() {
		c.remotePeerInfo.HostPort = c.conn.RemoteAddr().String()
	}
	c.remotePeerInfo.ProcessName = res.initParams[InitParamProcessName]

	c.withStateLock(func() error {
		if c.state == connectionWaitingToRecvInitRes {
			c.state = connectionActive
		}
		return nil
	})

	c.callOnActive()
	return nil
}

// Handles an incoming InitReq.  If we are waiting for the peer to send us an
// InitReq, and the InitReq is valid, send a corresponding InitRes and mark
// ourselves as active
func (c *Connection) handleInitReq(frame *Frame) {
	if err := c.withStateRLock(func() error {
		return nil
	}); err != nil {
		c.connectionError(err)
		return
	}

	var req initReq
	rbuf := typed.NewReadBuffer(frame.SizedPayload())
	if err := req.read(rbuf); err != nil {
		// TODO(mmihic): Technically probably a protocol error
		c.connectionError(err)
		return
	}

	if req.Version != CurrentProtocolVersion {
		// TODO(mmihic): Send protocol error
		c.connectionError(fmt.Errorf("Unsupported protocol version %d from peer", req.Version))
		return
	}

	c.remotePeerInfo.HostPort = req.initParams[InitParamHostPort]
	c.remotePeerInfo.ProcessName = req.initParams[InitParamProcessName]
	if c.remotePeerInfo.IsEphemeral() {
		c.remotePeerInfo.HostPort = c.conn.RemoteAddr().String()
	}

	res := initRes{initMessage{id: frame.Header.ID}}
	res.initParams = initParams{
		InitParamHostPort:    c.localPeerInfo.HostPort,
		InitParamProcessName: c.localPeerInfo.ProcessName,
	}
	res.Version = CurrentProtocolVersion
	if err := c.sendMessage(&res); err != nil {
		c.connectionError(err)
		return
	}

	c.withStateLock(func() error {
		switch c.state {
		case connectionWaitingToRecvInitReq:
			c.state = connectionActive
		}

		return nil
	})

	c.callOnActive()
}

// ping sends a ping message and waits for a ping response.
func (c *Connection) ping(ctx context.Context) error {
	req := &pingReq{id: c.NextMessageID()}
	mex, err := c.outbound.newExchange(ctx, c.framePool, req.messageType(), req.ID(), 1)
	if err != nil {
		return c.connectionError(err)
	}
	defer c.outbound.removeExchange(req.ID())

	if err := c.sendMessage(req); err != nil {
		return c.connectionError(err)
	}

	res := &pingRes{}
	err = c.recvMessage(ctx, res, mex.recvCh)
	if err != nil {
		return c.connectionError(err)
	}

	return nil
}

// handlePingRes calls registered ping handlers.
func (c *Connection) handlePingRes(frame *Frame) bool {
	if err := c.outbound.forwardPeerFrame(frame); err != nil {
		c.log.Warnf("Got unexpected ping response: %+v", frame.Header)
		return true
	}
	// ping req is waiting for this frame, and will release it.
	return false
}

// handlePingReq responds to the pingReq message with a pingRes.
func (c *Connection) handlePingReq(frame *Frame) {
	pingRes := &pingRes{id: frame.Header.ID}
	if err := c.sendMessage(pingRes); err != nil {
		c.connectionError(err)
	}
}

// Handles an incoming InitRes.  If we are waiting for the peer to send us an
// InitRes, forward the InitRes to the waiting goroutine

// TODO(mmihic): There is a race condition here, in that the peer might start
// sending us requests before the goroutine doing initialization has a chance
// to process the InitRes.  We probably want to move the InitRes checking to
// here (where it will run in the receiver goroutine and thus block new
// incoming messages), and simply signal the init goroutine that we are done
func (c *Connection) handleInitRes(frame *Frame) bool {
	if err := c.withStateRLock(func() error {
		switch c.state {
		case connectionWaitingToRecvInitRes:
			return nil
		case connectionClosed, connectionStartClose, connectionInboundClosed:
			return ErrConnectionClosed

		case connectionActive:
			return errConnectionAlreadyActive

		case connectionWaitingToSendInitReq:
			return ErrConnectionNotReady

		case connectionWaitingToRecvInitReq:
			return errConnectionWaitingOnPeerInit

		default:
			return fmt.Errorf("Connection in unknown state %d", c.state)
		}
	}); err != nil {
		c.connectionError(err)
		return true
	}

	if err := c.outbound.forwardPeerFrame(frame); err != nil {
		c.connectionError(errCannotHandleInitRes)
		return true
	}

	// init req waits for this message and will release it when done.
	return false
}

// sendMessage sends a standalone message (typically a control message)
func (c *Connection) sendMessage(msg message) error {
	frame := c.framePool.Get()
	if err := frame.write(msg); err != nil {
		c.framePool.Release(frame)
		return err
	}

	select {
	case c.sendCh <- frame:
		return nil
	default:
		return ErrSendBufferFull
	}
}

// recvMessage blocks waiting for a standalone response message (typically a
// control message)
func (c *Connection) recvMessage(ctx context.Context, msg message, resCh <-chan *Frame) error {
	select {
	case <-ctx.Done():
		return ctx.Err()

	case frame := <-resCh:
		err := frame.read(msg)
		c.framePool.Release(frame)
		return err
	}
}

// NextMessageID reserves the next available message id for this connection
func (c *Connection) NextMessageID() uint32 {
	return atomic.AddUint32(&c.nextMessageID, 1)
}

// connectionError handles a connection level error
func (c *Connection) connectionError(err error) error {
	doClose := false
	c.withStateLock(func() error {
		if c.state != connectionClosed {
			c.state = connectionClosed
			doClose = true
		}
		return nil
	})

	if doClose {
		c.closeNetwork()
	}

	return NewWrappedSystemError(ErrCodeNetwork, err)
}

// withStateLock performs an action with the connection state mutex locked
func (c *Connection) withStateLock(f func() error) error {
	c.stateMut.Lock()
	defer c.stateMut.Unlock()

	return f()
}

// withStateRLock an action with the connection state mutex held in a read lock
func (c *Connection) withStateRLock(f func() error) error {
	c.stateMut.RLock()
	defer c.stateMut.RUnlock()

	return f()
}

// readFrames is the loop that reads frames from the network connection and
// dispatches to the appropriate handler. Run within its own goroutine to
// prevent overlapping reads on the socket.  Most handlers simply send the
// incoming frame to a channel; the init handlers are a notable exception,
// since we cannot process new frames until the initialization is complete.
func (c *Connection) readFrames() {
	for {
		frame := c.framePool.Get()
		if err := frame.ReadFrom(c.conn); err != nil {
			c.framePool.Release(frame)
			c.connectionError(err)
			return
		}

		// call req and call res messages may not want the frame released immediately.
		releaseFrame := true
		switch frame.Header.messageType {
		case messageTypeCallReq:
			releaseFrame = c.handleCallReq(frame)
		case messageTypeCallReqContinue:
			releaseFrame = c.handleCallReqContinue(frame)
		case messageTypeCallRes:
			releaseFrame = c.handleCallRes(frame)
		case messageTypeCallResContinue:
			releaseFrame = c.handleCallResContinue(frame)
		case messageTypeInitReq:
			c.handleInitReq(frame)
		case messageTypeInitRes:
			releaseFrame = c.handleInitRes(frame)
		case messageTypePingReq:
			c.handlePingReq(frame)
		case messageTypePingRes:
			releaseFrame = c.handlePingRes(frame)
		case messageTypeError:
			c.handleError(frame)
		default:
			// TODO(mmihic): Log and close connection with protocol error
			c.log.Errorf("Received unexpected frame %s from %s", frame.Header, c.remotePeerInfo)
		}

		if releaseFrame {
			c.framePool.Release(frame)
		}
	}
}

// writeFrames is the main loop that pulls frames from the send channel and
// writes them to the connection.
func (c *Connection) writeFrames() {
	for f := range c.sendCh {
		c.log.Debugf("Writing frame %s", f.Header)

		err := f.WriteTo(c.conn)
		c.framePool.Release(f)
		if err != nil {
			c.connectionError(err)
			return
		}
	}

	// Close the network after we have sent the last frame
	c.closeNetwork()
}

// closeNetwork closes the network connection and all network-related channels.
// This should only be done in response to a fatal connection or protocol
// error, or after all pending frames have been sent.
func (c *Connection) closeNetwork() {
	// NB(mmihic): The sender goroutine will exit once the connection is
	// closed; no need to close the send channel (and closing the send
	// channel would be dangerous since other goroutine might be sending)
	if err := c.conn.Close(); err != nil {
		c.log.Warnf("could not close connection to peer %s: %v", c.remotePeerInfo, err)
	}
}
