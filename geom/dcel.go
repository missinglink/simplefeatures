package geom

import (
	"errors"
	"fmt"
)

type doublyConnectedEdgeList struct {
	faces     []*faceRecord
	halfEdges []*halfEdgeRecord
	vertices  map[XY]*vertexRecord
}

type faceRecord struct {
	outerComponent *halfEdgeRecord

	// TODO: can remove these when we're done?
	innerComponents  []*halfEdgeRecord
	internalVertices []*vertexRecord // only populated in the overlay

	label uint8
}

type halfEdgeRecord struct {
	origin     *vertexRecord
	twin       *halfEdgeRecord
	incident   *faceRecord
	next, prev *halfEdgeRecord
	label      uint8
}

// String shows the origin and destination of the edge (for debugging
// purposes). We can remove this once DCEL active development is completed.
func (e *halfEdgeRecord) String() string {
	if e == nil {
		return "nil"
	}
	return fmt.Sprintf("%v->%v", e.origin.coords, e.twin.origin.coords)
}

type vertexRecord struct {
	coords    XY
	incidents []*halfEdgeRecord
	label     uint8
}

func newDCELFromGeometry(g Geometry, mask uint8) (*doublyConnectedEdgeList, error) {
	switch g.Type() {
	case TypePolygon:
		poly := g.AsPolygon()
		return newDCELFromMultiPolygon(poly.AsMultiPolygon(), mask), nil
	case TypeMultiPolygon:
		mp := g.AsMultiPolygon()
		return newDCELFromMultiPolygon(mp, mask), nil
	case TypeLineString:
		mls := g.AsLineString().AsMultiLineString()
		return newDCELFromMultiLineString(mls, mask), nil
	case TypeMultiLineString:
		mls := g.AsMultiLineString()
		return newDCELFromMultiLineString(mls, mask), nil
	case TypePoint:
		mp := NewMultiPointFromPoints([]Point{g.AsPoint()})
		return newDCELFromMultiPoint(mp, mask), nil
	case TypeMultiPoint:
		mp := g.AsMultiPoint()
		return newDCELFromMultiPoint(mp, mask), nil
	case TypeGeometryCollection:
		// TODO: Add support for GeometryCollection inputs.
		return nil, errors.New("GeometryCollection argument not supported")
	default:
		panic(fmt.Sprintf("unknown geometry type: %v", g.Type()))
	}
}

func newDCELFromMultiPolygon(mp MultiPolygon, mask uint8) *doublyConnectedEdgeList {
	mp = mp.ForceCCW()

	dcel := &doublyConnectedEdgeList{vertices: make(map[XY]*vertexRecord)}

	infFace := &faceRecord{
		outerComponent:  nil, // infinite face has no outer component, so left nil
		innerComponents: nil, // populated later
		label:           populatedMask & mask,
	}
	dcel.faces = append(dcel.faces, infFace)

	var ghostFace *faceRecord // face for any ghost edges (populated lazily)

	for polyIdx := 0; polyIdx < mp.NumPolygons(); polyIdx++ {
		poly := mp.PolygonN(polyIdx)

		// Extract rings.
		rings := make([]Sequence, 1+poly.NumInteriorRings())
		rings[0] = poly.ExteriorRing().Coordinates()
		for i := 0; i < poly.NumInteriorRings(); i++ {
			rings[i+1] = poly.InteriorRingN(i).Coordinates()
		}

		// Populate vertices.
		for _, ring := range rings {
			for i := 0; i < ring.Length(); i++ {
				xy := ring.GetXY(i)
				if _, ok := dcel.vertices[xy]; !ok {
					dcel.vertices[xy] = &vertexRecord{xy, nil /* populated later */, mask}
				}
			}
		}

		polyFace := &faceRecord{
			outerComponent:  nil, // populated later
			innerComponents: nil, // populated later
			label:           mask,
		}
		dcel.faces = append(dcel.faces, polyFace)

		for ringIdx, ring := range rings {
			interiorFace := polyFace
			exteriorFace := infFace
			if ringIdx > 0 {
				holeFace := &faceRecord{
					outerComponent:  nil, // left nil
					innerComponents: nil, // populated later
					label:           populatedMask & mask,
				}
				// For inner rings, the exterior face is a hole rather than the
				// infinite face.
				exteriorFace = holeFace
				dcel.faces = append(dcel.faces, exteriorFace)
			}

			var newEdges []*halfEdgeRecord
			first := true
			for i := 0; i < ring.Length(); i++ {
				ln, ok := getLine(ring, i)
				if !ok {
					continue
				}
				vertA := dcel.vertices[ln.a]
				vertB := dcel.vertices[ln.b]
				internalEdge := &halfEdgeRecord{
					origin:   vertA,
					twin:     nil, // populated later
					incident: interiorFace,
					next:     nil, // populated later
					prev:     nil, // populated later
					label:    mask,
				}
				externalEdge := &halfEdgeRecord{
					origin:   vertB,
					twin:     internalEdge,
					incident: exteriorFace,
					next:     nil, // populated later
					prev:     nil, // populated later
					label:    mask,
				}
				internalEdge.twin = externalEdge
				vertA.incidents = append(vertA.incidents, internalEdge)
				vertB.incidents = append(vertB.incidents, externalEdge)
				newEdges = append(newEdges, internalEdge, externalEdge)

				// Set interior/exterior face linkage.
				if first {
					first = false
					if ringIdx == 0 {
						exteriorFace.innerComponents = append(exteriorFace.innerComponents, externalEdge)
						if interiorFace.outerComponent == nil {
							interiorFace.outerComponent = internalEdge
						}
					} else {
						interiorFace.innerComponents = append(interiorFace.innerComponents, internalEdge)
						if exteriorFace.outerComponent == nil {
							exteriorFace.outerComponent = externalEdge
						}
					}
				}
			}

			numEdges := len(newEdges)
			for i := 0; i < numEdges/2; i++ {
				newEdges[i*2].next = newEdges[(2*i+2)%numEdges]
				newEdges[i*2+1].next = newEdges[(i*2-1+numEdges)%numEdges]
				newEdges[i*2].prev = newEdges[(2*i-2+numEdges)%numEdges]
				newEdges[i*2+1].prev = newEdges[(2*i+3)%numEdges]
			}
			dcel.halfEdges = append(dcel.halfEdges, newEdges...)

			if ringIdx > 0 {
				// TODO: what if linkFrom and linkTo are the same?
				// TODO: What if the edge already exists?

				// Ghost edges are incident to the ghost face (which is created
				// lazily). If we don't have a ghost face yet, it's created
				// now.
				if ghostFace == nil {
					ghostFace = &faceRecord{
						outerComponent:  nil,
						innerComponents: nil,
						label:           mask & ^inSetMask,
					}
				}

				origin := dcel.vertices[rings[0].GetXY(0)]
				destin := dcel.vertices[ring.GetXY(0)]
				ghostFwd := &halfEdgeRecord{
					origin:   origin,
					twin:     nil, // populated below
					incident: ghostFace,
					next:     nil, // populated below
					prev:     nil, // populated below
					label:    mask & ^inSetMask,
				}
				ghostRev := &halfEdgeRecord{
					origin:   destin,
					twin:     ghostFwd,
					incident: ghostFace,
					next:     ghostFwd,
					prev:     ghostFwd,
					label:    mask & ^inSetMask,
				}
				origin.incidents = append(origin.incidents, ghostFwd)
				destin.incidents = append(destin.incidents, ghostRev)
				ghostFwd.twin = ghostRev
				ghostFwd.next = ghostRev
				ghostFwd.prev = ghostRev
				dcel.halfEdges = append(dcel.halfEdges, ghostFwd, ghostRev)
				// TODO: do we have to 'fix' the vertices?
			}
		}
	}

	// The ghost face is added as the final step so that it appears at the end
	// of the faces list (this is to help with testing).
	if ghostFace != nil {
		dcel.faces = append(dcel.faces, ghostFace)
	}

	return dcel
}

