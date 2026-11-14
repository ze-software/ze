// Design: docs/contributing/running-commands.md -- waiting for another session's job
// Overview: register_ui_le_job_admits_heavy_action.go -- the name test/ui/le-job-admits-heavy-action.ci runs this under
// Related: ui_fixture_le_job_report.go -- the scratch registry, the runner and the report document this driver reuses
// Related: ui_fixture_le_job_admits.go -- the process helpers this driver reuses

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	leHeavyFixture = "ui/le-job-admits-heavy-action"
	// leHeavyArea and leHeavyVerb are the heavy action the scenario drives:
	// `le chaos run`, which registers heavy by its name and declares no
	// handling of its own, so the dispatcher admits it (leroot.SlotOfLine
	// answers SlotCentral).
	leHeavyArea = "chaos"
	leHeavyVerb = "run"
	// leHeavyLabel is the label the dispatcher holds `chaos run` under: its
	// command name with the space as a hyphen (job.centralLabelOf).
	leHeavyLabel = "chaos-run"
	// leHeavyParentLabel is the `le job run` that starts a heavy le line as
	// its command, for the tree case.
	leHeavyParentLabel = "ui-heavy-tree-parent"
	// leHeavyConfigMark is a word every generated chaos config holds, read to
	// prove the body wrote the config and not an empty file.
	leHeavyConfigMark = "bgp"
	leHeavyHelp       = "help"
	// leHeavyBound is how long one step waits for a state the scenario caused.
	// It ends a broken run and paces no working one.
	leHeavyBound = 60 * time.Second
)

