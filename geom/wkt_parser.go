package geom

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Function names is the parser are chosen to match closely with the BNF
// productions in the WKT grammar.
//
// Convention: functions starting with 'next' consume token(s), and build the
// next production in the grammar.

// UnmarshalWKT parses a Well Known Text (WKT), and returns the corresponding
// Geometry.
func UnmarshalWKT(wkt string, opts ...ConstructorOption) (Geometry, error) {
	return UnmarshalWKTFromReader(strings.NewReader(wkt), opts...)
}

// UnmarshalWKTFromReader parses a Well Known Text (WKT), and returns the
// corresponding Geometry. It the same as UnmarshalWKT, but allows an io.Reader
// to be used instead of a string.
func UnmarshalWKTFromReader(r io.Reader, opts ...ConstructorOption) (Geometry, error) {
	p := newParser(r, opts)
	g, err := p.nextGeometryTaggedText()
	if err != nil {
		return Geometry{}, err
	}
	if err := p.checkEOF(); err != nil {
		return Geometry{}, err
	}
	return g, nil
}

func newParser(r io.Reader, opts []ConstructorOption) *parser {
	return &parser{lexer: newWKTLexer(r), opts: opts}
}

type parser struct {
	lexer *wktLexer
	opts  []ConstructorOption
}

func (p *parser) nextToken() (string, error) {
	tok, err := p.lexer.next()
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return tok, err
}

func (p *parser) peekToken() (string, error) {
	tok, err := p.lexer.peek()
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return tok, err
}

func (p *parser) dropToken() error {
	_, err := p.nextToken()
	return err
}

func (p *parser) checkEOF() error {
	tok, err := p.lexer.next()
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	return fmt.Errorf("expected EOF but encountered %v", tok)
}

func (p *parser) nextGeometryTaggedText() (Geometry, error) {
	geomType, ctype, err := p.nextGeomTag()
	if err != nil {
		return Geometry{}, err
	}
	switch geomType {
	case "POINT":
		c, ok, err := p.nextPointText(ctype)
		if err != nil {
			return Geometry{}, err
		}
		if !ok {
			return NewEmptyPoint(ctype).AsGeometry(), nil
		}
		return NewPoint(c, p.opts...).AsGeometry(), nil
	case "LINESTRING":
		ls, err := p.nextLineStringText(ctype)
		return ls.AsGeometry(), err
	case "POLYGON":
		poly, err := p.nextPolygonText(ctype)
		return poly.AsGeometry(), err
	case "MULTIPOINT":
		mp, err := p.nextMultiPointText(ctype)
		return mp.AsGeometry(), err
	case "MULTILINESTRING":
		mls, err := p.nextMultiLineString(ctype)
		return mls.AsGeometry(), err
	case "MULTIPOLYGON":
		mp, err := p.nextMultiPolygonText(ctype)
		return mp.AsGeometry(), err
	case "GEOMETRYCOLLECTION":
		gc, err := p.nextGeometryCollectionText(ctype)
		return gc.AsGeometry(), err
	default:
		return Geometry{}, fmt.Errorf("unexpected token: %v", geomType)
	}
}

func (p *parser) nextGeomTag() (string, CoordinatesType, error) {
	geomTypeTok, err := p.nextToken()
	if err != nil {
		return "", 0, err
	}
	ctypeTok, err := p.peekToken()
	if err != nil {
		return "", 0, err
	}

	var ctype CoordinatesType
	switch geomTypeTok {
	case "Z":
		ctype = DimXYZ
		if err := p.dropToken(); err != nil {
			return "", 0, err
		}
	case "M":
		ctype = DimXYM
		if err := p.dropToken(); err != nil {
			return "", 0, err
		}
	case "ZM":
		ctype = DimXYZM
		if err := p.dropToken(); err != nil {
			return "", 0, err
		}
	default:
		ctype = DimXY
	}

	return strings.ToUpper(geomTypeTok), ctype, nil
}

