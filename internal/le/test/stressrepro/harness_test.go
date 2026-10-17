package teststressrepro

import (
	"context"
	"slices"
	"strings"
	"testing"

	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
)

// TestEverySuiteLaunchesItsRegisteredHarness proves each suite name stress-repro
// accepts launches that suite's own `le test` argv, read from the functional
// suite registry, and never the bare name as a `le test` verb.
//
// VALIDATES: harnessArgs answers, for every registered suite, the suite's Args
// without the select-everything word, and keeps the words typed after the name.
// PREVENTS: `suite plugin` and `suite reload` launching `le test plugin` and
// `le test reload`, which answer "test is a namespace" and test nothing
// (plan/journal/delegation-hands-the-callee-a-different-argv.md).
func TestEverySuiteLaunchesItsRegisteredHarness(t *testing.T) {
	t.Parallel()
	if len(testfunctional.Suites) == 0 {
		t.Fatal("the functional suite registry is empty, so this test proves nothing")
	}
	for _, registered := range testfunctional.Suites {
		got, err := harnessArgs(registered.Name)
		if err != nil {
			t.Errorf("suite %s: refused: %v", registered.Name, err)
			continue
		}
		want := slices.DeleteFunc(slices.Clone(registered.Args), func(word string) bool {
			return word == testfunctional.AllTests
		})
		if !slices.Equal(got, want) {
			t.Errorf("suite %s launches le test %v, want le test %v", registered.Name, got, want)
		}
	}

	got, err := harnessArgs("plugin --draft")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"bgp", "plugin", "--draft"}; !slices.Equal(got, want) {
		t.Errorf("suite \"plugin --draft\" launches le test %v, want %v", got, want)
	}
	got, err = harnessArgs("reload")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"bgp", "reload", "-p", "1"}; !slices.Equal(got, want) {
		t.Errorf("suite reload launches le test %v, want %v", got, want)
	}
}

// TestUnknownSuiteIsRefusedBeforeLaunchNamingTheValidOnes proves a suite name
// the registry does not hold stops the run before any child or build starts, and
// the refusal names the word, every valid suite, and the command to type.
//
// VALIDATES: run answers exit 2 with the refusal and invokes nothing.
// PREVENTS: a load generator burning for its full window over a child that
// prints usage, and a refusal that leaves the reader guessing the right name.
func TestUnknownSuiteIsRefusedBeforeLaunchNamingTheValidOnes(t *testing.T) {
	// Not parallel: stressTree sets the environment.
	root := stressTree(t)
	fake := &fakeRunner{results: []processResult{{}}}
	deps, _ := testDependencies(fake)
	opts := baseOptions()
	opts.Suite = "bgp plugin"

	report, code := run(context.Background(), root, opts, deps)
	if code != 2 {
		t.Fatalf("exit %d, want 2: %#v", code, report)
	}
	if len(fake.invocations) != 0 {
		t.Fatalf("an unknown suite launched %d children", len(fake.invocations))
	}
	for _, want := range []string{`suite "bgp"`, "valid suites: ", "./le test stress-repro run suite "} {
		if !strings.Contains(report.SetupError, want) {
			t.Errorf("refusal lacks %q: %q", want, report.SetupError)
		}
	}
	for _, registered := range testfunctional.Suites {
		if !strings.Contains(report.SetupError, registered.Name) {
			t.Errorf("refusal does not name valid suite %s: %q", registered.Name, report.SetupError)
		}
	}
}
