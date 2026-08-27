// Design: docs/architecture/core-design.md -- native Go gates run through le
// Detail: parity.go -- dispatcher golden evaluation without Python
// Detail: fixtures.go -- behavioral fixture category census and native probes
// Detail: actions.go -- the gateless unit verb and checkout-bound invocation
//
// Package hookcheck runs the hook dispatcher and behavioral selftests in-process.
package hookcheck

import (
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	parityProducer  = "scripts/dev/hook-parity-check.py"
	fixtureProducer = "scripts/dev/hook-fixture-check.py"
)

func readCheckoutFile(root, relative string) ([]byte, error) {
	checkout, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	body, readErr := checkout.ReadFile(filepath.FromSlash(relative))
	return body, errors.Join(readErr, checkout.Close())
}

// Result is one independently actionable selftest verdict.
type Result struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// GoldenCase is one producer row in original order. ExpectedCode comes from the
// embedded Python golden; NativeCode comes from the translated Go rule.
type GoldenCase struct {
	Table        string `json:"table"`
	Name         string `json:"name"`
	ExpectedCode int    `json:"expected-code"`
	NativeCode   int    `json:"native-code"`
	Passed       bool   `json:"passed"`
}

// Population records the producer rows consumed by the native selftest.
type Population struct {
	Bash            int `json:"bash"`
	WriteEdit       int `json:"write-edit"`
	Weakening       int `json:"weakening"`
	PostWriteEdit   int `json:"post-write-edit"`
	FixtureChecks   int `json:"fixture-checks"`
	FixtureSections int `json:"fixture-sections"`
}

// Report is the structured answer of ze-unit-hook-test. Results retain producer
// order, and Code is the first failing child code rather than a flattened one.
type Report struct {
	Population Population   `json:"population"`
	Golden     []GoldenCase `json:"golden"`
	Results    []Result     `json:"results"`
	Code       int          `json:"code"`
}

// Text renders the selftest summary while keeping the report available to data
// pipe operators.
func (r Report) Text() string {
	passed := 0
	for _, result := range r.Results {
		if result.Passed {
			passed++
		}
	}
	var tb textbuf.Buffer
	return tb.Str("hook native selftest: ").Int(int64(passed)).Byte('/').
		Int(int64(len(r.Results))).Str(" passed\n").String()
}

// Run evaluates all golden rows and all behavioral fixture categories. Missing
// files, populations, categories, and outputs are failures rather than skips.
func Run(root string) (Report, int) {
	if root == "" {
		result := Result{
			Name: "hook-check-root", Code: 2, Message: "hook check checkout root is empty",
		}
		return Report{Results: []Result{result}, Code: result.Code}, result.Code
	}
	report := Report{Results: make([]Result, 0, 4+len(fixtureCategories))}
	parityResults, population, golden := runParity(root)
	report.Population = population
	report.Golden = golden
	report.Results = append(report.Results, parityResults...)

	fixtureResults, fixtureChecks := runFixtures(root)
	report.Population.FixtureChecks = fixtureChecks
	report.Population.FixtureSections = len(fixtureCategories)
	report.Results = append(report.Results, fixtureResults...)

	for _, result := range report.Results {
		if result.Passed {
			continue
		}
		if report.Code == 0 {
			report.Code = result.Code
		}
	}
	return report, report.Code
}

// CategoryNames returns the complete fixture category population in producer
// order. A caller receives a copy and cannot alter the selftest table.
func CategoryNames() []string {
	names := make([]string, 0, len(fixtureCategories))
	for _, category := range &fixtureCategories {
		names = append(names, category.name)
	}
	return slices.Clone(names)
}
