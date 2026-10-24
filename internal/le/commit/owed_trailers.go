// Design: docs/architecture/testing/verify-freshness-scope.md -- person-act obligations read from commit trailers
// Related: owed.go -- the derivation that reads these items
// Related: owed_answers.go -- the producers that answer each item
// Related: actions.go -- declaredTrailers, the one declaration of every key read here
package commit

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc"
)

// Gate keys a `Ledger-carried:` trailer names: the ledger rows the deletion
// commit carried were these two admissions.
const (
	carriedReviewKey = keywordReviewOverride
	carriedRFCKey    = keywordRFCChangeOK
)

// owedTrailer is one obligation a trailer declares on the commit carrying it.
type owedTrailer struct {
	commit  string
	kind    owedItemKind
	subject string
	reason  string
}

// carriedTrailer is one `Ledger-carried:` row that carries a patch-id, so it
// gates: the commit it names was not yet published when the ledger went.
type carriedTrailer struct {
	deletion string
	sha      string
	patchID  string
	kind     owedItemKind
	reason   string
}

// personActs is everything one range's trailers declare: what is owed, and
// the answers that are records rather than derivations.
type personActs struct {
	owed     []owedTrailer
	carried  []carriedTrailer
	attested map[string]bool
	approved map[string]bool
	problems []string
}

// readPersonActs reads the trailers over `P ^@{u}` (`HEAD ^@{u}` when no P
// exists, for display) and keeps each obligation no answer holds. Answers are
// read over the same range, so an attestation committed after P is not counted
// for a push of P.
func (d *derivedDebt) readPersonActs(root, head string) error {
	tip := d.TargetSHA
	if tip == "" {
		tip = head
	}
	bodies, err := gitOutput(root, "log", "--no-show-signature", "--format=%x1e%H%x00%B", tip, "^"+d.Upstream, "--")
	if err != nil {
		return fmt.Errorf("cannot derive the verification debt: %w", err)
	}
	acts := parsePersonActs(bodies)
	d.Refusals = append(d.Refusals, acts.problems...)
	if len(acts.owed) == 0 && len(acts.carried) == 0 {
		return nil
	}
	patchIDs, err := stablePatchIDs(root, tip, "^"+d.Upstream)
	if err != nil {
		return fmt.Errorf("cannot derive the verification debt: %w", err)
	}
	cache := newCommitCache()
	shas := make([]string, 0, len(acts.owed)+len(patchIDs))
	for _, owed := range acts.owed {
		shas = append(shas, owed.commit)
	}
	for sha := range patchIDs {
		shas = append(shas, sha)
	}
	cache.load(root, shas)
	answers := answering{root: root, cache: cache, acts: &acts, patchIDs: patchIDs}
	for _, owed := range acts.owed {
		if !answers.answered(owed.commit, owed.kind, owed.subject) {
			d.Items = append(d.Items, owedItem{Commit: owed.commit, Kind: owed.kind, Subject: owed.subject, Reason: owed.reason})
		}
	}
	for _, carried := range acts.carried {
		d.Items = append(d.Items, answers.carriedItems(carried)...)
	}
	d.Refusals = append(d.Refusals, answers.problems...)
	return nil
}

// parsePersonActs reads every declared key at the start of a message line.
// Message refuses such a line in a subject or body, so a line that starts with
// a key is the record. A malformed one is a problem, never skipped: dropping
// it would drop the obligation it declares.
func parsePersonActs(bodies string) personActs {
	acts := personActs{attested: make(map[string]bool), approved: make(map[string]bool)}
	for record := range strings.SplitSeq(bodies, "\x1e") {
		sha, body, found := strings.Cut(record, "\x00")
		if !found {
			continue
		}
		sha = strings.TrimSpace(sha)
		for line := range strings.SplitSeq(body, "\n") {
			acts.readLine(sha, strings.TrimRight(line, " \t\r"))
		}
	}
	return acts
}

