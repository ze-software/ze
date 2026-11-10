// Design: docs/contributing/committing.md -- the push debt derived from git: the upstream and the person-act trailers
// Related: actions.go -- the push and debt-status verbs that read the derivation, and the declared trailers
// Related: owed_answers.go -- the producers that answer a person-act obligation
// Related: owed_trailers.go -- reading the trailers and the stable patch-ids over one range
package commit

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// pushCommand is the command a cleared refusal is answered with.
const pushCommand = "./le commit push"

// pullCommand is the command an upstream that moved past P is answered with.
const pullCommand = "git pull --rebase --no-autostash"

// pushTargetKind names how the derivation chose the push target P. Zero is
// pushTargetUnspecified, so a derivation nobody filled never reads as a target.
//
// No target depends on a local verify record: no agent runs `./le verify`,
// and GitHub CI verifies the pushed commit (ai/META-DECISIONS.md).
type pushTargetKind uint8

const (
	pushTargetUnspecified pushTargetKind = iota
	// pushTargetNotApplicable is the answer outside the Ze checkout: no
	// derivation runs and the push is not gated.
	pushTargetNotApplicable
	// pushTargetHead says P is HEAD, the push's own choice.
	pushTargetHead
	// pushTargetNamed is `debt-status commit <rev>` naming a commit on HEAD's
	// branch, which the push-only script's re-check does.
	pushTargetNamed
	// pushTargetNone says the named commit is not admissible: it does not
	// resolve, or it is not on HEAD's branch.
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
	// owedItemLinuxLimit requires the owner's attestation of a Linux limit.
	owedItemLinuxLimit
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
	// `<package>.<Test>` unit for an RFC approval, or the RID for a Linux limit.
	// It is empty for a carried row no commit in range matched.
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
	// Upstream is the SHA `@{u}` resolves to.
	Upstream string
	// Published is `P ^@{u}`, the commits a push of P puts on the upstream.
	Published []owedCommit
	// Items are the person acts still open over `P ^@{u}`, or over
	// `HEAD ^@{u}` when no P exists.
	Items []owedItem
	// Refusals are the reasons a push of P is refused that are not items: no
	// admissible target, an upstream P does not fast-forward, a trailer nobody
	// can read. Each names its command.
	Refusals []string
}

// deriveDebt derives the push debt at root for candidate, the commit
// `debt-status commit <rev>` names, or for HEAD when candidate is empty. An
// error is "cannot derive" (no upstream, git failed), and a caller MUST NOT
// read it as zero owed.
func deriveDebt(root, candidate string) (derivedDebt, error) {
	if !lepath.IsCheckout(root) {
		return derivedDebt{Target: pushTargetNotApplicable}, nil
	}
	upstream, err := commitOf(root, "@{u}")
	if err != nil {
		return derivedDebt{}, fmt.Errorf("cannot derive the push debt: this branch has no upstream "+
			"(%w). Name one with: git branch --set-upstream-to=origin/<branch>", err)
	}
	head, err := commitOf(root, "HEAD")
	if err != nil {
		return derivedDebt{}, fmt.Errorf("cannot derive the push debt: %w", err)
	}
	derived := derivedDebt{Upstream: upstream}
	if err := derived.chooseTarget(root, head, candidate); err != nil {
		return derivedDebt{}, err
	}
	if err := derived.listPublished(root); err != nil {
		return derivedDebt{}, err
	}
	if err := derived.readPersonActs(root, head); err != nil {
		return derivedDebt{}, err
	}
	return derived, nil
}

// chooseTarget sets P: HEAD, or the named candidate when it is on HEAD's
// branch. Whatever P is, the upstream must fast-forward to it.
func (d *derivedDebt) chooseTarget(root, head, candidate string) error {
	if candidate == "" {
		d.Target, d.TargetSHA = pushTargetHead, head
	} else if err := d.chooseNamed(root, head, candidate); err != nil {
		return err
	}
	if d.TargetSHA == "" {
		return nil
	}
	forward, err := isAncestor(root, d.Upstream, d.TargetSHA)
	if err != nil {
		return fmt.Errorf("cannot derive the push debt: %w", err)
	}
	if !forward {
		d.Refusals = append(d.Refusals, "the upstream moved: "+pullCommand+", then "+pushCommand)
	}
	return nil
}

