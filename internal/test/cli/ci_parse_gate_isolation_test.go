package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/runner"
)

// TestEncodingCIGateIsolatesDrafts keeps the repo-wide parser gate on canonical
// inputs while an explicit draft run still refuses malformed draft content.
func TestEncodingCIGateIsolatesDrafts(t *testing.T) {
	base := t.TempDir()
	testRoot := filepath.Join(base, "test")
	const valid = "cmd=foreground:seq=1:exec=echo canonical\nexpect=exit:code=0\n"
	const malformed = "expect=stdout:not-contains=forbidden\n"
	canonical := []string{
		"plugin/valid.ci",
		"plugin/nested/nested.ci",
		"plugin/draft/nested-draft.ci",
		"drafting/prefix.ci",
	}
	files := map[string]string{
		"plugin/malformed.ci": malformed,
		"draft/plugin/wip.ci": malformed,
	}
	for _, path := range canonical {
		files[path] = valid
	}
	for relative, content := range files {
		path := filepath.Join(testRoot, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	dirs, err := encodingCIGateDirs(testRoot)
	if err != nil {
		t.Fatal(err)
	}
	records := make(map[string]*runner.Record)
	for _, dir := range dirs {
		runner.ResetNickCounter()
		tests := runner.NewEncodingTests(base)
		if err := tests.Discover(dir); err != nil {
			t.Fatalf("discover %s: %v", dir, err)
		}
		for _, rec := range tests.Registered() {
			records[rec.CIFile] = rec
		}
	}

	t.Run("canonical-survivors", func(t *testing.T) {
		for _, relative := range canonical {
			path := filepath.Join(testRoot, filepath.FromSlash(relative))
			rec, found := records[path]
			if !found {
				t.Errorf("canonical file disappeared from the gate: %s", relative)
				continue
			}
			if rec.ParseFailed {
				t.Errorf("canonical file did not parse: %s: %v", relative, rec.Error)
			}
		}
	})
	t.Run("draft-isolation", func(t *testing.T) {
		path := filepath.Join(testRoot, runner.DraftDirName, "plugin", "wip.ci")
		if _, found := records[path]; found {
			t.Error("repo-wide parser gate discovered a draft under development")
		}
	})
	t.Run("malformed-canonical-stays-failed", func(t *testing.T) {
		path := filepath.Join(testRoot, "plugin", "malformed.ci")
		rec, found := records[path]
		if !found {
			t.Fatal("malformed canonical file disappeared from the gate")
		}
		assertEncodingCIGateParseFailure(t, rec)
	})
	t.Run("explicit-draft-stays-failed", func(t *testing.T) {
		runner.ResetNickCounter()
		tests := runner.NewEncodingTests(base)
		if err := tests.Discover(runner.SuiteDir(base, "plugin", true)); err != nil {
			t.Fatal(err)
		}
		records := tests.Registered()
		if len(records) != 1 {
			t.Fatalf("explicit draft discovery recorded %d files, want one", len(records))
		}
		assertEncodingCIGateParseFailure(t, records[0])
	})
}

func assertEncodingCIGateParseFailure(t *testing.T, rec *runner.Record) {
	t.Helper()
	if !rec.ParseFailed {
		t.Fatal("malformed file was not recorded as ParseFailed")
	}
	if rec.Error == nil {
		t.Fatal("malformed file lost its parse error")
	}
	if !strings.Contains(rec.Error.Error(), "unknown key") {
		t.Fatalf("malformed file failed for an unrelated reason: %v", rec.Error)
	}
}
