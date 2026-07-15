package reactor

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/filterapi"

	// filter_community registers the code-8 (COMMUNITIES) AttrModHandler in its
	// init(); linked here so the staleModify case's community add actually
	// transforms the body (as it does in the full binary).
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/filter_community"

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
