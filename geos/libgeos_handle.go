package geos

/*
#cgo LDFLAGS: -lgeos_c
#include "geos_c.h"
#include <stdlib.h>
#include <string.h>

void sf_error_handler(const char *message, void *userdata) {
	strncpy(userdata, message, 1024);
}

GEOSContextHandle_t sf_init(void *userdata) {
	GEOSContextHandle_t ctx = GEOS_init_r();
	GEOSContext_setErrorMessageHandler_r(ctx, sf_error_handler, userdata);
	return ctx;
}

unsigned char *marshal(GEOSContextHandle_t handle, const GEOSGeometry *g, size_t *size, char *isWKT);

*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"github.com/peterstace/simplefeatures/geom"
)

// Handle is an opaque handle that can be used to invoke GEOS operations.
// Instances are not threadsafe and thus should only be used serially (e.g.
// protected by a mutex or similar).
type Handle struct {
	context C.GEOSContextHandle_t
	reader  *C.GEOSWKBReader
	errBuf  *C.char
}

// NewHandle creates a new GEOS handle.
func NewHandle() (*Handle, error) {
	h := &Handle{}

	h.errBuf = (*C.char)(C.malloc(1024))
	if h.errBuf == nil {
		h.Release()
		return nil, errors.New("malloc failed")
	}

	h.context = C.sf_init(unsafe.Pointer(&h.errBuf))
	if h.context == nil {
		h.Release()
		return nil, errors.New("could not create GEOS context")
	}

	h.reader = C.GEOSWKBReader_create_r(h.context)
	if h.reader == nil {
		h.Release()
		return nil, h.err()
	}

	return h, nil
}

// err gets the last error message reported by GEOS as an error type. It
// always returns a non-nil error. If no error message has been reported, then
// it returns a generic error message.
func (h *Handle) err() error {
	msg := h.errMsg()
	if msg == "" {
		// No error stored, which indicates that the error handler didn't get
		// trigged. The best we can do is give a generic error.
		msg = "GEOS internal error"
	}
	C.memset((unsafe.Pointer)(h.errBuf), 0, 1024) // Reset the buffer for the next error message.
	return errors.New(strings.TrimSpace(msg))
}

// errMsg gets the textual representation of the last error message reported by
// GEOS.
func (h *Handle) errMsg() string {
	// The error message is either NULL terminated, or fills the entire buffer.
	buf := C.GoBytes((unsafe.Pointer)(h.errBuf), 1024)
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf[:])
}

// Release releases any resources held by the handle. The handle should not be
// used after Release is called.
func (h *Handle) Release() {
	if h.reader != nil {
		C.GEOSWKBReader_destroy_r(h.context, h.reader)
		h.reader = (*C.GEOSWKBReader)(C.NULL)
	}
	if h.context != nil {
		C.GEOS_finish_r(h.context)
		h.context = C.GEOSContextHandle_t(C.NULL)
	}
	if h.errBuf != nil {
		C.free((unsafe.Pointer)(h.errBuf))
		h.errBuf = (*C.char)(C.NULL)
	}
}

// createGeometryHandle converts a Geometry object into a GEOS geometry handle.
func (h *Handle) createGeometryHandle(g geom.Geometry) (*C.GEOSGeometry, error) {
	wkb := g.AsBinary()
	gh := C.GEOSWKBReader_read_r(
		h.context,
		h.reader,
		(*C.uchar)(&wkb[0]),
		C.ulong(len(wkb)),
	)
	if gh == nil {
		return nil, h.err()
	}
	return gh, nil
}

// ErrGeometryCollectionNotSupported indicates that a GeometryCollection was
// passed to a function that does not support GeometryCollections.
var ErrGeometryCollectionNotSupported = errors.New("GeometryCollection not supported")

// Equals returns true if and only if the input geometries are spatially equal.
func (h *Handle) Equals(g1, g2 geom.Geometry) (bool, error) {
	if g1.IsEmpty() && g2.IsEmpty() {
		// Part of the mask is 'dim(I(a) ∩ I(b)) = T'.  If both inputs are
		// empty, then their interiors will be empty, and thus
		// 'dim(I(a) ∩ I(b) = F'. However, we want to return 'true' for this
		// case. So we just return true manually rather than using DE-9IM.
		return true, nil
	}
	return h.relate(g1, g2, "T*F**FFF*")
}

// Disjoint returns true if and only if the input geometries have no points in
// common.
func (h *Handle) Disjoint(g1, g2 geom.Geometry) (bool, error) {
	return h.relate(g1, g2, "FF*FF****")
}

// Touches returns true if and only if the geometries have at least 1 point in
// common, but their interiors don't intersect.
func (h *Handle) Touches(g1, g2 geom.Geometry) (bool, error) {
	return h.relatesAny(
		g1, g2,
		"FT*******",
		"F**T*****",
		"F***T****",
	)
}

// Contains returns true if and only if geometry A contains geometry B.  See
// the global Contains function for details.
func (h *Handle) Contains(a, b geom.Geometry) (bool, error) {
	return h.relate(a, b, "T*****FF*")
}

// Covers returns true if and only if geometry A covers geometry B. See the
// global Covers function for details.
func (h *Handle) Covers(a, b geom.Geometry) (bool, error) {
	return h.relatesAny(
		a, b,
		"T*****FF*",
		"*T****FF*",
		"***T**FF*",
		"****T*FF*",
	)
}

// Intersects returns true if and only if the geometries share at least one
// point in common.
func (h *Handle) Intersects(a, b geom.Geometry) (bool, error) {
	return h.relatesAny(
		a, b,
		"T********",
		"*T*******",
		"***T*****",
		"****T****",
	)
}

// Within returns true if and only if geometry A is completely within geometry
// B. See the global Within function for details.
func (h *Handle) Within(a, b geom.Geometry) (bool, error) {
	return h.relate(a, b, "T*F**F***")
}

// CoveredBy returns true if and only if geometry A is covered by geometry B.
// See the global CoveredBy function for details.
func (h *Handle) CoveredBy(a, b geom.Geometry) (bool, error) {
	return h.relatesAny(
		a, b,
		"T*F**F***",
		"*TF**F***",
		"**FT*F***",
		"**F*TF***",
	)
}

// Crosses returns true if and only if geometry A and B cross each other. See
// the global Crosses function for details.
func (h *Handle) Crosses(a, b geom.Geometry) (bool, error) {
	dimA := a.Dimension()
	dimB := b.Dimension()
	switch {
	case dimA < dimB: // Point/Line, Point/Area, Line/Area
		return h.relate(a, b, "T*T******")
	case dimA > dimB: // Line/Point, Area/Point, Area/Line
		return h.relate(a, b, "T*****T**")
	case dimA == 1 && dimB == 1: // Line/Line
		return h.relate(a, b, "0********")
	default:
		return false, nil
	}
}

// Overlaps returns true if and only if the geometry A and B overlap each
// other. See the global Overlaps function for details.
func (h *Handle) Overlaps(a, b geom.Geometry) (bool, error) {
	dimA := a.Dimension()
	dimB := b.Dimension()
	switch {
	case (dimA == 0 && dimB == 0) || (dimA == 2 && dimB == 2):
		return h.relate(a, b, "T*T***T**")
	case (dimA == 1 && dimB == 1):
		return h.relate(a, b, "1*T***T**")
	default:
		return false, nil
	}

}

// relatesAny checks if the two geometries are related using any of the masks.
func (h *Handle) relatesAny(g1, g2 geom.Geometry, masks ...string) (bool, error) {
	for _, m := range masks {
		r, err := h.relate(g1, g2, m)
		if err != nil {
			return false, err
		}
		if r {
			return true, nil
		}
	}
	return false, nil
}

// relate invokes the GEOS GEOSRelatePattern function, which checks if two
// geometries are related according to a DE-9IM 'relates' mask.
func (h *Handle) relate(g1, g2 geom.Geometry, mask string) (bool, error) {
	if g1.IsGeometryCollection() || g2.IsGeometryCollection() {
		return false, ErrGeometryCollectionNotSupported
	}
	if len(mask) != 9 {
		return false, fmt.Errorf("mask has invalid length: %q", mask)
	}

	// Not all versions of GEOS can handle Z and M geometries correctly. For
	// Relates, we only need 2D geometries anyway.
	g1 = g1.Force2D()
	g2 = g2.Force2D()

	gh1, err := h.createGeometryHandle(g1)
	if err != nil {
		return false, err
	}
	defer C.GEOSGeom_destroy(gh1)

	gh2, err := h.createGeometryHandle(g2)
	if err != nil {
		return false, err
	}
	defer C.GEOSGeom_destroy(gh2)

	var cmask [10]byte
	copy(cmask[:], mask)

	return h.boolErr(C.GEOSRelatePattern_r(
		h.context, gh1, gh2,
		(*C.char)(unsafe.Pointer(&cmask)),
	))
}

// boolErr converts a char result from GEOS into a boolean result.
func (h *Handle) boolErr(c C.char) (bool, error) {
	const (
		// From geos_c.h:
		// return 2 on exception, 1 on true, 0 on false.
		relateException = 2
		relateTrue      = 1
		relateFalse     = 0
	)
	switch c {
	case 0:
		return false, nil
	case 1:
		return true, nil
	case 2:
		return false, h.err()
	default:
		return false, fmt.Errorf("illegal result from GEOS: %v", c)
	}
}

// Union returns a geometry that that is the union of the input geometries. See
// the global Union function for details.
func (h *Handle) Union(g1, g2 geom.Geometry) (geom.Geometry, error) {
	return h.binaryOperation(g1, g2, func(gh1, gh2 *C.GEOSGeometry) *C.GEOSGeometry {
		return C.GEOSUnion_r(h.context, gh1, gh2)
	})
}

// Intersection returns a geometry that is the intersection of the input
// geometries. See the global Intersection function for details.
func (h *Handle) Intersection(g1, g2 geom.Geometry) (geom.Geometry, error) {
	return h.binaryOperation(g1, g2, func(gh1, gh2 *C.GEOSGeometry) *C.GEOSGeometry {
		return C.GEOSIntersection_r(h.context, gh1, gh2)
	})
}

func (h *Handle) binaryOperation(
	g1, g2 geom.Geometry,
	op func(*C.GEOSGeometry, *C.GEOSGeometry) *C.GEOSGeometry,
) (geom.Geometry, error) {
	// Not all versions of GEOS can handle Z and M geometries correctly. For
	// binary operations, we only need 2D geometries anyway.
	g1 = g1.Force2D()
	g2 = g2.Force2D()

	gh1, err := h.createGeometryHandle(g1)
	if err != nil {
		return geom.Geometry{}, err
	}
	defer C.GEOSGeom_destroy(gh1)

	gh2, err := h.createGeometryHandle(g2)
	if err != nil {
		return geom.Geometry{}, err
	}
	defer C.GEOSGeom_destroy(gh2)

	resultGH := op(gh1, gh2)
	if resultGH == nil {
		return geom.Geometry{}, h.err()
	}
	defer C.GEOSGeom_destroy(resultGH)

	return h.decode(resultGH)
}

// Buffer returns a geometry that contains all points within the given radius
// of the input geometry.
func (h *Handle) Buffer(g geom.Geometry, radius float64) (geom.Geometry, error) {
	return h.unaryOperation(g, func(gh *C.GEOSGeometry) *C.GEOSGeometry {
		return C.GEOSBufferWithStyle_r(h.context, gh, C.double(radius), 8, C.GEOSBUF_CAP_ROUND, C.GEOSBUF_JOIN_ROUND, 0.0)
	})
}

func (h *Handle) unaryOperation(
	g geom.Geometry,
	op func(*C.GEOSGeometry) *C.GEOSGeometry,
) (geom.Geometry, error) {
	// Not all versions of libgeos can handle Z and M geometries correctly. For
	// unary operations, we only need 2D geometries anyway.
	g = g.Force2D()

	gh, err := h.createGeometryHandle(g)
	if err != nil {
		return geom.Geometry{}, err
	}
	defer C.GEOSGeom_destroy(gh)

	resultGH := op(gh)
	if resultGH == nil {
		return geom.Geometry{}, h.err()
	}
	defer C.GEOSGeom_destroy(resultGH)

	return h.decode(resultGH)
}

func (h *Handle) decode(gh *C.GEOSGeometry) (geom.Geometry, error) {
	var (
		isWKT C.char
		size  C.size_t
	)
	serialised := C.marshal(h.context, gh, &size, &isWKT)
	if serialised == nil {
		return geom.Geometry{}, h.err()
	}
	defer C.GEOSFree_r(h.context, unsafe.Pointer(serialised))
	r := bytes.NewReader(C.GoBytes(unsafe.Pointer(serialised), C.int(size)))

	if isWKT != 0 {
		return geom.UnmarshalWKT(r)
	}
	return geom.UnmarshalWKB(r)
}
