// Design: docs/architecture/bgp/as112-coordination.md -- AS112 advisory coordination checks
// advisory doctor checks (H2/R-4, M5/R-3), built after the closed
// as112-3 spec deferred them pending a doctor-check "home" decision.
// Detail: checks_as112_coordination_test.go -- unit tests for both checks
// Related: checks_helpers.go -- nestedValue/nestedSlice config-tree helpers
// RFC: rfc/short/rfc7534.md -- AS112 Nameserver Operations (Section 3.2/3.3/3.4)
// RFC: rfc/short/rfc7535.md -- AS112 Redirection Using DNAME (Section 3.1)
// RFC: rfc/short/rfc6996.md -- Private Use ASN ranges

// Neither check can live in the as112 plugin nor the bgp component: the
// as112-3 spec's own Key Design Decisions required that neither the as112
// plugin read BGP config nor BGP hardcode AS112 knowledge. internal/component/doctor
// is the neutral third home per ai/rules/repo-maintenance.md ("dependency with
// no narrower owner") -- it already reads the whole config.Tree generically
// (see checkConfigReferences, checkBGPMD5) without importing either package.
// The AS112 plugin's own (unrelated) port-53 bind-capability check lives at
// internal/plugins/as112 instead, since that one is a same-plugin runtime
// dependency, not a cross-component coordination concern.

package doctor

import (
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// as112ASN is the well-known AS112 Autonomous System Number (RFC 7534
// Section 3.2) operators may optionally originate routes as, via
// asn.local + replace-as, for a global (non-local-use) AS112 node.
const as112ASN = "112"

// as112CoveringPrefixes are the four fixed AS112 covering prefixes (RFC 7534
// Section 3.4 Direct Delegation, RFC 7535 Section 3.1 DNAME Redirection) --
// not operator-configurable. Parsed once into netip.Prefix so a non-canonical
// but equivalent nlri token (leading zeros, uppercase hex, expanded IPv6)
// still compares equal instead of missing an exact-literal-string match --
// the production watchdog pool builder normalizes each token the same way
// (internal/component/bgp/plugins/watchdog/config.go's normalizePrefix).
//
// This list MUST mirror as112events.CoveringPrefixesV4/V6 (the producer's and
// fakeas112's shared source). It is deliberately NOT imported from there: this
// package's doc explains the doctor is the neutral third home that couples to
// NEITHER the as112 plugin nor the bgp component, so it re-states the RFC-fixed
// constants rather than importing the plugin. They change only if the RFC set
// changes; update both places together.
var as112CoveringPrefixes = []netip.Prefix{
	netip.MustParsePrefix("192.175.48.0/24"),   // Direct Delegation v4
	netip.MustParsePrefix("2620:4f:8000::/48"), // Direct Delegation v6
	netip.MustParsePrefix("192.31.196.0/24"),   // DNAME Redirection v4
	netip.MustParsePrefix("2001:4:112::/48"),   // DNAME Redirection v6
}

// checkAS112WatchdogWithdraw warns (AC-10, H2, R-4) when a BGP update block
// announces an AS112 covering prefix without a watchdog{withdraw true}
// marker. The marker's absence defaults to already-announced, not
// already-withdrawn -- so a worked example that omits it announces the
// AS112 route at startup before the DNS service is confirmed healthy
// (RFC 7534 Section 3.3).
func checkAS112WatchdogWithdraw(tree *config.Tree) []diagnostic.Diagnostic {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer

	check := func(path string, update *config.Tree) {
		if !updateCarriesAS112CoveringPrefix(update) {
			return
		}
		if updateHasWatchdogWithdraw(update) {
			return
		}
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-as112-watchdog-missing-withdraw",
			Severity: diagnostic.SeverityWarning,
			Message: tb.Reset().Str(path).
				Str(": update block announces an AS112 covering prefix without watchdog{withdraw true} -- the route will be announced at startup before AS112 health is confirmed (RFC 7534 Section 3.3)").
				String(),
		})
	}

	for _, u := range bgp.GetListOrdered("update") {
		check(tb.Reset().Str("bgp/update/").Str(u.Key).String(), u.Value)
	}
	for _, p := range bgp.GetListOrdered("peer") {
		for _, u := range p.Value.GetListOrdered("update") {
			check(tb.Reset().Str("bgp/peer/").Str(p.Key).Str("/update/").Str(u.Key).String(), u.Value)
		}
	}
	for _, g := range bgp.GetListOrdered("group") {
		for _, u := range g.Value.GetListOrdered("update") {
			check(tb.Reset().Str("bgp/group/").Str(g.Key).Str("/update/").Str(u.Key).String(), u.Value)
		}
		for _, p := range g.Value.GetListOrdered("peer") {
			for _, u := range p.Value.GetListOrdered("update") {
				check(tb.Reset().Str("bgp/group/").Str(g.Key).Str("/peer/").Str(p.Key).Str("/update/").Str(u.Key).String(), u.Value)
			}
		}
	}

	return diags
}

