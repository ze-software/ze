// Design: docs/architecture/cli/plugin-modes.md -- offline flag completion
// Overview: main.go -- completion dispatch
// Related: words.go -- command-word completion (flags/families complete the same surface)

package completion

import (
	"io"
	"os"
	"strings"

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
	args, pending := flagContext(args)
	if pending != registry.FlagValueNone {
		return 0
	}
	var key textbuf.Buffer
	path := key.Join(args, " ").String()
	for _, f := range registry.CommandFlags(path) {
		if err := writeCompletionRecord(w, f.Name, f.Description); err != nil {
			return 1
		}
	}
	return 0
}

// flagContext removes registered flags and their values from completed words.
// Its second result is the hint for a value that has not been supplied yet;
// None means the next word belongs to the command path. Unknown flag tokens
// are excluded, without guessing whether they consume a value.
func flagContext(args []string) ([]string, string) {
	path := make([]string, 0, len(args))
	pending := registry.FlagValueNone
	for _, word := range args {
		if pending != registry.FlagValueNone {
			pending = registry.FlagValueNone
			continue
		}
		if strings.HasPrefix(word, "-") {
			name, _, attached := strings.Cut(word, "=")
			if !attached {
				pending = flagValueHint(path, name)
			}
			continue
		}
		path = append(path, word)
	}
	return path, pending
}

func flagValueHint(path []string, name string) string {
	var key textbuf.Buffer
	for _, flag := range registry.CommandFlags(key.Join(path, " ").String()) {
		if flag.Name == name {
			return flag.ValueHint
		}
	}
	return registry.FlagValueNone
}

// flagValue answers which registered value the current token completes. The
// last argument is the current token (including an empty one); earlier words
// are complete. Shells ask before offering command words or flag names.
func flagValue(args []string) int {
	if len(args) == 0 {
		return 0
	}
	path, hint := flagContext(args[:len(args)-1])
	if hint == registry.FlagValueNone {
		name, _, attached := strings.Cut(args[len(args)-1], "=")
		if attached {
			hint = flagValueHint(path, name)
		}
	}
	if hint == registry.FlagValueNone {
		return 0
	}
	if err := writeCompletionRecord(os.Stdout, hint, ""); err != nil {
		return 1
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
