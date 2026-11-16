package yang

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// publishSecondModule is a second valid external module, of another plugin,
// with a root no other module of these tests defines.
const publishSecondModule = `module beta-probe-conf {
    namespace "urn:beta:probe";
    prefix beta;
    container beta-probe {
        leaf target { type string; }
    }
}`

// publishBrokenModule parses and does not resolve: its leaf names a type no
// module declares, so the join refuses it.
const publishBrokenModule = `module acme-broken-conf {
    namespace "urn:acme:broken";
    prefix broken;
    container acme-broken {
        leaf size { type no-such-type; }
    }
}`

// publishRegisteredModule is a module a test registers as compiled into ze,
// after a publish.
const publishRegisteredModule = `module ze-publish-probe-conf {
    namespace "urn:ze:publish-probe";
    prefix pubprobe;
    container publish-probe {
        leaf note { type string; }
    }
}`

// acmeMonitor is the declaration plugin "acme" makes for joinOwnRootModule.
func acmeMonitor() ExternalModule {
	return acmeModule("acme-monitor-conf", "urn:acme:monitor", joinOwnRootModule)
}

// betaProbe is the declaration plugin "beta" makes for publishSecondModule.
func betaProbe() ExternalModule {
	return ExternalModule{Plugin: "beta", Module: "beta-probe-conf", Namespace: "urn:beta:probe", Text: publishSecondModule}
}

// requireModuleSet answers the set DefaultLoader answers, and fails the test
// when it answers an error.
func requireModuleSet(t *testing.T, stage string) *Resolved {
	t.Helper()
	set, err := DefaultLoader()
	if err != nil {
		t.Fatalf("%s: DefaultLoader: %v", stage, err)
	}
	return set
}

// TestPublishMakesDefaultLoaderAnswerTheJoinedSet: a publish swaps the set
// DefaultLoader answers for the compiled-in set joined to the declared module.
// Method: take the set, publish one valid module, take the set again.
//
// VALIDATES: after a publish DefaultLoader holds the external module, the
// publish is recorded, and a reader that took the set before the publish still
// holds the set it took.
// PREVENTS: a joined set that reaches only the caller of the join, and a
// publish that changes a Resolved a reader already holds.
func TestPublishMakesDefaultLoaderAnswerTheJoinedSet(t *testing.T) {
	t.Cleanup(ResetPublishedModulesForTest())

	before := requireModuleSet(t, "before the publish")
	if before.GetModule("acme-monitor-conf") != nil {
		t.Fatal("the compiled-in set already holds the external module")
	}
	if _, published := PublishedExternalModules(); published {
		t.Fatal("a set is recorded as published before any publish")
	}

	if err := PublishExternalModules([]ExternalModule{acmeMonitor()}, joinValidators()); err != nil {
		t.Fatalf("publish: %v", err)
	}

	after := requireModuleSet(t, "after the publish")
	if after == before {
		t.Fatal("DefaultLoader still answers the set of before the publish")
	}
	if after.GetModule("acme-monitor-conf") == nil {
		t.Error("the published set lacks the external module")
	}
	if before.GetModule("acme-monitor-conf") != nil {
		t.Error("the publish changed the set a reader already held")
	}
	modules, published := PublishedExternalModules()
	if !published {
		t.Fatal("the publish is not recorded")
	}
	if !slices.Equal(modules, []ExternalModule{acmeMonitor()}) {
		t.Errorf("the recorded modules are %v, want the one declared", modules)
	}
}

