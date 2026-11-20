// Design: docs/contributing/committing.md -- terminal-only attestation issuance and exact retries.
package commit

import "github.com/ze-software/ze/internal/le/le/human"

// requireOwnerTerminal guards both registered and callable entry points of the
// two owner keywords, `attest` and `decision-change`: each records Thomas's
// words, so each is admitted only from his terminal. It MUST run before any
// claim lookup or commit-session resolution can write.
func requireOwnerTerminal(root string, options *Options) error {
	if !ownerKeywordGiven(options) {
		return nil
	}
	return human.RequireAuthorization(root, "", "commit create", ownerRetryArguments(options))
}

// ownerKeywordGiven answers whether the line carries an owner keyword.
func ownerKeywordGiven(options *Options) bool {
	if options.Attest != "" {
		return true
	}
	return options.DecisionChange != ""
}

// ownerRetryArguments is the whole line the refusal hands Thomas to run.
func ownerRetryArguments(options *Options) []string {
	arguments := []string{keywordSubject, options.Subject}
	if options.Attest != "" {
		arguments = append(arguments, keywordAttest, options.Attest, options.AttestWords)
	}
	for _, body := range options.Body {
		arguments = append(arguments, keywordBody, body)
	}
	// A retry MUST preserve incompatible operands too: dropping one could
	// turn the original refusal into an admissible owner attestation.
	for _, file := range options.Files {
		arguments = append(arguments, keywordFile, file)
	}
	for _, file := range options.Remove {
		arguments = append(arguments, keywordRemove, file)
	}
	for _, pair := range options.Hunks {
		arguments = append(arguments, keywordHunks, pair.Path, pair.Patch)
	}
	for _, row := range options.LedgerCarried {
		arguments = append(arguments, keywordLedgerCarried, row)
	}
	for _, pair := range []struct{ keyword, value string }{
		{keywordSession, options.Session}, {keywordTag, options.Tag}, {keywordScript, options.Script},
		{keywordNoTest, options.NoTest}, {keywordReviewOverride, options.ReviewOverride},
		{keywordRFCChangeOK, options.RFCChangeOK}, {keywordNoNews, options.NoNews},
		{keywordDecisionChange, options.DecisionChange},
	} {
		if pair.value != "" {
			arguments = append(arguments, pair.keyword, pair.value)
		}
	}
	if options.Append {
		arguments = append(arguments, keywordAppend)
	}
	if options.Replace {
		arguments = append(arguments, keywordReplace)
	}
	if options.DryRun {
		arguments = append(arguments, keywordDryRun)
	}
	return arguments
}
