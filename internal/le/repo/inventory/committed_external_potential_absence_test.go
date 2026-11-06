// Design: docs/architecture/plugin/feature-gates.md -- closed parent namespace proves absent imports.
package repoinventory

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"

	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

const externalAbsentPotentialTarget = "unrelated/missing/target"
const externalAbsentPotentialImporter = "arbitrary/absence/importer"

// TestCommittedExternalAbsentPotentialDependency keeps the exact report while
// excluding a whole potential package whose closed parent import has no source.
func TestCommittedExternalAbsentPotentialDependency(t *testing.T) {
	for _, source := range []string{"absent", "windows", "test", "cgo"} {
		t.Run(source, func(t *testing.T) {
			fixture := externalPotentialFixture(t)
			switch source {
			case "absent":
				// The complete retained namespace contains no target directory.
			case "windows":
				fixture.goFile(t, externalAbsentPotentialTarget+"/source_windows.go", "//go:build windows\n\npackage absent\n")
			case "test":
				fixture.goFile(t, externalAbsentPotentialTarget+"/source_test.go", "package absent\n")
			case "cgo":
				fixture.goFile(t, externalAbsentPotentialTarget+"/source.go", "package absent\nimport \"C\"\n")
			}
			fixture.goFile(t, externalAbsentPotentialImporter+"/source.go", externalPotentialInstaller("importer", "_ \"github.com/ze-software/ze/"+externalAbsentPotentialTarget+"\""))
			commitInventoryFixture(t, fixture.root)
			snapshot := externalEffectsPositive(t, fixture)
			private, err := newExternalSource(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if _, included := private.snapshot.packages[externalAbsentPotentialImporter]; included {
				t.Fatal("importer of absent parent package entered the private graph")
			}
		})
	}
}

// TestCommittedExternalAbsentPotentialDependencyClosure excludes transitive
// importers, including a dormant assignment that would expose an installer.
func TestCommittedExternalAbsentPotentialDependencyClosure(t *testing.T) {
	fixture := externalPotentialFixture(t)
	fixture.goFile(t, externalAbsentPotentialImporter+"/source.go", "package importer\nimport _ \"github.com/ze-software/ze/"+externalAbsentPotentialTarget+"\"\n")
	const dependent = "another/absence/dependent"
	fixture.goFile(t, dependent+"/source.go", externalPotentialInstaller("dependent", "_ \"github.com/ze-software/ze/"+externalAbsentPotentialImporter+"\""))
	commitInventoryFixture(t, fixture.root)
	snapshot := externalEffectsPositive(t, fixture)
	private, err := newExternalSource(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{externalAbsentPotentialImporter, dependent} {
		if _, included := private.snapshot.packages[directory]; included {
			t.Fatalf("dependent of absent parent package entered private graph: %s", directory)
		}
	}
}

// TestCommittedExternalAbsentCertainDependency preserves ordinary source
// refusal when initialization imports the absent package.
func TestCommittedExternalAbsentCertainDependency(t *testing.T) {
	fixture := externalPotentialFixture(t)
	fixture.goFile(t, "cmd/ze/absent_import.go", "package main\nimport _ \"github.com/ze-software/ze/"+externalAbsentPotentialTarget+"\"\n")
	commitInventoryFixture(t, fixture.root)
	inv, reportErr := Collect(fixture.root)
	plugins, pluginErr := Plugins(fixture.root)
	if reportErr == nil {
		t.Fatalf("certain absent package published report: %+v", inv)
	}
	if pluginErr == nil {
		t.Fatalf("certain absent package published plugins: %+v", plugins)
	}
	if !reflect.DeepEqual(inv, Inventory{}) {
		t.Fatalf("certain absence refusal leaked report: %+v", inv)
	}
	if plugins != nil {
		t.Fatalf("certain absence refusal leaked plugins: %+v", plugins)
	}
	for _, err := range []error{reportErr, pluginErr} {
		if !strings.Contains(err.Error(), "imports "+externalAbsentPotentialTarget+" but selects no Go source") {
			t.Fatalf("certain absent source cause absent: %v", err)
		}
	}
}

// TestCommittedExternalParentNamespaceAmbiguity refuses overlapping namespace
// authorities before either syntax or known-absence exclusion can hide them.
func TestCommittedExternalParentNamespaceAmbiguity(t *testing.T) {
	for _, authority := range []string{"child requirement", "ancestor requirement", "child vendor header", "root vendor header", "ancestor vendor header", "vendor package"} {
		for _, source := range []string{"syntax", "absence"} {
			t.Run(authority+"/"+source, func(t *testing.T) {
				fixture := externalPotentialFixture(t)
				body := "package importer\nimport _ \"github.com/ze-software/ze/" + externalAbsentPotentialTarget + "\"\n"
				if source == "syntax" {
					body = externalInvalidPotentialSource
				}
				fixture.goFile(t, externalAbsentPotentialImporter+"/source.go", body)
				const child = "github.com/ze-software/ze/unrelated"
				switch authority {
				case "child requirement":
					write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n\nrequire "+child+" v1.0.0\n")
				case "ancestor requirement":
					write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n\nrequire github.com/ze-software v1.0.0\n")
				case "child vendor header":
					write(t, fixture.root, "vendor/modules.txt", "# "+child+" v1.0.0\n## explicit; go 1.26\n")
				case "root vendor header":
					write(t, fixture.root, "vendor/modules.txt", "# github.com/ze-software/ze v1.0.0\n## explicit; go 1.26\n")
				case "ancestor vendor header":
					write(t, fixture.root, "vendor/modules.txt", "# github.com/ze-software v1.0.0\n## explicit; go 1.26\n")
				case "vendor package":
					write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n\nrequire github.com/ze-software v1.0.0\n")
					write(t, fixture.root, "vendor/modules.txt", "# github.com/ze-software v1.0.0\n## explicit; go 1.26\ngithub.com/ze-software/ze/"+externalAbsentPotentialTarget+"\n")
				}
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "parent namespace")
			})
		}
	}
}

// TestCommittedExternalAbsentPotentialFutureLanguage cannot certify known
// absence for an importing source language newer than the analyzer authority.
func TestCommittedExternalAbsentPotentialFutureLanguage(t *testing.T) {
	fixture := externalPotentialFixture(t)
	write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.999\n")
	fixture.goFile(t, externalAbsentPotentialImporter+"/source.go", "package importer\nimport _ \"github.com/ze-software/ze/"+externalAbsentPotentialTarget+"\"\n")
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "language")
}

