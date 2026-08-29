// Design: docs/architecture/core-design.md -- le's spec inventory
//
// Package specstatus reads the metadata table at the top of every plan/spec-*.md
// and answers the inventory `./le spec status` prints: one record per spec,
// carrying its status, its bucket, and whether a skeleton is past its TTL.
//
// It is the port of internal/le/spec/status/answer.go together with the two leaf
// packages that file kept beside it. Those two existed for one reason, stated in
// their own headers: the front end was a single-file `go run` script excluded
// from its own directory's package, so nothing could import it and its logic had
// to live somewhere a test could reach. A compiled package has no such problem,
// so the three are one here.
package specstatus

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Bucket names. The split separates committed backlog (work someone chose to
// start) from idea capture (skeleton stubs that are a title plus a template and
// may never be developed). Counting the two together inflates the apparent open
// backlog.
const (
	Backlog = "backlog" // committed work: design / ready / in-progress / verification
	Idea    = "idea"    // idea capture: skeleton stubs
	Other   = "other"   // blocked / deferred / done / unknown / unparsed
)

// SkeletonTTLWeeks is how long a skeleton may sit untouched before it is flagged
// for triage. Six weeks: long enough to survive a normal planning lull, short
// enough that a two-month-old stub trips it.
const SkeletonTTLWeeks = 6

// SkeletonTTLDays is the TTL expressed in days for age arithmetic.
const SkeletonTTLDays = SkeletonTTLWeeks * 7

// The statuses this package acts on. Naming them makes the three enumerations
// below -- the bucket, the sort key and the reporting position -- obviously
// about one vocabulary, and turns a typo in any of them into a compile error
// rather than a spec that is silently bucketed somewhere else.
//
// This is NOT the status vocabulary. That lives in ai/rules/planning.md and in
// the case statement of .claude/hooks/validate-spec.sh, and a third copy here
// would drift from both. A status named nowhere here is still counted, still
// bucketed and still printed: every reader below has a default.
const (
	// statusUnparsed is reported for a spec that carries no metadata table. It
	// is distinct from statusUnknown, which means the table was read and the
	// Status row was absent from it.
	statusUnparsed     = "unparsed"
	statusUnknown      = "unknown"
	statusInProgress   = "in-progress"
	statusVerification = "verification"
	statusReady        = "ready"
	statusDesign       = "design"
	statusSkeleton     = "skeleton"
	statusBlocked      = "blocked"
	statusDeferred     = "deferred"
)

// specGlob is the population: every spec file directly under plan/. It does not
// recurse, and plan/future/ and plan/to-review/ are therefore counted nowhere
// (plan/journal/gate-excludes-part-of-its-population.md, 2026-08-22).
const specGlob = "plan/spec-*.md"

// planDir is the directory the population lives in. Collect refuses a tree that
// does not hold it: see the comment on Collect.
const planDir = "plan"

// Category maps a spec status to its inventory bucket. A status named nowhere
// here lands in Other, which is correct for a terminal state such as `done` and
// for an unreadable spec.
func Category(status string) string {
	switch status {
	// `verification` is committed work waiting on a reviewer
	// (ai/rules/planning.md tells the implementing session to set it before it
	// commits), so it belongs with the work someone chose to start. Counting it
	// as Other filed finished-and-unreviewed work beside blocked and deferred,
	// which is where a reader looks for work nobody is carrying.
	case statusInProgress, statusVerification, statusReady, statusDesign:
		return Backlog
	case statusSkeleton:
		return Idea
	default:
		return Other
	}
}

// skeletonStale reports whether a skeleton last updated on `updated`
// (YYYY-MM-DD) is past the TTL relative to `now`. At exactly the TTL it is not
// yet stale (the flag boundary); beyond it, it is. An unparseable date is
// treated as not stale: it cannot be judged, and a false flag trains the reader
// to ignore the flag. TTL never deletes -- it promotes-or-flags; deletion stays
// a human action.
func skeletonStale(updated string, now time.Time) bool {
	t, err := time.Parse("2006-01-02", updated)
	if err != nil {
		return false
	}
	ageDays := now.Sub(t).Hours() / 24
	return ageDays > float64(SkeletonTTLDays)
}

// headerRE matches the header row of a spec's metadata table.
var headerRE = regexp.MustCompile(`^\|\s*Field\s*\|\s*Value\s*\|`)

// metaRows returns the body rows of a spec's metadata table and reports whether
// that table was found.
//
// The scan is anchored on the "| Field | Value |" header row rather than on a
// line count, because plan/TEMPLATE.md opens with a six-line authoring comment
// that pushes the table past any fixed window. It stops at the first "## "
// heading and at the first line that leaves the table, so the trailing header
// rows of the Assumptions, TDD and Interop tables further down the file are
// never matched.
func metaRows(content string) ([]string, bool) {
	var rows []string
	found := false
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, "## ") {
			break
		}
		if !found {
			if headerRE.MatchString(line) {
				found = true
			}
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			break
		}
		rows = append(rows, line)
	}
	return rows, found
}

