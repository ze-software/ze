package yang

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// joinBaseModule and joinSharedModule stand for the modules compiled into ze.
// Both define the top-level container "service", as six shipped modules do:
// the compiled-in set shares that container by a second definition, and the
// join MUST NOT refuse it for that.
const joinBaseModule = `module ze-join-base-conf {
    namespace "urn:ze:join-base";
    prefix base;
    container service {
        container resolver {
            leaf enabled { type boolean; }
        }
    }
    container system {
        leaf host { type string; }
    }
}`

const joinSharedModule = `module ze-join-shared-conf {
    namespace "urn:ze:join-shared";
    prefix shared;
    revision 2026-01-01 { description "Initial revision"; }
    container service {
        container relay {
            leaf enabled { type boolean; }
        }
    }
}`

// joinOwnRootModule is a valid external module: its name ends in -conf and it
// adds one top-level node no compiled-in module defines.
const joinOwnRootModule = `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8 { range "1..10"; } }
    }
}`

// joinKnownValidator is the one ze:validate name the test registry holds.
const joinKnownValidator = "join-known-check"

// joinCompiled answers a small resolved set that stands for the compiled-in
// modules: the two embedded library modules and the two modules above.
func joinCompiled(t *testing.T) *Resolved {
	t.Helper()
	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if err := loader.AddModuleFromText("ze-join-base-conf.yang", joinBaseModule); err != nil {
		t.Fatalf("add base module: %v", err)
	}
	if err := loader.AddModuleFromText("ze-join-shared-conf.yang", joinSharedModule); err != nil {
		t.Fatalf("add shared module: %v", err)
	}
	compiled, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve the compiled-in fixture: %v", err)
	}
	return compiled
}

// joinValidators answers a registry holding joinKnownValidator alone.
func joinValidators() *ValidatorRegistry {
	registry := NewValidatorRegistry()
	registry.Register(joinKnownValidator, CustomValidator{
		ValidateFn: func(string, any) error { return nil },
	})
	return registry
}

// acmeModule answers the declaration plugin "acme" makes for one module text.
func acmeModule(module, namespace, text string) ExternalModule {
	return ExternalModule{Plugin: "acme", Module: module, Namespace: namespace, Text: text}
}

// requireJoinRefusal fails unless err is an operator error that carries a fix
// and whose text holds every wanted part.
func requireJoinRefusal(t *testing.T, joined *Resolved, err error, wants ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("the join accepted the module, want a refusal")
	}
	if joined != nil {
		t.Fatal("the join answered a module set beside its refusal")
	}
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		t.Fatalf("the refusal is %T, want an operator error: %v", err, err)
	}
	if refusal.Fix() == "" {
		t.Fatalf("the refusal names no fix: %v", err)
	}
	text := err.Error()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("the refusal lacks %q:\n%s", want, text)
		}
	}
}

// TestJoinWithNoExternalModuleAnswersTheCompiledSet: a start with no external
// plugin pays nothing. Method: pointer identity, because every resolve
// allocates a new Resolved.
//
// VALIDATES: AC-9 at the join: with no external module the answer is the
// compiled-in set itself.
// PREVENTS: a second resolve of every module on a start that declares no
// external plugin, and a schema cache keyed on the Resolved being rebuilt.
func TestJoinWithNoExternalModuleAnswersTheCompiledSet(t *testing.T) {
	compiled := joinCompiled(t)
	joined, err := Join(compiled, nil, joinValidators())
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if joined != compiled {
		t.Fatal("Join with no external module answered another Resolved, want the compiled-in set itself")
	}
}

