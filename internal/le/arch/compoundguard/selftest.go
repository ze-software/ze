// Design: docs/contributing/ze-go-style.md -- the compound-guard gate, proved against fixtures
//
// selftest.go proves the detection independent of the live tree. A gate that
// reports nothing because its detection broke, and a gate over a clean change
// set, print the same page, and this is what tells them apart.
//
// The table is declared ONCE and read twice: `le arch compound-guard selftest`
// runs it, and the package test runs the same rows so a failure names the case.

package archcompoundguard

import (
	"go/token"
	"os"
	"path/filepath"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// selftestCase is one fixture and what the gate must say about it.
type selftestCase struct {
	// name is the file the fixture is written to, and the word a failure names.
	name string
	// source is the Go file the scanner is pointed at.
	source string
	// want is the number of guards the scanner must report.
	want int
	// why says what a failure of this case would mean.
	why string
}

// selftestCases is the whole selftest.
var selftestCases = []selftestCase{
	{
		name: "return",
		source: `package p
func f(a, b bool) error {
	if a || b {
		return nil
	}
	return nil
}
`,
		want: 1,
		why:  "an || guard ending in return not flagged",
	},
	{
		name: "loop-exits",
		source: `package p
func f(xs []int) {
	for _, x := range xs {
		if x < 0 || x > 9 {
			continue
		}
		if x == 5 || x == 6 {
			break
		}
	}
}
`,
		want: 2,
		why:  "an || guard ending in continue or break not flagged",
	},
	{
		name: "method-paren-init",
		source: `package p
type T struct{}
func (t *T) m(v func() (int, error)) int {
	if n, err := v(); (err != nil || n == 0) {
		return 0
	}
	return 1
}
`,
		want: 1,
		why:  "a parenthesized || guard with an init statement, in a method, not flagged",
	},
	{
		name: "and",
		source: `package p
func f(a, b bool) error {
	if a && b {
		return nil
	}
	return nil
}
`,
		why: "an && guard wrongly flagged",
	},
	{
		name: "else",
		source: `package p
func f(a, b bool) int {
	if a || b {
		return 1
	} else {
		return 2
	}
}
`,
		why: "an || if with an else wrongly flagged",
	},
	{
		name: "falls-through",
		source: `package p
func f(a, b bool) int {
	n := 0
	if a || b {
		n++
	}
	return n
}
`,
		why: "an || if whose body falls through wrongly flagged",
	},
	{
		name: "nested-or",
		source: `package p
func f(a, b, c bool) error {
	if a && (b || c) {
		return nil
	}
	return nil
}
`,
		why: "an || below a top-level && wrongly flagged",
	},
	{
		name: "for-header",
		source: `package p
func f(a, b bool) {
	for a || b {
		return
	}
	switch {
	case a || b:
		return
	}
}
`,
		why: "an || in a for header or a switch case wrongly flagged",
	},
}

// Selftest writes each fixture and answers one row per case.
//
// The error is a fixture that could not be written or parsed, which is a
// different fact from a scanner that stopped detecting.
func Selftest() (leroot.SelftestReport, error) {
	dir, err := os.MkdirTemp("", "compound-guard-selftest")
	if err != nil {
		return leroot.SelftestReport{}, err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // temp fixture

	fset := token.NewFileSet()
	results := make([]leroot.SelftestResult, 0, len(selftestCases))
	for _, testCase := range selftestCases {
		path := filepath.Join(dir, testCase.name+".go")
		if err := os.WriteFile(path, []byte(testCase.source), 0o600); err != nil {
			return leroot.SelftestReport{}, err
		}
		found, err := ScanFile(fset, path)
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
		"compound-guard selftest OK",
		"compound-guard selftest FAILED:",
		results...,
	), nil
}

// runSelftest is the `le arch compound-guard selftest` action.
func runSelftest() (any, int) {
	report, err := Selftest()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, report.Code(1)
}
