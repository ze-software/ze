// Design: docs/architecture/core-design.md -- le's spec inventory
//
// Package specstatus reads the metadata table at the top of every spec
// specpath names and answers the inventory `./le spec status` prints: one
// record per spec, carrying its release bucket, its status, its status
// category, and whether a skeleton is past its TTL.
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
	"github.com/ze-software/ze/internal/le/spec/specpath"
)

// Category names. The split separates committed backlog (work someone chose to
// start) from idea capture (skeleton stubs that are a title plus a template and
// may never be developed). Counting the two together inflates the apparent open
// backlog.
//
// A category is derived from a spec's STATUS. It is not the release bucket,
// which is derived from the directory the spec sits in and is declared once in
// internal/le/spec/specpath.
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
// below -- the category, the sort key and the reporting position -- obviously
// about one vocabulary, and turns a typo in any of them into a compile error
// rather than a spec that is silently filed somewhere else.
//
// This is NOT the status vocabulary. That lives in ai/rules/planning.md and in
// the oneOf call that validates a spec's Status row
// (internal/le/hookruntime.validateSpecText), and a third copy here would drift
// from both. A status named nowhere here is still counted, still categorized
// and still printed: every reader below has a default.
//
// That default is the whole safety property, and it is what the session-start
// summary lacked until 2026-08-29: it kept a seven-name list of its own and
// counted only what the list named, so two `done` specs went unreported.
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

// Category maps a spec status to its inventory category. A status named nowhere
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
	name := specpath.Stem(base)

	// A path that reached here came from specpath.All or specpath.Find, so it
	// IS a spec in a bucket. An unplaceable one is a caller defect rather than
	// a spec filed as "after": say so instead of naming a bucket.
	bucket, placed := specpath.Bucket(rel)
	if !placed {
		return Spec{}, fmt.Errorf("%s is not a spec in any release bucket", rel)
	}

	rows, found := metaRows(content)
	s := Spec{
		Name:        name,
		Bucket:      bucket,
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

// StatusPhrases answers the population's per-status breakdown as
// "<count> <status>" phrases, in the print order the `./le spec status` summary
// line uses, WITHOUT consulting git.
//
// It exists beside Collect because the session-start hook prints this same
// breakdown and must stay fast. Collect calls gitDate for every spec, which is
// one git process per spec, and the hook needs no date at all.
//
// The hook previously kept its own seven-name status list and its own scan, so
// it dropped `done` silently: on 2026-08-29 it summarized 231 specs over counts
// summing to 229, the two missing ones being done-but-never-closed. That is the
// same under-report summaryOrder's comment records for 2026-08-22, in a second
// copy that the first fix did not reach.
func StatusPhrases(root string) ([]string, error) {
	specs, err := specpath.All(root)
	if err != nil {
		return nil, err
	}

	counts := map[string]int{}
	for _, rel := range specs {
		if path.Base(rel) == specTemplateFile {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // a spec path specpath matched under the repository root
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		rows, found := metaRows(string(data))
		status := metaField(rows, "Status")
		switch {
		case !found:
			// Same fail-closed split loadSpec makes: no table at all is an
			// authoring error, an absent Status row inside one is not.
			status = statusUnparsed
		case status == "":
			status = statusUnknown
		}
		counts[status]++
	}

	return statusPhrases(counts), nil
}

// Collect reads every spec in every release bucket and answers the inventory,
// sorted by status order and then by updated date descending.
//
// specpath declares which directories the population is, and the walk does not
// recurse below one, so plan/to-review/ is counted nowhere. Collect REFUSES a
// tree that holds no plan/ directory: see the comment on specpath.All.
//
// now is a parameter because the skeleton TTL is the one judgement here that
// reads a clock, and a caller that cannot fix the clock cannot test the flag.
func Collect(ctx context.Context, root string, now time.Time, warn func(string)) (Inventory, error) {
	if warn == nil {
		warn = func(string) {}
	}
	paths, err := specpath.All(root)
	if err != nil {
		return nil, err
	}

	specs := make(Inventory, 0, len(paths))
	for _, rel := range paths {
		if path.Base(rel) == specTemplateFile {
			continue
		}
		s, err := loadSpec(ctx, root, rel, warn)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", rel, err)
		}
		specs = append(specs, s)
	}

	for i := range specs {
		specs[i].Category = Category(specs[i].Status)
		specs[i].Stale = specs[i].Category == Idea && skeletonStale(specs[i].Updated, now)
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
