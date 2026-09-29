package reactor

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// openCapabilityTLV is one capability as it appears in a sent OPEN.
type openCapabilityTLV struct {
	code  uint8
	value []byte
}

// sentOpenCapabilities writes the session's OPEN to a recording connection and
// returns every capability TLV of its Capabilities optional parameters, in
// wire order, read from the octets rather than through the capability parser.
func sentOpenCapabilities(t *testing.T, settings *PeerSettings) []openCapabilityTLV {
	t.Helper()
	s := NewSession(settings)
	conn := &recordingConn{}
	s.bufWriter = bufio.NewWriter(conn)
	require.NoError(t, s.sendOpen(conn))
	written := conn.written()
	require.Greater(t, len(written), message.HeaderLen)
	open, err := message.UnpackOpen(written[message.HeaderLen:])
	require.NoError(t, err)
	require.False(t, open.ExtendedParams, "the fixture OPEN fits the RFC 4271 parameter format")

	var tlvs []openCapabilityTLV
	params := open.OptionalParams
	for len(params) >= 2 {
		paramType, paramLen := params[0], int(params[1])
		require.GreaterOrEqual(t, len(params), 2+paramLen)
		if paramType == 2 {
			caps := params[2 : 2+paramLen]
			for len(caps) >= 2 {
				capLen := int(caps[1])
				require.GreaterOrEqual(t, len(caps), 2+capLen)
				tlvs = append(tlvs, openCapabilityTLV{code: caps[0], value: caps[2 : 2+capLen]})
				caps = caps[2+capLen:]
			}
			require.Empty(t, caps)
		}
		params = params[2+paramLen:]
	}
	require.Empty(t, params)
	return tlvs
}

// openCapabilityCount returns how many TLVs of the given code the OPEN carries.
func openCapabilityCount(tlvs []openCapabilityTLV, code uint8) int {
	count := 0
	for _, tlv := range tlvs {
		if tlv.code == code {
			count++
		}
	}
	return count
}

// routeRefreshTree is a one-peer configuration with the route-refresh
// capability in the given mode.
func routeRefreshTree(mode string) map[string]any {
	session := map[string]any{"asn": map[string]any{"remote": "65001"}}
	if mode != "" {
		session["capability"] = map[string]any{"route-refresh": mode}
	}
	return map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "10.0.0.1"},
			"local":  map[string]any{"ip": "auto"},
		},
		"session": session,
	}
}

// Goal: prove the Route Refresh capability leaves ze in the octets RFC 2918
// Section 2 names: code 2 and length 0.
// Method: configure route-refresh, write the OPEN sendOpen produces to a
// recording connection, and walk the Capabilities optional parameter octet by
// octet, so an encoder that wrote another code or a payload is visible.
//
// VALIDATES: RFC 2918 Section 2, capability code 2 and length 0 on the wire.
// PREVENTS: an encoder that writes another code or a payload.
//
// RFC requirement: RFC2918-2-1 positive -- with route-refresh enabled, the OPEN ze writes carries exactly one capability TLV with code 2, and its length octet is 0 (no value octets); with route-refresh not configured, no code 2 TLV is written.
func TestRFC2918RouteRefreshCapabilityOctetsInSentOpen(t *testing.T) {
	settings, err := parsePeerFromTree("peer1", routeRefreshTree("enable"), 65000, 0)
	require.NoError(t, err)
	tlvs := sentOpenCapabilities(t, settings)
	require.Equal(t, 1, openCapabilityCount(tlvs, 2))
	for _, tlv := range tlvs {
		if tlv.code == 2 {
			require.Empty(t, tlv.value, "the Route Refresh capability length is 0")
		}
	}

	settings, err = parsePeerFromTree("peer1", routeRefreshTree(""), 65000, 0)
	require.NoError(t, err)
	require.Zero(t, openCapabilityCount(sentOpenCapabilities(t, settings), 2))
}

// Goal: prove a speaker with BFD strict-mode enabled writes the BFD Strict-Mode
// Capability (code 74) in the OPEN itself, not only in its settings.
// Method: build peers from configuration, write the OPEN sendOpen produces to
// a recording connection, and count code 74 TLVs in the octets.
//
// VALIDATES: draft-ietf-idr-bgp-bfd-strict-mode Section 6, capability 74 in the OPEN.
// PREVENTS: a sendOpen that drops the capability the settings carry.
//
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1 positive -- with bfd enabled and strict true, the OPEN ze writes carries exactly one capability TLV with code 74.
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1 negative -- with strict true under a disabled bfd block, and with a bfd block that does not set strict, the OPEN ze writes carries no code 74 TLV.
func TestBFDStrictCapabilityInSentOpen(t *testing.T) {
	cases := []struct {
		name string
		bfd  map[string]any
		want int
	}{
		{name: "strict", bfd: map[string]any{"strict": "true"}, want: 1},
		{name: "strict-bfd-disabled", bfd: map[string]any{"enabled": "false", "strict": "true"}, want: 0},
		{name: "not-strict", bfd: map[string]any{"mode": "multi-hop", "min-ttl": "250"}, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peers, err := PeersFromTree(bfdStrictTree(tc.bfd))
			require.NoError(t, err)
			require.Len(t, peers, 1)
			require.Equal(t, tc.want, openCapabilityCount(sentOpenCapabilities(t, peers[0]), 74))
		})
	}
}
