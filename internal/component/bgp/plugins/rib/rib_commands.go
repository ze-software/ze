// Design: docs/architecture/plugin/rib-storage-design.md — RIB command handlers
// RFC: rfc/short/rfc4724.md — Graceful Restart (mark-stale, purge-stale, retain/release)
// RFC: rfc/short/rfc9494.md — LLGR stale propagation to Adj-RIB-Out
// RFC: rfc/short/rfc8950.md — IPv6 next-hop validation for injected routes
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_nlri.go — NLRI wire format helpers
// Related: rib_attr_format.go — attribute formatting for show enrichment
// Related: bestpath.go — best-path selection (extractCandidate, gatherCandidates, SelectBest)
// Related: rib_commands_community.go — community attach/delete operations
// Related: rib_pipeline.go — iterator pipeline for show commands (scope, filters, terminals)
// Related: rib_pipeline_best.go — best-path pipeline (bestSource, bestPipeline, bestJSONTerminal)
package rib

import (
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/stringsx"
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
func (r *RIBManager) autoExpireStale(peerAddr netip.Addr, owner *peerGRState) {
	type staleNLRI struct {
		fam     family.Family
		nlri    []byte
		addPath bool
	}
	var affected []staleNLRI

	r.peerMu.Lock()

	// Guard: skip if grState was replaced by a consecutive restart.
	if r.grState[peerAddr] != owner {
		r.peerMu.Unlock()
		return
	}

	peerRIB := r.bgpPeers[peerAddr]
	if peerRIB != nil {
		for _, fam := range peerRIB.Families() {
			ap := peerRIB.IsAddPath(fam)
			peerRIB.IterateFamily(fam, func(nlriBytes []byte, entry storage.RouteEntry) bool {
				if entry.StaleLevel > storage.StaleLevelFresh {
					cp := make([]byte, len(nlriBytes))
					copy(cp, nlriBytes)
					affected = append(affected, staleNLRI{fam: fam, nlri: cp, addPath: ap})
				}
				return true
			})
		}
		purged := peerRIB.PurgeAllStale()
		logger().Info("auto-expire stale", "peer", peerAddr, "purged", purged)
	}

	delete(r.grState, peerAddr)
	r.peerMu.Unlock()

	for _, a := range affected {
		change, ok := r.checkBestPathChange(a.fam, a.nlri, a.addPath, nil)
		if ok {
			publishBestChanges([]bestChangeEntry{change}, a.fam)
		}
	}
}

// CommandHandler is the signature for RIB command handlers.
// Registered by plugins via RegisterRIBCommand during init().
type CommandHandler func(r *RIBManager, selector string, args []string) (string, any, error)

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
		{[]string{"show bgp rib status"}, "Show RIB status (peer count, route counts)",
			func(r *RIBManager, sel string, args []string) (string, any, error) {
				// Optional first arg scopes the per-peer route-counts to one
				// family (summary.go passes its family filter here). No arg =
				// all-family totals.
				fam := ""
				if len(args) > 0 {
					fam = args[0]
				}
				return statusDone, r.status(fam), nil
			}},
		{[]string{"show bgp rib"}, "Show routes (scope: sent|received|sent-received, filters, terminals)",
			func(r *RIBManager, sel string, args []string) (string, any, error) {
				return statusDone, r.showPipeline(sel, args), nil
			}},
		{[]string{"clear bgp rib in"}, "Clear Adj-RIB-In routes",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				if len(args) == 0 {
					return statusError, "", errBgpRibClearInRequiresA
				}
				return statusDone, r.inboundEmpty(args[0]), nil
			}},
		{[]string{"clear bgp rib out"}, "Resend Adj-RIB-Out routes",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				if len(args) == 0 {
					return statusError, "", errBgpRibClearOutRequiresA
				}
				var family string
				if len(args) >= 2 && strings.Contains(args[1], "/") {
					family = args[1]
				}
				return statusDone, r.outboundResend(args[0], family), nil
			}},
		{[]string{"request bgp rib retain-routes"}, "Mark peer RIB for retention",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				if len(args) == 0 {
					return statusError, "", errBgpRibRetainRoutesRequiresA
				}
				return statusDone, r.retainRoutes(args[0]), nil
			}},
		{[]string{"request bgp rib release-routes"}, "Release retained peer RIB",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				if len(args) == 0 {
					return statusError, "", errBgpRibReleaseRoutesRequiresA
				}
				return statusDone, r.releaseRoutes(args[0]), nil
			}},
		{[]string{"request bgp rib mark-stale"}, "Mark peer routes at stale level",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				return r.markStaleCommand(args)
			}},
		{[]string{"request bgp rib purge-stale"}, "Purge stale routes for peer",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				return r.purgeStaleCommand(args)
			}},
		{[]string{"show bgp rib best"}, "Show best-path per prefix (add 'reason' terminal to narrate the RFC 4271 §9.1.2 decision process)",
			func(r *RIBManager, sel string, args []string) (string, any, error) {
				return statusDone, r.bestPipeline(sel, args), nil
			}},
		{[]string{"show bgp rib best status"}, "Show best-path computation status",
			func(r *RIBManager, _ string, _ []string) (string, any, error) {
				return statusDone, r.bestPathStatus(), nil
			}},
		{[]string{"show bgp rib help"}, "Show RIB subcommands",
			func(_ *RIBManager, _ string, _ []string) (string, any, error) {
				return statusDone, ribHelp(), nil
			}},
		{[]string{"show bgp rib commands"}, "List RIB commands",
			func(_ *RIBManager, _ string, _ []string) (string, any, error) {
				return statusDone, ribCommandList(), nil
			}},
		{[]string{"show bgp rib events"}, "List RIB event types",
			func(_ *RIBManager, _ string, _ []string) (string, any, error) {
				return statusDone, ribEventList(), nil
			}},
		{[]string{"request bgp rib inject"}, "Inject route into adj-rib-in: <peer> <family> <prefix> [origin <igp|egp|incomplete>] [nhop|nexthop <ip>] [aspath <asn,asn,...>] [localpref <n>] [med <n>]",
			func(r *RIBManager, sel string, args []string) (string, any, error) {
				return r.injectRoute(sel, args)
			}},
		{[]string{"request bgp rib withdraw"}, "Withdraw route from adj-rib-in: <peer> <family> <prefix>",
			func(r *RIBManager, sel string, args []string) (string, any, error) {
				return r.withdrawRoute(sel, args)
			}},
		{[]string{"show bgp rib rpf"}, "RPF lookup: <family> <source-addr> (longest-prefix-match in Loc-RIB)",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				return r.rpfLookup(args)
			}},
		{[]string{"request bgp rib fastpath"}, "Enable/disable/report the zero-copy forward-handle fast path (rib-arch-6): <enable|disable|status>",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				return r.fastpathCommand(args)
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

	registerInjectCommands()
}

