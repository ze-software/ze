// Design: docs/architecture/plugin/feature-gates.md -- a recursive type's capability is a least fixpoint.
package repoinventory

import (
	"strconv"
	"strings"
	"testing"
)

// externalRecursiveGrow is polymorphic recursion: every level instantiates
// grow with a longer argument, so no frame of the walk ever repeats and no
// fixpoint can summarize it. loop holds one, so a plain composite literal of
// loop reaches the depth guard.
const externalRecursiveGrow = "type grow[T any] struct{ next *grow[[]T] }; type loop struct{ next *grow[int] }\n"

const externalRecursiveDepth = "type capability depth exceeds 128"

// externalRecursiveArmed writes the source with FN naming quiet and requires
// the report to publish, then with FN naming armed and requires the escaped
// refusal. FN is replaced in the declarations and the call alike, so only the
// function a recursive type reaches changes between the two runs.
func externalRecursiveArmed(t *testing.T, kind, declarations, call string) {
	t.Helper()
	fixture := externalEffectsFixture(t, kind)
	for _, step := range []struct {
		function string
		publish  bool
	}{{"quiet", true}, {"armed", false}} {
		written := strings.ReplaceAll(externalChannelFunctions+declarations, "FN", step.function)
		exposed := strings.ReplaceAll(call, "FN", step.function)
		fixture.goFile(t, callableOwnerCaller, externalEffectsSource(written, exposed))
		commitInventoryFixture(t, fixture.root)
		if step.publish {
			externalEffectsPositive(t, fixture)
			continue
		}
		externalEffectsRefusal(t, fixture, externalChannelEscaped)
	}
}

// TestCommittedExternalRecursiveStatementPublishes requires the goyang shape
// that refused the actual-root inventory (vendor/github.com/openconfig/goyang/
// pkg/yang/parse.go, Statement holding []*Statement) to publish. Method: a
// Statement tree with a string field and a child slice is built during
// initialization, walked through its own methods and exposed whole to an
// opaque call; the report and plugins must match the control's.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "for range externalTypeRounds {", "for range 1 {") stops before the estimate
// settles: the bound refuses with today's cyclic unknown.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "\t\tif value.Op != token.AND {\n\t\t\treturn sourceExpr{}, false\n\t\t}\n\t\toperand, found",
// "\t\tif value.Op == token.AND {\n\t\t\treturn sourceExpr{}, false\n\t\t}\n\t\toperand, found")
// leaves x := &T{} without a static type, so its method calls mark the receiver formal indirect and refuse.
func TestCommittedExternalRecursiveStatementPublishes(t *testing.T) {
	const declarations = "type Statement struct {\n\tKeyword    string\n\tArgument   string\n\tstatements []*Statement\n}\n" +
		"func (s *Statement) Statement() *Statement { return s }\n" +
		"func (s *Statement) SubStatements() []*Statement { return s.statements }\n"
	const call = "root := &Statement{Keyword: \"module\", Argument: \"proof\", statements: []*Statement{{Keyword: \"leaf\"}}}; _ = root.Statement(); _ = root.SubStatements(); order.Slice(root, func(i, j int) bool { return false })"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
		})
	}
}

// TestCommittedExternalRecursiveChildCallbackRefuses requires a function
// stored in a nested child of a recursive type, and called through the parent,
// to be what the analysis sees. Method: a linked tree whose child holds FN is
// built during initialization, and the parent's method that calls the child's
// hook is handed to an opaque call; quiet publishes and armed refuses as
// escaped. A type alone carries no callable for a func field, so this proves
// the value equations run through the recursive type rather than stopping at it.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "for range externalTypeRounds {", "for range 1 {") restores today's cyclic
// unknown: the quiet control refuses.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "\t\tif value.Op != token.AND {\n\t\t\treturn sourceExpr{}, false\n\t\t}\n\t\toperand, found",
// "\t\tif value.Op == token.AND {\n\t\t\treturn sourceExpr{}, false\n\t\t}\n\t\toperand, found")
// leaves x := &T{} without a static type, so its method value root.fire is read as a missing field and the quiet control refuses.
// MUTATION: (internal/le/repo/inventory/externalformalproof.go,
// "\t\t\tif !formal.receiver {", "\t\t\tif formal.receiver || !formal.receiver {")
// leaves the receiver a method value binds unbound: root.fire refuses its
// formal t as indirect and the quiet control refuses.
func TestCommittedExternalRecursiveChildCallbackRefuses(t *testing.T) {
	const declarations = "type tree struct {\n\thook  func()\n\tchild *tree\n}\n" +
		"func (t *tree) fire() { t.child.hook() }\n"
	const call = "root := &tree{hook: quiet, child: &tree{hook: FN}}; _ = clock.AfterFunc(0, root.fire)"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			externalRecursiveArmed(t, kind, declarations, call)
		})
	}
}

