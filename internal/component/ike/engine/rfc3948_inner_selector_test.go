// Design: docs/architecture/ike/ipsec-14-responder.md -- the cross-peer inner-selector guard
// Related: inner_selectors.go -- the registry under test
// Related: ts_narrow.go -- narrowChildSelectors, whose filter step is under test
// VALIDATES: no two peers hold tunnel Child SAs one packet could be sent into, unless the
// operator configured the remote half of both at different policy priorities, on every
// responder path and at install.
// PREVENTS: an unbound peer taking another peer's inner address, the gap RFC 3948
// Section 5.1 names (rfc/audit/rfc3948/RFC3948-5.1-1.json).

package engine

import (
	"errors"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// innerPeerSA builds an IKE SA this node responds on, for the peer name reached at
// remote, sharing reg, with the operator's configured selectors (none = unconfigured).
func innerPeerSA(t *testing.T, reg *innerSelectorRegistry, name, remote string, selectors ...ipsec.TrafficSelectorPolicy) *SA {
	t.Helper()
	sa := testSAWithGCMKeys(t)
	sa.IsInitiator = false
	sa.ESPGroup = testESPGroup()
	sa.PeerName = name
	sa.PeerCfg = ipsec.SiteToSitePeer{
		Name: name, LocalAddress: "192.0.2.1", RemoteAddress: remote, TrafficSelectors: selectors,
	}
	sa.innerSelectors = reg
	return sa
}

// innerAuthChild runs the IKE_AUTH responder's two steps in the order selectAuthChildSA
// and buildAuthResponse run them: narrowChildSelectors, then createFirstChildSA.
func innerAuthChild(t *testing.T, sa *SA, dp dataplane.Dataplane, tsi, tsr []string) (*ChildSA, error) {
	t.Helper()
	if err := narrowChildSelectors(sa, tsPayload(t, wire.PayloadTypeTSi, tsi...),
		tsPayload(t, wire.PayloadTypeTSr, tsr...), nil); err != nil {
		return nil, err
	}
	ifID, err := resolveIfID(&sa.PeerCfg)
	if err != nil {
		t.Fatalf("resolveIfID: %v", err)
	}
	return createFirstChildSA(sa, sa.ESPGroup, sa.PeerCfg.LocalAddress, sa.PeerCfg.RemoteAddress, ifID, dp,
		slogutil.DiscardLogger())
}

// mustInnerChild is innerAuthChild that must succeed.
func mustInnerChild(t *testing.T, sa *SA, dp dataplane.Dataplane, tsi, tsr []string) *ChildSA {
	t.Helper()
	child, err := innerAuthChild(t, sa, dp, tsi, tsr)
	if err != nil {
		t.Fatalf("peer %s: %v", sa.PeerName, err)
	}
	return child
}

// mustInnerRefused asserts err is the cross-peer refusal answered as TS_UNACCEPTABLE.
func mustInnerRefused(t *testing.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: served, want TS_UNACCEPTABLE", what)
	}
	if !errors.Is(err, errTSUnacceptable) {
		t.Fatalf("%s: %v, want an error wrapping errTSUnacceptable", what, err)
	}
	if got := notifyForRefusal(err); got != wire.NotifyTSUnacceptable {
		t.Fatalf("%s: notify %d, want TS_UNACCEPTABLE (%d)", what, got, wire.NotifyTSUnacceptable)
	}
	if !strings.Contains(err.Error(), "RFC 3948 Section 5.1") {
		t.Fatalf("%s: refused for another reason: %v", what, err)
	}
}

// selectorPolicy is one configured selector: remote prefix, local prefix, protocol and
// remote port (0 = any).
//
//nolint:unparam // local names the configured local prefix at every call, so each case reads as the selector the operator wrote; the cases vary only the remote half.
func selectorPolicy(t *testing.T, remote, local string, proto uint8, remotePort uint16) ipsec.TrafficSelectorPolicy {
	t.Helper()
	anyPort := ipsec.PortSelector{Form: ipsec.PortAny}
	rp := anyPort
	if remotePort != 0 {
		rp = ipsec.PortSelector{Form: ipsec.PortSingle, Port: remotePort}
	}
	return ipsec.TrafficSelectorPolicy{
		Number: "1", LocalPrefix: mustNet(t, local), LocalPort: anyPort,
		RemotePrefix: mustNet(t, remote), RemotePort: rp, Protocol: proto,
	}
}

func policiesNaming(dp *mockDP, cidr string) int {
	n := 0
	for _, p := range dp.policies {
		if p.Src.String() == cidr || p.Dst.String() == cidr {
			n++
		}
	}
	return n
}

