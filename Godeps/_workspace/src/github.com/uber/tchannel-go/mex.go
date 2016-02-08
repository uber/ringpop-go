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

package tchannel

import (
	"errors"
	"sync"

	"github.com/uber/tchannel-go/typed"
	"golang.org/x/net/context"
)

var (
	errDuplicateMex        = errors.New("multiple attempts to use the message id")
	errMexChannelFull      = NewSystemError(ErrCodeBusy, "cannot send frame to message exchange channel")
	errUnexpectedFrameType = errors.New("unexpected frame received")
)

const (
	messageExchangeSetInbound  = "inbound"
	messageExchangeSetOutbound = "outbound"

	// mexChannelBufferSize is the size of the message exchange channel buffer.
	mexChannelBufferSize = 2
)

// A messageExchange tracks this Connections's side of a message exchange with a
// peer.  Each message exchange has a channel that can be used to receive
// frames from the peer, and a Context that can controls when the exchange has
// timed out or been cancelled.
type messageExchange struct {
	recvCh    chan *Frame
	ctx       context.Context
	msgID     uint32
	msgType   messageType
	mexset    *messageExchangeSet
	framePool FramePool
}

// forwardPeerFrame forwards a frame from a peer to the message exchange, where
// it can be pulled by whatever application thread is handling the exchange
func (mex *messageExchange) forwardPeerFrame(frame *Frame) error {
	if err := mex.ctx.Err(); err != nil {
		return GetContextError(err)
	}
	select {
	case mex.recvCh <- frame:
		return nil
	case <-mex.ctx.Done():
		// Note: One slow reader processing a large request could stall the connection.
		// If we see this, we need to increase the recvCh buffer size.
		return GetContextError(mex.ctx.Err())
	}
}

// recvPeerFrame waits for a new frame from the peer, or until the context
// expires or is cancelled
func (mex *messageExchange) recvPeerFrame() (*Frame, error) {
	if err := mex.ctx.Err(); err != nil {
		return nil, GetContextError(err)
	}

	select {
	case frame := <-mex.recvCh:
		if frame.Header.ID != mex.msgID {
			mex.mexset.log.WithFields(
				LogField{"msgId", mex.msgID},
				LogField{"header", frame.Header},
			).Error("recvPeerFrame received msg with unexpected ID.")
			return nil, errUnexpectedFrameType
		}
		return frame, nil
	case <-mex.ctx.Done():
		return nil, GetContextError(mex.ctx.Err())
	}
}

// recvPeerFrameOfType waits for a new frame of a given type from the peer, failing
// if the next frame received is not of that type.
// If an error frame is returned, then the errorMessage is returned as the error.
func (mex *messageExchange) recvPeerFrameOfType(msgType messageType) (*Frame, error) {
	frame, err := mex.recvPeerFrame()
	if err != nil {
		return nil, err
	}

	switch frame.Header.messageType {
	case msgType:
		return frame, nil

	case messageTypeError:
		// If we read an error frame, we can release it once we deserialize it.
		defer mex.framePool.Release(frame)

		errMsg := errorMessage{
			id: frame.Header.ID,
		}
		var rbuf typed.ReadBuffer
		rbuf.Wrap(frame.SizedPayload())
		if err := errMsg.read(&rbuf); err != nil {
			return nil, err
		}
		return nil, errMsg

	default:
		// TODO(mmihic): Should be treated as a protocol error
		mex.mexset.log.WithFields(
			LogField{"header", frame.Header},
			LogField{"expectedType", msgType},
			LogField{"expectedID", mex.msgID},
		).Warn("Received unexpected frame.")
		return nil, errUnexpectedFrameType
	}
}

// shutdown shuts down the message exchange, removing it from the message
// exchange set so  that it cannot receive more messages from the peer.  The
// receive channel remains open, however, in case there are concurrent
// goroutines sending to it.
func (mex *messageExchange) shutdown() {
	mex.mexset.removeExchange(mex.msgID)
}

// inboundTimeout is called when an exchange times out, but a handler may still be
// running in the background. Since the handler may still write to the exchange, we
// cannot shutdown the exchange, but we should remove it from the connection's
// exchange list.
func (mex *messageExchange) inboundTimeout() {
	mex.mexset.timeoutExchange(mex.msgID)
}

