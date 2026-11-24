package yang

import (
	"slices"
	"testing"
)

// joinEarlyModule is a compiled-in module that defines `bus`, marked with
// ze:first-pass so ze reads it before any external plugin runs.
const joinEarlyModule = `module ze-join-early-conf {
    namespace "urn:ze:join:early";
    prefix early;
    import ze-extensions { prefix ze; }
    container bus {
        ze:first-pass;
        leaf note {
            type string;
        }
    }
}`

// joinEarlyEnvModule is a second compiled-in module marking `environment` with
// ze:first-pass, proving FirstPassRoots collects and sorts roots across modules.
const joinEarlyEnvModule = `module ze-join-env-conf {
    namespace "urn:ze:join:env";
    prefix env;
    import ze-extensions { prefix ze; }
    container environment {
        ze:first-pass;
        leaf level {
            type string;
        }
    }
}`

// joinCompiledWithFirstPassRoot answers the compiled-in fixture of the join
// tests with joinEarlyModule beside it.
func joinCompiledWithFirstPassRoot(t *testing.T) *Resolved {
	t.Helper()
	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if err := loader.AddModuleFromText("ze-join-base-conf.yang", joinBaseModule); err != nil {
		t.Fatalf("add base module: %v", err)
	}
	if err := loader.AddModuleFromText("ze-join-early-conf.yang", joinEarlyModule); err != nil {
		t.Fatalf("add early module: %v", err)
	}
	compiled, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve the compiled-in fixture: %v", err)
	}
	return compiled
}

// TestFirstPassRootsDerivedFromExtension verifies that FirstPassRoots derives
// top-level roots from ze:first-pass annotations across resolved -conf modules
// rather than a hardcoded literal list.
//
// VALIDATES: Resolved.FirstPassRoots and FirstPassRoots return sorted,
// deduplicated root names annotated with ze:first-pass and never expose shared
// slice backing storage.
// PREVENTS: hardcoded first-pass root literals drifting from the YANG modules.
func TestFirstPassRootsDerivedFromExtension(t *testing.T) {
	restoreBus := RegisterModuleForTest("ze-join-early-conf.yang", joinEarlyModule)
	t.Cleanup(restoreBus)
	restoreEnv := RegisterModuleForTest("ze-join-env-conf.yang", joinEarlyEnvModule)
	t.Cleanup(restoreEnv)

	roots := FirstPassRoots()
	if !slices.Equal(roots, []string{"bus", "environment"}) {
		t.Fatalf("first-pass roots are %v, want [bus environment]", roots)
	}
	roots[0] = "changed"
	if FirstPassRoots()[0] != "bus" {
		t.Fatal("a caller changed the first-pass roots through the slice it was handed")
	}
}

// TestJoinRefusesAnAdditionUnderAFirstPassRoot: an external module that
// augments a root carrying ze:first-pass is refused at the join.
//
// VALIDATES: the refusal names the plugin, the module, the root and the added
// leaf, and carries a fix.
// PREVENTS: a module the join accepts whose leaf no config can ever hold: the
// first pass reads first-pass roots strictly before external plugins start.
func TestJoinRefusesAnAdditionUnderAFirstPassRoot(t *testing.T) {
	module := `module acme-early {
    namespace "urn:acme:early";
    prefix acmeearly;
    import ze-join-early-conf { prefix early; }
    augment "/early:bus" {
        leaf watchdog {
            type string;
        }
    }
}`
	joined, err := Join(joinCompiledWithFirstPassRoot(t), []ExternalModule{
		acmeModule("acme-early", "urn:acme:early", module),
	}, joinValidators())
	requireJoinRefusal(t, joined, err,
		`plugin "acme"`, `"acme-early"`, `config root "bus"`, "watchdog", "before it starts a plugin")
}

// TestJoinAcceptsTheSameAdditionUnderAnotherRoot: the same leaf augmented
// into a root without ze:first-pass is accepted, so the refusal above is tied
// to the extension and not to every augment.
//
// VALIDATES: an external module may augment a compiled-in container that does
// not carry ze:first-pass.
// PREVENTS: a check that refuses every augment of an external module.
func TestJoinAcceptsTheSameAdditionUnderAnotherRoot(t *testing.T) {
	module := `module acme-late {
    namespace "urn:acme:late";
    prefix acmelate;
    import ze-join-base-conf { prefix base; }
    augment "/base:service" {
        leaf watchdog {
            type string;
        }
    }
}`
	joined, err := Join(joinCompiledWithFirstPassRoot(t), []ExternalModule{
		acmeModule("acme-late", "urn:acme:late", module),
	}, joinValidators())
	if err != nil {
		t.Fatalf("the join refused an augment outside the first-pass roots: %v", err)
	}
	if joined == nil {
		t.Fatal("the join answered no module set and no refusal")
	}
}
