# Spec: ospf-accept-lifetime-receive-window

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.** The `accept-lifetime` container under
`key-chains/key` in `internal/plugins/ospf/yang/ze-ospf-conf.yang` is described
"When this key is accepted on receive.", and its two leaves are described
"RFC3339 start timestamp." and "RFC3339 end timestamp." The container, its
`start` and its `end` fail for one reason and share this spec. A reader takes
them as a bound on reception: before `start` and after `end`, a packet signed
with this key is refused. RFC 7474 Section 4 states the same rule for a key used
in OSPFv2 packet authentication: "For packet reception, the key validity interval
as defined by AcceptLifetimeStart and AcceptLifetimeEnd must include the current
time." The section introduces that line with "Generally, a key used for OSPFv2
packet authentication should satisfy the following requirements".

**What Ze does instead.** `parseKeyChain` (`internal/plugins/ospf/config.go`)
stores the container in `keyConfig.AcceptLifetime` through `parseLifetime`, and
`validateConfig` in the same file refuses the commit with `ErrKeyLifetimeFormat`
when either timestamp does not parse as RFC 3339. After that check the field has
no reader. `resolveChainKeys` (`internal/plugins/ospf/auth_keystore.go`) builds
each `resolvedKey` from `k.SendLifetime` alone, and `resolvedKey` carries
`sendStart` and `sendStop` and no accept bounds at all. `authStore.verify` in the
same file then loops over every key of the chain and accepts the packet on the
first key whose Key ID and digest match. The Key ID comparison happens inside
`Verify` (`internal/plugins/ospf/packet/auth_verify.go`). No clock is read
anywhere on the receive path. A key whose accept window closed last month
therefore still verifies, and deleting the key from the chain is the only way to
retire it. The `ze:help` beside the container states this in the sentence "Ze
does not gate reception on the window yet." The `description` does not.

**What closing it means.** The implementer either builds the receive window or
refuses the container at commit, the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses the `vrf` leaf. Building it
is the right answer, and refusing is not an equal option. RFC 7474 Section 4
states the reception requirement, `ai/rules/rfc-compliance.md` makes conformance
non-negotiable, and the security consequence lands on the operator: a key
believed expired still authenticates a neighbor. Refusing the container would
also take away hitless key rollover on OSPF, because an operator would have no
way to accept the incoming key before it starts signing. The shape to copy sits
in the sibling plugin. IS-IS parses the same YANG container into
`KeyConfig.AcceptStart` and `AcceptEnd` (`internal/plugins/isis/config.go`),
resolves it into a `lifetime` in `internal/plugins/isis/auth_keystore.go`, and
filters the verify set through `keyChain.acceptKeys` at the current time. The
OSPF work is the same three steps: carry the bounds into `resolvedKey`, filter in
`authStore.verify`, and decide what an empty accepted set does. That last
decision is the one the design owes an answer for, because `selectSendKey`
already documents a deliberate choice to keep signing with an expired key rather
than send unauthenticated, and the receive side needs a stated rule of its own.

## Progress (2026-09-06)

The receive window is BUILT, tested in both polarities, proven against FRR, and
documented. The package compiles and the OSPF unit suite and `./le functional
ospf` are green. The earlier "DOES NOT COMPILE" note above was written from
editor diagnostics and was already stale: `interfaceCost` had been removed from
`te_originate.go`, so nothing was redeclared.

One defect the earlier session left is fixed here. `authStore.verify` counted
in-window keys across the whole CHAIN, so a packet signed with a key the chain
holds and whose window had closed was reported as `digest-mismatch`. The secret
was correct, so that reading sent the operator to the key material when the
clock was the cause. `verify` now reads the Key ID the packet names
(`packet.AuthKeyID`) and reports `accept-lifetime` when the sender's own key is
the one outside its window.

