// Related: job.go -- the admission these tests drive from its entry point
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 for internal/le/job/answer.go. Admission
// concerns CONTENTION. Equivalence therefore depends on what happens when two
// processes request the machine at the same time, not on one process's output.
// Every case runs REAL processes against one registry. This test binary
// re-execs itself, so each job has its own process and pid and races with its
// sibling.
// PREVENTS: a port that admits every session at once. A case verifies each of
// the five load-bearing properties. Each case failed when its property was
// removed.
//
// These tests do not wait for the clock. The shortest stall window is one
// minute. A case that needs a stalled holder changes the holder's log time with
// os.Chtimes instead of waiting for the window. Thus, the file runs in seconds
// and evaluates the same code as a real run.

package job

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// helperMarker separates this test binary's own arguments from the ones that
// tell it to stand in for another session.
const helperMarker = "--"

// helperPoll is the interval at which a helper polls admission. This interval
// is the only change that a helper makes to the production path. The default is
// two seconds, which would delay a case that waits for a slot.
const helperPoll = 100 * time.Millisecond

// TestHelperAdmit is not a test case. It makes this binary act as another
// session. When invoked with a marker argument, it admits and runs one job and
// exits with the job's code.
//
// Every case that needs two concurrent jobs starts one of these helpers. A
// goroutine is insufficient because the entry is named for the PROCESS that
// holds it, and other processes must read the registry.
func TestHelperAdmit(t *testing.T) {
	args := helperArgs()
	if len(args) < 3 {
		return
	}

	root, label, argv := args[0], args[1], args[2:]
	adm, err := NewIn(root)
	if err != nil {
		t.Fatalf("helper admission: %v", err)
	}
	adm.Poll = helperPoll
	adm.Banner = helperPoll
	adm.Color = false

	_, code := adm.Run(label, argv, root, nil)
	os.Exit(code)
}

// helperArgs answers what this binary was told to stand in for, or nothing
// when it is running as an ordinary test.
func helperArgs() []string {
	for i, arg := range os.Args {
		if arg == helperMarker {
			return os.Args[i+1:]
		}
	}
	return nil
}

// TestOneJobRunsAndTheOtherWaits verifies serialization. When two sessions
// request the machine, one gets it. The other does not start until the first
// finishes.
//
// The two jobs run DIFFERENT commands, so sharing does not explain this
// behavior. The second job queued.
func TestOneJobRunsAndTheOtherWaits(t *testing.T) {
	root := fixtureRepo(t)
	order := filepath.Join(root, "tmp", "order")

	first := startHelper(t, root, "slow", nil, "sh", "-c", record(order, "first-in")+"; sleep 1; "+record(order, "first-out"))
	waitForEntry(t, root, "slow")

	second, code := runHelper(t, root, "slow", nil, "sh", "-c", record(order, "second-in"))
	if code != 0 {
		t.Fatalf("the second job answered %d: %s", code, second)
	}
	if err := first.Wait(); err != nil {
		t.Fatalf("the first job: %v", err)
	}

	want := "first-in\nfirst-out\nsecond-in\n"
	if got := read(t, order); got != want {
		t.Errorf("the jobs ran in this order:\n%s\nwant:\n%s\nthe second did not wait for the first", got, want)
	}
}

// TestIdenticalWorkAttachesRatherThanQueueing verifies the sharing rule. A
// second session with the same label, tree and work key does not queue a
// duplicate. It follows the running job and uses that job's verdict.
//
// The test verifies three properties together because no single property
// proves attachment. The command runs ONCE, the follower receives the run's
// output, and the follower returns the run's exit code.
func TestIdenticalWorkAttachesRatherThanQueueing(t *testing.T) {
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")
	argv := []string{"sh", "-c", record(ran, "once") + "; echo THE-SHARED-OUTPUT; sleep 1; exit 3"}

	first := startHelper(t, root, "shared", nil, argv...)
	waitForEntry(t, root, "shared")

	out, code := runHelper(t, root, "shared", nil, argv...)
	if err := first.Wait(); err == nil {
		t.Fatal("the first job answered 0, and this case needs its own 3 to be visible")
	}

	if code != 3 {
		t.Errorf("the follower answered %d, want the shared job's own 3", code)
	}
	if got := read(t, ran); got != "once\n" {
		t.Errorf("the command ran %q, want it to have run once: the follower ran its own copy", got)
	}
	if !strings.Contains(out, "THE-SHARED-OUTPUT") {
		t.Errorf("the follower's output is %q, want the shared run's output replayed into it", out)
	}
}

