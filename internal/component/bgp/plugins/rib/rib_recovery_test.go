// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
package rib

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

func recoveryTestCleanup(t *testing.T, r *RIBManager) {
	t.Helper()
	t.Cleanup(func() {
		for _, peer := range r.bgpPeers {
			peer.Release()
		}
		for _, families := range r.ribOut {
			for _, routes := range families {
				for _, entry := range routes {
					entry.release()
				}
			}
		}
	})
}

// recoveryKey names one key and its excluded sources for a test request.
func recoveryKey(raw []byte, excluded ...netip.Addr) ribevents.RecoveryKey {
	return ribevents.RecoveryKey{NLRI: raw, Excluded: excluded}
}

// recoveryReceived installs one received path for peer, with LOCAL_PREF pref
// and message ID id; a non-nil asPath replaces the default AS_PATH. raw carries
// a four-octet path identifier when addPath is set.
func recoveryReceived(r *RIBManager, peer netip.Addr, raw []byte, pref byte, id uint64, addPath bool, asPath []byte) []byte {
	attrs := nativeSentAttrs()
	if asPath != nil {
		attrs = []byte{0x40, 1, 1, 0}
		attrs = append(attrs, asPath...)
	}
	attrs = appendAttr(attrs, byte(attribute.AttrNextHop), 0x40, peer.AsSlice())
	attrs = appendAttr(attrs, byte(attribute.AttrLocalPref), 0x40, []byte{0, 0, 0, pref})
	rib, found := r.bgpPeers[peer]
	if !found {
		rib = storage.NewPeerRIB(peer.String())
		r.bgpPeers[peer], r.peerUp[peer] = rib, true
	}
	if addPath {
		rib.SetAddPath(family.IPv4Unicast, true)
	}
	rib.Insert(family.IPv4Unicast, attrs, raw)
	rib.ModifyFamilyEntry(family.IPv4Unicast, raw, func(entry *storage.RouteEntry) { entry.MsgID = id })
	return attrs
}

// recoverySources lists the survivors' sources in answer order.
func recoverySources(outcome ribevents.RecoveryOutcome) []netip.Addr {
	sources := make([]netip.Addr, len(outcome.Survivors))
	for i, survivor := range outcome.Survivors {
		sources[i] = survivor.Source
	}
	return sources
}

