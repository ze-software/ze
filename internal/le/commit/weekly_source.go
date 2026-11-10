// Design: docs/architecture/core-design.md -- the weekly report of tooling events
// Related: register.go -- registers both sections with le weekly report
// Related: newsfragment.go -- parseNewsFragment and newsFoundBy, which the fragments section reads
// Related: ../weekly/events.go -- EventRange and RegisterEventSource, the contract both sections answer
//
// weekly_source.go answers the two sections of `le weekly report` whose
// records this package owns: the approval lines commit messages carry, and the
// news fragments a commit validates. Both read the range's commits from git,
// so neither reads the exported endpoint trees.

package commit

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
	"github.com/ze-software/ze/internal/le/weekly"
)

// The two sections this package answers, kebab-case as weekly requires.
const (
	weeklyApprovalsSection = "approvals"
	weeklyFragmentsSection = "fix-fragments"
)

// testJudgeLine opens the body line a ze-test-judge verdict on a golden,
// fixture or expected-output refresh leaves when no command records it:
// `Test-judge: <path>: <verdict sentence>` (ai/skills/ze-test-judge.md). The
// line is hand-written, so this is its one declaration in Go.
const testJudgeLine = "Test-judge: "

// weeklyApprovalLines are the message lines the approvals section counts, in
// the order it lists them. Each key is the declared constant of its writer.
var weeklyApprovalLines = []struct {
	key  string
	name string
}{
	{key: rfc.ApprovalTrailer, name: "RFC-approved trailers"},
	{key: testweakened.ApprovalTrailer, name: "Weakening-approved trailers"},
	{key: testJudgeLine, name: "Test-judge lines"},
}

// weeklyApprovalsNote states what the count leaves out, so a reader does not
// take it for every verdict a judge gave (spec-weekly-event-report, R-4).
const weeklyApprovalsNote = "Counts only the approvals that landed in the range's commits. " +
	"A refused change leaves no record, so refusals are not counted."

// weeklyFragmentsNote states what the fragments section lists.
const weeklyFragmentsNote = "Lists the news fragments added between the two revisions, read at the second, " +
	"grouped by the check that found the defect each one fixes. " +
	"A fragment with no found-by was surfaced by no check and is counted as one total."

// answerWeeklyApprovals counts each approval line over the range's commits,
// each count with the commits that carry it.
func answerWeeklyApprovals(ctx context.Context, events weekly.EventRange) (weekly.SectionBody, error) {
	messages, err := weeklyCommitMessages(ctx, events)
	if err != nil {
		return weekly.SectionBody{}, err
	}
	facts := make([]weekly.Fact, 0, len(weeklyApprovalLines))
	for _, approval := range weeklyApprovalLines {
		count, commits := weeklyCountLines(events.Commits, messages, approval.key)
		facts = append(facts, weekly.Fact{Name: approval.name, Value: strconv.Itoa(count), Items: commits})
	}
	return weekly.SectionBody{Note: weeklyApprovalsNote, Facts: facts}, nil
}

// weeklyCountLines answers how many lines of the messages open with key, and
// the commits that carry at least one, in range order.
func weeklyCountLines(commits []string, messages map[string]string, key string) (int, []string) {
	count := 0
	var carriers []string
	for _, sha := range commits {
		carried := 0
		for line := range strings.SplitSeq(messages[sha], "\n") {
			if strings.HasPrefix(strings.TrimRight(line, " \t\r"), key) {
				carried++
			}
		}
		if carried == 0 {
			continue
		}
		count += carried
		carriers = append(carriers, sha)
	}
	return count, carriers
}

// weeklyCommitMessages reads the full message of every commit of the range in
// one git call. A commit git does not answer for is an error: its approvals
// would otherwise drop out of the count unseen.
func weeklyCommitMessages(ctx context.Context, events weekly.EventRange) (map[string]string, error) {
	messages := make(map[string]string, len(events.Commits))
	// An empty range reads nothing: `git log` given no revision reads HEAD.
	if len(events.Commits) == 0 {
		return messages, nil
	}
	args := append([]string{"log", gitNoShowSignature, "--no-walk=unsorted", "--format=%x1e%H%x00%B"},
		events.Commits...)
	output, err := gitOutputContext(ctx, events.Repo, append(args, "--")...)
	if err != nil {
		return nil, fmt.Errorf("read the messages of the range's commits: %w", err)
	}
	for record := range strings.SplitSeq(output, "\x1e") {
		sha, body, found := strings.Cut(record, "\x00")
		if !found {
			if strings.TrimSpace(record) != "" {
				return nil, errors.New("git answered a commit message record with no commit ID: " + strconv.Quote(record))
			}
			continue
		}
		messages[strings.TrimSpace(sha)] = body
	}
	for _, sha := range events.Commits {
		if _, read := messages[sha]; !read {
			return nil, errors.New("git answered no message for commit " + sha + " of the range, so its approvals cannot be counted")
		}
	}
	return messages, nil
}

