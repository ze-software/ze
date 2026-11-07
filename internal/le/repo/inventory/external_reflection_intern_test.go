// Design: docs/architecture/plugin/feature-gates.md -- interned reflection keys and the analysis-wide reflectedValue memo change no answer.
package repoinventory

import (
	"go/ast"
	"testing"
)

// TestExternalReflectionKeysMatchDirectComparison requires interned ids to be
// equal exactly when the identities are. Method: two definitions in two
// packages are interned in every combination, and the frame key of every
// definition and flag pair is compared with a field-by-field comparison.
//
// MUTATION: (internal/le/repo/inventory/externalreflectionintern.go,
// "key |= 2", "key |= 1"): a methods frame and a value-root frame share a key.
func TestExternalReflectionKeysMatchDirectComparison(t *testing.T) {
	definitions := []*ast.TypeSpec{{Name: ast.NewIdent("A")}, {Name: ast.NewIdent("B")}}
	packages := []*sourcePackage{{}, {}}
	var keys externalReflectionKeys
	type owner struct {
		pkg        *sourcePackage
		definition *ast.TypeSpec
	}
	ids := make(map[owner]int32)
	for range 2 {
		for _, pkg := range packages {
			for _, definition := range definitions {
				id := keys.owner(pkg, definition)
				if previous, seen := ids[owner{pkg, definition}]; seen {
					if previous != id {
						t.Fatalf("one owner interned to two ids: %d and %d", previous, id)
					}
				}
				ids[owner{pkg, definition}] = id
			}
		}
	}
	for left, leftID := range ids {
		for right, rightID := range ids {
			if (leftID == rightID) != (left == right) {
				t.Fatalf("owner ids disagree with the identities: %v and %v", left, right)
			}
		}
	}
}

// TestExternalReflectionNamedMemoMatchesResolution requires every kept
// resolution of a type reference to be what resolving it again answers.
// Method: a fixture with retained and standard-library field types is solved,
// then every entry the walk kept is resolved afresh and compared; the fixture
// must have kept both kinds, so the comparison is not vacuous.
//
// MUTATION: (internal/le/repo/inventory/externalreflectionintern.go,
// "return externalReflectionNamed{external: true, origin: origin}",
// "return externalReflectionNamed{origin: origin}"): a standard-library
// reference reads as a retained one with no declaration.
func TestExternalReflectionNamedMemoMatchesResolution(t *testing.T) {
	effects := externalTypeSettledSource(t, "type node struct {\n\tname fmt.Stringer\n\tnext *node\n}\n", "fmt.Print(node{})")
	if err := effects.solve(); err != nil {
		t.Logf("solve refused: %v", err)
	}
	external, retained := 0, 0
	for key, kept := range effects.reflectionNames {
		fresh := effects.resolveReflectionNamed(key.sourceOf())
		if kept != fresh {
			t.Fatalf("a kept resolution differs from a fresh one: %+v and %+v", kept, fresh)
		}
		if kept.external {
			external++
		}
		if kept.definition != nil {
			retained++
		}
	}
	if external == 0 {
		t.Fatal("the fixture kept no standard-library reference")
	}
	if retained == 0 {
		t.Fatal("the fixture kept no retained definition")
	}
}