// RFC 4271 Section 6 recalculates the best routes and advertises
// "either withdraws for the routes marked as invalid, or the new best routes".
// PREVENTS: a blind source-DOWN withdrawal replacing an available survivor, or
// replaying the failed source's attributes under the survivor's address.
func TestRecoverySelectsSurvivorWithItsOwnAttributes(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	failed := netip.MustParseAddr("192.0.2.10")
	backup := netip.MustParseAddr("192.0.2.11")
	dest := netip.MustParseAddr("192.0.2.20")
	raw := []byte{24, 198, 51, 100}
	recoveryReceived(r, failed, raw, 200, 80, false, nil)
	backupAttrs := recoveryReceived(r, backup, raw, 100, 81, false, nil)
	event := nativeSentEvent(t, dest, family.IPv4Unicast, raw, backupAttrs, false, false)
	event.RouteMeta["source-message-id"] = float64(80)
	r.handleSent(event)
	request := ribevents.RecoveryRequest{Destination: dest, Family: family.IPv4Unicast,
		Keys: []ribevents.RecoveryKey{recoveryKey(raw, failed)}}
	// RFC 4271 Section 6: query the same whole-set selection used by live changes.
	outcomes, err := r.recoveryRoutes(request)
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.False(t, outcomes[0].Withdraw, "a survivor is a replacement, never a transient withdrawal")
	require.Empty(t, outcomes[0].Sent, "a non-ADD-PATH key's advertisement is named by the writer's owed record")
	require.Len(t, outcomes[0].Survivors, 1)
	survivor := outcomes[0].Survivors[0]
	require.Equal(t, backup, survivor.Source)
	require.Equal(t, uint64(81), survivor.MessageID)
	require.Equal(t, raw, survivor.NLRI)
	require.Equal(t, backup.AsSlice(), survivor.NextHop)
	for _, wanted := range []attribute.AttributeCode{attribute.AttrASPath, attribute.AttrLocalPref, 250} {
		var before, after []byte
		for _, data := range []struct {
			raw    []byte
			target *[]byte
		}{{backupAttrs, &before}, {survivor.Attributes, &after}} {
			it := attribute.NewAttrIterator(data.raw)
			for code, _, value, ok := it.Next(); ok; code, _, value, ok = it.Next() {
				if code == wanted {
					*data.target = bytes.Clone(value)
				}
			}
		}
		require.NotEmpty(t, before)
		require.Equal(t, before, after, "replacement attribute %d", wanted)
	}
	// Reading recovery is not a destructive second received-route lifecycle.
	require.Contains(t, r.bgpPeers, failed)
	// The external command must reach the exact same producer and framing.
	status, answer, err := r.recoveryCommand("", []string{dest.String(), family.IPv4Unicast.String(),
		"false", "false", "false", ribevents.FormatRecoveryKey(&request.Keys[0])})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Equal(t, outcomes, answer)

	// Re-forwarding a backup before the fatal event does not remove the need
	// to advertise the surviving selection after the failed source is removed.
	event.RouteMeta["source-peer"] = backup.String()
	event.RouteMeta["source-message-id"] = float64(81)
	r.handleSent(event)
	// RFC 4271 Section 6: an already advertised survivor remains the replacement.
	outcomes, err = r.recoveryRoutes(request)
	require.NoError(t, err)
	require.Empty(t, outcomes[0].Sent)
	require.Equal(t, []netip.Addr{backup}, recoverySources(outcomes[0]))
	// Without a survivor, the answer is WITHDRAW. Whose advertisement it
	// withdraws is the reactor writer's owed record, which a departure creates
	// only where the departed source's ended session owns the entry
	// (TestAdjOutDepartureOwesOnlyDepartedSession).
	r.peerUp[backup] = false
	outcomes, err = r.recoveryRoutes(request)
	require.NoError(t, err)
	require.True(t, outcomes[0].Withdraw)
	require.Empty(t, outcomes[0].Sent, "a non-ADD-PATH key names no Sent")
	r.peerUp[backup] = true

	// RFC 4271 Section 3.1: a later advertisement already replaced the old path.
	event.RouteMeta["source-peer"] = failed.String()
	event.RouteMeta["source-message-id"] = float64(91)
	r.handleSent(event)
	outcomes, err = r.recoveryRoutes(request)
	require.NoError(t, err)
	require.Empty(t, outcomes[0].Sent, "a non-ADD-PATH key names no Sent")

	// With no survivor, only the failed advertisement is withdrawn.
	r.peerUp[backup] = false
	event.RouteMeta["source-message-id"] = float64(80)
	r.handleSent(event)
	// RFC 4271 Section 6: no feasible replacement leaves a withdrawal.
	outcomes, err = r.recoveryRoutes(request)
	require.NoError(t, err)
	require.True(t, outcomes[0].Withdraw)
	require.Empty(t, outcomes[0].Survivors)
	require.Empty(t, outcomes[0].Sent)
}

