// Design: docs/architecture/update-building.md -- one final ownership and duplicate boundary.
// Related: session_ownership.go -- peer-up relay evidence at the destination writer.
package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

func initialRelayFixtureItem(t *testing.T, f *ownershipRailFixture, body []byte) fwdItem {
	t.Helper()
	original := f.publish(t, f.source, f.ctxID, body)
	return fwdItem{receivedPeer: f.source, receivedGeneration: original.receivedGeneration,
		sourcePolicyEpoch: original.SourcePolicyEpoch, sourceReceipt: original.SourceReceipt,
		sourceMessageID: original.sourceMessageID(), authority: adjOutForwarded}
}

// TestInitialRelayDuplicateAdmission uses the real destination writer. The
// initial copy comes first as required by the replay fence. The queued live
// UPDATE also contains a new path and an explicit withdrawal. Only the copy
// of the same already-advertised path should be removed from the live body.
func TestInitialRelayDuplicateAdmission(t *testing.T) {
	f := ownershipRailNew(t, nil)
	fam := family.IPv4Unicast
	common := []byte{24, 192, 0, 2}
	newPath := []byte{24, 198, 51, 100}
	withdrawn := []byte{24, 203, 0, 113}
	adjOutTestForward(t, f.destination, f.source, withdrawn, 51)

	initial := ownershipRailBody(fam, false, common, 1)
	mixed, err := message.UnpackUpdate(initial)
	require.NoError(t, err)
	mixed.NLRI = bytes.Join([][]byte{common, newPath}, nil)
	mixed.WithdrawnRoutes = withdrawn
	live := fwdPackUpdateBody(mixed)
	item := initialRelayFixtureItem(t, f, live)
	replay := item
	replay.initialUpdate = true

	adjOutTestSend(t, f.destination, f.source, &replay, initial)
	adjOutTestSend(t, f.destination, f.source, &item, live)
	written := recoveryWrittenUpdates(t, f.conn.written())
	require.Len(t, written, 3)
	require.Equal(t, common, written[1].NLRI)
	require.Equal(t, newPath, written[2].NLRI)
	require.Equal(t, withdrawn, written[2].WithdrawnRoutes)
	require.Equal(t, uint64(1), f.destination.adjOut.suppressedCount())
}

// TestInitialRelayKeepsDifferentAdvertisements passes both rails through the
// final writer. A different original message remains eligible even at a
// lower unrelated numeric ID, and a changed final attribute is sent even
// when the two outputs derive from the same original UPDATE.
func TestInitialRelayKeepsDifferentAdvertisements(t *testing.T) {
	for _, scenario := range []struct {
		name             string
		earlierMessage   bool
		differentAttrs   bool
		expectWireWrites int
	}{
		{name: "same original and attributes", expectWireWrites: 1},
		{name: "different original ID", earlierMessage: true, expectWireWrites: 2},
		{name: "different attributes", differentAttrs: true, expectWireWrites: 2},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			prefix := []byte{24, 192, 0, 2}
			initial := ownershipRailBody(family.IPv4Unicast, false, prefix, 1)
			item := initialRelayFixtureItem(t, f, initial)
			replay := item
			replay.initialUpdate = true
			adjOutTestSend(t, f.destination, f.source, &replay, initial)

			live := initial
			if scenario.earlierMessage {
				item.sourceMessageID--
			}
			if scenario.differentAttrs {
				live = ownershipRailBody(family.IPv4Unicast, false, prefix, 2)
			}
			adjOutTestSend(t, f.destination, f.source, &item, live)
			require.Len(t, recoveryWrittenUpdates(t, f.conn.written()), scenario.expectWireWrites)
		})
	}
}

// TestInitialRelayMPDuplicateAdmission sends IPv4 unicast in MP_REACH as
// defined by RFC 4760 Section 3. The live batch needs an extended attribute
// length and carries an independent MP_UNREACH and a legacy sibling with
// its own NEXT_HOP. The common path still has the same attributes and next
// hop. The writer keeps the siblings and the withdrawal.
func TestInitialRelayMPDuplicateAdmission(t *testing.T) {
	f := ownershipRailNew(t, nil)
	fam := family.IPv4Unicast
	common := []byte{24, 192, 0, 2}
	legacyPath := []byte{24, 198, 51, 100}
	withdrawn := []byte{24, 203, 0, 113}
	adjOutTestForward(t, f.destination, f.source, withdrawn, 51)

	base, err := message.UnpackUpdate(ownershipRailBody(fam, false, common, 1))
	require.NoError(t, err)
	start, flags, legacyNextHop, found := attribute.AttrFind(base.PathAttributes, attribute.AttrNextHop)
	require.True(t, found)
	require.False(t, flags.IsExtLength())
	end := start + 3 + len(legacyNextHop)
	nextHop := []byte{192, 0, 2, 1}
	initialReach := buildMPReachSource(uint16(fam.AFI), byte(fam.SAFI), nextHop, common)
	initialAttrs := bytes.Join([][]byte{base.PathAttributes[:start], base.PathAttributes[end:], initialReach}, nil)
	initial := fwdPackUpdateBody(&message.Update{PathAttributes: initialAttrs})
	var newPaths []byte
	for suffix := range byte(64) {
		newPaths = append(newPaths, 24, 198, 18, suffix)
	}
	liveReach := buildMPReachSource(uint16(fam.AFI), byte(fam.SAFI), nextHop, bytes.Join([][]byte{common, newPaths}, nil))
	liveUnreach := append([]byte{byte(attribute.FlagOptional), byte(attribute.AttrMPUnreachNLRI), 7, 0, 1, 1}, withdrawn...)
	live := fwdPackUpdateBody(&message.Update{
		PathAttributes: bytes.Join([][]byte{base.PathAttributes, liveUnreach, liveReach}, nil), NLRI: legacyPath,
	})
	item := initialRelayFixtureItem(t, f, live)
	replay := item
	replay.initialUpdate = true

	adjOutTestSend(t, f.destination, f.source, &replay, initial)
	adjOutTestSend(t, f.destination, f.source, &item, live)
	written := recoveryWrittenUpdates(t, f.conn.written())
	require.Len(t, written, 3)
	require.Equal(t, legacyPath, written[2].NLRI)
	_, _, reach, found := attribute.AttrFind(written[2].PathAttributes, attribute.AttrMPReachNLRI)
	require.True(t, found)
	_, reachOffset, err := pathsLimitMP(reach, true)
	require.NoError(t, err)
	require.Equal(t, nextHop, reach[4:reachOffset-1])
	require.Equal(t, newPaths, reach[reachOffset:])
	_, _, unreach, found := attribute.AttrFind(written[2].PathAttributes, attribute.AttrMPUnreachNLRI)
	require.True(t, found)
	_, unreachOffset, err := pathsLimitMP(unreach, false)
	require.NoError(t, err)
	require.Equal(t, withdrawn, unreach[unreachOffset:])
	require.Equal(t, uint64(1), f.destination.adjOut.suppressedCount())
}
