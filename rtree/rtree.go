package rtree

import (
	"errors"
)

// node is a node in an R-Tree. nodes can either be leaf nodes holding entries
// for terminal items, or intermediate nodes holding entries for more nodes.
type node struct {
	entries    [1 + maxChildren]entry
	numEntries int
	parent     int
	isLeaf     bool
}

// entry is an entry under a node, leading either to terminal items, or more nodes.
type entry struct {
	box Box

	// For leaf nodes, this is a recordID. For non-leaf nodes, it is the child.
	data int
}

func (t *RTree) appendRecord(nodeIdx int, box Box, recordID int) {
	node := t.node(nodeIdx)
	node.entries[node.numEntries] = entry{box: box, data: recordID}
	node.numEntries++
}

func (t *RTree) appendChild(nodeIdx int, box Box, childIdx int) {
	node := t.node(nodeIdx)
	node.entries[node.numEntries] = entry{box: box, data: childIdx}
	node.numEntries++
	t.node(childIdx).parent = nodeIdx
}

// depth calculates the number of layers of nodes in the subtree rooted at the node.
func (t *RTree) nodeDepth(nodeIdx int) int {
	node := t.node(nodeIdx)
	var d = 1
	for !node.isLeaf {
		d++
		node = t.node(node.entries[0].data)
	}
	return d
}

// RTree is an in-memory R-Tree data structure. It holds record ID and bounding
// box pairs (the actual records aren't stored in the tree; the user is
// responsible for storing their own records). Its zero value is an empty
// R-Tree.
type RTree struct {
	nodes []node // 1-indexed, allowing 0 to represent "nil"
	root  int
}

// node converts a 1-indexed node index into a node pointer
func (t *RTree) node(nodeIdx int) *node {
	return &t.nodes[nodeIdx-1]
}

// Stop is a special sentinal error that can be used to stop a search operation
// without any error.
var Stop = errors.New("stop")

// RangeSearch looks for any items in the tree that overlap with the given
// bounding box. The callback is called with the record ID for each found item.
// If an error is returned from the callback then the search is terminated
// early.  Any error returned from the callback is returned by RangeSearch,
// except for the case where the special Stop sentinal error is returned (in
// which case nil will be returned from RangeSearch).
func (t *RTree) RangeSearch(box Box, callback func(recordID int) error) error {
	if t.root == 0 {
		return nil
	}
	var recurse func(*node) error
	recurse = func(n *node) error {
		for i := 0; i < n.numEntries; i++ {
			entry := n.entries[i]
			if !overlap(entry.box, box) {
				continue
			}
			if n.isLeaf {
				if err := callback(entry.data); err == Stop {
					return nil
				} else if err != nil {
					return err
				}
			} else {
				if err := recurse(t.node(entry.data)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return recurse(t.node(t.root))
}

// Extent gives the Box that most closely bounds the RTree. If the RTree is
// empty, then false is returned.
func (t *RTree) Extent() (Box, bool) {
	if t.root == 0 {
		return Box{}, false
	}
	root := t.node(t.root)
	if root.numEntries == 0 {
		return Box{}, false
	}
	return calculateBound(root), true
}
