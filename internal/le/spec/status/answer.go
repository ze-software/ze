// Design: docs/architecture/core-design.md -- the spec inventory as a command

package specstatus

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/le/lepath"
)

type closureAction uint8

const (
	closureUnspecified closureAction = iota
	closureList
	closureCheck
)

// Answer is the command. Its bare form is the inventory. The closure actions
// preserve the detector's list and single-spec contracts.
func Answer(args []string) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	if len(args) == 0 {
		inventory, err := Collect(context.Background(), root, time.Now(), warnToStderr)
		if err != nil {
			reportError(err)
			return nil, 1
		}
		// The inventory REPORTS; it does not gate. A tree full of blocked specs is
		// not a failure, so the code is 0 whenever the population was read.
		return inventory, 0
	}
	action, spec := parseClosureAction(args)
	switch action {
	case closureList:
		report, malformed, err := closureInventory(root)
		reportMalformedJournals(malformed)
		if err != nil {
			reportError(err)
			return nil, 2
		}
		return report, 0
	case closureCheck:
		report, malformed, err := CheckClosure(root, spec)
		reportMalformedJournals(malformed)
		if err != nil {
			reportError(err)
			return nil, 2
		}
		if report.Blocked() {
			return report, 3
		}
		return report, 0
	default:
		return nil, refuseStatus(args[0])
	}
}

func parseClosureAction(args []string) (closureAction, string) {
	if len(args) == 2 {
		if args[0] == "closure" {
			if args[1] == "list" {
				return closureList, ""
			}
		}
		return closureUnspecified, ""
	}
	if len(args) == 4 {
		if args[0] == "closure" {
			if args[1] == "check" {
				if args[2] == "spec" {
					return closureCheck, args[3]
				}
			}
		}
	}
	return closureUnspecified, ""
}

func reportMalformedJournals(paths []string) {
	for _, path := range paths {
		fmt.Fprintf(os.Stderr, "warning: malformed journal row in %s (run `./le journal report`)\n", path) //nolint:errcheck // CLI output
	}
}

// warnToStderr names a spec the inventory could not parse. It is the answer's
// side channel rather than part of the payload: the record for such a spec is
// still published, carrying the status `unparsed`.
func warnToStderr(line string) {
	fmt.Fprintln(os.Stderr, line) //nolint:errcheck // CLI output
}

// reportError writes one failure line in the spelling every ported le tool uses.
func reportError(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
}

func refuseStatus(got string) int {
	fmt.Fprintf(os.Stderr, "usage: le spec status [closure list|closure check spec <name>] (got %q)\n", got) //nolint:errcheck // CLI output
	return 2
}
