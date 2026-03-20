// Design: docs/architecture/plugin/rib-storage-design.md — RIB command handlers
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_nlri.go — NLRI wire format helpers
// Related: rib_attr_format.go — attribute formatting for show enrichment
// Related: bestpath.go — best-path selection (extractCandidate, gatherCandidates, SelectBest)
// Related: rib_pipeline.go — iterator pipeline for show commands (scope, filters, terminals)
// Related: rib_pipeline_best.go — best-path pipeline (bestSource, bestPipeline, bestJSONTerminal)
package rib

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
)

// grTimerMargin is the extra time added to restart-time for the RIB's safety-net timer.
// The margin avoids racing with bgp-gr's normal expiry path.
const grTimerMargin = 5 * time.Second

// autoExpireStale is called by the safety-net timer when restart-time + margin elapses.
// It purges all remaining stale routes for the peer and cleans up GR state.
// RFC 4724 Section 4.2: stale routes MUST NOT persist past restart-time.
//
// The owner parameter is the peerGRState that created this timer. If a consecutive
// restart replaced it (new mark-stale created a new state), the callback is stale
// and must be a no-op — otherwise it would purge the new cycle's routes.
func (r *RIBManager) autoExpireStale(peerAddr string, owner *peerGRState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Guard: skip if grState was replaced by a consecutive restart.
	if r.grState[peerAddr] != owner {
		return
	}

	peerRIB := r.ribInPool[peerAddr]
	if peerRIB != nil {
		purged := peerRIB.PurgeAllStale()
		logger().Info("auto-expire stale", "peer", peerAddr, "purged", purged)
	}

	delete(r.grState, peerAddr)
}

// CommandHandler is the signature for RIB command handlers.
// Registered by plugins via RegisterRIBCommand during init().
type CommandHandler func(r *RIBManager, selector string, args []string) (string, string, error)

// ribCommandEntry holds a registered command handler and its help text.
type ribCommandEntry struct {
	Handler CommandHandler
	Help    string
}

// registeredCommands is the command dispatch table, populated at startup.
// Read-only after startup; no mutex needed.
var registeredCommands = map[string]*ribCommandEntry{}

// builtinsRegistered guards against double-registration of builtin commands.
var builtinsRegistered bool

// registerCommand adds a command handler to the dispatch table.
// Returns an error if the command name is already registered.
func registerCommand(name, help string, handler CommandHandler) error {
	if _, exists := registeredCommands[name]; exists {
		return fmt.Errorf("RIB command %q already registered", name)
	}
	registeredCommands[name] = &ribCommandEntry{Handler: handler, Help: help}
	return nil
}

