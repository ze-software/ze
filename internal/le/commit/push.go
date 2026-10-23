// Design: docs/contributing/committing.md -- the push route over the derived verification debt
// Related: owed.go -- the derivation every answer here reads
// Related: script.go -- the commit script whose marker and header the push-only script shares
package commit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// pushTag is the tag a push-only script's name carries in place of a commit
// tag: the script prepares no commit, so no message file holds the tag.
const pushTag = "push"

// debtStatusCommand is the re-check the push-only script runs against P.
const debtStatusCommand = "./le commit " + keywordDebtStatus + " commit "

// pushPrepared is what `./le commit push` answers: the script it wrote, the
// commit the script pushes, and the commits the push leaves behind.
type pushPrepared struct {
	Script     string       `json:"script"`
	Target     string       `json:"target"`
	Remote     string       `json:"remote"`
	Branch     string       `json:"branch"`
	Applicable bool         `json:"applicable"`
	LeftBehind []owedCommit `json:"left-behind,omitempty"`
}

// preparePush derives what a push owes and, when the derivation admits it,
// writes the push-only script. The code is 1 for a refusal the derivation
// names and 2 for a push it cannot prepare at all.
func preparePush(root, authorisation string) (pushPrepared, int, error) {
	var prepared pushPrepared
	reason, err := pushAuthorisation(authorisation)
	if err != nil {
		return prepared, 2, err
	}
	derived, err := deriveDebt(root, "")
	if err != nil {
		return prepared, 2, err
	}
	if err := derived.pushRefusal(); err != nil {
		return prepared, 1, err
	}
	prepared.Applicable = derived.Target != pushTargetNotApplicable
	prepared.Target = derived.TargetSHA
	if !prepared.Applicable {
		// Outside the Ze checkout nothing is derived, so the push publishes
		// HEAD as it stands now and the script carries no re-check (AC-23).
		prepared.Target, err = commitOf(root, "HEAD")
		if err != nil {
			return prepared, 2, fmt.Errorf("read the HEAD this push publishes: %w", err)
		}
	}
	prepared.LeftBehind = derived.LeftBehind
	prepared.Remote, prepared.Branch, err = upstreamOf(root)
	if err != nil {
		return prepared, 2, err
	}
	session, err := lepath.CommitSession(root, "")
	if err != nil {
		return prepared, 2, err
	}
	prepared.Script, err = pushScriptPath(root, session)
	if err != nil {
		return prepared, 2, err
	}
	text := renderPushScript(root, session, reason, &prepared)
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		return prepared, 2, err
	}
	full := filepath.Join(root, filepath.FromSlash(prepared.Script))
	if err := os.WriteFile(full, []byte(text), 0o750); err != nil { //nolint:gosec // the prepared push script is executed, so it needs the owner execute bit
		return prepared, 2, err
	}
	if err := os.Chmod(full, 0o750); err != nil { //nolint:gosec // the prepared push script is executed, so it needs the owner execute bit
		return prepared, 2, err
	}
	return prepared, 0, nil
}

// upstreamOf answers the remote and the remote branch ref HEAD's branch
// tracks. A detached HEAD and a branch with no upstream are refusals that
// name the fix, never a guess at a remote.
func upstreamOf(root string) (string, string, error) {
	branch, err := gitOutput(root, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		return "", "", fmt.Errorf("push refused: HEAD is not on a branch (%w); check out the branch to push", err)
	}
	branch = strings.TrimSpace(branch)
	tracked, err := gitOutput(root, "for-each-ref", "--format=%(upstream:remotename)%00%(upstream:remoteref)", branch)
	if err != nil {
		return "", "", fmt.Errorf("push refused: read the upstream of %s: %w", branch, err)
	}
	remote, ref, _ := strings.Cut(strings.TrimSpace(tracked), "\x00")
	if remote == "" {
		return "", "", errors.New("push refused: " + branch + " has no upstream; name one with: git branch --set-upstream-to=origin/<branch>")
	}
	if ref == "" {
		return "", "", errors.New("push refused: " + branch + " has no upstream branch; name one with: git branch --set-upstream-to=origin/<branch>")
	}
	return remote, ref, nil
}