//func (p *parser) nextEmptySetOrLeftParen() string {
//	tok := p.nextToken()
//	if tok != "EMPTY" && tok != "(" {
//		p.errorf("expected 'EMPTY' or '(' but encountered %v", tok)
//	}
//	return tok
//}

func (p *parser) nextRightParen() error {
	tok, err := p.nextToken()
	if err != nil {
		return err
	}
	if tok != ")" {
		return fmt.Errorf("expected ')' but encountered %v", tok)
	}
	return nil
}

//func (p *parser) nextCommaOrRightParen() string {
//	tok := p.nextToken()
//	if tok != ")" && tok != "," {
//		p.check(fmt.Errorf("expected ')' or ',' but encountered %v", tok))
//	}
//	return tok
//}

func (p *parser) nextPoint(ctype CoordinatesType) (Coordinates, error) {
	var c Coordinates
	c.Type = ctype
	var err error
	c.X, err = p.nextSignedNumericLiteral()
	if err != nil {
		return Coordinates{}, err
	}
	c.Y, err = p.nextSignedNumericLiteral()
	if err != nil {
		return Coordinates{}, err
	}
	if ctype.Is3D() {
		c.Z, err = p.nextSignedNumericLiteral()
		if err != nil {
			return Coordinates{}, err
		}
	}
	if ctype.IsMeasured() {
		c.M, err = p.nextSignedNumericLiteral()
		if err != nil {
			return Coordinates{}, err
		}
	}
	return c, nil
}

func (p *parser) nextPointAppend(dst []float64, ctype CoordinatesType) ([]float64, error) {
	for i := 0; i < ctype.Dimension(); i++ {
		f, err := p.nextSignedNumericLiteral()
		if err != nil {
			return nil, err
		}
		dst = append(dst, f)
	}
	return dst, nil
}

func (p *parser) nextSignedNumericLiteral() (float64, error) {
	var negative bool
	tok, err := p.nextToken()
	if err != nil {
		return 0, err
	}
	if tok == "-" {
		negative = true
		tok, err = p.nextToken()
		if err != nil {
			return 0, err
		}
	}
	f, err := strconv.ParseFloat(tok, 64)
	if err != nil {
		return 0, err
	}
	// NaNs and Infs are not allowed by the WKT grammar.
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("invalid signed numeric literal: %s", tok)
	}
	if negative {
		f = -f
	}
	return f, nil
}

func (p *parser) nextPointText(ctype CoordinatesType) (Coordinates, bool, error) {
	//tok := p.nextEmptySetOrLeftParen()
	tok, err := p.nextToken()
	if err != nil {
		return Coordinates{}, false, err
	}
	switch tok {
	case "EMPTY":
		return Coordinates{}, false, nil
	case "(":
		c, err := p.nextPoint(ctype)
		if err != nil {
			return Coordinates{}, false, err
		}
		if err := p.nextRightParen(); err != nil {
			return Coordinates{}, false, err
		}
		return c, true, nil
	default:
		return Coordinates{}, false, fmt.Errorf("unexpected token: %v", tok)
	}
}

func (p *parser) nextLineStringText(ctype CoordinatesType) (LineString, error) {
	tok, err := p.nextToken()
	if err != nil {
		return LineString{}, err
	}

	if tok == "EMPTY" {
		return LineString{}.ForceCoordinatesType(ctype), nil
	}
	if tok != "(" {
		return LineString{}, fmt.Errorf("unexpected token: %v", tok)
	}

	floats, err := p.nextPointAppend(nil, ctype)
	if err != nil {
		return LineString{}, err
	}
	for {
		tok, err := p.nextToken()
		if err != nil {
			return LineString{}, err
		}
		if tok == ")" {
			break
		}
		if tok != "," {
			return LineString{}, fmt.Errorf("unexpected token: %v", tok)
		}
		floats, err := p.nextPointAppend(nil, ctype)
		if err != nil {
			return LineString{}, err
		}
	}
	seq := NewSequence(floats, ctype)
	return NewLineString(seq, p.opts...)
}

