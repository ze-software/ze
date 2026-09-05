# Spec: rfc7947-adj-rib-in-accepts-filtered-updates

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | `plan/pre-release/spec-rfcgate-6-supported-extraction-signoff.md` (owns the rfc7947 extraction sign-off this spec unblocks) |
| Phase | - |
| Handoff | verify |
| Updated | 2026-08-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 7947 Section 2.1 opens with an obligation Ze does not meet:

> "A route server MUST accept all UPDATE messages received from each of its
> clients for inclusion in its Adj-RIB-In. These UPDATE messages MAY be omitted
> from the route server's Loc-RIB or Loc-RIBs, due to filters configured for the
> purpose of implementing routing policy."

`notifyMessageReceiver` (`internal/component/bgp/reactor/reactor_notify.go`) runs
the ordered ingress step loop and returns as soon as a step denies the route. That
return sits AHEAD of the plugin dispatch that feeds `handleReceivedStructured`
(`internal/component/bgp/plugins/adj_rib_in/rib.go`) and ahead of
`reactorForwardRS`. A denied UPDATE therefore never reaches the Adj-RIB-In at all,
so the store holds the post-policy population where the RFC requires the
pre-policy one.

The path is reachable by DEFAULT, not only by operator configuration. `LoopIngress`
(`internal/component/bgp/reactor/filter/loop.go`) denies on the local AS appearing
in AS_PATH, on the local Router ID in ORIGINATOR_ID, and on the local Router ID in
CLUSTER_LIST. The per-peer import chain (`runIngressPolicyChain`) denies on
operator policy.

Thomas ruled on 2026-08-30 that Ze inserts into the Adj-RIB-In FIRST, before the
filter verdict is honored, for route-server client peers, marking the stored entry
filtered. He did not choose the narrower reading under which Section 2.1 governs
only the route server's own per-client policy.

The spec carries a second, smaller deliverable: two declared LEVELS in
`rfc/short/rfc7947.md` are wrong about their own source, and Thomas ruled they are
corrected under their existing ids.

### Goals

| # | Goal |
|---|------|
| G-1 | A route-server client's denied UPDATE is held in that peer's Adj-RIB-In, marked filtered, so Ze satisfies RFC 7947 Section 2.1 |
| G-2 | A filtered entry is never re-advertised, on the live rail or on the peer-up replay rail, so the policy decision still holds |
| G-3 | An operator can tell a filtered entry from an accepted one at the CLI, on every pipe rendering |
| G-4 | `RFC7947-x-4` and `RFC7947-x-5` declare the strength their own RFC text states |
| G-5 | `rfc/extraction/rfc7947.json` reaches a landed sign-off with no open site |

### Non-goals

| Item | Why |
|------|-----|
| A pre-policy Adj-RIB-In for a non-route-server peer | RFC 7947 Section 2.1 binds a route server. Key Design Decisions states what the wider scope would cost |
| BMP Adj-RIB-In Pre-Policy monitoring (RFC 7854 Section 5) | A separate obligation under a separate RFC, and bgp-bmp is a separate subscriber |
| Path-hiding mitigation (multiple Loc-RIBs, add-path) | RFC 7947 Section 2.3 is non-normative; this spec only corrects what `RFC7947-x-5` CLAIMS about it |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - the Adj-RIB-In raw hex store, declared by `rib.go` and `rib_commands.go`
  → Constraint: the store is a per-source-peer map of sequence maps holding raw wire hex (`AttrHex`, `NHopHex`, `NLRIHex`), never a decoded route. A filtered mark is a field on `RawRoute`, never a second map.
  → Decision: the sequence number lives in the seqmap, not in `RawRoute`, so a filtered entry consumes a sequence number like any other and the `Since` walk still visits it. The skip therefore has to be in the WALK, not in the numbering.
- [ ] `docs/architecture/api/process-protocol.md` - the event namespace and type registry, declared by `subscribe.go`, `events.go`, `bridge.go` and `replay_cut.go`
  → Constraint: `parseOneReceiveFlag` (`internal/component/bgp/reactor/config.go`) and `ReceiveEventValidator` (`internal/component/config/validators.go`) both resolve a `receive` token against the bgp event registry at call time. A new event type registered in that namespace therefore reaches the subscription grammar, the YANG validator and completion with no edit to any of the three.
  → Decision: delivery is per-subscription (`SubscriptionManager.getMatching`), so an event type nobody subscribes to reaches nobody. That is what makes a distinct event type fail-closed and a boolean on the existing `update` event fail-open.
- [ ] `docs/architecture/api/ipc_protocol.md` - the typed RPC enums, declared by `pkg/plugin/rpc/enums.go`
  → Constraint: `EventKind` is a small unsigned integer with a fixed-size string table and a `MarshalText`/`UnmarshalText` pair. A new kind takes the next ordinal, a wire string, a table row, an `UnmarshalText` case, and `EventKindCount` moves.
- [ ] `docs/architecture/core-design.md` - the message receiver dispatch, declared by `reactor_notify.go`, `session_read.go`, `config.go` and `internal/component/bgp/plugin/register.go`
  → Constraint: the ingress step loop is the ONE stage-ordered pipeline; step order is a declared Stage, not code position. Each `orderedIngressStep` carries a `name` (`filter_ordered.go`), which is the operator-visible reason a route was denied and needs no new plumbing.
  → Decision: `checkPrefixLimits` is called from the session read loop (`session_read.go`) BEFORE `notifyMessageReceiver`, so the per-family `prefix` maximum already counts a denied UPDATE's NLRIs. That is the memory bound on the filtered population.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7947.md` - the route server obligations, and the two rows this spec corrects
  → Constraint: a requirement id is anchored to the section its line CITES (`validateID`, `internal/le/rfc/summary.go`). `RFC7947-x-4` and `RFC7947-x-5` anchor to the no-section marker, so their corrected text MUST keep a trailing `(Key Requirements)` parenthetical and MUST name the section in the `RFC 7947 Section N` form that `crossRFCSecRE` scrubs. Putting a bare section citation in the trailing parenthetical would demand a section-anchored id and break the permanence rule.
  → Constraint: `checkLevelRatchet` (`internal/le/rfc/check_ratchets.go`) fires when a requirement leaves the gated set (MUST, MUST NOT, SHALL, SHALL NOT, REQUIRED). It is silenced only by a paragraph in the summary opening `Correction <YYYY-MM-DD>:`, naming the id in backticks, and quoting at least 24 characters, in double quotes, of a sentence that appears in `rfc/full/rfc7947.txt`.