// TestDifferentWorkUnderOneLabelDoesNotShare verifies the sharing boundary and
// prevents the 2026-08-19 defect. A label names the TARGET, so two sessions
// that test different packages must each run their own job.
func TestDifferentWorkUnderOneLabelDoesNotShare(t *testing.T) {
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")

	first := startHelper(t, root, "pkg", nil, "sh", "-c", record(ran, "package-a")+"; sleep 1")
	waitForEntry(t, root, "pkg")

	if _, code := runHelper(t, root, "pkg", nil, "sh", "-c", record(ran, "package-b")); code != 0 {
		t.Fatalf("the second job answered %d", code)
	}
	if err := first.Wait(); err != nil {
		t.Fatalf("the first job: %v", err)
	}

	if got := read(t, ran); !strings.Contains(got, "package-a") || !strings.Contains(got, "package-b") {
		t.Errorf("the two jobs left %q, want both: one took the other's verdict for its own", got)
	}
}

// TestAnUnreadableEntryCountsAsOccupied is FAIL CLOSED. An entry this package
// cannot parse is a job it cannot prove is gone, and reading "cannot parse" as
// "nothing is running" would admit every session at once.
func TestAnUnreadableEntryCountsAsOccupied(t *testing.T) {
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")
	entry := writeEntry(t, root, "corrupt", "PID=not-a-number\nSTATE=running\n")

	if waitingHelper(t, root, "probe", nil, "sh", "-c", record(ran, "admitted")) {
		t.Error("a job was admitted past an entry nothing can parse")
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("the job ran: an unparseable entry was read as a free slot")
	}
	if _, err := os.Stat(entry); err != nil {
		t.Error("the unreadable entry was dropped while it was still inside the stall window")
	}
}

// TestAnUnreadableEntryIsDroppedOnceItIsOlderThanTheStallWindow is what keeps
// the registry bounded. Failing closed for ever would leave one corrupt file
// blocking the machine until somebody deleted it by hand.
func TestAnUnreadableEntryIsDroppedOnceItIsOlderThanTheStallWindow(t *testing.T) {
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")
	entry := writeEntry(t, root, "corrupt", "PID=not-a-number\nSTATE=running\n")
	age(t, entry, 2*StallDefault)

	if _, code := runHelper(t, root, "probe", nil, "sh", "-c", record(ran, "admitted")); code != 0 {
		t.Fatalf("the job answered %d, want it admitted past an entry older than the window", code)
	}
	if _, err := os.Stat(entry); err == nil {
		t.Error("the stale unreadable entry survived, so the registry is not bounded")
	}
}

// TestAStalledHolderIsBrokenOnItsSilence verifies the liveness rule. The holder
// remains alive, but its log stops growing. The system breaks its slot, and the
// evidence that justifies the kill names the file and the silence.
func TestAStalledHolderIsBrokenOnItsSilence(t *testing.T) {
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")

	holder := sleeper(t)
	entry, logPath := heldEntry(t, root, "wedged", holder.Process.Pid)
	age(t, logPath, 2*StallDefault)

	out, code := runHelper(t, root, "probe", nil, "sh", "-c", record(ran, "admitted"))
	if code != 0 {
		t.Fatalf("the job answered %d: %s", code, out)
	}

	if err := holder.Wait(); err == nil {
		t.Error("the stalled holder exited cleanly, so nothing killed it")
	}
	if _, err := os.Stat(entry); err == nil {
		t.Error("the broken holder's entry survived")
	}
	if !strings.Contains(out, "has not grown for") || !strings.Contains(out, filepath.Base(logPath)) {
		t.Errorf("the kill's evidence is %q, want the file that stopped growing and for how long", out)
	}
}

