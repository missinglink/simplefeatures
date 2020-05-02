package geom

import (
	"math"
	"sort"
)

func newNearestPoint(target Point) nearestPoint {
	return nearestPoint{target: target}
}

type nearestPoint struct {
	target Point
	point  Point
	dist   float64
}

func (n *nearestPoint) add(candidate Point) {
	targetXY, ok := n.target.XY()
	if !ok {
		return
	}
	candidateXY, ok := candidate.XY()
	if !ok {
		return
	}

	// ulpTolerance is the number of ULPs that the candidate point has to be
	// nearer to the target compared to the existing best point for it to be
	// considerer the new best point. This is to help work around numerical
	// precision issues where two points should be the 'same' distance away
	// from another point, but one is is slightly closed when using IEEE
	// floating point arithmetic. In reality, it doesn't matter which point we
	// choose (it's arbitrary). However using this approach helps to match GEOS
	// behaviour.
	const ulpTolerance = 15

	delta := targetXY.Sub(candidateXY)
	candidateDist := delta.Dot(delta)
	if n.point.IsEmpty() || diffULP(n.dist, candidateDist) > ulpTolerance {
		n.dist = candidateDist
		n.point = candidate
	}
}

func pointOnAreaSurface(poly Polygon) (Point, float64) {
	// Algorithm overview:
	//
	// 1. Find the middle of the envelope around the Polygon.
	//
	// 2. If the Y value of any control points in the polygon share that
	// mid-envelope Y value, then choose a new Y value. The new Y value is the
	// average of the mid-envelope Y value and the Y value of the next highest
	// control point.
	//
	// 3. Construct a bisector line that crosses through the polygon at the
	// height of the chosen Y value.
	//
	// 4. Find the largest portion of the bisector line that intersects with the Polygon.
	//
	// 5. The PointOnSurface is the midpoint of that largest portion.

	// Find envelope midpoint.
	env, ok := poly.Envelope()
	if !ok {
		return Point{}, 0
	}
	midY := env.Center().Y

	// Adjust mid-y value if a control point has the same Y.
	var midYMatchesNode bool
	nextY := math.Inf(+1)
	for _, ring := range poly.rings {
		seq := ring.Coordinates()
		for i := 0; i < seq.Length(); i++ {
			xy := seq.GetXY(i)
			if xy.Y == midY {
				midYMatchesNode = true
			}
			if xy.Y < nextY && xy.Y > midY {
				nextY = xy.Y
			}
		}
	}
	if midYMatchesNode {
		midY = (midY + nextY) / 2
	}

	// Create bisector.
	bisector := line{
		XY{env.Min().X - 1, midY},
		XY{env.Max().X + 1, midY},
	}

	// Find intersection points between the bisector and the polygon.
	var xIntercepts []float64
	for _, ring := range poly.rings {
		seq := ring.Coordinates()
		n := seq.Length()
		for i := 0; i < n; i++ {
			ln, ok := getLine(seq, i)
			if !ok {
				continue
			}
			inter := ln.intersectLine(bisector)
			if inter.empty {
				continue
			}
			// It shouldn't _ever_ be the case that inter.ptA is different from
			// inter.ptB, as this would imply that there is a line in the
			// polygon that is horizontal and has the same Y value as our
			// bisector. But from the way the bisector was constructed, this
			// can't happen. So we can just use inter.ptA.
			xIntercepts = append(xIntercepts, inter.ptA.X)
		}
	}
	xIntercepts = sortAndUniquifyFloats(xIntercepts)

	// Find largest portion of bisector that intersects the polygon.
	if len(xIntercepts) < 2 || len(xIntercepts)%2 != 0 {
		// The only way this could happen is if the input Polygon is invalid,
		// or there is some sort of pathological case. So we just return an
		// arbitrary point on the Polygon.
		return poly.ExteriorRing().StartPoint(), 0
	}
	bestA, bestB := xIntercepts[0], xIntercepts[1]
	for i := 2; i+1 < len(xIntercepts); i += 2 {
		newA, newB := xIntercepts[i], xIntercepts[i+1]
		if newB-newA > bestB-bestA {
			bestA, bestB = newA, newB
		}
	}
	midX := (bestA + bestB) / 2

	return NewPointFromXY(XY{midX, midY}), bestB - bestA
}

func sortAndUniquifyFloats(fs []float64) []float64 {
	if len(fs) == 0 {
		return fs
	}
	sort.Float64s(fs)
	n := 1
	for i := 1; i < len(fs); i++ {
		if fs[i] != fs[i-1] {
			fs[n] = fs[i]
			n++
		}
	}
	return fs[:n]
}
