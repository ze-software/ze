// Design: docs/architecture/core-design.md -- the htmx-upgrade area, as one command
// Detail: tree.go -- the package scan and explanation verdict each action calls.
//
// The table carries the two existing Make targets unchanged. leaction derives
// check and report from those names and keeps their write markers, listing, help,
// and dispatch on one declaration.

package htmxupgrade

import (
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "htmx-upgrade"

var actions = leaction.New(area,
	leaction.Action{
		Gate: "ze-htmx-upgrade-check",
		Why:  "htmx's OWN scanner, vendored at third_party/web/htmx-upgrade-check.py, reports no htmx 4 issue that scripts/dev/htmx-upgrade-explained.txt does not account for. It builds a DOM, so it reads the inheritance carriers a text search cannot",
		Answer: func() (any, int) {
			return run(false)
		},
	},
	leaction.Action{
		Gate: "ze-htmx-upgrade-report",
		Why:  "print every htmx 4 upgrade issue, explained or not, and exit 0",
		Answer: func() (any, int) {
			return run(true)
		},
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le htmx-upgrade` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func run(reportAll bool) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		fmt.Fprintln(os.Stderr, err) //nolint:errcheck // CLI diagnostics cannot be recovered
		return nil, 1
	}
	return runAt(root, reportAll, os.Stderr)
}

func runAt(root string, reportAll bool, stderr io.Writer) (any, int) {
	var report Report
	var code int
	var err error
	if reportAll {
		report, code, err = reportTree(root, stderr)
	} else {
		report, code, err = checkTree(root, stderr)
	}
	if err != nil {
		fmt.Fprintln(stderr, err) //nolint:errcheck // CLI diagnostics cannot be recovered
		return nil, code
	}
	return report, code
}
