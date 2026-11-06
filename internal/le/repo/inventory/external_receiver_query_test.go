// Design: docs/architecture/plugin/feature-gates.md -- receiver queries reuse exact source/depth facts without caching capabilities.
// These source-selected controls preserve exact receiver identity and depth.
package repoinventory

import (
	"go/ast"
	"strconv"
	"testing"
)

// TestExternalReceiverQueryAllocationBudget retains the same structural tuple
// through one sink while a real selected alias receiver forces seen-map growth.
func TestExternalReceiverQueryAllocationBudget(t *testing.T) {
	const directory = "internal/receiverquerycost"
	const filename = directory + "/proof.go"
	body := "package receiverquerycost\ntype Root int\n"
	previous := "Root"
	for index := range 24 {
		name := "Alias" + strconv.Itoa(index)
		body += "type " + name + " = " + previous + "\n"
		previous = name
	}
	body += "func (" + previous + ") Act() {}\n"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{filename: body})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	root, found := file.Scope.Lookup("Root").Decl.(*ast.TypeSpec)
	if !found {
		t.Fatal("selected allocation fixture lost its root type")
	}
	source := receiverQuerySelectedMethod(t, effects, pkg, "Act")
	walk := externalTypeWalk{effects: effects}
	type observed struct {
		target  *ast.TypeSpec
		generic bool
		reason  string
	}
	query := func() observed {
		target, generic, reason := walk.receiver(source, 0)
		return observed{target: target, generic: generic, reason: reason}
	}
	want := observed{target: root}
	if got := query(); got != want {
		t.Fatalf("cold receiver query changed its structural result: %+v", got)
	}
	sink := make(chan observed, 1)
	var retained observed
	control := testing.AllocsPerRun(80, func() { sink <- want; retained = <-sink })
	actual := testing.AllocsPerRun(80, func() { sink <- query(); retained = <-sink })
	if retained != want {
		t.Fatalf("repeated receiver query changed its structural result: %+v", retained)
	}
	t.Logf("receiver allocations: repeated-query=%g tuple-control=%g", actual, control)
	if actual > control {
		t.Fatalf("repeated receiver query rebuilt source resolution: %g allocations vs control %g", actual, control)
	}
}

