// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions, transit traffic
// Related: spd_order.go -- spdSharedKernelPolicy and spdPeerSelectorRefusal, the commit refusal under test
// Related: spd_policy.go -- spdInstallRefusal, the refusal of an entry the ownership ledger refused
// Related: spd_peer_selector_integration_linux_test.go -- the same collision against the kernel

package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// peerSelectorConfig is a policy-based tunnel peer "cpe" protecting 10.2.0.0/24 (local)
// to 10.1.0.0/24 (remote) at the default policy-priority 2000, beside one operator
// bypass entry "hole", direction in, order 500, whose local and remote sides are the
// peer's remote and local ones. PROTOCOL is replaced by the entry's protocol leaf, or by
// nothing. With no protocol the entry's transit projection carries the kernel identity
// of two of the peer's policies: its FWD decision 10.2.0.0/24 to 10.1.0.0/24 is the
// peer's outbound forward allow, and its OUT gate with the same selectors is the peer's
// OUT PROTECT policy.
const peerSelectorConfig = `{
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
            "@traffic-selector": ["1"],
            "traffic-selector": {
              "1": {"local": {"prefix": "10.2.0.0/24"}, "remote": {"prefix": "10.1.0.0/24"}}
            },
            "authentication": {"mode": "pre-shared-secret", "pre-shared-secret": "cpe-secret"}
          }
        }
      },
      "@policy": ["hole"],
      "policy": {
        "hole": {
          "action": "bypass",
          "order": "500",
          "direction": "in",PROTOCOL
          "local": {"prefix": "10.1.0.0/24"},
          "remote": {"prefix": "10.2.0.0/24"}
        }
      }
    }
  }
}`

// VALIDATES: validateIPsecSections, the offline verifier `ze config validate`, every
// commit door and OnConfigVerify run, refuses an operator entry whose installed policies
// share a kernel identity (selector, direction, if_id) with a configured peer's, at an
// order that differs from the peer's policy-priority, with an operator error naming
// both, the selector, and `delete vpn ipsec policy <name>`. The same entry narrowed to
// UDP, which is another kernel identity inside the peer's selector, is accepted.
// METHOD: parse peerSelectorConfig through the plugin's own config path, check the
// fixture holds the peer selector and the entry the test means, then run the verifier.
// PREVENTS: the entry's OUT gate replacing the peer's OUT PROTECT policy in the kernel,
// which replaces by selector whatever the priority, so the peer's traffic left in clear.
// MUTATION: removing the spdSharedKernelPolicy step of spdOrderRefusals accepts the
// first configuration (order 500 against 2000 is no rank tie), and this test fails there.
func TestSPDPolicyOnAPeerSelectorRefusedOffline(t *testing.T) {
	collides := strings.Replace(peerSelectorConfig, "PROTOCOL", "", 1)
	narrowed := strings.Replace(peerSelectorConfig, "PROTOCOL", `"protocol": "17",`, 1)

	cfg, err := parseVPNSections(vpnSection(collides))
	if err != nil {
		t.Fatalf("parse the fixture: %v", err)
	}
	peer, held := cfg.Peers["cpe"]
	if !held || len(peer.TrafficSelectors) != 1 || peer.VTIBind != "" {
		t.Fatalf("the fixture does not hold the policy-based peer cpe with one traffic selector: %+v", cfg.Peers)
	}
	entry, held := cfg.Policies["hole"]
	if !held || entry.Order == peer.PolicyPriority || entry.Order == peer.PolicyPriority+1 {
		t.Fatalf("the fixture does not hold the entry hole at an order apart from the peer's ranks: %+v", cfg.Policies)
	}

	err = validateIPsecSections(vpnSection(collides))
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("an operator entry on a peer's selector answered %v, want an operator error", err)
	}
	for _, want := range []string{"vpn ipsec policy hole", "site-to-site peer cpe", "same SPD selector", "10.2.0.0/24 -> 10.1.0.0/24"} {
		if !strings.Contains(refusal.Problem(), want) {
			t.Errorf("the refusal %q does not carry %q", refusal.Problem(), want)
		}
	}
	if refusal.Fix() != "delete vpn ipsec policy hole" {
		t.Errorf("the refusal's fix is %q, want `delete vpn ipsec policy hole`", refusal.Fix())
	}

	if err := validateIPsecSections(vpnSection(narrowed)); err != nil {
		t.Fatalf("the entry narrowed to UDP, another kernel identity than the peer's policies, was refused: %v", err)
	}
}

