# Spec: l2tp-pool-reload-applies -- a committed pool edit reaches the running pools

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | Shipped defect: a reload that edits `l2tp pool` reports success and changes nothing, and a RADIUS-assigned address inside a pool range can be handed to a second subscriber |
| RFC gaps | - |
| Depends | - |
| Phase | 0/3 |
| Handoff | - |
| Updated | 2026-10-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Doors: `ai/doors/system.md`, `ai/doors/access-security.md` (l2tp, PPPoE, PPP).

## Task

The `l2tp-pool` plugin (`internal/component/l2tp/plugins/pool/register.go`)
registers config verify, apply and rollback handlers, so the SDK's
"no apply handler" refusal (`spec-plugin-reload-applies-or-refuses`) never
fires for it. The handlers do not apply anything on a reload:

| Step | What runs | What it does to the candidate |
|------|-----------|-------------------------------|
| Stage 2 configure | `OnConfigure` closure in `runPlugin` | parses with `parseFullPoolConfig`, installs, sets `pendingResult` to nil |
| Reload verify | `verifyPoolConfig` | parses with `parsePoolConfig` and discards the result |
| Reload apply | `OnConfigApply` closure | `pendingResult` is nil (only `OnConfigure` sets it), so it answers OK and installs nothing |
| Reload rollback | `OnConfigRollback` closure | sets `pendingResult` to nil |

So an operator who widens a range, adds a named pool or an IPv6 block, and
commits, sees the commit succeed while the boot pools keep serving. The
"cannot replace pools with N live allocations" refusal inside the apply closure
is unreachable for the same reason.

The second defect sits in the same ownership question. `poolPlugin.handle`
stores a RADIUS Framed-IP-Address with `fromPool: false` and never marks it in
the pool bitmap, so an address RADIUS assigns inside a configured range can be
allocated by `ipv4Pool.allocate` to another subscriber while the first holds it.

Goal: a committed pool edit is installed by the reload that commits it, live
subscribers keep their addresses across it, and no address is held by two
subscribers at once.

