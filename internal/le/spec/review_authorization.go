// Design: docs/contributing/spec-workflow.md -- terminal-only owner authorization and exact retries.
package spec

import (
	"strconv"

	"github.com/ze-software/ze/internal/le/le/human"
)

// requireOwnerReviewTerminal guards supplied owner words at both review entries.
// It MUST precede owner resolution as well as artifact recording.
func requireOwnerReviewTerminal(root string, request reviewRecord) error {
	if request.OwnerAuthorised == "" {
		return nil
	}
	arguments := []string{keywordSpec, request.Spec, keywordVerdict, request.Verdict, keywordRounds, strconv.Itoa(request.Rounds)}
	for _, file := range request.Files {
		arguments = append(arguments, keywordFile, file)
	}
	for _, pair := range request.Hunks {
		arguments = append(arguments, keywordHunks, pair.Path, pair.Patch)
	}
	for _, pair := range []struct{ keyword, value string }{
		{keywordReviewers, request.Reviewers}, {keywordFindingsFile, request.FindingsFile},
		{keywordRoundsReason, request.RoundsReason}, {keywordOwnerAuthorised, request.OwnerAuthorised},
	} {
		if pair.value != "" {
			arguments = append(arguments, pair.keyword, pair.value)
		}
	}
	return human.RequireAuthorization(root, request.SessionID, "spec review record", arguments)
}
