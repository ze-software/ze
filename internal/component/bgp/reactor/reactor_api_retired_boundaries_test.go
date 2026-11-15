// Design: docs/architecture/behavior/peer-lifecycle.md -- preserve framing and authority across a retired API writer.
// Related: reactor_api_retired.go -- the ordinary-input suffix retained for another session.
// Related: reactor_api_sealed_session_test.go -- synchronous seal seams and reconnect controls.
// Related: session_ownership_writer_test.go -- actual sent receipts and initial replay authority.

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestAnnounceRetiredQueueRefusalExplainsRecovery checks the failed fallback
// through the public API after encoding seals the old writer. The accepted
// prefix remains queued; the refusal keeps its identity and an operator fix.
func TestAnnounceRetiredQueueRefusalExplainsRecovery(t *testing.T) {
	for _, groups := range []bool{false, true} {
		name := "groups-off"
		if groups {
			name = "groups-on"
		}
		t.Run(name, func(t *testing.T) {
			peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
			session := peer.currentSession()
			t.Cleanup(session.timers.StopAll)
			t.Cleanup(session.closeConn)
			peer.opQueueMax = 1
			adapter := groupUpdatesReactor([]*Peer{peer}, groups)
			require.False(t, peer.shouldQueue())
			batch := gateRailBatch()
			sealing := &sealDuringBuildNLRI{NLRI: batch.NLRIs[0], session: session}
			batch.NLRIs[0] = sealing
			err := adapter.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender())
			require.True(t, sealing.sealed)
			require.ErrorIs(t, err, ErrOpQueueFull)
			require.Empty(t, conn.written())
			peer.mu.RLock()
			queued := len(peer.opQueue)
			peer.mu.RUnlock()
			require.Equal(t, 1, queued)
			var failure *opererr.Error
			require.ErrorAs(t, err, &failure)
			require.NotEmpty(t, failure.Problem())
			require.Contains(t, failure.Why(), "Some routes")
			require.Equal(t, "show bgp peer list", failure.Fix())
			require.Contains(t, failure.Error(), "\nfix: show bgp peer list")
		})
	}
}

// apiRetiredFamilyPeer negotiates the exact family and optional send-side
// ADD-PATH framing, rather than changing a builder flag independently of OPEN.
// The immutable context registry deduplicates these three encoding profiles.
func apiRetiredFamilyPeer(t *testing.T, fam family.Family, addPath bool) (*Peer, *recordingConn) {
	t.Helper()
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	session := peer.currentSession()
	t.Cleanup(session.timers.StopAll)
	t.Cleanup(session.closeConn)
	local := append(peerOffering(fam), &capability.ASN4{ASN: peer.settings.LocalAS})
	remote := append(peerOffering(fam), &capability.ASN4{ASN: peer.settings.PeerAS})
	if addPath {
		local = append(local, &capability.AddPath{Families: []capability.AddPathFamily{{AFI: fam.AFI, SAFI: fam.SAFI, Mode: capability.AddPathSend}}})
		remote = append(remote, &capability.AddPath{Families: []capability.AddPathFamily{{AFI: fam.AFI, SAFI: fam.SAFI, Mode: capability.AddPathReceive}}})
	}
	negotiated := capability.Negotiate(local, remote, capability.PeerIdentity{LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS})
	encoding := bgpctx.FromNegotiatedSend(negotiated)
	contextID, err := bgpctx.Registry.Register(encoding)
	require.NoError(t, err)
	peer.negotiated.Store(NewNegotiatedCapabilities(negotiated))
	peer.sendCtx.Store(encoding)
	peer.sendCtxID = contextID
	session.negotiated = negotiated
	session.sendCtxID = contextID
	session.adjOut = &peer.adjOut
	require.Equal(t, addPath, encoding.AddPath(fam), "the real negotiated send context decides path framing")
	return peer, conn
}

