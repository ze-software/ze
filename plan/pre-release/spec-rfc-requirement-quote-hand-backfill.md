# Spec: rfc-requirement-quote-hand-backfill

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Closed spec `spec-rfc-requirement-verbatim-quote` made every requirement
row in `rfc/short/<stem>.md` carry the RFC sentence it states, and
`./le rfc check` refuses a new or changed row whose quote is not in its section.
Its backfill (`./le rfc quote-backfill stem <stem> apply`) quoted 1821 rows
mechanically on 2026-09-26. This spec quotes every row the backfill could not.

The work ends when three things hold:

1. `./le rfc check` prints `unquoted: 0`, and no stem is named unjudged.
2. The row-quote rule applies to every row in the corpus, not only to a row a
   commit adds or changes. With the count at zero, the change scope has nothing
   left to protect.
3. Each row the backfill rewrote has had its tagged tests re-read against the
   new sentence (risk R-7 of the source spec). A test that proves less than the
   sentence now states is a finding: the test or the row is corrected, never
   the sentence.

### Rows left to quote

Figures from the backfill dry run over 201 stems (6354 rows), 2026-09-26. After
the apply, `./le rfc check` counted 3696 unquoted rows over 188 stems. The
difference from 3706 is the 10 `no-rfc-text` rows: rfc8326 (3) and rfc9129 (7)
have no text in `rfc/full/`, so the check names them unjudged and counts none
of their rows.

| Bucket | Kind | Rows | What a human does |
|--------|------|------|-------------------|
| review | partial | 256 | the sentence carries more obligations than the row; quote the whole sentence or split the row |
| review | number-absent | 217 | a number in the row is not in the sentence; find the sentence that carries it |
| review | qualified | 167 | the sentence qualifies the obligation; quote it with the qualifier |
| review | several-sites | 140 | the row maps to several sentences; pick one, or split the row |
| review | low-overlap | 108 | the paraphrase shares under half its content words with the sentence; check the mapping first |
| review | polarity-differs | 102 | NOT or MUST NOT differs between row and sentence; the row or the mapping is wrong |
| review | unresolved-anchor | 89 | the row's section does not resolve in the RFC text; correct the section |
| review | lead-in | 76 | the site is a lead-in to a list; quote the list item that states the obligation |
| review | outside-section | 69 | the sentence is not in the row's section; correct the section or the mapping |
| review | level-differs | 40 | the sentence's keyword differs from the row's level; correct the level or the mapping |
| review | short-sentence | 7 | the sentence is under the minimum length; quote the sentence with its context |
| human | unsourced | 1511 | the extraction declares the row has no source site; find the sentence, or retire the row |
| human | unmapped | 556 | no extraction site maps to the row; find the sentence |
| human | no-extraction | 358 | the stem has no extraction artifact; find each sentence by hand |
| human | no-rfc-text | 10 | fetch `rfc/full/rfc8326.txt` and `rfc/full/rfc9129.txt`, then quote |
| | **Total** | **3706** | review 1271, human 2435 |

### The review list

The per-stem list is not committed. It is derived: `./le rfc quote-backfill stem
<stem>` without `apply` writes nothing and lists every row it would not quote,
with its kind and reason. Run it for each stem in `ls rfc/short` to regenerate
the whole list over the tree in hand. A committed copy would be stale after the
first row quoted.

### Tagged tests to re-read (R-7)

Overlap cannot see a paraphrase whose words match and whose meaning differs.
The source spec measured about 1 in 15 applied rows as such. Three are known and
are read first:

| Row | What changed |
|-----|--------------|
| RFC7432-6.3-3 | the paraphrase named the normalized VID, the sentence names the originating VID |
| RFC9552-5.2-6 | the quoted sentence states a different obligation from the paraphrase |
| RFC5880-6.7.3-4 | the quoted sentence states a different obligation from the paraphrase |

Found by the rfc7606 re-audit (2026-09-26): four verdicts moved from enforced to
weak once the row states the full RFC sentence. Each note in
`rfc/audit/rfc7606.json` names the tag that moves it back; adding a tag owes a
discrimination record (`ai/rules/rfc-compliance.md`).

