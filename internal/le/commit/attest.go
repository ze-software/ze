// Design: docs/architecture/testing/verify-freshness-scope.md -- person acts, read from trailers
// Related: owed_trailers.go -- the reader of the trailers written here
// Related: prepare.go -- Create, which writes them into the message
package commit

import (
	"errors"
	"fmt"
	"strings"
)

// attestationTrailer answers the `Owner-attested:` trailer for the commit an
// attestation names, refusing what the owner's words could not answer: a
// commit outside `HEAD ^@{u}`, a commit carrying no owed trailer, a commit
// with no patch-id. The patch-id comes from stablePatchIDs, the producer the
// derivation keys attestations by, so the two never disagree.
func attestationTrailer(root string, native bool, options *Options) (string, error) {
	if !native {
		return "", errors.New("attest refused: this tree is not the Ze checkout, so no commit in it owes a person act to attest")
	}
	words := strings.TrimSpace(options.AttestWords)
	if words == "" {
		return "", errors.New(`attest refused: the owner's words are empty; give them as: attest <commit> "<owner words>"`)
	}
	sha, err := commitOf(root, options.Attest)
	if err != nil {
		return "", fmt.Errorf("attest refused: %q names no commit (%w); name the commit that owes the act, as ./le commit debt-status lists it", options.Attest, err)
	}
	upstream, err := commitOf(root, "@{u}")
	if err != nil {
		return "", fmt.Errorf("attest refused: this branch has no upstream (%w), so which commits are unpublished is unknown; "+
			"name one with: git branch --set-upstream-to=origin/<branch>", err)
	}
	if err := refuseOutsideLocalRange(root, sha, upstream); err != nil {
		return "", err
	}
	body, err := gitOutput(root, "log", "-1", "--no-show-signature", "--format=%x1e%H%x00%B", sha, "--")
	if err != nil {
		return "", fmt.Errorf("attest refused: read the message of %s: %w", shortSHA(sha), err)
	}
	acts := parsePersonActs(body)
	if len(acts.owed) == 0 && len(acts.carried) == 0 {
		return "", fmt.Errorf("attest refused: commit %s carries no %s, %s or gating %s trailer, so it owes no person act "+
			"to attest; ./le commit debt-status names the commits that do", shortSHA(sha),
			strings.TrimSpace(trailerReviewOwed), strings.TrimSpace(trailerRFCApprovalOwed), strings.TrimSpace(trailerLedgerCarried))
	}
	patchID, err := patchIDOf(root, sha)
	if err != nil {
		return "", fmt.Errorf("attest refused: %w", err)
	}
	return admissionTrailer(trailerOwnerAttested, patchID, words)
}

// refuseOutsideLocalRange refuses a commit that is not in `HEAD ^@{u}`: one
// already published, which no push can change any more, or one not on this
// branch, which no push of it would carry.
func refuseOutsideLocalRange(root, sha, upstream string) error {
	published, err := isAncestor(root, sha, upstream)
	if err != nil {
		return fmt.Errorf("attest refused: compare %s with the upstream: %w", shortSHA(sha), err)
	}
	if published {
		return fmt.Errorf("attest refused: commit %s is already on the upstream %s, so no push can change what it carries "+
			"and an attestation answers nothing", shortSHA(sha), shortSHA(upstream))
	}
	onBranch, err := isAncestor(root, sha, "HEAD")
	if err != nil {
		return fmt.Errorf("attest refused: compare %s with HEAD: %w", shortSHA(sha), err)
	}
	if !onBranch {
		return fmt.Errorf("attest refused: commit %s is not on this branch, so no push of it would carry the commit; "+
			"name a commit ./le commit debt-status lists", shortSHA(sha))
	}
	return nil
}

