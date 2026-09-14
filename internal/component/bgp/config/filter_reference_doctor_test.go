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
	bgp.GetOrCreateContainer("filter").SetSlice("import", []string{"nonexistent"})

	diags := filterReferenceDiags(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, codeFilterReferenceUndefined, diags[0].Code)
	assert.Contains(t, diags[0].Message, "nonexistent")
}

// TestFilterReferences_DefinedPolicyPasses drives a reference that resolves.
//
// VALIDATES: a plain name defined under bgp/policy reports nothing.
// PREVENTS: a false finding against a correct configuration.
func TestFilterReferences_DefinedPolicyPasses(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.GetOrCreateContainer("policy").AddListEntry("prefix-list", "customers", config.NewTree())
	bgp.GetOrCreateContainer("filter").SetSlice("import", []string{"customers"})

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
	bgp.GetOrCreateContainer("policy").AddListEntry("prefix-list", "customers", config.NewTree())
	bgp.GetOrCreateContainer("filter").SetSlice("import", []string{"bgp-filter-prefix:customers"})

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
	bgp.GetOrCreateContainer("policy").AddListEntry("prefix-list", "allowed", config.NewTree())

	peerTree := config.NewTree()
	peerTree.GetOrCreateContainer("filter").SetSlice("export", []string{"missing"})
	bgp.AddListEntry("peer", "192.0.2.1", peerTree)

	diags := filterReferenceDiags(tree)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "missing")
	assert.Contains(t, diags[0].Message, "bgp/peer/192.0.2.1/filter/export")
}
