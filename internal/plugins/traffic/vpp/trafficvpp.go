// Design: docs/architecture/core-design.md -- VPP traffic control backend

// Package trafficvpp implements the ze traffic-control Backend interface on
// top of VPP's binary API via GoVPP. It is registered under the name "vpp"
// alongside the netlink backend.
//
// Current scope: HTB and TBF qdiscs translate to a single VPP policer per
// interface bound to interface egress via PolicerOutput. Multi-class
// configurations, every other qdisc type, and every filter type are
// rejected at OnConfigVerify via traffic.RegisterVerifier per
// `rules/exact-or-reject.md`. Filter support and multi-class class-based
// shaping are deferred to follow-up specs that design the missing VPP
// pipelines (classify-table attachment, QoS record+mark chain); see
// plan/deferrals.md for the destination specs.
package trafficvpp

import (
	"log/slog"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// loggerPtr is the package-level logger. Defaults to the "traffic.vpp"
// slog subsystem so reconciliation warnings and other diagnostics reach
// the operator through ze's normal logging surface. Stored atomically
// so a future consumer can swap handlers without races.
var loggerPtr atomic.Pointer[slog.Logger]

func init() { //nolint:gochecknoinits // logger bootstrap only
	loggerPtr.Store(slogutil.Logger("traffic.vpp"))
}

// logger returns the package-level slog.Logger, never nil.
func logger() *slog.Logger { return loggerPtr.Load() }
