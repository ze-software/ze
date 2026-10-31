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
//
// FixtureProof is the second half: what a walked, proven stem holds on disk,
// so a test in another package can build one without spelling a sha.

package rfc

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

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
	"| Support | " + FixtureAggregate + " |\n| Support area | Widgets |\n" +
	"| Use cases | - |\n| Optional use cases | - |\n\n" +
	"## Compliance Checklist\n\n" +
	"- [ ] [" + FixtureRequirement + "] [MUST] A speaker MUST send the widget (§2)\n"

// FixtureAggregate is the technology aggregate the fixture summary names, and
// FixtureAggregateProfile is its rfc/profiles/ file: a summary naming an
// aggregate no profile declares does not parse, so the corpus carries both.
const FixtureAggregate = "bgp-core"

const FixtureAggregateProfile = "# Profile: " + FixtureAggregate + "\n\n| Field | Value |\n|-------|-------|\n" +
	"| Kind | technology |\n| Title | BGP core |\n| Heading | BGP core |\n| Intro | - |\n" +
	"| Shape | wide |\n| Release | n/a |\n"

// FixtureAggregateRel is where FixtureAggregateProfile lives in a fixture tree.
const FixtureAggregateRel = profileRel + "/" + FixtureAggregate + ".md"

// fixtureEvidenceBacklog is the exclusion evidence count file of a fixture
// whose rows carry every exclusion's evidence: zero for each kind, so a row
// lacking it is refused by id (checkExclusionEvidence).
const fixtureEvidenceBacklog = "lower-layer 0\nnot-applicable 0\n"

// FixtureFiles answers every file the three drivers write in common, keyed by
// its path relative to the checkout root: the summary, the RFC text the
// extraction walk reads, the drain budget the quota reads, the evidence counts
// the exclusion ratchet reads, and the workflow the
// carrier walk looks for.
//
// A caller adds what its own tree needs beside these -- a `feature-gates.txt`
// says something different to each of them -- and a fresh map is returned so
// adding to it cannot reach another caller.
func FixtureFiles() map[string]string {
	return map[string]string{
		"rfc/short/" + FixtureStem + ".md": fixtureSummary,
		FixtureAggregateRel:                FixtureAggregateProfile,
		"rfc/full/" + FixtureStem + ".txt": "A speaker MUST send the widget.\n",
		"rfc/drain-budget.txt":             "start 2026-07-29\nrate 0\n",
		evidenceBacklogRel:                 fixtureEvidenceBacklog,
		".github/workflows/nightly.yml":    "on:\n  schedule:\n    - cron: '0 3 * * *'\n",
	}
}

// FixtureProof gives stem, in the synthetic checkout at root, what a walked and
// proven stem holds: an extraction sign-off and one discrimination record per
// element of records, each written where the tree keeps it.
//
// mapped names, by inventory site id ("2:1" is the first normative site of
// section 2 of the stem's RFC text), the requirement each site maps to; every
// other site is excluded as not a requirement. Each record is sealed against
// root as it stands, by the production minter, so the caller writes the
// summary, the RFC text, the tagged units and the producers first.
//
// A sign-off the evaluator refuses, or a record that does not verify against
// root, is an error naming why. A fixture that quietly counts nothing would let
// the test that reads it pass for the wrong reason.
//
// Any stem but FixtureStem is refused before anything is written. The records
// are sealed by the production minter without a red ever being observed, which
// is honest only for the synthetic RFC no real registry holds: aimed at a real
// stem, this would forge the proof `./le rfc record` exists to earn.
func FixtureProof(root, stem string, mapped map[string]string, records ...DiscriminationRecord) error {
	if stem != FixtureStem {
		var tb textbuf.Buffer
		return errors.New(tb.Str("FixtureProof refuses stem ").Str(stem).
			Str(": it seals discrimination records without observing a red, so it may write them only for the synthetic ").
			Str(FixtureStem).Str(". Build the fixture summary as ").Str(FixtureStem).
			Str(", or record a real stem's proof with `./le rfc record`").String())
	}
	rel := summaryRelOf(stem)
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- path is inside the caller's fixture checkout
	if err != nil {
		return err
	}
	requirements, err := ParseSummaryText(string(body), stem, rel)
	if err != nil {
		return err
	}
	if err := fixtureSignOff(root, stem, mapped, requirements); err != nil {
		return err
	}
	return fixtureRecords(root, stem, records)
}

// fixtureSignOff writes stem's sign-off and refuses one the evaluator does not
// accept, with the evaluator's own violations.
func fixtureSignOff(root, stem string, mapped map[string]string, requirements []Requirement) error {
	deriver := NewDeriver(root)
	inventory, err := deriver.Inventory(stem, gatedCounts(requirements)[stem])
	if err != nil {
		return err
	}
	if inventory == nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str(stem).
			Str(" has no RFC text in the fixture, so there is nothing to sign off").String())
	}
	artifact, err := marshalSelftestJSON(signOffArtifact(inventory, mapped))
	if err != nil {
		return err
	}
	if err := writeSelftestFiles(root, map[string]string{extractionRel + "/" + stem + jsonSuffix: artifact}); err != nil {
		return err
	}
	signed, violations, err := evaluateExtractions(deriver, requirements)
	if err != nil {
		return err
	}
	if _, held := signed[stem]; held {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("the fixture sign-off for ").Str(stem).Str(" was refused")
	for _, violation := range violations {
		tb.Str("; ").Str(violation)
	}
	return errors.New(tb.String())
}

// fixtureRecords seals records against root, writes them, and refuses any the
// tree does not then verify.
func fixtureRecords(root, stem string, unsealed []DiscriminationRecord) error {
	covers, err := tagCoversIn(root)
	if err != nil {
		return err
	}
	records, err := sealRecords(root, covers, unsealed)
	if err != nil {
		return err
	}
	files, err := discriminationFiles(stem, records)
	if err != nil {
		return err
	}
	if err := writeSelftestFiles(root, files); err != nil {
		return err
	}
	verdicts, err := verifyDiscrimination(root, records, covers)
	if err != nil {
		return err
	}
	for position := range verdicts {
		verdict := &verdicts[position]
		if verdict.Verified() {
			continue
		}
		var tb textbuf.Buffer
		return errors.New(tb.Str("the fixture record for ").Str(verdict.Record.RID).Byte(' ').
			Str(verdict.Record.Polarity).Str(" does not verify: ").Str(verdict.State).
			Byte(' ').Str(verdict.Detail).String())
	}
	return nil
}
