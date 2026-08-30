// Design: docs/architecture/api/commands.md — tacacs command ownership
// Related: main.go — dataServers, the probe this serves
//
// Register the `tacacs` root command with the importable command registry.
// This is the owner package: the offline TACACS+ client CLI lives with
// internal/component/tacacs, not under cmd/ze.
//
// The probe answers with DATA, so it is registered through
// MustRegisterLocalData and `| json`, `| yaml` and `| table` are three
// renderings of one payload. It printed a table and a second, hand-written
// JSON rendering behind a `--json` flag before, which reached the pipe layer
// on no surface: no daemon serves a TACACS+ wire method, so
// `ze cli -c "show tacacs servers x.conf | json"` answered `unknown command`.
package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// modeOffline is the help tag for a command that answers with no running
// daemon. The probe reads a config file and dials from this process, so it is
// one.
const modeOffline = "offline"

// The command path is written as a literal at each call, not through a const.
// `./le docvalid command-contract` and the local-data coverage scan parse this
// file to bind a YANG command to its handler, and both read a string literal;
// a const identifier reaches them as no path at all.
func init() {
	registry.MustRegisterRootHandler("tacacs", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "TACACS+ client helpers",
		Mode:        modeOffline,
		Section:     registry.SectionConfiguration,
		Subs:        "show",
	})

	registry.MustRegisterLocalData("show tacacs servers", dataServers, registry.Meta{
		Description: "Probe every TACACS+ server a config names and report whether it answers.",
		Mode:        modeOffline,
	}, command.RenderLocalAnswer)

	// The answer is rows read against declared column names, so every row
	// operator applies and the published page can say so before the command
	// runs.
	command.RegisterShape([]string{"show tacacs servers"}, command.ShapeTab)
	command.RegisterColumns([]string{"show tacacs servers"},
		command.ColumnOrder{"address", "port", "reachable", "rtt", "error"},
	)
}
