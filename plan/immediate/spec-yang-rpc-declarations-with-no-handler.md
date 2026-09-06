# Spec: yang-rpc-declarations-with-no-handler

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator needs, and what the schema already promises.** An operator
adds and removes BGP peers on a running router. Ze offers one route today, and
it goes through the configuration file: `set bgp peer ...` then `commit`. That
route works and is proven end to end by `test/plugin/rest-peer-set-delete-lifecycle.ci`,
whose driver `restPeerLifecycle13` (`internal/test/fixture/plugin_fixture_13_rest.go`)
adds a peer, commits, deletes another, commits, and re-adds the first, checking
the reactor's peer table after each commit. Ze needs a second route beside it,
for a peer that lives in the running daemon and is never written to the file.
The schema already publishes that second route. `ze-bgp-api.yang`
(`internal/component/bgp/yang/ze-bgp-api.yang`) declares `peer-add` "Add a new
peer." and `peer-remove` "Remove a peer.".
`internal/component/bgp/plugins/cmd/peer/yang/ze-bgp-cmd-peer-api.yang` declares
`peer-add` "Add a peer dynamically." and `peer-save` "Save peer(s) to config
file.". `cmdMethods` (`internal/component/config/schema/cli/main.go`) prints
each one to an operator, and `Build` (`internal/component/aihelp/aihelp.go`)
puts each one in the `RPCs` array of `ze help ai --json`. An unread config leaf
is skipped in silence. A declared rpc with no handler is PUBLISHED as a method
Ze answers.

**The model this spec builds.** Three RPCs act on one runtime working set, and
the configuration file is reconciled to that set on demand.

| RPC | What it does | What it does NOT do |
|---|---|---|
| `peer-create` | Builds the peer in the reactor and starts it | Write the configuration file |
| `peer-delete` | Stops the peer and removes it from the reactor | Write the configuration file |
| `peer-save` | Makes the configuration file state the running peer set | Change the running set |

`peer-save` persists presence AND absence. A peer created at runtime is written
into the file. A configured peer deleted at runtime is taken out of it. This
sits BESIDE the configured route, and neither replaces the other.

**What Ze does instead today, read at each producer.** `handleBgpPeerRemove`
(`internal/component/bgp/plugins/cmd/peer/peer.go`) calls `Reactor.RemovePeer`
and touches no configuration. That is right for a runtime peer. For a configured
peer it is wrong twice: the peer returns at the next daemon start, and the
transaction tree still holds it. `Reactor.AddDynamicPeer`
(`internal/component/bgp/reactor/reactor_peers.go`) builds a peer from a config
tree and calls `AddPeer`, writing nothing to the configuration. At HEAD no
handler and no command node reach it. Uncommitted work in this checkout adds
`create bgp peer` as `ze-bgp:peer-add`, in `create.go` and `ze-peer-cmd.yang`
under `internal/component/bgp/plugins/cmd/peer/`. `peer-save` has no handler and
no command node, and it is declared twice. No handler registers the prefix
`ze-bgp-cmd-peer:` at all: `WireModule` (`internal/component/config/yang/rpc.go`)
strips the `-api` suffix, so all 14 rpcs of `ze-bgp-cmd-peer-api.yang` publish
under a prefix nothing serves. `peer-pause` and `peer-resume` work only because
a command node in `ze-peer-cmd.yang` names `ze-bgp:peer-pause`, which
`peer.go` does register.

**The runtime peer has no origin marker, and that blocks everything.**
`reconcilePeersJournaled` (`internal/component/bgp/reactor/reactor_api.go`)
removes every peer the new configuration does not name, unless
`PeerSettings.IsDynamic` is set. `IsDynamic`
(`internal/component/bgp/reactor/peer_settings.go`) marks a peer built from a
listen-range group template, and `AddDynamicPeer` sets it on no peer it builds.
So a runtime-created peer dies at the next commit of any leaf, and the operator
who created it is given no reason. A marker naming where each peer came from,
the configuration file, a listen range, or a command, is PREREQUISITE work. The
reconcile reads it to keep a command-created peer, and `peer-save` reads it to
tell a peer it must write into the file from a peer the file already names. The
marker becomes the configuration value once `peer-save` has written the peer
out.

