// Design: docs/architecture/plugin/feature-gates.md -- the reflection walk treats the type graph by reachability.
package repoinventory

import (
	"go/ast"
	"strconv"
	"strings"
	"testing"
)

// reachabilityInterfaces is the size of the protobuf-like descriptor graph:
// ten interfaces, each returning three others and a concrete item type whose
// field holds a fourth interface, so every type reaches every other one.
const reachabilityInterfaces = 10

// reachabilityDescriptorBody builds that graph, plus types outside it: Lone
// reaches only its own method, and Edge reaches the graph from outside.
func reachabilityDescriptorBody() string {
	var body strings.Builder
	name := func(prefix string, index int) string {
		return prefix + strconv.Itoa(index%reachabilityInterfaces)
	}
	for index := range reachabilityInterfaces {
		body.WriteString("type " + name("I", index) + " interface {\n")
		body.WriteString("\tNext() " + name("I", index+1) + "\n")
		body.WriteString("\tSkip() " + name("I", index+3) + "\n")
		body.WriteString("\tBack() " + name("I", index+reachabilityInterfaces-1) + "\n")
		body.WriteString("\tItem() *" + name("C", index) + "\n")
		body.WriteString("}\n")
		body.WriteString("type " + name("C", index) + " struct { Owner " + name("I", index+5) + " }\n")
		body.WriteString("func (" + name("C", index) + ") Touch" + strconv.Itoa(index) + "() {}\n")
	}
	body.WriteString("type Lone int\nfunc (Lone) Alone() {}\n")
	body.WriteString("type Edge struct { To I3; Own Lone }\n")
	return body.String()
}

// reachabilityFixture builds selected effects over body and answers the
// fixture package.
func reachabilityFixture(t *testing.T, body string) (*externalEffects, *sourcePackage) {
	t.Helper()
	effects := externalTypeSettledSource(t, body, "")
	pkg := effects.source.snapshot.packages[externalTerminalDirectory]
	if pkg == nil {
		t.Fatal("the fixture package was not captured")
	}
	return effects, pkg
}

// reachabilityQuery runs one reflection query for the named type on a fresh
// walker sharing effects, as every production query does.
func reachabilityQuery(t *testing.T, effects *externalEffects, pkg *sourcePackage, name string) externalReflection {
	t.Helper()
	definition, owner := methodCensusNamedType(t, effects, pkg, name)
	walk := externalReflectionWalk{effects: effects}
	return walk.value(sourceExpr{value: definition.Name, file: owner.file, pkg: owner.pkg}, nil, true, false, 0)
}

// reachabilityReference is the exhaustive answer the walk must give for a
// fixture with no generics and no aliases: every exported method with a body
// declared on a named type reachable from root through field, element, key,
// pointer, signature and interface-method types, each type visited once.
func reachabilityReference(t *testing.T, effects *externalEffects, pkg *sourcePackage, root string) map[ast.Node]sourceExpr {
	t.Helper()
	wanted := make(map[ast.Node]sourceExpr)
	seen := make(map[string]bool)
	var visit func(ast.Expr)
	visit = func(expression ast.Expr) {
		switch datatype := expression.(type) {
		case *ast.Ident:
			if seen[datatype.Name] {
				return
			}
			binding := pkg.files[0].File.Scope.Lookup(datatype.Name)
			if binding == nil {
				return
			}
			declaration, typed := binding.Decl.(*ast.TypeSpec)
			if !typed {
				return
			}
			seen[datatype.Name] = true
			for _, function := range effects.typeIdentities.methods[pkg] {
				receiver := function.decl.Recv.List[0].Type
				if star, pointer := receiver.(*ast.StarExpr); pointer {
					receiver = star.X
				}
				named, identifier := receiver.(*ast.Ident)
				if !identifier {
					continue
				}
				if named.Name != datatype.Name {
					continue
				}
				if !ast.IsExported(function.decl.Name.Name) {
					continue
				}
				if function.decl.Body == nil {
					continue
				}
				wanted[function.decl] = sourceExpr{value: function.decl.Name, file: function.file, pkg: pkg}
				visit(function.decl.Type)
			}
			visit(declaration.Type)
		case *ast.StarExpr:
			visit(datatype.X)
		case *ast.ArrayType:
			visit(datatype.Elt)
		case *ast.MapType:
			visit(datatype.Key)
			visit(datatype.Value)
		case *ast.StructType:
			for _, field := range datatype.Fields.List {
				visit(field.Type)
			}
		case *ast.InterfaceType:
			for _, field := range datatype.Methods.List {
				visit(field.Type)
			}
		case *ast.FuncType:
			for _, fields := range []*ast.FieldList{datatype.Params, datatype.Results} {
				if fields == nil {
					continue
				}
				for _, field := range fields.List {
					visit(field.Type)
				}
			}
		}
	}
	visit(ast.NewIdent(root))
	return wanted
}