// chooseNamed judges the commit `debt-status commit <rev>` names.
func (d *derivedDebt) chooseNamed(root, head, candidate string) error {
	d.Target = pushTargetNone
	sha, err := commitOf(root, candidate)
	if err != nil {
		d.Refusals = append(d.Refusals, "commit "+candidate+" does not resolve: "+err.Error())
		return nil //nolint:nilerr // an unresolvable candidate is an answer (not admissible), recorded as a refusal
	}
	onBranch, err := isAncestor(root, sha, head)
	if err != nil {
		return fmt.Errorf("cannot derive the push debt: %w", err)
	}
	if !onBranch {
		d.Refusals = append(d.Refusals, "commit "+candidate+" is not on HEAD's branch, so a push never publishes it")
		return nil
	}
	d.Target, d.TargetSHA = pushTargetNamed, sha
	return nil
}

// listPublished fills Published, `P ^@{u}`, in one git call. With no P
// nothing is published.
func (d *derivedDebt) listPublished(root string) error {
	if d.TargetSHA == "" {
		return nil
	}
	published, err := logCommits(root, d.TargetSHA, "^"+d.Upstream)
	if err != nil {
		return fmt.Errorf("cannot derive the push debt: %w", err)
	}
	d.Published = published
	return nil
}

// logCommits lists a range, newest first, with each commit's subject.
func logCommits(root string, revisions ...string) ([]owedCommit, error) {
	output, err := gitOutput(root, append([]string{gitLog, gitNoColor, "--format=%x1e%H%x00%s"},
		append(revisions, "--")...)...)
	if err != nil {
		return nil, err
	}
	commits := make([]owedCommit, 0)
	for record := range strings.SplitSeq(output, "\x1e") {
		sha, subject, found := strings.Cut(record, "\x00")
		if !found {
			continue
		}
		commits = append(commits, owedCommit{SHA: strings.TrimSpace(sha), Subject: strings.TrimSpace(subject)})
	}
	return commits, nil
}

// owed answers whether anything stands between this derivation and a clean
// answer. It is the one rule `debt-status`, the create report and the
// session-start warning read, so none of them can say "nothing owed" while
// another refuses.
//
// Outside the Ze checkout nothing is derived, so nothing is owed. With no
// admissible push target, or any refusal, or any open person act, something
// is owed. A local verify record is never read: none is owed.
func (d *derivedDebt) owed() bool {
	switch d.Target {
	case pushTargetNotApplicable:
		return false
	case pushTargetUnspecified, pushTargetNone:
		return true
	case pushTargetHead, pushTargetNamed:
	}
	return len(d.Refusals) != 0 || len(d.Items) != 0
}

// pushRefusal answers why a push of the derived target is refused, or nil when
// `./le commit push` may prepare its script: a target exists, it is ahead of
// the upstream, the upstream fast-forwards to it, and no item is open.
func (d *derivedDebt) pushRefusal() error {
	switch d.Target {
	case pushTargetNotApplicable:
		return nil
	case pushTargetUnspecified:
		return errors.New("BUG: a push debt derivation chose no push target")
	case pushTargetNone, pushTargetHead, pushTargetNamed:
	}
	if len(d.Refusals) != 0 {
		return errors.New("push refused: " + strings.Join(d.Refusals, "\n  "))
	}
	if len(d.Published) == 0 {
		return errors.New("push refused: nothing is ahead of the upstream " + shortSHA(d.Upstream) +
			"; commit the work, then " + pushCommand)
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
		return "commit " + shortSHA(item.Commit) + " (" + strings.TrimSpace(trailerRFCApprovalOwed) + ") owes a ze-test-judge approval of RFC-tagged test " +
			subjectOr(item.Subject) + " (" + item.Reason + "): ./le rfc approve commit " + item.Commit +
			" unit " + item.Subject + ` reason "<the judge's verdict>", then ./le commit create subject "publish exact judge approval" rfc-approval ` +
			item.Commit + " " + item.Subject + ", or the owner's " + attest
	case owedItemLinuxLimit:
		return "commit " + shortSHA(item.Commit) + " (" + strings.TrimSpace(trailerLinuxLimitOwed) +
			") owes the owner's approval of " + item.Subject + " (" + item.Reason + "): " + attest
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
