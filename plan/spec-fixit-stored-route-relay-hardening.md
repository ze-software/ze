# Spec: fixit-stored-route-relay-hardening

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | `plan/deferrals/fixit-stored-route-relay-hardening.md` |
| Updated | 2026-08-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. The `fixit-bgp-egress-rail-divergence` record (retired with the learned corpus) — the change this hardens
3. The Run 2 review artifact recorded under `tmp/review/` for that spec (findings R2-4..R2-9)
4. `plan/deferrals/fixit-bgp-egress-rail-divergence.md` — the rows this spec owns

## Task

`spec-fixit-bgp-egress-rail-divergence` routed the Adj-RIB-In peer-up replay through
the reactor's forward rail so a relayed route has ONE egress transform. That fixed the
four wrong-wire-bytes failures it targeted (372, 378, 394, 395) and shipped.

Two rounds of independent review found further defects in and around that path. The
ones that were mechanical were fixed there. The ones remaining are **investigations**,
not known fixes: each needs its behaviour established before code is written, because
the previous round proved that guessing at this layer produces worse outcomes than the
bug being fixed (a "partial relay must fail" guard was added that made a correctly
suppressed route fail an entire replay).

This spec investigates and closes them.

### I-1 (headline) — ADD-PATH replay is refused, and refusing removed working behaviour

`RelayStoredRoute` currently returns `errRelayAddPath` for any route whose SOURCE peer
negotiated ADD-PATH (`internal/component/bgp/reactor/reactor_api_relay.go`,
`buildRelayUpdate`). That is a deliberate fail-closed interim, but it is also a
functional regression, and both reviewers established the baseline independently:

- The old rail emitted `nlri <fam> add <hex>` with **no** `addpath` keyword (deleted
  `formatHexCommand`), and `parseWireNLRISection` defaults `addPath=false`
  (`internal/component/bgp/plugins/cmd/update/update_wire.go`).
- The structured ingest stores bare prefixes: `nlri.NLRIIterator.Next` advances past
  the 4-byte path-id and returns only the prefix (`internal/core/bgp/nlri/iterator.go`),
  which `installStructuredNLRIs` hex-encodes verbatim
  (`internal/component/bgp/plugins/adj_rib_in/rib.go`).

So add-path-sourced routes **did** replay correctly before, collapsed to path-id 0,
for single-path prefixes — and silently collapsed multi-path ones.

Worse, the refusal keys on the SOURCE context, so one add-path peer now kills peer-up
replay of its routes to EVERY destination, including non-add-path ones that worked
before. On a route server — the deployment that drives this replay — that is the
common case.

The root ambiguity: the two ingest paths store **different framings** and nothing
records which. Structured strips the path-id; legacy `prefixToWireHex` prepends it,
and only when non-zero although RFC 7911 permits path-id 0.

Investigate and decide, with evidence:
1. Whether to normalise storage (carry the path-id as a typed field on
   `rpc.StoredRoute`, or store the source framing consistently), or
2. Whether to tag the reconstruction with a context matching the source's ASN4 width
   but WITHOUT add-path (`bgpctx.EncodingContextForASN4`), which restores the old
   reach — but must be argued for multi-path routes, which it collapses.

Whichever lands must handle multi-path (several path-ids for one prefix), which
`compactRouteKey` already distinguishes but the old rail flattened.

#### I-1 FINDING (2026-08-03) — investigated, decided: normalise storage (option 1)

Every citation above was re-verified against today's tree. Two hold, one is
sharper than written, and the sharper one settles the choice.

**The two framings are real.** `nlri.NLRIIterator.Next`
(`internal/core/bgp/nlri/iterator.go`) consumes the 4-byte path-id and then sets
the prefix window to start AFTER it, so `installStructuredNLRIs`
(`internal/component/bgp/plugins/adj_rib_in/rib.go`) hex-encodes a BARE prefix,
always. `prefixToWireHex` (same file) prepends the path-id, but only under
`pathID != 0` — and RFC 7911 permits path-id 0, so a legal zero is stored bare
and is indistinguishable from a non-add-path prefix. Confirmed as written.

**The path-id is NOT lost at ingest. It is lost at the RPC boundary.** This is
the fact the spec's option list was missing. `installStructuredNLRIs` already
HOLDS the path-id and already uses it: `routeKeyFromWire(fam, pfx, pathID)`
(`adj_rib_in/compact_key.go`) puts it in `compactRouteKey.PathID`, so two paths
for one prefix are two distinct stored entries and both survive. What drops the
value is the payload: `RawRoute` has no path-id field, and neither does
`rpc.StoredRoute` (`pkg/plugin/rpc/types.go` — SourcePeer, Family, AttrHex,
NextHopHex, NLRIHex, and nothing else).

**So assumption A-2 is PARTLY BROKEN, and in the direction that matters.**
Multi-path is representable in STORAGE and not across the RELAY. Option 2
(tagging the reconstruction with a non-add-path context) therefore does not merely
"collapse" multi-path the way the old rail did: the relay carries N stored routes
for the same prefix, each reconstructed with no path-id, so the destination
receives the same prefix announced N times and keeps the last. That is silent
route loss on the deployment this replay exists for, not a documented limitation.

