// Design: docs/contributing/running-commands.md -- what le's exit codes mean
// Related: helpai.go -- `le help ai exit-codes` prints this table
//
// The exit-code table, declared once, so `le help ai exit-codes` reads it
// rather than restating it. It states the codes le answers today: the
// parser's refusals answer 2, and an action's body answers its own verdict.

package leroot

import "github.com/ze-software/ze/internal/component/aihelp/reference"

// exitCodes answers the table, a fresh copy for each reader.
func exitCodes() []reference.ExitCode {
	return []reference.ExitCode{
		{Code: 0, Meaning: "the action did what it names, or a help question was answered"},
		{Code: 1, Meaning: "the action ran and found a problem, or could not run"},
		{Code: 2, Meaning: "usage error: an unknown action, a stray or dash-leading word, a missing value, a refused rendering flag",
			Exempt: []string{
				"le ai hooks <hook>: the hook protocol, 0 allows and 2 blocks",
				"a pass-through command: its program's exit code is the command's",
			}},
	}
}
