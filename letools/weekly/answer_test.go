// Goal: pin the grammar an operator types, and prove the answer is data rather
// than text somebody already formatted.
// Method: the parser is a function over argv, so every refusal is a table row.
//
// VALIDATES: a keyword always comes before a value; the bare command publishes
//            nothing; a modifier that belongs to one post is refused on a
//            sweep; resume-from is 1-based at its boundary; the answer
//            round-trips through JSON with kebab-case keys.
// PREVENTS: a post file whose name collides with a keyword, a sweep silently
//           forcing every unfinished week, an off-by-one resume that repeats a
//           message already in the channel, and a tool that renders its own
//           JSON.

package weekly

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseOptionsReadsTheWholeGrammar(t *testing.T) {
	opts, err := parseOptions([]string{
		"source", "/posts/2026-08-10.md", "channel", "ze-test",
		"confirm", "resume-from", "3", "force", "date-stamp",
	}, "/default/dir")
	if err != nil {
		t.Fatalf("parseOptions: %v", err)
	}

	if !opts.Confirm {
		t.Error("confirm was not read, so the run would publish nothing")
	}
	if opts.Source != "/posts/2026-08-10.md" {
		t.Errorf("source = %q", opts.Source)
	}
	if opts.Channel != "ze-test" {
		t.Errorf("channel = %q", opts.Channel)
	}
	if opts.ResumeFrom != 3 {
		t.Errorf("resume-from = %d", opts.ResumeFrom)
	}
	if !opts.Force {
		t.Error("force was not read")
	}
	if opts.Stamp != StampOn {
		t.Errorf("stamp = %v, want StampOn", opts.Stamp)
	}
}

func TestTheBareCommandIsASweepThatPublishesNothing(t *testing.T) {
	opts, err := parseOptions(nil, "/default/dir")
	if err != nil {
		t.Fatalf("parseOptions: %v", err)
	}

	if opts.Confirm {
		t.Error("the bare command publishes; a mistyped invocation would post")
	}
	if opts.Dir != "/default/dir" {
		t.Errorf("dir = %q, want the default", opts.Dir)
	}
	if opts.Channel != "ze-news" {
		t.Errorf("channel = %q, want ze-news", opts.Channel)
	}
	if opts.Stamp != StampAuto {
		t.Errorf("stamp = %v, want StampAuto", opts.Stamp)
	}
}

func TestParseOptionsRefusesWhatTheGrammarDoesNotAllow(t *testing.T) {
	cases := []struct {
		name string
		args []string
		says string
	}{
		{"a value with no keyword to type it", []string{"/posts/2026-08-10.md"}, "unknown keyword"},
		{"a keyword nobody registered", []string{"publish"}, "publish"},
		{"a keyword with no value", []string{"source"}, "source"},
		{"a channel that is not a channel", []string{"channel", "general"}, "general"},
		{"source and dir together", []string{"source", "/a.md", "dir", "/b"}, "dir"},
		{"resume-from on a sweep", []string{"confirm", "resume-from", "2"}, "source"},
		{"force on a sweep", []string{"confirm", "force"}, "source"},
		{"resume-from that is not a number", []string{"source", "/a.md", "resume-from", "x"}, "x"},
		{"both date-stamp choices", []string{"date-stamp", "no-date-stamp"}, "date-stamp"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseOptions(tc.args, "/default/dir")

			if err == nil {
				t.Fatalf("parseOptions(%q) was accepted", tc.args)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("the refusal does not say %q: %v", tc.says, err)
			}
		})
	}
}

func TestResumeFromIsOneBasedAtItsBoundary(t *testing.T) {
	// The messages are numbered from 1, so 0 is the off-by-one that would
	// resend a message already in the channel. 1 is the first valid value and
	// means the beginning.
	cases := []struct {
		value  string
		valid  bool
		wanted int
	}{
		{"-1", false, 0},
		{"0", false, 0},
		{"1", true, 1},
		{"2", true, 2},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			opts, err := parseOptions([]string{"source", "/a.md", "resume-from", tc.value}, "/default/dir")

			if tc.valid != (err == nil) {
				t.Fatalf("parseOptions(resume-from %s) err = %v, want valid = %v", tc.value, err, tc.valid)
			}
			if tc.valid && opts.ResumeFrom != tc.wanted {
				t.Errorf("resume-from = %d, want %d", opts.ResumeFrom, tc.wanted)
			}
			if !tc.valid && !strings.Contains(err.Error(), "1-based") {
				t.Errorf("the refusal does not say the numbering starts at one: %v", err)
			}
		})
	}
}

func TestEveryWordOfTheGrammarIsAClosedKeyword(t *testing.T) {
	// The mechanical check from ai/rules/cli.md: no free-form value sits in an
	// untyped slot, so every accepted word is either a keyword or the value of
	// the keyword before it.
	bare := []string{"confirm", "force", "date-stamp", "no-date-stamp"}
	for _, word := range bare {
		if _, err := parseOptions([]string{word, "source", "/a.md"}, "/default/dir"); err != nil {
			t.Errorf("%q is not accepted as a keyword: %v", word, err)
		}
	}

	valued := []string{"source", "dir", "channel", "resume-from"}
	for _, word := range valued {
		if _, err := parseOptions([]string{word}, "/default/dir"); err == nil {
			t.Errorf("%q was accepted with no value, so a value could sit in an untyped slot", word)
		}
	}
}

func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	report := Report{
		Action:  StatusPlanned,
		Channel: "ze-news",
		Posts: []PostReport{{
			Source: "/posts/2026-08-10.md", Date: "2026-08-10",
			Covers: "2026-08-10 .. 2026-08-16", Status: StatusPlanned, DateStamped: true,
			Messages: []Message{{Number: 1, Chars: 12, Text: "**📅 hello**"}},
		}},
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	posts, ok := decoded["posts"].([]any)
	if !ok || len(posts) != 1 {
		t.Fatalf("posts = %v, want one row", decoded["posts"])
	}
	row, ok := posts[0].(map[string]any)
	if !ok {
		t.Fatalf("a post row is %T, want an object", posts[0])
	}
	if _, present := row["date-stamped"]; !present {
		t.Errorf("the row has no kebab-case date-stamped key: %v", row)
	}
	if _, present := row["archive"]; present {
		t.Error("an unpublished post carries an archive key, which reads as a missing value")
	}
}

func TestTextRendersAPlanAsTheMessagesAndAPublicationAsOneLine(t *testing.T) {
	plan := Report{Action: StatusPlanned, Channel: "ze-news", Posts: []PostReport{{
		Date: "2026-08-10", Covers: "2026-08-10 .. 2026-08-16", Status: StatusPlanned,
		Messages: []Message{{Number: 1, Chars: 5, Text: "hello"}},
	}}}

	if got := plan.Text(); !strings.Contains(got, "hello") {
		t.Errorf("a plan does not show the message an operator is reviewing:\n%s", got)
	}

	posted := plan
	posted.Posts[0].Status = StatusSent
	posted.Posts[0].Archive = "/archive/2026-08-10-weekly.md"

	got := posted.Text()
	if strings.Contains(got, "hello") {
		t.Errorf("a finished post repeats the message text that already went past:\n%s", got)
	}
	if !strings.Contains(got, "/archive/2026-08-10-weekly.md") {
		t.Errorf("a finished post does not name its archive:\n%s", got)
	}
}
