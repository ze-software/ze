package main

// AC-3 (spec-fixit-spec-hygiene-tooling): committed backlog (design/ready/
// in-progress/verification) and idea capture (skeleton) must render as DISTINCT
// buckets, and skeletons past a TTL must be flagged. The bucketing + TTL logic lives in the
// specbucket subpackage so it is importable both by the go-run spec_status.go
// script and by this test (spec_status.go itself is //go:build ignore and cannot
// be compiled into this package).

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/scripts/status/specbucket"
)

// TestSpecStatusBacklogSplit validates the committed-backlog vs idea-capture
// split at the heart of AC-3.
func TestSpecStatusBacklogSplit(t *testing.T) {
	cases := map[string]string{
		"in-progress": specbucket.Backlog,
		// Committed work waiting on a reviewer. ai/rules/planning.md tells the
		// implementing session to set it before it commits, so a spec sits here
		// between implementation and closure. Counting it as `other` files
		// finished-and-unreviewed work beside blocked and deferred, which is
		// where a reader looks for work nobody is carrying.
		"verification": specbucket.Backlog,
		"ready":        specbucket.Backlog,
		"design":       specbucket.Backlog,
		"skeleton":     specbucket.Idea,
		"blocked":      specbucket.Other,
		"deferred":     specbucket.Other,
		"unknown":      specbucket.Other,
	}
	for status, want := range cases {
		if got := specbucket.Category(status); got != want {
			t.Errorf("Category(%q) = %q, want %q", status, got, want)
		}
	}
}

// TestSkeletonTTLBoundary pins the flag boundary: at exactly the TTL a skeleton
// is not yet flagged; one day past it is (see the boundary table in the spec).
func TestSkeletonTTLBoundary(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := base.Format("2006-01-02")
	day := 24 * time.Hour

	atTTL := base.Add(time.Duration(specbucket.SkeletonTTLDays) * day)
	if specbucket.SkeletonStale(updated, atTTL) {
		t.Errorf("skeleton at exactly TTL (%d days) must not be flagged", specbucket.SkeletonTTLDays)
	}

	pastTTL := base.Add(time.Duration(specbucket.SkeletonTTLDays+1) * day)
	if !specbucket.SkeletonStale(updated, pastTTL) {
		t.Errorf("skeleton one day past TTL (%d days) must be flagged", specbucket.SkeletonTTLDays+1)
	}

	// A skeleton is only idea capture; a non-skeleton is never TTL-flagged.
	if specbucket.Category("skeleton") != specbucket.Idea {
		t.Fatal("skeleton must classify as idea capture")
	}

	// An unparseable date cannot be judged and must not be flagged (flagging
	// noise trains the reader to ignore the flag).
	if specbucket.SkeletonStale("not-a-date", pastTTL) {
		t.Error("unparseable date must not be flagged")
	}
}

// specStatusTimeout bounds one compile plus one run of the inventory tool.
const specStatusTimeout = 3 * time.Minute

// specStatusBinary compiles the spec inventory and returns the path to it.
//
// The tool is compiled rather than called in process because its own build tag
// keeps spec_status.go out of this package, so loadSpec is reachable from a Go
// test no other way. Compiling buys more than the call does. The test then
// drives what an operator drives, main and the plan/ glob and the exit code
// included, which is what ai/rules/evidence.md asks of a guard's test: the
// helper proves the helper, and only the entry point proves that the caller
// still acts on the answer.
func specStatusBinary(ctx context.Context, t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "spec-status")
	source := filepath.Join("scripts", "status", "spec_status.go")
	// Read the tool before compiling it. spec_status.go carries //go:build
	// ignore, so it is not in this test binary's own source hash, and go test
	// caches a result against the files the TEST BINARY opened rather than the
	// files an exec'd compiler read. Without this read both tests below report
	// their last verdict from the cache after the tool changes: measured
	// 2026-08-22, deleting the statusUnparsed branch from loadSpec left the
	// package "ok (cached)".
	if _, err := os.ReadFile(filepath.Join(repoRootFromScriptsStatus(), source)); err != nil {
		t.Fatalf("read %s: %v", source, err)
	}
	compile := exec.CommandContext(ctx, "go", "build", "-o", binary, source)
	compile.Dir = repoRootFromScriptsStatus()
	compile.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile the spec inventory: %v\n%s", err, out)
	}
	return binary
}

