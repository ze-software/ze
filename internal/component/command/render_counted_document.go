// Design: docs/architecture/api/ipc_protocol.md -- counted document rendering.
// Related: pipe.go -- the canonical document pipe plan and operators.
package command

import (
	"errors"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RenderCountedDocument renders a completed document through a remote pipe
// chain. Presentation preserves its original counts. A selection or count
// operator reports the count its actual output earned before final presentation.
// Original refused records remain refused. The caller MUST retain the
// producer's failure and warning beside this result. No transport I/O happens
// here, so a pipe refusal remains distinct from a later failed write.
func RenderCountedDocument(input, sessionFormat, document string, count, faults uint64) (string, RecordAnswer, error) {
	plan, problem := prepareDocumentPipes(input, sessionFormat, false)
	if problem != "" {
		return "", RecordAnswer{}, errors.New(problem)
	}
	last := countedDocumentLastSelection(plan.ops)
	var rendered string
	if last < 0 {
		rendered, problem = ApplyPipes(document, plan.ops, plan.meta, plan.columns)
	} else {
		// Each selection runs once on the original document. Count its actual
		// result before a later format adds table lines or wrapper metadata.
		// Both segments retain the whole chain's field ordering.
		request := columnsInChain(plan.ops)
		rendered, problem = applyPipesWithRequest(document, plan.ops[:last+1], nil, plan.columns, request)
		if problem != "" {
			return "", RecordAnswer{}, errors.New(problem)
		}
		count = renderedRecordCount(rendered, false)
		meta := plan.meta
		if !chainInjectsMetadata(plan.ops, meta) {
			meta = nil
		}
		rendered, problem = applyPipesWithRequest(rendered, plan.ops[last+1:], meta, plan.columns, request)
	}
	if problem != "" {
		return "", RecordAnswer{}, errors.New(problem)
	}
	if count > ^uint64(0)-faults {
		return "", RecordAnswer{}, opererr.New("Cannot render the command answer",
			"Its selected records and retained refused records exceed the answer counter limit", "ze show errors")
	}
	return rendered, RecordAnswer{Type: rpc.AnswerTypeDocument, Count: count, Faults: faults}, nil
}

// Only these operators change the number of records an answer holds. Aliases
// and command-owned filters have already been resolved by the pipe parser.
func countedDocumentLastSelection(ops []pipeOp) int {
	last := -1
	for index, op := range ops {
		// This predicate selects cardinality-changing operators, not formats.
		//exhaustive:ignore
		switch op.kind {
		case pipeMatch, pipeCount, pipeFirst, pipeLast:
			last = index
		}
	}
	return last
}

// documentPipePlan is the canonical preparation shared by document formatters.
// Its operators and metadata are read-only after preparation.
type documentPipePlan struct {
	command string
	ops     []pipeOp
	meta    pipeChainMeta
	columns []ColumnOrder
}

func prepareDocumentPipes(input, sessionFormat string, saveAllowed bool) (documentPipePlan, string) {
	var plan documentPipePlan
	var err error
	plan.command, plan.ops, err = parseExecutionPipeChain(input)
	if err != nil {
		return plan, err.Error()
	}
	plan.columns = ColumnsForCommand(plan.command)
	plan.command, plan.ops, plan.meta = foldFilters(plan.command, plan.ops)
	if problem := validatePipesForSurface(plan.command, plan.ops, pipeSurfaceOneShot, saveAllowed); problem != "" {
		return plan, problem
	}
	if !hasFormatOp(plan.ops) {
		plan.ops = append(plan.ops, pipeOp{kind: defaultFormatFor(plan.command, sessionFormat)})
	}
	return plan, ""
}
