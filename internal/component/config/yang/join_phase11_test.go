package yang

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// acmeTypesModule is a library module of plugin "acme": a typedef and nothing
// else, as the compiled-in ze-types is.
const acmeTypesModule = `module acme-types {
    namespace "urn:acme:types";
    prefix acmetypes;
    typedef probe-name {
        type string { length "1..16"; }
    }
}`

// acmeLibraryConf is the config module of plugin "acme" that imports
// acmeTypesModule for the type of its one leaf.
const acmeLibraryConf = `module acme-library-conf {
    namespace "urn:acme:library:conf";
    prefix acmelib;
    import acme-types { prefix acmetypes; }
    container acme-library {
        leaf target { type acmetypes:probe-name; }
    }
}`

// acmeAloneConf is a config module of plugin "acme" that imports nothing.
const acmeAloneConf = `module acme-alone-conf {
    namespace "urn:acme:alone:conf";
    prefix acmealone;
    container acme-alone {
        leaf target { type string; }
    }
}`

// TestAddedFailuresKeepsOnlyWhatTheJoinAdds: the failures of the joined set's
// PublishedRPCs that the compiled-in set also has are dropped, and the others
// are kept.
//
// VALIDATES: refuseRPCLinks blames a plugin only for a failure its modules add.
// PREVENTS: a compiled-in failure hiding every failure an external module adds.
func TestAddedFailuresKeepsOnlyWhatTheJoinAdds(t *testing.T) {
	held := errors.New("ze-a:x points at ze-a:y, which its module does not declare")
	added := errors.New("acmecmd:z points at acme-api:w, which its module does not declare")

	if err := addedFailures(errors.Join(held), errors.Join(held)); err != nil {
		t.Errorf("a failure both sets have was answered as added: %v", err)
	}
	if err := addedFailures(nil, nil); err != nil {
		t.Errorf("no failure was answered as added: %v", err)
	}
	got := addedFailures(errors.Join(held), errors.Join(held, added))
	if got == nil {
		t.Fatal("the failure the join adds was dropped")
	}
	if got.Error() != added.Error() {
		t.Errorf("added failures are %q, want %q alone", got.Error(), added.Error())
	}
	if got := addedFailures(nil, added); got == nil || got.Error() != added.Error() {
		t.Errorf("with a clean compiled-in set the joined failure is %v, want %q", got, added.Error())
	}
}

// joinCompiledWithBrokenPointer answers joinCompiledWithCommands with one more
// compiled-in -cmd module whose ze:rpc names an rpc its target module does not
// declare: a compiled-in set PublishedRPCs already refuses.
func joinCompiledWithBrokenPointer(t *testing.T) *Resolved {
	t.Helper()
	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	broken := `module ze-join-broken-cmd {
    namespace "urn:ze:join-broken-cmd";
    prefix brokencmd;
    import ze-extensions { prefix ze; }
    container show {
        config false;
        container broken {
            config false;
            ze:command "ze-join-broken:show-broken";
            ze:rpc "ze-join-base-cmd:no-such-rpc";
        }
    }
}`
	for name, text := range map[string]string{
		"ze-join-base-conf.yang":  joinBaseModule,
		"ze-join-base-cmd.yang":   joinBaseCommands,
		"ze-join-broken-cmd.yang": broken,
	} {
		if err := loader.AddModuleFromText(name, text); err != nil {
			t.Fatalf("add %s: %v", name, err)
		}
	}
	compiled, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve the compiled-in fixture: %v", err)
	}
	if _, err := PublishedRPCs(compiled); err == nil {
		t.Fatal("the compiled-in fixture was meant to fail PublishedRPCs")
	}
	return compiled
}

// TestJoinReportsRPCFailureTheJoinAddsBesideACompiledOne: with a compiled-in
// set that already fails PublishedRPCs, a plugin whose ze:rpc names an rpc its
// -api module does not declare is still refused, naming the plugin, and a
// plugin whose modules add no failure is joined.
//
// VALIDATES: defect (b) of the phase 11 command step: the joined check is no
// longer skipped when the compiled-in set fails it.
// PREVENTS: a broken external pointer joined in silence on a build that
// carries a broken compiled-in one.
func TestJoinReportsRPCFailureTheJoinAddsBesideACompiledOne(t *testing.T) {
	compiled := joinCompiledWithBrokenPointer(t)

	_, err := Join(compiled, acmeCommandModules("acme-status", "acmecmd:show-status",
		`ze:rpc "acme-monitor-api:no-such-rpc";`), joinValidators())
	if err == nil {
		t.Fatal("Join accepted a ze:rpc that names an rpc its module does not declare")
	}
	if !strings.Contains(err.Error(), `plugin "acme"`) {
		t.Errorf("the refusal does not name the plugin: %v", err)
	}
	if !strings.Contains(err.Error(), "acme-monitor-api:no-such-rpc") {
		t.Errorf("the refusal does not name the pointer the plugin added: %v", err)
	}
	if strings.Contains(err.Error(), "ze-join-base-cmd:no-such-rpc") {
		t.Errorf("the refusal blames the plugin for the compiled-in pointer: %v", err)
	}

	if _, err := Join(compiled, acmeCommandModules("acme-status", "acmecmd:show-status", acmeTargetRPC),
		joinValidators()); err != nil {
		t.Errorf("Join refused a plugin whose modules add no failure: %v", err)
	}
}