**Not committed with this spec:** the interop scenario's registration. See
Known Limitations.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/ospf/ospf-12-auth.md` - the OSPFv2 sign/verify path this spec gates
  → Decision: the receive gate runs BEFORE the digest and before the replay bookkeeping, so an out-of-window key can neither accept a packet nor advance the sequence high-water mark
  → Constraint: the send side and the receive side point the same way and do the opposite thing: `selectSendKey` keeps signing with an expired key, `verify` refuses one
- [ ] `docs/guide/ospf.md` - the operator-facing description of key chains and rotation
  → Constraint: the guide states the drop is counted under the `accept-lifetime` reason of `ze_ospf_auth_failures_total`, so the reason string is a documented interface and not a free label
- [ ] `docs/architecture/testing/interop.md` - the interop scenario surface, declared as the design document by `internal/le/interoplab/bgp/checkers.go` and `check_extras.go`, both of which this spec edits
  → Constraint: "Typed checker operations" makes `checkers.go` the complete scenario catalogue, and states that an absent value never proves a negative assertion by itself: the operation must also name positive evidence that the query mechanism ran, or a failed query reads as protocol absence. The out-of-window drop this spec asserts is exactly that kind of negative, so the scenario asks FRR a question it answers in both states and reads the absence out of the answer.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7474.md` - RFC7474-4-1 is the requirement this spec implements
  → Constraint: read at `rfc/full/rfc7474.txt` Section 4: "For packet reception, the key validity interval as defined by AcceptLifetimeStart and AcceptLifetimeEnd must include the current time." The section introduces the list with "Generally, a key used for OSPFv2 packet authentication should satisfy the following requirements", so the level is SHOULD.

