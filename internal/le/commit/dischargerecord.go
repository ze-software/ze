// Design: docs/architecture/testing/verify-freshness-scope.md -- a discharge record holds the INPUT alone, never a verdict
// Related: internal/le/commit/discharge.go -- the verifiers that re-derive every row this file stores.
package commit

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// dischargePath is the per-session record file. A separate file per session is
// what keeps two discharging sessions off one another's writes, exactly as the
// debt shards do.
func dischargePath(session string) string {
	return filepath.ToSlash(filepath.Join(debtDir, dischargeDirName, session+".md"))
}

// dischargeHeader is the record file's prose and table head, written once.
func dischargeHeader(session string) []string {
	return []string{
		"# Verification-debt discharges -- commit session " + session,
		"",
		"How one debt row's obligation was met. A row here holds the INPUT alone,",
		"the kind and its evidence: every read re-derives the verdict from git, so",
		"nothing on this page is a verdict a reader trusts.",
		"",
		"The row digest pins the discharge to the exact debt row it answers. An",
		"edited row drops its discharge and counts open again.",
		"",
		"The Commits cell holds every commit the debt row covers, separated by",
		"spaces. Each one is derived, so a row covering three commits carries three.",
		"",
		"Write rows only through `le commit debt-discharge`.",
		"",
		"| Date | Shard | Line | Row digest | Kind | Commits | Artifact | Authorisation |",
		"|------|-------|------|------------|------|---------|----------|---------------|",
	}
}

// writeDischargeRecords writes one row per record under an exclusive lock, and
// answers the record file's path. The caller MUST have verified every record.
//
// A row REPLACES the record its debt row already holds, so one debt row keeps
// at most one record here. An appended second attempt leaves the first on disk,
// and the reader re-derives it for ever: a superseded attempt then prints as an
// invalid record on every read, which is the one line that has to mean a tamper
// (R-1).
func writeDischargeRecords(root, session string, records []dischargeRecord) (string, error) {
	relative := dischargePath(session)
	if len(records) == 0 {
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

	content, err := os.ReadFile(path) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	if err != nil {
		return "", err
	}
	lines := dischargeHeader(session)
	if len(content) != 0 {
		lines = strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	}
	for _, record := range records {
		lines = replaceDischargeRow(lines, record)
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

// replaceDischargeRow writes one record into the file's lines, in the place of
// every record its debt row already holds.
//
// The first of them keeps its position, so the file's order stays the order the
// discharges were made in, and any further one is dropped: a file written
// before this rule can hold several rows for one debt row, and collapsing them
// here is what stops a reader re-deriving an attempt the operator has replaced.
func replaceDischargeRow(lines []string, record dischargeRecord) []string {
	kept := make([]string, 0, len(lines)+1)
	written := false
	for _, line := range lines {
		if !dischargeRowAnswers(line, record.Shard, record.Line) {
			kept = append(kept, line)
			continue
		}
		if written {
			continue
		}
		kept = append(kept, record.row())
		written = true
	}
	if written {
		return kept
	}
	return append(kept, record.row())
}

// dischargeRowAnswers answers whether this line is a record of the named debt
// row. A line whose own cells are wrong answers no: the reader reports such a
// line by name, and a writer that overwrote it would erase the evidence of a
// hand edit rather than surface it.
func dischargeRowAnswers(line, shard string, number int) bool {
	record, problem, isRecord := parseDischargeRow(line)
	if !isRecord {
		return false
	}
	if problem != "" {
		return false
	}
	if record.Shard != shard {
		return false
	}
	return record.Line == number
}

// row renders one record as a markdown table row.
func (r dischargeRecord) row() string {
	var text textbuf.Buffer
	text.Str("| ").Str(r.Date).Str(" | ").Str(r.Shard).Str(" | ").Int(int64(r.Line)).
		Str(" | ").Str(r.Digest).Str(" | ").Str(r.Kind).
		Str(" | ").Str(dischargeCell(strings.Join(r.Commits, " "))).
		Str(" | ").Str(dischargeCell(r.Artifact)).
		Str(" | ").Str(dischargeCell(r.Owner)).Str(" |")
	return text.String()
}

// readDischargeRecords reads every record file, and answers the lines that look
// like records and are not.
func readDischargeRecords(root string) ([]dischargeRecord, []string) {
	dir := filepath.Join(root, filepath.FromSlash(debtDir), dischargeDirName)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []string{"read " + dischargePath("*") + ": " + err.Error()}
	}
	records := make([]dischargeRecord, 0)
	invalid := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name())) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if err != nil {
			invalid = append(invalid, entry.Name()+": "+err.Error())
			continue
		}
		for number, line := range strings.Split(string(content), "\n") {
			record, kind, ok := parseDischargeRow(line)
			if !ok {
				continue
			}
			if kind != "" {
				invalid = append(invalid, entry.Name()+":"+strconv.Itoa(number+1)+": "+kind)
				continue
			}
			records = append(records, record)
		}
	}
	return records, invalid
}

