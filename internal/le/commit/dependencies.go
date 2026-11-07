// Design: docs/contributing/dependency-approval.md -- the commit gate on third-party imports
// Related: prepare.go -- Create, which calls this gate after the spec mechanisms gate
// Related: review.go -- workingTreeSides, the two sides this gate reads
// Related: ../deps/deps.go -- the ledger, the import reader and the approval record

package commit

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/hunks"
	"github.com/ze-software/ze/internal/le/deps"
)

// dependencyRefusal refuses a commit that adds a third-party import the
// ledger does not approve for the importing directory, a ledger row no
// `deps approve` recorded, or the ledger's removal. It judges only what the
// commit adds: an import a file already held, or one it drops, asks nothing,
// so the gate reads the carried Go files and the ledger and nothing else.
//
// The ledger is read as the commit leaves it: the carried blob when the
// commit carries the ledger, HEAD's otherwise. Reading the working tree
// instead would admit an import whose row lands in no commit.
func dependencyRefusal(root string, paths, removed []string, selection hunks.Selection, head string) error {
	if slices.Contains(removed, deps.LedgerRel) {
		return errors.New(deps.LedgerRel + " cannot be removed: with no ledger every third-party import would go unjudged; " +
			"narrow it by deleting rows instead (docs/contributing/dependency-approval.md)")
	}
	sides := workingTreeSides(root, head, selection)
	module, err := dependencyModule(sides)
	if err != nil {
		return couldNotRun(err)
	}
	ledger, problems, err := carriedLedger(root, paths, sides)
	if err != nil {
		return err
	}
	vendorModules, _, problem := sides.after(deps.VendorModulesRel)
	if problem != "" {
		return couldNotRun(errors.New("judge third-party imports: " + problem))
	}
	for _, path := range paths {
		judged, err := deps.Judged(root, path)
		if err != nil {
			return couldNotRun(errors.New("judge third-party imports of " + path + ": " + err.Error()))
		}
		if !judged {
			continue
		}
		added, err := addedImports(module, path, sides)
		if err != nil {
			return err
		}
		importer := deps.Importer(path)
		for _, pkg := range added {
			if ledger.Allows(pkg, importer) {
				continue
			}
			problems = append(problems, unapprovedImport(path, pkg, importer, deps.ModuleOf(vendorModules, pkg)))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "\n\n"))
}

// dependencyModule reads the module path from go.mod as the commit leaves it.
func dependencyModule(sides specSides) (string, error) {
	goMod, present, problem := sides.after("go.mod")
	if problem != "" {
		return "", errors.New("judge third-party imports: " + problem)
	}
	if !present {
		return "", errors.New("judge third-party imports: the checkout holds no go.mod")
	}
	return deps.ModulePath(goMod)
}

// carriedLedger answers the ledger the commit leaves and the refusal of every
// row the commit adds that the approval record does not hold. A ledger HEAD
// does not hold is the seed: its rows are the tree's existing imports, so the
// commit that creates it owes no record, and it is created once, because its
// removal is refused.
func carriedLedger(root string, paths []string, sides specSides) (deps.Ledger, []string, error) {
	before, beforePresent, problem := sides.before(deps.LedgerRel)
	if problem != "" {
		return deps.Ledger{}, nil, couldNotRun(errors.New("judge the dependency ledger: " + problem))
	}
	if !slices.Contains(paths, deps.LedgerRel) {
		ledger, err := deps.Parse(before)
		if err != nil {
			return deps.Ledger{}, nil, couldNotRun(errors.New("HEAD's " + err.Error()))
		}
		return ledger, nil, nil
	}
	after, _, problem := sides.after(deps.LedgerRel)
	if problem != "" {
		return deps.Ledger{}, nil, couldNotRun(errors.New("judge the dependency ledger: " + problem))
	}
	ledger, err := deps.Parse(after)
	if err != nil {
		return deps.Ledger{}, nil, err
	}
	if !beforePresent {
		return ledger, nil, nil
	}
	headLedger, err := deps.Parse(before)
	if err != nil {
		return deps.Ledger{}, nil, couldNotRun(errors.New("HEAD's " + err.Error()))
	}
	var problems []string
	for _, row := range ledger.Rows() {
		if slices.Contains(headLedger.Rows(), row) {
			continue
		}
		recorded, err := deps.Recorded(root, row)
		if err != nil {
			return deps.Ledger{}, nil, couldNotRun(errors.New("read " + deps.ApprovalsRel + ": " + err.Error()))
		}
		if recorded {
			continue
		}
		problems = append(problems, unrecordedRow(row))
	}
	return ledger, problems, nil
}

// addedImports answers the third-party paths path's carried text imports that
// HEAD's text at the same path did not. A file the commit removes adds nothing.
func addedImports(module, path string, sides specSides) ([]string, error) {
	after, present, problem := sides.after(path)
	if problem != "" {
		return nil, couldNotRun(errors.New("judge third-party imports of " + path + ": " + problem))
	}
	if !present {
		return nil, nil
	}
	afterImports, err := deps.Imports(path, []byte(after))
	if err != nil {
		return nil, errors.New("judge third-party imports of " + path + ": " + err.Error())
	}
	before, present, problem := sides.before(path)
	if problem != "" {
		return nil, couldNotRun(errors.New("judge third-party imports of " + path + ": " + problem))
	}
	var beforeImports []string
	if present {
		// A HEAD blob that does not parse leaves beforeImports empty, so every
		// carried import is judged as added: the strict answer.
		parsed, err := deps.Imports(path, []byte(before))
		if err == nil {
			beforeImports = parsed
		}
	}
	return deps.Added(module, beforeImports, afterImports), nil
}

// unapprovedImport is the refusal for one import: what, why, and the command.
func unapprovedImport(path, pkg, importer, module string) string {
	var text textbuf.Buffer
	text.Str("dependency ledger: ").Str(path).Str(" adds a third-party import Thomas has not approved for ").Str(importer).Byte('\n')
	text.Str("  package:  ").Str(pkg).Byte('\n')
	if module == "" {
		text.Str("  module:   none in ").Str(deps.VendorModulesRel).Str(": vendor it first, in a commit of its own, once Thomas agrees\n")
	} else {
		text.Str("  module:   ").Str(module).Byte('\n')
	}
	text.Str("  importer: ").Str(importer).Byte('\n')
	text.Str("  a third-party module is approved only for its approved use, so each new use needs Thomas's approval\n")
	text.Str("  ask Thomas; if he agrees, he runs from his own terminal:\n")
	text.Str("    ").Str(deps.ApproveCommand(pkg, importer)).Byte('\n')
	text.Str("  then carry ").Str(deps.LedgerRel).Str(" in this commit")
	return text.String()
}

// unrecordedRow is the refusal for a ledger row no approval recorded.
func unrecordedRow(row deps.Row) string {
	var text textbuf.Buffer
	text.Str("dependency ledger: ").Str(deps.LedgerRel).Str(" gains a row ").Str(deps.ApprovalsRel).Str(" does not hold\n")
	text.Str("  row: ").Str(deps.RowLine(row)).Byte('\n')
	text.Str("  a row is written by Thomas's approval, never by hand; ask Thomas, and if he agrees he runs from his own terminal:\n")
	text.Str("    ").Str(deps.ApproveCommand(row.Package, row.Importer))
	return text.String()
}
