# Spec: home-cpe-22-ike-responder-reauth

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | An `initiate` peer drops a reauthentication started by the other side (RFC 7296 Section 2.8.3 lets either party create the new IKE SA). The tunnel then breaks until the old SA is deleted and Ze reconnects. Interop behaviour with strongSwan gateways that schedule reauthentication; low impact because Ze reconnects within the backoff. |
| RFC gaps | - |
| Depends | - |
| Phase | 0/2 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

When the gateway, as responder of the original IKE SA, reauthenticates by sending
a fresh IKE_SA_INIT to the CPE, Ze drops it: `matchResponderPeer` considers only
`respond` peers. The gateway's reauthentication fails, it deletes the old IKE SA
(or lets it expire), and Ze's initiator loop reconnects from scratch: an outage
of one backoff step plus a full handshake, every reauthentication interval.

Accept, on an `initiate` peer that holds an ESTABLISHED IKE SA, a new
IKE_SA_INIT from that SA's authenticated peer address; authenticate it with the
same peer configuration; on success make it the peer's owner SA with its Child
SA, then retire the old SA, make before break. With no established SA, an
`initiate` peer still refuses unsolicited initiation.

### Placement decision

This stays a child of its own rather than moving into
`plan/post-release/features/spec-ike-reauth.md`:

| Question | Answer |
|----------|--------|
| What does spec-ike-reauth own? | Ze SCHEDULING its own reauthentication (a reauth timer, a new config leaf, make-before-break as initiator). Its Task, Data Flow and ACs never mention a reauthentication the other side starts (read 2026-10-31) |
| What does this spec own? | RECEIVING one on an `initiate` peer: no config, no timer, a dispatch and ownership change |
| Why not merge? | One is a P3 post-release feature, the other a fix to how a shipped peer type answers a permitted peer action; `ai/META-DECISIONS.md` asks that fixes and features be split when release planning needs them apart |
| What do they share? | The adoption of a superseding IKE SA by a peer session. The `respond` side already has it (`ps.supersede`, `established.go`); this spec generalises it to `initiate` sessions, and spec-ike-reauth reuses it. The main thread adds a pointer line to spec-ike-reauth's Required Reading |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` - IKE door
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - responder path, accept-in-parallel
  → Constraint: a fresh IKE_SA_INIT never touches the established SA until the new SA authenticates (`tryResponderSAInit` comment, RFC 7296 Section 2.4).
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - peer session ownership
- [ ] `plan/post-release/features/spec-ike-reauth.md` - Status design, P3
  → Constraint: "reuse the full-handshake machinery with explicit old/new SA ownership."

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - present
  → Constraint: RFC 7296 Section 2.8.3: "Reauthentication is done by creating a new IKE SA from scratch (using IKE_SA_INIT/IKE_AUTH exchanges, without any REKEY_SA Notify payloads), creating new Child SAs within the new IKE SA (without REKEY_SA Notify payloads), and finally deleting the old IKE SA (which deletes the old Child SAs as well)."
  → Constraint: RFC 7296 Section 2.8.3: "While creation of a new IKE SA can be initiated by either party (initiator or responder in the original IKE SA), the use of EAP and/ or Configuration payloads means in practice that reauthentication has to be initiated by the same party as the original IKE SA."
  → Decision: a peer whose original SA used EAP or a Configuration payload (virtual IP) keeps refusing; the CPE uses X.509 and no CP.

