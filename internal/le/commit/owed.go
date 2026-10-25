// Design: docs/architecture/testing/verify-freshness-scope.md -- verification debt derived from git and the scoped record
// Related: actions.go -- the push and debt-status verbs that read the derivation, and the declared trailers
// Related: owed_answers.go -- the producers that answer a person-act obligation
// Related: owed_trailers.go -- reading the trailers and the stable patch-ids over one range
package commit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
	"github.com/ze-software/ze/internal/le/verify"
)

// scopedCommand is the command every "a scoped verify is owed" answer
// names.
const scopedCommand = verify.ChangedCommand

// pullCommand is the command an upstream that moved past P is answered with.
const pullCommand = "git pull --rebase --no-autostash"

// pushTargetKind names how the derivation chose the push target P. Zero is
// pushTargetUnspecified, so a derivation nobody filled never reads as a target.
type pushTargetKind uint8

const (
	pushTargetUnspecified pushTargetKind = iota
	// pushTargetNotApplicable is the answer outside the Ze checkout: no
	// derivation runs and the push is not gated.
	pushTargetNotApplicable
	// pushTargetHead says the scoped tree covers HEAD^{tree}, so P is HEAD.
	pushTargetHead
	// pushTargetRecord says the scoped record's commit G is an ancestor of HEAD
	// whose tree does not cover HEAD's, so P is the last commit of the walk
	// from G and the commits after it wait.
	pushTargetRecord
	// pushTargetNamed is `debt-status commit <rev>` naming an admissible P.
	pushTargetNamed
	// pushTargetNone says no commit on HEAD's branch is covered by the green
	// record: no record, or G left the branch through a rebase.
	pushTargetNone
)

// owedItemKind names a person-act obligation a trailer declares. Zero is
// owedItemUnspecified, so an item nobody classified is never read as answered.
type owedItemKind uint8

const (
	owedItemUnspecified owedItemKind = iota
	// owedItemReview is a `Review-owed:` trailer, or a carried review-override row.
	owedItemReview
	// owedItemRFCApproval is an `RFC-approval-owed:` trailer, or a carried
	// rfc-change-ok row.
	owedItemRFCApproval
)

// owedCommit is one commit the derivation names for display: the full SHA and
// its subject line.
type owedCommit struct {
	SHA     string
	Subject string
}

// owedItem is one person-act obligation still open on a commit the push would
// publish.
type owedItem struct {
	// Commit is the full SHA of the commit that owes the act. For a carried
	// row no commit in range matched, it is the carried SHA.
	Commit string
	Kind   owedItemKind
	// Subject is what the act is about: the spec stem for a review, the
	// `<package>.<Test>` unit for an RFC approval. It is empty for a carried
	// row no commit in range matched.
	Subject string
	Reason  string
}

// derivedDebt is the derivation's answer for one candidate push target. It is
// read by the push verb, the push-only script's re-check, the create report
// and the session-start hook, and none of them holds a rule of its own.
type derivedDebt struct {
	Target pushTargetKind
	// TargetSHA is P. It is empty exactly when Target is pushTargetNone or
	// pushTargetNotApplicable.
	TargetSHA string
	// Green is the scoped record's commit, empty when there is no record.
	Scoped string
	// Upstream is the SHA `@{u}` resolves to.
	Upstream string
	// Published is `P ^@{u}`, the commits a push of P puts on the upstream.
	Published []owedCommit
	// LeftBehind is the commits in `HEAD ^P` when P exists, else in
	// `HEAD ^@{u} ^G`, that change a non-prose path: the commits that owe a
	// scoped verify before a push can carry them. A prose-only commit is never
	// listed (repochanged.IsProse).
	LeftBehind []owedCommit
	// Items are the person acts still open over `P ^@{u}`, or over
	// `HEAD ^@{u}` when no P exists.
	Items []owedItem
	// Refusals are the reasons a push of P is refused that are not items: no
	// target, an upstream P does not fast-forward, a comparison git could not
	// make. Each names its command.
	Refusals []string
}

