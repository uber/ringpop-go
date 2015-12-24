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

// Iterator is used to perform an in-order traversal of a Red-Black Tree
type Iterator struct {
	itFunc iteratorFunc
}

func NewIterator(tree *RBTree) Iterator {
	return Iterator{iterate(tree.root)}
}

func NewIteratorAt(tree *RBTree, x int) Iterator {
	return Iterator{iterateAt(tree.root, x)}
}

// Next returns the next value of the iteration. Yielding done indicates that
// the iteration is finished.
func (i *Iterator) Next() (n *Node) {
	if i.itFunc == nil {
		return nil
	}
	n, i.itFunc = i.itFunc()
	return n
}

// Functional style iterators. Calling the iteratorFunc yields the next
// value of the iterator and an iteratorFunc for the remaining values.
type iteratorFunc func() (*Node, iteratorFunc)

// Creates an iteratorFunc that iterates over the rb-tree in an in-order fashion
func iterate(n *Node) iteratorFunc {
	if n == nil {
		return nil
	}
	leftIt := iterate(n.left)
	rightIt := iterate(n.right)
	return compose(leftIt, prepend(n, rightIt))
}

func iterateAt(n *Node, x int) iteratorFunc {
	if n == nil {
		return nil
	}

	rightIt := iterate(n.right)
	if x == n.val {
		return prepend(n, rightIt)
	} else if x > n.val {
		return iterateAt(n.right, x)
	} else {
		return compose(iterateAt(n.left, x), prepend(n, rightIt))
	}
}

// Composes two IteratorFuncs into one
func compose(i1, i2 iteratorFunc) iteratorFunc {
	if i1 == nil {
		return i2
	}
	v, tail := i1()
	return prepend(v, compose(tail, i2))
}

// Prepends a value to an iteratorFunc yielding the new InteratorFunc
func prepend(n *Node, i iteratorFunc) iteratorFunc {
	return func() (*Node, iteratorFunc) {
		return n, i
	}
}
