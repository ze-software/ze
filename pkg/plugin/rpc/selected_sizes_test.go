// Design: docs/architecture/api/process-protocol.md -- exact selected frame bounds.
package rpc

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSelectRoutesItemSizesComposeExactFrames checks escaped request members,
// absence replies and array commas against the complete framing authority.
func TestSelectRoutesItemSizesComposeExactFrames(t *testing.T) {
	input := SelectRoutesInput{Destination: "2001:db8::1", DestinationSession: math.MaxUint64, Routes: []SelectedRoute{}}
	request, response, err := SelectRoutesExchangeSizes(input)
	require.NoError(t, err)
	routes := []SelectedRoute{
		{Kind: SelectedRouteAbsent, Family: "ipv4/unicast", KeyNLRI: []byte{24, 198, 18, 1}, Revision: math.MaxUint64},
		{Kind: SelectedRouteReceived, Family: "l2vpn/evpn", KeyNLRI: []byte{1, 2, 3}, Revision: 17,
			SourceReceipt: 31, Route: &StoredRoute{SourcePeer: "192.0.2.1", AttrHex: "quoted\"value\\tail", InitialUpdate: true}},
		{Kind: SelectedRouteReceived, Family: "ipv6/unicast", KeyNLRI: []byte{32, 0x20, 1, 0xd, 0xb8}, Revision: 99,
			SourceReceipt: 37, Route: &StoredRoute{SourcePeer: "2001:db8::2", NLRIHex: "2020010db8"}},
	}
	for i, route := range routes {
		itemRequest, itemResponse, err := SelectRoutesItemSizes(route)
		require.NoError(t, err)
		request += itemRequest
		response += itemResponse
		if i > 0 {
			request++
			response++
		}
		input.Routes = append(input.Routes, route)
		wholeRequest, wholeResponse, err := SelectRoutesExchangeSizes(input)
		require.NoError(t, err)
		require.Equal(t, request, wholeRequest)
		require.Equal(t, response, wholeResponse)
		whole, err := SelectRoutesExchangeSize(input)
		require.NoError(t, err)
		require.Equal(t, max(request, response), whole)
	}
}
