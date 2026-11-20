// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions, transit traffic
// Related: spd_policy.go -- spdEntryPolicies and spdTransitPolicies, the producer under test
// Related: spd_order.go -- spdOrderRefusals, which judges the same projection at commit
// Related: spd_forward_integration_linux_test.go -- the packet test of the same projection
// RFC: rfc/short/rfc4301.md -- forwarded traffic (Section 6), stateful fragment checking (Section 7.4)

package engine

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	// transitLocal is the protected subnet behind the gateway, transitRemote the far side.
	transitLocal  = "10.2.0.0/24"
	transitRemote = "10.1.0.0/24"
	transitOrder  = 1000
	// transitProtoICMP is protocol 1, the ICMP of the IPv4 prefixes above.
	transitProtoICMP = 1
)

// transitPolicy is one projected policy in the fields that decide a packet.
type transitPolicy struct {
	dir              dataplane.SADir
	src, dst         string
	action           dataplane.SPAction
	priority         int
	srcPort, dstPort dataplane.PortMatch
}

// transitAnyPort is a projected policy whose ports are both any.
func transitAnyPort(dir dataplane.SADir, src, dst string, action dataplane.SPAction, priority int) transitPolicy {
	return transitPolicy{
		dir: dir, src: src, dst: dst, action: action, priority: priority,
		srcPort: dataplane.AnyPortMatch(), dstPort: dataplane.AnyPortMatch(),
	}
}

// transitPolicies reads each policy back in transitPolicy's fields.
func transitPolicies(policies []dataplane.SPParams) []transitPolicy {
	out := make([]transitPolicy, 0, len(policies))
	for i := range policies {
		p := &policies[i]
		out = append(out, transitPolicy{
			dir: p.Dir, src: p.Src.String(), dst: p.Dst.String(), action: p.Action,
			priority: p.Priority, srcPort: p.SrcPort, dstPort: p.DstPort,
		})
	}
	return out
}

// transitEntry is a gateway's UDP entry between transitLocal and transitRemote, both
// ports any, at transitOrder.
func transitEntry(t *testing.T, action dataplane.SPAction, direction ipsec.SPDDirection) ipsec.SPDPolicy {
	t.Helper()
	return ipsec.SPDPolicy{
		Name: "transit", Action: action, Order: transitOrder, Direction: direction, Protocol: protoUDP,
		LocalPrefix: mustCIDR(t, transitLocal), LocalPort: ipsec.AnyPort(),
		RemotePrefix: mustCIDR(t, transitRemote), RemotePort: ipsec.AnyPort(),
	}
}

// transitForwardDP is a dataplane whose forwarding path consults a forward lookup, as
// Linux XFRM's does.
func transitForwardDP() *forwardPolicyDP {
	return &forwardPolicyDP{spdDP: newSPDDP()}
}