**Option 1, and the path-id must be a TYPED field rather than framing in the
hex.** The value is already in hand at the only producer that would have to carry
it, and the legacy producer takes it as an argument too, so `RawRoute` and
`rpc.StoredRoute` each need one field and no new plumbing. Holding the value
rather than inferring it from the bytes is also what lets `buildRelayUpdate` frame
the NLRI to match whichever context it tags the reconstruction with, which is how
`errRelayAddPath` lifts without a second rail (learned 1271's constraint). Putting
it back into the hex would re-create the ambiguity being removed.

**Trap for the implementer: 0 is a legal path-id, so an added field must not use
0 as "absent".** `rpc.StoredRoute` is a wire contract shared with FORKED
adj-rib-in plugins over JSON. A forked plugin built against the old struct omits
the field, and an omitted JSON number decodes as 0 — which reads as a valid RFC
7911 path-id, not as "this producer does not know". That is the zero-value trap in
`ai/rules/evidence.md`, on a cross-process boundary where the old producer keeps
running. Carry the framing explicitly (an enum, or a separate has-path-id bool),
not a bare `PathID uint32`. Fixing `prefixToWireHex`'s `pathID != 0` asymmetry is
part of the same change and is an independent RFC 7911 defect today.

Implementation of this finding is NOT in this session's diff: it changes a plugin
RPC contract and the stored-route shape, which is its own phase.

#### I-1 FINDING, part 2 (2026-08-14): A-1 confirmed, the legacy framing is worse than recorded, and the refusal's replacement

Written for Implementation Step 1. Every claim below was read at the function
that produces the behavior. Part 1 above is unchanged and still governs the
choice; this part supplies A-1, the per-path storage facts, the costed
comparison, the condition the refusal becomes, and the RFC 7911 position.

**A-1 is CONFIRMED. The legacy ingest path is reachable in a supported
deployment.** Four producers make the chain, and nothing else selects between
the two handlers:

| Step | Producer | What it does |
|------|----------|--------------|
| The plugin can run as a child process | `register.go` `reg.CLIHandler` into `cli.RunPlugin` (`internal/component/plugin/cli/cli.go`) | ends in Engine Mode: takes the connection from the environment and calls `RunEngine`. bgp-adj-rib-in carries no `IsInternal()` refusal of the kind `internal/plugins/vrrp/register.go` uses, and `docs/guide/graceful-restart.md` documents the external form for a sibling plugin |
| A child process has no bridge | `Process.HasStructuredHandler` (`internal/component/plugin/process/process.go`) | reports `p.bridge != nil` and the bridge exists only for in-process delivery |
| The engine then sends text or JSON | `onMessageReceived` (`internal/component/bgp/server/events.go`) | sends a `StructuredEvent` only for a process with a structured handler, and a formatted payload otherwise |
| The JSON lands on the legacy handler | `runAdjRIBInPlugin` `p.OnEvent` into `dispatch` into `handleReceived` (`adj_rib_in/rib.go`) | the second of the two ingest paths |

**What each path stores, per source kind.** The bare wire prefix means the
RFC 4271 length byte and the prefix bytes, with no path-id.

| Ingest path | Source without ADD-PATH | Source with ADD-PATH |
|-------------|-------------------------|----------------------|
| Structured, `installStructuredNLRIs` | the bare wire prefix | the bare wire prefix. `nlri.NLRIIterator.Next` consumes the 4 bytes and starts the returned window after them, so the value survives only in `compactRouteKey.PathID` |
| Legacy, `handleReceived`, raw NLRI section present | the bare wire prefix, split correctly by `splitRawNLRIHex` | MISALIGNED BYTES. See below |
| Legacy, `handleReceived`, raw NLRI section absent | `prefixToWireHex` writes the bare wire prefix | `prefixToWireHex` prepends the path-id, and writes the bare prefix when the path-id is 0 |

**The misaligned row is the new fact, and it is worse than the two-framings
story the refusal was written for.** `splitRawNLRIHex` takes no add-path
argument, so it reads the first byte of a 4-byte path-id as a prefix length. An
announcement of 10.0.0.0/24 with path-id 1 reaches the plugin as the raw section
`00000001180a0000`, and the split returns five entries beginning with `00`. The
route is keyed correctly, because the key comes from the parsed `nlri` list
through `bgp.ParseNLRIValue`, and it is stored with the wire bytes of the
default route. Key and bytes disagree. The same field is operator-visible:
`rib_commands.go` publishes it as `nlri-hex` under `show bgp adj-rib-in`.

**Which legacy row is live.** adj-rib-in requests format `full` for itself
through `SetStartupSubscriptions` (`rib.go`), and `registerSubscriptions`
(`internal/component/plugin/server/dispatch.go`) is the producer that sets the
process format from that request. The per-peer `content { format }` binding
resolves in `GetPeerProcessBindings` (`reactor/reactor_api.go`) into a
`plugin.PeerProcessBinding` that no non-test caller consumes. So the raw section
is present, the misaligned row is the live one, and the `pathID != 0` asymmetry
in `prefixToWireHex` sits on the fallback branch.

**Consequence for the design: the two competing framings are not the shape of
the problem.** Both paths agree on the framing for a source without ADD-PATH,
and the legacy path is broken rather than differently framed for a source with
one. No supported deployment stores a path-id inside the NLRI bytes except on
the unreached fallback branch. "Normalize the stored framing" is therefore
already true of the bytes. What is missing is the path-id VALUE, which is what
part 1 chose to carry as a typed field, and the legacy path's misparse, which
must be fixed in the same change or a forked adj-rib-in replays the wrong
prefix.

**Normalization versus context-tagging, costed.**

| Cost | Normalization (part 1's choice) | Context-tagging (`bgpctx.EncodingContextForASN4`) |
|------|--------------------------------|---------------------------------------------------|
| Storage shape | `RawRoute` gains a path-id value and an explicit present marker. Roughly 8 bytes per stored route after padding, so about 8 MB on a million-route table. `rpc.StoredRoute` gains the matching JSON fields | unchanged |
| Migration of already-stored routes | none. The Adj-RIB-In is in-memory only, there is no on-disk stored-route format, and `handleStructuredState` deletes a peer's routes when it goes down, so every stored route is rebuilt by the next session | none |
| Code touched | `RawRoute`, both install paths (each already holds the path-id), `handleReceived`, `buildReplayRoutes`, `rpc.StoredRoute`, `buildRelayUpdate`, the NLRI length and write helpers in `relay_payload.go`, plus `splitRawNLRIHex` and `prefixToWireHex` | `buildRelayUpdate` only |
| Correctness | full. Multi-path survives, path-id 0 survives, and both destination kinds receive wire that matches what a live forward would send | BROKEN in three ways: N stored paths for one prefix are each emitted with no path-id, so an ADD-PATH destination receives the prefix N times and keeps one; the reconstruction no longer carries the source's context, so a same-context destination loses the zero-copy forward in `buildFwdBody`; and the replayed copy stops being byte-identical to the live copy for an ADD-PATH destination, which is the one-egress-transform invariant this spec must preserve |

Context-tagging is one line and is not correct. Normalization is the simplest
FULLY correct answer, which is the test `ai/rules/simplicity.md` sets.

**What the refusal becomes.** Something is still owed, because a producer that
does not know the path-id must not have one invented for it. The condition
changes on three axes:

| Axis | Today | After |
|------|-------|-------|
| What it keys on | the SOURCE peer's context, so one ADD-PATH source loses every one of its routes to every destination, and the batch also returns an error that costs bgp-rs its delta convergence for the whole peer | the ROUTE: refuse only a route whose stored path-id is absent while its source context declares ADD-PATH for the family |
| Who it can still fire for | every route of an ADD-PATH source, including the common route-server case of a destination without ADD-PATH | only a producer that did not carry the field, which after this change means a forked adj-rib-in built against the old `rpc.StoredRoute` |
| What it means | the framing is unknowable, because nothing records it | the producer declared it does not know. Fail closed stays right: emitting a bare prefix under an ADD-PATH context corrupts the destination, and inventing a path-id 0 silently merges paths |

The framing of the reconstruction follows the context it is tagged with, which
stays the source's receive context: write the path-id when that context declares
ADD-PATH for the family, and write the bare prefix when it does not.
`fwdReencodeNLRIs` (`reactor/forward_body.go`) then converts per destination, in
both directions, exactly as it does for a live forward.

**RFC 7911, and one question that is Thomas's.**

`prefixToWireHex`'s `pathID != 0` branch is a defect, and it is NOT wire-visible
today. The only producer that writes a path-id onto the wire is
`fwdReencodeNLRIs`, which prepends 4 bytes for every NLRI whenever the
destination context declares ADD-PATH, path-id 0 included, so the encoder is
conformant with Section 3 (RFC7911-3-1, "the NLRI encoding MUST be extended by
prepending the Path Identifier field"), which is unconditional on the value, and
with the summary's own note that path-id 0 is a valid identifier with no special
meaning. `prefixToWireHex` writes storage, and its output reaches the wire only
through `writeRelayPayload`, which `errRelayAddPath` refuses to reach for an
ADD-PATH source. It becomes a wire violation on the day Step 2 lifts the
refusal, so it is release-gating for that reason and is fixed inside the same
change. The same asymmetry sits in two JSON producers, `appendNLRIJSON`
(`bgp/format/text_json.go`) and the raw-body walker in `text_update.go`: there
it is a plugin-API fidelity defect, because a peer's legal path-id 0 is reported
to a forked plugin as a route with no path-id at all.

~~RFC7911-2-2 is separate and is NOT settled here.~~ **SUPERSEDED 2026-08-17: the
question is CLOSED and no owner decision is owed.** Section 2 states that a
speaker which re-advertises a route MUST generate its own Path Identifier. The
paragraph below described the tree as it stood when this text was written; the
gap was closed on 2026-08-14, after it.

Re-verified at the producer 2026-08-17: `internal/component/bgp/reactor/forward_path_id.go`
mints ze's own identifier per ingress path (`fwdPathIDTable.generate`, keyed on
(source, received identifier)), and BOTH rails read it -- the raw same-context
forward through `fwdRegenerateRawPathIDs` and the re-encode through
`fwdPathIDMemo` inside `fwdReencodeNLRIs`. `docs/features/rfc-status.md` records
"Closed 2026-08-14: RFC7911-2-2", and both polarities are tagged in
`forward_path_id_test.go` and `forward_path_id_gen_test.go`.

→ Decision: **Step 2 carries the stored path-id through the relay and lets the
existing rail mint the egress identifier.** That is the one-egress-transform
invariant working as designed, and it inherits no gap. Do NOT put RFC7911-2-2 to
Thomas, and do NOT mint a second identifier inside the relay.

→ Root cause of the stale blocker, for the process rather than for this spec: a
spec parked a question against the tree and made a later step depend on it, but
nothing re-reads a parked question when the step that waits on it resumes. A
question answered elsewhere then survives as a blocker and costs the owner a
decision he has already made. General practice: re-verify a parked question at
its producer before acting on it, and treat spec prose about the tree as a dated
observation rather than a standing fact (`ai/rules/evidence.md`).

### I-1b — the ownership claim's ordering is not deterministic, and the naive fix deadlocks

bgp-rs claims replay ownership from `OnAllPluginsReady`
(`internal/component/bgp/plugins/rs/server_handlers.go`, `claimReplayOwnership`). That
guarantees the dispatcher's command registry is frozen, but NOT that the claim lands
before the first session establishes: `sendPostStartupToAll`
(`internal/component/plugin/server/startup.go`) fans out one goroutine per plugin and
returns, and `signalStartupComplete` then calls `SignalPluginStartupComplete` ->
`StartPeers`. So the FIRST peer can still race the claim and be replayed twice.

**Making `sendPostStartupToAll` wait was tried on 2026-07-25 and DEADLOCKS.** That
function runs immediately before peers start, so a handler that waits on peer activity
blocks the peers it is waiting for until `postStartupTimeout`. Three functional tests
failed that way (394, 395, and the adj-rib-in replay test, with the observer reporting
"no routes stored"). The wait was reverted and the finding recorded in the function's
own doc comment.

So determinism here needs a DECLARATIVE route: ownership carried through the ordered
startup stages (a registration field surfaced to adj-rib-in at configure time, which is
strictly before ready and before peers start) rather than a callback racing them.
Investigate that shape; do not re-attempt the wait.

### I-2 — `relayChunkSize` bounds route count, not bytes — DONE (2026-08-23)

`relayRoutes` chunked at 4096 routes to stay under the 16 MB IPC frame ceiling
(`pkg/plugin/rpc/framing.go`). Route count does not bound bytes: `AttrHex` is hex, so
roughly twice the attribute block, and 4096 x a 4 KB block is already ~33 MB. A forked
adj-rib-in with large communities or AS_PATH blocks still lost a whole chunk as one
oversized frame. It fails closed, so this was availability, not corruption.

**Replaced by a byte-budget accumulator**
(`internal/component/bgp/plugins/adj_rib_in/relay_chunk.go`). `relayChunkEnd(routes,
start, budget)` walks the replay accumulating `relayRouteJSONMax`, an UPPER bound on
what one route adds to the serialized `rpc.RelayStoredRouteInput`, and cuts before the
budget (`rpc.MaxMessageSize - relayFrameReserve`) is passed. `relayChunkSize` is
deleted rather than kept beside it (`ai/rules/no-layering.md`).

**One claim in the old comment was FALSE and was removed rather than carried over.**
It said chunking "also bounds how many reconstruction buffers the engine holds in
flight per call". It does not: `RelayStoredRoute`
(`internal/component/bgp/reactor/reactor_api_relay.go`) releases each route's cache
entry from a deferred closure before it builds the next one, so the only retains that
outlive a route are the per-destination writes, which the chunk size never governed.
That is the I-6 backpressure item, and it stays open.

### I-3 — replay ownership is process-global, but replay driving is per-peer — DONE (2026-08-23)

`replayOwned` is a process-wide `atomic.Bool`. Event delivery is per-peer, per-plugin
(`internal/component/bgp/reactor/config.go`, `parseOneReceiveFlag` case `"state"`), so
a peer whose `process` block gives `state` to `bgp-adj-rib-in` but not to `bgp-rs`
leaves NOBODY replaying: adj-rib-in stood down globally, bgp-rs never sees that peer.
That is a config away, not a crash away. Scope the stand-down to peers the owner
actually drives, or reject the config combination.

#### I-3 FINDING (2026-08-23) — the peer is served by NOBODY, not merely unreplayed

Read at the producers. `rs.peers[addr]` is created by `handleState` and by
`handleOpen`, and only `handleState` ever sets `Up`
(`internal/component/bgp/plugins/rs/server_handlers.go`); `selectForwardTargets`
(`rs/server_forward.go`) skips every peer whose `Up` is false. So a peer that grants
bgp-rs no `state` is not a forward target either. Measured in the functional
fixture: with the source's UPDATE in flight, bgp-rs logs `forward matched no
target ... peers-known=1`, and the dest peer's wire carries nothing but its own
End-of-RIB for the whole run. The stand-down did not cost that peer its peer-up
replay alone. It cost it every route.

**The fix is a RETRACTION delivered on the event, and the claim is unchanged.**
Two facts decide the stand-down and each has one producer. The CLAIM says the role
has an owner in this daemon; it arrives at Stage 2, ordered before peers start, and
that is what makes the first peer-up safe (I-1b). `Server.UnheldRoles`
(`internal/component/plugin/server/startup_claims.go`) says which advertised roles
NO process in THIS event's delivery set holds, computed over the `procs` slice
`PeerScopedProcs` already built, so it needs no second registry and no new
bookkeeping. `onPeerStateChange` (`internal/component/bgp/server/events.go`) puts
the answer on every copy of the state event, and `replayDrivenElsewhere`
(`adj_rib_in/rib_claims.go`) is the whole decision:
`replayOwned && !contains(unheldRoles, claimPeerUpReplay)`.

**Both delivery paths carry it, because a forked bgp-adj-rib-in is reachable
(A-1).** `StructuredEvent.UnheldRoles` (`pkg/plugin/rpc/bridge.go`, cleared on
pool return) serves the in-process bridge; `appendStateChangeJSON`
(`internal/component/bgp/format/text_json.go`) emits an escaped `unheld-roles`
array, and `Event.UnheldRoles` (`internal/component/bgp/event.go`) parses it. The
text encoding carries nothing: it is a human line, and the SDK builds no event
from one, so no plugin can act on it. Emitting the member only when it is non-empty
keeps every state event on a non-claiming daemon byte-identical to before.

**It also closes the gap `verifyAdvertisedClaims` documents.** A claimant that
never reached Running takes delivery of nothing, so it holds no role for any peer
and the retraction goes out for every one of them. That function's comment said no
engine-to-plugin channel could carry a revocation before StartPeers; the answer was
that the revocation belongs on the EVENT, not before the peers.

**Mutation-verified in three registers.** `UnheldRoles` returning nil (the pre-fix
producer) with the daemon REBUILT reddens `adj-rib-in-replay-unowned-peer.ci` at
the subject while the control, the store and the reconnect stay green, and reddens
four cases of `TestUnheldRolesRetractsAClaimForAPeerTheClaimantIsNotFed` plus
`TestUnheldRolesOverTheRealDeliverySet`. `replayDrivenElsewhere` ignoring its
argument reddens exactly the two self-replay cases of
`TestSelfReplayCoversAPeerTheOwnerIsNotFed`, one per ingest path, leaving the
stand-down cases green. `appendUnheldRolesJSON` returning its buffer unchanged
reddens four cases of `TestAppendStateChangeCarriesUnheldRoles`.

### I-4 — the ownership claim does not survive an adj-rib-in respawn — DONE (2026-08-23)

`SendPostStartup` has one call site, inside `signalStartupComplete`
(`internal/component/plugin/server/startup.go`). A mid-life respawn
(`internal/component/plugin/server/reload_tx.go` -> `internal/component/plugin/process/manager.go`
`Respawn`) receives no post-startup callback, and `replayOwned` resets with the
process, so a respawned adj-rib-in resumes self-replay and the duplicate announce
returns. Re-deliver post-startup on respawn, or make the claim re-confirmable.

#### I-4 FINDING (2026-08-23) — the claim is not the half that was missing

Read at the producers. The lost claim is a SYMPTOM, and the item's two proposed
fixes both treat it as the disease.

**A respawn re-enters no handshake at all.** `ProcessManager.Respawn`
(`internal/component/plugin/process/manager.go`) builds the replacement with
`NewProcess` and calls `newProc.StartWithContext`, and nothing else. The 5-stage
handshake is driven by `Server.handleProcessStartupRPC`
(`internal/component/plugin/server/startup.go`), whose only two non-test callers
are `runPluginPhase` and `HandleAdHocPluginSession`; neither is on the respawn
path. So the replacement was left holding no Stage-1 registration, no delivered
config, no subscriptions, no registered commands and no claim set, behind a pipe
the engine never read. The claim is one of six things it never received.

**So the fix is the handshake, not a new claim channel.** `Server.restartPlugin`
now respawns, releases the old process's name-keyed registrations
(`releasePluginRegistrations`, `internal/component/plugin/server/restart.go`, so
Stage 1 is not refused the plugin's own name), then runs the handshake and starts
the runtime handler (`restartHandshake`, same file). Stage 2 then delivers the
claim through `advertiseClaims`, which is the ordered declarative route I-1b
already established. No second channel was added.

**The TRIGGER is unreachable in a shipped daemon, and that is a separate defect.**
`ProcessManager.Respawn` returns before doing anything unless
`cfg.RespawnEnabled || cfg.Respawn` (`manager.go`), and NOTHING outside a test
sets either field: `plugin.PluginConfig` declares them
(`internal/component/plugin/types.go`), `cmd/ze/hub/main.go` copies `pc.Respawn`
from a `plugin.PluginConfig` that nobody wrote, and no YANG leaf reaches them. So
`restartPlugin` is a no-op for every plugin the daemon can be configured with, and
the broken-plugin recovery it serves has never run in production. Journal row in
`plan/journal/unwired-feature.md`. Wiring the operator-facing `respawn` option is a
FEATURE and is out of this spec: what this spec owed was that the mechanism behind
the trigger be correct, which it now is and which the tests drive directly.

**Two defects on the path were fixed with it.** `ProcessManager.Respawn` returned a
bare `nil` when respawn was not enabled, indistinguishable from a completed
restart by the caller that had just been told the plugin is broken; it now returns
`ErrRespawnNotEnabled` and the new process on success. And
`CommandRegistry.Register` wrote the mutable map without republishing the frozen
snapshot (`internal/component/plugin/server/command_registry.go`), so every command
registered after `signalStartupComplete` froze the registry was invisible to
`Lookup` -- which is every command of a restarted plugin AND of a plugin
auto-loaded by a config reload. `Unregister` had always republished; only the
addition did not. Journal row in `plan/journal/guard-added-to-one-half-of-a-pair.md`.

### I-5 — `test/plugin/adj-rib-in-replay-on-peerup.ci` does not gate — DONE (2026-08-07)

It replayed to `10.0.0.99`, which is not a configured peer, so `RelayStoredRoute`
returned at destination resolution and the test asserted on the SELECTED route count.
It passed with the relay entirely dead — it failed the mutation check in
`ai/rules/testing.md`.

**Reproduced before it was fixed.** With `return nil` as the first statement of
`RelayStoredRoute`, the old test still reported PASS.

**The rewrite.** The replay now targets an ESTABLISHED second peer on 127.0.0.2,
and the destination asserts exact wire bytes TWICE: seq=1 is the LIVE forward
(the control) and seq=2 is the RELAYED copy (the subject). Both pin the same
51-byte UPDATE, which is the direct statement of learned 1271's
one-egress-transform invariant rather than a restatement of the code. The second
peer is the DESTINATION, not the second SOURCE this item predicted: one source
suffices, because what the test lacked was a destination the relay could resolve.

**Why nothing else can produce the second copy.** No RIB plugin is loaded, so the
dest has no local-table delivery path, and adj-rib-in receives `state` for the
SOURCE peer only, so its own peer-up self-replay never fires for the dest. The
observer's explicit `request bgp adj-rib-in replay 127.0.0.2 0` is the sole
producer, and it passes two arguments so the cut stays UNBOUNDED.

**Mutation-verified both ways.** RED with the relay dead: the dest wire carries one
UPDATE and the observer reports `dest peer was never sent the relayed copy of the
stored route`, while the plugin still reports `replay selected 1 route(s)` — the
exact gap between plugin-side selection and engine-side relay that the old test
could not see. GREEN once restored. `make ze-functional-plugin-test` 558/558.

### I-6 — smaller relay gaps

- ~~RFC 2545 32-byte next hop (global + link-local) is truncated to 16, so a replay
  diverges from what a live forward relays verbatim and an on-link peer loses the
  link-local next hop.~~ **DONE 2026-08-23.**

  **The producer was `MPReachWire.NextHop` (`internal/component/bgp/wireu/mpwire.go`),
  not `nhopHexFromAddr`**: it returns one `netip.Addr` and keeps the first 16 octets
  for AFI 2, so the link-local half was gone before `nhopHexFromAddr`
  (`adj_rib_in/nlri_hex.go`) encoded what was left. The deferral shard's RFC 2545 row
  carries the same correction; the spec text here named the wrong function.

  **The RELAY side was already correct** -- `maxNextHopLen` is 0xFF and
  `mpReachValueLen` sizes from `len(nextHop)` (`reactor/relay_payload.go`) -- so only
  the STORE truncated, and the fix is at ingest. `MPReachWire.NextHopBytes` returns
  the whole field as the source framed it, and `NextHop` is rewritten to read it, so
  the two accessors cannot disagree about the length octet.

  **BOTH ingest paths are fixed, because both carried it.** The structured one takes
  `NextHopBytes` for simple and complex families alike, and `installComplexNLRIs`
  takes hex rather than an address string. The legacy one reads the field out of the
  raw attribute block through `mpReachNextHopHex` (`adj_rib_in/nlri_hex.go`), and
  falls back to the event's address string for a family the block's MP_REACH does not
  name: that event carries an address string, which cannot express the two-address
  form at all, so fixing only the in-process path would have left a forked
  bgp-adj-rib-in storing 16 octets.

  **Proof.** `TestMPReachWireNextHopBytesRFC2545` (`wireu/mpwire_test.go`) covers 16,
  32, 15 and a declared-longer-than-present field.
  `TestHandleReceivedStructuredStoresWholeRFC2545NextHop` and
  `TestHandleReceivedStoresWholeRFC2545NextHop` (`adj_rib_in/nlri_hex_test.go`) pin
  each ingest path, `TestHandleReceivedKeepsEventNextHopWithoutMPReach` bounds the
  legacy change, and `TestMPReachNextHopHexRefusesWhatItCannotRead` covers the
  derivation's own edges. `TestRelayPayloadKeepsRFC2545LinkLocalNextHop`
  (`reactor/relay_payload_test.go`) is the pair-check at the writer.
  `test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci` carries the RFC2545-3-1 and
  RFC2545-3-2 positives at functional tier: the destination receives the same 87-byte
  UPDATE twice, once from the live forward and once from the replay, and both carry
  the 32-octet pair under a Length octet of 0x20.

  **Mutation-verified in both registers.** Each ingest mutation reddens its own unit
  test with the truncation visible (64 hex chars expected, 32 received). Reverting the
  structured path and REBUILDING the daemon reddens the `.ci` at the destination's
  seq=2 while seq=1 stays green, reporting
  `expected=00020120...`, `got=00020110...` -- the Length octet and the missing 16
  octets. The `.ci` was authored in `test/draft/plugin/` and promoted green
  (`ai/rules/testing.md`).
- Complex families (VPN, EVPN, Flowspec) store the WHOLE MP_REACH NLRI block for the
  first NLRI and skip the rest (`adj_rib_in/rib.go`), so a replay re-announces every
  NLRI of the originating UPDATE — the failure the strip-and-resynthesize design
  claims to prevent, confined to these families. **STILL OPEN, and bigger than this
  row says — read the FINDING before scoping it.**

  **I-6 COMPLEX-FAMILY FINDING (2026-08-23).** Read at the producer. The row
  understates it twice, and the second half is why it cannot be fixed in place.

  **The KEY is fabricated, not just the block.** `installComplexNLRIs`
  (`adj_rib_in/rib.go`) parses the section with `wireNLRIsToAny`
  (`adj_rib_in/nlri_hex.go`), which walks `[prefix-len][address]` — the SIMPLE
  family framing. A VPN-IPv4 NLRI is `[len][label][RD][prefix]` and an EVPN one is
  `[route-type][length][body]`, so the first octet is read as a prefix length and
  the next octets as an address. The stored `compactRouteKey` therefore names a
  CIDR prefix no peer ever advertised. The dispatch comment above the call says
  complex families "fall back to Event path for correct NLRI handling", and that
  has not been true since this parser became the fallback.

  **So a withdrawal cannot remove the route.** `removeComplexNLRIs` derives its key
  the same wrong way, and the announce stored only the FIRST NLRI's key, so a
  withdraw of any other NLRI in that UPDATE deletes nothing while the stored entry
  keeps re-announcing it. A replay after a withdraw puts a withdrawn VPN or EVPN
  route back on the wire.

  ~~**The splitter this needs already exists**: `nlrisplit.Split(fam, data, addPath)`
  (`internal/core/bgp/nlri/nlrisplit/`) carves a section into per-NLRI slices,
  registered per family by the NLRI plugins, and each slice includes the RFC 7911
  identifier.~~ **CORRECTED 2026-08-24 — it exists for SOME of these families and
  not for the two the row names first.** The relay end is ready as written:
  `NLRIFramingSourceWire` already means "the stored bytes carry the source's
  framing" (`reactor_api_relay.go`, `buildRelayUpdate`), so one stored route per
  NLRI reconstructs unchanged.

  **The splitter registry does not cover MPLS-VPN or flowspec.** Read at the
  producers. Every registration in the tree is `nlrisplit.init`
  (`internal/core/bgp/nlri/nlrisplit/register.go`) plus one call in
  `internal/component/bgp/plugins/nlri/srpolicy/register.go`, and between them
  they cover IPv4/IPv6 unicast and multicast (`splitCIDR`), L2VPN/EVPN
  (`splitEVPN`), MVPN v4/v6, MUP v4/v6, MPLS-label v4/v6 and SR-Policy v4/v6.
  `SAFIVPN` (128), `SAFIFlowSpec` (133) and `SAFIFlowSpecVPN` (134) appear in
  none of them, and neither do `SAFIRTC` (132), `SAFIVPLS` (65) or BGP-LS
  (71, 72). `nlrisplit.Split` answers `ErrUnsupported` for each
  (`nlrisplit.go`, `Split` into `Get`).

  Which families reach this code is `isSimplePrefixFamily`
  (`adj_rib_in/nlri_hex.go`): everything except IPv4/IPv6 unicast and multicast,
  so the complex path takes the covered families AND the uncovered ones. A
  per-NLRI rewrite therefore has no splitter to call for VPN-IPv4/IPv6
  (RFC 4364 `[len][label stack][RD][prefix]`) or for flowspec (RFC 8955
  component tuples), which are exactly the two the row's first sentence names.
  Writing those two splitters is RFC work in a core package with its own
  conformance obligations, and it is a precondition of the key change rather
  than part of it.

  → The item is BIGGER than the 2026-08-23 finding recorded, for the second
    time. It needs its own spec, and that spec owes three things in order: the
    two missing splitters, then the key type, then the per-NLRI storage.

  **What blocks it is the KEY TYPE, and that is why this is spec-sized.**
  `compactRouteKey` is `{family, netip.Prefix, pathID}` (`compact_key.go`). No EVPN
  route type, VPN RD-prefix or flowspec rule is a `netip.Prefix`, so one route per
  complex NLRI needs a key holding the NLRI's own bytes. That key is read by the
  pending-validation map, the early-decision map, the `show bgp adj-rib-in` surface
  and `docs/architecture/plugin/rib-storage-design.md`. A HALF fix — per-NLRI
  storage under a still-fabricated prefix key — is worse than today, because two
  NLRIs of one UPDATE would then collide on one bogus key and overwrite each other.
  Scope the key first.
- No backpressure: each in-flight relay pins a read-pool buffer with no bound, so a
  slow destination pins many and then fails the remainder. **STILL OPEN. The premise
  in this row is STALE — read the FINDING.**

  **I-6 BACKPRESSURE FINDING (2026-08-23).** Read at the pool.

  **A parked item pins no read-pool buffer.** `DispatchOverflow`
  (`reactor/forward_pool.go`) calls `ownOverflowBodies` before queueing, precisely
  because "the item is about to sit in an unbounded queue". It takes an overflow-mux
  handle of its own, and when that pool is exhausted it proceeds WITHOUT one rather
  than failing. The same site states the rule the row contradicts: "routes never
  dropped". So the remainder is not failed, and the buffer the row names is not the
  thing that accumulates.

  **The queue is unbounded BY DESIGN**, with escalation delegated to the congestion
  controller's later layers (read throttling, then teardown). Both act on the
  DESTINATION. Neither can slow a producer that is not reading from a socket at all,
  which is exactly what a peer-up replay is.

  **That is the real gap, and it is the relay's.** `RelayStoredRoute`
  (`reactor_api_relay.go`) walks the whole stored table in one call, one
  `forwardUpdateCore` per route, with nothing in the loop reading the destination's
  queue depth. A slow destination therefore grows that unbounded queue by the size
  of the table, in one call, and nothing says so.

  **What exists is not the signal this needs.** `Peer.forwardOverflowPending`
  (`reactor/peer.go`) is an ORDERING gate — "the pool owes this peer bytes" — that
  the route-server rail reads to keep a direct write from overtaking a parked item
  (`forward_rs.go`). It carries no depth and no threshold.

  **So this needs an owner decision, not a patch**: what the replay waits on, and
  what it does when the destination stays behind. Failing the replay is already
  reportable (`errRelayIncomplete`), but it costs the peer its table; waiting spends
  the caller's goroutine against its own 2-minute deadline (`rs/server_handlers.go`,
  `replayForPeer`).

  **RE-VERIFIED 2026-08-24, at both producers. The finding stands unchanged.**
  `fwdPool.DispatchOverflow` (`reactor/forward_pool.go`) still calls
  `ownOverflowBodies(&item)` immediately before it queues, under its own comment
  "The item is about to sit in an unbounded queue", and the same function states
  "routes never dropped" on the pool-exhausted branch. So a parked item pins no
  read-pool buffer and the remainder is not failed: the row's original premise is
  stale in the tree as well as in the finding. `RelayStoredRoute`
  (`reactor_api_relay.go`) still walks every route in one call, one
  `forwardUpdateCore` per route, and reads no queue depth anywhere in the loop.

  → This is the ONE open question this spec puts to Thomas, and it is a "which
    way", not a "may I skip it". Both answers are implementable and neither is
    free: a producer-side wait spends `replayForPeer`'s 2-minute deadline and can
    starve the peer of its table anyway, while failing fast at a depth threshold
    reports `errRelayIncomplete` and costs the peer its table immediately. Nothing
    in the relay can be written until he says which cost the daemon should pay.
- ~~`routeRelayer` (the test seam) has no error return, so `replayCommand`'s
  `statusError` path cannot be driven by a test.~~ **DONE 2026-08-23.** The seam
  returns `error` and sits inside the chunk walk (`relayChunk`, `adj_rib_in/rib.go`),
  so a test sees the chunks the engine sees rather than the whole replay.
  `TestReplayCommandReportsRelayFailure` (`adj_rib_in/relay_chunk_test.go`) drives the
  `statusError` path and asserts the cause reaches the caller.
- ~~`Coordinator.RelayStoredRoute` has no test — neither the `ErrNoReactor` branch nor
  the delegation is exercised.~~ **DONE 2026-08-23.** `TestCoordinatorRelayStoredRoute`
  (`internal/component/plugin/coordinator_test.go`) drives both branches and asserts
  the destination and the route slice cross unchanged and the reactor's error comes
  back. The mock records what it was handed rather than only that it ran, because a
  delegation that drops the routes or swallows the error turns a failed replay into a
  reported one. Mutation-verified twice: reporting the no-reactor case as success, and
  dropping the routes while swallowing the error, each redden it.
- ~~`relay_payload_test.go` asserts `n <= size` where `n == size` is the real
  contract.~~ **DONE 2026-08-23.** `buildRelay` requires equality. A SHORT write
  overflows nothing: `buildRelayUpdate` hands the `WireUpdate` `buf[:n]`, and
  `writeRelayPayload` back-fills the attribute-length field from its own offset, so a
  truncated body agrees with itself and nothing downstream can tell. The bound
  accepted exactly that.

### R6-1 — an egress filter cannot report "I could not decide", and one already needs to

Homed here from `spec-fixit-bgp-egress-rail-divergence` Run 6. Verified against the
tree on 2026-08-03.

`filterapi.EgressFilterFunc` returns a bare `bool`, so an in-process egress filter
can say accept or reject and nothing else. `safeEgressFilter`
(`internal/component/bgp/reactor/reactor_notify.go`) therefore returns
`(accept, panicked)` where `panicked` is the only non-decision it can ever report,
because it is the only one it causes itself. `forwardUpdateCore`
(`reactor_api_forward.go`) folds that into `egressStepResult{accept, failed}`, and
a suppression with `failed == false` increments `suppressedCount`, which becomes
`errAllDestinationsSuppressed`, which `RelayStoredRoute` (`reactor_api_relay.go`)
counts as `relayed++` — a handled route.

The gap is already live in `OTCEgressFilter` (`role/otc.go`). Its export-set block
reads `destCapRole` from `getFilterConfig` (`role/role.go`), a bare map read of
`filterRemoteRoles` whose absent key yields `""`, and maps `""` to `roleUnknown`.
But `""` means BOTH "this peer's OPEN declared no role" (a decision) and "no OPEN
was ever recorded for this peer" (a failure). The second is reachable:
`broadcastValidateOpen` (`internal/component/bgp/server/validate.go`) skips a
plugin on a nil process manager, a nil conn, or an RPC error and lets the session
establish regardless; and `setFilterState` (`role/role.go`) nils the whole map on
every reconfigure, while an already-established peer sends no second OPEN.

Consequence: a source configured `role { export ... }` whose set omits `unknown`,
plus a destination whose role went unrecorded, suppresses EVERY stored route and
reports the replay complete. Both halves are required — the signature alone leaves
the map ambiguous, and the map alone has no channel to report the miss upward.

### R6-2 — a non-decision ACCEPT in the export policy chain

Homed here from the same review. `runEgressPolicyChainASN4`
(`internal/component/bgp/reactor/filter_ordered.go`) calls
`decodeFilterRawOverride` (`filter_chain.go`), which returns nil for ANY raw
shorter than 4 bytes. A raw filter that returns 1..3 bytes has asked for a full
UPDATE-body replacement that cannot be a valid body; the nil is discarded
silently and the route is forwarded UNMODIFIED, indistinguishable from "the filter
accepted it as-is". `PolicyFilterChain` (`filter_chain.go`) makes a raw response
terminal and returns `Text: current`, so the text-delta branch below does not run
either. The ingress twin `runIngressPolicyChain` has the identical shape, and
there the route is cached and dispatched unmodified.

This is the RFC 6996 private-ASN leak class the file exists to prevent: the raw
seam is what a filter uses for surgery the text delta cannot express.

## Required Reading

### Architecture Docs
- [ ] The `fixit-bgp-egress-rail-divergence` record (retired with the learned corpus) — what was built and why
  → Constraint: a relayed route must keep exactly ONE egress transform; do not
     reintroduce a second rail while fixing add-path.
  → Constraint: the stored attribute block is the WHOLE attribute section including
     MP_REACH/MP_UNREACH (assumption A-1, verified), so any reconstruction strips 14/15.
- [ ] `ai/rules/evidence.md`
  → Constraint: a guard that cannot deny must speak. The Run 2 blocker was a guard
     that denied something legitimate; both failure modes are in scope.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7911.md` — ADD-PATH; path-id 0 is legal, and multi-path means
      several path-ids per prefix
- [ ] `rfc/short/rfc4760.md` — MP_REACH_NLRI encoding
- [ ] RFC 2545 — 32-byte IPv6 next hop (global + link-local). **No summary and no
      full text are in the repository yet.** I-6 cannot be implemented until both
      are produced through `/ze-rfc` (which also owes enrolment, an extraction
      sign-off and a `docs/features/rfc-status.md` row). Recorded here so the
      obligation stays visible; do NOT implement I-6 against memory of the RFC.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing the design)
- [ ] `internal/component/bgp/reactor/reactor_api_relay.go` — `RelayStoredRoute`,
      `buildRelayUpdate`, `resolveRelaySource`, the `errRelay*` sentinels
- [ ] `internal/component/bgp/reactor/relay_payload.go` — the byte builders
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` — `RawRoute`, both ingest
      paths, `buildReplayRoutes`, `relayRoutes`, `replayOwned`
- [ ] `internal/core/bgp/nlri/iterator.go` — where the path-id is stripped
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` — `replayForPeer`,
      `claimReplayOwnership`, and the EOR-on-failure path

**Behavior to preserve:**
- The four target `.ci` tests stay green and non-reproducing under
  `scripts/dev/stress-repro.py`. **NAMED 2026-08-24: the spec carried them as the
  bare ids 372, 378, 394, 395, and those ids no longer resolve to them.** They are
  POSITIONS in the plugin suite, which was 495 tests when the numbers were written
  and is 702 now, so `ze-test bgp plugin -l` today reads 372 as
  `mcp-mrtr-unsolicited-state-rejected`, 378 as `mcp-tools-list-cache-hints`, 394
  as `metrics-name-show` and 395 as `metrics-session-lifecycle` -- four MCP and
  metrics tests with no egress rail in them.

  The mapping IS recoverable, from `plan/deferrals/fixit-load-dependent-functional-failures.md`,
  whose 2026-07-24 row spells the cluster out. The four are:

  | Then | Now | Name |
  |------|-----|------|
  | 372 | 547 | `remove-private-as-replace-peer` |
  | 378 | 560 | `rfc7606-relay-one-field` |
  | 394 | 577 | `role-otc-egress-filter` |
  | 395 | 578 | `role-otc-egress-stamp` |

  → Cite the NAME from here on. A `.ci` name is stable and its id is not, so an
    id is never a citation. Run them with `--pattern <name>`, and read the whole
    suite for the rest.
- One egress transform per relayed route.
- Originated / injected / redistribute routes keep going through
  `exportFilterForBody` (learned 1231, the private-ASN leak).

**Behavior to change:**
- Add-path sources must become replayable rather than refused (I-1).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The legacy (non-structured) ingest path is still reachable in a supported deployment | `handleReceived` exists for external text/JSON plugins | If dead, storage normalisation is far simpler — one framing only | grep for a config that delivers events as JSON to adj-rib-in | **confirmed (2026-08-14)**: a forked bgp-adj-rib-in has no bridge, so `Process.HasStructuredHandler` is false and `onMessageReceived` sends JSON, which `p.OnEvent` routes to `handleReceived`. See I-1 FINDING part 2 |
| A-2 | Multi-path (several path-ids for one prefix) is representable end to end today | `compactRouteKey` carries a path-id | The old rail's collapse was masking a storage gap, widening I-1 | store two paths for one prefix and inspect the seqmap | **broken (2026-08-03, re-verified 2026-08-14)**: representable in STORAGE (`routeKeyFromWire` keys on `PathID`), NOT across the relay: `RawRoute` (`adj_rib_in/rib.go`) and `rpc.StoredRoute` (`pkg/plugin/rpc/types.go`) still carry no path-id field. See the I-1 FINDING. **RESOLVED 2026-08-19 by Step 2**: `RawRoute` and `rpc.StoredRoute` each carry `PathID` plus an `NLRIFraming` marker, so N stored paths for one prefix reach an ADD-PATH destination under N identifiers. Pinned by `TestRelayMultiPathPreserved` and `TestHandleReceivedStructuredAddPathStoresIdentifier` |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | A fix for I-1 re-breaks the byte-identity the four target tests assert | the plugin suite goes red. The early-signal cell said "372/378/394/395 go red" and those ids no longer name those tests -- see the correction under Current Behavior | Run `make ze-functional-plugin-test` whole, plus stress-repro, on every change |
| R-2 | Chunking by bytes changes the `last-index` bgp-rs uses for delta convergence | rs delta loop never terminates, or replays forever | Assert `last-index` across a multi-chunk replay |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer-up replay from an ADD-PATH source | Routes are relayed, not refused; wire is well-formed for both add-path and non-add-path destinations |
| AC-2 | Multi-path source (two path-ids, one prefix) | Both paths survive the replay, or the limitation is explicit and tested |
| AC-3 | Forked adj-rib-in, replay whose routes exceed 16 MB | Chunked by bytes; every route arrives. **DONE 2026-08-23** -- `relayChunkEnd` (`adj_rib_in/relay_chunk.go`) cuts on the serialized byte budget and `relayRoutes` walks it. `TestRelayChunkStaysUnderFrameCeiling` marshals every chunk of a 130-route, 64 KiB-attribute replay and asserts each one lands below `rpc.MaxMessageSize`; `TestReplayLastIndexSurvivesMultiChunkRelay` drives the same shape through `handleCommand` and pins `last-index` and `replayed` across the boundary (R-2). Mutation-verified: restoring the 4096-route cut puts chunk 0 at 17 053 828 bytes and reddens three tests |
| AC-4 | Peer gives `state` to adj-rib-in but not bgp-rs | That peer is still replayed. **DONE 2026-08-23** -- the claim stays daemon-wide and the engine RETRACTS it per event: `Server.UnheldRoles` (`internal/component/plugin/server/startup_claims.go`) names the advertised roles no process in the delivery set holds, `onPeerStateChange` puts them on every copy of the state event (`StructuredEvent.UnheldRoles`, and the `unheld-roles` JSON member for a forked plugin), and `replayDrivenElsewhere` (`adj_rib_in/rib_claims.go`) reads them in BOTH ingest paths. `test/plugin/adj-rib-in-replay-unowned-peer.ci` proves it end to end; three unit tests pin the pieces. Mutation-verified in three registers -- see the I-3 FINDING |
| AC-5 | adj-rib-in respawns mid-life with bgp-rs loaded | Ownership is re-established; no duplicate announce on the next peer-up. **DONE 2026-08-23** -- `Server.restartPlugin` (`internal/component/plugin/server/reload_tx.go`) now respawns, releases the old process's name-keyed registrations and re-runs the 5-stage handshake (`restart.go`), so Stage 2's `advertiseClaims` re-delivers the claim before the replacement can receive any event. `TestRestartPluginReRunsTheStartupHandshake` drives it end to end with two real SDK plugins and asserts the replacement was configured again, was told the role is still claimed, and owns its command; `TestRestartPluginRefusesWhenRespawnIsNotEnabled` pins the refusal. Mutation-verified: with the pre-fix producer restored (no teardown, no handshake, and a silent nil for not-enabled) the first reports 1 configure against 2 and a nil command lookup, the second reports "an error is expected but got nil". **The trigger is unwired -- see the I-4 FINDING** |
| AC-6 | `adj-rib-in-replay-on-peerup.ci` with the relay stubbed to error | Test goes RED (mutation-verified). **DONE 2026-08-07 — see I-5** |
| AC-7 | An egress rail cannot carry out its decision: an in-process filter PANICS on the RS fast path (`forward_rs.go`) or on the LLGR stale re-advertise rail, or that rail's modifications cannot be built | No rail reports a failure of THIS speaker as a peer's policy. `reactorForwardRS` puts the destination on the skipped list so the plugin rail decides what it could not. `decideStaleReadvertise` returns `staleFilterFailed` or `staleBuildFailed`, and `AnnounceNLRIBatch` reports the matching cause wrapping `errStaleReadvertiseWithheld` instead of `ErrNoPeersAcceptedFamily`. The route is withheld fail-closed in every case; only the report changes. **REPLACED 2026-08-03 by Thomas's ruling — see "AC-7 replaced" below. DONE** |
| AC-8 | Destination peer whose remote role was never recorded, source with a non-empty `role { export }` set (R6-1) | The suppression is counted under its own reason (`role-unrecorded`) and carries recordDrop's first-occurrence WARN, so it is never reported as an export-set decision. **DONE** |
| AC-8b | Config reload while peers stay established (R6-1) | Learned remote roles survive for every peer the new config still names, so no peer is reclassified `unknown` and black-holed by a reload. **DONE** |
| AC-9 | An export or import policy filter returns a raw override of 1..3 bytes (R6-2) | The route is suppressed (export) / dropped (import) and says so; it is never forwarded unmodified. **DONE** |
| AC-10 | A zero-dispatch FAILURE branch of `forwardUpdateCore`: read-buffer pool exhaustion in the EBGP wire build and in both transcode acquisitions, a `buildFwdBody` failure, and both dispatch attempts failing on a stopped pool | Each counts as a DROP, never as a policy suppression, and a test drives at least one branch per producer. Today only two failure branches are covered (nil forward facts, and a failing egress step), so a future `suppressedCount++` on one of these paths would reopen the fail-open with every test green. Added 2026-08-14 from `plan/deferrals/fixit-bgp-egress-rail-divergence.md`, which named this spec as its destination while no criterion here enumerated it. **DONE 2026-08-23** -- `TestForwardZeroDispatchFailureIsADropNotASuppression` (`reactor/forward_failure_verdict_test.go`) drives the three producers no test reached: the rebuild (`buildModifiedPayload`), the RFC 9494 withdrawal conversion (`buildWithdrawalPayload`) and the EBGP AS-path record (`ASPathEdit.Record`). Each asserts BOTH halves -- `errNoEstablishedPeersToForwardTo` is returned and `errAllDestinationsSuppressed` is not -- and a fourth case pins that a filter's clean refusal still IS a suppression, so the negative assertions are not vacuous. Mutation-verified by making all three branches `suppressedCount++`, which is exactly the regression this criterion names: the three flip red, the control stays green. **The AC's own list was written against an older shape**: the EBGP wire build's read-buffer borrow no longer exists (both caches and their adoption were deleted with the edit-set fold), the transcode acquisitions surface as `buildFwdBody` returning !ok, which `TestForwardUpdateCoreReturnsModBufOnBodyFailure` already pins to the drop sentinel, and the stopped-pool dispatch pair is pinned by `TestForwardRefusedClientWithWithdrawalIsNotCountedSuppressed`. No functional test: the classification has no user-visible surface of its own, and its one consumer -- the relay's completeness report -- is gated by the three `adj-rib-in-replay-*.ci` (`ai/rules/testing.md`, "When Unit Tests Alone Are Sufficient", row 2) |
| AC-11 | The two non-decided `PolicyReject` producers that no test drives: a filter IPC error under a fail-closed `FilterOnError`, and the AC-13 undeclared-attribute override | Both set `Failed`, and removing the flag from either turns a test RED. `Reactor.api` is a concrete `*pluginserver.Server` rather than an interface, so this needs a live plugin server or an injection seam. Added 2026-08-14 from the same shard, for the same reason. **DONE 2026-08-23** -- `TestPolicyFilterFailedFlagMarksANonDecision` (`reactor/filter_failed_flag_test.go`) drives both, each beside its control: an `OnErrorAccept` policy forwards the same failed call, and a modify of a DECLARED attribute is applied. The IPC-error branch needs no seam at all -- a real `pluginserver.Server` holding no processes answers `unknown plugin` and `OnErrorReject`, which is the daemon's own answer for a filter whose plugin died. The undeclared-attribute branch needs a SUCCESSFUL filter call, which no exported surface lets a reactor-package test arrange (`Server.procManager` is written only from `runPluginPhase`), so `filterTransport` was added: the three-method surface the IPC body talks to, satisfied by `*pluginserver.Server`, with the seam field nil in production. `TestEgressChainCarriesTheFailedFlagToTheStep` carries the flag to `egressStepResult.failed`, which is the hop that reaches AC-10's verdict. Mutation-verified: dropping `Failed: true` from either producer reddens its case and leaves both controls green |
| AC-12 | bgp-rs joins MID-LIFE, auto-loaded by a config reload that adds the `bgp` root (`startup_autoload.go` `autoLoadForNewConfigPaths`), while bgp-adj-rib-in is already running | Exactly one plugin replays on the next peer-up. Today neither channel reaches this case: Stage 2 runs per handshake, so an already-configured bgp-adj-rib-in is never re-told, and the backstop `claimReplayOwnership` fires only from `OnAllPluginsReady`, which the engine produces once at startup. This is the mirror of AC-5, whose direction is the adj-rib-in respawn. Added 2026-08-14 by the closure review of `spec-fixit-bgp-egress-rail-divergence`. **DONE 2026-08-23** -- `autoLoadForNewConfigPaths` (`startup_autoload.go`) now delivers the post-startup callback to the plugins THAT phase started, and to nobody else (`sendPostStartupToNames`, `internal/component/plugin/server/poststartup.go`), so a mid-life bgp-rs runs its `OnAllPluginsReady` and its `claimReplayOwnership` backstop reaches the running bgp-adj-rib-in. The names rather than every running process, because a second delivery re-runs an already-run handler. The deadlock `sendPostStartupToAll` records does not reach the mid-life path: peers are already running, so a handler waiting on peer activity waits on peers nothing is holding back. `TestMidLifeAutoLoadDeliversPostStartup` (`poststartup_test.go`) drives a real reload-time auto-load and asserts the claim reaches the holder; mutation-verified, the pre-fix producer leaves it "Condition never satisfied" after 5s. The function's own comment said it signaled the plugins while it signaled only the reactor |

### Q-1 — RULED BY THOMAS, 2026-08-03: an UNRECORDED role KEEPS matching `export { unknown }`

**The question put to him.** Should a destination whose role was NEVER RECORDED
(validate-open RPC timeout, plugin conn not up, plugin respawn) still match an
explicit `export { unknown }`?

**His answer, verbatim:** "KEEP MATCHING. Pin it as intended." He accepted the
stated cost: during an RPC or plugin failure Ze advertises to a peer whose role
is genuinely unknown.

So the behavior in the tree stands, and it stands as a DECISION rather than as
an accident nobody had looked at. `export { unknown }` matches a peer whose role
we could not learn exactly as it matches one that announced no role. Operator
intent is honored literally, and no working config changes.

**What the ruling settles, beyond the token.** `roleUnknown` becomes a TOTAL
answer over the destination-role state: "recorded and empty" and "never recorded"
are one input class, and `unknown` is the operator's name for that class. The
export-set membership test in `OTCEgressFilter` therefore evaluates a defined
input in every case, so its suppression is a policy decision — which is what
`forwardUpdateCore` already counts it as, and what `RelayStoredRoute` already
reports as a handled route. The R6-1 chain from an unrecorded role to a silently
"complete" replay is closed at its head.

`dropRoleUnrecorded` (AC-8) survives the ruling and keeps its meaning: it tells
an operator WHICH flavor of unknown suppressed the route, so a validate-open
failure is not read as a deliberate export-set exclusion. It no longer claims the
guard could not decide.

**Where the ruling is recorded, so it is not re-opened.**

| Surface | What it now says |
|---------|------------------|
| `TestExportSetUnrecordedStillMatchesExplicitUnknown` (`role/role_recorded_test.go`) | tests the DECISION and names the owner and the date. Its assertions are unchanged |
| `OTCEgressFilter`'s export-set block (`role/otc.go`) | the "open question" note is replaced by the ruling |
| `filterapi.EgressFilterFunc` (`filterapi/filterapi.go`) | the KNOWN GAP note no longer waits on this question |

**No RFC-tagged assertion moved.** `otc_test.go` and `config_test.go` are
untouched, so `.claude/hooks/pretool-writeedit.py`'s `rfc-test-change-approved:`
marker is not needed and was not used. The RFC 9234 rows in
`docs/features/rfc-status.md` keep the proof they had.

### AC-7 replaced — RULED BY THOMAS, 2026-08-03

**His ruling, verbatim:** "REPLACE AC-7 with the work the evidence supports."
He directed that AC-7 be rewritten "to fix the discards instead": same spec,
honest scope, closing a live defect rather than adding an unused channel.

**What it replaced.** AC-7 asked for a second return on
`filterapi.EgressFilterFunc` so an in-process egress filter could report "I could
not decide". That is not landing. The reassessment below re-derived it from all
three registered filters and found no filter has a state the new return would
express, and Q-1's ruling made `roleUnknown` a total answer, so OTC's suppression
is a policy decision rather than a failure. A widened signature would have had no
producer.

**What AC-7 now asks for, and why the evidence supports it.**
`safeEgressFilter` (`reactor/reactor_notify.go`) already returns
`(accept, panicked)`. The failure channel EXISTS. Of its three call sites,
`forwardUpdateCore` (`reactor_api_forward.go`) bound both and is the precedent;
`forward_rs.go` and `decideStaleReadvertise` (`reactor_api_batch.go`) each bound
`accept, _` and dropped the rest, so two of the three rails could not tell a
policy suppression from a filter that crashed, and both withheld the route
silently either way. Reading a return that is already produced fixes that. The
signature change would not have.

**What each rail now does with it, decided per site rather than copied.**

| Rail | What a panic means there | What it now does |
|------|--------------------------|------------------|
| `reactorForwardRS` (`forward_rs.go`) | The rail decided nothing for that destination, but the caller sets `ReactorForwarded` as soon as ANY other destination is dispatched, and bgp-rs then takes `default: releaseCache` (`rs/server_withdrawal.go`) and forwards to nobody. So the crash was indistinguishable from a policy suppression and cost that peer the route with no second rail and no accounting | The destination joins the `skipped` list, the existing channel for "this rail did not decide". bgp-rs forwards it via `batchForwardUpdateSkipped` -> `forwardUpdateCore`, which reads both returns and classifies the failure. A WARN names the destination, as every other declining path on this rail already does |
| `decideStaleReadvertise` (`reactor_api_batch.go`) | RFC 9494 Section 4.3 ("SHOULD NOT be advertised to peers that have not advertised the LLGR capability") and Section 4.6's NO_EXPORT + LOCAL_PREF=0 both depend on the destination's LLGR capability, which is exactly what the crashed filter was reading. So the route MUST stay withheld: re-advertising or depreferencing on a guess are both wrong | A new `staleFilterFailed` outcome. `sendStaleReadvertise` returns `(sent, failErr)` and `AnnounceNLRIBatch` reports it. It used to report `ErrNoPeersAcceptedFamily`, whose stated cause is untrue here (the family IS negotiated) and which `DispatchNLRIGroups` downgrades to a warning on the strength of that cause |

**The follow-up, same day: the OTHER failure branch of the same function.**
`decideStaleReadvertise` still returned `staleSuppress` when
`buildModifiedPayload` refused the Section 4.6 depreference. There the filter DID
decide and only realizing the decision failed, so it was the identical
conflation, four lines below the branch just fixed, and it also reached the
operator as `no peers have family negotiated`. Fixed here rather than deferred:
it makes Ze more correct, which needs no ruling, and half a function that still
confuses "could not" with "decided not to" leaves this AC's goal half met at that
site. Two adjacent branches came with it, `buildBatchAnnounceUpdate` and
`buildBatchWithdrawUpdate` returning nil, which now report the causes the
non-stale rail beside them has always used (`errAnnounceTooLarge`,
`errWithdrawTooLarge`) instead of a family mismatch. The producer-side counter is
untouched: `recordModifyFailureAddr` still names the build's own reason, and this
is the caller-facing half rather than a second copy.

**The rename.** `errStaleReadvertiseFilterFailed` became false once a build
failure shared it, so it is now `errStaleReadvertiseWithheld`, a base naming what
is true of both, wrapped by `errStaleReadvertiseFilterPanic` and
`errStaleReadvertiseBuildFailed`, which each name what actually happened. One
`errors.Is` on the base still catches the pair at the caller. `staleFailed`
became `staleFilterFailed` for the same reason, beside `staleBuildFailed`.

**Proof.** `internal/component/bgp/reactor/egress_filter_failure_test.go`. Every
test is a PAIR or a TRIPLE: each failure beside a filter that cleanly returns
false for the same destination. Mutation-verified by restoring each conflation in
turn. The panic half goes red at both rails, the build half goes red at the
decision helper AND at the entry point (reporting the failure as `no peers have
family negotiated`), and the reject control stays green in every case.
`TestDecideStaleReadvertiseWithholdsOnModifyFailure` (renamed from
`...SuppressesOnModifyFailure`) keeps its withheld-not-advertised assertion and
gains a stricter classification; nothing was weakened.

### AC-7 reassessment (2026-08-03) — the evidence behind the replacement

AC-7 asked for a second return on `filterapi.EgressFilterFunc` so an in-process
egress filter can report "I could not decide", and for `forwardUpdateCore` to
classify that as a drop rather than a policy suppression. Q-1's ruling removes
its motivating example, so the AC was re-derived from the producers rather than
inherited. Evidence, every claim read at its producer:

| Producer | What it does today | Does it need the second return? |
|----------|--------------------|---------------------------------|
| `OTCEgressFilter` (`role/otc.go`) | export-set branch tests membership of `roleUnknown`, which the ruling makes a defined input for both unrecorded and recorded-empty | No. The suppression is a decision, and `suppressedCount` is the correct classification |
| `LLGREgressFilter` (`gr/gr_egress.go`) | `egressState.Load() == nil` returns **true** — accept — before the plugin starts | No. It fails OPEN, and `forwardUpdateCore` reads `failed` only when `accept` is false, so a second return would change nothing until the filter first switched to denying. That switch is expressible in the CURRENT signature (`return false`) |
| `egressFilter` (`filter_community/filter_community.go`) | returns true unconditionally; `!hasCfg` means "no filter configured for this peer" | No. It never suppresses, so it has no suppression to qualify |

So no registered filter has a state that the second return would newly express.
What the seam did have is two rails that DISCARDED the failure channel it already
carries: `forward_rs.go` (the RS fanout) and `decideStaleReadvertise`
(`reactor_api_batch.go`) both called `safeEgressFilter` and dropped `panicked` on
the floor, so a filter panic there was reported as policy. Widening the signature
would not have fixed that; reading the existing return does.

**That finding is what Thomas acted on.** He replaced AC-7 with the read rather
than retiring it, so the AC is DONE in this spec rather than dropped
(`ai/rules/no-partial-completion.md` — owed, done, or retired by the owner, and
no fourth state). The `EgressFilterFunc` signature change is not owed by anyone:
`spec-fixit-egress-filter-non-decision-channel` kept only its LLGR nil-state item
and closed on 2026-08-10, and the RFC-tagged edits to `otc_test.go` and
`config_test.go` that the signature change would have dragged in are not needed.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| **An UNRECORDED destination role keeps matching an explicit `export { unknown }`. Thomas ruled this on 2026-08-03: "KEEP MATCHING. Pin it as intended."** | Make the unrecorded state a non-decision so it stops matching `unknown` and the route is suppressed fail-closed | Owner decision, recorded here rather than made here (`ai/rules/rfc-compliance.md`: the implementer records an owner ruling, never substitutes one). `unknown` is the operator's own token for "this peer's role is not known to us", and a peer whose OPEN we never recorded is in exactly that state, so honoring the token literally changes no working config. Thomas accepted the stated cost: during a validate-open RPC failure or a plugin respawn, Ze advertises to a peer whose role is genuinely unknown. The consequence for R6-1 is that `roleUnknown` is a TOTAL answer, so the export-set suppression is a policy decision and the relay's completeness count over it is correct |
| The unrecorded case is still counted and warned APART from an export-set decision (`dropRoleUnrecorded`, AC-8) | Fold it back into `dropExportSet` now that both are decisions | The two call for opposite operator actions. "Your export set excludes this peer" points at the config; "we never learned what this peer is" points at validate-open. The ruling makes them one INPUT class, not one diagnosis |
| A learned remote role survives a config reload for every peer the new config still names (AC-8b) | Keep wiping `filterRemoteRoles` on every `OnConfigure` | A learned role is a property of the SESSION: it is what the peer put in its OPEN, and an established peer sends no second OPEN. The wipe was not a clean slate but a lie, and it manufactured most of the unrecorded peers the ruling is about |
| `recordNoRemoteRole` WRITES an empty string instead of deleting the key | Keep deleting, and infer "declared none" from absence | Deleting made a decision and a miss share one representation. They stay distinguishable for the operator signal even though the ruling makes them behave alike at the export set |

## Known Limitations

Rows live in `plan/deferrals/fixit-stored-route-relay-hardening.md`; each names an
existing destination spec.

- ~~`LLGREgressFilter` accepts when its plugin state is not yet loaded
  (`s == nil`), the RFC 9494 fail-open twin of R6-1 and outside R6-1/R6-2's
  scope.~~ CLOSED 2026-08-10 by `spec-fixit-egress-filter-non-decision-channel`:
  an unloaded state now resolves to `hasLLGR=false`, so the destination takes the
  RFC 9494 Section 4.3 withdraw or the Section 4.6 depreference
  (`internal/component/bgp/plugins/gr/gr_egress.go`, `LLGREgressFilter`).
- RFC 2545 has no summary and no full text in the repository, so I-6's 32-byte
  next hop cannot be implemented against the RFC text.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The per-NLRI splitter the complex-family fix needs already exists for every family that reaches the complex path | It exists for EVPN, MVPN, MUP, MPLS-label and SR-Policy, and NOT for MPLS-VPN (SAFI 128) or flowspec (SAFI 133/134) -- the two the row names first. Every registration in the tree is `nlrisplit.init` (`nlrisplit/register.go`) plus `srpolicy/register.go`; `nlrisplit.Split` answers `ErrUnsupported` for the rest, while `isSimplePrefixFamily` (`adj_rib_in/nlri_hex.go`) routes them all down the complex path | Enumerating `nlrisplit.Register` call sites for the I-6 disposition, 2026-08-24 | The complex-family item grew again: its spec owes two RFC-conformant splitters (RFC 4364, RFC 8955) BEFORE the key type it was already blocked on. Two successive findings each under-sized it |
| The four inherited target tests can be re-run by the ids the spec carries | 372/378/394/395 are POSITIONAL ids in a suite that grew from 495 to 702, so they now resolve to four MCP and metrics tests. The names survive in exactly one place, `plan/deferrals/fixit-load-dependent-functional-failures.md`, and in neither the commit that fixed them nor the shard that discussed them | Listing the suite to run them, 2026-08-24 | The Completion checklist ran the wrong four tests if taken literally. The spec now names all four with a then/now id table, and the ids are never cited again |
| The legacy ingest path stores an ADD-PATH route as a path-id followed by the prefix, per `prefixToWireHex`, so the two paths carried two valid framings | The branch that actually runs for a forked adj-rib-in is the raw-section split. `splitRawNLRIHex` takes no add-path argument and reads the first path-id byte as a prefix length, so the stored bytes are misaligned and disagree with the key the same route was stored under. `prefixToWireHex` runs only when the event carries no raw section | Reading both branches of `handleReceived` and the format producer for the I-1 finding, 2026-08-14 | Lifting the refusal needs the legacy split fixed as well as the typed field, or a forked adj-rib-in replays the wrong prefix. The error is also in the `errRelayAddPath` doc comment, which must be corrected with the fix |
| Multi-path survives storage, so I-1's only question was the reconstruction context | The path-id reaches storage and stops at the RPC: `installStructuredNLRIs` puts it in `compactRouteKey` but neither `RawRoute` nor `rpc.StoredRoute` carries it | Reading the producers for the I-1 finding, 2026-08-03 | Option 2 (context-tagging) would announce one prefix N times with no path-id and lose all but one, so the choice is settled rather than open |
| R6-2's second half: a `buildModifiedPayload` failure falls through to `accept: true` | It does not. `runEgressPolicyChainASN4` reaches that branch only under `exportMods.Len() > 0 \|\| nlriOverride != nil`, which is exactly the negation of `buildModifiedPayload`'s one legitimate nil, so `modFail.failed()` catches every nil and returns `failed: true` | Re-verifying the finding's citations against today's tree | Half of R6-2 was already fixed; only the raw-override half was live |
| Refusing add-path was purely a safety improvement | The old rail replayed add-path routes correctly (collapsed to path-id 0) for single-path prefixes, so refusing removed working behaviour | Independent review traced `parseWireNLRISection` defaulting `addPath=false` and the iterator storing bare prefixes | The refusal must be recorded as an accepted interim regression, and lifting it is this spec's headline |
| "No peers accepted" means the relay failed | `forwardUpdateCore` returns `errNoEstablishedPeersToForwardTo` for a correctly egress-SUPPRESSED route too | Independent review, Run 2 | A "partial relay must fail" guard made one suppressed route fail a whole replay; fixed before shipping |

## Data Flow

### Entry Point
A peer establishes. Either bgp-rs dispatches `request bgp adj-rib-in replay <peer> <index>`
(`internal/component/bgp/plugins/rs/server_handlers.go`, `replayForPeer`), or — when no
plugin has claimed ownership — adj-rib-in's own peer-up handler fires
(`internal/component/bgp/plugins/adj_rib_in/rib.go`, `handleStructuredState` / `handleState`).

### Transformation Path
1. `buildReplayRoutes` selects stored routes for the target peer and attaches each one's
   SOURCE peer, yielding `[]rpc.StoredRoute` (hex attribute block, hex next-hop, hex NLRI).
2. `relayRoutes` chunks them and calls the engine's `relay-stored-route` RPC (DirectBridge
   for internal plugins, JSON framing for forked ones).
