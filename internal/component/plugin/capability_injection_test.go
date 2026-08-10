package plugin

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCapabilityDecoding verifies plugin capability payloads are decoded correctly.
//
// VALIDATES: b64, hex, and text encodings decode properly.
// PREVENTS: Malformed capability bytes in OPEN messages.
func TestCapabilityDecoding(t *testing.T) {
	t.Run("b64_encoding", func(t *testing.T) {
		cap := PluginCapability{
			Code:     73,
			Encoding: rpc.CapEncodingBase64,
			Payload:  base64.StdEncoding.EncodeToString([]byte("router1.example.com")),
		}

		decoded, err := decodeCapabilityPayload(cap)
		require.NoError(t, err)
		assert.Equal(t, []byte("router1.example.com"), decoded)
	})

	t.Run("hex_encoding", func(t *testing.T) {
		cap := PluginCapability{
			Code:     73,
			Encoding: rpc.CapEncodingHex,
			Payload:  "74657374", // "test" in hex
		}

		decoded, err := decodeCapabilityPayload(cap)
		require.NoError(t, err)
		assert.Equal(t, []byte("test"), decoded)
	})

	t.Run("text_encoding", func(t *testing.T) {
		cap := PluginCapability{
			Code:     73,
			Encoding: rpc.CapEncodingText,
			Payload:  "router1.example.com",
		}

		decoded, err := decodeCapabilityPayload(cap)
		require.NoError(t, err)
		assert.Equal(t, []byte("router1.example.com"), decoded)
	})

	t.Run("invalid_b64", func(t *testing.T) {
		cap := PluginCapability{
			Code:     73,
			Encoding: rpc.CapEncodingBase64,
			Payload:  "not-valid-base64!!!",
		}

		_, err := decodeCapabilityPayload(cap)
		require.Error(t, err)
	})

	t.Run("invalid_hex", func(t *testing.T) {
		cap := PluginCapability{
			Code:     73,
			Encoding: rpc.CapEncodingHex,
			Payload:  "not-valid-hex-ZZ",
		}

		_, err := decodeCapabilityPayload(cap)
		require.Error(t, err)
	})

	t.Run("empty_payload_route_refresh", func(t *testing.T) {
		// RFC 2918: Route-refresh capability has 0-length value.
		// VALIDATES: Empty payload produces empty byte slice.
		// PREVENTS: RFC violation by injecting spurious bytes.
		cap := PluginCapability{
			Code:     2,
			Encoding: rpc.CapEncodingHex,
			Payload:  "",
		}

		decoded, err := decodeCapabilityPayload(cap)
		require.NoError(t, err)
		assert.Empty(t, decoded, "route-refresh should have empty payload")
	})

	t.Run("no_encoding_no_payload_flag_capability", func(t *testing.T) {
		// draft-ietf-idr-linklocal-capability: code 77, zero-length payload.
		// VALIDATES: Capability with no encoding and no payload decodes to nil.
		// PREVENTS: "unknown encoding" error for flag-only capabilities (e.g., LLNH code 77).
		cap := PluginCapability{
			Code: 77,
			// Encoding and Payload both empty — flag capability, no data
		}

		decoded, err := decodeCapabilityPayload(cap)
		require.NoError(t, err)
		assert.Nil(t, decoded, "flag capability should have nil payload")
	})
}

