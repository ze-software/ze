// Design: docs/architecture/core-design.md -- FIB dummy command answers
// Related: fibdummy.go -- held route table and atomic counters
// Related: register.go -- command declarations and RPC dispatch
package fibdummy

import (
	"cmp"
	"slices"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
)

const (
	forwardingNone = "none"
	programmedNo   = "no"
)

// routeRow is one row of the `show fib dummy` answer.
type routeRow struct {
	Table      uint32 `json:"table"`
	Prefix     string `json:"prefix"`
	Protocol   string `json:"protocol"`
	RouteType  string `json:"route-type,omitempty"`
	NextHop    string `json:"next-hop,omitempty"`
	Interface  string `json:"interface,omitempty"`
	Paths      int    `json:"paths"`
	Programmed string `json:"programmed"`
}

// countersSnapshot is the document answered by `show fib dummy counters`.
type countersSnapshot struct {
	Forwarding          string    `json:"forwarding"`
	RoutesHeld          int       `json:"routes-held"`
	Installs            uint64    `json:"installs"`
	Replaces            uint64    `json:"replaces"`
	Removals            uint64    `json:"removals"`
	InvalidChanges      uint64    `json:"invalid-changes"`
	LabelEntriesRefused uint64    `json:"label-entries-refused"`
	Since               time.Time `json:"since"`
}

// snapshotEntry pairs a table key with its held route value during show reads.
type snapshotEntry struct {
	key   routeKey
	route heldRoute
}

func formatRouteType(rt sysribevents.RouteType) string {
	switch rt {
	case 0, sysribevents.RouteTypeUnicast:
		return ""
	case sysribevents.RouteTypeBlackhole:
		return "blackhole"
	case sysribevents.RouteTypeUnreachable:
		return "unreachable"
	case sysribevents.RouteTypeProhibit:
		return "prohibit"
	default:
		// RouteType is open at plugin boundaries; omit unrecognized values.
		return ""
	}
}

// routeRows returns a deterministically sorted slice of all held routes.
// Every row carries Programmed: "no". Always returns a non-nil slice so JSON
// encodes [] rather than null when empty. Safe for concurrent use.
func (f *dummyFIB) routeRows() []routeRow {
	f.mu.RLock()
	entries := make([]snapshotEntry, 0, len(f.routes))
	for k, r := range f.routes {
		entries = append(entries, snapshotEntry{key: k, route: r})
	}
	f.mu.RUnlock()

	slices.SortFunc(entries, func(a, b snapshotEntry) int {
		if c := cmp.Compare(a.key.table, b.key.table); c != 0 {
			return c
		}
		if c := a.key.prefix.Addr().Compare(b.key.prefix.Addr()); c != 0 {
			return c
		}
		return cmp.Compare(a.key.prefix.Bits(), b.key.prefix.Bits())
	})

	rows := make([]routeRow, 0, len(entries))
	for _, e := range entries {
		nextHop := ""
		if e.route.nextHop.IsValid() {
			nextHop = e.route.nextHop.String()
		}
		rows = append(rows, routeRow{
			Table:      e.key.table,
			Prefix:     e.key.prefix.String(),
			Protocol:   e.route.protocol,
			RouteType:  formatRouteType(e.route.routeType),
			NextHop:    nextHop,
			Interface:  e.route.iface,
			Paths:      e.route.paths,
			Programmed: programmedNo,
		})
	}
	return rows
}

// counters returns a snapshot of the held route count and cumulative counters.
// Safe for concurrent use.
func (f *dummyFIB) counters() countersSnapshot {
	f.mu.RLock()
	held := len(f.routes)
	f.mu.RUnlock()

	return countersSnapshot{
		Forwarding:          forwardingNone,
		RoutesHeld:          held,
		Installs:            f.installs.Load(),
		Replaces:            f.replaces.Load(),
		Removals:            f.removals.Load(),
		InvalidChanges:      f.invalidChanges.Load(),
		LabelEntriesRefused: f.labelEntriesRefused.Load(),
		Since:               f.since,
	}
}
