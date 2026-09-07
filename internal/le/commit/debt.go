// Design: docs/architecture/testing/verify-freshness-scope.md -- verification debt follows local commits to push
package commit

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const debtDir = "plan/verification-debt"

var debtGates = []struct {
	Key  string
	Name string
}{
	{gateUnverified, "full native verification (not FRESH-green)"},
	{gateStructuralRedOK, "native structural checks (red)"},
	{gateMissingFullVerifyOK, "full native verification over this commit's Go"},
	{gateStaleIndexOK, "discovery-index freshness"},
	{gateReviewOverride, "independent critical review"},
	{gateBrokenHeadFix, "repository tracked-build/check (HEAD does not compile)"},
	{gateRFCChangeOK, "owner approval for an RFC-tagged test change"},
}

// Debt is one open or cleared verification obligation.
type Debt struct {
	Shard   string `json:"shard"`
	Line    int    `json:"line"`
	Date    string `json:"date"`
	Session string `json:"session"`
	Subject string `json:"subject"`
	Gate    string `json:"gate"`
	Reason  string `json:"reason"`
	Status  string `json:"status"`
	Raw     string `json:"-"`
}

func debtPath(session string) string {
	return filepath.ToSlash(filepath.Join(debtDir, session+".md"))
}

// recordDebt writes this commit's owed gates into the session's ledger shard.
// A (gate, reason) pair holds ONE open row: a later commit owing the same gate
// for the same reason EXTENDS that row's commit count instead of appending a
// copy of it. The freshness gates state one fact about the tree, in a reason
// string byte-identical for every commit a long verification run overlaps, so
// appending recorded that one fact thousands of times.
//
// The row keeps the date and the subject of the first commit it covers. Every
// commit that extends a row writes the shard, so `git log -- <shard>` names
// each commit the rows cover, and the date and subject of every commit after
// the first are read there.
func recordDebt(root, session, subject string, owed []Debt) (string, error) {
	relative := debtPath(session)
	if len(owed) == 0 {
		return relative, nil
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck // the explicit write/sync result owns the verdict
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return "", err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN) //nolint:errcheck // process exit releases the advisory lock
	content, err := os.ReadFile(path)                    //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	if err != nil {
		return "", err
	}
	lines := debtHeader(session)
	if len(content) != 0 {
		lines = strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	}
	stamp := time.Now().UTC().Format(time.DateOnly)
	for _, row := range owed {
		gate, reason := debtCell(row.Gate), debtCell(row.Reason)
		at := openDebtRowAt(lines, gate, reason)
		if at < 0 {
			lines = append(lines, debtRow(stamp, debtCell(session), debtCell(subject), gate, reason))
			continue
		}
		lines[at] = extendDebtRow(lines[at])
	}
	rendered := strings.Join(lines, "\n") + "\n"
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}
	if _, err := file.WriteString(rendered); err != nil {
		return "", err
	}
	if err := file.Truncate(int64(len(rendered))); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	return relative, nil
}

// debtHeader is the shard's prose and table head, written once per session.
func debtHeader(session string) []string {
	return []string{
		"# Verification debt -- commit session " + session,
		"",
		"Gates that had not run green over these commits when they were made.",
		"One row holds one gate and one reason, and covers every commit this",
		"session made under it. `git log -- <this file>` names those commits.",
		"Clear rows only through `le commit debt-clear` after the named gate exits 0.",
		"",
		"| Date | Session | Subject | Gate owed | Reason | Status |",
		"|------|---------|---------|-----------|--------|--------|",
	}
}

// debtRow renders one open row.
func debtRow(date, session, subject, gate, reason string) string {
	return "| " + date + " | " + session + " | " + subject +
		" | " + gate + " | " + reason + " | open |"
}

// openDebtRowAt answers the index of the open row this gate and reason already
// hold, and -1 when the pair holds none. A cleared row is never extended, so a
// gate owed again after it was cleared opens a row of its own.
func openDebtRowAt(lines []string, gate, reason string) int {
	for index, line := range lines {
		row, ok := parseDebtRow("", index+1, line)
		if !ok || row.Status != statusOpen {
			continue
		}
		if row.Gate == gate && row.Reason == reason {
			return index
		}
	}
	return -1
}

// extendDebtRow adds one commit to the row's cover. The caller MUST pass a line
// openDebtRowAt matched, which is why a malformed one is a Ze defect here.
func extendDebtRow(line string) string {
	cells := strings.Split(line, "|")
	if len(cells) != 8 {
		panic("BUG: extendDebtRow was given a line openDebtRowAt did not match")
	}
	subject, covered := debtCovered(strings.TrimSpace(cells[3]))
	cells[3] = " " + debtSubject(subject, covered+1) + " "
	return strings.Join(cells, "|")
}

