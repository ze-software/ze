// Design: docs/architecture/diagnostics/production-diagnostics.md -- show system drift handler
// Related: show.go -- ze-cmd:show-system-drift registration and pipe filter declaration
// Related: op_pipe.go -- shared single-text pipe filter extractor

package show

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/drift"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	pipeOpSubsystem  = "subsystem"
	driftPipeExample = "ze show system drift | subsystem <name>"
	keyDriftedCount  = "drifted-count"
)

// subsystemPipeFilter declares the command-owned `| subsystem <name>` pipe
// operation on `show system drift`.
var subsystemPipeFilter = command.PipeFilter{
	Name:        pipeOpSubsystem,
	Description: "Filter drift check to one registered subsystem",
	Argument:    command.ArgText,
}

func extractSubsystemPipeTarget(validated command.ValidatedArgs) (name string, present bool, refusal *plugin.Response) {
	return extractSingleTextPipeTarget(
		validated,
		pipeOpSubsystem,
		"duplicate | subsystem pipe operation",
		"only one | subsystem filter may be given on a command line",
		"| subsystem requires a subsystem name",
		"a registered subsystem name must follow | subsystem",
		"| subsystem takes exactly one subsystem name",
		"only one subsystem name may follow | subsystem",
		driftPipeExample,
	)
}

// handleShowSystemDrift runs read-only dataplane drift checks across every
// registered subsystem, or one subsystem when narrowed with `| subsystem <name>`.
func handleShowSystemDrift(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	subsystem, hasSubsystem, refusal := extractSubsystemPipeTarget(validated)
	if refusal != nil {
		return refusal, nil
	}

	reqCtx := ctx.Context()
	if hasSubsystem {
		rep, ok := drift.Check(reqCtx, subsystem)
		if !ok {
			registered := drift.Subsystems()
			registeredList := "none"
			if len(registered) > 0 {
				registeredList = textbuf.Join(registered, ", ")
			}
			return plugin.OperatorErrorResponse(opererr.New(
				"unknown drift subsystem: "+subsystem,
				"Only registered dataplane subsystems can be checked for drift.",
				"run 'ze show system drift' to list registered subsystems (registered: "+registeredList+")",
			)), nil
		}
		driftedCount := 0
		if rep.Status == drift.StatusDrifted {
			driftedCount = 1
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: plugin.Map{
				triageKeyStatus: string(rep.Status),
				keySubsystems:   []drift.Report{rep},
				keyCount:        1,
				keyDriftedCount: driftedCount,
			},
		}, nil
	}

	reports := drift.CheckAll(reqCtx)
	if reports == nil {
		reports = []drift.Report{}
	}
	status := drift.StatusClean
	driftedCount := 0
	for i := range reports {
		if reports[i].Status == drift.StatusDrifted {
			driftedCount++
		}
		if reports[i].Status == drift.StatusError {
			status = drift.StatusError
		}
	}
	if status != drift.StatusError {
		if driftedCount > 0 {
			status = drift.StatusDrifted
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			triageKeyStatus: string(status),
			keySubsystems:   reports,
			keyCount:        len(reports),
			keyDriftedCount: driftedCount,
		},
	}, nil
}