// TestALongRunningHolderThatKeepsWritingIsLeftAlone verifies that age alone does
// not break an active holder. During the contention that this package manages,
// a legitimate run can take longer while it continues to write. A decision
// based only on elapsed time would kill that run.
func TestALongRunningHolderThatKeepsWritingIsLeftAlone(t *testing.T) {
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")

	holder := sleeper(t)
	// Running for an hour, and writing now: the job is slow, not wedged.
	entry, logPath := heldEntrySince(t, root, "slow-but-alive", holder.Process.Pid, time.Hour)
	touch(t, logPath)

	if waitingHelper(t, root, "probe", nil, "sh", "-c", record(ran, "admitted")) {
		t.Error("a job was admitted past a holder that is still producing output")
	}
	if !alive(holder.Process.Pid) {
		t.Error("the holder is gone: an hour of elapsed time was read as a reason to kill it")
	}
	if _, err := os.Stat(entry); err != nil {
		t.Error("the holder's entry was dropped, so its slot was broken on age")
	}
	stop(t, holder)
}

// TestADeadHoldersEntryIsReaped is what makes a crashed job cost one poll
// interval rather than an operator.
func TestADeadHoldersEntryIsReaped(t *testing.T) {
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")

	gone := sleeper(t)
	pid := gone.Process.Pid
	stop(t, gone)

	entry, _ := heldEntry(t, root, "crashed", pid)
	if _, code := runHelper(t, root, "probe", nil, "sh", "-c", record(ran, "admitted")); code != 0 {
		t.Fatalf("the job answered %d, want it admitted past a holder that is gone", code)
	}
	if _, err := os.Stat(entry); err == nil {
		t.Error("the dead holder's entry survived, so its slot is held for ever")
	}
}

// TestANestedJobRunsInsideItsParentsSlot verifies the rule that prevents a
// deadlock. A wrapped job runs wrapped stages. If an inner job queued, it would
// wait for a slot that its own parent holds.
func TestANestedJobRunsInsideItsParentsSlot(t *testing.T) {
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")

	// The parent is this test process, and it holds the one slot.
	parent, _ := heldEntry(t, root, "parent", os.Getpid())

	var tb textbuf.Buffer
	inherited := []string{tb.Str("ZE_RUN_JOB=").Str(parent).String()}
	if _, code := runHelper(t, root, "stage", inherited, "sh", "-c", record(ran, "admitted")); code != 0 {
		t.Fatalf("the nested job answered %d, want it run inside its parent's slot", code)
	}
	if got := read(t, ran); got != "admitted\n" {
		t.Errorf("the nested job left %q, want it to have run: it queued behind its own parent", got)
	}
	if _, err := os.Stat(parent); err != nil {
		t.Error("the nested job removed its parent's entry")
	}
}

// TestANestedJobWhoseParentHasFinishedIsAdmittedNormally is the boundary of
// the rule above: the variable is not the claim, the parent's ENTRY is. A job
// whose parent is gone is a heavy job with nothing in front of it.
func TestANestedJobWhoseParentHasFinishedIsAdmittedNormally(t *testing.T) {
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")

	holder := sleeper(t)
	blocker, logPath := heldEntry(t, root, "holder", holder.Process.Pid)
	touch(t, logPath)

	var tb textbuf.Buffer
	stale := []string{tb.Str("ZE_RUN_JOB=").Str(filepath.Join(root, JobsDir, "parent.99999.job")).String()}
	if admitted := waitingHelper(t, root, "stage", stale, "sh", "-c", record(ran, "admitted")); admitted {
		t.Error("a job whose parent entry is gone ran unadmitted while a slot was held")
	}
	if _, err := os.Stat(blocker); err != nil {
		t.Error("the holder's entry went missing")
	}
	stop(t, holder)
}

// TestTheJobsOwnExitCodeReachesTheCaller verifies AC-8 at the wrapper. A gate
// that fails with 3 must return 3 because internal/le/commit/actions.go blocks
// on 3 but treats 1 as a warning.
func TestTheJobsOwnExitCodeReachesTheCaller(t *testing.T) {
	root := fixtureRepo(t)
	for _, want := range []int{0, 1, 3, 125} {
		var tb textbuf.Buffer
		script := tb.Str("exit ").Int(int64(want)).String()
		if _, code := runHelper(t, root, "coded", nil, "sh", "-c", script); code != want {
			t.Errorf("a job that exited %d answered %d", want, code)
		}
	}
}

