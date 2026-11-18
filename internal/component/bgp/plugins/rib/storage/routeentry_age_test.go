package storage

import (
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// Three readings of the route clock, in the order the tests below use them:
// the first insert, an equal re-announcement, and a changed announcement.
const (
	routeTimeInsert   RouteTime = 100
	routeTimeReplay   RouteTime = 130
	routeTimeReplaced RouteTime = 190
)

// routeAgePath is one way a route reaches a FamilyRIB. Each path decides
// "equal or changed" in its own branch, so each is driven through the same
// three announcements.
type routeAgePath struct {
	name    string
	fam     family.Family
	addPath bool
	nlri    []byte
	// parsed says the route goes through InsertEntry with an entry parsed
	// once, as a received UPDATE does, and not through InsertAt.
	parsed bool
}

// routeAgePaths lists every insert path of FamilyRIB: the direct BART store,
// the ADD-PATH path set and the opaque map, each reached from raw attribute
// bytes and from a parsed entry, plus the opaque path set.
func routeAgePaths() []routeAgePath {
	bgpLS := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	prefix := []byte{24, 10, 0, 0}
	prefixPath := []byte{0, 0, 0, 7, 24, 10, 0, 0}
	node := bgpLSNodeNLRI(0, 65001)
	nodePath := append([]byte{0, 0, 0, 7}, node...)
	return []routeAgePath{
		{name: "direct from bytes", fam: family.IPv4Unicast, nlri: prefix},
		{name: "direct from a parsed entry", fam: family.IPv4Unicast, nlri: prefix, parsed: true},
		{name: "add-path from bytes", fam: family.IPv4Unicast, addPath: true, nlri: prefixPath},
		{name: "add-path from a parsed entry", fam: family.IPv4Unicast, addPath: true, nlri: prefixPath, parsed: true},
		{name: "opaque from bytes", fam: bgpLS, nlri: node},
		{name: "opaque from a parsed entry", fam: bgpLS, nlri: node, parsed: true},
		{name: "opaque add-path from bytes", fam: bgpLS, addPath: true, nlri: nodePath},
		{name: "opaque add-path from a parsed entry", fam: bgpLS, addPath: true, nlri: nodePath, parsed: true},
	}
}

// announce stores attrs for the path's route as the message read at changedAt.
// The parsed form also carries a message identifier that differs for each
// reading, so an equal re-announcement has received identity to refresh.
func (p routeAgePath) announce(t *testing.T, rib *FamilyRIB, attrs []byte, changedAt RouteTime) {
	t.Helper()
	if !p.parsed {
		rib.InsertAt(attrs, p.nlri, changedAt)
		return
	}
	entry, fp, attrLen, err := ParseRouteEntry(attrs)
	require.NoError(t, err)
	entry.ChangedAt = changedAt
	entry.MsgID = uint64(changedAt)
	rib.InsertEntry(p.nlri, entry, fp, attrLen)
	entry.Release()
}

// stored answers the entry the RIB holds for the path's route.
func (p routeAgePath) stored(t *testing.T, rib *FamilyRIB) RouteEntry {
	t.Helper()
	entry, ok := rib.lookupEntry(p.nlri)
	require.True(t, ok, "the route is stored")
	return entry
}

// TestRouteEntrySize pins the bytes one stored route record costs, with the
// change time in it. The record is paid once per route per peer, so a field
// that grows it is a measured cost (AC-9).
//
// VALIDATES: RouteEntry is 64 bytes on a 64-bit layout, and ChangedAt sits in
// the padding between the three one-byte fields and MsgID.
// PREVENTS: a field added or reordered growing every stored route by eight
// bytes with nothing red.
func TestRouteEntrySize(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("the 64-byte figure is the 64-bit layout, which is what Ze ships on")
	}
	var entry RouteEntry
	assert.Equal(t, uintptr(64), unsafe.Sizeof(entry), "bytes per stored route record")
	assert.Equal(t, uintptr(4), unsafe.Offsetof(entry.ChangedAt), "the change time fills the padding after the one-byte fields")
	assert.Equal(t, uintptr(8), unsafe.Offsetof(entry.MsgID), "the change time did not push MsgID")
}

