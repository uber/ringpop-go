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
// It is the responsibillity of the caller to detect that the node is indeed
// separated in two partitions. Let A be the partition that this node is part
// of, and B be the partition that the target node is part of.
//
// The algorithm is as follows: First send a join request to the target and
// compare its membership with the membership of this node. Any node that
// is faulty in the target's membership and alive in this node's membership,
// needs to be reincarnated. We force this by marking the respective nodes
// as suspect in this node's membership. When the nodes receive the change
// through dissemination they will reassert their aliveness and reincarnate.
// Simmilarly for node that are faulty in this node's memberhsip and alive for
// the target's membership. We force the reincarnation of those nodes by sending
// a ping to the target in which we mark those nodes as suspect. Those nodes
// will now also be disseminated and reincarnated.
//
// When there are no nodes that are alive to one partition and faulty to the
// other partition, we can safely merge the two partitions. We do this by
// applying the membership from B to this node and by sending a ping with A's
// membership to the target node. Dissemination will cause this information
// to spread through the two partitions.
//
// Be mindfull that calling this function will not result in a heal when there
// are nodes that need to be reincarated. A cluster may therefore need
// multiple calls to this function with some time in between to heal.
func AttemptHeal(node *Node, target string) error {
	// If join request succeeds a partition is detected,
	// this node will now coordinate the healing mechanism.
	joinRes, err := sendJoinRequest(node, target, time.Second/2)
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
		if m1.Status == Faulty && m2.Status == Alive {
			copy := m1
			copy.Status = Suspect
			csForB = append(csForB, copy)
		}

		// alive for partition A, faulty for partition B
		// needs reincarnation in partition A.
		if m1.Status == Alive && m2.Status == Faulty {
			copy := m2
			copy.Status = Suspect
			csForA = append(csForA, copy)
		}
	}

	// Merge partitions if no node needs to reincarnate,
	if len(csForA) == 0 && len(csForB) == 0 {
		// TODO: send out event/log that we are merging

		// Add membership of B to this node, so that the membership
		// information of B will be disseminated through A.
		node.memberlist.Update(B)

		// Send membership of A to the target node, so that the membership
		// information of partition A will be disseminated through B.
		_, err := sendPingWithChanges(node, target, A, time.Second)
		if err != nil {
			return err
		}

		return nil
	}

	// reincarnate all nodes by disseminating that they are suspect
	// TODO: send out event/log that we are reincarnating
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
