package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// basePeerSettingsForReload returns the "before reload" baseline. Each test
// mutates exactly one field on a second copy and asserts the reload diff notices.
func basePeerSettingsForReload() *PeerSettings {
	return NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
}

// TestPeerSettingsEqualIdenticalIsEqual verifies that reloading an unchanged
// config does not mark the peer as changed.
//
// VALIDATES: AC-3 -- a byte-identical config reload bounces nothing.
// PREVENTS: over-triggering (R-2), where a stricter diff flaps every session on
// any reload. reconcilePeersJournaled (reactor_api.go:516-546) removes and
// re-adds a changed peer, so a false "changed" verdict costs a session reset.
func TestPeerSettingsEqualIdenticalIsEqual(t *testing.T) {
	a := basePeerSettingsForReload()
	b := basePeerSettingsForReload()

	assert.True(t, peerSettingsEqual(a, b), "identical settings must compare equal (no gratuitous bounce)")
}

// TestPeerSettingsEqualDetectsImportFilterChange verifies that a peer whose ONLY
// change is its import filter chain is reported as changed by the reload diff.
//
// VALIDATES: AC-1/AC-2 -- an import-policy edit must be seen by reload.
// PREVENTS: the reload defect. peerSettingsEqual (reactor_api.go:780) omits
// ImportFilters, so reconcilePeersJournaled (reactor_api.go:498-513) puts the peer
// in neither toRemove nor toAdd; the newly parsed *PeerSettings carrying the new
// chain is dropped, and runIngressPolicyChain (filter_ordered.go:138) keeps reading
// the stale chain off the never-reassigned peer.settings pointer (peer.go:318)
// until the daemon restarts. The operator sees a successful reload and the NEW
// policy in `bgp peer <ip>`, while the datapath enforces the OLD one.
func TestPeerSettingsEqualDetectsImportFilterChange(t *testing.T) {
	a := basePeerSettingsForReload()
	b := basePeerSettingsForReload()
	b.ImportFilters = []filterapi.FilterRef{{Name: "block-bogons"}}

	assert.False(t, peerSettingsEqual(a, b), "an import filter chain change must mark the peer changed")
}

// TestPeerSettingsEqualDetectsEachSignificantField is the table-driven guard over
// every functionally significant PeerSettings field the reload diff must not ignore.
//
// VALIDATES: AC-1, AC-2, AC-4 -- each field below governs real datapath or session
// behavior, so changing it in config must take effect on reload.
// PREVENTS: silent no-op reloads. Every field listed was verified ABSENT from
// peerSettingsEqual (reactor_api.go:780-825) during the 2026-07-16 audit: that
// function compares only identity, connectivity, 7 behavior fields,
// len(StaticRoutes), and capabilities.
func TestPeerSettingsEqualDetectsEachSignificantField(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PeerSettings)
		why    string
	}{
		{
			name:   "ImportFilters",
			mutate: func(s *PeerSettings) { s.ImportFilters = []filterapi.FilterRef{{Name: "deny-all"}} },
			why:    "governs inbound policy at filter_ordered.go:138",
		},
		{
			name:   "ExportFilters",
			mutate: func(s *PeerSettings) { s.ExportFilters = []filterapi.FilterRef{{Name: "deny-all"}} },
			why:    "governs outbound policy",
		},
		{
			name:   "RouteReflectorClient",
			mutate: func(s *PeerSettings) { s.RouteReflectorClient = !s.RouteReflectorClient },
			why:    "RFC 4456 reflection; read at peer_forward_facts.go:110",
		},
		{
			name:   "ClusterID",
			mutate: func(s *PeerSettings) { s.ClusterID = 0x0A0A0A0A },
			why:    "RFC 4456 Section 7 cluster identifier",
		},
		{
			name:   "ASOverride",
			mutate: func(s *PeerSettings) { s.ASOverride = !s.ASOverride },
			why:    "rewrites outbound AS_PATH",
		},
		{
			name:   "RSClient",
			mutate: func(s *PeerSettings) { s.RSClient = !s.RSClient },
			why:    "RFC 7947 Section 2.2.2 transparent AS_PATH",
		},
		{
			name:   "RSFastPath",
			mutate: func(s *PeerSettings) { s.RSFastPath = !s.RSFastPath },
			why:    "selects the reactor-native forwarding path",
		},
		{
			name:   "AcceptSRv6PrefixSID",
			mutate: func(s *PeerSettings) { s.AcceptSRv6PrefixSID = !s.AcceptSRv6PrefixSID },
			why:    "admits PrefixSID (code 40) from EBGP peers",
		},
		{
			name:   "PropagateSRv6PrefixSID",
			mutate: func(s *PeerSettings) { s.PropagateSRv6PrefixSID = !s.PropagateSRv6PrefixSID },
			why:    "RFC 8669 Section 8 admits PrefixSID (code 40) onto an EBGP peer's egress; an ignored edit leaves the operator's new boundary unenforced until restart",
		},
		{
			name:   "NextHopMode",
			mutate: func(s *PeerSettings) { s.NextHopMode++ },
			why:    "controls next-hop rewriting for forwarded UPDATEs",
		},
		{
			name:   "MD5Key",
			mutate: func(s *PeerSettings) { s.MD5Key = "new-secret" },
			why:    "RFC 2385 TCP-MD5 key; a silently ignored rotation is a security defect",
		},
		{
			name:   "LinkLocal",
			mutate: func(s *PeerSettings) { s.LinkLocal = mustParseAddr("fe80::1") },
			why:    "RFC 2545 Section 3 IPv6 MP_REACH next-hop encoding",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := basePeerSettingsForReload()
			b := basePeerSettingsForReload()
			tc.mutate(b)

			assert.False(t, peerSettingsEqual(a, b),
				"changing %s must mark the peer changed on reload (%s)", tc.name, tc.why)
		})
	}
}
