// Design: docs/architecture/plugin/feature-gates.md -- committed module selection bounds external effects.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalUnselectedNestedModule retains an independent example
// module with its own import namespace. Its sources cannot enter the parent
// module's certain population or its potential external-access population.
// The existing selected-nested-module refusal covers the opposite boundary.
func TestCommittedExternalUnselectedNestedModule(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, "ambient.go", "package ze\nconst RootLabel = \"root module\"\n")
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("",
				"parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			const moduleDirectory = "examples/plugin/go"
			write(t, fixture.root, moduleDirectory+"/go.mod",
				"module example/acme-monitor\n\ngo 1.26\n\n"+
					"require github.com/ze-software/ze v0.0.0\n"+
					"replace github.com/ze-software/ze => ../../..\n")
			fixture.goFile(t, moduleDirectory+"/main.go",
				"package main\nimport _ \"example/acme-monitor/private\"\nfunc main() {}\n")
			fixture.goFile(t, moduleDirectory+"/private/state.go",
				"package private\nvar Callback func()\n")
			fixture.goFile(t, moduleDirectory+"/vendor/example.invalid/helper/source.go",
				"package helper\nfunc Value() int { return 1 }\n")
			commitInventoryFixture(t, fixture.root)
			snapshot := externalEffectsPositive(t, fixture)
			sources, _ := snapshot.Sources()
			for _, source := range sources {
				if strings.HasPrefix(source.Path, moduleDirectory+"/") {
					t.Fatalf("unselected nested module entered public sources: %s", source.Path)
				}
			}
		})
	}
}

// TestCommittedExternalNestedModuleImportRefusals distinguishes an independent
// nested module from a directory the certain or potential import graph reaches
// under the parent module's identity. Neither graph may cross that boundary.
func TestCommittedExternalNestedModuleImportRefusals(t *testing.T) {
	for _, importer := range []string{"cmd/ze", "pkg/ambient"} {
		t.Run(importer, func(t *testing.T) {
			fixture := externalEffectsFixture(t, "plugin")
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("",
				"parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			const moduleDirectory = "examples/plugin/go"
			write(t, fixture.root, moduleDirectory+"/go.mod",
				"module example/acme-monitor\n\ngo 1.26\n")
			fixture.goFile(t, moduleDirectory+"/example.go", "package example\n")
			packageName := "main"
			if importer == "pkg/ambient" {
				packageName = "ambient"
			}
			fixture.goFile(t, importer+"/nested_import.go",
				"package "+packageName+"\nimport _ \"github.com/ze-software/ze/"+moduleDirectory+"\"\n")
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "nested module")
		})
	}
}