// TestMakesNoExecuteModesTakeNoSlot verifies the one route that skips admission.
// A wrapped recipe passes `$(MAKE) ...`, which GNU make runs even under -n. If
// a job queued there, `make -n` would hang until the stall window expired.
func TestMakesNoExecuteModesTakeNoSlot(t *testing.T) {
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")

	holder := sleeper(t)
	_, logPath := heldEntry(t, root, "holder", holder.Process.Pid)
	touch(t, logPath)

	dryRunEnv := []string{"MAKEFLAGS=n --no-print-directory"}
	if _, code := runHelper(t, root, "recipe", dryRunEnv, "sh", "-c", record(ran, "printed")); code != 0 {
		t.Fatalf("the no-execute job answered %d", code)
	}
	if got := read(t, ran); got != "printed\n" {
		t.Errorf("the no-execute job left %q, want it run through: it waited for a slot", got)
	}
	stop(t, holder)
}

// TestALabelThatIsNotAPathComponentIsRefused stops a job escaping the registry
// directory, which is the whole reason the label is restricted.
func TestALabelThatIsNotAPathComponentIsRefused(t *testing.T) {
	adm := &Admission{Root: t.TempDir(), Slots: 1, Stall: StallDefault}
	for _, label := range []string{"", "../escape", "a/b", "dotted.name"} {
		if _, err := adm.Admit(label, []string{"true"}); err == nil {
			t.Errorf("the label %q was accepted", label)
		}
	}
}

// TestAPolicyOutsideItsRangeIsRefusedRatherThanClamped keeps the two numbers a
// caller can get wrong from being silently replaced by numbers the caller did
// not type.
func TestAPolicyOutsideItsRangeIsRefusedRatherThanClamped(t *testing.T) {
	cases := []struct {
		name  string
		slots int
		stall time.Duration
	}{
		{"no slots at all queues every job for ever", 0, StallDefault},
		{"more slots than cores is not admission", 1 << 20, StallDefault},
		{"a stall window under the floor kills a healthy job between two lines", 1, time.Second},
		{"a stall window over the ceiling leaves a wedged job holding the slot", 1, 10 * time.Hour},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adm := &Admission{Root: t.TempDir(), Slots: tc.slots, Stall: tc.stall}
			if err := adm.Validate(); err == nil {
				t.Error("the policy was accepted")
			}
		})
	}
}

// TestTheDocumentedKnobsReachThePolicy verifies the three environment names
// that native actions and ai/rules tell a session to use.
//
// A misspelled key fails SILENTLY. env.Get returns the empty string and the
// default remains in effect.
func TestTheDocumentedKnobsReachThePolicy(t *testing.T) {
	detach(t)

	t.Setenv("ZE_RUN_SLOTS", "2")
	t.Setenv("ZE_JOB_STALL_SECONDS", "900")
	t.Setenv("MAY_ATTACH", "0")
	env.ResetCache()

	adm, err := NewIn(t.TempDir())
	if err != nil {
		t.Fatalf("the policy was refused: %v", err)
	}
	if adm.Slots != 2 {
		t.Errorf("ZE_RUN_SLOTS=2 gave %d slots", adm.Slots)
	}
	if adm.Stall != 900*time.Second {
		t.Errorf("ZE_JOB_STALL_SECONDS=900 gave a %s window", adm.Stall)
	}
	if adm.MayAttach {
		t.Error("MAY_ATTACH=0 still shares another job's verdict")
	}
}

// TestTheOlderStallSpellingStillReachesThePolicy pins the fourth name.
// ai/rules/git-safety.md tells readers to raise ZE_VERIFY_MAX_LOCK_AGE, so
// that instruction has to keep working after the rename.
//
// The canonical name is REMOVED rather than emptied. t.Setenv cannot unset.
// An empty value is not an absent one here. env.Get finds the canonical key in
// the cache holding "" and stops before it looks at the alias.
func TestTheOlderStallSpellingStillReachesThePolicy(t *testing.T) {
	detach(t)
	unset(t, "ZE_JOB_STALL_SECONDS")

	t.Setenv("ZE_VERIFY_MAX_LOCK_AGE", "1200")
	env.ResetCache()

	adm, err := NewIn(t.TempDir())
	if err != nil {
		t.Fatalf("the policy was refused: %v", err)
	}
	if adm.Stall != 1200*time.Second {
		t.Errorf("ZE_VERIFY_MAX_LOCK_AGE=1200 gave a %s window: the older spelling stopped working", adm.Stall)
	}
}

