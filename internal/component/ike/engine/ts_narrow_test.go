package engine

// rfc-test-change-approved: 2026-07-31 owner standing approval for
// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only. Every tag in this file is
// NEW in this package; the edits below build it, they never relax an existing proof.

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

func mustNet(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return n
}

func sel(t *testing.T, cidr string) tsSelector {
	t.Helper()
	return tsSelector{Net: mustNet(t, cidr), Port: ipsec.AnyPort()}
}

func selPort(t *testing.T, cidr string, p ipsec.PortSelector, proto uint8) tsSelector {
	t.Helper()
	return tsSelector{Net: mustNet(t, cidr), Port: p, Proto: proto}
}

// wireSel builds a received selector the way a peer would send it.
func wireSel(t *testing.T, cidr string, startPort, endPort uint16, proto uint8) wire.TrafficSelector {
	t.Helper()
	n := mustNet(t, cidr)
	start, end := netRange(n)
	ttype := wire.TSTypeIPv4AddrRange
	if len(start) == 16 {
		ttype = wire.TSTypeIPv6AddrRange
	}
	return wire.TrafficSelector{
		TSType:       ttype,
		IPProtocol:   proto,
		StartPort:    startPort,
		EndPort:      endPort,
		StartAddress: start,
		EndAddress:   end,
	}
}

// TestNarrowingIncludesFirstChoice drives RFC 7296 Section 2.9's third bullet.
//
// VALIDATES: a proposal whose first selector the policy accepts is answered with a
// subset of the proposal that INCLUDES that first choice, at the head of the answer.
// PREVENTS: the responder answering with a wildcard (today's behavior), and the
// responder answering with SOME acceptable subset that omits the first choice.
func TestNarrowingIncludesFirstChoice(t *testing.T) {
	// The initiator's first choice is 10.1.0.0/16 <-> 10.2.0.0/16; a second, less
	// preferred pair follows.
	proposedI := []tsSelector{sel(t, "10.1.0.0/16"), sel(t, "10.9.0.0/16")}
	proposedR := []tsSelector{sel(t, "10.2.0.0/16"), sel(t, "10.8.0.0/16")}

	// Policy accepts the first choice (a /8 covering it) and also the second pair.
	policy := []tsPair{
		{I: sel(t, "10.0.0.0/8"), R: sel(t, "10.0.0.0/8")},
	}

	// RFC requirement: RFC7296-2.9-2 positive -- "If the responder's policy allows it to
	// accept the first selector of TSi and TSr, then the responder MUST narrow the Traffic
	// Selectors to a subset that includes the initiator's first choices" (RFC 7296 S2.9,
	// rfc/full/rfc7296.txt:2434-2436). narrowSelectors returns a set whose every entry is a
	// subset of the proposal, and whose FIRST entry covers proposedI[0]/proposedR[0].
	got, ok := narrowSelectors(proposedI, proposedR, policy, nil)
	if !ok {
		t.Fatal("narrowSelectors refused a proposal the policy accepts")
	}
	if len(got) == 0 {
		t.Fatal("narrowSelectors returned an empty set with ok=true")
	}
	if got[0].I.Net.String() != "10.1.0.0/16" || got[0].R.Net.String() != "10.2.0.0/16" {
		t.Errorf("first answered pair = %v <-> %v, want the initiator's first choice 10.1.0.0/16 <-> 10.2.0.0/16",
			got[0].I.Net, got[0].R.Net)
	}
	// Never wider than the proposal.
	for i, p := range got {
		if !coveredBy(p.I, proposedI) {
			t.Errorf("answered pair %d TSi %v is not a subset of any proposed TSi", i, p.I.Net)
		}
		if !coveredBy(p.R, proposedR) {
			t.Errorf("answered pair %d TSr %v is not a subset of any proposed TSr", i, p.R.Net)
		}
	}

	// RFC requirement: RFC7296-2.9-2 negative -- the discriminator. When the policy does
	// NOT accept the initiator's first choice, RFC 7296 S2.9's fourth bullet still asks for
	// an acceptable subset rather than a refusal, so the answer is non-empty and its first
	// entry is NOT proposedI[0]. This separates "narrowed to include the first choice" from
	// "always echo the first selector".
	narrowPolicy := []tsPair{
		{I: sel(t, "10.9.0.0/16"), R: sel(t, "10.8.0.0/16")},
	}
	got2, ok2 := narrowSelectors(proposedI, proposedR, narrowPolicy, nil)
	if !ok2 {
		t.Fatal("narrowSelectors refused a proposal whose SECOND selector the policy accepts")
	}
	if got2[0].I.Net.String() == "10.1.0.0/16" {
		t.Error("answer led with the initiator's first choice, but policy rejects it; the first-choice branch is not gated")
	}
	if got2[0].I.Net.String() != "10.9.0.0/16" {
		t.Errorf("answered TSi = %v, want the acceptable second choice 10.9.0.0/16", got2[0].I.Net)
	}
}