// pushScriptPath answers where this session's push-only script goes. A
// push-only script already there is prepared again in place, because P is
// chosen again. A commit script with a block not yet run refuses the push:
// its commit is neither verified nor published, and the session would read
// the push as carrying it. A commit script whose every block ran is spent.
func pushScriptPath(root, session string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "tmp", "commit-"+session+"-*.sh"))
	if err != nil {
		return "", err
	}
	slices.Sort(matches)
	reuse := ""
	for _, match := range matches {
		content, err := os.ReadFile(match) //nolint:gosec // the path is this session's commit artifact under the checkout root
		if err != nil {
			return "", err
		}
		text := string(content)
		if !strings.Contains(text, scriptMarker) {
			continue
		}
		relative, err := filepath.Rel(root, match)
		if err != nil {
			return "", err
		}
		relative = filepath.ToSlash(relative)
		if pushOnly(text) {
			if reuse == "" {
				reuse = relative
			}
			continue
		}
		if pending := pendingBlocks(root, text); len(pending) != 0 {
			return "", fmt.Errorf("push refused: %s holds commit block(s) %s not yet run, and an unrun commit is "+
				"neither verified nor published; run it with: bash %s, then %s, then ./le commit push",
				relative, strings.Join(pending, ", "), relative, verifyWorktreeCommand)
		}
	}
	if reuse != "" {
		return reuse, nil
	}
	return allocateScript(root, session, pushTag)
}

// pushOnly answers whether a generated script is a push-only script: it
// carries the push marker and no commit block.
func pushOnly(text string) bool {
	if !strings.Contains(text, "\n"+pushMarker) {
		return false
	}
	return !strings.Contains(text, "\n"+blockMarker)
}

// pendingBlocks answers the tags of a commit script's blocks that have not
// run. A block deletes its message file once its commit lands (renderBlock),
// so a block whose message file still exists has not published.
func pendingBlocks(root, text string) []string {
	var pending []string
	tag := ""
	for line := range strings.SplitSeq(text, "\n") {
		if value, ok := strings.CutPrefix(line, blockMarker+" tag="); ok {
			tag, _, _ = strings.Cut(value, " ")
			continue
		}
		value, ok := strings.CutPrefix(line, "_ze_message=")
		if !ok {
			continue
		}
		words := parseShellWords(value)
		if len(words) != 1 {
			pending = append(pending, tag)
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(words[0]))); err == nil {
			pending = append(pending, tag)
		}
	}
	return pending
}

// renderPushScript writes the push-only script: no commit block, the
// re-derivation against P in the Ze checkout, then the push of P itself to
// the upstream branch. It never pushes the branch tip, so a commit that lands
// after preparation is not published (D-6).
func renderPushScript(root, session, reason string, prepared *pushPrepared) string {
	var page textbuf.Buffer
	page.Str(scriptHeader(root, prepared.Script, session))
	page.Str(markerLine(pushMarker, reason)).Byte('\n')
	page.Str(commentLine("Push " + prepared.Target + " to " + prepared.Remote + " " + prepared.Branch +
		". This script makes no commit.")).Byte('\n')
	if prepared.Applicable {
		page.Str("# The derivation again, against the prepared commit: a lost green record, a moved\n")
		page.Str("# upstream or a rebase that took the commit off the branch stops the push here.\n")
		page.Str("if ! ").Str(debtStatusCommand).Str(prepared.Target).Str("; then\n")
		page.Str("  echo ").Str(shellQuote("ERROR: the push of " + shortSHA(prepared.Target) +
			" is refused by the re-check above, and nothing was pushed. Clear what it names, then prepare the push again with ./le commit push.")).
			Str(" >&2\n  exit 1\nfi\n")
	}
	page.Str("git push ").Str(shellQuote(prepared.Remote)).Byte(' ').
		Str(shellQuote(prepared.Target + ":" + prepared.Branch)).Byte('\n')
	return page.String()
}

// Text prints the script path first, as create does, then what the push
// carries and what it leaves behind.
func (p *pushPrepared) Text() string {
	var text textbuf.Buffer
	text.Str("script=").Str(p.Script).Str("\npush=").Str(p.Target).Str(" -> ").Str(p.Remote).
		Byte(' ').Str(p.Branch).Byte('\n')
	if !p.Applicable {
		text.Str("verify=NOT-APPLICABLE (").Str(notCheckoutDetail).Str(")\n")
	}
	for _, left := range p.LeftBehind {
		text.Str("not in this push, owes ").Str(verifyWorktreeCommand).Str(": ").
			Str(shortSHA(left.SHA)).Byte(' ').Str(left.Subject).Byte('\n')
	}
	return text.String()
}

// debtStatus is the answer of `./le commit debt-status [commit <rev>]`.
type debtStatus struct {
	Applicable bool         `json:"applicable"`
	Owed       bool         `json:"owed"`
	Target     string       `json:"target,omitempty"`
	Green      string       `json:"green,omitempty"`
	LeftBehind []owedCommit `json:"left-behind,omitempty"`
	Items      []string     `json:"items,omitempty"`
	Refusals   []string     `json:"refusals,omitempty"`
}

