// Design: docs/architecture/testing/verify-freshness-scope.md -- current-checkout verification entry points
package verify

import (
	"context"
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/verifyengine"
)

const (
	modeFull    = "full"
	modeChanged = "changed"
)

// stageList is the ordered native population for one current-checkout mode.
type stageList struct {
	Mode   string                  `json:"mode"`
	Stages []verifyengine.Identity `json:"stages"`
}

// Text renders native stage names one per line for CI sharding.
func (l stageList) Text() string {
	var text strings.Builder
	for _, identity := range l.Stages {
		text.WriteString(identity.Name)
		text.WriteByte('\n')
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

// runCurrent verifies the shared checkout in place.
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
	return verifyengine.RunMode(ctx, root, commit, certificateMode, runner)
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
	if report.Failure != nil && report.Code == 2 {
		leaction.ReportError(errors.New(report.Failure.Message))
	}
	return report, report.Code
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
