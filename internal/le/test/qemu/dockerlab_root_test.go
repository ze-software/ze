// Design: docs/architecture/testing/qemu-integration.md -- Docker labs in the Ze-kernel guest
// Related: ../deployment/gokrazyroot_test.go -- the same proof for gokrazy-l2tp-ppp.

package testqemu

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
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
// The tree is a git repository with one commit, because job admission
// fingerprints its inputs from HEAD: a tree with no HEAD refuses every
// admitted build before its root is ever asked. ParentVariable is cleared, so
// a run under `./le job run` admits through the queue as a raw `go test` does,
// rather than running inside the job that started the test.
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

	commitTree(t, tree)
	t.Setenv(job.ParentVariable, "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(recordingGo), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
		t.Fatalf("write the go stub: %v", err)
	}
	record := filepath.Join(bin, "record")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_RECORD", record)
	return tree, record
}

// commitTree makes tree a git repository whose one commit holds every file
// already in it.
func commitTree(t *testing.T, tree string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"add", "."},
		{"commit", "--quiet", "-m", "fixture"},
	} {
		command := exec.CommandContext(t.Context(), "git", args...)
		// This private fixture must not inherit the operator's signing key or
		// hooks. It is not a commit to the development checkout.
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		command.Dir = tree
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v in the scratch tree: %v\n%s", args, err, out)
		}
	}
}

// VALIDATES: every build docker-lab starts runs under the verb's signal root,
// so the root ending (a SIGTERM to le) starts none of them: the host driver's
// compile behind the runtime-kernel query, the guest le and the linux ze.
// METHOD: the verb body and each admitted build run under a canceled root over
// a scratch checkout, with a go stub on PATH that records every invocation.
// The admitted builds go through the queue, whether the test runs raw or under
// `./le job run`. A build rooted on context.Background runs the stub; one
// under the root never starts it.
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
			_, code := runDockerLab(ctx, tree, leaction.Arguments{}, "")
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

// VALIDATES: a guest-le or linux-ze build that the verb's root stopped says an
// interrupt or SIGTERM stopped it, and names no exit code.
// METHOD: each admitted build runs under a canceled root over the staged
// checkout; its error must say it was stopped and must not say "exited".
// PREVENTS: a SIGTERM to le reading as "build the guest le ... exited 2",
// which reads as a compile failure.
func TestDockerLabBuildsSayTheyWereStopped(t *testing.T) {
	builds := []struct {
		name  string
		build func(ctx context.Context, tree string) error
	}{
		{"guest-le", func(ctx context.Context, tree string) error {
			return buildGuestLe(ctx, tree, GuestArch())
		}},
		{"linux-ze", func(ctx context.Context, tree string) error {
			_, err := BuildLinuxZe(ctx, tree, GuestArch())
			return err
		}},
	}
	for _, step := range builds {
		t.Run(step.name, func(t *testing.T) {
			tree, _ := stageDockerLabTree(t)
			canceled, cancel := context.WithCancel(t.Context())
			cancel()

			err := step.build(canceled, tree)
			if err == nil {
				t.Fatalf("%s under a canceled root answered success", step.name)
			}
			if !strings.Contains(err.Error(), "stopped by an interrupt or SIGTERM") {
				t.Errorf("%s answered %q, want it to say the build was stopped", step.name, err)
			}
			if strings.Contains(err.Error(), "exited") {
				t.Errorf("%s answered %q, which names an exit code for a stop", step.name, err)
			}
		})
	}
}
