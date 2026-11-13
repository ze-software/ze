// Design: docs/contributing/running-commands.md -- waiting for another session's job
// Overview: register_ui_le_job_report.go -- the name test/ui/le-job-report.ci runs this under
// Related: ui_fixture_le_job_admits.go -- the scratch checkout and process helpers this driver reuses

package fixture

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// leJobReportHelperEnv tells this fixture, run as the command of a job, which
// stand-in it is.
const leJobReportHelperEnv = "ZE_TEST_LE_JOB_REPORT_HELPER"

// The two jobs the scenario runs, and who each is started as. The labels
// differ, so the second cannot share the first's run and has to queue.
const (
	leJobReportFixture       = "ui/le-job-report"
	leJobReportArea          = "job"
	leJobReportSessionEnv    = "CLAUDE_CODE_SESSION_ID"
	leJobReportAgentEnv      = "ZE_AGENT_ID"
	leJobReportHolderLabel   = "ui-report-holder"
	leJobReportHolderSession = "ui-report-holder-session"
	leJobReportWaiterLabel   = "ui-report-waiter"
	leJobReportWaiterSession = "ui-report-waiter-session"
	leJobReportWaiterAgent   = "ui-report-waiter-agent"
	// leJobReportHolding is what the holder's command prints once it runs,
	// which is when its job holds the slot.
	leJobReportHolding = "HOLDING-THE-SLOT"
	leJobReportHold    = "hold"
	leJobReportQueued  = "queued"
	// leJobReportRunning is the liveness word of a row whose process the
	// report's scan listed (internal/le/job/overview.go, wordRunning).
	leJobReportRunning = "running"
	// leJobReportBound is how long the scenario waits for a process it
	// released to end. It ends a broken run and paces no working one.
	leJobReportBound = 60 * time.Second
)

// leJobReportRow is the part of a holder or waiter row the scenario judges.
type leJobReportRow struct {
	Label    string `json:"label"`
	PID      int    `json:"pid"`
	Liveness string `json:"liveness"`
	Session  string `json:"session"`
	Agent    string `json:"agent"`
}

// leJobReportDocument is `le job report | json` as the scenario reads it.
type leJobReportDocument struct {
	Slots    int                       `json:"slots"`
	Occupied int                       `json:"occupied"`
	Free     int                       `json:"free"`
	Holders  []leJobReportRow          `json:"holders"`
	Waiters  []leJobReportRow          `json:"waiters"`
	Machine  map[string]map[string]any `json:"machine"`
	Notice   string                    `json:"notice"`
}