// runSpecStatus writes each named spec into a plan/ tree of its own, runs the
// inventory over that tree, and returns stdout, stderr and the exit code.
//
// The tree is a temporary directory outside any repository, so gitDate reports
// "unknown" for every fixture and the output stays the same on every machine.
func runSpecStatus(t *testing.T, specs map[string]string, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), specStatusTimeout)
	defer cancel()

	tree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, "plan"), 0o755); err != nil {
		t.Fatalf("create the fixture plan directory: %v", err)
	}
	for name, body := range specs {
		if err := os.WriteFile(filepath.Join(tree, "plan", name), []byte(body), 0o600); err != nil {
			t.Fatalf("write the fixture spec %s: %v", name, err)
		}
	}

	cmd := exec.CommandContext(ctx, specStatusBinary(ctx, t), args...)
	cmd.Dir = tree
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("run the spec inventory: %v\nstderr: %s", err, stderr.String())
		}
		code = exit.ExitCode()
	}
	if ctx.Err() != nil {
		t.Fatalf("the spec inventory did not return inside %s", specStatusTimeout)
	}
	return stdout.String(), stderr.String(), code
}

// templateShapedSpec returns a spec of the shape plan/TEMPLATE.md produces: a
// title, a six-line HTML authoring comment, then the metadata table. That
// comment is what pushed the Status row past the old fixed ten-line window, so
// every spec written from the template read as "unknown" to this tool.
func templateShapedSpec(status string) string {
	return "# Spec: fixture\n" +
		"\n" +
		"<!-- Authoring note line 1\n" +
		"     line 2\n" +
		"     line 3\n" +
		"     line 4\n" +
		"     line 5 -->\n" +
		"\n" +
		"| Field | Value |\n" +
		"|-------|-------|\n" +
		"| Status | " + status + " |\n" +
		"| Depends | - |\n" +
		"| Phase | - |\n" +
		"| Updated | 2026-08-20 |\n" +
		"\n" +
		"## Task\n" +
		"\n" +
		"Fixture prose.\n"
}

// specStatusRecord is the subset of the JSON record these tests read.
type specStatusRecord struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// specStatusJSON decodes the tool's JSON output into a name-to-status map.
func specStatusJSON(t *testing.T, stdout string) map[string]string {
	t.Helper()
	var records []specStatusRecord
	if err := json.Unmarshal([]byte(stdout), &records); err != nil {
		t.Fatalf("decode the inventory JSON: %v\n%s", err, stdout)
	}
	byName := make(map[string]string, len(records))
	for _, r := range records {
		byName[r.Name] = r.Status
	}
	return byName
}