// TestRecoveryOwnerWithdrawalKeepsSourceOtherPaths asks with an EMPTY excluded
// set, as an owner withdrawal does: the withdrawing source's other received
// path stays electable (AC-9).
// PREVENTS: an owner withdrawal excluding the source's surviving ADD-PATH path.
func TestRecoveryOwnerWithdrawalKeepsSourceOtherPaths(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	owner := netip.MustParseAddr("192.0.2.10")
	other := netip.MustParseAddr("192.0.2.11")
	dest := netip.MustParseAddr("192.0.2.20")
	prefix := []byte{24, 198, 51, 100}
	remaining := append([]byte{0, 0, 0, 2}, prefix...)
	recoveryReceived(r, owner, remaining, 200, 80, true, nil)
	recoveryReceived(r, other, prefix, 100, 81, false, nil)
	outcomes, err := r.recoveryRoutes(ribevents.RecoveryRequest{Destination: dest, Family: family.IPv4Unicast,
		Keys: []ribevents.RecoveryKey{recoveryKey(prefix)}})
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.False(t, outcomes[0].Withdraw)
	require.Equal(t, []netip.Addr{owner, other}, recoverySources(outcomes[0]))
	first := outcomes[0].Survivors[0]
	require.Equal(t, uint32(2), first.PathID, "the received path identifier travels with the candidate")
	require.True(t, first.AddPath)
	require.Equal(t, uint64(80), first.MessageID)
	second := outcomes[0].Survivors[1]
	require.Zero(t, second.PathID)
	require.False(t, second.AddPath)
}

// TestRecoveryOwnerWithdrawalAnswersWithdrawWhenNoSurvivor asks with an empty
// set for a key no source holds any more (AC-2).
// PREVENTS: an empty answer read as "nothing to do" instead of WITHDRAW.
func TestRecoveryOwnerWithdrawalAnswersWithdrawWhenNoSurvivor(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	dest := netip.MustParseAddr("192.0.2.20")
	prefix := []byte{24, 198, 51, 100}
	outcomes, err := r.recoveryRoutes(ribevents.RecoveryRequest{Destination: dest, Family: family.IPv4Unicast,
		Keys: []ribevents.RecoveryKey{recoveryKey(prefix)}})
	require.NoError(t, err)
	require.Equal(t, []ribevents.RecoveryOutcome{{NLRI: prefix, Withdraw: true}}, outcomes)
}

// TestRecoveryElectsWithoutSentEntry asks for a key the destination's sent
// inventory does not hold (round-2 I-4).
// PREVENTS: the election depending on a ribOut entry and answering nothing.
func TestRecoveryElectsWithoutSentEntry(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	source := netip.MustParseAddr("192.0.2.11")
	dest := netip.MustParseAddr("192.0.2.20")
	prefix := []byte{24, 198, 51, 100}
	recoveryReceived(r, source, prefix, 100, 81, false, nil)
	require.Empty(t, r.ribOut[dest])
	request := ribevents.RecoveryRequest{Destination: dest, Family: family.IPv4Unicast,
		Keys: []ribevents.RecoveryKey{recoveryKey(prefix)}}
	outcomes, err := r.recoveryRoutes(request)
	require.NoError(t, err)
	require.False(t, outcomes[0].Withdraw)
	require.Equal(t, []netip.Addr{source}, recoverySources(outcomes[0]))
	r.peerUp[source] = false
	outcomes, err = r.recoveryRoutes(request)
	require.NoError(t, err)
	require.True(t, outcomes[0].Withdraw)
	require.Empty(t, outcomes[0].Survivors)
}

// TestRecoveryDepartureExcludesEverySourceCandidate excludes a departed source
// that holds two ADD-PATH paths for the prefix: neither is a candidate.
// PREVENTS: a departure electing the departed source's second path.
func TestRecoveryDepartureExcludesEverySourceCandidate(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	departed := netip.MustParseAddr("192.0.2.10")
	survivor := netip.MustParseAddr("192.0.2.11")
	dest := netip.MustParseAddr("192.0.2.20")
	prefix := []byte{24, 198, 51, 100}
	for id := range byte(2) {
		recoveryReceived(r, departed, append([]byte{0, 0, 0, id + 1}, prefix...), 250, 80, true, nil)
	}
	recoveryReceived(r, survivor, prefix, 100, 81, false, nil)
	outcomes, err := r.recoveryRoutes(ribevents.RecoveryRequest{Destination: dest, Family: family.IPv4Unicast,
		Keys: []ribevents.RecoveryKey{recoveryKey(prefix, departed)}})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{survivor}, recoverySources(outcomes[0]))
}

