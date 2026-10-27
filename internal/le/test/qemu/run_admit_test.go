package testqemu

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// VALIDATES: the setup line every `test qemu run` guest command runs after
// leaves job.ParentVariable naming a stand-in entry, and a le that admits under
// that environment runs inside it without measuring the checkout.
// PREVENTS: the way `le test firewall` died in the guest on 2026-10-27. The
// guest command named no parent, so the guest le admitted itself against the
// host's registry over 9p, and its whole-tree `git diff HEAD` ran past job's
// git limit: "fingerprint test-firewall inputs: read tracked changes: signal:
// killed". The method runs the real setup step in sh, then admits from a
// directory that is not a git checkout: an admission that measured inputs
// would fail there, so only a run inside the parent answers.
func TestRunGuestCommandRunsInsideTheHostSlot(t *testing.T) {
	run := fixtureRun(t, ArchAMD64)
	plan, err := run.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	step := guestRunParentSetup(guestRunJobParent)
	if !strings.HasSuffix(plan.SetupCommand, " && "+step) {
		t.Fatalf("setup command does not end by naming the host slot (%q):\n%s", step, plan.SetupCommand)
	}

	entry := filepath.Join(t.TempDir(), "host-job")
	shell := guestRunParentSetup(entry) + ` && printf '%s' "$` + job.ParentVariable + `"`
	out, err := exec.CommandContext(t.Context(), "sh", "-c", shell).Output()
	if err != nil {
		t.Fatalf("run the setup step: %v", err)
	}
	if string(out) != entry {
		t.Fatalf("the setup step exports %s=%q, want %q", job.ParentVariable, out, entry)
	}
	body, err := os.ReadFile(entry) //nolint:gosec // the test's own temporary entry
	if err != nil {
		t.Fatalf("the setup step wrote no entry: %v", err)
	}
	if string(body) != guestJobParentEntry {
		t.Fatalf("the setup step wrote %q, want %q", body, guestJobParentEntry)
	}

	nameParent(t, entry)
	admission, err := job.NewIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := admission.Admit("test-firewall", []string{"le", "test", "firewall"})
	if err != nil {
		t.Fatalf("admission under the guest's parent failed: %v", err)
	}
	if ticket.Kind != job.KindInside {
		t.Fatalf("admission under the guest's parent answered %s, want %s", ticket.Kind, job.KindInside)
	}
}

// VALIDATES: a `test qemu run` admitted inside a parent's slot runs in this
// process and answers the run's own code.
// PREVENTS: a host run that holds no slot while its guest command runs inside
// a stand-in that claims the host holds one.
func TestAdmitQEMURunRunsInsideAParent(t *testing.T) {
	entry := filepath.Join(t.TempDir(), "parent.job")
	if err := os.WriteFile(entry, []byte("LABEL=outer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nameParent(t, entry)
	ran := false
	_, code := admitQEMURun(t.TempDir(), []string{"le", "test", "qemu", "run", "command", "true"},
		func() (any, int) { ran = true; return nil, 3 })
	if !ran {
		t.Fatal("the run did not run inside its parent's slot")
	}
	if code != 3 {
		t.Fatalf("admitQEMURun answered %d, want the run's own 3", code)
	}
}

// nameParent names entry as this test's job parent. env caches what it read,
// so the cache is dropped on both sides: a value an earlier test named would
// otherwise answer for this one.
func nameParent(t *testing.T, entry string) {
	t.Helper()
	t.Setenv(job.ParentVariable, entry)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// VALIDATES: runArgv spells a parsed line back as the words that typed it, in
// the order the action declares its parameters, switches as bare keywords.
// PREVENTS: a claimed slot re-executing a different run than the one typed.
func TestRunArgvSpellsTheTypedRun(t *testing.T) {
	args := leaction.Arguments{
		keywordKernel:  {"tmp/kernel/build/vmlinuz"},
		keywordCommand: {"./le test firewall -p 1"},
		"keep-alive":   {},
		keywordTimeout: {"25m"},
	}
	want := []string{
		"le", "test", "qemu", "run",
		keywordCommand, "./le test firewall -p 1",
		keywordTimeout, "25m",
		keywordKernel, "tmp/kernel/build/vmlinuz",
		"keep-alive",
	}
	if got := runArgv(args); !slices.Equal(got, want) {
		t.Fatalf("runArgv answered %q, want %q", got, want)
	}
}
