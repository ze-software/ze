package spec

import (
	"testing"
)

// The helpers below run one line through a member's action table and then the
// action's body, with the checkout root and the session passed in rather than
// read from the working directory and the environment, so a fixture tree
// stands in for the checkout. A line the table's grammar refuses answers 2, as
// leaction.Area.Answer does.

// reviewLineSession is the session every review line runs under: the
// fixtures record and check one review, so one session stands for the caller.
const reviewLineSession = "session"

// answerReviewLine runs one `le spec review` line as reviewLineSession.
func answerReviewLine(t *testing.T, root string, line []string) (any, int) {
	t.Helper()
	sid := reviewLineSession
	parsed, err := ReviewArea().Parse(line[0], line[1:])
	if err != nil {
		return nil, 2
	}
	switch line[0] {
	case verbHash:
		return answerReviewHash(root, parsed)
	case verbRecord:
		return answerReviewRecord(root, sid, parsed)
	case verbCheck:
		return answerReviewCheck(root, sid, parsed)
	}
	t.Fatalf("spec review holds no action %q", line[0])
	return nil, 0
}

// answerModelLine runs one `le spec model` line.
func answerModelLine(t *testing.T, root string, line []string) (any, int) {
	t.Helper()
	parsed, err := ModelArea().Parse(line[0], line[1:])
	if err != nil {
		return nil, 2
	}
	return answerModel(root, parsed)
}

// parseReviewRecordLine reads the words after `le spec review record` into the
// record request, as the action does before it records.
func parseReviewRecordLine(args []string) (reviewRecord, error) {
	parsed, err := ReviewArea().Parse(verbRecord, args)
	if err != nil {
		return reviewRecord{}, err
	}
	return reviewRecordOf(parsed)
}

// TestTheSpecMembersKeepTheirSpellings checks that each spelling a hook, a
// skill or a page types still reads through its member's table: the rename
// phase of plan/post-release/engineering/spec-le-cli-usability.md changes
// spellings, and this phase must not. Method: parse each line through the
// table and run nothing.
//
// VALIDATES: the tables of spec claim, state, model and review read the lines
// their callers type today.
// PREVENTS: a migration that answers a hook's `spec claim spec <file>` with a
// usage refusal.
func TestTheSpecMembersKeepTheirSpellings(t *testing.T) {
	lines := []struct {
		name  string
		parse func() error
	}{
		{"spec claim spec <file>", func() error {
			parsed, err := ClaimArea().Parse(keywordSpec, []string{"plan/x/spec-y.md"})
			if err == nil && parsed.One(keywordSpec) != "plan/x/spec-y.md" {
				t.Errorf("claim read the spec as %q", parsed.One(keywordSpec))
			}
			return err
		}},
		{"spec state latest spec <stem>", func() error {
			_, err := StateArea().Parse(verbLatest, []string{"spec", "spec-y"})
			return err
		}},
		{"spec model current transcript <path>", func() error {
			_, err := ModelArea().Parse(verbCurrent, []string{"transcript", "/abs/t.jsonl"})
			return err
		}},
		{"spec review record ... hunks <path> <patch>", func() error {
			_, err := ReviewArea().Parse(verbRecord, []string{
				"spec", "demo", "verdict", "clean", "rounds", "1",
				"file", "a.go", "file", "b.go", "hunks", "c.go", "c.patch",
			})
			return err
		}},
	}
	for _, line := range lines {
		if err := line.parse(); err != nil {
			t.Errorf("%s is refused: %v", line.name, err)
		}
	}
}