// TestRFC3948TwoUnboundPeersSameInnerAddressSecondRefused replays the audit probe: two
// unbound, unconfigured responder peers both proposing TSi 10.1.2.3/32, the first with
// TSr 0.0.0.0/0 and the second with TSr 192.0.2.10/32. Method: peer a negotiates and
// installs through narrowChildSelectors and createFirstChildSA; peer b then runs the same
// steps and must be refused with TS_UNACCEPTABLE before its dataplane sees a call, while
// a keeps its policies and its record.
func TestRFC3948TwoUnboundPeersSameInnerAddressSecondRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- a second unbound tunnel peer proposing an inner address 10.1.2.3/32 another peer's live Child SA holds (TSr 0.0.0.0/0 against 192.0.2.10/32) is answered TS_UNACCEPTABLE, its dataplane receives no state or policy, and the first peer's two policies and record stay.
	reg := newInnerSelectorRegistry()
	dpA, dpB := &mockDP{}, &mockDP{}
	a := innerPeerSA(t, reg, "a", "198.18.0.7")
	mustInnerChild(t, a, dpA, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})

	b := innerPeerSA(t, reg, "b", "198.18.0.9")
	_, err := innerAuthChild(t, b, dpB, []string{"10.1.2.3/32"}, []string{"192.0.2.10/32"})
	mustInnerRefused(t, err, "peer b")
	if len(dpB.sas) != 0 || len(dpB.policies) != 0 {
		t.Fatalf("peer b's dataplane got %d states and %d policies, want none", len(dpB.sas), len(dpB.policies))
	}
	if got := policiesNaming(dpA, "10.1.2.3/32"); got != 2 {
		t.Fatalf("peer a holds %d policies naming 10.1.2.3/32, want 2 (in and out)", got)
	}
	if got := reg.holdings(); got != 1 {
		t.Fatalf("registry holds %d Child SAs, want 1 (peer a)", got)
	}
}

// TestRFC3948TwoUnboundPeersDistinctInnerAddressesServed is the positive polarity: two
// unbound peers proposing distinct inner hosts are both served and both recorded.
func TestRFC3948TwoUnboundPeersDistinctInnerAddressesServed(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 positive -- two unbound tunnel peers proposing distinct inner addresses 10.1.2.3/32 and 10.1.2.4/32 are both served, each dataplane receives its policies, and both Child SAs are recorded.
	reg := newInnerSelectorRegistry()
	dpA, dpB := &mockDP{}, &mockDP{}
	mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dpA, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	mustInnerChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), dpB, []string{"10.1.2.4/32"}, []string{"192.0.2.10/32"})
	if got := policiesNaming(dpB, "10.1.2.4/32"); got != 2 {
		t.Fatalf("peer b holds %d policies naming 10.1.2.4/32, want 2", got)
	}
	if got := reg.holdings(); got != 2 {
		t.Fatalf("registry holds %d Child SAs, want 2", got)
	}
}

// TestInnerSelectorDisjointLocalSidesNoConflict: one inner host, two disjoint local
// sides. No packet matches both pairs, so both peers are served (AC-3).
func TestInnerSelectorDisjointLocalSidesNoConflict(t *testing.T) {
	reg := newInnerSelectorRegistry()
	mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"192.0.2.10/32"})
	mustInnerChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"198.51.100.0/24"})
}

// TestRFC3948ConfiguredOverlapNarrowedByPeerRefused: both peers are configured with the
// remote prefix 10.0.0.0/8 at the unset default policy-priority, and each narrows to
// 10.1.2.3/32, so the second is refused (AC-4). Two clauses refuse it here: the narrowed
// remote half is the peer's choice, not an operator-fixed selector, and two peers at one
// priority are unordered. TestRFC3948NarrowedByPeerDistinctPriorityRefused runs the same
// case at distinct priorities, so only the classification refuses it there.
func TestRFC3948ConfiguredOverlapNarrowedByPeerRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- two tunnel peers each configured with remote prefix 10.0.0.0/8 at the unset default policy-priority that both narrow their proposal to 10.1.2.3/32: the second is answered TS_UNACCEPTABLE.
	reg := newInnerSelectorRegistry()
	wide := selectorPolicy(t, "10.0.0.0/8", "0.0.0.0/0", 0, 0)
	mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7", wide), &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	_, err := innerAuthChild(t, innerPeerSA(t, reg, "b", "198.18.0.9", wide), &mockDP{},
		[]string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	mustInnerRefused(t, err, "peer b")
}

// TestInnerSelectorConfiguredOverlapNegotiatedWholeAllowed: the operator configured
// overlapping remote prefixes on two peers, ordered by policy-priority, and each peer
// proposes exactly its configured selector, one of them TCP to port 179. Both are served
// at their own priority (AC-5, R-5).
func TestInnerSelectorConfiguredOverlapNegotiatedWholeAllowed(t *testing.T) {
	reg := newInnerSelectorRegistry()
	dpA, dpB := &mockDP{}, &mockDP{}
	a := innerPeerSA(t, reg, "a", "198.18.0.7", selectorPolicy(t, "10.1.0.0/16", "0.0.0.0/0", 0, 0))
	a.PeerCfg.PolicyPriority = 2000
	mustInnerChild(t, a, dpA, []string{"10.1.0.0/16"}, []string{"0.0.0.0/0"})

	b := innerPeerSA(t, reg, "b", "198.18.0.9", selectorPolicy(t, "10.1.2.0/24", "0.0.0.0/0", 6, 179))
	b.PeerCfg.PolicyPriority = 1500
	tsi := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: []wire.TrafficSelector{
		wireSel(t, "10.1.2.0/24", 179, 179, 6),
	}}
	tsr := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr, TrafficSelectors: []wire.TrafficSelector{
		wireSel(t, "0.0.0.0/0", 0, 65535, 6),
	}}
	if err := narrowChildSelectors(b, tsi, tsr, nil); err != nil {
		t.Fatalf("peer b narrowing: %v", err)
	}
	if _, err := createFirstChildSA(b, b.ESPGroup, "192.0.2.1", "198.18.0.9", 0, dpB, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("peer b install: %v", err)
	}
	for _, c := range []struct {
		dp   *mockDP
		want int
	}{{dpA, 2000}, {dpB, 1500}} {
		if len(c.dp.policies) == 0 {
			t.Fatalf("no policy installed")
		}
		for _, p := range c.dp.policies {
			if p.Priority != c.want {
				t.Fatalf("policy %s -> %s priority %d, want %d", p.Src, p.Dst, p.Priority, c.want)
			}
		}
	}
	if got := reg.holdings(); got != 2 {
		t.Fatalf("registry holds %d Child SAs, want 2 (peers a and b)", got)
	}
}

