package doccheck

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// selectorMatchesOf returns every finding for file as "old@line".
func selectorMatchesOf(report retiredSelectorReport, file string) []string {
	var found []string
	for _, match := range report.Matches {
		if match.File == file {
			found = append(found, match.Old+"@"+strconv.Itoa(match.Line))
		}
	}
	return found
}

// loadFixtureDir reads every file under dir into a map keyed by slash-separated
// path relative to dir.
func loadFixtureDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir(dir, func(curr string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, curr)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(curr)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatalf("loading fixture dir %s: %v", dir, err)
	}
	return files
}

func TestRetiredSelectorValueSweepRefusesEachShape(t *testing.T) {
	// VALIDATES: AC-20 -- over the fixture tree under
	// internal/le/doc/check/testdata/retired_selector/ plus every historical
	// record read directly from retiredRecords, the sweep refuses every
	// required selector shape (naming file, line, old value, and kernel) and
	// separates every covered/near-miss pair for F-1 to F-4, O-1 to O-7, and
	// line exceptions with zero unexpected findings and zero missed refusals.
	// PREVENTS: dropping a sub-rule (turning a covered occurrence into an
	// unexpected refusal) or widening a sub-rule past its near-miss (silently
	// passing a retired selector value).
	files := loadFixtureDir(t, filepath.Join("testdata", "retired_selector"))

	// F-1 records: read directly from retiredRecords so a sweep that copies
	// only a subset of paths or skips snapshot directories whole goes red.
	var expectedSkipped []string
	for _, record := range retiredRecords {
		if record.manifest != "" {
			dir := path.Dir(record.manifest)
			basePinned := dir + "/base/pinned.go.txt"
			unfinishedPinned := dir + "/unfinished/pinned.go.txt"
			files[record.manifest] = strings.Join([]string{
				`{`,
				`  "status": "clean fixture manifest",`,
				`  "files": [`,
				`    {"asset": "unfinished/pinned.go.txt", "sha256": "aaa", "base_asset": "base/pinned.go.txt", "base_sha256": "bbb"}`,
				`  ]`,
				`}`,
			}, "\n") + "\n"
			files[basePinned] = "backend nft\n"
			files[unfinishedPinned] = "backend nft\n"
			expectedSkipped = append(expectedSkipped, basePinned, unfinishedPinned)
			continue
		}
		target := record.path
		if strings.HasSuffix(target, "/") {
			target += "nested/record.md"
		}
		files[target] = "backend nft\n"
		expectedSkipped = append(expectedSkipped, target)
	}

	root := fixtureRepository(t, files)
	report, err := sweepRetiredSelectors(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Unreadable) != 0 {
		t.Fatalf("unexpected unreadable files: %v", report.Unreadable)
	}

	want := []RetiredSelectorMatch{
		// Part A: required refused shapes.
		{File: "test/parse/sample.ci", Line: 1, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "internal/component/sample/sample.yang", Line: 2, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "internal/component/sample/sample.yang", Line: 3, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "internal/component/sample/sample.yang", Line: 4, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "docs/sample.md", Line: 1, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/sample.md", Line: 2, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/sample.md", Line: 3, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/sample.md", Line: 3, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "docs/sample.md", Line: 4, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "docs/sample.md", Line: 5, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/sample.md", Line: 6, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "docs/sample.md", Line: 7, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/sample.md", Line: 8, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/sample.md", Line: 9, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/sample.md", Line: 10, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "internal/sample/sample.go", Line: 2, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "internal/sample/sample.go", Line: 3, Old: selectorValNFT, Replacement: retiredSelectorReplacement},

		// F-1 near-misses (unpinned snapshot file, README beside manifest, outside verify/engine/testdata, owned rows outside section / non-spec).
		{File: "plan/post-release/engineering/tooling-one-off-reboot-assets/base/unpinned.go.txt", Line: 1, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "plan/post-release/engineering/tooling-one-off-reboot-assets/README.md", Line: 1, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "internal/le/verify/engine/outside.go", Line: 2, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "plan/release/fixes/spec-fixture.md", Line: 5, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "docs/not-a-spec.md", Line: 2, Old: selectorValNFT, Replacement: retiredSelectorReplacement},

		// F-2 near-misses.
		{File: "rfc/short/rfc9999.md", Line: 1, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "gokrazy/kernel/live.txt", Line: 1, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "internal/le/vendorpatch/live.txt", Line: 1, Old: selectorValNFT, Replacement: retiredSelectorReplacement},

		// F-3 near-misses.
		{File: "internal/le/doc/check/retired.go", Line: 2, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "plan/release/fixes/spec-other.md", Line: 1, Old: selectorValNFT, Replacement: retiredSelectorReplacement},

		// F-4 near-misses.
		{File: "test/parse/other-proof.ci", Line: 1, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "test/plugin/other-proof.ci", Line: 1, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "internal/component/config/other_retired_test.go", Line: 2, Old: selectorValNFT, Replacement: retiredSelectorReplacement},

		// O-1 near-misses.
		{File: "internal/sample/o1_sample.go", Line: 5, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "internal/sample/o1_sample.go", Line: 6, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},

		// O-2 near-misses.
		{File: "docs/o2_sample.md", Line: 4, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/o2_sample.md", Line: 5, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/o2_sample.md", Line: 6, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/o2_sample.md", Line: 7, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/o2_sample.md", Line: 8, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "docs/o2_sample.md", Line: 8, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "docs/o2_sample.md", Line: 9, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "docs/o2_sample.md", Line: 10, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},

		// O-3 near-misses.
		{File: "docs/o3_sample.md", Line: 2, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "docs/o3_sample.md", Line: 3, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "docs/o3_sample.md", Line: 4, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "docs/o3_sample.md", Line: 5, Old: selectorValNFT, Replacement: retiredSelectorReplacement},

		// O-4 near-misses.
		{File: "internal/sample/o4_sample.yang", Line: 5, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "internal/sample/o4_sample.yang", Line: 6, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "internal/sample/o4_sample.yang", Line: 7, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "internal/sample/o4_sample.yang", Line: 8, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "docs/o4_sample.md", Line: 2, Old: selectorValTC, Replacement: retiredSelectorReplacement},

		// O-5 near-misses.
		{File: "docs/o5_sample.md", Line: 8, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/o5_sample.md", Line: 9, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "docs/o5_sample.md", Line: 10, Old: selectorValTC, Replacement: retiredSelectorReplacement},
		{File: "docs/o5_sample.md", Line: 11, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},

		// O-6 near-misses.
		{File: "docs/o6_sample.md", Line: 4, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/o6_sample.md", Line: 5, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/o6_sample.md", Line: 6, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/o6_sample.md", Line: 7, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/o6_sample.md", Line: 8, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
		{File: "docs/o6_sample.md", Line: 9, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},

		// O-7 near-misses.
		{File: "internal/sample/o7_sample.go", Line: 5, Old: selectorValNFT, Replacement: retiredSelectorReplacement},
		{File: "internal/sample/o7_sample.go", Line: 6, Old: selectorValTC, Replacement: retiredSelectorReplacement},

		// Line exception near-miss: undeclared line in the same file.
		{File: "internal/component/gtsm/doctor.go", Line: 5, Old: selectorValNetlink, Replacement: retiredSelectorReplacement},
	}

	// Verify every F-1 record path was skipped.
	for _, skipped := range expectedSkipped {
		if got := selectorMatchesOf(report, skipped); len(got) != 0 {
			t.Errorf("F-1 record %s was not skipped: %v", skipped, got)
		}
	}

	// Exact comparison: every expected refusal present, no unexpected refusal.
	sortMatches := func(list []RetiredSelectorMatch) {
		slices.SortFunc(list, func(a, b RetiredSelectorMatch) int {
			if a.File != b.File {
				return strings.Compare(a.File, b.File)
			}
			if a.Line != b.Line {
				return a.Line - b.Line
			}
			return strings.Compare(a.Old, b.Old)
		})
	}
	got := slices.Clone(report.Matches)
	sortMatches(got)
	sortMatches(want)

	for _, expected := range want {
		if !slices.Contains(got, expected) {
			t.Errorf("missing expected refusal %s:%d (%s -> %s)",
				expected.File, expected.Line, expected.Old, expected.Replacement)
		}
	}
	for _, actual := range got {
		if !slices.Contains(want, actual) {
			t.Errorf("unexpected refusal %s:%d (%s -> %s)",
				actual.File, actual.Line, actual.Old, actual.Replacement)
		}
	}

	// Verify rendered text names file, line, old value, and kernel for each finding.
	rendered := report.Text()
	for _, expected := range want {
		needle := expected.File + ":" + strconv.Itoa(expected.Line) +
			": retired selector value " + expected.Old + " (use " + retiredSelectorReplacement + ")\n"
		if !strings.Contains(rendered, needle) {
			t.Errorf("report.Text() lacks %q", needle)
		}
	}
}

func TestRetiredSelectorValueSweepPassesTheTree(t *testing.T) {
	// VALIDATES: AC-20 -- every whole-word occurrence of netlink, nft or tc in
	// the tracked repository checkout is covered by a declared class or line
	// exception. Expected RED until the N-1 rename converts all remaining
	// repository users, and GREEN once N-1 lands.
	// PREVENTS: a retired selector value surviving in any tracked file.
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	report, err := sweepRetiredSelectors(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Unreadable) != 0 {
		t.Fatalf("sweep had unreadable files:\n%s", report.Text())
	}
	if len(report.Matches) != 0 {
		t.Fatalf("found %d retired selector value occurrence(s):\n%s", len(report.Matches), report.Text())
	}
}

func TestRetiredSelectorEveryExceptionDeclaresItsLines(t *testing.T) {
	// VALIDATES: every row of retiredSelectorExceptions declares at least one
	// non-blank line and a known retired selector value.
	// PREVENTS: a blank line exception that would silence an entire file.
	for _, exception := range retiredSelectorExceptions {
		if _, ok := matchRetiredSelectorValue(exception.old); !ok {
			t.Errorf("exception %s declares unknown old value %q", exception.file, exception.old)
		}
		if len(exception.lines) == 0 {
			t.Errorf("exception %s %q declares no line", exception.file, exception.old)
		}
		for _, line := range exception.lines {
			if strings.TrimSpace(line) == "" {
				t.Errorf("exception %s %q declares a blank line, which covers the whole file", exception.file, exception.old)
			}
		}
	}
}
