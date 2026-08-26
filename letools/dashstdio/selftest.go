// Design: docs/architecture/cli/command-namespacing.md -- the guard, proved against fixtures
//
// selftest.go builds isolated fixtures reproducing the pre-migration violation
// shapes (each MUST be flagged) and the legitimate shapes (each MUST NOT be
// flagged), then scans them. It proves the detector FIRES, which is the
// requirement a gate over a migrated tree cannot meet on its own: a guard that
// stopped detecting and a tree with nothing to detect print the same page.
//
// The fixtures are consts rather than files in this package, so this package's
// own source carries no bare CLI-arg raw-os call for the guard-of-the-guard to
// trip on.
//
// The table is declared ONCE and read twice: `le dash-stdio selftest` runs it,
// and the package test runs the same rows so a failure names the case rather
// than a count.

package dashstdio

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/leroot"
)

// argsetDecl is the fake flag set every fixture calls into. It is declared once
// because fourteen fixtures would otherwise carry fourteen copies of it.
const argsetDecl = `
type argset struct{}
func (argset) Arg(int) string { return "" }
func (argset) Args() []string { return nil }
func (argset) String(a, b, c string) *string { return nil }
`

// selftestCase is one fixture package and what the guard must say about it.
type selftestCase struct {
	// name is the directory the fixture is written to, and the word a failure
	// names.
	name string
	// source is the Go file the guard is pointed at.
	source string
	// flagged is whether the fixture must draw at least one finding.
	flagged bool
	// why says what a failure of this case would mean, and it is the text the
	// report carries.
	why string
}

// selftestCases is the whole selftest. Nine fixtures MUST be flagged and five
// must not, which is what makes the guard's silence over the real tree mean
// something.
var selftestCases = []selftestCase{
	{
		name: "direct",
		source: `package p
import "os"
func cmd(fs argset) {
	if _, err := os.ReadFile(fs.Arg(0)); err != nil {
		return
	}
}
` + argsetDecl,
		flagged: true,
		why:     "direct fs.Arg read not flagged",
	},
	{
		name: "alias",
		source: `package p
import "os"
func cmd(fs argset) {
	pth := fs.Arg(0)
	if _, err := os.ReadFile(pth); err != nil {
		return
	}
}
` + argsetDecl,
		flagged: true,
		why:     "aliased CLI-arg read not flagged",
	},
	{
		name: "rangef",
		source: `package p
import "os"
func cmd(fs argset) {
	for _, pth := range fs.Args() {
		if _, err := os.ReadFile(pth); err != nil {
			return
		}
	}
}
` + argsetDecl,
		flagged: true,
		why:     "range-over-args read not flagged",
	},
	{
		name: "flagderef",
		source: `package p
import "os"
func cmd(fs argset) {
	out := fs.String("o", "", "")
	if _, err := os.Create(*out); err != nil {
		return
	}
}
` + argsetDecl,
		flagged: true,
		why:     "flag-pointer deref write not flagged",
	},
	{
		name: "funnel",
		source: `package p
import "os"
func load(pth string) {
	if _, err := os.Open(pth); err != nil {
		return
	}
}
func cmd(fs argset) { load(fs.Arg(0)) }
` + argsetDecl,
		flagged: true,
		why:     "funnel-parameter read not flagged",
	},
	{
		name: "twohop",
		source: `package p
import "os"
func inner(pth string) {
	if _, err := os.Open(pth); err != nil {
		return
	}
}
func outer(pth string) { inner(pth) }
func cmd(fs argset) { outer(fs.Arg(0)) }
` + argsetDecl,
		flagged: true,
		why:     "two-hop funnel-parameter read not flagged",
	},
	{
		name: "aliasedos",
		source: `package p
import fsys "os"
func cmd(fs argset) {
	if _, err := fsys.ReadFile(fs.Arg(0)); err != nil {
		return
	}
}
` + argsetDecl,
		flagged: true,
		why:     "aliased os import (import fsys \"os\") read not flagged",
	},
	{
		name: "osargs",
		source: `package p
import "os"
func cmd() {
	if _, err := os.ReadFile(os.Args[1]); err != nil {
		return
	}
}
`,
		flagged: true,
		why:     "os.Args index read not flagged",
	},
	{
		name: "writef",
		source: `package p
import "os"
func cmd(fs argset, data []byte) {
	if err := os.WriteFile(fs.Arg(1), data, 0o600); err != nil {
		return
	}
}
` + argsetDecl,
		flagged: true,
		why:     "fs.Arg write not flagged",
	},
	{
		name: "derived",
		source: `package p
import "os"
func cmd(fs argset) {
	pth := fs.Arg(0)
	derived := pth[1:]
	if _, err := os.ReadFile(derived); err != nil {
		return
	}
}
` + argsetDecl,
		why: "derived (sliced) path wrongly flagged",
	},
	{
		name: "helper",
		source: `package p
func cmd(fs argset) {
	if _, err := cliio.ReadFile(fs.Arg(0)); err != nil {
		return
	}
}
` + argsetDecl,
		why: "cliio call wrongly flagged",
	},
	{
		name: "joined",
		source: `package p
import (
	"os"
	"path/filepath"
	"strings"
)
func cmd(dir string) {
	if _, err := os.ReadFile(filepath.Join(dir, "x")); err != nil {
		return
	}
}
`,
		why: "filepath.Join path wrongly flagged",
	},
	{
		name: "store",
		source: `package p
func cmd(store backend, fs argset) {
	if _, err := store.ReadFile(fs.Arg(0)); err != nil {
		return
	}
}
type backend interface{ ReadFile(string) ([]byte, error) }
` + argsetDecl,
		why: "store.ReadFile (storage abstraction) wrongly flagged",
	},
	{
		name: "allowmarker",
		source: `package p
import "os"
func cmd(fs argset) {
	if _, err := os.ReadFile(fs.Arg(0)); err != nil { //cliio:allow reason: never "-"
		return
	}
}
` + argsetDecl,
		why: "cliio:allow-marked site wrongly flagged",
	},
}

// WriteFixture writes every fixture package under dir, each in its own
// subdirectory so their names never clash.
func WriteFixture(dir string) error {
	for _, testCase := range selftestCases {
		path := filepath.Join(dir, testCase.name, "x.go")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(testCase.source), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// Selftest writes the fixtures, scans them, and answers one row per case.
//
// The error is a fixture that could not be written or scanned, which is a
// different fact from a guard that stopped detecting, so it is answered apart
// from the rows rather than as one more failing case.
func Selftest() (leroot.SelftestReport, error) {
	dir, err := os.MkdirTemp("", "dash-stdio-selftest")
	if err != nil {
		return leroot.SelftestReport{}, err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // temp fixture

	if err := WriteFixture(dir); err != nil {
		return leroot.SelftestReport{}, err
	}

	findings, err := scan(dir, []string{dir}, 0)
	if err != nil {
		return leroot.SelftestReport{}, err
	}

	drawn := map[string]int{}
	for _, finding := range findings {
		drawn[strings.Split(finding.File, "/")[0]]++
	}

	results := make([]leroot.SelftestResult, 0, len(selftestCases))
	for _, testCase := range selftestCases {
		if (drawn[testCase.name] > 0) != testCase.flagged {
			results = append(results, leroot.Fail(testCase.name, testCase.why))
			continue
		}
		results = append(results, leroot.Pass(testCase.name))
	}

	return leroot.NewSelftestReport(
		"cli-dash-stdio selftest OK",
		"cli-dash-stdio selftest FAILED:",
		results...,
	), nil
}

// runSelftest is the `le dash-stdio selftest` action.
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
