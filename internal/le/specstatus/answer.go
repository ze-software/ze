// Design: docs/architecture/core-design.md -- the spec inventory as a command

package specstatus

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// Answer is the command. The tree is the checkout and the rendering is a pipe
// operator, so the inventory takes no argument: the script's `--json` flag is
// `| json` here, which every ported tool answers the same way
// (ai/rules/cli.md).
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		return nil, leroot.RefuseArgument("spec-status", args[0])
	}

	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}

	inventory, err := Collect(context.Background(), root, time.Now(), warnToStderr)
	if err != nil {
		reportError(err)
		return nil, 1
	}
	// The inventory REPORTS; it does not gate. A tree full of blocked specs is
	// not a failure, so the code is 0 whenever the population was read.
	return inventory, 0
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
