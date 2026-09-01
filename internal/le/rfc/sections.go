// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Overview: render.go -- the two pages this file's sections are assembled into
//
// sections.go holds the four body sections of the generated index: the coverage
// rollup, the audit coverage, the extraction sign-off table and the two status
// backlogs. Each one is a formatting of numbers coverage.go produced, and none
// of them reads a file of its own.
package rfc

import (
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// renderRollup is the actionable view: what is owed, per RFC, nearest to
// enrollable first.
//
// Without it the backlog exists but is unreadable -- thousands of rows with a
// dash in them is an inventory, not a worklist.
func renderRollup(in RenderInput) []string {
	cov := CoverageRows(in.Requirements, in.Tags, in.Carriers)
	// Enrolled first (regressions matter most), then closest to enrollable, so
	// the next RFC worth finishing is always at the top.
	sort.SliceStable(cov, func(i, j int) bool {
		a, b := cov[i], cov[j]
		if in.Enrolled[a.RFC] != in.Enrolled[b.RFC] {
			return in.Enrolled[a.RFC]
		}
		if a.Outstanding() != b.Outstanding() {
			return a.Outstanding() < b.Outstanding()
		}
		return a.RFC < b.RFC
	})

	totalOut, nightlyTotal := 0, 0
	var ready []string
	for _, c := range cov {
		totalOut += c.Outstanding()
		nightlyTotal += c.NightlyOnly
		if c.Outstanding() == 0 && !in.Enrolled[c.RFC] {
			ready = append(ready, c.RFC)
		}
	}

	var head textbuf.Buffer
	out := []string{
		"## Coverage by RFC",
		"",
		head.Int(int64(totalOut)).
			Str(" MUST-level requirement(s) still owe work across ").Int(int64(len(cov))).
			Str(" summaries. **Outstanding** = has only one polarity, or has no test and ").
			Str("no annotation; those are the tests that do not exist yet.").String(),
		"",
	}
	if len(ready) > 0 {
		shown := ready
		ellipsis := ""
		if len(shown) > 12 {
			shown, ellipsis = shown[:12], "..."
		}
		var tb textbuf.Buffer
		out = append(out, tb.Str("**Enrollable now** (").Int(int64(len(ready))).
			Str("): every MUST-level requirement is already covered or annotated, so ").
			Str("declaring `| Enrolment | enrolled |` on these would gate them without any new work: ").
			Str(joinBackticked(shown)).Str(ellipsis).String(), "")
	}
	var nightly textbuf.Buffer
	out = append(out, nightly.Str("**Nightly-only** (").Int(int64(nightlyTotal)).
		Str(" requirement(s)) counts what is proven ONLY by evidence no ").
		Str("`./le verify current mode full` stage runs -- today, interop scenarios, which are ").
		Str("scheduled and advisory. **Both** and **One polarity** are the polarity ").
		Str("view: they answer which polarities exist, not which pipeline runs them, ").
		Str("so a nightly-only requirement is counted there too. **Nightly-only** is ").
		Str("the tier view over the same rows -- an overlapping subset marker naming ").
		Str("which of them no merge-gate stage proves, never a total to sum with the ").
		Str("others.").String(), "")

	out = append(out, renderSupersededNote(in, cov)...)
	out = append(out,
		"| RFC | Gated | Both | One polarity | Annotated | No test | Outstanding | "+
			"Nightly-only | State |",
		"|---|---|---|---|---|---|---|---|---|")
	for _, c := range cov {
		state := "backlog"
		switch {
		case in.Enrolled[c.RFC]:
			state = "**enrolled**"
		case c.Outstanding() == 0:
			state = "enrollable"
		}
		if successor, held := in.Successors[c.RFC]; held {
			var tb textbuf.Buffer
			state = tb.Str(state).Str(", superseded by ").Str(Prefix(successor)).String()
		}
		var row textbuf.Buffer
		out = append(out, row.Str("| `").Str(c.RFC).Str("` | ").Int(int64(c.Gated)).
			Str(" | ").Int(int64(c.Both)).Str(" | ").Int(int64(c.One)).
			Str(" | ").Int(int64(c.Annotated)).Str(" | ").Int(int64(c.Missing)).
			Str(" | ").Int(int64(c.Outstanding())).Str(" | ").Int(int64(c.NightlyOnly)).
			Str(" | ").Str(state).Str(" |").String())
	}
	out = append(out, "")
	return out
}

// renderSupersededNote names which documents the IETF has replaced, derived
// from each summary's forward Meta row rather than kept in a list.
//
// The two debt dispositions are named as debt: one says the successor's text is
// not here, the other says its summary declares no row, and neither is a
// checked pointer. None of this lowers what any of these requirements owes -- a
// superseded MUST is gated, counted and ratcheted exactly as a current one is.
func renderSupersededNote(in RenderInput, cov []CoverageRow) []string {
	var superseded []string
	for _, c := range cov {
		if _, held := in.Successors[c.RFC]; held {
			superseded = append(superseded, c.RFC)
		}
	}
	if len(superseded) == 0 {
		return nil
	}
	sort.Strings(superseded)

	unextracted, unresolved := 0, 0
	for _, req := range in.Requirements {
		if req.Superseded == nil {
			continue
		}
		switch req.Superseded.Disposition {
		case successorUnextracted:
			unextracted++
		case successorUnresolved:
			unresolved++
		}
	}
	var pointers textbuf.Buffer
	for i, stem := range superseded {
		if i > 0 {
			pointers.Str(", ")
		}
		pointers.Byte('`').Str(stem).Str("` -> ").Str(Prefix(in.Successors[stem]))
	}
	var tb textbuf.Buffer
	return []string{tb.Str("**Superseded** (").Int(int64(len(superseded))).
		Str(" summaries): the document has been obsoleted, so every requirement it ").
		Str("states carries a `{superseded}` marker naming where that obligation now ").
		Str("lives. The marker is a fact about the DOCUMENT: these requirements stay ").
		Str("gated, counted and ratcheted. ").Str(pointers.String()).Str(". ").
		Int(int64(unextracted)).
		Str(" requirement(s) point at a section of a successor whose summary declares ").
		Str("no row (`unextracted`), and ").Int(int64(unresolved)).
		Str(" point at a document this repository does not hold (`unresolved`). Both ").
		Str("are debt, not settled pointers.").String(), ""}
}

// renderAuditCoverage is the semantic half of the gate's coverage, published
// rather than gated.
//
// Without it the audit exists and is invisible: a low-tens count of auditable
// requirements carries a verdict, all of them on a handful of RFCs, and nothing
// says so anywhere a reader would meet it. Publishing per-RFC is also what makes
// SAMPLING possible -- the only real check on whether a verdict was written by
// someone who read something -- which no gate can perform.
//
// The COLUMN COUNT here is load-bearing. internal/le/testhealth/actions.go pins the
// polarity rollup with a nine-cell regex and matches it against every line of
// this file, so a table whose rows had the same shape would be silently folded
// into that tool's proof-density figure.
func renderAuditCoverage(in RenderInput) []string {
	rows, worklist := auditCoverageRows(auditCoverageInput{
		Requirements: in.Requirements, Tags: in.Tags, Enrolled: in.Enrolled,
		Carriers: in.Carriers, Audits: in.Audits, States: in.States,
	})
	auditable, audited, proven, findings, verdicts, withVerdict := 0, 0, 0, 0, 0, 0
	for _, r := range rows {
		auditable += r.Auditable
		audited += r.Audited
		proven += r.Proven
		findings += r.Findings
		verdicts += r.Verdicts
		if r.Audited > 0 {
			withVerdict++
		}
	}
	pct := 0.0
	if auditable > 0 {
		pct = 100.0 * float64(audited) / float64(auditable)
	}

	var lead, provenNote, remainder, partitions textbuf.Buffer
	out := []string{
		"## Audit coverage",
		"",
		lead.Int(int64(audited)).Str(" of ").Int(int64(auditable)).
			Str(" auditable requirement(s) carry a `ze-rfc-audit` verdict (").Float2(pct).
			Str("%), across ").Int(int64(withVerdict)).Str(" of ").Int(int64(len(rows))).
			Str(" enrolled RFC(s). **Auditable** = gated, enrolled, and polarity ").
			Str("coverage complete: a pair of tests, or one test over a ").
			Str("`{single-polarity}` line saying why the other cannot exist. Until then ").
			Str("there is nothing for an auditor to judge.").String(),
		"",
		provenNote.Str("**Proven** (").Int(int64(proven)).
			Str(") is the count that means what the badge implies: a verdict of `").
			Str(VerdictEnforced).
			Str("` -- the tests would fail if the code stopped complying -- that is still ").
			Str("fresh. It is NOT the **Both** column of the rollup above: that one ").
			Str("answers which polarities exist, and a requirement can have both and ").
			Str("still be judged `").Str(VerdictWeak).Str("`. Every one of the ").
			Int(int64(findings)).
			Str(" verdict(s) that is audited but not proven is named below with its ").
			Str("verdict, so no requirement can read as proven and weak at once.").String(),
		"",
		remainder.Str("The remaining ").Int(int64(auditable - audited)).
			Str(" carry no verdict at all. That is not a violation: the audit is sampled ").
			Str("and the gate is total, so a missing verdict never fails ").
			Str("`./le rfc check`. It is published because an unmeasured semantic half ").
			Str("is indistinguishable from a clean one.").String(),
		"",
		partitions.Str("Two partitions over two populations, because one denominator ").
			Str("cannot carry both questions. **Requirements:** `Auditable` (").
			Int(int64(auditable)).Str(") = `Audited` (").Int(int64(audited)).
			Str(") + `Unaudited` (").Int(int64(auditable - audited)).Str("). **Records:** all ").
			Int(int64(verdicts)).Str(" recorded verdict(s) = `Proven` (").Int(int64(proven)).
			Str(") + `Not proven` (").Int(int64(findings)).
			Str("), and the worklist below names every one of those ").
			Int(int64(len(worklist))).
			Str(". A verdict can sit on a requirement that is not auditable -- an ").
			Str("annotated `{gap}` or `{not-applicable}` line carries no tagged test -- so ").
			Str("the record totals are the wider of the two and are never a subset of ").
			Str("`Audited`.").String(),
		"",
		"| RFC | Auditable | Audited | Proven | Not proven | Unaudited |",
		"|---|---|---|---|---|---|",
	}
	ordered := append([]auditCoverage(nil), rows...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Audited != ordered[j].Audited {
			return ordered[i].Audited > ordered[j].Audited
		}
		return ordered[i].RFC < ordered[j].RFC
	})
	for _, r := range ordered {
		if r.Auditable == 0 && r.Audited == 0 {
			continue
		}
		var row textbuf.Buffer
		out = append(out, row.Str("| `").Str(r.RFC).Str("` | ").Int(int64(r.Auditable)).
			Str(" | ").Int(int64(r.Audited)).Str(" | ").Int(int64(r.Proven)).
			Str(" | ").Int(int64(r.Findings)).Str(" | ").Int(int64(r.unaudited())).
			Str(" |").String())
	}
	out = append(out, "", "### Audited but not proven", "")
	if len(worklist) == 0 {
		out = append(out, "None: every recorded verdict is a fresh `enforced`. On a corpus "+
			"this size that is worth reading as a warning as much as a result -- the "+
			"`ze-rfc-audit` skill says `weak` and `wrong` are its valuable outputs, and a "+
			"run that returns all `enforced` has probably not read anything.")
	} else {
		var intro textbuf.Buffer
		out = append(out, intro.
			Str("One row per requirement whose verdict is anything other than a fresh `").
			Str(VerdictEnforced).
			Str("`. A blur is not a worklist: each is named so it can be picked up ").
			Str("individually.").String(), "",
			"| Requirement | Verdict | Meaning |", "|---|---|---|")
		for _, row := range worklist {
			var cell textbuf.Buffer
			out = append(out, cell.Str("| `").Str(row.RID).Str("` | `").Str(row.Reason).
				Str("` | ").Str(verdictMeaning(row.Reason)).Str(" |").String())
		}
	}
	out = append(out, "")
	return out
}