// TestCommittedExternalRecursiveFieldCarriesMethods requires the field that
// holds a type's own children to carry what the type carries. Method: tree's
// unexported method calls FN, which reflection cannot reach. The children
// field of the variable forest is exposed, and so is the children field of a
// tree literal. A field of a variable carries the variable's location, so its
// exposure reads the whole variable, whose own walk holds the method; a field
// of a literal holds no location, so the method reaches it only through the
// recursive reference's estimate. A control exposing the scalar sibling with
// the armed method still publishes, so the refusal is caused by the children
// field.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "result.join(externalFlat(*w.estimate(active), 0))",
// "result.join(externalFlat(externalValue{}, 0))") answers the recursive
// reference with an empty value: the armed literal's children field publishes.
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "if w.estimate(&w.path[own]).join(externalFlat(round, 0)) {",
// "if w.estimate(&w.path[own]).join(externalFlat(round, 0)) && false {") hides
// every growth, so the first round, computed with the empty estimate, settles:
// the armed literal's children field publishes.
func TestCommittedExternalRecursiveFieldCarriesMethods(t *testing.T) {
	const declarations = "type tree struct {\n\tname     string\n\tchildren []*tree\n}\n" +
		"func (t *tree) arm() { FN() }\n" +
		"var forest tree\n"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			written := strings.ReplaceAll(externalChannelFunctions+declarations, "FN", "armed")
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(written, "order.Slice(forest.name, func(i, j int) bool { return false })"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			externalRecursiveArmed(t, kind, declarations, "order.Slice(forest.children, func(i, j int) bool { return false })")
			externalRecursiveArmed(t, kind, declarations, "order.Slice(tree{}.children, func(i, j int) bool { return false })")
		})
	}
}

// TestCommittedExternalMutualRecursion requires alpha -> beta -> alpha to
// settle in both directions. Method: beta's unexported method calls FN. From a
// zero beta, its back field reaches beta again only through alpha; from a zero
// alpha, next.back reaches beta's method only after the outer alpha frame
// iterates, because the inner beta frame re-enters alpha, not itself. Each
// direction is exposed from a variable and from a literal: a variable's field
// carries its location, whose exposure reads the whole variable, so only the
// literal's field depends on the cycle's estimates. Quiet publishes and armed
// refuses as escaped.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "result.join(externalFlat(*w.estimate(active), 0))",
// "result.join(externalFlat(externalValue{}, 0))") empties the cycle: the
// armed alpha-literal direction publishes.
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "if w.estimate(&w.path[own]).join(externalFlat(round, 0)) {",
// "if w.estimate(&w.path[own]).join(externalFlat(round, 0)) && false {") stops
// the outer frame after its first round: the armed alpha-literal direction
// publishes.
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "if w.lowest < own {", "if w.lowest < own && false {") settles beta while
// alpha's estimate is still empty: alpha's next round reuses that smaller
// beta, so alpha{}.next.back lacks the armed method and publishes.
func TestCommittedExternalMutualRecursion(t *testing.T) {
	const declarations = "type alpha struct {\n\tname string\n\tnext *beta\n}\n" +
		"type beta struct {\n\tlabel string\n\tback  []*alpha\n}\n" +
		"func (b *beta) arm() { FN() }\n" +
		"var forestA alpha\nvar forestB beta\n"
	for _, direction := range []struct{ name, call string }{
		{"alpha-root", "order.Slice(forestA.next.back, func(i, j int) bool { return false })"},
		{"beta-root", "order.Slice(forestB.back, func(i, j int) bool { return false })"},
		{"alpha-literal", "order.Slice(alpha{}.next.back, func(i, j int) bool { return false })"},
		{"beta-literal", "order.Slice(beta{}.back, func(i, j int) bool { return false })"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(direction.name+"/"+kind, func(t *testing.T) {
				externalRecursiveArmed(t, kind, declarations, direction.call)
			})
		}
	}
}

// TestCommittedExternalPolymorphicRecursionRefuses requires recursion that
// never repeats a frame to keep refusing. Method: loop holds grow[int], whose
// field instantiates grow[[]int], then grow[[][]int], without end; a loop
// literal is exposed to an opaque call and must refuse at the depth guard.
//
// MUTATION: (internal/le/repo/inventory/externaltypeterms.go,
// "key := externalTypeFrameKey{owner: owner, kind: kind, arguments: list}",
// "key := externalTypeFrameKey{owner: owner, kind: kind, arguments: externalTypeTermAbsent}")
// drops the arguments from the frame key, so every grow instantiation is the
// active one: the fixpoint settles and the report publishes.
func TestCommittedExternalPolymorphicRecursionRefuses(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(externalRecursiveGrow, "order.Slice(loop{}, func(i, j int) bool { return false })"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "external registration capability is unknown: "+externalRecursiveDepth)
		})
	}
}

// TestCommittedExternalRecursiveUnknownKeepsSite requires an unknown inside a
// recursive type to survive the estimate with its own site. Method: tree holds
// a loop, whose polymorphic grow reaches the depth guard, and only tree's
// children field is exposed, so the unknown reaches the exposed value only
// through the recursive reference. The refusal must name the grow line.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "result.join(externalFlat(*w.estimate(active), 0))",
// "result.join(externalFlat(externalValue{}, 0))") drops the unknown with the
// estimate: the children field publishes.
// MUTATION: (internal/le/repo/inventory/externalvalues.go,
// "v.unknownAt = other.unknownAt", "v.unknownAt = sourceExpr{}") drops the
// site: the refusal no longer names the grow line.
func TestCommittedExternalRecursiveUnknownKeepsSite(t *testing.T) {
	const declarations = externalRecursiveGrow + "type tree struct {\n\tchildren []*tree\n\tstuck    loop\n}\n"
	source := externalEffectsSource(declarations, "order.Slice(tree{}.children, func(i, j int) bool { return false })")
	line := 0
	for index, text := range strings.Split(source, "\n") {
		if strings.HasPrefix(text, "type grow[") {
			line = index + 1
		}
	}
	if line == 0 {
		t.Fatal("fixture lost the grow declaration")
	}
	want := callableOwnerCaller + ":" + strconv.Itoa(line) + ": external registration capability is unknown: " + externalRecursiveDepth
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, source)
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, want)
		})
	}
}
