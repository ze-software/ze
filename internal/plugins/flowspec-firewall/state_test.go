package flowspecfirewall

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/firewall"
)

func TestRuleMapAddRemove(t *testing.T) {
	rm := newRuleMap(100)
	entry := ruleEntry{terms: []firewall.Term{{Name: "fs-test", Actions: []firewall.Action{firewall.Drop{}}}}}
	require.True(t, rm.add("key1", entry))
	require.True(t, rm.add("key2", entry))
	for _, chain := range rm.buildTable()[0].Chains {
		require.Len(t, chain.Terms, 2)
	}
	rm.remove("key1")
	for _, chain := range rm.buildTable()[0].Chains {
		require.Len(t, chain.Terms, 1)
	}
	rm.remove("key2")
	assert.Nil(t, rm.buildTable())
}

func TestRuleMapMaxRules(t *testing.T) {
	rm := newRuleMap(2)
	entry := ruleEntry{terms: []firewall.Term{{Name: "fs-test"}}}
	require.True(t, rm.add("k1", entry))
	require.True(t, rm.add("k2", entry))
	require.False(t, rm.add("k3", entry))
	require.True(t, rm.add("k1", entry), "replacing the selected action does not consume another slot")
	rm.remove("k2")
	require.True(t, rm.add("k3", entry), "withdrawal frees the selected-rule slot")
}

func TestFlowSpecTableCarriesTheOwnershipPrefix(t *testing.T) {
	rm := newRuleMap(100)
	rm.add("k1", ruleEntry{terms: []firewall.Term{{Name: "fs-fwd1"}}})
	tables := rm.buildTable()
	require.Len(t, tables, 1)
	require.NoError(t, firewall.RegisterTables("flowspec-prefix-test", tables))
	t.Cleanup(func() { _ = firewall.RegisterTables("flowspec-prefix-test", nil) })
	assert.Equal(t, "flowspec", firewall.StripZeTablePrefix(tables[0].Name))
	require.Len(t, tables[0].Chains, 2)
	assert.Equal(t, firewall.FamilyInet, tables[0].Family)
	assert.Equal(t, firewall.HookForward, tables[0].Chains[0].Hook)
	assert.Equal(t, firewall.HookInput, tables[0].Chains[1].Hook)
}
