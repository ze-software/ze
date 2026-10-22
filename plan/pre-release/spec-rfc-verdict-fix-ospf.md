# Spec: rfc-verdict-fix-ospf

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` (the parent: its `audit-stamp` `mode rejudge` phase before any re-judge here, and its narrowing-audit output for the OSPF group before any row edit, parent R-11) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec is child 3 of 8 of `plan/pre-release/spec-rfc-verdict-test-fix-pass.md`, the OSPF group. The parent holds the method, the inventory and the cross-cutting work. This child carries the test fixes, row edits and D-8 code fixes for its packages and owned stems, and closes on its own.

**Goal (AC-C1).** The derived weak-or-wrong listing restricted to this child's packages and owned stems holds only the verdicts listed under "Blocked by" below, each named in the acceptance criteria of the spec that blocks it. Every other verdict is resolved as the parent's Goal states: a test proves the whole quoted sentence in both polarities and is re-judged `enforced` by an agent that did not write it, or the row is corrected, retired or re-attributed and re-judged against its new text.

| ID | Owner decision (parent, 2026-09-28) |
|----|------|
| P-1 | D-15 is standing approval for every tagged-unit edit and tag move: record `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before the edit, and carry the `RFC-approved:` trailer in the commit |
| P-2 | a recorded verdict is replaced only through `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`; an audit entry is never hand-deleted |
| P-3 | this child closes when every verdict in its scope is `enforced`, except those listed under "Blocked by"; each blocked verdict moves into the blocking spec's acceptance criteria |

Measured 2026-09-28 over `rfc/audit/*.json` with the filter below: **106 weak and 9 wrong, 115 in all, over 21 stems**. The parent's Split table counted a cross-group verdict in each child and predates the last two file moves, so some of its group figures differ. This count is exclusive (each verdict in exactly one child) and leaves out the parent's five cross-group verdicts and RFC7296-2.4-2.

### Packages it owns

Test packages: `internal/plugins/ospf/...`.

| Package | Weak + wrong (2026-09-28) | Of which wrong |
|---------|--------------|----------------|
| `internal/plugins/ospf` | 60 | 7 |
| `internal/plugins/ospf/lsdb` | 19 | 0 |
| `internal/plugins/ospf/packet` | 14 | 0 |
| `internal/plugins/ospf/spf` | 11 | 1 |
| `internal/plugins/ospf/sr` | 5 | 0 |
| `internal/plugins/ospf/iface` | 4 | 0 |
| `internal/plugins/ospf/neighbor` | 4 | 0 |
| `internal/plugins/ospf/transport` | 3 | 1 |
| `internal/plugins/ospf/types` | 3 | 0 |
| `internal/core/probe` | 1 | 1 |

A verdict whose units span two of these packages counts in each.

### Derived listing

The verdict inventory is DERIVED. No id list is committed here, because the audit files change as the work proceeds.

