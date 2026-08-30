# Spec: rfc-superseded-requirements-carry-their-successor

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 7/7 |
| Deferral shard | `plan/deferrals/rfc-superseded-requirements-carry-their-successor.md` <!-- doc-links: ignore (shard created only on the first deferral, and none has happened) --> |
| Handoff | - |
| Updated | 2026-08-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Three enrolled summaries state obligations from documents the IETF has
superseded, and nothing in the machinery knows. A reader who opens a requirement
line sees a MUST with no sign that the document stating it was replaced, and
`ai/rules/rfc-compliance.md` says the lineage that matters runs FORWARD.

| Stem | Requirements | Obsoleted by | Successor summarised |
|------|--------------|--------------|----------------------|
| `rfc3768` | 50 | RFC 5798, then RFC 9568 | yes, enrolled |
| `rfc7752` | 51 | RFC 9552 | yes, enrolled |
| `rfc7627` | 27 | RFC 9846 | no, text absent |

Mark every requirement of a superseded document with where that obligation now
lives, and make a gate keep it true.

Two facts make this a machinery change rather than an editing pass.
`ANNOTATION_KINDS` (`internal/le/rfc/rfc.go`) is a CLOSED set of three
-- `not-applicable`, `gap`, `single-polarity` -- so there is no vocabulary for
"this obligation moved". And the `| Obsoleted by |` row in a summary's Meta table
is not parsed anywhere: it is prose a human writes and nothing reads, which is
why three summaries can carry it and still be gated as current.

## Required Reading

- [ ] `internal/le/rfc/rfc.go` - `ANNOTATION_KINDS`, `parse_checklist_line`, the check registry
- [ ] `rfc/short/rfc3768.md` - a superseded summary whose successor is enrolled
- [ ] `rfc/short/rfc7627.md` - a superseded summary whose successor is absent
- [ ] `rfc/short/rfc9568.md` - the successor, for what a forward pointer must resolve to
- [ ] `rfc/extraction/README.md` - the contract the derived-not-authored rule comes from

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
- [ ] `docs/architecture/testing/test-health.md` - how ze answers whether a regression would actually be caught

- [ ] `ai/rules/rfc-compliance.md` - the forward-lineage rule, and that annotations are the owner's to decide

## Current Behavior (MANDATORY)

Source read for this section:

- [ ] `internal/le/rfc/rfc.go`
- [ ] `rfc/short/rfc3768.md`
- [ ] `rfc/short/rfc7627.md`
- [ ] `rfc/enrolled.txt`

`ANNOTATION_KINDS` is `frozenset({"not-applicable", "gap", "single-polarity"})`.
A requirement line carries at most one annotation from that set, and every one
of them says something about Ze's coverage. None says anything about the
DOCUMENT's standing.

A grep for `Obsoleted by` and `obsoleted` over `internal/le/rfc/rfc.go`
returns nothing, so the Meta row is unparsed. Ten summaries carry the row; three
carry a real successor and the rest say `None` or `-`.

All three superseded stems are in `rfc/enrolled.txt`, so their MUST-level
requirements are gated, counted in the published totals, and ratcheted by
`check_coverage_ratchet` exactly as a current document's are.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

`./le rfc check` -> `run_check()` -> the check registry -> per-summary
`parse_checklist_line` over `rfc/short/*.md`.

### Transformation Path

A summary's Meta table and its checklist lines are read into requirement
records; `./le rfc index-update` derives `ai/RFC-REQUIREMENTS.md` and
`rfc/requirements/<stem>.md` from those records.

### Boundaries Crossed

| From | To | Where | What crosses |
|------|----|-------|--------------|
| summary Meta table | checker | a new Meta parser | the obsoleting RFC, if any |
| checklist line | checker | `parse_checklist_line` | the annotation, now including a successor pointer |
| checker | published ledger | `./le rfc index-update` | the superseded marking, so a reader of the ledger sees it too |

### Integration Points

The check registry is where a new check registers. `ANNOTATION_KINDS` is the
closed set a new kind must join, and `parse_checklist_line` is the one producer
that reads an annotation.