// TestExternalReflectionMemoAnswersMatch requires the reflectedValue memo to
// change no answer. Method: each recursive-type and terminal fixture is
// solved with the memo on and with the test switch turning it off; the
// published callables, origins, unknown, site and error must be identical,
// and the cluster fixture must answer from the memo at least once. The
// func-field fixture asks two queries in one scope at one depth, the fields of
// pair, which a non-generic declaration reads in the nil scope: name's
// descriptor holds nothing, and hook's signature descriptor reaches Arm, which
// no type walk reaches because a function type carries no retained method. Arm
// must reach the pool, so the memo must tell the two sources apart.
//
// MUTATION: (internal/le/repo/inventory/externalreflection.go,
// "key := externalReflectionMemoKey{source: source, scope: scope}",
// "key := externalReflectionMemoKey{scope: scope}"): every query in one scope
// answers the first query's descriptor, so hook answers name's and the
// func-field answer loses Arm.
func TestExternalReflectionMemoAnswersMatch(t *testing.T) {
	const mutual = "type alpha struct {\n\tname string\n\tnext *beta\n}\n" +
		"type beta struct {\n\tlabel string\n\tback  []*alpha\n}\n" +
		"func (b *beta) Arm() { registry.Register(label) }\n" +
		"var forestA alpha\nvar forestB beta\n"
	for _, fixture := range []struct{ name, declarations, statements string }{
		{"statement", "type Statement struct {\n\tKeyword    string\n\tstatements []*Statement\n}\nfunc (s *Statement) Statement() *Statement { return s }\n",
			"root := &Statement{Keyword: \"module\", statements: []*Statement{{Keyword: \"leaf\"}}}; _ = root.Statement(); fmt.Print(root)"},
		{"mutual-alpha", mutual, "fmt.Print(forestA.next.back)"},
		{"mutual-beta", mutual, "fmt.Print(forestB.back)"},
		{"cluster", externalTypeCluster, "fmt.Print(holder{})"},
		{"polymorphic", externalTerminalLoop, "fmt.Print(loop{})"},
		{"unknown-site", externalRecursiveGrow + "type tree struct {\n\tchildren []*tree\n\tstuck    loop\n}\n", "fmt.Print(tree{}.children)"},
		{"func-field", "type armed struct{}\nfunc (armed) Arm() { registry.Register(label) }\ntype pair struct {\n\tname string\n\thook func(armed)\n}\n", "fmt.Print(pair{}.hook)"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			memoized := externalTypeSettledSource(t, fixture.declarations, fixture.statements)
			memoized.counters = &externalAnalysisCounters{}
			want := externalTypeSettledAnswer(memoized, memoized.solve())
			unmemoized := externalTypeSettledSource(t, fixture.declarations, fixture.statements)
			unmemoized.typeProbe = &externalTypeProbe{unmemoized: true}
			got := externalTypeSettledAnswer(unmemoized, unmemoized.solve())
			if got != want {
				t.Errorf("the memo changed the answer:\nmemo:\n%s\nno memo:\n%s", want, got)
			}
			if fixture.name == "func-field" {
				if !externalTypeSettledHolds(memoized, "Arm") {
					t.Errorf("the memoized solve never published Arm:\n%s", want)
				}
				if !externalTypeSettledHolds(unmemoized, "Arm") {
					t.Errorf("the solve without the memo never published Arm:\n%s", got)
				}
			}
			if fixture.name != "cluster" {
				return
			}
			if memoized.counters.memoHits == 0 {
				t.Errorf("the cluster never answered from the memo: %d queries", memoized.counters.reflectedValues)
			}
		})
	}
}

// TestExternalReflectionMemoKeepsDepth requires the memo never to answer a
// query from another depth. Method: holder's descriptor is asked at depth 0,
// where the walk finishes, then near the depth guard, where it refuses; the
// second answer must be the refusal a walk without the memo gives.
//
// MUTATION: (internal/le/repo/inventory/externalreflectionmemo.go,
// "if slot.depth == depth {", "if slot.depth >= 0 {"): the deep query answers
// the shallow one and loses its depth refusal.
func TestExternalReflectionMemoKeepsDepth(t *testing.T) {
	ask := func(effects *externalEffects, depths ...int) externalReflection {
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
			source := sourceExpr{value: spec.Type, file: file.File, pkg: pkg}
			var answer externalReflection
			for _, depth := range depths {
				answer = effects.reflectedValue(source, effects.typeIdentities.lexicalScope(source), depth)
			}
			return answer
		}
		t.Fatal("holder is not declared")
		return externalReflection{}
	}
	const deep = 128
	memoized := externalTypeSettledSource(t, externalTypeCluster, "fmt.Print(holder{})")
	if shallow := ask(memoized, 0); shallow.unknown != "" {
		t.Fatalf("the shallow query refused: %s", shallow.unknown)
	}
	got := ask(memoized, deep)
	unmemoized := externalTypeSettledSource(t, externalTypeCluster, "fmt.Print(holder{})")
	unmemoized.typeProbe = &externalTypeProbe{unmemoized: true}
	want := ask(unmemoized, deep)
	if want.unknown == "" {
		t.Fatal("the deep query did not reach the depth guard; the fixture proves nothing")
	}
	if got.unknown != want.unknown {
		t.Fatalf("the memo answered the deep query with %q, the walk with %q", got.unknown, want.unknown)
	}
}
