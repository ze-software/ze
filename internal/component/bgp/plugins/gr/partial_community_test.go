// Design: docs/architecture/meta/README.md -- LLGR route metadata drives egress
// Related: gr_egress.go -- the automatic LLGR partial-deployment producer
// Related: filterapi/editset.go -- registered COMMUNITY plan materialization

package gr

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community" // Register the real COMMUNITY handler.
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// TestLLGRCommunityPartialSurvivesRebuild materializes the actual LLGR-generated
// COMMUNITY operation through the registered handler and shared EditSet writer.
// Complete twins and fresh/LLGR-capable controls preserve their received state.
//
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS."
// Only the COMMUNITY slot is materialized here: LP0 projection, whole-UPDATE
// rebuilding, Session/socket delivery and non-originator new attachment are not proved.
func TestLLGRCommunityPartialSurvivesRebuild(t *testing.T) {
	priorState := egressState.Load()
	priorWarning := egressStateMissingWarned.Load()
	t.Cleanup(func() {
		setEgressState(priorState)
		egressStateMissingWarned.Store(priorWarning)
	})
	// This helper's nil cleanup runs before the exact prior-state restoration.
	railsLoadedState(t)
	internal := filterapi.PeerFilterInfo{
		Address: railsEBGPDest.Address, PeerAS: 65000, LocalAS: 65000, IsIBGP: true,
	}
	source := filterapi.PeerFilterInfo{
		Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001, LocalAS: 65000,
		ASN4: true, LLGRAdvertised: true,
	}
	ordinary := []byte{0, 1, 0, 1} // 1:1 remains useful after NO_EXPORT is added.
	stale := []byte{0, 1, 0, 1, 0xff, 0xff, 0, 6}
	depreferred := []byte{0, 1, 0, 1, 0xff, 0xff, 0, 6, 0xff, 0xff, 0xff, 1}
	for _, flagCase := range []struct {
		name  string
		flags byte
	}{
		{"complete", 0xc0},
		{"partial", 0xe0},
	} {
		for _, tc := range []struct {
			name     string
			peer     filterapi.PeerFilterInfo
			level    uint8
			value    []byte
			want     []byte
			extended bool
		}{
			{"LLGR to non-LLGR internal peer", internal, 2, stale, depreferred, true},
			{"fresh to same internal peer", internal, 0, ordinary, ordinary, false},
			{"LLGR to capable peer", railsLLGRDest, 2, stale, stale, false},
		} {
			t.Run(flagCase.name+"/"+tc.name, func(t *testing.T) {
				community := append([]byte{flagCase.flags, 8, byte(len(tc.value))}, tc.value...)
				payload := partialLLGRPayload(community)
				original := append([]byte(nil), payload...)
				sections, err := wire.ParseUpdateSections(payload)
				require.NoError(t, err)
				require.Equal(t, []byte{24, 203, 0, 113}, sections.NLRI(payload), "the input advertises a useful route")
				var mods filterapi.ModAccumulator
				require.True(t, LLGREgressFilter(source, tc.peer, payload,
					map[string]any{"stale": tc.level}, &mods))
				require.False(t, mods.IsWithdraw(), "these destinations keep the route")
				require.Equal(t, original, payload, "the automatic producer leaves borrowed bytes intact")
				encoded := partialLLGRCommunitySlot(t, community, &mods)
				gotFlags, code, length, header, err := attribute.ParseHeader(encoded)
				require.NoError(t, err)
				require.Equal(t, attribute.AttrCommunity, code)
				require.Equal(t, len(encoded)-header, int(length))
				require.Equal(t, tc.want, encoded[header:], "retained values and automatic NO_EXPORT")
				wantFlags := flagCase.flags
				if tc.extended {
					wantFlags |= 0x10
				}
				require.Equal(t, wantFlags&0x10, byte(gotFlags)&0x10, "retain the chosen header class")
				require.Equal(t, wantFlags, byte(gotFlags), "the surviving COMMUNITY retains its Partial bit")
			})
		}
	}
}

// partialLLGRPayload is a canonical IPv4 route with the actual source COMMUNITY.
// Its NLRI is an input control only; this test does not rebuild the entire body.
func partialLLGRPayload(community []byte) []byte {
	attrs := []byte{
		0x40, 1, 1, 0,
		0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9,
		0x40, 3, 4, 192, 0, 2, 1,
	}
	attrs = append(attrs, community...)
	// The fixed fixture is far below the two-octet attribute-length ceiling.
	payload := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	payload = append(payload, attrs...)
	return append(payload, 24, 203, 0, 113)
}

// partialLLGRCommunitySlot follows touched-code planning: no COMMUNITY operation
// keeps the source, while a touched slot uses the real registry handler/writer.
// LOCAL_PREF operations remain in the shared list but are not projected here.
func partialLLGRCommunitySlot(t *testing.T, source []byte, mods *filterapi.ModAccumulator) []byte {
	t.Helper()
	ops := mods.GroupedOps()
	from, through := -1, 0
	for index, op := range ops {
		if op.Code == byte(attribute.AttrCommunity) {
			if from < 0 {
				from = index
			}
			through = index + 1
		}
	}
	if from < 0 {
		return source
	}
	handler := filterapi.AttrModHandlers()[byte(attribute.AttrCommunity)]
	require.NotNil(t, handler, "the real community plugin registered its handler")
	edit := mods.EditSet()
	edit.Begin()
	plan := edit.Attr(byte(attribute.AttrCommunity), source, 0, len(source), 3, len(source)-3,
		ops[from:through], from)
	handler(plan)
	id := edit.Commit(plan)
	require.False(t, edit.SlotFailed(id))
	require.False(t, edit.SlotDropped(id))
	encoded := make([]byte, edit.SlotSize(id))
	require.Equal(t, len(encoded), edit.SlotWrite(id, source, ops, encoded, 0))
	return encoded
}
