// Goal: prove a weekly update reaches Discord whole, or says how to finish it.
// Method: the transport is a parameter, so every reply Discord can give is a
// value in a table and no test waits, sends, or touches a channel.
//
// VALIDATES: a rate-limited message is retried on the fixed schedule and lands;
//            a refusal that waiting cannot fix stops the post and names the
//            message to restart from; a resumed post sends only what is left;
//            the archive records the text that actually landed.
// PREVENTS: the 2026-08-03 failure -- seven of eight messages in the channel,
//           nothing recording the week as posted, and a re-run that would have
//           repeated the seven that already landed.

package weekly

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// RATE_LIMITED is the whole of what discord.sh gives send() to recognize a rate
// limit by: it echoes the API's `message` field and drops retry_after.
const rateLimited = "error: You are being rate limited."

// recorder stands in for discord.sh, answering each call from a script of
// replies and keeping the text of every send.
type recorder struct {
	replies []SendResult
	sent    []string
}

func (r *recorder) send(_, text string) SendResult {
	r.sent = append(r.sent, text)
	if len(r.replies) == 0 {
		return SendResult{Stdout: "ok\n"}
	}
	reply := r.replies[0]
	r.replies = r.replies[1:]
	return reply
}

// poster builds a publisher whose transport is the recorder and whose waits are
// recorded rather than taken. DiscordSh names a file that exists, exactly as
// the Python test points it at its own source file.
func poster(t *testing.T, rec *recorder, slept *[]time.Duration) *publisher {
	t.Helper()
	return &publisher{
		Channel:    "ze-test",
		Send:       rec.send,
		DiscordSh:  filepath.Join(t.TempDir(), "discord.sh"),
		Sleep:      func(d time.Duration) { *slept = append(*slept, d) },
		Today:      day(t, "2026-08-20"),
		ArchiveDir: t.TempDir(),
		Progress:   &bytes.Buffer{},
	}
}

func day(t *testing.T, iso string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, iso)
	if err != nil {
		t.Fatalf("parse %q: %v", iso, err)
	}
	return parsed
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestRateLimitedMessageIsRetriedUntilItLands(t *testing.T) {
	// Without the retry this is the 2026-08-03 failure: message 3 never reaches
	// the channel and the post is left incomplete.
	rec := &recorder{replies: []SendResult{
		{Stdout: "ok"}, {Stdout: "ok"},
		{Code: 1, Stdout: rateLimited}, {Code: 1, Stdout: rateLimited}, {Stdout: "ok"},
	}}
	var slept []time.Duration
	p := poster(t, rec, &slept)
	touch(t, p.DiscordSh)

	sent, err := p.send([]string{"one", "two", "three"}, 1)

	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if sent != 3 {
		t.Errorf("send reported %d messages sent, want 3", sent)
	}
	wantSent := []string{"one", "two", "three", "three", "three"}
	if !equal(rec.sent, wantSent) {
		t.Errorf("sent %q, want %q", rec.sent, wantSent)
	}
	if !equalWaits(slept, rateLimitBackoff[:2]) {
		t.Errorf("waited %v, want the first two of the schedule %v", slept, rateLimitBackoff[:2])
	}
}

func TestRetriesAreBounded(t *testing.T) {
	// A channel that never lets up must end the run rather than wait for ever,
	// and it must still name the resume point.
	replies := []SendResult{{Stdout: "ok"}}
	for range 20 {
		replies = append(replies, SendResult{Code: 1, Stdout: rateLimited})
	}
	rec := &recorder{replies: replies}
	var slept []time.Duration
	p := poster(t, rec, &slept)
	touch(t, p.DiscordSh)

	_, err := p.send([]string{"one", "two"}, 1)

	if err == nil {
		t.Fatal("send waited out an unending rate limit instead of stopping")
	}
	if len(slept) != len(rateLimitBackoff) {
		t.Errorf("waited %d times, want the whole schedule of %d", len(slept), len(rateLimitBackoff))
	}
}

