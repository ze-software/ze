// Design: docs/architecture/diagnostics/production-diagnostics.md -- interface read-only drift checker
// Related: config_apply.go -- desiredState and bindDevices
// Related: register.go -- plugin engine registration of OnDriftCheck

package iface

import (
	"maps"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/drift"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	driftStatePresent = "present"
	driftStateAbsent  = "absent"
)

// checkIfaceDrift compares the active interface configuration against live
// backend state without mutating links, addresses, backend lifecycle, or
// report warnings. Safe for concurrent use.
func checkIfaceDrift(cfg *ifaceConfig) rpc.DriftCheckOutput {
	if cfg == nil {
		return rpc.DriftCheckOutput{
			Subsystem: componentNameInterface,
			Status:    string(drift.StatusClean),
			Checked:   0,
			Findings:  []rpc.DriftFinding{},
		}
	}

	b := GetBackend()
	if b == nil {
		return rpc.DriftCheckOutput{
			Subsystem: componentNameInterface,
			Status:    string(drift.StatusUnavailable),
			Checked:   0,
			Findings:  []rpc.DriftFinding{},
		}
	}

	infos, err := b.ListInterfaces()
	if err != nil {
		return rpc.DriftCheckOutput{
			Subsystem: componentNameInterface,
			Status:    string(drift.StatusError),
			Checked:   0,
			Findings:  []rpc.DriftFinding{},
			Error:     err.Error(),
		}
	}

	devices := cfg.bindDevices(infos)
	desiredAddrs, managed, _ := cfg.desiredState(devices)
	desiredMTU := configuredDriftMTUs(cfg, devices)

	liveByName := make(map[string]InterfaceInfo, len(infos))
	for i := range infos {
		liveByName[infos[i].Name] = infos[i]
	}

	targetSet := make(map[string]struct{}, len(managed)+len(desiredAddrs)+len(desiredMTU))
	for name := range managed {
		targetSet[name] = struct{}{}
	}
	for name := range desiredAddrs {
		targetSet[name] = struct{}{}
	}
	for name := range desiredMTU {
		targetSet[name] = struct{}{}
	}
	names := slices.Sorted(maps.Keys(targetSet))

	findings := make([]rpc.DriftFinding, 0)
	drifted := 0
	record := func(finding rpc.DriftFinding) {
		drifted++
		if len(findings) < drift.MaxFindingsPerSubsystem {
			findings = append(findings, finding)
		}
	}

	for _, name := range names {
		live, exists := liveByName[name]
		if !exists {
			record(rpc.DriftFinding{
				Subject:  name,
				Kind:     string(drift.KindMissing),
				Expected: driftStatePresent,
				Actual:   driftStateAbsent,
			})
			continue
		}

		if cfgMTU, hasMTU := desiredMTU[name]; hasMTU {
			if live.MTU != cfgMTU {
				record(rpc.DriftFinding{
					Subject:  name + "/mtu",
					Kind:     string(drift.KindMismatch),
					Expected: formatDriftInt(cfgMTU),
					Actual:   formatDriftInt(live.MTU),
				})
			}
		}

		wantAddrs := desiredAddrs[name]
		if len(wantAddrs) == 0 {
			continue
		}
		liveAddrs := liveDriftCIDRSet(live.Addresses)
		for _, addr := range wantAddrs {
			if liveAddrs[normalizeDriftCIDR(addr)] {
				continue
			}
			record(rpc.DriftFinding{
				Subject:  name + "/" + addr,
				Kind:     string(drift.KindMissing),
				Expected: addr,
				Actual:   driftStateAbsent,
			})
		}
	}

	status := string(drift.StatusClean)
	if drifted > 0 {
		status = string(drift.StatusDrifted)
	}
	return rpc.DriftCheckOutput{
		Subsystem: componentNameInterface,
		Status:    status,
		Checked:   len(names),
		Drifted:   drifted,
		Truncated: drifted > drift.MaxFindingsPerSubsystem,
		Findings:  findings,
	}
}

func configuredDriftMTUs(cfg *ifaceConfig, devices map[string]string) map[string]int {
	mtus := make(map[string]int)
	for _, e := range allIfaceEntries(cfg) {
		if e.Disable {
			continue
		}
		if e.MTU <= 0 {
			continue
		}
		device, bound := deviceFor(devices, e.Name)
		if !bound {
			continue
		}
		mtus[device] = e.MTU
		for i := range e.Units {
			u := &e.Units[i]
			if u.Disable {
				continue
			}
			if u.VLANID <= 0 {
				continue
			}
			mtus[unitOSName(device, u)] = e.MTU
		}
	}
	return mtus
}

func liveDriftCIDRSet(addrs []AddrInfo) map[string]bool {
	set := make(map[string]bool, len(addrs))
	for i := range addrs {
		a := &addrs[i]
		var tb textbuf.Buffer
		raw := tb.Str(a.Address).Byte('/').Int(int64(a.PrefixLength)).String()
		set[normalizeDriftCIDR(raw)] = true
	}
	return set
}

func normalizeDriftCIDR(cidr string) string {
	p, err := netip.ParsePrefix(cidr)
	if err != nil {
		return cidr
	}
	return p.String()
}

func formatDriftInt(n int) string {
	var tb textbuf.Buffer
	return tb.Int(int64(n)).String()
}