// registerBuiltinCommands populates the command table with RIB-native commands
// and LLGR extensions. Called from RIB startup (explicit, not init).
// Idempotent via bool guard.
func registerBuiltinCommands() {
	if builtinsRegistered {
		return
	}
	builtinsRegistered = true
	builtins := []struct {
		names   []string
		help    string
		handler CommandHandler
	}{
		{[]string{"rib status", "rib adjacent status"}, "Show RIB status (peer count, route counts)",
			func(r *RIBManager, sel string, _ []string) (string, string, error) {
				return statusDone, r.statusJSON(), nil
			}},
		{[]string{"rib show"}, "Show routes (scope: sent|received|sent-received, filters, terminals)",
			func(r *RIBManager, sel string, args []string) (string, string, error) {
				return statusDone, r.showPipeline(sel, args), nil
			}},
		{[]string{"rib clear in", "rib adjacent inbound empty"}, "Clear Adj-RIB-In routes",
			func(r *RIBManager, sel string, _ []string) (string, string, error) {
				return statusDone, r.inboundEmptyJSON(sel), nil
			}},
		{[]string{"rib clear out", "rib adjacent outbound resend"}, "Resend Adj-RIB-Out routes",
			func(r *RIBManager, sel string, _ []string) (string, string, error) {
				return statusDone, r.outboundResendJSON(sel), nil
			}},
		{[]string{"rib retain-routes"}, "Mark peer RIB for retention",
			func(r *RIBManager, sel string, _ []string) (string, string, error) {
				return statusDone, r.retainRoutesJSON(sel), nil
			}},
		{[]string{"rib release-routes"}, "Release retained peer RIB",
			func(r *RIBManager, sel string, _ []string) (string, string, error) {
				return statusDone, r.releaseRoutesJSON(sel), nil
			}},
		{[]string{"rib mark-stale"}, "Mark peer routes at stale level",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				return r.markStaleCommand(args)
			}},
		{[]string{"rib purge-stale"}, "Purge stale routes for peer",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				return r.purgeStaleCommand(args)
			}},
		{[]string{"rib best"}, "Show best-path per prefix",
			func(r *RIBManager, sel string, args []string) (string, string, error) {
				return statusDone, r.bestPipeline(sel, args), nil
			}},
		{[]string{"rib best status"}, "Show best-path computation status",
			func(r *RIBManager, _ string, _ []string) (string, string, error) {
				return statusDone, r.bestPathStatusJSON(), nil
			}},
		{[]string{"rib help"}, "Show RIB subcommands",
			func(_ *RIBManager, _ string, _ []string) (string, string, error) {
				return statusDone, ribHelpJSON(), nil
			}},
		{[]string{"rib command list"}, "List RIB commands",
			func(_ *RIBManager, _ string, _ []string) (string, string, error) {
				return statusDone, ribCommandListJSON(), nil
			}},
		{[]string{"rib event list"}, "List RIB event types",
			func(_ *RIBManager, _ string, _ []string) (string, string, error) {
				return statusDone, ribEventListJSON(), nil
			}},
	}

	for _, b := range builtins {
		for _, name := range b.names {
			registeredCommands[name] = &ribCommandEntry{Handler: b.handler, Help: b.help}
		}
	}

	// LLGR extension commands. Registered here (not by bgp-gr) because
	// handlers access RIBManager internals. The GR plugin sends these
	// as text commands via DispatchCommand -- no cross-plugin import needed.
	registerLLGRCommands()
}

// handleCommand processes command requests via SDK execute-command callback.
// Dispatches to registered handlers from the command table.
// Returns (status, data, error) for the SDK to send back to the engine.
func (r *RIBManager) handleCommand(command, selector string, args []string) (string, string, error) {
	if entry, ok := registeredCommands[command]; ok {
		return entry.Handler(r, selector, args)
	}
	return statusError, "", fmt.Errorf("unknown command: %s", command)
}

// ribHelpJSON returns RIB subcommands as JSON, built from the command registry.
func ribHelpJSON() string {
	seen := make(map[string]bool)
	var subs []string
	for name := range registeredCommands {
		after, ok := strings.CutPrefix(name, "rib ")
		if !ok {
			continue
		}
		parts := strings.SplitN(after, " ", 2)
		if len(parts) > 0 && !seen[parts[0]] {
			subs = append(subs, parts[0])
			seen[parts[0]] = true
		}
	}
	sort.Strings(subs)
	data, _ := json.Marshal(map[string]any{"subcommands": subs})
	return string(data)
}

