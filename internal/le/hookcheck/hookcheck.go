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

	"github.com/ze-software/ze/internal/core/textbuf"
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

// GoldenCase is one typed dispatcher fixture in authoritative order.
// NativeCode is the translated Go rule's verdict.
type GoldenCase struct {
	Table        string `json:"table"`
	Name         string `json:"name"`
	ExpectedCode int    `json:"expected-code"`
	NativeCode   int    `json:"native-code"`
	Passed       bool   `json:"passed"`
}

// MessageExpectation is one exact output contract and its comparison mode.
type MessageExpectation struct {
	Match string `json:"match"`
	Text  string `json:"text"`
}

// FixtureCase is one typed behavioral assertion. ExpectedExit is -1 when the
// assertion has no process exit contract.
type FixtureCase struct {
	Category     string               `json:"category"`
	Name         string               `json:"name"`
	Producer     string               `json:"producer"`
	ExpectedExit int                  `json:"expected-exit"`
	Messages     []MessageExpectation `json:"messages,omitempty"`
	Site         int                  `json:"site"`
	Variant      int                  `json:"variant"`
}

// Population records the typed rows consumed by the native selftest.
type Population struct {
	Bash            int `json:"bash"`
	WriteEdit       int `json:"write-edit"`
	Weakening       int `json:"weakening"`
	PostWriteEdit   int `json:"post-write-edit"`
	FixtureSites    int `json:"fixture-sites"`
	FixtureChecks   int `json:"fixture-checks"`
	FixtureSections int `json:"fixture-sections"`
}

// Report is the structured answer of ze-unit-hook-test. Results retain catalog
// order, and Code is the first failing child code rather than a flattened one.
type Report struct {
	Population Population    `json:"population"`
	Golden     []GoldenCase  `json:"golden"`
	Fixtures   []FixtureCase `json:"fixtures"`
	Results    []Result      `json:"results"`
	Code       int           `json:"code"`
}

// Text renders the selftest summary while keeping the report available to data
// pipe operators.
//
// Every failing result is named under the summary, because this string is the
// only thing the sweep prints: a count with no cause leaves the operator with
// nothing to act on (plan/journal/failing-gate-prints-no-cause.md).
func (r Report) Text() string {
	passed := 0
	for _, result := range r.Results {
		if result.Passed {
			passed++
		}
	}
	var tb textbuf.Buffer
	tb.Str("hook native selftest: ").Int(int64(passed)).Byte('/').
		Int(int64(len(r.Results))).Str(" passed\n")
	for _, result := range r.Results {
		if result.Passed {
			continue
		}
		tb.Str("  FAIL ").Str(result.Name).Str(" (exit ").Int(int64(result.Code)).Str("): ")
		if result.Message == "" {
			// A failing result with no message is a defect in the check that
			// produced it, so say that rather than print an empty reason.
			tb.Str("no cause recorded by the check\n")
			continue
		}
		tb.Str(result.Message).Byte('\n')
	}
	return tb.String()
}

// Run evaluates all typed dispatcher rows and behavioral fixture categories.
// Missing hook files, populations, categories, and outputs fail closed.
func Run(root string) (Report, int) {
	if root == "" {
		result := Result{
			Name: "hook-check-root", Code: 2, Message: "hook check checkout root is empty",
		}
		return Report{Results: []Result{result}, Code: result.Code}, result.Code
	}
	report := Report{
		Results: make([]Result, 0, len(parityCatalog)+len(fixtureCategories)+2),
	}
	parityResults, population, golden := runParity(root)
	report.Population = population
	report.Golden = golden
	report.Results = append(report.Results, parityResults...)

	fixtureResults, fixtureChecks := runFixtures(root)
	report.Population.FixtureSites = len(fixtureSites)
	report.Population.FixtureChecks = fixtureChecks
	report.Population.FixtureSections = len(fixtureCategories)
	report.Fixtures = fixtureCases()
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

func fixtureCases() []FixtureCase {
	cases := make([]FixtureCase, 0, len(fixtureCatalog))
	for _, fixture := range fixtureCatalog {
		row := FixtureCase{
			Category:     fixture.category,
			Name:         fixture.name,
			Producer:     fixtureProducerName(fixture.category),
			ExpectedExit: fixture.expectedExit,
			Messages:     make([]MessageExpectation, 0, len(fixture.messages)),
			Site:         fixture.site,
			Variant:      fixture.variant,
		}
		for _, message := range fixture.messages {
			row.Messages = append(row.Messages, MessageExpectation{
				Match: message.match,
				Text:  message.text,
			})
		}
		cases = append(cases, row)
	}
	return cases
}

func fixtureProducerName(categoryName string) string {
	for _, category := range &fixtureCategories {
		if category.name == categoryName {
			return category.runner
		}
	}
	return ""
}