// TestRFC3948ConfiguredOverlapEqualPriorityRefused is the audit case the exemption above
// must not reach: the operator configured remote 10.1.0.0/16 on peer a and 10.1.2.0/24
// on peer b, and gave both the same policy-priority, explicitly 2000 or unset (which the
// dataplane installs at the same default, dataplane.PriorityChildSA). Each peer proposes
// exactly its configured selector, so both pairs are operator-fixed, yet nothing orders
// them: a packet to 10.1.2.3 would go into whichever tunnel installed first. Method: peer
// a negotiates and installs; peer b runs the same steps and must be refused with
// TS_UNACCEPTABLE before its dataplane sees a call, while a keeps its policies and its
// record.
func TestRFC3948ConfiguredOverlapEqualPriorityRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- two tunnel peers configured with overlapping remote prefixes 10.1.0.0/16 and 10.1.2.0/24 at one policy-priority (both 2000, or both unset), each proposing exactly its configured selector: the second is answered TS_UNACCEPTABLE, its dataplane receives no state or policy, and the first peer's policies and record stay.
	for _, c := range []struct {
		name     string
		priority uint32
	}{{"both 2000", 2000}, {"both unset", 0}} {
		t.Run(c.name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			dpA, dpB := &mockDP{}, &mockDP{}
			a := innerPeerSA(t, reg, "a", "198.18.0.7", selectorPolicy(t, "10.1.0.0/16", "0.0.0.0/0", 0, 0))
			a.PeerCfg.PolicyPriority = c.priority
			mustInnerChild(t, a, dpA, []string{"10.1.0.0/16"}, []string{"0.0.0.0/0"})

			b := innerPeerSA(t, reg, "b", "198.18.0.9", selectorPolicy(t, "10.1.2.0/24", "0.0.0.0/0", 0, 0))
			b.PeerCfg.PolicyPriority = c.priority
			_, err := innerAuthChild(t, b, dpB, []string{"10.1.2.0/24"}, []string{"0.0.0.0/0"})
			mustInnerRefused(t, err, "peer b")
			if !strings.Contains(err.Error(), "set vpn ipsec site-to-site peer b policy-priority <n>") {
				t.Errorf("refusal %q does not name the policy-priority fix", err)
			}
			if len(dpB.sas) != 0 || len(dpB.policies) != 0 {
				t.Fatalf("peer b's dataplane got %d states and %d policies, want none", len(dpB.sas), len(dpB.policies))
			}
			if got := policiesNaming(dpA, "10.1.0.0/16"); got != 2 {
				t.Fatalf("peer a holds %d policies naming 10.1.0.0/16, want 2 (in and out)", got)
			}
			if got := reg.holdings(); got != 1 {
				t.Fatalf("registry holds %d Child SAs, want 1 (peer a)", got)
			}
		})
	}
}

// TestInnerSelectorPeerChosenOverlapsFixedRefused: an operator-fixed holder does not
// make a peer-chosen overlap legal; both pairs must be operator-fixed (AC-6).
func TestInnerSelectorPeerChosenOverlapsFixedRefused(t *testing.T) {
	reg := newInnerSelectorRegistry()
	mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7", selectorPolicy(t, "10.1.0.0/16", "0.0.0.0/0", 0, 0)),
		&mockDP{}, []string{"10.1.0.0/16"}, []string{"0.0.0.0/0"})
	_, err := innerAuthChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), &mockDP{},
		[]string{"10.1.5.0/24"}, []string{"0.0.0.0/0"})
	mustInnerRefused(t, err, "peer b")
}

// TestInnerSelectorFirstChoiceKeptWhenLaterPairConflicts: b proposes TSi {10.1.2.4/32,
// 10.1.2.3/32} where a holds the second. The conflicting pair is dropped and the first
// choice answered alone (RFC 7296 Section 2.9, AC-7).
func TestInnerSelectorFirstChoiceKeptWhenLaterPairConflicts(t *testing.T) {
	reg := newInnerSelectorRegistry()
	mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	b := innerPeerSA(t, reg, "b", "198.18.0.9")
	mustInnerChild(t, b, &mockDP{}, []string{"10.1.2.4/32", "10.1.2.3/32"}, []string{"0.0.0.0/0"})
	if len(b.NegotiatedPairs) != 1 || b.NegotiatedPairs[0].I.Net.String() != "10.1.2.4/32" {
		t.Fatalf("peer b answered %s, want only the first choice 10.1.2.4/32", pairsText(b.NegotiatedPairs))
	}
}

