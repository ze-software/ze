package peer

import (
	"context"
	"encoding/hex"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/chaos/scenario"
)

// The speakers and routes the tests below share. 65101 is 0xFE4D and 64500 is
// 0xFBF4, the AS numbers of the path as they appear in the expected bytes.
const (
	routeSetLocalAS     = 65000
	routeSetNeighbourAS = 65101
	routeSetFarAS       = 64500
)

var (
	routeSetOutsidePrefix = netip.MustParsePrefix("198.18.0.0/24")
	routeSetInsidePrefix  = netip.MustParsePrefix("198.19.0.0/24")
)

// updateHex spells a whole BGP message from its parts, so each test names the
// attribute every group of bytes is.
func updateHex(parts ...string) string {
	return strings.ToUpper(strings.Join(parts, ""))
}

// bgpMarker is the 16 bytes every BGP message starts with.
const bgpMarker = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"

// TestRouteSetUpdateBytesEBGP builds the UPDATE of one given route on an eBGP
// session and compares every byte.
//
// VALIDATES: the route goes out with the next hop and the AS path it was given.
// The path already starts with the speaker's own AS, and nothing is prepended
// to it. No LOCAL_PREF is present on an external session.
// PREVENTS: a simulated neighbour announcing its AS twice, or the fixed next hop
// of the seed-generated mode in place of the given one.
func TestRouteSetUpdateBytesEBGP(t *testing.T) {
	sender := NewSender(SenderConfig{
		ASN:      routeSetNeighbourAS,
		IsIBGP:   false,
		NextHop:  netip.MustParseAddr("10.255.0.1"),
		RouteSet: &scenario.RouteSet{},
	})

	data, err := sender.BuildAnnouncement(scenario.Announcement{
		Prefix:  routeSetOutsidePrefix,
		NextHop: netip.MustParseAddr("192.0.2.1"),
		ASPath:  []uint32{routeSetNeighbourAS, routeSetFarAS},
	})
	require.NoError(t, err)

	want := updateHex(
		bgpMarker, "0033", "02", // length 51, type UPDATE
		"0000",                                   // no withdrawn route
		"0018",                                   // 24 bytes of path attributes
		"40010100",                               // ORIGIN IGP
		"40020A", "0202", "0000FE4D", "0000FBF4", // AS_PATH: one AS_SEQUENCE of 65101 64500
		"400304", "C0000201", // NEXT_HOP 192.0.2.1
		"18C61200", // 198.18.0.0/24
	)
	assert.Equal(t, want, strings.ToUpper(hex.EncodeToString(data)))
}

// TestRouteSetUpdateBytesIBGP builds the UPDATE of two given routes on an iBGP
// session and compares every byte.
//
// VALIDATES: on an internal session the path goes out as given, with no local
// AS prepended, the next hop is the given one (a reflector's loopback here, not
// the speaker's router id), and LOCAL_PREF is present. A prefix that originates
// inside the AS goes out with an empty AS_PATH.
// PREVENTS: a simulated router sending the path the seed-generated mode derives
// from its AS number, or its own router id as next hop on a reflected route.
func TestRouteSetUpdateBytesIBGP(t *testing.T) {
	sender := NewSender(SenderConfig{
		ASN:      routeSetLocalAS,
		IsIBGP:   true,
		NextHop:  netip.MustParseAddr("10.0.0.4"),
		RouteSet: &scenario.RouteSet{},
	})

	learned, err := sender.BuildAnnouncement(scenario.Announcement{
		Prefix:  routeSetOutsidePrefix,
		NextHop: netip.MustParseAddr("10.0.0.3"),
		ASPath:  []uint32{routeSetNeighbourAS, routeSetFarAS},
	})
	require.NoError(t, err)
	wantLearned := updateHex(
		bgpMarker, "003A", "02", // length 58, type UPDATE
		"0000",                                   // no withdrawn route
		"001F",                                   // 31 bytes of path attributes
		"40010100",                               // ORIGIN IGP
		"40020A", "0202", "0000FE4D", "0000FBF4", // AS_PATH: 65101 64500, no 65000 before it
		"400304", "0A000003", // NEXT_HOP 10.0.0.3
		"400504", "00000064", // LOCAL_PREF 100
		"18C61200", // 198.18.0.0/24
	)
	assert.Equal(t, wantLearned, strings.ToUpper(hex.EncodeToString(learned)))

	originated, err := sender.BuildAnnouncement(scenario.Announcement{
		Prefix:  routeSetInsidePrefix,
		NextHop: netip.MustParseAddr("10.0.0.2"),
	})
	require.NoError(t, err)
	wantOriginated := updateHex(
		bgpMarker, "0030", "02", // length 48, type UPDATE
		"0000",               // no withdrawn route
		"0015",               // 21 bytes of path attributes
		"40010100",           // ORIGIN IGP
		"400200",             // empty AS_PATH
		"400304", "0A000002", // NEXT_HOP 10.0.0.2
		"400504", "00000064", // LOCAL_PREF 100
		"18C61300", // 198.19.0.0/24
	)
	assert.Equal(t, wantOriginated, strings.ToUpper(hex.EncodeToString(originated)))
}

