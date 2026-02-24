// Design: docs/architecture/plugin/rib-storage-design.md — Adj-RIB-In command handlers
// Related: rib.go — core types, event handlers, and raw hex storage
package bgp_adj_rib_in

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
		peers[peer] = len(routes)
		totalRoutes += len(routes)
	}

	data, _ := json.Marshal(map[string]any{
		"running":      true,
		"total-routes": totalRoutes,
		"peers":        peers,
	})
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
		routeList := make([]map[string]any, 0, len(routes))
		for key, rt := range routes {
			routeMap := map[string]any{
				"family":    rt.Family,
				"key":       key,
				"nhop-hex":  rt.NHopHex,
				"attr-hex":  rt.AttrHex,
				"nlri-hex":  rt.NLRIHex,
				"seq-index": rt.SeqIndex,
			}
			routeList = append(routeList, routeMap)
		}
		if len(routeList) > 0 {
			result[peer] = routeList
		}
	}

	data, _ := json.Marshal(map[string]any{"adj-rib-in": result})
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
