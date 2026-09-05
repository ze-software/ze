package specstatus

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/leroot"
)

// specSummaryRE captures the total and the per-status breakdown from the first
// line the page prints.
var specSummaryRE = regexp.MustCompile(`(?m)^Specs: (\d+) total \(([^)]*)\)$`)

// TestTextSummaryCountsEverySpec drives the page over specs whose statuses the
// reporting order does not name.
//
// VALIDATES: the counts in the summary line sum to the total it states.
// PREVENTS: the summary reading as complete while it is not. Measured on
// 2026-08-22 over the real tree, it printed "242 total" over six counts summing
// to 240, because two specs carry `done` and the reporting order had never
// heard of it.
func TestTextSummaryCountsEverySpec(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-inprogress.md":   templateShapedSpec("in-progress", "2026-08-20"),
		"spec-fixture-verification.md": templateShapedSpec("verification", "2026-08-20"),
		"spec-fixture-done.md":         templateShapedSpec("done", "2026-08-20"),
		"spec-fixture-invented.md":     templateShapedSpec("never-heard-of", "2026-08-20"),
	})
	page := mustPage(t, root)

	m := specSummaryRE.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no summary line in the page:\n%s", page)
	}
	total, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("total %q is not a number: %v", m[1], err)
	}

	sum := 0
	seen := map[string]bool{}
	for part := range strings.SplitSeq(m[2], ", ") {
		count, status, ok := strings.Cut(part, " ")
		if !ok {
			t.Fatalf("summary entry %q is not '<count> <status>'", part)
		}
		n, err := strconv.Atoi(count)
		if err != nil {
			t.Fatalf("summary entry %q does not start with a number: %v", part, err)
		}
		sum += n
		seen[status] = true
	}
	if sum != total {
		t.Errorf("the summary counts sum to %d and claim a total of %d; %d specs are missing from the breakdown:\n%s",
			sum, total, total-sum, page)
	}
	for _, status := range []string{"in-progress", "verification", "done", "never-heard-of"} {
		if !seen[status] {
			t.Errorf("the summary line never names %q, so those specs are invisible in it:\n%s", status, page)
		}
	}
}

// TestTextFilesVerificationUnderCommittedBacklog pins where a reviewer's queue
// is printed.
func TestTextFilesVerificationUnderCommittedBacklog(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-verification.md": templateShapedSpec("verification", "2026-08-20"),
		"spec-fixture-inprogress.md":   templateShapedSpec("in-progress", "2026-08-20"),
		"spec-fixture-blocked.md":      templateShapedSpec("blocked", "2026-08-20"),
	})
	page := mustPage(t, root)

	for name, want := range map[string]string{
		"fixture-verification": "Committed backlog",
		"fixture-inprogress":   "Committed backlog",
		"fixture-blocked":      "Other",
	} {
		if section := bucketSectionOf(t, page, name); !strings.Contains(section, want) {
			t.Errorf("%s is filed under %q, want a section naming %q", name, section, want)
		}
	}
}

// TestTextRendersAnEmptyBucketAsNone keeps a bucket with nothing in it visible.
//
// PREVENTS: a heading with no rows and no marker under it, which reads as
// output that got cut off rather than as an empty bucket.
func TestTextRendersAnEmptyBucketAsNone(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-ready.md": templateShapedSpec("ready", "2026-08-20"),
	})
	page := mustPage(t, root)

	if got := strings.Count(page, "  (none)\n"); got != 2 {
		t.Errorf("the page marks %d empty buckets, want 2 (idea capture and other):\n%s", got, page)
	}
	if !strings.Contains(page, "── Idea capture: skeleton stubs (STALE = past TTL, triage or drop) (0) ──") {
		t.Errorf("the empty idea-capture heading does not carry its count:\n%s", page)
	}
}

