// Design: docs/architecture/core-design.md -- the iana-asn area, as one command
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are letools/leaction, which every ported area shares.
//
// ONE action, and it writes. This generator's input is the network rather than
// the tree, so there is nothing for a check twin to compare a checkout against
// without asking five registries what they publish today. That is why
// ze-generated-files-check does not list it and why `make generate` does not run
// it: the seed table is refreshed deliberately, not on every build.

package ianaasn

import (
	"context"

	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
)

// area is the name this command is typed as.
const area = "iana-asn"

var actions = leaction.New(area,
	leaction.Action{
		Verb: "write",
		Why: "fetch the five RIR delegation files and rewrite the compiled ASN-to-RIR seed table." +
			" It reaches the network, so it is the one generator an offline checkout cannot run",
		Writes: true,
		Answer: runWriteHere,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le iana-asn` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runWriteHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runWrite(context.Background(), root, nil)
}

// runWrite answers the write over one tree, through one fetch. A nil fetch is
// the HTTP one.
func runWrite(ctx context.Context, root string, fetch Fetch) (any, int) {
	report, err := Write(ctx, root, fetch)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}
