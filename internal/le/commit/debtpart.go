// Design: docs/architecture/testing/verify-freshness-scope.md -- verification debt cleared one piece at a time
// Related: debt.go -- the ledger the proven pieces eventually clear
package commit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// debtPartsPath holds the pieces of one commit's owed verification that have
// exited 0. It sits beside the other `tmp/ze-verify-*` artifacts because it is
// the same kind of thing: a record of what a run proved about a tree.
const debtPartsPath = "tmp/ze-verify-debt-parts.json"

// debtPart is the piece of the owed verification one debt-clear pass runs.
// Both numbers count from one, so an uncut pass is part 1 of 1.
type debtPart struct {
	Index int
	Of    int
}

func uncutDebtPart() debtPart { return debtPart{Index: 1, Of: 1} }

// cut reports whether this pass runs one piece of several.
func (p debtPart) cut() bool { return p.Of != 1 }

// debtPartFrom reads the `part <n> of <m>` keywords.
//
// Naming neither keyword is the uncut pass, which runs the whole verification
// and clears on its own verdict. Naming one without the other is refused:
// "part 3" alone cannot say how many pieces the stages were dealt into, so a
// guess would run the wrong subset and record the wrong thing proven.
func debtPartFrom(values keywordValues) (debtPart, error) {
	index, hasIndex := values["part"], values.has("part")
	count, hasCount := values["of"], values.has("of")
	if !hasIndex && !hasCount {
		return uncutDebtPart(), nil
	}
	if hasIndex != hasCount {
		return debtPart{}, errors.New("a cut pass needs both keywords: debt-clear part <n> of <m>")
	}
	part, err := debtPartNumber("part", index[0])
	if err != nil {
		return debtPart{}, err
	}
	of, err := debtPartNumber("of", count[0])
	if err != nil {
		return debtPart{}, err
	}
	if part > of {
		return debtPart{}, fmt.Errorf("part %d is outside the 1 to %d the pass was cut into", part, of)
	}
	return debtPart{Index: part, Of: of}, nil
}

func debtPartNumber(keyword, declared string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(declared))
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s needs a whole number of 1 or more, got %q", keyword, declared)
	}
	return value, nil
}

// debtProgress is the durable record of which pieces of one commit's owed
// verification have exited 0.
type debtProgress struct {
	Commit  string `json:"commit"`
	Of      int    `json:"of"`
	Passed  []int  `json:"passed"`
	Updated string `json:"updated"`
}

// recordDebtPart adds one piece that exited 0 to the progress record, and
// answers every piece proven for commit under this cut.
//
// The record is pinned to the COMMIT and to the CUT, and either one moving
// starts it again. A verdict is evidence about the tree it ran on, so once HEAD
// moves the earlier pieces describe a tree nobody is clearing debt for. And a
// different piece count deals the stages differently, so pieces of two cuts
// never add up to the population.
//
// Several sessions share this checkout, so the read, the merge and the write
// happen under one advisory lock, exactly as the ledger shards do.
func recordDebtPart(root, commit string, part debtPart) ([]int, error) {
	if commit == "" {
		return nil, errors.New("a proven piece needs the commit it judged")
	}
	path := filepath.Join(root, filepath.FromSlash(debtPartsPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // the path is this checkout's verification artifact
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // the explicit write/sync result owns the verdict
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN) //nolint:errcheck // process exit releases the advisory lock

	content, err := os.ReadFile(path) //nolint:gosec // the path is this checkout's verification artifact
	if err != nil {
		return nil, err
	}
	progress := debtProgress{Commit: commit, Of: part.Of, Passed: []int{}}
	var held debtProgress
	if len(content) != 0 && json.Unmarshal(content, &held) == nil &&
		held.Commit == commit && held.Of == part.Of {
		progress.Passed = held.Passed
	}
	if !slices.Contains(progress.Passed, part.Index) {
		progress.Passed = append(progress.Passed, part.Index)
	}
	slices.Sort(progress.Passed)
	progress.Updated = time.Now().UTC().Format(time.RFC3339)

	rendered, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return nil, err
	}
	rendered = append(rendered, '\n')
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}
	if _, err := file.Write(rendered); err != nil {
		return nil, err
	}
	if err := file.Truncate(int64(len(rendered))); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	return progress.Passed, nil
}

// debtPartsComplete reports whether every piece of a cut into of pieces is
// proven. Nothing clears until it answers true.
func debtPartsComplete(passed []int, of int) bool {
	for index := 1; index <= of; index++ {
		if !slices.Contains(passed, index) {
			return false
		}
	}
	return true
}

// forgetDebtParts drops the progress record once its pieces have cleared their
// rows. A record kept past that would credit a row written later, at the same
// HEAD, to a verification that ran before it existed.
func forgetDebtParts(root string) error {
	err := os.Remove(filepath.Join(root, filepath.FromSlash(debtPartsPath)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