// A messageExchangeSet manages a set of active message exchanges.  It is
// mainly used to route frames from a peer to the appropriate messageExchange,
// or to cancel or mark a messageExchange as being in error.  Each Connection
// maintains two messageExchangeSets, one to manage exchanges that it has
// initiated (outbound), and another to manage exchanges that the peer has
// initiated (inbound).  The message-type specific handlers are responsible for
// ensuring that their message exchanges are properly registered and removed
// from the corresponding exchange set.
type messageExchangeSet struct {
	sync.RWMutex

	log        Logger
	name       string
	onRemoved  func()
	onAdded    func()
	sendChRefs sync.WaitGroup

	// exchanges is mutable, and is protected by the mutex.
	exchanges map[uint32]*messageExchange
}

// newExchange creates and adds a new message exchange to this set
func (mexset *messageExchangeSet) newExchange(ctx context.Context, framePool FramePool,
	msgType messageType, msgID uint32, bufferSize int) (*messageExchange, error) {
	if mexset.log.Enabled(LogLevelDebug) {
		mexset.log.Debugf("Creating new %s message exchange for [%v:%d]", mexset.name, msgType, msgID)
	}

	mex := &messageExchange{
		msgType:   msgType,
		msgID:     msgID,
		ctx:       ctx,
		recvCh:    make(chan *Frame, bufferSize),
		mexset:    mexset,
		framePool: framePool,
	}

	mexset.Lock()
	if existingMex := mexset.exchanges[mex.msgID]; existingMex != nil {
		if existingMex == mex {
			mexset.log.WithFields(
				LogField{"name", mexset.name},
				LogField{"msgType", mex.msgType},
				LogField{"msgID", mex.msgID},
			).Warn("mex registered multiple times.")
		} else {
			mexset.log.WithFields(
				LogField{"msgID", mex.msgID},
				LogField{"existingType", existingMex.msgType},
				LogField{"newType", mex.msgType},
			).Warn("Duplicate msg ID for active and new mex.")
		}

		mexset.Unlock()
		return nil, errDuplicateMex
	}

	mexset.exchanges[mex.msgID] = mex
	mexset.sendChRefs.Add(1)
	mexset.Unlock()

	mexset.onAdded()

	// TODO(mmihic): Put into a deadline ordered heap so we can garbage collected expired exchanges
	return mex, nil
}

// removeExchange removes a message exchange from the set, if it exists.
// It decrements the sendChRefs wait group, signalling that this exchange no longer has
// any active goroutines that will try to send to sendCh.
func (mexset *messageExchangeSet) removeExchange(msgID uint32) {
	if mexset.log.Enabled(LogLevelDebug) {
		mexset.log.Debugf("Removing %s message exchange %d", mexset.name, msgID)
	}

	mexset.Lock()
	delete(mexset.exchanges, msgID)
	mexset.Unlock()

	mexset.sendChRefs.Done()
	mexset.onRemoved()
}

// timeoutExchange is similar to removeExchange, however it does not decrement
// the sendChRefs wait group.
func (mexset *messageExchangeSet) timeoutExchange(msgID uint32) {
	mexset.log.Debugf("Removing %s message exchange %d due to timeout", mexset.name, msgID)

	mexset.Lock()
	delete(mexset.exchanges, msgID)
	mexset.Unlock()

	mexset.onRemoved()
}

// waitForSendCh waits for all goroutines with references to sendCh to complete.
func (mexset *messageExchangeSet) waitForSendCh() {
	mexset.sendChRefs.Wait()
}

func (mexset *messageExchangeSet) count() int {
	mexset.RLock()
	count := len(mexset.exchanges)
	mexset.RUnlock()

	return count
}

// forwardPeerFrame forwards a frame from the peer to the appropriate message
// exchange
func (mexset *messageExchangeSet) forwardPeerFrame(frame *Frame) error {
	if mexset.log.Enabled(LogLevelDebug) {
		mexset.log.Debugf("forwarding %s %s", mexset.name, frame.Header)
	}

	mexset.RLock()
	mex := mexset.exchanges[frame.Header.ID]
	mexset.RUnlock()

	if mex == nil {
		// This is ok since the exchange might have expired or been cancelled
		mexset.log.Infof("received frame %s for %s message exchange that no longer exists",
			frame.Header, mexset.name)
		return nil
	}

	if err := mex.forwardPeerFrame(frame); err != nil {
		mexset.log.Infof("Unable to forward frame %v length %v to %s: %v",
			frame.Header, frame.Header.FrameSize(), mexset.name, err)
		return err
	}

	return nil
}
