// The migration's proof for this tool: the script and the command publish the
// same thing.
//
// scripts/zeledon/post_weekly.py is being replaced by internal/le/weekly, and the
// two live side by side until the swap. This file is deliberately HERE rather
// than beside the new package: it is a migration artifact, so the commit that
// deletes the script deletes its proof with it.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- over the 37 real weekly posts in
// this checkout, the script and the command send byte-identical messages in the
// same order, write byte-identical archives, and answer the same exit code.
// PREVENTS: a silent change in what reaches a public Discord channel. A message
// there cannot be taken back, and the failure this port could introduce is not
// a crash: it is a body split one message differently, which every unit test on
// both sides would pass.
//
// The comparison is over the SIDE EFFECT rather than over the report. Both
// implementations are pointed at a recording stand-in for discord.sh and at a
// temporary archive, so what is compared is the argv that would have reached
// the channel. That is a stronger statement than a diff of two stdouts, and it
// is the only one available here: the two report their plans in their own
// words, and the command's words name its own grammar rather than the script's
// --yes flag.
//
// Nothing here touches Discord. The stand-in is a three-line shell script in a
// temporary directory, and the script is COPIED into that directory so its
// archive (WEEKLY_DIR, derived from __file__) lands there too. A run that wrote
// into the real scripts/zeledon/weekly would mark a real week as published.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/weekly"
)

// runTimeout bounds one Python run over one post. A parse and a message of a
// 10 KB file is milliseconds, so a run past this is a hung process.
const runTimeout = 60 * time.Second

// The separators the recording stand-in writes with. A weekly message carries
// newlines, blank lines and emoji, so the record needs bytes that cannot occur
// in one.
const (
	fieldSep  = "\x1f"
	recordSep = "\x00"
)

// fakeDiscord is the recording stand-in. It answers exactly what the script
// reads success by -- exit 0 with "ok" on stdout -- and writes one record per
// call.
const fakeDiscord = `#!/usr/bin/env bash
channel=""
text=""
while [ $# -gt 0 ]; do
  case "$1" in
    --channel) channel="$2"; shift 2 ;;
    --text) text="$2"; shift 2 ;;
    *) shift ;;
  esac
done
printf '%s\x1f%s\x00' "$channel" "$text" >> "LOGPATH"
echo ok
`

// sent is one message that reached the stand-in.
type sent struct {
	channel string
	text    string
}

// harness is one side-by-side comparison: a copy of the script, a stand-in for
// each implementation, and a temporary archive for each.
type harness struct {
	script     string
	pyFake     string
	pyLog      string
	pyArchive  string
	goFake     string
	goLog      string
	goArchive  string
	postsDir   string
	todayIsUTC time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()

	// The script is copied so WEEKLY_DIR, which it derives from __file__,
	// lands beside the copy rather than in the checkout.
	source, err := os.ReadFile("post_weekly.py")
	if err != nil {
		t.Fatalf("read the script: %v", err)
	}
	script := filepath.Join(dir, "post_weekly.py")
	if err := os.WriteFile(script, source, 0o644); err != nil {
		t.Fatalf("copy the script: %v", err)
	}

	h := &harness{
		script:    script,
		pyLog:     filepath.Join(dir, "python.log"),
		goLog:     filepath.Join(dir, "go.log"),
		pyArchive: filepath.Join(dir, "weekly"),
		goArchive: filepath.Join(t.TempDir(), "weekly"),
		postsDir:  filepath.Join(dir, "posts"),
	}
	h.pyFake = writeFake(t, filepath.Join(dir, "discord-python.sh"), h.pyLog)
	h.goFake = writeFake(t, filepath.Join(dir, "discord-go.sh"), h.goLog)

	if err := os.MkdirAll(h.postsDir, 0o755); err != nil {
		t.Fatalf("mkdir posts: %v", err)
	}

	// The script reads datetime.date.today() and offers no way to set it, so
	// the command is given the same day rather than the script being given a
	// fixture one. Every date in a fixture below is computed from it.
	now := time.Now()
	h.todayIsUTC = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return h
}

