package specstatus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// R-1 (spec-le-is-a-ze-binary): the tests these replace drove a COMPILED
// spec_status.go through exec. Each case below names the old assertion it
// carries and what it now calls instead.
//
//	internal/le/spec/status/answer.go
//	  TestCategory                   -> TestCategorySplitsCommittedBacklogFromIdeaCapture
//	  TestSkeletonStaleBoundary      -> TestSkeletonStaleBoundary (same name, same cases)
//	internal/le/spec/status/answer.go
//	  TestRowsFindsTablePastAFixedWindow -> TestMetaRowsFindsTablePastAFixedWindow
//	  TestRowsStopsAtTheFirstHeading     -> TestMetaRowsStopsAtTheFirstHeading
//	  TestRowsReportsAMissingTable       -> TestMetaRowsReportsAMissingTable
//	internal/le/spec/status/closure_test.go
//	  TestSpecStatusBacklogSplit          -> TestCategorySplitsCommittedBacklogFromIdeaCapture
//	  TestSkeletonTTLBoundary             -> TestSkeletonStaleBoundary
//	  TestSpecStatusReportsAnUnreadableSpec -> TestCollectDistinguishesUnparsedFromUnknown
//	                                          (records + warning) and
//	                                          TestTextSortsTheUnreadableSpecFirst (order)
//	  TestSpecStatusSummaryCountsEverySpec  -> TestTextSummaryCountsEverySpec and
//	                                          TestTextFilesVerificationUnderCommittedBacklog
//
// The old file compiled the tool because its build tag kept it out of every
// package. What compiling bought was the ENTRY POINT: main, the plan/ glob and
// the exit code. That is not lost. Collect owns the glob, Answer owns the exit
// code, and both are called directly here; the built binary is still driven end
// to end by internal/le/parity/parity_test.go and by
// test/ui/le-spec-status-answers.ci.

// templateShapedSpec returns a spec of the shape plan/TEMPLATE.md produces: a
// title, a six-line HTML authoring comment, then the metadata table. That
// comment is what pushed the Status row past the old fixed ten-line window, so
// every spec written from the template read as "unknown" to this tool.
func templateShapedSpec(status, updated string) string {
	var sb strings.Builder
	sb.WriteString("# Spec: fixture\n\n")
	sb.WriteString("<!-- Authoring note line 1\n     line 2\n     line 3\n     line 4\n     line 5 -->\n\n")
	sb.WriteString("| Field | Value |\n|-------|-------|\n")
	sb.WriteString("| Status | ")
	sb.WriteString(status)
	sb.WriteString(" |\n| Depends | - |\n| Phase | - |\n| Updated | ")
	sb.WriteString(updated)
	sb.WriteString(" |\n\n## Task\n\nFixture prose.\n")
	return sb.String()
}

// planTree writes each named spec into a plan/ directory of its own and answers
// the root. The tree is outside any repository, so gitDate answers "unknown" for
// every fixture and the records stay the same on every machine.
func planTree(t *testing.T, specs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan"), 0o750); err != nil {
		t.Fatalf("create the fixture plan directory: %v", err)
	}
	for name, body := range specs {
		if err := os.WriteFile(filepath.Join(root, "plan", name), []byte(body), 0o600); err != nil {
			t.Fatalf("write the fixture spec %s: %v", name, err)
		}
	}
	return root
}

// collect runs the tool over a fixture tree and returns the records with every
// warning it wrote.
func collect(t *testing.T, root string, now time.Time) (Inventory, []string) {
	t.Helper()
	var warnings []string
	inventory, err := Collect(context.Background(), root, now, func(line string) {
		warnings = append(warnings, line)
	})
	if err != nil {
		t.Fatalf("collect the inventory: %v", err)
	}
	return inventory, warnings
}

// byName indexes the records so a case can assert about one spec.
func byName(in Inventory) map[string]Spec {
	m := make(map[string]Spec, len(in))
	for _, s := range in {
		m[s.Name] = s
	}
	return m
}