// TestInnerSelectorSamePeerRekeyAllowed: a peer's Child SA rekey, through
// respondChildRekey with the live Child SA as the floor, and a second IKE SA of the same
// peer (re-authentication) proposing the same selectors, are both served (AC-8).
func TestInnerSelectorSamePeerRekeyAllowed(t *testing.T) {
	reg := newInnerSelectorRegistry()
	dp := &mockDP{}
	a := innerPeerSA(t, reg, "a", "198.18.0.7")
	old := mustInnerChild(t, a, dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	inner := []wire.PayloadEntry{
		{Payload: espSAPayload(0x01020304)},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(3)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, "10.1.2.3/32")},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0")},
	}
	if _, child, err := respondChildRekey(a, inner, old, 5, dp, slogutil.DiscardLogger()); err != nil || child == nil {
		t.Fatalf("respondChildRekey of the same peer: child %v, %v", child, err)
	}
	if got := reg.holdings(); got != 2 {
		t.Fatalf("registry holds %d Child SAs during make-before-break, want 2", got)
	}
	reauth := innerPeerSA(t, reg, "a", "198.18.0.7")
	mustInnerChild(t, reauth, dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
}

// TestInnerSelectorDistinctIfIDNoConflict: route-based peers on distinct if_id both
// negotiate 0.0.0.0/0 both ways. No packet can match both, so both install (AC-9).
// Method: the narrowing filter answers on the unbound if_id 0, and the install backstop
// on the if_id each Child SA carries, which is the decisive comparison here.
func TestInnerSelectorDistinctIfIDNoConflict(t *testing.T) {
	reg := newInnerSelectorRegistry()
	log := slogutil.DiscardLogger()
	for i, name := range []string{"a", "b"} {
		sa := innerPeerSA(t, reg, name, "198.18.0."+string(rune('7'+i)))
		if err := narrowChildSelectors(sa, tsPayload(t, wire.PayloadTypeTSi, "0.0.0.0/0"),
			tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0"), nil); err != nil {
			t.Fatalf("peer %s narrowing: %v", name, err)
		}
		if _, err := createFirstChildSA(sa, sa.ESPGroup, "192.0.2.1", sa.PeerCfg.RemoteAddress, uint32(10+i), &mockDP{}, log); err != nil {
			t.Fatalf("peer %s on if_id %d: %v", name, 10+i, err)
		}
	}
	// The control: the same two peers on one if_id collide.
	same := newInnerSelectorRegistry()
	for i, name := range []string{"a", "b"} {
		sa := innerPeerSA(t, same, name, "198.18.0."+string(rune('7'+i)))
		// The answer as narrowing records it, set directly so the install is what decides.
		sa.NegotiatedPairs = []tsPair{{I: tsSelector{Net: mustNet(t, "0.0.0.0/0")}, R: tsSelector{Net: mustNet(t, "0.0.0.0/0")}}}
		_, err := createFirstChildSA(sa, sa.ESPGroup, "192.0.2.1", sa.PeerCfg.RemoteAddress, 10, &mockDP{}, log)
		if i == 1 {
			mustInnerRefused(t, err, "peer b on the same if_id")
		}
	}
}

// TestInnerSelectorInstallBackstopRefusesSecond: two peers pass the narrowing filter for
// one inner address before either installs. The second install is refused before any
// dataplane call, with an error wrapping errTSUnacceptable (AC-10, R-2).
func TestInnerSelectorInstallBackstopRefusesSecond(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- two unbound tunnel peers that both passed narrowing for inner address 10.1.2.3/32 (TSr 0.0.0.0/0) before either installed: the second peer's install (createFirstChildSA) is answered TS_UNACCEPTABLE, its logged fix is the inner-selector conflict fix, and its dataplane receives no state or policy.
	reg := newInnerSelectorRegistry()
	a := innerPeerSA(t, reg, "a", "198.18.0.7")
	b := innerPeerSA(t, reg, "b", "198.18.0.9")
	for _, sa := range []*SA{a, b} {
		if err := narrowChildSelectors(sa, tsPayload(t, wire.PayloadTypeTSi, "10.1.2.3/32"),
			tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0"), nil); err != nil {
			t.Fatalf("peer %s narrowing: %v", sa.PeerName, err)
		}
	}
	log := slogutil.DiscardLogger()
	if _, err := createFirstChildSA(a, a.ESPGroup, "192.0.2.1", "198.18.0.7", 0, &mockDP{}, log); err != nil {
		t.Fatalf("peer a install: %v", err)
	}
	dpB := &mockDP{}
	_, err := createFirstChildSA(b, b.ESPGroup, "192.0.2.1", "198.18.0.9", 0, dpB, log)
	mustInnerRefused(t, err, "peer b install")
	// The refusal also wraps errChildInstall; the logged fix must still name the
	// conflict, never the kernel-install fix.
	if got := childRefusedResponderFix(err); got != childRefusedFixResponderInnerSelector {
		t.Fatalf("logged fix %q, want the inner-selector conflict fix", got)
	}
	if len(dpB.sas) != 0 || len(dpB.policies) != 0 {
		t.Fatalf("refused install reached the dataplane: %d states, %d policies", len(dpB.sas), len(dpB.policies))
	}
}

