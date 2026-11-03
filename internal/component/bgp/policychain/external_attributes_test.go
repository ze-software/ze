package policychain

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// testAttachmentAttributes lets a filter read and set every attribute field a
// modify filter changes.
const testAttachmentAttributes = "policychain-test attributes"

var attributeFields = []string{"origin", "next-hop", "aigp", "large-community", "extended-community", "med"}

func init() {
	fields := []policy.Field{policy.FieldOrigin, policy.FieldNextHop, policy.FieldAIGP,
		policy.FieldLargeCommunity, policy.FieldExtendedCommunity, policy.FieldMED}
	policy.MustRegisterAttachment("bgp", testAttachmentAttributes, fields, fields)
}

func aigpAttr(metric uint64) []byte {
	var value [attribute.AIGPWireLen]byte
	attribute.WriteAIGPMetric(value[:], 0, metric)
	return pathAttr(0x80, attribute.AttrAIGP, value[:])
}

// originIGP is an ORIGIN of IGP.
func originIGP() []byte {
	return pathAttr(0x40, attribute.AttrOrigin, []byte{byte(attribute.OriginIGP)})
}

// runApplied runs chain over attrs from direction and answers the outcome and
// the operations Apply writes for the folded sets.
func runApplied(t *testing.T, chain *Chain, caller ExternalCaller, direction string, attrs ...[]byte) (Outcome, []filterapi.AttrOp) {
	t.Helper()
	var view policyview.View
	sets := new(RunSets)
	sets.SetRoute(ExternalRoute{Caller: caller, Direction: direction, Peer: "192.0.2.1", PeerAS: 65001})
	defer sets.SetRoute(ExternalRoute{})
	view.Reset(update(t, attrs...), policyview.Session{})
	defer view.Clear()
	outcome := chain.Run(&view, sets)
	if outcome.Action != OutcomeAccept {
		return outcome, nil
	}
	var acc filterapi.ModAccumulator
	if err := chain.Apply(&view, sets, &acc); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return outcome, acc.Ops()
}

// TestExternalAttributeSetsWriteTheirOps proves an external filter's origin,
// next-hop, aigp, large-community and extended-community sets each become
// one ModAccumulator operation: a replacement of ORIGIN, NEXT_HOP and AIGP, and one
// add or remove per community value. The AIGP increment resolves against the
// route's metric. Method: one member answers every set; the folded ops are
// compared with the expected list, value bytes included.
func TestExternalAttributeSetsWriteTheirOps(t *testing.T) {
	caller := &fakeCaller{filters: map[string]fakeFilter{"ext:all": {
		decl: ExternalDeclaration{Reads: []string{"origin"}, Sets: attributeFields, OnError: rpc.OnErrorReject},
		answer: modify(
			set("origin", rpc.FilterSetActionSet, `"egp"`),
			set("next-hop", rpc.FilterSetActionSet, `"192.0.2.9"`),
			set("aigp", rpc.FilterSetActionIncrement, "5"),
			set("large-community", rpc.FilterSetActionAdd, `"65000:1:2"`),
			set("large-community", rpc.FilterSetActionRemove, `"65000:9:9"`),
			set("extended-community", rpc.FilterSetActionAdd, `"target:65000:100"`),
			set("extended-community", rpc.FilterSetActionRemove, `"target:65000:7"`),
		),
	}}}
	chain := compileMembers(t, testAttachmentAttributes, []Member{{Name: "all", Plugin: "ext"}})
	outcome, got := runApplied(t, chain, caller, "import", originIGP(), aigpAttr(10))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept", outcome)
	}

	var aigp [attribute.AIGPWireLen]byte
	attribute.WriteAIGPMetric(aigp[:], 0, 15)
	large := func(a, b, c uint32) []byte {
		return binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, a), b), c)
	}
	target := func(text string) []byte {
		ec, err := attribute.ParseSingleExtCommunity(text)
		if err != nil {
			t.Fatalf("extended community: %v", err)
		}
		return ec[:]
	}
	want := []filterapi.AttrOp{
		{Code: uint8(attribute.AttrOrigin), Action: filterapi.AttrModSet, Buf: []byte{byte(attribute.OriginEGP)}},
		{Code: uint8(attribute.AttrNextHop), Action: filterapi.AttrModSet, Buf: []byte{192, 0, 2, 9}},
		{Code: uint8(attribute.AttrAIGP), Action: filterapi.AttrModSet, Buf: aigp[:]},
		{Code: uint8(attribute.AttrLargeCommunity), Action: filterapi.AttrModAdd, Buf: large(65000, 1, 2)},
		{Code: uint8(attribute.AttrLargeCommunity), Action: filterapi.AttrModRemove, Buf: large(65000, 9, 9)},
		{Code: uint8(attribute.AttrExtCommunity), Action: filterapi.AttrModAdd, Buf: target("target:65000:100")},
		{Code: uint8(attribute.AttrExtCommunity), Action: filterapi.AttrModRemove, Buf: target("target:65000:7")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ops = %+v\nwant  %+v", got, want)
	}
}

// TestExternalAIGPIncrementOfAbsentMetricLeavesTheRoute proves an AIGP
// increment of a route that carries no AIGP writes nothing, as
// filter_modify's arithmetic did: AIGP has no configured base.
func TestExternalAIGPIncrementOfAbsentMetricLeavesTheRoute(t *testing.T) {
	caller := &fakeCaller{filters: map[string]fakeFilter{"ext:aigp": {
		decl:   ExternalDeclaration{Reads: []string{"aigp"}, Sets: []string{"aigp"}, OnError: rpc.OnErrorReject},
		answer: modify(set("aigp", rpc.FilterSetActionIncrement, "5")),
	}}}
	chain := compileMembers(t, testAttachmentAttributes, []Member{{Name: "aigp", Plugin: "ext"}})
	outcome, got := runApplied(t, chain, caller, "import", originIGP())
	if outcome.Action != OutcomeAccept || len(got) != 0 {
		t.Fatalf("outcome = %+v ops = %+v, want accept with no op", outcome, got)
	}
}

