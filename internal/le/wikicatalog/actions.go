// Design: docs/architecture/core-design.md -- one closed le action area
//
// The action table is the whole command grammar. A destination is accepted only
// after the `file` keyword, so an arbitrary path can never occupy an action or
// keyword position.
package wikicatalog

import "github.com/ze-software/ze/internal/le/leaction"

const area = "wiki-catalog"

var actions = leaction.New(area,
	leaction.Action{
		Verb: "check",
		Why:  "compare a command-catalog Markdown file with the live product command registries",
		Parameters: []leaction.Parameter{
			{Keyword: "file", Value: "destination"},
		},
		AnswerArgs: runCheck,
	},
	leaction.Action{
		Verb:   "update",
		Why:    "write the live product command catalog as exact Markdown",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "file", Value: "destination"},
		},
		AnswerArgs: runUpdate,
	},
)

// Actions answers the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line action hint used by le help.
func Subs() string { return actions.Subs() }

// Answer is the `le wiki-catalog` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runCheck(arguments leaction.Arguments) (any, int) {
	return run(arguments, Check)
}

func runUpdate(arguments leaction.Arguments) (any, int) {
	return run(arguments, Update)
}

func run(arguments leaction.Arguments, judge func(string) (Report, error)) (any, int) {
	destination, err := catalogDestination(arguments)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := judge(destination)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if report.Stale && !report.Written {
		return report, StaleExit
	}
	return report, 0
}

func catalogDestination(arguments leaction.Arguments) (string, error) {
	path, ok := arguments["file"]
	if !ok {
		return "", errDestinationRequired
	}
	return validateDestination(path)
}