func newDCELFromMultiLineString(mls MultiLineString, mask uint8) *doublyConnectedEdgeList {
	dcel := &doublyConnectedEdgeList{
		vertices: make(map[XY]*vertexRecord),
	}

	// Add vertices.
	for i := 0; i < mls.NumLineStrings(); i++ {
		ls := mls.LineStringN(i)
		seq := ls.Coordinates()
		for j := 0; j < seq.Length(); j++ {
			xy := seq.GetXY(j)
			dcel.vertices[xy] = &vertexRecord{xy, nil /* populated later */, mask}
		}
	}

	// Linear elements have no face structure, so everything just points to the
	// infinite face.
	infFace := &faceRecord{
		outerComponent:  nil,
		innerComponents: nil,
		label:           mask & populatedMask,
	}
	dcel.faces = []*faceRecord{infFace}

	type vertPair struct {
		a, b *vertexRecord
	}
	edgeSet := make(map[vertPair]bool)

	// Add edges.
	for i := 0; i < mls.NumLineStrings(); i++ {
		seq := mls.LineStringN(i).Coordinates()
		for j := 0; j < seq.Length(); j++ {
			ln, ok := getLine(seq, j)
			if !ok {
				continue
			}

			vOrigin := dcel.vertices[ln.a]
			vDestin := dcel.vertices[ln.b]

			pair := vertPair{vOrigin, vDestin}
			if pair.a.coords.Less(pair.b.coords) {
				pair.a, pair.b = pair.b, pair.a
			}
			if edgeSet[pair] {
				continue
			}
			edgeSet[pair] = true

			fwd := &halfEdgeRecord{
				origin:   vOrigin,
				twin:     nil, // set later
				incident: infFace,
				next:     nil, // set later
				prev:     nil, // set later
				label:    mask,
			}
			rev := &halfEdgeRecord{
				origin:   vDestin,
				twin:     fwd,
				incident: infFace,
				next:     fwd,
				prev:     fwd,
				label:    mask,
			}
			fwd.twin = rev
			fwd.next = rev
			fwd.prev = rev

			vOrigin.incidents = append(vOrigin.incidents, fwd)
			vDestin.incidents = append(vDestin.incidents, rev)

			dcel.halfEdges = append(dcel.halfEdges, fwd, rev)
			infFace.innerComponents = append(infFace.innerComponents, fwd)
		}
	}

	return dcel
}

func newDCELFromMultiPoint(mp MultiPoint, mask uint8) *doublyConnectedEdgeList {
	dcel := &doublyConnectedEdgeList{vertices: make(map[XY]*vertexRecord)}
	n := mp.NumPoints()
	for i := 0; i < n; i++ {
		xy, ok := mp.PointN(i).XY()
		if !ok {
			continue
		}
		record, ok := dcel.vertices[xy]
		if !ok {
			record = &vertexRecord{
				coords:    xy,
				incidents: nil,
				label:     0,
			}
			dcel.vertices[xy] = record
		}
		record.label |= mask
	}
	return dcel
}
