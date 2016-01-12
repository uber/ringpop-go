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

// redBlackNode is a node of the redBlackTree
type redBlackNode struct {
	val   int
	str   string
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
func (t *redBlackTree) Insert(val int, str string) (ret bool) {
	if t.root == nil {
		t.root = &redBlackNode{val: val, str: str}
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
				node = &redBlackNode{val: val, str: str, red: true}
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

			cmp := node.val - val

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

// Delete removes a value from the redBlackTree
// Returns true on succesful deletion, false if val is not in tree
func (t *redBlackTree) Delete(val int) bool {
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

		cmp := node.val - val

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
		found.val = node.val
		found.str = node.str
		parent.setChild(parent.right == node, node.Child(node.left == nil))
		t.size--
	}

	t.root = head.right
	if t.root != nil {
		t.root.red = false
	}

	return found != nil
}

func (n *redBlackNode) search(val int) (string, bool) {
	if n.val == val {
		return n.str, true
	} else if val < n.val {
		if n.left != nil {
			return n.left.search(val)
		}
	} else if n.right != nil {
		return n.right.search(val)
	}
	return "", false
}

// Search searches for a value in the redBlackTree, returns the string and true
// if found or the empty string and false if val is not in the tree.
func (t *redBlackTree) Search(val int) (string, bool) {
	if t.root == nil {
		return "", false
	}
	return t.root.search(val)
}

// LookupNAt iterates through the tree from the node with value val, and
// returns the next n unique strings. This function is not guaranteed to
// return n strings.
func (t *redBlackTree) LookupNUniqueAt(n int, val int, result map[string]struct{}) {
	findNUniqueAbove(t.root, n, val, result)
}

// findNUniqueAbove is a recursive search that finds n unique strings
// with a value bigger or equal than val
func findNUniqueAbove(node *redBlackNode, n int, val int, result map[string]struct{}) {
	if len(result) >= n || node == nil {
		return
	}

	// skip left branch when all its values are smaller than val
	if node.val >= val {
		findNUniqueAbove(node.left, n, val, result)
	}

	// Make sure to stop when we have n unique strings
	if len(result) >= n {
		return
	}

	if node.val >= val {
		result[node.str] = struct{}{}
	}

	findNUniqueAbove(node.right, n, val, result)
}
