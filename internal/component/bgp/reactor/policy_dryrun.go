// Design: docs/architecture/core-design.md -- policy filter chain
// Spec: plan/spec-pol-4-explain.md -- policy dry-run trace
// Related: policy_dryrun_library.go -- traceLibraryChain (a library chain's dry-run)
// Related: policy_dryrun_display.go -- routeDisplay (text-before, text-after, changed-attrs)
// RFC: rfc/short/rfc4271.md -- Section 5.1.4, the direction the med-remove directive is honored on

package reactor

import (
	"errors"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// errUncompiledChain is why a plugin filter on a policy that compiled no
// library chain cannot run in the dry-run: the session fails the route closed
// for the same reason.
var errUncompiledChain = errors.New("the peer's filter chain is not compiled, so this filter cannot run and the route is rejected (fail-closed); repeat show policy test once the peer session is established")

// traceUncompiledChain answers the dry-run of a policy that compiled no
// library chain. Its plugin filters have no chain to reach them, so the first
// active one fails closed: a reject with its trace entry naming why, as the
// session rejects the route. A named in-process check is marked
// not-evaluated, never called. A policy holding neither ends on its final,
// RFC 8212 Section 3 deciding an absent one. On a direction other than import
// a policy that requires the ingress stage is refused before any filter, its
// first in-process check named as not evaluated. The caller shows the
// resulting UPDATE: the pasted one on accept, none on reject.
func traceUncompiledChain(policy filterapi.Policy, external bool, direction string) (PolicyChainResult, []plugin.PolicyTraceEntry) {
	var trace []plugin.PolicyTraceEntry
	refused := direction != directionImport && policy.RequiresIngress()
	for i, ref := range policy.Filters() {
		if ref.Inactive {
			continue
		}
		_, filterName, _ := strings.Cut(ref.Name, ":")
		entry := plugin.PolicyTraceEntry{Filter: filterName, Canonical: ref.Name}
		if policy.InProcess(i) {
			entry.Action = "not-evaluated"
			trace = append(trace, entry)
			if refused {
				break
			}
			continue
		}
		if refused {
			continue
		}
		entry.Action = dryRunActionReject
		entry.Error = errUncompiledChain.Error()
		trace = append(trace, entry)
		return PolicyChainResult{Action: PolicyReject, Failed: true}, trace
	}
	if refused {
		return PolicyChainResult{Action: PolicyReject, Failed: true}, trace
	}
	// RFC 8212 Section 3.
	if !policy.AcceptsAtEnd(external) {
		return PolicyChainResult{Action: PolicyReject}, trace
	}
	return PolicyChainResult{Action: PolicyAccept}, trace
}

const (
	dryRunActionAccept = "accept"
	dryRunActionReject = "reject"
	dryRunActionModify = "modify"
	// dryRunActionWithdraw is a chain member's withdraw verdict: the route
	// is sent as a withdrawal.
	dryRunActionWithdraw = "withdraw"
)

var (
	errPeerNotFound      = errors.New("peer not found")
	errFilterNotFound    = errors.New("filter not found in peer chain")
	errInvalidUpdateBody = errors.New("invalid UPDATE message body")
	errInvalidDirection  = errors.New("direction must be import or export")
	errInProcessDryRun   = errors.New("filter executes in an in-process protocol stage, outside policy dry-run; test it with an UPDATE on the peer session")
)

// PolicyDryRun implements plugin.PolicyDryRunner on reactorAPIAdapter.
func (a *reactorAPIAdapter) PolicyDryRun(peerAddr, direction, filterOverride string, updateBody []byte, asn4 bool) (*plugin.PolicyDryRunResult, error) {
	// Defense-in-depth: the CLI parser already enforces import/export, but
	// PolicyDryRun is an exported interface. Reject an unknown direction so a
	// direct caller cannot silently get an empty (accept) chain.
	if direction != directionImport && direction != directionExport {
		return nil, errInvalidDirection
	}

	addr, err := netip.ParseAddr(peerAddr)
	if err != nil {
		return nil, errPeerNotFound
	}

	// Read policy and session type from the same dynamic-resolution generation.
	r := a.r
	var policy filterapi.Policy
	var peerAS, localAS uint32
	var facts filterapi.PeerFilterInfo
	internal := false
	found := false
	r.mu.RLock()
	for _, p := range r.peers {
		p.mu.RLock()
		if p.settings.Address != addr {
			p.mu.RUnlock()
			continue
		}
		found = true
		peerAS = p.settings.PeerAS
		localAS = p.settings.LocalAS
		internal = p.settings.IsIBGP()
		// The peer's session facts, as the live import and export steps
		// answer them: a session fact field a chain member reads (peer-name,
		// peer-as, rs-client, ...) then answers in the dry-run what it
		// answers on the session.
		facts = p.settings.SessionFacts(peerAS)
		if direction == directionImport && p.session != nil {
			// RFC 9494 Section 4.3: the received direction reads whether
			// this speaker advertised LLGR on the session, as the live
			// import does (reactor_notify.go).
			facts.LLGRAdvertised = p.session.localLLGR.Load()
		}
		switch direction {
		case directionImport:
			policy = p.settings.ImportPolicy
		case directionExport:
			policy = p.settings.ExportPolicy
		}
		p.mu.RUnlock()
		break
	}
	r.mu.RUnlock()

	if !found {
		return nil, errPeerNotFound
	}
	// The direction's effective policy, taken before any filter override: an
	// override replaces the refs and keeps the disposition, and the report is
	// the policy the live session runs, not the one-filter test chain.
	effective := policyInfo(policy, policySessionOf(peerAS, internal))
	if peerAS == 0 {
		return &plugin.PolicyDryRunResult{
			Direction: direction,
			Peer:      peerAddr,
			Policy:    effective,
			Action:    "undetermined",
			Reason:    "remote AS is unresolved; wait for the peer session to establish, then repeat show policy test",
		}, nil
	}

	// If a single filter override is specified, find it in the chain.
	if filterOverride != "" {
		ref := resolveFilterOverride(filterOverride, policy.Filters())
		if ref == "" {
			return nil, errFilterNotFound
		}
		for i, configured := range policy.Filters() {
			if configured.Name == ref {
				if policy.InProcess(i) {
					return nil, errInProcessDryRun
				}
			}
		}
		policy = policy.WithFilters([]routepolicy.Ref{{Name: ref}})
		if library := policy.Library(); library != nil {
			// The library chain runs the one member, display only, as the
			// whole chain runs below: no plugin serves a library member.
			only, ok := libraryMember(library, ref)
			if !ok {
				return nil, errFilterNotFound
			}
			policy = policy.WithLibrary(only)
		}
	}

	// Validate the UPDATE body parses correctly before proceeding.
	if _, err := message.UnpackUpdate(updateBody); err != nil {
		return nil, errInvalidUpdateBody
	}
	if !asn4 {
		// A route received over a two-octet session reaches the chains only
		// after the ingest collapse (Session's CollapseAS4Family call in
		// session_read.go) merged AS4_PATH and AS4_AGGREGATOR into the
		// four-octet form, so the dry-run explains that route, not the bytes
		// pasted (RFC 6793 Section 4.2.3).
		collapsed := make([]byte, wireu.CollapseAS4FamilySize(updateBody))
		n, _, collapseErr := wireu.CollapseAS4Family(collapsed, updateBody, false)
		if collapseErr != nil {
			return nil, errInvalidUpdateBody
		}
		if n > 0 {
			updateBody = collapsed[:n:n]
			asn4 = true
		}
	}

	// Build a temporary WireUpdate the chain judges and the display reads.
	ctxID := bgpctx.APIContextID
	if !asn4 {
		// Register the ASN2 encoding context. The only failure is registry
		// exhaustion (65535 distinct contexts); surface it rather than silently
		// falling back to ctxID 0, which would render with the wrong ASN4 setting.
		id, regErr := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(false))
		if regErr != nil {
			return nil, regErr
		}
		ctxID = id
	}
	wu := wireu.NewWireUpdate(updateBody, ctxID)

	// D-11: the pasted UPDATE as the operator is shown it, read from the wire.
	before, err := displayUpdate(wu)
	if err != nil {
		return nil, errInvalidUpdateBody
	}
	textBefore := before.text()

	var result PolicyChainResult
	var after *routeDisplay
	var trace []plugin.PolicyTraceEntry
	var wireChanges []string
	if library := policy.Library(); library != nil {
		// D-11: a library chain runs as it runs live, display only, its
		// plugin members reached through the chain (traceLibraryChain).
		dry := libraryDryRun{wireUpdate: wu, payload: updateBody, ctxID: ctxID, direction: direction, peer: peerAddr, session: dryRunSession(direction, facts),
			intent: libraryIntentContext{asn4: asn4, localAS: localAS, peerAS: peerAS, asPathEditable: direction == directionImport || !internal}}
		result, after, trace, wireChanges = r.traceLibraryChain(library, &dry, &before)
	} else {
		result, trace = traceUncompiledChain(policy, !internal, direction)
		if result.Action == PolicyAccept {
			result.Text = textBefore
			after = &before
		}
	}
	// A policy rejection removes announcements, not explicit withdrawals.
	// Execution failures and teardown still drop the entire UPDATE.
	if result.Action == PolicyReject && !policy.AcceptsAtEnd(!internal) && !result.Failed && !result.Teardown {
		if withdrawals := policyWithdrawals(updateBody); withdrawals != nil {
			withdrawn, displayErr := displayUpdate(wireu.NewWireUpdate(withdrawals, ctxID))
			if displayErr != nil {
				return nil, errInvalidUpdateBody
			}
			result = PolicyChainResult{Action: PolicyAccept, Text: withdrawn.text()}
			after = &withdrawn
		}
	}
	textAfter := result.Text

	var actionStr string
	switch result.Action {
	case PolicyReject:
		actionStr = dryRunActionReject
	case PolicyModify:
		// A raw rewrite changes the wire payload without a text delta.
		actionStr = dryRunActionModify
	case PolicyAccept:
		if result.Withdrawn {
			actionStr = dryRunActionWithdraw
			break
		}
		if textAfter == textBefore {
			actionStr = dryRunActionAccept
		} else {
			actionStr = dryRunActionModify
		}
	default:
		panic("BUG: invalid policy action")
	}

	// The changed attributes compare the two displays, each read from a wire
	// UPDATE; the wire changes are the operations the library chain's run
	// built (traceLibraryChain).
	var changedAttrs []string
	if after != nil && textBefore != textAfter && textAfter != "" {
		changedAttrs = before.changed(after)
	}

	return &plugin.PolicyDryRunResult{
		Direction:    direction,
		Peer:         peerAddr,
		Action:       actionStr,
		Policy:       effective,
		Trace:        trace,
		TextBefore:   textBefore,
		TextAfter:    textAfter,
		ChangedAttrs: changedAttrs,
		WireChanges:  wireChanges,
	}, nil
}