// TestCapabilityInjection verifies plugin capabilities are added to OPEN.
//
// VALIDATES: Capability bytes from plugins appear in OPEN message.
// PREVENTS: OPEN messages missing plugin-declared capabilities.
func TestCapabilityInjection(t *testing.T) {
	t.Run("single_capability", func(t *testing.T) {
		caps := &PluginCapabilities{
			PluginName: "hostname-plugin",
			Capabilities: []PluginCapability{
				{Code: 73, Encoding: rpc.CapEncodingBase64, Payload: base64.StdEncoding.EncodeToString([]byte("router1.example.com"))},
			},
			Done: true,
		}

		injector := NewCapabilityInjector()
		require.NoError(t, injector.AddPluginCapabilities(caps))

		// Get capabilities to inject
		toInject := injector.GetCapabilitiesForPeer("")
		require.Len(t, toInject, 1)
		assert.Equal(t, uint8(73), toInject[0].Code)
		assert.Equal(t, []byte("router1.example.com"), toInject[0].Value)
	})

	t.Run("multiple_capabilities_same_plugin", func(t *testing.T) {
		caps := &PluginCapabilities{
			PluginName: "multi-cap-plugin",
			Capabilities: []PluginCapability{
				{Code: 73, Encoding: rpc.CapEncodingBase64, Payload: base64.StdEncoding.EncodeToString([]byte("host1"))},
				{Code: 64, Encoding: rpc.CapEncodingBase64, Payload: base64.StdEncoding.EncodeToString([]byte{0x00, 0x78})},
			},
			Done: true,
		}

		injector := NewCapabilityInjector()
		require.NoError(t, injector.AddPluginCapabilities(caps))

		toInject := injector.GetCapabilitiesForPeer("")
		require.Len(t, toInject, 2)

		// Find by code
		var cap73, cap64 *InjectedCapability
		for i := range toInject {
			if toInject[i].Code == 73 {
				cap73 = &toInject[i]
			}
			if toInject[i].Code == 64 {
				cap64 = &toInject[i]
			}
		}

		require.NotNil(t, cap73)
		require.NotNil(t, cap64)
		assert.Equal(t, []byte("host1"), cap73.Value)
		assert.Equal(t, []byte{0x00, 0x78}, cap64.Value)
	})

	t.Run("multiple_plugins", func(t *testing.T) {
		injector := NewCapabilityInjector()

		caps1 := &PluginCapabilities{
			PluginName: "plugin1",
			Capabilities: []PluginCapability{
				{Code: 73, Encoding: rpc.CapEncodingBase64, Payload: base64.StdEncoding.EncodeToString([]byte("host"))},
			},
			Done: true,
		}
		require.NoError(t, injector.AddPluginCapabilities(caps1))

		caps2 := &PluginCapabilities{
			PluginName: "plugin2",
			Capabilities: []PluginCapability{
				{Code: 64, Encoding: rpc.CapEncodingBase64, Payload: base64.StdEncoding.EncodeToString([]byte{0x01})},
			},
			Done: true,
		}
		require.NoError(t, injector.AddPluginCapabilities(caps2))

		toInject := injector.GetCapabilitiesForPeer("")
		require.Len(t, toInject, 2)
	})
}

// TestCapabilityConflictAtInjection verifies conflicts are caught.
//
// VALIDATES: Same capability code from two plugins is rejected.
// PREVENTS: Conflicting capability values in OPEN.
func TestCapabilityConflictAtInjection(t *testing.T) {
	injector := NewCapabilityInjector()

	caps1 := &PluginCapabilities{
		PluginName: "plugin1",
		Capabilities: []PluginCapability{
			{Code: 73, Encoding: rpc.CapEncodingBase64, Payload: base64.StdEncoding.EncodeToString([]byte("host1"))},
		},
		Done: true,
	}
	require.NoError(t, injector.AddPluginCapabilities(caps1))

	// Second plugin tries same capability code
	caps2 := &PluginCapabilities{
		PluginName: "plugin2",
		Capabilities: []PluginCapability{
			{Code: 73, Encoding: rpc.CapEncodingBase64, Payload: base64.StdEncoding.EncodeToString([]byte("host2"))},
		},
		Done: true,
	}

	err := injector.AddPluginCapabilities(caps2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
	assert.Contains(t, err.Error(), "73")
}

// TestCapabilityInjectorConflictIsAtomic verifies a multi-capability batch does
// not retain earlier capabilities when a later capability conflicts.
//
// VALIDATES: AC-2, capability injection is all-or-nothing for one plugin declaration.
// PREVENTS: Failed plugin startup leaving a partially injected OPEN capability.
func TestCapabilityInjectorConflictIsAtomic(t *testing.T) {
	injector := NewCapabilityInjector()
	require.NoError(t, injector.AddPluginCapabilities(&PluginCapabilities{
		PluginName: "existing",
		Capabilities: []PluginCapability{
			{Code: 73, Encoding: rpc.CapEncodingBase64, Payload: base64.StdEncoding.EncodeToString([]byte("host"))},
		},
		Done: true,
	}))

	err := injector.AddPluginCapabilities(&PluginCapabilities{
		PluginName: "candidate",
		Capabilities: []PluginCapability{
			{Code: 74, Encoding: rpc.CapEncodingHex, Payload: "cafe"},
			{Code: 73, Encoding: rpc.CapEncodingBase64, Payload: base64.StdEncoding.EncodeToString([]byte("conflict"))},
		},
		Done: true,
	})
	require.Error(t, err)

	all := injector.AllCapabilities()
	require.Len(t, all, 1)
	assert.Equal(t, uint8(73), all[0].Code)
	assert.Equal(t, "existing", all[0].Plugin)
	assert.NotContains(t, injector.GetCapabilitiesForPeer("192.0.2.1"), InjectedCapability{Code: 74, Value: []byte{0xca, 0xfe}, Plugin: "candidate"})
}

// TestNoCapabilities verifies plugins with no capabilities work.
//
// VALIDATES: Plugin with empty capabilities works correctly.
// PREVENTS: Crash when plugin declares no capabilities.
func TestNoCapabilities(t *testing.T) {
	caps := &PluginCapabilities{
		PluginName:   "no-caps-plugin",
		Capabilities: nil,
		Done:         true,
	}

	injector := NewCapabilityInjector()
	require.NoError(t, injector.AddPluginCapabilities(caps))

	toInject := injector.GetCapabilitiesForPeer("")
	assert.Empty(t, toInject)
}
