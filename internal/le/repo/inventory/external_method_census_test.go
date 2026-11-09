// Design: docs/architecture/plugin/feature-gates.md -- ordered source census queries preserve method authority and refusals.
// These compatibility controls preserve ordered method source queries.
package repoinventory

import (
	"go/ast"
	"strconv"
	"strings"
	"testing"
)

// TestExternalMethodCensusRefusalOrder keeps visibility and target-specific
// refusals in the original receiver order. Later methods still contribute.
func TestExternalMethodCensusRefusalOrder(t *testing.T) {
	const unresolved = "retained type declaration ownership is unresolved"
	for _, scenario := range []struct {
		name           string
		body           string
		ordinary       string
		reflected      string
		ordinaryNames  []string
		reflectedNames []string
	}{
		{
			name:           "private unresolved receiver",
			body:           "type Root int\nfunc (Missing) hidden() {}\nfunc (Root) Act() {}\n",
			ordinary:       unresolved,
			ordinaryNames:  []string{"Act"},
			reflectedNames: []string{"Act"},
		},
		{
			name:           "bodyless before unrelated error",
			body:           "type Root int\nfunc (Root) Opaque()\nfunc (Missing) Other() {}\nfunc (Root) Later() {}\n",
			ordinary:       "bodyless retained method capability execution is unsupported",
			reflected:      "bodyless reflected method execution is unsupported",
			ordinaryNames:  []string{"Later"},
			reflectedNames: []string{"Later"},
		},
		{
			name:           "unrelated error before bodyless",
			body:           "type Root int\nfunc (Missing) Other() {}\nfunc (Root) Opaque()\nfunc (Root) Later() {}\n",
			ordinary:       unresolved,
			reflected:      unresolved,
			ordinaryNames:  []string{"Later"},
			reflectedNames: []string{"Later"},
		},
		{
			// A generic method executes as ordinary capability. The census
			// reads Root uninstantiated, so the reflected signature has no
			// argument to bind its T to.
			name:          "generic before unrelated error",
			body:          "type Root[T any] struct{}\nfunc (Root[T]) Generic() {}\nfunc (Missing) Other() {}\n",
			ordinary:      unresolved,
			reflected:     "reflected generic type argument count is unresolved",
			ordinaryNames: []string{"Generic"},
		},
		{
			name:          "unrelated error before generic",
			body:          "type Root[T any] struct{}\nfunc (Missing) Other() {}\nfunc (Root[T]) Generic() {}\n",
			ordinary:      unresolved,
			reflected:     unresolved,
			ordinaryNames: []string{"Generic"},
		},
		{
			name:           "unrelated bodyless and generic remain inert",
			body:           "type Root int\ntype Other[T any] struct{}\nfunc (Other[T]) Generic() {}\ntype Bodyless int\nfunc (Bodyless) Opaque()\nfunc (Root) Act() {}\n",
			ordinaryNames:  []string{"Act"},
			reflectedNames: []string{"Act"},
		},
		{
			name:           "multiple receiver fields",
			body:           "type Root int\ntype Other int\nfunc (one Root, two Other) Broken() {}\nfunc (Root) Act() {}\n",
			ordinary:       "retained receiver shape is unresolved",
			reflected:      "reflected receiver shape is unresolved",
			ordinaryNames:  []string{"Act"},
			reflectedNames: []string{"Act"},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			effects, definition, owner := methodCensusFixture(t, scenario.body)
			for range 2 {
				methodCensusCheck(t, effects, definition, owner, scenario.ordinary, scenario.reflected, scenario.ordinaryNames, scenario.reflectedNames)
			}
		})
	}
}

// TestExternalMethodCensusSignatureOrder proves a matching method's signature
// can precede an unrelated receiver failure. Reordering all errors first fails.
func TestExternalMethodCensusSignatureOrder(t *testing.T) {
	const directory = "internal/methodcensussignatures"
	const first = "package methodcensussignatures\ntype Root int\nfunc (Root) First() *Missing { return nil }\n"
	const later = "package methodcensussignatures\ntype A = B\ntype B = A\nfunc (A) Other() {}\nfunc (Root) Later() {}\n"
	const cycle = "cyclic retained receiver alias capability"
	for _, scenario := range []struct {
		name      string
		a         string
		b         string
		reflected string
	}{
		{name: "signature before global refusal", a: first, b: later, reflected: "reflected type ownership is unresolved: Missing"},
		{name: "global refusal before signature", a: later, b: first, reflected: cycle},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			effects := typeIdentitySelectedSource(t, directory, map[string]string{
				directory + "/a.go": scenario.a,
				directory + "/b.go": scenario.b,
			})
			pkg := effects.source.snapshot.packages[directory]
			definition, owner := methodCensusNamedType(t, effects, pkg, "Root")
			for range 2 {
				methodCensusCheck(t, effects, definition, owner, cycle, scenario.reflected, []string{"First", "Later"}, []string{"First", "Later"})
			}
		})
	}
}

