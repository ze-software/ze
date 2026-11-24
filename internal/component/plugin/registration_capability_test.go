// Design: docs/architecture/api/process-protocol.md — per-participant virtual session capabilities
// Related: registration.go — CapabilityInjector multi-owner tracking and conflict checks

package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	testEndpointOne    = "s1"
	testEndpointTwo    = "s2"
	testEndpointThree  = "s3"
	testTargetSelector = "192.0.2.1"
	testAddPathSendHex = "00010102"
	testAddPathRecvHex = "00010101"
)

// TestVirtualSessionCapabilitiesPerParticipant verifies multi-owner capability
// deduplication and differing-value refusal in CapabilityInjector.
//
// VALIDATES: AC-43, S40, Q-E9(a) — identical (code, value) declarations from
// multiple endpoints share ownership, deduplicate on the wire, survive removal
// of one owner, and vanish when the last owner leaves; differing values for the
// same code within one scope are refused on AddPluginCapabilities,
// CheckReplacePluginCapabilities, and ReplacePluginCapabilities with an
// operator error naming both endpoints, the code, both values, and the fix.
// PREVENTS: Multi-endpoint startup failure when two endpoints declare identical
// virtual session capabilities, premature deletion when one owner exits, or
// silent last-wins overwrites when two endpoints disagree on a capability value.
func TestVirtualSessionCapabilitiesPerParticipant(t *testing.T) {
	t.Parallel()

	t.Run("identical_global_dedup_and_owner_removal", func(t *testing.T) {
		t.Parallel()
		ci := NewCapabilityInjector()

		require.NoError(t, ci.AddPluginCapabilities(&PluginCapabilities{
			PluginName: testEndpointOne,
			Capabilities: []PluginCapability{
				{Code: 2, Encoding: rpc.CapEncodingHex, Payload: ""},
			},
			Done: true,
		}))
		require.NoError(t, ci.AddPluginCapabilities(&PluginCapabilities{
			PluginName: testEndpointTwo,
			Capabilities: []PluginCapability{
				{Code: 2}, // flag-only empty encoding decodes to nil; same 0-byte wire value
			},
			Done: true,
		}))

		got := ci.GetCapabilitiesForSelectors(testTargetSelector)
		require.Len(t, got, 1, "identical global capability from two endpoints must emit once")
		assert.Equal(t, uint8(2), got[0].Code)
		assert.Empty(t, got[0].Value)

		// Removing the first owner keeps the second owner's capability active.
		ci.RemovePluginCapabilities(testEndpointOne)
		got = ci.GetCapabilitiesForSelectors(testTargetSelector)
		require.Len(t, got, 1, "removing one owner must keep remaining owner's capability")
		assert.Equal(t, uint8(2), got[0].Code)
		assert.Equal(t, testEndpointTwo, got[0].Plugin)

		// A third endpoint declaring a differing value still conflicts with s2.
		err := ci.AddPluginCapabilities(&PluginCapabilities{
			PluginName: testEndpointThree,
			Capabilities: []PluginCapability{
				{Code: 2, Encoding: rpc.CapEncodingHex, Payload: "01"},
			},
			Done: true,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), testEndpointTwo)
		assert.Contains(t, err.Error(), testEndpointThree)
		assert.Contains(t, err.Error(), "empty")
		assert.Contains(t, err.Error(), "01")

		// Removing the last owner removes the capability completely.
		ci.RemovePluginCapabilities(testEndpointTwo)
		assert.Empty(t, ci.GetCapabilitiesForSelectors(testTargetSelector))
	})

	t.Run("identical_per_target_dedup_and_owner_removal", func(t *testing.T) {
		t.Parallel()
		ci := NewCapabilityInjector()

		for _, ep := range []string{testEndpointOne, testEndpointTwo} {
			require.NoError(t, ci.AddPluginCapabilities(&PluginCapabilities{
				PluginName: ep,
				Capabilities: []PluginCapability{
					{
						Code:     69,
						Encoding: rpc.CapEncodingHex,
						Payload:  testAddPathSendHex,
						Peers:    []string{testTargetSelector},
					},
				},
				Done: true,
			}))
		}

		got := ci.GetCapabilitiesForSelectors(testTargetSelector)
		require.Len(t, got, 1, "identical per-target capability from two endpoints must emit once")
		assert.Equal(t, uint8(69), got[0].Code)
		assert.Equal(t, []byte{0x00, 0x01, 0x01, 0x02}, got[0].Value)

		ci.RemovePluginCapabilities(testEndpointOne)
		got = ci.GetCapabilitiesForSelectors(testTargetSelector)
		require.Len(t, got, 1, "removing first target owner must keep second owner's capability")
		assert.Equal(t, testEndpointTwo, got[0].Plugin)

		ci.RemovePluginCapabilities(testEndpointTwo)
		assert.Empty(t, ci.GetCapabilitiesForSelectors(testTargetSelector))
	})

	t.Run("differing_values_refused_on_add_and_check_replace", func(t *testing.T) {
		t.Parallel()
		ci := NewCapabilityInjector()

		require.NoError(t, ci.AddPluginCapabilities(&PluginCapabilities{
			PluginName: testEndpointOne,
			Capabilities: []PluginCapability{
				{
					Code:     69,
					Encoding: rpc.CapEncodingHex,
					Payload:  testAddPathSendHex,
					Peers:    []string{testTargetSelector},
				},
			},
			Done: true,
		}))

		conflicting := &PluginCapabilities{
			PluginName: testEndpointTwo,
			Capabilities: []PluginCapability{
				{
					Code:     69,
					Encoding: rpc.CapEncodingHex,
					Payload:  testAddPathRecvHex,
					Peers:    []string{testTargetSelector},
				},
			},
			Done: true,
		}

		assertConflictError := func(t *testing.T, err error) {
			t.Helper()
			require.Error(t, err)
			var opErr *opererr.Error
			require.True(t, errors.As(err, &opErr), "conflict must return an *opererr.Error: %v", err)
			msg := err.Error()
			assert.Contains(t, msg, testEndpointOne)
			assert.Contains(t, msg, testEndpointTwo)
			assert.Contains(t, msg, "69")
			assert.Contains(t, msg, testAddPathSendHex)
			assert.Contains(t, msg, testAddPathRecvHex)
			assert.Contains(t, msg, "fix: align the capability declaration across endpoints s1 and s2, or narrow their bus send > to lists so they do not target the same destination")
		}

		assertConflictError(t, ci.AddPluginCapabilities(conflicting))
		assertConflictError(t, ci.CheckReplacePluginCapabilities(map[string]*PluginCapabilities{
			testEndpointTwo: conflicting,
		}))
		assertConflictError(t, ci.ReplacePluginCapabilities(map[string]*PluginCapabilities{
			testEndpointTwo: conflicting,
		}))

		// Refused calls leave the store untouched.
		got := ci.GetCapabilitiesForSelectors(testTargetSelector)
		require.Len(t, got, 1)
		assert.Equal(t, testEndpointOne, got[0].Plugin)
		assert.Equal(t, []byte{0x00, 0x01, 0x01, 0x02}, got[0].Value)
	})

	t.Run("identical_values_accepted_on_check_replace_and_replace", func(t *testing.T) {
		t.Parallel()
		ci := NewCapabilityInjector()

		same := &PluginCapabilities{
			PluginName: testEndpointOne,
			Capabilities: []PluginCapability{
				{Code: 2, Encoding: rpc.CapEncodingHex, Payload: ""},
			},
			Done: true,
		}
		require.NoError(t, ci.AddPluginCapabilities(same))

		second := &PluginCapabilities{
			PluginName: testEndpointTwo,
			Capabilities: []PluginCapability{
				{Code: 2, Encoding: rpc.CapEncodingHex, Payload: ""},
			},
			Done: true,
		}
		require.NoError(t, ci.CheckReplacePluginCapabilities(map[string]*PluginCapabilities{
			testEndpointTwo: second,
		}))
		require.NoError(t, ci.ReplacePluginCapabilities(map[string]*PluginCapabilities{
			testEndpointTwo: second,
		}))

		got := ci.GetCapabilitiesForSelectors(testTargetSelector)
		require.Len(t, got, 1)
		assert.Equal(t, uint8(2), got[0].Code)
	})
}

