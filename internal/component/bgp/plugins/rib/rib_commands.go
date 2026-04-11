// Design: docs/architecture/plugin/rib-storage-design.md — RIB command handlers
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_nlri.go — NLRI wire format helpers
// Related: rib_attr_format.go — attribute formatting for show enrichment
// Related: bestpath.go — best-path selection (extractCandidate, gatherCandidates, SelectBest)
// Related: rib_pipeline.go — iterator pipeline for show commands (scope, filters, terminals)
// Related: rib_pipeline_best.go — best-path pipeline (bestSource, bestPipeline, bestJSONTerminal)
package rib

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
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

// builtinsOnce guards against concurrent/double-registration of builtin commands.
var builtinsOnce sync.Once

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
// Idempotent via sync.Once (safe for concurrent calls from multiple plugin goroutines).
func registerBuiltinCommands() {
	builtinsOnce.Do(doRegisterBuiltinCommands)
}

func doRegisterBuiltinCommands() {
	builtins := []struct {
		names   []string
		help    string
		handler CommandHandler
	}{
		{[]string{"bgp rib status", "bgp rib adjacent status"}, "Show RIB status (peer count, route counts)",
			func(r *RIBManager, sel string, _ []string) (string, string, error) {
				return statusDone, r.statusJSON(), nil
			}},
		{[]string{"bgp rib show"}, "Show routes (scope: sent|received|sent-received, filters, terminals)",
			func(r *RIBManager, sel string, args []string) (string, string, error) {
				return statusDone, r.showPipeline(sel, args), nil
			}},
		{[]string{"bgp rib clear in", "bgp rib adjacent inbound empty"}, "Clear Adj-RIB-In routes",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				if len(args) == 0 {
					return statusError, "", fmt.Errorf("bgp rib clear in requires a selector (* for all peers)")
				}
				return statusDone, r.inboundEmptyJSON(args[0]), nil
			}},
		{[]string{"bgp rib clear out", "bgp rib adjacent outbound resend"}, "Resend Adj-RIB-Out routes",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				if len(args) == 0 {
					return statusError, "", fmt.Errorf("bgp rib clear out requires a selector (* for all peers)")
				}
				var family string
				if len(args) >= 2 && strings.Contains(args[1], "/") {
					family = args[1]
				}
				return statusDone, r.outboundResendJSON(args[0], family), nil
			}},
		{[]string{"bgp rib retain-routes"}, "Mark peer RIB for retention",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				if len(args) == 0 {
					return statusError, "", fmt.Errorf("bgp rib retain-routes requires a selector (* for all peers)")
				}
				return statusDone, r.retainRoutesJSON(args[0]), nil
			}},
		{[]string{"bgp rib release-routes"}, "Release retained peer RIB",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				if len(args) == 0 {
					return statusError, "", fmt.Errorf("bgp rib release-routes requires a selector (* for all peers)")
				}
				return statusDone, r.releaseRoutesJSON(args[0]), nil
			}},
		{[]string{"bgp rib mark-stale"}, "Mark peer routes at stale level",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				return r.markStaleCommand(args)
			}},
		{[]string{"bgp rib purge-stale"}, "Purge stale routes for peer",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				return r.purgeStaleCommand(args)
			}},
		{[]string{"bgp rib show best"}, "Show best-path per prefix (add 'reason' terminal to narrate the RFC 4271 §9.1.2 decision process)",
			func(r *RIBManager, sel string, args []string) (string, string, error) {
				return statusDone, r.bestPipeline(sel, args), nil
			}},
		{[]string{"bgp rib show best status"}, "Show best-path computation status",
			func(r *RIBManager, _ string, _ []string) (string, string, error) {
				return statusDone, r.bestPathStatusJSON(), nil
			}},
		{[]string{"bgp rib help"}, "Show RIB subcommands",
			func(_ *RIBManager, _ string, _ []string) (string, string, error) {
				return statusDone, ribHelpJSON(), nil
			}},
		{[]string{"bgp rib command list"}, "List RIB commands",
			func(_ *RIBManager, _ string, _ []string) (string, string, error) {
				return statusDone, ribCommandListJSON(), nil
			}},
		{[]string{"bgp rib event list"}, "List RIB event types",
			func(_ *RIBManager, _ string, _ []string) (string, string, error) {
				return statusDone, ribEventListJSON(), nil
			}},
		{[]string{"bgp rib inject"}, "Inject route into adj-rib-in: <peer> <family> <prefix> [origin <igp|egp|incomplete>] [nhop <ip>] [aspath <asn,asn,...>] [localpref <n>] [med <n>]",
			func(r *RIBManager, sel string, args []string) (string, string, error) {
				return r.injectRoute(sel, args)
			}},
		{[]string{"bgp rib withdraw"}, "Withdraw route from adj-rib-in: <peer> <family> <prefix>",
			func(r *RIBManager, sel string, args []string) (string, string, error) {
				return r.withdrawRoute(sel, args)
			}},
	}

	for _, b := range builtins {
		for _, name := range b.names {
			registeredCommands[name] = &ribCommandEntry{Handler: b.handler, Help: b.help}
		}
	}

	// Generic community manipulation commands. Plugins compose these
	// to implement protocol-specific behavior (e.g., GR/LLGR stale handling).
	registerCommunityCommands()
}

