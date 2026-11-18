package yang

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/opererr"
)

// joinBaseWith answers the text of the compiled-in fixture module
// ze-join-base-conf (joinBaseModule) with hostLeaf in place of leaf host, so
// a test can plant one defect in it while keeping every node the other join
// fixtures read.
func joinBaseWith(hostLeaf string) string {
	return `module ze-join-base-conf {
    namespace "urn:ze:join-base";
    prefix base;
    container service {
        container resolver {
            leaf enabled { type boolean; }
        }
    }
    container system {
        ` + hostLeaf + `
    }
}`
}

// plantCompiledText replaces the text the compiled-in set recorded for module
// name. No production path can do this: a Resolved's texts are written once,
// by the Loader that parsed them, before its checks ran. The test does it so
// that the only way a join can see the planted defect is by judging the
// compiled-in module again.
func plantCompiledText(t *testing.T, compiled *Resolved, name, text string) {
	t.Helper()
	module := compiled.GetModule(name)
	if module == nil {
		t.Fatalf("the compiled-in fixture lacks module %s", name)
	}
	compiled.set().texts[module] = text
}

// TestJoinDoesNotCheckCompiledInModulesAgain: a join runs checkExtensions,
// checkPatterns and checkStructure over the external modules only. Method:
// after the compiled-in fixture resolved, its recorded text of
// ze-join-base-conf is replaced by one that goyang processes and that one of
// the three checks refuses; a valid external module is then joined. A join
// that judged the compiled-in module again would refuse with that check's
// sentinel, so acceptance proves the check did not run over it. The same
// defects in an external module are refused
// (TestJoinRefusesEachDefectOfAnExternalModule).
//
// VALIDATES: AC-16, the checks judge the external modules only.
// PREVENTS: a start with an external plugin paying the per-module checks of
// every compiled-in module a second time.
func TestJoinDoesNotCheckCompiledInModulesAgain(t *testing.T) {
	cases := []struct {
		name     string
		hostLeaf string
		sentinel error
	}{
		{name: "checkExtensions", hostLeaf: `leaf host { type string; base:no-such-extension "probe"; }`, sentinel: ErrUndeclaredExtension},
		{name: "checkPatterns", hostLeaf: `leaf host { type string { pattern '[a-z'; } }`, sentinel: ErrUncompilablePattern},
		{name: "checkStructure length", hostLeaf: `leaf host { type string { length "10..20 | 1..5"; } }`, sentinel: ErrLengthOrder},
		{name: "checkStructure enum", hostLeaf: `typedef shade { type enumeration { enum red; enum blue; } }
        leaf host { type shade { enum green; } }`, sentinel: ErrEnumRestriction},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			compiled := joinCompiled(t)
			plantCompiledText(t, compiled, "ze-join-base-conf", joinBaseWith(tc.hostLeaf))
			joined, err := Join(compiled, []ExternalModule{
				acmeModule("acme-monitor-conf", "urn:acme:monitor", joinOwnRootModule),
			}, joinValidators())
			if errors.Is(err, tc.sentinel) {
				t.Fatalf("the join judged compiled-in module ze-join-base-conf again: %v", err)
			}
			if err != nil {
				t.Fatalf("Join: %v", err)
			}
			if joined.GetModule("acme-monitor-conf") == nil {
				t.Fatal("the joined set lacks the external module")
			}
		})
	}
}

