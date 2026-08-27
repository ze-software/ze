// Design: docs/architecture/core-design.md -- the weekly update's transport
// Overview: poster.go -- the run that calls this
// Related: weekly.go -- the messages this file delivers
//
// send.go is everything between a built message and the channel: where the
// transport lives, how a refusal is read, what is retried, and what is written
// down once a post has landed.
//
// The transport is a parameter rather than a call. discord.sh carries the bot
// token, so it lives outside this public repository, and a test that reached it
// would post to a real channel. Sender is the seam: the binary passes
// ExecSender and a test passes a recorder.
package weekly

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// DiscordShKey is the dot-notation spelling of DISCORD_SH. env.Get treats a dot
// and an underscore as the same character and matches case-insensitively, so
// this key reads the variable the Python tool reads.
const DiscordShKey = "discord.sh"

var discordShEntry = env.MustRegister(env.EnvEntry{
	Key:         DiscordShKey,
	Type:        "string",
	Default:     "",
	Description: "the discord.sh CLI the weekly update is posted through; the known private bins are searched when unset",
	// Private keeps the key out of `ze env list`. It is le's variable, and it
	// names a path in the owner's private bin.
	Private: true,
})

// rateLimitBackoff is how long to wait before each retry of a message Discord
// refused for rate limiting. One entry per retry, so the schedule's length is
// the retry count and the retries are bounded by construction.
//
// The schedule is fixed because it cannot be read: discord.sh reports the API's
// `message` field and drops `retry_after`.
var rateLimitBackoff = [...]time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second}

// RateLimitBackoff answers the retry schedule, which is a number the two
// implementations of this tool MUST agree on and which no comparison of their
// output can see: a schedule that waits longer sends the same messages.
// scripts/zeledon/parity_test.go reads it for that reason.
func RateLimitBackoff() []time.Duration {
	return rateLimitBackoff[:]
}

// sendTimeout bounds one call to the transport.
const sendTimeout = 60 * time.Second

// SendDelay separates two distinct posts in one sweep.
//
// Discord groups consecutive messages from one author that land close together,
// which is what a single update's messages want and what two different weeks must
// not have: without a real gap, separate weeks collapse into one wall of text
// with only the first showing who posted it. 65 seconds breaks the grouping
// with margin over the roughly one-minute window.
const SendDelay = 65 * time.Second

// SendResult is what one call to the transport answered. The fields are the
// ones a refusal is read from, and nothing else about the process is kept.
type SendResult struct {
	Code   int
	Stdout string
	Stderr string
}

// landed reports whether the message reached the channel. Both halves are
// required: discord.sh has exited 0 while reporting a refusal.
func (r SendResult) landed() bool {
	return r.Code == 0 && strings.Contains(r.Stdout, "ok")
}

// rateLimited reports whether waiting could fix this refusal. It is the only
// refusal that is retried, because it is the only one that is transient by
// definition.
func (r SendResult) rateLimited() bool {
	var tb textbuf.Buffer
	report := tb.Str(r.Stdout).Byte(' ').Str(r.Stderr).String()
	return strings.Contains(strings.ToLower(report), "rate limited")
}

// report answers what the transport said, for an operator who has to act on it.
func (r SendResult) report() string {
	var tb textbuf.Buffer
	return strings.TrimSpace(tb.Str(strings.TrimSpace(r.Stdout)).Byte(' ').
		Str(strings.TrimSpace(r.Stderr)).String())
}

// Sender delivers one message to one channel. It answers what the transport
// said rather than an error, because a refusal is data this package classifies.
type Sender func(channel, text string) SendResult

