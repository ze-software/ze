# Spec: bgp-refresh-reapplies-export-policy

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | A route refresh sends the peer an AS_PATH that the export policy never produced, and the stale rail's policy reconciliation has five gaps |
| RFC gaps | rfc2918 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-06 |

Scope is protocol: the wire change is the attributes of a re-advertised route, and
a withdrawal where a denied route used to stay. No config or CLI surface changes.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`bgp-rib` keeps Adj-RIB-Out as sent, after export policy. A refresh hands those
bytes back, and `sendBatchUpdate` writes them with `writeUpdate`, which runs the
export gate again with no replay exemption. A prepend member therefore applies one
extra time on each refresh. It does not accumulate, because `handleSent` drops a
sent event that carries `metaKeyReplay`, so `ribOut` keeps the first bytes. The
whole-chain run of the stale rail reads the same bytes and inherits the defect.

Skipping the chain on a refresh is wrong. RFC 2918 Section 4: "Otherwise, the BGP
speaker shall re-advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried
in the message, based on its outbound route filtering policy." The fix gives the
chain the bytes it should read: the RIB keeps the attributes as they stood before
export policy, and a refresh runs the chain once over those.

This spec owns five more defects of the same reconciliation:

| # | Defect | Producer |
|---|--------|----------|
| D-2 | A policy denial on the stale rail suppresses, and the peer keeps the route it holds | `sendStaleReadvertiseUnit`, the `staleSuppress` case |
| D-3 | A definition change inside an external plugin is invisible to the policy comparison | `applyHotSwappableSettings` compares `ExportPolicy`, which holds member names |
| D-4 | A rebuilt chain with an in-process func member never compares equal, so a no-op reload raises the watermark | `reflect.DeepEqual` in `applyHotSwappableSettings`, and in `peerSettingsEqual` |
| D-5 | A stale decision is made before `writeMu`, so a unit in flight during a reload can write under the old chain with a receipt newer than the watermark | `sendStaleReadvertiseUnit` decides, then `sendBatchUpdatePreFiltered` locks |
| D-6 | A reload changing policy during an initial-sync hold can stall up to 2 minutes at the barrier, then sets the watermark to `MaxUint64` | `raiseExportPolicyFrom`, `joinExportWriters` |

Goal: one chain run over pre-policy attributes on every re-advertisement, the peer
left holding what the current policy produces, and a reload that neither stalls nor
reports a change that did not happen.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` "Policy Filter Chain", "the stale readvertise rail", "final-only export policy reconciliation"
  → Decision: the engine holds no policy and no Loc-RIB; a chain member is reached through `filter-update` on both transports.
  → Constraint: every route-processing step is a filter in the peer's filter list (`ai/META-DECISIONS.md`). No special case may name one member, such as prepend.
- [ ] `docs/architecture/bgp/egress-attribute-rules.md` - the order of egress rules
  → Constraint: the forward rail runs the chain on the received wire and then applies the RFC 4271 Section 9.1.2 local-AS prepend. The replay keeps that order.
- [ ] `docs/architecture/plugin/rib-storage-design.md`, `ai/digests/rib.md` - `pool.RibOut`, interned attributes
  → Constraint: attribute bytes live in `pool.RibOut`, shared and refcounted.
- [ ] `docs/architecture/api/process-protocol.md`, `docs/architecture/api/json-format.md` - the sent event
  → Constraint: a sent event member is a contract for third-party plugins. A new member is additive and has its page.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2918.md` - re-advertisement
  → Constraint: RFC 2918 Section 4, quoted above. The chain runs on a refresh.
- [ ] `rfc/short/rfc7313.md` - BoRR and EoRR
  → Constraint: RFC 7313 Section 4: a speaker "MUST immediately remove any routes from the peer that are still marked as stale" at the EoRR, so with enhanced refresh a route not re-sent is already removed. Without it, nothing removes the route, and that is D-2's case.
- [ ] `rfc/short/rfc9494.md` - LLGR
  → Constraint: `llgr-egress` keeps its own suppress and withdraw verdicts on the opted-member run.