// TestRouteEntryCloneKeepsTime copies a stored route and reads the copy's
// change time (R-9).
//
// VALIDATES: Clone carries ChangedAt.
// PREVENTS: a cloned or retained route reading as one with no age.
func TestRouteEntryCloneKeepsTime(t *testing.T) {
	entry, _, _, err := ParseRouteEntry(concat(wireOriginIGP, wireASPath65001, wireNextHop))
	require.NoError(t, err)
	defer entry.Release()
	entry.ChangedAt = routeTimeInsert

	clone := entry.Clone()
	require.NotNil(t, clone)
	defer clone.Release()

	assert.Equal(t, routeTimeInsert, clone.ChangedAt)
}

// TestRouteAgeKeptOnEqualAnnounce announces one route three times to a peer's
// RIB: new, again with equal attributes, then with a changed attribute (AC-8,
// R-3).
//
// VALIDATES: the first insert writes the reading, an equal re-announcement
// keeps it, and an announcement that changes an attribute writes the new one.
// PREVENTS: a route refresh making every stable route look new.
func TestRouteAgeKeptOnEqualAnnounce(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	prefix := []byte{24, 10, 0, 0}

	rib.InsertAt(family.IPv4Unicast, attrs, prefix, routeTimeInsert)
	entry, ok := rib.Lookup(family.IPv4Unicast, prefix)
	require.True(t, ok)
	assert.Equal(t, routeTimeInsert, entry.ChangedAt, "the first insert writes the reading")

	rib.InsertAt(family.IPv4Unicast, attrs, prefix, routeTimeReplay)
	entry, ok = rib.Lookup(family.IPv4Unicast, prefix)
	require.True(t, ok)
	assert.Equal(t, routeTimeInsert, entry.ChangedAt, "an equal re-announcement keeps the reading")

	changed := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100)
	rib.InsertAt(family.IPv4Unicast, changed, prefix, routeTimeReplaced)
	entry, ok = rib.Lookup(family.IPv4Unicast, prefix)
	require.True(t, ok)
	assert.Equal(t, routeTimeReplaced, entry.ChangedAt, "a changed attribute writes the new reading")
}

// TestRouteAgeOnEveryInsertPath drives the three announcements of
// TestRouteAgeKeptOnEqualAnnounce through each insert path of FamilyRIB.
//
// VALIDATES: every path writes the reading on a new or changed route and keeps
// it on an equal one. On a parsed entry the equal branch still refreshes the
// received identity, so the kept time is not the branch doing nothing.
// PREVENTS: one backend (ADD-PATH, opaque) resetting or dropping the age the
// direct path keeps.
func TestRouteAgeOnEveryInsertPath(t *testing.T) {
	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	changed := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100)

	for _, path := range routeAgePaths() {
		t.Run(path.name, func(t *testing.T) {
			rib := newFamilyRIB(path.fam, path.addPath)
			defer rib.Release()

			path.announce(t, rib, attrs, routeTimeInsert)
			assert.Equal(t, routeTimeInsert, path.stored(t, rib).ChangedAt, "the first insert writes the reading")

			path.announce(t, rib, attrs, routeTimeReplay)
			replayed := path.stored(t, rib)
			assert.Equal(t, routeTimeInsert, replayed.ChangedAt, "an equal re-announcement keeps the reading")
			if path.parsed {
				assert.Equal(t, uint64(routeTimeReplay), replayed.MsgID, "the equal branch refreshed the received identity")
			}

			path.announce(t, rib, changed, routeTimeReplaced)
			assert.Equal(t, routeTimeReplaced, path.stored(t, rib).ChangedAt, "a changed attribute writes the new reading")
			assert.Equal(t, 1, rib.Len(), "the three announcements are one route")
		})
	}
}

