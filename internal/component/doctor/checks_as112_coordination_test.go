// Detail: checks_as112_coordination.go -- unit tests for both checks

// VALIDATES: AC-10 (doctor-as112-watchdog-missing-withdraw fires when an
// update block carries an AS112 covering prefix without watchdog{withdraw
// true}) and AC-11 (doctor-as112-global-origin-uncoordinated fires when
// asn.local 112 + replace-as targets a non-private remote ASN).
// PREVENTS: an operator's worked-example deviation announcing AS112 routes
// before health is proven, or silently becoming an uncoordinated global
// AS112 origin, going undetected by `ze doctor`.

package doctor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func newAS112UpdateTree(nlriContent, watchdogWithdraw string) *config.Tree {
	update := config.NewTree()
	nlri := config.NewTree()
	nlri.Set("content", nlriContent)
	update.AddListEntry("nlri", "ipv4/unicast", nlri)
	if watchdogWithdraw != "" {
		wd := update.GetOrCreateContainer("watchdog")
		wd.Set("name", "as112")
		wd.Set("withdraw", watchdogWithdraw)
	}
	return update
}

func TestCheckAS112WatchdogWithdraw_NoBGP(t *testing.T) {
	tree := config.NewTree()
	assert.Empty(t, checkAS112WatchdogWithdraw(tree))
}

func TestCheckAS112WatchdogWithdraw_NonAS112PrefixNoWithdraw_NoDiagnostic(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("update", "u1", newAS112UpdateTree("add 10.0.0.0/24", ""))
	assert.Empty(t, checkAS112WatchdogWithdraw(tree))
}

func TestCheckAS112WatchdogWithdraw_AS112PrefixMissingWithdraw_Flagged(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("update", "u1", newAS112UpdateTree("add 192.175.48.0/24", ""))

	diags := checkAS112WatchdogWithdraw(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-as112-watchdog-missing-withdraw", diags[0].Code)
	assert.Contains(t, diags[0].Message, "bgp/update/u1")
}

func TestCheckAS112WatchdogWithdraw_AS112PrefixWithdrawFalse_Flagged(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("update", "u1", newAS112UpdateTree("add 192.31.196.0/24", "false"))

	diags := checkAS112WatchdogWithdraw(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-as112-watchdog-missing-withdraw", diags[0].Code)
}

func TestCheckAS112WatchdogWithdraw_AS112PrefixWithWithdraw_NoDiagnostic(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("update", "u1", newAS112UpdateTree("add 2620:4f:8000::/48", "true"))
	assert.Empty(t, checkAS112WatchdogWithdraw(tree))
}

func TestCheckAS112WatchdogWithdraw_PeerLevelUpdate_Flagged(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	peer := config.NewTree()
	peer.AddListEntry("update", "u1", newAS112UpdateTree("add 2001:4:112::/48", ""))
	bgp.AddListEntry("peer", "peer1", peer)

	diags := checkAS112WatchdogWithdraw(tree)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "bgp/peer/peer1/update/u1")
}

func TestCheckAS112WatchdogWithdraw_DelOperation_NoDiagnostic(t *testing.T) {
	// REGRESSION: a "del" entry for an AS112 covering prefix is never
	// announced (production watchdog pool builder skips non-"add" ops), so
	// it must not be treated as an unwithdrawn announcement.
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("update", "u1", newAS112UpdateTree("del 192.175.48.0/24", ""))
	assert.Empty(t, checkAS112WatchdogWithdraw(tree))
}

func TestCheckAS112WatchdogWithdraw_NonCanonicalPrefixForm_Flagged(t *testing.T) {
	// REGRESSION: a non-canonical but equivalent form of an AS112 covering
	// prefix (fully-expanded IPv6, uppercase hex) must still be recognized,
	// not missed by exact-literal-string matching.
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("update", "u1", newAS112UpdateTree("add 2620:004F:8000:0000:0000:0000:0000:0000/48", ""))

	diags := checkAS112WatchdogWithdraw(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-as112-watchdog-missing-withdraw", diags[0].Code)
}

func TestCheckAS112WatchdogWithdraw_IPv4In6EmbeddedForm_Flagged(t *testing.T) {
	// REGRESSION: plain netip.Prefix equality treats an IPv4-in-IPv6-embedded
	// address (Is4In6()) as never equal to the native IPv4 form (Is4()),
	// even for the same network -- normalizeIPv4In6 must unmap it first.
	// bits=120 = 96 (embedding prefix) + 24 (the covering prefix's own length).
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("update", "u1", newAS112UpdateTree("add ::ffff:192.175.48.0/120", ""))

	diags := checkAS112WatchdogWithdraw(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-as112-watchdog-missing-withdraw", diags[0].Code)
}

func TestCheckAS112WatchdogWithdraw_GroupPeerLevelUpdate_Flagged(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	group := config.NewTree()
	peer := config.NewTree()
	peer.AddListEntry("update", "u1", newAS112UpdateTree("add 192.175.48.0/24", ""))
	group.AddListEntry("peer", "peer1", peer)
	bgp.AddListEntry("group", "g1", group)

	diags := checkAS112WatchdogWithdraw(tree)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "bgp/group/g1/peer/peer1/update/u1")
}

func newAS112PeerTree(localASN, remoteASN string, replaceAs bool) *config.Tree {
	peer := config.NewTree()
	session := peer.GetOrCreateContainer("session")
	asn := session.GetOrCreateContainer("asn")
	if localASN != "" {
		asn.Set("local", localASN)
	}
	if remoteASN != "" {
		asn.Set("remote", remoteASN)
	}
	if replaceAs {
		asn.SetSlice("local-options", []string{"replace-as"})
	}
	return peer
}

func TestCheckAS112GlobalOriginCoordination_NoBGP(t *testing.T) {
	tree := config.NewTree()
	assert.Empty(t, checkAS112GlobalOriginCoordination(tree))
}

func TestCheckAS112GlobalOriginCoordination_NotAS112Local_NoDiagnostic(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("65000", "65001", true))
	assert.Empty(t, checkAS112GlobalOriginCoordination(tree))
}

func TestCheckAS112GlobalOriginCoordination_NoReplaceAs_NoDiagnostic(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("112", "65001", false))
	assert.Empty(t, checkAS112GlobalOriginCoordination(tree))
}

