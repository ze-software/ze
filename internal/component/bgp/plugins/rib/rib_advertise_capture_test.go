// Design: docs/architecture/plugin/rib-storage-design.md -- retained selected output
// Related: rib_advertise_capture.go -- immutable learned-selection snapshots
package rib

import (
	"bytes"
	"math"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

const (
	advertiseCaptureOwner            uint64 = 701
	advertiseCaptureAlternativeOwner uint64 = 702
)

// advertiseCaptureRIB retains real received storage and resolves its next hop
// through the election's normal forwarding table. Each test owns its manager.
func advertiseCaptureRIB(t *testing.T) (*RIBManager, netip.Addr) {
	t.Helper()
	r, calls := sourceLeaseTestRIB(t, true)
	// These are the original fixture actors and issuing sessions. The SDK
	// boundary refuses an unissued receipt or a receipt rebound to another owner.
	calls.mu.Lock()
	calls.issued = map[uint64]uint64{
		701: advertiseCaptureOwner,
		702: advertiseCaptureAlternativeOwner,
		99:  advertiseCaptureOwner,
		100: advertiseCaptureOwner,
	}
	calls.mu.Unlock()
	peer := netip.MustParseAddr("192.0.2.71")
	electionFixtureLocRIB(t, r, peer)
	r.peerMeta[advertiseCaptureOwner] = &peerMetadata{Address: peer, PeerASN: 65001, LocalASN: 65000}
	r.peerUp[advertiseCaptureOwner] = true
	var err error
	r.bgpPeers[advertiseCaptureOwner], err = storage.NewAddressPeerRIB(peer)
	require.NoError(t, err)
	return r, peer
}

// advertiseCapturePath installs the message/epoch on the exact retained entry.
// The snapshot MUST read this entry, never the source's current session epoch.
func advertiseCapturePath(t *testing.T, r *RIBManager, owner uint64, fam family.Family, raw, attrs []byte, message, epoch uint64) {
	t.Helper()
	routes := r.bgpPeers[owner]
	require.NotNil(t, routes)
	routes.Insert(fam, attrs, raw)
	require.True(t, routes.ModifyFamilyEntry(fam, raw, func(entry *storage.RouteEntry) {
		entry.MsgID = message
		entry.SourcePolicyEpoch = epoch
	}))
	advertiseCaptureReceipt(t, r, owner, fam, raw, owner, 41)
}

// advertiseCaptureReceipt replaces the issuing Session on an existing path.
// The caller MUST supply its original physical connection. This helper MUST
// acquire through the SDK before storing the reference; metadata supplies only
// the observed address, never an owner, connection or receipt.
func advertiseCaptureReceipt(t *testing.T, r *RIBManager, owner uint64, fam family.Family, raw []byte, receipt, connectionID uint64) {
	t.Helper()
	meta, observed := r.peerMeta[owner]
	require.True(t, observed)
	require.NotNil(t, meta)
	ref, err := r.sourceLeases.acquire(t.Context(), meta.Address, owner, receipt, connectionID)
	require.NoError(t, err)
	require.NotNil(t, ref)
	defer ref.Release()
	require.True(t, r.bgpPeers[owner].ModifyFamilyEntry(fam, raw, func(entry *storage.RouteEntry) {
		entry.SetSourceReference(ref)
		entry.SourceReceipt = receipt
	}))
}

// TestAdvertiseCaptureOwnsElectedRevision holds an old captured winner across
// storage replacement and release, then checks its bytes and provenance.
func TestAdvertiseCaptureOwnsElectedRevision(t *testing.T) {
	r, peer := advertiseCaptureRIB(t)
	fam := family.IPv4Unicast
	raw := ipv4Prefix(24, 198, 18, 71)
	attrs := unicastAttrs(peer.As4(), 30, 100)
	advertiseCapturePath(t, r, advertiseCaptureOwner, fam, raw, attrs, 71, 701)
	_, changed := r.checkBestPathChange(fam, raw, false, nil)
	require.True(t, changed, "the ordinary election must choose a useful winner first")
	first, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Positive(t, first[0].revision)
	require.Equal(t, fam, first[0].key.family)
	require.Equal(t, raw, first[0].keyNLRI)
	require.Equal(t, raw, first[0].route.NLRI)
	require.Equal(t, peer, first[0].route.Source)
	require.Equal(t, uint64(71), first[0].route.MessageID)
	require.Equal(t, uint64(701), first[0].route.SourcePolicyEpoch)
	require.Equal(t, attrs, first[0].route.Attributes)
	require.Equal(t, peer.AsSlice(), first[0].route.NextHop)
	saved := bytes.Clone(first[0].route.Attributes)

	attrs[3] = 2
	advertiseCapturePath(t, r, advertiseCaptureOwner, fam, raw, attrs, 72, 702)
	_, changed = r.checkBestPathChange(fam, raw, false, nil)
	require.True(t, changed)
	second, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Greater(t, second[0].revision, first[0].revision)
	require.Equal(t, uint64(72), second[0].route.MessageID)
	require.Equal(t, uint64(702), second[0].route.SourcePolicyEpoch)
	require.NotEqual(t, saved, second[0].route.Attributes)
	require.Equal(t, saved, first[0].route.Attributes, "the old retained snapshot must outlive pool replacement")
	require.Equal(t, uint64(701), first[0].route.SourcePolicyEpoch)
}

// TestAdvertiseCaptureUnchangedWinnerKeepsRevision distinguishes a real
// selected change from an identical receipt and a losing path's churn.
func TestAdvertiseCaptureUnchangedWinnerKeepsRevision(t *testing.T) {
	r, peer := advertiseCaptureRIB(t)
	fam := family.IPv4Unicast
	raw := ipv4Prefix(24, 198, 18, 72)
	attrs := unicastAttrs(peer.As4(), 30, 200)
	advertiseCapturePath(t, r, advertiseCaptureOwner, fam, raw, attrs, 80, 801)
	_, changed := r.checkBestPathChange(fam, raw, false, nil)
	require.True(t, changed)
	first, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, first, 1)

	_, changed = r.checkBestPathChange(fam, raw, false, nil)
	require.False(t, changed)
	advertiseCapturePath(t, r, advertiseCaptureOwner, fam, raw, attrs, 82, 801)
	_, changed = r.checkBestPathChange(fam, raw, false, nil)
	require.False(t, changed, "identical route receipt does not change the selected route")
	loser := netip.MustParseAddr("192.0.2.72")
	electionFixtureLocRIB(t, r, peer, loser)
	r.peerMeta[advertiseCaptureAlternativeOwner] = &peerMetadata{Address: loser, PeerASN: 65001, LocalASN: 65000}
	r.bgpPeers[advertiseCaptureAlternativeOwner], err = storage.NewAddressPeerRIB(loser)
	require.NoError(t, err)
	advertiseCapturePath(t, r, advertiseCaptureAlternativeOwner, fam, raw, unicastAttrs(loser.As4(), 30, 100), 81, 802)
	_, changed = r.checkBestPathChange(fam, raw, false, nil)
	require.False(t, changed, "a lower LOCAL_PREF path cannot nominate a selected advertisement")
	r.bgpPeers[advertiseCaptureAlternativeOwner].Remove(fam, raw)
	_, changed = r.checkBestPathChange(fam, raw, false, nil)
	require.False(t, changed)
	current, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Equal(t, first, current, "unchanged selection keeps both provenance and revision")
}

