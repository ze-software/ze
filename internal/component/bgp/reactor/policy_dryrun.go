// Design: docs/architecture/core-design.md -- policy filter chain
// Spec: plan/spec-pol-4-explain.md -- policy dry-run trace
// Related: filter_chain.go -- PolicyFilterChain (runtime execution)
// Related: filter_format.go -- AppendUpdateForFilter (text rendering)
// Related: filter_delta.go -- textDeltaToModOps (wire diff extraction)
// RFC: rfc/short/rfc4271.md -- Section 5.1.4, the direction the med-remove directive is honored on

package reactor

import (
	"errors"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// tracePolicyFilterChain records each filter decision and applies the effective
// policy final at exhaustion or a terminal raw rewrite, just like PolicyFilterChain.
func tracePolicyFilterChain(policy filterapi.Policy, direction, peer string, peerAS uint32, updateText string, callFilter PolicyFilterFunc) (PolicyChainResult, []plugin.PolicyTraceEntry) {
	var trace []plugin.PolicyTraceEntry
	current := updateText

	for _, ref := range policy.Filters() {
		if ref.Inactive {
			continue
		}
		pluginName, filterName, _ := strings.Cut(ref.Name, ":")
		result := callFilter(pluginName, filterName, direction, peer, peerAS, current)

		entry := plugin.PolicyTraceEntry{
			Filter:    filterName,
			Canonical: ref.Name,
		}

		if result.Teardown {
			entry.Action = dryRunActionReject
			trace = append(trace, entry)
			return PolicyChainResult{
				Action:        PolicyReject,
				Failed:        result.Failed,
				Teardown:      true,
				NotifyCode:    result.NotifyCode,
				NotifySubcode: result.NotifySubcode,
			}, trace
		}

		switch result.Action {
		case PolicyReject:
			entry.Action = dryRunActionReject
			entry.TextAfter = ""
			trace = append(trace, entry)
			return PolicyChainResult{Action: PolicyReject, Failed: result.Failed}, trace
		case PolicyModify:
			entry.Action = dryRunActionModify
			if len(result.Raw) > 0 {
				entry.TextAfter = current
				trace = append(trace, entry)
				if !policy.AcceptsAtEnd() {
					return PolicyChainResult{Action: PolicyReject}, trace
				}
				return PolicyChainResult{Action: PolicyModify, Text: current, Raw: result.Raw}, trace
			}
			entry.Delta = result.Delta
			current = applyFilterDelta(current, result.Delta)
			entry.TextAfter = current
		case PolicyAccept:
			entry.Action = dryRunActionAccept
			entry.TextAfter = current
		default:
			panic("BUG: invalid policy action")
		}

		trace = append(trace, entry)
	}

	if !policy.AcceptsAtEnd() {
		return PolicyChainResult{Action: PolicyReject}, trace
	}
	return PolicyChainResult{Action: PolicyAccept, Text: current}, trace
}

const (
	dryRunActionAccept = "accept"
	dryRunActionReject = "reject"
	dryRunActionModify = "modify"
)

var (
	errPeerNotFound      = errors.New("peer not found")
	errFilterNotFound    = errors.New("filter not found in peer chain")
	errInvalidUpdateBody = errors.New("invalid UPDATE message body")
	errInvalidDirection  = errors.New("direction must be import or export")
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

	// Policies are immutable snapshots read through the peer's guarded
	// accessors, because dynamic establishment and reload can replace them.
	r := a.r
	var policy filterapi.Policy
	var peerAS, localAS uint32
	internal := false
	found := false
	r.mu.RLock()
	for _, p := range r.peers {
		s := p.Settings()
		if s.Address != addr {
			continue
		}
		found = true
		// PeerAS and filter chains via the guarded accessors: this policy dry-run runs on
		// an API goroutine that can race a dynamic peer's establishment write (caller holds
		// r.mu.RLock).
		peerAS = p.PeerAS()
		localAS = s.LocalAS
		internal = p.IsIBGP()
		switch direction {
		case directionImport:
			policy = p.ImportPolicy()
		case directionExport:
			policy = p.ExportPolicy()
		}
		break
	}
	r.mu.RUnlock()

	if !found {
		return nil, errPeerNotFound
	}

	// If a single filter override is specified, find it in the chain.
	if filterOverride != "" {
		ref := resolveFilterOverride(filterOverride, policy.Filters())
		if ref == "" {
			return nil, errFilterNotFound
		}
		policy = policy.WithFilters([]filterapi.FilterRef{{Name: ref}})
	}

	// Validate the UPDATE body parses correctly before proceeding.
	if _, err := message.UnpackUpdate(updateBody); err != nil {
		return nil, errInvalidUpdateBody
	}

	// Build a temporary WireUpdate for text rendering.
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

	// Parse attributes from the update body.
	attrs, err := wu.Attrs()
	if err != nil {
		return nil, errInvalidUpdateBody
	}

	var buf []byte
	buf = AppendUpdateForFilter(buf, attrs, wu, nil)
	textBefore := string(buf)

	// Run the trace chain using the real filter function (calls plugins via IPC).
	callFilter := r.policyFilterFunc(updateBody)
	result, trace := tracePolicyFilterChain(
		policy, direction, peerAddr, peerAS, textBefore, callFilter,
	)
	// A policy rejection removes announcements, not explicit withdrawals.
	// Execution failures and teardown still drop the entire UPDATE.
	if result.Action == PolicyReject && !policy.AcceptsAtEnd() && !result.Failed && !result.Teardown {
		if withdrawals := policyWithdrawals(updateBody); withdrawals != nil {
			withdrawalUpdate := wireu.NewWireUpdate(withdrawals, ctxID)
			withdrawalAttrs, attrErr := withdrawalUpdate.Attrs()
			if attrErr != nil {
				return nil, errInvalidUpdateBody
			}
			buf = AppendUpdateForFilter(buf[:0], withdrawalAttrs, withdrawalUpdate, nil)
			result = PolicyChainResult{Action: PolicyAccept, Text: string(buf)}
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
		if textAfter == textBefore {
			actionStr = dryRunActionAccept
		} else {
			actionStr = dryRunActionModify
		}
	default:
		panic("BUG: invalid policy action")
	}

	// Compute changed attributes. Parse each filter text exactly once and
	// share the maps read-only (spec filter-delta-parse-once).
	var changedAttrs []string
	var wireChanges []string
	if textBefore != textAfter && textAfter != "" {
		beforeAttrs := parseFilterAttrs(textBefore)
		afterAttrs := parseFilterAttrs(textAfter)
		changedAttrs = computeChangedAttrs(beforeAttrs, afterAttrs)
		wireChanges = computeWireChanges(beforeAttrs, afterAttrs, attrs, direction, asn4, peerAS, localAS, internal)
	}

	return &plugin.PolicyDryRunResult{
		Direction:    direction,
		Peer:         peerAddr,
		Action:       actionStr,
		Trace:        trace,
		TextBefore:   textBefore,
		TextAfter:    textAfter,
		ChangedAttrs: changedAttrs,
		WireChanges:  wireChanges,
	}, nil
}

// computeWireChanges mirrors the runtime egress/ingress text-to-wire path
// (reactor_api_forward.go and reactor_notify.go) to surface wire-level
// attribute modifications that the flat filter text cannot express. The
// principal case is remove-private-as (RFC 6996) suppressing or rewriting
// AS4_PATH (RFC 6793), which never appears in the text format because
// AppendUpdateForFilter renders only the merged "as-path".
//
// It builds the same ModAccumulator the forward path builds but does not
// construct a modified payload: this is a read-only explanation, so it only
// reports the operations as "<ATTRIBUTE> <verb>" strings. beforeAttrs and
// afterAttrs are the caller's single parseFilterAttrs results, shared
// read-only with computeChangedAttrs.
//
// direction is here because one of those operations is direction-dependent:
// RFC 4271 Section 5.1.4's med-remove directive is converted on the import
// chain alone (ExtractMEDRemoveOps, filter_delta.go). Reporting it on an export
// dry-run would promise a removal the runtime does not perform, and omitting it
// on an import one tells the operator the metric survives. internal is the
// session's own verdict (Peer.IsIBGP, migration-aware) for the same reason:
// the export chain applies no AS_PATH edit toward an internal peer.
func computeWireChanges(beforeAttrs, afterAttrs *filterAttrs, attrs *attribute.AttributesWire, direction string, asn4 bool, peerAS, localAS uint32, internal bool) []string {
	var mods filterapi.ModAccumulator

	values := acquireValueScratch()
	defer releaseValueScratch(values)

	textDeltaToModOps(values, beforeAttrs, afterAttrs, &mods)
	// RFC 4271 Section 5.1.2: "When a given BGP speaker advertises the route
	// to an internal peer, the advertising speaker SHALL NOT modify the
	// AS_PATH attribute associated with the route." The export chain
	// (runEgressPolicyChainASN4) therefore drops remove-private and
	// as-path-prepend toward an internal peer, and the dry-run reports what
	// the runtime does. The import chain applies both on every session.
	if direction == directionImport || !internal {
		ExtractRemovePrivateASOps(values, afterAttrs, attrs, asn4, peerAS, &mods)
		ExtractASPathPrependOps(values, afterAttrs, attrs, asn4, localAS, &mods)
	}
	if direction == directionImport && medRemoveHasWork(afterAttrs) {
		ExtractMEDRemoveOps(afterAttrs, &mods)
	}

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
// Inactive refs (prefixed "inactive:") are skipped: TracePolicyFilterChain
// never runs them, so matching one would silently yield an "accept" result
// with an empty trace, misleading the operator into thinking the filter is
// permissive. Returning "" makes the handler report "filter not found"
// instead.
func resolveFilterOverride(name string, chain []filterapi.FilterRef) string {
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

// computeChangedAttrs compares the parsed before and after filter attribute
// maps and returns the list of attribute names that differ. Both maps are
// the caller's single parseFilterAttrs results, shared read-only with
// computeWireChanges.
func computeChangedAttrs(beforeAttrs, afterAttrs *filterAttrs) []string {
	var changed []string
	for _, id := range formatFilterAttrsOrder {
		bv, bOK := beforeAttrs.get(id)
		av, aOK := afterAttrs.get(id)
		if bOK != aOK || bv != av {
			changed = append(changed, filterAttrNames[id])
		}
	}
	return changed
}
