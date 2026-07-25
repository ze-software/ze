// Design: plan/spec-mpls-3-rsvp-te.md -- MPLS forwarding-entry dispatch tests
package fibkernel

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

type mplsSwapRec struct {
	in  uint32
	out []uint32
	nh  netip.Addr
}

// mplsMockBackend implements routeBackend, richRouteBackend (promoted) and
// mplsBackend so handleMPLSEntry can dispatch push (rich) and swap/pop (MPLS).
type mplsMockBackend struct {
	*richMockBackend
	swapsAdded   []mplsSwapRec
	swapsDeleted []uint32
}

func (m *mplsMockBackend) addMPLSSwap(in uint32, out []uint32, nh netip.Addr) error { //nolint:unparam // mock satisfies the mplsBackend error contract
	m.swapsAdded = append(m.swapsAdded, mplsSwapRec{in: in, out: out, nh: nh})
	return nil
}

func (m *mplsMockBackend) delMPLSSwap(in uint32) error { //nolint:unparam // mock satisfies the mplsBackend error contract
	m.swapsDeleted = append(m.swapsDeleted, in)
	return nil
}

func newMPLSMockBackend() *mplsMockBackend {
	return &mplsMockBackend{richMockBackend: newRichMockBackend()}
}

// VALIDATES: AC-4 -- a push entry programs an IP route with the imposed label
// stack via the rich-route backend and is tracked for the gauge.
func TestHandleMPLSEntryPush(t *testing.T) {
	mb := newMPLSMockBackend()
	f := newFIBKernel(mb)

	f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
		Action:    mplsfibevents.ActionAdd,
		Op:        mplsfibevents.OpPush,
		FEC:       netip.MustParsePrefix("10.0.0.9/32"),
		OutLabels: []uint32{16000},
		NextHop:   netip.MustParseAddr("10.0.0.5"),
	}}})

	// First install uses Add (RouteAdd) so it fails safe on a foreign route rather
	// than clobbering it; it does not Replace.
	require.Len(t, mb.richAdded, 1)
	assert.Empty(t, mb.richReplaced, "first install must not Replace (would clobber a foreign route)")
	assert.Equal(t, netip.MustParsePrefix("10.0.0.9/32"), mb.richAdded[0].Prefix)
	assert.Equal(t, []uint32{16000}, mb.richAdded[0].Labels)
	assert.True(t, f.mplsInstalled["10.0.0.9/32"])
	assert.Equal(t, 1, f.mplsCountLocked())
}

// VALIDATES: mpls-2 -- re-advertising a FEC with a new label Replaces ze's own
// route (so the new label is imposed), while the first install used Add.
func TestHandleMPLSEntryPushRelabel(t *testing.T) {
	mb := newMPLSMockBackend()
	f := newFIBKernel(mb)
	fec := netip.MustParsePrefix("10.0.0.9/32")

	for _, label := range []uint32{16000, 17000} {
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:    mplsfibevents.ActionAdd,
			Op:        mplsfibevents.OpPush,
			FEC:       fec,
			OutLabels: []uint32{label},
			NextHop:   netip.MustParseAddr("10.0.0.5"),
		}}})
	}

	require.Len(t, mb.richAdded, 1, "first install uses Add")
	assert.Equal(t, []uint32{16000}, mb.richAdded[0].Labels)
	require.Len(t, mb.richReplaced, 1, "relabel of ze's own push uses Replace")
	assert.Equal(t, []uint32{17000}, mb.richReplaced[0].Labels, "latest label wins")
	assert.Equal(t, 1, f.mplsCountLocked(), "still a single FEC route")
}

// VALIDATES: AC-3 -- a swap entry programs an AF_MPLS route keyed by in-label,
// and remove withdraws it.
func TestHandleMPLSEntrySwapAndRemove(t *testing.T) {
	mb := newMPLSMockBackend()
	f := newFIBKernel(mb)

	f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
		Action:    mplsfibevents.ActionAdd,
		Op:        mplsfibevents.OpSwap,
		InLabel:   1000,
		OutLabels: []uint32{2000},
		NextHop:   netip.MustParseAddr("10.0.0.5"),
	}}})
	require.Len(t, mb.swapsAdded, 1)
	assert.Equal(t, uint32(1000), mb.swapsAdded[0].in)
	assert.Equal(t, []uint32{2000}, mb.swapsAdded[0].out)
	assert.True(t, f.mplsSwaps[1000])

	f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
		Action:  mplsfibevents.ActionRemove,
		Op:      mplsfibevents.OpSwap,
		InLabel: 1000,
	}}})
	require.Len(t, mb.swapsDeleted, 1)
	assert.Equal(t, uint32(1000), mb.swapsDeleted[0])
	assert.Empty(t, f.mplsSwaps)
}

// VALIDATES: a pop entry programs an AF_MPLS route with no outgoing labels.
func TestHandleMPLSEntryPop(t *testing.T) {
	mb := newMPLSMockBackend()
	f := newFIBKernel(mb)

	f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
		Action:  mplsfibevents.ActionAdd,
		Op:      mplsfibevents.OpPop,
		InLabel: 1001,
		NextHop: netip.MustParseAddr("10.0.0.9"),
	}}})
	require.Len(t, mb.swapsAdded, 1)
	assert.Equal(t, uint32(1001), mb.swapsAdded[0].in)
	assert.Empty(t, mb.swapsAdded[0].out, "pop has no outgoing label stack")
}

// VALIDATES: an out-of-range label is rejected and not programmed.
func TestHandleMPLSEntryRejectsBadLabel(t *testing.T) {
	mb := newMPLSMockBackend()
	f := newFIBKernel(mb)

	f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
		Action:  mplsfibevents.ActionAdd,
		Op:      mplsfibevents.OpSwap,
		InLabel: maxMPLSLabel + 1,
		NextHop: netip.MustParseAddr("10.0.0.5"),
	}}})
	assert.Empty(t, mb.swapsAdded, "out-of-range in-label rejected")
	assert.Empty(t, f.mplsSwaps)
}
