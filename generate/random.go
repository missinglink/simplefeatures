package generate

import (
	"math/rand"
	"sort"
	"strings"

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

func RandomPointWKT(rnd *rand.Rand) string {
	return geom.NewPointXY(
		RandomXYOnGrid(rnd, 0, 10),
		geom.DisableAllValidations,
	).AsText()
}

func RandomLineWKT(rnd *rand.Rand) string {
	ln, err := geom.NewLineXY(
		RandomXYOnGrid(rnd, 0, 10),
		RandomXYOnGrid(rnd, 0, 10),
		geom.DisableAllValidations,
	)
	if err != nil {
		panic(err)
	}
	return ln.AsText()
}

func RandomLineStringWKT(rnd *rand.Rand) string {
	last := geom.XY{
		X: float64(rnd.Intn(100) - 50),
		Y: float64(rnd.Intn(100) - 50),
	}
	coords := []geom.XY{last}
	for {
		if rnd.Float64() < 0.1 {
			ls, _ := geom.NewLineStringXY(coords, geom.DisableAllValidations)
			return ls.AsText()
		}
		last = last.Add(geom.XY{
			X: float64(rnd.Intn(10) - 5),
			Y: float64(rnd.Intn(10) - 5),
		})
		coords = append(coords, last)
	}
}

func WKTIsValidGeometry(wkt string) bool {
	_, err := geom.UnmarshalWKT(strings.NewReader(wkt))
	return err == nil
}

func WKTIsInvalidGeometry(wkt string) bool {
	return !WKTIsValidGeometry(wkt)
}

func AlwaysTrue(wkt string) bool {
	return true
}

type WeightedPredicate struct {
	Weight    float64
	Predicate func(wkt string) bool
}

func ForceDistribution(rnd *rand.Rand, wktGenerator func(*rand.Rand) string, predicates []WeightedPredicate) string {
	cumulative := make([]float64, len(predicates))
	for i, wp := range predicates {
		cumulative[i] = wp.Weight
		if i != 0 {
			cumulative[i] += cumulative[i-1]
		}
	}
	idx := sort.SearchFloat64s(
		cumulative,
		rnd.Float64()*cumulative[len(cumulative)-1],
	)
	for {
		if wkt := wktGenerator(rnd); predicates[idx].Predicate(wkt) {
			return wkt
		}
	}
}