// TestExternalReflectionReachabilityMatchesTheReference proves the walk over a
// strongly connected descriptor graph answers what an exhaustive reachability
// walk answers, from every root and in any query order, and computes each
// exact frame once per analysis. Method: every type of a protobuf-like graph
// of ten mutually returning interfaces, and two types outside it, is queried
// three times, forwards then backwards, on fresh walkers sharing one
// analysis; each answer must equal the reference written above, the frame
// computations must equal the frames the analysis kept, and the later rounds
// must compute none.
//
// MUTATION: (internal/le/repo/inventory/externalreflectiontypes.go,
// "w.effects.reflectionFrames[member.frame] = union",
// "w.effects.reflectionFrames[member.frame] = member.part.reflection"): each
// member of the component keeps only its own part, so a query rooted at any
// member but the first loses the methods it reaches through the cycle.
// MUTATION: (internal/le/repo/inventory/externalreflectiontypes.go,
// "\tw.collect(source, scope, methods, valueRoot, &result)\n",
// "\tw.effects.reflectionFrames = nil\n\tw.collect(source, scope, methods, valueRoot, &result)\n"):
// the table lives for one query, so every later query computes its frames again.
func TestExternalReflectionReachabilityMatchesTheReference(t *testing.T) {
	effects, pkg := reachabilityFixture(t, reachabilityDescriptorBody())
	counters := &externalAnalysisCounters{}
	effects.counters = counters
	var roots []string
	for index := range reachabilityInterfaces {
		roots = append(roots, "I"+strconv.Itoa(index), "C"+strconv.Itoa(index))
	}
	roots = append(roots, "Lone", "Edge")
	computed := 0
	for round := range 3 {
		order := roots
		if round == 1 {
			order = make([]string, 0, len(roots))
			for index := len(roots) - 1; index >= 0; index-- {
				order = append(order, roots[index])
			}
		}
		for _, root := range order {
			answer := reachabilityQuery(t, effects, pkg, root)
			if answer.unknown != "" {
				t.Fatalf("%s refused: %s", root, answer.unknown)
			}
			wanted := reachabilityReference(t, effects, pkg, root)
			if root == "I0" {
				if len(wanted) != reachabilityInterfaces {
					t.Fatalf("the reference reached %d methods from I0, want all %d", len(wanted), reachabilityInterfaces)
				}
			}
			if root == "Lone" {
				if len(wanted) != 1 {
					t.Fatalf("the reference reached %d methods from Lone, want its own", len(wanted))
				}
			}
			methodCensusCompareNodes(t, answer.methods, wanted)
		}
		if round == 0 {
			computed = counters.reflectionFrames
		}
	}
	if computed == 0 {
		t.Fatal("no frame was computed: the counter is not wired")
	}
	if counters.reflectionFrames != computed {
		t.Fatalf("the later rounds computed %d more frames", counters.reflectionFrames-computed)
	}
	if counters.reflectionFrames != len(effects.reflectionFrames) {
		t.Fatalf("%d frame computations for %d distinct frames", counters.reflectionFrames, len(effects.reflectionFrames))
	}
	if counters.namedVisits <= counters.reflectionFrames {
		t.Fatalf("%d named visits for %d frames: no query joined a kept frame", counters.namedVisits, counters.reflectionFrames)
	}
}

// TestExternalReflectionReachabilityMutualRecursion proves a recursive pair
// answers the same methods, origins, refusal and site from either member and
// in either query order. Method: A and B hold each other, B also holds a
// standard-library interface and a field of an unresolved type. Two fresh
// analyses query A then B, and B then A; every answer must carry both
// methods, the fmt origin and the unresolved reason sited at its identifier.
func TestExternalReflectionReachabilityMutualRecursion(t *testing.T) {
	const body = "type A struct { Next *B }\nfunc (A) TouchA() {}\n" +
		"type B struct { Back *A; Text fmt.Stringer; Bad missing }\nfunc (B) TouchB() {}\n"
	for _, order := range [][]string{{"A", "B"}, {"B", "A"}} {
		effects, pkg := reachabilityFixture(t, body)
		for _, root := range order {
			answer := reachabilityQuery(t, effects, pkg, root)
			label := strings.Join(order, " then ") + ", " + root
			methodCensusCompareNodes(t, answer.methods, methodCensusExpectedNodes(t, effects, pkg, []string{"TouchA", "TouchB"}))
			if len(answer.origins) != 1 {
				t.Fatalf("%s: origins %v, want fmt's alone", label, answer.origins)
			}
			for origin := range answer.origins {
				if !strings.Contains(origin, "fmt") {
					t.Fatalf("%s: origin %q is not fmt's", label, origin)
				}
			}
			if answer.unknown != "reflected type ownership is unresolved: missing" {
				t.Fatalf("%s: refused %q", label, answer.unknown)
			}
			site, identifier := answer.unknownAt.value.(*ast.Ident)
			if !identifier {
				t.Fatalf("%s: the refusal is sited at %T, want the missing identifier", label, answer.unknownAt.value)
			}
			if site.Name != "missing" {
				t.Fatalf("%s: the refusal is sited at %s, want missing", label, site.Name)
			}
		}
	}
}