// TestAdvertiseCaptureEpochChangeRefreshesSnapshot changes only the stored
// policy admission, as source-policy abort does, without changing attributes.
func TestAdvertiseCaptureEpochChangeRefreshesSnapshot(t *testing.T) {
	r, peer := advertiseCaptureRIB(t)
	fam := family.IPv4Unicast
	raw := ipv4Prefix(24, 198, 18, 73)
	advertiseCapturePath(t, r, advertiseCaptureOwner, fam, raw, unicastAttrs(peer.As4(), 30, 100), 90, 901)
	_, changed := r.checkBestPathChange(fam, raw, false, nil)
	require.True(t, changed)
	first, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, first, 1)
	r.bgpPeers[advertiseCaptureOwner].ModifyFamilyEntry(fam, raw, func(entry *storage.RouteEntry) {
		entry.SourcePolicyEpoch = 902
	})
	r.checkBestPathChange(fam, raw, false, nil)
	current, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Greater(t, current[0].revision, first[0].revision)
	require.Equal(t, uint64(902), current[0].route.SourcePolicyEpoch)
	require.Equal(t, uint64(901), first[0].route.SourcePolicyEpoch, "an old capture is never restamped")
	require.Equal(t, first[0].route.Attributes, current[0].route.Attributes)
}

