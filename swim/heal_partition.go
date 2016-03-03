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
// separated from the target.
//
// The algorithm to heal the partition works as follows:
// The goal is to first let every node reincarnate, we do this by declaring nodes
// suspect. For members that are faulty in partition B, we fabricate updates with
// a suspect in stead of faulty status and disseminate them through A. We also do
// the reverse. We then create a connection from partition A to B and visa versa.
// We do this by reincarnating this node and sending the respective update to the
// target. The target has also been reincarnated due to previous actions, this
// update is now applied on this node's membership to create the connection from
// partition B to partition A.
//
// The heal alogrithm in more detail:
// 1. send join request to the target, this tells us whether a partition exists
// 2. collect changes for partition B:
//    - collect all faulty members of this membership,
//      now change the status of the updates from faulty to suspect;
//    - reincarnate yourself and collect the respective change
// 3. send the changes to the target, the changes will be disseminated
//    through B automatically
// 4. collect changes for partition A:
//    - collect all faulty members from the join response of step 1,
//      now change the statues of the update from faulty to suspect;
//    - t has reincarnated itself upon receiving the ping from step 3,
//      find that update from the ping response. If t has reincarnated
//      for another reason the change might not be present. In this case
//      we search for the change in the join response of step 1.
// 5. apply the changes for partition A to the nodes membership,
//    the changes will be disseminated through A automatically
func AttemptHeal(node *Node, target string) error {
	// If join request succeeds a partition is detected,
	// this node will now coordinate the healing mechanism.
	joinRes, err := sendJoinRequest(node, target, time.Second/2)
	if err != nil {
		return err
	}

	// Create changes for partition B. Partition B is the partition the targets node is part of.
	csForB := node.disseminator.FullSync()
	// discard changes that are not faulty and change faulty to suspect
	for i := 0; i < len(csForB); i++ {
		// TODO (wieger): make filtering more precice
		if csForB[i].Status != Faulty {
			csForB[i] = csForB[len(csForB)-1]
			csForB = csForB[:len(csForB)-1]
			i--
		} else {
			csForB[i].Status = Suspect
		}
	}

	// Bump inc no of this node and add that change for B.
	bumpIncNoChange := node.memberlist.Reincarnate()
	csForB = append(csForB, bumpIncNoChange...)

	// Ping b with the changes that are just assembled.
	res, err := sendPingWithChanges(node, target, csForB, time.Second)
	if err != nil {
		return err
	}

	// Create changes for partition A. Partition A is the partition this node is part of.
	csForA := joinRes.Membership
	// discard changes that are not faulty and change faulty to suspect
	for i := 0; i < len(csForA); i++ {
		if csForA[i].Status != Faulty {
			csForA[i] = csForA[len(csForA)-1]
			csForA = csForA[:len(csForA)-1]
			i--
		} else {
			csForA[i].Status = Suspect
		}
	}

	// target has reasserted that it is alive. We select this change so that A now has a bridge to B.
	targetFound := false
	for _, c := range res.Changes {
		if c.Address == target {
			csForA = append(csForA, c)
			targetFound = true
		}
	}

	// There is a chance that the target was already reincarnated due to a
	// separate event. In this case we can find the status of the target node
	// in the join response we received earlier.
	for _, c := range joinRes.Membership {
		if targetFound {
			break
		}
		if c.Address == target {
			csForA = append(csForA, c)
			targetFound = true
		}
	}

	// Add changes to membership of this node so that they will be dissemiated through partition A.
	node.memberlist.Update(csForA)

	return nil
}
