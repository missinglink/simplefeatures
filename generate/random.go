package generate

import (
	"math/rand"

	"github.com/peterstace/simplefeatures/geom"
)

/*
func ValidityMix(rnd *rand.Rand, invalidRatio float64, gen WKTGenerator) WKTGenerator {
	return WKTGenerator(func() string {
		wantValid := rand.Float64() > invalidRatio
		for {
			wkt := gen()
			_, err := geom.UnmarshalWKT(strings.NewReader(wkt))
			if (err == nil) == wantValid {
				return wkt
			}
		}
	})
}
*/

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