**Key insights:**
- The bytes the chain reads are the defect; the writer is doing what it should.
- There are two rails with two orders: the chain before the local-AS prepend (forward), and after the builder (announce). The stored pre-policy form is the input of the rail that wrote the route.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `sendBatchUpdate` passes `(*Session).writeUpdate` whether or not `batch.Replay`. `sendStaleReadvertiseUnit` calls `decideStaleReadvertise` with no lock held, then writes; `staleSuppress` returns false, nil
- [ ] `internal/component/bgp/reactor/session_write.go` - `writeUpdate` is the gated writer; `writeUpdateGated` runs `egressRouteFilter` when `gate` is true. The comment on `writeUpdatePreFiltered` names the double-prepend risk
- [ ] `internal/component/bgp/reactor/stale_readvertise_chain.go` - `staleChainFor` reads the policy and the watermark together; `exportPolicyNewerThanLocked`; `runStaleChain` runs the whole chain over the rebuilt body when `judgeAll` is set
- [ ] `internal/component/bgp/reactor/peer_settings_apply.go` - `exportChanged` is `reflect.DeepEqual` over `ExportPolicy`, and its comment says a func member "never compares equal"; `exportPolicyFrom` moves when it reads as changed
- [ ] `internal/component/bgp/reactor/policy_reload.go` - `raiseExportPolicyFrom` runs `joinExportWriters` under a 2-minute context and sets `MaxUint64` when the join fails. The `writeMu.Lock` in `joinExportWriters` is not bound by that context. `reconcileExportFinal` restores retained routes through `RelayReplay` and `RelayStoredRoute`
- [ ] `internal/component/bgp/reactor/forward_pool_barrier.go` - `barrierPeer` queues a sentinel behind the peer's overflow
- [ ] `internal/component/bgp/reactor/forward_pool.go` - the worker takes `writeMu` with a blocking lock
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - `holdWrites` shuts both rails out; forwarded UPDATEs stay parked behind the initial sync until `wakeForwardOverflow`, so a sentinel waits with them
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `peerSettingsEqual` compares with `reflect.DeepEqual`; its comment says `PeerSettings` holds no funcs, which a chain with an in-process member contradicts
- [ ] `internal/component/bgp/filterapi/policy.go` - `Policy` holds `chain`, `library`, `templated`. `TemplatedChain` is documented as comparable; `library` is not
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `handleSent` returns at once on `metaKeyReplay`, and interns `raw-attributes` into `pool.RibOut`. `handleRefresh` collects `ribOut`, then BoRR, `sendRoutes`, EoRR
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` - `storeSentEntries` writes `ribOutEntry` for a non-replay write
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `sendRoutes` sends each route with `metaKeyReplay` and its `SentOwnerMessageMeta`
- [ ] `internal/component/bgp/plugins/rib/ribout_entry.go` - `ribOutEntry` holds one `AttrHandle`, the bytes as sent
- [ ] `internal/component/bgp/reactor/refresh_prepend_double_test.go` - the red test. It sends the first write's post-policy bytes back through `sendBatchUpdate`, with no RIB involved

**Behavior to preserve:**
- The chain runs on every refresh, for every member, with the current policy.
- `ribOut` holds what the peer was sent: `show`, withdraw decisions and Section 9.2 suppression read it.
- The opted-member stale run reads the sent body, so `llgr-egress` keeps its verdicts over what the peer holds.
- The forward rail's order: chain, then local-AS prepend.
- A join that fails never bypasses policy: it re-judges more, never less.

**Behavior to change:**
- A refresh and a whole-chain stale run read pre-policy attributes.
- A denial of a route the peer holds withdraws it.
- The watermark moves on a change of definition, never on a rebuild.
- A stale write re-checks the watermark under `writeMu`.
- A reload does not wait for an initial sync.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Adj-RIB-Out store | `ribout_entry.go` `ribOutEntry`, `pool.RibOut` | One interned attribute handle per route, as sent | Extends: a second handle holds the pre-policy bytes, absent when they equal the sent bytes, so a route no member changed costs nothing |
| Sent event | `rib.go` `handleSent`, `raw-attributes` | The engine tells the RIB what went out | Extends: an optional member carries the pre-policy attributes, set only when a member changed them |
| Export gate | `session_write.go` `writeUpdateGated`, `egressRouteFilter` | Runs the chain on an originated body | Extends: it already holds the body before the chain, which is the pre-policy form. Unchanged as a gate, and no replay exemption is added |
| Forward chain | `reactor_api_forward*.go` `forwardUpdateCore` | Chain on the original received wire, then the local-AS prepend | Extends: the sent event takes the wire it fed the chain. The route's replay keeps this order |
| Receive store replay | `policy_reload.go` `reconcileExportFinal`, `RelayReplay`, `RelayStoredRoute` | Restores retained routes from `bgp-adj-rib-in` through the live gate | Does not apply to refresh: it holds received routes only, and is absent unless loaded. A refresh of an originated route would have no source. Its restoration path stays as is |
| Stale rail | `stale_readvertise_chain.go`, `decideStaleReadvertise` | Opted-member or whole-chain run over a rebuilt body | Extends: `judgeAll` reads pre-policy bytes. The opted run keeps the sent bytes |
| Policy watermark | `peer.go` `exportPolicyFrom`, `exportPolicyNewerThanLocked` | A receipt older than the last change re-judges | Extends: it moves on a definition change only, and is re-checked under `writeMu` |
| Writer join | `policy_reload.go` `joinExportWriters`, `forward_pool_barrier.go` | Waits for every old decision to reach the session | Extends: deferred past an initial-sync hold, instead of waited for |
| Withdraw path | `sendStaleReadvertiseUnit`, `staleWithdraw`, `buildBatchWithdrawUpdate` | Sends a bare withdrawal for the same NLRIs | Extends: a denial of a held route takes this branch |
| Config transaction | `docs/architecture/config/transaction-protocol.md` | Each plugin answers for its own roots | Extends the signal: a filter plugin reports a definition change after its apply, through an event |
| Reload settings comparison | `reactor_api.go` `peerSettingsEqual`, `peer_settings_apply.go` | `reflect.DeepEqual`, fail-closed by construction | Extends: `filterapi.Policy` gains a definition comparison, and both sites use it. The rest of `PeerSettings` stays structural |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A peer sends ROUTE-REFRESH, or a stale replay runs: `handleRefresh`, `sendRoutes`, `update-route` with `metaKeyReplay`.
- A config reload that reaches `applyHotSwappableSettings`.
- A filter plugin's own config apply.

### Transformation Path
1. First write: the writer holds the body before the chain. When a member changes it, the sent event carries both forms.
2. `handleSent` interns both, and `ribOutEntry` holds both handles.
3. Refresh: `collectRibOutRoutes` reconstructs the route from the pre-policy handle, or the sent handle when none exists.
4. `sendBatchUpdate` builds, and the ordinary gate runs the chain once. A forward-rail route keeps chain then local-AS prepend.
5. A whole-chain stale run takes the same pre-policy bytes. A denial of a route with a live receipt withdraws it.
6. The stale writer re-reads the watermark under `writeMu`. If it moved after the decision, the unit returns to the decision and runs again.
7. A reload compares definitions. An unchanged definition moves nothing. During an initial sync, the raise is owed and runs when the sync ends.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to RIB | Sent event JSON and structured event, new optional member | No: proven by `TestSentEventCarriesPrePolicyAttributes` |
| RIB to engine | `update-route` text command, unchanged in shape | Yes: `sendRoutes` |
| Filter plugin to engine | New event: filter definition changed | No: proven by `TestFilterDefinitionEventRaisesWatermark` |
| Reactor to filter member | `filter-update`, unchanged | Yes: `runLibraryChain` |

### Changed-Wire Census

**Search evidence:** scope `internal`, `pkg`, `test`. Terms: `raw-attributes`, `GetRawAttributesHex`, `RawAttributes`, `handleSent`, `direction sent`. Control: `handleSent` is the known reader and the sent event writer in `internal/component/bgp/server` the known writer. Run the search again at implementation against the whole tree, the structured event path included, and add a row for each further reader.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Sent event, new optional member | writer | `internal/component/bgp/server` event build, from the reactor's write | writes the sent bytes today | New: present only when a member changed the attributes |
| Sent event, new optional member | reader | `rib.go` `handleSent`, `rib_structured.go` | interns `raw-attributes` | New: reads it when present |
| Sent event, `raw-attributes` | reader | `rib.go` `handleSent`; third-party plugins | unchanged meaning, the bytes as sent | Preserve |
| Filter definition event | writer | Each in-tree filter plugin after `OnConfigApply`; any third-party filter | new | New: named at implementation in the plugin's namespace, declared through `events.Register` |
| Filter definition event | reader | Reactor | new | New |
| BGP UPDATE to a peer | writer | Writer rails | AS_PATH and attributes differ on a refresh | Change: what the chain produces once |

### Integration Points
- `writeUpdateGated` and `forwardUpdateCore` - record the input of the chain for the sent event.
- `reconstructRoute` - prefers the pre-policy handle.
- `applyHotSwappableSettings` - the definition comparison and the deferred raise.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The refresh keeps going through the ordinary gate |
| No unintended coupling (components stay isolated) | Yes | The RIB stores bytes it cannot interpret; the reactor names no plugin |
| No duplicated functionality (extends existing, does not recreate) | Yes | See Existing Mechanisms |
| Zero-copy preserved where applicable (refs, not copies) | Yes | A route no member changed stores no second copy; both forms intern in `pool.RibOut` |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The event is one `events.Register`, and a filter opts in by emitting it |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | No code names prepend or any member; the reactor reads the event type only |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The input to the chain is available where the sent event is built | `writeUpdateGated` holds `body` before `egressRouteFilter`; `forwardUpdateCore` holds the received wire | The event needs another carrier, such as `sentMeta` | `TestSentEventCarriesPrePolicyAttributes` | unvalidated |
| A-2 | A chain's result depends on the bytes, the session facts and the policy, so one run over pre-policy bytes reproduces the first send when nothing changed | `policychain` compiles immutable chains | A stateful external filter answers differently, which RFC 2918 permits | The `.ci` below, with a stateless chain | unvalidated |
| A-3 | The attribute memory cost is small: only routes a member changed hold two forms, and `pool.RibOut` deduplicates | `ribOutEntry` comment, "shared across peers" | A chain changing every route for many peers holds more | `./le perf` allocation figures on a prepend chain, compared on one toolchain | unvalidated |
| A-4 | A receipt from the current session means the peer holds the route | `SentOwnerMessage`, `sentOwnerCurrent` in `sendBatchUpdate` | A withdrawal goes out for a route never sent, which a peer ignores | `TestStaleDenialWithdrawsHeldRoute`, `TestStaleDenialOnNewSessionSendsNothing` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A forward-rail route replayed through the announce rail applies the local-AS prepend in the wrong order | `TestRefreshForwardedRouteKeepsRailOrder` red | The stored form and the rail are recorded together, and the replay keeps the forward order |
| R-2 | An external plugin never emits the definition event | Its routes stay as first judged until the next change | The event is a documented contract in `docs/plugin-development/`; the failure is a stale answer on that plugin's own edit, never a bypass |
| R-3 | A re-decision loop under a burst of reloads | Repeated re-runs of one unit | A reload is rare, and each pass reads the newest policy. A bound with a logged give-up withholds the unit, fail closed |
| R-4 | A deferred raise is lost if the session ends first | Watermark unchanged on a successor | A new session starts with `exportPolicyFrom` set (`reactor_peers.go`) and its receipts are all new |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A peer sees wrong attributes on refresh, or a withdrawal of a route it should keep. Live routes on other paths are not touched |
| How is it reverted? | Single commit revert per phase; no persistent state |
| Who else touches this path? | `plan/release/fixes/spec-bgp-export-policy-reload-readvertise.md` (skeleton) re-runs the chain over advertised routes and needs these pre-policy attributes, so it depends on this spec. `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md` and `plan/release/fixes/spec-bgp-withdraw-only-exported-routes.md` touch `ribOut` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer sends ROUTE-REFRESH to a peer with a prepend member | → | `handleRefresh`, `reconstructRoute`, `sendBatchUpdate`, gate | `test/plugin/route-refresh-export-prepend.ci` |
| A reload that changes nothing, with an in-process member | → | `applyHotSwappableSettings`, definition comparison | `TestNoOpReloadKeepsExportWatermark` |
| A reload during an initial-sync hold | → | `raiseExportPolicyFrom`, deferred raise | `TestReloadDuringInitialSyncDoesNotWait` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A route sent through a prepend member, then any number of refreshes, policy unchanged | Each refresh carries the AS_PATH of the first send | A stateful member may answer differently: RFC 2918 says the policy decides |
| AC-2 | A refresh after the policy changed | The route carries what the new chain produces from pre-policy attributes | None |
| AC-3 | A refresh with a chain that changed nothing | The UPDATE is what was sent, and `ribOut` holds one handle for that route | None |
| AC-4 | A route the forward rail wrote, then a refresh | The chain runs on the received form, then the local-AS prepend, as on the first send | None |
| AC-5 | A whole-chain stale run after a policy change | It reads pre-policy attributes, and applies each member once | None |
| AC-6 | A whole-chain stale run denies a route with a live receipt | The peer receives a withdrawal for that NLRI | With a new session, no receipt, nothing sent: the peer holds nothing. An opted-member `llgr-egress` suppress keeps its meaning |
| AC-7 | A refresh denies a route the peer holds, without RFC 7313 | The peer receives a withdrawal | With RFC 7313, the EoRR removes the route, and the withdrawal is still correct |
| AC-8 | An external filter plugin changes its own definition | Stale routes toward peers whose chain names it re-judge on their next run | None |
| AC-9 | A reload rebuilds a chain with an in-process member and changes nothing | The watermark does not move and no route re-judges | None |
| AC-10 | A reload adds, removes, reorders or re-parameterizes a member, or changes the final | The watermark moves | None |
| AC-11 | A stale unit decided under the old chain reaches `writeMu` after the reload | It is decided again under the new chain, and what is written matches it | None |
| AC-12 | A reload changes policy during an initial-sync hold | The reload returns without waiting for the hold, and the raise takes effect when the sync ends | The hold keeps its own short bound for its own writers |
| AC-13 | A writer join truly fails | The raise is retried and logged, and the watermark ends at a real message id once the join succeeds | `MaxUint64` never stays as the resting state |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A peer operator runs a soft inbound reset against Ze with a prepend export policy | ROUTE-REFRESH -> `handleRefresh` -> gate once -> UPDATE | `test/plugin/route-refresh-export-prepend.ci`, interop `bgp-route-refresh-prepend-frr` |
| 2 | An operator reloads an unchanged config | reload -> definition comparison -> no change | `test/reload/export-policy-noop.ci` |
| 3 | An operator tightens a filter so a route is denied, then the peer refreshes | reload -> refresh -> withdrawal | `test/plugin/route-refresh-export-denied.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRefreshReplaysPrePolicyAttributes` | `internal/component/bgp/plugins/rib/rib_refresh_prepolicy_test.go` | AC-1, AC-3: the route handed back holds pre-policy attributes | |
| `TestSentEventCarriesPrePolicyAttributes` | `internal/component/bgp/reactor/session_write_prepolicy_test.go` | A-1, present only on a change | |
| `TestRefreshForwardedRouteKeepsRailOrder` | same | AC-4 | |
| `TestStaleRailReadsPrePolicyOnWholeRun` | `internal/component/bgp/reactor/stale_export_change_test.go` | AC-5 | |
| `TestStaleDenialWithdrawsHeldRoute` | same | AC-6 | |
| `TestStaleDenialOnNewSessionSendsNothing` | same | AC-6, the survivor | |
| `TestStaleWriteRecheckedUnderWriteMu` | same | AC-11 | |
| `TestNoOpReloadKeepsExportWatermark` | `internal/component/bgp/reactor/policy_reload_test.go` | AC-9 | |
| `TestDefinitionChangeRaisesWatermark` | same | AC-10, one row per change kind | |
| `TestFilterDefinitionEventRaisesWatermark` | same | AC-8 | |
| `TestReloadDuringInitialSyncDoesNotWait` | same | AC-12 | |
| `TestFailedJoinDoesNotRestAtMaxUint64` | same | AC-13 | |
| `TestPolicySameDefinition` | `internal/component/bgp/filterapi/policy_test.go` | Equal across a rebuild, different on each change kind | |

`TestOrdinaryRefreshDoesNotDoublePrepend` (`reactor/refresh_prepend_double_test.go`)
feeds post-policy bytes into the gated writer, so a fix in the RIB cannot turn it
green, and the only change that could is the replay exemption RFC 2918 rules out.
Its obligation moves to `TestRefreshReplaysPrePolicyAttributes` and the `.ci` below,
which drive the real path. Replacing it goes to `ze-test-judge`, which judges
whether the replacement proves at least as much.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Sent receipt against watermark | message ids | `message == exportPolicyFrom` is new | `message == exportPolicyFrom-1` is old | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `route-refresh-export-prepend` | `test/plugin/route-refresh-export-prepend.ci` | Two refreshes, each with the first AS_PATH, hex asserted | |
| `route-refresh-export-denied` | `test/plugin/route-refresh-export-denied.ci` | A denied route is withdrawn on refresh | |
| `export-policy-noop` | `test/reload/export-policy-noop.ci` | A no-op reload changes nothing on the wire | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-route-refresh-prepend-frr` | `test/interop/scenarios/` | FRR | FRR's soft inbound reset ends with the single-prepend path in its table. Read the deciding function in FRR's `bgpd` for `clear bgp ... soft in` before asserting | |