// TestAdvertiseCaptureStaleChangeRefreshesSnapshot drives the real GR command
// and observes the retained winner's LLGR treatment without another UPDATE.
func TestAdvertiseCaptureStaleChangeRefreshesSnapshot(t *testing.T) {
	r, peer := advertiseCaptureRIB(t)
	fam := family.IPv4Unicast
	raw := ipv4Prefix(24, 198, 18, 74)
	advertiseCapturePath(t, r, advertiseCaptureOwner, fam, raw, unicastAttrs(peer.As4(), 30, 100), 100, 1001)
	_, changed := r.checkBestPathChange(fam, raw, false, nil)
	require.True(t, changed)
	first, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, first, 1)
	status, _, err := r.markStaleCommand([]string{peer.String(), "0", "2"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	current, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Greater(t, current[0].revision, first[0].revision)
	require.Equal(t, uint8(2), current[0].staleLevel)
	require.Zero(t, first[0].staleLevel)
	require.Equal(t, uint64(1001), current[0].route.SourcePolicyEpoch)
}

// TestAdvertiseCaptureNativePathZeroKeepsIndependentKeys elects two VPN/EVPN
// keys and replaces source path zero of one without changing the other.
func TestAdvertiseCaptureNativePathZeroKeepsIndependentKeys(t *testing.T) {
	for _, tc := range opaqueFamilyCases() {
		t.Run(tc.name, func(t *testing.T) {
			r, peer := advertiseCaptureRIB(t)
			routes := r.bgpPeers[advertiseCaptureOwner]
			routes.SetAddPath(tc.fam, true)
			other := bytes.Clone(tc.key)
			if tc.fam == vpnv4Family {
				other[11] = 2 // The independent VPN route has another RD.
			} else {
				other[9] = 2 // The independent EVPN route has another RD.
			}
			zero := framedKey(0, tc.key)
			advertiseCapturePath(t, r, advertiseCaptureOwner, tc.fam, zero, unicastAttrs(peer.As4(), 30, 100), 110, 1101)
			advertiseCapturePath(t, r, advertiseCaptureOwner, tc.fam, framedKey(0, other), unicastAttrs(peer.As4(), 30, 100), 111, 1101)
			r.checkBestPathChange(tc.fam, zero, true, nil)
			r.checkBestPathChange(tc.fam, framedKey(0, other), true, nil)
			first, err := r.advertiseSnapshot()
			require.NoError(t, err)
			require.Len(t, first, 2, "native routes sharing an IP portion have separate selections")
			for _, selected := range first {
				require.True(t, selected.route.AddPath)
				require.Zero(t, selected.route.PathID, "zero is an actual ADD-PATH identity")
				require.Equal(t, selected.route.NLRI[4:], selected.keyNLRI)
			}
			replacement := framedKey(9, tc.key)
			advertiseCapturePath(t, r, advertiseCaptureOwner, tc.fam, replacement, unicastAttrs(peer.As4(), 30, 200), 112, 1102)
			r.checkBestPathChange(tc.fam, replacement, true, nil)
			current, err := r.advertiseSnapshot()
			require.NoError(t, err)
			require.Len(t, current, 2)
			var replaced, survivor bool
			for _, selected := range current {
				if bytes.Equal(selected.keyNLRI, tc.key) {
					replaced = true
					require.Equal(t, uint32(9), selected.route.PathID)
					require.Equal(t, replacement, selected.route.NLRI)
					require.Equal(t, uint64(112), selected.route.MessageID)
					continue
				}
				survivor = true
				require.Equal(t, other, selected.keyNLRI)
				require.Equal(t, uint64(111), selected.route.MessageID)
			}
			require.True(t, replaced)
			require.True(t, survivor)
		})
	}
}

// TestAdvertiseCaptureAbsenceReleasesUnsentKeys churns keys before any output
// is admitted. A later reappearance must have a newer revision without keeping
// withdrawn-key history in the current selection map.
func TestAdvertiseCaptureAbsenceReleasesUnsentKeys(t *testing.T) {
	r, peer := advertiseCaptureRIB(t)
	fam := family.IPv4Unicast
	var firstRevision uint64
	for key := range byte(64) {
		raw := ipv4Prefix(24, 198, 19, key)
		advertiseCapturePath(t, r, advertiseCaptureOwner, fam, raw, unicastAttrs(peer.As4(), 30, 100), uint64(key)+1, 1201)
		r.checkBestPathChange(fam, raw, false, nil)
		selected, err := r.advertiseSnapshot()
		require.NoError(t, err)
		require.Len(t, selected, 1)
		if key == 0 {
			firstRevision = selected[0].revision
		}
		r.bgpPeers[advertiseCaptureOwner].Remove(fam, raw)
		_, changed := r.checkBestPathChange(fam, raw, false, nil)
		require.True(t, changed, "an elected route's absence is a useful retirement change")
		selected, err = r.advertiseSnapshot()
		require.NoError(t, err)
		require.Empty(t, selected)
		r.advertise.mu.Lock()
		retained := len(r.advertise.current)
		r.advertise.mu.Unlock()
		require.Zero(t, retained, "never-admitted withdrawn keys leave no tombstones")
	}
	raw := ipv4Prefix(24, 198, 19, 0)
	advertiseCapturePath(t, r, advertiseCaptureOwner, fam, raw, unicastAttrs(peer.As4(), 30, 100), 200, 1202)
	r.checkBestPathChange(fam, raw, false, nil)
	selected, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, selected, 1)
	require.Greater(t, selected[0].revision, firstRevision, "releasing absence state cannot reuse an old revision")
}

