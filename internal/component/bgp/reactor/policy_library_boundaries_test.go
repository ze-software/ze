// Design: docs/architecture/core-design.md -- Policy Filter Chain
// Related: filter_answer_transport_test.go -- answerTransport, answerPolicy
// Related: filter_library_intents_test.go -- libraryIntentEval, the intent member

package reactor

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	bgpfilter "github.com/ze-software/ze/internal/component/bgp/reactor/loopdetect"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// boundaryRef is one answerPlugin filter of a boundaryPolicy chain, or, when
// stage is set, a reference an in-process stage owns (the automatically
// inserted loop-detection check): bound in-process and no library member, as
// compileLibraryChain skips every Policy.InProcess reference.
type boundaryRef struct {
	name     string
	inactive bool
	stage    *routepolicy.Ref
}

// boundaryPolicy compiles refs as bgp/config compileLibraryChain does: an
// inactive ref is no member, a chain left with no member carries no library
// chain, and final 0 is a direction the operator never configured, whose
// chain ends by the session (RFC 8212 Section 3).
func boundaryPolicy(t *testing.T, final routepolicy.Final, refs ...boundaryRef) filterapi.Policy {
	t.Helper()
	policyRefs := make([]routepolicy.Ref, len(refs))
	var members []policychain.Member
	var staged []routepolicy.Ref
	for i, ref := range refs {
		if ref.stage != nil {
			policyRefs[i] = *ref.stage
			staged = append(staged, *ref.stage)
			continue
		}
		policyRefs[i] = routepolicy.Ref{Name: answerPlugin + ":" + ref.name, Inactive: ref.inactive}
		if !ref.inactive {
			members = append(members, policychain.Member{Name: ref.name, Plugin: answerPlugin})
		}
	}
	policy := filterapi.InternalPolicy(policyRefs)
	if final != 0 {
		policy = mustPolicy(t, policyRefs, final)
	}
	if len(staged) > 0 {
		policy = bindRefs(policy, staged...)
	}
	if len(members) == 0 {
		return policy
	}
	reg, err := routepolicy.NewRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	bases := policychain.Bases{LocalPreference: 100}
	var chain *policychain.Chain
	if final == 0 {
		chain, err = policychain.CompileUnconfigured(answerAttachment, reg, members, bases)
	} else {
		chain, err = policychain.Compile(answerAttachment, reg, members, final, bases)
	}
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return policy.WithLibrary(chain)
}