// TestExternalMethodCensusExactIdentity keeps target and package pointers,
// lexical same-spelled declarations and separate readers distinct.
func TestExternalMethodCensusExactIdentity(t *testing.T) {
	effects, root, owner := methodCensusFixture(t, "type Root int\nfunc (Root) Alpha() {}\ntype Other int\nfunc (Other) Beta() {}\nfunc Local() { type Root int; _ = Root(0) }\n")
	other, otherOwner := methodCensusNamedType(t, effects, owner.pkg, "Other")
	methodCensusCheck(t, effects, root, owner, "", "", []string{"Alpha"}, []string{"Alpha"})
	methodCensusCheck(t, effects, other, otherOwner, "", "", []string{"Beta"}, []string{"Beta"})
	methodCensusCheck(t, effects, root, owner, "", "", []string{"Alpha"}, []string{"Alpha"})
	var local *ast.TypeSpec
	ast.Inspect(owner.file, func(node ast.Node) bool {
		declaration, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		if declaration == root {
			return true
		}
		if declaration.Name.Name == "Root" {
			local = declaration
		}
		return true
	})
	if local == nil {
		t.Fatal("authored identity fixture lost its local Root")
	}
	walk := externalTypeWalk{effects: effects}
	localDefinition, localOwner, reason := walk.definition(sourceExpr{value: local.Name, file: owner.file, pkg: owner.pkg})
	if reason != "" {
		t.Fatal(reason)
	}
	if localDefinition != local {
		t.Fatal("local type borrowed the package declaration")
	}
	methodCensusCheck(t, effects, localDefinition, localOwner, "", "", nil, nil)
	unindexed := *owner.pkg
	methodCensusCheck(t, effects, root, sourceExpr{pkg: &unindexed}, "", "", nil, nil)
	second, secondRoot, secondOwner := methodCensusFixture(t, "type Root int\nfunc (Root) Second() {}\n")
	methodCensusCheck(t, second, secondRoot, secondOwner, "", "", []string{"Second"}, []string{"Second"})
	methodCensusCheck(t, effects, secondRoot, owner, "", "", nil, nil)
	methodCensusCheck(t, effects, root, owner, "", "", []string{"Alpha"}, []string{"Alpha"})
}

// TestExternalMethodCensusOwnsOutputs prevents source-fact reuse from sharing
// mutable callable, field or reflected-method outputs between visits.
func TestExternalMethodCensusOwnsOutputs(t *testing.T) {
	effects, definition, owner := methodCensusFixture(t, "type Root int\nfunc (Root) Act() {}\n")
	ordinary, reflected := methodCensusCheck(t, effects, definition, owner, "", "", []string{"Act"}, []string{"Act"})
	for node := range ordinary.callables {
		delete(ordinary.callables, node)
	}
	// join allocates the field table only when a value carries fields, so the
	// injection makes its own before writing the prior output's field.
	if ordinary.fields == nil {
		ordinary.fields = make(map[string]externalValue)
	}
	ordinary.fields["from-prior-output"] = externalValue{unknown: "injected output"}
	for node := range reflected.methods {
		delete(reflected.methods, node)
	}
	reflected.methods[&ast.FuncDecl{Name: ast.NewIdent("Fabricated")}] = sourceExpr{}
	next, _ := methodCensusCheck(t, effects, definition, owner, "", "", []string{"Act"}, []string{"Act"})
	if len(next.fields) != 0 {
		t.Fatalf("a later method query borrowed prior output fields: %v", next.fields)
	}
}