// injectRoute inserts a route into adj-rib-in as if received from a peer.
// Syntax: bgp rib inject <peer> <family> <prefix> [origin <igp|egp|incomplete>] [nhop <ip>] [aspath <asn,asn,...>] [localpref <n>] [med <n>]
// The peer address is a label; no live BGP session required.
func (r *RIBManager) injectRoute(_ string, args []string) (string, string, error) {
	if len(args) < 3 {
		return statusError, "", fmt.Errorf("usage: rib inject <peer> <family> <prefix> [origin <val>] [nhop <ip>] [aspath <asn,...>] [localpref <n>] [med <n>]")
	}

	peer := args[0]
	familyStr := args[1]
	prefix := args[2]

	// Validate peer looks like an IP address.
	if net.ParseIP(peer) == nil {
		return statusError, "", fmt.Errorf("invalid peer address: %s", peer)
	}

	// Validate family is a simple prefix type (IPv4/IPv6 unicast/multicast).
	fam, ok := parseFamily(familyStr)
	if !ok {
		return statusError, "", fmt.Errorf("unknown family: %s", familyStr)
	}
	if !isSimplePrefixFamily(fam) {
		return statusError, "", fmt.Errorf("bgp rib inject only supports simple prefix families (IPv4/IPv6 unicast/multicast), not %s", familyStr)
	}

	// Validate remaining args form complete key-value pairs.
	attrArgs := args[3:]
	if len(attrArgs)%2 != 0 {
		return statusError, "", fmt.Errorf("attribute %q has no value", attrArgs[len(attrArgs)-1])
	}

	// Parse optional attributes from remaining args.
	ab := attribute.NewBuilder()
	ab.SetOrigin(uint8(attribute.OriginIGP)) // default

	for i := 0; i < len(attrArgs); i += 2 {
		key := attrArgs[i]
		val := attrArgs[i+1]

		if key == "origin" {
			code, ok := injectOriginValues[val]
			if !ok {
				return statusError, "", fmt.Errorf("unknown origin: %s (use igp, egp, incomplete)", val)
			}
			ab.SetOrigin(code)
			continue
		}
		if key == "nhop" {
			ip := net.ParseIP(val)
			if ip == nil {
				return statusError, "", fmt.Errorf("invalid next-hop IP: %s", val)
			}
			if ip4 := ip.To4(); ip4 != nil {
				ab.SetNextHop([4]byte(ip4))
			} else if err := r.validateIPv6NextHop(peer, fam); err != nil {
				return statusError, "", err
			}
			// IPv6 nhop accepted but not stored in NEXT_HOP attr (type 3 is IPv4 only).
			// Route is injected without wire next-hop; shows in bgp rib show as-is.
			continue
		}
		if key == "aspath" {
			asns, err := parseASNList(val)
			if err != nil {
				return statusError, "", fmt.Errorf("invalid aspath: %w", err)
			}
			ab.SetASPath(asns)
			continue
		}
		if key == "localpref" {
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return statusError, "", fmt.Errorf("invalid localpref: %w", err)
			}
			ab.SetLocalPref(uint32(n))
			continue
		}
		if key == "med" {
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return statusError, "", fmt.Errorf("invalid med: %w", err)
			}
			ab.SetMED(uint32(n))
			continue
		}
		return statusError, "", fmt.Errorf("unknown attribute: %s", key)
	}

	attrBytes := ab.Build()

	nlriBytes, err := prefixToWire(familyStr, prefix, 0, false)
	if err != nil {
		return statusError, "", fmt.Errorf("invalid prefix: %w", err)
	}

	r.mu.Lock()
	if r.ribInPool[peer] == nil {
		r.ribInPool[peer] = storage.NewPeerRIB(peer)
	}
	r.ribInPool[peer].Insert(fam, attrBytes, nlriBytes)
	r.mu.Unlock()

	data, _ := json.Marshal(map[string]any{"injected": prefix, "peer": peer, "family": familyStr})
	return statusDone, string(data), nil
}