// TestAdvertiseCaptureRevisionExhaustionFailsClosed reaches the last valid
// revision and then refuses another capture instead of wrapping into authority
// that an earlier delayed write could overtake.
func TestAdvertiseCaptureRevisionExhaustionFailsClosed(t *testing.T) {
	r, peer := advertiseCaptureRIB(t)
	fam := family.IPv4Unicast
	raw := ipv4Prefix(24, 198, 18, 75)
	r.advertise.revision = math.MaxUint64 - 1
	advertiseCapturePath(t, r, advertiseCaptureOwner, fam, raw, unicastAttrs(peer.As4(), 30, 100), 210, 2101)
	r.checkBestPathChange(fam, raw, false, nil)
	last, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, last, 1)
	require.Equal(t, uint64(math.MaxUint64), last[0].revision)
	advertiseCapturePath(t, r, advertiseCaptureOwner, fam, raw, unicastAttrs(peer.As4(), 20, 100), 211, 2102)
	r.checkBestPathChange(fam, raw, false, nil)
	_, err = r.advertiseSnapshot()
	require.ErrorContains(t, err, "revision exhausted")
	require.Equal(t, uint64(math.MaxUint64), r.advertise.revision)
	require.Equal(t, uint64(2101), last[0].route.SourcePolicyEpoch)
}

// TestAdvertiseCaptureFlowSpecUsesAuthorizedWinner drives the separate
// FlowSpec election and holds its selected bytes across a replacement.
func TestAdvertiseCaptureFlowSpecUsesAuthorizedWinner(t *testing.T) {
	r, peer := advertiseCaptureRIB(t)
	originator := netip.MustParseAddr("1.1.1.1")
	flowValidationReceive(t, r, advertiseCaptureOwner, peer, 65001, 220, family.IPv4Unicast, []byte{8, 10},
		flowValidationAttrs(flowValidationPath(2, 65001), originator, 0, nil), false)
	raw := flowspecNLRI(24, 10, 77, 0)
	flowValidationReceive(t, r, advertiseCaptureOwner, peer, 65001, 221, flowspecFamily, raw,
		flowValidationAttrs(flowValidationPath(2, 65001), originator, 30, nil), false)
	first, err := r.advertiseSnapshot()
	require.NoError(t, err)
	var captured bool
	var revision uint64
	var attrs []byte
	for _, selected := range first {
		if selected.key.family != flowspecFamily {
			continue
		}
		captured = true
		revision = selected.revision
		attrs = bytes.Clone(selected.route.Attributes)
		require.Equal(t, raw, selected.keyNLRI)
		require.Equal(t, raw, selected.route.NLRI)
		require.Equal(t, peer, selected.route.Source)
		require.Equal(t, uint64(221), selected.route.MessageID)
	}
	require.True(t, captured, "FlowSpec's independent winner producer must feed selected capture")
	flowValidationReceive(t, r, advertiseCaptureOwner, peer, 65001, 222, flowspecFamily, raw,
		flowValidationAttrs(flowValidationPath(2, 65001), originator, 20, nil), false)
	current, err := r.advertiseSnapshot()
	require.NoError(t, err)
	captured = false
	for _, selected := range current {
		if selected.key.family != flowspecFamily {
			continue
		}
		captured = true
		require.Greater(t, selected.revision, revision)
		require.Equal(t, uint64(222), selected.route.MessageID)
		require.NotEqual(t, attrs, selected.route.Attributes)
	}
	require.True(t, captured)
	for _, selected := range first {
		if selected.key.family == flowspecFamily {
			require.Equal(t, attrs, selected.route.Attributes, "FlowSpec releases cannot mutate an old capture")
		}
	}
}