// unprovenMeaning is one line per verdict word.
var unprovenMeaning = map[string]string{
	VerdictWeak:          "tagged and green, but cannot fail on non-compliance",
	VerdictWrong:         "the test asserts something the RFC does not say",
	VerdictUnimplemented: "the tests are fine; the CODE does not comply",
	VerdictNotApplicable: "no reachable code path could satisfy or violate it",
}

// stateMeaning is one line per freshness state.
//
// A NON-fresh row's meaning is its STATE, not its verdict word: the judgement
// itself may be perfectly good with only its fingerprints out of date. One line
// per state, because rendering them all as "the verdict no longer describes what
// it judged" told a `shifted` reader the exact opposite of the truth -- shifted
// means the tagged unit IS byte-identical -- and sent them to re-read an RFC
// when one mechanical command clears it.
var stateMeaning = map[string]string{
	ShiftedState: "the tagged unit is byte-identical and only the file around it moved; " +
		"nothing was re-judged, so re-stamp it with `./le rfc reseal`",
	StaleUnitState: "what it judged changed -- the tagged unit itself, or the producing " +
		"code it cites; it must be re-judged with the `ze-rfc-audit` skill before it " +
		"counts as anything",
	StaleRequirementState: "the requirement's own text changed since it was judged, so " +
		"every judgement under it is void; re-read it with the `ze-rfc-audit` skill",
}

