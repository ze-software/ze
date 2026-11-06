// Design: docs/architecture/plugin/feature-gates.md -- source facts and descriptor output ownership bound repeated work.
package repoinventory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestExternalReflectionOutputUnionAllocationBudget compares the same reflected
// method at two anonymous structural depths using the same escaping output sink.
func TestExternalReflectionOutputUnionAllocationBudget(t *testing.T) {
	const directory = "internal/reflectionoutputcost"
	const filename = directory + "/proof.go"
	deep := strings.Repeat("struct { Value ", 12) + "root" + strings.Repeat(" }", 12)
	effects := typeIdentitySelectedSource(t, directory, map[string]string{filename: "package reflectionoutputcost\ntype root int\nfunc (root) Touch() {}\nvar Shallow struct { Value root }\nvar Deep " + deep + "\n"})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	query := func(name string) sourceExpr {
		declaration, found := file.Scope.Lookup(name).Decl.(*ast.ValueSpec)
		if !found {
			t.Fatal("selected allocation fixture lost its declared type")
		}
		return sourceExpr{value: declaration.Type, file: file, pkg: pkg}
	}
	shallow := query("Shallow")
	deeper := query("Deep")
	sink := make(chan externalReflection, 1)
	var retained externalReflection
	measure := func(source sourceExpr) float64 {
		return testing.AllocsPerRun(50, func() {
			value := effects.reflectedValue(source, nil, 0)
			sink <- value
			retained = <-sink
		})
	}
	shallowAllocations := measure(shallow)
	if retained.unknown != "" {
		t.Fatal(retained.unknown)
	}
	if len(retained.methods) != 1 {
		t.Fatal("shallow control lost its reflected method")
	}
	shallowResult := retained
	deepAllocations := measure(deeper)
	if retained.unknown != "" {
		t.Fatal(retained.unknown)
	}
	if len(retained.methods) != 1 {
		t.Fatal("deep source lost its reflected method")
	}
	for method := range retained.methods {
		delete(retained.methods, method)
	}
	if len(shallowResult.methods) != 1 {
		t.Fatal("independent descriptor queries share their output map")
	}
	t.Logf("descriptor allocations: shallow=%g deep=%g", shallowAllocations, deepAllocations)
	if deepAllocations > shallowAllocations {
		t.Fatalf("descriptor depth recopied output maps: shallow=%g deep=%g", shallowAllocations, deepAllocations)
	}
}

// TestExternalPackageBindingLookupCompatibility checks existing production query
// results and exact identities; the root profile supplies the performance RED.
func TestExternalPackageBindingLookupCompatibility(t *testing.T) {
	const directory = "internal/reflectionbindingfacts"
	const filename = directory + "/first.go"
	sources := map[string]string{
		filename:                            "package reflectionbindingfacts\nimport _ \"github.com/ze-software/ze/internal/reflectionempty\"\ntype T string\nvar Value T\nvar int = 1\nfunc F() {}\n",
		directory + "/second.go":            "package reflectionbindingfacts\ntype Other string\n",
		"internal/reflectionempty/proof.go": "package reflectionempty\n",
	}
	effects := typeIdentitySelectedSource(t, directory, sources)
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	walk := externalTypeWalk{effects: effects}
	for _, name := range []string{"T", "Value", "int", "F"} {
		for range 16 {
			source := sourceExpr{value: ast.NewIdent(name), file: file, pkg: pkg}
			definition, owner, reason := walk.definition(source)
			object := file.Scope.Lookup(name)
			if object.Kind != ast.Typ {
				if reason != "retained declaration is not a type" {
					t.Fatalf("non-type binding %s changed its refusal: %q", name, reason)
				}
				continue
			}
			if reason != "" {
				t.Fatal(reason)
			}
			if definition != object.Decl {
				t.Fatalf("binding %s lost its exact parser declaration", name)
			}
			if owner.file != file {
				t.Fatal("binding lost its exact declaration file")
			}
		}
	}
	shadow := sourceExpr{value: ast.NewIdent("int"), file: file, pkg: pkg}
	if value := walk.value(shadow, nil, true, 0); value.unknown != "retained declaration is not a type" {
		t.Fatalf("ordinary type walk borrowed the shadowed builtin: %+v", value)
	}
	reflection := externalReflectionWalk{effects: effects}
	if value := reflection.value(shadow, nil, true, false, 0); value.unknown != "retained declaration is not a type" {
		t.Fatalf("descriptor walk borrowed the shadowed builtin: %+v", value)
	}
	if _, reason := reflection.interfaceRoot(shadow, nil, 0); reason != "retained declaration is not a type" {
		t.Fatalf("value-root query borrowed the shadowed builtin: %q", reason)
	}
	if _, _, reason := walk.definition(sourceExpr{value: ast.NewIdent("missing"), file: file, pkg: pkg}); reason != "retained type declaration ownership is unresolved" {
		t.Fatalf("missing binding changed its refusal: %q", reason)
	}
	empty := effects.source.snapshot.packages["internal/reflectionempty"]
	if _, _, reason := walk.definition(sourceExpr{value: ast.NewIdent("T"), pkg: empty}); reason != "retained type declaration ownership is unresolved" {
		t.Fatalf("empty selected package borrowed a type: %q", reason)
	}
	if _, _, reason := walk.definition(sourceExpr{value: ast.NewIdent("T")}); reason != "retained type package is unavailable" {
		t.Fatalf("nil package changed its existing refusal: %q", reason)
	}
	otherEffects := typeIdentitySelectedSource(t, directory, sources)
	otherPackage := otherEffects.source.snapshot.packages[directory]
	otherFile := typeIdentitySelectedFile(t, otherPackage, filename)
	if _, _, reason := walk.definition(sourceExpr{value: ast.NewIdent("T"), file: otherFile, pkg: otherPackage}); reason != "retained type defining file is unavailable" {
		t.Fatalf("unindexed package borrowed another snapshot's same-path fact: %q", reason)
	}
	const duplicate = "internal/reflectionduplicatefacts"
	duplicated := typeIdentitySelectedSource(t, duplicate, map[string]string{
		duplicate + "/first.go":  "package reflectionduplicatefacts\ntype T int\n",
		duplicate + "/second.go": "package reflectionduplicatefacts\ntype T string\n",
	})
	duplicatePackage := duplicated.source.snapshot.packages[duplicate]
	duplicateWalk := externalTypeWalk{effects: duplicated}
	if _, _, reason := duplicateWalk.definition(sourceExpr{value: ast.NewIdent("T"), pkg: duplicatePackage}); reason != "retained type package binding is ambiguous: T" {
		t.Fatalf("duplicate package lookup changed its refusal: %q", reason)
	}
	duplicateFile := typeIdentitySelectedFile(t, duplicatePackage, duplicate+"/first.go")
	name := ast.NewIdent("T")
	name.Obj = duplicateFile.Scope.Lookup("T")
	definition, owner, reason := duplicateWalk.definition(sourceExpr{value: name, file: duplicateFile, pkg: duplicatePackage})
	if reason != "" {
		t.Fatal(reason)
	}
	if definition != name.Obj.Decl {
		t.Fatal("package ambiguity replaced the exact lexical parser object")
	}
	if owner.file != duplicateFile {
		t.Fatal("exact lexical parser object lost its declaration owner")
	}
}

