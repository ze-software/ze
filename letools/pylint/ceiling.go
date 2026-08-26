// Design: docs/contributing/ze-python-style.md -- the legacy ratchet's one number
//
// ceiling.go reads the most legacy findings this repository currently tolerates.
//
// The number and its rule set are one fact, so both remain in this data file.
// A commit that lowers the ceiling shows both changes together.
// A Go literal would duplicate the record without any comparison to detect drift.
//
// The reader accepts only the table that owns this value.
// A silent zero default makes the ratchet impossible to pass.
// A silent large default disables the ratchet. Both conditions return errors.

package pylint

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Pyproject is where the ceiling is recorded, relative to the checkout.
const Pyproject = "pyproject.toml"

// The table and the key the ceiling lives under.
const (
	ceilingTable = "[tool.le.lint]"
	ceilingKey   = "legacy-max"
)

// ErrNoCeiling says pyproject.toml declares no integer ceiling under its own
// table.
var ErrNoCeiling = errors.New("pyproject.toml has no integer [tool.le.lint] legacy-max")

// LegacyCeiling answers the most legacy findings this checkout tolerates.
//
// The parser intentionally accepts one table, one key, and one integer.
// A broader parser would add a dependency but would not help parse this two-line file format.
func LegacyCeiling(root string) (int, error) {
	path := filepath.Join(root, Pyproject)

	raw, err := os.ReadFile(path) //nolint:gosec // a build tool reads the checkout it was pointed at
	if err != nil {
		return 0, err
	}

	inTable := false
	for _, line := range splitLines(string(raw)) {
		trimmed := strings.TrimSpace(line)

		// A table header ends the previous table, whichever one it opens. That
		// is what stops a legacy-max under another tool being read as this one.
		if strings.HasPrefix(trimmed, "[") {
			inTable = trimmed == ceilingTable
			continue
		}
		if !inTable {
			continue
		}

		key, value, found := strings.Cut(trimmed, "=")
		if !found || strings.TrimSpace(key) != ceilingKey {
			continue
		}
		count, ok := parseCount(strings.TrimSpace(value))
		if !ok {
			break
		}
		return count, nil
	}

	var tb textbuf.Buffer
	return 0, errors.New(tb.Str(path).Str(": ").Err(ErrNoCeiling).String())
}