// TestRouteAgeKeptWhenAttributeBytesDifferAndValuesAreEqual re-announces a
// route with the same attributes in another order. The fingerprint differs, so
// the fast path misses and entriesEqual decides.
//
// VALIDATES: the equality the RIB decides after parsing keeps the reading, as
// the fingerprint fast path does.
// PREVENTS: the age resetting on a re-announcement only because a peer
// re-encoded the same attributes.
func TestRouteAgeKeptWhenAttributeBytesDifferAndValuesAreEqual(t *testing.T) {
	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	reordered := concat(wireOriginIGP, wireNextHop, wireASPath65001)
	require.NotEqual(t, attrFingerprint(attrs), attrFingerprint(reordered), "the fast path must miss")

	for _, path := range routeAgePaths() {
		t.Run(path.name, func(t *testing.T) {
			rib := newFamilyRIB(path.fam, path.addPath)
			defer rib.Release()

			path.announce(t, rib, attrs, routeTimeInsert)
			path.announce(t, rib, reordered, routeTimeReplay)

			assert.Equal(t, routeTimeInsert, path.stored(t, rib).ChangedAt)
		})
	}
}

// TestRouteStoredWithoutReadingHasNoAge stores a route through Insert, which
// takes no reading, and asks its age.
//
// VALIDATES: the route holds RouteTimeUnrecorded and AgeAt refuses to answer.
// PREVENTS: a route stored without a reading answering an age of zero, or an
// age counted from the RIB's start.
func TestRouteStoredWithoutReadingHasNoAge(t *testing.T) {
	rib := newFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()
	prefix := []byte{24, 10, 0, 0}

	rib.Insert(concat(wireOriginIGP, wireASPath65001, wireNextHop), prefix)

	entry, ok := rib.lookupEntry(prefix)
	require.True(t, ok)
	assert.Equal(t, RouteTimeUnrecorded, entry.ChangedAt)
	_, known := entry.ChangedAt.AgeAt(routeTimeReplay)
	assert.False(t, known)
}

// TestRouteTimeSince reads the route clock at the edges of what a reading
// holds: the epoch itself, a second boundary, a time before the epoch, and the
// last and first-past-the-last second of the four-byte range.
//
// VALIDATES: a reading is the whole elapsed seconds plus one, and a time the
// type cannot hold is RouteTimeUnrecorded.
// PREVENTS: the epoch reading as "unrecorded", and a time out of range
// wrapping into a small, plausible reading.
func TestRouteTimeSince(t *testing.T) {
	epoch := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	const lastSecond = math.MaxUint32 - 1

	tests := []struct {
		name    string
		elapsed time.Duration
		want    RouteTime
	}{
		{name: "the epoch itself", elapsed: 0, want: 1},
		{name: "inside the first second", elapsed: 999 * time.Millisecond, want: 1},
		{name: "the first second boundary", elapsed: time.Second, want: 2},
		{name: "thirty seconds", elapsed: 30 * time.Second, want: 31},
		{name: "before the epoch", elapsed: -time.Nanosecond, want: RouteTimeUnrecorded},
		{name: "the last second a reading holds", elapsed: lastSecond * time.Second, want: math.MaxUint32},
		{name: "one second past the last", elapsed: (lastSecond + 1) * time.Second, want: RouteTimeUnrecorded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, RouteTimeSince(epoch, epoch.Add(tt.elapsed)))
		})
	}
}

// TestRouteTimeAgeAt asks the age between two readings, including each pair
// that has none.
//
// VALIDATES: the age is the seconds between the readings, zero included when
// both are the same second, and the three unanswerable pairs answer false.
// PREVENTS: an unrecorded reading, or a reading taken before the change,
// answering an age.
func TestRouteTimeAgeAt(t *testing.T) {
	tests := []struct {
		name      string
		changedAt RouteTime
		now       RouteTime
		want      time.Duration
		known     bool
	}{
		{name: "thirty seconds later", changedAt: 100, now: 130, want: 30 * time.Second, known: true},
		{name: "the same second", changedAt: 100, now: 100, want: 0, known: true},
		{name: "the route has no reading", changedAt: RouteTimeUnrecorded, now: 130},
		{name: "the clock has no reading", changedAt: 100, now: RouteTimeUnrecorded},
		{name: "read before the change", changedAt: 130, now: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			age, known := tt.changedAt.AgeAt(tt.now)
			assert.Equal(t, tt.known, known)
			assert.Equal(t, tt.want, age)
		})
	}
}
