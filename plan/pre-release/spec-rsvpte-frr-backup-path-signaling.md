# Spec: RSVP-TE fast reroute backup-path signaling and merge point (RFC 4090 Sections 6.1, 6.4.3, 6.4.4, 7.1.1)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-09 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The inherited implementation repaired only the data plane. The current working tree adds the protected PATH through the bypass, sender-template-specific reply mapping, merge-point ERO processing, and merge-point downstream refresh. `internal/plugins/rsvpte/frr_bypass_rfc4090_test.go::TestRFC4090ProtectedPathSurvivesRepair` covers link protection, node protection and an egress merge point. Head-end repair uses a distinct local sender address and a two-label ingress push; the two `TestRFC4090HeadEnd*Sender` tests cover that identity boundary. These are source and test-coverage claims only: Main's current validation and discrimination results are required before closure.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC4090-6.1-1 | "If the head-end of a tunnel is also acting as the PLR, it MUST choose an IP address different from the one used in the SENDER_TEMPLATE of the original LSP tunnel." (S6.1.1) | `frr.go::repairSender` selects a distinct configured local IPv4 address; without one, the head-end does not arm a bypass; validation owed |
| RFC4090-6.4-1 | "The RSVP_HOP object MUST contain an IP source address belonging to the PLR." (S6.4.3) | `engine.go::sendPath` builds the backup PATH with the PLR router-id; validation owed |
| RFC4090-6.4-2 | "The PLR MUST generate an EXPLICIT_ROUTE object toward the egress." (S6.4.3) | `frr.go::backupPath` constructs the MP-to-egress ERO; validation owed |
| RFC4090-6.4-3 | "the PLR MUST: - remove all the sub-objects proceeding the first address belonging to the MP, and - replace this first MP address with an IP address of the MP." (S6.4.4) | `frr.go::backupPath` trims through the first MP subobject and installs the MP host address; validation owed |
| RFC4090-7.1-1 | "If merging occurs and one of the Path messages merged was for the protected LSP, then the final Path message to be sent MUST be that of the protected LSP." (S7.1.1) | `frr.go::mergeBackupPath` retains the protected downstream PSB when a backup sender joins it; validation owed |
| RFC4090-7.1-2 | "Once the final Path message has been identified, the MP MUST start to refresh it downstream periodically." (S7.1.1) | `register.go::refreshPaths` refreshes the MP's merged PATH while incoming branches retain independent expiry; validation owed |

## Required Reading

<!-- Backfilled at closure (2026-10-09) from the spec's own content: the spec
     predates these template sections. -->

### Architecture Docs
- [ ] `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` - facility backup, bypass and merge-point behavior
  → Decision: an alternate merge-point source address is accepted only when a live native OSPF or IS-IS database attributes it to the same node
  → Constraint: the bypass PATH carries a SESSION_ATTRIBUTE with every flag clear (owner decision 2026-10-09)
- [ ] `docs/architecture/rsvpte/mpls-rsvp-te.md` - PSB, PATH builder, SESSION_ATTRIBUTE relay
  → Constraint: a transit forwards a received SESSION_ATTRIBUTE byte for byte (RFC 3209 Section 4.7.4)