// TestRecoveryMergedExclusionSet sends one key excluding two departed sources
// beside one with an empty set (round-2 I-3).
// PREVENTS: a merged departure honoring only one source, or an exclusion
// leaking from one key to another in the same request.
func TestRecoveryMergedExclusionSet(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	first := netip.MustParseAddr("192.0.2.10")
	second := netip.MustParseAddr("192.0.2.12")
	survivor := netip.MustParseAddr("192.0.2.11")
	dest := netip.MustParseAddr("192.0.2.20")
	merged := []byte{24, 198, 51, 100}
	open := []byte{24, 198, 51, 101}
	for _, raw := range [][]byte{merged, open} {
		recoveryReceived(r, first, raw, 250, 80, false, nil)
		recoveryReceived(r, second, raw, 200, 82, false, nil)
		recoveryReceived(r, survivor, raw, 100, 81, false, nil)
	}
	outcomes, err := r.recoveryRoutes(ribevents.RecoveryRequest{Destination: dest, Family: family.IPv4Unicast,
		Keys: []ribevents.RecoveryKey{recoveryKey(merged, first, second), recoveryKey(open)}})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{survivor}, recoverySources(outcomes[0]))
	require.Equal(t, []netip.Addr{first, second, survivor}, recoverySources(outcomes[1]))
}

// TestRecoveryExcludesDestinationCandidate makes the destination's own path
// the best one (AC-13), for an empty and a non-empty excluded set.
// PREVENTS: the destination being offered its own route back, or a withdrawal
// where another source survives.
func TestRecoveryExcludesDestinationCandidate(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	owner := netip.MustParseAddr("192.0.2.10")
	survivor := netip.MustParseAddr("192.0.2.11")
	dest := netip.MustParseAddr("192.0.2.20")
	prefix := []byte{24, 198, 51, 100}
	recoveryReceived(r, dest, prefix, 250, 79, false, nil)
	recoveryReceived(r, owner, prefix, 200, 80, false, nil)
	recoveryReceived(r, survivor, prefix, 100, 81, false, nil)
	for _, tc := range []struct {
		excluded []netip.Addr
		want     []netip.Addr
	}{{nil, []netip.Addr{owner, survivor}}, {[]netip.Addr{owner}, []netip.Addr{survivor}}} {
		outcomes, err := r.recoveryRoutes(ribevents.RecoveryRequest{Destination: dest, Family: family.IPv4Unicast,
			Keys: []ribevents.RecoveryKey{recoveryKey(prefix, tc.excluded...)}})
		require.NoError(t, err)
		require.Equal(t, tc.want, recoverySources(outcomes[0]))
	}
}