## Files to Modify
- `internal/component/bgp/plugins/rib/ribout_entry.go` - the pre-policy handle, its release, `reconstructRoute`
- `internal/component/bgp/plugins/rib/rib.go` - `handleSent`
- `internal/component/bgp/plugins/rib/rib_structured.go` - `storeSentEntries`, the structured path
- `internal/component/bgp/reactor/session_write.go` - record the chain's input for the sent event
- `internal/component/bgp/reactor/reactor_api_batch.go` - denial withdraws, re-check under `writeMu`
- `internal/component/bgp/reactor/stale_readvertise_chain.go` - `judgeAll` input
- `internal/component/bgp/reactor/peer_settings_apply.go` - definition comparison
- `internal/component/bgp/reactor/reactor_api.go` - `peerSettingsEqual`, its false comment
- `internal/component/bgp/reactor/policy_reload.go` - deferred raise, no resting `MaxUint64`
- `internal/component/bgp/reactor/peer_initial_sync.go` - run an owed raise when the sync ends
- `internal/component/bgp/filterapi/policy.go` - `Policy` definition comparison
- `internal/component/bgp/reactor/refresh_prepend_double_test.go` - replaced, under `ze-test-judge`
- `docs/architecture/core-design.md` - the stale rail and reconciliation text
- `docs/architecture/api/process-protocol.md` - the sent event member and the definition event
- `docs/plugin-development/handlers.md` - what a filter plugin owes after changing a definition
- `docs/architecture/plugin/rib-storage-design.md` - two forms per route

