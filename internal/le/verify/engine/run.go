// Design: docs/architecture/testing/verify-freshness-scope.md -- ordered native verification execution
package verifyengine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/job"
)

const (
	// Mode is the status identity for the full native action population.
	Mode = "full"
	// ChangedMode is the status identity for the scoped native population.
	ChangedMode = "changed"
	// Interrupted is the shell-compatible status for an interrupt signal.
	Interrupted = 130
	// Unjudged is the status of a run that reached no verdict about the tree.
	// It is neither a pass nor a red, and the three codes it sits beside are
	// taken: 0 certifies the tree, 1 says a stage judged it and found it wrong,
	// and 2 says the run itself broke. A caller that reads them apart keeps
	// reading them apart.
	Unjudged = 3
	// stageUnjudged is the status an ACTION answers when it could not judge its
	// own subject. It is the convention across le actions, and
	// `le staticcheck-feature-matrix check` is the stage that answers it for the
	// full population (internal/le/staticcheckfeaturematrix, runCheck).
	stageUnjudged = 2
)

// Defeated reports whether err is a full device.
//
// A device with no space left defeats a run rather than telling it anything
// about the tree: nothing the run wrote survived, so it cleared nothing. It is
// recognized at the write site that holds the typed error, never by matching
// text, because no stage output reaches a Report: validateResult records only
// "action exited N", StageReport carries no output field, and Report.Console is
// `json:"-"`.
func Defeated(err error) bool { return errors.Is(err, syscall.ENOSPC) }

// runCode folds one stage's status into the run's.
//
// A stage that could not judge its subject is UNJUDGED rather than failed,
// because flattening it reports a verdict the run never reached. A failure
// outranks it: a stage that found the tree wrong DID judge the tree.
func runCode(current, stage int) int {
	if current == 1 {
		return 1
	}
	if stage == stageUnjudged {
		return Unjudged
	}
	return 1
}

// brokenCode answers the status a verification write failure earns. A full
// device defeated the run before it could record what it saw, so the run judged
// nothing; any other write failure is the run itself breaking.
func brokenCode(err error) int {
	if Defeated(err) {
		return Unjudged
	}
	return 2
}

// Failure is one structured reason a run could not certify its commit.
type Failure struct {
	Kind    string `json:"kind"`
	Stage   string `json:"stage,omitempty"`
	Message string `json:"message"`
}

// ActionResult is the typed answer an in-process action implementation returns.
// Registered and Completed are independent so an omitted dispatcher answer
// fails closed instead of looking like exit zero.
type ActionResult struct {
	Identity   Identity `json:"identity"`
	Registered bool     `json:"registered"`
	Completed  bool     `json:"completed"`
	Code       int      `json:"code"`
	Output     string   `json:"output,omitempty"`
	Failure    *Failure `json:"failure,omitempty"`
}

// ActionRunner dispatches one registered action inside this process. A runner
// MUST execute the identity in root and return a populated ActionResult.
type ActionRunner func(context.Context, string, Identity) ActionResult

// StageReport records one attempted stage and the log that explains it.
type StageReport struct {
	Identity Identity `json:"identity"`
	Code     int      `json:"code"`
	Log      string   `json:"log"`
	Failure  *Failure `json:"failure,omitempty"`
}

type Report struct {
	Mode       string        `json:"mode"`
	Commit     string        `json:"commit"`
	Code       int           `json:"code"`
	Completed  bool          `json:"completed"`
	LogDir     string        `json:"log-dir"`
	StatusPath string        `json:"status-path"`
	Stages     []StageReport `json:"stages"`
	Failure    *Failure      `json:"failure,omitempty"`
	Console    string        `json:"-"`
}

// Text renders the run protocol and every stage's captured output.
func (r Report) Text() string { return r.Console }

// Run executes every full verification stage in order at root and writes its
// logs and status there. A red stage does not hide later reds; an interruption
// stops before another stage starts.
func Run(ctx context.Context, root, commit string, runner ActionRunner) Report {
	return RunMode(ctx, root, commit, Mode, runner)
}