// TestNarrowingEmptySetIsRefused drives RFC 7296 Section 2.9's first bullet.
//
// VALIDATES: a proposal disjoint from policy narrows to nothing, and the engine reports
// that rather than answering with something.
// PREVENTS: the responder answering a disjoint proposal with a wildcard, which is the
// widening defect this package removes.
func TestNarrowingEmptySetIsRefused(t *testing.T) {
	proposedI := []tsSelector{sel(t, "192.168.0.0/16")}
	proposedR := []tsSelector{sel(t, "192.168.1.0/24")}
	policy := []tsPair{{I: sel(t, "10.0.0.0/8"), R: sel(t, "10.0.0.0/8")}}

	if got, ok := narrowSelectors(proposedI, proposedR, policy, nil); ok {
		t.Errorf("narrowSelectors accepted a disjoint proposal and answered %v", got)
	}

	// The discriminator: an OVERLAPPING proposal is accepted, so the refusal is a
	// decision about this proposal rather than a blanket refusal.
	okI := []tsSelector{sel(t, "10.1.0.0/16")}
	okR := []tsSelector{sel(t, "10.2.0.0/16")}
	if _, ok := narrowSelectors(okI, okR, policy, nil); !ok {
		t.Error("narrowSelectors refused an overlapping proposal; the refusal is unconditional")
	}
}

// TestNarrowingUnconfiguredPeerAllowsEverything pins the load-bearing default.
//
// VALIDATES: a peer with no configured traffic-selector list accepts whatever the
// initiator proposes, and answers with the proposal itself.
// PREVENTS: narrowing an unconfigured peer to the empty set, which would answer every
// configuration written before this list existed with TS_UNACCEPTABLE.
func TestNarrowingUnconfiguredPeerAllowsEverything(t *testing.T) {
	proposedI := []tsSelector{sel(t, "10.1.0.0/16")}
	proposedR := []tsSelector{sel(t, "10.2.0.0/16")}

	got, ok := narrowSelectors(proposedI, proposedR, nil, nil)
	if !ok {
		t.Fatal("an unconfigured peer refused a proposal; every pre-existing config would break")
	}
	if len(got) != 1 {
		t.Fatalf("answered pairs = %d, want 1", len(got))
	}
	if got[0].I.Net.String() != "10.1.0.0/16" || got[0].R.Net.String() != "10.2.0.0/16" {
		t.Errorf("answer = %v <-> %v, want the proposal echoed back", got[0].I.Net, got[0].R.Net)
	}
}

