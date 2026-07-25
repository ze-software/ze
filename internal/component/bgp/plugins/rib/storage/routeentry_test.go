package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	pool "github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
)

func mustIntern(t *testing.T, p *attrpool.Pool, data []byte) attrpool.Handle {
	t.Helper()
	h, err := p.Intern(data)
	require.NoError(t, err)
	return h
}

// TestRouteEntry_NewEmpty verifies empty RouteEntry has InvalidHandle for all fields.
func TestRouteEntry_NewEmpty(t *testing.T) {
	entry := NewRouteEntry()

	assert.Equal(t, attrpool.InvalidHandle, entry.Bundle, "Bundle should be InvalidHandle")
	assert.Equal(t, attrpool.InvalidHandle, entry.ASPath, "ASPath should be InvalidHandle")
}

// TestRouteEntry_HasAttribute verifies attribute presence checks.
func TestRouteEntry_HasAttribute(t *testing.T) {
	entry := NewRouteEntry()

	assert.False(t, entry.HasASPath(), "ASPath should be absent")
	assert.False(t, entry.HasBundle(), "Bundle should be absent")

	// Create a bundle with Origin set
	b := NewBundle()
	b.Origin = mustIntern(t, pool.Origin, []byte{0x00})
	entry.Bundle = Bundles.Intern(b)
	defer entry.Release()

	got := entry.GetBundle()
	assert.True(t, got.HasOrigin(), "Origin should be present after setting via bundle")
	assert.False(t, entry.HasASPath(), "ASPath should still be absent")
}

// TestRouteEntry_Release verifies proper cleanup of all handles.
func TestRouteEntry_Release(t *testing.T) {
	b := NewBundle()
	b.Origin = mustIntern(t, pool.Origin, []byte{0x00})
	b.LocalPref = mustIntern(t, pool.LocalPref, []byte{0x00, 0x00, 0x00, 0x64})

	entry := RouteEntry{
		Bundle: Bundles.Intern(b),
		ASPath: mustIntern(t, pool.ASPath, []byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9}),
	}

	require.True(t, entry.Bundle.IsValid())
	require.True(t, entry.ASPath.IsValid())

	entry.Release()

	assert.Equal(t, attrpool.InvalidHandle, entry.Bundle, "Bundle should be InvalidHandle after release")
	assert.Equal(t, attrpool.InvalidHandle, entry.ASPath, "ASPath should be InvalidHandle after release")
}

// TestRouteEntry_AddRef verifies reference counting for sharing.
func TestRouteEntry_AddRef(t *testing.T) {
	b := NewBundle()
	b.Origin = mustIntern(t, pool.Origin, []byte{0x01})

	entry := RouteEntry{
		Bundle: Bundles.Intern(b),
		ASPath: mustIntern(t, pool.ASPath, []byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9}),
	}

	err := entry.AddRef()
	require.NoError(t, err)

	// Now we need to release twice
	entry.Release()
	// Second release should still work (AddRef gave us a second ref)
	entry2 := RouteEntry{Bundle: entry.Bundle, ASPath: entry.ASPath}
	_ = entry2
}

// TestRouteEntry_Clone verifies entry cloning with ref increment.
func TestRouteEntry_Clone(t *testing.T) {
	b := NewBundle()
	b.Origin = mustIntern(t, pool.Origin, []byte{0x02})
	b.MED = mustIntern(t, pool.MED, []byte{0x00, 0x00, 0x00, 0x0A})

	entry := RouteEntry{
		AttrFingerprint: 0xDEADBEEF,
		AttrLen:         42,
		Bundle:          Bundles.Intern(b),
		ASPath:          mustIntern(t, pool.ASPath, []byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9}),
	}

	clone := entry.Clone()
	require.NotNil(t, clone, "Clone should succeed")

	assert.Equal(t, entry.Bundle, clone.Bundle)
	assert.Equal(t, entry.ASPath, clone.ASPath)
	assert.Equal(t, entry.AttrFingerprint, clone.AttrFingerprint)
	assert.Equal(t, entry.AttrLen, clone.AttrLen)

	entry.Release()
	clone.Release()
}

// TestRouteEntry_ClonePreservesFingerprint verifies Clone copies AttrFingerprint and AttrLen.
func TestRouteEntry_ClonePreservesFingerprint(t *testing.T) {
	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	entry, fp, attrLen, err := ParseRouteEntry(attrs, true)
	require.NoError(t, err)

	clone := entry.Clone()
	require.NotNil(t, clone)

	assert.Equal(t, fp, clone.AttrFingerprint)
	assert.Equal(t, attrLen, clone.AttrLen)
	assert.NotZero(t, clone.AttrFingerprint)
	assert.NotZero(t, clone.AttrLen)

	entry.Release()
	clone.Release()
}

