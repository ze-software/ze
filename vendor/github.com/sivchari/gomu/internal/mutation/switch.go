package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

const (
	switchMutatorName         = "switch"
	switchCaseBodyRemovalType = "switch_case_body_removal"

	switchDefaultHeader = "default"
)

// SwitchMutator empties the body of a switch case clause, testing whether tests
// detect a skipped branch in the switch's case selection.
//
// This covers case/default clauses of expression switches (tagged and tagless)
// and, because the AST walker gives no parent context, also clauses of type
// switches; a type-switch clause that leaves the switch variable unused becomes
// a compile error and is reported NOT_VIABLE rather than skipped. select
// statements use *ast.CommClause and are untouched. The tag expression is never
// re-evaluated by this mutation.
type SwitchMutator struct {
}

// Name returns the name of the mutator.
func (m *SwitchMutator) Name() string {
	return switchMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *SwitchMutator) Description() string {
	return "Remove statements in a switch case clause"
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *SwitchMutator) CanMutate(node ast.Node) bool {
	clause, ok := node.(*ast.CaseClause)

	return ok && len(clause.Body) > 0
}

// Mutate generates mutants for the given node.
func (m *SwitchMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	clause, ok := node.(*ast.CaseClause)
	if !ok || len(clause.Body) == 0 {
		return nil
	}

	pos := fset.Position(clause.Pos())
	original := caseClauseHeader(clause)

	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        switchCaseBodyRemovalType,
			Original:    original,
			Mutated:     statementRemovalMutated,
			Description: fmt.Sprintf("Remove statements in %q case", original),
		},
	}
}

// Apply applies the mutation to the given AST node.
func (m *SwitchMutator) Apply(node ast.Node, mutant Mutant) bool {
	if mutant.Type != switchCaseBodyRemovalType {
		return false
	}

	clause, ok := node.(*ast.CaseClause)
	if !ok || len(clause.Body) == 0 {
		return false
	}

	clause.Body = nil

	return true
}

// caseClauseHeader renders a case clause's header, e.g. "case nil", "case 1, 2",
// or "default".
func caseClauseHeader(clause *ast.CaseClause) string {
	if clause.List == nil {
		return switchDefaultHeader
	}

	exprs := make([]string, len(clause.List))
	for i, expr := range clause.List {
		exprs[i] = exprToString(expr)
	}

	return "case " + strings.Join(exprs, ", ")
}
