// Design: plan/learned/936-isis-11-redistribution.md -- engine <-> redistribution wiring.
// Related: server.go -- the engine struct (prefixes / redistPrefixes) this extends
// Related: lsdb_wiring.go -- levelState merges connected + redistributed prefixes
// Related: spf_wiring.go -- the SPF Computer whose OnChange feeds the producer
// Related: internal/plugins/isis/redistribute -- the Consumer / Source / helpers
//
// RFC: rfc/short/rfc1195.md -- a passive interface's prefix is advertised (TLV 135) without forming an adjacency
//
// This file is the root-package glue between the IS-IS engine and the
// redistribution package (isis-11): it implements the LSPInjector seam the
// redistribution Consumer writes imported routes through, enumerates the node's
// own enabled/passive interface prefixes into its LSPs (connected-prefix
// advertisement, AC-8), and wires the SPF Computer's OnChange to the producer
// Source so SPF route changes are emitted as redistevents batches (export
// IS-IS -> BGP). None of this installs to the FIB (that is the spf Installer's
// Loc-RIB path); redistevents NEVER programs the kernel.

package isis

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	isisredistribute "github.com/ze-software/ze/internal/plugins/isis/redistribute"
)

// ---- LSPInjector implementation (consumer write seam) ----
//
// The engine satisfies isisredistribute.LSPInjector so the redistribution
// Consumer writes imported connected/static/BGP routes into the node's own LSPs as
// TLV 135 reachability and re-originates.

// OriginationLevels returns the LSDB levels the node originates own LSPs for, from
// the running config. With no config it returns nil (an idle engine imports
// nothing). Implements isisredistribute.LSPInjector.
func (e *engine) OriginationLevels() []lsdb.Level {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	if !cfg.Present() {
		return nil
	}
	return originationLevels(cfg.Level)
}

// SetRedistPrefix stores (adds or replaces) a redistributed TLV 135 prefix for a
// level. Held under the engine mutex; levelState merges these with connected
// prefixes on the next origination. Implements isisredistribute.LSPInjector.
func (e *engine) SetRedistPrefix(level lsdb.Level, info lsdb.PrefixInfo) {
	e.mu.Lock()
	if e.redistPrefixes == nil {
		e.redistPrefixes = make(map[lsdb.Level]map[netip.Prefix]lsdb.PrefixInfo)
	}
	if e.redistPrefixes[level] == nil {
		e.redistPrefixes[level] = make(map[netip.Prefix]lsdb.PrefixInfo)
	}
	e.redistPrefixes[level][info.Prefix] = info
	e.mu.Unlock()
}

// RemoveRedistPrefix removes a redistributed prefix for a level, reporting whether
// it existed. Implements isisredistribute.LSPInjector.
func (e *engine) RemoveRedistPrefix(level lsdb.Level, prefix netip.Prefix) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.redistPrefixes == nil || e.redistPrefixes[level] == nil {
		return false
	}
	_, ok := e.redistPrefixes[level][prefix]
	delete(e.redistPrefixes[level], prefix)
	return ok
}

// SetRedistPrefixV6 stores (adds or replaces) a redistributed TLV 236 IPv6 prefix
// for a level (isis-12). The IPv6 twin of SetRedistPrefix. Implements
// isisredistribute.LSPInjector.
func (e *engine) SetRedistPrefixV6(level lsdb.Level, info lsdb.PrefixInfoV6) {
	e.mu.Lock()
	if e.redistPrefixesV6 == nil {
		e.redistPrefixesV6 = make(map[lsdb.Level]map[netip.Prefix]lsdb.PrefixInfoV6)
	}
	if e.redistPrefixesV6[level] == nil {
		e.redistPrefixesV6[level] = make(map[netip.Prefix]lsdb.PrefixInfoV6)
	}
	e.redistPrefixesV6[level][info.Prefix] = info
	e.mu.Unlock()
}

// RemoveRedistPrefixV6 removes a redistributed IPv6 prefix for a level, reporting
// whether it existed (isis-12). Implements isisredistribute.LSPInjector.
func (e *engine) RemoveRedistPrefixV6(level lsdb.Level, prefix netip.Prefix) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.redistPrefixesV6 == nil || e.redistPrefixesV6[level] == nil {
		return false
	}
	_, ok := e.redistPrefixesV6[level][prefix]
	delete(e.redistPrefixesV6[level], prefix)
	return ok
}