// updateCarriesAS112CoveringPrefix mirrors the production watchdog pool
// builder's own content grammar (internal/component/bgp/plugins/watchdog/config.go):
// "content" is "<op> <prefix> [<prefix> ...]" (plus optional inline rd/label
// modifier pairs, which simply fail netip.ParsePrefix and are skipped here).
// Only "add" entries are ever announced -- "del"/other ops never reach the
// wire, so a covering prefix appearing only in a non-"add" entry must not be
// treated as announced (that previously caused a false-positive missing-withdraw
// warning on a block that only ever withdraws the prefix).
func updateCarriesAS112CoveringPrefix(update *config.Tree) bool {
	for _, n := range update.GetListOrdered("nlri") {
		content, ok := n.Value.Get("content")
		if !ok || content == "" {
			continue
		}
		fields := strings.Fields(content)
		if len(fields) < 2 || fields[0] != "add" {
			continue
		}
		for _, tok := range fields[1:] {
			p, err := netip.ParsePrefix(tok)
			if err != nil {
				continue
			}
			if slices.Contains(as112CoveringPrefixes, normalizeIPv4In6(p)) {
				return true
			}
		}
	}
	return false
}

// normalizeIPv4In6 rewrites an IPv4-in-IPv6-embedded prefix (e.g.
// "::ffff:192.175.48.0/120") to its native IPv4 form ("192.175.48.0/24") so
// it compares equal to as112CoveringPrefixes' native-IPv4 entries -- plain
// netip.Prefix equality treats Is4() and Is4In6() addresses as always
// unequal even when they represent the same network, per net/netip's
// documented behavior. Only unmaps when bits describe a prefix entirely
// within the embedded 32-bit space (bits >= 96); anything else is returned
// unchanged; comparison against the native-IPv4 table correctly stays false.
func normalizeIPv4In6(p netip.Prefix) netip.Prefix {
	addr := p.Addr()
	if !addr.Is4In6() || p.Bits() < 96 {
		return p
	}
	return netip.PrefixFrom(addr.Unmap(), p.Bits()-96)
}

func updateHasWatchdogWithdraw(update *config.Tree) bool {
	wd := update.GetContainer("watchdog")
	if wd == nil {
		return false
	}
	v, _ := wd.Get("withdraw")
	return v == configTrueValue
}