// TestRekeyFloorIsNotNarrowed drives both RFC 7296 Section 2.9.2 MUST NOTs.
//
// VALIDATES: a rekey answer is never narrower than the scope currently in use, even when
// the peer proposes less and even when the policy has since narrowed.
// PREVENTS: a rekey silently shrinking a working SA's scope.
func TestRekeyFloorIsNotNarrowed(t *testing.T) {
	inUse := []tsPair{{I: sel(t, "10.1.0.0/16"), R: sel(t, "10.2.0.0/16")}}

	// The peer proposes a wider set; policy would allow the wider set too.
	proposedI := []tsSelector{sel(t, "10.0.0.0/8")}
	proposedR := []tsSelector{sel(t, "10.0.0.0/8")}
	policy := []tsPair{{I: sel(t, "10.0.0.0/8"), R: sel(t, "10.0.0.0/8")}}

	// RFC requirement: RFC7296-2.9.2-1 positive -- "Thus, the new SA MUST NOT have narrower
	// selectors than the original" (RFC 7296 S2.9.2, rfc/full/rfc7296.txt:2539-2540). With a
	// floor set, the answer covers the in-use scope.
	got, ok := narrowSelectors(proposedI, proposedR, policy, inUse)
	if !ok {
		t.Fatal("rekey narrowing refused a proposal that covers the scope in use")
	}
	if !containsNet(got[0].I.Net, inUse[0].I.Net) {
		t.Errorf("rekey answer TSi %v does not cover the in-use scope %v", got[0].I.Net, inUse[0].I.Net)
	}

	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only. This ADDS the missing
	// negative polarity; it relaxes nothing.
	//
	// RFC requirement: RFC7296-2.9.2-1 negative -- the discriminator. A rekey proposing
	// EXACTLY the scope in use is answered with that scope, and is not widened to the
	// policy's /8. The floor is a floor, not a constant that replaces every answer.
	sameI := []tsSelector{sel(t, "10.1.0.0/16")}
	sameR := []tsSelector{sel(t, "10.2.0.0/16")}
	same, okSame := narrowSelectors(sameI, sameR, policy, inUse)
	if !okSame {
		t.Fatal("rekey narrowing refused a proposal equal to the scope in use")
	}
	if same[0].I.Net.String() != "10.1.0.0/16" || same[0].R.Net.String() != "10.2.0.0/16" {
		t.Errorf("rekey answer for an unchanged proposal = %v <-> %v, want the same 10.1.0.0/16 <-> 10.2.0.0/16",
			same[0].I.Net, same[0].R.Net)
	}

	// RFC requirement: RFC7296-2.9.2-2 positive -- "The responder MUST NOT narrow down the
	// Traffic Selectors narrower than the scope currently in use" (RFC 7296 S2.9.2,
	// rfc/full/rfc7296.txt:2551-2552). Policy has since narrowed to a /24, and the rekey
	// answer still carries the /16 in use rather than the /24 policy would now allow.
	narrowedPolicy := []tsPair{{I: sel(t, "10.1.1.0/24"), R: sel(t, "10.2.1.0/24")}}
	gotFloor, okFloor := narrowSelectors(proposedI, proposedR, narrowedPolicy, inUse)
	if !okFloor {
		t.Fatal("rekey narrowing refused when policy narrowed below the scope in use")
	}
	if gotFloor[0].I.Net.String() != "10.1.0.0/16" {
		t.Errorf("rekey answer TSi = %v, want the in-use /16; policy narrowed below the floor and the floor lost",
			gotFloor[0].I.Net)
	}

	// RFC requirement: RFC7296-2.9.2-2 negative -- the discriminator. A FRESH Child SA
	// (floor nil) with the SAME narrowed policy IS narrowed to the /24. This proves the
	// floor is rekey-specific rather than a blanket refusal to narrow.
	fresh, okFresh := narrowSelectors(proposedI, proposedR, narrowedPolicy, nil)
	if !okFresh {
		t.Fatal("fresh narrowing refused a proposal the narrowed policy accepts")
	}
	if fresh[0].I.Net.String() != "10.1.1.0/24" {
		t.Errorf("fresh answer TSi = %v, want the narrowed policy's /24; the floor is being applied without a rekey",
			fresh[0].I.Net)
	}
}

// TestNarrowingNeverWidensAnUnprogrammableProposal drives exact-or-reject at negotiation
// time.
//
// VALIDATES: a peer-proposed port range Ze cannot program is narrowed to a subset, never
// rounded outward to ANY; a non-CIDR address range becomes the largest prefix INSIDE it.
// PREVENTS: the wire and the dataplane disagreeing, which is the defect this package
// exists to remove.
func TestNarrowingNeverWidensAnUnprogrammableProposal(t *testing.T) {
	// A peer proposes ports 1024..2048 over TCP. XFRM carries a port plus a mask, so an
	// arbitrary inclusive range has no exact form.
	got := wireToSelectors([]wire.TrafficSelector{wireSel(t, "10.1.0.0/16", 1024, 2048, 6)})
	if len(got) != 1 {
		t.Fatalf("converted selectors = %d, want 1", len(got))
	}
	if got[0].Port.Form == ipsec.PortAny {
		t.Fatal("an unprogrammable port range 1024..2048 was widened to ANY; that answers with more traffic than was proposed")
	}
	if got[0].Port.Form != ipsec.PortSingle || got[0].Port.Port != 1024 {
		t.Errorf("port narrowed to %v, want the single port 1024 (a subset of 1024..2048)", got[0].Port)
	}

	// A non-CIDR address range narrows INWARD.
	nonCIDR := wire.TrafficSelector{
		TSType:       wire.TSTypeIPv4AddrRange,
		IPProtocol:   0,
		StartPort:    0,
		EndPort:      65535,
		StartAddress: []byte{10, 0, 0, 5},
		EndAddress:   []byte{10, 0, 0, 9},
	}
	inward := wireToSelectors([]wire.TrafficSelector{nonCIDR})
	if len(inward) != 1 {
		t.Fatalf("converted non-CIDR selectors = %d, want 1", len(inward))
	}
	ones, _ := inward[0].Net.Mask.Size()
	if ones < 30 {
		t.Errorf("non-CIDR range 10.0.0.5-10.0.0.9 became %v (/%d); a prefix that short is wider than the proposal",
			inward[0].Net, ones)
	}
	if !inward[0].Net.Contains(net.IP{10, 0, 0, 5}) {
		t.Errorf("narrowed prefix %v does not contain the proposed start address 10.0.0.5", inward[0].Net)
	}
}

