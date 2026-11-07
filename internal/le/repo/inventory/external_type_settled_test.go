// Design: docs/architecture/plugin/feature-gates.md -- settled recursive type frames are reused across one analysis.
package repoinventory

import (
	"go/ast"
	"slices"
	"strings"
	"testing"
)

// externalTypeCluster is six types, each pointing at the next and at itself,
// the last pointing back at the first, held six times by holder. One field
// declaration per holder field, so each field walks t1 on its own.
const externalTypeCluster = "type t1 struct {\n\tnext *t2\n\tself []*t1\n}\n" +
	"type t2 struct {\n\tnext *t3\n\tself []*t2\n}\n" +
	"type t3 struct {\n\tnext *t4\n\tself []*t3\n}\n" +
	"type t4 struct {\n\tnext *t5\n\tself []*t4\n}\n" +
	"type t5 struct {\n\tnext *t6\n\tself []*t5\n}\n" +
	"type t6 struct {\n\tfirst *t1\n\tself  []*t6\n}\n" +
	"type holder struct {\n\ta *t1\n\tb *t1\n\tc *t1\n\td *t1\n\te *t1\n\tf *t1\n}\n"

// externalTypeSettledSource builds selected effects the way
// externalTerminalSelectedSource does, for a test or a benchmark.
func externalTypeSettledSource(t testing.TB, declarations, statements string) *externalEffects {
	t.Helper()
	selected := typeIdentitySelectedSource(t, externalTerminalDirectory, map[string]string{
		"go.mod":                                     "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt":                          "",
		registrationPackage + "/register.go":         "package registry\nimport _ \"fmt\"\nfunc Register(label string) {}\n",
		"internal/component/config/yang/register.go": "package yang\nimport _ \"fmt\"\n",
		externalTerminalFilename: `package terminalproof
import (
	"fmt"
	registry "github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/component/config/yang"
)
var label = "retained registration metadata"
` + declarations + `
func init() {
	registry.Register(label)
` + statements + `
}
`,
	})
	source, err := newExternalSource(selected.source.snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return newExternalEffects(source)
}

// externalTypeSettledAnswer renders what a solve published or refused: the
// error, then the ambient pool's callables and origins by position, and its
// unknown with its site.
func externalTypeSettledAnswer(effects *externalEffects, err error) string {
	fset := effects.source.snapshot.fset
	var lines []string
	for node := range effects.amb.callables {
		lines = append(lines, "callable "+fset.Position(node.Pos()).String())
	}
	for origin := range effects.amb.origins {
		lines = append(lines, "origin "+origin)
	}
	slices.Sort(lines)
	if err != nil {
		lines = append(lines, "error "+err.Error())
	}
	lines = append(lines, "unknown "+effects.amb.unknown)
	if effects.amb.unknownAt.value != nil {
		lines = append(lines, "site "+fset.Position(effects.amb.unknownAt.value.Pos()).String())
	}
	return strings.Join(lines, "\n")
}

// TestExternalTypeSettledAnswersMatch requires the settled table to change no
// answer. Method: each recursive-type and terminal fixture is solved with the
// table on and with the test switch turning it off; the published callables,
// origins, unknown, site and error must be identical. A fixture naming a
// method must also publish it, so its comparison is not between two empty
// pools. The literal fixtures expose a field with no location, which reaches
// beta's method only through the cycle's estimates (a variable's field
// exposes the whole variable).
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "if w.lowest < own {", "if w.lowest < own && false {") settles a frame that
// read an outer estimate: the mutual-alpha-literal answer loses beta's method.
func TestExternalTypeSettledAnswersMatch(t *testing.T) {
	const arm = "func (t *tree) arm() { registry.Register(label) }\n"
	const mutual = "type alpha struct {\n\tname string\n\tnext *beta\n}\n" +
		"type beta struct {\n\tlabel string\n\tback  []*alpha\n}\n" +
		"func (b *beta) arm() { registry.Register(label) }\n" +
		"var forestA alpha\nvar forestB beta\n"
	for _, fixture := range []struct{ name, declarations, statements, holds string }{
		{"statement", "type Statement struct {\n\tKeyword    string\n\tstatements []*Statement\n}\nfunc (s *Statement) Statement() *Statement { return s }\n",
			"root := &Statement{Keyword: \"module\", statements: []*Statement{{Keyword: \"leaf\"}}}; _ = root.Statement(); fmt.Print(root)", ""},
		{"field-methods", "type tree struct {\n\tname     string\n\tchildren []*tree\n}\n" + arm + "var forest tree\n", "fmt.Print(forest.children)", ""},
		{"field-methods-literal", "type tree struct {\n\tname     string\n\tchildren []*tree\n}\n" + arm, "fmt.Print(tree{}.children)", "arm"},
		{"mutual-alpha", mutual, "fmt.Print(forestA.next.back)", ""},
		{"mutual-alpha-literal", mutual, "fmt.Print(alpha{}.next.back)", "arm"},
		{"mutual-beta", mutual, "fmt.Print(forestB.back)", ""},
		{"mutual-beta-literal", mutual, "fmt.Print(beta{}.back)", "arm"},
		{"cluster", externalTypeCluster, "fmt.Print(holder{})", ""},
		{"polymorphic", externalTerminalLoop, "fmt.Print(loop{})", ""},
		{"unknown-site", externalRecursiveGrow + "type tree struct {\n\tchildren []*tree\n\tstuck    loop\n}\n", "fmt.Print(tree{}.children)", ""},
		{"local-projection", externalTerminalLoop, "fmt.Print((struct { Blocked loop; Label string }{Label: \"scalar copy\"}).Label)", ""},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			settled := externalTypeSettledSource(t, fixture.declarations, fixture.statements)
			settled.typeProbe = &externalTypeProbe{}
			want := externalTypeSettledAnswer(settled, settled.solve())
			unsettled := externalTypeSettledSource(t, fixture.declarations, fixture.statements)
			unsettled.typeProbe = &externalTypeProbe{unsettled: true}
			got := externalTypeSettledAnswer(unsettled, unsettled.solve())
			if got != want {
				t.Errorf("the settled table changed the answer:\nsettled:\n%s\nunsettled:\n%s", want, got)
			}
			if fixture.holds != "" {
				if !externalTypeSettledHolds(settled, fixture.holds) {
					t.Errorf("the settled solve never published %s:\n%s", fixture.holds, want)
				}
				if !externalTypeSettledHolds(unsettled, fixture.holds) {
					t.Errorf("the unsettled solve never published %s:\n%s", fixture.holds, got)
				}
			}
			if settled.typeProbe.expansions > unsettled.typeProbe.expansions {
				t.Errorf("the settled table expanded more frames: %d > %d", settled.typeProbe.expansions, unsettled.typeProbe.expansions)
			}
		})
	}
}