// checkAS112GlobalOriginCoordination warns (AC-11, M5, R-3) when a BGP
// session has asn.local 112 with the replace-as local-option (overriding
// AS_PATH origin to 112) while eBGP-peering a non-private-use remote ASN
// (RFC 6996 Section 4) -- silently making this node an uncoordinated global
// AS112 origin, which RFC 7534 Section 3.2/Section 5 requires coordinating
// before deploying.
func checkAS112GlobalOriginCoordination(tree *config.Tree) []diagnostic.Diagnostic {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}

	// bgp/session/asn/local is a real third inheritance tier below group and
	// peer: PeersFromTree (internal/component/bgp/reactor/config.go) seeds
	// its local-AS default from exactly this leaf when neither group nor
	// peer overrides it, so a peer relying on it is a genuine live AS112
	// origin session, not just an unresolved default.
	globalLocal, _ := nestedValue(bgp, "session", "asn", "local")

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer

	check := func(path string, group, peer *config.Tree) {
		local, ok := inheritedValue(group, peer, "session", "asn", "local")
		if !ok {
			local = globalLocal
		}
		if local != as112ASN {
			return
		}
		if !hasReplaceAsOption(group, peer) {
			return
		}
		remoteStr, ok := inheritedValue(group, peer, "session", "asn", "remote")
		if !ok || remoteStr == "" {
			return
		}
		remote, err := strconv.ParseUint(remoteStr, 10, 32)
		if err != nil {
			return
		}
		if isPrivateUseASN(uint32(remote)) { //nolint:gosec // bounded by ParseUint bitSize=32
			return
		}
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-as112-global-origin-uncoordinated",
			Severity: diagnostic.SeverityWarning,
			Message: tb.Reset().Str(path).
				Str(": asn.local 112 with replace-as is set on an eBGP session to non-private ASN ").Str(remoteStr).
				Str(" -- this makes the node an uncoordinated global AS112 origin (RFC 7534 Section 3.2/Section 5); coordinate before deploying outside a local-use mirror").
				String(),
		})
	}

	for _, p := range bgp.GetListOrdered("peer") {
		check(tb.Reset().Str("bgp/peer/").Str(p.Key).String(), nil, p.Value)
	}
	for _, g := range bgp.GetListOrdered("group") {
		for _, p := range g.Value.GetListOrdered("peer") {
			check(tb.Reset().Str("bgp/group/").Str(g.Key).Str("/peer/").Str(p.Key).String(), g.Value, p.Value)
		}
	}

	return diags
}

// hasReplaceAsOption treats peer's local-options as a full override of
// group's whenever peer sets ANY value, same as inheritedValue's scalar
// inheritance (checkBGPMD5's hasMD5 accepts the identical imprecision) --
// config.Tree's GetSlice/SetSlice cannot distinguish "peer never set
// local-options" from "peer explicitly cleared it," so an operator who
// clears an inherited replace-as at the peer level is not detectable here.
func hasReplaceAsOption(group, peer *config.Tree) bool {
	if opts := nestedSlice(peer, "session", "asn", "local-options"); len(opts) > 0 {
		return slices.Contains(opts, "replace-as")
	}
	return slices.Contains(nestedSlice(group, "session", "asn", "local-options"), "replace-as")
}

// redistributeImportsSourceIntoBGP reports whether `redistribute { destination
// bgp { import <source> } }` is configured, in either the list form (import
// entries keyed by source) or the scalar fallback (import <source>;) -- mirroring
// internal/component/config/loader_redistribute.go's own parse. Read generically
// from the config tree so this neutral doctor package needs no bgp or
// redistribute-plugin import.
func redistributeImportsSourceIntoBGP(tree *config.Tree, source string) bool {
	rd := tree.GetContainer("redistribute")
	if rd == nil {
		return false
	}
	for _, dest := range rd.GetListOrdered("destination") {
		if dest.Key != "bgp" {
			continue
		}
		for _, imp := range dest.Value.GetListOrdered("import") {
			if imp.Key == source {
				return true
			}
		}
		if scalar, ok := dest.Value.Get("import"); ok && scalar == source {
			return true
		}
	}
	return false
}

