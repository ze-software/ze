// Design: plan/spec-cp-survival-5-detect-5-characterization.md -- Stage-2 characterization.
// Phase 1 ("Unblock"): on the rate-trigger active transition, query on-box flow
// data for the attack's target prefix and emit a populated AttackDetected so the
// responders (which gate on a valid DstPrefix) act. Proto/ports/flags/family and
// the flowexport tap are later phases.

package detect

import (
	"context"
	"encoding/json"
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// characterizeTimeout bounds the on-trigger flow query so a slow or wedged data
// source cannot stall characterization. Phase 1 uses a constant; a tuning leaf
// is a later-phase concern.
const characterizeTimeout = 2 * time.Second

// statusDone mirrors plugin.StatusDone ("done"). Duplicated as a local constant
// to keep the detector runtime path free of an internal/component/plugin import.
const statusDone = "done"

// dispatchFunc is the engine-routed command dispatcher (Plugin.DispatchCommand).
// Injected into the detector so tests can stub the trafficusage response without
// a live engine.
type dispatchFunc func(ctx context.Context, command string) (status string, data json.RawMessage, err error)

// characterizeAndEmit runs once per attack on its own goroutine (off the onRate
// hot path). It queries trafficusage for the attacked interface's dominant
// destination, fills the target prefix, and emits AttackDetected. When no source
// is reachable it falls back to an empty target with FamilyGenericFlood -- the
// pre-Phase-1 behavior, never worse than before.
func (d *detector) characterizeAndEmit(gen uint64, ifaceName string, peakPps, peakBps float64) {
	ctx, cancel := context.WithTimeout(context.Background(), characterizeTimeout)
	defer cancel()

	target := ddosevent.VectorTuple{}
	if prefix, ok := d.characterizeTarget(ctx, ifaceName); ok {
		target.DstPrefix = prefix
	}

	// Skip a stale emit: if the attack cleared (or a newer attack started) while
	// we were querying, attackGen has advanced. Emitting Detected now would install
	// a mitigation with no matching Cleared to remove it (max-mitigation-duration is
	// not enforced in ddos-local), leaving a stuck drop.
	d.mu.Lock()
	stale := d.attackGen != gen
	d.mu.Unlock()
	if stale {
		return
	}

	if _, err := ddosevent.Detected.Emit(d.bus, &ddosevent.AttackDetected{
		Interface:  ifaceName,
		Target:     target,
		Family:     ddosevent.FamilyGenericFlood,
		PeakRxPps:  peakPps,
		PeakRxBps:  peakBps,
		Observable: true,
	}); err != nil {
		logger().Warn("ddos-detect: emit detected failed", "error", err)
	}
	// Detected has now reached subscribers; allow onRate to emit Ongoing.
	d.detectedEmitted.Store(true)
}

// characterizeTarget asks trafficusage for the attacked interface and returns its
// dominant destination as a host prefix. ok is false when no dispatch is wired,
// the source is absent/errors, or it reports no destination -- the caller then
// emits the generic-flood fallback.
func (d *detector) characterizeTarget(ctx context.Context, ifaceName string) (netip.Prefix, bool) {
	if d.dispatch == nil {
		return netip.Prefix{}, false
	}

	var tb textbuf.Buffer
	tb.Str("show traffic-usage name ").Str(ifaceName)

	status, data, err := d.dispatch(ctx, tb.String())
	if err != nil || status != statusDone {
		d.sourceAbsentOnce.Do(func() {
			logger().Warn("ddos-detect: flow source unavailable; characterization falling back to generic-flood",
				"command", "show traffic-usage", "status", status, "error", err)
		})
		return netip.Prefix{}, false
	}
	return parseTopDestination(data)
}

// parseTopDestination picks the highest-byte destination from a
// "show traffic-usage name <iface>" response and returns it as a host prefix
// (/32 for IPv4, /128 for IPv6). trafficusage records destinations in egress-ips
// and is IPv4-only today; IPv6 targets arrive with the flowexport tap in a later
// phase. Malformed or empty input yields ok=false (caller falls back).
func parseTopDestination(data json.RawMessage) (netip.Prefix, bool) {
	var resp struct {
		EgressIPs []struct {
			IP    string `json:"ip"`
			Bytes uint64 `json:"bytes"`
		} `json:"egress-ips"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return netip.Prefix{}, false
	}

	var bestIP string
	var bestBytes uint64
	found := false
	for _, e := range resp.EgressIPs {
		if e.IP == "" {
			continue
		}
		if !found || e.Bytes > bestBytes {
			found = true
			bestBytes = e.Bytes
			bestIP = e.IP
		}
	}
	if !found {
		return netip.Prefix{}, false
	}

	addr, err := netip.ParseAddr(bestIP)
	if err != nil {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addr, addr.BitLen()), true
}
