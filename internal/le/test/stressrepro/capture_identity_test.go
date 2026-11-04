package teststressrepro

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// captureFixtureRunner launches an owned executable whose assertions run in a
// separate process. No compiler or stress burner runs in these regressions.
type captureFixtureRunner struct {
	realProcessRunner
	executable string
}

func (r captureFixtureRunner) Invoke(ctx context.Context, spec invocation) processResult {
	spec.testBin = r.executable
	return r.realProcessRunner.Invoke(ctx, spec)
}

func fixtureShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// captureFixture checks each real argv element before recording a completed
// child start in an actual file; it never answers by echoing its input.
func captureFixture(t *testing.T, root string, want []string) (string, string) {
	t.Helper()
	starts := filepath.Join(root, "completed child starts")
	program := "#!/bin/sh\nset -eu\n[ \"$#\" -eq " + strconv.Itoa(len(want)) + " ]\n"
	for _, word := range want {
		program += "[ \"$1\" = " + fixtureShellQuote(word) + " ]\nshift\n"
	}
	program += "[ \"$CLAUDE_CODE_SESSION_ID\" = 'capture-fixture-session' ]\n"
	program += "[ -f \"$ZE_BIN\" ]\n"
	program += "printf 'completed\\n' >> " + fixtureShellQuote(starts) + "\n"
	executable := filepath.Join(root, "differently named harness")
	if err := os.WriteFile(executable, []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}
	return executable, starts
}

// TestCaptureIdentityRetainsLongSelectorsAndFreshStarts proves 91 selectors,
// same-prefix lists, sanitization collisions and repeated starts retain separate
// real files and exact child argv even within one timestamp.
func TestCaptureIdentityRetainsLongSelectorsAndFreshStarts(t *testing.T) {
	root := stressTree(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "capture-fixture-session")
	t.Setenv("ZE_BIN", filepath.Join(root, "bin", "ze"))
	selectors := make([]string, 91)
	for index := range selectors {
		selectors[index] = "shared-selector-prefix-" + strconv.Itoa(index)
	}
	long := strings.Join(selectors, " ")
	cases := []struct {
		name string
		test string
	}{
		{"ninety-one", long},
		{"same-prefix", long + "-different-tail"},
		{"sanitized-slash", "alpha/beta"},
		{"sanitized-hyphen", "alpha-beta"},
		{"fresh-same-selection", "alpha-beta"},
		{"spaced-argument", `'one selector with spaces'`},
	}
	logs := make(map[string]bool)
	fixed := time.Now()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			words, err := shellWords(test.test)
			if err != nil {
				t.Fatal(err)
			}
			want := append([]string{"test", "bgp", "reload", "-p", "1", "-v"}, words...)
			executable, starts := captureFixture(t, root, want)
			opts := baseOptions()
			opts.Suite, opts.Test, opts.Iterations = "reload", test.test, 2
			deps, _ := testDependencies(captureFixtureRunner{executable: executable})
			deps.now = func() time.Time { return fixed }
			report, code := run(context.Background(), root, opts, deps)
			if code != 1 || report.SetupError != "" || report.Completed != 2 {
				t.Fatalf("run did not complete two real starts: %#v, exit %d", report, code)
			}
			if len(filepath.Base(report.Log)) > 255 {
				t.Fatalf("capture component exceeds filesystem bound: %q", report.Log)
			}
			if logs[report.Log] {
				t.Fatalf("capture reused an earlier invocation's file: %q", report.Log)
			}
			logs[report.Log] = true
			capture, err := os.ReadFile(report.Log)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for line := range strings.SplitSeq(string(capture), "\n") {
				if !strings.HasPrefix(line, "argv=") {
					continue
				}
				var got []string
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "argv=")), &got); err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(got, want) {
					t.Fatalf("capture argv %q, want %q", got, want)
				}
				found = true
			}
			if !found {
				t.Fatal("capture omitted exact argv evidence")
			}
			completed, err := os.ReadFile(starts)
			if err != nil {
				t.Fatal(err)
			}
			if string(completed) != "completed\ncompleted\n" {
				t.Fatalf("children did not both execute assertions: %q", completed)
			}
			if err := os.Remove(starts); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
		})
	}
	for path := range logs {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("earlier evidence was replaced: %v", err)
		}
	}
}
