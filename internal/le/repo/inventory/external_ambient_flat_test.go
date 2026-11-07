// Design: docs/architecture/plugin/feature-gates.md -- an external parameter absorbs every growth of the ambient pool.
package repoinventory

import (
	"go/ast"
	"testing"
)

// TestExternalParametersAbsorbEachAmbientGrowthOnce checks how an exposed
// callable's parameters take the ambient pool: a callable parameter takes its
// flattened union, a holder takes it whole, and a scalar takes none. A second
// exposure at the same pool version joins nothing new. The test proves that
// by placing a callable in the pool without a growth, which a re-join would
// copy. A growth is absorbed at once. The test drives externalParameters over
// an authored package, because an actual-root run that reaches it takes
// minutes.
//
// MUTATION: (externalbindings.go, `if e.formalAmbient[name.Obj] == e.ambient+1 {`,
// `if false {`): every exposure joins the pool again, so the callable placed
// without a growth reaches the parameter.
// MUTATION: (externalbindings.go, `if e.formalAmbient[name.Obj] == e.ambient+1 {`,
// `if e.formalAmbient[name.Obj] != 0 {`): a parameter that absorbed the pool
// once never absorbs it again, so the second growth's callable is missing.
// MUTATION: (externalbindings.go, `ambient = externalFlat(e.amb, 0)`,
// `ambient = e.amb`): the callable parameter takes the pool unflattened, so
// the nested callable stays under a field name it cannot select.
// MUTATION: (externalbindings.go, `if shape == externalFormalScalar {`,
// `if false {`): the scalar parameter takes the pool.
func TestExternalParametersAbsorbEachAmbientGrowthOnce(t *testing.T) {
	const directory = "internal/ambientproof"
	const filename = directory + "/proof.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		"go.mod":                                     "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt":                          "",
		registrationPackage + "/register.go":         "package registry\nimport _ \"fmt\"\nfunc Register(label string) {}\n",
		"internal/component/config/yang/register.go": "package yang\nimport _ \"fmt\"\n",
		filename: `package ambientproof
type box struct { Label string }
func Expose(callback func(), holder *box, count int) {}
`,
	})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	var expose *ast.FuncDecl
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok {
			expose = function
		}
	}
	if expose == nil {
		t.Fatal("authored Expose declaration is unavailable")
	}
	parameters := make(map[string]*ast.Ident)
	for _, field := range expose.Type.Params.List {
		for _, ident := range field.Names {
			parameters[ident.Name] = ident
		}
	}
	if len(parameters) != 3 {
		t.Fatalf("authored Expose parameters are unavailable: %v", parameters)
	}
	callback, holder, count := parameters["callback"].Obj, parameters["holder"].Obj, parameters["count"].Obj
	site := sourceExpr{file: file, pkg: pkg}
	nested := func(callable *ast.Ident) externalValue {
		return externalValue{fields: map[string]externalValue{
			"pooled": {callables: map[ast.Node]sourceExpr{callable: {value: callable}}},
		}}
	}

	first := ast.NewIdent("first")
	if !effects.growAmbient(nested(first)) {
		t.Fatal("the first growth did not change the pool")
	}
	effects.externalParameters(expose, site)
	if effects.bindings[callback].callables[first].value != first {
		t.Fatal("the callable parameter lost the pool's nested callable")
	}
	if len(effects.bindings[callback].fields) != 0 {
		t.Fatal("the callable parameter kept the pool's field names")
	}
	if effects.bindings[holder].fields["pooled"].callables[first].value != first {
		t.Fatal("the holder parameter did not take the pool whole")
	}
	if len(effects.bindings[count].callables) != 0 || len(effects.bindings[count].fields) != 0 {
		t.Fatal("the scalar parameter took the pool")
	}

	unannounced := ast.NewIdent("unannounced")
	effects.amb.callables = map[ast.Node]sourceExpr{unannounced: {value: unannounced}}
	effects.changed = false
	effects.externalParameters(expose, site)
	if _, joined := effects.bindings[callback].callables[unannounced]; joined {
		t.Fatal("an exposure at an unchanged pool version joined the pool again")
	}
	if effects.changed {
		t.Fatal("an exposure at an unchanged pool version reported a change")
	}

	second := ast.NewIdent("second")
	if !effects.growAmbient(nested(second)) {
		t.Fatal("the second growth did not change the pool")
	}
	effects.externalParameters(expose, site)
	if effects.bindings[callback].callables[second].value != second {
		t.Fatal("the callable parameter missed a growth of the pool")
	}
	if effects.bindings[holder].fields["pooled"].callables[second].value != second {
		t.Fatal("the holder parameter missed a growth of the pool")
	}
	if effects.bindings[callback].callables[first].value != first {
		t.Fatal("a later growth lost the earlier callable")
	}
}
