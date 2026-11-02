package role

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// rfc9234OTCIngress runs one UPDATE announcing 10.0.0.0/24 with an OTC
// attribute of the given value length (value octets 0x00 0x00 0xFD 0xE9 ...,
// truncated or extended) through the bgp-role:otc-ingress filter from a
// Provider, and returns the filter's answer.
func rfc9234OTCIngress(t *testing.T, otcLen int) *sdk.FilterUpdateOutput {
	t.Helper()
	setFilterState(map[string]*peerRoleConfig{"10.0.0.1": {role: roleCustomer}}, nil)
	setFilterRemoteRole("10.0.0.1", "", roleProvider)
	t.Cleanup(func() {
		setFilterState(nil, nil)
		filterMu.Lock()
		filterRemoteRoles = nil
		filterMu.Unlock()
	})

	value := []byte{0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x00}[:otcLen]
	attrs := append([]byte{0x40, 0x01, 0x01, 0x00, 0xC0, 35, byte(otcLen)}, value...)
	nlri := []byte{24, 10, 0, 0}
	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001}
	return otcIngressAnswer(t, src, buildTestPayload(attrs, nlri))
}

// RFC requirement: RFC9234-5-12 positive -- an OTC Attribute whose length is not
// 4 is malformed and the role filter answers the UPDATE with the withdraw
// verdict, the treat-as-withdraw the import step applies: for OTC lengths 0, 5
// and 8 (both sides of 4, beyond the length-3 case), the filter's answer for
// the UPDATE announcing 10.0.0.0/24 is withdraw, carrying no path attribute
// set, rather than a reject or an accept.
//
// The import step's rewrite of a withdraw verdict into the withdrawal of the
// UPDATE's routes is proven by reactor TestWithdrawVerdictReactorRails.
//
// VALIDATES: RFC 9234 Section 5 malformed OTC, every length other than 4.
// PREVENTS: a length check that only catches short attributes.
func TestRFC9234OTCOfAnyLengthButFourIsTreatedAsWithdraw(t *testing.T) {
	for _, otcLen := range []int{0, 5, 8} {
		out := rfc9234OTCIngress(t, otcLen)
		require.Equal(t, sdk.FilterWithdraw, out.Action, "length %d: treat-as-withdraw withdraws the announcement", otcLen)
		assert.Empty(t, out.Sets, "length %d: a withdrawal carries no path attribute", otcLen)
	}
}

// RFC requirement: RFC9234-5-12 negative -- an OTC Attribute of length 4 is not
// malformed: the same UPDATE with a four-octet OTC from a Provider is accepted
// unchanged, and its announcement is not turned into a withdrawal.
//
// VALIDATES: length 4 is the one well-formed OTC length.
// PREVENTS: treat-as-withdraw firing on a valid OTC.
func TestRFC9234OTCOfLengthFourIsNotTreatedAsWithdraw(t *testing.T) {
	out := rfc9234OTCIngress(t, 4)
	require.NotEqual(t, sdk.FilterWithdraw, out.Action)
	_, stamped := answerOTC(t, out)
	assert.False(t, stamped, "a well-formed OTC from a Provider leaves the UPDATE's attributes unchanged")
}
