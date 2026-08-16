# Spec: fixit-stored-route-relay-hardening

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/3 |
| Deferral shard | `plan/deferrals/fixit-stored-route-relay-hardening.md` |
| Updated | 2026-08-14 |

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

RFC7911-2-2 is separate and is NOT settled here. Section 2 states that a speaker
which re-advertises a route MUST generate its own Path Identifier.
`rfc/short/rfc7911.md` carries it as a `{gap}` with a public row in
`docs/features/rfc-status.md`, and the gap re-verified at the producer:
`fwdReencodeNLRIs` copies the received path-id and nothing in
`internal/component/bgp/reactor/` mints one. Carrying the stored path-id through
the relay inherits that gap on a second rail. Minting one in the relay alone
would diverge from the live forward rail and break the one-egress-transform
invariant, so the fix belongs at the rail and is not this spec's to make
unilaterally. The consequence is live on the deployment this replay exists for:
two route-server clients that each chose path-id 1 for one prefix are advertised
to a destination as the same (prefix, path-id) twice, so the destination keeps
one path and the other is lost. Under `ai/rules/rfc-compliance.md` this is a
choice to do LESS than full compliance, so it is put to Thomas as which way to
fix it: fix RFC7911-2-2 at the forward rail under its own spec, or keep the
disclosed gap and let the relay match the rail. Step 2 must not decide it by
default.

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

### I-2 — `relayChunkSize` bounds route count, not bytes

`relayRoutes` chunks at 4096 routes to stay under the 16 MB IPC frame ceiling
(`pkg/plugin/rpc/framing.go`). Route count does not bound bytes: `AttrHex` is hex, so
roughly twice the attribute block, and 4096 x a 4 KB block is already ~33 MB. A forked
adj-rib-in with large communities or AS_PATH blocks still loses a whole chunk as one
oversized frame. It fails closed, so this is availability, not corruption.
Replace with a byte-budget accumulator.

### I-3 — replay ownership is process-global, but replay driving is per-peer

`replayOwned` is a process-wide `atomic.Bool`. Event delivery is per-peer, per-plugin
(`internal/component/bgp/reactor/config.go`, `parseOneReceiveFlag` case `"state"`), so
a peer whose `process` block gives `state` to `bgp-adj-rib-in` but not to `bgp-rs`
leaves NOBODY replaying: adj-rib-in stood down globally, bgp-rs never sees that peer.
That is a config away, not a crash away. Scope the stand-down to peers the owner
actually drives, or reject the config combination.

### I-4 — the ownership claim does not survive an adj-rib-in respawn

`SendPostStartup` has one call site, inside `signalStartupComplete`
(`internal/component/plugin/server/startup.go`). A mid-life respawn
(`internal/component/plugin/server/reload_tx.go` -> `internal/component/plugin/process/manager.go`
`Respawn`) receives no post-startup callback, and `replayOwned` resets with the
process, so a respawned adj-rib-in resumes self-replay and the duplicate announce
returns. Re-deliver post-startup on respawn, or make the claim re-confirmable.

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

- RFC 2545 32-byte next hop (global + link-local) is truncated to 16 by
  `nhopHexFromAddr` (`adj_rib_in/rib.go`), so a replay diverges from what a live
  forward relays verbatim and an on-link peer loses the link-local next hop.
- Complex families (VPN, EVPN, Flowspec) store the WHOLE MP_REACH NLRI block for the
  first NLRI and skip the rest (`adj_rib_in/rib.go`), so a replay re-announces every
  NLRI of the originating UPDATE — the failure the strip-and-resynthesize design
  claims to prevent, confined to these families.
- No backpressure: each in-flight relay pins a read-pool buffer with no bound, so a
  slow destination pins many and then fails the remainder.
- `routeRelayer` (the test seam) has no error return, so `replayCommand`'s
  `statusError` path cannot be driven by a test.
- `Coordinator.RelayStoredRoute` has no test — neither the `ErrNoReactor` branch nor
  the delegation is exercised.
