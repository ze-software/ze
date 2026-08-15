# Spec: fixit-peer-process-event-filter

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 7/7 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A peer routes its BGP event stream to the processes named in its `process <name> { }`
blocks, and each process is handed its own copy filtered to the event types its
`receive [ ... ]` list names. Two processes on one peer means two independent copies
with two independent filters. A process with no block on a peer gets nothing from
that peer.

That is the design, the documented contract, and what the code did until 2026-01-23.
It is not what the daemon does. Both halves are ignored:

- The ROUTING is ignored. Delivery is decided by what each plugin subscribed to for
  itself at startup, with a nil peer filter, so every plugin receives every peer's
  events whether or not the peer names it.
- The TYPE FILTER is ignored. The eight base tokens (`update`, `open`, `notification`,
  `keepalive`, `refresh`, `state`, `sent`, `negotiated`) are parsed into
  `ProcessBinding` fields that no production code reads.

The config validates, nothing warns, and an operator who writes `receive [ state ]`
gets updates anyway. The goal is to make the config mean what it says.

Owner decision, 2026-08-14: honour it. The filter is uniform over processes, because
a process is a process: ze's own plugins sit in the same config surface as an
operator's application, and an operator who wants their program to see a peer's
updates without ze storing them must be able to say so.

**This is a half-made swap, not a retirement.** Commit `81451c40f` (2026-01-23,
`Co-Authored-By: Claude Opus 4.5`, titled Step 6 of a numbered plan) opens with
"Replace config-driven event routing with subscribe/unsubscribe commands". It changed
the delivery code and declared the old mechanism replaced. It did not remove or
deprecate the YANG leaf, the validator, the completion, the three operator-facing
documents, or any of the 172 files that use the syntax. A deliberate retirement
retires the surface an operator sees; this changed the engine and left the steering
wheel attached. The replacement mechanism is real and widely used (52 `.ci` fixtures
reach it through the plugin ready RPC), so the work here is to decide which of the
two mechanisms is primary and make the other one consistent with it, not to delete
either.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - the document that states the contract
  → Constraint: it promises "One process can serve multiple peers. Each peer-binding
    can have different message types", which is the behaviour this spec restores
  → Decision: the document is stale beyond the promise. It documents an `api <name> { receive { all; } }`
    syntax and the symbols `APIBindings`, `GetPeerAPIBindings`, `Server.OnMessageReceived`,
    none of which exist in the tree. It is rewritten by this spec, not cited as truth
- [ ] `docs/guide/plugins.md` - the operator-facing promise
  → Constraint: "Plugins receive BGP events through process bindings on each peer",
    with a per-plugin Typical Binding table. Both become true or both change
- [ ] `docs/architecture/api/process-protocol.md` - the engine to process event contract
  → Constraint: to be annotated during research
- [ ] `ai/rules/plugins.md` - plugin registration and process boundary rules
  → Constraint: to be annotated during research
- [ ] `ai/rules/config.md` - YANG leaf and validation conventions
  → Constraint: to be annotated during research

### RFC Summaries (Scope: protocol)
Not applicable. This changes which locally configured process is handed a copy of an
event. No wire behaviour, no negotiated capability, no RFC obligation is touched.

**Key insights:** (minimal context to resume after compaction)
- The filter was real from 2025-12-30 (`7cfcc668c`, `8036dd66a`) to 2026-01-23 (`81451c40f`).
- `81451c40f` replaced config-driven routing with plugin-declared subscriptions and
  deleted every read. Its own message says so. The config surface, the docs and 172
  files of usage were left behind.
- The mechanism to restore it already exists and is tested: `Subscription.PeerFilter`
  matches a wildcard, an exact address, a configured peer name, or a negation, and
  `SubscriptionManager.GetMatching` applies it per process.
- No plugin in the tree passes a non-nil peer list. All 15 `SetStartupSubscriptions`
  call sites pass nil, verified by reading each call.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - defines `leaf-list receive` under
  the peer's process block, validated by `ze:validate "receive-event-type"`, no default
  → Constraint: the vocabulary is fixed and closed. The description names the eight base
    types and says plugins may register more. `all` is explicitly not accepted
- [ ] `internal/component/bgp/reactor/config.go` - `parseOneReceiveFlag` parses each token
  → Constraint: unknown tokens are not rejected, they land in `ReceiveCustom`, which is
    the one part of the list that still does something
- [ ] `internal/component/bgp/reactor/peersettings.go` - `ProcessBinding`, the parsed shape
  → Decision: the struct is half live. `PluginName` drives config-graph edges and
    plugin-reference validation, `SendUpdate` drives the per-session API sync count and a
    config validation, `ReceiveCustom` and `SendCustom` drive plugin auto-loading. The
    eight base receive booleans and the content fields are the dead half
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `GetPeerProcessBindings`, the only
  production read of the dead fields
  → Constraint: it resolves per-peer `content { encoding format }` inheritance too, so
    that binding is inert for the same reason and by the same route
- [ ] `internal/component/plugin/coordinator.go` - `Coordinator.GetPeerProcessBindings`
  → Decision: the sole wrapper, and it has no production caller. The accurate statement is
    that the whole method is dead, not only the two fields
- [ ] `internal/component/plugin/server/subscribe.go` - `Subscription`, `PeerFilter`, `SubscriptionManager`
  → Decision: `PeerFilter.Matches` accepts `*`, an exact address, an exact configured peer
    name, or a `!x` exclusion, and `Subscription.Matches` applies it after namespace, event
    type and direction. This is the mechanism the restored filter uses
  → Constraint: `Subscription.PluginFilter` is parsed and compared in `Equals` but never
    read by `Matches`, so the `subscribe plugin <name>` selector filters nothing. Adjacent
    defect, not this spec's, but it must not be mistaken for a working precedent
- [ ] `internal/component/plugin/server/dispatch.go` - `registerSubscriptions`, the producer
  that turns a plugin's ready RPC into subscriptions
  → Constraint: it reads only the first entry of the peers list and drops the rest, and
    `Format`, `Encoding` and `Envelope` are per-process last-writer-wins. A config-driven
    filter that needs several peers per process must add one subscription per peer
- [ ] `internal/component/bgp/server/events.go` - **SEVEN** peer-scoped delivery entry
  points, not three. Corrected 2026-08-14 and verified by reading each call:
  `onMessageReceived`, `onMessageBatchReceived`, `onPeerStateChange`, `onPeerNegotiated`,
  `onEORReceived`, `onMessageSent`, `onPeerCongestionChange`
  → Constraint: received UPDATEs take the BATCH path, which fans out fire-and-forget with no
    result channel. Any filter must apply before that enqueue, not after
  → Decision: all seven pass `peerAddr` and `peer.Name` to `GetMatching`, so a per-peer
    filter has the inputs it needs at every point of decision
  → Constraint: all seven funnel into `deliverToProcs`, which is the one place a graph
    lookup can replace a subscription scan without touching seven call sites
- [ ] `internal/component/plugin/server/startup_autoload.go` - custom tokens auto-load plugins
  → Constraint: `receive [ update-rpki ]` auto-loads `bgp-rpki-decorator`. The list already
    has a second job, and restoring the filter must not break it
