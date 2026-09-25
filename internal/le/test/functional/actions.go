// Design: docs/architecture/core-design.md -- the functional area
// Detail: run.go -- the run the `gating` action reaches
// Overview: suites.go -- the table every row here is derived from
//
// Every action is derived from the suite table, so dispatch, listing, help, and
// completion share one command surface. A bare `le test functional` answers the
// complete suite catalog, which is what `le test functional list` answers, including
// suites that do not run in the aggregate. `le test functional gating` runs the
// gating suites.
//
// One isolated binary set serves an invocation and is built lazily. The label
// and extra-binary decision are derived before the first suite runs.

package testfunctional

import (
	"context"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/le/path"
)

// listVerb prints the suite table instead of running anything. It is a keyword
// rather than a flag, because the rendering is a pipe operator and the tree is
// the checkout (ai/rules/cli.md).
const listVerb = "list"

// gatingVerb runs the gating suites, which is what a bare `le test functional` used
// to start. It names a RUN LIST and not a suite, so it is dispatched beside the
// suite table rather than added to it. A table row would put it in the catalog,
// in the rerun lines, and in the evidence tiers. Every reader derives those
// from that same table (catalog.go).
const gatingVerb = "gating"

// selectVerb prints the run list a gating run would start for this checkout and
// runs nothing. An operator whose run started fewer suites than they expected
// reads here which suites the suite map ruled out and why, and a run that
// widened says which package it could not answer for.
const selectVerb = "select"

// session is one invocation of the functional area. It holds the checkout and
// the toolchain it derives at most once, and the isolated binary set it builds
// at most once.
//
// Each is derived on first use rather than before dispatch, because three of
// this area's actions need neither. `list` reads the suite table, and `select`
// reads the checkout and answers which suites a gating run would start. An
// operator asking either question waits on no Go toolchain probe.
type session struct {
	root    string
	found   bool
	tc      gotoolchain.Toolchain
	probed  bool
	set     BinarySet
	built   bool
	warmed  bool
	label   string
	buildFn func() (BinarySet, error)
	warmFn  func() error
}

// checkout answers the repository root this invocation runs against.
func (s *session) checkout() (string, error) {
	if s.found {
		return s.root, nil
	}
	root, err := lepath.Root()
	if err != nil {
		return "", err
	}
	s.root, s.found = root, true
	return root, nil
}

// toolchain answers the Go toolchain this invocation runs its suites with.
func (s *session) toolchain() (gotoolchain.Toolchain, error) {
	if s.probed {
		return s.tc, nil
	}
	root, err := s.checkout()
	if err != nil {
		return gotoolchain.Toolchain{}, err
	}
	tc, err := gotoolchain.New(root)
	if err != nil {
		return gotoolchain.Toolchain{}, err
	}
	s.tc, s.probed = tc, true
	return tc, nil
}

// binaries answers the set this invocation runs against, building it on first
// use. A command line naming no suite never pays for a build.
func (s *session) binaries() (BinarySet, error) {
	if s.built {
		return s.set, nil
	}
	set, err := s.buildFn()
	if err != nil {
		return BinarySet{}, err
	}
	s.set, s.built = set, true
	return set, nil
}

// warm compiles the packages that .ci commands build inside their own
// per-test deadlines. The aggregate run and the two OSPF suites call it before
// any test starts, preserving the prerequisite that the retired Make targets
// supplied.
func (s *session) warm() error {
	if s.warmed {
		return nil
	}
	if err := s.warmFn(); err != nil {
		return err
	}
	s.warmed = true
	return nil
}

// release removes the set this invocation built, if it built one.
func (s *session) release() {
	if s.built {
		Release(s.set)
	}
}

// areaActions are the three actions that name a RUN LIST or a listing rather
// than a suite. They open the table because they are the three a reader who
// typed the area name is looking for.
//
// They are rows like every suite is a row. Reading them beside the table made
// the area publish three verbs its dispatch did not hold. `le test functional list
// zzprobe` answered "no such action in functional: list", and the help every
// refusal prints named the 32 suites and left these three out.
//
// Each names the whole area, so each is declared Alone: `gating` is 24 suites
// and `list` and `select` are the catalog and the run list. A line naming one
// of them beside a suite is refused, which is what the area answered before
// they were rows.
func areaActions(s *session) []leaction.Action {
	return []leaction.Action{
		{Verb: listVerb, Why: "every suite and its budget", Alone: true, Answer: s.runList},
		{Verb: gatingVerb, Why: "every gating suite, under its own budget", Alone: true,
			Answer: s.runGating},
		{Verb: selectVerb,
			Why:    "the suites a gating run would start for this checkout, and why the rest are absent",
			Alone:  true,
			Answer: s.runSelect},
	}
}

// runList answers the suite table, and reads neither the checkout nor the
// toolchain to do it.
func (s *session) runList() (any, int) { return Catalog(), 0 }