3. `RelayStoredRoute` resolves the destination and, per route, resolves the source peer's
   forwarding facts and receive encoding context.
4. `buildRelayUpdate` rebuilds a received-shape UPDATE body into a pooled buffer
   (`relay_payload.go`), registers it in the recent-UPDATE cache for buffer ownership, and
   hands it to `forwardUpdateCore` — the same egress transform a live forward uses.

**Where this spec intervenes:** step 4 currently REFUSES when the source negotiated
ADD-PATH, because step 1's stored NLRI framing does not record whether it carries a
path-id. Step 2's chunking bounds route count rather than bytes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| adj-rib-in plugin ↔ engine | `relay-stored-route` RPC, both DirectBridge and forked JSON | [ ] |
| engine ↔ session write | forward rail writes pre-filtered (no re-gate) | [ ] |
| plugin startup ↔ peer startup | post-startup fan-out completes before `StartPeers` | [ ] |

### Integration Points
- `forwardUpdateCore` (`internal/component/bgp/reactor/reactor_api_forward.go`) — the single
  egress transform; must stay single.
- `nlri.NLRIIterator` (`internal/core/bgp/nlri/iterator.go`) — where the path-id is dropped.
- `rpc.StoredRoute` (`pkg/plugin/rpc/types.go`) — the wire contract that may need a path-id field.
- `sendPostStartupToAll` (`internal/component/plugin/server/startup.go`) — the ordering that
  makes ownership deterministic.

