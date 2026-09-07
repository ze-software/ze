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
// The same file writes it. publish records what one WHOLE gating run reached,
// through a temporary and a rename, so a session reading the artifact meets the
// old map or the new one and never a half-written one.
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
	"github.com/ze-software/ze/internal/le/job"
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

// suiteRecording is what one gating run observed, before it is published.
//
// An entry exists for every suite the run REDUCED, and its value is the
// packages that suite reached. An entry with no package is a suite that
// recorded nothing, which is a real state four suites are always in.
type suiteRecording struct {
	head    string
	reached map[string][]string
}

// newSuiteRecording starts a recording for a run sitting on the commit head.
//
// The commit is read BEFORE the first suite starts, so it names the tree the
// suites ran against. A run takes an hour and this checkout is shared, so the
// tree can move under it; reading the commit at the end would claim the map
// describes a tree no suite ever ran. Naming the earlier commit is the
// conservative direction, because a reader treats every package touched since
// as unknown and widens.
func newSuiteRecording(head string) *suiteRecording {
	return &suiteRecording{head: head, reached: map[string][]string{}}
}

// record keeps what one finished suite reached.
//
// An empty set is kept as one rather than dropped. publish tells a suite that
// recorded nothing from a suite this run never ran, and it can only do that if
// both states are visible here.
func (r *suiteRecording) record(suite string, packages []string) {
	r.reached[suite] = packages
}

// publish writes the suite map for a run that reduced every gating suite, and
// answers the suites it left out.
//
// ONLY A WHOLE GATING RUN MAY WRITE THE ARTIFACT. A suite the map does not name
// is unknown to the reader and therefore always runs, so a map written by a run
// that covered one suite would declare every other suite unknown while the next
// run narrowed on the one it named. A partial run is refused here rather than
// trusted not to call this, which is why the loop reads Gating rather than the
// recording.
//
// A suite that recorded nothing is OMITTED, never written with an empty set.
// The two are different answers: omitted means "this run learned nothing about
// that suite, so run it", and an empty set read back would mean "this suite
// covers nothing, so never run it again". editor, web, runner and policy record
// nothing every time, because they run the harness rather than an instrumented
// ze, or skip unprivileged.
func (r *suiteRecording) publish(root string) (omitted []string, err error) {
	if r.head == "" || r.head == job.Unknown {
		return nil, errors.New(
			"functional: this run cannot name the commit it ran at, so it publishes no suite map")
	}

	published := suiteMap{Head: r.head, Reached: map[string][]string{}}
	for _, name := range Gating {
		packages, reduced := r.reached[name]
		if !reduced {
			return nil, fmt.Errorf(
				"functional: this run did not run the gating suite %s, so only a full run can publish a suite map",
				name)
		}
		if len(packages) == 0 {
			omitted = append(omitted, name)
			continue
		}
		published.Reached[name] = packages
	}

	// The writer's own output has to satisfy the reader, and validate is where
	// that contract is declared. It is what refuses a run in which no suite
	// recorded anything at all.
	if err := published.validate(); err != nil {
		return omitted, err
	}
	body, err := json.MarshalIndent(published, "", "  ")
	if err != nil {
		return omitted, fmt.Errorf("functional: render the suite map: %w", err)
	}
	return omitted, atomicWrite(filepath.Join(root, suiteMapPath), append(body, '\n'))
}

// atomicWrite publishes one derived artifact through a temporary file in the
// same directory and one rename.
//
// tmp/ is shared by several sessions, and a reader that met a half-written map
// would have to tell it from a valid one. A rename means the reader sees the
// old map or the new one. This is the pattern atomicWrite
// (internal/le/verify/engine/status.go) uses for the verification artifacts,
// held here because that helper is private to its own package.
func atomicWrite(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".suite-map-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()

	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
