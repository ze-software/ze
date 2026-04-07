// Design: docs/architecture/core-design.md -- FIB kernel route monitoring
// Overview: fibkernel.go -- FIB kernel plugin
// Related: backend.go -- backend abstraction
// Related: monitor_linux.go -- Linux netlink route monitor
// Related: monitor_other.go -- noop monitor for non-Linux platforms
//
// Platform-independent external route change handling.
// When the kernel route monitor detects an external change on a ze-managed
// prefix, handleExternalChange re-asserts ze's route and emits
// (fib, external-change) on the EventBus for observability.
package fibkernel

import (
	"encoding/json"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// externalChangeEvent is the JSON payload for (fib, external-change).
type externalChangeEvent struct {
	Prefix           string `json:"prefix"`
	Action           string `json:"action"`
	ExternalProtocol int    `json:"external-protocol"`
	ExternalNextHop  string `json:"external-next-hop"`
	ZeNextHop        string `json:"ze-next-hop"`
	Resolved         string `json:"resolved"`
}

// handleExternalChange checks if an external route change affects a ze-managed prefix.
// If so, re-asserts ze's route and publishes a fib/external-change event.
// Called by the platform-specific route monitor (monitor_linux.go, monitor_other.go).
// Uses write lock for the entire read-check-replace to prevent TOCTOU races with processEvent.
func (f *fibKernel) handleExternalChange(prefix, externalNextHop string, externalProto int) {
	f.mu.Lock()
	zeNextHop, managed := f.installed[prefix]
	if !managed {
		f.mu.Unlock()
		return // Not our prefix, ignore.
	}

	// Re-assert ze's route under lock to prevent race with processEvent.
	reassertErr := f.backend.replaceRoute(prefix, zeNextHop)
	f.mu.Unlock()

	resolved := "reasserted"
	if reassertErr != nil {
		logger().Error("fib-kernel: re-assert failed", "prefix", prefix, "error", reassertErr)
		resolved = "failed"
	}

	// Always publish the external change event, even if re-assert failed,
	// so operators are aware of the conflict.
	publishExternalChange(externalChangeEvent{
		Prefix:           prefix,
		Action:           "overwritten",
		ExternalProtocol: externalProto,
		ExternalNextHop:  externalNextHop,
		ZeNextHop:        zeNextHop,
		Resolved:         resolved,
	})
}

// publishExternalChange emits a (fib, external-change) event on the EventBus.
func publishExternalChange(change externalChangeEvent) {
	eb := getEventBus()
	if eb == nil {
		return
	}

	payload, err := json.Marshal(change)
	if err != nil {
		logger().Warn("fib-kernel: marshal external change failed", "error", err)
		return
	}
	if _, err := eb.Emit(plugin.NamespaceFib, plugin.EventFibExternalChange, string(payload)); err != nil {
		logger().Warn("fib-kernel: external-change emit failed", "error", err)
	}
}