// TestTextMarksAStaleSkeleton pins the flag column.
func TestTextMarksAStaleSkeleton(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-old.md":   templateShapedSpec("skeleton", "2026-01-01"),
		"spec-fixture-fresh.md": templateShapedSpec("skeleton", "2026-06-01"),
	})
	inventory, _ := collect(t, root, mustDate(t, "2026-06-20"))
	page := inventory.Text()

	oldRow := rowNaming(t, page, "fixture-old")
	if !strings.HasPrefix(oldRow, "STALE") {
		t.Errorf("the stale row does not open with the flag: %q", oldRow)
	}
	freshRow := rowNaming(t, page, "fixture-fresh")
	if strings.Contains(freshRow, "STALE") {
		t.Errorf("a skeleton inside the TTL carries the flag: %q", freshRow)
	}
	if !strings.Contains(page, "(1 past the 6-week TTL)") {
		t.Errorf("the bucket line does not count the flagged skeleton:\n%s", page)
	}
}

// TestTextAlignsEveryRowOnTheSameColumns pins the column arithmetic.
//
// VALIDATES: the header row, the box-drawing rule and a data row all start
// their fifth cell at the same offset, and that offset is the sum of the
// widths declared above them.
// PREVENTS: one of the three rows being written from a different width, which
// leaves a rule that sits under the wrong columns while every assertion about
// the CONTENT of the page still passes.
//
// It does not reach the rune-versus-byte question: every value a fixture can
// put in a padded cell is ASCII, and the one multi-byte cell (the rule) is
// exactly its column's width, so both paddings answer zero for it. The
// byte-for-byte comparison against the script over the real tree
// (internal/le/parity/parity_test.go) is what covers that.
func TestTextAlignsEveryRowOnTheSameColumns(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-short.md": templateShapedSpec("ready", "2026-08-20"),
	})
	page := mustPage(t, root)

	header := rowNaming(t, page, "Flag")
	separator := rowNaming(t, page, "─────")
	data := rowNaming(t, page, "fixture-short")
	for i, line := range []string{header, separator, data} {
		if got := runeOffsetOfColumn(line, 4); got != colFlag+2+colBucket+2+colStatus+2+colUpdated+2 {
			t.Errorf("row %d starts its fifth column at rune %d, want %d:\n%q",
				i, got, colFlag+2+colBucket+2+colStatus+2+colUpdated+2, line)
		}
	}
}

// runeOffsetOfColumn answers the rune offset at which column n of a padded row
// begins. Columns are separated by exactly two spaces and no cell before the
// last may contain a double space, which is what the padding guarantees.
func runeOffsetOfColumn(line string, n int) int {
	offset := 0
	for range n {
		idx := strings.Index(line[offset:], "  ")
		if idx < 0 {
			return -1
		}
		rest := line[offset+idx:]
		width := len(rest) - len(strings.TrimLeft(rest, " "))
		offset += idx + width
	}
	return utf8.RuneCountInString(line[:offset])
}

// TestInventoryIsStructuredDataWithKebabCaseKeys is AC-7 at the payload.
//
// VALIDATES: the answer is a JSON array of records with kebab-case keys, which
// is what lets `| json`, `| yaml` and `| table` render it with no code in this
// package.
// PREVENTS: a tool that answers finished text and claims to satisfy the CLI
// contract.
func TestInventoryIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-ready.md": templateShapedSpec("ready", "2026-08-20"),
	})
	inventory, _ := collect(t, root, fixedNow)

	raw, err := json.Marshal(inventory)
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	var records []map[string]any
	if err := json.Unmarshal(raw, &records); err != nil {
		t.Fatalf("the payload is not an array of records: %v\n%s", err, raw)
	}
	if len(records) != 1 {
		t.Fatalf("the payload holds %d records, want 1", len(records))
	}
	want := []string{"name", "status", "depends", "phase", "set", "updated", "git-modified", "bucket", "category", "stale"}
	if len(records[0]) != len(want) {
		t.Errorf("the record carries %d keys, want %d: %v", len(records[0]), len(want), records[0])
	}
	for _, key := range want {
		if _, ok := records[0][key]; !ok {
			t.Errorf("the record has no %q key: %v", key, records[0])
		}
	}
	for key := range records[0] {
		if strings.ContainsAny(key, "_ ") || strings.ToLower(key) != key {
			t.Errorf("key %q is not kebab-case", key)
		}
	}
}

