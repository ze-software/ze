// Design: docs/architecture/testing/verify-freshness-scope.md -- ordered native verification execution
package verify

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// Mode is the status identity used by the full pre-commit gate.
	Mode = "ze-precommit-verify"
	// Interrupted is the shell-compatible status for an interrupt signal.
	Interrupted = 130
)

// Failure is one structured reason a run could not certify its commit.
type Failure struct {
	Kind    string `json:"kind"`
	Stage   string `json:"stage,omitempty"`
	Message string `json:"message"`
}

// GateResult is the typed answer an in-process gate implementation returns.
// Registered and Completed are independent on purpose. Their zero values make
// an omitted dispatcher answer fail closed instead of looking like exit zero.
type GateResult struct {
	Identity   Identity `json:"identity"`
	Registered bool     `json:"registered"`
	Completed  bool     `json:"completed"`
	Code       int      `json:"code"`
	Output     string   `json:"output,omitempty"`
	Failure    *Failure `json:"failure,omitempty"`
}

// GateRunner dispatches one registered gate inside this process. A runner MUST
// execute the identity in root and return a populated GateResult. It must not
// route through Make, le, Python, or a repository script.
type GateRunner func(context.Context, string, Identity) GateResult

// StageReport records one attempted stage and the log that explains it.
type StageReport struct {
	Identity Identity `json:"identity"`
	Code     int      `json:"code"`
	Log      string   `json:"log"`
	Failure  *Failure `json:"failure,omitempty"`
}

// Report is the complete structured verdict for one fixed commit.
type Report struct {
	Mode       string        `json:"mode"`
	Commit     string        `json:"commit"`
	Code       int           `json:"code"`
	Completed  bool          `json:"completed"`
	LogDir     string        `json:"log-dir"`
	StatusPath string        `json:"status-path"`
	Stages     []StageReport `json:"stages"`
	Failure    *Failure      `json:"failure,omitempty"`
}

// Run executes every full pre-commit stage in order and writes its logs and
// status below the detached worktree. A red stage does not hide later reds; an
// interruption stops before another stage starts.
func Run(ctx context.Context, root, commit string, runner GateRunner) Report {
	return run(ctx, root, commit, runner, time.Now)
}

func run(ctx context.Context, root, commit string, runner GateRunner, now func() time.Time) Report {
	stages := FullStages()
	report := Report{
		Mode:       Mode,
		Commit:     commit,
		Code:       2,
		LogDir:     filepath.ToSlash(filepath.Join("tmp", "verify")),
		StatusPath: filepath.ToSlash(filepath.Join("tmp", "ze-verify.status")),
		Stages:     make([]StageReport, 0, len(stages)),
	}
	logDir := filepath.Join(root, filepath.FromSlash(report.LogDir))
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		report.Failure = failure("log-setup", "", messageWithError("create verify log directory: ", err))
		return report
	}

	var combined textbuf.Buffer
	combined.Str("Ze verify protocol run: ").Str(now().UTC().Format(time.RFC3339)).
		Str("\nMode: ").Str(Mode).Str("\nCommit: ").Str(commit).Str("\n\n")
	report.Code = 0
	for index, current := range stages {
		if err := ctx.Err(); err != nil {
			report.Code = Interrupted
			report.Failure = failure("interrupted", current.Identity.Gate, err.Error())
			break
		}

		expected := cloneIdentity(current.Identity)
		result := GateResult{}
		if runner != nil {
			result = runner(ctx, root, cloneIdentity(expected))
		}
		stageReport := validateResult(expected, result)
		stageReport.Log = stageLogPath(report.LogDir, index+1, expected.Gate)

		stageStart := combined.Len()
		combined.Str("### Stage ")
		appendTwoDigits(&combined, index+1)
		combined.Byte('/')
		appendTwoDigits(&combined, len(stages))
		combined.Str(": ").Str(current.Identity.Gate).Str("\nCommand: ").
			Str(invocation(current.Identity)).Byte('\n').Str(result.Output)
		if result.Output != "" && !strings.HasSuffix(result.Output, "\n") {
			combined.Byte('\n')
		}
		combined.Str("### Stage result: ").Str(current.Identity.Gate).Str(" exit=").
			Int(int64(stageReport.Code)).Byte('\n')

		if err := os.WriteFile(
			filepath.Join(root, filepath.FromSlash(stageReport.Log)),
			combined.Bytes()[stageStart:],
			0o600,
		); err != nil {
			report.Code = 2
			report.Failure = failure("log-write", current.Identity.Gate,
				messageWithError("write stage log: ", err))
			report.Stages = append(report.Stages, stageReport)
			break
		}
		report.Stages = append(report.Stages, stageReport)
		if stageReport.Code != 0 && report.Code == 0 {
			report.Code = 1
		}
		if stageReport.Failure != nil && stageReport.Failure.Kind != "stage-failed" {
			report.Code = 2
			report.Failure = stageReport.Failure
			break
		}
		if ctx.Err() != nil {
			report.Code = Interrupted
			report.Failure = failure("interrupted", current.Identity.Gate, ctx.Err().Error())
			break
		}
	}

	report.Completed = len(report.Stages) == len(stages) && report.Failure == nil
	if err := os.WriteFile(filepath.Join(logDir, "ze-verify.log"), combined.Bytes(), 0o600); err != nil {
		report.Code = 2
		report.Completed = false
		report.Failure = failure("log-write", "",
			messageWithError("write combined verify log: ", err))
	}
	if err := writeStatus(root, report, now()); err != nil {
		report.Code = 2
		report.Completed = false
		report.Failure = failure("status-write", "", err.Error())
	}
	return report
}

