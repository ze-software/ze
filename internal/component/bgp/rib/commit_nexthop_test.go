package rib

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// unencodableNextHopRoutes returns one route per rail shape the guard has to
// cover, each carrying a next hop with no wire form.
func unencodableNextHopRoutes() []struct {
	name    string
	grouped bool
	route   *Route
} {
	attrs := []attribute.Attribute{attribute.Origin(0)}
	return []struct {
		name    string
		grouped bool
		route   *Route
	}{
		{"grouped-ipv6", true, NewRoute(newIPv6NLRI("2001:db8::/32"), netip.Addr{}, attrs)},
		{"ungrouped-ipv6", false, NewRoute(newIPv6NLRI("2001:db8::/32"), netip.Addr{}, attrs)},
		{"grouped-vpnv4", true, NewRoute(newVPNv4NLRI("192.168.1.0/24"), netip.Addr{}, attrs)},
		{"ungrouped-vpnv4", false, NewRoute(newVPNv4NLRI("10.42.0.0/16"), netip.Addr{}, attrs)},
	}
}

// TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm covers the deferred-commit
// rail's half of the MP_REACH next-hop guard.
//
// The rail sizes each attribute with attrSize and writes it with
// attribute.WriteAttrTo, never through CheckedWriteTo, so
// (*MPReachNLRI).ValidateNextHops was never asked. It was protected only by an
// accident: the length arithmetic disagreed with the write for the zero
// netip.Addr, and packAttributesWithASPath refuses a block whose size query and
// write disagree. Deriving both from netip.Addr.AsSlice made them agree at zero,
// which silenced the accidental refusal.
//
// Measured on this rail before the guard existed. An IPv6 unicast route whose
// next hop is the zero netip.Addr produced this MP_REACH_NLRI value:
//
//	00 02 01 00 00 20 20 01 0d b8
//	 └ AFI=0x0002 SAFI=0x01 NHLen=0x00 Reserved=0x00 NLRI=2001:db8::/32
//
// and a VPNv4 route with the same next hop produced this one:
//
//	00 01 80 18 00*24 00 18 c0 a8 01
//	 └ AFI=0x0001 SAFI=0x80 NHLen=0x18 then 24 zero octets
//
// attribute.ValidNextHopLens admits no zero length for any AFI/SAFI pair, so a
// peer treats the first as malformed and resets the session (RFC 7606 Section
// 7.11). The second is worse to diagnose: a syntactically well-formed field of
// 24 zeros, a Route Distinguisher with no router behind it.
//
// VALIDATES: Commit returns attribute.ErrUnencodableNextHop and hands the sender
// no UPDATE at all, on the standard branch and on the VPN branch, grouped and
// ungrouped.
// PREVENTS: the deferred-commit rail emitting a Length of Next Hop Network
// Address octet of 0x00, or a next-hop field of 24 zero octets.
//
// RFC requirement: RFC4760-3-2 negative -- the Length of Next Hop Network Address
// field is what identifies the next hop's network-layer protocol, and neither a
// zero there nor a Route Distinguisher with no address behind it identifies one.
// This drives the rail's own entry point, (*CommitService).Commit
// (internal/component/bgp/rib/commit.go), rather than the attribute or the
// builder, because that rail is the third one with no checked write between it
// and the socket.
func TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm(t *testing.T) {
	for _, testCase := range unencodableNextHopRoutes() {
		t.Run(testCase.name, func(t *testing.T) {
			sender := &mockUpdateSender{}
			cs := NewCommitService(sender, testContext(65000, 65001, true), testCase.grouped)

			stats, err := cs.Commit([]*Route{testCase.route}, CommitOptions{})

			require.Error(t, err, "Commit must refuse a next hop with no wire form")
			assert.ErrorIs(t, err, attribute.ErrUnencodableNextHop)
			assert.Empty(t, sender.updates, "no UPDATE may reach the sender")
			assert.Zero(t, stats.UpdatesSent)
			assert.Zero(t, stats.RoutesAnnounced)
		})
	}
}