Reference: accel-ppp PR #318 (https://github.com/accel-ppp/accel-ppp/pull/318,
merged) rebuilds the pool set on `EV_CONFIG_RELOAD` and reconciles live
sessions into it: "sessions are the source of truth, the bitmap is rebuilt from
them", an address whose range left the pools is kept by its session
(`reload-orphan = keep`, the default), and "foreign in-range addresses are
reserved to avoid duplicate assignment".

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` "Config Reload Protocol" - verify then apply; a verify refusal aborts the reload
  → Constraint: a refusal belongs in verify. An apply error after every plugin's verify accepted forces the transaction's rollback of other plugins, which is the shape `plan/journal/reload-rolls-back-instead-of-applying.md` records
- [ ] `docs/architecture/config/transaction-protocol.md` - the verify/apply/rollback transaction `sdk.ConfigTx` implements
  → Decision: the plugin moves to `sdk.NewConfigTx(...).Register(p)` (`pkg/plugin/sdk/config_tx.go`), the same move every plugin in `spec-plugin-reload-applies-or-refuses` made; its Configure, Verify, Apply and Rollback replace the four hand-written closures
- [ ] `docs/plugin-development/handlers.md` - `OnConfigApply` or `sdk.NewConfigTx` for a plugin that reads a leaf
- [ ] `docs/guide/l2tp.md` - the operator page for `l2tp pool`
  → Constraint: it states nothing about reload today; this spec adds what a reload does to live subscribers
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - names `pool/register.go`
- [ ] `docs/research/l2tpv2-ze-integration.md` - the design document `pool/register.go` declares
  → Constraint: if it describes the pool's reload or the live-allocation refusal, it is edited in the phase that changes them

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2865.md` - Section 5.8 Framed-IP-Address, the attribute whose address the pool must reserve
  → Constraint: RFC 2865 Section 5.8 says only what the attribute carries; reserving the address against the pool is Ze's own uniqueness invariant, not an RFC requirement, so no RFC row is added

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/plugins/pool/register.go` (781L) - `runPlugin`: `pendingResult` closure variable; `verifyPoolConfig` parse-and-discard; apply refusal on live allocations; `handle` (IPv4), `handlePrefix` (IPv6), `onSessionDown` releasing by `sessionAddr.poolName`; `sessionAddrs` and `sessionPrefixes` (`sync.Map`)
- [ ] `internal/component/l2tp/plugins/pool/pool.go` - `ipv4Pool`: one contiguous range, bitmap, `allocate`, `release` (out-of-range is a no-op), `stats`
- [ ] `internal/component/l2tp/plugins/pool/pool_v6.go` - `ipv6PrefixPool`: `allocate`, `release`, `prefixToIndex`
- [ ] `internal/component/l2tp/plugins/pool/l2tppool.go` - `parsePoolConfig`, `parseFullPoolConfig`, `poolConfigResult` (holds built `*ipv4Pool` and `*ipv6PrefixPool` objects, not a declarative description)
- [ ] `test/l2tp/pool-basic.ci`, `test/l2tp/session-auth-pool.ci` - boot only, no reload
- [ ] `test/reload/dhcpserver-range-reload.ci` - the model for a plugin reload `.ci` (SIGHUP fixture, install log line as the discriminating assertion)

**Behavior to preserve:**
- Boot install, `show l2tp pool` output, allocation order, named pools selected by RADIUS Framed-Pool, release on `subevents.SessionDown` for both L2TP and PPPoE (`spec-lifecycle-invariants` D-1).
- A RADIUS Framed-IP-Address outside every range is used as today, with the selected pool's gateway and DNS.

**Behavior to change:**
- A reload installs the committed pool config.
- A live allocation survives the reload, reserved in whichever new pool's range holds it.
- The live-allocation refusal is deleted, not kept beside the carry (`ai/rules/no-layering.md`).
- A RADIUS-assigned address inside a range is reserved in that pool for the session's life.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator commit or SIGHUP; the engine sends `config-verify` then `config-apply` for root `l2tp` to `l2tp-pool`.

### Transformation Path
1. Verify: the `l2tp` section is parsed into a declarative pool description (ranges, gateway, DNS, named pools, IPv6 blocks and delegation lengths). A parse error refuses the reload, as today.
2. Apply: when the description differs from the running one, new pool objects are built from it; under the pool locks, every live session record is re-marked in the new pool whose range holds its address or prefix, and the record is rewritten to name that pool; the new pools replace the old.
3. Rollback: the description the apply replaced is installed back the same way, carrying the allocations made since.
4. Allocation: a RADIUS Framed-IP-Address inside a range marks that bit and records the pool; a bit already set refuses the request.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> plugin | `config-verify`, `config-apply`, `config-rollback` callbacks | `TestPoolConfigTxAppliesReloadedRanges` |
| PPP -> plugin | `l2tp.RegisterPoolHandler` (`handle`) | `TestPoolRADIUSAddressInRangeIsReserved` |
| Bus -> plugin | `subevents.SessionDown` (`onSessionDown`) | `TestPoolReloadCarriedAddressReleasedOnSessionDown` |

### Integration Points
- `sdk.ConfigTx` (`pkg/plugin/sdk/config_tx.go`): parser and installer.
- `onSessionDown`: releases into the pool the session record names, which the carry rewrites.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| Registration over hardcoding | Yes | no new list; the plugin's existing registration |
| No layering | Yes | the hand-written closures and the live-allocation refusal are deleted when `ConfigTx` lands |
| Lazy over eager | N-A | config path, not a hot path |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | `sdk.ConfigTx`'s deep-equal skip needs a declarative description; `poolConfigResult` holds built pools, which never compare equal | `config_tx.go` "Apply installs nothing when the candidate deeply equals the running config" | every unrelated `l2tp` edit rebuilds the pools; still correct with the carry, only wasteful | unit test: an `l2tp` edit outside `pool` installs nothing | unvalidated |
| A-2 | `ipv4Pool.release` and `ipv6PrefixPool.release` of an address outside the range do nothing | `pool.go` `release` returns when out of range; `TestIPv6PrefixPoolReleaseOutOfRange` | an orphan release corrupts a bitmap | read 2026-10-28 for IPv4 | IPv4 confirmed 2026-10-28 |

### Risks
| ID | Risk | Detection | Mitigation |
|----|------|-----------|------------|
| R-1 | An allocation marks a bit in the old pool and stores its session record after the carry scanned the records, so the new bitmap misses it and the address is handed out twice | `-race` test that allocates concurrently with an apply | the bit mark and the record store happen under the same pool-plugin lock the install takes for writing |
| R-2 | A named pool's range moves so a carried address now sits in another pool; release by the old name frees nothing and the reservation leaks | `TestPoolReloadCarriedAddressReleasedOnSessionDown` | the carry rewrites the record's pool name to the pool that reserved it |
| R-3 | Two live subscribers already hold the same address (the second defect, before this fix) when a reload carries them | carry finds the bit already set | the carry keeps both sessions and logs a warning naming the address and both session keys; it never ends a session |

## Blast Radius
| Question | Answer |
|----------|--------|
| What else reads the pools? | `showPool`, `handle`, `handlePrefix`, `releasePrefix`, `onSessionDown`, all in `register.go` |
| What reaches the change? | every L2TP and PPPoE subscriber address request and teardown |
| How is it reverted? | per-phase commits |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP after widening the default range and adding a named pool | → | `ConfigTx` verify and apply in `l2tp-pool` | `test/reload/l2tp-pool-range-reload.ci` |
| RADIUS Access-Accept with a Framed-IP-Address inside the range | → | `handle` reserves the bit | `TestPoolRADIUSAddressInRangeIsReserved` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A reload widens the default IPv4 range | after the commit, `show l2tp pool` reports the new total, and a new subscriber can receive an address from the added part |
| AC-2 | A reload adds a named IPv4 pool and an IPv6 block | a subscriber whose RADIUS Framed-Pool names the new pool receives an address from it; the new IPv6 block delegates prefixes |
| AC-3 | A reload changes the pools while subscribers hold addresses | the reload is accepted; every live subscriber keeps its address; `show l2tp pool` counts each carried address as allocated in the pool whose range holds it; no new subscriber receives a carried address |
| AC-4 | A reload removes the range a live subscriber's address is in | the reload is accepted; the subscriber keeps its address until its session ends; one info line names how many allocations no pool holds; the release at session-down changes no pool |
| AC-5 | A carried subscriber's session ends after a reload that moved its address into another named pool | the address returns to the pool that reserved it and is allocatable again |
| AC-6 | A reload is rolled back after the pool apply | the previous pools are installed, and allocations made between apply and rollback are carried |
| AC-7 | A reload whose pool block fails to parse | the reload is refused at verify with the parse error, and the running pools are unchanged |
| AC-8 | An `l2tp` edit outside `pool` | the pools are not rebuilt (no install line logged) |
| AC-9 | RADIUS assigns a Framed-IP-Address inside a configured range | the bit is marked for the session's life, the pool never hands that address to another subscriber, and the address is released at session-down |
| AC-10 | RADIUS assigns a Framed-IP-Address inside a range whose bit is already held | the address request is refused with a reason naming the address as in use; no subscriber is given an address another subscriber holds |

## End-to-End User Stories
| # | Story | Path | Test |
|---|-------|------|------|
| 1 | An operator running out of addresses adds a second range without dropping subscribers | commit, reload, next subscriber served from the new range | `test/reload/l2tp-pool-range-reload.ci` plus `TestPoolReloadCarriesLiveAllocations` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPoolConfigTxAppliesReloadedRanges` | `internal/component/l2tp/plugins/pool/config_tx_test.go` | AC-1, AC-2 through verify then apply | |
| `TestPoolReloadCarriesLiveAllocations` | same | AC-3: IPv4 and IPv6 allocations marked in the new pools; allocation after apply skips them | |
| `TestPoolReloadOrphanKeepsAddress` | same | AC-4 | |
| `TestPoolReloadCarriedAddressReleasedOnSessionDown` | same | AC-5, R-2 | |
| `TestPoolConfigTxRollbackCarriesAllocations` | same | AC-6 | |
| `TestPoolVerifyRefusesBadPool` | same | AC-7 | |
| `TestPoolUnrelatedL2TPEditInstallsNothing` | same | AC-8, A-1 | |
| `TestPoolAllocateConcurrentWithApply` | same, `-race` | R-1 | |
| `TestPoolRADIUSAddressInRangeIsReserved` | `internal/component/l2tp/plugins/pool/register_test.go` | AC-9 | |
| `TestPoolRADIUSAddressAlreadyHeldIsRefused` | same | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the spec adds no numeric input; range parsing is unchanged | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `l2tp-pool-range-reload` | `test/reload/` | boot with a default range, SIGHUP to a wider range plus a named pool, assert the install log line carries the new totals. Discrimination: with the apply handler a no-op (today's code) the line never appears after the reload | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | config-only fix with no wire-visible change beyond which address IPCP offers; the address assignment path is already covered by the PPPoE and L2TP interop scenarios | - |

## Files to Modify
- `internal/component/l2tp/plugins/pool/register.go` - `runPlugin` registers `sdk.NewConfigTx`; the closures, `pendingResult` and the live-allocation refusal are deleted; `handle` reserves an in-range RADIUS address; the session record names the reserving pool
- `internal/component/l2tp/plugins/pool/l2tppool.go` - parse yields a declarative description; a builder makes the pools from it
- `internal/component/l2tp/plugins/pool/pool.go`, `pool_v6.go` - mark a given address or prefix (reserve), reporting whether it was in range and whether it was already held
- `docs/guide/l2tp.md` - what a reload does to the pools and to live subscribers; RADIUS addresses inside a range are reserved

## Files to Create
- `internal/component/l2tp/plugins/pool/config_tx_test.go`
- `test/reload/l2tp-pool-range-reload.ci`

### Integration Checklist
| Item | Needed | Notes |
|------|--------|-------|
| YANG schema | No | no leaf added |
| CLI grammar | No | `show l2tp pool` unchanged |
| Doctor check | No | |
| Env var | No | |

### Documentation Update Checklist (BLOCKING)
| # | Trigger | Needed | Where |
|---|---------|--------|-------|
| 1 | Reload behavior changed | Yes | `docs/guide/l2tp.md` pool section |
| 2 | Architecture changed | Yes, one paragraph | `docs/architecture/l2tp/subscriber-session-model.md`, pool ownership |

## Implementation Steps
1. **Phase 1: config transaction.** Declarative description, builder, `ConfigTx` registration, carry on install, closures and refusal deleted. Tests: AC-1 to AC-8, R-1, R-2, then `l2tp-pool-range-reload.ci` red against today's binary and green after.
2. **Phase 2: RADIUS in-range reservation.** `handle` marks and records; release frees it; carry covers it. Tests: AC-9, AC-10.
3. **Phase 3: docs.** Both pages in the same commit as the behavior they describe; the journal rows marked `fixed:`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the record store and the bit mark share the lock the install takes; a carried address is released into the pool that reserved it |
| No layering | no trace of `pendingResult` or the live-allocation refusal remains |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Reload applies | `l2tp-pool-range-reload.ci` green, recorded red against the pre-fix binary |
| Carry | `TestPoolReloadCarriesLiveAllocations` |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Address uniqueness | no path leaves one address held by two subscribers (AC-10, R-1, R-3) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Functional test fails | check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The session records, not the bitmaps, are the truth about who holds what (accel-ppp #318 says the same). A rebuilt bitmap is derived from them, so a reload never has to choose between refusing and losing subscribers.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-10-28 | `runPlugin` pool reload apply installs nothing |
| `two-owners-share-one-name.md` | 2026-10-28 | RADIUS Framed-IP-Address inside a range not reserved |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Carry live allocations into the new pools; no refusal | keep "refuse while any allocation is live", moved to verify | a BNG always has live subscribers, so the refusal makes every pool edit need a full disconnect; accel-ppp #318 reconciles instead. The refusal also judged allocations at verify that can change before apply |
| An orphaned address stays with its session | disconnect it; an operator knob like accel-ppp's `reload-orphan` | keeping is accel-ppp's default and ends no session the operator did not ask to end; a knob is an option nobody asked for (`ai/rules/simplicity.md`); `clear` commands already end a session on demand |
| `sdk.ConfigTx` | repair the hand-written closures | the same helper every reload fix in `spec-plugin-reload-applies-or-refuses` uses; it owns verify-to-apply hand-off and rollback |

## Known Limitations
- A carried subscriber keeps the gateway and DNS it negotiated; a reload that changes them reaches new sessions only, as with accel-ppp.

## RFC Documentation (Scope: protocol)
- No RFC requirement is added or changed; RFC 2865 Section 5.8 comments in `handle` stay byte-exact.

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
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-l2tp-pool-reload-applies.md` only, in the same `./le commit create` script
