// Design: docs/architecture/testing/verify-freshness-scope.md -- current-checkout verification entry points
package verify

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/setup"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// The two keywords that name a population. They are bare words, like `keep` on
// `le verify worktree`, and `current` and `list` both take them, so one spelling
// serves both actions.
const (
	keywordFull = "full"
	keywordFast = "fast"
	// keywordFirstRed stops a run at its first red stage, cheapest stages
	// first. Both entry points take it; a cut run refuses it.
	keywordFirstRed = "first-red"
	// keywordPart and keywordOf cut the worktree population into pieces.
	keywordPart = "part"
	keywordOf   = "of"
)

// jobLabel is the name a verification claims in the shared job registry.
//
// Nine sessions work this checkout, and a verification uses the whole machine:
// on 2026-09-05 sixteen of them ran at once at load 73 to 108, three past 5000
// seconds. Both entry points claim this one label, so a second verification
// queues instead of oversubscribing the box, and one asking for the SAME work
// over the SAME tree takes the running run's verdict instead of judging the
// tree twice.
const jobLabel = "verify"

// failureAdmission is the Failure.Kind for a run the registry could not admit.
// Both entry points answer with it, so a reader meets one word for one outcome.
const failureAdmission = "admission"

// stageList is the ordered native population for one mode.
type stageList struct {
	Mode   string                  `json:"mode"`
	Stages []verifyengine.Identity `json:"stages"`
}

// Text renders native stage names one per line for CI sharding.
func (l stageList) Text() string {
	var text textbuf.Buffer
	text.Reset()
	for _, identity := range l.Stages {
		text.Str(identity.Name).Byte('\n')
	}
	return text.String()
}

// listCurrent returns the ordered population for the full or the fast mode.
func listCurrent(certificateMode string) stageList {
	stages := verifyengine.StagesForMode(certificateMode)
	list := stageList{Mode: certificateMode, Stages: make([]verifyengine.Identity, len(stages))}
	for index := range stages {
		list.Stages[index] = stages[index].Identity
	}
	return list
}

// runFast admits the fast verification through the shared job registry and then
// verifies the shared checkout in place.
//
// Only the fast population runs in place. The full population runs in a fresh
// detached worktree (runWorktree), so local development never moves the tree it
// judges and the judged tree never moves under local development.
//
// The admission is what makes a second verification wait or share rather than
// start a duplicate. A run that shares one returns the holder's verdict: the
// holder judged this label's work over this tree, and the certificate on disk is
// the one it wrote.
func runFast(ctx context.Context, root string, firstRed bool, runner verifyengine.ActionRunner) verifyengine.Report {
	certificateMode := verifyengine.FastMode
	commit := job.Head(root)
	if commit == job.Unknown {
		return verifyengine.Report{Mode: certificateMode, Code: 2, Failure: &verifyengine.Failure{
			Kind: "commit-resolution", Message: "current checkout has no readable HEAD commit",
		}}
	}

	admission, err := job.NewIn(root)
	if err != nil {
		return admissionFailure(certificateMode, commit, err)
	}
	ticket, err := admission.Admit(jobLabel, fastArgv(firstRed))
	if err != nil {
		return admissionFailure(certificateMode, commit, err)
	}
	if ticket.Kind == job.KindAttached {
		return attachedReport(certificateMode, commit, ticket)
	}

	slot, closeSlot := slotFor(root, ticket)
	defer closeSlot()

	report := verifyengine.RunMode(ctx, root, commit, certificateMode,
		verifyengine.Plan{History: root, FirstRed: firstRed}, runner, slot)
	ticket.Release(report.Code)
	return report
}

// attachedReport answers the report for a run that took another run's verdict
// instead of judging the tree.
//
// It SAYS it attached. The report carries no stages and no log directory, so a
// bare Code reads as a verdict this run reached, and a zero one reads as a
// green nobody earned -- which is the reading admissionFailure below refuses to
// publish for the other path that reaches no verdict. `le verify worktree` has
// said it attached on its own diagnostics since it was written
// (lifecycle.go); this is the other half of that pair.
//
// It is a function rather than four lines inside runFast because the ticket
// is the only input: a test can hand it one and read the answer, where forcing
// a real attach means winning a race with the registry.
func attachedReport(mode, commit string, ticket *job.Ticket) verifyengine.Report {
	var text textbuf.Buffer

	return verifyengine.Report{
		Mode: mode, Commit: commit, Code: ticket.Code,
		Attached: &verifyengine.Attached{Entry: ticket.Entry, Log: ticket.Log},
		Console: text.Str("verify: shared the verification already running for ").
			Str(commit).Str(", which exited ").Int(int64(ticket.Code)).
			Str(". Its log is ").Str(ticket.Log).Byte('\n').String(),
	}
}

// fastArgv is what the registry fingerprints as this run's work. A worktree run
// judges a different stage population over a different tree, and its argv
// names `worktree`, so the two MUST NOT and cannot share one verdict. A
// first-red run stops early, so it is separate work from a run to the end.
func fastArgv(firstRed bool) []string {
	argv := []string{"le", area, "current", keywordFast}
	if firstRed {
		argv = append(argv, keywordFirstRed)
	}
	return argv
}