// TestJoinRefusesEachDefectOfAnExternalModule: every per-module check still
// judges an external module, and the refusal keeps today's text: the plugin,
// the module, the check's sentinel and the location in the plugin's file.
// Method: one external module per defect, joined alone.
//
// VALIDATES: AC-16, an external module is judged by checkExtensions,
// checkPatterns and checkStructure as a compiled-in one was.
// MUTATION: a check scope that skips every module of the joined loader turns
// each row red.
// PREVENTS: the scope that skips the compiled-in modules skipping an external
// one too.
func TestJoinRefusesEachDefectOfAnExternalModule(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		sentinel error
	}{
		{name: "undeclared extension", body: `leaf probe { type string; acme:no-such-extension "probe"; }`, sentinel: ErrUndeclaredExtension},
		{name: "uncompilable pattern", body: `leaf probe { type string { pattern '[a-z'; } }`, sentinel: ErrUncompilablePattern},
		{name: "length parts descending", body: `leaf probe { type string { length "10..20 | 1..5"; } }`, sentinel: ErrLengthOrder},
		{name: "enum restriction", body: `leaf probe { type shade { enum green; } }`, sentinel: ErrEnumRestriction},
		{name: "statement under an extension", body: `leaf probe { type string; acme:note "probe" { bogus "x"; } }`, sentinel: ErrExtensionSubstatement},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := `module acme-probe-conf {
    namespace "urn:acme:probe";
    prefix acme;
    extension note { argument text; }
    typedef shade { type enumeration { enum red; enum blue; } }
    container acme-probe {
        ` + tc.body + `
    }
}`
			joined, err := Join(joinCompiled(t), []ExternalModule{
				acmeModule("acme-probe-conf", "urn:acme:probe", text),
			}, joinValidators())
			requireJoinRefusal(t, joined, err,
				`plugin "acme"`, `"acme-probe-conf"`, "module acme-probe-conf", "acme/acme-probe-conf.yang:")
			if !errors.Is(err, tc.sentinel) {
				t.Errorf("the refusal does not wrap %v: %v", tc.sentinel, err)
			}
		})
	}
}

// acmeBrokenReference augments compiled-in container system with a leafref
// whose path names no node.
const acmeBrokenReference = `module acme-reference {
    namespace "urn:acme:reference";
    prefix acmeref;
    import ze-join-base-conf { prefix base; }
    augment "/base:system" {
        leaf peer {
            type leafref { path "/base:system/base:no-such-node"; }
        }
    }
}`

// TestJoinRefusesASetLevelDefectOfAnExternalModule: the leafref check judges
// the whole joined set. Method: an external module augments a compiled-in
// container with a leafref whose path names no node of the joined tree; the
// leaf sits in the compiled-in module's entry tree, so only a check of the set
// can refuse it.
//
// VALIDATES: AC-16, checkReferences still runs over the joined set.
// PREVENTS: the per-module scope reaching a check whose verdict depends on the
// other modules of the set.
func TestJoinRefusesASetLevelDefectOfAnExternalModule(t *testing.T) {
	joined, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-reference", "urn:acme:reference", acmeBrokenReference),
	}, joinValidators())
	requireJoinRefusal(t, joined, err, `plugin "acme"`, `"acme-reference"`, "no-such-node")
	if !errors.Is(err, ErrLeafrefPath) {
		t.Errorf("the refusal does not wrap ErrLeafrefPath: %v", err)
	}
}

// zenithModule answers the declaration plugin "zenith" makes for one module.
func zenithModule(module, namespace, text string) ExternalModule {
	return ExternalModule{Plugin: "zenith", Module: module, Namespace: namespace, Text: text}
}

// requireNamedRefusal fails unless err refuses module of plugin as the one
// module that is not valid, and its problem names none of others.
func requireNamedRefusal(t *testing.T, joined *Resolved, err error, plugin, module string, others ...string) {
	t.Helper()
	requireJoinRefusal(t, joined, err, `plugin "`+plugin+`"`, `"`+module+`"`)
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		return
	}
	problem := refusal.Problem()
	want := `The YANG module "` + module + `" that plugin "` + plugin + `" declares is not valid`
	if problem != want {
		t.Errorf("the refusal's problem is %q, want %q", problem, want)
	}
	for _, other := range others {
		if strings.Contains(problem, other) {
			t.Errorf("the refusal's problem names %q: %q", other, problem)
		}
	}
}

