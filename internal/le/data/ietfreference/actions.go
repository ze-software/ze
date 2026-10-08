// Design: reference/README.md -- the deliberate refresh of the reference tree
//
// actions.go is the table. The dispatch, the listing, the help line and the
// refusals are internal/le/le/action, which every ported area shares.
//
// ONE action, and it writes. Like data asn-delegation and data
// ip-special-purpose, its input is the network rather than the tree, so it has
// no offline check twin and no aggregate generator runs it: the reference
// tree is refreshed deliberately.

package dataietfreference

import (
	"context"
	"time"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as.
const area = "data ietf-reference"

var actions = leaction.New(area,
	leaction.Action{
		Verb: "write",
		Why: "read the IETF datatracker for every working group under reference/ietf/, download the texts the tree" +
			" holds and lacks, and rewrite reference/ietf/INDEX.tsv. It reaches the network, never writes rfc/full" +
			" or rfc/drafts, and deletes nothing",
		Writes: true,
		Answer: runWrite,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le data ietf-reference` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runWrite() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := Refresh(context.Background(), root, time.Now(), Canonical())
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}