// VALIDATES: AC-16, D-3. An operator entry is projected onto the lookups Linux runs for
// a forwarded packet: an in half's selectors in FWD with the entry's action at its
// order, a bypass's gates at dataplane.PriorityTransitGate (the in selectors in OUT, the
// out selectors in FWD), nothing for an out discard, whose OUT half already decides, and
// nothing for a port-scoped bypass or an ICMP type 0 code 0 selector. A projected policy
// that would carry the kernel identity of one already in the set is left out, decision
// first. A dataplane with no forward lookup is handed the halves alone.
// METHOD: spdEntryPolicies over a forward-lookup dataplane and a plain one, for each
// direction and action and the shapes that are not projected, read back field by field.
// PREVENTS: an operator entry that decides traffic this node sends and receives but no
// forwarded packet, which Linux checks in FWD and OUT and never in IN; and a projection
// that outranks a Child SA's PROTECT policy with an allow the operator never ranked.
func TestSPDTransitProjection(t *testing.T) {
	const (
		bypass  = dataplane.SPActionBypass
		discard = dataplane.SPActionDiscard
		fwd     = dataplane.SADirFwd
		out     = dataplane.SADirOut
		gate    = dataplane.PriorityTransitGate
	)
	echo := transitAnyPort(fwd, transitRemote, transitLocal, bypass, transitOrder)
	echo.srcPort = dataplane.ExactPortMatch(8)
	echoGate := echo
	echoGate.dir, echoGate.priority = out, gate
	scopedDiscard := transitAnyPort(fwd, transitRemote, transitLocal, discard, transitOrder)
	scopedDiscard.dstPort = dataplane.ExactPortMatch(5101)

	for _, tc := range []struct {
		name  string
		entry func(*testing.T) ipsec.SPDPolicy
		want  []transitPolicy
	}{
		{
			name:  "in bypass",
			entry: func(t *testing.T) ipsec.SPDPolicy { return transitEntry(t, bypass, ipsec.SPDDirIn) },
			want: []transitPolicy{
				transitAnyPort(fwd, transitRemote, transitLocal, bypass, transitOrder),
				transitAnyPort(out, transitRemote, transitLocal, bypass, gate),
			},
		},
		{
			name:  "in discard",
			entry: func(t *testing.T) ipsec.SPDPolicy { return transitEntry(t, discard, ipsec.SPDDirIn) },
			want:  []transitPolicy{transitAnyPort(fwd, transitRemote, transitLocal, discard, transitOrder)},
		},
		{
			name:  "out bypass",
			entry: func(t *testing.T) ipsec.SPDPolicy { return transitEntry(t, bypass, ipsec.SPDDirOut) },
			want:  []transitPolicy{transitAnyPort(fwd, transitLocal, transitRemote, bypass, gate)},
		},
		{
			name:  "out discard",
			entry: func(t *testing.T) ipsec.SPDPolicy { return transitEntry(t, discard, ipsec.SPDDirOut) },
		},
		{
			name:  "both bypass",
			entry: func(t *testing.T) ipsec.SPDPolicy { return transitEntry(t, bypass, ipsec.SPDDirBoth) },
			want: []transitPolicy{
				transitAnyPort(fwd, transitRemote, transitLocal, bypass, transitOrder),
				transitAnyPort(fwd, transitLocal, transitRemote, bypass, gate),
				transitAnyPort(out, transitRemote, transitLocal, bypass, gate),
			},
		},
		{
			name:  "both discard",
			entry: func(t *testing.T) ipsec.SPDPolicy { return transitEntry(t, discard, ipsec.SPDDirBoth) },
			want:  []transitPolicy{transitAnyPort(fwd, transitRemote, transitLocal, discard, transitOrder)},
		},
		{
			name: "both bypass with equal sides keeps its decision and drops both gates",
			entry: func(t *testing.T) ipsec.SPDPolicy {
				p := transitEntry(t, bypass, ipsec.SPDDirBoth)
				p.RemotePrefix = mustCIDR(t, transitLocal)
				return p
			},
			want: []transitPolicy{transitAnyPort(fwd, transitLocal, transitLocal, bypass, transitOrder)},
		},
		{
			name: "in bypass with a local port is not projected",
			entry: func(t *testing.T) ipsec.SPDPolicy {
				p := transitEntry(t, bypass, ipsec.SPDDirIn)
				p.LocalPort = ipsec.PortSelector{Form: ipsec.PortSingle, Port: 5101}
				return p
			},
		},
		{
			name: "in discard with a local port is projected with its port",
			entry: func(t *testing.T) ipsec.SPDPolicy {
				p := transitEntry(t, discard, ipsec.SPDDirIn)
				p.LocalPort = ipsec.PortSelector{Form: ipsec.PortSingle, Port: 5101}
				return p
			},
			want: []transitPolicy{scopedDiscard},
		},
		{
			name: "in ICMP type 0 is not projected",
			entry: func(t *testing.T) ipsec.SPDPolicy {
				p := transitEntry(t, bypass, ipsec.SPDDirIn)
				p.Protocol, p.ICMP = transitProtoICMP, ipsec.ICMPSelector{HasType: true, Type: 0}
				return p
			},
		},
		{
			name: "in ICMP type 8 is projected with its type",
			entry: func(t *testing.T) ipsec.SPDPolicy {
				p := transitEntry(t, bypass, ipsec.SPDDirIn)
				p.Protocol, p.ICMP = transitProtoICMP, ipsec.ICMPSelector{HasType: true, Type: 8}
				return p
			},
			want: []transitPolicy{echo, echoGate},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := tc.entry(t)
			halves := spdPolicyParams(entry)
			if len(halves) == 0 {
				t.Fatal("the entry installs no half, so an empty projection would prove nothing")
			}
			all := spdEntryPolicies(transitForwardDP(), entry)
			if len(all) < len(halves) || !slices.Equal(transitPolicies(all[:len(halves)]), transitPolicies(halves)) {
				t.Fatalf("the set handed to a forward-lookup dataplane does not start with the entry's halves: %+v", transitPolicies(all))
			}
			if got := transitPolicies(all[len(halves):]); !slices.Equal(got, tc.want) {
				t.Fatalf("transit projection\n got %+v\nwant %+v", got, tc.want)
			}
			if plain := spdEntryPolicies(newSPDDP(), entry); !slices.Equal(transitPolicies(plain), transitPolicies(halves)) {
				t.Fatalf("a dataplane with no forward lookup was handed %+v, want the halves alone", transitPolicies(plain))
			}
		})
	}
}