// ExecSender answers the Sender that runs discord.sh at script.
//
// The invocation is `bash <script> --channel <channel> --text <text>`, which is
// the form discord.sh takes and the form the Python tool used, so one fake
// script stands in for both implementations.
func ExecSender(script string) Sender {
	return func(channel, text string) SendResult {
		// One HTTPS request is what the transport makes, so a call still
		// running after this is hung rather than slow. Without the bound a
		// publication waits on it for ever, holding a post half delivered.
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()

		// The transport's path is configuration, and the two values are
		// separate argv entries rather than a command line: bash reads the
		// script from a file and never interprets the message text.
		cmd := exec.CommandContext(ctx, "bash", script, "--channel", channel, "--text", text) //nolint:gosec // G204: the script is the configured transport, and the arguments bypass any shell
		var out, errOut strings.Builder
		cmd.Stdout = &out
		cmd.Stderr = &errOut

		err := cmd.Run()
		result := SendResult{Stdout: out.String(), Stderr: errOut.String()}
		if err == nil {
			return result
		}

		if exited, ok := errors.AsType[*exec.ExitError](err); ok {
			result.Code = exited.ExitCode()
			return result
		}
		// The process never started. Report it the way a refusal is reported,
		// so the caller has one shape to classify.
		result.Code = 127
		var tb textbuf.Buffer
		result.Stderr = tb.Str(result.Stderr).Str(err.Error()).String()
		return result
	}
}

// ErrNoTransport says discord.sh is not where the search expected it.
var ErrNoTransport = errors.New("discord.sh not found")

// SendFailed says a message was refused for something waiting cannot fix, and
// carries the resume point that finishes the post.
//
// The messages before it are in the channel. That is why this error names them: a
// fresh run would send every one of them a second time.
type SendFailed struct {
	// Number is the 1-based message that failed, and the one to resume from.
	Number int
	// Total is how many messages the post splits into.
	Total int
	// Channel is where the earlier messages already are.
	Channel string
	// Report is what the transport said.
	Report string
}

func (e *SendFailed) Error() string {
	var tb textbuf.Buffer
	return tb.Str("send failed on message ").Int(int64(e.Number)).Byte('/').Int(int64(e.Total)).
		Str(": ").Str(e.Report).
		Str("\nmessages 1 to ").Int(int64(e.Number - 1)).Str(" are in ").Str(e.Channel).
		Str(". Finish this post with `resume-from ").Int(int64(e.Number)).
		Str("` (a fresh run would repeat them).").String()
}

// send delivers every message of one post back-to-back, with no wait between
// messages: the parts of a single weekly update are meant to read as one grouped
// block. The gap that breaks Discord's grouping goes between distinct posts.
//
// Sending them back-to-back is what meets Discord's per-channel rate limit, so
// a message refused that way is retried on rateLimitBackoff rather than killing
// the post. Anything else stops the post and answers a *SendFailed naming the
// resume point.
//
// resumeFrom is 1-based and names the first message to send, so a post left half
// delivered is finished without repeating what already landed.
func (p *Poster) send(messages []string, resumeFrom int) (sent int, err error) {
	// Before the loop, so a missing transport costs nothing to recover from:
	// nothing has been sent, so nothing has to be resumed.
	if _, statErr := os.Stat(p.DiscordSh); statErr != nil {
		var tb textbuf.Buffer
		return 0, errors.New(tb.Err(ErrNoTransport).Str(" at ").Str(p.DiscordSh).String())
	}

	for index, message := range messages {
		number := index + 1
		if number < resumeFrom {
			p.say(func(tb *textbuf.Buffer) {
				tb.Str("skip message ").Int(int64(number)).Byte('/').Int(int64(len(messages))).
					Str(" -- already sent")
			})
			continue
		}

		if failed := p.sendOne(message, number, len(messages)); failed != nil {
			return sent, failed
		}
		sent++

		p.say(func(tb *textbuf.Buffer) {
			tb.Str("sent message ").Int(int64(number)).Byte('/').Int(int64(len(messages))).
				Str(" (").Int(int64(charLen(message))).Str(" chars)")
		})
	}
	return sent, nil
}