## Wiring Test
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| peer-up replay from an ADD-PATH source | → | `buildRelayUpdate` add-path handling | `.ci` in `test/plugin/` asserting well-formed wire at an add-path destination (mutation-verified) |
| peer-up replay, `state` delivered to adj-rib-in but not bgp-rs | → | ownership scoping | `.ci` asserting the peer is still replayed exactly once |
| forked adj-rib-in, replay exceeding one IPC frame | → | byte-budget chunking in `relayRoutes` | unit test over the chunker + `.ci` with a large stored RIB |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRelayAddPathRoundTrip` | `internal/component/bgp/reactor/reactor_api_relay_test.go` | an add-path route reconstructs to wire the destination parses, for both add-path and non-add-path destinations | DONE 2026-08-19 (mutation-verified). It lives beside the fixture that builds the two peers rather than in `relay_payload_test.go`, which drives the byte builders with no reactor |
| `TestRelayMultiPathPreserved` | `internal/component/bgp/reactor/reactor_api_relay_test.go` | two path-ids for one prefix both survive, or the limit is explicit | DONE 2026-08-19 (mutation-verified). Both survive; there is no limit to state |
| `TestRelayAddPathZeroPathIDIsRelayed` | `internal/component/bgp/reactor/reactor_api_relay_test.go` | path-id 0 is a value, not an absence: the route reaches the destination and the four octets are written | DONE 2026-08-19 (mutation-verified). The boundary the framing marker exists for |
| `TestRelayStoredRouteRefusesUnrecordedFraming` | `internal/component/bgp/reactor/reactor_api_relay_test.go` | the narrowed refusal fires only for an unrecorded framing under an add-path source, and a non-add-path source still relays | DONE 2026-08-19 (mutation-verified). Replaces `TestRelayStoredRouteRefusesAddPathSource`, which asserted the behaviour this step removes |
| `TestSplitRawNLRIHexSkipsPathIdentifiers` | `internal/component/bgp/plugins/adj_rib_in/nlri_hex_test.go` | the legacy raw-section split reads the prefix length past the path-id, so stored bytes agree with the key | DONE 2026-08-19 (mutation-verified) |
| `TestPrefixToWireHexWritesBarePrefix` | same file | the text-prefix fallback writes RFC 4271 NLRI only, so a legal path-id 0 is not stored as an absence | DONE 2026-08-19 |
| `TestHandleReceivedAddPathStoresIdentifier` / `...Structured...` | same file | both ingest paths record `PathID` and `NLRIFraming` beside the bare prefix | DONE 2026-08-19 (mutation-verified) |
| `TestBuildReplayRoutesCarriesFraming` | same file | both fields reach `rpc.StoredRoute` | DONE 2026-08-19 |
| `TestRelayChunkStaysUnderFrameCeiling` | `internal/component/bgp/plugins/adj_rib_in/relay_chunk_test.go` | chunks stay under `rpc.MaxMessageSize` for large attribute blocks | DONE 2026-08-23 (mutation-verified). It lives in its own file beside the chunker, not in `rib_test.go`, because it drives a pure function over the SERIALIZED form |
| `TestRelayRouteJSONMaxBoundsMarshal` | same file | the per-route size estimate is an UPPER bound on `encoding/json`, for the widest form of every field | DONE 2026-08-23 (mutation-verified). This is what makes a field added to `rpc.StoredRoute` redden a test rather than lose a chunk |
| `TestRelayChunkCoversEveryRouteOnce` | same file | consecutive chunks partition the routes in order at budgets that cut on every boundary | DONE 2026-08-23 (mutation-verified) |
| `TestRelayChunkAlwaysAdvances` | same file | a route larger than the budget still forms a chunk, so the walk terminates | DONE 2026-08-23 (mutation-verified) |
| `TestReplayLastIndexSurvivesMultiChunkRelay` | same file | R-2: a multi-chunk replay reports one `last-index` over ALL its routes, and one `replayed` | DONE 2026-08-23 (mutation-verified) |
| `TestReplayCommandReportsRelayFailure` | same file | a relay failure surfaces as `statusError` carrying the cause and the destination | DONE 2026-08-23 |
| `TestRestartPluginReRunsTheStartupHandshake` | `internal/component/plugin/server/restart_test.go` | ownership re-established after respawn; no duplicate | DONE 2026-08-23 (mutation-verified). It lives at the ENGINE, not in `adj_rib_in/rib_test.go` as this row first said: what a respawn loses is the whole handshake, and the claim is one of six things it carries. Two real SDK plugins, one declaring the role and one standing down for it |
| `TestRestartPluginRefusesWhenRespawnIsNotEnabled` | same file | a restart that did not happen is reported as one that did not | DONE 2026-08-23 (mutation-verified) |
| `TestMidLifeAutoLoadDeliversPostStartup` | `internal/component/plugin/server/poststartup_test.go` | AC-12: a plugin auto-loaded by a reload runs its `OnAllPluginsReady` and reaches the plugin already holding the role | DONE 2026-08-23 (mutation-verified) |
| `TestCommandRegistryRegisterAfterFreezeIsVisible` / `...DeprecatedAliasAfterFreezeIsVisible` | `internal/component/plugin/server/command_registry_test.go` | a command or alias registered after `Freeze` is resolvable, which is every command of a restarted or reload-loaded plugin | DONE 2026-08-23 (mutation-verified) |
| `TestForwardZeroDispatchFailureIsADropNotASuppression` | `internal/component/bgp/reactor/forward_failure_verdict_test.go` | AC-10: the rebuild, the withdrawal conversion and the AS-path record each report a DROP when they leave the fan-out empty, and a clean policy refusal still reports a suppression | DONE 2026-08-23 (mutation-verified: all three branches made to count as suppression flip red, the control stays green) |
| `TestPolicyFilterFailedFlagMarksANonDecision` | `internal/component/bgp/reactor/filter_failed_flag_test.go` | AC-11: an IPC error under a fail-closed policy and an undeclared-attribute override both carry `Failed`, each beside the control that must not | DONE 2026-08-23 (mutation-verified) |
| `TestEgressChainCarriesTheFailedFlagToTheStep` | same file | AC-11's consequence: the flag reaches `egressStepResult.failed`, which is what AC-10's verdict reads | DONE 2026-08-23 (mutation-verified) |
| `TestUnheldRolesRetractsAClaimForAPeerTheClaimantIsNotFed` | `internal/component/plugin/server/startup_claims_test.go` | AC-4: an advertised role with no holder in the delivery set is named, and one whose claimant is fed is not | DONE 2026-08-23 (mutation-verified) |
| `TestUnheldRolesOverTheRealDeliverySet` | same file | AC-4 from the operator input that produces it: two peers, one attaching the claimant and one not, judged through `PeerScopedProcs` | DONE 2026-08-23 (mutation-verified). It drives the production path rather than a hand-built proc list, because the registry, the process table and the graph alone each answer the same for every peer |
| `TestSelfReplayCoversAPeerTheOwnerIsNotFed` | `internal/component/bgp/plugins/adj_rib_in/rib_claims_test.go` | AC-4 at the consumer: the retraction turns self-replay back on for that peer, in BOTH ingest paths, and an unrelated role changes nothing | DONE 2026-08-23 (mutation-verified) |
| `TestAppendStateChangeCarriesUnheldRoles` | `internal/component/bgp/format/text_test.go` | AC-4 on the JSON a forked plugin parses: the member is present, escaped, and absent when there is nothing to retract | DONE 2026-08-23 (mutation-verified) |
| `TestCoordinatorRelayStoredRoute` | `internal/component/plugin/coordinator_test.go` | `ErrNoReactor` branch, the delegation, and that the destination, routes and error all cross unchanged | DONE 2026-08-23 (mutation-verified twice) |
| `TestReactorForwardRSFilterPanicIsNotPolicy` | `internal/component/bgp/reactor/egress_filter_failure_test.go` | AC-7: a panicked filter puts the destination on the skipped list; a clean reject does not | DONE (mutation-verified) |
| `TestDecideStaleReadvertiseFailureIsNotPolicy` | same file | AC-7: `staleFilterFailed` for a panic, `staleBuildFailed` for an unbuildable modification, `staleSuppress` only for a reject | DONE (mutation-verified) |
| `TestAnnounceNLRIBatchStaleFailureIsNotFamilyMismatch` | same file | AC-7 at the entry point: each failure carries its own cause wrapping `errStaleReadvertiseWithheld` and is NOT `ErrNoPeersAcceptedFamily`; a reject still is | DONE (mutation-verified) |
| `TestDecideStaleReadvertiseWithholdsOnModifyFailure` | `internal/component/bgp/reactor/reactor_stale_readvertise_test.go` | the pre-existing withheld-not-advertised property, now pinned to `staleBuildFailed` | DONE (mutation-verified) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `adj-rib-in-replay-addpath-source.ci` | `test/plugin/` | a route learned from an add-path source is replayed to an add-path peer AND to one without, each receiving the framing it negotiated | DONE 2026-08-19 (mutation-verified both ways: refusing an add-path source, and omitting the four path-id octets, each redden it; baseline green). Carries the RFC7911-3-1 positive and negative tags and an RFC7911-2-2 positive, which is the first functional-tier evidence for 3-1 |
| `adj-rib-in-replay-on-peerup.ci` rewrite | `test/plugin/` | replay to an ESTABLISHED peer, asserted on wire bytes | DONE (mutation-verified, I-5) |
| `adj-rib-in-replay-unowned-peer.ci` | `test/plugin/` | a peer that gives `state` to bgp-adj-rib-in and attaches bgp-rs nowhere is replayed when it establishes | DONE 2026-08-23 (mutation-verified with a REBUILT daemon: `UnheldRoles` returning nil reddens it at the subject while the control, the store and the reconnect stay green). The bounce is driven by the peer's own script -- a marker prefix the observer injects only once the route is STORED completes the first connection, so the peer closes it and ze re-dials. No timer decides anything. Authored in `test/draft/plugin/` and promoted green, stable at 30/30 under `stress-repro.py --any-failure` |
| `adj-rib-in-replay-rfc2545-next-hop.ci` | `test/plugin/` | a route learned with an RFC 2545 form-2 next hop is replayed with both addresses, byte-identical to the live forward | DONE 2026-08-23 (mutation-verified with a REBUILT daemon: reverting the structured ingest reddens seq=2 while seq=1 stays green). Carries the RFC2545-3-1 and RFC2545-3-2 positives, the first functional-tier evidence for either over the relay rail |

## Files to Modify
- `internal/component/bgp/reactor/reactor_api_relay.go` — lift `errRelayAddPath`; context choice
- `internal/component/bgp/reactor/relay_payload.go` — path-id framing; RFC 2545 next hop
- `internal/component/bgp/plugins/adj_rib_in/rib.go` — storage framing, byte-budget chunking, ownership scope
- `pkg/plugin/rpc/types.go` — `StoredRoute` path-id field, if that is the chosen route
- `internal/component/plugin/server/startup.go` / `process/manager.go` — post-startup on respawn.
  Landed 2026-08-23 as: `restart.go` (new: `releasePluginRegistrations`,
  `runUnbarrieredStartupHandshake`, `restartHandshake`), `poststartup.go` (new: the
  fan-out moved out of `startup.go`, plus `sendPostStartupToNames`), `reload_tx.go`
  (`restartPlugin`), `process/manager.go` (`Respawn` returns the new process,
  `ErrRespawnNotEnabled`), `command_registry.go` (republish the frozen snapshot on
  add), `startup_autoload.go` (mid-life fan-out), `adhoc.go` (one spelling of the
  unbarriered handshake)
- `test/plugin/adj-rib-in-replay-on-peerup.ci` — make it gate
- The per-peer claim retraction (I-3, landed 2026-08-23): `plugin/server/startup_claims.go`
  (`UnheldRoles`, `advertisedClaimants`, `procsHoldRole`), `bgp/server/events.go`
  (compute once per state event), `pkg/plugin/rpc/bridge.go`
  (`StructuredEvent.UnheldRoles`), `bgp/format/text.go` + `text_json.go`
  (`unheld-roles`), `bgp/event.go` (`Event.UnheldRoles`), `adj_rib_in/rib_claims.go`
  (`replayDrivenElsewhere`), plus `docs/guide/plugins.md` and one point under
  `ai/rules/points/plugins/exclusive-role-claims-blocking-for-cross-plugin-default/`

### Documentation Update Checklist (BLOCKING)

Each file this spec edits carries a `// Design:` header naming its design
document. Every one is listed here with what this spec does to it.

