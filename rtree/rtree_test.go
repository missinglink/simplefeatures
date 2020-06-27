package rtree

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

func testBulkLoad(rnd *rand.Rand, pop int, maxStart, maxWidth float64) (*RTree, []Box) {
	boxes := make([]Box, pop)
	seenX := make(map[float64]bool)
	seenY := make(map[float64]bool)
	for i := range boxes {
		var box Box
		for {
			box = randomBox(rnd, maxStart, maxWidth)
			x := box.MinX + box.MaxX
			y := box.MinY + box.MaxY
			if !seenX[x] && !seenY[y] {
				seenX[x] = true
				seenY[y] = true
				break
			}
		}
		boxes[i] = box
	}
	inserts := make([]BulkItem, len(boxes))
	for i := range inserts {
		inserts[i].Box = boxes[i]
		inserts[i].RecordID = i
	}
	return BulkLoad(inserts), boxes
}

func testPopulations(manditory, max int, mult float64) []int {
	var pops []int
	for i := 0; i < manditory; i++ {
		pops = append(pops, i)
	}
	for pop := float64(manditory); pop < float64(max); pop *= mult {
		pops = append(pops, int(pop))
	}
	return pops
}

func TestRandom(t *testing.T) {
	for _, population := range testPopulations(66, 1000, 1.2) {
		t.Run(fmt.Sprintf("bulk_%d", population), func(t *testing.T) {
			rnd := rand.New(rand.NewSource(0))
			rt, boxes := testBulkLoad(rnd, population, 0.9, 0.1)
			checkInvariants(t, rt, boxes)
			checkSearch(t, rt, boxes, rnd)
		})
		name := fmt.Sprintf("insert_%d", population)
		t.Run(name, func(t *testing.T) {
			rnd := rand.New(rand.NewSource(0))
			boxes := make([]Box, population)
			for i := range boxes {
				boxes[i] = randomBox(rnd, 0.9, 0.1)
			}

			rt := new(RTree)
			for i, box := range boxes {
				rt.Insert(box, i)
				checkInvariants(t, rt, boxes[:i+1])
			}

			checkSearch(t, rt, boxes, rnd)
		})
	}
}

func TestDelete(t *testing.T) {
	for _, population := range testPopulations(66, 1000, 1.5) {
		t.Run(fmt.Sprintf("pop=%d", population), func(t *testing.T) {
			rnd := rand.New(rand.NewSource(0))
			rt, boxes := testBulkLoad(rnd, population, 0.9, 0.1)
			checkInvariants(t, rt, boxes)

			for i := len(boxes) - 1; i >= 0; i-- {
				t.Logf("deleting recordID %d", i)
				rt.Delete(boxes[i], i)
				checkInvariants(t, rt, boxes[:i])
				checkSearch(t, rt, boxes[:i], rnd)
			}
		})
	}
}

func checkSearch(t *testing.T, rt *RTree, boxes []Box, rnd *rand.Rand) {
	for i := 0; i < 10; i++ {
		searchBB := randomBox(rnd, 0.5, 0.5)
		var got []int
		rt.RangeSearch(searchBB, func(idx int) error {
			got = append(got, idx)
			return nil
		})

		var want []int
		for i, box := range boxes {
			if overlap(box, searchBB) {
				want = append(want, i)
			}
		}

		sort.Ints(want)
		sort.Ints(got)

		if !reflect.DeepEqual(want, got) {
			t.Logf("search box: %v", searchBB)
			t.Errorf("search failed, got: %v want: %v", got, want)
		}
	}
}

func randomBox(rnd *rand.Rand, maxStart, maxWidth float64) Box {
	box := Box{
		MinX: rnd.Float64() * maxStart,
		MinY: rnd.Float64() * maxStart,
	}
	box.MaxX = box.MinX + rnd.Float64()*maxWidth
	box.MaxY = box.MinY + rnd.Float64()*maxWidth

	box.MinX = float64(int(box.MinX*1_000_000)) / 1_000_000
	box.MinY = float64(int(box.MinY*1_000_000)) / 1_000_000
	box.MaxX = float64(int(box.MaxX*1_000_000)) / 1_000_000
	box.MaxY = float64(int(box.MaxY*1_000_000)) / 1_000_000
	return box
}

