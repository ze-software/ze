// VALIDATES: verifyworktree runs exact registered identities as their own le process inside the requested root.
// PREVENTS: a missing owner, missing handler, root leak, or concurrent stage certifying a stage,
// and a failing stage whose log lacks the reason its handler wrote to the process's stderr.
package verifydispatch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"

	// verifyengine links areas under repo and under the namespaces below, and
	// Dispatch refuses a first word with no summary, so the repo and site
	// areas and every namespace package are linked, as the composition root links
	// them.
	_ "github.com/ze-software/ze/internal/le/ai"
	_ "github.com/ze-software/ze/internal/le/arch"
	_ "github.com/ze-software/ze/internal/le/build"
	_ "github.com/ze-software/ze/internal/le/chaos"
	_ "github.com/ze-software/ze/internal/le/cli"
	_ "github.com/ze-software/ze/internal/le/config"
	_ "github.com/ze-software/ze/internal/le/data"
	_ "github.com/ze-software/ze/internal/le/doc"
	_ "github.com/ze-software/ze/internal/le/go"
	_ "github.com/ze-software/ze/internal/le/lab"
	_ "github.com/ze-software/ze/internal/le/plugin"
	_ "github.com/ze-software/ze/internal/le/repo"
	_ "github.com/ze-software/ze/internal/le/setup/docker"
	_ "github.com/ze-software/ze/internal/le/site"
	_ "github.com/ze-software/ze/internal/le/spec"
	_ "github.com/ze-software/ze/internal/le/test"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
	_ "github.com/ze-software/ze/internal/le/web"
	_ "github.com/ze-software/ze/internal/le/yang"
)

const probeRoot = "verify-dispatch-probe"

const (
	probeDiagnosis = "FAIL tier: internal/core imports internal/component/bgp\n"
	probeDeadline  = "staticcheck: part 1 of 6 passed its 4m0s deadline before judging\n"
)

type probeText string

func (p probeText) Text() string { return string(p) }

var probeOnce sync.Once

// registerProbe registers the probe in the test process, where dispatch asks
// whether le owns it, and in the child process, where it runs.
func registerProbe() {
	probeOnce.Do(func() {
		leroot.Register(probeRoot, leroot.GroupReport, probeAnswer, registry.Meta{
			ShortHelp: "verify dispatch capture probe",
			Mode:      "offline",
			Section:   registry.SectionTest,
		})
		// Every le area publishes a table, or Dispatch refuses to start
		// (D-P3-tablecheck). The probe reads its own words, as a
		// pass-through area's program does.
		leroot.RegisterForwarding(probeRoot, "the dispatch probe")
	})
}

func probeAnswer(args []string) (any, int) {
	if len(args) > 0 {
		if args[0] == "stderr" {
			return make(chan int), 99
		}
		// The shape of `le arch tier check` (internal/le/arch/tier, runCheck):
		// a report on stdout, its diagnosis written straight to the process's
		// stderr, and the report's failure code.
		if args[0] == "diagnosis" {
			os.Stderr.WriteString(probeDiagnosis) //nolint:errcheck // the shape under test
			return probeText("OK tier: every import points down\n"), 2
		}
		// The shape of `le go staticcheck check part 1 of 6`
		// (internal/le/go/staticcheck, the Judge deadline): the reason on the
		// process's stderr and no payload at all.
		if args[0] == "deadline" {
			fmt.Fprint(os.Stderr, probeDeadline) //nolint:errcheck // the shape under test
			return nil, 2
		}
		if args[0] == "die" {
			syscall.Kill(os.Getpid(), syscall.SIGKILL) //nolint:errcheck // the next line never runs when it works
			return nil, 3
		}
		// A stage that leaves a process running on the output it inherited,
		// as a harness that starts a daemon and never stops it does.
		if args[0] == "orphan" || args[0] == "orphan-red" {
			leftover := exec.CommandContext(context.Background(), "sleep", "2")
			leftover.Stdout = os.Stdout
			if err := leftover.Start(); err != nil {
				return probeText(err.Error()), 5
			}
			if args[0] == "orphan-red" {
				return probeText("left a process running\n"), 1
			}
			return probeText("left a process running\n"), 0
		}
		// A stage that holds resources and releases them on SIGTERM, as a
		// harness holding a job slot, a port lock or a namespace does.
		if args[0] == "term" {
			terms := make(chan os.Signal, 1)
			signal.Notify(terms, syscall.SIGTERM)
			select {
			case <-terms:
				os.Stdout.WriteString("released on SIGTERM\n") //nolint:errcheck // the shape under test
				return nil, 7
			case <-time.After(time.Minute):
				return nil, 4
			}
		}
		if args[0] == "panic" {
			panic("BUG: probe crash")
		}
		// A stage that relays another program's crash, as a test runner or a
		// daemon log does, and answers that it could not judge.
		if args[0] == "relay" {
			os.Stderr.WriteString("panic: BUG: relayed\n\ngoroutine 1 [running]:\nmain.main()\n") //nolint:errcheck // the shape under test
			return nil, 2
		}
		if args[0] == "hang" {
			time.Sleep(time.Minute)
			return nil, 4
		}
		// The root is read late, so two stages that overlap would see each
		// other's root if they shared one.
		if args[0] == "late-root" {
			time.Sleep(200 * time.Millisecond)
		}
	}
	root, err := lepath.Root()
	if err != nil {
		return probeText(err.Error()), 2
	}
	return probeText("captured stdout\nargs: " + strings.Join(args, " ") + "\nroot: " + root + "\n"), 37
}

