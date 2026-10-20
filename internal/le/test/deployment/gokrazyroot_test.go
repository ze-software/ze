// Design: docs/architecture/testing/runner-architecture.md -- How a suite's child processes end
// Related: proofroot_test.go -- the same proof for the docker-backed deployment proofs.

package testdeployment

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// stageRootedBuildTree answers a scratch checkout carrying the files the host
// driver's build plan and the runtime-kernel check read, and puts a go stub
// that records each invocation at the head of PATH. It answers the tree and
// the record.
//
// The patched toolchain is activated first, over the real checkout: activation
// puts its bin at the head of PATH once per process, and the stub must stand
// before it, or the real go would answer instead.
func stageRootedBuildTree(t *testing.T) (string, string) {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout: %v", err)
	}
	if _, err := gotoolchain.New(root); err != nil {
		t.Fatalf("activate the toolchain: %v", err)
	}

	tree := t.TempDir()
	for _, rel := range []string{"go.mod", "feature-gates.txt", KernelVersionFile, filepath.Join("gokrazy", "ze", "config.json")} {
		body, err := os.ReadFile(filepath.Join(root, rel)) //nolint:gosec // the checkout's own files
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(tree, rel)), 0o750); err != nil {
			t.Fatalf("make the directory of %s: %v", rel, err)
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

// VALIDATES: the gokrazy-l2tp-ppp-test image build runs under the verb's
// signal root, so the root ending (a SIGTERM to le) starts no go build: the
// host driver's compile, the first child of the build, is the one a canceled
// root must not reach.
// METHOD: gokrazyImage runs under a canceled root over a scratch checkout,
// with a go stub on PATH that records every invocation. A build rooted on
// context.Background runs the stub; one under the root never starts it.
// PREVENTS: the image build taking context.Background again, which a SIGTERM
// to le (or `./le job run`, which signals the le pid only) cannot reach, so
// the compile and the gok child ran to imageBuildTimeout after le was gone.
func TestGokrazyImageBuildsUnderTheVerbRoot(t *testing.T) {
	tree, record := stageRootedBuildTree(t)
	t.Setenv(GokrazySkipBuildEnv, "")
	t.Setenv(GokrazyImageEnv, "")
	work := t.TempDir()
	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := gokrazyImage(canceled, tree, work, filepath.Join(work, "ze.conf"), ArchAMD64, io.Discard); err == nil {
		t.Fatal("the image build under a canceled root answered no error")
	}
	if recorded, err := os.ReadFile(record); err == nil { //nolint:gosec // the test's own temp file
		t.Fatalf("the image build ran go under a canceled root:\n%s", recorded)
	}
}

// VALIDATES: a gokrazy proof run whose root ended answers the stop as an
// error, never a verdict, so a SIGTERM is not reported as a failed L2TP path.
// METHOD: stoppedRun wraps the root's cause, which errors.Is still reaches.
// PREVENTS: a signaled run reading as "appliance web server did not start".
func TestGokrazyStoppedRunCarriesTheCause(t *testing.T) {
	err := stoppedRun(context.Canceled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("stoppedRun lost its cause: %v", err)
	}
}

// grandchildSpawner is a stand-in for the child le: it starts a long sleeper
// of its own, prints the sleeper's pid, and waits, the shape of a gok build
// whose compiler processes run below the child le.
const grandchildSpawner = "#!/bin/sh\nsleep 300 &\necho $!\nwait\n"

// VALIDATES: a cancel during the gokrazy image build ends the build, the
// processes the child le started included, as runner-architecture.md says.
// METHOD: gokrazyBuildCommand runs a stub child that starts a grandchild
// sleeper and prints its pid; the context is canceled once the pid arrives,
// and the grandchild must then be gone (kill(pid, 0) answers ESRCH).
// PREVENTS: the default CommandContext cancel, which kills the child le alone,
// leaving the compilers of a canceled build running after le stopped.
func TestGokrazyBuildCancelEndsTheGrandchildren(t *testing.T) {
	stub := filepath.Join(t.TempDir(), "le")
	if err := os.WriteFile(stub, []byte(grandchildSpawner), 0o755); err != nil { //nolint:gosec // a stub child must be executable
		t.Fatalf("write the stub child: %v", err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("make the progress pipe: %v", err)
	}
	defer reader.Close() //nolint:errcheck // the test's own pipe

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	run := gokrazyBuildCommand(ctx, stub, t.TempDir(), os.Environ(), writer)
	if err := run.Start(); err != nil {
		t.Fatalf("start the stub child: %v", err)
	}

	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil {
		t.Fatalf("read the grandchild pid: %v", err)
	}
	grandchild, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("the stub child printed %q, not a pid: %v", line, err)
	}
	t.Cleanup(func() { syscall.Kill(grandchild, syscall.SIGKILL) }) //nolint:errcheck // the sleeper is gone when the fix holds

	cancel()
	if err := run.Wait(); err == nil {
		t.Fatal("a canceled build answered no error")
	}
	writer.Close() //nolint:errcheck,gosec // the test's own pipe
	// The group's SIGKILL is delivered at once, but the killed sleeper is
	// reaped by init a moment later, so its absence is awaited, bounded.
	deadline := time.Now().Add(10 * time.Second)
	for !errors.Is(syscall.Kill(grandchild, 0), syscall.ESRCH) {
		if time.Now().After(deadline) {
			t.Fatalf("grandchild %d still runs after the build was canceled", grandchild)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
