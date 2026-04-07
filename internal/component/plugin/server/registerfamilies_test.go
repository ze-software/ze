// Design: docs/architecture/api/process-protocol.md -- runtime family registration

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestRegisterPluginFamiliesAddsToNLRIRegistry verifies the runtime path for
// external (Python) plugins: declared families are registered with the nlri
// registry so Family.String() and LookupFamily() work after.
//
// VALIDATES: AC-12 -- External plugin registers family at runtime, Family.String()
// works after registration.
// PREVENTS: External plugins receiving "afi-N/safi-N" fallback strings because
// the engine never wired their declared families into the nlri registry.
func TestRegisterPluginFamiliesAddsToNLRIRegistry(t *testing.T) {
	// Use a non-standard SAFI value to avoid colliding with init-time registrations.
	const testSAFI = family.SAFI(199)
	const testName = "ipv4/test-runtime-199"

	// Pre-condition: family not yet registered.
	if _, ok := family.LookupFamily(testName); ok {
		t.Skipf("test family %q already registered (test pollution)", testName)
	}

	// Simulate what handleProcessStartupRPC does after declare-registration.
	families := []rpc.FamilyDecl{
		{Name: testName, Mode: "both", AFI: 1, SAFI: uint8(testSAFI)},
	}
	require.NoError(t, registerPluginFamilies(families))

	// Verify the family is now in the nlri registry.
	f, ok := family.LookupFamily(testName)
	require.True(t, ok, "family must be in nlri registry after RegisterPluginFamilies")
	assert.Equal(t, family.AFI(1), f.AFI)
	assert.Equal(t, testSAFI, f.SAFI)

	// Verify Family.String() returns the canonical name (not the fallback).
	assert.Equal(t, testName, f.String())

	// Verify SAFI.String() now returns the registered name.
	assert.Equal(t, "test-runtime-199", testSAFI.String())
}

// TestRegisterPluginFamiliesRejectsInvalidName verifies invalid family names are rejected.
func TestRegisterPluginFamiliesRejectsInvalidName(t *testing.T) {
	cases := []struct {
		name     string
		families []rpc.FamilyDecl
	}{
		{"empty name", []rpc.FamilyDecl{{Name: "", Mode: "both", AFI: 1, SAFI: 1}}},
		{"no slash", []rpc.FamilyDecl{{Name: "ipv4unicast", Mode: "both", AFI: 1, SAFI: 1}}},
		{"empty afi part", []rpc.FamilyDecl{{Name: "/unicast", Mode: "both", AFI: 1, SAFI: 1}}},
		{"empty safi part", []rpc.FamilyDecl{{Name: "ipv4/", Mode: "both", AFI: 1, SAFI: 1}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := registerPluginFamilies(tc.families)
			require.Error(t, err)
		})
	}
}

// TestRegisterPluginFamiliesNoOpOnSameValues verifies re-registration of an existing
// family with the same values is a no-op (no error).
func TestRegisterPluginFamiliesNoOpOnSameValues(t *testing.T) {
	// Register a family first.
	const testSAFI = family.SAFI(200)
	families := []rpc.FamilyDecl{
		{Name: "ipv4/test-runtime-200", Mode: "both", AFI: 1, SAFI: uint8(testSAFI)},
	}
	require.NoError(t, registerPluginFamilies(families))

	// Re-register the same family -- should be no-op.
	require.NoError(t, registerPluginFamilies(families))
}