// TestMain makes this test binary the le process a stage runs in, when a test
// in this file starts it with the le root word first, as stageProcess starts a
// binary that is not named le. Otherwise it runs the tests.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "le" {
		registerProbe()
		leroot.RunBeforeDispatch("ze le")
		os.Exit(leroot.Dispatch("ze le", os.Args[2:]))
	}
	os.Exit(m.Run())
}

// testSpawn starts this test binary as the stage process, in place of the le
// binary RunAction starts, so the tests run on every platform.
func testSpawn(ctx context.Context, words []string) (*exec.Cmd, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, executable, append([]string{"le"}, words...)...), nil
}

func runProbe(ctx context.Context, t *testing.T, root string, identity verifyengine.Identity) verifyengine.ActionResult {
	t.Helper()
	registerProbe()
	return dispatch(ctx, root, identity, leroot.Owns, leroot.LookupCommand, testSpawn)
}

func probeOutput(t *testing.T, root string, args []string) string {
	t.Helper()
	absolute, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	return "captured stdout\nargs: " + strings.Join(args, " ") + "\nroot: " + absolute + "\n"
}

func TestDispatchCapturesStdoutAndPreservesIdentityCodeArgsAndRoot(t *testing.T) {
	root := t.TempDir()
	identity := verifyengine.Identity{Name: "ze-probe", Command: probeRoot, Args: []string{"alpha", "beta"}}
	result := runProbe(context.Background(), t, root, identity)
	if !result.Registered || !result.Completed || result.Code != 37 || result.Failure != nil {
		t.Fatalf("dispatch result = %#v", result)
	}
	if result.Identity.Name != identity.Name || result.Identity.Command != identity.Command {
		t.Errorf("identity = %#v, want %#v", result.Identity, identity)
	}
	if !slices.Equal(result.Identity.Args, identity.Args) {
		t.Errorf("identity args = %q, want %q", result.Identity.Args, identity.Args)
	}
	// The handler's args and root reach the log through its own answer.
	if want := probeOutput(t, root, identity.Args); result.Output != want {
		t.Errorf("stdout = %q, want %q", result.Output, want)
	}
}

func TestDispatchCapturesStderrFromTheRegisteredHandler(t *testing.T) {
	identity := verifyengine.Identity{Name: "ze-probe-error", Command: probeRoot, Args: []string{"stderr"}}
	result := runProbe(context.Background(), t, t.TempDir(), identity)
	if !result.Registered || !result.Completed || result.Code != 1 {
		t.Fatalf("dispatch result = %#v", result)
	}
	if !strings.Contains(result.Output, "answered a payload that does not encode") {
		t.Errorf("stderr was not captured: %q", result.Output)
	}
}