// injectRoute inserts a route into adj-rib-in as if received from a peer.
// Syntax: request bgp rib inject <peer> <family> <prefix> [origin <igp|egp|incomplete>] [nhop|nexthop <ip>] [aspath <asn,asn,...>] [localpref <n>] [med <n>]
// The peer address is a label; no live BGP session required.
func (r *RIBManager) injectRoute(_ string, args []string) (string, any, error) {
	if len(args) < 3 {
		return statusError, "", errUsageRibInjectPeerFamilyPrefix
	}

	familyStr := args[1]
	prefix := args[2]

	// Parse the peer address once at the command boundary.
	peer, err := netip.ParseAddr(args[0])
	if err != nil {
		return statusError, "", fmt.Errorf("bgp rib inject: invalid peer address %q: %w (expected an IP address)", args[0], err)
	}

	// Validate family is a simple prefix type (IPv4/IPv6 unicast/multicast).
	fam, ok := parseFamily(familyStr)
	if !ok {
		return statusError, "", fmt.Errorf("unknown family: %s", familyStr)
	}
	if !isSimplePrefixFamily(fam) {
		return statusError, "", fmt.Errorf("request bgp rib inject only supports simple prefix families (IPv4/IPv6 unicast/multicast), not %s", familyStr)
	}

	// Validate remaining args form complete key-value pairs.
	attrArgs := args[3:]
	if len(attrArgs)%2 != 0 {
		return statusError, "", fmt.Errorf("attribute %q has no value", attrArgs[len(attrArgs)-1])
	}

	// Parse optional attributes from remaining args.
	ab := attribute.NewBuilder()
	ab.SetOrigin(uint8(attribute.OriginIGP)) // default

	// extNextHop holds an IPv6 next-hop that must be carried in MP_REACH_NLRI
	// (the legacy NEXT_HOP attribute is IPv4-only). Set below when the operator
	// supplies an IPv6 next-hop; emitted as an MP_REACH attribute after the loop.
	var extNextHop netip.Addr

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
		if key == "nhop" || key == "nexthop" {
			nhAddr, err := netip.ParseAddr(val)
			if err != nil {
				return statusError, "", fmt.Errorf("invalid next-hop IP: %s", val)
			}
			nhAddr = nhAddr.Unmap()
			if nhAddr.Is4() {
				// Legacy NEXT_HOP attribute (type 3, IPv4 only).
				ab.SetNextHop(nhAddr.As4())
			} else {
				// IPv6 next-hop. RFC 5549/8950: an IPv4 NLRI reachable via an IPv6
				// next-hop is a cross-family extended next-hop and requires the peer
				// to have negotiated extended-nexthop. A native IPv6 NLRI with an
				// IPv6 next-hop is ordinary MP-BGP and needs no such capability.
				// Either way the next-hop can only live in MP_REACH_NLRI (NEXT_HOP
				// type 3 is IPv4-only), so record it and emit MP_REACH below.
				if fam.AFI == family.AFIIPv4 {
					if err := r.validateIPv6NextHop(peer, fam); err != nil {
						return statusError, "", err
					}
				}
				extNextHop = nhAddr
			}
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

	// RFC 5549 / RFC 8950: an IPv6 next-hop (for an IPv4 NLRI -- extended next-hop
	// -- or a native IPv6 NLRI) is carried in MP_REACH_NLRI, not the IPv4-only
	// NEXT_HOP attribute. Mirror the receive path (rib_structured.go): store the
	// MP_REACH inside the attribute block and keep nlriBytes as the separate
	// storage key. On readback extractMPNextHopAddr recovers the IPv6 next-hop and
	// the forward encoder (commit.go useTraditionalNLRI -> buildMPReachNLRI)
	// re-emits it as an RFC 5549/8950 extended next-hop.
	if extNextHop.IsValid() {
		mpReach := attribute.NewMPReachNLRI(attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI), []netip.Addr{extNextHop}, nlriBytes)
		mpBuf := make([]byte, 4+mpReach.Len())
		n := attribute.WriteAttrTo(mpReach, mpBuf, 0)
		combined := make([]byte, 0, len(attrBytes)+n)
		combined = append(combined, attrBytes...)
		combined = append(combined, mpBuf[:n]...)
		attrBytes = combined
	}

	r.peerMu.Lock()
	if r.bgpPeers[peer] == nil {
		// Canonical string: PeerRIB.PeerAddr() feeds the best-path interner
		// and metric labels, which must match netip.Addr.String() everywhere.
		r.bgpPeers[peer] = storage.NewPeerRIB(peer.String())
	}
	r.bgpPeers[peer].Insert(fam, attrBytes, nlriBytes, true)
	r.peerMu.Unlock()

	r.reconcileBestPath(fam, nlriBytes)

	return statusDone, map[string]any{"injected": prefix, "peer": args[0], "family": familyStr}, nil
}