// TestRecoveryAnswersOrderedCandidates checks SURVIVORS is repeated SelectBest
// order (AC-5, round-2 N-1): once on a LOCAL_PREF ladder whose order is known,
// once on a MED set over two neighbor ASes where the comparator is not
// transitive, so no sort could produce the order.
// PREVENTS: survivors in storage order, or sorted with a non-transitive comparator.
func TestRecoveryAnswersOrderedCandidates(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	dest := netip.MustParseAddr("192.0.2.20")
	low, mid, high := netip.MustParseAddr("192.0.2.11"), netip.MustParseAddr("192.0.2.12"), netip.MustParseAddr("192.0.2.13")
	ladder := []byte{24, 198, 51, 100}
	recoveryReceived(r, low, ladder, 100, 81, false, nil)
	recoveryReceived(r, high, ladder, 250, 83, false, nil)
	recoveryReceived(r, mid, ladder, 200, 82, false, nil)
	medSet := []byte{24, 198, 51, 101}
	for i, path := range []struct {
		peer netip.Addr
		as   byte
		med  byte
	}{{low, 1, 50}, {mid, 2, 10}, {high, 1, 20}} {
		attrs := recoveryReceived(r, path.peer, medSet, 100, uint64(90+i), false, []byte{0x40, 2, 4, 2, 1, 0xfd, path.as})
		attrs = appendAttr(attrs, byte(attribute.AttrMED), 0x80, []byte{0, 0, 0, path.med})
		r.bgpPeers[path.peer].Insert(family.IPv4Unicast, attrs, medSet)
	}
	outcomes, err := r.recoveryRoutes(ribevents.RecoveryRequest{Destination: dest, Family: family.IPv4Unicast,
		Keys: []ribevents.RecoveryKey{recoveryKey(ladder), recoveryKey(medSet)}})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{high, mid, low}, recoverySources(outcomes[0]))

	r.peerMu.RLock()
	remaining := r.gatherPrefixCandidatesLocked(family.IPv4Unicast, netip.MustParsePrefix("198.51.101.0/24"))
	var want []netip.Addr
	for len(remaining) > 0 {
		best := SelectBest(remaining)
		want = append(want, best.PeerIP)
		for i, candidate := range remaining {
			if candidate == best {
				remaining = append(remaining[:i], remaining[i+1:]...)
				break
			}
		}
		releaseCandidates([]*Candidate{best})
	}
	r.peerMu.RUnlock()
	require.Len(t, want, 3)
	require.Equal(t, want, recoverySources(outcomes[1]))
}

// TestRecoveryAnswersEveryRequestedKey checks one outcome per key, in request
// order, duplicates included, and that the reactor-side check refuses an
// answer missing a key or holding no outcome (AC-11).
// PREVENTS: a dropped key read downstream as "no survivor".
func TestRecoveryAnswersEveryRequestedKey(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	source := netip.MustParseAddr("192.0.2.11")
	dest := netip.MustParseAddr("192.0.2.20")
	held := []byte{24, 198, 51, 100}
	gone := []byte{24, 198, 51, 101}
	recoveryReceived(r, source, held, 100, 81, false, nil)
	request := ribevents.RecoveryRequest{Destination: dest, Family: family.IPv4Unicast,
		Keys: []ribevents.RecoveryKey{recoveryKey(gone), recoveryKey(held), recoveryKey(gone)}}
	outcomes, err := r.recoveryRoutes(request)
	require.NoError(t, err)
	require.NoError(t, ribevents.CheckRecoveryAnswer(&request, outcomes))
	require.Len(t, outcomes, 3)
	require.True(t, outcomes[0].Withdraw)
	require.False(t, outcomes[1].Withdraw)
	require.True(t, outcomes[2].Withdraw)
	require.Error(t, ribevents.CheckRecoveryAnswer(&request, outcomes[:2]), "a missing key is a lookup failure")
	swapped := []ribevents.RecoveryOutcome{outcomes[1], outcomes[0], outcomes[2]}
	require.Error(t, ribevents.CheckRecoveryAnswer(&request, swapped), "an outcome names its own key")
	empty := []ribevents.RecoveryOutcome{outcomes[0], {NLRI: held}, outcomes[2]}
	require.Error(t, ribevents.CheckRecoveryAnswer(&request, empty), "neither WITHDRAW nor SURVIVORS is no outcome")
	both := []ribevents.RecoveryOutcome{outcomes[0], outcomes[1], outcomes[2]}
	both[1].Withdraw = true
	require.Error(t, ribevents.CheckRecoveryAnswer(&request, both), "WITHDRAW and SURVIVORS exclude each other")
	for _, malformed := range []string{"18c63364", "zz=", "=", "18c63364=not-an-address"} {
		_, err := ribevents.ParseRecoveryKey(malformed)
		require.Error(t, err, malformed)
	}
}