| Row | Clause no tagged unit drives |
|-----|------------------------------|
| RFC7606-3.g-2 | "whether recognized or unrecognized": both tagged units duplicate ORIGIN only |
| RFC7606-4-1 | the second §4 case, fewer than three octets remaining |
| RFC7606-7.10-2 | the zero-length half of "non-zero multiple of 4" |
| RFC7606-7.14-1 | the length-5 case; `TestRFC7606ExtendedCommunityLength` tests it but carries no tag |

The set to read is every row whose text changed in the backfill commit and whose
id a test tags (`RFC requirement:` tag). `git diff <backfill-commit>^ <backfill-commit> -- rfc/short`
names the rows.

### Owner decisions (2026-09-26)

| ID | Decision |
|----|----------|
| D-0 | One spec, run in phases: first the tooling, then the R-7 re-read, then the review bucket, then the human buckets stem by stem, then the rule flip. Each phase commits on its own |
| D-0b | Unsourced rows in prose-register stems are quoted here as well. The strength-4 blind walk later compares against the quoted rows |
| D-1 | In a stem with no numbered heading at all, the whole text is one citable section. A stem that has numbered sections still refuses a citation of the front matter |
| D-2 | A row that no RFC sentence states is retired through a dated paragraph in `rfc/corrections/<stem>.md`. That paragraph says why no sentence states the row. The tags move off it first, and the id is never reused |
| D-4 | Design approved. In every stem, a second agent that did not do the quoting re-derives a blind 10% sample, with at least one row. Any disagreement sends the whole stem back to be read again. No tool proposes a candidate sentence |
| D-3 | A row backed only by prose without an RFC 2119 keyword, or by a lowercase "must", is quoted verbatim and keeps its level. The level changes only when the sentence carries a different 2119 keyword. A demotion from MUST carries its correction paragraph |

## Required Reading

- `spec-rfc-requirement-verbatim-quote`, closed: its closure commit holds the
  spec (the rule, the backfill kinds, R-7, A-6); the rule and the kinds live on
  the page below
- `docs/contributing/rfc-conformance-gates.md`, "The row quote"
- `ai/rules/rfc-compliance.md`

## Current Behavior (MANDATORY)

Source files read so far. The design phase completes this list.

- [ ] `internal/le/rfc/quote_backfill.go` - `quoteBackfill` judges each row and names the kind that kept it unquoted
  → Constraint: `judgeBackfillRow` returns an empty quote for unmapped, unsourced and no-extraction rows, so 2423 rows get no candidate sentence today. `sitesFor` (inventory.go) plus `backfillOverlap` could propose one, but only if it matches every sentence and not just `siteKeywordRE`: an unsourced row has no keyword sentence by definition
- [ ] `internal/le/rfc/check_quote.go` - `checkRowQuotes` judges only rows the tip commit adds or changes; `unquotedFigures` prints the tree figure
  → Constraint: `unquotedFigures` already runs `rowQuoteRefusal` over every row on every `./le rfc check`, so the whole-corpus rule costs no extra pass. Fuse the two passes so each stem's `newQuoteSource` is built once
  → Decision: end condition 2 deletes the change scope rather than widening it (no-layering). What goes: `quoteRevisions`, `readQuoteRevisions`, `quoteChangedStems`, `quoteRowsAt`, `quoteSourceAt`, `scopeChangedRows`, `checkUnquotedRatchet`, `CheckReport.QuoteHistoryUnread` and its printed line. `stemPath` and `gitCatBlobs` stay because they have other callers
  → Constraint: once every row is judged, the fixture RFC text in `check_test.go:checkFixtureTree`, `selftest_state.go` and `selftest_core.go` must hold every fixture row verbatim. Rows such as "A speaker SHOULD count widgets (§2)" would otherwise add a violation to every `Check()` test. The fixture text grows; the assertions are not weakened. Tests that pin the scope get rewritten: `TestCheckCountsUnchangedUnquotedRow`, `TestCheckRefusesUnquotedCountRise*`, `TestCheckRefusesNewRowNotVerbatimInSection` (its violation count), `TestCheckNamesStemWithoutRFCTextUnjudged`
- [ ] `internal/le/rfc/inventory.go` - `sectionHeadingRE`, `quoteSource.has`
  → Constraint: only numbered or lettered headings are sections, and front matter can never be cited. In seven stems, rfc792, rfc2347, rfc2348, rfc2349, rfc1997, rfc2782 and rfc905 (69 rows), all the text is front matter, so no hand quote can pass there until the tooling changes
