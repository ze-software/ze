// Design: docs/architecture/wire/capabilities.md -- numeric Multiprotocol capability identity
// RFC: rfc/short/rfc5575.md -- Section 4, FlowSpec uses Capability Code 1
// RFC: rfc/short/rfc8955.md -- Section 4, current IPv4 FlowSpec capability requirement
// RFC: rfc/short/rfc7752.md -- Section 3.2, BGP-LS capability advertisement
// RFC: rfc/short/rfc9552.md -- Section 5.2, current BGP-LS capability requirement
// Related: rfc5575_capability_wire_test.go, rfc9552_capability_test.go -- negotiated UPDATE output

package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// RFC 5575 Section 4 and RFC 8955 Section 4 require the Multiprotocol
// Extension Capability Code (Code 1). RFC 7752 Section 3.2 and RFC 9552
// Section 5.2 likewise specify Code 1 for exchanging Link-State NLRI.
// The raw configured OPEN is checked without the production capability decoder
// or its CodeMultiprotocol constant; the peer input is handwritten separately.
// MUTATION: changing CodeMultiprotocol from 1 to 250 must fail the raw code
// assertion, even when the production encoder and decoder agree with each other.
// RFC requirement: RFC5575-4-1 positive -- configured IPv4 flow and flow-vpn OPEN capabilities carry literal code 1, and handwritten code-1 peer capabilities negotiate each application alongside IPv4 unicast.
// RFC requirement: RFC8955-4-1 positive -- configured IPv4 flow and flow-vpn OPEN capabilities carry literal code 1, and handwritten code-1 peer capabilities negotiate each application alongside IPv4 unicast.
// RFC requirement: RFC7752-3.2-1 positive -- the configured BGP-LS OPEN capability carries literal code 1 for AFI 16388 / SAFI 71, and the matching handwritten peer capability negotiates that family alongside IPv4 unicast.
// RFC requirement: RFC9552-5.2-7 positive -- the configured BGP-LS OPEN capability carries literal code 1 for AFI 16388 / SAFI 71, and the matching handwritten peer capability negotiates that family alongside IPv4 unicast.
func TestMultiprotocolApplicationsAdvertiseLiteralCodeOne(t *testing.T) {
	for _, application := range []struct {
		name string
		afi  uint16
		safi byte
	}{
		{name: "ipv4/flow", afi: 1, safi: 133},
		{name: "ipv4/flow-vpn", afi: 1, safi: 134},
		{name: "bgp-ls/bgp-ls", afi: 16388, safi: 71},
	} {
		t.Run(application.name, func(t *testing.T) {
			tree := map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}, "local": map[string]any{"ip": "auto"}},
				"session": map[string]any{
					"asn": map[string]any{"remote": "65001"},
					"family": map[string]any{
						"ipv4/unicast":   map[string]any{"prefix": map[string]any{"maximum": "100000"}},
						application.name: map[string]any{"prefix": map[string]any{"maximum": "100000"}},
					},
				},
			}
			settings, err := parsePeerFromTree("code-one-peer", tree, 65000, 0)
			require.NoError(t, err)
			open, err := NewSession(settings).buildOpen(settings, settings.Capabilities)
			require.NoError(t, err)
			require.False(t, open.ExtendedParams, "the small fixture uses ordinary OPEN parameter framing")
			wantValue := []byte{byte(application.afi >> 8), byte(application.afi), 0, application.safi}
			matches := 0
			parameters := open.OptionalParams
			for len(parameters) > 0 {
				require.GreaterOrEqual(t, len(parameters), 2, "complete optional parameter header")
				parameterType := parameters[0]
				parameterLength := int(parameters[1])
				parameters = parameters[2:]
				require.GreaterOrEqual(t, len(parameters), parameterLength, "complete optional parameter value")
				value := parameters[:parameterLength]
				parameters = parameters[parameterLength:]
				if parameterType != 2 {
					continue
				}
				for len(value) > 0 {
					require.GreaterOrEqual(t, len(value), 2, "complete capability header")
					code := value[0]
					length := int(value[1])
					value = value[2:]
					require.GreaterOrEqual(t, len(value), length, "complete capability value")
					payload := value[:length]
					value = value[length:]
					if !bytes.Equal(payload, wantValue) {
						continue
					}
					require.Equal(t, byte(1), code, "the application is advertised under the RFC's numeric Capability Code")
					require.Equal(t, 4, length)
					matches++
				}
			}
			require.Equal(t, 1, matches, "exactly one Multiprotocol advertisement for the configured application")

			peerWire := []byte{1, 4, 0, 1, 0, 1, 1, 4, byte(application.afi >> 8), byte(application.afi), 0, application.safi}
			peerCapabilities, err := capability.Parse(peerWire)
			require.NoError(t, err)
			negotiated := capability.Negotiate(settings.Capabilities, peerCapabilities, testPeerIdentity)
			applicationFamily := family.Family{AFI: family.AFI(application.afi), SAFI: family.SAFI(application.safi)}
			require.True(t, negotiated.SupportsFamily(applicationFamily), "the independently encoded code-1 capability negotiates the application")
			require.True(t, negotiated.SupportsFamily(family.IPv4Unicast), "the code-1 unicast control remains usable")
		})
	}
}

