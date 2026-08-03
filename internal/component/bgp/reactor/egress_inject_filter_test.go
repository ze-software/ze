package reactor

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exportFilterForBody is the single egress gate for originated / injected /
// redistributed / replayed routes (egress_inject_filter.go). Its guard used to
// fuse three unrelated conditions into one permissive early return:
//
//	if facts == nil || len(facts.exportFilters) == 0 || r.api == nil { return false, nil }
//
// The first two are legitimate accepts (no session / no export policy); the
// third -- a nil API server while the peer HAS export filters -- is a guard
// MISS, and returning (false, nil) sent the route to the wire UNFILTERED and
// silently, the exact fail-open siblings policyFilterFunc (filter_chain.go) and
// default-originate (peer_initial_sync.go) already reject loudly. These tests
// drive exportFilterForBody directly (the entry point that triggers the guard,
// per evidence.md) and pin each of the three split cases.
//
// Spec: plan/spec-fixit-private-asn-leak-deferred-nil-api-fail-open.md (AC-1, AC-2).

// peerWithExportFilters builds a Peer whose forwarding facts are established
// (non-nil) and carry the given export filter chain, without going through a
// live session. The facts pointer is what forwardFacts() reads.
func peerWithExportFilters(t *testing.T, addr string, filters []filterapi.FilterRef) *Peer {
	t.Helper()
	peer := NewPeer(&PeerSettings{
		Address: netip.MustParseAddr(addr),
		LocalAS: 65000,
		PeerAS:  65001,
	})
	peer.fwdFacts.Store(&peerForwardFacts{
		addrStr:       addr,
		exportFilters: filters,
	})
	return peer
}

// captureWarnPeers redirects the slog default to a warnRecorder for the test's
// duration and returns it, mirroring TestSignalPeerAPIReadyUnknownPeerWarns.
func captureWarnPeers(t *testing.T) *warnRecorder {
	t.Helper()
	rec := &warnRecorder{}
	old := slog.Default()
	slog.SetDefault(slog.New(rec))
	t.Cleanup(func() { slog.SetDefault(old) })
	return rec
}

// AC-1: a peer WITH export filters and a nil API server must NOT send the route
// unfiltered. The gate suppresses (suppress == true) AND says something (Warn
// naming the peer), matching the house answer for r.api == nil. Before the fix
// this returned (false, nil) with no log -- the silent fail-open.
func TestExportFilterForBodyNilAPIWithExportFiltersSuppressesAndWarns(t *testing.T) {
	rec := captureWarnPeers(t)

	r := &Reactor{} // r.api is nil: the whole subject
	peer := peerWithExportFilters(t, "10.0.0.1", []filterapi.FilterRef{{Name: "reject-private-asn"}})

	suppress, override := r.exportFilterForBody(peer, []byte{0x00})

	require.True(t, suppress,
		"nil API server with export filters configured must suppress the route (fail closed), not accept it unfiltered")
	assert.Nil(t, override,
		"a suppressed route carries no override body")
	require.Contains(t, rec.warnedPeers(), "10.0.0.1",
		"the guard miss must be logged at Warn naming the peer, not silently dropped")
}

// AC-2: a peer with NO export filters and a nil API server is a legitimate
// accept (no export policy to run). It must still return (false, nil) and must
// NOT warn -- this is not a guard miss.
func TestExportFilterForBodyNoExportFiltersAccepts(t *testing.T) {
	rec := captureWarnPeers(t)

	r := &Reactor{}
	peer := peerWithExportFilters(t, "10.0.0.2", nil)

	suppress, override := r.exportFilterForBody(peer, []byte{0x00})

	assert.False(t, suppress,
		"a peer with no export policy is a legitimate accept, not a guard miss")
	assert.Nil(t, override)
	assert.NotContains(t, rec.warnedPeers(), "10.0.0.2",
		"no export policy configured is not a miss -- it must not warn")
}

// Fused-condition split: nil facts means the peer is not established
// (peer_forward_facts.go:35). There is no session on which a route reaches the
// wire, so this is an absent precondition, not a guard miss: accept, no warn.
func TestExportFilterForBodyNotEstablishedAccepts(t *testing.T) {
	rec := captureWarnPeers(t)

	r := &Reactor{}
	peer := NewPeer(&PeerSettings{
		Address: netip.MustParseAddr("10.0.0.3"),
		LocalAS: 65000,
		PeerAS:  65001,
	})
	require.Nil(t, peer.forwardFacts(), "precondition: peer is not established (nil facts)")

	suppress, override := r.exportFilterForBody(peer, []byte{0x00})

	assert.False(t, suppress,
		"a not-established peer has no wire session -- absent precondition, not a miss")
	assert.Nil(t, override)
	assert.NotContains(t, rec.warnedPeers(), "10.0.0.3",
		"not established is not a guard miss -- it must not warn")
}
