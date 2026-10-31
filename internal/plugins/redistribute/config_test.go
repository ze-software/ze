package redistribute

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// redistSection is the `redistribute` section the engine delivers for
// `redistribute { destination isis { import <source> } }`, built by the
// config component's own section builder so the test reads the wire shape the
// daemon sends rather than a hand-written copy of it.
func redistSection(t *testing.T, source string) []sdk.ConfigSection {
	t.Helper()
	tree := config.NewTree()
	redist := config.NewTree()
	tree.SetContainer(configRoot, redist)
	dest := config.NewTree()
	dest.AddListEntry("import", source, config.NewTree())
	redist.AddListEntry("destination", "isis", dest)
	sections, err := config.BuildPluginConfigSections(tree.ToPluginMap(), []string{configRoot})
	require.NoError(t, err)
	require.Len(t, sections, 1)
	return sections
}

// restoreGlobal puts back the evaluator the test found, so tests that install
// one do not leak it into the next.
func restoreGlobal(t *testing.T) {
	t.Helper()
	saved := configredist.Global()
	t.Cleanup(func() { configredist.SetGlobal(saved) })
}

// TestConfigureInstallsTheRules proves the orchestrator installs the import
// rules from its own config root.
//
// VALIDATES: a config with no bgp block redistributes: the evaluator the
// dispatcher reads is set by the plugin that owns `redistribute`, not by BGP's
// reactor builder, which does not run without a bgp block.
// PREVENTS: an orchestrator that boots with no evaluator and drops every batch
// with "no evaluator configured".
func TestConfigureInstallsTheRules(t *testing.T) {
	restoreGlobal(t)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(nil)

	require.NoError(t, (&ruleState{}).configure(redistSection(t, "fakeredist")))

	ev := configredist.Global()
	require.NotNil(t, ev, "configure installs an evaluator")
	assert.True(t, ev.HasDestination("isis"))
	_, accepted := ev.Accept(&configredist.RedistRoute{Origin: "fakeredist", Source: "fakeredist", Route: ribpolicy.Route{Family: family.IPv4Unicast}}, "isis")
	assert.True(t, accepted)
}

