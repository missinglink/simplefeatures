package rtree

import (
	"math"
	"math/bits"
)

const (
	minChildren = 2
	maxChildren = 4
)

// Insert adds a new record to the RTree.
func (t *RTree) Insert(box Box, recordID int) {
	if !t.hasRoot() {
		t.nodes = append(t.nodes, node{isLeaf: true})
		t.root = len(t.nodes) - 1
	}

	level := t.nodeDepth(t.root) - 1
	leaf := t.chooseBestNode(box, level)

	t.appendRecord(leaf, box, recordID)
	t.adjustBoxesUpwards(leaf, box)

	if t.nodes[leaf].numEntries <= maxChildren {
		return
	}

	newNode := t.splitNode(leaf)
	root1, root2 := t.adjustTree(leaf, newNode)
	if root2 != -1 {
		t.joinRoots(root1, root2)
	}
}

// adjustBoxesUpwards expands the boxes from the given node all the way to the
// root by the given box.
func (t *RTree) adjustBoxesUpwards(node int, box Box) {
	for node != t.root {
		parent := t.nodes[node].parent
		for i := 0; i < t.nodes[parent].numEntries; i++ {
			e := &t.nodes[parent].entries[i]
			if e.data == node {
				e.box = combine(e.box, box)
			}
		}
		node = parent
	}
}

func (t *RTree) joinRoots(r1, r2 int) {
	t.nodes = append(t.nodes, node{
		entries: [1 + maxChildren]entry{
			entry{box: calculateBound(&t.nodes[r1]), data: r1},
			entry{box: calculateBound(&t.nodes[r2]), data: r2},
		},
		numEntries: 2,
		parent:     -1,
		isLeaf:     false,
	})
	newRoot := len(t.nodes) - 1
	t.nodes[r1].parent = newRoot
	t.nodes[r2].parent = newRoot
	t.root = newRoot
}

// TODO: rename n and nn to leaf and newNode
func (t *RTree) adjustTree(n, nn int) (int, int) {
	for {
		if n == t.root {
			return n, nn
		}
		parent := t.nodes[n].parent
		for i := 0; i < t.nodes[parent].numEntries; i++ {
			if t.nodes[parent].entries[i].data == n {
				t.nodes[parent].entries[i].box = calculateBound(&t.nodes[n])
				break
			}
		}

		// AT4
		var pp int
		if nn != -1 {
			t.appendChild(parent, calculateBound(&t.nodes[nn]), nn)
			if t.nodes[parent].numEntries > maxChildren {
				pp = t.splitNode(parent)
			}
		}

		n, nn = parent, pp
	}
}

// splitNode splits node with index n into two nodes. The first node replaces
// n, and the second node is newly created. The return value is the index of
// the new node.
func (t *RTree) splitNode(n int) int {
	var (
		// All zeros would not be valid split, so start at 1.
		minSplit = uint64(1)
		// The MSB should always be 0, to remove duplicates from inverting the
		// bit pattern. So we raise 2 to the power of one less than the number
		// of entries rather than the number of entries.
		//
		// E.g. for 4 entries, we want the following bit patterns:
		// 0001, 0010, 0011, 0100, 0101, 0110, 0111.
		//
		// (1 << (4 - 1)) - 1 == 0111, so the maths checks out.
		maxSplit = uint64((1 << (t.nodes[n].numEntries - 1)) - 1)
	)
	bestArea := math.Inf(+1)
	var bestSplit uint64
	for split := minSplit; split <= maxSplit; split++ {
		if ones := bits.OnesCount64(split); ones < minChildren || (t.nodes[n].numEntries-ones) < minChildren {
			continue
		}
		var boxA, boxB Box
		var hasA, hasB bool
		for i := 0; i < t.nodes[n].numEntries; i++ {
			entryBox := t.nodes[n].entries[i].box
			if split&(1<<i) == 0 {
				if hasA {
					boxA = combine(boxA, entryBox)
				} else {
					boxA = entryBox
				}
			} else {
				if hasB {
					boxB = combine(boxB, entryBox)
				} else {
					boxB = entryBox
				}
			}
		}
		combinedArea := area(boxA) + area(boxB)
		if combinedArea < bestArea {
			bestArea = combinedArea
			bestSplit = split
		}
	}

	// Use the existing node for the 0 bits in the split, and a new node for
	// the 1 bits in the split.
	t.nodes = append(t.nodes, node{isLeaf: t.nodes[n].isLeaf})
	newNode := len(t.nodes) - 1
	totalEntries := t.nodes[n].numEntries
	t.nodes[n].numEntries = 0
	for i := 0; i < totalEntries; i++ {
		entry := t.nodes[n].entries[i]
		if bestSplit&(1<<i) == 0 {
			t.nodes[n].entries[t.nodes[n].numEntries] = entry
			t.nodes[n].numEntries++
		} else {
			t.nodes[newNode].entries[t.nodes[newNode].numEntries] = entry
			t.nodes[newNode].numEntries++
		}
	}
	for i := t.nodes[n].numEntries; i < len(t.nodes[n].entries); i++ {
		t.nodes[n].entries[i] = entry{}
	}
	if !t.nodes[n].isLeaf {
		for i := 0; i < t.nodes[newNode].numEntries; i++ {
			t.nodes[t.nodes[newNode].entries[i].data].parent = newNode
		}
	}
	return newNode
}

// chooseBestNode chooses the best node in the tree under which to insert a new
// entry. The Box is the box of the new entry, and the level is the level of
// the tree on which the best node will be found (where the root is at level 0,
// the nodes under the root are level 1 etc.).
func (t *RTree) chooseBestNode(box Box, level int) int {
	node := t.root
	for {
		if level == 0 {
			return node
		}
		bestDelta := enlargement(box, t.nodes[node].entries[0].box)
		bestEntry := 0
		for i := 1; i < t.nodes[node].numEntries; i++ {
			entryBox := t.nodes[node].entries[i].box
			delta := enlargement(box, entryBox)
			if delta < bestDelta {
				bestDelta = delta
				bestEntry = i
			} else if delta == bestDelta && area(entryBox) < area(t.nodes[node].entries[bestEntry].box) {
				// Area is used as a tie breaking if the enlargements are the same.
				bestEntry = i
			}
		}
		node = t.nodes[node].entries[bestEntry].data
		level--
	}
}