// parseDischargeRow reads one line. A line is a RECORD when it has the row's
// cell count and a declared kind, so the table head and the prose are not
// records; a record with an unusable digest or line is a record that is WRONG,
// and it is reported rather than ignored.
func parseDischargeRow(line string) (record dischargeRecord, problem string, isRecord bool) {
	cells := strings.Split(line, "|")
	if len(cells) != 10 {
		return dischargeRecord{}, "", false
	}
	for index := range cells {
		cells[index] = strings.TrimSpace(cells[index])
	}
	if !slices.Contains(dischargeKinds, cells[5]) {
		return dischargeRecord{}, "", false
	}
	number, err := strconv.Atoi(cells[3])
	if err != nil || number < 1 {
		return dischargeRecord{}, "line " + strconv.Quote(cells[3]) + " is not a line number", true
	}
	if !isRowDigest(cells[4]) {
		return dischargeRecord{}, "digest " + strconv.Quote(cells[4]) +
			" is not 64 hexadecimal characters", true
	}
	if err := checkShardName(cells[2]); err != nil {
		return dischargeRecord{}, err.Error(), true
	}
	artifact := dischargeValue(cells[7])
	if err := checkArtifactPath(artifact); err != nil {
		return dischargeRecord{}, err.Error(), true
	}
	// One cell holds every commit the row covers, separated by spaces. A
	// revision holds no space, so the split is exact, and a record written when
	// the cell held one commit reads back as a one-commit list.
	commits := strings.Fields(dischargeValue(cells[6]))
	for _, revision := range commits {
		if err := checkRevision(revision); err != nil {
			return dischargeRecord{}, err.Error(), true
		}
	}
	return dischargeRecord{
		Date: cells[1], Shard: cells[2], Line: number, Digest: cells[4], Kind: cells[5],
		Commits: commits, Artifact: artifact, Owner: dischargeValue(cells[8]),
	}, "", true
}

// isRowDigest answers whether a cell is a SHA-256 in lowercase hexadecimal.
func isRowDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for index := range len(value) {
		digit := value[index]
		hexadecimal := (digit >= '0' && digit <= '9') || (digit >= 'a' && digit <= 'f')
		if !hexadecimal {
			return false
		}
	}
	return true
}

// dischargeCell renders a value into one table cell. A pipe and a newline are
// written as their character references, which is how a markdown table carries
// them, so the value round-trips through a cell split on "|" unchanged.
// dischargeValue is the inverse and MUST decode in the opposite order.
func dischargeCell(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "|", "&#124;")
	value = strings.ReplaceAll(value, "\r", "&#13;")
	return strings.ReplaceAll(value, "\n", "&#10;")
}

// dischargeValue reads a cell back. It MUST be called on every cell
// dischargeCell wrote, and it decodes the ampersand last.
func dischargeValue(cell string) string {
	cell = strings.ReplaceAll(cell, "&#10;", "\n")
	cell = strings.ReplaceAll(cell, "&#13;", "\r")
	cell = strings.ReplaceAll(cell, "&#124;", "|")
	return strings.ReplaceAll(cell, "&amp;", "&")
}