// debtCovered reads a subject cell back: the first commit's subject, and the
// number of commits the row covers.
func debtCovered(cell string) (string, int) {
	const opening = " (+"
	const closing = " more)"
	if !strings.HasSuffix(cell, closing) {
		return cell, 1
	}
	head := strings.TrimSuffix(cell, closing)
	at := strings.LastIndex(head, opening)
	if at < 0 {
		return cell, 1
	}
	after, err := strconv.Atoi(head[at+len(opening):])
	if err != nil || after < 1 {
		return cell, 1
	}
	return head[:at], after + 1
}

// debtSubject renders a subject cell covering the named number of commits.
func debtSubject(subject string, covered int) string {
	if covered < 2 {
		return subject
	}
	return subject + " (+" + strconv.Itoa(covered-1) + " more)"
}

func debtCell(value string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(value, "|", "/")), " ")
}

// ListDebt returns every valid debt row from every shard in stable order.
func ListDebt(root string) ([]Debt, error) {
	dir := filepath.Join(root, filepath.FromSlash(debtDir))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows := make([]Debt, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		file, err := os.Open(filepath.Join(dir, entry.Name())) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		line := 0
		for scanner.Scan() {
			line++
			row, ok := parseDebtRow(entry.Name(), line, scanner.Text())
			if ok {
				rows = append(rows, row)
			}
		}
		closeErr := file.Close()
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	sort.Slice(rows, func(left, right int) bool {
		if rows[left].Shard == rows[right].Shard {
			return rows[left].Line < rows[right].Line
		}
		return rows[left].Shard < rows[right].Shard
	})
	return rows, nil
}

func parseDebtRow(shard string, line int, text string) (Debt, bool) {
	cells := strings.Split(text, "|")
	if len(cells) != 8 {
		return Debt{}, false
	}
	for index := range cells {
		cells[index] = strings.TrimSpace(cells[index])
	}
	status := strings.ToLower(cells[6])
	if status != statusOpen && status != "cleared" {
		return Debt{}, false
	}
	return Debt{
		Shard: shard, Line: line, Date: cells[1], Session: cells[2], Subject: cells[3],
		Gate: cells[4], Reason: cells[5], Status: status, Raw: strings.TrimSpace(text),
	}, true
}

func openDebt(root string) ([]Debt, error) {
	rows, err := ListDebt(root)
	if err != nil {
		return nil, err
	}
	open := make([]Debt, 0)
	for _, row := range rows {
		if row.Status == statusOpen {
			open = append(open, row)
		}
	}
	return open, nil
}

func clearDebtRows(root string, passed map[string]bool) (int, error) {
	rows, err := openDebt(root)
	if err != nil {
		return 0, err
	}
	byShard := make(map[string]map[int]string)
	for _, row := range rows {
		if passed[row.Gate] {
			if byShard[row.Shard] == nil {
				byShard[row.Shard] = make(map[int]string)
			}
			byShard[row.Shard][row.Line] = row.Raw
		}
	}
	cleared := 0
	for shard, selected := range byShard {
		path := filepath.Join(root, filepath.FromSlash(debtDir), shard)
		file, err := os.OpenFile(path, os.O_RDWR, 0) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if err != nil {
			return cleared, err
		}
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
			_ = file.Close()
			return cleared, err
		}
		content, err := os.ReadFile(path) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if err != nil {
			_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
			_ = file.Close()
			return cleared, err
		}
		lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
		for index, line := range lines {
			lineNumber := index + 1
			if selected[lineNumber] != strings.TrimSpace(line) {
				continue
			}
			cells := strings.Split(line, "|")
			if len(cells) != 8 {
				continue
			}
			cells[6] = " cleared "
			lines[index] = strings.Join(cells, "|")
			cleared++
		}
		rendered := strings.Join(lines, "\n") + "\n"
		_, err = file.Seek(0, 0)
		if err == nil {
			_, err = file.WriteString(rendered)
		}
		if err == nil {
			err = file.Truncate(int64(len(rendered)))
		}
		if err == nil {
			err = file.Sync()
		}
		unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		closeErr := file.Close()
		if err != nil {
			return cleared, fmt.Errorf("clear debt shard %s: %w", shard, err)
		}
		if unlockErr != nil {
			return cleared, fmt.Errorf("unlock debt shard %s: %w", shard, unlockErr)
		}
		if closeErr != nil {
			return cleared, fmt.Errorf("close debt shard %s: %w", shard, closeErr)
		}
	}
	return cleared, nil
}

// The verification-debt gate names, the debt row status, the freshness state,
// and the two command keywords this package repeats.
const (
	statusOpen              = "open"
	keywordSession          = "session"
	actionReviewCheck       = "review-check"
	verifyFresh             = "fresh"
	verifyNotApplicable     = "not-applicable"
	gateUnverified          = "unverified"
	gateStructuralRedOK     = "structural-red-ok"
	gateMissingFullVerifyOK = "missing-full-verify-ok"
	gateStaleIndexOK        = "stale-index-ok"
	gateReviewOverride      = "review-override"
	gateBrokenHeadFix       = "broken-head-fix"
	gateRFCChangeOK         = "rfc-change-ok"
)
