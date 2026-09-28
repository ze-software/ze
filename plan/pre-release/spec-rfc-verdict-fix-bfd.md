# Spec: rfc-verdict-fix-bfd

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` (the parent: its `audit-stamp` `mode rejudge` phase before any re-judge here, and its narrowing-audit output for the BFD group before any row edit, parent R-11) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec is child 2 of 8 of `plan/pre-release/spec-rfc-verdict-test-fix-pass.md`, the BFD group. The parent holds the method, the inventory and the cross-cutting work. This child carries the test fixes, row edits and D-8 code fixes for its packages and owned stems, and closes on its own.

**Goal (AC-C1).** The derived weak-or-wrong listing restricted to this child's packages and owned stems holds only the verdicts listed under "Blocked by" below, each named in the acceptance criteria of the spec that blocks it. Every other verdict is resolved as the parent's Goal states: a test proves the whole quoted sentence in both polarities and is re-judged `enforced` by an agent that did not write it, or the row is corrected, retired or re-attributed and re-judged against its new text.

| ID | Owner decision (parent, 2026-09-28) |
|----|------|
| P-1 | D-15 is standing approval for every tagged-unit edit and tag move: record `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before the edit, and carry the `RFC-approved:` trailer in the commit |
| P-2 | a recorded verdict is replaced only through `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`; an audit entry is never hand-deleted |
| P-3 | this child closes when every verdict in its scope is `enforced`, except those listed under "Blocked by"; each blocked verdict moves into the blocking spec's acceptance criteria |

Measured 2026-09-28 over `rfc/audit/*.json` with the filter below: **61 weak and 4 wrong, 65 in all, over 4 stems**. The parent's Split table counted a cross-group verdict in each child and predates the last two file moves, so some of its group figures differ. This count is exclusive (each verdict in exactly one child) and leaves out the parent's five cross-group verdicts and RFC7296-2.4-2.

### Packages it owns

Test packages: `internal/component/bfd/...`.

| Package | Weak + wrong (2026-09-28) | Of which wrong |
|---------|--------------|----------------|
| `internal/component/bfd/session` | 19 | 1 |
| `internal/component/bfd/engine` | 18 | 0 |
| `internal/component/bfd/auth` | 17 | 2 |
| `internal/component/bfd` | 8 | 0 |
| `internal/component/bfd/transport` | 4 | 1 |
| `internal/component/bfd/packet` | 3 | 0 |

A verdict whose units span two of these packages counts in each.

### Derived listing

The verdict inventory is DERIVED. No id list is committed here, because the audit files change as the work proceeds.