// ribCommandListJSON returns all RIB commands as JSON, built from the command registry.
func ribCommandListJSON() string {
	type entry struct {
		Name string `json:"name"`
		Help string `json:"help"`
	}
	cmds := make([]entry, 0, len(registeredCommands))
	for name, e := range registeredCommands {
		cmds = append(cmds, entry{Name: name, Help: e.Help})
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	data, _ := json.Marshal(map[string]any{"commands": cmds})
	return string(data)
}

// ribEventListJSON returns RIB event types as JSON.
func ribEventListJSON() string {
	events := []string{"cache", "route", "peer", "memory"}
	data, _ := json.Marshal(map[string]any{"events": events})
	return string(data)
}

// matchesPeer returns true if peerAddr matches the selector string.
// Supports: *, IP, !IP (negation), IP,IP,IP (multi-IP).
func matchesPeer(peerAddr, selector string) bool {
	selector = strings.TrimSpace(selector)

	if selector == "" || selector == "*" {
		return true
	}

	// Negation: !IP matches all except that IP
	if strings.HasPrefix(selector, "!") {
		excludeIP := strings.TrimSpace(selector[1:])
		return peerAddr != excludeIP
	}

	// Multi-IP: IP,IP,IP matches any in list
	if strings.Contains(selector, ",") {
		for s := range strings.SplitSeq(selector, ",") {
			if strings.TrimSpace(s) == peerAddr {
				return true
			}
		}
		return false
	}

	// Single IP
	return peerAddr == selector
}

// inboundEmptyJSON clears Adj-RIB-In routes for matching peers, returns JSON result.
func (r *RIBManager) inboundEmptyJSON(selector string) string {
	r.mu.Lock()
	cleared := 0

	for peer, peerRIB := range r.ribInPool {
		if !matchesPeer(peer, selector) {
			continue
		}
		cleared += peerRIB.Len()
		peerRIB.Release()
		delete(r.ribInPool, peer)
		delete(r.peerMeta, peer)
	}
	r.mu.Unlock()

	data, _ := json.Marshal(map[string]any{"cleared": cleared})
	return string(data)
}

// outboundResendJSON replays Adj-RIB-Out routes for matching peers, returns JSON result.
// Does NOT send "plugin session ready" - that's only for initial reconnect.
func (r *RIBManager) outboundResendJSON(selector string) string {
	r.mu.RLock()
	var peersToResend []string
	routesToResend := make(map[string][]*Route)

	for peer, routes := range r.ribOut {
		if !matchesPeer(peer, selector) {
			continue
		}
		if !r.peerUp[peer] {
			continue // Only resend to up peers
		}
		peersToResend = append(peersToResend, peer)
		routesCopy := make([]*Route, 0, len(routes))
		for _, rt := range routes {
			routesCopy = append(routesCopy, rt)
		}
		routesToResend[peer] = routesCopy
	}
	r.mu.RUnlock()

	// Replay routes outside lock - use sendRoutes, not replayRoutes
	resent := 0
	for _, peer := range peersToResend {
		routes := routesToResend[peer]
		r.sendRoutes(peer, routes)
		resent += len(routes)
	}

	data, _ := json.Marshal(map[string]any{"resent": resent, "peers": len(peersToResend)})
	return string(data)
}

// sendRoutes sends routes to a peer without the "plugin session ready" signal.
// Used for manual resend operations. Includes full path attributes.
func (r *RIBManager) sendRoutes(peerAddr string, routes []*Route) {
	// Sort by MsgID to send in original announcement order
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].MsgID < routes[j].MsgID
	})

	for _, route := range routes {
		cmd := formatRouteCommand(route)
		r.updateRoute(peerAddr, cmd)
	}
}

