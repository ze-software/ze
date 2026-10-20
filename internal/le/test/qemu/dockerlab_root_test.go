// Related: ../deployment/gokrazyroot_test.go -- the same proof for gokrazy-l2tp-ppp-test.

package testqemu

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// recordingGo is a go on PATH that records each invocation and builds nothing.
const recordingGo = `#!/bin/sh
echo "$*" >> "$GO_RECORD"
exit 0
`

// stageDockerLabTree answers a scratch checkout carrying the files the
// docker-lab build plans read, and puts a go stub that records each invocation
// at the head of PATH. It answers the tree and the record.
//
// The patched toolchain is activated first, over the real checkout: activation
// puts its bin at the head of PATH once per process, and the stub must stand
// before it, or the real go would answer instead.
func stageDockerLabTree(t *testing.T) (string, string) {
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

// VALIDATES: every build docker-lab starts runs under the verb's signal root,
// so the root ending (a SIGTERM to le) starts none of them: the host driver's
// compile behind the runtime-kernel query, the guest le and the linux ze.
// METHOD: the verb body and each admitted build run under a canceled root over
// a scratch checkout, with a go stub on PATH that records every invocation. A
// build rooted on context.Background runs the stub; one under the root never
// starts it.
// PREVENTS: docker-lab taking its root only after the builds again, which
// left a SIGTERM to le (or `./le job run`, which signals the le pid only)
// unable to reach the compile, and let the queue for a build slot swallow a
// SIGTERM the root had captured.
func TestDockerLabBuildsUnderTheVerbRoot(t *testing.T) {
	steps := []struct {
		name string
		run  func(ctx context.Context, tree string) bool
	}{
		{"verb", func(ctx context.Context, tree string) bool {
			_, code := runDockerLab(ctx, tree, leaction.Arguments{})
			return code != 0
		}},
		{"guest-le", func(ctx context.Context, tree string) bool {
			return buildGuestLe(ctx, tree, GuestArch()) != nil
		}},
		{"linux-ze", func(ctx context.Context, tree string) bool {
			_, err := BuildLinuxZe(ctx, tree, GuestArch())
			return err != nil
		}},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			tree, record := stageDockerLabTree(t)
			canceled, cancel := context.WithCancel(t.Context())
			cancel()

			if !step.run(canceled, tree) {
				t.Fatalf("%s under a canceled root answered success", step.name)
			}
			if recorded, err := os.ReadFile(record); err == nil { //nolint:gosec // the test's own temp file
				t.Fatalf("%s ran go under a canceled root:\n%s", step.name, recorded)
			}
		})
	}
}
