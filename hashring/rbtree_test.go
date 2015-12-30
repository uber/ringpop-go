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
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func makeTree() RBTree {
	tree := RBTree{}

	tree.Insert(1, "one")
	tree.Insert(2, "two")
	tree.Insert(3, "three")
	tree.Insert(4, "four")
	tree.Insert(5, "five")
	tree.Insert(6, "six")
	tree.Insert(7, "seven")
	tree.Insert(8, "eight")

	//               4,B
	//             /     \
	//         2,R         6,R
	//       /     \     /     \
	//     1,B    3,B   5,B    7,B
	//                             \
	//                              8,R

	return tree
}

func TestEmptyTree(t *testing.T) {
	tree := RBTree{}

	assert.Nil(t, tree.root, "tree root is nil")
	assert.Equal(t, 0, tree.Size(), "tree has 0 nodes")
}

// counts the black height of the tree and validates it along the way
func validateRBTree(node *RBNode) (int, error) {
	if node == nil {
		return 1, nil
	}

	var err error

	if isRed(node) && (isRed(node.left) || isRed(node.right)) {
		err = errors.New(fmt.Sprint("red violation at node val ", node.val))
		return 0, err
	}

	leftHeight, err := validateRBTree(node.left)
	if err != nil {
		return 0, err
	}
	rightHeight, err := validateRBTree(node.right)
	if err != nil {
		return 0, err
	}

	if node.left != nil && node.left.val >= node.val ||
		node.right != nil && node.right.val <= node.val {
		err = errors.New(fmt.Sprint("binary tree violation at node val ", node.val))
		return 0, err
	}

	if leftHeight != 0 && rightHeight != 0 {
		if leftHeight != rightHeight {
			err = errors.New(fmt.Sprint("black height violation at node val ", node.val))
			return 0, err
		}

		if isRed(node) {
			return leftHeight, nil
		}
		return leftHeight + 1, nil
	}
	return 0, nil
}

func Walk(node *RBNode, bh int) {
	if node == nil {
		println("--", bh)
		return
	}
	println(node.val, node.red)

	var nbh int
	if node.red == false {
		nbh = bh + 1
	} else {
		nbh = bh
	}
	Walk(node.left, nbh)
	Walk(node.right, nbh)
}