| # | Design document | Declared by | Applies? |
|---|-----------------|-------------|----------|
| 1 | `docs/architecture/api/process-protocol.md` | `reactor/reactor_api_relay.go` | [ ] no. It documents the 5-stage handshake and the exclusive-role claim delivery. The relay's chunk size is a transport detail of one RPC, not a protocol stage, and no stage changed |
| 2 | `docs/architecture/api/ipc_protocol.md` | `pkg/plugin/rpc/types.go` | [ ] no. `rpc.MaxMessageSize` and the framing it governs are unchanged; the chunker stays UNDER the documented ceiling rather than moving it |
| 3 | `docs/architecture/plugin/rib-storage-design.md` | `adj_rib_in/rib.go` | [ ] **RESOLVED 2026-08-24: no.** The "maybe" was written on the assumption that this document describes `RawRoute`. It does not: the document is the bgp-rib plugin's Loc-RIB storage design (`NLRISet`, `DirectNLRISet`, `PooledNLRISet`, the attribute-keyed store), it names `RawRoute` nowhere, and its two next-hop passages are a design sketch of an `MPReach` interface whose `NextHop() []byte` returns the WHOLE field -- which is what `MPReachWire.NextHopBytes` does today, so the RFC 2545 fix moved the code TOWARD this document rather than away from it. Nothing here went stale |
| 4 | `docs/architecture/wire/messages.md` | `reactor/relay_payload.go` | [ ] no. The reconstruction's wire layout is unchanged; Step 3 touched only that file's TEST, to assert an equality the document already states |

