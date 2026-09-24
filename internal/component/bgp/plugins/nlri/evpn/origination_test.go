package evpn

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// RFC requirement: RFC7432-8.2.1-10 positive -- the route command originates a per-ES Ethernet A-D route with an IPv4-address-specific RD.
func TestOriginEthernetADPerESUsesTypeOneRD(t *testing.T) {
	frame, encoded, err := EncodeRoute("ethernet-ad rd 192.0.2.1:7 esi 00:01:02:03:04:05:06:07:08:09 etag 4294967295 next-hop 192.0.2.1 extended-community target:65000:100 extended-community 0x0601000000001000", "l2vpn/evpn", 65000, true, true, false)
	require.NoError(t, err)
	update, err := message.UnpackUpdate(frame[message.HeaderLen:])
	require.NoError(t, err)
	_, _, reach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	require.True(t, found)
	require.Equal(t, encoded, reach[5+int(reach[3]):])
	require.Equal(t, []byte{1, 25}, encoded[:2])
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(encoded[2:4]))
	require.Equal(t, uint32(0xffffffff), binary.BigEndian.Uint32(encoded[20:24]))
	require.Equal(t, []byte{0, 0, 0}, encoded[24:27])
	parsed, rest, err := ParseEVPN(encoded, false)
	require.NoError(t, err)
	require.Empty(t, rest)
	require.Equal(t, encoded, parsed.Bytes())
}

// RFC requirement: RFC7432-8.2.1-10 negative -- an AS-specific RD is refused for per-ES origination, without forbidding that RD on a per-EVI Ethernet A-D route.
func TestOriginEthernetADPerESRefusesASTypeRD(t *testing.T) {
	frame, encoded, err := EncodeRoute("ethernet-ad rd 65000:7 etag 4294967295 next-hop 192.0.2.1 extended-community target:65000:100 extended-community 0x0601000000001000", "l2vpn/evpn", 65000, true, true, false)
	require.Error(t, err)
	require.Nil(t, frame)
	require.Nil(t, encoded)

	_, encoded, err = EncodeRoute("ethernet-ad rd 65000:7 etag 100 label 100 next-hop 192.0.2.1", "l2vpn/evpn", 65000, true, true, false)
	require.NoError(t, err)
	require.Equal(t, uint16(0), binary.BigEndian.Uint16(encoded[2:4]))
	require.Equal(t, uint32(100), binary.BigEndian.Uint32(encoded[20:24]))
}

// RFC requirement: RFC7432-11.1-3 positive -- an IMET route command emits its configured Route Target in the UPDATE.
func TestOriginInclusiveMulticastIncludesRouteTarget(t *testing.T) {
	frame, encoded, err := EncodeRoute("multicast rd 65000:7 next-hop 192.0.2.1 extended-community target:65000:100", "l2vpn/evpn", 65000, true, true, false)
	require.NoError(t, err)
	update, err := message.UnpackUpdate(frame[message.HeaderLen:])
	require.NoError(t, err)
	_, _, communities, found := attribute.AttrFind(update.PathAttributes, attribute.AttrExtCommunity)
	require.True(t, found)
	require.Equal(t, []byte{0, 2, 0xfd, 0xe8, 0, 0, 0, 100}, communities)
	require.Equal(t, byte(3), encoded[0])
}

// RFC requirement: RFC7432-11.1-3 negative -- a different extended-community subtype cannot satisfy the mandatory Route Target on an IMET advertisement.
func TestOriginInclusiveMulticastRefusesMissingRouteTarget(t *testing.T) {
	for _, suffix := range []string{"", " extended-community origin:65000:100"} {
		frame, encoded, err := EncodeRoute("multicast rd 65000:7 next-hop 192.0.2.1"+suffix, "l2vpn/evpn", 65000, true, true, false)
		require.Error(t, err)
		require.Nil(t, frame)
		require.Nil(t, encoded)
	}
}

// RFC requirement: RFC7432-8.2.1-9 negative -- per-ES Ethernet A-D cannot originate a nonzero label field or a second label field.
func TestOriginEthernetADPerESRefusesLabelStack(t *testing.T) {
	for _, label := range []string{"label 100", "label2 100"} {
		frame, _, err := EncodeRoute("ethernet-ad rd 192.0.2.1:7 etag 4294967295 next-hop 192.0.2.1 extended-community target:65000:100 extended-community 0x0601000000001000 "+label, "l2vpn/evpn", 65000, true, true, false)
		require.Error(t, err)
		require.Nil(t, frame)
	}
}

func TestOriginEthernetADPerESRequiresBothCommunities(t *testing.T) {
	for _, communities := range []string{
		"extended-community target:65000:100",
		"extended-community 0x0601000000001000",
	} {
		frame, _, err := EncodeRoute("ethernet-ad rd 192.0.2.1:7 etag 4294967295 next-hop 192.0.2.1 "+communities, "l2vpn/evpn", 65000, true, true, false)
		require.Error(t, err)
		require.Nil(t, frame)
	}
}
