// Design: docs/architecture/core-design.md -- the platform-vet area, as one command
// Overview: platformvet.go -- the shared runner for both cross-target actions
//
// actions.go owns the two gate rows and their sweep behavior. The dispatch,
// listing, help line, and refusal text come from internal/le/leaction.
package platformvet

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "platform-vet"

type platformRun func(Platform) (any, int)

// actionTable derives both actions from the platform specifications. A sweep
// resolves every action name before it calls run.
func actionTable(run platformRun) leaction.Area {
	rows := make([]leaction.Action, 0, len(platformSpecs))
	for _, spec := range platformSpecs {
		platform := spec.platform
		rows = append(rows, leaction.Action{
			Gate: spec.gate,
			Why:  spec.why,
			Answer: func() (any, int) {
				return run(platform)
			},
		})
	}
	return leaction.New(area, rows...)
}

// metadataOnly builds the table for listing, help, and parity claims. Its
// actions are never called, so these paths do not read the checkout.
func metadataOnly() leaction.Area {
	return actionTable(func(Platform) (any, int) { return nil, 0 })
}

// Actions answers the two-action command surface as data.
func Actions() leaction.List { return metadataOnly().Actions() }

// Gates answers both claimed gate names from the action table.
func Gates() []string { return metadataOnly().Gates() }

// Subs is the one-line hint that help renders under the command.
func Subs() string { return metadataOnly().Subs() }

// Answer is the `le platform-vet` command. A bare command lists the two
// platforms. Named actions form one sweep, and the first failing code wins.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return Actions(), 0
	}

	var runner Runner
	var runnerErr error
	loaded := false
	run := func(platform Platform) (any, int) {
		if !loaded {
			loaded = true
			root, err := lepath.Root()
			if err != nil {
				runnerErr = err
			} else {
				runner, runnerErr = NewRunner(root)
			}
		}
		if runnerErr != nil {
			report := failedReport(platform, runnerErr)
			leaction.ReportError(runnerErr)
			return report, report.Code
		}
		return runner.Run(platform)
	}

	return actionTable(run).Sweep(args, leaction.RunEveryAction)
}
