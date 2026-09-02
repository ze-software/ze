// Design: docs/architecture/core-design.md -- the boundary guard, proved against fixtures
//
// selftest.go proves the detection independent of the live tree: four package
// fixtures, two that MUST be flagged and two that must not, plus three
// properties of the scan-root derivation.
//
// It exists for the case the gate's own output cannot show. The real tree has
// zero dangerous calls made through a RENAMED import, so a regression in alias
// resolution would leave the gate silent and green; and a derivation that
// answered an empty list would do the same.
//
// The table is declared ONCE and read twice: `le plugin boundary selftest` runs
// it, and the package test runs the same rows so a failure names the case
// rather than a count.

package pluginboundary

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/leroot"
)

// ifacePkg is the owning import path every package fixture calls into, and
// registerFile is the file name each one declares it in.
const (
	ifacePkg     = ifacePackage
	registerFile = "register.go"
)

// packageFixture is one plugin package the guard is pointed at, and what it
// must say about that package.
type packageFixture struct {
	// name is the package directory under internal/plugins, and the word a
	// failure names.
	name string
	// files are the sources of that package, by file name.
	files map[string]string
	// flagged is whether the package must appear in the findings.
	flagged bool
	// why says what a failure of this case would mean, and it is the text the
	// report carries.
	why string
}

// packageFixtures is the scan half of the selftest. Two packages MUST be
// flagged and two must not, which is what makes the guard's silence over the
// real tree mean something.
var packageFixtures = []packageFixture{
	{
		name: "plain",
		files: map[string]string{registerFile: `package plain

import "` + ifacePkg + `"

func run() {
	iface.GetBackend()
}
`},
		flagged: true,
		why:     "default-name dangerous call not flagged",
	},
	{
		name: "aliased",
		files: map[string]string{registerFile: `package aliased

import ifcomp "` + ifacePkg + `"

func run() {
	ifcomp.GetBackend()
}
`},
		flagged: true,
		why:     "RENAMED-IMPORT dangerous call not flagged (alias resolution regressed)",
	},
	{
		name: "guardedaliased",
		files: map[string]string{
			registerFile: `package guardedaliased

import ifcomp "` + ifacePkg + `"

func run() {
	ifcomp.GetBackend()
}
`,
			// The guard sits in a DIFFERENT file of the same package, which is
			// what makes this a package-directory-wide presence check.
			"guard.go": `package guardedaliased

func checkInternal() {
	p.IsInternal()
}
`,
		},
		why: "guarded package (guard in a sibling file) wrongly flagged",
	},
	{
		name: "blankimport",
		files: map[string]string{registerFile: `package blankimport

import _ "` + ifacePkg + `"
`},
		why: "blank import wrongly flagged",
	},
}

// rootCase is one property the derived scan roots must have. check answers the
// empty string when it holds, and what the failure means otherwise.
type rootCase struct {
	name  string
	check func(roots []string) string
}

// rootCases is the derivation half of the selftest. The roots come from ONE
// place, internal/le/pluginimports, and these say that answer is usable: a
// derivation that returned nothing would leave the gate scanning nothing and
// reporting OK.
var rootCases = []rootCase{
	{
		name: "roots-derived",
		check: func(roots []string) string {
			if slices.Contains(roots, "internal/plugins") {
				return ""
			}
			return "the derived roots do not include internal/plugins -- the gate would scan no plugin at all"
		},
	},
	{
		name: "roots-deduplicated",
		check: func(roots []string) string {
			seen := make(map[string]bool, len(roots))
			for _, root := range roots {
				if seen[root] {
					return "a root is derived twice, so its files would be scanned twice"
				}
				seen[root] = true
			}
			return ""
		},
	},
	{
		name: "roots-expand-nested-domains",
		check: func(roots []string) string {
			for _, root := range roots {
				if strings.HasPrefix(root, "internal/component/") && strings.HasSuffix(root, "/plugins") {
					return ""
				}
			}
			return "no internal/component/<domain>/plugins root was derived -- nested-domain expansion regressed"
		},
	},
}

// WriteFixture writes the four package fixtures under dir and answers the tree
// root the scan is pointed at.
func WriteFixture(dir string) error {
	for _, fixture := range packageFixtures {
		for name, body := range fixture.files {
			path := filepath.Join(dir, "internal", "plugins", fixture.name, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				return err
			}
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
	dir, err := os.MkdirTemp("", "plugin-boundary-selftest")
	if err != nil {
		return leroot.SelftestReport{}, err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // temp fixture

	if err := WriteFixture(dir); err != nil {
		return leroot.SelftestReport{}, err
	}

	findings, _, err := scan(dir, []string{filepath.Join(dir, "internal", "plugins")}, 0)
	if err != nil {
		return leroot.SelftestReport{}, err
	}

	flagged := make(map[string]bool, len(findings))
	for _, finding := range findings {
		flagged[filepath.ToSlash(filepath.Dir(finding.File))] = true
	}

	results := make([]leroot.SelftestResult, 0, len(packageFixtures)+len(rootCases))
	for _, fixture := range packageFixtures {
		if flagged[filepath.ToSlash(filepath.Join("internal", "plugins", fixture.name))] != fixture.flagged {
			results = append(results, leroot.Fail(fixture.name, fixture.why))
			continue
		}
		results = append(results, leroot.Pass(fixture.name))
	}

	roots := Roots()
	for _, testCase := range rootCases {
		if detail := testCase.check(roots); detail != "" {
			results = append(results, leroot.Fail(testCase.name, detail))
			continue
		}
		results = append(results, leroot.Pass(testCase.name))
	}

	return leroot.NewSelftestReport(
		"plugin-process-boundary selftest OK",
		"plugin-process-boundary selftest FAILED:",
		results...,
	), nil
}

// runSelftest is the `le plugin boundary selftest` action.
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
