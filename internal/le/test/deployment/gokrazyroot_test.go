// Related: proofroot_test.go -- the same proof for the docker-backed deployment proofs.

package testdeployment

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

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