| Question | Derived by |
|----------|-----------|
| Every weak or wrong verdict in this child's scope (stem, id, verdict) | `jq -r --arg pk '^internal/component/bfd/' --arg own '^(rfc5882)$' --arg fo '^$' 'input_filename as $f \| ($f\|ltrimstr("rfc/audit/")\|rtrimstr(".json")) as $s \| .requirements \| to_entries[] \| select(.value.verdict=="weak" or .value.verdict=="wrong") \| select(.key\|IN("RFC1071-1-4","RFC4303-2.1-1","RFC5882-4.4-1","RFC905-x-3","RFC905-x-4")\|not) \| select((($s\|test($own)) or ([(.value.tests // {})\|keys[]\|test($pk)]\|any)) and ($s\|test($fo)\|not)) \| "\($s)\t\(.key)\t\(.value.verdict)"' rfc/audit/*.json` |
| What the arguments mean | `pk` matches a tagged test path in this child's packages; `own` adds this child's owned stems tagged from another child's packages; `fo` drops stems another child owns (`^$` matches none); the five ids are the parent's cross-group verdicts |

### Owned stems

One writer per ledger file: this child alone writes `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/short/<stem>.md`, `rfc/corrections/<stem>.md` and `rfc/extraction/<stem>.json` for these stems.

| Stem | Weak + wrong (2026-09-28) | Of which wrong |
|------|--------------|----------------|
| rfc5880 | 50 | 3 |
| rfc5881 | 12 | 1 |
| rfc5882 | 1 | 0 |
| rfc5883 | 2 | 0 |

Test files that carry tags of two owners. A child edits one only while the other holds no uncommitted change in it:

| File | Shared by |
|------|-----------|
| `internal/component/bgp/reactor/config_bfd_strict_test.go` | BGP (its package) and this child, which owns rfc5882 |
| `internal/plugins/ospf/rfc5882_shared_key_test.go` | OSPF (its package) and this child, which owns rfc5882 |

### Un-enrolled R-7 rows

Copied verbatim from the parent. A row leaves this table when its re-judged verdict is `enforced`, recorded here with the date, or in the stem's audit file if the stem is enrolled first.

None of this table's rows belong to this child.

### Split-needed rows

Copied verbatim from the parent. The parent's narrowing audit adds rows for this group, and this child takes them when that output is committed. "Row correction" marks a claim no sentence states.

| ID | Dropped obligation | Section |
|----|--------------------|---------|
| RFC5880-6.7.2-2 | key SHOULD also be allowed in hexadecimal form, for Keyed MD5 and SHA1 | §6.7.3, §6.7.4 |
| RFC5880-6.7.3-5 | the XmitAuthSeq SHOULD, for SHA1 | §6.7.4 |
| RFC5880-6.7.2-3 | Auth Key ID MUST be set to the ID of the current key, for MD5 and SHA1 (the SHA1 tags on this row belong there) | §6.7.3, §6.7.4 |
| RFC5880-6.7.3-8 | Sequence Number MUST be set to bfd.XmitAuthSeq, for SHA1 (the Simple Password claim was fabricated: row correction) | §6.7.4 |
| RFC5880-6.7.3-9 | Keyed SHA1 receive window | §6.7.4 |
| RFC5880-6.7.3-10 | Meticulous SHA1 receive window | §6.7.4 |
| RFC5880-6.7.2-4 | the Auth Type discard for Keyed MD5 and Keyed SHA1; the quote is the Simple Password case, while the row's parenthetical still cites §6.7.3 and §6.7.4 | §6.7.3, §6.7.4 |
| RFC5880-6.7.2-5 | the Auth Key ID discard for Keyed MD5 and Keyed SHA1; the quote says "configured password", §6.7.2 only, while the parenthetical still cites §6.7.3 and §6.7.4 | §6.7.3, §6.7.4 |
| RFC5880-6.7.2-6 | the Auth Len discard for Keyed MD5 (24) and Keyed SHA1 (28); the quote is the Simple Password rule, password length plus three | §6.7.3, §6.7.4 |
| RFC5882-4.2.1-1 | SHOULD signal lack of connectivity in the control protocol, or emulate a timeout | §4.2.2.1, §4.2.2.2 |
| RFC5882-10.1.3-2 | BFD authentication SHOULD be used for EBGP | §10.2 |

### Missing rows

None of this table's rows belong to this child.

### Row-quality corrections

| Kind | Rows | What is wrong | Correction owed |
|------|------|---------------|-----------------|
| id names another section | RFC5880-4.1-1 | the id names §4.1, but the quote and cite are the §6.8.7 transmit rule; no obligation is lost (the receive half is RFC5880-6.8.6-1), and an id is permanent | a dated correction saying why the id stands |

### Mistagged units

None of this table's rows belong to this child.

### Deferred items the parent's tables missed

None of this table's rows belong to this child.

### Narrowing audit rows (parent, 2026-09-28)

Copied from the parent's "Narrowing audit findings, 2026-09-28", where the process and the call meanings are. AC-C3 includes these rows.

| ID | Call | Obligation | Section |
|----|------|------------|---------|
| RFC5880-6.7.3-11 | dropped | "Otherwise (the hash does not match the Auth Key/Hash field), the received packet MUST be discarded." | §6.7.4 |
| RFC5883-5-2 | retire | No RFC sentence requires binding RX sockets per session type; quote duplicates RFC5883-5-1 port 4784 obligation. | none |

### Blocked by

Each id below was checked on 2026-09-28: a weak or wrong verdict in `rfc/audit/<stem>.json`, or a row of the R-7 table above. The blocking spec owns the producer fix (AC-C4), and the verdict moves into its acceptance criteria (P-3).

No verdict in this child's scope is blocked by another spec.

- No code-defect spec owns a BFD row: the BFD defects were fixed by `7757c3e955` (parent "Dropped as already fixed").

## Required Reading

### Architecture Docs
- [ ] `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` - the Method table, the Required Reading constraints, D-2 to D-15, and the source tables above
  → Constraint: every row stays a verbatim span of its cited section, at least 24 characters, never across two sections (`checkRowQuotes`); a span that must cross a section is split into two rows
  → Constraint: widening, narrowing or re-citing a row stales its verdict (`stale-requirement`) until it is re-judged in the same commit; a new id takes n above the HEAD^ high-water mark of its section (`checkIDAllocation`)
  → Constraint: a new gated row lands with both polarity tags or an annotation, a discrimination record for each new cover (a moved tag is a new cover), and an extraction site mapped to it or listed in `unsourced-ids`
  → Constraint: an upgrade from weak or wrong to `enforced` with unchanged units needs an `upgrade_reason` (`checkAuditFindings`), so author and judge land in one commit where they can
  → Constraint: editing any function in a test file shifts every verdict tagging that file; `./le rfc reseal` is corpus-wide and rewrites other children's audit files, so a reseal runs only when no other child holds uncommitted audit changes
  → Constraint: D-14: RFC5880-6.7.3-12 seeds bfd.RcvAuthSeq before the digest check by owner decision; this child MUST NOT reverse it, and RFC5880-6.8.1-13 stays a gap
  → Constraint: RFC5882-4.4-1 tags units in bfd/engine, bgp/reactor and ospf: it stays in the parent (AC-9), and this child does not re-judge it
  → Constraint: RFC5880-4.1-1: the id names Section 4.1 but quotes the Section 6.8.7 transmit rule; an id is permanent, so it gets a dated correction saying why it stands
  → Constraint: rows fixed by `7757c3e955` (RFC5880-6.7.3-9, 6.8.1-5, 6.8.3-1, 6.8.6-8, 6.8.7-5) still owe their split rows and verdicts here
- [ ] `docs/contributing/rfc-conformance-gates.md` - the discrimination record, the row quote, the refusals
- [ ] `ai/skills/ze-rfc-audit.md` - the four judgement questions, the verdict vocabulary, STRICTNESS, and the re-judge route
  → Decision: judge strictly: a floor assertion, a confounded buffer or a positive that proves only "no error" stays `weak`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/<stem>.md` for each owned stem above - the row text a verdict is judged against; the RFC text in `rfc/full/<stem>.txt` is the authority for every split and correction
  → Constraint: a claim that an obligation was dropped quotes the RFC sentence with its section (parent R-9)

**Key insights:**
- 65 verdicts over 6 packages and 4 stems, plus 12 rows copied from the parent's tables
- The listing is derived with the filter above, never copied; each stem's ledger files belong to one child
- A verdict whose producer another spec fixes is listed under "Blocked by" and leaves this child through P-3

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` - the parent: Split section, inventory tables, Method, Required Reading, AC-C1 to AC-C7
- [ ] `rfc/audit/rfc5880.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc5881.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc5883.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `internal/le/rfc/check_audit.go` - `checkAuditFindings` refuses a deleted weak or wrong verdict and an unchanged-units upgrade without `upgrade_reason`

**Behavior to preserve:**
- a commit of this child adds no `./le rfc check` violation of its own; a weak or wrong verdict is never deleted
- product behavior of every package in scope, except a D-8 fix of a verified defect, which carries its own failing-first test

**Behavior to change:**
- the tagged tests, rows, tags and verdicts named in this spec's tables and in the derived listing; D-8 producer fixes where a test exposes a verified defect

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc check`, reading `rfc/short/`, `rfc/audit/`, `rfc/discrimination/` and the `RFC requirement:` tags in test files
- `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`, typed by a judging agent

### Transformation Path
1. The author agent records `./le rfc approve unit` (P-1), then changes a tagged test, a row, a tag, or a producer (D-8)
2. `./le rfc reseal` re-stamps sibling verdicts the edit only shifted
3. `./le rfc discriminate-record` records the observed red under a break for each added or changed cover
4. A judging agent that did not author the test writes the verdict in a pending file under session scratch
5. `./le rfc audit-stamp ... mode rejudge` replaces the verdict in place with fresh fingerprints
6. `./le rfc check` compares verdicts, records and rows against HEAD^

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| test file to ledger | the `RFC requirement:` tag, fingerprinted by `stampFingerprints` | `./le rfc check` reads the re-judged verdict fresh |
| pending file to audit file | `audit-stamp` `mode rejudge` | the parent's `TestAuditStampRejudgeReplacesInPlaceAndReadsFresh` |

### Integration Points
- the parent's `mode rejudge` of `./le rfc audit-stamp` (P-2)
- the blocking specs' acceptance criteria, which receive the blocked verdicts (P-3)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | every verdict enters through `audit-stamp`, every record through `discriminate-record` |
| No duplicated functionality | Yes | the listing is the parent's filter restricted to this scope; no id list is committed |
| Registration over hardcoding | N-A | no feature is added; this child changes tests, rows and D-8 producers |

## Method

The parent's Method table governs every phase and is not copied here. The rules that bite this group are the constraints under Required Reading above, plus: one agent per package and never two on one test file; strict judgement; every clause in both polarities; a discrimination record for every tag added or changed; an independent judge for every re-judged verdict; a D-8 defect fixed failing-test-first, with interop where a peer exists.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | the parent's `mode rejudge` has landed before the first re-judge here | parent Implementation Steps 1 and 2 | a re-judge has no route but hand deletion, which P-2 bans | `./le rfc audit-stamp` help names `mode` | unvalidated |
| A-2 | the blocked-by list is complete for this scope | the parent's code-defect and overlap tables, each id checked in `rfc/audit/` or the R-7 table on 2026-09-28 | the child cannot reach AC-C1 | each package's author agent names any verdict it cannot move without a producer fix another spec owns | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a test fix exposes a defect that grows past the package | a code change outside the package | D-8: fix it; where impossible, back to the owner |
| R-2 | another child or spec commits while this child holds uncommitted hunks in a shared test file, or runs a reseal | a commit diff shows another owner's ids | the shared-file rule above; reseal and interop `revert` runs are serialized between children |
| R-C1 | the nine RFC5880 split rows all land in §6.7.3 and §6.7.4, so their new ids collide if two phases allocate in one section | `checkIDAllocation` refuses a commit | one phase adds every RFC5880 split row, in one commit |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | test-only changes break nothing an operator sees; a D-8 fix changes protocol behavior and carries its own tests and interop |
| How is it reverted? | one commit per package phase |
| Who else touches this path? | the parent, the children named in the shared-file table, the blocking specs, `spec-rfc-evidence-strength-1` and `-2` (the same discrimination files) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` | → | `checkAuditFindings` over this child's stems | the derived listing above holds only the "Blocked by" ids, and `TestCheckAuditRatchetSeesTipCommit` (`internal/le/rfc/check_audit_baseline_test.go`) stays green |

## Acceptance Criteria

Inherited from the parent (AC-C1 to AC-C7), restated for the BFD group.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-C1 | the derived listing above | holds only the "Blocked by" ids, each named in its blocking spec's acceptance criteria |
| AC-C2 | every verdict this child moves to `enforced` | its tagged units carry a discrimination record, and an agent other than the test's author judged it |
| AC-C3 | every row of this child in the split-needed and missing-rows tables, including the rows the parent's narrowing audit adds for this group (the "Narrowing audit rows (parent, 2026-09-28)" table) | the dropped obligation is a row of its own, or a dated correction says why not, and the new row carries a verdict |
| AC-C4 | every spec in the parent's code-defect list | this child declares none of their defects and does not fix them; a test whose producer one of them changes waits for, or lands with, that spec |
| AC-C5 | `./le rfc check` after each commit of this child | no violation the commit added, and no stale verdict in this child's stems |
| AC-C6 | every row and unit of this child in the row-quality and mistagged-unit tables | corrected, merged or re-tagged, or a dated correction says why it stands; a changed row or tag is re-judged |
| AC-C7 | every row of this child in "Un-enrolled R-7 rows" that no spec blocks | resolved as the parent's Goal states and re-judged `enforced` by an agent that did not write the test, recorded in that table with the date or in the stem's audit file |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| every unit tagged by a listed verdict | `internal/component/bfd/session/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bfd/engine/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bfd/auth/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bfd/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bfd/transport/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bfd/packet/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| each numeric field a quoted sentence bounds | the RFC's range | the RFC's last valid value | tested where the sentence states a lower bound | tested where the sentence states an upper bound |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| an existing tagged `.ci` test in scope, changed only where its verdict demands | `test/` | the operator-visible behavior the quoted sentence names | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| named when a D-8 fix is found | `test/interop/scenarios/` | BFD interop scenario for any D-8 fix (FRR bfdd); none is planned before a defect is found | the fixed behavior against another implementation | |

## Files to Modify

- the `_test.go` and `.ci` files of the packages above that carry the listed tags
- `rfc/short/<stem>.md`, `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/corrections/<stem>.md`, `rfc/extraction/<stem>.json` for the owned stems only
- a producer under the packages above when a test exposes a verified defect (D-8), named in the phase that finds it
- `docs/features/rfc-status.md`, the row of each owned stem whose support claim changes
- the acceptance criteria of each blocking spec, which receive its blocked verdicts (P-3)

## Files to Create
- none planned; a split or missing row is a new row in an existing `rfc/short/<stem>.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| YANG validation constraints | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| YANG custom validators | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| CLI commands/flags | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| CLI grammar (keyword before value) | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Editor autocomplete | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Functional test for new RPC/API | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Pipe completeness | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Env var registration | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Doctor check for runtime dependencies | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Prometheus counters/metrics | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| BGP family surface (new SAFI / capability / attribute) | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | this child changes tests, rows and verdicts, not this surface |
| 2 | Config syntax changed? | No | this child changes tests, rows and verdicts, not this surface |
| 3 | CLI command added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 4 | API/RPC added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 5 | Plugin added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 6 | Has a user guide page? | No | this child changes tests, rows and verdicts, not this surface |
| 7 | Wire format changed? | No | tests and rows only; a D-8 fix that changes the wire names `docs/architecture/wire/*.md` in its phase |
| 8 | Plugin SDK/protocol changed? | No | this child changes tests, rows and verdicts, not this surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/<stem>.md` for each owned stem a phase changes, and its `docs/features/rfc-status.md` row, with source anchors; a new `wrong` verdict discloses non-support there first |
| 10 | Test infrastructure changed? | No | this child changes tests, rows and verdicts, not this surface |
| 11 | Affects daemon comparison? | No | this child changes tests, rows and verdicts, not this surface |
| 12 | Internal architecture changed? | No | this child changes tests, rows and verdicts, not this surface |
| 13 | Route metadata keys added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 14 | Prometheus counters added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | this child changes tests, rows and verdicts, not this surface |
| 16 | Any changed source file referenced by existing doc source anchors? | No | only test and ledger files change; a D-8 producer fix runs `./le spec citation anchors` in its phase |
| 17 | Existing docs show config/CLI/API examples for this area? | No | this child changes tests, rows and verdicts, not this surface |

## Implementation Steps

**Procedure P, for every package phase.** One author agent per package, never two on one test file. It reads the listing restricted to the package, records `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before each tagged-unit edit (P-1), makes each clause of the quoted sentence asserted in both polarities or corrects the row, and fixes a verified defect under D-8. It runs the scoped package test under `./le job run`, then `./le rfc discriminate-record` for each added or changed cover. An independent judge that wrote none of the tests writes the verdicts in a pending file under session scratch and stamps them with `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`. `./le rfc reseal` runs when the edit shifted sibling verdicts, serialized between children. Each package lands in one commit through `./le commit create` with the `RFC-approved:` trailer, author and judge together where they can. Verify: the listing filtered to the package holds only blocked ids, and `./le rfc check` adds no violation.

1. **Phase: Wiring** - confirm the parent's `mode rejudge` has landed (A-1), reproduce the derived listing and its counts, and move each "Blocked by" verdict into its blocking spec's acceptance criteria (P-3)
   - Tests: the Wiring Test row
   - Files: the blocking specs' Acceptance Criteria tables
   - Verify: the listing count equals the Task figure, or the difference is explained by commits since 2026-09-28
2. **Phase: `internal/component/bfd/session`** - 19 verdicts, 1 wrong; procedure P
3. **Phase: `internal/component/bfd/engine`** - 18 verdicts, 0 wrong; procedure P
4. **Phase: `internal/component/bfd/auth`** - 17 verdicts, 2 wrong; procedure P
5. **Phase: `internal/component/bfd`** - 8 verdicts, 0 wrong; procedure P
6. **Phase: `internal/component/bfd/transport`** - 4 verdicts, 1 wrong; procedure P
7. **Phase: `internal/component/bfd/packet`** - 3 verdicts, 0 wrong; procedure P
8. **Phase: split and missing rows** - after the parent commits its narrowing output for this group (parent R-11): each row of the split-needed and missing-rows tables becomes a row of its own with both polarity tags, a record and a verdict, or a dated correction says why not (AC-C3)
   - Verify: `./le rfc check` accepts the new ids, extraction sites and records
9. **Phase: row quality and mistagged units** - each row of those tables corrected, merged or re-tagged, or a dated correction says why it stands; each changed row or tag re-judged (AC-C6)
   - Verify: `./le rfc check` reads no stale verdict in the owned stems
10. **Phase: closure check** - the derived listing holds only "Blocked by" ids (AC-C1), each named in its blocking spec's acceptance criteria

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-C row demonstrated; the listing holds only blocked ids |
| Correctness | every `enforced` verdict names an identifier in its unit, has both polarities or `{single-polarity}`, and a judge other than the author |
| Rule: no weakened test | no assertion removed or loosened to reach `enforced` (`ai/rules/completion.md`) |
| Rule: one writer per ledger file | no commit touches a stem another child owns |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the listing reduced to blocked ids | the filter under "Derived listing" |
| a record for every changed cover | `./le rfc check` adds no missing-record violation |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a negative test that feeds malformed input asserts the refusal, not only the absence of a crash |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | re-read the RFC sentence; a verified defect is a D-8 fix in this child unless a blocking spec owns it |
| A verdict cannot move without another spec's producer fix | add it to "Blocked by" and to that spec's acceptance criteria (P-3) |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the owner |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Scope by package, with each stem's ledger at one owner | scope by stem | the parent's split: test files are per package, ledger files per stem |
| A verdict of an owned stem whose units sit only in another child's package belongs to the stem owner | give it to the package owner | the ledger file has one writer; the shared test file is edited one child at a time |

## Known Limitations

- verdicts under "Blocked by" leave this child through P-3 and close with their blocking spec
- the parent's cross-group verdicts and RFC7296-2.4-2 are not in this child's scope

## RFC Documentation (Scope: protocol)

A D-8 fix adds `// RFC NNNN Section X.Y: "<quoted requirement>"` above the enforcing code, per `ai/rules/rfc-compliance.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-C1..AC-C7 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for the D-8 protocol fixes (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** tests + rows + verdicts + records + D-8 code + edited spec
- [ ] **Commit B:** `remove plan/pre-release/spec-rfc-verdict-fix-bfd.md` only, in the same `./le commit create` script