// verdictMeaning explains one worklist row to a reader who has not read the
// skill. Derived from the reason string the coverage walk produced, never a
// second table.
//
// It fails closed on a vocabulary that grows: a verdict added to the unproven
// set, or a state added to the four, without a published meaning SAYS so in the
// ledger rather than rendering an empty cell or a wrong one, because an
// unexplained verdict in a worklist is a row a reader silently skips.
func verdictMeaning(reason string) string {
	value, rest, _ := strings.Cut(reason, " ")
	if rest != "" {
		state := strings.Trim(strings.TrimSpace(rest), "()")
		if meaning, held := stateMeaning[state]; held {
			return meaning
		}
		var tb textbuf.Buffer
		return tb.Str("recorded `").Str(value).
			Str("` in the unpublished freshness state `").Str(state).
			Str("` -- add it to _STATE_MEANING").String()
	}
	if _, known := auditVerdicts[value]; value == VerdictEnforced || !known {
		return "outside the recorded vocabulary"
	}
	if meaning, held := unprovenMeaning[value]; held {
		return meaning
	}
	return "no published meaning for this verdict -- add one to _UNPROVEN_MEANING"
}

// renderDiscrimination is the published state of the claim half of a tag.
//
// Every other table on this page judges the STRUCTURED half of a tag: the id
// resolves, the polarity is declared, the carrier runs. None of them can read
// the sentence after it, so a tag advertising an assertion its body never makes
// counts here as evidence. A discrimination record is what replaces reading
// that sentence, and this section publishes how much of the corpus carries one.
//
// Prose, no table: internal/le/testhealth/collect_rfc.go matches a nine-cell
// regex against every line of this file, so a table of the same width would be
// folded into the proof-density figure it reports.
func renderDiscrimination(in RenderInput) []string {
	proven, escaped := discriminationRouteCounts(in.Discrimination)
	byRoute := map[string]int{}
	byReason := map[string]int{}
	covered := make(map[Cover]bool, len(in.Discrimination))
	removable := 0
	for index := range in.Discrimination {
		verdict := &in.Discrimination[index]
		if !verdict.Verified() {
			if verdict.removable() {
				removable++
			}
			continue
		}
		covered[verdict.Record.Cover()] = true
		byRoute[verdict.Record.Route]++
		byReason[verdict.Record.Reason]++
	}

	gated := map[string]bool{}
	for _, req := range in.Requirements {
		if req.Gated() && in.Enrolled[req.RFC] {
			gated[req.RID] = true
		}
	}
	scope, backlog := 0, 0
	for key := range in.Covers {
		if !gated[key.RID] {
			continue
		}
		scope++
		if !covered[key] {
			backlog++
		}
	}
	// Two populations, and the record total is the wider one. A record can sit
	// on a requirement this gate does not oblige -- an un-enrolled RFC, or a
	// SHOULD -- and it is real evidence there. Publishing the two counts side
	// by side without this figure renders an arithmetic error to a reader who
	// subtracts one from the other.
	outside := proven + escaped - (scope - backlog)

	var lead, counts, debt, escape textbuf.Buffer
	out := []string{
		"## Claim discrimination",
		"",
		lead.Str("A tag names a requirement and a polarity, and then states in prose what ").
			Str("its test demonstrates. No gate can read that sentence, so a test that ").
			Str("asserts less than its tag claims counts as evidence everywhere else on ").
			Str("this page. A record under `rfc/discrimination/` replaces reading it: it ").
			Str("names a break of the producing code, and it stores the observation that ").
			Str("the tagged unit went RED under that break and green again after it. ").
			Str("`./le rfc check` replays the fingerprints on every run and refuses a ").
			Str("record whose unit, claim or producer has moved since.").String(),
		"",
		counts.Str("Proven: ").Int(int64(proven)).Byte(' ').Str(routePhrase(byRoute)).
			Str(". Escaped: ").
			Int(int64(escaped)).Str(". Unproven backlog: ").Int(int64(backlog)).
			Str(" of ").Int(int64(scope)).
			Str(" tagged unit(s) on a gated requirement of an enrolled RFC.").
			Str(outsidePhrase(outside)).String(),
		"",
		debt.Str("The backlog is grandfathered, as the extraction backlog is. The ").
			Str("obligation is CHANGE-SCOPED: a tagged unit that is new against git HEAD ").
			Str("owes its proof in the change that added it, and `./le rfc check` reports ").
			Str("that figure as `owed`. It is absent from this page on purpose. `owed` is ").
			Str("a fact about a commit boundary rather than about this tree, so a page ").
			Str("carrying it would go stale when nothing in the tree had changed.").String(),
		"",
		escape.Str("An escape (`").Str(RouteNoBreak).
			Str("`) is a recorded claim that no break exists, and the gate checks a ").
			Str("precondition for each reason it accepts: ").Str(reasonPhrase(byReason)).
			Str(". It is counted apart from a proof because a claim that nothing can ").
			Str("break is debt, not evidence.").String(),
		"",
	}
	if removable > 0 {
		var tb textbuf.Buffer
		out = append(out, tb.Int(int64(removable)).
			Str(" record(s) can be deleted: the tag each one proved is gone. A record ").
			Str("dies with its tag, so an orphan is removed rather than re-recorded.").
			String(), "")
	}
	return append(out, renderUnscanned(in.Unscanned)...)
}