// admissionFailure answers the report for a run the registry could not admit.
// The run reached no verdict about the tree and MUST NOT read as one, so it
// carries the broken-run status.
func admissionFailure(mode, commit string, err error) verifyengine.Report {
	return verifyengine.Report{Mode: mode, Commit: commit, Code: 2, Failure: &verifyengine.Failure{
		Kind: failureAdmission, Message: err.Error(),
	}}
}

// slotFor answers what a claimed ticket gives the run, and the close that MUST
// run after the run ends.
//
// The log is opened before the run and closed after it, because Release removes
// the file: a writer still holding it would keep the bytes alive in a path
// nothing can read.
//
// A ticket that names no entry holds no slot of its own: it runs inside a parent
// that admitted it, and that parent stays the parent of every stage below.
func slotFor(root string, ticket *job.Ticket) (verifyengine.Slot, func()) {
	var slot verifyengine.Slot
	if ticket.Entry != "" {
		slot.Entry = filepath.Join(root, filepath.FromSlash(ticket.Entry))
	}
	if ticket.Log == "" {
		return slot, func() {}
	}

	//nolint:gosec // this run's own registry log, named from a validated label
	file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(ticket.Log)),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		leaction.ReportError(err)
		return slot, func() {}
	}
	slot.Progress = file

	return slot, func() {
		if err := file.Close(); err != nil {
			leaction.ReportError(err)
		}
	}
}

// currentHere answers `le verify current`. Bare, or with `full`, it runs the
// full population in a fresh detached worktree at HEAD, through the same
// lifecycle `le verify worktree` runs; with `fast` it runs the changed-files
// population in place.
func currentHere(args leaction.Arguments) (any, int) {
	return routeCurrent(args, currentRuns{worktree: runWorktree, inPlace: runInPlace})
}

// currentRuns holds the two runs `le verify current` can route to. They are
// parameters rather than direct calls so a test can prove which one each
// spelling reaches without creating a worktree or judging the checkout.
type currentRuns struct {
	// worktree runs the full population in a fresh detached worktree.
	worktree func(Options) (any, int)
	// inPlace runs the named population over the shared checkout.
	inPlace func(certificateMode string, firstRed bool) (any, int)
}

// routeCurrent decides where a `le verify current` line runs. Only the fast
// population runs in place: the full one MUST go to a worktree, so local
// development never moves the tree it judges.
func routeCurrent(args leaction.Arguments, runs currentRuns) (any, int) {
	certificateMode, err := population("le verify current", args)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	firstRed := args.Has(keywordFirstRed)
	if certificateMode == verifyengine.Mode {
		var text textbuf.Buffer
		text.Str("verify: le verify current runs the full population in a fresh detached worktree at HEAD, ").
			Str("as le verify worktree does; le verify current fast runs the changed-files stages in place\n").
			StdErr() //nolint:errcheck // CLI progress line
		return runs.worktree(Options{FirstRed: firstRed})
	}
	return runs.inPlace(certificateMode, firstRed)
}

// runInPlace runs the fast population over the shared checkout.
func runInPlace(certificateMode string, firstRed bool) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if err := hostPreflight(certificateMode, verifyengine.Uncut(), setup.FunctionalHostPreflight); err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	ctx, stop := signalContext()
	defer stop()
	report := runFast(ctx, root, firstRed, actionRunner())
	if report.StoppedAt != "" {
		var text textbuf.Buffer
		text.Str("verify: ").Str(certificateMode).Str(" stopped at ").Str(report.StoppedAt).Str(", ").
			Int(int64(len(report.Unreached))).Str(" stages not run; le verify current fast reruns the fast population, ").
			Str("and the stages only the full population holds need le verify worktree\n").
			StdErr() //nolint:errcheck // CLI progress line
	}
	if report.Failure != nil && withoutStageLog(report.Code) {
		leaction.ReportError(errors.New(report.Failure.Message))
	}
	return report, report.Code
}

// withoutStageLog reports whether a run's status leaves the reader no stage log
// to open. A stage that judged the tree and found it wrong wrote one, and its
// reason is in there. A run that broke, or one that reached no verdict at all,
// wrote nothing a reader can go to, so its reason belongs on stderr.
func withoutStageLog(code int) bool {
	if code == 2 {
		return true
	}
	return code == verifyengine.Unjudged
}

func listHere(args leaction.Arguments) (any, int) {
	certificateMode, err := population("le verify list", args)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return listCurrent(certificateMode), 0
}

// population answers the stage population a `current` or `list` line names:
// fast when it says `fast`, full when it says `full` or names neither. Naming
// both is refused rather than resolved, because either answer would run work
// the operator did not ask for. command is the line the operator typed, so the
// refusal names the two spellings that work.
func population(command string, args leaction.Arguments) (string, error) {
	fast := args.Has(keywordFast)
	if fast && args.Has(keywordFull) {
		var text textbuf.Buffer
		return "", errors.New(text.Str(command).Str(" takes full or fast, never both: ").
			Str(command).Str(" full is every stage, which le verify current runs in a fresh worktree at HEAD, and ").
			Str(command).Str(" fast is the changed-files stages, which le verify current runs in place").String())
	}
	if fast {
		return verifyengine.FastMode, nil
	}
	return verifyengine.Mode, nil
}