// readLine records one message line of commit sha.
func (acts *personActs) readLine(sha, line string) {
	malformed := func(key string) {
		acts.problems = append(acts.problems, "commit "+shortSHA(sha)+" carries a malformed "+
			strings.TrimSpace(key)+" trailer "+strconv.Quote(line)+
			": the derivation cannot tell what it owes, so the push waits until that line is corrected")
	}
	if value, ok := strings.CutPrefix(line, trailerReviewOwed); ok {
		subject, reason, ok := cutOwed(value)
		if !ok {
			malformed(trailerReviewOwed)
			return
		}
		acts.owed = append(acts.owed, owedTrailer{commit: sha, kind: owedItemReview, subject: subject, reason: reason})
		return
	}
	if value, ok := strings.CutPrefix(line, trailerRFCApprovalOwed); ok {
		subject, reason, ok := cutOwed(value)
		if !ok {
			malformed(trailerRFCApprovalOwed)
			return
		}
		acts.owed = append(acts.owed, owedTrailer{commit: sha, kind: owedItemRFCApproval, subject: subject, reason: reason})
		return
	}
	if value, ok := strings.CutPrefix(line, trailerOwnerAttested); ok {
		if patchID, _, split := strings.Cut(value, ": "); split && patchID != "" {
			acts.attested[patchID] = true
		}
		return
	}
	if value, ok := strings.CutPrefix(line, rfc.ApprovalTrailer); ok {
		if unit, _, split := strings.Cut(value, ": "); split && unit != "" {
			acts.approved[unit] = true
		}
		return
	}
	if value, ok := strings.CutPrefix(line, trailerLedgerCarried); ok {
		acts.readCarried(sha, value, func() { malformed(trailerLedgerCarried) })
	}
}

// cutOwed splits an owed trailer's value, `<subject>: <reason>`, and answers
// false when either half is missing.
func cutOwed(value string) (subject, reason string, ok bool) {
	subject, reason, split := strings.Cut(value, ": ")
	if !split {
		return "", "", false
	}
	if strings.TrimSpace(subject) == "" {
		return "", "", false
	}
	return subject, reason, true
}

