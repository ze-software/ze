package policy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// testPolicyModules augments the policy root with one ze:filter list and one
// unmarked list, the shape policy routing's route list has beside the filter
// types.
var testPolicyModules = map[string]string{
	"ze-policy-registry-test.yang": `module ze-policy-registry-test {
    namespace "urn:ze:policy-registry-test";
    prefix prt;
    import ze-extensions { prefix ze; }
    import ze-policy-conf { prefix policy; }
    augment "/policy:policy" {
        list prefix-list {
            ze:filter;
            key "name";
            leaf name { type string; }
        }
        list as-path-list {
            ze:filter;
            key "name";
            leaf name { type string; }
        }
        list route {
            key "name";
            ze:help "A named policy route applied to ingress interfaces.";
            leaf name { type string; }
        }
    }
}`,
}

// policySchema loads the real policy root with the test augment.
func policySchema(t *testing.T) *config.ContainerNode {
	t.Helper()
	schema, err := config.YANGSchemaWithPlugins(testPolicyModules)
	require.NoError(t, err)
	root, ok := schema.Get(RootName).(*config.ContainerNode)
	require.True(t, ok, "the policy root must be a container in the loaded schema")
	return root
}

// TestPolicyRegistryDiscoversByMarker: only lists marked ze:filter are filter
// types. Method: load the YANG with a marked and an unmarked list under the
// policy root, add one entry to each, and check which become instances.
func TestPolicyRegistryDiscoversByMarker(t *testing.T) {
	tree := config.NewTree()
	tree.AddListEntry("prefix-list", "CUSTOMERS", config.NewTree())
	tree.AddListEntry("route", "R", config.NewTree())

	reg, err := BuildRegistry(tree, policySchema(t))
	require.NoError(t, err)
	assert.Equal(t, []string{"CUSTOMERS"}, reg.Names())

	inst, ok := reg.Lookup("CUSTOMERS")
	require.True(t, ok)
	assert.Equal(t, "CUSTOMERS", inst.Name)
	assert.Equal(t, "prefix-list", inst.Type)

	_, ok = reg.Lookup("R")
	assert.False(t, ok, "a policy route entry is never a filter instance")

	assert.True(t, reg.IsFilterType("prefix-list"))
	assert.False(t, reg.IsFilterType("route"))
}

// TestPolicyRegistryRefusesNonFilterReference: a chain naming a policy
// routing entry is refused naming what that entry is (AC-3).
func TestPolicyRegistryRefusesNonFilterReference(t *testing.T) {
	tree := config.NewTree()
	tree.AddListEntry("route", "R", config.NewTree())

	reg, err := BuildRegistry(tree, policySchema(t))
	require.NoError(t, err)

	err = reg.ValidateName("R", "peer 192.0.2.1 import filters")
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, `"R"`)
	assert.Contains(t, msg, "policy route")
	assert.Contains(t, msg, "not a filter")

	err = reg.ValidateName("MISSING", "peer 192.0.2.1 import filters")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown filter "MISSING"`)
	assert.Contains(t, err.Error(), "peer 192.0.2.1 import filters")

	assert.NoError(t, reg.ValidateName("rpki:validate", "peer 192.0.2.1 import filters"),
		"an external plugin filter is resolved at runtime, not at parse")
}

// TestPolicyRegistryDuplicateName: one instance name under two filter types
// is refused, naming both types.
func TestPolicyRegistryDuplicateName(t *testing.T) {
	tree := config.NewTree()
	tree.AddListEntry("prefix-list", "X", config.NewTree())
	tree.AddListEntry("as-path-list", "X", config.NewTree())

	_, err := BuildRegistry(tree, policySchema(t))
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, `"X"`)
	assert.True(t, strings.Contains(msg, "prefix-list") && strings.Contains(msg, "as-path-list"), msg)
}

// TestPolicyRegistryEmpty: an absent policy root yields a usable empty registry.
func TestPolicyRegistryEmpty(t *testing.T) {
	reg, err := BuildRegistry(nil, policySchema(t))
	require.NoError(t, err)
	assert.Equal(t, 0, reg.Len())
	assert.Empty(t, reg.Names())
}

// TestPolicyRegistryNilSchema: a missing policy schema is refused with an
// error, never a panic or an empty registry. Method: build from an empty tree
// and a nil schema.
func TestPolicyRegistryNilSchema(t *testing.T) {
	_, err := BuildRegistry(config.NewTree(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no schema")
}

// TestPolicyRegistryNamesSorted: Names answers in sorted order whatever the
// insertion order. Method: insert "B" then "A" and check count and order.
func TestPolicyRegistryNamesSorted(t *testing.T) {
	tree := config.NewTree()
	tree.AddListEntry("prefix-list", "B", config.NewTree())
	tree.AddListEntry("prefix-list", "A", config.NewTree())

	reg, err := BuildRegistry(tree, policySchema(t))
	require.NoError(t, err)
	assert.Equal(t, 2, reg.Len())
	assert.Equal(t, []string{"A", "B"}, reg.Names())
}