- [ ] `internal/component/bgp/config/resolve.go` - `ResolveBGPTree`, the group-to-peer merge
  → Decision: a GROUP can carry `process` blocks legally: `list process` lives in the
    `peer-fields` grouping, which `bgp/group`, `bgp/group/peer` and `bgp/peer` all use, and
    `test/exabgp-compat/native/parse-process.conf` does it. Group bindings merge into a
    member by process NAME through `deepMergeMaps`
  → Constraint: `receive` is a leaf-list and is not in `cumulativePaths`, so a peer that
    restates a process block REPLACES the group's list rather than adding to it. The spec
    must state that semantic, because an operator will expect one or the other
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` - `buildDynamicPeerSettings`
  → Decision: a dynamic peer DOES carry its group's process bindings. The template is
    copied by value and the divergence set does not include them, which
    `TestDynamicPeerInheritsEveryPeerSettingsField` pins. The 2026-07-27 deferral row that
    said otherwise is resolved and marked done, closed by `spec-fixit-dynamic-group-peer-config`
  → Constraint: what a dynamic peer lacks is an identity the config can name. Its `Name` is
    `dyn-<addr>`, which appears in no config document, so a config-derived edge keyed on the
    GROUP cannot match it until the match learns about groups
- [ ] `internal/component/config/graph.go` - `addProcessBindings`
  → Constraint: it reads the peer's own subtree and never the group's, so it misses every
    inherited binding. It feeds `ze config graph` only, not delivery, but the delivery graph
    must be derived from RESOLVED settings rather than from this structure

**The block has two directions and NEITHER is honoured** (owner statement, 2026-08-14).
`receive` is what the process wants to see. `send` is what kind of BGP message that
process is allowed to GENERATE toward the peer it is linked to. It is a permission,
not a subscription, and it is not enforced: `ProcessBinding.SendUpdate` has two
production readers, the parser that sets it and `validatePeerProcessCaps`
(`internal/component/bgp/config/peers.go`), which only asserts that SOME binding on the
peer carries it when route-refresh or graceful restart is configured. What a process may
generate is decided by the peer selector inside the command it sends: `AnnounceEOR` and
the announce path resolve `ctx.PeerSelector()` through `getMatchingPeersSel`, which is
DEFINED on `reactorAPIAdapter` in `internal/component/bgp/reactor/reactor_api.go`
(corrected 2026-08-14; the forward file holds two of its callers, not the definition)
and which consults no binding. It has six production callers: `AnnounceEOR` and
`sendRouteRefresh` in `reactor_api_forward.go`, `AnnounceNLRIBatch`, `WithdrawNLRIBatch`
and `SendRoutes` in `reactor_api_batch.go`, and `SoftClearPeer` in `reactor_api.go`. A
process sending `peer * update ...` reaches every peer, including peers that never named
it. Enforcing only the inbound half would leave that open, so both directions are in
scope and the graph carries both edges.

**Behavior to preserve:**
- The API sync count at session establishment, which reads the same `SendUpdate` field,
  and the `validatePeerProcessCaps` assertion. Both keep working; they gain an enforcement
  point rather than losing their current one.
- Custom-token auto-loading through `ConfiguredCustomEvents` and `ConfiguredCustomSendTypes`.
- The runtime `subscribe`/`unsubscribe` command surface. An operator command must still be
  able to change what a running process receives.
- Every event that is not peer-scoped. The filter is per peer, so system events are untouched.

**Behavior to change:**
- Delivery of peer-scoped events is decided by the peer's process blocks: which processes,
  and which event types each one is handed.
- A process with no block on a peer receives nothing for that peer.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A BGP message arrives on a session, or a session changes state.
- Config entry: the peer's `process <name> { receive [ ... ] }` block, parsed at load and reload.

### Transformation Path
1. Wire receive and parse in the reactor's session path.
2. Notification to the plugin server, through the batch path for UPDATEs and the single path
   for other messages, plus the state path for session changes.
3. Subscription match per process, which is where the filter belongs.
4. Enqueue onto the matched process's own event channel, then delivery on that process's loop.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → reactor | `ProcessBinding` per peer, parsed from YANG, now carrying `ReceiveAll`, `Receive` per type with direction, `SendAll` and `Send` | Yes: `TestEveryStartupSubscriptionIsExpressible` maps all 15 in-tree plugin declarations to config text and asserts the parse |
| Reactor → plugin server | peer address and configured peer name on every event; the index is published from resolved settings under one atomic pointer write | Yes: `TestGraphSwapIsAtomicAcrossReload` under `-race`, and `test/plugin/attach-process-reload.ci` asserts no surviving edge misses an event across a reload |
| Plugin server → process | the graph lookup intersected with the subscription match, then the per-process channel | Yes: `test/plugin/attach-process-receive-filter.ci` and `attach-process-unattached-is-silent.ci`, both proven to discriminate by returning the process set unfiltered |
| Reactor → peer wire | the send permission at `getMatchingPeersSel`, resolved per matched peer | Yes: `test/plugin/attach-process-send-permission.ci` plus `TestSendPermissionRefusesUnattachedPeer`, discrimination proven by short-circuiting the filter |

### Integration Points
- `SubscriptionManager` gains config-derived subscriptions beside plugin-declared ones.
- Config reload must re-derive them, because a peer's process block can change at runtime.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | All seven peer-scoped entry points in `internal/component/bgp/server/events.go` reach delivery through the single funnel `(*Server).PeerScopedProcs`. The filter sits at the lookup, before the batch path's fire-and-forget enqueue, so no path reaches a process without passing it |
| No unintended coupling (components stay isolated) | Yes | The index lives in the plugin server and is produced by the reactor from RESOLVED settings, never from the config tree. `plugin.Sender` was placed in `internal/component/plugin` for the same reason: the generic plugin server must not import BGP and `bgptypes` must not import the server. `ze config graph` was deliberately NOT extended, because it is an offline reader in a tier that must not import the BGP resolver, an import that used to pin `internal/component/bgp` into every binary |
| No duplicated functionality (extends existing, does not recreate) | Yes | The graph is an INDEX over subscriptions, not a second registry beside them. The subscription stays the unit of registration, the config became its primary producer, and `Inspect` reads its tokens back OUT through `Receivers` so the operator surface cannot disagree with delivery. One concept, two producers, one lookup |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `Receivers` returns a STORED slice and the delivery path treats it as read-only. `TestGraphLookupAllocatesNothing` measures 0 allocations on both the hit and the miss path, and requires edges to be returned first so the zero is not vacuous. `TestPeerScopedProcsAddsNoAllocation` shows the filter adds none over `GetMatching`'s own 2 |
| Registration over hardcoding | Yes | The token grammar resolves against the event-type registry rather than a hardcoded list, which is what makes registry-first direction parsing safe. `ze:flatten` is a YANG extension read from the schema, not a path special-cased in the serializer. No plugin name appears in a core package. The wildcard expands over the registry at build time |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The filter is uniform over processes, with no exemption for ze's own plugins | Owner, 2026-08-14: a process is a process, and an operator may want their app to see updates without the host routing table being updated | An exemption list is needed, and the config stops being the whole truth | Owner confirmation, recorded here | validated |
| A-2 | Every event type an in-tree plugin needs is expressible in the leaf's vocabulary | `ze-bgp-conf.yang` names eight base types plus registered custom types | Some plugin cannot be configured to work, and the vocabulary needs extending | Enumerate the subscriptions of all 15 `SetStartupSubscriptions` call sites against the vocabulary | **BROKEN 2026-08-14, REPAIRED by phase 2.** All 15 were read: ten could not be stated, eight for want of a direction and two for want of the wildcard. The vocabulary now carries both, and all 15 are stated and parsed in `TestEveryStartupSubscriptionIsExpressible` (`internal/component/bgp/reactor/config_test.go`), each row read from the plugin's own call |
| A-3 | The peer name a subscription filter matches is the same name an operator writes in config | `PeerInfo.Name` is copied from `PeerSettings.Name` | Name selectors silently match nothing | Functional test using a name selector, not an address | unvalidated. Phase 4 removed the graph's dependence on it: the index is keyed on the peer ADDRESS, `PeerSettings.Address.String()`, which is the same string `peer.addrString` and `PeerInfo.AddressStr` carry (`peer.go` `NewPeer`, `reactor_notify.go`). **CONFIRMED 2026-08-15** for the one scope it still governs, the runtime `subscribe` command's name selectors. `PeerFilter.Matches` (`internal/component/plugin/server/subscribe.go`) compares `peerName == pf.Selector`, and the delivery path passes `peer.Name`, which `peer.go` derives from `settings.Name`, the config key an operator writes. The name reaches the comparison unchanged, so a name selector matches the peer the operator meant. No assumption in this table is now `unvalidated` |
| A-4 | Config reload can re-derive subscriptions without dropping events mid-reload | To be established | A reload window silently loses events | Reload functional test asserting no gap | **confirmed 2026-08-15, phase 5.** The swap half is `TestGraphSwapIsAtomicAcrossReload` (4 readers, 200 republishes, `-race`). The delivery half is `test/plugin/attach-process-reload.ci`: the edge that survives the reload is fed both before and after it, and the edge the reload adds refuses any event that reaches it before the SIGHUP. A peer whose attach block changes is torn down and re-added, so the reload also republishes through `AddPeer` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Our own configs name lists written while they meant nothing, so honouring them breaks tests that are internally contradictory. **Measured 2026-08-14: 691 peer and group process bindings across 505 files.** 533 SAFE, 67 with no `receive` line at all, 50 that narrow below what their plugin subscribes to, 41 unresolvable | The plugin suite reds en masse on the first honouring build | Correct the configs in their own commit, before the behaviour change lands, so a revert of the switch does not also revert the corrections |
| R-1b | **The edit surface depends entirely on the strictness choice.** Under intersection it is 99 files and 117 occurrences. Under replacement it is all 691, because 463 of the 533 SAFE rows are safe only in the sense that their process subscribes to NOTHING today: `registerSubscriptions` is reached only when a plugin actually calls subscribe, and only 52 of 633 `.ci` files under `test/plugin` do | The correction pass runs 6x longer than planned | Settle strictness before sizing the correction phase, and state the number the chosen reading implies |
| R-2 | ~~A dynamic-group peer cannot be MATCHED by a config-derived edge~~ | Dynamic peers stop reaching plugins the moment the filter is live | **FALSE, and phase 7 falsified it at the producer, 2026-08-15.** No group key was needed and none was added. The risk assumed the match would be keyed on a config IDENTITY; the index is keyed on the peer ADDRESS, and a dynamic peer has one. Three producers carry the group's block to it: `ParseDynamicGroupTemplate` (`bgp/reactor/config.go`) parses the group's `attach` block with the SAME parser a static peer's goes through; `buildDynamicPeerSettings` (`bgp/reactor/reactor_dynamic.go`) copies the template whole and `ProcessBindings` is not in its 7-field divergence set, which `TestDynamicPeerInheritsEveryPeerSettingsField` refuses to let anybody add; `publishDeliveryGraphLocked` reads `r.peers`, which `createDynamicPeer` has already written. `PeerInfo.GroupName` stays where it was, and `PeerFilter.Matches` is untouched. Pinned by `TestDynamicPeerEntersTheIndexUnderItsOwnAddress`, which also asserts a lookup on the generated `dyn-<addr>` name finds NOTHING: the name is not a key and does not need to be |
| R-3 | `registerSubscriptions` reads only the first peer selector and drops the rest | A process bound to several peers sees only one | One subscription per peer binding, verified by a test with two peers on one process |
| R-4 | Content settings are per-process last-writer-wins, so per-peer `content { encoding format }` cannot be honoured by the same move | Two peers with different encodings for one process disagree | Out of scope, and stated as such. The per-peer content binding stays inert and is named in Known Limitations |
| R-5 | The runtime `subscribe` command and the config-derived filter can disagree about what a process receives | An operator subscribes at runtime and the config overrides it, or the reverse | Decide and document precedence explicitly, with a test for each direction |
| R-6 | ~~An operator upgrading ze finds plugins stop receiving events~~ **RETIRED 2026-08-14: ze is not released, so there is no installed base to break.** What survives is the diagnosis half: a config whose peers attach no process produces a silent daemon, and silence is indistinguishable from working | A first-run config feeds nothing and nothing says why | The startup summary naming every peer-to-process pairing the config produces. It is now a usability requirement for a new operator rather than an upgrade cushion |
| R-7 | A reload that rebuilds the index drops events for edges that survive the reload | A functional test that sends during a reload sees a gap | Build the new index, then swap it under a single pointer write. Readers take a snapshot, so no reader sees a half-built index and no surviving edge misses an event |
| R-8 | The lookup allocates per event, reintroducing the cost this design exists to remove | The alloc gate reds, or a benchmark shows an allocation per delivered event | The index returns a stored slice, and the delivery path treats it as read-only. Covered by an allocation test, not by inspection |
| R-9 | A config grants an event type the plugin never declared, so the operator believes a filter is in force that the program cannot act on | Nothing today. This is the failure mode that has to be given a voice | Reconcile at plugin ready, when both halves are known, and report peer, process and event type. Whether that is a warning or a refusal is a design-gate decision |
| R-10 | The runtime `subscribe` command and the config graph fight, and the last writer wins by accident | An operator subscribes, reloads, and the subscription vanishes with no message | Precedence stated in the spec and documented for the operator: config is durable truth rebuilt on every reload, a runtime subscription is a live override that a reload discards. Both directions get a test |
| R-11 | A hyphenated direction token collides with a hyphenated custom event type, so `receive [ update-rpki ]` is misread as `update` in some direction | A custom-token config stops auto-loading its plugin, or a type resolves to the wrong direction | **Implemented in phase 2.** `events.SplitTypeToken` (`internal/core/events/token.go`) resolves the whole token against the registry and splits only what does not resolve. Proven at the producer by `TestSplitTypeTokenRegistryWinsOverSplit` and at the config parser by `TestParseReceiveTokenRegistryWinsOverDirectionSplit`; both register a type ending in `-sent` beside its base name, so a splitter-first reading fails them |
| R-12 | The reconciliation report at plugin ready is noise on a large config, so operators learn to ignore it | Every start prints hundreds of lines | Report one line per disagreeing peer-process-type triple, not per edge, and say nothing when the two halves agree |
| R-15 | **A plain token grants BOTH directions, and both directions DEADLOCK three in-tree plugins.** `internal/component/bgp/plugins/rr/rr.go` carries the mechanism verbatim: subscribing to update in both directions is "a circular deadlock (ForwardUpdate to onMessageSent to deliver to block)". `bgp-rs` has the same rationale. So a config granting `update` to `bgp-rr`, `bgp-rs` or `bgp-rpki` is not a wrong filter, it is a hang, the moment phase 5 makes delivery honour it | Nothing today. Phase 5 turns it into a daemon that stops rather than a test that fails, which is the worst failure shape in this spec | **CLEARED by phase 3, 2026-08-15.** No config in the tree grants an update in the sent direction to `bgp-rr`, `bgp-rs`, `bgp-rpki` or `bgp-rpki-decorator`, and `TestNoConfigFeedsSentUpdatesToAReceivedOnlyPlugin` (`internal/component/bgp/reactor/config_direction_test.go`) walks every `.ci`, `.conf`, `.md` and `.et` under `test/`, `docs/`, `demos/` and `contrib/` and refuses one. It resolves each token through `events.SplitTypeToken`, the grammar's own producer, and it refuses `receive [ * ]` for the same four. Proven to discriminate: rewriting `test/plugin/rpki-as-set.ci` back to `receive [ update ]` fails it with the file, the line, the plugin and the reason. Also corrected in prose: `docs/guide/plugins.md`, `docs/guide/route-reflection.md` and `docs/guide/rpki.md` told operators to write the plain token |
| R-14 | **A `peer *` send from a plugin script goes silently nowhere once phase 6 lands.** A test whose process calls `send('peer * update ...')` reaches every peer today, because the send permission is unenforced. After phase 6 it reaches only peers that attached that process with `send`. A fixture whose peer block carries no attach line, or an attach line with no `send`, then announces to nobody | The announce succeeds, the peer receives nothing, and the failure reads as a routing bug rather than a missing permission | **DONE by phase 3, 2026-08-15.** The send side was driven from what each script CALLS, never from its config: `send [ update ]` where the script issues `peer <sel> update ...`, an ExaBGP `announce`/`withdraw`, a named commit or a cache forward; `send [ refresh ]` where it dispatches `request peer <sel> refresh`. 28 interop scenarios announced routes through `flush()` with no send permission at all and now carry one. `test/plugin/med-locally-set-reaches-peer.ci` was left untouched: its `send update` was already true |
| R-13 | **The two-word keyword defeats the CLI editor's key heuristic, silently.** `internal/component/cli/model_load.go` builds a block key from the first word when it is in a hardcoded `blockKeywords` map, and from the first two words otherwise. `attach` is not in that map, so every `attach process <name>` block on a peer collapses to the single key `attach` and the process name is lost | The editor shows one process block per peer where the config has several, and a commit drops the others | Verified by reading the producing function, 2026-08-14. Teach the heuristic the two-word form and key on all three words. A unit test loads a peer carrying two attached processes and asserts both survive a load-edit-commit round trip |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Plugins stop receiving events they need. For ze's own plugins that means no RIB, no route server, no BMP feed for the affected peer. Nothing is mis-encoded on the wire, but the daemon can go quiet |
| How is it reverted? | Single commit revert, provided the fixture corrections land as their own commit first |
| Who else touches this path? | `plan/spec-fixit-stored-route-relay-hardening.md` (in-progress) reads the same dead method for the per-peer `content { format }` binding and needs to know the answer |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer config attaching a process with `receive [ state ]` | → | `Server.PeerScopedProcs`, the one lookup the seven peer-scoped delivery sites make | `test/plugin/attach-process-receive-filter.ci` (phase 5, PASS) |
| Peer config attaching no block for a running plugin | → | `Server.PeerScopedProcs` | `test/plugin/attach-process-unattached-is-silent.ci` (phase 5, PASS) |
| Group config inherited by a dynamic peer | → | per-peer binding derivation from resolved settings | `test/plugin/attach-process-dynamic-group.ci` (phase 7, PASS) |
| Process announcing toward a peer that did not attach it | → | the send permission at the peer-selector resolver | `test/plugin/attach-process-send-permission.ci` |
| A config still spelling the retired `process <name>` | → | the parser's keyword refusal | `TestParseRefusesRetiredProcessKeyword` |
| Config load and reload | → | the delivery graph index the plugin server holds | `TestConfigLoadPublishesDeliveryGraph`, `TestReloadRepublishesDeliveryGraph` (phase 4, PASS) |
| `show event delivery` | → | `DeliveryGraph.Inspect` | `test/plugin/attach-process-delivery-graph.ci` (phase 4, PASS) |

## Acceptance Criteria

<!-- AC-6 and AC-7 wait on the group-inheritance and vocabulary sweeps. -->

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer names two processes with different `receive` lists | Each process is handed only the event types its own list names, from the same peer, independently of the other |
| AC-2 | A running plugin has no `process` block on a peer | That plugin receives no events for that peer, and receives events for peers that do name it |
| AC-3 | A peer's list omits an event type the plugin subscribed to for itself | The process is not handed that event type for that peer |
| AC-4 | A config reload changes one peer's `receive` list | Delivery follows the new list after the reload, and every edge that survives the reload misses no event during it |
| AC-5 | An operator issues the runtime `subscribe` command, then reloads the config | The subscription takes effect immediately, and the reload restores the graph the config describes |
| AC-6 | A peer created from a dynamic group whose group names a process with a `receive` list | That peer's events reach that process, filtered by the group's list, without the config naming the peer's generated `dyn-<addr>` identity. **MET, phase 7. No group-keyed match exists or is needed (R-2): the member inherits its group's bindings and enters the index under the address its connection arrived from.** `TestDynamicPeerEntersTheIndexUnderItsOwnAddress` and `test/plugin/attach-process-dynamic-group.ci` |
| AC-6b | A group names a process, and one member peer restates the same process with its own `receive` list | The member's list replaces the group's for that member, and other members keep the group's. **MET: `TestGraphMemberListReplacesGroupList` at the merge (phase 4), and `attach-process-dynamic-group.ci` at delivery (phase 7), where the member that restates is fed its own list and the member that states nothing is fed the group's** |
| AC-7 | A config grants a process an event type its plugin never declared | That type is not delivered, and a reconciliation report at plugin ready names the peer, the process and the event type. The daemon does not refuse the config, because the plugin's declaration arrives after config load |
| AC-7b | A plugin declares an event type for a peer whose config does not grant it | That type is not delivered for that peer, and the same reconciliation report names it. The config is the authority on what a process gets |
| AC-8 | Any peer-scoped event is delivered | The lookup that decides the recipients performs no allocation per event |
| AC-9 | A process attached to peer A with `send [ update ]` issues an announce whose selector names peer B, which has not attached it | The announce reaches A and not B, and the refusal for B is reported rather than silent |
| AC-10 | A process attached to a peer with no `send` line issues an announce toward it | The announce is refused, and the refusal names the peer and the process |
| AC-11 | A peer attaches a process with `send [ update ]` and no `receive` line | The process may announce to that peer and is fed nothing from it |
| AC-12 | A config written with the pre-rename keyword `process <name>` | Refused at parse with a message naming `attach process`. An ExaBGP config carrying `process` still converts, with the converter emitting the new keyword |
| ~~AC-12b~~ | ~~An operator's on-disk ze-native config carrying `process <name>` is run through the ze config migrator~~ | **WITHDRAWN 2026-08-14.** Owner: ze is not released and needs no backward compatibility. The parser refuses the retired keyword and nothing upgrades a file. AC-12 carries the whole behaviour |
| AC-13 | A peer attaches two processes and the config is loaded, edited and committed through the CLI editor | Both blocks survive the round trip, each keyed by its own name |

## End-to-End User Stories

Owner-approved at the design gate, 2026-08-14. The configuration text below is the
operator input format, not an implementation sketch. Each story states what the
daemon does today so the change is legible.

Both stories RUN, as of phase 4: `TestStoryOneProducesItsDeliveryTable` and
`TestStoryTwoProducesItsTwoIndependentDirections`
(`internal/component/bgp/config/delivery_graph_test.go`) load these configs and
assert the tables printed beside them, edge by edge. The tests add the
`connection` and `session` leaves every peer needs and that the stories elide;
nothing else differs. The middle columns are the graph's edges, not delivery,
which still runs the old way until phase 5.

### Story 1: two programs, six peers, one group

A program is declared once, under `plugin { external <name> { run ... } }`. Each peer
then attaches it and states the relationship for that peer.

```
plugin {
    external looking-glass  { run ./looking-glass.py }   # observes, never speaks
    external route-injector { run ./injector.py }        # originates routes
}

