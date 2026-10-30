// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet
// Overview: check_evidence.go -- checkExclusionEvidence, which judges against what this file reads
// Related: check_baseline.go -- baselineLevels, gitCatBlobs, gitTreePaths
package rfc

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// priorState is what git answers about HEAD^, the revision the ratchet
// compares against. Zero is no answer at all, which no caller may pass.
type priorState int

const (
	priorUnspecified priorState = iota
	// priorAbsent is a checkout with no HEAD^ to read: no .git above the
	// tree, no commit, or a root commit. Nothing is known to be new, so every
	// row is counted.
	priorAbsent
	// priorRead is a HEAD^ whose summaries and committed counts were read.
	priorRead
	// priorUnreadable is a HEAD^ git holds and could not answer for. It is a
	// refusal, never the absent case: a broken reader that passes is how a
	// ratchet stops judging.
	priorUnreadable
)

// evidenceBaseline is HEAD^ as the exclusion evidence ratchet reads it, with
// the committed counts it compares the working tree's against.
type evidenceBaseline struct {
	state priorState
	// reason says why the state is absent or unreadable, and when unreadable
	// ends with the command that repairs it.
	reason string
	// prints is each requirement id's evidenceFingerprint at HEAD^, and
	// unreadable the stems whose summary HEAD^ could not parse.
	prints     map[string]string
	unreadable map[string]bool
	// pushedPrints is the same at backlogRevision, over every summary that
	// branch parses, and pushedRead says it was read: false in a clone that
	// has not fetched the branch, where HEAD^ alone judges a row.
	pushedPrints map[string]string
	pushedRead   bool
	// backlog is the committed count of each kind at HEAD^, or at the newest
	// commit before it whose copy of the file parses, and backlogAt names that
	// commit. Nil when no commit before HEAD ever held a copy that parses.
	backlog   map[string]int
	backlogAt string
	// pushed is the same at backlogRevision, the pushed branch, so a count one
	// commit raised cannot be carried by the next. Nil when the branch is not
	// fetched or never held the file.
	pushed   map[string]int
	pushedAt string
	// pushedNotice says the pushed branch is absent, so rows and counts are
	// judged against HEAD^ alone. Empty when it was read.
	pushedNotice string
}

// repairGitSuffix closes a refusal naming a git command that failed.
const repairGitSuffix = "; repair the checkout until that command answers (`git fsck` names what is lost), then rerun `./le rfc check`"

// priorRevisionState answers whether HEAD^ exists, telling a checkout that
// has none from one git cannot read. A tree with no `.git` above it, a HEAD
// that names no commit yet (`git rev-parse --verify` exits 1 there) and a
// HEAD `git rev-list --parents` lists with no parent are absent. A shallow
// clone, a listed parent whose object cannot be read, and any other git
// failure are unreadable: git also exits 1 for a parent a partial clone or
// corruption lost, so the exit status alone never decides absence. Each
// unreadable reason ends with its own fix.
func priorRevisionState(tree string) (priorState, string) {
	if !insideGitCheckout(tree) {
		return priorAbsent, tree + " is not a git checkout"
	}
	shallow, failure := gitAnswer(tree, "rev-parse", "--is-shallow-repository")
	if failure != "" {
		return priorUnreadable, failure + repairGitSuffix
	}
	if shallow == "true" {
		return priorUnreadable, "the checkout is a shallow clone, so the commits before HEAD may be cut off; run `git fetch --unshallow`"
	}
	if _, code, failure := gitExit(tree, "rev-parse", "--verify", "--quiet", headRevision); code == 1 {
		return priorAbsent, "HEAD names no commit yet"
	} else if failure != "" {
		return priorUnreadable, failure + repairGitSuffix
	}
	listed, failure := gitAnswer(tree, "rev-list", "--parents", "-n1", headRevision)
	if failure != "" {
		return priorUnreadable, failure + repairGitSuffix
	}
	commits := strings.Fields(listed)
	if len(commits) == 0 {
		return priorUnreadable, "`git rev-list --parents -n1 HEAD` answered nothing" + repairGitSuffix
	}
	if len(commits) == 1 {
		return priorAbsent, "HEAD has no parent commit (a root commit)"
	}
	var tb textbuf.Buffer
	if _, failure := gitAnswer(tree, "cat-file", "-e", tb.Str(commits[1]).Str("^{commit}").String()); failure != "" {
		return priorUnreadable, "HEAD lists the parent " + commits[1] + ", whose commit git cannot read (" + failure +
			"); run `git fetch` to restore the missing object, or `git fsck` to find what is lost, then rerun `./le rfc check`"
	}
	return priorRead, ""
}

