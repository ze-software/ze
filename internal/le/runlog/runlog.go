// Design: docs/contributing/running-commands.md -- the lines of a run log a reader needs first
// Related: ../verify/engine/summary.go -- the verification failure index built from these lines
// Related: ../job/quiet.go -- the same lines, answered by one job
//
// A run log is thousands of lines, and a reader who wants to know what broke
// needs five of them. This package owns WHICH lines those are, so the
// verification failure index and a quiet job name the same lines instead of
// each carrying its own pattern of what a failure looks like.
//
// Both functions read one line at a time. A stage log of a full verification
// run is hundreds of megabytes, so memory here is bounded by the longest line
// plus the lines that are kept.
package runlog

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// failureLine matches the line openings that a reader of a failed run needs
// first: the assertion that failed, the package that failed, and the two ways
// a Go process dies.
var failureLine = regexp.MustCompile(`^(--- FAIL:|FAIL[[:space:]]|panic:|fatal error:|Error:|\[FAIL\])`)

// Key returns up to limit failure lines from a run log, each prefixed with its
// 1-based line number, so a reader can find the line in the full log.
//
// An empty result says the log holds no line of that shape. It does not say
// the run passed: a caller that wants the verdict reads the exit code.
func Key(reader io.Reader, limit int) ([]string, error) {
	lines := make([]string, 0, limit)
	err := forEachLine(reader, func(number int, text string) bool {
		if !failureLine.MatchString(text) {
			return true
		}
		lines = append(lines, strconv.Itoa(number)+":"+text)
		return len(lines) < limit
	})
	return lines, err
}

// Head returns the first limit lines of a run log, unnumbered. A linter writes
// its findings from the first line, so the head IS the answer there.
func Head(reader io.Reader, limit int) ([]string, error) {
	lines := make([]string, 0, limit)
	err := forEachLine(reader, func(_ int, text string) bool {
		lines = append(lines, text)
		return len(lines) < limit
	})
	return lines, err
}

// forEachLine calls visit for each line of reader, numbering from 1, and stops
// when visit answers false or the log ends. A final line with no newline is a
// line, and a trailing newline does not produce an empty one.
func forEachLine(reader io.Reader, visit func(number int, text string) bool) error {
	buffered := bufio.NewReader(reader)
	for number := 1; ; number++ {
		text, err := buffered.ReadString('\n')
		ended := err != nil
		text = strings.TrimSuffix(text, "\n")

		if !ended || text != "" {
			if !visit(number, text) {
				return nil
			}
		}
		if !ended {
			continue
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("read run log: %w", err)
	}
}