// TestRFC3948CreateChildSecondPeerSameInnerRefused: the CREATE_CHILD_SA responder
// (respondNewChild) refuses a second peer's new Child SA for an inner address another
// peer holds, with TS_UNACCEPTABLE.
func TestRFC3948CreateChildSecondPeerSameInnerRefused(t *testing.T) {
	reg := newInnerSelectorRegistry()
	mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	b := innerPeerSA(t, reg, "b", "198.18.0.9")
	inner := []wire.PayloadEntry{
		{Payload: espSAPayload(0x01020304)},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(4)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, "10.1.2.3/32")},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, "192.0.2.10/32")},
	}
	dpB := &mockDP{}
	_, child, err := respondNewChild(b, inner, b.ESPGroup, 7, dpB, slogutil.DiscardLogger())
	if child != nil {
		t.Fatalf("respondNewChild built a Child SA for peer b")
	}
	mustInnerRefused(t, err, "peer b CREATE_CHILD_SA")
	if len(dpB.policies) != 0 {
		t.Fatalf("peer b's dataplane got %d policies, want none", len(dpB.policies))
	}
}

// failingPolicyDP refuses every policy install, as a kernel that ran out of memory does.
type failingPolicyDP struct{ mockDP }

func (f *failingPolicyDP) InstallPolicy(dataplane.SPParams) error {
	return errors.New("policy install refused")
}

// TestInnerSelectorRegistryEmptyAfterTeardown: every Child SA removal path releases the
// record, so another peer can take the address afterwards (AC-11, A-3, R-1). Method: each
// removal funnel in turn ends peer a's Child SA, then peer b proposes a's address and
// must be served: removeChildSA (peer DELETE, clear, reload, failed answer encoding),
// removeChildSAExcept with a live survivor (rekey retire), cleanupChild (IKE SA death and
// every maintainSA exit), a nil dataplane, and a failed kernel install.
func TestInnerSelectorRegistryEmptyAfterTeardown(t *testing.T) {
	log := slogutil.DiscardLogger()
	ends := map[string]func(t *testing.T, a *SA, child *ChildSA, dp *mockDP){
		"remove": func(_ *testing.T, _ *SA, child *ChildSA, dp *mockDP) { removeChildSA(child, dp, log) },
		"rekey retire": func(t *testing.T, a *SA, child *ChildSA, dp *mockDP) {
			inner := []wire.PayloadEntry{
				{Payload: espSAPayload(0x01020304)},
				{Payload: &wire.PayloadNonce{NonceData: testNonce(3)}},
				{Payload: tsPayload(t, wire.PayloadTypeTSi, "10.1.2.3/32")},
				{Payload: tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0")},
			}
			_, next, err := respondChildRekey(a, inner, child, 5, dp, log)
			if err != nil {
				t.Fatalf("respondChildRekey: %v", err)
			}
			removeChildSAExcept(child, next, dp, log)
			removeChildSAExcept(next, nil, dp, log)
		},
		"ike sa death": func(_ *testing.T, _ *SA, child *ChildSA, dp *mockDP) {
			ps := &PeerSession{peerName: "a"}
			ps.setChildSA(child)
			ps.cleanupChild(dp, nil, log)
		},
		"nil dataplane": func(_ *testing.T, _ *SA, child *ChildSA, _ *mockDP) { removeChildSA(child, nil, log) },
	}
	for name, end := range ends {
		t.Run(name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			dp := &mockDP{}
			a := innerPeerSA(t, reg, "a", "198.18.0.7")
			child := mustInnerChild(t, a, dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
			end(t, a, child, dp)
			if got := reg.holdings(); got != 0 {
				t.Fatalf("registry holds %d Child SAs after %s, want 0", got, name)
			}
			mustInnerChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
		})
	}
	t.Run("failed install", func(t *testing.T) {
		reg := newInnerSelectorRegistry()
		if _, err := innerAuthChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), &failingPolicyDP{},
			[]string{"10.1.2.3/32"}, []string{"0.0.0.0/0"}); err == nil {
			t.Fatal("install on a refusing dataplane succeeded")
		}
		if got := reg.holdings(); got != 0 {
			t.Fatalf("registry holds %d Child SAs after a failed install, want 0", got)
		}
	})
}

