# Plugin Process Protocol

`EmitEvent` confirms queue admission. `EmitEventApplied` is the opt-in lifecycle
form of the same engine operation: `await-applied: true` and a positive absolute
`deadline-ms` request captured subscriber application and declared deferred-work
settlement. Ordinary inputs omit both new fields and retain their old behavior.
Its completion receipt explicitly confirms `applied` and the same absolute
deadline; an older queue-only host cannot satisfy it. The applied SDK method
uses the common cancellable RPC in both placements; the
legacy typed emit slot remains queue-only.

The host captures each matching Process and its original connection, refuses an
actual self subscription before publication, and obtains one application result
per receiver. It joins all handlers first, then calls `SettleEventsApplied` on
each captured receiver. This second pass includes events published between those
receivers by cleanup itself. A successor process or connection cannot satisfy
that receipt. Engine subscribers still run synchronously. A zero subscriber
count describes an empty captured population, never a fallback to a current
process by name.

Callers must avoid recipient locks and cyclic synchronous event dependencies.
Cancellation uses the existing remote request cancellation/join protocol. Partial
application is not rolled back; a refused handler, changed connection, timeout or
unsettled callback reports failure with remaining cleanup/output explicitly owed.
The caller must not proceed as if the lifecycle barrier succeeded. Frame-size
accounting includes the optional mode and its widest deadline.
<!-- source: pkg/plugin/sdk/sdk_emit_applied.go -- EmitEventApplied -->
<!-- source: internal/component/plugin/server/emit_event_applied.go -- emitEventApplied, emitEventCancellationScope -->
<!-- source: internal/component/plugin/server/dispatch.go -- deliverEventCaptured -->

GR's internal source lifecycle dispatch uses `request bgp rib source`
commands with explicit original Peer `owner` and physical `connection`. Their
registered RIB handlers narrow received references and existing pending leases;
those identifiers do not mint source rights. A release or purge command's
success joins the original captured output and source lease release. Partial
mutation followed by a failed join is an error with completion still owed, not
successful cleanup; the replacement physical connection cannot settle it.
The `retain-routes` answer carries `earlier-connections`: the earlier
connections of the same Peer whose routes stay retained beside the named one
after a consecutive restart, as decimal strings, empty for a first restart.
GR addresses each of them with its own purge and release.
<!-- source: internal/component/bgp/plugins/rib/rib_source_commands.go -- ribSourceCommands, retainedSourceAnswer -->
<!-- source: internal/component/bgp/plugins/rib/rib_source_mutation.go -- removePhysicalSource -->

**Purpose:** Document Ze's plugin communication protocol and process lifecycle.

---

## Overview

Ze plugins communicate with the engine via newline-framed YANG RPCs over a
single bidirectional connection. All messages use the wire format
`#<id> <verb> [<json>]\n`.
<!-- source: pkg/plugin/rpc/conn.go -- Conn doc comment -->

- **Events (engine to plugin):** BGP events delivered via `deliver-event` or `deliver-batch` RPCs. The `bgp` namespace registers `update-filtered` alongside `update`, so subscription grammar, YANG `ReceiveEventValidator`, and CLI completion derive it automatically; only subscribers to `update-filtered direction received` (such as `bgp-adj-rib-in`) receive ingress-denied route-server client UPDATEs.
<!-- source: internal/core/bgp/events/events.go -- EventUpdateFiltered -->
<!-- source: internal/component/bgp/plugin/register.go -- init -->
- **Commands (plugin to engine):** Route updates, command dispatch, event emission via engine RPCs
- **Callbacks (engine to plugin):** Config verification, command execution, OPEN validation

The protocol is the same for all invocation modes (internal goroutine, external subprocess).
Internal plugins get a performance optimization via `DirectBridge` after startup.
<!-- source: pkg/plugin/rpc/bridge.go -- DirectBridge -->

The in-process raw BGP observer receives the complete original header and body,
message type and direction, separately from semantic route callbacks. Receive
observation precedes message-body validation and coalescing; send observation follows accepted
transport bytes, not buffer staging. Synthetic withdrawals are not wire packets.
Its `PeerInfo.MessageContextID` identifies the immutable directional encoding
context; observers MUST use it rather than configured capabilities, MUST NOT
retain borrowed bytes, and MUST NOT reenter session writes. MRT copies that
complete message without reconstructing it and marks sent records LOCAL.
The observer's LocalAS is the ASN of the OPEN built for that connection.
RFC 7705 migration fallback can make it differ from configured LocalAS, so
`sendOpen` publishes the built OPEN identity before its first byte is written.
The connection retains that value for both directions and for an old writer
flushed after replacement; dispatch does not consult the current migration flag.
<!-- source: internal/component/plugin/types_bgp.go — PeerInfo -->
<!-- source: internal/component/bgp/reactor/session_wire.go — observeReceivedWire, notifyWireMessage -->

---

## SDK event correlation

`sdk.NewUnion` correlates primary and secondary events by peer and message ID.
Its sweep interval and pending-entry limit are fixed before its worker starts.
Package tests supply these values through the private constructor rather than
change a running worker. Callers MUST call `Stop` when the union is no longer needed.
<!-- source: pkg/plugin/sdk/union.go -- NewUnion, newUnion, Union.Stop -->

## Wire Format

Every message is a single newline-terminated line:
<!-- source: pkg/plugin/rpc/message.go -- AppendRequest, AppendResult, AppendError -->

| Message type | Format | Example |
|-------------|--------|---------|
| Request | `#<id> <method> [<json>]\n` | `#1 ze-plugin-engine:ready {"subscribe":{...}}` |
| Success | `#<id> ok [<json>]\n` | `#1 ok {"peers-affected":2}` |
| Error | `#<id> error [<json>]\n` | `#1 error {"code":"error","message":"..."}` |

A command answer has a second form, and every peer reads it and writes it. The
engine answers a plugin's `dispatch-command` and `dispatch-command-args` with a
head, zero or more records, and a terminator. The plugin answers the engine's
`execute-command` the same way. Every other request keeps the single line above.

Each answer line is `#<id> <kind> <positional fields>`, where the kind is `top`,
`row`, `bad`, `end` or `nay`. An answer line carries no verb and no key name.
"Command Execution" below names each word and decodes a real answer field by
field. [ipc_protocol.md](ipc_protocol.md), "Answer Protocol", carries the same
grammar with the buffering threshold and the failure cases beside it.
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerHead, AppendAnswerTerminator -->
<!-- source: internal/component/plugin/server/dispatch_registry.go -- recordAnswer, opDispatchCommand -->
<!-- source: internal/component/plugin/ipc/rpc.go -- PluginConn.SendExecuteCommandAnswer -->
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- Plugin.answerExecuteCommand -->

- `<id>` is a monotonically increasing `uint64` correlation ID
<!-- source: pkg/plugin/rpc/conn.go -- NextID -->
- Methods use YANG-style `<module>:<rpc-name>` naming
- JSON payloads are optional (omitted when empty or null)
- Error payloads carry `code` and `message` fields
<!-- source: pkg/plugin/rpc/message.go -- NewErrorPayload -->

**Framing:** `FrameReader` uses `bufio.Scanner` with newline splitting.
`FrameWriter` appends `\n` to each message. The plugin protocol keeps
`MaxMessageSize` (16 MiB) on both directions; `NewFrameReaderMax` and
`NewConnWithMaxRead` allow other consumers (such as the managed fleet listener)
to enforce a tighter inbound line bound without changing plugin framing.
<!-- source: pkg/plugin/rpc/framing.go -- FrameReader, NewFrameReaderMax, FrameWriter, MaxMessageSize, ErrMessageTooLong -->

**Multiplexing:** `MuxConn` wraps a `Conn` to support concurrent RPCs on a single
connection. A background reader goroutine routes responses (verb `ok`/`error`) to
waiting `CallRPC` callers by `#<id>`, and pushes inbound requests (verb is a method
name) to the `Requests()` channel.
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn, readLoop -->

A response already routed to a `CallRPC` caller is consumed before the reader's
terminal error, even when the peer closes immediately after sending it. A close
with no queued response remains an error. Waiting calls remain cancelable
through their context.
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn.CallRPC, MuxConn.queuedResponse -->

---

## 5-Stage Startup Protocol

Stage 1 can carry `dissemination`, a list of pure peer-role declarations.
Each names `namespace`, `role`, `source-receive`, `destination-receive` and
`destination-send`. Receive tokens use the attachment vocabulary, including
direction suffixes. Each required list is nonempty and names specific grants.
The engine validates the declaration, then retains it with that authenticated
process's Stage 1 value on both transports. It creates no attachment or grant.
A matching `claims` token identifies an explicit owner. The actual resolved
peer graph decides whether that configured instance is eligible; source and
destination grants on differently named instances never complete each other.
The query-mode declaration carries these values and Claims too.
<!-- source: pkg/plugin/rpc/dissemination.go -- DisseminationDecl -->
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput -->
<!-- source: internal/component/plugin/server/startup.go -- registrationFromRPC -->
<!-- source: internal/component/plugin/server/stage1_checks.go -- stage1Checks -->
<!-- source: internal/component/plugin/cli/main.go -- answerQuery -->

Stage 1 judges attachments from the exact configuration selected for that
process's Configure attempt. Each protocol registers a pure resolver for its
namespace; the scope builds immutable candidate graphs before holding its
lifecycle lock. A new external declaration is therefore checked against the
new grants in the same candidate, rather than the old running peer graph.
Validation publishes no candidate and grants no delivery. A missing namespace
resolver is a refusal. Runtime restarts outside a Configure attempt use the
running graph.
<!-- source: internal/component/plugin/server/dissemination_config.go -- RegisterDisseminationConfigResolver, candidateDisseminationGraphs, validateDisseminationForProcess -->
<!-- source: internal/component/bgp/config/dissemination_config.go -- peerSettingsFromTree, disseminationPeersFromTree -->

Before the first peer starts, the BGP reactor rebuilds its configuration once
against the complete runtime declarations, including programs absent from the
compiled registry. Only success completes this initial gate; a failed attempt
can be retried, and a replacement reactor owes its own gate. An auto-load during
reload does not rebuild the old running grants with the new declarations: the
existing final candidate apply validates and reconciles that configuration.
Outside a Configure attempt, a respawn or removal that changes runtime filter
insertions, Claims or Dissemination rebuilds peers against the running graph.
The runtime builder defers ownership to these declarations; offline validation
uses compiled declarations. A replacement whose changed declarations cannot
rebuild the peers is stopped and reported as a failed restart, including when
the rebuild waits for an in-flight transaction to release its running tree.
Before its handshake exposes a bridge, each replacement holds its own output
admission. JSON and typed output calls refuse while that exact process waits
for reconciliation; setup state calls and runtime cleanup remain available.
Only the current process for that configured name resumes output and receives
post-startup after success. A deferred unchanged successor keeps the hold until
the outstanding rebuild settles. A failed rebuild never delivers post-startup.
The initial fan-out and admission share a per-process single delivery, including
the interval before the daemon's startup-complete signal closes.
<!-- source: internal/component/bgp/plugin/initial_peers.go -- initialPeerConfig.resolve -->
<!-- source: internal/component/plugin/process/peer_declarations.go -- HoldPeerDeclarations, RequirePeerDeclarations, AdmitPeerDeclarations -->
<!-- source: internal/component/plugin/server/restart.go -- admitPeerDeclarationReplacement -->

State and received ROUTE-REFRESH events carry `destination-session`, an opaque
decimal string naming the captured connection for its entire lifetime. It
remains present after initial End-of-RIB, and a DOWN names the ended Session
rather than a replacement at the same address. A delayed refresh retains its
original receiving Session's token and cannot acquire the replacement's output
eligibility. An observation without a captured Session remains unrecorded. The
token is independent of `initial-replay`, which retains its initial routing-
update meaning; refresh never asserts another initial replay. The typed event
carries the same uint64 value.

For each actual recipient, `selected-destination: true` says that this process
is the resolved ordinary output owner with the complete explicit destination
grants. A state observer, a source-only process and a destination served by an
explicit role receive no such eligibility. On an attached-UP, only the eligible
ordinary owner receives its captured connection token as `initial-replay` so
it can seed a current selected snapshot. Legacy attached observers keep zero.
These fields describe the captured delivery; they grant no permission to an
RPC, whose final writer rechecks its own authenticated authority. Text state
and refresh events carry the same destination-session and selected-destination
fields. Refresh eligibility is projected for each actual recipient on typed and
JSON/text delivery; a sent control observation grants no received-refresh
eligibility. Cached renderings distinguish owner from observer. A cache miss
renders that recipient's exact event without growing the fixed cache.
<!-- source: internal/component/bgp/reactor/peer_selected_state.go -- selectedStateDestination, selectedDestinationForSession -->
<!-- source: internal/component/bgp/reactor/source_session.go -- sourceMessageCallback, messageReceiverForPeer -->
<!-- source: internal/component/bgp/server/selected_state.go -- selectedStatePeer, selectedRefreshPeer -->
<!-- source: internal/component/bgp/server/events.go -- deliverPeerState, getStructuredStateEvent -->
<!-- source: pkg/plugin/rpc/bridge.go -- StructuredEvent, PutStructuredEvent -->

An UP delivered to a peer-up replay claimant carries `update-send-denied: true`
when the captured peer does not grant that process `send [ update ]`.
`bgp-rs` keeps the peer as a route source and skips automatic replay and live
output to it. The writer checks the process's send grant for each attempt.
<!-- source: internal/component/bgp/reactor/peer_bound_info.go -- boundPeerInfo -->
<!-- source: internal/component/bgp/server/events.go -- deliverPeerState -->
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- handleState -->

On UP, `ordinary-replay-owned: true` tells every state recipient that the
captured connection has an ordinary selected-output owner. Auxiliary stores
then skip automatic peer-up replay and still report readiness for that connection.
This advisory grants no output eligibility or RPC permission. DOWN, a missing
connection token, and destinations served by explicit roles carry false.
Typed, JSON and text events carry the same advisory. Explicit replay commands
keep their existing behavior.
<!-- source: internal/component/bgp/server/selected_state.go -- selectedStatePeer -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- handleStructuredState, handleState -->

Ze uses a synchronized 5-stage startup protocol with barriers between stages.
Within each dependency tier, all plugins must complete each stage before any
can proceed to the next. Tiers are sequenced by dependency order (see
Tier-Ordered Startup below).

A plugin with post-ready initialization declares `started-handler` at Stage 1.
The Go SDK derives this declaration from `OnStarted`. After acknowledging
`ready`, the engine asks `runtime-ready` through the ordinary callback queue.
The SDK can answer only after `OnStarted` has returned successfully and its
event loop has begun. The startup tier waits for this receipt before starting
a dependent tier. A socket plugin with no initializer declares none and owes no
initializer receipt.

Every negotiated internal bridge also owes this receipt, even with no
initializer. The final ready response can reach the SDK reader before the SDK
activates its typed slots. The queued `runtime-ready` answer joins that
activation before startup exposes those slots. A completed pipe write alone
cannot establish runtime readiness.

Initialization failure, timeout or cancellation fails startup. The engine does
not publish a ready dependency until initialization succeeds.

Socket initialization may still call the engine while the receipt is pending:
the startup reader serves those calls through the normal engine-operation
registry. Internal initialization uses the same engine operations through its
bridge. Neither route waits for the later runtime supervisor to begin reading.
In tiered startup, the accepted apply allowance bounds initialization and
configure work separately. Ordinary wire stages retain the configured stage bound.
Unbarriered startup retains its caller's handshake scope as its only bound.
Parent cancellation still wins.
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->
<!-- source: internal/component/plugin/server/startup_driver.go -- runStartupHandshake -->

```
+---------------------------------------------------------------------------+
|                          STARTUP TIMELINE                                 |
+---------------------------------------------------------------------------+
|                                                                           |
|    Plugin A               Coordinator              Plugin B               |
|    --------               -----------              --------               |
|                                                                           |
|    STAGE 1: REGISTRATION                                                  |
|    #1 declare-registration  |                 #1 declare-registration     |
|    {...} ------------------>|                 {...} ------------------>    |
|    <-- #1 ok                |                 <-- #1 ok                   |
|                             |                                             |
|                  BARRIER (all plugins complete Stage 1)                   |
|                                                                           |
|    STAGE 2: CONFIG DELIVERY                                               |
|    <-- #1 configure {...}   |                 <-- #1 configure {...}      |
|    #1 ok ------------------>|                 #1 ok ------------------>   |
|                             |                                             |
|                  BARRIER (all plugins complete Stage 2)                   |
|                                                                           |
|    STAGE 3: CAPABILITY DECLARATION                                        |
|    #2 declare-capabilities  |                 #2 declare-capabilities     |
|    {...} ------------------>|                 {...} ------------------>    |
|    <-- #2 ok                |                 <-- #2 ok                   |
|                             |                                             |
|                  BARRIER (all plugins complete Stage 3)                   |
|                                                                           |
|    STAGE 4: REGISTRY SHARING                                              |
|    <-- #2 share-registry    |                 <-- #2 share-registry       |
|    {...} ------------------>|                 {...} ------------------>    |
|    #2 ok                    |                 #2 ok                       |
|                             |                                             |
|                  BARRIER (all plugins complete Stage 4)                   |
|                                                                           |
|    STAGE 5: READY                                                         |
|    #3 ready {...} --------->|                 #3 ready {...} --------->   |
|    <-- #3 ok                |                 <-- #3 ok                   |
|                             |                                             |
|                  BARRIER (all plugins ready)                              |
|                             |                                             |
|    [BGP peers start]        |                 [BGP peers start]           |
|                                                                           |
+---------------------------------------------------------------------------+
```

**Barrier Semantics:**
- Each plugin signals stage completion via `StageComplete(pluginID, stage)`
- Coordinator waits until ALL plugins complete the current stage
- Only then does coordinator advance to next stage
- All waiting plugins unblock simultaneously

**No BGP peer starts before the last barrier.** The bgp plugin starts its
reactor, and so opens its listeners, from its own configure callback, while
other plugins are still in their stages. Its peers start only from
`Reactor.StartPeers`, which the post-startup callback runs once every phase
has finished. Until then the reactor's `peersStarted` flag is false, and three
paths read it: the configured set, a peer `AddPeer` adds, and a member a
dynamic group creates for an accepted connection. A peer created in that
window stays inert, a passive one keeps its inbound socket in its inbound slot,
and `StartPeers` starts it with the rest. A dynamic member therefore cannot
reach Established before the filter plugins its import chain names are
connected, where its first UPDATE would meet "plugin not connected".

Before `StartPeers`, the post-startup callback rebuilds the peers once to
place the filter insertions the running plugins declared. That first rebuild
reads the reactor's current running map, which retains inactive member intent.
Startup, reload and compensation therefore use the same authority; a
deactivated member remains visible to insertion validation.
When the rebuild refuses the config (an operator names, moves or deactivates a
required insertion), the callback returns the refusal, the plugin server makes
it the startup error, and `ze` prints it and exits 1, as `ze config validate`
refuses the same config. A daemon never runs on with its peers held.
<!-- source: internal/component/bgp/plugin/register.go -- OnPostStartup -->
<!-- source: internal/component/plugin/server/startup.go -- signalStartupComplete -->
<!-- source: internal/component/bgp/reactor/reload_tree.go -- RebuildRunningPeers -->

Startup completion also checks whether the server lifetime has ended before
each remaining readiness step. Stop admitted before final publication, or
parent cancellation observed before that publication, produces a terminal
startup error and skips last-stop settlement. The final lifetime check decides
success: Stop admission is serialized with settlement and completion, while
parent cancellation after that check leaves the historical result unchanged.
Plugin and reactor callbacks run outside that lock and can themselves stop
the server. Ordinary live plugin failures still follow their failure policy.
<!-- source: internal/component/plugin/server/startup.go -- signalStartupComplete, finishStartupCompletion, startupCompletionStopped, WaitForStartupComplete -->

`StartPeers` refuses when a configured family has no decoder, and the daemon
keeps running after it. The barrier then never lifts for that run, so each peer
it holds, and each peer added or accepted later, is named in an error log
("peer will not start") rather than waiting in silence.
<!-- source: internal/component/bgp/reactor/reactor.go -- Reactor.peersStarted, openPeerStartBarrier, StartPeers, refusePeerStart -->
<!-- source: internal/component/bgp/reactor/reactor_dynamic.go -- createDynamicPeer -->
<!-- source: internal/component/bgp/reactor/reactor_peers.go -- AddPeer -->

**Stage RPCs:**
<!-- source: pkg/plugin/rpc/types.go -- all stage input types -->

| Stage | Direction | RPC Method | Input Type |
|-------|-----------|-----------|------------|
| 1. Registration | Plugin to Engine | `ze-plugin-engine:declare-registration` | `DeclareRegistrationInput`, answered with `DeclareRegistrationOutput` |
| 2. Config | Engine to Plugin | `ze-plugin-callback:configure` | `ConfigureInput` |
| 3. Capability | Plugin to Engine | `ze-plugin-engine:declare-capabilities` | `DeclareCapabilitiesInput` |
| 4. Registry | Engine to Plugin | `ze-plugin-callback:share-registry` | `ShareRegistryInput` |
| 5. Ready | Plugin to Engine | `ze-plugin-engine:ready` | `ReadyInput` |
| Post | Engine to Plugin | `ze-plugin-callback:post-startup` | (empty) |

`ReadyInput.subscribe` carries the startup subscriptions of one namespace.
`ReadyInput.subscriptions` carries further blocks, each naming its own
`namespace`, which the engine registers in the same step, before any event
flows (`sdk.Plugin.AddStartupSubscriptionsIn`). `bgp-rr` and `bgp-rs` use it to
hear `bgp-gr/retention` and `bgp-rib/validation-change` beside their `bgp`
events, the same way internal and forked; `bgp-rib` uses it to hear
`bgp-rib/validation-change` beside its own. Format, encoding and envelope stay
per process, set by the first block (`ReadyInput.format` accepts `parsed`,
`full`, `hex`, or `base64`, where `base64` delivers BGP UPDATEs as positional
counted `upd` lines while keeping control/session events as JSON).
<!-- source: internal/component/plugin/server/startup.go -- onReady -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- AddStartupSubscriptionsIn -->
<!-- source: internal/component/bgp/plugins/rib/rib_validation.go -- dispatchText -->

A `bgp-rib/validation-change` payload is a JSON array of received paths, each
`{"peer-address", "peer-owner", "connection-id", "family", "prefix", "path-id", "nlri"}`.
`peer-owner` is the original stable engine Peer identity, kept apart from its
address and source Session receipt. Zero means unrecorded provenance; an exact
lookup refuses it and never substitutes a current peer at that address. The
receive store captures this value with the stored or pending route, and both
typed queries and the JSON codec preserve it. `connection-id` is the original
physical connection, encoded as an exact decimal uint64 string. The key keeps
it apart from the stable owner and the received message ID. Pending and stored
paths preserve it through validation changes; a current UP cannot relabel old
bytes. An exact query with a missing or different connection does not match.
<!-- source: internal/core/bgp/ribevents/validation.go -- ValidationRoute, MarshalJSON, UnmarshalJSON -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_eligibility.go -- validationSourceMatches -->

`nlri` is the hex of
the native NLRI of a non-CIDR route (FlowSpec), absent for a CIDR one, and a
path whose `nlri` is not hex refuses the whole batch
(`ribevents.DecodeValidationChange`). `bgp-adj-rib-in` emits it through
`emit-event` (`sdk.Plugin.EmitEvent`), so it holds no EventBus of its own.
It emits only once validation has enabled its eligibility gate, in the
daemon or forked.
Large validation changes retain this array shape in consecutive byte-bounded
events. A validation command fails if any chunk is not acknowledged by the
engine; the store retains its unacknowledged keys and an explicit retry flushes
them before another mutation, even when validation is already disabled.
Acknowledged chunks are not replayed automatically. Consumers read current eligibility, so a retry after an
ambiguous transport timeout may repeat keys but never supplies an old verdict.
An ordinary receive or timeout delivery encountering paused debt fences the
receive store before mutation and closes its transport with a failure result.
A new hand-off failure also fences the store.
Its `fatal` declaration stops the daemon for any unexpected current owner exit.
An acknowledged live removal settles owed changes before fencing later mutations.
Runtime shutdown joins the timeout scanner before reporting the owner's result,
so a publication failure during shutdown remains a failure result.
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_publication_owner.go -- failValidationOwner, removeValidationOwner -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- runAdjRIBInPlugin -->
It also emits `bgp-rib/validation-gate`, `{"enabled": <bool>, "command":
<string>}`, after each `enable-validation` and `disable-validation`. `command`
names the hidden command the store serves for eligibility queries, so a
process that cannot read its lookup in memory asks through it, and refuses a
route the command cannot answer (`ribevents.NoteValidationGate`,
`docs/architecture/plugin/rib-storage-design.md`, "Validation eligibility").
`bgp-rib` subscribes to it beside `bgp-rib/validation-change`.
`bgp-rib` emits `bgp-rib/flowspec-gate`, `{"command": <string>}`, naming the
hidden command that answers its FlowSpec lookup (`ribevents.NoteFlowSpecGate`;
`bgp-rr`, `bgp-rs` and `bgp-adj-rib-in` subscribe to it). Any reader emits
`bgp-rib/gate-query`, `{}`, once its subscriptions are live, and each gate
owner then announces its gate again, so a late subscriber still learns it.
<!-- source: internal/core/bgp/ribevents/flowspec_remote.go -- FlowSpecGateState, GateQueryRequest, NoteFlowSpecGate -->
<!-- source: internal/core/bgp/ribevents/validation.go -- ValidationRoute.MarshalJSON, DecodeValidationChange -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_eligibility.go -- emitValidationEvent, emitValidationGateEvent -->

**Protocol version.** Every `declare-registration` carries `protocol-version`,
the highest plugin protocol version the plugin speaks, and the engine's Stage 1
answer includes `protocol-version`, the version both sides speak from then on:
the lower of the two (`rpc.NegotiateProtocolVersion`). One plugin-level version
serves every contract change, filters included. A declaration with none is the
text era, and the stage driver refuses it before any sink sees the
declaration, with one error naming the plugin, the version to declare and
`docs/plugin-development/protocol.md`. This engine speaks version 1
(`rpc.ProtocolVersion`).
<!-- source: pkg/plugin/rpc/declaration.go -- NegotiateProtocolVersion -->
<!-- source: internal/component/plugin/server/startup_driver.go -- runStartupHandshake -->

The Stage 1 answer also carries `plugin-name`: the configured instance name
from the authenticated process, which can differ from its implementation name.
The SDK exposes it through `Plugin.InstanceName` before configuration runs.
Before Stage 1 acknowledges it, that method returns an empty string. Runtime
diagnostics use this identity when they name an instance to restart.
<!-- source: pkg/plugin/rpc/declaration.go -- DeclareRegistrationOutput -->
<!-- source: pkg/plugin/sdk/sdk_identity.go -- InstanceName -->

**Nested engine calls during configuration.** The engine serves owner operations
declared `EngineOperationConfigureSafe` while it waits for the Stage 2 configure
answer. A handler receives the authenticated configured process and the bounded
startup context. It must finish without waiting for a later stage. Runtime-only
operations, state operations, codecs and unknown methods receive an immediate
error directing the caller to wait until ready. A nested call does not advance
the shared driver's barriers. The mux remains the only wire reader, and the
startup reader retains an already-arrived Stage 3 request until configure has
completed. A successfully sent nested refusal leaves configuration running.
If a nested response cannot be encoded or sent, startup cancels and joins the
configure waiter, then follows the normal configure-failure policy. Cancellation
also joins that waiter before delivery returns.
<!-- source: internal/component/plugin/server/startup_engine_op.go -- serveConfigureEngineOp -->
<!-- source: internal/component/plugin/ipc/configure_serving.go -- SendConfigureServing -->
<!-- test: internal/component/plugin/server/startup_route_view_test.go TestConfigureServesOwnerEngineOperation -->
<!-- test: internal/component/plugin/server/startup_route_view_test.go TestConfigureFailsWhenNestedReplyCannotEncode -->
<!-- test: internal/component/plugin/ipc/configure_serving_test.go TestConfigureServingRetainsNextStage -->

**A Stage 2 error response is a REFUSAL, and a dead transport is not.** A
plugin that cannot take the configuration it was sent answers `#<id> error`
with the reason. The engine reads that response, stops the plugin, and stops ze
itself when the plugin's registration carries `FatalOnConfigError`: a mistyped
address family gets an operator a refusal with the reason, not a running router
silently missing the feature that plugin owns.

A failure to DELIVER the configuration produces no response at all. A closed
connection, a timeout and a canceled context each fail the configure call
without the plugin having said anything, so none of them is a refusal and none
of them stops ze, whatever the registration asks for. The engine tells the two
apart by what comes back: a refusal parses into an `*rpc.RPCCallError`, which
only a response line can produce.
<!-- source: internal/component/plugin/server/startup.go -- isConfigRefusal, configRefusalIsFatal -->
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- serveOne, handleConfigure -->

**Stage 3 names no wire shape.** `DeclareCapabilitiesInput` carries the BGP
capabilities the plugin injects into OPEN and nothing else. A command answer has
one encoding on every connection, so a plugin READS the record answer form of
`dispatch-command` and `dispatch-command-args`, and it WRITES that form for
`execute-command`, without asking for either.
<!-- source: pkg/plugin/rpc/types.go -- DeclareCapabilitiesInput, CapabilityDecl -->
<!-- source: pkg/plugin/sdk/sdk.go -- Plugin.Run, Stage 3 declare-capabilities -->
<!-- source: internal/component/plugin/server/startup.go -- engineStartupSink -->

**A reload replaces the Stage 3 declaration through the config-verify answer.**
The engine holds the declaration for every OPEN. A config-verify answer may
carry `capabilities`, the plugin's whole declaration for the candidate, in the
shape Stage 3 sends; absent keeps the current one, and an empty list removes
them all. After every participating verifier has answered, including answers
that keep their declaration, the engine checks the complete candidate: staged
replacements together with declarations whose owners did not replace them.
One transaction may transfer a capability code and selector between owners;
two candidate owners claiming the same code and selector are refused before
any apply, as is a conflict with an unchanged owner. A participant that receives
only operation events is not a section verifier and does not hold this check.
The transaction's first apply publishes every staged replacement under the
capability injector's single write lock, before that apply's RPC, so a peer
the reload adds builds its OPEN from the complete candidate. Concurrent OPEN
readers see the previous or candidate set, never a removed or partially added
set. A transaction that does not commit restores the previous declarations
through the same atomic publication.
<!-- source: pkg/plugin/rpc/types.go -- ConfigVerifyOutput.Capabilities -->
<!-- source: internal/component/plugin/server/config_tx_capabilities.go -- stageVerifiedCapabilities, installStagedCapabilities, restoreCapabilities -->
<!-- source: internal/component/plugin/server/reload_tx.go -- runTxCoordinator -->

In the Go SDK the
declaration is the output of the extractor a plugin registers with
`DeclareCapabilitiesFrom`, run on the Stage 2 configuration for Stage 3; a
runtime `configure` redelivery never reruns it. At config-verify the SDK runs
the same extractor on the candidate sections after the plugin's own verify
handler accepts, and the answer carries its whole output as `capabilities`, an
empty list when it declares nothing. An extractor error refuses the candidate.
For a BGP plugin each declaration's `Peers` names a peer by its name or its
remote IP address, or `group:<name>` for a dynamic group's template; the
reactor injects capabilities under the same set (`configjson.PeerSelectors`),
so a plugin may key its declaration by either, as `bgp-role` keys by the
address. `sdk.CandidateCapabilities` returns the declaration the answer
carries, for a plugin's own tests.
<!-- source: pkg/plugin/sdk/capability_declaration.go -- DeclareCapabilitiesFrom, handleStartupConfigure, composeCapabilityVerify, CandidateCapabilities -->
<!-- source: internal/component/bgp/configjson/traverse.go -- PeerSelectors -->
<!-- source: internal/component/bgp/configjson/capability_extractor.go -- BGPCapabilityExtractor -->
<!-- source: pkg/plugin/sdk/sdk.go -- Plugin.Run, composeCapabilityVerify call -->

**A list of two or more entries arrives with its order beside it.** A config
section body is a JSON object, and a YANG list inside it is an object keyed by
the list key. A JSON object states no order, so the order the operator wrote is
delivered as a sibling key: the list name with an `@` prefix, holding the entry
keys as a JSON array of strings. A YANG node name never starts with `@`, so the
sidecar can never collide with a declared sibling.

```json
{"bgp":{"policy":{"prefix-list":{"ORDERED":{
  "@entry":["10.0.0.0/8","0.0.0.0/0"],
  "entry":{"0.0.0.0/0":{"action":"accept"},
           "10.0.0.0/8":{"action":"reject"}}}}}}}
```

The sidecar sits beside the list it orders, at whatever depth that list is.
Here the operator declared one prefix-list, so `prefix-list` has one entry and
no sidecar, and `entry` has two, so `@entry` rides beside it inside the named
list. A second prefix-list would give `prefix-list` a sidecar of its own.

A list of one entry carries no sidecar, because one entry has one order. A
plugin that does not read the order ignores the key. A plugin that needs the
order reads it with `configorder.Entries` (`internal/core/configorder`), which
refuses a list of two or more entries that arrives with no order rather than
substituting a sorted one.
<!-- source: internal/core/configorder/configorder.go -- Entries, OrderKey -->
<!-- source: internal/component/config/tree.go -- Tree.ToPluginMap -->

**Timeout:** `stage-timeout` bounds a dependency tier's period without plugin
progress, not the total duration of a stage. A plugin's first completion
extends that window; duplicate completions do not. If the tier makes no
progress within the configured window, startup aborts for all its plugins.
With no explicit per-process `stage-timeout`, the configure barrier allows the
larger of the environment's positive stage timeout and the largest accepted
`apply-budget` in that dependency tier. Budgets are integer seconds, capped at
600 before duration conversion. Registration completion freezes this maximum;
duplicate declarations cannot extend it. Other barriers retain their ordinary
timeouts, and a nonpositive environment timeout still disables the stall check.
The startup context can end any wait earlier.

A wire inactivity watcher leaves a sibling's failure to declaration admission
and the tier barrier. The plugin's own declaration and process end decide
whether it failed. The watcher retains its inactivity bound while that
declaration arrives.
<!-- source: internal/component/plugin/startup_coordinator.go -- WaitForStageProgress, WaitForStageInactivity, StageComplete -->
<!-- source: internal/component/plugin/startup_budget.go -- DeclareConfigureBudget, ConfigureStall -->
<!-- source: internal/component/plugin/server/startup_budget.go -- declaredConfigureBudget, startupStageStall -->

**Post-Startup Callback.** Stages 1-5 run per-phase. Plugins load across up to
five phases (config-path auto-load, explicit, family, event-type, send-type) in
serial order, so the dispatcher command registry only contains every plugin's
commands after the last phase completes. The engine sends the `post-startup`
callback to every running plugin after `signalStartupComplete` has frozen both
the plugin registry and the dispatcher command registry; at that point cross-
plugin `DispatchCommand` is guaranteed to resolve. Plugins register a handler
via the SDK `OnAllPluginsReady` method. Delivery is best-effort (fire-and-
forget, bounded timeout, per-plugin goroutine), and its outcome is recorded on
the plugin's process for the `plugin-poststartup` doctor check (`ze doctor`
reports a running plugin whose callback failed), so a single slow or broken
plugin cannot delay notification to the rest.
<!-- source: internal/component/plugin/server/startup.go — sendPostStartupToAll -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go — OnAllPluginsReady -->

**Why Barriers:**
- Ensures all plugins register commands before any receive config
- Ensures all capabilities declared before registry shared
- Prevents race conditions in multi-plugin configurations
- Guarantees consistent state before BGP peers start

**Shared stage-driver.** Both startup callers drive these five stages through a
single implementation, `runStartupHandshake`, rather than each hard-coding the
wire sequence. The driver owns the wire choreography -- reading each
plugin-initiated request, validating its method string, responding, and sending
the two engine-initiated callbacks -- while the caller-specific effects between
stages are injected through a `startupSink`. The engine's `engineStartupSink`
performs the full registration set (registry, families, capabilities, commands,
subscriptions, bridge dispatch, reactor signaling) and synchronizes each tier
through the `StartupCoordinator` barrier; the hub's `hubStartupSink` harvests the
plugin's declared commands and schema, delivers nil config and nil registry, and
runs with no barrier. A protocol change (a new method string, a reordered stage,
an added validation) therefore touches one place.
<!-- source: internal/component/plugin/server/startup_driver.go -- runStartupHandshake, startupSink -->
<!-- source: internal/component/plugin/server/startup.go -- engineStartupSink -->
<!-- source: internal/component/plugin/server/subsystem.go -- hubStartupSink -->

A hub subsystem may carry configured command names for pre-registration. Its
Stage-1 declaration replaces that provisional list, including an empty
declaration, so a command the child no longer declares is no longer routed to it.
<!-- source: internal/component/plugin/server/subsystem.go -- NewSubsystemHandler, hubStartupSink.onRegistration, findHandlerByCommand, SubsystemHandler.matchCommand -->

The driver retains the connection it captured at handshake entry through the
final ready acknowledgement, including bridge publication. A concurrent
`Process.Stop` closes that transport and clears the process's connection field;
publication must not look it up again. A stopped transport still fails the final
response, so the engine rolls startup back rather than treating cancellation as
successful readiness.
<!-- source: internal/component/plugin/server/startup_driver.go -- runStartupHandshake -->
<!-- source: internal/component/plugin/server/startup.go -- engineStartupSink.publishTransport, handleProcessStartupRPC -->

**Filter Declaration (Stage 1):**

Plugins may include a `filters` list in their `declare-registration` to offer named
route filters. Each entry declares a filter name or a name pattern, direction
(import/export/both), the route fields it reads and sets, failure mode, and
optional overrides of default filters.

| Field | Type | Description |
|-------|------|-------------|
| `filters[].name` | string | Filter name (referenced in config as `<plugin>:<name>`) |
| `filters[].name-pattern` | string | RE2 expression a whole filter name matches, in place of `name`, for names made at run time (one per AS number) |
| `filters[].direction` | enum | `import`, `export`, or `both` |
| `filters[].reads` | list | Route fields of the policy vocabulary the filter reads (`prefix`, `as-path`, `community`, ...), sent in `filter-update`'s `fields`; `raw` asks for the raw UPDATE body in `filter-update`'s `raw`. Stage 1 refuses a BGP field whose value formatter this binary does not link (a community field without `bgp-filter-community`), naming the field and the plugin |
| `filters[].sets` | list | Route fields of the policy vocabulary the filter may set |
| `filters[].on-error` | enum | `reject` (fail-closed) or `accept` (fail-open) |
| `filters[].overrides` | list | Default filters this filter replaces (e.g., `rfc:no-self-as`) |
| `filters[].readvertise` | bool | The export filter runs on the stale readvertise rail (default false): when a graceful-restart retention re-advertises its stale routes, the destination's export chain runs only the members whose filter declares it, with the route's `stale` metadata set, and a route reaching the end of those members is sent. A member whose plugin no longer declares its filter runs there and fails the route closed. Its answer MUST be safe to repeat on bytes it already judged, because the rail's body is what Ze already sent. Stage 1 refuses it on an import-only filter |
| `filters[].judges-final` | bool | The export filter runs once more over any body the export chain rewrote (default false), with `raw` holding the route as the chain received it and every other field the rewritten body, so its answer MUST be safe to repeat on bytes it already judged. Its answer holds only against a member that does not declare it: the pass runs the declaring members in chain order, so a later one can undo an earlier one. A raw answer is refused when the pass runs over a body that differs from the chain's input. On the stale readvertise rail a member must also declare `readvertise` to run in the pass, except after a change of the export policy, and the rail skips the pass when its one member is the chain's only declaring member. Stage 1 refuses it on an import-only filter |
<!-- source: internal/component/bgp/policychain/chain.go -- readvertiseRuns, Readvertises, judgesFinalRuns, HoldsFinalJudge, RailEndsOnFinalJudge -->
<!-- source: internal/component/bgp/reactor/filter_library.go -- runFinalJudges -->
<!-- source: internal/component/bgp/reactor/stale_readvertise_chain.go -- runStaleChain -->

Stage 1 refuses an entry carrying both `name` and `name-pattern`, or neither,
the text era's wildcard name `*` (declare `"name-pattern": ".+"` instead), a
pattern that does not compile, a read or set the policy vocabulary does not
hold, and `raw` declared as a set. An exact name wins over a pattern that also
covers it. A build that links no subsystem running filters (no `ze_bgp`)
refuses any plugin declaring one.

The `<plugin>` of a reference names the plugin's registration identity, never
the operator's endpoint alias. `bus { endpoint gr { internal bgp-gr; } }` runs
`bgp-gr` under endpoint name `gr` (in-daemon or with `process separate;`), and
its filters are `bgp-gr:<filter>`, the name the plugin itself declares its
insertions under, because a plugin cannot know what an operator called its
endpoint. A foreign external program no registry row describes is named by its
endpoint (`bus { endpoint my-filter { external { run "/usr/local/bin/x"; } } }`
serves `my-filter:<filter>`). Stage 1 judges the filters, the insertions and
the loaded config's references under that name, and at run time a member
reaches the process running the plugin whatever its endpoint is called. A
second endpoint running the same plugin, when the plugin declares filters or
insertions, is refused at its Stage 1 naming both endpoints: a reference could
not say which one judges the route.
<!-- source: internal/component/plugin/process/process.go -- Process.RegistrationName -->
<!-- source: internal/component/plugin/process/manager.go -- ProcessManager.ProcessServing, ProcessManager.BlocksServing, AmbiguousRegistration -->
<!-- source: internal/component/plugin/server/startup_driver.go -- runStartupHandshake -->
<!-- source: internal/component/plugin/server/server.go -- refuseAmbiguousRegistration, CallFilterUpdate -->

#### Filter Insertion

A plugin that owns a feature whose config implies a procedure (an RFC 9234
role, say) declares where its filter goes, in `filter-insertions`; config load
then inserts the member into the import or export list of every peer whose
resolved config (bgp, group and peer merged) meets the condition, ahead of the
operator's members. No engine or config code names the feature, the plugin or
the path (`ai/META-DECISIONS.md`, "Goal and scope").
<!-- source: internal/component/plugin/registry/filter_insertion.go -- CheckFilterInsertions -->
<!-- source: internal/component/plugin/registry/session_fact.go -- RegisterSessionFact -->
<!-- source: internal/component/config/schema_origin.go -- CheckFilterInsertionPaths, SchemaNodeModule -->

| Field | Type | Meaning |
|-------|------|---------|
| `filter-insertions[].filter` | string | A reference the plugin serves: `<plugin>:<filter>` of a filter it declares, or `<type>:<instance>` of a filter type it owns |
| `filter-insertions[].direction` | enum | `import` or `export`, exactly one |
| `filter-insertions[].priority` | number | 0 to 9999; inserted members run in ascending priority, then reference |
| `filter-insertions[].config` | list | The words of the path the condition reads under the peer's resolved entry (`["role"]`) |
| `filter-insertions[].values` | list | When given, the leaf must equal one of them; absent means the path is present |
| `filter-insertions[].session` | list | A session condition, in place of `config`: every listed fact holds for the peer |
| `filter-insertions[].session[].fact` | string | A session fact some owner registered: the BGP view registers `peer-name`, `peer-group`, `peer-address`, `local-as`, `peer-as`, `route-server-as`, `ibgp` and `rs-client` |
| `filter-insertions[].session[].values` | list | When given, the fact's text form must equal one of them (a flag `true` or `false`, an AS number in decimal, an address in its canonical form); absent means the peer has the fact |
| `filter-insertions[].scope` | enum | Absent: the condition is judged on each peer, and the member lands on that peer's list. `any-peer`: while any peer meets the config condition, the member lands on every peer's export list |
| `filter-insertions[].required` | bool | The operator cannot name, move or deactivate the member |
| `filter-insertions[].reason` | string | The text the refusal prints; owed when `required` is true |

An insertion carries exactly one condition, `config` or `session`. A session
condition binds a member by the kind of session rather than by a feature's
config: `[{"fact": "ibgp", "values": ["false"]}, {"fact": "rs-client",
"values": ["false"]}]` reaches every bilateral eBGP session, the peers RFC 7999
Section 3.1 obliges an agreement with. The facts are those of the peer itself,
known from its config: a fact known only per route (the `source-*` facts of an
exported route) or per negotiation (`llgr-advertised`) is not registered as one.
A static peer is judged at config load. A dynamic group's template states no
remote AS, so the member is placed in the template and each peer of the group
keeps or drops it when its OPEN gives its AS, before its chain compiles.

A source-keyed procedure declares `scope any-peer`: it acts, on every
peer's export, on the routes a peer meeting the condition sent, so the
condition is judged on any peer (static, or a dynamic group's template) and
the member goes on every peer's export list while one peer meets it. LLGR's
egress is the case: RFC 9494 Section 4.3 obliges a speaker that advertised
the capability to a neighbor for the routes from that neighbor, wherever it
sends them. The member itself tells those routes apart by their source.
The role plugin's OTC egress member uses the same scope: while any peer
declares a role, every destination runs its source export-set decision,
including a destination whose own config declares no role.
<!-- source: internal/component/bgp/plugins/role/otc_filter.go -- otcInsertions -->
<!-- source: internal/component/bgp/config/filter_insertion.go -- anyPeerHolds -->
<!-- source: internal/component/bgp/policyview/session.go -- SessionConditionHolds -->
<!-- source: internal/component/bgp/config/templated_chain.go -- keepHolding -->

Stage 1 refuses an insertion naming a reference the plugin does not serve, a
direction other than `import` or `export`, a priority outside 0 to 9999, no
condition or both, a scope other than `any-peer`, `any-peer` on an import
insertion or beside a session condition (a dynamic peer's facts arrive with
its OPEN, so no load can judge them on any peer), an empty or holed path, a session fact no owner registered
or named twice, `values` beside a session condition, a required insertion with
no reason, and the same reference twice for one direction. It also refuses a condition path that is
not a node of the compiled schema under `bgp peer` (an external plugin's own
YANG is not compiled, `docs/plugin-development/schema.md`), and, for a plugin
compiled into ze, a path naming a node another module defines: a
compiled-in plugin conditions only on config its own YANG module adds, an
augment of another module's node included (`bgp-gr`'s
`session capability graceful-restart long-lived family`), so its member
lands only where config it owns is set. A third-party
plugin, started by the operator's `plugin external` entry, may condition on any
compiled path. Config load judges a compiled-in plugin's insertions by the same
rules, and places them only in a daemon that runs that plugin, configured or
auto-loaded because its config is set: the plugin that owns the procedure is
its scope, and the condition then picks the peers, so a session-condition
member needs no config of its plugin on the peer. Where no plugin server runs
(an offline `ze config validate`), nothing says which plugins load, and every
compiled-in plugin's insertions are placed, so a refusal a daemon could meet
is reported there too.
<!-- source: internal/component/bgp/config/filter_insertion.go -- declaredInsertions -->
<!-- source: internal/component/plugin/registry/filter_declaration.go -- LoadedPlugins --> An operator who names a non-required member in
the list places it there, and `inactive:` on it runs the session without it,
as for a loop-detection default; naming a required member in any form is
refused at verify with the plugin's reason, naming the peer, the list and the
reference.

A third-party plugin's insertions apply like a compiled-in plugin's. Its
Stage 1 declaration is the only record of them: the engine stores them with
the process's registration, and config load reads them, for every running
process no compiled registration describes, beside the compiled
registrations' own. The daemon builds its peers from the config before any
plugin reaches Stage 1, and a compiled-in plugin's insertions apply only
once it runs, so when a running plugin declared an insertion, the BGP
subsystem rebuilds its peers from the running config once every
plugin finished startup and before the first peer starts; a peer whose
filter lists did not change is left as it is. A refusal there (an operator
naming a required member, say) starts no peer and logs the reason. A later
reload reads the same insertions, so it keeps the members in place.
<!-- source: internal/component/plugin/server/server.go -- thirdPartyFilterInsertions, loadedPluginNames -->
<!-- source: internal/component/bgp/plugin/register.go -- runBGPEngine -->
<!-- source: internal/component/bgp/plugin/initial_peers.go -- initialPeerConfig, resolve -->
<!-- source: internal/component/bgp/config/filter_insertion.go -- declaredInsertions -->
<!-- source: internal/component/bgp/reactor/reload_tree.go -- RebuildRunningPeers -->

The members follow the declarations after startup too. A plugin that is
started again (its failure policy, or a reload repairing a broken plugin)
and declares other insertions at its new Stage 1, and a plugin a reload
stops because its config went, each change what config load places, so the
plugin server tells the BGP subsystem, which rebuilds its peers from the
running config the same way: a member the plugin no longer declares leaves
the peers, and a new one joins. The rebuild never runs inside a config
transaction. A change met while one is in flight waits for it to end and
then runs from the config it left running, and otherwise it holds the
transaction lock itself, so a commit arriving meanwhile is refused as busy
and a SIGHUP reload waits for it.
The same change listener compares Claims and Dissemination too: changing a
role can change peer ownership even when no insertion changes. A restart that
declares the same three values rebuilds nothing. The rebuild
is no apply: an operator's live `subscribe` stays.
<!-- source: internal/component/plugin/server/filter_insertion_change.go -- announcePeerDeclarationsChanged, announcePeerDeclarationChange -->
<!-- source: internal/component/plugin/server/reload.go -- deferPeerDeclarationChange, release -->
<!-- source: internal/component/plugin/server/startup_autoload.go -- stopCollectedProcesses -->
<!-- source: internal/component/plugin/registry/filter_insertion_change.go -- SetPeerDeclarationsChanged -->
<!-- source: internal/component/bgp/plugin/register.go -- SetPeerDeclarationsChanged -->

The declared reads and sets are what config load judges. A chain at `bgp
import` or `bgp export` naming `<plugin>:<filter>` fails when no declared name
or pattern covers the filter (the refusal lists what the plugin declares), when
the filter reads a field that attachment does not carry, or when it sets one
that attachment does not allow: the same check a named policy instance gets.
The same name, direction, read and set guards apply when a plugin inserts the
reference instead of the operator naming it. A dynamic template is checked
before acceptance too, including members whose session condition is resolved
only when a peer's OPEN arrives; concrete template names are checked again at
resolution.
<!-- source: internal/component/bgp/config/filter_insertion.go -- insertForPeer -->
The daemon loads its config before an external plugin reaches Stage 1, so at
startup a reference is judged when its plugin declares. A refusal then fails
that plugin's Stage 1 and fails daemon startup with it, whatever the plugin's
failure policy: the config cannot be applied as written, so ze exits naming the
reference and the fix, as a reload refuses the same chain at load. A
declaration refused on its own (an unknown field, `raw` as a set) holds no
loaded chain, and the plugin's failure policy decides what ze does. A reload
judges a reference at once when its plugin already runs. When the reload itself
starts the plugin (its config root is new), the reference is judged at that
plugin's Stage 1, and a refusal refuses the whole reload before anything is
applied: the running config stays in force, ze keeps running, and the error
names the reference and the fix.

A reference to a plugin compiled into ze needs no Stage 1 to be judged when
that plugin serves no filter at all: no `Filters` and no `FilterTypes` on its
`registry.Registration`, and no in-process filter with a policy type. Config
load refuses it at once, `ze config validate` included, naming the reference,
the plugin and the `match` instance that reaches its lists. A compiled-in
plugin that declares filters (`bgp-filter-irr` carries its Stage 1 declaration
in `Registration.Filters`) is judged at its Stage 1 as above.
<!-- source: internal/component/plugin/registry/filter_declaration.go -- CompileFilterDeclarations, CheckFilterDeclarations, ErrLoadedChainRefused -->
<!-- source: internal/component/plugin/server/startup.go -- refuseStartupOnLoadedChain -->
<!-- source: internal/component/plugin/server/startup_autoload.go -- reloadChainRefusal -->
<!-- source: internal/component/bgp/config/external_filters.go -- checkExternalChainRef, servesNoFilter, refuseFilterlessPlugin, checkFilterDeclarations -->
<!-- source: internal/component/policy/declared_fields.go -- CheckDeclaredFilter -->

Config references filters as `<plugin>:<filter>` in nested direction policies:
`filter { import { filters [ name ]; final accept; } export { final reject; } }`.
Each configured direction requires `final accept` or `final reject`. References
accumulate global, group, then peer; the most-specific direction supplies the
final. An absent direction keeps automatic filters but has no operator policy.
It rejects on EBGP under RFC 8212 Section 3. On IBGP, import and explicit local
or role output keep their accepting default; ordinary learned export requires
operator policy.
<!-- source: internal/component/bgp/filterapi/policy.go -- Policy.AcceptsAtEnd, Policy.AcceptsExportAtEnd -->

The runtime uses the declaration to choose raw payload delivery, validate modify
deltas against declared attributes, and pick fail-open or fail-closed behavior
on filter RPC errors. Accept continues to the next filter. Exhaustion applies
the policy final, including empty and inactive-only chains. A raw full-UPDATE
rewrite stops later filters and still receives the final; on export, the
auto-appended trailing `next-hop:auto` step still runs on any next-hop field
(attr 3 `NEXT_HOP` or `MP_REACH_NLRI`) whose emitted bytes match the received
UPDATE when no earlier chain member already selected `unchanged`, `self`, or an
explicit address (and skips any field whose next-hop bytes the raw override
altered). Earlier reject,
fail-closed error and teardown remain terminal. A final-only export change
reconciles the affected route kinds on the current session. The ordinary owner
reoffers current learned selections. If the local decision changes, it also
reoffers current local selections. Event settlement joins their final output
receipts. Admitted origins keep their receipts through rejection and can return
when the policy accepts them again. A learned-only transition leaves local
output in place. Acceptance also regenerates configured local routes. A direct
one-shot API producer must reannounce if rejection removed its output.
<!-- source: internal/component/bgp/reactor/policy_reload.go -- reconcileExportFinalKinds -->
<!-- source: internal/component/bgp/reactor/policy_final_kinds.go -- reconcileSelectedExport -->

Explicit RS/RR forwarding and authorized Adj-RIB-In self-replay preserve the
legacy restoration path. Before non-FlowSpec replay, the receive store must apply
every UPDATE event already queued for it. FlowSpec restoration reads currently
authorized selecting-RIB paths instead, through the actual owner's send grant.
Runtime claims take precedence over compiled claims, including aliased plugins.
API withdrawal sections remain effective under final rejection. Arbitrary
filter-definition changes do not generally re-evaluate existing routes.
<!-- source: internal/component/bgp/reactor/policy_reload.go -- policyReplayOwner, reconcileExportFinalKinds, drainReceiveStore -->
<!-- source: internal/component/bgp/reactor/policy_final_kinds.go -- reconcileSelectedExport -->
<!-- source: internal/component/bgp/server/export_policy_event.go -- OnPeerExportPolicy -->

Operational `PeerInfo` carries `ImportPolicy` and `ExportPolicy` as optional
`PolicyInfo` values rather than bare reference lists, and no longer carries
separate `NextHopMode` or `NextHopAddress` fields (`session { next-hop }` is
removed; the effective `ExportPolicy` carries `next-hop:auto`, `next-hop:self`,
or `next-hop:unchanged`). Each carries `filters`
(canonical reference strings) and an optional `final` (`accept` or `reject`).
A configured policy, empty or not, reports its configured final. A direction
with no operator policy reports the final the session applies: `reject` with
`final-source` `rfc8212` on EBGP (RFC 8212 Section 3), no `final` on IBGP, and
`undetermined` on a dynamic peer whose remote AS is not resolved yet. An IBGP
direction with no policy and no automatic references is omitted.

`show policy chain peer <selector> [import|export]` nests `filters` (name/canonical objects) and `final` under
`import` and `export`; peer detail nests canonical strings and final under
`import-policy` and `export-policy`. These are whole effective policies, not
only the references declared directly on the peer. Dry-run reports the same
final outcome as runtime for its simulated class, including terminal raw
rewrites. The export-only `route-kind` argument takes `learned` or `local`;
omission simulates `learned`. It does not authorize live output.
`simulated-route-kind` names that class in the result. A resolved explicit
dissemination role retains its existing default for either class, reported
with `role-default: true`. Import refuses the argument, and an unknown value
is refused. An internal ordinary learned rejection has
`policy.final-source: learned-export-policy`, distinct from RFC 8212's
external-session rejection.
The command handler passes the resolved peer's stable owner to the dry-run
adapter. A missing or removed owner is refused; a same-address sibling cannot
supply its policy, session facts or role in the requested peer's place.
With `final reject`, dry-run preserves explicit withdrawals: a pure withdrawal
reports `accept`, while a mixed UPDATE reports `modify` and renders only the
surviving withdrawal sections. Execution failures and teardown remain `reject`
with no resulting text.
An unresolved remote AS instead produces `action: "undetermined"` and a `reason`
to wait for establishment and repeat `show policy test`. It executes no filters,
even with an override, and returns no trace, input text or resulting route.
Known-session results omit `reason` unless an explicit role's absent-policy
default applies. Every dry-run result also carries `policy`,
the direction's status object above, so the per-UPDATE action and the
direction's final stay distinct.
<!-- source: internal/component/bgp/reactor/policy_dryrun.go -- PolicyDryRun -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- policyInfo, absentPolicyInfo -->

<!-- source: internal/component/plugin/types_bgp.go -- PolicyInfo, PeerInfo -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- policyInfo -->
<!-- source: internal/component/bgp/plugins/cmd/policy/handler.go -- handleShowPolicyChain, toPolicyChain -->
<!-- source: internal/component/bgp/reactor/policy_dryrun.go -- PolicyDryRun -->

<!-- source: pkg/plugin/rpc/types.go -- FilterDecl -->
<!-- source: internal/component/plugin/server/startup.go -- registrationFromRPC filters -->
<!-- source: internal/component/plugin/server/server.go -- FilterReadsRaw and FilterOnError -->

**Doctor Check Declaration (Stage 1):**

Plugins may include a `doctor-checks` list in their `declare-registration` to provide
runtime health checks. Each entry declares a check name, phase, ordering, and the
diagnostic codes it may emit. The engine stores declarations in `PluginRegistration`
and invokes them via the `doctor-check` callback when `show doctor` runs.

| Field | Type | Description |
|-------|------|-------------|
| `doctor-checks[].name` | string | Check name (kebab-case, 1-128 chars) |
| `doctor-checks[].phase` | enum | `pre-config`, `missing-config`, `parsed-config`, `post-config`, or `runtime`. A `runtime` check reads the plugin's live state and runs only on the daemon route |
| `doctor-checks[].order` | int | Ordering within phase (0-9999, default 0) |
| `doctor-checks[].dependencies` | list | Other check names that must run first |
| `doctor-checks[].platforms` | list | Platform filter (empty defaults to `any`) |
| `doctor-checks[].codes` | list | Diagnostic codes (1-16, must start with `doctor-`) |

Offline `ze doctor` does not invoke plugin doctor checks (plugins are runtime
processes). Plugin checks cover runtime health; Go-registered checks cover
pre-start readiness.

The `doctor-check` answer (`DoctorCheckOutput`) lists `diagnostics`, each with
`code`, `severity` (`error`, `warning` or `info`), `message`, and `fix`: the
command or config statement the operator runs to clear the problem, alone and
with no label. `fix` is omitted when no command fixes the problem. The daemon
carries it unchanged into the `show doctor` answer, where the text report draws
it on a line of its own. An `error` makes the box not ready, a `warning` does
not, and an `info` row is a fact that is neither counted as a warning nor fails
readiness; any other severity is shown as a warning.
<!-- source: pkg/plugin/rpc/types.go -- DoctorCheckDiagnostic, DoctorPhaseRuntime -->
<!-- source: internal/component/doctor/cmd/show.go -- normalizeSeverity -->

<!-- source: pkg/plugin/rpc/types.go -- DoctorCheckDecl -->
<!-- source: internal/component/plugin/server/startup.go -- registrationFromRPC doctor checks, validateDoctorCheckDecls -->
<!-- source: internal/component/plugin/registration.go -- DoctorCheckRegistration -->

**Enricher Declaration (Stage 1):**

Plugins may include an `enrichers` list in their `declare-registration` to provide
show command enrichment. Each entry declares a command path and unique key. The
server registers a proxy enricher via `show.Register()` for each declaration. At
show time, the proxy serializes the base map, calls `enrich-show` with a 2s timeout,
and merges the response into the base map. Proxy enrichers are cleaned up via
`show.Unregister()` when the plugin process exits.

| Field | Type | Description |
|-------|------|-------------|
| `enrichers[].command` | string | Show command path (e.g., `show subscriber detail`), 1-128 octets, the bound the doctor record keeps a failing enricher's command at |
| `enrichers[].key` | string | Unique enricher key within command (kebab-case, 1-128 chars) |

<!-- source: pkg/plugin/rpc/types.go -- EnricherDecl -->
<!-- source: internal/component/plugin/server/enricher.go -- registerProxyEnrichers, validateEnricherDecls -->

**Ordering Declaration (Stage 1):**

A plugin CAN list `dependencies`, `optional-dependencies` and `start-after` in
its `declare-registration`. They are the ordering edges a compiled-in
plugin's registration carries (`Dependencies`, `OptionalDependencies`,
`StartAfter`): a `dependencies` entry must name a plugin this daemon runs or
Stage 1 is refused, and the other two order only and load nothing. A
dependency is met by a plugin a `bus { endpoint ... }` block configures, or by
one a startup phase loaded: auto-load resolves a compiled-in plugin's registry
dependencies and starts them in the same phase, so `bgp-gr` auto-loaded for
`bgp { }` finds the `bgp-rib` started beside it. An external program's
dependencies are resolved by nobody, so the operator configures them. The
refusal is a `*registry.MissingDependencyError` (`errors.Is`
`ErrMissingDependency`) whose fix names what to write: a
`bus { endpoint <name> { internal <name>; } }` block for a plugin this build
carries, a `bus { endpoint <name> { external { run ...; } } }` block for a name
no build carries, or the build tags for a plugin this build was compiled
without (`plugin-system.md`, "Dependencies").
<!-- source: internal/component/plugin/registry/missing_dependency.go -- UnloadedDependencyError -->
The engine keeps the
three with the process's registration. State events are delivered in reverse
dependency order, and a process that runs a foreign external program takes its
edges from this declaration alone, never from a compiled registration that
shares its endpoint name. `bgp-gr` declares `dependencies [bgp-rib, policy]`,
`optional-dependencies [bgp-rs]` and `start-after [bgp-rr]`, so a compiled-in
`bgp-gr` running with `process separate;` keeps its registration identity and
takes a DOWN, and emits its `bgp-gr/retention` verdict, before the route
server, the route reflector and the RIB take theirs. An edge's target matches
by registration identity as well as endpoint name, so the edge holds whether a
compiled-in plugin runs in-daemon or with `process separate;`.

| Field | Type | Description |
|-------|------|-------------|
| `dependencies` | list | Plugins this plugin needs and is ordered after |
| `optional-dependencies` | list | Plugins this plugin is ordered after when they run |
| `start-after` | list | Plugins this plugin starts after when they run |

<!-- source: internal/component/plugin/server/startup.go -- registrationFromRPC, onRegistration, runPluginPhaseWithin -->
<!-- source: internal/component/bgp/server/events.go -- sortByReverseDependencyTier -->

**Session-Ready Declaration (Stage 1):**

A plugin CAN set `signals-session-ready` in its `declare-registration` to state
that its routes belong to a peer's INITIAL routing update and that it dispatches
`request peer <addr> plugin session ready session <token>` once they are out,
with the token from the peer-up event. The End-of-RIB waits for no report (owner
ruling, 2026-09-18): Ze sends its marker once its own table is out. The report
does one thing: it lowers the peer's replay fence, which holds the peer's live
forwarded changes behind a plugin's peer-up replay
(`docs/architecture/plugin/plugin-system.md`, "Session-ready reports"). An external
plugin opts into the same hold with `fences-live-forwards: true`, alongside
`signals-session-ready: true`. Stage 1 refuses a fence with no ready report.
The declaration grants no receive or send permission.
<!-- source: internal/component/plugin/server/replay_fence.go -- validateReplayFenceDeclaration, declaresReplayFence -->

| Field | Type | Description |
|-------|------|-------------|
| `signals-session-ready` | bool | The plugin reports when its share of a peer's initial routing update is out |
| `fences-live-forwards` | bool | Hold live changes behind that replay until the captured session-ready report; requires `signals-session-ready` |
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput -->

Both readiness and fencing use the current authenticated Process. A compiled
registration with the same process name cannot override its Stage-1 flags.
Forked in-tree programs declare those flags too. Only an Internal runner can
use its compiled defaults.
<!-- source: internal/component/plugin/server/replay_fence.go -- sessionReplayDeclarations -->
<!-- source: internal/component/plugin/registry/registry.go -- SignalsSessionReady, FencesLiveForwards -->

The declaration is VOLUNTARY (owner directive, 2026-09-02) and it is the only
route an external plugin has to it: an external process appears in no
compile-time registry, so a plugin that leaves the field false owes no report.
Declaring is a claim about WHEN this plugin's routes belong, not about what it
may send.

Three facts have to hold before a peer credits the process's report, so
declaring alone does not name it. The compiled bus filter admits `bgp:update`
or `bgp:raw` from the endpoint toward the peer, the plugin declares
`signals-session-ready`, and the compiled bus filter admits `bgp:state` from
the peer to the endpoint. The last one is the peer telling the process its
session started, which is what the report answers: a process the peer never
tells cannot push into that session's initial update.

<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput.SignalsSessionReady -->
<!-- source: internal/component/plugin/server/events.go -- (*Server).declaresSessionReady -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- Peer.initialUpdateReporters -->

**Failure-Policy Declaration (Stage 1):**

A plugin CAN set `failure-policy` in its `declare-registration` to say what its
own failure means. The engine applies it when the plugin's process ends, and
when the plugin fails a startup stage after it has declared.

| Value | On plugin failure | May the plugin be restarted? |
|-------|-------------------|------------------------------|
| `restart` | The engine starts the plugin again | Yes |
| `ignore` | The engine logs and carries on without it | No |
| `fatal` | The daemon stops, and `ze` exits 1 | No |

The declaration is VOLUNTARY. A plugin that omits the field is read as `ignore`,
which is what the engine did before the field existed, so silence is never read
as consent to a restart. A value that is none of the three fails the whole
registration: the engine does not guess.

One value answers two questions, because the engine starts a plugin again for
exactly one reason. `restart` is also the plugin's statement that it MAY be
started again, and the other two are its statement that it must not be.

The `respawn` leaf of a `bus { endpoint <name> { external { ... } } }` block is
the operator's request inside that declaration. It can ask for less and never
for more. With no leaf the declaration decides. With `respawn false` a plugin
that would have been started again is left stopped. With `respawn true` against
a plugin that declares it must not be restarted, the daemon stops at startup
and the error names the plugin, the policy it declared and the leaf.

A stop a plugin causes, under `fatal` or through that disagreement, is a
failure, and `ze` exits 1 once the stop has completed, as it does for a refused
startup. Only a stop nothing failed in exits 0, so a service manager that
restarts on failure (systemd `Restart=on-failure`) and an operator's script
see the fatal stop for what it is. ze writes an error to stderr at the end of
the stop, naming the plugin and the reason. Its fix depends on the cause:

| Cause | Fix the error names |
|-------|---------------------|
| `fatal`, last-stop record written | `ze doctor`, which reads the plugin, the reason and the time from the record |
| `fatal`, the store refused the record | the ze log lines that name the plugin, because doctor cannot name this stop |
| The respawn disagreement | the config change the startup refusal names: remove the `respawn` leaf, or run a plugin that declares `restart` |

Only the first failure counts, and only before the stop begins. When two
plugins fail at once, the first is the cause, so the stderr error and the
record name the same plugin. Once a stop is under way (an accepted `request shutdown`, a signal, the drain), a fatal
plugin that exits is an effect of that stop, not its cause: it is logged, no
record is written, and the stop exits 0.
<!-- source: internal/component/plugin/server/server.go -- (*Server).StopFailure, stopUnderWay -->
<!-- source: internal/component/plugin/server/failure_policy.go -- (*Server).stopDaemonForPlugin, pluginStopFailure, respawnDisagreementFix -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig -->

A restart is bounded: `RespawnLimit` restarts in `RespawnWindow`, and
`MaxTotalRespawns` over the life of the daemon. Past either bound the plugin is
disabled and the daemon carries on, which is the `ignore` outcome.

Every path that leaves a configured plugin down records why and raises the
`doctor-plugin-stopped` warning, whose fix is `ze request plugin restart <name>`:

| Path | Cause the warning and the doctor row name |
|------|-------------------------------------------|
| The process ended and its policy is `ignore`, or it declared none | it exited and its failure policy does not start it again, with the policy and the exit status |
| The respawn limit disabled it | the respawn limit disabled it, with the limit |
| Its policy asked for a restart and the restart failed | it exited and could not be started again, with the error |
| Its startup handshake did not reach running, and ze runs without it | its startup handshake failed, with the error |

The warning clears when the plugin runs again, and when a reload removes it from
the configuration. `request plugin restart <name>` starts a configured plugin
again, stopping it first when it runs; the operator's order lifts a
respawn-limit disable atomically only after every pre-spawn refusal passes, and
the command refuses while plugin startup is still in progress (before
`startupComplete`, so it cannot race startup supervision; once startup finishes,
the restart succeeds), a name nothing configured, a restart while a config
transaction is in flight, a plugin asking for its own restart, and a plugin
whose removal a refused reload left pending. Every restart, whoever asks for
it, waits at most `restartCleanupGrace` (the daemon's 3-second stop budget) for
the stopped process's runtime cleanup, which runs only once its in-flight calls
into the engine have returned. A wait that expires fails the restart: the
replacement is stopped the way a failed handshake stops it, because the late
cleanup removes what is keyed by the plugin's name, and each later restart of
the name is refused until every recorded stopped process of that name has
finished its cleanup. Both the `doctor-plugin-stopped` report-bus warning
(`ze show warnings` and the daemon WARN log) and `ze show doctor`'s
`plugin-stopped` row render from one shared declaration
(`process.StoppedMessage` / `process.StoppedFix`, carrying
`PluginStop.CleanupOwed`): `Fix` stays `ze request plugin restart <name>`,
while the message carries the cleanup-owed refusal explanation and
`privilege.RestartRemedy()` last resort whenever cleanup is owed. Once every
recorded stopped process finishes its cleanup (`cleanupProcess` /
`lateCleanupOwed`), `ClearStoppedCleanupOwed` refreshes the bus warning to drop
the cleanup-owed notice while keeping the warning active until the plugin runs
again.

Each configured name has one restart controller through process replacement,
outgoing cleanup, startup, and result recording. An operator or rollback restart
refuses a name whose controller is still active, before changing its process or
stop record. Failure-policy restart acquires the config transaction lock first
and then waits for any earlier controller within the daemon lifetime (global
lock order `txLock` -> `restartControllers`), checking its exact outgoing
process and pending removals under both reservations before replacing anything.
A delayed failure of an older generation cannot replace
the current generation or remove its registrations.
<!-- source: internal/component/plugin/server/restart_controller.go -- restartControllerSet.tryAcquire, restartControllerSet.wait, restartController.release -->
<!-- source: internal/component/plugin/server/reload_tx.go -- restartPlugin, restartPluginAfterExit, restartPluginWithin -->
<!-- source: internal/component/plugin/server/register_plugin_restart.go -- pluginRestartFailure -->

Before an intentional restart stops its outgoing process, the manager withdraws
that exact generation from the current slot. Its shutdown is not classified as
an unexpected fatal exit. The admitted replacement call retains physical
ownership of the outgoing process through its actual join. If the join expires,
that owner remains responsible for terminal shutdown before providers close.
The replacement becomes current only after its own process start succeeds.
<!-- source: internal/component/plugin/process/manager.go -- Respawn -->

Every plugin restart (the operator's command, a reload rollback, and a
failure-policy restart) holds the config transaction lock, so each of its waits
has a bound. The replacement's
startup handshake gets five ordinary stage windows (25s by default). When no
per-process `stage-timeout` is explicit and the environment timeout is positive,
the configure window is the larger of the ordinary window and the outgoing
process's accepted, capped `apply-budget`. The daemon captures that allowance
before it replaces the process. The replacement's declaration cannot change its
own bound. The other four windows retain their ordinary duration.
<!-- source: internal/component/plugin/server/restart.go -- restartHandshakeBound, boundedRestartHandshake -->
<!-- source: internal/component/plugin/server/startup_budget.go -- previousStartupBudget, restartStartupHandshakeBound -->

When the outgoing accepted declaration includes `started-handler`, the handshake also
gets one initializer window: the accepted allowance, or the ordinary stage
window when that is larger. An explicit process timeout sets each window,
including that conditional initializer window. A missing or unaccepted
initializer declaration adds no initializer window. A bridge activation receipt
with no initializer uses the existing handshake scope. Nonpositive environment
timeouts retain their immediate-expiry behavior.
<!-- source: internal/component/plugin/server/startup_budget.go -- restartStartupHandshakeBound -->

The daemon's context can end the handshake earlier. A handshake that runs out of its bound fails
the restart the way a failed handshake does. The rollback that stops a
replacement waits for it for that process's own stop bound (2s for a forked
plugin on Linux, 1s for one in the daemon), then releases it. A forked
plugin still running then is killed by its exit supervisor, while an engine
in the daemon that does not return on its closed connection runs until it
returns. A failure-policy restart acquires the config transaction lock (`txLock.acquire`)
before its per-name restart controller so it cannot race a config reload or removal,
and bounds its replacement's handshake with `boundedRestartHandshake` (five stage
windows plus any accepted `apply-budget` and `started-handler` window, the same bound
`request plugin restart` uses). A replacement that does not reach running within that
bound is rolled back and recorded stopped (`doctor-plugin-stopped`), releasing `txLock`.
<!-- source: internal/component/plugin/server/restart.go -- restartHandshakeBound, boundedRestartHandshake, rollbackReplacement -->
<!-- source: internal/component/plugin/server/startup_budget.go -- previousStartupBudget, restartStartupHandshakeBound -->

A process exit clears the ordinary active warnings the plugin raised through
`report-issue`, after its admitted requests finish. A warning reserved by a
retained daemon resource stays owned by that original resource until its
recovery is confirmed. Historical error events remain in the report ring.
For an unexpected exit, `doctor-plugin-crashed` (an error, fix `ze show errors`)
then records its status and the policy ze applies.
<!-- source: internal/component/plugin/server/dispatch.go -- handleSingleProcessCommandsRPC, cleanupProcess -->
<!-- source: internal/core/report/report.go -- ClearSource, RaiseError, store.clearSource -->
<!-- source: internal/component/plugin/process/manager.go -- ReportCrash -->
<!-- source: internal/component/plugin/server/failure_policy.go -- superviseProcess, applyFailurePolicy -->

A stop ends every restart. From the start of the stop drain (`BeginDrain`), the
manager refuses each respawn with `respawn refused: ze is stopping`, so a plugin
that exits during the drain or teardown stays down: the daemon is going away and
a new generation would only be torn down again.

`fatal` is open to any plugin, whether ze ships it or an operator wrote it
(owner directive, 2026-09-06). Configuring a plugin is accepting its terms.

A failure during a reload means the plugin failed. A startup handshake that ended
because the reload's context did, a reload over its budget or abandoned by its
client, refuses that reload and leaves the daemon running, whatever the plugin
declared. So does a sibling's failure: a plugin aborted at the tier barrier while
it waited, still connected, has not failed. The same holds when the sibling
failed after the plugin's registration was answered and before the plugin reached
the barrier: the plugin's configure allowance meets the failed tier, and the
sibling's failure is not recorded as the plugin's own. A `fatal` plugin whose
connection had already ended when that wait, or that declaration, ended has
failed, and stops the daemon.

Boot has no running state to keep. A `fatal` plugin that does not reach running
stops ze in every startup phase, however its handshake ended, because running
without it is what the plugin declared must not happen. When a sibling's failure
aborted it, the error log line names the plugin and the sibling's failure.

<!-- source: pkg/plugin/rpc/enums.go -- FailurePolicy -->
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput.FailurePolicy -->
<!-- source: internal/component/plugin/server/failure_policy.go -- pluginFailurePolicy, refuseRespawnDisagreement, (*Server).applyFailurePolicy, (*Server).stopDaemonOnStartupFailure, reloadOwnsStartup, startupEndedOutsidePlugin -->
<!-- source: internal/component/plugin/server/startup_failure.go -- tierFailureCause, withTierCause -->
<!-- source: internal/component/plugin/server/startup.go -- (*Server).stageTransition -->
<!-- source: internal/component/plugin/startup_budget.go -- ErrStartupTierFailed, DeclareConfigureBudget -->
<!-- source: internal/component/plugin/process/process.go -- (*Process).PluginEnded -->
<!-- source: internal/component/plugin/process/manager.go -- RespawnLimit, RespawnWindow, MaxTotalRespawns, RefuseRespawns, ErrDaemonStopping, ReportCrash, RemoveProcess -->
<!-- source: internal/component/plugin/process/stopped.go -- StopCause, RecordStopped, ClearStopped, SetStoppedCleanupOwed, ClearStoppedCleanupOwed, StoppedMessage, StoppedFix, DaemonRestartLastResort, RestartCommand, Reenable -->
<!-- source: internal/component/plugin/server/reload_tx.go -- restartPlugin -->
<!-- source: internal/component/plugin/server/register_plugin_restart.go -- handlePluginRestart, pluginRestartRefusal -->
<!-- source: internal/component/plugin/server/restart.go -- restartCleanupGrace, awaitReplacedCleanup, lateCleanupOwed -->

**Pipe Alias Declaration (Stage 1):**

A plugin CAN include a `pipes` list in its `declare-registration` to name a CLI
pipe alias for one of its own commands. An alias is the word an operator types
after the pipe character, and it stands for an operator chain. The daemon
resolves it, because the daemon runs the chain.

| Field | Type | Description |
|-------|------|-------------|
| `pipes[].command` | string | Command path the alias sits on. MUST be one of this plugin's own declared commands |
| `pipes[].name` | string | The word an operator types after the pipe character (kebab-case, 1-64 chars) |
| `pipes[].description` | string | The line completion and `command help` show beside the name |
| `pipes[].expansion` | string | An alias: the operator chain the name stands for, as an operator would type it |
| `pipes[].argument` | string | An operation: `none`, `optional`, `text`, `count`, `fields` or `path`, the words the name takes |

An entry states exactly one of `expansion` and `argument`, and
`validatePipeDecls` refuses one with both or neither by name, before any
conversion, beside the shape and the ownership. An entry with `argument` is a
pipe OPERATION: `registerPluginPipes` records it into the command's own
operation set (`command.RegisterPluginPipeFilters`, the same registry an
in-tree command's `RegisterPipeFilters` fills) and the engine sends it to the
handler in the `pipe` list of `execute-command` (below). The aliases then go
into the alias registry, under `startupRegistrationMu`, between the registry
row and the runtime families. One bad entry refuses the whole list and fails
the plugin's startup, with a message naming the plugin, the command path and
the name, and an alias refused after the operations were recorded withdraws
them. `UnregisterPluginPipeFilters` and `UnregisterPluginAliases` take the
declaration back when the plugin stops or its startup is rolled back, which is
what lets a stopped plugin start again.
<!-- source: internal/component/command/pipe_filter_plugin.go -- RegisterPluginPipeFilters, UnregisterPluginPipeFilters -->

A pipe alias SELECTS and re-sequences an answer. It renames nothing, sums
nothing and counts nothing, so a command that wants one MUST emit the aggregate
fields beside the detail rows. The full contract, both collision rules and the
derived inheritance barrier are in `docs/architecture/api/commands.md`.

Three holders refuse a declared alias name: a built-in pipe operator that
carries the name, a pipe filter on an OVERLAPPING command path that carries it,
and an alias on the EXACT same command path that carries it. The two populations
differ because the two resolution rules differ: a filter wins its whole subtree,
and a longer alias path deliberately shadows a shorter one.

A declared alias resolves over the SSH exec channel and in the daemon-hosted
interactive session, because the daemon expands the chain. It does not resolve
in `ze cli` with no command argument. That process runs its own copy of the
interactive model and expands the chain before it sends anything, and no plugin
alias is registered there, so an operator reads
`pipe error: unknown pipe operator: <name>` and Tab offers the name nowhere.
`cliClient.StreamMonitor` has the same gap. The repair is a channel that carries
the daemon's alias table to the client, and it is not built.
<!-- source: internal/component/cli/model_mode.go -- executeOperationalCommand -->
<!-- source: internal/component/command/alias.go -- RegisterPluginAliases -->

<!-- source: pkg/plugin/rpc/types.go -- PipeDecl -->
<!-- source: pkg/plugin/sdk/sdk_types.go -- PipeDecl -->
<!-- source: internal/component/plugin/server/startup.go -- validatePipeDecls, registerPluginPipes -->
<!-- source: internal/component/command/alias.go -- RegisterPluginAliases, UnregisterPluginAliases -->

**Help Text Declaration (Stage 1):**

Each entry of the `commands` list carries the command's two help texts. They are
two declarations, and neither is derived from the other.

| Field | Type | Description |
|-------|------|-------------|
| `commands[].description` | string | The one-line SUMMARY. Every surface that shows the command on one line reads it: a completion candidate, a list row, a table cell. Maximum 256 bytes, no control character |
| `commands[].description` | string | The LONG explanation the command's own help page prints, under the summary. Maximum 4096 bytes, newlines kept, every other control character refused |

The key is `description` and NOT `help`, because `help` already names the summary
in a `Completion` row on this same boundary. One spelling, one meaning.

An absent `description` is what every plugin written before the key existed
sends. It renders as summary-present and explanation-absent, and it MUST NOT
render as a blank summary. The engine carries the two texts in two maps
(`PluginRegistration.CommandShortHelp`, `PluginRegistration.CommandDescription`)
and writes them into `RegisteredCommand.ShortHelp` and
`RegisteredCommand.Description`. `VisibleCommandEntries` then hands both to
`command.MergeCommandPaths`, which fills each field of the command tree on its
own. This package and the command package both spell them `ShortHelp` and
`Description`.

`validateHelpDecls` reads both texts where `validateShapeDecls` reads the
shapes, before any conversion. The summary is written into the tab-separated
shell-completion format and into the one-line terminal candidate, so a newline,
a tab or an ESC in it breaks the format for every row that follows and writes an
ANSI sequence to the operator's terminal. The explanation is a paragraph only
the command's own help page prints, so it keeps its newlines. The alias
`description` above is held to the same one-line rule.

`command help "<name>"` answers with both, under `short-help` and `description`,
for a builtin and for a plugin command alike. A builtin's two texts come from
its YANG node: `PathToDescription` and `PathToHelp` build the two maps
`loadBuiltinsWithAliases` registers them from.

<!-- source: pkg/plugin/rpc/types.go -- CommandDecl -->
<!-- source: internal/component/plugin/server/startup.go -- validateHelpDecls, validateDeclaredText, registrationFromRPC -->
<!-- source: internal/component/plugin/server/command_registry.go -- RegisteredCommand, VisibleCommandEntries -->
<!-- source: internal/component/config/yang/command.go -- PathToDescription, PathToHelp -->
<!-- source: internal/plugins/meta/cmd/help.go -- commandHelp, commandHelpText -->

**Answer Shape Declaration (Stage 1):**

Each entry of the `commands` list carries three optional fields that say what
the command's ANSWER holds. An absent field is an undeclared field, so a plugin
that sends none keeps the behavior it had before these fields existed.

| Field | Type | Description |
|-------|------|-------------|
| `commands[].shape` | string | `doc` for one document or one value, `map` for rows that carry their own keys, `tab` for rows read against column names, `blob` for text or bytes read as they are |
| `commands[].columns` | []string | The answer's keys, lowercase kebab-case, in the order a person reads them. Needs a shape that has rows. Maximum 64, each name 1 to 64 bytes |
| `commands[].address-fields` | []string | The keys whose value holds an IP address or a prefix. Needs a shape. Maximum 16, each name 1 to 64 bytes |

`validateShapeDecls` reads the three fields before any conversion. It refuses
four declarations:

- an unknown spelling.
- a field list with no shape.
- a declaration on a blank command path.
- a list or a name past its bound.

One bad entry refuses the whole list and fails the plugin's startup. The message
names the command and the offending value, clamped to 64 bytes so a plugin
cannot write an unbounded string into the daemon log.

`registerPluginShapes` then writes the accepted set into the shape, column and
address-field registries under `startupRegistrationMu`, beside the alias write.
`UnregisterPluginShapes` takes the declaration back when the plugin stops or its
startup is rolled back. A path that an in-core package declared EMPTY returns to
that empty declaration rather than to nothing.

A declared shape decides which pipe operators the command publishes, and which
it refuses by name before dispatch. A declared address-field list admits
`| resolve` and `| origin` and limits both transforms to those fields. The full
contract is in `docs/architecture/api/commands.md`.

<!-- source: pkg/plugin/rpc/types.go -- CommandDecl -->
<!-- source: internal/component/plugin/server/startup.go -- validateShapeDecls, validateDeclaredFieldName, clampDeclared, registerPluginShapes -->
<!-- source: internal/component/command/answer_shape.go -- PluginShapesFromDecls, RegisterPluginShapes, UnregisterPluginShapes -->
<!-- source: internal/component/command/pipe.go -- validateDeclaredShape -->

### Query Mode: Stage 1 and No Stage 2

A process started with `ZE_PLUGIN_MODE=declare` is being interrogated rather
than run. It writes the Stage 1 `declare-registration` message to STDOUT, in the
same newline framing and under the same request id `#1`, and exits 0. There is
no connection, no `ok` response to read, no barrier and no Stage 2, so none of
the five stages above runs.

| | Live start | Query mode |
|---|---|---|
| Carrier | the hub connection | the child's stdout |
| Message | `#1 ze-plugin-engine:declare-registration <json>` | the same line, byte for byte for the same declaration |
| After it | Stage 2 configure, then the barrier | the process exits 0 |
| Hub variables read | host, port, token, CA | none |

The declaration a query answers carries what the plugin states in its own
source. `Run` derives one field from a Plugin's registered callbacks,
`wants-validate-open`, so a plugin that wants that field in its query answer
states it in the declaration it hands the query entry point.

Two routes write that line, and neither drives the handshake. For a plugin the
ze binary carries, `cli.Run` answers from the looked-up registration and calls
no plugin code. For a plugin binary ze does not carry,
`sdk.RunOrDeclare(declaration, activate)` writes the line and never calls
`activate`. A plugin that adopts neither route runs its live start under a
query, and the reader records it as having sent nothing. Both routes write the
line through `rpc.WriteDeclaration`, so the id, the method and the framing have
one definition and cannot drift apart.

The reader judges the version the way a live start does. A query answer that
declares no `protocol-version` speaks the text-era contract the daemon refuses
at Stage 1, so `show plugin declarations` reports that plugin `unstartable`, with
the refusal `rpc.NegotiateProtocolVersion` gives, never as a plugin declaring
its commands.

The query answer also carries the plugin's config schema, and the conformance
check judges it. No command joins it to the config schema: the daemon joins the
module of the live Stage 1 at start, and a command that parses a config with no
daemon (`ze config validate` and the others) starts no plugin, in query mode or
any other. It parses against the compiled-in schema and says that a section
under an external plugin's root is not available with no daemon
(`docs/architecture/config/yang-config-design.md`, "Who publishes"). Query mode
is started only by the conformance check, which runs the program its own
command line names, and by `show plugin declarations config <file>` inside a
running daemon. With no daemon that command starts nothing, and the row of an
external plugin reads `not-available`.
<!-- source: internal/component/config/declared_schemas.go -- SchemaWithDeclaredPlugins -->
<!-- source: pkg/plugin/rpc/declaration.go -- WriteDeclaration -->
<!-- source: pkg/plugin/sdk/sdk_query.go -- RunOrDeclare -->
<!-- source: internal/component/plugin/declarations.go -- queriedDeclarationRow -->
<!-- source: internal/component/plugin/cli/main.go -- Run, answerQuery -->

A declaration query inherits its caller's context as well as the independent
`ze.plugin.query.timeout` budget. Caller cancellation or deadline stops the
query's isolated process group and joins its direct child before returning; the
bounded output drain still applies. This records a caller-canceled query,
including any declaration already read, rather than blaming the plugin for a
query-budget timeout. The conformance checker returns the caller's error and no
completed report. A query-budget expiry with its caller still active remains a
plugin timeout and a failed conformance row.
<!-- source: internal/component/plugin/declarations.go -- QueryDeclaration, canceledDeclarationQuery -->
<!-- source: internal/component/plugin/conformance/conformance.go -- Run -->

### Tier-Ordered Startup

Plugins are grouped into dependency tiers before handshake begins. All processes
are started at once (single ProcessManager), but the 5-stage handshake is
sequenced tier by tier. Tier 0 completes its full handshake -- including command
registration -- before tier 1 begins.

The config-path and explicit selections are checked together before spawning.
An explicitly selected prerequisite of a config-path plugin moves into the
config-path phase, transitively, so an order-only `StartAfter` edge cannot be
lost at the phase boundary. Unrelated explicit plugins remain in phase 2.
This reordering never selects a new plugin and never borrows the edges of a
compiled-in namesake for an external program. Registration edges name the
resolved implementation, not the operator's process label: a renamed provider
or consumer retains its ordering. Each edge expands to every selected alias of
its target implementation; the original process labels and execution configs
remain unchanged. Cycle preflight, phase promotion, and per-phase handshakes use
this same process-label graph.
<!-- source: internal/component/plugin/server/startup.go -- selectedPluginGraph, startupSelectedPhases, runPluginStartup, runPluginPhase -->

```
Tier computation (Kahn's algorithm / BFS layering):
  Tier 0: plugins with no dependencies      (e.g., bgp-adj-rib-in)
  Tier 1: plugins depending only on tier 0   (e.g., bgp-rs depends on bgp-adj-rib-in)
  Tier N: plugins whose deps are all in tiers < N
```

**Per-tier coordinator:** Each tier gets its own `StartupCoordinator` with
tier-local indices. Processes in later tiers block naturally on `net.Pipe` write
until the engine reads their `declare-registration` during their tier's turn.

```
+-------------------------------------------------+
|            TIER-ORDERED STARTUP                  |
+-------------------------------------------------+
|                                                  |
|  ProcessManager starts ALL processes             |
|         |                                        |
|         v                                        |
|  TopologicalTiers(names) -> [[rib], [rs]]        |
|         |                                        |
|         v                                        |
|  TIER 0: [bgp-adj-rib-in]                       |
|    Coordinator(1 plugin)                         |
|    5-stage handshake -> commands registered       |
|    procWg.Wait()                                 |
|         |                                        |
|         v                                        |
|  TIER 1: [bgp-rs]                                |
|    Coordinator(1 plugin)                         |
|    5-stage handshake -> can dispatch to rib      |
|    procWg.Wait()                                 |
|         |                                        |
|         v                                        |
|  ALL TIERS DONE                                  |
|    coordinator = nil                             |
|    Start async handlers for ALL processes        |
|                                                  |
+-------------------------------------------------+
```

**Why tier ordering:**
- Prevents "unknown command" errors when dependent plugins dispatch to
  dependencies during or immediately after startup
- Dependencies are registered in `CommandRegistry` after stage 5, so they
  must fully complete before dependents attempt `dispatch-command` RPCs

**Plugins within the same tier** still use the original barrier model (diagram
above) -- they progress through all 5 stages together. Tier ordering only
serializes across tiers, not within them.

### Shutdown

The optional Stage 1 field `exit-confirms-cleanup` gives a zero exit the meaning
that the original engine confirmed its native cleanup. The compiled internal
registration must carry the same promise. The host reserves warning capacity
before the internal runner starts, or before accepting the external declaration.
A full reservation set refuses admission before `OnStarted`.
<!-- source: internal/component/plugin/process/terminal_cleanup.go -- AdmitTerminalCleanup, CheckTerminalCleanupDeclaration -->

An internal runner can report its final cleanup cause through its original
connection's `ReportTerminalCleanupFailure(string) error` method before return.
The connection still exposes its `DirectBridge`. A forked runner reports the
cause in structured stderr with `code=doctor-plugin-cleanup` and `error`.
Cause capture precedes the log relay filter and retains one bounded first cause.
A stderr line never confirms successful cleanup.
<!-- source: internal/component/plugin/process/terminal_cleanup.go -- terminalCleanupConn, captureTerminalStderr -->

The host waits for the actual internal return, or the original child's reaping
and stderr join. A confirmed zero exit releases an unused reservation.
A failure, unknown result, signal or incomplete stderr retains its original
warning with `ze show warnings` as the next step. A new process cannot clear
that warning by reusing the plugin's configured name.
<!-- source: internal/component/plugin/process/terminal_cleanup.go -- finishTerminalCleanup, actualForkCleanupSuccess -->

Explicit subsystem shutdown sends `ze-plugin-callback:bye` with an optional reason:
<!-- source: pkg/plugin/rpc/types.go -- ByeInput -->

```
#99 ze-plugin-callback:bye {"reason":"shutdown"}
#99 ok
```

After a successful `bye`, the SDK exits its callback loop and the engine closes
the connection. `OnBye` has signature `func(reason string) error` on both socket
and bridge transports. The SDK sends an error response and keeps the plugin
running when cleanup fails.

A live configuration removal sends reason `removed` while the plugin's engine
connection and dependencies are still available. The callback can withdraw
installed state through engine RPCs. The server waits up to 500 ms for its
acknowledgement. A callback refusal or timeout rejects removal and retains the
dependencies; it does not close a live callback transport or report a successful
reload. A pending callback keeps its original acknowledgement waiter until it
completes or the daemon shuts down. Startup failures before the running stage
receive no callback. A disconnect or callback panic instead requires replacement
of the failed generation; the removal owner performs that recovery rather than
leaving it to the ordinary crash policy.
<!-- source: internal/component/plugin/server/startup_removal.go -- pluginRemoval, notifyPluginRemoval -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- ByeHandler, OnBye -->
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- handleBridgeCallback, eventLoop -->

Removal remains provisional until the whole reload is accepted. On rejection,
including a callback that succeeds after timeout, recovery waits for the outer
reload's completion and then acquires the existing transaction lock.
It reads the current committed configuration, joins an exited or removed
generation and restarts its source and needed dependencies, or redelivers
configuration to a live source whose cleanup failed. The first recovery attempt
runs without another reload. If restoration fails, its ownership record remains;
the next explicit reload retries restoration before applying new configuration.
Failed cleanup can be retried after recovery; an in-flight callback is never
duplicated.
Replacement startup receives the committed configuration, not the still-published
candidate. Its failure remains a retryable recovery error; ordinary startup and
runtime failures keep their declared failure policy.
<!-- source: internal/component/plugin/server/startup_removal.go -- finishRemovalScope, recoverPluginRemoval, restoreRemovedPlugin -->
<!-- source: internal/component/plugin/server/reload_tx.go -- reloadAcceptance -->

A failed removal also compensates participants that already committed another
change in the same reload, and restores reactor state, before returning the
error. Rejecting an outer publication performs the same compensation under the
transaction lock. An older scope cannot undo a newer successful reload, including
one that accepted the same tree without a diff. Rejected nested scopes restore
their predecessor's ownership, so both rejections return to the original tree.
If compensation itself fails, the server retains the inverse transaction and
blocks new reloads, including no-diff requests, until an explicit retry succeeds.
It does not publish a restored tree or repeat completed inverse phases on failure.
<!-- source: internal/component/plugin/server/reload_compensation.go -- compensateReload -->
<!-- source: internal/component/plugin/server/reload.go -- txLock.configOwner, reloadConfig -->

When one removal also removes dependencies, the server reverses the selected
dependency graph used at startup. That graph includes hard dependencies,
optional dependencies and `StartAfter` edges. Each producer must acknowledge
successful cleanup before the server stops the next dependency tier. Failed
auto-load rollback and orphan cleanup use this order too. A callback or ordering
error stops the sequence, and the caller receives that error.
<!-- source: internal/component/plugin/server/startup_autoload.go -- stopCollectedProcesses -->

Whole-daemon shutdown closes plugin connections without `bye`. The server
selects the reverse of the existing selected startup dependency graph. The process
manager stops and joins each tier under one 3-second cleanup deadline.
A missing resolver or a dependency cycle forces a simultaneous stop of the
remaining processes.
<!-- source: internal/component/plugin/server/shutdown.go -- shutdownPluginTiers -->
<!-- source: internal/component/plugin/process/manager.go -- StopInOrder -->

The manager retains the first ordering refusal and raises
`doctor-plugin-shutdown-order`. The server persists that distinct stop reason
before the store closes. Local doctor can still name it after the daemon exits.
This refusal is separate from cleanup that exceeds its deadline.
<!-- source: internal/component/plugin/process/manager_shutdown.go -- RecordShutdownOrderFailure, ShutdownOrderFailure -->
<!-- source: internal/component/plugin/server/server.go -- cleanup -->

Joins within a signaled tier run concurrently, so one slow consumer does not
consume another consumer's opportunity to finish before that shared deadline.
The manager joins an internal consumer's engine finalizers before it stops that
consumer's providers. For an external consumer, it first joins reaping and the
stderr relay. Admission is sealed before the acceptor is stopped, so failed
starts and replacements cannot take
another stop's ownership or publish an unjoined generation. All admitted but
unpublished starts remain owned through their failed-start cleanup.
<!-- source: internal/component/plugin/process/manager.go -- StopInOrder -->
<!-- source: internal/component/plugin/process/manager_shutdown.go -- joinStarting, stopEngines, waitShutdownProcess -->

The normal daemon stop keeps server dispatch and process parents live during
that ordered cleanup. It cancels server dispatch afterwards, then runs `OnStop`
hooks. An already-canceled parent or an expired cleanup deadline cannot promise
dependency order. The manager still stops every remaining process and reports
unfinished cleanup. Standalone `ProcessManager.Stop` remains reusable and simultaneous.
Plugins must distinguish live removal from connection closure: subsystem owners
retain their daemon-shutdown policy, including firewall rule retention.
<!-- source: internal/component/plugin/server/shutdown.go -- shutdownPluginTiers -->
<!-- source: internal/component/plugin/process/manager.go -- Stop, StopInOrder -->
<!-- source: internal/component/plugin/process/manager_shutdown.go -- joinStarting, stopEngines, waitShutdownProcess -->

A runtime request that the stop itself canceled gets no answer. Whole-daemon
stop cancels each process generation before it closes its connection, while server
dispatch remains available to the generations still cleaning up. A stop of one
process generation (a plugin restart, a removal after `bye`) likewise cancels
that generation's context before it closes its connection. A handler waiting
on either context returns the cancellation while the connection is still
open. The engine leaves that request unanswered, so the close answers it: the
plugin's pending call ends with the connection's terminal error, like every
other call still pending then, and never with an error line that the plugin
would have to tell apart from a live engine's refusal by its text.
<!-- source: internal/component/plugin/server/dispatch_registry.go -- serveEngineOpJSON, engineSessionEnded -->
<!-- source: internal/component/plugin/server/server.go -- Stop -->
<!-- source: internal/component/plugin/process/process.go -- Stop, monitorCmd -->

In process the rule is the same, and the fenced bridge answers: a native or
JSON-shaped direct call that the stop canceled returns `rpc.ErrBridgeClosed`, the answer
every call gets once `Process.Stop` fences the bridge's dispatch. A call
already admitted can meet the cancellation before that fence through parent
cancellation or the end of its process generation. Every other failure is
answered, a refusal during the stop included,
on both transports. A startup request answers every failure.
<!-- source: internal/component/plugin/server/dispatch_registry.go -- serveEngineOpJSON, engineSessionEnded, serveEngineOpDirect -->
<!-- source: internal/component/plugin/server/dispatch_native.go -- dispatchPluginRPCNative, directCallError -->
<!-- source: pkg/plugin/rpc/bridge.go -- StopDispatch, DispatchRPC -->
<!-- source: internal/component/plugin/server/server.go -- Stop -->
<!-- source: internal/component/plugin/process/process.go -- Stop, monitorCmd -->

#### The stop drain

`request restart` accepts an in-place daemon restart through the shared command
dispatcher. Its response completes before the hub's existing restart action
records `in-place`, preserves the BGP graceful-restart marker and signals the
stop. This uses the same restart preparation as SSH; it never requests a host
reboot. After teardown, in-place restart exits with the shared
`rpc.DaemonRestartExitStatus` (75), while a clean stop remains 0 and a daemon
failure remains 1. The generated systemd unit accepts 75 as successful and
forces a new start. Gokrazy's normal supervision loop also retries this nonzero
exit; a foreground caller must start the next process itself.
<!-- source: internal/component/plugin/server/daemon_restart.go -- handleDaemonRestart -->
<!-- source: cmd/ze/hub/infra_setup.go -- infraSetup -->
<!-- source: pkg/plugin/rpc/drain.go -- DaemonRestartExitStatus -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig -->
<!-- source: internal/plugins/systemd/unit.go -- buildUnitFile -->

Before whole-daemon shutdown closes any connection, the hub runs one drain
phase: after config changes are fenced, the engine sends
`ze-plugin-callback:drain` once, in parallel, to every plugin whose Stage 1
`declare-registration` carried a `drain-budget` above zero. The plugin makes
its protocol unattractive to its neighbors, waits its configured time, and
answers `ok`, or an error that the engine logs as a failed drain by plugin
name.

```
#99 ze-plugin-callback:drain {"kind":"stop","deadline":"2026-10-14T12:01:00Z"}
#99 ok {}
```
<!-- source: pkg/plugin/rpc/drain.go -- MethodDrain, DrainInput, DrainOutput, DrainKind -->

| Field | Type | Meaning |
|-------|------|---------|
| `kind` | `stop` or `restart` | How the daemon stops; each plugin decides what it means for its protocol |
| `restart-cause` | `in-place` or `reboot` | On `restart` only: why the daemon restarts |
| `deadline` | RFC 3339 time | After it the engine reads no answer and tears down |

Stage 1 declares the drain with two fields. `drain-budget` is the longest the
handler may take, in seconds, and `drain-handler` states that the plugin
answers `drain`; the Go SDK sets `drain-handler` when `OnDrain` registered a
handler. The engine refuses the plugin at Stage 1, naming the plugin, the
budget and the ceiling, when the budget is above the daemon drain ceiling of
60 s (`rpc.DrainCeilingSeconds`) or when a budget comes with no handler. A
budget is refused, never clamped.
<!-- source: internal/component/plugin/server/startup.go -- validateDrainDecl -->

The deadline is the earlier of the ceiling counted from the start of the stop
and the largest declared budget counted from the start of the drain, so a slow
config fence spends from the ceiling. The engine makes each drain call from a
long-lived caller per plugin generation, started by `superviseProcess` with the
generation and ended before its failure policy runs, under the generation's
context rather than the deadline, so an answer that arrives late is still read
by the caller and dropped by nobody. At the deadline the engine names every
plugin that has not answered and continues the teardown.
<!-- source: internal/component/plugin/server/drain.go -- drainCaller, DrainRound.Run -->
<!-- source: cmd/ze/hub/drain.go -- runStopDrain, stopDrainDeadline -->

The SDK runs the handler on one long-lived drain worker per connection, never
on the event loop, so the plugin keeps answering commands while its drain
waits; the answer travels for the drain request's own ID, after later
requests may already have been answered. The first stop-kind `drain` moves the
worker to `stopping`, and the event loop refuses any later stop-kind `drain`
on that connection at once, naming the plugin. `OnDrain` has signature
`func(ctx context.Context, input rpc.DrainInput) error`; `ctx` ends when the
connection closes.
<!-- source: pkg/plugin/sdk/sdk_drain.go -- OnDrain, drainWorker -->

#### Who stops the plugin server

**A component does not stop a server it borrowed. Whoever CONSTRUCTS the plugin
server stops it, and nobody else.**

The hub constructs the one plugin server of a normal daemon and stops it at
shutdown. A component that runs inside that server borrows the pointer: the BGP
reactor gets it through `registry.SetPluginServer` -> `registry.GetPluginServer`
-> `Reactor.SetPluginServerAny`, and its own engine runs as a plugin under the
same server. A borrowed server is read, never stopped.
<!-- source: cmd/ze/hub/main.go -- runYANGConfig, pluginserver.NewServer then apiServer.Stop -->
<!-- source: internal/component/bgp/reactor/reactor.go -- Reactor.cleanup, the !externalServer guard -->

Stopping a borrowed server costs twice, and both costs were measured:

| Occasion | What a component stopping its host does |
|----------|------------------------------------------|
| Daemon shutdown | `Server.Stop` calls `ProcessManager.Stop`, which waits for every plugin engine. The engine that made the call cannot return until the call returns, so the stop is bounded only by `pluginStopGrace` plus the group wait: 3.520s per stop, with `resources it installed may be left behind` logged twice, on a daemon that had released everything |
| A reload that removes the component | `runBGPEngine` returns when bgp is removed at reload, not only at shutdown, and its tail stops the reactor. An unguarded stop takes the hub's whole plugin server down, and every other plugin with it, while the daemon keeps running |

A component that CONSTRUCTS its own server still stops it: a standalone reactor
(`Config.Standalone`, the in-process runner of `le chaos run`) self-hosts, and its cleanup
is that server's only stop. So the rule is a test of ownership, never of the call
site.
<!-- source: internal/component/bgp/reactor/reactor.go -- startAPIServer, the self-host branch -->
<!-- source: internal/chaos/inprocess/runner.go -- the standalone reactor whose cleanup is its server's only stop -->

---

## Runtime RPCs

### Engine to Plugin (Callbacks)

<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- callback method constants, getCallback -->
<!-- source: pkg/plugin/rpc/types.go -- callback input and output types -->

| Method | Input | Response | Description |
|--------|-------|----------|-------------|
| `deliver-event` | `{"event":"<json>"}` | `ok` | Single event |
| `deliver-batch` | `{"events":[...]}` | `ok` | Batched events |
| `execute-command` | `ExecuteCommandInput` | a record answer, read as one value into `ExecuteCommandOutput` | Command execution |
| `config-verify` | `ConfigVerifyInput` | `ConfigVerifyOutput` | Validate candidate config |
| `config-apply` | `ConfigApplyInput` | `ConfigApplyOutput` | Apply config changes |
| `config-rollback` | `{"transaction-id":"..."}` | `ok` | Undo changes for a config transaction. The SDK runs the handler only for an accepted apply not yet committed |
| `config-committed` | `{"transaction-id":"..."}` | `ok` | The transaction committed: its applies are permanent, and a later rollback must not undo them. A plugin built on an SDK older than this callback answers "unknown method": the engine logs it and the plugin keeps running its rollback handler on every rollback, as before |
| `config-operation-decompose` | `ConfigOperationDecomposeInput` | `ConfigOperationDecomposeOutput` | Decompose a config diff into operations |
| `config-operation-verify` | `ConfigOperationVerifyInput` | `ConfigOperationVerifyOutput` | Validate one config operation |
| `config-operation-apply` | `ConfigOperationApplyInput` | `ConfigOperationApplyOutput` | Apply one config operation |
| `config-operation-rollback` | `ConfigOperationRollbackInput` | `ConfigOperationRollbackOutput` | Undo applied config operations |
| `config-operation-commit` | `ConfigOperationCommitInput` | `ConfigOperationCommitOutput` | Commit a config operation transaction |
| `config-complete` | `ConfigCompleteInput` (`transaction-id`, `accepted`, `deadline-ms`) | `ConfigCompleteOutput` echoes identity and decision with explicit `status` | Acknowledged whole-reload cleanup for Stage-1 `config-completion` owners; operation commit does not release their undo |
| `source-policy` | `SourcePolicyInput` | `SourcePolicyOutput` echoes transaction, operation, source and target epoch with explicit `status` | Acknowledged prepare/apply/abort/finalize for declared retained-source owners |
| `post-startup` | None | `ok` | Report that all plugin startup phases are complete |
| `validate-open` | `ValidateOpenInput` | `ValidateOpenOutput` | Validate an OPEN message |
| `encode-nlri` | `EncodeNLRIInput` | `{"hex":"..."}` | Encode NLRI |
| `decode-nlri` | `DecodeNLRIInput` | `{"json":<raw JSON>}` | Decode NLRI |
| `decode-capability` | `DecodeCapabilityInput` | `{"json":<raw JSON>}` | Decode a capability |
| `bye` | `ByeInput` | `ok` or error | Acknowledged cleanup; an error leaves the plugin running |
| `drain` | `DrainInput` | `DrainOutput` or error | Stop drain before whole-daemon shutdown ("The stop drain") |
| `filter-update` | `FilterUpdateBatchInput` | `FilterUpdateBatchOutput` | Route filter request: a batch of routes, one verdict each |
| `doctor-check` | `DoctorCheckInput` | `DoctorCheckOutput` | Doctor readiness check |
| `resolver-cache-clear` | `ResolverCacheClearInput` | `ResolverCacheClearOutput` | Clear one declared resolver cache |
| `enrich-show` | `EnrichShowInput` | `EnrichShowOutput` | Show command enrichment |

A Go plugin whose `config-apply` failed and could not put its running config
back answers with `sdk.RunningConfigLost(cause, problem, firstStep)`. The
error text the engine receives is the operator error's three parts, one per
line: the problem, then the SDK's why (`sdk.RunningConfigLostWhy`) with the
cause, then `fix: ` with the plugin's first step and the daemon restart
(`sdk.RunningConfigLostRestart()`), which spans several lines. A plugin that
passes an empty problem gets `sdk.RunningConfigLostProblem`, never a panic,
because the plugin's text is not Ze's to guarantee. The wire format
does not change: the answer carries that text as its error string, and
`errors.Is(err, sdk.ErrRunningConfigLost)` still opens the rollback. A plugin
in another language writes the same three parts into its error string.
<!-- source: pkg/plugin/sdk/config_tx.go -- RunningConfigLost, RunningConfigLostProblem, RunningConfigLostWhy, RunningConfigLostRestart -->

`config-complete` runs on the same serialized callback path as config operations
over either transport. The Go SDK's `OnConfigComplete` declares support and
passes the handler a context bounded by the absolute request deadline. An
already-expired request does not invoke the handler. The handler MUST finish
cleanup before returning and MUST honour cancellation; a queue admission or an
unjoined asynchronous cleanup is not success. The engine bounds its RPC wait
independently and checks the echoed transaction and accepted decision.
Both request and receipt MUST include `accepted` as an explicit JSON boolean.
Missing or `null` decisions are malformed, not rejection: the SDK refuses them
before invoking cleanup, and the engine refuses them as completion receipts.
Explicit `false` remains a valid rejected decision.

Accepted completion follows durable publication. A failure is returned with the
candidate still active, not described as rollback. Rejected completion follows
restoration of old section/filter configuration and original operation undo.
The server captures the applying process and connection and refuses a successor
lifetime. Duplicate outer completions return their recorded result without
repeating callbacks. Operation verify/apply/rollback/commit requests also carry
`deadline-ms`, including a fresh rollback deadline independent of apply expiry.
<!-- source: internal/component/plugin/server/reload_tx.go -- DeferReloadAcceptance, completeBatches -->
<!-- source: internal/component/plugin/server/config_tx_bridge.go -- lookupProcess, subscribeOperationRollback -->
<!-- source: internal/component/plugin/ipc/rpc.go -- SendConfigComplete -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigComplete -->

`source-policy` is a separate lifecycle from shutdown drain and config completion.
Stage 1 `source-policy-owner` declares retained source state; `source-policy`
declares handler support. `OnSourcePolicy` sets both. A pure observer declares
neither. The server's `CaptureSourcePolicyOwners(names)` examines the selected
process registrations, excludes observers and refuses any declared owner without
support before invoking anyone. Third-party state owners MUST declare ownership;
a Go plugin with no handler sets `Registration.SourcePolicyOwner` itself. The
refusal names the plugin, says it cannot remove the peer's routes when the
import policy is withdrawn, and gives the remedy: disable the peer, commit,
set the new policy, re-enable the peer and commit again.
In tree, `bgp-rib`, `bgp-adj-rib-in`, `bgp-rs` and `bgp-rr` are owners;
`bgp-rib` is the only one that writes to destinations, and the forwarders
settle only their own pending work. What each phase does to their stores is in
`docs/architecture/plugin/rib-storage-design.md`, "Source-policy lifecycle
owners".

The capture returns the owners in their registered dependency order
(`Dependencies`, `OptionalDependencies` and `StartAfter`, the edges startup
orders the same processes by), with ties broken by process name, and refuses a
cycle. The coordinator calls them in that order for every phase, and after
each owner's call drains the plugin-process subscribers of
`bgp-rib/validation-change` before calling the next, so a dependent owner's
settle covers the relays its prerequisites caused. The reactor coordinator
also refuses, before publication, a source whose UPDATEs reach a forwarder
while the process that answers the RIB recovery command, and holds the sent
inventory, is not among the captured owners.

Each immutable handle captures both `Process` and `PluginConn`. Its `Call` checks
that lifetime before invocation and after the acknowledged result; a successor
cannot finish the captured owner's transaction. No server lock survives a handler
call. Before prepare, the coordinator MUST settle its receive-publication
frontier; the handle then uses strict `DrainEventsApplied`, never soft quiesce.

The request carries `transaction-id`, `operation`, `source`, `next-policy-epoch`
and absolute `deadline-ms`: the earlier of the calling transaction's remaining
budget and the request's own deadline, so an owner never commits after its
caller has given up. The captured source contains `peer`, `peer-owner`,
`session-generation`, `policy-epoch`, `receive-frontier` and
`template-generation`. Every field is required and non-null, including explicit
zero frontiers/template generations. Peer owner, session generation and original
policy epoch are nonzero. Prepare and finalize use target zero, and apply
requires a target newer than the captured epoch. Abort takes one of two
targets. Zero aborts a transaction that was prepared and never applied: the
coordinator did not advance the source's epoch, and the owner only releases its
prepare pins, with no restamp, no eligibility change and no downstream write. A
target newer than the captured epoch aborts after apply advanced the epoch, and
the owner restamps the restored state to it. The coordinator MUST allocate
globally monotonic epochs and never reuse an apply epoch for abort. The
transport validates the target against the captured original epoch, not the
coordinator's transaction history; the owner's ledger
(`internal/core/bgp/sourcepolicy`) then refuses, before any change, a zero
target after apply, a nonzero one not newer than the apply target, and any
abort of a finalized or aborted transaction. A nonzero target is also admitted
from prepare, because a coordinator whose apply fails part way has already
advanced the epoch, and every prepared owner restamps to it.

The SDK passes a deadline-bounded context to the handler and refuses malformed,
unknown or already-expired requests before owner mutation. A successful receipt
echoes every identity field and target exactly. The owner MUST fence asynchronous
work and settle downstream writes before returning; enqueue is not completion.
Prepare pins state, apply changes eligibility, abort restores pinned state under
a fresh epoch (or, with target zero before apply, only releases the pins), and
finalize releases accepted pins. The coordinator MUST retain
individual prepared handles so partial prepare aborts only prepared owners,
plus the owner whose prepare failed: a prepare whose call timed out may still
finish and pin. An owner MUST answer a target-zero abort of a transaction it
never prepared (it holds no ledger, or another transaction's) as done, and a
prepare handler that settles before pinning MUST re-check its deadline after
settling and refuse without pinning once it has passed. A prepare finding a
prepared, never applied ledger of another transaction replaces it at any
identity, releasing its pins: it changes nothing but pins, and the coordinator
claims a peer for one transaction at a time, so the ledger was left by a lost
target-zero abort (an abort that expired in the SDK queue keeps the same
identity). A prepare finding an applied ledger of another transaction replaces
it only when the incoming identity is newer (a later session generation, or a
later policy epoch of the same session); one as new or newer still holds the
source (`sourcepolicy.Owner.Reprepare`). An owner whose write to a destination
is refused during apply or abort fails that phase: `bgp-rib` names the
destination, the engine's reason and the missing `send [ update ]` grant.
Transport support alone does not implement route invalidation or reload admission.

The SDK runs the handler off the callback loop, on one long-lived source-policy
worker per connection, the same way on DirectBridge and on the wire. An owner
that waits for its own downstream work, or makes an engine RPC whose answer
needs an event or a callback delivered to the same plugin, would otherwise
block the loop that delivers it until the deadline expired.

| Property | Behaviour |
|----------|-----------|
| Worker | Started with the plugin's callback loop when `OnSourcePolicy` is registered, and stopped with it. No goroutine per request. It shares no state with the stop drain worker |
| Admission | The loop validates each request where it arrives, so a malformed, unknown or expired request is refused as a callback error before any owner state changes. A valid request joins the worker's queue |
| Queue | At most 16 admitted requests wait behind the running one. A request that finds the queue full is answered at once with a `status: error` receipt saying the queue is full, and its handler never runs. The loop never blocks on the queue |
| Order | One handler call at a time, in arrival order, so each source's operations reach the owner in the order the engine sent them |
| Events | A request is admitted only after every callback that arrived before it has been applied, so the handler never sees state older than an event the engine drained before prepare. Events and other callbacks that arrive while it runs are applied by the loop in their own order, concurrently with the handler. The handler MUST synchronize the state it shares with `OnEvent` and the other callbacks |
| Deadline | The handler's context ends at the request's absolute `deadline-ms`, or when the plugin stops. A request whose deadline passed while it was queued fails without running the handler. A handler that returns after its deadline fails its receipt |
| Receipt | Sent only after the handler returns. Requests still queued when the plugin stops are answered with a failed receipt. A receipt is written even after the loop's context has ended, bounded by the later of the request's `deadline-ms` and one second from the moment it is written. The stop `drain` answer is bounded the same way |
| Panic | The request is answered with an error and the worker runs nothing more, because the owner's state is unknown. Every request already queued is answered with a failed receipt, then the connection ends with the panic: the bridge fails every callback, the wire connection closes, and the loop returns the panic rather than a clean stop |

The coordinator awaits each owner's receipt before it sends that source's next
operation, so the queue holds at most one request for each source being changed
at once. A queue-full receipt is a failed operation, handled like any other.

An owner's best-path changes reach other plugins as events, and a subscriber
that runs as a process receives them queued, after the owner's receipt. Its
answer (a write to a destination) is downstream work the owner cannot fence.
After the owners' apply or abort the coordinator calls the server's
`DrainEventSubscribers(ctx, namespace, event-type)` for each event the owners
publish: it runs strict `DrainEventsApplied` on every process the event's
subscription matches, the same processes delivery feeds for an event that
carries no peer address, so each such event queued before the call has been
applied when it returns. An unregistered event is an error, never an empty
drain. Engine-side subscribers run inside the emit and need no drain.
<!-- source: internal/component/plugin/server/source_policy.go -- DrainEventSubscribers -->
<!-- source: internal/component/bgp/reactor/source_policy_reload.go -- sourceInvalidation.settleDownstream -->
<!-- source: pkg/plugin/sdk/sdk_source_policy.go -- sourcePolicyWorker, sourcePolicyQueueDepth, admit, run, serve, call, refuseQueued -->
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- workerAnswerContext -->
<!-- source: pkg/plugin/sdk/sdk_async_callback.go -- answerAsyncCallback -->
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- eventLoop, bridgeEventLoop -->

`StoredRoute.source-policy-epoch` carries original ingress provenance independently
of dispatch session generation. Zero means unrecorded; replay MUST NOT replace it
with the source's current policy epoch.
<!-- source: pkg/plugin/rpc/source_policy.go -- SourcePolicyInput.Validate, UnmarshalJSON -->
<!-- source: pkg/plugin/rpc/types.go -- SourcePolicyIdentity, SourcePolicyInput, SourcePolicyOutput, StoredRoute -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnSourcePolicy -->
<!-- source: internal/component/plugin/ipc/rpc.go -- SendSourcePolicy -->
<!-- source: internal/component/plugin/server/source_policy.go -- CaptureSourcePolicyOwners, orderSourcePolicyOwners, SourcePolicyOwner.Call -->

All methods are prefixed with `ze-plugin-callback:`.

`EmitEventApplied` uses the existing `emit-event` operation with `await-applied`
and a positive absolute `deadline-ms`. Ordinary `EmitEvent` confirms queue
admission and keeps its typed slot. Applied emission uses the shared RPC
handler in both placements. A matching self subscription refuses before plugin
delivery or engine callbacks. Existing namespace, direction and peer matching
remain in force.
<!-- source: internal/component/plugin/server/dispatch.go -- deliverEventCaptured -->

The host captures matching Processes and connections, joins all handlers, then
uses the event settlement contract below. It rechecks every captured lifetime
before returning `applied` with the exact request deadline. Both placements use
the existing cancellable request timeout. Callers MUST release recipient locks
and avoid cyclic synchronous waits. Cancellation joins the outer request.
Partial application is not rolled back, and an error cannot confirm downstream
completion. Resolve the named failure before retrying the lifecycle operation.
<!-- source: internal/component/plugin/server/emit_event_applied.go -- emitEventApplied, eventEmissionReceiver.liveError -->
<!-- source: pkg/plugin/sdk/sdk_emit_applied.go -- EmitEventApplied -->
**Event delivery receipts:**

An asynchronous observer registers `OnEventSettlement` before `Run`, which
declares `event-settlement` at Stage 1. Its `ze-plugin-callback:event-settlement`
callback follows the applied FIFO receipt and acknowledges downstream work,
not callback admission. The request carries an absolute `deadline-ms`; the
receipt echoes it with `status` (`ok` or `error`) and the failure cause when
needed. Missing, null, expired or mismatched values cannot acknowledge work.
The failure field remains bounded to 4096 bytes. A producer whose complete
diagnostic exceeds that bound must provide an explicit bounded projection and
an existing retrieval path for omitted live details; the SDK refuses an
oversized failure instead of silently truncating it. The RIB names whole
destination identities, states omitted counts and points to the current
`selected-output` list in `ze show bgp rib status`.

A live export-final transition uses a targeted `state` event with state `up`,
the existing `destination-session` and the recipient's
`selected-destination` authority. It reaches only the captured ordinary owner;
it announces no connection transition and carries no initial-replay token.
Reason `export-policy` reoffers current learned selections and preserves local
output. Reason `export-policy-all` reoffers both route kinds when the local
export decision changes. Both preserve admitted origins and recheck the
current selection under the new policy. Neither emits ROUTE-REFRESH markers.
The event-settlement callback joins this work through final output receipts.
The caller captures an authenticated
`SelectedOwner`, not only its configured name. Delivery verifies that exact
current, ready Process and its granted destination-state subscription before
queueing and after settlement. A same-name replacement cannot receive or
acknowledge the old owner's reoffer. The engine bounds queue admission with
`DeliverContext` and reports application or settlement failure to reload.
<!-- source: internal/component/bgp/server/export_policy_event.go -- OnPeerExportPolicy -->
<!-- source: pkg/plugin/rpc/enums.go -- ReasonExportPolicy, ReasonExportPolicyAll -->
<!-- source: pkg/plugin/rpc/event_settlement.go -- EventSettlementInput, EventSettlementOutput -->
<!-- source: internal/component/bgp/plugins/rib/rib_selected_failure.go -- selectedBarrierError -->
<!-- source: internal/component/bgp/plugins/rib/rib_selected_status.go -- unsettledStatus -->

The SDK runs this callback on its own bounded FIFO worker, independently of
ordinary event delivery and current-selection lookup. The handler MUST honor
its context and synchronize shared state. It can wait for a nested engine RPC
or a later event without blocking either SDK callback loop. Each request has
its own completion; overload is refused and a handler panic fails the connection.
<!-- source: pkg/plugin/sdk/sdk_event_settlement.go -- OnEventSettlement, eventSettlementWorker -->

`Process.SettleEventsApplied` composes the strict FIFO receipt with this declared
callback on one captured `PluginConn`. The callback runs outside the event
delivery worker and every process lock. The same connection and process must
still be live after its receipt; a same-name replacement cannot complete the
old process's work. An observer without the support declaration retains strict
FIFO application semantics. A declared observer requires a caller deadline;
missing support, a failed receipt or a lost lifetime cannot imply completion.
The caller budget includes reply time. The Process gives work an earlier
deadline, reserving at most the existing one-second worker-answer allowance
and at most half its remaining budget. IPC may receive a failed receipt after
that work deadline, within the original caller ceiling, preserving the named
unfinished destinations and deadline cause. An `ok` receipt after the work
deadline is still refused; the reply allowance never extends successful work.
Settlement failure is retryable and never becomes the lifetime-sticky delivery
error. The pure `DrainEventsApplied` contract and its existing source-policy,
recovery and command-read callers remain unchanged.
<!-- source: internal/component/plugin/process/event_settlement.go -- SettleEventsApplied, eventSettlementOwnerError -->
<!-- source: internal/component/plugin/ipc/event_settlement.go -- SendEventSettlement -->

The global `request quiesce` plugin drain uses this composition for each live
process captured from the current manager. A process already stopped at capture
has no drain; one that ends after capture cannot acknowledge the old work. The
processes settle concurrently and no manager lock is held across their callbacks.
<!-- source: internal/component/plugin/process/manager.go -- DrainEventDelivery -->
<!-- source: internal/component/plugin/server/quiesce.go -- registerPluginEventQuiescer -->

Each process owns a FIFO event queue. A sent-event callback enqueues work without
waiting for its handler; the delivery worker applies it through DirectBridge or
the plugin's acknowledged `deliver-batch` RPC. Internal runners that never
activate their allocated bridge retain the same JSON IPC delivery path.

`DrainEvents` is a soft application drain: it waits for earlier delivery attempts, not for
successful application, and a stopped process owes no drain. Recovery instead
uses `DrainEventsApplied`. Its FIFO receipt includes the first delivery error
in that `Process` lifetime, including an earlier fire-and-forget failure, timeout
or panic. A later successful batch or barrier MUST NOT clear the error; only a
new process lifecycle represented by a new `Process` starts without it. There
is no authoritative in-place resynchronization. The doctor reads a copy of that
first error (`plugin-panic`), and every batch is timed: one that fails with no
caller waiting, or takes over 500 ms on any transport, is logged with code
`doctor-plugin-delivery-failed`, counted in `ze_plugin_delivery_failures_total`
or `ze_plugin_delivery_slow_batches_total`, and kept in a five-minute window
the `plugin-delivery` check reports.
<!-- source: internal/component/plugin/process/doctor_records.go -- noteBatch, recordProjectionErr -->

The strict drain bounds both queue admission and receipt waiting by the caller's
context. It rejects an unstarted, stopped or rejecting owner and checks lifecycle
and known transport failure again after a successful receipt. This check is not
a lease on the owner's future lifetime and sends no probe RPC. The caller MUST
release locks needed by handlers before waiting; neither application nor IPC runs
under the drain's lifecycle locks.

Transport liveness follows activation, not bridge allocation: a never-activated
bridge uses the live IPC connection's state. Once activated, the bridge remains
the strict receipt's authority even if shutdown or failure clears readiness;
a healthy startup connection MUST NOT mask that stopped bridge.

<!-- source: internal/component/plugin/process/delivery.go -- deliverBatch, DrainEvents, DrainEventsApplied, deliveryOwnerError -->
<!-- source: internal/component/plugin/process/process.go -- Process.projectionErr -->
<!-- source: pkg/plugin/rpc/bridge.go -- DeliveryError -->
<!-- source: internal/component/plugin/ipc/rpc.go -- PluginConn.Err -->

**config-operation-\*:** The five operation callbacks carry a `ConfigOperation`
whose payload is the ordering contract. Beside `id`, `root`, `owner`, `type`,
`target` and `params` it carries three kebab-case keys: `verb`, which is
`create`, `destroy` or `modify`; and `produces` and `consumes`, each a list of
`ResourceRef` values naming what the operation makes available and what it needs
another operation to have produced. The engine orders by `verb` plus the target's
kind, and never compares `type`, which is the plugin's own label for the work.

An operation payload that carries no `verb` is refused at planning and the
transaction aborts, naming the plugin, the root and the operation id.

None of the five has a default handler in the SDK, so a plugin that registered
none answers "unknown method" to each. That is why a participant the planner
produced no operation for is applied through `config-apply` rather than
`config-operation-apply`.

<!-- source: pkg/plugin/rpc/types.go -- ConfigOperation, OperationVerb -->
<!-- source: internal/component/config/transaction/operation.go -- ValidateOperations -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- initCallbackDefaults -->

**doctor-check:** Engine invokes a plugin's declared doctor check by name.
Plugin runs the check and returns diagnostics (code, severity, message).
Declared during Stage 1 via `doctor-checks` field in `declare-registration`.
Only invoked at runtime via `show doctor`; offline `ze doctor` does not reach plugins.

<!-- source: pkg/plugin/rpc/types.go -- DoctorCheckDecl, DoctorCheckInput, DoctorCheckOutput -->

**resolver-cache-clear:** Engine asks a plugin to clear one resolver cache it
declared in the Stage 1 `resolver-caches` list (`peeringdb`, `irr` or `cymru`;
the engine refuses any other name and a name given twice). The input is
`{"cache":<name>,"action":"clear-all"|"delete-entry","key":<key>}`, where the
key is a decimal AS number, an AS-SET name or a canonical address. The plugin
clears only the clients it uses now and answers `{"removed":N}`. Sent when an
operator runs `clear <cache> cache`; an error answer makes that command an error
naming the plugin.

<!-- source: pkg/plugin/rpc/resolver_cache.go -- MethodResolverCacheClear, ResolverCacheClearInput, ResolverCacheClearOutput -->
<!-- source: pkg/plugin/sdk/sdk_resolver_cache.go -- OnResolverCacheClear -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnDoctorCheck -->
<!-- source: internal/component/plugin/server/server.go -- CallDoctorCheck, DoctorCheckPlugins -->
<!-- source: internal/component/doctor/cmd/show.go -- HandleDoctor plugin integration -->

**enrich-show:** Engine invokes a plugin's declared show enricher at show time.
Plugin receives the command, key, mode ("detail" or "brief"), and base data map as
JSON, returns enrichment data to merge into the base map. Declared during Stage 1
via `enrichers` field in `declare-registration`. Invoked at runtime when a show
handler calls `show.Enrich()` (mode "detail") or `show.EnrichBrief()` (mode "brief").
2s timeout prevents hung plugins from blocking show commands.

<!-- source: pkg/plugin/rpc/types.go -- EnricherDecl, EnrichShowInput, EnrichShowOutput -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnEnrichShow -->
<!-- source: internal/component/plugin/server/enricher.go -- registerProxyEnrichers, makeProxyCall -->

**filter-update:** Engine sends a batch of routes to a plugin's filters, and
the plugin answers one verdict per route, in request order: accept, reject,
modify (sets, or a raw rewrite), or withdraw. A reject only
stops the route, so a version the peer already holds stays; a withdraw removes
it: on import the route is treated as withdrawn by the peer that sent it, on
export it is withdrawn toward the destination. Every route of the UPDATE is
withdrawn, as two UPDATEs when its MP_REACH_NLRI and MP_UNREACH_NLRI name two
families. A withdraw carries no sets. Each route names its filter so
the plugin can dispatch to the correct handler. The worker takes the routes
queued when an exchange starts, at most `rpc.FilterUpdateBatchMax` (64), and
splits that FIFO group into contiguous batches whose serialized request frames
fit `rpc.MaxMessageSize` (16 MiB), including the JSON envelope, method and
request ID. A route too large to fit even alone fails only itself under its
filter's `on-error`; its queued neighbors still run. Each emitted batch has one
`rpc.FilterUpdateTimeout`; a failed batch (timeout, error, a reply with the
wrong number of verdicts) applies each route's filter `on-error`, and a verdict
carrying `error` fails its route alone. One worker per plugin connection owns
the exchanges, and each caller waits for its own verdict, so borrowed route
bytes stay valid until its exchange completes (protocol.md, "The filter-update
batch").
The first route starts the worker under the same lifecycle lock that closes
the connection. A connection closed before its first route refuses it without
allocating the batch queue or scratch or starting a worker. Close prevents
further admission; an initialized worker settles every admitted route before
it exits, and later calls refuse with a connection-closed error.
<!-- source: internal/component/plugin/ipc/filter_batch.go -- SendFilterUpdate, filterBatcher -->
<!-- source: internal/component/plugin/ipc/filter_batch_bytes.go -- exchange, frameFits, filterBatchWireFixedSize -->
<!-- source: pkg/plugin/rpc/filter_batch_size.go -- FilterUpdateInput.JSONSizeBound -->
The request carries `fields`: one JSON object holding
exactly the route fields the filter declared in `reads`, keyed by vocabulary
field name, each value in the UPDATE event's `attr` shape. A declared field the
route does not carry is left out, `fields` is absent for a filter that declared
no route field, the announced NLRI are the `prefix` field and the withdrawn
ones the read-only `withdrawn` field, and `raw` is sent only to a filter that declared the `raw` read.
The same object carries the session facts the filter declared (`peer-name`,
`peer-as`, `ibgp`, `rs-client`, the `source-peer-*` facts on export, and the
rest of the table), so a filter learns which peer a route belongs to without a
header member of its own. Route metadata keys (`src-role`, `src-peer-role`,
`stale`, `rebuilt`) are fields too, and a filter at `bgp import` that
declared one may answer a `set` of it, which lands in the route's metadata map
(`docs/architecture/meta/README.md`).
A field Ze cannot read (a malformed attribute) applies the filter's `on-error`
instead of sending the message. The value shapes are in
`docs/plugin-development/protocol.md`, "The Route Fields of filter-update".
Every plugin filter is a member of the policy chain (`policychain`): it is
sent the fields as the earlier members left them, and answers its changes in
`sets`, which fold with the other members' changes (protocol.md, "The Sets of a
filter-update answer").
<!-- source: internal/component/bgp/policychain/external.go -- judgeExternal -->
<!-- source: internal/component/bgp/policyview/fields_json.go -- AppendFields, CheckFieldWriter -->

<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnFilter, answerFilterBatch -->
<!-- source: internal/component/plugin/server/server.go -- CallFilterUpdate -->
<!-- source: internal/component/bgp/reactor/filter_chain.go -- filter-update caller -->

### Plugin to Engine

<!-- source: pkg/plugin/sdk/sdk_engine.go -- Plugin engine RPC methods -->
<!-- source: pkg/plugin/rpc/types.go -- Method* constants and engine RPC payload types -->

| Method | Input | Output | Description |
|--------|-------|--------|-------------|
| `update-route` | `UpdateRouteInput` | `UpdateRouteOutput` | Inject route to peers |
| `forward-cached` | `ForwardCachedInput` | - | Forward cached UPDATEs to destination peers |
| `release-cached` | `ReleaseCachedInput` | - | Release cached UPDATEs and do not forward them |
| `relay-stored-route` | `RelayStoredRouteInput` | - | Relay stored wire routes to one established peer |
| `select-routes` | `SelectRoutesInput` | `SelectRoutesOutput` | Settle selected output for one captured destination session |
| `select-refresh` | `SelectRefreshInput` | `SelectRefreshOutput` | Settle an enhanced marker on the same captured selected-output destination |
| `cancel-request` | `CancelRequestInput` | `CancelRequestOutput` | Cancel and join an opt-in request on the same authenticated connection |
| `route-install` | `RouteInstallInput` | `RouteInstallOutput` | Insert a batch of computed routes into the engine Loc-RIB (forked route-installing plugin) |
| `route-remove` | `RouteRemoveInput` | `RouteRemoveOutput` | Withdraw a batch of routes from the engine Loc-RIB (forked route-installing plugin) |
| `route-metrics` | `RouteMetricsInput` | `RouteMetricsOutput` | Read recursive next-hop costs and the engine Loc-RIB revision |
| `inject-wire-route` | `InjectWireRouteInput` | - | Inject a raw BGP UPDATE body into the RIB |
| `batch-validate` | `BatchValidateInput` | `BatchValidateResult` | Apply a batch of RPKI validation decisions |
| `resolve-dns` | `ResolveDNSInput` | `ResolveDNSOutput` | Resolve a name through the engine's single DNS resolver, so a plugin never builds a second one |
| `state-get` | `StateInput` | `StateOutput` | Read an owned registered runtime key |
| `state-put` | `StateInput` | `StateOutput` | Persist an owned key with durable acknowledgement |
| `state-remove` | `StateInput` | `StateOutput` | Remove an owned key |
| `state-list` | `StateInput` | `StateOutput` | Enumerate owned keys beneath a prefix |
| `state-increment` | `StateInput` | `StateOutput` | Atomically advance a durable big-endian uint32 counter |
| `metrics-publish` | `MetricsPublishInput` | `MetricsPublishOutput` | Declare metrics and publish their changes since the last publish (see "Plugin metrics" below) |
| `report-issue` | `ReportIssueInput` | - | Raise or clear one problem on the daemon's report bus under the plugin's own name (see "Plugin problems" below) |
| `dispatch-command` | `DispatchCommandInput` | `DispatchCommandOutput` | Inter-plugin command |
| `dispatch-command-args` | `DispatchCommandArgsInput` | `DispatchCommandOutput` | Exact inter-plugin command with pre-tokenized args |
| `emit-event` | `EmitEventInput` | `EmitEventOutput` | Queue an event, or join captured application with `await-applied` |
| `subscribe-events` | `SubscribeEventsInput` | - | Subscribe to events |
| `unsubscribe-events` | - | - | Unsubscribe from events |
| `decode-nlri` | `DecodeNLRIInput` | `DecodeNLRIOutput` | Decode NLRI via registry |
| `encode-nlri` | `EncodeNLRIInput` | `EncodeNLRIOutput` | Encode NLRI via registry |
| `decode-mp-reach` | `DecodeMPReachInput` | `DecodeMPReachOutput` | Decode MP_REACH_NLRI |
| `decode-mp-unreach` | `DecodeMPUnreachInput` | `DecodeMPUnreachOutput` | Decode MP_UNREACH_NLRI |
| `decode-update` | `DecodeUpdateInput` | `DecodeUpdateOutput` | Decode full UPDATE |

All methods are prefixed with `ze-plugin-engine:`.

#### Selected-route output

##### Original source leases

<!-- source: pkg/plugin/rpc/source_lease.go -- SourceLeaseInput, SourceLeaseOutput -->
<!-- source: pkg/plugin/sdk/sdk_source_lease.go -- SourceLease -->
<!-- source: internal/component/bgp/reactor/register_source_lease.go -- source lease registration and generation cleanup -->

Process-generation cleanup receives the server that owned dispatch together
with the exact ended process. It revokes and joins original-source writes
before retiring that incarnation's current selected output through the
server's own reactor. It never looks up an owner by its configured name. A
failed output removal retires its captured destination session rather than
leaving the prior advertised frontier for a replacement to inherit.

`SourceLease(ctx, input)` calls the BGP owner's registered
`ze-bgp:source-lease` operation. It uses the generic native bridge dispatcher
internally and the same evaluator over JSON externally. The input names an
engine-issued `source-receipt`, its stable `peer-owner`, an absolute `deadline-ms`, and an explicit
numeric action: `1` acquire, `2` retain, or `3` release. The answer echoes the
receipt, peer owner and action only after that action has completed. Missing, null,
unknown and mismatched answers are refused. A receipt identifies an original
source Session; knowing its number grants no permission.

`peer-owner` is the engine's existing Peer incarnation identity, allocated once
when that Peer is created. It is the same value as
`SourcePolicyIdentity.PeerOwner` and the original received event's `peer-owner`.
It survives reconnects of that Peer; the reconnect gets a new `source-receipt`.
`connection-id` independently identifies the physical Session from its birth,
before either OPEN. It remains unchanged when the initial replay receipt
rotates at establishment and crosses both event transports as a read-only
fact. On a sent UPDATE, root `connection-id` names the destination connection.
The engine-reserved `route-meta.source-connection-id` decimal uint64 string
and structured `RawMessage.SourceConnectionID` instead retain the original
received source connection; route policy cannot supply or replace that scalar.
Local and API origins have no received connection. The original source-receipt snapshot and RIB source reference retain it;
a replacement UP never changes the identity of old retained routes.
Removing and recreating a peer creates another owner. Addresses, AS numbers and
TCP ports are metadata, never substitutes for this identity.

Acquire checks the exact original Peer's resolved attachment and the caller's
authenticated runtime dissemination declaration. The same configured process
must explicitly hold the complete declared source receive set. Grants from a
different process running the same plugin, a derived attachment, or a different
peer with the same address do not complete it. Retain extends an existing
lease through physical source DOWN; the engine chooses no GR or LLGR duration.
Acquire and Retain require a nonzero peer owner matching the receipt's captured
original Peer before changing authority. The SDK binds the answer to all three
requested values, with no floating-point identity conversion.
Release confirms that no authority remains for the authenticated selecting
incarnation at that receipt. It needs no receive grant, so cleanup can finish
after that grant is removed. An existing owned lease is revoked first and its
outstanding writes are canceled and joined before the answer. A canceled wait
leaves it revoked and discoverable until those writes finish. Their completion
can remove the lease before a retry arrives; an absent own lease or an unknown
positive receipt therefore already satisfies Release. This creates no lease,
keeps no completion history and never changes another owner's authority.
Release accepts zero peer owner when the original identity is unrecorded. A
nonzero owner is caller correlation, not an engine attestation: if the receipt
still exists, a mismatched owner is refused without revoking it. An unknown
receipt is already absent and creates no identity or completion history.
Acquire and Retain still require their original receipt, grant and ownership
checks. A zero receipt is invalid for every action.

The private reversible grant transaction pins only the existing leases of one
exact original Peer and selecting Process incarnation. The coordinator must
first cut that source's receive grant and settle admitted receive delivery;
prepare can then retain the original session facts without retaining any route
payload. Apply permanently cancels those old lease contexts and joins admitted
writes. It does not advance the peer's policy epoch or end its destination role.
Rollback requires the same authenticated Process and a restored complete,
explicit source grant. It issues fresh lease contexts from those pins, including
valid retained sessions already DOWN; an old queued write remains revoked.
Pins remain until the owner has acknowledged abort or accepted finalization.
Ordinary acquire cannot use a closed delivery reference to resurrect a lease.
This primitive alone does not purge plugin candidates or acknowledge a reload:
the owner lifecycle must also settle candidate removal, re-election and output.
<!-- source: internal/component/bgp/reactor/source_receipt_transaction.go -- prepareGrant, apply, restore, finish, finalize -->

The Process caches one selected-owner token while live. Generation cleanup
uses that same token after Stop or engine exit, rather than constructing a new
live identity. Every lease of the ended incarnation is revoked before cleanup
joins its writes. A same-name successor has a different token. A Process is
started once; restart creates a new Process with fresh delivery and cleanup
state.

That token also captures the exact connection's current-selection reader once.
Final writers invoke the captured reader without taking the process manager's
lock. A manager can therefore stop and join an incarnation while its final
writer finishes; neither operation needs a name lookup held by the other.
The token checks its original lifetime before and after the callback and
validates the echoed scope. A lifetime-only token with no reader refuses the
query.

The bound token also holds a Process-owned cancellation context. Stop cancels
it through the existing process context tree. Internal engine exit cancels only
the selected context before publishing `EngineDone`; it does not turn an engine
failure into an operator Stop. Source leases derive their writer contexts from
that tree. A typed flush therefore stops before bridge dispatch is joined,
independently of the generation cleanup that subsequently releases its lease.

Owner engine operations may declare an authenticated cancellation scope.
`EngineOperation.CancellationScope` validates the live resource before the
connection reader dispatches work. `NewEngineRequestScope` keeps the resource
key comparable and binds it to one method and deadline. The existing
connection-local request store enrolls and joins the call; source leases add
no cancellation inventory of their own.
<!-- source: internal/component/plugin/server/register_engine_op.go -- EngineOperation, NewEngineRequestScope -->
<!-- source: internal/component/plugin/process/selected_owner.go -- SelectedOwner, SelectedOwnerForCleanup -->

##### Current selection validation

<!-- source: pkg/plugin/rpc/current_selection.go -- current-selection DTO and frame bounds -->
<!-- source: pkg/plugin/sdk/sdk_current_selection.go -- OnCurrentSelection and independent worker -->

An owner registers `OnCurrentSelection` before `Run`. Stage 1 derives
`current-selection` from that registration; the declaration grants no peer
permission. The engine calls `ze-plugin-callback:current-selection` on the exact
selecting connection, so a similarly named plugin or a later process incarnation
cannot answer for it. Both `MuxConn.CurrentSelection` and
`DirectBridge.CurrentSelection` use the same bounded contract.
The engine's `PluginConn.SendCurrentSelection` selects those transports on that
exact connection and never falls back to ordinary queued callbacks after bridge
cutover. The typed bridge shares delivery admission, readiness and handler-presence
checks between its two snapshot operations. It checks readiness before reading
the installed handler; each operation retains its own deadline, missing-handler
error and output validator.
<!-- source: internal/component/plugin/ipc/current_selection.go -- SendCurrentSelection -->
<!-- source: internal/component/plugin/ipc/read_snapshot.go -- callReadSnapshot -->
<!-- source: pkg/plugin/rpc/bridge_read_snapshot.go -- bridgeReadSnapshot -->

Each request carries the destination, its exact session token, an absolute
deadline and native keys with the requested revisions. Each answer echoes that
scope and every ordered key/revision, the current global selection cut and an
explicit presence (`1` present, `2` absent). A present result carries its nonzero
current revision, at most the cut. An absent result carries current revision
zero; cut zero is valid for an empty fresh scope. No candidate route payload is
returned. Unknown presence, missing fields and incomplete or mismatched echoes
fail validation. A key still current at revision 3 remains valid under cut 500;
an absent key is rejected using the current cut without retaining engine
tombstones. A request newer than the published cut is not proof of current
authority and must fail selected admission.

The callback reads only the owner's published desired-selection snapshot. It
must not call the engine, perform I/O, or acquire peer or election locks. It runs
on one long-lived worker per connection, independently of the ordinary callback
and execute-command queue. Wire routing occurs in the Mux reader before that
queue; typed routing enters the same worker directly. This lets a source-policy
or retention handler await selected dispatch while its current-state lookup
still completes. Both request and widest reply must fit `MaxMessageSize`;
`CurrentSelectionExchangeSize` lets producers split larger queries. Queue
admission is bounded. A full typed queue returns an error; a full wire queue
ends the connection rather than blocking its sole reader or silently dropping
authority checks. Stop cancels and joins the worker and refuses queued work.

<!-- source: pkg/plugin/rpc/selected.go -- selected input and final receipts -->
<!-- source: pkg/plugin/sdk/sdk_selected.go -- SelectRoutes -->
<!-- source: internal/component/plugin/server/dispatch_selected.go -- authenticated dispatch -->

`SelectRoutes(ctx, input)` submits one destination batch through the typed
bridge or the same registered socket operation. It is separate from
`relay-stored-route`: a selected revision can replace output from a different
original source, while the legacy relay retains its source-owned semantics.
The engine derives the caller's process incarnation from the authenticated
connection. No caller-supplied process name, route metadata or source receipt
grants send permission or overrides an explicit forwarding owner.

The input carries `destination`, the exact UP event's `initial-replay` token
as `destination-session`, and `routes`. That token is required for initial and
live output. Each entry has `family`, `key-nlri` (base64 native advertisement
bytes with the source ADD-PATH identifier removed), and a nonzero uint64
`revision`. The engine derives the registered semantic identity, including
families that are not IP prefixes. Revisions increase per native key and
admitted scope within the selecting process incarnation; exhaustion requires
a new incarnation and replay boundary rather than wrapping.

The numeric `kind` selects one payload:

| Kind | Payload |
|------|---------|
| `1` received | `route` (`StoredRoute`), nonzero engine-issued `source-receipt`, `source-add-path`, explicit `stale-level`, and `retained` when the owner authorizes retention |
| `2` origin | `route`, nonzero engine-issued `origin-receipt`, and fresh `stale-level`; no received-source or retention authority |
| `3` absent | No route or source/origin receipt; retire this selected stream's previous output |

The engine has a separate private origin issuer domain. Its owned build value
keeps native NLRI, explicit path-ID presence, original attribute framing,
original attribute encoding context, late-bound next-hop choice and origin-AS intent. Its source value keeps the
original protocol/instance and accepted metric/distance override without
reading process-global preferences or classifying a route from BGP ORIGIN.
An issuer ID is a value for the existing `origin-receipt` field, not a received
source receipt or proof that the deciding plugin retained a candidate.

The issuer checks the original producer's actual send grant and the current
ordinary dissemination owner. Its current index is per producer/source/native
identity and configured Peer owner. Replacements revoke earlier IDs; only
records with actual in-flight writes remain beside current records. A write
guard checks the exact deciding process incarnation, original Peer and current
destination Session publication/token. Source-owned withdrawal and explicit
producer, Peer and store-close barriers seal matching active intake before
joining outstanding writes. Each issue enrolls before permission preflight and
remains enrolled through commit/refusal. The commit and revocation use the
same leaf mutex, so a preflighted issue cannot insert after a completed revoke,
even if the grant is restored before that issue resumes. Enrollments exist only
for actual active calls; no revoked-ID or historical scope deny map is retained.
An interrupted join does not restore authority or erase its real obligation.
A permanent lifecycle owner must publish the actual grant removal or original
Peer/process stop before its matching revoke, then join before acknowledging it.
Fresh calls under an explicit regrant or a new Session/Peer remain independent.

The registered intake callback and Announce/Withdraw adapter use this engine
domain. An issued origin passes through the existing final writer. The writer
reconstructs its owned build and applies local export once. The caller's
`StoredRoute` echo cannot change attributes or next hop. Unissued and stale
receipts refuse output.
<!-- source: internal/component/bgp/reactor/selected_origin.go -- selectOrigin -->

The reactor constructs its issuer before ingress and owns joined runtime
barriers. It does not itself install a candidate provider. Local admission
preserves no-RIB direct origination, non-established-peer scopes, existing
keep-on-exit behavior and RS/RR paths. Candidate mutation uses the registered
intake and acknowledged cancellation. The `current-selection` callback remains
read-only.
<!-- source: internal/component/bgp/reactor/origin_build.go -- newOriginBuildValue, newOriginIdentity -->
<!-- source: internal/component/bgp/reactor/origin_producer.go -- newProcessOriginProducer, newOriginSource -->
<!-- source: internal/component/bgp/reactor/origin_receipt.go -- originReceiptStore, issue -->
<!-- source: internal/component/bgp/reactor/origin_write.go -- admit, current, withdraw, revokeOwner, close -->
<!-- source: internal/component/bgp/reactor/selected_origin.go -- selectOrigin -->

Received FlowSpec entries additionally carry `flowspec-redirect-refused` when
the deciding owner authorizes that current revision without its redirect
action. The engine refuses the flag on origins, absences and non-FlowSpec
families, then uses the existing egress action-removal treatment. It does not
call an ordinary receive-validation handler while that handler may be joining
the selected writer. The selecting snapshot must bind this projection to the
same current revision it publishes.

`stale-level` is `1` fresh, `2` stale or `3` LLGR; zero is unrecorded and is
allowed only on absence. Received `route` preserves the original message,
source-policy epoch, NLRI framing and path identifier, including ADD-PATH zero.
Neither reconnect nor replay restamps the original policy epoch. The generic
boundary validates payload shape and owns its snapshot. The reactor must
authenticate issued receipts, source retention/revocation, effective role,
native identity and destination session before final admission. A reactor
without selected-output support returns an error.

Both the encoded request and its widest complete final answer must fit
`rpc.MaxMessageSize` before admission. `rpc.SelectRoutesExchangeSize` measures
the larger frame with the widest correlation ID, current revision and SDK
deadline; an absence answer can be larger than its request. The existing
`SelectRoutesFrameSize` measures the request alone. Route count alone cannot
bound either frame. `SelectRoutesExchangeSizes` exposes the separate request
and response budgets. `SelectRoutesItemSizes` uses the same envelope authority
for cacheable array-member budgets; callers add the empty nonnil-array
envelope and one comma between members, reserving the widest requested revision
when a cached item can be reused after revision changes. Callers keep input
unchanged until return. The result contains one ordered identity/revision echo per input and
the writer's `current-revision`. Numeric outcomes are `1` written, `2`
suppressed, `3` absent, or `4` stale revision. Suppression and absence confirm
retirement of the previous selected output. Missing, partial, unknown or
mismatched results fail the operation.

Success is final writer settlement, not queue admission or a later sent-event
callback. The RIB scheduler must also join its queued and in-flight work to
the process's applied-delivery settlement barrier before `request quiesce`
can confirm it; a final writer callback alone does not drain unpublished work.
Source-policy and retention lifecycle barriers use that same scheduler
settlement, rather than an independent completion inventory.

The current-selection server facade compares both process lifetime channels
with the authenticated owner before and after querying its connection. A
same-name replacement is a different owner. The reactor compares the reply's
current desired revision, allowing a still-current key at revision 3 beneath a
later scope cut of 500. An absent key keeps no historical revision: the scope
cut and the serialized destination writer fence a stale reappearance.
<!-- source: internal/component/plugin/server/selected_current.go -- CurrentSelection, selectedConnection -->
<!-- source: internal/component/bgp/reactor/selected_current.go -- selectedCurrentRevision -->

The selecting owner brackets its replay through `SelectRefresh(ctx, input)`
(`ze-plugin-engine:select-refresh`). The input narrows the destination by
address, original `peer-owner`, exact `destination-session`, one registered
negotiated `family`, action `1` begin or `2` end, and bounded `deadline-ms`.
The native owner adapter and public RPC use the same evaluator and
authenticated process token. It uses the selecting owner's update grant and
effective destination role, without acquiring normal refresh-request permission.
A normal ROUTE-REFRESH request remains a separate operation.

The complete receipt echoes destination, peer owner, Session, family and action.
Outcome `1` means the full marker reached that exact connection; outcome `2`
means enhanced markers were not negotiated. Zero, a missing receipt or a
mismatched echo fails the request. A replacement Session or configured alias
cannot satisfy an old request. Process stop cancels and joins a blocked final
write. Marker and route calls share the destination's selected-stream lock.
A complete Begin also records the exact selecting incarnation's unfinished
family on that Session. A complete End clears only its own incarnation's
obligation. The existing process-generation cleanup retires and joins the
original connection if the process ends between marker calls, including an
empty route replay; it does not ask the expired SDK to authorize End. Completed
Begin/End connections and unrelated Sessions remain usable. Source-specific
route cleanup leaves a healthy selecting process's refresh intact. A replacement
Session inherits no pending markers, and another process incarnation cannot
overwrite or complete an older Begin on the same connection.
Retirement joins the first captured socket close's completion claim. Calling
`net.Conn.Close` again is not proof that an earlier close has finished.

The selecting worker MUST await Begin, then all replay route receipts, then
End. The marker operation does not enumerate routes or settle a queued replay.
When our own connection OPEN advertised GR, Begin refuses until this connection has actually emitted that
family's complete End-of-RIB. Initial-sync flags and before-send EOR claims
cannot substitute for accepted EOR bytes; manual EOR therefore keeps its control.
The completion set belongs to one observed connection epoch and contains only
that Session's negotiated OPEN family intersection, including classic IPv4
without MP negotiation. Shared encoding IDs do not grant family authority.
MP EOR evidence requires a sole optional, non-transitive MP_UNREACH with no
withdrawn routes, using either legal attribute-length encoding. Malformed
raw diagnostic output remains sendable and cannot satisfy the EOR gate.
The sender GR fact comes from local OPEN publication, independently of remote GR.
<!-- source: internal/component/bgp/reactor/selected_refresh.go -- SelectRefresh -->
<!-- source: internal/component/bgp/reactor/selected_refresh_owner.go -- selectedRefreshPending, hasSelectedRefreshOwner -->
<!-- source: internal/component/bgp/reactor/selected_refresh_eor.go -- complete, completeEOR, eorFamily -->
<!-- source: pkg/plugin/rpc/selected_refresh.go -- SelectRefreshInput, SelectRefreshOutput -->
<!-- source: internal/component/plugin/server/dispatch_selected_refresh.go -- selectRefresh -->

The SDK derives `deadline-ms` from the caller's context, with a thirty-second
ceiling for one batch. The socket reader validates and enrolls the request's
authenticated resource before starting its handler, so a later cancellation
cannot overtake admission. A selected batch needs its destination's actual
`update` send grant, and only one batch may be live for that destination.
The final writer still rechecks all authority and the destination session.
Other opt-in operations validate an existing owned resource before enrollment;
the cancellation table holds only those live requests, not route state or a
completion history.

On cancellation, `MuxConn.CallRPCCancellable` sends `cancel-request` with the
original `request-id` and `method`, then waits up to five seconds for the
handler and writer to join. Outcomes are `1` settled or `2` no longer active
on this connection. Unknown or non-opt-in methods, zero IDs, self/future IDs
and a method different from the live request are refused. A completed entry
is removed rather than retained as a tombstone. Another process connection
cannot cancel it. The read loop cancels all its opt-in contexts before joining
handlers on EOF, even while the process itself still exists.

Correlation IDs remain reserved for every live request through its response
attempt, including ordinary methods, cancellation controls and refused input.
A frame reusing any live ID is connection-fatal: the daemon sends no collision
reply under that occupied ID, cancels admitted contexts, closes the stream and
joins the handlers. Such a reply could otherwise release the original caller
while its writer still owned work. Completed IDs leave no stored history, and
valid concurrent requests need not arrive in ID allocation order.
Fatal transport closure precedes request cancellation and the filter queue's
admission lock, so a waiting control cannot emit a reply under the corrupted
identity and filter admission cannot keep that stream open. Normal connection
closure keeps its existing order.
<!-- source: internal/component/plugin/ipc/rpc.go -- PluginConn.AbortRuntime -->

A transport failure or missing cancellation receipt is an explicit
`CancellationUnsettledError`; a lifecycle barrier must fail instead of claiming
that remote work joined. Cancellation during a partial request write closes
the stream, because its incomplete frame cannot be reused. Typed calls carry
the caller's context directly and have the same final-writer obligation.
Ordinary non-opt-in RPC methods keep their existing cancellation behavior.
<!-- source: pkg/plugin/rpc/cancel.go -- cancellation receipts, SelectedCallContext -->
<!-- source: pkg/plugin/rpc/mux_cancel.go -- MuxConn.CallRPCCancellable, cancelAndJoin -->
<!-- source: pkg/plugin/rpc/write_cancel.go -- Conn.armWriteCancellation -->
<!-- source: internal/component/plugin/server/request_cancel.go -- runtimeRequests, selectedCancellationScope, opCancelRequest -->
<!-- source: internal/component/plugin/server/request_runtime.go -- serveRuntimeRequests -->

Stored-route replay preserves `source-owner`, `source-connection-id` and
`source-receipt` as lossless uint64 strings. The engine resolves recorded source
facts through that original receipt, never through a peer at the same address.
The receipt belongs to one source Session, not one route or UPDATE: checking it
must not invent a per-route retention decision. Original policy admission and
current route eligibility remain separate checks. A malformed or partial
recorded source tuple refuses reconstruction. Explicit unowned operator input
keeps its separate manual contract; losing part of a recorded tuple never turns
it into manual input. Cached forwarding likewise uses the cached original Peer,
Session receipt, encoding and generation before examining destinations.
<!-- source: internal/component/bgp/reactor/relay_original_source.go -- originalRelaySource, originalCachedSource -->

The existing `RelayStoredRoute(ctx, RelayStoredRouteInput)` SDK method carries
an optional `destination-owner` and `destination-session` tuple. Both scalars
use lossless uint64 JSON strings. An explicitly present null, zero-only or
partial constraint refuses; it never becomes an unbound operator request.
The bounded deadline is positive, live and at most thirty seconds ahead. Both omitted
preserves explicit unbound selectors; both supplied narrow to that exact Peer
and UP token. `ForwardCached(ctx, ForwardCachedInput)` carries the complete
`targets` set described below. A partial or stale tuple is never rebound by
address or TCP port preference.
The SDK adds the positive absolute `deadline-ms` for bound work. Typed and
framed callers use the same evaluator, ordinary send grant, original source
facts and shared final socket writer. Bound success carries `completed: true`
and the request's exact `deadline-ms`; nil, enqueue-only, false or mismatched
answers cannot complete replay or cleanup. Framed cancellation uses the existing
connection-owned cancel-and-join mechanism.

Live legacy replay enrolls a transient write in the existing source-receipt
lease table. It retains that write's original generation even if an explicit
acquisition later upgrades the shared lease for retention. The transient row
leaves when its last actual write settles, and grant rollback does not restore
completed transient authority. Retained replay requires an already acquired
lease of the actual authenticated process, plus the caller's per-route retained
membership; the Session-wide receipt alone does not supply that membership.

A receipt-free owned withdrawal carries its original `source-owner`,
`source-connection-id` and native PrefixOnly NLRI plus original path ID, including
zero. It may retire only a current acknowledged SENT slot matching those facts
and its nonzero original received generation. Its current SENT record
keeps the physical source identity, with no completed-source history. A newer
physical source, newer named generation, selected stream, or unknown original
survives. Such a request cannot announce, use a sibling's source facts, or turn
into manual input. SourceWire input without its original Session receipt is
refused because its ADD-PATH framing cannot be guessed.
<!-- source: pkg/plugin/rpc/cached_bound.go -- ValidateCachedWrite, CachedWriteOutput -->
<!-- source: internal/component/plugin/server/dispatch_cached_bound.go -- forwardCachedRequest, relayStoredRouteRequest -->
<!-- source: internal/component/bgp/reactor/relay_bound.go -- boundForwardRequest, boundForwardStage -->
<!-- source: internal/component/bgp/reactor/source_receipt_live.go -- beginLive -->
<!-- source: internal/component/bgp/reactor/relay_retire_original.go -- retireOriginalStoredRoute -->

The reactor's source-receipt storage separates three lifetimes. An issued
receipt owns the immutable forwarding facts of its original source session:
its encoding, AS and router identities, peer classification and filter facts.
The session's event-delivery reference lasts through its applied DOWN, so an
earlier queued receive callback can acquire the receipt after the connection
has physically closed. That reference permits storage admission, not output.
The final writer still requires either the captured live generation or an
existing selecting owner's retained lease. A different process incarnation
cannot acquire that old source after the delivery reference closes.
Configured-source removal also refuses acquisition, even while that reference
is open. Process liveness is checked under the admission lock shared with
owner cleanup, so an earlier check cannot authorize insertion after cleanup.

The deciding plugin owns retention and MUST revoke its lease on expiry,
purge or removal. Revocation first refuses new writes and cancels captured
ones, then joins their final completion. A canceled wait leaves the lease
revoked and its outstanding writes owned; process cleanup can still find
and join them. Each write MUST complete after its final flush or cancellation,
and copied completion tokens are idempotent. The writer rechecks original
policy admission, process lifetime and source removal at its socket boundary;
neither a reconnect nor a retained lease replaces those checks.

This table stores facts and lifetime references, never candidates or GR
durations. It retains only open session-delivery references, current leases
and actual in-flight writes. Their final release deletes the entry; its
monotonic receipt clock leaves no per-session history. The table is the
engine-side lifetime primitive; selecting-owner grants and the selected writer
are separate protocol boundaries. Its clock is daemon-wide: replacing the BGP
reactor cannot reuse a receipt held by a deciding process.
<!-- source: internal/component/bgp/reactor/source_receipt.go -- sourceReceiptStore, sourceReceiptFacts, liveSourceReceipt, sourceReceiptLease, sourceReceiptWrite -->

The selected Session captures its source after negotiation and dynamic-AS
resolution, before announcing UP. Its immutable callback stamps that receipt on
received UPDATEs, including ingress-generated continuations and the retained
cache value. The notifier is bound to the exact configured Peer, so a sibling
using another port at the same IP cannot supply its facts. Its delivery reference
closes after the ordered DOWN finishes; a fallback registered before publication
also closes it on an early failure. Original sent provenance is read from the
writer's `SentOrigin`, never from the destination's or source's current Session.
Locally originated and reconstructed unrecorded messages keep an explicit zero.
<!-- source: internal/component/bgp/reactor/source_session.go -- captureSourceReceipt, sourceMessageCallback, closeSourceReceipt, messageReceiverForPeer -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- runOnce capture before announceEstablished -->
<!-- source: internal/component/bgp/reactor/peer_session_end.go -- sessionDown ordered receipt release -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyMessageReceiverFrom, notifyOneMessage -->

`update-route` also carries `source-policy-epoch` as a dedicated numeric
provenance field on `UpdateRouteInput`, outside user `meta`. A retained
received-origin producer uses `sdk.Plugin.UpdateRouteRequest(ctx, input)` to
preserve the route's original epoch through command dispatch on both transports.
Zero means unrecorded, never permission to load the source's current epoch.
Locally originated routes retain their separate authority; this field does not
turn a local injection into a received-source route.
<!-- source: pkg/plugin/rpc/types.go -- UpdateRouteInput -->
<!-- source: pkg/plugin/sdk/sdk_engine.go -- UpdateRouteRequest -->
<!-- source: internal/component/plugin/server/dispatch_registry.go -- opUpdateRoute -->


#### Original redistribution admission lookup

`OnOriginAdmissionLookup` registers a read-only accepted-record handler before
`Run`. Stage 1 derives `origin-admission-lookup` from that handler. The declaration
grants no publisher, source or peer permission.
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->
<!-- source: pkg/plugin/sdk/sdk_origin_admission_lookup.go -- OnOriginAdmissionLookup -->

The engine queries its captured original accepting connection with
`ze-plugin-callback:origin-admission-lookup`. Each query names one opaque
`admission-id` and a canonical `ipv4/unicast` or `ipv6/unicast` family and prefix.
It also names the concrete destination, Peer owner, operation and deadline. A reply
echoes that identity exactly. Missing or stale records return an error. A
retiring record answers only its exact withdrawal, never another announcement.
<!-- source: pkg/plugin/rpc/origin_admission_lookup.go -- OriginAdmissionLookupInput, OriginAdmissionLookupOutput, Validate -->

The snapshot preserves original protocol, BGP source class, instance, table and
source metric. It also carries distance presence and value, accepted next hop,
origin ASN, communities, tag and optional wire MED. The original source metric
is separate from transformed `metric` and `has-metric`. Explicit zero distance
and rule generation remain valid. Peer scope is explicit all or exact. A
missing scope or peer never implies a wildcard.
<!-- source: pkg/plugin/rpc/origin_admission_lookup.go -- OriginAdmissionSnapshot, OriginAdmissionPeerScope, OriginAdmissionBGPClass -->

Both transports validate complete values and frame bounds. A separate worker
serves at most one active and sixteen queued requests, independently of ordinary
commands and origin mutation. The handler reads a published snapshot, honors
cancellation and performs no engine call or I/O. Stop refuses new work, cancels
the handler and joins the worker.
<!-- source: pkg/plugin/sdk/sdk_origin_admission_lookup.go -- originAdmissionLookupWorker, startOriginAdmissionLookupWorker -->
<!-- source: pkg/plugin/sdk/sdk_read_snapshot.go -- runReadSnapshotWorker, serveReadSnapshot, startReadSnapshotWorker -->

An expired read keeps its strict work deadline. Its refusal uses a fresh bounded
response budget from the same process lifetime, so expiry leaves the connection
usable. Process stop still cancels that reply. An actual writer failure closes
the unusable stream.
<!-- source: pkg/plugin/sdk/sdk_origin_admission_lookup.go -- answer, serve -->

The snapshot carries at most 65535 communities. The shared validator and YANG
declare the same cardinality, separate from the encoded-frame bound.
<!-- source: pkg/plugin/rpc/origin_admission_lookup.go -- OriginAdmissionCommunityMax, Validate -->
<!-- source: internal/core/ipc/yang/ze-plugin-callback.yang -- origin-admission-snapshot -->

The reply describes accepted data. It is neither an origin receipt nor send
permission. The caller MUST retain the exact original process and separately
verify publisher, accepted-record lifetime, grants and immutable route build.
A published bridge never falls back to a socket if its slot is unavailable.
<!-- source: internal/component/plugin/ipc/origin_admission_lookup.go -- SendOriginAdmissionLookup -->
<!-- source: pkg/plugin/rpc/origin_admission_lookup.go -- Validate -->

#### Mutating local-origin intake

A deciding plugin registers `OnOriginIntake` before `Run`. Stage 1 derives
`origin-intake` from that handler. The declaration grants no peer permission.
The callback `ze-plugin-callback:origin-intake` carries the same owned input and
full `UpdateRouteOutput` on typed and framed transports.
<!-- source: pkg/plugin/sdk/sdk_origin_intake.go -- OnOriginIntake -->
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->

Each offer contains one native contribution and its original producer/source
metadata. It preserves original attributes and context, ADD-PATH presence
independently of zero, and the late-bound next-hop policy. An engine-issued
`OriginReceipt` identifies the contribution. No received-source reference
accompanies the offer.
<!-- source: pkg/plugin/rpc/origin_intake.go -- OriginIntakeInput, OriginBuild, ValidateOriginIntakeOutput -->

The source-context handle belongs to the engine. A forked reader treats that
handle as opaque. `as-path-known` and `as-path-asn4` identify the original
AS-path encoding width. Unknown bytes stay opaque, while known two-octet
encoding remains distinct from four-octet encoding.
<!-- source: internal/component/bgp/reactor/origin_intake.go -- originIntakeValue -->

Only the engine issuer binds a receipt to its live original producer, source
identity and concrete Peer. It also binds the deciding process incarnation.
A supplied receipt number alone grants no authority.
<!-- source: internal/component/bgp/reactor/origin_receipt.go -- originReceiptStore, issue -->

`getMatchingPeersSel` resolves the selector and sender grants. At the
Announce/Withdraw batch entry points, an ordinary configured owner with intake
receives the offer before direct output. The deciding plugin owns retention
and candidate election. It serializes each exact source/key/scope and refuses
an older issued generation after a replacement.
<!-- source: internal/component/bgp/reactor/reactor_api.go -- getMatchingPeersSel -->
<!-- source: internal/component/bgp/reactor/origin_intake.go -- offerOriginBatch, offerOriginToPeer, withdrawOriginFromPeer -->

The engine checks receipt freshness before and after the callback. An old
acknowledgment cannot make a revoked or superseded receipt current. A retained
receipt echoes the native key, destination, concrete peer owner and issued
generation. A retained loser keeps `Announced` zero. A bare zero is not acceptance.
<!-- source: internal/component/bgp/reactor/origin_intake.go -- offerOriginBatch, offerOriginToPeer, withdrawOriginFromPeer -->

Withdrawal keeps its completed command count, including a withheld withdrawal.
Tagged and lifetime batches keep the original authenticated process token for
later removal. A timer cannot substitute a same-name replacement. Command
results preserve earlier receipts, warnings and completed counts if later work
fails.
<!-- source: internal/component/bgp/reactor/origin_intake.go -- offerOriginBatch, offerOriginToPeer, withdrawOriginFromPeer -->

A joined announce callback can retain a contribution despite a failed reply,
freshness check or caller-completion check. In that case, the engine revokes
that receipt and sends an exact original Withdraw before the offer call returns.
<!-- source: internal/component/bgp/reactor/origin_intake_compensation.go -- compensateOriginIntake, withdrawOriginIntake -->

Compensation uses a fresh bounded context. It preserves source, key, path-ID,
scope and generation, so a newer candidate stays unchanged. It calls the
captured original connection, including a still-live old process after manager
replacement. The cleanup never resolves a same-name successor.
<!-- source: internal/component/bgp/reactor/origin_intake_compensation.go -- compensateOriginIntake, withdrawOriginIntake -->

Removal of an already-absent exact receipt confirms completed withdrawal and
preserves its replacement. The provider re-elects admitted survivors and joins
their output before it acknowledges removal. A cleanup failure remains in the
returned error. A stopped token alone proves neither provider destruction nor
a joined stop.
<!-- source: internal/component/bgp/reactor/origin_intake_compensation.go -- compensateOriginIntake, withdrawOriginIntake -->

Explicit RS/RR, received maintenance, source recovery and SENT replay keep
their existing paths. A fully registered, running owner without the optional
callback keeps direct origination. API-only/no-RIB peers without an ordinary
deciding owner also keep direct origination.
<!-- source: internal/component/bgp/reactor/origin_intake.go -- originBatchEligible, offerOriginBatch -->

An absent, stopped, pre-Stage1 or still-starting configured owner refuses the
offer before direct output. An unknown capability never establishes legacy
support.
<!-- source: internal/component/plugin/server/origin_intake.go -- OriginIntakeOwner -->

`ze-plugin-callback.yang` declares the callback RPC and its nested input. Its
output uses `update-route-result` from `ze-plugin-engine.yang`. The engine module
also declares Stage-1 `origin-intake` support and the update-route cancellation
deadline.
<!-- source: internal/core/ipc/yang/ze-plugin-callback.yang -- origin-intake -->
<!-- source: internal/core/ipc/yang/ze-plugin-engine.yang -- update-route-result, declare-registration, update-route -->

Intake runs on its own bounded mutation worker. `CurrentSelection` remains
read-only, and ordinary callbacks use separate workers. An intake handler can
call the engine while those workers continue.
<!-- source: pkg/plugin/sdk/sdk_origin_intake.go -- startOriginIntakeWorker, answer, runCancellations -->

The single reader enrolls each request in the same `RequestReceiver` used by
engine selected/source-lease operations. Enrollment precedes any later
`cancel-request`. A separate bounded cancellation worker cancels and joins the
mutation and its response attempt before it acknowledges settlement.
<!-- source: pkg/plugin/sdk/sdk_origin_intake.go -- startOriginIntakeWorker, answer, runCancellations -->

Origin intake cancellation uses the request correlation. It reserves no
exclusive peer/receipt resource because the single mutation worker serializes
announcements and withdrawals.
<!-- source: pkg/plugin/sdk/sdk_origin_intake.go -- originIntakeRequestScope, run -->

A caller can consume a reply before its response attempt returns and releases
enrollment. An exact withdrawal can therefore queue under a different request
ID. Completion still follows the response attempt, so cancellation joins
remote work and the SDK can queue the valid followup.
<!-- source: pkg/plugin/rpc/request_receiver.go -- NewRequestCancellationScope, RequestReceiver.CancelAndJoin -->

Typed cancellation joins the same mutation completion. Stop refuses new
requests, cancels active work and joins all workers. The SDK creates no
goroutine or unbounded queue for an individual request. A captured old process
connection cannot authorize a same-name replacement.
<!-- source: pkg/plugin/sdk/sdk_origin_intake.go -- offer, stopAdmission, supervise -->

New SDK framed update-route requests carry a deadline and use acknowledged
cancellation. Requests without a deadline keep the existing transport contract.
Parallel identical commands retain independent request correlations.
Selected/source-lease operations keep their existing exclusive resource scopes.
<!-- source: pkg/plugin/sdk/sdk_engine.go -- UpdateRouteRequestResult -->

The selected-origin writer reconstructs its issued immutable build and uses
the existing export and socket writer.
<!-- source: internal/component/bgp/reactor/selected_origin.go -- selectOrigin -->
<!-- source: internal/component/bgp/reactor/selected_flush.go -- writeSelected -->

The reactor constructs its origin domain before ingress and joins producer,
Peer, grant, Session and provider lifecycle barriers. Its retained original
lifecycle is described below. A registered RIB candidate provider and shared
cross-origin preference comparison remain required.
<!-- source: internal/component/bgp/reactor/reactor.go -- Reactor.originReceipts -->
<!-- source: internal/component/bgp/reactor/origin_receipt.go -- newOriginReceiptStore -->
<!-- source: internal/component/bgp/reactor/origin_write.go -- revokeOwner, revokePeer, revokeProducerPeer, close -->

`bgp-rib` registers origin intake with its selected dispatcher. At a destination
whose explicit grants resolve that running process as the ordinary selecting
owner, eligible API and admitted redistribution origins enter its scoped
election beside the learned winner. A peer without an ordinary selecting owner
keeps direct origination; an absent or unready configured owner refuses intake.
Configured-route restoration, static/default/plugin origination and
redistribution still require exact original source provenance. SENT state supplies
no candidate authority.
<!-- source: internal/component/bgp/plugins/rib/rib.go -- runRIBPlugin -->
<!-- source: internal/component/bgp/plugins/rib/rib_origins.go -- originIntake, electOriginLocked -->
<!-- source: internal/component/plugin/server/origin_intake.go -- OriginIntakeOwner -->
<!-- source: internal/component/bgp/reactor/origin_intake.go -- originBatchProducer, offerOriginBatch -->
<!-- source: internal/component/bgp/reactor/origin_producer.go -- newOriginSource -->

#### Original local output across process and Peer lifetimes

<!-- source: internal/component/bgp/reactor/origin_runtime.go -- bindLifetime, endOwner, replayOrigin, EndOriginOwner, ReplayOriginOwner -->
<!-- source: internal/component/bgp/reactor/origin_grants.go -- revokeRemovedOriginGrants, parkReboundOrigins, replayPeerOrigins, retireOriginCandidate -->
<!-- source: internal/component/bgp/reactor/register_origin_runtime.go -- originGenerationReady -->
<!-- source: internal/component/plugin/server/generation_ready.go -- ProcessGenerationReady, runProcessGenerationReady -->
<!-- source: internal/component/plugin/origin_retirement.go -- CaptureOriginRetirement, MatchesOrigin, WithOriginRetirement -->

The reactor owns one origin issuer before API admission. Stopping it cancels
and joins actual intake callbacks and final selected writes before its current
original data is released. A permanent Peer removal does the same for that
exact Peer identity. A replacement at the same address cannot reuse a receipt.
An admitted origin can belong to a configured Peer whose TCP session is down.
This does not publish forwarding facts or authorize a write: output still needs
the current captured destination Session and the selecting provider's grants.

An ended producer cannot submit new data. Generic `update-route` announcements
already survive their process's exit, so the engine keeps only their previously
admitted original bytes, provenance and exact Peer scope.

It cancels and joins
the old receipt's work, then offers the same original data under a fresh receipt
to the current deciding process. If that process also ended, the data stays
parked until the configured provider completes its replacement handshake. A
provider restart uses this generic registered generation hook, never a plugin
name branch. Rejected or failed replay is an error. It does not grant direct
output around the provider.

The ExaBGP bridge still owns child-script `on-exit keep`/`withdraw`, including
API-version defaults and restore-configured withdrawals. The bridge's child
ledger is distinct from its long-lived plugin Process. No generic plugin
`on-exit` leaf is introduced here.

An existing accepted timer/tag entry can
retire its exact accepted original, including an operator entry with no Process
token. The registry captures the successful result's concrete Peer and accepted
generation, together with the authenticated sender and named producer token.
Engine-owned reoffers preserve that original lineage and issue fresh write
receipts. Fresh ingress starts a new lineage even from the same Process/key.
An old timer or tag cannot remove that newer input. The captured value never
authorizes a new announcement or a successor's replacement data.

A newly started producer can replace or withdraw a kept predecessor from the
same configured source. It MUST use its own live Process token and current grant.

An accepted direct entry carries no ordinary admission. Its trusted registry
retirement is therefore restricted at the final writer. It cannot withdraw an
entry now owned by a selecting process. This includes local selected routes
that have no received source.

The restriction survives synchronous and queued
maintenance sends, down-peer opQueue entries and policy-removal children. Both
opQueue drains restore the private restriction at the owned final writer.
Established publication does not make old queued cleanup own a later selection.
It supplies no retroactive
receipt or new selection authority. Current direct withdrawals and explicit
manual API withdrawals retain their existing behavior.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- initBatchOwner, queueBehindForwards -->
<!-- source: internal/component/bgp/reactor/session_ownership.go -- allow -->

Published removal of a producer's send grant removes only that source's actual
admitted scope. Loss of the deciding destination grants removes that destination
scope. These barriers cancel and join affected callbacks and writes, then retire
the actual candidate through the deciding plugin so eligible survivors can be
selected. Unrelated scopes remain live.

Transient TCP DOWN keeps admitted local originals for reconnect. Terminal Peer
removal releases them.


An origin operation remains enrolled after the callback reply until issuer
postvalidation and any bounded captured-owner compensation have joined. Scope
revocation cancels the original operation and waits for that composite
completion. A withdrawal stays enrolled through its final receipt retirement.
Its own invalidation does not cancel the context that joins its accepted writes.
No issuer, Peer or reactor lock is held across these waits.

Destination permission removal first joins affected issuer work, then retires
only the captured Session/provider frontier using engine cleanup authority. The
existing joined `ReasonPeerRemoved` DOWN drops that exact provider destination
before the original candidate is withdrawn. No ordinary selected absence is
authorized by a lost grant. A receive-only binding rebound parks surviving
originals before its terminal DOWN. It re-offers them after the binding is
published and reconciled, even when the provider's process token did not change.

Managed generation-ready hooks run only after the full handshake and runtime
request readers are live. A replacement first reconciles and admits its peer
declarations, so restoration can call selected output through the normal guard.

The owning startup or immediate restart goroutine joins restoration before it
returns success. When a transaction defers admission, its completion goroutine
owns a fresh bounded restoration join before it completes the transaction.
The restart command reads that exact replacement's completed restoration and
admission, not a name-level doctor record from an older process.

A separate replacement hold blocks post-startup delivery from a racing global
fan-out while restoration still runs. Only successful restoration releases
that hold. A failed restoration stops and releases the exact replacement through
the existing rollback path. A superseded process cannot run restoration.
These steps permit a framed intake callback to call selected output with live
readers, reconciled authority and no premature post-startup delivery.

Trusted registry expiry removes its exact issuer original even while the
configured provider is unavailable. Remote cleanup failure stays explicit.
Later provider readiness cannot resurrect the expired data.

For a bridge child's `restore-configured` withdrawal, the engine first removes
its admitted API original through the current provider and joins that removal.
Only then does the existing configured-route builder give back the configured
route. The original cannot replay after that restoration. Direct/no-intake
scopes retain their existing restoration path. Configuration and SENT state
gain no candidate-election authority.

<!-- source: internal/component/bgp/reactor/origin_registry.go -- retireTrackedOriginBatch, retireTrackedLineage -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- withdrawNLRIBatch -->
<!-- source: internal/component/bgp/reactor/peer_static_restore.go -- restoreConfigured -->

#### Update-route command and admission results

`UpdateRouteResult`, `UpdateRouteWithMetaResult`, `UpdateRouteSelResult`,
`UpdateRouteSelWithMetaResult` and `UpdateRouteRequestResult` return the full
`UpdateRouteOutput` on the existing `update-route` operation. The typed and
framed paths carry identical counts, warnings, admission facts and failure
reason. The count-returning SDK and bridge methods wrap these methods and
retain their count/error contract. A partial command failure returns its result
beside the error; a transport failure creates no result. Nil or null whole
results fail. An explicit count-only direct-origin or control result remains
valid and establishes no retained candidate.

Each optional `admissions` entry identifies `family`, native `key-nlri`, concrete
`destination` and `peer-owner`, and the retained `generation`. Outcome is numeric:
1 retained, 2 refused, 3 failed; zero and unknown values are refused. Retained
admissions require a positive generation and disposition: 1 selected, 2 suppressed
by selection, 3 suppressed by export. Refused and failed admissions require a
reason, zero generation and disposition zero. These values report facts and
grant no output authority. They are separate from `SelectedRoute.OriginReceipt`
and the final selected-writer receipt. This transport change does not issue an
origin receipt or implement scoped origin election.

`Announced` keeps its command count and is zero for an accepted loser. Warnings
and zero counts alone do not establish admission. `Withdrawn` still counts a
completed withheld withdrawal: it is not a count of socket writes. Earlier
completed counts and warning reasons survive a later failure. The BGP
redistribution and BGP-LS result readers recognize explicit retention and keep
real admission or transport failures visible.
<!-- source: pkg/plugin/rpc/update_route_result.go -- RouteAdmission, Validate, Err, Retained -->
<!-- source: pkg/plugin/sdk/sdk_engine.go -- UpdateRouteResult, UpdateRouteRequestResult -->
<!-- source: internal/component/plugin/server/dispatch.go -- extractUpdateRouteOutput -->

#### Plugin metrics

A plugin that runs in its own process cannot reach the engine's metrics
registry, so it publishes its metrics with `metrics-publish`, and the plugin
decides when. Each `declare` entry creates the metric in the engine's
registry under the declared name and shape (`counter`, `gauge`, `histogram`,
`counter-vec`, `gauge-vec`, `histogram-vec`, with label names and resolved
buckets): the metric an in-process plugin would create. A name declared
again with the same shape shares the metric, so a restarted plugin carries on
from the engine's values; a different shape, or a name the engine already
holds with another shape, is refused rather than registered. A plugin updates
only names it declared.

Each `update` is the change of one series since the plugin's last publish: a
counter's `value` is the amount added, a gauge's is its current value, and a
histogram carries per bucket (`+Inf` last) the observations gained (`counts`)
and their sum (`sums`). The engine replays each bucket's observations at
their mean, which lies inside the bucket, so its bucket counts and sum match
the plugin's. The engine applies `declare`, then `delete`, then `update`.

The answer lists `refused` items with a reason. `unavailable` means the
engine has no metrics registry yet and applied nothing; the plugin keeps the
changes and sends them again. Per plugin the engine holds at most
`rpc.MetricsDeclaredMax` names and `rpc.MetricsSeriesMax` labeled series,
every name starts with `ze_`, and one update adds at most
`rpc.MetricBucketCountMax` observations to a bucket. The metrics outlive the
process that declared them, as an in-process plugin's do.

<!-- source: pkg/plugin/rpc/metrics.go -- MetricsPublishInput, ValidateMetricsPublish -->
<!-- source: internal/component/plugin/server/dispatch_metrics.go -- opMetricsPublish, pluginMetricSet, replayBuckets -->
<!-- source: internal/core/metrics/shape.go -- Shape, ShapeReader -->

#### Plugin problems

A plugin reports a problem to the operator by raising it on the daemon's
report bus, where `show warnings`, `show errors` and `show doctor` read it.
A forked plugin has its own copy of `internal/core/report` in its process,
which nothing reads, so it raises with `report-issue`; the SDK's
`Plugin.RaiseIssue` and `Plugin.ClearIssue` send it on both transports. Like
the state methods it is sent after the handshake, from `OnStarted` or a later
callback.

| Field | Meaning |
|-------|---------|
| `action` | `raise` adds an error event, or adds or refreshes the active warning of its code and subject; `clear` removes that warning |
| `severity` | On a raise, `warning` (active until cleared) or `error` (an event the error ring keeps; it cannot be cleared) |
| `code` | A registered event code (`diagnostic.RegisterEventCode`), at most 64 octets |
| `subject` | What the problem is about, at most 256 octets |
| `message` | On a raise, what is wrong, at most 1024 octets |
| `fix` | On a raise, the command that fixes the subject, at most 256 octets; `show doctor` prints it as the row's fix line |

The engine names the source itself, from the plugin's configured name, so a
plugin cannot raise or clear under another owner; the request has no source
field. It refuses, with an error answer naming the field and the bound:

- a request longer than the framing plus every string field at its bound
  with each octet written as a six-octet JSON escape (`<`, as
  `json.Marshal` writes `<`, `>`, `&`, a control character or an invalid
  UTF-8 octet), about 9.5 KiB, or one that does not decode. The bound is
  derived from the field bounds, so every request the SDK validates and
  encodes passes it
- a missing or oversized field, an unknown action or severity, and a clear
  of an error
- a request from a plugin that has not finished its startup handshake
- a code that is not a registered event code: the recent-events check
  declares exactly those codes, so `show doctor` renders the raise under its
  own code and `ze show diagnostic` describes it. A code another check owns,
  or one the daemon was not built with, is refused
- more than `rpc.ReportRaisesPerWindow` (60) raises from one plugin in one
  `rpc.ReportRaiseWindow` (a minute). The first refused raise in a window is
  logged at WARN and raised once as the error event
  `doctor-plugin-request-refused`, subject the plugin, so the operator learns
  that the plugin flooded the bus. A clear is never refused by this bound
- a new warning that would take the plugin past its share of the bus's
  active warnings: the warning cap (`ze.report.warnings.max`) divided by
  `rpc.ReportActiveWarningsShare` (16), so 64 at the default cap of 1024.
  The answer names the share and says to clear a warning first. A raise that
  refreshes a warning already active adds none and is accepted, and the
  first refusal in a window is raised once as `doctor-plugin-request-refused`.
  The count is read from the bus itself, so a clear frees a slot at once

The field bounds are the report bus's own (`report.CodeMax` and the rest,
from which the `rpc` bounds are declared), so a raise the engine accepts is
never dropped by the bus. The YANG leaf lengths of `report-issue` repeat
them, and a test compares the two.

A plugin's ordinary active warnings leave the bus when its process goes: the
process manager clears its source when it removes the plugin or records its
crash. A concretely reserved daemon resource's warning stays owned by that
original resource. An ordinary `report-issue` raise or clear against its
protected subject refuses over both transports; it never acknowledges a
warning change it did not perform.
An ad-hoc plugin session (`HandleAdHocPluginSession`, an SSH channel) is never
in the process manager, so its warnings are cleared, and its raise window
forgotten, when the session ends. Every plugin's rate window is forgotten
when its process is cleaned up, so the windows hold only processes that are
alive.

<!-- source: pkg/plugin/rpc/report.go -- MethodReportIssue, ReportIssueInput, ValidateReportIssue, ReportRaisesPerWindow, ReportActiveWarningsShare -->
<!-- source: pkg/plugin/sdk/report.go -- Issue, RaiseIssue, ClearIssue -->
<!-- source: internal/component/plugin/server/dispatch_report.go -- opReportIssue, raiseReported, reportIssueEncodedMax, pluginReportRate, raiseWarningWithin, activeWarningsMax, forgetPluginReports -->
<!-- source: internal/component/plugin/server/adhoc.go -- endAdHocSession -->
<!-- source: internal/core/report/report.go -- SourceWarnings, CodeMax -->
<!-- source: internal/core/report/resource_warning.go -- TryClearWarning, TryRaiseWarningFix -->
<!-- test: internal/component/plugin/server/dispatch_report_test.go TestReportRaiseEscapedAtBoundsBothTransports -->
<!-- test: internal/component/plugin/server/dispatch_report_test.go TestReportActiveWarningsSharePerPlugin -->
<!-- test: internal/component/plugin/server/dispatch_report_test.go TestAdHocSessionEndClearsItsReports -->
<!-- test: internal/component/plugin/server/dispatch_report_test.go TestReportRaiseRouteBothTransports -->

#### Panic containment

A panic in the engine's side of the protocol ends one unit, never the daemon.
Each boundary counts the panic in
`ze_task_panics_total{component="plugin",task=...}`, logs it at ERROR and
writes a crash record that `ze show crashes` lists
(`docs/guide/monitoring.md`, "Contained Panics").

| Task | Where | What one panic ends |
|------|-------|---------------------|
| `engine-op` | `dispatchPluginRPC`, one goroutine per plugin-to-engine request | That request. The plugin's call fails with `internal Ze defect while handling <method>: the request was not applied and may be sent again; the plugin connection stays open`, followed by the crash record's name. The panic value is never sent to the plugin. The plugin's other requests are still served |
| `rpc-read` | `Conn.readLoop`, the frame reader | The connection. Its read error is the panic, the engine releases the plugin as it does one whose process exited, and the plugin reads EOF |
| `rpc-route` | `MuxConn.readLoop`, which routes answers and requests | The same connection, and the plugin's calls in flight fail |
| `connect-back` | `PluginAcceptor.handleConn`, before the `#0 auth` handshake completes | That connection. The plugin then fails to start, as one that never connected |
| `run` | the goroutine `startInternal` runs an in-process plugin on | The plugin, which ends as if its run function had returned |

No boundary restarts a plugin: the engine releases it as it releases any
plugin that exited, and a reload starts it again. A request that arrives over
`DirectBridge` runs synchronously on the calling plugin's goroutine, not under
`engine-op`. A panic in it unwinds the calling goroutine, so the boundary of
that goroutine's task contains it: the `run` boundary when the call came from
the plugin's run goroutine, and the task's own boundary when it came from a
goroutine the plugin started to read network input (each such reader has its
own task in "Contained Panics", for example `bmp`/`receive`). A goroutine a
plugin starts for its own internal work carries no engine boundary, as the
plugin's other internal workers do not.
<!-- source: internal/component/plugin/server/dispatch.go -- dispatchPluginRPC -->
<!-- source: internal/component/plugin/server/panic_guard.go -- engineOpPanics, engineOpPanicMessage -->
<!-- source: pkg/plugin/rpc/panic_guard.go -- readerPanics -->
<!-- source: internal/component/plugin/ipc/panic_guard.go -- acceptorPanics -->
<!-- source: internal/component/plugin/process/panic_guard.go -- runnerPanics -->

#### Persistent runtime state

Plugins call `StateGet`, `StatePut`, `StateRemove`, `StateList` and
`StateIncrement` on the SDK after their handshake, from `OnStarted` or a later
runtime callback. Construction and `OnConfigure` cannot send these requests:
the startup coordinator accepts only the stage methods until the handshake ends.
An engine created by a reload completes the same state initialization before
subscribing to interface events or opening a packet transport.

The daemon routes all five operations through `engineOps` on both the JSON
connection and `DirectBridge`. It uses the lifetime-owned `storage.Storage`
registered in `statestore`; a plugin never obtains that handle over the wire.
Grants come from compiled registration and come in three kinds:

| Grant | Registered with | Admits |
|-------|-----------------|--------|
| Write | `statestore.RegisterPluginKeys(owner, keys...)` | get, list, put, remove and increment on every instance of each pattern |
| Read | `statestore.RegisterPluginReadKeys(reader, keys...)` | get and list only: put, remove and increment are refused. A plugin reads another plugin's keys this way (the IRR consumers read what the `irr` plugin stores) |
| One instance | `statestore.RegisterPluginKeyInstance(owner, key, params...)` | write access to the one key `key.Key(params...)` and no other instance of the pattern. A plugin whose key names the plugin itself takes it, so it cannot write the key of another plugin sharing the template (`meta/irr-interest/{consumer}`) |

The dispatcher picks the grant by the operation: `state-get` and `state-list`
need a read or a write grant (`statestore.PluginKeyReadable`), and
`state-put`, `state-remove` and `state-increment` need a write grant
(`statestore.PluginKeyWritable`). Compiled-in plugins key state grants on their
compiled registration identity (`proc.RegistrationName()`) under any endpoint
alias or transport (`in-daemon` or `process separate;`), while foreign
`external` programs hold none of those grants even if their endpoint name
matches a compiled-in plugin. A request cannot choose another owner or grant
itself access to credentials. Template parameters match one path segment
(`zefs.KeyEntry.Matches`), and a list returns only keys that the caller can
read. The daemon's key commands `delete data key` and `update data key` read
the same grants by registration identity (`statestore.PluginKeyOwners`, which
names write grants only) and refuse a key while any running process holding
that registration identity is active (under any endpoint alias), while a plugin
that only reads a key does not block it.

`StateInput.key` is bounded to 512 bytes. A value is bounded by its key's
registration: one MiB (`rpc.StateDataMax`) unless the key's `zefs.KeyEntry`
declares its own `ValueMax`. The IRR dataset key
`meta/irr-data/{server}/{as-set}/{digest}` declares 12,000,000 bytes, which travel as
16,000,000 bytes of base64 inside the 16 MiB message. The SDK
(`rpc.ValidateStateInput` before sending, the result check after) and the
daemon (`stateInput` on every request, `opStateGet` on the stored value) apply
the same bound, read by `rpc.StateDataMaxFor`; one byte over refuses the
request on either side. Values use base64 in JSON. A default-bound value is
more than an operator can write back through the daemon: `update data key`
carries at most 195,840 bytes, because the command and its content travel in
one SSH packet. A larger value a plugin wrote is rewritten with
`ze data write <key> <file>` while ze is stopped, which the refusal names.
Lists return at most 4096 keys; excess fails rather than returning a partial
result. `ze-plugin-engine.yang` models the key and list bounds, and its
state-data length is the largest declared value bound (`rpc.StateDataLargest`,
compared by `TestStateDataYANGLengthIsTheLargestBound`).
`StateOutput.status` explicitly names `ok`, `absent`, `unavailable`, `corrupt`,
`read-failed`, or `persist-failed`. Only a successful mutation returns `ok`.
`StateGet` turns `absent` into `found=false` with no error; failed outcomes
become `rpc.StateError`. Existing in-daemon best-effort `statestore.Put/Get/Remove`
callers keep their no-store behaviour, but the strict RPC path never treats
`false, nil` as persistence.

`StateIncrement` holds a runtime-state guard across read, increment and write,
and waits for guard release before acknowledging the new value. An absent
counter starts at one. Corrupt data and uint32 exhaustion fail without reset or
wrap. OSPF uses this operation for its boot word, so separate instances cannot
allocate the same sequence space.

A mutation waits only for another runtime-state writer, or for a guard that
has reached runtime state, such as a backup or a history repair walking the
whole store. It does not wait for a configuration write,
whose syncs can take seconds on a slow disk: the boot record makes those once
the hub serves, while plugins keep writing their state (OSPF persists a
planned graceful restart's fact under a five second deadline). `StateGet` and
`StateList` wait for no writer at all, so a plugin reading its state at
startup is answered at once (`shared` in `statestore.go`, and "An unguarded
read does not wait for a guard" and "Writers are split into two key spaces" in
[Storage backends](../storage-backends.md)).

The SDK honours caller cancellation before dispatch and when reading the result.
A disk mutation already in progress is completed by the daemon. Cancellation or
a lost reply leaves the caller without an acknowledgement, even if the mutation
committed; callers cannot infer that an increment is safe to retry.

<!-- source: pkg/plugin/rpc/state.go -- StateInput, StateOutput, StateError, StateDataMaxFor, StateDataLargest -->
<!-- source: pkg/plugin/sdk/sdk_state.go -- stateCall, StateIncrement -->
<!-- source: internal/component/plugin/server/dispatch_state.go -- stateInput, stateAccess, opStateGet, opStatePut -->
<!-- source: internal/core/statestore/permissions.go -- RegisterPluginKeys, RegisterPluginReadKeys, RegisterPluginKeyInstance, PluginKeyReadable, PluginKeyWritable, PluginKeyOwners -->
<!-- source: internal/core/statestore/permissions.go -- RegisterPluginKeys, PluginList, PluginKeyOwners -->
<!-- source: internal/component/config/storage/cli/data_rpc.go -- runningOwnerError -->
<!-- source: internal/core/statestore/statestore.go -- Increment, Read, Write, shared -->
<!-- source: pkg/plugin/sdk/state_keys.go -- Plugin.StateKeys -->

For existing raw-key consumers, `Plugin.StateKeys(ctx)` supplies the same SDK
operations as `ReadKey`, `WriteKey`, `RemoveKey` and `ListKeys`. Each call has a
five-second timeout beneath the supplied lifetime context. The caller retains
that context's cancellation obligation, and the adapter has the same
post-handshake restriction as the explicit state methods.


#### Forked route install (`route-install` / `route-remove`)

Route-installing plugins (OSPF, IS-IS) do not program the FIB directly: their SPF
installers insert `locrib.Path` values into the process-wide Loc-RIB singleton
(`locrib.Default()`), which `sysrib` arbitrates and `fibkernel` programs. In a
FORKED (external) plugin subprocess, `locrib.Default()` returns nil (the singleton
lives in the engine's address space), so those installers instead hold a
`routeinstall.Sink` and ship each operation over `route-install` / `route-remove`.
The engine applies the batch to its real Loc-RIB, where `sysrib`'s `OnChange`
programs the kernel exactly as for an in-process installer. Each entry carries the
redistribute protocol **name** (not the numeric `ProtocolID`, which is per-process).
The engine resolves it against its registered protocols and refuses an unknown name.
<!-- source: internal/component/plugin/server/dispatch_route.go -- applyRouteInstall -->
<!-- source: internal/core/rib/routeinstall/sink.go -- Sink -->

An entry carries no protocol-wide administrative distance. The engine's Loc-RIB
looks up the configured distance when it ranks the path: for a BGP path,
`is-bgp` and `is-ebgp` select the `ibgp` or `ebgp` configuration leaf without
changing the canonical `bgp` route owner, and other protocols use their
registered name. The optional `distance` field is a route's own distance (a
static route's `distance` leaf), and overrides the configured value for that
route alone.
<!-- source: internal/core/rib/locrib/distance.go -- resolvedDistance, DistanceProtocol -->
<!-- source: internal/component/plugin/server/dispatch_route.go -- applyRouteInstall -->

The process boundary preserves the primary next-hop device and `on-link` marker,
each ECMP member's device and marker, the selected SRv6 service SID, and the BGP
metric fields (`is-bgp`, `aigp`, `aigp-present`, `metric-recursive`). Invalid
addresses reject the batch before any route is installed. These fields let
recursive resolution and the FIB use the same path properties for a forked
producer as for an in-process producer.

Four optional fields carry what a Loc-RIB import filter reads, so the gate
judges a forked producer's route on the same values as an in-process one's:

| Field | Meaning |
|-------|---------|
| `tag` | The route tag. Meaningful only with `has-tag`, because 0 is a real tag |
| `has-tag` | The route carries a tag. Absent, the route carries none, never tag 0 |
| `level` | The IS-IS level, 1 or 2. Absent or 0, the route carries no level |
| `ospf-route-type` | The OSPF path type: 1 intra-area, 2 inter-area, 3 external-1, 4 external-2, 5 nssa-1, 6 nssa-2. Absent or 0, the route carries none |

A plugin that omits them sends a route that carries none of them. A `level`
above 2 or an `ospf-route-type` above 6 rejects the batch before any route is
installed.
<!-- source: pkg/plugin/rpc/types.go -- RouteInstallEntry -->
<!-- source: internal/core/rib/routeinstall/sink.go -- Sink.InsertForward -->
<!-- source: internal/component/plugin/server/dispatch_route.go -- parseFilterFields -->

`route-metrics` returns one distance per requested address, in request order.
Each distance distinguishes unresolved reachability from a resolved zero cost
and reports whether a recursive BGP hop lacked AIGP. The revision is sampled
before resolution, so a routing change during the batch invalidates the snapshot
on the next poll. An empty address list requests the revision alone. Requests
are limited to 4096 addresses; an invalid address rejects the whole batch. The
SDK refuses a missing result or a short distance vector rather than treating
either as a usable snapshot.
<!-- source: internal/component/plugin/server/dispatch_route_metrics.go -- opRouteMetrics -->
<!-- source: pkg/plugin/sdk/sdk_engine.go -- RouteMetrics -->

An optional `prospective-routes` list uses the `RouteInstallEntry` wire shape
and must contain one contribution per requested address. Each distance resolves
through the table that would result from that one contribution, under the same
import policy and selection rules as an actual install. The proposals are
independent: they are not installed, combined into one hypothetical batch, or
published as routing changes. This lets a selecting plugin exclude a next hop
that would become recursive through its own destination. The SDK exposes this
query as `RouteMetricsProspective`; ordinary `RouteMetrics` calls keep the
current-table behavior.
<!-- source: pkg/plugin/rpc/types.go -- RouteMetricsInput, RouteInstallEntry -->
<!-- source: pkg/plugin/sdk/sdk_engine.go -- RouteMetricsProspective -->
<!-- source: internal/component/plugin/server/dispatch_route.go -- parseRouteInstall -->
<!-- source: internal/component/plugin/server/dispatch_route_metrics.go -- opRouteMetrics -->
<!-- source: internal/core/rib/locrib/prospective.go -- PreviewSelected -->

`batch-validate` carries `ValidationDecision.MsgID` as the received UPDATE
generation. `peer-owner` and `connection-id` are decimal-string uint64 fields
captured on that received source, never resolved by address when a callback
arrives. Both must be supplied together; both zero identify observations only.
A zero message ID selects the current or next generation within that exact
source, never a same-address sibling. The explicit text batch command carries
nine fields per decision: action, peer address, family, prefix, path ID, validation
state, received message ID, original peer owner and original connection ID. A rejected route marked `Ineligible` remains stored for later
validation; `Accept` and `Ineligible` cannot both be true. A successful call
returns `BatchValidateResult`, including for an empty batch; an empty or null
result is an error at the SDK boundary.

The engine sends each complete batch to the process that registered
`request bgp adj-rib-in batch-validate`, using the existing nine-field command.
The configured process name may be an alias. Command registration refuses a
second owner of the same path; dispatch retains the registered process and
connection for the call. Both socket and direct SDK requests use that route, so a forked receive store
is never replaced by a process-local manager callback. Direct SDK dispatch
preserves the supplied caller context. This ordinary socket operation runs
under the server's runtime request context and the command timeout; it has no
resource cancellation scope. A socket caller abandoning its wait does not
prove that the receive-store command was canceled or joined. A missing owner, refused command, invalid JSON or null
answer fails the whole request; there is no implicit receiver fallback.

An empty validation batch still enters the receive store's normal mutation
admission. It refuses a removed, stopped or terminally failed owner, and must
settle older owed validation publication before it can acknowledge zero new
decisions. Empty input does not bypass that lifecycle boundary.
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_commands.go -- batchValidateCommand -->

<!-- source: internal/component/plugin/server/dispatch_batch_validate.go -- dispatchBatchValidation -->
<!-- source: pkg/plugin/rpc/bridge.go -- ValidationDecision -->
<!-- source: pkg/plugin/sdk/sdk_engine.go -- BatchValidate -->

---

## Event Delivery

Every participant registers the messages it sends and receives at compile time
(`registry.Registration`) or Stage 1 (`declare-registration`). With nothing
written in config, those registered defaults are what runs. Any `bus { }`
override on the participant, on its `bus { endpoint <name> }` block, or under
`bus { participant <path> }` compiles with the registrations into one filter
per participant (`DeliveryGraph`). A peer-scoped event is delivered when both
the sender's and receiver's compiled filters admit the message, direction,
endpoints, and any tag filters.

A reload can widen a receive-only `bus` filter so a running endpoint begins
receiving from a peer whose session is already up (with no route-sending
permission added). That endpoint receives a `state` up with reason `attached`,
then `negotiated` if its compiled filter admits it, before any other message of
that session reaches it. The up carries no initial-replay token, arms no
End-of-RIB barrier, and is not a new session: a plugin that replays routes
toward the peer on up MUST skip the replay for it. An endpoint removed from a
peer's compiled filter by a reload receives a `state` down with reason
`removed`, to it alone. A changed filter scope is a down, then an `attached`
up.
<!-- source: internal/component/plugin/server/delivery_graph.go -- DeliveryGraph, PeerScopedProcs -->
<!-- source: internal/component/bgp/server/events.go -- onPeerBound, onPeerUnbound -->
<!-- source: internal/component/plugin/server/delivery_reconcile.go -- deliveryDisagreements -->

Events that pass both halves are enqueued into a per-process bounded,
dynamically growable, lossless FIFO ring queue (`eventQueue`). The ring starts
at `64` slots, doubles on demand up to its configured ceiling (`65,536` by
default for ordinary endpoints, `256` for `CacheConsumer` plugins such as
`bgp-rs` and `bgp-rr`, overridable per endpoint via `delivery-queue-max` in
`64..1048576`), and shrinks back to `64` slots once drained empty from
capacities above `256`. Engine-to-plugin event delivery never drops events: when
the queue reaches its ceiling, producers block with backpressure (cancellable
via `DeliverContext` or process stop) until the consumer frees slots.

The single long-lived delivery goroutine drains up to `512` available events
into a batch and sends them in a single `deliver-batch` RPC (relying on
`CallBatchRPC` to split frames exceeding 16 MiB), reducing syscalls and
goroutine churn. Single events are delivered as a batch of 1. An internal
plugin with a structured handler receives a batch holding both structured and
text events as one bridge call per run of same-kind events, in queue order, so
a process always handles its events in the order they were queued. That order
is what lets a bus event a plugin emits inside its state handler
(`bgp-gr/retention` inside `bgp-gr`'s DOWN handler) reach another plugin ahead
of the state event the engine delivers after it.
<!-- source: internal/component/plugin/process/event_queue.go -- eventQueue, eventQueueInitialCap, eventQueueShrinkThreshold, eventDeliveryBatchMax -->
<!-- source: internal/component/plugin/process/delivery.go -- Process.DeliverContext, Process.deliveryLoop, deliverMixedBatch, flushStructuredRun, flushTextRun -->

### Batched Delivery

<!-- source: pkg/plugin/rpc/batch.go -- WriteBatchFrame -->

```
#42 ze-plugin-callback:deliver-batch {"events":["<json-event-1>","<json-event-2>"]}
#42 ok
```

The SDK unpacks the batch and dispatches each event to the `OnEvent` handler individually.
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- callbackDeliverBatch -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- the callbackDeliverBatch handler -->

`CallBatchRPC` splits between events, not inside an event's payload. A producer
of a whole-table batch must split its domain entries before `emit-event`.
`EmitEventInput.CanonicalPayloadBudget` gives a conservative byte budget for
HTML-escaped canonical JSON in both that request and a singleton enveloped
`deliver-batch`, including string quoting and the widest correlation ID.
The emit-event envelope carries no event receipt metadata; the BGP receive producer's
metadata has a separate ownership contract. The receive store's validation
arrays and the forked BGP RIB's best-change, redistribution and validation
batches use this budget, and also cut a chunk at `ribevents.FrameEntriesMax`
entries, because a subscriber handles a chunk's entries before the next one
is sent and its work per chunk must stay bounded. Each chunk keeps the original event shape and replay
metadata, and each chunk carries a deadline of its own: a dead engine
connection fails the hand-off within one chunk's deadline, while a live engine
acknowledging a whole family chunk by chunk is never cut off by the family's
size. A failed chunk stops the
hand-off; no chunk is retried by the framing layer. An indivisible value that
does not meet the conservative proof is left to the actual writer and its
actual ID, not declared unencodable from the proof alone.
<!-- source: pkg/plugin/rpc/event_frame_size.go -- EmitEventInput.CanonicalPayloadBudget -->
<!-- source: internal/component/bgp/plugins/rib/rib_event_sdk.go -- sdkEventBus.Emit, sdkEventArray, sdkEventChunkEnd -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_eligibility.go -- emitValidationFrames, emitValidationChunk -->


### Subscription Namespace

`SubscribeEventsInput` carries an optional `namespace` field. Empty (the default,
and the wire-compatible value for every pre-existing caller) resolves to the
namespace registered by the owning protocol component (`bgp` today). A non-empty
value subscribes to another namespace (e.g. `vpn-ipsec`) at startup; an
unregistered namespace is logged and the subscribe block is skipped rather than
registered as a silently-dead `NamespaceUnknown` subscription. The skip, an
empty namespace with no default registered, and `ready` parameters that do not
parse (which carry the startup subscriptions, so none register) are each
recorded on the plugin's process and reported by the `plugin-subscriptions`
doctor check under `doctor-plugin-subscription-dropped`. The SDK exposes
this via `SetStartupSubscriptionsIn(namespace, events, peers, format)`;
`SetStartupSubscriptions` (namespace `""`) is unchanged.
<!-- source: internal/component/plugin/server/dispatch.go -- registerSubscriptions, resolveSubscriptionNamespace -->

A `"*"` event expands at registration time into the namespace's registered event
types, for each peer selector (no wildcard branch on the per-event match path).
Startup and runtime subscriptions register every entry in `peers` as an
independent alternative: an IP address or peer name selects that peer, `"*"`
selects all peers, and an empty list also selects all peers. A `"!value"` selector
matches peers whose address and name both differ from `value`. Alternatives are
unioned, so an exclusion is not a veto on another selector's match.

Overlapping selectors deliver an event once per process, not once per matching
subscription. Peer-scoped delivery still requires the configured receive grant
for that process, event type and direction; selecting more peers does not grant
permission to receive their events.
<!-- source: internal/component/plugin/server/dispatch.go -- registerSubscriptions -->
<!-- source: internal/component/plugin/server/subscribe.go -- (*PeerFilter).Matches, (*SubscriptionManager).getMatching -->
<!-- source: internal/component/plugin/server/delivery_graph.go -- (*Server).PeerScopedProcs -->

### Interface address replay

A plugin that needs the interface address inventory subscribes to
`interface/addr-added` and `interface/addr-removed`, then emits
`interface/address-replay-request` with no payload. An active interface
monitor replays its current usable addresses through `addr-added`, using the
same JSON fields as live changes: `name`, `unit`, `index`, `address`,
`prefix-length`, `family`, and optional `origin`. Consumers MUST tolerate
duplicate adds. A request does not load or start an interface backend, and
there is no completion response when a provider is absent.
<!-- source: internal/core/iface/events/register.go -- AddressReplayRequest -->
<!-- source: internal/plugins/iface/netlink/monitor_linux.go -- addrEventPayload, run, handleAddrUpdate -->
<!-- source: internal/plugins/iface/vpp/monitor.go -- addrEventPayload, seedAddresses, replayAddresses -->

### The `system-dns` namespace

The router's resolver registry (`system.Resolvers`) publishes its state on
the `system-dns` namespace, so a plugin in the engine process or in its own
process reads the resolver addresses Ze learned without importing the
registry.

| Event | Direction | Payload |
|-------|-----------|---------|
| `snapshot` | engine to plugin, after every change | the whole state: `sequence`, `records`, `router` |
| `snapshot-request` | plugin to engine | none. The engine answers by publishing the current `snapshot` again |
| `selection` | plugin to engine, after its selection changes | one consumer's `consumer` name and its `selections`, each a list as `router` is shaped plus an `instance` (the subnet, unit or pool) and, for the LAN resolver, the `serves` addresses it answers on. `withdrawn` true, with no selections, from the plugin's `OnBye` handler when the engine removes it while the daemon runs (a `bye` with reason `removed`). A daemon stop sends no `bye` to a plugin, so no withdrawal is sent then, and the registry stops with the daemon. The engine keeps them for `show system dns` and `ze doctor` |

```
{"sequence":7,
 "records":[{"origin":"dhcpv4","rank":100,"interface":"eth0","unit":"0",
             "addresses":["192.0.2.53"],"received":"2026-11-08T10:15:15Z"}],
 "router":{"path":"system dns-source","sources":["configured","received"],
           "mode":"first-usable","default":true,"used":["received"],
           "servers":["192.0.2.53"]}}
```

<!-- source: internal/core/dnssource/record.go -- Namespace, SnapshotEventType, SnapshotRequestEventType, PublicationEventType, Record, ConsumerSelection, Publication, Snapshot -->
<!-- source: internal/core/dnssource/register.go -- SnapshotEvent, SnapshotRequest, PublicationEvent -->
<!-- source: internal/component/config/system/resolv.go -- (*Resolvers).PublishOn, (*Resolvers).SetPublication -->
<!-- source: internal/component/plugin/server/startup_removal.go -- notifyPluginRemoval, the bye with reason removed -->
<!-- source: internal/plugins/dnsresolver/register.go -- followLearnedResolvers, the OnBye withdrawal -->

A `records` entry is one learned source: its `origin` and the origin's
`rank` (a lower rank lists first), the `interface` and `unit` that learned
it (both empty for the host's `/etc/resolv.conf`), the `addresses` in the
order the source sent them, and the time it was `received`. An `expires`
member, when present, is the end of the record's validity; when absent, the
record is valid until its source withdraws it. A `fallback` member set to
true (the host's `/etc/resolv.conf`) marks a record the received class lists
only while no other record lists an address; a plugin that selects from
`records` itself applies the same rule, as `dnssource.ReceivedAddresses`
does. `router` is the router's
list (`sources` and `mode`, with `default` true while the built-in default
applies, and `received-interfaces` when `received-interface` narrows the
received class), the classes that supplied a server (`used`), and the
`servers` Ze asks, in order. `mode` is `unspecified` for a one-source list
written without one. A record whose validity ended is withdrawn before the
next snapshot, so no published record is ever expired.

A plugin emits `snapshot-request` once it has subscribed, so a plugin that
starts late or restarts gets the state without waiting for a change. One
state can therefore arrive twice, and the two copies can arrive out of
order: a plugin MUST drop a `snapshot` whose `sequence` is below the last
one it applied.
<!-- source: internal/component/config/system/resolv.go -- PublishOn, emitDNSSnapshot -->

### Enveloped Delivery

`SubscribeEventsInput.envelope` (opt-in, default false; SDK `SetEnvelope(true)`)
wraps each delivered event string in an `EventEnvelope`:

```
{"namespace":"vpn-ipsec","event":"sa-up","payload":<bare payload JSON>}
```

The envelope rides INSIDE the delivered event string, so it is transparent to
both `deliver-event` and `deliver-batch` (both still carry a JSON string). It
lets a plugin subscribed to several event types discriminate which one arrived
even when two events share a payload type (e.g. `sa-up` vs `sa-down`). Without
the opt-in, delivery is byte-identical to before: the bare payload. The engine
renders the envelope at most once per emit and only when a matching subscriber
opted in, preserving the lazy-marshal-once cost of the default path.
<!-- source: internal/component/plugin/server/dispatch.go -- deliverEvent, buildEventEnvelope; pkg/plugin/rpc/types.go -- EventEnvelope -->

The optional `metadata` member is opaque to the generic event transport. Its
namespace owns its shape and meaning; an event with no such metadata omits it.
A text payload (`encoding=text`) or a `base64` UPDATE line (`format=base64`)
is a JSON string inside the envelope, not text interpolated into a JSON object.
The SDK's single-event and batch callbacks carry both forms unchanged.

**BGP UPDATE forwarding receipts.** An enveloped `bgp/update` carries the
engine's per-message receipt in `metadata`, beside the unchanged JSON payload or
quoted text / `base64` payload:

| Member | Type | Meaning |
|--------|------|---------|
| `peer` | IP address | The peer named by the payload |
| `peer-owner` | uint64 | The engine's stable Peer incarnation named by the payload; zero is explicitly unrecorded |
| `message-id` | uint64 | The payload's received message ID |
| `source-policy-epoch` | uint64 | The payload's original policy admission, including an explicit unrecorded zero |
| `source-receipt` | uint64 | The payload's original source session identity, including an explicit unrecorded zero |
| `reactor-forwarded` | bool | The reactor fast path has already decided the eligible destinations |
| `fast-path-skipped` | captured target list | Destinations handed back to the forwarding plugin; omitted when empty |

`peer`, `peer-owner`, `message-id`, `source-policy-epoch`, `source-receipt` and `reactor-forwarded` are mandatory
and non-null. An explicit zero ID or unrecorded policy epoch and an explicit
`false` decision remain values, not missing members. An omitted or null decision
must not silently become `false` and send a second copy to a target the reactor
already served.

These are delivery facts from the engine's `RawMessage`, never filter-writable
`route-meta`. A reader binds the receipt to the actual payload's peer, message ID
and stable peer owner, policy epoch and source receipt before using it. The route server snapshots the skipped list
into its worker item on structured delivery; JSON decoding owns the list on the
forked rail. Neither rail keeps a mutable alias to a publisher's receipt.

When `reactor-forwarded` is true, the route server forwards only to the skipped
destinations and eligible sessions established after the message was numbered,
under the same peer-up replay cut as ordinary forwarding. It still records the
announcement or withdrawal for source-DOWN recovery. A destination already
served by the reactor gets no second cached forwarding request; the session
writer does not suppress legitimate reannouncements to compensate for a lost
receipt. Subscriptions without envelope delivery keep their existing bare
text/JSON format.

Only a received UPDATE is enveloped. A sent UPDATE reaches a subscriber that
opted in as the bare JSON document or text line, with no envelope and no receipt,
so a subscriber to both directions parses each delivery by its form: a sent
UPDATE is never an `EventEnvelope`.
<!-- source: internal/component/bgp/server/events.go -- onMessageReceived, onMessageBatchReceived, onMessageSent -->
<!-- source: internal/component/bgp/types/rawmessage.go -- RawMessage.ReactorForwarded, RawMessage.FastPathSkipped -->
<!-- source: internal/component/bgp/types/update_receipt.go -- UpdateReceipt, ParseUpdateReceipt, Validate, UnwrapUpdateText -->
<!-- source: internal/component/bgp/types/update_receipt_scalar.go -- receiptUint64.UnmarshalJSON, receiptDecision.UnmarshalJSON -->
<!-- source: internal/component/bgp/server/event_update_receipt.go -- updateEnvelopeCache.wrap -->
<!-- source: internal/component/bgp/event.go -- ParseEvent -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- dispatchText -->
<!-- source: internal/component/bgp/plugins/rs/server.go -- dispatchText, dispatchStructured -->
<!-- source: internal/component/bgp/plugins/rs/server_withdrawal.go -- processForward -->
<!-- source: internal/component/bgp/plugins/rs/server_forward.go -- batchForwardUpdateSkipped -->

### An Event Emitted Before the Subscribers Exist

A subscription is registered when its plugin handshakes, and plugins handshake
in phases: config-path plugins load first, explicit `bus { endpoint ... }`
plugins after them. An event a config-path plugin emits during its own configure
callback is therefore routed correctly and delivered to nobody. `getMatching`
returns an empty set, `deliverEvent` returns 0, and no error is raised: an
empty subscriber set and a filtered-out subscriber look the same.

A startup subscription does not close this window. It is registered at the
subscribing plugin's ready handshake, which for a Phase 2 plugin is still after
Phase 1 has run.

So a plugin that emits an event an external plugin must see MUST NOT start the
work that emits it from its configure callback. It starts that work from
`OnAllPluginsReady`, which the engine fans out after every plugin in every phase
is running and both registries are frozen. BGP does this by starting its reactor
at configure and its peers from `coord.OnPostStartup`. IKE does it by stashing
the parsed configuration and reconciling peers from `OnAllPluginsReady`, which is
why the first `sa-up` of a session now has a reader.

The handler MUST NOT wait on the activity it starts. `sendPostStartupToAll` is
not ordered against peer startup, so a handler that blocks on peer activity
deadlocks.
<!-- source: internal/component/plugin/server/startup.go -- phase order; internal/component/ike/engine/register.go -- OnAllPluginsReady; internal/component/bgp/plugin/register.go -- OnPostStartup -->

### DirectBridge Optimization

For internal plugins with an active `DirectBridge`, `deliverBatch()` calls
`bridge.DeliverEvents(events)` directly instead of `conn.SendDeliverBatch()`,
bypassing RPC envelope construction, newline framing, and pipe I/O. The plugin's
`onEvent` handler is called synchronously in the delivery goroutine.
<!-- source: pkg/plugin/sdk/sdk.go -- Run, bridge activation -->

#### Structured Event Delivery

When a plugin registers `OnStructuredEvent`, the engine delivers `*rpc.StructuredEvent`
instead of formatted text strings. `StructuredEvent` carries pre-extracted peer metadata
(PeerAddress, PeerAS, LocalAS, etc.) and a `RawMessage` pointer for wire message events.
This eliminates the JSON round-trip: the engine skips text formatting, and the plugin
reads data directly from `AttrsWire` (lazy per-attribute parsing) and `WireUpdate`
(zero-copy section access) instead of calling `ParseEvent`.
<!-- source: pkg/plugin/rpc/bridge.go -- StructuredEvent -->

`StructuredEvent.PeerAS` is an `rpc.RemoteAS`, not a bare number. A dynamic
peer's AS is learned from its OPEN, so an event built before the OPENs are
exchanged (the OPEN Ze sends, a NOTIFICATION Ze sends in OpenSent, the FSM
changes before OpenConfirm) has no AS to name, and the zero value of
`RemoteAS` is that "not yet known" state, never AS 0. Read it with
`ASN() (uint32, bool)` and branch on the bool: the number means nothing when
it is `false`. The JSON peer object omits `remote.as` in the same case, and
`Event.GetPeerASN` answers the unknown `RemoteAS` for it and an error for a
peer object it cannot read.

A plugin that renders the value without branching gets the same answer from
the type. `encoding/json` writes the number, or `null` while it is unknown, and
a field tagged `omitzero` is left out instead. `fmt`'s `%v` and `%s` and a
`log/slog` attribute write the number or `unknown`. Decoding reads `null` back
as unknown and refuses AS 0, which no peer can hold (RFC 7607). `%d` has no
such method, so it prints the struct, and `go vet` reports it.
<!-- source: pkg/plugin/rpc/remote_as.go -- RemoteAS, RemoteASOf, ASN, MarshalJSON, UnmarshalJSON, IsZero, String, LogValue -->
<!-- source: internal/component/bgp/event.go -- GetPeerASN -->

For UPDATE events, `RawMessage` carries `AttrsWire` and `WireUpdate` with lazy accessors.
For state events, `StructuredEvent.State` and `StructuredEvent.Reason` carry the data
directly. A down also carries `StructuredEvent.SessionGeneration`, the forward
generation of the session the down ended, which is the `session-generation`
member of the JSON and text forms (`json-format.md`, "State Events"); it is
zero on every other event. For other wire messages (OPEN, NOTIFICATION, REFRESH), `RawMessage.RawBytes`
contains the raw wire bytes.

Decoded OPEN events name the selected connection. An unselected incoming
connection retains its first actual sent and received OPEN observations
without publishing them under the incumbent's peer identity. If selected,
it publishes those exact bodies once before peer-up, after the previous
connection's cleanup and selected OPEN validation. A rejected or losing
connection publishes neither decoded OPEN. Packet capture and physical
OPEN counters still observe its actual exchange.
<!-- source: internal/component/bgp/reactor/session_open_publication.go -- privateNegotiationEpoch.messageCallback, observedOpen.publish -->
<!-- source: internal/component/bgp/reactor/session_open_admission.go -- selectNegotiationEpoch -->

`StructuredEvent` instances are pooled via `GetStructuredEvent`/`PutStructuredEvent`
to eliminate per-event heap allocations on the hot path.
`SourceReceipt` and `SourcePolicyEpoch` are copied directly from the raw message
and cleared on pool return, so a later event cannot inherit another session's
source identity.
<!-- source: internal/component/bgp/server/events.go -- getStructuredEvent, getStructuredStateEvent -->

Plugins that register both `OnStructuredEvent` and `OnEvent` receive structured events
via the former and text events via the latter. The delivery pipeline (`deliverMixedBatch`)
routes each event to the appropriate handler based on whether `Event` or `Output` is set.
<!-- source: internal/component/plugin/process/delivery.go -- deliverMixedBatch -->

### Text Event Formatting: the scratch discipline

A plugin without a structured handler receives text, and the engine builds that
text on the delivery path. Four rules keep the path free of allocations, and the
comments in `internal/component/bgp/server/events.go` name this section rather
than restate them.

1. **No format strings.** No `fmt.Sprintf`, no `fmt.Fprintf`, no `fmt.Appendf`.
   Reflection defeats escape analysis even when the output size is trivial. Use
   `strconv.AppendUint`, `netip.Addr.AppendTo`, `hex.AppendEncode`, and
   `append(buf, "literal"...)`.
2. **No intermediate string lists.** Never build a `[]string` and `strings.Join`
   it. Write the element, write the separator byte, repeat. No `strings.Builder`,
   no `strings.ReplaceAll`, no `strings.Replacer`.
3. **One `string(scratch)` per named boundary.** Every other code path stays on
   `[]byte`. In the event path the boundary is the per-encoding format cache and
   the RPC payload; each is one conversion per distinct encoding, not one per
   subscriber.
4. **Scratch is stack-local to the outer caller.** `var scratchArr [N]byte`
   lives on the goroutine stack of the event function that owns the loop. It is
   never a struct field, never a `sync.Pool`, and never per-peer. Output larger
   than `N` spills to the heap through `append` growth for that one call, which
   is correct and costs one allocation in the pathological case: the array size
   is chosen to cover the realistic maximum, not every input.

The formatters this path calls take the `AppendXxx(buf []byte, ...) []byte`
shape, so they append into the caller's scratch and never allocate a string of
their own.
<!-- source: internal/component/bgp/server/events.go -- formatMessageForSubscription, onPeerStateChange, onEORReceived, onPeerCongestionChange -->

### Event Subscription

A sent UPDATE's routing publication is pinned to its original configured
Peer before capturing process recipients. Permanent Peer removal seals that
original Peer, joins its admitted SENT enqueues outside reactor and peer locks,
then delivers the terminal DOWN through each process's same FIFO queue and
awaits its applied acknowledgement. Late callbacks cannot enqueue routing SENT
after that acknowledgement. Physical DOWN keeps reconnect inventory, and a
SENT may still precede UP or receipt capture. The pin is only an observation lifetime:
it issues no received-source lease, forwarding grant or retention right. A
refused late callback remains visible to wire monitors. The internal callback
handle never enters event JSON, StructuredEvent, SDK values or peer queries.
<!-- source: internal/component/bgp/reactor/source_sent_observation.go -- BeginSentObservation, EndSentObservation -->
<!-- source: internal/component/bgp/server/events.go -- onMessageSent -->

Plugins subscribe to events using either:

1. **Startup subscription** (recommended): included in the `ready` RPC so the engine
   registers atomically before `SignalAPIReady`, avoiding the race between the reactor
   sending routes and the plugin subscribing.
   <!-- source: pkg/plugin/rpc/types.go -- ReadyInput -->

2. **Runtime subscription**: via `subscribe-events` RPC in `OnStarted` callback.
   Safe but has a small window where events could be missed.
   <!-- source: pkg/plugin/sdk/sdk_engine.go -- SubscribeEvents -->

Neither plugin subscription form nor the operator's `request subscribe` can
widen what a peer grants. The operator command can add to the process's live
capability within that grant, and the next config apply discards the addition.
<!-- source: internal/component/plugin/server/delivery_graph.go -- (*Server).PeerScopedProcs, (*Server).DiscardRuntimeSubscriptions -->

A session that reaches Established before a plugin registers its subscription
raises its state event into an empty list, and nothing replays it. This is why
the startup form is recommended: it registers before `SignalAPIReady`, and ze
starts its peers after every plugin tier.

**Cross-Plugin DispatchCommand from Startup.** A plugin whose startup logic
must call `DispatchCommand` on a sibling plugin's command (e.g., `bgp-rpki`
enabling the `request bgp adj-rib-in enable-validation` gate) MUST register the call via
`OnAllPluginsReady`.

`OnStarted` fires after the plugin's ready acknowledgment, within Stage 5.
Plugins in later startup phases may not be loaded, so the dispatcher may not
yet know about the target command.
`OnAllPluginsReady` fires via the event loop once the engine has frozen every
registry after every phase completes, so the dispatch is guaranteed to resolve.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go — OnStarted vs OnAllPluginsReady -->

### The Raw Body in filter-update

A filter reads the route through the fields it declared (protocol.md, "The
Route Fields of filter-update"). A filter that must see the UPDATE as encoded,
such as the AS_PATH and AS4_PATH attributes as they arrived or the NLRI of a
family the vocabulary does not split, declares the `raw` read and parses the
UPDATE body (RFC 4271 Section 4.3, without the 19-byte header) from
`FilterUpdateInput.Raw`. Such a filter may answer a whole-body rewrite in
`FilterUpdateOutput.Raw`, which ends the chain's members for that route
(protocol.md, "The Sets of a filter-update answer").
<!-- source: pkg/plugin/rpc/types.go -- FilterFieldRaw, FilterUpdateInput, FilterUpdateOutput -->
<!-- source: internal/component/plugin/registration.go -- FilterRegistration.Raw -->
<!-- source: internal/component/bgp/policychain/external.go -- rawAnswer -->

If an original chain input carries valid ATOMIC_AGGREGATE, Ze restricts any
modified result that still announces routes. The announcements must retain
that marker and form an exact native-key subset of the original input,
including family and received ADD-PATH identity. Attribute edits, reordering, subset removal and pure
withdrawal/EOR output remain supported. New marked keys, including
less-specific or disjoint raw rewrites, and marker removal are unavailable in
this release. This is a product restriction stronger than RFC 4271 Section
5.1.6. Unmarked original input retains its existing rewrite facility.
<!-- source: internal/component/bgp/reactor/filter_library.go -- runLibraryChain -->
<!-- source: internal/component/bgp/reactor/filter_atomic_aggregate.go -- atomicAggregatePolicyAllowed -->

The same post-chain admission applies to raw and typed results on both
transports and both directions, including final-judge and stale passes. A
refusal uses the source route's withdraw outcome. It is not a filter transport
failure subject to `on-error`. No request or response fields are added.
<!-- source: internal/component/bgp/reactor/filter_library.go -- runLibraryChain, runFinalJudges -->
<!-- source: internal/component/bgp/reactor/filter_ordered.go -- runIngressPolicyChainMeta, runEgressPolicyChainASN4 -->
<!-- source: internal/component/bgp/reactor/stale_readvertise_chain.go -- runStaleChain -->
<!-- source: pkg/plugin/rpc/types.go -- FilterFieldRaw, FilterUpdateInput, FilterUpdateOutput -->

An accepted marked subset also carries removals for source keys absent from
the final result. These removals include original withdrawn siblings that a
raw answer omitted. A final
output announcement or withdrawal covers the same native identity. A changed
final result recomputes the removals against the original input. An unchanged
final pass retains the preceding result and removals.
<!-- source: internal/component/bgp/reactor/filter_atomic_subset.go -- atomicAggregatePolicyRemovals -->
<!-- source: internal/component/bgp/reactor/filter_library.go -- runLibraryChain, runFinalJudges -->

Import publishes these internal, source-owned continuations before the
effective result or EOR. Export admits them first under the grouped write
fence and does not run the filter again. An active RIB recovery owner that
receives the source can defer a non-ADD-PATH removal to an asynchronous election.
That election later withdraws the key or installs an export-permitted replacement.
Later output does not prove that election complete. No more plugin request, response
or event fields are required.
<!-- source: internal/component/bgp/reactor/filter_atomic_subset.go -- atomicAggregatePolicyRemovals -->
<!-- source: internal/component/bgp/reactor/filter_library.go -- runLibraryChain, runFinalJudges -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyMessageReceiver, notifyOneMessage -->
<!-- source: internal/component/bgp/reactor/forward_policy_removal.go -- writePolicyRemovals -->
<!-- source: internal/component/bgp/reactor/session_ownership.go -- owesWithdrawal -->
<!-- source: internal/component/bgp/reactor/recovery_owed.go -- recoveryOwedFor, run, decide -->

**Subscription fields:**
<!-- source: pkg/plugin/rpc/types.go -- SubscribeEventsInput -->

| Field | Type | Description |
|-------|------|-------------|
| `events` | `[]string` | Event types (e.g., `["update","state"]`); `"*"` expands to all event types of the namespace |
| `peers` | `[]string` | Peer filter (e.g., `["*"]` for all) |
| `format` | `string` | Format preference (`"parsed"`, `"full"`, `"hex"`, or `"base64"`) |
| `encoding` | `string` | `"json"` (default) or `"text"` |
| `namespace` | `string` | Event namespace; empty resolves to the protocol component's default (`bgp`), non-empty (e.g. `vpn-ipsec`) subscribes to another namespace (see Subscription Namespace) |
| `envelope` | `bool` | When true, deliveries are wrapped in an `EventEnvelope` (see Enveloped Delivery); default false = bare payload |

---

## Internal Plugin Invocation Modes

Ze plugins run as **long-lived processes** (goroutines for Go, subprocesses for external).
Each plugin registers the families it handles at startup, then processes requests in a loop.

### Architecture Overview

```
+--------------------------------------------------------------------------+
|                              ENGINE                                       |
|                                                                           |
|   +------------------------------------------------------------------+   |
|   |                        Family Registry                            |   |
|   |   ipv4/flowspec     -> flowspec plugin                            |   |
|   |   ipv6/flowspec     -> flowspec plugin                            |   |
|   |   ipv4/flowspec-vpn -> flowspec plugin                            |   |
|   |   ipv6/flowspec-vpn -> flowspec plugin                            |   |
|   +------------------------------------------------------------------+   |
|                                    |                                      |
|                        RPC (MuxConn / DirectBridge)                       |
|                                    |                                      |
|   +------------------------------------------------------------------+   |
|   |       FLOWSPEC PLUGIN (long-lived goroutine / process)            |   |
|   |                                                                    |   |
|   |  1. 5-stage startup (YANG RPCs)                                   |   |
|   |  2. Event loop (encode/decode callbacks)                          |   |
|   +------------------------------------------------------------------+   |
+--------------------------------------------------------------------------+
```

### Automatic OPEN Capability Injection

**Key design:** When a plugin declares `decode` for a family, the engine automatically
advertises that family in OPEN messages via Multiprotocol capability (Code 1).

**Rationale:**
- If a plugin can decode a family, peers should be able to send it
- No explicit capability declaration needed for Multiprotocol
- Reduces protocol overhead and prevents duplicate capability issues

**How it works:**

```
Plugin Stage 1: declare-registration with family ipv4/flow mode=decode
                     |
Registry: families["ipv4/flow"] = "flowspec"
                     |
Session.sendOpen(): GetDecodeFamilies() -> ["ipv4/flow", ...]
                     |
OPEN: Multiprotocol(AFI=1, SAFI=133)
```

**Override behavior:** Config families completely override plugin families:
- Config has `family {}` block: ONLY config families used, plugin families ignored
- Config has NO `family {}` block: plugin decode families used

This is intentional: explicit config = full control. Plugin families provide defaults
when config doesn't specify families.

**Auto-loading plugins:** When a family is configured but no plugin has claimed it,
the engine automatically loads the internal plugin for that family (if one exists).

**Three-phase plugin startup:**
1. **Phase 1:** Config-path plugins start first (for example, BGP, interface, and FIB infrastructure plugins), together with any already-selected explicit prerequisites they need.
2. **Phase 2:** Remaining explicit plugins from `bus { endpoint ... }` start after config-path infrastructure is available.
3. **Phase 3:** The engine checks which configured families are still unclaimed. Internal plugins are auto-loaded only for unclaimed families.

Custom event-type and send-type auto-load phases are deleted: a registered reception whose sender is not loaded stays dormant and is named by `show config running | implied` and `show doctor`.

Before Phase 1 starts any plugin, the engine resolves the dependencies of the Phase 1 and Phase 3 auto-load sets. A plugin whose `Dependencies` names a plugin this build does not register refuses startup, naming the plugin, the dependency and the build tags that carry it; no plugin starts. It is never loaded without its dependency. Every resolution judges the startup config's needs: an optional dependency that a `ConfigNeeds` entry names is auto-loaded only when the config holds that need, so flow export with `enrichment bgp` off does not start `bgp-rib`.
<!-- source: internal/component/plugin/server/startup.go -- runPluginStartup, refuseBootAutoLoad -->

**Family auto-loading** (Phase 3) is **prevented** when:
1. An explicit plugin declares `decode` for the family (family-based check)
2. `--plugin <name>` is passed on command line (prevents auto-load for that plugin)

The check is based on **family claims**, not plugin name. Plugin names are informational only.

| Config | Plugin | Result |
|--------|--------|--------|
| `family { ipv4/flow; }` | None | Auto-loads `bgp-nlri-flowspec` |
| `family { ipv4/flow; }` | `--plugin bgp-nlri-flowspec` | Uses explicit plugin (no auto-load) |
| `family { ipv4/flow; }` | `bus { endpoint my-traffic { external { run ...; } } }` declaring `ipv4/flow` at Stage 1 | Uses config plugin (no auto-load, family claimed) |
| `family { ipv4/foo; }` | None | Startup fails (no plugin for family) |

**Functional tests:**
- `test/plugin/flowspec-open-capability.ci` - auto-load for known family
- `test/plugin/family-no-plugin-failure.ci` - failure for unknown family
- `test/plugin/explicit-plugin-precedence.ci` - explicit `--plugin` prevents auto-load
- `test/plugin/explicit-plugin-config.ci` - config plugin prevents auto-load

**Ordering:** Plugin families are sorted alphabetically for deterministic OPEN messages.

**What plugins should NOT do:**
- Send `declare-capabilities` with Multiprotocol (Code 1) for their families
- Assume plugin families will be used if config has a `family {}` block

**What plugins SHOULD do:**
- Declare `decode` for all families they can parse (provides defaults)
- Use `declare-capabilities` only for non-Multiprotocol capabilities (GR, hostname, etc.)

### NLRI Routing via Engine RPCs

NLRI encode/decode requests are routed via the engine's plugin registry:
<!-- source: pkg/plugin/sdk/sdk_engine.go -- EncodeNLRI, DecodeNLRI -->

| Direction | RPC Method | Input | Output |
|-----------|-----------|-------|--------|
| Plugin to Engine | `ze-plugin-engine:encode-nlri` | `{"family":"...","args":[...]}` | `{"hex":"..."}` |
| Plugin to Engine | `ze-plugin-engine:decode-nlri` | `{"family":"...","hex":"...","add-path":<bool>,"withdraw":<bool>}` | `{"json":<raw JSON>}` |
| Engine to Plugin | `ze-plugin-callback:encode-nlri` | `{"family":"...","args":[...]}` | `{"hex":"..."}` |
| Engine to Plugin | `ze-plugin-callback:decode-nlri` | `{"family":"...","hex":"...","add-path":<bool>,"withdraw":<bool>}` | `{"json":<raw JSON>}` |

Both directions use `DecodeNLRIInput`. `add-path` preserves the negotiated
four-octet Path Identifier framing, including identifiers of zero. `withdraw`
selects MP_UNREACH withdrawal semantics; false or absent means announcement.
These facts come from negotiation and the enclosing message, not from the NLRI
octets. In particular, RFC 8277 Section 2.4 requires receivers to ignore the
withdrawal Compatibility field's value rather than decode it as a label stack.
The SDK callback and registry share
`func(family, hex string, addPath, withdraw bool) (any, error)`; the flags retain
the same meaning on both transports, with no separate RPC or registry mode.

The response's `json` field is a JSON document, not a quoted string containing
one. The SDK marshals the decoder's object or array once; `SendDecodeNLRI`
reads it as `DecodeNLRIOutput.JSON` (`json.RawMessage`) and returns those
document bytes as its string result. Singleton and packed-section responses
use the same contract, with no string-encoded alternative.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnDecodeNLRI -->
<!-- source: internal/component/plugin/ipc/rpc.go -- SendDecodeNLRI -->

**How it works:**
1. Plugin calls `EncodeNLRI`/`DecodeNLRI` via engine RPC
2. Engine looks up the family plugin via `registry.LookupFamily()`
3. Engine sends callback to the appropriate family plugin
4. Family plugin processes and returns result

For in-process plugins with `DirectBridge`, the RPC path is replaced by direct
function calls, bypassing JSON marshaling and pipe I/O entirely.

### Mode 1: In-Process (goroutine + net.Pipe + DirectBridge)

For Go plugins (`ze.pluginname`) -- runs in same process:
<!-- source: pkg/plugin/sdk/sdk.go -- NewWithConn, Run bridge activation -->

1. `startInternal()` creates a single `net.Pipe` for bidirectional YANG RPC
2. Creates a `DirectBridge` and wraps the plugin-side connection in `BridgedConn`
3. Runner goroutine receives `BridgedConn` (implements `net.Conn`) transparently
4. SDK discovers bridge via `Bridger` type assertion in `NewWithConn()`
5. 5-stage startup runs over sockets (cold path, 5 round-trips total)
6. After Stage 5: bridge activates for direct function calls (hot path)

**Bridge activation sequence:**

| Step | Side | Action |
|------|------|--------|
| 1 | Engine | On reading the Stage 5 ready request, atomically publishes the negotiated bridge with `conn.SetBridge(bridge)` (`publishTransport`), before registering the startup subscriptions |
| 2 | Engine | `wireBridgeDispatch()` registers `DispatchRPC` handler on bridge, then the final startup barrier runs and runtime readiness is signaled |
| 3 | Engine | Sends Stage 5 OK response over pipe (last pipe message) |
| 4 | SDK | Receives OK, registers `DeliverEvents` handler on bridge |
| 5 | SDK | Calls `bridge.SetReady()` -- bridge now active |
| 6 | SDK | Closes pipe (`engineMux.Close()`), enters `bridgeEventLoop` |
| 7 | Engine | Joins the queued `runtime-ready` answer before startup succeeds, including when no initializer was declared |

The engine wires its handler and publishes the negotiated callback transport
before acknowledging ready. Each callback selects one published bridge; it does
not fall back to IPC if that bridge stops. Liveness still uses startup IPC until
the SDK activates the bridge, then follows sticky activation even after shutdown.
If IPC closes while liveness is being read, the engine rechecks activation before
reporting the IPC failure. The SDK publishes readiness before sticky activation,
so an observer cannot confuse incomplete activation with shutdown.

The startup owner MUST join the queued `runtime-ready` answer for every
negotiated bridge. Its SDK default handler needs no `OnStarted` declaration.
The SDK loop can answer only after bridge activation. This protects typed
mutation and snapshot callbacks from a false closed-bridge result between the
final response write and SDK activation. A stopped bridge still refuses calls
and never falls back to the startup socket.

This also applies to `deliver-batch` events queued between bridge publication
and SDK activation. They wait on the negotiated callback channel, not the
startup pipe that the SDK is about to close. Publication comes before the
startup subscriptions register because an event can reach the plugin from
that moment: another plugin that left the barrier first may emit while this
one still waits for its final OK. Sent on the pipe, such an event is never
read, and the engine's delivery fails with EOF once the SDK closes the pipe. The batch keeps each event's JSON
unchanged and is applied once; transport cutover does not retry it. In particular,
GR must receive both OPEN directions before DOWN can use their capability state.
<!-- source: internal/component/plugin/ipc/rpc.go -- CallBatchRPC -->
<!-- test: internal/component/plugin/ipc/rpc_bridge_batch_sdk_test.go TestPluginConnBridgeBatchCutover -->
<!-- test: internal/component/bgp/reactor/llgr_down_writer_test.go TestLLGRActualDownKeepsWriterOwner -->

After activation the pipe is fully shut down, the MuxConn read loop exits, and
engine-to-plugin callbacks flow through the bridge callback channels. A failed
final OK leaves startup unsuccessful: the engine records the error and restores
the pre-running stage so startup rollback stops the process and closes its bridge.
<!-- source: pkg/plugin/sdk/sdk.go -- Run, bridge activation -->
<!-- source: internal/component/plugin/server/startup.go -- handleProcessStartupRPC -->
<!-- source: internal/component/plugin/ipc/rpc.go -- SetBridge -->

**Engine-side dispatch registry:** a plugin-to-engine RPC's socket JSON path,
in-process Direct path, and typed `DirectBridge` adapters derive from one method
registry. Each `engineOp` entry carries its wire method, a raw handler receiving
the request context and process, and optional typed-slot or native-value adapters. `wireBridgeDispatch`
installs the typed slots by iterating the entries that declare a descriptor (not a
hand-written `Set*` list), and `dispatchPluginRPC` / `dispatchPluginRPCDirect` resolve
the method through the same table, so adding an operation touches one place and the
JSON / Direct / bridge paths cannot drift. The `rpc.Method*` constants are shared with
the SDK caller so the sent and dispatched method strings stay in lockstep.
<!-- source: internal/component/plugin/server/dispatch_registry.go -- engineOps, engineOp, lookupEngineOp -->
<!-- source: internal/component/plugin/server/dispatch.go -- dispatchPluginRPC, dispatchPluginRPCDirect, wireBridgeDispatch -->

An owner adds operations from its `register.go` initializer through
`server.RegisterEngineOp(EngineOperation{...})`. Registration derives the method
namespace from the caller's package, validates the handler and explicit startup
policy, and applies the shared IPC method syntax and length limit. It refuses
duplicate methods and writes after `registry.Seal`. The
operation joins the same table as builtins, including optional typed bridge
wiring; JSON and Direct retain the same result and error contract. A handler
receives the actual `*process.Process`; names in its JSON are not authenticated
identities. Owners must authorize the configured registration and scope leases
or sessions by that process generation. `RegisterProcessGenerationCleanup`
receives the same identity after admitted requests drain, before replacement,
alongside the existing name-only cleanup hooks.
<!-- source: internal/component/plugin/server/register_engine_op.go -- RegisterEngineOp, RegisterProcessGenerationCleanup -->
<!-- test: internal/component/plugin/server/engine_op_register_test.go TestOwnerEngineOperationTransportParity -->

An owner's optional `Native` handler receives self-contained Go values through
`DispatchNative`, without JSON conversion. The required raw handler keeps the
operation available to external plugins. Both handlers MUST validate their input
and use the same evaluator. Native dispatch preserves caller context, uses the
existing request panic guard, and joins the bridge's dispatch drain. It is
available only after readiness; configure-safe startup calls retain their existing
JSON admission. A native miss permits the caller to choose its public RPC form.
An activated bridge's shutdown or failure returns an error, including through
ordinary SDK RPC calls, and never returns to startup IPC.
<!-- source: internal/component/plugin/server/dispatch_native.go -- dispatchPluginRPCNative, engineOpCallError -->
<!-- source: pkg/plugin/rpc/bridge_native.go -- DispatchNative -->
<!-- source: pkg/plugin/sdk/sdk_native.go -- TryEngineNative, engineBridgeActive -->
<!-- source: pkg/plugin/sdk/sdk.go -- callEngineRaw -->

### System routing views

The system RIB registers six `ze-sysrib` operations with the shared engine
registry. The handlers receive the authenticated process generation. Only the
configured `rib` implementation can open a session, including an aliased instance.
Payload fields cannot select another process's identity.
<!-- source: internal/component/sysrib/register_route_view.go -- init, routeViewOpenHandler -->
<!-- source: internal/component/sysrib/route_view.go -- routeViewAuthorize, match -->

| Method suffix | Request | Result | Startup admission |
|---------------|---------|--------|-------------------|
| `route-view-open` | Optional reader: `selected-feed` or `local-authority` | Session, epoch, snapshot and initial cursor zero | Configure-safe |
| `route-view-read` | Session, consumed cursor, limit and `wait-ms` | Ordered snapshot, delta or reset page, cursor and completion flag | Runtime |
| `route-view-candidates` | Session, independent snapshot/cursor, limit and optional family/prefix | Whole-prefix candidate JSON, including hidden paths | Runtime |
| `route-view-distances` | Session, positive operation and complete protocol/distance table | Applied operation and authoritative selected frontier | Configure-safe |
| `route-view-publish` | Session, consumed cursor, positive operation and existing best-change batch | Accepted operation and cursor | Runtime |
| `route-view-close` | Session | Session after fencing and cleanup | Runtime |
<!-- source: pkg/plugin/rpc/route_view.go -- RouteViewOpenInput through RouteViewCloseOutput -->
<!-- source: internal/component/sysrib/yang/ze-sysrib-api.yang -- route-view-open through route-view-close -->
<!-- source: internal/component/sysrib/register_route_view_policy.go -- init -->
<!-- source: internal/component/sysrib/register_route_view_output.go -- init -->

`selected-feed` is the default. It transfers only the daemon's selected routes,
with resolved distance, metric, tag/AIGP presence, target ownership, multipath,
labels, Service SID and repair metadata. The receiver completes its snapshot
before forwarding. Reset invalidates the projection and requires a fresh view.
An empty delta leaves the projection intact. `local-authority` requires an
internal authenticated instance and uses the daemon reader at cursor zero.
<!-- source: pkg/plugin/rpc/route_view.go -- RouteViewPath, RouteViewNextHop -->
<!-- source: internal/component/sysrib/route_view.go -- open, read -->
<!-- source: internal/component/sysrib/projection.go -- Apply, Synchronized -->

Selected and candidate pages contain at most 4096 records and must fit the
complete RPC frame. Selected reads wait at most 1000 milliseconds. A read
whose wait the daemon's stop, or the stop of the owner's process generation,
ends gets no answer: the connection close answers it ("Shutdown"). Mandatory
zero-valued fields remain explicit, including cursor and `wait-ms`. Candidate
snapshots have independent identities: a selected revision cannot validate a
candidate scan after a hidden-only edit.
<!-- source: pkg/plugin/rpc/route_view_input_codec.go -- RouteViewReadInput.UnmarshalJSON, RouteViewRequestBudget -->
<!-- source: internal/component/sysrib/route_view_candidates.go -- readCandidates, page -->

Distance acknowledgment follows validation of the complete schema-derived table,
daemon publication and reselection. Retrying its ordered identity with the same
values returns the original acknowledgment. A valid large-table change can
acknowledge a reset frontier, which requires reconstruction before output resumes.
<!-- source: internal/component/sysrib/route_view_distances.go -- applyDistances, distanceBarrier, routeViewDistanceTable -->

Forwarding acknowledgment confirms event delivery, not backend installation.
Identical publication retries do not emit twice. New operations require the
latest consumed cursor. Reset, close and process cleanup fence the session and
withdraw its outstanding output. Failed withdrawal retains ownership and blocks
replacement until cleanup succeeds. The downstream BGP and FIB payloads retain
their existing formats.
<!-- source: internal/component/sysrib/route_view_output.go -- publish, withdraw -->
<!-- source: internal/component/sysrib/route_view.go -- cleanupLocked -->

System replay requests admit only the Broadcast `replay-id`. Zero and targeted
tokens receive an operator refusal before the request queue changes, with
`ze request reload` as the recovery command. Both placements keep useful live
forwarding after that refusal. Repeated broadcast arrivals retain their own
revision until acknowledged. Before answering, the local reader applies its
captured authority changes, just as the selected reader applies its next page.
<!-- source: internal/core/replay/replay.go -- Broadcast -->
<!-- source: internal/component/sysrib/route_view_worker.go -- HandleEvent, replay -->
<!-- source: internal/component/sysrib/route_view_worker_local.go -- applyLocal -->

A valid publication can arrive after its selected feed was fenced. Its refusal
is recoverable only when a read on that same authenticated lease returns an
explicit Reset with `rejected-publication` equal to the refused operation.
This optional positive identity proves complete batch validation and refusal
before emission. It grants no forwarding acknowledgment. Only an exact retry
of that retained batch, cursor and operation can repeat the proof; another
admitted malformed or mismatched publication clears its visibility. Delivery
failure, failed cleanup, foreign ownership and replacement grant no proof.
The consumer validates the reset's epoch and shape before reconstructing its
current view. Error text never decides recovery.
<!-- source: internal/component/sysrib/route_view_output.go -- publish, provePublicationReset -->
<!-- source: internal/component/sysrib/route_view_worker_output.go -- publicationReset -->
<!-- source: pkg/plugin/rpc/route_view_codec.go -- RouteViewReadOutput.UnmarshalJSON -->

The SDK's [routing-view reference](../../plugin-development/protocol.md#system-routing-views)
defines the public types and calls. The [Loc-RIB architecture](../rib/unified-locrib.md#authoritative-selected-views)
defines snapshot retention, overflow and worker lifetimes.

### Owned interface devices and firewall table contribution

The interface and firewall components each register engine operations in their
own namespace, so a plugin that owns kernel state reaches them over the direct
path internal and over RPC forked. The kernel state they create is the
interface component's and the firewall component's, never the plugin's: the
plugin holds no netlink call or nftables call. Compiled-in plugins key firewall
table reservations and interface owned-device grants on their compiled
registration identity (`proc.RegistrationName()`) under any endpoint alias or
transport (`in-daemon` or `process separate;`), never a name in the request,
while foreign `external` programs hold none of those grants even if their
endpoint name matches a compiled-in plugin. Per-process runtime ledgers
(`metrics-publish`, `report-issue`, `release-cached`) remain keyed on the
endpoint name (`proc.Name()`).
<!-- source: internal/component/iface/cmd/owned_devices.go -- init, ownedBridgesReplaceHandler -->
<!-- source: internal/component/firewall/register_contribution.go -- init, tablesReplaceHandler -->

| Method | Request | Result | Startup admission |
|--------|---------|--------|-------------------|
| `ze-iface:owned-bridges-replace` | The caller's complete set of bridges: name, MTU and addresses | The bridges the caller owns after it | Configure-safe |
| `ze-iface:owned-link-create` | A record key, the owned bridge, a slot, the process id and namespace inode of the target network namespace, the peer end's name, hardware address, MTU, addresses and gateways | The host end's name | Configure-safe |
| `ze-iface:owned-link-remove` | A record key | The host ends removed | Configure-safe |
| `ze-firewall:tables-replace` | The caller's complete set of tables as a firewall config section, `{"firewall":{"table":{...}}}` | The kernel tables the caller contributes after it | Configure-safe |
<!-- source: pkg/plugin/rpc/owned_devices.go -- OwnedBridgesReplaceInput through OwnedLinkRemoveOutput -->
<!-- source: pkg/plugin/rpc/firewall_tables.go -- FirewallTablesReplaceInput, FirewallTablesReplaceOutput -->
<!-- source: internal/component/iface/yang/ze-iface-owned-api.yang -- owned-bridges-replace through owned-link-remove -->
<!-- source: internal/component/firewall/yang/ze-firewall-api.yang -- tables-replace -->

Who may call them is decided by declarations made from `init()`, so every
process of the ze binary holds the same set and a plugin built outside the
binary holds none:

- The interface requests are accepted only from an owner that granted itself
  device name prefixes with `iface.MustGrantOwnedDeviceNames`. Any other caller
  is refused with an operator error before its request is read.
- The firewall request is accepted only from an owner that reserved table names
  with `firewall.MustReserveTableNames`, and only for tables it reserved. A name
  another owner reserved, and a `ze_` name no owner reserved (an operator's
  firewall table), are refused. A section with no `firewall` table map is
  refused rather than read as the withdrawal of every table: the withdrawal is
  an explicit empty table map.
<!-- source: internal/component/iface/device_owner.go -- MustGrantOwnedDeviceNames, RefuseUngrantedDeviceOwner -->
<!-- source: internal/component/firewall/contribution.go -- refuseOwnerWithoutReservation, refuseUnreservedContribution -->
<!-- source: internal/component/firewall/register_contribution.go -- refuseSectionWithoutTableMap -->

Each request is decoded strictly: one JSON object holding only the declared
fields, within the frame bound on both transports
(`pluginserver.DecodeEngineOperationInput`).
<!-- source: internal/component/plugin/server/engine_op_decode.go -- DecodeEngineOperationInput -->

The three interface requests change the kernel before they answer, so a
plugin's answer is the kernel's outcome, never a promise of a later pass:

- `owned-bridges-replace` makes the request the caller's complete set of
  bridges, then runs the owned-device pass of the interface reconcile. A bridge
  left out of the set is deleted, and so is every link of the caller that
  joined it. An empty set withdraws every bridge and link the caller holds.
  When the kernel refuses a step, the steps the pass made are undone, the
  previous set is restored and reconciled once more, and the request is
  refused with an operator error.
- `owned-link-create` names the target network namespace by a process id and
  that namespace's inode number together. The handler opens
  `/proc/<pid>/ns/net` once, reads the inode of the opened descriptor, refuses
  the request when it differs, and creates the link through that same
  descriptor, so a process id reused in between cannot redirect the link. The
  bridge must be in the caller's bridge set and carry the caller's mark. The
  host end is named from the caller's first granted prefix, the bridge's
  interface index and the slot. The peer end is created inside the namespace
  under its final name, then addressed, set up and given one default route per
  gateway. When any step fails the pair is deleted and nothing is recorded.
- `owned-link-remove` deletes every host end recorded under the key and
  forgets the record. A host end that is already gone, because the kernel
  removed the pair with its namespace, is forgotten without a call. A host end
  the kernel refuses to delete keeps its record and fails the request.
- A request refused before the kernel is reached changes nothing: an owner
  with no grant, a device name outside the caller's prefixes, an MTU outside 68
  to 16000, more than 64 bridges in a set, 16 addresses on a device or 1024
  links held by one owner, or a record key over 64 bytes.
<!-- source: internal/component/iface/owned_bridge_link.go -- ReplaceOwnedBridges, CreateOwnedLink, RemoveOwnedLink, ownedBridgeSpecs, refuseOwnedLinkInput -->
<!-- source: internal/component/iface/owned_namespace_linux.go -- openOwnedLinkNamespace -->
<!-- source: internal/plugins/iface/netlink/owned_link_linux.go -- CreateOwnedLink, configureOwnedPeer -->

When a plugin process ends, a process-generation cleanup withdraws the bridges
and links that process declared, and the reconcile deletes their devices. An
older process of the same name that ends after its replacement declared
withdraws nothing. A withdrawal that fails is logged with the code
`doctor-iface-owned-device-cleanup`.
<!-- source: internal/component/iface/cmd/owned_devices.go -- ownedDevicesCleanup, noteOwnedDeviceCaller -->
<!-- source: internal/component/iface/owned_bridge_link.go -- WithdrawOwnedDevices -->

The firewall request changes the kernel before it answers too.
`tables-replace` parses the section with the parser of the operator's firewall
config, validates the tables as that config is validated, makes them the
caller's complete set in the table registry under its authenticated name, and
reconciles the merged set of every owner, the backend's verifier included. The
answer names the kernel tables the caller contributes. An empty table map
withdraws them all. When the backend refuses the set, the caller's previous set
is registered and reconciled again and the request is refused with an operator
error, so the kernel keeps what it held.
<!-- source: internal/component/firewall/register_contribution.go -- tablesReplaceHandler -->
<!-- source: internal/component/firewall/contribution.go -- replaceContributedTables, restoreContributedTables -->

Contributed tables take the path configured tables take, so they coexist with
them: a config commit registers only the operator's owners and leaves the
contributed tables in place, and the firewall engine's orderly stop removes
every Ze table, contributed ones included. When a plugin process ends, its
generation cleanup withdraws the tables that process contributed and
reconciles; an older process of the same name that ends after its replacement
contributed withdraws nothing. A restore or a withdrawal that the backend
refuses has no caller to answer, so it is logged and raised with the code
`doctor-firewall-contributed-tables`, which the doctor check
`firewall-contributed-tables` reports until the next successful reconcile
clears it.
<!-- source: internal/component/firewall/contribution.go -- withdrawContributedTables, raiseContributionFailure -->
<!-- source: internal/component/firewall/doctor_contribution.go -- checkContributedTables, clearContributionWarnings -->

### Direct bridge calls and shutdown

Generic direct calls use `DispatchRPC(ctx, method, params)`. The typed route and
command slots also carry the SDK caller's context into `CommandContext`.
For batch route announce and withdrawal, that context reaches write admission
and the peer socket write and flush. Cancellation ends the operation itself,
not just a goroutine waiting for an uncancelled writer.
<!-- source: pkg/plugin/sdk/sdk.go -- callEngineRaw -->
<!-- source: pkg/plugin/rpc/bridge.go -- DispatchRPC, UpdateRouteSel, DispatchCommand -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- AnnounceNLRIBatch, WithdrawNLRIBatch -->

**Runtime hot path (after bridge activates):**

| Direction | Socket path (before) | Direct path (after) |
|-----------|---------------------|---------------------|
| Engine to Plugin events (text) | RPC envelope -> newline frame -> `net.Pipe.Write` -> read -> unmarshal -> `onEvent` | `bridge.DeliverEvents(events)` -> `onEvent` directly |
| Engine to Plugin events (structured) | -- | `bridge.DeliverStructured([]any)` -> `onStructuredEvent` with `*StructuredEvent` (no text formatting, no JSON parsing) |
| Plugin to Engine RPCs (generic) | `json.Marshal` -> newline frame -> `net.Pipe.Write` -> read -> unmarshal -> `dispatcher.Dispatch` | `bridge.DispatchRPC(ctx, method, params)` -> `dispatcher.Dispatch` directly |
| Plugin to Engine dispatch-command | JSON marshal `DispatchCommandInput` -> RPC -> unmarshal -> dispatch | `bridge.DispatchCommand(ctx, command)` -> `dispatchCommand()` directly (struct passthrough, no serialization) |
| Plugin to Engine dispatch-command-args | JSON marshal `DispatchCommandArgsInput` -> RPC -> unmarshal -> exact command route | `bridge.DispatchCommandArgs(ctx, command, args, peer)` -> `dispatchCommandArgs()` directly (Go args slice, no tokenizer) |
| Plugin to Engine emit-event | JSON marshal `EmitEventInput` -> RPC -> unmarshal -> deliver | `bridge.EmitEvent(namespace, eventType, ...)` -> `deliverEvent()` directly (Go strings, no JSON) |
| Engine to Plugin callbacks | MuxConn RPC + 3-way select | `bridge.SendCallback()` -> callback channel -> `bridgeEventLoop` 2-way select |
| Engine to Plugin `filter-update` | `Server.CallFilterUpdate` -> `PluginConn.CallFilter` -> `DirectBridge.CallFilterSlot` when the plugin installed a filter slot (`sdk.OnFilter` at Stage 5), else `SendFilterUpdate` (queued on the connection's batch worker) | Typed slot: the evaluator is called on the engine's goroutine with the engine's lazy view (`rpc.FilterView`), nothing marshaled and nothing queued on `callbackCh`; a fast-path route is encoded only when the evaluator asks for `Fields`, or when the route goes to a forked plugin's batch. A closed slot (before Stage 5, after `CloseCallbacks`, after a callback failure) answers `ErrBridgeClosed`, which `Server.CallFilterUpdate` reports as the plugin being unavailable, and the filter's `on-error` applies; once a slot existed the route never falls back to the queue |
<!-- source: pkg/plugin/rpc/filter_slot.go -- CallFilterSlot -->
<!-- source: internal/component/plugin/ipc/filter_slot.go -- CallFilter -->

**Callback dispatch:** Both event loops (pipe and bridge) dispatch through a generic
callback registry (`map[string]callbackHandler`). Each `On*` method registers a typed
wrapper in the map. Adding a new callback requires only one `On*` method -- zero changes
to the dispatch or event loop code. See `rules/plugin-design.md` "SDK Is Generic".
Callbacks that wait for later events register an asynchronous worker factory.
Both loops use the same factory map for stop `drain`, `source-policy` and
`event-settlement`; adding a worker changes no method list in either loop.
Each worker owns its queue, execution and stop/join obligations.
<!-- source: pkg/plugin/sdk/sdk_async_callback.go -- setAsyncCallback, startAsyncCallbacks -->
Current-selection and original admission lookup use registered ingress lanes
before that loop and its queue. Each has its own worker on both transports.
Snapshot reads must complete even when an ordinary handler waits for engine
dispatch. Admission through that blocked ordinary loop would deadlock.
<!-- source: pkg/plugin/sdk/sdk_origin_admission_lookup.go -- startOriginAdmissionLookupWorker -->
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- eventLoop, bridgeEventLoop, getCallback -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- initCallbackDefaults, On* methods -->

**Shutdown and callback failure:** `DirectBridge.CloseCallbacks` fences event
delivery and plugin-to-engine dispatch. It closes `callbacksStopped` before
waiting for callback-send admission, waking full-queue senders and unanswered
result waiters, then closes both callback channels. Buffered replies remain
authoritative; a callback panic remains an `ErrBridgeFailed` cause.
`CloseCallbacks` and SDK `Plugin.Close` do not join handlers, so a handler can
request its own shutdown. SDK `Plugin.Run` drains admitted event deliveries on
every return path; its owner must await that return before releasing handler
state. `WaitDelivery` belongs outside handlers. This drains admitted work, not
every queued event, and does not forcibly cancel arbitrary handler work.
<!-- source: pkg/plugin/rpc/bridge.go -- CloseCallbacks, StopDelivery, WaitDelivery, SendCallback, ExecuteCommand -->
<!-- source: pkg/plugin/sdk/sdk.go -- Close, Run -->
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- bridgeEventLoop -->

**Files:**

| File | Purpose |
|------|---------|
| `pkg/plugin/rpc/bridge.go` | `DirectBridge`, `BridgedConn`, `Bridger`, `BridgeCallback`, `SendCallback`, `CloseCallbacks` |
| `internal/component/plugin/server/dispatch_registry.go` | `engineOp`, `engineOps` (the unified plugin-to-engine method registry), `serveEngineOpJSON`, `serveEngineOpDirect` |
| `pkg/plugin/sdk/sdk_callbacks.go` | `initCallbackDefaults`, `On*` wrappers, `callbackHandler` registry |
| `pkg/plugin/sdk/sdk_dispatch.go` | `eventLoop`, `bridgeEventLoop`, `getCallback` -- generic dispatch |
| `internal/component/plugin/process/process.go` | Bridge creation in `startInternal()`, bridge check in `deliverBatch()`, `CloseCallbacks` in `Stop()` |
| `internal/component/plugin/ipc/rpc.go` | `PluginConn.SetBridge()`, `CallRPC` bridge routing |
| `internal/component/plugin/server/startup.go` | Bridge transport publication before Stage 5 OK |
| `pkg/plugin/sdk/sdk.go` | Bridge discovery, `callEngineRaw()` bridge path, `SetReady()`, pipe close |

### Mode 2: Subprocess (TLS connect-back)

For external plugins (Python, Rust, etc.) -- runs as separate process:
<!-- source: pkg/plugin/sdk/sdk.go -- NewFromTLSEnv -->

1. Engine starts TLS listener from `bus { hub { server <name> { ip ...; port ...; secret ...; } } }` config
2. Engine forks `/bin/sh -c <run>` as the child, with env vars: `ZE_PLUGIN_HUB_HOST`, `ZE_PLUGIN_HUB_PORT`, `ZE_PLUGIN_HUB_TOKEN` (per-plugin unique token), `ZE_PLUGIN_CA_PEM` (the PEM certificate authority root that issued the listener's certificate), `ZE_PLUGIN_NAME`
3. Child validates the engine certificate chain against that root and nothing else, then authenticates with `#0 auth {"token":"...","name":"..."}`
4. Engine validates token matches the per-plugin token generated for that name (name binding prevents impersonation)
5. Token is cleared from the child's OS environment after first read (`Secret: true` registration)
6. Single bidirectional connection using `MuxConn` (responses routed by `#<id>`, requests via `Requests()` channel)
7. No `DirectBridge` -- always uses newline-framed RPC over TLS
8. Same 5-stage handshake over the same connection

#### A refused connect-back

The engine answers a refused `#0 auth` with an error line and closes the
connection. It refuses a claimed name that is not 1 to 64 letters, digits and
hyphens, the first a letter or a digit (the form the commit also requires of a
`bus endpoint` name), a token that is not the one bound to the claimed
name, and any request when no token is bound to the name and no shared secret
exists: an empty expected token never authenticates. The acceptor logs each
refusal at WARN and raises `doctor-plugin-connect-back-refused` on the report
bus, where `show doctor` lists it. The event names the claimed name, or the
remote address when the request was not an auth request and so named none;
an auth request whose claimed name is empty is shown as `""`. It never
carries the token. A
claimed name of the plugin name form is shown unchanged. Any other name is
escaped the way `strconv.Quote` escapes it (control characters, non-printable
and bidirectional runes, invalid bytes, the quote mark and the backslash),
without the outer quote marks, and cut to 64 octets of escaped text with
`...` after the cut. An empty claim is shown as `""`. The plugin whose start waited for that connection fails as
one that never connected.
<!-- source: internal/component/plugin/ipc/tls.go -- AuthenticateWithLookup -->
<!-- source: internal/component/plugin/ipc/connect_back_refusal.go -- AuthRefusal, reportConnectBackRefused -->
<!-- source: internal/component/plugin/ipc/plugin_name.go -- PluginNameForm -->

The Go SDK registers `ze.plugin.name` with the transport environment keys, so
SDK consumers can read the daemon-assigned identity without the plugin CLI.
Compiled fixture observers use that identity when they call `NewFromTLSEnv`;
the SDK constructor authenticates with the name its caller supplies.
The plugin CLI keeps `go-plugin` as its fallback when no name is assigned.
<!-- source: pkg/plugin/sdk/sdk.go -- environment registrations, NewFromTLSEnv, dialAndAuth -->
<!-- source: internal/test/fixture/fixture.go -- newObserver -->
<!-- source: internal/component/plugin/cli/cli.go -- connFromEnv -->

**The trust anchor is the issuer, not one certificate.** `TLSConfigWithRoot`
builds the client config from `ZE_PLUGIN_CA_PEM` alone. It fails closed: an
empty root and an unparsable root each return an error and no config, so no
child holds a config it cannot verify a peer with. A certificate the engine
reissues still validates, because the anchor outlives it. The engine reissues on
its next start, and also while it runs: the acceptor serves through
`plugin.ServingLeaf`, which answers `tls.Config.GetCertificate` and mints a
fresh leaf once two thirds of the current one's life is spent. A plugin that
connects back on day two therefore meets a valid certificate.

Two dialers reach the acceptor, and both go through `TLSConfigWithRoot`:
`sdk.dialAndAuth`, which a standalone plugin binary uses, and `cli.connFromEnv`,
which the framework every in-tree system plugin registers through uses when the
engine starts it out of process. Neither writes its token before the chain
validates.

`ZE_PLUGIN_HUB_HOST` is the acceptor host, except that an unspecified listen
address (`0.0.0.0` or `::`) is handed to the child as `127.0.0.1`. The issued
certificate carries no SAN for an unspecified address, and the plugin runs
beside the engine, so the loopback is both reachable and verifiable.
<!-- source: internal/component/plugin/process/process.go -- startExternal env var setup -->
<!-- source: internal/component/plugin/ipc/tls.go -- TokenForPlugin, TLSConfigWithRoot, StartListeners, PluginAcceptor.RootPEM, combinedLookup -->
<!-- source: internal/component/plugin/cli/cli.go -- connFromEnv -->
<!-- source: internal/component/plugin/leaf.go -- ServingLeaf -->

### Benefits of Long-Lived Design

| Benefit | Description |
|---------|-------------|
| No per-request overhead | Plugin starts once, handles many requests |
| Language agnostic | Same protocol for Go/Python/Rust |
| Hot-swappable | Restart plugin without engine restart |
| Testable | Plugin protocol can be tested independently |
| Internal optimization | In-process plugins bypass transport overhead via DirectBridge |

---

## Family Plugin NLRI System

Family plugins provide NLRI encoding/decoding for address families that require complex parsing
(FlowSpec, EVPN, BGP-LS, VPN). This section details the complete protocol.

### Family Registration (Stage 1)

Plugins declare which families they handle via the `families` field in
`DeclareRegistrationInput`. Each declaration carries both the canonical name
AND the RFC 4760 wire-format AFI/SAFI numbers, so the engine can register the
family in `internal/core/family/` (the cross-component family registry) at
runtime alongside families registered by internal plugins at init.
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput, FamilyDecl -->

```json
{
  "families": [
    {"name": "ipv4/flow",     "mode": "encode", "afi": 1, "safi": 133},
    {"name": "ipv4/flow",     "mode": "decode", "afi": 1, "safi": 133},
    {"name": "ipv6/flow",     "mode": "encode", "afi": 2, "safi": 133},
    {"name": "ipv6/flow",     "mode": "decode", "afi": 2, "safi": 133},
    {"name": "ipv4/flow-vpn", "mode": "both",   "afi": 1, "safi": 134},
    {"name": "ipv6/flow-vpn", "mode": "both",   "afi": 2, "safi": 134}
  ]
}
```

**FamilyDecl fields:**

| Field | Values | Description |
|-------|--------|-------------|
| `name` | `"ipv4/flow"`, `"l2vpn/evpn"`, etc. | Address family (`afi/safi` canonical form) |
| `mode` | `"encode"`, `"decode"`, `"both"` | Direction of conversion |
| `afi` | RFC 4760 AFI number (e.g., `1` = IPv4, `2` = IPv6, `25` = L2VPN, `16388` = BGP-LS) | Required for runtime registration |
| `safi` | RFC 4760 SAFI number (e.g., `1` = unicast, `133` = FlowSpec) | Required for runtime registration |

**Runtime family registration:**
<!-- source: internal/component/plugin/server/startup.go -- registerPluginFamilies -->

After the plugin completes Stage 1, the engine calls `registerPluginFamilies`,
which validates the whole `FamilyDecl` batch and commits it through
`family.RegisterFamilyBatch`. This is the runtime equivalent of internal plugins
calling `family.MustRegister` at init. The server records the families actually
added for that plugin and removes them if a later startup stage fails before the
plugin reaches ready. After committed startup, `Family.String()` and
`family.LookupFamily()` return the plugin's family. Re-registration with
identical values is a no-op; conflicting AFI or SAFI names abort plugin startup.

**Registry conflict detection:**
- Only ONE plugin can register for a family+mode combination
- Conflict results in startup error

**OPEN capability injection (decode mode):**
- Families declared with `decode` are automatically advertised in OPEN
- Engine adds Multiprotocol capability (Code 1) for each decode family
- No explicit `declare-capabilities` needed from plugins for Multiprotocol

### Encode/Decode Protocol

**Engine to Plugin (callback):**

| RPC | Input | Output |
|-----|-------|--------|
| `ze-plugin-callback:encode-nlri` | `{"family":"ipv4/flow","args":["destination","10.0.0.0/24"]}` | `{"hex":"0701180A0000"}` |
| `ze-plugin-callback:decode-nlri` | `{"family":"ipv4/flow","hex":"0701180A0000"}` | `{"json":{"destination-ipv4":...}}` |

**Plugin to Engine (via registry):**

| RPC | Input | Output |
|-----|-------|--------|
| `ze-plugin-engine:encode-nlri` | `{"family":"ipv4/flow","args":["destination","10.0.0.0/24"]}` | `{"hex":"0701180A0000"}` |
| `ze-plugin-engine:decode-nlri` | `{"family":"ipv4/flow","hex":"0701180A0000"}` | `{"json":{"destination-ipv4":...}}` |

### Additional Decode RPCs (Plugin to Engine)

These RPCs allow plugins to request full UPDATE or MP attribute decoding:
<!-- source: pkg/plugin/rpc/types.go -- DecodeMPReachInput, DecodeMPUnreachInput, DecodeUpdateInput -->

| RPC | Input | Output |
|-----|-------|--------|
| `ze-plugin-engine:decode-mp-reach` | `{"hex":"...","add-path":false}` | `{"family":"...","next-hop":"...","nlri":[...]}` |
| `ze-plugin-engine:decode-mp-unreach` | `{"hex":"...","add-path":false}` | `{"family":"...","nlri":[...]}` |
| `ze-plugin-engine:decode-update` | `{"hex":"...","add-path":false}` | `{"json":"..."}` |

`decode-update` answers the "Cannot decode" error below when an NLRI section
does not parse (MP_REACH_NLRI, MP_UNREACH_NLRI, the NLRI field or the Withdrawn
Routes field), naming the section. It never leaves the section out of the JSON,
which would read as an UPDATE that carries no route.
<!-- source: internal/component/bgp/format/codec.go -- FormatDecodeUpdateJSON -->

### Error Handling

| Error Type | Response |
|------------|----------|
| Invalid family | `#<id> error {"code":"error","message":"unknown family: ipv4/unknown"}` |
| Parse error (encode) | `#<id> error {"code":"error","message":"invalid prefix: 10.0.0/24"}` |
| Cannot decode | `#<id> error {"code":"error","message":"..."}` |
| Handler not registered | `#<id> error {"code":"error","message":"encode-nlri not supported"}` |

### Files

| File | Purpose |
|------|---------|
| `internal/component/plugin/registration.go` | Family registry, conflict detection |
| `internal/component/plugin/server/events.go` | NLRI routing |
| `pkg/plugin/rpc/types.go` | RPC input/output types |
| `pkg/plugin/sdk/sdk_engine.go` | SDK encode/decode methods |
| `pkg/plugin/sdk/sdk_dispatch.go` | SDK encode/decode callback handlers |

---

## Command Execution

Plugins register commands in Stage 1 via the `commands` field of `DeclareRegistrationInput`.
At runtime, the engine dispatches commands to plugins via `execute-command`:
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandInput, ExecuteCommandOutput -->

**Engine to Plugin:**
```
#5 ze-plugin-callback:execute-command {"serial":"abc","command":"rib adjacent status","args":[],"peer":"*"}
```

When the operator's line typed pipe operations the command declares (a
`pipes` entry with an `argument`), the request carries them in `pipe`, in the
order typed, each with its decoded argument words; `pipe` is absent when the
line declared none, and an operation typed with no argument has no `args`:

```
#6 ze-plugin-callback:execute-command {"serial":"abd","command":"show x","pipe":[{"name":"top-talkers","args":["5"]},{"name":"histogram"}]}
```

The handler computes with the list and answers data; the engine runs every
other operation of the chain over that answer. A plugin running in the daemon
receives the same list through `DirectBridge.ExecuteCommand`.

**Plugin to Engine:** the answer is a head, its records and a terminator. Every
plugin writes it. The frame is the same whatever the payload is, so a handler
that built one value takes the same three lines as a handler that walked a
table.

An answer line carries no verb and no key name. The field after the id is a
three-byte word saying what the line IS, and every field after that is
positional. The six words:

| Word | The line is | Its fields, in order |
|------|-------------|----------------------|
| `top` | the head. It opens the answer and is always the first line for this id | item type, envelope name, column names |
| `row` | one record the command produced | the payload |
| `bad` | one record the command rejected. The walk goes on | the payload |
| `end` | the terminator. It ends the answer and is always the last line for this id | records produced, rows rejected, message |
| `wrn` | the terminator of a command that completed and states a warning, in place of `end` | records produced, rows rejected, warning lines as a JSON array of strings |
| `nay` | the whole answer to a command text naming no command | error code, message |

The head's item type is a three-byte word too, and it says how the records read:

| Word | The records are |
|------|-----------------|
| `doc` | one document. The whole answer is that one value |
| `map` | one map of names to values for each record |
| `tab` | one positional row for each record, read against the head's column names |

Two field shapes carry every value. A NUMBER is decimal digits closed by a space
or by the end of the line. A TEXT is decimal digits, a colon, then that many
BYTES. **The count is a BYTE count, never a count of characters.** A value
holding multi-byte utf-8 is therefore sliced by the bytes that arrived. A text of
zero bytes is written `0:`, present and empty, so a line's field count never
varies.

A command that RAN and failed states its reason on the TERMINATOR, which is the
one line an answer states an outcome on. It is not an `error` response, because
the RPC itself succeeded even when the command failed. The `error` verb is kept
for a handler that returned a Go error, where the RPC itself did not succeed.

A handler that built one value writes it as the one record of a `doc` answer.
The head states no outcome, and the record carries the value byte for byte:

```
#5 top doc 0: 0:
|  |   |   |  |
|  |   |   |  +----- column names, 0 BYTES, so the records are not positional
|  |   |   +-------- envelope name, 0 BYTES, so the document carries its own
|  |   +------------ item type doc: the whole answer is one document
|  +---------------- kind top: the head, always the first line
+------------------- correlation id 5, the digits a space closes

#5 row 26:{"running":true,"peers":1}
|  |   |  |
|  |   |  +----- those 26 bytes, the handler's value byte for byte
|  |   +-------- 26, the payload's BYTE count, then the colon every text carries
|  +------------ kind row: one record the command produced
+--------------- correlation id 5

#5 end 1 0 0:
|  |   | | |
|  |   | | +----- message, 0 BYTES, so the command stated none
|  |   | +------- 0 rows rejected
|  |   +--------- 1 record produced
|  +------------- kind end: the terminator, always the last line
+---------------- correlation id 5
```

A handler that answered with a `plugin.Records` walk of more than 256 rows
writes one line for each row. A walk over a large table therefore never becomes
one 16 MB line. A shorter walk collapses to the `doc` document above. Only the
head differs from the three lines above, and it differs in two fields:

```
#5 top map 5:peers 0:
|  |   |   | |     |
|  |   |   | |     +----- column names, 0 BYTES, so the records are not positional
|  |   |   | +----------- those 5 bytes: peers, the key the records go under
|  |   |   +------------- 5, the envelope name's BYTE count, then its colon
|  |   +----------------- item type map: each record is one map of names to values
|  +--------------------- kind top: the head
+------------------------ correlation id 5

#5 row 44:{"address":"10.0.0.1","state":"established"}
|  |   |  |
|  |   |  +----- those 44 bytes, one row of the walk byte for byte
|  |   +-------- 44, the payload's BYTE count, then its colon
|  +------------ kind row: one record the walk produced
+--------------- correlation id 5

#5 end 1 0 0:
```

The terminator reads exactly as the one above it: one record produced, no row
rejected, and no message.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- executeCommandAnswer -->
<!-- source: pkg/plugin/records.go -- Records, Records.WriteAnswer -->
<!-- source: internal/component/plugin/ipc/rpc.go -- PluginConn.SendExecuteCommandAnswer, ExecuteCommandValue -->
<!-- source: internal/component/plugin/server/command.go -- routeToProcess, pluginAnswerRows -->

A command that returns a record walk keeps its original produced and refused
counts when an in-process call holds that walk as one document. The existing
execute-command value DTO carries optional `record-answer` metadata beside
`data`: `count`, `faults`, `message`, `warning`, `warnings` and
`document-absent`. The SDK walks
the producer once, keeps every row in the buffered document, and derives
`status` from that exact terminator. The existing collapse validates the
envelope and column schema before it enters that producer. A built-value
producer may omit this metadata. Projecting a completed Answer retains its
terminator even when its document is absent: zero produced and refused counts
then describe no data, while an empty record collection still carries its
real empty JSON document. Absence never becomes a public null document.
The absence bit comes from the actual producer's document presence. A
no-record failure retains its existing raw `data` reason and tail message;
the explicit bit distinguishes that reason from a real collection containing
the same bytes, even when the failure text itself is valid JSON.

The receiving boundary refuses missing or null counts, unknown or repeated
metadata fields, count overflow and a status that contradicts the terminator.
Every supplied `record-answer` field is decoded, including repeated outer
fields; explicit null is refused before it can take the built-value path.
Omitting the optional field preserves ordinary built-value decoding. A
metadata-bearing answer without a document must have zero produced and refused
counts; its message and warning kind and lines survive the reconstruction.
An explicit absence bit additionally refuses any nonzero count or a `data`
reason that differs from its original message. Payload bytes never select
the absence variant.
The built-value route also retains a declared warning when the SDK has not
supplied original-record metadata. It counts the actual document once, or
counts zero when the plugin supplied no data.
It never infers original row counts from the document's keys. This metadata
travels in the existing value DTO; it adds no transport or negotiation.

Buffered SDK dispatch retains `data` and warning lines when an answer is
partial or failed. It returns the actual verdict and a Go error for refused
records or incomplete evaluation. The command's own failure text is preserved.
A failure carried only by refused records gets a bounded explanation; all
individual causes stay in the structured data. An interrupted record reader
retains the progress it already read and cannot report completed evaluation.

The daemon retains the same original outcome in its typed command response.
A buffered document keeps the original produced and refused counts, including
an empty collection or an explicitly absent document. Text surfaces render the
completed and rejected records before they report the command's final failure.
A streamed response opens before completion and publishes its outcome only
after the record walk ends. The daemon's wire writer reads that final message
and warning at the same point, so a late failure or warning reaches the next
socket hop as well as in-process consumers. Warning lines remain beside a
partial answer, whose refused records still determine its verdict.
Transport readers obtain an owned metadata snapshot through
`Response.RecordAnswerMetadata` after consuming the walk; an unfinished walk
has no completed original outcome to supply.

The forwarded writer counts the validated records it actually sends. Its
existing line-width check can replace an unsendable buffered document with one
refused record, which then states zero produced and one refused. A transport
that ends before its terminator still adds the existing fixed public fault;
the underlying transport cause stays in the daemon's diagnostic log.
When a bounded document arrived before that interruption, it remains one
observed document beside the fault. The missing terminator supplied no original
walk counts, so this interrupted response counts the received document once
and the fault once; it never derives original counts from that document's keys.
<!-- source: internal/component/plugin/record_answer.go -- CompleteRecordAnswer, RecordAnswerMetadata -->
<!-- source: internal/component/plugin/dispatch.go -- ResponseJSON, AnswerFor, WriteAnswer -->
<!-- source: internal/component/plugin/server/command.go -- routeToProcessWithEvents, pluginAnswerRows -->
<!-- source: internal/component/plugin/server/command_answer.go -- documentPluginResponse, interruptedDocumentResponse -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteRecordAnswerFinal, WriteCountedDocumentAnswer -->

<!-- source: pkg/plugin/rpc/execute_record_answer.go -- ExecuteCommandRecordAnswer, RecordAnswerMetadata, Failure -->
<!-- source: pkg/plugin/sdk/execute_record_answer.go -- executeCommandRecordOutput -->
<!-- source: pkg/plugin/sdk/sdk_engine.go -- answerValue, dispatchCommandResult -->
<!-- source: internal/component/plugin/ipc/rpc.go -- ExecuteCommandValue, valueAnswer -->

### Inter-Plugin Communication

Plugins can dispatch commands to other plugins via the external-compatible string API:
<!-- source: pkg/plugin/sdk/sdk_engine.go -- DispatchCommand -->

```
#4 ze-plugin-engine:dispatch-command {"command":"rib adjacent inbound show"}
```

The `command` line may carry a pipe chain, written as an operator types it:
`show bgp rib | received | count` answers the count of the received routes.
The dispatcher reads the line once, command part and chain. The operations the
command declares reach its handler as a list, and the rest of the chain runs
over the answer the way it does on every other surface. A format such as
`| text` still answers `done`, its data the rendered text. A pipe inside double
quotes stays an argument value.
<!-- source: internal/component/plugin/server/command.go -- Dispatcher.Dispatch -->
<!-- source: internal/component/command/pipe_partition.go -- partitionPipeChain -->
<!-- source: internal/component/plugin/server/dispatch.go -- dispatchCommandResponse -->

Internal plugins that already know the exact registered command should use the
typed args API. It carries the command name, pre-tokenized arguments, and optional
peer selector separately, so runtime values are not split by the command tokenizer.
<!-- source: pkg/plugin/rpc/types.go -- DispatchCommandArgsInput -->
<!-- source: internal/component/plugin/server/dispatch.go -- dispatchCommandArgs -->

```
#4 ze-plugin-engine:dispatch-command-args {"command":"request bgp adj-rib-in replay","args":["peer key with spaces","0"],"peer":"*"}
```

Both APIs answer with a head, records, and a terminator. See
[ipc_protocol.md](ipc_protocol.md), "Answer Protocol".
<!-- source: internal/component/plugin/server/dispatch_registry.go -- recordAnswer, opDispatchCommand, opDispatchCommandArgs -->
<!-- source: internal/component/plugin/dispatch.go -- WriteAnswer -->

A caller that reads that answer as one value gets `DispatchCommandOutput`. Its
`data` field carries raw JSON (single-decode). A completed record answer keeps
its document, original `record-answer` metadata and warning lines beside a
partial or failed verdict. The server consumes a streamed document once before
it copies that final outcome. An ordinary failed built value keeps its
error-only shape: `{"status":"error","error":"message"}`. Warning lines are
the text the operator's own surface prints beside the same answer. On the
socket they ride in the `wrn` terminator (ipc_protocol.md, "Answer Protocol").
The in-process `DirectBridge` carries the value directly.
<!-- source: internal/component/plugin/server/dispatch.go -- responseToDispatchOutput -->
<!-- source: pkg/plugin/rpc/types.go -- DispatchCommandOutput -->

The Go SDK keeps `DispatchCommand` and `DispatchCommandArgs` answering
`(status, data, err)`, so a plugin that reads only the status loses nothing it
read before. `DispatchCommandResult` and `DispatchCommandArgsResult` answer a
`DispatchResult` holding the status, data, warning lines and optional original
`RecordAnswer` metadata. The SDK validates supplied metadata, document presence,
error text and warning lines before relying on that outcome. Both socket and
bridge readers return a Go error for refused records or incomplete evaluation
while retaining completed data. A plugin reading the answer row by row through
`DispatchCommandAnswer` reads `Answer.Warnings` after the range.
<!-- source: internal/component/plugin/server/dispatch.go -- responseToDispatchOutput -->
<!-- source: pkg/plugin/sdk/sdk_engine.go -- DispatchResult, DispatchCommandResult, DispatchCommandArgsResult -->
<!-- source: pkg/plugin/rpc/types.go -- DispatchCommandOutput -->
<!-- source: pkg/plugin/rpc/dispatch_record_answer.go -- UnmarshalJSON -->
<!-- source: pkg/plugin/sdk/sdk_engine.go -- Plugin.dispatchCommandValue, answerValue -->
<!-- source: internal/component/plugin/server/dispatch_registry.go -- serveEngineOpDirect -->

The string API routes by normal dispatcher parsing and remains the compatibility
surface for CLI and external plugin callers. The typed args API routes by exact
registered plugin command and sends the existing `execute-command` callback to
the target plugin with `args []string`.
<!-- source: internal/component/plugin/server/command.go -- ForwardToPlugin -->
---

## Config Reload Protocol

Config reload uses a two-phase verify/apply pattern:
<!-- source: pkg/plugin/rpc/types.go -- ConfigVerifyInput, ConfigApplyInput -->

**Phase 1: Verify** -- engine sends candidate config to all plugins for validation:

```
#10 ze-plugin-callback:config-verify {"sections":[{"root":"bgp","data":"{...}"}]}
#10 ok {"status":"ok"}
```

If any plugin rejects, the reload is aborted.

A plugin built on the Go SDK that wants config and registered neither an apply
handler (`OnConfigApply`) nor `DeclareAppliesNothing` rejects here, whatever
its verify handler would say, because it would accept the change and keep
running the old config. The reload is refused and the text names the plugin,
its roots and the restart that applies the change at startup:

```
#10 ok {"status":"error","error":"plugin <name> cannot apply a change to bgp while ze runs: it registered no config apply handler and does not declare that it applies nothing, so it would accept the change and keep running the old config\n..."}
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- configVerifyCallback, refuseWithoutApply -->

**Phase 2: Apply** -- engine sends config diffs to all plugins:

```
#11 ze-plugin-callback:config-apply {"sections":[{"root":"bgp","added":"{...}","removed":"","changed":""}]}
#11 ok {"status":"ok"}
```

**ConfigDiffSection fields:**
<!-- source: pkg/plugin/rpc/types.go -- ConfigDiffSection -->

| Field | Type | Description |
|-------|------|-------------|
| `root` | `string` | Config root name |
| `added` | `string` | JSON-encoded added config |
| `removed` | `string` | JSON-encoded removed config |
| `changed` | `string` | JSON-encoded changed config |

**Writer judgement and FIB process transitions on reload:**

Every reload carries the candidate plugin selection judged by `LoadConfig`
(`LoadConfigResult.CandidateSelection`) beside the candidate tree into
`Server.ReloadConfigWithSelection`. Even when a diff only removes config
sections, `reloadConfig` runs `judgeReloadWriters` over the whole candidate:

- Adding a chosen `ProgramsFIB` plugin (via a `fib` config root or a
  compiled-in `bus > endpoint`) starts it before apply and stops any displaced
  engine-added writer or removed/changed `ProgramsFIB` bus endpoint during
  post-apply removal. Same-name endpoint replacement (changing `internal` or
  `process` on the same endpoint key) defers the replacement start until
  `stopCollectedProcesses` frees the process slot.
- Removing the last chosen `ProgramsFIB` plugin while a `NeedsDataPlane`
  producer remains starts the configured data plane's writer (via
  `registry.JudgeDataPlaneWriter`) before apply and stops the removed FIB
  plugin afterwards.
- Removing the last `NeedsDataPlane` producer with no chosen `ProgramsFIB`
  plugin stops both the producer and the engine-added writer; `stopOrderGraph`
  orders every stopping producer ahead of every stopping `ProgramsFIB` writer
  so routes are withdrawn while the writer is still live.
- Whenever `stopCollectedProcesses` finishes stopping one or more `ProgramsFIB`
  processes without error, it emits `(mpls-fib, writer-stopped)`
  (`mplsfib.WriterStopped`) once so surviving data-plane writers trigger a
  targeted label replay after the displaced writer has unsubscribed.

<!-- source: internal/component/plugin/server/reload.go -- ReloadConfigWithSelection, reloadConfig -->
<!-- source: internal/component/plugin/server/reload_dataplane_writer.go -- judgeReloadWriters, autoLoadForReload, stopOrderGraph -->
<!-- source: internal/component/plugin/server/startup_autoload.go -- stopCollectedProcesses -->
<!-- source: internal/core/mplsfib/events.go -- WriterStopped -->

---

## OPEN Validation

Plugins can validate incoming OPEN messages by registering `OnValidateOpen`.
The engine sends both local and remote OPENs for inspection:
<!-- source: pkg/plugin/rpc/types.go -- ValidateOpenInput, ValidateOpenOutput -->

`provisional` is true for an incoming connection that has not been selected.
The plugin checks its OPEN policy and returns the normal accept or rejection
answer, but must leave peer-scoped learned state unchanged. A losing connection
never publishes that state. After joining the previous connection's cleanup,
selection repeats the exact retained OPEN pair with `provisional: false`, once,
before the selected connection can announce peer-up or deliver routes. Ordinary
selected connections use false on their initial validation. The callback's
failure, elapsed original HoldTimer budget, or closed socket prevents adoption.
This field has the same meaning over the internal and JSON RPC transports.
<!-- source: internal/component/bgp/reactor/session_open_admission.go -- selectNegotiationEpoch -->
<!-- source: internal/component/bgp/reactor/session_open_validation.go -- runOpenValidator -->
<!-- source: internal/component/bgp/server/validate.go -- broadcastStampedValidateOpen -->

```
#7 ze-plugin-callback:validate-open {"peer":"192.168.1.1","local":{"asn":65001,"router-id":"1.1.1.1","hold-time":90,"capabilities":[...]},"remote":{"asn":65002,...}}
#7 ok {"accept":true}
```

`peer` is the peer's configured name. `group` is its enclosing group, and the
field is omitted for a peer that stands alone:

```
#8 ze-plugin-callback:validate-open {"peer":"dyn-192.0.2.7","group":"ix","local":{...},"remote":{...}}
```

A peer created from a dynamic group's template has only that second identity. The
engine builds such a peer when a connection arrives inside the group's range, and
names it `dyn-<addr>`. Neither that name nor its address is in the config the
plugin read. A plugin that keys per-peer policy on the config resolves that peer
through `group` or resolves nothing for it.

`generation` orders the OPEN against the peer's state events, which a plugin
receives on another loop. It is the newest forward generation the engine had
issued when it asked, from the same clock as a down event's
`session-generation`. A down above it was issued after the OPEN was validated.
The engine issues a removal down's generation as it deletes the peer, so the
removal of an earlier peer of the same name is at or below the OPEN's
`generation`, and the removal of this peer is above it. `bgp-role` keys its
learned role on it (`docs/architecture/meta/role.md`, "Learned role
lifecycle").
<!-- source: internal/component/bgp/reactor/peer.go -- validateOpen -->

`wants-validate-open` makes the process an OPEN observer for every peer, not
only peers whose local OPEN carries one of its capabilities. Peer-removal
delivery (`down` with reason `removed`) to OPEN validators is a registered
reception (`bgp:state`) governed by the compiled bus filter (`DeliveryGraph`),
shown on `show event delivery` with origin `registered` and excluded if the
operator's `receive > allow` omits it.
<!-- source: internal/component/bgp/server/validate.go -- askOpenValidators -->
<!-- source: internal/component/bgp/server/events.go -- onPeerStateChange -->

To reject:
```
#7 ok {"accept":false,"notify-code":2,"notify-subcode":6,"reason":"unacceptable hold time"}
```

---

## Plugin Transport

### Internal Plugins (goroutine)

Transport: `net.Pipe()` for 5-stage startup, then `DirectBridge` for hot path.
No network, no TLS, no auth. Fastest path.

### External Plugins (TLS connect-back)

Transport: single TLS connection per plugin.
<!-- source: pkg/plugin/sdk/sdk.go -- NewFromTLSEnv -->

The child is started as `/bin/sh -c <run>`, because the `run` string is written
for a shell and the engine appends no argv to it. The shell is therefore a
runtime dependency of every external plugin. On a host that carries none, such
as a gokrazy appliance image, the start names the absent shell rather than the
plugin, and `ze doctor` reports the same fact before the daemon runs, under
`doctor-plugin-shell-missing`.
The child is started in a process group of its own, and the daemon's stop
signals that whole group. On Linux it does so only after the plugin has had
its chance to exit. The stop closes the plugin's connection first, which ends
the SDK's `Run`, and the plugin's own exit then writes out the stderr it already
wrote: every ze process points `os.Stderr` at a pipe a goroutine copies on
(`crashlog.Init`), and only an orderly exit drains it, so a kill on the stop
lost the plugin's last line. A plugin still running `pluginExitGrace` (1
second) after its connection closed is SIGKILLed with its group; one that
exited on its own is reaped only after its group is signaled, so its
descendants get the same stop. A plugin that never connected back is killed at
once. `ProcessManager.Stop` cancels the context every plugin process context
derives from, and that cancellation is a stop too: it starts the same grace,
never an immediate kill. The fork carries no context of its own there, and the
daemon records the plugin's exit before it reaps it: a kill is sent only while
the plugin is unreaped, because the reap hands its pid, and the group number,
to the next process the kernel starts.

On other platforms the stop kills the group at once, through the fork's
context, and a plugin's last stderr line can be lost to it. No call there waits
for a child's exit while leaving it unreaped (Linux's `waitid` with
`WNOWAIT`), so a kill sent a grace later could reach a stranger that took the
freed pid, and a stop that waits would have nothing safe to finish with.
The shell is the daemon's direct child, and a shell
that does not exec-optimize the `run` string stays alive with the plugin as its
own child, so a stop aimed at the direct child leaves the plugin running with
nothing to talk to. The declaration query stops its child by the group too, the
moment its context ends, through `KillGroupOnCancel`. On Linux the
fork also carries `Pdeathsig`, which the kernel delivers to that direct child
when ze dies. It reaches the plugin for a `run` string the shell
exec-optimizes, and a `run` string the shell keeps a process for leaves the
plugin a grandchild, which no death signal reaches.

The kernel sends that death signal when the THREAD that forked the child
exits, not when ze does. The Go runtime destroys a thread whose goroutine
returns while it holds `runtime.LockOSThread`, and ze does that on purpose:
a kernel capability probe moves a locked thread into a throwaway network
namespace and lets the runtime destroy it, and the start and reload gates run
such probes. A plugin forked from an ordinary goroutine could therefore be
killed, with no log line of its own, by the next probe that ran on its thread.
Every fork that carries these attributes, the live start and the declaration
query alike, goes through `StartChild`, which forks from one spawner thread
that stays locked for the life of ze and so is never destroyed.
<!-- source: internal/component/plugin/sysproc_linux.go -- StartChild, runSpawner -->
<!-- source: internal/component/kernelcap/namespace_linux.go -- InThrowawayNetworkNamespace -->
<!-- source: internal/component/plugin/shell.go -- Shell, ShellAvailable -->
<!-- source: internal/component/plugin/process/process.go -- startFailure -->
<!-- source: internal/component/plugin/sysproc.go -- KillGroupOnCancel, KillProcessGroup -->
<!-- source: internal/component/plugin/process/process.go -- pluginExitGrace, superviseExit, pluginGroup, monitorCmd -->
<!-- source: internal/component/plugin/process/exitwait_linux.go -- stopGivesExitGrace, newPluginShell, waitExitedUnreaped -->
<!-- source: internal/component/plugin/process/exitwait_other.go -- stopGivesExitGrace, newPluginShell -->
<!-- source: internal/core/crashlog/stderr.go -- redirectStderr -->
<!-- source: internal/component/plugin/sysproc_linux.go -- NewSysProcAttr -->

1. Engine reads `bus { hub { server <name> { ip ...; port ...; secret ...; } } }` from config
2. Engine starts TLS listener(s) (one per `server` entry), creates `PluginAcceptor` holding the certificate authority root that issued the served certificate
3. Engine generates per-plugin token, registers the waiter for the plugin's name (`ExpectPlugin`) so a child that authenticates before the engine waits is held rather than dropped, then forks child with `ZE_PLUGIN_HUB_HOST`, `ZE_PLUGIN_HUB_PORT`, `ZE_PLUGIN_HUB_TOKEN` (unique per plugin), `ZE_PLUGIN_CA_PEM`, `ZE_PLUGIN_NAME` env vars
4. Child validates the engine chain against that root, connects via TLS, sends `#0 auth {"token":"...","name":"..."}`
5. Engine authenticates: per-plugin token lookup by name (constant-time comparison), name binding enforced
6. Token cleared from child OS environment after first read
7. Single `MuxConn` handles bidirectional RPC (responses by `#<id>`, requests via `Requests()` channel)
8. Standard 5-stage handshake proceeds over the same connection
<!-- source: internal/component/plugin/ipc/tls.go -- combinedLookup, AuthenticateWithLookup, ExpectPlugin -->

### Config

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `bus.hub.server` | named list | -- | TLS listener entries (keyed by name) |
| `bus.hub.server.<name>.ip` | string | `127.0.0.1` | Bind address |
| `bus.hub.server.<name>.port` | uint16 | `12700` | Bind port |
| `bus.hub.server.<name>.secret` | string | (required, min 32 chars) | Auth token |

### Files

| File | Purpose |
|------|---------|
| `internal/component/plugin/ipc/tls.go` | TLS listener, auth, cert gen, PluginAcceptor |
| `internal/component/plugin/ipc/rpc.go` | PluginConn with MuxConn |
| `internal/component/plugin/process/process.go` | startInternal, startExternal, InitConns |
| `pkg/plugin/sdk/sdk.go` | NewFromTLSEnv, NewWithConn |

---

## Plugin Examples

### Capability-Only Plugin (e.g., GR)

The GR plugin only participates in startup -- it injects GR capabilities into OPEN messages.
No event subscription needed because it doesn't need runtime events.

```
Ze Engine                              GR Plugin
----------                             ---------

STAGE 1: REGISTRATION
                       <--- #1 ze-plugin-engine:declare-registration
                            {"protocol-version":1,"wants-config":["bgp"]}
#1 ok {"protocol-version":1}  --->

STAGE 2: CONFIG DELIVERY
#1 ze-plugin-callback:configure
  {"sections":[{"root":"bgp","data":"{...}"}]}  --->
                       <--- #1 ok

STAGE 3: CAPABILITY DECLARATION
                       <--- #2 ze-plugin-engine:declare-capabilities
                            {"capabilities":[
                              {"code":64,"encoding":"hex","payload":"0078",
                               "peers":["192.168.1.1"]},
                              {"code":64,"encoding":"hex","payload":"005a",
                               "peers":["10.0.0.1"]}
                            ]}
#2 ok                  --->

STAGE 4: REGISTRY SHARING
#2 ze-plugin-callback:share-registry {"commands":[...]}  --->
                       <--- #2 ok

STAGE 5: READY
                       <--- #3 ze-plugin-engine:ready
#3 ok                  --->

=== BGP PEERS START - GR capability included in OPEN ===

RUNTIME: (waits for bye)
#99 ze-plugin-callback:bye {"reason":"shutdown"}  --->
                       <--- #99 ok
(plugin exits)
```

**Capability hex format:** Code 64 = Graceful Restart (RFC 4724).
`0078` = restart-time 120 (0x78 = 120). `005a` = restart-time 90.

### Event-Driven Plugin (e.g., RIB)

The RIB plugin tracks routes and replays them on peer reconnect.
Requires event subscription for runtime events.

```
Ze Engine                              RIB Plugin
----------                             ----------

STAGE 1: REGISTRATION
                       <--- #1 ze-plugin-engine:declare-registration
                            {"protocol-version":1,"commands":[
                              {"name":"rib adjacent status"},
                              {"name":"rib adjacent inbound show"},
                              {"name":"rib adjacent outbound resend"}
                            ]}
#1 ok {"protocol-version":1}  --->

STAGE 2: CONFIG DELIVERY
#1 ze-plugin-callback:configure {"sections":[]}  --->
                       <--- #1 ok

STAGE 3: CAPABILITY DECLARATION
                       <--- #2 ze-plugin-engine:declare-capabilities
                            {"capabilities":[]}
#2 ok                  --->

STAGE 4: REGISTRY SHARING
#2 ze-plugin-callback:share-registry {"commands":[...]}  --->
                       <--- #2 ok

STAGE 5: READY (with startup subscription)
                       <--- #3 ze-plugin-engine:ready
                            {"subscribe":{"events":["update","state","sent"],
                                          "peers":["*"],"format":"json"}}
#3 ok                  --->

=== BGP PEERS START ===

RUNTIME: Peer comes up
#42 ze-plugin-callback:deliver-batch
  {"events":["{\"type\":\"state\",\"peer\":\"192.168.1.1\",\"state\":\"up\"}"]}  --->
                       <--- #42 ok

RUNTIME: Route sent to peer
#43 ze-plugin-callback:deliver-batch
  {"events":["{\"type\":\"sent\",\"peer\":\"192.168.1.1\",...}"]}  --->
                       <--- #43 ok

RUNTIME: Command request
#44 ze-plugin-callback:execute-command
  {"serial":"abc","command":"rib adjacent status","args":[],"peer":"*"}  --->
                       <--- #44 top doc 0: 0:        head: doc, no envelope, no columns
                       <--- #44 row 26:{"running":true,"peers":1}   26 BYTES of payload
                       <--- #44 end 1 0 0:           1 produced, 0 rejected, no message

RUNTIME: Plugin sends route update to engine
                       <--- #4 ze-plugin-engine:update-route
                            {"peer-selector":"192.168.1.1",
                             "command":"update text next-hop 10.0.0.1 nlri add 10.0.1.0/24"}
#4 ok {"peers-affected":1,"routes-sent":1}  --->

SHUTDOWN
#99 ze-plugin-callback:bye {"reason":"shutdown"}  --->
                       <--- #99 ok
(plugin exits)
```

---

## Capability Decode API

Plugins can provide capability decoding for `ze bgp decode --plugin <name>`.

This is a **standalone mode** separate from the 5-stage startup protocol.

### Usage

```bash
# Decode OPEN message with plugin-provided capability decoding
ze bgp decode --plugin bgp-hostname --open FFFF...
```

Without plugin, unknown capabilities show raw hex:
```json
{"code": 73, "name": "unknown", "raw": "0C6D792D686F73742D6E616D65..."}
```

With plugin, capabilities are decoded:
```json
{"name": "fqdn", "hostname": "my-host-name", "domain": "my-domain-name.com"}
```

### Protocol

Plugin is spawned with `--decode` flag and communicates via stdin/stdout.

#### Request Formats

| Request | Description |
|---------|-------------|
| `decode capability <code> <hex>` | JSON output (default) |
| `decode json capability <code> <hex>` | JSON output (explicit) |
| `decode text capability <code> <hex>` | Human-readable text output |
| `decode nlri <family> <hex>` | JSON output (default) |
| `decode json nlri <family> <hex>` | JSON output (explicit) |
| `decode text nlri <family> <hex>` | Human-readable text output |

#### Response Formats

| Response | Description |
|----------|-------------|
| `decoded json <json>` | JSON-formatted result |
| `decoded text <text>` | Human-readable single-line text |
| `decoded unknown` | Plugin cannot decode this input |

#### Examples

**Capability decode (JSON):**

| Direction | Message |
|-----------|---------|
| ze to plugin | `decode json capability 73 0C6D792D686F7374...` |
| plugin to ze | `decoded json {"name":"fqdn","hostname":"my-host","domain":"dom.com"}` |

**Capability decode (text):**

| Direction | Message |
|-----------|---------|
| ze to plugin | `decode text capability 73 0C6D792D686F7374...` |
| plugin to ze | `decoded text fqdn                 my-host.dom.com` |

**NLRI decode (text):**

| Direction | Message |
|-----------|---------|
| ze to plugin | `decode text nlri ipv4/flow 0501180a0000` |
| plugin to ze | `decoded text destination 10.0.0.0/24` |

If plugin cannot decode:

| Direction | Message |
|-----------|---------|
| plugin to ze | `decoded unknown` |

### Plugin Implementation

Plugin entry point with `--decode` flag:

```bash
ze plugin bgp-hostname --decode
```

Plugin reads decode requests from stdin, writes responses to stdout, exits on EOF.

### Files

| File | Purpose |
|------|---------|
| `internal/component/bgp/cli/decode_plugin.go` | Invokes plugin decode API |
| `internal/component/bgp/cli/cmd_plugin.go` | `ze plugin <name> --decode` entrypoint |
| `internal/component/bgp/plugins/hostname/hostname.go` | `RunDecodeMode()` - hostname capability |
| `internal/component/bgp/plugins/nlri/flowspec/plugin.go` | `RunFlowSpecDecode()` - FlowSpec NLRI |

---

**Last Updated:** 2026-03-22


### Accepted redistribution at BGP input

The BGP redistribution consumer registers its independent
`origin-admission-lookup` callback before SDK startup. Factory construction
therefore precedes `Run`; consumer publication and source replay still wait
for `Started`. A full `update-route` request carries `origin-admission-id` as
its own scalar field. Replay markers refuse a nonzero accepted-origin ID,
so a mixed request cannot discard route provenance. Zero follows ordinary local/API behavior. A nonzero ID
must resolve on the authenticated accepting Process connection captured for
that request. Route metadata and the process name cannot declare source kind.

The text update handler carries the accepted-origin ID into announcement and
withdrawal batches. When a native caller supplies the ID only in the request
context, each reactor entry copies it into the batch before origin admission.

The consumer owns one immutable accepted record per original protocol,
source instance, table, native family/prefix and admitted peer scope. A new
version gets a new opaque ID. The snapshot preserves the original preference
metric separately from transformed wire MED, an explicit zero distance
override, and the evaluator generation, including initial generation zero.
An unchanged rule reload advances the evaluator without resending unchanged
routes. A later targeted replay with identical source/build facts and a generation
at least as new reuses the original all-peer admission ID and immutable snapshot,
including its original generation. Different source/build facts and an older
generation refuse this reuse; a full accepted replacement creates a new ID.
Lookup copies caller-owned communities and performs no election or I/O while
holding its leaf lock. The independent worker can answer while the original
route request waits for its deciding plugin and final output.

The reactor validates the native key and original attribute/next-hop choices
against that record before classifying it as redistributed. It checks the
same snapshot again before intake and final writing, outside election and
issuer locks, then rechecks the existing write/grant lease. Peer grants remain
separate authority. An unknown ID, changed record, wrong native key or a
same-name replacement connection cannot authorize output.

A direct destination (one whose owner does not take origin intake) writes an
accepted original on the original's own rail, and its withdrawal takes the same
rail. While the destination's queueing gate is closed (initial sync), the
original waits on the peer's operation queue with its accepted ID, and the
withdrawal joins that queue behind it, never the forward queue. Under the peer
lock it already holds, the withdrawal marks each queued announcement of the
same ID and route as retired, and both drains drop a marked announcement
without writing it. The drain makes no lookup and no RPC. While the gate is
open the original was written directly, so its withdrawal is written directly
too, even when the replay fence or owed forwards hold other withdrawals behind
the forward queue. Either way, a replacement announced after the withdrawal is
never withdrawn by it. An original a drain has already taken off the queue for
its write cannot be marked; its withdrawal is then written after it, from the
same queue. An original whose record vanished without a withdrawal, because its
accepting Process ended, is not marked and is written: on a direct destination
it stays, as the Process's routes already written there stay.
<!-- source: internal/component/bgp/reactor/peer.go -- QueueWithdraw, retireQueuedAcceptedLocked -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- sendInitialRoutes, drainAndCloseQueueGate -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- destinationBatchQueues, withdrawBatchFromPeers, splitOnAdvertised -->

Exact source withdrawal marks only that source's records as retiring. Such a
record may answer its exact withdrawal while that operation joins; it cannot
authorize an announcement. Completed records leave no withdrawal history.
The adapter mutation gate orders record replacement with the actual route
results. A failed replacement preserves its original predecessor IDs as
withdraw-only records, including scopes no longer in the current group. RPC
errors, missing results, failed or partial answers, and zero counts without
retained admission do not confirm replacement. The new immutable record still
describes the current accepted input; it does not claim successful engine
output. An identical retry reuses that original ID and snapshot. A successful
retry does not erase an earlier unresolved predecessor: exact source cleanup
visits every surviving ID, removes only confirmed withdrawals, and retries
failed withdrawals even after the current group has gone. Independent source
instances, tables and peer scopes remain separate. These live cleanup
obligations share the existing accepted-record bound and leave no completed
history. An admitted loser retains the existing full-result receipt and is
not reported as a zero-peer failure. When the accepting Process ends, the
engine retires its redistributed contributions through the live deciding
connection. Ordinary local/API keep-and-reoffer behavior is unchanged.

The consumer's withdrawal answers the joined failures of its `update-route`
results, so the orchestrator learns whether the withdrawal was confirmed. The
orchestrator sends each withdrawal to the consumer instance that accepted the
contribution, never to a replacement registered later under the same name, and
releases its held contribution only after that instance confirms. An
unconfirmed withdrawal keeps that instance and, inside the consumer, its
original withdraw-only admission IDs; the next removal of the contribution or
the next reload of the rules withdraws it there again, with no timer. Each
failure is reported as `doctor-redistribute-withdrawal-unconfirmed` with the
original source instance and table. These held records share the held set's
bound: one per accepted contribution not yet confirmed withdrawn.
<!-- source: internal/component/bgp/redistribute/consumer.go -- BGPConsumer.WithdrawRoute -->
<!-- source: internal/plugins/redistribute/held.go -- retireEpisode, claimRemove, settleWithdrawal -->
<!-- source: internal/plugins/redistribute/withdrawal_diagnostic.go -- reportUnconfirmedWithdrawal -->

This boundary consumes already-accepted redistribution data. Upstream kernel,
IPsec, OSPF and IS-IS producer lifetime and replay mechanisms remain owned by
redistribution. The shared removal value includes exact source identity;
native OSPF/IS-IS consumers continue applying their existing prefix removal.

Validation-triggered `request bgp adj-rib-in replay-path` takes destination,
source address, family, prefix, path ID, original peer owner and original
connection ID. The last two come from the original `ValidationRoute`; the
receive store never substitutes a sibling's path when that original is absent.
<!-- source: internal/core/bgp/ribevents/replay.go -- RelayReplayPath -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_replay_path.go -- replayPathCommand -->

A stored replay route carries `source-owner`, `source-receipt` and
`source-connection-id` as lossless
uint64 decimal strings. They are the original received facts retained with the
current route, not a source selected by its address at replay. The engine must
verify its original issuing record before treating them as learned-source
authority. Storing or echoing them acquires no lease or permission. Explicit
manual or observation values can carry zero; zero never requests that the
engine supply a current learned owner or receipt.
<!-- source: pkg/plugin/rpc/types.go -- StoredRoute -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- replayStoredRoute -->

An absent exact path still returns a withdrawal carrying its original owner and
source connection, even when its receipt was released. The engine can then
restrict it to acknowledged output of that same owner and physical connection.
It must not retire a newer connection or classify the withdrawal as a fresh
manual announcement. On an announcement the physical echo is checked against
the original receipt; the echo grants no authority by itself. Source connection
is independent of the request's captured destination tuple.


### Original physical peer flow control

A plugin dispatching pause or resume can bind the existing `dispatch-command`
request to its original `peer-owner` and immutable `connection-id`. Both fields
are required together, with a positive bounded `deadline-ms`. The command's
registration must explicitly accept this constraint; every other command is
refused before its handler runs. The native value and framed paths use the same
dispatch evaluator and authenticated process. Bound requests use acknowledged
request cancellation and return the exact owner, connection and deadline after
the handler completes. An absent or different receipt is refused by the SDK.

The peer controller finds the original owner, then checks the original physical
connection and request context while holding the peer's session lock. It pauses
or resumes that captured session under the same lock. An old request therefore
cannot look up a replacement session by address. Unbound operator pause and
resume retain their existing address/name semantics.
<!-- source: pkg/plugin/rpc/dispatch_peer.go -- ValidatePeer, ValidatePeerAnswer -->
<!-- source: pkg/plugin/sdk/sdk_dispatch_peer.go -- DispatchPeerCommand -->
<!-- source: internal/component/plugin/server/dispatch_peer.go -- dispatchPeerCommand -->
<!-- source: internal/component/bgp/reactor/reactor_connection.go -- peerReadControl -->


### Captured cached forwarding targets

The existing `forward-cached` input carries either unbound `destinations` or
bound `targets`, never both. Each target contains `destination`,
`destination-owner` and `destination-session`; both identifiers are lossless
uint64 strings in JSON. The address is a constraint, never identity. The
reactor captures this same tuple when producing `fast-path-skipped`. Typed
delivery, enveloped TEXT and full JSON preserve the whole tuple. A reader
clones the list before asynchronous use and never finds an owner by address.
Missing identifiers, duplicate owners, invalid destinations and empty bound
sets are refused, including when the ID list is empty.
<!-- source: pkg/plugin/rpc/forward_targets.go -- ForwardTarget, ForwardCachedInput.Validate -->

A bound cache call dispatches its complete target set once. Every target uses
the existing egress pipeline and its captured final writer. The evaluator
attempts every target on the existing bounded per-peer worker queues before
joining any writer; a held target cannot prevent a ready sibling from writing.
A runnable carrier is refused when the channel is full and no ordered overflow
is pending. Held work or work behind an existing overflow item can use the
same worker's ordered overflow. The worker admits at most one channel's worth
of additional live carriers there and one separate initial replay carrier.
When a hold moves admitted work from the channel, overflow takes its debt
before the channel releases it. Initial replay can pass live carriers parked
behind the replay fence. Cancellation wakes the same worker, and canceled or
retired work completes without writing to a successor's session.
A carrier parked behind a destination's initial-sync gate can belong to a
plugin's peer-up handler, which the engine's Established callback waits for.
The destination's initial sync is then started without waiting for that
handler, so the gate opens and the carrier joins: Ze's own table, the parked
replay, then the End-of-RIB
(`docs/architecture/plugin/plugin-system.md`, "Peer-up barrier").
The existing cache
consumer is acknowledged once for each unique ID, after all target writes
have joined, also on partial refusal or cancellation. A target failure does not
turn another target's useful output into absence. No per-target call can
acknowledge shared cached IDs early. A final completion receipt confirms the
whole call settled; it does not grant send or retained-source authority.
<!-- source: internal/component/bgp/reactor/relay_bound.go -- ForwardUpdatesBound, boundForwardStage.flush -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- tryDispatchBound, reserveBoundOverflow, requeueForwardHold, takeOverflowReleased -->
<!-- source: internal/component/bgp/reactor/peer_up_delivery.go -- noteInitialGateWaiter, awaitInitialSyncStart -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward_batch.go -- forwardUpdates -->


### Original destination readiness

The existing `dispatch-command` operation accepts `destination-owner` and
`destination-session` only for commands whose registration opts in. These are
the configured Peer and original UP token, independent of physical pause/resume
`peer-owner` and `connection-id`. Mixing the two constraints is refused.
`DispatchDestinationCommand` adds the bounded deadline, joins cancellation and
checks the exact owner/session/deadline receipt on either transport.
<!-- source: pkg/plugin/rpc/dispatch_destination.go -- ValidateDestination, ValidateDestinationAnswer -->
<!-- source: pkg/plugin/sdk/sdk_dispatch_destination.go -- DispatchDestinationCommand -->

Readiness still checks the reporting process and its initial-replay token, with
no send-update grant. The reactor resolves the original owner, then under the
peer's session lock checks both original destination and replay tokens and the
request context before crediting the reporter. A state observer may report as
before; a reporter outside the expected set changes no fence. Explicit unbound
operator commands keep their existing behavior.
<!-- source: internal/component/bgp/plugins/cmd/peer/session.go -- handlePeerSessionReady -->
<!-- source: internal/component/bgp/reactor/peer.go -- creditAPIReadyContext -->


### Published source generation

An ordinary or targeted attached UP publishes the forward generation assigned
when that exact Session entered Established, beside its independent physical
connection identity. A delayed targeted UP retains the captured Session's
generation after replacement. DOWN names that same generation. A source-policy
transaction matches the original peer owner and published generation; callers
may map this pair to the immutable connection they observed in the same UP.
Neither number is converted into the other, and no current-peer lookup can
replace this published pair.
<!-- source: internal/component/bgp/reactor/peer.go -- setState -->
<!-- source: internal/component/bgp/reactor/peer_bound_info.go -- boundPeerInfo -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- establishedPeerInfo -->


Live bound replay rechecks the authenticated reporter's received-UPDATE grant
on the original Peer before enrolling its existing source-receipt write. It
holds that Peer's settings read lock until enrollment, then releases the lock
before any output. A config cut either follows an enrolled write, which the
existing grant cleanup must cancel and join, or precedes admission and refuses
it. The receipt table is retaken under its lock after obtaining the Peer lock;
no stale record pointer can resurrect a released delivery. Retained replay
continues to use its previously acquired, actual process-owned lease.
<!-- source: internal/component/bgp/reactor/source_receipt_live.go -- beginLive, liveSourceReceiveGranted -->


The normal `request peer <peer> refresh <family>` registration also accepts
the destination constraint. Its captured variant uses the same capability and
refresh-send grant checks, then the shared request-bound socket writer. It
checks the original tuple under that final write lock and joins the flush; it
never queues refresh for a successor session. The explicit unbound form keeps
its existing operator behavior.
<!-- source: internal/component/bgp/plugins/route_refresh/handler/refresh.go -- handleRefresh -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- SendRefreshBound, sendRouteRefreshContext -->

The RIB's `clear bgp rib out-excluding-owner owner <id> [family <afi/safi>]`
consumer preserves this destination constraint for every output operation.
It captures each remaining owner's UP receipt, keeps same-address owners
distinct and joins the captured ordinary selected-refresh worker. Auxiliary
SENT replay uses bounded `update-route` requests carrying that same captured
`destination-owner` and `destination-session` for markers and immutable raw
route groups. Each write rechecks the observation; no address lookup may
borrow a successor session. This operational replay sends no initial replay
or readiness claim.
<!-- source: internal/component/bgp/plugins/rib/rib_resend_excluding_owner.go -- resend, resendExcludedSent -->


### Cached-write errors and joined output

A bound writer's failure is returned in the same completion output only after
all admitted target carriers join, or after refusal before writer admission.
`completed: true`, the exact `deadline-ms`, `failure` and `failure-cause` let the
SDK return `CachedWriteJoinedError`; this proves no request-owned target writer
remains. It does not prove the requested advertisement or cleanup succeeded.
A local shape/deadline refusal before the SDK calls the engine returns
`CachedWriteNotDispatchedError`.
`CachedWriteNotDispatched` requires that actual refusal cause. A nil cause
returns an ordinary error, not success or either settlement class; it cannot
prove that dispatch was avoided or that target writers joined.
<!-- source: pkg/plugin/rpc/cached_settlement.go -- CachedWriteJoinedError, CachedWriteNotDispatchedError -->
<!-- source: internal/component/plugin/server/dispatch_cached_bound.go -- forwardCachedRequest, relayStoredRouteRequest -->

Generic cancel-request confirms request-handler completion only. It is not a
cached target-writer completion receipt. Plain context cancellation and deadline
errors remain unknown even after acknowledged request cancellation.
`CancellationUnsettledError`, transport failure, malformed
or absent completion, false completion and mismatched deadline prove no such
join. Every other generic error remains unknown. Lifecycle owners must preserve
their current unknown or required cleanup/replay obligations; they must not
read a generic error or a cache ACK as completion. Ordinary completed forward
refusals need no separate output history.
<!-- source: pkg/plugin/rpc/mux_cancel.go -- CallRPCCancellable, cancelAndJoin -->
<!-- source: pkg/plugin/sdk/sdk_cached_bound.go -- cachedWriteCall -->


Actual SENT UPDATE events carry the original producing session's
`destination-session` as descriptive provenance on both the structured and
full JSON transports. The engine captures it from that exact writer's Session;
RIB rows retain it beside their sent message. This field grants no selected
ownership. An old cleanup cannot borrow a replacement destination's session
from current metadata. The existing constrained update-route request carries
that original destination owner/session and the original sent-owner-message;
its bound maintenance path joins synchronously after that destination's FIFO.

<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyOneMessage -->
<!-- source: internal/component/bgp/server/events.go -- getStructuredEvent -->
<!-- test: internal/component/bgp/reactor/sent_destination_capture_test.go TestSentDestinationCaptureKeepsOriginalSession -->
<!-- test: internal/component/bgp/server/events_sent_destination_test.go TestStructuredSentKeepsCapturedDestinationSession -->

## Design decisions

| Decision | It forbids | Source | Date |
|----------|------------|--------|------|
| Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter | An operation with no declaration; authorizing a plugin identity by prefix | plan/release/fixes/spec-explicit-plugin-and-attach-config.md | 2026-11-23 |
| Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly | Any drop-on-full or lossy overflow policy on engine-to-plugin event delivery | plan/release/fixes/spec-bus-observer-delivery-decoupling.md | 2026-11-25 |