// TestLibraryRFC8212ChainBoundaries is the library form of the text chain's
// RFC 8212 boundary test. It runs the import step over every session type and
// final: an absent final on EBGP rejects whatever the members answer, and
// neither an inactive member nor a raw rewrite grants it acceptance (RFC 8212
// Section 3: "Routes SHALL NOT be added to an Adj-RIB-In associated with an
// EBGP peer if no explicit Import Policy has been applied."). A reject ends
// the chain before the later member is called, and a failure or a teardown is
// terminal under every final. An automatically inserted loop-detection
// reference is no member: it grants nothing, and show policy test marks it
// not-evaluated. In every case the display-only run (Chain.Trace, and show
// policy test over the whole step) reaches the live run's outcome with the
// same filter calls.
//
// VALIDATES: reject short-circuits later members; an inactive member is never
// called; a failure or teardown is never replaced by the final; the automatic
// reference supplies no policy presence; trace and live run agree.
// PREVENTS: a library chain that runs past a reject, calls an inactive member,
// lets the final rewrite a terminal verdict, counts the automatic check as an
// import policy, or a dry-run that predicts a different verdict than the
// session applies.
func TestLibraryRFC8212ChainBoundaries(t *testing.T) {
	received := libraryTestBody(100)
	raw := libraryTestBody(200)
	two := []boundaryRef{{name: "first"}, {name: "last"}}
	automatic, bound := bgpfilter.BindLoopDetection(routepolicy.Ref{Name: "AUTO"}, "loop-detection")
	if !bound {
		t.Fatal("loop registration did not bind the automatic reference")
	}
	cases := []struct {
		name   string
		refs   []boundaryRef
		raw    bool
		answer rpc.FilterUpdateOutput
		calls  int
	}{
		{name: "empty"},
		{name: "inactive", refs: []boundaryRef{{name: "skip", inactive: true}}},
		{name: "automatic", refs: []boundaryRef{{stage: &automatic}}},
		{name: "automatic-external", refs: []boundaryRef{{stage: &automatic}, {name: "keep"}},
			answer: rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, calls: 1},
		{name: "accept", refs: two, answer: rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, calls: 2},
		{name: "modify", refs: two, answer: answerSet("med", "200"), calls: 2},
		{name: "inactive-external", refs: []boundaryRef{{name: "skip", inactive: true}, {name: "keep"}},
			answer: rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, calls: 1},
		{name: "raw", refs: two, raw: true, answer: rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: raw}, calls: 1},
		{name: "reject", refs: two, answer: rpc.FilterUpdateOutput{Action: rpc.FilterReject}, calls: 1},
		{name: "failure", refs: two, answer: rpc.FilterUpdateOutput{Action: rpc.FilterAction(255)}, calls: 1},
		{name: "teardown", refs: two, answer: rpc.FilterUpdateOutput{Action: rpc.FilterAccept, Teardown: true,
			NotifyCode: 6, NotifySubcode: 2}, calls: 1},
	}
	finals := map[routepolicy.Final]string{0: "absent", routepolicy.FinalAccept: "accept", routepolicy.FinalReject: "reject"}
	for _, ebgp := range []bool{false, true} {
		session := "ibgp"
		peerAS := uint32(65000)
		if ebgp {
			session = "ebgp"
			peerAS = 65001
		}
		for _, final := range []routepolicy.Final{0, routepolicy.FinalAccept, routepolicy.FinalReject} {
			for _, tc := range cases {
				t.Run(session+"/"+finals[final]+"/"+tc.name, func(t *testing.T) {
					transport := &answerTransport{reads: []string{"med"}, sets: []string{"med"},
						answer: func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput { return tc.answer }}
					if tc.raw {
						transport.reads, transport.sets = []string{rpc.FilterFieldRaw}, nil
					}
					r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
					addr := netip.MustParseAddr("192.0.2.1")
					peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: peerAS,
						ImportPolicy: boundaryPolicy(t, final, tc.refs...)})

					got := r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(received), received)

					if calls := transport.calls(); calls != tc.calls {
						t.Fatalf("filter calls = %d, want %d", calls, tc.calls)
					}
					accept := final == routepolicy.FinalAccept || final == 0 && !ebgp
					terminal := tc.name == "reject" || tc.name == "failure" || tc.name == "teardown"
					if got.accept != (accept && !terminal) {
						t.Fatalf("import = %+v, want accept %v", got, accept && !terminal)
					}
					teardown := tc.name == "teardown"
					if got.teardown != teardown {
						t.Fatalf("teardown = %v, want %v under every final", got.teardown, teardown)
					}
					if teardown && (got.notifyCode != 6 || got.notifySubcode != 2) {
						t.Fatalf("teardown NOTIFICATION = %d/%d, want 6/2", got.notifyCode, got.notifySubcode)
					}
					assertLibraryRunMatchesTrace(t, r, peer, transport, received, tc.calls, tc.name == "failure")
					assertDryRunMatchesStep(t, r, peer, transport, received, got, tc.calls, tc.name, automatic.Name)
					if !got.accept {
						return
					}
					switch tc.name {
					case "modify":
						if med := attrValues(t, got.modifiedPayload)[byte(attribute.AttrMED)]; !bytes.Equal(med, []byte{0, 0, 0, 200}) {
							t.Fatalf("MULTI_EXIT_DISC = %x, want the 200 the members set", med)
						}
					case "raw":
						if !bytes.Equal(got.modifiedPayload, raw) {
							t.Fatalf("body = %x, want the raw rewrite %x", got.modifiedPayload, raw)
						}
					default:
						if got.modifiedPayload != nil {
							t.Fatalf("body = %x, want the received body unchanged", got.modifiedPayload)
						}
					}
				})
			}
		}
	}
}

