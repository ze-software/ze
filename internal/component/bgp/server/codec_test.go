package server

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestCodecRPCHandlerRouting verifies CodecRPCHandler returns handlers for known methods.
//
// VALIDATES: Each codec method maps to a non-nil handler.
// PREVENTS: Regression where a method returns nil (would cause "unknown method" error).
func TestCodecRPCHandlerRouting(t *testing.T) {
	methods := []string{
		"ze-plugin-engine:decode-nlri",
		"ze-plugin-engine:encode-nlri",
		"ze-plugin-engine:decode-mp-reach",
		"ze-plugin-engine:decode-mp-unreach",
		"ze-plugin-engine:decode-update",
	}
	for _, m := range methods {
		t.Run(m, func(t *testing.T) {
			h := CodecRPCHandler(m)
			require.NotNil(t, h, "handler for %s must not be nil", m)
		})
	}

	t.Run("unknown_method", func(t *testing.T) {
		h := CodecRPCHandler("ze-plugin-engine:unknown")
		require.Nil(t, h)
	})
}

// TestHandleDecodeNLRI verifies decode-nlri routes through compile-time registry.
//
// VALIDATES: Plugin→engine decode-nlri RPC routes through registry.DecodeNLRIByFamily.
// PREVENTS: Engine rejecting decode-nlri as unknown method.
func TestHandleDecodeNLRI(t *testing.T) {
	// No t.Parallel(): mutates global compile-time registry.
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	registry.Reset()
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-decoder",
		Description: "test",
		Families:    []string{"ipv4/flow"},
		InProcessNLRIDecoder: func(family, hex string) (string, error) {
			return fmt.Sprintf(`[{"family":%q,"hex":%q}]`, family, hex), nil
		},
		RunEngine:  func(_ net.Conn) int { return 0 },
		CLIHandler: func([]string) int { return 0 },
	}))

	params, err := json.Marshal(&rpc.DecodeNLRIInput{
		Family: "ipv4/flow",
		Hex:    "0701180A0000",
	})
	require.NoError(t, err)

	result, err := handleDecodeNLRI(params)
	require.NoError(t, err)

	out, ok := result.(*rpc.DecodeNLRIOutput)
	require.True(t, ok)
	assert.Equal(t, `[{"family":"ipv4/flow","hex":"0701180A0000"}]`, out.JSON)
}

// TestHandleEncodeNLRI verifies encode-nlri routes through compile-time registry.
//
// VALIDATES: Plugin→engine encode-nlri RPC routes through registry.EncodeNLRIByFamily.
// PREVENTS: Engine rejecting encode-nlri as unknown method.
func TestHandleEncodeNLRI(t *testing.T) {
	// No t.Parallel(): mutates global compile-time registry.
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	registry.Reset()
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-encoder",
		Description: "test",
		Families:    []string{"ipv4/flow"},
		InProcessNLRIEncoder: func(family string, args []string) (string, error) {
			return "DEADBEEF", nil
		},
		RunEngine:  func(_ net.Conn) int { return 0 },
		CLIHandler: func([]string) int { return 0 },
	}))

	params, err := json.Marshal(&rpc.EncodeNLRIInput{
		Family: "ipv4/flow",
		Args:   []string{"match", "source", "10.0.0.0/24"},
	})
	require.NoError(t, err)

	result, err := handleEncodeNLRI(params)
	require.NoError(t, err)

	out, ok := result.(*rpc.EncodeNLRIOutput)
	require.True(t, ok)
	assert.Equal(t, "DEADBEEF", out.Hex)
}