// plainStem answers whether a trailer's spec stem is one plain path segment,
// which is what keeps the artifact glob inside tmp/review/.
func plainStem(stem string) bool {
	if stem == "" {
		return false
	}
	if path.Base(stem) != stem {
		return false
	}
	return !strings.ContainsAny(stem, `*?[\`)
}

// readCarried reads `<sha> [<patch-id>] <gate key>: <reason>`. A row with no
// patch-id names a commit already published: it is audit only.
func (acts *personActs) readCarried(deletion, value string, malformed func()) {
	head, reason, split := strings.Cut(value, ": ")
	if !split {
		malformed()
		return
	}
	fields := strings.Fields(head)
	if len(fields) < 2 {
		malformed()
		return
	}
	if len(fields) > 3 {
		malformed()
		return
	}
	if len(fields) == 2 {
		return
	}
	kind := owedItemUnspecified
	switch fields[2] {
	case carriedReviewKey:
		kind = owedItemReview
	case carriedRFCKey:
		kind = owedItemRFCApproval
	}
	if kind == owedItemUnspecified {
		malformed()
		return
	}
	acts.carried = append(acts.carried, carriedTrailer{
		deletion: deletion, sha: fields[0], patchID: fields[1], kind: kind, reason: reason,
	})
}

// answering evaluates the Person-act answers over one range. problems names
// each ask it could not judge because a read failed (unreadProblem); the item
// stays open and
// readPersonActs hands the problem on as a refusal, so the cause reaches the
// operator. Not safe for concurrent use.
type answering struct {
	root     string
	cache    *commitCache
	acts     *personActs
	patchIDs map[string]string
	problems []string
}

// unreadProblem names an obligation subject on commit that could not be
// judged because a read failed, and the command that shows the commit.
func unreadProblem(commit, subject string, err error) string {
	return "commit " + commit + ": cannot judge " + strconv.Quote(subject) +
		", so it stays owed: " + err.Error() + "; check the commit with `git show " + commit + "`"
}

// answered says whether one obligation on commit is answered: not applicable,
// closed, reviewed or attested for a review; not applicable, approved or
// attested for an RFC approval. A commit whose facts git could not read
// answers nothing, so its item stays open, and the read failure is recorded in
// problems.
func (a *answering) answered(commit string, kind owedItemKind, subject string) bool {
	if patchID := a.patchIDs[commit]; patchID != "" && a.acts.attested[patchID] {
		return true
	}
	facts, err := a.cache.read(a.root, commit)
	if err != nil {
		a.problems = append(a.problems, unreadProblem(commit, subject, err))
		return false
	}
	switch kind {
	case owedItemReview:
		if answerNotApplicable(a.root, owedItemReview, facts) == nil {
			return true
		}
		if answerClosed(a.root, facts) == nil {
			return true
		}
		return a.reviewed(subject, facts)
	case owedItemRFCApproval:
		if a.acts.approved[subject] {
			return true
		}
		changed, err := changedTaggedUnits(a.root, facts)
		if err != nil {
			a.problems = append(a.problems, unreadProblem(commit, subject, err))
			return false
		}
		return !slices.Contains(changed, subject)
	case owedItemUnspecified:
	}
	return false
}

// reviewed says whether a clean `tmp/review/<stem>-*.md` artifact covers the
// commit's own blobs. A stem that is not one plain path segment names no
// artifact, so a trailer cannot point the read outside tmp/review/.
func (a *answering) reviewed(stem string, facts commitFacts) bool {
	if !plainStem(stem) {
		return false
	}
	artifacts, err := filepath.Glob(filepath.Join(a.root, "tmp", "review", stem+"-*.md"))
	if err != nil {
		a.problems = append(a.problems, unreadProblem(facts.SHA, stem, err))
		return false
	}
	for _, artifact := range artifacts {
		relative, err := filepath.Rel(a.root, artifact)
		if err != nil {
			continue
		}
		if answerReviewed(a.root, filepath.ToSlash(relative), facts) == nil {
			return true
		}
	}
	return false
}

// carriedItems answers one gating carried row. The row is matched to the
// commit in range with its patch-id and answered over that commit; with no
// match (a rebase conflict changed the diff) it stays open under the carried
// SHA unless the owner attested the deletion commit itself.
func (a *answering) carriedItems(carried carriedTrailer) []owedItem {
	matched := ""
	for sha, patchID := range a.patchIDs {
		if patchID == carried.patchID {
			matched = sha
			break
		}
	}
	if matched == "" {
		if deletionID := a.patchIDs[carried.deletion]; deletionID != "" && a.acts.attested[deletionID] {
			return nil
		}
		return []owedItem{{Commit: carried.sha, Kind: carried.kind, Reason: carried.reason}}
	}
	open := owedItem{Commit: matched, Kind: carried.kind, Reason: carried.reason}
	facts, err := a.cache.read(a.root, matched)
	if err != nil {
		a.problems = append(a.problems, unreadProblem(matched, carried.reason, err))
		return []owedItem{open}
	}
	if carried.kind == owedItemReview {
		stem, err := closedSpecStem(facts.Paths, facts.Removed, commitSides(a.root, facts))
		if err != nil {
			a.problems = append(a.problems, unreadProblem(matched, carried.reason, err))
			return []owedItem{open}
		}
		if a.answered(matched, owedItemReview, stem) {
			return nil
		}
		open.Subject = stem
		return []owedItem{open}
	}
	units, err := changedTaggedUnits(a.root, facts)
	if err != nil {
		a.problems = append(a.problems, unreadProblem(matched, carried.reason, err))
		return []owedItem{open}
	}
	items := make([]owedItem, 0, len(units))
	for _, unit := range units {
		if a.answered(matched, owedItemRFCApproval, unit) {
			continue
		}
		item := open
		item.Subject = unit
		items = append(items, item)
	}
	return items
}

// stablePatchIDs answers `git patch-id --stable` for each commit of a range
// that changes something, keyed by full SHA. An empty commit has no patch-id
// and is absent. It is the one producer of the id an `Owner-attested:`
// trailer names, so the attestation and the derivation key one commit the
// same way.
func stablePatchIDs(root string, revisions ...string) (map[string]string, error) {
	shown, err := gitOutput(root, append([]string{"log", "-p", gitNoColor, gitNoExtDiff,
		"--no-show-signature", "--format=commit %H"}, append(revisions, "--")...)...)
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(context.Background(), "git", "patch-id", "--stable") // #nosec G204 -- fixed Git query; the diff goes through stdin.
	command.Dir = root
	command.Stdin = strings.NewReader(shown)
	var stdout, complaint bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git patch-id --stable: %w: %s", err, strings.TrimSpace(complaint.String()))
	}
	ids := make(map[string]string)
	for line := range strings.SplitSeq(stdout.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			ids[fields[1]] = fields[0]
		}
	}
	return ids, nil
}
