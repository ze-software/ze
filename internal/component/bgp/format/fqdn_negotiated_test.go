package format

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestNegotiatedToDecodedFQDN(t *testing.T) {
	t.Parallel()
	id := capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002}

	t.Run("both_hostname_and_domain", func(t *testing.T) {
		t.Parallel()
		neg := capability.Negotiate(nil, []capability.Capability{
			&capability.FQDN{Hostname: "edge01", DomainName: "example.net"},
		}, id)
		decoded := NegotiatedToDecoded(neg)
		require.NotNil(t, decoded.FQDN)
		assert.Equal(t, "edge01", decoded.FQDN.Hostname)
		assert.Equal(t, "example.net", decoded.FQDN.DomainName)
	})

	t.Run("hostname_only", func(t *testing.T) {
		t.Parallel()
		neg := capability.Negotiate(nil, []capability.Capability{
			&capability.FQDN{Hostname: "edge01"},
		}, id)
		decoded := NegotiatedToDecoded(neg)
		require.NotNil(t, decoded.FQDN)
		assert.Equal(t, "edge01", decoded.FQDN.Hostname)
		assert.Empty(t, decoded.FQDN.DomainName)
	})

	t.Run("absent", func(t *testing.T) {
		t.Parallel()
		neg := capability.Negotiate(nil, nil, id)
		decoded := NegotiatedToDecoded(neg)
		assert.Nil(t, decoded.FQDN)
	})
}

func TestJSONEncoderNegotiatedFQDN(t *testing.T) {
	t.Parallel()
	enc := NewJSONEncoder("1.0")
	peer := &plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.0.2.1"),
		LocalAddress: netip.MustParseAddr("192.0.2.2"),
		LocalAS:      65001,
		PeerAS:       rpc.RemoteASOf(65002),
	}

	t.Run("with_both", func(t *testing.T) {
		t.Parallel()
		out := enc.Negotiated(peer, DecodedNegotiated{
			MessageSize:  4096,
			HoldTime:     90,
			RouteRefresh: "absent",
			Families:     []string{"ipv4/unicast"},
			FQDN:         &DecodedFQDN{Hostname: "edge01", DomainName: "example.net"},
		})
		var doc map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &doc))
		bgp := doc["bgp"].(map[string]any)
		neg := bgp["negotiated"].(map[string]any)
		fqdn, ok := neg["fqdn"].(map[string]any)
		require.True(t, ok, "negotiated JSON should contain fqdn: %s", out)
		assert.Equal(t, "edge01", fqdn["hostname"])
		assert.Equal(t, "example.net", fqdn["domain"])
	})

	t.Run("hostname_only_omits_domain", func(t *testing.T) {
		t.Parallel()
		out := enc.Negotiated(peer, DecodedNegotiated{
			MessageSize:  4096,
			HoldTime:     90,
			RouteRefresh: "absent",
			Families:     []string{"ipv4/unicast"},
			FQDN:         &DecodedFQDN{Hostname: "edge01"},
		})
		var doc map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &doc))
		neg := doc["bgp"].(map[string]any)["negotiated"].(map[string]any)
		fqdn, ok := neg["fqdn"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "edge01", fqdn["hostname"])
		_, hasDomain := fqdn["domain"]
		assert.False(t, hasDomain, "empty domain should be omitted")
	})

	t.Run("domain_only_omits_hostname", func(t *testing.T) {
		t.Parallel()
		out := enc.Negotiated(peer, DecodedNegotiated{
			MessageSize:  4096,
			HoldTime:     90,
			RouteRefresh: "absent",
			Families:     []string{"ipv4/unicast"},
			FQDN:         &DecodedFQDN{DomainName: "example.net"},
		})
		var doc map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &doc))
		neg := doc["bgp"].(map[string]any)["negotiated"].(map[string]any)
		fqdn, ok := neg["fqdn"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "example.net", fqdn["domain"])
		_, hasHost := fqdn["hostname"]
		assert.False(t, hasHost, "empty hostname should be omitted")
	})

	t.Run("empty_fqdn_struct_omits_fqdn", func(t *testing.T) {
		t.Parallel()
		out := enc.Negotiated(peer, DecodedNegotiated{
			MessageSize:  4096,
			HoldTime:     90,
			RouteRefresh: "absent",
			Families:     []string{"ipv4/unicast"},
			FQDN:         &DecodedFQDN{},
		})
		var doc map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &doc))
		neg := doc["bgp"].(map[string]any)["negotiated"].(map[string]any)
		_, hasFQDN := neg["fqdn"]
		assert.False(t, hasFQDN, "empty FQDN struct must not emit an empty object: %s", out)
	})

	t.Run("absent_omits_fqdn", func(t *testing.T) {
		t.Parallel()
		out := enc.Negotiated(peer, DecodedNegotiated{
			MessageSize:  4096,
			HoldTime:     90,
			RouteRefresh: "absent",
			Families:     []string{"ipv4/unicast"},
		})
		var doc map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &doc))
		neg := doc["bgp"].(map[string]any)["negotiated"].(map[string]any)
		_, hasFQDN := neg["fqdn"]
		assert.False(t, hasFQDN, "absent fqdn must not emit an empty key: %s", out)
	})
}