// leJobAdmitsHeavyAction proves, through the built launcher, that the
// dispatcher takes one job slot for a heavy action before its body runs: the
// running action is a holder under its label, a second one waits for the only
// slot and runs once it frees, a heavy le line started inside a job takes no
// slot of its own, and a light line answers while the only slot is held.
//
// Method: the heavy action is `le chaos run --config-only --config-out <fifo>`.
// Its body opens the FIFO for writing, and that open blocks until this driver
// opens the other end, so the body is held inside its slot for exactly as long
// as the driver needs, and the driver's own open returning is the proof that
// the body started. The registry is a scratch checkout with one slot
// (leJobReportScratch). No step waits for a duration.
func leJobAdmitsHeavyAction(ctx context.Context) error {
	run, cleanup, err := leJobReportScratch(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	// The FIFOs sit beside the scratch tree, outside the checkout le judges.
	dir := filepath.Dir(run.scratch)
	if err := leHeavyHolderAndWaiter(ctx, run, dir); err != nil {
		return err
	}
	if err := leHeavyTreeCountsOnce(ctx, run, dir); err != nil {
		return err
	}
	fmt.Println("OK")
	return nil
}

// leHeavyHolderAndWaiter runs steps 1, 2 and 4: a held heavy action is the one
// holder, light lines answer meanwhile, a second heavy action waits and does
// not start, and once the holder ends the waiter runs and the registry is
// empty.
func leHeavyHolderAndWaiter(ctx context.Context, run leJobReportRunner, dir string) error {
	holder, err := leHeavyStart(ctx, run, filepath.Join(dir, "holder.fifo"))
	if err != nil {
		return err
	}
	defer holder.stop()

	// The barrier: the report lists the holder. Its entry is written when the
	// slot is claimed, and the body cannot end before this driver opens the
	// holder's FIFO.
	held, err := leHeavyAwaitReport(ctx, run, holder, func(report *leJobReportDocument) bool {
		return len(report.Holders) != 0
	})
	if err != nil {
		return fmt.Errorf("`le job report` never listed the running heavy action as a holder: %w", err)
	}
	if err := leHeavyJudgeHeld(&held, leHeavyLabel); err != nil {
		return err
	}
	if len(held.Waiters) != 0 {
		return fmt.Errorf("with one heavy action running the report lists waiters %+v, want none", held.Waiters)
	}

	if err := leHeavyLightLines(ctx, run); err != nil {
		return err
	}

	waiter, err := leHeavyStart(ctx, run, filepath.Join(dir, "waiter.fifo"))
	if err != nil {
		return err
	}
	defer waiter.stop()
	// The waiter's FIFO is opened at once: its open returns only when the
	// waiter's body opens the other end, so it must not return while the
	// holder still holds the only slot.
	waiterReader := leHeavyStartReader(waiter.fifo)
	defer waiterReader.close()

	queued, err := leHeavyAwaitReport(ctx, run, waiter, func(report *leJobReportDocument) bool {
		return len(report.Waiters) != 0
	})
	if err != nil {
		return fmt.Errorf("`le job report` never listed the second heavy action as a waiter: %w", err)
	}
	if err := leHeavyJudgeQueued(&queued); err != nil {
		return err
	}
	if waiterReader.openedYet() {
		return errors.New("the second heavy action started its body while the first held the only slot")
	}

	// Release the holder: open its FIFO, read the config its body writes, and
	// see it end. The waiter is then admitted and its body opens its FIFO.
	if err := leHeavyRelease(ctx, holder); err != nil {
		return err
	}
	config, err := waiterReader.await(ctx)
	if err != nil {
		return fmt.Errorf("the second heavy action never started once the slot was free: %w", err)
	}
	if !strings.Contains(config, leHeavyConfigMark) {
		return fmt.Errorf("the second heavy action wrote no chaos config once admitted: %q", config)
	}
	if err := waiter.wait(ctx); err != nil {
		return err
	}
	return leHeavyJudgeIdle(ctx, run)
}

// leHeavyTreeCountsOnce runs step 3: `le job run` holds the only slot and its
// command is a heavy le line, which runs inside that slot and asks for none of
// its own. With one slot, a child that asked would wait for its own parent
// forever, so its body opening the FIFO at all is the proof.
func leHeavyTreeCountsOnce(ctx context.Context, run leJobReportRunner, dir string) error {
	fifo := filepath.Join(dir, "tree.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		return fmt.Errorf("make the FIFO %s: %w", fifo, err)
	}
	heavy := leHeavyWords(fifo)
	words := make([]string, 0, 6+len(heavy))
	words = append(words, leJobReportArea, actionRun, "label", leHeavyParentLabel, argCommand, run.binary)
	words = append(words, heavy...)
	parent := exec.CommandContext(ctx, run.binary, words...) //nolint:gosec // the fixture chooses the program and its arguments
	parent.Dir = run.scratch
	parent.Env = run.environment
	tree, err := leHeavyRun(parent, fifo)
	if err != nil {
		return err
	}
	defer tree.stop()

	held, err := leHeavyAwaitReport(ctx, run, tree, func(report *leJobReportDocument) bool {
		return len(report.Holders) != 0
	})
	if err != nil {
		return fmt.Errorf("`le job report` never listed the parent job as a holder: %w", err)
	}
	if err := leHeavyJudgeHeld(&held, leHeavyParentLabel); err != nil {
		return err
	}

	// From here every report is judged until the nested line's body opens
	// the FIFO: a waiter row in any of them is the nested line asking for a
	// slot of its own, which it would wait for until the bound.
	reader := leHeavyStartReader(fifo)
	defer reader.close()
	var nestedErr, lastErr error
	if !Poll(ctx, 240, 250*time.Millisecond, func() bool {
		if reader.openedYet() {
			return true
		}
		report, err := run.report(ctx)
		if err != nil {
			lastErr = err
			return false
		}
		if len(report.Waiters) != 0 {
			nestedErr = fmt.Errorf("the heavy le line inside the parent job asked for a slot of its own: the report lists waiters %+v", report.Waiters)
			return true
		}
		if report.Occupied > 1 {
			nestedErr = fmt.Errorf("the parent job and its nested heavy line occupy %d slots, want 1", report.Occupied)
			return true
		}
		return false
	}) {
		if lastErr != nil {
			return fmt.Errorf("the heavy le line inside the parent job never started its body: %w", lastErr)
		}
		return errors.New("the heavy le line inside the parent job never started its body")
	}
	if nestedErr != nil {
		return nestedErr
	}
	config, err := reader.await(ctx)
	if err != nil {
		return err
	}
	if !strings.Contains(config, leHeavyConfigMark) {
		return fmt.Errorf("the heavy le line inside the parent job wrote no chaos config: %q", config)
	}
	if err := tree.wait(ctx); err != nil {
		return err
	}
	return leHeavyJudgeIdle(ctx, run)
}

// leHeavyLightLines runs `le help` and `le job report` while the only slot is
// held. A light line takes no slot, so each answers at once; one the
// dispatcher admitted would wait for the held slot until the bound ended it.
func leHeavyLightLines(ctx context.Context, run leJobReportRunner) error {
	bounded, cancel := context.WithTimeout(ctx, leHeavyBound)
	defer cancel()

	help := run.le(bounded, leHeavyHelp)
	if help.exitCode != 0 {
		return fmt.Errorf("`le help` answered %d while the only slot was held: %s", help.exitCode, help.stderr)
	}
	if strings.TrimSpace(help.stdout) == "" {
		return errors.New("`le help` printed nothing while the only slot was held")
	}
	report := run.le(bounded, leJobReportArea, actionReport)
	if report.exitCode != 0 {
		return fmt.Errorf("`le job report` answered %d while the only slot was held: %s", report.exitCode, report.stderr)
	}
	if want := "job slots: 1 total, 1 occupied, 0 free"; !strings.Contains(report.stdout, want) {
		return fmt.Errorf("`le job report` printed no line %q while the only slot was held:\n%s", want, report.stdout)
	}
	return nil
}

// leHeavyJudgeHeld judges a report taken while one job holds the only slot:
// one occupied, none free, and the one holder running under label.
func leHeavyJudgeHeld(report *leJobReportDocument, label string) error {
	if report.Slots != 1 {
		return fmt.Errorf("the report counts %d slots, want the 1 ZE_RUN_SLOTS sets", report.Slots)
	}
	if report.Occupied != 1 {
		return fmt.Errorf("with one job running the report counts %d occupied slots, want 1: %+v", report.Occupied, report.Holders)
	}
	if report.Free != 0 {
		return fmt.Errorf("with the only slot held the report counts %d free, want 0", report.Free)
	}
	if len(report.Holders) != 1 {
		return fmt.Errorf("the report lists holders %+v, want the one running job", report.Holders)
	}
	row := report.Holders[0]
	if row.Label != label {
		return fmt.Errorf("the holder row is labeled %q, want %q: %+v", row.Label, label, row)
	}
	if row.PID <= 0 {
		return fmt.Errorf("the holder row names no process: %+v", row)
	}
	if row.Liveness != leJobReportRunning {
		return fmt.Errorf("the holder row is %q, want %s: %+v", row.Liveness, leJobReportRunning, row)
	}
	return nil
}

// leHeavyJudgeQueued judges the report taken while the first heavy action
// holds the only slot and the second waits for it.
func leHeavyJudgeQueued(report *leJobReportDocument) error {
	if err := leHeavyJudgeHeld(report, leHeavyLabel); err != nil {
		return err
	}
	if len(report.Waiters) != 1 {
		return fmt.Errorf("the report lists waiters %+v, want the one queued heavy action", report.Waiters)
	}
	row := report.Waiters[0]
	if row.Label != leHeavyLabel {
		return fmt.Errorf("the waiter row is labeled %q, want %q: %+v", row.Label, leHeavyLabel, row)
	}
	if row.Liveness != leJobReportRunning {
		return fmt.Errorf("the waiter row is %q, want %s: %+v", row.Liveness, leJobReportRunning, row)
	}
	if row.PID == report.Holders[0].PID {
		return fmt.Errorf("the holder and the waiter rows name one process, %d", row.PID)
	}
	return nil
}

// leHeavyJudgeIdle takes the report once every job ended: no holder, no
// waiter, and the slot free.
func leHeavyJudgeIdle(ctx context.Context, run leJobReportRunner) error {
	idle, err := run.report(ctx)
	if err != nil {
		return err
	}
	if idle.Occupied != 0 {
		return fmt.Errorf("after every job ended the report counts %d occupied: %+v", idle.Occupied, idle)
	}
	if idle.Free != 1 {
		return fmt.Errorf("after every job ended the report counts %d free, want 1: %+v", idle.Free, idle)
	}
	if len(idle.Holders) != 0 {
		return fmt.Errorf("after every job ended the report lists holders %+v", idle.Holders)
	}
	if len(idle.Waiters) != 0 {
		return fmt.Errorf("after every job ended the report lists waiters %+v", idle.Waiters)
	}
	return nil
}

// leHeavyAwaitReport polls the report until done holds of it, and answers that
// report. A line that ends first is an error: the state it was to reach can no
// longer come.
func leHeavyAwaitReport(ctx context.Context, run leJobReportRunner, line *leHeavyLine, done func(*leJobReportDocument) bool) (leJobReportDocument, error) {
	var report leJobReportDocument
	var lastErr error
	ended := false
	reached := Poll(ctx, 240, 250*time.Millisecond, func() bool {
		if line.ended() {
			ended = true
			return true
		}
		report, lastErr = run.report(ctx)
		if lastErr != nil {
			return false
		}
		return done(&report)
	})
	if ended {
		return report, fmt.Errorf("the line ended first, with code %d: %s", line.code, line.output.String())
	}
	if reached {
		return report, nil
	}
	if lastErr != nil {
		return report, lastErr
	}
	return report, fmt.Errorf("the last report was %+v", report)
}

// leHeavyRelease opens the holder's FIFO, reads the config its body writes,
// and waits for it to end with code 0.
func leHeavyRelease(ctx context.Context, holder *leHeavyLine) error {
	reader := leHeavyStartReader(holder.fifo)
	defer reader.close()

	config, err := reader.await(ctx)
	if err != nil {
		return fmt.Errorf("the held heavy action never opened its FIFO: %w", err)
	}
	if !strings.Contains(config, leHeavyConfigMark) {
		return fmt.Errorf("the held heavy action wrote no chaos config: %q", config)
	}
	return holder.wait(ctx)
}

// leHeavyWords is the heavy action's line after `le`, with its config written
// to fifo.
func leHeavyWords(fifo string) []string {
	return []string{leHeavyArea, leHeavyVerb, "--config-only", argQuiet, "--peers", "3", "--config-out", fifo}
}

// leHeavyStart makes fifo and starts the heavy action that writes its config
// to it, in the scratch checkout.
func leHeavyStart(ctx context.Context, run leJobReportRunner, fifo string) (*leHeavyLine, error) {
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		return nil, fmt.Errorf("make the FIFO %s: %w", fifo, err)
	}
	command := exec.CommandContext(ctx, run.binary, leHeavyWords(fifo)...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = run.scratch
	command.Env = run.environment
	return leHeavyRun(command, fifo)
}

// leHeavyLine is one started le process whose heavy body writes to fifo.
// NOT safe for concurrent use: the driver alone calls its methods.
type leHeavyLine struct {
	command *exec.Cmd
	// output is what the process printed. It is read only after done
	// delivered, because exec writes it until the process ends.
	output *bytes.Buffer
	done   chan error
	fifo   string
	reaped bool
	code   int
}

// leHeavyRun starts command, whose heavy body writes to fifo.
func leHeavyRun(command *exec.Cmd, fifo string) (*leHeavyLine, error) {
	output := &bytes.Buffer{}
	command.Stdout = output
	command.Stderr = output
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start %v: %w", command.Args, err)
	}
	line := &leHeavyLine{command: command, output: output, done: make(chan error, 1), fifo: fifo}
	go leJobAdmitsWait(command, line.done)
	return line, nil
}