func TestCheckAS112GlobalOriginCoordination_PrivateRemoteASN_NoDiagnostic(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("112", "65001", true))
	assert.Empty(t, checkAS112GlobalOriginCoordination(tree))
}

func TestCheckAS112GlobalOriginCoordination_PublicRemoteASN_Flagged(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	// 15169 (Google) is a real public ASN, well outside the RFC 6996
	// Section 4 private-use range (64512-65534) -- unlike 65002/65010,
	// which are inside that range and would NOT be flagged.
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("112", "15169", true))

	diags := checkAS112GlobalOriginCoordination(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-as112-global-origin-uncoordinated", diags[0].Code)
	assert.Contains(t, diags[0].Message, "bgp/peer/p1")
	assert.Contains(t, diags[0].Message, "15169")
}

func TestCheckAS112GlobalOriginCoordination_GroupPeerOverride_Flagged(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	group := config.NewTree()
	groupSession := group.GetOrCreateContainer("session")
	groupSession.GetOrCreateContainer("asn").Set("local", "112")
	groupSession.GetOrCreateContainer("asn").SetSlice("local-options", []string{"replace-as"})

	peer := config.NewTree()
	peer.GetOrCreateContainer("session").GetOrCreateContainer("asn").Set("remote", "13335")
	group.AddListEntry("peer", "p1", peer)
	bgp.AddListEntry("group", "g1", group)

	diags := checkAS112GlobalOriginCoordination(tree)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "bgp/group/g1/peer/p1")
	assert.Contains(t, diags[0].Message, "13335")
}

