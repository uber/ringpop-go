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

package tchannel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/testutils"
)

type chanSet struct {
	main     tchannel.Registrar
	sub      tchannel.Registrar
	isolated tchannel.Registrar
}

func withNewSet(t *testing.T, f func(*testing.T, chanSet)) {
	ch := testutils.NewClient(t, nil)
	f(t, chanSet{
		main:     ch,
		sub:      ch.GetSubChannel("hyperbahn"),
		isolated: ch.GetSubChannel("ringpop", tchannel.Isolated),
	})
}

// Assert that two Registrars have references to the same Peer.
func assertHaveSameRef(t *testing.T, r1, r2 tchannel.Registrar) {
	p1, err := r1.Peers().Get(nil)
	assert.NoError(t, err, "First registrar has no peers.")

	p2, err := r2.Peers().Get(nil)
	assert.NoError(t, err, "Second registrar has no peers.")

	assert.True(t, p1 == p2, "Registrars have references to different peers.")
}

func assertNoPeer(t *testing.T, r tchannel.Registrar) {
	_, err := r.Peers().Get(nil)
	assert.Equal(t, err, tchannel.ErrNoPeers)
}

func TestMainAddVisibility(t *testing.T) {
	withNewSet(t, func(t *testing.T, set chanSet) {
		// Adding a peer to the main channel should be reflected in the
		// subchannel, but not the isolated subchannel.
		set.main.Peers().Add("127.0.0.1:3000")
		assertHaveSameRef(t, set.main, set.sub)
		assertNoPeer(t, set.isolated)
	})
}

func TestSubchannelAddVisibility(t *testing.T) {
	withNewSet(t, func(t *testing.T, set chanSet) {
		// Adding a peer to a non-isolated subchannel should be reflected in
		// the main channel but not in isolated siblings.
		set.sub.Peers().Add("127.0.0.1:3000")
		assertHaveSameRef(t, set.main, set.sub)
		assertNoPeer(t, set.isolated)
	})
}

func TestIsolatedAddVisibility(t *testing.T) {
	withNewSet(t, func(t *testing.T, set chanSet) {
		// Adding a peer to an isolated subchannel shouldn't change the main
		// channel or sibling channels.
		set.isolated.Peers().Add("127.0.0.1:3000")

		_, err := set.isolated.Peers().Get(nil)
		assert.NoError(t, err)

		assertNoPeer(t, set.main)
		assertNoPeer(t, set.sub)
	})
}

func TestAddReusesPeers(t *testing.T) {
	withNewSet(t, func(t *testing.T, set chanSet) {
		// Adding to both a channel and an isolated subchannel shouldn't create
		// two separate peers.
		set.main.Peers().Add("127.0.0.1:3000")
		set.isolated.Peers().Add("127.0.0.1:3000")

		assertHaveSameRef(t, set.main, set.sub)
		assertHaveSameRef(t, set.main, set.isolated)
	})
}
