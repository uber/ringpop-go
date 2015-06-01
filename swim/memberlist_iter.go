// Copyright (c) 2015 Uber Technologies, Inc.
//
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

package swim

type memberIter interface {
	Next() (*Member, bool)
}

// A memberlistIter iterates on a memberlist. Whenever the iterator runs out of
// members, it shuffles the Memberlist and starts from the beginning.
type memberlistIter struct {
	m            *memberlist
	currentIndex int
	currentRound int
}

// NewMemberlistIter returns a new MemberlistIter
func newMemberlistIter(m *memberlist) *memberlistIter {
	iter := &memberlistIter{
		m:            m,
		currentIndex: -1,
		currentRound: 0,
	}

	iter.m.Shuffle()

	return iter
}

// Next returns the next pingable member in the member list, if it
// visits all members but none are pingable returns nil, false
func (i *memberlistIter) Next() (*Member, bool) {
	maxToVisit := i.m.NumMembers()
	visited := make(map[string]bool)

	for len(visited) < maxToVisit {
		i.currentIndex++

		if i.currentIndex >= i.m.NumMembers() {
			i.currentIndex = 0
			i.currentRound++
			i.m.Shuffle()
		}

		member := i.m.MemberAt(i.currentIndex)
		visited[member.Address] = true

		if i.m.Pingable(*member) {
			return member, true
		}
	}

	return nil, false
}
