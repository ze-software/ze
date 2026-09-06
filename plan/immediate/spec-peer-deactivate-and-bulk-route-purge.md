# Spec: peer-deactivate-and-bulk-route-purge

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.** `ze config deactivate router.conf bgp peer peer1`
is an advertised example of the deactivate verb. Its help text says the
deactivated node "round-trips through save/load and is skipped at apply time"
(`runDeactivateLike`, `internal/component/config/cli/cmd_deactivate.go`). An
operator reads that as a pause. The peer keeps its configuration and its
identity, the session stops, the routes go, and `activate` brings it back as the
same peer. The owner asked for exactly that, plus one bulk message meaning "drop
every route from this peer", so a peer holding a full table does not emit one bus
event per prefix on teardown.

**What Ze does instead.** A deactivated peer is not paused. It is erased.
`parseTreeWithYANG` (`internal/component/config/loader.go`) calls `PruneInactive`
before the tree becomes the reload map, so the deactivated peer is absent from
the candidate root that `decomposeBGPOperations`
(`internal/component/bgp/plugin/operation.go`) diffs. The diff emits
`OperationRemovePeer`, `applyConfigOperation`
(`internal/component/bgp/reactor/operation.go`) routes it to
`removePeerForOperation`, and `Reactor.RemovePeer`
(`internal/component/bgp/reactor/reactor_peers.go`) runs. Deactivate and remove
are one code path, so nothing distinguishes a pause from a deletion. Three
consequences follow. The peer leaves every operational surface rather than
showing as administratively down. `doRemovePeer` calls
`fwdPathIDs.releaseSource`, whose own comment reserves that release for a peer
whose paths are gone for good, so a paused peer loses the RFC 7911 Path
Identifiers a reconnecting peer keeps. And `Peer.Stop`
(`internal/component/bgp/reactor/peer.go`) only cancels the peer context, so the
socket closes with no NOTIFICATION. `Peer.cleanup`
(`internal/component/bgp/reactor/peer_run.go`) records that the session guard
which once sent one was always false, and `shutdownNotify` has one non-test
caller, `Reactor.stop` (`internal/component/bgp/reactor/reactor.go`), which runs
at daemon shutdown alone. RFC 4271 Section 8.2.2, Established State, ManualStop
requires the local system to send "the NOTIFICATION message with a Cease" and to
delete "all routes associated with this connection". Ze does the second and not
the first.

**What the purge already does.** The route half of the requirement is largely
built, so this spec must not ask for it again. `RIBManager.handleState` and
`RIBManager.handleStructuredState` (`internal/component/bgp/plugins/rib/rib.go`)
release the departing peer's Adj-RIB-In and call `purgeBestPrevForPeer`, which
deletes the peer's best-path records, calls `locRIB.Remove` for each, and returns
per-family withdraw batches
(`internal/component/bgp/plugins/rib/rib_bestchange.go`).
`AdjRIBInManager.handleState` (`internal/component/bgp/plugins/adj_rib_in/rib.go`)
deletes the peer's `ribIn` entry on the same event. `routeServer.handleStateDown`
(`internal/component/bgp/plugins/rs/server_handlers.go`) sends the wire
withdrawals to the other clients, batched at `withdrawalBatchSize` NLRIs per
command.

**Why the bulk message is not a batching change.** `emitPurgedWithdraws` calls
`publishBestChanges` once per family, so the bus already carries one event per
family rather than one per prefix. What stays proportional to the table size is
the payload: the `Changes` slice holds one `BestChangeEntry` per route, and the
bus marshals that slice to JSON for every external subscriber. A message meaning
"drop every route from this peer" cannot be added to that contract alone.
`BestChangeBatch` and `BestChangeEntry`
(`internal/core/bgp/ribevents/ribevents.go`) carry no peer identity, and their
action vocabulary is `BestChangeAdd`, `BestChangeUpdate` and
`BestChangeWithdraw`. No subscriber indexes routes by contributing peer: neither
`internal/component/sysrib/sysrib.go` nor
`internal/plugins/fib/kernel/fibkernel.go` names a peer anywhere. So the bulk
message needs a per-peer attribution on the way in before it can replace the
per-prefix list on the way out. That is the design question this spec exists to
answer, and it is why the four requirements are one work item rather than four.

