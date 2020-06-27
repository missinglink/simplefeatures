package rtree

import "container/heap"

// PrioritySearch iterates over the records in the RTree in priority order of
// distance from the input box (shortest distanace first using the Euclidean
// metric).  The callback is called for every element iterated over. If an
// error is returned from the callback, then iteration stops immediately. Any
// error returned from the callback is returned by PrioritySearch, except for
// the case where the special Stop sentinal error is returned (in which case
// nil will be returned from PrioritySearch).
func (t *RTree) PrioritySearch(box Box, callback func(recordID int) error) error {
	if !t.hasRoot() {
		return nil
	}

	queue := entriesQueue{origin: box}
	equeueNode := func(n *node) {
		for i := 0; i < n.numEntries; i++ {
			heap.Push(&queue, entryWithChildMarker{&n.entries[i], !n.isLeaf})
		}
	}

	equeueNode(&t.nodes[t.root])
	for len(queue.entries) > 0 {
		nearest := heap.Pop(&queue).(entryWithChildMarker)
		if !nearest.hasChild {
			if err := callback(nearest.data); err != nil {
				if err == Stop {
					return nil
				}
				return err
			}
		} else {
			equeueNode(&t.nodes[nearest.data])
		}
	}
	return nil
}

type entryWithChildMarker struct {
	*entry
	hasChild bool
}

type entriesQueue struct {
	entries []entryWithChildMarker
	origin  Box
}

func (q *entriesQueue) Len() int {
	return len(q.entries)
}

func (q *entriesQueue) Less(i int, j int) bool {
	e1 := q.entries[i]
	e2 := q.entries[j]
	return squaredEuclideanDistance(e1.box, q.origin) < squaredEuclideanDistance(e2.box, q.origin)
}

func (q *entriesQueue) Swap(i int, j int) {
	q.entries[i], q.entries[j] = q.entries[j], q.entries[i]
}

func (q *entriesQueue) Push(x interface{}) {
	q.entries = append(q.entries, x.(entryWithChildMarker))
}

func (q *entriesQueue) Pop() interface{} {
	e := q.entries[len(q.entries)-1]
	q.entries = q.entries[:len(q.entries)-1]
	return e
}