- [ ] `rfc/full/rfc7947.txt` Sections 2.1, 2.3 and 2.3.3 - the authority
  → Constraint: Section 2.3 states "Neither Section 2.3 nor its subsections form part of the normative specification of this document; they are included for information purposes only." Every obligation `RFC7947-x-4` and `RFC7947-x-5` claim is drawn from Section 2.3 or its subsections.
  → Constraint: the only normative neighbour is Section 2.1's "The route server SHOULD forward UPDATE messages from its Loc-RIB or Loc-RIBs to its clients as determined by local policy", and Section 2.3.3's "Authors of route server implementations may wish to consider one of the methods described in Section 2.3.2".
- [ ] `rfc/short/rfc4271.md` - the Adj-RIB-In definition the store implements
  → Constraint: RFC 4271 Section 3.2 makes the Adj-RIB-In the unprocessed information a peer advertised. RFC 7947 Section 2.1 is not in tension with it; it makes explicit for a route server what the base spec leaves to the implementation.

**Key insights:** (minimal context to resume after compaction)
- The reject return is in `reactor_notify.go`, inside the ordered ingress step loop, and it precedes BOTH the Adj-RIB-In dispatch and `reactorForwardRS`.
- Seven plugins subscribe to `update direction received`: bgp-rib, bgp-rs, bgp-rr, bgp-rpki, bgp-rpki-decorator, bgp-bmp, bgp-adj-rib-in. Any design that reuses the `update` event has to be honored by six of them that must ignore it.
- The receive-token vocabulary, the subscription grammar and the YANG completion are all DERIVED from the bgp event registry, so a new event type costs one constant plus one registration argument.
- The filtered mark's only replay consumer is `buildReplayRoutes`. Missing that skip turns a policy denial into a route leak on the next peer-up.
- `RFC7947-x-4` moving MUST to SHOULD is the only one of the two corrections `checkLevelRatchet` charges for.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` builds the `RawMessage`, runs the ordered ingress pipeline, caches the `ReceivedUpdate`, runs the RS fast path, then dispatches. The step loop's non-accept branch is where a denied UPDATE stops, with the comment "Route rejected by filter; don't cache or dispatch."
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `orderedIngressStep` carries `name`, `stage`, `priority`, and exactly one executor (an in-process filter func, or the policy chain flag). `ingressStepResult` carries `accept`, `modifiedPayload`, `teardown`, `notifyCode`, `notifySubcode`.
- [ ] `internal/component/bgp/reactor/filter/loop.go` - `LoopIngress` denies on the local ASN in AS_PATH, and on iBGP sessions on ORIGINATOR_ID or CLUSTER_LIST naming the local Router ID. Inert only when the peer sets `LoopDisabled`.
- [ ] `internal/component/bgp/reactor/session_read.go` - the read loop calls `checkPrefixLimits` before `notifyMessageReceiver`, so the configured prefix maximum counts offered NLRIs.
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `checkPrefixLimits` collects every prefix section of a message before counting any of it, and settles the installed-versus-offered counting modes.
- [ ] `internal/component/bgp/reactor/config.go` - `ps.RSClient` and `ps.RSFastPath` come from the plugin-owned `rs-client` and `rs-fast-path` YANG leaves. `parseOneReceiveFlag` resolves an attach receive token against the bgp event registry.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - `runAdjRIBInPlugin` subscribes to `update direction received` and `state` at format `full`; `handleReceivedStructured` walks withdrawals before announces and installs through `installStructuredNLRIs` and `installComplexNLRIs`; `buildReplayRoutes` walks every source peer's seqmap and emits `rpc.StoredRoute`; `handleStructuredState` deletes a peer's whole map on down.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - `status` answers `running`, `total-routes` and `peers`; `show` answers an `adj-rib-in` envelope mapping a peer to a list of route objects carrying `family`, `key`, `nhop-hex`, `attr-hex`, `nlri-hex`, `seq-index` and `validation-state`; `replayCommand` calls `buildReplayRoutes` then `relayRoutes`. `commandDecls` declares both show commands with a document shape.
- [ ] `internal/component/bgp/plugins/adj_rib_in/replay_cut.go` - `replayCut.excludes` bounds a replay by reactor MessageID; presence and value are carried separately because a cut of zero is a real cut.
- [ ] `internal/core/bgp/events/events.go` - the bgp event type constants, a leaf package with no dependencies.
- [ ] `internal/component/bgp/plugin/register.go` - registers the bgp namespace with its event types.
- [ ] `pkg/plugin/rpc/enums.go` - `EventKind` and its wire strings, twelve ordinals including the unspecified zero and the count sentinel.
- [ ] `internal/component/config/validators.go` - `ReceiveEventValidator` resolves a token against the bgp event registry and completes from the registry's own names.
- [ ] `internal/component/plugin/server/subscribe.go` - `ParseSubscription` and `SubscriptionManager.getMatching`: delivery is to the processes whose subscription matches namespace, type, direction and peer.
- [ ] `internal/le/rfc/check_ratchets.go` - `checkLevelRatchet` and `correctionAuthorizes`.
- [ ] `internal/le/rfc/check_core.go` - `evaluate` demands both polarities for every gated requirement of an enrolled RFC.
- [ ] `internal/le/rfc/summary.go` - `parseChecklistLine`, `extractSection` and `validateID`: the id is anchored to the trailing section citation.
- [ ] `internal/le/rfc/rfc.go` - the five gated levels, the six advisory levels, and the no-section anchor.
- [ ] `rfc/full/rfc7947.txt` - Sections 2.1, 2.2, 2.3 and 2.3.3.
- [ ] `rfc/short/rfc7947.md` - the eight checklist rows and the three existing Correction paragraphs.
- [ ] `test/interop/scenarios/bgp-route-server-frr/ze.conf` - the working route-server scenario: two `rs-client true` peers, each attaching the rs process with an explicit receive and send grant.

**Behavior to preserve:** (unless the user explicitly said to change it)
- A denied UPDATE is still NOT forwarded, NOT installed in the RIB, NOT reflected, NOT validated by RPKI, and NOT reported to BMP. The filter verdict keeps every consequence it has today except the Adj-RIB-In one.
- A denied UPDATE from a peer that is not a route-server client stays absent from the Adj-RIB-In.
- `show bgp adj-rib-in` keeps its document shape, its per-peer envelope, and every existing route key.
- `status` keeps `running`, `total-routes` and `peers`, with `total-routes` counting every entry the store holds.
- `buildReplayRoutes` keeps its cursor semantics, its replay cut bound, and its rule that a peer's own routes are never replayed to it.
- The reactor still returns false from `notifyMessageReceiver` on a denial, so the cache is not populated and the pool buffer's lifetime is unchanged.
- A teardown verdict still queues the NOTIFICATION and drops the route with no Adj-RIB-In insertion: a session being torn down has no Adj-RIB-In to hold anything.

**Behavior to change:** (only what the user asked for)
- On a denial, when the source peer has `rs-client true`, the reactor emits one filtered-update event carrying the same `RawMessage` and the denying step's name, before returning false.
- bgp-adj-rib-in subscribes to that event type and stores the routes it carries with a filtered mark.
- `buildReplayRoutes` skips a filtered entry.
- `show bgp adj-rib-in` and `show bgp adj-rib-in status` report the filtered population.
- `RFC7947-x-4` moves from MUST to SHOULD; `RFC7947-x-5` moves from SHOULD to MAY; a new `RFC7947-2.1-1` is added at MUST.
- `rfc/extraction/rfc7947.json` lands with its Section 2.1 site mapped.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire bytes: a BGP UPDATE arriving on an established session from a peer configured `rs-client true`.
- Format at entry: the raw UPDATE body, already header-validated and prefix-limit-counted by the session read loop, wrapped as a lazily parsed `WireUpdate` over a pool buffer.
- Second entry point, operator-facing: `show bgp adj-rib-in` and `show bgp adj-rib-in status`, typed at the CLI or sent as a command RPC.

### Transformation Path
1. `session_read.go` reads the message, runs `checkPrefixLimits`, and calls `notifyMessageReceiver`.
2. `notifyMessageReceiver` builds the `RawMessage` and walks the ordered ingress steps.
3. A step answers accept false. Today: return false. New: when the peer's `RSClient` setting is true, emit one filtered-update received event carrying the `RawMessage` and the denying step's name, then return false.
4. The event dispatcher delivers the event only to processes whose subscription matches the bgp namespace, the filtered-update type and the received direction, and whose peer binding grants that token.
5. bgp-adj-rib-in's structured handler routes the new event kind to a filtered-ingest path that reuses the existing withdrawal-before-announce walk and installs each `RawRoute` with the filtered mark set.
6. `buildReplayRoutes` walks the seqmap and skips a marked entry, so no `rpc.StoredRoute` ever carries one.
7. `show` and `status` read the same entries and report the mark.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session read goroutine → reactor ingress pipeline | direct call, `notifyMessageReceiver` | No |
| Reactor → plugin process | `rpc.StructuredEvent` over DirectBridge, or the JSON event rail for a forked plugin | No |
| Reactor config → peer settings | `ps.RSClient` from the plugin-owned `rs-client` YANG leaf | No |
| Config attach grant → delivery graph | a filtered-update receive token resolved against the bgp event registry | No |
| Plugin → engine (replay) | `rpc.StoredRoute` via the relay call; a filtered entry must never appear here | No |
| Plugin → operator | command answer through `ApplyPipes`, document shape | No |

### Integration Points
- `internal/core/bgp/events/events.go` - the one declaration of the type name; the subscription grammar, the YANG validator and completion all derive from it.
- `internal/component/bgp/plugin/register.go` - adds the new type to the namespace registration argument list.
- `pkg/plugin/rpc/enums.go` - the typed kind carried on `StructuredEvent`.
- `internal/component/bgp/plugins/adj_rib_in/rib.go` `SetStartupSubscriptions` - adds the new type in the received direction.
- `internal/component/bgp/plugins/adj_rib_in/rib.go` `buildReplayRoutes` - the one place the mark gates re-advertisement.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `buildReplayRoutes` is the ONLY path by which an Adj-RIB-In entry becomes an advertisement | `gopls references` on the store field: the only non-test readers are `installStructuredNLRIs`, `removeStructuredNLRIs`, `installComplexNLRIs`, `removeComplexNLRIs`, `handleStructuredState`, `handleReceived`, `handleState`, `buildReplayRoutes`, `status`, `show`, and the promote and expire paths in `rib_validation.go`. Only `buildReplayRoutes` produces `rpc.StoredRoute` | a filtered route leaks on a second, unguarded rail | `TestBuildReplayRoutesOmitsFilteredEntries` plus the interop scenario's negative assertion | unvalidated |
| A-2 | The per-family prefix maximum bounds the filtered population, because `checkPrefixLimits` runs before the filter chain | the session read loop in `session_read.go` calls it ahead of `notifyMessageReceiver` | the Adj-RIB-In of an rs-client peer is unbounded and a hostile client can exhaust memory with routes that are all denied | `TestFilteredRoutesCountAgainstThePrefixMaximum`, driving a peer past its configured maximum with denied UPDATEs only | unvalidated |
| A-3 | A family configured to count only INSTALLED prefixes still counts a denied UPDATE, because the count is taken before the verdict exists | the two counting modes are settled inside `checkPrefixLimits` (`session_prefix.go`), which the filter verdict never reaches | the bound in A-2 does not hold for that mode, and the spec owes an explicit cap on the filtered population | reading `collectPrefixSections` and `restoreInstalledPrefixCounts` at the producer, then a boundary test | unvalidated |
| A-4 | No existing consumer reads `show bgp adj-rib-in` route objects by exact key set | `plan/journal/gate-excludes-part-of-its-population.md` records `med-removal-before-decision` parsing this command's shape; adding keys removes none | a functional fixture goes red on an added key | grep the functional suite for the command, then read each hit's assertions | unvalidated |
| A-5 | `RFC7947-x-4` and `RFC7947-x-5` carry no test tags today, so lowering their level orphans nothing | both rows are unticked with no annotation, and `evaluate` reports an unknown RID for a tag naming a requirement that does not exist | a tagged test loses its gate silently | `./le rfc check` before and after, comparing the gated population | unvalidated |
| A-6 | bgp-rs takes no delivery of the new event type, so its forward-target selection and its replay cut are unchanged | `rs/server.go` subscribes to update received, open received, state and refresh; a new type reaches only a subscriber | the route server forwards a filtered route, which is the leak G-2 exists to prevent | `TestRouteServerTakesNoDeliveryOfFilteredUpdates` plus the interop negative | unvalidated |
| A-7 | The ingest-position contract is not disturbed by a second event kind carrying a MessageID | `noteIngested` publishes the position bgp-rs's cut is compared against, and a filtered UPDATE also has a MessageID | the peer-up cut advances past routes the live rail will not deliver, and the replay omits them: both rails silent | `TestIngestPositionAdvancesOnFilteredUpdates`, asserting the cut and the replay agree | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The filtered entry is replayed on peer-up and the policy denial becomes a route leak | the interop scenario's third party learns a prefix its policy denies | `buildReplayRoutes` skips the mark, and the interop scenario asserts the ABSENCE, proven by reverting the skip and watching it go red |
| R-2 | A future subscriber to the new type treats it as an ordinary UPDATE | a new plugin's `SetStartupSubscriptions` names the type | the event kind is distinct and its name says what it is; the handler that routes it is the only one that stores |
| R-3 | The withdrawal path does not remove a filtered entry, so a withdrawn prefix stays in the store forever | `show bgp adj-rib-in` reports a prefix the client withdrew | the filtered-ingest path reuses the SAME withdrawal walk; a withdrawal is not filtered separately, it removes whatever key it names |
| R-4 | An rs-client peer subject to a teardown verdict inserts a route into a store about to be deleted | none, it is harmless | teardown returns before the emit, and the state handler deletes the whole map on down |
| R-5 | The filtered population grows without bound for an rs-client peer with no configured prefix maximum | `total-routes` climbs while `show bgp rib` stays flat | A-2 and A-3 settle whether the existing bound covers it; if not, this spec owes an explicit cap and the AC table gains a row |
| R-6 | Adding a new event kind moves the enum count, breaking a consumer that indexes the table by ordinal | compile failure, or a wrong wire string | the new kind takes the next ordinal and the count moves with it; run `gopls references` on the count constant before the edit |
| R-7 | The correction paragraph's quote does not appear verbatim in `rfc/full/rfc7947.txt` after whitespace squashing | `./le rfc check` reports the level ratchet with the id named | `correctionAuthorizes` squashes whitespace on both sides; the quote is copied from the file, not retyped |
| R-8 | The new `RFC7947-2.1-1` is gated and one polarity is missing at closure | `./le rfc check` names the missing polarity | both polarities are in the AC table and the TDD plan from the start |
| R-9 | Another session is mid-edit in `internal/component/bgp/reactor` (`api_sync_test.go`, `SignalPeerAPIReady`), so a red there is not this spec's | a red in a test file this spec does not name | judge the change by the evidence the change produced; report what could not be verified (`ai/rules/principles.md`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A route the operator's policy denies is advertised to other route-server clients at an IXP. That is a route leak with a third party's traffic behind it, and it is visible outside this repository |
| How is it reverted? | Single commit revert. Nothing persists: the Adj-RIB-In is in-memory and rebuilt on peer-up, and the new event type is inert with no subscriber |
| Who else touches this path? | A concurrent session is editing `internal/component/bgp/reactor` (`api_sync_test.go`, `SignalPeerAPIReady`). `plan/immediate/spec-bgp-session-ready-contract.md` owns the initial-sync barrier. `plan/pre-release/spec-rfcgate-6-supported-extraction-signoff.md` owns the rfc7947 sign-off this spec unblocks |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE denied by `LoopIngress` on an `rs-client true` peer | → | the filtered-update emit in `notifyMessageReceiver` | `TestNotifyEmitsFilteredUpdateForRouteServerClient` |
| A filtered-update received event | → | the `AdjRIBInManager` filtered-ingest handler | `TestAdjRIBInStoresFilteredUpdate` |
| Peer establishes, bgp-rs drives `request bgp adj-rib-in replay` | → | `buildReplayRoutes` | `TestBuildReplayRoutesOmitsFilteredEntries` |
| Operator types `show bgp adj-rib-in` | → | the `show` handler in `rib_commands.go` | `TestShowReportsFilteredMark` |
| Operator types `show bgp adj-rib-in status` | → | the `status` handler in `rib_commands.go` | `TestStatusReportsFilteredCount` |
| Config attach block granting the filtered receive token | → | `parseOneReceiveFlag` | `TestReceiveGrantAcceptsUpdateFilteredToken` |
| A route-server client announces a prefix its policy denies | → | the whole chain, over the wire | `bgp-route-server-filtered-adj-rib-in-frr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An `rs-client true` peer sends an UPDATE that `LoopIngress` denies | `show bgp adj-rib-in` reports the prefix under that peer, marked filtered, naming the loop step as the denying step |
| AC-2 | An `rs-client true` peer sends an UPDATE that its import policy chain denies | `show bgp adj-rib-in` reports the prefix under that peer, marked filtered, naming the peer policy chain as the denying step |
| AC-3 | The condition of AC-1 or AC-2 holds | `show bgp rib` reports no route for that prefix from that peer, and no other route-server client is sent an UPDATE for it |
| AC-4 | The condition of AC-1 holds, then a second route-server client establishes | The second client receives no advertisement for the filtered prefix, and receives every accepted prefix |
| AC-5 | A peer with `rs-client` unset (the default false) sends an UPDATE that `LoopIngress` denies | `show bgp adj-rib-in` reports no entry for that prefix: today's behavior is unchanged for every non-route-server peer |
| AC-6 | An `rs-client true` peer announces a prefix that is denied, then withdraws it | The filtered entry is gone from `show bgp adj-rib-in` |
| AC-7 | An `rs-client true` peer holding filtered entries goes down | Its filtered entries are gone from `show bgp adj-rib-in`, with the rest of its store |
| AC-8 | `show bgp adj-rib-in` is rendered through the json, yaml and table pipe operators | Each rendering distinguishes a filtered entry from an accepted one, from the same payload |
| AC-9 | `show bgp adj-rib-in status` on a store holding both populations | The answer reports the filtered count separately, and `total-routes` still counts every entry |
| AC-10 | A config names the filtered receive token in an attach process block | The config loads, and completion offers the token |
| AC-11 | A config names a misspelling of that token | The config is refused, naming the token and listing the valid names |
| AC-12 | `./le rfc check` runs on the tree | `RFC7947-2.1-1` exists at MUST with a positive and a negative tagged test, and the check is clean |
| AC-13 | `rfc/short/rfc7947.md` is read | `RFC7947-x-4` reads SHOULD and `RFC7947-x-5` reads MAY; each has a `Correction 2026-08-30:` paragraph naming its id in backticks and quoting the RFC sentence that states the lower strength |
| AC-14 | `./le rfc extraction-status` runs | rfc7947 is reported signed, with no site carrying a null disposition |
| AC-15 | An `rs-client true` peer with a configured prefix maximum of N sends N+1 denied prefixes | The session is torn down with Cease and the Maximum Number of Prefixes Reached subcode, exactly as it is for accepted prefixes |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs an IXP route server, an import policy denies a client's prefix, and asks what that client actually sent | wire → session read → ingress deny → filtered event → Adj-RIB-In marked → `show bgp adj-rib-in` | `adj-rib-in-filtered-visible` |
| 2 | Confirms the denied prefix reaches no other client, on the live rail and after a reconnect | wire → ingress deny → no forward; peer-up → replay → `buildReplayRoutes` skip | `bgp-route-server-filtered-adj-rib-in-frr` |
| 3 | Counts how much of a client's table policy is rejecting | `show bgp adj-rib-in status` → filtered count beside total | `adj-rib-in-filtered-status` |
| 4 | Grants a process delivery of filtered UPDATEs in the config editor, using completion | YANG leaf-list → the receive validator's completion func → the bgp event registry | `TestReceiveGrantAcceptsUpdateFilteredToken` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNotifyEmitsFilteredUpdateForRouteServerClient` | `internal/component/bgp/reactor/reactor_notify_test.go` | a denial on an rs-client peer emits one filtered-update received event carrying the denying step's name (RFC7947-2.1-1 positive) | |
| `TestNotifyEmitsNoFilteredUpdateForOrdinaryPeer` | `internal/component/bgp/reactor/reactor_notify_test.go` | a denial on a peer with rs-client false emits nothing (AC-5) | |
| `TestNotifyEmitsNoFilteredUpdateOnTeardown` | `internal/component/bgp/reactor/reactor_notify_test.go` | a teardown verdict returns before the emit | |
| `TestFilteredUpdateIsNotForwarded` | `internal/component/bgp/reactor/forward_rs_test.go` | a denied UPDATE reaches neither the RS fast path nor the delivery channel (RFC7947-2.1-1 negative) | |
| `TestAdjRIBInStoresFilteredUpdate` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | the filtered-ingest path stores the route with the mark and the denying step name | |
| `TestAdjRIBInFilteredWithdrawalRemovesEntry` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | AC-6 | |
| `TestAdjRIBInPeerDownClearsFilteredEntries` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | AC-7 | |
| `TestBuildReplayRoutesOmitsFilteredEntries` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | AC-4, the security constraint: no stored route carries a filtered entry | |
| `TestBuildReplayRoutesCursorSkipsFilteredWithoutStalling` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | a filtered entry consumes a sequence number, so the returned cursor still advances past it and a later incremental replay does not re-walk it | |
| `TestIngestPositionAdvancesOnFilteredUpdates` | `internal/component/bgp/plugins/adj_rib_in/ingest_position_test.go` | A-7: the ingest position and the replay cut agree when a filtered UPDATE sits between two accepted ones | |
| `TestShowReportsFilteredMark` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | AC-8, at the payload level | |
| `TestStatusReportsFilteredCount` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | AC-9 | |
| `TestRouteServerTakesNoDeliveryOfFilteredUpdates` | `internal/component/bgp/plugins/rs/server_test.go` | A-6: the route server's subscription set does not match the new type | |
| `TestReceiveGrantAcceptsUpdateFilteredToken` | `internal/component/bgp/reactor/config_test.go` | AC-10 and AC-11 | |
| `TestEventKindUpdateFilteredRoundTrips` | `pkg/plugin/rpc/enums_test.go` | the wire string marshals and unmarshals, and the enum count covers it | |
| `TestFilteredRoutesCountAgainstThePrefixMaximum` | `internal/component/bgp/reactor/session_prefix_test.go` | AC-15 and A-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| the new event kind's ordinal | 1 to one below the enum count | the next free ordinal | 0, the unspecified kind, which refuses to marshal | the count sentinel and above, which the marshaller rejects |
| prefix maximum counting a denied UPDATE | 0 to 4294967295 | N, the session survives | N/A | N+1, Cease with the max-prefix subcode |
| filtered entries per source peer | 0 to the family's configured maximum | the maximum | N/A | one above the maximum is unreachable: the session is torn down first |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `adj-rib-in-filtered-visible` | `test/plugin/` | an rs-client peer announces a prefix a loop check denies; the operator sees it in `show bgp adj-rib-in` marked filtered, and absent from `show bgp rib` | |
| `adj-rib-in-filtered-status` | `test/plugin/` | `show bgp adj-rib-in status` reports the filtered count beside the total | |
| `adj-rib-in-filtered-not-replayed` | `test/plugin/` | a second rs-client peer establishes after the filtered UPDATE and is sent every accepted prefix and none of the filtered one | |
| `adj-rib-in-filtered-absent-for-plain-peer` | `test/plugin/` | the same denied UPDATE on a peer without rs-client leaves the Adj-RIB-In empty | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-route-server-filtered-adj-rib-in-frr` | `test/interop/scenarios/` | FRR and BIRD, modelled on `bgp-route-server-frr`: the same two-client shape, `rs-client true` on both, the rs attach block on each, plus bgp-adj-rib-in attached and an import policy on one client that denies one prefix | An FRR client's denied prefix is held in Ze's Adj-RIB-In, is never advertised to the BIRD client on the live rail, and is still never advertised after the BIRD client reconnects and is replayed | |

## Files to Modify

- `internal/core/bgp/events/events.go` - the filtered-update event type constant beside the existing type constants. Design doc: `docs/architecture/api/process-protocol.md`
- `internal/component/bgp/plugin/register.go` - name the new type in the namespace registration. Design doc: `docs/architecture/core-design.md`
- `pkg/plugin/rpc/enums.go` - the new event kind, its wire string, its table row, its unmarshal case, and the count. Design doc: `docs/architecture/api/ipc_protocol.md`
- `internal/component/bgp/reactor/reactor_notify.go` - emit the filtered event before the deny return, gated on the peer's `RSClient` setting, carrying the denying step's name. Design doc: `docs/architecture/core-design.md`
- `internal/component/bgp/reactor/session_prefix.go` - only if A-3 shows the installed-count mode does not cover a denied UPDATE. Design doc: `docs/architecture/core-design.md`
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - `RawRoute` gains the filtered mark and the denying step name; the startup subscriptions gain the new type; the structured dispatch routes the new kind; `buildReplayRoutes` skips a marked entry. Design doc: `docs/architecture/plugin/rib-storage-design.md`
- `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - `show` and `status` report the mark and the count. Design doc: `docs/architecture/plugin/rib-storage-design.md`
- `rfc/short/rfc7947.md` - add `RFC7947-2.1-1`; correct `RFC7947-x-4` and `RFC7947-x-5`; add two `Correction 2026-08-30:` paragraphs
- `docs/features/rfc-status.md` - the RFC 7947 row: the Adj-RIB-In acceptance, and the two corrected levels
- `docs/architecture/plugin/rib-storage-design.md` - the filtered population and the replay skip
- `docs/architecture/api/process-protocol.md` - the filtered-update event type and its receive-grant token
- `docs/architecture/api/ipc_protocol.md` - the new event kind
- `docs/architecture/api/commands.md` - the `show bgp adj-rib-in` and status answer keys
- `docs/architecture/core-design.md` - the ingress deny path now has a consumer
- `docs/guide/plugins.md`, `docs/features/plugins.md` - the bgp-adj-rib-in subscription set
- `docs/guide/route-reflection.md`, `docs/guide/rpki.md` - checked for staleness against the changed `show bgp adj-rib-in` shape

