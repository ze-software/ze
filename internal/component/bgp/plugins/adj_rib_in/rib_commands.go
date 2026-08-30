// Design: docs/architecture/plugin/rib-storage-design.md — Adj-RIB-In command handlers
// Overview: rib.go — core types, event handlers, and raw hex storage
// Related: rib_validation.go — RPKI validation gate (pending routes, timeout, state constants)
package adj_rib_in

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var (
	errAdjRibInReplayRequiresTarget         = errors.New("request bgp adj-rib-in replay requires target peer address")
	errAcceptRoutesRequiresPeerFamilyPrefix = errors.New("accept-routes requires: <peer> <family> <prefix> <pathID> <state>")
	errRejectRoutesRequiresPeerFamilyPrefix = errors.New("reject-routes requires: <peer> <family> <prefix> <pathID>")
	errRevalidateRequiresFamilyPrefix       = errors.New("revalidate requires: <family> <prefix>")
	errBatchValidateStride                  = errors.New("batch-validate requires args in groups of 6: action peer family prefix pathID state")
)

// handleCommand processes command requests via SDK execute-command callback.
// Returns (status, data, error) for the SDK to send back to the engine.
func (r *AdjRIBInManager) handleCommand(command string, args []string, peer string) (string, any, error) {
	switch command {
	case "show bgp adj-rib-in status":
		return statusDone, r.status(), nil
	case "show bgp adj-rib-in":
		return statusDone, r.show(showSelector(args, peer)), nil
	case "request bgp adj-rib-in replay":
		return r.replayCommand(args)
	case "request bgp adj-rib-in claim-replay":
		return r.claimReplayCommand()
	case "request bgp adj-rib-in enable-validation":
		return r.enableValidationCommand()
	case "request bgp adj-rib-in accept-routes":
		return r.acceptRoutesCommand(args)
	case "request bgp adj-rib-in reject-routes":
		return r.rejectRoutesCommand(args)
	case "request bgp adj-rib-in batch-validate":
		return r.batchValidateCommand(args)
	case "request bgp adj-rib-in revalidate":
		return r.revalidateCommand(args)
	} // unknown commands return error below
	return statusError, "", fmt.Errorf("unknown command: %s", command)
}

// JSON keys of the adj-rib-in command responses.
const (
	jsonKeyEarly  = "early"
	jsonKeyStatus = "status"
)

const (
	batchValidateStride   = 6
	maxBatchValidateCount = 256 // higher than rpki sender's 128: external plugins may batch up to 256
)