bgp {
    group transit {
        attach process looking-glass { receive [ update state ] }

        peer 192.0.2.1 { }
        peer 192.0.2.2 { }
        peer 192.0.2.3 {
            attach process looking-glass { receive [ state ] }
        }
    }

    peer 198.51.100.1 {
        attach process looking-glass  { receive [ update state ] }
        attach process route-injector { send [ update ] }
    }

    peer 198.51.100.2 {
        attach process route-injector { receive [ state ] send [ update ] }
    }

    peer 203.0.113.1 { }
}
```

| Peer | looking-glass is fed | route-injector is fed | route-injector may announce to it |
|------|----------------------|-----------------------|-----------------------------------|
| 192.0.2.1 | updates and state, from the group | nothing | no |
| 192.0.2.2 | updates and state, from the group | nothing | no |
| 192.0.2.3 | state only, its line replaces the group's | nothing | no |
| 198.51.100.1 | updates and state | nothing | yes |
| 198.51.100.2 | nothing | state only | yes |
| 203.0.113.1 | nothing | nothing | no |

The two injector lines carry the point of the pair. On 198.51.100.1 it is write-only:
it announces routes and is told nothing, not even whether the session is up. On
198.51.100.2 it also asks for `state`, because a program that originates routes
usually wants to know when the peer comes up so it can push its table. Both are
legal, and one word separates them.

Today every cell in the last column reads yes for both programs on all six peers,
because `send` is never checked. Both middle columns read "whatever the program asked
for at startup" on all six peers, because `receive` is never read.

### Story 2: one peer, both directions

A policy engine reads what a peer sends and answers it.

```
plugin {
    external policy-engine { run ./policy-engine.py }
}