## Files to Create

- `rfc/extraction/rfc7947.json` - the landed sign-off, promoted from this session's scratch with the Section 2.1 site mapped to `RFC7947-2.1-1`
- `test/plugin/adj-rib-in-filtered-visible.ci`
- `test/plugin/adj-rib-in-filtered-status.ci`
- `test/plugin/adj-rib-in-filtered-not-replayed.ci`
- `test/plugin/adj-rib-in-filtered-absent-for-plain-peer.ci`
- `test/interop/scenarios/bgp-route-server-filtered-adj-rib-in-frr/` with its ze, frr and bird configs
- the retired deferral shard "rfc7947-adj-rib-in-accepts-filtered-updates"

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new leaf. The filtered receive token becomes a valid VALUE of the existing receive leaf-list because its validator resolves the bgp event registry at call time (`ReceiveEventValidator`, `internal/component/config/validators.go`) |
| YANG validation constraints | N-A | The receive leaf-list is already constrained by `ReceiveEventValidator`; the new value is admitted by the registry, not by a widened constraint |
| YANG custom validators | Yes | `ReceiveEventValidator` (`internal/component/config/validators.go`) needs no edit; AC-10 and AC-11 prove it admits the new token and refuses a misspelling |
| CLI commands/flags | No | No new command. `show bgp adj-rib-in` and `show bgp adj-rib-in status` gain answer keys, declared in `commandDecls` (`internal/component/bgp/plugins/adj_rib_in/rib.go`) |
| CLI grammar (keyword before value) | N-A | No new verb, noun or selector; the grammar is untouched |
| Editor autocomplete | Yes | Automatic: the receive validator's completion func reads the registry the new type registers into |
| Functional test for new RPC/API | Yes | `test/plugin/adj-rib-in-filtered-visible.ci` and its three siblings |
| Pipe completeness | Yes | Both commands keep their document shape; the mark is a FIELD on the route object, never a character glued to the key, so json, yaml and table each render it (AC-8) |
| Env var registration | N-A | No leaf under the environment subtree |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, kernel module, procfs, netlink, binary or certificate. The change is in-memory and on an existing event rail |
| Prometheus counters/metrics | Yes | A filtered-route gauge per peer, named and labelled in the implementation phase beside the existing prefix-count metric (`setPrefixCountMetric`, `internal/component/bgp/reactor/session_prefix.go`) |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new address family, NLRI type, capability code or attribute code. The change is in the ingress pipeline and a plugin's store |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - the route-server Adj-RIB-In holds the pre-policy population |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` - the filtered receive token |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - the new `show bgp adj-rib-in` and status keys |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - the answer shapes |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - the bgp-adj-rib-in subscription set |
| 6 | Has a user guide page? | Yes | `docs/guide/route-reflection.md` and `docs/guide/rpki.md` both show `show bgp adj-rib-in` output; verified against the new shape |
| 7 | Wire format changed? | No | No BGP message, attribute or NLRI encoding changes. Only which in-process consumer sees a message already parsed |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` and `docs/architecture/api/ipc_protocol.md` - the event type and the event kind |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7947.md` and the RFC 7947 row of `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | No runner, harness or fixture-format change. Four new functional fixtures and one new interop scenario use the existing shapes |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - a pre-policy Adj-RIB-In for route-server clients is a comparison row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (the ingress deny path has a consumer) and `docs/architecture/plugin/rib-storage-design.md` (the filtered population and the replay skip) |
| 13 | Route metadata keys added/changed? | No | The mark lives on `RawRoute` inside the plugin, not on the route metadata map the reactor threads through the ingress pass |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` - the filtered-route gauge |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` - a new registered event type |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, do not answer from memory: run `./le spec citation anchors spec plan/immediate/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md` at the start of the implementation phase and name every document it lists. The four Design documents the changed files DECLARE are named above in Files to Modify |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/route-reflection.md` and `docs/guide/rpki.md` carry `show bgp adj-rib-in` samples; each is checked against the handler and updated |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the event type exists end to end and nobody stores anything yet
   - Tests: `TestEventKindUpdateFilteredRoundTrips`, `TestReceiveGrantAcceptsUpdateFilteredToken`, `TestNotifyEmitsFilteredUpdateForRouteServerClient`
   - Files: `internal/core/bgp/events/events.go`, `internal/component/bgp/plugin/register.go`, `pkg/plugin/rpc/enums.go`, `internal/component/bgp/reactor/reactor_notify.go`
   - Verify: the emit fires, the token validates, and `TestAdjRIBInStoresFilteredUpdate` fails because nothing subscribes
