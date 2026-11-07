// Design: docs/architecture/plugin/feature-gates.md -- standard-library named types are external on every route.
package repoinventory

import (
	"go/ast"
	"strconv"
	"strings"
	"testing"
)

// externalStandardTypeShape names one route by which retained code reaches a
// standard-library named type. throughArgument is set when parcel is reachable
// only through that type's type argument, so the registering variant proves
// the argument is still walked.
type externalStandardTypeShape struct {
	name, imports, declaration, value string
	throughArgument                   bool
}

func externalStandardTypeShapes() []externalStandardTypeShape {
	return []externalStandardTypeShape{
		{"atomic-pointer-var", `"sync/atomic"`, "var shared atomic.Pointer[parcel]\n", "&shared", true},
		{"atomic-pointer-literal", `"sync/atomic"`, "", "[]atomic.Pointer[parcel]{{}}", true},
		{"pointer-to-instantiation", `"sync/atomic"`, "type holder struct { Value *atomic.Pointer[parcel] }\n", "[]holder{{}}", true},
		{"seq-field", `"iter"`, "type holder struct { Each iter.Seq[parcel] }\n", "[]holder{{}}", true},
		{"seq2-field", `"iter"`, "type holder struct { Pairs iter.Seq2[string, parcel] }\n", "[]holder{{}}", true},
		{"embedded", "\"sync\"\n \"sync/atomic\"", "type holder struct { sync.Mutex; atomic.Pointer[parcel] }\n", "[]holder{{}}", true},
		{"pool-field", `"sync"`, "type holder struct { Pool sync.Pool }\n", "[]holder{{}}", false},
		{"pool-literal", `"sync"`, "func newParcel() any { return parcel{} }\n", "[]any{&sync.Pool{New: newParcel}}", true},
	}
}

// externalStandardTypeFixture adds the import-cycle witnesses that certify
// sync, sync/atomic and iter as opaque origins, beside the fixture's own.
func externalStandardTypeFixture(t *testing.T, kind string) *asmInventoryFixture {
	t.Helper()
	fixture := externalEffectsFixture(t, kind)
	for _, sink := range []struct{ directory, name string }{
		{registrationPackage, "registry"},
		{"internal/component/config/yang", "yang"},
	} {
		fixture.goFile(t, sink.directory+"/standard_type_witness.go", "package "+sink.name+"\nimport (_ \"sync\"; _ \"sync/atomic\"; _ \"iter\")\n")
	}
	return fixture
}

func externalStandardTypeSource(shape externalStandardTypeShape, body string) string {
	// fire is unexported, so ordinary reflection never lists it: only the
	// capability type walk can reach it, through the type argument.
	declarations := "type parcel struct{}\nfunc (parcel) fire() { " + body + " }\n" + shape.declaration
	call := "order.Slice(" + shape.value + ", func(i, j int) bool { return false })"
	return callableOwnerSource(callableOwnerCase{
		declarations: declarations,
		extraImport:  "order \"sort\"\n " + shape.imports,
		prefix:       "_ = order.Slice\n",
	}, call)
}

// TestCommittedExternalStandardTypesPublish requires every route to a
// standard-library named type, reached from initialization through an opaque
// call, to publish the full ordinary report. Method: each shape replaces the
// caller and must produce the same report and plugins as the control.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "return w.external(identity, arguments, depth)",
// "return externalValue{unknown: identity}") refuses every shape but
// pool-literal.
// MUTATION: (internal/le/repo/inventory/externalliteralshapes.go,
// "return externalLiteralLayout{kind: externalLiteralOpaque}, \"\"",
// "return externalLiteralLayout{}, \"opaque\"") refuses pool-literal.
func TestCommittedExternalStandardTypesPublish(t *testing.T) {
	for _, shape := range externalStandardTypeShapes() {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalStandardTypeFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalStandardTypeSource(shape, ""))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// TestCommittedExternalStandardTypeArgumentRegisters requires a registering
// method reachable only through a standard-library type's type argument, or
// through a standard-library literal's field value, to still refuse. Method:
// the control body is empty; the same source with fire calling the installer
// must refuse with "registration".
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "result.join(externalFlat(retained, 0))", "_ = retained") publishes every
// generic shape.
func TestCommittedExternalStandardTypeArgumentRegisters(t *testing.T) {
	for _, shape := range externalStandardTypeShapes() {
		if !shape.throughArgument {
			continue
		}
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalStandardTypeFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalStandardTypeSource(shape, ""))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
				fixture.goFile(t, callableOwnerCaller, externalStandardTypeSource(shape, "install(0)"))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}

