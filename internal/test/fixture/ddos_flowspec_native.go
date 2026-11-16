// Design: docs/functional-tests.md -- native FlowSpec orphan repair drivers
// Related: ddos_flowspec_orphan_cleared.go -- the restart's clearing barrier
// Related: register_ddos_flowspec_orphan_cleared.go -- driver registration

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/anmitsu/go-shlex"
)

// ddosFlowspecCapture is runCaptured's existing process-and-stream contract.
// The injectable runner lets unit controls check ordering and failure stops.
type ddosFlowspecCapture func(context.Context, []string, string, ...string) (int, string, string, error)

const ddosFlowspecRequestWord = "request"
const ddosFlowspecDataWord = "data"

func ddosFlowspecRestoreCause(ctx context.Context, args []string) error {
	return ddosFlowspecRestoreCauseUsing(ctx, args, runCaptured, daemonAwaitAnswer)
}

func ddosFlowspecRestoreCauseUsing(ctx context.Context, args []string, run ddosFlowspecCapture, await func(context.Context, []string) error) error {
	if len(args) != 0 {
		return errors.New("usage: plugin/ddos-flowspec-restore-cause")
	}
	config, err := os.ReadFile("running.conf")
	if err != nil {
		return fmt.Errorf("read the original FlowSpec attach configuration: %w", err)
	}
	// These are disposable runner inputs, not the daemon's store. The daemon
	// must apply the restored attach through its real reload command.
	if err := os.WriteFile("running2.conf", config, 0o600); err != nil {
		return fmt.Errorf("restore the FlowSpec attach configuration: %w", err)
	}
	if err := ddosFlowspecExecute(ctx, os.Environ(), run, []string{"ze", ddosFlowspecRequestWord, actionReload}); err != nil {
		return err
	}
	return await(ctx, []string{`"peers-established": 1`, "show bgp | json"})
}

func ddosFlowspecRunRestart(ctx context.Context, args []string) error {
	return ddosFlowspecRunRestartUsing(ctx, args, runCaptured)
}

func ddosFlowspecRunRestartUsing(ctx context.Context, args []string, run ddosFlowspecCapture) error {
	if len(args) != 0 {
		return errors.New("usage: plugin/ddos-flowspec-run-restart")
	}
	environment := os.Environ()
	report, err := ddosFlowspecDoctor(ctx, environment, run)
	if err != nil {
		return err
	}
	// The original restart helper reads doctor's stdout only.
	fix, err := ddosFlowspecReportedFix(report.stdout)
	if err != nil {
		return err
	}
	if !slices.Equal(fix.argv, []string{"ze", ddosFlowspecRequestWord, "plugin", "restart", "ddos-flowspec"}) {
		return fmt.Errorf("the orphan diagnostic printed no FlowSpec restart fix: %q", fix.line)
	}
	if err := ddosFlowspecReport("FIX-RUN: " + fix.line); err != nil {
		return err
	}
	return ddosFlowspecExecute(ctx, environment, run, fix.argv)
}

func ddosFlowspecClearOrphan(ctx context.Context, args []string) error {
	return ddosFlowspecClearOrphanUsing(ctx, args, runCaptured)
}

func ddosFlowspecClearOrphanUsing(ctx context.Context, args []string, run ddosFlowspecCapture) error {
	if len(args) != 0 {
		return errors.New("usage: plugin/ddos-flowspec-clear-orphan")
	}
	environment := os.Environ()
	report, err := ddosFlowspecDoctor(ctx, environment, run)
	if err != nil {
		return err
	}
	fix, err := ddosFlowspecReportedFix(report.stdout + "\n" + report.stderr)
	if err != nil {
		return err
	}
	if len(fix.argv) != 4 {
		return fmt.Errorf("the orphan diagnostic printed no quoted withdrawal fix: %q", fix.line)
	}
	if !slices.Equal(fix.argv[:3], []string{"ze", "cli", "-c"}) {
		return fmt.Errorf("the orphan diagnostic printed no ze cli withdrawal fix: %q", fix.line)
	}
	if !strings.HasPrefix(fix.argv[3], "send bgp ") {
		return fmt.Errorf("the orphan diagnostic printed no BGP withdrawal fix: %q", fix.line)
	}
	if !strings.Contains(fix.argv[3], " update text mp ipv4/flow del ") {
		return fmt.Errorf("the orphan diagnostic printed no FlowSpec withdrawal fix: %q", fix.line)
	}
	removal, err := fix.removal()
	if err != nil {
		return err
	}
	if err := ddosFlowspecReport("FIX-RUN: " + fix.line); err != nil {
		return err
	}
	if err := ddosFlowspecExecute(ctx, environment, run, fix.argv); err != nil {
		return err
	}
	// Execute the producer's actual command through the daemon's owner route;
	// never open or edit its store from this fixture.
	if err := ddosFlowspecExecute(ctx, environment, run, removal); err != nil {
		return err
	}
	after, err := ddosFlowspecDoctor(ctx, environment, run)
	if err != nil {
		return err
	}
	text := after.stdout + "\n" + after.stderr
	if strings.Contains(text, "[doctor-runtime-not-checked]") {
		return fmt.Errorf("the second ze doctor was not answered by the daemon: %s", text)
	}
	if strings.Contains(text, "["+ddosFlowspecOrphanCode+"]") {
		return fmt.Errorf("ze doctor still reports the FlowSpec orphan after removal: %s", text)
	}
	if !ddosFlowspecDoctorCompleted(text) {
		return fmt.Errorf("the second ze doctor printed no completed report, so clearing is unproved: %q", text)
	}
	_, err = fmt.Fprintln(os.Stdout, "CLEARED: "+ddosFlowspecOrphanCode)
	return err
}