2. **Phase: Store the filtered entry** -- the mark and the ingest path
   - Tests: `TestAdjRIBInStoresFilteredUpdate`, `TestAdjRIBInFilteredWithdrawalRemovesEntry`, `TestAdjRIBInPeerDownClearsFilteredEntries`, `TestIngestPositionAdvancesOnFilteredUpdates`
   - Files: `internal/component/bgp/plugins/adj_rib_in/rib.go`
   - Verify: `show bgp adj-rib-in` holds the entry; `show bgp rib` still does not
3. **Phase: Close the replay rail (the security half)** -- write `TestBuildReplayRoutesOmitsFilteredEntries` FIRST, watch it fail against phase 2's code, then add the skip
   - Tests: `TestBuildReplayRoutesOmitsFilteredEntries`, `TestBuildReplayRoutesCursorSkipsFilteredWithoutStalling`, `TestRouteServerTakesNoDeliveryOfFilteredUpdates`, `TestFilteredUpdateIsNotForwarded`
   - Files: `internal/component/bgp/plugins/adj_rib_in/rib.go`
   - Verify: no stored route carries a filtered entry, on any path
4. **Phase: Operator surface** -- the mark reaches the CLI
   - Tests: `TestShowReportsFilteredMark`, `TestStatusReportsFilteredCount`, the four functional fixtures
   - Files: `internal/component/bgp/plugins/adj_rib_in/rib_commands.go`, the four new fixtures under `test/plugin/`
   - Verify: json, yaml and table each distinguish the two populations
