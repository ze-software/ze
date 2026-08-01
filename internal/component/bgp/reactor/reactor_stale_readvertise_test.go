package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"

	// filter_community registers the code-8 (COMMUNITIES) AttrModHandler in its
	// init(); linked here so the staleModify case's community add actually
	// transforms the body (as it does in the full binary).
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalAnnounceBody is a valid RFC 4271 §4.3 UPDATE body: withdrawn-len=0,
// attr-len=4, ORIGIN=IGP, then NLRI 10.0.0.0/24. buildModifiedPayload needs a
// parseable body to rebuild.
var minimalAnnounceBody = []byte{
	0x00, 0x00, // withdrawn routes length = 0
	0x00, 0x04, // total path attribute length = 4
	0x40, 0x01, 0x01, 0x00, // ORIGIN (well-known, type 1, len 1, IGP)
	0x18, 0x0a, 0x00, 0x00, // NLRI 10.0.0.0/24
}

// TestDecideStaleReadvertise verifies the readvertise filter -> outcome mapping
// that AnnounceNLRIBatch uses for LLGR stale batches (RFC 9494): SetWithdraw ->
// staleWithdraw, an attribute mod -> staleModify, no mods -> staleKeep, and a
// rejecting filter -> staleSuppress. Uses a test egress filter so the wiring is
// exercised deterministically without the gr plugin's internal state.
func TestDecideStaleReadvertise(t *testing.T) {
	dest := filterapi.PeerFilterInfo{
		Address: mustParseAddr("10.0.0.2"),
		PeerAS:  65001,
		LocalAS: 65000,
	}

	tests := []struct {
		name    string
		filter  filterapi.EgressFilterFunc
		want    staleOutcome
		wantMod bool // expect a non-nil modified body
	}{
		{
			name: "withdraw for non-LLGR eBGP",
			filter: func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
				mods.SetWithdraw()
				return true
			},
			want: staleWithdraw,
		},
		{
			name: "modify (depreference) for non-LLGR iBGP",
			filter: func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
				// NO_EXPORT community add, as the LLGR filter does for iBGP.
				mods.Op(8, filterapi.AttrModAdd, []byte{0xFF, 0xFF, 0xFF, 0x01})
				return true
			},
			want:    staleModify,
			wantMod: true,
		},
		{
			name: "keep unchanged for LLGR-capable peer",
			filter: func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
				return true // no mods
			},
			want: staleKeep,
		},
		{
			name: "suppress when a filter rejects",
			filter: func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
				return false
			},
			want: staleSuppress,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &Reactor{
				readvertiseEgressFilters: []filterapi.EgressFilterFunc{tc.filter},
				attrModHandlers:          attrModHandlersWithDefaults(),
			}
			a := &reactorAPIAdapter{r: r}

			body := append([]byte(nil), minimalAnnounceBody...)
			outcome, modified := a.decideStaleReadvertise(dest, body, 1)

			assert.Equal(t, tc.want, outcome, "outcome")
			if tc.wantMod {
				require.NotNil(t, modified, "expected a modified body")
				assert.NotEqual(t, minimalAnnounceBody, modified, "modified body must differ from original")
			}
		})
	}
}

// VALIDATES: AC-1 — a readvertise modification that cannot be applied suppresses
// the route instead of re-advertising it unmodified.
// PREVENTS: The RFC 9494 stale re-advertise rail silently undoing its own egress
// filter. Before T1-1 this call site read the build's nil as "no change needed"
// and returned staleModify with a nil body, so the caller re-advertised the
// stale route with none of the depreference the filter asked for. This is one of
// the two call sites the spec's A-2 never named.
func TestDecideStaleReadvertiseSuppressesOnModifyFailure(t *testing.T) {
	dest := filterapi.PeerFilterInfo{
		Address: mustParseAddr("10.0.0.2"),
		PeerAS:  65001,
		LocalAS: 65000,
	}

	// A withdrawn rewrite past the 2-octet Withdrawn Routes Length field. It is
	// a modification (so HasModifications is true and the empty-accumulator
	// early return cannot fire) that buildModifiedPayload must refuse.
	oversize := func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
		mods.SetWithdrawnRewrite(make([]byte, 65536))
		return true
	}

	r := &Reactor{
		readvertiseEgressFilters: []filterapi.EgressFilterFunc{oversize},
		attrModHandlers:          attrModHandlersWithDefaults(),
	}
	a := &reactorAPIAdapter{r: r}

	body := append([]byte(nil), minimalAnnounceBody...)
	outcome, modified := a.decideStaleReadvertise(dest, body, 1)

	assert.Equal(t, staleSuppress, outcome,
		"an unapplicable modification must suppress, never re-advertise unmodified")
	assert.Nil(t, modified, "suppression carries no body")
}

// TestDecideStaleReadvertise_NoFilters verifies that with no readvertise filters
// registered the outcome is always keep (the common no-LLGR deployment): the
// stale branch is inert.
func TestDecideStaleReadvertise_NoFilters(t *testing.T) {
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	a := &reactorAPIAdapter{r: r}

	outcome, modified := a.decideStaleReadvertise(filterapi.PeerFilterInfo{PeerAS: 65001}, minimalAnnounceBody, 1)
	assert.Equal(t, staleKeep, outcome)
	assert.Nil(t, modified)
}

