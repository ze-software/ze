// Goal: pin the decisions that decide what reaches a public Discord channel --
// where a message is split, what the header says, and which file is refused.
// Method: every case here drives a pure function with a literal string, so a
// boundary is stated rather than approached through a fixture tree.
//
// VALIDATES: a body is split only at a boundary a reader can see, measured in
//            the characters Discord counts; site-only metadata never reaches
//            the channel; a backfilled post says which week it describes.
// PREVENTS: a section arriving in the channel cut in half, an emoji-heavy body
//           being split into more messages than the limit needs, an HTML
//           comment being posted verbatim, and a late post whose Discord
//           timestamp lies about when the work happened.
//
// A mistake in any of these is visible to everyone in the channel and cannot be
// taken back, which is why the boundaries are pinned one below and one above.

package weekly

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseTextKeepsSnapshotMetadataOutOfTheBody(t *testing.T) {
	const want = "Public text before.\n\nPublic text after."
	post, err := ParseText("---\ncovers: 2026-08-10 .. 2026-08-16\nze-stat-snapshot: true\n---\n\n" +
		want + "\n")
	if err != nil {
		t.Fatalf("ParseText: %v", err)
	}

	if got := post.Meta["ze-stat-snapshot"]; got != "true" {
		t.Errorf("ze-stat-snapshot front matter = %q, want %q", got, "true")
	}
	if post.Body != want {
		t.Errorf("body = %q, want %q", post.Body, want)
	}
	if strings.Contains(post.Body, "ze-stat-snapshot") {
		t.Errorf("site-only metadata reached the Discord body: %q", post.Body)
	}
}

func TestParseTextRefusesTheLegacyBodyMarker(t *testing.T) {
	_, err := ParseText("---\ncovers: 2026-08-10 .. 2026-08-16\n---\n\n" +
		"Public text before.\n\n" + statSnapshotMarker + "\n\nPublic text after.\n")

	if err == nil {
		t.Fatal("ParseText accepted the retired HTML marker; it would be posted verbatim")
	}
	for _, want := range []string{"ze-stat-snapshot: true", "front matter"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
}

func TestParseTextRemovesATerminalNoBreakSpace(t *testing.T) {
	// A word processor leaves U+00A0 at the end of a line. Python's str.strip
	// removes it and Go's strings.TrimSpace must too, or the archived body and
	// the sent body stop agreeing on their last character.
	post, err := ParseText("---\ncovers: 2026-08-10 .. 2026-08-16\n---\n\nPublic text.\u00a0\n")
	if err != nil {
		t.Fatalf("ParseText: %v", err)
	}

	if post.Body != "Public text." {
		t.Errorf("body = %q, want %q", post.Body, "Public text.")
	}
	if got := Messages(post.Body); len(got) != 1 || got[0] != "Public text." {
		t.Errorf("Message = %q, want one message %q", got, "Public text.")
	}
}

func TestParseTextRefusesAFileWithNoFrontMatter(t *testing.T) {
	if _, err := ParseText("**📅 Ze Weekly Update**\n\nNo fence at all.\n"); !errors.Is(err, ErrNoFrontMatter) {
		t.Fatalf("ParseText err = %v, want %v", err, ErrNoFrontMatter)
	}
}

func TestParseTextSplitsFrontMatterAtTheFirstColonOnly(t *testing.T) {
	post, err := ParseText("---\ncovers: 2026-08-10 .. 2026-08-16\nnote: a: b\n---\n\nBody.\n")
	if err != nil {
		t.Fatalf("ParseText: %v", err)
	}
	if got := post.Meta["note"]; got != "a: b" {
		t.Errorf("note = %q, want %q", got, "a: b")
	}
}

func TestCoversReportsAPostThatDeclaresNoWeek(t *testing.T) {
	post, err := ParseText("---\ntitle: nothing\n---\n\nBody.\n")
	if err != nil {
		t.Fatalf("ParseText: %v", err)
	}
	if _, err := post.Covers(); !errors.Is(err, ErrNoCovers) {
		t.Fatalf("Covers err = %v, want %v", err, ErrNoCovers)
	}
}

func TestMessagesKeepWholeSectionsTogetherUnderTheLimit(t *testing.T) {
	body := "**📅 One**\n\nalpha\n\n**🔧 Two**\n\nbeta"

	got := Messages(body)

	if len(got) != 1 {
		t.Fatalf("Message produced %d messages, want 1: %q", len(got), got)
	}
	if got[0] != body {
		t.Errorf("message = %q, want the whole body %q", got[0], body)
	}
}

func TestMessagesStartANewOneAtASectionBoundary(t *testing.T) {
	// Two sections that cannot share a message. The split MUST land between
	// them, never inside one: half a section in the channel reads as a fault.
	first := "**🔧 One**\n\n" + strings.Repeat("a", Limit-20)
	second := "**📦 Two**\n\n" + strings.Repeat("b", Limit-20)

	got := Messages(first + "\n\n" + second)

	if len(got) != 2 {
		t.Fatalf("Message produced %d messages, want 2", len(got))
	}
	if got[0] != first {
		t.Errorf("first message is not the first section whole")
	}
	if got[1] != second {
		t.Errorf("second message is not the second section whole")
	}
}

func TestMessagesSplitAnOversizedSectionAtBlankLines(t *testing.T) {
	paragraph := strings.Repeat("c", Limit-100)
	body := "**🔧 One**\n\n" + paragraph + "\n\n" + paragraph

	got := Messages(body)

	if len(got) != 2 {
		t.Fatalf("Message produced %d messages, want 2: lengths %v", len(got), lengths(got))
	}
	for i, message := range got {
		if charLen(message) > Limit {
			t.Errorf("message %d is %d characters, over the %d limit", i+1, charLen(message), Limit)
		}
		if strings.Contains(message, "\n\n\n") {
			t.Errorf("message %d carries a blank-line boundary that was not split cleanly", i+1)
		}
	}
}

func TestMessagesAreMeasuredInCharactersNotBytes(t *testing.T) {
	// Discord counts characters. A weekly update is full of emoji, so a byte
	// count splits it into more messages than the channel needs -- and every
	// extra message is another rate-limit refusal on a post that is already at
	// the edge of the limit.
	//
	// Each rune below is four bytes, so this body is exactly at the character
	// limit and at four times the limit in bytes.
	section := "**📅 H**\n\n" + strings.Repeat("📦", Limit-9)
	if charLen(section) != Limit {
		t.Fatalf("the fixture is %d characters, want exactly %d", charLen(section), Limit)
	}
	if len(section) <= Limit {
		t.Fatalf("the fixture is %d bytes, which does not exercise the byte/character split", len(section))
	}

	got := Messages(section)

	if len(got) != 1 {
		t.Fatalf("Message produced %d messages for a body at exactly the character limit, want 1", len(got))
	}
}

func TestMessagesSplitOneCharacterOverTheLimit(t *testing.T) {
	// The boundary from the other side: one character more than fits.
	section := "**📅 H**\n\n" + strings.Repeat("📦", Limit-8)
	if charLen(section) != Limit+1 {
		t.Fatalf("the fixture is %d characters, want %d", charLen(section), Limit+1)
	}

	if got := Messages(section); len(got) != 2 {
		t.Fatalf("Message produced %d messages one character over the limit, want 2", len(got))
	}
}

func TestAWholeBoldLineIsASectionHeader(t *testing.T) {
	cases := []struct {
		name   string
		line   string
		header bool
	}{
		{"emoji heading", "**📅 Ze Weekly Update**", true},
		{"shortest possible heading", "**a**", true},
		{"four stars is not a heading", "****", false},
		{"bold word inside a sentence", "and **bold** here", false},
		{"opens bold but does not close the line", "**bold and more", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSectionHeader(tc.line); got != tc.header {
				t.Errorf("isSectionHeader(%q) = %v, want %v", tc.line, got, tc.header)
			}
		})
	}
}