// TestAnUnreadableStallWindowIsRefusedRatherThanDefaulted prevents silent use
// of the default when the knob has an invalid value. A session that specifies a
// window must receive notice that its value was not accepted.
func TestAnUnreadableStallWindowIsRefusedRatherThanDefaulted(t *testing.T) {
	detach(t)

	t.Setenv("ZE_JOB_STALL_SECONDS", "half an hour")
	env.ResetCache()

	if _, err := NewIn(t.TempDir()); err == nil {
		t.Error("a stall window nothing can parse was accepted")
	}
}

// unset removes one environment variable for the length of a case, and puts
// back whatever was there.
func unset(t *testing.T, name string) {
	t.Helper()
	before, had := os.LookupEnv(name)
	if !had {
		return
	}
	t.Cleanup(func() {
		if err := os.Setenv(name, before); err != nil {
			t.Logf("restore %s: %v", name, err)
		}
	})
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unset %s: %v", name, err)
	}
}

// TestAMalformedInvocationIsRefusedWithTheUsageCode pins the answer for a
// caller's own mistake. The code is 2, which matches the shell usage answer and
// distinguishes the mistake from a job failure.
func TestAMalformedInvocationIsRefusedWithTheUsageCode(t *testing.T) {
	cases := [][]string{
		{"walk"},
		{"run"},
		{"run", "label"},
		{"run", "label", "probe"},
		{"run", "label", "probe", "command"},
		{"run", "probe", "command", "true"},
	}

	for _, args := range cases {
		if payload, code := Answer(args); code != 2 || payload != nil {
			t.Errorf("`le job %v` answered (%v, %d), want no payload and 2", args, payload, code)
		}
	}
}

// TestReleaseRecordsTheVerdictWhereAFollowerFindsIt pins the write order of
// Release. The result exists when the entry is gone. Thus, an attacher that
// watches the entry disappear never loses the race.
func TestReleaseRecordsTheVerdictWhereAFollowerFindsIt(t *testing.T) {
	detach(t)
	root := fixtureRepo(t)
	adm := admission(t, root)

	ticket, err := adm.Admit("held", []string{"true"})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	if ticket.Kind != KindClaimed {
		t.Fatalf("admission answered %s, want a claimed slot", ticket.Kind)
	}
	ticket.Release(3)

	if _, err := os.Stat(adm.abs(ticket.Entry)); err == nil {
		t.Error("the entry survived the release, so the slot is still held")
	}
	code, ok := adm.readResult(ticket.Entry)
	if !ok || code != 3 {
		t.Errorf("the release recorded (%d, %v), want the job's own 3", code, ok)
	}
}