// RunMode executes the native stage population selected by mode.
func RunMode(ctx context.Context, root, commit, mode string, runner ActionRunner) Report {
	leave, err := enterVerifyEnvironment()
	if err != nil {
		return Report{Mode: mode, Commit: commit, Code: 2, Failure: failure("environment", "", err.Error())}
	}
	defer leave()
	return runMode(ctx, root, commit, mode, runner, time.Now)
}

func run(ctx context.Context, root, commit string, runner ActionRunner, now func() time.Time) Report {
	return runMode(ctx, root, commit, Mode, runner, now)
}

func runMode(ctx context.Context, root, commit, mode string, runner ActionRunner, now func() time.Time) Report {
	stages := StagesForMode(mode)
	report := Report{
		Mode:       mode,
		Commit:     commit,
		Code:       2,
		StatusPath: StatusPath,
		Stages:     make([]StageReport, 0, len(stages)),
	}
	if len(stages) == 0 {
		report.Failure = failure("unknown-mode", "", "no verify stages configured for mode "+mode)
		return report
	}
	start := job.SnapshotTree(root)
	started := now()
	logFailure := func(err error) Report {
		report.Code = brokenCode(err)
		report.Failure = failure("log-setup", "", messageWithError("create verify log directory: ", err))
		if _, statusErr := WriteCertificate(root, WriteRequest{
			Exit: report.Code, Mode: report.Mode, Skipped: SkippedSuites(),
			GitSHA: commit, Start: start, At: now(),
		}); statusErr != nil {
			report.Failure = failure("status-write", "", statusErr.Error())
		}
		return report
	}
	logParent := filepath.Join(root, "tmp", "verify")
	if err := os.MkdirAll(logParent, 0o750); err != nil {
		return logFailure(err)
	}
	logDir, err := os.MkdirTemp(logParent, mode+"-")
	if err != nil {
		return logFailure(err)
	}
	logRel, err := filepath.Rel(root, logDir)
	if err != nil {
		return logFailure(err)
	}
	report.LogDir = filepath.ToSlash(logRel)

	var combined textbuf.Buffer
	combined.Str("Ze verify protocol run: ").Str(started.UTC().Format(time.RFC3339)).
		Str("\nMode: ").Str(mode).Str("\nCommit: ").Str(commit).Str("\n\n")
	report.Code = 0
	for index, current := range stages {
		if err := ctx.Err(); err != nil {
			report.Code = Interrupted
			report.Failure = failure("interrupted", current.Identity.Name, err.Error())
			break
		}

		expected := cloneIdentity(current.Identity)
		result := ActionResult{}
		if runner != nil {
			result = runner(ctx, root, cloneIdentity(expected))
		}
		stageReport := validateResult(expected, result)
		stageReport.Log = stageLogPath(report.LogDir, index+1, expected.Name)

		stageStart := combined.Len()
		combined.Str("### Stage ")
		appendTwoDigits(&combined, index+1)
		combined.Byte('/')
		appendTwoDigits(&combined, len(stages))
		combined.Str(": ").Str(current.Identity.Name).Str("\nCommand: ").
			Str(invocation(current.Identity)).Byte('\n').Str(result.Output)
		if result.Output != "" && !strings.HasSuffix(result.Output, "\n") {
			combined.Byte('\n')
		}
		combined.Str("### Stage result: ").Str(current.Identity.Name).Str(" exit=").
			Int(int64(stageReport.Code)).Byte('\n')

		if err := os.WriteFile(
			filepath.Join(root, filepath.FromSlash(stageReport.Log)),
			combined.Bytes()[stageStart:],
			0o600,
		); err != nil {
			report.Code = brokenCode(err)
			report.Failure = failure("log-write", current.Identity.Name,
				messageWithError("write stage log: ", err))
			report.Stages = append(report.Stages, stageReport)
			break
		}
		report.Stages = append(report.Stages, stageReport)
		if stageReport.Code != 0 {
			report.Code = runCode(report.Code, stageReport.Code)
		}
		if stageReport.Failure != nil && stageReport.Failure.Kind != "stage-failed" {
			report.Code = 2
			report.Failure = stageReport.Failure
			break
		}
		if ctx.Err() != nil {
			report.Code = Interrupted
			report.Failure = failure("interrupted", current.Identity.Name, ctx.Err().Error())
			break
		}
	}

	report.Completed = len(report.Stages) == len(stages) && report.Failure == nil
	if err := os.WriteFile(filepath.Join(logDir, "ze-verify.log"), combined.Bytes(), 0o600); err != nil {
		report.Code = brokenCode(err)
		report.Completed = false
		report.Failure = failure("log-write", "",
			messageWithError("write combined verify log: ", err))
	}
	report.Console = combined.String()
	if err := writeRunArtifacts(root, report, started); err != nil {
		report.Code = brokenCode(err)
		report.Completed = false
		report.Failure = failure("artifact-write", "", err.Error())
	}
	if _, err := WriteCertificate(root, WriteRequest{
		Exit: report.Code, Mode: report.Mode, Skipped: SkippedSuites(),
		GitSHA: commit, Start: start, At: now(),
	}); err != nil {
		report.Code = brokenCode(err)
		report.Completed = false
		report.Failure = failure("status-write", "", err.Error())
	}
	return report
}