**What `withdraw-all` is not.** `ze-bgp:withdraw-all` is registered and answers
`send bgp <selector> withdraw all` (`handleWithdrawAll`,
`internal/component/bgp/plugins/cmd/announce/announce.go`). It reaches
`Registry.withdrawAll`
(`internal/component/bgp/plugins/cmd/announce/registry.go`), which withdraws the
announcements this daemon made through the announce registry. That is the
announce direction. The owner asked for the receive direction, and no command or
event addresses it today.

**What closing it means.** The implementer faces one choice on requirement 1 and
one on requirement 4. On the pause: give a peer an explicit administrative state
the config carries to the reactor, or keep routing deactivate through removal and
refuse the verb on a peer node the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses the `vrf` leaf. Neither is
obviously right and the owner asked for the feature, so the refusal is a
placeholder rather than an answer. On the bulk message: carry the contributing
peer on each route so a peer-scoped purge is actionable, or keep the per-prefix
list and accept its cost. The first is more work and is the only one that
satisfies what the owner asked for.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/behavior/peer-lifecycle.md` - the outer per-peer run loop, `Stop`, `Teardown` and `cleanup`
  → Constraint: the page's `cleanup()` step 3 says the session close sends a Cease NOTIFICATION. `Peer.cleanup` no longer holds that call, so the page is stale and this spec's change must correct it.
- [ ] `docs/architecture/config/apply-ordering.md` - the operation handlers `operation.go` declares in its `// Design:` header
  → Decision: [what the apply order constrains for a new pause operation]
- [ ] `docs/architecture/rib-transition.md` - [why relevant]
  → Constraint: [specific rule from the doc that applies here]
- [ ] `docs/architecture/api/process-protocol.md` - the event contract external plugin processes decode
  → Constraint: [what a new bulk event owes an external subscriber]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Section 8.2.2, Established State, ManualStop
  → Constraint: "sends the NOTIFICATION message with a Cease" and "deletes all routes associated with this connection" are both listed under the operator-initiated stop.
- [ ] `rfc/short/rfc4486.md` - the Cease subcodes
  → Constraint: [which subcode an administrative pause carries]
- [ ] `rfc/short/rfc8203.md` - the shutdown communication message
  → Constraint: [what an operator-supplied reason owes the wire]
- [ ] `rfc/short/rfc7911.md` - Path Identifiers
  → Constraint: [what a pause owes the identifiers a reconnecting peer keeps]

**Key insights:** (minimal context to resume after compaction)
- Deactivate reaches the BGP layer as `OperationRemovePeer` and nothing else. A pause needs its own operation type or an explicit administrative-state leaf.
- The bus already batches per family. The cost that remains is the payload, and no consumer can act on a peer-scoped purge until routes carry a peer.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/loader.go` - `parseTreeWithYANG` prunes inactive nodes before the tree reaches any consumer
- [ ] `internal/component/config/prune.go` - `PruneInactive` removes inactive containers and list entries in place
- [ ] `internal/component/bgp/config/peers.go` - `peersAndDynamicGroups` prunes again before `ResolveBGPTree`
- [ ] `internal/component/bgp/plugin/operation.go` - `decomposeBGPOperations` emits `OperationRemovePeer` for a peer present in the active root and absent from the candidate root
- [ ] `internal/component/bgp/reactor/operation.go` - `applyConfigOperation` routes `OperationRemovePeer` to `removePeerForOperation`
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - `RemovePeer` and `doRemovePeer` stop the peer, release the router-id claim and the Path Identifiers, then emit one peer-down carrying `rpc.ReasonPeerRemoved`
- [ ] `internal/component/bgp/reactor/peer.go` - `Peer.Stop` cancels the context only. `shutdownNotify` sends the Cease and runs from `Reactor.stop` alone
- [ ] `internal/component/bgp/reactor/peer_run.go` - `Peer.cleanup` sends no NOTIFICATION
- [ ] `internal/component/bgp/server/events.go` - `onPeerStateChange` fans the state event to peer-scoped processes in dependency order
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `handleState` and `handleStructuredState` release the Adj-RIB-In on a down transition, unless GR retention holds the peer
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `purgeBestPrevForPeer` collects the withdrawals, `emitPurgedWithdraws` and `publishBestChanges` emit one event per family
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - `AdjRIBInManager.handleState` deletes the peer's `ribIn` entry
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` - `handleStateDown` and `sendBatchedWithdrawals` put the withdrawals on the wire for the other clients
- [ ] `internal/core/bgp/ribevents/ribevents.go` - `BestChangeBatch` and `BestChangeEntry` carry no peer identity and no purge action
- [ ] `internal/component/bgp/plugins/cmd/announce/registry.go` - `Registry.withdrawAll` acts on announcements Ze made, not on routes Ze received
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - `handleTeardown` sends a Cease with an operator subcode, and the run loop reconnects afterwards
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - no per-peer administrative shutdown leaf exists