// A four-octet AFI/reserved/SAFI payload under an unrecognized Capability Code
// is not an advertisement of the Multiprotocol capability the application needs.
// The literal code-1 unicast capability is a permitted survivor: rejecting every
// peer capability must not satisfy this negative test.
// MUTATION: changing CodeMultiprotocol from 1 to 250 must make the handwritten
// code-250 application negotiate and fail the application-family assertion.
// RFC requirement: RFC5575-4-1 negative -- a peer's code-250 payload for IPv4 flow or flow-vpn does not negotiate that application, while its genuine code-1 IPv4 unicast capability remains usable.
// RFC requirement: RFC8955-4-1 negative -- a peer's code-250 payload for IPv4 flow or flow-vpn does not negotiate that application, while its genuine code-1 IPv4 unicast capability remains usable.
// RFC requirement: RFC7752-3.2-1 negative -- a peer's code-250 payload for AFI 16388 / SAFI 71 does not negotiate BGP-LS, while its genuine code-1 IPv4 unicast capability remains usable.
// RFC requirement: RFC9552-5.2-7 negative -- a peer's code-250 payload for AFI 16388 / SAFI 71 does not negotiate BGP-LS, while its genuine code-1 IPv4 unicast capability remains usable.
func TestMultiprotocolApplicationsRejectUnrecognizedCapabilityCode(t *testing.T) {
	for _, application := range []struct {
		name string
		afi  uint16
		safi byte
	}{
		{name: "ipv4/flow", afi: 1, safi: 133},
		{name: "ipv4/flow-vpn", afi: 1, safi: 134},
		{name: "bgp-ls/bgp-ls", afi: 16388, safi: 71},
	} {
		t.Run(application.name, func(t *testing.T) {
			tree := map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}, "local": map[string]any{"ip": "auto"}},
				"session": map[string]any{
					"asn": map[string]any{"remote": "65001"},
					"family": map[string]any{
						"ipv4/unicast":   map[string]any{"prefix": map[string]any{"maximum": "100000"}},
						application.name: map[string]any{"prefix": map[string]any{"maximum": "100000"}},
					},
				},
			}
			settings, err := parsePeerFromTree("unknown-code-peer", tree, 65000, 0)
			require.NoError(t, err)
			peerWire := []byte{1, 4, 0, 1, 0, 1, 250, 4, byte(application.afi >> 8), byte(application.afi), 0, application.safi}
			peerCapabilities, err := capability.Parse(peerWire)
			require.NoError(t, err)
			negotiated := capability.Negotiate(settings.Capabilities, peerCapabilities, testPeerIdentity)
			applicationFamily := family.Family{AFI: family.AFI(application.afi), SAFI: family.SAFI(application.safi)}
			require.False(t, negotiated.SupportsFamily(applicationFamily), "an unknown code with a matching payload is not a Multiprotocol advertisement")
			require.True(t, negotiated.SupportsFamily(family.IPv4Unicast), "the genuine code-1 unicast capability still negotiates")
		})
	}
}
