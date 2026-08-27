// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7 -- the call-site gate is called as
// a function and answers structured data.
// PREVENTS: the fail-open the script carried. It walked six roots relative to
// the working directory and dropped every walk error, so a tree it never read
// reported `Emitters checked: 0` and `ci-dispatch-check: OK`.

package cidispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// surfaceOnce builds the command surface once per test binary. Building it
// walks internal/ and pkg/ and reads the linked registry, so a per-case build
// would repeat the most expensive thing this package does.
var (
	surfaceOnce   sync.Once
	sharedTree    string
	sharedSurf    Surface
	sharedSurfErr error
)

// surface answers the shared command surface over this checkout.
func surface(t *testing.T) (string, Surface) {
	t.Helper()
	surfaceOnce.Do(func() {
		sharedTree, sharedSurfErr = lepath.Root()
		if sharedSurfErr != nil {
			return
		}
		sharedSurf, sharedSurfErr = NewSurface(sharedTree)
	})
	if sharedSurfErr != nil {
		t.Fatalf("build the command surface: %v", sharedSurfErr)
	}
	return sharedTree, sharedSurf
}

// VALIDATES: this checkout passes the gate, and the walk actually found
// emitters to check.
// PREVENTS: a port whose walk roots resolve to nothing, which looks identical to
// a tree in which every emitter resolves.
func TestTheRealCheckoutPassesAndWasRead(t *testing.T) {
	tree, loaded := surface(t)

	report, err := CheckWith(loaded, tree, emitterFloor)
	if err != nil {
		t.Fatalf("the gate could not read this checkout: %v", err)
	}
	if !report.Valid() {
		t.Errorf("this checkout fails the dispatch gate:\n%s", report.Text())
	}
	if report.CommandsKnown < surfaceFloor {
		t.Errorf("the surface carries %d commands, want at least %d", report.CommandsKnown, surfaceFloor)
	}
	if report.EmittersChecked == 0 {
		t.Error("the gate checked no emitter at all: a population it never walked")
	}
}

// VALIDATES: a tree holding no emitter is an ERROR.
// PREVENTS: the script's fail-open, demonstrated on the built script run in an
// empty directory: Emitters checked 0, OK, exit 0.
func TestATreeWithNoEmitterIsAnError(t *testing.T) {
	_, loaded := surface(t)

	dir := t.TempDir()
	for _, root := range scanRoots {
		if err := os.MkdirAll(filepath.Join(dir, root), 0o750); err != nil {
			t.Fatalf("create %s: %v", root, err)
		}
	}
	if _, err := CheckWith(loaded, dir, emitterFloor); err == nil {
		t.Error("a tree holding no emitter passed the floor")
	}

	// The surface itself refuses a tree it cannot walk, which is the earlier of
	// the two guards.
	if _, err := NewSurface(t.TempDir()); err == nil {
		t.Error("the surface was built over a tree holding neither internal nor pkg")
	}
}

// VALIDATES: the recogniser tells five shapes apart over one fixture.
// PREVENTS: a computed command silently becoming a pass, which is the shape
// every defect this gate exists for took.
func TestTheFixtureDrawsEachShape(t *testing.T) {
	_, loaded := surface(t)

	findings, scanned, passthroughs := ScanFile(loaded, "fixture.ci", fixtureSource, pyEmitters)
	if scanned != 6 {
		t.Errorf("the fixture read %d emitters, want 6", scanned)
	}
	if passthroughs != 1 {
		t.Errorf("the fixture counted %d pass-through variables, want 1", passthroughs)
	}
	if len(findings) != 3 {
		t.Fatalf("the fixture drew %d findings, want 3: %+v", len(findings), findings)
	}
	if findings[0].Kind != KindDead || findings[0].Command != "bgp health" {
		t.Errorf("finding 0 is %+v, want the dead literal", findings[0])
	}
	if findings[1].Kind != KindDead {
		t.Errorf("finding 1 is %+v, want the dead static prefix", findings[1])
	}
	if findings[2].Kind != KindUnverifiable {
		t.Errorf("finding 2 is %+v, want the prefix-less computed command", findings[2])
	}
	for _, finding := range findings {
		if finding.Detail == "" {
			t.Errorf("%+v carries no detail, so a reader is told nothing", finding)
		}
	}
}

