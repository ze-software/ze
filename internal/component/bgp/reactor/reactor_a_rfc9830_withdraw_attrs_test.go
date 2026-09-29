// Design: docs/architecture/wire/messages.md -- UPDATE attribute rules on the withdraw rail
// Related: reactor_api_batch.go -- buildBatchWithdrawUpdate, the rail under test

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/srpolicy"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC9830SRPolicyWithdrawalCarriesMandatoryAttributes drives RFC 9830 Section
// 2.1 on the withdraw rail a plugin or the CLI enters (`update text ... withdraw
// nlri ipv4/sr-policy ...` reaches buildBatchWithdrawUpdate): "A BGP UPDATE message
// that carries the MP_REACH_NLRI or MP_UNREACH_NLRI attribute with the SR Policy
// SAFI MUST also carry the BGP mandatory attributes."
//
// VALIDATES: an SR Policy withdrawal UPDATE carries MP_UNREACH_NLRI for AFI 1 /
// SAFI 73 together with ORIGIN and AS_PATH, the well-known mandatory attributes of
// RFC 4271 Section 5 (NEXT_HOP is not one for a family carried in MP_REACH_NLRI,
// RFC 4760 Section 3), toward an external and an internal peer; toward the
// internal peer LOCAL_PREF is present too (RFC 4271 Section 5.1.5).
// PREVENTS: an SR Policy withdrawal sent as a bare MP_UNREACH_NLRI, which is the
// shape the unicast withdraw branch writes.
//
// Method: each case builds the withdrawal the rail sends. The negative case forces
// the input toward the violation: the batch names no attribute at all (the shape
// the LLGR rail and a bare `withdraw` send), and the output must still carry both.
//
// RFC requirement: RFC9830-2.1-3 positive -- an SR Policy (SAFI 73) withdrawal built on the withdraw rail carries MP_UNREACH_NLRI with ORIGIN and AS_PATH present, toward both an external and an internal peer.
// RFC requirement: RFC9830-2.1-3 negative -- an SR Policy withdrawal whose batch names no attribute, the input that would yield a bare MP_UNREACH_NLRI, still carries ORIGIN and AS_PATH.
func TestRFC9830SRPolicyWithdrawalCarriesMandatoryAttributes(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	policy := srpolicy.New(family.AFIIPv4, 1, 100, netip.MustParseAddr("192.0.2.9"))
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	named := attribute.NewBuilder().SetMED(10)

	for _, tc := range []struct {
		name   string
		attrs  *attribute.Builder // The caller's attribute block; nil names none.
		isIBGP bool
	}{
		{"external peer, operator attributes", named, false},
		{"internal peer, operator attributes", named, true},
		{"external peer, no attributes named", nil, false},
		{"internal peer, no attributes named", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batch := bgptypes.NLRIBatch{Family: fam, NLRIs: []nlri.NLRI{policy}}
			if tc.attrs != nil {
				batch.Attrs = tc.attrs
			}
			update := adapter.buildBatchWithdrawUpdate(make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen),
				batch, announceFacts{isIBGP: tc.isIBGP, asn4: true, prepend: localASOnly(65000)})
			require.NotNil(t, update)
			require.Empty(t, update.NLRI, "an SR Policy withdrawal carries no inline NLRI")

			_, _, unreach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
			require.True(t, found, "the withdrawal must carry MP_UNREACH_NLRI")
			require.GreaterOrEqual(t, len(unreach), 3)
			require.Equal(t, []byte{0, 1, 73}, unreach[:3], "MP_UNREACH_NLRI must name AFI 1 / SAFI 73")

			for _, code := range []attribute.AttributeCode{attribute.AttrOrigin, attribute.AttrASPath} {
				_, _, _, present := attribute.AttrFind(update.PathAttributes, code)
				require.True(t, present, "an SR Policy withdrawal must carry mandatory attribute %d", code)
			}
			_, _, _, localPref := attribute.AttrFind(update.PathAttributes, attribute.AttrLocalPref)
			require.Equal(t, tc.isIBGP, localPref, "LOCAL_PREF is present toward an internal peer only")
		})
	}
}