// assertLibraryRunMatchesTrace runs the peer's import library chain through
// runLibraryChain, the one entry point the import step and show policy test
// both reach, twice: live (record nil, Chain.Run) and display only
// (Chain.Trace). The two runs reach the same outcome, the same failed flag and
// the same rewritten body, and call the filter calls times each; the
// outcome's Failed is set exactly when failed (a member that could not
// decide). A policy with no library chain has no run to compare: the dry-run
// half (assertDryRunMatchesStep) covers it.
func assertLibraryRunMatchesTrace(t *testing.T, r *Reactor, peer *Peer, transport *answerTransport, received []byte, calls int, failed bool) {
	t.Helper()
	settings := peer.Settings()
	library := settings.ImportPolicy.Library()
	if library == nil {
		return
	}
	intent := libraryIntentContext{asn4: true, localAS: settings.LocalAS, peerAS: settings.PeerAS, asPathEditable: true}
	session := func() policyview.Session {
		return importSession(nil, settings.IsEBGP(), settings.PeerAS).WithImportMetadata(make(map[string]any, 2))
	}
	addr := settings.Address.String()

	before := transport.calls()
	live := r.runLibraryChain(library, testWireUpdate(received), received, session(), intent,
		directionImport, modifySiteImportChain, addr, nil)
	liveCalls := transport.calls() - before

	var steps int
	before = transport.calls()
	traced := r.runLibraryChain(library, testWireUpdate(received), received, session(), intent,
		directionImport, modifySiteImportChain, addr, func(policychain.MemberTrace) { steps++ })
	tracedCalls := transport.calls() - before

	if liveCalls != calls {
		t.Fatalf("live run called the filter %d times, want %d", liveCalls, calls)
	}
	if tracedCalls != calls {
		t.Fatalf("traced run called the filter %d times, want %d as the live run", tracedCalls, calls)
	}
	if steps != calls {
		t.Fatalf("trace recorded %d member steps, want %d", steps, calls)
	}
	if live.outcome != traced.outcome {
		t.Fatalf("trace outcome %+v differs from the live outcome %+v", traced.outcome, live.outcome)
	}
	if live.outcome.Failed != failed {
		t.Fatalf("outcome Failed = %v, want %v", live.outcome.Failed, failed)
	}
	if live.failed != traced.failed {
		t.Fatalf("trace failed = %v, live failed = %v", traced.failed, live.failed)
	}
	if !bytes.Equal(live.modifiedPayload, traced.modifiedPayload) {
		t.Fatalf("trace body %x differs from the live body %x", traced.modifiedPayload, live.modifiedPayload)
	}
}

// assertDryRunMatchesStep runs show policy test (reactorAPIAdapter.PolicyDryRun)
// on the peer's import direction over the received body and checks it
// predicts what the import step did: a reject exactly when the step dropped
// the route, with the filter called calls times, and one trace entry per
// call. A reference an in-process stage owns (stageName) is never run by the
// dry-run: when it is in the trace it is marked not-evaluated with no
// text-after, and on a policy whose only reference it is (the automatic case)
// it is the trace's one entry.
func assertDryRunMatchesStep(t *testing.T, r *Reactor, peer *Peer, transport *answerTransport, received []byte, got ingressStepResult, calls int, name, stageName string) {
	t.Helper()
	r.peers = map[netip.AddrPort]*Peer{peer.Settings().PeerKey(): peer}
	before := transport.calls()
	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(peer.Settings().Address.String(), directionImport, "", received, true)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if dryCalls := transport.calls() - before; dryCalls != calls {
		t.Fatalf("dry-run called the filter %d times, want %d as the import step", dryCalls, calls)
	}
	if accepted := result.Action != dryRunActionReject; accepted != got.accept {
		t.Fatalf("dry-run action %q, but the import step accept = %v", result.Action, got.accept)
	}
	entries := calls
	for _, entry := range result.Trace {
		if entry.Canonical != stageName {
			continue
		}
		if entry.Action != "not-evaluated" || entry.TextAfter != "" {
			t.Fatalf("the automatic check claimed execution: %+v", entry)
		}
		entries++
	}
	if len(result.Trace) != entries {
		t.Fatalf("dry-run trace %+v, want %d entries", result.Trace, entries)
	}
	if name == "automatic" && entries != 1 {
		t.Fatalf("dry-run trace %+v, want the automatic check as its one entry", result.Trace)
	}
}