**Key insights:**
- The new SA's role is responder while the peer's configured role is `initiate`; every role-dependent step (AUTH octets, rekey collision, DPD) must read the SA's role, not the peer's connection type.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/register.go` `matchResponderPeer` (line 722) - skips every peer whose `ConnectionType` is not `respond`; compares the configured remote string to the datagram source.
- [ ] `register.go` `tryResponderSAInit` (about line 740) - cookie gate, one half-open handshake per peer (`responderBusy`), parallel acceptance while a tunnel is up, superseding only after the new SA authenticates.
- [ ] `internal/component/ike/engine/established.go` (about line 323) - the `ps.supersede` arm: relinquish the old SA after the parallel SA authenticated, remove only the old Child SA (make before break), let `runResponder` promote the new one.
- [ ] `internal/component/ike/ipsec/types.go` (line 263) - connection types `initiate` and `respond` only.
- [ ] `test/interop-ipsec/scenarios/responder-accepts-reinit/` - strongSwan initiates against a Ze `respond` peer; nothing covers a reinit toward an `initiate` peer. `mobike-initiator/swanctl.conf` sets `reauth_time = 0s` on strongSwan.

**Behavior to preserve:**
- `initiate` peers refuse unsolicited initiation while no IKE SA is established.
- The cookie gate and the one-half-open rule.
- `respond` peers unchanged.

**Behavior to change:**
- An `initiate` peer with an established SA accepts and adopts a reauthentication from that SA's peer address.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire: IKE_SA_INIT request with a zero responder SPI from the address of an established `initiate` peer's SA.

### Transformation Path
1. `tryResponderSAInit` asks the matcher; the matcher also returns an `initiate` session whose established SA's authenticated peer endpoint equals the source.
2. Cookie gate and busy CAS as today.
3. `newResponderSA` with the session's configuration; IKE_AUTH verifies the same identities and trust anchors.
4. On authentication the session's owner loop receives the supersede signal, retires the old SA and adopts the new one; the new Child SA (no REKEY_SA) is installed before the old one is removed.
5. The gateway deletes the old IKE SA; Ze's delete handling finds it already retired and does not reconnect.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ strongSwan | IKE_SA_INIT, IKE_AUTH, INFORMATIONAL Delete | No |
| Engine ↔ kernel | Child SA install then removal | No |

### Integration Points
- `matchResponderPeer`, `tryResponderSAInit`, the `ps.supersede` path, the initiator run loop in `fsm.go`/`reconcile.go`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Same responder path and gates |
| No unintended coupling | Yes | |
| No duplicated functionality | Yes | Generalises the existing supersede |
| Zero-copy preserved where applicable | N-A | |
| Registration over hardcoding, outbound | N-A | |
| Registration over hardcoding, inbound | N-A | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | strongSwan, as responder of the original SA with `reauth_time` set, reauthenticates by initiating a new IKE SA toward the peer | strongSwan source not present locally; read `libcharon/sa/ike_sa.c` (reauth) in the 6.1.0 tarball the lab builds, or observe it | The defect never fires with strongSwan; spec drops to documentation of Ze's refusal | First scenario run | unvalidated |
| A-2 | The initiator run loop can hand ownership to a responder-role SA | `respond` sessions do it | A larger change to the run loop | Phase 1 wiring test | unvalidated |
| A-3 | Behind NAT the gateway's IKE_SA_INIT reaches Ze only through the NAT mapping the old SA keeps open | NAT keepalives | Reauth from the gateway cannot work behind NAT; documented | `real-nat` variant | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Simultaneous reauth: Ze's own reconnect and the gateway's reinit cross | Two IKE SAs established | Keep the newest authenticated SA, delete the other (RFC 7296 Section 2.8.3 ends with the old SA deleted) |
| R-2 | A spoofed IKE_SA_INIT from the peer address holds the half-open slot | Busy-drop log | Existing cookie gate applies unchanged |
| R-3 | Role-dependent code reads the peer's connection type | Rekey collision or DPD misbehaves after adoption | Grep for `ConnectionType` reads in the established path; test rekey after adoption |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `initiate` peers: tunnel torn down at reauth, or two SAs |
| How is it reverted? | Single commit |
| Who else touches this path? | `spec-ike-reauth.md`, `responder-accepts-reinit` scenario, child 4 (resolved remote for `respond` matching) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_SA_INIT from an established `initiate` peer's address | → | `matchResponderPeer` → `tryResponderSAInit` → supersede | `TestInitiatePeerAdoptsPeerStartedReauth` |
| strongSwan responder with `reauth_time` | → | full stack | interop `responder-reauths-ze-initiator` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `initiate` peer established; new IKE_SA_INIT from that SA's peer address | Accepted on the responder path; cookie gate applies |
| AC-2 | AC-1, IKE_AUTH authenticates | New IKE SA and Child SA adopted; old Child SA removed only after the new one is installed; traffic continues (ping loss at most one packet) |
| AC-3 | AC-2, gateway deletes the old IKE SA | No reconnect is triggered |
| AC-4 | AC-1, IKE_AUTH fails | Old SA untouched; failure logged |
| AC-5 | `initiate` peer with no established SA | IKE_SA_INIT dropped as today |
| AC-6 | Original SA used EAP or a Configuration payload | Dropped, with a log quoting the Section 2.8.3 reason |
| AC-7 | Adopted SA later rekeyed, by either side | Rekey works with Ze as responder of the IKE SA |
| AC-8 | Source differs from the SA's peer address | Dropped |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs the CPE against a gateway that reauthenticates hourly | gateway IKE_SA_INIT → adopt → old deleted | `responder-reauths-ze-initiator` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInitiatePeerAdoptsPeerStartedReauth` | `internal/component/ike/engine/reauth_receive_test.go` | AC-1, AC-2 | |
| `TestInitiatePeerIgnoresDeleteOfRetiredSA` | same | AC-3 | |
| `TestInitiatePeerReauthFailureKeepsOldSA` | same | AC-4 | |
| `TestInitiatePeerRefusesUnsolicitedInit` | same | AC-5, AC-8 | |
| `TestInitiatePeerRefusesReauthAfterEAPOrCP` | same | AC-6 | |
| `TestAdoptedResponderRoleSARekeys` | same | AC-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | needs a real peer; covered by interop | | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `responder-reauths-ze-initiator` | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0 responder, `reauth_time` short, `make_before_break = yes` | AC-1 to AC-3 and AC-7: new SPIs, continuous ping, no reconnect log | |