**Both front doors, and what each can reach.** An operator types the command.
A plugin calls `Plugin.DispatchCommand` (`pkg/plugin/sdk/sdk_engine.go`), which
routes the same string through the engine's command dispatcher.
`plugin01APIPeerRemove` (`internal/test/fixture/plugin_fixture_01_api.go`)
already drives `delete bgp peer 127.0.0.1` that way, under
`test/plugin/api-peer-remove.ci`. A third caller reaches the same commands:
`convertNeighborCreate` (`internal/exabgp/bridge/bridge_neighbor.go`) translates
ExaBGP's `create neighbor` to `create bgp peer` and `delete neighbor` to `delete
bgp peer`, and the `api-peer-lifecycle` profile
(`internal/le/interoplab/bgp/exabgp_helpers.go`) sends `create neighbor` then
`announce route 1.1.0.0/24`, whose UPDATE bytes
`test/exabgp-compat/api/api-peer-lifecycle.ci` asserts. A plugin cannot start a
configuration transaction: the SDK's config surface is
`OnConfigOperationDecompose` and `OnConfigOperationApply`
(`pkg/plugin/sdk/sdk_callbacks.go`), which RECEIVE an operation the daemon
decided, and `ze-config-cli-cmd.yang` declares read commands only. REST is the
one initiating surface, through `ConfigSessionManager.Commit`
(`internal/component/api/config_session.go`). So `peer-save` is what gives a
plugin any way to persist a peer at all.

**The seam `peer-save` lands on.** `handleBgpPeerPrefixUpdate`
(`internal/component/bgp/plugins/cmd/peer/prefix_update.go`) is the only handler
that writes configuration: it opens `cli.NewEditor` over the daemon's config
path and calls `SaveDraft`, leaving a draft the operator must commit. That is
not enough for `peer-save`, which must land in the file AND in the running
configuration. `ConfigSessionManager.Commit` calls the `onCommit` reload hook.
`Editor.CommitSession` (`internal/component/cli/editor_commit.go`) writes the
file and fires no hook, so a handler that calls it leaves the reactor holding
the old tree.

**`peer-save` takes no selector, and the declaration that says "peer(s)" is
wrong.** The reading this spec takes is that `peer-save` acts on the whole
running set. A selector cannot express the half the owner asked for: after
`peer-delete 10.0.0.1` the peer is gone from the running set, so a selector
naming it selects nothing, and the ABSENCE is the fact being persisted. A
selector form would answer "0 peers saved" whether the peer was deleted, never
existed, or the word was mistyped, which is the silently wrong value
`ai/rules/principles.md` bans. An operator who wants one specific peer in the
file already has `set bgp peer ...` and `commit`. That reading constrains where
the command node goes: the `update bgp peer` subtree in `ze-peer-cmd.yang`
declares a mandatory `selector` leaf that every node under it inherits, so a
selector-free save cannot live there. `update bgp config` is the candidate the
design phase starts from, and the grammar feeders decide it
(`./le cli-grammar`, `ai/rules/cli.md`).

**One verb, and the sites that carry the other two spellings.** The tree
declares `peer-remove`, the owner says `peer-delete`, and the served handler is
`ze-delete:bgp-peer`. `ai/rules/cli.md` states "Ze is unreleased, so a second
spelling MUST be renamed outright rather than aliased".
`docs/contributing/ze-go-style.md` states "Ze keeps `delete` for config, `clear`
for counters, and `remove` for a route", so `remove` names a route operation and
`peer-remove` takes a word that is spoken for. The operator types `delete bgp
peer`, and `create` and `delete` are the runtime-resource lifecycle pair the
verb table declares (`Verbs`, `internal/component/command/verbs.go`). The three
published methods are therefore `ze-bgp:peer-create`, `ze-bgp:peer-delete` and
`ze-bgp:peer-save`, one prefix for one family.

| Site | What changes |
|---|---|
| `internal/component/bgp/yang/ze-bgp-api.yang` | `rpc peer-add` becomes `peer-create`, `rpc peer-remove` becomes `peer-delete` |
| `internal/component/bgp/plugins/cmd/peer/yang/ze-bgp-cmd-peer-api.yang` | the whole module, see the verdict below |
| `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` | `ze:command "ze-bgp:peer-add"` and `ze:command "ze-delete:bgp-peer"` |
| `internal/component/bgp/plugins/cmd/peer/peer.go` | the two `RPCRegistration` wire methods |
| `internal/component/bgp/plugins/cmd/peer/yang/cmd_schema_test.go` | asserts the `ze:command` string |
| `internal/component/cmd/delete/yang/self_containment_test.go` | maps the wire method to its owning package |
| `internal/component/command/help_test.go` | a fixture row carries the wire method |
| `internal/component/config/yang/command_test.go` | asserts `GetCommandExtension` answers the wire method |
| `internal/core/ipc/yang_test.go` | `TestYANGBGPAPIRPCs` and `TestExtractRPCs` name lists |
| `docs/architecture/exabgp-bridge.md` | states which command `create neighbor` reaches |

**The other declarations, judged under the same model.**
`ze-bgp-cmd-peer-api.yang` is a second declaration of a command surface
`ze-peer-cmd.yang` already carries, at a prefix no handler serves. Five of its
14 names are declared again in `ze-bgp-api.yang`. Every one of its rpcs whose
command works has its own `description` and `ze:help` on its command node in
`ze-peer-cmd.yang`, `pause` and `resume` among them, so deleting the module
loses no prose and removes 14 unreachable catalog entries. It holds the only
`peer-save` declaration in the tree, and the replacement goes in
`ze-bgp-api.yang` beside `peer-create` and `peer-delete`. The two
`ze-cli-set-api.yang` names are the clearest deletion in the set, because their
own sibling records the removal: `ze-cli-set-cmd.yang` carries `revision
2026-06-03` reading "Removed set bgp peer with/save.", above a comment reading
"Peer config goes through the editor, and the parallel runtime-then-persist path
is dropped." That decision is what the owner has now reversed, and the new path
is the three RPCs above rather than a revival of `set bgp peer with`.
`peer-update-hex` holds no wire method of its own: `handleUpdate`
(`internal/component/bgp/plugins/cmd/update/update_text.go`) is registered once
as `ze-bgp:peer-update` and switches on the encoding word to reach text, hex,
b64 and cursor, so the declaration is deleted. `command-help` and
`command-complete` in `ze-rib-api.yang`
(`internal/component/bgp/plugins/rib/yang/ze-rib-api.yang`) publish
`ze-rib:command-help` and `ze-rib:command-complete`, which no handler serves,
while `internal/plugins/meta/cmd/help.go` registers `ze-bgp:command-help` and
`ze-bgp:command-complete`. The RIB pair is deleted. `peer-show`,
`peer-show-capabilities` and `peer-show-statistics` in `ze-bgp-api.yang` are
near misses of a rename: `ze-bgp:peer-detail`, `ze-bgp:peer-capabilities` and
`ze-bgp:peer-statistics` are registered and answer what the three describe, so
the three declarations are repointed at the served names.

**Which tests exist, and which do not.** `test/plugin/api-peer-remove.ci` proves
a plugin can delete a peer and that it leaves `show bgp peer list`.
`test/plugin/rest-peer-set-delete-lifecycle.ci` proves the configured route over
REST. `test/editor/workflow/workflow-peer-lifecycle.et` proves `set`, `delete`
and `commit` over the configuration tree. None of the three reads a RIB.
`create bgp peer` has unit tests over the handler with a mock reactor
(`create_test.go`) and no functional test of its own. One case comes closest:
`test/exabgp-compat/native/api-peer-lifecycle.conf` drives ze through the
ExaBGP bridge, and its `.ci` asserts the UPDATE bytes a route announced through
the created peer puts on the wire. Whether that case is red at HEAD is
UNVERIFIED here, and `./le functional exabgp-test` settles it. The owner's bar
is that both routes work with the RIBs, so create means routes reach the
Adj-RIB-In and the RIB, and delete means they leave and the withdrawals reach
consumers. That bar is met by no test today.

**Why no gate saw any of this.** `./le docvalid command-contract` answers "All
commands validated." and states its purpose as "every YANG command node has a
handler, and every handler a node" (`internal/le/docvalid/actions.go`). `Validate`
(`internal/le/docvalid/contract.go`) keeps only modules whose name ends in
`-cmd`, so an `-api` module is never opened, and it collects a node only where
`GetCommandExtension` answers a `ze:command` value, which a YANG `rpc` never
carries. The same function computes `orphanLocalHandlers` and hands it to the
report, while `contractSatisfied` reads `orphanYANG` and `orphanHandlers` alone,
so the rows the run prints under "Local handlers with no YANG command" change no
verdict. Both holes are closed by this spec. One test does see part of it:
`TestEveryCommandNodeHasASummary` (`internal/le/docvalid/helpshape_test.go`) is
RED in this checkout with 6 refusals, and five of them are the rpc declarations
this spec removes, `ze-bgp-cmd-peer-api:peer-add`,
`ze-bgp-cmd-peer-api:peer-save`, `ze-bgp-cmd-update-api:peer-update-hex`,
`ze-cli-set-api:bgp-peer-save` and `ze-cli-set-api:bgp-peer-with`. Two tests
assert the dead declarations exist and both are wrong about what they assert,
because neither checks that a handler answers: `TestYANGBGPAPIRPCs` asserts each
name is present, and `TestExtractRPCs` matches the whole set with
`assert.ElementsMatch` (`internal/core/ipc/yang_test.go`). The rows for this
class are already written in `plan/journal/unwired-feature.md`, dated 2026-09-03
and 2026-09-05, so this spec adds none.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `path/to/file.go` - [what it currently does]

**Behavior to preserve:** (unless the user explicitly said to change it)
- [output format, function signature, or `.ci` expectation callers depend on]

**Behavior to change:** (only what the user asked for)
- [list, or "None - preserve all existing behavior"]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A YANG API module is loaded at start, and its `rpc` statements are read as schema text.
- An operator or an AI agent reads a method name from `ze schema methods` or from `ze help ai --json`.
- A caller sends that method name over the plugin IPC transport as a wire method.

### Transformation Path
1. Module load and resolve in `internal/component/config/yang/loader.go`.
2. RPC extraction in `ExtractRPCs` (`internal/component/config/yang/rpc.go`).
3. Wire method construction in `RegisterRPCs` (`internal/component/plugin/server/schema.go`), which strips the `-api` suffix through `WireModule`.
4. Publication in `cmdMethods` (`internal/component/config/schema/cli/main.go`) and in `Build` (`internal/component/aihelp/aihelp.go`).
5. Dispatch, which reads the handler registry `AllBuiltinRPCs` answers and never consults stage 2.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema ↔ handler registry | no link exists: the declaration and the `RPCRegistration` name the method independently | No |
| Gate ↔ YANG modules | `Validate` (`internal/le/docvalid/contract.go`) opens `-cmd` modules only | No |

### Integration Points
- `Validate` and `contractSatisfied` (`internal/le/docvalid/contract.go`) - the gate the new check extends rather than duplicates.
- `collectRPCs` (`internal/le/docvalid/helpshape.go`) - already walks every module's RPCs, so the corpus the new check needs is loaded beside it.

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
| AC-1 | An operator types `create bgp peer 10.0.0.1 asn 65001` on a running daemon | `show bgp peer list` names 10.0.0.1, and the peer dials the address |
| AC-2 | The peer of AC-1 reaches Established and sends an UPDATE carrying 192.0.2.0/24 | `show bgp peer 10.0.0.1 rib` and `show bgp rib` both answer the prefix, and the best path for it names that peer |
| AC-3 | AC-1 has run | The configuration file on disk is byte-identical to what it was before, and `show config` names no peer 10.0.0.1 |
| AC-4 | A plugin calls `ze-bgp:peer-create` for 10.0.0.1 through the command dispatcher | The answer carries the peer address, the remote AS and the outcome, and AC-1 through AC-3 hold identically |
| AC-5 | An operator types `delete bgp peer 10.0.0.1` while that peer holds 192.0.2.0/24 | `show bgp peer list` names no 10.0.0.1, and 192.0.2.0/24 is absent from `show bgp rib` |
| AC-6 | A second peer had received 192.0.2.0/24 from Ze before AC-5 | That peer receives a WITHDRAW for 192.0.2.0/24, and a plugin subscribed to route events is told the route is gone |
| AC-7 | AC-5 has run against a peer the configuration file declares | The configuration file on disk is unchanged, and a daemon started on that file brings the peer up again |
| AC-8 | A plugin calls `ze-bgp:peer-delete` for 10.0.0.1 | AC-5 through AC-7 hold identically |
| AC-9 | An unrelated leaf is set and committed while a peer created by AC-1 is running | The created peer stays up, keeps its session, and keeps its Adj-RIB-In |
| AC-10 | `ze-bgp:peer-save` runs after AC-1 | The configuration file names peer 10.0.0.1 with the AS and every other value the create command stated, and a daemon started on that file brings the peer up |
| AC-11 | `ze-bgp:peer-save` runs after AC-5 removed a peer the file declared | The configuration file names that peer no longer, and a daemon started on that file does not bring it up |
| AC-12 | `ze-bgp:peer-save` runs at all | The running configuration and the file agree afterwards, so a later commit of an unrelated leaf removes no peer |
| AC-13 | An operator types a word after `peer-save` | The command is refused, and the refusal says the command takes no selector and acts on the whole running set |
| AC-14 | `ze schema methods` and `ze help ai --json` are read on a built daemon | Every method they publish has a registered handler |
| AC-15 | A YANG `-api` module declares an rpc whose published wire method no handler serves | `./le docvalid command-contract` fails and names the module, the rpc and the wire method |
| AC-16 | A registered handler has no YANG command node and no rpc declaration | `./le docvalid command-contract` fails, rather than printing the row under a passing verdict |
| AC-17 | `ze-bgp-cmd-peer-api.yang`, the two `ze-cli-set-api.yang` names, `peer-update-hex`, and the two `ze-rib-api.yang` introspection names are gone | `TestEveryCommandNodeHasASummary` reports no refusal for an rpc declaration |

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
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-feature-peer` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

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
