// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: spd_policy.go -- spdPolicyParams, the producer under test
// RFC: rfc/short/rfc4301.md -- SPD dispositions and ordering (Sections 4.4.1, 7.4)
package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// spdEntry builds one operator SPD entry with the fields these tests vary. The
// prefixes are constant, because what is under test is the disposition, the direction
// swap and the order rather than the address parsing.
func spdEntry(t *testing.T, name string, action dataplane.SPAction, dir ipsec.SPDDirection) ipsec.SPDPolicy {
	t.Helper()
	_, local, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatalf("parse local prefix: %v", err)
	}
	_, remote, err := net.ParseCIDR("198.51.100.0/24")
	if err != nil {
		t.Fatalf("parse remote prefix: %v", err)
	}
	return ipsec.SPDPolicy{
		Name:         name,
		Action:       action,
		Order:        1000,
		Direction:    dir,
		LocalPrefix:  local,
		LocalPort:    ipsec.AnyPort(),
		RemotePrefix: remote,
		RemotePort:   ipsec.AnyPort(),
	}
}

// VALIDATES: RFC4301-7.4-1. The operator's DISCARD entry becomes a dataplane policy
// carrying SPActionDiscard, in both directions, with the operator's order as the
// policy priority and no template field set.
// PREVENTS: the disposition being lost between the configuration and the dataplane.
// This is the wiring the config surface exists for: without it the YANG leaf commits
// and nothing reaches the kernel.
// RFC requirement: RFC4301-7.4-1 positive -- a configured discard reaches the dataplane.
func TestDiscardPolicyReachesTheBackend(t *testing.T) {
	params := spdPolicyParams(spdEntry(t, "drop-guest", dataplane.SPActionDiscard, ipsec.SPDDirBoth))
	if len(params) != 2 {
		t.Fatalf("a both-direction entry produced %d policies, want 2 (SPD-O and SPD-I)", len(params))
	}
	for _, p := range params {
		if p.Action != dataplane.SPActionDiscard {
			t.Errorf("dir %d: Action = %d, want SPActionDiscard (%d)", p.Dir, p.Action, dataplane.SPActionDiscard)
		}
		if p.Priority != 1000 {
			t.Errorf("dir %d: Priority = %d, want the operator's order of 1000", p.Dir, p.Priority)
		}
		// A discard hands traffic to no transform, so every template field stays at
		// its zero value. A mode or a tunnel endpoint here would make the backend
		// build a protect policy.
		if p.Mode != 0 || p.ReqID != 0 || p.SAID != 0 || p.TunnelSrc != nil || p.TunnelDst != nil {
			t.Errorf("dir %d: a template field is set on a discard entry: mode=%d reqid=%d said=%d tunnel=%v/%v",
				p.Dir, p.Mode, p.ReqID, p.SAID, p.TunnelSrc, p.TunnelDst)
		}
	}
}

// VALIDATES: RFC4301-7.4-1. A BYPASS entry written in the same list reaches the
// dataplane as a bypass, so the producer carries the operator's disposition rather
// than one disposition for every entry.
// PREVENTS: the two template-free dispositions collapsing at the producer. They share
// every field and one code path, so a producer that hardcoded either one would leave
// the other silently doing its opposite: a bypass turned into a discard black-holes
// traffic the operator asked to pass in the clear.
// RFC requirement: RFC4301-7.4-1 negative -- a bypass entry is not turned into a discard.
func TestBypassPolicyReachesTheBackendAsBypass(t *testing.T) {
	params := spdPolicyParams(spdEntry(t, "pass-mgmt", dataplane.SPActionBypass, ipsec.SPDDirBoth))
	if len(params) != 2 {
		t.Fatalf("a both-direction entry produced %d policies, want 2", len(params))
	}
	for _, p := range params {
		if p.Action != dataplane.SPActionBypass {
			t.Errorf("dir %d: Action = %d, want SPActionBypass (%d)", p.Dir, p.Action, dataplane.SPActionBypass)
		}
	}
}