- [ ] `docs/architecture/testing/interop.md` - freeRtr RSVP-TE lab shapes and scenario rows
  → Constraint: an interop proof needs a forced red with the Ze image rebuilt

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4090.md` - requirement rows 6.1-1, 6.4-1, 6.4-2, 6.4-3, 7.1-1, 7.1-2
  → Constraint: the backup PATH's sender and RSVP_HOP are the PLR's, and its ERO starts at the merge point
- [ ] `rfc/short/rfc3209.md` - SESSION_ATTRIBUTE (Section 4.7.1, C-Type 7) and the PATH grammar (Section 3.1)
  → Constraint: Name Length is the length before padding, and the name is null padded to four bytes

**Key insights:**
- freeRtr refuses a PATH with no SESSION_ATTRIBUTE, so a freeRtr merge point needs the bypass to carry one.
- freeRtr answers the backup PATH from its eth0 address, not the merge point, so the lab supplies the IGP identity proof.

## Current Behavior

**Source files read:**
- [ ] `internal/plugins/rsvpte/frr.go` - `tryLocalRepair`, `backupPath`, `repairedLSP`, `mergeBackupPath`, `repairSender`
- [ ] `internal/plugins/rsvpte/register.go` - `setupBypass`, `refreshPaths`
- [ ] `internal/plugins/rsvpte/build.go` - `buildPath` emits the PSB's SESSION_ATTRIBUTE
- [ ] `internal/le/interoplab/rsvpte/checkers.go` - freeRtr scenario checkers

**Behavior to preserve:**
- An ordinary tunnel and a transit build or relay SESSION_ATTRIBUTE exactly as before.
- The merge-point identity rule in `repairedLSP` (`samePeer`) is unchanged.

**Behavior to change:**
- The PLR signals a backup PATH through the bypass after a failure (RFC 4090 Sections 6.1, 6.4.3, 6.4.4), and the MP merges and refreshes it (Section 7.1.1).
- The bypass PATH carries a SESSION_ATTRIBUTE (owner decision 2026-10-09).

## Data Flow

### Entry Point
- Config: `rsvp-te bypass <name>` and a protected tunnel; a netlink link-down event on the protected interface.

### Transformation Path
1. `register.go::setupBypass` builds the bypass PSB, including `bypassSessionAttr`, and sends its PATH.
2. On link-down, `frr.go::tryLocalRepair` picks `repairSender`, builds `backupPath`, and sends it through the bypass.
3. The MP's RESV arrives; `repairedLSP` maps it back to the protected LSP when `samePeer` holds.
4. At an MP, `mergeBackupPath` joins the backup sender to the protected PSB, and `refreshPaths` refreshes it downstream.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze PLR to freeRtr MP | RSVP PATH inside the bypass LSP (MPLS), RESV back | Yes, `plr-backup-path-to-egress-merge-point` |
| rsvp-te to OSPF | `samePeer` reads the live OSPF database | Yes, same scenario (ResvErr code 4 without OSPF, scratch run5.log) |

### Integration Points
- `fib-kernel` - programs the two-label backup stack (`refreshBackupForwarding`).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the backup PATH goes through `sendPath` on the bypass |
| No unintended coupling (components stay isolated) | Yes | no new import outside `internal/plugins/rsvpte` |
| No duplicated functionality (extends existing, does not recreate) | Yes | `bypassSessionAttr` reuses `encodeSessionAttr` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | one config-time allocation per bypass |
| Registration over hardcoding, outbound | Yes | scenario check registered in `internal/le/interoplab/rsvpte/register.go` |
| Registration over hardcoding, inbound | Yes | no central list learns the scenario name; `interoplab.Discover` reads the directory |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | freeRtr accepts a Ze bypass PATH once it carries a SESSION_ATTRIBUTE | freeRtr `packRsvp.parseSesAtr` | no interop proof | scenario green, scratch run6.log and run8-green.log | confirmed |
| A-2 | freeRtr, as MP, answers the backup PATH with a RESV the PLR accepts | RFC 4090 Section 6.4.3 | AC-7 red | green after OSPF was added; ResvErr code 4 without it (run5.log) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The lab fails for a lab reason (OSPF not Full) and reads as a Ze failure | the checker error names the OSPF wait | `waitOSPFFull` runs before the link is cut and reports the last `show ospf neighbor` answer |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | FRR local repair: the protected LSP loses its backup signaling |
| How is it reverted? | single commit revert |
| Who else touches this path? | the RSVP-TE bypass bandwidth protection spec |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config bypass and protected tunnel, link down | → | `tryLocalRepair`, `backupPath`, `mergeBackupPath` | `TestEngineZeToZeFRRLocalRepair`, `TestRFC4090ProtectedPathSurvivesRepair` |
| config bypass, freeRtr MP | → | `setupBypass`, `bypassSessionAttr`, `backupPath` | interop `plr-backup-path-to-egress-merge-point` (`checkBackupPathToMP`) |

## Acceptance Criteria

Written at closure (2026-10-08) from the requirement rows above. The completion
boundary is Main's validation: each row's producer, its tagged tests in both
polarities, and a discrimination record per polarity.

| AC ID | Requirement | Evidence owed |
|-------|-------------|---------------|
| AC-1 | RFC4090-6.1-1 | `TestRFC4090HeadEndUsesDistinctSender` (positive) and `TestRFC4090HeadEndRequiresAlternateSender` (negative) pass; both recorded in `rfc/discrimination/rfc4090.json` |
| AC-2 | RFC4090-6.4-1 | `TestRFC4090ProtectedPathSurvivesRepair` (positive) and `TestRFC4090NoBackupPathBeforeRepair` (negative) pass; both recorded |
| AC-3 | RFC4090-6.4-2 | same two units; both recorded |
| AC-4 | RFC4090-6.4-3 | same two units; both recorded |
| AC-5 | RFC4090-7.1-1 | `TestRFC4090ProtectedPathSurvivesRepair` (positive) and `TestRFC4090DifferentPathsDoNotMerge` (negative) pass; both recorded |
| AC-6 | RFC4090-7.1-2 | same two units; both recorded |
| AC-7 | interop (`ai/rules/interop-and-goal-validation.md`) | a scenario against another RSVP-TE implementation in which Ze's PLR signals the backup PATH to a merge point: `test/interop-rsvpte/scenarios/plr-backup-path-to-egress-merge-point` against freeRtr, checker `internal/le/interoplab/rsvpte/checkers.go::checkBackupPathToMP` |

### Validation status (2026-10-08)

AC-1 to AC-6 are met. The six positive and six negative units pass
(`go test -run 'TestRFC4090(HeadEndUsesDistinctSender|HeadEndRequiresAlternateSender|ProtectedPathSurvivesRepair|NoBackupPathBeforeRepair|DifferentPathsDoNotMerge)$' ./internal/plugins/rsvpte/`).
The positive 6.4-1 to 7.1-2 records already existed. On 2026-10-08
`./le rfc discriminate-record` added the two RFC4090-6.1-1 records, with
`frr.go::repairSender` as the producer. It also added the five negative records
that were missing for 6.4-1, 6.4-2 (`frr.go::tryLocalRepair`), 6.4-3
(`routing.go::resolveExplicitPath`), 7.1-1 (`frr.go::mergeBackupPath`) and
7.1-2 (`register.go::refreshPaths`). After these additions,
`./le rfc discriminate stem rfc4090` lists none of these rows as unproven or
stale.

AC-7 blocks closure. No interop scenario exercises RFC 4090 backup-path
signaling. `TestRSVPFreeRouterInterop` runs against freeRtr but configures no
bypass. `TestEngineZeToZeFRRLocalRepair` is ze-to-ze, so it is not another
implementation. Building one needs the freeRtr lab, which needs Docker. The
owner decides how the scenario is built or homed.

-> Decision (owner, 2026-10-09): option A. Ze's bypass tunnel PATH carries a
SESSION_ATTRIBUTE (RFC 3209 Section 4.7, C-Type 7): protection flags clear,
setup and holding priority 7 (the default of an ordinary tunnel), session name
the configured bypass name. Reason: freeRtr's `packRsvp.parseSesAtr` refuses a
PATH with no SESSION_ATTRIBUTE, and the PATH grammar permits the object
(RFC 3209 Section 3.1, "[ <SESSION_ATTRIBUTE> ]"; the decision as dictated cited
Section 4.3.1, which is ERO applicability). Producer:
`register.go::bypassSessionAttr`, called from `setupBypass`; unit
`TestBypassPathCarriesSessionAttribute` (red with the field removed, green
restored, 2026-10-09).

### Validation status, AC-7 (2026-10-09)

AC-7 is met by `plr-backup-path-to-egress-merge-point`: Ze ingress, Ze PLR,
freeRtr relay on the bypass, freeRtr egress as merge point. After `prot0` goes
down the egress captures, through the bypass, a labelled PATH whose sender and
RSVP_HOP are the PLR and whose ERO is `10.0.14.14` alone; the PLR captures at
least two labelled RESVs from the egress for its backup sender and sends no
ResvErr for the protected session.

freeRtr answers from its eth0 address 172.29.81.14, not from the merge point
10.0.14.14, and Ze accepts an alternate merge-point source only when a live
native IGP database ties it to the same node. The lab therefore runs OSPF
between the PLR and the egress (freeRtr router-id 10.0.14.14), and the checker
waits for a Full adjacency on the PLR (`show ospf neighbor`) before it cuts
`prot0`. -> Decision (main thread, 2026-10-09): Ze's acceptance rule is
unchanged; the lab supplies the identity proof a real deployment would. Without
OSPF the PLR refused the RESV with ResvErr code 4 (session scratch `run5.log`).

Red/green, run with `RSVPTE_INTEROP_SCENARIO=plr-backup-path-to-egress-merge-point
./le --name rsvpfrr test integration interop-rsvpte`, which rebuilds the Ze
image from the tree: green (`interop: 1 passed, 0 failed`, scratch `run6.log`);
forced red with `frr.go::backupPath` keeping the protected sender instead of
the PLR's (RFC 4090 Section 6.1): `FAIL: wait for freeRouter egress receives
the PLR's backup PATH timed out` after the OSPF wait passed (scratch
`run7-red.log`); restored, green again (scratch `run8-green.log`).

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures a bypass around a protected link, and the link fails | netlink link-down -> `tryLocalRepair` -> backup PATH through the bypass -> freeRtr MP RESV -> `repairedLSP` | `plr-backup-path-to-egress-merge-point` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4090HeadEndUsesDistinctSender`, `TestRFC4090HeadEndRequiresAlternateSender` | `internal/plugins/rsvpte/rfc4090_frr_bypass_test.go` | RFC4090-6.1-1 | pass |
| `TestRFC4090ProtectedPathSurvivesRepair`, `TestRFC4090NoBackupPathBeforeRepair`, `TestRFC4090DifferentPathsDoNotMerge` | `internal/plugins/rsvpte/rfc4090_frr_bypass_test.go` | RFC4090-6.4-1 to 7.1-2 | pass |
| `TestBypassPathCarriesSessionAttribute` | `internal/plugins/rsvpte/bypass_session_attr_test.go` | bypass SESSION_ATTRIBUTE | pass |
| `TestParseCaptureLabelled` | `internal/le/interoplab/rsvpte/rsvpte_test.go` | labelled capture parsing | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| bypass session name length | 0 to 64 octets encoded (`maxSessionName`) | 64 | N/A | truncated to 64 by `encodeSessionAttr` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestEngineZeToZeFRRLocalRepair` | `internal/plugins/rsvpte/interop_test.go` | four Ze engines repair a protected LSP | pass |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `plr-backup-path-to-egress-merge-point` | `test/interop-rsvpte/scenarios/` | freeRtr | a Ze PLR signals the backup PATH to a freeRtr MP, which answers it | pass (run6 green, run7 red, run8 green) |