// detach makes a case's own admission independent of the job this test binary
// is running inside.
//
// It is needed because the nesting rule WORKS. `./le test-unit core` routes
// through internal/le/job/answer.go, which exports ZE_RUN_JOB. Thus, a test
// process that admits its own job finds a live parent entry and runs inside that slot.
// The helper processes are unaffected because their environment names nothing.
// A case that admits in-process has to say so.
//
// env.Get caches the environment once per process, so the cache is reset on
// both sides of the change.
func detach(t *testing.T) {
	t.Helper()
	t.Setenv("ZE_RUN_JOB", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// admission answers an admission over one fixture tree, polling fast enough
// for a test.
func admission(t *testing.T, root string) *Admission {
	t.Helper()
	return &Admission{
		Root: root, Slots: 1, Stall: StallDefault, Poll: helperPoll,
		Banner: helperPoll, ResultWait: 2 * time.Second, MayAttach: true,
		Out: os.Stdout, Err: os.Stderr,
	}
}

// startHelper starts one job in a process of its own and answers it, still
// running. The caller waits for it.
func startHelper(t *testing.T, root, label string, environ []string, argv ...string) *exec.Cmd {
	t.Helper()
	cmd := helperCommand(t, context.Background(), root, label, environ, argv)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start a helper: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return cmd
}

// runHelper runs one job in a process of its own and answers what it printed
// and what it exited with.
func runHelper(t *testing.T, root, label string, environ []string, argv ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := helperCommand(t, ctx, root, label, environ, argv)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("run a helper: %v: %s", err, out)
	}
	return string(out), exit.ExitCode()
}

// waitingHelper runs one job that is expected NOT to be admitted, and reports
// whether it was admitted anyway.
//
// A job that is refused a slot waits for as long as the slot is held, which is
// for ever in a case that never releases it, so the wait is bounded here and
// the bound IS the assertion: a job still waiting when the bound expires is a
// job that was not admitted.
func waitingHelper(t *testing.T, root, label string, environ []string, argv ...string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := helperCommand(t, ctx, root, label, environ, argv)
	return cmd.Run() == nil
}

// helperCommand builds the invocation that makes this test binary stand in for
// another session.
func helperCommand(t *testing.T, ctx context.Context, root, label string, environ, argv []string) *exec.Cmd {
	t.Helper()
	args := append([]string{"-test.run=^TestHelperAdmit$", helperMarker, root, label}, argv...)
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Dir = root
	// Use a deterministic environment because the work key reads MAKEFLAGS. The
	// session that runs this test can itself run inside make.
	cmd.Env = append([]string{"MAKEFLAGS=", "PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}, environ...)
	return cmd
}

// record answers a shell fragment that appends one word to a file, which is
// how a case sees what ran and in what order.
//
// The file lives under tmp/, which the fixture tree ignores. A marker outside
// tmp/ would move the tree hash between one job's admission and the next. Two
// jobs that disagree about the tree never share.
func record(path, word string) string {
	var tb textbuf.Buffer
	return tb.Str("echo ").Str(word).Str(" >> ").Str(path).String()
}

// waitForEntry blocks until a job of this label holds a slot. A case can then
// start a second job against a registry that is occupied.
func waitForEntry(t *testing.T, root, label string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var tb textbuf.Buffer
		names, err := filepath.Glob(filepath.Join(root, JobsDir, tb.Str(label).Str(".*.job").String()))
		if err == nil && len(names) > 0 {
			return
		}
		time.Sleep(helperPoll)
	}
	t.Fatalf("no %s entry appeared: the first job never took its slot", label)
}

// writeEntry puts one fabricated entry in the registry and answers its path.
func writeEntry(t *testing.T, root, label, body string) string {
	t.Helper()
	dir := filepath.Join(root, JobsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("make the registry: %v", err)
	}
	path := filepath.Join(dir, entryName(label, os.Getpid()+1, entrySuffix))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write an entry: %v", err)
	}
	return path
}

// heldEntry puts a complete, parseable entry in the registry for a process
// that is running, with a log beside it, and answers both paths.
func heldEntry(t *testing.T, root, label string, pid int) (string, string) {
	t.Helper()
	return heldEntrySince(t, root, label, pid, 0)
}

// heldEntrySince is heldEntry for a holder that started some time ago.
//
// A scan reads elapsed time from the STARTED field, not from the entry file's
// timestamps. Thus, a case about a long-running holder has to write it here. A
// case that aged the FILE instead would leave elapsed at zero. That case would
// pass against an implementation that breaks a slot on age.
func heldEntrySince(t *testing.T, root, label string, pid int, since time.Duration) (string, string) {
	t.Helper()
	dir := filepath.Join(root, JobsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("make the registry: %v", err)
	}

	logRel := filepath.Join(JobsDir, entryName(label, pid, logSuffix))
	var tb textbuf.Buffer
	tb.Str("LABEL=").Str(label).Byte('\n')
	tb.Str("PID=").Int(int64(pid)).Byte('\n')
	// PGID=0 is never signaled, so a broken holder is stopped by its pid
	// alone. A fabricated entry naming this session's real process group would
	// have the case kill the test runner.
	tb.Str("PGID=0\n")
	tb.Str("TREE=").Str(TreeHash(root)).Byte('\n')
	tb.Str("KEY=fabricated\n")
	tb.Str("STARTED=").Int(time.Now().Add(-since).Unix()).Byte('\n')
	tb.Str("LOG=").Str(logRel).Byte('\n')
	tb.Str("STATE=running\n")
	tb.Str("CMD=sleep\n")

	path := filepath.Join(dir, entryName(label, pid, entrySuffix))
	if err := os.WriteFile(path, []byte(tb.String()), 0o644); err != nil {
		t.Fatalf("write an entry: %v", err)
	}
	logPath := filepath.Join(root, logRel)
	if err := os.WriteFile(logPath, []byte("working\n"), 0o644); err != nil {
		t.Fatalf("write a log: %v", err)
	}
	return path, logPath
}

// sleeper starts a process that does nothing and answers it. It stands in for
// a holder: a case needs a pid that is genuinely alive.
func sleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start a stand-in holder: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return cmd
}