// TestPublishSameModulesTwiceChangesNothing: a second publish of the modules
// the process already published is accepted and does nothing, whatever their
// order. Method: publish two modules, publish them again reversed.
//
// VALIDATES: the second call returns nil and DefaultLoader answers the same
// Resolved as after the first.
// PREVENTS: a second road to the same publish (offline check, then boot)
// refusing a start, or resolving the set twice.
func TestPublishSameModulesTwiceChangesNothing(t *testing.T) {
	t.Cleanup(ResetPublishedModulesForTest())

	if err := PublishExternalModules([]ExternalModule{acmeMonitor(), betaProbe()}, joinValidators()); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	first := requireModuleSet(t, "after the first publish")

	if err := PublishExternalModules([]ExternalModule{betaProbe(), acmeMonitor()}, joinValidators()); err != nil {
		t.Fatalf("second publish of the same modules: %v", err)
	}
	if second := requireModuleSet(t, "after the second publish"); second != first {
		t.Error("the second publish of the same modules changed the set DefaultLoader answers")
	}
}

// TestPublishOtherModulesIsRefusedNamingTheRestart: once a set is published, a
// publish of other modules is refused. Method: publish one module, then
// publish the same plugin with another text, then publish no module.
//
// VALIDATES: the refusal is an operator error whose fix is the restart of ze
// and that names the plugin whose declaration differs; the published set is
// unchanged.
// PREVENTS: a plugin's module changing under a running process.
func TestPublishOtherModulesIsRefusedNamingTheRestart(t *testing.T) {
	t.Cleanup(ResetPublishedModulesForTest())

	if err := PublishExternalModules([]ExternalModule{acmeMonitor()}, joinValidators()); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	published := requireModuleSet(t, "after the first publish")

	changed := acmeMonitor()
	changed.Text = strings.Replace(changed.Text, `range "1..10"`, `range "1..20"`, 1)
	err := PublishExternalModules([]ExternalModule{changed}, joinValidators())
	requireJoinRefusal(t, nil, err, "restart ze", `"acme"`, "differ")

	err = PublishExternalModules(nil, joinValidators())
	requireJoinRefusal(t, nil, err, "restart ze", `"acme"`)

	if requireModuleSet(t, "after the refusals") != published {
		t.Error("a refused publish changed the set DefaultLoader answers")
	}
	if verdict := PublishedModule(acmeMonitor()); verdict != ModulePublished {
		t.Errorf("the first declaration is no longer the published one: verdict %d", verdict)
	}
}

// TestPublishRefusedModulePublishesNothing: when the join refuses a module, the
// publish returns the join's error and the process is as it was. Method:
// publish a module that does not resolve, compare with the error Join gives
// for it, then publish a valid module.
//
// VALIDATES: the error is Join's, DefaultLoader answers the compiled-in set,
// no set is recorded, and a later publish is still possible.
// PREVENTS: a half-published set, and a refused module fixing the process's
// set as empty.
func TestPublishRefusedModulePublishesNothing(t *testing.T) {
	t.Cleanup(ResetPublishedModulesForTest())

	compiled := requireModuleSet(t, "before the publish")
	broken := acmeModule("acme-broken-conf", "urn:acme:broken", publishBrokenModule)
	_, joinErr := Join(compiled, []ExternalModule{broken}, joinValidators())
	if joinErr == nil {
		t.Fatal("the join accepted the broken module: the fixture no longer proves a refusal")
	}

	err := PublishExternalModules([]ExternalModule{broken}, joinValidators())
	requireJoinRefusal(t, nil, err, `"acme"`, "acme-broken-conf")
	if err.Error() != joinErr.Error() {
		t.Errorf("the publish changed the join's error:\npublish: %v\njoin:    %v", err, joinErr)
	}
	if requireModuleSet(t, "after the refusal") != compiled {
		t.Error("a refused publish changed the set DefaultLoader answers")
	}
	if _, published := PublishedExternalModules(); published {
		t.Error("a refused publish is recorded as a publish")
	}

	if err := PublishExternalModules([]ExternalModule{acmeMonitor()}, joinValidators()); err != nil {
		t.Fatalf("a valid publish after a refused one: %v", err)
	}
}