## Files to Create
- `internal/component/bgp/plugins/rib/rib_refresh_prepolicy_test.go` - RIB half
- `internal/component/bgp/reactor/session_write_prepolicy_test.go` - writer half
- `test/plugin/route-refresh-export-prepend.ci` - end to end
- `test/plugin/route-refresh-export-denied.ci` - end to end
- `test/reload/export-policy-noop.ci` - reload path

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config |
| YANG validation constraints | No | No leaf |
| YANG custom validators | No | No leaf |
| CLI commands/flags | No | None |
| CLI grammar (keyword before value) | No | None |
| Editor autocomplete | No | None |
| Functional test for new RPC/API | Yes | The `.ci` files above |
| Pipe completeness | No | None |
| Env var registration | No | None |
| Doctor check for runtime dependencies | No | None |
| Prometheus counters/metrics | No | The existing refused counter stays |
| BGP family surface (new SAFI / capability / attribute) | No | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Fix |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md` |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/config-reload.md`: a reload that changes no definition re-judges nothing |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/handlers.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2918.md` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, `docs/architecture/plugin/rib-storage-design.md` |
| 13 | Route metadata keys added/changed? | No | The new member is on the event, not route metadata |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md` if it lists event types |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/pool-architecture.md`: `pool.RibOut` holds a second handle per changed route. Others: `./le spec citation anchors spec plan/release/fixes/spec-bgp-refresh-reapplies-export-policy.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/config-reload.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the sent event member and the RIB's second handle
   - Tests: `TestSentEventCarriesPrePolicyAttributes`, `TestRefreshReplaysPrePolicyAttributes`
   - Files: `session_write.go`, `rib.go`, `rib_structured.go`, `ribout_entry.go`
   - Verify: the RIB holds both forms, and refresh still reads the sent form