## Implementation Steps

1. **Investigate I-1 first, and write the finding before any code.** Establish what the two
   ingest paths actually store (A-1, A-2), then choose normalisation vs context-tagging with
   evidence. Nothing else in this spec is blocked on it, so it goes first while context is fresh.
   **DONE 2026-08-14, see the I-1 FINDING and its part 2. A-1 confirmed, A-2 broken and
   re-verified, normalisation chosen, the refusal's replacement condition stated. One RFC 7911
   question (RFC7911-2-2, minting a path-id on re-advertisement) is open for Thomas and Step 2
   must not settle it by default.**
2. Lift the add-path refusal behind the chosen design; prove with the new `.ci` plus the four
   inherited target tests still green under `scripts/dev/stress-repro.py`.
   **DONE 2026-08-19 (AC-1, AC-2).** `rpc.NLRIFraming` plus `rpc.StoredRoute.PathID`
   carry the identifier and its presence (`pkg/plugin/rpc/types.go`); both ingest
   paths record them; `splitRawNLRIHex` takes the add-path flag and returns bare
   prefixes, and `prefixToWireHex` writes no identifier
   (`adj_rib_in/nlri_hex.go`); `buildRelayUpdate` writes the four octets whenever
   the SOURCE context declares ADD-PATH, identifier 0 included, and refuses only an
   unrecorded framing under such a source (`errRelayNLRIFraming`). One defect was
   fixed on the way, and it was in the LIVE rail as much as the relay:
   `forwardUpdateCore` rebuilt the wire after an egress modification without
   re-stamping the source, so ze's RFC 7911 Section 2 identifiers keyed under the
   singleton config source. Journal row in
   `plan/journal/helper-bypassed-by-an-open-coded-copy.md`.
3. Byte-budget chunking (I-2), with the `last-index` contract asserted across chunks.
   **DONE 2026-08-23 (AC-3).** `relay_chunk.go` carries the budget, the per-route
   upper bound and the walk; `relayChunkSize` is gone. Two I-6 items came with it
   because the chunking is what makes them reachable: the `routeRelayer` seam returns
   an error (it now wraps ONE chunk, so a test sees what the engine sees), and
   `relay_payload_test.go` asserts the writer's exact length rather than a bound. NO
   `.ci` was added: the user-visible path is already gated by
   `adj-rib-in-replay-on-peerup.ci` and `adj-rib-in-replay-addpath-source.ci`, both
   mutation-verified, and the byte bound itself is arithmetic over a serialized form
   that no `.ci` can reach without a 16 MB stored table
   (`ai/rules/testing.md`, "When Unit Tests Alone Are Sufficient", row 2).
4. Ownership scope and respawn (I-3, I-4).
   **DONE 2026-08-23 (AC-4, AC-5, AC-12).** The respawn half is fixed at the engine:
   a restart re-runs the 5-stage handshake, and a mid-life auto-load gets its
   post-startup callback. See the I-4 FINDING for what was actually missing, for the
   two defects fixed on the path, and for the unwired `respawn` trigger that a
   closure must not read as this spec's job. The scope half is fixed at the engine
   too: the claim stays daemon-wide and the engine retracts it per event for the
   peers its holder takes no delivery of. See the I-3 FINDING for why the peer was
   losing every route rather than only its replay.
5. Make `adj-rib-in-replay-on-peerup.ci` gate (I-5); mutation-verify.
6. The smaller gaps (I-6).
   **PART DONE. Two remain, and NEITHER is small — each carries a FINDING above
   that corrects the row it sits under.** The complex-family one is spec-sized: the
   stored KEY is fabricated for VPN/EVPN/flowspec, so a withdrawal removes nothing,
   and fixing it means changing `compactRouteKey`, which four other surfaces read.
   The backpressure one needs an owner decision rather than code: the queue is
   unbounded by design and a parked item pins no read buffer, so what is missing is
   a producer-side wait in the replay, and what it waits on is the question.

   **DISPOSITION 2026-08-24 (Phase 3). Both re-verified at their producers, and
   NEITHER lands in this spec.** Each is separable future work by the test in
   `ai/rules/rule-precedence.md`: this spec's goal is that a relayed route take
   exactly one egress transform and that a peer-up replay reach its destination,
   and both hold today for every family the relay can key. Neither item blocks it.

   | Item | Verified | Why it is not fixed here | What it needs next |
   |------|----------|--------------------------|--------------------|
   | Complex families | `wireNLRIsToAny` (`adj_rib_in/nlri_hex.go`) walks `[prefix-len][address]` and `installComplexNLRIs` (`rib.go`) keys on its output, so the key is fabricated exactly as the 2026-08-23 finding says. NEW: the splitter registry covers neither MPLS-VPN nor flowspec — see the correction under the row | Its own spec. It owes two RFC-conformant splitters (RFC 4364, RFC 8955), THEN the `compactRouteKey` change, THEN per-NLRI storage. A half fix collides two NLRIs on one bogus key and is worse than today | a spec, then Thomas's word on whether it runs |
   | Backpressure | `DispatchOverflow` (`reactor/forward_pool.go`) still owns the bodies before queueing and still states "routes never dropped"; `RelayStoredRoute` still reads no queue depth | It is a design question, not a defect with a known fix. Writing either answer without his ruling picks which cost the daemon pays | Thomas's answer: producer-side wait, or fail fast at a depth threshold |

   → Both are reported to the owner rather than recorded and left. Recording is
     not fixing (`ai/rules/completion.md`); what makes this a report rather than
     a park is that neither is this spec's to close and both have a named next
     step with an owner.
7. `make ze-precommit-verify`, `make ze-unit-reactor-test-race`, per-test stress-repro; independent review to clean.

## Checklist
### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
### Completion (BLOCKING — before ANY commit)
- [ ] Every AC has working code + test
- [ ] `make ze-functional-plugin-test` green whole, and non-reproducing under stress
      (this replaces "372 / 378 / 394 / 395": the ids drifted, see Current Behavior)
- [ ] `make ze-unit-reactor-test-race` green
- [ ] `make ze-standard-test` passes
- [ ] Independent review clean
- [ ] Learned summary written

## Checklists this spec never carried (authored 2026-08-24 at closure)

The spec was written without a Critical Review Checklist, a Deliverables
Checklist or a Security Review Checklist. That is an AUTHORING gap, not template
drift: `plan/TEMPLATE.md` still carries all three, under its Implementation Steps
section, and `plan/TEMPLATE-CLOSURE.md` never did. The closure agent authored
them from the finished work rather than stopping, because the three tables are
the FORMAT of steps 1, 2 and 5 of `/ze-close` and those steps ran either way.
The main thread owns the question of whether authoring-at-closure is acceptable
here or whether the spec should have been sent back.

### Critical Review Checklist

| Check | What to verify for this spec | Result |
|-------|------------------------------|--------|
| Completeness | Every AC-N has an implementation at a named producer | done, see the Implementation Audit below |
| Correctness | The relayed copy is byte-identical to the live forward for the same destination | `adj-rib-in-replay-addpath-source.ci` and `adj-rib-in-replay-rfc2545-next-hop.ci` each assert the same hex twice, seq=1 the live forward and seq=2 the replay |
| One egress transform | No second rail was introduced while lifting the ADD-PATH refusal | `buildRelayUpdate` still hands its reconstruction to `forwardUpdateCore`; the framing decision is made once, and `fwdReencodeNLRIs` converts per destination as it does for a live forward |
| Guard direction | Every guard this spec adds or narrows denies on its miss path | `errRelayNLRIFraming` refuses an unrecorded framing under an ADD-PATH source; `decodeFilterRawOverride`'s malformed answer drops on ingress and suppresses with `failed: true` on egress; `filterAPI()` returns a true nil interface so no caller reads a typed nil as "the filter engine is present" |
| Naming | The failure tokens name what happened, not what they were first written for | `errStaleReadvertiseFilterPanic` and `errStaleReadvertiseBuildFailed` wrap `errStaleReadvertiseWithheld`; `staleFilterFailed` sits beside `staleBuildFailed` |
| Data flow | The path identifier travels as a VALUE beside the bytes, never inside them | `RawRoute.PathID` plus `NLRIFraming`, `rpc.StoredRoute` carrying both, and `prefixToWireHex` writing no identifier at all |
| Rule: `ai/rules/evidence.md` | A zero value is never a valid-looking answer on a cross-process boundary | `NLRIFramingUnrecorded` is the zero value and it REFUSES; a bare `PathID uint32` would have read an omitted JSON number as the legal identifier 0 |
| Rule: `ai/rules/no-layering.md` | X is deleted before Y is implemented | `relayChunkSize` is gone rather than kept beside `relayChunkEnd`; `clearFilterRemoteRole` became `recordNoRemoteRole` rather than gaining a sibling |

