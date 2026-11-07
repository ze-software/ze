// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- a dual-family proposal narrowed to one family on a VTI
// Related: rekey.go -- proposeChildTSPayloads, the proposal under test
// Related: ts_narrow.go -- recordInitiatorSelectorsIn, the answer check under test
// Related: child.go -- createFirstChildSA and childPolicies, the install under test

package engine

import (
	"errors"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// dualFamilyCPEConfig is the home CPE's management peer as the plugin receives it: one
// Child SA bound to vti0, proposing IPv4 0.0.0.0/0 to 0.0.0.0/0 first and IPv6 ::/0 to
// ::/0 second, over an IPv4 outer path. The guide's "Traffic selectors and narrowing"
// section names this list as the dual-stack form.
//
// "@traffic-selector" is the entry order Tree.ToPluginMap delivers beside any list of
// two or more entries. config.TreeFromPluginMap records a list's order only from it, and
// GetListOrdered, which parseTrafficSelectors reads, answers no entries without it.
const dualFamilyCPEConfig = `{
  "vpn": {
    "ipsec": {
      "interface": "eth0",
      "ike-group": {
        "IKE-1": {
          "key-exchange": "ikev2",
          "lifetime": "28800",
          "proposal": {"1": {"encryption": "aes256", "hash": "sha256", "dh-group": "14"}}
        }
      },
      "esp-group": {
        "ESP-1": {
          "lifetime": "3600",
          "pfs": "disable",
          "proposal": {"1": {"encryption": "aes256", "hash": "sha256"}}
        }
      },
      "site-to-site": {
        "peer": {
          "cpe": {
            "ike-group": "IKE-1",
            "esp-group": "ESP-1",
            "connection-type": "initiate",
            "mode": "tunnel",
            "local-address": "192.0.2.10",
            "remote-address": "198.51.100.1",
            "vti": {"bind": "vti0"},
            "@traffic-selector": ["1", "2"],
            "traffic-selector": {
              "1": {"local": {"prefix": "0.0.0.0/0"}, "remote": {"prefix": "0.0.0.0/0"}},
              "2": {"local": {"prefix": "::/0"}, "remote": {"prefix": "::/0"}}
            },
            "authentication": {"mode": "pre-shared-secret", "pre-shared-secret": "cpe-secret"}
          }
        }
      }
    }
  }
}`

// dualFamilyVTIIfID is the if_id the stubbed vti0 answers, the value
// `interface xfrm vti0 { if-id 7; }` configures.
const dualFamilyVTIIfID = 7

// dualFamilyPeer parses dualFamilyCPEConfig through the plugin's own config path and
// answers the one peer it holds, with the parsed configuration for its groups.
func dualFamilyPeer(t *testing.T) (ipsec.SiteToSitePeer, *ipsec.IPsecConfig) {
	t.Helper()
	cfg, err := parseVPNSections(vpnSection(dualFamilyCPEConfig))
	if err != nil {
		t.Fatalf("parseVPNSections: %v", err)
	}
	peer, ok := cfg.Peers["cpe"]
	if !ok {
		t.Fatalf("the parsed configuration holds no peer cpe: %v", cfg.Peers)
	}
	if len(peer.TrafficSelectors) != 2 {
		t.Fatalf("peer cpe parsed %d traffic-selector entries, want 2", len(peer.TrafficSelectors))
	}
	return peer, cfg
}

// stubVTIIfID makes vti0 an XFRM interface with dualFamilyVTIIfID, and every other name
// not one, for the duration of the test.
func stubVTIIfID(t *testing.T) {
	t.Helper()
	previous := xfrmIfIDOf
	xfrmIfIDOf = func(name string) (uint32, error) {
		if name != "vti0" {
			return 0, errors.New("test: " + name + " is not an XFRM interface")
		}
		return dualFamilyVTIIfID, nil
	}
	t.Cleanup(func() { xfrmIfIDOf = previous })
}

// requireWireSelectors refuses unless payload carries exactly the prefixes want, in that
// order. It reads the wire selectors, not the pairs the code recorded.
func requireWireSelectors(t *testing.T, label string, payload *wire.PayloadTS, want ...string) {
	t.Helper()
	if payload == nil {
		t.Fatalf("%s: no payload was built", label)
	}
	if len(payload.TrafficSelectors) != len(want) {
		t.Fatalf("%s carries %d selectors, want %d (%v)", label, len(payload.TrafficSelectors), len(want), want)
	}
	for i, sel := range payload.TrafficSelectors {
		got := selectorsText(wireToSelectors([]wire.TrafficSelector{sel}))
		if got != want[i] {
			t.Errorf("%s selector %d = %s, want %s", label, i, got, want[i])
		}
	}
}

// TestDualFamilyProposalNarrowedToIPv6InstallsVTIPolicy drives the home CPE's management
// tunnel from the parsed configuration to the dataplane calls, on a fake dataplane.
//
// VALIDATES: the configuration verifies; the IKE_AUTH TSi and TSr each carry 0.0.0.0/0
// then ::/0, in the configured order; strongSwan's answer narrowed to ::/0 === ::/0 is
// adopted as one IPv6 pair; the Child SA installs its two states and one IPv6 policy per
// direction, every one carrying vti0's if_id, with IPv4 tunnel endpoints and no IPv4
// policy.
// PREVENTS: an IPv4 policy installed from the proposal rather than the answer, which would
// capture the CPE's IPv4 traffic into an SA that does not carry it, and an IPv6-only answer
// refused because it dropped a family the proposal named.
func TestDualFamilyProposalNarrowedToIPv6InstallsVTIPolicy(t *testing.T) {
	stubVTIIfID(t)
	if err := validateIPsecSections(vpnSection(dualFamilyCPEConfig)); err != nil {
		t.Fatalf("the dual-family VTI peer failed config verify: %v", err)
	}
	peer, cfg := dualFamilyPeer(t)

	sa := testSA()
	sa.IsInitiator = true
	sa.PeerCfg = peer
	sa.ESPGroup = cfg.ESPGroups[peer.ESPGroup]

	// AC-1: the proposal on the wire, IPv4 first because the list names it first.
	tsi, tsr := proposeChildTSPayloads(sa)
	requireWireSelectors(t, "proposed TSi", tsi, "{0.0.0.0/0}", "{::/0}")
	requireWireSelectors(t, "proposed TSr", tsr, "{0.0.0.0/0}", "{::/0}")
	if len(sa.ProposedChildPairs) != 2 {
		t.Fatalf("the recorded proposal holds %d pairs, want 2", len(sa.ProposedChildPairs))
	}

	// AC-2: strongSwan answers the IPv6 pair alone.
	answerI := tsPayload(t, wire.PayloadTypeTSi, "::/0")
	answerR := tsPayload(t, wire.PayloadTypeTSr, "::/0")
	if err := recordInitiatorSelectors(sa, answerI, answerR, nil); err != nil {
		t.Fatalf("the IPv6-only narrowing of a two-family proposal was refused: %v", err)
	}
	if len(sa.NegotiatedPairs) != 1 {
		t.Fatalf("negotiated %d pairs %s, want the one IPv6 pair", len(sa.NegotiatedPairs), pairsText(sa.NegotiatedPairs))
	}

	ifID, err := resolveIfID(&sa.PeerCfg)
	if err != nil {
		t.Fatalf("resolveIfID: %v", err)
	}
	if ifID != dualFamilyVTIIfID {
		t.Fatalf("resolveIfID = %d, want vti0's %d", ifID, dualFamilyVTIIfID)
	}

	// AC-3: the install.
	dp := &mockDP{}
	child, err := initiatorFirstChildSA(sa, sa.PeerCfg, ifID, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("initiatorFirstChildSA: %v", err)
	}
	t.Cleanup(child.Clear)

	requireOuterIPv4States(t, dp.sas)
	requireIPv6PoliciesOnly(t, dp.policies, []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut})
}