// TestLibraryFinalDoesNotReplaceAFailure proves, on the export step, that a
// member that could not decide leaves the step failed under either final: the
// later member is never called, and neither final turns the failure into a
// policy decision (forward_failure_verdict_test.go reads the flag).
func TestLibraryFinalDoesNotReplaceAFailure(t *testing.T) {
	received := libraryTestBody(100)
	for _, final := range []routepolicy.Final{routepolicy.FinalAccept, routepolicy.FinalReject} {
		transport := &answerTransport{reads: []string{"med"}, answer: func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
			return rpc.FilterUpdateOutput{Action: rpc.FilterAction(255)}
		}}
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}

		got := r.runEgressPolicyChainASN4(answerPolicy(t, final, "first", "unreachable"), "10.0.0.2", 65002, 65000, false,
			policyview.Session{EBGP: true}, testWireUpdate(received), true)

		if got.accept || !got.failed {
			t.Fatalf("final %d: export = %+v, want a failed step", final, got)
		}
		if calls := transport.calls(); calls != 1 {
			t.Fatalf("final %d: filter calls = %d, want 1", final, calls)
		}
	}
}

// medRemoveAnswer is a modify verdict removing MULTI_EXIT_DISC.
func medRemoveAnswer() rpc.FilterUpdateOutput {
	return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
		{Field: "med", Action: rpc.FilterSetActionRemove, Value: json.RawMessage("null")}}}
}

// medBody is libraryTestBody's route with MULTI_EXIT_DISC metric in place of
// LOCAL_PREF, or with no MULTI_EXIT_DISC when metric is nil.
func medBody(metric []byte) []byte {
	attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), []byte{2, 1, 0, 0, 0xfd, 0xe9})...)
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 1})...)
	if metric != nil {
		attrs = append(attrs, makeAttr(0x80, byte(attribute.AttrMED), metric)...)
	}
	return buildModTestPayload(attrs, modTestNLRI)
}

// runMEDImport runs the import step over received with members first and
// second answering their verdicts, and answers the body the step left.
func runMEDImport(t *testing.T, received []byte, first, second rpc.FilterUpdateOutput) []byte {
	t.Helper()
	transport := &answerTransport{reads: []string{"med"}, sets: []string{"med"}, answer: func(input rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
		if input.Filter == "first" {
			return first
		}
		return second
	}}
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
	addr := netip.MustParseAddr("10.0.0.4")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
		ImportPolicy: answerPolicy(t, routepolicy.FinalAccept, "first", "second")})
	got := r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(received), received)
	if !got.accept {
		t.Fatalf("import = %+v, want accept", got)
	}
	if got.modifiedPayload == nil {
		return received
	}
	return got.modifiedPayload
}

// TestLibraryMEDRemovalFollowsTheChainOrder is the reactor form of the text
// chain's MED-removal order: RFC 4271 Section 5.1.4 lets the operator remove
// a received MULTI_EXIT_DISC, and when two import members disagree the later
// one decides what reaches the RIB, including on a route that arrived with no
// metric and was given one by the first member.
func TestLibraryMEDRemovalFollowsTheChainOrder(t *testing.T) {
	withMED := medBody([]byte{0, 0, 0, 100})

	t.Run("a later set wins", func(t *testing.T) {
		got := runMEDImport(t, withMED, medRemoveAnswer(), answerSet("med", "200"))
		if med := attrValues(t, got)[byte(attribute.AttrMED)]; !bytes.Equal(med, []byte{0, 0, 0, 200}) {
			t.Fatalf("MULTI_EXIT_DISC = %x, want the 200 the second member set", med)
		}
	})

	t.Run("a later removal wins", func(t *testing.T) {
		got := runMEDImport(t, withMED, answerSet("med", "200"), medRemoveAnswer())
		if med, ok := attrValues(t, got)[byte(attribute.AttrMED)]; ok {
			t.Fatalf("MULTI_EXIT_DISC %x reached the RIB, want the second member's removal", med)
		}
	})

	t.Run("a later removal wins on a metric-less route", func(t *testing.T) {
		got := runMEDImport(t, medBody(nil), answerSet("med", "200"), medRemoveAnswer())
		if med, ok := attrValues(t, got)[byte(attribute.AttrMED)]; ok {
			t.Fatalf("MULTI_EXIT_DISC %x reached the RIB, want the metric the first member added removed", med)
		}
	})
}

