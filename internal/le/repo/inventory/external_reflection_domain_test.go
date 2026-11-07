// Design: docs/architecture/plugin/feature-gates.md -- reflection authority has independent ownership and bounded propagation.
package repoinventory

import (
	"go/ast"
	"strings"
	"testing"
)

// TestExternalReflectionJoinOwnership proves stable monotone unions and that
// describing reflected methods cannot make them ordinary function owners.
func TestExternalReflectionJoinOwnership(t *testing.T) {
	method := ast.NewIdent("Method")
	source := sourceExpr{value: method}
	input := externalReflection{methods: map[ast.Node]sourceExpr{method: source}, origins: map[string]bool{"fmt": false}, unknown: "first reflection unknown"}
	var value externalValue
	if !value.join(externalValue{reflection: input}) {
		t.Fatal("first reflection authority did not grow the value")
	}
	if value.join(externalValue{reflection: input}) {
		t.Fatal("identical reflection authority kept growing")
	}
	if len(value.callables) != 0 {
		t.Fatal("reflection method became an ordinary callable owner")
	}
	if len(value.origins) != 0 {
		t.Fatal("unexposed reflection origin became ordinary authority")
	}
	if value.unknown != "" {
		t.Fatal("unexposed reflection uncertainty became ordinary execution uncertainty")
	}
	if !value.reflection.origins["fmt"] {
		t.Fatal("reflection origin map membership was dropped")
	}
	var independent externalValue
	independent.join(value)
	delete(value.reflection.methods, method)
	delete(value.reflection.origins, "fmt")
	if input.methods[method] != source {
		t.Fatal("output mutation changed its method input")
	}
	if independent.reflection.methods[method] != source {
		t.Fatal("two reflection outputs shared their method maps")
	}
	if !independent.reflection.origins["fmt"] {
		t.Fatal("two reflection outputs shared their origin maps")
	}
	independent.reflection.join(externalReflection{unknown: "later reflection unknown"})
	if independent.reflection.unknown != input.unknown {
		t.Fatal("later reflection uncertainty replaced its first cause")
	}
	effects := externalEffects{}
	effects.promoteReflection(independent.reflection)
	if effects.amb.callables[method] != source {
		t.Fatal("exposure lost the reflected method")
	}
	if !effects.amb.origins["fmt"] {
		t.Fatal("exposure lost the reflected origin")
	}
	if effects.amb.unknown != input.unknown {
		t.Fatal("exposure lost the explicit reflection unknown")
	}
}

// TestExternalReflectionFlatBoundary proves per-field authority survives whole
// value flattening while the existing cutoff excludes an unreachable node.
func TestExternalReflectionFlatBoundary(t *testing.T) {
	method := ast.NewIdent("Method")
	excluded := ast.NewIdent("Excluded")
	child := externalValue{reflection: externalReflection{methods: map[ast.Node]sourceExpr{method: {value: method}}}}
	root := externalValue{fields: map[string]externalValue{
		"child":  child,
		"deeper": {fields: map[string]externalValue{"excluded": {reflection: externalReflection{methods: map[ast.Node]sourceExpr{excluded: {value: excluded}}}}}},
	}}
	flat := externalFlat(root, 126)
	if flat.reflection.methods[method].value != method {
		t.Fatal("flattening lost its reachable field reflection authority")
	}
	if _, found := flat.reflection.methods[excluded]; found {
		t.Fatal("flattening included reflection authority beyond its cutoff")
	}
	if flat.unknown != "function capture structure exceeds 128 steps" {
		t.Fatalf("flattening lost its explicit cutoff: %q", flat.unknown)
	}
	delete(flat.reflection.methods, method)
	if child.reflection.methods[method].value != method {
		t.Fatal("flattening shared a field's reflection map")
	}
	cutoff := externalFlat(child, 128)
	if len(cutoff.reflection.methods) != 0 {
		t.Fatal("flattening retained excluded root reflection authority")
	}
	if cutoff.reflection.unknown != "" {
		t.Fatal("flattening invented reflection uncertainty for the excluded root")
	}
}

