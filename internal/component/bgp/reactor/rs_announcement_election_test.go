// Design: docs/architecture/bgp/structural-forwarding.md -- announcement election at the final writer
// Related: session_ownership.go -- adjOutWrite.oweAnnouncement
// Related: rs_withdrawal_replacement_test.go -- the live route-server fixtures
package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// announcementLive is one live route-server run: sources A, B and E, and
// client C with no ADD-PATH, over real sessions, with the real bgp-rs and
// bgp-rib.
type announcementLive struct {
	a, b, e, c *lowLivePeer
	peers      []*lowLivePeer
	markers    int
}

func announcementLiveRouter(t *testing.T, fastPath bool) *announcementLive {
	t.Helper()
	settings := []*PeerSettings{
		lowLiveSettings("10.0.0.1", 65000, 65002),
		lowLiveSettings("10.0.0.2", 65000, 65003),
		lowLiveSettings("10.0.0.4", 65000, 65005),
		lowLiveSettings("10.0.0.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
		setting.RSFastPath = fastPath
	}
	peers := lowLiveRouter(t, settings...)
	return &announcementLive{a: peers[0], b: peers[1], e: peers[2], c: peers[3], peers: peers}
}

// settle returns once every UPDATE source sent before it has passed C's
// writer and C owes nothing: source's marker reaches C, C's owed set
// empties, and a second marker written after any decision reaches C.
func (run *announcementLive) settle(t *testing.T, source *lowLivePeer, attrs []byte) {
	t.Helper()
	for range 2 {
		run.markers++
		marker := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 9, byte(100 + run.markers), 0}), 24)
		ownershipLiveSend(t, source, nil, attrs, marker) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, run.c, marker, attrs)
		lowEventually(t, func() bool { return adjOutOwedCount(run.c.peer) == 0 }, "C owes nothing")
	}
}

// announcementElects runs AC-22 (a) to (d) on one rail. A (AS_PATH 65002)
// is better than B and E (one AS more), so the RIB elects A.
func announcementElects(t *testing.T, fastPath bool) {
	t.Helper()
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65100)
	attrsE := firstASAttrs(4, 65005, 65100)
	worseA := firstASAttrs(4, 65002, 65100, 65101)
	t.Run("a-worse-later-route-writes-nothing", func(t *testing.T) {
		run := announcementLiveRouter(t, fastPath)
		ownershipLiveSend(t, run.a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, run.c, prefix, attrsA)
		ownershipLiveSend(t, run.b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.b, attrsB)
		run.settle(t, run.a, attrsA)
		view := ownershipLiveRead(t, run.c)
		ownershipLiveRoute(t, view, prefix, attrsA)
		ownershipLiveAnnouncements(t, view, prefix, attrsA)
		ownershipLiveWithdrawals(t, view, prefix, 0)
		ownershipLiveEstablished(t, run.peers)
	})
	t.Run("b-better-later-route-written", func(t *testing.T) {
		run := announcementLiveRouter(t, fastPath)
		ownershipLiveSend(t, run.b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, run.c, prefix, attrsB)
		ownershipLiveSend(t, run.a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.a, attrsA)
		view := ownershipLiveRead(t, run.c)
		ownershipLiveRoute(t, view, prefix, attrsA)
		ownershipLiveAnnouncements(t, view, prefix, attrsB, attrsA)
		ownershipLiveWithdrawals(t, view, prefix, 0)
		ownershipLiveEstablished(t, run.peers)
	})
	t.Run("c-owner-worsens-other-elected", func(t *testing.T) {
		run := announcementLiveRouter(t, fastPath)
		ownershipLiveSend(t, run.a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, run.c, prefix, attrsA)
		ownershipLiveSend(t, run.b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.b, attrsB)
		ownershipLiveSend(t, run.a, nil, worseA, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.a, attrsA)
		view := ownershipLiveRead(t, run.c)
		ownershipLiveRoute(t, view, prefix, attrsB)
		ownershipLiveAnnouncements(t, view, prefix, attrsA, attrsB)
		ownershipLiveWithdrawals(t, view, prefix, 0)
		ownershipLiveEstablished(t, run.peers)
	})
	t.Run("d-third-source-in-and-out-writes-nothing", func(t *testing.T) {
		run := announcementLiveRouter(t, fastPath)
		ownershipLiveSend(t, run.a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, run.c, prefix, attrsA)
		ownershipLiveSend(t, run.b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.b, attrsB)
		ownershipLiveSend(t, run.e, nil, attrsE, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.e, attrsE)
		ownershipLiveSend(t, run.e, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
		run.settle(t, run.e, attrsE)
		view := ownershipLiveRead(t, run.c)
		ownershipLiveRoute(t, view, prefix, attrsA)
		ownershipLiveAnnouncements(t, view, prefix, attrsA)
		ownershipLiveWithdrawals(t, view, prefix, 0)
		ownershipLiveEstablished(t, run.peers)
	})
}

