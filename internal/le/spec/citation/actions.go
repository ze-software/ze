// Design: docs/architecture/core-design.md -- le's native development gates

package speccitation

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/le/lepath"
)

const name = "spec citation"

// Answer is the `le spec citation` command. Its existing bare form runs the
// citation gate. `anchors spec <path>` runs the design-document owner audit.
func Answer(args []string) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	if len(args) == 0 {
		report, err := Scan(root)
		if err != nil {
			reportError(err)
			return nil, 1
		}
		return report, verdict(report)
	}
	if len(args) != 3 {
		return nil, refuseCitation(firstCitationArgument(args))
	}
	if args[0] != "anchors" {
		return nil, refuseCitation(args[0])
	}
	if args[1] != "spec" {
		return nil, refuseCitation(args[1])
	}
	report, err := AuditAnchors(root, args[2])
	if err != nil {
		reportError(err)
		return nil, 2
	}
	if len(report.Owners) > 0 {
		return report, 1
	}
	return report, 0
}

func reportError(err error) {
	fmt.Fprintf(os.Stderr, "error: spec-citation: %v\n", err) //nolint:errcheck // CLI output
}

func firstCitationArgument(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func refuseCitation(got string) int {
	fmt.Fprintf(os.Stderr, "usage: le spec citation anchors spec <bucket>/spec-<name>.md (got %q)\n", got) //nolint:errcheck // CLI output
	return 2
}