func writeFake(t *testing.T, path, logPath string) string {
	t.Helper()
	body := strings.Replace(fakeDiscord, "LOGPATH", logPath, 1)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write the stand-in: %v", err)
	}
	return path
}

// runScript publishes through the script and answers what reached the stand-in.
func (h *harness) runScript(t *testing.T, args ...string) (messages []sent, code int) {
	t.Helper()
	if err := os.Remove(h.pyLog); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clear the log: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", append([]string{h.script}, args...)...)
	cmd.Env = append(os.Environ(), "DISCORD_SH="+h.pyFake)
	output, err := cmd.CombinedOutput()

	var exited *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exited):
		code = exited.ExitCode()
	default:
		t.Fatalf("run the script: %v\n%s", err, output)
	}
	return readLog(t, h.pyLog), code
}

// runCommand publishes through the port and answers what reached its stand-in.
func (h *harness) runCommand(t *testing.T, opts weekly.Options) (messages []sent, code int) {
	t.Helper()
	if err := os.Remove(h.goLog); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clear the log: %v", err)
	}

	poster := &weekly.Poster{
		Channel:    "ze-test",
		Send:       weekly.ExecSender(h.goFake),
		DiscordSh:  h.goFake,
		Sleep:      func(time.Duration) {},
		Today:      h.todayIsUTC,
		ArchiveDir: h.goArchive,
		Progress:   nil,
	}
	if _, err := poster.Run(opts); err != nil {
		code = 1
	}
	return readLog(t, h.goLog), code
}

// readLog answers the messages a stand-in recorded, in order.
func readLog(t *testing.T, path string) []sent {
	t.Helper()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	messages := []sent{}
	for record := range strings.SplitSeq(string(raw), recordSep) {
		if record == "" {
			continue
		}
		channel, text, _ := strings.Cut(record, fieldSep)
		messages = append(messages, sent{channel: channel, text: text})
	}
	return messages
}

// compare states the whole of AC-11 for one run: the same messages, in the same
// order, to the same channel, and the same verdict.
func compare(t *testing.T, what string, script, port []sent, scriptCode, portCode int) {
	t.Helper()
	if scriptCode != portCode {
		t.Errorf("%s: the script exited %d and the command exited %d", what, scriptCode, portCode)
	}
	if len(script) != len(port) {
		t.Fatalf("%s: the script sent %d message(s) and the command sent %d",
			what, len(script), len(port))
	}
	for i := range script {
		if script[i].channel != port[i].channel {
			t.Errorf("%s: message %d went to %q and %q", what, i+1, script[i].channel, port[i].channel)
		}
		if script[i].text != port[i].text {
			t.Errorf("%s: message %d differs\nscript (%d chars): %q\ncommand (%d chars): %q",
				what, i+1, len([]rune(script[i].text)), script[i].text,
				len([]rune(port[i].text)), port[i].text)
		}
	}
}

