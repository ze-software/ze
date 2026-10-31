package config

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/config/redistribute"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Names no other test registers, so the root each declares is the one under test.
const (
	rootNeedSource      = "rootneed-src"
	rootNeedDestination = "rootneed-dest"
	rootNeedPlainSource = "rootneed-plain"
	rootNeedRoot        = "rootneedproto"
)

var registerRootNeedOnce sync.Once

// registerRootNeed registers a source and a destination that need the
// rootNeedRoot root, and a source that needs no root, once per test binary.
// The destination is a bare declaration with no consumer and no factory, the
// shape IS-IS and OSPF register from init(): their consumers register only
// when their plugin starts, after the config doors have run.
func registerRootNeed(t *testing.T) {
	t.Helper()
	registerRootNeedOnce.Do(func() {
		require.NoError(t, redistribute.RegisterSource(redistribute.RouteSource{
			Name: rootNeedSource, Protocol: rootNeedRoot, ConfigRoot: rootNeedRoot,
		}))
		require.NoError(t, redistribute.RegisterSource(redistribute.RouteSource{
			Name: rootNeedPlainSource, Protocol: rootNeedPlainSource,
		}))
		require.NoError(t, redistribute.RegisterDestination(redistribute.Destination{
			Name:       rootNeedDestination,
			ConfigRoot: rootNeedRoot,
		}))
	})
}

// rootNeedTree builds `redistribute { destination <dest> { import <source> } }`,
// with a `rootneedproto { }` block when withRoot.
func rootNeedTree(dest, source string, withRoot bool) *Tree {
	tree := NewTree()
	redist := NewTree()
	tree.SetContainer("redistribute", redist)
	entry := NewTree()
	entry.AddListEntry("import", source, NewTree())
	redist.AddListEntry("destination", dest, entry)
	if withRoot {
		tree.SetContainer(rootNeedRoot, NewTree())
	}
	return tree
}

// TestRefuseRedistributeRulesNeedsTheSourceRoot proves a rule importing a
// source whose protocol has no config root in this config is refused, and the
// refusal names the statement and the fix.
//
// VALIDATES: `import bgp` in a config with no `bgp` block is refused at load
// and commit rather than moving nothing in silence. BGP declares the root on
// its sources (internal/component/bgp/redistribute/bgp.go); this test declares
// its own so the config package names no protocol.
// PREVENTS: a redistribute rule naming BGP that boots and does nothing.
func TestRefuseRedistributeRulesNeedsTheSourceRoot(t *testing.T) {
	registerRootNeed(t)

	err := RefuseRedistributeRules(rootNeedTree("isis", rootNeedSource, false))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"import rootneed-src" under "destination isis" needs rootneedproto configured, and this config has no rootneedproto { } block`)
	assert.Contains(t, err.Error(), "fix: configure rootneedproto in a rootneedproto { } block")

	assert.NoError(t, RefuseRedistributeRules(rootNeedTree("isis", rootNeedSource, true)),
		"the same rule with its root present is accepted")
}

// TestRefuseRedistributeRulesNeedsTheDestinationRoot proves the same for a
// destination declared with RegisterDestination, with no consumer registered.
//
// VALIDATES: `destination bgp`, `destination isis` and `destination ospf` in a
// config with no block of their own are refused before startup. Before, `bgp`
// pulled in the BGP plugin, which stopped the daemon with "missing required
// bgp { } block", and `isis` and `ospf` booted and moved nothing, because the
// check read only consumer factories and those two protocols have none.
// PREVENTS: a destination nothing would receive the routes for.
func TestRefuseRedistributeRulesNeedsTheDestinationRoot(t *testing.T) {
	registerRootNeed(t)

	_, hasConsumer := redistribute.LookupConsumer(rootNeedDestination)
	require.False(t, hasConsumer, "the destination is judged from its declaration, with no consumer")
	_, hasFactory := redistribute.LookupConsumerFactory(rootNeedDestination)
	require.False(t, hasFactory, "the destination is judged from its declaration, with no factory")

	err := RefuseRedistributeRules(rootNeedTree(rootNeedDestination, rootNeedPlainSource, false))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"destination rootneed-dest" needs rootneedproto configured, and this config has no rootneedproto { } block`)
	assert.Contains(t, err.Error(), `fix: configure rootneedproto in a rootneedproto { } block, or remove "destination rootneed-dest" from the redistribute block`)

	assert.NoError(t, RefuseRedistributeRules(rootNeedTree(rootNeedDestination, rootNeedPlainSource, true)))
}

// TestRefuseRedistributeRulesAcceptsRulesWithNoRoot proves the check touches
// only what declares a root: a source and a destination that declare none
// pass with no other root in the config.
//
// VALIDATES: `redistribute { destination isis { import static } }` with no
// `bgp` block is a valid config.
func TestRefuseRedistributeRulesAcceptsRulesWithNoRoot(t *testing.T) {
	registerRootNeed(t)

	assert.NoError(t, RefuseRedistributeRules(rootNeedTree("isis", rootNeedPlainSource, false)))
}

// TestVerifyDoorRefusesRedistributeWithoutRoot proves the validate and commit
// door, which holds the lowered map rather than the tree, refuses what boot
// refuses.
//
// VALIDATES: `ze config validate` and an editor or API commit refuse a rule
// whose source or destination protocol has no block, in the words LoadConfig
// uses.
func TestVerifyDoorRefusesRedistributeWithoutRoot(t *testing.T) {
	registerRootNeed(t)

	errs := VerifyPluginConfigMapTransition(nil, rootNeedTree("isis", rootNeedSource, false).ToPluginMap())
	require.NotEmpty(t, errs)
	assert.Contains(t, JoinPluginConfigVerifyErrors(errs).Error(), `"import rootneed-src" under "destination isis" needs rootneedproto configured`)

	assert.Empty(t, VerifyPluginConfigMapTransition(nil, rootNeedTree("isis", rootNeedSource, true).ToPluginMap()))

	errs = VerifyPluginConfigMapTransition(nil, rootNeedTree(rootNeedDestination, rootNeedPlainSource, false).ToPluginMap())
	require.NotEmpty(t, errs)
	assert.Contains(t, JoinPluginConfigVerifyErrors(errs).Error(), `"destination rootneed-dest" needs rootneedproto configured`)

	assert.Empty(t, VerifyPluginConfigMapTransition(nil, rootNeedTree(rootNeedDestination, rootNeedPlainSource, true).ToPluginMap()))
}
