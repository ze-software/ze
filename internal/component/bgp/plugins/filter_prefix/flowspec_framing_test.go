// Design: docs/architecture/core-design.md -- a match's prefix-list judges a FlowSpec rule's destination
// Related: rfc8955_flowspec_test.go -- the destination verdicts without ADD-PATH

package filter_prefix

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// flowFramingChain compiles a one-member chain naming a match whose
// prefix-list accepts 10.0.0.0/8 ge 16 le 24 and every IPv6 prefix, so an
// IPv6 rule is accepted exactly when its destination reads as a prefix.
func flowFramingChain(t *testing.T) *policychain.Chain {
	t.Helper()
	listCfg := map[string]any{"entry": []any{
		map[string]any{"prefix": "10.0.0.0/8", "ge": "16", "le": "24", "action": "accept"},
		map[string]any{"prefix": "::/0", "ge": "0", "le": "128", "action": "accept"},
	}}
	reg, err := policy.NewRegistry(
		policy.Instance{Name: "framing", Type: filterTypeName, Config: listCfg},
		policy.Instance{Name: "m", Type: policy.MatchFilterType, Config: map[string]any{filterTypeName: "framing"}},
	)
	require.NoError(t, err)
	chain, err := policychain.Compile(testAttachmentBGP, reg, []policychain.Member{{Name: "m"}}, policy.FinalAccept, policychain.Bases{})
	require.NoError(t, err)
	return chain
}

// TestPrefixPolicyFlowSpecFraming proves a match's prefix-list reads a
// FlowSpec rule's destination through the framing the session negotiated:
// the ADD-PATH path identifier ahead of each rule, the two-octet rule length,
// the route distinguisher of flow-vpn, the IPv6 offset octet, and a
// destination cut short by its rule length, and a rule whose length runs past
// the end of the attribute.
//
// Method: each case is one UPDATE whose MP_REACH_NLRI carries one rule, behind
// path identifier 0xf0ffffff when the case negotiated ADD-PATH for the rule's
// family and bare otherwise, run through a compiled chain. An accepted rule drops
// nothing from MP_REACH_NLRI; a destination that is refused, or that does not
// read as a prefix (an IPv6 offset other than zero, a truncated pattern),
// rejects the whole UPDATE.
func TestPrefixPolicyFlowSpecFraming(t *testing.T) {
	family.RegisterTestFamilies()
	ipv4Flow := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	vpn4Flow := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN}
	ipv6Flow := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}
	chain := flowFramingChain(t)
	for _, tc := range []struct {
		name    string
		fam     family.Family
		addPath bool
		rule    []byte
		want    policychain.OutcomeAction
	}{
		{"ipv4 10.1.0.0/24", ipv4Flow, true, []byte{5, 1, 24, 10, 1, 0}, policychain.OutcomeAccept},
		{"ipv4 192.0.2.0/24", ipv4Flow, true, []byte{5, 1, 24, 192, 0, 2}, policychain.OutcomeReject},
		// RFC 8955 Section 4.1: a length of 240 or more takes two octets, 0xfnnn.
		{"ipv4 two-octet length", ipv4Flow, true, []byte{0xf0, 5, 1, 24, 10, 1, 0}, policychain.OutcomeAccept},
		{"vpn4 10.1.0.0/24", vpn4Flow, true, []byte{13, 0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 1, 24, 10, 1, 0}, policychain.OutcomeAccept},
		{"ipv6 2001:db8::/32", ipv6Flow, true, []byte{7, 1, 32, 0, 0x20, 1, 0x0d, 0xb8}, policychain.OutcomeAccept},
		// An offset of 16 then four pattern octets: read past the offset as a
		// /32, these would be an IPv6 prefix the list accepts.
		{"ipv6 offset", ipv6Flow, true, []byte{7, 1, 32, 16, 0x20, 1, 0x0d, 0xb8}, policychain.OutcomeReject},
		{"ipv4 truncated destination", ipv4Flow, true, []byte{4, 1, 24, 10, 1}, policychain.OutcomeReject},
		// A rule length of 5 with three octets behind it, and no ADD-PATH: the
		// rule runs past the end of MP_REACH_NLRI, so it never frames and its
		// octets are never read as a destination prefix.
		{"ipv4 rule past the attribute end", ipv4Flow, false, []byte{5, 1, 24, 10}, policychain.OutcomeReject},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{tc.fam: tc.addPath})
			ctxID, err := bgpctx.Registry.Register(ctx)
			require.NoError(t, err)
			var framed []byte
			if tc.addPath {
				framed = binary.BigEndian.AppendUint32(framed, 0xf0ffffff)
			}
			framed = append(framed, tc.rule...)
			attrs := mpReach(uint16(tc.fam.AFI), uint8(tc.fam.SAFI), nil, framed)
			payload := binary.BigEndian.AppendUint16(nil, 0)
			payload = binary.BigEndian.AppendUint16(payload, uint16(len(attrs))) //nolint:gosec // G115: a test attribute of a few octets
			payload = append(payload, attrs...)

			var view policyview.View
			view.Reset(wireu.NewWireUpdate(payload, ctxID), policyview.Session{})
			defer view.Clear()
			sets := new(policychain.RunSets)
			outcome := chain.Run(&view, sets)
			require.False(t, outcome.Failed, "the chain failed the route: %+v", outcome)
			require.Equal(t, tc.want, outcome.Action)
			if tc.want == policychain.OutcomeAccept {
				require.Empty(t, sets.Folded().NLRIDrops(policychain.NLRISectionMPReach),
					"a FlowSpec rule is never rewritten into its destination")
			}
		})
	}
}