// validateIPv6NextHop checks whether an IPv6 next-hop is valid for this peer and family.
// Real peers (seen in peerMeta): check ExtendedNextHop capability (RFC 8950).
// Unknown peers (injected, no session): accept with a warning log.
func (r *RIBManager) validateIPv6NextHop(peer string, fam family.Family) error {
	meta := r.peerMeta[peer]
	if meta == nil {
		// Unknown peer (injected, no prior session). Accept any valid IP.
		logger().Warn("peer not known, accepting IPv6 next-hop without capability check", "peer", peer)
		return nil
	}

	if meta.ContextID == 0 {
		// Peer seen via JSON events (no structured event yet). Accept with warning.
		logger().Warn("peer has no encoding context, accepting IPv6 next-hop without capability check", "peer", peer)
		return nil
	}

	// RFC 8950 Section 4: check negotiated ExtendedNextHop for this family.
	ctx := bgpctx.Registry.Get(meta.ContextID)
	if ctx == nil {
		logger().Warn("encoding context not found, accepting IPv6 next-hop", "peer", peer, "context-id", meta.ContextID)
		return nil
	}

	if ctx.ExtendedNextHopFor(fam) == 0 {
		return fmt.Errorf("peer %s has not negotiated extended-nexthop (RFC 8950) for %s", peer, formatFamily(fam))
	}

	return nil
}

// withdrawRoute removes a route from adj-rib-in.
// Syntax: bgp rib withdraw <peer> <family> <prefix>
// The peer address is a label; no live BGP session required.
func (r *RIBManager) withdrawRoute(_ string, args []string) (string, string, error) {
	if len(args) < 3 {
		return statusError, "", fmt.Errorf("usage: rib withdraw <peer> <family> <prefix>")
	}

	peer := args[0]
	familyStr := args[1]
	prefix := args[2]

	if net.ParseIP(peer) == nil {
		return statusError, "", fmt.Errorf("invalid peer address: %s", peer)
	}

	nlriBytes, err := prefixToWire(familyStr, prefix, 0, false)
	if err != nil {
		return statusError, "", fmt.Errorf("invalid prefix: %w", err)
	}

	fam, ok := parseFamily(familyStr)
	if !ok {
		return statusError, "", fmt.Errorf("unknown family: %s", familyStr)
	}

	r.mu.RLock()
	peerRIB := r.ribInPool[peer]
	r.mu.RUnlock()

	if peerRIB == nil {
		return statusError, "", fmt.Errorf("no RIB for peer %s", peer)
	}

	removed := peerRIB.Remove(fam, nlriBytes)
	data, _ := json.Marshal(map[string]any{"withdrawn": prefix, "peer": peer, "family": familyStr, "existed": removed})
	return statusDone, string(data), nil
}

// parseASNList parses a comma-separated list of ASNs into uint32 slice.
func parseASNList(s string) ([]uint32, error) {
	parts := strings.Split(s, ",")
	asns := make([]uint32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid ASN %q: %w", p, err)
		}
		asns = append(asns, uint32(n))
	}
	return asns, nil
}

