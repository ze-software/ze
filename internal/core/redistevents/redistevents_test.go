package redistevents

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetRegistryForTest gives each test a clean slate so registration order
// across tests is deterministic.
func resetRegistryForTest(t *testing.T) {
	t.Helper()
	ResetForTest()
	t.Cleanup(ResetForTest)
}

// VALIDATES: RegisterProtocol returns the same ProtocolID on repeated calls
// with the same name (idempotency contract). IDs start at 1.
// PREVENTS: Producer registering twice during reload allocating fresh IDs and
// orphaning the old ID, which would silently lose subscribers.
func TestRegisterProtocolIdempotent(t *testing.T) {
	resetRegistryForTest(t)

	first := RegisterProtocol("l2tp")
	second := RegisterProtocol("l2tp")

	assert.Equal(t, ProtocolID(1), first, "first allocation must start at 1")
	assert.Equal(t, first, second, "same name must return same ID")
}

// VALIDATES: RegisterProtocol allocates monotonically increasing IDs for
// distinct names.
// PREVENTS: Different protocols colliding on the same numeric identity.
func TestRegisterProtocolDistinctNames(t *testing.T) {
	resetRegistryForTest(t)

	a := RegisterProtocol("l2tp")
	b := RegisterProtocol("connected")
	c := RegisterProtocol("static")

	assert.Equal(t, ProtocolID(1), a)
	assert.Equal(t, ProtocolID(2), b)
	assert.Equal(t, ProtocolID(3), c)
}

// VALIDATES: ProtocolName returns "" for ProtocolUnspecified and unknown IDs
// without panicking.
// PREVENTS: A corrupted batch with Protocol=0 crashing the consumer.
func TestProtocolNameUnknown(t *testing.T) {
	resetRegistryForTest(t)

	assert.Empty(t, ProtocolName(ProtocolUnspecified))
	assert.Empty(t, ProtocolName(ProtocolID(99)))

	id := RegisterProtocol("l2tp")
	assert.Equal(t, "l2tp", ProtocolName(id))
}

// VALIDATES: ProtocolIDOf returns (0, false) for unknown names.
// PREVENTS: Consumer treating an unknown name as ProtocolID=0 (== ProtocolUnspecified).
func TestProtocolIDOfUnknown(t *testing.T) {
	resetRegistryForTest(t)

	id, ok := ProtocolIDOf("missing")
	assert.False(t, ok)
	assert.Equal(t, ProtocolUnspecified, id)

	want := RegisterProtocol("l2tp")
	got, ok := ProtocolIDOf("l2tp")
	require.True(t, ok)
	assert.Equal(t, want, got)
}

// VALIDATES: RegisterProducer is idempotent -- the second call is a no-op.
// PREVENTS: A reload that re-registers a producer toggling its presence flag.
func TestRegisterProducerIdempotent(t *testing.T) {
	resetRegistryForTest(t)

	id := RegisterProtocol("l2tp")
	RegisterProducer(id)
	RegisterProducer(id)

	prods := Producers()
	assert.Equal(t, []ProtocolID{id}, prods)
}

// VALIDATES: Producers returns a snapshot independent of the registry's
// internal state. Mutating the returned slice does not affect subsequent calls.
// PREVENTS: Consumer mutating the result and corrupting registry state.
func TestProducersSnapshot(t *testing.T) {
	resetRegistryForTest(t)

	a := RegisterProtocol("l2tp")
	b := RegisterProtocol("connected")
	RegisterProducer(a)
	RegisterProducer(b)

	first := Producers()
	first[0] = ProtocolID(99) // mutate the caller's copy

	second := Producers()
	assert.Equal(t, []ProtocolID{a, b}, second, "registry must be unaffected by caller mutation")
}

// VALIDATES: Producers returns IDs sorted ascending so consumer enumeration
// is deterministic across runs.
// PREVENTS: Test flakes that depend on registration order.
func TestProducersSorted(t *testing.T) {
	resetRegistryForTest(t)

	// Register out of allocation order (allocated 1,2,3 -> registered as
	// producers 3,1,2). Producers() must return them sorted.
	a := RegisterProtocol("l2tp")
	b := RegisterProtocol("connected")
	c := RegisterProtocol("static")
	RegisterProducer(c)
	RegisterProducer(a)
	RegisterProducer(b)

	got := Producers()
	want := []ProtocolID{a, b, c}
	assert.Equal(t, want, got)
	assert.True(t, slices.IsSorted(got))
}