// leJobReport proves `le job report`, typed at the built launcher, shows the
// queue: one job holds the only slot of a scratch checkout, a second waits,
// and the report lists both with their sessions, one slot occupied and none
// free, exits 0, and leaves every registry file and both processes as they
// were.
//
// Method: the holder's command is this fixture again, which blocks until the
// driver closes its stdin, so the slot is held for exactly as long as the
// driver needs. The waiter is a second `le job run` under another label. The
// driver polls the report until it lists the waiter, which is the state the
// assertions need, then judges that one answer. No step waits for a duration.
func leJobReport(ctx context.Context) error {
	switch os.Getenv(leJobReportHelperEnv) {
	case leJobReportHold:
		fmt.Println(leJobReportHolding)
		// `le job run` hands its stdin to the command it runs, so this ends
		// when the driver releases the holder.
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			return fmt.Errorf("wait for the driver to release the holder: %w", err)
		}
		return nil
	case leJobReportQueued:
		return nil
	}

	run, cleanup, err := leJobReportScratch(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	holder := run.job(ctx, leJobReportHolderLabel, map[string]string{
		leJobReportHelperEnv:  leJobReportHold,
		leJobReportSessionEnv: leJobReportHolderSession,
	})
	holderStdin, err := holder.StdinPipe()
	if err != nil {
		return fmt.Errorf("open holder stdin: %w", err)
	}
	holderStdout, err := holder.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open holder stdout: %w", err)
	}
	var holderStderr bytes.Buffer
	holder.Stderr = &holderStderr
	if err := holder.Start(); err != nil {
		return fmt.Errorf("start holder: %w", err)
	}
	// The holder's stdout is drained before its Wait, because exec forbids a
	// Wait while a pipe is still being read. holderWaiting says the Wait was
	// handed to its bridge, so the cleanup receives from it and never waits
	// a second time.
	holderWait := make(chan error, 1)
	holderDrained := make(chan struct{})
	holderWaiting, holderReaped := false, false
	defer func() {
		if holderReaped {
			return
		}
		_ = holderStdin.Close()
		_ = holder.Process.Kill()
		<-holderDrained
		if holderWaiting {
			<-holderWait
			return
		}
		_ = holder.Wait()
	}()

	reader := bufio.NewReader(holderStdout)
	announced, readErr := reader.ReadString('\n')
	go leJobAdmitsDrain(reader, holderDrained)
	if !strings.Contains(announced, leJobReportHolding) {
		// holderStderr is not read here: the holder may still write it.
		if readErr != nil {
			return fmt.Errorf("the holder never reached its command: it printed %q: %w", announced, readErr)
		}
		return fmt.Errorf("the holder never reached its command: it printed %q", announced)
	}

	waiter := run.job(ctx, leJobReportWaiterLabel, map[string]string{
		leJobReportHelperEnv:  leJobReportQueued,
		leJobReportSessionEnv: leJobReportWaiterSession,
		leJobReportAgentEnv:   leJobReportWaiterAgent,
	})
	var waiterOutput bytes.Buffer
	waiter.Stdout = &waiterOutput
	waiter.Stderr = &waiterOutput
	if err := waiter.Start(); err != nil {
		return fmt.Errorf("start waiter: %w", err)
	}
	waiterWait := make(chan error, 1)
	go leJobAdmitsWait(waiter, waiterWait)
	waiterReaped := false
	defer func() {
		if waiterReaped {
			return
		}
		_ = waiter.Process.Kill()
		<-waiterWait
	}()

	// The barrier: the report lists the waiter. A waiting job polls the
	// registry every two seconds, so the bound is the scenario's patience and
	// never its pace.
	var queued leJobReportDocument
	var lastErr error
	if !Poll(ctx, 240, 250*time.Millisecond, func() bool {
		queued, lastErr = run.report(ctx)
		return lastErr == nil && len(queued.Waiters) != 0
	}) {
		// waiterOutput is not read here: the waiter still writes it.
		if lastErr != nil {
			return fmt.Errorf("`le job report` never listed the waiting job: %w", lastErr)
		}
		return fmt.Errorf("`le job report` never listed the waiting job: %+v", queued)
	}
	if err := leJobReportJudgeQueued(&queued); err != nil {
		return err
	}
	if err := run.judgeReadOnly(ctx); err != nil {
		return err
	}
	select {
	case err := <-waiterWait:
		waiterReaped = true
		return fmt.Errorf("the waiting job ended with code %d while the slot was still held: %s", commandExitCode(err), waiterOutput.String())
	default:
	}

	// Release the holder. The waiter is then admitted, runs and ends, and the
	// report shows an empty registry with its slot free.
	if err := holderStdin.Close(); err != nil {
		return fmt.Errorf("release the holder: %w", err)
	}
	<-holderDrained
	holderWaiting = true
	go leJobAdmitsWait(holder, holderWait)
	bound := time.NewTimer(leJobReportBound)
	defer bound.Stop()
	select {
	case err := <-holderWait:
		holderReaped = true
		if code := commandExitCode(err); code != 0 {
			return fmt.Errorf("the holder answered %d: %s", code, holderStderr.String())
		}
	case <-bound.C:
		return errors.New("the holder did not finish within its bound after its release")
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-waiterWait:
		waiterReaped = true
		if code := commandExitCode(err); code != 0 {
			return fmt.Errorf("the waiting job answered %d once admitted: %s", code, waiterOutput.String())
		}
	case <-bound.C:
		return errors.New("the waiting job was not admitted within its bound after the release")
	case <-ctx.Done():
		return ctx.Err()
	}

	idle, err := run.report(ctx)
	if err != nil {
		return err
	}
	if idle.Occupied != 0 || idle.Free != 1 || len(idle.Holders) != 0 || len(idle.Waiters) != 0 {
		return fmt.Errorf("after both jobs ended the report is %+v, want no holder, no waiter and the slot free", idle)
	}

	fmt.Println("OK")
	return nil
}

// leJobReportRunner runs the built le in the scratch checkout.
type leJobReportRunner struct {
	binary      string
	testBinary  string
	scratch     string
	environment []string
}