// TestHandleDecodeNLRI_NoDecoder verifies error when no decoder registered.
//
// VALIDATES: Engine returns error for unregistered family.
// PREVENTS: Nil pointer or silent failure on unregistered family.
func TestHandleDecodeNLRI_NoDecoder(t *testing.T) {
	// No t.Parallel(): mutates global compile-time registry.
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	registry.Reset()

	params, err := json.Marshal(&rpc.DecodeNLRIInput{
		Family: "ipv4/flow",
		Hex:    "0701180A0000",
	})
	require.NoError(t, err)

	_, err = handleDecodeNLRI(params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no NLRI decoder")
}

// TestHandleDecodeMPReach verifies decode-mp-reach parses MP_REACH_NLRI.
//
// VALIDATES: Handler parses MP_REACH_NLRI and returns structured output.
// PREVENTS: Regression in MP_REACH_NLRI parsing (family, next-hop, NLRI).
func TestHandleDecodeMPReach(t *testing.T) {
	// MP_REACH_NLRI for IPv4 unicast: AFI=1, SAFI=1, NH=192.168.1.1, NLRI=10.0.0.0/24
	// RFC 4760 Section 3: AFI(2) + SAFI(1) + NHLen(1) + NH(4) + Reserved(1) + NLRI
	params, err := json.Marshal(&rpc.DecodeMPReachInput{
		Hex: "00010104C0A8010100180A0000",
	})
	require.NoError(t, err)

	result, err := handleDecodeMPReach(params)
	require.NoError(t, err)

	out, ok := result.(*rpc.DecodeMPReachOutput)
	require.True(t, ok)
	assert.Equal(t, "ipv4/unicast", out.Family)
	assert.Equal(t, "192.168.1.1", out.NextHop)
	assert.Contains(t, string(out.NLRI), "10.0.0.0/24")
}

// TestHandleDecodeMPReach_Malformed verifies error on truncated MP_REACH_NLRI.
//
// VALIDATES: Handler returns error for too-short input.
// PREVENTS: Panic or silent failure on truncated MP_REACH_NLRI data.
func TestHandleDecodeMPReach_Malformed(t *testing.T) {
	// Only 2 bytes — too short for MP_REACH_NLRI (need at least 5)
	params, err := json.Marshal(&rpc.DecodeMPReachInput{Hex: "0001"})
	require.NoError(t, err)

	_, err = handleDecodeMPReach(params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

// TestHandleDecodeMPUnreach verifies decode-mp-unreach parses MP_UNREACH_NLRI.
//
// VALIDATES: Handler parses MP_UNREACH_NLRI and returns structured output.
// PREVENTS: Regression in MP_UNREACH_NLRI parsing (family, withdrawn NLRI).
func TestHandleDecodeMPUnreach(t *testing.T) {
	// MP_UNREACH_NLRI for IPv4 unicast: AFI=1, SAFI=1, Withdrawn=192.168.0.0/24
	// RFC 4760 Section 4: AFI(2) + SAFI(1) + Withdrawn
	params, err := json.Marshal(&rpc.DecodeMPUnreachInput{
		Hex: "00010118C0A800",
	})
	require.NoError(t, err)

	result, err := handleDecodeMPUnreach(params)
	require.NoError(t, err)

	out, ok := result.(*rpc.DecodeMPUnreachOutput)
	require.True(t, ok)
	assert.Equal(t, "ipv4/unicast", out.Family)
	assert.Contains(t, string(out.NLRI), "192.168.0.0/24")
}

// TestHandleDecodeUpdate verifies decode-update parses full UPDATE body.
//
// VALIDATES: Handler parses UPDATE and returns ze-bgp JSON with attributes and NLRI.
// PREVENTS: Regression in UPDATE parsing (origin, next-hop, NLRI prefix).
func TestHandleDecodeUpdate(t *testing.T) {
	// UPDATE body: withdrawn_len=0, attr_len=11, ORIGIN=IGP, NEXT_HOP=192.168.1.1, NLRI=10.0.0.0/24
	// RFC 4271 Section 4.3: Withdrawn(2) + Attrs(2+N) + NLRI
	params, err := json.Marshal(&rpc.DecodeUpdateInput{
		Hex: "0000000B40010100400304C0A80101180A0000",
	})
	require.NoError(t, err)

	result, err := handleDecodeUpdate(params)
	require.NoError(t, err)

	out, ok := result.(*rpc.DecodeUpdateOutput)
	require.True(t, ok)
	assert.Contains(t, out.JSON, "update")
	assert.Contains(t, out.JSON, "10.0.0.0/24")
	assert.Contains(t, out.JSON, "igp")
}
