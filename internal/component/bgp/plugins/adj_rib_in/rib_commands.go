// Design: docs/architecture/plugin/rib-storage-design.md — Adj-RIB-In command handlers
// Overview: rib.go — core types, event handlers, and raw hex storage
// Related: rib_validation.go — RPKI validation gate (pending routes, timeout, state constants)
package adj_rib_in

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// handleCommand processes command requests via SDK execute-command callback.
// Returns (status, data, error) for the SDK to send back to the engine.
func (r *AdjRIBInManager) handleCommand(command, selector string) (string, string, error) {
	switch command {
	case "adj-rib-in status":
		return statusDone, r.statusJSON(), nil
	case "adj-rib-in show":
		return statusDone, r.showJSON(selector), nil
	case "adj-rib-in replay":
		return r.replayCommand(selector)
	case "adj-rib-in enable-validation":
		return r.enableValidationCommand()
	case "adj-rib-in accept-routes":
		return r.acceptRoutesCommand(selector)
	case "adj-rib-in reject-routes":
		return r.rejectRoutesCommand(selector)
	case "adj-rib-in revalidate":
		return r.revalidateCommand(selector)
	} // unknown commands return error below
	return statusError, "", fmt.Errorf("unknown command: %s", command)
}

// statusJSON returns status as JSON.
func (r *AdjRIBInManager) statusJSON() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalRoutes := 0
	peers := make(map[string]int)
	for peer, routes := range r.ribIn {
		peers[peer] = routes.Len()
		totalRoutes += routes.Len()
	}

	data, err := json.Marshal(map[string]any{
		"running":      true,
		"total-routes": totalRoutes,
		"peers":        peers,
	})
	if err != nil {
		return `{"error":"marshal failed"}`
	}
	return string(data)
}

// showJSON returns routes for a peer as human-readable JSON.
func (r *AdjRIBInManager) showJSON(selector string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]map[string]any)

	for peer, routes := range r.ribIn {
		if !matchesPeer(peer, selector) {
			continue
		}
		routeList := make([]map[string]any, 0, routes.Len())
		routes.Range(func(key string, seq uint64, rt *RawRoute) bool {
			routeMap := map[string]any{
				"family":           rt.Family.String(),
				"key":              key,
				"nhop-hex":         rt.NHopHex,
				"attr-hex":         rt.AttrHex,
				"nlri-hex":         rt.NLRIHex,
				"seq-index":        seq,
				"validation-state": rt.ValidationState,
			}
			routeList = append(routeList, routeMap)
			return true
		})
		if len(routeList) > 0 {
			result[peer] = routeList
		}
	}

	data, err := json.Marshal(map[string]any{"adj-rib-in": result})
	if err != nil {
		return `{"error":"marshal failed"}`
	}
	return string(data)
}

// replayCommand handles "adj-rib-in replay" via execute-command.
// Selector format: "<target-peer> [<from-index>]"
// Replays routes from ALL source peers except target, filtered by from-index.
func (r *AdjRIBInManager) replayCommand(selector string) (string, string, error) {
	parts := strings.Fields(selector)
	if len(parts) == 0 {
		return statusError, "", fmt.Errorf("adj-rib-in replay requires target peer address")
	}

	targetPeer := parts[0]
	var fromIndex uint64
	if len(parts) > 1 {
		var err error
		fromIndex, err = strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return statusError, "", fmt.Errorf("invalid from-index: %s", parts[1])
		}
	}

	cmds, maxSeq := r.buildReplayCommands(targetPeer, fromIndex)

	// Send all replay commands to target peer.
	for _, cmd := range cmds {
		r.updateRoute(targetPeer, cmd)
	}

	data := fmt.Sprintf(`{"last-index":%d,"replayed":%d}`, maxSeq, len(cmds))
	return statusDone, data, nil
}

// enableValidationCommand handles "adj-rib-in enable-validation".
// Sets the validationEnabled flag so subsequent routes use pending state.
func (r *AdjRIBInManager) enableValidationCommand() (string, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.validationEnabled = true
	logger().Info("validation gate enabled")
	return statusDone, `{"validation-enabled":true}`, nil
}