// metaField returns the value of one metadata field from the rows metaRows
// returned. The rows look like "| Status | design |". It returns "" when no row
// names the field.
//
// The pattern is built for each call, which is the shape the script this ports
// had and which is preserved: four fields over 228 specs is about 900 compiles
// for a whole run, against one git process per spec beside it. The git calls are
// three orders of magnitude more expensive, so caching the four patterns would
// buy nothing a reader could measure and would cost a second place for the field
// names to live.
func metaField(rows []string, field string) string {
	var tb textbuf.Buffer
	pattern := regexp.MustCompile(tb.Str(`^\|\s*`).Str(regexp.QuoteMeta(field)).Str(`\s*\|\s*([^|]*?)\s*\|`).String())
	for _, line := range rows {
		if m := pattern.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

// statusOrder returns the sort key for a status (lower = sorted first). A
// status named nowhere here sorts last, and it is still counted and printed.
func statusOrder(status string) int {
	switch status {
	case statusUnparsed:
		// Sorted first: a spec the inventory cannot read is the one row a
		// reader must act on, and burying it reads as "nothing to see".
		return 0
	case statusInProgress:
		return 1
	case statusVerification:
		// Committed work waiting on a reviewer, so it sorts beside in-progress
		// rather than beside blocked. ai/rules/planning.md tells the
		// implementing session to set this status before it commits.
		return 2
	case statusReady:
		return 3
	case statusDesign:
		return 4
	case statusSkeleton:
		return 5
	case statusBlocked:
		return 6
	case statusDeferred:
		return 7
	}
	return 9
}

// setRE matches the spec-set pattern in a filename: "spec-<prefix>-<N>-<name>.md".
var setRE = regexp.MustCompile(`^spec-([a-z]+(?:-[a-z]+)*)-\d+-.*\.md$`)

// detectSet returns the spec set from the filename pattern, or "-" if no match.
func detectSet(filename string) string {
	m := setRE.FindStringSubmatch(filename)
	if m == nil {
		return "-"
	}
	return m[1]
}

// gitDate returns git's last-modified date for a file (YYYY-MM-DD), falling
// back to "unknown" if git is unavailable or the file is untracked.
//
// "unknown" is an ANSWER rather than a swallowed error: the inventory reports on
// a tree whose specs are routinely untracked, so a spec git has never seen is
// the common case and not a failure to read anything.
func gitDate(ctx context.Context, root, rel string) string {
	//nolint:gosec // fixed binary; rel is a path the plan/ glob produced
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--format=%as", "--", rel)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return statusUnknown
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return statusUnknown
	}
	return s
}

// loadSpec reads metadata from a single spec file. rel is the path relative to
// root, which is what git is asked about and what an unreadable-spec warning
// names, so the warning reads the same however the caller reached the tree.
func loadSpec(ctx context.Context, root, rel string, warn func(string)) (Spec, error) {
	//nolint:gosec // the path comes from the plan/ glob under the repository root
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return Spec{}, err
	}
	content := string(data)
	base := path.Base(rel)
	name := strings.TrimSuffix(strings.TrimPrefix(base, "spec-"), ".md")

	rows, found := metaRows(content)
	s := Spec{
		Name:        name,
		Status:      metaField(rows, "Status"),
		Depends:     metaField(rows, "Depends"),
		Phase:       metaField(rows, "Phase"),
		Updated:     metaField(rows, "Updated"),
		Set:         detectSet(base),
		GitModified: gitDate(ctx, root, rel),
	}
	// Fail closed: a spec with no metadata table at all is an authoring error,
	// not a spec whose Status row is merely absent. Reporting both as "unknown"
	// dresses a zero-information answer as data, so the two are kept distinct
	// and the unreadable one names itself.
	if !found {
		var tb textbuf.Buffer
		warn(tb.Str("spec-status: ").Str(rel).Str(" has no '| Field | Value |' metadata table").String())
		s.Status = statusUnparsed
	}
	if s.Status == "" {
		s.Status = statusUnknown
	}
	if s.Depends == "" {
		s.Depends = "-"
	}
	if s.Phase == "" {
		s.Phase = "-"
	}
	if s.Updated == "" {
		s.Updated = s.GitModified
	}
	return s, nil
}

// Collect reads every spec under root's plan/ directory and answers the
// inventory, sorted by status order and then by updated date descending.
//
// It REFUSES a tree that holds no plan/ directory, and that is the one
// behavioral difference from the script it ports. filepath.Glob answers an
// empty list and no error for a pattern whose directory does not exist, so the
// script printed "Specs: 0 total" and exited 0 over any tree it was not standing
// in -- an inventory of a population it never read. An EMPTY plan/ is a
// different fact and stays an answer: every spec can legitimately be closed.
//
// now is a parameter because the skeleton TTL is the one judgement here that
// reads a clock, and a caller that cannot fix the clock cannot test the flag.
func Collect(ctx context.Context, root string, now time.Time, warn func(string)) (Inventory, error) {
	if warn == nil {
		warn = func(string) {}
	}
	info, err := os.Stat(filepath.Join(root, planDir))
	if err != nil {
		return nil, fmt.Errorf("read the spec population under %s: %w", specGlob, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("read the spec population under %s: %s is not a directory", specGlob, planDir)
	}

	matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(specGlob)))
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", specGlob, err)
	}

	specs := make(Inventory, 0, len(matches))
	for _, match := range matches {
		base := filepath.Base(match)
		if base == specTemplateFile {
			continue
		}
		rel := path.Join(planDir, base)
		s, err := loadSpec(ctx, root, rel, warn)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", rel, err)
		}
		specs = append(specs, s)
	}

	for i := range specs {
		specs[i].Bucket = Category(specs[i].Status)
		specs[i].Stale = specs[i].Bucket == Idea && skeletonStale(specs[i].Updated, now)
	}
	sort.SliceStable(specs, func(i, j int) bool {
		oi, oj := statusOrder(specs[i].Status), statusOrder(specs[j].Status)
		if oi != oj {
			return oi < oj
		}
		// Updated date descending.
		return specs[i].Updated > specs[j].Updated
	})
	return specs, nil
}

// specTemplateFile is the spec template every new spec starts from.
const (
	specTemplateFile = "spec-template.md"
)
