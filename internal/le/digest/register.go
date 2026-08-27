// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package digest

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

// name is the word this command is typed as.
const name = "digest"

func init() {
	leroot.Register(name, Answer, registry.Meta{
		Description: "every file:line anchor in ai/digests/*.md resolves to a real file and an in-range line",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
	})

	// The answer carries counts beside two row sets, so the any-shape operators
	// render the whole document rather than being refused.
	leroot.RegisterShape(name, command.ShapeMap)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.
	parity.Claim(name, "ze-digest-check")
}

// Answer is the `le digest` command. It takes no argument: the tree is the
// checkout and the rendering is a pipe operator.
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		return nil, leroot.RefuseArgument(name, args[0])
	}

	tree, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: digest: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	report, err := Check(tree)
	if err != nil {
		// 2 rather than 1: the gate cannot read the judged population. The
		// caller can distinguish this from a digest with a rotted anchor.
		fmt.Fprintf(os.Stderr, "error: digest: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	if len(report.Errors) > 0 {
		os.Stderr.WriteString(report.Diagnosis()) //nolint:errcheck // CLI output
		return report, 1
	}
	return report, 0
}