// TestPortEncodingFollowsSection3131 drives all three RFC 7296 Section 3.13.1 port MUSTs.
//
// VALIDATES: the ANY encoding is 0/65535, a configured single port is emitted as N/N,
// and the OPAQUE encoding is 65535/0.
// PREVENTS: a hardcoded 0/65535 passing as conformance, and OPAQUE being emitted as ANY.
func TestPortEncodingFollowsSection3131(t *testing.T) {
	sels := []tsSelector{
		selPort(t, "10.1.0.0/16", ipsec.AnyPort(), 0),
		selPort(t, "10.2.0.0/16", ipsec.PortSelector{Form: ipsec.PortSingle, Port: 443}, 6),
		selPort(t, "10.3.0.0/16", ipsec.PortSelector{Form: ipsec.PortOpaque}, 6),
	}
	payload := selectorsToWire(sels, wire.PayloadTypeTSi)
	if payload == nil {
		t.Fatal("selectorsToWire returned no payload")
	}
	// Anti-vacuity guard: the sweeps below must run over a non-empty set.
	if len(payload.TrafficSelectors) != 3 {
		t.Fatalf("encoded selectors = %d, want 3; the assertions below would sweep over the wrong set",
			len(payload.TrafficSelectors))
	}

	anyTS, singleTS, opaqueTS := payload.TrafficSelectors[0], payload.TrafficSelectors[1], payload.TrafficSelectors[2]

	// RFC requirement: RFC7296-3.13.1-1 positive -- "For protocols for which port is
	// undefined (including protocol 0), or if all ports are allowed, this field MUST be
	// zero" (RFC 7296 S3.13.1 Start Port, rfc/full/rfc7296.txt:6033-6036).
	if anyTS.IPProtocol != 0 {
		t.Fatalf("fixture protocol = %d, want 0 so the MUST's antecedent holds", anyTS.IPProtocol)
	}
	if anyTS.StartPort != 0 {
		t.Errorf("all-ports selector StartPort = %d, want 0", anyTS.StartPort)
	}
	// RFC requirement: RFC7296-3.13.1-1 negative -- the discriminator. The encoder CAN emit
	// a non-zero Start Port for a protocol that defines ports, so the zero above is a
	// decision taken because the protocol is 0, not a constant the encoder cannot leave.
	if singleTS.StartPort != 443 {
		t.Errorf("single-port selector StartPort = %d, want 443; the encoder cannot express a specific port, so the zero above proves nothing",
			singleTS.StartPort)
	}

	// RFC requirement: RFC7296-3.13.1-2 positive -- "For protocols for which port is
	// undefined (including protocol 0), or if all ports are allowed, this field MUST be
	// 65535" (RFC 7296 S3.13.1 End Port, rfc/full/rfc7296.txt:6055-6058).
	if anyTS.EndPort != 65535 {
		t.Errorf("all-ports selector EndPort = %d, want 65535", anyTS.EndPort)
	}
	// RFC requirement: RFC7296-3.13.1-2 negative -- the discriminator. The encoder CAN emit
	// an End Port other than 65535, so the 65535 above is a decision taken because the
	// protocol is 0, not a constant.
	if singleTS.EndPort != 443 {
		t.Errorf("single-port selector EndPort = %d, want 443; the encoder cannot express a specific port, so the 65535 above proves nothing",
			singleTS.EndPort)
	}

	// rfc-test-change-approved: 2026-08-01 owner standing approval for
	// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only. The block below was
	// deliberately untagged while an owner decision was open. The owner ruled on
	// 2026-08-01 that the row LANDS as encoder-proven. The tags are added, and nothing
	// else moves.
	//
	// RFC requirement: RFC7296-3.13.1-3 positive -- "Systems that wish to indicate
	// 'OPAQUE' ports, but not 'ANY' ports, MUST set the start port to 65535 and the end
	// port to 0" (RFC 7296 S3.13.1, rfc/full/rfc7296.txt:6074-6079).
	//
	// The obligation is an ENCODING rule. It is proven at the layer that owns the
	// encoding. ipsec.PortSelector.Wire (ipsec/traffic_selector.go) is the single
	// producer of every port pair Ze puts on the wire, and selectorsToWire is its only
	// caller. A PortOpaque selector therefore encodes as 65535/0 wherever one appears.
	//
	// Ze never WISHES to indicate OPAQUE. That refusal is Ze's conformant behavior and
	// not a hole. No dataplane backend can program an opaque-port policy EXACTLY.
	//
	// The vendored netlink derives the port mask from the port VALUE. selFromPolicy
	// (vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go:117-124) sets DportMask
	// only when the port is non-zero. An exact match on port 0 is therefore
	// inexpressible, and it would install as ANY port. That is WIDER than the selector
	// negotiated, which RFC 7296 Section 2.9 forbids.
	//
	// Ze refuses the form at commit (ipsec.checkPortProgrammable). It drops the form at
	// negotiation (programmableSelector). That is ai/rules/exact-or-reject.md applied
	// correctly. A backend that cannot deliver the operator's config exactly must reject
	// it, and must never approximate it.
	//
	// WHAT WOULD FALSIFY THIS: PortSelector.Wire returning any pair other than 65535/0
	// for PortOpaque, or a second producer of wire ports that bypasses it.
	if opaqueTS.StartPort != 65535 || opaqueTS.EndPort != 0 {
		t.Errorf("opaque selector ports = %d/%d, want 65535/0", opaqueTS.StartPort, opaqueTS.EndPort)
	}
	// RFC requirement: RFC7296-3.13.1-3 negative -- the discriminator. It rests on a
	// property the encoder HAS and not on a guard that is absent. The encoder emits a
	// DIFFERENT pair for ANY (0/65535, asserted above). 65535/0 is therefore the form it
	// chooses for OPAQUE alone, and not a constant it cannot leave. If the two were
	// aliased, the positive would prove nothing. "Wish to indicate OPAQUE ports, but not
	// ANY ports" is exactly the distinction the MUST exists to draw.
	if anyTS.StartPort == 65535 && anyTS.EndPort == 0 {
		t.Error("an ANY selector was emitted in the OPAQUE form; the two encodings are not distinguished")
	}
}