// runSelect answers the run list a gating run would start for this checkout,
// and runs nothing. An operator whose run started fewer suites than they
// expected reads here which suites the suite map ruled out and why.
func (s *session) runSelect() (any, int) {
	root, err := s.checkout()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	plan, err := planRun(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return plan.Report, 0
}

// runGating runs the gating suites under their own budgets.
func (s *session) runGating() (any, int) {
	tc, err := s.toolchain()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runGating(tc)
}

// table builds this invocation's action table: the three area actions, one row
// per suite, and the verifier-only ExaBGP action.
func table(s *session) leaction.Area {
	rows := make([]leaction.Action, 0, len(Suites)+len(areaActions(s))+1)
	rows = append(rows, areaActions(s)...)
	for _, suite := range Suites {
		rows = append(rows, leaction.Action{
			Verb:   suite.Name,
			Why:    suite.Why,
			Answer: s.suiteRunner(suite),
		})
	}
	rows = append(rows, leaction.Action{
		Verb:   "exabgp-test",
		Why:    "build the isolated ze and le test subjects and run every ExaBGP compatibility case",
		Answer: s.runExaBGP,
	})
	return leaction.New(Area, rows...)
}

// suiteRunner answers the action that runs one suite under its own cap.
func (s *session) suiteRunner(suite Suite) func() (any, int) {
	return func() (any, int) {
		tc, err := s.toolchain()
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		if suite.Warm {
			if err := s.warm(); err != nil {
				leaction.ReportError(err)
				return nil, 1
			}
		}
		set, err := s.binaries()
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		covers, err := coverRoot(tc.Root)
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		cover, reduce := suiteCoverage(tc, suite, covers)
		code, seconds := Execute(tc, suite, set, cover)
		reduce()
		return SuiteRun{
			Suite:   suite.Name,
			Budget:  suite.Budget(),
			Seconds: seconds,
			Code:    code,
			Expired: code == killedByBudget,
		}, code
	}
}

func (s *session) runExaBGP() (any, int) {
	root, err := s.checkout()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runExaBGP(context.Background(), root, nil)
}

// Actions answers the command surface as data, and is what a bare command line
// prints. It is the table and nothing beside it, so a verb a reader sees here
// is a verb this area dispatches.
func Actions() leaction.List { return table(&session{}).Actions() }

// Subs is the one-line hint help renders under the command.
//
// The hint shows the shape of a suite verb instead of all 32 verbs. This area
// differs because leaction usually derives a hint by naming every action. A
// list of 32 names is not a useful hint. The next keyword reveals those names.
func Subs() string {
	var tb textbuf.Buffer
	for _, action := range areaActions(&session{}) {
		tb.Str(action.Verb).Str(" (").Str(action.Why).Str(") | ")
	}
	return tb.Str("<suite>-test | exabgp-test").String()
}

// Answer is the `le test functional` command.
//
// One table dispatches, lists, helps and refuses, so nothing here reads the
// command line. A line naming one action runs that action. A line naming
// several sweeps them in the order they were typed, and stops at the first
// failure.
func Answer(args []string) (any, int) {
	// A bare command line answers the verbs and builds nothing (owner directive,
	// 2026-09-02). A developer who types the area name starts no run and waits
	// on no toolchain probe. The listing names `gating` as the run that name
	// used to start. `list` keeps the suite table, which answers what a suite
	// costs.
	if len(args) == 0 {
		return Actions(), 0
	}

	current := newSession(args)
	defer current.release()
	return table(current).AnswerOrSweep(args, leaction.StopAtFirstFailure)
}

// newSession reads the whole command line before anything runs.
// The binary set therefore carries the label and the extra binaries the named
// suites drive.
func newSession(args []string) *session {
	named := make([]Suite, 0, len(args))
	for _, verb := range args {
		if suite, ok := suiteForVerb(verb); ok {
			named = append(named, suite)
		}
	}

	current := &session{label: "functional"}
	if len(named) == 1 {
		current.label = named[0].Name
	}
	// Both closures ask for the toolchain rather than reading current.tc.
	// The field is filled lazily, behind s.probed, so reading it directly
	// hands Prepare and warmCITestPackages a zero Toolchain with an empty
	// Root whenever binaries() or warm() runs before toolchain() does. That
	// is a value neither of them can tell from a real answer, and only the
	// statement order inside suiteRunner kept it populated: an obligation
	// nothing named and no test held.
	current.buildFn = func() (BinarySet, error) {
		tc, err := current.toolchain()
		if err != nil {
			return BinarySet{}, err
		}
		return Prepare(tc, current.label)
	}
	current.warmFn = func() error {
		tc, err := current.toolchain()
		if err != nil {
			return err
		}
		return warmCITestPackages(tc)
	}
	return current
}

// suiteForVerb accepts the bare suite names printed by the failure rerun and
// their historical "-test" aliases.
func suiteForVerb(verb string) (Suite, bool) {
	name, isAlias := strings.CutSuffix(verb, "-test")
	if !isAlias {
		name = verb
	}
	return SuiteNamed(name)
}
