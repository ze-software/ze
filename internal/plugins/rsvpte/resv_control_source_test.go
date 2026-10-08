// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- the reservation error and
// teardown paths, and which neighbour each one is accepted from.
// Related: rfc2205_resv_error_test.go -- the RFC 2205 ResvErr and ResvTear
// obligations these source checks sit in front of.
//
// VALIDATES: a ResvErr is acted on only when its IP source is the previous hop
// of the held reservation, and a ResvTear only when its IP source is the next
// hop the reservation came from (samePeer in handleResvErr and handleResvTear).
// PREVENTS: a host that is not the neighbour holding the state tearing down a
// transit's reservation, blockading it, or having its error relayed to the
// receiver, by sending a message whose objects all match.
//
// The tests carry no RFC requirement tag. RFC 2205 states where these messages
// are sent (Section 3.1.6: "its IP destination address will be the unicast
// address of a previous hop"; Section 3.1.8: "At each hop, the IP destination
// address is the unicast address of a next-hop node"), and it states what a
// ResvTear must match (SESSION, STYLE, FILTER_SPEC and the RSVP_HOP LIH). It
// states no requirement that a receiver refuse one from another source, so the
// check proven here is Ze's own guard and no rfc/short/rfc2205.md row names it.
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sourceStranger is an address that is neither neighbour of the transit and
// that no IGP database attributes to either of them.
var sourceStranger = netip.MustParseAddr("10.0.0.66")

// reservationSnapshot is the reservation, label and state a ResvErr or a
// ResvTear could change at the transit.
type reservationSnapshot struct {
	rsb      resvStateBlock
	reserved bool
	inLabel  uint32
	outLabel uint32
	state    lspState
}

// transitWithReservation returns a transit holding path state from the
// ingress and reservation state from the egress, with the FIB it programmed
// and a snapshot of the reservation the RESV installed.
func transitWithReservation(t *testing.T) (*engine, *fakeTransport, *fakeFIB, *pathStateBlock, reservationSnapshot) {
	t.Helper()
	e, ft, fib := testEngine(t, rfc2205Transit.String(), nil)
	psb := rfc2205PSB()
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(psb, rfc2205Ingress, defaultIPTTL)})
	e.handlePacket(Packet{Src: rfc2205Egress, Payload: resvFrom(psb, StyleSharedExplicit, resvErrFlowSpec)})
	before := snapshotReservation(t, e, psb)
	require.Equal(t, LSPStateUp, before.state, "the RESV brought the transit up")
	require.NotZero(t, before.inLabel, "the transit allocated an incoming label")
	require.Len(t, fib.swapped, 1, "the transit programmed its swap")
	return e, ft, fib, psb, before
}

// snapshotReservation copies the reservation state of psb's LSP.
func snapshotReservation(t *testing.T, e *engine, psb *pathStateBlock) reservationSnapshot {
	t.Helper()
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok, "the transit holds the LSP")
	lsp.mu.Lock()
	defer lsp.mu.Unlock()
	require.NotNil(t, lsp.RSB, "the transit holds the reservation")
	return reservationSnapshot{
		rsb:      *lsp.RSB,
		reserved: lsp.Reserved,
		inLabel:  lsp.InLabel,
		outLabel: lsp.OutLabel,
		state:    lsp.State,
	}
}

// admissionFailure returns the ResvErr a node upstream of the transit sends
// for psb's sender: an admission control failure without the InPlace flag,
// which blockades the reservation and is relayed to the receiver.
func admissionFailure(psb *pathStateBlock) []byte {
	es := errorSpec{ErrorNode: rfc2205Ingress, ErrorCode: ErrCodeAdmissionControlFailure, ErrorValue: ErrValueRequestedBandwidth}
	return reservationError(psb, psb.SenderTemplate, StyleSharedExplicit, es)
}

