// Design: docs/architecture/plugin/rib-storage-design.md -- pre-policy received routes
// Related: filter_library.go -- buildDeniedAnnouncementPayload
// Related: reactor_notify.go -- partialDenied continuation retention
// RFC naming: untagged -- partial-subset retention probes complementing rfc7947_filtered_conformance_test.go.
package reactor

import (
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

type partialFilteredRow struct {
	Family     string `json:"family"`
	AttrHex    string `json:"attr-hex"`
	NLRIHex    string `json:"nlri-hex"`
	Filtered   bool   `json:"filtered"`
	FilterStep string `json:"filter-step"`
}

func partialRows(t *testing.T, f *filteredReceivedLive, source *lowLivePeer) []partialFilteredRow {
	t.Helper()
	var answer struct {
		Peers map[string][]partialFilteredRow `json:"adj-rib-in"`
	}
	require.NoError(t, json.Unmarshal(f.command(t, "show bgp adj-rib-in"), &answer))
	return answer.Peers[source.peer.Settings().Address.String()]
}

func findPartialRow(rows []partialFilteredRow, prefix netip.Prefix) (partialFilteredRow, bool) {
	want := hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{prefix}))
	for _, row := range rows {
		if row.NLRIHex == want {
			return row, true
		}
	}
	return partialFilteredRow{}, false
}

// TestRFC7947PartialMultiNLRIDenialRetained proves that when a single UPDATE
// from an rs-client carries multiple prefixes with identical attributes (as
// FRR and BIRD pack on the wire) and an import policy drops only a subset, the
// accepted prefix proceeds normally while the denied prefix is retained in
// Adj-RIB-In with its unmodified pre-policy attributes and filtered metadata,
// and withheld from replay. A non-rs-client peer leaves the denied prefix absent.
func TestRFC7947PartialMultiNLRIDenialRetained(t *testing.T) {
	acceptedPrefix := netip.MustParsePrefix("10.99.0.0/24")
	deniedPrefix := netip.MustParsePrefix("10.99.1.0/24")
	deniedHex := hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{deniedPrefix}))
	prevEval := libraryIntentEval
	t.Cleanup(func() { libraryIntentEval = prevEval })

	for _, client := range []bool{true, false} {
		name := "ordinary-peer"
		if client {
			name = "route-server-client"
		}
		t.Run(name, func(t *testing.T) {
			dropFn := dropCIDRs(t, deniedHex)
			libraryIntentEval = func(view *policyview.View, sets *policychain.Sets) error {
				if err := sets.PrependASPath(1); err != nil {
					return err
				}
				return dropFn(view, sets)
			}
			policy := libraryPolicy(t, libraryIntentImport, libraryTypeIntents, routepolicy.FinalAccept)
			f := newFilteredReceivedLive(t, client, policy)
			attrs := firstASAttrs(4, 65002)
			f.send(t, f.source, &message.Update{
				PathAttributes: attrs,
				NLRI:           ownershipLiveNLRI([]netip.Prefix{acceptedPrefix, deniedPrefix}),
			})

			rows := partialRows(t, f, f.source)
			accRow, ok := findPartialRow(rows, acceptedPrefix)
			require.True(t, ok, "accepted prefix must be stored in Adj-RIB-In")
			require.False(t, accRow.Filtered)
			require.Empty(t, accRow.FilterStep)
			require.NotEqual(t, hex.EncodeToString(attrs), accRow.AttrHex,
				"accepted prefix carries the import chain's AS_PATH prepend")

			denRow, hasDenied := findPartialRow(rows, deniedPrefix)
			if !client {
				require.False(t, hasDenied, "ordinary peer must not retain partially denied prefix")
				return
			}
			require.True(t, hasDenied, "rs-client must retain partially denied prefix in Adj-RIB-In")
			require.True(t, denRow.Filtered)
			require.Equal(t, policyChainStepName, denRow.FilterStep)
			require.Equal(t, hex.EncodeToString(attrs), denRow.AttrHex,
				"retained denied prefix must preserve original pre-policy attributes without AS_PATH prepend")

			// Replay must include acceptedPrefix and withhold deniedPrefix.
			f.requireWithheld(t, deniedPrefix)
			var page ribevents.ReplayPage
			require.NoError(t, json.Unmarshal(f.command(t, ribevents.ReplayRoutesCommand, "192.0.2.254", "0"), &page))
			wantAccepted := hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{acceptedPrefix}))
			foundAccepted := false
			for _, rt := range page.Routes {
				if rt.NLRIHex == wantAccepted {
					foundAccepted = true
					require.False(t, rt.Withdraw)
				}
			}
			require.True(t, foundAccepted, "accepted prefix from partial multi-NLRI UPDATE must remain replayable")

			// Explicit withdrawal of deniedPrefix removes only the filtered entry.
			f.send(t, f.source, &message.Update{
				WithdrawnRoutes: ownershipLiveNLRI([]netip.Prefix{deniedPrefix}),
			})
			rowsAfter := partialRows(t, f, f.source)
			_, stillDenied := findPartialRow(rowsAfter, deniedPrefix)
			require.False(t, stillDenied, "explicit withdrawal must remove partially retained filtered entry")
			_, stillAccepted := findPartialRow(rowsAfter, acceptedPrefix)
			require.True(t, stillAccepted, "accepted prefix must survive withdrawal of sibling filtered prefix")
		})
	}
}