// TestExternalReflectionReachabilityDiamond proves shared subgraphs cost one
// walk. Method: sixteen levels of two fields to the level below reach one
// Touch method through 2^16 paths; the answer must be that method alone, and
// the frames computed must stay within two per level.
func TestExternalReflectionReachabilityDiamond(t *testing.T) {
	const levels = 16
	var body strings.Builder
	body.WriteString("type Leaf int\nfunc (Leaf) Touch() {}\ntype Node0 = Leaf\n")
	for index := 1; index <= levels; index++ {
		current, previous := strconv.Itoa(index), strconv.Itoa(index-1)
		body.WriteString("type Node" + current + " struct { First Node" + previous + "; Second Node" + previous + " }\n")
	}
	body.WriteString("type Root = Node" + strconv.Itoa(levels) + "\n")
	effects, pkg := reachabilityFixture(t, body.String())
	counters := &externalAnalysisCounters{}
	effects.counters = counters
	answer := reachabilityQuery(t, effects, pkg, "Root")
	if answer.unknown != "" {
		t.Fatalf("the diamond refused: %s", answer.unknown)
	}
	if len(answer.origins) != 0 {
		t.Fatalf("the diamond found origins %v", answer.origins)
	}
	methodCensusCompareNodes(t, answer.methods, methodCensusExpectedNodes(t, effects, pkg, []string{"Touch"}))
	if counters.reflectionFrames > 2*(levels+3) {
		t.Fatalf("%d frames computed for %d levels: shared levels were walked again", counters.reflectionFrames, levels)
	}
}

// TestExternalReflectionReachabilityGrowthRefuses proves the one depth bound
// left still stops an instantiation that grows without end, with the reason
// callers know. Method: grow[T] holds *grow[[]T], so every step is a new
// frame; the query must refuse with the depth reason after about one frame
// per nesting level, and a finite generic recursion must not refuse.
//
// MUTATION: (internal/le/repo/inventory/externalreflectiontypes.go,
// "w.noteUnknown(result, \"reflection type capability depth exceeds 128\")\n\t\treturn\n\t}\n\tif contribution",
// "return\n\t}\n\tif contribution"): the growth bound stops the walk silently,
// so the growing type publishes as harmless.
func TestExternalReflectionReachabilityGrowthRefuses(t *testing.T) {
	const body = "type grow[T any] struct { Next *grow[[]T] }\ntype Root struct { G grow[int] }\n" +
		"type list[T any] struct { Next *list[T]; Item T }\ntype Item int\nfunc (Item) Touch() {}\ntype Finite struct { L list[Item] }\n"
	effects, pkg := reachabilityFixture(t, body)
	counters := &externalAnalysisCounters{}
	effects.counters = counters
	answer := reachabilityQuery(t, effects, pkg, "Root")
	if answer.unknown != "reflection type capability depth exceeds 128" {
		t.Fatalf("the growing instantiation refused %q", answer.unknown)
	}
	if counters.reflectionFrames < externalTypeTermDepthMax {
		t.Fatalf("%d frames computed: the bound fired before the argument nested %d levels", counters.reflectionFrames, externalTypeTermDepthMax)
	}
	if counters.reflectionFrames > 2*externalTypeTermDepthMax {
		t.Fatalf("%d frames computed: the bound did not stop the growth", counters.reflectionFrames)
	}
	finite := reachabilityQuery(t, effects, pkg, "Finite")
	if finite.unknown != "" {
		t.Fatalf("a finite generic recursion refused: %s", finite.unknown)
	}
	methodCensusCompareNodes(t, finite.methods, methodCensusExpectedNodes(t, effects, pkg, []string{"Touch"}))
}

