// Design: docs/architecture/config/yang-config-design.md -- the heuristic, proved against a fixture
//
// selftest.go proves the leaf-mention heuristic against a fixture whose answer
// is known. An always-report check and an always-silent check both look like a
// working heuristic until something is known to be CONSUMED, so the fixture
// carries one leaf that must be reported and two that must not.
//
// The table is declared ONCE and read twice: `le yang leaf-mentions selftest`
// runs it, and the package test runs the same rows so a failure names the case
// rather than a count.

package yangleafmentions

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/leroot"
)

// fixtureYANG is a config module with three leaves: one read as a map key, one
// named inside a path literal, and one nothing names.
const fixtureYANG = `module ze-fixture-conf {
    container fixture {
        leaf read-leaf { type string; }
        leaf never-read-leaf { type string; }
        container inner {
            leaf-list also-read { type string; }
        }
    }
}`

// fixtureGo is the owning package: it names two of the fixture's leaves and
// never names the third.
const fixtureGo = `package fixture

func extract(m map[string]any) (string, []string) {
	v, _ := m["read-leaf"].(string)
	path := "fixture/inner/also-read"
	_ = path
	return v, nil
}`

// selftestCase is one property the scan of the fixture MUST have. check answers
// the empty string when it holds, and what the failure means otherwise.
type selftestCase struct {
	name  string
	check func(Report) string
}

// selftestCases is the whole selftest. Discovery, parsing and both polarities
// of the heuristic each get a row, so a failure names which of them broke.
var selftestCases = []selftestCase{
	{
		name: "module-discovered",
		check: func(rep Report) string {
			if rep.Modules == 1 {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("want 1 fixture module scanned, got ").Int(int64(rep.Modules)).
				Str(": discovery no longer finds a <pkg>/yang/<stem>-conf.yang").String()
		},
	},
	{
		name: "leaves-parsed",
		check: func(rep Report) string {
			if rep.Leaves == 3 {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("want 3 fixture leaves parsed, got ").Int(int64(rep.Leaves)).
				Str(": the leaf parser no longer matches leaf and leaf-list statements").String()
		},
	},
	{
		name: "unread-leaf-reported",
		check: func(rep Report) string {
			if _, ok := found(rep)["never-read-leaf"]; ok {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("the unread fixture leaf was not reported: ").Str(summary(rep)).String()
		},
	},
	{
		name: "leaf-path-carried",
		check: func(rep Report) string {
			path, ok := found(rep)["never-read-leaf"]
			if !ok || path == "fixture/never-read-leaf" {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("want leaf path ").Quoted("fixture/never-read-leaf").
				Str(", got ").Quoted(path).String()
		},
	},
	{
		name: "map-key-literal-not-reported",
		check: func(rep Report) string {
			if _, ok := found(rep)["read-leaf"]; !ok {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("a leaf named by a map key literal was reported: ").Str(summary(rep)).String()
		},
	},
	{
		name: "path-literal-not-reported",
		check: func(rep Report) string {
			if _, ok := found(rep)["also-read"]; !ok {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("a leaf named inside a path literal was reported: ").Str(summary(rep)).String()
		},
	},
	{
		name: "exactly-one-finding",
		check: func(rep Report) string {
			if len(rep.Findings) == 1 {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("want exactly 1 finding over the fixture, got ").
				Int(int64(len(rep.Findings))).Str(": ").Str(summary(rep)).String()
		},
	},
}

// found indexes a report by leaf name, which is how every case above asks
// whether one leaf was reported.
func found(rep Report) map[string]string {
	out := make(map[string]string, len(rep.Findings))
	for _, finding := range rep.Findings {
		out[finding.Leaf] = finding.Path
	}
	return out
}

// summary renders the findings a failing case has to show, as leaf=path pairs.
func summary(rep Report) string {
	var tb textbuf.Buffer
	tb.Byte('[')
	for i, finding := range rep.Findings {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(finding.Leaf).Byte('=').Str(finding.Path)
	}
	return tb.Byte(']').String()
}

// WriteFixture writes the selftest fixture under dir, in the layout discovery
// walks: an owning package with a yang/ sibling holding one config module.
//
// It is exported so the package test drives the same tree the action does,
// rather than a second copy of it.
func WriteFixture(dir string) error {
	owner := filepath.Join(dir, "internal", "plugins", "fixture")
	yangDir := filepath.Join(owner, "yang")
	if err := os.MkdirAll(yangDir, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(yangDir, "ze-fixture-conf.yang"), []byte(fixtureYANG), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(owner, "config.go"), []byte(fixtureGo), 0o600)
}

// Selftest scans the fixture and answers one row per declared property.
//
// The error is the fixture failing to be written or scanned, which is a
// different fact from the heuristic being wrong, so it is answered apart from
// the rows rather than as one more failing case.
func Selftest() (leroot.SelftestReport, error) {
	dir, err := os.MkdirTemp("", "yang-leaf-mentions-selftest")
	if err != nil {
		return leroot.SelftestReport{}, err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // temp fixture

	if err := WriteFixture(dir); err != nil {
		return leroot.SelftestReport{}, err
	}

	rep, err := ScanTree(dir)
	if err != nil {
		return leroot.SelftestReport{}, err
	}

	results := make([]leroot.SelftestResult, 0, len(selftestCases))
	for _, testCase := range selftestCases {
		if detail := testCase.check(rep); detail != "" {
			results = append(results, leroot.Fail(testCase.name, detail))
			continue
		}
		results = append(results, leroot.Pass(testCase.name))
	}

	return leroot.NewSelftestReport(
		"yang-leaf-mentions: selftest OK",
		"yang-leaf-mentions: selftest FAILED",
		results...,
	), nil
}

// runSelftest is the `le yang leaf-mentions selftest` action.
func runSelftest() (any, int) {
	report, err := Selftest()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, report.Code(1)
}
