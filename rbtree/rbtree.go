package rbtree

// An RBTree is an automatically balancing tree of KVP pairs (presumably)
type RBTree struct {
	root *RingNode
	size int
}

type RingNode struct {
	val   int
	str   string
	left  *RingNode
	right *RingNode
	red   bool
}

func (t *RBTree) Size() int {
	return t.size
}

func (n *RingNode) Child(right bool) *RingNode {
	if right {
		return n.right
	} else {
		return n.left
	}
}

func (n *RingNode) Str() string {
	return n.str
}

func (n *RingNode) setChild(right bool, node *RingNode) {
	if right {
		n.right = node
	} else {
		n.left = node
	}
}

// returns true if RingNode is red
func isRed(node *RingNode) bool {
	return node != nil && node.red
}

func singleRotate(oldroot *RingNode, dir bool) *RingNode {
	newroot := oldroot.Child(!dir)

	oldroot.setChild(!dir, newroot.Child(dir))
	newroot.setChild(dir, oldroot)

	oldroot.red = true
	newroot.red = false

	return newroot
}

func doubleRotate(root *RingNode, dir bool) *RingNode {
	root.setChild(!dir, singleRotate(root.Child(!dir), !dir))
	return singleRotate(root, dir)
}

// Insert inserts a value and string into the tree
// Returns true on succesful insertion, false if duplicate exists
func (t *RBTree) Insert(val int, str string) (ret bool) {
	if t.root == nil {
		t.root = &RingNode{val: val, str: str}
		ret = true
	} else {
		var head = &RingNode{}

		var dir = true
		var last = true

		var parent *RingNode  // parent
		var gparent *RingNode // grandparent
		var ggparent = head   // great grandparent
		var node = t.root

		ggparent.right = t.root

		for {
			if node == nil {
				// insert new node at bottom
				node = &RingNode{val: val, str: str, red: true}
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

// Delete removes a value from the RBTree
// Returns true on succesful deletion, false if val is not in tree
func (t *RBTree) Delete(val int) bool {
	if t.root == nil {
		return false
	}

	var head = &RingNode{red: true} // fake red node to push down
	var node = head
	var parent *RingNode  //parent
	var gparent *RingNode //grandparent
	var found *RingNode

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

func (n *RingNode) search(val int) (string, bool) {
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

// Search searches for a value in the RBTree
// Returns the string and true if found, empty string and false if val is not
// in the tree
func (t *RBTree) Search(val int) (string, bool) {
	if t.root == nil {
		return "", false
	} else {
		return t.root.search(val)
	}
}

// Min returns the node in the tree with the smallest value, or nil if the tree
// is empty
func (t *RBTree) Min() *RingNode {
	node := t.root

	if node == nil {
		return nil
	}

	for node.left != nil {
		node = node.left
	}

	return node
}

func (n *RingNode) genIter(val int, ch chan<- *RingNode) {
	cmp := n.val >= val
	if cmp && n.left != nil {
		n.left.genIter(val, ch)
	}
	if cmp {
		ch <- n
	}
	if n.right != nil {
		n.right.genIter(val, ch)
	}
}

func (t *RBTree) MinIter() <-chan *RingNode {
	ch := make(chan *RingNode)
	if t.root != nil {
		min := t.Min()
		go func() {
			t.root.genIter(min.val, ch)
			close(ch)
		}()
	}
	return ch
}

// Iter returns a channel of RingNodes containing only those nodes in the tree
// with a value greater than or equal to the given val
func (t *RBTree) Iter(val int) <-chan *RingNode {
	ch := make(chan *RingNode)
	if t.root != nil {
		go func() {
			t.root.genIter(val, ch)
			close(ch)
		}()
	}
	return ch
}