// TestJoinAcceptsOwnRoot: a -conf module that adds its own top-level node is
// joined, and the compiled-in modules stay in the set.
//
// VALIDATES: the joined set holds the external module with its node and its
// range, and every compiled-in module.
// PREVENTS: an external module's root staying unknown to the schema.
func TestJoinAcceptsOwnRoot(t *testing.T) {
	compiled := joinCompiled(t)
	joined, err := Join(compiled, []ExternalModule{
		acmeModule("acme-monitor-conf", "urn:acme:monitor", joinOwnRootModule),
	}, joinValidators())
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if joined == compiled {
		t.Fatal("Join answered the compiled-in set, want a set that also holds the external module")
	}
	entry := joined.GetEntry("acme-monitor-conf")
	if entry == nil {
		t.Fatal("the joined set lacks acme-monitor-conf")
	}
	root := entry.Dir["acme-monitor"]
	if root == nil {
		t.Fatal("acme-monitor-conf lacks its top-level node acme-monitor")
	}
	interval := root.Dir["interval"]
	if interval == nil {
		t.Fatal("acme-monitor lacks leaf interval")
	}
	if len(interval.Type.Range) != 1 {
		t.Fatalf("leaf interval carries %d range parts, want 1", len(interval.Type.Range))
	}
	for _, name := range []string{"ze-join-base-conf", "ze-join-shared-conf", "ze-extensions", "ze-types"} {
		if joined.GetModule(name) == nil {
			t.Errorf("the joined set lost compiled-in module %s", name)
		}
	}
	if compiled.GetModule("acme-monitor-conf") != nil {
		t.Error("the join added the external module to the compiled-in set it was given")
	}
}

// TestJoinRefusesInvalidModule: a module text that does not parse is refused
// with the plugin, the module and the loader's location.
//
// VALIDATES: AC-4 for a syntax error.
// PREVENTS: a plugin with a broken module starting with no schema to parse
// its config against.
func TestJoinRefusesInvalidModule(t *testing.T) {
	broken := `module acme-broken-conf {
    namespace "urn:acme:broken";
    prefix broken;
    container acme-broken {
        leaf interval { type uint8; }
`
	joined, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-broken-conf", "urn:acme:broken", broken),
	}, joinValidators())
	requireJoinRefusal(t, joined, err,
		`plugin "acme"`, `"acme-broken-conf"`, "acme/acme-broken-conf.yang:")
	if errors.Unwrap(err) == nil {
		t.Error("the refusal carries no loader error as its cause")
	}
}

// TestJoinRefusesModuleThatDoesNotResolve: a module that parses and fails a
// check of Resolve is refused the same way.
//
// VALIDATES: AC-4 for a module that names a type nothing defines.
// PREVENTS: the join skipping the loader's checks for an external module.
func TestJoinRefusesModuleThatDoesNotResolve(t *testing.T) {
	unresolved := `module acme-unresolved-conf {
    namespace "urn:acme:unresolved";
    prefix unresolved;
    container acme-unresolved {
        leaf interval { type no-such-type; }
    }
}`
	joined, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-unresolved-conf", "urn:acme:unresolved", unresolved),
	}, joinValidators())
	requireJoinRefusal(t, joined, err, `plugin "acme"`, `"acme-unresolved-conf"`, "no-such-type")
	if errors.Unwrap(err) == nil {
		t.Error("the refusal carries no loader error as its cause")
	}
}

// TestJoinRefusesDeclaredFieldMismatch: the module and namespace fields of the
// declaration must agree with the text.
//
// VALIDATES: a declared module name or namespace that differs from the text
// is refused with both values.
// PREVENTS: a plugin being reported under a module name its text does not hold.
func TestJoinRefusesDeclaredFieldMismatch(t *testing.T) {
	t.Run("module", func(t *testing.T) {
		joined, err := Join(joinCompiled(t), []ExternalModule{
			acmeModule("acme-other-conf", "urn:acme:monitor", joinOwnRootModule),
		}, joinValidators())
		requireJoinRefusal(t, joined, err,
			`Plugin "acme"`, `"acme-other-conf"`, `"acme-monitor-conf"`)
	})
	t.Run("namespace", func(t *testing.T) {
		joined, err := Join(joinCompiled(t), []ExternalModule{
			acmeModule("acme-monitor-conf", "urn:acme:other", joinOwnRootModule),
		}, joinValidators())
		requireJoinRefusal(t, joined, err,
			`Plugin "acme"`, `"urn:acme:other"`, `"urn:acme:monitor"`)
	})
}

