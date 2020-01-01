package geom

import (
	"database/sql/driver"
	"io"
	"unsafe"
)

// MultiPoint is a 0-dimensional geometric collection of points. The points are
// not connected or ordered.
//
// Its assertions are:
//
// 1. It must be made up of 0 or more valid Points.
type MultiPoint struct {
	pts []Point
}

func NewMultiPoint(pts []Point, opts ...ConstructorOption) MultiPoint {
	return MultiPoint{pts}
}

// NewMultiPointOC creates a new MultiPoint consisting of a Point for each
// non-empty OptionalCoordinate.
func NewMultiPointOC(coords []OptionalCoordinates, opts ...ConstructorOption) MultiPoint {
	var pts []Point
	for _, c := range coords {
		if c.Empty {
			continue
		}
		pt := NewPointC(c.Value, opts...)
		pts = append(pts, pt)
	}
	return NewMultiPoint(pts, opts...)
}

// NewMultiPointC creates a new MultiPoint consisting of a point for each coordinate.
func NewMultiPointC(coords []Coordinates, opts ...ConstructorOption) MultiPoint {
	var pts []Point
	for _, c := range coords {
		pt := NewPointC(c, opts...)
		pts = append(pts, pt)
	}
	return NewMultiPoint(pts, opts...)
}

// NewMultiPointXY creates a new MultiPoint consisting of a point for each XY.
func NewMultiPointXY(pts []XY, opts ...ConstructorOption) MultiPoint {
	return NewMultiPointC(oneDimXYToCoords(pts))
}

// AsGeometry converts this MultiPoint into a Geometry.
func (m MultiPoint) AsGeometry() Geometry {
	return Geometry{multiPointTag, unsafe.Pointer(&m)}
}

// NumPoints gives the number of element points making up the MultiPoint.
func (m MultiPoint) NumPoints() int {
	return len(m.pts)
}

// PointN gives the nth (zero indexed) Point.
func (m MultiPoint) PointN(n int) Point {
	return m.pts[n]
}

func (m MultiPoint) AsText() string {
	return string(m.AppendWKT(nil))
}

func (m MultiPoint) AppendWKT(dst []byte) []byte {
	dst = append(dst, []byte("MULTIPOINT")...)
	if len(m.pts) == 0 {
		return append(dst, []byte(" EMPTY")...)
	}
	dst = append(dst, '(')
	for i, pt := range m.pts {
		dst = pt.appendWKTBody(dst)
		if i != len(m.pts)-1 {
			dst = append(dst, ',')
		}
	}
	return append(dst, ')')
}

// IsSimple returns true iff no two of its points are equal.
func (m MultiPoint) IsSimple() bool {
	seen := make(map[XY]bool)
	for _, p := range m.pts {
		if seen[p.coords.XY] {
			return false
		}
		seen[p.coords.XY] = true
	}
	return true
}

func (m MultiPoint) Intersection(g GeometryX) (GeometryX, error) {
	return intersection(m, g)
}

func (m MultiPoint) Intersects(g GeometryX) bool {
	return hasIntersection(m, g)
}

func (m MultiPoint) IsEmpty() bool {
	return len(m.pts) == 0
}

func (m MultiPoint) Dimension() int {
	return 0
}

func (m MultiPoint) Equals(other GeometryX) (bool, error) {
	return equals(m, other)
}

func (m MultiPoint) Envelope() (Envelope, bool) {
	if len(m.pts) == 0 {
		return Envelope{}, false
	}
	env := NewEnvelope(m.pts[0].coords.XY)
	for _, pt := range m.pts[1:] {
		env = env.ExtendToIncludePoint(pt.coords.XY)
	}
	return env, true
}

func (m MultiPoint) Boundary() GeometryX {
	// This is a little bit more complicated than it really has to be (it just
	// has to always return an empty set). However, this is the behavour of
	// Postgis.
	if m.IsEmpty() {
		return m
	}
	return NewGeometryCollection(nil)
}

func (m MultiPoint) Value() (driver.Value, error) {
	return wkbAsBytes(m)
}

func (m MultiPoint) AsBinary(w io.Writer) error {
	marsh := newWKBMarshaller(w)
	marsh.writeByteOrder()
	marsh.writeGeomType(wkbGeomTypeMultiPoint)
	n := m.NumPoints()
	marsh.writeCount(n)
	for i := 0; i < n; i++ {
		pt := m.PointN(i)
		marsh.setErr(pt.AsBinary(w))
	}
	return marsh.err
}

// ConvexHull finds the convex hull of the set of points. This may either be
// the empty set, a single point, a line, or a polygon.
func (m MultiPoint) ConvexHull() GeometryX {
	return convexHull(m)
}

func (m MultiPoint) convexHullPointSet() []XY {
	n := m.NumPoints()
	points := make([]XY, n)
	for i := 0; i < n; i++ {
		points[i] = m.PointN(i).XY()
	}
	return points
}

func (m MultiPoint) MarshalJSON() ([]byte, error) {
	return marshalGeoJSON("MultiPoint", m.Coordinates())
}

// Coordinates returns the coordinates of the points represented by the
// MultiPoint.
func (m MultiPoint) Coordinates() []Coordinates {
	coords := make([]Coordinates, len(m.pts))
	for i := range coords {
		coords[i] = m.pts[i].Coordinates()
	}
	return coords
}

// TransformXY transforms this MultiPoint into another MultiPoint according to fn.
func (m MultiPoint) TransformXY(fn func(XY) XY, opts ...ConstructorOption) (GeometryX, error) {
	coords := m.Coordinates()
	transform1dCoords(coords, fn)
	return NewMultiPointC(coords, opts...), nil
}

// EqualsExact checks if this MultiPoint is exactly equal to another MultiPoint.
func (m MultiPoint) EqualsExact(other GeometryX, opts ...EqualsExactOption) bool {
	o, ok := other.(MultiPoint)
	return ok && multiPointExactEqual(m, o, opts)
}

// IsValid checks if this MultiPoint is valid. However, there is no way to indicate
// whether or not MultiPoint is valid, so this function always returns true
func (m MultiPoint) IsValid() bool {
	return true
}