// dryRunSession answers the session show policy test runs a chain under,
// built as the live step of direction builds it, so a session fact field or a
// metadata field answers in the dry-run what it answers on the session.
//
// On import the session is the peer's (filter_ordered.go importSession) with a
// fresh writable metadata map (WithImportMetadata): a member's metadata set
// succeeds as it does live, and the map is discarded with the dry-run. On
// export the peer is the destination (reactor_api_forward.go ExportSession). A
// pasted UPDATE came from no peer, so the source facts are unknown and the
// source fields answer absent, and it carries no received metadata.
func dryRunSession(direction string, peer filterapi.PeerFilterInfo) policyview.Session {
	if direction == directionImport {
		return policyview.ImportSession(peer).WithImportMetadata(make(map[string]any, 2))
	}
	return policyview.ExportSession(peer, filterapi.PeerFilterInfo{}, false)
}

// describeWireOps renders each attribute operation as "<ATTRIBUTE> <verb>"
// for the dry-run's wire-changes; nil when there is none. Display only.
func describeWireOps(mods *filterapi.ModAccumulator) []string {
	ops := mods.Ops()
	if len(ops) == 0 {
		return nil
	}

	changes := make([]string, 0, len(ops))
	for _, op := range ops {
		changes = append(changes, attribute.AttributeCode(op.Code).String()+" "+wireModVerb(op.Action))
	}
	return changes
}