// checkAS112RedistributeOriginCoordination warns (AC-11, R-3) when the as112
// service originates its covering prefixes as AS112 -- asn 112, the default --
// via `redistribute { destination bgp { import as112 } }` while an eBGP session
// to a non-private remote ASN exists. This is the redistribute-path form of the
// same uncoordinated-global-AS112-origin risk (RFC 7534 Section 3.2/Section 5)
// the sibling checkAS112GlobalOriginCoordination catches for the hand-authored
// asn.local + replace-as path. An explicit non-112 asn is the operator
// originating under its own or a private AS and is not flagged.
func checkAS112RedistributeOriginCoordination(tree *config.Tree) []diagnostic.Diagnostic {
	svc := tree.GetContainer("service")
	if svc == nil {
		return nil
	}
	as112 := svc.GetContainer("as112")
	if as112 == nil {
		return nil
	}
	if enabled, _ := as112.Get("enabled"); enabled != configTrueValue {
		return nil
	}
	// asn unset defaults to 112 (an AS112 virtual-router origin); an explicit
	// non-112 asn is the operator's own/private origin and not this concern.
	if asn, ok := as112.Get("asn"); ok && asn != as112ASN {
		return nil
	}
	if !redistributeImportsSourceIntoBGP(tree, "as112") {
		return nil
	}

	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}
	globalLocal, _ := nestedValue(bgp, "session", "asn", "local")

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	check := func(path string, group, peer *config.Tree) {
		remoteStr, ok := inheritedValue(group, peer, "session", "asn", "remote")
		if !ok || remoteStr == "" {
			return
		}
		local, ok := inheritedValue(group, peer, "session", "asn", "local")
		if !ok || local == "" {
			local = globalLocal
		}
		// Only an eBGP session leaks the origin outward; iBGP (local == remote)
		// keeps it internal. Skip when the local AS is undeterminable.
		if local == "" || local == remoteStr {
			return
		}
		remote, err := strconv.ParseUint(remoteStr, 10, 32)
		if err != nil {
			return
		}
		if isPrivateUseASN(uint32(remote)) { //nolint:gosec // bounded by ParseUint bitSize=32
			return
		}
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-as112-redistribute-origin-uncoordinated",
			Severity: diagnostic.SeverityWarning,
			Message: tb.Reset().Str(path).
				Str(": service as112 originates AS112 (asn 112) via 'import as112' toward the eBGP non-private ASN ").Str(remoteStr).
				Str(" -- this makes the node an uncoordinated global AS112 origin (RFC 7534 Section 3.2/Section 5); coordinate before deploying, restrict the route with an egress community/prefix filter, or set an operator/private asn").
				String(),
		})
	}

	for _, p := range bgp.GetListOrdered("peer") {
		check(tb.Reset().Str("bgp/peer/").Str(p.Key).String(), nil, p.Value)
	}
	for _, g := range bgp.GetListOrdered("group") {
		for _, p := range g.Value.GetListOrdered("peer") {
			check(tb.Reset().Str("bgp/group/").Str(g.Key).Str("/peer/").Str(p.Key).String(), g.Value, p.Value)
		}
	}
	return diags
}

// checkAS112RedistributeNotImported warns when service as112 is enabled and sets
// a redistribute-only knob (an explicit asn or a community) but no `redistribute
// { destination bgp { import as112 } }` exists. Those knobs only affect the
// BGP-originated covering prefixes (the YANG documents each as "Ignored unless
// import as112 is configured"), so setting one without the import is the silent
// misconfiguration where an operator expects the covering prefixes in BGP but
// the producer's events never reach the RIB. Advisory only: running as112 for
// DNS alone (no import, no redistribute knobs) is a valid, common deployment, so
// this fires ONLY when a redistribute-specific knob signals redistribution
// intent -- it never nags a DNS-only node.
func checkAS112RedistributeNotImported(tree *config.Tree) []diagnostic.Diagnostic {
	svc := tree.GetContainer("service")
	if svc == nil {
		return nil
	}
	as112 := svc.GetContainer("as112")
	if as112 == nil {
		return nil
	}
	if enabled, _ := as112.Get("enabled"); enabled != configTrueValue {
		return nil
	}
	_, hasASN := as112.Get("asn")
	hasCommunity := len(as112.GetSlice("community")) > 0
	if !hasASN && !hasCommunity {
		return nil // DNS-only node, nothing to redistribute
	}
	if redistributeImportsSourceIntoBGP(tree, "as112") {
		return nil // wired correctly
	}

	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     "doctor-as112-redistribute-not-imported",
		Severity: diagnostic.SeverityWarning,
		Message: tb.Reset().
			Str("service as112 sets a redistribute knob (asn/community) but 'redistribute { destination bgp { import as112 } }' is absent -- the AS112 covering prefixes will NOT be originated into BGP (the knob is ignored without the import); add the import, or remove the knob if this node serves DNS only").
			String(),
	}}
}

// isPrivateUseASN reports whether asn falls in an RFC 6996 Section 4
// Private Use range. Duplicated from filter_remove_private_as/private_as.go
// and reactor/filter_delta.go's identical checks rather than imported: a
// doctor check importing a BGP plugin package would violate the same
// layering this file's package doc explains.
func isPrivateUseASN(asn uint32) bool {
	return (asn >= 64512 && asn <= 65534) || (asn >= 4200000000 && asn <= 4294967294)
}