- [ ] `internal/le/rfc/freshness.go` - `verdictFreshness` compares an audit verdict's `requirement_sha` against `RequirementSHA(req.Text)`
  → Constraint: rewording a row stales its audit verdict, which is a violation (`check_audit.go:checkAuditFreshness`). Only rfc7606 (65 verdicts) and draft-abraitis-idr-addpath-paths-limit (9) have audit files. A reworded row in either one is re-audited in the same commit
- [ ] `internal/le/rfc/signoff.go`, `discriminate.go:claimSHA`, `render_ledger.go`
  → Constraint: none of these reads row text. The extraction sign-off keys on RFC-text SHA and ids, discrimination records key on the test tag's prose, and the status page keys on counts. A hand reword stales none of them
- [ ] `internal/le/rfc/check_ratchets.go` - `checkRetiredRequirements`, `checkIDAllocation`, `checkLevelRatchet`
  → Constraint: deleting an id that `HEAD^` held in an enrolled stem is refused, and no documented route retires a fabricated row. A section fix under the same id is allowed. A level demotion from MUST needs a "Correction <date>:" paragraph in `rfc/corrections/<stem>.md`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Rows enter as lines of `rfc/short/<stem>.md`, parsed by the `reChecklist` grammar (`rfc.go`) into a `Requirement`. RFC text enters from `rfc/full/<stem>.txt` or `rfc/drafts/`, through `SourceText`.

### Transformation Path

1. `Requirement.Quote` (`summary.go`) peels the trailing `{...}` markers and the section parenthetical off the row, which leaves the quote.
2. `newQuoteSource` (`inventory.go`) strips the page furniture, splits the text into section bodies at `sectionHeadingRE` and squashes whitespace.
3. `rowQuoteRefusal` (`check_quote.go`) resolves the cited section to its nearest heading ancestor, then matches the quote in that section and its subsections.
4. `checkRowQuotes` reports the refusals as violations; `unquotedFigures` counts them as the backlog.

### Boundaries Crossed

Summary file, then RFC text, then `./le rfc check` violations and JSON (`unquoted`, `unquoted-total`, `unquoted-unjudged`). Audit verdicts (`rfc/audit/<stem>.json`) are keyed on the row-text SHA, so a row edit crosses into the audit.

### Integration Points

`./le verify worktree` runs `./le rfc check` on a detached checkout. Test tags (`RFC requirement: <ID>`) name row ids, never row text. `./le rfc quote-backfill` shares `quoteSource` and `rowQuoteRefusal` with the check.

### Measured 2026-09-26 (research phase)

