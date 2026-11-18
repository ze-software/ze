// Design: docs/architecture/plugin/rib-storage-design.md -- the route clock and the `age` row member
// Related: rib_route_age.go -- routeClock, markReadAt, writeRouteAge
// Related: storage/routeentry_age_test.go -- the storage half: which insert keeps the time

package rib

import (
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
)

// routeAgeClock is a clock a test moves by hand. Only Now is replaced: the
// route clock reads nothing else.
type routeAgeClock struct {
	clock.RealClock
	at time.Time
}

func (c *routeAgeClock) Now() time.Time { return c.at }

func (c *routeAgeClock) advance(elapsed time.Duration) { c.at = c.at.Add(elapsed) }

// startRouteAgeClock puts r on a clock the test controls, with the route
// clock's epoch at the instant returned.
func startRouteAgeClock(r *RIBManager) *routeAgeClock {
	hand := &routeAgeClock{at: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)}
	r.routeClock = newRouteClock(hand)
	return hand
}

// receivedRouteRow answers the received row `show bgp rib` writes for prefix.
func receivedRouteRow(t *testing.T, r *RIBManager, prefix string) map[string]any {
	t.Helper()
	var answer map[string]any
	require.NoError(t, json.Unmarshal([]byte(dataStr(t, r.showPipeline("*", []string{"received"}))), &answer))
	rows, ok := answer["routes"].([]any)
	require.True(t, ok, "expected a routes list, got %v", answer)
	for _, raw := range rows {
		row, isRow := raw.(map[string]any)
		require.True(t, isRow)
		if row["prefix"] == prefix {
			return row
		}
	}
	require.Failf(t, "no received row", "prefix %s is not in %v", prefix, rows)
	return nil
}

// injectRouteAgeRoute stores one route through `request bgp rib inject`, the
// command every looking glass functional test feeds routes with.
func injectRouteAgeRoute(t *testing.T, r *RIBManager, extra ...string) {
	t.Helper()
	args := append([]string{"10.0.0.1", "ipv4/unicast", "10.0.0.0/24"}, extra...)
	status, _, err := r.handleCommand("request bgp rib inject", "", args)
	require.NoError(t, err)
	require.Equal(t, "done", status)
}

// TestRowCarriesAge stores a route, lets thirty seconds pass on the route
// clock, and reads the row `show bgp rib` writes for it (AC-7, engine side).
//
// VALIDATES: a received row carries an `age` member, a JSON number counting
// whole seconds since the route last changed.
// PREVENTS: the looking glass API answering age zero for every route because
// the RIB row has no such member.
func TestRowCarriesAge(t *testing.T) {
	r := newTestRIBManager(t)
	hand := startRouteAgeClock(r)

	injectRouteAgeRoute(t, r)
	hand.advance(30 * time.Second)

	row := receivedRouteRow(t, r, "10.0.0.0/24")
	assert.Equal(t, float64(30), row[rowKeyAge], "the age is thirty, so its unit is the second")
}

// TestInjectedRouteAgeKeptOnEqualInject injects one route three times: new,
// again unchanged, then with another next hop (AC-8 through the inject
// command, which bypasses the received-message handlers).
//
// VALIDATES: the row's age keeps growing over an equal injection and restarts
// when the injection changes the route.
// PREVENTS: lg-route-age reading a reset age after its equal re-injection.
func TestInjectedRouteAgeKeptOnEqualInject(t *testing.T) {
	r := newTestRIBManager(t)
	hand := startRouteAgeClock(r)

	injectRouteAgeRoute(t, r)
	hand.advance(30 * time.Second)
	injectRouteAgeRoute(t, r)
	hand.advance(15 * time.Second)
	assert.Equal(t, float64(45), receivedRouteRow(t, r, "10.0.0.0/24")[rowKeyAge], "an equal injection keeps the age growing")

	injectRouteAgeRoute(t, r, "nexthop", "10.0.0.9")
	hand.advance(5 * time.Second)
	assert.Equal(t, float64(5), receivedRouteRow(t, r, "10.0.0.0/24")[rowKeyAge], "a changed next hop restarts the age")
}

// TestReceivedUpdateStampsItsRoutes drives the received-UPDATE entry point
// three times for one prefix: new, the same UPDATE again, then one with
// another ORIGIN.
//
// VALIDATES: handleReceivedStructured hands its one clock reading to the route
// it stores, an equal UPDATE keeps the stored reading, and a changed one
// stores the new reading.
// PREVENTS: a route learned over a real session having no age, which the
// inject-fed functional tests cannot see.
func TestReceivedUpdateStampsItsRoutes(t *testing.T) {
	r := newTestRIBManager(t)
	electionFixtureLocRIB(t, r, netip.MustParseAddr("10.0.0.1"))
	hand := startRouteAgeClock(r)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	peer := netip.MustParseAddr("192.0.2.1")
	cidr := []byte{8, 10}
	feed := func(body []byte) storage.RouteEntry {
		feedReceivedWithIdentifier(t, r, 1, peer, identifierFor(t, "10.0.0.9"), ctxID, body)
		entry, ok := r.bgpPeers[1].Lookup(family.IPv4Unicast, cidr)
		require.True(t, ok, "the announced route is stored")
		return entry
	}

	learned := r.routeClock.now()
	require.NotEqual(t, storage.RouteTimeUnrecorded, learned)
	assert.Equal(t, learned, feed(identifierTestUpdate()).ChangedAt, "a new route holds the UPDATE's reading")

	hand.advance(30 * time.Second)
	assert.Equal(t, learned, feed(identifierTestUpdate()).ChangedAt, "an equal UPDATE keeps the stored reading")

	changedBody := identifierTestUpdate()
	changedBody[7] = 0x02 // ORIGIN INCOMPLETE, where the first UPDATE said IGP
	hand.advance(30 * time.Second)
	changed := r.routeClock.now()
	require.NotEqual(t, learned, changed)
	assert.Equal(t, changed, feed(changedBody).ChangedAt, "a changed attribute stores the new reading")
}

