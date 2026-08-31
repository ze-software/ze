// Design: docs/architecture/testing/verify-freshness-scope.md -- verification debt follows local commits to push
package commit

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

func recordDebt(root, session, subject string, owed []Debt) (string, error) {
	relative := debtPath(session)
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
	held := make(map[string]bool)
	for line := range strings.SplitSeq(string(content), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) == 8 && strings.EqualFold(strings.TrimSpace(cells[6]), statusOpen) {
			held[strings.TrimSpace(line)] = true
		}
	}
	lines := make([]string, 0)
	if len(content) == 0 {
		lines = append(lines,
			"# Verification debt -- commit session "+session,
			"",
			"Gates that had not run green over these commits when they were made.",
			"Clear rows only through `le commit debt-clear` after the named gate exits 0.",
			"",
			"| Date | Session | Subject | Gate owed | Reason | Status |",
			"|------|---------|---------|-----------|--------|--------|",
		)
	}
	stamp := time.Now().UTC().Format(time.DateOnly)
	for _, row := range owed {
		rendered := "| " + stamp + " | " + debtCell(session) + " | " + debtCell(subject) +
			" | " + debtCell(row.Gate) + " | " + debtCell(row.Reason) + " | open |"
		if !held[rendered] {
			lines = append(lines, rendered)
		}
	}
	if len(lines) == 0 {
		return relative, nil
	}
	if _, err := file.Seek(0, 2); err != nil {
		return "", err
	}
	prefix := ""
	if len(content) != 0 && content[len(content)-1] != '\n' {
		prefix = "\n"
	}
	if _, err := file.WriteString(prefix + strings.Join(lines, "\n") + "\n"); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	return relative, nil
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
