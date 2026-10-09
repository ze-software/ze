// Design: docs/architecture/module-tiers.md -- imports must preserve plugin ownership.
// Related: tier.go -- the shared, build-tag-independent import scanner.
package archtier

import (
	"go/ast"
	"go/token"
	"path"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// pluginOwner uses the generator's search roots, not a second ownership list.
// Top-level component roots are hosts: their own API stays shared, while a
// registered descendant owns its implementation. Nested standalone roots are
// plugins. In a plugins namespace, the first registration separates grouped
// owners (nlri/ls and nlri/flowspec); descendants stay within that owner.
func pluginOwner(rel string, roots, declared []string) string {
	root := ""
	for _, candidate := range roots {
		if underDir(rel, candidate) && len(candidate) > len(root) {
			root = candidate
		}
	}
	if root == "" {
		return ""
	}
	namespace := path.Base(root) == "plugins"
	if !namespace && root != topSubsystem(root) {
		return root
	}
	if rel == root {
		return ""
	}
	owner := ""
	for _, candidate := range declared {
		if candidate == root {
			continue
		}
		if !underDir(candidate, root) {
			continue
		}
		if !underDir(rel, candidate) {
			continue
		}
		if owner == "" || len(candidate) < len(owner) {
			owner = candidate
		}
	}
	if owner != "" {
		return owner
	}
	if !namespace {
		return ""
	}
	end := len(rel)
	if slash := strings.IndexByte(rel[len(root)+1:], '/'); slash >= 0 {
		end = len(root) + 1 + slash
	}
	return rel[:end]
}

// compositionRegistration judges an edge, never a filename convention. Named
// and dot imports can call implementation and cannot qualify. The generated
// all package MUST remain import-only; cmd/ze's main package wires personalities
// and dispatch alongside executable startup code.
func compositionRegistration(rel string, file *ast.File, spec *ast.ImportSpec) bool {
	if spec.Name == nil {
		return false
	}
	if spec.Name.Name != "_" {
		return false
	}
	if path.Dir(rel) == "cmd/ze" {
		return file.Name.Name == "main"
	}
	if path.Dir(rel) != "internal/component/plugin/all" {
		return false
	}
	if file.Name.Name != "all" {
		return false
	}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			return false
		}
		if general.Tok != token.IMPORT {
			return false
		}
	}
	return true
}

// schemaDirName is the directory name of a schema package: YANG modules and
// their generated registration, no implementation.
const schemaDirName = "yang"

// schemaDependency admits a blank import from one schema package (a `yang`
// directory) of another, which is how a YANG module's import of a module
// another package registers reaches the Go linker (internal/le/yang/glue,
// dependencyImports). A schema package holds modules and their registration
// and no implementation, so the edge pins a schema, never a plugin's code.
// The owner approved schema packages importing each other on 2026-10-09, so a
// plugin's YANG can serve as a template another module imports.
func schemaDependency(rel, imported string, spec *ast.ImportSpec) bool {
	if spec.Name == nil {
		return false
	}
	if spec.Name.Name != "_" {
		return false
	}
	if path.Base(path.Dir(rel)) != schemaDirName {
		return false
	}
	return path.Base(imported) == schemaDirName
}

// pluginNonProduction identifies source which exercises or inspects the product,
// rather than implementing it. Checking every other edge also finds indirect
// dependencies: a shared helper importing a plugin is itself a forbidden edge,
// regardless of how many production callers stand in front of that helper.
func pluginNonProduction(rel string) bool {
	if strings.HasSuffix(rel, "_test.go") {
		return true
	}
	if underDir(rel, "internal/le") {
		return true
	}
	if underDir(rel, "bin") {
		return true
	}
	return hasAnyPrefix(rel, DisableableNonProdPrefixes[:])
}

// pluginOwnershipGate has no baseline. Shared contracts belong outside the owned
// subtree; calling a child package "api" does not make deleting its owner safe.
// This checks dependency edges, not copied feature logic or runtime removeability.
func pluginOwnershipGate(module string, graph sourceGraph) CheckResult {
	roots := PluginDirs()
	result := CheckResult{Name: "plugin-ownership"}
	if len(roots) == 0 {
		result.Code = 2
		result.Diagnosis = "FAIL: plugin search roots contain no ownership boundaries\n"
		return result
	}
	var violations []corePair
	prefix := module + "/"
	for imported, importers := range graph.edges {
		rel := strings.TrimPrefix(imported, prefix)
		owner := pluginOwner(rel, roots, graph.declared)
		if owner == "" {
			continue
		}
		for _, importer := range importers {
			if pluginNonProduction(importer) {
				continue
			}
			if pluginOwner(path.Dir(importer), roots, graph.declared) == owner {
				continue
			}
			if graph.registrations[corePair{File: importer, Package: imported}] {
				continue
			}
			violations = append(violations, corePair{File: importer, Package: rel})
		}
	}
	if len(violations) == 0 {
		result.Page = "plugin ownership OK\n"
		return result
	}
	sortPairs(violations)
	var diagnosis textbuf.Buffer
	diagnosis.Str("FAIL: production imports cross plugin ownership boundaries:\n")
	for _, violation := range violations {
		diagnosis.Str("  ").Str(violation.File).Str(" imports ").Str(violation.Package).Byte('\n')
	}
	diagnosis.Str("  Depend on shared contracts or component infrastructure, not another plugin's implementation (ai/rules/plugins.md).\n")
	result.Code = 2
	result.Diagnosis = diagnosis.String()
	return result
}
