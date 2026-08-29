// Design: docs/architecture/config/yang-config-design.md -- the guard, proved against fixtures
//
// selftest.go proves the AST detection independent of the live tree: four
// fixtures, two that MUST be flagged and two that must not. A guard that
// reported nothing because its detection broke, and a guard over a clean tree,
// print the same page, and this is what tells them apart.
//
// The table is declared ONCE and read twice: `le config coercion selftest` runs
// it, and configcoercion_test.go runs the same rows so a failure names the case
// rather than a count. A second copy of these fixtures is where the two would
// begin to disagree about what the guard catches.

package configcoercion

import (
	"go/token"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leroot"
)

// selftestCase is one fixture and what the guard must say about it.
type selftestCase struct {
	// name is the directory the fixture is written to, and the word a failure
	// names.
	name string
	// source is the config.go the guard is pointed at.
	source string
	// switches and asserts are the counts of each finding kind the guard must
	// draw. Both zero means the fixture is correct code the guard must leave
	// alone.
	switches int
	asserts  int
	// why says what a failure of this case would mean, and it is the text the
	// report carries.
	why string
}

// selftestCases is the whole selftest. Two fixtures MUST be flagged and two
// must not, which is what makes the guard's silence over the real tree mean
// something.
var selftestCases = []selftestCase{
	{
		name: "buggy_switch",
		source: `package p
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	}
	return 0, false
}
`,
		switches: 1,
		why:      "numeric type switch without case string: not flagged",
	},
	{
		name: "good_switch",
		source: `package p
import "strconv"
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case string:
		i, _ := strconv.Atoi(n)
		return i, true
	}
	return 0, false
}
`,
		why: "string-aware type switch wrongly flagged",
	},
	{
		name: "direct_bool",
		source: `package p
func parse(m map[string]any) bool {
	if b, ok := m["enabled"].(bool); ok {
		return b
	}
	return false
}
`,
		asserts: 1,
		why:     "direct v.(bool) config assertion not flagged",
	},
	{
		name: "good_bool",
		source: `package p
import "strconv"
func cfgBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		pb, _ := strconv.ParseBool(b)
		return pb, true
	}
	return false, false
}
`,
		why: "string-aware cfgBool wrongly flagged",
	},
}

// countKinds answers how many findings of each kind the guard draws over one
// fixture written into dir.
func (c selftestCase) countKinds(fset *token.FileSet, dir string) (switches, asserts int, err error) {
	rel := filepath.Join(c.name, "config.go")
	path := filepath.Join(dir, rel)
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o750); mkErr != nil {
		return 0, 0, mkErr
	}
	if writeErr := os.WriteFile(path, []byte(c.source), 0o600); writeErr != nil {
		return 0, 0, writeErr
	}

	found, err := ScanFile(fset, path, filepath.ToSlash(rel))
	if err != nil {
		return 0, 0, err
	}
	for _, finding := range found {
		if finding.Kind == KindTypeSwitch {
			switches++
			continue
		}
		asserts++
	}
	return switches, asserts, nil
}

// verdict scores one fixture against what it declared, and says what a failure
// means. It is a function of its own because the counts a fixture DRAWS come
// from the guard and the counts it WANTS come from the table, and mixing the
// two is how a selftest comes to report a pass it never computed.
func (c selftestCase) verdict(switches, asserts int) leroot.SelftestResult {
	if switches == c.switches && asserts == c.asserts {
		return leroot.Pass(c.name)
	}
	var tb textbuf.Buffer
	return leroot.Fail(c.name, tb.Str(c.why).Str(" (drew ").Int(int64(switches)).
		Str(" switch and ").Int(int64(asserts)).Str(" assert finding(s), want ").
		Int(int64(c.switches)).Str(" and ").Int(int64(c.asserts)).Byte(')').String())
}

// Selftest runs every fixture and answers one result per case.
//
// The error is about the fixture DIRECTORY rather than about the guard: a
// selftest that could not write its own fixtures has proved nothing, and
// answering an empty pass there is the failure this whole file exists to catch.
func Selftest() (leroot.SelftestReport, error) {
	dir, err := os.MkdirTemp("", "config-string-coercion-selftest-*")
	if err != nil {
		return leroot.SelftestReport{}, err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // temporary directory

	fset := token.NewFileSet()
	results := make([]leroot.SelftestResult, 0, len(selftestCases))
	for _, testCase := range selftestCases {
		switches, asserts, caseErr := testCase.countKinds(fset, dir)
		if caseErr != nil {
			return leroot.SelftestReport{}, caseErr
		}
		results = append(results, testCase.verdict(switches, asserts))
	}
	return leroot.NewSelftestReport(
		"config-string-coercion selftest OK",
		"config-string-coercion selftest FAILED:",
		results...), nil
}

// runSelftest is the `selftest` action.
func runSelftest() (any, int) {
	report, err := Selftest()
	if err != nil {
		reportError(err)
		return nil, 2
	}
	return report, report.Code(1)
}