// fixedNow is the clock every case that does not test the TTL reads, so a
// skeleton fixture is never flagged by the day the suite runs.
var fixedNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// TestCategorySplitsCommittedBacklogFromIdeaCapture pins the committed-backlog
// vs idea-capture split.
//
// VALIDATES: the bucket a reader counts open work from.
// PREVENTS: finished-and-unreviewed work being filed beside blocked and
// deferred, which is where a reader looks for work nobody is carrying.
func TestCategorySplitsCommittedBacklogFromIdeaCapture(t *testing.T) {
	cases := map[string]string{
		"in-progress": Backlog,
		// Committed work waiting on a reviewer. ai/rules/planning.md tells the
		// implementing session to set it before it commits, so a spec sits here
		// between implementation and closure.
		"verification": Backlog,
		"ready":        Backlog,
		"design":       Backlog,
		"skeleton":     Idea,
		"blocked":      Other,
		"deferred":     Other,
		// Terminal: the work is finished, so it is not open backlog.
		"done":           Other,
		"unknown":        Other,
		statusUnparsed:   Other,
		"":               Other,
		"never-heard-of": Other,
	}
	for status, want := range cases {
		if got := Category(status); got != want {
			t.Errorf("Category(%q) = %q, want %q", status, got, want)
		}
	}
}

// TestSkeletonStaleBoundary pins the TTL flag boundary: not flagged at exactly
// the TTL, flagged one day past it, never flagged for a date it cannot read.
func TestSkeletonStaleBoundary(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := base.Format("2006-01-02")
	day := 24 * time.Hour

	atTTL := base.Add(time.Duration(SkeletonTTLDays) * day)
	if skeletonStale(updated, atTTL) {
		t.Errorf("at exactly TTL (%d days) must not be flagged", SkeletonTTLDays)
	}
	pastTTL := base.Add(time.Duration(SkeletonTTLDays+1) * day)
	if !skeletonStale(updated, pastTTL) {
		t.Errorf("one day past TTL (%d days) must be flagged", SkeletonTTLDays+1)
	}
	// An unparseable date cannot be judged and must not be flagged: flagging
	// noise trains the reader to ignore the flag.
	if skeletonStale("not-a-date", pastTTL) {
		t.Error("unparseable date must not be flagged")
	}
}

// templateSpec reproduces the shape plan/TEMPLATE.md produces, with an
// Assumptions table below the first heading whose own header row ends in
// "| Status |".
const templateSpec = "# Spec: example\n" +
	"\n" +
	"<!-- Authoring note line 1\n" +
	"     line 2\n" +
	"     line 3\n" +
	"     line 4\n" +
	"     line 5 -->\n" +
	"\n" +
	"| Field | Value |\n" +
	"|-------|-------|\n" +
	"| Status | ready |\n" +
	"| Scope | protocol |\n" +
	"| Depends | - |\n" +
	"| Phase | 2/5 |\n" +
	"| Updated | 2026-08-18 |\n" +
	"\n" +
	"Recovery after compaction.\n" +
	"\n" +
	"## Task\n" +
	"\n" +
	"Some prose.\n" +
	"\n" +
	"| ID | Assumption | Basis | If wrong | Validation | Status |\n" +
	"|----|-----------|-------|----------|------------|--------|\n" +
	"| A-1 | something | a basis | a cost | a check | unvalidated |\n"

// TestMetaRowsFindsTablePastAFixedWindow reads a table the template pushes to
// line 12.
//
// VALIDATES: the anchor is the "| Field | Value |" header, never a line count.
// PREVENTS: every spec written from the template reading as "unknown".
func TestMetaRowsFindsTablePastAFixedWindow(t *testing.T) {
	rows, found := metaRows(templateSpec)
	if !found {
		t.Fatal("metaRows reported no metadata table for a spec written from the template")
	}
	for field, want := range map[string]string{
		"Status":  "ready",
		"Scope":   "protocol",
		"Depends": "-",
		"Phase":   "2/5",
		"Updated": "2026-08-18",
	} {
		if got := metaField(rows, field); got != want {
			t.Errorf("metaField(%q) = %q, want %q", field, got, want)
		}
	}
}

