// The migration's proof for the admission wrapper: scripts/dev/ze-run.sh and
// internal/le/lejob are one mechanism over one registry, and either half admits
// the other's jobs.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 for scripts/dev/ze-run.sh. Admission
// concerns CONTENTION, so one process output cannot show equivalence. The test
// must observe two processes that want the machine at once. Thus, every case
// runs the two halves AGAINST EACH OTHER over one tmp/.ze-jobs. It tests shell
// holding with Go asking, then Go holding with shell asking. This proves more
// than a comparison of each half with its own transcript.
// PREVENTS: a port that interprets the registry differently from the script.
// Both halves run on this machine during migration. A port that fails open or
// uses a different work key can admit every session at once. A different tree
// hash has the same effect, while output comparisons still pass.
//
// This file is deliberately HERE instead of beside internal/le/lejob. It is a
// migration artifact, so the commit that deletes the script also deletes its
// proof. internal/le/lejob/contention_test.go survives the swap and tests the same
// five properties against only the Go half.
//
// The tests never wait for the clock. The shortest stall window is one minute.
// A case that needs a stalled holder uses os.Chtimes to AGE the holder log
// instead of sleeping through the window.

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/lejob"
)

// admitMarker separates this test binary's own arguments from the ones that
// tell it to stand in for a session running the GO half.
const admitMarker = "--"

// admitPoll is what the Go half polls at inside a helper. The shell half polls
// at two seconds and is not configurable, which is what sets the pace of every
// cross case here.
const admitPoll = 200 * time.Millisecond

// admitBound is how long a case waits for a job that must remain excluded. It
// exceeds the shell half's two-second poll, so an admissible job has time to
// start.
const admitBound = 6 * time.Second

// TestHelperAdmit is not a case. It is this binary standing in for a session
// that admits its jobs through the Go half.
func TestHelperAdmit(t *testing.T) {
	args := admitHelperArgs()
	if len(args) < 3 {
		return
	}

	root, label, argv := args[0], args[1], args[2:]
	adm, err := lejob.NewIn(root)
	if err != nil {
		t.Fatalf("admission: %v", err)
	}
	adm.Poll = admitPoll
	adm.Banner = admitPoll
	adm.Color = false

	_, code := adm.Run(label, argv, root, nil)
	os.Exit(code)
}

// admitHelperArgs answers what this binary was told to stand in for, or
// nothing when it is running as an ordinary test.
func admitHelperArgs() []string {
	for i, arg := range os.Args {
		if arg == admitMarker {
			return os.Args[i+1:]
		}
	}
	return nil
}

// half is one implementation of admission, driven as a process.
type half struct {
	name string
	// command builds the invocation that runs one job through this half.
	command func(t *testing.T, ctx context.Context, root, label string, argv []string) *exec.Cmd
}

// halves are the two implementations, and every case below runs the same
// scenario through each of them.
func halves() []half {
	return []half{
		{name: "shell", command: shellJob},
		{name: "go", command: goJob},
	}
}

// shellJob runs one job through scripts/dev/ze-run.sh.
func shellJob(t *testing.T, ctx context.Context, root, label string, argv []string) *exec.Cmd {
	t.Helper()
	script, err := filepath.Abs("ze-run.sh")
	if err != nil {
		t.Fatalf("locate ze-run.sh: %v", err)
	}
	return admitCommand(ctx, root, append([]string{"bash", script, label}, argv...))
}

// goJob runs one job through internal/le/lejob, in a process of its own.
func goJob(t *testing.T, ctx context.Context, root, label string, argv []string) *exec.Cmd {
	t.Helper()
	invocation := append([]string{os.Args[0], "-test.run=^TestHelperAdmit$", admitMarker, root, label}, argv...)
	return admitCommand(ctx, root, invocation)
}