bgp {
    peer 198.51.100.3 {
        attach process policy-engine {
            receive [ update state ]
            send    [ update ]
        }
    }
}
```

One block, two independent directions. `receive [ update state ]` is the inbound edge:
ze hands `policy-engine` a copy of every UPDATE arriving from 198.51.100.3 and tells
it when that session goes up or down, and it is fed nothing about any other peer.
`send [ update ]` is the outbound permission: `policy-engine` may originate
announcements and withdrawals toward 198.51.100.3, may send that peer nothing else,
and may send no peer that carries no such line.

The two lists are independent, which is what makes the pair expressive. The same
program on a different peer is a silent observer with `receive [ update ]` alone, fed
that peer's routes and forbidden from answering it, or write-only with `send [ update ]`
alone, allowed to announce and told nothing. One instance of the program runs, and its
relationship to each peer is stated per peer.

Today all three shapes behave identically: the program is fed whatever it asked for at
startup from every peer, and may announce to all of them, because neither line is
consulted.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseRefusesRetiredProcessKeyword` | `internal/component/bgp/reactor/config_test.go` | phase 1: the bare `process` keyword is refused with a message naming `attach process` | |
| `TestExabgpConverterEmitsAttachProcess` | the ExaBGP converter's package test | phase 1: an ExaBGP config spelling `process` converts to `attach process` | |
| `TestParseReceiveDirectionTokens` | `internal/component/bgp/reactor/config_test.go` | phase 2: a direction-carrying token parses to the right type and direction, and a plain token means both | PASS |
| `TestReceiveEventValidatorAcceptsDirectionAndWildcard` | `internal/component/config/validators_test.go` | phase 2: the YANG validator accepts the new vocabulary and still refuses an unregistered type | PASS |
| `TestSplitTypeTokenRegistryWinsOverSplit` | `internal/core/events/token_test.go` | phase 2: R-11, the whole token is resolved before a direction suffix is cut | PASS |
| `TestEveryStartupSubscriptionIsExpressible` | `internal/component/bgp/reactor/config_test.go` | phase 2: A-2, all 15 in-tree startup declarations are stateable in a receive list | PASS |
| `TestNoConfigFeedsSentUpdatesToAReceivedOnlyPlugin` | `internal/component/bgp/reactor/config_direction_test.go` | phase 3: R-15, no config in the tree feeds bgp-rr, bgp-rs, bgp-rpki or bgp-rpki-decorator the UPDATEs ze sends | PASS |
| `TestGraphFeedsOnlyAttachedProcesses` | `internal/component/plugin/server/delivery_graph_test.go` | phase 4: a peer's edges name exactly the processes its config attaches | PASS |
| `TestGraphMemberListReplacesGroupList` | `internal/component/bgp/config/delivery_graph_test.go` | phase 4: AC-6b, the member's list replaces the group's and other members keep the group's. It lives with `ResolveBGPTree`, the producer of the merge, because a test built on hand-made bindings asserts nothing about inheritance | PASS |
| `TestGraphInheritsGroupAttachBlock` | `internal/component/bgp/config/delivery_graph_test.go` | phase 4: a peer that states no block of its own is fed through its group's, which is what building from RESOLVED settings buys over the config document | PASS |
| `TestGraphLookupAllocatesNothing` | `internal/component/plugin/server/delivery_graph_test.go` | AC-8: the per-event lookup returns a stored slice and allocates zero. Measured 0 on the hit path and 0 on the miss path | PASS |
| `TestGraphSwapIsAtomicAcrossReload` | `internal/component/plugin/server/delivery_graph_test.go` | R-7: a reader takes a snapshot, so no reader sees a half-built index. Run under `-race` | PASS |
| `TestGraphWildcardGrantsEveryRegisteredType` | `internal/component/plugin/server/delivery_graph_test.go` | phase 4: `*` expands over the registry at build time, and a type named beside it keeps both directions | PASS |
| `TestGraphReportsUnresolvedTokens` | `internal/component/plugin/server/delivery_graph_test.go` | phase 4: a token the registry does not know carries no edge and is named where an operator sees it | PASS |
| `TestEmptyGraphFeedsNobody` | `internal/component/plugin/server/delivery_graph_test.go` | phase 4: the guard fails closed. A server that has applied no config feeds nothing, and the accessor never hands back nil | PASS |
| `TestConfigLoadPublishesDeliveryGraph` | `internal/component/bgp/reactor/delivery_graph_test.go` | phase 4 WIRING: the config load path publishes the index into the plugin server | PASS |
| `TestReloadRepublishesDeliveryGraph` | `internal/component/bgp/reactor/delivery_graph_test.go` | phase 4 WIRING and AC-4's first half: a reload's remove-and-add republishes, and an untouched peer keeps its edges | PASS |
| `TestPeerScopedProcsIsTheOverlapOfBothHalves` | `internal/component/plugin/server/delivery_filter_test.go` | phase 5: AC-1, AC-3 and AC-7 at the producer. Every process subscribes to everything, so only the config can tell them apart | PASS |
| `TestPeerScopedProcsFeedsNobodyForAnUnattachedPeer` | `internal/component/plugin/server/delivery_filter_test.go` | phase 5: AC-2 both ways, and the empty index still feeds nobody | PASS |
| `TestPeerScopedProcsAddsNoAllocation` | `internal/component/plugin/server/delivery_filter_test.go` | phase 5: AC-8 over the whole funnel. MEASURED: `GetMatching` alone 2 allocations, filtered 2 -- the filter adds none | PASS |
| `TestRuntimeSubscribeSurvivesAPublishThatIsNotAnApply` | `internal/component/plugin/server/delivery_filter_test.go` | phase 5: AC-5 and R-10, the override direction. It REPLACES `TestRuntimeSubscribeOverridesTheConfigUntilTheNextApply`, which asserted the discard against `UpdateDeliveryGraph` and so pinned the defect a dynamic peer connecting used to cause. Discrimination: moving the discard back into that publish fails it | PASS |
| `TestConfigApplyDiscardsRuntimeSubscriptions` | `internal/component/bgp/reactor/delivery_graph_test.go` | phase 5: AC-5 and R-10, the discard direction, driven from `reconcilePeers` because `reconcilePeersJournaled` is the only caller of `DiscardRuntimeSubscriptions`. Discrimination: deleting that call fails it | PASS |
| `TestEmittedPeerScopedEventIsFilteredByTheGraph` | `internal/component/plugin/server/delivery_filter_test.go` | phase 5: the emit-event rail, the second producer of peer-scoped delivery. Drives `deliverEvent` with a peer address and without one. Discrimination: restoring the direct `GetMatching` call fails it | PASS |
| `TestCacheForwardEntryPointRefusesAnUnattachedProcess` | `internal/component/bgp/reactor/send_permission_rails_test.go` | phase 6: AC-9 and AC-10 on the `ze-bgp:cache-forward` rail, dispatched through the registered command over a real plugin server and real peers. Deny and accept differ only in the process name | PASS |
| `TestPeerRawEntryPointRefusesAnUnattachedProcess` | `internal/component/bgp/reactor/send_permission_rails_test.go` | phase 6: the same for `peer <addr> raw`, gated on attachment alone. The refused case writes nothing to the peer's socket | PASS |
| `TestForwardCachedRailRefusesAnUnattachedProcess` | `internal/component/bgp/reactor/send_permission_rails_test.go` | phase 6: the forward-cached rail refuses a destination that does not attach the process and dispatches nothing | PASS |
| `TestRelayStoredRouteRailRefusesAnUnattachedProcess` | `internal/component/bgp/reactor/send_permission_rails_test.go` | phase 6: the relay rail, same shape. Its plugin-server entry point cannot be called from the reactor package, since the dependency runs the other way | PASS |
| `TestCachedRailsNameTheProcessAsTheSender` | `internal/component/plugin/server/dispatch_cached_sender_test.go` | phase 6: the half the reactor cannot see. `forwardCached` and `opRelayStoredRoute` hand the guard a `ProcessSender` naming the calling process, never the operator | PASS |
| `TestRelayStoredRouteFromAnUnnamedCallerStaysUnnamed` | `internal/component/plugin/server/dispatch_cached_sender_test.go` | phase 6: `procSender(nil)` stays the zero Sender, which the rail refuses, rather than being read as the operator | PASS |
| `TestDispatchBGPPeerRaw` | `internal/component/bgp/plugins/cmd/raw/dispatch_test.go` | phase 6: the raw dispatch chain carries the issuer's identity. It used to dispatch with the zero Sender and assert success, which documented the opposite of the rule | PASS |
| `TestDispatchBGPPeerRawCarriesAProcessIdentity` | `internal/component/bgp/plugins/cmd/raw/dispatch_test.go` | phase 6: a process identity arrives as that process, neither dropped nor upgraded to the operator's exemption | PASS |
| `TestReconcileNamesEachDisagreementAndIsSilentOnAgreement` | `internal/component/plugin/server/delivery_filter_test.go` | phase 5: AC-7, AC-7b, the direction pair that never meets, and R-12's silence | PASS |
| `TestSendPermissionRefusesUnattachedPeer` | `internal/component/bgp/reactor/send_permission_test.go` | phase 6: AC-9 and AC-10, the selector resolver drops a peer that did not attach the process, and reports it. It lives with the FEATURE rather than in `reactor_api_forward_test.go` as this table first said: the tests drive four entry points across three source files, and that file owns one of them | PASS |
| `TestSendPermissionSeparatesUpdateFromRefresh` | `internal/component/bgp/reactor/send_permission_test.go` | phase 6: the six selector-resolving commands are gated on the message type each puts on the wire. `send [ refresh ]` must not permit an announce, and `send [ update ]` must not permit a ROUTE-REFRESH | PASS |
| `TestSendPermissionDoesNotGateAnOperator` | `internal/component/bgp/reactor/send_permission_test.go` | phase 6: the guard's population. A CLI, SSH or REST command carries no process and is not gated, so a peer that attaches nothing still answers `bgp peer <addr> refresh` | PASS |
| `TestPeerUpSignalsSessionReady` | `internal/component/bgp/plugins/watchdog/server_test.go` | phase 6: bgp-watchdog declares its initial contribution complete on every peer-up, which is what releases the peer's End-of-RIB hold once its send grant is truthful | PASS |
| `TestDynamicPeerEntersTheIndexUnderItsOwnAddress` | `internal/component/bgp/reactor/delivery_graph_test.go` | phase 7: AC-6. It REPLACES the planned `TestGraphMatchesDynamicPeerByGroup`, whose premise (R-2) was falsified at the producer: there is no group-keyed match, so a test of one would assert machinery nobody should build. It drives the real chain instead -- `ParseDynamicGroupTemplate` parses the group's attach block, `buildDynamicPeerSettings` builds the member, the index answers on the member's ADDRESS and answers nothing on its generated name | PASS |
| `TestDynamicGroupTemplateMatchesAStaticPeer` | `internal/component/bgp/config/dynamic_group_parity_test.go` | phase 7 cites it rather than duplicating it: it walks `PeerSettings` by reflection over a config holding a dynamic group and a static peer stating the same leaves, and `parityConfig`'s group carries a non-empty `attach process rib` block, so the group's bindings surviving resolution into the template is already pinned | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `attach-process-receive-filter.ci` | `test/plugin/` | AC-1 and AC-3: one peer attaches two programs with different `receive` lists, and each is handed only what its own list names | PASS |
| `attach-process-unattached-is-silent.ci` | `test/plugin/` | AC-2: a running program that no peer attaches is fed nothing, while the peer that attaches it is served | PASS |
| `attach-process-reload.ci` | `test/plugin/` | AC-4: a reload changes one peer's list, delivery follows the new list, and a surviving edge misses no event across the reload | PASS |
| `attach-process-runtime-subscribe.ci` | `test/plugin/` | AC-5: the runtime `subscribe` command takes effect at once, and a reload restores the config's graph | PASS |
| `attach-process-dynamic-group.ci` | `test/plugin/` | AC-6 and AC-6b: one program, three peers, three relationships. A peer a dynamic group created is fed the group's list; a static member that states no block keeps its group's list; a static member that restates it is fed its own and NOT its group's | PASS |
| `attach-process-send-permission.ci` | `test/plugin/` | AC-9, AC-10 and AC-11: an announce reaches the peer that attached the program with `send`, is refused for one that did not, and the refusal names both | PASS |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | no wire-visible behaviour changes, so no peer daemon can observe this | N-A |

## Files to Modify

Enumerated 2026-08-14 and each path verified to exist. Grouped by the phase that
touches it.

### Phase 1, rename

| File | Symbol or content |
|------|-------------------|
| `internal/component/bgp/yang/ze-bgp-conf.yang` | `list process` inside `grouping peer-fields` |
| `internal/component/bgp/reactor/config.go` | `parseProcessBindingsFromTree` |
| `internal/component/bgp/config/plugins.go` | `extractInlinePluginsFromMap`, `validatePeerProcessRefs` |
| `internal/component/config/graph.go` | `addProcessBindings` |
| `internal/component/config/migration/api.go` | `migrateAPIFromPeer`, `migrateAPIBlock`, `buildNewAPIBlock`, and the stale doc comment that still writes `api` in examples the code reads as `process`. These read the keyword, so after the rename they would look for a key that no longer exists and silently do nothing. No `process` to `attach process` rewrite is added |
| `internal/component/config/migration/detect.go` | `hasOldStyleAPI` |
| `internal/component/cli/model_load.go` | the hardcoded `blockKeywords` map holding `"process": true`. See R-13: the two-word keyword defeats its key heuristic |
| `internal/exabgp/migration/migrate.go` | `migrateProcesses`, `addProcessBinding`, and the `listFields` entry naming `process` |
| `internal/exabgp/migration/migrate_serialize.go` | writes the literal `process ` into ze-native output |
| 516 config and doc files | 707 `process <name> {` occurrences across 518 files. Two are ExaBGP-format INPUT and keep the old word: `test/exabgp-compat/etc/api-watchdog.conf` and `test/exabgp/process/input.conf`. The `test/exabgp/*/expected.conf` files are converter OUTPUT and flip |

