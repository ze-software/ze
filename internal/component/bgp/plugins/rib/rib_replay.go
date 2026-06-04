// Design: plan/spec-rib-feed-replay-batch.md — grouped collection and cursor replay
// Overview: rib.go — replayRoutes, updateRoute
// Related: ribout_entry.go — ribOutEntry, reconstructRoute, pool.RibOut
// Related: ../cmd/update/cursor.go — handleUpdateCursor (engine side)
package rib

import (
	"encoding/hex"
	"hash/fnv"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
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
func (r *RIBManager) collectGroupedRibOutRoutes(peerAddr string) []replayGroup {
	return r.collectGroupedRibOutRoutesFiltered(peerAddr, family.Family{})
}

// collectGroupedRibOutRoutesForFamily is like collectGroupedRibOutRoutes but
// restricted to a single address family.
func (r *RIBManager) collectGroupedRibOutRoutesForFamily(peerAddr string, fam family.Family) []replayGroup {
	return r.collectGroupedRibOutRoutesFiltered(peerAddr, fam)
}

// collectGroupedRibOutRoutesFiltered groups ribOut entries for replay.
// When filterFam is zero-value, all families are included.
func (r *RIBManager) collectGroupedRibOutRoutesFiltered(peerAddr string, filterFam family.Family) []replayGroup {
	peerFamilies := r.ribOut[peerAddr]
	if peerFamilies == nil {
		return nil
	}

	type pendingGroup struct {
		key      groupKey
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
			prefix, pathID := parseOutRouteKey(key)
			gk := groupKey{
				Family:     fam,
				AttrHandle: entry.AttrHandle,
				PathID:     pathID,
				StaleLevel: entry.StaleLevel,
			}
			pg, ok := groups[gk]
			if !ok {
				src := r.ribOutSourcePeer(fam, key)
				pg = &pendingGroup{
					key:      gk,
					minMsgID: entry.MsgID,
					srcPeer:  src,
				}
				groups[gk] = pg
			}
			pg.prefixes = append(pg.prefixes, prefix)
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
			}, pg.key.Family, pg.prefixes[0], pg.srcPeer)
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
	var b strings.Builder
	b.WriteString("update cursor ")

	if prev == nil {
		appendAllAttrs(&b, g.Route)
	} else {
		appendAttrDelta(&b, prev, g.Route)
	}

	familyStr := g.Family.String()
	nlriSuffix := "nlri " + familyStr
	if g.PathID != 0 {
		nlriSuffix += " path-information " + textbuf.Uint32(g.PathID)
	}
	nlriSuffix += " add"

	header := b.String()

	totalSize := len(header) + len(nlriSuffix)
	for _, p := range g.Prefixes {
		totalSize += 1 + len(p)
	}

	if totalSize <= maxNLRIBytesPerCommand || len(g.Prefixes) == 1 {
		b.WriteString(nlriSuffix)
		for _, p := range g.Prefixes {
			b.WriteByte(' ')
			b.WriteString(p)
		}
		return []string{b.String()}
	}

	var commands []string
	cmdBuf := strings.Builder{}
	cmdBuf.WriteString(header)
	cmdBuf.WriteString(nlriSuffix)
	currentSize := len(header) + len(nlriSuffix)
	count := 0

	for _, p := range g.Prefixes {
		entryLen := 1 + len(p)
		if count > 0 && currentSize+entryLen > maxNLRIBytesPerCommand {
			commands = append(commands, cmdBuf.String())
			cmdBuf.Reset()
			cmdBuf.WriteString("update cursor ")
			cmdBuf.WriteString(nlriSuffix)
			currentSize = len("update cursor ") + len(nlriSuffix)
			count = 0
		}
		cmdBuf.WriteByte(' ')
		cmdBuf.WriteString(p)
		currentSize += entryLen
		count++
	}

	if count > 0 {
		commands = append(commands, cmdBuf.String())
	}
	return commands
}