// RFC 7911 Section 3 identifies a route by prefix AND the
// advertised identifier. RFC 8277 Section 2.4 uses one Compatibility field on
// withdrawal, regardless of the former label stack. Zero is a valid identifier.
// PREVENTS: native source-DOWN recovery withdrawing another source's path or
// copying ingress identifiers over independently assigned egress identifiers.
func TestRecoveryAddPathDepartureWithdrawsEverySentIdentifier(t *testing.T) {
	vpn4 := []byte{112, 0, 6, 65, 0, 0, 0, 1, 0, 0, 0, 2, 198, 51, 100}
	for _, tc := range []struct {
		name string
		fam  family.Family
		raw  []byte
	}{
		{"ipv4", family.IPv4Unicast, []byte{24, 198, 51, 100}},
		{"ipv6", family.IPv6Unicast, []byte{32, 32, 1, 13, 184}},
		{"labeled", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}, []byte{72, 0, 6, 64, 0, 12, 129, 198, 51, 100}},
		{"vpn4", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}, vpn4},
		{"evpn", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, nativeEVPNRoute()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManager(t)
			recoveryTestCleanup(t, r)
			failed := netip.MustParseAddr("192.0.2.10")
			backup := netip.MustParseAddr("192.0.2.11")
			dest := netip.MustParseAddr("192.0.2.20")
			for _, path := range []struct {
				id      uint32
				source  netip.Addr
				message uint64
			}{
				{0, failed, 80}, {7, failed, 80}, {17, backup, 81}, {99, failed, 91},
			} {
				raw := make([]byte, 4+len(tc.raw))
				binary.BigEndian.PutUint32(raw, path.id)
				copy(raw[4:], tc.raw)
				event := nativeSentEvent(t, dest, tc.fam, raw, nativeSentAttrs(), true, false)
				event.RouteMeta["source-peer"] = path.source.String()
				event.RouteMeta["source-message-id"] = float64(path.message)
				r.handleSent(event)
			}
			request := ribevents.RecoveryRequest{Destination: dest, Family: tc.fam,
				Keys: []ribevents.RecoveryKey{recoveryKey(tc.raw, failed)}, SentAddPath: true}
			// RFC 7911 Section 3: the failed source's advertised IDs depart,
			// its reconnected identifier 99 included: the RIB holds no session
			// generation, so the reactor's writer is the reconnect guard
			// (TestAdjOutDepartureOwesOnlyDepartedSession).
			outcomes, err := r.recoveryRoutes(request)
			require.NoError(t, err)
			require.Len(t, outcomes, 1)
			require.True(t, outcomes[0].Withdraw, "ADD-PATH departure runs no election")
			require.Empty(t, outcomes[0].Survivors)
			require.Len(t, outcomes[0].Sent, 3)
			ids := make(map[uint32]bool)
			for _, sent := range outcomes[0].Sent {
				key, ok := ribOutRouteKeyForAction(tc.fam, sent, true, true)
				require.True(t, ok)
				ids[key.PathID] = true
				require.Contains(t, r.ribOut[dest][tc.fam], key)
				if tc.fam.SAFI == family.SAFIMPLSLabel || tc.fam.SAFI == family.SAFIVPN {
					require.Equal(t, []byte{0x80, 0, 0}, sent[5:8])
				}
				r.handleSent(nativeSentEvent(t, dest, tc.fam, sent, nil, true, true))
			}
			require.Equal(t, map[uint32]bool{0: true, 7: true, 99: true}, ids)
			require.Len(t, r.ribOut[dest][tc.fam], 1, "only the survivor remains")
			// RFC 7911 Section 3: repeating DOWN cannot withdraw a different path.
			outcomes, err = r.recoveryRoutes(request)
			require.NoError(t, err)
			require.Empty(t, outcomes[0].Sent)
		})
	}
}

