// Design: filter_reference_doctor.go -- the check these cases drive
//
// The cases moved here with the check, from the doctor component's own test
// file. Each one calls the registration's Check field rather than the function
// directly, so a table that registers the wrong function is red here.

package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// filterReferenceDiags runs the registered check over one tree.
func filterReferenceDiags(tree *config.Tree) []diagnostic.Diagnostic {
	return filterReferenceDoctorCheck.Check(diagnostic.DoctorCheckContext{Tree: tree})
}

// TestFilterReferences_NoBGP asks for a configuration with no bgp block.
//
// VALIDATES: a tree carrying no BGP configuration reports nothing.
// PREVENTS: `ze doctor` reporting BGP findings on a box that runs no BGP.
func TestFilterReferences_NoBGP(t *testing.T) {
	assert.Empty(t, filterReferenceDiags(config.NewTree()))
}

// TestFilterReferences_NoPolicyNoRefs asks for a bgp block with neither
// policies nor chains.
//
// VALIDATES: an empty bgp block reports nothing.
// PREVENTS: an empty defined set being read as "every reference is dangling".
func TestFilterReferences_NoPolicyNoRefs(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("bgp")
	assert.Empty(t, filterReferenceDiags(tree))
}

// TestFilterReferences_DanglingGlobalRef drives the global chain.
//
// VALIDATES: a global import naming an undefined policy is one error naming it.
// PREVENTS: a peer built with a shorter filter chain than the operator wrote.
func TestFilterReferences_DanglingGlobalRef(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.GetOrCreateContainer("filter").GetOrCreateContainer("import").SetSlice("filters", []string{"nonexistent"})
	bgp.GetOrCreateContainer("filter").GetOrCreateContainer("import").Set("final", "accept")

	diags := filterReferenceDiags(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, codeFilterReferenceUndefined, diags[0].Code)
	assert.Contains(t, diags[0].Message, "nonexistent")
}

// TestFilterReferences_DefinedPolicyPasses drives a reference that resolves.
//
// VALIDATES: a plain name defined under the policy root reports nothing.
// PREVENTS: a false finding against a correct configuration.
func TestFilterReferences_DefinedPolicyPasses(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	tree.GetOrCreateContainer("policy").AddListEntry("prefix-list", "customers", config.NewTree())
	bgp.GetOrCreateContainer("filter").GetOrCreateContainer("import").SetSlice("filters", []string{"customers"})
	bgp.GetOrCreateContainer("filter").GetOrCreateContainer("import").Set("final", "accept")

	assert.Empty(t, filterReferenceDiags(tree))
}

// TestFilterReferences_NamespacedRefPasses drives the prefixed reference form.
//
// VALIDATES: `bgp-filter-prefix:customers` resolves to the policy `customers`,
// through the one filterInstanceName this package declares.
// PREVENTS: the prefixed form every plugin writes reporting as undefined.
func TestFilterReferences_NamespacedRefPasses(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	tree.GetOrCreateContainer("policy").AddListEntry("prefix-list", "customers", config.NewTree())
	bgp.GetOrCreateContainer("filter").GetOrCreateContainer("import").SetSlice("filters", []string{"bgp-filter-prefix:customers"})
	bgp.GetOrCreateContainer("filter").GetOrCreateContainer("import").Set("final", "accept")

	assert.Empty(t, filterReferenceDiags(tree))
}

// TestFilterReferences_PeerLevelRef drives a chain on a peer rather than on the
// global block.
//
// VALIDATES: a peer-level export names the peer's own path in the message.
// PREVENTS: a finding the operator cannot locate in the configuration.
func TestFilterReferences_PeerLevelRef(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	tree.GetOrCreateContainer("policy").AddListEntry("prefix-list", "allowed", config.NewTree())

	peerTree := config.NewTree()
	peerTree.GetOrCreateContainer("filter").GetOrCreateContainer("export").SetSlice("filters", []string{"missing"})
	peerTree.GetOrCreateContainer("filter").GetOrCreateContainer("export").Set("final", "accept")
	bgp.AddListEntry("peer", "192.0.2.1", peerTree)

	diags := filterReferenceDiags(tree)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "missing")
	assert.Contains(t, diags[0].Message, "bgp/peer/192.0.2.1/filter/export/filters")
}

// TestFilterReferences_PolicyRouteRuleNamedAsSuch: a chain naming a `policy
// route` entry is reported as a policy routing rule, not a filter, in the
// words the load refusal uses (AC-3).
// Method: define policy route R, reference R from the global import chain, and
// compare the doctor's message with policy.Registry.ValidateName's refusal.
func TestFilterReferences_PolicyRouteRuleNamedAsSuch(t *testing.T) {
	if registry.Lookup("policy-routes") == nil {
		t.Skip("this build carries no policy routing (ze_policyroute)")
	}
	tree := config.NewTree()
	tree.GetOrCreateContainer("policy").AddListEntry("route", "R", config.NewTree())
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.GetOrCreateContainer("filter").GetOrCreateContainer("import").SetSlice("filters", []string{"R"})

	diags := filterReferenceDiags(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, codeFilterReferenceUndefined, diags[0].Code)
	assert.Contains(t, diags[0].Message, `"R" is a policy route entry`)
	assert.Contains(t, diags[0].Message, "not a filter")
	assert.NotContains(t, diags[0].Message, "undefined filter")
}
