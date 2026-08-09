// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- SHARED EXPLICIT admission (RFC 3209 6.1)
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seSession(tunnelID uint16) sessionID {
	return sessionID{endpoint: netip.MustParseAddr("10.0.0.9"), tunnelID: tunnelID, extID: 0}
}

// VALIDATES: a make-before-break replacement LSP (same SESSION, same rate)
// shares the reservation instead of double-counting it -- the bug this feature
// fixes. Two 5Gbps LSPs of one session on an 8Gbps link both admit.
func TestSEAdmissionMBBDoesNotDoubleReserve(t *testing.T) {
	ac := newAdmissionController()
	ac.setInterface("eth0", 10e9, 8e9)
	sess := seSession(1)

	require.NoError(t, ac.reserveSession("eth0", sess, 5e9), "old LSP admits")
	require.NoError(t, ac.reserveSession("eth0", sess, 5e9), "MBB replacement shares, also admits")

	ib, ok := ac.GetInterface("eth0")
	require.True(t, ok)
	assert.Equal(t, 5e9, ib.ReservedBandwidth, "session footprint counted once, not 10Gbps")
}

// VALIDATES: without SE sharing the second 5Gbps reservation would exceed the
// 8Gbps limit -- proves the test above is a real admission that SE enables.
func TestSEAdmissionDistinctSessionsDoNotShare(t *testing.T) {
	// RFC requirement: RFC3209-6-1 negative -- reservation sharing is keyed on the SESSION (endpoint, tunnel-id, ext-id; admission.go:29-41). LSPs of DIFFERENT sessions do not share (they sum and are denied), so only same-SESSION LSPs -- the make-before-break old/new pair -- share.
	ac := newAdmissionController()
	ac.setInterface("eth0", 10e9, 8e9)

	require.NoError(t, ac.reserveSession("eth0", seSession(1), 5e9))
	err := ac.reserveSession("eth0", seSession(2), 5e9)
	assert.ErrorIs(t, err, errAdmissionDenied, "distinct sessions sum and exceed 8Gbps")

	ib, _ := ac.GetInterface("eth0")
	assert.Equal(t, 5e9, ib.ReservedBandwidth, "denied reservation left no residue")
}

// VALIDATES: a replacement that requests MORE bandwidth charges only the
// increment (footprint growth), not the full new rate.
func TestSEAdmissionLargerReplacementChargesDelta(t *testing.T) {
	ac := newAdmissionController()
	ac.setInterface("eth0", 10e9, 8e9)
	sess := seSession(1)

	require.NoError(t, ac.reserveSession("eth0", sess, 3e9))
	require.NoError(t, ac.reserveSession("eth0", sess, 7e9), "footprint grows 3->7Gbps, delta 4Gbps fits")

	ib, _ := ac.GetInterface("eth0")
	assert.Equal(t, 7e9, ib.ReservedBandwidth, "footprint is the max, not 3+7")
}

// VALIDATES: the shared reservation persists until the LAST LSP of the session
// is released (the old LSP can be torn down after MBB without freeing the link
// the new LSP still uses).
func TestSEAdmissionReleaseKeepsSharedUntilLast(t *testing.T) {
	ac := newAdmissionController()
	ac.setInterface("eth0", 10e9, 8e9)
	sess := seSession(1)

	require.NoError(t, ac.reserveSession("eth0", sess, 5e9))
	require.NoError(t, ac.reserveSession("eth0", sess, 5e9))

	ac.ReleaseSession("eth0", sess, 5e9) // tear down the old LSP
	ib, _ := ac.GetInterface("eth0")
	assert.Equal(t, 5e9, ib.ReservedBandwidth, "new LSP still holds the reservation")

	ac.ReleaseSession("eth0", sess, 5e9) // tear down the new LSP
	ib, _ = ac.GetInterface("eth0")
	assert.Equal(t, 0.0, ib.ReservedBandwidth, "session fully released")
}

// VALIDATES: ReserveSession on an unconfigured interface is a no-op (accounting
// skipped, mirroring Reserve) and never denies.
func TestSEAdmissionUnconfiguredInterface(t *testing.T) {
	ac := newAdmissionController()
	assert.NoError(t, ac.reserveSession("eth99", seSession(1), 1e9))
	assert.NotPanics(t, func() { ac.ReleaseSession("eth99", seSession(1), 1e9) })
}