// TestExternalReceiverDepthOutcomeCompatibility interleaves successes and
// refusals so no shallow or successful tuple can erase exact depth/error state.
func TestExternalReceiverDepthOutcomeCompatibility(t *testing.T) {
	const directory = "internal/receiverquerydepth"
	const filename = directory + "/proof.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{filename: `package receiverquerydepth
type Root int
type Alias = Root
type Box[T any] struct{}
func (Root) Direct() {}
func (*Root) Pointer() {}
func (Alias) Aliased() {}
func (*Alias) PointerAlias() {}
func (Box[T]) Generic() {}
`})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	root, found := file.Scope.Lookup("Root").Decl.(*ast.TypeSpec)
	if !found {
		t.Fatal("selected depth fixture lost Root")
	}
	box, found := file.Scope.Lookup("Box").Decl.(*ast.TypeSpec)
	if !found {
		t.Fatal("selected depth fixture lost Box")
	}
	walk := externalTypeWalk{effects: effects}
	const exhausted = "receiver capability depth exceeds 128"
	for _, scenario := range []struct {
		method  string
		depth   int
		target  *ast.TypeSpec
		generic bool
		reason  string
	}{
		{method: "Direct", depth: 0, target: root},
		{method: "Direct", depth: 127, target: root},
		{method: "Direct", depth: 128, reason: exhausted},
		{method: "Direct", depth: 0, target: root},
		{method: "Direct", depth: -1, target: root},
		{method: "Pointer", depth: 126, target: root},
		{method: "Pointer", depth: 127, reason: exhausted},
		{method: "Pointer", depth: 0, target: root},
		{method: "Aliased", depth: 126, target: root},
		{method: "Aliased", depth: 127, reason: exhausted},
		{method: "Aliased", depth: 0, target: root},
		{method: "PointerAlias", depth: 125, target: root},
		{method: "PointerAlias", depth: 126, reason: exhausted},
		{method: "Generic", depth: 127, target: box, generic: true},
		{method: "Generic", depth: 128, reason: exhausted},
		{method: "Generic", depth: 0, target: box, generic: true},
	} {
		source := receiverQuerySelectedMethod(t, effects, pkg, scenario.method)
		target, generic, reason := walk.receiver(source, scenario.depth)
		if target != scenario.target {
			t.Fatalf("%s depth%d changed declaration identity", scenario.method, scenario.depth)
		}
		if generic != scenario.generic {
			t.Fatalf("%s depth%d changed its generic flag", scenario.method, scenario.depth)
		}
		if reason != scenario.reason {
			t.Fatalf("%s depth%d got %q, want %q", scenario.method, scenario.depth, reason, scenario.reason)
		}
	}
	direct := receiverQuerySelectedMethod(t, effects, pkg, "Direct")
	direct.value = &ast.ParenExpr{X: direct.value}
	receiverQueryAssert(t, &walk, direct, 126, root, false, "")
	receiverQueryAssert(t, &walk, direct, 127, nil, false, exhausted)
	for _, scenario := range []struct{ name, text, reason string }{
		{name: "missing", text: "func (Missing) Bad() {}", reason: "retained type declaration ownership is unresolved"},
		{name: "non-type", text: "var NotType = 1\nfunc (NotType) Bad() {}", reason: "retained declaration is not a type"},
		{name: "cycle", text: "type A = B\ntype B = A\nfunc (A) Bad() {}", reason: "cyclic retained receiver alias capability"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			selected := typeIdentitySelectedSource(t, directory, map[string]string{filename: "package receiverquerydepth\ntype Root int\n" + scenario.text + "\n"})
			owner := selected.source.snapshot.packages[directory]
			source := receiverQuerySelectedMethod(t, selected, owner, "Bad")
			query := externalTypeWalk{effects: selected}
			for range 2 {
				receiverQueryAssert(t, &query, source, 0, nil, false, scenario.reason)
			}
			if scenario.name == "cycle" {
				if _, _, reason := query.receiver(source, 125); reason != scenario.reason {
					t.Fatalf("cycle before bound changed: %q", reason)
				}
				if _, _, reason := query.receiver(source, 126); reason != exhausted {
					t.Fatalf("exhaustion before cycle changed: %q", reason)
				}
			}
		})
	}
}

// TestExternalReceiverSourceIdentityCompatibility keeps snapshot ownership and
// lexical import proof attached to exact file/package/AST identities.
func TestExternalReceiverSourceIdentityCompatibility(t *testing.T) {
	const directory = "internal/receiverqueryidentity"
	const first = directory + "/first.go"
	const second = directory + "/second.go"
	sources := map[string]string{
		first:                                    "package receiverqueryidentity\nimport foreign \"github.com/ze-software/ze/internal/receiverqueryforeign\"\ntype Root int\ntype Imported = foreign.Root\nfunc (Root) Direct() {}\n",
		second:                                   "package receiverqueryidentity\ntype Other string\n",
		"internal/receiverqueryforeign/proof.go": "package receiverqueryforeign\ntype Root int\n",
	}
	effects := typeIdentitySelectedSource(t, directory, sources)
	other := typeIdentitySelectedSource(t, directory, sources)
	pkg := effects.source.snapshot.packages[directory]
	walk := externalTypeWalk{effects: effects}
	valid := receiverQuerySelectedMethod(t, effects, pkg, "Direct")
	file := typeIdentitySelectedFile(t, pkg, first)
	root, found := file.Scope.Lookup("Root").Decl.(*ast.TypeSpec)
	if !found {
		t.Fatal("selected identity fixture lost Root")
	}
	receiverQueryAssert(t, &walk, valid, 0, root, false, "")
	unselected := receiverQuerySelectedMethod(t, other, other.source.snapshot.packages[directory], "Direct")
	receiverQueryAssert(t, &walk, unselected, 0, nil, false, "retained type defining file is unavailable")
	receiverQueryAssert(t, &walk, valid, 0, root, false, "")
	imported, found := file.Scope.Lookup("Imported").Decl.(*ast.TypeSpec)
	if !found {
		t.Fatal("selected identity fixture lost its alias")
	}
	foreign := effects.source.snapshot.packages["internal/receiverqueryforeign"]
	foreignFile := typeIdentitySelectedFile(t, foreign, "internal/receiverqueryforeign/proof.go")
	foreignRoot, found := foreignFile.Scope.Lookup("Root").Decl.(*ast.TypeSpec)
	if !found {
		t.Fatal("selected identity fixture lost its imported Root")
	}
	source := sourceExpr{value: imported.Type, file: file, pkg: pkg}
	receiverQueryAssert(t, &walk, source, 0, foreignRoot, false, "")
	source.file = typeIdentitySelectedFile(t, pkg, second)
	receiverQueryAssert(t, &walk, source, 0, nil, false, "retained type import binding is unresolved")
	source.file = file
	receiverQueryAssert(t, &walk, source, 0, foreignRoot, false, "")
}

func receiverQueryAssert(t *testing.T, walk *externalTypeWalk, source sourceExpr, depth int, wantTarget *ast.TypeSpec, wantGeneric bool, wantReason string) {
	t.Helper()
	target, generic, reason := walk.receiver(source, depth)
	if target != wantTarget {
		t.Fatalf("depth%d changed exact receiver target: got%p want%p", depth, target, wantTarget)
	}
	if generic != wantGeneric {
		t.Fatalf("depth%d changed receiver generic flag: got%t want%t", depth, generic, wantGeneric)
	}
	if reason != wantReason {
		t.Fatalf("depth%d changed receiver cause: got%q want%q", depth, reason, wantReason)
	}
}

func receiverQuerySelectedMethod(t *testing.T, effects *externalEffects, pkg *sourcePackage, name string) sourceExpr {
	t.Helper()
	for _, function := range effects.typeIdentities.methods[pkg] {
		if function.decl.Name.Name != name {
			continue
		}
		if len(function.decl.Recv.List) != 1 {
			t.Fatal("authored receiver fixture has unsupported shape")
		}
		return sourceExpr{value: function.decl.Recv.List[0].Type, file: function.file, pkg: pkg}
	}
	t.Fatalf("authored receiver method %s is unavailable", name)
	return sourceExpr{}
}
