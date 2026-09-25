// VALIDATES: a runner command admits its run through internal/le/job when a
// checkout resolves, runs in its parent's slot when one is held, runs
// unadmitted when no checkout exists (a container), and a helper tool never
// admits (AC-45 of spec-le-subject-first-command-tree).
// PREVENTS: a suite run that oversubscribes the host, a container run that
// fails on a missing registry, and a helper that queues behind its own suite.
package harnesstool

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// hostFacts points admitRun at a checkout and a child for one test, clears any
// parent job the test process inherited, and restores all three afterwards.
func hostFacts(t *testing.T, root string, rootErr error, self string) {
	t.Helper()
	savedRoot, savedSelf := checkoutRoot, selfExecutable
	savedParent := env.Get(job.ParentKey)
	checkoutRoot = func() (string, error) { return root, rootErr }
	selfExecutable = func() (string, error) { return self, nil }
	setParent(t, "")
	t.Cleanup(func() {
		checkoutRoot, selfExecutable = savedRoot, savedSelf
		setParent(t, savedParent)
	})
}

func setParent(t *testing.T, entry string) {
	t.Helper()
	if err := env.Set(job.ParentKey, entry); err != nil {
		t.Fatalf("set %s: %v", job.ParentKey, err)
	}
}

// TestHarnessSuiteAdmitsOnHostOnly drives the answer RunnerAnswer builds, the
// one a suite's register.go hands to leroot.Register, through its four cases.
// The claimed case re-runs the command as a child: here the child is a shell
// script that records its words and the parent entry it was given.
func TestHarnessSuiteAdmitsOnHostOnly(t *testing.T) {
	const runner = "test zzrunner"
	inProcess := 0
	answer := RunnerAnswer(runner, func([]string) int {
		inProcess++
		return 3
	})
	if !leroot.Admits(runner) {
		t.Fatal("RunnerAnswer did not mark its command admitted")
	}

	t.Run("no checkout runs unadmitted", func(t *testing.T) {
		hostFacts(t, "", lepath.ErrNoCheckout, "/nonexistent")
		inProcess = 0
		if _, code := answer([]string{"a"}); code != 3 || inProcess != 1 {
			t.Errorf("code %d, in-process runs %d: want the handler's 3, run once", code, inProcess)
		}
	})

	t.Run("claimed slot runs the command as a child", func(t *testing.T) {
		root := t.TempDir()
		record := filepath.Join(root, "child.txt")
		self := filepath.Join(root, "le")
		script := "#!/bin/sh\n[ -f \"$ZE_RUN_JOB\" ] || exit 9\necho \"$@\" > " + record + "\nexit 5\n"
		if err := os.WriteFile(self, []byte(script), 0o700); err != nil { //nolint:gosec // an executable test child
			t.Fatal(err)
		}
		hostFacts(t, root, nil, self)
		inProcess = 0
		_, code := answer([]string{"a", "b"})
		if code != 5 {
			t.Fatalf("code %d, want the child's 5 (9 means it held no parent entry)", code)
		}
		if inProcess != 0 {
			t.Errorf("the handler ran in-process %d times while this process held the slot", inProcess)
		}
		words, err := os.ReadFile(record) //nolint:gosec // written by the test child above
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(string(words)); got != "test zzrunner a b" {
			t.Errorf("child ran with %q, want %q", got, "test zzrunner a b")
		}
		entries, err := os.ReadDir(filepath.Join(root, job.JobsDir))
		if err != nil {
			t.Fatalf("admission wrote no registry: %v", err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "test-zzrunner") && strings.HasSuffix(entry.Name(), ".job") {
				t.Errorf("slot %s was not released", entry.Name())
			}
		}
	})

	t.Run("inside a parent runs in its slot", func(t *testing.T) {
		root := t.TempDir()
		hostFacts(t, root, nil, "/nonexistent")
		parent := filepath.Join(root, "parent.job")
		if err := os.WriteFile(parent, []byte("LABEL=functional\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		setParent(t, parent)
		inProcess = 0
		if _, code := answer(nil); code != 3 || inProcess != 1 {
			t.Errorf("code %d, in-process runs %d: want the handler's 3, run once", code, inProcess)
		}
	})

	t.Run("a helper tool never admits", func(t *testing.T) {
		root := t.TempDir()
		hostFacts(t, root, nil, "/nonexistent")
		helper := Answer(func([]string) int { return 4 })
		if _, code := helper([]string{"--help"}); code != 4 {
			t.Errorf("code %d, want the handler's 4", code)
		}
		if _, err := os.Stat(filepath.Join(root, job.JobsDir)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("a helper tool touched the job registry: %v", err)
		}
	})
}
