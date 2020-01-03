package main

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/peterstace/simplefeatures/geom"
)

func CheckWKTParse(t *testing.T, pg PostGIS, candidates []string) {
	var any bool
	for i, wkt := range candidates {
		any = true
		t.Run(fmt.Sprintf("CheckWKTParse_%d", i), func(t *testing.T) {

			// The simple feature library accepts LINEARRING WKTs. However,
			// postgis doesn't accept them. A workaround for this is to just
			// substitute LINEARRING for LINESTRING. However, this will give a
			// false negative if the corpus contains a LINEARRING WKT that
			// isn't closed (and thus won't be accepted by simple features).
			wkt := strings.ReplaceAll(wkt, "LINEARRING", "LINESTRING")

			_, sfErr := geom.UnmarshalWKT(strings.NewReader(wkt))
			isValid, reason := pg.WKTIsValidWithReason(wkt)
			if (sfErr == nil) != isValid {
				t.Logf("SimpleFeatures err: %v", sfErr)
				t.Logf("PostGIS IsValid: %v", isValid)
				t.Logf("PostGIS Reason: %v", reason)
				t.Errorf("mismatch")
			}
		})
	}
	if !any {
		// We know there are some some valid WKT strings, so if this happens
		// then something is wrong with the extraction or conversion logic.
		t.Errorf("could not extract any WKTs")
	}
}

func CheckWKBParse(t *testing.T, pg PostGIS, candidates []string) {
	var any bool
	for i, wkb := range candidates {
		buf, err := hexStringToBytes(wkb)
		if err != nil {
			continue
		}
		any = true
		t.Run(fmt.Sprintf("CheckWKBParse_%d", i), func(t *testing.T) {
			_, sfErr := geom.UnmarshalWKB(bytes.NewReader(buf))
			isValid, reason := pg.WKBIsValidWithReason(t, wkb)
			if (sfErr == nil) != isValid {
				t.Logf("WKB: %v", wkb)
				t.Logf("SimpleFeatures err: %v", sfErr)
				t.Logf("PostGIS IsValid: %v", isValid)
				t.Logf("PostGIS Reason: %v", reason)
				t.Errorf("mismatch")
			}
		})
	}
	if !any {
		// We know there are some some valid hex strings, so if this happens
		// then something is wrong with the extraction or conversion logic.
		t.Errorf("could not extract any WKBs")
	}
}

func hexStringToBytes(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, errors.New("hex string must have even length")
	}
	var buf []byte
	for i := 0; i < len(s); i += 2 {
		x, err := strconv.ParseUint(s[i:i+2], 16, 8)
		if err != nil {
			return nil, err
		}
		buf = append(buf, byte(x))
	}
	return buf, nil
}

func CheckGeoJSONParse(t *testing.T, pg PostGIS, candidates []string) {
	var any bool
	for i, geojson := range candidates {
		if geojson == `{"type":"Point","coordinates":[]}` {
			// From https://tools.ietf.org/html/rfc7946#section-3.1:
			//
			// > GeoJSON processors MAY interpret Geometry objects with
			// > empty "coordinates" arrays as null objects.
			//
			// Simplefeatures chooses to accept this as an empty point, but
			// Postgres rejects it.
			continue
		}
		any = true
		t.Run(fmt.Sprintf("CheckGeoJSONParse_%d", i), func(t *testing.T) {
			_, sfErr := geom.UnmarshalGeoJSON([]byte(geojson))
			isValid, reason := pg.GeoJSONIsValidWithReason(t, geojson)
			if (sfErr == nil) != isValid {
				t.Logf("GeoJSON: %v", geojson)
				t.Logf("SimpleFeatures err: %v", sfErr)
				t.Logf("PostGIS IsValid: %v", isValid)
				t.Logf("PostGIS Reason: %v", reason)
				t.Errorf("mismatch")
			}
		})
	}
	if !any {
		// We know there are some some valid geojson strings, so if this happens
		// then something is wrong with the extraction or conversion logic.
		t.Errorf("could not extract any geojsons")
	}
}