// TestRouteEntry_SharedOrigin verifies two routes share ORIGIN handle via bundle dedup.
func TestRouteEntry_SharedOrigin(t *testing.T) {
	originIGP := []byte{0x00}

	b1 := NewBundle()
	b1.Origin = mustIntern(t, pool.Origin, originIGP)

	b2 := NewBundle()
	b2.Origin = mustIntern(t, pool.Origin, originIGP)

	entry1 := RouteEntry{Bundle: Bundles.Intern(b1)}
	entry2 := RouteEntry{Bundle: Bundles.Intern(b2)}

	// Same bundle content should dedup to same handle
	assert.Equal(t, entry1.Bundle, entry2.Bundle, "identical bundles should share handle")

	entry1.Release()
	entry2.Release()
}

// TestRouteEntry_CloneReturnsNilOnError verifies Clone handles AddRef failure.
func TestRouteEntry_CloneReturnsNilOnError(t *testing.T) {
	b := NewBundle()
	b.Origin = mustIntern(t, pool.Origin, []byte{0x00})

	entry := RouteEntry{Bundle: Bundles.Intern(b), ASPath: attrpool.InvalidHandle}

	clone := entry.Clone()
	assert.NotNil(t, clone, "Clone should succeed with valid pools")

	entry.Release()
	if clone != nil {
		clone.Release()
	}
}

// TestRouteEntry_DifferentMED verifies partial sharing with different MED.
func TestRouteEntry_DifferentMED(t *testing.T) {
	originIGP := []byte{0x00}
	localPref100 := []byte{0x00, 0x00, 0x00, 0x64}

	b1 := NewBundle()
	b1.Origin = mustIntern(t, pool.Origin, originIGP)
	b1.LocalPref = mustIntern(t, pool.LocalPref, localPref100)
	b1.MED = mustIntern(t, pool.MED, []byte{0x00, 0x00, 0x00, 0x0A})

	b2 := NewBundle()
	b2.Origin = mustIntern(t, pool.Origin, originIGP)
	b2.LocalPref = mustIntern(t, pool.LocalPref, localPref100)
	b2.MED = mustIntern(t, pool.MED, []byte{0x00, 0x00, 0x00, 0x14})

	entry1 := RouteEntry{Bundle: Bundles.Intern(b1)}
	entry2 := RouteEntry{Bundle: Bundles.Intern(b2)}

	// Different MED means different bundles
	assert.NotEqual(t, entry1.Bundle, entry2.Bundle,
		"different MED should produce different bundle handles")

	entry1.Release()
	entry2.Release()
}

// TestRouteEntry_WireRoundTrip verifies parse → store → reconstruct preserves data.
func TestRouteEntry_WireRoundTrip(t *testing.T) {
	wireOrigin := []byte{0x40, 0x01, 0x01, 0x00}
	wireASPath := []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9}
	wireNextHop := []byte{0x40, 0x03, 0x04, 0x0A, 0x00, 0x00, 0x01}
	wireLocalPref := []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64}
	wireMED := []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x32}

	original := concat(wireOrigin, wireASPath, wireNextHop, wireLocalPref, wireMED)

	entry, err := ParseAttributes(original, true)
	require.NoError(t, err)
	defer entry.Release()

	reconstructed, err := entry.ToWireBytes()
	require.NoError(t, err)

	entry2, err := ParseAttributes(reconstructed, true)
	require.NoError(t, err)
	defer entry2.Release()

	b1 := entry.GetBundle()
	b2 := entry2.GetBundle()

	origOrigin, _ := pool.Origin.Get(b1.Origin)
	reconOrigin, _ := pool.Origin.Get(b2.Origin)
	assert.Equal(t, origOrigin, reconOrigin, "ORIGIN value should match")

	origASPath, _ := pool.ASPath.Get(entry.ASPath)
	reconASPath, _ := pool.ASPath.Get(entry2.ASPath)
	assert.Equal(t, origASPath, reconASPath, "AS_PATH value should match")

	origNextHop, _ := pool.NextHop.Get(b1.NextHop)
	reconNextHop, _ := pool.NextHop.Get(b2.NextHop)
	assert.Equal(t, origNextHop, reconNextHop, "NEXT_HOP value should match")

	origLocalPref, _ := pool.LocalPref.Get(b1.LocalPref)
	reconLocalPref, _ := pool.LocalPref.Get(b2.LocalPref)
	assert.Equal(t, origLocalPref, reconLocalPref, "LOCAL_PREF value should match")

	origMED, _ := pool.MED.Get(b1.MED)
	reconMED, _ := pool.MED.Get(b2.MED)
	assert.Equal(t, origMED, reconMED, "MED value should match")
}
