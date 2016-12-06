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

package hashring

import (
	"sort"
	"strings"
	"testing"

	"code.uber.internal/rt/cag.git/Godeps/_workspace/src/code.uber.internal/go-common.git/Godeps/_workspace/src/github.com/dgryski/go-farm"
	"github.com/stretchr/testify/assert"
)

func TestAddressChecksum_Compute(t *testing.T) {
	members := genMembers(1, 1, 10, false)
	ring := New(farm.Fingerprint32, 1)
	ring.AddMembers(members...)
	checksum := &addressChecksum{}

	addresses := make([]string, 0, 10)
	for _, members := range members {
		addresses = append(addresses, members.GetAddress())
	}

	sort.Strings(addresses)
	bytes := []byte(strings.Join(addresses, ";"))

	expected := farm.Fingerprint32(bytes)
	actual := checksum.Compute(ring)

	assert.Equal(t, expected, actual)
}

func TestIdentityChecksum_Compute(t *testing.T) {
	identityChecksummer := &identityChecksum{}

	ringWithoutIdentities := New(farm.Fingerprint32, 1)
	ringWithoutIdentities.AddMembers(genMembers(1, 1, 10, false)...)

	legacyChecksum := (&addressChecksum{}).Compute(ringWithoutIdentities)
	identityChecksum := identityChecksummer.Compute(ringWithoutIdentities)

	assert.Equal(t, legacyChecksum, identityChecksum, "Identity checksum should be the same as legacy on ring without identities")

	ringWithIdentities := New(farm.Fingerprint32, 1)
	ringWithIdentities.AddMembers(genMembers(1, 1, 10, true)...)

	identityChecksum = identityChecksummer.Compute(ringWithIdentities)

	assert.NotEqual(t, legacyChecksum, identityChecksum, "IdentityChecksummer should not match legacy checksummer on ring with identites ")
}

func TestReplicaPointChecksum_Compute(t *testing.T) {
	replicaPointChecksummer := &replicaPointChecksum{}
	members := genMembers(1, 1, 10, false)

	ring1ReplicaPoint := New(farm.Fingerprint32, 1)
	ring1ReplicaPoint.AddMembers(members...)

	ring2ReplicaPoints := New(farm.Fingerprint32, 2)
	ring2ReplicaPoints.AddMembers(members...)

	checksum1ReplicaPoint := replicaPointChecksummer.Compute(ring1ReplicaPoint)
	checksum2ReplicaPoints := replicaPointChecksummer.Compute(ring2ReplicaPoints)

	assert.NotEqual(t, checksum1ReplicaPoint, checksum2ReplicaPoints, "Checksum should not match with different replica point counts")
}
