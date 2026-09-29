package role

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// rfc9234OTCIngress runs one UPDATE announcing 10.0.0.0/24 with an OTC
// attribute of the given value length (value octets 0x00 0x00 0xFD 0xE9 ...,
// truncated or extended) through OTCIngressFilter from a Provider.
func rfc9234OTCIngress(t *testing.T, otcLen int) (bool, []byte, []byte) {
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
	accept, modified := OTCIngressFilter(src, buildTestPayload(attrs, nlri), make(map[string]any))
	return accept, modified, nlri
}

// RFC requirement: RFC9234-5-12 positive -- an OTC Attribute whose length is not
// 4 is malformed and the UPDATE is handled as treat-as-withdraw: for OTC
// lengths 0, 5 and 8 (both sides of 4, beyond the length-3 case), the
// announced 10.0.0.0/24 moves into the withdrawn routes, the path attributes
// are cleared, and the message is kept rather than dropped.
//
// VALIDATES: RFC 9234 Section 5 malformed OTC, every length other than 4.
// PREVENTS: a length check that only catches short attributes.
func TestRFC9234OTCOfAnyLengthButFourIsTreatedAsWithdraw(t *testing.T) {
	for _, otcLen := range []int{0, 5, 8} {
		accept, modified, nlri := rfc9234OTCIngress(t, otcLen)
		require.True(t, accept, "length %d: treat-as-withdraw keeps the message", otcLen)
		require.NotNil(t, modified, "length %d: the announcement must be rewritten to a withdrawal", otcLen)
		wdLen := binary.BigEndian.Uint16(modified[0:2])
		require.Equal(t, uint16(len(nlri)), wdLen, "length %d: NLRI must move to withdrawn", otcLen)
		assert.Equal(t, nlri, modified[2:2+wdLen], "length %d", otcLen)
		assert.Equal(t, uint16(0), binary.BigEndian.Uint16(modified[2+wdLen:2+wdLen+2]), "length %d: attributes cleared", otcLen)
	}
}

// RFC requirement: RFC9234-5-12 negative -- an OTC Attribute of length 4 is not
// malformed: the same UPDATE with a four-octet OTC from a Provider is accepted
// unchanged, and its announcement is not turned into a withdrawal.
//
// VALIDATES: length 4 is the one well-formed OTC length.
// PREVENTS: treat-as-withdraw firing on a valid OTC.
func TestRFC9234OTCOfLengthFourIsNotTreatedAsWithdraw(t *testing.T) {
	accept, modified, _ := rfc9234OTCIngress(t, 4)
	require.True(t, accept)
	assert.Nil(t, modified, "a well-formed OTC from a Provider leaves the UPDATE unchanged")
}