**Behavior to preserve:** (unless the user explicitly said to change it)
- Removing a peer from the configuration keeps meaning removal. The peer leaves every operational surface and its Path Identifiers are released.
- Graceful Restart retention keeps its precedence. The retained-peer branch suppresses the Adj-RIB-In release on a down transition.
- `publishBestChanges` keeps its per-family event shape and its JSON contract with external plugin processes.
- `ze-bgp:withdraw-all` keeps acting on the announce registry.

**Behavior to change:** (only what the user asked for)
- A deactivated peer becomes a distinct administrative state rather than an absent peer.
- An operator-initiated stop sends a Cease NOTIFICATION before it drops the connection.
- A peer's route purge gets a bulk form that does not carry one entry per prefix.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze config deactivate <file> bgp peer <name>` followed by a commit, and the `activate` inverse.
- Format at entry: an inactive marker on the peer's list entry in the config tree.

### Transformation Path
1. `runDeactivateLike` (`internal/component/config/cli/cmd_deactivate.go`) sets the marker and writes the file.
2. `parseTreeWithYANG` (`internal/component/config/loader.go`) parses the file and calls `PruneInactive`. This is where the marker is lost today.
3. `Server.reloadConfig` (`internal/component/plugin/server/reload.go`) diffs the running tree against the candidate tree.
4. `decomposeBGPOperations` (`internal/component/bgp/plugin/operation.go`) turns the peer's absence into `OperationRemovePeer`.
5. `applyConfigOperation` (`internal/component/bgp/reactor/operation.go`) applies it through `Reactor.RemovePeer`.
6. `Reactor.RemovePeer` (`internal/component/bgp/reactor/reactor_peers.go`) stops the peer and emits one peer-down carrying `rpc.ReasonPeerRemoved`.
7. `onPeerStateChange` (`internal/component/bgp/server/events.go`) delivers that event to each peer-scoped process in reverse dependency order.
8. `RIBManager.handleStructuredState` (`internal/component/bgp/plugins/rib/rib.go`) releases the Adj-RIB-In and calls `purgeBestPrevForPeer`.
9. `emitPurgedWithdraws` and `publishBestChanges` (`internal/component/bgp/plugins/rib/rib_bestchange.go`) put the withdrawals on the event bus. This is the seam a bulk purge event is published at.
10. The `BestChange` subscribers read them: `internal/component/sysrib/sysrib.go`, `internal/component/bgp/plugins/bmp/bmp_locrib.go` and `internal/plugins/flowexport/enrichbgp.go`. This is the seam a bulk purge event is consumed at.
11. `routeServer.handleStateDown` (`internal/component/bgp/plugins/rs/server_handlers.go`) puts the wire withdrawals to the other clients out, batched.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config editor ↔ config loader | the inactive marker, dropped by `PruneInactive` | No |
| Config transaction ↔ BGP reactor | an `rpc.ConfigOperation` of a declared type | No |
| Reactor ↔ plugin processes | the peer state event, as text or as an `rpc.StructuredEvent` | No |
| BGP RIB ↔ every Loc-RIB consumer | the `BestChange` handle, in process as a batch pointer and out of process as its JSON | No |
| Ze ↔ BGP peer | UPDATE withdrawals and the Cease NOTIFICATION | No |