### Deliverables Checklist

| Deliverable | Verification method | Result |
|-------------|---------------------|--------|
| `rpc.StoredRoute` carries the path identifier and its framing | grep `PathID` and `NLRIFraming` in `pkg/plugin/rpc/types.go` | both fields plus the three-valued `NLRIFraming` enum with `MarshalText` and `UnmarshalText` |
| The ADD-PATH refusal is narrowed, not lifted blindly | grep `errRelayNLRIFraming` in `reactor/reactor_api_relay.go` | one sentinel, one `default:` arm reached only under an ADD-PATH source with an unrecorded framing |
| The chunker bounds BYTES | `ls` on `adj_rib_in/relay_chunk.go`, and grep `relayChunkSize` over `adj_rib_in/` | file present at 3993 bytes; zero hits for the deleted constant |
| The claim is retracted per event | grep `UnheldRoles` in `plugin/server/startup_claims.go`, `bgp/server/events.go` and `adj_rib_in/rib_claims.go` | producer, carrier and consumer each present |
| A restart re-runs the handshake | `ls` on `plugin/server/restart.go` | present at 4816 bytes; `restartHandshake` called from `restartPlugin` |
| Four `.ci` tests gate the user-visible path | `ls test/plugin/adj-rib-in-replay-*.ci` | four files, 12174 to 15695 bytes |
| The 39 tests the TDD plan names all pass | `go test -v` over the 39, counting `--- PASS` rather than trusting `ok` | 39 PASS, 0 FAIL, 0 SKIP |

### Security Review Checklist

| Check | What to look for | Result |
|-------|------------------|--------|
| Untrusted length octet | `NextHopBytes` reads a peer-supplied length and slices on it | bounds-checked: a declared length longer than the attribute returns nil, and the result is a view rather than a copy, so no allocation is sized from the wire |
| Untrusted prefix length | `splitRawNLRIHex` reads a peer-supplied prefix-length octet | an entry that runs past the end ends the walk, and the loop advances by at least the length byte each iteration, so it terminates on any input |
| Unbounded allocation | any `make()` sized from peer or plugin data | none added. `buildRelayUpdate` decodes into ONE pooled read buffer and refuses a scratch length above the buffer with `errRelayTooLarge` before writing |
| Frame-size denial of service | a plugin-supplied replay large enough to be refused whole | that IS the defect this spec fixed: `relayChunkEnd` cuts on the serialized byte budget, so a large stored table is chunked instead of lost |
| Injection into a JSON envelope | a token from an external plugin reaching an event's JSON | `appendUnheldRolesJSON` escapes each role through `appendJSONString`; a claim token comes from a forked plugin's Stage-1 RPC, so a quote in one would otherwise break the framing of every state event |
| Guard that fails open | a miss, error or empty path returning the permissive value | none. Each guard's miss path is named in the Critical Review Checklist above, and each denies |
| Information leakage | the new WARN and Debug lines | they name a peer address, a destination and a role token, all of which the operator already sees in `show bgp` output. No key material, no payload bytes |
| Cross-process contract | an older forked plugin talking to a newer engine | it omits `nlri-framing`, which decodes to `NLRIFramingUnrecorded`, which REFUSES under an ADD-PATH source. A newer plugin talking to an older engine loses the field, and the older engine keeps its blanket refusal |

## Implementation Summary

### What Was Implemented

- **AC-1, AC-2 (2026-08-19).** The relay stopped refusing an ADD-PATH source. The
  identifier travels as a VALUE beside the stored bytes: `RawRoute.PathID` plus
  `NLRIFraming`, carried to the engine on `rpc.StoredRoute`, and
  `buildRelayUpdate` (`internal/component/bgp/reactor/reactor_api_relay.go`)
  writes the four octets whenever the SOURCE context declares ADD-PATH,
  identifier 0 included. Two storage defects were fixed with it:
  `splitRawNLRIHex` read the first octet of a path identifier as a prefix length,
  and `prefixToWireHex` wrote the identifier only when it was non-zero.
- **AC-3 (2026-08-23).** `relayChunkEnd` (`adj_rib_in/relay_chunk.go`) cuts a
  replay on the SERIALIZED byte budget. `relayChunkSize` is deleted.
- **AC-4 (2026-08-23).** A daemon-wide claim is RETRACTED per event for the peers
  its holder is not fed: `Server.UnheldRoles`, the `unheld-roles` JSON member and
  `StructuredEvent.UnheldRoles`, read by `replayDrivenElsewhere`.
- **AC-5, AC-12 (2026-08-23).** A restarted plugin re-runs the whole 5-stage
  handshake (`restart.go`), and a plugin auto-loaded MID-LIFE by a config reload
  receives the post-startup callback (`sendPostStartupToNames`, `poststartup.go`).
- **AC-6 (2026-08-07).** `adj-rib-in-replay-on-peerup.ci` replays to an
  ESTABLISHED second peer and asserts exact wire bytes twice.
- **AC-7, AC-8, AC-8b, AC-9, Q-1 (2026-08-03).** Thomas ruled that an unrecorded
  destination role keeps matching an explicit `export { unknown }`, and replaced
  AC-7 with reading the failure return `safeEgressFilter` already produced. See
  the AC-7 sections above.
- **AC-10, AC-11 (2026-08-23).** The zero-dispatch failure branches and the two
  non-decided `PolicyReject` producers are driven by tests, each beside a control.
- **I-6 RFC 2545 (2026-08-23).** `MPReachWire.NextHopBytes` returns the whole
  next-hop field and `NextHop` reads it, so a stored route keeps the link-local
  half of a Section 3 pair.

### Bugs Found/Fixed

Each was walked into while implementing, and each carries a journal row.

- `forwardUpdateCore` rebuilt the wire after an egress modification without
  re-stamping the source, so ze's own RFC 7911 identifiers keyed under the
  singleton config source. `plan/journal/helper-bypassed-by-an-open-coded-copy.md`.
- `CommandRegistry.Register` wrote the mutable map without republishing the frozen
  snapshot, so every command of a restarted or reload-loaded plugin was invisible
  to `Lookup`. `Unregister` had always republished.
  `plan/journal/guard-added-to-one-half-of-a-pair.md`.
- `ProcessManager.Respawn` returned a bare `nil` when respawn was not enabled,
  indistinguishable from a completed restart. It returns `ErrRespawnNotEnabled`.
- The respawn TRIGGER is unreachable in a shipped daemon: nothing outside a test
  sets `RespawnEnabled` or `Respawn`. `plan/journal/unwired-feature.md`. Wiring the
  operator-facing option is a FEATURE and is not this spec's.
- `setFilterState` wiped `filterRemoteRoles` on every reload, so a source
  configured `role { export customer }` silently stopped advertising to its
  customers until each session bounced (AC-8b).
- `RawRoute.NHopHex`'s own doc still described a single decoded address after the
  RFC 2545 fix made it the whole field. Found by this closure's review; fixed
  2026-08-24.

### Documentation Updates

- `docs/guide/plugins.md` gained the per-event claim retraction, with the anchor
  `<!-- source: internal/component/plugin/server/startup_claims.go -- (*Server).UnheldRoles -->`.
- `ai/rules/plugins.md` plus a new point file under
  `ai/rules/points/plugins/exclusive-role-claims-blocking-for-cross-plugin-default/`
  say a plugin that stood a role down MUST run its default behaviour for an event
  that names the role.
- `docs/architecture/api/ipc_protocol.md` and `docs/architecture/meta/role.md`
  were updated by the AC-7 commit.
- `rfc/requirements/rfc2545.md` and `rfc/audit/rfc7606.json` were updated by the
  RFC 2545 commit.
- Nothing else was owed. The four design documents the changed files declare are
  answered one by one in the Documentation Update Checklist above, and the
  `rib-storage-design.md` "maybe" was RESOLVED to "no" against the document
  itself: grep counts zero occurrences of `RawRoute` in it.
- `rpc.StoredRoute`'s two new JSON fields need no doc: grep for `StoredRoute`
  over `docs/` finds only the METHOD name in four documents, never a field list.
- `make ze-doc-links-check`: 24 findings, the same 24 before and after this
  closure's edits, none on a file this spec touches.

### Deviations from Plan

- Step 6 (I-6) is PART done by design. The complex-family and backpressure items
  were re-verified at their producers on 2026-08-24 and dispositioned as separable
  future work; see the DISPOSITION table under Implementation Steps. Neither
  blocks this spec's goal.
- `make ze-unit-reactor-test-race` has NO VERDICT. It cannot pass on this
  hardware: one iteration of `./internal/component/bgp/reactor/...` under `-race`
  measured 161.127s here, so `-count=20` needs about 54 minutes against a
  `GO_TEST_TIMEOUT` of 20m, and `scripts/dev/ze-run.sh`'s stall watchdog reaps the
  holder besides, because the target runs `go test` without `-v` and its log
  cannot grow. Both are recorded in
  `plan/journal/gate-verdict-depends-on-the-machine.md`. Substitute evidence is in
  Goal Validation and is LABELLED as substitute.
- AC-7 was REPLACED by Thomas on 2026-08-03 rather than implemented as written,
  and Q-1 was RULED by him the same day. Both are recorded in full above.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| I-1: an ADD-PATH source becomes replayable | Done | `buildRelayUpdate` (`reactor/reactor_api_relay.go`), `rpc.StoredRoute` (`pkg/plugin/rpc/types.go`) | normalisation chosen over context-tagging; the finding is above |