// injectOriginValues maps origin text to wire code for rib inject.
var injectOriginValues = map[string]uint8{
	"igp":        uint8(attribute.OriginIGP),
	"egp":        uint8(attribute.OriginEGP),
	"incomplete": uint8(attribute.OriginIncomplete),
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
		after, ok := strings.CutPrefix(name, "bgp rib ")
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
// If family is non-empty, only routes from that family are resent.
// Does NOT send "plugin session ready" - that's only for initial reconnect.
func (r *RIBManager) outboundResendJSON(selector, family string) string {
	r.mu.RLock()
	var peersToResend []string
	routesToResend := make(map[string][]*Route)

	for peer, peerFamilies := range r.ribOut {
		if !matchesPeer(peer, selector) {
			continue
		}
		if !r.peerUp[peer] {
			continue // Only resend to up peers
		}
		var routesCopy []*Route
		if family != "" {
			// Single family resend
			for _, rt := range peerFamilies[family] {
				routesCopy = append(routesCopy, rt)
			}
		} else {
			// All families
			for _, familyRoutes := range peerFamilies {
				for _, rt := range familyRoutes {
					routesCopy = append(routesCopy, rt)
				}
			}
		}
		if len(routesCopy) > 0 {
			peersToResend = append(peersToResend, peer)
			routesToResend[peer] = routesCopy
		}
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
// RFC 9494: stale routes carry meta["stale"] so egress filters can suppress or modify.
func (r *RIBManager) sendRoutes(peerAddr string, routes []*Route) {
	// Sort by MsgID to send in original announcement order
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].MsgID < routes[j].MsgID
	})

	for _, route := range routes {
		cmd := formatRouteCommand(route)
		if route.StaleLevel > 0 {
			r.updateRouteWithMeta(peerAddr, cmd, map[string]any{"stale": route.StaleLevel})
		} else {
			r.updateRoute(peerAddr, cmd)
		}
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
	for _, peerFamilies := range r.ribOut {
		for _, familyRoutes := range peerFamilies {
			routesOut += len(familyRoutes)
		}
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
// Called by bgp-gr plugin via DispatchCommand("bgp rib retain-routes <peer>").
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
// Called by bgp-gr plugin via DispatchCommand("bgp rib release-routes <peer>").
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

// markStaleCommand handles "bgp rib mark-stale <peer> <restart-time>".
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

	// RFC 9494: Propagate stale level to ribOut routes for all destination peers.
	// During LLGR readvertisement, sendRoutes carries meta["stale"] to egress filters.
	for _, peerFamilies := range r.ribOut {
		for _, familyRoutes := range peerFamilies {
			for _, route := range familyRoutes {
				route.StaleLevel = staleLevel
			}
		}
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

// purgeStaleCommand handles "bgp rib purge-stale <peer> [family]".
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
			fam, ok := parseFamily(familyFilter)
			if ok {
				purged = peerRIB.PurgeFamilyStale(fam)
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
func (r *RIBManager) gatherCandidates(fam family.Family, nlriBytes []byte) []*Candidate {
	var candidates []*Candidate
	for peer, peerRIB := range r.ribInPool {
		entry, ok := peerRIB.Lookup(fam, nlriBytes)
		if !ok {
			continue
		}
		c := r.extractCandidate(peer, entry)
		candidates = append(candidates, c)
	}
	return candidates
}

// extractCandidate builds a Candidate from a RouteEntry by reading pool handles.
// Extracts attribute values needed for RFC 4271 §9.1.2 comparison.
func (r *RIBManager) extractCandidate(peerAddr string, entry storage.RouteEntry) *Candidate {
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
	// ASPathHandle is also stashed: because the attribute pool deduplicates
	// identical byte sequences to the same handle, two candidates with
	// byte-equal AS_PATHs share a handle and SelectMultipath can compare
	// them in O(1) without re-reading the underlying bytes.
	if entry.HasASPath() {
		c.ASPathHandle = entry.ASPath
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

// registerCommunityCommands registers generic community manipulation commands.
// Plugins compose these to implement protocol-specific behavior.
func registerCommunityCommands() {
	cmds := []struct {
		name    string
		help    string
		handler CommandHandler
	}{
		{"bgp rib attach-community", "Attach a community to stale routes for a peer family",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				return r.attachCommunityCommand(args)
			}},
		{"bgp rib delete-with-community", "Delete stale routes that have a specific community",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				return r.deleteWithCommunityCommand(args)
			}},
	}
	for _, c := range cmds {
		if err := registerCommand(c.name, c.help, c.handler); err != nil {
			logger().Warn("community command registration failed", "command", c.name, "error", err)
		}
	}
}

// attachCommunityCommand handles "bgp rib attach-community <peer> <family> <community-hex>".
// Attaches a 4-byte community to all stale routes for the specified peer and family.
// Also raises StaleLevel to DepreferenceThreshold for attached routes.
// Args: [0]=peer, [1]=family, [2]=community as 8-char hex (e.g., "ffff0006").
func (r *RIBManager) attachCommunityCommand(args []string) (string, string, error) {
	if len(args) < 3 {
		return statusError, "", fmt.Errorf("attach-community requires <peer> <family> <community-hex>")
	}

	peerAddr := args[0]
	familyStr := args[1]
	commHex := args[2]

	fam, ok := parseFamily(familyStr)
	if !ok {
		return statusError, "", fmt.Errorf("invalid family %q", familyStr)
	}

	commBytes, err := hex.DecodeString(commHex)
	if err != nil || len(commBytes) != 4 {
		return statusError, "", fmt.Errorf("invalid community hex %q (must be 8 hex chars = 4 bytes)", commHex)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	peerRIB := r.ribInPool[peerAddr]
	if peerRIB == nil {
		data, _ := json.Marshal(map[string]any{"attached": 0})
		return statusDone, string(data), nil
	}

	attached := 0
	peerRIB.ModifyFamilyAll(fam, func(entry *storage.RouteEntry) {
		if entry.StaleLevel == storage.StaleLevelFresh {
			return
		}
		if r.attachCommunity(entry, commBytes) {
			entry.StaleLevel = storage.DepreferenceThreshold
			attached++
		}
	})

	logger().Debug("attach-community", "peer", peerAddr, "family", familyStr,
		"community", commHex, "attached", attached)

	data, _ := json.Marshal(map[string]any{"attached": attached})
	return statusDone, string(data), nil
}

// deleteWithCommunityCommand handles "bgp rib delete-with-community <peer> <family> <community-hex>".
// Deletes stale routes that contain the specified community.
// Args: [0]=peer, [1]=family, [2]=community as 8-char hex.
func (r *RIBManager) deleteWithCommunityCommand(args []string) (string, string, error) {
	if len(args) < 3 {
		return statusError, "", fmt.Errorf("delete-with-community requires <peer> <family> <community-hex>")
	}

	peerAddr := args[0]
	familyStr := args[1]
	commHex := args[2]

	fam, ok := parseFamily(familyStr)
	if !ok {
		return statusError, "", fmt.Errorf("invalid family %q", familyStr)
	}

	commBytes, err := hex.DecodeString(commHex)
	if err != nil || len(commBytes) != 4 {
		return statusError, "", fmt.Errorf("invalid community hex %q (must be 8 hex chars = 4 bytes)", commHex)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	peerRIB := r.ribInPool[peerAddr]
	if peerRIB == nil {
		data, _ := json.Marshal(map[string]any{"deleted": 0})
		return statusDone, string(data), nil
	}

	// Collect NLRIs to delete (avoid modifying during iteration)
	var toDelete [][]byte
	peerRIB.IterateFamily(fam, func(nlriBytes []byte, entry storage.RouteEntry) bool {
		if entry.StaleLevel == storage.StaleLevelFresh {
			return true
		}
		if entry.HasCommunities() {
			if data, getErr := pool.Communities.Get(entry.Communities); getErr == nil {
				if containsCommunity(data, commBytes) {
					nlriCopy := make([]byte, len(nlriBytes))
					copy(nlriCopy, nlriBytes)
					toDelete = append(toDelete, nlriCopy)
				}
			}
		}
		return true
	})

	deleted := 0
	for _, nlriBytes := range toDelete {
		if peerRIB.Remove(fam, nlriBytes) {
			deleted++
		}
	}

	logger().Debug("delete-with-community", "peer", peerAddr, "family", familyStr,
		"community", commHex, "deleted", deleted)

	data, _ := json.Marshal(map[string]any{"deleted": deleted})
	return statusDone, string(data), nil
}

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

// attachCommunity appends a 4-byte community to a route's community attribute.
// If no community attribute exists, creates one. Idempotent: skips if already present.
// Pool handles are updated: old handle released, new handle interned.
// Returns true on success (or already present).
func (r *RIBManager) attachCommunity(entry *storage.RouteEntry, comm []byte) bool {
	var newData []byte

	if entry.HasCommunities() {
		oldData, err := pool.Communities.Get(entry.Communities)
		if err != nil {
			return false
		}
		if containsCommunity(oldData, comm) {
			return true
		}
		newData = make([]byte, len(oldData)+4)
		copy(newData, oldData)
		copy(newData[len(oldData):], comm)
	} else {
		newData = make([]byte, 4)
		copy(newData, comm)
	}

	newHandle, err := pool.Communities.Intern(newData)
	if err != nil {
		return false
	}

	if entry.HasCommunities() {
		_ = pool.Communities.Release(entry.Communities)
	}
	entry.Communities = newHandle
	return true
}
