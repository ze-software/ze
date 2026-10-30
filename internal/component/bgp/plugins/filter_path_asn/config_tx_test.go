package filter_path_asn

import (
	"testing"

	"github.com/ze-software/ze/internal/component/policy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// rejectASNSection is a delivery whose bgp section holds peer p, its import
// chain naming the first name, and whose policy section defines one
// reject-asn list per name.
func rejectASNSection(names ...string) []sdk.ConfigSection {
	data := `{"policy":{"reject-asn":{`
	for i, name := range names {
		if i > 0 {
			data += ","
		}
		data += `"` + name + `":{"anywhere":["6939"]}`
	}
	data += `}}}`
	bgpData := `{"bgp":{"peer":{"p":{"connection":{"remote":{"ip":"192.0.2.1"}},` +
		`"filter":{"import":{"filters":["` + names[0] + `"],"final":"accept"}}}}}}`
	return []sdk.ConfigSection{{Root: configRootBGP, Data: bgpData}, {Root: policy.RootName, Data: data}}
}

// installedLists answers the installed list names' count and the import
// attachment count of the list named name, read from one snapshot.
func installedLists(t *testing.T, name string) (lists, importPeers int) {
	t.Helper()
	held := installedRejectASN.Load()
	require.NotNil(t, held, "no config installed")
	return len(held.lists), held.attachments[name].importPeers
}

// VALIDATES: spec-plugin-reload-applies-or-refuses AC-1 to AC-4 for
// bgp-filter-path-asn. A reload's verify installs nothing, apply installs the
// candidate's lists and attachment counts together, rollback restores the
// boot config, and a candidate that does not parse is refused at verify and
// leaves the running one.
// PREVENTS: the silent reload: OnConfigure alone answered every config-apply
// OK and kept the startup lists; and lists and counts stored apart, so a
// reader could pair one delivery's lists with another's counts.
// Not parallel: installedRejectASN is package state.
func TestConfigTxAppliesReloadedRejectASNLists(t *testing.T) {
	held := installedRejectASN.Load()
	t.Cleanup(func() { installedRejectASN.Store(held) })

	tx := newRejectASNConfigTx()
	require.NoError(t, tx.Configure(rejectASNSection("A")))

	require.NoError(t, tx.Verify(rejectASNSection("B", "A")))
	lists, attachedA := installedLists(t, "A")
	assert.Equal(t, 1, lists, "verify installed the candidate")
	assert.Equal(t, 1, attachedA, "verify moved the attachment counts")

	require.NoError(t, tx.Apply())
	lists, attachedA = installedLists(t, "A")
	_, attachedB := installedLists(t, "B")
	assert.Equal(t, 2, lists, "apply did not install the candidate's lists")
	assert.Equal(t, 0, attachedA, "apply kept the old counts beside the new lists")
	assert.Equal(t, 1, attachedB, "apply did not install the candidate's counts")

	require.NoError(t, tx.Rollback())
	lists, attachedA = installedLists(t, "A")
	assert.Equal(t, 1, lists, "rollback did not restore the boot lists")
	assert.Equal(t, 1, attachedA, "rollback did not restore the boot counts")

	require.Error(t, tx.Verify([]sdk.ConfigSection{{Root: configRootBGP, Data: "{not json"}}),
		"a candidate that is not JSON is refused at verify")
	lists, _ = installedLists(t, "A")
	assert.Equal(t, 1, lists, "a refused verify changed the running config")
}
