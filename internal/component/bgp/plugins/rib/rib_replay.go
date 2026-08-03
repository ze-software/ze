// Design: plan/learned/824-rib-feed-replay-batch.md — grouped collection and cursor replay
// RFC: rfc/short/rfc9494.md -- LLGR stale metadata on replay
// Overview: rib.go — replayRoutes, updateRoute
// Related: ribout_entry.go — ribOutEntry, reconstructRoute, pool.RibOut
// Related: ../cmd/update/cursor.go — handleUpdateCursor (engine side)
package rib

import (
	"encoding/hex"
	"hash/fnv"
	"net/netip"
	"sort"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// replayGroup represents a set of routes sharing the same attributes.
type replayGroup struct {
	Route      *Route
	Prefixes   []string
	MinMsgID   uint64
	Family     family.Family
	PathID     uint32
	StaleLevel uint8
}

// groupKey identifies a unique attribute group for replay batching.
type groupKey struct {
	Family     family.Family
	AttrHandle attrpool.Handle
	PathID     uint32
	StaleLevel uint8
}

// collectGroupedRibOutRoutes groups ribOut entries by (family, AttrHandle, pathID, StaleLevel).
// Each distinct AttrHandle is decoded once. Returns groups ready for replay.
func (r *RIBManager) collectGroupedRibOutRoutes(peerAddr netip.Addr) []replayGroup {
	return r.collectGroupedRibOutRoutesFiltered(peerAddr, family.Family{})
}

// collectPeerUpReplay returns the Adj-RIB-Out groups to re-advertise to a peer
// that has just come up. seenBefore reports whether this plugin had a state
// record for the peer BEFORE this event, i.e. whether the peer has ever been up.
//
// A peer's FIRST session starts with an empty Adj-RIB-Out. RFC 4271 Section 3.2
// defines Adj-RIB-Out as the routes selected for advertisement TO a peer, and a
// session that has only just been established has been advertised nothing. So an
// entry already present for a peer this plugin has never seen up cannot be
// history to re-advertise: it was recorded from a send made on THIS session, by a
// rail that has already put it on the wire. Replaying it puts a second copy of the
// same route out, and because the replay travels the announce rail (no RFC 4456
// reflection, no LLGR stale depreference) the two copies do not even agree --
// which is what test/plugin/llgr-readvertise-multipeer.ci caught.
//
// The window is a genuine event reordering, not a lost event: the sent event that
// records the entry and the state event that reports the session are produced by
// different goroutines, so under load the send is recorded first. Nothing is lost
// by declining -- the rail that sent it is the one delivering it.
//
// Caller must hold peerMu.
func (r *RIBManager) collectPeerUpReplay(peerAddr netip.Addr, seenBefore bool) []replayGroup {
	if seenBefore {
		return r.collectGroupedRibOutRoutes(peerAddr)
	}
	// Say it rather than decline silently (ai/rules/evidence.md): a
	// non-empty Adj-RIB-Out here means the reordering actually happened and a
	// duplicate was suppressed, which is invisible everywhere else.
	if n := len(r.ribOut[peerAddr]); n > 0 {
		logger().Warn("adj-rib-out replay skipped on a peer's first session: entries were recorded by this session's own sends",
			"peer", peerAddr, "families", n)
	}
	return nil
}

// collectGroupedRibOutRoutesForFamily is like collectGroupedRibOutRoutes but
// restricted to a single address family.
func (r *RIBManager) collectGroupedRibOutRoutesForFamily(peerAddr netip.Addr, fam family.Family) []replayGroup {
	return r.collectGroupedRibOutRoutesFiltered(peerAddr, fam)
}

// collectGroupedRibOutRoutesFiltered groups ribOut entries for replay.
// When filterFam is zero-value, all families are included.
//
// CONTRACT: returns nil when there is nothing to replay, which is the normal
// state of a fresh peer. Callers MUST NOT read nil-ness as "this peer did not
// come up" or "do not replay" -- an empty result is a legitimate answer and
// still requires replayRoutesWithCursor to run, because that is what emits the
// "plugin session ready" signal the reactor waits on. Test the emptiness you
// mean (len(groups) == 0), never the nil-ness of the return.
func (r *RIBManager) collectGroupedRibOutRoutesFiltered(peerAddr netip.Addr, filterFam family.Family) []replayGroup {
	peerFamilies := r.ribOut[peerAddr]
	if peerFamilies == nil {
		return nil
	}

	type pendingGroup struct {
		key      groupKey
		firstKey ribOutKey
		prefixes []string
		minMsgID uint64
		srcPeer  string
	}

	groups := make(map[groupKey]*pendingGroup)
	hasFilter := filterFam != (family.Family{})

	for fam, familyRoutes := range peerFamilies {
		if hasFilter && fam != filterFam {
			continue
		}
		for key, entry := range familyRoutes {
			gk := groupKey{
				Family:     fam,
				AttrHandle: entry.AttrHandle,
				PathID:     key.PathID,
				StaleLevel: entry.StaleLevel,
			}
			pg, ok := groups[gk]
			if !ok {
				src := r.ribOutSourcePeer(fam, key)
				pg = &pendingGroup{
					key:      gk,
					firstKey: key,
					minMsgID: entry.MsgID,
					srcPeer:  src,
				}
				groups[gk] = pg
			}
			pg.prefixes = append(pg.prefixes, key.Prefix.String())
			if entry.MsgID < pg.minMsgID {
				pg.minMsgID = entry.MsgID
			}
		}
	}

	if len(groups) == 0 {
		return nil
	}

	decoded := make(map[attrpool.Handle]*Route)
	result := make([]replayGroup, 0, len(groups))

	for _, pg := range groups {
		route, ok := decoded[pg.key.AttrHandle]
		if !ok {
			route = reconstructRoute(ribOutEntry{
				MsgID:      pg.minMsgID,
				AttrHandle: pg.key.AttrHandle,
			}, pg.key.Family, pg.firstKey, pg.srcPeer)
			decoded[pg.key.AttrHandle] = route
		}

		routeCopy := *route
		routeCopy.Family = pg.key.Family
		routeCopy.PathID = pg.key.PathID
		routeCopy.MsgID = pg.minMsgID
		routeCopy.StaleLevel = pg.key.StaleLevel

		result = append(result, replayGroup{
			Route:      &routeCopy,
			Prefixes:   pg.prefixes,
			MinMsgID:   pg.minMsgID,
			Family:     pg.key.Family,
			PathID:     pg.key.PathID,
			StaleLevel: pg.key.StaleLevel,
		})
	}

	return result
}

// sortGroupsForMinimalDeltas sorts replay groups to minimize attribute deltas.
func sortGroupsForMinimalDeltas(groups []replayGroup) {
	attrHashes := make([]uint64, len(groups))
	asPathStrs := make([]string, len(groups))
	for i := range groups {
		attrHashes[i] = attrHashSansASPath(groups[i].Route)
		asPathStrs[i] = asPathString(groups[i].Route.ASPath)
	}

	indices := make([]int, len(groups))
	for i := range indices {
		indices[i] = i
	}

	sort.SliceStable(indices, func(a, b int) bool {
		i, j := indices[a], indices[b]
		si, sj := groups[i].StaleLevel, groups[j].StaleLevel
		if si != sj {
			return si < sj
		}
		hi, hj := attrHashes[i], attrHashes[j]
		if hi != hj {
			return hi < hj
		}
		if asPathStrs[i] != asPathStrs[j] {
			return asPathStrs[i] < asPathStrs[j]
		}
		fi, fj := groups[i].Family, groups[j].Family
		if fi.AFI != fj.AFI {
			return fi.AFI < fj.AFI
		}
		if fi.SAFI != fj.SAFI {
			return fi.SAFI < fj.SAFI
		}
		return groups[i].PathID < groups[j].PathID
	})

	sorted := make([]replayGroup, len(groups))
	for i, idx := range indices {
		sorted[i] = groups[idx]
	}
	copy(groups, sorted)
}

// attrHashSansASPath hashes route attributes excluding AS_PATH.
func attrHashSansASPath(route *Route) uint64 {
	h := fnv.New64a()
	if route.Origin != nil {
		h.Write([]byte{byte(*route.Origin)}) //nolint:errcheck // hash.Write never errors
	}
	if route.MED != nil {
		var buf [4]byte
		buf[0] = byte(*route.MED >> 24)
		buf[1] = byte(*route.MED >> 16)
		buf[2] = byte(*route.MED >> 8)
		buf[3] = byte(*route.MED)
		h.Write(buf[:]) //nolint:errcheck // hash.Write never errors
	}
	if route.LocalPreference != nil {
		var buf [4]byte
		buf[0] = byte(*route.LocalPreference >> 24)
		buf[1] = byte(*route.LocalPreference >> 16)
		buf[2] = byte(*route.LocalPreference >> 8)
		buf[3] = byte(*route.LocalPreference)
		h.Write(buf[:]) //nolint:errcheck // hash.Write never errors
	}
	if route.NextHop != "" {
		h.Write([]byte(route.NextHop)) //nolint:errcheck // hash.Write never errors
	}
	for _, c := range route.Communities {
		var buf [4]byte
		v := uint32(c)
		buf[0] = byte(v >> 24)
		buf[1] = byte(v >> 16)
		buf[2] = byte(v >> 8)
		buf[3] = byte(v)
		h.Write(buf[:]) //nolint:errcheck // hash.Write never errors
	}
	for _, lc := range route.LargeCommunities {
		var buf [12]byte
		buf[0] = byte(lc.GlobalAdmin >> 24)
		buf[1] = byte(lc.GlobalAdmin >> 16)
		buf[2] = byte(lc.GlobalAdmin >> 8)
		buf[3] = byte(lc.GlobalAdmin)
		buf[4] = byte(lc.LocalData1 >> 24)
		buf[5] = byte(lc.LocalData1 >> 16)
		buf[6] = byte(lc.LocalData1 >> 8)
		buf[7] = byte(lc.LocalData1)
		buf[8] = byte(lc.LocalData2 >> 24)
		buf[9] = byte(lc.LocalData2 >> 16)
		buf[10] = byte(lc.LocalData2 >> 8)
		buf[11] = byte(lc.LocalData2)
		h.Write(buf[:]) //nolint:errcheck // hash.Write never errors
	}
	for _, ec := range route.ExtendedCommunities {
		h.Write(ec[:]) //nolint:errcheck // hash.Write never errors
	}
	return h.Sum64()
}

func asPathString(path []uint32) string {
	if len(path) == 0 {
		return ""
	}
	b := textbuf.Get()
	defer b.Release()
	for i, asn := range path {
		if i > 0 {
			b.Byte(' ')
		}
		b.Uint32(asn)
	}
	return b.String()
}

// replayRoutesWithCursor replays routes using cursor mode for efficiency.
func (r *RIBManager) replayRoutesWithCursor(peerAddr string, groups []replayGroup) {
	if len(groups) == 0 {
		r.dispatchPeerAction(peerAddr, "plugin session ready")
		return
	}

	// Clear any cursor state left over from a prior replay that aborted before
	// its terminating "update cursor done" (e.g. the peer dropped mid-replay,
	// which is best-effort and only logged). Without this reset, the first
	// group's full-attribute command merges against the stale cursor and
	// announces phantom attributes the new route does not carry. (I4)
	r.updateRoute(peerAddr, "update cursor done")

	sortGroupsForMinimalDeltas(groups)

	var prev *Route
	for i := range groups {
		g := &groups[i]
		cmds := formatCursorCommands(g, prev)
		for _, cmd := range cmds {
			r.updateRoute(peerAddr, cmd)
		}
		prev = g.Route
	}

	r.updateRoute(peerAddr, "update cursor done")
	r.dispatchPeerAction(peerAddr, "plugin session ready")
}

// resendRoutesWithCursor replays routes using cursor mode for manual resend.
// Unlike replayRoutesWithCursor, it does not send "plugin session ready" and
// carries stale metadata (RFC 9494) through updateRouteWithMeta for stale groups.
func (r *RIBManager) resendRoutesWithCursor(peerAddr string, groups []replayGroup) int {
	if len(groups) == 0 {
		return 0
	}

	// Reset stale cursor state from a prior aborted replay before the first
	// group merges against it (see replayRoutesWithCursor). (I4)
	r.updateRoute(peerAddr, "update cursor done")

	sortGroupsForMinimalDeltas(groups)

	total := 0
	var prev *Route
	for i := range groups {
		g := &groups[i]
		cmds := formatCursorCommands(g, prev)
		if g.StaleLevel > 0 {
			meta := map[string]any{"stale": g.StaleLevel}
			for _, cmd := range cmds {
				r.updateRouteWithMeta(peerAddr, cmd, meta)
			}
		} else {
			for _, cmd := range cmds {
				r.updateRoute(peerAddr, cmd)
			}
		}
		total += len(g.Prefixes)
		prev = g.Route
	}

	r.updateRoute(peerAddr, "update cursor done")
	return total
}

const maxNLRIBytesPerCommand = 4000

// formatCursorCommands builds one or more cursor commands for a replay group.
func formatCursorCommands(g *replayGroup, prev *Route) []string {
	var b textbuf.Buffer
	b.Str("update cursor ")

	if prev == nil {
		appendAllAttrs(&b, g.Route)
	} else {
		appendAttrDelta(&b, prev, g.Route)
	}

	var tb textbuf.Buffer
	tb.Str("nlri ").Str(g.Family.String())
	if g.PathID != 0 {
		tb.Str(" path-information ").Uint32(g.PathID)
	}
	tb.Str(" add")
	nlriSuffix := tb.String()

	header := b.String()

	totalSize := len(header) + len(nlriSuffix)
	for _, p := range g.Prefixes {
		totalSize += 1 + len(p)
	}

	if totalSize <= maxNLRIBytesPerCommand || len(g.Prefixes) == 1 {
		// header was extracted via b.String(), which detaches the heap-backed
		// buffer and leaves b empty; rebuild from header rather than appending
		// to b (textbuf.String() is not a non-destructive snapshot like
		// strings.Builder.String()).
		b.Reset().Str(header).Str(nlriSuffix)
		for _, p := range g.Prefixes {
			b.Byte(' ').Str(p)
		}
		return []string{b.String()}
	}

	var commands []string
	var cmdBuf textbuf.Buffer
	cmdBuf.Str(header).Str(nlriSuffix)
	currentSize := len(header) + len(nlriSuffix)
	count := 0

	for _, p := range g.Prefixes {
		entryLen := 1 + len(p)
		if count > 0 && currentSize+entryLen > maxNLRIBytesPerCommand {
			commands = append(commands, cmdBuf.String())
			cmdBuf.Reset().Str("update cursor ").Str(nlriSuffix)
			currentSize = len("update cursor ") + len(nlriSuffix)
			count = 0
		}
		cmdBuf.Byte(' ').Str(p)
		currentSize += entryLen
		count++
	}

	if count > 0 {
		commands = append(commands, cmdBuf.String())
	}
	return commands
}

func appendAllAttrs(b *textbuf.Buffer, route *Route) {
	if route.Origin != nil {
		b.Str("origin ")
		b.Str(route.Origin.LowerString())
		b.Byte(' ')
	}
	if len(route.ASPath) > 0 {
		b.Str("as-path [")
		for i, asn := range route.ASPath {
			if i > 0 {
				b.Byte(' ')
			}
			b.Str(textbuf.StringUint32(asn))
		}
		b.Str("] ")
	}
	if route.MED != nil {
		b.Str("med ")
		b.Str(textbuf.StringUint32(*route.MED))
		b.Byte(' ')
	}
	if route.LocalPreference != nil {
		b.Str("local-preference ")
		b.Str(textbuf.StringUint32(*route.LocalPreference))
		b.Byte(' ')
	}
	if len(route.Communities) > 0 {
		b.Str("community [")
		for i, c := range route.Communities {
			if i > 0 {
				b.Byte(' ')
			}
			b.Str(c.String())
		}
		b.Str("] ")
	}
	if len(route.LargeCommunities) > 0 {
		b.Str("large-community [")
		for i, lc := range route.LargeCommunities {
			if i > 0 {
				b.Byte(' ')
			}
			b.Str(lc.String())
		}
		b.Str("] ")
	}
	if len(route.ExtendedCommunities) > 0 {
		b.Str("extended-community [")
		for i, ec := range route.ExtendedCommunities {
			if i > 0 {
				b.Byte(' ')
			}
			b.Str(hex.EncodeToString(ec[:]))
		}
		b.Str("] ")
	}
	if route.NextHop != "" {
		b.Str("next-hop ")
		b.Str(route.NextHop)
		b.Byte(' ')
	}
}

func appendAttrDelta(b *textbuf.Buffer, prev, curr *Route) {
	if !originEqual(prev.Origin, curr.Origin) {
		if curr.Origin != nil {
			b.Str("origin ")
			b.Str(curr.Origin.LowerString())
			b.Byte(' ')
		} else {
			b.Str("del origin ")
		}
	}
	if !asPathEqual(prev.ASPath, curr.ASPath) {
		if len(curr.ASPath) > 0 {
			b.Str("as-path [")
			for i, asn := range curr.ASPath {
				if i > 0 {
					b.Byte(' ')
				}
				b.Str(textbuf.StringUint32(asn))
			}
			b.Str("] ")
		} else {
			b.Str("del as-path ")
		}
	}
	if !uint32PtrEqual(prev.MED, curr.MED) {
		if curr.MED != nil {
			b.Str("med ")
			b.Str(textbuf.StringUint32(*curr.MED))
			b.Byte(' ')
		} else {
			b.Str("del med ")
		}
	}
	if !uint32PtrEqual(prev.LocalPreference, curr.LocalPreference) {
		if curr.LocalPreference != nil {
			b.Str("local-preference ")
			b.Str(textbuf.StringUint32(*curr.LocalPreference))
			b.Byte(' ')
		} else {
			b.Str("del local-preference ")
		}
	}
	if !communitiesEqual(prev.Communities, curr.Communities) {
		if len(curr.Communities) > 0 {
			b.Str("community [")
			for i, c := range curr.Communities {
				if i > 0 {
					b.Byte(' ')
				}
				b.Str(c.String())
			}
			b.Str("] ")
		} else {
			b.Str("del community ")
		}
	}
	if !largeCommunitiesEqual(prev.LargeCommunities, curr.LargeCommunities) {
		if len(curr.LargeCommunities) > 0 {
			b.Str("large-community [")
			for i, lc := range curr.LargeCommunities {
				if i > 0 {
					b.Byte(' ')
				}
				b.Str(lc.String())
			}
			b.Str("] ")
		} else {
			b.Str("del large-community ")
		}
	}
	if !extCommunitiesEqual(prev.ExtendedCommunities, curr.ExtendedCommunities) {
		if len(curr.ExtendedCommunities) > 0 {
			b.Str("extended-community [")
			for i, ec := range curr.ExtendedCommunities {
				if i > 0 {
					b.Byte(' ')
				}
				b.Str(hex.EncodeToString(ec[:]))
			}
			b.Str("] ")
		} else {
			b.Str("del extended-community ")
		}
	}
	if prev.NextHop != curr.NextHop {
		if curr.NextHop != "" {
			b.Str("next-hop ")
			b.Str(curr.NextHop)
			b.Byte(' ')
		} else {
			b.Str("del next-hop ")
		}
	}
}

func originEqual(a, b *attribute.Origin) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func asPathEqual(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func uint32PtrEqual(a, b *uint32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func communitiesEqual(a, b []attribute.Community) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func largeCommunitiesEqual(a, b []attribute.LargeCommunity) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func extCommunitiesEqual(a, b []attribute.ExtendedCommunity) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