func validateResult(identity Identity, result GateResult) StageReport {
	report := StageReport{Identity: identity, Code: result.Code, Failure: result.Failure}
	var text textbuf.Buffer
	switch {
	case !result.Registered:
		report.Code = 2
		report.Failure = failure("unregistered", identity.Gate, "gate runner returned no registered action")
	case !result.Completed:
		report.Code = 2
		report.Failure = failure("empty-result", identity.Gate, "gate runner returned no completed result")
	case !sameIdentity(result.Identity, identity):
		report.Code = 2
		text.Str("runner answered gate ").Quoted(result.Identity.Gate).
			Str(" command ").Quoted(result.Identity.Command).Str(" args ")
		appendQuotedStrings(&text, result.Identity.Args)
		report.Failure = failure("identity-mismatch", identity.Gate, text.String())
	case result.Code < 0:
		report.Code = 2
		report.Failure = failure("invalid-status", identity.Gate,
			text.Str("gate returned negative exit status ").Int(int64(result.Code)).String())
	case result.Failure != nil && result.Code == 0:
		report.Code = 2
		report.Failure = failure("inconsistent-result", identity.Gate, "gate returned a failure with exit status zero")
	case result.Code != 0 && result.Failure == nil:
		report.Failure = failure("stage-failed", identity.Gate,
			text.Str("gate exited ").Int(int64(result.Code)).String())
	}
	return report
}

func sameIdentity(left, right Identity) bool {
	return left.Gate == right.Gate &&
		left.Command == right.Command &&
		slices.Equal(left.Args, right.Args)
}

func cloneIdentity(identity Identity) Identity {
	identity.Args = slices.Clone(identity.Args)
	return identity
}

func invocation(identity Identity) string {
	if len(identity.Args) == 0 {
		return identity.Command
	}
	var text textbuf.Buffer
	return text.Str(identity.Command).Byte(' ').Join(identity.Args, " ").String()
}

func failure(kind, stage, message string) *Failure {
	return &Failure{Kind: kind, Stage: stage, Message: message}
}

func messageWithError(prefix string, err error) string {
	var text textbuf.Buffer
	return text.Str(prefix).Err(err).String()
}

func stageLogPath(logDir string, number int, gate string) string {
	var text textbuf.Buffer
	if number < 10 {
		text.Byte('0')
	}
	name := text.Int(int64(number)).Byte('-').Str(gate).Str(".log").String()
	return filepath.ToSlash(filepath.Join(logDir, name))
}

func appendTwoDigits(text *textbuf.Buffer, value int) {
	if value < 10 {
		text.Byte('0')
	}
	text.Int(int64(value))
}

func appendQuotedStrings(text *textbuf.Buffer, values []string) {
	text.Byte('[')
	for index, value := range values {
		if index != 0 {
			text.Byte(' ')
		}
		text.Quoted(value)
	}
	text.Byte(']')
}

func writeStatus(root string, report Report, at time.Time) error {
	var status textbuf.Buffer
	status.Str("exit=").Int(int64(report.Code)).
		Str("\ntimestamp=").Str(at.UTC().Format(time.RFC3339)).
		Str("\nmode=").Str(report.Mode).
		Str("\nskipped=\ngit_sha=").Str(report.Commit).
		Str("\ntree_hash=").Str(report.Commit).Byte('\n')
	if err := os.WriteFile(
		filepath.Join(root, filepath.FromSlash(report.StatusPath)),
		status.Bytes(),
		0o600,
	); err != nil {
		return fmt.Errorf("write verify status: %w", err)
	}
	return nil
}