// TestInnerSelectorRefusalNamesBothPeersAndFix: the refusal an operator reads names the
// refused peer and its pair, the holder and its pair, the RFC 3948 Section 5.1 reason,
// and both fix commands (AC-12).
func TestInnerSelectorRefusalNamesBothPeersAndFix(t *testing.T) {
	reg := newInnerSelectorRegistry()
	mustInnerChild(t, innerPeerSA(t, reg, "road-a", "198.18.0.7"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	_, err := innerAuthChild(t, innerPeerSA(t, reg, "road-b", "198.18.0.9"), &mockDP{},
		[]string{"10.1.2.3/32"}, []string{"192.0.2.10/32"})
	mustInnerRefused(t, err, "road-b")
	for _, want := range []string{
		"peer road-b proposed the tunnel local 192.0.2.10/32 remote 10.1.2.3/32",
		"the tunnel local 0.0.0.0/0 remote 10.1.2.3/32 that peer road-a already holds",
		"one packet could be sent into either tunnel (RFC 3948 Section 5.1)",
		"set vpn ipsec site-to-site peer road-b traffic-selector <n> remote prefix <prefix>",
		"set vpn ipsec site-to-site peer road-b virtual-ip-pool <pool>",
		"set vpn ipsec site-to-site peer road-b policy-priority <n>",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}
}

// TestInnerSelectorTransportModeRecordedAndLeftToDataplane: a transport-mode Child SA IS
// recorded, so a tunnel another peer negotiates around its address can be refused (RFC
// 3948 Section 5.1, AC-13), and two transport-mode Child SAs of two peers are not
// compared by the registry: their overlap is the dataplane's (RFC 3948 Section 5.2,
// policyOwners.claim), so the mock dataplane, which runs no such check, serves both.
func TestInnerSelectorTransportModeRecordedAndLeftToDataplane(t *testing.T) {
	reg := newInnerSelectorRegistry()
	mustInnerChild(t, transportPeerSA(t, reg, "b"), &mockDP{}, transportTSi, transportTSr)
	if got := reg.holdings(); got != 1 {
		t.Fatalf("registry holds %d Child SAs after a transport-mode install, want 1", got)
	}
	mustInnerChild(t, transportPeerSA(t, reg, "c"), &mockDP{}, transportTSi, transportTSr)
	if got := reg.holdings(); got != 2 {
		t.Fatalf("registry holds %d Child SAs after two transport-mode installs, want 2", got)
	}
}

// TestRFC3948LeaseVersusUnboundPeerFirstComeWins: a pool-bound peer's leased address and
// an unbound peer's proposed one are compared like any other pair, whichever arrives
// second is refused (AC-14, R-8). Method: the real IKE_AUTH with a CFG_REQUEST grants the
// lease; then the order is run both ways.
func TestRFC3948LeaseVersusUnboundPeerFirstComeWins(t *testing.T) {
	t.Run("lease first", func(t *testing.T) {
		reg := newInnerSelectorRegistry()
		ini, resp, ps := cpResponder(t, cpVIP(t, cpPool()), "pool", true)
		resp.innerSelectors = reg
		cpAuth(t, ini, resp, ps, cfgRequest(wire.ConfigAttr{Type: wire.CPAttrInternalIP4Address}))
		// if_id 0, the if_id resolveIfID gives a peer bound to no VTI: one packet can
		// then match both peers' pairs.
		if _, err := createFirstChildSA(resp, testESPGroup(), resp.PeerCfg.LocalAddress, resp.PeerCfg.RemoteAddress, 0,
			&mockDP{}, slogutil.DiscardLogger()); err != nil {
			t.Fatalf("pool peer install: %v", err)
		}
		lease := resp.VirtualIPs()[0].String() + "/32"
		_, err := innerAuthChild(t, innerPeerSA(t, reg, "unbound", "198.18.0.9"), &mockDP{},
			[]string{lease}, []string{"0.0.0.0/0"})
		mustInnerRefused(t, err, "unbound peer proposing the lease "+lease)
	})
	t.Run("unbound first", func(t *testing.T) {
		reg := newInnerSelectorRegistry()
		mustInnerChild(t, innerPeerSA(t, reg, "unbound", "198.18.0.9"), &mockDP{},
			[]string{"10.10.0.0/29"}, []string{"0.0.0.0/0"})
		ini, resp, ps := cpResponder(t, cpVIP(t, cpPool()), "pool", true)
		resp.innerSelectors = reg
		answer := cpAuth(t, ini, resp, ps, cfgRequest(wire.ConfigAttr{Type: wire.CPAttrInternalIP4Address}))
		refused := false
		for _, p := range answer {
			if n, ok := p.Payload.(*wire.PayloadNotify); ok && n.NotifyMsgType == wire.NotifyTSUnacceptable {
				refused = true
			}
		}
		if !refused {
			t.Fatal("the pool peer's IKE_AUTH Child SA was not answered TS_UNACCEPTABLE")
		}
	})
}

// TestInnerSelectorInitiatorInstallRefusedInstallsNothing: an install this node makes as
// the initiator, from a responder's answer, is refused by the backstop before any
// dataplane call when another peer holds the selectors (R-6).
func TestInnerSelectorInitiatorInstallRefusedInstallsNothing(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- an install this node makes as initiator (createFirstChildSA) from an adopted answer of local 192.0.2.0/24 and remote 10.1.2.3/32, while another peer's live Child SA holds inner address 10.1.2.3/32, is answered TS_UNACCEPTABLE and its dataplane receives no state or policy.
	reg := newInnerSelectorRegistry()
	mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	ini := innerPeerSA(t, reg, "spoke", "198.18.0.11")
	ini.IsInitiator = true
	// The answer adopted from the responder, TSi = this node's side.
	ini.NegotiatedPairs = []tsPair{{I: tsSelector{Net: mustNet(t, "192.0.2.0/24")}, R: tsSelector{Net: mustNet(t, "10.1.2.3/32")}}}
	dp := &mockDP{}
	_, err := createFirstChildSA(ini, ini.ESPGroup, "192.0.2.1", "198.18.0.11", 0, dp, slogutil.DiscardLogger())
	mustInnerRefused(t, err, "initiator install")
	if len(dp.sas) != 0 || len(dp.policies) != 0 {
		t.Fatalf("refused initiator install reached the dataplane: %d states, %d policies", len(dp.sas), len(dp.policies))
	}
}

// failingRemoveDP answers every policy removal with whatever err the case supplies.
type failingRemoveDP struct {
	mockDP
	err error
}

func (f *failingRemoveDP) RemovePolicyParams(dataplane.SPParams) error { return f.err }

// TestRFC3948FailedPolicyRemovalKeepsRecord: the record outlives the kernel policy. Method:
// peer a installs inner address 10.1.2.3/32 on a dataplane whose policy removal answers
// err, removeChildSA ends it, and peer b then proposes the same address. A failed removal
// (EIO) leaves a's policy in the kernel, so a's record must stay and b must be refused; a
// removal that policyGone accepts (ENOENT, no XFRM on the dataplane, another peer owns the
// selector) leaves nothing, so the record goes and b is served.
func TestRFC3948FailedPolicyRemovalKeepsRecord(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- peer a's Child SA for inner address 10.1.2.3/32 (TSr 0.0.0.0/0) is removed by removeChildSA on a dataplane whose policy removal fails with EIO: the registry still holds one record, and peer b proposing 10.1.2.3/32 is answered TS_UNACCEPTABLE with no state or policy on its dataplane; when the removal answers ENOENT the registry holds none and peer b is served.
	log := slogutil.DiscardLogger()
	cases := []struct {
		name string
		err  error
		kept bool
	}{
		{name: "removal fails", err: syscall.EIO, kept: true},
		{name: "policy already gone", err: syscall.ENOENT, kept: false},
		{name: "dataplane holds no policies", err: dataplane.ErrNotSupported, kept: false},
		{name: "another peer owns the selector", err: &dataplane.PolicyOwnedError{HeldBy: "x", Wanted: "a"}, kept: false},
		{name: "kernel offers no XFRM", err: syscall.EPROTONOSUPPORT, kept: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			dp := &failingRemoveDP{err: tc.err}
			child := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
			// A failed removal stays in the process-wide SPD ledger (spd_left.go); a
			// successful one drops it, so the next test starts from an empty ledger.
			leftover := append(childPolicies(child, dataplane.SADirOut), childPolicies(child, dataplane.SADirIn)...)
			t.Cleanup(func() {
				for i := range leftover {
					spdLeft.record(&leftover[i], nil)
				}
			})
			removeChildSA(child, dp, log)
			want := 0
			if tc.kept {
				want = 1
			}
			if got := reg.holdings(); got != want {
				t.Fatalf("registry holds %d Child SAs after a removal answered %v, want %d", got, tc.err, want)
			}
			dpB := &mockDP{}
			_, err := innerAuthChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), dpB, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
			if !tc.kept {
				if err != nil {
					t.Fatalf("peer b after a removal answered %v: %v", tc.err, err)
				}
				return
			}
			mustInnerRefused(t, err, "peer b while a's policy is still installed")
			if len(dpB.sas) != 0 || len(dpB.policies) != 0 {
				t.Fatalf("refused peer b reached its dataplane: %d states, %d policies", len(dpB.sas), len(dpB.policies))
			}
		})
	}
}

