// Design: docs/architecture/plugin/feature-gates.md -- one fixpoint round evaluates an object's definitions once.
package repoinventory

import (
	"go/ast"
	"testing"
)

// TestExternalReadObjectAnswersARoundFromItsCache checks the round cache of
// readObject: a second read in the same round evaluates no definition and
// answers the same value, a caller's copy cannot change the cache, and once
// the round clears the cache a read evaluates again and sees a binding that
// grew. Method: readObject over an authored package variable defined from a
// function literal, with the test-only counters set.
//
// MUTATION: (externalbindings.go, `if cached, found := e.readCache[object]; found {`,
// `if cached, found := e.readCache[object]; found && false {`): every read
// evaluates the definitions again, so the second read counts a second evaluation.
// MUTATION: (externaleffects.go, `	clear(e.readCache)`, `	_ = e.readCache`):
// a new round answers from the old round's cache, so the grown binding is missed.
// MUTATION: (externalbindings.go, `		answered.join(cached)`, `		answered = cached`):
// the caller shares the cache's sets, and the caller's write reaches the cache.
func TestExternalReadObjectAnswersARoundFromItsCache(t *testing.T) {
	const directory = "internal/readcacheproof"
	const filename = directory + "/proof.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		"go.mod":            "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt": "",
		filename:            "package readcacheproof\nvar hook = func() {}\n",
	})
	effects.counters = &externalAnalysisCounters{}
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	object := file.Scope.Lookup("hook")
	if object == nil {
		t.Fatal("authored hook declaration is unavailable")
	}
	effects.indexDefinitions([]string{directory})

	first := effects.readObject(object)
	if effects.counters.objectEvaluations != 1 {
		t.Fatalf("first read evaluated %d times, want 1", effects.counters.objectEvaluations)
	}
	if len(first.callables) == 0 {
		t.Fatal("the first read lost the variable's function literal")
	}
	second := effects.readObject(object)
	if effects.counters.objectEvaluations != 1 {
		t.Fatalf("a second read in the round evaluated again: %d evaluations", effects.counters.objectEvaluations)
	}
	if len(second.callables) != len(first.callables) {
		t.Fatal("the cached read answered a different value")
	}
	for node := range second.callables {
		delete(second.callables, node)
	}
	if third := effects.readObject(object); len(third.callables) != len(first.callables) {
		t.Fatal("a caller's write reached the round's cache")
	}

	grown := ast.NewIdent("grown")
	effects.bind(object, externalValue{callables: map[ast.Node]sourceExpr{grown: {value: grown}}})
	effects.beginRound()
	after := effects.readObject(object)
	if effects.counters.objectEvaluations != 2 {
		t.Fatalf("a read in a new round evaluated %d times in all, want 2", effects.counters.objectEvaluations)
	}
	if _, joined := after.callables[grown]; !joined {
		t.Fatal("a read in a new round missed a binding that grew")
	}
	if effects.counters.objectReads != 4 {
		t.Fatalf("counted %d object reads, want 4", effects.counters.objectReads)
	}
	if effects.counters.fixpointRounds != 1 {
		t.Fatalf("counted %d fixpoint rounds, want 1", effects.counters.fixpointRounds)
	}
}
