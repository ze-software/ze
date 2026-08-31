// Design: (none -- shell completion flag/value inventory)
// Overview: main.go -- completion dispatch
// Related: words.go -- command-word completion (flags/families complete the same surface)

package completion

import (
	"io"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// flags emits "flagname\tdescription" for every flag registered at a command
// path, e.g. `ze completion flags exabgp plugin` -> "--family\t...". The shell
// generators call it to complete flag names without hardcoding per-subcommand
// lists.
func flags(args []string) int {
	return writeFlags(os.Stdout, args)
}

func writeFlags(w io.Writer, args []string) int {
	var key textbuf.Buffer
	path := key.Join(args, " ").String()
	for _, f := range registry.CommandFlags(path) {
		if err := writeCompletionRecord(w, f.Name, f.Description); err != nil {
			return 1
		}
	}
	return 0
}

// families emits "family\tsource" for every registered address family so a
// shell can complete `--family <TAB>`. It draws from command.FamilyValueHints
// (which reads registry.AllFamilies) to keep a single source of truth.
func families() int {
	return writeFamilies(os.Stdout)
}

func writeFamilies(w io.Writer) int {
	for _, s := range command.FamilyValueHints() {
		if err := writeCompletionRecord(w, s.Text, s.Description); err != nil {
			return 1
		}
	}
	return 0
}