// TestTheCommandDeclaresItsAnswerShape reads the declaration back out of the
// engine both binaries share.
//
// VALIDATES: the row operators act on the specs instead of being refused.
func TestTheCommandDeclaresItsAnswerShape(t *testing.T) {
	shape, declared := command.ShapeForCommand(leroot.CommandPath("spec status"))
	if !declared {
		t.Fatal("the spec-status command declares no answer shape")
	}
	if shape != command.ShapeMap {
		t.Errorf("the spec-status command declares shape %v, want ShapeMap", shape)
	}
}

// TestAnswerRefusesAnArgument pins the grammar: the rendering is a pipe
// operator, so the script's --json flag is not an argument here.
func TestAnswerRefusesAnArgument(t *testing.T) {
	payload, code := Answer([]string{"--json"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2 for an argument the command takes none of", code)
	}
	if payload != nil {
		t.Errorf("the refusal carries a payload: %v", payload)
	}
}

// TestAnswerReadsTheCheckoutNamedByTheEnvironment drives the entry point.
//
// VALIDATES: Answer resolves the tree through lepath.Root and answers 0 with
// the records for it.
// PREVENTS: Collect being proven while the caller stops acting on its answer.
func TestAnswerReadsTheCheckoutNamedByTheEnvironment(t *testing.T) {
	root := planTree(t, map[string]string{
		"spec-fixture-ready.md": templateShapedSpec("ready", "2026-08-20"),
	})
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	payload, code := Answer(nil)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0: the inventory reports, it does not gate", code)
	}
	inventory, ok := payload.(Inventory)
	if !ok {
		t.Fatalf("the payload is %T, want Inventory", payload)
	}
	if len(inventory) != 1 || inventory[0].Name != "fixture-ready" {
		t.Errorf("the answer does not name the fixture spec: %v", inventory)
	}
}

// TestAnswerRefusesATreeWithNoPopulation carries the fail-open fix to the exit
// code, which is the fact a caller reads.
func TestAnswerRefusesATreeWithNoPopulation(t *testing.T) {
	t.Setenv("ZE_REPO_ROOT", t.TempDir())
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	payload, code := Answer(nil)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 for a tree that holds no plan/ directory", code)
	}
	if payload != nil {
		t.Errorf("the refusal carries a payload: %v", payload)
	}
}

// mustPage collects over a fixture tree and renders the page.
func mustPage(t *testing.T, root string) string {
	t.Helper()
	inventory, _ := collect(t, root, fixedNow)
	return inventory.Text()
}

// mustDate parses a YYYY-MM-DD fixture date.
func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse the fixture date %q: %v", s, err)
	}
	return parsed
}

// bucketSectionOf returns the heading of the bucket section that holds the row
// naming spec.
func bucketSectionOf(t *testing.T, page, spec string) string {
	t.Helper()
	heading := ""
	for line := range strings.SplitSeq(page, "\n") {
		if strings.HasPrefix(line, "── ") {
			heading = line
			continue
		}
		if strings.Contains(line, spec) {
			return heading
		}
	}
	t.Fatalf("no row names %s:\n%s", spec, page)
	return ""
}

// rowNaming returns the first line of the page that contains want.
func rowNaming(t *testing.T, page, want string) string {
	t.Helper()
	for line := range strings.SplitSeq(page, "\n") {
		if strings.Contains(line, want) {
			return line
		}
	}
	t.Fatalf("no line names %q:\n%s", want, page)
	return ""
}