// TestRestartedParticipantDeclaresCapabilitiesAgain verifies an endpoint that
// exits and restarts can declare its capabilities again without self-conflict.
//
// VALIDATES: S42 — RemovePluginCapabilities followed by AddPluginCapabilities
// for the same endpoint succeeds cleanly, both when running alone and while a
// second endpoint shares the same capability.
// PREVENTS: Stale capability ownership records blocking a restarted endpoint's
// Stage 3 declaration.
func TestRestartedParticipantDeclaresCapabilitiesAgain(t *testing.T) {
	t.Parallel()

	t.Run("sole_owner_restart", func(t *testing.T) {
		t.Parallel()
		ci := NewCapabilityInjector()

		decl := &PluginCapabilities{
			PluginName: testEndpointOne,
			Capabilities: []PluginCapability{
				{Code: 2, Encoding: rpc.CapEncodingHex, Payload: ""},
			},
			Done: true,
		}
		require.NoError(t, ci.AddPluginCapabilities(decl))

		ci.RemovePluginCapabilities(testEndpointOne)
		assert.Empty(t, ci.GetCapabilitiesForSelectors(testTargetSelector))

		require.NoError(t, ci.AddPluginCapabilities(decl))
		got := ci.GetCapabilitiesForSelectors(testTargetSelector)
		require.Len(t, got, 1)
		assert.Equal(t, uint8(2), got[0].Code)
		assert.Equal(t, testEndpointOne, got[0].Plugin)
	})

	t.Run("shared_owner_restart", func(t *testing.T) {
		t.Parallel()
		ci := NewCapabilityInjector()

		declOne := &PluginCapabilities{
			PluginName: testEndpointOne,
			Capabilities: []PluginCapability{
				{Code: 2, Encoding: rpc.CapEncodingHex, Payload: ""},
			},
			Done: true,
		}
		declTwo := &PluginCapabilities{
			PluginName: testEndpointTwo,
			Capabilities: []PluginCapability{
				{Code: 2, Encoding: rpc.CapEncodingHex, Payload: ""},
			},
			Done: true,
		}
		require.NoError(t, ci.AddPluginCapabilities(declOne))
		require.NoError(t, ci.AddPluginCapabilities(declTwo))

		// s1 exits while s2 remains active; capability stays advertised.
		ci.RemovePluginCapabilities(testEndpointOne)
		got := ci.GetCapabilitiesForSelectors(testTargetSelector)
		require.Len(t, got, 1)
		assert.Equal(t, testEndpointTwo, got[0].Plugin)

		// s1 restarts and re-declares code 2 cleanly; still emitted once.
		require.NoError(t, ci.AddPluginCapabilities(declOne))
		got = ci.GetCapabilitiesForSelectors(testTargetSelector)
		require.Len(t, got, 1)
		assert.Equal(t, uint8(2), got[0].Code)

		// s2 later exits; s1's re-declared capability remains active.
		ci.RemovePluginCapabilities(testEndpointTwo)
		got = ci.GetCapabilitiesForSelectors(testTargetSelector)
		require.Len(t, got, 1)
		assert.Equal(t, testEndpointOne, got[0].Plugin)
	})
}