| Measure | Value | How |
|---------|-------|-----|
| Rows still unquoted | 3672 (review 1249, human 2423), in 189 stems | `./le rfc quote-backfill stem <s>` dry run over all 201 stems. The spec's 3706 was 34 higher; the difference was not reconciled |
| Rows the backfill commit changed | 1855 in 153 stems (the commit message says 1821) | the rows of `0bf0696576^` and `0bf0696576`, parsed and compared by id |
| Changed rows a test tags | 986, over 1582 units (1559 Go funcs, 23 `.ci`) | `rfc.ScanTree` over the working tree |
| Rows citing section `x` | 257 | every one needs a section correction first |
| Rows in stems with no numbered heading | 69, in 7 stems | `sectionHeadingRE` |
| Stems with no extraction | rfc6514 (201 rows), rfc7454 (64), rfc8362 (50), rfc7627 (27), draft-ietf-sidrops-8210bis (13), rfc8195 (3) | backfill dry run |
| Sample of 21 hand-checked rows | 12 verbatim in the cited section, 4 in another section, 2 span consecutive sentences, 1 with no supporting sentence, 2 blocked by the tooling | read against `rfc/full` |
| Sample level mismatches | 5 of 21: the RFC has no 2119 keyword or lowercase "must" (5216, 8195, 7474, 792), or the row's level is wrong (RFC7950-7.6.4-1 is SHOULD, the RFC says MUST) | same |
| Sample effort | about 5 min a row on average, roughly 300 hours over 3672 rows | same |
| R-7 calibration | 2 of 3 known rows are only partly proven. RFC9552-5.2-6 does not drive adding or removing a TLV. RFC5880-6.7.3-4 does not drive the 32-bit wrap. RFC7432-6.3-3 is not-applicable, so there is no test | test bodies read |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | "unsourced" does not mean fabricated | in the sample, 2 of 3 unsourced rows had a sentence (RFC4271-4.3-4 shares the §4.3 sentence with -3) | retiring by kind would delete real obligations | every row is read against the RFC before any retire; the retire count is recorded per stem | unvalidated |
| A-2 | about 5% of rows have no supporting sentence | 1 of 21 sampled (RFC1877-x-5) | a larger retire volume means the ledger was inflated, and its public figures move | retire count per tranche; a tranche above 15% stops and is reported to the owner | unvalidated |
| A-3 | a verbatim span over consecutive sentences in one section replaces most row splits | the check matches any span inside one section body (`rowQuoteRefusal`) | rows split, and tags must follow the new ids | the sampled class (c) rows (RFC1661-2-1, RFC7474-5-2) pass as spans | unvalidated |
| A-4 | a hand reword stales only audit verdicts | `verdictFreshness` is the only reader of row text; sign-off (`signoff.go`), discrimination (`claimSHA`) and the render (`render_ledger.go`) do not read it | a mass reword triggers refusals across the ledger | the first stem commit of phase 3 runs `./le rfc check` with no new violation | unvalidated |
| A-5 | an audit file may carry verdicts for only some rows of a stem | `checkAuditSchema` and `checkAuditFreshness` iterate the verdicts present; no check demands a verdict per row (`check_audit.go`) | R-7 verdicts could not land stem by stem | the first R-7 stem commit passes `./le rfc check` with a partial audit file | unvalidated |
| A-6 | the audit check accepts a verdict on any enrolled stem | `checkAuditFiles` refuses only a file for an un-enrolled stem or a missing summary | R-7 rows of an un-enrolled stem have no durable record | list the R-7 stems that are not enrolled before phase 2 starts; their re-reads go in the per-stem table in this spec | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | a quoter picks a sentence that is in the RFC but states another obligation, which is the class the backfill's R-7 exposed | the blind sample disagrees with the quoter | owner decision D-4: in every stem, a second agent that did not quote re-derives a 10% sample blind, and any disagreement sends the whole stem back |
| R-2 | 69 rows in 7 stems cannot pass without a tooling change | unresolved-anchor refusals that name no numbered section | D-1, phase 1 |
| R-3 | the whole-corpus rule reddens every `Check()` fixture test | fixture rows that are not verbatim in `checkFixtureSource` | the fixture RFC text grows in the same phase-5 commit, and no assertion is weakened |
| R-4 | concurrent sessions edit `rfc/short`, `rfc/audit` or the tagged tests while stems land | a foreign hunk in a stem's summary or test file; SHIFTED verdicts (seen 2026-09-26 from another session's lint rewrite of the rfc7606 tests) | one stem per commit; a foreign hunk is judged against HEAD (`ai/rules/git-safety.md`), and a SHIFTED verdict from foreign work is resealed only after that work is committed |
| R-5 | a level demotion from MUST owes a correction paragraph and changes the ledger's MUST count | `checkLevelRatchet` refusal | the correction paragraph quotes the sentence and lands in the same commit; the per-stem table records the count of level changes |
| R-6 | a retired row drops tags, an extraction mapping, or an audit verdict that still names it | "unknown RFC requirement" (`check_core.go:evaluate`); `signoff.go` refusing a `mapped-to` or `unsourced-ids` entry that names a missing id; `checkAuditSchema` refusing a verdict on a missing id | the retire commit moves the tags to the row that states the obligation, or deletes a tag whose only claim was the fabricated row. It also drops the id from the extraction and the audit file, and D-2's correction paragraph names each move |
| R-7 | an R-7 re-read finds a test that proves less than its sentence, and the fix is a code change, not a test change | the audit verdict is `wrong` or `unimplemented` | the verdict is recorded and disclosed on the status page (`checkAuditDisclosure`). A defect in an implemented capability is fixed under `ai/rules/completion.md`; an absent feature is recorded as a gap (`ai/rules/rfc-compliance.md`). The spec stays open until each one has a home |
| R-8 | the row-count figures shift under other sessions (3706 in the skeleton, 3672 measured; 1821 in the backfill commit message, 1855 measured) | the phase-start dry run disagrees with the figure in this spec | each phase starts from a fresh dry run; the spec's figures are a baseline, and AC-1 is the end condition, not a count |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the daemon. The public RFC ledger and `docs/features/rfc-status.md` counts move when rows retire or change level. A wrong quote misstates what Ze claims to conform to |
| How is it reverted? | per stem commit; the phase-1 and phase-5 tooling commits revert on their own |
| Who else touches this path? | strength-4 (the blind second walk of the prose stems, which reads the quoted rows afterwards); any session that edits `rfc/short`, `rfc/audit` or RFC-tagged tests |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a fixture commit whose unquoted row is unchanged from `HEAD^` | → | `checkRowQuotes` over every row | `TestCheckRefusesUnchangedUnquotedRow` (`internal/le/rfc/check_quote_test.go`), which replaces `TestCheckCountsUnchangedUnquotedRow` |
| `./le rfc check` over a fixture stem with no numbered heading | → | `quoteSource` resolving the whole-text section | `TestCheckAcceptsWholeTextQuoteInUnnumberedStem` (`check_quote_test.go`) |
| `./le rfc check` over a fixture commit that deletes an enrolled id | → | `checkRetiredRequirements` reading `rfc/corrections/<stem>.md` | `TestCheckAcceptsRetiredRowWithCorrection` (`check_ratchets_test.go`, or the file that holds the retire tests) |
| `./le rfc self-test` quote stage | → | `runQuoteSelftest` | `TestRFCSelftestQuoteStageRefusesFabricatedRow` stays green unchanged |

