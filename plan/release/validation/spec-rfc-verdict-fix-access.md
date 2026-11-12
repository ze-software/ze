# Spec: rfc-verdict-fix-access

| Field | Value |
|-------|-------|
| Status | blocked |
| Priority | P2 |
| Release reason | Required evidence for shipped behavior or truthful release support claims; historical work and proof debts remain distinct. |
| Scope | tooling |
| Depends | `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` (the parent: its `audit-stamp` `mode rejudge` phase before any re-judge here, and its narrowing-audit output for the access group before any row edit, parent R-11) |
| Phase | 13/13 — D-8 existing-scenario acceptance and closure |
| Handoff | `plan/handover/2026-11-10-close-open-specs.md`, active Access checkpoint 2026-11-12 |
| Blocker | Guest compilation: foreign `internal/plugins/fib/vpp/routesource.go:32` imports missing vendor package `go.fd.io/govpp/binapi/fib`; all five pending cases stop before assertions. Scoped lint: foreign `internal/plugins/fib/vpp/mpls.go` changed during planning after host; exit 2, no plan. Both belong to active `spec-fib-depth.md` owner `f5ef1144-6c05-40fc-8506-68a94ded2a6f`; preserve its work. |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Blocker

Native commit preparation found two owned error-reporting sites and produced
no script. The duplicate literal `tc` verifier registration in
`register_verify_linux.go` now panics as a programmer BUG: `RegisterVerifier`
fails only for duplicate names during startup. CoS `executeCommand` now gives
an unknown command an operator refusal naming that command, the declared
command boundary and `ze show class-of-service` recovery. Independent bounded
source review approves both classifications; valid registrations and command
dispatch are unchanged. Main observed all nine focused race roots PASS without
skips in `scratch/job-access-final-error-surface-controls-8c7f5c5b.log`. Refresh
the historical native artifact's two affected source pins before closure.
This checkpoint adds no fourth whole-source pass.

All source findings are repaired and independently source-reviewed CLEAN at
round 3. Main observed native review artifact/check PASS over 240 carriers at
the completed source checkpoint, including the byte-identically renamed tests.
Later foreign diagnostic additions are excluded; the owned patch retains
reviewed SHA `eec1496845df93f13dd7c189a18bb2ddb14a42567ffc3947709b160efa1fd5e6`.
Main observed 393 affected
race roots PASS with zero skips, CoS without L2TP PASS and the current named
build PASS. Seventeen native proof renewals completed in
`scratch/job-access-final-seventeen-rfc-proofs-d82b2fea.log`; eight semantic
rejudgments and the mechanical shifts are applied. The final regenerated
thirteen-stem reconciliation `scratch/access-final-owned-rfc-lines.json` is
empty: zero owned findings. This does not claim a clean global RFC check.

Keep the earlier `01-ppp-ipv4` and `03-ze-lac-xl2tpd-lns` acceptance passes.
The final retry `scratch/job-access-final-five-ze-guest-retry-8670e830.log`
boots Ze/KVM, passes the kernel check and reaches Docker readiness, then all
five cases stop at the missing FIB vendor import before assertions. Each
reports zero scenarios; lab-exit is 1 and the VM stops with no leftovers.
The final bounded lint attempt also stops before a plan, on changing foreign
MPLS input, in `scratch/job-access-scoped-lint-final-checkpoint-476d68e0.log`.
No lint or guest acceptance pass is inferred from these attempts.

To unblock, let the FIB owner complete its import/vendor and dependency edits,
then rebuild in main and rerun exactly `04-radius-acct-attrs`,
`01-pppoe-chap-ipv4`, `02-ze-ac-pppd-client`, `radius-admin-eap-freeradius`
and `radius-coa-disconnect-freeradius` under the Ze Docker-host VM. The last
case verifies the current repair against actual kernel shaping. Rerun the
same 13-directory scoped lint once its dependencies settle, validate the five
Dockerfile-specific allowlists through native builds, then complete remaining
documentation/citation/planned-file checks and the closing commit workflow.
Do not repeat completed proofs merely to refresh timestamps. All execution
remains main-only; no closure or push is claimed.

## Task

This spec is child 6 of 8 of `plan/release/validation/spec-rfc-verdict-test-fix-pass.md`, the access group. The parent holds the method, the inventory and the cross-cutting work. This child carries the test fixes, row edits and D-8 code fixes for its packages and owned stems, and closes on its own.

**Goal (AC-C1).** The derived weak-or-wrong listing restricted to this child's packages and owned stems holds only the verdicts listed under "Blocked by" below, each named in the acceptance criteria of the spec that blocks it. Every other verdict is resolved as the parent's Goal states: a test proves the whole quoted sentence in both polarities and is re-judged `enforced` by an agent that did not write it, or the row is corrected, retired or re-attributed and re-judged against its new text.