// sendOne delivers one message, waiting out a rate limit up to the length of the
// schedule. The loop is bounded by rateLimitBackoff, so a channel that never
// lets up ends the run rather than waiting for ever.
func (p *Poster) sendOne(message string, number, total int) error {
	for attempt := 0; attempt <= len(rateLimitBackoff); attempt++ {
		result := p.Send(p.Channel, message)
		if result.landed() {
			return nil
		}
		if attempt == len(rateLimitBackoff) || !result.rateLimited() {
			return &SendFailed{Number: number, Total: total, Channel: p.Channel, Report: result.report()}
		}

		wait := rateLimitBackoff[attempt]
		p.say(func(tb *textbuf.Buffer) {
			tb.Str("rate limited on message ").Int(int64(number)).Byte('/').Int(int64(total)).
				Str(", retrying in ").Int(int64(wait / time.Second)).
				Str("s (attempt ").Int(int64(attempt + 1)).Byte('/').
				Int(int64(len(rateLimitBackoff))).Byte(')')
		})
		p.Sleep(wait)
	}
	// Unreachable: the last iteration returns either way. Stated rather than
	// panicked, because nothing a peer or an operator does can reach it.
	return &SendFailed{Number: number, Total: total, Channel: p.Channel}
}

// archive writes down the week that just landed and answers the file it wrote.
//
// The body recorded is the text that was SENT, not the source file's text, so
// the archive is a byte-accurate record of what is in the channel. Its presence
// is also what marks a week as posted, which is what stops a sweep sending it
// again.
func (p *Poster) archive(date, covers, body string, dateStamped bool) (string, error) {
	if err := os.MkdirAll(p.ArchiveDir, 0o750); err != nil {
		return "", err
	}

	var name textbuf.Buffer
	path := filepath.Join(p.ArchiveDir, name.Str(date).Str("-weekly.md").String())

	var tb textbuf.Buffer
	tb.Str("---\nposted: ").Str(p.Today.Format(time.DateOnly)).
		Str("\nchannel: ").Str(p.Channel).
		Str("\ncovers: ").Str(covers)
	if dateStamped {
		tb.Str("\nbackfilled: true")
	}
	tb.Str("\n---\n\n").Str(body).Byte('\n')

	if err := os.WriteFile(path, []byte(tb.String()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// say writes one line of running commentary.
//
// Progress is not the answer, so it goes to the progress writer rather than
// into the payload: an operator watching a post that takes minutes needs to see
// each message land, and a caller piping the answer into a script needs the
// answer alone (ai/rules/cli.md).
func (p *Poster) say(write func(tb *textbuf.Buffer)) {
	if p.Progress == nil {
		return
	}
	var tb textbuf.Buffer
	write(&tb)
	tb.Byte('\n')
	io.WriteString(p.Progress, tb.String()) //nolint:errcheck // progress output
}

// FindDiscordSh answers where the transport is.
//
// DISCORD_SH wins. Otherwise the known private bins are searched in turn and
// PATH answers the rest. The token means discord.sh cannot live in this public
// repository, and which private bin holds it differs per machine: a single
// hardcoded default sent one weekly update to a "not found" exit.
func FindDiscordSh() string {
	return findDiscordSh(discordShCandidates())
}

// discordShCandidates are the private bins discord.sh is kept in, most likely
// first. A home directory that cannot be read leaves the list empty, and the
// PATH search still answers.
func discordShCandidates() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, "Unix", "bin", "discord.sh"),
		filepath.Join(home, "bin", "discord.sh"),
	}
}

// findDiscordSh takes the candidate list as a parameter so the search order is
// driven by a test without a home directory standing in for one.
//
// When nothing is found it answers the FIRST candidate rather than an empty
// string, so the refusal a caller prints names a path an operator can act on.
func findDiscordSh(candidates []string) string {
	if override := env.Get(discordShEntry.Key); override != "" {
		return override
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if onPath, err := exec.LookPath("discord.sh"); err == nil {
		return onPath
	}
	if len(candidates) == 0 {
		return "discord.sh"
	}
	return candidates[0]
}
