// Design: docs/guide/route-reflection.md -- route-server replacement on an owner withdrawal.
// Related: rs_withdrawal_replacement_test.go -- the non-ADD-PATH replacement probes.
// Related: rs_withdrawal_ownership_test.go -- the recipient TCP projection these tests read.
// Related: rfc7705_live_behavior_test.go -- lowLiveRouter, which this file's router follows.
// RFC 7911 Section 3 -- see rfc/short/rfc7911.md.
package reactor

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// TestRSOwnerWithdrawalAddPathSourceKeepsOtherPath proves AC-9 on C's wire.
// Source A negotiates ADD-PATH send for IPv4 unicast with Ze and announces
// 10.9.0.0/24 twice, as path 1 (AS_PATH 65002) and then path 2 (AS_PATH
// 65002 65010). Client C has no ADD-PATH, so it holds one route: path 2
// contests path 1, is owed to bgp-rib's election (owner, Q-3), and path 1,
// the shorter AS_PATH, stays C's route; path 2 is never written. Then A
// withdraws one path:
//
//   - other-path: A withdraws path 2. The writer matches the owner by A's
//     received path identifier, so the withdrawal of a path C does not hold
//     reaches C as nothing, and C keeps path 1.
//   - owner-path: A withdraws path 1. The withdrawal is owed to bgp-rib's
//     recovery, which elects A's remaining path 2, and C receives it.
//
// Either way C ends holding one of A's paths and is never sent a withdrawal.
// Each case runs on bgp-rs's cached rail and on the reactor fast path.
//
// VALIDATES: AC-9, the owner key the writer matches carries A's received path
// identifier, and an owner withdrawal from an ADD-PATH source is replaced by
// that source's other path.
// PREVENTS: a client without ADD-PATH losing a prefix its source still offers
// on another path.
func TestRSOwnerWithdrawalAddPathSourceKeepsOtherPath(t *testing.T) {
	for _, rail := range []struct {
		name string
		fast bool
	}{{"cached", false}, {"fast-path", true}} {
		for _, tc := range []struct {
			name      string
			withdrawn uint32
			kept      int
		}{{"other-path", 2, 0}, {"owner-path", 1, 1}} {
			t.Run(rail.name+"/"+tc.name, func(t *testing.T) {
				addPathKeepsOtherPath(t, rail.fast, tc.withdrawn, tc.kept)
			})
		}
	}
}