// TestMetaRowsStopsAtTheFirstHeading keeps the Assumptions table out.
//
// PREVENTS: reading "unvalidated" as the spec's Status, which is what a parser
// that scanned the whole file would do.
//
// Two distinct bounds are at work and only the SECOND case reaches the heading
// one. In the template shape the metadata table is followed by a blank line, so
// the "line that leaves the table" bound stops the scan before any heading is
// seen; the heading bound is what refuses a "| Field | Value |" header that
// appears further down the file, inside a section. The port inherited a test
// that covered the first bound alone, and a mutation of the heading prefix
// survived it (measured 2026-08-26).
func TestMetaRowsStopsAtTheFirstHeading(t *testing.T) {
	rows, _ := metaRows(templateSpec)
	if got := metaField(rows, "Status"); got != "ready" {
		t.Errorf("Status = %q, want %q: the scan ran past the first heading", got, "ready")
	}
	if last := rows[len(rows)-1]; last != "| Updated | 2026-08-18 |" {
		t.Errorf("last row = %q, want the Updated row: the scan ran past the table", last)
	}
	for _, line := range rows {
		if metaField([]string{line}, "A-1") != "" {
			t.Errorf("row %q came from the Assumptions table", line)
		}
	}

	// A spec with NO metadata table of its own, whose prose quotes one. The
	// header row is real and it is not this spec's metadata, so the scan must
	// never reach it.
	quoted := "# Spec: quotes a table\n" +
		"\n" +
		"This spec has no metadata table.\n" +
		"\n" +
		"## Task\n" +
		"\n" +
		"Every spec opens with a table of this shape:\n" +
		"\n" +
		"| Field | Value |\n" +
		"|-------|-------|\n" +
		"| Status | design |\n"
	if rows, found := metaRows(quoted); found {
		t.Errorf("metaRows read a table from past the first heading: %q", rows)
	}
}

// TestMetaRowsReportsAMissingTable keeps "no table" apart from "no Status row".
func TestMetaRowsReportsAMissingTable(t *testing.T) {
	if rows, found := metaRows("# Spec: example\n\nNo table here.\n\n## Task\n"); found {
		t.Errorf("metaRows found a table where there is none: %q", rows)
	}
	rows, found := metaRows("| Field | Value |\n|---|---|\n| Scope | cli |\n")
	if !found {
		t.Fatal("metaRows reported no table for a spec that has one")
	}
	if got := metaField(rows, "Status"); got != "" {
		t.Errorf("metaField(Status) = %q, want \"\" when the table omits the row", got)
	}
}

// TestCollectDistinguishesUnparsedFromUnknown drives the whole read over a tree
// holding all three shapes.
//
// VALIDATES: an unreadable spec reports "unparsed" and names itself, a table
// that omits Status reports "unknown" and says nothing, and neither is an error.
// PREVENTS: the fail-closed branch being proven at metaRows alone. metaRows can
// keep reporting a missing table correctly for ever while loadSpec stops acting
// on the answer, and a helper test cannot see that.
func TestCollectDistinguishesUnparsedFromUnknown(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-readable.md":  templateShapedSpec("ready", "2026-08-20"),
		"spec-fixture-no-table.md":  "# Spec: no table\n\nThis file carries no metadata table at all.\n\n## Task\n\nFixture prose.\n",
		"spec-fixture-no-status.md": "# Spec: no status row\n\n| Field | Value |\n|-------|-------|\n| Depends | - |\n| Phase | - |\n| Updated | 2026-08-20 |\n\n## Task\n\nFixture prose.\n",
	})

	inventory, warnings := collect(t, root, fixedNow)
	got := byName(inventory)
	for name, want := range map[string]string{
		"fixture-readable":  "ready",
		"fixture-no-table":  statusUnparsed,
		"fixture-no-status": "unknown",
	} {
		if got[name].Status != want {
			t.Errorf("status of %s = %q, want %q", name, got[name].Status, want)
		}
	}

	want := "spec-status: plan/spec-fixture-no-table.md has no '| Field | Value |' metadata table"
	if len(warnings) != 1 || warnings[0] != want {
		t.Errorf("warnings = %q, want exactly one line:\n%s", warnings, want)
	}
}

