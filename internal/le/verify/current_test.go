// VALIDATES: current-checkout full and changed entry points select distinct
// native populations, preserve mode certificates, publish reader artifacts, and
// enter the shared job admission so a second verification shares one run.
// PREVENTS: the sixteen concurrent verifications measured on 2026-09-05, and a
// stage that admits its own job queueing behind the run that started it.
package verify

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
	"github.com/ze-software/ze/internal/le/job"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

func TestRunCurrentFullAndChangedModes(t *testing.T) {
	for _, test := range []struct {
		mode        string
		wantMode    string
		firstAction string
		omits       string
	}{
		{mode: "full", wantMode: verifyengine.Mode, firstAction: "verify lint/run"},
		{mode: "changed", wantMode: verifyengine.ChangedMode, firstAction: "verify lint/run", omits: "verify deps/alloc"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			standalone(t)
			repo := newFixtureRepo(t)
			repo.commit(t, "fixture", "one")
			var called []string
			runner := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
				called = append(called, identity.Name)
				return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true, Output: identity.Name + " ok\n"}
			}
			report := runCurrent(context.Background(), repo.root, test.mode, runner)
			if report.Code != 0 || !report.Completed || report.Mode != test.wantMode {
				t.Fatalf("report = %#v", report)
			}
			if len(called) == 0 || called[0] != test.firstAction {
				t.Fatalf("called stages = %q", called)
			}
			if test.omits != "" && strings.Contains(strings.Join(called, " "), test.omits) {
				t.Fatalf("changed mode called full-only %s", test.omits)
			}
			certificate, err := verifyengine.ReadCertificate(repo.root)
			if err != nil || certificate.Mode != test.wantMode {
				t.Fatalf("certificate = %#v, err %v", certificate, err)
			}
			for _, rel := range []string{verifyengine.CombinedLogPath, verifyengine.FailuresLogPath, verifyengine.FailuresJSONPath} {
				if _, err := os.Stat(filepath.Join(repo.root, filepath.FromSlash(rel))); err != nil {
					t.Errorf("artifact %s: %v", rel, err)
				}
			}
			_, fullErr := os.Stat(filepath.Join(repo.root, filepath.FromSlash(verifyengine.FullJSONPath)))
			if test.mode == "full" && fullErr != nil {
				t.Errorf("full index: %v", fullErr)
			}
			if test.mode == "changed" && !os.IsNotExist(fullErr) {
				t.Errorf("changed mode wrote full index: %v", fullErr)
			}
		})
	}
}

func TestListCurrentAndModeGrammarFailClosed(t *testing.T) {
	full, err := listCurrent("")
	if err != nil || full.Mode != verifyengine.Mode || full.Stages[0].Name != "verify lint/run" {
		t.Fatalf("full list = %#v, err %v", full, err)
	}
	changed, err := listCurrent("changed")
	if err != nil || changed.Mode != verifyengine.ChangedMode || changed.Stages[0].Name != "verify lint/run" {
		t.Fatalf("changed list = %#v, err %v", changed, err)
	}
	if _, err := listCurrent("chnaged"); err == nil {
		t.Fatal("unknown mode was accepted")
	}
}

// TestVerifyCurrentEntersAdmission drives the entry point and reads the shared
// job registry while a stage runs. It proves the run holds a slot of its own,
// names that slot to every stage, and gives it back when the run ends.
//
// Until 2026-09-05 runCurrent called the engine directly and asked for nothing,
// which is how sixteen verifications ran at once at load 73 to 108.
func TestVerifyCurrentEntersAdmission(t *testing.T) {
	standalone(t)
	repo := newFixtureRepo(t)

	held := ""
	named := ""
	runner := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
		if held == "" {
			held = onlyRegistryEntry(t, repo.root)
			named = env.Get(job.ParentKey)
		}
		return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true}
	}

	report := runCurrent(context.Background(), repo.root, modeFull, runner)
	if report.Code != 0 || !report.Completed {
		t.Fatalf("report = %#v", report)
	}
	if held == "" {
		t.Fatal("no registry entry existed while a stage ran, so the run claimed no slot")
	}
	if named != held {
		t.Fatalf("stages were told their parent is %q, want the entry %q this run holds", named, held)
	}
	if left := registryEntries(t, repo.root); len(left) != 0 {
		t.Fatalf("the run left %q behind, so its slot is never given back", left)
	}
	duration, err := os.ReadFile(filepath.Join(repo.root, filepath.FromSlash(job.DurationFile)))
	if err != nil || !strings.HasPrefix(string(duration), jobLabel+"\t") {
		t.Fatalf("duration record = %q, err %v", duration, err)
	}
	if got := env.Get(job.ParentKey); got != "" {
		t.Fatalf("the run left %s naming %q, so whatever runs next reports a dead parent", job.ParentKey, got)
	}
}