// RFC 8277 Section 2.4 encodes a labeled withdrawal as one
// Compatibility field and the prefix, even when the advertisement had a stack.
// PREVENTS: the text withdrawal grammar losing the labeled route's sent identity.
func TestRecoveryLabeledTextPrefixMatchesNativeSentStack(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	failed := netip.MustParseAddr("192.0.2.10")
	dest := netip.MustParseAddr("192.0.2.20")
	advertised := []byte{72, 0, 6, 64, 0, 12, 129, 198, 51, 100}
	r.handleSent(nativeSentEvent(t, dest, fam, advertised, nativeSentAttrs(), false, false))
	// RFC 8277 Section 2.4: the label-free text name is the same route identity.
	identity, err := recoveryIdentity(fam, []byte{24, 198, 51, 100}, false, true)
	require.NoError(t, err)
	require.Contains(t, r.ribOut[dest][fam], identity, "the text key names the advertised stack's identity")
	outcomes, err := r.recoveryRoutes(ribevents.RecoveryRequest{Destination: dest, Family: fam,
		Keys: []ribevents.RecoveryKey{recoveryKey([]byte{24, 198, 51, 100}, failed)}, PrefixOnly: true})
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.True(t, outcomes[0].Withdraw)
}

// TestRecoveryBatchedIdentitiesShareOneElectionResult drives both lookup
// surfaces with many distinct prefixes and repeated ingress identities. Each
// requested key is answered once, the answer names every egress ID the failed
// source owns, its reconnected identifier included (the reactor's writer is
// the reconnect guard), and survives actual sent-withdrawal projection without
// a restart for each path.
func TestRecoveryBatchedIdentitiesShareOneElectionResult(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	failed := netip.MustParseAddr("192.0.2.10")
	backup := netip.MustParseAddr("192.0.2.11")
	destination := netip.MustParseAddr("192.0.2.20")
	request := ribevents.RecoveryRequest{Destination: destination,
		Family: family.IPv4Unicast, AddPath: true, SentAddPath: true}
	args := []string{destination.String(), request.Family.String(), "true", "false", "true"}
	for prefix := range 64 {
		raw := []byte{0, 0, 0, 42, 24, 198, 51, byte(prefix)}
		key := recoveryKey(raw, failed)
		request.Keys = append(request.Keys, key, key)
		args = append(args, ribevents.FormatRecoveryKey(&key), ribevents.FormatRecoveryKey(&key))
		for _, path := range []struct {
			id      uint32
			source  netip.Addr
			message uint64
		}{{0, failed, 80}, {7, failed, 80}, {17, backup, 81}, {99, failed, 91}} {
			sent := bytes.Clone(raw)
			binary.BigEndian.PutUint32(sent, path.id)
			event := nativeSentEvent(t, destination, request.Family, sent, nativeSentAttrs(), true, false)
			event.RouteMeta["source-peer"] = path.source.String()
			event.RouteMeta["source-message-id"] = float64(path.message)
			r.handleSent(event)
		}
	}
	outcomes, err := r.recoveryRoutes(request)
	require.NoError(t, err)
	require.NoError(t, ribevents.CheckRecoveryAnswer(&request, outcomes))
	status, document, err := r.recoveryCommand("", args)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	external, ok := document.([]ribevents.RecoveryOutcome)
	require.True(t, ok)
	require.Len(t, external, len(outcomes), "external IPC must use the same batched producer")
	unique := make(map[string][]byte)
	for i, outcome := range outcomes {
		require.True(t, outcome.Withdraw)
		require.Len(t, outcome.Sent, 3)
		require.ElementsMatch(t, outcome.Sent, external[i].Sent)
		for _, sent := range outcome.Sent {
			require.Contains(t, []uint32{0, 7, 99}, binary.BigEndian.Uint32(sent))
			unique[string(sent)] = sent
		}
	}
	require.Len(t, unique, 192, "every prefix and every failed sent identifier exactly once")
	for _, sent := range unique {
		r.handleSent(nativeSentEvent(t, destination, request.Family, sent, nil, true, true))
	}
	require.Len(t, r.ribOut[destination][request.Family], 64, "only the survivors remain")
	outcomes, err = r.recoveryRoutes(request)
	require.NoError(t, err)
	for _, outcome := range outcomes {
		require.Empty(t, outcome.Sent)
	}
}
