// Copyright (c) 2016 Uber Technologies, Inc.
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

import "time"

// AttemptHeal attempts to heal a partition between the node and the target.
//
// Be mindfull that calling this function will not result in a heal when there
// are nodes that need to be reincarated to take precedence over the faulty
// declarations that occur during a network partition. A cluster may therefore
// need multiple calls to this function with some time in between to heal.
//
// Check out ringpop-common/docs for a full description of the algorithm.
func AttemptHeal(node *Node, target string) error {
	node.emit(AttemptHealEvent{})
	node.logger.WithField("target", target).Info("attempt heal")

	// If join request succeeds a partition is detected,
	// this node will now coordinate the healing mechanism.
	joinRes, err := sendJoinRequest(node, target, time.Second)
	if err != nil {
		return err
	}

	A := node.disseminator.FullSync()
	B := joinRes.Membership
	var csForA, csForB []Change

	// Find changes that are alive for B and faulty for A and visa versa.
	for _, m2 := range B {
		m1, ok := selectMember(A, m2.Address)
		if !ok {
			continue
		}

		// faulty for partition A, alive for partition B
		// needs reincarnation in partition B.
		if m1.Status == Faulty && m2.Status == Alive && m1.Incarnation >= m2.Incarnation {
			copy := m1
			copy.Status = Suspect
			csForB = append(csForB, copy)
		}

		// alive for partition A, faulty for partition B
		// needs reincarnation in partition A.
		if m1.Status == Alive && m2.Status == Faulty && m1.Incarnation <= m2.Incarnation {
			copy := m2
			copy.Status = Suspect
			csForA = append(csForA, copy)
		}
	}
	// Merge partitions if no node needs to reincarnate,
	if len(csForA) == 0 && len(csForB) == 0 {
		node.healer.logger.WithField("target", target).Info("merge two partitions")

		// Add membership of B to this node, so that the membership
		// information of B will be disseminated through A.
		node.memberlist.Update(B)

		// Send membership of A to the target node, so that the membership
		// information of partition A will be disseminated through B.
		A1 := node.disseminator.FullSync()
		_, err := sendPingWithChanges(node, target, A1, time.Second)
		if err != nil {
			return err
		}

		return nil
	}

	// reincarnate all nodes by disseminating that they are suspect
	node.healer.logger.WithField("target", target).Info("reincarnate nodes before we can merge the partitions")
	node.memberlist.Update(csForA)

	_, err = sendPingWithChanges(node, target, csForB, time.Second)
	if err != nil {
		return err
	}

	return nil
}

// selectMember selects the member with the specified address from the partition.
func selectMember(partition []Change, address string) (Change, bool) {
	for _, m := range partition {
		if m.Address == address {
			return m, true
		}
	}

	return Change{}, false
}
