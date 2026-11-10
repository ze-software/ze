// Design: docs/functional-tests.md -- a compiled fixture that stops the daemon it runs inside
// Detail: register_stops_daemon_walk_test.go -- the walk that judges each registration site
// Related: fixture.go -- Register, StopsDaemon

package fixture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"strings"
	"testing"
)

// TestEveryDaemonStoppingRegistrationDeclaresStopsDaemon proves the StopsDaemon
// declarations cannot drift from the code: every registration whose driver
// reaches `request shutdown` declares it, no other one does, and every helper
// that registers a driver its caller passes hands the caller's properties on.
//
// VALIDATES: each Register call in the package's non-test source, and each call
//
//	of a helper that registers a driver its caller supplies, is judged from
//	the driver expression it passes: the functions, methods, package
//	variables and local variables it names are followed through the package
//	until one holds "request shutdown" (Observe and observeConfigured do, so
//	every driver built on them is covered by the same walk).
//
// PREVENTS: a peerless .ci whose Go observer stops the daemon being signaled
//
//	about a second after start because its registration forgot the
//	declaration, and the runner waiting out its self-stop grace on a daemon a
//	falsely declared fixture never stops (runner.recordStartsSelfStoppingFixture).
func TestEveryDaemonStoppingRegistrationDeclaresStopsDaemon(t *testing.T) {
	fset := token.NewFileSet()
	files := parsePackageSource(t, fset)
	walk := newStopsDaemonWalk(files)
	sites := walk.sites(fset, files)
	if len(sites) == 0 {
		t.Fatal("found no registration site in the fixture package; the walk no longer recognizes Register calls")
	}

	findings := make(map[stopsDaemonFindingKind][]string)
	for _, site := range sites {
		where := site.position.String() + " (" + site.callee + " in " + site.scope + ")"
		findings[site.finding] = append(findings[site.finding], where)
	}
	if unjudged := findings[stopsDaemonFindingUnspecified]; len(unjudged) > 0 {
		t.Fatalf("BUG: the walk left %d site(s) unjudged:\n  %s", len(unjudged), strings.Join(unjudged, "\n  "))
	}
	if missing := findings[stopsDaemonMissing]; len(missing) > 0 {
		t.Errorf("%d registration(s) pass a driver that reaches %q without declaring %s, so the runner signals "+
			"a peerless test's daemon about a second after start; add %s to each call:\n  %s",
			len(missing), daemonStopCommand, stopsDaemonProperty, stopsDaemonProperty, strings.Join(missing, "\n  "))
	}
	if undue := findings[stopsDaemonUndue]; len(undue) > 0 {
		t.Errorf("%d registration(s) declare %s but their driver never reaches %q, so the runner waits half "+
			"the test budget for a daemon nothing stops; remove %s from each call:\n  %s",
			len(undue), stopsDaemonProperty, daemonStopCommand, stopsDaemonProperty, strings.Join(undue, "\n  "))
	}
	if dropped := findings[stopsDaemonDropped]; len(dropped) > 0 {
		t.Errorf("%d helper(s) register a driver their caller passes but drop the caller's properties, so no "+
			"caller can declare %s; give the helper a final `properties ...Property` parameter and pass "+
			"`properties...` on:\n  %s",
			len(dropped), stopsDaemonProperty, strings.Join(dropped, "\n  "))
	}
}

// parsePackageSource parses every non-test Go file of the fixture package,
// for every platform, so a Linux-only registration is judged on any host.
func parsePackageSource(t *testing.T, fset *token.FileSet) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the fixture package directory: %v", err)
	}
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		t.Fatal("found no fixture source to walk; the test must run in internal/test/fixture")
	}
	return files
}

// stopsDaemonShapePrelude stands in for the package: Observe holds the
// command the way observeConfigured does, and harness has a method of the
// same name that does not.
const stopsDaemonShapePrelude = `package fixture

type Driver func() error

type Property uint8

const StopsDaemon Property = 1

func Register(name string, driver Driver, properties ...Property) {}

func dispatch(command string) error { return nil }

func Observe() error { return dispatch("request shutdown") }

func idle() error { return nil }

type harness struct{}

func (harness) Observe() error { return idle() }

func (harness) run() error { return Observe() }
`