// writePost writes one fixture post and answers its path.
func (h *harness) writePost(t *testing.T, start, end, body string) string {
	t.Helper()
	text := "---\ncovers: " + start + " .. " + end + "\n---\n\n" + body + "\n"
	path := filepath.Join(h.postsDir, start+".md")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// day answers an ISO date a whole number of days from the run's today.
func (h *harness) day(offset int) string {
	return h.todayIsUTC.AddDate(0, 0, offset).Format(time.DateOnly)
}

func TestScriptAndCommandSendTheSameMessagesForEveryRealPost(t *testing.T) {
	// The real corpus is the evidence that matters. A fixture is written by
	// somebody who already knows where the boundaries are; these files are the
	// ones that have actually been published, they carry emoji headers and
	// typographic dashes, and several of them sit close to the message limit.
	posts, err := filepath.Glob(filepath.Join("..", "..", "website", "changes", "posts", "*.md"))
	if err != nil {
		t.Fatalf("glob the posts: %v", err)
	}
	if len(posts) < 20 {
		t.Fatalf("found %d weekly posts, too few for this comparison to mean anything", len(posts))
	}

	h := newHarness(t)
	compared := 0
	for _, post := range posts {
		// force publishes a week whose dates would otherwise refuse it, and
		// no-date-stamp removes the one decision that depends on today.
		scriptSent, scriptCode := h.runScript(t, post, "--yes", "--force",
			"--no-date-stamp", "--channel", "ze-test")
		portSent, portCode := h.runCommand(t, weekly.Options{
			Confirm: true, Source: post, Force: true, Stamp: weekly.StampOff,
		})

		compare(t, filepath.Base(post), scriptSent, portSent, scriptCode, portCode)
		if len(scriptSent) > 0 {
			compared++
		}
	}

	if compared < 20 {
		t.Fatalf("only %d post(s) produced any message, so the comparison is close to vacuous", compared)
	}
	t.Logf("compared %d posts", compared)
}

func TestScriptAndCommandWriteTheSameArchive(t *testing.T) {
	// The archive is what marks a week as published, so a difference here is a
	// week that one implementation would publish twice.
	h := newHarness(t)
	start, end := h.day(-30), h.day(-24)
	post := h.writePost(t, start, end, "**📅 Ze Weekly Update**\n\nOne short section.")

	if _, code := h.runScript(t, post, "--yes", "--channel", "ze-test"); code != 0 {
		t.Fatalf("the script exited %d", code)
	}
	if _, code := h.runCommand(t, weekly.Options{Confirm: true, Source: post}); code != 0 {
		t.Fatalf("the command exited %d", code)
	}

	name := start + "-weekly.md"
	fromScript, err := os.ReadFile(filepath.Join(h.pyArchive, name))
	if err != nil {
		t.Fatalf("read the script's archive: %v", err)
	}
	fromCommand, err := os.ReadFile(filepath.Join(h.goArchive, name))
	if err != nil {
		t.Fatalf("read the command's archive: %v", err)
	}

	if !bytes.Equal(fromScript, fromCommand) {
		t.Errorf("the archives differ\nscript:\n%s\ncommand:\n%s", fromScript, fromCommand)
	}
	// A post 30 days late is stamped by both, which is what makes this file
	// cover the stamp decision as well as the archive format.
	if !strings.Contains(string(fromScript), "backfilled: true") {
		t.Errorf("the fixture did not exercise the stamp decision:\n%s", fromScript)
	}
}

func TestScriptAndCommandAgreeOnTheAutomaticStampDecision(t *testing.T) {
	// The script reads the wall clock, so the fixtures are placed relative to
	// it: one week that ended inside the stale window and one well past it.
	cases := []struct {
		name    string
		endsAgo int
		stamped bool
	}{
		{"inside the stale window", weekly.StaleAfterDays, false},
		{"one day past the stale window", weekly.StaleAfterDays + 1, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			start, end := h.day(-tc.endsAgo-6), h.day(-tc.endsAgo)
			post := h.writePost(t, start, end, "**📅 Ze Weekly Update**\n\nOne short section.")

			scriptSent, scriptCode := h.runScript(t, post, "--yes", "--channel", "ze-test")
			portSent, portCode := h.runCommand(t, weekly.Options{Confirm: true, Source: post})

			compare(t, tc.name, scriptSent, portSent, scriptCode, portCode)
			if len(scriptSent) != 1 {
				t.Fatalf("the script sent %d message(s), want 1", len(scriptSent))
			}
			carries := strings.Contains(scriptSent[0].text, "Week of "+start)
			if carries != tc.stamped {
				t.Errorf("the header carries the week = %v, want %v", carries, tc.stamped)
			}
		})
	}
}