## Files to Modify
- `internal/component/ike/engine/register.go` - matcher also returns an established `initiate` session for its SA's peer address
- `internal/component/ike/engine/established.go`, `fsm.go`, `reconcile.go` - the initiator loop adopts a superseding SA
- `internal/component/ike/engine/delete.go` - a delete for the retired SA does not trigger reconnect
- `docs/guide/ipsec.md` - reauthentication by the gateway

## Files to Create
- `internal/component/ike/engine/reauth_receive_test.go`
- `test/interop-ipsec/scenarios/responder-reauths-ze-initiator/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | |
| YANG validation constraints | No | |
| YANG custom validators | No | |
| CLI commands/flags | No | |
| CLI grammar | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |
| BGP family surface | N-A | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | Feature scope changed? | Yes | the IPsec feature file under `features/` |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior changed? | Yes | `rfc/short/rfc7296.md` Section 2.8.3 row if one exists |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-14-responder.md` |
| 13 | Route metadata keys | No | |
| 14 | Prometheus counters | No | |
| 15 | Registered plugin/event changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-22-ike-responder-reauth.md` |
| 17 | Existing examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring and A-1 (MANDATORY FIRST)**
   - Tests: `TestInitiatePeerAdoptsPeerStartedReauth` (red); the interop scenario run once against today's Ze to observe strongSwan's reauth and Ze's drop
   - Verify: A-1 confirmed with strongSwan's log; if strongSwan never reinitiates, stop and report
2. **Phase: Adoption** - AC-1 to AC-8; scenario green; the matcher change reverted once and recorded red

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Role read from the SA, not the connection type, after adoption |
| Rule: make before break | New Child SA installed before the old is removed |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Scenario | `ls test/interop-ipsec/scenarios/responder-reauths-ze-initiator` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Identity | New SA authenticates the same remote identity and anchor as the old |
| DoS | Cookie gate and one half-open slot unchanged |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 broken | Report; spec reduces to a guide sentence, owner decides |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

`respond` and `initiate` describe who dials first, not who may create the next IKE
SA. The responder path already knew how to replace an SA; it only asked the wrong
question about which peers it serves.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | No row exists (grep of `plan/journal/` for reauth and `matchResponderPeer` on 2026-10-31 found none) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Stand-alone child, pointer from spec-ike-reauth | Fold into spec-ike-reauth | See Placement decision |
| Accept only while an SA is established, only from its authenticated peer address | Accept any initiation on `initiate` peers | Keeps `initiate` semantics; a reauth only exists relative to an SA |
| Refuse after EAP or CP | Accept always | RFC 7296 Section 2.8.3 says such reauth "has to be initiated by the same party as the original IKE SA" |

## Known Limitations
- Behind a NAT without an open mapping the gateway cannot reach the CPE (A-3).
- The CPE's own configuration (IKE lifetime 0) does not prevent the gateway from reauthenticating.

## RFC Documentation (Scope: protocol)

The matcher's new branch carries the RFC 7296 Section 2.8.3 "either party" quote;
the EAP/CP refusal carries the "same party" quote.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`
