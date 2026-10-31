// Design: docs/architecture/core-design.md — RFC 2545 Section 3 link-local next-hop condition
// RFC: rfc/short/rfc2545.md
// Overview: link_scope.go — the linkScope snapshot this view is part of
// Related: reactor_iface.go — neighborWatchWorker, which publishes the watch state a view carries

package reactor

import (
	"errors"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/network"
)

// errNeighborTableNotRead is the answer of a snapshot built without a neighbor
// read, which only a test builds; production snapshots always carry a read.
var errNeighborTableNotRead = errors.New("the neighbor table was not read for this session")

// errNeighborWatchNotStarted names the watch state of a view built by a
// reactor whose watch worker never published one, which only a test builds.
var errNeighborWatchNotStarted = errors.New("no neighbor table watch was started")

// neighborWatchStatus says whether the kernel neighbor table is watched, so a
// read of it is known to stay current. The zero value is not started, never
// running.
type neighborWatchStatus uint8

const (
	neighborWatchNotStarted neighborWatchStatus = iota
	// neighborWatchRunning: a watch is open and every change re-reads the table.
	neighborWatchRunning
	// neighborWatchStopped: no watch is open; err says why.
	neighborWatchStopped
)

// neighborWatchState is what neighborWatchWorker publishes. err is set only
// when status is neighborWatchStopped.
type neighborWatchState struct {
	status neighborWatchStatus
	err    error
}

// equal reports whether two states would log, and warn, the same way.
func (s neighborWatchState) equal(other neighborWatchState) bool {
	if s.status != other.status {
		return false
	}
	if s.err == nil {
		return other.err == nil
	}
	return other.err != nil && s.err.Error() == other.err.Error()
}

// reason names why the table is not watched, for a log line.
func (s neighborWatchState) reason() error {
	switch s.status {
	case neighborWatchStopped:
		return s.err
	case neighborWatchNotStarted:
		return errNeighborWatchNotStarted
	case neighborWatchRunning:
		panic("BUG: reason asked of a running neighbor watch")
	default:
		panic("BUG: invalid neighbor watch status")
	}
}

// NeighborTableFix is the operator's next step for a neighbor table that
// cannot be read because of err. The run-time warning and the bgp-neighbor-table
// doctor check (internal/component/bgp/config) both name it, so the two say the
// same thing.
func NeighborTableFix(err error) string {
	if errors.Is(err, network.ErrNeighborTableNotRegistered) {
		return "use a ze build that includes the interface component, which reads and watches the kernel neighbor table"
	}
	return "run ze on Linux with the netlink interface backend, the default when no interface block names one; check with: show neighbor ipv6"
}

// neighborWatchFix is the operator's next step for a neighbor table that is
// not watched because of err.
func neighborWatchFix(err error) string {
	if errors.Is(err, network.ErrNeighborTableNotRegistered) {
		return NeighborTableFix(err)
	}
	if errors.Is(err, errNeighborWatchEnded) {
		return "none if the watch starts again, which ze logs; if this repeats, look for the netlink error ze logged just before this line, and for kernel messages with: dmesg | grep -i -e netlink -e neigh"
	}
	return "if the interface backend is vpp, which reports no neighbor changes, this is a known limit: a router's link-local is read again at session establishment, at a config reload and on an interface address change; otherwise run ze on Linux with the netlink interface backend, the default when no interface block names one; ze tries the watch again at least once a minute"
}

// neighborView is one read of the kernel neighbor table, kept beside the
// interface table in a linkScope snapshot, with the state of the watch that
// says when to read it again. Exactly one of links and err is set by
// readNeighborView; the zero value is the not-read state, which linkLocalOf
// names rather than answering an empty table.
//
// Immutable after construction, so safe for concurrent use.
type neighborView struct {
	links *network.NeighborLinkLocals
	err   error
	// watch is the watch state when this view was read. A miss answered from
	// a table nobody watches may be stale, and its warning says so.
	watch neighborWatchState
}

// readNeighborTable is the neighbor table read readNeighborView makes. It is
// network.ReadNeighbors; a test that drives the refresh paths replaces it and
// restores it in t.Cleanup, because the table registry is a one-shot slot
// filled by the interface component's init().
var readNeighborTable = network.ReadNeighbors

// readNeighborView reads the neighbor table through internal/core/network and
// records watch, the state of the watch that keeps the read current. It reads
// the kernel, so it runs where a linkScope is built: at establishment, at
// reload, and on an interface event or a neighbor watch signal, never per
// UPDATE.
func readNeighborView(watch neighborWatchState) neighborView {
	entries, err := readNeighborTable()
	if err != nil {
		return neighborView{err: err, watch: watch}
	}
	return neighborView{links: network.NewNeighborLinkLocals(entries), watch: watch}
}

