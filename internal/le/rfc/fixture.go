// Design: docs/architecture/testing/verify-freshness-scope.md -- the synthetic checkout the RFC gate is exercised over
// Related: meta.go -- ParseMeta, which decides what a summary MUST declare
//
// The smallest checkout `./le rfc` can judge, declared ONCE.
//
// Three packages drive the gate over a synthetic tree and none can import
// another: `internal/le/doc/wiring` checks the freshness stage,
// `internal/le/hookruntime` checks what session-start renders, and
// `internal/test/fixture` drives the whole loop from a `.ci`. Each spelled this
// corpus itself, and one of them says so in its own comment.
//
// The cost of that came due on 2026-09-21. `Implementation` became a required
// Meta row, every copy stopped parsing, and three packages went red on one
// sentence with the compiler naming none of them. The summary is what moves
// when the grammar moves, so the summary has one declaration and the callers
// add whatever else their own tree needs.
//
// It lives in the production package rather than a test file because one of the
// three callers is not a test: the `.ci` driver
// (internal/test/fixture/misc_fixture_runner_rfcledger.go) builds the tree at
// run time.

package rfc

// FixtureStem is the RFC the synthetic corpus declares. It is outside every
// real registry on purpose, so a fixture tree can never be confused with the
// enrolled population.
const FixtureStem = "rfc9999"

// FixtureRequirement is the one MUST that summary gates. Every step of the loop
// names it: the gate reports it untested, the shard carries a row for it, and a
// tagged test binds it.
const FixtureRequirement = "RFC9999-2-1"

// fixtureSummary is the smallest `rfc/short/` summary ParseMeta accepts and the
// gate can judge: one enrolled document, implemented in Go, gating one MUST.
//
// Every row here is one ParseMeta REFUSES a summary without, so a row that
// looks like decoration is the parser's own requirement. `Implementation` is
// the newest of them (owner directive, 2026-09-21).
//
// Unexported because every caller wants the corpus rather than this string, and
// an exported name no other package reads is one `./le doc wiring` refuses.
const fixtureSummary = "# RFC 9999\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n" +
	"| Title | Widgets |\n| Enrolment | enrolled |\n" +
	"| Enrolment reason | the fixture RFC, gated so the gate has a population |\n" +
	"| Implementation | ze |\n" +
	"| Implementation reason | the fixture RFC's own Go answers it (internal/widget) |\n" +
	"| Support | bgp-base 10 |\n| Support area | Widgets |\n" +
	"| Support status | Partial |\n| Support coverage | unit tests |\n" +
	"| Support remaining | Zero MUST gaps. |\n\n" +
	"## Compliance Checklist\n\n" +
	"- [ ] [" + FixtureRequirement + "] [MUST] A speaker MUST send the widget (§2)\n"

// FixtureFiles answers every file the three drivers write in common, keyed by
// its path relative to the checkout root: the summary, the RFC text the
// extraction walk reads, the drain budget the quota reads, and the workflow the
// carrier walk looks for.
//
// A caller adds what its own tree needs beside these -- a `feature-gates.txt`
// says something different to each of them -- and a fresh map is returned so
// adding to it cannot reach another caller.
func FixtureFiles() map[string]string {
	return map[string]string{
		"rfc/short/" + FixtureStem + ".md": fixtureSummary,
		"rfc/full/" + FixtureStem + ".txt": "A speaker MUST send the widget.\n",
		"rfc/drain-budget.txt":             "start 2026-07-29\nrate 0\n",
		".github/workflows/nightly.yml":    "on:\n  schedule:\n    - cron: '0 3 * * *'\n",
	}
}