// TestPublishNoModuleResolvesNothing: a process that declares no external
// plugin publishes nothing and pays nothing. Method: pointer identity of the
// Resolved before and after a publish of no module.
//
// VALIDATES: AC-9 at the module set: DefaultLoader answers the same Resolved,
// and the process's set is recorded as holding no external module, so a later
// publish of a module is refused.
// PREVENTS: a second resolve of the module set on every start, and a module
// joining a process that started without it.
func TestPublishNoModuleResolvesNothing(t *testing.T) {
	t.Cleanup(ResetPublishedModulesForTest())

	before := requireModuleSet(t, "before the publish")
	if err := PublishExternalModules(nil, joinValidators()); err != nil {
		t.Fatalf("publish of no module: %v", err)
	}
	if requireModuleSet(t, "after the publish") != before {
		t.Error("a publish of no module changed the set DefaultLoader answers")
	}
	modules, published := PublishedExternalModules()
	if !published {
		t.Error("a publish of no module is not recorded")
	}
	if len(modules) != 0 {
		t.Errorf("a publish of no module recorded %v", modules)
	}
	if verdict := PublishedModule(acmeMonitor()); verdict != ModuleUndeclared {
		t.Errorf("a plugin that declared nothing answers verdict %d, want ModuleUndeclared", verdict)
	}

	err := PublishExternalModules([]ExternalModule{acmeMonitor()}, joinValidators())
	requireJoinRefusal(t, nil, err, "restart ze", `"acme"`)
}

// TestPublishRefusesAfterAKeeperReadTheSet: a value built once for the life of
// the process records itself, and a publish that would change the set after it
// is refused. Method: record a keeper, publish a valid module.
//
// VALIDATES: the refusal names the keeper and the plugin, nothing is
// published, and a publish of no module, which changes nothing, still passes.
// PREVENTS: a once-built value answering without the published modules, in
// silence, because something was built before the publish.
func TestPublishRefusesAfterAKeeperReadTheSet(t *testing.T) {
	t.Cleanup(ResetPublishedModulesForTest())

	before := requireModuleSet(t, "before the publish")
	KeepsModuleSet("the probe value")
	if !slices.Contains(moduleSetKeeperNames(), "the probe value") {
		t.Fatal("the keeper is not recorded")
	}

	err := PublishExternalModules([]ExternalModule{acmeMonitor()}, joinValidators())
	requireJoinRefusal(t, nil, err, "the probe value", `"acme"`, "Ze defect")
	if requireModuleSet(t, "after the refusal") != before {
		t.Error("the refused publish changed the set DefaultLoader answers")
	}
	if _, published := PublishedExternalModules(); published {
		t.Error("the refused publish is recorded as a publish")
	}

	if err := PublishExternalModules(nil, joinValidators()); err != nil {
		t.Errorf("a publish of no module after a keeper read the set: %v", err)
	}
}

// TestPublishRebuildsCommandArgDefs: the command argument definitions can be
// built before the publish, so the publish builds them again. Method: seed a
// sentinel map, publish, and look for the sentinel; then the same with a
// refused publish, and with no definitions built.
//
// VALIDATES: a publish that changes the set replaces definitions that were
// built, a refused publish leaves them, and definitions never built stay
// unbuilt for the first use.
// PREVENTS: argument definitions of the set before the publish judging
// commands for the life of the daemon.
func TestPublishRebuildsCommandArgDefs(t *testing.T) {
	const sentinel = "publish probe sentinel path"
	seedSentinel := func() {
		SeedCommandArgDefs(map[string][]command.ArgDef{sentinel: nil})
	}
	holdsSentinel := func() bool {
		_, held := heldCommandArgDefs()[sentinel]
		return held
	}

	t.Run("a refused publish leaves them", func(t *testing.T) {
		t.Cleanup(ResetPublishedModulesForTest())
		seedSentinel()
		broken := acmeModule("acme-broken-conf", "urn:acme:broken", publishBrokenModule)
		if err := PublishExternalModules([]ExternalModule{broken}, joinValidators()); err == nil {
			t.Fatal("the broken module was published")
		}
		if !holdsSentinel() {
			t.Error("a refused publish replaced the argument definitions")
		}
	})

	t.Run("a publish replaces them", func(t *testing.T) {
		t.Cleanup(ResetPublishedModulesForTest())
		seedSentinel()
		if err := PublishExternalModules([]ExternalModule{acmeMonitor()}, joinValidators()); err != nil {
			t.Fatalf("publish: %v", err)
		}
		if holdsSentinel() {
			t.Error("the argument definitions are still the ones of before the publish")
		}
		if heldCommandArgDefs() == nil {
			t.Error("the publish dropped the argument definitions instead of building them again")
		}
	})

	t.Run("none built stays none built", func(t *testing.T) {
		t.Cleanup(ResetPublishedModulesForTest())
		SeedCommandArgDefs(nil)
		if err := PublishExternalModules([]ExternalModule{acmeMonitor()}, joinValidators()); err != nil {
			t.Fatalf("publish: %v", err)
		}
		if heldCommandArgDefs() != nil {
			t.Error("the publish built argument definitions no route had asked for")
		}
	})
}