func TestOtherFailuresStopThePostAndNameTheResumePoint(t *testing.T) {
	// A refusal waiting cannot fix is not retried, and the operator is told
	// which message to restart from -- message 3, the one that failed, never the
	// top.
	rec := &recorder{replies: []SendResult{
		{Stdout: "ok"}, {Stdout: "ok"}, {Code: 1, Stdout: "error: Missing Access"},
	}}
	var slept []time.Duration
	p := poster(t, rec, &slept)
	touch(t, p.DiscordSh)

	_, err := p.send([]string{"one", "two", "three"}, 1)

	if err == nil {
		t.Fatal("send accepted a permission refusal")
	}
	if _, ok := errors.AsType[*sendFailed](err); !ok {
		t.Fatalf("send err = %T, want *sendFailed", err)
	}
	for _, want := range []string{"resume-from 3", "messages 1 to 2", "ze-test"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
	if len(slept) != 0 {
		t.Errorf("a permission error was retried after %v", slept)
	}
}

func TestResumeFromSkipsWhatAlreadyLanded(t *testing.T) {
	// Finishing a half-sent post must send message 3 alone. Sending all three
	// again is the duplicate the resume point exists to prevent.
	rec := &recorder{replies: []SendResult{{Stdout: "ok"}}}
	var slept []time.Duration
	p := poster(t, rec, &slept)
	touch(t, p.DiscordSh)

	if _, err := p.send([]string{"one", "two", "three"}, 3); err != nil {
		t.Fatalf("send: %v", err)
	}

	if !equal(rec.sent, []string{"three"}) {
		t.Errorf("sent %q, want only the last message", rec.sent)
	}
}

func TestSendRefusesBeforeAnyMessageWhenTheTransportIsAbsent(t *testing.T) {
	// Nothing is sent, so nothing has to be resumed. The check is before the
	// loop for that reason.
	rec := &recorder{}
	var slept []time.Duration
	p := poster(t, rec, &slept)

	_, err := p.send([]string{"one"}, 1)

	if err == nil {
		t.Fatal("send ran with no discord.sh")
	}
	if len(rec.sent) != 0 {
		t.Errorf("sent %q with no transport", rec.sent)
	}
	if !strings.Contains(err.Error(), p.DiscordSh) {
		t.Errorf("the refusal does not name the path it looked at: %v", err)
	}
}

func TestSendReadsSuccessAsBothTheCodeAndTheWord(t *testing.T) {
	// discord.sh has exited 0 while reporting a refusal, so a zero code alone
	// is not success here. Both halves are required, and each is pinned.
	//
	// A rate limit is answered on every attempt here, which is what makes the
	// row assert the retry rather than the recorder's default reply.
	cases := []struct {
		name    string
		reply   SendResult
		lands   bool
		retried bool
	}{
		{"code zero and ok", SendResult{Stdout: "ok"}, true, false},
		{"code zero without ok", SendResult{Stdout: "queued"}, false, false},
		{"ok with a non-zero code", SendResult{Code: 1, Stdout: "ok"}, false, false},
		{"rate limit on stderr only", SendResult{Code: 1, Stderr: rateLimited}, false, true},
		{"rate limit in another case", SendResult{Code: 1, Stdout: "Rate Limited"}, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			replies := make([]SendResult, len(rateLimitBackoff)+1)
			for i := range replies {
				replies[i] = tc.reply
			}
			rec := &recorder{replies: replies}
			var slept []time.Duration
			p := poster(t, rec, &slept)
			touch(t, p.DiscordSh)

			_, err := p.send([]string{"one"}, 1)

			if tc.lands != (err == nil) {
				t.Errorf("send err = %v, want landed = %v", err, tc.lands)
			}
			if tc.retried != (len(slept) > 0) {
				t.Errorf("waited %v, want retried = %v", slept, tc.retried)
			}
		})
	}
}

func TestArchiveRecordsWhatLandedRatherThanTheSource(t *testing.T) {
	var slept []time.Duration
	p := poster(t, &recorder{}, &slept)

	path, err := p.archive("2026-08-10", "2026-08-10 .. 2026-08-16", "**📅 Ze Weekly Update -- Week of 2026-08-10**", true)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := "---\nposted: 2026-08-20\nchannel: ze-test\ncovers: 2026-08-10 .. 2026-08-16\n" +
		"backfilled: true\n---\n\n**📅 Ze Weekly Update -- Week of 2026-08-10**\n"
	if string(got) != want {
		t.Errorf("archive =\n%q\nwant\n%q", got, want)
	}
	if filepath.Base(path) != "2026-08-10-weekly.md" {
		t.Errorf("archive file = %q, want it named for the week it starts", filepath.Base(path))
	}
}

func TestArchiveOmitsTheBackfilledLineForAPostInItsOwnWeek(t *testing.T) {
	var slept []time.Duration
	p := poster(t, &recorder{}, &slept)

	path, err := p.archive("2026-08-10", "2026-08-10 .. 2026-08-16", "body", false)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(got), "backfilled") {
		t.Errorf("an unstamped post was archived as backfilled: %q", got)
	}
}

func TestFindDiscordShPrefersTheEnvironmentOverride(t *testing.T) {
	t.Setenv("DISCORD_SH", "/somewhere/discord.sh")
	resetEnvCache(t)

	if got := resolveDiscordSh(); got != "/somewhere/discord.sh" {
		t.Errorf("resolveDiscordSh = %q, want the override", got)
	}
}

func TestFindDiscordShTakesTheSecondCandidateWhenTheFirstIsAbsent(t *testing.T) {
	// The machine that exited "not found": ~/Unix/bin holds nothing and ~/bin
	// holds discord.sh. A single hardcoded default fails there.
	t.Setenv("DISCORD_SH", "")
	resetEnvCache(t)
	home := t.TempDir()
	absent := filepath.Join(home, "Unix", "bin", "discord.sh")
	present := filepath.Join(home, "bin", "discord.sh")
	if err := os.MkdirAll(filepath.Dir(present), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	touch(t, present)

	if got := findDiscordSh([]string{absent, present}); got != present {
		t.Errorf("findDiscordSh = %q, want %q", got, present)
	}
}

func TestFindDiscordShFallsBackToTheFirstCandidateSoTheErrorNamesAPath(t *testing.T) {
	t.Setenv("DISCORD_SH", "")
	resetEnvCache(t)
	// An empty PATH so the search reaches its last branch. On a machine that
	// carries discord.sh on PATH the branch above answers first, which is the
	// behavior the next case pins.
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	first := filepath.Join(home, "Unix", "bin", "discord.sh")
	second := filepath.Join(home, "bin", "discord.sh")

	if got := findDiscordSh([]string{first, second}); got != first {
		t.Errorf("findDiscordSh = %q, want %q so the refusal names a path", got, first)
	}
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func equalWaits(got, want []time.Duration) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestFindDiscordShTakesWhatIsOnPathWhenNoCandidateHasIt(t *testing.T) {
	t.Setenv("DISCORD_SH", "")
	resetEnvCache(t)
	onPath := t.TempDir()
	touch(t, filepath.Join(onPath, "discord.sh"))
	t.Setenv("PATH", onPath)
	absent := filepath.Join(t.TempDir(), "Unix", "bin", "discord.sh")

	if got := findDiscordSh([]string{absent}); got != filepath.Join(onPath, "discord.sh") {
		t.Errorf("findDiscordSh = %q, want the one on PATH", got)
	}
}