| ID | Owner decision (parent, 2026-09-28) |
|----|------|
| P-1 | D-15 is standing approval for every tagged-unit edit and tag move: record `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before the edit, and carry the `RFC-approved:` trailer in the commit |
| P-2 | a recorded verdict is replaced only through `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`; an audit entry is never hand-deleted |
| P-3 | this child closes when every verdict in its scope is `enforced`, except those listed under "Blocked by"; each blocked verdict moves into the blocking spec's acceptance criteria |

Measured 2026-09-28 over `rfc/audit/*.json` with the filter below: **104 weak and 7 wrong, 111 in all, over 13 stems**. The parent's Split table counted a cross-group verdict in each child and predates the last two file moves, so some of its group figures differ. This count is exclusive (each verdict in exactly one child) and leaves out the parent's five cross-group verdicts and RFC7296-2.4-2.

### Packages it owns

Test packages: L2TP, PPP, PPPoE, RADIUS and TACACS: `internal/component/l2tp/...` (with `ppp`, `pppoe`, `pppoeclient`, `plugins/authradius`, `plugins/authlocal`), `internal/component/radius`, `internal/component/tacacs`, `test/l2tp`.

| Package | Weak + wrong (2026-09-28) | Of which wrong |
|---------|--------------|----------------|
| `internal/component/l2tp/ppp` | 31 | 4 |
| `internal/component/tacacs` | 18 | 0 |
| `internal/component/l2tp` | 17 | 0 |
| `internal/component/radius` | 16 | 1 |
| `internal/component/l2tp/plugins/authradius` | 15 | 0 |
| `internal/component/l2tp/pppoe` | 12 | 2 |
| `internal/component/l2tp/pppoeclient` | 8 | 0 |
| `test/l2tp` | 6 | 0 |
| `internal/component/l2tp/plugins/authlocal` | 2 | 0 |

A verdict whose units span two of these packages counts in each.

### Derived listing

The verdict inventory is derived. Since `dcc219072a`, each verdict lives at
`rfc/audit/<stem>/<requirement-id>.json`; it is no longer a member of a flat
stem file's `requirements` map. Read each file's `verdict` and `tests` fields,
derive its stem and requirement ID from its path, then apply the package,
owned-stem and cross-group exclusions below. The query below replaces the old
flat-file query. Main owes the
current sharded listing and RFC freshness reconciliation before closure.

| Question | Derived by |
|----------|-----------|
| Every weak or wrong verdict in this child's scope (stem, id, verdict) | `jq -r --arg pk '^(internal/component/l2tp/\|internal/component/radius/\|internal/component/tacacs/\|test/l2tp/)' --arg own '^(rfc1332\|rfc1334\|rfc1661\|rfc1994\|rfc2516\|rfc2661\|rfc2865\|rfc2866\|rfc2869\|rfc3579\|rfc5072\|rfc5176\|rfc8907)$' 'input_filename as $f \| ($f\|split("/")\|.[-2]) as $s \| ($f\|split("/")\|.[-1]\|rtrimstr(".json")) as $id \| select(.verdict=="weak" or .verdict=="wrong") \| select($id\|IN("RFC1071-1-4","RFC4303-2.1-1","RFC5882-4.4-1","RFC905-x-3","RFC905-x-4")\|not) \| select(($s\|test($own)) or ([(.tests // {})\|keys[]\|test($pk)]\|any)) \| "\($s)\t\($id)\t\(.verdict)"' rfc/audit/*/*.json` |
| What the arguments mean | `pk` matches a tagged test path in this child's packages; `own` includes this child's thirteen owned stems, including units in another child's packages; the five excluded ids are the parent's cross-group verdicts |

**Wiring, 2026-09-28, at `cb4801b4cc`:** the filter returns 104 weak and 7 wrong, 111 in all, over 13 stems. This equals the Task figure, so `fefe38b68c` (it re-judged only the five cross-group verdicts, which the filter drops) and `cb4801b4cc` changed no count here. `./le rfc audit-stamp` accepts `mode rejudge` (A-1). The "Blocked by" verdicts are now `spec-radius-rfc-defects` AC-4.

### Owned stems

One writer per ledger file: this child alone writes `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/short/<stem>.md`, `rfc/corrections/<stem>.md` and `rfc/extraction/<stem>.json` for these stems.

| Stem | Weak + wrong (2026-09-28) | Of which wrong |
|------|--------------|----------------|
| rfc1332 | 4 | 0 |
| rfc1334 | 2 | 0 |
| rfc1661 | 17 | 2 |
| rfc1994 | 8 | 2 |
| rfc2516 | 13 | 2 |
| rfc2661 | 17 | 0 |
| rfc2865 | 7 | 0 |
| rfc2866 | 7 | 1 |
| rfc2869 | 2 | 0 |
| rfc3579 | 4 | 0 |
| rfc5072 | 3 | 0 |
| rfc5176 | 9 | 0 |
| rfc8907 | 18 | 0 |

Test files that carry tags of two owners. A child edits one only while the other holds no uncommitted change in it:

None: no file in this child's packages carries another owner's tag.

### Un-enrolled R-7 rows

Copied verbatim from the parent. A row leaves this table when its re-judged verdict is `enforced`, recorded here with the date, or in the stem's audit file if the stem is enrolled first.

None of this table's rows belong to this child.

### Split-needed rows

Copied verbatim from the parent. The parent's narrowing audit adds rows for this group, and this child takes them when that output is committed. "Row correction" marks a claim no sentence states.

| ID | Dropped obligation | Section |
|----|--------------------|---------|
| RFC1994-2-1 | SHOULD take action to terminate the link | §4.2 |
| RFC2661-6.1-1 | a message missing a required AVP is logged and the control connection cleared (lowercase should) | §7.1 |
| RFC2661-6.2-1 | SCCRP not acceptable: send StopCCN, clean up (state table, no keyword) | §7.2.1 |
| RFC2661-24.10-1 | refusing a peer's zero Tunnel or Session ID with StopCCN (lowercase should) | §7.1 |
| RFC2661-10-1 | CDN is valid in any non-idle session state (state tables only) | §7.4.1, §7.4.2, §7.5.1, §7.5.2 |
| RFC2865-3-4 | Access-Challenge: the Response Authenticator MUST be correct; invalid packets silently discarded | §4.4 |
| RFC2865-5.11-1 | MUST NOT affect operation of the protocol: Reply-Message, Framed-Route, Vendor-Specific, Proxy-State | §5.18, §5.22, §5.26, §5.33 |
| RFC2865-5.25-1 | the client MUST NOT interpret the State attribute locally (two of the row's tags prove this half) | §5.24 |
| RFC5176-2.3-5 | a Disconnect-Request MUST contain only NAS and session identification attributes; otherwise Disconnect-NAK | §3 |
| RFC5176-2.3-7 | with multi-session support, a CoA-Request or Disconnect-Request MUST apply to all matching sessions | §3 |
| RFC5176-3.2-1 | an unsupported Service-Type value MUST get a CoA-NAK | §2.2 |
| RFC5176-3.3-2 | the Dynamic Authorization Server MUST NOT interpret State locally | §3.3, last paragraph |
| RFC5176-3.5-5 | Error-Cause code placement: 201 only in Disconnect-ACK, 202 never sent, 502 not by a NAS, 504 only in Disconnect-NAK | §3.5, after the code table |
| RFC5176-3.6-1 | the same attribute MUST NOT be used for identification and authorization at once (Note 7) | §3.6 |
| RFC5176-6.3-1 | the duplicate-detection window MUST equal the stale Event-Timestamp window | §6.3 |
| RFC5176-3.1-1 | "in the order they arrived" binds only a forwarding proxy, not the NAS: row correction | §3.1 |
| RFC2869-x-2, x-5 | the Gigaword obligations for Acct-Output-Gigawords | §5.2 |
| RFC1661-5.8-5 | the same Magic-Number sentence for Discard-Request, "Until the Magic-Number Configuration Option has been successfully negotiated, the Magic-Number MUST be transmitted as zero.": no row states it for §5.9 (RFC1661-5.9-1 says Ze sends no Discard-Request today) | §5.9 |

### Missing rows

None of this table's rows belong to this child.

### Row-quality corrections

None of this table's rows belong to this child.

### Mistagged units

None of this table's rows belong to this child.

### Deferred items the parent's tables missed

None of this table's rows belong to this child.

### Narrowing audit rows (parent, 2026-09-28)

Copied from the parent's "Narrowing audit findings, 2026-09-28", where the process and the call meanings are. AC-C3 includes these rows.

| ID | Call | Obligation | Section |
|----|------|------------|---------|
| RFC2865-1.1-2 | dropped | "A NAS is not required to implement all of these service types, and MUST treat unknown or unsupported Service-Types as though an Access-Reject had been received instead." | §5.6 |
| RFC2865-5-8 | dropped | "Strings of length zero (0) MUST NOT be sent; omit the entire attribute instead." | §5 |
| RFC2869-x-5 | retire | No RFC 2869 sentence requires computing from byte count; quote only defines the counter. | none |
| RFC3579-3.2-1 | dropped | "The Message-Authenticator is calculated and inserted in the packet before the Response Authenticator is calculated." | §3.2 |
| RFC8907-x-1 | dropped | "For example, a server MUST be configured to time out a Single Connection Mode TCP connection after a specific period of inactivity to preserve its resources." | §4.3 |

### Blocked by

Each id below was checked on 2026-09-28: a weak or wrong verdict in `rfc/audit/<stem>.json`, or a row of the R-7 table above. The blocking spec owns the producer fix (AC-C4), and the verdict moves into its acceptance criteria (P-3).

| ID | Verdict | Blocking spec |
|----|---------|---------------|
| RFC3579-3.3-2 | weak | `spec-radius-rfc-defects` |
| RFC5176-2.3-2 | weak | `spec-radius-rfc-defects` |
| RFC2866-5.5-1 | weak | `spec-radius-rfc-defects` (D3, RFC 2866 Section 5.5: the row quotes the D3 sentence verbatim) |


## Required Reading

### Architecture Docs
- [ ] `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` - the Method table, the Required Reading constraints, D-2 to D-15, and the source tables above
  → Constraint: every row stays a verbatim span of its cited section, at least 24 characters, never across two sections (`checkRowQuotes`); a span that must cross a section is split into two rows
  → Constraint: widening, narrowing or re-citing a row stales its verdict (`stale-requirement`) until it is re-judged in the same commit; a new id takes n above the HEAD^ high-water mark of its section (`checkIDAllocation`)
  → Constraint: a new gated row lands with both polarity tags or an annotation, a discrimination record for each new cover (a moved tag is a new cover), and an extraction site mapped to it or listed in `unsourced-ids`
  → Constraint: an upgrade from weak or wrong to `enforced` with unchanged units needs an `upgrade_reason` (`checkAuditFindings`), so author and judge land in one commit where they can
  → Constraint: editing any function in a test file shifts every verdict tagging that file; `./le rfc reseal` is corpus-wide and rewrites other children's audit files, so a reseal runs only when no other child holds uncommitted audit changes
  → Constraint: D-3: the RFC2661 split rows carry a lowercase "should" or a state table with no keyword, so each new row keeps the level its sentence carries
  → Constraint: RFC5176-3.1-1 is a row correction ("in the order they arrived" binds a forwarding proxy, not the NAS), not a new row
- [ ] `docs/contributing/rfc-conformance-gates.md` - the discrimination record, the row quote, the refusals
- [ ] `ai/skills/ze-rfc-audit.md` - the four judgement questions, the verdict vocabulary, STRICTNESS, and the re-judge route
  → Decision: judge strictly: a floor assertion, a confounded buffer or a positive that proves only "no error" stays `weak`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/<stem>.md` for each owned stem above - the row text a verdict is judged against; the RFC text in `rfc/full/<stem>.txt` is the authority for every split and correction
  → Constraint: a claim that an obligation was dropped quotes the RFC sentence with its section (parent R-9)