func TestCheckAS112GlobalOriginCoordination_StandalonePeerUsesGlobalLocalDefault_Flagged(t *testing.T) {
	// REGRESSION: PeersFromTree (internal/component/bgp/reactor/config.go)
	// seeds a standalone peer's local AS from bgp/session/asn/local when the
	// peer sets no override of its own -- a real third inheritance tier
	// below group and peer that the check previously never consulted.
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.GetOrCreateContainer("session").GetOrCreateContainer("asn").Set("local", "112")

	peer := config.NewTree()
	session := peer.GetOrCreateContainer("session").GetOrCreateContainer("asn")
	session.Set("remote", "15169")
	session.SetSlice("local-options", []string{"replace-as"})
	bgp.AddListEntry("peer", "p1", peer)

	diags := checkAS112GlobalOriginCoordination(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-as112-global-origin-uncoordinated", diags[0].Code)
	assert.Contains(t, diags[0].Message, "bgp/peer/p1")
}

func TestCheckAS112GlobalOriginCoordination_GroupPeerUsesGlobalLocalDefault_Flagged(t *testing.T) {
	// Same global-default fallback, but for a peer nested in a group where
	// neither the group nor the peer sets asn.local.
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.GetOrCreateContainer("session").GetOrCreateContainer("asn").Set("local", "112")

	group := config.NewTree()
	peer := config.NewTree()
	session := peer.GetOrCreateContainer("session").GetOrCreateContainer("asn")
	session.Set("remote", "13335")
	session.SetSlice("local-options", []string{"replace-as"})
	group.AddListEntry("peer", "p1", peer)
	bgp.AddListEntry("group", "g1", group)

	diags := checkAS112GlobalOriginCoordination(tree)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "bgp/group/g1/peer/p1")
}

func TestCheckAS112GlobalOriginCoordination_GroupPeerWithoutRemote_NoDiagnostic(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("112", "", true))
	assert.Empty(t, checkAS112GlobalOriginCoordination(tree))
}

func TestCheckAS112GlobalOriginCoordination_RemoteASNBoundaries(t *testing.T) {
	// VALIDATES: RFC 6996 Section 4 Private Use ASN range boundaries
	// (16-bit 64512-65534, 32-bit 4200000000-4294967294) are applied
	// inclusively/exclusively at every edge, not off-by-one.
	cases := []struct {
		name    string
		remote  string
		flagged bool
	}{
		{"16bit-below-range", "64511", true},
		{"16bit-range-start", "64512", false},
		{"16bit-range-end", "65534", false},
		{"16bit-above-range", "65535", true},
		{"32bit-below-range", "4199999999", true},
		{"32bit-range-start", "4200000000", false},
		{"32bit-range-end", "4294967294", false},
		{"32bit-above-range", "4294967295", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := config.NewTree()
			bgp := tree.GetOrCreateContainer("bgp")
			bgp.AddListEntry("peer", "p1", newAS112PeerTree("112", tc.remote, true))

			diags := checkAS112GlobalOriginCoordination(tree)
			if tc.flagged {
				require.Len(t, diags, 1)
				assert.Equal(t, "doctor-as112-global-origin-uncoordinated", diags[0].Code)
			} else {
				assert.Empty(t, diags)
			}
		})
	}
}

// TestDoctorAS112CoordinationFunctional exercises both checks through the
// real user entry point (ze doctor --json <config>), not just the check
// functions directly, per ai/rules/doctor-checks.md's functional-test
// requirement. The config text omits the mandatory watchdog{withdraw true}
// marker (AC-10) and sets asn.local 112 + replace-as against a public
// remote ASN (AC-11) on the same peer, so both codes must appear together.
func TestDoctorAS112CoordinationFunctional(t *testing.T) {
	const cfg = `
bgp {
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.1
			}
		}
		session {
			asn {
				local 112
				local-options [ replace-as ]
				remote 15169
			}
		}
		update {
			attribute {
				origin igp
			}
			nlri {
				ipv4/unicast add 192.175.48.0/24
			}
		}
	}
}
`
	cfgPath := writeTestConfig(t, cfg)
	out := captureStdout(t, func() {
		code := Run([]string{"--json", cfgPath})
		assert.Equal(t, 0, code, "advisory warnings must not fail readiness")
	})

	var result diagnostic.DoctorResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))

	var codes []string
	for i := range result.Diagnostics {
		codes = append(codes, result.Diagnostics[i].Code)
	}
	assert.Contains(t, codes, "doctor-as112-watchdog-missing-withdraw")
	assert.Contains(t, codes, "doctor-as112-global-origin-uncoordinated")
}

func TestDoctorAS112CoordinationCodesRegistered(t *testing.T) {
	for _, code := range []string{
		"doctor-as112-watchdog-missing-withdraw",
		"doctor-as112-global-origin-uncoordinated",
	} {
		meta := diagnostic.Lookup(code)
		require.NotNil(t, meta, "%s code must be registered", code)
		assert.NotEmpty(t, meta.Title)
		assert.NotEmpty(t, meta.Description)
	}
}
