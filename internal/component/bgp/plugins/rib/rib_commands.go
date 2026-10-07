// Design: docs/architecture/plugin/rib-storage-design.md — RIB command handlers
// RFC: rfc/short/rfc4724.md — Graceful Restart (mark-stale, purge-stale, retain/release)
// RFC: rfc/short/rfc9494.md — LLGR stale propagation to Adj-RIB-Out
// RFC: rfc/short/rfc8950.md — IPv6 next-hop validation for injected routes
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_nlri.go — NLRI wire format helpers
// Related: rib_attr_format.go — attribute formatting for show enrichment
// Related: bestpath.go — best-path selection (extractCandidate, gatherCandidatesLocked, SelectBest)
// Related: rib_commands_community.go — community attach/delete operations
// Related: rib_pipeline.go — iterator pipeline for show commands (scope, filters, terminals)
// Related: rib_pipeline_best.go — best-path pipeline (bestSource, bestPipeline, bestPathRows)
package rib

import (
	"fmt"
	"net/netip"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
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
	writes := r.reconcileSentSourceLocked(peerAddr, family.Family{}, nil)
	r.peerMu.Unlock()
	r.dispatchSentLifecycle(writes)

	for _, a := range affected {
		change, ok := r.checkBestPathChange(a.fam, a.nlri, a.addPath, nil)
		if ok {
			publishBestChanges([]bestChangeEntry{change}, a.fam)
		}
	}
}

// CommandHandler is the signature for RIB command handlers. Every handler is
// registered by doRegisterBuiltinCommands, which is the table's only writer.
type CommandHandler func(r *RIBManager, selector string, args []string) (string, any, error)

// ribCommandEntry holds a registered command handler and its help text.
type ribCommandEntry struct {
	Handler     CommandHandler
	Description string
}

// registeredCommands is the command dispatch table. It is built once, by
// registerBuiltinCommands, and read-only after that, so it needs no mutex.
//
// The build happens at init() in every process linking the composition root:
// this plugin's init() calls commandDecls() (rib.go) to put the declaration on
// its registry.Registration, and commandDecls() reads the table for each
// command's summary rather than restating it.
var registeredCommands = map[string]*ribCommandEntry{}

// builtinsOnce guards against concurrent/double-registration of builtin commands.
var builtinsOnce sync.Once

// registerCommand adds a command handler to the dispatch table.
// Returns an error if the command name is already registered.
func registerCommand(name, help string, handler CommandHandler) error {
	if _, exists := registeredCommands[name]; exists {
		return fmt.Errorf("RIB command %q already registered", name)
	}
	registeredCommands[name] = &ribCommandEntry{Handler: handler, Description: help}
	return nil
}