// TestRouteSetRefusesPathTheSenderWouldChange gives an eBGP sender paths the
// builder would prepend its AS to.
//
// VALIDATES: a path that does not start with the speaker's AS, and an empty
// path, are refused with an error naming the prefix and the AS the path must
// start with, and RouteSetProblem reports the same for a whole set.
// PREVENTS: a route going out with a path other than the one given, silently.
func TestRouteSetRefusesPathTheSenderWouldChange(t *testing.T) {
	foreign := scenario.Announcement{
		Prefix:  routeSetOutsidePrefix,
		NextHop: netip.MustParseAddr("192.0.2.1"),
		ASPath:  []uint32{routeSetFarAS},
	}
	sender := NewSender(SenderConfig{
		ASN:      routeSetNeighbourAS,
		RouteSet: &scenario.RouteSet{Announcements: []scenario.Announcement{foreign}},
	})

	_, err := sender.BuildAnnouncement(foreign)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "198.18.0.0/24")
	assert.Contains(t, err.Error(), "starting with 64500")
	assert.Contains(t, err.Error(), "must start with 65101")

	_, err = sender.BuildAnnouncement(scenario.Announcement{
		Prefix:  routeSetOutsidePrefix,
		NextHop: netip.MustParseAddr("192.0.2.1"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty AS path")

	require.Error(t, sender.RouteSetProblem())
	assert.Nil(t, sender.BuildRoute(routeSetOutsidePrefix), "a refused announcement builds no UPDATE")

	_, err = sender.BuildAnnouncement(scenario.Announcement{NextHop: netip.MustParseAddr("192.0.2.1")})
	require.Error(t, err, "an announcement with no prefix")
	_, err = sender.BuildAnnouncement(scenario.Announcement{Prefix: routeSetOutsidePrefix})
	require.Error(t, err, "an announcement with no next hop")
}

// TestRouteSetBuildRouteAnswersForItsPrefixes asks a sender with a route set
// for a prefix of the set and for one outside it.
//
// VALIDATES: BuildRoute, the entry every chaos and route action re-announces
// through, gives the UPDATE of the set's announcement, and nil for a prefix the
// set does not hold. A sender with no route set still builds any prefix.
// PREVENTS: a re-announcement after a withdrawal going out with the fixed next
// hop of the seed-generated mode.
func TestRouteSetBuildRouteAnswersForItsPrefixes(t *testing.T) {
	announcement := scenario.Announcement{
		Prefix:  routeSetOutsidePrefix,
		NextHop: netip.MustParseAddr("192.0.2.1"),
		ASPath:  []uint32{routeSetNeighbourAS, routeSetFarAS},
	}
	config := SenderConfig{
		ASN:      routeSetNeighbourAS,
		NextHop:  netip.MustParseAddr("10.255.0.1"),
		RouteSet: &scenario.RouteSet{Announcements: []scenario.Announcement{announcement}},
	}
	sender := NewSender(config)
	require.NoError(t, sender.RouteSetProblem())

	want, err := sender.BuildAnnouncement(announcement)
	require.NoError(t, err)
	assert.Equal(t, want, sender.BuildRoute(routeSetOutsidePrefix))
	assert.Nil(t, sender.BuildRoute(routeSetInsidePrefix), "a prefix the set does not hold")

	config.RouteSet = nil
	seeded := NewSender(config)
	require.NoError(t, seeded.RouteSetProblem())
	assert.NotNil(t, seeded.BuildRoute(routeSetInsidePrefix), "the seed-generated mode builds any prefix")
	assert.NotEqual(t, want, seeded.BuildRoute(routeSetOutsidePrefix), "the seed-generated mode uses its own next hop")

	empty := NewSender(SenderConfig{ASN: routeSetNeighbourAS, RouteSet: &scenario.RouteSet{}})
	assert.Nil(t, empty.BuildRoute(routeSetOutsidePrefix), "an empty set announces nothing, it is not the seed-generated mode")
}

// TestSimulatorAnnouncesItsRouteSet runs a whole simulated session with a
// route set against a fake daemon.
//
// VALIDATES: the simulator sends exactly the routes of the set, in order, each
// with the bytes BuildAnnouncement gives, then End-of-RIB with the set's count,
// and the daemon side reads that many UPDATEs plus the End-of-RIB.
// PREVENTS: a profile with a route set still announcing RouteCount routes
// generated from the seed.
func TestSimulatorAnnouncesItsRouteSet(t *testing.T) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	zeResult := make(chan int, 1)
	go fakeZePeer(t, ln, zeResult)

	set := &scenario.RouteSet{Announcements: []scenario.Announcement{
		{Prefix: routeSetOutsidePrefix, NextHop: netip.MustParseAddr("10.0.0.3"), ASPath: []uint32{routeSetNeighbourAS, routeSetFarAS}},
		{Prefix: routeSetInsidePrefix, NextHop: netip.MustParseAddr("10.0.0.2")},
	}}
	profile := SimProfile{
		Index:    0,
		ASN:      routeSetLocalAS,
		RouterID: netip.MustParseAddr("10.0.0.4"),
		IsIBGP:   true,
		HoldTime: 90,
		// A count the seed-generated mode would send, so a simulator that
		// ignored the set would send 50 routes and fail the comparison below.
		RouteCount: 50,
		TotalPeers: 1,
		RouteSet:   set,
	}
	events := make(chan Event, 100)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		RunSimulator(ctx, SimulatorConfig{Profile: profile, Seed: 42, Addr: ln.Addr().String(), Events: events, Quiet: true})
		close(events)
	}()

	var sent [][]byte
	var prefixes []netip.Prefix
	eorCount := -1
	for ev := range events {
		if ev.Type == EventError {
			t.Fatalf("simulator error: %v", ev.Err)
		}
		if ev.Type == EventRouteSent {
			sent = append(sent, ev.BGPMessage)
			prefixes = append(prefixes, ev.Prefix)
		}
		if ev.Type == EventEORSent {
			eorCount = ev.Count
			cancel()
		}
	}

	assert.Equal(t, []netip.Prefix{routeSetOutsidePrefix, routeSetInsidePrefix}, prefixes)
	assert.Equal(t, 2, eorCount, "End-of-RIB follows the two routes of the set")
	sender := newIPv4Sender(profile)
	require.Len(t, sent, 2)
	for index, announcement := range set.Announcements {
		want, buildErr := sender.BuildAnnouncement(announcement)
		require.NoError(t, buildErr)
		assert.Equal(t, want, sent[index], "route %d", index)
	}
	// Two routes, the End-of-RIB (an UPDATE too), and no route beyond them.
	assert.Equal(t, 3, <-zeResult)
}

