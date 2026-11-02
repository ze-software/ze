package policychain

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The attachments this test registers: one letting a filter set the
// preserve-opaque-attributes treatment, as bgp export does, and one that does
// not, as bgp import does.
const (
	testAttachmentTreatmentExport = "test treatment export"
	testAttachmentTreatmentImport = "test treatment import"
)

func init() {
	reads := []policy.Field{policyview.FieldRSClient}
	treatment := []policy.Field{policyview.FieldPreserveOpaqueAttributes}
	policy.MustRegisterAttachment("bgp", testAttachmentTreatmentExport, reads, treatment)
	policy.MustRegisterAttachment("bgp", testAttachmentTreatmentImport, reads, nil)
}

// TestInsertedMemberTreatmentSets proves AC-16's chain half: a plugin
// filter's set of preserve-opaque-attributes (bgp-rs's transparency step)
// reaches the run's folded treatments, and survives a later member that sets
// nothing; a value other than true, an undeclared set, and the attachment
// that does not let a filter set it each fail the route with no treatment.
// Method: external members through the fake caller, read back through
// RunSets.Folded().Treatments().
func TestInsertedMemberTreatmentSets(t *testing.T) {
	field := policyview.FieldPreserveOpaqueAttributes.String()
	// Each probe reads rs-client, as bgp-rs's step does, so no text update
	// is owed.
	reads := []string{policyview.FieldRSClient.String()}
	accept := func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
		return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
	}
	caller := &fakeCaller{filters: map[string]fakeFilter{
		"probe:transparency": {
			decl:   ExternalDeclaration{Reads: reads, Sets: []string{field}, OnError: rpc.OnErrorReject},
			answer: modify(set(field, rpc.FilterSetActionSet, "true")),
		},
		"probe:later": {decl: ExternalDeclaration{Reads: reads, OnError: rpc.OnErrorReject}, answer: accept},
		"probe:false": {
			decl:   ExternalDeclaration{Reads: reads, Sets: []string{field}, OnError: rpc.OnErrorReject},
			answer: modify(set(field, rpc.FilterSetActionSet, "false")),
		},
		"probe:undeclared": {
			decl:   ExternalDeclaration{Reads: reads, OnError: rpc.OnErrorReject},
			answer: modify(set(field, rpc.FilterSetActionSet, "true")),
		},
	}}
	bit := policyview.TreatmentPreserveOpaqueAttributes.Bit()

	chain := compileMembers(t, testAttachmentTreatmentExport,
		[]Member{{Name: "transparency", Plugin: "probe"}, {Name: "later", Plugin: "probe"}})
	outcome, treatments := runTreatmentRoute(t, chain, caller)
	if outcome.Action != OutcomeAccept {
		t.Fatalf("export outcome = %+v, want accept", outcome)
	}
	if treatments != bit {
		t.Errorf("treatments = %08b, want %08b (preserve-opaque-attributes)", treatments, bit)
	}

	chain = compileMembers(t, testAttachmentTreatmentExport, []Member{{Name: "later", Plugin: "probe"}})
	if _, treatments = runTreatmentRoute(t, chain, caller); treatments != 0 {
		t.Errorf("a chain no member sets a treatment in answered %08b, want none", treatments)
	}

	for _, refused := range []struct {
		name       string
		attachment string
	}{
		{"false", testAttachmentTreatmentExport},
		{"undeclared", testAttachmentTreatmentExport},
		{"transparency", testAttachmentTreatmentImport},
	} {
		chain = compileMembers(t, refused.attachment, []Member{{Name: refused.name, Plugin: "probe"}})
		outcome, treatments = runTreatmentRoute(t, chain, caller)
		if outcome.Action != OutcomeReject || !outcome.Failed || treatments != 0 {
			t.Errorf("%s at %s: outcome %+v, treatments %08b; want a failed reject and none",
				refused.name, refused.attachment, outcome, treatments)
		}
	}
}

func runTreatmentRoute(t *testing.T, chain *Chain, caller ExternalCaller) (Outcome, uint8) {
	t.Helper()
	var view policyview.View
	sets := new(RunSets)
	sets.SetRoute(ExternalRoute{Caller: caller, Direction: "export", Peer: "192.0.2.1", PeerAS: 65001})
	view.Reset(update(t, localPref(100)), policyview.Session{})
	defer view.Clear()
	outcome := chain.Run(&view, sets)
	if outcome.Action != OutcomeAccept {
		return outcome, 0
	}
	return outcome, sets.Folded().Treatments()
}