// VALIDATES: the reconciler installs, replaces and removes the transit projection with
// its entry: a both-direction bypass leaves its five kernel identities, an edit to
// direction in takes the out half and the local-to-remote gate away, and shutdown leaves
// nothing.
// METHOD: reconcileSPDPolicies and removeSPDPolicies over a forward-lookup dataplane that
// upserts by selector, as XFRM does, reading the policies it holds.
// PREVENTS: a forward entry that outlives the edit or the daemon, still passing or
// dropping transit traffic no configuration explains.
func TestSPDTransitPoliciesLeaveWithTheirEntry(t *testing.T) {
	dp := transitForwardDP()
	log := slogutil.DiscardLogger()
	both := map[string]ipsec.SPDPolicy{"transit": transitEntry(t, dataplane.SPActionBypass, ipsec.SPDDirBoth)}
	if _, err := reconcileSPDPolicies(dp, nil, both, log); err != nil {
		t.Fatalf("install the both-direction bypass: %v", err)
	}
	if got := len(dp.policies); got != 5 {
		t.Fatalf("the dataplane holds %d policies, want 5: out and in halves, the FWD decision and two gates", got)
	}

	in := map[string]ipsec.SPDPolicy{"transit": transitEntry(t, dataplane.SPActionBypass, ipsec.SPDDirIn)}
	if _, err := reconcileSPDPolicies(dp, both, in, log); err != nil {
		t.Fatalf("edit the entry to direction in: %v", err)
	}
	for _, gone := range []spdKey{
		{src: transitLocal, dst: transitRemote, dir: dataplane.SADirOut},
		{src: transitLocal, dst: transitRemote, dir: dataplane.SADirFwd},
	} {
		if _, held := dp.policies[gone]; held {
			t.Errorf("after the edit to direction in, the dataplane still holds %+v", gone)
		}
	}
	if got := len(dp.policies); got != 3 {
		t.Fatalf("after the edit the dataplane holds %d policies, want 3: the in half, the FWD decision and the OUT gate", got)
	}

	removeSPDPolicies(dp, in, log)
	if got := len(dp.policies); got != 0 {
		t.Fatalf("shutdown left %d policies installed", got)
	}
}

