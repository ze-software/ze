package cli

import (
	"slices"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// backendProbeModule declares two containers a component could declare. Neither
// name appears in completer.go, so a literal list of backend roots cannot see
// either one.
//
// test-backend-open/component holds the shape a component's backend selector
// has: a `backend` leaf of open string type, because the backend registers at
// run time. test-backend-closed holds the shape a log destination has: the same
// leaf name over an enumeration the schema already closes.
const backendProbeModule = `module ze-test-backend-conf {
    namespace "urn:ze:test-backend:conf";
    prefix tbe;

    revision 2026-01-01 { description "Probe module for the backend-root derivation."; }

    container test-backend-open {
        description "A component whose backend registers at run time.";
        container component {
            description "The container declaring the selector.";
            leaf backend {
                type string;
                description "Backend implementation name.";
            }
        }
    }

    container test-backend-closed {
        description "A destination the schema closes.";
        leaf backend {
            type enumeration {
                enum first { description "First destination."; }
                enum second { description "Second destination."; }
            }
            description "Output destination, not a component backend.";
        }
    }
}`

// probeCompleter builds a Completer over the shipped model plus the probe
// module above.
func probeCompleter(t *testing.T) *Completer {
	t.Helper()

	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load embedded modules: %v", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		t.Fatalf("load registered modules: %v", err)
	}
	if err := loader.AddModuleFromText("ze-test-backend-conf", backendProbeModule); err != nil {
		t.Fatalf("add the probe module: %v", err)
	}
	if err := loader.Resolve(); err != nil {
		t.Fatalf("resolve the model: %v", err)
	}
	return &Completer{loader: loader}
}

// TestBackendRootsSeeAContainerCompleterGoDoesNotName is the discrimination
// test for the schema derivation. The probe module declares a container whose
// name appears nowhere in completer.go, and backendRoots must answer it.
//
// A literal map of backend roots makes this RED: the probe container is in no
// list, so the derived set omits it and the editor completes that component's
// backend-gated nodes as though no backend were active.
//
// VALIDATES: backendRoots derives its containers from the compiled YANG model,
// so a component that gains a `backend` leaf is covered with no edit to
// internal/component/cli.
// PREVENTS: a component root spelled in a central completion package
// (ai/rules/plugins.md), which goes stale the moment a component is added.
func TestBackendRootsSeeAContainerCompleterGoDoesNotName(t *testing.T) {
	roots := probeCompleter(t).backendRoots()

	const want = "test-backend-open/component"
	if !slices.Contains(roots, want) {
		t.Errorf("want %q among the derived backend roots, got %v", want, roots)
	}
}

// TestBackendRootsRefuseAClosedEnumeration proves the discriminator that keeps
// a log destination out of the active-backend set.
//
// VALIDATES: a `backend` leaf over a closed enumeration is not a component
// backend selector and is not derived as one.
// PREVENTS: environment/log's stderr, stdout and syslog entering the active
// backend values, which backendAllowed matches against every ze:backend
// annotation -- a node annotated ze:backend "syslog" would then be completed
// for every operator whose log goes there.
func TestBackendRootsRefuseAClosedEnumeration(t *testing.T) {
	roots := probeCompleter(t).backendRoots()

	for _, refused := range []string{"test-backend-closed", "environment/log"} {
		if slices.Contains(roots, refused) {
			t.Errorf("%q declares a closed `backend` enumeration and must not be a backend root: %v", refused, roots)
		}
	}
}

// TestEveryBackendRootCarriesAnOpenBackendLeaf proves the derivation answers
// only roots deriveBackends can actually read, and answers something rather
// than nothing.
//
// VALIDATES: each derived root resolves in the model and carries an
// open-string `backend` leaf there.
// PREVENTS: an empty or wrong derivation answering silently, which disables
// every ze:backend filter with no error at all (ai/rules/principles.md, "a
// zero value is never an answer").
func TestEveryBackendRootCarriesAnOpenBackendLeaf(t *testing.T) {
	completer := probeCompleter(t)

	roots := completer.backendRoots()
	if len(roots) == 0 {
		t.Fatal("the probe module declares a backend root and the derivation answered none")
	}

	for _, root := range roots {
		leaf := findSchemaLeaf(t, completer.loader, root, leafBackend)
		if leaf == nil {
			t.Errorf("root %q: the model declares no %q leaf there", root, leafBackend)
			continue
		}
		if !isOpenString(leaf) {
			t.Errorf("root %q: its %q leaf is not an open string", root, leafBackend)
		}
	}
}

// TestDeriveBackendsReadsADerivedRoot proves the whole path: a value written at
// a derived root reaches the map backendAllowed reads.
//
// VALIDATES: deriveBackends reads the backend value of a container the schema
// named, not of a container completer.go named.
// PREVENTS: a derivation that finds the right containers and never reads them.
func TestDeriveBackendsReadsADerivedRoot(t *testing.T) {
	completer := probeCompleter(t)

	const root = "test-backend-open/component"
	if !slices.Contains(completer.backendRoots(), root) {
		t.Fatalf("the probe root %q was not derived, so this test cannot read it", root)
	}

	tree := config.NewTree()
	tree.GetOrCreateContainer("test-backend-open").GetOrCreateContainer("component").Set(leafBackend, "probe-backend")

	backends := completer.deriveBackends(tree)
	if backends[root] != "probe-backend" {
		t.Errorf("want backend %q at %q, got %q from %v", "probe-backend", root, backends[root], backends)
	}
}

// findSchemaLeaf resolves a slash path against every loaded conf module and
// answers the named leaf under it, or nil when no module declares it.
func findSchemaLeaf(t *testing.T, loader *yang.Loader, path, leaf string) *gyang.Entry {
	t.Helper()

	for _, module := range loader.ConfModuleNames() {
		entry := loader.GetEntry(module)
		if entry == nil {
			continue
		}
		for _, segment := range config.SplitPath(path) {
			entry = entry.Dir[segment]
			if entry == nil {
				break
			}
		}
		if entry == nil {
			continue
		}
		if child := entry.Dir[leaf]; child != nil {
			return child
		}
	}
	return nil
}
