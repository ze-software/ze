// Design: docs/architecture/core-design.md — policy filter chain
// Related: reactor_notify.go — ingress filter invocation point
// Related: reactor_api_forward.go — egress filter invocation point

package reactor

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/filterapi"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

const policyAttrNLRI = "nlri"
const policyAttrAtomicAggregate = "atomic-aggregate"

// Policy filter chain direction tokens (the `direction` argument passed to
// PolicyFilterChain and on to each filter via the RPC FilterUpdateInput).
const (
	directionImport = "import"
	directionExport = "export"
)

type filterAttrID uint8

const (
	faOrigin filterAttrID = iota
	faASPath
	faNextHop
	faMED
	faLocalPreference
	faAtomicAggregate
	faAggregator
	faCommunity
	faOriginatorID
	faClusterList
	faExtendedCommunity
	faAIGP
	faLargeCommunity
	faCommunityAdd
	faCommunityRemove
	faLargeCommunityAdd
	faLargeCommunityRemove
	faExtendedCommunityAdd
	faExtendedCommunityRemove
	faASPathPrepend
	faRemovePrivate
	faNLRI
	faCount
)

var filterAttrNames = [faCount]string{
	faOrigin: "origin", faASPath: "as-path", faNextHop: "next-hop",
	faMED: "med", faLocalPreference: "local-preference",
	faAtomicAggregate: policyAttrAtomicAggregate, faAggregator: "aggregator",
	faCommunity: "community", faOriginatorID: "originator-id",
	faClusterList: "cluster-list", faExtendedCommunity: "extended-community",
	faAIGP: "aigp", faLargeCommunity: "large-community",
	faCommunityAdd: "community-add", faCommunityRemove: "community-remove",
	faLargeCommunityAdd: "large-community-add", faLargeCommunityRemove: "large-community-remove",
	faExtendedCommunityAdd: "extended-community-add", faExtendedCommunityRemove: "extended-community-remove",
	faASPathPrepend: "as-path-prepend", faRemovePrivate: policyAttrRemovePrivate,
	faNLRI: "nlri",
}

var filterAttrNameToID map[string]filterAttrID

func init() {
	filterAttrNameToID = make(map[string]filterAttrID, faCount)
	for id := filterAttrID(0); id < faCount; id++ { //nolint:modernize // prealloc linter crashes on range-over-int
		filterAttrNameToID[filterAttrNames[id]] = id
	}
}

type filterAttrs struct {
	values      [faCount]string
	present     uint32
	unknownName string // first unrecognized token at key position (for validateModifyDelta)
}

func (a *filterAttrs) set(id filterAttrID, val string) {
	a.values[id] = val
	a.present |= 1 << id
}

func (a *filterAttrs) get(id filterAttrID) (string, bool) {
	if a.present&(1<<id) == 0 {
		return "", false
	}
	return a.values[id], true
}

func (a *filterAttrs) has(id filterAttrID) bool {
	return a.present&(1<<id) != 0
}

func (a *filterAttrs) merge(delta *filterAttrs) {
	for id := filterAttrID(0); id < faCount; id++ { //nolint:modernize // prealloc linter crashes on range-over-int
		if delta.has(id) {
			a.set(id, delta.values[id])
		}
	}
}

// PolicyAction is the result of a policy filter evaluation.
type PolicyAction int

const (
	// PolicyAccept passes the update through unchanged.
	PolicyAccept PolicyAction = iota
	// PolicyReject drops the update (short-circuits the chain).
	PolicyReject
	// PolicyModify passes the update with delta-only attribute changes.
	PolicyModify
)

// PolicyResponse holds the outcome of a single filter invocation.
type PolicyResponse struct {
	Action PolicyAction
	// Delta contains only changed attribute text (action=modify).
	// Empty for accept/reject.
	Delta string
	// Raw, when non-empty on a modify, is a raw full UPDATE-body replacement
	// produced by a raw=true filter (e.g. MP_REACH/MP_UNREACH surgery the text
	// delta cannot express). A raw rewrite is terminal for the chain: it cannot
	// be composed with downstream text deltas.
	Raw []byte
	// Teardown requests the session be terminated (NOTIFICATION + close) after
	// the import chain. Honored only for import (received) UPDATEs. The route is
	// dropped. NotifyCode/NotifySubcode default to Cease / Connection Rejected.
	Teardown      bool
	NotifyCode    uint8
	NotifySubcode uint8
	// Failed marks a PolicyReject that the filter did NOT decide: an IPC error
	// under a fail-closed on-error policy, or a response we could not parse.
	// The route is still rejected -- that is the fail-closed contract -- but a
	// caller counting outcomes must not record it as a policy decision. Without
	// this, a plugin timing out under load is indistinguishable from a plugin
	// deliberately rejecting every route.
	Failed bool
}

