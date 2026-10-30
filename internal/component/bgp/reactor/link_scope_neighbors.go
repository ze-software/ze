// Design: docs/architecture/core-design.md — RFC 2545 Section 3 link-local next-hop condition
// RFC: rfc/short/rfc2545.md
// Overview: link_scope.go — the linkScope snapshot this view is part of

package reactor

import (
	"errors"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/network"
)

// errNeighborTableNotRead is the answer of a snapshot built without a neighbor
// read, which only a test builds; production snapshots always carry a read.
var errNeighborTableNotRead = errors.New("the neighbor table was not read for this session")

// neighborView is one read of the kernel neighbor table, kept beside the
// interface table in a linkScope snapshot. Exactly one of links and err is
// set by readNeighborView; the zero value is the not-read state, which
// linkLocalOf names rather than answering an empty table.
//
// Immutable after construction, so safe for concurrent use.
type neighborView struct {
	links *network.NeighborLinkLocals
	err   error
}

// readNeighborTable is the neighbor table read readNeighborView makes. It is
// network.ReadNeighbors; a test that drives the refresh paths replaces it and
// restores it in t.Cleanup, because the reader registry is a one-shot slot
// filled by the interface component's init().
var readNeighborTable = network.ReadNeighbors

// readNeighborView reads the neighbor table through internal/core/network. It
// reads the kernel, so it runs where a linkScope is built: at establishment,
// at reload, and on an interface or neighbor event, never per UPDATE.
func readNeighborView() neighborView {
	entries, err := readNeighborTable()
	if err != nil {
		return neighborView{err: err}
	}
	return neighborView{links: network.NewNeighborLinkLocals(entries)}
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

// equal reports whether two views answer every global the same way, so a
// refresh over an unchanged table can be skipped (refreshPeerLinkScopes). Two
// failed reads are equal when they failed the same way; an unread view equals
// nothing, so a peer that never read a table always rebuilds.
func (v neighborView) equal(other neighborView) bool {
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

// thirdPartyLinkLocalLog holds one window per lookup outcome plus one for an
// unread table. The set is closed, so a peer cannot grow it; the peer and next
// hop are named in the line that does get logged.
var thirdPartyLinkLocalLog struct {
	unread   logWindow
	outcomes [network.LinkLocalAmbiguous + 1]logWindow
}

// warnNeighborTableUnread says that a third-party next hop on the common
// subnet went out without its router's Link-Local because the neighbor table
// could not be read, and what makes it readable.
func warnNeighborTableUnread(peer, global netip.Addr, err error) {
	emit, suppressed := thirdPartyLinkLocalLog.unread.allow(time.Now().UnixNano(), thirdPartyLinkLocalLogInterval)
	if !emit {
		return
	}
	reactorLogger().Warn("RFC 2545 next hop sent without the link-local address of the router it names: the neighbor table cannot be read",
		"peer", peer, "next-hop", global, "error", err, "suppressed", suppressed,
		"fix", "run ze on Linux with the netlink interface backend: set interface backend netlink")
}

// warnThirdPartyLinkLocalUnknown says that the neighbor table does not name
// the Link-Local of the router holding global, why, and what resolves it.
func warnThirdPartyLinkLocalUnknown(peer, global netip.Addr, outcome network.LinkLocalOutcome) {
	window := &thirdPartyLinkLocalLog.unread
	if int(outcome) < len(thirdPartyLinkLocalLog.outcomes) {
		window = &thirdPartyLinkLocalLog.outcomes[outcome]
	}
	emit, suppressed := window.allow(time.Now().UnixNano(), thirdPartyLinkLocalLogInterval)
	if !emit {
		return
	}
	reactorLogger().Warn("RFC 2545 next hop sent without the link-local address of the router it names",
		"peer", peer, "next-hop", global, "reason", outcome.String(), "suppressed", suppressed,
		"fix", "the next-hop router must answer neighbor discovery on the shared link for its global and its link-local address; check with: show neighbor ipv6")
}
