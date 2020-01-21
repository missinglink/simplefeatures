package generate

import (
	"math"
	"math/rand"

	"github.com/peterstace/simplefeatures/geom"
)

type PerlinGenerator struct {
	env       geom.Envelope
	gradients [][]geom.XY
}

// NewPerlinGenerator constructs a perlin generator, rounding the envelope
// bounds to their nearest integers.
func NewPerlinGenerator(env geom.Envelope, rnd *rand.Rand) PerlinGenerator {
	roundedEnv := geom.NewEnvelope(
		geom.XY{X: math.Round(env.Min().X), Y: math.Round(env.Min().Y)},
		geom.XY{X: math.Round(env.Max().X), Y: math.Round(env.Max().Y)},
	)
	if roundedEnv.Area() == 0 {
		panic("envelope has no area")
	}

	gridw := int(roundedEnv.Max().X) - int(roundedEnv.Min().X)
	gridh := int(roundedEnv.Max().Y) - int(roundedEnv.Min().Y)

	// Create a grid of 2D unit vectors.
	gradients := make([][]geom.XY, gridw)
	for i := range gradients {
		gradients[i] = make([]geom.XY, gridh)
		for j := range vectors[i] {
			angle := rnd.Float64() * math.Pi * 2
			gradients[i][j] = geom.XY{
				X: math.Sin(angle),
				Y: math.Cos(angle),
			}
		}
	}
	return PerlinGenerator{roundedEnv, gradients}
}

func (p PerlinGenerator) Sample(pt geom.XY) float64 {
	type intxy struct {
		x, y int
	}
	a := intxy{int(pt.X), int(pt.Y)}
	b := intxy{int(pt.X), int(pt.Y) + 1}
	c := intxy{int(pt.X) + 1, int(pt.Y)}
	d := intxy{int(pt.X) + 1, int(pt.Y) + 1}

	ag := p.gradients[a.x][a.y]
	bg := p.gradients[b.x][b.y]
	cg := p.gradients[c.x][c.y]
	dg := p.gradients[d.x][d.y]

	ao := geom.XY{a.X, a.Y}.Sub(pt)
	bo := geom.XY{b.X, b.Y}.Sub(pt)
	co := geom.XY{c.X, c.Y}.Sub(pt)
	do := geom.XY{d.X, d.Y}.Sub(pt)

	adot := ao.Dot(ag)
	bdot := bo.Dot(bg)
	cdot := co.Dot(cg)
	ddot := do.Dot(dg)

	// TODO: lerp between the dot products based on the distance

}