- `relay_payload_test.go` asserts `n <= size` where `n == size` is the real contract.

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
- The four target `.ci` tests (372, 378, 394, 395) stay green and non-reproducing
  under `scripts/dev/stress-repro.py`.
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
| A-2 | Multi-path (several path-ids for one prefix) is representable end to end today | `compactRouteKey` carries a path-id | The old rail's collapse was masking a storage gap, widening I-1 | store two paths for one prefix and inspect the seqmap | **broken (2026-08-03, re-verified 2026-08-14)**: representable in STORAGE (`routeKeyFromWire` keys on `PathID`), NOT across the relay: `RawRoute` (`adj_rib_in/rib.go`) and `rpc.StoredRoute` (`pkg/plugin/rpc/types.go`) still carry no path-id field. See the I-1 FINDING |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | A fix for I-1 re-breaks the byte-identity the four target tests assert | 372/378/394/395 go red | Run all four plus stress-repro on every change |
| R-2 | Chunking by bytes changes the `last-index` bgp-rs uses for delta convergence | rs delta loop never terminates, or replays forever | Assert `last-index` across a multi-chunk replay |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer-up replay from an ADD-PATH source | Routes are relayed, not refused; wire is well-formed for both add-path and non-add-path destinations |
| AC-2 | Multi-path source (two path-ids, one prefix) | Both paths survive the replay, or the limitation is explicit and tested |
| AC-3 | Forked adj-rib-in, replay whose routes exceed 16 MB | Chunked by bytes; every route arrives |
| AC-4 | Peer gives `state` to adj-rib-in but not bgp-rs | Either that peer is still replayed, or the config is rejected — never silently unreplayed |
| AC-5 | adj-rib-in respawns mid-life with bgp-rs loaded | Ownership is re-established; no duplicate announce on the next peer-up |
| AC-6 | `adj-rib-in-replay-on-peerup.ci` with the relay stubbed to error | Test goes RED (mutation-verified). **DONE 2026-08-07 — see I-5** |
| AC-7 | An egress rail cannot carry out its decision: an in-process filter PANICS on the RS fast path (`forward_rs.go`) or on the LLGR stale re-advertise rail, or that rail's modifications cannot be built | No rail reports a failure of THIS speaker as a peer's policy. `reactorForwardRS` puts the destination on the skipped list so the plugin rail decides what it could not. `decideStaleReadvertise` returns `staleFilterFailed` or `staleBuildFailed`, and `AnnounceNLRIBatch` reports the matching cause wrapping `errStaleReadvertiseWithheld` instead of `ErrNoPeersAcceptedFamily`. The route is withheld fail-closed in every case; only the report changes. **REPLACED 2026-08-03 by Thomas's ruling — see "AC-7 replaced" below. DONE** |
| AC-8 | Destination peer whose remote role was never recorded, source with a non-empty `role { export }` set (R6-1) | The suppression is counted under its own reason (`role-unrecorded`) and carries recordDrop's first-occurrence WARN, so it is never reported as an export-set decision. **DONE** |
| AC-8b | Config reload while peers stay established (R6-1) | Learned remote roles survive for every peer the new config still names, so no peer is reclassified `unknown` and black-holed by a reload. **DONE** |
| AC-9 | An export or import policy filter returns a raw override of 1..3 bytes (R6-2) | The route is suppressed (export) / dropped (import) and says so; it is never forwarded unmodified. **DONE** |
| AC-10 | A zero-dispatch FAILURE branch of `forwardUpdateCore`: read-buffer pool exhaustion in the EBGP wire build and in both transcode acquisitions, a `buildFwdBody` failure, and both dispatch attempts failing on a stopped pool | Each counts as a DROP, never as a policy suppression, and a test drives at least one branch per producer. Today only two failure branches are covered (nil forward facts, and a failing egress step), so a future `suppressedCount++` on one of these paths would reopen the fail-open with every test green. Added 2026-08-14 from `plan/deferrals/fixit-bgp-egress-rail-divergence.md`, which named this spec as its destination while no criterion here enumerated it |
| AC-11 | The two non-decided `PolicyReject` producers that no test drives: a filter IPC error under a fail-closed `FilterOnError`, and the AC-13 undeclared-attribute override | Both set `Failed`, and removing the flag from either turns a test RED. `Reactor.api` is a concrete `*pluginserver.Server` rather than an interface, so this needs a live plugin server or an injection seam. Added 2026-08-14 from the same shard, for the same reason |
| AC-12 | bgp-rs joins MID-LIFE, auto-loaded by a config reload that adds the `bgp` root (`startup_autoload.go` `autoLoadForNewConfigPaths`), while bgp-adj-rib-in is already running | Exactly one plugin replays on the next peer-up. Today neither channel reaches this case: Stage 2 runs per handshake, so an already-configured bgp-adj-rib-in is never re-told, and the backstop `claimReplayOwnership` fires only from `OnAllPluginsReady`, which the engine produces once at startup. This is the mirror of AC-5, whose direction is the adj-rib-in respawn. Added 2026-08-14 by the closure review of `spec-fixit-bgp-egress-rail-divergence` |

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
| `TestRelayAddPathRoundTrip` | `internal/component/bgp/reactor/relay_payload_test.go` | an add-path route reconstructs to wire the destination parses, for both add-path and non-add-path destinations | |
| `TestRelayMultiPathPreserved` | `internal/component/bgp/reactor/relay_payload_test.go` | two path-ids for one prefix both survive, or the limit is explicit | |
| `TestRelayChunkByteBudget` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | chunks stay under `rpc.MaxMessageSize` for large attribute blocks | |
| `TestReplayOwnershipRespawn` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | ownership re-established after respawn; no duplicate | |
| `TestCoordinatorRelayStoredRouteNoReactor` | `internal/component/plugin/coordinator_test.go` | `ErrNoReactor` branch and the delegation | |
| `TestReactorForwardRSFilterPanicIsNotPolicy` | `internal/component/bgp/reactor/egress_filter_failure_test.go` | AC-7: a panicked filter puts the destination on the skipped list; a clean reject does not | DONE (mutation-verified) |
| `TestDecideStaleReadvertiseFailureIsNotPolicy` | same file | AC-7: `staleFilterFailed` for a panic, `staleBuildFailed` for an unbuildable modification, `staleSuppress` only for a reject | DONE (mutation-verified) |
| `TestAnnounceNLRIBatchStaleFailureIsNotFamilyMismatch` | same file | AC-7 at the entry point: each failure carries its own cause wrapping `errStaleReadvertiseWithheld` and is NOT `ErrNoPeersAcceptedFamily`; a reject still is | DONE (mutation-verified) |
| `TestDecideStaleReadvertiseWithholdsOnModifyFailure` | `internal/component/bgp/reactor/reactor_stale_readvertise_test.go` | the pre-existing withheld-not-advertised property, now pinned to `staleBuildFailed` | DONE (mutation-verified) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| add-path replay | `test/plugin/` | an add-path peer establishes late and receives its routes | |
| `adj-rib-in-replay-on-peerup.ci` rewrite | `test/plugin/` | replay to an ESTABLISHED peer, asserted on wire bytes | DONE (mutation-verified, I-5) |