// apiRetiredFamilyBatch supplies host prefixes of fixed encoded length. Its
// expected bytes independently name every prefix and Path Identifier, so a
// suffix beginning inside an identifier cannot pass a prefix-only comparison.
func apiRetiredFamilyBatch(t *testing.T, fam family.Family, addPath bool, count int) (bgptypes.NLRIBatch, []byte, int) {
	t.Helper()
	batch := bgptypes.NLRIBatch{Family: fam, NLRIs: make([]nlri.NLRI, count),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("1.1.1.1"))}
	if fam == family.IPv6Unicast {
		batch.NextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1"))
	}
	var expected []byte
	for i := range batch.NLRIs {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 40, byte(i >> 8), byte(i)}), 32)
		if fam == family.IPv6Unicast {
			address := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0x30}
			binary.BigEndian.PutUint16(address[14:], uint16(i))
			prefix = netip.PrefixFrom(netip.AddrFrom16(address), 128)
		}
		pathID := uint32(0)
		if addPath {
			pathID = uint32(i + 1)
			expected = binary.BigEndian.AppendUint32(expected, pathID)
		}
		batch.NLRIs[i] = nlri.NewINET(fam, prefix, pathID, addPath)
		expected = append(expected, byte(prefix.Bits()))
		expected = append(expected, prefix.Addr().AsSlice()...)
	}
	require.NotEmpty(t, batch.NLRIs)
	return batch, expected, len(expected) / count
}

// apiRetiredFamilyWire borrows the existing UPDATE and attribute framing
// readers. It concatenates announcement bytes without decoding away Path IDs.
func apiRetiredFamilyWire(t *testing.T, stream []byte, fam family.Family) []byte {
	t.Helper()
	var result []byte
	for _, body := range updateBodies(t, stream) {
		withdrawn, attrs, reachable := updateSections(t, body)
		require.Empty(t, withdrawn, "these batches carry announcements only")
		if fam == family.IPv4Unicast {
			result = append(result, reachable...)
			continue
		}
		require.Empty(t, reachable, "the MP family has no legacy IPv4 NLRI")
		_, mp, found := findPathAttr(attrs, byte(attribute.AttrMPReachNLRI))
		require.True(t, found, "each MP announcement must carry MP_REACH")
		require.GreaterOrEqual(t, len(mp), 5)
		require.Equal(t, uint16(fam.AFI), binary.BigEndian.Uint16(mp[:2]))
		require.Equal(t, byte(fam.SAFI), mp[2])
		offset := 5 + int(mp[3])
		require.LessOrEqual(t, offset, len(mp), "the next hop and reserved octet fit MP_REACH")
		result = append(result, mp[offset:]...)
	}
	return result
}

// TestAnnounceRetiredFamilySplit preserves the exact original NLRI suffix when
// a real UPDATE chunk was flushed before sealing. MP framing and four-octet
// ADD-PATH identifiers must not move the retained boundary into a prefix.
func TestAnnounceRetiredFamilySplit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		family  family.Family
		addPath bool
		count   int
	}{
		{name: "legacy-add-path", family: family.IPv4Unicast, addPath: true, count: 455},
		{name: "mp-reach", family: family.IPv6Unicast, count: 238},
		{name: "mp-reach-add-path", family: family.IPv6Unicast, addPath: true, count: 193},
	} {
		for _, groups := range []bool{false, true} {
			name := tc.name + "/groups-off"
			if groups {
				name = tc.name + "/groups-on"
			}
			t.Run(name, func(t *testing.T) {
				peer, conn := apiRetiredFamilyPeer(t, tc.family, tc.addPath)
				session := peer.currentSession()
				adapter := groupUpdatesReactor([]*Peer{peer}, groups)
				batch, expected, routeBytes := apiRetiredFamilyBatch(t, tc.family, tc.addPath, tc.count)
				require.LessOrEqual(t, len(expected), message.MaxMsgLen, "the NLRI build slot holds the requested batch")
				ctx := &sealAfterFlushContext{Context: t.Context(), session: session, conn: conn}
				err := adapter.AnnounceNLRIBatch(ctx, selector.All(), batch, plugin.OperatorSender())
				require.True(t, ctx.sealed, "the real sender must flush before reaching the seal")
				oldWire := conn.written()
				oldNLRI := apiRetiredFamilyWire(t, oldWire, tc.family)
				require.NotEmpty(t, oldNLRI)
				require.Less(t, len(oldNLRI), len(expected), "the seal interrupts a real split")
				require.Zero(t, len(oldNLRI)%routeBytes, "completed output ends on a full NLRI including its path identifier")
				require.Equal(t, expected[:len(oldNLRI)], oldNLRI)
				require.NoError(t, err, "the exact unsent suffix belongs to the replacement session")
				peer.mu.RLock()
				queued := len(peer.opQueue)
				peer.mu.RUnlock()
				require.Equal(t, tc.count-len(oldNLRI)/routeBytes, queued)

				next, nextConn := apiRetiredFamilyPeer(t, tc.family, tc.addPath)
				nextSession := next.currentSession()
				nextSession.adjOut = &peer.adjOut
				peer.mu.Lock()
				peer.session = nextSession
				peer.mu.Unlock()
				nextSession = peer.setTestInitialSync(1, true)
				require.False(t, peer.drainAndCloseQueueGate(nextSession, peer.addrString, message.MaxMsgLen))
				require.Equal(t, expected[len(oldNLRI):], apiRetiredFamilyWire(t, nextConn.written(), tc.family))
				require.Equal(t, oldWire, conn.written(), "the retired connection receives no later bytes")
				requireGateSettled(t, peer)
			})
		}
	}
}