func (p *parser) nextPolygonText(ctype CoordinatesType) (Polygon, error) {
	rings := p.nextPolygonOrMultiLineStringText(ctype)
	if len(rings) == 0 {
		return Polygon{}.ForceCoordinatesType(ctype)
	}
	poly, err := NewPolygonFromRings(rings, p.opts...)
	p.check(err)
	return poly
}

func (p *parser) nextMultiLineString(ctype CoordinatesType) (MultiLineString, error) {
	lss := p.nextPolygonOrMultiLineStringText(ctype)
	if len(lss) == 0 {
		return MultiLineString{}.ForceCoordinatesType(ctype)
	}
	return NewMultiLineStringFromLineStrings(lss, p.opts...)
}

func (p *parser) nextPolygonOrMultiLineStringText(ctype CoordinatesType) ([]LineString, error) {
	tok := p.nextEmptySetOrLeftParen()
	if tok == "EMPTY" {
		return nil
	}
	ls := p.nextLineStringText(ctype)
	lss := []LineString{ls}
	for {
		tok := p.nextCommaOrRightParen()
		if tok == "," {
			lss = append(lss, p.nextLineStringText(ctype))
		} else {
			break
		}
	}
	return lss
}

func (p *parser) nextMultiPointText(ctype CoordinatesType) (MultiPoint, error) {
	var floats []float64
	var empty BitSet
	tok := p.nextEmptySetOrLeftParen()
	if tok == "(" {
		for i := 0; true; i++ {
			if p.peekToken() == "EMPTY" {
				p.nextToken()
				for j := 0; j < ctype.Dimension(); j++ {
					floats = append(floats, 0)
				}
				empty.Set(i, true)
			} else {
				floats = p.nextMultiPointStylePointAppend(floats, ctype)
			}
			tok := p.nextCommaOrRightParen()
			if tok != "," {
				break
			}
		}
	}
	seq := NewSequence(floats, ctype)
	return NewMultiPointWithEmptyMask(seq, empty, p.opts...)
}

func (p *parser) nextMultiPointStylePointAppend(dst []float64, ctype CoordinatesType) ([]float64, error) {
	// This is an extension of the spec, and is required to handle WKT output
	// from non-complying implementations. In particular, PostGIS doesn't
	// comply to the spec (it outputs points as part of a multipoint without
	// their surrounding parentheses).
	var useParens bool
	if p.peekToken() == "(" {
		p.nextToken()
		useParens = true
	}
	dst = p.nextPointAppend(dst, ctype)
	if useParens {
		p.nextRightParen()
	}
	return dst
}

func (p *parser) nextMultiPolygonText(ctype CoordinatesType) (MultiPolygon, error) {
	var polys []Polygon
	tok := p.nextEmptySetOrLeftParen()
	if tok == "(" {
		for {
			poly := p.nextPolygonText(ctype)
			polys = append(polys, poly)
			tok := p.nextCommaOrRightParen()
			if tok != "," {
				break
			}
		}
	}
	if len(polys) == 0 {
		return MultiPolygon{}.ForceCoordinatesType(ctype)
	}
	mp, err := NewMultiPolygonFromPolygons(polys, p.opts...)
	p.check(err)
	return mp
}

func (p *parser) nextGeometryCollectionText(ctype CoordinatesType) (GeometryCollection, error) {
	var geoms []Geometry
	tok := p.nextEmptySetOrLeftParen()
	if tok == "(" {
		for {
			g, err := p.nextGeometryTaggedText()
			if err != nil {
				return GeometryCollection{}, err
			}
			geoms = append(geoms, g)
			tok := p.nextCommaOrRightParen()
			if tok != "," {
				break
			}
		}
	}
	if len(geoms) == 0 {
		return GeometryCollection{}.ForceCoordinatesType(ctype), nil
	}
	return NewGeometryCollection(geoms, p.opts...), nil
}