// TestJoinRefusesModuleNameClash: a module named like a compiled-in module is
// refused at any revision. goyang keys a module by name and revision, so it
// accepts a second revision as another module.
//
// VALIDATES: AC-5 for the name, with and without a different revision.
// PREVENTS: an external module replacing or shadowing a compiled-in one.
func TestJoinRefusesModuleNameClash(t *testing.T) {
	t.Run("same name", func(t *testing.T) {
		clash := `module ze-join-base-conf {
    namespace "urn:acme:clash";
    prefix clash;
    container acme-clash { leaf enabled { type boolean; } }
}`
		joined, err := Join(joinCompiled(t), []ExternalModule{
			acmeModule("ze-join-base-conf", "urn:acme:clash", clash),
		}, joinValidators())
		requireJoinRefusal(t, joined, err,
			`plugin "acme"`, `"ze-join-base-conf"`, "compiled into ze")
	})
	t.Run("another revision", func(t *testing.T) {
		clash := `module ze-join-shared-conf {
    namespace "urn:acme:clash";
    prefix clash;
    revision 2027-06-01 { description "A later revision"; }
    container acme-clash { leaf enabled { type boolean; } }
}`
		joined, err := Join(joinCompiled(t), []ExternalModule{
			acmeModule("ze-join-shared-conf", "urn:acme:clash", clash),
		}, joinValidators())
		requireJoinRefusal(t, joined, err,
			`plugin "acme"`, `"ze-join-shared-conf"`, "compiled into ze")
	})
}

// TestJoinRefusesNamespaceClash: a namespace a compiled-in module holds is
// refused, naming both modules.
//
// VALIDATES: AC-5 for the namespace.
// PREVENTS: two modules answering for one namespace.
func TestJoinRefusesNamespaceClash(t *testing.T) {
	clash := `module acme-namespace-conf {
    namespace "urn:ze:join-base";
    prefix clash;
    container acme-namespace { leaf enabled { type boolean; } }
}`
	joined, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-namespace-conf", "urn:ze:join-base", clash),
	}, joinValidators())
	requireJoinRefusal(t, joined, err,
		`plugin "acme"`, `"acme-namespace-conf"`, `"urn:ze:join-base"`, `"ze-join-base-conf"`)
}

// TestJoinRefusesTopLevelRedefinition: an external module that defines a
// top-level node a compiled-in module defines is refused, with the augment to
// write in its place.
//
// VALIDATES: AC-5 for the node, with the augment form in the fix.
// PREVENTS: Schema.Define merging an external module's container into a
// compiled-in root the way it merges the compiled-in ones.
func TestJoinRefusesTopLevelRedefinition(t *testing.T) {
	redefinition := `module acme-system-conf {
    namespace "urn:acme:system";
    prefix acmesystem;
    container system {
        leaf probe { type string; }
    }
}`
	joined, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-system-conf", "urn:acme:system", redefinition),
	}, joinValidators())
	requireJoinRefusal(t, joined, err,
		`plugin "acme"`, `"acme-system-conf"`, `"system"`, `"ze-join-base-conf"`,
		`import ze-join-base-conf { prefix base; }`, `augment "/base:system"`)
}

// TestJoinAcceptsAugmentOfSharedContainer: an external module reaches a
// container two compiled-in modules define by augmenting one of them. Its name
// does not end in -conf: an augment contributes whatever the module is named.
//
// VALIDATES: AC-6 at the module set: the augmented child sits under the
// compiled-in container of the joined set.
// PREVENTS: the redefinition refusal, or the naming rule, refusing the
// sanctioned way to share a container.
func TestJoinAcceptsAugmentOfSharedContainer(t *testing.T) {
	augment := `module acme-chrony {
    namespace "urn:acme:chrony";
    prefix chrony;
    import ze-join-base-conf { prefix base; }
    augment "/base:service" {
        container chrony {
            leaf server { type string; }
        }
    }
}`
	compiled := joinCompiled(t)
	joined, err := Join(compiled, []ExternalModule{
		acmeModule("acme-chrony", "urn:acme:chrony", augment),
	}, joinValidators())
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	service := joined.GetEntry("ze-join-base-conf").Dir["service"]
	if service == nil {
		t.Fatal("the joined set lacks container service of ze-join-base-conf")
	}
	if service.Dir["chrony"] == nil {
		t.Error("container service lacks the augmented child chrony")
	}
	if service.Dir["resolver"] == nil {
		t.Error("container service lost its compiled-in child resolver")
	}
	if compiled.GetEntry("ze-join-base-conf").Dir["service"].Dir["chrony"] != nil {
		t.Error("the join augmented the compiled-in set it was given")
	}
}