## Files to Modify
- `internal/plugins/rsvpte/frr.go` - backup PATH, repair sender, merge
- `internal/plugins/rsvpte/register.go` - `setupBypass`, `bypassSessionAttr`, `refreshPaths`
- `internal/plugins/rsvpte/fsm.go` - PSB `SessionAttr` contract comment
- `internal/plugins/rsvpte/interop_test.go` - the ze-to-ze test's comment named no open-source peer
- `internal/le/interoplab/rsvpte/checkers.go`, `register.go`, `rsvpte.go`, `rsvpte_test.go` - scenario checker, MPLS capture
- `test/interop-rsvpte/run-freertr.sh` - the capture filter keeps MPLS
- `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md`, `docs/architecture/rsvpte/mpls-rsvp-te.md`, `docs/architecture/testing/interop.md`
- `rfc/short/rfc4090.md`, `rfc/discrimination/rfc4090.json`

## Files to Create
- `internal/plugins/rsvpte/bypass_session_attr_test.go` - bypass SESSION_ATTRIBUTE unit
- `test/interop-rsvpte/scenarios/plr-backup-path-to-egress-merge-point/` - the interop scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config added; bypass config exists |
| YANG validation constraints | N-A | no leaf added |
| YANG custom validators | N-A | no leaf added |
| CLI commands/flags | N-A | none added |
| CLI grammar (keyword before value) | N-A | none added |
| Editor autocomplete | N-A | none added |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output added |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Prometheus counters/metrics | N-A | none added |
| BGP family surface (new SAFI / capability / attribute) | N-A | RSVP-TE only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | feature level unchanged |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | `docs/guide/rsvp-te.md` speaks of the ordinary tunnel only, still true |
| 7 | Wire format changed? | Yes | `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` (bypass SESSION_ATTRIBUTE, ae7a5972a6); `docs/architecture/rsvpte/mpls-rsvp-te.md` (which head-end builds the object, closure) |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4090.md` Support coverage names the interop proof (closure) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (4a314524e7) |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `mpls-rsvp-te-fast-reroute.md` anchors `setupBypass, bypassSessionAttr`; `mpls-rsvp-te.md` anchors `buildPath` (sentence updated); `interop.md` carries the checker row |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: backup PATH and merge point** -- RFC 4090 producers and units (AC-1 to AC-6, discrimination 172a6971f3)
2. **Phase: bypass SESSION_ATTRIBUTE** -- ae7a5972a6
3. **Phase: interop** -- 4a314524e7

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has a producer and a test |
| Correctness | SESSION_ATTRIBUTE C-Type 7 layout per RFC 3209 Section 4.7.1 |
| Rule: interop-and-goal-validation | scenario forced red with the Ze image rebuilt |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| backup PATH producers | `go test -run TestRFC4090 ./internal/plugins/rsvpte/` |
| bypass SESSION_ATTRIBUTE | `go test -run TestBypassPathCarriesSessionAttribute ./internal/plugins/rsvpte/` |
| interop scenario | `ls test/interop-rsvpte/scenarios/plr-backup-path-to-egress-merge-point/` and the green run log |
| discrimination records | `grep RFC4090-6.1-1 rfc/discrimination/rfc4090.json` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a RESV from an unrelated source is not taken as the MP's (`repairedLSP`, `samePeer`) |
| Resource bounds | bypass session name bounded by `maxSessionName`, object length by `maxSessionAttrLen` |
| Secrets | the lab password hash is a test fixture |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Interop red at the OSPF wait | lab setup, not Ze's repair |
| Interop red at the backup PATH wait | `frr.go::backupPath` or `tryLocalRepair` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- freeRtr answers the backup PATH from its interface address, so the merge point's identity reaches the PLR only through the IGP.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The bypass PATH carries a SESSION_ATTRIBUTE | leave it absent | owner decision 2026-10-09; freeRtr refuses a PATH without it |
| The lab runs OSPF between PLR and MP | relax `samePeer` | the acceptance rule stays; the lab supplies the identity a deployment would |

## Known Limitations
- One-to-one detours and bandwidth-guaranteed bypasses are absent (`rfc/short/rfc4090.md`, Support remaining).

## RFC Documentation (Scope: protocol)

`bypassSessionAttr` quotes RFC 3209 Section 4.7.1; `backupPath`, `mergeBackupPath` and `repairSender` quote RFC 4090 Sections 6.4.3 and 6.4.4, 7.1.1, and 6.1.1.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes (owed by the main thread; another session holds it)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`

