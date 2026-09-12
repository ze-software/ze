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

**Two peer lifecycles, both wanted.** The owner wants a peer to be creatable and
deletable without going into the configuration, as well as through it. A
CONFIGURED peer is created by `set bgp peer ...` and a commit. It lives in the
file and in the reactor and it survives a restart. An EPHEMERAL peer is created
at runtime, is never written to the file, and is gone on restart.
`Reactor.AddDynamicPeer` (`internal/component/bgp/reactor/reactor_peers.go`) is
the seam. Uncommitted work in this checkout already builds `create bgp peer` on
it (`internal/component/bgp/plugins/cmd/peer/create.go`) together with a fix to
that function, which was unreachable at HEAD. That work is what exists, and this
spec does not replace it.

**The reactor cannot tell the two kinds apart.** No peer carries an origin
marker. `PeerSettings.IsDynamic`
(`internal/component/bgp/reactor/peer_settings.go`) looks like one and is not:
its only two setters are `ParseDynamicGroupTemplate`
(`internal/component/bgp/reactor/config.go`) and `buildDynamicPeerSettings`
(`internal/component/bgp/reactor/reactor_dynamic.go`), so it marks a peer built
from a listen-range GROUP template. `AddDynamicPeer` sets no marker at all: it
calls `parsePeerFromTree` and then `AddPeer`, so a runtime-created peer is
indistinguishable from a configured one everywhere downstream. The name of the
function says dynamic and the peer it builds is not. That marker is the first
thing this spec owes, because every question below is decided by it.

**What the missing marker already costs.** `reconcilePeersJournaled`
(`internal/component/bgp/reactor/reactor_api.go`) keeps a peer the configuration
does not name when `IsDynamic` is set, and removes any other peer the
configuration does not name. An ephemeral peer is not `IsDynamic`, so it takes
the removal branch. Both branches of `Server.reloadConfig`
(`internal/component/plugin/server/reload.go`) call `ApplyConfigDiff`, which
reaches that reconcile (`internal/component/bgp/reactor/reactor.go`). So an
ephemeral peer does not survive the next commit of any kind, not only a restart.

**`delete bgp peer` diverges the reactor from the file.** `handleBgpPeerRemove`
(`internal/component/bgp/plugins/cmd/peer/peer.go`) resolves a selector and calls
`Reactor.RemovePeer`, and it touches no configuration. On an ephemeral peer that
is correct and complete. On a configured peer the peer returns at the next
restart, and the transaction's tree still holds it, so the next commit's diff
sees no change and does not re-add it. The spec must say what happens instead.
Refusing the verb on a configured peer and routing it to deactivate are both
candidates. The owner has not chosen and this spec must not choose for him.

**Four words for two ideas.** `deactivate` and `activate` are config editor
verbs, so they have nothing to act on for an ephemeral peer, which owns no config
node. `pause` and `resume` are already served, and they mean something else
entirely: `handleBgpPeerPause`
(`internal/component/bgp/plugins/cmd/peer/peer.go`) reaches `Reactor.PausePeer`
(`internal/component/bgp/reactor/reactor_connection.go`), which calls
`peer.pauseReading()`. That is read-loop backpressure for a slow plugin. The
session stays established, the TCP connection stays open, and no route is purged.
So the operator meets four words today, and none of them names an administrative
pause. The spec must relate the pairs rather than add a fifth word.

**The missing Cease is a conformance gap, not a nicety.** RFC 4271 Section 8.2.2,
Established State, ManualStop lists "sends the NOTIFICATION message with a Cease"
first among the actions an operator-initiated stop takes. Ze omits it on every
route that reaches `Peer.Stop`, which is every removal and every deactivation.
That framing decides the priority: this is an unmet MUST on a path an operator
reaches with one command, not a missing convenience. `plan/journal/comment-describes-superseded-behaviour.md`
carries the row for the page that says otherwise.

