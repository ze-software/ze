// Design: docs/functional-tests.md -- the native command keeps its closed option table
// Related: actions.go -- the action table this test reads its expectations from

// The external test package links le's composition root, which imports this
// package, so the dispatcher answers with every area the shipped le holds and
// its startup check finds every namespace this area sits in.
package teststressrepro_test

import (
	"io"
	"os"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/le" // the le root handler and every area
	leroot "github.com/ze-software/ze/internal/le/le/root"
	teststressrepro "github.com/ze-software/ze/internal/le/test/stressrepro"
)

// collapsed folds every run of whitespace into one space, so a line the page
// wrapped at its width still reads as the sentence the table declared.
func collapsed(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// VALIDATES: AC-18 -- `le test stress-repro run --help`, typed through the le
// dispatcher, answers 0 and prints the run action's page as the registered
// table declares it: the summary, the whole `Why`, the grammar, and every
// keyword with its help text.
// PREVENTS: a hand-written usage standing in for the table, which drifts from
// the grammar the parser reads, or the help word reaching the run as a value
// and starting the load generator.
// Method: dispatch the line with stdout on a pipe, then check the page against
// the fields of the table's own `run` row, never a copied list.
func TestStressReproRunHelpComesFromTheTable(t *testing.T) {
	listing := teststressrepro.Actions()
	if len(listing.Actions) != 1 {
		t.Fatalf("the table holds %#v, want the one action run", listing.Actions)
	}
	run := listing.Actions[0]
	if run.Verb != "run" {
		t.Fatalf("the table's action is %q, want run", run.Verb)
	}
	grammar, held := listing.Grammar(run.Verb)
	if !held {
		t.Fatalf("the table does not hold %q", run.Verb)
	}
	if len(run.Parameters) == 0 {
		t.Fatal("the run row declares no keyword")
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	code := leroot.Dispatch("le", []string{"test", "stress-repro", "run", "--help"})
	os.Stdout = previous
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("le test stress-repro run --help exited %d, want 0:\n%s", code, raw)
	}
	page := collapsed(string(raw))

	wants := []string{
		"le test stress-repro run - " + run.Summary,
		collapsed(run.Why),
		"Usage: le test stress-repro " + collapsed(grammar),
	}
	for _, parameter := range run.Parameters {
		wants = append(wants, collapsed(parameter.Help))
	}
	for _, want := range wants {
		if !strings.Contains(page, want) {
			t.Errorf("the run page does not carry %q from the table:\n%s", want, raw)
		}
	}
}
