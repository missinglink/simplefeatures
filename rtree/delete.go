package rtree

// Delete removes a single record with a matching recordID from the RTree. The
// box specifies where to search in the RTree for the record (the search box
// must intersect with the box of the record for it to be found and deleted).
// The returned bool indicates whether or not the record could be found and
// thus removed from the RTree (true indicates success).
func (t *RTree) Delete(box Box, recordID int) bool {
	if t.root == 0 {
		return false
	}

	// D1 [Find node containing record]
	foundNode := 0
	var foundEntryIndex int
	var recurse func(int)
	recurse = func(nodeIdx int) {
		n := t.node(nodeIdx)
		for i := 0; i < n.numEntries; i++ {
			entry := n.entries[i]
			if !overlap(entry.box, box) {
				continue
			}
			if !n.isLeaf {
				recurse(entry.data)
				if foundNode != 0 {
					break
				}
			} else {
				if entry.data == recordID {
					foundNode = nodeIdx
					foundEntryIndex = i
					break
				}
			}
		}
	}
	recurse(t.root)
	if foundNode == 0 {
		return false
	}

	// D2 [Delete record]
	t.deleteEntry(foundNode, foundEntryIndex)

	// D3 [Propagate changes]
	t.condenseTree(foundNode)

	// D4 [Shorten tree]
	if root := t.node(t.root); !root.isLeaf && root.numEntries == 1 {
		t.root = root.entries[0].data
		t.node(t.root).parent = 0
	}

	return true
}

func (t *RTree) deleteEntry(nodeIdx int, entryIdx int) {
	n := t.node(nodeIdx)
	n.entries[entryIdx] = n.entries[n.numEntries-1]
	n.numEntries--
	n.entries[n.numEntries] = entry{}
}

func (t *RTree) condenseTree(leaf int) {
	// CT1 [Initialise]
	var eliminated []int
	current := leaf

	for current != t.root {
		// CT2 [Find Parent Entry]
		currentNode := t.node(current)
		parent := currentNode.parent
		parentNode := t.node(parent)
		entryIdx := -1
		for i := 0; i < parentNode.numEntries; i++ {
			if parentNode.entries[i].data == current {
				entryIdx = i
				break
			}
		}

		// CT3 [Eliminate Under-Full Node]
		if currentNode.numEntries < minChildren {
			eliminated = append(eliminated, current)
			t.deleteEntry(parent, entryIdx)
		} else {
			// CT4 [Adjust Covering Rectangle]
			newBox := currentNode.entries[0].box
			for i := 1; i < currentNode.numEntries; i++ {
				newBox = combine(newBox, currentNode.entries[i].box)
			}
			parentNode.entries[entryIdx].box = newBox
		}

		// CT5 [Move Up One Level In Tree]
		current = parent
	}

	// CT6 [Reinsert orphaned entries]
	for _, nodeIdx := range eliminated {
		node := t.node(nodeIdx)
		if node.isLeaf {
			for i := 0; i < node.numEntries; i++ {
				e := node.entries[i]
				t.Insert(e.box, e.data)
			}
		} else {
			for i := 0; i < node.numEntries; i++ {
				t.reInsertNode(node.entries[i].data)
			}
		}
	}
}

// reInsertNode reinserts the subtree rooted at a node that was previously
// deleted from the tree.
func (t *RTree) reInsertNode(node int) {
	box := calculateBound(t.node(node))
	treeDepth := t.nodeDepth(t.root)
	nodeDepth := t.nodeDepth(node)
	insNode := t.chooseBestNode(box, treeDepth-nodeDepth-1)

	t.appendChild(insNode, box, node)
	t.adjustBoxesUpwards(node, box)

	if t.node(insNode).numEntries <= maxChildren {
		return
	}

	newNode := t.splitNode(insNode)
	root1, root2 := t.adjustTree(insNode, newNode)
	if root2 != 0 {
		t.joinRoots(root1, root2)
	}
}
