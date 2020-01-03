package geom

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"io"
	"unsafe"
)

// LineString is a curve defined by linear interpolation between a finite set
// of points. Each consecutive pair of points defines a line segment.
//
// Its assertions are:
//
// 1. It must contain at least 2 distinct points.
type LineString struct {
	lines []Line
}

// NewLineStringC creates a line string from the coordinates defining its
// points.
func NewLineStringC(pts []Coordinates, opts ...ConstructorOption) (LineString, error) {
	var lines []Line
	for i := 0; i < len(pts)-1; i++ {
		if pts[i].XY.Equals(pts[i+1].XY) {
			continue
		}
		ln := must(NewLineC(pts[i], pts[i+1], opts...)).(Line)
		lines = append(lines, ln)
	}
	if doCheapValidations(opts) && len(lines) == 0 {
		return LineString{}, errors.New("LineString must contain at least two distinct points")
	}
	return LineString{lines}, nil
}

// NewLineStringXY creates a line string from the XYs defining its points.
func NewLineStringXY(pts []XY, opts ...ConstructorOption) (LineString, error) {
	return NewLineStringC(oneDimXYToCoords(pts), opts...)
}

// AsGeometry converts this LineString into a Geometry.
func (s LineString) AsGeometry() Geometry {
	return Geometry{lineStringTag, unsafe.Pointer(&s)}
}

// StartPoint gives the first point of the line string.
func (s LineString) StartPoint() Point {
	return s.lines[0].StartPoint()
}

// EndPoint gives the last point of the line string.
func (s LineString) EndPoint() Point {
	return s.lines[len(s.lines)-1].EndPoint()
}

// NumPoints gives the number of control points in the line string.
func (s LineString) NumPoints() int {
	return len(s.lines) + 1
}

// PointN gives the nth (zero indexed) point in the line string. Panics if n is
// out of range with respect to the number of points.
func (s LineString) PointN(n int) Point {
	if n == s.NumPoints()-1 {
		return s.EndPoint()
	}
	return s.lines[n].StartPoint()
}

func (s LineString) AsText() string {
	return string(s.AppendWKT(nil))
}

func (s LineString) AppendWKT(dst []byte) []byte {
	dst = append(dst, []byte("LINESTRING")...)
	return s.appendWKTBody(dst)
}

func (s LineString) appendWKTBody(dst []byte) []byte {
	dst = append(dst, '(')
	for _, ln := range s.lines {
		dst = appendFloat(dst, ln.a.X)
		dst = append(dst, ' ')
		dst = appendFloat(dst, ln.a.Y)
		dst = append(dst, ',')
	}
	last := s.lines[len(s.lines)-1].b
	dst = appendFloat(dst, last.X)
	dst = append(dst, ' ')
	dst = appendFloat(dst, last.Y)
	return append(dst, ')')
}

// IsSimple returns true iff the curve defined by the LineString doesn't pass
// through the same point twice (with the possible exception of the two
// endpoints being coincident).
func (s LineString) IsSimple() bool {
	// 1. Check for pairwise intersection.
	//  a. Point is allowed if lines adjacent.
	//  b. Start to end is allowed if first and last line.
	n := len(s.lines)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			intersection := ToGeometry(mustIntersection(s.lines[i], s.lines[j]))
			if intersection.IsEmpty() {
				continue
			}
			if intersection.Dimension() >= 1 {
				// two overlapping line segments
				return false
			}
			// The intersection must be a single point.
			if i+1 == j {
				// Adjacent lines will intersect at a point due to
				// construction, so this case is okay.
				continue
			}
			if i == 0 && j == n-1 {
				// The first and last segment are allowed to intersect at a
				// point, so long as that point is the start of the first
				// segment and the end of the last segment (i.e. a linear
				// ring).
				aPt := NewPointC(s.lines[i].a).AsGeometry()
				bPt := NewPointC(s.lines[j].b).AsGeometry()
				if !intersection.EqualsExact(aPt) || !intersection.EqualsExact(bPt) {
					return false
				}
			} else {
				// Any other point intersection (e.g. looping back on
				// itself at a point) is disallowed for simple linestrings.
				return false
			}
		}
	}
	return true
}

