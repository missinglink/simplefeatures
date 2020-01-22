package generate

import (
	"math"
	"math/rand"

	"github.com/peterstace/simplefeatures/geom"
)

type PerlinGenerator struct {
	env       geom.Envelope
	gradients [][]geom.XY
	originX   int
	originY   int
}

// NewPerlinGenerator constructs a perlin generator, which can generate perlin
// noise within the given envelope.
func NewPerlinGenerator(env geom.Envelope, rnd *rand.Rand) PerlinGenerator {
	roundedEnv := geom.NewEnvelope(
		geom.XY{X: math.Floor(env.Min().X) - 1, Y: math.Floor(env.Min().Y) - 1},
		geom.XY{X: math.Ceil(env.Max().X) + 1, Y: math.Ceil(env.Max().Y) + 1},
	)

	gridw := int(roundedEnv.Max().X) - int(roundedEnv.Min().X) + 1
	gridh := int(roundedEnv.Max().Y) - int(roundedEnv.Min().Y) + 1

	// Create a grid of 2D unit vectors.
	gradients := make([][]geom.XY, gridw)
	for i := range gradients {
		gradients[i] = make([]geom.XY, gridh)
		for j := range gradients[i] {
			angle := rnd.Float64() * math.Pi * 2
			gradients[i][j] = geom.XY{
				X: math.Sin(angle),
				Y: math.Cos(angle),
			}
		}
	}
	return PerlinGenerator{
		roundedEnv,
		gradients,
		int(roundedEnv.Min().X),
		int(roundedEnv.Min().Y),
	}
}

func (p PerlinGenerator) Sample(pt geom.XY) float64 {
	x0 := int(pt.X - p.env.Min().X)
	x1 := x0 + 1
	y0 := int(pt.Y - p.env.Min().Y)
	y1 := y0 + 1

	n0 := p.dotGridGradient(x0, y0, pt)
	n1 := p.dotGridGradient(x1, y0, pt)
	n2 := p.dotGridGradient(x0, y1, pt)
	n3 := p.dotGridGradient(x1, y1, pt)

	sx := pt.X - float64(x0+p.originX)
	sy := pt.Y - float64(y0+p.originY)

	lerp := func(a, b, w float64) float64 {
		return (1-w)*a + w*b
	}
	return lerp(lerp(n0, n1, sx), lerp(n2, n3, sx), sy)
}

func (p PerlinGenerator) dotGridGradient(x, y int, pt geom.XY) float64 {
	distance := geom.XY{
		X: pt.X - float64(x+p.originX),
		Y: pt.Y - float64(y+p.originY),
	}
	return distance.Dot(p.gradients[x][y])
}