// TestExternalMethodCensusWidePackage is a controlled source-width workload,
// not a timing or allocation assertion. The actual root supplies the resource RED.
func TestExternalMethodCensusWidePackage(t *testing.T) {
	var body strings.Builder
	body.WriteString("type Root int\nfunc (Root) Act() {}\n")
	for index := range 256 {
		name := strconv.Itoa(index)
		body.WriteString("type Other" + name + " int\nfunc (Other" + name + ") Foreign" + name + "() {}\n")
	}
	effects, definition, owner := methodCensusFixture(t, body.String())
	wanted := methodCensusExpectedNodes(t, effects, owner.pkg, []string{"Act"})
	ordinaryWalk := externalTypeWalk{effects: effects}
	reflectionWalk := externalReflectionWalk{effects: effects}
	for range 128 {
		ordinary := ordinaryWalk.methods(definition, owner)
		if ordinary.unknown != "" {
			t.Fatal(ordinary.unknown)
		}
		methodCensusCompareNodes(t, ordinary.callables, wanted)
		var part externalReflectionPart
		reflectionWalk.collectMethods(definition, owner, nil, &part)
		reflected := part.reflection
		if reflected.unknown != "" {
			t.Fatal(reflected.unknown)
		}
		methodCensusCompareNodes(t, reflected.methods, wanted)
	}
}

func methodCensusFixture(t testing.TB, body string) (*externalEffects, *ast.TypeSpec, sourceExpr) {
	t.Helper()
	const directory = "internal/methodcensusfixture"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		directory + "/proof.go": "package methodcensusfixture\n" + body,
	})
	definition, owner := methodCensusNamedType(t, effects, effects.source.snapshot.packages[directory], "Root")
	return effects, definition, owner
}

func methodCensusNamedType(t testing.TB, effects *externalEffects, pkg *sourcePackage, name string) (*ast.TypeSpec, sourceExpr) {
	t.Helper()
	for _, file := range pkg.files {
		if file.File.Scope == nil {
			continue
		}
		binding := file.File.Scope.Lookup(name)
		if binding == nil {
			continue
		}
		declaration, ok := binding.Decl.(*ast.TypeSpec)
		if !ok {
			t.Fatalf("authored type %s is not a type declaration", name)
		}
		walk := externalTypeWalk{effects: effects}
		definition, owner, reason := walk.definition(sourceExpr{value: declaration.Name, file: file.File, pkg: pkg})
		if reason != "" {
			t.Fatalf("authored type %s: %s", name, reason)
		}
		return definition, owner
	}
	t.Fatalf("authored type %s is unavailable", name)
	return nil, sourceExpr{}
}

func methodCensusCheck(t *testing.T, effects *externalEffects, definition *ast.TypeSpec, owner sourceExpr, ordinaryCause, reflectedCause string, ordinaryNames, reflectedNames []string) (externalValue, externalReflection) {
	t.Helper()
	ordinaryWalk := externalTypeWalk{effects: effects}
	ordinary := ordinaryWalk.methods(definition, owner)
	if ordinary.unknown != ordinaryCause {
		t.Fatalf("ordinary reason = %q, want %q", ordinary.unknown, ordinaryCause)
	}
	methodCensusCompareNodes(t, ordinary.callables, methodCensusExpectedNodes(t, effects, owner.pkg, ordinaryNames))
	reflectionWalk := externalReflectionWalk{effects: effects}
	var part externalReflectionPart
	reflectionWalk.collectMethods(definition, owner, nil, &part)
	reflected := part.reflection
	if reflected.unknown != reflectedCause {
		t.Fatalf("reflected reason = %q, want %q", reflected.unknown, reflectedCause)
	}
	methodCensusCompareNodes(t, reflected.methods, methodCensusExpectedNodes(t, effects, owner.pkg, reflectedNames))
	return ordinary, reflected
}

func methodCensusExpectedNodes(t *testing.T, effects *externalEffects, pkg *sourcePackage, names []string) map[ast.Node]sourceExpr {
	t.Helper()
	wanted := make(map[ast.Node]sourceExpr)
	for _, name := range names {
		found := false
		for _, function := range effects.typeIdentities.methods[pkg] {
			if function.decl.Name.Name != name {
				continue
			}
			wanted[function.decl] = sourceExpr{value: function.decl.Name, file: function.file, pkg: pkg}
			found = true
		}
		if !found {
			t.Fatalf("authored method %s is unavailable", name)
		}
	}
	return wanted
}

func methodCensusCompareNodes(t *testing.T, got, wanted map[ast.Node]sourceExpr) {
	t.Helper()
	if len(got) != len(wanted) {
		t.Fatalf("method count = %d, want %d exact declarations", len(got), len(wanted))
	}
	for node, source := range wanted {
		actual, present := got[node]
		if !present {
			t.Fatalf("method %s lost its exact declaration", node.(*ast.FuncDecl).Name.Name)
		}
		if actual != source {
			t.Fatalf("method %s changed its defining source", node.(*ast.FuncDecl).Name.Name)
		}
	}
}