// patchIDOf answers the stable patch-id of one commit. An empty commit has
// none, and that is a refusal, never an empty key.
func patchIDOf(root, sha string) (string, error) {
	ids, err := stablePatchIDs(root, "-1", sha)
	if err != nil {
		return "", fmt.Errorf("read the patch-id of %s: %w", shortSHA(sha), err)
	}
	if ids[sha] == "" {
		return "", fmt.Errorf("commit %s changes nothing, so it has no patch-id to key the record by", shortSHA(sha))
	}
	return ids[sha], nil
}

// ledgerCarriedTrailers answers one `Ledger-carried:` trailer per row the
// ledger-deletion commit carries, each given as `<sha> <gate key>: <reason>`.
// The patch-id is never the caller's: a row on a commit already in `@{u}` gets
// none and is audit only, and a row on a commit in `HEAD ^@{u}` gets the
// commit's stable patch-id, so the derivation holds the push until it is
// answered (D-8 rule). The route is the ledger-deletion commit alone, which
// removes and adds nothing.
func ledgerCarriedTrailers(root string, rows, paths, removed []string) ([]string, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	if len(paths) != 0 {
		return nil, errors.New("ledger-carried refused: it belongs on the ledger-deletion commit, which adds no file; " +
			"name only remove paths")
	}
	if len(removed) == 0 {
		return nil, errors.New("ledger-carried refused: it belongs on the ledger-deletion commit, which removes the ledger; " +
			"name its files with remove")
	}
	upstream, err := commitOf(root, "@{u}")
	if err != nil {
		return nil, fmt.Errorf("ledger-carried refused: this branch has no upstream (%w); "+
			"name one with: git branch --set-upstream-to=origin/<branch>", err)
	}
	trailers := make([]string, 0, len(rows))
	for _, row := range rows {
		trailer, err := ledgerCarriedTrailer(root, upstream, row)
		if err != nil {
			return nil, err
		}
		trailers = append(trailers, trailer)
	}
	return trailers, nil
}

// ledgerCarriedTrailer renders one carried row.
func ledgerCarriedTrailer(root, upstream, row string) (string, error) {
	form := `ledger-carried "<sha> <gate key>: <reason>", the gate key ` + carriedReviewKey + ` or ` + carriedRFCKey
	head, reason, split := strings.Cut(row, ": ")
	if !split {
		return "", fmt.Errorf("ledger-carried refused: %q is not %s", row, form)
	}
	fields := strings.Fields(head)
	if len(fields) != 2 {
		return "", fmt.Errorf("ledger-carried refused: %q is not %s", row, form)
	}
	key := fields[1]
	if key != carriedReviewKey && key != carriedRFCKey {
		return "", fmt.Errorf("ledger-carried refused: gate key %q is not a person act; %s", key, form)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", fmt.Errorf("ledger-carried refused: %q gives no reason; %s", row, form)
	}
	sha, err := commitOf(root, fields[0])
	if err != nil {
		return "", fmt.Errorf("ledger-carried refused: %q names no commit (%w)", fields[0], err)
	}
	published, err := isAncestor(root, sha, upstream)
	if err != nil {
		return "", fmt.Errorf("ledger-carried refused: compare %s with the upstream: %w", shortSHA(sha), err)
	}
	if published {
		return admissionTrailer(trailerLedgerCarried, sha+" "+key, reason)
	}
	onBranch, err := isAncestor(root, sha, "HEAD")
	if err != nil {
		return "", fmt.Errorf("ledger-carried refused: compare %s with HEAD: %w", shortSHA(sha), err)
	}
	if !onBranch {
		return "", fmt.Errorf("ledger-carried refused: commit %s is neither on the upstream nor on this branch, so the row "+
			"cannot be keyed to a commit a push would carry; find where the commit went before the ledger is deleted", shortSHA(sha))
	}
	patchID, err := patchIDOf(root, sha)
	if err != nil {
		return "", fmt.Errorf("ledger-carried refused: %w", err)
	}
	return admissionTrailer(trailerLedgerCarried, sha+" "+patchID+" "+key, reason)
}
