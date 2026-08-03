package claims

import (
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

const fixtureYANG = `module ze-fixture-conf {
  namespace "urn:ze:fixture";
  prefix zefix;
  container fixture {
    leaf size { type uint32; }
    container inner { leaf depth { type uint32; } }
    container state-only {
      config false;
      leaf counter { type uint32; }
    }
  }
  rpc fixture-rpc { input { leaf arg { type string; } } }
}`

// TestSchemaTreeSkipsNonConfigNodes checks the enumeration side.
//
// The gate's subject is config an operator can write. A state container, an
// RPC, and a notification are not that, and reporting them as unclaimed would
// bury the real findings.
//
// VALIDATES: the config-schema tree carries config data nodes only.
// PREVENTS: the gate reporting read-only state as undelivered config.
func TestSchemaTreeSkipsNonConfigNodes(t *testing.T) {
	l := configyang.NewLoader()
	if err := l.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if err := l.AddModuleFromText("ze-fixture-conf.yang", fixtureYANG); err != nil {
		t.Fatalf("AddModuleFromText: %v", err)
	}
	if err := l.Resolve(); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	root, err := SchemaTree(l)
	if err != nil {
		t.Fatalf("SchemaTree: %v", err)
	}

	fixture := root.Children["fixture"]
	if fixture == nil {
		t.Fatalf("fixture container missing from the config schema tree: %v", childNames(root))
	}
	if fixture.Children["size"] == nil || !fixture.Children["size"].IsLeaf {
		t.Error("config leaf fixture/size is missing or not marked as a leaf")
	}
	if fixture.Children["inner"] == nil {
		t.Error("config container fixture/inner is missing")
	}
	if fixture.Children["state-only"] != nil {
		t.Error("state container fixture/state-only must not be in the config schema tree")
	}
	if root.Children["fixture-rpc"] != nil {
		t.Error("RPC fixture-rpc must not be in the config schema tree")
	}
	if got := fixture.Modules; len(got) != 1 || got[0] != "ze-fixture-conf" {
		t.Errorf("want the contributing module recorded, got %v", got)
	}
}