// TestTextSortsTheUnreadableSpecFirst pins the reporting order.
//
// VALIDATES: a spec the inventory cannot read sorts ahead of one whose table
// merely omits Status.
// PREVENTS: the row a reader has to act on being scrolled past.
func TestTextSortsTheUnreadableSpecFirst(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-no-table.md":  "# Spec: no table\n\nNo metadata table.\n",
		"spec-fixture-no-status.md": "# Spec: no status row\n\n| Field | Value |\n|-------|-------|\n| Depends | - |\n",
	})
	inventory, _ := collect(t, root, fixedNow)
	table := inventory.Text()

	unparsedAt := strings.Index(table, "fixture-no-table")
	unknownAt := strings.Index(table, "fixture-no-status")
	if unparsedAt < 0 {
		t.Fatalf("no row names the unreadable spec:\n%s", table)
	}
	if unknownAt < 0 {
		t.Fatalf("no row names the spec whose table omits Status:\n%s", table)
	}
	if unparsedAt > unknownAt {
		t.Errorf("the unparsed row sorts after the unknown one:\n%s", table)
	}
}

// TestCollectRefusesATreeWithNoPlanDirectory is the fail-open fix.
//
// VALIDATES: a tree that holds no plan/ is refused, not reported on.
// PREVENTS: an inventory of a population it never read. filepath.Glob answers an
// empty list and NO error for a pattern whose directory does not exist, so the
// script this ports prints "Specs: 0 total" and exits 0 over any tree it is not
// standing in. internal/le/parity/parity_test.go asserts the script still does.
func TestCollectRefusesATreeWithNoPlanDirectory(t *testing.T) {
	_, err := Collect(context.Background(), t.TempDir(), fixedNow, nil)
	if err == nil {
		t.Fatal("Collect reported an inventory for a tree that holds no plan/ directory")
	}
	if !strings.Contains(err.Error(), specGlob) {
		t.Errorf("the error does not name the population it could not read: %v", err)
	}
}

// TestCollectRefusesAPlanThatIsNotADirectory covers the second shape of the same
// mistake: the name exists and is a file, so a glob under it matches nothing.
func TestCollectRefusesAPlanThatIsNotADirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plan"), []byte("not a directory\n"), 0o600); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}
	_, err := Collect(context.Background(), root, fixedNow, nil)
	if err == nil {
		t.Fatal("Collect reported an inventory for a tree whose plan/ is a file")
	}
}

// TestCollectAcceptsAnEmptyPlanDirectory names the case that stays an ANSWER.
//
// PREVENTS: the fix above being written as "refuse an empty inventory". Every
// spec can legitimately be closed, and a tree in that state has a population of
// zero rather than a population nobody read.
func TestCollectAcceptsAnEmptyPlanDirectory(t *testing.T) {
	inventory, warnings := collect(t, planTree(t, nil), fixedNow)
	if len(inventory) != 0 {
		t.Errorf("inventory = %d records over an empty plan/, want 0", len(inventory))
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %q over an empty plan/, want none", warnings)
	}
	if !strings.Contains(inventory.Text(), "Specs: 0 total") {
		t.Errorf("the page does not report an empty population:\n%s", inventory.Text())
	}
}

// TestCollectFailsOnASpecItCannotRead pins the other half of failing closed: a
// file the glob LISTED and the tool cannot open is an error, never a record it
// quietly omits.
//
// A dangling symbolic link is the permission-independent way to reach it:
// filepath.Glob lists the name and os.ReadFile answers fs.ErrNotExist, and root
// cannot defeat it the way it defeats chmod 000.
func TestCollectFailsOnASpecItCannotRead(t *testing.T) {
	root := planTree(t, map[string]string{"spec-fixture-real.md": templateShapedSpec("ready", "2026-08-20")})
	link := filepath.Join(root, "plan", "spec-fixture-dangling.md")
	if err := os.Symlink(filepath.Join(root, "plan", "gone.md"), link); err != nil {
		t.Skipf("this filesystem does not support symbolic links: %v", err)
	}
	if _, err := Collect(context.Background(), root, fixedNow, nil); err == nil {
		t.Fatal("Collect answered an inventory that silently omits a spec it could not read")
	}
}