**Key insights:** (minimal context to resume after compaction)
- `packet.Verify` already compares the Key ID for AuType 2 and AuType 3, so a foreign key never verified. The gap was that no clock was read, not that any key matched.
- IS-IS solved the same problem in `internal/plugins/isis/auth_keystore.go`; `keyChain.acceptKeys` is the shape ported here.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/ospf/auth_keystore.go` - resolves chains into `resolvedKey`, selects the send key by `send-lifetime`, and verified against every chain key with no clock read
- [ ] `internal/plugins/ospf/packet/auth_verify.go` - `Verify` compares the Key ID (AuType 2 at auth-field offset 2, AuType 3 at offset 4) before the digest, so a foreign key id never verified
- [ ] `internal/plugins/ospf/config.go` - `parseKeyChain` stores `AcceptLifetime`, `validateConfig` rejects a non-RFC-3339 timestamp with `ErrKeyLifetimeFormat`, and nothing read the field after that
- [ ] `internal/plugins/ospf/auth_wiring.go` - `verifyPacket` turns the reason string into the `reason` label of `ze_ospf_auth_failures_total`
- [ ] `internal/plugins/isis/auth_keystore.go` - `keyChain.acceptKeys` filters the verify set at the current time; the shape ported here

**Behavior to preserve:** (unless the user explicitly said to change it)
- A key with no `accept-lifetime` is unbounded and verifies at every time, so a chain that configures none behaves exactly as before.
- `selectSendKey` is untouched: it still signs with an expired key rather than sending unauthenticated.
- The existing reason strings `decode`, `autype-mismatch`, `replay`, `password-mismatch` and `digest-mismatch` keep their meanings.

**Behavior to change:** (only what the user asked for)
- Reception is gated on the accept window. A key outside it is skipped before its digest is computed, so it can neither accept the packet nor record its sequence number.
- A new `accept-lifetime` reason is reported when the window is what refused the packet.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `ospf/key-chains/key/accept-lifetime/{start,end}`, two RFC 3339 strings.
- Wire: an OSPFv2 packet on a raw socket, carrying an AuType 2 or AuType 3 authentication field.

### Transformation Path
1. `parseKeyChain` (`internal/plugins/ospf/config.go`) reads the container into `keyConfig.AcceptLifetime`.
2. `validateConfig` refuses the commit when either timestamp is not RFC 3339.
3. `resolveChainKeys` (`internal/plugins/ospf/auth_keystore.go`) turns it into `resolvedKey.acceptStart` / `acceptStop` through `lifetimeBounds`.
4. `authStore.verify` reads `s.now()` once, skips every key whose `acceptsAt` is false, and only then calls `packet.Verify`.
5. `verifyPacket` (`internal/plugins/ospf/auth_wiring.go`) drops the packet and labels `ze_ospf_auth_failures_total` with the reason.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ packet codec | `packet.AuthKeyID(Header)` returns the Key ID the packet names, so the store reports which key was refused without re-deriving the auth-field offsets | Yes -- `TestAuthKeyID` |
| Config ↔ keystore | `authStore.configure(ospfConfig)` | Yes -- `TestAcceptLifetimeReachesVerifyFromConfig` runs the real parser |

### Integration Points
- `resolvedKey` - gains `acceptStart` / `acceptStop` beside the existing `sendStart` / `sendStop`.
- `authStore.now` - already existed for send-key selection; the receive gate reads the same clock, so one test hook moves both.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The gate sits in `authStore.verify`, the one function `verifyPacket` calls; no caller reaches a key another way |
| No unintended coupling (components stay isolated) | Yes | The only new cross-package call is `packet.AuthKeyID`, inside the OSPF plugin |
| No duplicated functionality (extends existing, does not recreate) | Yes | `lifetimeBounds` and `authStore.now` already existed for the send side and are reused |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `AuthKeyID` reads the already-decoded `Header`; no packet bytes are copied |
| Registration over hardcoding | N-A | No new command, view, family or handler; the change is inside one existing function |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A key with no `accept-lifetime` must stay unbounded | Every existing chain configures none, and `docs/guide/ospf.md` promised hitless rotation | Every OSPF adjacency using authentication drops on upgrade | `TestVerifyUnsetAcceptLifetimeAlwaysVerifies`, and the unchanged `ospf-auth-frr` interop scenario still reaching Full | confirmed |
| A-2 | `packet.Verify` already refuses a foreign Key ID, so the gap is the clock and not key selection | Read at `internal/plugins/ospf/packet/auth_verify.go`, both AuType 2 and AuType 3 branches | The design would have needed key selection as well as a window | Producer read, plus `TestAuthKeyID` pinning the two offsets | confirmed |
| A-3 | FRR signs OSPF packets with no accept-lifetime of its own, so a closed window on Ze's side is the only variable in the interop scenario | `test/interop/scenarios/ospf-accept-lifetime-frr/frr.conf` uses `ip ospf message-digest-key`, which has no lifetime | The scenario would prove FRR's behavior rather than Ze's | The recorded red: with the gate cut, the same scenario reaches Full | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator sets an accept window and locks every neighbor out | Adjacencies drop at the window edge and `ze_ospf_auth_failures_total{reason="accept-lifetime"}` climbs | The reason is specific, so the counter names the cause; the YANG `ze:help` states that a chain with no key in its window drops every packet |
| R-2 | The refusal is reported as a digest failure and sends the operator to the wrong place | An operator checks a secret that is correct | `verify` reads the Key ID the packet names and reports `accept-lifetime` for the sender's own retired key; `TestVerifyWrongSecretInsideAcceptLifetimeReportsDigestMismatch` pins the other pole |
| R-3 | A clock skew between neighbors retires a key on one side only | One-way adjacency loss at a window edge | Out of scope for this spec: the windows are operator-authored absolute times, as RFC 7474 Section 4 defines them |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every OSPF adjacency on an authenticated interface. A gate that refuses too much drops the adjacency; a gate that refuses too little leaves the security defect in place |
| How is it reverted? | Single commit revert. No config migration: the leaves already parsed and validated before this change |
| Who else touches this path? | `spec-ospf-auto-cost-reference-bandwidth` ran in the same package at the same time and touches `internal/plugins/ospf/yang/ze-ospf-conf.yang` and `docs/guide/ospf.md`, in different hunks |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `accept-lifetime { start end }` in the config file | → | `parseKeyChain` → `resolveChainKeys` → `resolvedKey.acceptsAt` → `authStore.verify` | `TestAcceptLifetimeReachesVerifyFromConfig` |
| An authenticated OSPF packet on the wire | → | `verifyPacket` → `authStore.verify` | `ospf-accept-lifetime-frr` interop scenario |
| `ze config validate` on a config carrying the container | → | `validateConfig` | `test/ospf/ospf-auth.ci` seq 3 and seq 4 |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A correctly signed packet arrives before the key's `accept-lifetime` start | Refused, reason `accept-lifetime` |
| AC-2 | The same packet bytes arrive inside the window | Accepted |
| AC-3 | The same packet bytes arrive after the window ends | Refused, reason `accept-lifetime` |
| AC-4 | Two keys with overlapping windows, a packet signed with each | Both accepted during the overlap; after the first window closes only the second is accepted, and the first is refused with reason `accept-lifetime` |
| AC-5 | A key with no `accept-lifetime` at any clock value | Accepted |
| AC-6 | A key inside its window signed with the wrong secret | Refused, reason `digest-mismatch`, not `accept-lifetime` |
| AC-7 | An out-of-window key would have matched the packet | Its sequence number is NOT recorded, so it cannot block a later legitimate packet |
| AC-8 | A peer signs correctly with a key whose window on Ze has closed | No adjacency forms, and the peer sees Ze but never reaches Full |
| AC-9 | `accept-lifetime { start yesterday }` at commit | Refused, the error names `accept-lifetime` |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Retires a key by closing its `accept-lifetime`, without editing the chain | config -> parseKeyChain -> resolveChainKeys -> acceptsAt -> verify -> packet dropped | `ospf-accept-lifetime-frr` interop scenario |
| 2 | Rolls a key over hitlessly by overlapping the two windows | same path, two keys in window at once | `TestVerifyAcceptLifetimePerKey` |
| 3 | Commits a config carrying the container | `ze config validate` -> validateConfig | `test/ospf/ospf-auth.ci` |
| 4 | Reads why packets are dropped | verify reason -> `ze_ospf_auth_failures_total{interface,reason}` | `TestVerifyWrongSecretInsideAcceptLifetimeReportsDigestMismatch` distinguishes the two reasons |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifyRejectsOutsideAcceptLifetime` | `internal/plugins/ospf/auth_keystore_test.go` | AC-1, AC-2, AC-3 with identical packet bytes and only the clock moving | pass |
| `TestVerifyAcceptLifetimePerKey` | `internal/plugins/ospf/auth_keystore_test.go` | AC-4 | pass |
| `TestVerifyUnsetAcceptLifetimeAlwaysVerifies` | `internal/plugins/ospf/auth_keystore_test.go` | AC-5 | pass |
| `TestVerifyWrongSecretInsideAcceptLifetimeReportsDigestMismatch` | `internal/plugins/ospf/auth_keystore_test.go` | AC-6 | pass |
| `TestVerifyUnknownKeyIDOutsideEveryWindow` | `internal/plugins/ospf/auth_keystore_test.go` | the chain-level arm of the reason | pass |
| `TestAcceptLifetimeReachesVerifyFromConfig` | `internal/plugins/ospf/auth_keystore_test.go` | the wiring: the real parser feeds the gate | pass |
| `TestAuthKeyID` | `internal/plugins/ospf/packet/auth_verify_test.go` | the AuType 2 and AuType 3 Key ID offsets, and that AuType 0 and 1 name no key | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| clock against `[acceptStart, acceptStop)` | half-open | `acceptStop` minus one instant is inside | any instant before `acceptStart` (AC-1) | `acceptStop` itself is outside (AC-3) |
| `start` / `end` string | RFC 3339, length 1..64 | `2026-03-01T00:00:00Z` | `yesterday` is refused at commit (AC-9) | N/A |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-auth` seq 3 | `test/ospf/ospf-auth.ci` | An `accept-lifetime { start end }` window validates at commit | pass |
| `ospf-auth` seq 4 | `test/ospf/ospf-auth.ci` | A timestamp that is not RFC 3339 is refused and the error names `accept-lifetime` | pass |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-accept-lifetime-frr` | `test/interop/scenarios/ospf-accept-lifetime-frr/` | FRR 10.3.1 | FRR signs correctly with the shared key and Ze refuses it, because Ze's copy of that key is outside its accept window. No adjacency forms | pass; RED observed with `acceptsAt` cut: "assertion 3: peer output unexpectedly contains \"Full\"" |
| `ospf-auth-frr` | `test/interop/scenarios/ospf-auth-frr/` | FRR 10.3.1 | Unchanged control: a chain with NO accept-lifetime still reaches Full, so the gate did not break rotation | pass |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/plugins/ospf/auth_keystore.go` - `resolvedKey` accept bounds, `acceptsAt`, the gate and the reason inside `verify`
- `internal/plugins/ospf/packet/auth_verify.go` - `AuthKeyID`, so the store can name the key a packet selected
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - the `accept-lifetime` `ze:help`, which said Ze did not gate reception yet
- `internal/plugins/ospf/auth_keystore_test.go`, `internal/plugins/ospf/packet/auth_verify_test.go` - unit and wiring tests
- `test/ospf/ospf-auth.ci` - the config-surface cases
- `docs/guide/ospf.md` - the key-chain paragraph
- `docs/architecture/ospf/ospf-12-auth.md` - the ordering decision, declared by `auth_keystore.go`'s `// Design:` header
- `rfc/short/rfc7474.md`, `rfc/requirements/rfc7474.md` - the RFC7474-4-1 row
- `internal/le/interoplab/bgp/checkers.go`, `internal/le/interoplab/bgp/check_extras.go` - the scenario's assertions (NOT COMMITTED, see Known Limitations)

