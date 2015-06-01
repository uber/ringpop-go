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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIndexOf(t *testing.T) {
	nums := []string{"0", "1", "2"}

	assert.Equal(t, 0, IndexOf(nums, "0"), "expected 0 to be at index 0")
	assert.Equal(t, 1, IndexOf(nums, "1"), "expected 1 to be at index 1")
	assert.Equal(t, 2, IndexOf(nums, "2"), "expected 2 to be at index 2")
	assert.Equal(t, -1, IndexOf(nums, "3"), "expected 3 to not be in slice")
}

func TestTakeNode(t *testing.T) {
	nodes := []string{"0", "1", "2", "3", "4"}

	node := TakeNode(&nodes, 2)
	assert.Equal(t, "2", node, "expected to get 2 from slice of nodes")
	assert.Len(t, nodes, 4, "expected nodes to be mutated")
	assert.Equal(t, -1, IndexOf(nodes, "2"), "expected 2 to be removed from slice")

	node = TakeNode(&nodes, 0)
	assert.Equal(t, "0", node, "expected to get 0 from slice of nodes")
	assert.Len(t, nodes, 3, "expected nodes to be mutated")
	assert.Equal(t, -1, IndexOf(nodes, "0"), "expected 2 to be removed from slice")

	node = TakeNode(&nodes, 2)
	assert.Equal(t, "4", node, "expected to get 4 from slice of nodes")
	assert.Len(t, nodes, 2, "expected nodes to be mutated")
	assert.Equal(t, -1, IndexOf(nodes, "4"), "expected 4 to be removed from slice")

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
	badHostport := "127.0..1:3004"

	assert.Equal(t, "127.0.0.1", CaptureHost(hostport), "expected hostport to be captured")
	assert.Equal(t, "", CaptureHost(badHostport), "expected empty string as return value")
}

func TestUtilSelectOptInt(t *testing.T) {
	opt, zopt, def := 1, 0, 2

	assert.Equal(t, opt, SelectInt(opt, def), "expected to get option")
	assert.Equal(t, def, SelectInt(zopt, def), "expected to get default")
}

func TestUtilsSelectOptDuration(t *testing.T) {
	opt, zopt, def := time.Duration(1), time.Duration(0), time.Duration(2)

	assert.Equal(t, opt, SelectDuration(opt, def), "expected to get option")
	assert.Equal(t, def, SelectDuration(zopt, def), "expected to get default")
}