// gitAnswer runs git in tree and answers its trimmed output, or a failure
// naming the command and what git said.
func gitAnswer(tree string, args ...string) (string, string) {
	out, _, failure := gitExit(tree, args...)
	return out, failure
}

// gitExit is gitAnswer with git's exit status, -1 when git did not run.
func gitExit(tree string, args ...string) (string, int, string) {
	cmd := exec.Command("git", args...) //nolint:gosec,noctx // this developer tool queries the checkout it was given
	cmd.Dir = tree
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), 0, ""
	}
	code := -1
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		code = exit.ExitCode()
	}
	return "", code, "`git " + strings.Join(args, " ") + "` failed: " + strings.TrimSpace(err.Error()+" "+stderr.String())
}

// insideGitCheckout answers whether tree or a directory above it holds a
// `.git` entry: a directory in a clone, a file in a worktree. A path that
// cannot be made absolute answers true, so git is asked and answers
// unreadable rather than the checkout passing as absent.
func insideGitCheckout(tree string) bool {
	dir, err := filepath.Abs(tree)
	// refuse-by-default: guards priorRevisionState -- true sends the tree to git, which answers unreadable, never absent
	if err != nil {
		return true
	}
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// priorEvidence reads what the ratchet compares against: the HEAD^
// fingerprints baselineLevels already parsed, the pushed branch's
// fingerprints over every summary, and the committed counts at HEAD^ and at
// the pushed branch, each from the newest commit whose copy parses.
func priorEvidence(tree string, prints map[string]string, unreadable map[string]bool, levelsKnown bool) evidenceBaseline {
	state, reason := priorRevisionState(tree)
	if state != priorRead {
		return evidenceBaseline{state: state, reason: reason}
	}
	if !levelsKnown {
		return evidenceBaseline{state: priorUnreadable, reason: "git could not read the summaries under " + summaryRel +
			" at HEAD^; repair the checkout until `git ls-tree HEAD^ -- " + summaryRel + "` and `git cat-file` answer, then rerun `./le rfc check`"}
	}
	base := evidenceBaseline{state: priorRead, prints: prints, unreadable: unreadable}
	prior, failure := committedBacklogAt(tree, priorRevision)
	if failure != "" {
		return evidenceBaseline{state: priorUnreadable, reason: failure}
	}
	base.backlog, base.backlogAt = prior.counts, prior.at
	present, failure := pushedBranchPresent(tree)
	if failure != "" {
		return evidenceBaseline{state: priorUnreadable, reason: failure}
	}
	if !present {
		base.pushedNotice = "exclusion evidence: no " + backlogRevision + " (" + pushedBranchRef +
			" is absent), so rows and committed counts are judged against HEAD^ alone; run `git fetch origin` to judge against the pushed branch too"
		return base
	}
	pushed, failure := committedBacklogAt(tree, backlogRevision)
	if failure != "" {
		return evidenceBaseline{state: priorUnreadable, reason: failure}
	}
	base.pushed, base.pushedAt = pushed.counts, pushed.at
	base.pushedPrints, failure = pushedFingerprints(tree)
	if failure != "" {
		return evidenceBaseline{state: priorUnreadable, reason: failure}
	}
	base.pushedRead = true
	return base
}

// committedBacklog is the committed counts as one revision's history holds
// them: counts and at from the newest commit whose copy parses, nil when none
// does.
type committedBacklog struct {
	counts map[string]int
	at     string
}

// pushedBranchRef is the ref backlogRevision names.
const pushedBranchRef = "refs/remotes/" + backlogRevision

// pushedBranchPresent answers whether the pushed branch exists, telling a
// clone that never fetched it from one whose ref git cannot resolve. An
// absent ref (`git show-ref --verify --quiet` exits 1) is absence; a ref that
// exists and names no readable commit, or git failing otherwise, is the
// failure answer, since the branch the ratchet must judge against is then
// unknown rather than missing.
func pushedBranchPresent(tree string) (bool, string) {
	_, code, failure := gitExit(tree, "show-ref", "--verify", "--quiet", pushedBranchRef)
	if code == 1 {
		return false, ""
	}
	if failure != "" {
		return false, failure + repairGitSuffix
	}
	var tb textbuf.Buffer
	if _, failure := gitAnswer(tree, "rev-parse", "--verify", "--quiet", tb.Str(pushedBranchRef).Str("^{commit}").String()); failure != "" {
		return false, pushedBranchRef + " exists but names no commit git can read (" + failure +
			"); run `git fetch origin` to restore it, then rerun `./le rfc check`"
	}
	return true, ""
}

// committedBacklogAt answers the committed counts at revision, or at the
// newest commit before it whose copy of the file parses. A copy that does not
// parse is skipped, and the walk goes on to an older one, so a commit that
// breaks the file cannot let the next raise it; the commit that broke it was
// refused when it was checked (readEvidenceBacklog). The second answer is a
// git failure.
func committedBacklogAt(tree, revision string) (committedBacklog, string) {
	blobs, ok := gitCatBlobs(tree, revision, []string{evidenceBacklogRel})
	if !ok {
		return committedBacklog{}, "`git cat-file` could not read " + revision + ":" + evidenceBacklogRel + repairGitSuffix
	}
	if text, held := blobs[evidenceBacklogRel]; held {
		if counts, refusal := parseEvidenceBacklog(text, revision+":"+evidenceBacklogRel); refusal == "" {
			return committedBacklog{counts: counts, at: revision}, ""
		}
	}
	// A deletion is not a commit that holds the file, so --diff-filter=d
	// skips it and the commits that still held it answer, newest first.
	holders, failure := gitAnswer(tree, "log", "--format=%H", "--diff-filter=d", revision, "--", evidenceBacklogRel)
	if failure != "" {
		return committedBacklog{}, failure + repairGitSuffix
	}
	for holder := range strings.FieldsSeq(holders) {
		blobs, ok := gitCatBlobs(tree, holder, []string{evidenceBacklogRel})
		text, held := blobs[evidenceBacklogRel]
		if !ok {
			return committedBacklog{}, "`git cat-file` could not read " + holder + ":" + evidenceBacklogRel + repairGitSuffix
		}
		if !held {
			return committedBacklog{}, "`git cat-file` found no " + evidenceBacklogRel + " at " + holder +
				", which `git log` names as holding it" + repairGitSuffix
		}
		at := holder + " (the newest commit at or before " + revision + " whose copy parses)"
		counts, refusal := parseEvidenceBacklog(text, holder+":"+evidenceBacklogRel)
		if refusal != "" {
			continue
		}
		return committedBacklog{counts: counts, at: at}, ""
	}
	return committedBacklog{}, ""
}

// pushedFingerprints answers the evidenceFingerprint backlogRevision records
// for each row of every summary it parses, so a row is counted only when the
// pushed branch, as well as HEAD^, shows it unchanged: a row one commit added
// is still told new in the next. A summary the branch does not hold or cannot
// parse records nothing, and its rows are judged new. A git failure is the
// second answer.
func pushedFingerprints(tree string) (map[string]string, string) {
	paths, ok := gitTreePaths(tree, backlogRevision, summaryRel, ".md")
	if !ok {
		return nil, "`git ls-tree` could not list " + summaryRel + " at " + backlogRevision + "; run `git fetch origin`, then rerun `./le rfc check`"
	}
	blobs, ok := gitCatBlobs(tree, backlogRevision, paths)
	if !ok {
		return nil, "`git cat-file` could not read the summaries under " + summaryRel + " at " + backlogRevision +
			"; run `git fetch origin`, then rerun `./le rfc check`"
	}
	prints := map[string]string{}
	for _, rel := range paths {
		text, held := blobs[rel]
		if !held {
			continue
		}
		requirements, err := ParseSummaryText(text, strings.TrimSuffix(filepath.Base(rel), ".md"), rel)
		if err != nil {
			continue
		}
		for i := range requirements {
			prints[requirements[i].RID] = evidenceFingerprint(&requirements[i])
		}
	}
	return prints, ""
}
