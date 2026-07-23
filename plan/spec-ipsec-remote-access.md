# Spec: ipsec-remote-access

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-fixit-ipsec-verify-siblings |
| Phase | - |
| Updated | 2026-07-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc7296.md` Section 2.19 + 3.15 (Configuration payload), `rfc/short/rfc5216.md`
4. `internal/component/ike/engine/register.go` (dispatch + admission), `reconcile.go` (PeerSession),
   `responder_eap.go`, `internal/component/ike/eap/{eap.go,pool.go}`,
   `internal/component/ike/wire/payload_cp.go`

## Task

`vpn ipsec remote-access` is a complete, documented, YANG-validated config surface that the
daemon **silently ignores**. Traced 2026-07-23 (recorded in
`plan/spec-fixit-ipsec-verify-siblings.md` and in the correction appended to
`plan/learned/1255-fixit-codeql-security-triage.md`):

| Field | Consumer today | Effect |
|-------|----------------|--------|
| `ra.Pool.*` | `engine/register.go:313-320` builds `eap.NewPool(...)` into `ipPool` | **discarded**: `register.go:372` is `_ = ipPool` |
| `ra.Auth.*` | none | none |
| `ra.Users` (`eap-user`) | none | none |

The IKE responder admits only sources that `matchResponderPeer` (`engine/register.go:536-555`)
resolves to a configured **site-to-site** peer with a literal `remote-address` match; everything
else is logged "unsolicited IKE_SA_INIT from unconfigured source" and dropped
(`register.go:564-567`). `PeerSession.peerCfg` is populated exclusively from `cfg.Peers`
(`reconcile.go:366`). A road-warrior client, whose address is by definition not preconfigured,
can never establish.

Owner decision 2026-07-23: implement the feature rather than reject or warn about the
inert config.

**What already works and is reused, not rebuilt.** EAP-MSCHAPv2 and EAP-TLS authentication
against a *responder* are implemented and interop-proven today
(`test/ipsec-interop/scenarios/08-responder-eap-mschapv2`, `04-eap-tls`), expressed as a
site-to-site peer with `connection-type respond` and a fixed `remote-address`. The Configuration
payload codec (`wire/payload_cp.go`) and the virtual IP pool (`eap/pool.go`, with
`Allocate`/`Release`/`Available`) are both complete and both have zero callers.

So the missing pieces are admission, per-user credentials, and address assignment:

1. **Admission**: accept an IKE_SA_INIT from an unconfigured source when `remote-access` is
   configured, and give each client its own session.
2. **Per-user credentials**: resolve the EAP identity to an `eap-user` entry, instead of the
   single `pre-shared-secret` a site-to-site peer carries. Unknown user must fail closed.
3. **Address assignment**: honour the client's CFG_REQUEST, allocate from the pool, answer
   CFG_REPLY (RFC 7296 Section 2.19), narrow the responder traffic selector to the assigned
   address, and release the address when the SA goes away.
4. **Config correctness** (inherited from `plan/deferrals/fixit-ipsec-verify-siblings.md`):
   resolve `ra.Auth` certificate references against the candidate PKI and require a
   `ca-certificate` for `mode eap-tls` (RFC 5216 Section 5.3), now that the config is live.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-design.md` - registration and proximity
  -> Constraint: no new communication mechanism; the engine already owns its dispatch loop.
- [ ] `ai/rules/goroutine-lifecycle.md`
  -> Constraint: a per-client session is a goroutine per *lifecycle* (allowed), never a
     goroutine per event. It MUST be reaped when its SA dies, or a road-warrior gateway leaks
     one goroutine and one pool address per connection attempt.
