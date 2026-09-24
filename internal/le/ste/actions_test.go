package ste

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

// TestReviewEachMatchesASerialWalk proves the parallel review answers what a
// serial walk answers, slot for slot in list order.
//
// The method builds a tree whose documents each carry a different number of
// findings, plus a path that is absent and a path no surface reads, and
// compares every slot with Review called directly. Run under -race, it also
// proves the workers share no state.
func TestReviewEachMatchesASerialWalk(t *testing.T) {
	root := t.TempDir()
	var files []string
	for i := range 64 {
		rel := "docs/doc" + strconv.Itoa(i) + ".md"
		text := "# Title\n\n"
		for range i % 5 {
			text += "The daemon may start. It should typically work.\n\n"
		}
		if err := os.MkdirAll(filepath.Join(root, "docs"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		files = append(files, rel)
	}
	files = append(files, "docs/vanished.md", "docs/image.png")

	results := reviewEach(root, files)
	if len(results) != len(files) {
		t.Fatalf("got %d results for %d files", len(results), len(files))
	}
	sawFinding := false
	for i, rel := range files {
		result := results[i]
		if result.err != nil {
			t.Fatalf("%s: %v", rel, result.err)
		}
		body, err := os.ReadFile(filepath.Join(root, rel)) //nolint:gosec // the test's own temporary tree
		if err != nil {
			if result.present {
				t.Fatalf("%s is absent but was reported present", rel)
			}
			continue
		}
		surface, ok := surfaceOf(rel)
		if !ok {
			if result.present {
				t.Fatalf("%s has no surface but was reported present", rel)
			}
			continue
		}
		want, wantSkip := Review(rel, string(body), surface)
		if !result.present || result.skipReason != wantSkip || !reflect.DeepEqual(result.findings, want) {
			t.Fatalf("%s: the parallel slot differs from a serial Review\ngot  %v %q\nwant %v %q",
				rel, result.findings, result.skipReason, want, wantSkip)
		}
		if len(want) > 0 {
			sawFinding = true
		}
	}
	if !sawFinding {
		t.Fatal("no document produced a finding, so the comparison is vacuous")
	}
}
