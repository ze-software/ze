package yang

import (
	"slices"
	"testing"
)

// joinEarlyModule is a compiled-in module that defines `plugin`, a root ze
// reads before any plugin runs.
const joinEarlyModule = `module ze-join-early-conf {
    namespace "urn:ze:join:early";
    prefix early;
    container plugin {
        leaf note {
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

// TestFirstPassRootsAreThePluginAndEnvironmentBlocks: the roots the join
// protects are the ones the first pass of the config load reads. Method: read
// the declaration.
//
// VALIDATES: the set is plugin and environment, and a caller cannot change it
// through the slice it is handed.
// PREVENTS: the join and the first pass disagreeing about which roots are read
// before a plugin runs.
func TestFirstPassRootsAreThePluginAndEnvironmentBlocks(t *testing.T) {
	roots := FirstPassRoots()
	if !slices.Equal(roots, []string{"plugin", "environment"}) {
		t.Fatalf("first-pass roots are %v, want plugin and environment", roots)
	}
	roots[0] = "changed"
	if FirstPassRoots()[0] != "plugin" {
		t.Fatal("a caller changed the first-pass roots through the slice it was handed")
	}
}

// TestJoinRefusesAnAdditionUnderAFirstPassRoot: an external module that
// augments `plugin` is refused at the join (spec risk R-18). Method: a module
// that augments the compiled-in `plugin` container with one leaf.
//
// VALIDATES: the refusal names the plugin, the module, the root and the added
// leaf, and carries a fix.
// PREVENTS: a module the join accepts whose leaf no config can ever hold: the
// first pass reads `plugin` strictly, from the compiled-in schema, before the
// plugin that declares the leaf has run, and refuses it as unknown.
func TestJoinRefusesAnAdditionUnderAFirstPassRoot(t *testing.T) {
	module := `module acme-early {
    namespace "urn:acme:early";
    prefix acmeearly;
    import ze-join-early-conf { prefix early; }
    augment "/early:plugin" {
        leaf watchdog {
            type string;
        }
    }
}`
	joined, err := Join(joinCompiledWithFirstPassRoot(t), []ExternalModule{
		acmeModule("acme-early", "urn:acme:early", module),
	}, joinValidators())
	requireJoinRefusal(t, joined, err,
		`plugin "acme"`, `"acme-early"`, `config root "plugin"`, "watchdog", "before it starts a plugin")
}

// TestJoinAcceptsTheSameAdditionUnderAnotherRoot: the same leaf augmented
// into a root the first pass does not read is accepted, so the refusal above
// is tied to the root and not to the augment.
//
// VALIDATES: an external module may augment a compiled-in container that is
// not a first-pass root.
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
