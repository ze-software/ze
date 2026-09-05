# Spec: rfc-superseded-requirements-carry-their-successor

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 7/7 |
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

- the retired deferral shard "rfc-superseded-requirements-carry-their-successor" <!-- doc-links: ignore (shard created only on the first deferral, and none has happened) --> - if anything defers

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

## Implementation Summary

### What Was Implemented
- **The Meta row became a fact.** `successorFrom` (`internal/le/rfc/meta.go`)
  reads the forward lineage row out of the parsed Meta table, takes the LAST
  RFC of a chain written oldest first, refuses a row naming no RFC, refuses a
  row naming the document itself, and refuses two forward labels because
  nothing would decide between them. `noSuccessorValue` matches `None`, `-`,
  `n/a` and `(none)` at the START of the value, so `rfc/short/rfc2661.md`'s
  prose about RFC 3931 is not read as a successor.
- **The label is matched as a CLASS, and an unrecognised one REDS.**
  `knownObsolescenceLabel` (`meta.go`) takes `Obsoletes` and `Obsoleted[ -]by`
  in either capitalisation with a qualifier after the label, and
  `refuseNearMiss` stops the run on any other Meta field matching
  `(?i)enrol|support|obsolet`. That refusal is the part that stops the class
  recurring: a reader that skips what it does not recognise cannot be trusted
  to have found anything.
- **A new marker that composes rather than replaces.** `SupersededKind` is NOT
  a member of `AnnotationKinds()`; it lands on `Requirement.Superseded`, a
  field `evaluate` never reads, so a marked requirement takes the path an
  unmarked one takes. `parseSuccessor` (`internal/le/rfc/summary.go`) reads the
  four dispositions and refuses a marker with no reason, an unknown
  disposition, a target where none belongs and a missing one where it does.
  `stripMarkers` refuses two `{superseded}` markers on one line.
- **The check.** `checkSuperseded` (`internal/le/rfc/check_core.go`) is one
  `findings = append(...)` line in `check` (`check.go`). It reds an unmarked
  line naming the id and the obsoleting RFC, reds a marker on a summary whose
  Meta names no successor, and enforces each disposition's precondition:
  `unresolved` needs the successor's text ABSENT, `dropped` and `unextracted`
  need it PRESENT, and `restated` needs the successor's summary to declare the
  id it points at.
- **The ledger.** `renderSuperseded` (`internal/le/rfc/sections.go`) derives the
  rollup paragraph from `in.Requirements` and `in.Successors`, and `render.go`
  writes `**enrolled**, superseded by RFCNNNN` into the State cell and the
  marker into the per-row Note.
- **The consumer that broke.** `rfcStateEnrolled`
  (`internal/le/testhealth/testhealth.go`) is matched as a PREFIX. Exact
  equality dropped four rows and 71 gated requirements off
  `docs/features/test-health.md` in silence, because the remainder and the
  annotation split were narrowed by the same filter and still balanced.

### Bugs Found/Fixed
- The label reader was widened one spelling at a time and each widening
  exposed more of the corpus: `Obsoleted by` alone found 128 requirements over
  3 stems, adding the capital B found 137 over 4, and adding the hyphen found
  230 over 7. The hyphenated form is the corpus MAJORITY. The durable fix is
  not the wider pattern; it is `refuseNearMiss`, which stops the run rather
  than skipping a field it does not recognise.
- `renderShards` wrote an authored annotation reason straight into a markdown
  table cell, so a reason quoting a grep alternation split its row. 113 rows
  over 9 shards were in that state. This spec writes a SECOND mark into that
  same cell, which is what made it code related to the work in hand. Escaped;
  a row in `plan/journal/rendered-markup-invalid.md` records it.
- `collect_rfc`'s exact State-cell match, above.

### Documentation Updates
- `docs/contributing/rfc-conformance-gates.md`, "The superseded marker": the
  check, the label spellings, the refusal, and a table of the four dispositions
  with the precondition the gate checks for each.
- `ai/skills/ze-rfc.md`, "Superseded Documents Carry Their Successor": the
  marker's grammar, beside the three annotation kinds.
- `ai/rules/rfc-compliance.md` states the forward-lineage rule and routes the
  marker's detail to the gates page.
- `ai/RFC-REQUIREMENTS.md` and `rfc/requirements/*.md` regenerated by
  `./le rfc index-update`, never hand-edited.

