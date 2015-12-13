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

package ring_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func iterTree() *RBTree {
	tree := new(RBTree)

	tree.Insert(40, "fourty")
	tree.Insert(20, "twenty")
	tree.Insert(60, "sixty")
	tree.Insert(10, "ten")
	tree.Insert(30, "thirty")
	tree.Insert(50, "fifty")
	tree.Insert(70, "seventy")

	return tree
}

func TestIterNilTree(t *testing.T) {
	iter := NewRBIter(nil)
	assert.Nil(t, iter, "expected iter to be nil")

	iter = NewRBIterAt(nil, 5)
	assert.Nil(t, iter, "expected iter to be nil")
}

func TestIterEmptyTree(t *testing.T) {
	tree := new(RBTree)

	iter := tree.Iter()
	assert.Nil(t, iter.current, "expected current node to be nil")
	iter.Next()
	assert.Nil(t, iter.current, "expected current node to be nil")

	iter = tree.IterAt(5)
	assert.Nil(t, iter.current, "expected current node to be nil")
	iter.Next()
	assert.Nil(t, iter.current, "expected current node to be nil")
}

func TestIterOverAllNodes(t *testing.T) {
	tree := iterTree()

	iter := tree.Iter()
	assert.Equal(t, 10, iter.Val(), "expected iter to be at ten")
	assert.Equal(t, "ten", iter.Str(), "expected iter to be at ten")

	iter.Next()
	assert.Equal(t, 20, iter.Val(), "expected iter to be at twenty")
	assert.Equal(t, "twenty", iter.Str(), "expected iter to be at twenty")

	iter.Next()
	assert.Equal(t, 30, iter.Val(), "expected iter to be at thirty")
	assert.Equal(t, "thirty", iter.Str(), "expected iter to be at thirty")

	iter.Next()
	assert.Equal(t, 40, iter.Val(), "expected iter to be at fourty")
	assert.Equal(t, "fourty", iter.Str(), "expected iter to be at fourty")

	iter.Next()
	assert.Equal(t, 50, iter.Val(), "expected iter to be at fifty")

	iter.Next()
	assert.Equal(t, 60, iter.Val(), "expected iter to be at sixty")
	assert.Equal(t, "sixty", iter.Str(), "expected iter to be at sixty")

	iter.Next()
	assert.Equal(t, 70, iter.Val(), "expected iter to be at seventy")
	assert.Equal(t, "seventy", iter.Str(), "expected iter to be at seventy")

	iter.Next()
	assert.Equal(t, 10, iter.Val(), "expected iter to be at ten")
	assert.Equal(t, "ten", iter.Str(), "expected iter to be at ten")
}

func TestIterAt(t *testing.T) {
	tree := iterTree()

	iter := tree.IterAt(10)
	assert.Equal(t, 10, iter.Val(), "expected iter to be at ten")
	assert.Equal(t, "ten", iter.Str(), "expected iter to be at ten")

	iter = tree.IterAt(5)
	assert.Equal(t, 10, iter.Val(), "expected iter to be at ten")
	assert.Equal(t, "ten", iter.Str(), "expected iter to be at ten")

	iter = tree.IterAt(39)
	assert.Equal(t, 40, iter.Val(), "expected iter to be at fourty")
	assert.Equal(t, "fourty", iter.Str(), "expected iter to be at fourty")

	iter = tree.IterAt(55)
	assert.Equal(t, 60, iter.Val(), "expected iter to be at sixty")
	assert.Equal(t, "sixty", iter.Str(), "expected iter to be at sixty")

	iter = tree.IterAt(61)
	assert.Equal(t, 70, iter.Val(), "expected iter to be at seventy")
	assert.Equal(t, "seventy", iter.Str(), "expected iter to be at seventy")

	iter = tree.IterAt(80)
	assert.Equal(t, 10, iter.Val(), "expected iter to be at ten")
	assert.Equal(t, "ten", iter.Str(), "expected iter to be at ten")
}