// TestNarrowedSelectorsReachTheInstalledPolicy re-binds the RFC 4301 obligation that
// TestNarrowTS used to carry. narrowTS had no non-test caller, so its tag proved nothing
// about an installed Child SA.
//
// VALIDATES: the selectors narrowSelectors returns are the selectors the Child SA
// install writes into the inbound policy.
// PREVENTS: the wire and the dataplane carrying different selectors.
func TestNarrowedSelectorsReachTheInstalledPolicy(t *testing.T) {
	proposedI := []tsSelector{sel(t, "10.1.0.0/16")}
	proposedR := []tsSelector{sel(t, "10.2.0.0/16")}
	policy := []tsPair{{I: sel(t, "10.0.0.0/8"), R: sel(t, "10.0.0.0/8")}}

	// RFC requirement: RFC4301-4.4.2-1 positive -- inbound SAD/SPD selectors come from the
	// negotiated (narrowed) traffic selector: narrowSelectors produces the /16 intersection
	// and createFirstChildSA writes it into the inbound policy (child.go, via
	// sa.NegotiatedTSi/TSr).
	narrowed, ok := narrowSelectors(proposedI, proposedR, policy, nil)
	if !ok {
		t.Fatal("narrowSelectors refused an acceptable proposal")
	}
	if narrowed[0].I.Net.String() != "10.1.0.0/16" {
		t.Fatalf("narrowed TSi = %v, want 10.1.0.0/16", narrowed[0].I.Net)
	}

	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only. This test is NEW and it
	// RE-BINDS RFC4301-4.4.2-1 from TestNarrowTS, whose subject (narrowTS) had no
	// non-test caller, onto the narrowing engine that the responder actually calls.
	sa := testSA()
	sa.IsInitiator = true
	sa.NegotiatedTSi = narrowed[0].I.Net
	sa.NegotiatedTSr = narrowed[0].R.Net
	dp := &mockDP{}
	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only.
	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 7, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	defer child.Clear()

	if len(dp.policies) == 0 {
		t.Fatal("no policy was installed; the assertion below would sweep over nothing")
	}
	inPol := dp.policies[0]
	if inPol.Dst.String() != narrowed[0].I.Net.String() {
		t.Errorf("inbound policy Dst = %v, want the narrowed TSi %v", inPol.Dst, narrowed[0].I.Net)
	}
	if inPol.Src.String() != narrowed[0].R.Net.String() {
		t.Errorf("inbound policy Src = %v, want the narrowed TSr %v", inPol.Src, narrowed[0].R.Net)
	}
}