// TestJoinNamesTheFailingModuleOfSeveral: a refused join of several external
// modules names the one that fails from the errors of its one resolve, which
// carry the location in each plugin's file. Method: three sets of two
// modules. In the second, the first module fails when resolved alone with the
// compiled-in set (it imports the second) and is valid in the joined set,
// while the second carries the defect: a join that named the failing module by
// resolving each module alone with the set would name the first.
//
// VALIDATES: AC-16, the failing module is named without one more whole-set
// resolve per declared module.
// PREVENTS: a refusal that names a module valid in the set, or a refusal of
// "modules not valid together" when one module's own text is at fault.
func TestJoinNamesTheFailingModuleOfSeveral(t *testing.T) {
	zenithProbe := `module zenith-probe-conf {
    namespace "urn:zenith:probe";
    prefix zenith;
    container zenith-probe {
        leaf probe { type string; zenith:no-such-extension "probe"; }
    }
}`
	t.Run("the second module fails", func(t *testing.T) {
		joined, err := Join(joinCompiled(t), []ExternalModule{
			acmeModule("acme-monitor-conf", "urn:acme:monitor", joinOwnRootModule),
			zenithModule("zenith-probe-conf", "urn:zenith:probe", zenithProbe),
		}, joinValidators())
		requireNamedRefusal(t, joined, err, "zenith", "zenith-probe-conf", "acme")
		requireJoinRefusal(t, joined, err, "zenith/zenith-probe-conf.yang:")
		if !errors.Is(err, ErrUndeclaredExtension) {
			t.Errorf("the refusal does not wrap ErrUndeclaredExtension: %v", err)
		}
	})
	t.Run("the first module fails alone and not in the set", func(t *testing.T) {
		// acme-uses-extra refers to the leaf zenith-extra adds: alone with the
		// compiled-in set it imports a module nothing loaded.
		acmeUsesExtra := `module acme-uses-extra {
    namespace "urn:acme:uses-extra";
    prefix acmeuses;
    import ze-join-base-conf { prefix base; }
    import zenith-extra { prefix zextra; }
    augment "/base:system" {
        leaf mirror {
            type leafref { path "/base:system/zextra:extra"; }
        }
    }
}`
		zenithExtra := `module zenith-extra {
    namespace "urn:zenith:extra";
    prefix zextra;
    import ze-join-base-conf { prefix base; }
    augment "/base:system" {
        leaf extra { type string; zextra:no-such-extension "probe"; }
    }
}`
		joined, err := Join(joinCompiled(t), []ExternalModule{
			acmeModule("acme-uses-extra", "urn:acme:uses-extra", acmeUsesExtra),
			zenithModule("zenith-extra", "urn:zenith:extra", zenithExtra),
		}, joinValidators())
		requireNamedRefusal(t, joined, err, "zenith", "zenith-extra", "acme")
		requireJoinRefusal(t, joined, err, "zenith/zenith-extra.yang:")
	})
	t.Run("the first module's leafref fails in the set", func(t *testing.T) {
		joined, err := Join(joinCompiled(t), []ExternalModule{
			acmeModule("acme-reference", "urn:acme:reference", acmeBrokenReference),
			zenithModule("zenith-monitor-conf", "urn:zenith:monitor", `module zenith-monitor-conf {
    namespace "urn:zenith:monitor";
    prefix zenith;
    container zenith-monitor { leaf enabled { type boolean; } }
}`),
		}, joinValidators())
		requireNamedRefusal(t, joined, err, "acme", "acme-reference", "zenith")
		requireJoinRefusal(t, joined, err, "acme/acme-reference.yang:")
		if !errors.Is(err, ErrLeafrefPath) {
			t.Errorf("the refusal does not wrap ErrLeafrefPath: %v", err)
		}
	})
}