// Originate re-originates the node's own LSP set across all configured levels. It
// always succeeds in v1 (origination is best-effort full regeneration that cannot
// fail structurally), so it returns nil; the error return exists so a future
// origination failure can be surfaced to the consumer instead of swallowed (R-3).
// It counts the re-origination per level on ze_isis_lsp_reoriginations_total
// (isis-11 OWNS this row): this method is the redistribution-driven re-origination
// path. Implements isisredistribute.LSPInjector.
func (e *engine) Originate() error {
	for _, level := range e.OriginationLevels() {
		e.lspReorigs.With(level.String()).Inc()
	}
	e.originate()
	return nil
}

// ---- connected-prefix advertisement (source/own-interface, AC-8) ----

// refreshConnectedPrefixes enumerates the node's own enabled and passive interface
// prefixes and advertises them as internal TLV 135 reachability at every
// origination level (AC-8). A passive interface forms no adjacency but its prefix
// is still advertised (RFC 1195). The prefix metric is the interface's configured
// metric. Called at circuit open/close and config apply; it replaces the connected
// set for each level, then the caller re-originates.
func (e *engine) refreshConnectedPrefixes() {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	if !cfg.Present() {
		return
	}

	// Build per-level connected prefix sets. An interface that is not enabled
	// advertises nothing; an enabled interface (passive or not) contributes its
	// connected IPv4 prefixes at the level it participates in.
	byLevel := map[lsdb.Level][]lsdb.PrefixInfo{}
	byLevelV6 := map[lsdb.Level][]lsdb.PrefixInfoV6{}
	for _, ic := range cfg.Interfaces {
		if !ic.Enabled {
			continue
		}
		prefixes := interfaceIPv4Prefixes(ic.Name)
		// IPv6 connected prefixes only on circuits that enable the IPv6 family
		// (TLV 236, isis-12); link-local prefixes are excluded at the source.
		var prefixesV6 []netip.Prefix
		if advertisesIPv6(ic) {
			prefixesV6 = interfaceIPv6Prefixes(ic.Name)
		}
		if len(prefixes) == 0 && len(prefixesV6) == 0 {
			continue
		}
		for _, level := range originationLevels(cfg.Level) {
			if !configFormsLevel(ic.Level, level) {
				continue
			}
			metric := levelMetric(ic, level)
			byLevel[level] = append(byLevel[level], isisredistribute.ConnectedPrefixInfos(prefixes, metric)...)
			byLevelV6[level] = append(byLevelV6[level], isisredistribute.ConnectedPrefixInfosV6(prefixesV6, metric)...)
		}
	}

	for _, level := range originationLevels(cfg.Level) {
		e.setPrefixes(level, byLevel[level])
		e.setPrefixesV6(level, byLevelV6[level])
	}
}

// interfaceIPv4Prefixes returns the named interface's connected IPv4 prefixes
// (network-masked), resolved via the iface resolver (logical-name aware). A
// missing interface or read error yields an empty slice (the interface advertises
// no connected prefix). IPv6 (TLV 236) is owned by isis-12, so only IPv4 prefixes
// are returned here.
func interfaceIPv4Prefixes(name string) []netip.Prefix {
	addrs, err := iface.Addresses(name)
	if err != nil {
		return nil
	}
	out := make([]netip.Prefix, 0, len(addrs))
	for _, a := range addrs {
		if a.Family != familyIPv4 {
			continue
		}
		addr, perr := netip.ParseAddr(a.Address)
		if perr != nil || !addr.Is4() {
			continue
		}
		p := netip.PrefixFrom(addr, a.PrefixLength)
		if !p.IsValid() {
			continue
		}
		out = append(out, p.Masked())
	}
	return out
}

// ---- producer wiring (SPF -> redistevents) ----

// wireRedistProducer connects the SPF Computer's OnChange callback to the
// redistribution producer Source so every SPF run that changes the route set emits
// a redistevents batch (export IS-IS -> BGP). A nil SPF Computer (early test) or
// nil bus leaves the producer a no-op. Called from OnStarted after the bus is
// wired.
func (e *engine) wireRedistProducer(src *isisredistribute.Source) {
	if e.spf == nil || src == nil {
		return
	}
	e.spf.SetOnChange(src.OnSPFChange)
	// IPv6 (isis-12): the IPv6 SPF delta is emitted as an AFI=2 redistevents batch.
	e.spf.SetOnChangeV6(src.OnSPFChangeV6)
}