## Implementation Summary

### What Was Implemented
- RFC 4090 backup-path signaling at the PLR and merge-point processing at the MP: `frr.go::tryLocalRepair`, `backupPath`, `repairSender`, `repairedLSP`, `mergeBackupPath`, `register.go::refreshPaths` (earlier commits, discrimination 172a6971f3).
- The bypass PATH carries a SESSION_ATTRIBUTE: `register.go::bypassSessionAttr`, called from `setupBypass` (ae7a5972a6).
- Interop scenario `plr-backup-path-to-egress-merge-point` against freeRtr, checker `checkBackupPathToMP` with `waitOSPFFull` (4a314524e7).
- Closure: the PSB `SessionAttr` comment, two compound guards split in `checkBackupPathToMP`, the `mpls-rsvp-te.md` head-end sentence, the ze-to-ze test comment, and the RFC 4090 Support coverage cell.

### Bugs Found/Fixed
- None in the product. Review findings are in the Review Gate.

### Documentation Updates
- `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md`: bypass SESSION_ATTRIBUTE paragraph, anchor `register.go -- setupBypass, bypassSessionAttr` (ae7a5972a6).
- `docs/architecture/testing/interop.md`: scenario row and MPLS capture sentence (4a314524e7).
- `docs/architecture/rsvpte/mpls-rsvp-te.md`: "Only a head-end builds a SESSION_ATTRIBUTE", tunnel and bypass head-end both named, anchor `register.go -- bypassSessionAttr` added (closure).
- `rfc/short/rfc4090.md` Support coverage names the freeRtr interop proof and `frr.go` `backupPath` (closure); `./le rfc index-update` regenerated the ignored `docs/features/rfc-status.md`.
- `./le doc check verify`: FAILED on findings outside this change (wiki command catalog drift, six source anchors in api/commands.md, cli-commands.md, command-reference.md, config-editor.md, graceful-restart.md); no finding names an rsvpte file.

