package geom

type doublyConnectedEdgeList struct {
	faces []*faceRecord
}

type faceRecord struct {
	outerComponent  *halfEdgeRecord
	innerComponents []*halfEdgeRecord
}

type halfEdgeRecord struct {
	origin       *vertexRecord
	twin         *halfEdgeRecord
	incidentFace *faceRecord
	next         *halfEdgeRecord
	prev         *halfEdgeRecord
}

type vertexRecord struct {
	coords       XY
	incidentEdge *halfEdgeRecord
}

func fromPolygon(p Polygon) *doublyConnectedEdgeList {
	verts := make(map[XY]*vertexRecord)

	addVerts := func(ring LineString) {
		seq := ring.Coordinates()
		n := seq.Length()
		for i := 0; i < n; i++ {
			xy := seq.GetXY(i)
			if _, ok := verts[xy]; !ok {
				verts[xy] = &vertexRecord{
					coords:       xy,
					incidentEdge: nil, // added later
				}
			}
		}
	}

	addVerts(p.ExteriorRing())
	for i := 0; i < p.NumInteriorRings(); i++ {
		addVerts(p.InteriorRingN(i))
	}
}
