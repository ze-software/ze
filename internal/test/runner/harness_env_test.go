package runner

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	leroot "github.com/ze-software/ze/internal/le/le/root"
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

// runShim runs one shim with args and answers its standard output.
func runShim(path string, args ...string) (string, error) {
	out, err := exec.Command(path, args...).Output() //nolint:gosec // a shim this test wrote
	return string(out), err
}

// TestNoBuildReadsBothNames proves le.test.no.build answers under its LE_
// spelling, its retired ZE_ spelling, or both (AC-42: the variable stays, and
// moves into this package).
//
// Method: set one spelling, the other, then both, and read through NoBuild,
// the resolver Build calls. The LE_ value MUST win, and only a value that came
// from the ZE_ spelling prints the deprecation line, once, naming the LE_
// spelling.
//
// VALIDATES: AC-42, the LE_ spelling wins and the ZE_ spelling warns once.
// PREVENTS: an environment that sets only ZE_TEST_NO_BUILD going unread.
func TestNoBuildReadsBothNames(t *testing.T) {
	t.Cleanup(env.ResetCache)
	cases := []struct {
		name, fresh, retired string
		want, warns          bool
	}{
		{"LE_ only", "1", "", true, false},
		{"ZE_ only", "", "1", true, true},
		{"both", "1", "junk", true, false},
		{"neither", "", "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvNoBuild, tc.fresh)
			t.Setenv("ZE_TEST_NO_BUILD", tc.retired)
			env.ResetCache()
			var first, second bool
			warning := captureStderr(t, func() {
				first = NoBuild()
				second = NoBuild()
			})
			if first != tc.want || second != tc.want {
				t.Errorf("read %v then %v, want %v", first, second, tc.want)
			}
			lines := strings.Count(warning, "deprecated")
			if !tc.warns {
				if lines != 0 {
					t.Errorf("unexpected warning %q", warning)
				}
				return
			}
			if lines != 1 || !strings.Contains(warning, EnvNoBuild) {
				t.Errorf("warning %q, want one line naming %s", warning, EnvNoBuild)
			}
		})
	}
}

// TestRunnerUsesItsOwnExecutable proves every `le` exec head runs the runner's
// own executable (AC-37), that no harness variable redirects it, and that no
// retired harness head reaches it (AC-43).
//
// Method: NewRunner with a stale LE_TEST_BIN exported, then read lePath and
// resolve each head through resolveParseExec.
//
// VALIDATES: AC-37, the head `le`; AC-43, heads `le-test`, `ze-test` and
// `ze-peer` are left to a PATH lookup.
// PREVENTS: a .ci step reaching whichever `le` a PATH lookup finds, or a
// harness binary named by a variable that no longer exists.
func TestRunnerUsesItsOwnExecutable(t *testing.T) {
	t.Setenv("LE_TEST_BIN", "/stale/harness")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	baseDir := t.TempDir()
	r, err := NewRunner(NewEncodingTests(baseDir), baseDir)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	defer r.Cleanup()
	own, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if r.lePath != own {
		t.Errorf("runner le path %q, want its own executable %q", r.lePath, own)
	}

	if got, err := resolveParseExec("le test peer --mode sink", "/bin/ze", own); err != nil || got != own+" test peer --mode sink" {
		t.Errorf("head le resolved to %q (%v), want the runner's own executable", got, err)
	}
	for _, head := range []string{"le-test", "ze-test", "ze-peer", "lean", "sh", "./le"} {
		line := head + " bgp"
		if got, err := resolveParseExec(line, "/bin/ze", own); err != nil || got != line {
			t.Errorf("head %q resolved to %q (%v), want it left for a PATH lookup", head, got, err)
		}
	}
}

// TestRunnerShimsResolveLeToItself proves the child PATH directory holds `le`
// linked to the runner's executable and no retired harness name, that no child
// environment carries ZE_LE_BUILD_NAME, and that
// `le test <harness command>` runs in the work directory while any other `le`
// head keeps the repository root (AC-37).
//
// Method: setupBinShims on a runner whose le is a stand-in script that prints
// its arguments, run the `le` link, and read what reached the stand-in.
// A forwarding `test` command is registered so leHarnessArea has one to find.
//
// VALIDATES: AC-37, the shims, the dropped variable and the working directory.
// PREVENTS: a daemon-spawned `le test engine-steps` finding no `le`, or every
// child refused by refuseWrongBuildName under `./le --name x`.
func TestRunnerShimsResolveLeToItself(t *testing.T) {
	dir := t.TempDir()
	standIn := filepath.Join(dir, "stand-in")
	if err := os.WriteFile(standIn, []byte("#!/bin/sh\necho \"$@\"\n"), 0o750); err != nil { //nolint:gosec // test stand-in executable
		t.Fatalf("write stand-in: %v", err)
	}
	r := &Runner{tmpDir: dir, zePath: filepath.Join(dir, "ze"), lePath: standIn, baseDir: "/repo"}
	if err := r.setupBinShims(); err != nil {
		t.Fatalf("setupBinShims: %v", err)
	}
	target, err := os.Readlink(filepath.Join(r.binShimDir, "le"))
	if err != nil || target != standIn {
		t.Fatalf("le shim links to %q (%v), want %q", target, err, standIn)
	}
	out, err := runShim(filepath.Join(r.binShimDir, "le"), "test", "bgp", "--list")
	if err != nil {
		t.Fatalf("run the le link: %v", err)
	}
	if got := strings.TrimSpace(out); got != "test bgp --list" {
		t.Errorf("the le link answered %q, want %q", got, "test bgp --list")
	}
	for _, head := range []string{"le-test", "ze-test", "ze-peer"} {
		if _, statErr := os.Lstat(filepath.Join(r.binShimDir, head)); !os.IsNotExist(statErr) {
			t.Errorf("the child PATH directory holds the retired name %s (%v)", head, statErr)
		}
	}

	t.Setenv("ZE_LE_BUILD_NAME", "p1")
	t.Setenv("ze.le.build.name", "p1")
	for _, entry := range childEnv() {
		name, _, _ := strings.Cut(entry, "=")
		if droppedFromChild(name) {
			t.Errorf("child environment carries %q", entry)
		}
	}

	const harnessWords = "test shim-probe"
	leroot.RegisterForwarding(harnessWords)
	rec := &Record{WorkDir: "/scratch/ze-work-1"}
	if got := r.childWorkingDirectory("le", []string{"test", "shim-probe", "x"}, rec); got != rec.WorkDir {
		t.Errorf("le %s ran in %q, want the work directory", harnessWords, got)
	}
	if got := r.childWorkingDirectory("le", []string{"test", "unit"}, rec); got != "/repo" {
		t.Errorf("le test unit ran in %q, want the repository root", got)
	}
}