func checkInvariants(t *testing.T, rt *RTree, boxes []Box) {
	var recurse func(int, string)
	recurse = func(currentIdx int, indent string) {
		current := &rt.nodes[currentIdx]
		t.Logf("%sNode addr=%d parent=%d leaf=%t numEntries=%d", indent, currentIdx, current.parent, current.isLeaf, current.numEntries)
		indent += "\t"
		if current.isLeaf {
			for i := 0; i < current.numEntries; i++ {
				e := current.entries[i]
				t.Logf("%sEntry[%d] recordID=%d box=%v", indent, i, e.data, e.box)
			}
		} else {
			for i := 0; i < current.numEntries; i++ {
				e := &current.entries[i]
				t.Logf("%sEntry[%d] box=%v", indent, i, e.box)
				recurse(e.data, indent+"\t")
			}
		}
	}
	t.Log("---")
	if rt.hasRoot() {
		recurse(rt.root, "")
	} else {
		t.Log("Root is nil")
	}
	t.Log("---")

	unfound := make(map[int]struct{})
	for i := range boxes {
		unfound[i] = struct{}{}
	}

	leafLevel := -1
	var check func(n int, level int)
	check = func(currentIdx int, level int) {
		current := &rt.nodes[currentIdx]
		if current.isLeaf {
			if leafLevel == -1 {
				leafLevel = level
			} else if leafLevel != level {
				t.Fatalf("inconsistent leaf level: %d vs %d", leafLevel, level)
			}

			for i := 0; i < current.numEntries; i++ {
				e := current.entries[i]
				if _, ok := unfound[e.data]; !ok {
					t.Fatal("record ID found in tree but wasn't in unfound map")
				}
				delete(unfound, e.data)
			}
		} else {
			for i := 0; i < current.numEntries; i++ {
				e := &current.entries[i]
				if rt.nodes[e.data].parent != currentIdx {
					t.Fatalf("node %d has wrong parent", e.data)
				}
				box := rt.nodes[e.data].entries[0].box
				for j := 1; j < rt.nodes[e.data].numEntries; j++ {
					box = combine(box, rt.nodes[e.data].entries[j].box)
				}
				if box != e.box {
					t.Fatalf("entry box doesn't match smallest box enclosing children")
				}
				check(e.data, level+1)
			}
		}
		for i := current.numEntries; i < len(current.entries); i++ {
			e := current.entries[i]
			if e.box != (Box{}) || e.data != 0 {
				t.Fatal("entry past numEntries is not the zero value")
			}
		}
		if current.numEntries > maxChildren || (currentIdx != rt.root && current.numEntries < minChildren) {
			t.Fatalf("%p: unexpected number of entries", current)
		}
	}
	if rt.hasRoot() {
		check(rt.root, 0)
		if rt.nodes[rt.root].parent != -1 {
			t.Fatalf("root parent should be -1, but is %d", rt.nodes[rt.root].parent)
		}
	}

	if len(unfound) != 0 {
		t.Fatalf("there were still unfound record IDs after traversing tree")
	}

	gotExtent, hasExtent := rt.Extent()
	if len(boxes) == 0 {
		if hasExtent {
			t.Fatal("expected not to get an extent, but got one")
		}
	} else {
		if !hasExtent {
			t.Fatalf("expected to get an extent, but didn't")
		}
		wantExtent := boxes[0]
		for _, b := range boxes[1:] {
			wantExtent = combine(wantExtent, b)
		}
		if wantExtent != gotExtent {
			t.Fatalf("unexpected bounding box: want=%v got=%v", wantExtent, gotExtent)
		}
	}
}
