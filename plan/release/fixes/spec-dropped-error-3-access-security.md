# Spec: dropped-error-3-access-security

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | 60 errors in IKE, L2TP/PPP/PPPoE, RADIUS and TACACS+ are caught and logged only below WARN; the MOBIKE route-source drop in `serviceMobike` kept a moved initiator from ever announcing its new address with nothing in the log, and the same shape remains there and in 58 other places |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/access-security.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under the door's base paths
(`internal/component/{aaa,authz,tacacs,radius,pki,ike,l2tp}`,
`internal/core/{ipsecinventory,ikeprobe,eap,audit}`). At HEAD on 2026-11-02:
**60 instances in 30 files** (IKE engine 32, IKE dataplane 6, IKE transport 1,
L2TP/PPP/PPPoE 15, TACACS+ 5, RADIUS 1).

The two instances the brief names:

| Instance | Today | Owed |
|----------|-------|------|
| `internal/component/ike/engine/mobike.go:576`, `PeerSession.serviceMobike` | `mobikeRouteSource` fails, `log.Debug("ike: MOBIKE source unavailable")`, the SA keeps its old path | Going on is right (the next tick retries), and the operator can act (a route or a policy refuses the lookup): WARN naming the peer, why the address cannot move, and the fix; once per condition, not per tick |
| `internal/component/ike/engine/mobike.go:593`, `PeerSession.serviceMobike` | The RFC 3948 Section 4 NAT keepalive send fails, `log.Debug("ike: MOBIKE NAT keepalive failed")` | A missed keepalive lets the NAT binding expire; WARN once per condition with the peer and the path, or count it in the IKE telemetry and WARN on the first |

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid |
| 2 | Going on is right and an operator can act | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`); on a per-packet path, count it and WARN once per condition |
| 3 | Going on is right and nobody can act (a close at teardown, an error reply to a peer already gone) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above |

`spec-ipsec-11-mobike.md` (design) owns the MOBIKE protocol logic and
`spec-ppp-l2tp-aaa-one-off-defects.md` owns open PPP defects; neither owns
these log levels. Where an answer-1 instance is a protocol defect already in
one of them, this child resolves the log level and names that spec.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/access-security.md`
  → Constraint: a backend that cannot build degrades, it does not abort; a returned error in an AAA `Build` path must not lock the operator out (`radius/aaa.go` comments)
  → Constraint: explicit rejection stops the AAA chain, a connection error moves to the next backend; a TACACS+ connection error made visible must not turn into a rejection
  → Constraint: a datagram that fails to decrypt must not kill a live SA (RFC 7296 Section 2.4); an error returned from an inbound path must not take that path
- [ ] `ai/digests/ipsec-ike.md` - IKE SA negotiation, rekey, XFRM install
  → Constraint: XFRM install and remove failures are per SA; a WARN names the SA and the peer
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: what, why, fix; never only the errno
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a guard

**Key insights:**
- The MOBIKE drops are the class's origin (d6ef7093a8 fixed the lookup; the Debug line stayed).
- Many IKE instances are error replies (`respondError`, `sendInvalidIKESPI`, `sendAuthFailed`): a reply that fails to send to an unauthenticated peer is answer 3; one to an established peer is answer 2.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - the shape
- [ ] `internal/component/ike/engine/mobike.go` - `serviceMobike`: route-source failure and NAT keepalive failure both at Debug

**Behavior to preserve:**
- IKE, L2TP and PPP wire behavior, the AAA chain order, and an SA's state on a failed reply.

**Behavior to change:**
- Each listed instance resolved by one answer.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists these 60 | Applies: the acceptance check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts a named guard | Applies: answer 3 only |
| IKE telemetry | `docs/guide/monitoring.md` (the IKE counters) | IKE SA and message counters | Extends: answer 2 on a per-packet path counts here, no new family |
| Operator warnings | `internal/core/report` (`show warnings`) | Lasting conditions | Applies where a failure is a state (an SA stuck on its old path) |
| MOBIKE route-source fix | d6ef7093a8, `TestMOBIKERouteSourceUnderTheCatchAllDiscard` | The lookup no longer crosses the SPD | Applies: its test is the base for the WARN test |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed call in the IKE engine, dataplane or transport, the L2TP reactor, PPP, PPPoE, the RADIUS EAP path or the TACACS+ client.

### Transformation Path
1. The branch catches the error.
2. It returns it, counts it and WARNs once, WARNs, or carries the annotation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IKE engine ↔ operator | Log at WARN, IKE counters | No |
| AAA backend ↔ chain | Returned error (connection error moves to the next backend) | No |

