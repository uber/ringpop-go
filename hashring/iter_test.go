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
	"testing"

	"github.com/stretchr/testify/assert"
)

func iterTree() *RedBlackTree {
	tree := &RedBlackTree{}

	tree.Insert(40, "fourty")
	tree.Insert(20, "twenty")
	tree.Insert(60, "sixty")
	tree.Insert(10, "ten")
	tree.Insert(30, "thirty")
	tree.Insert(50, "fifty")
	tree.Insert(70, "seventy")

	return tree
}

func TestIterNil(t *testing.T) {
	// empty tree
	tree := &RedBlackTree{}
	iter := NewIterator(tree)

	n := iter.Next()
	assert.Nil(t, n, "expected iter to be nil")
}

func TestIterOverAllNodes(t *testing.T) {
	strs := []string{"ten", "twenty", "thirty", "fourty", "fifty", "sixty", "seventy"}
	tree := iterTree()
	iter := NewIterator(tree)

	for i := 0; i < 7; i++ {
		n := iter.Next()
		assert.Equal(t, (i+1)*10, n.Val(), "expected iter to be at "+strs[i])
		assert.Equal(t, strs[i], n.Str(), "expected iter to be at "+strs[i])
	}

	n := iter.Next()
	assert.Nil(t, n, "expected to be done")
}

func TestIterAt(t *testing.T) {
	cases := []struct {
		at, val int
		str     string
	}{
		{5, 10, "ten"},
		{10, 10, "ten"},
		{39, 40, "fourty"},
		{55, 60, "sixty"},
		{70, 70, "seventy"},
	}

	tree := iterTree()
	for _, c := range cases {
		iter := NewIteratorAt(tree, c.at)
		n := iter.Next()
		assert.Equal(t, c.val, n.Val(), "expected iter to be at "+c.str)
		assert.Equal(t, c.str, n.Str(), "expected iter to be at "+c.str)
	}

	iter := NewIteratorAt(tree, 71)
	n := iter.Next()
	assert.Nil(t, n, "expected iter to be done")
}