// TestCollectSkipsTheTemplate keeps plan/spec-template.md out of the counts.
func TestCollectSkipsTheTemplate(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-template.md":     templateShapedSpec("design", "2026-08-20"),
		"spec-fixture-real.md": templateShapedSpec("ready", "2026-08-20"),
	})
	inventory, _ := collect(t, root, fixedNow)
	if len(inventory) != 1 {
		t.Fatalf("inventory = %d records, want 1: the template is not a spec", len(inventory))
	}
	if inventory[0].Name != "fixture-real" {
		t.Errorf("the record names %q, want fixture-real", inventory[0].Name)
	}
}

// TestCollectFlagsAStaleSkeletonAndNothingElse drives the TTL through the whole
// read with a fixed clock.
//
// PREVENTS: the flag being proven at skeletonStale alone while Collect stops
// asking, and the flag reaching a status that is not a skeleton.
func TestCollectFlagsAStaleSkeletonAndNothingElse(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-old-skeleton.md":   templateShapedSpec("skeleton", "2026-01-01"),
		"spec-fixture-fresh-skeleton.md": templateShapedSpec("skeleton", "2026-06-01"),
		"spec-fixture-old-design.md":     templateShapedSpec("design", "2026-01-01"),
	})
	// Well past the six-week TTL for the January date, well inside it for June.
	now := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	inventory, _ := collect(t, root, now)
	got := byName(inventory)

	if !got["fixture-old-skeleton"].Stale {
		t.Error("a skeleton months past the TTL is not flagged")
	}
	if got["fixture-fresh-skeleton"].Stale {
		t.Error("a skeleton inside the TTL is flagged")
	}
	if got["fixture-old-design"].Stale {
		t.Error("a design spec is TTL-flagged; only idea capture is")
	}
	if got["fixture-old-design"].Bucket != Backlog {
		t.Errorf("a design spec is bucketed %q, want %q", got["fixture-old-design"].Bucket, Backlog)
	}
}

// TestCollectDetectsTheSpecSet reads the set out of the filename.
func TestCollectDetectsTheSpecSet(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-rib-arch-3-something.md": templateShapedSpec("ready", "2026-08-20"),
		"spec-standalone.md":           templateShapedSpec("ready", "2026-08-20"),
	})
	inventory, _ := collect(t, root, fixedNow)
	got := byName(inventory)
	if got["rib-arch-3-something"].Set != "rib-arch" {
		t.Errorf("set = %q, want rib-arch", got["rib-arch-3-something"].Set)
	}
	if got["standalone"].Set != "-" {
		t.Errorf("set = %q, want - for a filename with no set", got["standalone"].Set)
	}
}

// TestCollectFillsTheAbsentFields pins the placeholder each missing field takes,
// which is what keeps a column from rendering blank.
func TestCollectFillsTheAbsentFields(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-bare.md": "# Spec: bare\n\n| Field | Value |\n|---|---|\n| Status | ready |\n",
	})
	inventory, _ := collect(t, root, fixedNow)
	s := inventory[0]
	if s.Depends != "-" || s.Phase != "-" {
		t.Errorf("Depends = %q and Phase = %q, want - for both", s.Depends, s.Phase)
	}
	// A spec with no Updated row falls back to git's date, and a fixture tree
	// outside any repository has none.
	if s.Updated != "unknown" || s.GitModified != "unknown" {
		t.Errorf("Updated = %q and GitModified = %q, want unknown for both", s.Updated, s.GitModified)
	}
}

// TestCollectSortsNewestFirstWithinAStatus pins the second sort key.
//
// VALIDATES: two specs of one status are ordered by Updated, descending.
// PREVENTS: the inventory opening on the spec nobody has touched for months.
// The status key alone cannot see this: every case above holds one spec per
// status, so an ascending date order passes all of them.
func TestCollectSortsNewestFirstWithinAStatus(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-oldest.md": templateShapedSpec("ready", "2026-01-01"),
		"spec-fixture-newest.md": templateShapedSpec("ready", "2026-08-20"),
		"spec-fixture-middle.md": templateShapedSpec("ready", "2026-04-10"),
	})
	inventory, _ := collect(t, root, fixedNow)
	got := make([]string, 0, len(inventory))
	for _, s := range inventory {
		got = append(got, s.Name)
	}
	want := []string{"fixture-newest", "fixture-middle", "fixture-oldest"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", got, want)
	}
}