## Acceptance Criteria

| AC | Given / When | Then |
|----|--------------|------|
| AC-1 | `./le rfc check` on the tree at the end of phase 4 | its output carries no unquoted figure above zero and names no unjudged stem, and the JSON has `unquoted-total` 0 and an empty `unquoted-unjudged` |
| AC-2 | a fixture commit whose summary holds a row that is not verbatim and identical at `HEAD^` | `./le rfc check` exits non-zero and names that row, with the refusal reason (`rowQuoteRefusal`) |
| AC-3 | the phase-5 tree | none of `quoteRevisions`, `readQuoteRevisions`, `quoteChangedStems`, `quoteRowsAt`, `quoteSourceAt`, `scopeChangedRows`, `checkUnquotedRatchet` or `CheckReport.QuoteHistoryUnread` exists, and a grep for each returns nothing |
| AC-4 | a fixture stem with no numbered heading, and a row that cites the whole-text section with a verbatim quote | accepted. The same citation in a stem that has numbered headings is refused as an unresolved anchor |
| AC-5 | a fixture commit that deletes an enrolled id | refused with no correction paragraph; accepted with a dated retirement paragraph naming the id in `rfc/corrections/<stem>.md`; a later row reusing the id is refused (`checkIDAllocation`) |
| AC-6 | rfc8326 and rfc9129 | their text is in `rfc/full/`, and their 10 rows are quoted |
| AC-7 | each of the 986 tagged rows the backfill changed, in an enrolled stem | carries a fresh verdict in `rfc/audit/<stem>.json` (`verdictFreshness` answers fresh). Each row in an un-enrolled stem has a line in the R-7 table of this spec |
| AC-8 | a verdict `weak`, `wrong` or `unimplemented` from AC-7 | is resolved inside this spec: the test is fixed (verdict `enforced`, unit fingerprint moved), or the row is corrected, or the defect or gap is homed under R-7. Nothing sits as an open finding at closure without a named home |
| AC-9 | RFC7606-3.g-2, 4-1, 7.10-2, 7.14-1 | `enforced` in `rfc/audit/rfc7606.json`, and each new tag carries a discrimination record (`./le rfc discriminate-record`) |
| AC-10 | each quoted stem | its blind 10% sample (at least one row) is recorded in the per-stem table with agreement yes or no; every "no" has a re-read recorded |
| AC-11 | a row retired under D-2 | its correction paragraph quotes the search that found no sentence (sections read) and names where each of its tags moved |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestCheckRefusesUnchangedUnquotedRow` | `internal/le/rfc/check_quote_test.go` | AC-2 |
| `TestCheckRefusesNewRowNotVerbatimInSection` (the violation count drops from 2 to 1 once the ratchet goes) | same | AC-2, AC-3 |
| `TestCheckNamesStemWithoutRFCTextRefused` (replaces `TestCheckNamesStemWithoutRFCTextUnjudged`) | same | a stem with no RFC text is refused row by row, not left unjudged |
| `TestCheckAcceptsWholeTextQuoteInUnnumberedStem` | same | AC-4 accept |
| `TestCheckRefusesFrontCitationInNumberedStem` | same | AC-4 refuse |
| `TestCheckRefusesRetiredRowWithoutCorrection` | the file that holds the `checkRetiredRequirements` tests | AC-5 refuse |
| `TestCheckAcceptsRetiredRowWithCorrection` | same | AC-5 accept |
| `TestCheckRefusesRetiredIDReuse` | same | AC-5 reuse |
| Deleted: `TestCheckRefusesUnquotedCountRise`, `TestCheckRefusesUnquotedCountRiseFromRFCTextChange` | `check_quote_test.go` | their subject, the ratchet, is deleted (no-layering) |

### Boundary Tests (numeric inputs)

N-A: the change takes no new numeric input. The 24-character minimum quote is already pinned by existing tests.

### Functional Tests

| Test | Validates |
|------|-----------|
| `./le rfc check` over the real tree at the end of phase 4 and of phase 5 | AC-1 (output pasted into the spec) |

### Interop Tests

N-A: tooling and ledger text; no protocol behaviour changes. An R-7 finding that turns out to be a protocol defect is handled under R-7 and carries its own interop obligation.

## Files to Modify

- `internal/le/rfc/check_quote.go` - judge every row; delete the change scope and the ratchet; fuse with `unquotedFigures`
- `internal/le/rfc/check.go` - drop the calls, `QuoteHistoryUnread`, and its printed line
- `internal/le/rfc/inventory.go` - `quoteSource`: a stem with no numbered heading exposes its whole text as one section (D-1)
- `internal/le/rfc/check_ratchets.go` - `checkRetiredRequirements` accepts a retirement named by a dated paragraph in `rfc/corrections/<stem>.md` (D-2)
- `internal/le/rfc/quote_backfill.go` - same whole-text section, so the dry run stops sending those rows to review
- `internal/le/rfc/check_quote_test.go`, `check_test.go`, `selftest_state.go`, `selftest_core.go` - the fixture RFC text grows to hold every fixture row verbatim
- `rfc/short/<stem>.md` - the rows, stem by stem
- `rfc/audit/<stem>.json` - R-7 verdicts, and re-judged verdicts for reworded rows in rfc7606 and draft-abraitis-idr-addpath-paths-limit
- `rfc/corrections/<stem>.md` and `rfc/corrections/README.md` - retirement and level-correction paragraphs, and the retirement form
- `rfc/extraction/<stem>.json` - drop the ids of retired rows from `mapped-to` and `unsourced-ids`
- tagged tests (`*_test.go`, `.ci`) - R-7 fixes and the rfc7606 tags; `rfc/discrimination/<stem>.json` for each new tag
- `docs/contributing/rfc-conformance-gates.md` - "The row quote" (every row judged, no ratchet, whole-text section) and "Requirements do not vanish" (retirement by correction)

## Files to Create

- `rfc/full/rfc8326.txt`, `rfc/full/rfc9129.txt`
- `rfc/audit/<stem>.json` for each enrolled R-7 stem that has none

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | no config |
| YANG validation constraints | N-A | no config |
| YANG custom validators | N-A | no config |
| CLI commands/flags | No | `./le rfc check` keeps its surface; only its verdicts widen. The JSON loses the field behind `QuoteHistoryUnread` if one is emitted |
| CLI grammar | N-A | no new verb |
| Editor autocomplete | N-A | no YANG |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | le tooling |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | no family |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | tooling only |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | `./le rfc check` verdicts widen; the gates page carries it (row 12) |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/<stem>.md` rows; `docs/features/rfc-status.md` regenerates from the summaries when a retirement or a level change moves a count |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/contributing/rfc-conformance-gates.md`, "The row quote" and "Requirements do not vanish", edited in the phase that changes each behaviour |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors` (2026-09-26) names two pages. The first is `docs/architecture/core-design.md`, declared by `check.go`, `inventory.go` and `check_ratchets.go`. It is unaffected: it describes the le artifact and hook model, not the row check, the section resolution or the retire rule. The second is `docs/architecture/testing/test-health.md`, which cites `check_ratchets.go` for `checkDiscriminationRatchet` only. It is unaffected because that ratchet does not change. The gates page is the one that changes (row 12). The anchors are re-run at phase 1 and phase 5 |
| 17 | Existing docs show examples for this area? | Yes | the gates page describes the ratchet and the change scope; phase 5 rewrites those paragraphs |

