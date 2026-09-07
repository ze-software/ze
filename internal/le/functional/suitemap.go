// Design: docs/architecture/testing/verify-freshness-scope.md -- the suite map's contract and its fail-open rule
// Detail: run.go -- the gating run that consults the answer
// Overview: suites.go -- the suite names a recorded set is keyed by
//
// suitemap.go reads the SUITE MAP, which is the recorded answer to one
// question: which Go packages did each functional suite reach when it last ran?
// A gating run consults it to leave out the suites a change set cannot reach.
//
// EVERY route that cannot answer WIDENS to every suite. An absent map, an
// unreadable one, a truncated one, a map that records no commit, and a suite
// this run knows nothing about each mean "I could not tell", so the run stays
// the full one. The map is read from a file under tmp/ that several sessions
// share, so a malformed map MUST widen and MUST NOT narrow.
//
// A caller cannot read a widening as an empty selection. suiteSelection carries
// a verdict whose zero value is neither answer, and runs reports true for every
// verdict except the one that names the suites.
//
// A suite that recorded an EMPTY package set is a REFUSAL, not a suite that
// covers nothing. Zero suites selected is a valid answer, because a docs-only
// change reaches none. Zero packages under a suite means the recording broke,
// and reading it the other way would skip that suite for ever.

package functional

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// suiteMapPath is where the suite map lives, relative to the checkout root.
//
// It is derived and never committed: a gating run rewrites it, and its absence
// costs the behavior every run has today. It sits beside the other derived
// verification artifacts under tmp/ rather than in a session scratch directory,
// because the run that records it and the run that reads it are two sessions.
const suiteMapPath = "tmp/ze-suite-map.json"

// suiteMap is what a recording gating run observed.
//
// Head is the commit the recording ran at. A reader needs it to ask which files
// moved since, because a suite that newly reaches a package is what makes a map
// stale, and a map with no commit can answer nothing.
//
// Reached names, for each suite, every package that suite reached. The package
// spelling is the change-set selector's own: "./internal/component/ssh", rooted
// at the checkout, so an answer from `./le changed packages` can be compared
// with a recorded set without either side normalizing the other.
type suiteMap struct {
	Head    string              `json:"head"`
	Reached map[string][]string `json:"reached"`
}

// errNoSuiteMap says this checkout holds no recorded map. It is the ordinary
// state of a fresh checkout and of CI, so a caller widens rather than failing.
var errNoSuiteMap = errors.New("functional: this checkout records no suite map")

// readSuiteMap reads and validates the suite map for the checkout at root.
//
// Every unusable map is an error here rather than a thin answer, because the
// caller's only correct response to each of them is the same widening. A
// suiteMap returned with no error records a commit and at least one suite, and
// every suite it records holds at least one package.
func readSuiteMap(root string) (suiteMap, error) {
	path := filepath.Join(root, suiteMapPath)
	body, err := os.ReadFile(path) //nolint:gosec // a derived artifact at a path this package declares
	if errors.Is(err, os.ErrNotExist) {
		return suiteMap{}, errNoSuiteMap
	}
	if err != nil {
		return suiteMap{}, fmt.Errorf("functional: read the suite map at %s: %w", suiteMapPath, err)
	}

	var recorded suiteMap
	if err := json.Unmarshal(body, &recorded); err != nil {
		return suiteMap{}, fmt.Errorf("functional: the suite map at %s is malformed: %w", suiteMapPath, err)
	}
	if err := recorded.validate(); err != nil {
		return suiteMap{}, err
	}
	return recorded, nil
}

// validate answers what makes a parsed map unusable.
//
// Each refusal names the condition, because the reason reaches the run's stderr
// and it is what tells a reader to record the map again.
func (m suiteMap) validate() error {
	if strings.TrimSpace(m.Head) == "" {
		return fmt.Errorf("functional: the suite map at %s records no commit", suiteMapPath)
	}
	if len(m.Reached) == 0 {
		return fmt.Errorf("functional: the suite map at %s records no suite", suiteMapPath)
	}
	for suite, packages := range m.Reached {
		if len(packages) == 0 {
			return fmt.Errorf(
				"functional: the suite map at %s records no package for suite %s, so the recording broke",
				suiteMapPath, suite)
		}
		for _, name := range packages {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf(
					"functional: the suite map at %s records an unnamed package for suite %s",
					suiteMapPath, suite)
			}
		}
	}
	return nil
}

// suiteVerdict says what the suite map could answer about a gating run.
//
// The zero value is verdictUnspecified, so a suiteSelection nobody filled in
// can never read as "no suite runs".
type suiteVerdict uint8

const (
	// verdictUnspecified is the zero value and is never an answer.
	verdictUnspecified suiteVerdict = iota
	// verdictEverySuite says the map could not rule any suite out. It is what
	// every failure to answer widens to.
	verdictEverySuite
	// verdictSelected says Suites names the suites this run must run, and that
	// the map ruled the rest out.
	verdictSelected
)

// suiteSelection is what the suite map answered for one gating run.
//
// Suites is meaningful under verdictSelected alone. Under every other verdict
// it is empty and means nothing, which is why a caller asks runs rather than
// reading the slice.
//
// Reason says why, in one sentence, and the gating run prints it before the
// first suite starts.
type suiteSelection struct {
	Verdict suiteVerdict
	Suites  []string
	Reason  string
}

// runs reports whether the gating run must run the suite called name.
//
// Every verdict other than verdictSelected widens, the zero value included. A
// selection that could not answer therefore runs the suite, and an empty Suites
// slice can never subtract one.
func (s suiteSelection) runs(name string) bool {
	if s.Verdict != verdictSelected {
		return true
	}
	return slices.Contains(s.Suites, name)
}

// everySuite is the widening, and it is the only way to build one.
func everySuite(reason string) suiteSelection {
	return suiteSelection{Verdict: verdictEverySuite, Reason: reason}
}

// selectSuites answers which suites the gating run at root must run.
//
// Every answer today is verdictEverySuite. The map is read and validated, and
// no change set is intersected with it yet, so a recorded map subtracts no
// suite and an absent one costs nothing.
func selectSuites(root string) suiteSelection {
	recorded, err := readSuiteMap(root)
	if err != nil {
		return everySuite(err.Error())
	}
	var tb textbuf.Buffer
	return everySuite(tb.Str("the suite map records ").Int(int64(len(recorded.Reached))).
		Str(" suite(s) at commit ").Str(recorded.Head).
		Str(", and no change set is compared against it yet").String())
}

// gatingRunList splits the gating suites into the ones this run runs and the
// ones it leaves out.
//
// ZE_SKIP_SUITES wins over the map. An operator override is absolute, so a
// recorded map can only ever subtract a suite and never add a skipped one back.
func gatingRunList(suites []Suite, skip map[string]bool, selection suiteSelection) (running, skipped []Suite) {
	running = make([]Suite, 0, len(suites))
	for _, suite := range suites {
		if skip[suite.Name] {
			skipped = append(skipped, suite)
			continue
		}
		if selection.runs(suite.Name) {
			running = append(running, suite)
		}
	}
	return running, skipped
}