func TestScriptAndCommandAgreeOnWhichWeeksASweepPublishes(t *testing.T) {
	// Three weeks: one already archived, one still running, one due. A sweep
	// must publish exactly the third.
	h := newHarness(t)
	done := h.day(-40)
	h.writePost(t, done, h.day(-34), "**📅 Ze Weekly Update**\n\nAlready published.")
	due := h.day(-20)
	h.writePost(t, due, h.day(-14), "**📅 Ze Weekly Update**\n\nDue now.")
	running := h.day(-2)
	h.writePost(t, running, h.day(4), "**📅 Ze Weekly Update**\n\nStill running.")

	for _, archive := range []string{h.pyArchive, h.goArchive} {
		if err := os.MkdirAll(archive, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", archive, err)
		}
		if err := os.WriteFile(filepath.Join(archive, done+"-weekly.md"), []byte("x"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", archive, err)
		}
	}

	scriptSent, scriptCode := h.runScript(t, "--all", h.postsDir, "--yes", "--channel", "ze-test")
	portSent, portCode := h.runCommand(t, weekly.Options{Confirm: true, Dir: h.postsDir})

	compare(t, "sweep", scriptSent, portSent, scriptCode, portCode)
	if len(scriptSent) != 1 {
		t.Fatalf("the sweep sent %d message(s), want only the week that is due", len(scriptSent))
	}
	if !strings.Contains(scriptSent[0].text, "Due now.") {
		t.Errorf("the sweep published the wrong week: %q", scriptSent[0].text)
	}
}

func TestScriptAndCommandRefuseTheSameFiles(t *testing.T) {
	cases := []struct {
		name string
		body string
		meta string
	}{
		{"no front matter", "**📅 Ze Weekly Update**\n\nNothing above.", ""},
		{
			"the retired HTML snapshot marker",
			"Before.\n\n<!-- ze-stat-snapshot: weekly update, frozen at publication -->\n\nAfter.",
			"covers: 2026-08-10 .. 2026-08-16",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			path := filepath.Join(h.postsDir, "broken.md")
			text := tc.body
			if tc.meta != "" {
				text = "---\n" + tc.meta + "\n---\n\n" + tc.body
			}
			if err := os.WriteFile(path, []byte(text+"\n"), 0o644); err != nil {
				t.Fatalf("write the fixture: %v", err)
			}

			scriptSent, scriptCode := h.runScript(t, path, "--yes", "--force", "--channel", "ze-test")
			portSent, portCode := h.runCommand(t, weekly.Options{
				Confirm: true, Source: path, Force: true,
			})

			if scriptCode == 0 {
				t.Fatalf("the script accepted %s", tc.name)
			}
			compare(t, tc.name, scriptSent, portSent, scriptCode, portCode)
			if len(portSent) != 0 {
				t.Errorf("the command sent %d message(s) from a file it refused", len(portSent))
			}
		})
	}
}

func TestScriptAndCommandRefuseAWeekThatHasNotEnded(t *testing.T) {
	h := newHarness(t)
	start, end := h.day(-2), h.day(4)
	post := h.writePost(t, start, end, "**📅 Ze Weekly Update**\n\nStill running.")

	scriptSent, scriptCode := h.runScript(t, post, "--yes", "--channel", "ze-test")
	portSent, portCode := h.runCommand(t, weekly.Options{Confirm: true, Source: post})

	if scriptCode != 1 {
		t.Fatalf("the script exited %d for a week still running, want 1", scriptCode)
	}
	compare(t, "unfinished week", scriptSent, portSent, scriptCode, portCode)
}

