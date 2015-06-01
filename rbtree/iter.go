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

// Package rbtree provides an implementation of a Red Black Tree.
package rbtree

//Iter returns an iterator starting at the leftmost node in the tree
func (t *RBTree) Iter() *RBIter {
	return NewRBIter(t)
}

// IterAt returns an iter set at, or immediately after, val
func (t *RBTree) IterAt(val int) *RBIter {
	return NewRBIterAt(t, val)
}

// An RBIter iterates over nodes in an RBTree
type RBIter struct {
	tree      *RBTree
	current   *RBNode
	ancestors []*RBNode
}

// NewRBIter returns an RBIter for the given tree, or nil if the given tree is nil
func NewRBIter(tree *RBTree) *RBIter {
	if tree == nil {
		return nil
	}

	iter := &RBIter{
		tree:    tree,
		current: nil,
	}
	iter.minNode(tree.root)

	return iter
}

// NewRBIterAt returns an RBIter for the given tree at or immediately after the given
// val, or nil if the tree, or the tree's root, is nil
func NewRBIterAt(tree *RBTree, val int) *RBIter {
	if tree == nil {
		return nil
	}

	iter := &RBIter{
		tree:    tree,
		current: nil,
	}

	if tree.size == 0 {
		return iter
	}

	iter.current = tree.root
	for iter.current != nil {
		if iter.current.val == val {
			return iter
		}

		iter.pushA(iter.current) // add ancestor
		iter.current = iter.current.Child(val > iter.current.val)
	}

	// iter current is nil, thus we did not find an equal value, so go back up
	// ancestors and find the next greatest
	for iter.current != tree.root {
		iter.current = iter.popA()
		if iter.current.val > val {
			return iter
		}
	}

	// val is greater than max in tree, go to min node
	iter.minNode(tree.root)

	return iter
}

// Nil returns true if the current node is nil
func (i *RBIter) Nil() bool {
	return i.current == nil
}

// Val returns the val contained at the current node
func (i *RBIter) Val() int {
	return i.current.Val()
}

// Str returns the str contained at the current node
func (i *RBIter) Str() string {
	return i.current.Str()
}

// Next returns the next node in the tree
func (i *RBIter) Next() *RBNode {
	if i.current == nil {
		i.minNode(i.tree.root)
	} else {
		if i.current.right == nil {
			for {
				save := i.current
				i.current = i.popA()
				if i.current == nil {
					i.minNode(i.tree.root)
					break
				}

				if i.current.right != save {
					break
				}
			}

		} else {
			i.pushA(i.current)
			i.minNode(i.current.right)
		}

	}

	return i.current
}

// sets current to left most node in subtree of root
func (i *RBIter) minNode(root *RBNode) {
	if root == nil {
		i.current = root
		return
	}
	for root.left != nil {
		i.pushA(root)
		root = root.left
	}
	i.current = root
}

func (i *RBIter) pushA(node *RBNode) {
	i.ancestors = append(i.ancestors, node)
}

func (i *RBIter) popA() *RBNode {
	ind := len(i.ancestors) - 1
	if ind < 0 {
		return nil
	}
	a := i.ancestors[ind]
	i.ancestors = i.ancestors[:ind]
	return a
}