// TestExternalAIGPDecrementOfAbsentMetricLeavesTheRoute proves an AIGP
// decrement of a route that carries no AIGP writes nothing either, the
// decrement half filter_modify's TestAigpArithmeticOnAbsentAttributeCreatesNothing
// carried. RFC 7311 Section 3.4.1: "A BGP speaker MUST NOT add the AIGP
// attribute to any route whose path leads outside the AIGP administrative
// domain to which the BGP speaker belongs." A decrement clamped at zero would
// otherwise add an AIGP of 0, which Section 4.1 scores as the best metric.
// A route that does carry one is decremented as usual.
func TestExternalAIGPDecrementOfAbsentMetricLeavesTheRoute(t *testing.T) {
	caller := &fakeCaller{filters: map[string]fakeFilter{"ext:aigp": {
		decl:   ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"aigp"}, OnError: rpc.OnErrorReject},
		answer: modify(set("aigp", rpc.FilterSetActionDecrement, "30")),
	}}}
	chain := compileMembers(t, testAttachmentAttributes, []Member{{Name: "aigp", Plugin: "ext"}})
	outcome, got := runApplied(t, chain, caller, "import", originIGP())
	if outcome.Action != OutcomeAccept || len(got) != 0 {
		t.Fatalf("absent: outcome = %+v ops = %+v, want accept with no op", outcome, got)
	}

	var value [attribute.AIGPWireLen]byte
	attribute.WriteAIGPMetric(value[:], 0, 12)
	outcome, got = runApplied(t, chain, caller, "import", originIGP(), aigpAttr(42))
	want := []filterapi.AttrOp{{Code: uint8(attribute.AttrAIGP), Action: filterapi.AttrModSet, Buf: value[:]}}
	if outcome.Action != OutcomeAccept || !reflect.DeepEqual(got, want) {
		t.Fatalf("carried: outcome = %+v ops = %+v, want accept with %+v", outcome, got, want)
	}
}

// TestExternalMEDRemove proves RFC 4271 Section 5.1.4's configured removal
// through a filter's answer: on import a route carrying MULTI_EXIT_DISC gets
// one suppression of attribute 4, a route without one gets no op, and on
// export the answer fails the route, because the removal MUST come before
// the decision process.
func TestExternalMEDRemove(t *testing.T) {
	remove := rpc.FilterSet{Field: "med", Action: rpc.FilterSetActionRemove, Value: []byte("null")}
	caller := &fakeCaller{filters: map[string]fakeFilter{"ext:drop": {
		decl:   ExternalDeclaration{Reads: []string{"med"}, Sets: []string{"med"}, OnError: rpc.OnErrorReject},
		answer: modify(remove),
	}}}
	chain := compileMembers(t, testAttachmentAttributes, []Member{{Name: "drop", Plugin: "ext"}})

	outcome, got := runApplied(t, chain, caller, "import", med(50))
	want := []filterapi.AttrOp{{Code: uint8(attribute.AttrMED), Action: filterapi.AttrModSuppress}}
	if outcome.Action != OutcomeAccept || !reflect.DeepEqual(got, want) {
		t.Fatalf("import with med: outcome = %+v ops = %+v, want accept with %+v", outcome, got, want)
	}
	outcome, got = runApplied(t, chain, caller, "import", originIGP())
	if outcome.Action != OutcomeAccept || len(got) != 0 {
		t.Fatalf("import without med: outcome = %+v ops = %+v, want accept with no op", outcome, got)
	}
	outcome, _ = runApplied(t, chain, caller, "export", med(50))
	if outcome.Action != OutcomeReject || !outcome.Failed {
		t.Fatalf("export: outcome = %+v, want a failed route", outcome)
	}
}

// TestExternalAttributeSetsAreReadAfterSet proves a later member is sent the
// origin and the absent MED an earlier member's sets left, not the received
// ones (read-after-set). The large-community overlay is exercised by the
// later member's read only where the community formatter is linked, so this
// package test checks it through the folded ops instead.
func TestExternalAttributeSetsAreReadAfterSet(t *testing.T) {
	caller := &fakeCaller{filters: map[string]fakeFilter{
		"ext:first": {
			decl: ExternalDeclaration{Reads: []string{"origin"}, Sets: attributeFields, OnError: rpc.OnErrorReject},
			answer: modify(set("origin", rpc.FilterSetActionSet, `"incomplete"`),
				set("large-community", rpc.FilterSetActionAdd, `"65000:1:2"`),
				rpc.FilterSet{Field: "med", Action: rpc.FilterSetActionRemove, Value: []byte("null")}),
		},
		"ext:second": {
			decl: ExternalDeclaration{Reads: []string{"origin", "med"}, OnError: rpc.OnErrorReject},
			answer: func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
				return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
			},
		},
	}}
	chain := compileMembers(t, testAttachmentAttributes,
		[]Member{{Name: "first", Plugin: "ext"}, {Name: "second", Plugin: "ext"}})
	outcome, _ := runApplied(t, chain, caller, "import", originIGP(), med(50))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept", outcome)
	}
	if got, want := caller.sent["ext:second"], `{"origin":"incomplete"}`; got != want {
		t.Fatalf("second member was sent %s, want %s", got, want)
	}
}
