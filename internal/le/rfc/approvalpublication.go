// Design: docs/contributing/rfc-implementation-guide.md -- captured ordinary approvals are single-use at each actual publication parent.
package rfc

import (
	"errors"
	"fmt"
	"strings"
)

// CheckOrdinaryApprovalPublication refuses a verdict already spent in parent's
// history. Callers MUST retain the actual full-SHA publication parent for this
// query and every captured original verdict; pending rows are not authority at
// publication time because a preceding appended block may have pruned them.
func CheckOrdinaryApprovalPublication(root, parent string, verdicts []string) error {
	if !ValidApprovalTarget(parent) {
		return errors.New("ordinary approval publication requires the exact full-SHA parent")
	}
	if len(verdicts) == 0 {
		return errors.New("ordinary approval publication requires a captured verdict")
	}
	history, err := approvalGit(root, "log", "--no-show-signature", "--format=%H", parent, "--")
	if err != nil {
		return fmt.Errorf("cannot read ordinary approval publication history: %w", err)
	}
	text := history
	first, _, _ := strings.Cut(text, "\n")
	if first != parent {
		return errors.New("ordinary approval publication parent is not the exact named Git commit")
	}
	queries := make([]approvalHistoryQuery, len(verdicts))
	for index, verdict := range verdicts {
		queries[index] = newApprovalHistoryQuery(verdict, "")
	}
	carrier, matched, problem := approvalHistoryLanded(root, text, queries)
	if problem != "" {
		return errors.New(problem)
	}
	if matched >= 0 {
		query := queries[matched]
		command := ""
		if query.rfcVerdict {
			command = ApproveCommand(query.unit)
		} else if strings.HasPrefix(query.trailer, "Weakening-approved: ") {
			command = "./le test weakened approve unit " + query.unit + ` reason "<the judge's verdict>"`
		}
		hint := ""
		if command != "" {
			hint = "\nAn independent judge must record the fresh verdict with:\n  " + command +
				"\nThen prepare a fresh ./le commit create subject \"<subject>\" with the intended file/remove/hunks selection; do not rerun the captured block."
		}
		return fmt.Errorf("ordinary approval %q was already published in %s; obtain a fresh independent judgement before preparing this change again%s", verdicts[matched], carrier, hint)
	}
	return nil
}