**Key insights:**
- 111 verdicts over 9 packages and 13 stems, plus 18 rows copied from the parent's tables
- The listing is derived with the filter above, never copied; each stem's ledger files belong to one child
- A verdict whose producer another spec fixes is listed under "Blocked by" and leaves this child through P-3

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` - the parent: Split section, inventory tables, Method, Required Reading, AC-C1 to AC-C7
- [ ] `rfc/audit/rfc8907.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc1661.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc2661.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `internal/le/rfc/check_audit.go` - `checkAuditFindings` refuses a deleted weak or wrong verdict and an unchanged-units upgrade without `upgrade_reason`

**Behavior to preserve:**
- a commit of this child adds no `./le rfc check` violation of its own; a weak or wrong verdict is never deleted
- product behavior of every package in scope, except a D-8 fix of a verified defect, which carries its own failing-first test

**Behavior to change:**
- the tagged tests, rows, tags and verdicts named in this spec's tables and in the derived listing; D-8 producer fixes where a test exposes a verified defect

## Existing Mechanisms (MANDATORY)

Added 2026-11-11 at closure; the spec predates the section. Every mechanism below already existed, and this child uses each one as built.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Audit finding ratchet | `internal/le/rfc/check_audit.go` `checkAuditFindings`, `upgradeOverUnchangedUnits` | Refuses a deleted weak, wrong or partial verdict, and an upgrade to `enforced` over byte-identical units without an `upgrade_reason` | Uses it unchanged: every verdict this child moved went through it (P-2, AC-C5) |
| Re-judge stamp | `./le rfc audit-stamp ... mode rejudge`, which applies `upgradeOverUnchangedUnits` when it stamps | Replaces a recorded verdict in place with fresh fingerprints | Uses it unchanged (A-1, P-2) |
| Discrimination record | `./le rfc record`, `rfc/discrimination/<stem>/`, `docs/contributing/rfc-conformance-gates.md` | Observes the red of a tagged unit under a break of its producer and stores it | Uses it unchanged for every added or changed cover (AC-C2) |
| Interop scenario harness | `internal/le/interoplab`, `docs/architecture/testing/interop.md` | Discovers and runs named scenarios against real peers (xl2tpd, accel-ppp, pppd, FreeRADIUS) | Extended by five new scenarios for the D-8 fixes; the six existing scenarios are re-run as acceptance |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc check`, reading `rfc/short/`, `rfc/audit/`, `rfc/discrimination/` and the `RFC requirement:` tags in test files
- `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`, typed by a judging agent

### Transformation Path
1. The author agent records `./le rfc approve unit` (P-1), then changes a tagged test, a row, a tag, or a producer (D-8)
2. `./le rfc reseal` re-stamps sibling verdicts the edit only shifted
3. `./le rfc record` records the observed red under a break for each added or changed cover
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
| A-1 | the parent's `mode rejudge` has landed before the first re-judge here | parent Implementation Steps 1 and 2 | a re-judge has no route but hand deletion, which P-2 bans | `./le rfc audit-stamp` help names `mode` | validated 2026-10-11: `./le rfc audit-stamp` takes `mode rejudge` (round-1 review NOTE-3) |
| A-2 | the blocked-by list is complete for this scope | the parent's code-defect and overlap tables, each id checked in `rfc/audit/` or the R-7 table on 2026-09-28 | the child cannot reach AC-C1 | each package's author agent names any verdict it cannot move without a producer fix another spec owns | validated 2026-10-11: the derived listing gives exactly the 3 blocked ids, RFC2866-5.5-1, RFC3579-3.3-2 and RFC5176-2.3-2 (round-1 review NOTE-3) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a test fix exposes a defect that grows past the package | a code change outside the package | D-8: fix it; where impossible, back to the owner |
| R-2 | another child or spec commits while this child holds uncommitted hunks in a shared test file, or runs a reseal | a commit diff shows another owner's ids | the shared-file rule above; reseal and interop `revert` runs are serialized between children |
| R-C1 | eight RFC5176 split rows allocate new ids in §2.3, §3.x and §6.3 at once | `checkIDAllocation` refuses a commit | one phase adds every RFC5176 row, in one commit |

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

Inherited from the parent (AC-C1 to AC-C7), restated for the access group.

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
| every unit tagged by a listed verdict | `internal/component/l2tp/ppp/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/tacacs/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/l2tp/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/radius/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/l2tp/plugins/authradius/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/l2tp/pppoe/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/l2tp/pppoeclient/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `test/l2tp/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/l2tp/plugins/authlocal/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |

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
| named when a D-8 fix is found | `test/interop/scenarios/` | L2TP, PPPoE and RADIUS interop for any D-8 fix (accel-ppp, FreeRADIUS); none is planned before a defect is found | the fixed behavior against another implementation | |

## Files to Modify

- the `_test.go` and `.ci` files of the packages above that carry the listed tags
- `rfc/short/<stem>.md`, `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/corrections/<stem>.md`, `rfc/extraction/<stem>.json` for the owned stems only
- a producer under the packages above when a test exposes a verified defect (D-8), named in the phase that finds it
- `docs/features/rfc-status.md`, the row of each owned stem whose support claim changes
- the acceptance criteria of each blocking spec, which receive its blocked verdicts (P-3)

## Files to Create
- none planned; a split or missing row is a new row in an existing `rfc/short/<stem>.md`

### Integration Checklist

The initial N-A rows below described the verdict-only plan. D-8 subsequently
changed shipped behavior. The closure applicability table supersedes those
initial answers.

| D-8 surface | Applies now | Source and integration |
|-------------|-------------|------------------------|
| Config and validation | Yes | `traffic/yang/ze-traffic-control-conf.yang` adds the tc-only classless TBF root rate; `traffic/config.go` `parseInterfaceQoS` and netlink `verifyDesired` validate it before apply. VPP `verifyInterface` refuses it. |
| CLI output | Yes | `traffic/cmd/traffic.go` `handleShowTraffic` exposes `rate-bps`; `show.go` `FormatQoS` renders the root rate. Existing commands and completion remain the entry points. |
| Protocol and subscriber behavior | Yes | L2TP AVP handling, PPP termination, PPPoE CHAP re-challenge, RADIUS framing and CoA carry the D-8 fixes listed below. Five real-peer scenarios provide their wire and state evidence. |
| Runtime prerequisites | Yes | `internal/appliance/kernelreq.go` requires built-in `CONFIG_NET_ACT_POLICE`; `80175d768d` adds the requirement and negative controls. Existing PPP/tc runtime facilities are reused. |
| New plugin, event, SDK method, BGP family or environment key | No | D-8 uses existing registrations, subscriber events, backend verifier registry and interop scenario selectors. |

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

Closure review found that the initial verdict-only answers below no longer
describe D-8. These are the current applicable categories and committed pages.

| Category | Applies now | Existing update and producer |
|----------|-------------|------------------------------|
| User guides and wire behavior | Yes | `docs/guide/l2tp.md`, `docs/guide/pppoe.md`, `docs/guide/radius.md`, `docs/architecture/wire/l2tp.md` and `docs/architecture/l2tp/cpe-1-pppoe-client.md` describe the D-8 guards and lifecycle; commits `60817da535`, `c7fd516b4d`, `aaebde365c`, `f6c11e93f1`, `9793ba4da8`, `1870461d15` and `10b1ef1ebb`. |
| Config, CLI and traffic architecture | Yes | `0d9da69ff3` updates `docs/guide/traffic-control.md`, `docs/guide/command-reference.md`, `docs/guide/l2tp.md` and `docs/architecture/core-design.md` for classless TBF, bits/bytes, bounds and `rate-bps`. |
| Appliance prerequisites | Yes | `80175d768d` updates `docs/architecture/appliance/kernel-profiles.md` and `docs/guide/appliance.md` for built-in upload policing. |
| Test infrastructure | Yes | `9a51de9deb`, `1573a9970a` and `f200020a7f` update `docs/architecture/testing/interop.md`; PPPoE also updates `docs/labs/pppoe-interop.md`. |
| RFC support | Yes | Owned summaries, independent audit verdicts and discrimination records remain the authority; generated status pages are not hand-edited. Main is renewing 11 positive tuples and four mechanical RFC8907 shifts found by the current native check. |
| New API/RPC, plugin SDK, metadata, metric or comparison capability | No | The fixes use existing protocol/backend interfaces and preserve the existing feature scope. They add no such public surface. |
| Operator news | Required; prepared | `website/changes/fragments/rfc-verdict-fix-access.md` records the D-8 and confirmed-authorization behavior, supported selectors and recovery, with six headings and source anchors. Main carries it in commit A. |

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

**Procedure P, for every package phase.** One author agent per package, never two on one test file. It reads the listing restricted to the package, records `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before each tagged-unit edit (P-1), makes each clause of the quoted sentence asserted in both polarities or corrects the row, and fixes a verified defect under D-8. It runs the scoped package test under `./le job run`, then `./le rfc record` for each added or changed cover. An independent judge that wrote none of the tests writes the verdicts in a pending file under session scratch and stamps them with `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`. `./le rfc reseal` runs when the edit shifted sibling verdicts, serialized between children. Each package lands in one commit through `./le commit create` with the `RFC-approved:` trailer, author and judge together where they can. Verify: the listing filtered to the package holds only blocked ids, and `./le rfc check` adds no violation.

