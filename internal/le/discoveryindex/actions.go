// Design: docs/architecture/core-design.md -- the discovery-index area, as one command
//
// actions.go carries the TABLE of what `le discovery-index` can do. Each action
// carries the reason that `--list` prints, and whether it WRITES.
//
// The `check` verb was deleted with the map's tracked copy on 2026-09-11. It
// re-rendered the whole map to compare it against the committed file, so the
// walk that decided the verdict was the walk that could have written the
// answer, and it is now that write.
//
// The dispatch, the listing, the help line and the two refusals live in
// internal/le/leaction. What stays here is the TABLE, because the table is the only
// part of an area that is about the package map.

package discoveryindex

import (
	"errors"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "discovery-index"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "update", Why: "regenerate ai/PACKAGE-MAP.md",
		Writes: true,
		Answer: runUpdate},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le discovery-index` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runUpdate locates the checkout and rewrites the map from it.
//
// Two failure codes leave here, and each names a different fact. A tree without
// ai/ answers 1, the generator failure code. An unreadable tree answers 2,
// which distinguishes an incomplete scan from a tree with nowhere to write.
//
// There is no third code for drift. The map is DERIVED and untracked, so no
// stored copy exists to disagree with the tree: a write hook removes it when an
// input moves and a read hook rebuilds it.
func runUpdate() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	report, err := Update(tree)
	if err != nil {
		leaction.ReportError(err)
		if errors.Is(err, ErrNoAIDir) {
			return nil, 1
		}
		return nil, 2
	}
	return report, 0
}