// TestModuleCommandsAnswersTheNodesOfTheNamedModules: the ze:command nodes a
// set of modules wrote are answered as typed paths with their module, and the
// nodes of every other module are left out.
//
// VALIDATES: the input of the Stage 1 check command-nodes-served.
// PREVENTS: a plugin judged against another module's commands.
func TestModuleCommandsAnswersTheNodesOfTheNamedModules(t *testing.T) {
	joined, err := Join(joinCompiledWithCommands(t),
		acmeCommandModules("acme-status", "acmecmd:show-status", acmeTargetRPC), joinValidators())
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	want := []ModuleCommand{
		{Path: "show acme-monitor target", Module: "acme-monitor-cmd"},
		{Path: "show acme-status", Module: "acme-monitor-cmd"},
	}
	if got := ModuleCommands(joined, []string{"acme-monitor-cmd", "acme-monitor-api"}); !slices.Equal(got, want) {
		t.Errorf("ModuleCommands of the plugin = %+v, want %+v", got, want)
	}
	compiledOnly := []ModuleCommand{{Path: "show system", Module: "ze-join-base-cmd"}}
	if got := ModuleCommands(joined, []string{"ze-join-base-cmd"}); !slices.Equal(got, compiledOnly) {
		t.Errorf("ModuleCommands of the compiled-in module = %+v, want %+v", got, compiledOnly)
	}
	if got := ModuleCommands(joined, []string{"no-such-module"}); len(got) != 0 {
		t.Errorf("a module the set does not hold wrote %+v", got)
	}
}

// TestJoinAcceptsALibraryModuleAnotherModuleImports: a plugin's typedef module
// that its own config module imports is joined, as the compiled-in ze-types
// is loaded, and the same module that no module imports is still refused as a
// module that adds nothing.
//
// VALIDATES: defect (c) of the phase 11 command step: refuseIdleModules is the
// rule IdleModuleNames applies to every module, compiled-in or not.
// PREVENTS: an external plugin refused for a library module a compiled-in one
// may ship.
func TestJoinAcceptsALibraryModuleAnotherModuleImports(t *testing.T) {
	library := acmeModule("acme-types", "urn:acme:types", acmeTypesModule)
	if _, err := Join(joinCompiledWithCommands(t), []ExternalModule{
		library, acmeModule("acme-library-conf", "urn:acme:library:conf", acmeLibraryConf),
	}, joinValidators()); err != nil {
		t.Errorf("Join refused a library module the plugin's config module imports: %v", err)
	}

	_, err := Join(joinCompiledWithCommands(t), []ExternalModule{
		library, acmeModule("acme-alone-conf", "urn:acme:alone:conf", acmeAloneConf),
	}, joinValidators())
	if err == nil {
		t.Fatal("Join accepted a module that adds nothing and that no module imports")
	}
	if !strings.Contains(err.Error(), `Module "acme-types"`) || !strings.Contains(err.Error(), "adds nothing") {
		t.Errorf("the refusal does not name the idle module: %v", err)
	}
}

// acmeEmptyModule declares no statement at all: a header and a revision, as
// the compiled-in ze-rpki-decorator is.
const acmeEmptyModule = `module acme-empty {
    namespace "urn:acme:empty";
    prefix acmeempty;
    description "Nothing but a header.";
    revision 2026-11-17 { description "Initial."; }
}`

// acmeAnchorCmd is a verb-root anchor: a config false container with a
// ze:help summary and no command, as the compiled-in ze-cli-monitor-cmd is.
const acmeAnchorCmd = `module acme-anchor-cmd {
    namespace "urn:acme:anchor:cmd";
    prefix acmeanchor;
    import ze-extensions { prefix ze; }
    container acme-verb {
        config false;
        ze:help "Commands other modules add under acme-verb.";
    }
}`

