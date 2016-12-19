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

	"github.com/dgryski/go-farm"
	"github.com/stretchr/testify/assert"
)

// addressChecksum implements the now obsolete checksum method that didn't support identities.
// It's moved to this test file to test backwards compatibility of the identityChecksummer
type addressChecksummer struct{}

func (i *addressChecksummer) Checksum(ring *HashRing) uint32 {
	addresses := ring.copyServersNoLock()
	sort.Strings(addresses)
	bytes := []byte(strings.Join(addresses, ";"))
	return farm.Fingerprint32(bytes)
}

func TestAddressChecksum_Compute(t *testing.T) {
	members := genMembers(1, 1, 10, false)
	ring := New(farm.Fingerprint32, 1)
	ring.AddMembers(members...)
	checksum := &addressChecksummer{}

	addresses := make([]string, 0, 10)
	for _, members := range members {
		addresses = append(addresses, members.GetAddress())
	}

	sort.Strings(addresses)
	bytes := []byte(strings.Join(addresses, ";"))

	expected := farm.Fingerprint32(bytes)
	actual := checksum.Checksum(ring)

	assert.Equal(t, expected, actual)
}

func TestIdentityChecksum_Compute(t *testing.T) {
	identityChecksummer := &identityChecksummer{}

	ringWithoutIdentities := New(farm.Fingerprint32, 1)
	ringWithoutIdentities.AddMembers(genMembers(1, 1, 10, false)...)

	legacyChecksum := (&addressChecksummer{}).Checksum(ringWithoutIdentities)
	identityChecksum := identityChecksummer.Checksum(ringWithoutIdentities)

	assert.Equal(t, legacyChecksum, identityChecksum, "Identity checksum should be the same as legacy on ring without identities")

	ringWithIdentities := New(farm.Fingerprint32, 1)
	ringWithIdentities.AddMembers(genMembers(1, 1, 10, true)...)

	identityChecksum = identityChecksummer.Checksum(ringWithIdentities)

	assert.NotEqual(t, legacyChecksum, identityChecksum, "IdentityChecksummer should not match legacy checksummer on ring with identites ")
}

func TestReplicaPointChecksum_Compute(t *testing.T) {
	replicaPointChecksummer := &replicaPointChecksummer{}
	members := genMembers(1, 1, 10, false)

	ring1ReplicaPoint := New(farm.Fingerprint32, 1)
	ring1ReplicaPoint.AddMembers(members...)

	ring2ReplicaPoints := New(farm.Fingerprint32, 2)
	ring2ReplicaPoints.AddMembers(members...)

	checksum1ReplicaPoint := replicaPointChecksummer.Checksum(ring1ReplicaPoint)
	checksum2ReplicaPoints := replicaPointChecksummer.Checksum(ring2ReplicaPoints)

	assert.NotEqual(t, checksum1ReplicaPoint, checksum2ReplicaPoints, "Checksum should not match with different replica point counts")
}