// TestExternalReflectionQueryOutputIndependence reuses one walker across known,
// primitive and exhausted queries, retaining each output independently.
func TestExternalReflectionQueryOutputIndependence(t *testing.T) {
	const directory = "internal/reflectionqueryoutputs"
	const filename = directory + "/proof.go"
	selected := typeIdentitySelectedSource(t, directory, map[string]string{
		"go.mod":                                     "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt":                          "",
		registrationPackage + "/register.go":         "package registry\nimport _ \"fmt\"\nfunc Register(label string) {}\n",
		"internal/component/config/yang/register.go": "package yang\nimport _ \"fmt\"\n",
		filename: `package reflectionqueryoutputs
import (
	"fmt"
	_ "github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/component/config/yang"
)
type First int
func (First) One() {}
type Second string
func (Second) Two() {}
type Remote func(fmt.State)
type payload struct { Leaf First; Callback Remote }
type pair[A, B any] struct { Left A; Right B }
func unbound[U any]() { var value pair[payload, U]; _ = value }
`,
	})
	source, err := newExternalSource(selected.source.snapshot)
	if err != nil {
		t.Fatal(err)
	}
	effects := newExternalEffects(source)
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	query := func(name string) sourceExpr { return sourceExpr{value: ast.NewIdent(name), file: file, pkg: pkg} }
	walk := externalReflectionWalk{effects: effects}
	first := walk.value(query("First"), nil, true, false, 0)
	primitive := walk.value(query("string"), nil, true, false, 0)
	if primitive.unknown != "" {
		t.Fatal(primitive.unknown)
	}
	if len(primitive.methods) != 0 {
		t.Fatal("primitive query inherited an earlier method")
	}
	second := walk.value(query("Second"), nil, true, false, 0)
	if len(first.methods) != 1 {
		t.Fatal("later query replaced the first output")
	}
	if len(second.methods) != 1 {
		t.Fatal("second query borrowed the first method set")
	}
	remote := walk.value(query("Remote"), nil, true, false, 0)
	if remote.unknown != "" {
		t.Fatal(remote.unknown)
	}
	if !remote.origins["fmt"] {
		t.Fatal("source-certified origin query lost its exact identity")
	}
	primitive = walk.value(query("string"), nil, true, false, 0)
	if len(primitive.origins) != 0 {
		t.Fatal("primitive query inherited an earlier origin")
	}
	if primitive.unknown != "" {
		t.Fatal("primitive query inherited earlier uncertainty")
	}
	if len(primitive.methods) != 0 {
		t.Fatal("primitive query inherited earlier methods")
	}
	for method := range first.methods {
		delete(first.methods, method)
	}
	if len(second.methods) != 1 {
		t.Fatal("separate query outputs share method maps")
	}
	if exhausted := walk.value(query("string"), nil, true, false, 128); exhausted.unknown == "" {
		t.Fatal("exhausted query lost its explicit uncertainty")
	}
	third := walk.value(query("First"), nil, true, false, 0)
	if third.unknown != "" {
		t.Fatal("earlier exhausted query contaminated a valid output")
	}
	if len(third.methods) != 1 {
		t.Fatal("valid query after exhaustion lost its own method")
	}
	t.Run("late-unbound-argument", func(t *testing.T) {
		function := pkg.functions["unbound"]
		statement, found := function.decl.Body.List[0].(*ast.DeclStmt)
		if !found {
			t.Fatal("selected unbound fixture lost its declaration")
		}
		group, found := statement.Decl.(*ast.GenDecl)
		if !found {
			t.Fatal("selected unbound fixture lost its declaration group")
		}
		variable, found := group.Specs[0].(*ast.ValueSpec)
		if !found {
			t.Fatal("selected unbound fixture lost its declared type")
		}
		source := sourceExpr{value: variable.Type, file: file, pkg: pkg}
		failed := walk.value(source, effects.typeIdentities.lexicalScope(source), true, false, 0)
		if failed.unknown != "reflected generic type argument ownership is unresolved" {
			t.Fatalf("late argument failure changed its exact cause: %+v", failed)
		}
		if len(failed.methods) != 0 {
			t.Fatal("late argument failure retained discarded earlier methods")
		}
		if len(failed.origins) != 0 {
			t.Fatal("late argument failure retained a discarded earlier origin")
		}
		valid := walk.value(query("Second"), nil, true, false, 0)
		if valid.unknown != "" {
			t.Fatal("late argument failure contaminated a later query")
		}
		if len(valid.methods) != 1 {
			t.Fatal("later query lost its independent method set")
		}
	})
}