// apiRetiredAuthority drives the API entry with a receipt produced by a real
// successful Session.SendUpdate. A local initial replay additionally uses the
// replacement session's actual token and open initial-sync phase. The unsealed
// companion must emit the route, excluding an always-refusing authority check.
func apiRetiredAuthority(t *testing.T, initial, groups, seal bool) {
	t.Helper()
	peer, conn, _ := ownershipWriterPeer(t)
	old := peer.currentSession()
	t.Cleanup(old.timers.StopAll)
	t.Cleanup(old.closeConn)
	require.NoError(t, old.SendUpdate(ownershipWriterUpdate(t, syncOrderAnnounceBody)))
	receipt := old.sentReceipt.MessageID()
	require.NotZero(t, receipt, "the maintenance request captures a real sent advertisement")
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix(syncOrderPrefix)}, wirePrefixes(t, conn.written(), false))
	batch := adjOutBatch(syncOrderPrefix, "10.0.0.1")
	batch.Replay = true
	batch.SentOwnerMessage = receipt
	session := old
	if initial {
		next, nextConn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
		session = next.currentSession()
		t.Cleanup(session.timers.StopAll)
		t.Cleanup(session.closeConn)
		session.adjOut = &peer.adjOut
		session.sendCtxID = old.sendCtxID
		peer.mu.Lock()
		peer.session = session
		peer.mu.Unlock()
		session = peer.setTestInitialSync(0, true)
		batch.InitialReplay = peer.initialReplayToken()
		batch.InitialLocal = true
		require.NotZero(t, batch.InitialReplay)
		require.Equal(t, session.initialReplay.Load(), batch.InitialReplay)
		conn = nextConn
	}
	adapter := groupUpdatesReactor([]*Peer{peer}, groups)
	require.False(t, peer.shouldQueue(), "the request starts with an open admission gate")
	before := conn.written()
	sealing := &sealDuringBuildNLRI{NLRI: batch.NLRIs[0], session: session}
	if seal {
		batch.NLRIs[0] = sealing
	}
	err := adapter.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender())
	peer.mu.RLock()
	queued := len(peer.opQueue)
	peer.mu.RUnlock()
	require.Zero(t, queued, "captured authority must never become local origination for another session")
	if seal {
		require.True(t, sealing.sealed, "retirement follows API admission and precedes its final writer")
		require.Equal(t, before, conn.written(), "the sealed writer receives no maintenance or replay route")
		// Retired maintenance can be refused or already satisfied; the proof
		// is that neither answer invents a local queue entry or another write.
		return
	}
	require.NoError(t, err)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix(syncOrderPrefix)}, wirePrefixes(t, conn.written()[len(before):], false), "valid live authority must reach the wire through AnnounceNLRIBatch")
}

// TestAnnounceRetiredCapturedReceiptNeverQueues preserves a real same-session
// advertisement receipt across an encoding-to-writer retirement boundary.
func TestAnnounceRetiredCapturedReceiptNeverQueues(t *testing.T) {
	for _, groups := range []bool{false, true} {
		for _, seal := range []bool{false, true} {
			name := "groups-off/live"
			if groups {
				name = "groups-on/live"
			}
			if seal {
				name += "/retired"
			}
			t.Run(name, func(t *testing.T) { apiRetiredAuthority(t, false, groups, seal) })
		}
	}
}

// TestAnnounceRetiredInitialReplayNeverQueues preserves a valid local peer-UP
// replay token without recasting that replay as a new local route after seal.
func TestAnnounceRetiredInitialReplayNeverQueues(t *testing.T) {
	for _, groups := range []bool{false, true} {
		for _, seal := range []bool{false, true} {
			name := "groups-off/live"
			if groups {
				name = "groups-on/live"
			}
			if seal {
				name += "/retired"
			}
			t.Run(name, func(t *testing.T) { apiRetiredAuthority(t, true, groups, seal) })
		}
	}
}