// TestNestedLintStageDoesNotQueueBehindItsParent asks for the admission the
// `verify lint/run` stage asks for, from inside a stage of an admitted run.
//
// The stage runs in this process, so a lint that queued would be waiting for the
// slot its own parent holds and neither would ever finish.
func TestNestedLintStageDoesNotQueueBehindItsParent(t *testing.T) {
	standalone(t)
	repo := newFixtureRepo(t)

	var nested job.Kind
	var nestedErr error
	runner := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
		if nested == job.KindUnspecified && nestedErr == nil {
			admission, err := job.NewIn(repo.root)
			if err != nil {
				nestedErr = err
			} else {
				ticket, admitErr := admission.Admit(job.LintLabel, []string{"le", "verify", "lint", "run"})
				nestedErr = admitErr
				if ticket != nil {
					nested = ticket.Kind
					ticket.Release(0)
				}
			}
		}
		return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true}
	}

	report := runCurrent(context.Background(), repo.root, modeFull, runner)
	if report.Code != 0 {
		t.Fatalf("report = %#v", report)
	}
	if nestedErr != nil {
		t.Fatalf("the nested stage could not ask for admission: %v", nestedErr)
	}
	if nested != job.KindInside {
		t.Fatalf("the nested lint was admitted as %s, want %s: it MUST run in its parent's slot",
			nested, job.KindInside)
	}
}

// The two variables the follower process below is driven by. A test binary
// re-executed as a helper takes its whole subject from its environment.
const (
	followerRootEnv   = "ZE_TEST_VERIFY_FOLLOWER_ROOT"
	followerMarkerEnv = "ZE_TEST_VERIFY_FOLLOWER_MARKER"
	// markerUnwritable is the status the follower ends on when it cannot record
	// that it ran a stage. No verify run answers it, so the parent reads it as
	// the follower failing rather than as the holder's verdict.
	markerUnwritable = 9
)

// TestTwoVerifiesShareOneRun starts a second verification, in a second PROCESS,
// while the first is inside a stage.
//
// The second process is what makes the answer readable. Admission coordinates
// processes: two runs in one process share this process's environment, so the
// second would report itself nested inside the first and never reach the share.
// The holder exits non-zero, so the follower's status is readable as the
// holder's rather than as a default, and it is read from the process itself
// with no pipe and no wrapper between.
func TestTwoVerifiesShareOneRun(t *testing.T) {
	standalone(t)
	repo := newFixtureRepo(t)

	inStage := make(chan struct{})
	finish := make(chan struct{})
	stages := 0
	holderRunner := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
		stages++
		if stages == 1 {
			close(inStage)
			<-finish
		}
		return verifyengine.ActionResult{
			Identity: identity, Registered: true, Completed: true, Code: 1,
			Failure: &verifyengine.Failure{Kind: "stage-failed", Stage: identity.Name, Message: "red"},
		}
	}

	holder := make(chan verifyengine.Report, 1)
	go func() { holder <- runCurrent(context.Background(), repo.root, modeFull, holderRunner) }()
	<-inStage

	marker := filepath.Join(t.TempDir(), "follower-ran-a-stage")
	console := filepath.Join(t.TempDir(), "follower.log")
	follower := startFollower(t, repo.root, marker, console)

	waitForConsole(t, console, "attaching to the "+jobLabel)
	close(finish)

	holderReport := <-holder
	code := waitFollower(t, follower)

	if holderReport.Code != 1 {
		t.Fatalf("holder report = %#v, want the red its stages answered", holderReport)
	}
	if code != holderReport.Code {
		t.Fatalf("the follower process exited %d, want the holder's %d", code, holderReport.Code)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the follower ran a stage, so the tree was judged twice")
	}
}

// TestMain runs the follower process TestTwoVerifiesShareOneRun needs, and
// otherwise runs this package's tests.
//
// The follower is a second PROCESS asking for the same verification. Admission
// coordinates processes and this binary is the only one already built, so the
// child is this binary with the checkout named in its environment. The dispatch
// is here rather than inside a test, because a test that answered the
// environment would have to stand down when the variable is unset, and standing
// down is what a suite must never learn to do.
func TestMain(m *testing.M) {
	root := os.Getenv(followerRootEnv)
	if root == "" {
		os.Exit(m.Run())
	}
	os.Exit(followerVerify(root, os.Getenv(followerMarkerEnv)))
}

