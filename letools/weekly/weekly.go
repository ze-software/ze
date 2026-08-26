// Design: docs/architecture/core-design.md -- the weekly update's Discord form
// Overview: poster.go -- the run that drives these decisions
// Related: send.go -- what carries the messages this file builds
//
// Package weekly turns an approved weekly post into the messages Discord takes.
//
// A post is a file under website/changes/posts/, named for the Monday its week
// starts on, carrying `covers: <start> .. <end>` in its front matter. Discord
// refuses a message over 2000 characters, so the body is split at section
// boundaries and never inside one: a section that arrives cut in half reads as
// a fault to everyone in the channel, and a channel message cannot be taken
// back.
//
// Everything in this file is pure. It reads no clock, runs no process and
// writes no file, so every boundary in it is driven from a test with no fixture
// beyond a string.
package weekly

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Limit is the largest message this tool will build, in CHARACTERS.
//
// Discord's hard cap is 2000 and it counts characters rather than bytes, which
// matters here: a weekly update carries emoji section headers and typographic
// dashes, so its byte length runs well ahead of its character length. The
// margin absorbs the "-- Week of <date>" a backfilled post gains in its header.
const Limit = 1900

// StaleAfterDays is how long after a week ends a post can still be published
// without saying which week it describes.
//
// A message posted inside that window is dated well enough by Discord's own
// timestamp. Past it the timestamp lies about when the work happened, so the
// header states the week in the text.
const StaleAfterDays = 7

// statSnapshotMarker is the HTML comment an older publication flow wrote into
// the body to mark a frozen statistic. It is site-only metadata: HTML does not
// render in a Discord message, so it reaches the channel verbatim.
const statSnapshotMarker = "<!-- ze-stat-snapshot: weekly update, frozen at publication -->"

// Errors an operator can act on. Each one says what is wrong with a file the
// operator wrote, so each is returned rather than fatal.
var (
	// ErrNoFrontMatter says the file does not open with a `---` block.
	ErrNoFrontMatter = errors.New("no front matter")
	// ErrLegacyMarker says the body carries the retired HTML snapshot marker.
	ErrLegacyMarker = errors.New("legacy ze-stat-snapshot HTML marker in the body; " +
		"site-only metadata is invalid in the Discord body; put ze-stat-snapshot: true in front matter")
	// ErrNoCovers says the front matter declares no week.
	ErrNoCovers = errors.New(`front matter declares no "covers: <start> .. <end>"`)
	// ErrCoversRange says the covers line names no end date.
	ErrCoversRange = errors.New(`covers must read "<start> .. <end>"`)
)