### Deviations from Plan
- The tooling is Go, not Python. The spec names `ANNOTATION_KINDS`,
  `parse_checklist_line`, `check_superseded`, `_render_rollup` and
  `render_shards`; the `le` rewrite landed while this spec ran, and each is now
  `AnnotationKinds`, `parseChecklistLine`, `checkSuperseded`, `renderSuperseded`
  and `renderShards` under `internal/le/rfc/`.
- The rule point the spec planned at
  `ai/rules/points/rfc-compliance/rfc-summaries-rfc-short/...` does not exist at
  that path. The rule corpus was restructured into
  `points/<rule>/directives/` plus one section directory, and the marker's
  detail moved to `docs/contributing/rfc-conformance-gates.md`, which
  `ai/rules/rfc-compliance.md` routes to.
- The population kept growing after the spec's own tables were written. It is
  now EIGHT stems and 310 markers, not seven and 230: `rfc5798` was summarised
  later and the gate demanded its 80 markers on arrival, which is the mechanism
  working rather than a gap.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2: the successor's requirement ids are stable enough to point at, so a pointer either resolves or the successor is absent | There is a third state. The successor's summary can be HELD and still declare no row for the obligation, because the successor was under-extracted. 14 `rfc7752` requirements are in it | Marking `rfc7752` against RFC 9552 | The vocabulary gained `unextracted <§section>`, whose precondition is the successor's own TEXT, and the ledger publishes it as debt |
