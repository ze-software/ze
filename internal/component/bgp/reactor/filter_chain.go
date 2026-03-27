// Design: docs/architecture/core-design.md — policy filter chain
// Related: reactor_notify.go — ingress filter invocation point
// Related: reactor_api_forward.go — egress filter invocation point

package reactor

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

const policyAttrNLRI = "nlri"
const policyAttrAtomicAggregate = "atomic-aggregate"

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
// Returns the final action and the accumulated update text (may be modified).
func PolicyFilterChain(filterRefs []string, direction, peer string, peerAS uint32, updateText string, callFilter PolicyFilterFunc) (PolicyAction, string) {
	if len(filterRefs) == 0 {
		return PolicyAccept, updateText
	}

	current := updateText
	for _, ref := range filterRefs {
		pluginName, filterName, _ := strings.Cut(ref, ":")
		result := callFilter(pluginName, filterName, direction, peer, peerAS, current)

		switch result.Action {
		case PolicyReject:
			return PolicyReject, ""
		case PolicyModify:
			current = applyFilterDelta(current, result.Delta)
		case PolicyAccept:
			// continue with current text
		}
	}

	return PolicyAccept, current
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

	// Apply delta: overwrite matching keys, append new ones.
	maps.Copy(currentAttrs, deltaAttrs)

	return formatFilterAttrs(currentAttrs)
}

// parseFilterAttrs parses text-format attributes into a map.
// Each attribute is "name value" where value may contain spaces.
// Special key "nlri" captures the NLRI section.
func parseFilterAttrs(text string) map[string]string {
	attrs := make(map[string]string)
	if text == "" {
		return attrs
	}

	// Single-token attributes (one value after name).
	singleToken := map[string]bool{
		"origin": true, "next-hop": true, "med": true,
		"local-preference": true, policyAttrAtomicAggregate: true,
		"aggregator": true, "originator-id": true,
	}

	fields := strings.Fields(text)
	i := 0
	for i < len(fields) {
		name := fields[i]
		i++

		if name == policyAttrNLRI {
			// Capture everything from "nlri" to end or next known attr.
			start := i - 1
			for i < len(fields) && !isPolicyAttrName(fields[i]) {
				i++
			}
			attrs["nlri"] = strings.Join(fields[start:i], " ")
			continue
		}

		if name == policyAttrAtomicAggregate {
			attrs[name] = ""
			continue
		}

		if singleToken[name] {
			if i < len(fields) {
				attrs[name] = fields[i]
				i++
			}
			continue
		}

		// Multi-token: collect until next attribute name or end.
		var values []string
		for i < len(fields) && !isPolicyAttrName(fields[i]) {
			values = append(values, fields[i])
			i++
		}
		attrs[name] = strings.Join(values, " ")
	}

	return attrs
}

// isPolicyAttrName returns true if the token is a known BGP attribute name.
func isPolicyAttrName(s string) bool {
	switch s {
	case "origin", "as-path", "next-hop", "med", "local-preference",
		policyAttrAtomicAggregate, "aggregator", "community", "originator-id",
		"cluster-list", "extended-community", "large-community", "nlri":
		return true
	}
	return false
}

// formatFilterAttrs converts a map of attributes back to text format.
// Attributes are output in a fixed order for deterministic results.
func formatFilterAttrs(attrs map[string]string) string {
	order := []string{
		"origin", "as-path", "next-hop", "med", "local-preference",
		policyAttrAtomicAggregate, "aggregator", "community", "originator-id",
		"cluster-list", "extended-community", "large-community", "nlri",
	}

	var parts []string
	for _, key := range order {
		val, ok := attrs[key]
		if !ok {
			continue
		}
		if key == "nlri" {
			parts = append(parts, val)
			continue
		}
		if key == policyAttrAtomicAggregate {
			parts = append(parts, key)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", key, val))
	}

	return strings.Join(parts, " ")
}

// policyFilterTimeout is the per-filter IPC timeout (spec: 5 seconds).
const policyFilterTimeout = 5 * time.Second

// policyFilterFunc returns a PolicyFilterFunc that calls external plugins via IPC.
// The reactor's API server is used to look up plugin connections.
func (r *Reactor) policyFilterFunc() PolicyFilterFunc {
	return func(pluginName, filterName, direction, peer string, peerAS uint32, updateText string) PolicyResponse {
		if r.api == nil {
			reactorLogger().Warn("policy filter: no API server", "plugin", pluginName, "filter", filterName)
			return PolicyResponse{Action: PolicyReject} // fail-closed
		}

		ctx, cancel := context.WithTimeout(context.Background(), policyFilterTimeout)
		defer cancel()

		input := &rpc.FilterUpdateInput{
			Filter:    filterName,
			Direction: direction,
			Peer:      peer,
			PeerAS:    peerAS,
			Update:    updateText,
		}

		out, err := r.api.CallFilterUpdate(ctx, pluginName, input)
		if err != nil {
			onError := r.api.FilterOnError(pluginName, filterName)
			reactorLogger().Warn("policy filter IPC error", "plugin", pluginName, "filter", filterName, "on-error", onError, "error", err)
			if onError == "accept" {
				return PolicyResponse{Action: PolicyAccept}
			}
			return PolicyResponse{Action: PolicyReject}
		}

		action, parseErr := parsePolicyAction(out.Action)
		if parseErr != nil {
			reactorLogger().Warn("policy filter: invalid action", "plugin", pluginName, "filter", filterName, "action", out.Action, "error", parseErr)
			return PolicyResponse{Action: PolicyReject} // fail-closed on invalid response
		}

		return PolicyResponse{Action: action, Delta: out.Update}
	}
}

// parsePolicyAction converts a string action from the wire protocol to PolicyAction.
func parsePolicyAction(s string) (PolicyAction, error) {
	switch s {
	case "accept":
		return PolicyAccept, nil
	case "reject":
		return PolicyReject, nil
	case "modify":
		return PolicyModify, nil
	}
	return PolicyReject, fmt.Errorf("unknown filter action %q, expected accept/reject/modify", s)
}
