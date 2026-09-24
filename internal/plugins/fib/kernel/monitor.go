// Design: docs/architecture/core-design.md -- FIB kernel route monitoring
// Overview: fibkernel.go -- FIB kernel plugin
// Related: backend.go -- backend abstraction
// Related: monitor_linux.go -- Linux netlink route monitor
// Related: monitor_other.go -- noop monitor for non-Linux platforms
//
// Platform-independent external route change handling.
// When the kernel route monitor detects an external change on a Ze-managed
// prefix, handleExternalChange attempts an ownership-checked reassertion and
// reports the result through (fib, external-change).
package fibkernel

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/core/routewatch"
	"github.com/ze-software/ze/internal/core/rtproto"
	fibevents "github.com/ze-software/ze/internal/plugins/fib/kernel/events"
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

// handleExternalChange restores a currently owned route after a matching kernel
// event. processEvent holds the same lock through the write and cache update,
// so notifications for withdrawn or superseded routes see the retired ownership.
func (f *fibKernel) handleExternalChange(ev routewatch.RouteEvent) {
	if rtproto.IsZe(ev.Protocol) {
		if ev.Protocol != rtproto.FIBKernel || ev.Action != routewatch.ActionRemove {
			return
		}
	}
	prefix := ev.Prefix.String()
	f.mu.Lock()
	zeNextHop, managed := f.installed[prefix]
	if !managed || f.stopped {
		f.mu.Unlock()
		return
	}
	change := f.forwarding[prefix]
	table, priority := change.TableID, change.Metric
	if table == 0 {
		table = 254 // Linux RT_TABLE_MAIN.
	}
	if change.Prefix.Addr().Is6() && priority == 0 {
		priority = 1024 // Linux's default priority for an IPv6 user route.
	}
	if ev.TableID != table || ev.Metric != priority {
		f.mu.Unlock()
		return
	}

	// Reassert all accepted forwarding metadata. The backend checks the current
	// kernel owner before replacement, including a foreign replacement.
	reassertErr := f.replaceChange(&change, prefix, f.asRichBackend())
	f.mu.Unlock()

	resolved := "reasserted"
	if reassertErr != nil {
		logger().Error("fib-kernel: re-assert failed", "prefix", prefix, "error", reassertErr)
		resolved = "failed"
	}
	var externalNextHop string
	if ev.NextHop.IsValid() {
		externalNextHop = ev.NextHop.String()
	}

	publishExternalChange(externalChangeEvent{
		Prefix:           prefix,
		Action:           "overwritten",
		ExternalProtocol: ev.Protocol,
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
	if _, err := eb.Emit(fibevents.Namespace, fibevents.EventExternalChange, string(payload)); err != nil {
		logger().Warn("fib-kernel: external-change emit failed", "error", err)
	}
}