// TestSpecStatusReportsAnUnreadableSpec drives the tool's own entry point over a
// spec that carries no metadata table.
//
// VALIDATES: AC-3. An unreadable spec reports "unparsed", names itself on
// stderr, sorts ahead of an "unknown" row, and leaves the exit code at 0.
// PREVENTS: the fail-closed branch being proven at specmeta.Rows alone.
// specmeta.Rows can keep reporting a missing table correctly for ever while
// loadSpec stops acting on the answer, and a helper test cannot see that.
func TestSpecStatusReportsAnUnreadableSpec(t *testing.T) {
	specs := map[string]string{
		"spec-fixture-readable.md": templateShapedSpec("ready"),
		"spec-fixture-no-table.md": "# Spec: no table\n\nThis file carries no metadata table at all.\n\n## Task\n\nFixture prose.\n",
		// A table that IS present and omits the Status row. This is the answer
		// "unparsed" must stay distinct from: the tool read the table and found
		// no Status, which is a different fact from being unable to read it.
		"spec-fixture-no-status.md": "# Spec: no status row\n\n| Field | Value |\n|-------|-------|\n| Depends | - |\n| Phase | - |\n| Updated | 2026-08-20 |\n\n## Task\n\nFixture prose.\n",
	}

	stdout, stderr, code := runSpecStatus(t, specs, "--json")
	if code != 0 {
		t.Errorf("exit code = %d, want 0: the inventory reports, it does not gate\nstderr: %s", code, stderr)
	}

	want := "spec-status: plan/spec-fixture-no-table.md has no '| Field | Value |' metadata table"
	if !strings.Contains(stderr, want) {
		t.Errorf("stderr does not name the unreadable spec.\nwant a line containing: %s\ngot:\n%s", want, stderr)
	}
	if strings.Contains(stderr, "spec-fixture-no-status.md") {
		t.Errorf("stderr names the spec whose table omits Status; that table was read fine:\n%s", stderr)
	}

	byName := specStatusJSON(t, stdout)
	for name, wantStatus := range map[string]string{
		"fixture-readable":  "ready",
		"fixture-no-table":  "unparsed",
		"fixture-no-status": "unknown",
	} {
		if got := byName[name]; got != wantStatus {
			t.Errorf("status of %s = %q, want %q", name, got, wantStatus)
		}
	}

	// The table output must put the unreadable spec ahead of the merely
	// incomplete one: it is the row a reader has to act on.
	table, _, _ := runSpecStatus(t, specs)
	unparsedAt := strings.Index(table, "fixture-no-table")
	unknownAt := strings.Index(table, "fixture-no-status")
	if unparsedAt < 0 {
		t.Fatalf("no row names the unreadable spec:\n%s", table)
	}
	if unknownAt < 0 {
		t.Fatalf("no row names the spec whose table omits Status:\n%s", table)
	}
	if unparsedAt > unknownAt {
		t.Errorf("the unparsed row sorts after the unknown one; a spec the tool cannot read must not be scrolled past:\n%s", table)
	}
}

// specSummaryRE captures the total and the per-status breakdown from the first
// line the tool prints.
var specSummaryRE = regexp.MustCompile(`(?m)^Specs: (\d+) total \(([^)]*)\)$`)

// TestSpecStatusSummaryCountsEverySpec drives the tool over specs whose statuses
// the reporting order does not name.
//
// VALIDATES: AC-4 and AC-5. The counts in the summary line sum to the total it
// states, and a spec at `verification` is counted as committed backlog.
// PREVENTS: the summary reading as complete while it is not. Measured on
// 2026-08-22, the real tree printed "242 total" over six counts summing to 240,
// because two specs carry `done` and the reporting order had never heard of it.
func TestSpecStatusSummaryCountsEverySpec(t *testing.T) {
	specs := map[string]string{
		"spec-fixture-inprogress.md":   templateShapedSpec("in-progress"),
		"spec-fixture-verification.md": templateShapedSpec("verification"),
		"spec-fixture-done.md":         templateShapedSpec("done"),
	}

	stdout, stderr, code := runSpecStatus(t, specs)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}

	m := specSummaryRE.FindStringSubmatch(stdout)
	if m == nil {
		t.Fatalf("no summary line in the output:\n%s", stdout)
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
			sum, total, total-sum, stdout)
	}
	for _, status := range []string{"in-progress", "verification", "done"} {
		if !seen[status] {
			t.Errorf("the summary line never names %q, so those specs are invisible in it:\n%s", status, stdout)
		}
	}

	if section := bucketSectionOf(t, stdout, "fixture-verification"); !strings.Contains(section, "Committed backlog") {
		t.Errorf("a spec at verification is filed under %q; it is committed work waiting on a reviewer", section)
	}
	if section := bucketSectionOf(t, stdout, "fixture-inprogress"); !strings.Contains(section, "Committed backlog") {
		t.Errorf("a spec in progress is filed under %q", section)
	}
}

// bucketSectionOf returns the heading of the bucket section that holds the row
// naming spec.
func bucketSectionOf(t *testing.T, table, spec string) string {
	t.Helper()
	heading := ""
	for line := range strings.SplitSeq(table, "\n") {
		if strings.HasPrefix(line, "── ") {
			heading = line
			continue
		}
		if strings.Contains(line, spec) {
			return heading
		}
	}
	t.Fatalf("no row names %s:\n%s", spec, table)
	return ""
}
