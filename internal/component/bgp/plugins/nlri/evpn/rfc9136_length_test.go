package evpn

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRFC9136IPPrefixRouteSentWithLength34Or58 encodes EVPN IP Prefix routes
// through the in-process encoder every Ze announcement of the family uses.
//
// VALIDATES: RFC 9136 Section 3.1 -- "The Length field of the BGP EVPN NLRI for
// an EVPN IP Prefix route MUST be either 34 (if IPv4 addresses are carried) or
// 58 (if IPv6 addresses are carried)." The send half: an IPv4 route leaves with
// Length 0x22 and 34 octets after it, an IPv6 route with 0x3A and 58, with or
// without an operator label.
// PREVENTS: an encoder whose Length follows the label count instead of the
// address family.
//
// RFC requirement: RFC9136-3.1-1 positive -- sent: EncodeNLRIHex type5 with prefix 10.1.2.0/24 (with label 100, and with no label) emits route type 05, Length 22, then 34 octets; with prefix 2001:db8::/32 it emits 05, Length 3A, then 58 octets.
func TestRFC9136IPPrefixRouteSentWithLength34Or58(t *testing.T) {
	for _, tc := range []struct {
		command string
		length  string
		octets  int
	}{
		{"type5 rd 65000:7 prefix 10.1.2.0/24 label 100", "0522", 34},
		{"type5 rd 65000:7 prefix 10.1.2.0/24", "0522", 34},
		{"type5 rd 65000:7 prefix 2001:db8::/32 label 100", "053A", 58},
		{"type5 rd 65000:7 prefix 2001:db8::/32", "053A", 58},
	} {
		t.Run(tc.command, func(t *testing.T) {
			encoded, err := EncodeNLRIHex("l2vpn/evpn", strings.Fields(tc.command))
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(encoded), 4)
			assert.Equal(t, tc.length, encoded[:4], "route type 5 and its Length octet")
			assert.Len(t, encoded, 2*(2+tc.octets), "the Length counts every octet that follows it")
		})
	}
}

// TestRFC9136IPPrefixRouteNeverSentWithAnotherLength asks the encoder for an IP
// Prefix route with two label fields, the one input that would add three octets
// past 34 or 58.
//
// VALIDATES: RFC 9136 Section 3.1 Length rule on the send side: the route is
// refused with "IP Prefix route requires one label field" and nothing is
// encoded.
// PREVENTS: a 37- or 61-octet IP Prefix route reaching a peer.
//
// RFC requirement: RFC9136-3.1-1 negative -- sent: EncodeNLRIHex type5 with two label fields (label 100 label 200), for IPv4 and for IPv6, returns the error "IP Prefix route requires one label field" and no encoding.
func TestRFC9136IPPrefixRouteNeverSentWithAnotherLength(t *testing.T) {
	for _, prefix := range []string{"10.1.2.0/24", "2001:db8::/32"} {
		encoded, err := EncodeNLRIHex("l2vpn/evpn", strings.Fields("type5 rd 65000:7 prefix "+prefix+" label 100 label 200"))
		require.Error(t, err, prefix)
		assert.Contains(t, err.Error(), "IP Prefix route requires one label field")
		assert.Empty(t, encoded)
	}
}

// TestRFC9136IPPrefixRouteReceivedWithAnotherLengthIsRefused decodes IP Prefix
// routes whose Length is one octet either side of 34 and 58.
//
// VALIDATES: RFC 9136 Section 3.1 Length rule on the receive side: each is
// refused with ErrEVPNInvalidAddress, the parser's error for this check.
// PREVENTS: a parser that refuses only short routes, or refuses for another
// reason after slicing past the field.
//
// RFC requirement: RFC9136-3.1-1 negative -- received: an RT-5 NLRI with Length 33, 35, 57 or 59 fails ParseEVPN with ErrEVPNInvalidAddress.
func TestRFC9136IPPrefixRouteReceivedWithAnotherLengthIsRefused(t *testing.T) {
	for _, length := range []byte{33, 35, 57, 59} {
		data := append([]byte{byte(EVPNRouteType5), length}, make([]byte, length)...)
		_, _, err := ParseEVPN(data, false)
		require.ErrorIs(t, err, ErrEVPNInvalidAddress, "Length %d", length)
	}
}