// admitCommand builds one job's process with an environment that says nothing
// about the session running these tests.
//
// MAKEFLAGS is empty because the work key reads it. ZE_RUN_JOB is absent because
// this test binary already runs inside an admitted job. `make ze-unit-pkg-test`
// uses the script under test. A helper that inherited the variable would use
// THIS run's slot instead of requesting one from the fixture registry.
func admitCommand(ctx context.Context, root string, argv []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = root
	cmd.Env = []string{
		"MAKEFLAGS=",
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
	return cmd
}

// TestBothHalvesSerializeOneMachine tests the purpose of the wrapper. Two
// sessions want the machine. One runs, and the second starts only after the
// first finishes. The case uses each half as the holder and as the asker.
//
// The two jobs run DIFFERENT commands, so nothing here can be explained by
// sharing: the second really queued.
func TestBothHalvesSerializeOneMachine(t *testing.T) {
	for _, holder := range halves() {
		for _, asker := range halves() {
			t.Run(holder.name+"-holds-"+asker.name+"-asks", func(t *testing.T) {
				root := admitRepo(t)
				order := filepath.Join(root, "tmp", "order")

				first := admitStart(t, holder, root, "slow",
					admitRecord(order, "first-in")+"; sleep 1; "+admitRecord(order, "first-out"))
				admitWaitForEntry(t, root, "slow")

				out, code := admitRun(t, asker, root, "slow", admitRecord(order, "second-in"))
				if code != 0 {
					t.Fatalf("the %s asker answered %d: %s", asker.name, code, out)
				}
				if err := first.Wait(); err != nil {
					t.Fatalf("the %s holder: %v", holder.name, err)
				}

				want := "first-in\nfirst-out\nsecond-in\n"
				if got := admitRead(order); got != want {
					t.Errorf("the jobs ran in this order:\n%s\nwant:\n%s", got, want)
				}
			})
		}
	}
}

// TestBothHalvesShareOneRunningJob tests the attach rule across both halves. A
// second asker that requests the same work over the same tree follows the
// running job instead of queuing a duplicate.
//
// The test asserts three facts together because a new job can produce any one
// of them alone. The command ran ONCE, the follower saw its output, and the
// follower answered its exit code.
//
// This is also the strongest test of both fingerprints. A follower attaches
// only when its tree hash AND work key match the holder. Thus, a cross-half
// attach proves agreement on both.
func TestBothHalvesShareOneRunningJob(t *testing.T) {
	for _, holder := range halves() {
		for _, asker := range halves() {
			t.Run(holder.name+"-holds-"+asker.name+"-asks", func(t *testing.T) {
				root := admitRepo(t)
				ran := filepath.Join(root, "tmp", "ran")
				script := admitRecord(ran, "once") + "; echo THE-SHARED-OUTPUT; sleep 2; exit 3"

				first := admitStart(t, holder, root, "shared", script)
				admitWaitForEntry(t, root, "shared")

				out, code := admitRun(t, asker, root, "shared", script)
				if err := first.Wait(); err == nil {
					t.Fatal("the holder answered 0, and this case needs its own 3 to be visible")
				}

				if code != 3 {
					t.Errorf("the %s follower answered %d, want the shared job's own 3", asker.name, code)
				}
				if got := admitRead(ran); got != "once\n" {
					t.Errorf("the command ran %q, want once: the follower ran its own copy", got)
				}
				if !strings.Contains(out, "THE-SHARED-OUTPUT") {
					t.Errorf("the follower's output is %q, want the shared run replayed into it", out)
				}
			})
		}
	}
}

// TestBothHalvesFailClosedOnAnUnreadableEntry pins the direction both halves
// fail in. An entry neither can parse is a job neither can prove is gone, and
// reading "cannot parse" as "nothing is running" would admit every session at
// once.
func TestBothHalvesFailClosedOnAnUnreadableEntry(t *testing.T) {
	for _, asker := range halves() {
		t.Run(asker.name, func(t *testing.T) {
			root := admitRepo(t)
			ran := filepath.Join(root, "tmp", "ran")
			entry := admitWriteEntry(t, root, "corrupt", os.Getpid()+1, "PID=not-a-number\nSTATE=running\n")

			if admitRuns(t, asker, root, "probe", admitRecord(ran, "admitted")) {
				t.Errorf("the %s half was admitted past an entry nothing can parse", asker.name)
			}
			if _, err := os.Stat(entry); err != nil {
				t.Errorf("the %s half dropped an unreadable entry inside the stall window", asker.name)
			}
		})
	}
}

// TestBothHalvesBreakAStalledHolderOnItsSilence tests the liveness rule. A live
// holder that stops writing loses its slot. The evidence for the kill names the
// file that stopped growing.
func TestBothHalvesBreakAStalledHolderOnItsSilence(t *testing.T) {
	for _, asker := range halves() {
		t.Run(asker.name, func(t *testing.T) {
			root := admitRepo(t)
			ran := filepath.Join(root, "tmp", "ran")

			holder := admitSleeper(t)
			entry, logPath := admitHeldEntry(t, root, "wedged", holder.Process.Pid)
			admitAge(t, logPath, 2*lejob.StallDefault)

			out, code := admitRun(t, asker, root, "probe", admitRecord(ran, "admitted"))
			if code != 0 {
				t.Fatalf("the %s half answered %d: %s", asker.name, code, out)
			}
			if err := holder.Wait(); err == nil {
				t.Errorf("the %s half left the stalled holder running", asker.name)
			}
			if _, err := os.Stat(entry); err == nil {
				t.Errorf("the %s half left the broken holder's entry behind", asker.name)
			}
			if !strings.Contains(out, "has not grown for") {
				t.Errorf("the %s half's kill printed %q, want the file that stopped growing", asker.name, out)
			}
		})
	}
}

// TestNeitherHalfBreaksAHolderThatIsStillWriting tests the error in age-based
// breaking. This case matters most under the contention that the wrapper
// manages. A legitimate run takes longer in that condition, and a clock-based
// decision kills it.
func TestNeitherHalfBreaksAHolderThatIsStillWriting(t *testing.T) {
	for _, asker := range halves() {
		t.Run(asker.name, func(t *testing.T) {
			root := admitRepo(t)
			ran := filepath.Join(root, "tmp", "ran")

			holder := admitSleeper(t)
			// Running for an hour, and writing now: slow, not wedged.
			entry, logPath := admitHeldEntrySince(t, root, "slow-but-alive", holder.Process.Pid, time.Hour)
			admitTouch(t, logPath)

			if admitRuns(t, asker, root, "probe", admitRecord(ran, "admitted")) {
				t.Errorf("the %s half ran past a holder that is still producing output", asker.name)
			}
			if _, err := os.Stat(entry); err != nil {
				t.Errorf("the %s half broke a slot on elapsed time", asker.name)
			}
			admitStop(t, holder)
		})
	}
}

// TestEitherHalfRunsInsideTheOthersSlot tests nesting during the migration. A
// verify admitted by one half runs stages admitted by the other. An inner job
// that queues would wait for the slot held by its parent.
func TestEitherHalfRunsInsideTheOthersSlot(t *testing.T) {
	for _, asker := range halves() {
		t.Run(asker.name, func(t *testing.T) {
			root := admitRepo(t)
			ran := filepath.Join(root, "tmp", "ran")

			// The parent is this test process, and it holds the one slot.
			parent, _ := admitHeldEntry(t, root, "parent", os.Getpid())

			ctx, cancel := context.WithTimeout(context.Background(), admitBound)
			defer cancel()
			cmd := asker.command(t, ctx, root, "stage", []string{"sh", "-c", admitRecord(ran, "admitted")})
			cmd.Env = append(cmd.Env, "ZE_RUN_JOB="+parent)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("the nested %s job: %v: %s", asker.name, err, out)
			}

			if got := admitRead(ran); got != "admitted\n" {
				t.Errorf("the nested %s job left %q, want it run inside its parent's slot", asker.name, got)
			}
			if _, err := os.Stat(parent); err != nil {
				t.Errorf("the nested %s job removed its parent's entry", asker.name)
			}
		})
	}
}