func TestApplyDateStampWritesTheWeekIntoTheHeader(t *testing.T) {
	body := "**📅 Ze Weekly Update**\n\nSomething shipped."

	got, found := ApplyDateStamp(body, "2026-08-10")

	if !found {
		t.Fatal("ApplyDateStamp did not find the header it was given")
	}
	const want = "**📅 Ze Weekly Update -- Week of 2026-08-10**\n\nSomething shipped."
	if got != want {
		t.Errorf("stamped body = %q, want %q", got, want)
	}
}

func TestApplyDateStampStampsOnlyTheFirstHeader(t *testing.T) {
	body := "**📅 Ze Weekly Update**\n\nquoted below\n\n**📅 Ze Weekly Update**"

	got, _ := ApplyDateStamp(body, "2026-08-10")

	if strings.Count(got, "Week of") != 1 {
		t.Errorf("ApplyDateStamp stamped %d headers, want 1: %q", strings.Count(got, "Week of"), got)
	}
}

func TestApplyDateStampReportsABodyWithNoHeader(t *testing.T) {
	body := "No header here."

	got, found := ApplyDateStamp(body, "2026-08-10")

	if found {
		t.Error("ApplyDateStamp claims it found a header in a body with none")
	}
	if got != body {
		t.Errorf("body changed to %q, want it returned unchanged", got)
	}
}

func TestStartDateReadsTheFirstTokenOfTheRange(t *testing.T) {
	if got := StartDate("2026-08-10 .. 2026-08-16"); got != "2026-08-10" {
		t.Errorf("StartDate = %q, want %q", got, "2026-08-10")
	}
}

func TestEndDateRefusesACoversWithNoRange(t *testing.T) {
	if _, err := EndDate("2026-08-10"); !errors.Is(err, ErrCoversRange) {
		t.Fatalf("EndDate err = %v, want %v", err, ErrCoversRange)
	}
}

func TestDaysSinceWeekEndIsNegativeWhileTheWeekRuns(t *testing.T) {
	const covers = "2026-08-10 .. 2026-08-16"
	cases := []struct {
		name string
		day  string
		want int
	}{
		{"the day before the week ends", "2026-08-15", -1},
		{"the last day of the week", "2026-08-16", 0},
		{"the day after the week ends", "2026-08-17", 1},
		{"one day past the stale window", "2026-08-24", StaleAfterDays + 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			today, err := time.Parse(time.DateOnly, tc.day)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.day, err)
			}
			got, err := DaysSinceWeekEnd(covers, today)
			if err != nil {
				t.Fatalf("DaysSinceWeekEnd: %v", err)
			}
			if got != tc.want {
				t.Errorf("DaysSinceWeekEnd(%q) = %d, want %d", tc.day, got, tc.want)
			}
		})
	}
}

func lengths(messages []string) []int {
	out := make([]int, len(messages))
	for i, message := range messages {
		out[i] = charLen(message)
	}
	return out
}
