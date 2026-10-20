# Spec: dhcpv6-client-2-confirm

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | spec-dhcpv6-client-1-gap-annotations (closed 2026-10-19) |
| Phase | 2/4 of `plan/spec-dhcpv6-client-0-umbrella.md` |
| Handoff | - |
| Updated | 2026-10-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

When the DHCPv6 client may have moved to a new link and its lease holds
addresses and no delegated prefix, RFC 9915 Section 18.2.12 says "the client
SHOULD initiate a Confirm/Reply message exchange". Ze sends nothing in that
case today (`noteMove`, `dhcp_v6_attach_linux.go`): the addresses stay until
their lifetimes run out, even on a link where they are wrong. The owner put
Confirm in scope on 2026-10-19. This child sends Confirm, keeps the lease on a
Success Reply, discovers again when every Reply says NotOnLink, and keeps the
lease when no server answers. It closes the four `{gap}` rows
RFC9915-18.2.3-1, RFC9915-18.2.3-2, RFC9915-18.2.3-3, RFC9915-18.2.12-1.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - DHCPv6 move paragraph
  → Constraint: the sentence "A lease of addresses alone sends nothing on a move, because Ze does not send Confirm" is rewritten in the same change
  → Constraint: a move re-installs every leased address with its remaining lifetimes and re-runs DAD; Confirm runs after that, never instead of it

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9915.md` - RFC9915-18.2.3-1..3, RFC9915-18.2.12-1, RFC9915-18.2.12-4, RFC9915-16.5-1
  → Constraint: RFC9915-18.2.12-4 rate limit (at most one move exchange every 30 s) already holds for the Rebind; Confirm shares the same `moveLast`
  → Constraint: RFC9915-16.5-1 (clients discard a received Confirm) is unaffected; Confirm is only sent
- [ ] `rfc/full/rfc9915.txt` - Sections 18.2.3, 18.2.10.3, 18.2.12, 7.6, Appendix B
  → Constraint: Section 18.2.3 "The client MUST include a Client Identifier option", "MUST include an Elapsed Time option", "The first Confirm message from the client on the interface MUST be delayed by a random amount of time between 0 and CNF_MAX_DELAY", IRT CNF_TIMEOUT 1 s, MRT CNF_MAX_RT 4 s, MRC 0, MRD CNF_MAX_RD 10 s
  → Constraint: Section 18.2.3 "The client SHOULD set the T1 and T2 fields in any IA_NA options ... and the preferred-lifetime and valid-lifetime fields in the IA Address options ... to 0"
  → Constraint: Appendix B: Confirm carries Client ID, IA_NA and Elapsed Time; no Server ID, no IA_PD, no Option Request option
  → Constraint: Section 18.2.10.3 "If the client receives any Reply messages that indicate a status of Success (explicit or implicit), the client can use the addresses in the IA and ignore any messages that indicate a NotOnLink status. When the client only receives one or more Reply messages with the NotOnLink status in response to a Confirm message, the client performs DHCP server discovery"
  → Constraint: Section 18.2.3 "If the client receives no responses before the message transmission process terminates ... the client SHOULD continue to use any leases, using the last known lifetimes"

**Key insights:**
- The Confirm constants `v6CnfMaxDelay`, `v6CnfTimeout`, `v6CnfMaxRT`, `v6CnfMaxRD` already exist and today serve only the link-change Rebind.
- A Confirm answer may come from several servers; the exchange must not stop at a NotOnLink Reply while a Success can still arrive within the window, which is the Advertise collector's shape (`v6AdvertiseCollector`, `windowClosed`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` - `linkMoved` (down then up is a move), `noteMove` (re-install, re-queue DAD, Rebind owed only with a prefix), `moveOwed`, `rebindAfterMove` (CNF_MAX_DELAY wait, Confirm parameters)
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go` - `runExchange`, `v6Exchange.windowClosed`, `replyMatcher`, `v6Status`, `setLeaseIAs`
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - `maintain` loop dispatch on `moveOwed`, `drop`, `expireDue`
- [ ] `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go` - `v6Rig.move`, the "move without a prefix" case asserting nothing is sent

**Behavior to preserve:**
- a lease holding a delegated prefix sends the link-change Rebind, never Confirm (RFC9915-18.2.12-2)
- the 30 s rate limit across every move exchange
- DAD re-run and address re-install on every move

**Behavior to change:**
- a move with an addresses-only lease owes a Confirm (the "move without a prefix" test case changes from "sends nothing" to "sends one Confirm")

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `iface.LinkEvent` down then up on the worker's subscription.

### Transformation Path
1. `linkMoved` reports a move; `noteMove` re-installs the addresses and sets `moveDue` for both lease shapes.
2. `maintain` sees `moveOwed`; a lease with a live prefix runs `rebindAfterMove`, an addresses-only lease runs the new Confirm exchange.
3. The Confirm exchange waits RAND times CNF_MAX_DELAY, builds the message, runs `runExchange` with a collector that answers at the first Success Reply or at the end with NotOnLink.
4. Success keeps the lease; NotOnLink only drops every address through the existing drop and expiry path (removal, `dhcp-expired` per binding) and returns to `acquire`; exhaustion keeps the lease.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin to wire | `v6Link.exchange` multicast to ff02::1:2 | No |
| Plugin to iface | address removal through the existing drop path | No |

### Integration Points
- `runExchange` with `windowClosed` - collector shape reused.
- `moveDue` and `moveLast` - one rate limit for Rebind and Confirm.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Confirm runs on `runExchange` |
| No unintended coupling (components stay isolated) | Yes | plugin-internal |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses the move scheduling and the drop path |
| Zero-copy preserved where applicable (refs, not copies) | Yes | the message is built once and updated per transmission like every exchange |
| Registration over hardcoding, outbound | N-A | no registry |
| Registration over hardcoding, inbound | Yes | `v6Status` and `replyMatcher` are the plugin's own; no foreign list learns Confirm |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Kea 2.6.2 answers a Confirm with Success when the addresses fit its subnet and NotOnLink when they do not | `pkt6-confirm-received` in the image; RFC 9915 Section 18.3.3 | interop needs another peer | `dhcpv6-client-confirm-kea` | confirmed 2026-10-20: Kea 2.6.2 (`kea-dhcp6 -V`) answered the replug CONFIRM with `Success(0) "All addresses are on-link"` and, after the subnet reload, `NotOnLink(4) "Address fd00:1e:1::50 is not on link."` (DHCP6_RESPONSE_DATA, read by `kea6ReplugVerdict` / `kea6MovedVerdict`) |
| A-2 | a Confirm Reply passes `replyMatcher` as it stands (transaction id, Client ID, Server ID present) | Section 16.10 applies to every Reply | the matcher needs a Confirm branch | `TestRFC9915Section18212ConfirmOnMove` | broken, design holds: `replyMatcher` accepts a Success Confirm Reply but discards a NotOnLink one as a failure status, so Confirm runs on `v6ConfirmCollector.accept` (`dhcp_v6_exchange_linux.go`), which applies the same `v6Identified` check; `TestRFC9915Section182103ConfirmNotOnLinkRediscovers` and `...ConfirmMixedReplies` pass (2026-10-20) |
| A-3 | a Reply with no Status Code option is Success (implicit) | Section 18.2.10.3 "(explicit or implicit)"; Section 21.13 | NotOnLink handling misfires | `TestRFC9915Section182103ConfirmReplySuccessImplicit` | confirmed: `v6Status` answers Success when the Status Code option is absent; the `implicit` subtest keeps the lease with one Confirm (2026-10-20) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a NotOnLink Reply arriving first ends the exchange and drops a lease another server confirms | the mixed-reply test | collector: Success ends at once, NotOnLink is held until the window or MRD ends |
| R-2 | the Confirm wait blocks a lease expiry or Stop | rig timing tests | the CNF_MAX_DELAY wait uses `waitUntil(w.ctx, ...)` like the Rebind |
| R-3 | a down without an up drops nothing but a Confirm is owed on a link that never returns | rig | Confirm is scheduled only on the up, as the Rebind is |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | an interface loses its DHCPv6 addresses after a cable replug, or keeps wrong ones |
| How is it reverted? | single commit revert |
| Who else touches this path? | child 3 (Information-request on a move of a stateless worker) edits `noteMove` too |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| link down then up, addresses-only lease | → | Confirm exchange from `maintain` | `TestRFC9915Section18212ConfirmOnMove` |
| link down then up against a real server | → | Confirm and Reply on the wire | `dhcpv6-client-confirm-kea` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | move, lease of IA_NA addresses, no live prefix | one Confirm is sent, first transmission after a delay drawn in [0, 1 s] (RFC9915-18.2.3-3) |
| AC-2 | that Confirm | carries one Client Identifier with the worker's DUID (RFC9915-18.2.3-1), one Elapsed Time updated per retransmission (RFC9915-18.2.3-2), every IA_NA of the lease with every live address of it (RFC9915-18.2.12-1), T1, T2 and both lifetimes 0, and no Server Identifier, IA_PD or Option Request option |
| AC-3 | no answer | retransmits on IRT 1 s, MRT 4 s, stops at MRD 10 s, then keeps every binding with its last lifetimes and sends nothing more |
| AC-4 | a Reply with Success, explicit or with no Status Code | lease and addresses unchanged, no Solicit |
| AC-5 | only NotOnLink Replies until the exchange ends | every leased address removed, one `dhcp-expired` per binding, the worker solicits |
| AC-6 | a NotOnLink Reply then a Success Reply within the exchange | lease kept, as AC-4 |
| AC-7 | two moves 10 s apart | the second Confirm starts no earlier than 30 s after the first (RFC9915-18.2.12-4) |
| AC-8 | move with a live delegated prefix | the link-change Rebind runs as today, no Confirm (RFC9915-18.2.12-2) |
| AC-9 | Stop during the Confirm delay or exchange | the worker ends without waiting the exchange out |
| AC-10 | the four rows | no `{gap}`; each carries a positive and a negative tagged test with a discrimination record; `Support remaining` count lowered by four and Confirm removed from its list |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | replugs the cable on the same link | link event, Confirm, Success Reply | `dhcpv6-client-confirm-kea` (same subnet phase) |
| 2 | moves the box to another link | link event, Confirm, NotOnLink, Solicit, new address | `dhcpv6-client-confirm-kea` (renumbered phase) |

### Planned Goal Validation
<!-- The evidence each goal owes at closure (ai/rules/interop-and-goal-validation.md); /ze-close copies it into the closure table. -->
| Goal | Evidence owed |
|------|---------------|
| ze keeps a valid lease across a replug on the same link | `dhcpv6-client-confirm-kea` phase 1 passing against Kea: one CONFIRM in Kea's log, no SOLICIT, the /128 present on every 2 s sample |
| ze leaves a link whose addresses no longer fit | the same scenario's phase 2: the old address gone, an address of the new subnet leased, and the recorded red with the Confirm branch reverted |
| the four RFC rows are met | discrimination records for RFC9915-18.2.3-1..3 and RFC9915-18.2.12-1 accepted by `./le rfc check` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC9915Section18212ConfirmOnMove` | `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go` | AC-1, AC-2: delay bound, contents, and (negative) no Confirm with a prefix | |
| `TestRFC9915Section1823ConfirmRetransmission` | same | AC-3 | |
| `TestRFC9915Section182103ConfirmReplySuccessImplicit` | same | AC-4 | |
| `TestRFC9915Section182103ConfirmNotOnLinkRediscovers` | same | AC-5 | |
| `TestRFC9915Section182103ConfirmMixedReplies` | same | AC-6 | |
| `TestRFC9915Section18212ConfirmRateLimit` | same | AC-7 | |
| `TestDHCPv6ConfirmStopDuringDelay` | same | AC-9 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| first Confirm delay | 0 to CNF_MAX_DELAY 1 s | 1 s | N/A | above 1 s |
| Confirm MRD | 10 s | last transmission before 10 s | N/A | no transmission at or after 10 s |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | no config surface; the interop scenario drives the real binary end to end | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dhcpv6-client-confirm-kea` | `test/interop/scenarios/` | Kea 2.6.2 (`test/interop/Dockerfile.kea`) | phase 1: ze leases an IA_NA address, the checker sets ze's eth0 down then up, Kea logs one CONFIRM and ze keeps the address with no SOLICIT; phase 2: the checker rewrites Kea's subnet and reloads it, sets eth0 down then up, Kea answers NotOnLink, ze removes the old address and leases one in the new subnet. Red phase: with the Confirm branch reverted, phase 2 fails because the old address stays | green 2026-10-20 (`pass (48s)`, again `pass (52s)` after the red walk); red A (Confirm dispatch skipped in `maintain`): assertion 3 "ze ran 0 CONFIRM exchanges before the subnet moved, want 1"; red B (`dropAddresses` skipped on NotOnLink): phase 1 passed, assertion 5 "ze's eth0 still holds fd00:1e::50/128 after the move" |

## Files to Modify
- `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` - `noteMove` schedules the move exchange for both lease shapes; the Confirm exchange beside `rebindAfterMove`
- `internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go` - Confirm builder and Reply collector
- `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - `maintain` dispatch between Rebind and Confirm
- `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go` - "move without a prefix" case expects a Confirm
- `docs/features/interfaces.md` - move paragraph and source anchors
- `rfc/short/rfc9915.md`, `docs/features/rfc-status.md` - four rows, Meta
- `features/interfaces.md` - DHCPv6 evidence
- `docs/architecture/testing/interop.md` - Kea row names the new scenario