// admitStart starts one job through one half and answers it, still running.
func admitStart(t *testing.T, which half, root, label, script string) *exec.Cmd {
	t.Helper()
	cmd := which.command(t, context.Background(), root, label, []string{"sh", "-c", script})
	if err := cmd.Start(); err != nil {
		t.Fatalf("start the %s half: %v", which.name, err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return cmd
}

// admitRun runs one job through one half and answers what it printed and what
// it exited with.
func admitRun(t *testing.T, which half, root, label, script string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cmd := which.command(t, ctx, root, label, []string{"sh", "-c", script})
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("run the %s half: %v: %s", which.name, err, out)
	}
	return string(out), exit.ExitCode()
}

// admitRuns reports whether the system admitted a job that the test expected to
// remain excluded.
//
// A job refused a slot waits until the holder releases it. A case that never
// releases the slot would wait forever, so the test sets a bound. A job still
// waiting at that bound was not admitted.
func admitRuns(t *testing.T, which half, root, label, script string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), admitBound)
	defer cancel()

	cmd := which.command(t, ctx, root, label, []string{"sh", "-c", script})
	return cmd.Run() == nil
}

// admitRepo builds one case checkout with an ignored tmp/ directory. The ignored
// directory keeps the registry out of the tree hash from both halves.
func admitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	admitGit(t, dir, "init", "--quiet")
	admitGit(t, dir, "config", "user.email", "test@example.com")
	admitGit(t, dir, "config", "user.name", "test")
	admitWrite(t, filepath.Join(dir, ".gitignore"), "tmp/\n")
	admitWrite(t, filepath.Join(dir, "tracked.txt"), "one\n")
	admitGit(t, dir, "add", ".")
	admitGit(t, dir, "commit", "--quiet", "-m", "fixture")

	if err := os.MkdirAll(filepath.Join(dir, lejob.JobsDir), 0o755); err != nil {
		t.Fatalf("make the registry: %v", err)
	}
	return dir
}

