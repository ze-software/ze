// Goal: pin which weeks are posted, which are left alone, and what an operator
// sees before anything leaves the machine.
// Method: a temporary posts directory and a temporary archive, a recorded
// transport and recorded waits, so a whole sweep runs in microseconds and
// touches no channel.
//
// VALIDATES: a week already archived is never sent twice; a week still running
//            is never sent early; two posts in one sweep are separated; a plan
//            sends nothing; the archive records the text that landed.
// PREVENTS: the duplicate post, the half-week post, two weeks collapsing into
//           one wall of text in the channel, and a dry run that posts.

package weekly

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

// resetEnvCache makes env.Get read the variables this test set. env caches
// os.Environ() once per process, so without this a t.Setenv is invisible.
func resetEnvCache(t *testing.T) {
	t.Helper()
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// sweep builds a Poster over a temporary posts directory, and answers both.
func sweep(t *testing.T, rec *recorder, slept *[]time.Duration, today string) (*Poster, string) {
	t.Helper()
	p := &Poster{
		Channel:    "ze-test",
		Send:       rec.send,
		DiscordSh:  filepath.Join(t.TempDir(), "discord.sh"),
		Sleep:      func(d time.Duration) { *slept = append(*slept, d) },
		Today:      day(t, today),
		ArchiveDir: t.TempDir(),
		Progress:   &bytes.Buffer{},
	}
	touch(t, p.DiscordSh)
	return p, t.TempDir()
}

// writePost writes one weekly source file and answers its path.
func writePost(t *testing.T, dir, start, end, body string) string {
	t.Helper()
	var text strings.Builder
	text.WriteString("---\ncovers: ")
	text.WriteString(start)
	text.WriteString(" .. ")
	text.WriteString(end)
	text.WriteString("\n---\n\n")
	text.WriteString(body)
	text.WriteString("\n")

	path := filepath.Join(dir, start+".md")
	if err := os.WriteFile(path, []byte(text.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

const shortBody = "**📅 Ze Weekly Update**\n\nOne short section."

func TestPlanSendsNothing(t *testing.T) {
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-20")
	writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)

	report, err := p.Run(Options{Dir: dir})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rec.sent) != 0 {
		t.Fatalf("a plan sent %q", rec.sent)
	}
	if len(report.Posts) != 1 || report.Posts[0].Status != StatusPlanned {
		t.Fatalf("report = %+v, want one planned post", report.Posts)
	}
	if len(report.Posts[0].Messages) != 1 {
		t.Errorf("the plan names %d messages, want 1", len(report.Posts[0].Messages))
	}
	if entries, _ := os.ReadDir(p.ArchiveDir); len(entries) != 0 {
		t.Errorf("a plan wrote %d archive file(s)", len(entries))
	}
}

func TestSweepSkipsAWeekAlreadyArchived(t *testing.T) {
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-20")
	writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)
	if err := os.WriteFile(filepath.Join(p.ArchiveDir, "2026-08-10-weekly.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed the archive: %v", err)
	}

	report, err := p.Run(Options{Confirm: true, Dir: dir})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rec.sent) != 0 {
		t.Fatalf("an archived week was posted a second time: %q", rec.sent)
	}
	if got := report.Posts[0].Status; got != StatusSkipped {
		t.Errorf("status = %q, want %q", got, StatusSkipped)
	}
	if !strings.Contains(report.Posts[0].Reason, "archived") {
		t.Errorf("reason = %q, want it to name the archive", report.Posts[0].Reason)
	}
}

func TestSweepSkipsAWeekThatHasNotEnded(t *testing.T) {
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-14")
	writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)

	report, err := p.Run(Options{Confirm: true, Dir: dir})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rec.sent) != 0 {
		t.Fatalf("a week that had not ended was posted: %q", rec.sent)
	}
	if got := report.Posts[0].Status; got != StatusSkipped {
		t.Errorf("status = %q, want %q", got, StatusSkipped)
	}
}