### Integration Points
- The subsystem loggers and the IKE metric families.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | No new path |
| No unintended coupling (components stay isolated) | Yes | No new import |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing loggers and metrics |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Per-packet paths count |
| Registration over hardcoding, outbound | N-A | Nothing registers |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 60 are still the population | Report of 2026-11-02 | Work from the fresh report | Report filtered to this child's paths before the first edit | unvalidated |
| A-2 | `serviceMobike` runs on a periodic tick, so a WARN per call repeats | It is called with `now` and keeps `lastKeepalive` | A once-per-condition WARN is unnecessary | Read its caller | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN per IKE tick or per PPP frame floods the log | An ipsec `.ci` log grows per second | WARN once per SA and condition, clear on recovery; count per event |
| R-2 | A returned error in an inbound IKE path kills a live SA | An interop scenario's SA drops on a malformed packet | Answer 1 only where the caller's handling of the error is correct per RFC 7296 Section 2.4 |
| R-3 | A WARN prints a shared secret or key material | Review grep of new WARN arguments | Name the peer and the operation only |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise, or an SA or subscriber session torn down by a newly returned error |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-ipsec-11-mobike`, `spec-ipsec-strongswan-interop-reds`, `spec-ppp-l2tp-aaa-one-off-defects` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Report filtered to the door's base paths: zero findings of the shape |
| A MOBIKE initiator whose route-source lookup fails | → | `PeerSession.serviceMobike` | `TestMOBIKERouteSourceFailureIsWarned` (ike engine package) asserts one WARN naming the peer and the fix |
| A NAT keepalive send that fails | → | `PeerSession.serviceMobike` | `TestMOBIKENATKeepaliveFailureIsWarned` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to this child's paths | No `findings` entry with rule `error-dropped-below-warn` |
| AC-2 | `mobike.go` route-source lookup fails | One WARN at the default level naming the peer, that the SA cannot move to a new address, and what to check; no repeat per tick while the condition holds |
| AC-3 | `mobike.go` NAT keepalive send fails | One WARN (or a counter and a first WARN) naming the peer and the path |
| AC-4 | Each answer-1 instance | A unit test where the caller observes the error |
| AC-5 | Each file with an answer-2 WARN | A unit test asserts one WARN line names what, why and the fix |
| AC-6 | Each annotation | Names a guard the file can see |
| AC-7 | `./le test ipsec`, `./le test l2tp` and the plugin `.ci` for TACACS+ and RADIUS that the change reaches, and the `mobike-initiator` interop scenario | Green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMOBIKERouteSourceFailureIsWarned` | `internal/component/ike/engine/mobike_test.go` | AC-2 | |
| `TestMOBIKENATKeepaliveFailureIsWarned` | `internal/component/ike/engine/mobike_test.go` | AC-3 | |
| One test per answer-1 instance | The package of the instance | AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/ipsec/*.ci`, `test/l2tp/*.ci` the change reaches | `test/` | Unchanged behavior; WARN where a `.ci` drives the failure | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `mobike-initiator` | `test/interop-ipsec/scenarios/` | strongSwan | MOBIKE still moves the SA with the WARN path in place | |

## Files to Modify
- `internal/component/ike/dataplane/espform_linux.go` - 5: `espFormReceiver.startLocked` 275,282; `espFormReceiver.stopLocked` 303,306; `espFormReceiver.run` 369
- `internal/component/ike/dataplane/xfrm_linux.go` - 1: `xfrmBackend.InstallSA` 114
- `internal/component/ike/engine/bypass.go` - 1: `removeIKEBypass` 172
- `internal/component/ike/engine/certurl.go` - 1: `certURLFetcher.start.func` 335
- `internal/component/ike/engine/child.go` - 2: `removeChildSAOutgoing` 1182; `removeChildSAIncoming` 1215
- `internal/component/ike/engine/delete.go` - 2: `PeerSession.writeDelete` 59; `sendIKESATeardown` 200
- `internal/component/ike/engine/established.go` - 4: `sendRaw` 885; `emitChildUp` 1081; `emitChildDown` 1102; `emitChildRekey` 1123
- `internal/component/ike/engine/fsm.go` - 1: `handleInbound` 466
- `internal/component/ike/engine/inbound.go` - 1: `PeerSession.handleInformationalOwned` 837
- `internal/component/ike/engine/mobike.go` - 2: `PeerSession.serviceMobike` 576,593
- `internal/component/ike/engine/notify_error.go` - 3: `PeerSession.respondError` 175; `sendInvalidIKESPI` 316,326
- `internal/component/ike/engine/notify_invalid_msgid.go` - 1: `PeerSession.sendInvalidMessageID` 111
- `internal/component/ike/engine/notify_suspicious.go` - 1: `PeerSession.informSuspicious` 239
- `internal/component/ike/engine/probe.go` - 1: `PeerSession.sendProbeDFClear` 336
- `internal/component/ike/engine/redistribute.go` - 1: `emitRouteRemove` 103
- `internal/component/ike/engine/register.go` - 1: `closeSDK` 571
- `internal/component/ike/engine/responder.go` - 7: `replayCachedResponse` 173; `resendResponderSAInit` 340; `sendSAInitNotifyRaw` 510; `PeerSession.respondAuthError` 1279,1288; `PeerSession.sendAuthFailed` 1301,1305
- `internal/component/ike/engine/spd_policy.go` - 2: `installSPDPolicies` 109; `removeSPDPolicies` 144
- `internal/component/ike/engine/unmatched.go` - 1: `removeUnmatched` 292
- `internal/component/ike/transport/keepalive.go` - 1: `Keepalive.Run` 61
- `internal/component/l2tp/plugins/authradius/register.go` - 1: `runPlugin.func` 105
- `internal/component/l2tp/ppp/dhcpv6_linux.go` - 3: `startDHCPv6Server` 55; `dhcpv6ServerLoop` 118,132
- `internal/component/l2tp/ppp/ra_linux.go` - 1: `startRASender` 65
- `internal/component/l2tp/ppp/ra_send.go` - 1: `raSender.send` 61
- `internal/component/l2tp/ppp/session_run.go` - 5: `pppSession.handleFrame` 958,979,1000,1023; `pppSession.handleLCPPacket` 1250
- `internal/component/l2tp/pppoe/panic_guard.go` - 1: `Subsystem.dispatchDiscovery` 92
- `internal/component/l2tp/pppoe/server.go` - 1: `InterfaceServer.sendFrame` 450
- `internal/component/l2tp/session_timeout.go` - 2: `l2tpReactor.runSessionTimeout` 69; `l2tpReactor.runIdleTimeout` 101
- `internal/component/radius/authenticator_eap.go` - 1: `radiusAuthenticator.authenticateEAP` 118
- `internal/component/tacacs/client.go` - 5: `TacacsClient.Close` 141; `TacacsClient.trySend` 476; `TacacsClient.storeConn` 588,594; `TacacsClient.closeAndEvict` 611

Lines are those of HEAD on 2026-11-02; the report is the current list.

## Files to Create
- `internal/component/ike/engine/mobike_test.go` additions (or the package's existing MOBIKE test file).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No dependency |
| Prometheus counters/metrics | Yes, if answer 2 counts | Existing IKE and L2TP families; name each counter used at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | If answer 2 adds a counter | `docs/guide/monitoring.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `docs/architecture/core-design.md`, `docs/architecture/ike/ipsec-6-ikev2-crypto.md`, `docs/architecture/ike/ipsec-7-ikev2-engine.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`, `docs/architecture/ike/ipsec-13-rekey-wire.md`, `docs/architecture/ike/ipsec-14-responder.md`, `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/architecture/l2tp/bng-5-pppoe.md`, `docs/guide/monitoring.md`, `docs/guide/radius.md`, `docs/research/l2tpv2-ze-integration.md`, `plan/release/validation/spec-ipsec-esp-dual-form-receive.md`. A log level changes none of them unless answer 1 changes described behavior; confirm with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-3-access-security.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the report filtered to this child's paths; write the two MOBIKE tests red.
2. **Phase: MOBIKE** -- resolve `mobike.go:576` and `:593` (AC-2, AC-3); run the `mobike-initiator` interop scenario.
3. **Phase: Classify** -- every other instance gets its answer and a one-line reason in Design Insights.
4. **Phase: Resolve** -- per file, tests first for answers 1 and 2; annotations last.
5. **Phase: Prove** -- AC-1, AC-4 to AC-7.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every instance has its answer at file:line |
| Correctness | No returned error makes an inbound failure kill a live SA; no AAA connection error becomes a rejection |
| Rule: user-facing-errors | Each WARN names the peer or subscriber, the operation, why and the fix, never a secret |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered by path |
| Package tests | `./le job run label ike command go test ./internal/component/ike/... ./internal/component/l2tp/... ./internal/component/tacacs/ ./internal/component/radius/` through `./le` so the patched GOROOT is selected (door: EAP-TLS) |
| Functional | `./le test ipsec`, `./le test l2tp` for the `.ci` the change reaches |
| Interop | `./le test integration interop-ipsec` with the `mobike-initiator` scenario |
| Change set judged | `./le verify changed` after the closing commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | A WARN or returned error carrying a PSK, a RADIUS or TACACS+ shared secret, a password or key material |
| Fail-open | A returned error in an AAA path that the chain reads as success |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `dropped-error-logged-below-warn.md` | 2026-11-01 | MOBIKE source lookup `routeSourceTo` (fixed by d6ef7093a8; the Debug line in `serviceMobike` remains and is AC-2 here) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| MOBIKE failures WARN once per SA and condition | WARN per tick | `serviceMobike` runs periodically; a per-tick WARN floods |
| The umbrella's three answers, in order | Raise every line to WARN | A WARN does not make a default correct |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-3-access-security.md` only, in the same `./le commit create` script