| I-1b: ownership ordering is deterministic | Done | `advertiseClaims` at Stage 2 (`plugin/server/startup_claims.go`) | the declarative route, not the wait that deadlocks |
| I-2: chunk by bytes | Done | `relayChunkEnd` (`adj_rib_in/relay_chunk.go`) | |
| I-3: ownership scoped to the peers the owner drives | Done | `Server.UnheldRoles`, `replayDrivenElsewhere` | the fix is a retraction on the event |
| I-4: the claim survives a respawn | Done | `Server.restartPlugin`, `restartHandshake` (`plugin/server/restart.go`) | the claim was one of six things a respawn lost |
| I-5: `adj-rib-in-replay-on-peerup.ci` gates | Done | `test/plugin/adj-rib-in-replay-on-peerup.ci` | mutation-verified with `return nil` first in `RelayStoredRoute` |
| I-6: smaller relay gaps | Partial | RFC 2545 and the two test gaps done; complex families and backpressure dispositioned | owner-approved by the DISPOSITION under Implementation Steps: separable future work, neither blocks the goal |
| R6-1: an egress filter can report "I could not decide" | Changed | `safeEgressFilter`'s three call sites | Thomas replaced the signature change with reading the existing return, 2026-08-03 |
| R6-2: a non-decision ACCEPT in the export policy chain | Done | `decodeFilterRawOverride` (`reactor/filter_chain.go`) | fails closed on both rails |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRelayAddPathRoundTrip`, `test/plugin/adj-rib-in-replay-addpath-source.ci` | both destination kinds in one run |
| AC-2 | Done | `TestRelayMultiPathPreserved`, `TestRelayAddPathZeroPathIDIsRelayed` | both paths survive; there is no limitation to state |
| AC-3 | Done | `TestRelayChunkStaysUnderFrameCeiling`, `TestReplayLastIndexSurvivesMultiChunkRelay` | R-2 pinned across a chunk boundary |
| AC-4 | Done | `test/plugin/adj-rib-in-replay-unowned-peer.ci` plus four unit tests | mutation-verified in three registers |
| AC-5 | Done | `TestRestartPluginReRunsTheStartupHandshake`, `TestRestartPluginRefusesWhenRespawnIsNotEnabled` | trigger unwired; see the I-4 FINDING |
| AC-6 | Done | `test/plugin/adj-rib-in-replay-on-peerup.ci` | RED with the relay dead, GREEN restored |
| AC-7 | Done | `TestReactorForwardRSFilterPanicIsNotPolicy`, `TestDecideStaleReadvertiseFailureIsNotPolicy`, `TestAnnounceNLRIBatchStaleFailureIsNotFamilyMismatch` | replaced by owner ruling, then implemented |
| AC-8 | Done | `dropRoleUnrecorded` (`role/metrics.go`), `remoteRoleRecorded` (`role/role.go`) | its own counter and a first-occurrence WARN |
| AC-8b | Done | `setFilterState` (`role/role.go`) | learned roles survive a reload for every peer still named |
| AC-9 | Done | `decodeFilterRawOverride` and both callers in `reactor/filter_ordered.go` | drop on ingress, suppress with `failed: true` on egress |
| AC-10 | Done | `TestForwardZeroDispatchFailureIsADropNotASuppression` | four cases, three subjects and one control |
| AC-11 | Done | `TestPolicyFilterFailedFlagMarksANonDecision`, `TestEgressChainCarriesTheFailedFlagToTheStep` | `filterTransport` seam, nil in production |
| AC-12 | Done | `TestMidLifeAutoLoadDeliversPostStartup` | pre-fix producer leaves it "Condition never satisfied" after 5s |

### Tests from TDD Plan

| Test group | Status | Location | Notes |
|------------|--------|----------|-------|
| Relay reconstruction, 4 tests | Done | `reactor/reactor_api_relay_test.go` | AC-1, AC-2, and the narrowed refusal |
| Storage framing, 4 tests | Done | `adj_rib_in/nlri_hex_test.go` | both ingest paths, plus the bare-prefix fallback |
| Chunker, 6 tests | Done | `adj_rib_in/relay_chunk_test.go` | includes the `relayRouteJSONMax` upper-bound proof |
| Plugin restart and mid-life auto-load, 5 tests | Done | `plugin/server/restart_test.go`, `poststartup_test.go`, `command_registry_test.go` | AC-5, AC-12 |
| Forward verdict and filter flag, 3 tests | Done | `reactor/forward_failure_verdict_test.go`, `filter_failed_flag_test.go` | AC-10, AC-11 |
| Claim retraction, 4 tests | Done | `plugin/server/startup_claims_test.go`, `adj_rib_in/rib_claims_test.go`, `format/text_test.go` | AC-4, producer, carrier and consumer |
| Coordinator delegation, 1 test | Done | `plugin/coordinator_test.go` | both branches |
| Egress failure classification, 4 tests | Done | `reactor/egress_filter_failure_test.go`, `reactor_stale_readvertise_test.go` | AC-7 |
| RFC 2545, 6 tests | Done | `wireu/mpwire_test.go`, `adj_rib_in/nlri_hex_test.go`, `reactor/relay_payload_test.go` | ingest, derivation and writer |
| Owner ruling, 1 test | Done | `role/role_recorded_test.go` | Q-1 |
| Functional, 4 `.ci` | Done | `test/plugin/adj-rib-in-replay-*.ci` | each mutation-verified with a REBUILT daemon |
| **Total** | **39 unit + 4 functional** | | 39 top-level PASS, 0 FAIL, 0 SKIP, counted under `-v` |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `reactor/reactor_api_relay.go` | Done | refusal narrowed to `errRelayNLRIFraming` |
| `reactor/relay_payload.go` | Done | `relayPathIDLen`; the writer already sized from `len(nextHop)` |
| `adj_rib_in/rib.go` | Done | storage framing and ownership; the hex helpers moved to `nlri_hex.go` and the chunker to `relay_chunk.go` |
| `pkg/plugin/rpc/types.go` | Done | `PathID` plus the `NLRIFraming` enum |
| `plugin/server/startup.go`, `process/manager.go` | Changed | landed as `restart.go`, `poststartup.go`, `reload_tx.go`, `command_registry.go`, `startup_autoload.go`, `adhoc.go` -- see the Files to Modify note above |
| `test/plugin/adj-rib-in-replay-on-peerup.ci` | Done | rewritten to gate |
| `wireu/mpwire.go` | Changed | not in the original list; the RFC 2545 truncation is here, not in `nhopHexFromAddr` |

### Audit Summary

- **Total items:** 9 requirements, 12 acceptance criteria, 43 tests, 7 file groups.
- **Done:** 8 requirements, 12 ACs, 43 tests, 5 file groups.
- **Partial:** 1 (I-6, owner-approved by the DISPOSITION under Implementation Steps).
- **Skipped:** 0.
- **Changed:** R6-1 (AC-7, replaced by owner ruling 2026-08-03) and 2 file groups
  (recorded in Deviations).

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A relayed route takes exactly ONE egress transform, so the replayed copy is what a live forward would have sent | functional | `adj-rib-in-replay-addpath-source.ci` asserts the SAME hex at seq=1 (live forward) and seq=2 (replay) for two destinations that negotiated differently: the ADD-PATH peer receives `00000000180A0000` and the one without receives `180A0000`. `adj-rib-in-replay-rfc2545-next-hop.ci` does the same for an 87-byte UPDATE carrying a 32-octet next-hop pair under a Length octet of 0x20 |
| A peer-up replay reaches its destination, whatever the source negotiated | functional | the refusal is gone from the source axis: `adj-rib-in-replay-addpath-source.ci` mutation-verified twice, once by restoring the blanket refusal and once by omitting the four identifier octets, each reddening seq=2 while seq=1 stays green |
| A peer nobody drives is replayed by somebody | functional | `adj-rib-in-replay-unowned-peer.ci`, mutation-verified with a REBUILT daemon: `UnheldRoles` returning nil reddens it at the subject while the control, the store and the reconnect stay green. Stable at 30/30 under `stress-repro.py --any-failure` |
| A large stored table is chunked rather than lost | unit, on the serialized form | `TestRelayChunkStaysUnderFrameCeiling` marshals every chunk of a 130-route 64 KiB-attribute replay. Mutation-verified: restoring the 4096-route cut puts chunk 0 at 17 053 828 bytes, above `rpc.MaxMessageSize`, and reddens three tests |
| A failure of THIS speaker is never reported as a peer's policy | unit, paired | every test in `egress_filter_failure_test.go`, `forward_failure_verdict_test.go` and `filter_failed_flag_test.go` is a PAIR or a TRIPLE: each failure beside a filter that cleanly refuses the same destination, so the negative assertions cannot be vacuous |
| RFC 7911 Section 3 conformance over the relay rail | RFC gate plus functional tags | `make ze-rfc-check` GREEN: 2966 gated MUST-level requirements across 171 enrolled RFCs, 3595 tags resolved, all eight ratchets satisfied. `adj-rib-in-replay-addpath-source.ci` carries the RFC7911-3-1 positive AND negative, the first functional-tier evidence for that requirement |
| RFC 2545 Section 3 conformance over the relay rail | RFC gate plus functional tags | same gate run; `adj-rib-in-replay-rfc2545-next-hop.ci` carries RFC2545-3-1 and RFC2545-3-2 positives, the first functional-tier evidence for either over this rail |
| No data race under the reactor's stress gate | **SUBSTITUTE evidence, labelled** | `make ze-unit-reactor-test-race` HAS NO VERDICT and cannot get one on this hardware, for two composing reasons neither of which is this spec's: one race iteration of `./internal/component/bgp/reactor/...` measured 161.127s here, so `-count=20` needs about 54 minutes against a `GO_TEST_TIMEOUT` of 20m; and `_ze-unit-reactor-test-race-impl` runs `go test` without `-v`, so its log cannot grow and `ze-run.sh`'s stall watchdog reaps it. Both are in `plan/journal/gate-verdict-depends-on-the-machine.md`. What WAS run: the 39 tests this spec names under `-race -count=20` across 7 packages, 780 executions, 0 data races and 0 failures in 2m03s; and the WHOLE reactor tree under `-race -count=1`, 0 data races, 161.127s. That is narrower than the gate and MUST NOT be read as the gate passing |
| The whole plugin suite stays green | functional, one unrelated red | 660/661 with 41 skipped, 32-way parallel. The one red is `test/plugin/plugin-reads-engine-answer.ci`, which has zero references to adj-rib-in, replay or relay, passes standalone in 3.6s, and failed only under load with `answer queue full: consumer fell behind`. Journal row in `plan/journal/gate-fires-outside-its-population.md` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `LLGREgressFilter` accepts on a nil egress state (RFC 9494 fail-open twin of R6-1) | done | closed 2026-08-10 by `spec-fixit-egress-filter-non-decision-channel`; an unloaded state now resolves to `hasLLGR=false` |
| Two of `safeEgressFilter`'s three call sites discard `panicked` | done | not deferred after all: Thomas replaced AC-7 with this work on 2026-08-03 and it landed here |
| `decideStaleReadvertise` returns `staleSuppress` on its build-failure branch | done | fixed here the same day; it now returns `staleBuildFailed` |
| RFC 2545 has no summary and no full text in the repository | done | closed 2026-08-07 by `spec-followup-rfc-enrollment`: text fetched, summary written, enrolled, extraction signed off at register `prose`, status row rewritten |
| **Shard verdict** | **all four terminal** | `plan/deferrals/fixit-stored-route-relay-hardening.md` is removed by the closure commit |

Eight rows of the FOREIGN shard `plan/deferrals/fixit-bgp-egress-rail-divergence.md`
name this spec as their destination. Seven are now `done` with their landing
evidence written into the row. The eighth, "four smaller relay gaps", stays LIVE:
two of its four landed (RFC 2545, the `Coordinator.RelayStoredRoute` test) and two
did not (complex families, backpressure). That shard therefore is NOT residue and
MUST NOT be removed. Its live row now names what each remaining half needs, and
both are put to Thomas in this closure's report.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-stored-route-relay-hardening-9ad8358c-695f-41be-8019-5d92ba08f8e6.md` |
| `review_gate.py check` | OK: 63 code files, clean, hashes match |
| Rounds | 2. Round 1 read the whole changeset of the seven implementation commits and found 1 BLOCKER, 1 ISSUE and 3 NOTEs. Round 2 read only the fixes and found nothing |
| Reviewer lenses used | one closure agent applying every `/ze-review` lens in one context: automated pre-checks, size, wiring, functional-test coverage, documentation drift, history and removed-behaviour, comments and invariants, data flow, edge cases, security, allocation, logic and guard audit, performance, plugin and config surface, altitude and simplicity, project rules and the ze-style pass, interop and goal validation, RFC compliance |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | Commit B's `git rm` of this spec turns `make ze-doc-links-check` red on four backticked citations in tracked files that survive the closure. Check 5 of `scripts/dev/check_doc_links.py` reports a dead path reference in ANY tracked file, and `check_baseline_growth` refuses adding the pairs to `scripts/dev/doc_citation_baseline.txt` | `plan/deferrals/fixit-load-dependent-functional-failures.md`, `plan/known-failures/README.md`, and `plan/known-failures/bgp-plugin-rs-forward-duplicate-and-order.md` (two citations there) | each restated with the bare stem `spec-fixit-stored-route-relay-hardening`, which the checker's path grammar ignores. The gate still reports the same 24 findings it reported before, none on this spec's files |
| 2 | ISSUE | `RawRoute.NHopHex`'s doc still said "the next-hop IP converted to wire hex" after the RFC 2545 fix made it the WHOLE MP_REACH next-hop field: 32 octets for a Section 3 pair, RD plus address for a VPN family. `ai/rules/stale-comments.md` | `adj_rib_in/rib.go`, the `RawRoute` doc block and the `NHopHex` field comment | rewritten to state the whole-field contract, its 4-to-32-octet range, its producer and the one case that still stores a single address |

Three NOTEs were recorded and did not block: a rename that
`audit-test-relaxation.py` reads as a deletion (the assertions are all still
there and one was added), the process-global coordinator nil inside
`runUnbarrieredStartupHandshake` (a verbatim extraction whose new caller is
unreachable in a shipped daemon), and the per-event goroutine in
`sendPostStartupTo` (a verbatim move). Full text in the artifact.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/adj-rib-in-replay-addpath-source.ci` | yes | `ls -l` 15695 bytes |
| `test/plugin/adj-rib-in-replay-on-peerup.ci` | yes | `ls -l` 13811 bytes |
| `test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci` | yes | `ls -l` 13467 bytes |
| `test/plugin/adj-rib-in-replay-unowned-peer.ci` | yes | `ls -l` 12174 bytes |
| `internal/component/bgp/plugins/adj_rib_in/relay_chunk.go` | yes | `ls -l` 3993 bytes |
| `internal/component/bgp/plugins/adj_rib_in/nlri_hex.go` | yes | `ls -l` 8605 bytes |
| `internal/component/bgp/plugins/adj_rib_in/rib_claims.go` | yes | `ls -l` 4057 bytes |
| `internal/component/plugin/server/poststartup.go` | yes | `ls -l` 4603 bytes |
| `internal/component/plugin/server/restart.go` | yes | `ls -l` 4816 bytes |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | an ADD-PATH source relays, and multi-path survives | `go test -v` over `TestRelayAddPathRoundTrip`, `TestRelayMultiPathPreserved`, `TestRelayAddPathZeroPathIDIsRelayed` and `TestRelayStoredRouteRefusesUnrecordedFraming` in `./internal/component/bgp/reactor/`: 4 `--- PASS`, 0 FAIL |
| AC-3 | the chunker bounds bytes and `last-index` survives | same run over `./internal/component/bgp/plugins/adj_rib_in/`: 6 `--- PASS` including `TestReplayLastIndexSurvivesMultiChunkRelay` |
| AC-4 | the claim is retracted per event, producer to consumer | 4 `--- PASS` across `plugin/server`, `adj_rib_in` and `format` |
| AC-5, AC-12 | a restart re-runs the handshake; a mid-life auto-load gets its callback | 3 `--- PASS` in `plugin/server` |
| AC-6 | the peer-up `.ci` gates | `ls` above, and the mutation record in the I-5 section |
| AC-7, AC-9 | a speaker-side failure is never reported as policy | 4 `--- PASS` in `reactor` |
| AC-8, AC-8b, Q-1 | the unrecorded role is diagnosed apart and survives a reload | 1 `--- PASS` in `role` |
| AC-10, AC-11 | the zero-dispatch and non-decided producers are driven | 3 `--- PASS` in `reactor` |
| **All 12** | every AC has working code and a passing test | one run of the 39 named tests, `-v`, `-count=1`, 7 packages: **39 `--- PASS`, 0 `--- FAIL`, 0 `--- SKIP`**. Counted rather than read off `ok`, because a `-run` filter that matches nothing also prints `ok` |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| peer-up replay from an ADD-PATH source | `test/plugin/adj-rib-in-replay-addpath-source.ci` | read, not inferred: six `expect=bgp` lines, two destinations, seq=1 and seq=2 asserted to the same hex at each, and the ADD-PATH destination's bytes carry `00000000` ahead of `180A0000` while the other's do not |
| peer-up replay, `state` to adj-rib-in and bgp-rs attached nowhere | `test/plugin/adj-rib-in-replay-unowned-peer.ci` | read: the bounce is driven by the peer's own script through a marker the observer injects once the route is STORED, so no timer decides anything |
| replay of a route learned with an RFC 2545 form-2 next hop | `test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci` | read: the destination receives the same 87-byte UPDATE twice, both carrying the 32-octet pair under Length 0x20 |
| forked adj-rib-in, replay exceeding one IPC frame | `TestRelayChunkStaysUnderFrameCeiling` (no `.ci`) | deliberate and stated in Step 3: the user-visible path is gated by two mutation-verified `.ci`, and the byte bound is arithmetic over a serialized form no `.ci` can reach without a 16 MB stored table (`ai/rules/testing.md`, "When Unit Tests Alone Are Sufficient", row 2) |
| exported symbols reachable from production | `make ze-repository-check` | 8 ISSUEs, every one in `iface`, `ike/engine` or `web/testing` -- other sessions' uncommitted work. None on this spec's files, so no symbol it adds is unwired |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | 2026-08-14, four producers make the chain: `reg.CLIHandler` into `cli.RunPlugin`, `Process.HasStructuredHandler` reporting `p.bridge != nil`, `onMessageReceived` sending JSON to a process without one, and `p.OnEvent` into `handleReceived`. The legacy ingest path is reachable in a supported deployment |
| A-2 | broken, then RESOLVED | broken 2026-08-03 and re-verified 2026-08-14: multi-path was representable in STORAGE and not across the RELAY. Resolved 2026-08-19 by AC-1 and AC-2. Mistake Log row: "Multi-path survives storage, so I-1's only question was the reconstruction context" |
| R-1 | did not fire | the plugin suite is 660/661 and the one red has no egress rail in it. The four inherited target tests are named rather than numbered now: `remove-private-as-replace-peer`, `rfc7606-relay-one-field`, `role-otc-egress-filter`, `role-otc-egress-stamp` |
| R-2 | mitigated and pinned | `TestReplayLastIndexSurvivesMultiChunkRelay` drives a multi-chunk replay through `handleCommand` and asserts one `last-index` and one `replayed` over all chunks |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/api/process-protocol.md` -- no update owed | it documents the 5-stage handshake and claim delivery; the chunk budget is a transport detail of one RPC and no stage changed | yes |
| `docs/architecture/api/ipc_protocol.md` -- no update owed for the chunker | `rpc.MaxMessageSize` is unchanged and `relayChunkBudget` stays UNDER it rather than moving it | yes |
| `docs/architecture/plugin/rib-storage-design.md` -- no update owed | grep counts zero occurrences of `RawRoute` in it. The document is the bgp-rib plugin's Loc-RIB design, and its `MPReach.NextHop() []byte` sketch returns the WHOLE field, which is what `NextHopBytes` now does: the fix moved the code TOWARD the document | yes |
| `docs/architecture/wire/messages.md` -- no update owed | the reconstruction's wire layout is unchanged; only that file's TEST moved, to assert an equality the document already states | yes |
| `rpc.StoredRoute`'s two new JSON fields -- no update owed | grep for `StoredRoute` over `docs/` finds the METHOD name in four documents and a field list in none | yes |
| `docs/guide/plugins.md` -- updated | carries the per-event retraction with the anchor `<!-- source: internal/component/plugin/server/startup_claims.go -- (*Server).UnheldRoles -->`, which resolves to a live symbol | yes |
| `ai/rules/plugins.md` and its point file -- updated | a plugin that stood a role down MUST run its default behaviour for an event naming the role | yes |
| RFC status | `make ze-rfc-check` GREEN, all eight ratchets satisfied, 3595 tags resolved. No enrolment changed, so `check_status_completeness` needs no new row | yes |
| Link health | `make ze-doc-links-check`: 24 findings before this closure and 24 after, the same 24, none on a file this spec touches | yes |

## Core Insight

**A framing is a property of the SESSION, so recording it beats normalising it,
and the zero value has to be the refusal.**

The spec opened with two options, normalise the stored bytes or tag the
reconstruction with a different context, and the finding that settled it was that
neither question was the real one. Both ingest paths already agreed on the bytes
for a source without ADD-PATH; the legacy path was simply BROKEN for a source
with one. What was missing was never a byte layout. It was the path identifier's
VALUE and the fact of its presence, and `rpc.StoredRoute` is a JSON contract a
forked plugin built against an older struct still writes. An omitted JSON number
decodes to 0, and 0 is a legal RFC 7911 identifier, so a bare `PathID uint32`
would have read "this producer does not know" as "path 0". The three-valued
marker exists for that one reason, and its zero value refuses.

The same shape appears twice more in this spec and it is the thing to carry
forward. `Server.UnheldRoles` retracts a daemon-wide promise per event because
delivery is per-peer and a claim is not; saying nothing means the promise holds,
so the engine speaks only to withdraw. And `recordNoRemoteRole` writes an empty
string rather than deleting a key, because a deletion made "this peer declared no
role" and "no OPEN was ever recorded" share one representation. Three times, the
defect was a single representation carrying two facts, and three times the fix
was to give the second fact somewhere of its own to live.