// statusJSON returns status as JSON.
func (r *RIBManager) statusJSON() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routesIn := 0
	staleRoutes := 0
	for _, peerRIB := range r.ribInPool {
		routesIn += peerRIB.Len()
		staleRoutes += peerRIB.StaleCount()
	}

	routesOut := 0
	for _, routes := range r.ribOut {
		routesOut += len(routes)
	}

	result := map[string]any{
		"running":      true,
		"peers":        len(r.peerUp),
		"routes-in":    routesIn,
		"routes-out":   routesOut,
		"stale-routes": staleRoutes,
	}

	// Add per-peer GR state if any peers have stale routes.
	if len(r.grState) > 0 {
		grPeers := make(map[string]any, len(r.grState))
		for peer, state := range r.grState {
			grPeers[peer] = map[string]any{
				"stale-at":     state.StaleAt.Format(time.RFC3339),
				"restart-time": state.RestartTime,
				"expires-at":   state.ExpiresAt.Format(time.RFC3339),
			}
		}
		result["gr-state"] = grPeers
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// retainRoutesJSON marks a peer's Adj-RIB-In for retention during GR.
// RFC 4724: Receiving speaker retains routes from restarting peer.
// Called by bgp-gr plugin via DispatchCommand("rib retain-routes <peer>").
func (r *RIBManager) retainRoutesJSON(selector string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	retained := 0
	for peer := range r.ribInPool {
		if !matchesPeer(peer, selector) {
			continue
		}
		r.retainedPeers[peer] = true
		retained++
	}

	data, _ := json.Marshal(map[string]any{"retained-peers": retained})
	return string(data)
}

// releaseRoutesJSON clears the retain flag and deletes Adj-RIB-In for matching peers.
// RFC 4724: Called when restart timer expires or GR completes.
// Called by bgp-gr plugin via DispatchCommand("rib release-routes <peer>").
func (r *RIBManager) releaseRoutesJSON(selector string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	released := 0
	for peer := range r.retainedPeers {
		if !matchesPeer(peer, selector) {
			continue
		}
		delete(r.retainedPeers, peer)
		if peerRIB := r.ribInPool[peer]; peerRIB != nil {
			peerRIB.Release()
			delete(r.ribInPool, peer)
		}
		delete(r.peerMeta, peer)
		// Cancel expiry timer if present.
		if state := r.grState[peer]; state != nil && state.expiryTimer != nil {
			state.expiryTimer.Stop()
		}
		delete(r.grState, peer)
		released++
	}

	data, _ := json.Marshal(map[string]any{"released-peers": released})
	return string(data)
}

// markStaleCommand handles "rib mark-stale <peer> <restart-time>".
// Marks all routes for the peer as stale and stores GR metadata.
// RFC 4724 Section 4.2: mark routes stale on GR-capable peer session drop.
// Args: [0]=peer address, [1]=restart time in seconds, [2]=optional stale level (default 1).
func (r *RIBManager) markStaleCommand(args []string) (string, string, error) {
	if len(args) < 2 {
		return statusError, "", fmt.Errorf("mark-stale requires <peer> <restart-time> [level]")
	}

	peerAddr := args[0]
	restartSec, err := strconv.ParseUint(args[1], 10, 16)
	if err != nil {
		return statusError, "", fmt.Errorf("invalid restart-time %q: %w", args[1], err)
	}

	// Stale level: plugin-defined, defaults to 1. Level 0 is fresh (not stale)
	// and rejected to prevent accidental unstaling via a "mark-stale" command.
	staleLevel := uint8(1)
	if len(args) >= 3 {
		lvl, lvlErr := strconv.ParseUint(args[2], 10, 8)
		if lvlErr != nil {
			return statusError, "", fmt.Errorf("invalid stale level %q: %w", args[2], lvlErr)
		}
		if lvl == 0 {
			return statusError, "", fmt.Errorf("stale level must be > 0 (0 means fresh)")
		}
		staleLevel = uint8(lvl)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	marked := 0
	peerRIB := r.ribInPool[peerAddr]
	if peerRIB != nil {
		peerRIB.MarkAllStale(staleLevel)
		marked = peerRIB.StaleCount()
	}

	// Cancel existing expiry timer if consecutive restart.
	if existing := r.grState[peerAddr]; existing != nil && existing.expiryTimer != nil {
		existing.expiryTimer.Stop()
	}

	// Store GR state for status display and start expiry timer.
	now := time.Now()
	restartTime := uint16(restartSec)
	expiryDuration := time.Duration(restartTime)*time.Second + grTimerMargin
	state := &peerGRState{
		StaleAt:     now,
		RestartTime: restartTime,
		ExpiresAt:   now.Add(time.Duration(restartTime) * time.Second),
	}
	state.expiryTimer = time.AfterFunc(expiryDuration, func() {
		r.autoExpireStale(peerAddr, state)
	})
	r.grState[peerAddr] = state

	logger().Debug("mark-stale", "peer", peerAddr, "marked", marked, "restart-time", restartTime)

	data, _ := json.Marshal(map[string]any{"marked": marked})
	return statusDone, string(data), nil
}

// purgeStaleCommand handles "rib purge-stale <peer> [family]".
// Deletes only stale routes, optionally for a specific family.
// RFC 4724 Section 4.2: purge stale routes on EOR receipt or timer expiry.
// Args: [0]=peer address, [1]=optional family (e.g., "ipv4/unicast").
func (r *RIBManager) purgeStaleCommand(args []string) (string, string, error) {
	if len(args) < 1 {
		return statusError, "", fmt.Errorf("purge-stale requires <peer>")
	}

	peerAddr := args[0]
	familyFilter := ""
	if len(args) >= 2 {
		familyFilter = args[1]
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	purged := 0
	peerRIB := r.ribInPool[peerAddr]
	if peerRIB != nil {
		if familyFilter != "" {
			family, ok := parseFamily(familyFilter)
			if ok {
				purged = peerRIB.PurgeFamilyStale(family)
			}
		} else {
			purged = peerRIB.PurgeAllStale()
		}
	}

	// If no stale routes remain, stop expiry timer and clear GR state.
	if peerRIB != nil && peerRIB.StaleCount() == 0 {
		if state := r.grState[peerAddr]; state != nil && state.expiryTimer != nil {
			state.expiryTimer.Stop()
		}
		delete(r.grState, peerAddr)
	}

	logger().Debug("purge-stale", "peer", peerAddr, "purged", purged, "family", familyFilter)

	data, _ := json.Marshal(map[string]any{"purged": purged})
	return statusDone, string(data), nil
}

// bestPathStatusJSON returns summary statistics about the best-path computation.
func (r *RIBManager) bestPathStatusJSON() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalPeers := len(r.ribInPool)
	totalRoutes := 0
	for _, peerRIB := range r.ribInPool {
		totalRoutes += peerRIB.Len()
	}

	data, _ := json.Marshal(map[string]any{
		"running":        true,
		"peers-with-rib": totalPeers,
		"total-routes":   totalRoutes,
	})
	return string(data)
}

// gatherCandidates collects best-path candidates for a given (family, nlri) across all peers.
// Returns extracted Candidate structs ready for SelectBest.
// Caller must hold at least read lock.
func (r *RIBManager) gatherCandidates(family nlri.Family, nlriBytes []byte) []*Candidate {
	var candidates []*Candidate
	for peer, peerRIB := range r.ribInPool {
		entry, ok := peerRIB.Lookup(family, nlriBytes)
		if !ok || entry == nil {
			continue
		}
		c := r.extractCandidate(peer, entry)
		candidates = append(candidates, c)
	}
	return candidates
}

// extractCandidate builds a Candidate from a RouteEntry by reading pool handles.
// Extracts attribute values needed for RFC 4271 §9.1.2 comparison.
func (r *RIBManager) extractCandidate(peerAddr string, entry *storage.RouteEntry) *Candidate {
	c := &Candidate{
		PeerAddr:  peerAddr,
		LocalPref: 100, // RFC 4271 default
	}

	// Peer metadata for eBGP/iBGP detection.
	if meta := r.peerMeta[peerAddr]; meta != nil {
		c.PeerASN = meta.PeerASN
		c.LocalASN = meta.LocalASN
	}

	// LOCAL_PREF (type 5): 4 bytes, higher wins.
	if entry.HasLocalPref() {
		if data, err := pool.LocalPref.Get(entry.LocalPref); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				c.LocalPref = v
			}
		}
	}

	// AS_PATH (type 2): wire bytes, count length and extract first AS.
	if entry.HasASPath() {
		if data, err := pool.ASPath.Get(entry.ASPath); err == nil {
			c.ASPathLen = asPathLength(data)
			c.FirstAS = firstASInPath(data)
		}
	}

	// ORIGIN (type 1): 1 byte (0=IGP, 1=EGP, 2=INCOMPLETE).
	if entry.HasOrigin() {
		if data, err := pool.Origin.Get(entry.Origin); err == nil && len(data) > 0 {
			c.Origin = data[0]
		}
	}

	// MED (type 4): 4 bytes, lower wins.
	if entry.HasMED() {
		if data, err := pool.MED.Get(entry.MED); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				c.MED = v
			}
		}
	}

	// ORIGINATOR_ID (type 9): 4 bytes, used as Router ID tiebreak (RFC 4456).
	if entry.HasOriginatorID() {
		if data, err := pool.OriginatorID.Get(entry.OriginatorID); err == nil {
			c.OriginatorID = formatNextHop(data) // same 4-byte IP format
		}
	}

	// RFC 9494: LLGR-stale flag for best-path depreference.
	c.StaleLevel = entry.StaleLevel

	return c
}