- [ ] `ai/rules/exact-or-reject.md`
  -> Constraint: pool exhaustion must refuse the client with a clear reason, never assign a
     duplicate or silently omit the address.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` - IKEv2
  -> Constraint: Section 2.19 the initiator requests an internal address with CP(CFG_REQUEST)
     in IKE_AUTH; the responder answers CP(CFG_REPLY) in the same exchange that carries SAr2/TSr.
  -> Constraint: Section 3.15 CFG_REQUEST attribute values MAY be zero-length (a request);
     the reply carries the assigned value.
  -> Constraint: Section 2.16 EAP: the responder authenticates with its own long-term credential
     in the first IKE_AUTH response, before the EAP exchange.
- [ ] `rfc/short/rfc5216.md` - EAP-TLS
  -> Constraint: Section 5.3 both sides MUST path-validate. The gateway validates the client
     chain against `ra.Auth.ca-certificate`.
- [ ] `rfc/short/rfc3748.md` - EAP
  -> Constraint: Section 5.1 the authenticator begins with an Identity request; the identity is
     therefore known before the method starts, which is what makes per-user lookup possible.

**Key insights:**
- `Session.handleIdentity` (`eap/eap.go:205-217`) sets `s.identity` and only then calls
  `s.method.Start(...)`. That ordering is the hook for per-user credential resolution: the
  method can be handed the right password after the identity is known and before it is used.
- `MethodConfig.Password` (`eap/eap.go:157`) is a single value captured at
  `newMSCHAPv2Method` (`eap/eap_mschapv2.go:38-42`), so it cannot express a user table as-is.
- `PeerSession.responderBusy` gates ONE in-flight half-open handshake **per session**
  (`reconcile.go:25-35`). Sharing one session across all road warriors would serialize them
  and let one client's handshake block every other's; a per-client session avoids this and
  reuses the established/DPD/rekey machinery unchanged.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/register.go` - `OnConfigure` (:250), pool built + discarded
  (:313-320, :372), `matchResponderPeer` (:536), `tryResponderSAInit` (:557), dispatch
- [ ] `internal/component/ike/engine/reconcile.go` - `PeerSession` (:18), `startPeerSession`
  (:353), `run`/`runOnce` reconnect loop
- [ ] `internal/component/ike/engine/responder.go` - `newResponderSA` (:25), `isEAPMode` (:49)
- [ ] `internal/component/ike/engine/responder_eap.go` - `eapMethodConfig` (:22),
  `startResponderEAP` (:88), `handleResponderEAP` (:164)
- [ ] `internal/component/ike/eap/eap.go` - `Session`, `MethodConfig` (:156), `Begin` (:163),
  `handleIdentity` (:205), `Identity()` (:192)
- [ ] `internal/component/ike/eap/eap_mschapv2.go` - `newMSCHAPv2Method` (:38), `Start` (:48)
- [ ] `internal/component/ike/eap/pool.go` - `NewPool` (:35), `Allocate` (:89), `Release` (:126)
- [ ] `internal/component/ike/wire/payload_cp.go` - complete CP codec, zero callers
- [ ] `internal/component/ike/ipsec/types.go` - `RemoteAccessConfig` (:420), `EAPUser` (:399)
- [ ] `test/ipsec-interop/scenarios/08-responder-eap-mschapv2/` - the shape that works today

**Behavior to preserve:**
- Site-to-site peers keep exact-match admission and priority. A configured peer address must
  never be diverted into the remote-access path.
- `responderBusy` semantics per session (RFC 7296 Section 2.4 accept-in-parallel) unchanged.
- Existing interop scenarios 01-11 unchanged and green.
- A config with no `remote-access` block behaves exactly as today.

**Behavior to change:**
- An unsolicited IKE_SA_INIT from an unconfigured source is admitted when `remote-access` is
  configured (today: dropped).
- `eap-user` entries become live credentials.
- The virtual IP pool is assigned rather than discarded.

## Data Flow (MANDATORY)

### Entry Point
Inbound UDP IKE_SA_INIT from an arbitrary source address, on the IKE (500) or NAT-T (4500) port.

### Transformation Path
1. `dispatchInbound` -> no SATable entry, `ExchangeIKESAInit`, not a response.
2. `matchResponderPeer(src)` -> nil (no configured peer).
3. **NEW** `matchRemoteAccess(src)` -> if `remote-access` is configured, create (or find) a
   per-client `PeerSession` from a `SiteToSitePeer` synthesized from `RemoteAccessConfig`.
4. `newResponderSA` -> IKE_SA_INIT exchange -> IKE_AUTH.
5. IKE_AUTH carries IDi, no AUTH (EAP) plus **CP(CFG_REQUEST)**: stash the request on the SA.
6. `startResponderEAP` -> gateway AUTH from `ra.Auth` -> EAP-Request/Identity.
7. EAP rounds; **NEW** identity resolves to an `eap-user` before the method starts.
8. On EAP success + AUTH-from-MSK: **NEW** allocate from the pool, build CP(CFG_REPLY),
   narrow TSr to the assigned address, send with SAr2.