// externalTypeSettledHolds answers whether a solve published the method or
// function declared under name to the ambient pool.
func externalTypeSettledHolds(effects *externalEffects, name string) bool {
	for node := range effects.amb.callables {
		declaration, declared := node.(*ast.FuncDecl)
		if !declared {
			continue
		}
		if declaration.Name.Name == name {
			return true
		}
	}
	return false
}

// externalTypeClusterWalk walks holder once and answers the frames expanded.
func externalTypeClusterWalk(t testing.TB, effects *externalEffects) int {
	t.Helper()
	pkg := effects.source.snapshot.packages[externalTerminalDirectory]
	for _, file := range pkg.files {
		object := file.File.Scope.Lookup("holder")
		if object == nil {
			continue
		}
		spec, declared := object.Decl.(*ast.TypeSpec)
		if !declared {
			t.Fatal("holder is not a type declaration")
		}
		before := effects.typeProbe.expansions
		walk := externalTypeWalk{effects: effects}
		value := walk.value(sourceExpr{value: spec.Name, file: file.File, pkg: pkg}, nil, true, 0)
		if value.unknown != "" {
			t.Fatalf("the cluster walk refused: %s", value.unknown)
		}
		if !walk.references {
			t.Fatal("the cluster walk lost its reference types")
		}
		return effects.typeProbe.expansions - before
	}
	t.Fatal("fixture lost holder")
	return 0
}

// TestExternalTypeSettledBoundsClusterVisits requires a recursive cluster held
// six times to be expanded about once. Method: holder holds t1 six times; t1
// to t6 form one component with a self loop each. With the table, the frames
// expanded stay within four per distinct frame (holder and six types); without
// it, every holder field repeats the whole component, at least three times as
// many.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypesettled.go,
// "slot, found := w.effects.settledTypes[frame.id]",
// "slot, found := w.effects.settledTypes[frame.id]; found = false") never reuses a
// settled frame: the settled count reaches the unsettled one.
// MUTATION: (internal/le/repo/inventory/externaleffecttypesettled.go,
// "w.effects.settledTypes[frame.id] = externalTypeSettled{", "_ = externalTypeSettled{") never
// stores a settled frame: the settled count reaches the unsettled one.
func TestExternalTypeSettledBoundsClusterVisits(t *testing.T) {
	const distinct = 7
	settled := externalTypeSettledSource(t, externalTypeCluster, "")
	settled.typeProbe = &externalTypeProbe{}
	with := externalTypeClusterWalk(t, settled)
	unsettled := externalTypeSettledSource(t, externalTypeCluster, "")
	unsettled.typeProbe = &externalTypeProbe{unsettled: true}
	without := externalTypeClusterWalk(t, unsettled)
	if with > 4*distinct {
		t.Errorf("the settled walk expanded %d frames for %d distinct frames", with, distinct)
	}
	if without < 3*with {
		t.Errorf("the unsettled walk expanded %d frames, the settled one %d: the table saved nothing", without, with)
	}
}