func TestScriptAndCommandResumeFromTheSameMessage(t *testing.T) {
	// A half-sent post is finished from the message that failed. Sending the
	// earlier ones again is the duplicate the resume point exists to prevent,
	// so the two implementations must count messages the same way.
	h := newHarness(t)
	start, end := h.day(-30), h.day(-24)
	section := strings.Repeat("a", weekly.Limit-40)
	body := "**📅 Ze Weekly Update**\n\n" + section + "\n\n**🔧 Two**\n\n" + section
	post := h.writePost(t, start, end, body)

	// no-date-stamp on both sides, so this case is about the resume point
	// alone: the stamp adds 22 characters to the header and would split the
	// first message, which is the interaction the stamp cases already cover.
	scriptSent, scriptCode := h.runScript(t, post, "--yes", "--no-date-stamp",
		"--channel", "ze-test", "--resume-from", "2")
	portSent, portCode := h.runCommand(t, weekly.Options{
		Confirm: true, Source: post, ResumeFrom: 2, Stamp: weekly.StampOff,
	})

	compare(t, "resume-from 2", scriptSent, portSent, scriptCode, portCode)
	if len(scriptSent) != 1 {
		t.Fatalf("resuming at message 2 sent %d message(s), want 1", len(scriptSent))
	}
	if !strings.Contains(scriptSent[0].text, "**🔧 Two**") {
		t.Errorf("resuming sent the wrong message: %q", scriptSent[0].text[:60])
	}
}

func TestCommandDeclaresItsAnswerShape(t *testing.T) {
	// The command answers rows, so the row operators act on the posts rather
	// than being refused. This is what `| json`, `| count` and `| match` are
	// bought with, and the script had none of them.
	shape, declared := command.ShapeForCommand("weekly")
	if !declared {
		t.Fatal("weekly declares no answer shape, so every row operator is judged after the tool has run")
	}
	if shape != command.ShapeMap {
		t.Errorf("weekly declares %v, want ShapeMap", shape)
	}
}

// readConstants is a Python one-liner that imports the script and prints the
// four numbers it publishes with. Importing runs find_discord_sh, which reads
// the filesystem and nothing else.
const readConstants = `import importlib.util, json, sys
spec = importlib.util.spec_from_file_location("pw", sys.argv[1])
pw = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pw)
print(json.dumps({
    "limit": pw.LIMIT,
    "stale_after_days": pw.STALE_AFTER_DAYS,
    "send_delay": pw.SEND_DELAY,
    "rate_limit_backoff": pw.RATE_LIMIT_BACKOFF,
}))`

func TestScriptAndCommandShareTheSameNumbers(t *testing.T) {
	// These four numbers decide what reaches the channel and how long a post
	// takes, and none of them is fully observable from the comparisons above:
	// a message limit one character lower splits every post in this corpus
	// exactly the same way, so the output cannot see the difference. They are
	// therefore compared directly, which is the only test here that reads the
	// script rather than running it.
	h := newHarness(t)

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "python3", "-c", readConstants, h.script).Output()
	if err != nil {
		t.Fatalf("read the script's constants: %v", err)
	}

	var script struct {
		Limit            int       `json:"limit"`
		StaleAfterDays   int       `json:"stale_after_days"`
		SendDelay        float64   `json:"send_delay"`
		RateLimitBackoff []float64 `json:"rate_limit_backoff"`
	}
	if err := json.Unmarshal(out, &script); err != nil {
		t.Fatalf("decode the script's constants: %v\n%s", err, out)
	}

	if script.Limit != weekly.Limit {
		t.Errorf("message limit: the script uses %d and the command uses %d",
			script.Limit, weekly.Limit)
	}
	if script.StaleAfterDays != weekly.StaleAfterDays {
		t.Errorf("stale window: the script uses %d day(s) and the command uses %d",
			script.StaleAfterDays, weekly.StaleAfterDays)
	}
	if got := weekly.SendDelay.Seconds(); got != script.SendDelay {
		t.Errorf("gap between posts: the script waits %vs and the command waits %vs",
			script.SendDelay, got)
	}

	backoff := weekly.RateLimitBackoff()
	if len(backoff) != len(script.RateLimitBackoff) {
		t.Fatalf("retry schedule: the script has %d step(s) and the command has %d",
			len(script.RateLimitBackoff), len(backoff))
	}
	for i, want := range script.RateLimitBackoff {
		if got := backoff[i].Seconds(); got != want {
			t.Errorf("retry %d: the script waits %vs and the command waits %vs", i+1, want, got)
		}
	}
}