// stop ends a stand-in holder and reaps it.
func stop(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd.Process == nil {
		return
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Logf("kill a stand-in holder: %v", err)
	}
	_ = cmd.Wait()
}

// age sets a file's timestamps to an earlier time. This lets a case reach a
// one-minute stall window without a one-minute wait.
func age(t *testing.T, path string, by time.Duration) {
	t.Helper()
	when := time.Now().Add(-by)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("age %s: %v", path, err)
	}
}

// touch moves a file's timestamps to now, which is what a job still producing
// output does to its log.
func touch(t *testing.T, path string) {
	t.Helper()
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatalf("touch %s: %v", path, err)
	}
}

// read answers what a marker file holds, and nothing when there is none.
func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path) //nolint:gosec // a path this test wrote
	if err != nil {
		return ""
	}
	return string(body)
}

// A job that waits carries a tree hash taken before its wait, and the holder it
// finds wrote one taken at its claim. Several sessions work this checkout, so
// something changes during almost every wait. Comparing the two recorded hashes
// answers "different tree" for two jobs doing identical work, and sharing stops
// happening for exactly the jobs it exists to serve.
func TestAWaiterMeasuresItsTreeAgainBeforeItDeclinesToShare(t *testing.T) {
	root := fixtureRepo(t)
	admission, err := NewIn(root)
	if err != nil {
		t.Fatalf("admission: %v", err)
	}
	held := entry{state: "running", label: "shared", key: "same-work", tree: TreeHash(root)}
	waited := &pending{
		label: "shared", key: "same-work", mayAttach: true,
		tree: "the-hash-this-job-took-before-it-waited", treeStale: true,
	}

	if !admission.shares(waited, held) {
		t.Fatal("a waiter did not share a running job doing its own work on its own tree")
	}
	if waited.tree != held.tree {
		t.Fatalf("the waiter still carries %q, want the tree it measured again", waited.tree)
	}
}

// The refresh above must not become a way in. A job whose work differs shares
// nothing whatever the tree says, and measuring the tree for it would spend
// three git calls per poll on an answer that is already no.
func TestADifferentWorkKeyDeclinesWithoutMeasuringTheTree(t *testing.T) {
	root := fixtureRepo(t)
	admission, err := NewIn(root)
	if err != nil {
		t.Fatalf("admission: %v", err)
	}
	held := entry{state: "running", label: "shared", key: "other-work", tree: TreeHash(root)}
	waited := &pending{
		label: "shared", key: "same-work", mayAttach: true,
		tree: "the-hash-this-job-took-before-it-waited", treeStale: true,
	}

	if admission.shares(waited, held) {
		t.Fatal("a job shared a run of different work")
	}
	if !waited.treeStale {
		t.Fatal("the tree was measured for a job whose key had already answered no")
	}
}

// A job that did NOT wait holds a current hash. A holder on another tree is a
// holder judging something else, and that stays true after the change above.
func TestAJobThatDidNotWaitStillDeclinesAHolderOnAnotherTree(t *testing.T) {
	root := fixtureRepo(t)
	admission, err := NewIn(root)
	if err != nil {
		t.Fatalf("admission: %v", err)
	}
	held := entry{state: "running", label: "shared", key: "same-work", tree: "another-tree"}
	fresh := &pending{label: "shared", key: "same-work", mayAttach: true, tree: TreeHash(root)}

	if admission.shares(fresh, held) {
		t.Fatal("a job shared a run judging a different tree")
	}
}