// BenchmarkExternalTypeSettledCluster walks the cluster with the settled table
// on and off.
func BenchmarkExternalTypeSettledCluster(b *testing.B) {
	for _, mode := range []struct {
		name      string
		unsettled bool
	}{{"settled", false}, {"unsettled", true}} {
		b.Run(mode.name, func(b *testing.B) {
			effects := externalTypeSettledSource(b, externalTypeCluster, "")
			effects.typeProbe = &externalTypeProbe{unsettled: mode.unsettled}
			b.ReportAllocs()
			for b.Loop() {
				externalTypeClusterWalk(b, effects)
			}
		})
	}
}

// externalTypeSettledRender renders a type value with its field structure:
// callables by position, origins, references and unknown, then each field
// under its name, so two values compare by everything a consumer reads.
func externalTypeSettledRender(effects *externalEffects, value externalValue, prefix string) []string {
	fset := effects.source.snapshot.fset
	var lines []string
	for node := range value.callables {
		lines = append(lines, prefix+"callable "+fset.Position(node.Pos()).String())
	}
	for node := range value.reflection.methods {
		lines = append(lines, prefix+"reflected "+fset.Position(node.Pos()).String())
	}
	for origin := range value.origins {
		lines = append(lines, prefix+"origin "+origin)
	}
	if value.references {
		lines = append(lines, prefix+"references")
	}
	if value.unknown != "" {
		lines = append(lines, prefix+"unknown "+value.unknown)
	}
	for name := range value.fields {
		lines = append(lines, externalTypeSettledRender(effects, value.fields[name], prefix+name+".")...)
	}
	slices.Sort(lines)
	return lines
}

// externalTypeSettledDeclaration answers a declaration of the selected
// package, with the file that declares it.
func externalTypeSettledDeclaration(t *testing.T, effects *externalEffects, name string) (any, sourceExpr) {
	t.Helper()
	pkg := effects.source.snapshot.packages[externalTerminalDirectory]
	for _, file := range pkg.files {
		object := file.File.Scope.Lookup(name)
		if object != nil {
			return object.Decl, sourceExpr{file: file.File, pkg: pkg}
		}
	}
	t.Fatalf("fixture lost %s", name)
	return nil, sourceExpr{}
}

// externalTypeSettledName answers the declared name of a type declaration.
func externalTypeSettledName(t *testing.T, declaration any) *ast.Ident {
	t.Helper()
	spec, declared := declaration.(*ast.TypeSpec)
	if !declared {
		t.Fatal("fixture name is not a type declaration")
	}
	return spec.Name
}

