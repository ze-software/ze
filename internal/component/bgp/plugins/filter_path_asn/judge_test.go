package filter_path_asn

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// judgeUpdate answers in the way a BGP chain's match instance naming the list
// in.Filter judges the route: the reject-asn condition (conditionCheck) over a
// view of a wire UPDATE whose AS_PATH carries the ASNs in.Update's as-path
// field lists, on a live run (MatchRoute.Reports), so a refusal is counted.
// The text is only how these tests spell their fixed inputs: the AS path text
// the view answers is the same flattened decimal string, so the condition
// judges the very subject the test wrote. On import the session names
// in.PeerAS as the sender; on any other direction the sender is unknown.
// A list the installed config does not hold fails the test: a match instance
// naming an unknown list is refused at config load and never runs.
func judgeUpdate(t *testing.T, in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	t.Helper()
	held := installedRejectASN.Load()
	if held == nil {
		t.Fatalf("no reject-asn config installed for %q", in.Filter)
	}
	list, ok := held.lists[in.Filter]
	if !ok {
		t.Fatalf("reject-asn list %q is not installed", in.Filter)
	}
	var path []uint32
	for token := range strings.FieldsSeq(filtertext.ASPath(in.Update)) {
		number, err := strconv.ParseUint(token, 10, 32)
		if err != nil {
			t.Fatalf("as-path token %q: %v", token, err)
		}
		path = append(path, uint32(number))
	}
	session := policyview.Session{EBGP: true}
	if in.Direction == directionImport {
		session.Sender, session.SenderKnown = in.PeerAS, true
	}
	var view policyview.View
	view.Reset(pathUpdate(t, path...), session)
	defer view.Clear()
	passed, err := conditionCheck(list)(&policychain.MatchRoute{View: &view, Reports: true})
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	if passed {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
}
