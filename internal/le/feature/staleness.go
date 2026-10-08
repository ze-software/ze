// Design: docs/contributing/feature-maturity.md -- attestation staleness and interop resolution
// Related: check.go -- the verdicts these criteria write into
//
// Two attestations go stale by date. A Doc review older than the newest change
// to a listed Docs page or to the declaration itself is refused outright, at any
// level (owner decision D-8: a known-false sentence is never published, so the
// commit waits for the prose fix or a fresh review). A Defect review older than
// the newest journal row naming a Components path owes a re-review before
// Supported (AC-8).
//
// A change date is the committer date of the newest commit touching the path,
// and today for a path with uncommitted changes. A shallow checkout holds no
// history to read that date from, so it is refused rather than answered.

package feature

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// gitDeadline bounds one git query.
const gitDeadline = time.Minute

// changeDates answers, per repository path, the date it last changed. Not safe
// for concurrent use.
type changeDates struct {
	tree string
	// today is the day the check judges on, a UTC calendar day: the change
	// date of a dirty path and the age of a recorded run (runrecord.go).
	today  time.Time
	cached map[string]string
}

func newChangeDates(tree string, today time.Time) (*changeDates, error) {
	shallow, err := gitOutput(tree, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return nil, err
	}
	if shallow == "true" {
		return nil, errors.New("the checkout is shallow, so no change date can be read and no Doc review " +
			"staleness can be judged; fetch the full history (fetch-depth: 0)")
	}
	return &changeDates{tree: tree, today: today, cached: map[string]string{}}, nil
}

// of answers the date rel last changed, YYYY-MM-DD.
func (c *changeDates) of(rel string) (string, error) {
	if date, held := c.cached[rel]; held {
		return date, nil
	}
	dirty, err := gitOutput(c.tree, "status", "--porcelain", "--", rel)
	if err != nil {
		return "", err
	}
	date := c.today.Format(attestationLayout)
	if dirty == "" {
		committed, err := gitOutput(c.tree, "log", "-1", "--format=%cs", "--", rel)
		if err != nil {
			return "", err
		}
		if committed != "" {
			date = committed
		}
	}
	c.cached[rel] = date
	return date, nil
}

func gitOutput(tree string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitDeadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", tree}, args...)...) //nolint:gosec // args are fixed git verbs and paths a declaration lists
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("git " + args[0] + ": " + err.Error())
	}
	return strings.TrimSpace(string(out)), nil
}

// staleDocReview is D-8(b): a refusal at any level, naming the newer change.
func (in *evidence) staleDocReview(d *Declaration, verdict *Verdict) {
	if !d.DocReview.Present() {
		return
	}
	reviewed := d.DocReview.Date.Format(attestationLayout)
	paths := append([]string{declarationDir + "/" + d.ID + ".md"}, d.Docs...)
	for _, rel := range paths {
		changed, err := in.dates.of(rel)
		if err != nil {
			verdict.Refusals = append(verdict.Refusals, "Doc review staleness of "+rel+" cannot be judged: "+err.Error())
			continue
		}
		if changed > reviewed {
			verdict.Refusals = append(verdict.Refusals, "Doc review "+reviewed+" is older than the change to "+
				rel+" on "+changed+": fix the prose or redo the review before this lands")
		}
	}
}

// staleDefectReview is AC-8: a journal row newer than the Defect review that
// names a Components path owes a re-review before Supported.
func (in *evidence) staleDefectReview(d *Declaration, verdict *Verdict) {
	if !d.DefectReview.Present() {
		return
	}
	reviewed := d.DefectReview.Date.Format(attestationLayout)
	for _, row := range in.journal {
		if row.Date <= reviewed {
			continue
		}
		cells := row.Surface + " " + row.Symptom + " " + row.Fix
		for _, component := range d.Components {
			if !strings.Contains(cells, component) {
				continue
			}
			verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported],
				"S5: re-review owed: journal class "+row.Class+" row "+row.Date+" names "+component+
					", after the Defect review of "+reviewed)
			break
		}
	}
}