// TestRSAnnouncementElectsOnCachedRail proves AC-22 (a) to (d) on bgp-rs's
// cached rail. Method: real sessions over the live router fixture, C's wire
// parsed into its announcement and withdrawal history per prefix. A worse
// later route writes nothing, a better one is written, an owner that
// worsens is replaced by the elected route, and a third source that comes
// and goes writes nothing.
//
// VALIDATES: Data Flow 3a end to end on the cached rail (owner, Q-3).
// PREVENTS: forward-all, where C's wire carries the newest arrival.
func TestRSAnnouncementElectsOnCachedRail(t *testing.T) {
	announcementElects(t, false)
}

// TestRSAnnouncementElectsOnFastPath proves AC-22 (a) to (d) on
// reactorForwardRS. C's history counts every announcement, so a second copy
// from the cached rail after the fast path handled an owed UPDATE shows as an
// extra entry.
//
// VALIDATES: Data Flow 3a end to end on the fast path (owner, Q-3).
// PREVENTS: forward-all on the fast path, and a duplicate cached-rail write.
func TestRSAnnouncementElectsOnFastPath(t *testing.T) {
	announcementElects(t, true)
}

// addPathClientPaths reads C's received UPDATEs as ADD-PATH IPv4 unicast and
// returns, in arrival order, the path identifier and attributes of each
// announcement of prefix.
func addPathClientPaths(t *testing.T, client *lowLivePeer, prefix netip.Prefix) (ids []uint32, attrs [][]byte) {
	t.Helper()
	want := ownershipLiveNLRI([]netip.Prefix{prefix})
	client.mu.Lock()
	defer client.mu.Unlock()
	for _, frame := range client.frames {
		if frame[18] != byte(msgtype.TypeUPDATE) {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:]) // RFC 4271 Section 4.3.
		require.NoError(t, err)
		// RFC 7911 Section 3: each NLRI is a four-octet Path Identifier,
		// then the RFC 4271 Section 4.3 prefix tuple.
		for field := update.NLRI; len(field) >= 5; {
			size := 5 + (int(field[4])+7)/8
			require.LessOrEqual(t, size, len(field), "an ADD-PATH NLRI overruns its field")
			if string(field[4:size]) == string(want) {
				ids = append(ids, binary.BigEndian.Uint32(field))
				attrs = append(attrs, update.PathAttributes)
			}
			field = field[size:]
		}
	}
	return ids, attrs
}

// TestRSAnnouncementAddPathClientGetsEveryPath proves AC-23. Method: client C
// negotiated ADD-PATH send for IPv4 unicast with Ze; A and B announce the
// prefix in both orders, B's route worse. C receives both paths, each with
// its own identifier, in arrival order, and C owes nothing, so no election
// runs.
//
// VALIDATES: oweAnnouncement leaves an ADD-PATH destination to forwarding
// (RFC 7947 Section 2.3.2.2).
// PREVENTS: an ADD-PATH client given only the elected path.
func TestRSAnnouncementAddPathClientGetsEveryPath(t *testing.T) {
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65100)
	for _, betterFirst := range []bool{true, false} {
		name := "worse-route-second"
		if !betterFirst {
			name = "worse-route-first"
		}
		t.Run(name, func(t *testing.T) {
			settings := []*PeerSettings{
				lowLiveSettings("192.0.2.1", 65000, 65002),
				lowLiveSettings("192.0.2.2", 65000, 65003),
				lowLiveSettings("192.0.2.3", 65000, 65004),
			}
			for _, setting := range settings {
				setting.RSClient = true
			}
			// RFC 7911 Section 4: Ze is able to send multiple paths to C.
			settings[2].Capabilities = append(settings[2].Capabilities, &capability.AddPath{
				Families: []capability.AddPathFamily{{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathSend}},
			})
			peers := addPathLiveRouter(t, settings...)
			a, b, c := peers[0], peers[1], peers[2]
			order := []struct {
				peer  *lowLivePeer
				attrs []byte
			}{{a, attrsA}, {b, attrsB}}
			if !betterFirst {
				order[0], order[1] = order[1], order[0]
			}
			for i, step := range order {
				ownershipLiveSend(t, step.peer, nil, step.attrs, prefix) // RFC 4271 Section 4.3.
				lowEventually(t, func() bool {
					ids, _ := addPathClientPaths(t, c, prefix)
					return len(ids) == i+1
				}, "C receives the path")
			}
			require.Zero(t, adjOutOwedCount(c.peer), "an ADD-PATH client owes nothing")
			ids, attrs := addPathClientPaths(t, c, prefix)
			require.Len(t, ids, 2)
			require.NotEqual(t, ids[0], ids[1], "each path has its own identifier")
			require.Equal(t, [][]byte{order[0].attrs, order[1].attrs}, attrs, "the paths arrive in arrival order")
			ownershipLiveEstablished(t, peers)
		})
	}
}
