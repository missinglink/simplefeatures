package generate

import (
	"math/rand"

	"github.com/peterstace/simplefeatures/geom"
)

func RandomXYOnGrid(rnd *rand.Rand, min, max int) geom.XY {
	x := rnd.Intn(max-min) + min
	y := rnd.Intn(max-min) + min
	return geom.XY{
		X: float64(x),
		Y: float64(y),
	}
}

func RandomPoint(rnd *rand.Rand) geom.Point {
	xy := RandomXYOnGrid(rnd, 0, 10)
	return geom.NewPointXY(xy)
}

func RandomLine(rnd *rand.Rand) geom.Line {
	for {
		ln, err := geom.NewLineXY(
			RandomXYOnGrid(rnd, 0, 10),
			RandomXYOnGrid(rnd, 0, 10),
		)
		if err == nil {
			return ln
		}
	}
}

type LineStringSpec struct {
	NumPoints int
	IsClosed  bool
	IsSimple  bool
}

func RandomLineStringRandomWalk(rnd *rand.Rand, spec LineStringSpec) geom.LineString {
	if spec.IsClosed {
		spec.NumPoints--
	}
	for {
		last := geom.XY{
			X: float64(rnd.Intn(100) - 50),
			Y: float64(rnd.Intn(100) - 50),
		}
		var coords []geom.XY
		for i := 0; i < spec.NumPoints; i++ {
			coords = append(coords, last)
			last = last.Add(geom.XY{
				X: float64(rnd.Intn(7) - 3),
				Y: float64(rnd.Intn(7) - 3),
			})
		}
		if spec.IsClosed {
			coords = append(coords, coords[0])
		}
		ls, err := geom.NewLineStringXY(coords)
		if err == nil &&
			ls.IsSimple() == spec.IsSimple &&
			ls.IsClosed() == spec.IsClosed {
			return ls
		}
	}
}

type PolygonSpec struct {
	Valid      bool
	RingPoints []int
}

// TODO: This is pretty slow even for modest sized polygons. Idea: create a
// circle, then distort it using perlin noise. We can then choose a center and
// radius of the circle that makes it easier for the RNG to build a valid
// polygon.

func RandomPolygon(rnd *rand.Rand, spec PolygonSpec) string {
	if len(spec.RingPoints) == 0 {
		panic("bad spec: polygon must have at least 1 ring")
	}
	for {
		rings := make([]geom.LineString, len(spec.RingPoints))
		for i, pts := range spec.RingPoints {
			rings[i] = RandomLineStringRandomWalk(rnd, LineStringSpec{
				NumPoints: pts,
				IsClosed:  true,
				IsSimple:  true,
			})
		}
		poly, err := geom.NewPolygon(rings[0], rings[1:])
		if err == nil && spec.Valid {
			return poly.AsText()
		} else if err != nil && !spec.Valid {
			poly, _ = geom.NewPolygon(rings[0], rings[1:], geom.DisableAllValidations)
			return poly.AsText()
		}
	}
}
