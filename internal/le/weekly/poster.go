// Design: docs/architecture/core-design.md -- the weekly update's publication
// Overview: answer.go -- the operator's words, turned into the options below
// Detail: weekly.go -- the pure decisions this file drives
// Detail: send.go -- the transport, the retry schedule and the archive
// Related: report.go -- what this file fills in
//
// poster.go decides WHICH weeks are published and in what order, and it holds
// every seam the process has: the clock, the transport, the waits, and the two
// directories. Nothing below reads a global, so a whole sweep runs in a test
// with no channel, no home directory and no wall clock.
package weekly

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Status is what happened to one post in one run. The four values are the whole
// set: a post is planned, sent whole, skipped for a stated reason, or begun and
// left unfinished.
const (
	// StatusPlanned means a plan built the messages and sent nothing.
	StatusPlanned = "planned"
	// StatusSent means every message reached the channel and the week is
	// archived.
	StatusSent = "sent"
	// StatusSkipped means a sweep passed the post over. Reason says why.
	StatusSkipped = "skipped"
	// StatusFailed means the post was begun and did not finish. Messages says
	// how far it got.
	StatusFailed = "failed"
)

// Stamp is whether the covered week is written into the header line. The zero
// value defers to StaleAfterDays, which is the answer an operator wants almost
// every time.
type Stamp uint8

const (
	// StampAuto stamps a post published more than StaleAfterDays after its
	// week ended, and leaves an in-time post to Discord's own timestamp.
	StampAuto Stamp = iota
	// StampOn stamps whatever the dates say.
	StampOn
	// StampOff leaves the header alone whatever the dates say.
	StampOff
)

// Options is what the operator chose. Source and Dir are exclusive: one names a
// single post, the other the directory a sweep reads.
type Options struct {
	// Confirm publishes. Without it the run builds the messages and prints
	// them, which is the default because publishing cannot be undone: a
	// message is in a public channel the moment it lands.
	Confirm bool
	// Source names one post file. Empty means sweep Dir.
	Source string
	// Dir is the directory a sweep reads, one post per file.
	Dir string
	// Channel is the Discord channel the messages go to.
	Channel string
	// Stamp overrides the automatic date-stamp decision.
	Stamp Stamp
	// Force publishes a week that has not ended. Single-post runs only: a
	// sweep that forced would publish every unfinished week it found.
	Force bool
	// ResumeFrom is the 1-based message a half-sent post restarts at. Zero and
	// one both mean the beginning. Single-post runs only.
	ResumeFrom int
}

// publisher runs one publication. Every field is a seam, and the binary is the
// only caller that fills them from the machine (answer.go).
type publisher struct {
	// Channel is the Discord channel every message goes to.
	Channel string
	// Send delivers one message. execSender answers the one that runs
	// discord.sh; a test passes a recorder.
	Send Sender
	// DiscordSh is where the transport lives. send refuses before the first
	// message when it is absent, because a half-sent post is the expensive
	// failure here and an absent transport sends nothing at all.
	DiscordSh string
	// Sleep waits out a rate limit and separates two posts.
	Sleep func(time.Duration)
	// Today decides eligibility, the automatic date stamp, and the archive's
	// posted line. One fact, so one field.
	Today time.Time
	// ArchiveDir holds one file per published week. A file's presence is what
	// marks that week as done, so a sweep reads this directory before it sends
	// anything.
	ArchiveDir string
	// Progress receives the running commentary. It is not the answer, so it
	// goes to stderr in the binary and to a buffer in a test.
	Progress io.Writer
}

// Run publishes what the options ask for and answers what happened.
//
// The error is the run's verdict for the operator: a refused week, an
// unreadable post, a transport that would not deliver. The report is returned
// beside it either way, because a post that failed half way through still has
// to say which messages are in the channel.
func (p *publisher) Run(opts Options) (Report, error) {
	report := Report{Action: StatusPlanned, Channel: p.Channel, Posts: []PostReport{}}
	if opts.Confirm {
		report.Action = "published"
	}

	if opts.Source != "" {
		one, err := p.publish(opts.Source, opts, opts.Force, opts.ResumeFrom)
		report.Posts = append(report.Posts, one)
		return report, err
	}
	return p.sweep(report, opts)
}

// sweep publishes every eligible post in a directory, oldest first.
//
// Falling behind and catching up are the same operation here: an already
// archived week is skipped and an unfinished week is skipped, so one missed
// week or ten is the same command.
//
// The bound is the file count of the posts directory.
func (p *publisher) sweep(report Report, opts Options) (Report, error) {
	files, err := filepath.Glob(filepath.Join(opts.Dir, "*.md"))
	if err != nil {
		return report, err
	}
	if len(files) == 0 {
		var tb textbuf.Buffer
		return report, errors.New(tb.Str("no .md files in ").Str(opts.Dir).String())
	}
	// The order is chronological already: every post is named for the Monday
	// its week starts on, and filepath.Glob answers sorted names. A sort here
	// would restate that and hide where the order comes from.

	postedAny := false
	for _, file := range files {
		one, skip, err := p.eligible(file)
		if err != nil {
			return report, err
		}
		if skip {
			report.Posts = append(report.Posts, one)
			continue
		}

		// The gap goes BETWEEN posts, never between the messages of one post.
		if postedAny {
			p.Sleep(SendDelay)
		}

		// force is never carried into a sweep: an operator forcing one post is
		// making a decision about that post.
		published, err := p.publish(file, opts, false, 1)
		report.Posts = append(report.Posts, published)
		if err != nil {
			return report, err
		}
		if published.Status == StatusSent {
			postedAny = true
		}
	}
	return report, nil
}