// TestExternalStandardTypeModulePackageStillRefuses keeps the exemption to the
// standard library: an instantiated type from a module package the inventory
// does not retain refuses with its own reason, sited at that expression.
// Method: the foreign package is selected, then dropped from the retained set.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "if identity, external := w.effects.source.externalImport(source, selector); external {",
// "if identity, external := w.effects.source.externalImport(source, selector); external || selector != nil {")
// treats the module selector as external.
func TestExternalStandardTypeModulePackageStillRefuses(t *testing.T) {
	const directory = "internal/standardtypeproof"
	const filename = directory + "/proof.go"
	const foreign = "internal/standardtypeforeign"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		"go.mod":            "module github.com/ze-software/ze\n\ngo 1.26\n",
		foreign + "/box.go": "package standardtypeforeign\ntype Box[T any] struct{ Value T }\n",
		filename:            "package standardtypeproof\nimport foreign \"github.com/ze-software/ze/" + foreign + "\"\ntype local struct{}\ntype holder struct { Remote foreign.Box[local] }\n",
	})
	if _, selected := effects.source.snapshot.packages[foreign]; !selected {
		t.Fatal("fixture did not select the foreign module package")
	}
	delete(effects.source.snapshot.packages, foreign)
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	holder, declared := file.Scope.Lookup("holder").Decl.(*ast.TypeSpec)
	if !declared {
		t.Fatal("fixture lost the holder declaration")
	}
	fields, structured := holder.Type.(*ast.StructType)
	if !structured {
		t.Fatal("fixture holder is not a struct")
	}
	remote := fields.Fields.List[0].Type
	walk := externalTypeWalk{effects: effects}
	value := walk.value(sourceExpr{value: remote, file: file, pkg: pkg}, nil, true, 0)
	if value.unknown != "retained type import binding is unresolved" {
		t.Fatalf("unretained module type lost its refusal reason: %q", value.unknown)
	}
	if len(value.origins) != 0 {
		t.Fatalf("unretained module type gained an external origin: %+v", value.origins)
	}
	if value.unknownAt.value != remote {
		t.Fatalf("refusal reason is not sited at the instantiation: %+v", value.unknownAt.value)
	}
}

// TestExternalTerminalUnknownNamesItsSite requires a terminal ambient unknown
// produced deep in a type walk to refuse at the committed line that produced
// it, not at the call that exposed it. Method: a loop field two struct levels
// down a declaration is exposed whole by fmt.Print; loop holds a polymorphically
// recursive grow, so the depth guard is reached on the grow declaration's line.
//
// MUTATION: (internal/le/repo/inventory/externalvalues.go,
// "v.unknownAt = other.unknownAt", "v.unknownAt = sourceExpr{}") drops the site.
// MUTATION: (internal/le/repo/inventory/externaleffects.go,
// "cause: e.cause(e.amb.unknownAt, reason)", "cause: e.cause(sourceExpr{}, reason)")
// drops the site from the message.
func TestExternalTerminalUnknownNamesItsSite(t *testing.T) {
	effects, file := externalTerminalSelectedSource(t, `
type grow[T any] struct{ next *grow[[]T] }; type loop struct{ next *grow[int] }
type carrier struct {
	Inner struct {
		Blocked loop
	}
}
`, "fmt.Print(carrier{})")
	err := effects.solve()
	externalTerminalRefused(t, effects, err)
	if err == nil {
		return
	}
	var cycle, call ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.TypeSpec:
			if value.Name.Name == "grow" {
				cycle = value.Name
			}
		case *ast.CallExpr:
			call = node
		}
		return true
	})
	if cycle == nil {
		t.Fatal("fixture lost the grow declaration")
	}
	if call == nil {
		t.Fatal("fixture lost its exposing call")
	}
	fset := effects.source.snapshot.fset
	site := fset.Position(cycle.Pos())
	if site.Line == fset.Position(call.Pos()).Line {
		t.Fatal("fixture puts the producer on the exposing call's line")
	}
	want := "committed inventory " + site.Filename + ":" + strconv.Itoa(site.Line) + ": external registration capability is unknown: " + externalTerminalDepthUnknown
	if !strings.Contains(err.Error(), want) {
		t.Errorf("terminal refusal is not sited at its producer:\n got %v\nwant %s", err, want)
	}
	if !strings.Contains(err.Error(), "before publishing inventory") {
		t.Errorf("terminal refusal lost its fix: %v", err)
	}
}
