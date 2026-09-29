// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- the label a RESV carries
// and the forwarding entry it programs.
//
// VALIDATES: RFC 3032 Section 2.1 item iv through handlePacket and local
// repair: a RESV whose label is Implicit NULL (3) programs a pop at a transit
// and an empty push stack at an ingress, and a bypass label of 3 is never
// stacked. 3 never appears in an imposed stack.
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// transitResvLabel sends the transit a RESV carrying label from the egress and
// returns the forwarding entries the transit programmed.
func transitResvLabel(t *testing.T, label uint32) *fakeFIB {
	t.Helper()
	e, _, fib := testEngine(t, rfc2205Transit.String(), nil)
	psb := rfc2205PSB()
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(psb, rfc2205Ingress, defaultIPTTL)})
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: label}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: rfc2205Egress, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Egress)})
	return fib
}

// RFC requirement: RFC3032-2.1-1 positive -- a transit whose downstream RESV carries the Implicit NULL label (3) pops the stack instead of replacing the top label: it programs a pop of its own in-label and no swap, so 3 never appears in the encapsulation.
func TestRFC3032ImplicitNullResvProgramsPop(t *testing.T) {
	fib := transitResvLabel(t, labelImplicitNull)

	require.Len(t, fib.popped, 1, "the transit pops for an Implicit NULL next label")
	assert.GreaterOrEqual(t, fib.popped[0], uint32(firstDynamicLabel), "the pop is keyed by the transit's own in-label")
	assert.Empty(t, fib.swapped, "no swap puts label 3 on the wire")
}

// ingressResvLabel signals a tunnel from the ingress and answers it with a RESV
// carrying label, returning the FIB the ingress programmed.
func ingressResvLabel(t *testing.T, label uint32) *fakeFIB {
	t.Helper()
	e, _, fib := testEngine(t, rfc2205Ingress.String(), nil)
	psb := rfc2205PSB()
	setupTunnel(e.log, e.table, tunnelConfig{Destination: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID, ERO: psb.ERO}, e.cfg(), e)
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: label}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: rfc2205Transit, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Transit)})
	return fib
}

// TestRFC3032IngressNeverPushesImplicitNull: an ingress whose downstream label
// is Implicit NULL programs the tunnel FEC with an empty push stack, which the
// fib-kernel backend installs as a plain IP route via the next hop.
//
// RFC requirement: RFC3032-2.1-1 positive -- an ingress whose RESV carries the Implicit NULL label (3) programs its tunnel FEC with an empty label stack: 3 never appears in the encapsulation it imposes.
func TestRFC3032IngressNeverPushesImplicitNull(t *testing.T) {
	fib := ingressResvLabel(t, labelImplicitNull)

	require.Len(t, fib.pushLabels, 1, "the ingress programs its tunnel FEC")
	assert.Empty(t, fib.pushLabels[0], "label 3 is never imposed: the FEC forwards as plain IP")
}

// TestRFC3032IngressPushesOrdinaryLabel: an ordinary downstream label is
// imposed as the one-label stack.
//
// RFC requirement: RFC3032-2.1-1 negative -- an ingress whose RESV carries an ordinary label is not treated as Implicit NULL: it pushes exactly that label.
func TestRFC3032IngressPushesOrdinaryLabel(t *testing.T) {
	fib := ingressResvLabel(t, 16050)

	require.Len(t, fib.pushLabels, 1, "the ingress programs its tunnel FEC")
	assert.Equal(t, []uint32{16050}, fib.pushLabels[0], "the ordinary label is imposed")
}

// TestRFC3032ImplicitNullBypassLabelNotStacked: a facility bypass whose own
// label is Implicit NULL stacks only the protected label on local repair.
//
// RFC requirement: RFC3032-2.1-1 positive -- a PLR whose bypass tunnel label is Implicit NULL reprograms the protected in-label with the protected label alone, never with 3 on top of it.
func TestRFC3032ImplicitNullBypassLabelNotStacked(t *testing.T) {
	e, _, fib := plrEngine(t)
	protectedIn := armAndUpProtected(t, e)
	bringBypassUp(t, e, labelImplicitNull, netip.MustParseAddr("10.0.1.3"))

	e.handleLinkDown("eth0")

	require.Len(t, fib.backups, 1, "the protected link's failure switches the LSP")
	assert.Equal(t, protectedIn, fib.backups[0].in)
	assert.Equal(t, []uint32{18000}, fib.backups[0].out, "only the protected label is stacked")
}

// RFC requirement: RFC3032-2.1-1 negative -- a RESV carrying an ordinary label is not treated as Implicit NULL: the transit swaps its in-label to that label and programs no pop.
func TestRFC3032OrdinaryResvLabelSwapped(t *testing.T) {
	fib := transitResvLabel(t, 16050)

	require.Len(t, fib.swapped, 1, "the transit swaps to the downstream label")
	assert.Equal(t, uint32(16050), fib.swapped[0].out)
	assert.Empty(t, fib.popped, "an ordinary label is never popped")
}
