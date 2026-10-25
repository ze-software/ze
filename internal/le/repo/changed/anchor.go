// Design: docs/architecture/testing/verify-freshness-scope.md -- one change-set selection
// Related: selector.go -- the package and tag selector that reads the anchor
// Related: changed.go -- the race-group selector that reads the anchor
//
// The change set is the working tree plus every commit HEAD holds that the
// upstream does not. The second term starts at the anchor, and the anchor
// belongs to git: merge-base(HEAD, U), where U is the branch's upstream when it
// has one and refs/remotes/origin/HEAD when it does not.
//
// The anchor used to be the commit the last green verify recorded. That record
// is overwritten by every run, red or green, so one interrupted run made every
// later scoped answer `./...` until somebody ran a full verification, and the
// only way out of the wide answer was the run the scoped answer exists to avoid.
// No verify run can move a git ref, so this anchor cannot be widened by one.
//
// A commit already on the upstream is outside the change set. It was judged
// when it was pushed, and the release check judges the whole tree again.

package repochanged

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The two refs the anchor is computed against, in the order they are asked.
const (
	anchorUpstreamRef   = "@{u}"
	anchorOriginHeadRef = "refs/remotes/origin/HEAD"
	// anchorHead is HEAD as git names it, and what `rev-parse --abbrev-ref`
	// prints in its place when HEAD is detached.
	anchorHead = "HEAD"
)

// ErrNoUpstream says the checkout holds neither an upstream for its branch nor
// refs/remotes/origin/HEAD, so the change set has no committed term to start
// from. A caller refuses on it: answering the working tree alone would drop
// every unpushed commit, and answering `./...` would make every scoped run a
// full run, which is the loop this anchor exists to break.
var ErrNoUpstream = errors.New("the branch has no upstream and the checkout has no refs/remotes/origin/HEAD")

// Anchor is where the committed term of the change set starts.
type Anchor struct {
	// Commit is merge-base(HEAD, Upstream): the newest commit the upstream
	// already holds.
	Commit string
	// Upstream is the ref the anchor was computed against, @{u} or
	// refs/remotes/origin/HEAD.
	Upstream string
	// UpstreamCommit is the commit Upstream named when the anchor was computed.
	UpstreamCommit string
}

// ResolveAnchor computes the anchor of the checkout at root through query,
// which runs one git command there and answers its standard output.
//
// It never answers an anchor it did not compute. A missing upstream is
// ErrNoUpstream wrapped with the command that sets one; a merge-base git cannot
// compute is an error naming both commits.
func ResolveAnchor(root string, query func(string, ...string) (string, error)) (Anchor, error) {
	upstream, upstreamCommit, err := resolveUpstream(root, query)
	if err != nil {
		return Anchor{}, err
	}
	out, err := query(root, "merge-base", anchorHead, upstreamCommit)
	if err != nil {
		return Anchor{}, errors.New(anchorFailure(upstream, upstreamCommit, err.Error()))
	}
	commit := strings.TrimSpace(out)
	if commit == "" {
		return Anchor{}, errors.New(anchorFailure(upstream, upstreamCommit, "git answered no commit"))
	}
	return Anchor{Commit: commit, Upstream: upstream, UpstreamCommit: upstreamCommit}, nil
}

// resolveUpstream answers the first of @{u} and refs/remotes/origin/HEAD that
// names a commit, with that commit. A lookup that errors or answers nothing
// does not name one, so the next ref is asked.
func resolveUpstream(root string, query func(string, ...string) (string, error)) (string, string, error) {
	for _, ref := range []string{anchorUpstreamRef, anchorOriginHeadRef} {
		out, err := query(root, "rev-parse", "--verify", "-q", ref+"^{commit}")
		if err != nil {
			continue
		}
		commit := strings.TrimSpace(out)
		if commit == "" {
			continue
		}
		return ref, commit, nil
	}
	return "", "", noUpstreamError(root, query)
}

// noUpstreamError wraps ErrNoUpstream with why it matters and the command that
// fixes the case at hand, on its own line. On a branch that is the command
// giving the branch an upstream. With HEAD detached, as in the worktree a
// verify run judges, no branch exists and only refs/remotes/origin/HEAD can
// name the upstream, so the command is the one that sets that ref; a linked
// worktree shares the refs, so running it in the checkout fixes the run.
//
// Which case holds is git's answer to `git rev-parse --abbrev-ref HEAD`, which
// prints the branch, or `HEAD` when HEAD is detached, and exits 0 for both. A
// query that fails, or answers nothing, is a git error and is reported as one,
// with the command that shows it: it is never read as a detached HEAD.
func noUpstreamError(root string, query func(string, ...string) (string, error)) error {
	var message textbuf.Buffer
	message.Str(ErrNoUpstream.Error()).
		Str(", so the change set cannot tell which commits are not yet pushed\n")
	out, err := query(root, "rev-parse", "--abbrev-ref", anchorHead)
	branch := strings.TrimSpace(out)
	if err != nil {
		message.Str("  git could not say whether HEAD is on a branch (").Str(err.Error()).
			Str(");\n  fix what git reports here, then run the command again:\n").
			Str("    git -C ").Str(root).Str(" rev-parse --abbrev-ref HEAD")
		return &anchorError{message: message.String()}
	}
	if branch == "" {
		message.Str("  git answered no branch and no detached HEAD;\n").
			Str("  fix what git reports here, then run the command again:\n").
			Str("    git -C ").Str(root).Str(" rev-parse --abbrev-ref HEAD")
		return &anchorError{message: message.String()}
	}
	if branch == anchorHead {
		message.Str("  HEAD is detached, so only ").Str(anchorOriginHeadRef).
			Str(" can name the upstream, and it names no commit;\n").
			Str("  set it from the remote, then run the command again:\n").
			Str("    git remote set-head origin --auto")
		return &anchorError{message: message.String()}
	}
	message.Str("  set the branch's upstream, then run the command again:\n").
		Str("    git branch --set-upstream-to=origin/").Str(branch)
	return &anchorError{message: message.String()}
}

// anchorFailure is the message for a merge-base git could not compute.
func anchorFailure(upstream, upstreamCommit, cause string) string {
	var message textbuf.Buffer
	message.Str("the change set's anchor, merge-base of HEAD and ").Str(upstream).
		Str(" (").Str(upstreamCommit).Str("), could not be computed: ").Str(cause)
	return message.String()
}

// anchorError carries the full no-upstream message and still answers
// errors.Is(err, ErrNoUpstream).
type anchorError struct {
	message string
}

func (e *anchorError) Error() string { return e.message }

// Unwrap makes errors.Is(err, ErrNoUpstream) hold.
func (e *anchorError) Unwrap() error { return ErrNoUpstream }
