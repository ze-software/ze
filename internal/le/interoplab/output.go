// Design: docs/architecture/testing/interop.md -- fail-closed peer assertions.
// Related: wait.go -- a timed-out wait carries the peer's last answer.
package interoplab

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The bound on the peer output an assertion failure carries. A routing table
// dump can run to megabytes, so a failure keeps only its tail: the last lines
// are where a show command prints the state the assertion asked about, and the
// byte cap stops one very long line from flooding the report.
const (
	outputExcerptLinesMax = 40
	outputExcerptBytesMax = 4096
)

// OutputExcerpt renders a peer's answer for an assertion failure, so a red names
// what the peer printed and not only what the checker expected. The answer is
// trimmed, then cut to its last 40 lines and its last 4096 bytes, and a note on
// the first line says how much was cut. An empty answer is named as empty,
// because "the peer printed nothing" is itself the evidence.
// Safe for concurrent use: it holds no state.
func OutputExcerpt(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return "(the peer printed nothing)"
	}
	linesTotal := strings.Count(output, "\n") + 1
	bytesTotal := len(output)
	tail := output
	if linesTotal > outputExcerptLinesMax {
		cut := len(tail)
		for range outputExcerptLinesMax {
			cut = strings.LastIndexByte(tail[:cut], '\n')
		}
		tail = tail[cut+1:]
	}
	if len(tail) > outputExcerptBytesMax {
		tail = tail[len(tail)-outputExcerptBytesMax:]
		// A byte cut can land inside a rune: start at the next whole one.
		for tail != "" && !utf8.RuneStart(tail[0]) {
			tail = tail[1:]
		}
	}
	if len(tail) == bytesTotal {
		return output
	}
	var note textbuf.Buffer
	note.Str("(output cut: showing the last ").
		Int(int64(strings.Count(tail, "\n") + 1)).Str(" of ").
		Int(int64(linesTotal)).Str(" lines, ").
		Int(int64(len(tail))).Str(" of ").
		Int(int64(bytesTotal)).Str(" bytes)\n").Str(tail)
	return note.String()
}

// WithLastAnswer carries the text a probe last read on the error of a wait whose
// value is a verdict (a bool, a count) rather than that text. Wait's own timeout
// quotes its value, which for such a wait says only "false"; the text is what the
// peer printed. A nil err and an empty text pass through, because a probe that
// never read anything has no answer to quote and its error already says why.
// Safe for concurrent use: it holds no state.
func WithLastAnswer(err error, text string) error {
	if err == nil {
		return nil
	}
	if strings.TrimSpace(text) == "" {
		return err
	}
	return fmt.Errorf("%w; last answer:\n%s", err, OutputExcerpt(text))
}

// WithSearchedOutput carries the answer a one-shot assertion searched, so a red
// says what the peer printed beside what the checker expected, and diagnosing it
// needs no temporary code edit (journal row: failing-gate-prints-no-cause).
// It names an empty answer as empty, because that answer was read. A wait needs
// no wrapper: Wait's timeout already carries the last answer.
// Safe for concurrent use: it holds no state.
func WithSearchedOutput(err error, output string) error {
	return fmt.Errorf("%w; searched output:\n%s", err, OutputExcerpt(output))
}
