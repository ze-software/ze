package redistribute_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	"github.com/ze-software/ze/internal/component/plugin/registry"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/redistribute_egress"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
)

// TestLocRIBPluginNamesARegisteredPlugin compares the copy against its source.
//
// LocRIBPlugin spells a plugin name the BGP engine must not import, so the name
// exists twice. It is here, and in the registry row the plugin writes at init.
// This check keeps the two from disagreeing (ai/rules/principles.md, "when a
// copy is unavoidable, the copy names its source and a check compares them").
//
// VALIDATES: a config with a BGP-sourced redistribution rule derives a process
// binding naming a plugin that exists.
// PREVENTS: a rename in internal/component/bgp/plugins/rib/register.go that
// leaves every derived binding naming a process nothing starts. That failure is
// silent: the delivery graph resolves nobody and the route is dropped.
func TestLocRIBPluginNamesARegisteredPlugin(t *testing.T) {
	reg := registry.Lookup(bgpredist.LocRIBPlugin)
	require.NotNilf(t, reg, "no plugin is registered as %q", bgpredist.LocRIBPlugin)
	assert.Equal(t, bgpredist.LocRIBPlugin, reg.Name)
}

// TestOrchestratorPluginNamesARegisteredPlugin is the same drift check for the
// other derived binding's process.
//
// VALIDATES: a `destination bgp` rule derives a send permission for a plugin
// that exists.
// PREVENTS: a rename that leaves the permission naming a process nothing runs,
// which refuses every route the consumer tries to put on a peer's wire.
func TestOrchestratorPluginNamesARegisteredPlugin(t *testing.T) {
	reg := registry.Lookup(bgpredist.OrchestratorPlugin)
	require.NotNilf(t, reg, "no plugin is registered as %q", bgpredist.OrchestratorPlugin)
	assert.Equal(t, bgpredist.OrchestratorPlugin, reg.Name)
}

// TestDestinationIsBGPNamesTheConsumerThisPackageRegisters keeps the predicate
// beside the consumer it answers about.
//
// VALIDATES: the name the BGP consumer registers under is the name a
// `destination` key has to carry for the send permission to be derived.
func TestDestinationIsBGPNamesTheConsumerThisPackageRegisters(t *testing.T) {
	assert.True(t, bgpredist.DestinationIsBGP(bgpredist.NewBGPConsumer(nil).Name()))
	assert.False(t, bgpredist.DestinationIsBGP("ospf"))
	assert.False(t, bgpredist.DestinationIsBGP(""), "a rule with no destination is agnostic, not BGP")
}

// TestSourceIsBGPAnswersFromTheRegistry holds the predicate to the registry
// rather than to a list of names.
//
// VALIDATES: the three sources this package registers answer true, a source
// another protocol registers answers false, and an unregistered name answers
// false rather than defaulting either way.
// PREVENTS: a derived Loc-RIB binding on a config whose only rule exports IS-IS
// into BGP, which would make every peer of that daemon pay a per-UPDATE
// delivery it never asked for (R-1).
func TestSourceIsBGPAnswersFromTheRegistry(t *testing.T) {
	for _, name := range []string{"bgp", "ibgp", "ebgp"} {
		assert.Truef(t, bgpredist.SourceIsBGP(name), "%q is a BGP source", name)
	}
	assert.False(t, bgpredist.SourceIsBGP("connected"), "connected routes come from iface, not the Loc-RIB")
	assert.False(t, bgpredist.SourceIsBGP("rip"), "an unregistered name is not a BGP source")
}
