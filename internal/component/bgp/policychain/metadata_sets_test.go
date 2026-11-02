package policychain

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The attachments this test registers: one carrying the rebuilt metadata
// field and letting a filter set it, as bgp import does, and one that only
// carries it, as bgp export does.
const (
	testAttachmentMetaImport = "test meta import"
	testAttachmentMetaExport = "test meta export"
)

func init() {
	fields := []policy.Field{policyview.FieldRebuilt}
	policy.MustRegisterAttachment("bgp", testAttachmentMetaImport, fields, fields)
	policy.MustRegisterAttachment("bgp", testAttachmentMetaExport, fields, nil)
}

// TestInsertedMemberMetadataSets proves AC-5's chain half: a plugin filter's
// metadata set lands in the route's metadata map, a later member is sent it,
// a value of the wrong kind fails the route, and the attachment that does not
// let a filter set the field refuses it. Method: two external members over
// an import session's map, then the same set at an export-like attachment.
func TestInsertedMemberMetadataSets(t *testing.T) {
	caller := &fakeCaller{filters: map[string]fakeFilter{
		"probe:mark": {
			decl:   ExternalDeclaration{Reads: []string{"rebuilt"}, Sets: []string{"rebuilt"}, OnError: rpc.OnErrorReject},
			answer: modify(set("rebuilt", rpc.FilterSetActionSet, "true")),
		},
		"probe:see": {
			decl: ExternalDeclaration{Reads: []string{"rebuilt"}, OnError: rpc.OnErrorReject},
			answer: func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
				return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
			},
		},
		"probe:bad": {
			decl:   ExternalDeclaration{Reads: []string{"rebuilt"}, Sets: []string{"rebuilt"}, OnError: rpc.OnErrorReject},
			answer: modify(set("rebuilt", rpc.FilterSetActionSet, `"yes"`)),
		},
	}}
	members := []Member{{Name: "mark", Plugin: "probe"}, {Name: "see", Plugin: "probe"}}

	meta := map[string]any{}
	outcome := runMetaRoute(t, compileMembers(t, testAttachmentMetaImport, members), caller,
		policyview.Session{}.WithImportMetadata(meta))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("import outcome = %+v, want accept", outcome)
	}
	if meta["rebuilt"] != true {
		t.Errorf("metadata map = %v, want rebuilt true", meta)
	}
	if got := caller.sent["probe:see"]; got != `{"rebuilt":true}` {
		t.Errorf("later member was sent %s, want the rebuilt the first one set", got)
	}

	meta = map[string]any{}
	outcome = runMetaRoute(t, compileMembers(t, testAttachmentMetaImport, []Member{{Name: "bad", Plugin: "probe"}}), caller,
		policyview.Session{}.WithImportMetadata(meta))
	if outcome.Action != OutcomeReject || !outcome.Failed || len(meta) != 0 {
		t.Errorf("wrong-kind set: outcome %+v, map %v; want a failed reject and no key", outcome, meta)
	}

	shared := map[string]any{}
	outcome = runMetaRoute(t, compileMembers(t, testAttachmentMetaExport, members), caller,
		policyview.Session{}.WithExportMetadata(shared))
	if outcome.Action != OutcomeReject || !outcome.Failed || len(shared) != 0 {
		t.Errorf("export set: outcome %+v, map %v; want a failed reject and the shared map untouched", outcome, shared)
	}
}

func runMetaRoute(t *testing.T, chain *Chain, caller ExternalCaller, session policyview.Session) Outcome {
	t.Helper()
	var view policyview.View
	sets := new(RunSets)
	sets.SetRoute(ExternalRoute{Caller: caller, Direction: "import", Peer: "192.0.2.1", PeerAS: 65001})
	view.Reset(update(t, localPref(100)), session)
	defer view.Clear()
	return chain.Run(&view, sets)
}