1. **Phase: Wiring** - confirm the parent's `mode rejudge` has landed (A-1), reproduce the derived listing and its counts, and move each "Blocked by" verdict into its blocking spec's acceptance criteria (P-3)
   - Tests: the Wiring Test row
   - Files: the blocking specs' Acceptance Criteria tables
   - Verify: the listing count equals the Task figure, or the difference is explained by commits since 2026-09-28
2. **Phase: `internal/component/l2tp/ppp`** - 31 verdicts, 4 wrong; procedure P
3. **Phase: `internal/component/tacacs`** - 18 verdicts, 0 wrong; procedure P
4. **Phase: `internal/component/l2tp`** - 17 verdicts, 0 wrong; procedure P
5. **Phase: `internal/component/radius`** - 16 verdicts, 1 wrong; procedure P
6. **Phase: `internal/component/l2tp/plugins/authradius`** - 15 verdicts, 0 wrong; procedure P
7. **Phase: `internal/component/l2tp/pppoe`** - 12 verdicts, 2 wrong; procedure P
8. **Phase: `internal/component/l2tp/pppoeclient`** - 8 verdicts, 0 wrong; procedure P
9. **Phase: `test/l2tp`** - 6 verdicts, 0 wrong; procedure P
10. **Phase: `internal/component/l2tp/plugins/authlocal`** - 2 verdicts, 0 wrong; procedure P
11. **Phase: split and missing rows** - after the parent commits its narrowing output for this group (parent R-11): each row of the split-needed and missing-rows tables becomes a row of its own with both polarity tags, a record and a verdict, or a dated correction says why not (AC-C3)
   - Verify: `./le rfc check` accepts the new ids, extraction sites and records
12. **Phase: row quality and mistagged units** - each row of those tables corrected, merged or re-tagged, or a dated correction says why it stands; each changed row or tag re-judged (AC-C6)
   - Verify: `./le rfc check` reads no stale verdict in the owned stems