## Files to Create
- `test/interop/scenarios/ospf-accept-lifetime-frr/ze.conf`, `frr.conf` - the interop scenario (NOT COMMITTED, see Known Limitations)
- `rfc/discrimination/rfc7474.json` - the recorded reds for RFC7474-4-1, both polarities

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The `accept-lifetime` container and its two leaves already existed; only the `ze:help` changed |
| YANG validation constraints | No | `length "1..64"` and the RFC 3339 parse in `validateConfig` already existed |
| YANG custom validators | No | `validateConfig` already refuses a non-RFC-3339 timestamp with `ErrKeyLifetimeFormat` |
| CLI commands/flags | No | No command added or changed |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | The leaves are free-form RFC 3339 strings, unchanged |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-auth.ci` seq 3 and seq 4 |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No leaf under `environment/` |
| Doctor check for runtime dependencies | No | No new file path, socket, port or binary; the change is a clock read |
| Prometheus counters/metrics | Yes | `ze_ospf_auth_failures_total{interface,reason}` gains the `accept-lifetime` reason value. The metric itself is unchanged and already registered in `internal/plugins/ospf/instance.go` |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The leaves were already published; they now do what they said |
| 2 | Config syntax changed? | No | No new leaf, no new container |
| 3 | CLI command added/changed? | No | -- |
| 4 | API/RPC added/changed? | No | -- |
| 5 | Plugin added/changed? | Yes | `docs/guide/ospf.md`, the key-chain paragraph |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md`, "Authentication" |
| 7 | Wire format changed? | No | Nothing Ze sends changed; the change is receive-side selection |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7474.md` gains RFC7474-4-1, `rfc/requirements/rfc7474.md` its row, `rfc/discrimination/rfc7474.json` both recorded reds. `docs/features/rfc-status.md` counts MUST-level gated requirements and RFC7474-4-1 is a SHOULD, so its row is unaffected and `./le rfc check` does not flag it |
| 10 | Test infrastructure changed? | No | The scenario uses the existing operation kinds |
| 11 | Affects daemon comparison? | No | -- |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ospf/ospf-12-auth.md`, declared by `auth_keystore.go`'s `// Design:` header |
| 13 | Route metadata keys added/changed? | N-A | -- |
| 14 | Prometheus counters added/changed? | No | The counter is unchanged; one new value of its existing `reason` label, stated in `docs/guide/ospf.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | -- |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/ospf.md` and `docs/architecture/ospf/ospf-12-auth.md` both anchor `internal/plugins/ospf/auth_keystore.go` and both are updated here |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `docs/guide/ospf.md` key-chain paragraph was checked against the YANG and rewritten |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the config value reaches the receive path
   - Tests: `TestAcceptLifetimeReachesVerifyFromConfig`
   - Files: `internal/plugins/ospf/auth_keystore.go`
   - Verify: the test failed because `resolvedKey` carried no accept bounds