5. **Phase: Bound the population** -- settle A-2 and A-3 at the producer
   - Tests: `TestFilteredRoutesCountAgainstThePrefixMaximum`
   - Files: `internal/component/bgp/reactor/session_prefix.go`, only if the reading shows the existing count does not cover the denied UPDATE
   - Verify: AC-15; if A-3 is broken, the AC table gains an explicit cap row before any code is written for it
6. **Phase: Interop** -- the scenario, with its red phase forced
   - Tests: `bgp-route-server-filtered-adj-rib-in-frr`
   - Files: the scenario directory, modelled on `bgp-route-server-frr`
   - Verify: revert phase 3's skip, rebuild the daemon binary the scenario drives, confirm the third party learns the filtered prefix (RED), restore, confirm GREEN, and record the RED output
7. **Phase: RFC ledger** -- the requirement row, the two corrections, the sign-off
   - Tests: `./le rfc check`, `./le rfc extraction-status`
   - Files: `rfc/short/rfc7947.md`, `rfc/extraction/rfc7947.json`, `docs/features/rfc-status.md`
   - Verify: AC-12, AC-13, AC-14; the level ratchet is silent because each correction quotes the RFC verbatim
8. **Phase: Documentation** -- every row of the checklist above that reads Yes
   - Files: as listed in Files to Modify
   - Verify: `./le spec citation anchors spec plan/immediate/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md` names no unlisted owner document

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol, and AC-4 and AC-5 each have a test that FAILS when the guard is removed |
| Feature completeness | The chain from a denied wire UPDATE to `show bgp adj-rib-in` runs with no stub, on the DirectBridge rail and on the forked JSON rail |
| Correctness | `buildReplayRoutes` skips the mark on every branch of its walk, including the replay-cut early return and the same-peer skip; the cursor still advances |
| Correctness | The emit is gated on the peer's `RSClient` setting, not on `RSFastPath`: the fast path is a performance rail and the RFC obligation is about the peer's ROLE |
| Correctness | The teardown branch returns before the emit |
| Naming | The two new JSON keys are lowercase kebab-case; the event type name matches its event kind wire string exactly |
| Data flow | No consumer other than bgp-adj-rib-in subscribes to the new type; verified by reading each of the seven startup-subscription call sites, not by grep |
| Rule: `ai/rules/principles.md` | The guard fails CLOSED: an entry with no mark is treated as accepted only because a filtered entry cannot arrive on the accepted rail. Confirm no path stores from the filtered handler without setting the mark |
| Rule: `ai/rules/rfc-compliance.md` | The RFC 7947 Section 2.1 comment with the quoted sentence sits directly above the emit, and above the replay skip |
| Rule: `ai/rules/interop-and-goal-validation.md` | The interop scenario's RED phase was forced by reverting the change and REBUILDING the artifact, and the RED output is recorded |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The new event type reaches only its subscriber | grep the plugin tree for startup subscriptions and read all seven call sites |
| No filtered entry ever becomes a stored route | `TestBuildReplayRoutesOmitsFilteredEntries` plus `gopls references` on the store field, re-run after the change |
| Both polarities for `RFC7947-2.1-1` | `./le rfc check` |
| rfc7947 signed off | `./le rfc extraction-status` reports rfc7947 as signed |
| Level corrections accepted | `./le rfc check` reports no level-ratchet violation |
| Interop scenario exists and is named, not numbered | list `test/interop/scenarios/bgp-route-server-filtered-adj-rib-in-frr` |
| Spec valid | `./le hook-check validate-spec` exits 0 |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The filtered UPDATE is a message a filter already REFUSED. Its bytes are stored, not trusted: the store keeps hex, and the only consumer that could re-emit it is `buildReplayRoutes`, which must skip it |
| Resource exhaustion | A hostile route-server client can now cost memory with routes that are all denied. A-2, A-3, R-5 and AC-15 settle whether the configured prefix maximum bounds it, and the answer must be proven, not assumed |
| Authorization that could fail open | The receive grant is what admits the new event to a process. A process granted the wildcard gains it implicitly, which is the documented meaning of the wildcard; confirm no fixture grants the wildcard where the filtered population would surprise its assertions |
| Information disclosure | `show bgp adj-rib-in` now shows routes an operator's policy rejected. That is the point of the feature, and the command sits behind the same authz as the accepted population; confirm no new command path bypasses it |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A red in `internal/component/bgp/reactor` naming `api_sync_test.go` or `SignalPeerAPIReady` | Not this spec's: another session owns it. Report it, do not repair it (`ai/rules/principles.md`) |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The receive-grant vocabulary, the subscription grammar and the config completion are all DERIVED from one event registry, so adding a new event type costs a constant and one registration argument, and reaches four surfaces. That is what makes the fail-closed design cheaper than it looks.
- `checkPrefixLimits` running BEFORE the ingress filter chain is not an accident of ordering: it is why RFC 7947 Section 2.1 is affordable at all. A route server that must hold everything a client sends needs its bound applied to what the client SENDS, not to what policy keeps.
- The three existing Correction paragraphs in `rfc/short/rfc7947.md` (2026-07-23, 2026-08-14, 2026-08-15) each cite the RFC sentence that states the true strength. Two more of the same shape land here, which makes five corrections on one document: the extraction walk that produced the original rows read a summary rather than the source.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Deliver the filtered UPDATE on a DISTINCT registered event type | A filtered boolean on the existing `RawMessage`, dispatched on the ordinary update event | Seven plugins subscribe to update received. A boolean makes the correctness of every ingress filter depend on six of them each remembering to check it, and on every plugin added later. That is a guard that fails open, which `ai/rules/principles.md` forbids. A distinct type reaches only a subscriber, so today it reaches exactly one plugin, and a future plugin has to ASK for it |
| Scope the change to peers with `rs-client true` | Every peer | RFC 7947 Section 2.1 binds "a route server", and `rs-client` is the only thing in Ze that makes a peer a route-server client (`ps.RSClient`, `internal/component/bgp/reactor/config.go`). Full compliance with this requirement is the narrow scope, so nothing is traded away. The wide scope would cost: every deployment's Adj-RIB-In grows by its denied population, `show bgp adj-rib-in` changes meaning for every operator, and the memory bound in A-2 would have to hold for peers nobody configured a prefix maximum on. It would BUY a complete pre-policy view for BMP (RFC 7854 Section 5) and for `show bgp adj-rib-in` on an ordinary peer, which are real but are separate obligations under separate documents |
| Carry the denying step's name on the entry | A boolean alone; or a full filter verdict object | `orderedIngressStep.name` already exists and is the operator's answer to "why". A boolean leaves an operator worse off than before, because a route that is present but silent about its status reads as accepted. A verdict object is machinery the problem does not need (`ai/rules/simplicity.md`) |
| Skip filtered entries in `buildReplayRoutes` | A separate store for filtered routes | A second map duplicates the peer keying, the seqmap, the withdrawal walk and the peer-down teardown, and the two would disagree the first time one was edited (`ai/rules/principles.md`, one declaration). One map with one field, and one skip in the one walk that re-advertises, is the smaller and safer shape |
| `RFC7947-x-5` becomes MAY, not a deleted row | Deleting the row; inventing an informational level | `checkRetiredRequirements` refuses a deleted id, and ids are permanent. The level vocabulary is closed at eleven RFC 2119 keywords (`gatedLevels` plus `advisoryLevels`, `internal/le/rfc/rfc.go`), so "informational" is not spellable as a level. MAY is the weakest available and matches Section 2.3.3's "may wish to consider". `rfc/short/rfc3765.md` sets the precedent: a document that gates nothing carries MAY rows |
| Correct `RFC7947-x-4` and `-x-5` in place, keeping the `(Key Requirements)` trailing citation | Re-anchoring them to section-anchored ids | `validateID` anchors an id to the section its line cites, so re-anchoring means new ids, which is renumbering. Naming the section in the `RFC 7947 Section 2.1` form keeps the reader informed while the cross-RFC scrubber removes it before the anchor is read |