13. **Phase: closure check** - the derived listing holds only "Blocked by" ids (AC-C1), each named in its blocking spec's acceptance criteria

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
- [ ] The scoped runs of what the change reaches pass (no agent runs `./le verify`; GitHub CI verifies the pushed commit)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for the D-8 protocol fixes (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** tests + rows + verdicts + records + D-8 code + edited spec
- [ ] **Commit B:** `remove plan/release/validation/spec-rfc-verdict-fix-access.md` only, in the same `./le commit create` script

## Progress, 2026-10-11

Written from `plan/handover/rfc-verdict-test-fix-pass/` (QUEUE.md, HANDOFF.md) and git history, each claim re-checked against HEAD `c0d31c3660` on 2026-10-11. Implementation is finished; the spec waits for its independent `/ze-review`. It is not closed.

## Implementation Summary

### What Was Implemented
- 34 commits since 2026-09-28 touch this child's thirteen `rfc/audit/<stem>.json` files; the last author/judge pairs are `4e55b63ea6` (RFC1994-4.2-5, RFC5176-3-1 re-judged) and `b3c573f601` (RFC1332-2.1-1).
- 2026-10-11: `internal/component/l2tp/ppp/rfc1661_transition_cells_test.go` (landed untagged in the wip commit `af69577952`) carries `// RFC naming: untagged -- ...`. RFC 1661 Section 4.1 is the transition table and holds no MUST, SHOULD or REQUIRED (checked in `rfc/full/rfc1661.txt`), and `rfc/short/rfc1661.md` has no 4.1 row, so there is no requirement to tag; a tag would also need a discrimination record. Test bodies unchanged; `go test -run TestRFC1661 ./internal/component/l2tp/ppp/` passes.
- 2026-10-11, round-1 review ISSUE-1 (RFC 2869 Section 5.2, Acct-Output-Gigawords): `rfc/short/rfc2869.md` gains `RFC2869-5.2-1` [MUST], the verbatim Section 5.2 sentence ("This attribute indicates how many times the Acct-Output-Octets counter has wrapped around 2^32 in the course of delivering this service, and can only be present in Accounting-Request records where the Acct-Status-Type is set to Stop or Interim-Update."), at the level `RFC2869-x-2` uses. Two new tagged tests in `internal/component/l2tp/plugins/authradius/rfc2869_output_gigawords_test.go`: positive `TestRFC2869OutputGigawordsOnInterimUpdate` (Interim-Update, transmit counter three wraps plus 200: Acct-Output-Gigawords 3, Acct-Output-Octets 200) and negative `TestRFC2869OutputGigawordsAbsentOnStart`. Records by `./le rfc record`, route revert: the positive over `acct.go::splitGigawords`, the negative over `acct.go::buildAcctPacket` (the same producer the other buildAcctPacket negatives use), each observed red naming its unit. `rfc/extraction/rfc2869.json` Section 5.2 declares `RFC2869-5.2-1` in `unsourced-ids`, and Section 5.1's stale "five gated Gigawords rows" now says one row, `RFC2869-x-2`, is left after the retirements. No exclusion was added, so the sign-off is not re-signed. The verdict is NOT stamped: the author of the row and the tags cannot judge it, so an independent `/ze-rfc-audit rfc2869` judge owes it.
- 2026-10-11, owner decision on RFC8907-4.3-3 (round-1 review ISSUE-5): "Drop the tag, re-judge". `TestRFC8907ClosureOnFreshDialIsNotRetried` lost its negative tag (its body is unchanged, and a one-line comment says why it is untagged), and its record in `rfc/discrimination/rfc8907.json` was removed. The verdict was re-stamped with `./le rfc audit-stamp ... mode rejudge` as `enforced` on the two remaining units, `TestRFC8907ServerClosureOfPooledConnectionIsAccommodated` (positive) and `TestRFC8907ClosureMidSessionOnPooledConnectionIsAccommodated` (negative). The tooling does not tell an author's verdict from a judge's, so its note says it was made by the author of the tag change and an independent judge must confirm it.

### Bugs Found/Fixed (D-8, each with production code)
| Commit | Fix |
|--------|-----|
| `0cc6cb1b22` | RADIUS CoA value checks and atomic apply (RFC 5176) |
| `60817da535`, `c7fd516b4d` | L2TP RFC 2661 fixes; 4.1-3 unknown M=1 AVP, 7.1-1 out-of-state SCCRQ/SCCRP |
| `aaebde365c`, `f6c11e93f1` | PPP session fixes; tlf on Stopped |
| `9793ba4da8` | PPPoE client answers CHAP re-challenge in network phase |
| `1870461d15`, `189db75308`, `10b1ef1ebb` | RADIUS Framed-Protocol, NAS identity (RFC3579-3-1), EAP header and Framed-IP |
| `8c68fcf514`, `4e55b63ea6` | CoA 504 on SessionNotRemovable and on service stop; L2TP idle SCCRP |

### Deviations from Plan
- None beyond the naming marker above.

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Verdict-only integration and documentation answers survived after D-8 added product changes. | TBF config/output, subscriber shaping and protocol lifecycle changed and had committed documentation. | Independent closure review, 2026-11-11. | Current applicability tables above replace those initial answers. |
| approach | CoA admission equated event delivery with an applied authorization change. | Backend rate refusal returns through a void event callback, while the listener sends ACK. | Independent Review Gate round 1 traced `handleCoA` through `onSessionRateChange` to `rateBytesPerSecond`. | Repaired through synchronous owner confirmation and compensation. Round 3 is source CLEAN; current RFC reconciliation and interop remain separate gates. |
| approach | The rate owner bounded its pending slot count but gave the slot no lifetime. | A lost prepare reply leaves the coordinator no token to release the slot; expired applied state also needs compensation. | Review 2 traced the lost-reply path; main observed three deterministic clock regressions blocked by the occupied slot. | A one-minute lease now recovers before the next admitted operation. Failed compensation retains its token and snapshot. Main's scoped race and native proof runs passed; overall closure remains open. |
| wrong assumption | The new useful-control test treated plain 20mbit as a download-only request preserving a 5M upload. | `ParseFilterIDRate` intentionally makes a plain rate symmetric. | Main's baseline exposed the upload assertion and read the parser contract. | Changed the new test input to explicit 20mbit/5mbit, retaining the 5M assertion. The production parser is unchanged, and the corrected control passed main's scoped race run. |

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-C1 | Source and owned RFC reconciliation complete; closure blocked | Current selectors and complete accounting IDs match; unsupported identification attributes NAK. Round 3 is source CLEAN; final thirteen-stem check has zero owned findings. | Fresh evidence is in the 2026-11-12 checkpoint. Guest acceptance and lint remain open. |
| AC-C2 | Native proof renewals complete | 121 verdicts moved to `enforced` since `174862502f`; historical sweep found zero unrecorded tagged units. Main recorded all 17 current renewals, including the eight new real-owner/signed-wire bindings. | `scratch/job-access-final-seventeen-rfc-proofs-d82b2fea.log`; actual kernel CoA acceptance remains separately pending. |
| AC-C3 | Done | rfc5176 split rows `8c68fcf514`, RFC1994-4.2-5 row `4e55b63ea6`, RFC2866 table split `10b1ef1ebb` | reviewer to re-walk the split table |
| AC-C4 | Done | the three blocked verdicts are in `spec-radius-rfc-defects` | |
| AC-C5 | Current owned reconciliation complete | `scratch/access-final-owned-rfc-lines.json` is empty after fresh native regeneration/check. | Eight independent semantic rejudgments and mechanical shifts applied; foreign global findings remain. |
| AC-C6 | N-A | the parent's row-quality and mistagged-unit tables hold no row of the access stems | RFC2759-x-8/x-9 belong to `spec-rfc-verdict-fix-ike-eap`; the `8c68fcf514` corrections cited here before are AC-C3's evidence (round-1 review NOTE-1) |
| AC-C7 | N-A | no R-7 row belongs to this child | the Linux-only RFC2866-4.1-1 `.ci` record the handover listed as owed landed in `dd09cdd4ff` |

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| AC-C1: the derived listing holds only "Blocked by" ids | ledger query | 3 rows, all in `spec-radius-rfc-defects` ACs |
| every resolved verdict proven and judged by a non-author | discrimination records + audit | 0 unrecorded tagged units over 121 moved verdicts; `./le rfc check` clean for the access stems |
| D-8 fixes work against another implementation | interop | All five new scenarios have native green / intended-red / restored-green evidence. The three link scenarios are recorded in `9a51de9deb` and `1573a9970a`; the two subscriber scenarios are recorded below. Existing-scenario acceptance is the "Existing-scenario acceptance, 2026-11-11" table, pending |

## D-8 Interop Implementation, 2026-10-26

The five scenarios below were implemented on 2026-10-26. Their native
discrimination is complete; the OPEN GATE remains in force for the named
existing-scenario acceptance runs and closure evidence.

| Scenario | Producer and distinguishing observation |
|----------|-----------------------------------------|
| `l2tp-unknown-mandatory-avp-xl2tpd` | `l2tp/mandatory_avp.go` `checkUnknownMandatoryAVP`: a transparent relay appends an optional unknown AVP to real xl2tpd SCCCN, proves PPP traffic, then appends the mandatory form to real HELLO; requires matching StopCCN 2/8 and both-peer cleanup |
| `pppoe-chap-rechallenge-ze-client` | `pppoe/check_lifecycle.go` `checkCHAPRechallenge`: two native accel-ppp reauthentication exchanges after IPCP traffic, exact CHAP digest and Identifier, unchanged session and subsequent traffic |
| `pppoe-peer-terminate-ze-ac` | `pppoe/check_lifecycle.go` `checkPeerTerminate`: real pppd Terminate-Request with its outgoing PADT filtered; requires Ze Terminate-Ack, delayed Ze PADT, REST removal, both PPP interfaces removed and an empty Ze kernel PPPoE table |
| `radius-coa-disconnect-freeradius` | `radiussubscriber/check_coa_disconnect.go` `checkCoADisconnect`: real radclient ACK/NAK, kernel download and upload rate changes, mixed-validity CoA refusal without partial changes, refused then accepted Disconnect and matching Accounting-Stop |
| `l2tp-radius-framed-protocol-reject` | `radiussubscriber/check_framed_protocol.go` `checkFramedReject`: the same FreeRADIUS user receives PPP, SLIP, then PPP authorization; requires initial/recovery traffic and intermediate Ze refusal, pppd CHAP failure and no Accounting-Start |

The subscriber scenarios reuse the FreeRADIUS subscriber suite, not the
admin-login suite or the L2TP RADIUS mock. The scenario implementation changed
no RFC tags or audit ledger. Native execution subsequently exposed the kernel
POLICE omission and userspace shaping defects; `80175d768d` and `0d9da69ff3`
repair those producers. No full-tree check was run.

Owner-approved test maintenance replaced the seven affected population,
topology and configuration-hash units after preserving existing selector and
shared peer-start/readiness failure coverage and adding focused missing-input,
invalid-network/rendering and subscriber-selector behavior tests. No hashes or
counts were repinned. The five real-peer scenarios and packet/parser/digest/relay
tests remain. Native weakening approvals and exact unit names are retained in
the per-spec session state. The scoped eight-package race proof completed in
two retained runs, continuing only roots not completed before the first run
was interrupted; already-passing tests were not repeated.

### Current native proof, 2026-10-27

| Scenario | Observed proof | Commit or remaining obligation |
|----------|----------------|--------------------------------|
| `pppoe-chap-rechallenge-ze-client` | Green, private network-phase CHAP break red, normal rebuild green | `9a51de9deb` |
| `pppoe-peer-terminate-ze-ac` | Green, private layer-finished break red, normal rebuild green; peer/kernel cleanup retained | `9a51de9deb` |
| `l2tp-unknown-mandatory-avp-xl2tpd` | Optional-AVP control and mandatory-AVP teardown green, intended private break red, normal rebuild green | `1573a9970a`; earlier infrastructure-contaminated attempts are not the discrimination evidence |
| `radius-coa-disconnect-freeradius` | Baseline green; private `readCoAChange` break accepts an unsupported second Filter-Id and receives CoA-ACK instead of CoA-NAK; normal rebuild green in 13s, including both kernel rate directions, atomic refusal, traffic and Disconnect/accounting checks | Product correction `0d9da69ff3`; checker repair `f200020a7f`; native red `job-access-radius-coa-guest-red-20261027-n-293bab6b.log`, restored `job-access-radius-coa-guest-restored-20261027-o-37e5eb58.log` |
| `l2tp-radius-framed-protocol-reject` | Baseline green; private framing-guard break admits real SLIP authorization with CHAP Success and bad Accounting-Start; normal rebuild green in 18s with PPP recovery | Checker repair `f200020a7f`; native red `job-access-radius-framed-guest-red-20261027-h-23e4c12c.log`, restored `job-access-radius-framed-guest-restored-20261027-k-0be19b47.log` |

Framing used Ze Docker-host image `789bdd6a7c1a5487`. The CoA red and restore
used the newly built image `bfc811e1b78a73f6` and runtime kernel key
`7.2.9-runtime-amd64-runtime-docker-a2bfd4f0-34cba457`. Both pairs ran under
KVM with kernel-check exit zero; every VM stopped with no leftover resources.
The deliberate negatives returned lab exit one and their normal restores
returned zero. Private Go overlays changed only the named guard; shared
product source was not mutated.

The full commands, six logs, mutation hashes and image provenance are retained
under `tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/scratch/`,
indexed by `access-radius-proof-final-20261027.txt`. Checked 2026-11-11: that
scratch directory no longer holds them (only `appliance-amd64-proof/` remains),
so the durable record is the commit bodies: the eight-package race proof is
stated in `f200020a7f`, `9a51de9deb` and `1573a9970a`, and the weakening
approvals are those commits' `Weakening-approved:` trailers. Earlier capacity,
fixture and foreign-source compilation failures are not discrimination evidence.
These runs do not refresh the historical RFC verdict or record inventory above
and do not close AC-C1 through AC-C7 by themselves.

## Existing-scenario acceptance, 2026-11-11

The six existing scenarios that exercise the D-8 producers, run in the main
checkout over the committed fixes. Logs are retained at
`tmp/access-close/<scenario>.log`.

| Scenario | Command | Result | Log |
|----------|---------|--------|-----|
| `01-ppp-ipv4` | `ZE_L2TP_INTEROP_SCENARIO=01-ppp-ipv4 ./le test deployment docker-l2tp-ppp` | pending | `tmp/access-close/01-ppp-ipv4.log` |
| `03-ze-lac-xl2tpd-lns` | `ZE_L2TP_INTEROP_SCENARIO=03-ze-lac-xl2tpd-lns ./le test deployment docker-l2tp-ppp` | pending | `tmp/access-close/03-ze-lac-xl2tpd-lns.log` |
| `04-radius-acct-attrs` | `ZE_L2TP_INTEROP_SCENARIO=04-radius-acct-attrs ./le test deployment docker-l2tp-ppp` | pending | `tmp/access-close/04-radius-acct-attrs.log` |
| `01-pppoe-chap-ipv4` | `ZE_PPPOE_INTEROP_SCENARIO=01-pppoe-chap-ipv4 ./le test deployment docker-pppoe-accel` | pending | `tmp/access-close/01-pppoe-chap-ipv4.log` |
| `02-ze-ac-pppd-client` | `ZE_PPPOE_INTEROP_SCENARIO=02-ze-ac-pppd-client ./le test deployment docker-pppoe-accel` | pending | `tmp/access-close/02-ze-ac-pppd-client.log` |
| `radius-admin-eap-freeradius` | `RADIUS_INTEROP_SCENARIO=radius-admin-eap-freeradius ./le test integration interop-radius` | pending | `tmp/access-close/radius-admin-eap-freeradius.log` |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| RFC2866-5.5-1, RFC3579-3.3-2, RFC5176-2.3-2 | producer defects (P-3) | `spec-radius-rfc-defects`, closed 2026-10-20 with all three verdicts `enforced` |
| OPEN GATE: interop over the wire-visible D-8 fixes | All five new scenarios have green / intended-red / restored-green proof; existing-scenario acceptance remains | This spec still owes acceptance runs of `01-ppp-ipv4`, `03-ze-lac-xl2tpd-lns`, `04-radius-acct-attrs`, `01-pppoe-chap-ipv4`, `02-ze-ac-pppd-client` and `radius-admin-eap-freeradius`, with retained logs |
| Scoped evidence for the committed change | No agent runs any `./le verify` command, `./le verify changed` included (`ai/META-DECISIONS.md`, "Ze is pre-release"); the evidence is the scoped runs of what the change reaches | The evidence already recorded in this spec: the scoped eight-package race proof (two retained runs, "D-8 Interop Implementation, 2026-10-26"), the native red/green records of the five new scenarios ("Current native proof, 2026-10-27"), and the "Existing-scenario acceptance, 2026-11-11" table once its rows are filled. GitHub CI's `verify` workflow verifies the pushed closing commits; a red it reports gets a fix commit of its own and does not reopen this spec. Full verification is Thomas's release check, not a closure gate |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | Source CLEAN at round 3. Main recorded `tmp/review/rfc-verdict-fix-access-01a1c842-0a58-7752-bbff-2a3f1633b66f.md` over 240 carriers. Independent reports are `scratch/access-closure-round1.{md,json}`, `access-closure-round2.{md,json}` and `access-closure-round3.{md,json}` under this session. |
| `./le spec review check` | Main observed PASS over 240 carriers at the completed source checkpoint after the byte-identical renames. Later foreign diagnostic hunks are excluded; the owned patch retains the reviewed hash. Source CLEAN remains round 3. |
| Rounds | 3 official passes: round 1 found three product/style groups; round 2 found two lifecycle/event issues; round 3 reviewed those fixes and touched siblings with zero BLOCKER and zero ISSUE. |
| Reviewer lenses used | Correctness and wiring; protocol and security boundaries; lifecycle/resource ownership; test preservation; Go style; source-linked documentation and all AC-C rows. |

### Round 1 scope and result, 2026-11-11

The historical audit-touch cohort through 2026-10-12 plus the five recorded
D-8 commit groups is pinned in `scratch/access-review1-commits.txt` (43 commits).
The historical count of 34 is not the current review population. Shared commits
are restricted to Access paths, the thirteen owned RFC stems and their guides;
the D-8 kernel, traffic and harness pieces are included in full. The source
patch is `scratch/access-review1-complete.patch`, SHA256
`edb82ef1b76e46f17fa1c693b2f5596bbf64e4bfb8c7bf5befb457a6f0f980a3`.
Its manifest has 371 historical paths, 283 current files and 212 current Go
files. Current foreign dirty hunks are outside approval.

The pass found two CoA product issues and one registration-style issue. The
subscriber-selection limitation is already disclosed in `docs/guide/l2tp.md`;
the finding is its conflict with owned RFC5176-3.3-1's enforced whole-combination
claim, not a claim that the limitation is newly introduced. PPP/L2TP lifecycle
and the five real-peer checker paths yielded no additional product finding.
No test, build, lint, proof or native metadata action was executed by this agent.

Main's latest global prechecks reported 31 repo findings and 21 test deletions;
main must compare their paths with this manifest before recording ownership.
The deletions belong to the foreign MPLS test-file move. These global reports
are not represented as globally clean. Historical committed weakening approvals
for the D-8 harness replacements remain in `9a51de9deb` and `1573a9970a`.

### Round 2 scope and result, 2026-11-11

Round 2 retains the original 43-commit review context and adds the 35 repair
paths in `scratch/access-review2-fix-paths.txt`, including new command owners,
health reporting, exact accounting identity, assertion-preserving fixtures,
YANG registration and source-linked docs. The repair patch is
`scratch/access-review2-fixes.patch`, SHA256
`38880e2229856a38b2b792f5476692350a7523c1610085e0fae23ce9613d8a38`.
The union contains 309 current files and 238 code, fixture and schema carriers;
`scratch/access-review2-source-pins.json` retains their hashes. Foreign shared
diagnostic hunks are outside approval.

The original three groups are repaired in source: ACK follows confirmed owner
application with compensation, supported selectors intersect across registry
and complete current-accounting fallback, unsupported selectors NAK, and tc
verifier registration has a register-named carrier. Two new issues remain:
an abandoned rate prepare has no expiry and permanently blocks different
transactions; a CoS command receipt permanently suppresses later legacy events
for the same profile name. The source verdict is ISSUES, not CLEAN. Main's full
Authradius result and native RFC renewals remain separate execution gates.

### Round 3 final source verdict, 2026-11-11

Round 3 covers the two round-2 fixes and affected sibling paths, retaining the
earlier full implementation review. Source is CLEAN: zero BLOCKER and zero
ISSUE. `rateOperation` recovers expired pending transactions through
`restoreRate` before new work, retaining failed compensation for retry.
`consumeCommandReceipt` consumes only a matching committed receipt whose two
map sets still equal the current profile, so lost or delivered notifications
cannot suppress a later profile update. The three expiry controls and both
CoS reload cases drive those exact producers.

Reports are `scratch/access-closure-round3.md` and `.json`. The current repair
patch `scratch/access-review3-fixes.patch` has SHA256
`ffb343dc039051880b6739412ea49d0bf3e57487752ab3b21bcd3ec20d7ba366`.
Its union with the original cohort contains 311 current files and 240 code,
fixture and schema carriers, listed in `scratch/access-review3-code-paths.txt`.
Source hashes are in `scratch/access-review3-source-pins.json`, SHA256
`7ec5678f47ef273cfb6d5dff5533ad4dbee7b14951ad51c8dad1eff0bd961ba4`.
Main observed all 393 affected-package race roots PASS with zero skips, the
CoS build without L2TP PASS, and the current named build PASS. Native review
record/check passed over 240 carriers. Four new test carriers were then renamed
byte-identically to RFC5176 names; current manifests and pin keys reflect
`scratch/access-test-filename-renames.json`, preserving every content hash.
Main refreshed and checked the same round-3 artifact after this naming NOTE.
Owned RFC renewals/audits are complete; guest acceptance and lint remain open.
Source CLEAN is not closure. The prose and naming refresh adds no review round.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE, source fixed (round 2) | CoA can ACK an unapplied change: `1bit` passes value parsing, the tc backend refuses it, and the void event callback cannot return that failure to the listener. | `internal/component/l2tp/plugins/authradius/coa.go` `setRate`, `handleCoA`, `applySubscriberCoA`; `internal/component/l2tp/plugins/shaper/shaper.go` `onSessionRateChange`, `applyTC`; `internal/plugins/traffic/netlink/translate_linux.go` `rateBytesPerSecond` | `confirmCoA` waits for real owner results and compensates failures; exact rate/backend/mixed-control regressions are retained. Native RFC renewal remains main's separate gate. |
| 2 | ISSUE, source fixed (round 2) | Subscriber lookup by Acct-Session-Id bypasses conflicting User-Name/NAS-Port selectors; the whole-combination RFC5176-3.3-1 claim is not enforced on that path. | `internal/component/l2tp/plugins/authradius/coa.go` `findSubscriberSession`, `handleCoA`, `findSessions`; `internal/component/l2tp/subscriber/registry.go` `LookupByAcctSessionID` | Implemented selectors intersect on both routes; unsupported types NAK and fallback matches the complete current accountant ID. Registered-subscriber, stale-ID and useful live-ID controls are retained. |
| 3 | ISSUE, source fixed (round 2) | The tc verifier registers from `verify_linux.go`, with `os.Exit`, outside the required `register*.go` placement. | `internal/plugins/traffic/netlink/verify_linux.go` `init` | Registration is now in `register_verify_linux.go`; `verifyDesired` behavior is unchanged. |
| 4 | ISSUE, source fixed (round 3) | Losing a successful rate prepare's reply leaves a global pending slot with no expiry, so the coordinator cannot recover its unknown token and later rate changes remain refused. | `internal/component/l2tp/plugins/shaper/rate_transaction.go` `prepareRate`; `internal/component/l2tp/plugins/authradius/coa_transaction.go` `confirmCoA` | `rateOperation` checks the one-minute lease and uses `restoreRate`; three deterministic expiry/retry controls cover the fix. Source reviewed CLEAN in round 3. |
| 5 | ISSUE, source fixed (round 3) | The CoS command notification marker is never consumed; a later legacy event for the same profile is skipped even after that profile's maps change. | `internal/plugins/cos/handler.go` `commandAlreadyApplied`, `onCoSChange` | `consumeCommandReceipt` requires a matching committed token and equal captured maps, then consumes one receipt; delivered/lost notification reload controls preserve later events. Source reviewed CLEAN in round 3. |

## Pre-Commit Verification

This record remains blocked on the two foreign execution inputs in the current
checkpoint. Owned RFC freshness and source review are complete; lint and the
five pending guest runs remain open. No closing commit is claimed.

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| Owned Go and D-8 harness sources | Source-read population located | `scratch/access-review1-existing.txt` lists 283 current files, including 212 Go files; main's bounded planned-file `ls` remains owed. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-C1, AC-C2, AC-C5 | Source and owned RFC gates complete; closure remains blocked. | All 17 native renewals recorded; eight semantic rejudgments and mechanical shifts applied; final owned thirteen-stem finding list empty. |
| AC-C3, AC-C4 | Split/correction history and former blocked work are retained. | Commits `8c68fcf514`, `4e55b63ea6`, `10b1ef1ebb` and closed `spec-radius-rfc-defects`; no new cross-owner ledger edit is authorized. |
| AC-C6, AC-C7 | No applicable rows were assigned by the parent's named tables. | Those source tables in this spec are empty; RFC2759 belongs to the IKE/EAP child. |
| D-8 end-to-end behavior | Historical discriminated scenarios retained; two existing acceptance passes preserved. | `01-ppp-ipv4` and `03-ze-lac-xl2tpd-lns` PASS in their retained logs. The final five-case retry (four other existing cases plus current kernel CoA control) stops before assertions on the foreign missing FIB vendor import; the VM stopped with no leftovers. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| RFC record and audit readers | Native `./le rfc check`, not a daemon `.ci` | Current source `checkAuditFindings` reads requirement-sharded files; main owns final execution. |
| L2TP unknown AVP, PPPoE CHAP/termination, RADIUS framing/CoA | Native interop scenarios, not `.ci` wrappers | `scenarioCheckerMap` in L2TP/subscriber suites and PPPoE `checkers` reach all five actual peer checkers; green/intended-red/restore evidence remains in the recorded commit bodies. |
| Existing subscriber and admin-login controls | Six named existing scenarios plus current kernel CoA control | Two earlier passes remain valid. Five-case final retry is blocked before assertions; exact commands and unblock action are in Blocker. No broad suite pass is substituted. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | Confirmed | `audit-stamp` supports `mode rejudge`; independent stamps and historical commit records use it. |
| A-2 | Confirmed; source and owned RFC work complete | The three old blocked rows were closed by the RADIUS defect spec. Current CoA findings are fixed and source-reviewed CLEAN at round 3; final owned RFC reconciliation is zero. Guest acceptance and lint remain blocked. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| L2TP AVP and PPP/PPPoE lifetime | `AVPIterator.Next`, `handleHello`, `applyTransition`, `signalLayerFinished`, `networkPhaseCHAP.handle` | Governing wire, subscriber and CPE pages have corresponding source anchors and committed D-8 edits. |
| CoA atomicity and session selection | `handleCoA`, `applySubscriberCoA`, `confirmCoA`, `findSubscriberSession` | Docs describe confirmed owner application, compensation, all implemented selectors, complete accounting identity and explicit refusal of unsupported identification. Source-reviewed CLEAN; actual kernel acceptance remains pending. |
| Traffic config, output and kernel prerequisite | `parseInterfaceQoS`, `FormatQoS`, `verifyDesired`, `rateBytesPerSecond`, `runtimeKernelRequirements` | Current guide/schema describe root rate, exact units and built-in police support; D-8 applicability above is corrected. |
| Native peer observations | `checkCHAPRechallenge`, `checkPeerTerminate`, `checkUnknownMandatoryAVP`, `checkCoADisconnect`, `checkFramedReject` | The interop page describes actual external peers and observed wire/kernel state. |
| Closure news and final doc checks | D-8 has operator-visible behavior changes. | Canonical six-heading news is prepared for this blocked handback. Main's final documentation/citation/planned-file checks remain owed; no verdict-only exemption is claimed. |
