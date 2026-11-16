# Spec: l2tp-pool-reload-applies -- a committed pool edit reaches the running pools

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | plugin |
| Priority | P1 |
| Release reason | Shipped defect: a reload that edits `l2tp pool` reports success and changes nothing, and a RADIUS-assigned address inside a pool range can be handed to a second subscriber |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Doors: `ai/doors/system.md`, `ai/doors/access-security.md` (l2tp, PPPoE, PPP).

One-off work admission: the owner answered "Go." after being asked to approve
`ZE_SPEC_WIP_CAP=16` for this spec on 2026-11-15. This authorizes its claim
above the normal cap of 12; it does not change the standing cap.

## Blocker

The remaining implementation commit depends on the active PPP primitive work.
`internal/component/l2tp/ppp/start_session.go` (`StartSession`) now transfers
explicit frame and data-plane owners, and the migrated admission test uses
that API. `internal/component/l2tp/pppoe/subsystem_access.go`
(`prepareAccessConfiguration`) refuses interface indices that the PPP routing
key cannot represent. Those producers and their callers are not yet committed.
The pool identity change must land with a coherent producer checkpoint and
the coupled metadata API, callers, tests and RFC records.

The pool implementation is written. Its scoped race tests, eight Linux
lifetime tests on Ze's system, validation counterfactuals and final daemon
reload regression passed. Validation landed in `ea14ad498d`. The remaining
source, documentation and proof records stay in the shared checkout; the
active PPP owners have agreed to assemble the coupled commit. No push is
authorized.

Resume when that exact commit scope is available and its producer checkpoint
lands. Inspect what it carried, commit any remaining owned changes, then run
the independent closure review and the two closure commits. The next review
is round five; no sixth round has been authorized. Detailed evidence and
scope provenance remain in the spec's current session state.

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

Two more defects of the same ownership question joined on 2026-11-15; each was
read at its producer.

- **An answer no session receives leaks its address.** `startPoolDrain`
  (`internal/component/l2tp/drain.go`) and `startPPPoEPoolDrain`
  (`internal/component/l2tp/pppoe/drain.go`) call the pool handler, then
  `d.IPResponse`. `handle` has already stored the allocation in `sessionAddrs`.
  When the session ended while its IPv4 request was queued (PADT, CDN or a
  timeout), `IPResponse` returns `ErrSessionNotFound`, the drain only logs it,
  and the session-down release has already found nothing, so the address never
  returns. The pool drains toward exhaustion, and its live count never reaches
  zero, which today also refuses every reload. The lifetime half (a successor
  that drew the same key taking the stale answer) is part 1 of
  `spec-l2tp-session-identity-and-publish-stall`; this spec owns the release.
- **Verify accepts pools that cannot be served exactly.** `parseIPv4Pool`
  checks only that the range is ordered and misses the gateway. It accepts
  named pools overlapping each other or the default pool, so one address can
  reach two subscribers. It accepts any size, so 1.0.0.0-127.255.255.255
  builds a 254 MiB bitmap, twice (verify and configure both build it). In
  `newIPv4Pool`, 0.0.0.1-255.255.255.255 wraps the uint32 word count to 0,
  so every request is answered "pool exhausted" with nothing saying why. It
  accepts a range holding 0.0.0.0 or 255.255.255.255. And it parses
  `dns-primary` and `dns-secondary` without checking they are IPv4. A value
  that is silently wrong must not be reachable, and a config error is refused
  at verify (`ai/rules/principles.md`, `ai/doors/system.md`).

Verification must also distinguish a malformed present pool block from an
intentional removal. A missing root or pool block remains a valid removal.
Empty root, pool and keyed-list objects retain their empty meaning. A present
scalar, array or null container or named-list entry is refused before apply,
so it cannot silently clear the running pools (AC-7).

Goal: a committed pool edit is installed by the reload that commits it, live
subscribers keep their addresses across it, no address is held by two
subscribers at once, an address whose answer reached no session returns to
its pool, and verify refuses a pool the plugin cannot serve exactly.