// answerWeeklyFragments lists the news fragments added in the range, grouped
// by found-by. A fragment that does not parse fails the section with its path:
// dropping it would remove a fix from the count with nothing to show it.
func answerWeeklyFragments(ctx context.Context, events weekly.EventRange) (weekly.SectionBody, error) {
	paths, err := weeklyAddedFragments(ctx, events)
	if err != nil {
		return weekly.SectionBody{}, err
	}
	owners := make(map[string][]string, len(newsFoundBy))
	unattributed := 0
	var vocabulary map[string]string
	for _, path := range paths {
		if problem := newsFragmentNameProblem(path); problem != "" {
			return weekly.SectionBody{}, errors.New("the range adds " + path + ", which cannot be a fragment: " + problem)
		}
		text, present, problem := committedText(events.Repo, events.ToCommit, path)
		if problem != "" {
			return weekly.SectionBody{}, errors.New("read news fragment " + path + " at " + shortSHA(events.ToCommit) + ": " + problem)
		}
		if !present {
			return weekly.SectionBody{}, errors.New("git lists news fragment " + path + " as added by " +
				shortSHA(events.ToCommit) + ", and that commit does not hold it")
		}
		// The vocabulary is read once, from the same commit, the way the
		// commit gate reads it from the tree it judges.
		if vocabulary == nil {
			loaded, err := newsTopicVocabulary(events.Repo, events.ToCommit)
			if err != nil {
				return weekly.SectionBody{}, fmt.Errorf("read the topic vocabulary at %s: %w", shortSHA(events.ToCommit), err)
			}
			vocabulary = loaded
		}
		fragment, err := parseNewsFragment(path, text, vocabulary)
		if err != nil {
			return weekly.SectionBody{}, fmt.Errorf("a news fragment added in the range does not parse at %s: %w",
				shortSHA(events.ToCommit), err)
		}
		if fragment.foundBy == "" {
			unattributed++
			continue
		}
		owners[fragment.foundBy] = append(owners[fragment.foundBy], newsFragmentOwner(fragment))
	}
	return weeklyFragmentsBody(owners, unattributed)
}

// weeklyFragmentsBody answers one fact per declared found-by value, in the
// order newsFoundBy declares them, then the total with no found-by.
func weeklyFragmentsBody(owners map[string][]string, unattributed int) (weekly.SectionBody, error) {
	facts := make([]weekly.Fact, 0, len(newsFoundBy)+1)
	for _, foundBy := range newsFoundBy {
		listed := owners[foundBy]
		facts = append(facts, weekly.Fact{Name: "found by " + foundBy, Value: strconv.Itoa(len(listed)), Items: listed})
		delete(owners, foundBy)
	}
	// parseNewsFragment refuses a value outside newsFoundBy, so one left here
	// is a defect that would otherwise vanish from the section.
	if len(owners) != 0 {
		return weekly.SectionBody{}, errors.New("BUG: parsed news fragments carry found-by " +
			strings.Join(slices.Sorted(maps.Keys(owners)), ", ") + ", which newsFoundBy does not declare")
	}
	facts = append(facts, weekly.Fact{Name: "with no found-by", Value: strconv.Itoa(unattributed)})
	return weekly.SectionBody{Note: weeklyFragmentsNote, Facts: facts}, nil
}

// weeklyAddedFragments answers the news fragment paths the second revision
// holds and the first does not, sorted as git lists them. Renames are not
// followed: a fragment moved into the directory is added there.
func weeklyAddedFragments(ctx context.Context, events weekly.EventRange) ([]string, error) {
	output, err := gitOutputContext(ctx, events.Repo, "diff", "--no-renames", "--diff-filter=A", "--name-only", "-z",
		events.FromCommit, events.ToCommit, "--", newsFragmentDirectory)
	if err != nil {
		return nil, fmt.Errorf("list the news fragments added in the range: %w", err)
	}
	var paths []string
	for path := range strings.SplitSeq(output, "\x00") {
		if isNewsFragmentPath(path) {
			paths = append(paths, path)
		}
	}
	return paths, nil
}