// VALIDATES: the commit refusal is keyed on the kernel identity alone. It holds at any
// order, above and below the peer's policy-priority; it holds where only the entry's
// OUT gate meets the peer (a transport-mode peer installs no forward entry); and it
// names the direction of the shared policy; a discard on the peer's own inbound selector
// is refused too, since it would replace the inbound PROTECT policy. A peer bound to an
// XFRM interface (an SPD of its own, another if_id) is accepted, and so is a mirrored
// discard beside a transport-mode peer: it projects no gate, and neither its in half nor
// its decision lands on a selector that peer installs.
// METHOD: verifySPDOrder, the step validateIPsecSections runs, over each configuration.
// PREVENTS: a refusal keyed on equal rank (spdRankTie), which misses every row here: a
// gate sits at dataplane.PriorityTransitGate, never at a peer's rank.
func TestSPDPolicyOnAPeerSelectorRefusedWhateverTheOrder(t *testing.T) {
	// The entry's local and remote sides are the peer's remote and local ones.
	mirrored := func(order uint32, action dataplane.SPAction) ipsec.SPDPolicy {
		p := orderTotalEntry(t, "hole", "10.1.0.0/24", "10.2.0.0/24", order)
		p.Protocol, p.Direction, p.Action = 0, ipsec.SPDDirIn, action
		return p
	}
	tunnel := orderTotalPeer(t, "10.2.0.0/24", "10.1.0.0/24", "")
	transport := tunnel
	transport.Mode = dataplane.ModeTransport
	routed := orderTotalPeer(t, "10.2.0.0/24", "10.1.0.0/24", "vti0")

	for _, tc := range []struct {
		name  string
		entry ipsec.SPDPolicy
		peer  ipsec.SiteToSitePeer
		// dir is the direction the refusal names, as dirLabel writes it; empty means accepted.
		dir string
	}{
		{"a bypass ranked above a tunnel peer", mirrored(500, dataplane.SPActionBypass), tunnel, "dir fwd"},
		{"a bypass ranked below a tunnel peer", mirrored(5000, dataplane.SPActionBypass), tunnel, "dir fwd"},
		{"a bypass whose OUT gate alone meets a transport peer", mirrored(500, dataplane.SPActionBypass), transport, "dir out"},
		{"a discard beside a transport peer, which projects no gate", mirrored(500, dataplane.SPActionDiscard), transport, ""},
		{"a bypass beside a peer bound to an XFRM interface", mirrored(500, dataplane.SPActionBypass), routed, ""},
		{"a discard on the peer's inbound selector", func() ipsec.SPDPolicy {
			p := orderTotalEntry(t, "hole", "10.2.0.0/24", "10.1.0.0/24", 500)
			p.Protocol, p.Direction, p.Action = 0, ipsec.SPDDirIn, dataplane.SPActionDiscard
			return p
		}(), transport, "dir in"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ipsec.IPsecConfig{
				Policies: map[string]ipsec.SPDPolicy{"hole": tc.entry},
				Peers:    map[string]ipsec.SiteToSitePeer{"branch": tc.peer},
			}
			err := verifySPDOrder(cfg)
			if tc.dir == "" {
				if err != nil {
					t.Fatalf("an entry that shares no kernel policy with the peer was refused: %v", err)
				}
				return
			}
			refusal, ok := errors.AsType[*opererr.Error](err)
			if !ok {
				t.Fatalf("an entry on the peer's selector answered %v, want an operator error", err)
			}
			for _, want := range []string{"vpn ipsec policy hole", "site-to-site peer branch", "same SPD selector", "(" + tc.dir + ")"} {
				if !strings.Contains(refusal.Problem(), want) {
					t.Errorf("the refusal %q does not carry %q", refusal.Problem(), want)
				}
			}
			if refusal.Fix() != "delete vpn ipsec policy hole" {
				t.Errorf("the refusal's fix is %q, want `delete vpn ipsec policy hole`", refusal.Fix())
			}
		})
	}
}

// VALIDATES: an operator entry the dataplane refuses because a peer's Child SA policy
// holds its selector (dataplane.PolicyOwnedError, the ownership ledger's answer for a
// selector a peer negotiated) refuses the reload with an operator error that wraps the
// ledger's answer and names `delete vpn ipsec policy <name>`, never the retry: a retry
// cannot succeed while the Child SA lives. A start records the same refusal for the
// ipsec-spd-install doctor check. Any other refusal keeps its retry.
// METHOD: reconcileSPDPolicies and installSPDPoliciesAtStart, the reload and start
// installers, over a dataplane that answers the ledger's error for the entry's inbound
// policy; then the same with a plain netlink error.
// PREVENTS: the operator being told to commit again, or to restart, for a refusal that
// only removing the entry clears.
func TestSPDInstallOnAnOwnedSelectorNamesTheEntryRemoval(t *testing.T) {
	resetSPDLeft(t)
	entry := spdDiscardEntry(t)
	next := map[string]ipsec.SPDPolicy{"drop": entry}
	log := slogutil.DiscardLogger()
	owned := &dataplane.PolicyOwnedError{HeldBy: "branch", WantedRole: dataplane.PolicyRoleOperator}
	refuseWith := func(cause error) *refusingSPDDP {
		return &refusingSPDDP{refuse: func(p dataplane.SPParams) error {
			if p.Role == dataplane.PolicyRoleOperator && p.Dir == dataplane.SADirIn {
				return cause
			}
			return nil
		}}
	}

	_, err := reconcileSPDPolicies(refuseWith(owned), nil, next, log)
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("a reload whose entry the ledger refused answered %v, want an operator error", err)
	}
	if held, wraps := errors.AsType[*dataplane.PolicyOwnedError](err); !wraps || held != owned {
		t.Fatalf("the refusal does not wrap the ledger's answer: %v", err)
	}
	if refusal.Fix() != "delete vpn ipsec policy drop" {
		t.Errorf("the reload refusal's fix is %q, want `delete vpn ipsec policy drop`", refusal.Fix())
	}
	if !strings.Contains(refusal.Problem(), "drop") || !strings.Contains(refusal.Why(), "Child SA policy") {
		t.Errorf("the refusal does not name the entry and the Child SA policy: %q, %q", refusal.Problem(), refusal.Why())
	}

	_, refused, _ := installSPDPoliciesAtStart(refuseWith(owned), next, log)
	if got := refused["drop"]; got == nil || got.Fix() != "delete vpn ipsec policy drop" {
		t.Errorf("the start recorded %v for the entry, want a refusal whose fix is `delete vpn ipsec policy drop`", got)
	}

	_, err = reconcileSPDPolicies(refuseWith(errSPDRefusedByTest), nil, next, log)
	refusal, ok = errors.AsType[*opererr.Error](err)
	if !ok || refusal.Fix() != spdInstallFixCommit {
		t.Errorf("a reload refused for another cause answered %v, want an operator error whose fix is %q", err, spdInstallFixCommit)
	}
}