// ended reports whether the process has ended, without waiting.
func (l *leHeavyLine) ended() bool {
	if l.reaped {
		return true
	}
	select {
	case err := <-l.done:
		l.reaped = true
		l.code = commandExitCode(err)
		return true
	default:
		return false
	}
}

// wait waits, within the bound, for the process to end, and answers an error
// unless it ended with code 0.
func (l *leHeavyLine) wait(ctx context.Context) error {
	if !l.reaped {
		bound := time.NewTimer(leHeavyBound)
		defer bound.Stop()
		select {
		case err := <-l.done:
			l.reaped = true
			l.code = commandExitCode(err)
		case <-bound.C:
			return fmt.Errorf("%v did not end within its bound", l.command.Args)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if l.code != 0 {
		return fmt.Errorf("%v answered %d: %s", l.command.Args, l.code, l.output.String())
	}
	return nil
}

// stop ends a process the scenario did not see end. A body blocked opening
// its FIFO for writing is released first by a reader that opens and closes
// at once, in case the pid exec knows is not the process that holds the
// FIFO; the kill then ends the line wherever it is.
func (l *leHeavyLine) stop() {
	if l.reaped {
		return
	}
	reader, err := os.OpenFile(l.fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0) //nolint:gosec // a FIFO in the fixture's own scratch directory
	if err == nil {
		_ = reader.Close() //nolint:errcheck // fixture cleanup
	}
	_ = l.command.Process.Kill() //nolint:errcheck // fixture cleanup of a process that may already have ended
	<-l.done
	l.reaped = true
}

// leHeavyOpened is the result of one blocking open of a FIFO for reading.
type leHeavyOpened struct {
	file *os.File
	err  error
}

// leHeavyReader opens a FIFO for reading on its own goroutine, because that
// open blocks until a writer opens the other end. NOT safe for concurrent use.
type leHeavyReader struct {
	fifo     string
	opened   chan leHeavyOpened
	received bool
	result   leHeavyOpened
}

// leHeavyStartReader starts the blocking open of fifo. The caller MUST call
// close.
func leHeavyStartReader(fifo string) *leHeavyReader {
	reader := &leHeavyReader{fifo: fifo, opened: make(chan leHeavyOpened, 1)}
	go leHeavyOpenFIFO(fifo, reader.opened)
	return reader
}

// leHeavyOpenFIFO is the reader's goroutine: one open, one result.
func leHeavyOpenFIFO(fifo string, opened chan<- leHeavyOpened) {
	file, err := os.Open(fifo) //nolint:gosec // a FIFO in the fixture's own scratch directory
	opened <- leHeavyOpened{file: file, err: err}
}

// openedYet reports whether the open returned, which is when a writer opened
// the FIFO, without waiting.
func (r *leHeavyReader) openedYet() bool {
	if r.received {
		return true
	}
	select {
	case r.result = <-r.opened:
		r.received = true
		return true
	default:
		return false
	}
}

// await waits, within the bound, for a writer, and answers everything it
// wrote up to its close.
func (r *leHeavyReader) await(ctx context.Context) (string, error) {
	if !r.received {
		bound := time.NewTimer(leHeavyBound)
		defer bound.Stop()
		select {
		case r.result = <-r.opened:
			r.received = true
		case <-bound.C:
			return "", fmt.Errorf("no writer opened %s within its bound", r.fifo)
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if r.result.err != nil {
		return "", fmt.Errorf("open %s: %w", r.fifo, r.result.err)
	}
	body, err := io.ReadAll(r.result.file)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", r.fifo, err)
	}
	return string(body), nil
}

// close ends the reader. An open still blocked is released by a writer that
// opens and closes at once; the reader's own open counts as the reader that
// lets a non-blocking writer open.
func (r *leHeavyReader) close() {
	if !r.received {
		writer, err := os.OpenFile(r.fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0) //nolint:gosec // a FIFO in the fixture's own scratch directory
		if err == nil {
			_ = writer.Close() //nolint:errcheck // fixture cleanup
		}
		bound := time.NewTimer(leHeavyBound)
		defer bound.Stop()
		select {
		case r.result = <-r.opened:
			r.received = true
		case <-bound.C:
			return
		}
	}
	if r.result.file != nil {
		_ = r.result.file.Close() //nolint:errcheck // fixture cleanup
	}
}