// TestJoinNamesTheModuleThatAugmentsADuplicateNode: an augment that adds a
// node its target already holds is refused, naming the augmenting module.
// goyang's Entry.merge records the error on the target's entry, beginning with
// the location of the augment and quoting the location of the node already
// there, and Modules.Process reports a target's entry errors only while its
// module holds an augment not yet applied, so the duplicate was dropped in
// silence. Method: plugin acme brings a container, plugin zenith augments it
// with a leaf of a name it holds; and one module augments a compiled-in
// container the same way.
//
// VALIDATES: AC-16, the module named is the one the error begins with.
// PREVENTS: an augmented node dropped in silence, and a refusal that tells the
// plugin whose module is valid to correct it.
func TestJoinNamesTheModuleThatAugmentsADuplicateNode(t *testing.T) {
	t.Run("between two external modules", func(t *testing.T) {
		zenithAugment := `module zenith-augment {
    namespace "urn:zenith:augment";
    prefix zaugment;
    import acme-monitor-conf { prefix acme; }
    augment "/acme:acme-monitor" {
        leaf interval { type uint8; }
    }
}`
		joined, err := Join(joinCompiled(t), []ExternalModule{
			acmeModule("acme-monitor-conf", "urn:acme:monitor", joinOwnRootModule),
			zenithModule("zenith-augment", "urn:zenith:augment", zenithAugment),
		}, joinValidators())
		requireNamedRefusal(t, joined, err, "zenith", "zenith-augment", "acme")
		requireJoinRefusal(t, joined, err, "zenith/zenith-augment.yang:", "interval")
	})
	t.Run("into a compiled-in container", func(t *testing.T) {
		acmeAugment := `module acme-augment {
    namespace "urn:acme:augment";
    prefix acmeaugment;
    import ze-join-base-conf { prefix base; }
    augment "/base:system" {
        leaf host { type uint8; }
    }
}`
		joined, err := Join(joinCompiled(t), []ExternalModule{
			acmeModule("acme-augment", "urn:acme:augment", acmeAugment),
		}, joinValidators())
		requireJoinRefusal(t, joined, err, `plugin "acme"`, `"acme-augment"`, "acme/acme-augment.yang:", "host")
	})
}

// TestJoinNamesTheModuleOfAnErrorWithNoLocation: goyang reports an import of a
// module nothing loaded as "no such module: <name>", with no location
// (Modules.include). Method: plugin acme imports a misspelled module and
// plugin zenith's module is valid; the one resolve's errors point into no
// file, so the join resolves each module alone to find the one that fails.
//
// VALIDATES: AC-16, a module is named when its error carries no location.
// PREVENTS: a refusal of "modules not valid together" that tells the plugin
// whose module is valid to correct its text.
func TestJoinNamesTheModuleOfAnErrorWithNoLocation(t *testing.T) {
	acmeTypo := `module acme-typo-conf {
    namespace "urn:acme:typo";
    prefix acme;
    import ze-join-bse-conf { prefix base; }
    container acme-typo { leaf enabled { type boolean; } }
}`
	joined, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-typo-conf", "urn:acme:typo", acmeTypo),
		zenithModule("zenith-monitor-conf", "urn:zenith:monitor", zenithMonitorModule),
	}, joinValidators())
	requireNamedRefusal(t, joined, err, "acme", "acme-typo-conf", "zenith")
	requireJoinRefusal(t, joined, err, "ze-join-bse-conf")
}

// zenithMonitorModule is a valid module of plugin zenith.
const zenithMonitorModule = `module zenith-monitor-conf {
    namespace "urn:zenith:monitor";
    prefix zenith;
    container zenith-monitor { leaf enabled { type boolean; } }
}`

// TestJoinDoesNotNameAModuleAnErrorQuotes: a check's error quotes the text it
// refuses, and that text can hold another plugin's file name. Method: plugin
// zenith's module is valid and declared first; plugin acme's pattern, which
// does not compile, holds the text "zenith/zenith-monitor-conf.yang:1".
//
// VALIDATES: AC-16, a module is named by the check that refused it, never by
// a file name its quoted text holds.
// PREVENTS: a refusal that blames the plugin another plugin's text names.
func TestJoinDoesNotNameAModuleAnErrorQuotes(t *testing.T) {
	acmeQuote := `module acme-quote-conf {
    namespace "urn:acme:quote";
    prefix acme;
    container acme-quote {
        leaf probe { type string { pattern 'zenith/zenith-monitor-conf.yang:1 [a-z'; } }
    }
}`
	joined, err := Join(joinCompiled(t), []ExternalModule{
		zenithModule("zenith-monitor-conf", "urn:zenith:monitor", zenithMonitorModule),
		acmeModule("acme-quote-conf", "urn:acme:quote", acmeQuote),
	}, joinValidators())
	requireNamedRefusal(t, joined, err, "acme", "acme-quote-conf", "zenith")
	if !errors.Is(err, ErrUncompilablePattern) {
		t.Errorf("the refusal does not wrap ErrUncompilablePattern: %v", err)
	}
}