// acceptRoutesCommand handles "adj-rib-in accept-routes <peer> <family> <prefix> <state>".
// Promotes a pending route to installed with the given validation state.
func (r *AdjRIBInManager) acceptRoutesCommand(selector string) (string, string, error) {
	parts := strings.Fields(selector)
	if len(parts) < 4 {
		return statusError, "", fmt.Errorf("accept-routes requires: <peer> <family> <prefix> <state>")
	}

	peerAddr := parts[0]
	fam := parts[1]
	prefix := parts[2]
	valState, err := parseValidationState(parts[3])
	if err != nil {
		return statusError, "", err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	rKey := bgp.RouteKey(fam, prefix, 0)
	key := pendingKey(peerAddr, rKey)
	pr, ok := r.pending[key]
	if !ok {
		return statusError, "", fmt.Errorf("no pending route for %s %s %s", peerAddr, fam, prefix)
	}

	r.promoteToInstalled(pr, valState)
	delete(r.pending, key)

	return statusDone, `{"status":"ok"}`, nil
}

// rejectRoutesCommand handles "adj-rib-in reject-routes <peer> <family> <prefix>".
// Discards a pending route (does not install it).
func (r *AdjRIBInManager) rejectRoutesCommand(selector string) (string, string, error) {
	parts := strings.Fields(selector)
	if len(parts) < 3 {
		return statusError, "", fmt.Errorf("reject-routes requires: <peer> <family> <prefix>")
	}

	peerAddr := parts[0]
	fam := parts[1]
	prefix := parts[2]

	r.mu.Lock()
	defer r.mu.Unlock()

	rKey := bgp.RouteKey(fam, prefix, 0)
	key := pendingKey(peerAddr, rKey)
	if _, ok := r.pending[key]; !ok {
		return statusError, "", fmt.Errorf("no pending route for %s %s %s", peerAddr, fam, prefix)
	}

	delete(r.pending, key)
	logger().Debug("rejected pending route", "peer", peerAddr, "family", fam, "prefix", prefix)

	return statusDone, `{"status":"ok"}`, nil
}

// revalidateCommand handles "adj-rib-in revalidate <family> <prefix>".
// Returns installed route data for the given prefix so the validator can re-validate.
func (r *AdjRIBInManager) revalidateCommand(selector string) (string, string, error) {
	parts := strings.Fields(selector)
	if len(parts) < 2 {
		return statusError, "", fmt.Errorf("revalidate requires: <family> <prefix>")
	}

	famStr := parts[0]
	prefix := parts[1]

	fam, ok := family.LookupFamily(famStr)
	if !ok {
		return statusError, "", fmt.Errorf("unknown family: %s", famStr)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var routes []map[string]any
	allPrefixes := prefix == "*"
	for peer, peerRoutes := range r.ribIn {
		peerRoutes.Range(func(key string, _ uint64, rt *RawRoute) bool {
			if rt.Family != fam {
				return true
			}
			// Match exact prefix via RouteKey, or all prefixes with "*".
			if !allPrefixes && !strings.HasPrefix(key, famStr+":"+prefix+":") &&
				key != famStr+":"+prefix {
				return true
			}
			routes = append(routes, map[string]any{
				"peer":             peer,
				"family":           famStr,
				"prefix":           prefix,
				"attr-hex":         rt.AttrHex,
				"nhop-hex":         rt.NHopHex,
				"nlri-hex":         rt.NLRIHex,
				"validation-state": rt.ValidationState,
			})
			return true
		})
	}

	data, err := json.Marshal(map[string]any{"routes": routes})
	if err != nil {
		return statusError, "", fmt.Errorf("marshal revalidate response: %w", err)
	}
	return statusDone, string(data), nil
}

// matchesPeer returns true if peerAddr matches the selector string.
// Supports: *, empty (all), IP, !IP (negation).
func matchesPeer(peerAddr, selector string) bool {
	selector = strings.TrimSpace(selector)
	if selector == "" || selector == "*" {
		return true
	}
	if strings.HasPrefix(selector, "!") {
		return peerAddr != strings.TrimSpace(selector[1:])
	}
	return peerAddr == selector
}