// registerBuiltinCommands populates the command table with RIB-native commands
// and LLGR extensions. It is called from this package's init(), through
// commandDecls(), and again when the engine starts.
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
		{[]string{"request bgp rib recovery"}, "Read source-owned DOWN recovery", (*RIBManager).recoveryCommand},
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
		{[]string{"request bgp rib retain-routes"}, "Retain peer routes: <selector> [on-down] [family ...]; on-down delegates unretained withdrawals to the forwarding owner",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				if len(args) == 0 {
					return statusError, "", errBgpRibRetainRoutesRequiresA
				}
				familyArgs := args[1:]
				onDown := len(familyArgs) != 0 && familyArgs[0] == "on-down"
				if onDown {
					familyArgs = familyArgs[1:]
				}
				var families []family.Family
				for _, name := range familyArgs {
					fam, ok := parseFamily(name)
					if !ok {
						return statusError, "", fmt.Errorf("retain-routes: unknown family %q", name)
					}
					families = append(families, fam)
				}
				return statusDone, r.retainRoutes(args[0], families, onDown), nil
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
		// The three words are the model's (ze-rib-cmd.yang), not this
		// sentence's.
		{[]string{"request bgp rib fastpath"}, "Switch or report the zero-copy forward-handle fast path (rib-arch-6)",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				return r.fastpathCommand(args)
			}},
	}

	for _, b := range builtins {
		for _, name := range b.names {
			registeredCommands[name] = &ribCommandEntry{Handler: b.handler, Description: b.help}
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

		if key == kwOrigin {
			// The names and their wire values come from the attribute package,
			// which holds the one spelling of them.
			origin, ok := attribute.OriginFromText(val)
			if !ok {
				return statusError, "", fmt.Errorf("unknown origin: %s (use %s)", val, strings.Join(attribute.OriginTextNames(), ", "))
			}
			ab.SetOrigin(uint8(origin))
			continue
		}
		if key == kwNextHop || key == kwNextHopLong {
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
		if key == kwASPath {
			asns, err := parseASNList(val)
			if err != nil {
				return statusError, "", fmt.Errorf("invalid aspath: %w", err)
			}
			ab.SetASPath(asns)
			continue
		}
		if key == kwLocalPref {
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return statusError, "", fmt.Errorf("invalid localpref: %w", err)
			}
			ab.SetLocalPref(uint32(n))
			continue
		}
		if key == kwMED {
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
	r.bgpPeers[peer].Insert(fam, attrBytes, nlriBytes)
	r.peerMu.Unlock()

	r.reconcileBestPath(fam, nlriBytes)

	return statusDone, map[string]any{jsonKeyInjected: prefix, rowKeyPeer: args[0], rowKeyFamily: familyStr}, nil
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

	return statusDone, map[string]any{jsonKeyWithdrawn: prefix, rowKeyPeer: args[0], rowKeyFamily: familyStr, jsonKeyExisted: removed}, nil
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

	loc := r.locRIB.Load()

	if loc == nil {
		return statusError, "", fmt.Errorf("loc-rib not available")
	}

	best, pfx, found := loc.LPM(fam, addr)
	if !found {
		return statusDone, map[string]any{
			"source":     addrStr,
			rowKeyFamily: familyStr,
			"found":      false,
		}, nil
	}

	nextHop := ""
	if best.NextHop.IsValid() {
		nextHop = best.NextHop.String()
	}
	return statusDone, map[string]any{
		"source":         addrStr,
		rowKeyFamily:     familyStr,
		"found":          true,
		"matched-prefix": pfx.String(),
		"next-hop":       nextHop,
		"distance":       best.AdminDistance,
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
		// asn.Parse reads the asdot spellings beside the decimal one, so an
		// operator injecting a route types the AS path the way they read it.
		number, err := asn.Parse(p)
		if err != nil {
			return nil, fmt.Errorf("invalid ASN %q: %w", p, err)
		}
		asns = append(asns, number)
	}
	return asns, nil
}

// Attribute keywords of `request bgp rib inject`. An operator types these, and
// each one names a BGP path attribute rather than a metric label or a JSON
// field, even where the three spell the same word.
const (
	kwOrigin      = "origin"
	kwNextHop     = "nhop"
	kwNextHopLong = "nexthop"
	kwASPath      = "aspath"
	kwLocalPref   = "localpref"
	kwMED         = "med"
)

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
	slices.Sort(subs)
	return map[string]any{"subcommands": subs}
}

// ribCommandList returns all RIB commands, built from the command registry.
func ribCommandList() any {
	type entry struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	cmds := make([]entry, 0, len(registeredCommands))
	for name, e := range registeredCommands {
		cmds = append(cmds, entry{Name: name, Description: e.Description})
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
		if famStr == "" {
			groups = r.collectGroupedRibOutRoutes(peer)
		} else {
			if fam, ok := family.LookupFamily(famStr); ok {
				groups = r.collectGroupedRibOutRoutesForFamily(peer, fam)
			}
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
//
// Every command carries meta["replay"], for the reason resendRoutesWithCursor
// states: the session is UP, so the destination peer's Adj-RIB-Out holds every
// route being re-sent and withholds each one under RFC 4271 Section 9.2
// (reactor/adj_rib_out.go). The peer asked for them back, so the request says so.
//
// RFC 2918 Section 4: "Otherwise, the BGP speaker shall re-advertise to that
// peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its
// outbound route filtering policy." That "shall" outranks the Section 9.2
// "SHOULD NOT advertise ... the same BGP route as was previously advertised",
// because a refresh exists to send exactly what the peer already holds.
//
// Silence here is worse than a missing UPDATE. RFC 7313 Section 4 has the peer
// mark every route of the <AFI, SAFI> stale on the BoRR and "MUST immediately
// remove any routes from the peer that are still marked as stale" on the EoRR,
// so a refresh answered by the two markers alone withdraws the whole family.
func (r *RIBManager) sendRoutes(peerAddr string, routes []*Route) {
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].MsgID < routes[j].MsgID
	})

	for _, route := range routes {
		cmd := formatRouteCommand(route)
		meta := map[string]any{metaKeyReplay: true,
			bgptypes.SentOwnerMessageMeta: strconv.FormatUint(route.MsgID, 10)}
		if route.StaleLevel > 0 {
			meta["stale"] = route.StaleLevel
		}
		r.updateRouteWithMeta(peerAddr, cmd, meta)
	}
}

// status returns RIB status.
func (r *RIBManager) status(famFilter string) any {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()

	// Per-peer Adj-RIB-In / Adj-RIB-Out sizes, keyed by peer address. summary.go
	// merges these into `show bgp` (routes-received/accepted from "in",
	// routes-sent from "out") via ForwardToPlugin, so the birdwatcher LG can show
	// per-peer route counts without cmd/peer importing this plugin. Only BGP
	// peers (bgpPeers/ribOut, keyed by netip.Addr) are per-peer; ribInPool holds
	// non-BGP redistribution sources that do not map to a BGP peer row.
	// RFC 4271 Section 3.2: Adj-RIB-In holds routes advertised by a peer,
	// Adj-RIB-Out holds routes advertised to a peer.
	//
	// famFilter scopes the per-peer counts to one family (so a family-filtered
	// `show bgp <afi/safi>` reports family-scoped, not all-family,
	// counts). Empty famFilter, or an unrecognized one, reports all-family
	// totals. The global routes-in/routes-out stay all-family totals regardless,
	// since other consumers depend on them.
	scopeFam, scoped := family.Family{}, false
	if famFilter != "" {
		scopeFam, scoped = family.LookupFamily(famFilter)
	}
	// A peer with routes only in other families gets a {in:0,out:0} entry under
	// a family filter. That is intentional, not spurious: a peer that appears in
	// a family-filtered `show bgp <fam>` (it negotiated <fam>) but holds
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
// An empty families argument preserves the operator's peer-wide retention.
// A supplied allowlist prunes other families now, without storing another set.
// onDown is the explicit GR handoff to the ordinary forwarding DOWN owner;
// standalone commands instead dispatch their own pruned-family withdrawals.
func (r *RIBManager) retainRoutes(selectorStr string, families []family.Family, onDown bool) any {
	sel := selector.ParseDefault(selectorStr)
	r.peerMu.Lock()

	retained := 0
	var affected []netip.Addr
	var writes []sentLifecycleWrite
	for peer := range r.bgpPeers {
		if !sel.Matches(peer) {
			continue
		}
		r.retainedPeers[peer] = true
		if len(families) != 0 {
			r.bgpPeers[peer].RetainFamilies(families)
			if !onDown {
				writes = append(writes, r.reconcileSentSourceLocked(peer, family.Family{}, nil)...)
			}
			// On DOWN, keep sent ownership until RS's fenced withdrawal or
			// replacement reaches the destination. GR owns retained families.
			affected = append(affected, peer)
		}
		retained++
	}
	r.peerMu.Unlock()
	r.dispatchSentLifecycle(writes)
	r.reconcileBestPathBulk(affected)

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
	var writes []sentLifecycleWrite
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
		writes = append(writes, r.reconcileSentSourceLocked(peer, family.Family{}, nil)...)
		released++
	}
	r.peerMu.Unlock()
	r.dispatchSentLifecycle(writes)

	r.reconcileBestPathBulk(purgedPeers)

	return map[string]any{"released-peers": released}
}

// markStaleCommand handles "request bgp rib mark-stale <peer> <restart-time> [level [family]]".
// Marks peer routes as stale and stores GR metadata. Omitting family preserves
// the peer-wide operator command; GR's LLGR transition supplies its own family.
// RFC 4724 Section 4.2: mark routes stale on GR-capable peer session drop.
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
	var selected family.Family
	if len(args) >= 4 {
		fam, ok := parseFamily(args[3])
		if !ok {
			return statusError, "", fmt.Errorf("mark-stale: unknown family %q", args[3])
		}
		selected = fam
	}

	defer r.reconcileFlowSpecs()
	r.peerMu.Lock()
	defer r.peerMu.Unlock()

	marked := 0
	peerRIB := r.bgpPeers[peerAddr]
	if peerRIB != nil {
		if selected == (family.Family{}) {
			peerRIB.MarkAllStale(staleLevel)
			marked = peerRIB.StaleCount()
		} else {
			peerRIB.ModifyFamilyAll(selected, func(entry *storage.RouteEntry) {
				entry.StaleLevel = staleLevel
				marked++
			})
		}
	}

	// RFC 9494: Propagate stale level to ribOut routes sourced from the restarting peer.
	// Only routes originally received from peerAddr are marked; routes from other
	// peers are left fresh. During LLGR readvertisement, sendRoutes carries
	// meta["stale"] through ForwardUpdate to egress filters.
	// SourcePeer belongs to each destination's advertisement, so two destinations
	// may retain the same native key from different source peers independently.
	peerStr := peerAddr.String()
	for _, peerFamilies := range r.ribOut {
		for fam, familyRoutes := range peerFamilies {
			if selected != (family.Family{}) && fam != selected {
				continue
			}
			for key, entry := range familyRoutes {
				if entry.SourcePeer != peerStr {
					continue
				}
				entry.StaleLevel = staleLevel
				familyRoutes[key] = entry
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
	// An absent second argument purges every family the peer holds. A present
	// one is resolved here, before the lock: a name no family carries used to
	// fall through the loop below and report success over nothing purged, so
	// the operator was told the stale routes were gone.
	familyFilter := ""
	var filtered family.Family
	if len(args) >= 2 {
		familyFilter = args[1]
		fam, ok := parseFamily(familyFilter)
		if !ok {
			return statusError, "", fmt.Errorf("purge-stale: unknown family %q", familyFilter)
		}
		filtered = fam
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
		if familyFilter == "" {
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
		} else {
			ap := peerRIB.IsAddPath(filtered)
			peerRIB.IterateFamily(filtered, func(nlriBytes []byte, entry storage.RouteEntry) bool {
				if entry.StaleLevel > storage.StaleLevelFresh {
					cp := make([]byte, len(nlriBytes))
					copy(cp, nlriBytes)
					affected = append(affected, staleNLRI{fam: filtered, nlri: cp, addPath: ap})
				}
				return true
			})
			purged = peerRIB.PurgeFamilyStale(filtered)
		}
	}

	if peerRIB != nil && peerRIB.StaleCount() == 0 {
		if state := r.grState[peerAddr]; state != nil && state.expiryTimer != nil {
			state.expiryTimer.Stop()
		}
		delete(r.grState, peerAddr)
	}
	writes := r.reconcileSentSourceLocked(peerAddr, filtered, nil)
	r.peerMu.Unlock()
	r.dispatchSentLifecycle(writes)

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

// gatherCandidatesLocked collects best-path candidates for the route nlriBytes
// names, across all peers, where nlriBytes leads with the sender's 4-byte path
// identifier under addPath (RFC 7911 Section 3). The NLRI is read as an
// announcement: a caller holding a withdrawal's NLRI keys it itself
// (routeIdentity) and calls gatherKeyCandidatesLocked. Caller MUST hold
// peerMu.RLock while gathering peer metadata. The returned snapshots remain
// readable after that lock is released; the caller MUST releaseCandidates
// after its last borrowed-handle read. Elections take their bestPrev shard
// before this peer read admission.
//
// A CIDR family is gathered by PREFIX, and every other family by its route
// key, the NLRI without its path identifier, over every path of every peer in
// either case, so the path identifier never partitions the election.
func (r *RIBManager) gatherCandidatesLocked(fam family.Family, nlriBytes []byte, addPath bool) []*Candidate {
	if !storage.IsCIDRFamily(fam) {
		var scratch [nlrisplit.PrefixKeyScratchSize]byte
		routeKey, ok := routeIdentity(fam, nlriBytes, addPath, false, scratch[:])
		if !ok {
			return nil
		}
		return r.gatherKeyCandidatesLocked(fam, routeKey)
	}
	_, pfx, ok := parsePrevKey(fam, nlriBytes, addPath)
	if !ok {
		return nil
	}
	return r.gatherPrefixCandidatesLocked(fam, pfx)
}

// gatherPrefixCandidatesLocked collects every stored path of pfx from every
// peer. Caller MUST hold r.peerMu.RLock and later call releaseCandidates.
//
// RFC 7911 Section 2: "a particular path for an address prefix can be
// identified by the combination of the address prefix and the Path
// Identifier". The identifier names a path; it does not make a second prefix.
//
// RFC 8277 Section 3.1: "the two UPDATEs are received on the same session,
// add-paths is used on that session, and the NLRIs of the two UPDATEs have
// different path identifiers. These two routes MUST be considered to be
// comparable, even if they specify different labels." So the paths one
// ADD-PATH session holds for a prefix are candidates of ONE selection.
//
// Each peer is asked by prefix, never with another session's wire key: a key
// framed for ADD-PATH reads as a different prefix in a peer stored without it.
func (r *RIBManager) gatherPrefixCandidatesLocked(fam family.Family, pfx netip.Prefix) []*Candidate {
	var candidates []*Candidate
	// Loaded ONCE, so every candidate for this prefix is judged against the same
	// set of this speaker's own addresses (rib_self_nexthop.go). A per-candidate
	// load could straddle a session change and admit one route while excluding
	// its equal.
	selfNextHops := r.selfNextHops.Load()
	// Stack-backed: a prefix carries one to a few paths per peer, so the common
	// case appends without allocating. A peer with more paths grows it once.
	var pathsArray [4]storage.PrefixPath
	validate := ribevents.ValidationEnabled()
	for peer, peerRIB := range r.bgpPeers {
		paths, peerAddPath := peerRIB.AppendPrefixPathsRetained(fam, pfx, pathsArray[:0])
		for i := range paths {
			path := &paths[i]
			if validate && !ribevents.RouteEligible(ribevents.ValidationRoute{
				Peer: peer, Family: fam, Prefix: pfx, PathID: path.PathID,
			}, path.Entry.MsgID) {
				path.Release()
				continue
			}
			if !r.candidateAdmitted(fam, peerRIB, path.Entry, selfNextHops) {
				path.Release()
				continue
			}
			c := r.extractCandidate(fam, peer, peerRIB.PeerAddr(), path.Entry)
			c.PathID = path.PathID
			c.AddPath = peerAddPath
			c.entry, c.labelHandle = path.Entry, path.Labels
			candidates = append(candidates, c)
		}
	}
	return candidates
}

// gatherKeyCandidatesLocked collects every stored path of the route routeKey
// names from every peer, for a family whose NLRI is no CIDR prefix. routeKey
// is a routeIdentity result: no path identifier and no label. Caller MUST hold
// r.peerMu.RLock and later call releaseCandidates.
//
// RFC 8277 Section 3.1 compares routes "even if they specify different
// labels": two PEs announcing one RD and prefix under different labels are
// paths of ONE route, so the route key never carries the label.
//
// RFC 7911 Section 2 makes the path identifier a name for one path of a route,
// so it does not make a second route: the paths of one ADD-PATH session, and
// the same route from a session without ADD-PATH, are candidates of ONE
// selection. Each peer is asked by the route key, never with the triggering
// session's wire NLRI, whose four path-id octets no other framing matches.
func (r *RIBManager) gatherKeyCandidatesLocked(fam family.Family, routeKey []byte) []*Candidate {
	var candidates []*Candidate
	selfNextHops := r.selfNextHops.Load()
	// Stack-backed for the same reason gatherPrefixCandidatesLocked's is.
	var pathsArray [4]storage.PrefixPath
	var nlriBuf [opaqueKeyOctetsInline]byte
	for peer, peerRIB := range r.bgpPeers {
		paths, peerAddPath := peerRIB.AppendKeyPathsRetained(fam, routeKey, pathsArray[:0])
		for i := range paths {
			path := &paths[i]
			// Validation keys a route by the NLRI its own session sent.
			nlri := framedRouteNLRI(nlriBuf[:0], path.Route, path.PathID, peerAddPath)
			if !r.validationEligible(peer, peerRIB, fam, nlri, path.Entry.MsgID) {
				path.Release()
				continue
			}
			if !r.candidateAdmitted(fam, peerRIB, path.Entry, selfNextHops) {
				path.Release()
				continue
			}
			// The map key gives the typed address; PeerRIB caches the canonical
			// string, so the hot path performs no parse and no conversion.
			c := r.extractCandidate(fam, peer, peerRIB.PeerAddr(), path.Entry)
			c.PathID = path.PathID
			c.AddPath = peerAddPath
			c.Route = path.Route
			c.entry, c.labelHandle = path.Entry, path.Labels
			candidates = append(candidates, c)
		}
	}
	return candidates
}

// releaseCandidates returns snapshots acquired by the gather functions. The
// extracted scalar values remain available for the show-best reason terminal,
// but no caller may compare or dereference ASPathHandle after this release.
func releaseCandidates(candidates []*Candidate) {
	for _, c := range candidates {
		c.entry.Release()
		if c.labelHandle.IsValid() {
			_ = pool.Labels.Release(c.labelHandle)
			c.labelHandle = attrpool.InvalidHandle
		}
	}
}

// candidateAdmitted reports whether a stored path may enter the decision
// process at all: the RFC 9252 SRv6 and RFC 4271 Section 5.1.3 exclusions, the
// same for every family.
func (r *RIBManager) candidateAdmitted(fam family.Family, peerRIB *storage.PeerRIB, entry storage.RouteEntry, selfNextHops *[]netip.Addr) bool {
	// RFC 9252 Section 5: path with SRv6 Service TLVs but no valid SID is ineligible.
	if isSRv6Ineligible(entry) {
		return false
	}
	// RFC 4271 Section 5.1.3: "A BGP speaker SHALL NOT install a route with
	// itself as the next hop." A route naming one of this speaker's own
	// addresses tells it to forward through itself, which is a local loop.
	//
	// EXCLUDED FROM THE DECISION PROCESS, not refused at install. Refusing at
	// install would leave the prefix with no route at all whenever the broken
	// path happened to win, even with a sound alternative in the RIB. Removing
	// it from candidacy lets the runner-up win, which is what Section 9.1.2
	// already does with a next hop it cannot resolve, and this is a next hop
	// that resolves to this speaker.
	//
	// Nothing legitimate reaches here with a self next hop. Every candidate is
	// a route another speaker SENT (r.bgpPeers is the Adj-RIB-In), and
	// Section 6.3 calls that address semantically incorrect on arrival. Locally
	// originated routes -- static, connected, redistributed -- never enter this
	// map and so are untouched by this test.
	if selfNextHops != nil && len(*selfNextHops) > 0 {
		if nh := entryNextHopAddr(fam, entry); isSelfNextHop(selfNextHops, nh) {
			// RFC 4271 Section 6.3: "the error SHOULD be logged, and the route
			// SHOULD be ignored". A guard that drops a route in silence is how
			// "routes vanish sometimes" gets reported instead of the cause.
			logger().Warn("route excluded from best-path: its next hop is this speaker's own address",
				"peer", peerRIB.PeerAddr(), "next-hop", nh, "family", fam,
				"rfc", "RFC 4271 Section 5.1.3",
				"action", "route not installed; another path to this prefix is used if one exists")
			return false
		}
	}
	return true
}

// extractCandidate builds a Candidate from a RouteEntry by reading pool handles.
// Extracts attribute values needed for RFC 4271 §9.1.2 comparison.
// peerAddr is the typed map key; peerStr is PeerRIB's cached canonical string
// (kept alongside to avoid a per-candidate Addr.String() allocation).
func (r *RIBManager) extractCandidate(fam family.Family, peerAddr netip.Addr, peerStr string, entry storage.RouteEntry) *Candidate {
	c := &Candidate{
		PeerAddr:  peerStr,
		PeerIP:    peerAddr,
		LocalPref: 100, // RFC 4271 default
		IGPCost:   ^uint64(0),
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
	// RFC 4271 Section 9.1.2.2 (f): "prefer the route received from the peer
	// with the lowest BGP Identifier", which RFC 4456 Section 9 (a) replaces
	// with ORIGINATOR_ID when the route carries one. The fallback is therefore
	// the PEER's identifier, RemoteRouterID, and never RouterID: that one is
	// this speaker's own, so every candidate carried it, the step tied on every
	// comparison, and selection fell through to step g).
	if !c.OriginatorIP.IsValid() {
		if meta := r.peerMeta[peerAddr]; meta != nil && meta.RemoteRouterID != 0 {
			id := meta.RemoteRouterID
			c.OriginatorIP = netip.AddrFrom4([4]byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)})
		}
	}

	// RFC 4456 Section 9: "The CLUSTER_LIST length is zero if a route does not
	// carry the CLUSTER_LIST attribute." Absent leaves the field at its zero
	// value, which is the answer the RFC gives rather than "unknown".
	if b.HasClusterList() {
		// The handle was valid one call ago, so a Get failure here is a pool
		// defect rather than a wire condition. Leaving the count at zero would
		// say "no CLUSTER_LIST" and win the step outright, so a value this
		// speaker cannot read saturates and loses it instead.
		c.ClusterListEntries = clusterListEntriesMax
		if data, err := pool.ClusterList.Get(b.ClusterList); err == nil {
			c.ClusterListEntries = clusterListEntries(data)
		}
	}

	// RFC 9494: LLGR-stale flag for best-path depreference.
	c.StaleLevel = entry.StaleLevel

	c.AIGP, c.HasAIGP = entryAIGP(entry)

	// RFC 7311 Sections 4.1 and 4.2 use the selected family's next hop,
	// including MP_REACH_NLRI, with the received metric kept separate.
	if nextHop := entryNextHopAddr(fam, entry); nextHop.IsValid() {
		if distance := r.igpDistance(nextHop); distance.Resolved {
			c.IGPCost = distance.Cost
		}
	}

	return c
}