// answerDebtStatus derives the debt for HEAD's choice of P, or for candidate,
// and answers the exit code: 0 nothing owed, 1 something owed, 2 cannot
// answer. With no candidate, nothing is owed only when P is HEAD with nothing
// left behind and no item open. With a candidate, what follows it on the
// branch does not count: it is owed only when it is not an admissible target
// or an item is open in `<candidate> ^@{u}`.
func answerDebtStatus(root, candidate string) (debtStatus, int, error) {
	derived, err := deriveDebt(root, candidate)
	if err != nil {
		return debtStatus{}, 2, err
	}
	status := debtStatus{
		Applicable: derived.Target != pushTargetNotApplicable,
		Target:     derived.TargetSHA, Green: derived.Green,
		LeftBehind: derived.LeftBehind, Refusals: derived.Refusals,
	}
	for _, item := range derived.Items {
		status.Items = append(status.Items, item.describe())
	}
	if !status.Applicable {
		return status, 0, nil
	}
	status.Owed = len(status.Refusals) != 0 || len(status.Items) != 0
	if candidate == "" && derived.Target != pushTargetHead {
		status.Owed = true
	}
	if candidate == "" && len(status.LeftBehind) != 0 {
		status.Owed = true
	}
	if status.Owed {
		return status, 1, nil
	}
	return status, 0, nil
}

// Text names the answer on its first line, then every reason it is owed.
func (s *debtStatus) Text() string {
	if !s.Applicable {
		return "verification debt: not applicable (" + notCheckoutDetail + ")\n"
	}
	if !s.Owed {
		return "verification debt: nothing owed; push target " + shortSHA(s.Target) + "\n"
	}
	var text textbuf.Buffer
	text.Str("verification debt: owed\n  push target: ").Str(orNone(shortSHA(s.Target))).
		Str("\n  green record: ").Str(orNone(shortSHA(s.Green))).Byte('\n')
	for _, refusal := range s.Refusals {
		text.Str("  refused: ").Str(refusal).Byte('\n')
	}
	for _, left := range s.LeftBehind {
		text.Str("  not in the push target, owes ").Str(verifyWorktreeCommand).Str(": ").
			Str(shortSHA(left.SHA)).Byte(' ').Str(left.Subject).Byte('\n')
	}
	for _, item := range s.Items {
		text.Str("  ").Str(item).Byte('\n')
	}
	return text.String()
}

func orNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

// debtWarningItemsMax bounds the items the session-start warning lists, so
// the hook's message stays one screen however much is owed.
const debtWarningItemsMax = 5

// DebtWarning answers the session-start warning over the derived debt: one
// line and at most five items, or "" when HEAD's tree is covered and no
// person act is open. An error is "cannot answer", and the caller MUST NOT
// read it as nothing owed.
func DebtWarning(root string) (string, error) {
	derived, err := deriveDebt(root, "")
	if err != nil {
		return "", err
	}
	if len(derived.LeftBehind) == 0 && len(derived.Items) == 0 {
		return "", nil
	}
	var text textbuf.Buffer
	text.Str("verification debt: ")
	if len(derived.LeftBehind) == 0 {
		text.Str(strconv.Itoa(len(derived.Items))).Str(" person act(s) owed")
	} else {
		text.Str(strconv.Itoa(len(derived.LeftBehind))).Str(" commit(s) owe a full verify since ").Str(derived.since())
	}
	text.Str(", push refused until cleared\n")
	lines := make([]string, 0, len(derived.LeftBehind)+len(derived.Items))
	for _, left := range derived.LeftBehind {
		lines = append(lines, shortSHA(left.SHA)+" "+left.Subject)
	}
	for _, item := range derived.Items {
		lines = append(lines, item.describe())
	}
	for index, line := range lines {
		if index == debtWarningItemsMax {
			text.Str("  ... and ").Str(strconv.Itoa(len(lines) - index)).Str(" more: ./le commit ").Str(keywordDebtStatus).Byte('\n')
			break
		}
		text.Str("  ").Str(line).Byte('\n')
	}
	return text.String(), nil
}

// debtReport is the create report's `debt=` value: how many commits owe a
// full verify, and since which green record. A derivation that cannot answer
// says so; it never reads as zero.
func debtReport(root string) string {
	derived, err := deriveDebt(root, "")
	if err != nil {
		return "cannot answer: " + commentSafe(err.Error())
	}
	return strconv.Itoa(len(derived.LeftBehind)) + " commit(s) owe a full verify since " + derived.since()
}

// since names the green record a debt is counted from.
func (d *derivedDebt) since() string {
	if d.Green == "" {
		return "no green record"
	}
	return shortSHA(d.Green)
}