// TestCommitRefusalOfAnUnencodableNextHopIsLogged covers the visibility half of
// the same guard.
//
// (*reactorAPIAdapter).SendRoutes (component/bgp/reactor/reactor_api_batch.go)
// discards the error Commit returns and continues to the next peer, so the log
// record is what reaches an operator. A guard that fails closed in silence would
// leave a peer skipped for no stated reason.
//
// It is a separate test from the refusal above, and carries no RFC tag, because
// the RFC governs the wire and this assertion governs a Ze log line. Keeping the
// two apart stops an edit to the log wording from touching tagged evidence.
//
// VALIDATES: exactly one Warn record, naming the family and the next hop.
// PREVENTS: a silent refusal (ai/rules/evidence.md, fail closed or say
// something).
func TestCommitRefusalOfAnUnencodableNextHopIsLogged(t *testing.T) {
	capture := captureRecords(t)
	sender := &mockUpdateSender{}
	cs := NewCommitService(sender, testContext(65000, 65001, true), true)

	route := NewRoute(newIPv6NLRI("2001:db8::/32"), netip.Addr{}, []attribute.Attribute{attribute.Origin(0)})
	_, err := cs.Commit([]*Route{route}, CommitOptions{})
	require.Error(t, err)

	require.Len(t, capture.records, 1, "exactly one record for one refused announce")
	record := capture.records[0]
	assert.Equal(t, slog.LevelWarn, record.Level)

	logged := map[string]string{}
	record.Attrs(func(a slog.Attr) bool {
		logged[a.Key] = a.Value.String()
		return true
	})
	assert.Equal(t, family.IPv6Unicast.String(), logged["family"], "the record names the family")
	assert.Contains(t, logged, "nextHop", "the record names the next hop")
}

// TestCommitVPNAnnounceCarriesTheRFC4364NextHop pins the whole MP_REACH_NLRI
// value a VPNv4 announce puts on the wire, field by field.
//
// The rib rail carried its own SAFI 128 encoder because the core one was once
// blind to the Route Distinguisher. It is not any more: (*MPReachNLRI).nextHopOctets
// adds RDSize for SAFIVPN and WriteTo writes the eight zero octets before each
// address. This test is the control that let the duplicate be deleted: it passed
// against the old encoder and passes against the core one, so no octet moved
// under the deletion.
//
// VALIDATES: AFI 1, SAFI 128, a next-hop length of 12, an 8-octet zero Route
// Distinguisher, the four octets of the address, a zero Reserved octet, then the
// NLRI (RFC 4364 Section 4.3.4, RFC 4760 Section 3).
// PREVENTS: a VPN next hop that loses its RD, gains a second one, or is sized
// from the address family rather than from the address.
//
// RFC requirement: RFC4760-3-2 positive -- 12 octets of RD-plus-IPv4 is the
// length that names IPv4 as the next hop's network-layer protocol under SAFI 128.
func TestCommitVPNAnnounceCarriesTheRFC4364NextHop(t *testing.T) {
	sender := &mockUpdateSender{}
	cs := NewCommitService(sender, testContext(65000, 65000, true), true)

	routes := []*Route{NewRoute(
		newVPNv4NLRI("192.168.1.0/24"),
		netip.MustParseAddr("10.0.0.1"),
		[]attribute.Attribute{attribute.Origin(0)},
	)}

	_, err := cs.Commit(routes, CommitOptions{})
	require.NoError(t, err)
	require.Len(t, sender.updates, 1)

	update := sender.updates[0]
	assert.Nil(t, update.NLRI, "a VPNv4 announce carries its NLRI inside MP_REACH_NLRI")

	_, flags, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	require.True(t, found, "MP_REACH_NLRI must be present")
	assert.Equal(t, attribute.FlagOptional, flags)

	want := []byte{
		0x00, 0x01, // AFI: IPv4 (RFC 4760 Section 3)
		0x80,                   // SAFI: 128, MPLS VPN (RFC 4364)
		0x0c,                   // Length of Next Hop Network Address: RD(8) + IPv4(4)
		0, 0, 0, 0, 0, 0, 0, 0, // Route Distinguisher, zero (RFC 4364 Section 4.3.4)
		10, 0, 0, 1, // Network Address of Next Hop: 10.0.0.1
		0x00,              // Reserved, "MUST be set to 0"
		0x18, 192, 168, 1, // NLRI: 192.168.1.0/24
	}
	assert.Equal(t, want, value)
}

// warnCapture collects the slog records a test provokes, so the guard's Warn can
// be asserted rather than assumed. It records every level and lets the test say
// which one it expected.
type warnCapture struct {
	records []slog.Record
}

func (w *warnCapture) Enabled(context.Context, slog.Level) bool { return true }

// Handle takes slog.Record by value because slog.Handler fixes this signature;
// gocritic's hugeParam cannot be satisfied without breaking the interface.
//
//nolint:gocritic // slog.Handler requires the value receiver for record
func (w *warnCapture) Handle(_ context.Context, record slog.Record) error {
	w.records = append(w.records, record.Clone())
	return nil
}

func (w *warnCapture) WithAttrs([]slog.Attr) slog.Handler { return w }

func (w *warnCapture) WithGroup(string) slog.Handler { return w }

// captureRecords redirects the slog default to a warnCapture for the test's
// duration, mirroring captureWarnPeers in the reactor package. No test in this
// package calls t.Parallel, so the default logger is not shared with a
// concurrent test.
func captureRecords(t *testing.T) *warnCapture {
	t.Helper()
	capture := &warnCapture{}
	old := slog.Default()
	slog.SetDefault(slog.New(capture))
	t.Cleanup(func() { slog.SetDefault(old) })
	return capture
}