`internal/exabgp/migration/exabgp.yang` keeps `list process`: it models ExaBGP's own
input grammar, which this rename does not touch.

The 518-file, 707-occurrence measure supersedes the "691 across 505" figure recorded
earlier in this spec. Both counted the same thing; the later sweep used the looser
pattern and found more.

### Phase 2, vocabulary

| File | Symbol or content |
|------|-------------------|
| `internal/component/bgp/yang/ze-bgp-conf.yang` | `leaf-list receive` with `ze:validate "receive-event-type"`, `leaf-list send` with `ze:validate "send-message-type"`, and the description that refuses `all` |
| `internal/component/config/validators.go` | `ReceiveEventValidator`, `SendMessageValidator`, `allBGPEventNames`, `allSendTypeNames`, `baseSendTypes` |
| `internal/component/config/validators_register.go` | `RegisterValidators`, which binds both names |
| `internal/core/events/events.go` | `IsValidEvent`, `ValidEventNames`, `RegisterEventType`, `IsValidSendType`, `ValidSendTypeNames` |
| `internal/component/bgp/reactor/config.go` | `parseReceiveFlags`, `parseOneReceiveFlag`, `parseSendFlags`, `parseOneSendFlag` |
| `internal/component/bgp/reactor/peersettings.go` | `ProcessBinding`, which gains per-type direction |
| `internal/component/bgp/config/loader_create.go` | derives `ConfiguredCustomEvents` and `ConfiguredCustomSendTypes`, which must survive the new token shapes |

### Phases 4 to 7, delivery

| File | Symbol or content |
|------|-------------------|
| `internal/component/bgp/server/events.go` | the seven peer-scoped `GetMatching` sites and their funnel `deliverToProcs` |
| `internal/component/plugin/server/subscribe.go` | `Subscription`, `PeerFilter.Matches`, `Subscription.Matches`, `SubscriptionManager.GetMatching`, `ParseSubscription` |
| `internal/component/plugin/server/dispatch.go` | `registerSubscriptions`, and the further peer-scoped `GetMatching` in `emitEvent` that passes an empty peer name |
| `internal/component/plugin/server/startup.go` | one of two production callers of `registerSubscriptions` |
| `internal/component/plugin/server/dispatch_registry.go` | the other caller |
| `internal/component/plugin/server/startup_autoload.go` | custom-token auto-loading, preserved |
| `internal/component/bgp/reactor/reactor_api.go` | `getMatchingPeersSel`, where the send permission is enforced, and `SoftClearPeer` |
| `internal/component/bgp/reactor/reactor_api_forward.go` | `AnnounceEOR`, `sendRouteRefresh` |
| `internal/component/bgp/reactor/reactor_api_batch.go` | `AnnounceNLRIBatch`, `WithdrawNLRIBatch`, `SendRoutes` |
| `internal/component/bgp/config/peers.go` | `validatePeerProcessCaps`, preserved and given a real enforcement point |
| `internal/component/bgp/reactor/reactor_dynamic.go` | `buildDynamicPeerSettings`, for phase 7 |

`internal/component/plugin/server/monitor.go` carries `MonitorManager.GetMatching`, the
same filter shape for `ze monitor` clients. It is NOT process delivery and is out of
scope; named here so it is not mistaken for a missed call site.

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/plugin/server/delivery_graph.go` (+ test) | the per-peer, per-type edge index, its atomic swap, and the operator view (DONE, phase 4) |
| `internal/component/bgp/reactor/delivery_graph.go` (+ test) | `DeliveryPeersFromSettings` and `publishDeliveryGraphLocked`, the config producer (DONE, phase 4) |
| `internal/component/bgp/config/delivery_graph_test.go` | AC-6b, driven through the real group merge (DONE, phase 4) |
| `test/plugin/attach-process-delivery-graph.ci` | the index is inspectable from outside the daemon (DONE, phase 4) |
| `test/plugin/attach-process-receive-filter.ci` | AC-1, AC-3 |
| `test/plugin/attach-process-unattached-is-silent.ci` | AC-2 |
| `test/plugin/attach-process-reload.ci` | AC-4 |
| `test/plugin/attach-process-runtime-subscribe.ci` | AC-5 |
| `test/plugin/attach-process-dynamic-group.ci` | AC-6, AC-6b |
| `test/plugin/attach-process-send-permission.ci` | AC-9, AC-10, AC-11 |

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang`: `list process` renamed, `leaf-list receive` and `leaf-list send` gain direction tokens and `*` |
| YANG validation constraints | Yes | Both leaf-lists stay `ze:validate`-driven rather than gaining an enumeration, because the type set is extensible at runtime through `RegisterEventType` |
| YANG custom validators | Yes | `ReceiveEventValidator` and `SendMessageValidator` in `internal/component/config/validators.go`, with their `CompleteFn` helpers `allBGPEventNames` and `allSendTypeNames` |
| CLI commands/flags | Yes | **Corrected in phase 4.** `show event delivery` is new: `container delivery` under the existing `container event` in `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`, `rpc event-delivery` in `ze-cli-show-api.yang`, handler `handleShowEventDelivery` in `internal/component/cmd/show/show.go`. The index is built inside the daemon from resolved settings, and `ze config graph` cannot show it: that command is offline and lives in `internal/component/config/cli`, a tier that must not import the BGP resolver (the import that used to pin `internal/component/bgp` into every binary and defeat `//go:build ze_bgp`). No verb means no way to debug the index |
| CLI grammar (keyword before value) | Yes | `attach process <name>` is keyword-before-value. `internal/component/cli/model_load.go` `blockKeywords` needs the two-word form, see R-13 |
| Editor autocomplete | Yes | Automatic through `RegisterValidators`, which `internal/component/cli/completer.go` calls. The new tokens appear once the `CompleteFn` helpers emit them |
| Functional test for new RPC/API | Yes | The six `test/plugin/attach-process-*.ci` files in Files to Create |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No leaf under `environment/` |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, or binary. The reconciliation report at plugin ready is a log line, not a dependency |
| Prometheus counters/metrics | Yes | A counter for announces refused by the send permission, so AC-10's refusal is observable rather than only logged |
| BGP family surface | N-A | No SAFI, capability, or attribute |

### Documentation Update Checklist

