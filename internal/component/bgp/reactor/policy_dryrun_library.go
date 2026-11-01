// Design: docs/architecture/core-design.md -- the in-process BGP chain, display-only trace
// Related: policy_dryrun.go -- PolicyDryRun, which runs a library chain here
// Related: filter_library.go -- runLibraryChain, the run this shares with the live path

package reactor

import (
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// libraryDryRun is the route and session one library dry-run judges.
type libraryDryRun struct {
	wireUpdate *wireu.WireUpdate
	payload    []byte
	ctxID      bgpctx.ContextID
	direction  string
	peer       string
	internal   bool
	intent     libraryIntentContext
}

// traceLibraryChain runs a library chain for show policy test (D-11): the
// same runLibraryChain the live import and export steps run, in its
// display-only mode, so the decision, the final and the rewritten UPDATE are
// the live path's, while no failure is counted or logged. Each member's trace
// entry is drawn from its step: a passing member's sets as text in delta, a
// failure's error. A member reads the view, not a text subject, so its entry
// carries no text-after; the resulting text is the result's.
func (r *Reactor) traceLibraryChain(library filterapi.LibraryChain, dry libraryDryRun, textBefore string) (PolicyChainResult, []plugin.PolicyTraceEntry) {
	var trace []plugin.PolicyTraceEntry
	record := func(step policychain.MemberTrace) {
		trace = append(trace, libraryTraceEntry(step))
	}
	res := r.runLibraryChain(library, dry.wireUpdate, dry.payload, policyview.Session{EBGP: !dry.internal}, dry.intent,
		dry.direction, "", dry.peer, record)
	outcome := res.outcome
	if outcome.Teardown {
		return PolicyChainResult{Action: PolicyReject, Teardown: true,
			NotifyCode: outcome.NotifyCode, NotifySubcode: outcome.NotifySubcode}, trace
	}
	if outcome.Action != policychain.OutcomeAccept {
		return PolicyChainResult{Action: PolicyReject, Failed: outcome.Failed}, trace
	}
	if res.failed {
		return PolicyChainResult{Action: PolicyReject, Failed: true}, trace
	}
	if res.modifiedPayload == nil {
		return PolicyChainResult{Action: PolicyAccept, Text: textBefore}, trace
	}
	modified := wireu.NewWireUpdate(res.modifiedPayload, dry.ctxID)
	attrs, err := modified.Attrs()
	if err != nil {
		return PolicyChainResult{Action: PolicyReject, Failed: true}, trace
	}
	return PolicyChainResult{Action: PolicyAccept, Text: string(AppendUpdateForFilter(nil, attrs, modified, nil))}, trace
}

// libraryTraceEntry renders one member's step as a dry-run trace entry.
func libraryTraceEntry(step policychain.MemberTrace) plugin.PolicyTraceEntry {
	entry := plugin.PolicyTraceEntry{Filter: step.Name, Canonical: step.Name}
	if _, filter, ok := strings.Cut(step.Name, ":"); ok {
		entry.Filter = filter
	}
	switch step.Step {
	case policychain.TracePass:
		entry.Action = dryRunActionAccept
		if step.Sets != nil && !step.Sets.Empty() {
			entry.Action = dryRunActionModify
			entry.Delta = string(step.Sets.AppendText(nil))
		}
	case policychain.TraceReject, policychain.TraceFailClosed:
		entry.Action = dryRunActionReject
	case policychain.TraceFailOpen:
		entry.Action = dryRunActionAccept
	case policychain.TraceUnspecified:
		panic("BUG: reactor: a library trace step is unspecified")
	}
	if step.Err != nil {
		entry.Error = step.Err.Error()
	}
	return entry
}