// outsidePhrase says how many records sit outside the obliged population.
//
// DERIVED, so the sentence appears exactly when the fact does. Silent at zero,
// because a reader who meets it there learns nothing and still pays for it.
func outsidePhrase(outside int) string {
	if outside <= 0 {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Byte(' ').Int(int64(outside)).
		Str(" further record(s) sit outside that population, on a requirement no gate ").
		Str("obliges: an un-enrolled RFC, or a level below MUST. They are counted in ").
		Str("the totals above and not in the backlog, so the two figures are two ").
		Str("populations rather than one arithmetic.").String()
}

// routePhrase names every proof route, including the zeros.
//
// The escape route is left out: it is counted in its own sentence, because a
// reader who meets three numbers in one list reads their sum as the proven
// count (R-9).
func routePhrase(counts map[string]int) string {
	var tb textbuf.Buffer
	tb.Byte('(')
	first := true
	for _, name := range DiscriminationRoutes() {
		if name == RouteNoBreak {
			continue
		}
		if !first {
			tb.Str(", ")
		}
		first = false
		tb.Str(name).Byte(' ').Int(int64(counts[name]))
	}
	return tb.Byte(')').String()
}

// reasonPhrase names every escape reason, including the zeros.
func reasonPhrase(counts map[string]int) string {
	var tb textbuf.Buffer
	for index, name := range escapeReasonNames() {
		if index > 0 {
			tb.Str(", ")
		}
		tb.Byte('`').Str(name).Str("` ").Int(int64(counts[name]))
	}
	return tb.String()
}

// renderUnscanned publishes the `RFC requirement:` comments that sit in
// production Go on no carrier.
//
// Reported rather than refused, in the shape the extraction backlog is
// reported. They predate the check, and a ratchet that reds the tree over
// standing debt gets removed rather than obeyed. Published because a comment
// that reads as evidence to a person and is counted by no gate is the exact
// failure this section exists to make visible.
func renderUnscanned(tags []UnscannedTag) []string {
	if len(tags) == 0 {
		return nil
	}
	var tb textbuf.Buffer
	return []string{tb.Int(int64(len(tags))).
		Str(" `RFC requirement:` comment(s) sit in production Go that no carrier claims. ").
		Str("Nothing resolves them and nothing runs them, and ").
		Int(int64(unscannedRefused(tags))).
		Str(" of them carry no polarity, so no scanner would accept them even if a ").
		Str("carrier did claim the file. `./le rfc check` names each one.").String(), ""}
}

// renderExtractionTable is the published backlog: how much of the standards
// claim is BOUNDED.
//
// Derived columns are shown only for a stem that HAS a sign-off. That is both
// honest and affordable. Honest, because a register derived for a stem nobody
// has walked is not a fact this repository has established. Affordable, because
// deriving the inventory for every enrolled RFC costs seconds on top of the
// gate, on EVERY run, and a gate that doubles verify time is a gate people learn
// to skip.
func renderExtractionTable(in RenderInput) ([]string, error) {
	valid, _, err := evaluateExtractions(in.Deriver, in.Requirements)
	if err != nil {
		return nil, err
	}
	signed := credited(valid, in.Enrolled)
	counts := registerCounts(signed)
	gated := gatedCounts(in.Requirements)
	var unsigned []string
	for _, stem := range sortedSet(in.Enrolled) {
		if _, held := signed[stem]; !held {
			unsigned = append(unsigned, stem)
		}
	}

	var split textbuf.Buffer
	out := []string{
		"## Extraction sign-off",
		"",
		"Every other table here judges the requirements a summary LISTS. None of them " +
			"can see an obligation nobody wrote down, so a green gate is bounded by what " +
			"was extracted (`ai/rules/rfc-compliance.md`, Extraction Completeness). A " +
			"sign-off (`rfc/extraction/<stem>.json`) bounds the MISS: every normative " +
			"site of the RFC's own text is mapped to a requirement id or excluded with a " +
			"reason, and the gate re-derives the inventory and re-checks the arithmetic " +
			"on every run.",
		"",
		// NEVER a bare total: reading "N signed off" as "N keyword-verified" is
		// the category error this whole machinery exists to correct.
		split.Str("Signed off by register: ").Str(registerPhrase(counts)).
			Str(". Unsigned (grandfathered) backlog: ").Int(int64(len(unsigned))).
			Str(" of ").Int(int64(len(in.Enrolled))).
			Str(" enrolled. Every register counts toward the drain quota; each is ").
			Str("published apart so a count can never be read as stronger evidence than ").
			Str("it is.").String(),
		"",
		"`Register` is DERIVED from the source text and refused when an artifact claims " +
			"a stronger grade than the derivation supports: `rfc2119` (capitalised " +
			"keywords, at least as many sites as the summary declares gated rows), " +
			"`prose` (lowercase indicative modals), `manual-walk` (no mechanical " +
			"inventory exists at all -- an assertion the gate cannot verify). Derived " +
			"columns are blank for an unsigned stem: nobody has walked it, so there is " +
			"nothing established to publish.",
		"",
		"| RFC | Register | Sites | Mapped | Excluded | Exclusion ratio | Gated rows | " +
			"Signed off |",
		"|---|---|---|---|---|---|---|---|",
	}
	for _, stem := range sortedKeysOf(signed) {
		art := signed[stem]
		total := len(art.Sites)
		var ratio textbuf.Buffer
		if total == 0 {
			ratio.Str("--")
		} else {
			ratio.Float2(float64(art.Excluded()) / float64(total))
		}
		var row textbuf.Buffer
		out = append(out, row.Str("| `").Str(stem).Str("` | ").Str(art.Register).
			Str(" | ").Int(int64(total)).Str(" | ").Int(int64(art.Mapped())).
			Str(" | ").Int(int64(art.Excluded())).Str(" | ").Str(ratio.String()).
			Str(" | ").Int(int64(gated[stem])).Str(" | ").Str(art.SignedOff).
			Str(" (").Str(art.Reviewer).Str(") |").String())
	}
	for _, stem := range unsigned {
		var row textbuf.Buffer
		out = append(out, row.Str("| `").Str(stem).
			Str("` | -- | -- | -- | -- | -- | ").Int(int64(gated[stem])).
			Str(" | UNSIGNED (grandfathered) |").String())
	}
	out = append(out, "")
	return append(out, renderRelocationNote(signed)...), nil
}

// renderRelocationNote names the exclusions that homed an obligation elsewhere.
//
// DERIVED, so it appears exactly when the fact does and its absence is a true
// statement about the corpus. Not a column: the exclusion ratio above must keep
// counting a relocation, because a walk that relocated everything would
// otherwise publish a pristine 0.00 -- the very shape the ratio exists to make
// visible to a reviewer approving a first sign-off.
func renderRelocationNote(signed map[string]Extraction) []string {
	var stems []string
	total := 0
	for _, stem := range sortedKeysOf(signed) {
		if n := signed[stem].Relocated(); n > 0 {
			stems = append(stems, stem)
			total += n
		}
	}
	if len(stems) == 0 {
		return nil
	}
	var detail textbuf.Buffer
	for i, stem := range stems {
		if i > 0 {
			detail.Str(", ")
		}
		detail.Byte('`').Str(stem).Str("` ").Int(int64(signed[stem].Relocated()))
	}
	var tb textbuf.Buffer
	return []string{tb.Str("Of those exclusions, ").Int(int64(total)).
		Str(" carry `relocated-to-spec` (").Str(detail.String()).
		Str("): the obligation is owed, by the spec the site names, under the ").
		Str("requirement id reserved for it there. `./le rfc check` refuses the ").
		Str("sign-off unless that spec exists and still names that id, so a relocation ").
		Str("cannot outlive the document it points at. It is counted in `Excluded` and ").
		Str("in the ratio above because this walk did decline to map the sentence HERE, ").
		Str("and it is counted apart because a homed obligation and a dismissed ").
		Str("sentence are not the same fact.").String(), ""}
}

// registerPhrase names every register, including the zeros.
func registerPhrase(counts map[string]int) string {
	var tb textbuf.Buffer
	for i, name := range Registers() {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(name).Byte(' ').Int(int64(counts[name]))
	}
	return tb.String()
}

// renderStatusBacklog is the two backlogs the status ratchets grandfather,
// rendered rather than listed.
//
// The completeness ratchet lets the rowless enrolments that predate it through
// and the disposition check lets a declared summary through, so both would
// otherwise be invisible until someone re-ran the census by hand. Deriving them
// here keeps them countable in review without a second hand-maintained list.
// Sorted, because the freshness check compares bytes.
func renderStatusBacklog(in RenderInput) []string {
	var missing []string
	for _, stem := range sortedSet(in.Enrolled) {
		if _, held := in.Rows[stem]; !held {
			missing = append(missing, stem)
		}
	}
	var head textbuf.Buffer
	out := []string{
		"## Enrolled without a public status row",
		"",
		head.Int(int64(len(missing))).
			Str(" enrolled RFC(s) have no row in `docs/features/rfc-status.md`. Every ").
			Str("MUST-level requirement they declare is gated, so the obligation is ").
			Str("enforced while the public page says nothing about the RFC at all. ").
			Str("`check_status_completeness` grandfathers the ones that predate it and ").
			Str("refuses any NEW enrolment without a row, so this number can only shrink. ").
			Str("It is derived from `enrolled - rows`, never listed.").String(),
		"",
	}
	if len(missing) > 0 {
		out = append(out, "| RFC | Gated requirements enforced with no public row |", "|---|---|")
		for _, stem := range missing {
			var row textbuf.Buffer
			out = append(out, row.Str("| `").Str(stem).Str("` | yes |").String())
		}
	} else {
		out = append(out, "None: every enrolled RFC has a row.")
	}
	out = append(out, "",
		"## Declared not enrolled",
		"",
		"Each un-enrolled summary records why in its own `| Enrolment |` and `| Enrolment reason |` Meta rows, "+
			"so the remainder is a decision rather than an absence. Only `non-normative` "+
			"is a claim about the document; `backlog` and `blocked` are **DEBT** and are "+
			"listed as such. A disposition is discharged by enrolment and by nothing else.",
		"")
	if len(in.Dispositions) > 0 {
		out = append(out, "| Summary | Kind | Debt? | Reason |", "|---|---|---|---|")
		for _, stem := range sortedKeysOf(in.Dispositions) {
			disp := in.Dispositions[stem]
			debt := "**DEBT**"
			if disp.Kind == dispositionNonNormative {
				debt = "no"
			}
			// TableCell for the same reason the shard rows need it: the
			// reason is AUTHORED prose from a Meta cell, and an author
			// writes a grep alternation in that register.
			var row textbuf.Buffer
			out = append(out, row.Str("| `").Str(stem).Str("` | ").Str(disp.Kind).
				Str(" | ").Str(debt).Str(" | ").Str(TableCell(disp.Reason)).
				Str(" |").String())
		}
	} else {
		out = append(out, "None: every summary is enrolled.")
	}
	return append(out, "")
}

// renderUnconverted names the summaries that gate nothing.
//
// That is correct for a genuinely non-normative reference, and a capture failure
// for anything else. The pair of counts is what tells them apart: a zero
// uppercase count with a large lowercase count is the pre-RFC-2119 signature,
// and a genuinely non-normative document shows zero for both.
func renderUnconverted(in RenderInput) []string {
	stale := unconvertedSummaries(in.Tree, in.Stems, capturedGated(in.Requirements))
	if len(stale) == 0 {
		return nil
	}
	out := []string{
		"## Summaries declaring no MUST-level requirement",
		"",
		"These summaries gate nothing. That is correct for a genuinely non-normative " +
			"reference, and a capture failure for anything else. The uppercase column " +
			"counts MUST/MUST NOT/SHALL/SHALL NOT in the RFC's own text; the lowercase " +
			"column counts the same four words in indicative prose, which is the only " +
			"form a pre-RFC-2119 document has. A non-zero uppercase count with nothing " +
			"captured means the summary needs re-authoring (`/ze-rfc <stem>`) before its " +
			"RFC can ever be enrolled. A summary appears here whenever it captured no " +
			"MUST-level row, even if it captured SHOULD or MAY rows.",
		"",
		"| Summary | Uppercase | Lowercase | Verdict |",
		"|---|---|---|---|",
	}
	for _, row := range stale {
		var upper textbuf.Buffer
		var verdict string
		switch {
		case !row.UpperHeld:
			upper.Byte('?')
			verdict = "no source text under `rfc/full/` -- cannot judge"
		case row.Upper == 0 && row.Prose != 0:
			// The pre-2119 case, and the one the old verdict got wrong. RFC 1035
			// (1987) shows 0 uppercase and 23 lowercase `must`, and "consistent:
			// source declares none" read a normative wire specification as
			// non-normative.
			upper.Byte('0')
			var tb textbuf.Buffer
			verdict = tb.Str("**UNDECIDED**: 0 uppercase keywords but ").Int(int64(row.Prose)).
				Str(" lowercase -- a pre-RFC-2119 document states obligations in ").
				Str("indicative prose, so a zero uppercase count is not evidence of a ").
				Str("non-normative source. Needs a human reading, not a keyword count").
				String()
		case row.Upper == 0:
			upper.Byte('0')
			verdict = "consistent: source declares none in either register " +
				"(0 uppercase, 0 lowercase)"
		default:
			upper.Int(int64(row.Upper))
			verdict = "**RE-AUTHOR**: source is normative, summary captured nothing"
		}
		prose := "?"
		if row.ProseHeld {
			var tb textbuf.Buffer
			prose = tb.Int(int64(row.Prose)).String()
		}
		var cell textbuf.Buffer
		out = append(out, cell.Str("| `").Str(row.Stem).Str("` | ").Str(upper.String()).
			Str(" | ").Str(prose).Str(" | ").Str(verdict).Str(" |").String())
	}
	return append(out, "")
}