func TestSweepPostsOnTheDayTheWeekEnds(t *testing.T) {
	// The boundary from the other side: day zero is over, so the week ships.
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-16")
	writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)

	report, err := p.Run(Options{Confirm: true, Dir: dir})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rec.sent) != 1 {
		t.Fatalf("sent %d messages on the last day of the week, want 1", len(rec.sent))
	}
	if got := report.Posts[0].Status; got != StatusSent {
		t.Errorf("status = %q, want %q", got, StatusSent)
	}
}

func TestSweepSeparatesTwoPostsSoDiscordDoesNotGroupThem(t *testing.T) {
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-20")
	writePost(t, dir, "2026-08-03", "2026-08-09", shortBody)
	writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)

	report, err := p.Run(Options{Confirm: true, Dir: dir})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Posts) != 2 {
		t.Fatalf("posted %d weeks, want 2", len(report.Posts))
	}
	if report.Posts[0].Date != "2026-08-03" {
		t.Errorf("the sweep ran out of order: first post is %q", report.Posts[0].Date)
	}
	if !equalWaits(slept, []time.Duration{SendDelay}) {
		t.Errorf("waits = %v, want exactly one %v between the two posts", slept, SendDelay)
	}
}

func TestSweepWaitsForNothingWhenItPostsOneWeek(t *testing.T) {
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-20")
	writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)

	if _, err := p.Run(Options{Confirm: true, Dir: dir}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(slept) != 0 {
		t.Errorf("waited %v before the only post of the sweep", slept)
	}
}

func TestSweepRefusesADirectoryWithNoPosts(t *testing.T) {
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-20")

	_, err := p.Run(Options{Dir: dir})

	if err == nil {
		t.Fatal("an empty directory was accepted; a typo in a path would look like nothing to do")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("the refusal does not name the directory: %v", err)
	}
}

func TestSingleFileRefusesAWeekStillRunning(t *testing.T) {
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-14")
	source := writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)

	_, err := p.Run(Options{Confirm: true, Source: source})

	if err == nil {
		t.Fatal("a week that had not ended was posted on an explicit request")
	}
	if len(rec.sent) != 0 {
		t.Fatalf("sent %q", rec.sent)
	}
	for _, want := range []string{"2026-08-16", "force"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
}

func TestForceOverridesTheUnfinishedWeek(t *testing.T) {
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-14")
	source := writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)

	report, err := p.Run(Options{Confirm: true, Source: source, Force: true})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rec.sent) != 1 {
		t.Fatalf("sent %d messages under force, want 1", len(rec.sent))
	}
	if got := report.Posts[0].Status; got != StatusSent {
		t.Errorf("status = %q, want %q", got, StatusSent)
	}
}

func TestDateStampIsAutomaticOnlyPastTheStaleWindow(t *testing.T) {
	cases := []struct {
		name    string
		today   string
		stamped bool
	}{
		{"inside the window", "2026-08-23", false},
		{"the last day inside the window", "2026-08-23", false},
		{"one day past the window", "2026-08-24", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			var slept []time.Duration
			p, dir := sweep(t, rec, &slept, tc.today)
			source := writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)

			report, err := p.Run(Options{Source: source})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}

			if got := report.Posts[0].DateStamped; got != tc.stamped {
				t.Errorf("date-stamped = %v on %s, want %v", got, tc.today, tc.stamped)
			}
			carries := strings.Contains(report.Posts[0].Messages[0].Text, "Week of 2026-08-10")
			if carries != tc.stamped {
				t.Errorf("the header carries the week = %v, want %v", carries, tc.stamped)
			}
		})
	}
}

func TestDateStampCanBeForcedEitherWay(t *testing.T) {
	for _, tc := range []struct {
		name    string
		stamp   Stamp
		stamped bool
	}{
		{"forced on inside the window", StampOn, true},
		{"forced off past the window", StampOff, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			var slept []time.Duration
			today := "2026-08-20"
			if tc.stamp == StampOff {
				today = "2026-09-20"
			}
			p, dir := sweep(t, rec, &slept, today)
			source := writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)

			report, err := p.Run(Options{Source: source, Stamp: tc.stamp})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}

			if got := report.Posts[0].DateStamped; got != tc.stamped {
				t.Errorf("date-stamped = %v, want %v", got, tc.stamped)
			}
		})
	}
}

