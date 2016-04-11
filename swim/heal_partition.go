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
func AttemptHeal(node *Node, target string) ([]string, error) {
	node.emit(AttemptHealEvent{})
	node.logger.WithField("target", target).Info("attempt heal")

	// If join request succeeds a partition is detected,
	// this node will now coordinate the healing mechanism.
	joinRes, err := sendJoinRequest(node, target, time.Second)
	if err != nil {
		return nil, err
	}

	A := node.disseminator.MembershipAsChanges()
	B := joinRes.Membership

	// Get the nodes that aren't mergeable and need to reincarnate
	csForA, csForB := nodesThatNeedToReincarnate(A, B)

	// Reincarnate the nodes that need to reincarnate
	if len(csForA) != 0 || len(csForB) != 0 {
		err = reincarnateNodes(node, target, csForA, csForB)
		return aliveHosts(B), err
	}

	// Merge partitions if no node needs to reincarnate
	err = mergePartitions(node, target, B)
	return aliveHosts(B), err
}

// nodesThatNeedToReincarnate finds all nodes would become faulty when
// membership A gets merged with membership B. These need to reincarnate
// first, before we can heal the partition with a merge.
func nodesThatNeedToReincarnate(A, B []Change) (csForA, csForB []Change) {
	// Find changes that are alive for B and faulty for A and visa versa.
	for _, b := range B {
		a, ok := selectMember(A, b.Address)
		if !ok {
			continue
		}

		// faulty for partition A, alive for partition B
		// needs reincarnation in partition B.
		if a.Status == Faulty && b.Status == Alive && a.Incarnation >= b.Incarnation {
			csForB = append(csForB, Change{
				Address:     a.Address,
				Incarnation: a.Incarnation,
				Status:      Suspect,
			})
		}

		// alive for partition A, faulty for partition B
		// needs reincarnation in partition A.
		if a.Status == Alive && b.Status == Faulty && a.Incarnation <= b.Incarnation {
			csForA = append(csForA, Change{
				Address:     b.Address,
				Incarnation: b.Incarnation,
				Status:      Suspect,
			})
		}
	}

	return csForA, csForB
}

// reincarnateNodes applies csForA to this nodes membership, and sends a ping
// with csForB to B's membership, so that B will apply those changes in its
// ping handler.
func reincarnateNodes(node *Node, target string, csForA, csForB []Change) error {
	// reincarnate all nodes by disseminating that they are suspect
	node.healer.logger.WithField("target", target).Info("reincarnate nodes before we can merge the partitions")
	node.memberlist.Update(csForA)

	var err error
	if len(csForB) != 0 {
		_, err = sendPingWithChanges(node, target, csForB, time.Second)
	}

	return err
}

// mergePartitions applies the membership of B to a and send the membership
// A to B piggybacked on top of a ping.
func mergePartitions(node *Node, target string, B []Change) error {
	node.healer.logger.WithField("target", target).Info("merge two partitions")

	// Add membership of B to this node, so that the membership
	// information of B will be disseminated through A.
	node.memberlist.Update(B)

	// Send membership of A to the target node, so that the membership
	// information of partition A will be disseminated through B.
	A1 := node.disseminator.MembershipAsChanges()
	_, err := sendPingWithChanges(node, target, A1, time.Second)
	return err
}

// aliveHosts returns the address of those changes that have the Alive status.
func aliveHosts(cs []Change) (ret []string) {
	for _, b := range cs {
		if b.Status == Alive {
			ret = append(ret, b.Address)
		}
	}
	return
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
