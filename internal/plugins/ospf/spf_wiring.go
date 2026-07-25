// Design: plan/learned/962-ospf-8-spf-rib.md -- engine to SPF wiring.
// This mirrors the IS-IS SPF wiring shape: LSDB changes trigger a debounced SPF
// run, SPF reads the LSDB through a narrow Source interface, and route install is
// owned by the SPF package via Loc-RIB insertion.

package ospf

import (
	"context"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routeinstall"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func (e *engine) initSPF() {
	if e.lsdb == nil {
		return
	}
	var strategy ospfspf.AFPrefixStrategy
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		// OSPFv3 family: decode the address-free v6 topology LSAs + the v6 prefix model
		// (Intra-Area-Prefix LSAs) and resolve next-hops from the adjacency table,
		// through the v6 strategy instead of the OSPFv2 default.
		strategy = v6Strategy{eng: e}
	}
	// RFC 5838: each address family installs into its own Loc-RIB family; the family is
	// derived from the engine's AF (Instance-ID range). This also corrects the IPv6-base
	// hardcode where the IPv6-unicast engine installed under family.IPv4Unicast.
	loc := locrib.Default()
	inst := ospfspf.NewInstallerFamily(loc, e.installFamily())
	if loc == nil {
		// Forked subprocess: no local Loc-RIB (default.go returns nil under
		// ze.plugin.hub.token). Ship SPF routes to the engine over RPC via the
		// route-install client set at engine start. (spec-forked-route-install)
		if client := routeInstallClient(); client != nil {
			inst.SetRemoteSink(routeinstall.New(context.Background(), client))
		}
	}
	e.spf = ospfspf.NewComputer(ospfspf.Config{
		Source:      e.lsdb,
		Resolver:    (*ospfNextHopResolver)(e),
		Installer:   inst,
		SummarySink: e.lsdb,
		Strategy:    strategy,
	})
	// Graceful Restart (spec-ospf-ext-9): suppress FIB install/removal while the restarter is
	// in-restart or performing a graceful stop, so SPF still computes but the pre-restart FIB
	// is retained (RFC 3623 sec 2/2.1).
	e.spf.SetInstallSuppress(e.gr.suppressInstall)
	e.lsdb.SetOnChange(e.triggerSPF)
	// Segment Routing (spec-ospf-ext-5): install SR MPLS labels from the post-SPF hook,
	// AFTER the IP-route Installer applied, so a label push rides an existing IP route
	// (R-8). The mpls-fib Source tag and the Explicit NULL label are address-family
	// specific; the install decision is shared.
	srFib := newSRFIB(getEventBus(), e.srSourceTag())
	e.srInstaller = newSRInstaller(srFib, e.srAF(), e.srExplicitNull())
	e.spf.SetPostRun(e.srInstallFromRoutes)
	// Adj-SID lifecycle (spec-ospf-ext-5 AC-12/AC-13): allocate from the SRLB on Full,
	// withdraw below. The allocator is seeded lazily from the resolved SRLB config.
	e.srAdj = &srAdjManager{fib: srFib, store: srWire, self: e.cfg.RouterID, labels: map[srAdjKey]srAdjRecord{}}
	// spec-ospf-ext-7: the SPF computer resolves each configured virtual link against its
	// transit area's intra-area result every run and calls back with the resolved set,
	// which drives the synthetic virtual interface up/down (RFC 2328 sec 16.1).
	e.spf.SetOnVirtualLinks(e.onVirtualLinksResolved)
	// spec-ospf-ext-6 TI-LFA: the IPv4 family resolves repair-list SR labels from ext-5's
	// Prefix-SID / Adj-SID maps. OSPFv3 SR label carriage (RFC 8666) is out of scope, so the
	// v6 engine gets base-LFA next-hop selection only (a nil resolver disables TI-LFA there).
	if e.dispatch == nil || !e.dispatch.codec.IsV6() {
		e.spf.SetSRResolver(srTILFAResolver{e: e})
	}
}

func (e *engine) configureSPF(cfg ospfConfig) {
	if e.spf == nil {
		return
	}
	e.spf.SetRoot(cfg.RouterID)
	areas := make([]types.AreaID, 0, len(cfg.Areas))
	areaConfigs := make([]ospfspf.AreaConfig, 0, len(cfg.Areas))
	for _, area := range cfg.Areas {
		areas = append(areas, area.AreaID)
		areaConfigs = append(areaConfigs, ospfspf.AreaConfig{
			AreaID:      area.AreaID,
			Options:     ospfOptionsForAreaType(string(area.AreaType)),
			Ranges:      spfRanges(area.Ranges),
			AreaType:    string(area.AreaType),
			NoSummary:   area.NoSummary,
			DefaultCost: area.DefaultCost,
		})
	}
	e.spf.SetAreas(areas)
	e.spf.SetAreaConfigs(areaConfigs)
	e.spf.SetMaxPaths(int(cfg.MaximumPaths))
	// spec-ospf-ext-6: thread the RFC 5286 / TI-LFA fast-reroute policy into the SPF
	// computer. Disabled leaves the route set + install byte-for-byte as today.
	mode := ospfspf.FastRerouteLFA
	if cfg.FastReroute.TILFA {
		mode = ospfspf.FastRerouteTILFA
	}
	e.spf.SetFastReroute(ospfspf.FastRerouteConfig{
		Enabled:        cfg.FastReroute.Enabled,
		Mode:           mode,
		NodeProtection: cfg.FastReroute.NodeProtection,
	})
	e.configureVirtualLinks(cfg)
	e.spf.SetTimers(
		time.Duration(cfg.Timers.SPFDelayMS)*time.Millisecond,
		time.Duration(cfg.Timers.SPFHoldMS)*time.Millisecond,
		time.Duration(cfg.Timers.SPFMaxHoldMS)*time.Millisecond,
	)
	e.spf.Trigger()
}

func (e *engine) triggerSPF(area types.AreaID) {
	if e.spf == nil {
		return
	}
	e.spf.TriggerArea(area)
}

// triggerAllSPF recomputes SPF for every configured area. Used on graceful-restart exit:
// once route install is no longer suppressed, SPF re-runs and the Installer re-programs the
// FIB, refreshing the retained RTPROT_ZE kernel routes (see gr_restarter.go exitRestart).
func (e *engine) triggerAllSPF() {
	if e.spf == nil {
		return
	}
	e.mu.Lock()
	areas := make([]types.AreaID, 0, len(e.areas))
	for id := range e.areas {
		areas = append(areas, id)
	}
	e.mu.Unlock()
	e.spf.Trigger() // backbone
	for _, id := range areas {
		e.spf.TriggerArea(id)
	}
}

func spfRanges(in []rangeConfig) []ospfspf.AreaRange {
	if len(in) == 0 {
		return nil
	}
	out := make([]ospfspf.AreaRange, 0, len(in))
	for _, r := range in {
		out = append(out, ospfspf.AreaRange{Prefix: r.Prefix, Advertise: r.Advertise, Cost: r.Cost, HasCost: r.HasCost})
	}
	return out
}

type ospfNextHopResolver engine

func (r *ospfNextHopResolver) ResolveInterface(addr netip.Addr) (string, bool) {
	if !addr.IsValid() {
		return "", false
	}
	e := (*engine)(r)
	if e.neighbors == nil {
		return "", false
	}
	want := addr.String()
	rows := e.neighbors.Snapshot()
	for i := range rows {
		if rows[i].Address == want && rows[i].State == neighborStateFull {
			return rows[i].Interface, true
		}
	}
	return "", false
}