// VALIDATES: the commit judges the forward entries of both sides: an operator entry
// whose transit projection ties with a tunnel-mode peer's forwarding entries one rank
// below its policy-priority is refused with that rank and the fix, the next rank is
// accepted, and a transport-mode peer, which installs no forward entry, is not compared
// there.
// METHOD: verifySPDOrder over a both-direction entry and a peer whose traffic selector
// names the same flow from the other side, at the peer's policy-priority + 1, + 2, and
// with the peer in transport mode.
// PREVENTS: a forward entry and a Child SA's forward entry at one rank, decided by the
// sequence the kernel received them in.
func TestSPDOrderJudgesTheForwardEntries(t *testing.T) {
	peer := orderTotalPeer(t, "192.0.2.0/24", "10.0.0.0/8", "")
	entry := orderTotalEntry(t, "reversed", "10.1.0.0/16", "192.0.2.0/24", 2001)
	cfg := &ipsec.IPsecConfig{
		Policies: map[string]ipsec.SPDPolicy{"reversed": entry},
		Peers:    map[string]ipsec.SiteToSitePeer{"branch": peer},
	}
	refusal, ok := errors.AsType[*opererr.Error](verifySPDOrder(cfg))
	if !ok {
		t.Fatal("an entry tied with the peer's forwarding entries was accepted")
	}
	for _, want := range []string{"vpn ipsec policy reversed", "site-to-site peer branch (its forwarding entries at policy-priority + 1)", "share order 2001"} {
		if !strings.Contains(refusal.Problem(), want) {
			t.Fatalf("the refusal %q does not carry %q", refusal.Problem(), want)
		}
	}
	if refusal.Fix() != "set vpn ipsec policy reversed order 2002" {
		t.Fatalf("the refusal's fix is %q", refusal.Fix())
	}

	entry.Order = 2002
	cfg.Policies["reversed"] = entry
	if err := verifySPDOrder(cfg); err != nil {
		t.Fatalf("an entry ranked apart from every peer entry was refused: %v", err)
	}

	entry.Order = 2001
	cfg.Policies["reversed"] = entry
	peer.Mode = dataplane.ModeTransport
	cfg.Peers["branch"] = peer
	if err := verifySPDOrder(cfg); err != nil {
		t.Fatalf("a transport-mode peer, which installs no forward entry, refused the entry: %v", err)
	}
}

// VALIDATES: RFC4301-7.4-2, the case just outside the clause. RFC 4301 Section 7.4: "All
// implementations MUST support stateful fragment checking to accommodate BYPASS traffic
// for which a non-trivial port range is specified." A bypass whose ports are both any
// names no port range, and the forward path is where Ze hands it: its remote-to-local
// decision at its order and its local-to-remote gate. The same entry with a local port
// is handed none on a dataplane that does not gather fragments (transitForwardDP
// declares no dataplane.ForwardFragmentGatherer), so the withholding
// TestPortScopedBypassNeverReachesTheForwardPath proves is tied to the port and is not a
// refusal of every forward bypass.
// METHOD: spdEntryPolicies over a forward-lookup dataplane for a both-direction UDP
// bypass with both ports any, then with local port 161, reading every FWD policy.
// PREVENTS: a port test that withholds every bypass from FWD, which would pass the
// positive test while leaving every forwarded flow an operator passes to the catch-all.
//
// RFC requirement: RFC4301-7.4-2 negative -- an operator BYPASS with both ports any
// reaches the forward path in both orientations, and the same entry with a local port
// does not on a dataplane that does not gather fragments.
func TestAnyPortBypassReachesTheForwardPath(t *testing.T) {
	dp := transitForwardDP()
	entry := transitEntry(t, dataplane.SPActionBypass, ipsec.SPDDirBoth)
	var remoteToLocal, localToRemote bool
	for _, p := range spdEntryPolicies(dp, entry) {
		if p.Dir != dataplane.SADirFwd || p.Action != dataplane.SPActionBypass {
			continue
		}
		if p.Src.String() == transitRemote && p.Dst.String() == transitLocal {
			remoteToLocal = true
		}
		if p.Src.String() == transitLocal && p.Dst.String() == transitRemote {
			localToRemote = true
		}
	}
	if !remoteToLocal {
		t.Error("an any-port bypass reached no forward policy from the remote side to the protected side")
	}
	if !localToRemote {
		t.Error("an any-port bypass reached no forward policy from the protected side to the remote side")
	}

	scoped := entry
	scoped.LocalPort = ipsec.PortSelector{Form: ipsec.PortSingle, Port: 161}
	for _, p := range spdEntryPolicies(dp, scoped) {
		if p.Dir == dataplane.SADirFwd {
			t.Errorf("the same bypass with local port 161 reached the forward path: src=%v dst=%v sport=%v dport=%v",
				p.Src, p.Dst, p.SrcPort, p.DstPort)
		}
	}
}