func appendAllAttrs(b *strings.Builder, route *Route) {
	if route.Origin != nil {
		b.WriteString("origin ")
		b.WriteString(route.Origin.LowerString())
		b.WriteByte(' ')
	}
	if len(route.ASPath) > 0 {
		b.WriteString("as-path [")
		for i, asn := range route.ASPath {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(textbuf.Uint32(asn))
		}
		b.WriteString("] ")
	}
	if route.MED != nil {
		b.WriteString("med ")
		b.WriteString(textbuf.Uint32(*route.MED))
		b.WriteByte(' ')
	}
	if route.LocalPreference != nil {
		b.WriteString("local-preference ")
		b.WriteString(textbuf.Uint32(*route.LocalPreference))
		b.WriteByte(' ')
	}
	if len(route.Communities) > 0 {
		b.WriteString("community [")
		for i, c := range route.Communities {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(c.String())
		}
		b.WriteString("] ")
	}
	if len(route.LargeCommunities) > 0 {
		b.WriteString("large-community [")
		for i, lc := range route.LargeCommunities {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(lc.String())
		}
		b.WriteString("] ")
	}
	if len(route.ExtendedCommunities) > 0 {
		b.WriteString("extended-community [")
		for i, ec := range route.ExtendedCommunities {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(hex.EncodeToString(ec[:]))
		}
		b.WriteString("] ")
	}
	if route.NextHop != "" {
		b.WriteString("next-hop ")
		b.WriteString(route.NextHop)
		b.WriteByte(' ')
	}
}

func appendAttrDelta(b *strings.Builder, prev, curr *Route) {
	if !originEqual(prev.Origin, curr.Origin) {
		if curr.Origin != nil {
			b.WriteString("origin ")
			b.WriteString(curr.Origin.LowerString())
			b.WriteByte(' ')
		} else {
			b.WriteString("del origin ")
		}
	}
	if !asPathEqual(prev.ASPath, curr.ASPath) {
		if len(curr.ASPath) > 0 {
			b.WriteString("as-path [")
			for i, asn := range curr.ASPath {
				if i > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(textbuf.Uint32(asn))
			}
			b.WriteString("] ")
		} else {
			b.WriteString("del as-path ")
		}
	}
	if !uint32PtrEqual(prev.MED, curr.MED) {
		if curr.MED != nil {
			b.WriteString("med ")
			b.WriteString(textbuf.Uint32(*curr.MED))
			b.WriteByte(' ')
		} else {
			b.WriteString("del med ")
		}
	}
	if !uint32PtrEqual(prev.LocalPreference, curr.LocalPreference) {
		if curr.LocalPreference != nil {
			b.WriteString("local-preference ")
			b.WriteString(textbuf.Uint32(*curr.LocalPreference))
			b.WriteByte(' ')
		} else {
			b.WriteString("del local-preference ")
		}
	}
	if !communitiesEqual(prev.Communities, curr.Communities) {
		if len(curr.Communities) > 0 {
			b.WriteString("community [")
			for i, c := range curr.Communities {
				if i > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(c.String())
			}
			b.WriteString("] ")
		} else {
			b.WriteString("del community ")
		}
	}
	if !largeCommunitiesEqual(prev.LargeCommunities, curr.LargeCommunities) {
		if len(curr.LargeCommunities) > 0 {
			b.WriteString("large-community [")
			for i, lc := range curr.LargeCommunities {
				if i > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(lc.String())
			}
			b.WriteString("] ")
		} else {
			b.WriteString("del large-community ")
		}
	}
	if !extCommunitiesEqual(prev.ExtendedCommunities, curr.ExtendedCommunities) {
		if len(curr.ExtendedCommunities) > 0 {
			b.WriteString("extended-community [")
			for i, ec := range curr.ExtendedCommunities {
				if i > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(hex.EncodeToString(ec[:]))
			}
			b.WriteString("] ")
		} else {
			b.WriteString("del extended-community ")
		}
	}
	if prev.NextHop != curr.NextHop {
		if curr.NextHop != "" {
			b.WriteString("next-hop ")
			b.WriteString(curr.NextHop)
			b.WriteByte(' ')
		} else {
			b.WriteString("del next-hop ")
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