## Implementation Steps

Each phase runs in subagents through `/ze-implement`. The main thread supervises, verifies each report against source, and gates the next phase. Every stem tranche is one commit.

1. **Phase: Wiring and tooling (D-1, D-2, texts)**
   - Tests: `TestCheckAcceptsWholeTextQuoteInUnnumberedStem`, `TestCheckRefusesFrontCitationInNumberedStem`, `TestCheckRefusesRetiredRowWithoutCorrection`, `TestCheckAcceptsRetiredRowWithCorrection`, `TestCheckRefusesRetiredIDReuse`
   - Files: `inventory.go`, `quote_backfill.go`, `check_ratchets.go`, `rfc/corrections/README.md`, the gates page; fetch `rfc/full/rfc8326.txt` and `rfc/full/rfc9129.txt`
   - Verify: tests fail, then pass. The dry run no longer lists the 69 rows of the seven stems as unresolved-anchor
2. **Phase: R-7 re-read**
   - Input: the 986 ids (the backfill commit `0bf0696576`, rows parsed at both revisions, tags from `rfc.ScanTree`), grouped by stem. The three known rows and the rfc7606 four go first
   - Per stem: an agent applies `/ze-rfc-audit` to the changed tagged rows only and writes their verdicts. A second agent re-judges 10% blind (D-4). Findings go through AC-8. The rfc7606 four get their tags and discrimination records (AC-9)
   - Verify: `./le rfc check` shows no stale or missing verdict for the stem; the per-stem table below gains a row