Every row answered. Phase 7 wrote the rows marked DONE below; phases 1 to 6
wrote the rest and phase 7 verified each one against the file.

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | **DONE, phase 7.** `docs/features.md`, the Plugins row: the peer's config decides which program sees which peer, `receive` and `send`, silence by default, group inheritance, `show event delivery` |
| 2 | Config syntax changed? | Yes | **DONE.** `docs/architecture/config/syntax.md` "Process Section" (phase 2 the enums, phase 6 the send enforcement, phase 7 the receive enforcement and the group/dynamic merge rule), `docs/guide/configuration.md` "Process Bindings" and `docs/config-reference.md` "Process Bindings" (both phase 7: the send table, the absent-block rule, the group rule) |
| 3 | CLI command added/changed? | Yes | **DONE, phase 4.** `show event delivery`: `docs/guide/command-reference.md` Discovery table, and "Reading the delivery graph" in `docs/guide/plugins.md` |
| 4 | API/RPC added/changed? | Yes | **DONE, phase 5.** `docs/architecture/api/commands.md` "Event Subscription Commands" carries the two halves and the precedence rule. Phase 7 also removed two fictional `api route-server { content { nlri ... } }` examples there: `ContentConfig.NLRI` and `.Attributes` are engine-set and no config leaf reaches them |
| 5 | Plugin added/changed? | Yes | **DONE.** `docs/guide/plugins.md` "Binding Plugins to Peers" and "Directions" (phase 7 corrected the retired keyword in its prose and added the absent-block rule). The per-plugin Typical Binding table is true as of phase 3; phase 7 added the missing `bgp-rr` row, read from its own `SetStartupSubscriptions` |
| 6 | Has a user guide page? | Yes | **DONE, phase 3**, verified phase 7: no `.md` under `docs/` or `ai/` still spells the retired keyword or the retired `receive { }` block form |
| 7 | Wire format changed? | No | Nothing wire-visible |
| 8 | Plugin SDK/protocol changed? | Yes | **DONE, phase 7.** `docs/architecture/api/process-protocol.md` "Event Delivery" opened by saying a subscription states what a plugin CAN handle and the peer's config decides what it GETS, plus the plugin-author obligation to say which `receive` list the program needs, and "Event Subscription" on what a runtime subscription does and does not widen. `ai/rules/plugins.md` needs no change: the obligation is documentation, not registration |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Scope is not protocol |
| 10 | Test infrastructure changed? | N-A | **Corrected in phase 7.** `docs/functional-tests.md` documents the `.ci` DIRECTIVE grammar and shows no peer config block, so the rename reached nothing in it (`grep` for `attach process`, `receive [`, `send [`: zero hits) |
| 11 | Affects daemon comparison? | No | No comparable behaviour changes |
| 12 | Internal architecture changed? | Yes | **DONE, phase 7.** `docs/architecture/api/architecture.md` rewritten: "Process and Peer API Binding" is now "Attaching a Process to a Peer" (the model, the real syntax, groups and dynamic peers, config-to-delivery flow, both halves, encoding); the `ReactorInterface` snippet (20 fictional methods, `GetPeerAPIBindings` among them) is replaced by a pointer to `bgptypes.BGPReactor`; the attribute-filtering example loses its `api foo { }` block and its `nlri` leaf, which no YANG carries. Two Implementation Status rows and one TL;DR row added. `docs/architecture/overview.md` no longer carries the keyword (phase 1) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | **DONE, phase 6.** `ze_bgp_send_refused_total{process,type}` in `docs/guide/monitoring.md` and `docs/guide/plugins.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | **DONE.** The vocabulary lives in `docs/architecture/config/syntax.md` and `docs/config-reference.md`, both updated. `docs/plugin-overview.md` and `docs/features/plugins.md` enumerate plugins, never event or send types, so neither carries the vocabulary and neither changed. `docs/architecture/api/capability-contract.md` did carry it, in two quoted error messages that no longer matched the producer: corrected in phase 7 against `validatePeerProcessCaps` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | **DONE.** `make ze-verify-wiring-docs` passes all 10 gates and `make ze-doc-test` passes, `ai/CODE-TO-DOCS.md` regenerated |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | **DONE.** `docs/exabgp/exabgp-migration.md` said ze's syntax was `api <name> { }` and now says `attach process <name> { }`; `docs/plugin-development/README.md` gained the sentence a plugin author needs, that a plugin no peer attaches is fed nothing; `ai/rationale/compatibility.md` carries no example of this block |

## Implementation Steps

Ordered so each step is separately revertible, and so the mechanical work never sits
in the same commit as a behaviour change.

1. **Phase: Rename.** `process <name>` becomes `attach process <name>` across the YANG,
   the parser, the CLI editor's key heuristic, the docs and all 518 config files. The
   ExaBGP converter maps the old keyword on the way in and emits the new one. Nothing
   upgrades an existing ze-native file: the parser refuses the old keyword outright,
   because ze is not released. Two ExaBGP-format inputs keep the old word. Delivery is
   untouched, so the whole suite must stay green on an unchanged daemon, which is what
   makes this phase separately revertible.
   - Tests: `TestParseRefusesRetiredProcessKeyword`, `TestExabgpConverterEmitsAttachProcess`,
     and the AC-13 editor round trip that R-13 requires
2. **Phase: Vocabulary.** Add the hyphenated direction tokens and the `*` wildcard,
   with registry-first token resolution. Still no delivery change: the new tokens
   parse, validate and are carried, and nothing reads them yet. Ten of the 15 in-tree
   plugins cannot state their subscription without this phase, which is why it comes
   before every delivery change.
3. **Phase: Correct the configs.** Make every `receive` and `send` list in the tree say
   what its author meant, using the evidence from the 2026-08-14 sweep. Still no
   behaviour change, so the suite stays green throughout and this phase is provably
   safe on its own.
   - **Drive the correction from what each plugin script CALLS, never from what its
     config already says.** A fixture's config is the thing this spec has established
     is not load-bearing, so reading it back tells you nothing about intent. Read the
     script: a `send(...)` needs a `send` permission on every peer it addresses, and a
     handler for an event type needs that type in `receive`. R-14 is the failure this
     ordering prevents, and it was found in a live fixture outside this spec.
4. **Phase: Wiring.** Build the graph from resolved peer settings, expose it for
   inspection, and prove it is reachable, with delivery still driven the old way.
5. **Phase: Inbound.** Delivery consults the graph. The correction phase means the
   suite should stay green.
6. **Phase: Outbound.** The send permission is enforced at the point a process
   originates a message toward a peer.
7. **Phase: Groups and dynamic peers.** The match learns about groups so a dynamic
   peer, whose generated name no config contains, is reached by its group's edges.

## Phase Evidence

Recorded by the main thread from its own runs, not relayed from a phase agent's
report. A phase's own claim of green is a claim; this table is what was measured.

| Date | Phase | Evidence | Result |
|------|-------|----------|--------|
| 2026-08-15 | 3 | `make ze-plugin-test`, the suite that owns the 480 corrected config fixtures | **594 of 594 passed, 100%, exit 0**, 44 platform skips. Run in the main thread after the blocking break cleared, on a tree also carrying phase 4 work in progress |
| 2026-08-15 | 1 to 4 | `go vet` over every package this spec touches: `internal/core/events`, `internal/component/config`, `bgp/reactor`, `bgp/config`, `bgp/plugin`, `component/plugin`, `component/cli`, `internal/exabgp` | exit 0. This COMPILES `_test.go`, which `make ze-tracked-build-check` does not: that gate runs `go build`, so a broken test build reaches HEAD invisibly (`ai/rules/git-safety.md` names the blind spot, and a session hit it in this checkout on 2026-08-14). Repeat this before treating the work as committable |
| 2026-08-14 | 1 | `test/plugin/rfc7606-reset.ci` and `rfc7606-withdraw.ci` diffs, read by the main thread before accepting the audit re-stamp | one config line each, every requirement tag, hex buffer and assertion byte-identical. The re-stamp of RFC7606-3.a-1, 3.j-1, 5.3-6 and 7.1-1 is justified |

| 2026-08-15 | 5 | `make ze-plugin-test`, re-run in the main thread after the phase agent's own run, because phase 5 is where delivery starts consulting the config and an agent's green over its own behaviour change is a claim | **601 of 601 passed, 100%, exit 0**, 234s, 44 platform skips |

**A full disk fakes a red, and it did so here.** On 2026-08-15 the volume reached 100%
with under 550MB free. A phase agent took **54 false test failures** before finding the
cause, and a second session took a bogus typecheck red and a `no space left on device`
out of `ze-tracked-build-check`. A full disk does not reliably announce itself: it
surfaces as a compile error in code nobody touched, a truncated write, or a corrupt
cache entry. `plan/journal/full-disk-false-red.md` is the class. **Before believing any
red, check `df -h`.** `go clean -cache` reclaimed 42GB. `~/.cache/ze` holds a further
83GB of durable kernel artifacts and costs a 30-minute rebuild, so clearing it is the
owner's call and not an agent's.

| 2026-08-15 | 6 | `make ze-plugin-test` after a sibling session repaired 15 same-address fixtures its RFC 4271 Section 5.1.3 guard had reddened | **599 of 602**, 296s. Three remain: `redistribute-as112-announce` (the IPv6 half of that session's guard, which it is fixing by teaching `make ze-setup` to add an IPv6 loopback alias), `reload-listener-rejected` (new, unattributed, the runner broke before it could be investigated), and `test-pipe-first-last` (under diagnosis; my delivery filter is PROVEN innocent by disabling it) |

**THE COMMIT IS INDIVISIBLE, and this is the sharpest constraint on this work.**
Verified 2026-08-15: `attach` appears nowhere in `internal/component/bgp/config/` at
HEAD and only in the working tree. So the ~500 migrated fixtures and the parser that
reads them MUST land in ONE commit. A commit carrying migrated `.ci` files without
`internal/component/bgp/config/plugins.go` puts files at HEAD that HEAD cannot parse,
failing with `unknown field in peer: attach`. A commit carrying the parser without the
fixtures reds every unmigrated one. This is `ai/rules/git-safety.md`'s "commit the
producer with its consumer", and here the consumer is 500 files wide. A sibling session
holding repairs to 15 of those fixtures is blocked on the same fact and cannot land
early, in either direction.

**FOUR FILES MUST NOT ENTER THIS COMMIT, verified 2026-08-15.** Rendering a rule
regenerates the WHOLE rule set, so `make ze-rules-render` swept other sessions'
uncommitted point edits into files this spec never touched. Dirty rendered rules are
`CORE.md`, `commands.md`, `git-safety.md`, `testing.md` and `writing.md`. **Only
`ai/rules/writing.md` is ours**, together with `ai/rules/points/writing/manifest.md`
and the new `ai/rules/points/writing/documentation/write-every-config-example-on-several-lines.md`.
Committing the other four would land a stranger's half-written rule text under this
spec's message, and nothing in the commit helper would object.

**A blocking dependency, recorded because it will recur.** The functional test runner
links `internal/component/web`. While another session's uncommitted templ refactor left
that package uncompilable, EVERY `.ci` suite was unbuildable and no phase of this spec
could be functionally verified. The break is invisible from HEAD, which compiles. Time
a run to a window when the shared tree builds rather than retrying into it.

## Design Insights

**Owner design direction, 2026-08-14: a registered delivery graph, not a broadcast
that each consumer filters.** Data is sourced from BGP, and the edges it travels are
registered up front from the config: peer to process, carrying the set of event types
that edge accepts. Delivery then walks registered edges only, by callback for an
in-process plugin and by channel plus its own goroutine for an external program. The
engine stops asking "who wants this" per event and starts knowing.

This is the same registration shape the rest of ze uses (`ai/rules/architecture.md`,
small core plus registration), and it fixes a hot-path cost that exists today.
`SubscriptionManager.GetMatching` scans every process and every subscription on every
event and allocates a fresh result slice each time. On the UPDATE path that is a scan
and an allocation per message. A graph built at config load and inverted for lookup
(per peer, per event type, a precomputed list of edges) replaces both with one index
read and no allocation.

Delivery guarantees today: `Process.Deliver` blocks on a full channel rather than
dropping, so back pressure reaches the caller and events are not silently lost. The
one exception is a process that is stopping, where `Deliver` returns false and the
only caller uses that return for cache counting. The graph should make that case
explicit rather than silent.

**The graph is an INDEX over subscriptions, not a second concept beside them.** The
temptation is to build a new structure and leave `Subscription` for the runtime
command, which would put two mechanisms back in charge of one question, the exact
state that produced this defect. Instead the subscription stays the unit of
registration, the config becomes its primary producer, and the graph is a derived
index keyed by peer and event type so the delivery path does one read instead of a
scan. One concept, two producers, one lookup.

**The config vocabulary is strictly weaker than the subscription grammar, and that is
probably WHY the January swap happened.** A subscription is `<type> [direction
received|sent|both]`. The config leaf is a flat list of single words: `parseReceiveFlags`
splits on whitespace, and `events.RegisterEventType` refuses any type containing
whitespace, so no token can ever carry a direction. The one direction word the config
has is `sent`, and it is a flag on the BINDING rather than on a type. Three shapes
therefore cannot be said at all today:

| Shape | Who needs it | Why it cannot be written |
|-------|--------------|--------------------------|
| One type received-only while another is both | `bgp-rs` (update received, refresh both) | `sent` applies to the whole binding, not per type |
| A type sent-only | `bgp-persist` (`update direction sent`) | Any spelling that grants sent also grants received |
| Every type, including ones registered later | `exabgp` and `exabgp-bridge` (`*`) | `all` is deliberately refused so a new type is never granted silently |

Read in that light, the January commit is less a shortcut than a retreat: the config
could not express what plugins needed, so the declaration moved to the plugin, where
the grammar is richer. Making the config authoritative therefore means EXTENDING the
config vocabulary first. That is the fork this spec has to settle before any delivery
code changes.

**The full enumeration, which is A-2's validation method (2026-08-14).** Every
`SetStartupSubscriptions` call site was read. All 15 pass `nil` for the peer list.
The Expressible column asks whether the peer's `receive` leaf-list can state the same
thing today, given that a plain token grants both directions and `all` is refused.

| Call site | Declares | Expressible today |
|-----------|----------|-------------------|
| `internal/component/bgp/plugins/adj_rib_in/rib.go` | update received, state | No, direction |
| `internal/component/bgp/plugins/bmp/bmp.go` | state, and update, open, notification, keepalive, refresh in both directions | Yes, plain tokens mean both |
| `internal/component/bgp/plugins/gr/gr.go` | open received, state, eor | No, direction. `eor` is not a base token and takes the custom path |
| `internal/component/bgp/plugins/persist/server.go` | update sent, state, open received | No, direction |
| `internal/component/bgp/plugins/redistribute_egress/register.go` | state | Yes |
| `internal/component/bgp/plugins/rib/rib.go` | update both directions, state, refresh | Yes, plain tokens mean both |
| `internal/component/bgp/plugins/rpki/rpki.go` | update received | No, direction |
| `internal/component/bgp/plugins/rpki_decorator/decorator.go` | update received, rpki | No, direction. `rpki` takes the custom path |
| `internal/component/bgp/plugins/rr/rr.go` | update received, open received | No, direction |
| `internal/component/bgp/plugins/rs/server.go` | update received, open received | No, direction |
| `internal/component/bgp/plugins/watchdog/watchdog.go` | state | Yes |
| `internal/plugins/flowspec-firewall/engine.go` | update received, state | No, direction |
| `internal/plugins/exabgp/main_sdk.go` | `*` | No, wildcard |
| `internal/plugins/exabgp/bridgeplugin/internal.go` | `*` | No, wildcard |
| `internal/test/cli/cmd_text_plugin.go` | update | Yes |

Ten of the 15 cannot be stated in today's config: eight need a direction, two need the
wildcard. That is the size of the vocabulary gap, and it is why phase 2 precedes every
delivery change rather than being an optional extra.

Two more facts the design must absorb. `sent` is registered as a bgp event TYPE as
well as being a binding flag, with a source comment saying it is "a config receive
flag, not a true event type", so a naive config-to-subscription mapping mints a
subscription no emitter ever matches. **Settled by phase 2: the fake type is gone.**
The direction now belongs to the type, so `receive [ sent ]` is refused with a message
naming `update-sent`, the bgp namespace no longer registers `sent`, and the three
sites that worked around the duality (`excludedFromMonitor` in
`internal/component/plugin/server/event_monitor.go`, the filter in `bgpEventTypes`
in `internal/plugins/meta/cmd/help.go`, and the binding-wide `ReceiveSent` flag) are
deleted. Every event type in the registry is now a type an emitter can raise.
And `ReceiveEventValidator` is hardcoded to the
bgp namespace, so the leaf can never name an event from `iface`, `ospf`, `isis`, `fib`
or any other namespace; no in-tree plugin needs that today, but the SDK's own test
uses a non-bgp namespace, so an operator's plugin could.

**Config is parsed before an external plugin can declare anything.** A forked program
hands over its subscriptions in the ready RPC, which happens after config load, so a
check of "the config grants an event this plugin never declared" cannot live at config
validation time. It belongs at plugin ready, where both halves are known. This
ordering also means the graph has to be buildable with edges whose process has not
started yet.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The keyword becomes `attach process <name>`. The bare word `process` is retired from ze's native config and survives only in the ExaBGP converter, which maps it on the way in | Keep `process`; use `connect`; use `attach` alone | Owner, 2026-08-14. `process` is ExaBGP's word and says nothing about the relationship. `connect` collides twice: `connection` is already the peer's TCP container and it already holds a `leaf connect`. `attach process` says what the block does, which is attach this peer to that program |
| An old config hard-fails at parse. Nothing upgrades it, and no migration path is written | Rewrite the file through the ze config migrator; accept both keywords in the parser | **Owner, 2026-08-14: ze is not released, so no backward compatibility is owed.** This replaces the migrator route adopted earlier the same day. The simplest fully correct answer is now the smallest one (`ai/rules/simplicity.md`), and `ai/rules/no-layering.md` is satisfied outright: one spelling in the parser, no second path to maintain. `internal/component/config/migration/api.go` still needs its keyword read updated, because `migrateAPIFromPeer` and `hasOldStyleAPI` would otherwise look for a key that no longer exists and silently do nothing. That is keeping a live function correct, not a compatibility path |
| Both directions are in scope and enforced by one graph: `receive` is what the process is fed, `send` is what it may generate toward that peer | Fix the inbound half only | Owner, 2026-08-14. `send` is a permission and is not checked anywhere: a process reaching `peer *` today addresses every peer, including peers that never attached it. Enforcing only the inbound half leaves route injection open, which is the more serious of the two |
| A peer that does not attach a process feeds it nothing and may not be announced to by it | Absent block means the program's own startup request applies | Follows from the model the owner stated: `receive` is what the process wants to see, stated per peer. A program not attached to a peer has expressed no wish about it |
| A member's list replaces the group's for that process | Union the two | This is the existing merge semantic, since `receive` is a leaf-list outside `cumulativePaths`. Changing it would surprise every config that already relies on the merge |
| Direction is expressed by hyphenated tokens: `update-sent`, `update-received`, with the plain type meaning both. A token is resolved against the event-type registry FIRST, and only an unresolved token is split on a trailing `-sent` or `-received` | A container per type carrying a `direction` leaf; a second direction word applying to the whole list | Adopted 2026-08-14, owner may overturn. The flat leaf-list survives and no existing config changes meaning; the nested form reads better and rewrites all 691 bindings. Registry-first resolution is what makes it safe: hyphenated custom types already exist (`update-rpki` auto-loads `bgp-rpki-decorator`), so a bare split would misread one the day a plugin registers a type ending in `-sent` |
| The wildcard is `*`, the same character the subscription grammar and `PeerFilter` already use | An English word such as `any`; reinstate `all`; enumerate types for exabgp; exempt wildcard-declaring plugins | Adopted 2026-08-14, owner may overturn. Changed from the earlier "explicit word" recommendation: `*` cannot collide with a registered type name, is already what `exabgp` and `exabgp-bridge` declare, and leaves the YANG description's refusal of `all` true rather than reversing a deliberate decision. An English word would look like a type and would still grant a later-registered type silently, which is what that refusal was about. Enumeration is not available to the ExaBGP bridge, whose contract is to forward whatever the API can express |
| Config-derived registered graph is the source of truth for who receives what | Keep plugin-declared subscriptions primary and intersect the config on top; keep broadcast-and-filter | The config is what the operator writes and what the documentation promises. Intersection leaves two mechanisms deciding one question, which is the state that produced this defect |
| Filter is uniform over processes, with no exemption for ze's own plugins | Exempt in-process plugins so the RIB always receives everything | A process is a process. The operator case that motivates this work is an application seeing a peer's updates while the host routing table is not updated, which an exemption makes unsayable |
| The plugin's own subscription declares what it CAN handle; the config decides what it GETS. The effective set is the overlap, and the difference is reported at plugin ready | Deliver whatever the config names regardless of the declaration; deliver whatever the plugin declared regardless of the config | Settled 2026-08-14. Delivering an undeclared type spends IPC on an event the program has no callback for, and delivering an ungranted type is the defect this spec exists to fix. Reporting the difference is what gives R-9 a voice: today neither half can tell the operator they disagree |
| The index lives in `internal/component/plugin/server` and the reactor pushes it after every config apply. It is keyed on the peer ADDRESS, and its per-process view is read back OUT of the index rather than stored beside it | Keep the index in the reactor and have delivery reach back for it; store the operator view as a second structure | Adopted 2026-08-15, phase 4. The plugin server owns delivery, so it owns the lookup, and `internal/component/bgp/reactor` already imports `pluginserver` (`pluginServerFactory`, `reactor.go`), so the push needs no new seam. Address rather than name: every peer-scoped delivery site already carries `peer.AddrStr()`, so the lookup needs no second key and A-3 stops gating the graph. The view is derived so `show event delivery` shows the edges delivery reads, not a parallel structure that can disagree with them |
| The publish points are `AddPeer`, `RemovePeer` and the end of `StartWithContext` | Publish at the end of each config-apply entry point (`reconcilePeersJournaled`, `applyConfigOperation`) | Adopted 2026-08-15, phase 4. `ProcessBindings` is outside `hotSwappableSettings` (`peer_settings_apply.go`), so a peer whose attach block changed is always torn down and re-added: `AddPeer` and `RemovePeer` are the only paths a changed block can take, journal rollbacks included. Publishing per apply-site instead would miss the rollback path and would need one call per entry point. `AddPeer` and `RemovePeer` publish only while the reactor runs, so the startup load pays one build for the whole peer set rather than one per peer |
| Config is durable truth, rebuilt on every reload. A runtime `subscribe` is a live override that the next reload discards | Persist a runtime subscription across reloads; refuse the runtime command once the config is authoritative | Settled 2026-08-14. A reload's job is to make the daemon match the config document, so a runtime change surviving one would make the document a lie. Refusing the command instead would remove an operator's only way to look at a live session. AC-5 tests both halves |

### Critical Review Checklist

Feature-specific only. The generic checks in `ai/rules/quality.md` always apply and
are not repeated. Added 2026-08-15, after phases 1 and 2 ran without it.

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation named as file plus symbol, and every one has a test that fails when the implementation is reverted |
| Feature completeness | Both End-to-End User Story configs parse, and each produces exactly the delivery table printed beside it. A story that cannot be run is not a story |
| Correctness: direction | A plain token grants BOTH directions. No config in the tree may grant plain `update` to `bgp-rr`, `bgp-rs` or `bgp-rpki`: both directions deadlock them (R-15), so this check is about a hang, not a filter |
| Correctness: merge | A member's list REPLACES its group's, and a peer that attaches no block for a running plugin is fed nothing by it. Both directions of AC-6b are asserted, not just the member's |
| Correctness: send | `getMatchingPeersSel` drops a peer that did not attach the process, and the drop is REPORTED. A silent refusal fails this row even when the filtering is right |
| Correctness: wildcard | `receive [ * ]` names no type, so it contributes no auto-load token. Rewriting a custom token to `*` must not stop its plugin loading |
| Naming | The YANG leaf, the token grammar in `internal/core/events/token.go` and the subscription grammar agree on every type name and every direction word. Three spellings of one concept is the defect this spec exists to fix |
| Data flow | The graph is an INDEX over subscriptions, never a second registry beside them. One concept, two producers, one lookup |
| Registration over hardcoding | No plugin name appears in a core package. The token grammar and `ze:flatten` read the registry, never a hardcoded list |
| Rule: `ai/rules/evidence.md` | Every guard fails closed. A missing edge means no delivery. A lookup that cannot find a peer must never return "everything" |
| Rule: `ai/rules/performance.md` | The per-event lookup allocates nothing (AC-8), proven by a test rather than by inspection |
| Rule: `ai/rules/no-layering.md` | The replaced mechanism is DELETED, not left beside the new one. `sent` as an event type, `ReceiveCustom`, `SendCustom` and the eight receive booleans are gone, and nothing reintroduces a second path |

## Goal Validation

`ai/rules/interop-and-goal-validation.md` requires evidence per stated goal, beyond
individual assertions. The Task section states one goal in two halves, and the owner
added a third at the design gate.

| Goal | Evidence |
|------|----------|
| A peer's `receive` list decides which event types each attached process is handed, per peer | `test/plugin/attach-process-receive-filter.ci`: one peer, two programs declaring the same types, each fed only what its own list names. It discriminates: with `PeerScopedProcs` returning its process set unfiltered, it reds |
| A process no peer attaches is fed nothing by that peer | `test/plugin/attach-process-unattached-is-silent.ci`: the silent program refuses the first event it ever receives, so a leak fails the test rather than passing quietly |
| A process may originate toward a peer only where that peer's `send` list permits it | `test/plugin/attach-process-send-permission.ci` covering AC-9, AC-10 and AC-11, plus `TestSendPermissionRefusesUnattachedPeer`. Discrimination proven by short-circuiting the filter: the functional test reports `the unattached peer was sent 1 update(s)` |
| The config keeps meaning what it says across a reload and a runtime override | `test/plugin/attach-process-reload.ci` (the added edge refuses any event beating the SIGHUP marker) and `attach-process-runtime-subscribe.ci` (keepalives at 1/s give both windows several frames, and it asserts the override does not rewrite the config's index) |
| A dynamic peer, whose generated identity no config names, is reached by its group's edges | `test/plugin/attach-process-dynamic-group.ci`: one program, three peers, a dynamic member fed its group's list while a member restating it is fed its own and not the group's. `TestDynamicPeerEntersTheIndexUnderItsOwnAddress` pins the mechanism |
| The change costs no per-event allocation | `TestGraphLookupAllocatesNothing` measures 0 on hit and miss; `TestPeerScopedProcsAddsNoAllocation` shows the filter adds none over `GetMatching`'s own 2 |
| The whole daemon still works | `make ze-plugin-test` 603 of 603, exit 0, measured by the main thread rather than reported by an implementing agent |

Interop: not applicable. No wire behaviour, negotiated capability, or RFC obligation
changes. Which locally configured process receives a copy of an event is invisible to
a peer daemon, so no interop scenario can observe it.

## Review Gate, round 1

Three independent lenses over the whole diff, none of them an author. Round 1 is the
only pass that sees the whole change, so its lens count is the change's coverage.

**The finding that mattered: the send permission was BYPASSABLE, and it is the hole
this spec exists to close.** Phase 6 gated `getMatchingPeersSel` and left four rails
around it. `ze-bgp:cache-forward` is a registered wire method any connected process may
call; it reaches `ForwardUpdate`, whose signature carried a plain `pluginName string`
for accounting and no permission. So a process holding an update id from peer A could
relay that whole UPDATE into peer B, which never attached it. `ze-bgp:peer-raw` reaches
`SendRawMessage`, which took no sender at all: arbitrary bytes on any peer's socket, a
forged NOTIFICATION included. Verified by the main thread at the producer before any
fix was commissioned.

**Why it survived phase 6.** The implementing agent named the four rails in its own
handoff and argued that gating them needed a vocabulary decision the spec never posed,
since the send list holds only `update` and `refresh` while a raw message can be an
OPEN or a whole packet. The main thread accepted that framing. It is right for
`SendRawMessage` and WRONG for the other three, which emit UPDATEs, for which
`send [ update ]` is the existing word. A correct argument about one rail was allowed
to cover three others it does not reach.

**Three further findings were false safety claims**, which `ai/rules/evidence.md` holds
severe because they stop the next reader asking: the `BGPReactor` doc said every
wire-writing method takes a `Sender` while four did not; two documents said every
peer-naming command inherits the permission while `cache-forward` names peers and
inherits nothing; and `peer_initial_sync.go` claimed a KNOWN DEFECT retired on the
strength of a property `handleBgpCacheForward` disproves.

**A guard is only real on the path that carries the traffic.** The resolver was gated,
tested and correct, and the traffic had four other ways to the wire. The tests missed
it for the same reason: they drove the reactor method rather than the registered wire
method an operator's program actually calls. That is why the fixes are tested from the
entry point, and it is the same shape as a finding a sibling session made tonight about
an authorization check sitting on a page template while a script re-fetched the data
from an ungated handler.

## Review Gate, round 2

Two lenses, bounded to round 1's fixes and what they touched.

**The fix repeated the defect it was fixing, inside its own tests.** Round 1 found the
send permission bypassable because the tests drove the internal reactor method while
the traffic arrived through a registered wire method. The fix gated four rails, and
then every one of its tests passed `plugin.OperatorSender()`, which returns at the
first line of the filter before any peer is read. So the four rails had ZERO deny-side
evidence. The guard code was correct in all four; nothing proved it fires.

**This invalidated a reassurance the main thread had offered.** Zero `send refused`
lines across 604 functional tests was reported as evidence the gates do not over-fire.
It is equally consistent with a guard that never runs, and no test made any of the four
fire. A measurement is only evidence of what it could have contradicted.

**A unit package was red while the main thread reported the tree green**, because it
had been measuring with `make ze-plugin-test`, the functional suite, and treating that
as the state of the tree. `internal/component/plugin/server` failed on the very test
that pins R-10. Functional green says nothing about unit packages, and this spec's
evidence must name both.

**What the parse fixtures do and do not prove.** `test/parse/process-receive-leaflist.ci`
and its four siblings were restored because their subject had been deleted rather than
migrated. They assert `ze config validate` exit 0, so they discriminate the GRAMMAR and
not the SEMANTICS. They are NOT filter coverage, and this spec must not count them as
such: a `receive` list parsed and then ignored is precisely the regression this work
exists to fix, and fixtures of exactly that shape stayed green through all seven months
of it.

**One earlier finding's provenance was wrong, and the fix stands anyway.** Round 1 cited
five `test/exabgp-compat/native/api-*.conf` as proof the brace form `receive { update }`
was live config. Those files are excluded from parser coverage by name in
`internal/component/bgp/config/loader_test.go` as legacy awaiting conversion, and the
form does not parse: `expected value or ';' in receive, got LBRACE`. Reading it in the
guard is fail-closed and harmless, so it stays; the claim about why is corrected.

## Pre-Commit Verification

Re-checked by the main thread at `ded58c666`, not copied from an agent's report.
The spec is NOT closed: what blocks closure is named at the end.

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| the seven `test/plugin/attach-process-*.ci` fixtures | Yes | `git cat-file -e HEAD:<path>` succeeds for receive-filter, unattached-is-silent, reload, runtime-subscribe, dynamic-group, send-permission and delivery-graph |
| `internal/component/plugin/server/delivery_graph.go` | Yes | in HEAD |
| `internal/component/bgp/reactor/send_permission.go` | Yes | in HEAD |
| `internal/component/plugin/sender.go` | Yes | in HEAD |
| `internal/core/events/token.go` | Yes | in HEAD |
| `internal/component/config/flatten.go` and `retired.go` | Yes | both in HEAD |

### AC Verified (grep/test)

| AC | Evidence |
|----|----------|
| AC-1, AC-3 | `attach-process-receive-filter.ci`; discrimination proven by returning the process set unfiltered, which reds it |
| AC-2 | `attach-process-unattached-is-silent.ci`: the silent program refuses the first event it ever receives |
| AC-4 | `attach-process-reload.ci`: the added edge refuses any event beating the SIGHUP marker |
| AC-5 | `attach-process-runtime-subscribe.ci`, plus `TestRuntimeSubscribeSurvivesAPublishThatIsNotAnApply` and `TestConfigApplyDiscardsRuntimeSubscriptions` for the two halves |
| AC-6, AC-6b | `attach-process-dynamic-group.ci` and `TestDynamicPeerEntersTheIndexUnderItsOwnAddress` |
| AC-7, AC-7b | `TestReconcileNamesAProcessNoPeerAttaches`, which also stays silent when both halves agree |
| AC-8 | `TestGraphLookupAllocatesNothing` measures 0 on hit and miss; `TestPeerScopedProcsAddsNoAllocation` shows the filter adds none |
| AC-9, AC-10, AC-11 | `attach-process-send-permission.ci` and `TestSendPermissionRefusesUnattachedPeer`; the four formerly ungated rails by `TestRailsRefuseACommandWithNoSender` and four entry-point deny tests |
| AC-12 | `TestParseRefusesRetiredProcessKeyword`. AC-12b withdrawn on the owner's ruling that ze is not released |
| AC-13 | `TestLoadEditCommitKeepsBothAttachedProcesses` and its two siblings |

### Wiring Verified

| Claim | Evidence |
|-------|----------|
| Every peer-scoped delivery path reaches the graph | All seven sites in `internal/component/bgp/server/events.go` call `PeerScopedProcs`; `deliverEvent` branches to it when a peer address is present |
| Every rail to a peer's wire is permission-checked | `filterPermittedPeers` is the one shared filter, read at `send_permission.go`; the four rails that bypassed the resolver now carry a `plugin.Sender` and are tested from their registered wire methods |
| The index is republished wherever the peer set changes | startup, `AddPeer`, `doRemovePeer`, `createDynamicPeer`, `removeDynamicPeer`, `reconcilePeersJournaled` |

### Assumptions Resolved

| ID | Status |
|----|--------|
| A-1 | validated, owner statement |
| A-2 | broken, then repaired by phase 2: all 15 declarations read, ten needed vocabulary that did not exist |
| A-3 | confirmed for its residual scope, the runtime `subscribe` name selector |
| A-4 | confirmed by `TestGraphSwapIsAtomicAcrossReload` under `-race` |

### Not Verified, and blocking closure

**Until every row here is cleared this spec MUST NOT be removed from `plan/`.**

| What | State | Evidence |
|------|-------|----------|
| Review Gate artifact | OWED | Five rounds ran and none was recorded. `ls tmp/review/` holds nothing for this spec, so `review_gate.py check` cannot pass and a closure commit is refused |
| `make ze-verify` | NEVER RUN | The pre-commit gate. In a checkout this busy it cannot come back green, so its reds must be attributed rather than waved through |
| `make ze-rfc-check` | RED | RFC7606-5.1-2 and 5.1-3 carry stale verdicts: `rfc7606-relay-one-field.ci` is a file-scoped tagged unit and its reject rule changed. `ze-rfc-reseal` refuses this by design, and the re-audit must be run by someone who did not author the change |

## Known Limitations

- Per-peer `content { encoding format }` stays inert. It resolves through the same dead
  method, but format and encoding are per-process last-writer-wins in the subscription
  producer, so honouring it is a separate change with its own design question.
- `Subscription.PluginFilter` is parsed and compared but never matched, so
  `subscribe plugin <name>` filters nothing. Adjacent defect, named here so it is not
  mistaken for a working precedent.
- **A plugin ze auto-loads from a config PATH is not attached by that path.** `watchdog { }`
  on a peer loads bgp-watchdog, `route-reflector-client` loads bgp-rs, a custom receive token
  loads its decorator, and none of them creates a delivery edge: the peer must also name the
  plugin in an `attach process` block. Phase 5 corrected every in-tree fixture that needed
  one (about 60) and taught the ExaBGP migrator to emit the block for a watchdog route, and
  the same requirement now falls on an operator. Whether asking for a FEATURE should imply
  the attachment is a design question this spec did not answer; the literal reading of AC-2
  is what is implemented.
- **The plugin must be attachable by a name the config knows.** `attach process <name>` takes
  the name in a `plugin { external <name> }` or `plugin { internal <name> }` block, or the
  registry name of a plugin loaded with `--plugin` (`ValidatePluginReferences`,
  `internal/component/bgp/config/plugins.go`). A plugin auto-loaded by a config path alone is
  refused there, so such a fixture must declare it as well.
- **A truthful `send [ update ]` on a plugin that never signals plugin-session-ready held the
  peer's End-of-RIB for the full initial-sync gate.** `peer_run.go` counts the permission into
  `ResetAPISync`, so the marker waited out `apiSyncTimeout` while `ShouldQueue` stayed true, and
  an event-driven announce raised inside that window drained AHEAD of the marker. **Phase 6
  resolved it in two places**: bgp-watchdog now sends `plugin session ready` on every peer-up
  (`watchdogServer.signalReady`), and the fixed 500ms pre-sleep is gone, leaving one bounded
  wait that ends the moment every expected signal arrives (`sendInitialRoutes`). What survives
  is any OTHER plugin granted the permission that never signals: it still waits out the timeout.
  `spec-fixit-forward-rail-initial-sync-ordering` closed on 2026-08-11 and fixed the forward
  rails, not this hold, so the residual has no owning spec; `plan/journal/grace-bound-hides-a-missing-signal.md`
  carries its row.

## RFC Documentation (Scope: protocol)

Not applicable. No wire behaviour changes.

## Checklist

The Integration Checklist and the Documentation Update Checklist are filled in full
at the write gate, once the design names the files.

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
