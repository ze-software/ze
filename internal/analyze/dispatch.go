// Design: docs/architecture/system-architecture.md -- le mrt action-first dispatch

package analyze

import "github.com/ze-software/ze/internal/core/subdispatch"

var dispatcher = subdispatch.New("analyze", "BGP MRT analysis tools")

func Register(name string, handler func([]string) int, meta subdispatch.SubMeta) {
	dispatcher.Register(name, handler, meta)
}

func Dispatch(args []string) int { return dispatcher.Dispatch(args) }
func Subcommands() string        { return dispatcher.Subcommands() }

// Targets answers every registered subcommand with its description, sorted by name.
func Targets() []subdispatch.Target { return dispatcher.Targets() }

// helpAsked reports whether a subcommand's words are one help word alone. A
// subcommand that reads its words as MRT file names asks it first, so `le mrt
// statistics --help` prints the subcommand's usage instead of opening a file
// named `--help`. The four spellings are the ones le answers everywhere.
func helpAsked(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch args[0] {
	case "help", "--help", "-h", "-help":
		return true
	}
	return false
}

// usageExit is the code a subcommand's usage answers: 0 when the reader asked
// for it with a help word, 1 when the words it needs are missing.
func usageExit(args []string) int {
	if helpAsked(args) {
		return 0
	}
	return 1
}