// deriveDebt derives the verification debt at root for candidate, the commit
// `debt-status commit <rev>` names, or for HEAD's own choice of P when
// candidate is empty. An error is "cannot derive" (no upstream, git failed,
// an unreadable scoped record), and a caller MUST NOT read it as zero owed.
func deriveDebt(root, candidate string) (derivedDebt, error) {
	if !lepath.IsCheckout(root) {
		return derivedDebt{Target: pushTargetNotApplicable}, nil
	}
	upstream, err := commitOf(root, "@{u}")
	if err != nil {
		return derivedDebt{}, fmt.Errorf("cannot derive the verification debt: this branch has no upstream "+
			"(%w). Name one with: git branch --set-upstream-to=origin/<branch>", err)
	}
	head, err := commitOf(root, "HEAD")
	if err != nil {
		return derivedDebt{}, fmt.Errorf("cannot derive the verification debt: %w", err)
	}
	derived := derivedDebt{Upstream: upstream}
	record, present, err := readScoped(root, &derived)
	if err != nil {
		return derivedDebt{}, err
	}
	greenKnown := false
	if present {
		derived.Scoped = record.Commit()
		shas, err := resolveCommits(root, []string{record.Commit()})
		if err != nil {
			return derivedDebt{}, fmt.Errorf("cannot derive the verification debt: %w", err)
		}
		greenKnown = shas[0] != ""
	}
	if present {
		if err := derived.chooseTarget(root, head, candidate, record, greenKnown); err != nil {
			return derivedDebt{}, err
		}
	} else {
		derived.Target = pushTargetNone
	}
	if derived.Target == pushTargetNone {
		derived.Refusals = append(derived.Refusals, noTargetRefusal(candidate))
	}
	if err := derived.listCommits(root, head, greenKnown); err != nil {
		return derivedDebt{}, err
	}
	if err := derived.readPersonActs(root, head); err != nil {
		return derivedDebt{}, err
	}
	return derived, nil
}

// readScoped reads the scoped record. An absent record and one that does not
// prove a push are answers, each recorded as a refusal naming the verify; any
// other failure to read it is "cannot derive".
func readScoped(root string, derived *derivedDebt) (verify.ScopedRecord, bool, error) {
	record, err := verify.ReadScopedRecord(root)
	if err == nil {
		return record, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		derived.Refusals = append(derived.Refusals, "no scoped record: "+scopedCommand+" has not passed in this checkout")
		return verify.ScopedRecord{}, false, nil
	}
	if errors.Is(err, verify.ErrNotScoped) {
		derived.Refusals = append(derived.Refusals, "the scoped record proves no push: "+err.Error())
		return verify.ScopedRecord{}, false, nil
	}
	return verify.ScopedRecord{}, false, fmt.Errorf("cannot derive the verification debt: read %s: %w", verify.ChangedPath, err)
}

// noTargetRefusal is the refusal for a derivation that found no commit the
// scoped record covers.
func noTargetRefusal(candidate string) string {
	if candidate != "" {
		return "commit " + candidate + " is not a verified push target: run " + scopedCommand +
			", then ./le commit push"
	}
	return "no commit on this branch is covered by the scoped record: run " + scopedCommand +
		", then ./le commit push"
}

// chooseTarget sets P by the push target rules: HEAD when the scoped tree
// covers its tree, else the last commit of the walk from G along HEAD's first
// parents, else none. A named candidate is admissible only when the scoped tree
// covers it and it is on HEAD's branch. Whatever P is, the upstream must
// fast-forward to it.
func (d *derivedDebt) chooseTarget(root, head, candidate string, record verify.ScopedRecord, greenKnown bool) error {
	switch {
	case candidate != "":
		if err := d.chooseNamed(root, head, candidate, record.Tree()); err != nil {
			return err
		}
	case d.covered(root, record.Tree(), head):
		d.Target, d.TargetSHA = pushTargetHead, head
	case greenKnown:
		if err := d.walkFromRecord(root, head, record); err != nil {
			return err
		}
	default:
		d.Target = pushTargetNone
	}
	if d.TargetSHA == "" {
		return nil
	}
	forward, err := isAncestor(root, d.Upstream, d.TargetSHA)
	if err != nil {
		return fmt.Errorf("cannot derive the verification debt: %w", err)
	}
	if !forward {
		d.Refusals = append(d.Refusals, "the upstream moved: "+pullCommand+", then "+scopedCommand)
	}
	// The record proves P only when its change set held every commit the push
	// publishes: A..C holds @{u}..P exactly when the anchor A is an ancestor of,
	// or equal to, the current @{u}. An @{u} that only advanced still holds A,
	// and the forward check above answers it. A fails here when the upstream no
	// longer holds it: the upstream was rewritten under the run, or the run
	// anchored on another ref, so commits this push publishes were never in the
	// run's scope.
	anchored, err := isAncestor(root, record.Anchor(), d.Upstream)
	if err != nil {
		return fmt.Errorf("cannot derive the verification debt: %w", err)
	}
	if !anchored {
		d.Refusals = append(d.Refusals, "the upstream moved past the scoped run's anchor "+shortSHA(record.Anchor())+
			" without holding it ("+shortSHA(record.Anchor())+" is not an ancestor of "+shortSHA(d.Upstream)+
			": the upstream was rewritten, or the run anchored on another ref), so the run did not judge every "+
			"commit this push publishes: "+pullCommand+", then "+scopedCommand)
	}
	return nil
}