// TestPoolReceivedRouteHoldsTheReading stores a route through insertPoolNLRIs,
// the JSON event sibling of the structured path, then the same route again.
//
// VALIDATES: the pool path stamps a new route and keeps the reading on an
// equal one.
// PREVENTS: an external RIB process, which takes the JSON path, storing routes
// with no age.
func TestPoolReceivedRouteHoldsTheReading(t *testing.T) {
	r := newTestRIBManager(t)
	hand := startRouteAgeClock(r)
	peerRIB := storage.NewPeerRIB("192.0.2.11")
	nlri := []byte{24, 10, 0, 0}

	learned := r.routeClock.now()
	r.insertPoolNLRIs(peerRIB, family.IPv4Unicast, nlri, staleInsertAttrs, false, 1, 0, 0, false, nil)
	entry, ok := peerRIB.Lookup(family.IPv4Unicast, nlri)
	require.True(t, ok, "the route is stored")
	assert.Equal(t, learned, entry.ChangedAt, "a new route holds the reading")

	hand.advance(30 * time.Second)
	r.insertPoolNLRIs(peerRIB, family.IPv4Unicast, nlri, staleInsertAttrs, false, 2, 0, 0, false, nil)
	entry, ok = peerRIB.Lookup(family.IPv4Unicast, nlri)
	require.True(t, ok)
	assert.Equal(t, learned, entry.ChangedAt, "an equal announcement keeps the reading")
}

// TestUnparsedLabeledRouteHoldsTheReading stores a labeled route through
// insertLabeled, the fallback that parses the attributes per route and reads
// the route clock itself, then the same route again.
//
// VALIDATES: the fallback stamps a new route and keeps the reading on an equal
// one, as the parsed path does.
// PREVENTS: the one insert that takes no reading from its handler storing
// routes with no age.
func TestUnparsedLabeledRouteHoldsTheReading(t *testing.T) {
	r := newTestRIBManager(t)
	hand := startRouteAgeClock(r)
	peerRIB := storage.NewPeerRIB("192.0.2.11")
	wire := labeledNLRI(0, false, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100})
	cidr := []byte{8, 10}
	var affected []affectedPrefix

	learned := r.routeClock.now()
	r.insertLabeled(peerRIB, labeledFamily, staleInsertAttrs, wire, false, false, 81, 0, 0, &affected, nil)
	entry, ok := peerRIB.Lookup(labeledFamily, cidr)
	require.True(t, ok, "the labeled route is stored")
	assert.Equal(t, learned, entry.ChangedAt, "a new route holds the reading")

	hand.advance(30 * time.Second)
	r.insertLabeled(peerRIB, labeledFamily, staleInsertAttrs, wire, false, false, 82, 0, 0, &affected, nil)
	entry, ok = peerRIB.Lookup(labeledFamily, cidr)
	require.True(t, ok)
	assert.Equal(t, learned, entry.ChangedAt, "an equal announcement keeps the reading")
}

// TestRouteAgeWrittenOnlyWhenKnown writes the age member for each pair of
// readings a row can hold.
//
// VALIDATES: the member is the seconds between the two readings, and a pair
// with no age writes no member.
// PREVENTS: a route stored without a reading, or a row whose source took none,
// answering `age: 0`, which reads as "changed this second".
func TestRouteAgeWrittenOnlyWhenKnown(t *testing.T) {
	tests := []struct {
		name      string
		changedAt storage.RouteTime
		readAt    storage.RouteTime
		want      any
	}{
		{name: "thirty seconds old", changedAt: 100, readAt: 130, want: int64(30)},
		{name: "changed this second", changedAt: 100, readAt: 100, want: int64(0)},
		{name: "the route holds no reading", changedAt: storage.RouteTimeUnrecorded, readAt: 130},
		{name: "the row holds no reading", changedAt: 100, readAt: storage.RouteTimeUnrecorded},
		{name: "read before the change", changedAt: 130, readAt: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := map[string]any{}
			writeRouteAge(row, tt.changedAt, tt.readAt)
			if tt.want == nil {
				assert.NotContains(t, row, rowKeyAge)
				return
			}
			assert.Equal(t, tt.want, row[rowKeyAge])
		})
	}
}

// TestManagerBuiltWithoutConstructorHasNoRouteClock reads the route clock of a
// RIBManager built as a struct literal, as several tests build one.
//
// VALIDATES: its reading is RouteTimeUnrecorded.
// PREVENTS: a manager with no clock stamping routes from an epoch of zero, or
// reading a nil clock.
func TestManagerBuiltWithoutConstructorHasNoRouteClock(t *testing.T) {
	r := &RIBManager{}
	assert.Equal(t, storage.RouteTimeUnrecorded, r.routeClock.now())
}
