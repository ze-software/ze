// Design: docs/architecture/doctor-and-health-checks.md -- the gate, proved against fixtures
// Related: detail.go -- the failure sentences a case answers
//
// selftest.go proves the comparison independent of the live tree: eight cases,
// each naming one thing the gate must say. A gate whose comparison broke and a
// gate over a table that agrees print the same page, and this is what tells them
// apart.
//
// The table is declared ONCE and read twice: `le port-defaults selftest` runs
// it, and portdefaults_test.go runs the same rows so a failure names the case
// rather than a line of output.

package portdefaults

import (
	"fmt"
	"strconv"

	"github.com/ze-software/ze/letools/leroot"
)

// selftestCase is one synthetic comparison and what it must answer.
type selftestCase struct {
	// name is the word a failure names.
	name string
	// run answers the detail of a failure, or "" when the case behaved.
	run func() string
}

// moduleFixture is the module path every synthetic case names. The path itself
// carries no meaning here: what a case is about is the content behind it.
const moduleFixture = "x.yang"

// reader answers a readFile over a fixed map, so a case names module content
// rather than a tree.
func reader(modules map[string]string) readFile {
	return func(path string) (string, error) {
		content, ok := modules[path]
		if !ok {
			return "", fmt.Errorf("no such file: %s", path)
		}
		return content, nil
	}
}

// selftestCases is the whole selftest. Cases 6 to 8 are about the READER rather
// than the comparison: a registration spelling the gate cannot read costs a
// whole service, which is how l2tp went unchecked while the gate stayed green.
var selftestCases = []selftestCase{
	{"match", func() string {
		drifts := compare(
			map[string]int{"x": 100},
			map[string]string{"x": moduleFixture},
			reader(map[string]string{moduleFixture: "uses zt:listener { refine port { default 100; } }"}),
		)
		if len(drifts) != 0 {
			return detail("case1 (match): expected 0 drifts, got ", len(drifts))
		}
		return ""
	}},
	{"drift", func() string {
		drifts := compare(
			map[string]int{"x": 100},
			map[string]string{"x": moduleFixture},
			reader(map[string]string{moduleFixture: "refine port { default 200; }"}),
		)
		if len(drifts) != 1 || drifts[0].Reason != ReasonMismatch || drifts[0].GoPort != 100 || drifts[0].YANGPort != 200 {
			return describe("case2 (drift): unexpected result ", drifts)
		}
		return ""
	}},
	{"unmapped", func() string {
		drifts := compare(map[string]int{"y": 100}, map[string]string{}, reader(map[string]string{}))
		if len(drifts) != 1 || drifts[0].Reason != ReasonUnmapped || drifts[0].Service != "y" {
			return describe("case3 (unmapped): unexpected result ", drifts)
		}
		return ""
	}},
	{"no-default", func() string {
		drifts := compare(
			map[string]int{"x": 100},
			map[string]string{"x": moduleFixture},
			reader(map[string]string{moduleFixture: "container x { leaf name { type string; } }"}),
		)
		if len(drifts) != 1 || drifts[0].Reason != ReasonNoDefault {
			return describe("case4 (no-default): unexpected result ", drifts)
		}
		return ""
	}},
	{"stale-map", func() string {
		drifts := compare(
			map[string]int{},
			map[string]string{"gone": "gone.yang"},
			reader(map[string]string{"gone.yang": "refine port { default 1; }"}),
		)
		if len(drifts) != 1 || drifts[0].Reason != ReasonStaleMap || drifts[0].Service != "gone" {
			return describe("case5 (stale map): unexpected result ", drifts)
		}
		return ""
	}},
	{"both-kinds-read", func() string {
		table := map[string]int{}
		for _, match := range goEntryRe.FindAllStringSubmatch(centralTableFixture, -1) {
			port, _ := strconv.Atoi(match[2]) //nolint:errcheck // the regex captured digits
			table[match[1]] = port
		}
		if len(table) != 2 || table["plain"] != 111 || table["entryonly"] != 222 {
			return detail("case6 (both kinds read): read ", len(table))
		}
		return ""
	}},
	{"known-kinds-and-comments", func() string {
		if unknown := unknownRegistrations(centralTableFixture); len(unknown) != 0 {
			return describeStrings("case7 (known kinds and comments): got ", unknown)
		}
		return ""
	}},
	{"unknown-kind-refused", func() string {
		unknown := unknownRegistrations(dualStackFixture)
		if len(unknown) != 1 || unknown[0] != "RegisterListenerDefaultIPs(" {
			return describeStrings("case8 (unknown kind refused): got ", unknown)
		}
		if len(goEntryRe.FindAllStringSubmatch(dualStackFixture, -1)) != 0 {
			return "case8 (unknown kind refused): goEntryRe matched the dual-stack kind"
		}
		return ""
	}},
}

// centralTableFixture is a central table using both readable spellings, plus a
// PROSE mention of the unreadable one. The mention is what case 7 is about: the
// real file names these functions in comments to say which one to use.
const centralTableFixture = `func RegisterBuiltinListenerDefaults() {
	RegisterListenerDefault("plain", "0.0.0.0", "111")
	RegisterListenerEntryDefault("entryonly", "0.0.0.0", "222")
	// see RegisterListenerDefaultIPs( for the dual-stack kind
}`

// dualStackFixture is the live example of a spelling the reader cannot parse:
// its second argument is a slice, so there is no single ip literal to compare.
const dualStackFixture = `	RegisterListenerDefaultIPs("dual", []string{"127.0.0.1", "::1"}, "53")`

// Selftest runs every case and answers one result per case.
func Selftest() leroot.SelftestReport {
	results := make([]leroot.SelftestResult, 0, len(selftestCases))
	for _, testCase := range selftestCases {
		if failure := testCase.run(); failure != "" {
			results = append(results, leroot.Fail(testCase.name, failure))
			continue
		}
		results = append(results, leroot.Pass(testCase.name))
	}
	return leroot.NewSelftestReport("port-defaults: selftest OK", "port-defaults: SELFTEST FAILED", results...)
}

// runSelftest is the `selftest` action. A failure answers 2, which is the code
// the script answered: a selftest that did not hold is a broken gate rather
// than a tree that drifted, and a caller reads the two apart.
func runSelftest() (any, int) {
	report := Selftest()
	return report, report.Code(2)
}