### Architectural Verification

| Claim | Holds? | Evidence |
|-------|--------|----------|
| `ANNOTATION_KINDS` is closed and has exactly three members | yes | `internal/le/rfc/rfc.go`, `ANNOTATION_KINDS` = `frozenset({"not-applicable", "gap", "single-polarity"})`; `_parse_annotation` raises on any other kind. It still holds exactly three: `superseded` is a separate kind (`SUPERSEDED_KIND`) on a separate `Requirement` field, so it cannot evict a coverage annotation |
| No code reads the `Obsoleted by` Meta row | was true, now false | `grep -rn "Obsoleted by\|obsoleted\|supersed" internal/le/` found no hit in `rfc_requirements.py` before this change. `parse_successor_stem` and `summary_successors` now read it |
| All three superseded stems are enrolled | NO -- broken | `rfc7627` is not in `rfc/enrolled.txt`; it carries a `backlog` disposition in `rfc/not-enrolled.txt`. The other two are enrolled. `check_superseded` therefore runs over EVERY summary rather than the enrolled set, which is what the Task section's reader-facing goal needs anyway |
| A new check registers rather than being called directly | yes | `check_superseded` is one `errs.extend(...)` line in `run_check`, beside `evaluate`, and `TestSupersededWiring` drives `run_check` end to end so an unwired check fails the test |
| The published ledger is derived, never hand-edited | yes | `check_ledger_fresh` re-renders `ai/RFC-REQUIREMENTS.md` and every shard through `render_index` / `render_shards` and reds on any difference. The superseded facts come from `summary_successors()` and `Requirement.superseded`, both derived at run time |
| A FOURTH stem is superseded and the spec missed it | found | `rfc/short/rfc5549.md` writes its Meta row as `Obsoleted By` with a capital B, which the spec's case-sensitive grep missed. RFC 8950 obsoletes it, both its summary and its text are held, and its 9 requirements are now marked. The real population is 137, not 128 |
| THREE MORE stems, and the reader was fixed one spelling at a time | found and fixed | `_OBSOLETED_ROW_RE` matched `Obsoleted by` alone, and the case fix for rfc5549 left the hyphen open. `Obsoleted-by` is the corpus MAJORITY: 28 rows against 18 for the space. `rfc5575` -> RFC 8955, `rfc6810` -> RFC 8210 and `rfc1334` -> RFC 1994 each name a real successor the repository holds, and each got no obligation at all. The label now reads `Obsoleted[ -]by[^\|]*` (the tail absorbs rfc1334's `(partial)`), `_NO_SUCCESSOR_RE` takes rfc8654's `(none)`, and `parse_successor_stem` REFUSES any other Meta field matching `obsolet` rather than skipping it, which is the part that stops the class recurring. The real population is 230 over 7 stems, not 137 over 4 |
| One CONSUMER of the ledger's State cell broke | found and fixed | `collect_rfc` (`internal/le/testhealth/testhealth.go`) compared the State cell with `== "**enrolled**"`. The suffix dropped four rows and 71 gated requirements off `docs/features/test-health.md`, silently, because the remainder and the annotation split were narrowed by the same filter and still balanced. Now a prefix match, pinned by `test_a_superseded_enrolled_row_stays_in_the_population` |
| The shard Note cell escapes nothing | found and fixed | `render_shards` wrote an authored annotation reason straight into a markdown table cell, so a reason quoting a grep alternation split its row. 113 rows over 9 shards were in that state at HEAD. This spec writes a SECOND mark into that same cell, so it is code related to the work in hand: `_table_cell` escapes `\|`, `test_a_pipe_in_a_reason_does_not_split_the_shard_row` counts the unescaped pipes, and every row in every shard now has 6 cells. Row in `plan/journal/rendered-markup-invalid.md` |

## Risks & Assumptions

### Assumptions

- A-1: A superseded obligation usually survives into its successor, sometimes
  renumbered and sometimes reworded. Unvalidated: some are simply dropped, and
  the marker must be able to say "dropped, not moved" without that reading as
  "Ze need not comply".
  **CONFIRMED, and the drops are common: 24 of the 230 requirements are `dropped`.**
  VRRPv3 removed authentication and the legacy-media appendices, which drops 7
  RFC 3768 requirements. RFC 9552 deprecated the BGP-LS Identifier sub-TLV and
  turned four RFC 7752 obligations into indicative prose, which drops 7 more. RFC
  8950 dropped RFC 5549's dynamic-capability MAY. The largest group is `rfc1334`:
  RFC 1994 replaced its CHAP half and defines no PAP packet at all, so 9 of its 10
  requirements are `dropped` and every one of them is still owed, because Ze
  authenticates PAP peers. That is what A-1 predicted and it is the case the
  `dropped` disposition exists for.
- A-2: The successor's requirement ids are stable enough to point at. For
  `rfc9568` and `rfc9552` the summaries exist so a pointer can be checked; for
  `rfc9846` no text is in the repository, so a pointer cannot resolve.
  **BROKEN in one direction the spec did not foresee.** The successor's summary
  can be held and still declare no row for the obligation, because the successor
  was under-extracted. 14 RFC 7752 requirements are in that state: RFC 9552 states
  each one, and `rfc/short/rfc9552.md` declares no row for any of them. That is a
  third answer, not a pointer and not a drop, so the vocabulary gained
  `unextracted <§section>` for it. See Key Design Decisions.
- A-3: Marking a requirement superseded must NOT lower what Ze owes. It is a
  statement about the document, not about coverage, so it must be orthogonal to
  `gap` / `not-applicable` / `single-polarity` rather than replacing one.
  **CONFIRMED and enforced by construction.** The marker lands on
  `Requirement.superseded`, a field `evaluate` never reads, so a marked
  requirement takes the same path an unmarked one takes. Proven by
  `TestSupersededDoesNotLowerCoverage` (five cases: still gated, still needs both
  polarities, still counted by `rfc_coverage`, still fires
  `check_coverage_ratchet`, and does not evict a `{gap}`), and at corpus scale by
  the ledger: every Gated, Both, Annotated and Outstanding count for the four
  stems is byte-identical to HEAD, and the gate still reports 2966 gated
  requirements across 171 enrolled RFCs.

### Risks

- R-1: A `{superseded}` marker that a reader treats as an exemption is worse than
  no marker. It must read as "look here instead", never as "not owed".
- R-2: Requiring a pointer on all 128 lines at once is a large authoring pass,
  and a pass done to satisfy a gate produces pointers nobody checked.
- R-3: `rfc7627`'s successor is not in the repository, so its lines cannot carry
  a resolvable pointer until RFC 9846 is fetched and summarised.

## Blast Radius

`internal/le/rfc/rfc.go`, the three superseded summaries, and the
derived ledger. No production code. Every other summary is unaffected, because a
summary whose Meta names no successor gains no obligation.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` on a superseded summary with an unmarked line | → | `check_superseded`, reached from `run_check` | `TestSupersededWiring.test_an_unmarked_superseded_summary_fails_the_gate`, and `TestSupersededCheck.test_superseded_requires_a_successor_pointer` over the helper |
| `./le rfc check` on a current summary | → | the same check | `TestSupersededCheck.test_current_summary_gains_no_obligation`, and its stale-marker twin `test_marker_on_a_current_summary_reds` |
| a `{superseded: ...}` line | → | `parse_checklist_line` -> `_strip_markers` -> `_parse_successor` | `TestSupersededMarkerParsing`, 9 cases |
| `./le rfc index-update` | → | `render_shards` and `_render_rollup` | `TestSupersededLedger.test_shard_banner_and_note_name_the_successor` and `test_rollup_states_the_successor_and_counts_unresolved_as_debt` |
| the ledger's State cell | → | `collect_rfc` (`internal/le/testhealth/testhealth.go`) | `TestRfcLedgerParse.test_a_superseded_enrolled_row_stays_in_the_population` |

## Acceptance Criteria

- AC-1: A summary's `| Obsoleted by |` Meta row is parsed into a fact the checker
  holds, and a row naming no successor yields no obligation.
- AC-2: `ANNOTATION_KINDS` gains a kind that names where an obligation now lives,
  and it composes with the existing three rather than replacing them.
- AC-3: A requirement line in a superseded summary that carries no successor
  pointer reds `./le rfc check`, naming the id and the obsoleting RFC.
- AC-4: The marker cannot lower coverage: a requirement marked superseded is
  still counted, still gated, and still ratcheted.
- AC-5: A pointer that names a requirement id in a summary the repository holds
  is checked to resolve; one that names a document not in the repository is
  accepted with its reason stated, and counted as debt rather than as settled.
- AC-6: All 50 `rfc3768` and 51 `rfc7752` requirements carry a pointer.
- AC-7: The published ledger shows which stems are superseded, derived rather
  than hand-written.

## End-to-End User Stories

A reader opens `RFC3768-5.2.3-2` and sees, on the line, that VRRPv3 restates it
and where.

An implementer picking up a VRRP task learns from the summary that RFC 3768 was
replaced, without knowing to check the Meta table.

Someone adding a summary for a document obsoleted last year cannot enrol it
without saying what replaced each obligation.

## 🧪 TDD Test Plan

### Unit Tests

All in `internal/le/` unless the row says otherwise. 49 cases
across six classes; the file runs 872 tests.

| Test | Asserts |
|------|---------|
| `TestSupersededMarkerParsing` (9 cases) | the marker parses with and without a target; it COMPOSES with `{gap}` and with `{single-polarity}` in either order; a missing reason, an unknown disposition, a `restated` with no id, a `dropped` with one, and two markers of one register each raise |
| `TestObsoletedByMetaRow` (14 cases) | the chain's LAST RFC wins; all four label spellings parse, hyphen and space in either capitalisation, and a qualifier after the label is kept; `None`, `-`, `n/a`, `(none)` and an absent row yield no successor; rfc2661's `-` followed by prose naming RFC 3931 yields none; a row naming no RFC, a row naming itself, and a Meta field naming obsolescence any other way each raise, while `Obsoletes` and `Obsoletes / Updates` do not; the real corpus derives all seven successors |
| `TestSupersededCheck` (11 cases) | an unmarked line reds naming the id and the obsoleting RFC; a marked line passes (the discriminating twin); a current summary gains nothing and a stale marker on one reds; each disposition's precondition reds when unmet; a pointer at a nonexistent id and a pointer at the wrong document each red |
| `TestSupersededDoesNotLowerCoverage` (5 cases) | a marked requirement is still gated by `evaluate`, still needs both polarities, still counted by `rfc_coverage`, still fires `check_coverage_ratchet`, and does not evict a `{gap}` |
| `TestSupersededLedger` (3 cases) | the shard banner and the per-row Note name the successor; the rollup states it and counts the debt; a current summary gains no ledger prose |
| `TestSupersededWiring` (4 cases) | `run_check` reds on an unmarked superseded summary and exits 0 on a marked one; it reds the same way when the Meta row uses the hyphenated label, which is the spelling that failed open; and an unrecognised `obsolet` Meta field stops the run with exit 2, naming the field |
| `TestRfcLedgerParse.test_a_superseded_enrolled_row_stays_in_the_population` (`internal/le/`) | the suffixed State cell keeps its row in the health page's enrolled population |

### Functional Tests

N-A with a reason, and the reason is the point rather than an excuse. This spec
changes no daemon behavior: it touches `internal/le/rfc/rfc.go` and
three markdown summaries, and nothing it does is reachable from a running `ze`.
A `.ci` drives a daemon, and there is no daemon surface here to drive. The
end-to-end evidence is the gate itself:

| Test | Location | Scenario |
|------|----------|----------|
| `./le rfc check` end to end | `internal/le/` | the real tree passes with the three summaries marked, and reds naming the id when one pointer is removed |
| `./le rfc index-update` | `internal/le/` | regenerates, and the ledger diff is only this change |

## Files to Modify

- `internal/le/rfc/rfc.go` - Meta parsing, the new marker kind, the new check
- `internal/le/` - the unit tests above
- `internal/le/testhealth/testhealth.go` - the State-cell consumer this change broke, now a
  prefix match
- `internal/le/` - the test that pins it
- `rfc/short/rfc3768.md` - 50 markers against RFC 9568 (43 restated, 7 dropped)
- `rfc/short/rfc7752.md` - 51 markers against RFC 9552 (30 restated, 7 dropped, 14
  unextracted)
- `rfc/short/rfc5549.md` - 9 markers against RFC 8950 (8 restated, 1 dropped). NOT in
  the spec's original table: its Meta row writes `Obsoleted By` with a capital B
- `rfc/short/rfc7627.md` - 27 markers against RFC 9846, all `unresolved` until its text
  is held
- `rfc/short/rfc5575.md` - 16 markers against RFC 8955 (14 restated, 2 unextracted).
  Hidden by the hyphenated label
- `rfc/short/rfc6810.md` - 67 markers against RFC 8210 (56 restated, 11 unextracted).
  Hidden by the hyphenated label
- `rfc/short/rfc1334.md` - 10 markers against RFC 1994 (1 restated, 9 dropped). Hidden
  by the hyphenated label, and the only stem whose successor replaced HALF the document
- `ai/rules/points/rfc-compliance/rfc-summaries-rfc-short/mark-every-requirement-of-a-superseded-summary-with-its-successor.md`
  and the rule's `manifest.md` - the new rule point
- `ai/skills/ze-rfc.md` - the marker's grammar, beside the three annotation kinds
- `ai/RFC-REQUIREMENTS.md` and `rfc/requirements/*.md` - regenerated, never hand-edited

## Files to Create

- `plan/deferrals/rfc-superseded-requirements-carry-their-successor.md` <!-- doc-links: ignore (shard created only on the first deferral, and none has happened) --> - if anything defers

### Integration Checklist

- [ ] `./le rfc check` exits 0 with all SEVEN summaries marked. `check_superseded`
      reports 0 over the whole corpus, and the gate's remaining 5 violations are
      `rfc/short/rfc9552.md` rows another session committed in `02ca02af6` and left
      needing the owner's coverage ruling. They were red in this session's first
      baseline run, before any edit. `./le rfc check` itself was unusable through
      the the native action tables under `internal/le/` for part of the session while a concurrent session moved the
      functional-suite list from `internal/le/functional/suites.go` to `internal/le/`; every run
      recorded here is `./le rfc check` directly
- [ ] Removing one pointer reds it, naming the id. Yes, stripping the marker from
      `RFC3768-5.2.3-2` gives:

```
RFC3768-5.2.3-2 [MUST] states an obligation of a document RFC9568 obsoletes,
and does not say where that obligation now lives
```

- [ ] A pointer at an id the successor does not declare reds. Yes, repointing the
      same line at `RFC9568-99.9-1` gives:

```
points at RFC9568-99.9-1, which rfc/short/rfc9568.md does not declare
```

- [ ] `./le rfc index-update` regenerates cleanly and the diff is only this
      change. Yes: 178 shards rewritten; every Gated / Both / Annotated /
      Outstanding count is byte-identical to HEAD, and only the State cell,
      the shard banner and the per-row Note gained the superseded facts
- [ ] The three newly exposed stems change no count. Yes. Regenerating after they
      were marked touched exactly three shards and four lines of the index: the
      rollup (4 summaries to 7, `unextracted` debt 14 to 27) and one State cell per
      stem. Their numeric columns are unchanged -- `rfc1334` 7 gated, `rfc5575` 12,
      `rfc6810` 39 -- and parsing each summary with every marker stripped yields
      requirements identical in `level`, `gated`, `section`, `text` and annotation

### Documentation Update Checklist (BLOCKING)

- [ ] `rfc/extraction/README.md` or its sibling - document the new annotation kind.
      Yes, in the sibling: `ai/skills/ze-rfc.md` is where the other three annotation
      kinds are documented, and it gained a "Superseded Documents Carry Their
      Successor" section with the grammar, the four dispositions and their
      preconditions. `rfc/extraction/README.md` is the extraction-artifact contract
      and says nothing about annotations, so nothing there changed
- [ ] `ai/rules/rfc-compliance.md` - state that a superseded document's requirements
      carry their successor. Yes: new point
      `points/rfc-compliance/rfc-summaries-rfc-short/mark-every-requirement-of-a-superseded-summary-with-its-successor.md`,
      listed in the manifest, rendered by `./le rules render-update`, and
      `./le rules render-check`, `-index-check`, `-condensed-check`,
      `-points-roundtrip-check` and `-lint` all exit 0

## Implementation Steps

1. Parse the Meta row and prove no current summary changes behavior.
2. Add the annotation kind and its parser test before any summary is edited.
3. Add the check, and prove it reds on an unmarked line.
4. Mark `rfc3768` against `rfc9568`, one requirement at a time, reading both.
   This is the step R-2 warns about: a pointer written to satisfy the gate is
   worse than none.
5. Mark `rfc7752` against `rfc9552` the same way.
6. Mark `rfc7627` against `rfc9846`, which needs its text fetched first.

Steps 1 to 3 landed before any summary was edited, and the gate was proven to red
on all 137 unmarked lines at that point. Step 4 and step 5 were done against the
RFC texts, not the summaries alone: `rfc/full/rfc9568.txt` and
`rfc/full/rfc9552.txt` were read for every requirement whose successor the summary
comparison left ambiguous.

Two steps were added, and one was cut.

Step 4b marks `rfc5549` against `rfc9568`'s sibling RFC 8950. The spec's stem table
missed it because `rfc/short/rfc5549.md` writes `Obsoleted By` with a capital B.

Step 7 repairs `collect_rfc` in `internal/le/testhealth/testhealth.go`, the one consumer of
the ledger's State cell, which this change would otherwise have silently narrowed.

Step 6 does NOT fetch RFC 9846. Fetching and summarising an RFC is its own spec with
its own extraction sign-off, and doing it inside this one would have made the marking
pass hostage to a 200-page TLS document. All 27 `rfc7627` lines therefore carry
`{superseded: unresolved; ...}`, which is the disposition AC-5 asks for, and the
ledger publishes them as debt.

Step 8 fixes the label as a CLASS and marks the 93 lines it exposes. Step 4b had
fixed one spelling, the capital B, and left the hyphen open; the hyphenated form is
the corpus majority. `rfc5575` (16 markers against RFC 8955), `rfc6810` (67 against
RFC 8210) and `rfc1334` (10 against RFC 1994) were each gated as current documents.
The reader now takes `Obsoleted by` and `Obsoleted-by` in either capitalisation, with
a qualifier after the label, and REFUSES any other Meta field matching `obsolet`
instead of skipping it. Every one of the 93 was read against the successor's own
text: `rfc/full/rfc8955.txt`, `rfc/full/rfc8210.txt` and `rfc/full/rfc1994.txt`,
section by section, not by matching id shapes.

### Critical Review Checklist

- [ ] Does a `{superseded}` marker read as "look here", never as "not owed"?
- [ ] Can a marked requirement still fail the coverage ratchet?
- [ ] Does a summary with no successor gain any obligation at all?

### Deliverables Checklist

- [ ] Every AC has working code and a test that can fail
- [ ] No requirement lost a polarity or a level to gain a pointer

### Security Review Checklist

- [ ] No annotation kind can be used to remove a MUST from the gated population

### Failure Routing

A red that is a missing pointer is the gate working. A red that is a pointer
into a requirement that does not exist is a real finding: either the successor
renumbered it or the obligation was dropped, and those are different answers.

## Design Insights

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| `superseded` is NOT a member of `ANNOTATION_KINDS`, and lands on its own `Requirement.superseded` field | AC-2 asks for a kind that "composes rather than replaces". `Requirement.annotation` is one slot, so joining the set would have made marking a requirement EVICT its `{gap}` or `{single-polarity}`, and obsolescence would have become a route out of the population every ratchet judges. A separate field makes AC-4 hold by construction instead of by care: `evaluate` never reads it |
| The Meta row is read as a CHAIN, oldest first, and the LAST RFC named wins | `rfc/short/rfc3768.md` already writes `RFC 5798, which was in turn obsoleted by RFC 9568`. Pointing rfc3768 at RFC 5798 would point it at a document that is itself superseded, which `ai/rules/rfc-compliance.md` forbids: the lineage that matters runs forward |
| `None` and `-` are matched at the START of the row's value, never anywhere in it | `rfc/short/rfc2661.md` opens with `-` and then explains in prose that RFC 3931 is a distinct protocol rather than a successor. A whole-value scan for an RFC number would have read that explanation as a successor and demanded 18 forward pointers into a document that obsoletes nothing |
| Four dispositions, not three, and each carries a precondition a machine checks | `restated` needs the successor's summary and the id in it; `dropped` and `unextracted` need the successor's own text, because both claim somebody read it; `unresolved` needs that text to be ABSENT. Without the preconditions, marking every line `unresolved` would be the cheapest route from red to green. The simpler design considered and rejected was three dispositions with `unresolved` meaning "no checkable pointer": it collapses "we do not have the document" into "we did not extract it", which are different debts with different fixes, and it leaves `unresolved` with no precondition at all |
| `unextracted` records the debt rather than extracting the missing rows now | RFC 9552 states 14 obligations `rfc/short/rfc9552.md` does not declare. Adding a MUST-level row to an enrolled summary requires a coverage annotation, and `ai/rules/rfc-compliance.md` reserves that judgement to the owner. So the extraction pass over `rfc9552` is separable work with its own spec, and the marker names the section so the next reader can check the claim |
| `check_superseded` runs over EVERY summary, not the enrolled set | A reader who opens a requirement line of an obsoleted document needs to know it was obsoleted whether or not that document is gated. `rfc7627` is not enrolled and is exactly that case |

## Known Limitations

Three debts, two of them counted and published in `ai/RFC-REQUIREMENTS.md` rather
than hidden, and each drained by its own spec. The third is a ledger-policy
question the owner has not ruled on.

`dropped` carries NO published debt. `_render_rollup` counts `unextracted` and
`unresolved` and never counts `dropped`, so 24 of the 230 markers -- 9 of
`rfc1334`'s 10 among them -- publish as settled. That is defensible on the
disposition's own terms, because `dropped` is a finished reading of the successor
and nothing further is owed to the LEDGER. It is not obviously right for the
READER: a `dropped` obligation is still owed on the wire, and the page that names
every other debt says nothing about the group that Ze must implement with no
successor to point at. Making it a counted debt is a one-line change in
`_render_rollup` plus its prose, and it is the owner's call because it changes
what the public page claims. Nothing was changed here.

RFC 9846 is not in the repository, so `rfc7627`'s 27 pointers cannot resolve
until it is fetched and summarised. All 27 read `{superseded: unresolved; ...}`.

`rfc/short/rfc9552.md` under-extracts RFC 9552: obligations RFC 9552 STATES have
no row in its summary, so 14 `rfc7752` requirements read
`{superseded: unextracted <§section>; ...}`, over nine sections -- §5.2,
§5.2.1.1, §5.2.2.1, §8.1.2, §8.1.6, §8.2.2, §8.2.3, §8.2.5 and §8.2.6.

Seven of the missing obligations are MUST-level in RFC 9552, so extracting them
gates them, and a gated row needs coverage or a conformance annotation that
`ai/rules/rfc-compliance.md` reserves to Thomas. They are: §5.2.1.1 (A) and (B),
the same node/key uniqueness pair RFC 7752 stated; the two "MUST perform the
following syntactic validation" lists of §8.2.2, one over the NLRI and one over
the BGP-LS Attribute; the §8.2.6 operator import policy, which RFC 9552 raised
from SHOULD to MUST; the §8.2.3 obligation to let an operator configure the
8-octet BGP-LS Instance-ID, which RFC 9552 raised from MAY; and the §5.2.2.1 rule
that the MT-ID reserved bits are zero on origination, which RFC 9552 raised from
SHOULD. That last group matters beyond bookkeeping: three obligations got
STRONGER in the successor and the summary records neither the old level nor the
new one.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