// TestConfigureRefusesAnUnknownSource proves a rule the parser refuses stops
// the plugin's startup rather than leaving redistribution off.
//
// VALIDATES: the refusal is the config component's, in its words.
// PREVENTS: one mistyped source silently disabling every rule in the file.
func TestConfigureRefusesAnUnknownSource(t *testing.T) {
	restoreGlobal(t)
	err := (&ruleState{}).configure(redistSection(t, "rip"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown source "rip" under destination "isis"`)
}

// TestConfigureWithNoRulesInstallsAnEmptyEvaluator proves a startup or reload
// that carries no rules replaces whatever was installed before.
//
// VALIDATES: an evaluator is always installed, and it accepts nothing when the
// config asks for nothing.
// PREVENTS: a removed `redistribute` block that keeps redistributing.
func TestConfigureWithNoRulesInstallsAnEmptyEvaluator(t *testing.T) {
	restoreGlobal(t)
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{{Source: "connected", Destination: "ospf"}}))

	require.NoError(t, (&ruleState{}).configure([]sdk.ConfigSection{{Root: configRoot, Data: `{}`}}))

	ev := configredist.Global()
	require.NotNil(t, ev)
	assert.False(t, ev.HasDestination("ospf"), "the stale rule is gone")
	assert.Empty(t, ev.Rules())
}

// TestReloadAppliesAndRollsBack proves a reload installs the verified rules
// only at apply, and a rollback restores the rules it replaced.
//
// VALIDATES: the reload path of the evaluator, which BGP's reload used to own.
// PREVENTS: a rejected transaction leaving its rules live, and a verify alone
// changing what redistributes.
func TestReloadAppliesAndRollsBack(t *testing.T) {
	restoreGlobal(t)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	running := configredist.NewEvaluator([]configredist.ImportRule{{Source: "fakeredist", Destination: "ospf"}})
	configredist.SetGlobal(running)

	state := &ruleState{}
	require.NoError(t, state.verify(redistSection(t, "fakeredist")))
	assert.Same(t, running, configredist.Global(), "verify changes nothing")

	require.NoError(t, state.apply())
	require.True(t, configredist.Global().HasDestination("isis"), "apply installs the candidate")

	require.NoError(t, state.rollback())
	assert.Same(t, running, configredist.Global(), "rollback restores the running rules")

	require.NoError(t, state.apply())
	assert.Same(t, running, configredist.Global(), "an apply with no verified candidate changes nothing")
}

// TestFactoryConsumersAreBuiltOverTheOrchestratorConnection proves a consumer
// factory's destination is built over the connection the orchestrator lends it.
//
// VALIDATES: the orchestrator names no destination: a destination's own
// package registers a factory (BGP does), and the orchestrator builds it.
// PREVENTS: a protocol-specific consumer constructed inside this plugin, which
// is what made it depend on BGP.
func TestFactoryConsumersAreBuiltOverTheOrchestratorConnection(t *testing.T) {
	configredist.ResetConsumersForTest()
	t.Cleanup(configredist.ResetConsumersForTest)
	var lent configredist.RouteUpdater
	require.NoError(t, configredist.RegisterDestination(configredist.Destination{Name: "factorytest"}))
	require.NoError(t, configredist.RegisterConsumerFactory(configredist.ConsumerFactory{
		Name: "factorytest",
		New: func(u configredist.RouteUpdater) configredist.RedistConsumer {
			lent = u
			return &stubConsumer{name: "factorytest"}
		},
	}))
	updater := &nopUpdater{}

	buildFactoryConsumers(updater)

	_, ok := configredist.LookupConsumer("factorytest")
	assert.True(t, ok, "the built consumer is registered")
	assert.Same(t, updater, lent, "the factory receives the orchestrator's connection")
}

// nopUpdater is a RouteUpdater that is never asked to send.
type nopUpdater struct{}

func (*nopUpdater) UpdateRoute(context.Context, string, string) (uint32, uint32, error) {
	return 0, 0, nil
}

// TestReloadThatDeletesTheRootInstallsAnEmptyEvaluator proves a reload that
// removes the `redistribute` block stops redistributing.
//
// VALIDATES: the engine delivers a deleted root as an empty section
// (reloadConfigSections), and verify then apply installs an evaluator that
// accepts nothing.
// PREVENTS: a removed `redistribute` block whose rules stay live until restart.
func TestReloadThatDeletesTheRootInstallsAnEmptyEvaluator(t *testing.T) {
	restoreGlobal(t)
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{{Source: "connected", Destination: "ospf"}}))

	state := &ruleState{}
	require.NoError(t, state.verify([]sdk.ConfigSection{{Root: configRoot, Data: `{}`}}))
	require.NoError(t, state.apply())

	ev := configredist.Global()
	require.NotNil(t, ev)
	assert.False(t, ev.HasDestination("ospf"), "the deleted rules are gone")
	assert.Empty(t, ev.Rules())
}

// chainSections is the delivery for `redistribute { destination <dest> {
// import fakeredist { filters [ T42 ]; final <final>; } } }` beside a policy
// root defining `match T42 { tag [ 42 ]; }`. An empty final names no chain.
func chainSections(t *testing.T, dest, final string) []sdk.ConfigSection {
	t.Helper()
	tree := config.NewTree()
	redist := config.NewTree()
	tree.SetContainer(configRoot, redist)
	destTree := config.NewTree()
	entry := config.NewTree()
	if final != "" {
		entry.SetSlice("filters", []string{"T42"})
		entry.Set("final", final)
	}
	destTree.AddListEntry("import", "fakeredist", entry)
	redist.AddListEntry("destination", dest, destTree)
	sections, err := config.BuildPluginConfigSections(tree.ToPluginMap(), []string{configRoot})
	require.NoError(t, err)
	return append(sections, sdk.ConfigSection{Root: policy.RootName, Data: `{"policy":{"match":{"T42":{"tag":["42"]}}}}`})
}

// TestRuleChainCompilesFromPolicy is AC-14 at the orchestrator's config: an
// import's `filters [ T42 ]; final accept;` compiles against the delivered
// policy root, and the installed rule moves only the routes T42 passes. A name
// the policy root does not define is refused at configure.
//
// VALIDATES: the redistribute import chain compiles through
// policy.CompileRouteChain and decides acceptance.
// PREVENTS: a chain parsed and never compiled, which Accept would hit as a
// final with no chain.
func TestRuleChainCompilesFromPolicy(t *testing.T) {
	restoreGlobal(t)
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))

	require.NoError(t, (&ruleState{}).configure(chainSections(t, "isis", "accept")))
	ev := configredist.Global()
	require.NotNil(t, ev)
	route := func(tag uint32) *configredist.RedistRoute {
		return &configredist.RedistRoute{Origin: "fakeredist", Source: "fakeredist",
			Route: ribpolicy.Route{Family: family.IPv4Unicast, Tag: tag, HasTag: tag != 0}}
	}
	_, accepted := ev.Accept(route(42), "isis")
	assert.True(t, accepted, "a route T42 passes moves")
	_, accepted = ev.Accept(route(7), "isis")
	assert.False(t, accepted, "a route T42 rejects does not move")

	sections := chainSections(t, "isis", "accept")
	sections[len(sections)-1].Data = `{"policy":{}}`
	err := (&ruleState{}).configure(sections)
	require.Error(t, err, "a chain naming an undefined instance is refused")
	assert.Contains(t, err.Error(), "T42")
}

// TestRuleNarrowingWithdrawsHeldRoutes is AC-14's reload clause: a reload that
// narrows an import's chain withdraws, from the consumer, every held route the
// new chain rejects, and keeps the ones it still accepts.
//
// VALIDATES: the held set keeps each route's view, and apply re-judges it.
// PREVENTS: a narrowed chain that stops new routes but leaves the routes it now
// rejects in the destination until their source next changes them.
func TestRuleNarrowingWithdrawsHeldRoutes(t *testing.T) {
	resetState(t)
	id := redistevents.RegisterProtocol("fakeredist")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	consumer := registerBGPConsumer(t)

	state := &ruleState{}
	require.NoError(t, state.configure(chainSections(t, "bgp", "")))
	batch := &redistevents.RouteChangeBatch{Protocol: id, AFI: afiIPv4, SAFI: safiUnicst, Entries: []redistevents.RouteChangeEntry{
		{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.0.0.42/32"), Tag: 42},
		{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.0.0.7/32"), Tag: 7},
	}}
	handleBatch(context.Background(), skipIDs(), batch)
	require.Len(t, consumer.snapshotInjected(), 2, "with no chain both routes move")
	require.Empty(t, consumer.snapshotWithdrawn())

	require.NoError(t, state.verify(chainSections(t, "bgp", "accept")))
	require.NoError(t, state.apply())

	withdrawn := consumer.snapshotWithdrawn()
	require.Len(t, withdrawn, 1, "only the route the narrowed chain rejects is withdrawn")
	assert.Equal(t, "10.0.0.7/32", withdrawn[0].prefix)
	assert.Equal(t, family.IPv4Unicast, withdrawn[0].fam)
}