// TestExternalTypeSettledSharedAcrossCalls requires the settled table to serve
// every typeMethods call of one analysis, and never across a grown ambient pool.
// Method: typeMethods on holder twice; the second call expands no frame and
// answers what the first did and what an unsettled analysis answers, fields
// included. Then a generic recursive type, whose unbound type parameter reads
// the pool, is walked before and after the pool gains probe: only the later
// answer may hold probe, and it must.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypesettled.go,
// "w.effects.settledTypes[frame.id] = externalTypeSettled{", "_ = externalTypeSettled{")
// keeps nothing: the second holder call expands the component again.
// MUTATION: (internal/le/repo/inventory/externaleffecttypesettled.go,
// "\t\tif slot.ambient != w.effects.ambient {", "\t\tif slot.ambient != slot.ambient {")
// answers a value read from the smaller pool: the later watched answer lacks probe.
func TestExternalTypeSettledSharedAcrossCalls(t *testing.T) {
	const declarations = externalTypeCluster + "func probe() {}\n" +
		"type watched[T any] struct {\n\tchildren []*watched[T]\n\tvalue    T\n}\n"
	effects := externalTypeSettledSource(t, declarations, "")
	effects.typeProbe = &externalTypeProbe{}
	holder, scope := externalTypeSettledDeclaration(t, effects, "holder")
	holderType := scope
	holderType.value = externalTypeSettledName(t, holder)
	first := strings.Join(externalTypeSettledRender(effects, effects.typeMethods(holderType, 0), ""), "\n")
	expanded := effects.typeProbe.expansions
	second := strings.Join(externalTypeSettledRender(effects, effects.typeMethods(holderType, 0), ""), "\n")
	if effects.typeProbe.expansions != expanded {
		t.Errorf("the second typeMethods call expanded %d frames", effects.typeProbe.expansions-expanded)
	}
	if second != first {
		t.Errorf("the settled answer differs from the first:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	unsettled := externalTypeSettledSource(t, declarations, "")
	unsettled.typeProbe = &externalTypeProbe{unsettled: true}
	_, unsettledScope := externalTypeSettledDeclaration(t, unsettled, "holder")
	unsettledHolder, _ := externalTypeSettledDeclaration(t, unsettled, "holder")
	unsettledScope.value = externalTypeSettledName(t, unsettledHolder)
	want := strings.Join(externalTypeSettledRender(unsettled, unsettled.typeMethods(unsettledScope, 0), ""), "\n")
	if second != want {
		t.Errorf("the settled answer differs from the unsettled one:\nsettled:\n%s\nunsettled:\n%s", second, want)
	}

	watched, watchedScope := externalTypeSettledDeclaration(t, effects, "watched")
	watchedType := watchedScope
	watchedType.value = externalTypeSettledName(t, watched)
	probe, probeScope := externalTypeSettledDeclaration(t, effects, "probe")
	declaration, function := probe.(*ast.FuncDecl)
	if !function {
		t.Fatal("probe is not a function declaration")
	}
	// A plain walk, as referenceType and the range walks make: typeMethods
	// adds reflection, which refuses a type parameter with no argument.
	walk := externalTypeWalk{effects: effects}
	before := walk.value(watchedType, nil, true, 0)
	if before.unknown != "" {
		t.Fatalf("watched refused, so it was never settled: %s", before.unknown)
	}
	if _, held := before.callables[declaration]; held {
		t.Fatal("watched held probe before the pool gained it")
	}
	probeScope.value = declaration.Name
	if !effects.growAmbient(externalCallable(declaration, probeScope)) {
		t.Fatal("the ambient pool did not grow")
	}
	again := externalTypeWalk{effects: effects}
	after := again.value(watchedType, nil, true, 0)
	if _, held := after.callables[declaration]; !held {
		t.Error("watched answered from the smaller ambient pool: probe is missing")
	}
}

// TestExternalTypeSettledSurvivesAmbientGrowth requires a settled value that
// never read the ambient pool to hold after the pool grows, and one that read
// it to be computed again. Method: holder's cluster reads no pool and outside
// holds a standard-library type, which reads it; each is walked, the pool
// grows by one origin, and each is walked again. holder must expand nothing
// the second time, and outside must answer the new origin.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypesettled.go,
// "if slot.ambientRead {", "if false {"): outside answers its value from the
// smaller pool and loses the new origin.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "w.lowest, w.references, w.ambientRead = own, false, false",
// "w.lowest, w.references, w.ambientRead = own, false, true"): every frame
// reads as a pool reader, so holder's cluster expands again.
func TestExternalTypeSettledSurvivesAmbientGrowth(t *testing.T) {
	effects := externalTypeSettledSource(t, externalTypeCluster+"type outside struct {\n\ts fmt.Stringer\n}\n", "fmt.Print(holder{}, outside{})")
	effects.typeProbe = &externalTypeProbe{}
	walk := func(name string) (externalValue, int) {
		t.Helper()
		pkg := effects.source.snapshot.packages[externalTerminalDirectory]
		for _, file := range pkg.files {
			object := file.File.Scope.Lookup(name)
			if object == nil {
				continue
			}
			spec, declared := object.Decl.(*ast.TypeSpec)
			if !declared {
				t.Fatalf("%s is not a type declaration", name)
			}
			before := effects.typeProbe.expansions
			walker := externalTypeWalk{effects: effects}
			value := walker.value(sourceExpr{value: spec.Name, file: file.File, pkg: pkg}, nil, true, 0)
			if value.unknown != "" {
				t.Fatalf("the %s walk refused: %s", name, value.unknown)
			}
			return value, effects.typeProbe.expansions - before
		}
		t.Fatalf("fixture lost %s", name)
		return externalValue{}, 0
	}
	if _, expanded := walk("holder"); expanded == 0 {
		t.Fatal("the first holder walk expanded nothing; the fixture proves nothing")
	}
	walk("outside")
	const grown = "github.com/ze-software/ze/internal/test/grown"
	if !effects.growAmbient(externalOrigin(grown)) {
		t.Fatal("the pool did not grow")
	}
	if _, expanded := walk("holder"); expanded != 0 {
		t.Errorf("holder reads no pool, yet expanded %d frames after it grew", expanded)
	}
	if value, _ := walk("outside"); !value.origins[grown] {
		t.Error("outside reads the pool, yet answered without the origin it gained")
	}
}