The same selected-pool path must serve a named-only IPv4 configuration.
`handle` currently refuses a missing default pool before reading RADIUS
`Framed-Pool`, even when that attribute selects a configured named pool.
Pool selection must precede that refusal; a request selecting no pool still
refuses when no default exists (AC-19).

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
- [ ] `docs/architecture/l2tp/bng-1-radius-attributes.md` - canonical authorization metadata storage and downstream profile readers
- [ ] `docs/functional-tests.md` - draft, state assertions, stress/promotion and the zetest-only command scope

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2865.md` - Section 5.8 Framed-IP-Address, the attribute whose address the pool must reserve
  → Constraint: RFC 2865 Section 5.8 says only what the attribute carries; reserving the address against the pool is Ze's own uniqueness invariant, not an RFC requirement, so no RFC row is added

## Current Behavior (MANDATORY)

This section records the pre-fix behavior that motivated the work. Phase 4
is final documentation authoring; status remains in-progress while scoped
verification, discrimination, draft stress/promotion and the Review Gate
remain to be completed in the main thread.

**Source files read:**
- [ ] `internal/component/l2tp/plugins/pool/register.go` (781L) - `runPlugin`: `pendingResult` closure variable; `verifyPoolConfig` parse-and-discard; apply refusal on live allocations; `handle` (IPv4), `handlePrefix` (IPv6), `onSessionDown` releasing by `sessionAddr.poolName`; `sessionAddrs` and `sessionPrefixes` (`sync.Map`)
- [ ] `internal/component/l2tp/plugins/pool/pool.go` - `ipv4Pool`: one contiguous range, bitmap, `allocate`, `release` (out-of-range is a no-op), `stats`
- [ ] `internal/component/l2tp/plugins/pool/pool_v6.go` - `ipv6PrefixPool`: `allocate`, `release`, `prefixToIndex`
- [ ] `internal/component/l2tp/plugins/pool/l2tppool.go` - `parsePoolConfig`, `parseFullPoolConfig`, `poolConfigResult` (holds built `*ipv4Pool` and `*ipv6PrefixPool` objects, not a declarative description)
- [ ] `test/l2tp/pool-basic.ci`, `test/l2tp/session-auth-pool.ci` - boot only, no reload
- [ ] `test/reload/dhcpserver-range-reload.ci` - the model for a plugin reload `.ci` (SIGHUP fixture, install log line as the discriminating assertion)
- [ ] `internal/component/l2tp/drain.go` `startPoolDrain`, `internal/component/l2tp/pppoe/drain.go` `startPPPoEPoolDrain` - call the handler, then `d.IPResponse`; an error is logged and nothing else (read 2026-11-15)
- [ ] `internal/component/l2tp/plugins/pool/register.go` `parseIPv4Pool`, `pool.go` `newIPv4Pool` - the range checks are order and gateway only; the word count is `(size + 63) / 64` in uint32 (read 2026-11-15)

**Behavior to preserve:**
- Boot install, `show l2tp pool` output, allocation order, named pools selected by RADIUS Framed-Pool, release on `subevents.SessionDown` for both L2TP and PPPoE (`spec-lifecycle-invariants` D-1).
- A RADIUS Framed-IP-Address outside every range is used as today, with the selected pool's gateway and DNS.

**Behavior to change:**
- A reload installs the committed pool config.
- A live allocation survives the reload, reserved in whichever new pool's range holds it.
- The live-allocation refusal is deleted, not kept beside the carry (`ai/rules/no-layering.md`).
- A RADIUS-assigned address inside a range is reserved in that pool for the session's life.
- An allocation whose answer `IPResponse` cannot deliver is released into the pool that made it.
- Verify refuses overlapping pools, an oversized range, a range holding 0.0.0.0 or 255.255.255.255, and a non-IPv4 DNS server, each naming the leaf and the fix.

## Existing Mechanisms (MANDATORY)

Added 2026-11-15, when the two defects above joined; the rows restate what the
sections above already decided.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Config transaction helper | `pkg/plugin/sdk/config_tx.go` `sdk.NewConfigTx` | Verify, apply and rollback with a deep-equal skip, used by every plugin of `spec-plugin-reload-applies-or-refuses` | Replaces the four hand-written closures (Key Design Decisions) |
| Session-down release | `internal/component/l2tp/plugins/pool/register.go` `onSessionDown`, `sessionAddrs` | Releases the address the session record names when `subevents.SessionDown` arrives | Extends: the carry rewrites the record's pool; an undeliverable answer is released by the same release path |
| Pool drains | `internal/component/l2tp/drain.go` `startPoolDrain`, `pppoe/drain.go` `startPPPoEPoolDrain` | Hand the handler's answer to `d.IPResponse` and log a failure | Extends: a failed delivery releases the allocation; the lifetime check stays with `spec-l2tp-session-identity-and-publish-stall` |
| Range parse | `register.go` `parseIPv4Pool` | Refuses an unordered range and a gateway inside it | Extends: the overlap, size, network and broadcast, and DNS-family refusals join it, in verify |
| accel-ppp reload reconciliation | accel-ppp PR #318 | Rebuilds pools from live sessions on reload | Followed for the carry (Design Insights) |

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
| PPP -> plugin | shared registered pool handler (`handle`), carrying the admitted opaque SubscriberID | `TestPoolRADIUSAddressInRangeIsReserved`, `TestPoolTransportIdentitySurvivesReloadAndRelease` |
| Bus -> plugin | `subevents.SessionDown` (`onSessionDown`) | `TestPoolReloadCarriedAddressReleasedOnSessionDown` |

### Pool status response population

AC-3 requires the status answer to include the named pool that holds a carried
reservation. The existing default statistics stay at the top level. Optional
`named-pools` and `named-ipv6-pools` maps add named statistics without removing
or changing those fields. A named-only configuration returns its pool summaries
instead of the false "no pool configured" status.

Search evidence: searched `internal/`, `cmd/`, `pkg/` and `test/`, including Go,
`.ci`, Python, shell and YANG sources, for `show l2tp pool`, `poolShowResult`,
`poolV6ShowResult`, `named-pools` and `named-ipv6-pools`. The known writer is
`runPlugin`'s command callback, and the known JSON reader control is
`l2tpPoolReloadState`. No Python or shell reader of this status answer was found.
The `.ci` `radius-framed-ip` reads the separate `named-pools=1` installation log,
which this status change does not alter.

| Response | Role | Source | Carrier | Disposition |
|----------|------|--------|---------|-------------|
| Pool status JSON | writer | `register.go` `showPool`, `showPoolSnapshot` | `show l2tp pool` command answer | Existing default fields retained, named maps added |
| same | writer | `pkg/plugin/sdk/sdk_callbacks.go` `executeCommandOutput` | `ExecuteCommandOutput.Data` | Generic JSON marshaling unchanged |
| same | reader | `internal/test/fixture/l2tp_pool_reload.go` `l2tpPoolReloadState` | SDK command dispatch | Existing `total` and `allocated` retained |
| same | reader | `internal/test/fixture/l2tp_pool_reload.go` `l2tpPoolReloadNamedState`, `internal/test/fixture/l2tp_pool_identity.go` `l2tpPoolReloadIdentityState` | SDK command dispatch after real reload and subscriber teardown | Exact named IPv4/IPv6 maps and named IPv4 ownership counts asserted |
| same | reader | `internal/le/interoplab/topology/check_access.go` pool check | REST command execution | Existing `allocated` and `sessions` retained |
| same | reader | `show_pool_test.go` `poolShowRead` | JSON marshaling of the command answer | New named maps and default-field preservation asserted |
| same | reader | Operator and API clients outside this repository | Generic command response | Default field names and meanings retained, optional named summaries documented |

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
| A-1 | `sdk.ConfigTx`'s deep-equal skip needs a declarative description; `poolConfigResult` holds built pools, which never compare equal | `config_tx.go` "Apply installs nothing when the candidate deeply equals the running config" | every unrelated `l2tp` edit rebuilds the pools; still correct with the carry, only wasteful | `TestPoolUnrelatedL2TPEditInstallsNothing`; main's full pool-package PASS in `scratch/job-unit-pkg-f6b24a23.log` | confirmed 2026-11-15 |
| A-2 | `ipv4Pool.release` and `ipv6PrefixPool.release` of an address outside the range do nothing | `pool.go` `release` returns when out of range; `TestIPv6PrefixPoolReleaseOutOfRange` | an orphan release corrupts a bitmap | `TestIPv6PrefixPoolReleaseOutOfRange` and `TestPoolReloadOrphanKeepsAddress`; main's full pool-package PASS in `scratch/job-unit-pkg-f6b24a23.log` | confirmed for the covered IPv4/IPv6 release cases 2026-11-15 |
| A-3 | The numeric PPP driver pair identifies a subscriber across L2TP and PPPoE | The original pool and metadata stores used tunnel/session pairs; `subscriber.Session.PPPKey` explicitly states that those pairs are not unique across transports | one subscriber receives another transport's cached address/profile, or releases its reservation | `TestPoolTransportIdentitySurvivesReloadAndRelease`, `TestPoolPPPoETeardownPreservesEqualL2TPPrefix`, `TestAuthMetadataSeparatesTransportIdentities`; main's focused PASS in `scratch/job-unit-pkg-6ebc2cc5.log` | broken, corrected with existing canonical `Session.ID`; numeric coordinates remain display only, 2026-11-15 |

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
| AC-11 | An IPv4 request waits in the drain while its session ends, on L2TP and on PPPoE | the allocation is released into its pool, `show l2tp pool` does not count it, and after every session ends the live count is zero |
| AC-12 | A config whose named pools overlap each other, or the default pool | refused at verify, naming both pools and the overlapping addresses |
| AC-13 | A range larger than the plugin's stated bound, including 0.0.0.1-255.255.255.255 | refused at verify, naming the range, its size and the bound; no bitmap is built for it |
| AC-14 | A range holding 0.0.0.0 or 255.255.255.255 | refused at verify, naming the address |
| AC-15 | `dns-primary` or `dns-secondary` set to an IPv6 address | refused at verify, naming the leaf |
| AC-16 | Default or named IPv6 pool blocks overlap | refused at verify, naming both pools and their overlapping blocks |
| AC-17 | RADIUS assigns an IPv4 address already held outside every current range, including an allocation orphaned by reload | request refused naming the address as in use; the existing subscriber retains it; after that session ends the same RADIUS address is admissible |
| AC-18 | RADIUS supplies a delegated IPv6 prefix | refuse any overlap with a live delegated prefix, including external and carried prefixes; otherwise keep the requested prefix and reserve every overlapping configured slot until release. Disjoint RADIUS prefixes may share a coarse pool slot, which stays reserved until their last release |
| AC-19 | Only a named IPv4 pool exists, and RADIUS Framed-Pool selects it | the subscriber receives that pool's address, gateway and DNS; a request selecting no pool remains refused because no default pool exists |
| AC-20 | L2TP and PPPoE subscribers have equal numeric PPP driver pairs | distinct canonical subscriber IDs retain distinct allocations, tokens, profiles and usernames; reload, rollback and cleanup of either subscriber preserve the other; PPPoE teardown cannot release the equal pair's L2TP-only prefix |
| AC-21 | A production NAS authorization or pool request lacks its canonical subscriber ID | the handler or transport drain refuses it before provider selection, realm relay or subscriber state changes, names the missing identity and reports the existing internal-defect doctor code; requests with valid opaque identities remain usable |

## Canonical subscriber identity correction

The original pair key violated the existing subscriber identity contract.
This correction carries the existing canonical ID through authorization,
allocation and cleanup; it introduces no new session epoch or lifetime.
The separate session-identity/publish-stall work retains that lifetime scope.
The pool record stores numeric coordinates only for status and logs.

Metadata census: source searches across production Go in `internal/`,
`pkg/` and `cmd/` for Store/Load/ClearSessionMetadata found the following
sixteen call sites, including two zetest instrumentation calls. The definitions in `session_metadata.go` were the known
control, and `handler.go`'s Access-Accept store was the writer control.

| Metadata operation | Producer or consumer | Identity source |
|--------------------|----------------------|-----------------|
| Store | authradius `radiusAuth.doRADIUS` | Auth request SubscriberID |
| Load, two sites | pool `handle`, `handlePrefix` | IP request SubscriberID; canonical L2TP ID for prefix API |
| Load | L2TP `subscriberRouteObserver.OnSessionIPUp` | Canonical L2TP ID |
| Load | L2TP `startSessionTimeouts` | Canonical L2TP ID |
| Load | L2TP `subscriberBridge.onSessionUp` | Existing registry session ID |
| Load | authradius `onSessionIPAssigned` | Canonical L2TP ID |
| Load | shaper `onSessionUp` | Canonical L2TP ID |
| Load | CoS `onSessionUp` | Canonical L2TP ID |
| Load | PPPoE `onSessionUp` | Existing subscriber Session.ID |
| Clear | L2TP `L2TPTunnel.removeSession` | Canonical L2TP ID |
| Clear, two sites | L2TP `handleLCPDown` | Canonical L2TP ID |
| Clear | PPPoE `withdrawSubscriber` | Existing subscriber Session.ID |
| Store and Clear | zetest `fakel2tp.poolAddressForAccess` | Command-selected explicit L2TP or PPPoE canonical ID |

Carrier census: L2TP `l2tpReactor.handleKernelSuccess` and PPPoE
`InterfaceServer.handlePADR` construct StartSession with the existing
canonical ID. `Driver.spawnSession` retains it privately, and
`pppSession.awaitAuthDecision` and `awaitIPDecision` stamp it on requests.
The pool and RADIUS consumers read that opaque ID; teardown uses Session.ID.

| Carrier | Writer | Reader / boundary |
|---------|--------|-------------------|
| `StartSession.SubscriberID` | L2TP `handleKernelSuccess`, PPPoE `handlePADR`, before numeric narrowing | `Driver.spawnSession` copies into private `pppSession.subscriberID` |
| `EventAuthRequest.SubscriberID` | `awaitAuthDecision`, shared by PAP, CHAP, MS-CHAPv2 and no-auth request producers | Both production auth drains require it before provider or realm activity; authradius stores metadata and auth-result events retain it unchanged |
| `EventIPRequest.SubscriberID` | `awaitIPDecision`, shared by IPCP and IPv6CP request producers | Both production pool drains require it; pool state uses it directly |
| Undelivered answer ID plus AllocationID | Both production pool drains after failed `IPResponse` | `onPoolAnswerUndelivered` releases only the matching canonical owner and allocation token |
| `SessionDownPayload.Session.ID` | L2TP subscriber bridge and PPPoE subscriber teardown; zetest `poolRelease` for the daemon proof | Pool address and prefix cleanup use the canonical ID |
| L2TP prefix request/release pair | Existing L2TP-only prefix API | Derives the canonical L2TP ID; an equal PPPoE pair cannot release its prefix |

These state and request APIs are Go in-process carriers. The functional
observer reaches them through zetest-only YANG commands and JSON command
answers; no production non-Go command gains a subscriber-identity argument.
`SessionID` formats the existing AccessType namespace and full transport ID;
PPP copies the result without importing subscriber or interpreting the ID.
Protocol-only PPP callers may omit it, while production NAS drains refuse
omission. No compatibility namespace, successor generation or epoch is added.

### Deviations from Plan

- A-3 widened the original reload/reservation repair to carry the existing
  canonical subscriber ID through admission, requests, metadata and cleanup.
  This repairs shared ownership; the separate session-identity/publish-stall
  spec retains successor-generation and publication-lifetime design.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 reused the numeric PPP pair for shared pool/metadata ownership, so equal L2TP/PPPoE pairs could return or release another subscriber's reservation/profile | `subscriber.Session.ID` already provides cross-transport identity; `PPPKey` explicitly documents pair collisions | Independent producer trace compared L2TP and PPPoE StartSession construction with pool caching and SessionDown cleanup; focused identity/metadata tests passed after correction on 2026-11-15 (`scratch/job-unit-pkg-6ebc2cc5.log`) | Source fix: copy the admitted opaque ID through central auth/NCP emission and key shared state/cleanup by it. Rejected workarounds: ambiguous refcounts, skipped releases or rejecting legitimate equal pairs. No epoch redesign |

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
| `TestPoolRADIUSAddressInRangeIsReserved` | `internal/component/l2tp/plugins/pool/radius_reservation_test.go` | AC-9 | |
| `TestPoolRADIUSAddressAlreadyHeldIsRefused` | same | AC-10 | |
| `TestPoolDrainReleasesUndeliveredAnswer` | `internal/component/l2tp/` and `internal/component/l2tp/pppoe/` drain tests | AC-11, one case per drain | |
| `TestPoolAllocationIdentifiersDoNotWrap` | `internal/component/l2tp/plugins/pool/allocation_id_test.go` | AC-11: dynamic and RADIUS final token succeeds; exhaustion refuses without mutation, same-owner retry keeps its address with token0, and exact-token cleanup still frees the reservation | |
| `TestPoolVerifyRefusesOverlappingPools` | `internal/component/l2tp/plugins/pool/config_validation_test.go` | AC-12 | |
| `TestPoolVerifyRefusesOversizedRange` | same | AC-13, the wrapping range among the cases | |
| `TestPoolVerifyRefusesZeroAndBroadcast` | same | AC-14 | |
| `TestPoolVerifyRefusesNonIPv4DNS` | same | AC-15 | |
| `TestPoolVerifyRefusesOverlappingIPv6Pools` | same | AC-16 | |
| `TestPoolRADIUSAddressOutsideRangesAlreadyHeldIsRefused` | `internal/component/l2tp/plugins/pool/radius_reservation_test.go` | AC-17: two sessions cannot hold the same external RADIUS address | |
| `TestPoolRADIUSOrphanAddressAlreadyHeldIsRefused` | same | AC-17: a removed range does not make a still-held address available to RADIUS | |
| `TestPoolRADIUSPrefixReservesAllOverlappingSlots` | `internal/component/l2tp/plugins/pool/radius_prefix_reservation_test.go` | AC-18: reserve every covered slot across default and named pools until release | |
| `TestPoolRADIUSPrefixOverlappingHeldIsRefused` | same | AC-18: refuse covering, contained and external prefix overlap with another subscriber | |
| `TestPoolRADIUSPrefixOutsideRangesIsPreserved` | same | AC-18: retain unique external RADIUS delegations without requiring a configured pool | |
| `TestPoolRADIUSPrefixCannotOverlapCarriedDelegation` | same | AC-18: a reload-carried delegation remains exclusive against new RADIUS admission | |
| `TestPoolRADIUSDisjointPrefixesSharePoolSlot` | same | AC-18: retain disjoint smaller prefixes and reserve their shared coarse slot until final release | |
| `TestPoolNamedOnlyPoolSelection` | `internal/component/l2tp/plugins/pool/named_pool_only_test.go` | AC-19: selected named-only pool supplies address, gateway and DNS; unselected request remains refused | |
| `TestPoolReloadSplitsLiveDelegation` | `internal/component/l2tp/plugins/pool/config_tx_test.go` | AC-3, AC-5: every overlapping new slot is reserved, even across separate new pools | |
| `TestPoolReloadMergesLiveDelegations` | same | AC-3, AC-5: a coarse slot remains reserved until the last smaller delegation ends | |
| `TestPoolShowIncludesNamedReservationsAfterReload` | `internal/component/l2tp/plugins/pool/show_pool_test.go` | AC-3: JSON answer counts carried reservations in replacement named IPv4 and IPv6 pools | |
| `TestPoolShowNamedOnlyPools` | same | AC-3, AC-19: named-only pools have usable status summaries | |
| `TestPoolShowPreservesDefaultFields` | same | Preserve existing default statistics, session fields and absent-map omission | |
| `TestPoolVerifyRefusesMalformedPresentBlocks` | `internal/component/l2tp/plugins/pool/parse_shape_test.go` | AC-7: present malformed root, pool, container and named-list bodies are refused without changing the running allocator | |
| `TestPoolVerifyAcceptsExplicitRemoval` | same | AC-4, AC-7: actual empty/absent removal forms clear pools while live assignments remain until teardown | |
| `TestPoolTransportIdentitySurvivesReloadAndRelease` | `internal/component/l2tp/plugins/pool/subscriber_identity_test.go` | AC-20: equal pairs across transports retain distinct addresses and tokens through reload/rollback and either cleanup path | |
| `TestPoolPPPoETeardownPreservesEqualL2TPPrefix` | same | AC-20: a transport-local pair cannot release another access type's prefix | |
| `TestPoolTransportIdentityStatusUsernames` | same | AC-20: canonical registry lookup preserves both subscriber usernames | |
| `TestPoolMissingSubscriberIdentityIsRefused` | same | AC-21: missing pool identity is refused before allocation and reported through doctor | |
| `TestPoolDrainRefusesMissingSubscriberIdentity` | L2TP and PPPoE drain identity test files | AC-21: each production drain refuses before invoking the allocator | |
| `TestPoolDrainMissingIdentityDoesNotResolveProvider` | `internal/component/l2tp/drain_subscriber_identity_test.go` | AC-21: L2TP refuses before dynamic provider lookup; a valid opaque ID resolves and invokes exactly once unchanged | |
| `TestAuthDrainRequiresSubscriberIdentity` | L2TP and PPPoE `drain_auth_identity_test.go` | AC-21: each production auth loop refuses before provider, relay, event and pending-authorization actions; valid opaque identity reaches provider and result event unchanged | |
| `TestAuthMetadataSeparatesTransportIdentities` | `internal/component/l2tp/session_metadata_identity_test.go` | AC-20: equal driver pairs retain independent RADIUS profiles and cleanup | |
| `TestAuthMetadataRefusesMissingIdentity` | same | AC-21: an empty ID cannot create a shared authorization entry | |
| `TestSessionID` | `internal/component/l2tp/subscriber/id_test.go` | Existing L2TP/PPPoE spellings, full transport width and open explicit access namespace | |
| `TestAuthRequestSubscriberID` | `internal/component/l2tp/ppp/subscriber_id_test.go` | Every auth method receives the admitted opaque ID at the central emitter; protocol-only omission remains usable | |
| `TestIPRequestSubscriberID` | same | Both NCP families receive the admitted opaque ID; a stale request ID cannot override it | |
| `TestDriverAdmissionSubscriberID` | same | Real queued driver admission retains the StartSession ID through request emission | |
| `TestPoolAnswerReleaseFailureIsReported` | `internal/component/l2tp/subscriber/events/pool_answer_test.go` | AC-11: cleanup publication failure reports the existing internal-defect code | |
| `TestPoolAnswerSynchronousReleaseDoesNotReportFailure` | `internal/component/l2tp/subscriber/events/pool_answer_recipient_test.go` | Synchronous cleanup delivery with recipient count zero remains successful | |
| `TestRADIUSAuthRequiresSubscriberIdentity` | `internal/component/l2tp/plugins/authradius/subscriber_identity_test.go` | AC-21: direct RADIUS handler refuses a missing ID before no-auth admission; a valid opaque ID retains synchronous no-auth acceptance without invoking the asynchronous callback. Untagged guard test authored, main verification pending | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IPv4 range size | 1 to 1,048,576 addresses per pool | 1,048,576 | N/A (an unordered range is refused today) | 1,048,577, and 2^32 - 1 (0.0.0.1-255.255.255.255) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `l2tp-pool-range-reload` | `test/draft/reload/l2tp-pool-range-reload.ci`, then `test/reload/l2tp-pool-range-reload.ci` after stress proof | AC-1/2/3: allocate the boot address, widen the default pool by real SIGHUP, require the new total and added address, then exact named IPv4/IPv6 map fields and reservations. AC-20: allocate a distinct PPPoE address under the L2TP owner's exact numeric pair, widen the named pool by another SIGHUP, and require either real subscriber-bus teardown to preserve the other owner's address and sole reservation; final release leaves zero named IPv4 reservations. The old apply no-op fails runtime totals, and tuple-keyed caching fails the distinct-address assertion. This portable proof makes no wire/kernel claim | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | config-only fix with no wire-visible change beyond which address IPCP offers; the address assignment path is already covered by the PPPoE and L2TP interop scenarios | - |

## Files to Modify
- `internal/component/l2tp/plugins/pool/register.go` - `runPlugin` registers `sdk.NewConfigTx`; the closures, `pendingResult` and the live-allocation refusal are deleted; `handle` reserves an in-range RADIUS address; the session record names the reserving pool
- `internal/component/l2tp/plugins/pool/l2tppool.go` - retains logger helpers after parsing and pool construction move to `config.go`
- `internal/component/l2tp/plugins/pool/pool.go`, `pool_v6.go` - mark a given address or prefix (reserve), reporting whether it was in range and whether it was already held
- `docs/guide/l2tp.md` - what a reload does to the pools and to live subscribers; RADIUS addresses inside a range are reserved; the pool refusals verify gives
- `internal/component/l2tp/drain.go`, `internal/component/l2tp/pppoe/drain.go` - a failed `IPResponse` releases the allocation (AC-11)
- `internal/component/l2tp/ppp/start_session.go`, `internal/component/l2tp/ppp/session.go`, `internal/component/l2tp/ppp/manager.go` - carry the transport's admitted opaque subscriber ID
- `internal/component/l2tp/ppp/auth_events.go`, `internal/component/l2tp/ppp/ip_events.go`, `internal/component/l2tp/ppp/auth.go`, `internal/component/l2tp/ppp/ncp.go` - request fields and central ID stamps
- `internal/component/l2tp/reactor_kernel.go`, `internal/component/l2tp/pppoe/server.go`, `internal/component/l2tp/pppoe/subsystem.go`, `internal/component/l2tp/subscriber_bridge.go` - transport producers and canonical metadata/registry identity
- `internal/component/l2tp/session_metadata.go`, `internal/component/l2tp/route_observer.go`, `internal/component/l2tp/session_timeout.go`, `internal/component/l2tp/session.go` - canonical metadata storage, readers and teardown
- `internal/component/l2tp/plugins/authradius/handler.go`, `internal/component/l2tp/plugins/authradius/acct.go`, `internal/component/l2tp/plugins/shaper/shaper.go`, `internal/plugins/cos/handler.go` - existing metadata writer/readers use the canonical identity
- `internal/component/l2tp/subscriber/session.go` - distinguish driver coordinates from shared state identity
- `internal/component/l2tp/plugins/pool/register_test.go`, `internal/component/l2tp/plugins/pool/pool_test.go`, `internal/component/l2tp/plugins/pool/pool_v6_test.go`, `internal/component/l2tp/plugins/pool/named_pool_shape_test.go`, `internal/component/l2tp/plugins/pool/session_reader_contract_test.go`, `internal/component/l2tp/plugins/pool/rfc2548_dns_source_test.go` - preserve existing assertions with explicit canonical test identities
- `internal/component/l2tp/drain_test.go`, `internal/component/l2tp/route_observer_test.go`, `internal/component/l2tp/session_event_order_test.go`, `internal/component/l2tp/session_metadata_test.go`, `internal/component/l2tp/subscriber_bridge_test.go`, `internal/component/l2tp/subscriber_lifetime_linux_test.go`, `internal/component/l2tp/pppoe/subsystem_test.go`, `internal/component/l2tp/pppoe/subscriber_lifetime_linux_test.go` - existing transport fixtures use canonical identity
- `internal/component/l2tp/plugins/shaper/filter_rate_test.go`, `internal/component/l2tp/plugins/shaper/shaper_test.go`, `internal/plugins/cos/handler_helpers_test.go` - metadata fixtures retain their existing shaping assertions
- `internal/component/l2tp/plugins/authradius/accounting_port_test.go`, `internal/component/l2tp/plugins/authradius/exclude_test.go`, `internal/component/l2tp/plugins/authradius/panic_guard_test.go`, `internal/component/l2tp/plugins/authradius/nasportid_test.go`, `internal/component/l2tp/plugins/authradius/provider_reload_test.go`, `internal/component/l2tp/plugins/authradius/slot_ownership_test.go` - existing RADIUS fixture identity migration
- `internal/component/l2tp/plugins/authradius/rfc2865_access_challenge_test.go`, `internal/component/l2tp/plugins/authradius/rfc2865_extension_credential_test.go`, `internal/component/l2tp/plugins/authradius/rfc2865_access_request_shape_test.go`, `internal/component/l2tp/plugins/authradius/rfc2865_mschap2_success_test.go`, `internal/component/l2tp/plugins/authradius/rfc2865_handler_test.go`, `internal/component/l2tp/plugins/authradius/rfc2865_test.go`, `internal/component/l2tp/plugins/authradius/rfc2865_nas_obligations_test.go`, `internal/component/l2tp/plugins/authradius/rfc2865_unavailable_service_test.go` - independently judged RFC fixture identity migration
- `internal/component/l2tp/plugins/authradius/rfc2866_zero_marked_login_test.go`, `internal/component/l2tp/plugins/authradius/rfc2869_interim_interval_precedence_test.go`, `internal/component/l2tp/plugins/authradius/rfc2869_access_request_service_test.go`, `internal/component/l2tp/plugins/authradius/rfc3162_nas_attributes_test.go` - independently judged RFC fixture identity migration
- `internal/test/plugins/fakel2tp/fakel2tp.go`, `internal/test/plugins/fakel2tp/yang/ze-fakel2tp-cmd.yang`, `internal/test/plugins/fakel2tp/yang/ze-fakel2tp-api.yang` - register hidden test-only allocation and teardown commands
- `docs/guide/configuration.md`, `docs/guide/plugins.md`, `docs/architecture/l2tp/subscriber-session-model.md`, `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/research/l2tpv2-ze-integration.md`, `docs/functional-tests.md` - behavior, identity boundary and portable proof

## Files to Create
- `internal/component/l2tp/plugins/pool/config_tx_test.go`
- `test/reload/l2tp-pool-range-reload.ci`
- `internal/component/l2tp/plugins/pool/config.go` - immutable descriptions and parsing, separate from the existing logger helpers in `l2tppool.go`
- `internal/component/l2tp/plugins/pool/reload.go` - ConfigTx adapter, atomic installation, carry and exact allocation release
- `internal/component/l2tp/plugins/pool/config_validation.go` - bounded IPv4 descriptions, DNS-family checks and cross-pool overlap refusals
- `internal/component/l2tp/plugins/pool/config_validation_test.go` - validation refusals, valid survivors, numeric boundaries and concrete correction preservation
- `internal/component/l2tp/plugins/pool/config_shape.go` - distinguish absent blocks from malformed present containers and list entries
- `internal/component/l2tp/plugins/pool/parse_shape_test.go` - malformed-candidate refusal and explicit-removal preservation
- `internal/component/l2tp/plugins/pool/radius.go`, `internal/component/l2tp/plugins/pool/radius_reservation_test.go`, `internal/component/l2tp/plugins/pool/radius_prefix_reservation_test.go` - exclusive IPv4/IPv6 RADIUS ownership and release
- `internal/component/l2tp/plugins/pool/named_pool_only_test.go`, `internal/component/l2tp/plugins/pool/show_pool_test.go`, `internal/component/l2tp/plugins/pool/subscriber_identity_test.go` - named-only/status and cross-transport identity regressions
- `internal/component/l2tp/subscriber/id.go`, `internal/component/l2tp/subscriber/id_required.go`, `internal/component/l2tp/subscriber/id_test.go`, `internal/component/l2tp/ppp/subscriber_id_test.go` - existing canonical formatter, missing-ID boundary and carrier tests
- `internal/component/l2tp/session_metadata_identity_test.go`, `internal/component/l2tp/drain_subscriber_identity_test.go`, `internal/component/l2tp/pppoe/drain_subscriber_identity_test.go`, `internal/component/l2tp/drain_auth_identity_test.go`, `internal/component/l2tp/pppoe/drain_auth_identity_test.go` - metadata separation and real production drain boundary tests
- `internal/component/l2tp/plugins/authradius/subscriber_identity_test.go` - direct RADIUS missing-ID guard and valid opaque-ID control, independently of transport-drain guards
- `internal/component/l2tp/drain_pool_release_test.go`, `internal/component/l2tp/pppoe/drain_pool_release_test.go`, `internal/component/l2tp/subscriber/events/pool_answer.go`, `internal/component/l2tp/subscriber/events/pool_answer_test.go`, `internal/component/l2tp/subscriber/events/pool_answer_recipient_test.go` - exact undelivered-answer cleanup and synchronous bus proof
- `internal/test/plugins/fakel2tp/pool.go`, `internal/test/fixture/l2tp_pool_reload.go`, `internal/test/fixture/l2tp_pool_identity.go` - test-only real-handler and bus instrumentation plus daemon observer

Implementation placement: the descriptions and builder live in `config.go`
rather than extending `l2tppool.go`. The latter keeps its existing name and
logger helpers. This separates configuration from logging and changes no
configuration or handler contract.

### Integration Checklist
| Item | Needed | Notes |
|------|--------|-------|
| YANG schema | Test-only | Hidden fakel2tp address/prefix, explicit PPPoE address and subscriber release RPC declarations; no production configuration leaf added |
| CLI grammar | Test-only | `show l2tp pool` spelling unchanged; hidden fakel2tp commands declare their explicit transport context |
| Doctor check | Existing code reused | Missing NAS identity or failed cleanup publication reports internal-defect; no new check/code |
| Env var | No | |

### Documentation Update Checklist (BLOCKING)
| # | Trigger | Needed | Where |
|---|---------|--------|-------|
| 1 | Reload behavior changed | Yes | `docs/guide/l2tp.md` pool section |
| 2 | Architecture changed | Yes | `docs/architecture/l2tp/subscriber-session-model.md`, pool ownership and canonical identity |
| 3 | Pool validation and named selection changed | Yes | `docs/guide/configuration.md`, `docs/guide/plugins.md` |
| 4 | Shared authorization identity changed | Yes | `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/research/l2tpv2-ze-integration.md` |
| 5 | Portable daemon proof and zetest commands added | Yes | `docs/functional-tests.md`; retain its wire/kernel scope boundary |

## Implementation Steps
1. **Phase 1: config transaction.** Declarative description, builder, `ConfigTx` registration, carry on install, closures and refusal deleted. Tests: AC-1 to AC-8, R-1, R-2, then `l2tp-pool-range-reload.ci` red against today's binary and green after.
2. **Phase 2: RADIUS ownership and named selection.** Reserve IPv4 and IPv6 assignments against live records and every overlapping configured slot; preserve unique external assignments; release and carry use the reserving pool. Select the named pool before refusing a missing default. Tests: AC-9, AC-10, AC-17, AC-18, AC-19.
3. **Phase 3: cleanup, validation and canonical identity.** Release only an undelivered answer's matching owner/token; verify rejects malformed present containers, overlaps, excessive size, unusable endpoints and wrong DNS family before building bitmaps. Extend the existing subscriber ID contract from transport admission through central PPP auth/NCP stamps, metadata, pool records, result events and cleanup; both NAS drains refuse missing IDs before side effects. Numeric pairs remain driver/display coordinates; the separate identity/publish-stall spec retains successor-generation and lifetime design. Tests: AC-7, AC-11 to AC-16, AC-20, AC-21 plus the carrier tests.
4. **Phase 4: docs and daemon proof.** Update every page in the documentation checklist with its behavior; preserve all original default/named assertions and add the real pool-handler/subscriber-bus transport-identity lifecycle proof. Main completes scoped verification, discrimination, stress, promotion, affected-suite run and the Review Gate before closure; authored source or this phase number does not mark those checks complete. Journal rows are marked `fixed:` only through closure.

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
| IPv4/IPv6 RADIUS uniqueness and named-only selection | `radius_reservation_test.go`, `radius_prefix_reservation_test.go`, `named_pool_only_test.go` |
| Failed-answer cleanup | Both `TestPoolDrainReleasesUndeliveredAnswer` tests and subscriber event tests |
| Validation before bitmap construction | `config_validation_test.go` and `parse_shape_test.go` |
| Canonical identity through admission, requests, metadata and either teardown | Carrier, metadata, pool/drain identity tests and the daemon functional identity sequence |

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
- A changed IPv6 delegation length can split one live prefix across several new slots, or place several live prefixes in one new slot. Carry reserves every overlapping slot, including slots in separate new pools. Per-slot reservation counts retain a shared slot until its last owner ends. IPv6 blocks are checked for overlap for the same uniqueness reason as IPv4 ranges.
- Each IPv4 pool is limited to 1,048,576 addresses, a 128 KiB bitmap. Verification measures range size in 64 bits before any bitmap is allocated and refuses the next address past that bound.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-10-28 | `runPlugin` pool reload apply installs nothing |
| `two-owners-share-one-name.md` | 2026-10-28 | RADIUS Framed-IP-Address inside a range not reserved |
| `record-written-before-the-operation-succeeds.md` | 2026-11-15 | AC-11: pool drains drop an undeliverable answer and leak its address |
| `bound-wraps-before-it-refuses.md` | 2026-11-15 | AC-12 to AC-15: `parseIPv4Pool` accepts overlap, any size, a wrapping range, 0.0.0.0 or broadcast, non-IPv4 DNS |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Carry live allocations into the new pools; no refusal | keep "refuse while any allocation is live", moved to verify | a BNG always has live subscribers, so the refusal makes every pool edit need a full disconnect; accel-ppp #318 reconciles instead. The refusal also judged allocations at verify that can change before apply |
| An orphaned address stays with its session | disconnect it; an operator knob like accel-ppp's `reload-orphan` | keeping is accel-ppp's default and ends no session the operator did not ask to end; a knob is an option nobody asked for (`ai/rules/simplicity.md`); `clear` commands already end a session on demand |
| `sdk.ConfigTx` | repair the hand-written closures | the same helper every reload fix in `spec-plugin-reload-applies-or-refuses` uses; it owns verify-to-apply hand-off and rollback |

## Known Limitations
- A carried subscriber keeps the gateway and DNS it negotiated; a reload that changes them reaches new sessions only, as with accel-ppp.

## Retained temporary work
| Location | Purpose | Removal condition |
|----------|---------|-------------------|
| `tmp/stress-repro/reload-draft-pattern-l2tp-pool-range-reload-9681e20db2de0e2cb7eee13f0d0e53094d81b4695abc025e4c7d94aef52f6fdf-20261115-220714-999335715.log` | Current 80-run daemon stress evidence; the test is promoted to `test/reload/` and the draft no longer exists | Remove after the promoted-suite evidence and closure record retain the result |
| Session scratch under `tmp/session/2026-11-15-195627/` and its named `le` build | Per-phase handoff, scoped test execution and minimal failure evidence | Remove superseded outputs as each failure clears; retain only evidence needed for unresolved findings until closure |

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
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, plus the main-thread checks owed by the changed surfaces. Agents run no `./le verify` command; GitHub CI verifies a pushed commit. A red in the scoped proof gets a fix before this gate is complete
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