// TestLibraryMEDRemovalLeavesNoZeroLengthAttribute proves a removed
// MULTI_EXIT_DISC leaves the rebuilt body whole: RFC 4271 Section 4.3 gives
// MULTI_EXIT_DISC a four-octet value, so emitting it with no value would be a
// malformed attribute, never a removal.
func TestLibraryMEDRemovalLeavesNoZeroLengthAttribute(t *testing.T) {
	accept := rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
	got := runMEDImport(t, medBody([]byte{0, 0, 0, 50}), medRemoveAnswer(), accept)
	attrs := attrValues(t, got)
	if med, ok := attrs[byte(attribute.AttrMED)]; ok {
		t.Fatalf("MULTI_EXIT_DISC is still on the wire with value %x", med)
	}
	for code, value := range attrs {
		if len(value) == 0 {
			t.Fatalf("attribute %d is on the wire with a zero-length value", code)
		}
	}
	if _, ok := attrs[byte(attribute.AttrOrigin)]; !ok {
		t.Fatal("the untouched ORIGIN did not survive the rebuild")
	}
}

// TestLibraryPrependEncodesAtTheDestinationASNWidth proves the library
// prepend intent on the export step writes the local AS at the destination's
// width: toward a two-octet peer AS_TRANS leads the AS_PATH and the real local
// AS leads the AS4_PATH beside it, and toward a four-octet peer the AS_PATH
// carries it with no AS4_PATH (RFC 6793 Section 4.2.2: "The NEW BGP speaker
// MUST also send the AS path information in the AS4_PATH attribute (encoded
// with four-octet AS numbers)").
func TestLibraryPrependEncodesAtTheDestinationASNWidth(t *testing.T) {
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	libraryIntentEval = func(_ *policyview.View, s *policychain.Sets) error { return s.PrependASPath(1) }
	cases := []struct {
		name    string
		asn4    bool
		asPath  []byte // AS_SEQUENCE [64496] at the destination width
		wantAS  []byte
		wantAS4 []byte // nil: no AS4_PATH
	}{
		{"two-octet destination", false, []byte{2, 1, 0xfb, 0xf0},
			[]byte{2, 2, 0x5b, 0xa0, 0xfb, 0xf0},        // AS_TRANS 23456, 64496
			[]byte{2, 2, 0, 2, 0, 0, 0, 0, 0xfb, 0xf0}}, // 131072, 64496
		{"four-octet destination", true, []byte{2, 1, 0, 0, 0xfb, 0xf0},
			[]byte{2, 2, 0, 2, 0, 0, 0, 0, 0xfb, 0xf0}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			received := buildModTestPayload(slicesConcat(
				makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0}),
				makeAttr(0x40, byte(attribute.AttrASPath), tc.asPath)), modTestNLRI)
			policy := libraryPolicy(t, libraryIntentExport, libraryTypeIntents, routepolicy.FinalAccept)

			out := r.runEgressPolicyChainASN4(policy, "10.0.0.5", 65002, 131072, false,
				policyview.Session{EBGP: true}, testWireUpdate(received), tc.asn4)

			if !out.accept || out.wireOverride == nil {
				t.Fatalf("export = %+v, want accept with the prepend", out)
			}
			attrs := attrValues(t, out.wireOverride.Payload())
			if got := attrs[byte(attribute.AttrASPath)]; !bytes.Equal(got, tc.wantAS) {
				t.Fatalf("AS_PATH = %x, want %x", got, tc.wantAS)
			}
			as4, ok := attrs[byte(attribute.AttrAS4Path)]
			if tc.wantAS4 == nil {
				if ok {
					t.Fatalf("AS4_PATH %x accompanies the four-octet AS_PATH", as4)
				}
				return
			}
			if !bytes.Equal(as4, tc.wantAS4) {
				t.Fatalf("AS4_PATH = %x, want %x", as4, tc.wantAS4)
			}
		})
	}
}

// attrValues answers each path attribute of an UPDATE body by its type code,
// as its value octets without the header (rebuiltAttrs keeps the header), so
// a zero-length value reads as an empty slice.
func attrValues(t *testing.T, payload []byte) map[byte][]byte {
	t.Helper()
	values := rebuiltAttrs(t, payload)
	for code, attr := range values {
		header := 3
		if attr[0]&0x10 != 0 { // RFC 4271 Section 4.3 Extended Length
			header = 4
		}
		values[code] = attr[header:]
	}
	return values
}

// slicesConcat joins attribute encodings into one attribute section.
func slicesConcat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