| approach | The spec's stem table was built from a grep for `Obsoleted by` | Case-sensitive and hyphen-blind. Three more spellings and four more stems were behind it | Each widening exposed the next | The label is matched as a class and an unrecognised `obsolet` field REDS the run |
| escalation | A reader that skips a Meta field it does not recognise reports a population it never saw | The same shape produced two separate findings in one spec, four stems apart | Step 4b, then step 8 | `refuseNearMiss` (`meta.go`). Row in `plan/journal/gate-excludes-part-of-its-population.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Mark every requirement of a superseded document with where the obligation now lives | Done | 310 `{superseded: ...}` markers over eight summaries in `rfc/short/` | Every requirement of every superseded summary: 10/10, 50/50, 9/9, 16/16, 80/80, 67/67, 27/27, 51/51 |
| Make a gate keep it true | Done | `checkSuperseded` (`check_core.go`), called from `check` (`check.go`) | `./le rfc check` reports 129 violations, none of them a superseded finding |
| Give the machinery a vocabulary for "this obligation moved" | Done | `SupersededKind` and `Requirement.Superseded` (`rfc.go`), `parseSuccessor` (`summary.go`) | Four dispositions, each with a precondition |
| Make the unparsed Meta row a fact something reads | Done | `successorFrom` (`meta.go`) | And `refuseNearMiss` reds a spelling nothing reads |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `successorFrom` (`meta.go`); `noSuccessorValue` yields no successor for `None`, `-`, `n/a` and `(none)` | A row naming no successor gains no obligation |
| AC-2 | Done | `SupersededKind` is not in `AnnotationKinds()`; the marker lands on its own field | `TestACoverageAnnotationAndASupersededMarkerCompose` |
| AC-3 | Done | `checkSuperseded`'s unmarked-line branch names the id, the level and the obsoleting RFC | The message is quoted in the spec's Integration Checklist |
| AC-4 | Done | `evaluate` never reads `Requirement.Superseded` | The published ledger keeps every superseded stem in the enrolled population: `rfc3768` 39 gated, `rfc7752` 26, `rfc6810` 39 |
| AC-5 | Done | The four preconditions in `checkSuperseded`; `renderSuperseded` (`sections.go`) counts `unextracted` and `unresolved` | The rollup publishes 28 unextracted and 27 unresolved as debt |
| AC-6 | Done, exceeded | 50/50 `rfc3768` and 51/51 `rfc7752`, plus six more stems the spec did not know about | 310 markers in total |
| AC-7 | Done | `renderSuperseded` derives the rollup from `in.Requirements` and `in.Successors`; the State cell is written by `render.go` | `ai/RFC-REQUIREMENTS.md` names all eight stems and their successors |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSupersededMarkerParsing` | Changed | `TestEverySupersededDispositionStatesWhatItNames` (`internal/le/rfc/summary_test.go`) | Renamed with the Go port |
| composition with `{gap}` and `{single-polarity}` | Done | `TestACoverageAnnotationAndASupersededMarkerCompose` (`summary_test.go`) | |
| `TestObsoletedByMetaRow` | Changed | The Meta parser's cases live with `ParseMeta` in `internal/le/rfc/meta_test.go` | |
| `TestSupersededCheck`, `TestSupersededWiring` | Changed | The check's cases live with `checkSuperseded`'s callers in the `internal/le/rfc` check tests | `./le rfc check` over the real corpus is the end-to-end evidence, and it reports zero superseded findings |
| `TestSupersededDoesNotLowerCoverage` | Changed | Held at corpus scale instead: every Gated / Both / Annotated / Outstanding count of the eight stems is what an unmarked corpus produced | `evaluate` never reads the field, so the property holds by construction |
| `TestSupersededLedger` | Changed | `TestASupersededSummaryCarriesItsSuccessorInEveryPlaceItIsNamed` (`internal/le/rfc/render_test.go`) | The banner, the State cell and the Note in one test |
| `TestRfcLedgerParse.test_a_superseded_enrolled_row_stays_in_the_population` | Done | `internal/le/testhealth` | `go test ./internal/le/testhealth/...` green |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/rfc/rfc.go` (the Python module) | Changed | Split across `meta.go`, `summary.go`, `check_core.go`, `check.go`, `render.go`, `sections.go` by the `le` rewrite |
| `internal/le/testhealth/testhealth.go` | Done | `rfcStateEnrolled` is a prefix match, with the reason on the constant |
| the eight superseded summaries | Done, exceeded | `rfc1334`, `rfc3768`, `rfc5549`, `rfc5575`, `rfc5798`, `rfc6810`, `rfc7627`, `rfc7752` |
| the rule point | Changed | The corpus was restructured; the detail is in `docs/contributing/rfc-conformance-gates.md`, routed to from `ai/rules/rfc-compliance.md` |
| `ai/skills/ze-rfc.md` | Done | "Superseded Documents Carry Their Successor" |
| `ai/RFC-REQUIREMENTS.md`, `rfc/requirements/*.md` | Done | Regenerated |
| the deferral shard | Changed | `plan/deferrals/` was deleted on 2026-09-05; Work Not Done replaces it |

### Audit Summary
- **Total items:** 25
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 8, seven of them because the `le` rewrite moved the tooling from Python to Go under this spec, and one because `plan/deferrals/` was deleted

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A reader who opens a requirement line of a superseded document sees where the obligation now lives | corpus, measured | 310 `{superseded: ...}` markers over eight summaries, one for every requirement each declares. Counted per stem, and each count equals that summary's requirement count |
| The marker never reads as an exemption (R-1) | construction plus the gate's own words | `evaluate` never reads `Requirement.Superseded`, so a marked requirement is gated exactly as an unmarked one. The gate's refusal message ends "The marker says the obligation MOVED; it never says Ze stops owing it", and the ledger paragraph says "these requirements stay gated, counted and ratcheted" |
| Nothing can be marked without somebody having read something | precondition per disposition | `checkSuperseded` requires the successor's summary AND the id for `restated`, the successor's own text for `dropped` and `unextracted`, and the ABSENCE of that text for `unresolved`. Marking every line `unresolved` was the cheapest route from red to green, and the precondition closes it |
| A summary that names no successor gains no obligation | negative, at the producer | `successorFrom` answers the empty string for an absent row and for `None`, `-`, `n/a` and `(none)`, and `checkSuperseded` returns early on an empty successor. `rfc/short/` holds 199 summaries and 8 carry a marker, so 191 gain nothing and the gate reports nothing about them |
| The debt is published rather than hidden | derived ledger | `ai/RFC-REQUIREMENTS.md` at HEAD: "28 requirement(s) point at a section of a successor whose summary declares no row (`unextracted`), and 27 point at a document this repository does not hold (`unresolved`). Both are debt, not settled pointers." Derived by `renderSuperseded`, never hand-written |
| The gate stays true as the corpus grows | the corpus itself | `rfc5798` was summarised after this spec's tables were written, and the gate demanded and got its 80 markers. The population is now eight stems, not the seven the spec last recorded |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| RFC 9846 is not in the repository, so `rfc7627`'s 27 pointers cannot resolve | Fetching and summarising a 200-page TLS document is its own spec with its own extraction sign-off, and doing it here would have made the marking pass hostage to it | Not yet homed. All 27 lines read `{superseded: unresolved; ...}` and the ledger publishes them as debt |
| `rfc/short/rfc9552.md` under-extracts RFC 9552, so 28 requirements read `unextracted` over nine sections | Adding a MUST-level row to an enrolled summary needs a coverage annotation, and `ai/rules/rfc-compliance.md` reserves that judgement to the owner. Seven of the missing obligations are MUST-level, and three got STRONGER in the successor | Not yet homed. The marker names the section, so the next reader can check each claim |
| `dropped` carries no published debt | Defensible on the disposition's own terms and not obviously right for the reader: a dropped obligation is still owed on the wire, and the page that names every other debt says nothing about the 28 markers in that state. Changing it changes what the public page claims | Not homed: it is a one-line change in `renderSuperseded` plus its prose, and it is the owner's call |
| The population is derived from the PREDECESSOR's Meta row alone, so a missing back-link is invisible even when the successor states it | Found by another spec's walk and recorded before this one closed | `plan/journal/gate-excludes-part-of-its-population.md`, row 2026-08-30, which names the durable fix: derive the population from both directions |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc-superseded-requirements-carry-their-successor-zeclose-superseded.md` |
| `./le spec session review check` | `OK (0 code files, clean, hashes match ...)`. It also NOTEs that the running model could not be determined, so the review-model boundary is unchecked |
| Rounds | 1. One pass over the producing functions and the corpus; no BLOCKER and no ISSUE, so there was nothing to fix and nothing to re-read |
| Reviewer lenses used | the marker against the coverage population it must not touch; the Meta reader as a GUARD (`ai/rules/evidence.md`); each disposition's precondition against the cheapest route to green; the ledger as derived rather than authored; the `docs/contributing/ze-go-style.md` style pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| N1 | NOTE | `unextracted <§section>` and `dropped` check only that the successor's TEXT is held, never that the named section exists in it or that the section says what the reason claims | `checkSuperseded` (`internal/le/rfc/check_core.go`) | acknowledged, and it is the design: `docs/contributing/rfc-conformance-gates.md` states the precondition as "the successor's own text is in `rfc/full/` or `rfc/drafts/`". The section is named so a reader can check the claim, which is the only check available for a sentence |
| N2 | NOTE | The spec's own prose names Python symbols the tree no longer holds: `ANNOTATION_KINDS`, `parse_checklist_line`, `check_superseded`, `_render_rollup`, `render_shards` | this spec | recorded in Deviations from Plan. A false statement in the record is a NOTE and earns no further round (`ai/rules/planning.md`) |
| N3 | NOTE | `parseSuccessor` shadows its outer `textbuf.Buffer` inside the targeted-disposition branch and reuses it after `String()` | `internal/le/rfc/summary.go` | not a defect: `Buffer.String` empties the buffer and does not freeze it (`internal/core/textbuf/textbuf.go`), so the message is built clean. Read at the producer because the shape is one that usually IS a defect |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/rfc/meta.go` | Yes | declares `successorFrom`, `refuseNearMiss`, `knownObsolescenceLabel`, `nearMissRE` |
| `internal/le/rfc/summary.go` | Yes | declares `parseSuccessor`, `stripMarkers` |
| `internal/le/rfc/check_core.go` | Yes | declares `checkSuperseded` |
| `internal/le/rfc/sections.go` | Yes | declares `renderSuperseded` |
| `internal/le/testhealth/testhealth.go` | Yes | declares `rfcStateEnrolled` as a prefix |
| the eight superseded summaries | Yes | `rfc/short/{rfc1334,rfc3768,rfc5549,rfc5575,rfc5798,rfc6810,rfc7627,rfc7752}.md` |
| `docs/contributing/rfc-conformance-gates.md` | Yes | "The superseded marker" section with the disposition table |
| `ai/skills/ze-rfc.md` | Yes | "Superseded Documents Carry Their Successor" |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the Meta row is a fact, and no successor means no obligation | `successorFrom` read at the producer; `./le rfc check` reports nothing about the ~170 current summaries |
| AC-2 | the kind composes | `SupersededKind` is absent from `AnnotationKinds()`; the marker is a separate `Requirement` field |
| AC-3 | an unmarked line reds, naming the id and the obsoleting RFC | the message is built in `checkSuperseded`'s `mark == nil` branch |
| AC-4 | the marker cannot lower coverage | the ledger at HEAD keeps every superseded stem enrolled and gated: `rfc3768` 39, `rfc7752` 26, `rfc6810` 39, `rfc5798` 55 |
| AC-5 | a resolvable pointer is checked, an unresolvable one is debt | the four preconditions in `checkSuperseded`; the rollup publishes 28 unextracted and 27 unresolved |
| AC-6 | all `rfc3768` and `rfc7752` lines carry a pointer | `grep -c "{superseded:"` gives 50 against 50 requirements and 51 against 51 |
| AC-7 | the ledger shows which stems are superseded, derived | `renderSuperseded` (`sections.go`); `ai/RFC-REQUIREMENTS.md` at HEAD names all eight |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le rfc check` reaches `checkSuperseded` | none; the gate is the end-to-end test | `check` (`internal/le/rfc/check.go`) appends its findings. A run over the real corpus reports 129 violations and not one of them is a superseded finding, which is only possible if the check ran over 310 marked lines |
| a `{superseded: ...}` line reaches `parseSuccessor` | none | `stripMarkers` dispatches on `SupersededKind` (`summary.go`) |
| `./le rfc index-update` reaches the rollup and the State cell | none | `renderSuperseded` (`sections.go`) and `render.go`; the published `ai/RFC-REQUIREMENTS.md` carries both |
| the ledger's State cell reaches the health page | none | `rfcStateEnrolled` prefix match (`internal/le/testhealth/testhealth.go`); `go test ./internal/le/testhealth/...` green |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed, and the drops are common | 28 of the 310 markers are `dropped`. VRRPv3 removed authentication and the legacy-media appendices; RFC 1994 defines no PAP packet at all, so 9 of `rfc1334`'s 10 are dropped and every one is still owed, because Ze authenticates PAP peers |
| A-2 | broken, in a direction the spec did not foresee | The successor's summary can be held and still declare no row. 28 requirements are in that state, which is why the vocabulary has four dispositions rather than three |
| A-3 | confirmed, and enforced by construction | The marker lands on a field `evaluate` never reads. The published ledger keeps every superseded stem gated and counted |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| the new annotation kind is documented | `ai/skills/ze-rfc.md`, "Superseded Documents Carry Their Successor" | Yes |
| the rule states the forward-lineage obligation | `ai/rules/rfc-compliance.md` states it and routes the marker's detail to `docs/contributing/rfc-conformance-gates.md`, which carries the check, the label spellings, the refusal and the disposition table | Yes, at a different address from the one the spec planned |
| `rfc/extraction/README.md` | Unchanged, correctly: it is the extraction-artifact contract and says nothing about annotations | Yes |
| the published ledger is derived | `renderSuperseded` (`internal/le/rfc/sections.go`); `checkLedgerFresh` re-renders and reds on any difference | Yes |
| `./le rfc check` | 129 violations at HEAD, none of them a superseded finding, plus two stale-ledger lines caused by another session's uncommitted `rfc/short/rfc2661.md` and `rfc7911.md` edits | Foreign |
| `go test ./internal/le/rfc/...` | three failures, all caused by the same uncommitted edit: `rfc/short/rfc7911.md` loses a `{gap}` in the working tree, which moves the gap count from 514 to 513 and with it the published share and the fixture digest | Foreign |

## Core Insight

The vocabulary was the whole problem, twice over. The machinery had three
annotation kinds and every one of them said something about Ze's COVERAGE, so
there was no way to say something about the DOCUMENT, and the obvious fix,
adding a fourth kind, would have made obsolescence a route OUT of the population
every ratchet judges. Keeping it off `Requirement.annotation` is what makes AC-4
hold by construction rather than by care.

The second one is the reader. The hyphenated spelling is the corpus MAJORITY:
32 rows against 21 for the space today, over 199 summaries. A case-sensitive
space-only pattern therefore found the smaller half and reported a clean
population over the rest. Widening the pattern is not the lesson.
Refusing an unrecognised spelling is: a reader that skips what it does not
recognise cannot be trusted to have found anything.

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
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

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
