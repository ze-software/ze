// VALIDATES: every .ci file the EncodingTests runner discovers parses cleanly.
// PREVENTS: a directive nobody reads sitting in the tree unnoticed. The runner
//           records an unparseable file as a suite failure, which is loud only
//           when that suite is actually run -- a Linux-only or QEMU-only suite
//           can carry a broken line for months. This gate reads them all in one
//           `go test`, at the price of a directory walk.

package cli

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/runner"
)

// nonEncodingCIDirs names the test/<dir> trees whose .ci files a DIFFERENT
// parser reads, so EncodingTests must not be pointed at them. Every entry comes
// from the dispatch that routes it (zeTestRunSimpleTests reads decode and
// parse, and the predecessor-compatibility runner reads predecessorTestDir), so
// a re-routed suite moves this set with it.
var nonEncodingCIDirs = map[string]bool{
	cmdDecode:          true,
	cmdParse:           true,
	predecessorTestDir: true,
}

func TestEveryEncodingCIFileParses(t *testing.T) {
	baseDir, err := FindBaseDir()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	testRoot := filepath.Join(baseDir, "test")

	// Every directory holding .ci files, including test/draft/<suite>: a draft
	// is run with --draft and is exactly as able to carry a dead directive.
	seen := make(map[string]bool)
	var dirs []string
	err = filepath.WalkDir(testRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".ci" {
			return nil
		}
		dir := filepath.Dir(path)
		if seen[dir] {
			return nil
		}
		seen[dir] = true
		dirs = append(dirs, dir)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", testRoot, err)
	}
	if len(dirs) == 0 {
		// Deliberately fatal, not a skip. A gate that disappears when its input
		// moves reads green forever (ai/rules/evidence.md).
		t.Fatalf("no .ci directories found under %s", testRoot)
	}

	checked := 0
	for _, dir := range dirs {
		rel, relErr := filepath.Rel(testRoot, dir)
		if relErr != nil {
			t.Fatalf("relative path for %s: %v", dir, relErr)
		}
		// The suite is the first path element, so test/draft/<suite> and a
		// nested tree both resolve to the directory the dispatch names.
		suite, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
		if suite == runner.DraftDirName {
			_, after, _ := strings.Cut(filepath.ToSlash(rel), "/")
			suite, _, _ = strings.Cut(after, "/")
		}
		if nonEncodingCIDirs[suite] {
			continue
		}

		// Discover is the real entry point: it is what a suite run calls, and it
		// marks an unparseable file ParseFailed rather than returning an error.
		runner.ResetNickCounter()
		tests := runner.NewEncodingTests(baseDir)
		if discErr := tests.Discover(dir); discErr != nil {
			t.Errorf("%s: discover: %v", rel, discErr)
			continue
		}
		for _, rec := range tests.Registered() {
			checked++
			if rec.ParseFailed {
				t.Errorf("%s: %v", rec.CIFile, rec.Error)
			}
		}
	}

	if checked == 0 {
		t.Fatal("no .ci records parsed; the gate covered nothing")
	}
	t.Logf("parsed %d .ci records across %d directories", checked, len(dirs))
}
