// Design: docs/architecture/plugin/feature-gates.md -- generated-table proof is required and remains symbol-scoped

package archenumeration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/plugin/gatedtable"
)

// TestGeneratedPluginTablesRequireCurrentProof rejects stale, missing and
// absent-file proof before a generated table receives declaration status.
func TestGeneratedPluginTablesRequireCurrentProof(t *testing.T) {
	for _, proof := range []gatedtable.CheckReport{
		{File: "internal/component/plugin/registry/gated_plugins_gen.go", Reason: "is stale"},
		{File: "internal/component/plugin/registry/gated_plugins_gen.go", Reason: "is missing"},
		{},
	} {
		if corpora, err := verifiedPluginCorpora(nil, proof); err == nil {
			t.Fatalf("unverified report %+v admitted corpora %v", proof, corpora)
		}
	}
}

// TestGeneratedPluginProofExemptsOnlyItsThreeTables checks the positive
// boundary and an extra handwritten copy in the same file. Current-table
// proof cannot exempt that fourth symbol or mutate the input corpus.
func TestGeneratedPluginProofExemptsOnlyItsThreeTables(t *testing.T) {
	root := t.TempDir()
	file := "internal/component/plugin/registry/gated_plugins_gen.go"
	path := filepath.Join(root, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	const source = `package registry
var gatedPluginTags = []string{"alpha", "beta"}
var gatedConfigRoots = []string{"alpha", "beta"}
var gatedDataPlaneWriters = []string{"alpha", "beta"}
var manualCopy = []string{"alpha", "beta"}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	original := []Corpus{{Name: "plugin names", Groups: []Group{{Name: "plugin names", Keys: []string{"alpha", "beta"}}}}}
	corpora, err := verifiedPluginCorpora(original, gatedtable.CheckReport{File: file})
	if err != nil {
		t.Fatal(err)
	}
	if len(original[0].Declarations) != 0 {
		t.Fatal("proof mutated the caller's corpus")
	}
	found, err := Check(root, []string{"internal"}, corpora, 0)
	if err != nil {
		t.Fatal(err)
	}
	wantOneCopy(t, found, "plugin names", "manualCopy")
}