// admitGit runs one git command in a fixture tree.
func admitGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// admitWriteEntry puts one fabricated entry in the registry and answers its
// path.
func admitWriteEntry(t *testing.T, root, label string, pid int, body string) string {
	t.Helper()
	path := filepath.Join(root, lejob.JobsDir, label+"."+admitDigits(pid)+".job")
	admitWrite(t, path, body)
	return path
}

// admitHeldEntry puts a complete, parseable entry in the registry for a
// process that is running, with a log beside it.
//
// PGID=0 is never signaled by either half.
// A broken holder is stopped by its pid alone.
// An entry that names this session's real process group would make the case kill the test runner.
func admitHeldEntry(t *testing.T, root, label string, pid int) (string, string) {
	t.Helper()
	return admitHeldEntrySince(t, root, label, pid, 0)
}

// admitHeldEntrySince is admitHeldEntry for a holder that started some time
// ago.
//
// Both halves read the elapsed time from the STARTED field instead of the entry file's timestamps.
// Thus, a case about a long-running holder writes the STARTED field here.
// A case that aged only the FILE would leave elapsed at zero.
// Such a case would pass against an implementation that breaks a slot on age.
func admitHeldEntrySince(t *testing.T, root, label string, pid int, since time.Duration) (string, string) {
	t.Helper()
	logRel := filepath.Join(lejob.JobsDir, label+"."+admitDigits(pid)+".log")
	body := strings.Join([]string{
		"LABEL=" + label,
		"PID=" + admitDigits(pid),
		"PGID=0",
		"TREE=" + lejob.TreeHash(root),
		"KEY=fabricated",
		"PARAMS=",
		"STARTED=" + admitDigits(int(time.Now().Add(-since).Unix())),
		"LOG=" + logRel,
		"STATE=running",
		"CMD=sleep",
		"",
	}, "\n")

	entry := filepath.Join(root, lejob.JobsDir, label+"."+admitDigits(pid)+".job")
	admitWrite(t, entry, body)
	logPath := filepath.Join(root, logRel)
	admitWrite(t, logPath, "working\n")
	return entry, logPath
}

// admitWaitForEntry blocks until a job of this label holds a slot, so a case
// can ask against a registry that is genuinely occupied.
func admitWaitForEntry(t *testing.T, root, label string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		names, err := filepath.Glob(filepath.Join(root, lejob.JobsDir, label+".*.job"))
		if err == nil && len(names) > 0 {
			return
		}
		time.Sleep(admitPoll)
	}
	t.Fatalf("no %s entry appeared: the holder never took its slot", label)
}

// admitSleeper starts a process that does nothing, to stand in for a holder: a
// case needs a pid that is genuinely alive.
func admitSleeper(t *testing.T) *exec.Cmd {
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

// admitStop ends a stand-in holder and reaps it.
func admitStop(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd.Process == nil {
		return
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Logf("kill a stand-in holder: %v", err)
	}
	_ = cmd.Wait()
}

// admitRecord answers a shell fragment appending one word to a file, which is
// how a case sees what ran and in what order.
//
// The file lives under tmp/, which the fixture tree ignores.
// A marker outside tmp/ would change the tree hash between two job admissions.
// Two jobs with different tree hashes never share.
func admitRecord(path, word string) string {
	return "echo " + word + " >> " + path
}

// admitAge moves a file timestamp into the past. This lets a case reach a
// one-minute stall window immediately.
func admitAge(t *testing.T, path string, by time.Duration) {
	t.Helper()
	when := time.Now().Add(-by)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("age %s: %v", path, err)
	}
}

// admitTouch moves a file's timestamps to now, which is what a job still
// producing output does to its log.
func admitTouch(t *testing.T, path string) {
	t.Helper()
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatalf("touch %s: %v", path, err)
	}
}

// admitWrite puts one file in a fixture tree.
func admitWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("make %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// admitRead answers what a marker file holds, and nothing when there is none.
func admitRead(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(body)
}

// admitDigits spells a non-negative number for a file name.
func admitDigits(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
