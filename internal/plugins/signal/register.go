// Register the signal + status root commands with the command registry.

package signal

import (
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// subcommands returns the signal verbs as the comma-separated Subs hint,
// derived from the commands table so help and shell completion name every
// verb Run dispatches.
func subcommands() string {
	names := make([]string, len(commands))
	for i, cmd := range commands {
		names[i] = cmd.Name
	}
	return textbuf.Join(names, ", ")
}

func init() {
	registry.MustRegisterRootHandler("signal", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		ShortHelp: "Send signals to the daemon via SSH",
		Mode:      "daemon",
		Section:   registry.SectionSystem,
		Subs:      subcommands(),
	})
	// `ze status` takes no sub-verb: its answer is the exit code, which
	// `ze status --help` documents in its "Exit codes" section (main.go).
	// Subs stays empty so completion offers no word.
	registry.MustRegisterRootHandler("status", func(_ *registry.RuntimeContext, args []string) int {
		return RunStatus(args)
	}, registry.Meta{
		ShortHelp:   "Check if daemon is running",
		Description: "Exit 0 when the daemon is running, 1 when it is not.",
		Mode:        "daemon",
		Section:     registry.SectionSystem,
	})
}
