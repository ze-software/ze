// Design: docs/architecture/core-design.md -- the functional area
// Detail: run.go -- the run the bare command reaches
// Overview: suites.go -- the table every row here is derived from
//
// Every action is derived from the suite table, so dispatch, listing, help, and
// completion share one command surface. A bare `le functional` runs the gating
// suites. `le functional list` returns the complete suite catalog, including
// suites that do not run in the aggregate.
//
// One isolated binary set serves an invocation and is built lazily. The label
// and chaos decision are derived before the first suite runs.

package functional

import (
	"context"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// listVerb prints the suite table instead of running anything. It is a keyword
// rather than a flag, because the rendering is a pipe operator and the tree is
// the checkout (ai/rules/cli.md).
const listVerb = "list"

// session is one invocation of the functional area: the toolchain it derived,
// and the isolated binary set it builds at most once.
type session struct {
	tc      gotoolchain.Toolchain
	set     BinarySet
	built   bool
	warmed  bool
	label   string
	chaos   bool
	buildFn func() (BinarySet, error)
	warmFn  func() error
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

// table builds this invocation's action table: one row per suite and the
// verifier-only ExaBGP action.
func table(s *session) leaction.Area {
	rows := make([]leaction.Action, 0, len(Suites)+1)
	for _, suite := range Suites {
		rows = append(rows, leaction.Action{
			Verb:   suite.Name,
			Why:    suite.Why,
			Answer: s.suiteRunner(suite),
		})
	}
	rows = append(rows, leaction.Action{
		Verb:   "exabgp-test",
		Why:    "build the isolated ze and ze-test subjects and run every ExaBGP compatibility case",
		Answer: s.runExaBGP,
	})
	return leaction.New(Area, rows...)
}

// suiteRunner answers the action that runs one suite under its own cap.
func (s *session) suiteRunner(suite Suite) func() (any, int) {
	return func() (any, int) {
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
		code, seconds := Execute(s.tc, suite, set, "")
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
	return runExaBGP(context.Background(), s.tc.Root, nil)
}

// Actions answers the command surface as data.

func Actions() leaction.List { return table(&session{}).Actions() }

// Subs is the one-line hint help renders under the command.
//
// The hint shows the shape of a verb instead of all 32 verbs.
// This area differs because leaction usually derives a hint by naming every action.
// A list of 32 names is not a useful hint. The next keyword reveals those names.
func Subs() string {
	var tb textbuf.Buffer
	return tb.Str(listVerb).Str(" (every suite and its budget) | <suite>-test | ").
		Str("exabgp-test").String()
}

// Answer is the `le functional` command.
func Answer(args []string) (any, int) {
	if len(args) == 1 && args[0] == listVerb {
		return Catalog(), 0
	}

	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	tc, err := gotoolchain.New(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	if len(args) == 0 {
		return runGating(tc)
	}

	current := newSession(tc, args)
	defer current.release()
	return table(current).Sweep(args, leaction.StopAtFirstFailure)
}

// newSession reads the whole command line before anything runs.
// The binary set therefore carries the label and chaos dashboard for the named suites.
func newSession(tc gotoolchain.Toolchain, args []string) *session {
	named := make([]Suite, 0, len(args))
	for _, verb := range args {
		if suite, ok := suiteForVerb(verb); ok {
			named = append(named, suite)
		}
	}

	current := &session{tc: tc, label: "functional"}
	if len(named) == 1 {
		current.label = named[0].Name
	}
	for _, suite := range named {
		if suite.Chaos {
			current.chaos = true
		}
	}
	current.buildFn = func() (BinarySet, error) {
		return Prepare(current.tc, current.label, current.chaos)
	}
	current.warmFn = func() error {
		return warmCITestPackages(current.tc)
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
