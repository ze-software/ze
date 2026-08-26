// Design: docs/architecture/core-design.md -- the persistence guard, proved against fixtures
//
// selftest.go proves the AST detection independent of the live tree: eight
// fixtures, four that MUST be flagged and four that must not. A guard that
// reported nothing because its detection broke, and a guard over a clean tree,
// print the same page, and this is what tells them apart.
//
// The fixtures are consts rather than files in this package, so this package's
// own source carries no bare raw-write call for the guard-of-the-guard to trip
// on.
//
// The table is declared ONCE and read twice: `le fs-persistence selftest` runs
// it, and the package test runs the same rows so a failure names the case
// rather than a count.

package fspersistence

import (
	"go/token"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/leroot"
)

// selftestCase is one fixture and what the guard must say about it.
type selftestCase struct {
	// name is the directory the fixture is written to, and the word a failure
	// names.
	name string
	// source is the Go file the guard is pointed at.
	source string
	// want is the number of findings the guard must draw. Zero means the
	// fixture is correct code the guard must leave alone.
	want int
	// why says what a failure of this case would mean, and it is the text the
	// report carries.
	why string
}

// selftestCases is the whole selftest. Four fixtures MUST be flagged and four
// must not, which is what makes the guard's silence over the real tree mean
// something.
var selftestCases = []selftestCase{
	{
		name: "state-write",
		source: `package p
import "os"
func save(path, tmp string, data []byte) error {
	if werr := os.WriteFile(tmp, data, 0o600); werr != nil {
		return werr
	}
	return os.Rename(tmp, path)
}
`,
		want: 2,
		why:  "os.WriteFile + os.Rename state write not both flagged",
	},
	{
		name: "good",
		source: `package p
import (
	"os"
	"github.com/ze-software/ze/internal/core/statestore"
)
func save(key string, data []byte) (bool, error) { return statestore.Put(key, data) }
func load(path string) ([]byte, error) { return os.ReadFile(path) }
func has(path string) bool { _, serr := os.Stat(path); return serr == nil }
`,
		why: "statestore.Put / os.ReadFile / os.Stat wrongly flagged",
	},
	{
		name: "open-read",
		source: `package p
import "os"
func read(p string) (*os.File, error) { return os.OpenFile(p, os.O_RDONLY, 0) }
`,
		why: "os.OpenFile O_RDONLY wrongly flagged",
	},
	{
		name: "open-write",
		source: `package p
import "os"
func w(p string) (*os.File, error) { return os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644) }
`,
		want: 1,
		why:  "os.OpenFile with a write flag not flagged",
	},
	{
		name: "non-write",
		source: `package p
import "os"
func c(dir string) error {
	if merr := os.MkdirAll(dir, 0o755); merr != nil {
		return merr
	}
	if rerr := os.Remove(dir); rerr != nil {
		return rerr
	}
	f, terr := os.CreateTemp(dir, "x")
	if terr != nil {
		return terr
	}
	return f.Close()
}
`,
		why: "os.MkdirAll/Remove/CreateTemp wrongly flagged",
	},
	{
		name: "var-flag",
		source: `package p
import "os"
func w(p string) (*os.File, error) {
	mode := os.O_WRONLY | os.O_CREATE
	return os.OpenFile(p, mode, 0o644)
}
`,
		want: 1,
		why:  "os.OpenFile with a variable flag not flagged",
	},
	{
		name: "aliased",
		source: `package p
import fsys "os"
func save(p string, d []byte) error { return fsys.WriteFile(p, d, 0o600) }
`,
		want: 1,
		why:  "aliased os.WriteFile not flagged",
	},
	{
		name: "rd-nonblock",
		source: `package p
import (
	"os"
	"syscall"
)
func read(p string) (*os.File, error) { return os.OpenFile(p, os.O_RDONLY|syscall.O_NONBLOCK, 0) }
`,
		why: "O_RDONLY|O_NONBLOCK read wrongly flagged",
	},
}

// Selftest writes each fixture and answers one row per case.
//
// The error is a fixture that could not be written or parsed, which is a
// different fact from a guard that stopped detecting, so it is answered apart
// from the rows rather than as one more failing case.
func Selftest() (leroot.SelftestReport, error) {
	dir, err := os.MkdirTemp("", "fs-persistence-selftest")
	if err != nil {
		return leroot.SelftestReport{}, err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // temp fixture

	fset := token.NewFileSet()
	results := make([]leroot.SelftestResult, 0, len(selftestCases))
	for _, testCase := range selftestCases {
		path := filepath.Join(dir, testCase.name, "k.go")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return leroot.SelftestReport{}, err
		}
		if err := os.WriteFile(path, []byte(testCase.source), 0o600); err != nil {
			return leroot.SelftestReport{}, err
		}
		found, err := ScanFile(fset, path, testCase.name)
		if err != nil {
			return leroot.SelftestReport{}, err
		}
		if len(found) != testCase.want {
			results = append(results, leroot.Fail(testCase.name, testCase.why))
			continue
		}
		results = append(results, leroot.Pass(testCase.name))
	}

	return leroot.NewSelftestReport(
		"direct-fs-persistence selftest OK",
		"direct-fs-persistence selftest FAILED:",
		results...,
	), nil
}

// runSelftest is the `le fs-persistence selftest` action.
func runSelftest() (any, int) {
	report, err := Selftest()
	if err != nil {
		// 2 rather than 1: a fixture that could not be written is a different
		// fact from a guard that stopped detecting.
		leaction.ReportError(err)
		return nil, 2
	}
	return report, report.Code(1)
}
