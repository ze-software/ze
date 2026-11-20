// Design: docs/contributing/spec-workflow.md -- the records Thomas's approval and his absence leave
// Related: judge.go -- the judge record, written through replaceRecord
// Related: approve.go -- the approval record
// Related: away.go -- the away record
//
// records.go holds what the three records under tmp/spec-design/ share: the
// atomic replace, the `Key: value` line reader, and the one-line rule for the
// words Thomas gives (plan/release/validation/spec-design-needs-owner-approval.md,
// "The records").
package specdesign

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// replaceRecord replaces the record file name in dir, a slash path under the
// checkout root, atomically: a reader sees the old record or the new one,
// never a part.
func replaceRecord(root, dir, name, body string) error {
	target := filepath.Join(root, filepath.FromSlash(dir))
	if err := os.MkdirAll(target, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	temporary, err := os.CreateTemp(target, "."+name+"-*")
	if err != nil {
		return fmt.Errorf("write %s/%s: %w", dir, name, err)
	}
	if _, err := temporary.WriteString(body); err != nil {
		temporary.Close()           //nolint:errcheck,gosec // the write error is the one reported
		os.Remove(temporary.Name()) //nolint:errcheck,gosec // best effort: the record was not replaced
		return fmt.Errorf("write %s/%s: %w", dir, name, err)
	}
	if err := temporary.Close(); err != nil {
		os.Remove(temporary.Name()) //nolint:errcheck,gosec // best effort: the record was not replaced
		return fmt.Errorf("write %s/%s: %w", dir, name, err)
	}
	if err := os.Rename(temporary.Name(), filepath.Join(target, name)); err != nil {
		os.Remove(temporary.Name()) //nolint:errcheck,gosec // best effort: the record was not replaced
		return fmt.Errorf("write %s/%s: %w", dir, name, err)
	}
	return nil
}

// recordLines is one record as read: the last value of each single field, and
// every value of a repeated one, in file order.
type recordLines struct {
	single   map[string]string
	repeated map[string][]string
}

// readRecord reads the record at file. A file that does not exist answers
// found false; one that cannot be read is an error naming it, never an absent
// record, so an unreadable approval never passes for none.
func readRecord(file string, repeatedFields ...string) (lines recordLines, found bool, err error) {
	body, err := os.ReadFile(file) //nolint:gosec // a record of a validated stem under tmp/spec-design
	if errors.Is(err, os.ErrNotExist) {
		return recordLines{}, false, nil
	}
	if err != nil {
		return recordLines{}, false, fmt.Errorf("read the record %s: %w", file, err)
	}
	lines = recordLines{single: map[string]string{}, repeated: map[string][]string{}}
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		name, value, ok := strings.Cut(scanner.Text(), ": ")
		if !ok {
			continue
		}
		if slices.Contains(repeatedFields, name) {
			lines.repeated[name] = append(lines.repeated[name], value)
			continue
		}
		lines.single[name] = value
	}
	if err := scanner.Err(); err != nil {
		return recordLines{}, false, fmt.Errorf("read the record %s: %w", file, err)
	}
	return lines, true, nil
}

// missingField answers the first of fields the record carries no value for,
// or "".
func (r recordLines) missingField(fields []string) string {
	for _, name := range fields {
		if r.single[name] == "" {
			return name
		}
	}
	return ""
}

// wordsProblem answers what is wrong with the words Thomas gives an approval
// or an absence, or "": they are one line, and never start like a trailer key,
// since a commit carries them into a Design-approved: line.
func wordsProblem(words string) string {
	if strings.TrimSpace(words) == "" {
		return "the words are empty; fix: give what Thomas said, in one line"
	}
	if strings.ContainsAny(words, "\r\n") {
		return "the words span more than one line; fix: give what Thomas said in one line"
	}
	if trailerStart.MatchString(words) {
		return "the words start like a trailer key; fix: start them with a word, not `Key:`"
	}
	return ""
}

// refuseAction prints, on stderr, why an action records nothing and answers
// its exit code: 2, a refusal.
func refuseAction(lines ...string) int {
	var tb textbuf.Buffer
	for _, line := range lines {
		tb.Str(line).Byte('\n')
	}
	tb.StdErr() //nolint:errcheck // CLI output
	return 2
}
