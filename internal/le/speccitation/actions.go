// Design: docs/architecture/core-design.md -- le's native development gates

package speccitation

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

const name = "spec-citation"

// Answer is the `le spec-citation` gate. Repository selection belongs to
// lepath, so the command takes no positional arguments or private path flags.
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		return nil, leroot.RefuseArgument(name, args[0])
	}
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	report, err := Scan(root)
	if err != nil {
		reportError(err)
		return nil, 1
	}
	return report, verdict(report)
}

func reportError(err error) {
	fmt.Fprintf(os.Stderr, "error: spec-citation: %v\n", err) //nolint:errcheck // CLI output
}