// TestResvErrFromWrongSourceIgnored sends a transit holding a reservation a
// ResvErr whose SESSION, STYLE and FILTER_SPEC all match it, from a stranger
// and from the egress (a real neighbour, but the next hop rather than the
// previous one), and checks neither message reaches the reservation or the
// receiver.
func TestResvErrFromWrongSourceIgnored(t *testing.T) {
	for _, source := range []netip.Addr{sourceStranger, rfc2205Egress} {
		e, ft, _, psb, before := transitWithReservation(t)

		e.handlePacket(Packet{Src: source, Payload: admissionFailure(psb)})

		errors, _ := decodedSent(t, ft, MsgTypeResvErr)
		assert.Empty(t, errors, "source %s: the ResvErr is not relayed to the receiver", source)
		assert.Equal(t, before, snapshotReservation(t, e, psb),
			"source %s: the reservation, its blockade, labels and state are untouched", source)
	}
}

// TestResvErrFromPreviousHopActedOn is the positive control of
// TestResvErrFromWrongSourceIgnored: the same ResvErr from the previous hop
// blockades the reservation and is relayed to the egress.
func TestResvErrFromPreviousHopActedOn(t *testing.T) {
	e, ft, _, psb, before := transitWithReservation(t)
	require.Zero(t, before.rsb.Blockade, "no blockade before the error")

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: admissionFailure(psb)})

	errors, targets := decodedSent(t, ft, MsgTypeResvErr)
	require.Len(t, errors, 1, "the ResvErr is relayed")
	assert.Equal(t, rfc2205Egress, targets[0], "to the next hop the Resv came from")
	after := snapshotReservation(t, e, psb)
	assert.InDelta(t, resvErrFlowSpec.TokenRate, after.rsb.Blockade.TokenRate, 1, "the failed FLOWSPEC is blockaded")
	assert.False(t, after.rsb.BlockadeUntil.IsZero(), "the blockade has an expiry")
}

// TestResvTearFromWrongSourceIgnored sends a transit holding a reservation a
// ResvTear whose SESSION, STYLE, FILTER_SPEC and RSVP_HOP all match it, from a
// stranger and from the ingress (a real neighbour, but the previous hop rather
// than the next one), and checks the reservation, its label and its forwarding
// entry stay and no ResvTear goes upstream.
func TestResvTearFromWrongSourceIgnored(t *testing.T) {
	for _, source := range []netip.Addr{sourceStranger, rfc2205Ingress} {
		e, ft, fib, psb, before := transitWithReservation(t)

		e.handlePacket(Packet{Src: source, Payload: reservationTear(psb, rsvpHop{NextHop: rfc2205Egress})})

		assert.Zero(t, ft.countByType(MsgTypeResvTear), "source %s: no ResvTear is forwarded", source)
		assert.Empty(t, fib.removedSwap, "source %s: the swap entry stays", source)
		assert.Equal(t, before, snapshotReservation(t, e, psb),
			"source %s: the reservation, labels and state are untouched", source)
	}
}

// TestResvTearFromNextHopActedOn is the positive control of
// TestResvTearFromWrongSourceIgnored: the same ResvTear from the next hop the
// reservation came from removes it, withdraws its swap and is sent upstream.
func TestResvTearFromNextHopActedOn(t *testing.T) {
	e, ft, fib, psb, before := transitWithReservation(t)

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: reservationTear(psb, rsvpHop{NextHop: rfc2205Egress})})

	tears, targets := decodedSent(t, ft, MsgTypeResvTear)
	require.Len(t, tears, 1, "the ResvTear goes upstream")
	assert.Equal(t, rfc2205Ingress, targets[0])
	assert.Equal(t, []uint32{before.inLabel}, fib.removedSwap, "the swap entry is withdrawn")
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok)
	lsp.mu.Lock()
	defer lsp.mu.Unlock()
	assert.Nil(t, lsp.RSB, "the reservation is removed")
	assert.Zero(t, lsp.InLabel, "the incoming label is released")
	assert.Equal(t, LSPStatePathReceived, lsp.State)
}