// catalog answers the scenarios of one interop suite, name -> directory
// relative to the tree, read once per run.
func (in *evidence) catalog(suite string) (map[string]string, bool) {
	if names, held := in.scenarios[suite]; held {
		return names, true
	}
	catalog, registered := interoplab.CatalogNamed(suite)
	if !registered {
		return nil, false
	}
	names := map[string]string{}
	sources, err := catalog.Scenarios(in.tree)
	if err != nil {
		in.catalogErrors[suite] = err.Error()
	}
	for _, source := range sources {
		rel, err := filepath.Rel(in.tree, source.Directory)
		if err != nil {
			in.catalogErrors[suite] = err.Error()
			continue
		}
		names[source.Name] = filepath.ToSlash(rel)
	}
	in.scenarios[suite] = names
	return names, true
}

// interopItem answers why an Interop entry `<suite>/<scenario>` does not
// resolve through its suite's catalog, or "" (AC-4).
func (in *evidence) interopItem(item string) string {
	suite, scenario, found := strings.Cut(item, "/")
	if !found {
		return "'" + item + "' is not <suite>/<scenario>"
	}
	names, registered := in.catalog(suite)
	if !registered {
		return "'" + item + "': no interop suite '" + suite + "' is registered; suites: " +
			strings.Join(interoplab.CatalogSuites(), ", ")
	}
	if problem, failed := in.catalogErrors[suite]; failed {
		return "'" + item + "': suite " + suite + " cannot list its scenarios: " + problem
	}
	if _, listed := names[scenario]; !listed {
		return "'" + item + "': suite " + suite + " runs no scenario named '" + scenario + "'"
	}
	return ""
}

// isInteropPointer reports whether pointer names an interop entry: a
// `<suite>/...` whose suite is a registered catalog.
func (in *evidence) isInteropPointer(pointer string) bool {
	suite, _, found := strings.Cut(pointer, "/")
	if !found {
		return false
	}
	_, registered := in.catalog(suite)
	return registered
}

// interopRun answers why the Interop entry item of feature id has no current
// recorded green run, or "" (D-6 as the owner reads it: interop scenarios
// too), with the run answer it read. An entry that does not resolve answers "":
// interopItem refused it. A problem the record could not answer carries a run
// answer of runStateUnspecified, which is never stale.
func (in *evidence) interopRun(id, item string) (string, runAnswer) {
	if in.interopItem(item) != "" {
		return "", runAnswer{}
	}
	suite, scenario, _ := strings.Cut(item, "/")
	names, _ := in.catalog(suite)
	tree, err := scenarioTreeID(in.tree, names[scenario])
	if err != nil {
		return item + ": " + err.Error(), runAnswer{}
	}
	record, err := loadRunRecord(in.tree, id, in.dates.today)
	if err != nil {
		return err.Error(), runAnswer{}
	}
	answer := record.scenarioStateOf(item, tree, in.dates.today)
	return answer.problem(item, "scenario", runRecordRel(id)), answer
}

// countedInterop answers the Interop entries that count toward a level: every
// one not listed in Stub evidence.
func countedInterop(d *Declaration) []string {
	var counted []string
	for _, item := range d.Interop {
		if !slices.Contains(d.StubEvidence, item) {
			counted = append(counted, item)
		}
	}
	return counted
}

// criterionInterop is S2: a protocol feature reaches Supported only with at
// least one non-stub interop scenario, and every non-stub one needs a recorded
// green run of the scenario as it is now (D-6). For the other kinds S2 is not
// an interop-lab scenario (spec, per-Kind table), so a feature that needs it
// carries it as an extra criterion.
func (in *evidence) criterionInterop(d *Declaration, verdict *Verdict) {
	if d.Kind != KindProtocol {
		return
	}
	counted := countedInterop(d)
	if len(counted) == 0 {
		verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported],
			"S2: no non-stub interop scenario is listed for a protocol feature")
		return
	}
	for _, item := range counted {
		problem, answer := in.interopRun(d.ID, item)
		if problem == "" {
			continue
		}
		if in.warnStale(d, verdict, answer, "S2: "+problem) {
			continue
		}
		verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported], "S2: "+problem)
	}
}