// linkLocalOf answers which Link-Local the router holding global uses, or the
// error that kept this view from reading the table.
func (v neighborView) linkLocalOf(global netip.Addr) (netip.Addr, network.LinkLocalOutcome, error) {
	if v.err != nil {
		return netip.Addr{}, network.LinkLocalOutcomeUnspecified, v.err
	}
	if v.links == nil {
		return netip.Addr{}, network.LinkLocalOutcomeUnspecified, errNeighborTableNotRead
	}
	linkLocal, outcome := v.links.LinkLocalOf(global)
	return linkLocal, outcome, nil
}

// equal reports whether two views answer every global the same way and carry
// the same watch state, so a refresh over an unchanged table can be skipped
// (refreshPeerLinkScopes). Two failed reads are equal when they failed the
// same way; an unread view equals nothing, so a peer that never read a table
// always rebuilds.
func (v neighborView) equal(other neighborView) bool {
	if !v.watch.equal(other.watch) {
		return false
	}
	if v.err != nil {
		return other.err != nil && v.err.Error() == other.err.Error()
	}
	if other.err != nil {
		return false
	}
	if v.links == nil {
		return false
	}
	if other.links == nil {
		return false
	}
	if v.links == other.links {
		return true
	}
	return v.links.Equal(other.links)
}

// thirdPartyLinkLocalLogInterval bounds each third-party Link-Local warning to
// one line per interval. The lookup runs per UPDATE, so an unbounded warning
// would be one line per route.
const thirdPartyLinkLocalLogInterval = 10 * time.Second

// thirdPartyLinkLocalLog holds one window per lookup outcome, one for an
// outcome outside that set, and one for a miss answered from a table nobody
// watches. The set is closed, so a peer cannot grow it; the peer and next hop
// are named in the line that does get logged. An unreadable table is not
// rate-limited here: it warns once per peer (warnNeighborTableUnread).
var thirdPartyLinkLocalLog struct {
	otherOutcome logWindow
	unwatched    logWindow
	outcomes     [network.LinkLocalAmbiguous + 1]logWindow
}

// warnNeighborTableUnread says that a third-party next hop on the common
// subnet went out without its router's Link-Local because the neighbor table
// could not be read, and what makes it readable. The cause is the host's, not
// a route's, so it holds for every route to the peer until it is fixed: the
// line is given once per peer (warned is the peer's flag), never once per
// UPDATE.
func warnNeighborTableUnread(warned *atomic.Bool, peer, global netip.Addr, err error) {
	if !warned.CompareAndSwap(false, true) {
		return
	}
	reactorLogger().Warn("RFC 2545 next hop sent without the link-local address of the router it names: the neighbor table cannot be read, so every such next hop to this peer goes out as the global address alone until it can",
		"peer", peer, "next-hop", global, "error", err,
		"fix", NeighborTableFix(err))
}

// warnThirdPartyLinkLocalUnknown says that the neighbor table does not name
// the Link-Local of the router holding global, why, and what resolves it.
// When the table is not watched the answer may be stale, so the line says
// that first and names what makes the watch run: blaming the next-hop router
// for an entry that appeared after the last read would send the operator to
// the wrong machine.
func warnThirdPartyLinkLocalUnknown(now int64, peer, global netip.Addr, outcome network.LinkLocalOutcome, watch neighborWatchState) {
	if watch.status != neighborWatchRunning {
		warnThirdPartyLinkLocalUnwatched(now, peer, global, outcome, watch)
		return
	}
	window := &thirdPartyLinkLocalLog.otherOutcome
	if int(outcome) < len(thirdPartyLinkLocalLog.outcomes) {
		window = &thirdPartyLinkLocalLog.outcomes[outcome]
	}
	emit, suppressed := window.allow(now, thirdPartyLinkLocalLogInterval)
	if !emit {
		return
	}
	reactorLogger().Warn("RFC 2545 next hop sent without the link-local address of the router it names",
		"peer", peer, "next-hop", global, "reason", outcome.String(), "suppressed", suppressed,
		"fix", "the next-hop router must answer neighbor discovery on the shared link for its global and its link-local address; check with: show neighbor ipv6")
}

// warnThirdPartyLinkLocalUnwatched is warnThirdPartyLinkLocalUnknown for a
// view read while no watch ran.
func warnThirdPartyLinkLocalUnwatched(now int64, peer, global netip.Addr, outcome network.LinkLocalOutcome, watch neighborWatchState) {
	emit, suppressed := thirdPartyLinkLocalLog.unwatched.allow(now, thirdPartyLinkLocalLogInterval)
	if !emit {
		return
	}
	reason := watch.reason()
	reactorLogger().Warn("RFC 2545 next hop sent without the link-local address of the router it names: the neighbor table is not watched, so the read this answer came from may be out of date",
		"peer", peer, "next-hop", global, "table-answer", outcome.String(), "watch-error", reason, "suppressed", suppressed,
		"fix", neighborWatchFix(reason))
}
