# Spec: rfc-verdict-fix-bfd

| Field | Value |
|-------|-------|
| Status | verification |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` (the parent: its `audit-stamp` `mode rejudge` phase before any re-judge here, and its narrowing-audit output for the BFD group before any row edit, parent R-11) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec is child 2 of 8 of `plan/pre-release/spec-rfc-verdict-test-fix-pass.md`, the BFD group. The parent holds the method, the inventory and the cross-cutting work. This child carries the test fixes, row edits and D-8 code fixes for its packages and owned stems, and closes on its own.

**Goal (AC-C1).** The derived weak-or-wrong listing restricted to this child's packages and owned stems holds only the verdicts listed under "Blocked by" below, each named in the acceptance criteria of the spec that blocks it. Every other verdict is resolved as the parent's Goal states: a test proves the whole quoted sentence in both polarities and is re-judged `enforced` by an agent that did not write it, or the row is corrected, retired or re-attributed and re-judged against its new text.

| ID | Owner decision (parent, 2026-09-28) |
|----|------|
| P-1 | D-15 is standing approval for every tagged-unit edit and tag move: record `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before the edit, and carry the `RFC-approved:` trailer in the commit |
| P-2 | a recorded verdict is replaced only through `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`; an audit entry is never hand-deleted |
| P-3 | this child closes when every verdict in its scope is `enforced`, except those listed under "Blocked by"; each blocked verdict moves into the blocking spec's acceptance criteria |
| P-4 | Owner ruling 2026-09-30 (Thomas). RFC5880-6.1-3: "we can not see the peer", so no refusal path exists. RFC5880-6.8.6-15: accepted as having no genuine negative. Route for both: move the HEAD negative tag to the neighbouring row it proves (D-15), then mark the row `{single-polarity: positive}` citing this ruling, and re-judge. `./le rfc check`'s coverage ratchet refused both markers ("no longer proven -- the negative test(s) that covered it at HEAD are gone"), so both rows keep their HEAD text and their HEAD negative tags, and the neighbouring tags were added beside them: `TestRFC5880PassiveRoleSilentUntilReception` also tags RFC5880-6.1-1 negative, `TestRFC5880NoPeriodicTransmitWhileAdminDown` also tags RFC5880-6.8.16-3 negative. Judged on the positive (2026-09-30): 6.1-3 enforced, its HEAD negative supplementary; 6.8.6-15 stays weak, because its positives never drive D=0 with both ends Up, nor D=1 with the local end not Up and the remote Up; 6.8.16-3 weak, because a MAY Ze does not take cannot be violated |

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
| Every weak or wrong verdict in this child's scope (stem, id, verdict) | `jq -r --arg pk '^internal/component/bfd/' --arg own '^(rfc5880|rfc5881|rfc5882|rfc5883)$' --arg fo '^$' 'input_filename as $f \| ($f\|ltrimstr("rfc/audit/")\|rtrimstr(".json")) as $s \| .requirements \| to_entries[] \| select(.value.verdict=="weak" or .value.verdict=="wrong") \| select(.key\|IN("RFC1071-1-4","RFC4303-2.1-1","RFC5882-4.4-1","RFC905-x-3","RFC905-x-4")\|not) \| select((($s\|test($own)) or ([(.value.tests // {})\|keys[]\|test($pk)]\|any)) and ($s\|test($fo)\|not)) \| "\($s)\t\(.key)\t\(.value.verdict)"' rfc/audit/*.json` |
| What the arguments mean | `pk` matches a tagged test path in this child's packages; `own` adds this child's owned stems tagged from another child's packages; `fo` drops stems another child owns (`^$` matches none); the five ids are the parent's cross-group verdicts |

**Correction, 2026-09-30:** `own` named only rfc5882, so an untagged weak row in rfc5880 (RFC5880-6.7.4-9, whose `tests` map is empty) matched neither `own` nor `pk` and never showed. `own` now names every stem this child owns. A corpus-wide query of weak/wrong verdicts with an empty `tests` map returned only this row and RFC7296-2.4-2, which `plan/immediate/spec-ike-dpd-demand-driven.md` AC-2 owns.

**Wiring, 2026-09-28, at `cb4801b4cc`:** the filter returns 61 weak and 4 wrong, 65 in all, over 4 stems. This equals the Task figure, so `fefe38b68c` (it re-judged only the five cross-group verdicts, which the filter drops) and `cb4801b4cc` changed no count here. `./le rfc audit-stamp` accepts `mode rejudge` (A-1). No verdict is blocked, so no blocking spec received one.

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
| `bfd-frr`, `bfd-simple-password-bird`, `bgp-bfd-strict-frr`, `ospf-bfd-frr`, `ospfv3-bfd-frr` | `test/interop/scenarios/` | FRR bfdd, BIRD | the D-8 fixes (session state, AdminDown transmission, egress pinning, off-subnet refusal) against another implementation | PASS 2026-10-11 at `1a0294116d` (Goal Validation) |

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

## Progress, 2026-10-11

Written from `plan/handover/rfc-verdict-test-fix-pass/` (QUEUE.md, HANDOFF.md) and git history, each claim re-checked against HEAD `c0d31c3660` on 2026-10-11. Implementation is finished; the spec waits for its independent `/ze-review`. It is not closed.

## Implementation Summary

### What Was Implemented
- 25 commits since 2026-09-28 touch `rfc/audit/rfc588[0-3].json` (`git log --since=2026-09-28 -- rfc/audit/rfc5880.json rfc/audit/rfc5881.json rfc/audit/rfc5882.json rfc/audit/rfc5883.json`), from `246dbae78c` (auth verdict fixes) to `1248bf9d3d` (P-4 owner ruling) and the AC-C2 record sweep `66249a3391`.
- Verdict state at HEAD: rfc5880 100 enforced and 2 unimplemented, rfc5881 15 enforced, rfc5882 5 enforced and 1 weak (RFC5882-4.4-1, the parent's, see below), rfc5883 5 enforced.
- Split rows (AC-C3): `7faf23a2b7` added the RFC5880 §6.7.3/§6.7.4 split rows and the RFC5882 SHOULD rows (RFC5882-4.2.2.1-1/-2, 4.2.2.2-1, 10.2-1); RFC5883-5-2 retired in `b86ff37e11`.
- Row quality (AC-C6): RFC5880-4.1-1 carries its dated correction in `rfc/corrections/rfc5880.md`.

### Bugs Found/Fixed (D-8, each with production code)
| Commit | Fix |
|--------|-----|
| `ac27529c11`, `65f29eba17` | session/engine RFC 5880 fixes; session revival and sharing |
| `0bde29b2db` | multi-hop BFD sessions shared across clients |
| `c48b1585fd` | RFC5881-6-1 egress pinning (netns) |
| `637aa9c186`, `ff834590e3` | RFC5881-6-2 off-subnet refusal, IPv4 link-local |
| `380aa13f83`, `a0ad32312b` | AdminDown handling; AdminDown sent for 3x Detection Time (RFC5880-6.8.16-2/-3) |
| `460ae9faf5`, `dfb07c0c7d` | a join with differing authentication refused; EBGP peer with unauthenticated BFD profile warned (RFC5882-10.2-1, RFC5880-6.7.4-9) |

### Documentation Updates
- `docs/architecture/bfd-client.md` and the two OSPF BFD interop scenario configs, `1c21de3153`.

### Deviations from Plan
- RFC5882-4.4-1 was never this child's (constraint under Required Reading, parent AC-9). Owner decision 2026-10-11: it is transferred to AC-24 of `plan/immediate/spec-bgp-live-receive-binding-and-bfd-keep-session.md` (Phase 4, step 1), which owns the `internal/component/bfd/engine/engine.go` fix. Recorded in the parent's AC-9 and AC-11. This child holds no row for it.

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | the first `own` filter named only rfc5882 | an untagged weak rfc5880 row (RFC5880-6.7.4-9) was invisible to it | Correction 2026-09-30 in the Task | filter widened; row fixed in `dfb07c0c7d` |

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-C1 | Done | the "Derived listing" filter run on 2026-10-11 at HEAD returns no row | "Blocked by" is empty, so nothing moves to another spec |
| AC-C2 | Done | 84 verdicts moved to `enforced` since `174862502f` (the child specs' creation); every tagged unit of each carries a record in `rfc/discrimination/rfc588[0-3].json` (script over audit and record files, 2026-10-11: 0 unrecorded) | sweep `66249a3391`; judge independence per commit is in QUEUE.md and is the reviewer's to confirm |
| AC-C3 | Done | split rows `7faf23a2b7`; RFC5880-6.7.3-11 is one of them; RFC5883-5-2 retired `b86ff37e11` | |
| AC-C4 | Done | no code-defect spec owns a BFD row ("Blocked by") | |
| AC-C5 | Done | `./le rfc check` 2026-10-11: no violation in rfc5880..5883 | |
| AC-C6 | Done | RFC5880-4.1-1 dated correction in `rfc/corrections/rfc5880.md` | |
| AC-C7 | N-A | no R-7 row belongs to this child | |

### Assumptions
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `./le rfc audit-stamp` takes `mode rejudge` (parent AC-1) and every BFD re-judge went through it |
| A-2 | confirmed | AC-C1 listing is empty |

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| AC-C1: the derived listing holds only "Blocked by" ids | ledger query | filter in "Derived listing" returns 0 rows at `c0d31c3660` |
| every resolved verdict proven in both polarities and judged by a non-author | discrimination records + audit | 0 unrecorded tagged units over the 84 moved verdicts; `./le rfc check` clean for the four stems |
| D-8 fixes work against another implementation | interop | 2026-10-11, all five PASS, each run alone in the Ze-kernel QEMU guest (`./le test qemu docker-lab lab 'test integration interop' env 'INTEROP_SCENARIO=<name>'`, guest kernel 7.2.9, `le setup docker-kernel check` verdict pass) over a `git archive` export of `1a0294116d`, which contains `460ae9faf5` and `1c21de3153`. Logs under `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/interop/`: `bfd-frr.log` line 81, and `bfd-simple-password-bird.log`, `bgp-bfd-strict-frr.log`, `ospf-bfd-frr.log`, `ospfv3-bfd-frr.log` line 83, each `interop: 1 passed, 0 failed` followed by `QEMU VM: PASS`. `bfd-frr` passed for the first time. colima's daemon refuses every Ze lab (AppArmor `ze-kernel-probe` not applicable, MPLS absent on its 6.8 kernel: `bfd-frr.colima-refused.log`), so the guest is the macOS route `docs/architecture/testing/interop.md` names |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| RFC5882-4.4-1 shared-session `joined` key | parent verdict, transferred by the owner 2026-10-11 | `plan/immediate/spec-bgp-live-receive-binding-and-bfd-keep-session.md` AC-24 |
| interop for the D-8 fixes | DONE 2026-10-11: all five scenarios PASS in the QEMU guest at `1a0294116d`; evidence in Goal Validation. Kept here only so the Review Gate reader sees it was owed | nothing owed |
| `./le verify worktree` | needs a quiet machine; never green for this pass | this spec, before closure |

## Review round 1 fixes, 2026-10-11

Findings from `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/review-rfc-verdict-bfd.md` (0 BLOCKER, 5 ISSUE). The Review Gate below stays open for an independent re-review.

| # | Finding | Fixed by | Evidence |
|---|---------|----------|----------|
| ISSUE 1 | RFC5880-6.7.2-2 hex password SHOULD unrecorded | `d5d22258ef` | `{gap}` annotation and `unimplemented` verdict in `rfc/short/rfc5880.md` and `rfc/audit/rfc5880.json`; extraction site `6.7.3:6` reason corrected |
| ISSUE 2 | `sameAuth` Type and Meticulous comparisons untested | the commit that carries this row | `TestRFC5882AuthJoinDifferentAuthRefused` gained three cases on both arms: `md5` (Keyed MD5 against Keyed SHA1, Type the only difference), `meticulous` (Keyed SHA1 then Meticulous Keyed SHA1, owner-named), and the first profile's auth with only Meticulous inverted, sent under no profile because `authTypeFromEnum` ties Meticulous to its own wire Auth Type. Overlay red, `go test -overlay`: removing `a.Type != b.Type` reddens only `different_auth_type/{exact_key,shared_join}`; removing `a.Meticulous != b.Meticulous` reddens only `different_meticulous_flag/{exact_key,shared_join}`; green with neither. Negative record re-written by `./le rfc discriminate-record` (revert of `joinAuthCheck`), owner approval in `tmp/commit-rfc-approved-d1987d95.md` |
| ISSUE 3 | the two RFC 5882 Section 10.2 warnings named no fix | `3e6e76a6de` | `peer_bfd.go` messages name the `auth` block and the authenticated-profile route; `docs/guide/bfd.md` and the discrimination record follow |
| ISSUE 4 | no `.ci` drove the warning or the join refusal through ze | `e64da01079` | `test/plugin/bgp-bfd-auth-ebgp-warn.ci`, `test/plugin/bgp-bfd-auth-join-refused.ci` |
| ISSUE 5 | `./le verify worktree` not met | owed | whole-tree gate, run by the main thread or a fresh agent on a quiet tree |

Owed after ISSUE 2: the test change stales the RFC5882-10.2-1 audit verdict (`./le rfc check`: "STALE audit verdict ... func-scoped", reseal refuses it). A judge who did not author the change re-reads rfc5882 with `/ze-rfc-audit` and stamps with `./le rfc audit-stamp mode rejudge`.

## Review round 4 fixes, 2026-10-11

Findings from `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/review-rfc-verdict-bfd-r4.md` (0 BLOCKER, 1 ISSUE, 5 NOTE). Owner decision (Thomas, 2026-10-11, "Apply live, in keep-session (Recommended)"): making a profile edit reach a live session is built in `plan/immediate/spec-bgp-live-receive-binding-and-bfd-keep-session.md` Phase 4; this spec makes the guide and the refusal true today (route B of the ISSUE). Evidence in `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/red-bfd-r5/`.

| # | Finding | Fixed by | Evidence |
|---|---------|----------|----------|
| ISSUE 1 | fixes the refusal named never took effect; "Editing live" promised in-place propagation | the commit that carries this row | `sessionEntry.authFrom` records the profile the auth came from (create and every `replaceAuth`); `authMismatchCase` gains the edited-profile branch (same profile name, different auth: release from every client); every refusal ends with `authReleaseHow` (how each client releases) and `authRetryHow` (when a refused client asks again); case 4 names the session's source profile instead of "the same bfd profile". `docs/guide/bfd.md` "Editing live" rewritten with a release/ask-again table per client; `docs/architecture/bfd.md` names `authFrom` and corrects the shared-revive sentence |
| NOTE 1 | stale `entry.profile` on shared revive | journal row | `plan/journal/field-carries-two-meanings.md`, 2026-10-11; the refusal does not read `profile` |
| NOTE 2 | OSPF fix named no leaf | same commit | `ospfDisableHow`: `bfd { enabled false; }` or delete the block (`ze-ospf-conf.yang` default false) |
| NOTE 3 | case 3 omitted a pinned session with no profile | same commit | case 3 text and guide table name the pinned single-hop-session or multi-hop-session |
| NOTE 4 | guide stated "the shared session runs no authentication" as a fact | same commit | reworded as the condition under which OSPF can share |
| NOTE 5 | `interface ""` in the refusal | same commit | `refusalInterface` omits it |

Tests: `TestAuthJoinRefusalNamesApplicableFix` (untagged) gained the pinned-no-profile case and two edited-profile cases (applied through `runtimeState.applyConfig`), plus release, retry and no-empty-interface assertions. Overlay reds: dropping the edited branch reddens exactly the two edited cases (`red-nobranch.log`); always printing the interface reddens all seven (`red-emptyiface.log`); the HEAD producer reddens all seven (`red.log`). `./le --name bfdr4 test bgp plugin --pattern bgp-bfd-auth`: 105 and 106 PASS (`ci-green.log`). Race with the gate tags over `bfd/...`, `static/...`, `test/fixture/...`: ok (`race.log`). No RFC-tagged test asserts the changed text.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | |
| `./le spec review check` | not run |
| Rounds | 5 so far (scratch `review-rfc-verdict-bfd.md`, `-r2` to `-r5`); round 6 owed |
| Owner authorised | round 6: Thomas, 2026-10-11, "Authorise round 6" |
| Reviewer lenses used | |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