// TestInnerSelectorPairsOfNoSelectorsMatchesPolicy: a Child SA with no negotiated pairs is
// recorded as the policy childPolicies installs from TSLocal and TSRemote, not reversed.
// Method: compare the recorded outbound policy with the installed one, and check the
// pair the operator configured is classified operator-fixed.
func TestInnerSelectorPairsOfNoSelectorsMatchesPolicy(t *testing.T) {
	local, remote := mustNet(t, "192.0.2.0/24"), mustNet(t, "10.1.2.3/32")
	child := &ChildSA{
		Owner: "a", Mode: modeTunnel, TSLocal: local, TSRemote: remote,
		innerGuard: innerSelectorGuard{configured: []policyPair{{local: local, remote: remote}}},
	}
	pairs := innerSelectorPairsOf(child)
	if len(pairs) != 1 {
		t.Fatalf("recorded %d pairs, want 1", len(pairs))
	}
	installed := childPolicies(child, dataplane.SADirOut)[0]
	if !reflect.DeepEqual(pairs[0].policy, installed) {
		t.Fatalf("recorded policy %v -> %v, installed %v -> %v", pairs[0].policy.Src, pairs[0].policy.Dst,
			installed.Src, installed.Dst)
	}
	if !pairs[0].operatorFixed {
		t.Fatal("the configured pair 192.0.2.0/24 to 10.1.2.3/32 was not classified operator-fixed")
	}
}

// TestInnerSelectorDeclinedTransportRequestFiltered: a transport request this node
// declines installs a tunnel, so the narrowing filter runs for it. Method: peer a holds
// 10.1.2.3/32; peer b, configured for tunnel mode, requests transport mode for the same
// address; narrowChildSelectors alone must answer TS_UNACCEPTABLE.
func TestInnerSelectorDeclinedTransportRequestFiltered(t *testing.T) {
	reg := newInnerSelectorRegistry()
	mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	b := innerPeerSA(t, reg, "b", "198.18.0.9")
	b.UseTransportMode = decideResponderTransportMode(b, true) == modeTransport
	if b.UseTransportMode {
		t.Fatal("setup: a tunnel-mode peer's transport request was accepted")
	}
	err := narrowChildSelectors(b, tsPayload(t, wire.PayloadTypeTSi, "10.1.2.3/32"),
		tsPayload(t, wire.PayloadTypeTSr, "192.0.2.10/32"), nil)
	mustInnerRefused(t, err, "peer b's declined transport request at narrowing")
}