func ddosFlowspecDoctorCompleted(report string) bool {
	for raw := range strings.Lines(report) {
		line := strings.TrimSpace(raw)
		if line == "all checks passed" {
			return true
		}
		if strings.HasSuffix(line, " warnings)") {
			if strings.HasPrefix(line, "ready (") {
				return true
			}
			if strings.HasPrefix(line, "not ready (") {
				return true
			}
		}
	}
	return false
}

// ddosFlowspecFix is one uniquely identified orphan diagnostic and its parsed
// fix. Only ddosFlowspecReportedFix constructs it; its zero value is not valid.
type ddosFlowspecFix struct {
	message string
	line    string
	argv    []string
}

func ddosFlowspecReportedFix(report string) (ddosFlowspecFix, error) {
	if strings.Contains(report, "[doctor-runtime-not-checked]") {
		return ddosFlowspecFix{}, errors.New("ze doctor was not answered by the daemon; the orphan fix cannot be judged")
	}
	var fix ddosFlowspecFix
	diagnostics, fixes := 0, 0
	inOrphan := false
	for raw := range strings.Lines(report) {
		line := strings.TrimSpace(raw)
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			switch fields[0] {
			case "WARN", "ERROR", "INFO":
				inOrphan = fields[1] == "["+ddosFlowspecOrphanCode+"]"
				if inOrphan {
					diagnostics++
					fix.message = line
				}
				continue
			}
		}
		if !inOrphan {
			continue
		}
		command, isFix := strings.CutPrefix(line, "fix: ")
		if !isFix {
			continue
		}
		fixes++
		fix.line = command
	}
	if diagnostics != 1 {
		return ddosFlowspecFix{}, fmt.Errorf("ze doctor printed %d orphan diagnostics, want exactly one", diagnostics)
	}
	if fixes != 1 {
		return ddosFlowspecFix{}, fmt.Errorf("FIX-MISSING: ze doctor printed %d fixes for the orphan, want exactly one", fixes)
	}
	argv, err := shlex.Split(fix.line, true)
	if err != nil {
		return ddosFlowspecFix{}, fmt.Errorf("parse the orphan fix %q: %w", fix.line, err)
	}
	if len(argv) == 0 {
		return ddosFlowspecFix{}, errors.New("FIX-MISSING: ze doctor printed an empty orphan fix")
	}
	fix.argv = argv
	return fix, nil
}

func (fix ddosFlowspecFix) removal() ([]string, error) {
	_, tail, named := strings.Cut(fix.message, "then remove the record with ")
	if !named {
		return nil, errors.New("the orphan diagnostic did not name its record removal")
	}
	command, _, complete := strings.Cut(tail, ", which works while ze runs")
	if !complete {
		return nil, errors.New("the orphan diagnostic did not finish its record removal command")
	}
	argv, err := shlex.Split(command, true)
	if err != nil {
		return nil, fmt.Errorf("parse the orphan record removal: %w", err)
	}
	if !slices.Equal(argv, []string{"ze", ddosFlowspecDataWord, "rm", ddosFlowspecExitRecord}) {
		return nil, fmt.Errorf("the orphan diagnostic named a different record removal: %q", command)
	}
	return argv, nil
}

func ddosFlowspecDoctor(ctx context.Context, environment []string, run ddosFlowspecCapture) (awaitAnswerRun, error) {
	var report awaitAnswerRun
	report.code, report.stdout, report.stderr, report.err = run(ctx, environment, "", "ze", "doctor")
	if ctx.Err() != nil {
		return report, ctx.Err()
	}
	if report.err != nil {
		return report, fmt.Errorf("start ze doctor for the FlowSpec orphan: %w", report.err)
	}
	// Doctor's not-ready report exits 1; all other unsuccessful exits fail.
	if report.code != 0 {
		if report.code != 1 {
			return report, fmt.Errorf("ze doctor exited %d instead of answering: stdout %q, stderr %q", report.code, report.stdout, report.stderr)
		}
	}
	return report, nil
}

func ddosFlowspecExecute(ctx context.Context, environment []string, run ddosFlowspecCapture, argv []string) error {
	code, stdout, stderr, err := run(ctx, environment, "", argv...)
	if _, writeErr := fmt.Fprint(os.Stdout, stdout); writeErr != nil {
		return writeErr
	}
	if _, writeErr := fmt.Fprint(os.Stderr, stderr); writeErr != nil {
		return writeErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("start %q: %w", argv, err)
	}
	if code != 0 {
		return fmt.Errorf("%q exited %d: stdout %q, stderr %q", argv, code, stdout, stderr)
	}
	return nil
}
