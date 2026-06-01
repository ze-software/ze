// Design: docs/architecture/plugin/rib-storage-design.md — Adj-RIB-In command handlers
// Overview: rib.go — core types, event handlers, and raw hex storage
// Related: rib_validation.go — RPKI validation gate (pending routes, timeout, state constants)
package adj_rib_in

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

var (
	errAdjRibInReplayRequiresTarget         = errors.New("request adj-rib-in replay requires target peer address")
	errAcceptRoutesRequiresPeerFamilyPrefix = errors.New("accept-routes requires: <peer> <family> <prefix> <pathID> <state>")
	errRejectRoutesRequiresPeerFamilyPrefix = errors.New("reject-routes requires: <peer> <family> <prefix> <pathID>")
	errRevalidateRequiresFamilyPrefix       = errors.New("revalidate requires: <family> <prefix>")
)

// handleCommand processes command requests via SDK execute-command callback.
// Returns (status, data, error) for the SDK to send back to the engine.
func (r *AdjRIBInManager) handleCommand(command string, args []string, peer string) (string, any, error) {
	switch command {
	case "show adj-rib-in status":
		return statusDone, r.status(), nil
	case "show adj-rib-in":
		return statusDone, r.show(showSelector(args, peer)), nil
	case "request adj-rib-in replay":
		return r.replayCommand(args)
	case "request adj-rib-in enable-validation":
		return r.enableValidationCommand()
	case "request adj-rib-in accept-routes":
		return r.acceptRoutesCommand(args)
	case "request adj-rib-in reject-routes":
		return r.rejectRoutesCommand(args)
	case "request adj-rib-in revalidate":
		return r.revalidateCommand(args)
	} // unknown commands return error below
	return statusError, "", fmt.Errorf("unknown command: %s", command)
}

func showSelector(args []string, peer string) string {
	if peer != "" && peer != "*" {
		return peer
	}
	if len(args) > 0 {
		return args[0]
	}
	return peer
}

// status returns adj-RIB-in status.
func (r *AdjRIBInManager) status() any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalRoutes := 0
	peers := make(map[string]int)
	for peer, routes := range r.ribIn {
		peers[peer] = routes.Len()
		totalRoutes += routes.Len()
	}

	return map[string]any{
		"running":      true,
		"total-routes": totalRoutes,
		"peers":        peers,
	}
}

// show returns routes for a peer as human-readable JSON.
func (r *AdjRIBInManager) show(selector string) any {
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

	return map[string]any{"adj-rib-in": result}
}

// replayCommand handles "request adj-rib-in replay" via execute-command.
// Args format: "<target-peer> [<from-index>]".
// Replays routes from ALL source peers except target, filtered by from-index.
func (r *AdjRIBInManager) replayCommand(args []string) (string, any, error) {
	if len(args) == 0 {
		return statusError, "", errAdjRibInReplayRequiresTarget
	}

	targetPeer := args[0]
	var fromIndex uint64
	if len(args) > 1 {
		var err error
		fromIndex, err = strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return statusError, "", fmt.Errorf("invalid from-index: %s", args[1])
		}
	}

	cmds, maxSeq := r.buildReplayCommands(targetPeer, fromIndex)

	// Send all replay commands to target peer.
	for _, cmd := range cmds {
		r.updateRoute(targetPeer, cmd)
	}

	return statusDone, map[string]any{"last-index": maxSeq, "replayed": len(cmds)}, nil
}

// enableValidationCommand handles "request adj-rib-in enable-validation".
// Sets the validationEnabled flag so subsequent routes use pending state.
func (r *AdjRIBInManager) enableValidationCommand() (string, any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.validationEnabled = true
	logger().Info("validation gate enabled")
	return statusDone, map[string]any{"validation-enabled": true}, nil
}

// acceptRoutesCommand handles "request adj-rib-in accept-routes <peer> <family> <prefix> <pathID> <state>".
// Promotes a pending route to installed with the given validation state.
func (r *AdjRIBInManager) acceptRoutesCommand(args []string) (string, any, error) {
	if len(args) < 5 {
		return statusError, "", errAcceptRoutesRequiresPeerFamilyPrefix
	}

	peerAddr := args[0]
	fam := args[1]
	prefix := args[2]
	pathID, err := strconv.ParseUint(args[3], 10, 32)
	if err != nil {
		return statusError, "", fmt.Errorf("accept-routes: invalid pathID %q: %w", args[3], err)
	}
	valState, err := parseValidationState(args[4])
	if err != nil {
		return statusError, "", err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	rKey := bgp.RouteKey(fam, prefix, uint32(pathID))
	key := pendingKey(peerAddr, rKey)
	pr, ok := r.pending[key]
	if !ok {
		r.storeEarlyDecision(peerAddr, rKey, earlyAccept, valState)
		return statusDone, map[string]any{"status": "ok", "early": true}, nil
	}

	r.promoteToInstalled(pr, valState)
	delete(r.pending, key)

	return statusDone, map[string]any{"status": "ok"}, nil
}

// rejectRoutesCommand handles "request adj-rib-in reject-routes <peer> <family> <prefix> <pathID>".
// Discards a pending route (does not install it).
func (r *AdjRIBInManager) rejectRoutesCommand(args []string) (string, any, error) {
	if len(args) < 4 {
		return statusError, "", errRejectRoutesRequiresPeerFamilyPrefix
	}

	peerAddr := args[0]
	fam := args[1]
	prefix := args[2]
	pathID, err := strconv.ParseUint(args[3], 10, 32)
	if err != nil {
		return statusError, "", fmt.Errorf("reject-routes: invalid pathID %q: %w", args[3], err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	rKey := bgp.RouteKey(fam, prefix, uint32(pathID))
	key := pendingKey(peerAddr, rKey)
	if _, ok := r.pending[key]; !ok {
		r.storeEarlyDecision(peerAddr, rKey, earlyReject, 0)
		return statusDone, map[string]any{"status": "ok", "early": true}, nil
	}

	delete(r.pending, key)
	logger().Debug("rejected pending route", "peer", peerAddr, "family", fam, "prefix", prefix, "pathID", pathID)

	return statusDone, map[string]any{"status": "ok"}, nil
}

// revalidateCommand handles "request adj-rib-in revalidate <family> <prefix>".
// Returns installed route data for the given prefix so the validator can re-validate.
func (r *AdjRIBInManager) revalidateCommand(args []string) (string, any, error) {
	if len(args) < 2 {
		return statusError, "", errRevalidateRequiresFamilyPrefix
	}

	famStr := args[0]
	prefix := args[1]

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

	return statusDone, map[string]any{"routes": routes}, nil
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