// VALIDATES: RFC 4301 Section 5.2, step 4, past an operator bypass in FWD (owner
// rulings 2026-11-20, K1 bit 7): every template-free FWD allow an operator bypass
// projects asks the dataplane for the forwarded ICMP error payload check
// (SPParams.ICMPErrorPayload): the FWD transit gate of an out or both bypass, and the
// FWD decision of an in or both bypass at the entry's order. No other policy of the
// entry does: not its out or in halves, not its OUT gate, and nothing a discard
// projects, its FWD decision included.
// METHOD: spdEntryPolicies over a forward-lookup dataplane for each direction and
// action; each policy's flag is compared with "a FWD policy of a bypass", and the
// flagged FWD gates and FWD decisions are counted apart, so each case must yield the
// exact flagged set its direction projects and the comparison is not vacuous.
// PREVENTS: a FWD gate or FWD bypass decision without the flag, which on Ze's kernel
// still refuses a consistent forwarded ICMP error that arrived on a Child SA
// (XfrmInTmplMismatch); the flag on an OUT gate or an OUT half, where
// icmp_route_lookup would source an error this node sends from the address it quotes;
// and the flag on a discard, where the kernel acts on it for an ALLOW only.
func TestSPDTransitFWDBypassesCarryTheICMPErrorPayloadCheck(t *testing.T) {
	for _, tc := range []struct {
		name      string
		action    dataplane.SPAction
		direction ipsec.SPDDirection
		gates     int
		decisions int
	}{
		{"in bypass", dataplane.SPActionBypass, ipsec.SPDDirIn, 0, 1},
		{"out bypass", dataplane.SPActionBypass, ipsec.SPDDirOut, 1, 0},
		{"both bypass", dataplane.SPActionBypass, ipsec.SPDDirBoth, 1, 1},
		{"in discard", dataplane.SPActionDiscard, ipsec.SPDDirIn, 0, 0},
		{"out discard", dataplane.SPActionDiscard, ipsec.SPDDirOut, 0, 0},
		{"both discard", dataplane.SPActionDiscard, ipsec.SPDDirBoth, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policies := spdEntryPolicies(transitForwardDP(), transitEntry(t, tc.action, tc.direction))
			gates, decisions := 0, 0
			for i := range policies {
				p := &policies[i]
				want := p.Dir == dataplane.SADirFwd && p.Action == dataplane.SPActionBypass
				if p.ICMPErrorPayload != want {
					t.Errorf("dir=%d priority=%d action=%d src=%v dst=%v: ICMPErrorPayload=%v, want %v",
						p.Dir, p.Priority, p.Action, p.Src, p.Dst, p.ICMPErrorPayload, want)
				}
				if !p.ICMPErrorPayload {
					continue
				}
				if p.Priority == dataplane.PriorityTransitGate {
					gates++
					continue
				}
				decisions++
			}
			if gates != tc.gates {
				t.Errorf("%d FWD gates carry the payload check, want %d: %+v", gates, tc.gates, policies)
			}
			if decisions != tc.decisions {
				t.Errorf("%d FWD decisions carry the payload check, want %d: %+v", decisions, tc.decisions, policies)
			}
		})
	}
}