// TestExternalPackageBindingStructuralCompatibility supplies parser-scope edge
// inputs before index capture. It proves the raw lookup contract, not compiler
// validity or inventory publication, and creates no capability authority facts.
func TestExternalPackageBindingStructuralCompatibility(t *testing.T) {
	for _, scenario := range []struct {
		name                                 string
		distinct, repeat, nilFirst, nilScope bool
	}{
		{name: "scope-map-key"},
		{name: "identical-object-duplicate", repeat: true},
		{name: "distinct-object-ambiguity", distinct: true},
		{name: "sticky-ambiguity-after-repeat", distinct: true, repeat: true},
		{name: "nil-object-does-not-shadow", nilFirst: true, repeat: true},
		{name: "nil-scope-is-empty", nilScope: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			positions := token.NewFileSet()
			parse := func(name, text string) *ast.File {
				file, err := parser.ParseFile(positions, name, text, 0)
				if err != nil {
					t.Fatal(err)
				}
				return file
			}
			first := parse("first.go", "package structural\ntype Original string\nvar Shadow = 1\n")
			second := parse("second.go", "package structural\ntype Alternate string\n")
			third := parse("third.go", "package structural\n")
			original := first.Scope.Lookup("Original")
			shadow := first.Scope.Lookup("Shadow")
			alternate := second.Scope.Lookup("Alternate")
			// Complete all raw input construction before snapshot/index capture.
			first.Scope.Objects = map[string]*ast.Object{"Alias": original, "int": shadow, "Nil": nil}
			second.Scope.Objects = map[string]*ast.Object{"Nil": nil}
			if scenario.nilFirst {
				first.Scope.Objects["Alias"] = nil
			}
			if scenario.distinct {
				second.Scope.Objects["Alias"] = alternate
			}
			if scenario.repeat {
				third.Scope.Objects["Alias"] = original
			}
			if scenario.nilScope {
				second.Scope = nil
			}
			pkg := &sourcePackage{dir: "internal/structuralbinding", files: []SourceFile{{Path: "first.go", File: first}, {Path: "second.go", File: second}, {Path: "third.go", File: third}}}
			snapshot := &Snapshot{packages: map[string]*sourcePackage{pkg.dir: pkg}, fset: positions}
			effects := newExternalEffects(&externalSource{snapshot: snapshot})
			walk := externalTypeWalk{effects: effects}
			query := func(name string) sourceExpr { return sourceExpr{value: ast.NewIdent(name), file: first, pkg: pkg} }
			definition, owner, reason := walk.definition(query("Alias"))
			if scenario.distinct {
				if reason != "retained type package binding is ambiguous: Alias" {
					t.Fatalf("distinct objects lost sticky ambiguity: %q", reason)
				}
			} else {
				if reason != "" {
					t.Fatal(reason)
				}
				if definition != original.Decl {
					t.Fatal("lookup used object.Name instead of the scope map key")
				}
				if owner.file != first {
					t.Fatal("identical object lost its captured declaration owner")
				}
			}
			if _, _, reason := walk.definition(query("Original")); reason != "retained type declaration ownership is unresolved" {
				t.Fatalf("object.Name invented an absent map key: %q", reason)
			}
			if value := walk.value(query("int"), nil, true, 0); value.unknown != "retained declaration is not a type" {
				t.Fatalf("raw non-type map key borrowed a builtin: %+v", value)
			}
		})
	}
}
