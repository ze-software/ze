// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- engine-owned telemetry and state events
//
// The engine owns the STATE series (what the virtual router is doing); the
// transport owns the WIRE series (what crossed the socket). Splitting them by
// owner keeps each counter next to the code that can actually increment it,
// which is how a metric avoids becoming a lie -- holo defines master_transitions
// and priority_zero_sent and never increments either (digest bug 9/10).
package vrrp

import (
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/pkg/ze"
)

// Namespace is this plugin's eventbus namespace.
const Namespace = "vrrp"

// EventStateChange is emitted on every virtual-router state transition.
const EventStateChange = "state-change"

// StateChange is the eventbus payload for a transition.
//
// Self-contained value types only: it crosses the plugin boundary, so no
// pointers into plugin-owned data (ai/rules/plugins.md Cross-Boundary
// Value Types).
type StateChange struct {
	Interface string `json:"interface"`
	Unit      string `json:"unit"`
	Device    string `json:"device"`
	Family    string `json:"family"`
	Group     string `json:"group"`
	VRID      uint8  `json:"vrid"`
	From      string `json:"from"`
	To        string `json:"to"`
	Reason    string `json:"reason"`
}

// stateChangeEvent is the typed handle. Declared at init so consumers can
// subscribe before the engine runs.
var stateChangeEvent = events.Register[StateChange](Namespace, EventStateChange)

// engineMetrics are the state-side series (the transport owns the wire series).
//
// Labels are (device, group, vrid, family), not the spec's original
// (interface, vrid, family): the logical interface is NOT unique per virtual
// router, because two units of one interface (eth0 and eth0.100) can each host
// vrid 10 in the same family, and both would have collapsed onto one series.
// `device` is the unit's OS device and IS unique; `group` is added so a
// dashboard can name the router the way the operator configured it.
type engineMetrics struct {
	state       metrics.GaugeVec   // ze_vrrp_state{device,group,vrid,family}
	transitions metrics.CounterVec // ze_vrrp_transitions_total{device,group,vrid,family,to}
}

var (
	metricsPtr  atomic.Pointer[engineMetrics]
	eventBusPtr atomic.Pointer[ze.EventBus]
)

// newEngineMetrics registers the state series. Names and labels come from the
// spec's Metrics table.
func newEngineMetrics(reg metrics.Registry) *engineMetrics {
	return &engineMetrics{
		state: reg.GaugeVec(
			"ze_vrrp_state",
			"Current VRRP state per virtual router: 0 initialize, 1 backup, 2 master.",
			[]string{"device", "group", "vrid", "family"},
		),
		transitions: reg.CounterVec(
			"ze_vrrp_transitions_total",
			"Total VRRP state transitions, by device, group, vrid, family and target state.",
			[]string{"device", "group", "vrid", "family", "to"},
		),
	}
}

// setMetricsRegistry installs the metrics registry (ConfigureMetrics hook).
func setMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		return
	}
	metricsPtr.Store(newEngineMetrics(reg))
}

// setEventBus installs the event bus (ConfigureEventBus hook).
func setEventBus(bus ze.EventBus) {
	if bus != nil {
		eventBusPtr.Store(&bus)
	}
}

// stateValue maps a state to its gauge value.
func stateValue(s fsm.State) float64 {
	switch s {
	case fsm.StateBackup:
		return 1
	case fsm.StateMaster:
		return 2
	case fsm.StateInitialize:
		return 0
	default:
		return 0
	}
}

// recordTransition updates the state gauge and the transition counter. Both are
// incremented on every transition, so neither can silently sit at zero.
func recordTransition(spec GroupSpec, to fsm.State) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	vrid := textbuf.StringUint8(spec.VRID)
	m.state.With(spec.ParentDevice, spec.Name, vrid, spec.Family).Set(stateValue(to))
	m.transitions.With(spec.ParentDevice, spec.Name, vrid, spec.Family, viewState(to)).Inc()
}

// clearMetrics drops an instance's series when it is torn down, so a deleted
// group stops reporting a stale state.
func clearMetrics(spec GroupSpec) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.state.Delete(spec.ParentDevice, spec.Name, textbuf.StringUint8(spec.VRID), spec.Family)
}

// publishStateChange emits the transition on the eventbus, if one is wired.
func publishStateChange(spec GroupSpec, from, to fsm.State, reason string) {
	busPtr := eventBusPtr.Load()
	if busPtr == nil {
		return
	}
	if _, err := stateChangeEvent.Emit(*busPtr, StateChange{
		Interface: spec.Interface,
		Unit:      spec.Unit,
		Device:    spec.ParentDevice,
		Family:    spec.Family,
		Group:     spec.Name,
		VRID:      spec.VRID,
		From:      viewState(from),
		To:        viewState(to),
		Reason:    reason,
	}); err != nil {
		logger().Debug("vrrp: emit state-change event failed",
			"interface", spec.Interface, "group", spec.Name, "vrid", spec.VRID, "error", err)
	}
}

// unusedAddrGuard keeps the netip import honest if the file evolves; the
// eventbus payload deliberately carries no addresses (value types + small
// payloads cross the boundary better than address slices).
var _ = netip.Addr{}
