package generate

import (
	"math"

	"github.com/peterstace/simplefeatures/geom"
)

// RegularPolygon computes a regular polygon circumscribed by a circle with the
// given center and radius. Sides must be at least 3 or it will panic.
func RegularPolygon(center geom.XY, radius float64, sides int) geom.Polygon {
	if sides <= 2 {
		panic(sides)
	}
	coords := make([]geom.Coordinates, sides+1)
	for i := 0; i < sides; i++ {
		angle := math.Pi/2 + float64(i)/float64(sides)*2*math.Pi
		coords[i] = geom.Coordinates{XY: geom.XY{
			X: center.X + math.Cos(angle)*radius,
			Y: center.Y + math.Sin(angle)*radius,
		}}
	}
	coords[sides] = coords[0]
	poly, err := geom.NewPolygonC([][]geom.Coordinates{coords})
	if err != nil {
		panic(err)
	}
	return poly
}
