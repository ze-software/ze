// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: dockerkernel.go -- runDockerKernelCheck, the signal root these tests drive.

package dockerkernel

import (
	"context"
	"debug/elf"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// ctxRecorder is the check's docker process boundary: it records whether
// each command was issued under a live context, and answers like the process
// runner does for an ended one.
type ctxRecorder struct {
	live int
}

func (r *ctxRecorder) Run(ctx context.Context, _ interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	if err := ctx.Err(); err != nil {
		return interoplab.ProcessResult{}, err
	}
	r.live++
	return interoplab.ProcessResult{Stdout: "amd64\n"}, nil
}

// VALIDATES: `./le setup docker-kernel check` runs its docker commands under
// the verb's root, so the root ending (a Ctrl-C reaching the signal root)
// ends them.
// METHOD: the check runs under a canceled root against a recording double;
// a check rooted on context.Background issues its daemon query live.
// PREVENTS: a Ctrl-C that the docker client, in a process group of its own,
// never sees, leaving the check running to its bound.
func TestDockerKernelCheckRunsUnderTheVerbRoot(t *testing.T) {
	recorder := &ctxRecorder{}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	if _, code := dockerKernelCheck(canceled, interoplab.NewFixtureDocker(recorder), ""); code != 1 {
		t.Fatalf("a check under a canceled root answered %d, want 1", code)
	}
	if recorder.live != 0 {
		t.Fatalf("a check under a canceled root issued %d docker commands live, want 0", recorder.live)
	}
}

// cancelingRecorder answers the daemon's architecture live, then ends the
// root: every later command is the kernel probe, and it records whether the
// probe was issued under a live context.
type cancelingRecorder struct {
	cancel context.CancelFunc
	live   int
}

func (r *cancelingRecorder) Run(ctx context.Context, _ interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	if err := ctx.Err(); err != nil {
		return interoplab.ProcessResult{}, err
	}
	r.live++
	r.cancel()
	return interoplab.ProcessResult{Stdout: "amd64\n"}, nil
}

// VALIDATES: the kernel probe itself runs under the verb's root, not only the
// architecture query before it.
// METHOD: an operator-named amd64 ELF skips the build; the double answers the
// architecture query live and then cancels the root. A probe bounded from
// context.Background issues its commands live after the cancel; one under the
// root issues none, so exactly one command ran live.
// PREVENTS: dockerKernelCheck bounding the probe from context.Background,
// which TestDockerKernelCheckRunsUnderTheVerbRoot cannot see because its root
// is canceled before the architecture query.
func TestDockerKernelProbeRunsUnderTheVerbRoot(t *testing.T) {
	named := writeELF(t, elf.EM_X86_64)
	root, cancel := context.WithCancel(t.Context())
	defer cancel()
	recorder := &cancelingRecorder{cancel: cancel}

	if _, code := dockerKernelCheck(root, interoplab.NewFixtureDocker(recorder), named); code != 1 {
		t.Fatalf("a check whose root ended after the architecture query answered %d, want 1", code)
	}
	if recorder.live != 1 {
		t.Fatalf("the check issued %d docker commands live, want 1 (the architecture query alone)", recorder.live)
	}
}

// recordingGo is a go on PATH that records each invocation and builds nothing.
const recordingGo = `#!/bin/sh
echo "$*" >> "$GO_RECORD"
exit 0
`

// stageInstallTree answers a scratch checkout carrying the files the ze-host
// build plan reads, and puts a go stub that records each invocation at the
// head of PATH. It answers the tree and the record.
//
// The patched toolchain is activated first, over the real checkout:
// activation puts its bin at the head of PATH once per process, and the stub
// must stand before it, or the real go would answer instead.
func stageInstallTree(t *testing.T) (string, string) {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout: %v", err)
	}
	if _, err := gotoolchain.New(root); err != nil {
		t.Fatalf("activate the toolchain: %v", err)
	}

	tree := t.TempDir()
	for _, rel := range []string{"go.mod", "feature-gates.txt"} {
		body, err := os.ReadFile(filepath.Join(root, rel)) //nolint:gosec // the checkout's own files
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if err := os.WriteFile(filepath.Join(tree, rel), body, 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(recordingGo), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
		t.Fatalf("write the go stub: %v", err)
	}
	record := filepath.Join(bin, "record")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_RECORD", record)
	return tree, record
}

// VALIDATES: `./le setup docker-kernel install` resolves its cache entry
// under the verb's root, so a Ctrl-C ends the ze-host compile behind the
// query, which heads a process group of its own and never sees the signal.
// METHOD: installCache runs the real runtime-kernel query under a canceled
// root over a scratch checkout, with a go stub on PATH that records every
// invocation. A query rooted on context.Background runs the stub; one under
// the root never starts it.
// PREVENTS: the install verb holding no signal root, so a Ctrl-C killed le
// and left the ze-host compile running on in its own group.
func TestDockerKernelInstallCacheBuildsUnderTheVerbRoot(t *testing.T) {
	tree, record := stageInstallTree(t)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := installCache(canceled, tree, ""); err == nil {
		t.Fatal("the cache query under a canceled root answered success")
	}
	if recorded, err := os.ReadFile(record); err == nil { //nolint:gosec // the test's own temp file
		t.Fatalf("the cache query ran go under a canceled root:\n%s", recorded)
	}
}

// stepBound is how long a case waits for a root step to answer once its root
// has ended. A step the root stops answers at once.
const stepBound = 10 * time.Second

// VALIDATES: a root step of the install starts nothing under an ended root,
// and a running one is sent SIGTERM when the root ends, each answering that
// an interrupt or SIGTERM stopped it.
// METHOD: one step runs under a canceled root and must not create its marker.
// A second runs a shell whose trap records SIGTERM, and the root is canceled
// once the shell is up: the step must answer inside stepBound with the trap
// fired. A step started with no ctx outlasts the bound; one canceled with
// SIGKILL, which sudo cannot relay, never runs the trap.
// PREVENTS: a SIGTERM to le, now captured by the install's root, leaving a
// root step running and the next one starting.
func TestDockerKernelInstallStepsStopWithTheRoot(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	err := runInstallStep(canceled, installStep{Why: "probe", Argv: []string{"sh", "-c", "echo ran > " + marker}})
	if err == nil || !strings.Contains(err.Error(), "stopped by an interrupt or SIGTERM") {
		t.Fatalf("a step under a canceled root answered %v, want it stopped", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("a step ran under a canceled root")
	}

	up := filepath.Join(dir, "up")
	termed := filepath.Join(dir, "termed")
	script := "trap 'echo term > " + termed + "; kill $child; exit 143' TERM; " +
		"sleep 30 >/dev/null 2>&1 & child=$!; echo up > " + up + "; wait"
	root, stop := context.WithCancel(t.Context())
	defer stop()
	answered := make(chan error, 1)
	go func() {
		answered <- runInstallStep(root, installStep{Why: "probe", Argv: []string{"sh", "-c", script}})
	}()
	deadline := time.Now().Add(stepBound)
	for !exists(up) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	stop()

	select {
	case err := <-answered:
		if err == nil || !strings.Contains(err.Error(), "stopped by an interrupt or SIGTERM") {
			t.Errorf("a step its root stopped answered %v, want it stopped", err)
		}
	case <-time.After(stepBound):
		t.Fatal("a running step outlasted its root")
	}
	if !exists(termed) {
		t.Error("the step never saw SIGTERM")
	}
}

// exists reports whether path is there.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// VALIDATES: the linux ze the check builds for the daemon is built under the
// verb's root, so a Ctrl-C that lands during the cross-compile ends it.
// METHOD: no ze is named, so the check builds one; the double answers the
// architecture query live and then cancels the root, and the builder records
// whether its ctx had ended. A build closure that passes
// context.Background builds under a live ctx.
// PREVENTS: the cross-compile behind `setup docker-kernel check` running on
// in its own process group after the operator stopped the check.
func TestDockerKernelCheckBuildsUnderTheVerbRoot(t *testing.T) {
	saved := linuxZe
	t.Cleanup(func() { linuxZe = saved })
	var builds, live int
	linuxZe = func(ctx context.Context, _, _ string) (string, error) {
		builds++
		if ctx.Err() == nil {
			live++
		}
		return "", context.Canceled
	}
	root, cancel := context.WithCancel(t.Context())
	defer cancel()
	recorder := &cancelingRecorder{cancel: cancel}

	if _, code := dockerKernelCheck(root, interoplab.NewFixtureDocker(recorder), ""); code != 1 {
		t.Fatalf("a check whose root ended before its build answered %d, want 1", code)
	}
	if builds != 1 {
		t.Fatalf("the check asked for %d linux ze builds, want 1", builds)
	}
	if live != 0 {
		t.Fatal("the check built its linux ze under a live ctx after its root ended")
	}
}