// TestIdleModuleNamesJudgesEveryModuleOfASet: over a whole set, the rule names
// each module whose definitions nothing reads and that nothing imports, and
// no other; it names apart a module that declares nothing at all, and it
// takes a -cmd module whose navigation node carries only a ze:help.
//
// The set holds ze-types, which LoadEmbedded loads, and none of the
// compiled-in modules that import it, so ze-types is judged here exactly as
// acme-types is: a typedef library no module of the set imports. That is the
// one rule over a compiled-in module and an external one.
//
// VALIDATES: IdleModuleNames and EmptyModuleNames, the forms of the rule the
// shipped modules are held to (TestNoShippedModuleAddsNothing).
// PREVENTS: the rule answering differently for a compiled-in set, or a
// verb-root anchor reported as a module that adds nothing.
func TestIdleModuleNamesJudgesEveryModuleOfASet(t *testing.T) {
	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	for name, text := range map[string]string{
		"acme-types.yang":       acmeTypesModule,
		"acme-alone-conf.yang":  acmeAloneConf,
		"acme-empty.yang":       acmeEmptyModule,
		"acme-anchor-cmd.yang":  acmeAnchorCmd,
		"ze-join-base-cmd.yang": joinBaseCommands,
	} {
		if err := loader.AddModuleFromText(name, text); err != nil {
			t.Fatalf("add %s: %v", name, err)
		}
	}
	set, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := IdleModuleNames(set); !slices.Equal(got, []string{"acme-types", "ze-types"}) {
		t.Errorf("IdleModuleNames = %v, want [acme-types ze-types]", got)
	}
	if got := EmptyModuleNames(set); !slices.Equal(got, []string{"acme-empty"}) {
		t.Errorf("EmptyModuleNames = %v, want [acme-empty]", got)
	}
}

// TestJoinAcceptsAnEmptyModuleAndAnAnchor: an external module that declares
// no statement at all is joined, since nothing of it is lost and the
// compiled-in set ships such modules; an external -cmd module whose one node
// carries a ze:help and no command is joined, as the compiled-in verb-root
// anchors are; a -cmd module whose node carries neither is still refused.
//
// VALIDATES: refuseIdleModules refuses what IdleModuleNames names, and only
// that, for an external module.
// PREVENTS: an external plugin refused for a module shape the compiled-in set
// ships.
func TestJoinAcceptsAnEmptyModuleAndAnAnchor(t *testing.T) {
	if _, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-empty", "urn:acme:empty", acmeEmptyModule),
	}, joinValidators()); err != nil {
		t.Errorf("Join refused a module that declares nothing: %v", err)
	}
	if _, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-anchor-cmd", "urn:acme:anchor:cmd", acmeAnchorCmd),
	}, joinValidators()); err != nil {
		t.Errorf("Join refused a -cmd anchor whose node carries a ze:help: %v", err)
	}
	bare := strings.Replace(acmeAnchorCmd, `ze:help "Commands other modules add under acme-verb.";`, "", 1)
	_, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-anchor-cmd", "urn:acme:anchor:cmd", bare),
	}, joinValidators())
	if err == nil {
		t.Fatal("Join accepted a -cmd module whose node carries no ze:command and no ze:help")
	}
	if !strings.Contains(err.Error(), `Module "acme-anchor-cmd"`) || !strings.Contains(err.Error(), "adds nothing") {
		t.Errorf("the refusal does not name the idle module: %v", err)
	}
}

// TestDefinesConfigReadsWhatAModuleAdds: a -conf module and an augmenting
// module add config; a -cmd module, an -api module and a typedef library add
// none.
//
// VALIDATES: the rule schemaOutcome reads (defect (a) of the phase 11 command
// step).
// PREVENTS: a plugin that brings only commands reported as one that defines
// a config section.
func TestDefinesConfigReadsWhatAModuleAdds(t *testing.T) {
	augment := `module acme-chrony {
    namespace "urn:acme:chrony";
    prefix chrony;
    import ze-join-base-conf { prefix base; }
    augment "/base:service" {
        container chrony { leaf server { type string; } }
    }
}`
	cases := []struct {
		module ExternalModule
		want   bool
	}{
		{acmeModule("acme-alone-conf", "urn:acme:alone:conf", acmeAloneConf), true},
		{acmeModule("acme-chrony", "urn:acme:chrony", augment), true},
		{acmeCommandModules("acme-status", "acmecmd:show-status", acmeTargetRPC)[0], false},
		{acmeCommandModules("acme-status", "acmecmd:show-status", acmeTargetRPC)[1], false},
		{acmeModule("acme-types", "urn:acme:types", acmeTypesModule), false},
	}
	for _, tc := range cases {
		got, err := DefinesConfig(tc.module)
		if err != nil {
			t.Errorf("%s: %v", tc.module.Module, err)
			continue
		}
		if got != tc.want {
			t.Errorf("DefinesConfig(%s) = %v, want %v", tc.module.Module, got, tc.want)
		}
	}
	if _, err := DefinesConfig(acmeModule("acme-broken", "urn:acme:broken", "module acme-broken {")); err == nil {
		t.Error("a text that does not parse was answered with no error")
	}
}
