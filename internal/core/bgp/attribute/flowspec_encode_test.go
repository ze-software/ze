// Design: flowspec_encode.go — the shared FlowSpec traffic-action encoders
// RFC: rfc/short/rfc8955.md — traffic filtering actions (Section 7)

package attribute

import (
	"math"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFlowSpecTrafficRateRejectsUnusableRates verifies the guards every caller of
// the traffic-rate encoder now shares.
//
// VALIDATES: an unspecified unit, a negative rate, a NaN and an infinity are each
// refused; a zero and a finite positive rate encode.
//
// PREVENTS: a caller reaching the wire through a sub-type it never named, which
// the zero value of FlowSpecRateUnit would otherwise pick for it, and the three
// copies of this guard disagreeing again about what a rate may be.
func TestFlowSpecTrafficRateRejectsUnusableRates(t *testing.T) {
	t.Parallel()

	_, err := FlowSpecTrafficRate(0, 0, 1000)
	require.Error(t, err, "the zero unit names no sub-type and must not encode")

	for _, rate := range []float64{-1, -0.5, math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := FlowSpecTrafficRate(FlowSpecRateBytes, 0, rate)
		require.Error(t, err, "rate %v", rate)

		_, err = FlowSpecTrafficRate(FlowSpecRatePackets, 0, rate)
		require.Error(t, err, "rate %v packets", rate)
	}

	// RFC 8955 Section 7.1: the first two octets carry a 2-octet id, and the
	// remaining four the rate in IEEE 754 form.
	ec, err := FlowSpecTrafficRate(FlowSpecRateBytes, 0xfde8, 1000)
	require.NoError(t, err)
	assert.Equal(t, ExtendedCommunity{0x80, 0x06, 0xfd, 0xe8, 0x44, 0x7a, 0x00, 0x00}, ec)

	ec, err = FlowSpecTrafficRate(FlowSpecRatePackets, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, ExtendedCommunity{0x80, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, ec)
}

// TestFlowSpecTrafficMarkingBound verifies the DSCP boundary the encoder holds.
//
// VALIDATES: 63 is the last valid DSCP, 64 the first invalid one, and the
// reserved octets are literal zero.
//
// PREVENTS: a DSCP above the six-bit field being truncated into the RFC 8955
// Section 7.5 reserved bits.
func TestFlowSpecTrafficMarkingBound(t *testing.T) {
	t.Parallel()

	ec, err := FlowSpecTrafficMarking(FlowSpecDSCPMax)
	require.NoError(t, err)
	assert.Equal(t, ExtendedCommunity{0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3f}, ec)

	_, err = FlowSpecTrafficMarking(FlowSpecDSCPMax + 1)
	require.Error(t, err, "the first invalid DSCP must be refused")

	_, err = FlowSpecTrafficMarking(255)
	require.Error(t, err)
}

// TestFlowSpecRedirectToIPRefusesTheWrongFamily verifies each redirect-to-IP
// constructor refuses an address of the family it cannot encode.
//
// VALIDATES: the IPv4 constructor refuses an IPv6 address and the IPv6 one
// refuses an IPv4 address, including its IPv4-in-IPv6 spelling.
//
// PREVENTS: a 4-in-6 address being written as a 16-octet global administrator,
// which would put an IPv4 next hop on the wire in a community no receiver reads
// as one.
func TestFlowSpecRedirectToIPRefusesTheWrongFamily(t *testing.T) {
	t.Parallel()

	_, err := FlowSpecRedirectToIPv4(netip.MustParseAddr("2001:db8::1"))
	require.Error(t, err)

	_, err = FlowSpecRedirectToIPv6(netip.MustParseAddr("1.2.3.4"))
	require.Error(t, err)

	_, err = FlowSpecRedirectToIPv6(netip.MustParseAddr("::ffff:1.2.3.4"))
	require.Error(t, err, "an IPv4-in-IPv6 address is an IPv4 next hop")

	// RFC 5701 Section 2: 0x00 marks a transitive sub-type, then the sub-type,
	// then 16 octets of global administrator and 2 of local administrator.
	ec, err := FlowSpecRedirectToIPv6(netip.MustParseAddr("2001:db8::1"))
	require.NoError(t, err)
	assert.Equal(t, byte(0x00), ec[0])
	assert.Equal(t, byte(0x0c), ec[1])
	assert.Equal(t, netip.MustParseAddr("2001:db8::1").As16(), [16]byte(ec[2:18]))
	assert.Equal(t, []byte{0x00, 0x00}, ec[18:20])
}

// TestFlowSpecTrafficActionWordsAreDerived verifies the diagnostic names every
// word the encoder accepts.
//
// VALIDATES: the accepted-word list has one entry per table row, sorted.
//
// PREVENTS: a word being added to the table while the error keeps telling the
// operator about the old vocabulary.
func TestFlowSpecTrafficActionWordsAreDerived(t *testing.T) {
	t.Parallel()

	got := flowSpecTrafficActionWords()
	require.Len(t, got, len(flowSpecTrafficActionFlags), "every table entry must appear in the diagnostic")
	assert.Equal(t, []string{"none", "sample", "terminal"}, got)
}