func (s LineString) IsClosed() bool {
	return s.lines[0].a.XY.Equals(s.lines[len(s.lines)-1].b.XY)
}

func (s LineString) Intersection(g GeometryX) (GeometryX, error) {
	return intersection(s, g)
}

func (s LineString) Intersects(g Geometry) bool {
	return hasIntersection(s.AsGeometry(), g)
}

func (s LineString) IsEmpty() bool {
	return false
}

func (s LineString) Equals(other GeometryX) (bool, error) {
	return equals(s, other)
}

func (s LineString) Envelope() (Envelope, bool) {
	env := NewEnvelope(s.lines[0].a.XY)
	for _, line := range s.lines {
		env = env.ExtendToIncludePoint(line.b.XY)
	}
	return env, true
}

func (s LineString) Boundary() Geometry {
	if s.IsClosed() {
		// Same behaviour as Postgis, but could instead be any other empty set.
		return NewMultiPoint(nil).AsGeometry()
	}
	return NewMultiPoint([]Point{
		NewPointXY(s.lines[0].a.XY),
		NewPointXY(s.lines[len(s.lines)-1].b.XY),
	}).AsGeometry()
}

func (s LineString) Value() (driver.Value, error) {
	var buf bytes.Buffer
	err := s.AsBinary(&buf)
	return buf.Bytes(), err
}

func (s LineString) AsBinary(w io.Writer) error {
	marsh := newWKBMarshaller(w)
	marsh.writeByteOrder()
	marsh.writeGeomType(wkbGeomTypeLineString)
	n := s.NumPoints()
	marsh.writeCount(n)
	for i := 0; i < n; i++ {
		marsh.writeFloat64(s.PointN(i).XY().X)
		marsh.writeFloat64(s.PointN(i).XY().Y)
	}
	return marsh.err
}

func (s LineString) ConvexHull() Geometry {
	return convexHull(s.AsGeometry())
}

func (s LineString) MarshalJSON() ([]byte, error) {
	return marshalGeoJSON("LineString", s.Coordinates())
}

// Coordinates returns the coordinates of each point along the LineString.
func (s LineString) Coordinates() []Coordinates {
	n := s.NumPoints()
	coords := make([]Coordinates, n)
	for i := range coords {
		coords[i] = s.PointN(i).Coordinates()
	}
	return coords
}

// TransformXY transforms this LineString into another LineString according to fn.
func (s LineString) TransformXY(fn func(XY) XY, opts ...ConstructorOption) (GeometryX, error) {
	coords := s.Coordinates()
	transform1dCoords(coords, fn)
	return NewLineStringC(coords, opts...)
}

// EqualsExact checks if this LineString is exactly equal to another curve.
func (s LineString) EqualsExact(other Geometry, opts ...EqualsExactOption) bool {
	var c curve
	switch {
	case other.IsLine():
		c = other.AsLine()
	case other.IsLineString():
		c = other.AsLineString()
	default:
		return false
	}
	return curvesExactEqual(s, c, opts)
}

// IsValid checks if this LineString is valid
func (s LineString) IsValid() bool {
	_, err := NewLineStringC(s.Coordinates())
	return err == nil
}

// IsRing returns true iff this LineString is both simple and closed (i.e. is a
// linear ring).
func (s LineString) IsRing() bool {
	return s.IsSimple() && s.IsClosed()
}

// Length gives the length of the line string.
func (s LineString) Length() float64 {
	var sum float64
	for _, ln := range s.lines {
		sum += ln.Length()
	}
	return sum
}

// AsMultiLineString is a convinience function that converts this LineString
// into a MultiLineString.
func (s LineString) AsMultiLineString() MultiLineString {
	return NewMultiLineString([]LineString{s})
}