// TestJoinRefusesUnregisteredValidator: an external module that names a
// ze:validate function the registry does not hold is refused.
//
// VALIDATES: AC-11 for ze:validate on a leaf of the module's own root.
// PREVENTS: an annotation that checks nothing being accepted in silence.
func TestJoinRefusesUnregisteredValidator(t *testing.T) {
	module := `module acme-validate-conf {
    namespace "urn:acme:validate";
    prefix acmevalidate;
    import ze-extensions { prefix ze; }
    container acme-validate {
        leaf target {
            type string;
            ze:validate "acme-no-such-check";
        }
    }
}`
	joined, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-validate-conf", "urn:acme:validate", module),
	}, joinValidators())
	requireJoinRefusal(t, joined, err,
		`plugin "acme"`, `"acme-validate-conf"`, "ze:validate", `"acme-no-such-check"`)
}

// TestJoinRefusesUnregisteredValidatorInAugment: the same refusal reaches a
// leaf the module augments into a compiled-in container, where the leaf sits
// in the entry tree of the compiled-in module.
//
// VALIDATES: AC-11 for ze:validate under an augment, named for the plugin
// whose module wrote it.
// PREVENTS: the check reading only the external module's own entry tree.
func TestJoinRefusesUnregisteredValidatorInAugment(t *testing.T) {
	module := `module acme-augment-validate {
    namespace "urn:acme:augment-validate";
    prefix acmeaugment;
    import ze-extensions { prefix ze; }
    import ze-join-base-conf { prefix base; }
    augment "/base:service" {
        container chrony {
            leaf server {
                type string;
                ze:validate "acme-no-such-check";
            }
        }
    }
}`
	joined, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-augment-validate", "urn:acme:augment-validate", module),
	}, joinValidators())
	requireJoinRefusal(t, joined, err,
		`plugin "acme"`, `"acme-augment-validate"`, "ze:validate", `"acme-no-such-check"`)
}

// TestJoinAcceptsRegisteredValidator: a ze:validate name the registry holds is
// accepted, so the refusal above is tied to the registry and not to the
// extension.
//
// VALIDATES: an external module may use a validator the daemon holds.
// PREVENTS: a check that refuses every ze:validate of an external module.
func TestJoinAcceptsRegisteredValidator(t *testing.T) {
	module := `module acme-known-conf {
    namespace "urn:acme:known";
    prefix acmeknown;
    import ze-extensions { prefix ze; }
    container acme-known {
        leaf target {
            type string;
            ze:validate "` + joinKnownValidator + `";
        }
    }
}`
	_, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-known-conf", "urn:acme:known", module),
	}, joinValidators())
	if err != nil {
		t.Fatalf("Join refused a registered validator: %v", err)
	}
}

// TestJoinRefusesUnregisteredReferenceSource: an external module that names a
// ze:reference source no component registered is refused.
//
// VALIDATES: AC-11 for ze:reference.
// PREVENTS: a leaf that refuses every value at commit being accepted at start.
func TestJoinRefusesUnregisteredReferenceSource(t *testing.T) {
	module := `module acme-reference-conf {
    namespace "urn:acme:reference";
    prefix acmereference;
    import ze-extensions { prefix ze; }
    container acme-reference {
        leaf target {
            type string;
            ze:reference "acme-no-such-source";
        }
    }
}`
	joined, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-reference-conf", "urn:acme:reference", module),
	}, joinValidators())
	requireJoinRefusal(t, joined, err,
		`plugin "acme"`, `"acme-reference-conf"`, "ze:reference", `"acme-no-such-source"`)
}