// PolicyChainResult is the aggregate outcome of running a filter chain.
type PolicyChainResult struct {
	Action PolicyAction
	// Failed propagates PolicyResponse.Failed for the reject that
	// short-circuited the chain. See that field.
	Failed bool
	// Text is the accumulated modified update text (valid unless Action==PolicyReject).
	Text string
	// Raw is a raw full-payload replacement from a raw filter; terminal.
	// Empty when no raw filter rewrote the payload.
	Raw []byte
	// Teardown (import only) requests NOTIFICATION + session close; route dropped.
	Teardown      bool
	NotifyCode    uint8
	NotifySubcode uint8
}

// PolicyFilterFunc is the signature for calling a named filter.
// The caller provides direction, peer info, and the text-format update.
// Returns the filter's decision.
type PolicyFilterFunc func(pluginName, filterName, direction, peer string, peerAS uint32, updateText string) PolicyResponse

// PolicyFilterChain runs a chain of named filters on an update.
// Filters are piped: each sees the previous filter's output.
// Reject short-circuits. Default is accept at end of chain.
//
// filterRefs is the ordered list of "<plugin>:<filter>" strings.
// callFilter is the function to invoke each filter.
// updateText is the initial text representation of the update.
//
// Returns the aggregate chain result: final action, accumulated update text, an
// optional raw full-payload override, and an optional teardown request.
//
// A raw=true filter that returns a full-payload rewrite (PolicyResponse.Raw) is
// terminal: the rewrite replaces the wire payload and the chain stops, because a
// raw rewrite cannot be composed with downstream text deltas (the text was
// derived from the original payload). A teardown request also short-circuits and
// drops the route.
func PolicyFilterChain(filterRefs []filterapi.FilterRef, direction, peer string, peerAS uint32, updateText string, callFilter PolicyFilterFunc) PolicyChainResult {
	if len(filterRefs) == 0 {
		return PolicyChainResult{Action: PolicyAccept, Text: updateText}
	}

	current := updateText
	for _, ref := range filterRefs {
		if ref.Inactive {
			continue
		}
		pluginName, filterName, _ := strings.Cut(ref.Name, ":")
		result := callFilter(pluginName, filterName, direction, peer, peerAS, current)

		// Teardown short-circuits: the session is going away, so drop the route.
		if result.Teardown {
			return PolicyChainResult{
				Action:        PolicyReject,
				Teardown:      true,
				Failed:        result.Failed,
				NotifyCode:    result.NotifyCode,
				NotifySubcode: result.NotifySubcode,
			}
		}

		switch result.Action {
		case PolicyReject:
			return PolicyChainResult{Action: PolicyReject, Failed: result.Failed}
		case PolicyModify:
			// A raw full-payload rewrite is terminal (see doc comment).
			if len(result.Raw) > 0 {
				return PolicyChainResult{Action: PolicyModify, Text: current, Raw: result.Raw}
			}
			current = applyFilterDelta(current, result.Delta)
		case PolicyAccept:
			// continue with current text
		}
	}

	return PolicyChainResult{Action: PolicyAccept, Text: current}
}

// applyFilterDelta merges delta-only attribute changes into the current update text.
// The delta contains only changed fields. Fields not in delta remain unchanged.
//
// Both current and delta use the same text format:
//
//	"<attr-name> <value> [<attr-name> <value> ...] [nlri <family> <op> <prefix>...]"
//
// Delta fields overwrite corresponding fields in current.
func applyFilterDelta(current, delta string) string {
	if delta == "" {
		return current
	}

	currentAttrs := parseFilterAttrs(current)
	deltaAttrs := parseFilterAttrs(delta)

	currentAttrs.merge(deltaAttrs)

	return formatFilterAttrs(currentAttrs)
}

var policySingleToken = map[string]bool{
	"origin": true, "next-hop": true, "med": true,
	"local-preference": true, policyAttrAtomicAggregate: true,
	"aggregator": true, "originator-id": true,
	"as-path-prepend": true, policyAttrRemovePrivate: true, "aigp": true,
}

// parseFilterAttrsCalls counts parseFilterAttrs invocations. Test seam for
// TestFilterDeltaParseCallCount, which proves the filter modify paths parse
// each filter text exactly once (spec filter-delta-parse-once AC-2/AC-3).
var parseFilterAttrsCalls atomic.Uint64