// validateIPv6NextHop checks whether an IPv6 next-hop is valid for this peer and family.
// Real peers (seen in peerMeta): check ExtendedNextHop capability (RFC 8950).
// Unknown peers (injected, no session): accept with a warning log.
//
// Acquires r.peerMu.RLock for the brief peerMeta read.
func (r *RIBManager) validateIPv6NextHop(peer netip.Addr, fam family.Family) error {
	r.peerMu.RLock()
	meta := r.peerMeta[peer]
	r.peerMu.RUnlock()
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
// Syntax: request bgp rib withdraw <peer> <family> <prefix>
// The peer address is a label; no live BGP session required.
func (r *RIBManager) withdrawRoute(_ string, args []string) (string, any, error) {
	if len(args) < 3 {
		return statusError, "", errUsageRibWithdrawPeerFamilyPrefix
	}

	familyStr := args[1]
	prefix := args[2]

	peer, err := netip.ParseAddr(args[0])
	if err != nil {
		return statusError, "", fmt.Errorf("bgp rib withdraw: invalid peer address %q: %w (expected an IP address)", args[0], err)
	}

	nlriBytes, err := prefixToWire(familyStr, prefix, 0, false)
	if err != nil {
		return statusError, "", fmt.Errorf("invalid prefix: %w", err)
	}

	fam, ok := parseFamily(familyStr)
	if !ok {
		return statusError, "", fmt.Errorf("unknown family: %s", familyStr)
	}

	r.peerMu.RLock()
	peerRIB := r.bgpPeers[peer]
	r.peerMu.RUnlock()

	if peerRIB == nil {
		return statusError, "", fmt.Errorf("no RIB for peer %s", peer)
	}

	removed := peerRIB.Remove(fam, nlriBytes)

	r.reconcileBestPath(fam, nlriBytes)

	return statusDone, map[string]any{"withdrawn": prefix, "peer": args[0], "family": familyStr, "existed": removed}, nil
}

// rpfLookup performs a Reverse Path Forwarding lookup: longest-prefix-match
// against the Loc-RIB for a given family and source address.
func (r *RIBManager) rpfLookup(args []string) (string, any, error) {
	if len(args) < 2 {
		return statusError, "", fmt.Errorf("usage: bgp rib rpf <family> <source-addr>")
	}

	familyStr := args[0]
	addrStr := args[1]

	fam, ok := parseFamily(familyStr)
	if !ok {
		return statusError, "", fmt.Errorf("unknown family: %s", familyStr)
	}
	if !isSimplePrefixFamily(fam) {
		return statusError, "", fmt.Errorf("rpf only supports CIDR families (IPv4/IPv6 unicast/multicast), not %s", familyStr)
	}

	addr, err := netip.ParseAddr(addrStr)
	if err != nil {
		return statusError, "", fmt.Errorf("invalid source address: %s", addrStr)
	}

	r.peerMu.RLock()
	loc := r.locRIB
	r.peerMu.RUnlock()

	if loc == nil {
		return statusError, "", fmt.Errorf("loc-rib not available")
	}

	best, pfx, found := loc.LPM(fam, addr)
	if !found {
		return statusDone, map[string]any{
			"source": addrStr,
			"family": familyStr,
			"found":  false,
		}, nil
	}

	nextHop := ""
	if best.NextHop.IsValid() {
		nextHop = best.NextHop.String()
	}
	return statusDone, map[string]any{
		"source":         addrStr,
		"family":         familyStr,
		"found":          true,
		"matched-prefix": pfx.String(),
		"next-hop":       nextHop,
		"admin-distance": best.AdminDistance,
		"metric":         best.Metric,
	}, nil
}

// parseASNList parses a comma-separated list of ASNs into uint32 slice.
func parseASNList(s string) ([]uint32, error) {
	parts, count := stringsx.SplitCount(s, ",")
	asns := make([]uint32, 0, count)
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
func (r *RIBManager) handleCommand(command, selector string, args []string) (string, any, error) {
	if entry, ok := registeredCommands[command]; ok {
		return entry.Handler(r, selector, args)
	}
	return statusError, "", fmt.Errorf("unknown command: %s", command)
}

// ribHelp returns RIB subcommands, built from the command registry.
func ribHelp() any {
	seen := make(map[string]bool)
	var subs []string
	for name := range registeredCommands {
		// Strip any verb prefix to find the "bgp rib" subcommands.
		for _, prefix := range []string{"show bgp rib ", "clear bgp rib ", "request bgp rib "} {
			after, ok := strings.CutPrefix(name, prefix)
			if !ok {
				continue
			}
			parts := strings.SplitN(after, " ", 2)
			if len(parts) > 0 && !seen[parts[0]] {
				subs = append(subs, parts[0])
				seen[parts[0]] = true
			}
		}
	}
	sort.Strings(subs)
	return map[string]any{"subcommands": subs}
}

// ribCommandList returns all RIB commands, built from the command registry.
func ribCommandList() any {
	type entry struct {
		Name string `json:"name"`
		Help string `json:"help"`
	}
	cmds := make([]entry, 0, len(registeredCommands))
	for name, e := range registeredCommands {
		cmds = append(cmds, entry{Name: name, Help: e.Help})
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	return map[string]any{"commands": cmds}
}

// ribEventList returns RIB event types.
func ribEventList() any {
	events := []string{"cache", "route", "peer", "memory"}
	return map[string]any{"events": events}
}

// inboundEmpty clears Adj-RIB-In routes for matching peers.
func (r *RIBManager) inboundEmpty(selectorStr string) any {
	sel := selector.ParseDefault(selectorStr)
	r.peerMu.Lock()
	cleared := 0
	var purgedPeers []netip.Addr

	for peer, peerRIB := range r.bgpPeers {
		if !sel.Matches(peer) {
			continue
		}
		cleared += peerRIB.Len()
		peerRIB.Release()
		delete(r.bgpPeers, peer)
		delete(r.peerMeta, peer)
		purgedPeers = append(purgedPeers, peer)
	}
	r.peerMu.Unlock()

	r.reconcileBestPathBulk(purgedPeers)

	return map[string]any{"cleared": cleared}
}

// outboundResend replays Adj-RIB-Out routes for matching peers.
// If family is non-empty, only routes from that family are resent.
// Does NOT send "plugin session ready" - that's only for initial reconnect.
// Uses cursor mode for efficient batched replay with delta encoding.
func (r *RIBManager) outboundResend(selectorStr, famStr string) any {
	sel := selector.ParseDefault(selectorStr)
	r.peerMu.RLock()
	var peersToResend []netip.Addr
	groupsToResend := make(map[netip.Addr][]replayGroup)

	for peer := range r.ribOut {
		if !sel.Matches(peer) {
			continue
		}
		if !r.peerUp[peer] {
			continue // Only resend to up peers
		}
		var groups []replayGroup
		if famStr != "" {
			if fam, ok := family.LookupFamily(famStr); ok {
				groups = r.collectGroupedRibOutRoutesForFamily(peer, fam)
			}
		} else {
			groups = r.collectGroupedRibOutRoutes(peer)
		}
		if len(groups) > 0 {
			peersToResend = append(peersToResend, peer)
			groupsToResend[peer] = groups
		}
	}
	r.peerMu.RUnlock()

	resent := 0
	for _, peer := range peersToResend {
		// RPC selector is a string boundary: one conversion per resent peer.
		resent += r.resendRoutesWithCursor(peer.String(), groupsToResend[peer])
	}

	return map[string]any{"resent": resent, "peers": len(peersToResend)}
}

// sendRoutes sends routes to a peer without the "plugin session ready" signal.
// Used by RFC 7313 route refresh (BoRR/EoRR). Includes full path attributes.
// RFC 9494: stale routes carry meta["stale"] so egress filters can suppress or modify.
func (r *RIBManager) sendRoutes(peerAddr string, routes []*Route) {
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

// status returns RIB status.
func (r *RIBManager) status(famFilter string) any {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()

	// Per-peer Adj-RIB-In / Adj-RIB-Out sizes, keyed by peer address. summary.go
	// merges these into `show bgp summary` (routes-received/accepted from "in",
	// routes-sent from "out") via ForwardToPlugin, so the birdwatcher LG can show
	// per-peer route counts without cmd/peer importing this plugin. Only BGP
	// peers (bgpPeers/ribOut, keyed by netip.Addr) are per-peer; ribInPool holds
	// non-BGP redistribution sources that do not map to a BGP peer row.
	// RFC 4271 Section 3.2: Adj-RIB-In holds routes advertised by a peer,
	// Adj-RIB-Out holds routes advertised to a peer.
	//
	// famFilter scopes the per-peer counts to one family (so a family-filtered
	// `show bgp summary <afi/safi>` reports family-scoped, not all-family,
	// counts). Empty famFilter, or an unrecognized one, reports all-family
	// totals. The global routes-in/routes-out stay all-family totals regardless,
	// since other consumers depend on them.
	scopeFam, scoped := family.Family{}, false
	if famFilter != "" {
		scopeFam, scoped = family.LookupFamily(famFilter)
	}
	// A peer with routes only in other families gets a {in:0,out:0} entry under
	// a family filter. That is intentional, not spurious: a peer that appears in
	// a family-filtered `show bgp summary <fam>` (it negotiated <fam>) but holds
	// no <fam> routes should report 0, a real count, not an omitted key. Entries
	// for peers absent from the summary are simply never merged. Bounded by peer
	// count either way.
	peerCounts := make(map[string]any, len(r.bgpPeers))
	perPeer := func(addr netip.Addr) map[string]any {
		key := addr.String()
		if m, ok := peerCounts[key].(map[string]any); ok {
			return m
		}
		m := map[string]any{"in": 0, "out": 0}
		peerCounts[key] = m
		return m
	}

	routesIn := 0
	staleRoutes := 0
	for addr, peerRIB := range r.bgpPeers {
		peerIn := peerRIB.Len()
		routesIn += peerIn
		staleRoutes += peerRIB.StaleCount()
		if scoped {
			perPeer(addr)["in"] = peerRIB.FamilyLen(scopeFam)
		} else {
			perPeer(addr)["in"] = peerIn
		}
	}
	for _, protoPeers := range r.ribInPool {
		for _, peerRIB := range protoPeers {
			routesIn += peerRIB.Len()
			staleRoutes += peerRIB.StaleCount()
		}
	}

	routesOut := 0
	for addr, peerFamilies := range r.ribOut {
		peerOut := 0
		for fam, familyRoutes := range peerFamilies {
			routesOut += len(familyRoutes)
			if scoped {
				if fam == scopeFam {
					peerOut = len(familyRoutes)
				}
			} else {
				peerOut += len(familyRoutes)
			}
		}
		perPeer(addr)["out"] = peerOut
	}

	result := map[string]any{
		"running":      true,
		"peers":        len(r.peerUp),
		"routes-in":    routesIn,
		"routes-out":   routesOut,
		"stale-routes": staleRoutes,
		"route-counts": peerCounts,
	}

	// Add per-peer GR state if any peers have stale routes.
	if len(r.grState) > 0 {
		grPeers := make(map[string]any, len(r.grState))
		for peer, state := range r.grState {
			grPeers[peer.String()] = map[string]any{
				"stale-at":     state.StaleAt.Format(time.RFC3339),
				"restart-time": state.RestartTime,
				"expires-at":   state.ExpiresAt.Format(time.RFC3339),
			}
		}
		result["gr-state"] = grPeers
	}

	return result
}

// retainRoutes marks a peer's Adj-RIB-In for retention during GR.
// RFC 4724: Receiving speaker retains routes from restarting peer.
// Called by bgp-gr plugin via DispatchCommandArgs("request bgp rib retain-routes", []string{peer}).
func (r *RIBManager) retainRoutes(selectorStr string) any {
	sel := selector.ParseDefault(selectorStr)
	r.peerMu.Lock()
	defer r.peerMu.Unlock()

	retained := 0
	for peer := range r.bgpPeers {
		if !sel.Matches(peer) {
			continue
		}
		r.retainedPeers[peer] = true
		retained++
	}

	return map[string]any{"retained-peers": retained}
}

// releaseRoutes clears the retain flag and deletes Adj-RIB-In for matching peers.
// RFC 4724: Called when restart timer expires or GR completes.
// Called by bgp-gr plugin via DispatchCommandArgs("request bgp rib release-routes", []string{peer}).
func (r *RIBManager) releaseRoutes(selectorStr string) any {
	sel := selector.ParseDefault(selectorStr)
	r.peerMu.Lock()

	released := 0
	var purgedPeers []netip.Addr
	for peer := range r.retainedPeers {
		if !sel.Matches(peer) {
			continue
		}
		delete(r.retainedPeers, peer)
		if peerRIB := r.bgpPeers[peer]; peerRIB != nil {
			peerRIB.Release()
			delete(r.bgpPeers, peer)
			purgedPeers = append(purgedPeers, peer)
		}
		delete(r.peerMeta, peer)
		if state := r.grState[peer]; state != nil && state.expiryTimer != nil {
			state.expiryTimer.Stop()
		}
		delete(r.grState, peer)
		released++
	}
	r.peerMu.Unlock()

	r.reconcileBestPathBulk(purgedPeers)

	return map[string]any{"released-peers": released}
}

// markStaleCommand handles "request bgp rib mark-stale <peer> <restart-time>".
// Marks all routes for the peer as stale and stores GR metadata.
// RFC 4724 Section 4.2: mark routes stale on GR-capable peer session drop.
// Args: [0]=peer address, [1]=restart time in seconds, [2]=optional stale level (default 1).
func (r *RIBManager) markStaleCommand(args []string) (string, any, error) {
	if len(args) < 2 {
		return statusError, "", errMarkStaleRequiresPeerRestartTime
	}

	peerAddr, err := netip.ParseAddr(args[0])
	if err != nil {
		return statusError, "", fmt.Errorf("mark-stale: invalid peer address %q: %w (expected an IP address)", args[0], err)
	}
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
			return statusError, "", errStaleLevelMustBe00
		}
		staleLevel = uint8(lvl)
	}

	r.peerMu.Lock()
	defer r.peerMu.Unlock()

	marked := 0
	peerRIB := r.bgpPeers[peerAddr]
	if peerRIB != nil {
		peerRIB.MarkAllStale(staleLevel)
		marked = peerRIB.StaleCount()
	}

	// RFC 9494: Propagate stale level to ribOut routes sourced from the restarting peer.
	// Only routes originally received from peerAddr are marked; routes from other
	// peers are left fresh. During LLGR readvertisement, sendRoutes carries
	// meta["stale"] through ForwardUpdate to egress filters.
	// ribOutSourceRef.peer is the engine's canonical source-peer string;
	// compare against the canonical form of the command argument (cold path).
	peerStr := peerAddr.String()
	for fam, keys := range r.ribOutSource {
		for key, src := range keys {
			if src.peer != peerStr {
				continue
			}
			for _, peerFamilies := range r.ribOut {
				if familyRoutes, ok := peerFamilies[fam]; ok {
					if entry, exists := familyRoutes[key]; exists {
						entry.StaleLevel = staleLevel
						familyRoutes[key] = entry
					}
				}
			}
		}
	}

	// Cancel existing expiry timer if consecutive restart.
	if existing := r.grState[peerAddr]; existing != nil && existing.expiryTimer != nil {
		existing.expiryTimer.Stop()
	}

	// Store GR state for status display and conditionally start expiry timer.
	// When restart-time is 0 (used by LLGR to raise stale level without a new
	// safety timer), skip the timer -- the LLST per-family timer handles expiry.
	now := time.Now()
	restartTime := uint16(restartSec)
	state := &peerGRState{
		StaleAt:     now,
		RestartTime: restartTime,
		ExpiresAt:   now.Add(time.Duration(restartTime) * time.Second),
	}
	if restartTime > 0 {
		expiryDuration := time.Duration(restartTime)*time.Second + grTimerMargin
		state.expiryTimer = time.AfterFunc(expiryDuration, func() {
			r.autoExpireStale(peerAddr, state)
		})
	}
	r.grState[peerAddr] = state

	logger().Debug("mark-stale", "peer", peerAddr, "marked", marked, "restart-time", restartTime)

	return statusDone, map[string]any{"marked": marked}, nil
}

// purgeStaleCommand handles "request bgp rib purge-stale <peer> [family]".
// Deletes only stale routes, optionally for a specific family.
// RFC 4724 Section 4.2: purge stale routes on EOR receipt or timer expiry.
// Args: [0]=peer address, [1]=optional family (e.g., "ipv4/unicast").
func (r *RIBManager) purgeStaleCommand(args []string) (string, any, error) {
	if len(args) < 1 {
		return statusError, "", errPurgeStaleRequiresPeer
	}

	peerAddr, err := netip.ParseAddr(args[0])
	if err != nil {
		return statusError, "", fmt.Errorf("purge-stale: invalid peer address %q: %w (expected an IP address)", args[0], err)
	}
	familyFilter := ""
	if len(args) >= 2 {
		familyFilter = args[1]
	}

	// Collect stale NLRIs under peerMu so no concurrent INSERT can change
	// stale state between snapshot and purge. Copies NLRI bytes per entry;
	// acceptable for this cold-path GR command even on a full table.
	r.peerMu.Lock()

	purged := 0
	peerRIB := r.bgpPeers[peerAddr]

	type staleNLRI struct {
		fam     family.Family
		nlri    []byte
		addPath bool
	}
	var affected []staleNLRI

	if peerRIB != nil {
		if familyFilter != "" {
			fam, ok := parseFamily(familyFilter)
			if ok {
				ap := peerRIB.IsAddPath(fam)
				peerRIB.IterateFamily(fam, func(nlriBytes []byte, entry storage.RouteEntry) bool {
					if entry.StaleLevel > storage.StaleLevelFresh {
						cp := make([]byte, len(nlriBytes))
						copy(cp, nlriBytes)
						affected = append(affected, staleNLRI{fam: fam, nlri: cp, addPath: ap})
					}
					return true
				})
				purged = peerRIB.PurgeFamilyStale(fam)
			}
		} else {
			for _, fam := range peerRIB.Families() {
				ap := peerRIB.IsAddPath(fam)
				peerRIB.IterateFamily(fam, func(nlriBytes []byte, entry storage.RouteEntry) bool {
					if entry.StaleLevel > storage.StaleLevelFresh {
						cp := make([]byte, len(nlriBytes))
						copy(cp, nlriBytes)
						affected = append(affected, staleNLRI{fam: fam, nlri: cp, addPath: ap})
					}
					return true
				})
			}
			purged = peerRIB.PurgeAllStale()
		}
	}

	if peerRIB != nil && peerRIB.StaleCount() == 0 {
		if state := r.grState[peerAddr]; state != nil && state.expiryTimer != nil {
			state.expiryTimer.Stop()
		}
		delete(r.grState, peerAddr)
	}
	r.peerMu.Unlock()

	for _, a := range affected {
		change, ok := r.checkBestPathChange(a.fam, a.nlri, a.addPath, nil)
		if ok {
			publishBestChanges([]bestChangeEntry{change}, a.fam)
		}
	}

	logger().Debug("purge-stale", "peer", peerAddr, "purged", purged, "family", familyFilter)

	return statusDone, map[string]any{"purged": purged}, nil
}

// bestPathStatus returns summary statistics about the best-path computation.
func (r *RIBManager) bestPathStatus() any {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()

	totalPeers := 0
	totalRoutes := 0
	for _, peerRIB := range r.bgpPeers {
		totalPeers++
		totalRoutes += peerRIB.Len()
	}
	for _, protoPeers := range r.ribInPool {
		for _, peerRIB := range protoPeers {
			totalPeers++
			totalRoutes += peerRIB.Len()
		}
	}

	return map[string]any{
		"running":        true,
		"peers-with-rib": totalPeers,
		"total-routes":   totalRoutes,
	}
}

// gatherCandidates collects best-path candidates for a given (family, nlri)
// across all peers. Acquires r.peerMu.RLock internally.
//
// Go's sync.RWMutex forbids recursive read-locking when a writer is pending
// (documented deadlock in sync/rwmutex.go), so callers that ALREADY hold
// r.peerMu.RLock MUST call gatherCandidatesLocked instead. The hot-path
// caller is checkBestPathChange, which runs with no outer lock held.
func (r *RIBManager) gatherCandidates(fam family.Family, nlriBytes []byte) []*Candidate {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()
	return r.gatherCandidatesLocked(fam, nlriBytes)
}

// gatherCandidatesLocked is gatherCandidates without the internal RLock.
// Caller MUST hold r.peerMu.RLock for the duration of the call, including
// across the returned candidates' lifetime if they reference peer state.
// PeerRIB content reads (peerRIB.Lookup) use PeerRIB's own lock.
func (r *RIBManager) gatherCandidatesLocked(fam family.Family, nlriBytes []byte) []*Candidate {
	var candidates []*Candidate
	for peer, peerRIB := range r.bgpPeers {
		entry, ok := peerRIB.Lookup(fam, nlriBytes)
		if !ok {
			continue
		}
		// RFC 9252 Section 5: path with SRv6 Service TLVs but no valid SID is ineligible.
		if isSRv6Ineligible(entry) {
			continue
		}
		// The map key gives the typed address; PeerRIB caches the canonical
		// string, so the hot path performs no parse and no conversion.
		c := r.extractCandidate(peer, peerRIB.PeerAddr(), entry)
		candidates = append(candidates, c)
	}
	return candidates
}

// extractCandidate builds a Candidate from a RouteEntry by reading pool handles.
// Extracts attribute values needed for RFC 4271 §9.1.2 comparison.
// peerAddr is the typed map key; peerStr is PeerRIB's cached canonical string
// (kept alongside to avoid a per-candidate Addr.String() allocation).
func (r *RIBManager) extractCandidate(peerAddr netip.Addr, peerStr string, entry storage.RouteEntry) *Candidate {
	c := &Candidate{
		PeerAddr:  peerStr,
		PeerIP:    peerAddr,
		LocalPref: 100, // RFC 4271 default
	}

	// Peer metadata for eBGP/iBGP detection.
	if meta := r.peerMeta[peerAddr]; meta != nil {
		c.PeerASN = meta.PeerASN
		c.LocalASN = meta.LocalASN
	}

	b := entry.GetBundle()

	if b.HasLocalPref() {
		if data, err := pool.LocalPref.Get(b.LocalPref); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				c.LocalPref = v
			}
		}
	}

	if entry.HasASPath() {
		c.ASPathHandle = entry.ASPath
		if data, err := pool.ASPath.Get(entry.ASPath); err == nil {
			c.ASPathLen = asPathLength(data)
			c.FirstAS = firstASInPath(data)
		}
	}

	if b.HasOrigin() {
		if data, err := pool.Origin.Get(b.Origin); err == nil && len(data) > 0 {
			c.Origin = attribute.Origin(data[0])
		}
	}

	if b.HasMED() {
		if data, err := pool.MED.Get(b.MED); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				c.MED = v
			}
		}
	}

	if b.HasOriginatorID() {
		if data, err := pool.OriginatorID.Get(b.OriginatorID); err == nil {
			if addr, ok := netip.AddrFromSlice(data); ok {
				c.OriginatorIP = addr
			}
		}
	}
	if !c.OriginatorIP.IsValid() {
		if meta := r.peerMeta[peerAddr]; meta != nil && meta.RouterID != 0 {
			id := meta.RouterID
			c.OriginatorIP = netip.AddrFrom4([4]byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)})
		}
	}

	// RFC 9494: LLGR-stale flag for best-path depreference.
	c.StaleLevel = entry.StaleLevel

	// RFC 4271 Section 9.1.2.2 Step 6: IGP cost to next-hop.
	if b.HasNextHop() {
		if data, err := pool.NextHop.Get(b.NextHop); err == nil {
			if nhAddr := parseNextHopAddr(data); nhAddr.IsValid() {
				c.IGPCost = lookupIGPCost(nhAddr)
			}
		}
	}

	return c
}