// VALIDATES: the selftest table passes over this checkout and answers 0.
// PREVENTS: a selftest that cannot pass, which would be discovered only by the
// gate that runs it.
func TestTheSelftestPassesOverThisCheckout(t *testing.T) {
	_, loaded := surface(t)

	report := SelftestWith(loaded)
	if failures := report.Failures(); len(failures) != 0 {
		t.Errorf("the selftest failed: %v", failures)
	}
	if code := report.Code(1); code != 0 {
		t.Errorf("a passing selftest answers %d, want 0", code)
	}
	if len(report.Results) != len(selftestCases) {
		t.Fatalf("the selftest answered %d rows for %d cases", len(report.Results), len(selftestCases))
	}
	for i, result := range report.Results {
		if result.Case != selftestCases[i].name {
			t.Errorf("row %d names %q, want %q", i, result.Case, selftestCases[i].name)
		}
	}
}

// VALIDATES: every fixture case FAILS over a fixture result that breaks the
// property it declares.
// PREVENTS: a case whose check can never fail, which is a selftest row that
// proves nothing and still counts as a pass.
func TestEveryFixtureCaseFailsOnABrokenResult(t *testing.T) {
	_, loaded := surface(t)

	// A result that breaks all four fixture cases at once: no findings and no
	// pass-through.
	broken := fixtureResult{}
	for _, testCase := range selftestCases {
		if !strings.HasPrefix(testCase.name, "fixture-") {
			continue
		}
		if detail := testCase.check(loaded, broken); detail == "" {
			t.Errorf("case %s passed over an empty fixture result", testCase.name)
		}
	}

	// The surface case fails over a surface carrying nothing.
	for _, testCase := range selftestCases {
		if testCase.name != "surface-loaded" {
			continue
		}
		if detail := testCase.check(Surface{}, broken); detail == "" {
			t.Error("the surface case passed over a surface holding no command")
		}
	}
}

// VALIDATES: AC-7 -- the payload is data a JSON encoder takes, with the
// script's own keys.
// PREVENTS: a port that answers a rendered page, which no pipe operator can act
// on.
func TestReportIsStructuredData(t *testing.T) {
	raw, err := json.Marshal(Report{
		SchemaVersion: 1, CommandsKnown: 425, EmittersChecked: 1075, PassThrough: 53,
		Findings: []Finding{{File: "a.ci", Line: 3, Kind: KindDead, Emitter: "dispatch", Command: "bgp health", Detail: "d"}},
	})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, key := range []string{
		`"schema-version"`, `"commands-known"`, `"emitters-checked"`, `"pass-through"`,
		`"findings"`, `"file"`, `"line"`, `"kind"`, `"emitter"`, `"command"`, `"detail"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
}

// VALIDATES: the page carries the three counts and the right verdict.
// PREVENTS: a failing gate whose page still reads OK, which is what a reader
// acts on.
func TestThePageCarriesItsCountsAndItsVerdict(t *testing.T) {
	clean := Report{SchemaVersion: 1, CommandsKnown: 425, EmittersChecked: 1075, PassThrough: 53}.Text()
	for _, want := range []string{"Registered commands: 425", "Emitters checked:    1075", "Pass-through (var):  53", "ci-dispatch-check: OK"} {
		if !strings.Contains(clean, want) {
			t.Errorf("the clean page has no %q:\n%s", want, clean)
		}
	}

	failed := Report{Findings: []Finding{
		{File: "a.ci", Line: 3, Kind: KindDead, Emitter: "dispatch", Command: "bgp health", Detail: "gone"},
		{File: "b.ci", Line: 9, Kind: KindUnverifiable, Emitter: "send", Command: "build(x)", Detail: "opaque"},
	}}.Text()
	if strings.Contains(failed, "OK") {
		t.Errorf("a page holding findings still says OK:\n%s", failed)
	}
	if !strings.Contains(failed, "ci-dispatch-check: FAIL (1 dead, 1 unverifiable)") {
		t.Errorf("the failing page does not carry its verdict:\n%s", failed)
	}
	if !strings.Contains(failed, `a.ci:3: dead: dispatch("bgp health")`) {
		t.Errorf("the failing page does not carry its site:\n%s", failed)
	}
}

// VALIDATES: the area dispatches its two actions and refuses the two mistakes.
// PREVENTS: a verb that drifts from its gate name, which would leave the Make
// target pointing at nothing after the swap.
func TestTheAreaDispatchesItsActions(t *testing.T) {
	if _, code := Answer([]string{"nope"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code := Answer([]string{"check", "value"}); code != 1 {
		t.Errorf("a value after an action answers %d, want 1", code)
	}

	verbs := Actions()
	if len(verbs.Actions) != 2 {
		t.Fatalf("the area holds %d actions, want 2", len(verbs.Actions))
	}
	if verbs.Actions[0].Verb != "check" || verbs.Actions[1].Verb != "selftest" {
		t.Errorf("the verbs are %q and %q, want check and selftest", verbs.Actions[0].Verb, verbs.Actions[1].Verb)
	}
}