// leJobReportScratch builds the scratch checkout whose registry the scenario
// uses, and answers the runner over it and the cleanup of its directory.
//
// The registry is the scratch tree's, with one slot, and nothing of the
// surrounding run's identity or slot reaches the driven processes: a
// functional test must not take a slot the sessions sharing this machine are
// queueing for.
func leJobReportScratch(ctx context.Context) (leJobReportRunner, func(), error) {
	testBinary, err := os.Executable()
	if err != nil {
		return leJobReportRunner{}, nil, fmt.Errorf("locate the le executable: %w", err)
	}
	testBinary, err = filepath.Abs(testBinary)
	if err != nil {
		return leJobReportRunner{}, nil, fmt.Errorf("make the le executable path absolute: %w", err)
	}
	binary, err := nativeLEBinary()
	if err != nil {
		return leJobReportRunner{}, nil, fmt.Errorf("FAIL: %w", err)
	}

	testDir, err := os.MkdirTemp("", "ze-le-job-report-")
	if err != nil {
		return leJobReportRunner{}, nil, fmt.Errorf("create fixture directory: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(testDir) //nolint:errcheck // fixture cleanup
	}

	scratch := filepath.Join(testDir, "tree")
	if err := os.MkdirAll(filepath.Join(scratch, "tmp"), 0o750); err != nil {
		cleanup()
		return leJobReportRunner{}, nil, fmt.Errorf("create scratch registry tree: %w", err)
	}
	if err := uiLeJobAdmitsCheckout(ctx, scratch); err != nil {
		cleanup()
		return leJobReportRunner{}, nil, err
	}

	environment := withoutEnv(os.Environ(), "ZE_RUN_JOB", leJobReportHelperEnv, leJobReportAgentEnv, leJobReportSessionEnv)
	environment = childEnvironment(environment, map[string]string{
		envRepoRoot:    scratch,
		"MAKEFLAGS":    "",
		"ZE_RUN_SLOTS": "1",
	})
	return leJobReportRunner{binary: binary, testBinary: testBinary, scratch: scratch, environment: environment}, cleanup, nil
}

// job builds, without starting it, one `le job run` whose command is this
// fixture as the stand-in the extra environment names.
func (r leJobReportRunner) job(ctx context.Context, label string, extra map[string]string) *exec.Cmd {
	command := exec.CommandContext(ctx, r.binary, //nolint:gosec // the fixture chooses the program and its arguments
		leJobReportArea, actionRun, "label", label, argCommand,
		r.testBinary, areaTest, "fixture", leJobReportFixture,
	)
	command.Dir = r.scratch
	command.Env = childEnvironment(r.environment, extra)
	return command
}

// le runs one le command line and answers what it printed and its code.
func (r leJobReportRunner) le(ctx context.Context, args ...string) leJobAdmitsResult {
	command := exec.CommandContext(ctx, r.binary, args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = r.scratch
	command.Env = r.environment
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return leJobAdmitsResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: commandExitCode(err)}
}

// report runs `le job report | json` and decodes it. A nonzero code or an
// answer that is not the document is an error, never an empty report.
func (r leJobReportRunner) report(ctx context.Context) (leJobReportDocument, error) {
	answer := r.le(ctx, leJobReportArea, actionReport, "|", renderJSON)
	if answer.exitCode != 0 {
		return leJobReportDocument{}, fmt.Errorf("`le job report | json` answered %d: %s", answer.exitCode, answer.stderr)
	}
	var document leJobReportDocument
	if err := json.Unmarshal([]byte(answer.stdout), &document); err != nil {
		return leJobReportDocument{}, fmt.Errorf("`le job report | json` did not answer its document: %w\n%s", err, answer.stdout)
	}
	if document.Notice == "" {
		return leJobReportDocument{}, fmt.Errorf("`le job report | json` answered no notice on what the slots count:\n%s", answer.stdout)
	}
	if len(document.Machine) == 0 {
		return leJobReportDocument{}, fmt.Errorf("`le job report | json` answered no machine reading:\n%s", answer.stdout)
	}
	return document, nil
}

// judgeReadOnly runs the bare report while one job holds the slot and one
// waits, and proves it changed no entry and no waiter record (AC-17 of
// spec-main-thread-schedules-heavy-work).
func (r leJobReportRunner) judgeReadOnly(ctx context.Context) error {
	before, err := leJobReportRecords(r.scratch)
	if err != nil {
		return err
	}
	if len(before) != 2 {
		return fmt.Errorf("the registry holds %d entries and waiter records, want the holder's entry and the waiter's record: %v", len(before), before)
	}

	text := r.le(ctx, leJobReportArea, actionReport)
	if text.exitCode != 0 {
		return fmt.Errorf("`le job report` answered %d: %s", text.exitCode, text.stderr)
	}
	if want := "job slots: 1 total, 1 occupied, 0 free"; !strings.Contains(text.stdout, want) {
		return fmt.Errorf("`le job report` printed no line %q:\n%s", want, text.stdout)
	}

	after, err := leJobReportRecords(r.scratch)
	if err != nil {
		return err
	}
	if len(after) != len(before) {
		return fmt.Errorf("the report left %d entries and waiter records where there were %d", len(after), len(before))
	}
	for name, was := range before {
		if after[name] != was {
			return fmt.Errorf("the report changed or removed the registry file %s", name)
		}
	}
	return nil
}

// leJobReportJudgeQueued judges the report taken while one job held the slot
// and one waited.
//
// A row's pid is required to name a process and to differ from the other
// row's. It is not compared with the pid this driver started: the `le` on
// PATH may hand the command to another process, and the pid an entry records
// is the one that holds the slot (internal/le/job proves that equality on
// processes it starts itself, TestWaitingJobIsListedByTheReport).
func leJobReportJudgeQueued(report *leJobReportDocument) error {
	if report.Slots != 1 || report.Occupied != 1 || report.Free != 0 {
		return fmt.Errorf("the report counts %d slots, %d occupied, %d free; want 1, 1, 0", report.Slots, report.Occupied, report.Free)
	}
	if len(report.Holders) != 1 {
		return fmt.Errorf("the report lists holders %+v, want the one job holding the slot", report.Holders)
	}
	held := report.Holders[0]
	if held.Label != leJobReportHolderLabel || held.PID <= 0 || held.Liveness != leJobReportRunning {
		return fmt.Errorf("the holder row is %+v, want label %s, a process and running", held, leJobReportHolderLabel)
	}
	if held.Session != leJobReportHolderSession || held.Agent != "" {
		return fmt.Errorf("the holder row names owner %q / %q, want session %s and no agent", held.Session, held.Agent, leJobReportHolderSession)
	}
	if len(report.Waiters) != 1 {
		return fmt.Errorf("the report lists waiters %+v, want the one queued job", report.Waiters)
	}
	queued := report.Waiters[0]
	if queued.Label != leJobReportWaiterLabel || queued.PID <= 0 || queued.Liveness != leJobReportRunning {
		return fmt.Errorf("the waiter row is %+v, want label %s, a process and running", queued, leJobReportWaiterLabel)
	}
	if queued.PID == held.PID {
		return fmt.Errorf("the holder and the waiter rows name one process, %d", held.PID)
	}
	if queued.Session != leJobReportWaiterSession || queued.Agent != leJobReportWaiterAgent {
		return fmt.Errorf("the waiter row names owner %q / %q, want %s / %s", queued.Session, queued.Agent, leJobReportWaiterSession, leJobReportWaiterAgent)
	}
	// Every machine reading is a value or says why there is none.
	for name, reading := range report.Machine {
		_, valued := reading["value"]
		reason, _ := reading["unavailable"].(string)
		if valued == (reason != "") {
			return fmt.Errorf("the machine reading %s is %v, want either a value or the reason it is unavailable", name, reading)
		}
	}
	return nil
}

// leJobReportRecords answers the bytes of every entry and waiter record in
// the scratch registry, by file name.
func leJobReportRecords(scratch string) (map[string]string, error) {
	records := map[string]string{}
	for _, pattern := range []string{"*.job", "*.wait"} {
		names, err := filepath.Glob(filepath.Join(scratch, "tmp", ".ze-jobs", pattern))
		if err != nil {
			return nil, fmt.Errorf("list the scratch registry: %w", err)
		}
		for _, name := range names {
			body, err := os.ReadFile(name) //nolint:gosec // a file in the fixture's own scratch registry
			if err != nil {
				return nil, fmt.Errorf("read the registry file %s: %w", name, err)
			}
			records[filepath.Base(name)] = string(body)
		}
	}
	return records, nil
}