## Files to Modify
- `internal/component/bgp/reactor/reactor_api_relay.go` — lift `errRelayAddPath`; context choice
- `internal/component/bgp/reactor/relay_payload.go` — path-id framing; RFC 2545 next hop
- `internal/component/bgp/plugins/adj_rib_in/rib.go` — storage framing, byte-budget chunking, ownership scope
- `pkg/plugin/rpc/types.go` — `StoredRoute` path-id field, if that is the chosen route
- `internal/component/plugin/server/startup.go` / `process/manager.go` — post-startup on respawn
- `test/plugin/adj-rib-in-replay-on-peerup.ci` — make it gate

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
3. Byte-budget chunking (I-2), with the `last-index` contract asserted across chunks.
4. Ownership scope and respawn (I-3, I-4).
5. Make `adj-rib-in-replay-on-peerup.ci` gate (I-5); mutation-verify.
6. The smaller gaps (I-6).
7. `make ze-precommit-verify`, `make ze-unit-reactor-test-race`, per-test stress-repro; independent review to clean.

## Checklist
### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
### Completion (BLOCKING — before ANY commit)
- [ ] Every AC has working code + test
- [ ] 372 / 378 / 394 / 395 still green and non-reproducing under stress
- [ ] `make ze-unit-reactor-test-race` green
- [ ] `make ze-standard-test` passes
- [ ] Independent review clean
- [ ] Learned summary written