2. **Phase: Refresh reads pre-policy** -- D-1
   - Tests: `TestRefreshForwardedRouteKeepsRailOrder`, `test/plugin/route-refresh-export-prepend.ci`
   - Files: `ribout_entry.go`, `refresh_prepend_double_test.go` under `ze-test-judge`
   - Verify: revert, rebuild, see red, restore, see green
3. **Phase: Stale rail** -- `judgeAll` input, D-2, D-5
   - Tests: `TestStaleRailReadsPrePolicyOnWholeRun`, `TestStaleDenialWithdrawsHeldRoute`, `TestStaleDenialOnNewSessionSendsNothing`, `TestStaleWriteRecheckedUnderWriteMu`, `test/plugin/route-refresh-export-denied.ci`
   - Files: `reactor_api_batch.go`, `stale_readvertise_chain.go`
   - Verify: tests fail, implement, tests pass
4. **Phase: Watermark truth** -- D-3, D-4
   - Tests: `TestPolicySameDefinition`, `TestNoOpReloadKeepsExportWatermark`, `TestDefinitionChangeRaisesWatermark`, `TestFilterDefinitionEventRaisesWatermark`, `test/reload/export-policy-noop.ci`
   - Files: `filterapi/policy.go`, `peer_settings_apply.go`, `reactor_api.go`, the in-tree filter plugins
   - Verify: tests fail, implement, tests pass