func TestDispatchRefusesUnownedMissingAndAbsentRoots(t *testing.T) {
	root := t.TempDir()
	identity := verifyengine.Identity{Name: "ze-refusal", Command: "refusal-probe", Args: []string{"exact"}}
	missing := func(string) command.LocalDataHandler { return nil }

	unowned := dispatch(context.Background(), root, identity,
		func(string) bool { return false }, missing, testSpawn)
	if unowned.Registered || unowned.Completed || unowned.Code != 2 {
		t.Fatalf("unowned result = %#v", unowned)
	}
	if unowned.Failure == nil || unowned.Failure.Kind != "unowned-root" {
		t.Errorf("unowned failure = %#v", unowned.Failure)
	}

	missingHandler := dispatch(context.Background(), root, identity,
		func(string) bool { return true }, missing, testSpawn)
	if missingHandler.Registered || missingHandler.Completed || missingHandler.Code != 2 {
		t.Fatalf("missing-handler result = %#v", missingHandler)
	}
	if missingHandler.Failure == nil || missingHandler.Failure.Kind != "missing-handler" {
		t.Errorf("missing-handler failure = %#v", missingHandler.Failure)
	}

	absent := filepath.Join(root, "absent")
	missingRoot := dispatch(context.Background(), absent, identity,
		func(string) bool { return true }, missing, testSpawn)
	if missingRoot.Failure == nil || missingRoot.Failure.Kind != "root-missing" {
		t.Errorf("missing-root failure = %#v", missingRoot.Failure)
	}
}

// TestDispatchGivesOverlappingStagesTheirOwnRoot proves two stages that run at
// the same time each judge the root they were given. Method: both read their
// root after a pause long enough for the other to have started.
func TestDispatchGivesOverlappingStagesTheirOwnRoot(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir()}
	results := make([]verifyengine.ActionResult, len(roots))
	var calls sync.WaitGroup
	for i := range roots {
		calls.Go(func() {
			identity := verifyengine.Identity{Name: "overlap", Command: probeRoot, Args: []string{"late-root"}}
			results[i] = runProbe(context.Background(), t, roots[i], identity)
		})
	}
	calls.Wait()
	for i := range roots {
		if want := probeOutput(t, roots[i], []string{"late-root"}); results[i].Output != want {
			t.Errorf("stage %d log = %q, want %q", i, results[i].Output, want)
		}
	}
}

// TestDispatchReportsAStageThatDidNotExit proves a stage killed by a signal,
// a stage canceled, and a stage whose process cannot be built are each
// refused rather than read as an exit status.
func TestDispatchReportsAStageThatDidNotExit(t *testing.T) {
	killed := runProbe(context.Background(), t, t.TempDir(),
		verifyengine.Identity{Name: "die", Command: probeRoot, Args: []string{"die"}})
	if killed.Completed || killed.Code != 2 || killed.Failure == nil || killed.Failure.Kind != "killed" {
		t.Errorf("killed result = %#v", killed)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	hung := runProbe(ctx, t, t.TempDir(),
		verifyengine.Identity{Name: "hang", Command: probeRoot, Args: []string{"hang"}})
	if hung.Completed || hung.Code != verifyengine.Interrupted || hung.Failure == nil || hung.Failure.Kind != "interrupted" {
		t.Errorf("canceled result = %#v", hung)
	}

	registerProbe()
	unbuilt := dispatch(context.Background(), t.TempDir(),
		verifyengine.Identity{Name: "unbuilt", Command: probeRoot},
		leroot.Owns, leroot.LookupCommand,
		func(context.Context, []string) (*exec.Cmd, error) { return nil, errors.New("no executable") })
	if unbuilt.Completed || unbuilt.Code != 2 || unbuilt.Failure == nil || unbuilt.Failure.Kind != "spawn" {
		t.Errorf("unbuilt result = %#v", unbuilt)
	}

	unstarted := dispatch(context.Background(), t.TempDir(),
		verifyengine.Identity{Name: "unstarted", Command: probeRoot},
		leroot.Owns, leroot.LookupCommand,
		func(ctx context.Context, _ []string) (*exec.Cmd, error) {
			return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "absent-le")), nil
		})
	if unstarted.Completed || unstarted.Code != 2 || unstarted.Failure == nil || unstarted.Failure.Kind != "spawn" {
		t.Errorf("unstarted result = %#v", unstarted)
	}
}

// TestDispatchSaysWhenALeftoverProcessHeldTheLog proves the log stops waiting
// for a process the stage left on its output, and says it stopped, whether the
// stage passed or failed. Method: the probe starts a sleep on its stdout and
// exits; the drain wait is shortened so the test does not wait ten seconds.
func TestDispatchSaysWhenALeftoverProcessHeldTheLog(t *testing.T) {
	drain := outputDrainMax
	outputDrainMax = 200 * time.Millisecond
	t.Cleanup(func() { outputDrainMax = drain })

	for verb, code := range map[string]int{"orphan": 0, "orphan-red": 1} {
		result := runProbe(context.Background(), t, t.TempDir(),
			verifyengine.Identity{Name: verb, Command: probeRoot, Args: []string{verb}})
		if !result.Completed || result.Code != code || result.Failure != nil {
			t.Fatalf("%s result = %#v", verb, result)
		}
		if !strings.Contains(result.Output, "left a process running\n") {
			t.Errorf("%s stage output lost: %q", verb, result.Output)
		}
		if !strings.Contains(result.Output, "still held its output open") {
			t.Errorf("%s log does not say it stopped waiting: %q", verb, result.Output)
		}
	}
}