3. **Phase: Review bucket (1249 rows)**
   - Per stem: an agent reads each row's cited section in `rfc/full` and writes the verbatim sentence or span. It fixes the section or the level (D-3, with a correction paragraph when it demotes a MUST), or retires the row (D-2). The tagged tests of each quoted row get their `/ze-rfc-audit` verdict (R-1). A second agent re-derives 10% blind
   - Verify: the stem's dry run lists nothing; `./le rfc check` shows no new violation
4. **Phase: Human buckets (2423 rows)**
   - Same procedure as phase 3, largest stems first (rfc6514, rfc9830, rfc4035, rfc2131, rfc4271, ...). The 257 rows citing `x` get their section first
   - Verify: at the end, `./le rfc check` meets AC-1 (output pasted)
5. **Phase: Rule over every row**
   - Tests: `TestCheckRefusesUnchangedUnquotedRow`, `TestCheckNamesStemWithoutRFCTextRefused`, the rewritten `TestCheckRefusesNewRowNotVerbatimInSection`; the ratchet tests are deleted
   - Files: `check_quote.go`, `check.go`, the fixtures, the gates page
   - Verify: AC-2, AC-3; `./le verify worktree`

### Deriving the R-7 set

Any session can rebuild the list; it is not committed, because it is derived. The steps:

1. Parse `rfc/short/<stem>.md` at `0bf0696576^` and at `0bf0696576` with the `reChecklist` grammar (`rfc.go`), for every stem that commit touched.
2. Pair the rows by id, and keep each id whose text differs. That gives 1855 on 2026-09-26, with no ids added or removed.
3. Scan tags with `rfc.ScanTree` (`carriers.go`), and keep the changed ids that at least one tag names. That gave 986.
4. Group by stem. For each stem, list the tagged units (`rfc.UnitAt`).

A `go run` of this lives only in a session's scratch. A session that repeats it states its own counts in the per-stem table and does not copy these ones.

### Stem brief (phases 2 to 4)

Every agent for a stem gets this brief, in this order. Phase 2 runs only steps 1, 6, 7, 8 and 9, on the stem's R-7 ids.

