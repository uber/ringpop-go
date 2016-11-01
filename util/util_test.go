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

package util

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// indexOf returns the index of element in slice, or -1 if the element is not in slice
func indexOf(slice []string, element string) int {
	for i, e := range slice {
		if e == element {
			return i
		}
	}

	return -1
}

func TestStringInSlice(t *testing.T) {
	slice := []string{"a", "b", "c"}

	assert.True(t, StringInSlice(slice, "a"))
	assert.True(t, StringInSlice(slice, "b"))
	assert.True(t, StringInSlice(slice, "c"))
	assert.False(t, StringInSlice(slice, "d"))
}

func TestTakeNode(t *testing.T) {
	nodes := []string{"0", "1", "2", "3", "4"}

	node := TakeNode(&nodes, 2)
	assert.Equal(t, "2", node, "expected to get 2 from slice of nodes")
	assert.Len(t, nodes, 4, "expected nodes to be mutated")
	assert.Equal(t, -1, indexOf(nodes, "2"), "expected 2 to be removed from slice")

	node = TakeNode(&nodes, 0)
	assert.Equal(t, "0", node, "expected to get 0 from slice of nodes")
	assert.Len(t, nodes, 3, "expected nodes to be mutated")
	assert.Equal(t, -1, indexOf(nodes, "0"), "expected 2 to be removed from slice")

	node = TakeNode(&nodes, 2)
	assert.Equal(t, "4", node, "expected to get 4 from slice of nodes")
	assert.Len(t, nodes, 2, "expected nodes to be mutated")
	assert.Equal(t, -1, indexOf(nodes, "4"), "expected 4 to be removed from slice")

	node = TakeNode(&nodes, -1)
	assert.Len(t, nodes, 1, "expected nodes to be mutated, random node taken")

	node = TakeNode(&nodes, 100)
	assert.Equal(t, "", node, "expected to get empty string for bad index")
	assert.Len(t, nodes, 1, "expected nodes to stay the same")

	node = TakeNode(&nodes, -1)
	assert.Len(t, nodes, 0, "expecte nodes to be empty")

	node = TakeNode(&nodes, 0)
	assert.Equal(t, "", node, "expected to get back empty string from empty slice")
	assert.Len(t, nodes, 0, "expecte nodes to be empty")
}

func TestCaptureHost(t *testing.T) {
	hostport := "127.0.0.1:3001"
	badHostport := "127.0.0.1::3004"

	assert.Equal(t, "127.0.0.1", CaptureHost(hostport), "expected hostport to be captured")
	assert.Equal(t, "", CaptureHost(badHostport), "expected empty string as return value")
}

func TestUtilSelectOptInt(t *testing.T) {
	opt, zopt, def := 1, 0, 2

	assert.Equal(t, opt, SelectInt(opt, def), "expected to get option")
	assert.Equal(t, def, SelectInt(zopt, def), "expected to get default")
}

func TestUtilSelectOptFloat(t *testing.T) {
	opt, zopt, def := 1.0, 0.0, 2.0

	assert.Equal(t, opt, SelectFloat(opt, def), "expected to get option")
	assert.Equal(t, def, SelectFloat(zopt, def), "expected to get default")
}

func TestUtilsSelectOptDuration(t *testing.T) {
	opt, zopt, def := time.Duration(1), time.Duration(0), time.Duration(2)

	assert.Equal(t, opt, SelectDuration(opt, def), "expected to get option")
	assert.Equal(t, def, SelectDuration(zopt, def), "expected to get default")
}

func TestUtilsSelectBool(t *testing.T) {
	var tableTests = []struct {
		opt    bool
		def    bool
		result bool
	}{
		{false, false, false},
		{false, true, true},
		{true, false, true},
		{true, true, true},
	}

	for _, test := range tableTests {
		assert.Equal(t, test.result, SelectBool(test.opt, test.def), "inputs opt: %t def: %t", test.opt, test.def)
	}
}

func TestNoHostnameMismatch(t *testing.T) {
	mismatches, err := CheckHostnameIPMismatch("192.0.2.1:1", map[string][]string{
		"192.0.2.1": []string{
			"192.0.2.1:1",
		},
	})
	assert.NoError(t, err)
	assert.Nil(t, mismatches)
}

func TestLocalHostIPInconsistency(t *testing.T) {
	mismatches, err := CheckHostnameIPMismatch("192.0.2.1:1", map[string][]string{
		"192.0.2.1": []string{
			"192.0.2.1:1",
		},
		"foo.local": []string{
			"foo.local:1",
			"foo.local:2",
		},
	})
	assert.EqualError(t, err, "Your host identifier looks like an IP address and there are bootstrap hosts that appear to be specified with hostnames. These inconsistencies may lead to subtle node communication issues.")
	assert.Equal(t, []string{"foo.local:1", "foo.local:2"}, mismatches)
}

