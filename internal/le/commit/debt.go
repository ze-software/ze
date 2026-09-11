// Design: docs/architecture/testing/verify-freshness-scope.md -- verification debt follows local commits to push
package commit

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const debtDir = "plan/verification-debt"

// debtGates declares every gate a commit can owe: the keyword that overrides
// it, the name its row carries, whether a verification can RE-RUN it, and the
// names earlier eras of this command wrote for the same gate.
//
// Runnable is what `le commit debt-clear` reads. A review and an owner approval
// are acts a person performs, so no verification produces them and a green run
// says nothing about a row that names one. Declaring the fact here rather than
// comparing two string literals at the point of use keeps ONE statement of
// which gates a machine can answer.
//
// Aliases are DECLARED, never a widened fallback. A gate string that is neither
// a Name nor an alias is unrecognized, and an unrecognized row is not cleared
// by a green verify: a name nobody declared says nothing about what ran.
var debtGates = []struct {
	Key      string
	Name     string
	Runnable bool
	Aliases  []string
}{
	{gateUnverified, "full native verification (not FRESH-green)", true, []string{
		"./le verify current mode full (not FRESH-green)",
	}},
	{gateStructuralRedOK, "native structural checks (red)", true, []string{
		"./le verify current mode full structural gates (red)",
	}},
	{gateMissingFullVerifyOK, "full native verification over this commit's Go", true, []string{
		"full ./le verify current mode full over this commit's Go",
	}},
	// RETIRED as an override on 2026-09-11: ai/PACKAGE-MAP.md is derived rather
	// than tracked, the gate that compared it is deleted, and `stale-index-ok`
	// is no longer a `le commit create` keyword, so no commit can owe this row
	// again.
	//
	// The DECLARATION stays because the ledger already holds rows naming it.
	// debtGateAt answers -1 for a gate no table declares, an unrecognized row
	// is never cleared by a green verify, and TestEveryLedgerGateNameIsDeclared
	// (ledger_test.go) refuses a ledger row nothing declares, so deleting this
	// line strands every one of those rows for good. How many there are is what
	// `grep -rn "discovery-index freshness" plan/verification-debt/` answers,
	// over the tree in hand; a count written here drifts with every commit any
	// session lands.
	//
	// Runnable is FALSE because it is now false. `debt-clear` clears a row by
	// re-running the gate it names, and there is no longer a gate to run, so
	// `true` promised a clearance no command can perform and, worse, refused
	// the one route that stays open: verifyDischarge (discharge.go) rejects a
	// discharge for a runnable gate. Every open row naming this gate WAS then
	// discharged, under `kind owner` and the owner's authorisation of
	// 2026-09-11.
	//
	// Their shards still read `open`, and that is not a half-done sweep: a
	// discharge is an overlay applied at READ time by applyDischarges, so the
	// shard on disk never moves. `./le commit debt-status` is what answers,
	// and a grep of plan/verification-debt/ is not, twice over: it reads the
	// pre-overlay text, and the discharge RECORD lives under that directory
	// and holds one row per discharge, so a grep counts its own output.
	{gateStaleIndexOK, "discovery-index freshness", false, nil},
	{gateReviewOverride, "independent critical review", false, nil},
	{gateBrokenHeadFix, "repository tracked-build/check (HEAD does not compile)", true, []string{
		"repository-tracked-build/check (HEAD does not compile)",
		"./le repository tracked-build check (HEAD does not compile)",
	}},
	{gateRFCChangeOK, "owner approval for an RFC-tagged test change", false, nil},
}

// debtGateAt answers the row of debtGates a ledger gate string names, and -1
// when the string is declared neither as a Name nor as an alias.
//
// The -1 is the refusal and every caller MUST read it: index 0 is a real gate,
// so a caller that skips the check indexes the table with -1 and panics rather
// than answering about the wrong gate.
func debtGateAt(name string) int {
	for index, gate := range debtGates {
		if gate.Name == name || slices.Contains(gate.Aliases, name) {
			return index
		}
	}
	return -1
}

// Debt is one verification obligation: open, cleared, or discharged.
//
// open and cleared are the only statuses the ledger WRITES. discharged exists
// in memory alone, produced by the discharge overlay in readDebt, so the 3526
// rows on disk stay behind one parser and no consumer needs a second rule.
type Debt struct {
	Shard             string `json:"shard"`
	Line              int    `json:"line"`
	Date              string `json:"date"`
	Session           string `json:"session"`
	Subject           string `json:"subject"`
	Gate              string `json:"gate"`
	Reason            string `json:"reason"`
	Status            string `json:"status"`
	DischargeKind     string `json:"discharge-kind,omitempty"`
	DischargeEvidence string `json:"discharge-evidence,omitempty"`
	Raw               string `json:"-"`
}

// DebtLedger is every debt row with the discharge overlay applied, beside the
// discharge records that could NOT be applied.
//
// The invalid records travel with the rows because a record that no longer
// derives leaves its row open, and a reader who sees only the open row has no
// way to learn that a discharge was claimed for it.
type DebtLedger struct {
	Rows    []Debt
	Invalid []string
}

func debtPath(session string) string {
	return debtShardPath(session + ".md")
}

// debtShardPath is the ledger path of one shard. Every commit a row covers
// WROTE that file: recordDebt writes it and the commit script commits it, which
// is what binds a commit to the rows of one shard.
func debtShardPath(shard string) string {
	return filepath.ToSlash(filepath.Join(debtDir, shard))
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
	for index := range owed {
		gate, reason := debtCell(owed[index].Gate), debtCell(owed[index].Reason)
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

// ListDebt returns every valid debt row from every shard in stable order, with
// the discharge overlay applied. It is the ONE producer of "is this row open":
// openDebt, clearDebtRows, refusePushWithDebt and the session-start hook all
// read it and hold no rule of their own.
func ListDebt(root string) ([]Debt, error) {
	ledger, err := readDebt(root)
	return ledger.Rows, err
}

// readDebt is ListDebt with the unapplied discharge records kept, for the one
// caller that reports them. Splitting the two keeps every other consumer on a
// signature that cannot ignore the overlay.
func readDebt(root string) (DebtLedger, error) {
	rows, err := readDebtRows(root)
	if err != nil {
		return DebtLedger{}, err
	}
	invalid := applyDischarges(root, rows)
	return DebtLedger{Rows: rows, Invalid: invalid}, nil
}

// readDebtRows returns every valid debt row from every shard in stable order,
// exactly as the shards hold it.
func readDebtRows(root string) ([]Debt, error) {
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
	if status != statusOpen && status != statusCleared {
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
	for index := range rows {
		if rows[index].Status == statusOpen {
			open = append(open, rows[index])
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
	for index := range rows {
		row := &rows[index]
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
			cells[6] = " " + statusCleared + " "
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
	statusCleared           = "cleared"
	statusDischarged        = "discharged"
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
