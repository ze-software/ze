package config

import (
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSchemaNodeModuleNamesTheAugmentingModule proves SchemaNodeModule
// answers the module that defines a node, the augmenting one for a node
// another module adds, and refuses a path the schema does not hold.
// Method: a base node of ze-bgp-conf, an unknown word under it, and the
// module name YANGModuleName reads from a module's text.
func TestSchemaNodeModuleNamesTheAugmentingModule(t *testing.T) {
	module, err := SchemaNodeModule([]string{"bgp", "peer", "session"})
	if err != nil {
		t.Fatal(err)
	}
	if module != "ze-bgp-conf" {
		t.Fatalf("bgp peer session is defined by %q, want ze-bgp-conf", module)
	}
	if _, err := SchemaNodeModule([]string{"bgp", "peer", "no-such-node"}); err == nil ||
		!strings.Contains(err.Error(), "bgp peer no-such-node") {
		t.Fatalf("err = %v, want it to name the missing path", err)
	}
	name, err := YANGModuleName("module ze-test-origin { namespace \"urn:t\"; prefix t; }")
	if err != nil {
		t.Fatal(err)
	}
	if name != "ze-test-origin" {
		t.Fatalf("module name = %q", name)
	}
}

// insertionTestYANG augments a peer with one container, so a test can
// condition an insertion on config a module other than ze-bgp-conf owns.
const insertionTestYANG = `module ze-test-insertion-conf {
    namespace "urn:ze:test:insertion";
    prefix ti;
    import ze-bgp-conf { prefix bgp; }
    augment "/bgp:bgp/bgp:peer" {
        container insertion-test {
            leaf mode { type string; }
        }
    }
}`

// TestFilterInsertionPathsJudged proves the A-10 rule and the ownership rule:
// a path the schema does not hold is refused for every plugin; a compiled-in
// plugin's path must start at a node its own module adds; a third-party
// plugin (no module text) may name any compiled path.
// Method: a test module augmenting bgp peer, judged as owner and as stranger.
func TestFilterInsertionPathsJudged(t *testing.T) {
	restore := configyang.RegisterModuleForTest("ze-test-insertion-conf.yang", insertionTestYANG)
	t.Cleanup(restore)
	own := []rpc.FilterInsertion{{Filter: "p:f", Config: []string{"insertion-test", "mode"}}}
	if err := CheckFilterInsertionPaths("p", insertionTestYANG, own); err != nil {
		t.Fatalf("own path refused: %v", err)
	}
	if err := CheckFilterInsertionPaths("p", "", own); err != nil {
		t.Fatalf("third-party compiled path refused: %v", err)
	}
	foreign := []rpc.FilterInsertion{{Filter: "p:f", Config: []string{"session"}}}
	err := CheckFilterInsertionPaths("p", insertionTestYANG, foreign)
	if err == nil || !strings.Contains(err.Error(), "ze-bgp-conf defines, not the plugin's own module ze-test-insertion-conf") {
		t.Fatalf("foreign path err = %v, want the ownership refusal", err)
	}
	if err := CheckFilterInsertionPaths("p", "", foreign); err != nil {
		t.Fatalf("third-party foreign compiled path refused: %v", err)
	}
	unknown := []rpc.FilterInsertion{{Filter: "p:f", Config: []string{"insertion-test", "nope"}}}
	for _, owner := range []string{insertionTestYANG, ""} {
		err := CheckFilterInsertionPaths("p", owner, unknown)
		if err == nil || !strings.Contains(err.Error(), "bgp peer insertion-test nope") {
			t.Fatalf("unknown path err = %v, want it to name the path", err)
		}
	}
	module, err := SchemaNodeModule([]string{"bgp", "peer", "insertion-test"})
	if err != nil || module != "ze-test-insertion-conf" {
		t.Fatalf("augmented node module = %q, %v; want ze-test-insertion-conf", module, err)
	}
}