// chooseNamed judges the commit `debt-status commit <rev>` names.
func (d *derivedDebt) chooseNamed(root, head, candidate, greenTree string) error {
	d.Target = pushTargetNone
	sha, err := commitOf(root, candidate)
	if err != nil {
		d.Refusals = append(d.Refusals, "commit "+candidate+" does not resolve: "+err.Error())
		return nil //nolint:nilerr // an unresolvable candidate is an answer (not admissible), recorded as a refusal
	}
	if !d.covered(root, greenTree, sha) {
		return nil
	}
	onBranch, err := isAncestor(root, sha, head)
	if err != nil {
		return fmt.Errorf("cannot derive the verification debt: %w", err)
	}
	if !onBranch {
		d.Refusals = append(d.Refusals, "commit "+candidate+" is not on HEAD's branch, so a push never publishes it")
		return nil
	}
	d.Target, d.TargetSHA = pushTargetNamed, sha
	return nil
}

// walkFromRecord advances X from G along `--first-parent G..HEAD` while the
// scoped tree covers each commit, and stops at the first it does not cover. G
// itself is judged too: a record whose tree does not cover its own commit
// proves nothing about it.
func (d *derivedDebt) walkFromRecord(root, head string, record verify.ScopedRecord) error {
	d.Target = pushTargetNone
	onBranch, err := isAncestor(root, record.Commit(), head)
	if err != nil {
		return fmt.Errorf("cannot derive the verification debt: %w", err)
	}
	if !onBranch {
		return nil
	}
	listed, err := gitOutput(root, "rev-list", "--reverse", "--first-parent", record.Commit()+".."+head)
	if err != nil {
		return fmt.Errorf("cannot derive the verification debt: %w", err)
	}
	walk := append([]string{record.Commit()}, strings.Fields(listed)...)
	for _, commit := range walk {
		if !d.covered(root, record.Tree(), commit) {
			break
		}
		d.Target, d.TargetSHA = pushTargetRecord, commit
	}
	return nil
}

// covered answers whether the scoped tree covers revision's tree: every path
// `git diff --name-only` names between them is prose. An empty diff is
// covered. A diff git could not compute is NOT covered, and the refusal names
// git's error: it is never read as an empty diff. Renames are off, so a file
// moved out of a prose tree names its old path too.
func (d *derivedDebt) covered(root, greenTree, revision string) bool {
	names, err := gitOutput(root, "-c", "core.quotePath=false", "diff", "--no-renames", gitNoExtDiff,
		"--name-only", greenTree, revision, "--")
	if err != nil {
		d.Refusals = append(d.Refusals, "cannot compare "+revision+" with the scoped tree, so it is not covered: "+
			err.Error()+"; run "+scopedCommand)
		return false
	}
	return proseOnly(strings.Split(names, "\n"))
}

// proseOnly answers whether every named path is prose. No path at all is
// prose only: nothing a full verification judges changed.
func proseOnly(paths []string) bool {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if !repochanged.IsProse(path) {
			return false
		}
	}
	return true
}

// listCommits fills Published (`P ^@{u}`) and LeftBehind (`HEAD ^P`, or
// `HEAD ^@{u} ^G` with no P, or `HEAD ^@{u}` when G is unknown here), each in
// one git call.
func (d *derivedDebt) listCommits(root, head string, greenKnown bool) error {
	if d.TargetSHA != "" {
		published, err := logCommits(root, d.TargetSHA, "^"+d.Upstream)
		if err != nil {
			return fmt.Errorf("cannot derive the verification debt: %w", err)
		}
		for _, one := range published {
			d.Published = append(d.Published, one.owedCommit)
		}
	}
	behind := make([]string, 0, 3)
	switch {
	case d.TargetSHA != "":
		behind = append(behind, head, "^"+d.TargetSHA)
	case greenKnown:
		behind = append(behind, head, "^"+d.Upstream, "^"+d.Scoped)
	default:
		behind = append(behind, head, "^"+d.Upstream)
	}
	left, err := logCommits(root, behind...)
	if err != nil {
		return fmt.Errorf("cannot derive the verification debt: %w", err)
	}
	for _, one := range left {
		if proseOnly(one.paths) {
			continue
		}
		d.LeftBehind = append(d.LeftBehind, one.owedCommit)
	}
	return nil
}

// loggedCommit is one commit of a range with the paths it changes against its
// first parent.
type loggedCommit struct {
	owedCommit
	paths []string
}