## Known Limitations

- BMP Adj-RIB-In Pre-Policy monitoring (RFC 7854 Section 5) still reports the post-policy population. bgp-bmp does not subscribe to the new event type, deliberately: adding it changes what a monitoring station receives and is a separate RFC's obligation.
- A peer that is not a route-server client keeps its post-policy Adj-RIB-In. RFC 4271 Section 3.2 arguably wants the pre-policy view there too; RFC 7947 does not require it, and Key Design Decisions records what changing it would cost.
- Path hiding is still unmitigated. `RFC7947-x-5` is corrected to say so honestly at MAY; it is not implemented here.
- The filtered population is reported but not separately capped. AC-15 proves the configured prefix maximum covers it; a deployment with no configured maximum has the same unbounded exposure it already has for accepted routes.

## RFC Documentation (Scope: protocol)

Add an `// RFC NNNN Section X.Y:` comment with the quoted requirement above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

| Site | Comment content |
|------|-----------------|
| The filtered-event emit in `notifyMessageReceiver` | RFC 7947 Section 2.1, quoting "A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In.", with the tag `RFC7947-2.1-1` |
| The `buildReplayRoutes` skip | RFC 7947 Section 2.1, quoting "These UPDATE messages MAY be omitted from the route server's Loc-RIB or Loc-RIBs, due to filters configured for the purpose of implementing routing policy." -- the sentence that confines the MUST, and the reason a filtered entry is never re-advertised |
| The `RSClient` gate on the emit | A one-line note that Section 2.1 binds a route server, so the obligation follows the `rs-client` role rather than every peer |

