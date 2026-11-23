// Design: docs/contributing/gh-pages.md -- the looking glass replay: what ./le site check refuses to publish
// Related: lgreplay.go -- lgReplayBlockHTML, which states the marks this reads
// Related: actions.go -- checkSiteArtifact, the check population this belongs to

package site

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

const (
	// A commit a recording states is hexadecimal, from the shortest
	// abbreviation git writes to a whole SHA-256 name.
	lgReplayCommitLengthMin = 7
	lgReplayCommitLengthMax = 64

	// lgReplayBuildCommand renders the replay's page again.
	lgReplayBuildCommand = "./le site build"
)

// replayRefusal is one reason the artifact's replay is not for publication:
// the three parts of an operator error, kept apart so the check's answer
// carries them as data.
type replayRefusal struct {
	Problem string `json:"problem"`
	Why     string `json:"why"`
	Fix     string `json:"fix"`
}

// operatorError answers the refusal as the operator reads it.
func (refusal replayRefusal) operatorError() *opererr.Error {
	return opererr.New(refusal.Problem, refusal.Why, refusal.Fix)
}

// reportReplayRefusals writes each refusal to the operator: the problem, why
// it matters, and the command that fixes it on the last line.
func reportReplayRefusals(refusals []replayRefusal) {
	for _, refusal := range refusals {
		leaction.ReportError(opererr.New(refusal.Problem, refusal.Why, refusal.Fix))
	}
}

// lgReplayMarks is what the built page states about its replay.
type lgReplayMarks struct {
	// mounted says the page holds the player's mount, so it carries a replay.
	mounted bool
	// stated says the block states its recording's commit, in commit.
	stated bool
	commit string
	// trial says the block states that its recording is a trial.
	trial bool
}

// checkLGReplay judges the replay the artifact carries, from the marks its
// page states (lgReplayBlockHTML): a site build renders any recording, and
// this is where one that must not be published is refused. It refuses a trial
// recording, made from a checkout with uncommitted changes, and a recording
// whose commit is not in the history of the checkout's HEAD.
//
// An artifact without the page, and a page whose block says the looking glass
// is not recorded yet, carry no replay: there is nothing to refuse, and the
// answer is no refusal.
func checkLGReplay(ctx context.Context, paths Paths) ([]replayRefusal, error) {
	page, err := os.ReadFile(filepath.Join(paths.Output, filepath.FromSlash(lgReplayPage))) //nolint:gosec // a site check reads the artifact it was pointed at
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	marks, err := lgReplayPageMarks(string(page))
	if err != nil {
		return nil, err
	}
	if !marks.stated {
		if !marks.mounted {
			return nil, nil
		}
		// A replay with no stated commit cannot be judged, and is not read as
		// one that passed.
		return []replayRefusal{{
			Problem: "The replay on " + lgReplayPage + " does not say which recording it comes from, so it cannot be checked",
			Why: "The page was built before the replay stated its recording's commit, " +
				"so a trial recording would pass unseen. Build the site again",
			Fix: lgReplayBuildCommand,
		}}, nil
	}
	var refusals []replayRefusal
	if marks.trial {
		refusals = append(refusals, replayRefusal{
			Problem: "The replay on " + lgReplayPage + " comes from a trial recording, so this site is not ready to publish",
			Why: sitelgreplay.File + " was recorded from a checkout with uncommitted changes, so the commit the page names " +
				"did not alone produce the answers it shows. A trial is for looking at the replay locally. " +
				"Commit the changes, record again from the committed tree, then build the site again",
			Fix: sitelgreplay.RecorderCommand,
		})
	}
	if !lgReplayCommitWellFormed(marks.commit) {
		return append(refusals, replayRefusal{
			Problem: "The replay on " + lgReplayPage + " names " + strconv.Quote(marks.commit) + " as its commit, which is not a commit id",
			Why: "The page tells a reader which code answered, and this names none. " +
				"Record again from a committed tree, then build the site again",
			Fix: sitelgreplay.RecorderCommand,
		}), nil
	}
	inHistory, err := lgReplayCommitInHistory(ctx, paths.Repository, marks.commit)
	if err != nil {
		return nil, err
	}
	if !inHistory {
		refusals = append(refusals, replayRefusal{
			Problem: "The replay on " + lgReplayPage + " names commit " + marks.commit + ", which is not in the history of this checkout",
			Why: "The page tells a reader which code answered, and no reader can find that commit: " +
				"the recording was made on a branch that was rebased or never landed. " +
				"Record again from a committed tree, then build the site again",
			Fix: sitelgreplay.RecorderCommand,
		})
	}
	return refusals, nil
}

// lgReplayPageMarks reads the replay's marks from one built page: the commit
// and the trial state on the block's root element, and the player's mount.
func lgReplayPageMarks(page string) (lgReplayMarks, error) {
	var marks lgReplayMarks
	tokenizer := xhtml.NewTokenizer(strings.NewReader(page))
	for {
		kind := tokenizer.Next()
		if kind == xhtml.ErrorToken {
			if errors.Is(tokenizer.Err(), io.EOF) {
				return marks, nil
			}
			return lgReplayMarks{}, tokenizer.Err()
		}
		if kind != xhtml.StartTagToken {
			continue
		}
		attributes := map[string]string{}
		for _, attribute := range tokenizer.Token().Attr {
			attributes[attribute.Key] = attribute.Val
		}
		if _, mounted := attributes[lgReplayFrameMark]; mounted {
			marks.mounted = true
		}
		commit, stated := attributes[lgReplayCommitMark]
		if !stated {
			continue
		}
		marks.stated = true
		marks.commit = commit
		_, marks.trial = attributes[lgReplayTrialMark]
	}
}

// lgReplayCommitWellFormed reports whether text has the form of a commit id.
// It is checked before the text reaches git as an argument.
func lgReplayCommitWellFormed(text string) bool {
	if len(text) < lgReplayCommitLengthMin {
		return false
	}
	if len(text) > lgReplayCommitLengthMax {
		return false
	}
	return strings.Trim(text, "0123456789abcdef") == ""
}

// lgReplayCommitInHistory reports whether commit is HEAD or one of its
// ancestors in the checkout at repository. A commit the checkout does not
// hold is not in its history. commit MUST be well formed
// (lgReplayCommitWellFormed). An error means git could not answer, which is
// never read as either answer.
func lgReplayCommitInHistory(ctx context.Context, repository, commit string) (bool, error) {
	// With --quiet, rev-parse answers 1 for a name that is no commit here.
	_, err := publicationGit(ctx, repository, "rev-parse", "--verify", "--quiet", commit+"^{commit}")
	if err != nil {
		return false, lgReplayGitRefused(err)
	}
	// merge-base --is-ancestor answers 1 when the commit is not an ancestor.
	_, err = publicationGit(ctx, repository, "merge-base", "--is-ancestor", commit, "HEAD")
	if err != nil {
		return false, lgReplayGitRefused(err)
	}
	return true, nil
}

// lgReplayGitRefused answers nil when err is git's exit status 1, the answer
// "no" of the two queries lgReplayCommitInHistory asks, and an operator error
// for any other failure, where git did not answer the question.
func lgReplayGitRefused(err error) error {
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		if exit.ExitCode() == 1 {
			return nil
		}
	}
	return opererr.Wrap(err, "The site check could not ask git whether the replay's commit is in this checkout's history",
		"The replay's page names the commit that answered, and the check confirms a reader can find it. "+
			"Run the check from the Ze checkout, with git installed",
		"./le site check")
}
