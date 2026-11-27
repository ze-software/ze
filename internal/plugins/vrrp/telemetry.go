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
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

const (
	fixShowVRRP             = "ze show vrrp"
	subjectVRRPFallback     = "vrrp"
	stateEventWarnInterval  = time.Minute
	maxStateEventWarnGroups = 1024
)

func fitSubject(s string) string {
	if s == "" {
		return subjectVRRPFallback
	}
	if len(s) <= rpc.ReportSubjectMax {
		return s
	}
	cut := rpc.ReportSubjectMax
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

func groupSubject(spec GroupSpec) string {
	var tb textbuf.Buffer
	return tb.Str(spec.Interface).Byte('/').Str(spec.Name).Byte('/').Uint8(spec.VRID).String()
}

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
	metricsPtr         atomic.Pointer[engineMetrics]
	eventBusPtr        atomic.Pointer[ze.EventBus]
	issueQueuePtr      atomic.Pointer[issuequeue.Queue]
	stateEventWarnMu   sync.Mutex
	stateEventWarnLast = make(map[string]time.Time)
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

// setIssueQueue installs or clears the plugin's report-bus queue.
func setIssueQueue(q *issuequeue.Queue) {
	issueQueuePtr.Store(q)
	stateEventWarnMu.Lock()
	clear(stateEventWarnLast)
	stateEventWarnMu.Unlock()
}

func stateEventWarnDue(groupKey string) bool {
	now := time.Now()
	stateEventWarnMu.Lock()
	defer stateEventWarnMu.Unlock()
	if last, ok := stateEventWarnLast[groupKey]; ok {
		if now.Sub(last) < stateEventWarnInterval {
			return false
		}
	}
	if len(stateEventWarnLast) >= maxStateEventWarnGroups {
		for k, ts := range stateEventWarnLast {
			if now.Sub(ts) >= stateEventWarnInterval {
				delete(stateEventWarnLast, k)
			}
		}
		if len(stateEventWarnLast) >= maxStateEventWarnGroups {
			clear(stateEventWarnLast)
		}
	}
	stateEventWarnLast[groupKey] = now
	return true
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
		panic("BUG: vrrp: invalid FSM state")
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
	stateEventWarnMu.Lock()
	delete(stateEventWarnLast, groupSubject(spec))
	stateEventWarnMu.Unlock()
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
		groupKey := groupSubject(spec)
		if !stateEventWarnDue(groupKey) {
			return
		}
		subject := fitSubject(groupKey)
		msg := report.FitMessage("VRRP group " + groupKey + " transitioned from " + viewState(from) +
			" to " + viewState(to) + ", but publishing the state-change event on the daemon event bus failed; " +
			"downstream subscribers may miss this transition until the next state change. Cause: " + err.Error())
		failed := func(rErr error) {
			logger().Warn("VRRP could not deliver the state-change emit failure report to the daemon; check daemon health",
				"code", codeDoctorVRRPStateEventEmitFailed,
				"interface", spec.Interface, "group", spec.Name, "vrid", spec.VRID,
				"error", rErr, "fix", fixShowVRRP)
		}
		qErr := issueQueuePtr.Load().RaiseError(codeDoctorVRRPStateEventEmitFailed, subject, msg, fixShowVRRP, failed)
		logger().Warn("VRRP could not emit a state-change event; downstream subscribers may miss the transition until the next state change",
			"code", codeDoctorVRRPStateEventEmitFailed,
			"interface", spec.Interface, "group", spec.Name, "vrid", spec.VRID,
			"error", err, "fix", fixShowVRRP, issuequeue.Outcome(qErr))
	}
}

// unusedAddrGuard keeps the netip import honest if the file evolves; the
// eventbus payload deliberately carries no addresses (value types + small
// payloads cross the boundary better than address slices).
var _ = netip.Addr{}
