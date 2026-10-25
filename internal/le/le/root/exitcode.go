// Design: docs/contributing/running-commands.md -- what le's exit codes mean
// Related: helpai.go -- `le help ai exit-codes` prints this table
//
// The exit-code table, declared once, so `le help ai exit-codes` reads it
// rather than restating it. Every le command answers one of these four codes,
// except the declared exemptions, which answer a protocol or another
// program's code.

package leroot

import "github.com/ze-software/ze/internal/component/aihelp/reference"

// exitCodes answers the table, a fresh copy for each reader.
func exitCodes() []reference.ExitCode {
	return []reference.ExitCode{
		{Code: 0, Meaning: "the action did what it names, or a help question was answered"},
		{Code: 1, Meaning: "the action ran and found a problem, or the operation it ran failed"},
		{Code: 2, Meaning: "usage error: an unknown command or action, a word a namespace does not hold, " +
			"a stray or dash-leading word, a missing or malformed value, a retired spelling, a refused rendering flag",
			Exempt: []string{
				"le ai hooks <hook>: the hook protocol, 0 allows and 2 blocks",
				"a forwarding command (test fixture, test peer, every mock area, every pass-through): " +
					"its program's exit code is the command's",
			}},
		{Code: 3, Meaning: "could not run: the checkout, a file or a program the action needs is absent or unreadable, " +
			"or a | save could not write its file"},
	}
}