// logCommits lists a range, newest first, with each commit's subject and the
// paths it changes. Renames are off for the same reason as covered.
func logCommits(root string, revisions ...string) ([]loggedCommit, error) {
	output, err := gitOutput(root, append([]string{"-c", "core.quotePath=false", "log", "--no-renames",
		gitNoExtDiff, gitNoColor, "--diff-merges=first-parent", "--name-only",
		"--format=%x1e%H%x00%s%x00"}, append(revisions, "--")...)...)
	if err != nil {
		return nil, err
	}
	commits := make([]loggedCommit, 0)
	for record := range strings.SplitSeq(output, "\x1e") {
		fields := strings.SplitN(record, "\x00", 3)
		if len(fields) != 3 {
			continue
		}
		one := loggedCommit{SHA: strings.TrimSpace(fields[0]), Subject: fields[1]}
		for path := range strings.SplitSeq(fields[2], "\n") {
			if path = strings.TrimSpace(path); path != "" {
				one.paths = append(one.paths, path)
			}
		}
		commits = append(commits, one)
	}
	return commits, nil
}

// owed answers whether anything stands between this derivation and a clean
// answer. It is the one rule `debt-status`, the create report and the
// session-start warning read, so none of them can say "nothing owed" while
// another refuses.
//
// Outside the Ze checkout nothing is derived, so nothing is owed. Any refusal
// or open person act is owed. With no push target, HEAD's tree is not covered
// by a scoped verify, and that is owed even when no commit is listed. A target
// the derivation found itself (HEAD's choice) owes a scoped verify when it is not
// HEAD, or when it leaves a commit behind. A target `debt-status commit <rev>`
// named is judged alone: what follows it on the branch does not count (AC-25).
func (d *derivedDebt) owed() bool {
	switch d.Target {
	case pushTargetNotApplicable:
		return false
	case pushTargetUnspecified, pushTargetNone:
		return true
	case pushTargetHead, pushTargetRecord, pushTargetNamed:
	}
	if len(d.Refusals) != 0 {
		return true
	}
	if len(d.Items) != 0 {
		return true
	}
	if d.Target == pushTargetNamed {
		return false
	}
	if d.Target != pushTargetHead {
		return true
	}
	return len(d.LeftBehind) != 0
}

// pushRefusal answers why a push of the derived target is refused, or nil when
// `./le commit push` may prepare its script: a target exists, it is ahead of
// the upstream, the upstream fast-forwards to it, and no item is open.
func (d *derivedDebt) pushRefusal() error {
	switch d.Target {
	case pushTargetNotApplicable:
		return nil
	case pushTargetUnspecified:
		return errors.New("BUG: a verification debt derivation chose no push target")
	case pushTargetNone, pushTargetHead, pushTargetRecord, pushTargetNamed:
	}
	if len(d.Refusals) != 0 {
		return errors.New("push refused: " + strings.Join(d.Refusals, "\n  "))
	}
	if len(d.Published) == 0 {
		return errors.New("push refused: nothing verified is ahead of the upstream " + shortSHA(d.Upstream) +
			"; commit the work, run " + scopedCommand + ", then ./le commit push")
	}
	if len(d.Items) == 0 {
		return nil
	}
	lines := make([]string, 0, len(d.Items))
	for _, item := range d.Items {
		lines = append(lines, item.describe())
	}
	return errors.New("push refused: person act(s) still owed:\n  " + strings.Join(lines, "\n  "))
}

// describe names one open item and the commands that answer it.
func (item *owedItem) describe() string {
	attest := `./le commit create ... attest ` + shortSHA(item.Commit) + ` "<owner words>"`
	switch item.Kind {
	case owedItemReview:
		return "commit " + shortSHA(item.Commit) + " (" + strings.TrimSpace(trailerReviewOwed) + ") owes an independent review of " + subjectOr(item.Subject) +
			" (" + item.Reason + "): record a clean review with /ze-review, or the owner's " + attest
	case owedItemRFCApproval:
		return "commit " + shortSHA(item.Commit) + " (" + strings.TrimSpace(trailerRFCApprovalOwed) + ") owes the owner's approval of RFC-tagged test " +
			subjectOr(item.Subject) + " (" + item.Reason + "): ./le rfc approve unit " + item.Subject +
			` reason "<owner words>" and commit it, or the owner's ` + attest
	case owedItemUnspecified:
	}
	return "BUG: commit " + shortSHA(item.Commit) + " carries an owed item of no kind"
}

func subjectOr(subject string) string {
	if subject == "" {
		return "the carried commit (no commit in range has its patch-id)"
	}
	return subject
}

// shortSHA is the seven-character form an operator reads.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// commitOf resolves one revision to a commit SHA.
func commitOf(root, revision string) (string, error) {
	sha, err := gitOutput(root, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}

// isAncestor answers `git merge-base --is-ancestor ancestor descendant`: exit
// 0 is yes, exit 1 is no, and anything else is a git failure, never a no.
func isAncestor(root, ancestor, descendant string) (bool, error) {
	_, err := gitOutput(root, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}