9. SA teardown (`StateDead`, DPD, delete, reap): **NEW** release the address, reap the session.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| UDP transport <-> engine dispatch | `transport.Packet` | [ ] |
| engine <-> EAP session | `eap.MethodConfig`, `eap.Session` | [ ] |
| engine <-> pool | `eap.Pool.Allocate`/`Release` | [ ] |
| engine <-> wire | `wire.PayloadCP` | [ ] |

### Integration Points
- `eap.Pool` gains no API; `Allocate`/`Release` are already the right shape.
- `wire.PayloadCP` gains no API; the codec is complete.
- `eap.MethodConfig` gains a per-user credential resolver (the one genuinely new SDK-ish surface).

### Architectural Verification
- [ ] No bypassed layers - admission stays in dispatch, auth stays in the EAP session
- [ ] No unintended coupling - the pool is owned by the engine, not reached from `eap` internals
- [ ] No duplicated functionality - reuses `PeerSession`, `newResponderSA`, the EAP session, the
      CP codec and the pool; builds no parallel FSM
- [ ] Registration over hardcoding - no new per-feature switch in a shared package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | A `SiteToSitePeer` synthesized from `RemoteAccessConfig` drives the existing responder FSM unchanged | scenario 08 proves the same FSM works for EAP with a peer struct | a parallel responder path would be needed, much larger | interop scenario 12 | unvalidated |
| A-2 | `wire.PayloadCP` round-trips the attributes strongSwan sends | codec reads/writes per RFC 7296 3.15 but has never run against a real peer | CP codec fixes needed first | unit round-trip + interop | unvalidated |
| A-3 | `eap.Pool.Allocate` is safe under concurrent road-warrior handshakes | `pool.go` takes a lock in `allocateV4`/`allocateV6` | duplicate address assignment | `-race` test with concurrent Allocate | unvalidated |
| A-4 | A per-client `PeerSession` can be reaped without disturbing configured peers | `activePeersMap` is name-keyed and reconcile iterates config peers | reconcile would delete or resurrect dynamic sessions | reconcile test with both kinds present | unvalidated |
| A-5 | strongSwan can be driven as a road-warrior client in the existing lab | the lab already runs strongSwan as initiator (scenarios 01-06) | interop proof needs another client | scenario 12 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Admitting unconfigured sources is a DoS surface: each attempt costs a goroutine, an SA and a pool address | memory/goroutine growth under a flood | cap concurrent half-open remote-access sessions; release the address only on success, never on half-open; reuse the existing `responderBusy` per-client gate |
| R-2 | A dynamic session leaks when its SA dies in an unusual state | goroutine count grows across connect/disconnect cycles | explicit reap path plus a test that cycles N clients and asserts the session map returns to its base size |
| R-3 | Pool exhaustion silently assigns nothing and the client establishes with no address | client connects but cannot route | refuse the exchange with a clear log and a NOTIFY; never send an empty CFG_REPLY |
| R-4 | Per-user lookup changes `eap.Session` shape and breaks the site-to-site EAP path | scenario 08 goes red | keep `MethodConfig.Password` working as-is; the resolver is additive and only consulted when set |
| R-5 | The reaper races the dispatch goroutine, which may be mid-handshake for that client | `-race` failures in the ike suite | reuse the existing atomic/ownedSA discipline; run `-race` on the engine package |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_SA_INIT from an unconfigured source, remote-access configured | -> | `matchRemoteAccess` -> dynamic `PeerSession` | `TestRemoteAccessAdmitsUnconfiguredSource` |
| EAP identity of a configured `eap-user` | -> | per-user credential resolver | `TestRemoteAccessResolvesEAPUserPassword` |
| CP(CFG_REQUEST) in IKE_AUTH | -> | pool allocate + CFG_REPLY | `TestRemoteAccessAssignsVirtualIP` |
| SA teardown | -> | pool release + session reap | `TestRemoteAccessReleasesAddressOnTeardown` |
| strongSwan road-warrior client | -> | the whole path | `test/ipsec-interop/scenarios/12-remote-access-eap-mschapv2` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IKE_SA_INIT from an unconfigured source, `remote-access` configured | admitted; a per-client session is created |
| AC-2 | IKE_SA_INIT from an unconfigured source, NO `remote-access` configured | dropped exactly as today |
| AC-3 | IKE_SA_INIT from a source matching a configured site-to-site peer, `remote-access` also configured | handled by the site-to-site peer, never diverted |
| AC-4 | EAP identity matching an `eap-user` with a password | authenticates with that user's password |
| AC-5 | EAP identity matching no `eap-user` | EAP-Failure; no fallback credential is tried |
| AC-6 | two road warriors with different identities | each authenticates with its own credential |
| AC-7 | CP(CFG_REQUEST) asking for INTERNAL_IP4_ADDRESS | CFG_REPLY carries an address from the pool, plus netmask and any configured DNS/domain |
| AC-8 | responder TSr after assignment | narrowed to the assigned address |
| AC-9 | pool exhausted | client refused with a clear reason; no duplicate address; no empty CFG_REPLY |
| AC-10 | SA teardown | the address returns to the pool (`Available()` restored) and the session is reaped |
| AC-11 | N connect/disconnect cycles | goroutine count and session map return to base |
| AC-12 | `remote-access mode eap-tls` with no `ca-certificate` | config verify rejects (RFC 5216 Section 5.3) |
| AC-13 | `ra.Auth` certificate/ca-certificate absent from candidate PKI | config verify rejects naming the reference |
| AC-14 | strongSwan road-warrior client, EAP-MSCHAPv2 | establishes, receives a virtual IP, passes traffic |
| AC-15 | concurrent `Allocate` under `-race` | no duplicate address, no race report |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | road warrior dials in with EAP-MSCHAPv2 and gets a virtual IP | UDP -> admission -> responder SA -> EAP -> user lookup -> pool -> CFG_REPLY -> child SA | interop scenario 12 |
| 2 | road warrior dials in with EAP-TLS | same, with client-chain validation against `ra.Auth.ca-certificate` | interop scenario 13 |
| 3 | operator commits a remote-access block with a bad certificate reference | commit -> tx bridge -> `OnConfigVerify` -> rejection | `test/reload/test-tx-ipsec-remote-access-pki.ci` |
| 4 | operator inspects who is connected | `show vpn ipsec ...` reflects dynamic sessions | `TestRemoteAccessSessionsVisible` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRemoteAccessAdmitsUnconfiguredSource` | `engine/remote_access_test.go` | AC-1 | |
| `TestRemoteAccessDropsWhenNotConfigured` | `engine/remote_access_test.go` | AC-2 | |
| `TestRemoteAccessNeverDivertsConfiguredPeer` | `engine/remote_access_test.go` | AC-3 | |
| `TestRemoteAccessResolvesEAPUserPassword` | `eap/eap_user_test.go` | AC-4 | |
| `TestRemoteAccessUnknownUserFailsClosed` | `eap/eap_user_test.go` | AC-5 | |
| `TestRemoteAccessDistinctUsers` | `eap/eap_user_test.go` | AC-6 | |
| `TestConfigPayloadRoundTrip` | `wire/payload_cp_test.go` | AC-7 codec | |
| `TestRemoteAccessAssignsVirtualIP` | `engine/remote_access_test.go` | AC-7 | |
| `TestRemoteAccessNarrowsTrafficSelector` | `engine/remote_access_test.go` | AC-8 | |
| `TestRemoteAccessPoolExhaustionRefuses` | `engine/remote_access_test.go` | AC-9 | |
| `TestRemoteAccessReleasesAddressOnTeardown` | `engine/remote_access_test.go` | AC-10 | |
| `TestRemoteAccessNoSessionLeak` | `engine/remote_access_test.go` | AC-11 | |
| `TestValidateRemoteAccessRequiresCAForEAPTLS` | `ipsec/validate_test.go` | AC-12 | |
| `TestValidateRemoteAccessRejectsUnknownPKIRefs` | `ipsec/validate_test.go` | AC-13 | |
| `TestPoolAllocateConcurrent` | `eap/pool_test.go` | AC-15 (`-race`) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pool `/N` IPv4 | /8../30 (existing `validatePoolPrefix`) | /30 | /7 | /31 |
| pool `/N` IPv6 | /48../126 (existing) | /126 | /47 | /127 |
| addresses issued from a /30 pool | usable hosts | last usable | N/A | one past the pool -> AC-9 refusal |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-tx-ipsec-remote-access-pki` | `test/reload/test-tx-ipsec-remote-access-pki.ci` | commit of a remote-access block with an unresolvable certificate reference is refused | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `12-remote-access-eap-mschapv2` | `test/ipsec-interop/scenarios/` | strongSwan (road-warrior client) | AC-14: unconfigured source establishes, gets a virtual IP, passes traffic | |
| `13-remote-access-eap-tls` | `test/ipsec-interop/scenarios/` | strongSwan | EAP-TLS road warrior, client chain validated | |