// handleBatchValidateTyped processes typed validation decisions from the
// DirectBridge fast path, bypassing string serialization entirely.
// Pre-validates all decisions before mutating state (same as batchValidateCommand).
func (r *AdjRIBInManager) handleBatchValidateTyped(decisions []rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
	if len(decisions) == 0 {
		return &rpc.BatchValidateResult{}, nil
	}
	if len(decisions) > maxBatchValidateCount {
		return nil, fmt.Errorf("batch-validate: %d decisions exceeds maximum %d", len(decisions), maxBatchValidateCount)
	}
	peerAddrs := make([]netip.Addr, len(decisions))
	for i := range decisions {
		// RFC requirement: RFC6811-2-1 -- accepted routes retain any of the
		// three lookup results defined by RFC 6811 Section 2.
		if decisions[i].Accept &&
			decisions[i].ValState != ValidationValid &&
			decisions[i].ValState != ValidationNotFound &&
			decisions[i].ValState != ValidationInvalid {
			return nil, fmt.Errorf("batch-validate: invalid validation state %d at index %d (expected 1=Valid, 2=NotFound, or 3=Invalid)", decisions[i].ValState, i)
		}
		addr, err := netip.ParseAddr(decisions[i].PeerAddr)
		if err != nil {
			return nil, fmt.Errorf("batch-validate: invalid peer address %q at index %d: %w (expected an IP address)", decisions[i].PeerAddr, i, err)
		}
		peerAddrs[i] = addr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var accepted, rejected, early int
	for i := range decisions {
		d := &decisions[i]
		peerAddr := peerAddrs[i]
		dFam, _ := family.LookupFamily(d.Family)
		rKey := routeKeyFromStrings(dFam, d.Prefix, d.PathID)
		pKey := pendingKey(peerAddr, rKey)

		if d.Accept {
			pr, ok := r.pending[pKey]
			if !ok {
				if r.applyToInstalled(peerAddr, rKey, true, d.ValState) {
					accepted++
					continue
				}
				r.storeEarlyDecision(peerAddr, rKey, earlyAccept, d.ValState)
				early++
				accepted++
				continue
			}
			r.promoteToInstalled(pr, d.ValState)
			delete(r.pending, pKey)
			accepted++
		} else {
			if _, ok := r.pending[pKey]; !ok {
				if r.applyToInstalled(peerAddr, rKey, false, 0) {
					rejected++
					continue
				}
				r.storeEarlyDecision(peerAddr, rKey, earlyReject, 0)
				early++
				rejected++
				continue
			}
			delete(r.pending, pKey)
			logger().Debug("rejected pending route (batch-typed)", "peer", d.PeerAddr, "family", d.Family, "prefix", d.Prefix, "pathID", d.PathID)
			rejected++
		}
	}

	return &rpc.BatchValidateResult{Accepted: accepted, Rejected: rejected, Early: early}, nil
}

// batchValidateCommand processes multiple accept/reject decisions under a single lock.
// Args are groups of 6: action("a"|"r"), peer, family, prefix, pathID, state.
// State is "1" (Valid) or "2" (NotFound) for accepts; ignored for rejects.
//
// Pre-validates the entire args slice before mutating state so a parse error
// mid-batch does not leave the RIB partially applied.
func (r *AdjRIBInManager) batchValidateCommand(args []string) (string, any, error) {
	if len(args) == 0 {
		return statusDone, map[string]any{"accepted": 0, "rejected": 0, jsonKeyEarly: 0}, nil
	}
	if len(args)%batchValidateStride != 0 {
		return statusError, "", errBatchValidateStride
	}

	n := len(args) / batchValidateStride
	if n > maxBatchValidateCount {
		return statusError, "", fmt.Errorf("batch-validate: %d decisions exceeds maximum %d", n, maxBatchValidateCount)
	}
	type decision struct {
		action   byte
		peerAddr netip.Addr
		peerStr  string // original arg, for debug logging
		fam      string
		prefix   string
		pathID   uint32
		valState uint8
	}
	decisions := make([]decision, n)

	for i := range n {
		off := i * batchValidateStride
		act := args[off]
		if act != "a" && act != "r" {
			return statusError, "", fmt.Errorf("batch-validate: unknown action %q at index %d (expected \"a\" or \"r\")", act, off)
		}
		peerAddr, err := netip.ParseAddr(args[off+1])
		if err != nil {
			return statusError, "", fmt.Errorf("batch-validate: invalid peer address %q at index %d: %w (expected an IP address)", args[off+1], off+1, err)
		}
		pathID, err := strconv.ParseUint(args[off+4], 10, 32)
		if err != nil {
			return statusError, "", fmt.Errorf("batch-validate: invalid pathID %q at index %d: %w", args[off+4], off+4, err)
		}
		var valState uint8
		if act == "a" {
			valState, err = parseValidationState(args[off+5])
			if err != nil {
				return statusError, "", err
			}
		}
		decisions[i] = decision{
			action: act[0], peerAddr: peerAddr, peerStr: args[off+1], fam: args[off+2],
			prefix: args[off+3], pathID: uint32(pathID), valState: valState,
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var accepted, rejected, early int
	for i := range decisions {
		d := &decisions[i]
		dFam, _ := family.LookupFamily(d.fam)
		rKey := routeKeyFromStrings(dFam, d.prefix, d.pathID)
		pKey := pendingKey(d.peerAddr, rKey)

		switch d.action {
		case 'a':
			pr, ok := r.pending[pKey]
			if !ok {
				if r.applyToInstalled(d.peerAddr, rKey, true, d.valState) {
					accepted++
					continue
				}
				r.storeEarlyDecision(d.peerAddr, rKey, earlyAccept, d.valState)
				early++
				accepted++
				continue
			}
			r.promoteToInstalled(pr, d.valState)
			delete(r.pending, pKey)
			accepted++
		case 'r':
			if _, ok := r.pending[pKey]; !ok {
				if r.applyToInstalled(d.peerAddr, rKey, false, 0) {
					rejected++
					continue
				}
				r.storeEarlyDecision(d.peerAddr, rKey, earlyReject, 0)
				early++
				rejected++
				continue
			}
			delete(r.pending, pKey)
			logger().Debug("rejected pending route (batch)", "peer", d.peerStr, "family", d.fam, "prefix", d.prefix, "pathID", d.pathID)
			rejected++
		}
	}

	return statusDone, map[string]any{"accepted": accepted, "rejected": rejected, jsonKeyEarly: early}, nil
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
		peers[peer.String()] = routes.Len()
		totalRoutes += routes.Len()
	}

	return map[string]any{
		"running":      true,
		"total-routes": totalRoutes,
		"peers":        peers,
	}
}

// show returns routes for a peer as human-readable JSON.
func (r *AdjRIBInManager) show(selectorStr string) any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sel := selector.ParseDefault(selectorStr)
	result := make(map[string][]map[string]any)

	for peer, routes := range r.ribIn {
		if !sel.Matches(peer) {
			continue
		}
		routeList := make([]map[string]any, 0, routes.Len())
		routes.Range(func(key compactRouteKey, seq uint64, rt *RawRoute) bool {
			keyStr := key.Fam.String() + ":" + key.Prefix.String()
			if key.PathID > 0 {
				keyStr += ":" + textbuf.StringUint32(key.PathID)
			}
			routeMap := map[string]any{
				"family":           rt.Family.String(),
				"key":              keyStr,
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
			result[peer.String()] = routeList
		}
	}

	return map[string]any{"adj-rib-in": result}
}

// replayCommand handles "request bgp adj-rib-in replay" via execute-command.
// Args format: "<target-peer> [<from-index> [<max-msg-id>]]".
// Replays routes from ALL source peers except target, resuming strictly after
// from-index and stopping at the max-msg-id cut.
//
// max-msg-id is the peer-up cut: the reactor MessageID that was the newest the
// CALLER had seen when it made this peer a live forward target. Routes newer
// than that belong to the live rail, so replaying them too would deliver the
// same route twice, in an order decided by goroutine scheduling.
//
// PRESENCE, not value, decides whether the replay is bounded. Supplying the
// argument bounds the replay even at 0; OMITTING it means unbounded, which stays
// correct for a caller that does not track a cut. Treating the VALUE 0 as
// unbounded is what let a peer that established before bgp-rs had taken delivery
// of its first UPDATE (cut 0, the ordinary case) receive every route twice --
// once replayed here, once forwarded live. See replay_cut.go.
func (r *AdjRIBInManager) replayCommand(args []string) (string, any, error) {
	if len(args) == 0 {
		return statusError, "", errAdjRibInReplayRequiresTarget
	}

	targetPeer, err := netip.ParseAddr(args[0])
	if err != nil {
		return statusError, "", fmt.Errorf("adj-rib-in replay: invalid target peer address %q: %w (expected an IP address)", args[0], err)
	}
	var fromIndex uint64
	if len(args) > 1 {
		fromIndex, err = strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return statusError, "", fmt.Errorf("invalid from-index: %s", args[1])
		}
	}
	cut := unboundedReplay()
	if len(args) > 2 {
		maxMsgID, parseErr := strconv.ParseUint(args[2], 10, 64)
		if parseErr != nil {
			return statusError, "", fmt.Errorf("invalid max-msg-id: %s", args[2])
		}
		cut = replayUpTo(maxMsgID)
	}

	routes, maxSeq := r.buildReplayRoutes(targetPeer, fromIndex, cut)

	// The relay carries the whole replay (original arg as the destination), in
	// bounded chunks. A failure surfaces as statusError so the caller can tell a
	// replay that happened from one that did not: bgp-rs logs it at ERROR and
	// skips its delta-convergence loop rather than converging against nothing.
	// It still sends End-of-RIB either way (rs/server_handlers.go, "Always send
	// EOR when replay terminates"), so this is visibility, not suppression.
	if err := r.relayRoutes(args[0], routes); err != nil {
		return statusError, "", fmt.Errorf("adj-rib-in replay to %s: %w", args[0], err)
	}

	// ingested-msg-id is the caller's convergence signal: it lets a cut-bounded
	// caller stop for a reason instead of inferring completion from an empty
	// answer, which an empty STORE also produces. Conflating those two is what
	// let a caller stop at once while the cut was still suppressing the same
	// routes on the live rail, so neither rail delivered them.
	//
	// Omitted rather than zeroed when untracked: 0 is the real "nothing ingested
	// yet" and is precisely the losing case (see replay_cut.go).
	result := map[string]any{"last-index": maxSeq, "replayed": len(routes)}
	if pos, tracked := r.ingestPosition(); tracked {
		result["ingested-msg-id"] = pos
	}
	return statusDone, result, nil
}

// claimReplayCommand handles "request bgp adj-rib-in claim-replay".
//
// This is the LATE-JOIN corrective, not the startup path. The startup path is
// declarative: bgp-rs puts the peer-up-replay role in its registration, the
// engine unions the claims of the startup set and delivers them on this
// plugin's Stage-2 configure callback, and applyStartupClaims (rib.go) stands
// self-replay down there -- before the engine starts peers, with no timing
// window. This command exists for a bgp-rs that appears AFTER this plugin was
// configured (mid-life config-reload auto-load, or a respawn), where that
// declaration could not have reached it.
//
// Do not move the startup decision back onto this command. It is dispatched
// from the owner's OnAllPluginsReady, which the engine fans out on detached
// goroutines immediately before StartPeers
// (internal/component/plugin/server/startup.go, sendPostStartupToAll -- waiting
// there deadlocks, as its doc comment records). While that WAS the startup
// path, the first peer raced it by 1-2 ms on an idle host and was replayed
// twice under load.
//
// Standing down matters because the owner does NOT gate the concurrent forward
// while its replay runs: bgp-rs selects forward targets on peer.Up alone
// (rs/server_forward.go selectForwardTargets), deliberately, because excluding a
// replaying peer loses routes and a duplicate UPDATE does not. So ownership being
// settled before a peer establishes is the only thing preventing a doubled replay.
//
// It is deliberately NOT the plain "replay" verb doing the claiming. Latching on
// the first replay had two defects: the FIRST peer-up still raced (nothing had
// claimed yet, so both the self-replay and the owner's replay fired and the route
// went out twice -- the very duplicate this spec fixes), and an operator running
// the diagnostic "request bgp adj-rib-in replay" once on a standalone deployment
// silently disabled peer-up replay for the lifetime of the process.
//
// Hidden from the CLI: this is plumbing between two plugins, not an operator verb.
func (r *AdjRIBInManager) claimReplayCommand() (string, any, error) {
	already := r.replayOwned.Swap(true)
	if !already {
		logger().Info("peer-up replay ownership claimed by another plugin; self-replay disabled")
	}
	return statusDone, map[string]any{"claimed": true, "already-owned": already}, nil
}

// enableValidationCommand handles "request bgp adj-rib-in enable-validation".
// Sets the validationEnabled flag so subsequent routes use pending state.
func (r *AdjRIBInManager) enableValidationCommand() (string, any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.validationEnabled = true
	logger().Info("validation gate enabled")
	return statusDone, map[string]any{"validation-enabled": true}, nil
}

// acceptRoutesCommand handles "request bgp adj-rib-in accept-routes <peer> <family> <prefix> <pathID> <state>".
// Promotes a pending route to installed with the given validation state.
func (r *AdjRIBInManager) acceptRoutesCommand(args []string) (string, any, error) {
	if len(args) < 5 {
		return statusError, "", errAcceptRoutesRequiresPeerFamilyPrefix
	}

	peerAddr, err := netip.ParseAddr(args[0])
	if err != nil {
		return statusError, "", fmt.Errorf("accept-routes: invalid peer address %q: %w (expected an IP address)", args[0], err)
	}
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

	lookupFam, _ := family.LookupFamily(fam)
	rKey := routeKeyFromStrings(lookupFam, prefix, uint32(pathID))
	key := pendingKey(peerAddr, rKey)
	pr, ok := r.pending[key]
	if !ok {
		if r.applyToInstalled(peerAddr, rKey, true, valState) {
			return statusDone, map[string]any{jsonKeyStatus: "ok"}, nil
		}
		r.storeEarlyDecision(peerAddr, rKey, earlyAccept, valState)
		return statusDone, map[string]any{jsonKeyStatus: "ok", jsonKeyEarly: true}, nil
	}

	r.promoteToInstalled(pr, valState)
	delete(r.pending, key)

	return statusDone, map[string]any{jsonKeyStatus: "ok"}, nil
}

// rejectRoutesCommand handles "request bgp adj-rib-in reject-routes <peer> <family> <prefix> <pathID>".
// Discards a pending route (does not install it).
func (r *AdjRIBInManager) rejectRoutesCommand(args []string) (string, any, error) {
	if len(args) < 4 {
		return statusError, "", errRejectRoutesRequiresPeerFamilyPrefix
	}

	peerAddr, err := netip.ParseAddr(args[0])
	if err != nil {
		return statusError, "", fmt.Errorf("reject-routes: invalid peer address %q: %w (expected an IP address)", args[0], err)
	}
	fam := args[1]
	prefix := args[2]
	pathID, err := strconv.ParseUint(args[3], 10, 32)
	if err != nil {
		return statusError, "", fmt.Errorf("reject-routes: invalid pathID %q: %w", args[3], err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	lookupFam, _ := family.LookupFamily(fam)
	rKey := routeKeyFromStrings(lookupFam, prefix, uint32(pathID))
	key := pendingKey(peerAddr, rKey)
	if _, ok := r.pending[key]; !ok {
		if r.applyToInstalled(peerAddr, rKey, false, 0) {
			return statusDone, map[string]any{jsonKeyStatus: "ok"}, nil
		}
		r.storeEarlyDecision(peerAddr, rKey, earlyReject, 0)
		return statusDone, map[string]any{jsonKeyStatus: "ok", jsonKeyEarly: true}, nil
	}

	delete(r.pending, key)
	logger().Debug("rejected pending route", "peer", peerAddr, "family", fam, "prefix", prefix, "pathID", pathID)

	return statusDone, map[string]any{jsonKeyStatus: "ok"}, nil
}

// revalidateCommand handles "request bgp adj-rib-in revalidate <family> <prefix>".
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
		peerRoutes.Range(func(key compactRouteKey, _ uint64, rt *RawRoute) bool {
			if rt.Family != fam {
				return true
			}
			if !allPrefixes && key.Prefix.String() != prefix {
				return true
			}
			routes = append(routes, map[string]any{
				"peer":             peer.String(),
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