## Files to Create
- `test/interop/scenarios/dhcpv6-client-confirm-kea/` - `kea-dhcp6.conf`, `ze.conf`
- the scenario checker beside `internal/le/interoplab/bgp/check_dhcpv6.go`
- `rfc/discrimination/rfc9915.json` entries for the four rows

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Confirm needs no config |
| YANG validation constraints | No | none |
| YANG custom validators | No | none |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | No | same socket |
| Prometheus counters/metrics | No | none in this client |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/interfaces.md` |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | same plugin |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | Confirm format lives in `rfc/short/rfc9915.md` |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9915.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `dhcp-expired` already exists |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/interfaces.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none for moves |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- change the "move without a prefix" rig case to expect one Confirm and add `TestRFC9915Section18212ConfirmOnMove`; both fail.
2. **Phase: Confirm exchange** -- builder, collector, `maintain` dispatch; AC-1 to AC-9 tests fail then pass.
3. **Phase: Interop** -- scenario and checker; record the red with Confirm reverted.
4. **Phase: Ledger and docs** -- tags, `./le rfc discriminate-record` per row, Meta, pages.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has an implementation at file:line |
| Correctness | NotOnLink never ends the exchange before its window when a Success may still arrive |
| Data flow | address removal goes through the existing drop path, never a new one |
| Rule: rfc-compliance | the Confirm builder quotes Section 18.2.3; the call in `maintain` names Section 18.2.12 |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Confirm on move | `TestRFC9915Section18212ConfirmOnMove` passes |
| four rows closed | `grep -E "RFC9915-18.2.3-[123]\|RFC9915-18.2.12-1" rfc/short/rfc9915.md` shows no `{gap` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a NotOnLink Reply from an unauthenticated server can drop addresses; this is what RFC 9915 prescribes, and the Reply must still pass `replyMatcher` (transaction id, Client ID) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| collect Replies until Success or the window ends | take the first Reply | Section 18.2.10.3 lets any Success win over NotOnLink |
| NotOnLink drops through the existing expiry path | a dedicated removal | one teardown path, one event per binding |
| T1, T2 and lifetimes 0 in the Confirm | send the lease's values | Section 18.2.3 SHOULD, and the server ignores them |

## Known Limitations
- Confirm after a ze restart is impossible: ze keeps no lease state across restarts (umbrella Known Limitations).
- The "significant change regarding the prefixes available on the link" trigger of Section 18.2.12 (a SHOULD) is not detected; only link down then up is a move, as today.

## RFC Documentation (Scope: protocol)

Add `// RFC 9915 Section 18.2.3: "..."` above the builder and the delay, `// RFC 9915 Section 18.2.10.3: "..."` above the Reply decision, and `// RFC 9915 Section 18.2.12` at the call in `maintain`.

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-dhcpv6-client-2-confirm.md` only, in the same `./le commit create` script

## Owner Decisions (2026-10-20)

| Decision | Owner's words | Where it acts |
|----------|---------------|---------------|
| WIP cap raised to claim this spec | "start" | `./le spec claim` |
| Edit of RFC-tagged `TestRFC9915Section18212NoMoveNoRebind`: the "move without a prefix" case expects one Confirm | "1 approve" | `./le rfc approve`, `tmp/commit-rfc-approved-19f2971a.md`; the no-Rebind assertion and the tag claim are unchanged |
| RFC9915-18.2.3-1 and -2 carry `{single-polarity: positive}` instead of AC-10's positive and negative pair | "2 accept" | no input can make the client omit its own Client ID or Elapsed Time; the same reading as 18.2.1-1/2, 18.2.4-5/6, 18.2.7-2/3, 18.2.8-3/4 |
| No full verify: `./le verify worktree`, `./le test unit all`, `./le test functional gating` stay verification debt | standing ruling, `plan/handover/07-session-2026-10-19-vyos-followups.md` | Goal Gates row "`./le verify worktree` passes" is that debt |

## Implementation Summary

### What Was Implemented
- `noteMove` schedules the move exchange for every lease shape; `maintain` calls `startMoveExchange` (one 30 s `moveLast` for both shapes), then `rebindAfterMove` with a live delegated prefix, else `confirmAfterMove` (`internal/plugins/iface/dhcp/dhcp_v6_linux.go`, `dhcp_v6_attach_linux.go`).
- `confirmAfterMove`: RAND times CNF_MAX_DELAY wait on `w.ctx`, `buildConfirm` and `setConfirmIAs` (Client ID, Elapsed Time, one IA_NA with every live address, T1/T2/lifetimes 0, no Server ID, IA_PD or ORO), `runExchange` on IRT 1 s / MRT 4 s / MRD 10 s cut at the last expiry, `v6ConfirmCollector` (Success at once, NotOnLink held to the window's close, other status no answer); Success or no answer keeps the lease, NotOnLink runs `dropAddresses` (removal plus one `dhcp-expired` per binding through `drop`), so `maintain` finds the lease empty and the worker solicits.
- Interop scenario `dhcpv6-client-confirm-kea` with checker `checkDHCPv6ConfirmKea` and its predicates; `Dockerfile.kea` runs Kea on a copy so SIGHUP re-reads a rewritten config; `prepare.go` mounts `kea-dhcp6-moved.conf`.
- Ledger: four `{gap}` rows closed, Meta `Support remaining` 23 to 19 and Confirm removed from its list, `Support coverage` names the Confirm; ten discrimination records added or re-recorded in `rfc/discrimination/rfc9915.json`.

### Bugs Found/Fixed
- None in shipped code. The closure review fixed one style finding (below).

### Documentation Updates
- `docs/features/interfaces.md` move paragraph rewritten; anchors add `startMoveExchange`, `confirmAfterMove`, `dropAddresses`, `v6ConfirmCollector`, `buildConfirm`, `setConfirmIAs`.
- `docs/architecture/testing/interop.md` Kea row: copy-on-start, the moved config, the confirm scenario's two phases. The same file also drops a duplicated `Docker.RemoveContainer` anchor beside the `startsettle.go` anchor (the next anchor line already names it); that hunk came from another session's tree and is prose only.
- `features/interfaces.md`: `Interop` row added, Doc review redone 2026-10-20. `features/ipv6-router-advertisements.md`: Doc review redone 2026-10-20 (its page is `docs/features/interfaces.md`; only the DHCPv6 paragraph changed).
- `docs/features/rfc-status.md` regenerated by `./le rfc index-update` (generated, untracked).
- `./le doc check verify`: one failure, the `docs/architecture/hub-architecture.md` anchor naming `caseInEffect` in `internal/component/config/yang/validator_default.go`, another session's uncommitted config work; no failure names a file of this spec.

### Deviations from Plan
- R-1 mitigation said a NotOnLink is held "until the window or MRD ends"; the collector holds it to the close of the listen window it arrived in (the Advertise collector's shape). A Success in that window still wins (`TestRFC9915Section182103ConfirmMixedReplies`); RFC 9915 Section 18.2.10.3 names no collection period.
- A-2 broke: Confirm runs on its own collector, not `replyMatcher`.
- AC-10 for 18.2.3-1/-2: single-polarity positive (owner "2 accept").
- `TestRFC9915Section18212NoMoveNoConfirm` (18.2.12-1 negative) and the 18.2.3-3 negative tag on `TestDHCPv6ConfirmStopDuringDelay` were added beyond the TDD plan, to give AC-10 its negatives.
- The 18.2.1-6 positive record moved its producer from `windowClosed` to `v6Preference`, because the new `v6ConfirmCollector.windowClosed` made the bare name ambiguous in `dhcp_v6_exchange_linux.go`.
- Files to Modify named `docs/features/rfc-status.md`; it is generated and untracked, so it is regenerated, not committed.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2: a Confirm Reply passes `replyMatcher` as it stands | `replyMatcher` accepts only Success, so a NotOnLink Confirm Reply is discarded as a failure status and the exchange would retransmit to MRD and keep the lease | phase 1-2 implementation, writing `TestRFC9915Section182103ConfirmNotOnLinkRediscovers` | Confirm uses `v6ConfirmCollector.accept`, which applies the same `v6Identified` check and holds NotOnLink |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| send Confirm on a move of an addresses-only lease | Done | `dhcp_v6_linux.go` `maintain` move branch; `dhcp_v6_attach_linux.go` `confirmAfterMove` | |
| keep the lease on Success | Done | `confirmAfterMove`, `v6Status != NotOnLink` branch | |
| discover again when every Reply says NotOnLink | Done | `confirmAfterMove` -> `dropAddresses`; `maintain` `lease.empty()` | |
| keep the lease when no server answers | Done | `confirmAfterMove`, `out.end == v6EndExhausted` | |
| close the four `{gap}` rows | Done | `rfc/short/rfc9915.md` 18.2.3-1..3, 18.2.12-1 | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC9915Section18212ConfirmOnMove/addresses_only` | delay 0.5 s at RAND 0.5 |
| AC-2 | Done | same | contents and absent options |
| AC-3 | Done | `TestRFC9915Section1823ConfirmRetransmission` | |
| AC-4 | Done | `TestRFC9915Section182103ConfirmReplySuccessImplicit` | explicit and implicit |
| AC-5 | Done | `TestRFC9915Section182103ConfirmNotOnLinkRediscovers` | |
| AC-6 | Done | `TestRFC9915Section182103ConfirmMixedReplies` | both orders |
| AC-7 | Done | `TestRFC9915Section18212ConfirmRateLimit` | [5s 35s] |
| AC-8 | Done | `TestRFC9915Section18212ConfirmOnMove/prefix_held`, `TestRFC9915Section18212MoveRebindsWithConfirmParameters` | |
| AC-9 | Done | `TestDHCPv6ConfirmStopDuringDelay` | delay and exchange |
| AC-10 | Changed | `rfc/short/rfc9915.md`, `rfc/discrimination/rfc9915.json` | 18.2.3-1/-2 single-polarity, owner "2 accept" |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRFC9915Section18212ConfirmOnMove` | Done | `rfc9915_dhcp_v6_attach_linux_test.go` | |
| `TestRFC9915Section1823ConfirmRetransmission` | Done | same file | |
| `TestRFC9915Section182103ConfirmReplySuccessImplicit` | Done | same file | |
| `TestRFC9915Section182103ConfirmNotOnLinkRediscovers` | Done | same file | |
| `TestRFC9915Section182103ConfirmMixedReplies` | Done | same file | |
| `TestRFC9915Section18212ConfirmRateLimit` | Done | same file | |
| `TestDHCPv6ConfirmStopDuringDelay` | Done | same file | |
| `dhcpv6-client-confirm-kea` | Done | `test/interop/scenarios/dhcpv6-client-confirm-kea/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` | Done | |
| `internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go` | Done | |
| `internal/plugins/iface/dhcp/dhcp_v6_linux.go` | Done | |
| `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go` | Done | |
| `docs/features/interfaces.md` | Done | |
| `rfc/short/rfc9915.md` | Done | |
| `docs/features/rfc-status.md` | Changed | generated, untracked |
| `features/interfaces.md` | Done | |
| `docs/architecture/testing/interop.md` | Done | |
| scenario dir, checker, `rfc/discrimination/rfc9915.json` | Done | plus `Dockerfile.kea`, `prepare.go`, `register_dhcp.go`, `check_dhcpv6*.go` |

### Audit Summary
- **Total items:** 33
- **Done:** 31
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (AC-10 single-polarity, owner-approved; rfc-status.md generated)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| ze keeps a valid lease across a replug on the same link | interop | `dhcpv6-client-confirm-kea` phase 1 against Kea 2.6.2: `pass (48s)` and `pass (52s)`; Kea answered the CONFIRM `Success(0) "All addresses are on-link"`, no second SOLICIT, the /128 on every 2 s sample for 20 s. Red A (Confirm dispatch skipped in `maintain`): "assertion 3: ze ran 0 CONFIRM exchanges before the subnet moved, want 1" |
| ze leaves a link whose addresses no longer fit | interop | the same scenario's phase 2: Kea answered `NotOnLink(4)`, ze dropped `fd00:1e::50` and leased `fd00:1e:0:1::50`. Red B (`dropAddresses` skipped): phase 1 passed, "assertion 5: ze's eth0 still holds fd00:1e::50/128 after the move" |
| the four RFC rows are met | ledger | `./le rfc discriminate stem rfc9915`: `stale` and `unproven` empty (closure run, after the comment edit); `./le rfc check` names no rfc9915 violation. The two negative revert records (18.2.12-1, 18.2.12-2) are corroborated by a hand mutant of `linkMoved` (a bare up counted as a move): `NoMoveNoConfirm` red at "CONFIRM at [5s] with no move to answer", `NoMoveNoRebind/up_without_down` red |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None: every Task item and AC is done | - | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/dhcpv6-client-2-confirm-dc676f37-dfa7-4fc5-82f7-41a7eda9292e.md` (22 files, verdict clean) |
| `./le spec review check` | clean: "OK (12 code files, clean, hashes match)" |
| Rounds | 2 |
| Reviewer lenses used | wiring and data flow, removed-behavior audit, RFC 9915 18.2.3/18.2.10.3/18.2.12 against the code, security of the NotOnLink path, style pass (six questions) over the three changed Go files and the checker, discrimination soundness, doc and feature-row freshness |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Style, "A caller obligation: MUST, on both sides of the pair": `startMoveExchange` says maintain MUST call it before `rebindAfterMove`/`confirmAfterMove`, and neither callee said so | `dhcp_v6_attach_linux.go` `rebindAfterMove`, `confirmAfterMove` doc comments | both now say "It MUST be called after startMoveExchange." (comment only; discrimination stays fresh) |
| 2 | ISSUE | `./le feature check` refused `features/interfaces.md` and `features/ipv6-router-advertisements.md`: Doc review 2026-10-07 older than the 2026-10-20 page change | both feature files | Doc review redone 2026-10-20 against `confirmAfterMove`, `dropAddresses`, `v6ConfirmCollector`, the RA leaf and counters; `./le feature check` no longer names either |

### NOTEs
- The 18.2.12-1 and 18.2.12-2 negative records use the revert route: the red comes from the halt in `linkMoved`, which proves the unit reaches the producer, not that the negative assertion discriminates. `docs/contributing/rfc-conformance-gates.md` ("Producing a record: the two proof routes") admits a halt that names the unit, so the gate is satisfied; the hand mutant in Goal Validation shows the assertions themselves discriminate.
- `./le commit audit` reports the test file `[WEAKENED]`: the change is the owner-approved `NoMoveNoRebind` edit plus the `awaitRebind` helper now calling `awaitSentOf` (same wait); no other tagged body changed.
- R-1 holding reading recorded under Deviations.

### Run 2
Re-read the two comment edits and both feature rows: 0 BLOCKER, 0 ISSUE.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/interop/scenarios/dhcpv6-client-confirm-kea/` | Yes | `ze.conf`, `kea-dhcp6.conf`, `kea-dhcp6-moved.conf` |
| `internal/le/interoplab/bgp/check_dhcpv6_confirm.go` and its predicate and test | Yes | `ls internal/le/interoplab/bgp/check_dhcpv6_confirm*` three files |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-9 | the seven Confirm tests plus the Rebind tests pass | closure run `go test -count=1 -v -run 'Confirm\|NoMoveNo\|MoveRebind'`: all eleven `--- PASS`; `go test -race -count=1 ./internal/plugins/iface/dhcp/... ./internal/le/interoplab/bgp/` ok |
| AC-10 | no `{gap` on the four rows | `grep -E "RFC9915-18.2.3-[123]\|RFC9915-18.2.12-1\]" rfc/short/rfc9915.md`: none carries `{gap` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| link down then up, addresses-only lease | interop `dhcpv6-client-confirm-kea` (no `.ci`: no config surface) | `replugZe` runs `ip link set dev eth0 down/up` in the ze container; assertion 3 counts Kea's CONFIRM |
| link event into `maintain` | `TestRFC9915Section18212ConfirmOnMove` | rig `move` sends LinkDown then LinkUp on the subscription; `maintain` -> `confirmAfterMove` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Kea 2.6.2 Success then NotOnLink in the interop logs |
| A-2 | broken | Mistake Log row; `v6ConfirmCollector` |
| A-3 | confirmed | `v6Status` returns Success when the option is absent; `ConfirmReplySuccessImplicit/implicit` passes |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| move paragraph, `docs/features/interfaces.md` | `confirmAfterMove`, `dropAddresses`, `v6ConfirmCollector.windowClosed` | Yes |
| Kea row, `docs/architecture/testing/interop.md` | `Dockerfile.kea` ENTRYPOINT, `prepare.go` `keaMovedConfigTarget`, `checkDHCPv6ConfirmKea` | Yes |
| RFC Meta, `rfc/short/rfc9915.md` | Nineteen MUST gaps: 23 minus the four closed rows | Yes |
| feature rows | `./le feature check` names neither feature file | Yes |