func CheckWKT(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckWKT", func(t *testing.T) {
		got := g.AsText()
		if strings.Contains(got, "MULTIPOINT") {
			// Skip Multipoints. This is because Postgis doesn't follow the SFA
			// spec by not including parenthesis around each individual point.
			// The simplefeatures library follows the spec correctly.
			return
		}
		want := pg.AsText(t, g)
		if want != got {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckWKB(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckWKB", func(t *testing.T) {
		if g.IsEmptySet() && g.AsText() == "POINT EMPTY" {
			// Empty point WKB use NaN as part of their representation.
			// Go's math.NaN() and Postgis use slightly different (but
			// compatible) representations of NaN.
			return
		}
		// TODO: Use g.IsEmpty() instead of g.AsGeometryCollection().IsEmpty()
		if g.IsGeometryCollection() && g.AsGeometryCollection().IsEmpty() {
			// The behaviour for GeometryCollections in Postgis is to just
			// give 'GEOMETRYCOLLECTION EMPTY' whenever the contents of a
			// geometry collection are all empty geometries. This doesn't
			// seem like correct behaviour, so these cases are skipped.
			return
		}
		var got bytes.Buffer
		if err := g.AsBinary(&got); err != nil {
			t.Fatalf("writing wkb: %v", err)
		}
		want := pg.AsBinary(t, g)
		if !bytes.Equal(got.Bytes(), want) {
			t.Logf("got:  %v", got.Bytes())
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckGeoJSON(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckGeoJSON", func(t *testing.T) {

		// PostGIS cannot convert to geojson in the case where it has
		// nested geometry collections. That seems to be based on the
		// following section of https://tools.ietf.org/html/rfc7946:
		//
		// To maximize interoperability, implementations SHOULD avoid
		// nested GeometryCollections.  Furthermore, GeometryCollections
		// composed of a single part or a number of parts of a single type
		// SHOULD be avoided when that single part or a single object of
		// multipart type (MultiPoint, MultiLineString, or MultiPolygon)
		// could be used instead.
		if g.IsGeometryCollection() {
			gc := g.AsGeometryCollection()
			for i := 0; i < gc.NumGeometries(); i++ {
				if gc.GeometryN(i).IsGeometryCollection() {
					return
				}
			}
		}

		got, err := g.MarshalJSON()
		if err != nil {
			t.Fatalf("could not convert to geojson: %v", err)
		}
		want := pg.AsGeoJSON(t, g)
		if !bytes.Equal(got, want) {
			t.Logf("got:  %v", string(got))
			t.Logf("want: %v", string(want))
			t.Error("mismatch")
		}
	})
}

func CheckIsEmpty(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckIsEmpty", func(t *testing.T) {
		got := g.IsEmpty()
		want := pg.IsEmpty(t, g)
		if got != want {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckDimension(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckDimension", func(t *testing.T) {
		got := g.Dimension()
		want := pg.Dimension(t, g)
		if got != want {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckEnvelope(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckEnvelope", func(t *testing.T) {
		if g.IsEmpty() {
			// PostGIS allows envelopes on empty geometries, but they are empty
			// envelopes. In simplefeatures, an envelope is never empty, so we
			// skip testing that case.
			return
		}
		env, ok := g.Envelope()
		if !ok {
			// We just checked IsEmpty, so this should never happen.
			panic("could not get envelope")
		}
		got := env.AsGeometry()
		want := pg.Envelope(t, g)

		if !got.EqualsExact(want) {
			t.Logf("got:  %v", got.AsText())
			t.Logf("want: %v", want.AsText())
			t.Error("mismatch")
		}
	})
}

func CheckIsSimple(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckIsSimple", func(t *testing.T) {
		var got bool
		switch {
		case g.IsGeometryCollection():
			return // GeometryCollection doesn't have IsSimple
		case g.IsEmptySet():
			got = g.AsEmptySet().IsSimple()
		case g.IsPoint():
			got = g.AsPoint().IsSimple()
		case g.IsLine():
			got = g.AsLine().IsSimple()
		case g.IsLineString():
			got = g.AsLineString().IsSimple()
		case g.IsPolygon():
			got = g.AsPolygon().IsSimple()
		case g.IsMultiPoint():
			got = g.AsMultiPoint().IsSimple()
		case g.IsMultiLineString():
			got = g.AsMultiLineString().IsSimple()
		case g.IsMultiPolygon():
			got = g.AsMultiPolygon().IsSimple()
		default:
			panic(g)
		}

		// PostGIS doesn't treat MultiLineStrings containing duplicated
		// LineStrings as non-simple, e.g. MULTILINESTRING((0 0,1 1),(0 0,1
		// 1)). This doesn't seem like correct behaviour to me. It must be
		// deduplicating the LineStrings before checking simplicity. This
		// library doesn't do that, so skip any LineStrings that contain
		// duplicates.
		if g.IsMultiLineString() {
			mls := g.AsMultiLineString()
			n := mls.NumLineStrings()
			for i := 0; i < n; i++ {
				for j := 0; j < n; j++ {
					if mls.LineStringN(i).EqualsExact(mls.LineStringN(j).AsGeometry()) {
						return
					}
				}
			}
		}

		want := pg.IsSimple(t, g)
		if got != want {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckBoundary(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckBoundary", func(t *testing.T) {
		if g.IsGeometryCollection() {
			// PostGIS cannot calculate the boundary of GeometryCollections.
			// Some other libraries can, so simplefeatures does as well.
			return
		}
		got := g.Boundary()
		want := pg.Boundary(t, g)
		if !got.EqualsExact(want, geom.IgnoreOrder) {
			t.Logf("got:  %v", got.AsText())
			t.Logf("want: %v", want.AsText())
			t.Error("mismatch")
		}
	})
}

func CheckConvexHull(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckConvexHull", func(t *testing.T) {
		got := g.ConvexHull()
		want := pg.ConvexHull(t, g)
		if !got.EqualsExact(want, geom.IgnoreOrder) {
			t.Logf("got:  %v", got.AsText())
			t.Logf("want: %v", want.AsText())
			t.Error("mismatch")
		}
	})
}

func CheckIsValid(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckIsValid", func(t *testing.T) {
		got := g.IsValid()
		want := pg.IsValid(t, g)
		if got != want {
			t.Logf("got:  %t", got)
			t.Logf("want: %t", want)
			t.Error("mismatch")
		}
	})
}