// TestDispatchStopsACanceledStageWithSIGTERM proves a canceled stage is asked
// to stop, so it can release what it holds, rather than killed outright.
// Method: the probe waits for SIGTERM and reports that it released.
func TestDispatchStopsACanceledStageWithSIGTERM(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	result := runProbe(ctx, t, t.TempDir(),
		verifyengine.Identity{Name: "term", Command: probeRoot, Args: []string{"term"}})
	if result.Completed || result.Code != verifyengine.Interrupted || result.Failure == nil || result.Failure.Kind != "interrupted" {
		t.Errorf("canceled result = %#v", result)
	}
	if !strings.Contains(result.Output, "released on SIGTERM") {
		t.Errorf("stage was not given SIGTERM: %q", result.Output)
	}
}

// TestDispatchRefusesACrashedStageAsAFailedStage proves a stage whose process
// died of a Go panic is a red stage named crashed, never the unjudged reading
// of its exit status 2, and that the trace stays in the log.
func TestDispatchRefusesACrashedStageAsAFailedStage(t *testing.T) {
	result := runProbe(context.Background(), t, t.TempDir(),
		verifyengine.Identity{Name: "panic", Command: probeRoot, Args: []string{"panic"}})
	if !result.Completed || result.Code != 1 || result.Failure == nil || result.Failure.Kind != "crashed" {
		t.Fatalf("crashed result = %#v", result)
	}
	if !strings.Contains(result.Output, "panic: BUG: probe crash") {
		t.Errorf("log lost the panic: %q", result.Output)
	}
}

// TestDispatchReadsRelayedPanicTextAsTheStageAnswer proves only the stage's own
// runtime can make it crashed: a stage that prints another program's panic
// trace and answers 2 stays the unjudged stage it said it was.
func TestDispatchReadsRelayedPanicTextAsTheStageAnswer(t *testing.T) {
	result := runProbe(context.Background(), t, t.TempDir(),
		verifyengine.Identity{Name: "relay", Command: probeRoot, Args: []string{"relay"}})
	if !result.Completed || result.Code != 2 || result.Failure != nil {
		t.Fatalf("relay result = %#v", result)
	}
	if !strings.Contains(result.Output, "panic: BUG: relayed") {
		t.Errorf("log lost the relayed text: %q", result.Output)
	}
}

// TestRunActionReexecsTheRunningImage runs a stage through the production
// route end to end: RunAction, stageProcess and /proc/self/exe, with this test
// binary as the image, its TestMain taking the le root word as le does.
func TestRunActionReexecsTheRunningImage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/proc/self/exe is Linux only; elsewhere stageProcess runs the binary's path")
	}
	registerProbe()
	root := t.TempDir()
	identity := verifyengine.Identity{Name: "reexec", Command: probeRoot, Args: []string{"alpha"}}
	result := RunAction(context.Background(), root, identity)
	if !result.Completed || result.Code != 37 || result.Failure != nil {
		t.Fatalf("reexec result = %#v", result)
	}
	if want := probeOutput(t, root, identity.Args); result.Output != want {
		t.Errorf("stdout = %q, want %q", result.Output, want)
	}
}

// TestStageProcessStartsThisBinaryUnderItsOwnName proves the stage process is
// the running image, started under the name this process was, with the le root
// word only a tagged ze build needs.
func TestStageProcessStartsThisBinaryUnderItsOwnName(t *testing.T) {
	cmd, err := stageProcess(context.Background(), []string{"arch", "tier", "check"})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "linux" && cmd.Path != "/proc/self/exe" {
		t.Errorf("path = %q, want the running image /proc/self/exe", cmd.Path)
	}
	words := []string{"arch", "tier", "check"}
	for program, want := range map[string][]string{
		"le":    {"/repo/bin/le", "arch", "tier", "check"},
		"ze le": {"/repo/bin/le", "le", "arch", "tier", "check"},
	} {
		if got := stageArgv("/repo/bin/le", program, words); !slices.Equal(got, want) {
			t.Errorf("%s argv = %q, want %q", program, got, want)
		}
	}
}
