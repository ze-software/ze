// RFC: rfc/short/rfc9568.md -- VRRPv3 (default for both address families)
// RFC: rfc/short/rfc3768.md -- VRRPv2 (opt-in per IPv4 group)
//
// Design: plan/learned/1124-vrrp-first-hop-redundancy.md -- vrrp plugin package doc, logger, show views
//
// Package vrrp implements the Virtual Router Redundancy Protocol on ze
// interfaces: RFC 9568 (VRRPv3, the default for both address families) and RFC
// 3768 (VRRPv2, opt-in per IPv4 group for keepalived interop).
//
// The plugin is the integration layer over four self-contained pieces:
//
//	packet/    wire codec + receive validation (pure)
//	fsm/       per-instance state machine, events in / actions out (pure)
//	transport/ raw proto-112 sockets, gratuitous ARP, unsolicited NA (linux)
//	iface      macvlan devices + virtual-address ownership (the iface component)
//
// Config lives under the interface unit that hosts the virtual router
// (`interface ethernet eth0 unit 0 ipv4 vrrp group 10 { ... }`), so the plugin
// shares the `interface` config root with the iface component and walks only
// the vrrp-bearing path of it.
//
// Virtual MAC handling is RFC-strict: each instance owns a macvlan device
// carrying 00:00:5e:00:01:{vrid} (IPv4) or 00:00:5e:00:02:{vrid} (IPv6), so a
// failover moves the MAC as well as the address and peers need not re-learn
// their ARP/ND caches.
package vrrp

import (
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
)

// loggerPtr is the package logger, disabled until the engine configures it.
// atomic.Pointer, not a plain var: functional tests run several in-process
// plugin instances concurrently and a plain var races (ai/patterns/plugin.md).
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())
}

func logger() *slog.Logger { return loggerPtr.Load() }

// setLogger installs the engine logger. Called from the registration's
// ConfigureEngineLogger hook.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// viewState renders an FSM state for the operator-facing surface.
//
// The FSM's String() is capitalized because the RFC names the states that way
// and its logs quote the RFC; show output follows ze's convention of lowercase
// state values instead (the iface component reports "up"/"down", iface.go:191).
func viewState(s fsm.State) string {
	switch s {
	case fsm.StateInitialize:
		return "initialize"
	case fsm.StateBackup:
		return "backup"
	case fsm.StateMaster:
		return "master"
	default:
		return "unknown"
	}
}

// instanceView is one virtual router as the show commands render it. Value
// types only: it crosses the plugin/CLI boundary as JSON
// (ai/rules/plugin-design.md Cross-Boundary Value Types).
type instanceView struct {
	Interface string `json:"interface"`
	Unit      string `json:"unit"`
	Family    string `json:"family"`
	// Group is the operator's config label; VRID is the wire identity. Both are
	// reported: the operator greps for the name they wrote, the protocol
	// engineer for the vrid the peers see.
	Group   string `json:"group"`
	VRID    uint8  `json:"vrid"`
	Device  string `json:"device"`
	State   string `json:"state"`
	Version uint8  `json:"version"`
	// Priority is the configured value; EffectivePriority is what the FSM runs
	// with, which differs for an address owner (RFC 9568 forces 255).
	Priority             uint8     `json:"priority"`
	EffectivePriority    uint8     `json:"effective-priority"`
	IsOwner              bool      `json:"is-owner"`
	Preempt              bool      `json:"preempt"`
	AcceptMode           bool      `json:"accept-mode"`
	ConfiguredIntervalMs int       `json:"configured-interval-milliseconds"`
	ActiveIntervalMs     int       `json:"active-interval-milliseconds"`
	VIPs                 []string  `json:"virtual-addresses"`
	LastAdvertSource     string    `json:"last-advertisement-source,omitempty"`
	Since                time.Time `json:"since"`
}

// statisticsView is one virtual router's counters. Timer-derived fields are
// microseconds, not milliseconds: a valid VRRPv3 skew time is sub-millisecond
// (prio 254 at a 10 ms interval is 78.125 us), so milliseconds would render the
// most interesting values as 0 (spec-vrrp-5 D-G).
type statisticsView struct {
	Interface              string            `json:"interface"`
	Unit                   string            `json:"unit"`
	Family                 string            `json:"family"`
	Group                  string            `json:"group"`
	VRID                   uint8             `json:"vrid"`
	State                  string            `json:"state"`
	AdvertsSent            uint64            `json:"advertisements-sent"`
	AdvertsReceived        uint64            `json:"advertisements-received"`
	PriorityZeroSent       uint64            `json:"priority-zero-sent"`
	PriorityZeroReceived   uint64            `json:"priority-zero-received"`
	AnnouncementsGARP      uint64            `json:"gratuitous-arps-sent"`
	AnnouncementsNA        uint64            `json:"neighbor-advertisements-sent"`
	PacketErrors           map[string]uint64 `json:"packet-errors,omitempty"`
	SkewTimeMicroseconds   int64             `json:"skew-time-microseconds"`
	MasterDownMicroseconds int64             `json:"master-down-interval-microseconds"`
}