// followerVerify verifies the named checkout and answers that run's status.
//
// It records any stage it is asked to run, because a follower that reaches one
// judged the tree a second time, which is the failure the parent process is
// watching for. A marker that cannot be written ends this process on a status no
// verify run answers, so the parent cannot read it as the holder's verdict.
func followerVerify(root, marker string) int {
	unwritable := false
	runner := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
		if err := os.WriteFile(marker, []byte(identity.Name+"\n"), 0o600); err != nil {
			unwritable = true
		}
		return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true}
	}
	report := runCurrent(context.Background(), root, modeFull, runner)
	if unwritable {
		return markerUnwritable
	}
	return report.Code
}

// startFollower re-executes this test binary as the second verification.
//
// The child's environment names no parent job. A second session's process never
// inherits the holder's registry entry, and this process is holding one while
// the child starts: leaving it in place would make the child report itself
// nested inside the run it is supposed to share.
func startFollower(t *testing.T, root, marker, console string) *exec.Cmd {
	t.Helper()
	file, err := os.Create(console) //nolint:gosec // a path this test built under its own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})

	//nolint:gosec // the executable is this test binary and every argument is authored here
	cmd := exec.CommandContext(t.Context(), os.Args[0])
	cmd.Env = append(withoutParentJob(os.Environ()),
		followerRootEnv+"="+root, followerMarkerEnv+"="+marker)
	cmd.Stdout = file
	cmd.Stderr = file
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd
}

// waitFollower answers the follower process's own exit status.
func waitFollower(t *testing.T, cmd *exec.Cmd) int {
	t.Helper()
	err := cmd.Wait()
	if err == nil {
		return 0
	}
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("the follower process did not run: %v", err)
	}
	return exit.ExitCode()
}

// withoutParentJob drops every spelling of the job entry key from an
// environment. env.Set writes the canonical dotted name beside the underscored
// one, and env.Get treats a dot and an underscore as the same character, so
// dropping one spelling would leave the other in force.
func withoutParentJob(environ []string) []string {
	kept := make([]string, 0, len(environ))
	for _, pair := range environ {
		name, _, _ := strings.Cut(pair, "=")
		if strings.EqualFold(strings.ReplaceAll(name, "_", "."), job.ParentKey) {
			continue
		}
		kept = append(kept, pair)
	}
	return kept
}

// waitForConsole waits until a child process has written what the test needs to
// see before it moves, and fails when the child never writes it.
//
// The bound is what keeps a broken share from hanging the suite: a follower that
// queues instead of attaching writes a waiting banner and never this one.
func waitForConsole(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(followerWait)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(path) //nolint:gosec // a path this test built under its own temporary directory
		if err == nil && strings.Contains(string(body), want) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	body, _ := os.ReadFile(path) //nolint:errcheck // the read already failed the test; its output is context
	t.Fatalf("the follower never wrote %q; it wrote:\n%s", want, body)
}

// followerWait bounds that wait. The follower re-executes this test binary and
// then polls the registry, so a second is far short of what a healthy attach
// costs and a minute is far past it.
const followerWait = 60 * time.Second

// standalone drops the registry entry this test process inherited, so the run
// under test admits on its own account.
//
// Every native test action starts `go test` through `./le job run`, which
// exports its own entry to the child: without this, the run under test reports
// itself nested inside the job that started the test binary, claims no slot, and
// every assertion below it passes while measuring nothing.
func standalone(t *testing.T) {
	t.Helper()
	previous := env.Get(job.ParentKey)
	if err := env.Set(job.ParentKey, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.Set(job.ParentKey, previous); err != nil {
			t.Error(err)
		}
	})
}

// registryEntries lists the job entries under root, which is what the registry
// counts as occupied slots.
func registryEntries(t *testing.T, root string) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(job.JobsDir), "*.job"))
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// onlyRegistryEntry answers the one entry the registry holds, and fails when it
// holds any other number.
func onlyRegistryEntry(t *testing.T, root string) string {
	t.Helper()
	names := registryEntries(t, root)
	if len(names) != 1 {
		t.Fatalf("registry holds %q, want exactly one entry", names)
	}
	return names[0]
}
