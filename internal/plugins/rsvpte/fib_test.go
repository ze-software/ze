// Design: plan/spec-mpls-3-rsvp-te.md -- busFIB emit tests
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// captureBus records Emit calls so a test can assert the (namespace, eventType,
// payload) a producer publishes.
type captureBus struct {
	emits []capturedEmit
}

type capturedEmit struct {
	namespace string
	eventType string
	payload   any
}

func (c *captureBus) Emit(namespace, eventType string, payload any) (int, error) {
	c.emits = append(c.emits, capturedEmit{namespace: namespace, eventType: eventType, payload: payload})
	return 0, nil
}

func (c *captureBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

func (c *captureBus) lastEntry(t *testing.T) mplsfibevents.Entry {
	t.Helper()
	require.NotEmpty(t, c.emits, "an event was emitted")
	last := c.emits[len(c.emits)-1]
	assert.Equal(t, mplsfibevents.Namespace, last.namespace)
	assert.Equal(t, mplsfibevents.EventEntry, last.eventType)
	batch, ok := last.payload.(*mplsfibevents.EntryBatch)
	require.True(t, ok, "payload is *EntryBatch")
	require.Len(t, batch.Entries, 1)
	return batch.Entries[0]
}

// VALIDATES: busFIB translates each fibProgrammer call into the matching
// (mpls-fib, entry) event so fib-kernel programs the kernel.
func TestBusFIBEmitsEntries(t *testing.T) {
	bus := &captureBus{}
	fib := newBusFIB(bus, slogutil.DiscardLogger())
	nh := netip.MustParseAddr("10.0.0.5")

	require.NoError(t, fib.programPush(netip.MustParsePrefix("10.0.0.9/32"), 16000, nh))
	e := bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.ActionAdd, e.Action)
	assert.Equal(t, mplsfibevents.OpPush, e.Op)
	assert.Equal(t, []uint32{16000}, e.OutLabels)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.9/32"), e.FEC)
	assert.Equal(t, mplsSourceRSVPTE, e.Source)

	require.NoError(t, fib.programSwap(1000, 2000, nh))
	e = bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.OpSwap, e.Op)
	assert.Equal(t, uint32(1000), e.InLabel)
	assert.Equal(t, []uint32{2000}, e.OutLabels)

	require.NoError(t, fib.programPop(1001, nh))
	e = bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.OpPop, e.Op)
	assert.Equal(t, uint32(1001), e.InLabel)
	assert.Empty(t, e.OutLabels)

	require.NoError(t, fib.removePush(netip.MustParsePrefix("10.0.0.9/32")))
	e = bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.ActionRemove, e.Action)
	assert.Equal(t, mplsfibevents.OpPush, e.Op)

	require.NoError(t, fib.removeSwap(1000))
	e = bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.ActionRemove, e.Action)
	assert.Equal(t, mplsfibevents.OpSwap, e.Op)
	assert.Equal(t, uint32(1000), e.InLabel)
}

// VALIDATES: busFIB with no bus does not panic (degraded mode).
func TestBusFIBNilBus(t *testing.T) {
	fib := newBusFIB(nil, slogutil.DiscardLogger())
	assert.NoError(t, fib.programPush(netip.MustParsePrefix("10.0.0.9/32"), 16000, netip.Addr{}))
}