// noExportCommunityWire is the NO_EXPORT COMMUNITIES attribute exactly as the
// forward/readvertise rail's buildModifiedPayload emits it: flags 0xD0
// (Optional | Transitive | Extended-Length), type 8, 2-byte length 0x0004,
// value 0xFFFFFF01. (The community AttrModHandler always uses extended length;
// this is the true on-wire form, which the never-run `.wip` fixture mis-guessed
// as the 1-byte-length C00804FFFFFF01.)
var noExportCommunityWire = []byte{0xD0, 0x08, 0x00, 0x04, 0xFF, 0xFF, 0xFF, 0x01}

// localPrefZeroWire is LOCAL_PREF=0 on the wire (flags 0x40, type 5, len 4,
// value 0) -- the `.ci` grep target 40050400000000.
var localPrefZeroWire = []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x00}

// tenSlash24NLRI is the wire NLRI for 10.0.0.0/24 (prefix-len 0x18, then 0a0000).
var tenSlash24NLRI = []byte{0x18, 0x0a, 0x00, 0x00}

// llgrDepreferenceFilter mimics LLGREgressFilter's mods for a non-LLGR iBGP peer
// (RFC 9494 Section 4.5.3): add NO_EXPORT + set LOCAL_PREF=0. A test double keeps
// this reactor-package test independent of the gr plugin's internal state;
// gr_egress_test.go separately proves LLGREgressFilter emits exactly these mods.
func llgrDepreferenceFilter(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
	mods.Op(8, filterapi.AttrModAdd, []byte{0xFF, 0xFF, 0xFF, 0x01})
	mods.Op(5, filterapi.AttrModSet, []byte{0x00, 0x00, 0x00, 0x00})
	return true
}

// staleReadvertiseBatch builds an IPv4 announce batch for 10.0.0.0/24 with a
// minimal attribute set (ORIGIN + AS_PATH), stale level 1.
func staleReadvertiseBatch(t *testing.T) bgptypes.NLRIBatch {
	t.Helper()
	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, tenSlash24NLRI, false)
	require.NoError(t, err)
	ab := attribute.NewBuilder()
	ab.SetOrigin(uint8(attribute.OriginIGP))
	ab.SetASPath([]uint32{65001})
	return bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
		Attrs:   ab,
		Stale:   1,
	}
}

// TestStaleReadvertiseWireOutput drives the actual wire-byte output of a stale
// (LLGR) readvertise for all three per-peer outcomes -- the coverage the
// multi-peer `.ci` would give, obtained deterministically without a live BGP
// session.
//
// VALIDATES: rib-arch-7 -- the readvertise rail emits, per destination peer, the
// unchanged announce (LLGR-capable), NO_EXPORT + LOCAL_PREF=0 (non-LLGR iBGP), or
// a withdrawal (non-LLGR eBGP), byte-for-byte matching the `.ci` grep targets.
// PREVENTS: the egress filter silently not transforming the readvertised route,
// or the depreference/withdraw building the wrong wire bytes.
func TestStaleReadvertiseWireOutput(t *testing.T) {
	batch := staleReadvertiseBatch(t)
	r := &Reactor{
		config:                   &Config{LocalAS: 65000},
		attrModHandlers:          attrModHandlersWithDefaults(),
		readvertiseEgressFilters: []filterapi.EgressFilterFunc{llgrDepreferenceFilter},
	}
	a := &reactorAPIAdapter{r: r}

	// The announce body an LLGR-capable peer receives unchanged (the keep case).
	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	announce := a.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, netip.MustParseAddr("10.0.0.1"), true, false, false, false, 65000)
	require.NotNil(t, announce)
	announceBody := fwdPackUpdateBody(announce)

	t.Run("keep: LLGR-capable peer gets the announce unchanged", func(t *testing.T) {
		assert.False(t, bytes.Contains(announceBody, noExportCommunityWire),
			"LLGR-capable peer must NOT get NO_EXPORT")
		sec, err := wire.ParseUpdateSections(announceBody)
		require.NoError(t, err)
		assert.True(t, bytes.Contains(sec.NLRI(announceBody), tenSlash24NLRI), "announce carries the prefix")
	})

	t.Run("modify: non-LLGR iBGP peer gets NO_EXPORT + LOCAL_PREF=0", func(t *testing.T) {
		dest := filterapi.PeerFilterInfo{Address: mustParseAddr("10.0.0.3"), PeerAS: 65000, LocalAS: 65000}
		outcome, modified := a.decideStaleReadvertise(dest, announceBody, batch.Stale)
		require.Equal(t, staleModify, outcome)
		require.NotNil(t, modified)
		assert.True(t, bytes.Contains(modified, noExportCommunityWire), "modified body carries the NO_EXPORT community")
		assert.True(t, bytes.Contains(modified, localPrefZeroWire), "modified body carries LOCAL_PREF=0 (40050400000000)")
		assert.True(t, bytes.Contains(modified, tenSlash24NLRI), "modified body still carries the prefix")
	})

	t.Run("withdraw: non-LLGR eBGP peer gets a withdrawal", func(t *testing.T) {
		wdAttr := make([]byte, message.MaxMsgLen)
		wdNlri := make([]byte, message.MaxMsgLen)
		wd := a.buildBatchWithdrawUpdate(wdAttr, wdNlri, batch, false)
		require.NotNil(t, wd)
		wdBody := fwdPackUpdateBody(wd)
		sec, err := wire.ParseUpdateSections(wdBody)
		require.NoError(t, err)
		assert.True(t, bytes.Contains(sec.Withdrawn(wdBody), tenSlash24NLRI),
			"withdrawal removes the prefix from the peer")
		assert.Nil(t, sec.NLRI(wdBody), "a withdrawal announces no NLRI")
	})
}