| Question | Derived by |
|----------|-----------|
| Every weak or wrong verdict in this child's scope (stem, id, verdict) | `jq -r --arg pk '^internal/plugins/ospf/' --arg own '^(rfc1071)$' --arg fo '^(rfc4301\|rfc4303\|rfc905\|rfc5882)$' 'input_filename as $f \| ($f\|ltrimstr("rfc/audit/")\|rtrimstr(".json")) as $s \| .requirements \| to_entries[] \| select(.value.verdict=="weak" or .value.verdict=="wrong") \| select(.key\|IN("RFC1071-1-4","RFC4303-2.1-1","RFC5882-4.4-1","RFC905-x-3","RFC905-x-4")\|not) \| select((($s\|test($own)) or ([(.value.tests // {})\|keys[]\|test($pk)]\|any)) and ($s\|test($fo)\|not)) \| "\($s)\t\(.key)\t\(.value.verdict)"' rfc/audit/*.json` |
| What the arguments mean | `pk` matches a tagged test path in this child's packages; `own` adds this child's owned stems tagged from another child's packages; `fo` drops stems another child owns (`^$` matches none); the five ids are the parent's cross-group verdicts |

**Wiring, 2026-09-28, at `cb4801b4cc`:** the filter returns 106 weak and 9 wrong, 115 in all, over 21 stems. This equals the Task figure, so `fefe38b68c` (it re-judged only the five cross-group verdicts, which the filter drops) and `cb4801b4cc` changed no count here. `./le rfc audit-stamp` accepts `mode rejudge` (A-1). The "Blocked by" verdict is now `spec-ospf-nssa-translator-reachability` AC-3.

### Owned stems

One writer per ledger file: this child alone writes `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/short/<stem>.md`, `rfc/corrections/<stem>.md` and `rfc/extraction/<stem>.json` for these stems.

| Stem | Weak + wrong (2026-09-28) | Of which wrong |
|------|--------------|----------------|
| rfc1071 | 4 | 1 |
| rfc2328 | 32 | 1 |
| rfc3101 | 10 | 2 |
| rfc3623 | 5 | 0 |
| rfc4302 | 5 | 2 |
| rfc4552 | 3 | 0 |
| rfc4577 | 2 | 0 |
| rfc5187 | 4 | 0 |
| rfc5250 | 6 | 0 |
| rfc5286 | 1 | 0 |
| rfc5340 | 4 | 0 |
| rfc5392 | 2 | 0 |
| rfc5443 | 5 | 1 |
| rfc5709 | 4 | 0 |
| rfc5838 | 4 | 2 |
| rfc6138 | 1 | 0 |
| rfc7474 | 6 | 0 |
| rfc7684 | 3 | 0 |
| rfc7770 | 1 | 0 |
| rfc8665 | 9 | 0 |
| rfc8666 | 4 | 0 |
| rfc8362 | 0 in `rfc/audit/` | out of audit scope: its one R-7 row (RFC8362-2-1) is recorded in the R-7 table |
| rfc3630 | 0 in `rfc/audit/` | row-quality RFC3630-1-1; no weak or wrong verdict |
| rfc7166 | 0 in `rfc/audit/` | narrowing audit row RFC7166-4.6-4; no audit file |

Stems tagged in this child's packages that another child owns:

| Stem | Owner | Tags here |
|------|-------|-----------|
| rfc4301, rfc4303 | IKE/EAP | the tags in `ospf/config_ipsec_test.go` and `ospf/ipsec_install_test.go`: RFC4301-4.2-1, RFC4303-1-1, 2-1 and 2.1-2 are IKE/EAP's |
| rfc905 | routing | the tags in `ospf/packet/checksum_test.go`; RFC905-x-3 and x-4 are parent verdicts |
| rfc5882 | BFD | the tag in `ospf/rfc5882_shared_key_test.go` (RFC5882-4.4-1, a parent verdict) |

Test files that carry tags of two owners. A child edits one only while the other holds no uncommitted change in it:

| File | Shared by |
|------|-----------|
| `internal/plugins/ospf/config_ipsec_test.go`, `internal/plugins/ospf/ipsec_install_test.go` | this child (rfc4302, rfc4552 tags) and IKE/EAP (rfc4301, rfc4303) |
| `internal/plugins/ospf/packet/checksum_test.go` | this child (rfc1071, rfc2328) and routing (rfc905) |
| `internal/core/probe/icmp_test.go` | services (its package, rfc792) and this child, which owns rfc1071 (RFC1071-1-5) |

### Un-enrolled R-7 rows

Copied verbatim from the parent. A row leaves this table when its re-judged verdict is `enforced`, recorded here with the date, or in the stem's audit file if the stem is enrolled first.

| ID | Verdict | Unit | What the test fails to prove |
|----|---------|------|------------------------------|
| RFC8362-2-1 | weak | `TestRFC8362ExtendedLSAsSetUBitOnTheWire`, `TestRFC8362BaseLSAsKeepUBitClearOnTheWire` (`internal/plugins/ospf/rfc8362_test.go`) | the U-bit on the E-Inter-Area-Prefix-LSA, the third Extended LSA Ze originates |

- Re-judged 2026-10-21, `enforced` (positive), by a judge that wrote neither test. The positive now also originates an E-Inter-Area-Prefix-LSA through `v6OriginateInterAreaSR` and reads 0xA023 on the wire (`90dce367a3`); E-Router and E-Intra-Area-Prefix read 0xA021 and 0xA029. The negative tag came off `TestRFC8362BaseLSAsKeepUBitClearOnTheWire`, which checks RFC 5340 A.4.2.1, a different rule; the row carries `{single-polarity: positive}` because the U-bit is part of every Extended LS Type constant (`50044040c5`). Only those three Extended types are originated.

### Split-needed rows

Copied verbatim from the parent. The parent's narrowing audit adds rows for this group, and this child takes them when that output is committed. "Row correction" marks a claim no sentence states.

| ID | Dropped obligation | Section |
|----|--------------------|---------|
| RFC3101-2.2-1 | the aggregate's forwarding address is set to 0.0.0.0 | §3.2 step (3), §2.3 |
| RFC5392-3.2.1-4 | the Remote AS Number sub-TLV is REQUIRED in a Link TLV advertising an inter-AS TE link | §3.3.1 |
| RFC8665-3.1-7 | area-scoped flooding is REQUIRED for the SID/Label Range TLV and the SRLB TLV | §3.2, §3.3 |
| RFC5709-3.2-4 | key storage SHOULD persist across a warm or cold restart | §3.2 |
| RFC7770-2.1-1 | OSPFv3 twin: Instance ID 0 SHOULD carry the Router Informational Capabilities TLV | §2.2 |
| RFC2328-8.2-3 | authenticate every received packet; accept a non-Hello packet only from an active neighbor. Neither is consecutive with the AuType sentence the row quotes | §D.4, §8.2 later paragraph |
| RFC2328-10.5-1 | declare bidirectional communication only when the router is listed in the neighbor's Hello | §10.5 later paragraph, §9.5 |
| RFC2328-10.6-1 | process DD packets in sequence and, as slave, reply to each; the quote is the §10.8 duplicate resend only | §10.6 |
| RFC2328-12.2-1 | a network-LSA is looked up on its Link State ID alone | §16.1 |
| RFC2328-12.4-1 | flush an AS-external-LSA for an unreachable destination, and an LSA no longer advertisable to an area; the quote is the §12.4.3 summary-LSA case | §12.4.4, §16.7 |
| RFC2328-12.4.3-1 | condense summaries as the configured area address ranges require | §12.4.3 |
| RFC2328-13-3 | drop an LS Acknowledgment from a neighbor below Exchange; the quote is the §13 LS Update case | §13.7 |
| RFC2328-13.3-3 | on non-broadcast networks, delayed LS Acknowledgments are sent as separate unicasts | §13.5 |
| RFC2328-13.4-1 | re-originate or flush by premature aging a received self-originated LSA; the quote is the detection only | §13.4 later paragraph, §14.1 |
| RFC2328-16.2-2 | the same skips for AS-external-LSAs; the quote is the §16.2 summary-LSA steps | §16.4 |
| RFC2328-D.3-2 | the sequence number is non-decreasing and reset to zero when the neighbor goes Down; Figure 18 separates it from the quoted sentence, so one span cannot carry both | §D.3 |

### Missing rows

None of this table's rows belong to this child.

### Row-quality corrections

| Kind | Rows | What is wrong | Correction owed |
|------|------|---------------|-----------------|
| text after the cite, or a wrapped hyphen | RFC3630-1-1 (prose after the section cite); DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1 ("strict- mode"), RFC7684-5-1 ("sub- TLV") | the prose is not RFC text, and the hyphen split is the RFC's line wrap carried into the quote | move the prose out of the quote; join the hyphen |

- The row starting `text after the cite, or a wrapped hyphen`: this child owns RFC3630-1-1 and RFC7684-5-1 only; the other ids go to the child that owns their stem.

### Mistagged units

None of this table's rows belong to this child.

### Deferred items the parent's tables missed

None of this table's rows belong to this child.

### Narrowing audit rows (parent, 2026-09-28)

Copied from the parent's "Narrowing audit findings, 2026-09-28", where the process and the call meanings are. AC-C3 includes these rows.

| ID | Call | Obligation | Section |
|----|------|------------|---------|
| RFC2328-13-6 | dropped | "The best route to the destination described by the summary-LSA must be recalculated (see Section 16.5)." | §13.2 |
| RFC2328-9.5.1-1 | dropped | "The interface state must be at least Waiting for any Hello Packets to be sent out the NBMA interface." | §9.5.1 |
| RFC3101-2.3-1 | dropped | "The Type field in the LSA header is 7." | §2.3 |
| RFC3101-2.4-4 | dropped | "A Type-7 default LSA may be installed by NSSA border routers if and only if its P-bit is set." | §2.4 |
| RFC3101-3.1-2 | dropped | "If there exists another border router in this list whose router-LSA has bit Nt set or who has a higher router ID, then its NSSATranslatorState is disabled." | §3.1 |
| RFC5250-5-1 | dropped | "If no entries exist for the ASBR (i.e., the ASBR is unreachable), the router MUST do nothing with this LSA." | §5 |
| RFC5709-3.3-1 | dropped | "Apad is the hexadecimal value 0x878FE1F3 repeated (L/4) times." | §3.3 |
| RFC7166-4.6-4 | dropped | "If the two do not match, the packet MUST be discarded, and an error event SHOULD be logged." | §4.6 |
| RFC8665-5-7 | dropped | "This MUST be done regardless of whether the next-hop router contributes to the best path to the prefix." | §5 |
| RFC8666-6-13 | dropped | "If both the NP-Flag and E-Flag are set, then: Any upstream neighbor of the Prefix-SID originator MUST replace the Prefix-SID with an Explicit NULL label." | §6 |
| RFC8666-6-8 | dropped | "This MUST be done regardless of whether the next-hop router contributes to the best path to the prefix." | §6 |

### Blocked by

Each id below was checked on 2026-09-28: a weak or wrong verdict in `rfc/audit/<stem>.json`, or a row of the R-7 table above. The blocking spec owns the producer fix (AC-C4), and the verdict moves into its acceptance criteria (P-3).

| ID | Verdict | Blocking spec |
|----|---------|---------------|
| RFC3101-3.2-2 | weak | `spec-ospf-nssa-translator-reachability` |

- The other OSPF defects were fixed by `b12697744d` (parent "Dropped as already fixed").

## Required Reading

### Architecture Docs
- [ ] `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` - the Method table, the Required Reading constraints, D-2 to D-15, and the source tables above
  → Constraint: every row stays a verbatim span of its cited section, at least 24 characters, never across two sections (`checkRowQuotes`); a span that must cross a section is split into two rows
  → Constraint: widening, narrowing or re-citing a row stales its verdict (`stale-requirement`) until it is re-judged in the same commit; a new id takes n above the HEAD^ high-water mark of its section (`checkIDAllocation`)
  → Constraint: a new gated row lands with both polarity tags or an annotation, a discrimination record for each new cover (a moved tag is a new cover), and an extraction site mapped to it or listed in `unsourced-ids`
  → Constraint: an upgrade from weak or wrong to `enforced` with unchanged units needs an `upgrade_reason` (`checkAuditFindings`), so author and judge land in one commit where they can
  → Constraint: editing any function in a test file shifts every verdict tagging that file; `./le rfc reseal` is corpus-wide and rewrites other children's audit files, so a reseal runs only when no other child holds uncommitted audit changes
  → Constraint: D-12: RFC 3101 translator equivalence is same mask, metric and non-zero forwarding address
  → Constraint: rfc8362 is out of audit scope, so `audit-stamp` refuses it: RFC8362-2-1 is re-judged and recorded in this spec's R-7 table with the date
  → Constraint: RFC1071-1-4 (with services) and RFC4303-2.1-1, RFC905-x-3, RFC905-x-4 tag OSPF files but stay in the parent (AC-9)
  → Constraint: RFC1071-1-5 is this child's (it owns rfc1071), and its units sit in `internal/core/probe/icmp_test.go`, a services file: edited only while services holds no uncommitted change there
- [ ] `docs/contributing/rfc-conformance-gates.md` - the discrimination record, the row quote, the refusals
- [ ] `ai/skills/ze-rfc-audit.md` - the four judgement questions, the verdict vocabulary, STRICTNESS, and the re-judge route
  → Decision: judge strictly: a floor assertion, a confounded buffer or a positive that proves only "no error" stays `weak`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/<stem>.md` for each owned stem above - the row text a verdict is judged against; the RFC text in `rfc/full/<stem>.txt` is the authority for every split and correction
  → Constraint: a claim that an obligation was dropped quotes the RFC sentence with its section (parent R-9)

**Key insights:**
- 115 verdicts over 10 packages and 23 stems, plus 18 rows copied from the parent's tables
- The listing is derived with the filter above, never copied; each stem's ledger files belong to one child
- A verdict whose producer another spec fixes is listed under "Blocked by" and leaves this child through P-3

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` - the parent: Split section, inventory tables, Method, Required Reading, AC-C1 to AC-C7
- [ ] `rfc/audit/rfc2328.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc3101.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc8665.json` - weak and wrong verdicts with their `tests` maps and notes
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
| R-C1 | the eleven RFC2328 split rows are the largest single-stem split set in the pass | a row lands before the parent's narrowing output for rfc2328 | the row phase waits for the OSPF group output (parent R-11) |

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

Inherited from the parent (AC-C1 to AC-C7), restated for the OSPF group.

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
| every unit tagged by a listed verdict | `internal/plugins/ospf/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/ospf/lsdb/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/ospf/packet/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/ospf/spf/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/ospf/sr/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/ospf/iface/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/ospf/neighbor/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/ospf/transport/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/ospf/types/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/core/probe/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |

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
| named when a D-8 fix is found | `test/interop/scenarios/` | OSPF interop scenario for any D-8 fix (FRR ospfd); none is planned before a defect is found | the fixed behavior against another implementation | |

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
2. **Phase: `internal/plugins/ospf`** - 60 verdicts, 7 wrong; procedure P
3. **Phase: `internal/plugins/ospf/lsdb`** - 19 verdicts, 0 wrong; procedure P
4. **Phase: `internal/plugins/ospf/packet`** - 14 verdicts, 0 wrong; procedure P
5. **Phase: `internal/plugins/ospf/spf`** - 11 verdicts, 1 wrong; procedure P
6. **Phase: `internal/plugins/ospf/sr`** - 5 verdicts, 0 wrong; procedure P
7. **Phase: `internal/plugins/ospf/iface`** - 4 verdicts, 0 wrong; procedure P
8. **Phase: `internal/plugins/ospf/neighbor`** - 4 verdicts, 0 wrong; procedure P
9. **Phase: `internal/plugins/ospf/transport`** - 3 verdicts, 1 wrong; procedure P
10. **Phase: `internal/plugins/ospf/types`** - 3 verdicts, 0 wrong; procedure P
11. **Phase: `internal/core/probe`** - 1 verdicts, 1 wrong; procedure P
12. **Phase: split and missing rows** - after the parent commits its narrowing output for this group (parent R-11): each row of the split-needed and missing-rows tables becomes a row of its own with both polarity tags, a record and a verdict, or a dated correction says why not (AC-C3)
   - Verify: `./le rfc check` accepts the new ids, extraction sites and records
13. **Phase: row quality and mistagged units** - each row of those tables corrected, merged or re-tagged, or a dated correction says why it stands; each changed row or tag re-judged (AC-C6)
   - Verify: `./le rfc check` reads no stale verdict in the owned stems
14. **Phase: un-enrolled R-7 rows** - each row not under "Blocked by" resolved and re-judged by an agent that did not write the test, recorded in the R-7 table above with the date (AC-C7)
15. **Phase: closure check** - the derived listing holds only "Blocked by" ids (AC-C1), each named in its blocking spec's acceptance criteria

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
- [ ] **Commit B:** `remove plan/pre-release/spec-rfc-verdict-fix-ospf.md` only, in the same `./le commit create` script

## Progress, 2026-10-11

Written from `plan/handover/rfc-verdict-test-fix-pass/` (QUEUE.md, HANDOFF.md) and git history, each claim re-checked against HEAD on 2026-10-11. **Not ready for review: AC-C2 is open** (below). Status stays `in-progress`.

## Implementation Summary

### What Was Implemented
- Verdict work in continuations 1 to 19, from `50094a5e6d` (2026-09-29) to `7488f60bc2` (c19, 2026-10-02), then `c797d66a9b`, `55581e8304`, `30b572216a` and `821aed75e3` (c18, rfc1071 and rfc2328 rows).
- 130 verdicts in scope are newly `enforced` since `174862502f`, 115 of them moved from weak or wrong.

### Bugs Found/Fixed (D-8, each with production code, all wire-visible)
| Commit | Fix |
|--------|-----|
| `50094a5e6d` | NSSA: `translateNSSAV6` forwarding address handling |
| `033720151f` | NSSA translator list (R31) |
| `7335b407ef` | RFC 2328 Section 8.2: received packets accepted by source network (`acceptsArea`) |
| `bec87a7d44` | RFC 5709 last-key handling; RFC 4552 11-3 interface open and registration (R32) |
| `3c69504a4b`, `99046264fd`, `79ce425e26` | RFC 3623 GR; RFC 5250 Type-11 LSA accepted only from a reachable ASBR; Type-11 originator E-bit |
| `2d206c22b5`, `790472c47f` | RFC 2328 13.3 flood age; RFC 7770 RI capabilities follow the flooding scope |
| `7708c6b240` | RFC 7684 N-Flag across areas; RFC 8666 SID/Label sub-TLV |
| `7488f60bc2` | RFC 2328 Section 13: a bad LS checksum discards that LSA and processing continues with the next |
| `c797d66a9b` | an opaque LSA whose body is not 4-octet aligned is discarded |
| `55581e8304`, `30b572216a` | RFC 8665: a zero-length SR-Algorithm TLV refused; PHP for mapping-server SIDs (Section 5) |
| `50044040c5` | RFC 3623 Section 3.1: on broadcast, NBMA and P2MP the GR helper identifies X by the Grace-LSA interface address (`graceRestarterV4`), as FRR and BIRD do; it used the Advertising Router |
| `50044040c5` | RFC 7684 Section 2.1 / RFC 5250 Section 3: an NSSA-external Extended Prefix maps to area scope, not AS scope (`extPrefixScope`; no origination path reaches it yet) |
| `19de202493` | RFC 8665 Section 5 / RFC 8666 Section 6: the next hop's own E/NP/M flags apply whenever it advertised the SID, off the best path too (`sr_install.go::forwarding`, `sr_malformed.go::resolve`) |
| `19de202493` | RFC 2328 Sections 13.4 and 14.1: an unwanted received self-originated LSA is flushed at MaxAge with the received sequence, and a withdrawn one is not re-originated (`lsdb/origination.go`) |
| `19de202493` | RFC 2328 Section D.5.3: the AuType 2 replay sequence is kept per neighbour, not per packet type (`auth_keystore.go::replaySlotFor`) |

### Deviations from Plan
- None recorded.

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| omission | the AC-C2 record sweep was never run for this child, unlike bfd (`66249a3391`) and ike-eap (`6b968f093b`) | 64 moved verdicts carry tagged units with no record | script over audit and record files, 2026-10-11 | closed: sweep 2026-10-15, judged 2026-10-21 |
| wrong assumption | the 2026-10-15 sweep chose each record's producer by what the unit reached, so a red under a whole-body revert was read as proof of the clause | the judges flagged records in 33 of the 64 requirement ids (rfc2328 8, group B 8, group C 17) that named setup writers, fixtures, decoders, wrappers or a show-path helper; two units hid product defects (GR helper identity, NSSA-external scope) | three non-author judges, 2026-10-21 | records re-targeted and verdicts re-judged in `50044040c5` |

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-C1 | Done | the "Derived listing" filter at `19de202493`, 2026-10-21, returns one row, RFC3101-3.2-2 (weak), as it did on 2026-10-11 | owned by AC-3 of `plan/immediate/spec-ospf-nssa-translator-reachability.md` |
| AC-C2 | Done 2026-10-21 | RFC5443-2-2 positive `TestLDPSyncMaxMetricValue` now drives `effectiveP2PCost` and has a record (`90dce367a3`). Three judges that wrote none of the tests read all 118 sweep records: every break is a whole-body revert, so many proved only reach (setup writers, fixtures, the test's decoder, wrappers, a show-path helper). Authors re-targeted those records, strengthened the units, moved or removed tags that claimed neighbouring rules, and fixed two product defects; every staled verdict was re-judged `enforced` by a non-author (`50044040c5`). `./le rfc check` 2026-10-21: no finding in any owned stem. Earlier state, 2026-10-15: | Sweep run 2026-10-15. Owed set re-derived (same filter as the 2026-10-11 listing, intersected with the `unproven` list of `./le rfc discriminate stem <stem>`): 119 tag sites over 64 verdicts, 19 stems (the 118 audit keys plus one rfc3101 tag the audit does not key). **118 recorded**, all on the revert route with an observed red that names the unit, all host-runnable (no unit needs the Linux guest), in 19 per-stem commits `4742079904`..`7384785d1d`. No break stayed green. **1 unrecorded:** RFC5443-2-2 positive, `internal/plugins/ospf/ldp_sync_test.go::TestLDPSyncMaxMetricValue` asserts only that the constant `ospflsdb.LSInfinity` equals 0xFFFF; it calls no function, so no revert break exists, and `declaration-only` does not apply (`lsdb/origination.go` declares functions). Finding: the unit checks a constant, not the substitution; its negative and RFC5443-2-1 siblings are recorded on `effectiveP2PCost`. **Judge owed:** a non-author must read the producers, because a revert proves reachability, not the clause; producers to look at first: RFC5187-3.1-1/3.2-1 negative on `gr_nvs.go::writeRestartFact` (a setup writer), RFC5340-A.3.1-2 on `dispatcher.go::dispatch`, RFC2328-16.2-2 negative on `spf/external.go::ComputeExternal`, RFC2328-C.3-1 and RFC5340-C.3-2 on `config.go::validateConfigAF`, RFC5443-3-1 positive on `ldpSyncWithholdTransit` | scripts and per-attempt logs in `tmp/session/2026-10-14-3be8cd06-0062-4950-8b30-09ebec04cbaf/scratch/` (`acc2_*.py`, `acc2_sweep.sh`, `sweep*.tsv`, `sweeplogs/`). Two attempts first went red before the break from a shared-cache or vendor-tree hiccup and recorded green on retry |
| AC-C3 | Done 2026-10-21 | review round 1 found most split and narrowing rows still unresolved; `19de202493` gives every one a verbatim row of its own (RFC2328-8.2-4, 8.2-5, 9.5.1-2, 10.5-2, 10.6-2, 10.6-3, 12.4.3-2, 12.4.4-1, 13.2-1, 13.4-2, 13.4-3, 13.5-3, 13.7-1, 16.7-1, D.3-4, D.5.3-1; RFC3101-2.3-4, 2.4-7; RFC5250-5-4; RFC5392-3.3.1-2; RFC5709-3.3-5; RFC8665-3.2-15, 3.3-10, 5-16; RFC8666-6-18), or a dated correction (`rfc/corrections/rfc2328.md`: 12.2-1, D.3-2), or shows the row that already carries it (RFC2328-16.2-2 by 16.4-2, RFC3101-3.1-2 by 3.1-4); RFC8666-6-13 widened to carry its NP+E condition (round 2); all judged `enforced` by non-authors. First pass: RFC3101-3.2-3 (`{feature-declined}`, not-applicable), RFC5709-3.2-6 (single-polarity, enforced, `0de9b98e6c`), RFC7770-2.2-2 (enforced), RFC7166-4.6-7 and 4.6-4 (`{gap}`, unimplemented) (`b75f0355c6`); plus RFC2328-16.4-2 (enforced, `50044040c5`). Earlier, 2026-10-11 lookup: every split-needed and narrowing row of this child is enforced, except four rows present in `rfc/short/` with no verdict and no annotation: RFC3101-2.2-1 (MAY), RFC5709-3.2-4, RFC7770-2.1-1, RFC7166-4.6-4 (SHOULD; rfc7166 has no audit file) | |
| AC-C4 | Done | RFC3101-3.2-2 sits in its owning spec's AC | |
| AC-C5 | Done 2026-10-21 | `50044040c5` left one stale record (RFC7474-3-1 positive, round 1), re-taken in `19de202493`; `./le rfc check` after `19de202493` shows no violation in any owned stem | 2026-10-11: no violation in an OSPF stem (the three left were BGP's) |
| AC-C6 | Done 2026-10-21 | RFC3630-1-1 trailing note removed and RFC7684-5-1 hyphen joined, each with a dated correction (`rfc/corrections/rfc3630.md`, `rfc7684.md`), both re-judged `enforced` (`19de202493`) | |
| AC-C7 | Done 2026-10-21 | RFC8362-2-1 re-judged `enforced` by a non-author, recorded with the date under the R-7 table | rfc8362 is out of audit scope |

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| AC-C1: the derived listing holds only "Blocked by" ids | ledger query | 1 row, RFC3101-3.2-2, in its owning spec's AC |
| every resolved verdict proven and judged by a non-author | discrimination records + audit | MET 2026-10-21: every owed record exists and names the producer of its clause; every re-judged verdict stamped by a judge that wrote none of the tests (`90dce367a3`, `50044040c5`); `./le discriminate stem` shows no stale record in any owned stem |
| the six 2026-10-21 D-8 fixes each have a red that a revert produces | revert in a `git archive` export of `df2e303665`, rebuild, run, restore | 2026-10-22, unit level, observed red then green (logs summarized here; scratch only): GR helper identity (`graceRestarterV4` call replaced by the Advertising Router): `TestRFC3623HelperIdentifiesRestarterByInterfaceAddress`, `...RefusesUnmatchedInterfaceAddress` ("1 helper sessions, want 0"), `...FlushExitsNamedRestarter` red. AuType 2 replay (`replaySlotFor` keyed by packet type): `TestOSPFAuthReplayPerNeighborAuType2` red (`expected "replay"`); `TestOSPFAuthReplay` does NOT discriminate. NSSA-external scope (`extPrefixScope` maps type 7 to AS): `TestExtPrefixScopeSelection` red ("extPrefixScope(7) = as, want area"). Interop red unreachable for these three, FRR 10.3.1 read: `ospf_gr.c::ospf_gr_lsa_new` sets the Advertising Router to `oi->ospf->router_id`, the Hello Router ID, and `ospf_gr_lsa_body_set` the TLV to `oi->address`, the Hello source, so both codes pick the same X; `ospf_auth.c::ospf_auth_make` keeps one strictly increasing `oi->crypt_seqnum` per interface across every type and key, so FRR never sends what the per-type slot wrongly admits; `ospf_ext.c::ospf_ext_schedule_prefix_index` originates only `OSPF_PATH_INTRA_AREA` and no Ze production caller passes route type 7 (`ext_prefix.go` builds intra, inter, AS-external only). Interop-level, written and NOT yet run: `ospf-sr-anycast-nexthop-frr` (SR flags off the best path; red expected as `16100 as to 17100` under the `forwarding` revert), `ospf-self-lsa-flush-frr` (13.4 MaxAge branch of `handleSelfReceived`; the sequence-bump half is invisible to FRR, which purges a MaxAge flush at N or N+1, `ospf_lsa.c::ospf_lsa_more_recent`), `ospf-accept-lifetime-last-key-frr` (RFC 5709 3.2 last key) |
| D-8 fixes work against another implementation | interop | 2026-10-22: nine scenario configs `ze config validate` refused (`ospf-gr-frr`, `ospfv3-gr-frr`, both `*-gr-fib-retention`, four `*-debug-*`, `ospfv3-sr-frr`) fixed in `df2e303665`; those scenarios could never have passed before. QUEUE.md (2026-10-02): `ospf-p2p-frr` and `ospf-broadcast-frr` PASS after c19 `7488f60bc2`; no log is committed. `ospf-opaque-frr`, `ospf-te-frr` and `ospf-ext-prefix-link-frr` fail identically at the `410c67f1d4` baseline on the Mac host (journal `assertion-reads-state-the-scheduler-owns.md`). `ospf-bfd-frr` and `ospfv3-bfd-frr` PASS, see the bfd child |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| RFC3101-3.2-2 | producer defect | `plan/immediate/spec-ospf-nssa-translator-reachability.md` AC-3 |
| interop for the D-8 fixes | the c19 runs cover only the fixes up to `7488f60bc2` and have no log; three scenarios fail at baseline on the Mac host. Owed, each with `INTEROP_SCENARIO=<name> ./le test integration interop` and its log kept: `ospf-p2p-frr`, `ospf-broadcast-frr` (8.2 source network, 13.3 flood age, per-LSA discard); `ospf-opaque-frr`, `ospf-ext-prefix-link-frr` (opaque alignment `c797d66a9b`, Type-11 `99046264fd`/`79ce425e26`), on a Linux host since they fail at baseline here; `ospf-te-frr`, `ospf-ri-frr` (RI scope `790472c47f`, `7708c6b240`); `ospf-sr-frr` (`55581e8304`, `30b572216a`); `ospf-auth-frr`, `ospf-accept-lifetime-frr` (RFC 5709 `bec87a7d44`); `ospf-gr-frr` (RFC 3623); `ospf-stub-nssa-frr`, `ospfv3-nssa-redist-frr` (NSSA `50094a5e6d`, `033720151f`). The 2026-10-21 fixes add: `ospf-gr-frr` and `ospf-broadcast-frr` (GR helper identity, `50044040c5`), `ospf-sr-frr` (next-hop flags off the best path, `19de202493`), `ospf-auth-frr` (AuType 2 per-neighbour replay), `ospf-broadcast-frr` (§13.4 self-LSA flush). BLOCKED on the Mac 2026-10-21: the plain Docker route refuses on Colima's Ubuntu kernel (no MPLS, L2TP, XFRM_MIGRATE) and names `./le test qemu docker-lab lab 'test integration interop' env INTEROP_SCENARIO=<name>`; that route failed three of three runs in the guest at `cross-compile le into test/interop/le-linux: context deadline exceeded: the compiler printed nothing` (`ospf-p2p-frr`, `ospf-gr-frr`; `ospf-broadcast-frr` lost the guest SSH port to another VM). That is the open row of 2026-10-09 in `plan/journal/bound-too-small-for-its-own-burst.md` (`labBuildTimeout`, 5 min, no override). Run them on a Linux host, or after that bound is fixed | this spec, before closure |
| interop runs, 2026-10-22 | BLOCKED: the queued run of the twelve owed scenarios from export `tmp/zoh` (`df2e303665`) waited from 00:42 behind other sessions' `le verify worktree` and `le verify current fast`, and at 01:46 the disk had 3 GB free, below the 15 GB floor. Still owed, in order: greens for `ospf-opaque-frr`, `ospf-accept-lifetime-last-key-frr`, `ospf-gr-frr`, `ospf-gr-helper-broadcast-frr`, `ospf-auth-frr`, `ospf-accept-lifetime-frr`, `ospf-sr-frr`, `ospf-ext-prefix-link-frr`, `ospf-te-frr`, `ospf-ri-frr`, `ospf-stub-nssa-frr`, `ospfv3-nssa-redist-frr`, `ospf-sr-anycast-nexthop-frr`, `ospf-self-lsa-flush-frr`; then reds under the reverts for the last-key, SR anycast and self-flush scenarios (patches named in the handover) | this spec, before closure |
| `./le verify worktree` | the machine ran other sessions' verify runs all afternoon; not run for this spec | this spec, before closure |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc-verdict-fix-ospf-0b4a8e87-9c06-4f85-90a2-0445cb843906.md` (verdict findings, 135 files, round 2) |
| `./le spec review check` | BLOCKED 2026-10-21 (round 2): verdict is 'findings', not clean |
| Rounds | 2 |
| Reviewer lenses used | round 1: wiring, removed-behavior and test-rewrite audit, logic and lock order, RFC text, ledger (`./le rfc check`, `./le commit audit base 90dce367a3^`, tag-set diff). Round 2 (scope: the fixes in `19de202493`): round-1 re-check, row and correction walk against `rfc/full/`, product-fix logic (SR next-hop flags, self-LSA flush, AuType 2 replay slot, GR flush), test-rewrite audit, ledger (`./le rfc check`, `./le commit audit base 19de202493^`, derived listing), scoped `go test` over six ospf packages |

### Open findings (round 2, 2026-10-21, reviewer authored none of the change)
| # | Severity | Finding | Location |
|---|----------|---------|----------|
| 1 | BLOCKER | AC-C3: narrowing row RFC8666-6-13 (call: dropped) is unresolved. The row still omits the condition "If both the NP-Flag and E-Flag are set, then:" (`rfc/full/rfc8666.txt`:415), which its twin RFC8665-5-12 carries; no own row and no dated correction (`rfc/corrections/rfc8666.md` does not exist) | `rfc/short/rfc8666.md` RFC8666-6-13 |
| 2 | NOTE | record: the Implementation Audit is stale against `19de202493`. AC-C3 lists six rows, AC-C6 says "reviewer to confirm", AC-C5 names the RFC7474-3-1 stale record HEAD no longer reports, AC-C1 cites 2026-10-11 (the listing at HEAD returns only RFC3101-3.2-2). Supersedes round-1 #8 | this spec, Implementation Audit |
| 3 | NOTE | RFC7684-5-1 still carries implementation prose after its cite ("-- bound-checked decode ..."); the table owed only the hyphen for this row, and 98 rows in `rfc/short/` carry a ") -- " note, so this is a corpus convention | `rfc/short/rfc7684.md` |
| 4 | NOTE | round-1 #7 stands and extends: every record of the `19de202493` product fixes (RFC8665-5-16, RFC8666-6-18, RFC2328-13.4-2, 13.4-3, D.5.3-1, RFC3623-A-4 flush) is a whole-body panic break. Each unit, read from source, asserts the outcome the old code fails, but the record stores reach | `rfc/discrimination/rfc8665.json`, `rfc8666.json`, `rfc2328.json`, `rfc3623.json` |
| 5 | NOTE | untracked probe `TestZZProbeACC3` (t.Logf only, no assertion) runs with every ospf package test and must not ride into a commit | `internal/plugins/ospf/zz_probe_acc3_test.go` |
| 6 | NOTE | sibling `srTILFAResolver.PrefixSIDLabel` still matches only `rs.Originator` and ignores the new `Also` list; behaviour is unchanged from before the fix and a node SID is per router, so no defect is shown | `internal/plugins/ospf/sr_tilfa.go` |

Pending, not findings: the interop runs under Work Not Done, `./le verify worktree`, and `./le repo check` (not run by this reviewer).

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| R1-1 | BLOCKER | stale RFC7474-3-1 record: `./le rfc check` at HEAD (2026-10-21) reports 119 violations, none in an OSPF stem | `rfc/discrimination/rfc7474.json` | `19de202493`, verified round 2 |
| R1-2 | BLOCKER | AC-C3 rows with no own row or correction: 25 new rows, each with an enforced verdict and both polarities; corrections for RFC2328-12.2-1 and D.3-2 checked against `rfc/full/rfc2328.txt`. RFC8666-6-13 remains (open #1) | `rfc/short/`, `rfc/corrections/rfc2328.md` | `19de202493`, verified round 2 |
| R1-3 | BLOCKER | AC-C6: RFC3630-1-1 ends at its cite and RFC7684-5-1 reads "sub-TLV", both with dated corrections | `rfc/short/rfc3630.md`, `rfc7684.md` | `19de202493`, verified round 2 |
| R1-4 | ISSUE | `TestRFC3623HelperFlushExitsNamedRestarter` delivers a Withdrawn Grace-LSA through `graceOnReceive`; B's session ends and A's stays | `rfc3623_grace_helper_identity_test.go` | `19de202493`, verified round 2 |
| R1-5 | NOTE | the `graceRestarterV4` doc comment and the `docs/guide/ospf.md` helper paragraph say an unmatched shared-media flush is ignored, as FRR does | `gr_helper.go`, `docs/guide/ospf.md` | `19de202493`, verified round 2 |
| R1-6 | NOTE | identity and flush tests drive broadcast, NBMA and point-to-multipoint | `rfc3623_grace_helper_identity_test.go` | `19de202493`, verified round 2 |