// TestExternalReflectionDescriptorBoundaries uses real selected declarations to
// distinguish aliases, defined representations, substitutions and nil interfaces.
func TestExternalReflectionDescriptorBoundaries(t *testing.T) {
	const directory = "internal/reflectionbounds"
	const filename = directory + "/proof.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{filename: `package reflectionbounds
type leaf int
func (leaf) Touch() {}
type alias = leaf
type distinct leaf
type holder[T any] struct { Value T }
var Instantiated holder[leaf]
type opaque interface { Inspect(leaf) }
type genericOpaque[T any] interface { Inspect(T) }
type nilAlias = genericOpaque[leaf]
type withOpaque struct { Hidden genericOpaque[leaf] }
var NilGeneric genericOpaque[leaf]
type channelOnly func(chan int)
type chain struct { Next *chain }
func (chain) Touch() {}
type alternate chain
`})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	for _, scenario := range []struct {
		name    string
		methods int
	}{
		{name: "leaf", methods: 1},
		{name: "alias", methods: 1},
		{name: "distinct"},
		{name: "opaque"},
		{name: "nilAlias"},
		{name: "withOpaque", methods: 1},
		{name: "channelOnly"},
		{name: "alternate", methods: 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			name := ast.NewIdent(scenario.name)
			name.Obj = file.Scope.Lookup(scenario.name)
			result := effects.reflectedValue(sourceExpr{value: name, file: file, pkg: pkg}, nil, 0)
			if result.unknown != "" {
				t.Fatal(result.unknown)
			}
			if len(result.methods) != scenario.methods {
				t.Fatalf("wrong reflected method set: got %d, want %d", len(result.methods), scenario.methods)
			}
		})
	}
	declaration, declared := file.Scope.Lookup("Instantiated").Decl.(*ast.ValueSpec)
	if !declared {
		t.Fatal("selected generic fixture lost its variable type")
	}
	result := effects.reflectedValue(sourceExpr{value: declaration.Type, file: file, pkg: pkg}, nil, 0)
	if result.unknown != "" {
		t.Fatal(result.unknown)
	}
	if len(result.methods) != 1 {
		t.Fatal("generic descriptor lost its actual leaf method")
	}
	declaration, declared = file.Scope.Lookup("NilGeneric").Decl.(*ast.ValueSpec)
	if !declared {
		t.Fatal("selected generic interface fixture lost its variable type")
	}
	result = effects.reflectedValue(sourceExpr{value: declaration.Type, file: file, pkg: pkg}, nil, 0)
	if result.unknown != "" {
		t.Fatal(result.unknown)
	}
	if len(result.methods) != 0 {
		t.Fatal("nil generic interface exposed its type argument's methods")
	}
	name := ast.NewIdent("opaque")
	name.Obj = file.Scope.Lookup("opaque")
	walk := externalReflectionWalk{effects: effects}
	result = walk.value(sourceExpr{value: name, file: file, pkg: pkg}, nil, true, false, 0)
	if len(result.methods) != 1 {
		t.Fatal("known interface descriptor lost its method parameter's type")
	}
	if result.unknown != "" {
		t.Fatal(result.unknown)
	}
	missing := effects.reflectedValue(sourceExpr{value: ast.NewIdent("missing"), file: file, pkg: pkg}, nil, 0)
	if !strings.Contains(missing.unknown, "unresolved") {
		t.Fatalf("unknown descriptor silently became empty: %+v", missing)
	}
	primitive := sourceExpr{value: ast.NewIdent("int"), file: file, pkg: pkg}
	if result := walk.value(primitive, nil, true, false, 127); result.unknown != "" {
		t.Fatalf("primitive inside the descriptor bound was refused: %+v", result)
	}
	if result := walk.value(primitive, nil, true, false, 128); !strings.Contains(result.unknown, "128") {
		t.Fatalf("descriptor cutoff lost its uncertainty: %+v", result)
	}
}

// TestExternalReflectionVisitIdentity proves reconstructed scopes cannot grow
// monotone identities and suppressed method coverage cannot hide a child edge.
func TestExternalReflectionVisitIdentity(t *testing.T) {
	parameter := ast.NewIdent("T")
	parameter.Obj = &ast.Object{Name: "T", Kind: ast.Typ}
	bind := func(argument externalTypeArgument) *externalTypeScope {
		return &externalTypeScope{parameters: map[string]externalTypeBinding{"T": {object: parameter.Obj, argument: argument}}}
	}
	actual := externalTypeArgument{source: sourceExpr{value: ast.NewIdent("int")}}
	other := externalTypeArgument{source: sourceExpr{value: ast.NewIdent("string")}}
	composite := sourceExpr{value: &ast.ArrayType{Elt: parameter}}
	var terms externalTypeTerms
	frame := func(scope *externalTypeScope, methods, valueRoot bool) (int32, bool) {
		return terms.frame(0, externalReflectionFrameKind(methods, valueRoot), []externalTypeArgument{{source: composite, scope: scope}})
	}
	id := func(scope *externalTypeScope, methods, valueRoot bool) int32 {
		t.Helper()
		frame, bounded := frame(scope, methods, valueRoot)
		if !bounded {
			t.Fatal("a shallow argument exceeded the nesting bound")
		}
		return frame
	}
	left := id(bind(actual), true, false)
	if id(bind(actual), true, false) != left {
		t.Fatal("rebuilt equivalent substitution scopes interned to different frames")
	}
	if id(bind(actual), false, false) == left {
		t.Fatal("suppressed methods interned to the enabled-method frame")
	}
	if id(bind(actual), true, true) == left {
		t.Fatal("nil-interface value suppression interned to the descriptor frame")
	}
	if id(bind(other), true, false) == left {
		t.Fatal("different actual substitutions interned to one frame")
	}
	scope := bind(actual)
	for range externalTypeTermDepthMax + 1 {
		scope = bind(externalTypeArgument{source: composite, scope: scope})
	}
	if _, bounded := frame(scope, true, false); bounded {
		t.Fatal("an argument nested past the bound interned a frame")
	}
}

// TestExternalReflectionDescriptorOrigin uses the same source-selected witness
// path as the metadata solver fixtures; a type origin is retained and certified.
func TestExternalReflectionDescriptorOrigin(t *testing.T) {
	const directory = "internal/reflectionorigin"
	const filename = directory + "/proof.go"
	selected := typeIdentitySelectedSource(t, directory, map[string]string{
		"go.mod":                                     "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt":                          "",
		registrationPackage + "/register.go":         "package registry\nimport _ \"fmt\"\nfunc Register(label string) {}\n",
		"internal/component/config/yang/register.go": "package yang\nimport _ \"fmt\"\n",
		filename: `package reflectionorigin
import (
	"fmt"
	registry "github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/component/config/yang"
)
var label = "retained metadata"
type signature func(fmt.State)
var callback signature
func init() { registry.Register(label); fmt.Print(callback) }
`,
	})
	source, err := newExternalSource(selected.source.snapshot)
	if err != nil {
		t.Fatal(err)
	}
	effects := newExternalEffects(source)
	pkg := source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	name := ast.NewIdent("signature")
	name.Obj = file.Scope.Lookup("signature")
	reflection := effects.reflectedValue(sourceExpr{value: name, file: file, pkg: pkg}, nil, 0)
	if reflection.unknown != "" {
		t.Fatal(reflection.unknown)
	}
	if !reflection.origins["fmt"] {
		t.Fatal("reflected external type lost its exact package origin")
	}
	if err := effects.solve(); err != nil {
		t.Fatal(err)
	}
	if !effects.metadata[file.Scope.Lookup("label")] {
		t.Fatal("origin control lost real registration metadata")
	}
	if len(effects.admitted) != 1 {
		t.Fatal("origin control did not admit its opaque call")
	}
}

// TestExternalReflectionDescriptorLexicalOwner proves alias targets retain their
// defining imports and a generic parameter cannot borrow a package type's methods.
func TestExternalReflectionDescriptorLexicalOwner(t *testing.T) {
	const directory = "internal/reflectionlexical"
	const filename = directory + "/first.go"
	const foreign = "internal/reflectionforeign"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		filename: `package reflectionlexical
import imported "github.com/ze-software/ze/internal/reflectionforeign"
type alias = imported.Leaf
func scoped[Leaf any]() { var unused func(Leaf); _ = unused }
`,
		directory + "/second.go": "package reflectionlexical\ntype Leaf int\nfunc (Leaf) Local() {}\n",
		foreign + "/proof.go":    "package reflectionforeign\ntype Leaf int\nfunc (Leaf) Foreign() {}\n",
	})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	name := ast.NewIdent("alias")
	name.Obj = file.Scope.Lookup("alias")
	reflection := effects.reflectedValue(sourceExpr{value: name, file: file, pkg: pkg}, nil, 0)
	if reflection.unknown != "" {
		t.Fatal(reflection.unknown)
	}
	if len(reflection.methods) != 1 {
		t.Fatal("cross-file alias lost its exact foreign method set")
	}
	for node, source := range reflection.methods {
		method, declared := node.(*ast.FuncDecl)
		if !declared {
			t.Fatal("reflected method lost its declaration identity")
		}
		if method.Name.Name != "Foreign" {
			t.Fatal("alias borrowed the local same-spelled type's method")
		}
		if source.pkg.dir != foreign {
			t.Fatal("alias method borrowed its importing package's execution owner")
		}
	}
	function := pkg.functions["scoped"]
	statement, declared := function.decl.Body.List[0].(*ast.DeclStmt)
	if !declared {
		t.Fatal("scoped fixture lost its declaration statement")
	}
	group, declared := statement.Decl.(*ast.GenDecl)
	if !declared {
		t.Fatal("scoped fixture lost its declaration group")
	}
	variable, declared := group.Specs[0].(*ast.ValueSpec)
	if !declared {
		t.Fatal("scoped fixture lost its variable type")
	}
	source := sourceExpr{value: variable.Type, file: file, pkg: pkg}
	reflection = effects.reflectedValue(source, effects.typeIdentities.lexicalScope(source), 0)
	if !strings.Contains(reflection.unknown, "parameter") {
		t.Fatalf("unbound lexical parameter silently borrowed a package type: %+v", reflection)
	}
	if len(reflection.methods) != 0 {
		t.Fatal("unbound lexical parameter acquired package methods")
	}
}
