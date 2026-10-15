package cos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreCos "github.com/ze-software/ze/internal/core/cos"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// cosSection returns one class-of-service delivery holding a profile per
// name, each with egress priority 1 marked as the given PCP.
func cosSection(egressPCP map[string]string) []sdk.ConfigSection {
	data := `{"class-of-service":{"ieee-802.1p":{`
	first := true
	for name, pcp := range egressPCP {
		if !first {
			data += ","
		}
		first = false
		data += `"` + name + `":{"egress":{"priority":{"1":{"pcp":"` + pcp + `"}}}}`
	}
	data += `}}}`
	return []sdk.ConfigSection{{Root: configRootClassOfService, Data: data}}
}

// installedEgress returns the installed egress PCP for priority 1 of profile
// name, and whether the profile is installed.
func installedEgress(name string) (uint32, bool) {
	p, ok := coreCos.Lookup(name)
	if !ok {
		return 0, false
	}
	return p.EgressMap[1], true
}

// TestCoSVerifyLeavesRegistryUntouched proves AC-13 and R-15: neither the
// in-process verifier nor the transaction's verify changes the installed
// profiles, whether the candidate edits a profile, drops one, or deletes the
// root. Method: boot a profile set, run both verifiers, read the registry.
func TestCoSVerifyLeavesRegistryUntouched(t *testing.T) {
	t.Cleanup(func() { coreCos.Replace(nil) })

	tx := newCoSConfigTx()
	require.NoError(t, tx.Configure(cosSection(map[string]string{"gold": "5", "bronze": "1"})))

	candidates := map[string][]sdk.ConfigSection{
		"edit gold":   cosSection(map[string]string{"gold": "7", "bronze": "1"}),
		"drop bronze": cosSection(map[string]string{"gold": "5"}),
		"delete root": {{Root: configRootClassOfService, Data: "{}"}},
	}
	for name, sections := range candidates {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, verifyCoSConfig(sections))
			require.NoError(t, tx.Verify(sections))

			pcp, ok := installedEgress("gold")
			require.True(t, ok, "gold must stay installed after a verify")
			assert.Equal(t, uint32(5), pcp, "gold must keep its installed map after a verify")
			_, ok = installedEgress("bronze")
			assert.True(t, ok, "bronze must stay installed after a verify")
		})
	}
}

// TestCoSApplyReplacesAndRollbackRestores proves AC-13's second half: the
// installed profiles change at apply, as one whole-set replace (a dropped
// profile is gone, an edited one carries its new map), and a rollback puts
// back the set the apply replaced.
func TestCoSApplyReplacesAndRollbackRestores(t *testing.T) {
	t.Cleanup(func() { coreCos.Replace(nil) })

	tx := newCoSConfigTx()
	require.NoError(t, tx.Configure(cosSection(map[string]string{"gold": "5", "bronze": "1"})))

	require.NoError(t, tx.Verify(cosSection(map[string]string{"gold": "7"})))
	require.NoError(t, tx.Apply())

	pcp, ok := installedEgress("gold")
	require.True(t, ok)
	assert.Equal(t, uint32(7), pcp, "apply installs the edited gold")
	_, ok = installedEgress("bronze")
	assert.False(t, ok, "apply drops the profile the candidate omits")

	require.NoError(t, tx.Rollback())

	pcp, ok = installedEgress("gold")
	require.True(t, ok)
	assert.Equal(t, uint32(5), pcp, "rollback restores gold's previous map")
	_, ok = installedEgress("bronze")
	assert.True(t, ok, "rollback restores the dropped profile")
}

// TestCoSVerifyRefusesInvalidProfile proves a candidate with an out-of-range
// value is refused at verify and installs nothing.
func TestCoSVerifyRefusesInvalidProfile(t *testing.T) {
	t.Cleanup(func() { coreCos.Replace(nil) })

	tx := newCoSConfigTx()
	require.NoError(t, tx.Configure(cosSection(map[string]string{"gold": "5"})))

	bad := cosSection(map[string]string{"gold": "9"})
	require.Error(t, verifyCoSConfig(bad))
	require.Error(t, tx.Verify(bad))
	require.ErrorIs(t, tx.Apply(), sdk.ErrConfigApplyUnverified)

	pcp, ok := installedEgress("gold")
	require.True(t, ok)
	assert.Equal(t, uint32(5), pcp)
}
