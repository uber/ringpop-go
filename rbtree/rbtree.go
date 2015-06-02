package rbtree

// An RBTree is an automatically balancing tree of KVP pairs (presumably)
type RBTree struct {
	root *RingNode
	size int
}

type RingNode struct {
	val   int64
	str   string
	left  *RingNode
	right *RingNode
	red   bool
}

func (this *RBTree) Size() int {
	return this.size
}

func (this *RingNode) Child(right bool) *RingNode {
	if right {
		return this.right
	} else {
		return this.left
	}
}

func (this *RingNode) Str() string {
	return this.str
}

func (this *RingNode) setChild(right bool, n *RingNode) {
	if right {
		this.right = n
	} else {
		this.left = n
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
func (this *RBTree) Insert(val int64, str string) (ret bool) {
	if this.root == nil {
		this.root = &RingNode{val: val, str: str}
		ret = true
	} else {
		var head = &RingNode{}

		var dir = true
		var last = true

		var parent *RingNode  // parent
		var gparent *RingNode // grandparent
		var ggparent = head   // great grandparent
		var node = this.root

		ggparent.right = this.root

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

		this.root = head.right
	}

	// make root black
	this.root.red = false

	if ret {
		this.size++
	}

	return ret
}

// Delete removes a value from the RBTree
// Returns true on succesful deletion, false if val is not in tree
func (this *RBTree) Delete(val int64) bool {
	if this.root == nil {
		return false
	}

	var head = &RingNode{red: true} // fake red node to push down
	var node = head
	var parent *RingNode  //parent
	var gparent *RingNode //grandparent
	var found *RingNode

	var dir = true

	node.right = this.root

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
		this.size--
	}

	this.root = head.right
	if this.root != nil {
		this.root.red = false
	}

	return found != nil
}

func (this *RingNode) search(val int64) (string, bool) {
	if this.val == val {
		return this.str, true
	} else if val < this.val {
		if this.left != nil {
			return this.left.search(val)
		}
	} else if this.right != nil {
		return this.right.search(val)
	}
	return "", false
}

// Search searches for a value in the RBTree
// Returns the string and true if found, empty string and false if val is not
// in the tree
func (this *RBTree) Search(val int64) (string, bool) {
	if this.root == nil {
		return "", false
	} else {
		return this.root.search(val)
	}
}

// Min returns the node in the tree with the smallest value, or nil if the tree
// is empty
func (this *RBTree) Min() *RingNode {
	node := this.root

	if node == nil {
		return nil
	}

	for node.left != nil {
		node = node.left
	}

	return node
}

func (this *RingNode) genIter(val int64, ch chan<- *RingNode) {
	cmp := this.val >= val
	if cmp && this.left != nil {
		this.left.genIter(val, ch)
	}
	if cmp {
		ch <- this
	}
	if this.right != nil {
		this.right.genIter(val, ch)
	}
}

func (this *RBTree) MinIter() <-chan *RingNode {
	ch := make(chan *RingNode)
	if this.root != nil {
		min := this.Min()
		go func() {
			this.root.genIter(min.val, ch)
			close(ch)
		}()
	}
	return ch
}

// Iter returns a channel of RingNodes containing only those nodes in the tree
// with a value greater than or equal to the given val
func (this *RBTree) Iter(val int64) <-chan *RingNode {
	ch := make(chan *RingNode)
	if this.root != nil {
		go func() {
			this.root.genIter(val, ch)
			close(ch)
		}()
	}
	return ch
}