// frontMatterRE splits the leading `---` block off the body. The block is
// matched non-greedily so the FIRST closing fence ends it, and (?s) lets the
// body carry newlines.
var frontMatterRE = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n(.*)\z`)

// headerLineRE matches the one line a date stamp is written into. It is
// anchored to a whole line, so a mention of the header inside a paragraph is
// left alone.
var headerLineRE = regexp.MustCompile(`(?m)^\*\*📅 Ze Weekly Update\*\*$`)

// Post is one parsed weekly source file.
type Post struct {
	// Meta is the front matter, one entry per line, split at the first colon.
	Meta map[string]string
	// Body is everything after the front matter, with the surrounding
	// whitespace removed. It is what reaches the channel.
	Body string
}

// ParseText reads one weekly source file's text.
//
// It refuses a file with no front matter, and a body carrying the retired HTML
// snapshot marker.
func ParseText(text string) (Post, error) {
	match := frontMatterRE.FindStringSubmatch(text)
	if match == nil {
		return Post{}, ErrNoFrontMatter
	}

	// The bound is the front matter's line count, which is the block the
	// operator wrote above the first `---`.
	meta := make(map[string]string, 8)
	for line := range strings.SplitSeq(match[1], "\n") {
		key, value, _ := strings.Cut(line, ":")
		meta[trimSpace(key)] = trimSpace(value)
	}

	body := trimSpace(match[2])
	if strings.Contains(body, statSnapshotMarker) {
		return Post{}, ErrLegacyMarker
	}
	return Post{Meta: meta, Body: body}, nil
}

// Covers answers the post's declared week, or ErrNoCovers when it declares
// none.
func (p Post) Covers() (string, error) {
	covers := p.Meta["covers"]
	if covers == "" {
		return "", ErrNoCovers
	}
	return covers, nil
}

// StartDate answers the first token of a covers range, which is the date the
// post and its archive file are both named for.
func StartDate(covers string) string {
	start, _, _ := strings.Cut(covers, "..")
	return firstField(trimSpace(start))
}

// EndDate answers the last day the covers range describes.
func EndDate(covers string) (time.Time, error) {
	_, end, found := strings.Cut(covers, "..")
	if !found {
		return time.Time{}, ErrCoversRange
	}
	return time.Parse(time.DateOnly, firstField(trimSpace(end)))
}

// DaysSinceWeekEnd answers how many days have passed since the covered week
// ended. It is NEGATIVE while the week is still running, which is the state
// that refuses a post.
func DaysSinceWeekEnd(covers string, today time.Time) (int, error) {
	end, err := EndDate(covers)
	if err != nil {
		return 0, err
	}
	// Both operands are midnight UTC on a whole day, so the division is exact
	// and rounds nowhere.
	return int(today.Sub(end) / (24 * time.Hour)), nil
}

// ApplyDateStamp writes the covered week into the header line.
//
// The second result reports whether the header was found. A post without one is
// returned unchanged: refusing it would block a publication over a heading, and
// stamping nothing in silence would leave a backfilled update reading as this
// week's, so the caller warns and continues.
func ApplyDateStamp(body, date string) (stamped string, headerFound bool) {
	if !headerLineRE.MatchString(body) {
		return body, false
	}

	var tb textbuf.Buffer
	replacement := tb.Str("**📅 Ze Weekly Update -- Week of ").Str(date).Str("**").String()

	// Only the FIRST header is stamped: a body that carries the header line
	// again later is quoting it, not declaring the week.
	stampWritten := false
	return headerLineRE.ReplaceAllStringFunc(body, func(match string) string {
		if stampWritten {
			return match
		}
		stampWritten = true
		return replacement
	}), true
}

// Messages splits a body into the messages that will be sent, in order.
//
// Sections are kept whole and packed greedily: a message takes as many whole
// sections as fit under Limit. A section over Limit on its own is split again
// at blank lines, the last boundary a reader can see. Below that the text is
// emitted over Limit rather than cut mid-sentence, because a message Discord
// refuses is visible to the operator and a sentence cut in half is not.
//
// The bound is the section count of one weekly post, which is a file an
// operator wrote.
func Messages(body string) []string {
	sections := make([]string, 0, 16)
	for _, raw := range splitSections(body) {
		if trimmed := trimSpace(raw); trimmed != "" {
			sections = append(sections, trimmed)
		}
	}

	messages := []string{}
	current := ""
	for _, section := range sections {
		if candidate := paste(current, section); charLen(candidate) <= Limit {
			current = candidate
			continue
		}
		if current != "" {
			messages = append(messages, current)
		}
		if charLen(section) <= Limit {
			current = section
			continue
		}

		current = ""
		for paragraph := range strings.SplitSeq(section, "\n\n") {
			if piece := paste(current, paragraph); charLen(piece) <= Limit {
				current = piece
				continue
			}
			if current != "" {
				messages = append(messages, current)
			}
			current = paragraph
		}
	}
	if current != "" {
		messages = append(messages, current)
	}
	return messages
}

// isSectionHeader reports whether a line is a section heading: a whole line in
// bold. Two stars, at least one character and two more stars is five
// characters, so a shorter line cannot be one however it is spelled.
func isSectionHeader(line string) bool {
	const fence = "**"
	return charLen(line) >= 5 && strings.HasPrefix(line, fence) && strings.HasSuffix(line, fence)
}

// splitSections divides a body immediately BEFORE every section header, so a
// header always opens the part it belongs to. The first part is never split off
// empty.
//
// The bound is the line count of one weekly post.
func splitSections(body string) []string {
	parts := make([]string, 0, 16)
	start := 0
	for offset := 0; offset < len(body); {
		line, next := lineAt(body, offset)
		if offset > start && isSectionHeader(line) {
			parts = append(parts, body[start:offset])
			start = offset
		}
		offset = next
	}
	return append(parts, body[start:])
}

// lineAt answers the line beginning at offset, without its newline, and the
// offset the next line begins at.
func lineAt(text string, offset int) (line string, next int) {
	end := strings.IndexByte(text[offset:], '\n')
	if end < 0 {
		return text[offset:], len(text)
	}
	return text[offset : offset+end], offset + end + 1
}

// paste joins two parts with the blank line that separates them in Markdown,
// and answers the second part alone when the first is empty.
func paste(first, second string) string {
	if first == "" {
		return second
	}
	var tb textbuf.Buffer
	return tb.Str(first).Str("\n\n").Str(second).String()
}

// charLen answers a string's length in CHARACTERS, which is the unit Discord
// counts a message in. A byte length over-counts every emoji and every
// typographic dash in a weekly update, so this is the only length this package
// measures text with.
func charLen(s string) int { return utf8.RuneCountInString(s) }

// trimSpace removes the surrounding whitespace, the no-break space included. A
// weekly post is drafted in an editor that leaves a terminal U+00A0 often
// enough, and it must not reach the channel.
func trimSpace(s string) string { return strings.TrimSpace(s) }

// firstField answers the text before the first space, which reads a date out of
// a covers token that carries a weekday or a note after it.
func firstField(s string) string {
	field, _, _ := strings.Cut(s, " ")
	return field
}