**What closing it means.** The implementer faces three choices, and must not
settle any of them alone. On the pause: give a peer an explicit administrative
state the config carries to the reactor, or keep routing deactivate through
removal and refuse the verb on a peer node the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses the `vrf` leaf. The owner
asked for the feature, so the refusal is a placeholder rather than an answer. On
`delete bgp peer` against a configured peer: refuse it, or route it to
deactivate. On the bulk message: carry the contributing peer on each route so a
peer-scoped purge is actionable, or keep the per-prefix list and accept its cost.
The first is more work and is the only one that satisfies what the owner asked
for.

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
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - `handleTeardown` sends a Cease with an operator subcode and the run loop reconnects afterwards. `handleBgpPeerRemove` calls `Reactor.RemovePeer` and touches no configuration. `handleBgpPeerPause` and `handleBgpPeerResume` reach the read-loop flow control
- [ ] `internal/component/bgp/reactor/reactor_connection.go` - `Reactor.PausePeer` calls `peer.pauseReading()`. The session and the TCP connection both survive, and no route is purged
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - `AddDynamicPeer` builds a peer from a caller-supplied tree and starts it through `AddPeer`, setting no origin marker
- [ ] `internal/component/bgp/reactor/peer_settings.go` - `PeerSettings.IsDynamic` marks a peer built from a listen-range group template, not a runtime-created one
- [ ] `internal/component/bgp/reactor/config.go` - `ParseDynamicGroupTemplate` is one of the two `IsDynamic` setters
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` - `buildDynamicPeerSettings` is the other
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `reconcilePeersJournaled` keeps an `IsDynamic` peer the configuration does not name, and removes every other peer it does not name
- [ ] `internal/component/plugin/server/reload.go` - both branches of `Server.reloadConfig` call `ApplyConfigDiff`, so every commit reaches that reconcile
- [ ] `internal/component/bgp/plugins/cmd/peer/create.go` - uncommitted in this checkout. `handleBgpPeerAdd` answers `create bgp peer <address> asn <asn> ...` and calls `AddDynamicPeer`. This is the ephemeral create path as it exists today
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - no per-peer administrative shutdown leaf exists
- [ ] `internal/test/fixture/plugin_fixture_13_rest.go` - the fixture behind `test/plugin/rest-peer-set-delete-lifecycle.ci`. It asserts on `show bgp peer * detail` and queries no RIB

**What the existing tests do NOT cover.** No test today asserts that a peer's
routes leave the Adj-RIB-In and the RIB when the peer is removed or deactivated,
and none counts the purge events a teardown emits. The two suites closest to the
behavior stop short of it, so neither may be listed as coverage.
`test/plugin/rest-peer-set-delete-lifecycle.ci` proves that a commit builds and
tears down the peer object, through presence and absence in
`show bgp peer * detail`. `test/plugin/api-peer-remove.ci` proves that
`delete bgp peer` removes the peer from the peer list.
`test/plugin/bgp-rs-ipv4-withdrawal.ci` drives an explicit protocol withdrawal
for one prefix and never removes a peer. Every AC below that names a RIB or an
event count therefore needs a test that does not exist yet.

**Behavior to preserve:** (unless the user explicitly said to change it)
- Removing a peer from the configuration keeps meaning removal. The peer leaves every operational surface and its Path Identifiers are released.
- Graceful Restart retention keeps its precedence. The retained-peer branch suppresses the Adj-RIB-In release on a down transition.
- `publishBestChanges` keeps its per-family event shape and its JSON contract with external plugin processes.
- `ze-bgp:withdraw-all` keeps acting on the announce registry.

- `create bgp peer` keeps building a peer the configuration does not hold, and `delete bgp peer` keeps deleting an ephemeral peer outright.
- Read-loop flow control keeps its own verbs and its own meaning, whatever the administrative pause is called.

**Behavior to change:** (only what the user asked for)
- A deactivated peer becomes a distinct administrative state rather than an absent peer.
- An operator-initiated stop sends a Cease NOTIFICATION before it drops the connection.
- A peer's route purge gets a bulk form that does not carry one entry per prefix.
- A peer records its origin, so a runtime command can tell a configured peer from an ephemeral one.
- `delete bgp peer` stops diverging the reactor from the file when the peer is a configured one.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Two lifecycles reach the same reactor, so there are two entry points and one
shared tail. Stages 6 onward are common to both.

- Configured: `set bgp peer ...`, `ze config deactivate <file> bgp peer <name>` and the `activate` inverse, each followed by a commit. Format at entry: an inactive marker on the peer's list entry in the config tree.
- Ephemeral: `create bgp peer <address> asn <asn> ...` and `delete bgp peer <selector>`, applied at runtime. Format at entry: command tokens the dispatcher types from `ze-peer-cmd.yang`.

### Transformation Path

The ephemeral path is two stages long. `handleBgpPeerAdd`
(`internal/component/bgp/plugins/cmd/peer/create.go`) builds a tree from the
command tokens and calls `AddDynamicPeer`
(`internal/component/bgp/reactor/reactor_peers.go`), which parses it and calls
`AddPeer`. `handleBgpPeerRemove` (`internal/component/bgp/plugins/cmd/peer/peer.go`)
calls `Reactor.RemovePeer` and joins the configured path at stage 6.

The configured path is:

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
- `internal/core/bgp/configop/configop.go` - `OperationRemovePeer` and its siblings. A pause needs a new declared operation label, or a peer setting the existing modify path carries. The labels moved out of `pkg/plugin/rpc/types.go` on 2026-09-08 (`6ffcdaf25`), because the shared ABI carrying a per-root label list is a central enumeration; the ABI now carries a verb and a resource kind, and each root owns its own labels.
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
Every route criterion below is stated over the RIBs and not over the peer list.
A peer row appearing or disappearing in `show bgp peer` satisfies none of them.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An operator deactivates a configured peer and commits | The peer stays visible on the operational surfaces, reported in an administratively down state, with its configured address, ASN and name unchanged |
| AC-2 | An operator activates a peer it had deactivated, and commits | The peer starts a session again under the same identity, and the Path Identifiers it assigned before the pause are unchanged |
| AC-3 | An operator deactivates a peer that holds an established session | Ze sends a Cease NOTIFICATION before it closes the TCP connection, and the remote daemon records an administrative shutdown rather than a reset connection |
| AC-4 | An operator removes a configured peer that holds an established session, and commits | Ze sends a Cease NOTIFICATION before it closes the TCP connection, and the peer leaves the operational surfaces |
| AC-5 | A peer holding a full table is deactivated or removed | The Adj-RIB-In holds no route from that peer afterwards, and the Loc-RIB holds no best path it contributed |
| AC-6 | The same teardown, observed by another BGP peer that received those routes | The other peer receives a withdrawal for every prefix whose best path the departing peer contributed, and for no other prefix |
| AC-7 | A peer carrying many prefixes is deactivated or removed, and the event bus is counted | The purge produces one message rather than one per prefix, and the count does not grow with the number of prefixes the peer held |
| AC-8 | A consumer receives the bulk purge message for a peer it holds no route from | It removes nothing and it reports no error |
| AC-9 | A peer under Graceful Restart retention is deactivated | The retained routes follow the existing Graceful Restart rules, and no bulk purge is emitted for them |
| AC-10 | An operator creates a configured peer and commits, and the peer sends routes | The session reaches Established, and every prefix it announced is readable in the Adj-RIB-In and in the RIB |
| AC-11 | An operator creates an ephemeral peer with `create bgp peer`, and the peer sends routes | The session reaches Established, and every prefix it announced is readable in the Adj-RIB-In and in the RIB, on the same query an operator uses for a configured peer |
| AC-12 | An ephemeral peer holding routes is deleted with `delete bgp peer` | The session drops, the Adj-RIB-In holds no route from it, the RIB holds no best path it contributed, and the withdrawals reach every downstream consumer |
| AC-13 | An operator asks the daemon which peers it holds | Each peer reports whether it is configured or ephemeral, and the answer survives the peer going down and coming back |
| AC-14 | An operator runs `delete bgp peer` against a CONFIGURED peer | Ze does not leave the reactor and the configuration disagreeing. The chosen answer is uniform, so the same command against the same kind of peer always does the same thing |
| AC-15 | An ephemeral peer exists, and an operator commits a configuration change that does not name it | The ephemeral peer's fate is the one the spec states, and it is the same whether the commit reached the transaction path or the direct apply path |
| AC-16 | An operator uses the administrative pause verb and the flow-control pause verb on one peer | Each does its own job, and neither is reachable by the other's name. The flow-control pause leaves the session established and purges no route |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [for example "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

## 🧪 TDD Test Plan

**Every row in this plan is a test to WRITE.** None of them exists today, and the
suites that look adjacent are listed under Current Behavior with what they
actually assert. A row is satisfied only by a test that reads a RIB or counts
events, so an assertion over `show bgp peer` output does not close one.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerRecordsItsOrigin` | `internal/component/bgp/reactor/` | A peer built by `AddDynamicPeer` reports a different origin from one built by the config loader (AC-13) | to write |
| `TestPurgeEmitsOneMessageForManyPrefixes` | `internal/component/bgp/plugins/rib/` | The purge of a peer holding many prefixes emits a count that does not grow with the prefix count (AC-7) | to write |
| `TestBulkPurgeForAnUnknownPeerRemovesNothing` | `internal/component/bgp/plugins/rib/` | A consumer given the purge for a peer it holds no route from removes nothing and reports no error (AC-8) | to write |
| `TestGracefulRestartRetentionSuppressesTheBulkPurge` | `internal/component/bgp/plugins/rib/` | A retained peer emits no bulk purge (AC-9) | to write |
| `TestOperatorStopSendsCeaseBeforeClose` | `internal/component/bgp/reactor/` | The removal path writes a Cease NOTIFICATION before the connection closes (AC-3, AC-4) | to write |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
Each row drives one lifecycle in one direction and asserts on a RIB. The create
rows prove the routes ARRIVE, the teardown rows prove they LEAVE, and the count
row proves the bulk purge is bulk.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `peer-configured-create-routes-reach-rib` | `test/plugin/` | An operator commits a peer, the peer announces prefixes, and the operator reads them back from the Adj-RIB-In and the RIB (AC-10) | to write |
| `peer-ephemeral-create-routes-reach-rib` | `test/plugin/` | An operator runs `create bgp peer`, the peer announces prefixes, and the same queries answer for it as for a configured peer (AC-11) | to write |
| `peer-configured-delete-purges-rib` | `test/plugin/` | An operator removes a committed peer, and the RIB afterwards holds none of its prefixes (AC-5) | to write |
| `peer-deactivate-purges-rib` | `test/plugin/` | An operator deactivates a committed peer, and the RIB afterwards holds none of its prefixes while the peer stays visible as administratively down (AC-1, AC-5) | to write |
| `peer-ephemeral-delete-purges-rib` | `test/plugin/` | An operator runs `delete bgp peer` on an ephemeral peer, and the RIB afterwards holds none of its prefixes (AC-12) | to write |
| `peer-purge-emits-one-event` | `test/plugin/` | A peer carrying many prefixes is torn down, and the consumer counts the purge messages rather than the prefixes (AC-7) | to write |
| `peer-activate-restores-the-session` | `test/plugin/` | An operator activates a deactivated peer, and the session and its routes return under the same identity (AC-2) | to write |
| `peer-origin-reported` | `test/plugin/` | An operator asks which peers are configured and which are ephemeral, and gets the right answer for each (AC-13) | to write |
| `peer-delete-on-a-configured-peer` | `test/plugin/` | An operator runs `delete bgp peer` on a configured peer and meets the answer the spec chose, with the reactor and the file still agreeing (AC-14) | to write |
| `peer-ephemeral-survives-or-does-not-survive-a-commit` | `test/plugin/` | An ephemeral peer meets a commit that does not name it, and the outcome is the same on both apply paths (AC-15) | to write |
| `peer-flow-control-pause-keeps-the-session` | `test/plugin/` | An operator uses the flow-control pause and the session stays established with its routes in place (AC-16) | to write |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->

This spec changes what reaches the wire twice. It adds a Cease NOTIFICATION to an
operator-initiated stop, and it removes routes another daemon holds. Both are
peer-observable, so this spec owes an interop scenario. A functional test can
read Ze's own RIB and cannot show that the neighbouring daemon dropped the
prefixes, so the scenario is what proves AC-3, AC-4 and AC-6 against an
implementation Ze did not write. The scenario directory is named and carries no
numeric prefix (`ai/rules/interop-and-goal-validation.md`). Neither scenario
exists today.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `peer-deactivate-withdraws` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP] | The deactivated peer receives a Cease NOTIFICATION, and a third daemon loses every route that peer contributed and no other | to write |
| `peer-ephemeral-delete-withdraws` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP] | A peer created by `create bgp peer` and deleted by `delete bgp peer` produces the same Cease and the same withdrawals a configured peer produces | to write |

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