// addPathKeepsOtherPath runs one case of
// TestRSOwnerWithdrawalAddPathSourceKeepsOtherPath: paths 1 and 2 of A,
// withdraw the path named withdrawn, and expect C on paths[kept].
func addPathKeepsOtherPath(t *testing.T, fast bool, withdrawn uint32, kept int) {
	t.Helper()
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
		setting.RSFastPath = fast
	}
	// RFC 7911 Section 4: Ze is able to receive multiple paths from A.
	settings[0].Capabilities = append(settings[0].Capabilities, &capability.AddPath{
		Families: []capability.AddPathFamily{{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathReceive}},
	})
	peers := addPathLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	paths := [][]byte{firstASAttrs(4, 65002), firstASAttrs(4, 65002, 65010)}

	addPathLiveSend(t, a, 0, paths[0], 1, prefix) // RFC 7911 Section 3.
	ownershipLiveThrough(t, c, prefix, paths[0])
	addPathLiveSend(t, a, 0, paths[1], 2, prefix) // RFC 7911 Section 3.
	// A's marker passes C's writer after path 2, and the election of the
	// owed key settles before the history is read.
	marker := netip.MustParsePrefix("10.9.253.0/24")
	addPathLiveSend(t, a, 0, paths[0], 3, marker) // RFC 7911 Section 3.
	ownershipLiveThrough(t, c, marker, paths[0])
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C owes nothing after path 2")
	// RFC 4271 Section 9.1.2.2, step a: path 1's shorter AS_PATH is
	// elected, so C is never sent path 2 while path 1 stands.
	elected := ownershipLiveRead(t, c)
	ownershipLiveRoute(t, elected, prefix, paths[0])
	ownershipLiveAnnouncements(t, elected, prefix, paths[0])

	// Count the recovery lookups for C that name the prefix from here on:
	// a withdrawal matched by Path ID owes nothing when it names a path C
	// does not hold, so no election runs for it.
	destination := c.peer.Settings().Address
	prefixKey := ownershipLiveNLRI([]netip.Prefix{prefix})
	var lookups atomic.Int32
	publishLookupTap(t, func(request *ribevents.RecoveryRequest, outcomes []ribevents.RecoveryOutcome, err error) ([]ribevents.RecoveryOutcome, error) {
		if request.Destination != destination {
			return outcomes, err
		}
		for i := range request.Keys {
			if bytes.Equal(request.Keys[i].NLRI, prefixKey) {
				lookups.Add(1)
			}
		}
		return outcomes, err
	})

	addPathLiveSend(t, a, withdrawn, nil, 0, prefix) // RFC 7911 Section 3.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if bytes.Equal(ownershipLiveRead(t, c).routes[prefix], paths[kept]) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C owes nothing")
	// Let any late write for the prefix land before the history is judged.
	later := netip.MustParsePrefix("10.9.254.0/24")
	ownershipLiveSend(t, b, nil, firstASAttrs(4, 65003), later) // RFC 4271 Section 4.3.
	view := ownershipLiveThrough(t, c, later, firstASAttrs(4, 65003))
	// RFC 4271 Section 9.2: "All newly installed routes and all newly
	// unfeasible routes for which there is no replacement route SHALL be
	// advertised to its peers by means of an UPDATE message." A's other path
	// is the replacement, so C is sent no withdrawal.
	ownershipLiveRoute(t, view, prefix, paths[kept])
	ownershipLiveWithdrawals(t, view, prefix, 0)
	// The history proves which rule decided. A withdrawal of a path C does
	// not hold writes nothing, so C saw path 1 only; an owner withdrawal is
	// replaced once, by path 2. Were the owner matched by source alone, path
	// 2 would have been written as the owner's own re-announcement, which the
	// elected view above refuses.
	want := [][]byte{paths[0]}
	if kept == 1 {
		want = append(want, paths[1])
	}
	if got := view.announcements[prefix]; !slices.EqualFunc(got, want, bytes.Equal) {
		t.Errorf("C's announcements of %s = %x, want %x", prefix, got, want)
	}
	// The other-path withdrawal ran no election for the prefix at C, which
	// proves the writer matched it by Path ID rather than owing it; the
	// owner-path withdrawal is elected through recovery.
	if got := lookups.Load(); (kept == 0) != (got == 0) {
		t.Errorf("recovery lookups for %s at C after withdrawing path %d = %d, want none only for the other path",
			prefix, withdrawn, got)
	}
	// A frame the session misread as plain NLRI would have ended A's
	// session, so an Established A also shows ADD-PATH was negotiated.
	ownershipLiveEstablished(t, peers)
}

// addPathLiveSend sends one IPv4 unicast UPDATE on an ADD-PATH session: the
// withdrawal of path identifier withdrawn when it is nonzero, and the
// announcement of prefix as path identifier announced with attrs when attrs
// is set.
// RFC 7911 Section 3: the NLRI "encoding MUST be extended by prepending the
// Path Identifier field, which is of four octets."
//
//	Byte offset: | 0..3: Path Identifier | 4: Length | 5..: Prefix |
func addPathLiveSend(t *testing.T, peer *lowLivePeer, withdrawn uint32, attrs []byte, announced uint32, prefix netip.Prefix) {
	t.Helper()
	update := &message.Update{PathAttributes: attrs}
	if withdrawn != 0 {
		update.WithdrawnRoutes = addPathLiveNLRI(withdrawn, prefix)
	}
	if attrs != nil {
		update.NLRI = addPathLiveNLRI(announced, prefix)
	}
	peer.send(t, message.PackTo(update, nil)) // RFC 4271 Section 4.3.
}

// addPathLiveNLRI writes one ADD-PATH IPv4 NLRI: the path identifier, then
// the RFC 4271 Section 4.3 prefix tuple.
func addPathLiveNLRI(pathID uint32, prefix netip.Prefix) []byte {
	field := binary.BigEndian.AppendUint32(nil, pathID)
	return append(field, ownershipLiveNLRI([]netip.Prefix{prefix})...)
}