func TestBootstrapHostIPInconsistency(t *testing.T) {
	mismatches, err := CheckHostnameIPMismatch("foo.local:1", map[string][]string{
		"foo.local": []string{
			"foo.local:1",
			"foo.local:2",
		},
		"192.0.2.1": []string{
			"192.0.2.1:1",
			"192.0.2.1:2",
		},
	})
	assert.EqualError(t, err, "Your host identifier looks like a hostname and there are bootstrap hosts that appear to be specified with IP addresses. These inconsistencies may lead to subtle node communication issues")
	assert.Equal(t, []string{"192.0.2.1:1", "192.0.2.1:2"}, mismatches)
}

func TestHostPortsByHost(t *testing.T) {
	assert.Equal(t, map[string][]string{
		"192.0.2.1": []string{
			"192.0.2.1:1",
		},
		"192.0.2.2": []string{
			"192.0.2.2:1",
			"192.0.2.2:2",
		},
		"192.0.2.3": []string{
			"192.0.2.3:1",
			"192.0.2.3:1",
		},
		"foo": []string{
			"foo:1",
			"foo:2",
		},
		"foo.bar.local": []string{
			"foo.bar.local:1",
			"foo.bar.local:2",
		},
	}, HostPortsByHost([]string{
		"192.0.2.1:1",
		"192.0.2.2:1",
		"192.0.2.2:2",
		"192.0.2.3:1",
		"192.0.2.3:1",
		"foo:1",
		"foo:2",
		"foo.bar.local:1",
		"foo.bar.local:2",
	}))
}

func TestLocalMissing(t *testing.T) {
	err := CheckLocalMissing("foo.local:1", []string{
		"192.0.2.1:1",
	})
	assert.Error(t, err)
}

func TestLocalNotMissing(t *testing.T) {
	err := CheckLocalMissing("foo.local:1", []string{
		"192.0.2.1:1",
		"foo.local:1",
	})
	assert.NoError(t, err)

}

func TestSingleNodeCluster(t *testing.T) {
	assert.True(t, SingleNodeCluster("foo.local:1", map[string][]string{
		"foo.local": []string{
			"foo.local:1",
		},
	}))
	assert.False(t, SingleNodeCluster("foo.local:1", map[string][]string{
		"foo.local": []string{
			"foo.local:2",
		},
	}))
	assert.False(t, SingleNodeCluster("foo.local:1", map[string][]string{
		"192.0.2.1": []string{
			"192.0.2.1:1",
		},
	}))
	assert.False(t, SingleNodeCluster("foo.local:1", map[string][]string{
		"foo.local": []string{
			"foo.local:1",
			"foo.local:2",
		},
	}))
	assert.False(t, SingleNodeCluster("foo.local:1", map[string][]string{
		"foo.local": []string{
			"foo.local:2",
		},
		"192.0.2.1": []string{
			"192.0.2.1:1",
		},
	}))
}

func TestMin(t *testing.T) {
	var minTableTest = []struct {
		numbers []int
		min     int
	}{
		// tests for two numbers
		{[]int{1, 2}, 1},
		{[]int{10, 2}, 2},
		{[]int{3, 3}, 3},

		// tests for multiple numbers
		{[]int{1, 2, 3}, 1},
		{[]int{1, 3, 2}, 1},
		{[]int{2, 1, 3}, 1},
		{[]int{2, 3, 1}, 1},
		{[]int{3, 1, 2}, 1},
		{[]int{3, 2, 1}, 1},
	}

	for _, test := range minTableTest {
		assert.Equal(t, test.min, Min(test.numbers[0], test.numbers[1:]...), "expected %d to be the min of %v", test.min, test.numbers)
	}
}

func TestTimeZero(t *testing.T) {
	assert.True(t, TimeZero().IsZero())
}

func TestShuffleStringsInPlace(t *testing.T) {
	strs := make([]string, 1000)
	strs2 := make([]string, 1000)
	for i := range strs {
		strs[i] = fmt.Sprint(i)
		strs2[i] = strs[i]
	}

	ShuffleStringsInPlace(strs)

	collisions := 0
	for i := range strs {
		if strs[i] == strs2[i] {
			collisions++
		}
	}

	// expected probability of 1/1000 for every index so the expected number of
	// collisions is 1. We add some slack and expect smaller or equal than 3.
	assert.True(t, collisions <= 3, "expected that array is shuffled")

	sort.Strings(strs)
	sort.Strings(strs2)
	assert.Equal(t, len(strs), len(strs2), "expected size of slice did not change")
	for i := range strs {
		if strs[i] != strs2[i] {
			assert.Fail(t, "expected contents of slice did not change")
			break
		}
	}
}