// TestInnerSelectorNilDataplaneRecorded: with no dataplane, a Child SA is still recorded,
// so the control plane never holds two peers' Child SAs for one inner address. Method:
// peer a and then peer b negotiate 10.1.2.3/32 with a nil dataplane; b must be refused,
// and removing a must release the record.
func TestInnerSelectorNilDataplaneRecorded(t *testing.T) {
	reg := newInnerSelectorRegistry()
	child := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), nil, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	if got := reg.holdings(); got != 1 {
		t.Fatalf("registry holds %d Child SAs with no dataplane, want 1", got)
	}
	_, err := innerAuthChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), nil, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	mustInnerRefused(t, err, "peer b with no dataplane")
	removeChildSA(child, nil, slogutil.DiscardLogger())
	if got := reg.holdings(); got != 0 {
		t.Fatalf("registry holds %d Child SAs after removal, want 0", got)
	}
}

// TestInnerSelectorInitiatorRefusalKeepsIKESA: a first Child SA this node refuses as the
// IKE_AUTH initiator, because another peer holds an overlapping tunnel, costs the Child
// SA only. Method: establish an initiator SA against a stand-in responder, record another
// peer holding the negotiated selectors, run runEstablished; it must return cleanly with
// no Child SA and no state installed, the IKE SA established, and an ESP Delete for this
// node's inbound SPI sent to the peer (RFC 7296 Section 2.21.2).
func TestInnerSelectorInitiatorRefusalKeepsIKESA(t *testing.T) {
	quiet := slogutil.DiscardLogger()
	dp := &mockDP{}
	loadFailClosedBackend(t, dp)
	peerTr, myTr := encapNATTPeerLink(t)
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "inner-refusal-initiator")
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	ini, resp, table := encapSAInit(t, iniPeer, respPeer)
	respNATT, _ := bindNATT(t, resp)
	bindNATT(t, ini)
	respPS := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	respPS.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, respNATT, encapPeerAt(encapTestPeerPort), quiet)
	dp.sas = nil
	handleInbound(ini, transport.Packet{Data: resp.LastSentMsg, RemoteAddr: nttPeerAddr(t, peerTr), NATT: true}, table, myTr, quiet)
	if ini.State != StateEstablished || ini.IKEAuthChildless {
		t.Fatalf("setup: initiator state %v childless=%v, want established with a Child SA", ini.State, ini.IKEAuthChildless)
	}

	reg := newInnerSelectorRegistry()
	ini.innerSelectors = reg
	ifID, err := resolveIfID(&iniPeer)
	if err != nil {
		t.Fatalf("resolveIfID: %v", err)
	}
	holder := &ChildSA{
		Owner: "other", Mode: modeTunnel, IfID: ifID, PolicyPriority: iniPeer.PolicyPriority,
		Selectors: ini.NegotiatedPairs, SelectorsLocalIsTSi: true,
	}
	if err := reg.beginInstall(holder, "other", innerSelectorPairsOf(holder)); err != nil {
		t.Fatalf("setup: record the other peer: %v", err)
	}
	reg.finishInstall(holder, nil)

	ps := &PeerSession{peerName: "ze", peerCfg: iniPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	encapRunEstablished(t, ps, ini, table, myTr)
	if len(dp.sas) != 0 {
		t.Errorf("%d states installed, want none", len(dp.sas))
	}
	if ps.getChildSA() != nil {
		t.Error("the refused Child SA was adopted")
	}
	if ini.State != StateEstablished {
		t.Errorf("the IKE SA is %v, want established", ini.State)
	}
	if !deleteListsSPI(t, resp, peerTr, ini.ChildInboundSPI) {
		t.Errorf("no ESP Delete listing ze's inbound SPI %#x reached the peer", ini.ChildInboundSPI)
	}
}

// TestInnerSelectorOverlapSymmetric: comparing the outbound policies of two Child SAs
// gives the verdict comparing their inbound policies gives, so recording one direction
// is enough (A-6).
func TestInnerSelectorOverlapSymmetric(t *testing.T) {
	pairs := [][2]string{
		{"10.1.2.3/32", "0.0.0.0/0"}, {"10.1.2.3/32", "192.0.2.10/32"}, {"10.1.2.4/32", "0.0.0.0/0"},
		{"10.1.0.0/16", "198.51.100.0/24"}, {"2001:db8::1/128", "::/0"},
	}
	child := func(remote, local string) *ChildSA {
		return &ChildSA{Owner: "x", Selectors: []tsPair{{I: tsSelector{Net: mustNet(t, remote)}, R: tsSelector{Net: mustNet(t, local)}}}}
	}
	for _, x := range pairs {
		for _, y := range pairs {
			a, b := child(x[0], x[1]), child(y[0], y[1])
			out := dataplane.PolicySelectorsOverlap(childPolicies(a, dataplane.SADirOut)[0], childPolicies(b, dataplane.SADirOut)[0])
			in := dataplane.PolicySelectorsOverlap(childPolicies(a, dataplane.SADirIn)[0], childPolicies(b, dataplane.SADirIn)[0])
			if out != in {
				t.Errorf("%v against %v: outbound overlap %v, inbound %v", x, y, out, in)
			}
		}
	}
}