// TestCommittedExternalMissingExternalSourceRefuses preserves uncertainty for
// an unavailable dependency outside the closed parent namespace.
func TestCommittedExternalMissingExternalSourceRefuses(t *testing.T) {
	for _, source := range []string{"valid", "syntax"} {
		t.Run(source, func(t *testing.T) {
			fixture := externalPotentialFixture(t)
			body := "package importer\nimport _ \"example.invalid/unavailable\"\n"
			if source == "syntax" {
				body += "func broken( {\n"
			}
			fixture.goFile(t, externalAbsentPotentialImporter+"/source.go", body)
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "identity")
		})
	}
}

// TestCommittedExternalExactRootProvider keeps an importer when the exact root
// module import has selected source at '.', independent of ordinary recursive
// selection's separate support for that import spelling.
func TestCommittedExternalExactRootProvider(t *testing.T) {
	fixture := externalPotentialFixture(t)
	fixture.goFile(t, "root.go", "package ze\nconst Label = \"root provider\"\n")
	fixture.goFile(t, externalAbsentPotentialImporter+"/source.go", "package importer\nimport _ \"github.com/ze-software/ze\"\n")
	commitInventoryFixture(t, fixture.root)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := modfile.Parse("go.mod", snapshot.blobs["go.mod"], nil)
	if err != nil {
		t.Fatal(err)
	}
	gates, err := repofeaturetags.ParseGates("feature-gates.txt", snapshot.blobs["feature-gates.txt"])
	if err != nil {
		t.Fatal(err)
	}
	context := linuxContext(gates, snapshot.blobs)
	if directory := externalPotentialImportDirectory("github.com/ze-software/ze"); directory != "." {
		t.Fatalf("exact root import provider = %q, want '.'", directory)
	}
	directories, err := externalPotentialDirectories(snapshot, &context, manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{".", externalAbsentPotentialImporter} {
		if _, included := directories[directory]; !included {
			t.Fatalf("available root provider or its importer excluded: %s", directory)
		}
	}
}

// TestCommittedExternalFutureLanguageDependentRefuses cannot use current
// import grammar to exclude a future-language dependent of invalid source.
func TestCommittedExternalFutureLanguageDependentRefuses(t *testing.T) {
	fixture := externalPotentialFixture(t)
	write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n\nrequire example.invalid/dependent v1.0.0\n")
	write(t, fixture.root, "vendor/modules.txt", "# example.invalid/dependent v1.0.0\n## explicit; go 1.999\nexample.invalid/dependent\n")
	fixture.goFile(t, externalInvalidPotentialDir+"/broken.go", externalInvalidPotentialSource)
	fixture.goFile(t, "vendor/example.invalid/dependent/source.go", "package dependent\nimport _ \"github.com/ze-software/ze/"+externalInvalidPotentialDir+"\"\n")
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "language")
}