// TestCommittedExternalReflectionDeepInCycleRefuses proves a registering
// method reachable only deep inside a strongly connected descriptor graph is
// still found. Method: an opaque call receives holders whose only field is
// the graph's first interface, and only the seventh interface returns the
// type with the exported method; the control publishes, and the method
// calling the registration function refuses.
func TestCommittedExternalReflectionDeepInCycleRefuses(t *testing.T) {
	var graph strings.Builder
	for index := range reachabilityInterfaces {
		current := strconv.Itoa(index)
		graph.WriteString("type I" + current + " interface { Next() I" + strconv.Itoa((index+1)%reachabilityInterfaces) +
			"; Skip() I" + strconv.Itoa((index+3)%reachabilityInterfaces) + "; Back() I" + strconv.Itoa((index+reachabilityInterfaces-1)%reachabilityInterfaces) + " }\n")
	}
	graph.WriteString("type I7deep interface { I7; Leaf() leaf }\ntype leaf uint32\nfunc (leaf) Fire() { BODY }\n")
	declarations := strings.Replace(graph.String(), "type I7 interface { Next() I8;", "type I7 interface { Deep() I7deep; Next() I8;", 1)
	if declarations == graph.String() {
		t.Fatal("the fixture did not link the deep interface into the graph")
	}
	declarations += "type holder struct { Desc I0 }\n"
	const call = "order.Slice([]holder{{}}, func(i, j int) bool { return false })"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(strings.Replace(declarations, "BODY", "", 1), call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(strings.Replace(declarations, "BODY", "install(0)", 1), call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestExternalReflectionReachabilityKeepsArgumentsApart proves two
// instantiations of one generic type stay two frames. Method: Siblings holds
// Box[Near] and Box[Far] side by side, and Nested holds Nest[Near], whose
// Inner field is Box[Far] and whose Self field is a backedge to itself. Both
// queries must reach TouchNear and TouchFar with no refusal, and Box[Near]
// and Box[Far] must intern to distinct frames: the walk also collects type
// arguments directly, so the methods alone would not show two merged frames.
//
// MUTATION: (internal/le/repo/inventory/externaltypeterms.go,
// "key := externalTypeFrameKey{owner: owner, kind: kind, arguments: list}",
// "key := externalTypeFrameKey{owner: owner, kind: kind, arguments: externalTypeTermAbsent}"):
// frames ignore their arguments, so Box[Near] and Box[Far] share one id.
func TestExternalReflectionReachabilityKeepsArgumentsApart(t *testing.T) {
	const body = "type Near int\nfunc (Near) TouchNear() {}\ntype Far int\nfunc (Far) TouchFar() {}\n" +
		"type Box[T any] struct { Item T; Self *Box[T] }\n" +
		"type Siblings struct { A Box[Near]; B Box[Far] }\n" +
		"type Nest[T any] struct { Item T; Inner Box[Far]; Self *Nest[T] }\n" +
		"type Nested struct { N Nest[Near] }\n"
	effects, pkg := reachabilityFixture(t, body)
	for _, root := range []string{"Siblings", "Nested"} {
		answer := reachabilityQuery(t, effects, pkg, root)
		if answer.unknown != "" {
			t.Fatalf("%s refused: %s", root, answer.unknown)
		}
		methodCensusCompareNodes(t, answer.methods, methodCensusExpectedNodes(t, effects, pkg, []string{"TouchNear", "TouchFar"}))
	}
	box, boxOwner := methodCensusNamedType(t, effects, pkg, "Box")
	siblings, siblingsOwner := methodCensusNamedType(t, effects, pkg, "Siblings")
	structure, isStruct := siblings.Type.(*ast.StructType)
	if !isStruct {
		t.Fatal("Siblings is not a struct")
	}
	owner := effects.reflectionKeys.owner(boxOwner.pkg, box)
	var ids []int32
	for _, field := range structure.Fields.List {
		instance, indexed := field.Type.(*ast.IndexExpr)
		if !indexed {
			t.Fatalf("a Siblings field is %T, want an instantiated Box", field.Type)
		}
		use := sourceExpr{value: instance, file: siblingsOwner.file, pkg: siblingsOwner.pkg}
		argument := externalTypeArgument{source: sourceExpr{value: instance.Index, file: use.file, pkg: use.pkg}, scope: effects.typeIdentities.lexicalScope(use)}
		id, bounded := effects.typeTerms.frame(owner, externalReflectionFrameKind(true, false), []externalTypeArgument{argument})
		if !bounded {
			t.Fatal("a Box argument exceeded the nesting bound")
		}
		ids = append(ids, id)
	}
	if len(ids) != 2 {
		t.Fatalf("Siblings has %d fields, want 2", len(ids))
	}
	if ids[0] == ids[1] {
		t.Fatal("Box[Near] and Box[Far] interned to one frame")
	}
}