2. **Phase: The gate** -- carry the bounds and skip an out-of-window key before the digest
   - Tests: `TestVerifyRejectsOutsideAcceptLifetime`, `TestVerifyAcceptLifetimePerKey`, `TestVerifyUnsetAcceptLifetimeAlwaysVerifies`
   - Files: `internal/plugins/ospf/auth_keystore.go`
   - Verify: green, and the recorded reds under a cut `acceptsAt`
3. **Phase: The reason** -- report `accept-lifetime` when the window is what refused the packet
   - Tests: `TestVerifyWrongSecretInsideAcceptLifetimeReportsDigestMismatch`, `TestVerifyUnknownKeyIDOutsideEveryWindow`, `TestAuthKeyID`
   - Files: `internal/plugins/ospf/packet/auth_verify.go`, `internal/plugins/ospf/auth_keystore.go`
   - Verify: a retired key reports the window, a wrong secret still reports the digest
4. **Phase: Prove it against a peer** -- the interop scenario and its red
   - Tests: `ospf-accept-lifetime-frr`, and `ospf-auth-frr` as the unchanged control
   - Files: `test/interop/scenarios/ospf-accept-lifetime-frr/`, the two checker maps
   - Verify: green with the gate, red without it

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The window is half-open `[start, stop)`, the same interval `lifetimeBounds` documents and `selectSendKey` applies |
| Correctness | The gate runs BEFORE `packet.Verify`, so an out-of-window key never records a sequence number |
| Naming | The reason string is `accept-lifetime`, matching the YANG leaf name and the sentence in `docs/guide/ospf.md` |
| Data flow | Only `authStore.verify` reads the window; `selectSendKey` is untouched |
| Rule: `ai/rules/rfc-compliance.md` | The quoted sentence in the code comment matches `rfc/full/rfc7474.txt` Section 4 verbatim, and the claim on each tag states what the test body checks and no more |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| The gate is reached from the real config parser | `go test -run TestAcceptLifetimeReachesVerifyFromConfig ./internal/plugins/ospf` |
| A peer's correctly signed packet is refused on a closed window | `INTEROP_SCENARIO=ospf-accept-lifetime-frr ./le integration interop` |
| Rotation still works with no accept-lifetime | `INTEROP_SCENARIO=ospf-auth-frr ./le integration interop` |
| The RFC tags carry recorded reds | `./le rfc discriminate id RFC7474-4-1` reports records rather than unproven |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | The two timestamps are operator-authored and already parsed by `validateConfig`; nothing from the wire reaches `lifetimeBounds` |
| Fail closed | A chain whose keys are all out of window refuses every packet rather than falling through to a digest comparison. The zero value of `acceptStart`/`acceptStop` means unbounded, which is a documented guard and not an accidental default (`ai/rules/principles.md`) |
| Error leakage | The reason string names the class of failure and never the key material |
| Replay interaction | An out-of-window key is skipped before `packet.Verify`, so it cannot advance `recvSeq` and block a legitimate packet (AC-7) |

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
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- The gap was never key SELECTION. `packet.Verify` has always compared the Key ID, in both the AuType 2 and the AuType 3 branch, so a foreign key never verified. What was missing was that no clock was read at all. A spec written from the caller would have designed key selection twice.
- A refusal reason is an interface. Reporting `digest-mismatch` for a key whose secret is correct is the silently-wrong value `ai/rules/principles.md` forbids, wearing a diagnostic's clothes: it sends the operator to the key material when the clock is the cause.
- The send side and the receive side of a key chain point the same way by doing opposite things. Signing with an expired key beats sending unauthenticated; refusing an expired key beats authenticating a neighbor the operator retired. Neither rule can be derived from the other, so each is stated where it is enforced.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Skip an out-of-window key BEFORE computing its digest | Compare the digest first and then check the window | A gate after the digest lets an out-of-window key record its sequence number, and the packet the operator meant to refuse then blocks the legitimate one behind it |
| A chain with no key in window refuses every packet | Fall back to accepting any key when the chain has expired entirely | Falling back would make the window advisory. The operator who closed the last window meant to stop accepting |
| Report `accept-lifetime` when the SENDER's own key is out of window | Report it only when no key of the chain is in window | The chain-level rule reports `digest-mismatch` for a key whose secret is correct, which is a wrong answer that costs the operator the search |
| A zero bound means unbounded | Require both timestamps whenever the container is present | Every existing chain configures no window, and `lifetimeBounds` already gave the send side that meaning |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- **The interop scenario is written, run and proven, but NOT COMMITTED.** Its two
  registration files, `internal/le/interoplab/bgp/checkers.go` and
  `internal/le/interoplab/bgp/check_extras.go`, carry another session's
  uncommitted RFC 7854 `scenarioStatisticsPMACCT` work in the same hunks, and a
  commit naming either file would carry that work too. `interoplab.Discover`
  errors on a scenario directory with no checker, so committing
  `test/interop/scenarios/ospf-accept-lifetime-frr/` alone would break
  `./le integration interop` for every session. Both stay in the working tree
  until the BMP session commits, then land together.
- Clock skew between neighbors is not addressed. RFC 7474 Section 4 defines the
  windows as absolute times and says nothing about synchronizing them.
- A simple-password chain (AuType 1) carries no Key ID on the wire, so its
  refusal can only be answered for the chain as a whole. `packet.AuthKeyID`
  reports that explicitly rather than inventing a key.

## RFC Documentation (Scope: protocol)

`internal/plugins/ospf/auth_keystore.go` carries the requirement verbatim above
both `resolvedKey.acceptsAt` and the skip inside `authStore.verify`, read from
`rfc/full/rfc7474.txt` Section 4: "For packet reception, the key validity
interval as defined by AcceptLifetimeStart and AcceptLifetimeEnd must include
the current time." The section introduces its list with "Generally, a key used
for OSPFv2 packet authentication should satisfy the following requirements", so
the level is SHOULD and `rfc/short/rfc7474.md` records it as one.

`internal/plugins/ospf/packet/auth_verify.go` carries the two Key ID field
layouts above `AuthKeyID`: RFC 2328 Appendix D for the one-octet AuType 2 field
and RFC 7474 Section 3 for the 32-bit AuType 3 field.

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
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