## Files to Modify
- `internal/component/ike/engine/register.go` - admission fallback; stop discarding the pool
- `internal/component/ike/engine/reconcile.go` - dynamic session creation and reaping
- `internal/component/ike/engine/responder_eap.go` - per-user credential resolution
- `internal/component/ike/engine/responder.go` / `fsm.go` - CP request stash, CFG_REPLY, TS narrowing
- `internal/component/ike/eap/eap.go` - `MethodConfig` per-user resolver (additive)
- `internal/component/ike/eap/eap_mschapv2.go` - accept the resolved credential
- `internal/component/ike/ipsec/validate.go` - `ValidateRemoteAccess` PKI-aware (inherited item 4)
- `internal/component/ike/engine/config.go` - pass the candidate PKI closures through

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | the surface already exists in `ipsec/yang/ze-ipsec-conf.yang` |
| CLI commands | Yes | dynamic sessions must appear in the existing `show vpn ipsec` views |
| Functional test | Yes | `test/reload/test-tx-ipsec-remote-access-pki.ci` |
| Doctor check | Yes | pool sanity (a pool that cannot serve one client) reuses the `ike` `DoctorChecks` added by the sibling spec |
| Prometheus counters | Yes | connected remote-access clients, pool utilisation, auth failures |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | remote-access becomes live, not new syntax; check the guide |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` if session views change |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [ ] | `docs/guide/vpn/*` remote-access section |
| 7 | Wire format changed? | [ ] | CP payload now emitted: `docs/architecture/wire/*` |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [ ] | `docs/features/rfc-status.md` RFC 7296 Section 2.19 row |
| 10 | Test infrastructure changed? | [ ] | new interop scenarios |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` remote-access VPN row |
| 12 | Internal architecture changed? | [ ] | IKE subsystem doc |
| 13 | Route metadata keys? | [ ] | |
| 14 | Prometheus counters? | [ ] | telemetry doc |
| 15 | Registered inventory changed? | [ ] | doctor check inventory |
| 16 | Doc source anchors on changed files? | [ ] | grep `docs/` per changed file |
| 17 | Existing docs show examples? | [ ] | verify remote-access examples now describe live behavior |

## Files to Create
- `internal/component/ike/engine/remote_access.go` + `_test.go`
- `internal/component/ike/eap/eap_user.go` + `_test.go` (or additive in `eap.go`)
- `test/reload/test-tx-ipsec-remote-access-pki.ci`
- `test/ipsec-interop/scenarios/12-remote-access-eap-mschapv2/{ze.conf,swanctl.conf,check.py}`
- `test/ipsec-interop/scenarios/13-remote-access-eap-tls/{ze.conf,swanctl.conf,check.py}`

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** - admission fallback + dynamic session, failing wiring test
   - Tests: `TestRemoteAccessAdmitsUnconfiguredSource`, `TestRemoteAccessDropsWhenNotConfigured`,
     `TestRemoteAccessNeverDivertsConfiguredPeer`
   - Files: `engine/remote_access.go`, `engine/register.go`, `engine/reconcile.go`
   - Verify: an unconfigured source reaches a session; no site-to-site behavior changes
2. **Phase: Per-user credentials** - AC-4..AC-6
   - Tests: the three `eap/eap_user_test.go` tests
   - Files: `eap/eap.go`, `eap/eap_mschapv2.go`, `engine/responder_eap.go`
   - Verify: unknown identity fails closed; scenario 08 still green
3. **Phase: Configuration payload + pool** - AC-7..AC-9
   - Tests: `TestConfigPayloadRoundTrip`, `TestRemoteAccessAssignsVirtualIP`,
     `TestRemoteAccessNarrowsTrafficSelector`, `TestRemoteAccessPoolExhaustionRefuses`
   - Files: `engine/responder.go`, `engine/fsm.go`, `engine/remote_access.go`
4. **Phase: Lifecycle** - AC-10, AC-11, AC-15
   - Tests: `TestRemoteAccessReleasesAddressOnTeardown`, `TestRemoteAccessNoSessionLeak`,
     `TestPoolAllocateConcurrent`
   - Verify: `go test -race` on the engine and eap packages
5. **Phase: Config validation** - AC-12, AC-13 (inherited deferral)
   - Files: `ipsec/validate.go`, `engine/config.go`, `test/reload/*.ci`
6. **Phase: Interop** - AC-14
   - `test/ipsec-interop/scenarios/12-*`, `13-*`; `make ze-ipsec-interop-test`
7. **Observability + docs** - counters, `show` views, documentation checklist
8. **Full verification, review gate, closure**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Fail-closed | Unknown identity, exhausted pool, and unparseable CP each DENY with a named reason |
| Resource safety | No goroutine, SA, or pool address outlives its client (R-1, R-2) |
| Site-to-site untouched | Scenarios 01-11 green; admission precedence proven by AC-3 |
| Concurrency | `-race` on engine + eap; concurrent Allocate has no duplicate |
| Mutation-verify | Disable each new guard; its test must go red |
| Rule: no-layering | The old `_ = ipPool` discard is deleted, not bypassed |
| Rule: exact-or-reject | Pool exhaustion refuses; never a partial CFG_REPLY |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| pool no longer discarded | `grep -n '_ = ipPool' internal/component/ike/engine/register.go` returns nothing |
| CP payload emitted | interop capture or `TestRemoteAccessAssignsVirtualIP` |
| eap-user live | `TestRemoteAccessResolvesEAPUserPassword` |
| interop green | `make ze-ipsec-interop-test` scenarios 12, 13 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Unauthenticated resource consumption | admission happens before authentication by construction (RFC 7296); bound half-open sessions and do not allocate a pool address until EAP succeeds |
| Credential handling | `eap-user` passwords never logged, never serialized (`json:"-"` as `MethodConfig.Password` already is) |
| Identity spoofing | the EAP identity selects the credential but does not by itself authenticate; the method must still verify |
| Address reuse | a released address must not be handed out while the old child SA still forwards |
| Client chain validation | EAP-TLS gateway validates against `ra.Auth.ca-certificate`, never an empty pool (see the sibling spec's finding) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| CP codec mismatch with strongSwan | fix `wire/payload_cp.go`, add a round-trip case from the real capture |
| Scenario 08 regresses | the per-user resolver was not additive; restore `MethodConfig.Password` precedence |
| Goroutine leak | Phase 4 reap path |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The feature is mostly *wiring already-built parts*: the CP codec, the IP pool, and the EAP
  authenticator all exist and all have zero callers. That is worth stating plainly, because it
  is also how the gap survived so long -- every individual piece looked done.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-client dynamic `PeerSession` synthesized from `RemoteAccessConfig` | one shared remote-access session; a parallel road-warrior FSM | reuses the established/DPD/rekey/teardown machinery unchanged, and gives each client its own `responderBusy` gate so one client cannot block another |
| Per-user credential resolver on `MethodConfig`, consulted after the EAP identity | rebuild the session per identity; look the user up in the engine and recreate the method | `handleIdentity` already sequences identity before method start; additive so the site-to-site EAP path is untouched |
| Allocate the pool address only after EAP success | allocate at admission | an unauthenticated source must not be able to drain the pool (R-1) |

## Known Limitations
- (fill during implementation)

## RFC Documentation

`// RFC 7296 Section 2.19` above the CFG_REQUEST/CFG_REPLY handling,
`// RFC 7296 Section 3.15.1` above the attribute encoding,
`// RFC 5216 Section 5.3` above the client-chain validation,
`// RFC 3748 Section 5.1` above the identity-then-method ordering the resolver depends on.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| a road warrior can establish against ze | interop test | scenario 12 vs strongSwan |
| per-user credentials are live | unit + interop | |
| virtual IP assignment works | interop (client holds the address) | |
| no resource leak per connection | unit lifecycle test | |
| remote-access config is validated | functional `.ci` | |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING - before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