// TestRFC7947PartialMultiNLRIDenialSupersedesAccepted proves that when a prefix
// previously accepted from an rs-client is later denied as part of a multi-NLRI
// partial-reject UPDATE, the old accepted entry is superseded by the new
// filtered entry with updated pre-policy attributes and withheld from replay.
func TestRFC7947PartialMultiNLRIDenialSupersedesAccepted(t *testing.T) {
	acceptedPrefix := netip.MustParsePrefix("10.99.0.0/24")
	deniedPrefix := netip.MustParsePrefix("10.99.1.0/24")
	deniedHex := hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{deniedPrefix}))
	prevEval := libraryIntentEval
	t.Cleanup(func() { libraryIntentEval = prevEval })

	dropDenied := false
	dropFn := dropCIDRs(t, deniedHex)
	libraryIntentEval = func(view *policyview.View, sets *policychain.Sets) error {
		if dropDenied {
			return dropFn(view, sets)
		}
		return nil
	}

	policy := libraryPolicy(t, libraryIntentImport, libraryTypeIntents, routepolicy.FinalAccept)
	f := newFilteredReceivedLive(t, true, policy)

	firstAttrs := firstASAttrs(4, 65002)
	f.send(t, f.source, &message.Update{
		PathAttributes: firstAttrs,
		NLRI:           ownershipLiveNLRI([]netip.Prefix{deniedPrefix}),
	})
	rowsBefore := partialRows(t, f, f.source)
	beforeRow, ok := findPartialRow(rowsBefore, deniedPrefix)
	require.True(t, ok)
	require.False(t, beforeRow.Filtered)

	// Now deny deniedPrefix while accepting acceptedPrefix in the same UPDATE.
	dropDenied = true
	secondAttrs := firstASAttrs(4, 65002, 65099)
	f.send(t, f.source, &message.Update{
		PathAttributes: secondAttrs,
		NLRI:           ownershipLiveNLRI([]netip.Prefix{acceptedPrefix, deniedPrefix}),
	})

	rowsAfter := partialRows(t, f, f.source)
	denRow, ok := findPartialRow(rowsAfter, deniedPrefix)
	require.True(t, ok)
	require.True(t, denRow.Filtered)
	require.Equal(t, policyChainStepName, denRow.FilterStep)
	require.Equal(t, hex.EncodeToString(secondAttrs), denRow.AttrHex)
	f.requireWithheld(t, deniedPrefix)
}

// TestRFC7947PartialMPReachDenialRetained exercises buildDeniedAnnouncementPayload
// on the MP_REACH_NLRI (IPv6 unicast) branch when a policy drops one of two
// IPv6 prefixes packed into a single MP_REACH_NLRI attribute.
func TestRFC7947PartialMPReachDenialRetained(t *testing.T) {
	prevEval := libraryIntentEval
	t.Cleanup(func() { libraryIntentEval = prevEval })

	// Two /48 IPv6 prefixes: 2001:db8:1::/48 (kept) and 2001:db8:2::/48 (dropped).
	accNLRI := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}
	denNLRI := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02}
	libraryIntentEval = dropCIDRs(t, hex.EncodeToString(denNLRI))

	// Build a minimal IPv6 MP_REACH_NLRI UPDATE carrying ORIGIN + AS_PATH + MP_REACH_NLRI.
	mpVal := make([]byte, 0, 21+len(accNLRI)+len(denNLRI))
	mpVal = append(mpVal, 0x00, 0x02, 0x01, 16)                                       // AFI=2 (IPv6), SAFI=1 (unicast), NHLen=16
	mpVal = append(mpVal, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1) // NH 2001:db8::1
	mpVal = append(mpVal, 0x00)                                                       // Reserved
	mpVal = append(mpVal, accNLRI...)
	mpVal = append(mpVal, denNLRI...)

	attrs := make([]byte, 0, 3+len(mpVal)+16)
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00)                               // ORIGIN IGP
	attrs = append(attrs, 0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xea) // AS_PATH 65002 (4-byte)
	attrs = append(attrs, 0x80, 0x0e, byte(len(mpVal)))                         // MP_REACH_NLRI
	attrs = append(attrs, mpVal...)

	body := make([]byte, 0, 4+len(attrs))
	body = append(body, 0, 0, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)

	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	addr := netip.MustParseAddr("10.0.0.4")
	peer := NewPeer(&PeerSettings{
		Address:      addr,
		LocalAS:      65000,
		PeerAS:       65002,
		RSClient:     true,
		ImportPolicy: libraryPolicy(t, libraryIntentImport, libraryTypeIntents, routepolicy.FinalAccept),
	})
	src := &filterapi.PeerFilterInfo{Address: addr, PeerAS: 65002, LocalAS: 65000, RSClient: true}
	var meta map[string]any
	res := r.runIngressPolicyChainMeta(peer, addr, src, &meta, testWireUpdate(body), body)
	require.True(t, res.accept)
	require.NotNil(t, res.modifiedPayload)
	require.NotNil(t, res.deniedPayload, "rs-client partial MP_REACH drop must populate deniedPayload")

	modWU := testWireUpdate(res.modifiedPayload)
	modMP, err := modWU.MPReach()
	require.NoError(t, err)
	require.NotNil(t, modMP)
	require.Equal(t, accNLRI, modMP.NLRIBytes(), "accepted payload keeps only 2001:db8:1::/48")

	denWU := testWireUpdate(res.deniedPayload)
	denMP, err := denWU.MPReach()
	require.NoError(t, err)
	require.NotNil(t, denMP)
	require.Equal(t, denNLRI, denMP.NLRIBytes(), "denied payload keeps only 2001:db8:2::/48")
}