var verifyEnvironment struct {
	sync.Mutex
	active   int
	previous string
	had      bool
}

func enterVerifyEnvironment() (func(), error) {
	verifyEnvironment.Lock()
	if verifyEnvironment.active == 0 {
		verifyEnvironment.previous, verifyEnvironment.had = os.LookupEnv("ZE_VERIFY_MODE")
		if err := os.Setenv("ZE_VERIFY_MODE", "1"); err != nil {
			verifyEnvironment.Unlock()
			return nil, err
		}
	}
	verifyEnvironment.active++
	verifyEnvironment.Unlock()
	return func() {
		verifyEnvironment.Lock()
		defer verifyEnvironment.Unlock()
		verifyEnvironment.active--
		if verifyEnvironment.active != 0 {
			return
		}
		if verifyEnvironment.had {
			_ = os.Setenv("ZE_VERIFY_MODE", verifyEnvironment.previous)
		} else {
			_ = os.Unsetenv("ZE_VERIFY_MODE")
		}
	}, nil
}

func validateResult(identity Identity, result ActionResult) StageReport {
	report := StageReport{Identity: identity, Code: result.Code, Failure: result.Failure}
	var text textbuf.Buffer
	switch {
	case !result.Registered:
		report.Code = 2
		report.Failure = failure("unregistered", identity.Name, "action runner returned no registered action")
	case !result.Completed:
		report.Code = 2
		report.Failure = failure("empty-result", identity.Name, "action runner returned no completed result")
	case !sameIdentity(result.Identity, identity):
		report.Code = 2
		text.Str("runner answered action ").Quoted(result.Identity.Name).
			Str(" command ").Quoted(result.Identity.Command).Str(" args ")
		appendQuotedStrings(&text, result.Identity.Args)
		report.Failure = failure("identity-mismatch", identity.Name, text.String())
	case result.Code < 0:
		report.Code = 2
		report.Failure = failure("invalid-status", identity.Name,
			text.Str("action returned negative exit status ").Int(int64(result.Code)).String())
	case result.Failure != nil && result.Code == 0:
		report.Code = 2
		report.Failure = failure("inconsistent-result", identity.Name, "action returned a failure with exit status zero")
	case result.Code != 0 && result.Failure == nil:
		report.Failure = failure("stage-failed", identity.Name,
			text.Str("action exited ").Int(int64(result.Code)).String())
	}
	return report
}

func sameIdentity(left, right Identity) bool {
	return left.Name == right.Name &&
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

func stageLogPath(logDir string, number int, stage string) string {
	var text textbuf.Buffer
	if number < 10 {
		text.Byte('0')
	}
	// Both separators flatten to a hyphen, so an artifact path never moves and
	// never holds a space. A stage names a command and its verb, and a command
	// is now an object and a member: `verify lint/run` is three words and one
	// file, 01-verify-lint-run.log, which is the name it had when the command
	// was spelled verify-lint. The failure index, every rerun line and the
	// functional fixtures all read these paths.
	stage = strings.ReplaceAll(stage, "/", "-")
	stage = strings.ReplaceAll(stage, " ", "-")
	name := text.Int(int64(number)).Byte('-').Str(stage).Str(".log").String()
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