// parseFilterAttrs parses text-format attributes into a fixed struct.
// Each attribute is "name value" where value may contain spaces.
// Special key "nlri" captures the NLRI section.
func parseFilterAttrs(text string) *filterAttrs {
	parseFilterAttrsCalls.Add(1)
	attrs := &filterAttrs{}
	if text == "" {
		return attrs
	}

	fields := strings.Fields(text)
	i := 0
	for i < len(fields) {
		name := fields[i]
		i++

		id, known := filterAttrNameToID[name]
		if !known {
			if attrs.unknownName == "" {
				attrs.unknownName = name
			}
			continue
		}

		if name == policyAttrNLRI {
			start := i - 1
			for i < len(fields) && !isPolicyAttrName(fields[i]) {
				i++
			}
			attrs.set(id, textbuf.Join(fields[start:i], " "))
			continue
		}

		if name == policyAttrAtomicAggregate {
			attrs.set(id, "")
			continue
		}

		if policySingleToken[name] {
			if i < len(fields) {
				attrs.set(id, fields[i])
				i++
			}
			continue
		}

		var values []string
		for i < len(fields) && !isPolicyAttrName(fields[i]) {
			values = append(values, fields[i])
			i++
		}
		attrs.set(id, textbuf.Join(values, " "))
	}

	return attrs
}

// isPolicyAttrName returns true if the token is a known BGP attribute name.
func isPolicyAttrName(s string) bool {
	switch s {
	case "origin", "as-path", "next-hop", "med", "local-preference",
		policyAttrAtomicAggregate, "aggregator", "community", "originator-id",
		"cluster-list", "extended-community", "aigp", "large-community", "nlri",
		"as-path-prepend", policyAttrRemovePrivate,
		"community-add", "community-remove",
		"large-community-add", "large-community-remove",
		"extended-community-add", "extended-community-remove":
		return true
	}
	return false
}

// formatFilterAttrsOrder defines the output order for formatFilterAttrs.
// Matches the historical map-based order for deterministic results.
var formatFilterAttrsOrder = [...]filterAttrID{
	faOrigin, faASPath, faNextHop, faMED, faLocalPreference,
	faAtomicAggregate, faAggregator, faCommunity, faOriginatorID,
	faClusterList, faExtendedCommunity, faAIGP, faLargeCommunity,
	faCommunityAdd, faCommunityRemove,
	faLargeCommunityAdd, faLargeCommunityRemove,
	faExtendedCommunityAdd, faExtendedCommunityRemove,
	faASPathPrepend, faRemovePrivate, faNLRI,
}

// formatFilterAttrs converts a filterAttrs struct back to text format.
// Attributes are output in a fixed order for deterministic results.
func formatFilterAttrs(attrs *filterAttrs) string {
	var buf []byte
	for _, id := range formatFilterAttrsOrder {
		val, ok := attrs.get(id)
		if !ok {
			continue
		}
		if len(buf) > 0 {
			buf = append(buf, ' ')
		}
		name := filterAttrNames[id]
		switch id {
		case faNLRI:
			buf = append(buf, val...)
		case faAtomicAggregate:
			buf = append(buf, name...)
		default:
			buf = append(buf, name...)
			buf = append(buf, ' ')
			buf = append(buf, val...)
		}
	}
	return string(buf)
}

// policyFilterTimeout is the per-filter IPC timeout (spec: 5 seconds).
const policyFilterTimeout = 5 * time.Second

