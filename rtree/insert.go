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
	if t.root == 0 {
		t.nodes = append(t.nodes, node{isLeaf: true})
		t.root = len(t.nodes)
	}

	level := t.nodeDepth(t.root) - 1
	leafIdx := t.chooseBestNode(box, level)

	t.appendRecord(leafIdx, box, recordID)
	t.adjustBoxesUpwards(leafIdx, box)

	if t.node(leafIdx).numEntries <= maxChildren {
		return
	}

	newNodeIdx := t.splitNode(leafIdx)
	root1, root2 := t.adjustTree(leafIdx, newNodeIdx)
	if root2 != 0 {
		t.joinRoots(root1, root2)
	}
}

// adjustBoxesUpwards expands the boxes from the given node all the way to the
// root by the given box.
func (t *RTree) adjustBoxesUpwards(nodeIdx int, box Box) {
	for nodeIdx != t.root {
		node := t.node(nodeIdx)
		parent := t.node(node.parent)
		for i := 0; i < parent.numEntries; i++ {
			e := &parent.entries[i]
			if e.data == nodeIdx {
				e.box = combine(e.box, box)
			}
		}
		nodeIdx = node.parent
	}
}

func (t *RTree) joinRoots(root1Idx, root2Idx int) {
	t.nodes = append(t.nodes, node{
		entries: [1 + maxChildren]entry{
			entry{box: calculateBound(t.node(root1Idx)), data: root1Idx},
			entry{box: calculateBound(t.node(root2Idx)), data: root2Idx},
		},
		numEntries: 2,
		parent:     0,
		isLeaf:     false,
	})
	newRootIdx := len(t.nodes)
	t.node(root1Idx).parent = newRootIdx
	t.node(root2Idx).parent = newRootIdx
	t.root = newRootIdx
}

func (t *RTree) adjustTree(leafIdx, newNodeIdx int) (int, int) {
	for {
		if leafIdx == t.root {
			return leafIdx, newNodeIdx
		}
		leaf := t.node(leafIdx)
		parent := t.node(leaf.parent)
		for i := 0; i < parent.numEntries; i++ {
			if parent.entries[i].data == leafIdx {
				parent.entries[i].box = calculateBound(leaf)
				break
			}
		}

		// AT4
		var splitParentIdx int
		leafParentIdx := leaf.parent
		if newNodeIdx != 0 {
			t.appendChild(leaf.parent, calculateBound(t.node(newNodeIdx)), newNodeIdx)
			if parent.numEntries > maxChildren {
				splitParentIdx = t.splitNode(leaf.parent)
			}
		}

		leafIdx, newNodeIdx = leafParentIdx, splitParentIdx
	}
}

// splitNode splits node with index n into two nodes. The first node replaces
// n, and the second node is newly created. The return value is the index of
// the new node.
func (t *RTree) splitNode(nodeIdx int) int {
	n := t.node(nodeIdx)

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
		maxSplit = uint64((1 << (n.numEntries - 1)) - 1)
	)
	bestArea := math.Inf(+1)
	var bestSplit uint64
	for split := minSplit; split <= maxSplit; split++ {
		if ones := bits.OnesCount64(split); ones < minChildren || (n.numEntries-ones) < minChildren {
			continue
		}
		var boxA, boxB Box
		var hasA, hasB bool
		for i := 0; i < n.numEntries; i++ {
			entryBox := n.entries[i].box
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
	t.nodes = append(t.nodes, node{isLeaf: n.isLeaf})
	newNodeIdx := len(t.nodes)
	newNode := t.node(newNodeIdx)
	n = t.node(nodeIdx)
	totalEntries := n.numEntries
	n.numEntries = 0
	for i := 0; i < totalEntries; i++ {
		entry := n.entries[i]
		if bestSplit&(1<<i) == 0 {
			n.entries[n.numEntries] = entry
			n.numEntries++
		} else {
			newNode.entries[newNode.numEntries] = entry
			newNode.numEntries++
		}
	}
	for i := n.numEntries; i < len(n.entries); i++ {
		n.entries[i] = entry{}
	}
	if !n.isLeaf {
		for i := 0; i < newNode.numEntries; i++ {
			t.node(newNode.entries[i].data).parent = newNodeIdx
		}
	}
	return newNodeIdx
}

// chooseBestNode chooses the best node in the tree under which to insert a new
// entry. The Box is the box of the new entry, and the level is the level of
// the tree on which the best node will be found (where the root is at level 0,
// the nodes under the root are level 1 etc.).
func (t *RTree) chooseBestNode(box Box, level int) int {
	currentIdx := t.root
	for {
		if level == 0 {
			return currentIdx
		}
		current := t.node(currentIdx)
		bestDelta := enlargement(box, current.entries[0].box)
		bestEntry := 0
		for i := 1; i < current.numEntries; i++ {
			entryBox := current.entries[i].box
			delta := enlargement(box, entryBox)
			if delta < bestDelta {
				bestDelta = delta
				bestEntry = i
			} else if delta == bestDelta && area(entryBox) < area(current.entries[bestEntry].box) {
				// Area is used as a tie breaking if the enlargements are the same.
				bestEntry = i
			}
		}
		currentIdx = current.entries[bestEntry].data
		level--
	}
}