5. **Phase: Reload does not wait** -- D-6
   - Tests: `TestReloadDuringInitialSyncDoesNotWait`, `TestFailedJoinDoesNotRestAtMaxUint64`
   - Files: `policy_reload.go`, `peer_initial_sync.go`
   - Verify: tests fail, implement, tests pass
6. **Phase: Interop and pages** -- `bgp-route-refresh-prepend-frr`, the pages above

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No replay exemption in the writer: the chain runs on each refresh |
| Correctness | `release` covers the second handle on every path that drops an entry |
| Naming | The event member is kebab-case |
| Data flow | The reactor keeps no pre-policy store; the RIB is the only one |
| Rule: one chain | Nothing names prepend, or any member (`ai/META-DECISIONS.md`) |
| Rule: stale comments | The comments on `peerSettingsEqual`, `exportChanged`, `raiseExportPolicyFrom` and `TemplatedChain` say what is now true |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Refresh path | `./le test bgp plugin route-refresh-export-prepend` |
| Reload path | `./le test functional reload` |
| Scoped unit tests | `./le job run label refresh-unit command go test -tags ze_bgp ./internal/component/bgp/reactor/ ./internal/component/bgp/plugins/rib/ ./internal/component/bgp/filterapi/` |
| Peer proof | `./le test integration interop` with scenario `bgp-route-refresh-prepend-frr` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open | A failed comparison or a failed join re-judges more, never less |
| Resource exhaustion | The second handle is interned and refcounted; a leak shows in `pool.RibOut` |
| Input validation | The event names a filter by its registered reference; an unknown one raises nothing |

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

