// Design: plan/spec-host-1-observability.md — hardware-change event detection
// Overview: inventory.go — Inventory struct and section types
// Related: cached.go — CachedDetector produces the snapshots diffed here

package host

import "fmt"

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
				Message: fmt.Sprintf("NIC %s carrier %s", nic.Name, state),
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
		events = append(events, DiffEvent{
			Code:    "ecc-error",
			Subject: "memory",
			Message: fmt.Sprintf("ECC errors: correctable=%d uncorrectable=%d",
				curr.ECCCorrectableErrors, curr.ECCUncorrectableErrors),
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
				events = append(events, DiffEvent{
					Code:    "throttle",
					Subject: fmt.Sprintf("cpu%d", c.CPU),
					Message: fmt.Sprintf("CPU %d throttle: core=%d pkg=%d",
						c.CPU, c.CoreThrottleCount, c.PackageThrottleCount),
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
