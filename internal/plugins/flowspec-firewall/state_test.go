package flowspecfirewall

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
)

func TestRuleMapAddRemove(t *testing.T) {
	rm := newRuleMap(100)

	entry := ruleEntry{terms: []firewall.Term{{Name: "fs-test1"}}, local: false}
	assert.True(t, rm.add("10.0.0.1", "key1", entry))
	assert.True(t, rm.add("10.0.0.1", "key2", entry))
	assert.True(t, rm.add("10.0.0.2", "key1", entry))

	tables := rm.buildTable()
	require.NotNil(t, tables)
	require.Len(t, tables, 1)
	require.Len(t, tables[0].Chains, 1)
	assert.Equal(t, "flowspec-fwd", tables[0].Chains[0].Name)
	assert.Len(t, tables[0].Chains[0].Terms, 3)

	rm.remove("10.0.0.1", "key1")
	tables = rm.buildTable()
	require.NotNil(t, tables)
	assert.Len(t, tables[0].Chains[0].Terms, 2)

	rm.remove("10.0.0.1", "key2")
	rm.remove("10.0.0.2", "key1")
	tables = rm.buildTable()
	assert.Nil(t, tables)
}

func TestRuleMapMaxRules(t *testing.T) {
	rm := newRuleMap(2)

	entry := ruleEntry{terms: []firewall.Term{{Name: "fs-test"}}, local: false}
	assert.True(t, rm.add("peer1", "k1", entry))
	assert.True(t, rm.add("peer1", "k2", entry))
	assert.False(t, rm.add("peer1", "k3", entry))

	// Updating existing key should succeed (no count increase)
	assert.True(t, rm.add("peer1", "k1", entry))
}

func TestRuleMapPeerDown(t *testing.T) {
	rm := newRuleMap(100)

	entry := ruleEntry{terms: []firewall.Term{{Name: "fs-test"}}, local: false}
	rm.add("peer1", "k1", entry)
	rm.add("peer1", "k2", entry)
	rm.add("peer2", "k1", entry)

	n := rm.removePeer("peer1")
	assert.Equal(t, 2, n)

	tables := rm.buildTable()
	require.NotNil(t, tables)
	assert.Len(t, tables[0].Chains[0].Terms, 1)

	n = rm.removePeer("peer1")
	assert.Equal(t, 0, n)
}

func TestBuildTable(t *testing.T) {
	rm := newRuleMap(100)

	fwd := ruleEntry{terms: []firewall.Term{{Name: "fs-fwd1"}}, local: false}
	inp := ruleEntry{terms: []firewall.Term{{Name: "fs-in1"}}, local: true}

	rm.add("peer1", "k1", fwd)
	rm.add("peer1", "k2", inp)

	tables := rm.buildTable()
	require.NotNil(t, tables)
	require.Len(t, tables, 1)
	assert.Equal(t, "flowspec", tables[0].Name)
	assert.Equal(t, firewall.FamilyInet, tables[0].Family)
	require.Len(t, tables[0].Chains, 2)

	var fwdChain, inChain *firewall.Chain
	for i := range tables[0].Chains {
		switch tables[0].Chains[i].Name {
		case "flowspec-fwd":
			fwdChain = &tables[0].Chains[i]
		case "flowspec-in":
			inChain = &tables[0].Chains[i]
		}
	}

	require.NotNil(t, fwdChain)
	assert.Equal(t, firewall.HookForward, fwdChain.Hook)
	assert.Len(t, fwdChain.Terms, 1)

	require.NotNil(t, inChain)
	assert.Equal(t, firewall.HookInput, inChain.Hook)
	assert.Len(t, inChain.Terms, 1)
}
