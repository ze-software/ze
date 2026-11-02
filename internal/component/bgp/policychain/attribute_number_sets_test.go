package policychain

import (
	"bytes"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// testAttachmentNumber carries and lets a filter set an attribute number the
// probe plugin owns, as bgp import and export do for otc.
const testAttachmentNumber = "test attribute number"

// testNumberCode is an unassigned attribute type code, so no attribute the
// engine knows is shadowed.
const testNumberCode = attribute.AttributeCode(254)

var fieldTestNumber = policy.MustRegisterField("test-number", policy.RouteView{})

func init() {
	policyview.MustRegisterAttributeNumber(fieldTestNumber, testNumberCode, "probe")
	fields := []policy.Field{fieldTestNumber}
	policy.MustRegisterAttachment("bgp", testAttachmentNumber, fields, fields)
}

// TestOwnerAttributeNumberSets proves an owner-registered attribute number
// (otc's shape) is settable by its owner's filter only: the owner's set
// becomes one AttrModSet on the attribute's code and a later member reads
// the set value; another plugin's set, and an action other than set, fail
// the route. Method: external members over one route, then Apply.
func TestOwnerAttributeNumberSets(t *testing.T) {
	decl := ExternalDeclaration{Reads: []string{"test-number"}, Sets: []string{"test-number"}, OnError: rpc.OnErrorReject}
	caller := &fakeCaller{filters: map[string]fakeFilter{
		"probe:stamp": {decl: decl, answer: modify(set("test-number", rpc.FilterSetActionSet, "65001"))},
		"probe:see": {
			decl: ExternalDeclaration{Reads: []string{"test-number"}, OnError: rpc.OnErrorReject},
			answer: func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
				return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
			},
		},
		"probe:add":   {decl: decl, answer: modify(set("test-number", rpc.FilterSetActionAdd, "1"))},
		"other:stamp": {decl: decl, answer: modify(set("test-number", rpc.FilterSetActionSet, "65001"))},
	}}

	chain := compileMembers(t, testAttachmentNumber, []Member{{Name: "stamp", Plugin: "probe"}, {Name: "see", Plugin: "probe"}})
	outcome, ops := runApplied(t, chain, caller, "import", localPref(100))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("owner set: outcome %+v, want accept", outcome)
	}
	want := []byte{0, 0, 0xfd, 0xe9}
	found := slices.ContainsFunc(ops, func(op filterapi.AttrOp) bool {
		return op.Code == uint8(testNumberCode) && op.Action == filterapi.AttrModSet && bytes.Equal(op.Buf, want)
	})
	if !found {
		t.Errorf("ops = %+v, want one set of code %d to %x", ops, testNumberCode, want)
	}
	if got := caller.sent["probe:see"]; got != `{"test-number":65001}` {
		t.Errorf("later member was sent %s, want the value the owner set", got)
	}

	for _, member := range []Member{{Name: "stamp", Plugin: "other"}, {Name: "add", Plugin: "probe"}} {
		outcome, ops = runApplied(t, compileMembers(t, testAttachmentNumber, []Member{member}), caller, "import", localPref(100))
		if outcome.Action != OutcomeReject || !outcome.Failed || ops != nil {
			t.Errorf("%s:%s: outcome %+v, ops %+v; want a failed reject", member.Plugin, member.Name, outcome, ops)
		}
	}
}
