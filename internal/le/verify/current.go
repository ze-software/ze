// Design: docs/architecture/testing/verify-freshness-scope.md -- current-checkout verification entry points
package verify

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

const (
	modeFull    = "full"
	modeChanged = "changed"
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

// stageList is the ordered native population for one current-checkout mode.
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

// listCurrent returns the ordered population for full or changed mode.
func listCurrent(mode string) (stageList, error) {
	certificateMode, err := certificateMode(mode)
	if err != nil {
		return stageList{}, err
	}
	stages := verifyengine.StagesForMode(certificateMode)
	list := stageList{Mode: certificateMode, Stages: make([]verifyengine.Identity, len(stages))}
	for index := range stages {
		list.Stages[index] = stages[index].Identity
	}
	return list, nil
}

// runCurrent admits this verification through the shared job registry and then
// verifies the shared checkout in place.
//
// The admission is what makes a second verification wait or share rather than
// start a duplicate. A run that shares one returns the holder's verdict: the
// holder judged this label's work over this tree, and the certificate on disk is
// the one it wrote.
func runCurrent(ctx context.Context, root, mode string, runner verifyengine.ActionRunner) verifyengine.Report {
	certificateMode, err := certificateMode(mode)
	if err != nil {
		return verifyengine.Report{Mode: mode, Code: 2, Failure: &verifyengine.Failure{Kind: "unknown-mode", Message: err.Error()}}
	}

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
	ticket, err := admission.Admit(jobLabel, currentArgv(certificateMode))
	if err != nil {
		return admissionFailure(certificateMode, commit, err)
	}
	if ticket.Kind == job.KindAttached {
		return verifyengine.Report{Mode: certificateMode, Commit: commit, Code: ticket.Code}
	}

	slot, closeSlot := slotFor(root, ticket)
	defer closeSlot()

	report := verifyengine.RunMode(ctx, root, commit, certificateMode, runner, slot)
	ticket.Release(report.Code)
	return report
}

// currentArgv is what the registry fingerprints as this run's work. A full run
// and a changed run judge different stage populations, so they MUST NOT share
// one verdict.
func currentArgv(certificateMode string) []string {
	return []string{"le", "verify", "current", "mode", certificateMode}
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

func currentHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	mode := args["mode"]
	ctx, stop := signalContext()
	defer stop()
	report := runCurrent(ctx, root, mode, actionRunner())
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
	list, err := listCurrent(args["mode"])
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return list, 0
}

func certificateMode(mode string) (string, error) {
	switch mode {
	case "", modeFull:
		return verifyengine.Mode, nil
	case modeChanged:
		return verifyengine.ChangedMode, nil
	default:
		return "", errors.New("verify mode must be full or changed")
	}
}