// TestSimulatorRefusesRouteSetWithAnotherFamily gives a profile a route set
// and a second family.
//
// VALIDATES: the run ends with an error before any session, naming the family.
// PREVENTS: a profile announcing its exact IPv4 routes beside seed-generated
// routes of another family, which is no exact route set.
func TestSimulatorRefusesRouteSetWithAnotherFamily(t *testing.T) {
	events := make(chan Event, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	RunSimulator(ctx, SimulatorConfig{
		Profile: SimProfile{
			ASN:      routeSetLocalAS,
			RouterID: netip.MustParseAddr("10.0.0.4"),
			IsIBGP:   true,
			HoldTime: 90,
			Families: []string{familyIPv4Unicast, familyIPv6Unicast},
			RouteSet: &scenario.RouteSet{},
		},
		// No listener: a run that reached the dial would report a connection
		// error, never the refusal.
		Addr:   "127.0.0.1:1",
		Events: events,
		Quiet:  true,
	})
	close(events)

	var refusals []string
	for ev := range events {
		if ev.Type == EventError {
			refusals = append(refusals, ev.Err.Error())
		}
	}
	require.Len(t, refusals, 1)
	assert.Contains(t, refusals[0], "route set")
	assert.Contains(t, refusals[0], familyIPv6Unicast)
}