// addPathLiveRouter is lowLiveRouter whose remote peers advertise ADD-PATH
// send for IPv4 unicast in their OPEN wherever Ze's settings for them carry
// an ADD-PATH capability. lowLiveRouter's OPEN is fixed, and its file holds
// RFC-tagged units, so the OPEN is built here instead.
func addPathLiveRouter(t *testing.T, settings ...*PeerSettings) []*lowLivePeer {
	t.Helper()
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	for _, s := range settings {
		for _, binding := range []struct{ name, receive string }{
			{"bgp-rib", "update state refresh"},
			{"bgp-rs", "update-received state open-received refresh"},
			{"bgp-adj-rib-in", "update-received state"},
		} {
			if err := EnsureProcessBinding(s, binding.name, binding.receive, "update"); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.AddPeer(s); err != nil {
			t.Fatal(err)
		}
	}
	srv := newBorrowedPluginServer(t, r, "bgp-rib", "bgp-adj-rib-in", "bgp-rs")
	r.SetPluginServer(srv)
	if err := r.StartWithContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)
	out := make([]*lowLivePeer, 0, len(settings))
	for _, s := range settings {
		p := r.peers[s.PeerKey()]
		lowEventually(t, func() bool { return p.currentSession() != nil && p.SessionState() == fsm.StateActive }, "peer active")
		server, remote := net.Pipe()
		v := &lowLivePeer{peer: p, remote: remote, stopped: make(chan struct{})}
		v.startCapture(t)
		t.Cleanup(func() {
			// MUST close before joining: the reader may be waiting for a header.
			_ = remote.Close() //nolint:errcheck // Cleanup: the pipe may already be closed.
			_ = server.Close() //nolint:errcheck // Cleanup: the reactor may have closed it.
			<-v.stopped
		})
		if err := p.acceptConnection(server); err != nil {
			t.Fatal(err)
		}
		open := &message.Open{Version: 4, MyAS: uint16(s.PeerAS), HoldTime: 90, BGPIdentifier: 0x0a000001 + uint32(len(out)),
			OptionalParams: addPathLiveOpenParams(s)}
		v.send(t, message.PackTo(open, nil)) // RFC 4271 Section 4.2.
		lowEventually(t, func() bool { return p.SessionState() == fsm.StateOpenConfirm }, "OPEN accepted")
		v.send(t, message.PackTo(message.NewKeepalive(), nil))
		lowEventually(t, func() bool { return p.State() == PeerStateEstablished && !p.pendingSync() }, "peer initial sync")
		out = append(out, v)
	}
	return out
}

// addPathLiveOpenParams is the remote OPEN's optional parameters: four-octet
// AS and IPv4 unicast, as lowLiveRouter sends, and ADD-PATH send for IPv4
// unicast when Ze's settings for the peer carry an ADD-PATH capability.
// RFC 7911 Section 4: the Capability Value is one or more <AFI (2 octets),
// SAFI (1 octet), Send/Receive (1 octet)> tuples, and Send/Receive 2 says the
// sender is able to send multiple paths.
//
//	Optional parameter octets: | 0: Type=2 | 1: Length=6 | 2: Capability=65 |
//	3: Length=4 | 4..7: ASN | 8: Type=2 | 9: Length=6 | 10: Capability=1 |
//	11: Length=4 | 12..13: AFI=1 | 14: 0 | 15: SAFI=1 | then, for ADD-PATH,
//	16: Type=2 | 17: Length=6 | 18: Capability=69 | 19: Length=4 |
//	20..21: AFI=1 | 22: SAFI=1 | 23: Send/Receive=2 |
func addPathLiveOpenParams(s *PeerSettings) []byte {
	params := []byte{2, 6, 65, 4, 0, 0, byte(s.PeerAS >> 8), byte(s.PeerAS), 2, 6, 1, 4, 0, 1, 0, 1}
	for _, c := range s.Capabilities {
		if _, ok := c.(*capability.AddPath); ok {
			return append(params, 2, 6, 69, 4, 0, 1, 1, byte(capability.AddPathSend))
		}
	}
	return params
}
