// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RRO collection + ERO/RRO display helpers

package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrependRROAddsSelfAtHead(t *testing.T) {
	// VALIDATES: a node records its own address at the head of the RRO ahead of
	// the downstream route (RFC 3209 Section 4.4).
	self := netip.MustParseAddr("10.0.0.1")
	downstream := []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.2")}}
	got, _ := prependRRO(self, downstream)
	if assert.Len(t, got, 2) {
		assert.Equal(t, self, got[0].Address)
		assert.Equal(t, RROSubIPv4, got[0].Type)
		assert.Equal(t, netip.MustParseAddr("10.0.0.2"), got[1].Address)
	}
}

func TestPrependRROInvalidSelf(t *testing.T) {
	// VALIDATES: an invalid self address is not recorded (no zero-address entry).
	downstream := []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.2")}}
	got, _ := prependRRO(netip.Addr{}, downstream)
	assert.Len(t, got, 1)
}

func TestFormatERO(t *testing.T) {
	hops := []eroHop{
		{Loose: false, Address: netip.MustParsePrefix("10.0.0.2/32")},
		{Loose: true, Address: netip.MustParsePrefix("10.0.0.3/32")},
	}
	got := formatERO(hops)
	assert.Equal(t, []string{"10.0.0.2/32 strict", "10.0.0.3/32 loose"}, got)
	assert.Nil(t, formatERO(nil))
}

func TestFormatRRO(t *testing.T) {
	entries := []rroEntry{
		{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.1")},
		{Type: RROSubLabel, Label: 1001},
	}
	got := formatRRO(entries)
	assert.Equal(t, []string{"10.0.0.1", "label 1001"}, got)
	assert.Nil(t, formatRRO(nil))
}