| Step | Action | Record |
|------|--------|--------|
| 1 | Run `./le rfc quote-backfill stem <stem>` (dry) and take the stem's review and human rows, with their kinds. For phase 2, take the stem's R-7 ids instead | the counts in the per-stem row |
| 2 | For each row, read the cited section of `rfc/full/<stem>.txt` (or `rfc/drafts/`). When the row cites `x` or an unresolved section, first find the section that states the obligation | - |
| 3 | Write the verbatim sentence, or a verbatim span of consecutive sentences in one section, as the row text. Keep the id, the markers and the parenthetical. Correct the section when the sentence is in another one | - |
| 4 | Level (D-3): keep the row's level unless the sentence carries a different RFC 2119 keyword. A demotion from MUST adds a dated "Correction" paragraph to `rfc/corrections/<stem>.md`, quoting the sentence | the level-change count |
| 5 | Retire (D-2), only after reading the whole RFC for the obligation, not just the cited section. Check first whether another document states it; a re-attribution then follows `plan/pre-release/spec-rfc-requirement-reattribution.md`, and is not a retirement. That spec is a skeleton, and the gates refuse a move today, so a re-attribution case is reported to the owner, not forced. Otherwise: move each tag to the row that does state the obligation, or delete a tag whose only claim was this row. Drop the id from `rfc/extraction/<stem>.json` and `rfc/audit/<stem>.json`. Delete the row and write the dated retirement paragraph naming the sections read and where each tag went | the retired count; stop and report if it passes 15% of the stem's rows (A-2) |
| 6 | For every row whose text changed in this step or in the backfill, and that a test tags, apply `/ze-rfc-audit` to that row only: read the RFC sentence and each tagged unit, and write the verdict in `rfc/audit/<stem>.json`. An un-enrolled stem gets a line in the per-stem table instead (A-6) | verdict counts |
| 7 | A `weak`, `wrong` or `unimplemented` verdict follows AC-8: fix the test (a new tag owes `./le rfc discriminate-record`), correct the row, or home the defect or gap under R-7 | the findings, named in the per-stem row |
| 8 | Run `./le rfc check`: no new violation for the stem, and the stem's dry run lists no row | - |
| 9 | Blind sample (D-4). A second agent gets the stem, the row ids and the RFC text, but not the new row text or the verdicts. It re-derives 10% of the changed rows, at least one, spread over the kinds. The main thread compares the two. Any disagreement returns the whole stem to step 2 | agreed yes/no |
| 10 | One commit per stem through `./le commit create`, naming the summary, the audit, the extraction, the corrections and any tests | the commit SHA |

### Per-stem progress

One row per stem, appended when the stem's commit lands.

| Stem | Phase | Rows quoted | Retired | Level changed | R-7 verdicts (enforced / weak / wrong) | Blind sample agreed | Commit |
|------|-------|-------------|---------|---------------|-----------------------------------------|---------------------|--------|

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation, and the per-stem table covers every stem the dry run listed |
| Correctness | A quote states the row's obligation, not a neighbouring one: the blind sample agrees |
| Retire discipline | Every retired row has its correction paragraph, and no tag, mapping or verdict still names it |
| Rule: no-layering | Phase 5 deletes the change scope and the ratchet, and nothing keeps both paths |
| Rule: never weaken a test | An R-7 finding fixes the test or the row, and never removes a tag to clear a verdict |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Unquoted backlog at zero | `./le rfc check` output, pasted |
| Change scope gone | grep for each AC-3 symbol returns nothing |
| R-7 verdicts | `./le rfc check` with no stale verdict, and the per-stem table sums to 986 |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | `rfc/corrections` parsing: a malformed paragraph must not accept a retirement it does not name |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No sentence proposer | the backfill proposes a best-overlap sentence for the human rows | overlap is the mechanism that let the backfill quote sentences stating other obligations (R-7). Search is not the cost; judgement is |
| R-7 verdicts in `rfc/audit/<stem>.json` | a per-row table in this spec | audit verdicts already exist, are keyed on the row text, and stale by themselves when a row changes again |
| Delete the change scope at the end | widen it to every row and keep the ratchet | no-layering: with the whole corpus judged, the ratchet protects nothing |
| Tooling first, rule flip last | flip first and quote under a red gate | a red `./le rfc check` on every commit would block every other session |

## Known Limitations

- The strength-4 blind second walk of the prose stems stays in `plan/pre-release/spec-rfc-evidence-strength-4-prose-second-walk.md`. This spec quotes the rows it will compare against.
- Tagged tests of rows the backfill did NOT change, and that phases 3 and 4 do not quote, are outside R-7: their row text did not move.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** remove the spec