// TestJoinRefusesModuleThatContributesNothing: a module whose name does not
// end in -conf and that augments nothing would add nothing to the config
// schema, and is refused with the naming rule.
//
// VALIDATES: the -conf rule is stated to the author.
// PREVENTS: a plugin whose config root stays an unknown keyword with no
// reason given.
func TestJoinRefusesModuleThatContributesNothing(t *testing.T) {
	module := `module acme-monitor {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8; }
    }
}`
	joined, err := Join(joinCompiled(t), []ExternalModule{
		acmeModule("acme-monitor", "urn:acme:monitor", module),
	}, joinValidators())
	requireJoinRefusal(t, joined, err,
		`plugin "acme"`, `"acme-monitor"`, "-conf", `"acme-monitor-conf"`)
}

// TestJoinRefusesClashBetweenTwoExternalModules: two plugins whose modules
// clash with each other are refused, naming both plugins.
//
// VALIDATES: a module name, a namespace and a top-level node that two external
// modules share are each refused with both plugin names.
// PREVENTS: the second plugin's module replacing the first one's.
func TestJoinRefusesClashBetweenTwoExternalModules(t *testing.T) {
	first := acmeModule("acme-monitor-conf", "urn:acme:monitor", joinOwnRootModule)
	t.Run("module name", func(t *testing.T) {
		second := ExternalModule{
			Plugin: "zenith", Module: "acme-monitor-conf", Namespace: "urn:zenith:monitor",
			Text: `module acme-monitor-conf {
    namespace "urn:zenith:monitor";
    prefix zenith;
    container zenith-monitor { leaf enabled { type boolean; } }
}`,
		}
		joined, err := Join(joinCompiled(t), []ExternalModule{first, second}, joinValidators())
		requireJoinRefusal(t, joined, err, `plugin "acme"`, `plugin "zenith"`, `"acme-monitor-conf"`)
	})
	t.Run("namespace", func(t *testing.T) {
		second := ExternalModule{
			Plugin: "zenith", Module: "zenith-monitor-conf", Namespace: "urn:acme:monitor",
			Text: `module zenith-monitor-conf {
    namespace "urn:acme:monitor";
    prefix zenith;
    container zenith-monitor { leaf enabled { type boolean; } }
}`,
		}
		joined, err := Join(joinCompiled(t), []ExternalModule{first, second}, joinValidators())
		requireJoinRefusal(t, joined, err, `plugin "acme"`, `plugin "zenith"`, `"urn:acme:monitor"`)
	})
	t.Run("top-level node", func(t *testing.T) {
		second := ExternalModule{
			Plugin: "zenith", Module: "zenith-monitor-conf", Namespace: "urn:zenith:monitor",
			Text: `module zenith-monitor-conf {
    namespace "urn:zenith:monitor";
    prefix zenith;
    container acme-monitor { leaf enabled { type boolean; } }
}`,
		}
		joined, err := Join(joinCompiled(t), []ExternalModule{first, second}, joinValidators())
		requireJoinRefusal(t, joined, err,
			`plugin "acme"`, `plugin "zenith"`, `"acme-monitor"`, `augment "/acme:acme-monitor"`)
	})
}

// TestExternalModuleFromTextReadsTheDeclarationFromTheText: a caller that holds
// only a module text gets the declaration the text states, and a text that
// does not parse is refused there.
//
// VALIDATES: the name and namespace are the text's own; a broken text answers
// the invalid-module refusal.
// PREVENTS: a Go caller of the join writing the name and namespace by hand
// beside the text that already states them.
func TestExternalModuleFromTextReadsTheDeclarationFromTheText(t *testing.T) {
	external, err := ExternalModuleFromText("acme", joinOwnRootModule)
	if err != nil {
		t.Fatalf("ExternalModuleFromText: %v", err)
	}
	want := acmeModule("acme-monitor-conf", "urn:acme:monitor", joinOwnRootModule)
	if external != want {
		t.Errorf("ExternalModuleFromText = %+v, want %+v", external, want)
	}
	_, err = ExternalModuleFromText("acme", "module acme-broken-conf {")
	requireJoinRefusal(t, nil, err, `plugin "acme"`)
}
