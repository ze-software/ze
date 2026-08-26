// Design: docs/architecture/core-design.md -- the weekly update's answer
// Related: poster.go -- what fills this in
//
// report.go holds what `le weekly` ANSWERS, apart from what produced it.
//
// The answer is one row per post, each carrying the messages that were built
// from it. That is structured data every operator can act on: `| json` feeds a
// script, `| match skipped` keeps the weeks nothing was done to, `| count` says
// how many weeks the run looked at. The report also renders ITSELF (Text),
// because what an operator reads before publishing is the message text, and the
// engine would render the rows as a table (letools/leroot, Prose).

package weekly

import "github.com/ze-software/ze/internal/core/textbuf"

// Message is one Discord message built from a post. It is what reaches the
// channel, so the text is carried rather than summarized: an operator reviewing
// a plan is reviewing these words.
type Message struct {
	// Number is the 1-based position in the post, and the number a resumed
	// post restarts at.
	Number int `json:"number"`
	// Chars is the length in the characters Discord counts, so an operator can
	// see how close the message is to the limit.
	Chars int `json:"chars"`
	// Text is the message.
	Text string `json:"text"`
	// Sent reports whether this message reached the channel in this run.
	Sent bool `json:"sent"`
}

// PostReport is one weekly post's row.
type PostReport struct {
	// Source is the file the post was read from.
	Source string `json:"source"`
	// Date is the Monday the covered week starts on, and the name of the
	// archive file.
	Date string `json:"date"`
	// Covers is the week the post declares.
	Covers string `json:"covers"`
	// Status is planned, sent, skipped or failed.
	Status string `json:"status"`
	// Reason says why, and is ABSENT when the status carries no reason of its
	// own. An empty reason on a sent post would read as a missing value.
	Reason string `json:"reason,omitempty"`
	// DateStamped reports whether the covered week was written into the header.
	// It says the stamp was APPLIED, not that a header was there to take it: a
	// body with no header line is published unchanged with a warning, and this
	// stays true so the archive records the decision the run made.
	DateStamped bool `json:"date-stamped"`
	// Archive is the file that records the published week, and is ABSENT until
	// the post has landed whole.
	Archive string `json:"archive,omitempty"`
	// Messages are what the body was split into, in send order.
	Messages []Message `json:"messages"`
}

// Report is the whole answer of one run.
//
// Posts is the only row set in it, which is what lets the engine derive the
// answer's shape and act on the rows with `| match`, `| first` and `| count`
// (internal/component/command/answer_shape.go, rowsIn).
type Report struct {
	// Action is what was asked for: planned or post.
	Action string `json:"action"`
	// Channel is where the messages go.
	Channel string `json:"channel"`
	// Posts is one row per post the run looked at.
	Posts []PostReport `json:"posts"`
}

// Text renders the run for a person, and ends in a newline.
//
// A plan prints every message in full, because reviewing the words is the whole
// purpose of a plan and a summary would hide what is about to be published. A
// post prints one line per week, because the messages have already gone past on
// the progress stream as they landed.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.SetColor(true)
	color := textbuf.C

	if len(r.Posts) == 0 {
		return tb.Colored(color.BrightGreen).Str("Nothing to publish").
			Colored(color.Reset).Byte('\n').String()
	}

	for _, post := range r.Posts {
		switch post.Status {
		case StatusSkipped:
			tb.Colored(color.BrightYellow).Str("skip ").Str(post.Date).Colored(color.Reset).
				Str(" -- ").Str(post.Reason).Byte('\n')
			continue
		case StatusSent:
			tb.Colored(color.BrightGreen).Str("sent ").Str(post.Date).Colored(color.Reset).
				Str(" -- ").Int(int64(len(post.Messages))).Str(" message(s) to ").Str(r.Channel).
				Str(", archived -> ").Str(post.Archive).Byte('\n')
			continue
		case StatusFailed:
			tb.Colored(color.BoldRed).Str("failed ").Str(post.Date).Colored(color.Reset).
				Str(" -- ").Int(int64(sentCount(post.Messages))).Byte('/').
				Int(int64(len(post.Messages))).Str(" message(s) reached ").Str(r.Channel).Byte('\n')
			continue
		}

		tb.Colored(color.BoldMagenta).Str("=== ").Str(post.Date).Str(" (covers ").Str(post.Covers).
			Str(") -- date-stamped: ").Bool(post.DateStamped).Str(" ===").Colored(color.Reset).Byte('\n')
		tb.Int(int64(len(post.Messages))).Str(" message(s):").Byte('\n')
		for _, message := range post.Messages {
			tb.Colored(color.BoldMagenta).Str("--- message ").Int(int64(message.Number)).Byte('/').
				Int(int64(len(post.Messages))).Str(" (").Int(int64(message.Chars)).Str(" chars) ---").
				Colored(color.Reset).Byte('\n')
			tb.Str(message.Text).Str("\n\n")
		}
		tb.Colored(color.BrightYellow).Str("(nothing sent -- `le weekly confirm` publishes this)").
			Colored(color.Reset).Byte('\n')
	}
	return tb.String()
}

// sentCount answers how many of a post's messages reached the channel, which is
// what a failed post's reader needs before resuming it.
func sentCount(messages []Message) int {
	sent := 0
	for _, message := range messages {
		if message.Sent {
			sent++
		}
	}
	return sent
}