### Integration Points
- `pkg/plugin/rpc/types.go` - `OperationRemovePeer` and its siblings. A pause needs a new declared operation type, or a peer setting the existing modify path carries.
- `internal/core/bgp/ribevents/ribevents.go` - the bulk purge event and any per-peer attribution live here, because the consumer side stays compiled when the BGP engine is compiled out.
- `pkg/plugin/rpc/enums.go` - `ReasonPeerRemoved`. A pause needs its own reason, because `internal/component/bgp/plugins/gr/gr.go` already branches on this one.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

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
| A-1 | [what this design assumes] | [where the assumption comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what goes wrong] | [how we notice it] | [what we do about it] |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [live sessions dropped / routes mis-encoded / config rejected / nothing user-visible] |
| How is it reverted? | [single commit revert / needs config migration / not revertible once peers see it] |
| Who else touches this path? | [other plugins, components, or specs working the same files] |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An operator deactivates a configured peer and commits | The peer stays visible on the operational surfaces, reported in an administratively down state, with its configured address, ASN and name unchanged |
| AC-2 | An operator activates a peer it had deactivated, and commits | The peer starts a session again under the same identity, and the Path Identifiers it assigned before the pause are unchanged |
| AC-3 | An operator deactivates a peer that holds an established session | Ze sends a Cease NOTIFICATION before it closes the TCP connection, and the remote daemon records an administrative shutdown rather than a reset connection |
| AC-4 | An operator removes a configured peer that holds an established session, and commits | Ze sends a Cease NOTIFICATION before it closes the TCP connection, and the peer leaves the operational surfaces |
| AC-5 | A peer holding a full table is deactivated or removed | The Adj-RIB-In holds no route from that peer afterwards, and the Loc-RIB holds no best path it contributed |
| AC-6 | The same teardown, observed by another BGP peer that received those routes | The other peer receives a withdrawal for every prefix whose best path the departing peer contributed, and for no other prefix |
| AC-7 | The same teardown, observed on the event bus | The number of bus messages does not grow with the number of routes the peer held. A consumer that installed those routes removes all of them |
| AC-8 | A consumer receives the bulk purge message for a peer it holds no route from | It removes nothing and it reports no error |
| AC-9 | A peer under Graceful Restart retention is deactivated | The retained routes follow the existing Graceful Restart rules, and no bulk purge is emitted for them |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [for example "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->

This spec changes what reaches the wire twice. It adds a Cease NOTIFICATION to an
operator-initiated stop, and it removes routes another daemon holds. Both are
peer-observable, so this spec owes an interop scenario. A unit test over the
purge cannot show that the other daemon dropped the prefixes, so the scenario is
what proves AC-3, AC-4 and AC-6 against an implementation Ze did not write. The
scenario directory is named and carries no numeric prefix
(`ai/rules/interop-and-goal-validation.md`).

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `peer-deactivate-withdraws` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP] | The deactivated peer receives a Cease NOTIFICATION, and a third daemon loses every route that peer contributed and no other | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/...` - [feature changes]

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves. Dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED, do not answer from memory: `./le spec citation anchors spec plan/<this-spec>.md` lists them. A doc DECLARED by a changed file's `// Design:` header BLOCKS until named here; a doc that only `<!-- source: -->` mentions it is advisory. Naming it as unaffected, with the reason, satisfies the check |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: the entry point exists and is reachable. The wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [feature-specific, for example "merge order correct", "error messages name the offending value"] |
| Naming | [feature-specific, for example "JSON keys kebab-case", "YANG leaf matches env var leaf"] |
| Data flow | [feature-specific, for example "resolution in X only, reactor unaware of Y"] |
| Rule: [relevant rule] | [what to check] |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

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

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
