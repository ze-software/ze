package filter_path_asn

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// judgeUpdate answers in the way a BGP chain's match instance naming the list
// in.Filter judges the route: the reject-asn condition (conditionCheck) over a
// view of a wire UPDATE whose AS_PATH carries segments, on a live run
// (MatchRoute.Reports), so a refusal is counted. in.Update is not read. On import the session names
// in.PeerAS as the sender; on any other direction the sender is unknown.
// A list the installed config does not hold fails the test: a match instance
// naming an unknown list is refused at config load and never runs.
func judgeUpdate(t *testing.T, in *sdk.FilterUpdateInput, segments []attribute.ASPathSegment) *sdk.FilterUpdateOutput {
	t.Helper()
	held := installedRejectASN.Load()
	if held == nil {
		t.Fatalf("no reject-asn config installed for %q", in.Filter)
	}
	list, ok := held.lists[in.Filter]
	if !ok {
		t.Fatalf("reject-asn list %q is not installed", in.Filter)
	}
	session := policyview.Session{EBGP: true}
	if in.Direction == directionImport {
		session.Sender, session.SenderKnown = in.PeerAS, true
	}
	var view policyview.View
	view.Reset(segmentsUpdate(t, segments...), session)
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
