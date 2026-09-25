package runner

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/harnessbin"
)

// captureStderr runs fn with os.Stderr redirected, and answers what fn wrote.
// env.Get writes its deprecation warning to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = writer
	fn()
	os.Stderr = saved
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	written, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(written)
}

// TestHarnessVariablesReadBothNames proves each harness variable the runner
// reads answers under its LE_ spelling, its retired ZE_ spelling, or both
// (AC-9).
//
// Method: for le.test.bin and le.test.no.build, set one spelling, the other,
// then both, and read through harnessbin, the resolver the runner calls. The
// LE_ value MUST win, and only a value that came from the ZE_ spelling prints
// the deprecation line, once, naming the LE_ spelling. NewRunner closes the
// loop for le.test.bin: its harness path is the resolved value.
//
// VALIDATES: AC-9, the LE_ spelling wins and the ZE_ spelling warns once.
// PREVENTS: an environment that sets only ZE_TEST_BIN going unread.
func TestHarnessVariablesReadBothNames(t *testing.T) {
	t.Cleanup(env.ResetCache)
	variables := []struct {
		fresh, retired string
		value          string
		read           func() string
	}{
		{harnessbin.EnvTestBin, "ZE_TEST_BIN", "/opt/harness", harnessbin.TestBin},
		{harnessbin.EnvNoBuild, "ZE_TEST_NO_BUILD", "1", func() string {
			if harnessbin.NoBuild() {
				return "1"
			}
			return ""
		}},
	}
	for _, variable := range variables {
		cases := []struct {
			name, fresh, retired, want string
			warns                      bool
		}{
			{"LE_ only", variable.value, "", variable.value, false},
			{"ZE_ only", "", variable.value, variable.value, true},
			{"both", variable.value, "junk", variable.value, false},
			{"neither", "", "", "", false},
		}
		for _, tc := range cases {
			t.Run(variable.fresh+"/"+tc.name, func(t *testing.T) {
				t.Setenv(variable.fresh, tc.fresh)
				t.Setenv(variable.retired, tc.retired)
				env.ResetCache()
				var first, second string
				warning := captureStderr(t, func() {
					first = variable.read()
					second = variable.read()
				})
				if first != tc.want || second != tc.want {
					t.Errorf("read %q then %q, want %q", first, second, tc.want)
				}
				lines := strings.Count(warning, "deprecated")
				if !tc.warns {
					if lines != 0 {
						t.Errorf("unexpected warning %q", warning)
					}
					return
				}
				if lines != 1 || !strings.Contains(warning, variable.fresh) {
					t.Errorf("warning %q, want one line naming %s", warning, variable.fresh)
				}
			})
		}
	}

	baseDir := t.TempDir()
	t.Setenv(harnessbin.EnvTestBin, "")
	t.Setenv("ZE_TEST_BIN", "old-harness")
	env.ResetCache()
	r, err := NewRunner(NewEncodingTests(baseDir), baseDir)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if want := filepath.Join(baseDir, "old-harness"); r.testPath != want {
		t.Errorf("runner harness path %q, want %q", r.testPath, want)
	}
}
