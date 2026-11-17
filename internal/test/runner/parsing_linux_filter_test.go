// Design: docs/architecture/testing/qemu-integration.md -- only needs-linux selection
// Related: parsing.go -- the parse suite's actual discovery and execution
// Related: record_parse.go -- the canonical generic Linux-only filter

package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// discoverLinuxFilterCI reaches the same Discover entry point as bgp parse.
// It returns the real suite as well, so a malformed case reaches its runner.
func discoverLinuxFilterCI(t *testing.T, content string) (*ParsingTests, *parsingTest) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "linux-filter.ci")
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	pt := NewParsingTests(dir)
	if err := pt.Discover(dir); err != nil {
		t.Fatal(err)
	}
	if pt.Count() != 1 {
		t.Fatalf("discovered %d tests, want one", pt.Count())
	}
	return pt, pt.Registered()[0]
}

// TestParseCILinuxOnlySkipsUnmarked compares the parse suite's selection with
// the real generic reader, both with the Linux-only mode on and with it off.
// MAIN must observe the enabled case fail semantically before the shared fix.
func TestParseCILinuxOnlySkipsUnmarked(t *testing.T) {
	for _, mode := range []string{"1", "0"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("ZE_QEMU_LINUX_ONLY", mode)
			pt, test := discoverLinuxFilterCI(t, dialectBaseCI+"expect=exit:code=0\n")
			if test.ParseError != nil {
				t.Fatal(test.ParseError)
			}
			shared, err := NewEncodingTests(pt.baseDir).parseAndAdd(test.File)
			if err != nil {
				t.Fatalf("generic selection control failed: %v", err)
			}
			if mode == "1" {
				if shared.SkipReason == "" {
					t.Fatal("generic Linux-only control did not exclude the unmarked question")
				}
			}
			if mode == "0" {
				if shared.SkipReason != "" {
					t.Fatalf("generic ordinary-mode control skipped: %s", shared.SkipReason)
				}
			}
			if test.SkipReason != shared.SkipReason {
				t.Fatalf("parse selection = %q, want the canonical generic reason %q", test.SkipReason, shared.SkipReason)
			}
		})
	}
}

// TestParseCILinuxOnlyRetainsMarkedAdmission keeps the actual host's shared
// admission: runnable on Linux, or the original non-Linux host skip elsewhere.
func TestParseCILinuxOnlyRetainsMarkedAdmission(t *testing.T) {
	t.Setenv("ZE_QEMU_LINUX_ONLY", "1")
	pt, test := discoverLinuxFilterCI(t, "option=needs-linux\n"+dialectBaseCI+"expect=exit:code=0\n")
	if test.ParseError != nil {
		t.Fatal(test.ParseError)
	}
	shared, err := NewEncodingTests(pt.baseDir).parseAndAdd(test.File)
	if err != nil {
		t.Fatal(err)
	}
	if !shared.NeedsLinux {
		t.Fatal("the shared control lost its Linux declaration")
	}
	if test.SkipReason != shared.SkipReason {
		t.Fatalf("marked admission = %q, want actual shared host admission %q", test.SkipReason, shared.SkipReason)
	}
	if runtime.GOOS == "linux" {
		if test.SkipReason != "" {
			t.Fatalf("marked Linux question was filtered on Linux: %s", test.SkipReason)
		}
		return
	}
	if test.SkipReason == "" {
		t.Fatal("the non-Linux host's shared admission was dropped")
	}
}

// TestParseCILinuxOnlyMalformedStillFails ensures filtering cannot hide a
// broken unmarked file. It must fail through discovery and the real runner.
func TestParseCILinuxOnlyMalformedStillFails(t *testing.T) {
	t.Setenv("ZE_QEMU_LINUX_ONLY", "1")
	pt, test := discoverLinuxFilterCI(t, dialectBaseCI+"expect=stdout:pattern=[unterminated\n")
	if test.ParseError == nil {
		t.Fatal("malformed unmarked question did not fail discovery")
	}
	if !strings.Contains(test.ParseError.Error(), "invalid expect=stdout:pattern=") {
		t.Fatalf("malformed question failed for another reason: %v", test.ParseError)
	}
	test.SetActive(true)
	runner := NewParsingRunner(pt, pt.baseDir, filepath.Join(pt.baseDir, "ze-must-not-run"))
	if runner.Run(context.Background(), false, true) {
		t.Fatal("Linux-only filtering turned a malformed unmarked question into skipped success")
	}
}