// requireOuterIPv4States refuses unless the install holds the two states of one Child SA,
// both bound to vti0's if_id and both addressed by the IPv4 tunnel endpoints.
func requireOuterIPv4States(t *testing.T, sas []dataplane.SAParams) {
	t.Helper()
	if len(sas) != 2 {
		t.Fatalf("installed %d SAs, want 2 (inbound + outbound)", len(sas))
	}
	for i := range sas {
		s := &sas[i]
		if s.IfID != dualFamilyVTIIfID {
			t.Errorf("SA[%d] if_id = %d, want %d", i, s.IfID, dualFamilyVTIIfID)
		}
		if s.Src.To4() == nil || s.Dst.To4() == nil {
			t.Errorf("SA[%d] outer %s -> %s, want the IPv4 tunnel endpoints", i, s.Src, s.Dst)
		}
		if s.Mode != modeTunnel {
			t.Errorf("SA[%d] mode = %d, want tunnel (%d)", i, s.Mode, modeTunnel)
		}
	}
}

// requireIPv6PoliciesOnly refuses unless policies hold exactly one ::/0 === ::/0 policy per
// direction in dirs, each on vti0's if_id with IPv4 template endpoints, and no other
// policy at all.
func requireIPv6PoliciesOnly(t *testing.T, policies []dataplane.SPParams, dirs []dataplane.SADir) {
	t.Helper()
	if len(policies) != len(dirs) {
		t.Fatalf("installed %d policies, want %d (one IPv6 policy per direction)", len(policies), len(dirs))
	}
	seen := make(map[dataplane.SADir]int, len(dirs))
	for i := range policies {
		p := &policies[i]
		seen[p.Dir]++
		if !isIPv6Wildcard(p.Src) || !isIPv6Wildcard(p.Dst) {
			t.Errorf("policy[%d] dir %d selects %v -> %v, want ::/0 -> ::/0; an IPv4 pair was never negotiated",
				i, p.Dir, p.Src, p.Dst)
		}
		if p.IfID != dualFamilyVTIIfID {
			t.Errorf("policy[%d] dir %d if_id = %d, want %d", i, p.Dir, p.IfID, dualFamilyVTIIfID)
		}
		if p.TunnelSrc.To4() == nil || p.TunnelDst.To4() == nil {
			t.Errorf("policy[%d] dir %d template %s -> %s, want the IPv4 tunnel endpoints",
				i, p.Dir, p.TunnelSrc, p.TunnelDst)
		}
	}
	for _, dir := range dirs {
		if seen[dir] != 1 {
			t.Errorf("direction %d has %d policies, want 1", dir, seen[dir])
		}
	}
}

// isIPv6Wildcard reports whether n is ::/0.
func isIPv6Wildcard(n *net.IPNet) bool {
	if n == nil {
		return false
	}
	if n.IP.To4() != nil {
		return false
	}
	ones, bits := n.Mask.Size()
	return ones == 0 && bits == 128
}