func TestResumeFromPastTheLastMessageIsRefusedBeforeAnythingIsSent(t *testing.T) {
	// The resume point names a message of a specific post, so a number past the
	// end of that post is a mistake the operator must see before a send.
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-20")
	source := writePost(t, dir, "2026-08-03", "2026-08-09", shortBody)

	_, err := p.Run(Options{Confirm: true, Source: source, ResumeFrom: 9})

	if err == nil {
		t.Fatal("a resume point past the last message was accepted")
	}
	if len(rec.sent) != 0 {
		t.Fatalf("sent %q before the refusal", rec.sent)
	}
}

func TestPostArchivesTheTextThatLanded(t *testing.T) {
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-09-20")
	source := writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)

	report, err := p.Run(Options{Confirm: true, Source: source})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	archived, err := os.ReadFile(report.Posts[0].Archive)
	if err != nil {
		t.Fatalf("read the archive: %v", err)
	}
	if len(rec.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(rec.sent))
	}
	// The post is months late, so it was stamped. The archive must hold the
	// STAMPED text: it is the record of what is in the channel, not a copy of
	// the source file.
	if !strings.Contains(string(archived), "Week of 2026-08-10") {
		t.Errorf("the archive holds the source text rather than what was sent:\n%s", archived)
	}
	if !strings.Contains(string(archived), rec.sent[0]) {
		t.Errorf("the archive and the channel disagree:\narchive %s\nsent %q", archived, rec.sent[0])
	}
}

func TestPostReportsEveryMessageAsSent(t *testing.T) {
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-20")
	source := writePost(t, dir, "2026-08-10", "2026-08-16", shortBody)

	report, err := p.Run(Options{Confirm: true, Source: source})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, message := range report.Posts[0].Messages {
		if !message.Sent {
			t.Errorf("message %d is reported unsent after a clean post", message.Number)
		}
	}
}

func TestAFailedPostReportsWhatDidLand(t *testing.T) {
	// The answer must still say what is in the channel, because that is what
	// the operator needs to finish the post.
	rec := &recorder{replies: []SendResult{{Stdout: "ok"}, {Code: 1, Stdout: "error: Missing Access"}}}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-08-20")
	body := "**📅 One**\n\n" + strings.Repeat("a", Limit-20) + "\n\n**🔧 Two**\n\n" + strings.Repeat("b", Limit-20)
	source := writePost(t, dir, "2026-08-10", "2026-08-16", body)

	report, err := p.Run(Options{Confirm: true, Source: source})

	if _, ok := errors.AsType[*SendFailed](err); !ok {
		t.Fatalf("Run err = %v, want a *SendFailed", err)
	}
	messages := report.Posts[0].Messages
	if len(messages) != 2 {
		t.Fatalf("the report names %d messages, want 2", len(messages))
	}
	if !messages[0].Sent || messages[1].Sent {
		t.Errorf("sent flags = %v, %v; want the first landed and the second not",
			messages[0].Sent, messages[1].Sent)
	}
	if report.Posts[0].Archive != "" {
		t.Error("a half-sent post was archived, which would mark the week as done")
	}
}

func TestABodyWithNoHeaderIsStampedWithAWarningRatherThanRefused(t *testing.T) {
	rec := &recorder{}
	var slept []time.Duration
	p, dir := sweep(t, rec, &slept, "2026-09-20")
	progress := &bytes.Buffer{}
	p.Progress = progress
	source := writePost(t, dir, "2026-08-10", "2026-08-16", "No header at all.")

	report, err := p.Run(Options{Source: source})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Posts[0].Messages) != 1 {
		t.Fatalf("the plan names %d messages, want 1", len(report.Posts[0].Messages))
	}
	if !strings.Contains(progress.String(), "warning") {
		t.Errorf("nothing warned that the header was missing: %q", progress.String())
	}
}