// VALIDATES: ActionUnspecified == 0 and is distinct from Add/Remove.
// PREVENTS: An uninitialised RouteChangeEntry being silently treated as Add or Remove.
func TestRouteActionZeroValueInvalid(t *testing.T) {
	var zero RouteAction
	assert.Equal(t, ActionUnspecified, zero, "zero value must be ActionUnspecified")
	assert.NotEqual(t, ActionAdd, zero)
	assert.NotEqual(t, ActionRemove, zero)
}

// VALIDATES: AC-13 boundary -- ProtocolID is uint16; first valid is 1, zero
// is the sentinel. RouteAction enum range 0..2; ge-3 is invalid (handler
// rejects, see Phase 3).
// PREVENTS: Out-of-range numeric inputs not being detected.
func TestEnumBoundaries(t *testing.T) {
	resetRegistryForTest(t)

	id := RegisterProtocol("p1")
	assert.Equal(t, ProtocolID(1), id, "first ProtocolID must be 1, not 0")

	// ProtocolUnspecified rejected by ProtocolName.
	assert.Empty(t, ProtocolName(0))
	// Out-of-range above also rejected (no entry).
	assert.Empty(t, ProtocolName(ProtocolID(65535)))

	// RouteAction stringification covers the valid range.
	assert.Equal(t, "add", ActionAdd.String())
	assert.Equal(t, "remove", ActionRemove.String())
	assert.Equal(t, "unspecified", ActionUnspecified.String())
	assert.Equal(t, "unspecified", RouteAction(99).String())
}

// VALIDATES: AcquireBatch returns a clean batch and ReleaseBatch resets all
// fields so the pool entry is reusable.
// PREVENTS: A producer leaking entries into the next caller's batch.
func TestBatchPoolReuse(t *testing.T) {
	first := AcquireBatch()
	require.NotNil(t, first)
	assert.Equal(t, ProtocolUnspecified, first.Protocol)
	assert.Equal(t, uint16(0), first.AFI)
	assert.Equal(t, uint8(0), first.SAFI)
	assert.Empty(t, first.Entries)

	first.Protocol = ProtocolID(7)
	first.AFI = 1
	first.SAFI = 1
	first.Entries = append(first.Entries, RouteChangeEntry{
		Action: ActionAdd,
		Prefix: netip.MustParsePrefix("10.0.0.0/24"),
	})
	prevCap := cap(first.Entries)

	ReleaseBatch(first)

	second := AcquireBatch()
	defer ReleaseBatch(second)
	assert.Equal(t, ProtocolUnspecified, second.Protocol, "Release must clear Protocol")
	assert.Equal(t, uint16(0), second.AFI, "Release must clear AFI")
	assert.Equal(t, uint8(0), second.SAFI, "Release must clear SAFI")
	assert.Empty(t, second.Entries, "Release must truncate Entries to len 0")
	assert.GreaterOrEqual(t, cap(second.Entries), prevCap, "backing array should be reused")
}

// VALIDATES: ReleaseBatch tolerates a nil batch (so producers can defer it
// directly after AcquireBatch).
// PREVENTS: A nil-check requirement at every producer call site.
func TestReleaseBatchNilSafe(t *testing.T) {
	assert.NotPanics(t, func() {
		ReleaseBatch(nil)
	})
}

// VALIDATES: Producer lifecycle (Acquire -> mutate -> Release -> Acquire)
// produces a fresh-looking batch each time and the entries pointer for an
// equal-or-larger backing slice.
// PREVENTS: Pool returning a half-recycled batch.
func TestPoolLifecycleRoundTrip(t *testing.T) {
	for i := range 8 {
		b := AcquireBatch()
		assert.Equal(t, ProtocolUnspecified, b.Protocol)
		assert.Empty(t, b.Entries)
		b.Protocol = ProtocolID(i + 1)
		b.Entries = append(b.Entries, RouteChangeEntry{Action: ActionAdd})
		ReleaseBatch(b)
	}
}