// VALIDATES: RFC4301-4.4.1-4. The inbound half of a both-direction entry swaps the
// local and remote sides, because the local side of a flow is the SOURCE of an
// outbound packet and the DESTINATION of an inbound one.
// PREVENTS: an inbound entry installed with the outbound selector. It would match no
// arriving packet, so a discard would silently stop discarding on the side the
// attacker sends from, and nothing would report the entry as ineffective.
// RFC requirement: RFC4301-4.4.1-4 positive -- SPD-O and SPD-I carry mirrored selectors.
func TestSPDPolicyMirrorsTheInboundSelector(t *testing.T) {
	params := spdPolicyParams(spdEntry(t, "drop-guest", dataplane.SPActionDiscard, ipsec.SPDDirBoth))

	var out, in *dataplane.SPParams
	for i := range params {
		switch params[i].Dir {
		case dataplane.SADirOut:
			out = &params[i]
		case dataplane.SADirIn:
			in = &params[i]
		case dataplane.SADirFwd:
			t.Fatalf("an operator SPD entry produced a forward policy; RFC 4301 Section 4.4.1 "+
				"splits the database into SPD-O and SPD-I, and a forward entry would exempt or "+
				"drop transit traffic the operator never named (dir %d)", params[i].Dir)
		}
	}
	if out == nil || in == nil {
		t.Fatalf("want one outbound and one inbound policy, got %d entries", len(params))
	}
	if out.Src.String() != "192.0.2.0/24" || out.Dst.String() != "198.51.100.0/24" {
		t.Errorf("outbound selector = %s -> %s, want the local prefix as source", out.Src, out.Dst)
	}
	if in.Src.String() != "198.51.100.0/24" || in.Dst.String() != "192.0.2.0/24" {
		t.Errorf("inbound selector = %s -> %s, want the remote prefix as source", in.Src, in.Dst)
	}
}

// VALIDATES: a one-way entry installs one policy, on the side the operator named.
// PREVENTS: a direction chosen for the operator. RFC 4301 Section 4.4.1 splits the
// database into SPD-O and SPD-I, and an entry written for one side that also landed
// on the other would drop return traffic the operator meant to carry.
func TestSPDPolicyOneWayEntriesInstallOnePolicy(t *testing.T) {
	for _, tc := range []struct {
		dir  ipsec.SPDDirection
		want dataplane.SADir
	}{
		{ipsec.SPDDirOut, dataplane.SADirOut},
		{ipsec.SPDDirIn, dataplane.SADirIn},
	} {
		params := spdPolicyParams(spdEntry(t, "one-way", dataplane.SPActionDiscard, tc.dir))
		if len(params) != 1 {
			t.Fatalf("direction %v produced %d policies, want 1", tc.dir, len(params))
		}
		if params[0].Dir != tc.want {
			t.Errorf("direction %v installed on Dir %d, want %d", tc.dir, params[0].Dir, tc.want)
		}
	}
}

// VALIDATES: an entry carrying no direction installs nothing.
// PREVENTS: SPDDirection's zero value being read as a side of the boundary. An entry
// that reached here with an unset direction skipped the parser, and choosing a side
// would put the operator's rule on a half of the database they never named
// (ai/rules/principles.md, a zero value is never an answer).
func TestSPDPolicyWithNoDirectionInstallsNothing(t *testing.T) {
	entry := spdEntry(t, "unset", dataplane.SPActionDiscard, ipsec.SPDDirection(0))
	if params := spdPolicyParams(entry); len(params) != 0 {
		t.Errorf("an entry with no direction produced %d policies, want 0", len(params))
	}
}

// VALIDATES: the reconciler treats an edited entry as a change and an untouched one as
// unchanged, comparing every field the producer reads.
// PREVENTS: the kernel keeping the previous form after a commit that changed it. The
// kernel identifies a policy by its selector alone, so an edit that spdPolicySame
// missed would leave the old policy installed with no way to tell from the outside.
func TestSPDPolicySameComparesEveryInstalledField(t *testing.T) {
	base := spdEntry(t, "e", dataplane.SPActionDiscard, ipsec.SPDDirBoth)
	if !spdPolicySame(base, spdEntry(t, "e", dataplane.SPActionDiscard, ipsec.SPDDirBoth)) {
		t.Error("two identical entries compared unequal, so every reload would reinstall every policy")
	}

	_, otherPrefix, _ := net.ParseCIDR("203.0.113.0/24") //nolint:errcheck // constant prefix
	for name, edit := range map[string]func(p *ipsec.SPDPolicy){
		"action":     func(p *ipsec.SPDPolicy) { p.Action = dataplane.SPActionBypass },
		"order":      func(p *ipsec.SPDPolicy) { p.Order = 1500 },
		"direction":  func(p *ipsec.SPDPolicy) { p.Direction = ipsec.SPDDirOut },
		"protocol":   func(p *ipsec.SPDPolicy) { p.Protocol = 6 },
		"localPort":  func(p *ipsec.SPDPolicy) { p.LocalPort = ipsec.PortSelector{Form: ipsec.PortSingle, Port: 443} },
		"remotePort": func(p *ipsec.SPDPolicy) { p.RemotePort = ipsec.PortSelector{Form: ipsec.PortSingle, Port: 443} },
		"localNet":   func(p *ipsec.SPDPolicy) { p.LocalPrefix = otherPrefix },
		"remoteNet":  func(p *ipsec.SPDPolicy) { p.RemotePrefix = otherPrefix },
	} {
		edited := spdEntry(t, "e", dataplane.SPActionDiscard, ipsec.SPDDirBoth)
		edit(&edited)
		if spdPolicySame(base, edited) {
			t.Errorf("a change to %s compared equal, so the kernel would keep the previous policy", name)
		}
	}
}
