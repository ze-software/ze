// Design: docs/architecture/api/commands.md — log command registration

package cmd

import (
	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(RPCs()...)
	command.RegisterPipeFilters(
		[]string{"show log recent"},
		command.PipeFilter{
			Name:        pipeOpPlugin,
			Description: "Inspect recent logs from a running plugin process by endpoint name instead of the daemon",
			Argument:    command.ArgText,
		},
		command.PipeFilter{
			Name:        pipeOpOperation,
			Description: "Filter entries by operation correlation ID",
			Argument:    command.ArgText,
		},
	)
}