// policyFilterFunc returns a PolicyFilterFunc that calls external plugins via IPC.
// The reactor's API server is used to look up plugin connections.
// rawPayload is the raw UPDATE body bytes for AC-15 (raw mode) - may be nil.
// Implements AC-13 (reject modify of undeclared attributes) and AC-15 (raw mode).
func (r *Reactor) policyFilterFunc(rawPayload []byte) PolicyFilterFunc {
	return func(pluginName, filterName, direction, peer string, peerAS uint32, updateText string) PolicyResponse {
		if r.api == nil {
			reactorLogger().Warn("policy filter: no API server", "plugin", pluginName, "filter", filterName)
			// Failed: a guard MISS, the filter never ran. Mirrors the same
			// condition in runEgressPolicyChainASN4, which reaches this state
			// first on the egress path but does not shadow it entirely.
			return PolicyResponse{Action: PolicyReject, Failed: true} // fail-closed
		}

		// Look up filter declaration for AC-13 (attribute validation) and AC-15 (raw mode).
		declaredAttrs, wantsRaw := r.api.FilterInfo(pluginName, filterName)

		// AC-15: If filter declared raw=true, include the raw UPDATE body as bytes
		// (encoding/json base64-encodes it; the plugin always receives a copy via
		// the DirectBridge/socket marshal, so passing rawPayload directly is safe).
		var rawBytes []byte
		if wantsRaw && len(rawPayload) > 0 {
			rawBytes = rawPayload
		}

		ctx, cancel := context.WithTimeout(context.Background(), policyFilterTimeout)
		defer cancel()

		input := &rpc.FilterUpdateInput{
			Filter:    filterName,
			Direction: direction,
			Peer:      peer,
			PeerAS:    peerAS,
			Update:    updateText,
			Raw:       rawBytes,
		}

		out, err := r.api.CallFilterUpdate(ctx, pluginName, input)
		if err != nil {
			onError := r.api.FilterOnError(pluginName, filterName)
			reactorLogger().Warn("policy filter IPC error", "plugin", pluginName, "filter", filterName, "on-error", onError, "error", err)
			if onError == rpc.OnErrorAccept {
				return PolicyResponse{Action: PolicyAccept}
			}
			// Failed: the filter never decided. Still rejected (fail-closed),
			// but not a policy decision -- see PolicyResponse.Failed.
			return PolicyResponse{Action: PolicyReject, Failed: true}
		}

		action, ok := toPolicyAction(out.Action)
		if !ok {
			reactorLogger().Warn("policy filter: invalid action", "plugin", pluginName, "filter", filterName, "action", out.Action)
			return PolicyResponse{Action: PolicyReject, Failed: true} // fail-closed on invalid response
		}

		// AC-13: Validate that modify delta only touches declared attributes.
		if action == PolicyModify && len(declaredAttrs) > 0 && out.Update != "" {
			if violation := validateModifyDelta(out.Update, declaredAttrs); violation != "" {
				reactorLogger().Warn("policy filter: modify of undeclared attribute",
					"plugin", pluginName, "filter", filterName, "violation", violation)
				// Failed: the filter decided PolicyModify and ze overrode it. The
				// route is still rejected (fail-closed), but no filter chose to
				// drop it, so an outcome counter must not read this as policy.
				return PolicyResponse{Action: PolicyReject, Failed: true} // reject invalid modify
			}
		}

		resp := PolicyResponse{Action: action, Delta: out.Update}
		// Raw full-payload replacement is only meaningful on a modify.
		if action == PolicyModify {
			resp.Raw = out.Raw
		}
		// Teardown is honored only for import (received) UPDATEs; ignore it on
		// export so a misconfigured/misbehaving filter cannot drop sessions there.
		if out.Teardown && direction == directionImport {
			resp.Teardown = true
			resp.NotifyCode = out.NotifyCode
			resp.NotifySubcode = out.NotifySubcode
		}
		return resp
	}
}

// decodeFilterRawOverride validates a full UPDATE-body replacement from a raw
// policy filter. Returns nil on empty input or a body too short to be a valid
// UPDATE (the caller then keeps the unmodified payload). A valid UPDATE body is
// at least 4 bytes: withdrawn-routes-length(2) + path-attr-length(2).
func decodeFilterRawOverride(raw []byte) []byte {
	if len(raw) < 4 {
		return nil
	}
	return raw
}

// validateModifyDelta checks that a modify delta only contains attributes
// from the declared set. Returns the first violating attribute name, or "".
func validateModifyDelta(delta string, declaredAttrs []string) string {
	allowed := make(map[string]bool, len(declaredAttrs))
	for _, a := range declaredAttrs {
		allowed[a] = true
	}

	// Parse the delta to find which attributes it touches.
	deltaAttrs := parseFilterAttrs(delta)
	if deltaAttrs.unknownName != "" && !allowed[deltaAttrs.unknownName] {
		return deltaAttrs.unknownName
	}
	for id := filterAttrID(0); id < faCount; id++ { //nolint:modernize // prealloc linter crashes on range-over-int
		if deltaAttrs.has(id) && !allowed[filterAttrNames[id]] {
			return filterAttrNames[id]
		}
	}

	return ""
}

// toPolicyAction maps the plugin's typed FilterAction to the reactor's
// internal PolicyAction. Returns false for unspecified or unknown values;
// the caller fails closed (reject) in that case.
func toPolicyAction(a rpc.FilterAction) (PolicyAction, bool) {
	switch a {
	case rpc.FilterAccept:
		return PolicyAccept, true
	case rpc.FilterReject:
		return PolicyReject, true
	case rpc.FilterModify:
		return PolicyModify, true
	case rpc.FilterUnspecified:
		return PolicyReject, false
	}
	return PolicyReject, false
}
