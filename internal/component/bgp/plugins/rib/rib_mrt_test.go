package rib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

func mustInternMRT(t *testing.T, p *attrpool.Pool, data []byte) attrpool.Handle {
	t.Helper()
	h, err := p.Intern(data)
	require.NoError(t, err)
	return h
}

func TestReconstructWireAttrs_RoundTrip(t *testing.T) {
	originVal := []byte{0x00}
	aspathVal := []byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9}
	localPrefVal := []byte{0x00, 0x00, 0x00, 0x64}

	b := storage.NewBundle()
	b.Origin = mustInternMRT(t, pool.Origin, originVal)
	b.LocalPref = mustInternMRT(t, pool.LocalPref, localPrefVal)
	bundleH := storage.Bundles.Intern(b)
	aspathH := mustInternMRT(t, pool.ASPath, aspathVal)

	entry := storage.RouteEntry{
		Bundle: bundleH,
		ASPath: aspathH,
	}
	defer entry.Release()

	wire := reconstructWireAttrs(entry, nil)
	require.NotNil(t, wire)
	require.True(t, len(wire) > 0)

	iter := attribute.NewAttrIterator(wire)
	found := map[attribute.AttributeCode][]byte{}
	for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
		cp := make([]byte, len(value))
		copy(cp, value)
		found[code] = cp
	}

	assert.Equal(t, originVal, found[attribute.AttrOrigin], "ORIGIN value mismatch")
	assert.Equal(t, aspathVal, found[attribute.AttrASPath], "AS_PATH value mismatch")
	assert.Equal(t, localPrefVal, found[attribute.AttrLocalPref], "LOCAL_PREF value mismatch")
}

func TestReconstructWireAttrs_EmptyEntry(t *testing.T) {
	entry := storage.NewRouteEntry()
	entry.Bundle = storage.Bundles.Intern(storage.NewBundle())
	defer entry.Release()

	wire := reconstructWireAttrs(entry, nil)
	assert.Equal(t, 0, len(wire))
}

func TestReconstructWireAttrs_ExtendedLength(t *testing.T) {
	bigCommunity := make([]byte, 300)
	for i := range bigCommunity {
		bigCommunity[i] = byte(i)
	}

	b := storage.NewBundle()
	b.Communities = mustInternMRT(t, pool.Communities, bigCommunity)
	bundleH := storage.Bundles.Intern(b)

	entry := storage.RouteEntry{
		Bundle: bundleH,
		ASPath: attrpool.InvalidHandle,
	}
	defer entry.Release()

	wire := reconstructWireAttrs(entry, nil)
	require.True(t, len(wire) > 300)

	iter := attribute.NewAttrIterator(wire)
	code, flags, value, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, attribute.AttrCommunity, code)
	assert.True(t, flags&attribute.FlagExtLength != 0, "should use extended length for >255 byte value")
	assert.Equal(t, 300, len(value))
	assert.Equal(t, bigCommunity, value)
}

func TestReconstructWireAttrs_AttributeOrder(t *testing.T) {
	b := storage.NewBundle()
	b.Origin = mustInternMRT(t, pool.Origin, []byte{0x00})
	b.NextHop = mustInternMRT(t, pool.NextHop, []byte{10, 0, 0, 1})
	b.MED = mustInternMRT(t, pool.MED, []byte{0, 0, 0, 10})
	b.LocalPref = mustInternMRT(t, pool.LocalPref, []byte{0, 0, 0, 100})
	bundleH := storage.Bundles.Intern(b)
	aspathH := mustInternMRT(t, pool.ASPath, []byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9})

	entry := storage.RouteEntry{Bundle: bundleH, ASPath: aspathH}
	defer entry.Release()

	wire := reconstructWireAttrs(entry, nil)

	var codes []attribute.AttributeCode
	iter := attribute.NewAttrIterator(wire)
	for code, _, _, ok := iter.Next(); ok; code, _, _, ok = iter.Next() {
		codes = append(codes, code)
	}

	assert.Equal(t, []attribute.AttributeCode{
		attribute.AttrOrigin,
		attribute.AttrASPath,
		attribute.AttrNextHop,
		attribute.AttrMED,
		attribute.AttrLocalPref,
	}, codes, "attributes should be in type-code order")
}

func TestAppendWireAttr_ShortValue(t *testing.T) {
	buf := appendWireAttr(nil, 1, 0x40, []byte{0x00})
	assert.Equal(t, []byte{0x40, 0x01, 0x01, 0x00}, buf)
}

func TestAppendWireAttr_ExtendedLength(t *testing.T) {
	value := make([]byte, 256)
	buf := appendWireAttr(nil, 8, 0xC0, value)
	assert.Equal(t, byte(0xD0), buf[0], "flags should include Extended Length bit")
	assert.Equal(t, byte(8), buf[1], "type code")
	assert.Equal(t, byte(1), buf[2], "length high byte")
	assert.Equal(t, byte(0), buf[3], "length low byte")
	assert.Equal(t, 4+256, len(buf), "total length")
}

func TestAppendOtherAttrsWire(t *testing.T) {
	// OtherAttrs pool format: typeCode(1) + flags(1) + lenHi(1) + lenLo(1) + value
	poolData := []byte{
		99, 0xC0, 0, 3, 0x0A, 0x0B, 0x0C,
	}
	buf := appendOtherAttrsWire(nil, poolData)

	iter := attribute.NewAttrIterator(buf)
	code, flags, value, ok := iter.Next()
	require.True(t, ok)
	assert.Equal(t, attribute.AttributeCode(99), code)
	assert.True(t, flags.IsOptional())
	assert.True(t, flags.IsTransitive())
	assert.Equal(t, []byte{0x0A, 0x0B, 0x0C}, value)
}
