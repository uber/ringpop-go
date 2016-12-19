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

// redBlackTree is an implemantation of a Red Black Tree
type redBlackTree struct {
	root *redBlackNode
	size int
}

type keytype interface {
	Compare(other interface{}) int
}

type valuetype interface{}

// redBlackNode is a node of the redBlackTree
type redBlackNode struct {
	key   keytype
	value valuetype
	left  *redBlackNode
	right *redBlackNode
	red   bool
}

// Size returns the number of nodes in the redBlackTree
func (t *redBlackTree) Size() int {
	return t.size
}

// Child returns the left or right node of the redBlackTree
func (n *redBlackNode) Child(right bool) *redBlackNode {
	if right {
		return n.right
	}
	return n.left
}

func (n *redBlackNode) setChild(right bool, node *redBlackNode) {
	if right {
		n.right = node
	} else {
		n.left = node
	}
}

// returns true if redBlackNode is red
func isRed(node *redBlackNode) bool {
	return node != nil && node.red
}

func singleRotate(oldroot *redBlackNode, dir bool) *redBlackNode {
	newroot := oldroot.Child(!dir)

	oldroot.setChild(!dir, newroot.Child(dir))
	newroot.setChild(dir, oldroot)

	oldroot.red = true
	newroot.red = false

	return newroot
}

func doubleRotate(root *redBlackNode, dir bool) *redBlackNode {
	root.setChild(!dir, singleRotate(root.Child(!dir), !dir))
	return singleRotate(root, dir)
}

// Insert inserts a value and string into the tree
// Returns true on succesful insertion, false if duplicate exists
func (t *redBlackTree) Insert(key keytype, value valuetype) (ret bool) {
	if t.root == nil {
		t.root = &redBlackNode{
			key:   key,
			value: value,
		}
		ret = true
	} else {
		var head = &redBlackNode{}

		var dir = true
		var last = true

		var parent *redBlackNode  // parent
		var gparent *redBlackNode // grandparent
		var ggparent = head       // great grandparent
		var node = t.root

		ggparent.right = t.root

		for {
			if node == nil {
				// insert new node at bottom
				node = &redBlackNode{
					key:   key,
					value: value,
					red:   true,
				}
				parent.setChild(dir, node)
				ret = true
			} else if isRed(node.left) && isRed(node.right) {
				// flip colors
				node.red = true
				node.left.red, node.right.red = false, false
			}
			// fix red violation
			if isRed(node) && isRed(parent) {
				dir2 := ggparent.right == gparent

				if node == parent.Child(last) {
					ggparent.setChild(dir2, singleRotate(gparent, !last))
				} else {
					ggparent.setChild(dir2, doubleRotate(gparent, !last))
				}
			}

			cmp := node.key.Compare(key)

			// stop if found
			if cmp == 0 {
				break
			}

			last = dir
			dir = cmp < 0

			// update helpers
			if gparent != nil {
				ggparent = gparent
			}
			gparent = parent
			parent = node

			node = node.Child(dir)
		}

		t.root = head.right
	}

	// make root black
	t.root.red = false

	if ret {
		t.size++
	}

	return ret
}

// Delete removes the entry for key from the redBlackTree. Returns true on
// succesful deletion, false if the key is not in tree
func (t *redBlackTree) Delete(key keytype) bool {
	if t.root == nil {
		return false
	}

	var head = &redBlackNode{red: true} // fake red node to push down
	var node = head
	var parent *redBlackNode  //parent
	var gparent *redBlackNode //grandparent
	var found *redBlackNode

	var dir = true

	node.right = t.root

	for node.Child(dir) != nil {
		last := dir

		// update helpers
		gparent = parent
		parent = node
		node = node.Child(dir)

		cmp := node.key.Compare(key)

		dir = cmp < 0

		// save node if found
		if cmp == 0 {
			found = node
		}

		// pretend to push red node down
		if !isRed(node) && !isRed(node.Child(dir)) {
			if isRed(node.Child(!dir)) {
				sr := singleRotate(node, dir)
				parent.setChild(last, sr)
				parent = sr
			} else {
				sibling := parent.Child(!last)
				if sibling != nil {
					if !isRed(sibling.Child(!last)) && !isRed(sibling.Child(last)) {
						// flip colors
						parent.red = false
						sibling.red, node.red = true, true
					} else {
						dir2 := gparent.right == parent

						if isRed(sibling.Child(last)) {
							gparent.setChild(dir2, doubleRotate(parent, last))
						} else if isRed(sibling.Child(!last)) {
							gparent.setChild(dir2, singleRotate(parent, last))
						}

						gpc := gparent.Child(dir2)
						gpc.red = true
						node.red = true
						gpc.left.red, gpc.right.red = false, false
					}
				}
			}
		}
	}

	// get rid of node if we've found one
	if found != nil {
		found.key = node.key
		found.value = node.value
		parent.setChild(parent.right == node, node.Child(node.left == nil))
		t.size--
	}

	t.root = head.right
	if t.root != nil {
		t.root.red = false
	}

	return found != nil
}

func (n *redBlackNode) search(key keytype) (valuetype, bool) {
	cmp := n.key.Compare(key)
	if cmp == 0 {
		return n.value, true
	} else if 0 < cmp {
		if n.left != nil {
			return n.left.search(key)
		}
	} else if n.right != nil {
		return n.right.search(key)
	}
	return nil, false
}

// traverseWhile traverses the nodes in the tree in-order invoking the `condition`-function argument for each node.
// If the condition-function returns `false` traversal is stopped and no more nodes will be
// visited. Returns `true` if all nodes are visited; `false` if not.
func (n *redBlackNode) traverseWhile(condition func(*redBlackNode) bool) bool {
	if n == nil {
		// the end of the tree does not signal the end of walking, but we can't
		// walk this node (nil) nor left or right anymore
		return true
	}

	// walk left first
	if !n.left.traverseWhile(condition) {
		// stop if walker indicated to break
		return false
	}
	// now visit this node
	if !condition(n) {
		// stop if walker indicated to break
		return false
	}
	// lastly visit right
	if !n.right.traverseWhile(condition) {
		// stop if walker indicated to break
		return false
	}

	// signal that we reached the end and walking should continue till we hit
	// end of tree
	return true
}

// Search searches for the entry for key in the redBlackTree, returns the value
// and true if found or nil and false if there is no entry for key in the tree.
func (t *redBlackTree) Search(key keytype) (valuetype, bool) {
	if t.root == nil {
		return nil, false
	}
	return t.root.search(key)
}

// LookupNUniqueAt iterates through the tree from the last node that is smaller
// than key or equal, and returns the next n unique values. This function is not
// guaranteed to return n values, less might be returned
func (t *redBlackTree) LookupNUniqueAt(n int, key keytype, result map[valuetype]struct{}) {
	findNUniqueAbove(t.root, n, key, result)
}

// findNUniqueAbove is a recursive search that finds n unique values with a key
// bigger or equal than key
func findNUniqueAbove(node *redBlackNode, n int, key keytype, result map[valuetype]struct{}) {
	if len(result) >= n || node == nil {
		return
	}

	// skip left branch when all its keys are smaller than key
	cmp := node.key.Compare(key)
	if cmp >= 0 {
		findNUniqueAbove(node.left, n, key, result)
	}

	// Make sure to stop when we have n unique values
	if len(result) >= n {
		return
	}

	if cmp >= 0 {
		result[node.value] = struct{}{}
	}

	findNUniqueAbove(node.right, n, key, result)
}
