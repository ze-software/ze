// Design: docs/architecture/plugin/rib-storage-design.md — attribute bundle dedup

package storage

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	pool "github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
)

func TestBundleSizeCompact(t *testing.T) {
	assert.Equal(t, uintptr(48), unsafe.Sizeof(Bundle{}), "Bundle must be exactly 48 bytes (12 handles x 4)")
}

func TestBundlePoolInternDedup(t *testing.T) {
	bp := newBundlePool()

	originH := mustIntern(t, pool.Origin, []byte{0x00})
	nextHopH := mustIntern(t, pool.NextHop, []byte{0x0A, 0x00, 0x00, 0x01})

	b1 := NewBundle()
	b1.Origin = originH
	b1.NextHop = nextHopH

	h1 := bp.Intern(b1)
	require.True(t, h1.IsValid())
	assert.Equal(t, 1, bp.Len(), "one unique bundle")
	assert.Equal(t, uint32(1), bp.RefCount(h1))

	originH2 := mustIntern(t, pool.Origin, []byte{0x00})
	nextHopH2 := mustIntern(t, pool.NextHop, []byte{0x0A, 0x00, 0x00, 0x01})

	b2 := NewBundle()
	b2.Origin = originH2
	b2.NextHop = nextHopH2

	h2 := bp.Intern(b2)
	assert.Equal(t, h1, h2, "identical bundles must return same handle")
	assert.Equal(t, 1, bp.Len(), "still one unique bundle")
	assert.Equal(t, uint32(2), bp.RefCount(h1), "refcount incremented on dedup")

	bp.Release(h1)
	bp.Release(h2)
}

func TestBundlePoolReleaseCascade(t *testing.T) {
	bp := newBundlePool()

	// Unique data unlikely to be interned by any other test.
	originH := mustIntern(t, pool.Origin, []byte{0xFE})
	lpH := mustIntern(t, pool.LocalPref, []byte{0xCA, 0xFE, 0xBA, 0xBE})

	b := NewBundle()
	b.Origin = originH
	b.LocalPref = lpH

	h := bp.Intern(b)
	require.Equal(t, uint32(1), bp.RefCount(h))

	bp.Release(h)
	assert.Equal(t, uint32(0), bp.RefCount(h), "refcount should be zero")
	assert.Equal(t, 0, bp.Len(), "bundle should be removed from index")

	_, errOrigin := pool.Origin.Get(originH)
	assert.Error(t, errOrigin, "inner Origin handle should be released by cascade")

	_, errLP := pool.LocalPref.Get(lpH)
	assert.Error(t, errLP, "inner LocalPref handle should be released by cascade")
}

func TestBundlePoolAddRef(t *testing.T) {
	bp := newBundlePool()

	originH := mustIntern(t, pool.Origin, []byte{0x00})
	b := NewBundle()
	b.Origin = originH

	h := bp.Intern(b)
	assert.Equal(t, uint32(1), bp.RefCount(h))

	bp.AddRef(h)
	assert.Equal(t, uint32(2), bp.RefCount(h))

	bp.Release(h)
	assert.Equal(t, uint32(1), bp.RefCount(h), "one release, still alive")
	assert.Equal(t, 1, bp.Len())

	data, err := pool.Origin.Get(originH)
	require.NoError(t, err, "inner handle must still be alive while bundle refcount > 0")
	assert.Equal(t, []byte{0x00}, data)

	bp.Release(h)
	assert.Equal(t, uint32(0), bp.RefCount(h))
	assert.Equal(t, 0, bp.Len())
}

func TestBundlePoolGet(t *testing.T) {
	bp := newBundlePool()

	originH := mustIntern(t, pool.Origin, []byte{0x02})
	medH := mustIntern(t, pool.MED, []byte{0x00, 0x00, 0x00, 0x32})

	b := NewBundle()
	b.Origin = originH
	b.MED = medH

	h := bp.Intern(b)
	got := bp.Get(h)

	assert.Equal(t, originH, got.Origin)
	assert.Equal(t, medH, got.MED)
	assert.Equal(t, attrpool.InvalidHandle, got.NextHop)
	assert.Equal(t, attrpool.InvalidHandle, got.LocalPref)

	bp.Release(h)
}

func TestBundlePoolSlotReuse(t *testing.T) {
	bp := newBundlePool()

	originH1 := mustIntern(t, pool.Origin, []byte{0xFA})
	b1 := NewBundle()
	b1.Origin = originH1
	h1 := bp.Intern(b1)
	slot1 := h1.Slot()

	bp.Release(h1)
	assert.Equal(t, 0, bp.Len())

	originH2 := mustIntern(t, pool.Origin, []byte{0xFB})
	b2 := NewBundle()
	b2.Origin = originH2
	h2 := bp.Intern(b2)

	assert.Equal(t, slot1, h2.Slot(), "freed slot should be reused")
	assert.Equal(t, 1, bp.Len())

	bp.Release(h2)
}

func TestBundlePoolHandleEncoding(t *testing.T) {
	bp := newBundlePool()

	b := NewBundle()
	h := bp.Intern(b)

	assert.Equal(t, bundlePoolIdx, h.PoolIdx(), "handle must use bundlePoolIdx")
	assert.True(t, h.IsValid())

	bp.Release(h)
}

func TestBundleNewBundleAllInvalid(t *testing.T) {
	b := NewBundle()
	assert.False(t, b.HasOrigin())
	assert.False(t, b.HasNextHop())
	assert.False(t, b.HasLocalPref())
	assert.False(t, b.HasMED())
	assert.False(t, b.HasAtomicAggregate())
	assert.False(t, b.HasAggregator())
	assert.False(t, b.HasCommunities())
	assert.False(t, b.hasLargeCommunities())
	assert.False(t, b.hasExtCommunities())
	assert.False(t, b.hasClusterList())
	assert.False(t, b.HasOriginatorID())
	assert.False(t, b.HasOtherAttrs())
}