// wireModVerb maps a ModAccumulator action to a human-readable verb for
// dry-run output.
func wireModVerb(action uint8) string {
	switch action {
	case filterapi.AttrModSet:
		return "set"
	case filterapi.AttrModAdd:
		return "add"
	case filterapi.AttrModRemove:
		return "remove"
	case filterapi.AttrModPrepend:
		return "prepend"
	case filterapi.AttrModSuppress:
		return "suppressed"
	default:
		return "modified"
	}
}

// resolveFilterOverride finds the canonical ref for a filter override name.
// Accepts plain names, type-prefixed, and plugin-prefixed forms.
//
// Inactive refs (prefixed "inactive:") are skipped: no chain runs them, so matching one would silently yield an "accept" result
// with an empty trace, misleading the operator into thinking the filter is
// permissive. Returning "" makes the handler report "filter not found"
// instead.
func resolveFilterOverride(name string, chain []routepolicy.Ref) string {
	for _, ref := range chain {
		if ref.Inactive {
			continue
		}
		if ref.Name == name {
			return ref.Name
		}
		// Match by filter instance name (after first colon, e.g. "plugin:FILTER" -> "FILTER").
		if _, after, ok := strings.Cut(ref.Name, ":"); ok {
			if after == name {
				return ref.Name
			}
		}
	}
	return ""
}