// enterLLGRCommand handles "rib enter-llgr <peer> <family> <llst>".
// RFC 9494: Transition a family to LLGR period. For each stale route:
//   - Delete routes with NO_LLGR community (0xFFFF0007)
//   - Attach LLGR_STALE community (0xFFFF0006) to remaining stale routes
//   - Set LLGRStale=true for best-path depreference
//
// registerLLGRCommands registers the LLGR-specific RIB commands.
// Called from registerBuiltinCommands. Uses registerCommand for uniform
// collision detection.
func registerLLGRCommands() {
	cmds := []struct {
		name    string
		help    string
		handler CommandHandler
	}{
		{"rib enter-llgr", "Enter LLGR for peer family (attach LLGR_STALE, delete NO_LLGR)",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				return r.enterLLGRCommand(args)
			}},
		{"rib depreference-stale", "Raise stale level to depreference threshold",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				return r.depreferenceStaleCommand(args)
			}},
		{"rib readvertise-llgr-stale", "Resend routes from Adj-RIB-Out excluding source peer",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				return r.readvertiseLLGRStaleCommand(args)
			}},
	}
	for _, c := range cmds {
		if err := registerCommand(c.name, c.help, c.handler); err != nil {
			logger().Warn("LLGR command registration failed", "command", c.name, "error", err)
		}
	}
}