- The brief's reading holds at the producers: `sendBatchUpdate` passes `writeUpdate`, `handleSent` drops replays so nothing accumulates, and RFC 2918 Section 4 rules out skipping the chain.
- One consequence the brief did not state: the named red test cannot go green under the RIB fix, because it feeds post-policy bytes to the gated writer.
- D-2 is narrower on a refresh with RFC 7313, where the EoRR already removes a route not re-sent. It stays a defect without RFC 7313, and on every non-refresh path. The ordinary writer has the same shape: its `suppress` returns with no withdrawal, so AC-7 covers that sibling.
- D-4 reaches `peerSettingsEqual` too. Its comment claims `PeerSettings` holds no funcs, so a no-op reload reads as a change there first.
- `joinExportWriters` bounds the barrier by its context and does not bound `writeMu.Lock`, so the 2-minute bound does not cover a wait on that lock.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `non-idempotent-append-duplicates-the-record.md` | 2026-11-06 | refresh re-applies export policy over post-policy `ribOut` bytes |
| `policy-change-never-re-evaluates-what-it-admitted.md` | 2026-11-06 | stale rail denial suppresses and leaves the peer's route |
| `memo-suppresses-a-change-it-cannot-see.md` | 2026-11-06 | external plugin definition change invisible to `exportChanged` |
| `state-event-emitted-without-comparing-state.md` | 2026-11-06 | rebuilt chain with a func member never compares equal |
| `late-write-lands-on-the-successor.md` | 2026-11-06 | stale decision before `writeMu` writes under the old chain |
| `grace-bound-hides-a-missing-signal.md` | 2026-11-06 | export policy reload stalls at the barrier, then `MaxUint64` |
| `state-event-emitted-without-comparing-state.md` | 2026-11-06 | `peerSettingsEqual` runs `DeepEqual` over the policies and reads `refused` |
| `state-event-emitted-without-comparing-state.md` | 2026-11-06 | raw injection carries no generation and raises the export watermark |
| `policy-change-never-re-evaluates-what-it-admitted.md` | 2026-11-06 | a filter's Withdraw on a no-owner stale batch removes whatever the key holds |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-06 | import member strips LLGR_STALE; forward-rail LOCAL_PREF and NO_EXPORT undo |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The RIB stores pre-policy attributes | A replay exemption in the writer; a receive-store replay | The exemption breaks RFC 2918 Section 4. The receive store holds no originated route and is optional |
| Second handle only when different | Always two handles | Most chains change nothing on most routes, and `pool.RibOut` exists to avoid the copy |
| Definition comparison on `filterapi.Policy` | Removing the func from the chain; hashing the compiled chain | The compiled chain holds readers by design. The definition is the config-derived part, which is what changed or did not |
| An event for a filter's own change | A new RPC; the engine reading each plugin's config | The engine never names a feature, and an external plugin can emit an event on any transport |
| Re-check under `writeMu` | Decide under `writeMu` | An external member's RPC under the session's write lock stalls keepalives |
| Deferred raise past the hold | Fire and forget a goroutine per reload | The initial sync already has one end point, and no goroutine is added per event |

## Known Limitations
- A stateful external filter can answer a refresh differently from the first send. RFC 2918 makes that its right.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. RFC 2918 Section 4 stays above the refresh path; the new denial branch quotes it and RFC 7313 Section 4 where relevant.

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
- [ ] The targeted tests pass, run in the main checkout: `./le test bgp plugin route-refresh-export-prepend`, `./le test functional reload`, the scoped package tests above. GitHub CI verifies the pushed commit
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
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script
