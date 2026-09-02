// Design: docs/architecture/testing/tracked-build-gate.md -- the matrix area, as one command
//
// actions.go is the Python area, ported. The dispatch, the listing, the help
// line and the two refusals live in internal/le/leaction. What stays here is the
// TABLE, because the table is the only part of an area that is about
// type-checking the tree once per feature-tag combination.

package staticcheckfeaturematrix

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "staticcheck-feature-matrix"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb: "check",
		Why: "the tree type-checks in every feature-tag combination Ze can be built in, so a package compiled out of the default build is still judged; " +
			"part <index> of <count> judges one piece of the rows, and the pieces together judge every row",
		Parameters: []leaction.Parameter{
			{Keyword: "part", Value: "index"},
			{Keyword: "of", Value: "count"},
		},
		AnswerArgs: runCheck,
	},
	leaction.Action{
		// No Make target names this one: it is the script's --print-matrix, and
		// it exists so a reader can see WHICH combinations a run judges without
		// waiting for Staticcheck.
		Verb:   "rows",
		Why:    "the feature-tag combinations this run judges, derived from the feature manifest and narrowed to what the change set can move",
		Answer: runRows,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le staticcheck-feature-matrix` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runRows is the `le staticcheck-feature-matrix rows` action.
func runRows() (any, int) {
	matrix, notice, err := derive()
	if err != nil {
		reportFailure(err)
		return nil, 2
	}
	reportNotice(notice)
	return matrix, 0
}

// runCheck is the `le staticcheck-feature-matrix check` action.
//
// The three codes stay apart: 0 for a tree that type-checks in every judged
// combination, 1 for one that does not, and 2 for a matrix that could not be
// judged at all. A caller that reads them apart keeps reading them apart.
func runCheck(args leaction.Arguments) (any, int) {
	index, count, err := partFrom(args)
	if err != nil {
		reportFailure(err)
		return nil, 2
	}
	tree, err := lepath.Root()
	if err != nil {
		reportFailure(err)
		return nil, 2
	}
	matrix, notice, err := derive()
	if err != nil {
		reportFailure(err)
		return nil, 2
	}
	reportNotice(notice)

	rows, err := matrix.Part(index, count)
	if err != nil {
		reportFailure(err)
		return nil, 2
	}
	if len(rows) == 0 {
		// Fewer rows than pieces, which a scoped run reaches: this piece has
		// nothing of its own, and every row it might have held is judged by a
		// sibling piece of the same run.
		return Verdict{Part: index, Parts: count, Passed: true}, 0
	}

	deadline, err := Deadline(len(rows))
	if err != nil {
		reportFailure(err)
		return nil, 2
	}

	verdict, judged, err := Judge(tree, rows, deadline)
	verdict.Part, verdict.Parts = index, count
	if err != nil {
		reportFailure(err)
		if verdict.Tool != "" {
			fmt.Fprint(os.Stderr, verdict.Tool) //nolint:errcheck // CLI output
		}
		return nil, 2
	}
	if verdict.Tool != "" {
		fmt.Fprint(os.Stderr, verdict.Tool) //nolint:errcheck // CLI output
	}
	if !judged || !verdict.Passed {
		return verdict, 1
	}
	return verdict, 0
}

// partFrom answers the piece of the matrix this invocation judges.
//
// An invocation that names neither keyword judges part 1 of 1, which is every
// row: the whole matrix IS one piece, so no number here stands for "the caller
// said nothing". Naming one keyword without the other is refused, because
// "part 3" alone cannot say how many pieces the rows were dealt into, and a
// guess would silently judge the wrong subset.
func partFrom(args leaction.Arguments) (int, int, error) {
	declaredIndex, hasIndex := args["part"]
	declaredCount, hasCount := args["of"]
	if !hasIndex && !hasCount {
		return 1, 1, nil
	}
	if hasIndex != hasCount {
		return 0, 0, fmt.Errorf(
			"a cut run needs both keywords: check part <index> of <count>; matrix could not be judged")
	}
	index, err := wholeNumber("part", declaredIndex)
	if err != nil {
		return 0, 0, err
	}
	count, err := wholeNumber("of", declaredCount)
	if err != nil {
		return 0, 0, err
	}
	return index, count, nil
}

// wholeNumber reads one keyword's value as a counting number.
func wholeNumber(keyword, declared string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(declared))
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s needs a whole number of 1 or more, got %q; matrix could not be judged",
			keyword, declared)
	}
	return value, nil
}

// derive resolves the checkout and answers the rows this run judges.
func derive() (Matrix, Notice, error) {
	tree, err := lepath.Root()
	if err != nil {
		return nil, Notice{}, err
	}
	return Derive(tree)
}

// reportNotice writes what the run says about its own scope, which is a fact
// about the RUN rather than part of any answer.
func reportNotice(notice Notice) {
	if text := notice.Text(); text != "" {
		fmt.Fprint(os.Stderr, text) //nolint:errcheck // CLI output
	}
}

// reportFailure states a matrix that could not be judged, in the one form every
// caller of this check reads.
func reportFailure(err error) { leaction.ReportError(err) }