func TestInsert(t *testing.T) {
	tree := makeTree()

	height, err := validateRBTree(tree.root)

	assert.NoError(t, err, "expected tree to be a valid red black tree")
	assert.Equal(t, 3, height, "expected tree to have black height of 3")
	assert.Equal(t, 8, tree.Size(), "expected tree to have 8 nodes")

	// 4, B
	node := tree.root
	assert.Equal(t, 4, node.val, "expected tree root val to be 4")
	assert.Equal(t, "four", node.str, "expected tree root str to be 'four'")
	assert.Equal(t, false, node.red, "expected tree root to be black")
	assert.NotNil(t, node.left, "expected tree root to have left child")
	assert.NotNil(t, node.right, "expected tree root to have right child")

	// 2,R
	node = tree.root.left
	assert.Equal(t, 2, node.val, "got unexpected node val")
	assert.Equal(t, "two", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour")
	assert.NotNil(t, node.left, "expected node to have left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 1,B
	node = tree.root.left.left
	assert.Equal(t, 1, node.val, "got unexpected node val")
	assert.Equal(t, "one", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// 3, B
	node = tree.root.left.right
	assert.Equal(t, 3, node.val, "got unexpected node val")
	assert.Equal(t, "three", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// 6, R
	node = tree.root.right
	assert.Equal(t, 6, node.val, "got unexpected node val")
	assert.Equal(t, "six", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour")
	assert.NotNil(t, node.left, "expected node to have left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 5, B
	node = tree.root.right.left
	assert.Equal(t, 5, node.val, "got unexpected node val")
	assert.Equal(t, "five", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// 7, B
	node = tree.root.right.right
	assert.Equal(t, 7, node.val, "got unexpected node val")
	assert.Equal(t, "seven", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 8, R
	node = tree.root.right.right.right
	assert.Equal(t, 8, node.val, "got unexpected node val")
	assert.Equal(t, "eight", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	str, found := tree.Search(7)
	assert.True(t, found, "expected value to be found in tree")
	assert.Equal(t, "seven", str, "expected serach return value of 'seven'")

	str, found = tree.Search(9)
	assert.False(t, found, "expected value to not be found in tree")
	assert.Equal(t, "", str, "expected search to return empty string ''")
}

func TestDuplicateInsert(t *testing.T) {
	tree := makeTree()

	assert.Equal(t, 8, tree.Size(), "expected tree to have 8 nodes")

	assert.True(t, tree.Insert(9, "nine"), "failed, expected insertion success")
	assert.Equal(t, 9, tree.Size(), "expected tree to have 9 nodes")

	assert.False(t, tree.Insert(1, "one"), "success, expected insertion to fail (duplicate value)")
	assert.Equal(t, 9, tree.Size(), "expected tree to have 9 nodes")
}

func TestRemoveInsert(t *testing.T) {
	tree := makeTree()

	assert.Equal(t, 8, tree.Size(), "expected tree to have 8 nodes")

	assert.True(t, tree.Delete(2), "failed, expected successful deletion")
	assert.True(t, tree.Delete(4), "failed, expected successful deletion")

	assert.Equal(t, 6, tree.Size(), "expected tree to have 6 nodes")

	assert.True(t, tree.Insert(2, "two"), "failed, expected insertions success")
	_, err := validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")

	assert.Equal(t, 7, tree.Size(), "expected tree to have 7 nodes")

	assert.True(t, tree.Insert(4, "four"), "failed, expected insertion success")
	_, err = validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")

	assert.Equal(t, 8, tree.Size(), "expected tree to have 8 nodes")
}

func TestDelete(t *testing.T) {
	tree := makeTree()

	height, err := validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")
	assert.Equal(t, 3, height, "expected tree to have black height of 3")
	assert.Equal(t, 8, tree.Size(), "expected tree to have 8 nodes")

	//               4,B
	//             /     \
	//         2,R         6,R
	//       /     \     /     \
	//     1,B    3,B   5,B    7,B
	//                             \
	//                              8,R

	// REMOVE 1
	assert.True(t, tree.Delete(1), "expected node to be found and removed from tree")
	assert.Equal(t, 7, tree.Size(), "expected tree to have 7 nodes")
	height, err = validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")
	assert.Equal(t, 3, height, "expected tree to have black height of 3")

	// new tree:
	//               4,B
	//             /     \
	//         2,B         6,R
	//             \     /     \
	//            3,R   5,B    7,B
	//                             \
	//                              8,R

	// 4,B
	node := tree.root
	assert.Equal(t, 4, node.val, "expected tree root val to be 4")
	assert.Equal(t, "four", node.str, "expected tree root str to be 'four'")
	assert.Equal(t, false, node.red, "expected tree root to be black")
	assert.NotNil(t, node.left, "expected tree root to have left child")
	assert.NotNil(t, node.right, "expected tree root to have right child")

	// 2,R
	node = tree.root.left
	assert.Equal(t, 2, node.val, "got unexpected node val")
	assert.Equal(t, "two", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 3,B
	node = tree.root.left.right
	assert.Equal(t, 3, node.val, "got unexpected node val")
	assert.Equal(t, "three", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// 6,R
	node = tree.root.right
	assert.Equal(t, 6, node.val, "got unexpected node val")
	assert.Equal(t, "six", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour.")
	assert.NotNil(t, node.left, "expected node to have left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 5, B
	node = tree.root.right.left
	assert.Equal(t, 5, node.val, "got unexpected node val")
	assert.Equal(t, "five", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// 7, B
	node = tree.root.right.right
	assert.Equal(t, 7, node.val, "got unexpected node val")
	assert.Equal(t, "seven", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 8, R
	node = tree.root.right.right.right
	assert.Equal(t, 8, node.val, "got unexpected node val")
	assert.Equal(t, "eight", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// REMOVE 2
	assert.True(t, tree.Delete(2), "expected node to be found and removed from tree")
	assert.Equal(t, 6, tree.Size(), "expected tree to have 6 nodes")
	height, err = validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")
	assert.Equal(t, 3, height, "expected tree to have black height of 3")

	// new tree:
	//                        6,B
	//                      /     \
	//                  4,R        7,B
	//                 /   \          \
	//               3,B   5,B        8,R

	// 6,B
	node = tree.root
	assert.Equal(t, 6, node.val, "expected tree root val to be 6")
	assert.Equal(t, "six", node.str, "expected tree root str to be 6")
	assert.Equal(t, false, node.red, "expected tree root to be black")
	assert.NotNil(t, node.left, "expected tree root to have left child")
	assert.NotNil(t, node.right, "expected tree root to have right child")

	// 4,R
	node = tree.root.left
	assert.Equal(t, 4, node.val, "got unexpected node val")
	assert.Equal(t, "four", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour")
	assert.NotNil(t, node.left, "node should have left child")
	assert.NotNil(t, node.right, "node should have right child")

	// 3,B
	node = tree.root.left.left
	assert.Equal(t, 3, node.val, "got unexpected node val")
	assert.Equal(t, "three", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// 5,B
	node = tree.root.left.right
	assert.Equal(t, 5, node.val, "got unexpected node val")
	assert.Equal(t, "five", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// 7,B
	node = tree.root.right
	assert.Equal(t, 7, node.val, "got unexpected node val")
	assert.Equal(t, "seven", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 8,R
	node = tree.root.right.right
	assert.Equal(t, 8, node.val, "got unexpected node val")
	assert.Equal(t, "eight", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// REMOVE 3
	assert.True(t, tree.Delete(3), "expected node to be found and removed from tree")
	assert.Equal(t, 5, tree.Size(), "expected tree to have 5 nodes")
	height, err = validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")
	assert.Equal(t, 3, height, "expected tree to have black height of 3")

	// new tree:
	//                        6,B
	//                      /     \
	//                  4,B        7,B
	//                     \          \
	//                     5,R        8,R

	// 6,B
	node = tree.root
	assert.Equal(t, 6, node.val, "expected tree root val to be 6")
	assert.Equal(t, "six", node.str, "expected tree root str to be 6")
	assert.Equal(t, false, node.red, "expected tree root to be black")
	assert.NotNil(t, node.left, "expected tree root to have left child")
	assert.NotNil(t, node.right, "expected tree root to have right child")

	// 4,B
	node = tree.root.left
	assert.Equal(t, 4, node.val, "got unexpected node val")
	assert.Equal(t, "four", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 5,R
	node = tree.root.left.right
	assert.Equal(t, 5, node.val, "got unexpected node val")
	assert.Equal(t, "five", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// 7,B
	node = tree.root.right
	assert.Equal(t, 7, node.val, "got unexpected node val")
	assert.Equal(t, "seven", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 8,R
	node = tree.root.right.right
	assert.Equal(t, 8, node.val, "got unexpected node val")
	assert.Equal(t, "eight", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// REMOVE 4
	assert.True(t, tree.Delete(4), "expected node to be found and removed from tree")
	assert.Equal(t, 4, tree.Size(), "expected tree to have 4 nodes")
	height, err = validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")
	assert.Equal(t, 3, height, "expected tree to have black height of 3")

	// new tree:
	//                        6,B
	//                      /     \
	//                  5,B        7,B
	//                                \
	//                                8,R

	// 6,B
	node = tree.root
	assert.Equal(t, 6, node.val, "expected tree root val to be 6")
	assert.Equal(t, "six", node.str, "expected tree root str to be 6")
	assert.Equal(t, false, node.red, "expected tree root to be black")
	assert.NotNil(t, node.left, "expected tree root to have left child")
	assert.NotNil(t, node.right, "expected tree root to have right child")

	// 5,B
	node = tree.root.left
	assert.Equal(t, 5, node.val, "got unexpected node val")
	assert.Equal(t, "five", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// 7,B
	node = tree.root.right
	assert.Equal(t, 7, node.val, "got unexpected node val")
	assert.Equal(t, "seven", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 8,R
	node = tree.root.right.right
	assert.Equal(t, 8, node.val, "got unexpected node val")
	assert.Equal(t, "eight", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// REMOVE 5
	assert.True(t, tree.Delete(5), "expected node to be found and removed from tree")
	assert.Equal(t, 3, tree.Size(), "expected tree to have 3 nodes")
	height, err = validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")
	assert.Equal(t, 3, height, "expected tree to have black height of 3")

	// new tree:
	//                        7,B
	//                      /     \
	//                  6,B        8,B

	// 7,B
	node = tree.root
	assert.Equal(t, 7, node.val, "got unexpected node val")
	assert.Equal(t, "seven", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.NotNil(t, node.left, "expected node to have left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 6,B
	node = tree.root.left
	assert.Equal(t, 6, node.val, "expected tree root val to be 6")
	assert.Equal(t, "six", node.str, "expected tree root str to be 6")
	assert.Equal(t, false, node.red, "expected tree root to be black")
	assert.Nil(t, node.left, "expected tree root to have left child")
	assert.Nil(t, node.right, "expected tree root to have right child")

	// 8,R
	node = tree.root.right
	assert.Equal(t, 8, node.val, "got unexpected node val")
	assert.Equal(t, "eight", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// REMOVE 5
	assert.True(t, tree.Delete(6), "expected node to be found and removed from tree")
	assert.Equal(t, 2, tree.Size(), "expected tree to have 2 nodes")
	height, err = validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")
	assert.Equal(t, 2, height, "expected tree to have black height of 2")

	// new tree:
	//                        7,B
	//                            \
	//                             8,R

	// 7,B
	node = tree.root
	assert.Equal(t, 7, node.val, "got unexpected node val")
	assert.Equal(t, "seven", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.NotNil(t, node.right, "expected node to have right child")

	// 8,R
	node = tree.root.right
	assert.Equal(t, 8, node.val, "got unexpected node val")
	assert.Equal(t, "eight", node.str, "got unexpected node str")
	assert.Equal(t, true, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// REMOVE 7
	assert.True(t, tree.Delete(7), "expected node to be found and removed from tree")
	assert.Equal(t, 1, tree.Size(), "expected tree to have 1 node")
	height, err = validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")
	assert.Equal(t, 2, height, "expected tree to have black height of 2")

	// new tree:
	//                        8,B

	// 8,B
	node = tree.root
	assert.Equal(t, 8, node.val, "got unexpected node val")
	assert.Equal(t, "eight", node.str, "got unexpected node str")
	assert.Equal(t, false, node.red, "got unexpected node colour")
	assert.Nil(t, node.left, "expected node to have no left child")
	assert.Nil(t, node.right, "expected node to have no right child")

	// REMOVE 7
	assert.True(t, tree.Delete(8), "expected node to be found and removed from tree")
	assert.Equal(t, 0, tree.Size(), "expected tree to be empty")
	height, err = validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")
	assert.Equal(t, 1, height, "expected tree to have black height of 2")

	// EMPTY TREE!
	assert.Nil(t, tree.root, "tree root is nil")

	assert.False(t, tree.Delete(1), "success, expected deletion to fail with empty tree")
	assert.Equal(t, 0, tree.Size(), "expected tree to be empty")
	height, err = validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")
	assert.Equal(t, 1, height, "expected tree to have black height of 1")
}

func TestSearchEmpty(t *testing.T) {
	tree := RBTree{}

	str, ok := tree.Search(5)
	assert.False(t, ok, "expected node to not be found")
	assert.Equal(t, "", str, "expected str to be empty")
}

func TestSearch(t *testing.T) {
	tree := makeTree()

	//               4,B
	//             /     \
	//         2,R         6,R
	//       /     \     /     \
	//     1,B    3,B   5,B    7,B
	//                             \
	//                              8,R

	str, ok := tree.Search(5)
	assert.True(t, ok, "expected node to be found")
	assert.Equal(t, "five", str, "expected str to be five")

	str, ok = tree.Search(3)
	assert.True(t, ok, "expected node to be found")
	assert.Equal(t, "three", str, "expected str to be three")

	str, ok = tree.Search(8)
	assert.True(t, ok, "expected node to be found")
	assert.Equal(t, "eight", str, "expected str to be three")

	str, ok = tree.Search(9)
	assert.False(t, ok, "expected node to not be found")
	assert.Equal(t, "", str, "expected str to be empty")
}

func TestBig(t *testing.T) {
	tree := RBTree{}
	random := rand.New(rand.NewSource(1337))

	for i := 0; i < 2000; i++ {
		n := random.Intn(10000)
		tree.Insert(n, strconv.Itoa(n))
	}

	_, err := validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")

	for i := 0; i < 1000; i++ {
		n := random.Intn(10000)
		tree.Delete(n)
	}

	_, err = validateRBTree(tree.root)
	assert.NoError(t, err, "expected tree to be a valid red black tree")
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// BENCHMARKS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func benchmarkRBTreeInsert(b *testing.B) {
	for n := 0; n < b.N; n++ {
		tree := RBTree{}
		for i := 0; i < 1000000; i++ {
			tree.Insert(i, "something")
		}
	}
}

func benchmarkRBTreeDelete(b *testing.B) {
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		// stop timer while tree is created
		tree := RBTree{}
		for i := 0; i < 1000000; i++ {
			tree.Insert(i, "something")
		}
		b.StartTimer()
		// restart timer and time only deletion
		for i := 0; i < 1000000; i++ {
			tree.Delete(i)
		}
	}
}

func benchmarkRBTreeSearch(b *testing.B) {
	b.StopTimer()
	tree := RBTree{}
	for i := 0; i < 1000000; i++ {
		tree.Insert(i, "something")
	}
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < 1000000; i++ {
			tree.Search(i)
		}
	}
}

func opsPerSec(nspop int64) int64 {
	return int64(math.Floor(float64(1000000 * float64(1e9) / float64(nspop))))
}

// func TestRunBenchmarks(t *testing.T) {
// 	bmres := testing.Benchmark(benchmarkRBTreeInsert)
//
// 	fmt.Print("BenchmarkRBTreeInsert - Inserting 1 000 000 items into tree		")
// 	fmt.Println("Ops per second: ", opsPerSec(bmres.NsPerOp()))
//
// 	bmres = testing.Benchmark(benchmarkRBTreeDelete)
//
// 	fmt.Print("BenchmarkRBTreeDelete - Deleting 1 000 000 items from tree		")
// 	fmt.Println("Ops per second: ", opsPerSec(bmres.NsPerOp()))
//
// 	bmres = testing.Benchmark(benchmarkRBTreeSearch)
//
// 	fmt.Print("BenchmarkRBTreeSearch - Searching for each item in tree			")
// 	fmt.Println("Ops per second: ", opsPerSec(bmres.NsPerOp()))
//
// 	fmt.Println()
// }
