// Design: plan/spec-ospf-8-spf-rib.md -- engine to SPF wiring.
// This mirrors the IS-IS SPF wiring shape: LSDB changes trigger a debounced SPF
// run, SPF reads the LSDB through a narrow Source interface, and route install is
// owned by the SPF package via Loc-RIB insertion.

package ospf

import (
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/rib/locrib"
	ospfspf "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/spf"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
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
	e.spf = ospfspf.NewComputer(ospfspf.Config{
		Source:      e.lsdb,
		Resolver:    (*ospfNextHopResolver)(e),
		Installer:   ospfspf.NewInstaller(locrib.Default()),
		SummarySink: e.lsdb,
		Strategy:    strategy,
	})
	e.lsdb.SetOnChange(e.triggerSPF)
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
	for _, row := range e.neighbors.Snapshot() {
		if row.Address == want && row.State == "full" {
			return row.Interface, true
		}
	}
	return "", false
}