// eligible answers whether a sweep should pass a post over, and the row that
// records the reason when it should.
func (p *publisher) eligible(file string) (skipped PostReport, skip bool, err error) {
	_, covers, err := readPost(file)
	if err != nil {
		return PostReport{}, false, err
	}
	date := StartDate(covers)
	row := PostReport{Source: file, Date: date, Covers: covers, Status: StatusSkipped, Messages: []Message{}}

	if archive := p.archivePath(date); fileExists(archive) {
		var tb textbuf.Buffer
		row.Reason = tb.Str("already archived at ").Str(archive).String()
		return row, true, nil
	}

	days, err := DaysSinceWeekEnd(covers, p.Today)
	if err != nil {
		return PostReport{}, false, wrapPost(file, err)
	}
	if days < 0 {
		var tb textbuf.Buffer
		row.Reason = tb.Str("week not over yet (covers ").Str(covers).Byte(')').String()
		return row, true, nil
	}

	return PostReport{}, false, nil
}

// publish builds one post's messages, and sends them when the action says to.
//
// Every refusal happens before the first message leaves: a week still running,
// a resume point past the end. Once sending starts the only failure left is the
// transport's, and send answers that one with the point to restart from.
func (p *publisher) publish(file string, opts Options, force bool, resumeFrom int) (PostReport, error) {
	post, covers, err := readPost(file)
	if err != nil {
		return PostReport{Source: file, Status: StatusFailed, Messages: []Message{}}, err
	}

	date := StartDate(covers)
	row := PostReport{Source: file, Date: date, Covers: covers, Messages: []Message{}}

	days, err := DaysSinceWeekEnd(covers, p.Today)
	if err != nil {
		row.Status = StatusFailed
		return row, wrapPost(file, err)
	}
	if days < 0 && !force {
		row.Status = StatusSkipped
		var tb textbuf.Buffer
		_, end, _ := strings.Cut(covers, "..")
		row.Reason = tb.Str("week doesn't end until ").Str(strings.TrimSpace(end)).
			Str(" (").Int(int64(-days)).Str(" day(s) away). Use force to override.").String()
		return row, errors.New(row.Reason)
	}

	body := post.Body
	row.DateStamped = stampWanted(opts.Stamp, days)
	if row.DateStamped {
		stamped, headerFound := ApplyDateStamp(body, date)
		if !headerFound {
			p.say(func(tb *textbuf.Buffer) {
				tb.Str("warning: no '**📅 Ze Weekly Update**' header line found, " +
					"posting body unchanged (no date inserted)")
			})
		}
		body = stamped
	}

	messages := Messages(body)
	if resumeFrom < 1 {
		resumeFrom = 1
	}
	if resumeFrom > len(messages) {
		row.Status = StatusFailed
		var tb textbuf.Buffer
		return row, errors.New(tb.Str("resume-from ").Int(int64(resumeFrom)).Str(" but ").
			Str(filepath.Base(file)).Str(" splits into ").Int(int64(len(messages))).Str(" message(s)").String())
	}

	row.Messages = make([]Message, len(messages))
	for index, message := range messages {
		row.Messages[index] = Message{Number: index + 1, Chars: charLen(message), Text: message}
	}

	if !opts.Confirm {
		row.Status = StatusPlanned
		return row, nil
	}

	sent, err := p.send(messages, resumeFrom)
	for index := range row.Messages {
		row.Messages[index].Sent = index+1 >= resumeFrom && index < resumeFrom-1+sent
	}
	if err != nil {
		row.Status = StatusFailed
		return row, err
	}

	archive, err := p.archive(date, covers, body, row.DateStamped)
	if err != nil {
		row.Status = StatusFailed
		return row, err
	}
	row.Archive = archive
	row.Status = StatusSent
	return row, nil
}

// stampWanted answers whether the covered week is written into the header.
func stampWanted(choice Stamp, daysSinceWeekEnd int) bool {
	switch choice {
	case StampOn:
		return true
	case StampOff:
		return false
	default:
		return daysSinceWeekEnd > StaleAfterDays
	}
}

// archivePath answers where a week's record lives, whether or not it exists.
func (p *publisher) archivePath(date string) string {
	var tb textbuf.Buffer
	return filepath.Join(p.ArchiveDir, tb.Str(date).Str("-weekly.md").String())
}

// readPost reads and parses one source file, and answers the week it covers. A
// post with no covers line is refused here rather than at the point a date is
// needed, so the message names the file.
func readPost(file string) (Post, string, error) {
	text, err := os.ReadFile(file) //nolint:gosec // G304: the operator names the post to publish; reading it is the command
	if err != nil {
		return Post{}, "", err
	}
	post, err := ParseText(string(text))
	if err != nil {
		return Post{}, "", wrapPost(file, err)
	}
	covers, err := post.Covers()
	if err != nil {
		return Post{}, "", wrapPost(file, err)
	}
	return post, covers, nil
}

// wrapPost names the file a refusal is about, so an operator sweeping thirty
// posts knows which one to open.
func wrapPost(file string, err error) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str(file).Str(": ").Err(err).String())
}

// fileExists reports whether a path is there to be read.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