// TestStopsDaemonWalkJudgesEveryShape pins the walk against each shape a
// registration takes in the fixture package, so a change to the walk that
// stops following one of them goes red here rather than passing the guard in
// silence.
//
// VALIDATES: for each source, the findings counted by kind.
// PREVENTS: a driver shape the walk does not follow (a factory, a package
// variable, a method value, a loop over a local map, a helper registrar)
// escaping the guard, and a same-named method being taken for Observe.
func TestStopsDaemonWalkJudgesEveryShape(t *testing.T) {
	tests := []struct {
		name string
		body string
		want map[stopsDaemonFindingKind]int
	}{
		{"inline literal undeclared", `func init() { Register("a", func() error { return Observe() }) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonMissing: 1}},
		{"inline literal declared", `func init() { Register("a", func() error { return Observe() }, StopsDaemon) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonFine: 1}},
		{"named driver", `func drive() error { return Observe() }
func init() { Register("a", drive) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonMissing: 1}},
		{"factory", `func observer(n string) Driver { return func() error { return Observe() } }
func init() { Register("a", observer("x")) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonMissing: 1}},
		{"package variable", `func wrap(d Driver) Driver { return d }
var drive = wrap(func() error { return Observe() })
func init() { Register("a", drive) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonMissing: 1}},
		{"method value", `func init() { var h harness; Register("a", h.run) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonMissing: 1}},
		{"loop over a local map", `func drive() error { return Observe() }
func init() {
	for n, d := range map[string]Driver{"a": drive} {
		Register(n, d)
	}
}`,
			map[stopsDaemonFindingKind]int{stopsDaemonMissing: 1}},
		{"direct dispatch through a constant", `const stop = "request shutdown"
func init() { Register("a", func() error { return dispatch(stop) }) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonMissing: 1}},
		{"same-named method is not Observe", `func init() { var h harness; Register("a", func() error { return h.Observe() }) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonFine: 1}},
		{"idle undeclared", `func init() { Register("a", idle) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonFine: 1}},
		{"idle declared", `func init() { Register("a", idle, StopsDaemon) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonUndue: 1}},
		{"registrar drops properties", `func reg(name string, d Driver) { Register(name, d) }
func init() { reg("a", Observe) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonDropped: 1, stopsDaemonMissing: 1}},
		{"registrar forwards properties", `func reg(name string, d Driver, properties ...Property) { Register(name, d, properties...) }
func init() { reg("a", Observe, StopsDaemon); reg("b", idle) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonFine: 3}},
		{"registrar through an alias of a func type", `type scenario = Driver
func reg(name string, d scenario) { Register(name, d) }
func init() { reg("a", Observe) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonDropped: 1, stopsDaemonMissing: 1}},
		{"helper that declares for every caller", `func reg(name string, d Driver) {
	Register(name, func() error { _ = d(); return Observe() }, StopsDaemon)
}
func init() { reg("a", idle) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonFine: 1}},
		{"helper that declares but never stops", `func reg(name string, d Driver) { Register(name, d, StopsDaemon) }
func init() { reg("a", Observe) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonUndue: 1}},
		{"registrar whose own body stops the daemon", `func reg(name string, d Driver, properties ...Property) {
	Register(name, func() error { _ = d(); return Observe() }, properties...)
}
func init() { reg("a", idle) }`,
			map[stopsDaemonFindingKind]int{stopsDaemonFine: 1, stopsDaemonMissing: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			prelude, err := parser.ParseFile(fset, "prelude.go", stopsDaemonShapePrelude, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse prelude: %v", err)
			}
			shape, err := parser.ParseFile(fset, "shape.go", "package fixture\n"+tt.body, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse shape: %v", err)
			}
			files := []*ast.File{prelude, shape}
			got := make(map[stopsDaemonFindingKind]int)
			for _, site := range newStopsDaemonWalk(files).sites(fset, files) {
				got[site.finding]++
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("findings by kind = %v, want %v (kinds: fine=%d missing=%d undue=%d dropped=%d)",
					got, tt.want, stopsDaemonFine, stopsDaemonMissing, stopsDaemonUndue, stopsDaemonDropped)
			}
		})
	}
}
