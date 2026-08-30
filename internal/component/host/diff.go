// Design: docs/architecture/host/observability.md -- hardware-change event detection
// Overview: inventory.go — Inventory struct and section types
// Related: cached.go — CachedDetector produces the snapshots diffed here

package host

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// DiffEvent describes a single hardware state change between two
// inventory snapshots. Callers translate DiffEvents into report bus
// issues (RaiseWarning / RaiseError) at the integration boundary.
type DiffEvent struct {
	Code    string
	Subject string
	Message string
	Detail  map[string]any
}

// DiffInventory compares prev and curr inventory snapshots and returns
// events for notable changes. Returns nil when both are nil or when
// nothing changed. First-ever snapshot (prev==nil) produces no events
// because there is no baseline to compare against.
func DiffInventory(prev, curr *Inventory) []DiffEvent {
	if prev == nil || curr == nil {
		return nil
	}

	events := make([]DiffEvent, 0, 4) //nolint:mnd // small pre-alloc for typical event count
	events = append(events, diffNICCarrier(prev.NICs, curr.NICs)...)
	events = append(events, diffECC(prev.Memory, curr.Memory)...)
	events = append(events, diffThrottle(prev.CPU, curr.CPU)...)
	return events
}

// diffNICCarrier reports carrier state changes for NICs present in both
// snapshots. NICs that appear or disappear between snapshots are ignored
// (hardware presence changes are out of scope for link-state diffing).
func diffNICCarrier(prev, curr []NICInfo) []DiffEvent {
	prevMap := make(map[string]bool, len(prev))
	for i := range prev {
		prevMap[prev[i].Name] = prev[i].Carrier
	}

	events := make([]DiffEvent, 0, len(curr))
	for i := range curr {
		nic := &curr[i]
		if prevCarrier, ok := prevMap[nic.Name]; ok && prevCarrier != nic.Carrier {
			state := "down"
			if nic.Carrier {
				state = "up"
			}
			events = append(events, DiffEvent{
				Code:    "carrier-change",
				Subject: nic.Name,
				Message: "NIC " + nic.Name + " carrier " + state,
				Detail:  map[string]any{"carrier": nic.Carrier},
			})
		}
	}
	return events
}

func diffECC(prev, curr *MemoryInfo) []DiffEvent {
	if prev == nil || curr == nil {
		return nil
	}
	if !prev.ECCPresent || !curr.ECCPresent {
		return nil
	}

	events := make([]DiffEvent, 0, 1)
	if curr.ECCCorrectableErrors > prev.ECCCorrectableErrors ||
		curr.ECCUncorrectableErrors > prev.ECCUncorrectableErrors {
		var b textbuf.Buffer
		events = append(events, DiffEvent{
			Code:    "ecc-error",
			Subject: "memory",
			Message: b.Reset().Str("ECC errors: correctable=").Uint(curr.ECCCorrectableErrors).Str(" uncorrectable=").Uint(curr.ECCUncorrectableErrors).String(),
			Detail: map[string]any{
				"correctable":   curr.ECCCorrectableErrors,
				"uncorrectable": curr.ECCUncorrectableErrors,
			},
		})
	}
	return events
}

func diffThrottle(prev, curr *CPUInfo) []DiffEvent {
	if prev == nil || curr == nil {
		return nil
	}

	prevMap := make(map[int]CoreInfo, len(prev.Cores))
	for _, c := range prev.Cores {
		prevMap[c.CPU] = c
	}

	events := make([]DiffEvent, 0, len(curr.Cores))
	for _, c := range curr.Cores {
		if pc, ok := prevMap[c.CPU]; ok {
			if c.CoreThrottleCount > pc.CoreThrottleCount ||
				c.PackageThrottleCount > pc.PackageThrottleCount {
				var b textbuf.Buffer
				events = append(events, DiffEvent{
					Code:    "throttle",
					Subject: textbuf.StrInt(cpuNamePrefix, int64(c.CPU)),
					Message: b.Reset().Str("CPU ").Int(int64(c.CPU)).Str(" throttle: core=").Int(int64(c.CoreThrottleCount)).Str(" pkg=").Int(int64(c.PackageThrottleCount)).String(),
					Detail: map[string]any{
						"cpu":              c.CPU,
						"core-throttle":    c.CoreThrottleCount,
						"package-throttle": c.PackageThrottleCount,
					},
				})
			}
		}
	}
	return events
}