func CheckIsRing(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckIsRing", func(t *testing.T) {
		var got bool
		if g.IsLineString() {
			got = g.AsLineString().IsRing()
		}
		want := pg.IsRing(t, g)
		if got != want {
			t.Logf("got:  %t", got)
			t.Logf("want: %t", want)
			t.Error("mismatch")
		}
	})
}

func CheckLength(t *testing.T, pg PostGIS, g geom.Geometry) {
	if !g.IsLine() && !g.IsLineString() && !g.IsMultiLineString() {
		if _, ok := g.Length(); ok {
			t.Error("didn't expect to be able to get length but could")
		}
		return
	}
	t.Run("CheckLength", func(t *testing.T) {
		got, ok := g.Length()
		if !ok {
			t.Error("could not get length")
		}
		want := pg.Length(t, g)
		if math.Abs(got-want) > 1e-6 {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckEqualsExact(t *testing.T, pg PostGIS, g1, g2 geom.Geometry) {
	t.Run("CheckEqualsExact", func(t *testing.T) {
		got := g1.EqualsExact(g2)
		want := pg.OrderingEquals(t, g1, g2)
		if got != want {
			t.Logf("got:  %t", got)
			t.Logf("want: %t", want)
			t.Error("mismatch")
		}
	})
}

func CheckEquals(t *testing.T, pg PostGIS, g1, g2 geom.Geometry) {
	t.Run("CheckEquals", func(t *testing.T) {
		if g1.IsGeometryCollection() || g2.IsGeometryCollection() {
			// PostGIS cannot calculate Equals for geometry collections.
			return
		}
		got, err := g1.Equals(g2)
		if err != nil {
			return // operation not implemented
		}
		want := pg.Equals(t, g1, g2)
		if got != want {
			t.Logf("got:  %t", got)
			t.Logf("want: %t", want)
			t.Error("mismatch")
		}
	})
}

func CheckIntersects(t *testing.T, pg PostGIS, g1, g2 geom.Geometry) {
	t.Run("CheckIntersects", func(t *testing.T) {
		got := g1.Intersects(g2)
		want := pg.Intersects(t, g1, g2)
		if got != want {
			t.Logf("got:  %t", got)
			t.Logf("want: %t", want)
			t.Error("mismatch")
		}
	})
}

func CheckIntersection(t *testing.T, pg PostGIS, g1, g2 geom.Geometry) {
	t.Run("CheckIntersection", func(t *testing.T) {
		got, err := g1.Intersection(g2)
		if err != nil {
			return // operation not implemented
		}
		want := pg.Intersection(t, g1, g2)

		if got.IsEmpty() && want.IsEmpty() {
			return // Both empty, so they match.
		}

		if got.IsGeometryCollection() || want.IsGeometryCollection() {
			// GeometryCollections are not supported by ST_Equals. So there's
			// not much that we can do here.
			return
		}

		// PostGIS TolerantEquals (a chain of ST_SnapToGrid and ST_Equals) is
		// used rather than in memory ExactEquals because simplefeatures does
		// not implement intersect in exactly the same way as PostGIS.

		if !pg.TolerantEquals(t, got, want) {
			t.Logf("g1:   %s", g1.AsText())
			t.Logf("g2:   %s", g2.AsText())
			t.Logf("got:  %s", got.AsText())
			t.Logf("want: %s", want.AsText())
			t.Error("mismatch")
		}
	})
}

func CheckArea(t *testing.T, pg PostGIS, g geom.Geometry) {
	if !g.IsPolygon() && !g.IsMultiPolygon() {
		return
	}
	t.Run("CheckArea", func(t *testing.T) {
		var got float64
		switch {
		case g.IsPolygon():
			got = g.AsPolygon().Area()
		case g.IsMultiPolygon():
			got = g.AsMultiPolygon().Area()
		default:
			panic(g)
		}
		want := pg.Area(t, g)
		const eps = 0.000000001
		if math.Abs(got-want) > eps {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckCentroid(t *testing.T, pg PostGIS, g geom.Geometry) {
	t.Run("CheckCentroid", func(t *testing.T) {
		var got geom.Point
		var empty bool
		switch {
		case g.IsPolygon():
			got = g.AsPolygon().Centroid()
		case g.IsMultiPolygon():
			var ok bool
			got, ok = g.AsMultiPolygon().Centroid()
			empty = !ok
		default:
			return
		}
		want := pg.Centroid(t, g)

		if empty {
			if !want.IsEmpty() {
				t.Log("got:  empty", got)
				t.Logf("want: %v", want)
				t.Error("mismatch")
			}
		} else {
			if !got.EqualsExact(want, geom.Tolerance(0.000000001)) {
				t.Logf("got:  %v", got)
				t.Logf("want: %v", want)
				t.Error("mismatch")
			}
		}
	})
}