New and corrected requirement rows in `rfc/short/rfc7947.md`:

| Requirement | Level | Change | Section it cites |
|-------------|-------|--------|------------------|
| `RFC7947-2.1-1` | MUST | NEW: a route server accepts every client UPDATE for inclusion in its Adj-RIB-In | Section 2.1, in the trailing parenthetical, so the id anchors there |
| `RFC7947-x-4` | MUST becomes SHOULD | Text corrected: the forwarding of UPDATEs from the Loc-RIB to clients is determined by local policy, at Section 2.1's SHOULD strength. The MUST was read out of Section 2.3, which is non-normative | Key Requirements, naming RFC 7947 Section 2.1 in the scrubbed form |
| `RFC7947-x-5` | SHOULD becomes MAY | Text corrected: Section 2.3.3 says authors "may wish to consider" a mitigation, and Section 2.3 is informational | Key Requirements, naming RFC 7947 Section 2.3.3 in the scrubbed form |

Each correction is recorded as a paragraph opening `Correction 2026-08-30:`, naming
its id in backticks, and quoting in double quotes at least 24 characters of the
sentence from `rfc/full/rfc7947.txt` that states the lower strength. For `x-4` that
sentence is Section 2.1's "The route server SHOULD forward UPDATE messages from its
Loc-RIB or Loc-RIBs to its clients as determined by local policy." For `x-5` it is
Section 2.3's "Neither Section 2.3 nor its subsections form part of the normative
specification of this document". Only `x-4` is charged by `checkLevelRatchet`, since
SHOULD is not a gated level; `x-5` carries the same form for the reader.

`rfc/extraction/rfc7947.json` lands with its Section 2.1 site moving from a null
disposition to mapped, pointing at `RFC7947-2.1-1`, with a reason naming the
producing functions. That is the last open site: 24 of 24 sections and 2 of 3 sites
were classified on 2026-08-30, and the artifact is held in this session's scratch
until this spec closes.

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
- [ ] AC-1 through AC-15 all demonstrated
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
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code, tests, docs, spec and learned summary
- [ ] **Commit B:** remove the spec only (commit A preserves it in history)

## Review Gate

<!-- Filled at implementation time by /ze-review. Left empty here by design. -->

### Round 1
| Lens | Scope | BLOCKER | ISSUE | NOTE |
|------|-------|---------|-------|------|

### Round 2
| Lens | Scope | BLOCKER | ISSUE | NOTE |
|------|-------|---------|-------|------|