// Args: [0]=peer address, [1]=family (e.g., "ipv4/unicast"), [2]=LLST seconds.
func (r *RIBManager) enterLLGRCommand(args []string) (string, string, error) {
	if len(args) < 3 {
		return statusError, "", fmt.Errorf("enter-llgr requires <peer> <family> <llst>")
	}

	peerAddr := args[0]
	familyStr := args[1]
	llstSec, err := strconv.ParseUint(args[2], 10, 32)
	if err != nil {
		return statusError, "", fmt.Errorf("invalid llst %q: %w", args[2], err)
	}

	family, ok := parseFamily(familyStr)
	if !ok {
		return statusError, "", fmt.Errorf("invalid family %q", familyStr)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	peerRIB := r.ribInPool[peerAddr]
	if peerRIB == nil {
		data, _ := json.Marshal(map[string]any{"entered": 0, "deleted": 0})
		return statusDone, string(data), nil
	}

	entered := 0
	deleted := 0

	// Walk stale routes for the specified family.
	// Collect NLRIs to delete (NO_LLGR) separately to avoid modifying during iteration.
	var toDelete [][]byte

	peerRIB.IterateFamily(family, func(nlriBytes []byte, entry *storage.RouteEntry) bool {
		if entry.StaleLevel == storage.StaleLevelFresh {
			return true // skip non-stale routes
		}

		// Check for NO_LLGR community
		if entry.HasCommunities() {
			if data, getErr := pool.Communities.Get(entry.Communities); getErr == nil {
				if containsCommunity(data, communityNoLLGR) {
					// Copy NLRI bytes since they may be invalidated after delete
					nlriCopy := make([]byte, len(nlriBytes))
					copy(nlriCopy, nlriBytes)
					toDelete = append(toDelete, nlriCopy)
					return true
				}
			}
		}

		// Attach LLGR_STALE community; only depreference if attachment succeeded
		if r.attachLLGRStaleCommunity(entry) {
			entry.StaleLevel = storage.DepreferenceThreshold
			entered++
		}
		return true
	})

	// Delete NO_LLGR routes
	for _, nlriBytes := range toDelete {
		if peerRIB.Remove(family, nlriBytes) {
			deleted++
		}
	}

	logger().Debug("enter-llgr", "peer", peerAddr, "family", familyStr,
		"llst", llstSec, "entered", entered, "deleted", deleted)

	data, _ := json.Marshal(map[string]any{"entered": entered, "deleted": deleted})
	return statusDone, string(data), nil
}

// communityLLGRStale is the LLGR_STALE well-known community wire value (RFC 9494).
var communityLLGRStale = []byte{0xFF, 0xFF, 0x00, 0x06}

// communityNoLLGR is the NO_LLGR well-known community wire value (RFC 9494).
var communityNoLLGR = []byte{0xFF, 0xFF, 0x00, 0x07}

// containsCommunity checks if a community wire blob contains a specific 4-byte community.
func containsCommunity(data, community []byte) bool {
	if len(data)%4 != 0 || len(community) != 4 {
		return false
	}
	for i := 0; i+4 <= len(data); i += 4 {
		if data[i] == community[0] && data[i+1] == community[1] &&
			data[i+2] == community[2] && data[i+3] == community[3] {
			return true
		}
	}
	return false
}

// attachLLGRStaleCommunity appends LLGR_STALE to a route's community attribute.
// If no community attribute exists, creates one with just LLGR_STALE.
// Pool handles are updated: old handle released, new handle interned.
// Returns true if the community was successfully attached (or already present).
func (r *RIBManager) attachLLGRStaleCommunity(entry *storage.RouteEntry) bool {
	var newData []byte

	if entry.HasCommunities() {
		oldData, err := pool.Communities.Get(entry.Communities)
		if err != nil {
			return false
		}
		// Check if LLGR_STALE already present
		if containsCommunity(oldData, communityLLGRStale) {
			return true // already attached
		}
		newData = make([]byte, len(oldData)+4)
		copy(newData, oldData)
		copy(newData[len(oldData):], communityLLGRStale)
	} else {
		newData = make([]byte, 4)
		copy(newData, communityLLGRStale)
	}

	newHandle, err := pool.Communities.Intern(newData)
	if err != nil {
		return false
	}

	// Release old handle
	if entry.HasCommunities() {
		_ = pool.Communities.Release(entry.Communities)
	}
	entry.Communities = newHandle
	return true
}

// depreferenceStaleCommand handles "rib depreference-stale <peer>".
// RFC 9494: Mark all stale routes for a peer as LLGR-stale (least preferred in best-path).
// Args: [0]=peer address.
func (r *RIBManager) depreferenceStaleCommand(args []string) (string, string, error) {
	if len(args) < 1 {
		return statusError, "", fmt.Errorf("depreference-stale requires <peer>")
	}

	peerAddr := args[0]

	r.mu.Lock()
	defer r.mu.Unlock()

	peerRIB := r.ribInPool[peerAddr]
	if peerRIB == nil {
		data, _ := json.Marshal(map[string]any{"depreferenced": 0})
		return statusDone, string(data), nil
	}

	depreferenced := 0
	peerRIB.Iterate(func(_ nlri.Family, _ []byte, entry *storage.RouteEntry) bool {
		if entry.StaleLevel > storage.StaleLevelFresh && entry.StaleLevel < storage.DepreferenceThreshold {
			entry.StaleLevel = storage.DepreferenceThreshold
			depreferenced++
		}
		return true
	})

	logger().Debug("depreference-stale", "peer", peerAddr, "depreferenced", depreferenced)

	data, _ := json.Marshal(map[string]any{"depreferenced": depreferenced})
	return statusDone, string(data), nil
}

// readvertiseLLGRStaleCommand handles "rib readvertise-llgr-stale <source-peer>".
// RFC 9494: After entering LLGR, stale routes with LLGR_STALE community need
// re-advertisement so that downstream peers receive the updated community.
// Resends Adj-RIB-Out to all up peers except the source peer.
// The LLGR_STALE community is already attached to pool entries (by enter-llgr),
// so resent routes will include it in their attributes.
// Args: [0]=source peer address (whose routes entered LLGR).
func (r *RIBManager) readvertiseLLGRStaleCommand(args []string) (string, string, error) {
	if len(args) < 1 {
		return statusError, "", fmt.Errorf("readvertise-llgr-stale requires <source-peer>")
	}

	sourcePeer := args[0]

	// Resend ribOut to all up peers except the source.
	result := r.outboundResendJSON("!" + sourcePeer)

	logger().Debug("readvertise-llgr-stale", "source", sourcePeer, "result", result)

	return statusDone, result, nil
}
