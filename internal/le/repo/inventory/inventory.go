// Design: (none -- build tool)
//
// Detail: report.go -- the answer this file produces
//
// Package repoinventory is the `ze-inventory` gate: what ze is made of, counted.
// Plugins, address families, capabilities, YANG modules, RPCs and their .ci
// coverage, functional-test files, and Go package statistics.
//

package repoinventory

import (
	"context"
	"fmt"
	"os"

	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// generatedLayout is the timestamp the page carries. Minute resolution, UTC,
// as the page has always carried it.
const generatedLayout = "2006-01-02 15:04 UTC"

// codeAreas are the first-party source directories covered by Go statistics,
// YANG paths and RPC discovery, in the order the statistics table lists them.
var codeAreas = []string{"internal", "pkg", "cmd"}

// Collect reports one committed source population; no field consults the host
// registry or the mutable checkout.
func Collect(root string) (Inventory, error) {
	snapshot, err := ReadCommitted(context.Background(), root)
	if err != nil {
		return Inventory{}, err
	}
	return snapshot.collect()
}

// pluginDirOf answers the plugin directory a path sits under, and whether it
// sits under one at all.
func pluginDirOf(path string) (dir string, under bool) {
	_, after, found := strings.Cut(path, "/plugins/")
	if !found {
		return "", false
	}
	dir, _, _ = strings.Cut(after, "/")
	return dir, true
}

// yangSuffix is the extension a YANG document carries.
const yangSuffix = ".yang"

// rpcHasCoverage reports whether an RPC name appears in the .ci content, in any
// of the three spellings a test can use it in: the hyphenated wire name, the
// spaced CLI form, and the spaced form with a glob between the first two words.
func rpcHasCoverage(rpcName, ciContent string) bool {
	if strings.Contains(ciContent, rpcName) {
		return true
	}
	spaced := strings.ReplaceAll(rpcName, "-", " ")
	if strings.Contains(ciContent, spaced) {
		return true
	}
	first, rest, found := strings.Cut(spaced, " ")
	if !found {
		return false
	}
	var glob textbuf.Buffer
	glob.Str(first).Str(" * ").Str(rest)
	return strings.Contains(ciContent, glob.String())
}

// reportVerb is the one action: the inventory of the checkout.
const reportVerb = "report"

// actions is the whole command surface. The report takes no arguments: the
// tree is the checkout, and the rendering is the operator's to choose with a
// pipe operator (ai/rules/cli.md).
var actions = leaction.New("repo inventory", leaction.Action{
	Verb:    reportVerb,
	Default: true,
	Summary: "report what ze is made of",
	Why:     "what ze is made of: plugins, families, YANG modules, RPCs, tests and package sizes",
	Answer:  answerReport,
})

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le repo inventory` command. A bare line runs the report, as
// callers have always typed it (D-P3-bare in
// spec-le-cli-usability), so the table
// supplies the verb (leaction.Action.Default) and the table reads the rest of
// the line. A word the area declares travels as it stands, so `le repo
// inventory report` is the same action.
func Answer(args []string) (any, int) {
	return actions.Answer(args)
}

// answerReport collects the inventory of the checkout.
func answerReport() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}

	inv, err := Collect(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}
	return inv, 0
}

// Inventory answers a rendering of itself, so the bare command prints the page
// the gate has always printed.
var _ leroot.Prose = Inventory{}
