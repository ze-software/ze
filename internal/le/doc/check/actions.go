// Design: docs/architecture/core-design.md -- native documentation verifier actions
// Detail: links.go -- path, citation, suppression, and baseline checks.
// Detail: register.go -- root registration and answer shape.
//
// Package doccheck owns the three documentation actions that the verifier runs
// directly. The links action owns its scan. Verify and templ-output call the Go
// packages that already own those compositions.

package doccheck

import (
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/doc/wiring"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "doc check"

type action struct {
	verb string
	why  string
	run  func(string) (any, int)
}

var actions = [...]action{
	{
		verb: "links",
		why:  "instruction links, tracked citations, suppression reasons, and Design references resolve",
		run:  runLinks,
	},
	{
		verb: "verify",
		why:  "the complete ordered documentation gate passes",
		run:  docwiring.Verify,
	},
	{
		verb: "templ-output",
		why:  "generated templ output matches its sources",
		run:  docwiring.TemplOutput,
	},
}

// ActionRow is one callable documentation action.
type ActionRow struct {
	Verb string `json:"verb"`
	Why  string `json:"why"`
}

// ActionList is the bare command's answer.
type ActionList struct {
	Area    string      `json:"area"`
	Actions []ActionRow `json:"actions"`
}

// Actions returns the whole command surface from its canonical table.
func Actions() ActionList {
	list := ActionList{Area: area, Actions: make([]ActionRow, 0, len(actions))}
	for _, one := range actions {
		list.Actions = append(list.Actions, ActionRow{Verb: one.verb, Why: one.why})
	}
	return list
}

// Text renders the action list for a person.
func (l ActionList) Text() string {
	var out textbuf.Buffer
	out.Str(l.Area).Str(":\n")
	width := 0
	for _, row := range l.Actions {
		width = max(width, len(row.Verb))
	}
	for _, row := range l.Actions {
		out.Str("  ").PadRight(row.Verb, width).Str("  checks  ").Str(row.Why).Byte('\n')
	}
	return out.String()
}

// Subs is the one-line action hint shown by help.
func Subs() string {
	verbs := make([]string, 0, len(actions))
	for _, one := range actions {
		verbs = append(verbs, one.verb)
	}
	return strings.Join(verbs, " | ")
}

// Answer is the `le doc-check` command.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return Actions(), 0
	}
	for _, one := range actions {
		if args[0] != one.verb {
			continue
		}
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "error: %s takes no value: %s\n", one.verb, args[1]) //nolint:errcheck // CLI output
			return nil, 2
		}
		root, err := lepath.Root()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err) //nolint:errcheck // CLI output
			return nil, 2
		}
		return one.run(root)
	}
	fmt.Fprintf(os.Stderr, "error: no such action in %s: %s\n", area, args[0]) //nolint:errcheck // CLI output
	fmt.Fprintln(os.Stderr, "try one of:", Subs())                             //nolint:errcheck // CLI output
	return nil, 2
}

func runLinks(root string) (any, int) {
	report, err := checkLinks(root)
	if err != nil {
		return errorReport{Error: err.Error()}, 2
	}
	if len(report.Errors) > 0 {
		return report, 1
	}
	return report, 0
}

// errorReport is an operational failure that prevented a complete judgement.
type errorReport struct {
	Error string `json:"error"`
}

// Text renders the failure for a person.
func (r errorReport) Text() string {
	var out textbuf.Buffer
	return out.Str("error: ").Str(r.Error).Byte('\n').String()
}