### Deviations from Plan
- The bypass SESSION_ATTRIBUTE was added for AC-7 by owner decision (2026-10-09); it was not in the original requirement table.
- The interop lab runs OSPF between the PLR and the MP so Ze's identity rule holds; Ze's rule is unchanged.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | the first AC-7 run expected the PLR to accept freeRtr's RESV with no IGP | Ze accepts an alternate MP source only through a live IGP database; it answered ResvErr code 4 | run5.log | OSPF added to the lab, decision recorded in AC-7 validation status |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RFC4090-6.1-1 | Done | `internal/plugins/rsvpte/frr.go` `repairSender` | |
| RFC4090-6.4-1 | Done | `internal/plugins/rsvpte/frr.go` `tryLocalRepair`, `engine.go` `sendPath` | |
| RFC4090-6.4-2 | Done | `internal/plugins/rsvpte/frr.go` `backupPath` | |
| RFC4090-6.4-3 | Done | `internal/plugins/rsvpte/frr.go` `backupPath` | |
| RFC4090-7.1-1 | Done | `internal/plugins/rsvpte/frr.go` `mergeBackupPath` | |
| RFC4090-7.1-2 | Done | `internal/plugins/rsvpte/register.go` `refreshPaths` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC4090HeadEndUsesDistinctSender`, `TestRFC4090HeadEndRequiresAlternateSender` | records in `rfc/discrimination/rfc4090.json` |
| AC-2 | Done | `TestRFC4090ProtectedPathSurvivesRepair`, `TestRFC4090NoBackupPathBeforeRepair` | recorded |
| AC-3 | Done | same two units | recorded |
| AC-4 | Done | same two units | recorded |
| AC-5 | Done | `TestRFC4090ProtectedPathSurvivesRepair`, `TestRFC4090DifferentPathsDoNotMerge` | recorded |
| AC-6 | Done | same two units | recorded |
| AC-7 | Done | interop `plr-backup-path-to-egress-merge-point` | green, forced red, green |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| RFC 4090 units | Done | `internal/plugins/rsvpte/rfc4090_frr_bypass_test.go` | |
| `TestBypassPathCarriesSessionAttribute` | Done | `internal/plugins/rsvpte/bypass_session_attr_test.go` | |
| `TestParseCaptureLabelled` | Done | `internal/le/interoplab/rsvpte/rsvpte_test.go` | |
| interop scenario | Done | `test/interop-rsvpte/scenarios/plr-backup-path-to-egress-merge-point/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/rsvpte/frr.go`, `register.go`, `fsm.go` | Done | |
| `internal/le/interoplab/rsvpte/*` | Done | |
| scenario directory | Done | |
| docs and `rfc/short/rfc4090.md` | Done | |

### Audit Summary
- **Total items:** 13
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A Ze PLR signals the protected PATH through the bypass to the merge point, per RFC 4090 Sections 6.1 and 6.4 | interop | `plr-backup-path-to-egress-merge-point` vs freeRtr: the egress captures a labelled PATH with sender and RSVP_HOP 172.29.81.3 and ERO 10.0.14.14/32 (run5.log capture); green run6.log and run8-green.log; red run7-red.log with `backupPath` keeping the protected sender |
| The merge point answers and the PLR keeps the repaired LSP | interop | same scenario: two labelled RESVs from the MP for the PLR's sender, no ResvErr for the protected session |
| MP merge and refresh (Section 7.1.1) | unit, ze-to-ze | `TestRFC4090ProtectedPathSurvivesRepair`, `TestRFC4090DifferentPathsDoNotMerge`, discrimination records in `rfc/discrimination/rfc4090.json` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC is met | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rsvpte-frr-backup-path-signaling-12d06ccf-2460-42c7-a707-30bb0a427796.md` |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | RFC 3209 Section 4.7.1 encoding, interop vacuity, lab-vs-Ze failure separation, stale comments and docs, Go style (compound guards) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | PSB `SessionAttr` comment said nil at the head-end; `setupBypass` now sets it for a bypass | `internal/plugins/rsvpte/fsm.go` | comment states the tunnel and bypass head-end cases |
| 2 | ISSUE | two terminating `a \|\| b` guards on changed lines (`./le arch compound-guard check`) | `internal/le/interoplab/rsvpte/checkers.go` `checkBackupPathToMP` | split into one guard per fact; check clean |
| 3 | ISSUE | doc said only the head-end builds the object from its protection request; a bypass builds its own | `docs/architecture/rsvpte/mpls-rsvp-te.md` | sentence and anchor updated |

### Run 2
0 BLOCKER, 0 ISSUE. NOTEs: `isBackupPath` proves the bypass crossing through RSVP_HOP and sender, not the MPLS label line (the run5 capture shows the label; a relay processing the PATH would rewrite RSVP_HOP); `answers` matches addresses by prefix, safe in the lab's address plan; `repairSender` picks among several alternates in map order, stored once per repair; the `frr.go` `maxSessionAttrLen` comment cites Section 4.7.1 for the C-Type 1 layout, which is Section 4.7.2 (predates this spec); the test file uses testify like the rest of the package.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/interop-rsvpte/scenarios/plr-backup-path-to-egress-merge-point/` | Yes | ls: egress-hw.txt egress-sw.txt ingress.conf ingress-setup.sh relay-hw.txt relay-sw.txt transit.conf transit-setup.sh |
| `internal/plugins/rsvpte/bypass_session_attr_test.go` | Yes | read at closure |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-6 | units pass | `go test -count=1 ./internal/plugins/rsvpte/` ok (closure, under `./le job run`) |
| AC-7 | scenario passes and discriminates | `interop: 1 passed, 0 failed` (run8-green.log); checker package `go test -count=1 ./internal/le/interoplab/rsvpte/` ok after the guard split |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| config bypass, freeRtr MP | interop scenario `plr-backup-path-to-egress-merge-point` (no .ci; interop lab) | Yes, `transit.conf` configures `bypass around-prot0`, the checker cuts prot0 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | run6.log, run8-green.log |
| A-2 | confirmed | green with OSPF; ResvErr code 4 without it (run5.log) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| bypass SESSION_ATTRIBUTE paragraph | `register.go` `bypassSessionAttr`: priorities `defaultLSPPriority`, no flag, bypass name | Yes |
| head-end builds SESSION_ATTRIBUTE | `build.go` `buildPath` encodes `psb.SessionAttr` or `pr.sessionAttr()` | Yes |
| RFC 4090 Support coverage | `frr.go` `backupPath`, scenario green | Yes |
