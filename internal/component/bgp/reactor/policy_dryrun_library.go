// Design: docs/architecture/core-design.md -- the in-process BGP chain, display-only trace
// Related: policy_dryrun.go -- PolicyDryRun, which runs a library chain here
// Related: filter_library.go -- runLibraryChain, the run this shares with the live path

package reactor

import (
	"errors"
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
// failure's error, and in text-after the UPDATE as the chain rewrote it up to
// that member (libraryTraceText).
func (r *Reactor) traceLibraryChain(library filterapi.LibraryChain, dry libraryDryRun, textBefore string) (PolicyChainResult, []plugin.PolicyTraceEntry) {
	var trace []plugin.PolicyTraceEntry
	record := func(step policychain.MemberTrace) {
		entry := libraryTraceEntry(step)
		text, err := r.libraryTraceText(step, dry, textBefore)
		entry.TextAfter = text
		if err != nil && entry.Error == "" {
			entry.Error = err.Error()
		}
		trace = append(trace, entry)
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

// libraryTraceText renders the UPDATE as the chain rewrote it up to step,
// for the step's text-after, the way a text filter's entry carries the text
// after its delta: after a passing or failed-open member the folded sets so
// far (a failed member's none), rebuilt through rewriteLibrary, the builder
// the chain's result takes; after a member that rejected no text, as the text
// path's rejecting entry. Display only: it allocates the rendered text. An
// error says why the sets so far cannot be rendered, and the text is empty.
func (r *Reactor) libraryTraceText(step policychain.MemberTrace, dry libraryDryRun, textBefore string) (string, error) {
	switch step.Step {
	case policychain.TraceReject, policychain.TraceFailClosed:
		return "", nil
	case policychain.TracePass, policychain.TraceFailOpen:
	case policychain.TraceUnspecified:
		panic("BUG: reactor: a library trace step is unspecified")
	}
	var mods filterapi.ModAccumulator
	folded, err := step.Apply(&mods)
	if err != nil {
		return "", errors.Join(errTraceRender, err)
	}
	if folded.Empty() {
		return textBefore, nil
	}
	rewrite := r.rewriteLibrary(&mods, folded, dry.wireUpdate, dry.payload, dry.intent)
	if rewrite.nlriRefused {
		return "", errors.New("the NLRI changes up to this filter cannot be applied: " + rewrite.refusal.reason)
	}
	if rewrite.modFail.failed() {
		return "", errors.New("the changes up to this filter cannot be applied: " + rewrite.modFail.String())
	}
	if rewrite.payload == nil {
		return textBefore, nil
	}
	modified := wireu.NewWireUpdate(rewrite.payload, dry.ctxID)
	attrs, err := modified.Attrs()
	if err != nil {
		return "", errors.Join(errTraceRender, err)
	}
	return string(AppendUpdateForFilter(nil, attrs, modified, nil)), nil
}

// errTraceRender is joined to why a member's rewritten UPDATE cannot be shown.
var errTraceRender = errors.New("the UPDATE as rewritten up to this filter cannot be shown")

// libraryMember answers the library chain of the one member ref names, for a
// single-filter override (Chain.Only); false when the chain holds none.
func libraryMember(library filterapi.LibraryChain, ref string) (filterapi.LibraryChain, bool) {
	chain, ok := library.(*policychain.Chain)
	if !ok {
		panic("BUG: reactor: a policy's library chain is not a *policychain.Chain")
	}
	only, ok := chain.Only(ref)
	if !ok {
		return nil, false
	}
	return only, true
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