// TestPublishedModuleComparesADeclarationWithThePublishedSet: the four answers
// a later declaration of a plugin can get. Method: ask before any publish,
// then publish one module and ask for it, for a changed text and for another
// plugin.
//
// VALIDATES: unpublished, published, changed and undeclared are told apart.
// PREVENTS: "no set published" reading as "this plugin declared nothing", and
// a changed module reading as a new plugin.
func TestPublishedModuleComparesADeclarationWithThePublishedSet(t *testing.T) {
	t.Cleanup(ResetPublishedModulesForTest())

	if verdict := PublishedModule(acmeMonitor()); verdict != ModuleSetUnpublished {
		t.Fatalf("before any publish the verdict is %d, want ModuleSetUnpublished", verdict)
	}
	if err := PublishExternalModules([]ExternalModule{acmeMonitor()}, joinValidators()); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if verdict := PublishedModule(acmeMonitor()); verdict != ModulePublished {
		t.Errorf("the published declaration answers %d, want ModulePublished", verdict)
	}
	changed := acmeMonitor()
	changed.Text += "\n"
	if verdict := PublishedModule(changed); verdict != ModuleChanged {
		t.Errorf("a declaration with another text answers %d, want ModuleChanged", verdict)
	}
	if verdict := PublishedModule(betaProbe()); verdict != ModuleUndeclared {
		t.Errorf("a plugin with no published module answers %d, want ModuleUndeclared", verdict)
	}
}

// TestModuleRegisteredAfterAPublishIsJoinedAgain: a test that registers a
// module after a publish gets the registered module and the published one.
// Method: publish, register a module for the test, read the set, restore.
//
// VALIDATES: the set of a new registry generation is the compiled-in set of
// that generation joined to the published modules, and the restore brings back
// a set that still holds them.
// PREVENTS: a registry change dropping the external layer from the set.
func TestModuleRegisteredAfterAPublishIsJoinedAgain(t *testing.T) {
	t.Cleanup(ResetPublishedModulesForTest())

	if err := PublishExternalModules([]ExternalModule{acmeMonitor()}, joinValidators()); err != nil {
		t.Fatalf("publish: %v", err)
	}

	restore := RegisterModuleForTest("ze-publish-probe-conf.yang", publishRegisteredModule)
	registered := requireModuleSet(t, "with the registered module")
	if registered.GetModule("ze-publish-probe-conf") == nil {
		t.Error("the set lacks the module registered after the publish")
	}
	if registered.GetModule("acme-monitor-conf") == nil {
		t.Error("the set lost the published module when the registry changed")
	}
	restore()

	restored := requireModuleSet(t, "after the restore")
	if restored.GetModule("ze-publish-probe-conf") != nil {
		t.Error("the set still holds the module after its registration was restored")
	}
	if restored.GetModule("acme-monitor-conf") == nil {
		t.Error("the set lost the published module after the restore")
	}
}