// TestJoinRefusesModulesValidOnlyAlone: two modules that each resolve with
// the compiled-in set, and fail together with an error that carries no
// location, are refused together, naming both plugins. Method: plugins acme
// and zenith each deviate compiled-in leaf host as not supported; whichever
// goyang applies second finds no target ("cannot find target node to
// deviate", Entry.ApplyDeviate), and each resolves alone.
//
// VALIDATES: AC-16, invalidTogetherRefusal is reached only when every module
// resolves alone, and says so.
// PREVENTS: one module blamed for a failure no module holds alone.
func TestJoinRefusesModulesValidOnlyAlone(t *testing.T) {
	trim := func(module, prefix string) string {
		return `module ` + module + ` {
    namespace "urn:` + module + `";
    prefix ` + prefix + `;
    import ze-join-base-conf { prefix base; }
    deviation "/base:system/base:host" { deviate not-supported; }
}`
	}
	joined, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-trim", "urn:acme-trim", trim("acme-trim", "acmetrim")),
		zenithModule("zenith-trim", "urn:zenith-trim", trim("zenith-trim", "zenithtrim")),
	}, joinValidators())
	requireJoinRefusal(t, joined, err, `plugins "acme", "zenith"`, "cannot find target node to deviate")
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		return
	}
	if want := `The YANG modules that plugins "acme", "zenith" declare are not valid together`; refusal.Problem() != want {
		t.Errorf("the refusal's problem is %q, want %q", refusal.Problem(), want)
	}
	if !strings.Contains(refusal.Why(), "each module compiles alone") {
		t.Errorf("the refusal's reason does not say each module compiles alone: %q", refusal.Why())
	}
}

// joinCommandModule is a compiled-in -cmd module whose command argument
// carries a pattern, which the command lowering reads from the Resolved.
const joinCommandModule = `module ze-join-probe-cmd {
    namespace "urn:ze:join-probe-cmd";
    prefix probecmd;
    import ze-extensions { prefix ze; }
    container show {
        config false;
        container ping {
            config false;
            ze:command "ze-join-probe:show-ping";
            ze:help "Ping host";
            leaf timeout {
                type string { pattern '\d+[smh]?'; }
                ze:help "Timeout duration";
            }
        }
    }
}`

// TestJoinedSetKeepsThePatternsOfCompiledInModules: the joined set holds every
// pattern the compiled-in set compiled, though checkPatterns no longer walks
// the compiled-in modules. Method: a compiled-in -cmd module with a pattern on
// a command argument, a valid external module joined, then the command tree
// lowered from the joined set; the lowering reads each pattern from the
// Resolved and panics on one it does not hold.
//
// VALIDATES: AC-16, the joined Resolved answers the compiled-in patterns.
// MUTATION: a joined set seeded with the external modules' patterns alone
// makes BuildCommandTree panic with "holds a pattern Loader.Resolve did not
// check".
// PREVENTS: a joined set whose command arguments accept any string, or whose
// lowering panics.
func TestJoinedSetKeepsThePatternsOfCompiledInModules(t *testing.T) {
	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if err := loader.AddModuleFromText("ze-join-probe-cmd.yang", joinCommandModule); err != nil {
		t.Fatalf("add command module: %v", err)
	}
	compiled, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve the compiled-in fixture: %v", err)
	}
	joined, err := Join(compiled, []ExternalModule{
		acmeModule("acme-monitor-conf", "urn:acme:monitor", joinOwnRootModule),
	}, joinValidators())
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	ping := BuildCommandTree(joined).Children["show"].Children["ping"]
	if ping == nil {
		t.Fatal("the joined command tree lacks show ping")
	}
	if len(ping.ArgDefs) != 1 {
		t.Fatalf("show ping carries %d arguments, want 1", len(ping.ArgDefs))
	}
	timeout := &ping.ArgDefs[0]
	if err := command.ValidateArgString("30s", timeout); err != nil {
		t.Errorf("the joined set refused a timeout its pattern accepts: %v", err)
	}
	if err := command.ValidateArgString("abc", timeout); err == nil {
		t.Error("the joined set accepted a timeout its pattern refuses")
	}
}
