# Spec: filter-wire-0-umbrella -- BGP policy filters without the text round-trip (route-policy phase 4, BGP half)

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P3 |
| Release reason | Release-independent filter wire 0 umbrella work; historical findings do not establish a current product defect. |
| Depends | Route-policy phases 1-3, landed (490618ef43). This builds on their library in `internal/component/policy/`: `policy.CompileRouteChain`, `policy.RouteChain` and `policy.CheckChain`; the field registry, `policy.MustRegisterField` with a `policy.RouteView` (`Read`, `Written`); `policy.MustRegisterRouteAttachment`; and `ribpolicy.Route` and `ribpolicy.Sets` (`internal/core/rib/policy`) |
| Phase | - |
| Updated | 2026-10-31 |

## Read this first

Status: **ready**, body current at c032f72335 (2026-10-31). Every phase is
unblocked: the owner answered R-12 on 2026-10-31 with option (a), recorded in
the revised D-6.

**D-6 was revised by the owner on 2026-10-31 (R-12).** The five simple
filters, `filter_aspath`, `filter_aspath_length`, `filter_community_match`,
`filter_path_asn` and `filter_prefix`, become route-policy library `match`
conditions and stop being plugins. Each condition registers from its own
package; `ze config migrate` rewrites the old filter names and instances into
the equivalent `match` conditions; the five plugin wrappers are deleted, with
no layering. They are pure checks on route fields and hold no state of their
own, so a forked variant buys nothing. At HEAD `matchConditions` is a literal
table in `policy/match.go` and `RouteMatch` holds one field per condition, so
phase 4 first turns the condition set into a registry. AC-14, AC-22 and AC-30
are revised, AC-10 gains the migrated configs, and AC-38 to AC-41 are added.

**The owner approved the design on 2026-10-31.** He accepted all twelve
recommendations for the open design questions (Q-1 to Q-12). Each answer is a
row of `## Key Design Decisions`, marked "owner, 2026-10-31", and every AC,
wiring row, test, risk, assumption, file entry and phase that waited on a
question now states the decided behaviour, citing the decision as D-1 to
D-12. The design gate is complete: `/ze-implement` runs next, starting with
the child specs named in `## Implementation Steps`.

Every section from `## Task` through `## Work Inherited From a Deferral Row`
was rewritten against HEAD on 2026-10-31. It replaces the July 2026 design,
which `## Disposition` (kept at the end as the audit trail) judged REWORK. The
owner's fixed requirements in the Disposition are stated as requirements in
`## Requirements`; nothing in this body overrides them.

**D-12 was revised by the owner on 2026-10-31** and supersedes the earlier
D-12 ("a compiled-in filter type always evaluates in process"). A filter is
owned by its plugin, whatever process the plugin runs in. A plugin running
internal is called through a typed `DirectBridge` filter slot, the same way
`DeliverStructured` hands it events, with the lazy wire view in and the verdict
and sets back, nothing encoded; a plugin running forked speaks the batched
protocol of D-1 to D-11. The author writes one evaluator. `filterapi.Register`,
the in-process-only filter family, merges into that one mechanism (D-13). The
revision closes R-9, adds AC-31 to AC-37, and puts the IRR design in
`## Implementation Steps`, phase 4.

Facts at HEAD that shape how the decisions land, each with a risk row: the
event encoders D-1 reuses are registered by plugins (R-10); the `match` type's
`metric` condition stops being usable at a BGP attachment under D-8 (R-11); and
the revised D-12 meets four facts it does not settle by itself: D-6's `match`
conditions sit in the library, not in a plugin (R-12, closed by the revised
D-6: they are the library's and run where the library runs); an out-of-tree Go module
has no route into a compiled ze or to the view type (R-13); some
`filterapi.Register` filters read reactor-only state (R-14); and the bridge's
generic callback runs on the plugin's one event-loop goroutine, where the typed
slot runs on reactor goroutines (R-15).

Recovery after compaction: `.claude/rules/post-compaction.md`. Re-read this
file, then `docs/architecture/core-design.md` ("Ingress Filter Pipeline",
"Route Metadata and Modification Accumulator", "Policy Filter Chain"), then
`plan/post-release/features/spec-route-policy-0-umbrella.md` ("Route Field
Vocabulary", AC-17, Implementation Steps phase 4).

## Task

**Goal: no text round-trip on the BGP filter path.** Today every UPDATE that
meets an active external policy filter is rendered from wire bytes to a text
subject, sent to each filter plugin as text, answered with a text delta,
merged as text, parsed back, and converted to wire attribute operations
(see Current Behavior below). This spec removes that text path and puts BGP's import
and export chains on the shared route-policy library. It is the BGP half of
route-policy phase 4 (`spec-route-policy-0-umbrella`, AC-17 and Implementation
Steps phase 4).

Two evaluation paths share one vocabulary, the library's registered route
fields:

| Path | Who uses it | What it carries | How a field is decoded |
|------|-------------|-----------------|------------------------|
| In-process fast path | A plugin running internal (a goroutine in the daemon, started by `startInternal`), reached through a typed `DirectBridge` filter slot (D-12) | A lazy view over the received `WireUpdate`, handed over with no encoding | On first read, by the view, from the wire bytes |
| Out-of-process slow path | A plugin running forked, in any language, an in-tree plugin configured `external` included, speaking the process protocol | A language-neutral, versioned message: the fields the plugin declared at startup, decoded; the raw UPDATE only when declared | Once per route, only for the fields declared |

The two paths are two transports of one filter: the same plugin, the same
evaluator, the same verdict for the same route (AC-31).

Both paths answer a verdict plus set fields named from the same vocabulary,
and Ze turns every set field into the same UPDATE change through the same
attribute handlers. One chain may mix members of both paths. The text
`Update` format of the `filter-update` RPC is replaced, not kept beside the
new one, and the in-tree BGP filter plugins move with it.

**Scope.** BGP import and export policy chains (`filter { import|export { ...
} }` at bgp, group and peer level), the `filter-update` contract in
`pkg/plugin/rpc` and `pkg/plugin/sdk`, the reactor call sites, the dry-run
surface (`show policy test`), the nine in-tree BGP filter plugins (five of
which become library `match` conditions and stop being plugins, D-6), the
`match` condition registry, the `ze config migrate` transformation for the old
filter names, and the protocol pages. Loc-RIB, redistribution and IGP chains are route-policy
phases 2 and 3 and stay untouched.

## Required Reading

<!-- NEVER tick [ ] to [x]. Annotations are the knowledge. -->

### Design doors
- [ ] `ai/doors/system.md` - the design every subsystem shares
  → Constraint: a third-party plugin "in Go or any other language" is a forked process speaking the process protocol; the slow path's contract is that protocol, never a Go type. A change under `pkg/` is a published contract change, made on purpose with its page.
  → Constraint: DirectBridge is "an in-process optimization of the same protocol, not a second protocol". A typed bridge slot for `filter-update` is exactly that: the same callback a forked plugin answers over IPC, delivered without encoding, as `DeliverStructured` delivers events (D-12). It is not a registration beside the protocol, and an in-tree plugin run forked still works through the slow path.
- [ ] `ai/doors/bgp-engine.md` - reactor, wire, filter pipeline
  → Constraint: "A policy plugin" depends on the `filter-update` RPC (`FilterUpdateInput`, `FilterUpdateOutput`); changing it breaks code outside the repository, so the page edit lands in the same change.
  → Constraint: received bytes are borrowed (`RawMessage.IsAsyncSafe()` false); a lazy view over them must not escape the synchronous chain run, and a batch held across goroutines must copy or retain the cache entry first.
  → Constraint: the trap "a filter modify rewrote only the IPv4 body NLRI" (2026-10-17) is what `sectionNLRIDelta` fixed; its MP_REACH/MP_UNREACH behaviour must survive the redesign.
- [ ] `ai/doors/bgp-plugins.md` - the filter plugins
  → Constraint: in-tree plugins hold no privilege over a third-party one on the protocol; every in-tree filter must be reachable through the documented contract. The fast path is a transport of that contract (a typed bridge slot), not a replacement of it. The door's hook table lists `filterapi.Register` filters as in-process only, "Go function pointers"; D-13 merges them into the plugin filter so each one also works forked, or names why it cannot.
  → Constraint: "a plugin declares twice, and both must agree" (`registry.Registration` and `sdk.Registration`); the field declaration a filter makes must come from one source both read.
- [ ] `ai/doors/plugin-infra.md` - process protocol, SDK
  → Constraint: "a field or method name in `rpc` is wire, not Go: renaming a JSON tag breaks every plugin written in another language"; the new message shape is designed as JSON first.
  → Constraint: one engine-op entry per RPC; the SDK stays generic (one `On*` method per callback); a request line is newline-framed, so payloads are compact `json.Marshal` output.
- [ ] `ai/doors/core.md` - `internal/core/rib/policy`
  → Constraint: `ribpolicy.Route` and `ribpolicy.Sets` are core leaf value types read by the Loc-RIB gate; BGP attributes are not added to them (route-policy Key Design Decisions: the Loc-RIB best carries no communities or AS path).
- [ ] `ai/doors/api-surfaces.md` - read because `show policy test` and the web BGP policy pages render filter results
  → Constraint: adapters own no command; a dry-run rendering change is made in the command's owner (`internal/component/bgp/plugins/cmd/policy`) and reaches every surface.
- [ ] `ai/doors/le-tooling.md` - functional and interop harness, `./le perf`
  → Constraint: a `.ci` needing Linux carries `option=needs-linux`; a perf run happens only on Thomas's Mac, so the harness is prepared and handed over, never run by an agent.

There is no door for `internal/component/policy`; `system.md` covers it.

### Architecture Docs
- [ ] `docs/architecture/core-design.md`, "Ingress Filter Pipeline"
  → Decision: ingress runs one stage-ordered pass over `r.orderedIngressSteps`; the external per-peer chain is ONE ordered step at `filterapi.FilterStagePeerChain` (300), after every in-process filter. The redesign keeps that position for the chain as a whole.
  → Constraint: after the pass the cached `WireUpdate` is the canonical post-filter representation; copy on modify; a pure withdrawal stays on the unchanged-payload path with no new `WireUpdate`.
  → Constraint: `PeerFilterInfo.ASN4` describes the bytes (four-octet after `Session.collapseASPathFamily`), not the session; the lazy view's `as-path` decodes at that width.
- [ ] `docs/architecture/core-design.md`, "Route Metadata and Modification Accumulator" and "Progressive Build (applyMods)"
  → Decision: an UPDATE change is a list of `AttrOp` (`Code`, `Action`, `Buf`) in a `filterapi.ModAccumulator`, applied by `buildModifiedPayload` through per-code `AttrModHandler`s. A set field becomes ops through this, never through a new apply engine.
  → Constraint: the accumulator is hoisted above the destination loop and `Reset` per destination; nothing may keep a slice from `Ops()` past `Reset`. A modification that cannot be applied suppresses the route for that destination (`modifyFailure`).
  → Constraint: for COMMUNITY, EXTENDED_COMMUNITY and LARGE_COMMUNITY, `Buf` holds a whole number of fixed-width values; the handler checks it.
- [ ] `docs/architecture/core-design.md`, "Policy Filter Chain"
  → Decision: the chain is protocol-neutral (`policy.Ref`, `policy.Final`, `policy.Chain`, `policy.ConcatChains`, `policy.CanonicalName`); BGP compiles it into `filterapi.Policy`, which adds the in-process bindings (`filterapi.BindInProcess`), `RequiresIngress` and the RFC 8212 default (`Policy.AcceptsAtEnd`). These semantics are preserved exactly.
  → Constraint: accept continues; reject, fail-closed error and teardown are terminal; exhaustion applies the final; a raw rewrite stops later filters and still receives the final.
- [ ] `docs/architecture/api/process-protocol.md`, "Filter Declaration (Stage 1)", "filter-update", and the four sections from "The Attribute Names in the Filter Text Protocol" to "NLRI Blocks and the Modify Answer"
  → Constraint: `filters[]` in `declare-registration` already carries `name`, `direction`, `attributes`, `nlri`, `raw`, `on-error`, `overrides`; the startup declaration is where the field list lives, so no new stage is needed for it.
  → Constraint: the four text-protocol sections define today's contract (thirteen attribute names, AS4 merge, non-CIDR marker blocks, NLRI subset rules); each is replaced by its new-contract equivalent in the same change that removes the text.
  → Constraint: there is no protocol version field anywhere in the startup today (grep of the page and of `DeclareRegistrationInput`, rechecked at 5d9b21066b), and Stage 1 is answered with a bare `#1 ok`; D-2 adds the plugin-level version to `declare-registration` and the version Ze will speak to that answer.
- [ ] `docs/architecture/api/process-protocol.md`, "Mode 1: In-Process (goroutine + net.Pipe + DirectBridge)", its "Runtime hot path" table, and "DirectBridge Optimization"; `pkg/plugin/rpc/bridge.go` (`DirectBridge`, `SetDeliverStructured`, `DeliverStructured`, `DeliverEvents`, `DispatchRPC`, `SendCallback`, `CloseCallbacks`); `plan/learned/DESIGN-HISTORY.md`, the two rejected "Direct-wire in-process delivery" attempts (1a5aa8029b introduced the bridge)
  → Decision: the fast path of D-12 is one more typed slot on this bridge. `DeliverStructured` is the precedent: the SDK registers the handler after Stage 5 (`SetDeliverStructured`), the engine calls it directly on its own goroutine, and `beginDelivery` refuses with `ErrBridgeClosed` once `CloseCallbacks` ran.
  → Constraint: `filter-update` has no typed slot at HEAD. `(*Server).CallFilterUpdate` calls `(*PluginConn).SendFilterUpdate` (`internal/component/plugin/ipc/rpc.go`), whose `CallRPC` marshals JSON and, with a bridge, calls the generic `DirectBridge.SendCallback`, which queues on `callbackCh` for the plugin's `bridgeEventLoop` goroutine. A compiled-in filter therefore skips the pipe but pays for the text subject and two JSON passes per route.
  → Constraint: DESIGN-HISTORY rejected an eager `StructuredEvent` and an `UpdateHandle` identity wrapper; the third attempt passed the raw message and used the existing wire iterators. The view of D-7 must not be either rejected shape.
- [ ] `docs/architecture/api/architecture.md`, "Wire Bytes in Events", and `docs/architecture/api/json-format.md`
  → Decision: the UPDATE event already has a decoded JSON attribute map (`attr`) beside optional raw hex (`raw`); D-1 takes its value shapes for the filter message, so a plugin author reads one JSON style for events and filters.
  → Constraint: the `attr` keys come from `attribute.RegisterJSONFormatter`, and three differ from the vocabulary names D-1 keys by: `communities`, `large-communities`, `extended-communities` against the fields `community`, `large-community`, `extended-community`. The filter message keys by the field name and borrows only the value shape. The `attr` table has no row for `prefix`, `family`, `protocol`, `origin-as` or `next-hop` (an `nlri` group member in events), so those shapes are written on the protocol page.
  → Constraint: the community formatters are registered by `bgp-filter-community` (`filter_community/register.go`) and `originator-id`, `cluster-list` by `bgp-rr` (`rr/register.go`), not by the engine; reuse depends on them being linked (R-10).
- [ ] `docs/plugin-development/protocol.md`, "Runtime Callbacks (Engine to Plugin)" and "The Filter Text an `OnFilterUpdate` Handler Reads"
  → Constraint: the author-facing page promises `rpc.FilterUpdateTimeout` (5 s) per call and the on-error policy on expiry; under D-3 the page restates it as one bound per batch, with on-error applied to each route of a failed batch.
- [ ] `docs/guide/route-policy.md` - the library as operators see it
  → Constraint: the Loc-RIB gate never calls a plugin process and refuses a `<plugin>:<filter>` reference; that refusal stays. BGP chains, which do call plugin processes, are the slow path's only attachment.
- [ ] `docs/guide/bgp-policy.md` - the BGP operator contract
  → Constraint: chain inheritance (bgp, group, peer), `final` per direction, inactive references, RFC 8212 default and dry-run output are operator-visible and preserved.
- [ ] `docs/architecture/plugin/plugin-system.md`, "The process boundary" and "Cross-boundary value types"
  → Constraint: what crosses the boundary is a self-contained value; a lazy view is in-process only and never crosses it.
- [ ] `plan/post-release/features/spec-route-policy-0-umbrella.md` - "Route Field Vocabulary", Key Design Decisions, AC-17, Implementation Steps phase 4, "Phase 3 record"
  → Decision: "BGP import stays ingress": the BGP chain stays the per-peer ingress and egress chain; the library never adds a text representation for BGP (its R-6).
  → Decision: the BGP attachment carries `prefix`, `family`, `protocol`, `next-hop`, `metric`, `origin-as`, the three community lists, `as-path`, `local-preference`, `origin`, `aigp`, `med`, and lets a filter set the BGP attributes of `modify` (its "Fields carried" and "Fields a filter may set" tables).
  → Constraint: phase 4's verify line is "BGP throughput benchmark prepared for Thomas; no new text representation".

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - UPDATE layout and path attributes
  → Constraint: every set field produces a well-formed attribute (flags, length) through the existing handlers; a field the view decodes from a malformed attribute is an error, never a zero.
- [ ] `rfc/short/rfc6793.md` - AS4_PATH reconstruction
  → Constraint: the `as-path` field a filter reads is the reconstructed path (RFC 6793 Section 4.2.3), as the text `as-path` pair is today; a filter that needs the encoded AS_PATH and AS4_PATH declares raw.
- [ ] `rfc/short/rfc6996.md` - private ASN ranges
  → Constraint: remove-private-as keeps reactor-side computation that needs the peer AS and ASN4 width (`ExtractRemovePrivateASOps`).
- [ ] `rfc/short/rfc7454.md` - BGP operations and security guidance behind default import filtering
  → Constraint: fail-closed on filter error stays the default `on-error`.

**Key insights:**
- The text subject is derived from wire at filter time for every route (`AppendUpdateForFilter` with a nil declared list), so the round-trip is pure overhead: by the time a filter runs, every route is already wire.
- BGP's chains already pass `policy.CheckChain` at load for named instances (`validateChainNames` in `internal/component/bgp/config/peers.go`, attachments `bgp import` and `bgp export`), but an external `<plugin>:<filter>` reference skips it ("validated by that plugin at runtime"), because `FilterDecl.Attributes` is not mapped onto the vocabulary.
- The BGP fields are registered with an empty `policy.RouteView{}` (`internal/component/policy/register.go`): `RouteView` reads a `ribpolicy.Route`, which holds no BGP attribute. The lazy wire view therefore needs its own read path: under D-7, readers registered from the BGP side over `WireUpdate` and `AttributesWire`.
- The `match` type (`policy.MatchFilterType`, registered in `internal/component/policy/register.go`) evaluates the conditions of the literal table `matchConditions` (`policy/match.go`): `prefix-list`, `family`, `protocol`, `next-hop`, `interface`, `metric`, `tag`, `level`, `route-type`. It has no AS-path or community condition, so D-6 adds the conditions of the five plugin types it folds in, each registered from its own package rather than written into that table. The `modify` type is already `filter_modify`'s (`filterTypeName = "modify"`).
- `matchConditions` is not a registry at c032f72335. It is an unexported slice literal of `matchCondition` (`key`, `reads`, `source`, `parse`), and no exported function adds a row. Its `parse` functions write named fields of `RouteMatch` (`prefixList`, `families`, `interfaces`, `hasMetric`, `tags`, `levels`, `routeTypes`, ...), and `(*RouteMatch).Matches` tests each of those fields in a fixed sequence. A condition can therefore not be added from outside `match.go`: the table, the struct and `Matches` all name every condition (`ai/rules/principles.md`, registration over enumeration).
- `filter_path_asn` is tied to `filterapi` only through `registry.Registration.FilterObligations` (`filterapi.ObligationTransitLeak`), read by `validateLeakFilterObligations` through `registry.FilterTypesDischarging` (`bgp/config/peers.go`); it does not call `filterapi.Register`. It also carries the `show ... reject-asn` commands (`register_command.go`, `command.go`) and metrics (`metrics.go`), which outlive its plugin wrapper (R-16).
- `ze config migrate` already takes registered transformations: `migration.MustRegisterTransformation` (`internal/component/config/migration`), used by `internal/plugins/ospf/register_migration.go` and `internal/plugins/redistribute/register_migration.go`.
- Three BGP filter plugins already compile in-process route evaluators for the Loc-RIB (`filter_prefix`, `filter_family`, `filter_modify`, each `rib_evaluator.go`), but over `ribpolicy.Route`, not over an UPDATE; BGP chains still reach them by RPC.
- Ze already has the two-transport pattern for events: an internal plugin receives `*rpc.StructuredEvent` through `DirectBridge.DeliverStructured`, a forked one receives JSON. `filter-update` was never given its slot, and `filterapi.Register` grew beside it as a second, in-process-only filter family. D-12 adds the slot; D-13 merges the family.

## Current Behavior (MANDATORY)

Read at 544f8f930c. Symbols named by file; line numbers deliberately omitted.

**Source files read:**
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - import and export entry, the round-trip call sequence
- [ ] `internal/component/bgp/reactor/filter_chain.go` - `PolicyFilterChain`, `applyFilterDelta`, `parseFilterAttrs`, `formatFilterAttrs`, `policyFilterFunc`, `validateModifyDelta`, `decodeFilterRawOverride`
- [ ] `internal/component/bgp/reactor/filter_format.go` - `AppendUpdateForFilter`, `AppendAttrsForFilter`, `asPathForFilter`
- [ ] `internal/component/bgp/reactor/filter_delta.go` - `textDeltaToModOps`, `ExtractASPathPrependOps`, `ExtractRemovePrivateASOps`
- [ ] `internal/component/bgp/reactor/filter_nlri_delta.go` - `sectionNLRIDelta`, `mpSectionDelta`, `subsetNLRI`
- [ ] `internal/component/bgp/reactor/forward_build.go`, `filter_delta_handlers.go` - `buildModifiedPayload`, `attrModHandlersWithDefaults`
- [ ] `internal/component/bgp/reactor/policy_dryrun.go`, `peer_initial_sync.go` - the other callers
- [ ] `internal/component/bgp/filterapi/filterapi.go`, `editset.go`, `policy.go` - `ModAccumulator`, `AttrPlan`, `Filter`, `Register`, `RegisterAttrModHandler`, `BindInProcess`, `Policy`
- [ ] `pkg/plugin/rpc/types.go`, `pkg/plugin/rpc/bridge.go` - `FilterDecl`, `FilterUpdateInput`, `FilterUpdateOutput`, `FilterUpdateTimeout`, `DirectBridge.SendCallback`
- [ ] `pkg/plugin/sdk/sdk_callbacks.go`, `pkg/plugin/sdk/sdk_types.go` - `FilterUpdateHandler`, `OnFilterUpdate`
- [ ] `internal/component/plugin/server/server.go` - `CallFilterUpdate`
- [ ] `internal/component/bgp/config/register.go`, `peers.go` - the `bgp import`/`bgp export` attachments and `validateChainNames`
- [ ] `internal/component/policy/fields.go`, `register.go`, `rib.go`, `rib_compile.go` - the vocabulary and `CompileRouteChain`
- [ ] `internal/core/rib/policy/policy.go` - `Route`, `Sets`
- [ ] `internal/component/bgp/plugins/filter_*/register.go` and each plugin's `filter-update` handler

**The text round-trip (import and export):**

| Step | Producer | What it does |
|------|----------|--------------|
| 1 | `(*Reactor).runIngressPolicyChain`, `(*Reactor).runEgressPolicyChainASN4` (`internal/component/bgp/reactor/filter_ordered.go`) | Entry for import and export. Skips the chain when no active external filter (`hasActiveFilter`); fails closed when the API server is absent |
| 2 | `AppendUpdateForFilter` (`reactor/filter_format.go`) | Renders the whole UPDATE to text into a stack scratch buffer, always with a nil declared list, so every rendered attribute is present (`AppendAttrsForFilter`, `asPathForFilter` merges AS4_PATH, `attrNameToCode`) |
| 3 | `PolicyFilterChain` (`reactor/filter_chain.go`) | Walks `filterapi.Policy` refs: skips inactive and in-process-bound refs, calls each external filter through the `PolicyFilterFunc`, folds each modify delta into the running text with `applyFilterDelta` (which parses both sides with `parseFilterAttrs` and re-renders with `formatFilterAttrs`), applies the final |
| 4 | `(*Reactor).policyFilterFunc` (`reactor/filter_chain.go`) | Per filter: reads `FilterInfo` (declared attributes, raw flag), builds `rpc.FilterUpdateInput` (`Filter`, `Direction`, `Peer`, `PeerAS`, `Update` text, `Raw` when declared), calls `CallFilterUpdate` with `rpc.FilterUpdateTimeout`, applies `on-error` on failure, rejects a modify delta touching an undeclared attribute (`validateModifyDelta`, only when attributes were declared), honours teardown on import only |
| 5 | `parseFilterAttrsInto` twice, `textDeltaToModOps` (`reactor/filter_delta.go`) | Diffs the original and final text and emits `ModAccumulator` ops per attribute |
| 6 | `ExtractRemovePrivateASOps`, `ExtractASPathPrependOps` (`reactor/filter_delta.go`) | The two reactor-side intents: remove-private needs the raw attributes, ASN4 width and peer AS; prepend needs the local AS |
| 7 | `sectionNLRIDelta`, `mpSectionDelta`, `subsetNLRI` (`reactor/filter_nlri_delta.go`) | Rewrites the NLRI, Withdrawn, MP_REACH_NLRI and MP_UNREACH_NLRI sections from the text `nlri add/del` and `mp <family> add/del` blocks; a block may only remove prefixes, in order; a non-CIDR marker must be echoed; anything else drops the route (`nlriRefusal`) |
| 8 | `buildModifiedPayload` (`reactor/forward_build.go`) | Progressive build from the accumulator through `r.attrModHandlers` (`attrModHandlersWithDefaults` in `reactor/filter_delta_handlers.go`: `genericAttrSetHandler`, `aspathHandler`, `mpReachNextHopHandler`, `mpUnreachNLRIHandler`, `originatorIDHandler`, `clusterListHandler`, `tombstoneHandler`, plus handlers registered with `filterapi.RegisterAttrModHandler`) and the NLRI override |

Other callers of the same machinery: `peer_initial_sync.go` runs
`PolicyFilterChain` on a synthetic default-originate subject
(`appendDefaultOriginateFilterText`, nil raw payload); `policy_dryrun.go`
(`PolicyDryRun`, `tracePolicyFilterChain`) renders, runs and diffs the same
text for `show policy test`; `egress_inject_filter.go` calls
`buildModifiedPayloadWithLimit` without text. A raw full-body rewrite is
validated by `decodeFilterRawOverride` and is terminal.

**The contract (`pkg/plugin/rpc/types.go`, `pkg/plugin/sdk`):**

| Type | Fields |
|------|--------|
| `FilterDecl` | `Name`, `Direction`, `Attributes` (text attribute names), `NLRI`, `Raw`, `OnError`, `Overrides` |
| `FilterUpdateInput` | `Filter`, `Direction`, `Peer`, `PeerAS`, `Update` (text), `Raw` (bytes, base64 in JSON) |
| `FilterUpdateOutput` | `Action` (`accept`, `reject`, `modify`), `Update` (text delta), `Raw` (full body), `Teardown`, `NotifyCode`, `NotifySubcode` |
| `sdk.FilterUpdateHandler`, `(*Plugin).OnFilterUpdate` (`sdk_callbacks.go`) | Unmarshal JSON, call, marshal. `sdk.FilterUpdateInput` and `sdk.FilterUpdateOutput` are aliases of the rpc types (`sdk_types.go`) |
| Transport | `(*Server).CallFilterUpdate` (`internal/component/plugin/server/server.go`). An internal plugin's callback crosses `rpc.DirectBridge.SendCallback`, which still carries `json.RawMessage`: there is no typed bridge for this callback, so even in-process plugins marshal JSON |

`internal/component/bgp/filtertext` (`ASPath`, `CommunityValues`,
`HasCommunity`, `CanonicalCommunity`, `CIDRFamily`) is the plugin-side text
parser the in-tree filters share.

**The accumulator (`internal/component/bgp/filterapi`):** `ModAccumulator`
with `Op`, `OpCopy`, `OpGen` (an `AttrGenerator` writes a value of known
length in place), `RemoveOps`, `GroupedOps`, out-of-band `SetWithdraw`,
`SetNLRIRewrite`, `SetWithdrawnRewrite`, and a per-destination rebuild plan
`EditSet`; actions `AttrModSet`, `AttrModAdd`, `AttrModRemove`,
`AttrModPrepend`, `AttrModSuppress`; `AttrModHandler` is a function of an
`AttrPlan` (`editset.go`). In-process filters register with
`filterapi.Register` (`gr`, `filter_community`, `role`, `rs`, and the loop
filter in `reactor/filter`), with `IngressFilterFunc` and `EgressFilterFunc`.

**The in-process-only filter family (`filterapi.Register`), read at HEAD:**

| Registration | Package | Stage | Hooks | What it reads besides the UPDATE |
|--------------|---------|-------|-------|----------------------------------|
| `loop` | `internal/component/bgp/reactor/filter` (engine, not a plugin) | `FilterStageProtocol`, `PolicyType` `loopFilterType` (bound by `BindInProcess`) | ingress, egress | `PeerFilterInfo` `LocalAS`, `RouterID`, `ClusterID`, `AllowOwnAS`, `LoopDisabled` |
| `bgp-filter-community` | `plugins/filter_community` | `FilterStagePolicy` | ingress, egress, `ConfigureMetrics` | the plugin's own config; also registers the COMMUNITY, LARGE_COMMUNITY and EXTENDED_COMMUNITY `AttrModHandler`s |
| `bgp-filter-community-relation` | `plugins/filter_community` | `FilterStageAnnotation`, priority 1 | ingress | `meta["src-role"]`, which `bgp-role` writes at priority 0 of the same stage |
| `bgp-role` | `plugins/role` | `FilterStageAnnotation` | `OTCIngressFilter`, `OTCEgressFilter` | the plugin's per-peer role config; writes `meta["src-role"]`; registers the OTC `AttrModHandler` |
| two `bgp-gr` filters | `plugins/gr` | `FilterStageAnnotation` (LLGR egress), `FilterStageProtocol` (`LLGRIngressFilter`) | egress, ingress | `PeerFilterInfo.LLGRAdvertised`, filled by the reactor from the session's OPEN or the relay rail; `meta["stale"]` |
| `bgp-rs` | `plugins/rs` | none | `PreserveOpaqueAttributes` only | RFC 7947 Section 2.2 forwarding choice, no verdict |

`filter_path_asn` imports `filterapi` only for `ObligationTransitLeak`; it
registers no in-process filter. Every row is a Go function pointer that exists
only in a binary linking the package (`ai/doors/bgp-plugins.md`, the hook
table), so none of them works with its plugin forked.

**The nine in-tree filter plugins that answer `filter-update`** (all under
`internal/component/bgp/plugins/`). This is HEAD. Under the revised D-6 the
first four rows and `filter_prefix` stop being plugins and become library
`match` conditions; the other four stay plugins:

| Plugin | Policy type registration | Reads today | Answers | Library field declaration (`register.go`) | Loc-RIB evaluator |
|--------|--------------------------|-------------|---------|--------------------------------------------|-------------------|
| `filter_aspath` | `FilterTypes` | `as-path` text via `filtertext.ASPath` | accept, reject | reads `as-path` | no |
| `filter_aspath_length` | `FilterTypes` | `as-path` hop count | accept, reject | reads `as-path` | no |
| `filter_community_match` | `FilterTypes` | the three community lists in text | accept, reject | reads the three community fields | no |
| `filter_path_asn` | `FilterTypes` | `as-path` text, plus `PeerAS` and `Direction` from the envelope | accept, reject | reads `as-path` | no |
| `filter_family` | `FilterTypes`, `FilterDecl` with `raw=true` | the raw body | reject, teardown (import), modify by full raw rewrite (strip MP sections, suppress unicast) | reads `family` | yes |
| `filter_prefix` | `FilterTypes` | NLRI blocks in text, FlowSpec destination prefix | accept, reject, modify by NLRI subset | reads `prefix`, `family` | yes |
| `filter_irr` | none: referenced as `bgp-filter-irr:<asn>`, ASN taken from the filter name, no `FilterDecl` | NLRI blocks in text | accept, reject, modify by NLRI subset; waits bounded on first IRR resolution | none, so the load-time field check never sees it | no |
| `filter_modify` | `FilterTypes` | its match fields (communities) and the current values of fields it increments | modify by text delta, including `as-path-prepend` | reads communities, `protocol`, `tag`, `level`, `route-type`; sets the BGP attributes plus the Loc-RIB sets | yes |
| `filter_remove_private_as` | `FilterTypes`, `FilterDecl` with `Attributes` `as-path` and the `remove-private` directive, `raw=true` | `as-path` text, AS4_PATH from raw | modify by the remove-private directive the reactor resolves | reads and sets `as-path` | no |

**The library from route-policy phases 1-3 (`internal/component/policy`,
`internal/core/rib/policy`):**

| Symbol | Role today |
|--------|-----------|
| `MustRegisterField` with `RouteView` (`Read` over `*ribpolicy.Route`, `Written` over `*ribpolicy.Sets`) | The vocabulary; BGP fields registered with an empty view |
| `MustRegisterFilterType`, `MustRegisterInstanceFields` | Fields a type (or one instance) reads and sets |
| `MustRegisterAttachment` | Used by BGP for `bgp import`, `bgp export` (`internal/component/bgp/config/register.go`), with the carries and settable lists above |
| `MustRegisterRouteAttachment` | Loc-RIB, redistribute and IGP export attachments, whose chains compile with `CompileRouteChain` |
| `CheckChain` | Load-time read/set refusal; BGP calls it through `validateChainNames` |
| `CompileRouteChain`, `RouteChain.Evaluate`, `MustRegisterRouteEvaluator`, `RouteEvaluator`, `RouteResult` | Compile and run an engine-judged chain over `ribpolicy.Route` by value, allocation-free, sets applied for the next filter |
| `ribpolicy.Route`, `ribpolicy.Sets`, `ribpolicy.Verdict` | The non-BGP route view and its set actions |

**How BGP chains attach today:** `policy.ExtractChain` and `ConcatChains`
build the chain from config; BGP wraps it in `filterapi.Policy`
(`NewPolicy`, `CompilePolicy`, `WithFilters`, `WithBoundFilters`, `Prepend`,
`AcceptsAtEnd`, `RequiresIngress`, `ActiveExternal`) carried through
`PeerSettings` as `ImportPolicy` and `ExportPolicy`; reload is in
`reactor/policy_reload.go` (`reconcileExportFinal`, `policyReplayOwner`).

**Behavior to preserve:**
- Every accept, reject, modify and teardown decision every existing BGP policy `.ci` and interop scenario asserts (route-policy AC-17).
- Chain semantics: inheritance, `final`, inactive refs, in-process bindings at their stage, RFC 8212 default, fail-closed and fail-open per `on-error`, teardown import-only, a raw rewrite terminal but still subject to the final, withdrawals preserved under `final reject`.
- The cached post-filter `WireUpdate` as canonical; copy on modify; the pure-withdrawal fast path.
- NLRI subset rules for every section, MP_REACH/MP_UNREACH included, with path identifiers and label stacks kept.
- The reactor-side computation of prepend (local AS) and remove-private (peer AS, ASN4 width).
- Dry-run decisions and trace (`show policy test`), and `show policy chain`.
- The 5 s per-call bound and the on-error policy on expiry.
- What the five simple filters decide, at every attachment that accepts them today (BGP chains, and `prefix-list` at the Loc-RIB, redistribute and IGP attachments through `filter_prefix`'s `rib_evaluator.go`), and the named list definitions under `policy` with their names.
- `bgp-filter-path-asn`'s operator surface: the `show ... reject-asn` commands, its metrics, and its discharge of the RFC 9234 transit-leak obligation (`validateLeakFilterObligations`).

**Behavior to change:**
- The `filter-update` message: text `Update` in and out is replaced by declared decoded fields, optional raw, and set fields from the vocabulary, under a negotiated contract version. Third-party text-format plugins stop working (owner-accepted).
- A filter of a plugin running internal is called through a typed `DirectBridge` filter slot over a lazy wire view, with no text and no JSON; the same filter of the same plugin run forked answers the batched protocol (D-12).
- The `filterapi.Register` family stops being a second filter mechanism (D-13).
- The five simple filter plugins (`bgp-filter-aspath`, `bgp-filter-aspath-length`, `bgp-filter-community-match`, `bgp-filter-path-asn`, `bgp-filter-prefix`) stop being plugins. Their checks become library `match` conditions, each registered from its own package, evaluated wherever the library runs and never over a plugin transport (D-6).
- A chain that names one of their filter types (`as-path-list`, `as-path-length`, `community-match`, `reject-asn`, `prefix-list`) as a member is refused at config load, naming the old reference, the `match` condition that replaces it, and `ze config migrate`; `ze config migrate` rewrites it into the equivalent `match` instance (D-6).
- `matchConditions` stops being a literal table: the condition set becomes a registry (D-6).
- `bgp-filter-irr` no longer waits inside its handler for a first resolution (`firstResolveWait`): an unresolved set takes the on-error verdict and the route is re-judged once the set resolves (AC-32).

## Existing Mechanisms (MANDATORY)

Read at HEAD. Each row is a mechanism Ze already has that does part of this
spec's job.

| Mechanism | Where it lives | What it does today | Relationship |
|-----------|----------------|--------------------|--------------|
| `DirectBridge` and its typed slots | `pkg/plugin/rpc/bridge.go` (`DirectBridge`, `SetDeliverStructured`, `DeliverStructured`, `DeliverEvents`, `DispatchRPC`, `CloseCallbacks`); `process-protocol.md`, "Mode 1: In-Process (goroutine + net.Pipe + DirectBridge)" and "Runtime hot path" | After Stage 5, carries events and plugin-to-engine RPCs between the engine and an internal plugin with no encoding; `DeliverStructured` calls the plugin's handler directly on the engine's goroutine and refuses with `ErrBridgeClosed` once closed | Extends: D-12 adds one typed `filter-update` slot of the same shape, installed by the SDK after Stage 5 |
| The generic `filter-update` callback route | `(*Server).CallFilterUpdate` (`internal/component/plugin/server/server.go`), `(*PluginConn).SendFilterUpdate` and `CallRPC` (`internal/component/plugin/ipc/rpc.go`), `DirectBridge.SendCallback` | Marshals a text-subject `FilterUpdateInput` to JSON for every route; with a bridge, queues it on `callbackCh` for the plugin's `bridgeEventLoop` goroutine, else sends it over MuxConn | Replaced: internal plugins take the typed slot; forked plugins take the batched, versioned message (D-1 to D-3); the text subject is deleted (AC-8) |
| `filterapi.Register` and `BindInProcess` | `internal/component/bgp/filterapi/filterapi.go` (`Filter`, `Register`, `IngressFilters`, `EgressFilters`, `BindInProcess`) | A second, in-process-only filter family: Go function pointers at fixed stages (`loop`, `bgp-filter-community`, `bgp-filter-community-relation`, `bgp-role`, `bgp-gr`, `bgp-rs`), absent when the plugin runs forked | Merged (D-13): each becomes a plugin filter on the bridge path that also works forked, or is named with the reason it cannot (R-14) |
| The route-policy library | `internal/component/policy` (`MustRegisterField`, `RouteView`, `MustRegisterFilterType`, `MustRegisterAttachment`, `MustRegisterRouteAttachment`, `CheckChain`, `CompileRouteChain`, `RouteChain.Evaluate`, `MustRegisterRouteEvaluator`); `internal/core/rib/policy` | The field vocabulary, the load-time read/set check, and engine-judged chains over `ribpolicy.Route` for the Loc-RIB, redistribute and IGP attachments | Extends: BGP chains compile through it at `bgp import`/`bgp export`, with BGP field readers registered from the BGP side (D-7) |
| The attribute modification accumulator | `filterapi.ModAccumulator`, `RegisterAttrModHandler`, `AttrModHandlerFor` (`filterapi/filterapi.go`, `editset.go`); `buildModifiedPayload`, `attrModHandlersWithDefaults` (`reactor/forward_build.go`, `filter_delta_handlers.go`); `core-design.md`, "Route Metadata and Modification Accumulator" | Turns per-attribute ops into the modified UPDATE body through registered per-code handlers | Extends: set fields and named intents become ops through it, with a handler per settable field (D-5); no new apply engine |
| The UPDATE event JSON encoders | `attribute.RegisterJSONFormatter`, `GetJSONFormatter` (`internal/core/bgp/attribute/json.go`); `docs/architecture/api/json-format.md` | Render each attribute's value in the UPDATE event's `attr` map | Reused: D-1 writes the slow-path message's values with them, keyed by field name |
| The `match` condition table | `internal/component/policy/match.go` (`matchCondition`, `matchConditions`, `RouteMatch`, `(*RouteMatch).Matches`, `CompileRouteMatch`, `MatchFields`, `IsSourceConditionKey`), registered as `policy.MatchFilterType` in `policy/register.go` | Compiles a `match` instance's conditions into one `RouteMatch` and judges a `ribpolicy.Route` against them, allocation-free. Not a registry: the slice literal, the per-condition fields of `RouteMatch` and the fixed sequence in `Matches` each name all nine conditions, and nothing outside the file can add one | Extended: the condition set becomes a registry, each condition registering its key, the fields it reads, whether it is a source condition, and its own compiled check; the nine library conditions register through it too, and the five D-6 conditions register from their own packages (phase 4, first task) |
| Config migration transformations | `internal/component/config/migration` (`Transformation`, `MustRegisterTransformation`, `Migrate`, `NeedsMigration`), registered from `internal/plugins/ospf/register_migration.go` and `internal/plugins/redistribute/register_migration.go` | `ze config migrate` runs every registered transformation over the parsed tree | Reused: each D-6 condition package registers the transformation that rewrites its old filter type's chain references and instances into `match` (AC-39) |
| Filter obligations | `registry.Registration.FilterObligations`, `registry.FilterTypesDischarging`, `filterapi.ObligationTransitLeak`, `validateLeakFilterObligations` (`bgp/config/peers.go`) | Lets a filter type declare that it discharges the RFC 9234 transit-leak obligation; `bgp-filter-path-asn` is the only declarer | Extended: a `match` condition can declare the obligation, and the `reject-asn` condition does, so the config rule still names no filter type (R-16) |
| Re-judging a route after its policy input changes | Import side: `bgp-filter-irr`'s `readmitter` (`filter_irr/readmit.go`, `readmit`) sending `request peer <peer> refresh <family>` (RFC 2918); export side: the Adj-RIB-In replay of `policyReplayOwner` and `ribevents.ReplayRoutesCommand` (`reactor/policy_reload.go`) | Import: asks the peer to send again the prefixes a not-yet-resolved list rejected, because `bgp-adj-rib-in` stores only accepted routes and holds nothing to replay. Export: replays stored accepted routes through the export chain | Reused: FR-10's re-judge is the ROUTE-REFRESH route on import (AC-32); no new replay mechanism |
| The plugin lifecycle | `startInternal`/`startExternal` and respawn (`internal/component/plugin/process/process.go`, `manager.go`, `RespawnLimit`); the five-stage startup (`process-protocol.md`, "5-Stage Startup Protocol") | Starts a plugin internal or forked, runs Stage 1 to 5, activates the bridge after Stage 5, respawns a failed process within its limits, and closes the bridge on stop | Reused: the typed slot becomes callable at bridge activation, is swapped with the connection a respawn creates, and answers `ErrBridgeClosed` after stop (FR-11, AC-33) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A received UPDATE (wire bytes, borrowed read buffer) reaching the ingress pass in `notifyMessageReceiver` (`reactor/reactor_notify.go`), after `collapseASPathFamily`.
- A cached UPDATE leaving through the forward rails (`forwardUpdateCore` in `reactor/forward_validation.go`, `reactorForwardRS` in `reactor/forward_rs.go`) to a peer with an export chain.
- A synthetic default-originate route (`peer_initial_sync.go`) and a dry-run body (`PolicyDryRun`).

### Transformation Path (today)
1. `WireUpdate` with lazy `AttributesWire`.
2. Text subject rendered by `AppendUpdateForFilter` (every attribute, every NLRI block).
3. Per external filter: JSON `FilterUpdateInput` over `CallFilterUpdate`; text delta back; `applyFilterDelta` re-parses and re-renders.
4. Original and final text parsed again; `textDeltaToModOps`, the two intent extractors and `sectionNLRIDelta` fill the `ModAccumulator`.
5. `buildModifiedPayload` writes the modified body; the new `WireUpdate` is cached (import) or sent (export).

### Transformation Path (target, both paths)
1. `WireUpdate` with lazy `AttributesWire`, unchanged.
2. The chain is compiled once at config load through the library, against the `bgp import` or `bgp export` attachment, members in order. A member is either a library `match` instance, whose registered conditions (the five D-6 conditions among them) the library evaluates in the daemon over the BGP view with no plugin transport (D-6), or a filter owned by a plugin (D-12): a `modify` instance, the dedicated types `remove-private-as`, `family-filter` and the IRR filter (D-6), or a third-party `<plugin>:<filter>`. A `match` instance is checked at load against the fields its conditions register; a plugin filter against the fields its Stage 1 declaration reads and sets, or against the name pattern it declared (D-10). Which transport a plugin member takes is decided at run time by where its plugin runs, never at load: the compiled chain names the filter, not the transport.
3. Member of a plugin running internal: the reactor calls the plugin's typed `DirectBridge` filter slot synchronously on its own goroutine, as `DeliverStructured` is called, with no encoding. The plugin's evaluator reads fields through the BGP view (D-7). Each field decodes on first read from the wire bytes and is returned as a borrowed slice, valid only during this synchronous run; the reconstructed AS path is built once per run and cached; a malformed attribute fails the route under the member's `on-error`. The evaluator reads its plugin's state from a snapshot that the plugin's own goroutines swap atomically, and never blocks (the fast-path contract, AC-36). A slot that answers `ErrBridgeClosed` (the plugin stopped, crashed or is respawning) is "unavailable" and the member's `on-error` applies (AC-33).
4. Member of a plugin running forked: Ze decodes only the fields the filter declared, keyed by field name in the `attr` value shapes (D-1), adds raw only when raw is one of the declared fields (D-4), and sends them under the negotiated contract version (D-2). The routes queued for that filter at dispatch, up to the batch bound, go in one exchange under one `rpc.FilterUpdateTimeout`; the plugin answers one verdict per route, in request order, each with its own sets, and per-source FIFO order holds across batches (D-3).
5. Each member's sets apply to the view the next member reads, as `RouteChain.Evaluate` does for `ribpolicy.Route`. A set of an undeclared or unsettable field fails that route, counted and logged (D-9).
6. After the chain, set fields with their action (set, add, remove, prepend, increment, decrement) and the named intents Ze resolves (AS-path prepend, remove-private-AS, NLRI subset per section, family strip) become `ModAccumulator` ops through registered per-field handlers (D-5); `buildModifiedPayload` writes the body. Increment and decrement resolve against the view's current value into a set op, because the accumulator has no arithmetic action (`AttrModSet`, `AttrModAdd`, `AttrModRemove`, `AttrModPrepend`, `AttrModSuppress`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to a forked filter plugin | The versioned, batched `filter-update` message, JSON over MuxConn and TLS | No |
| Reactor to an internal filter plugin | The typed `DirectBridge` filter slot: the lazy view in, the verdict and sets out, on the reactor's goroutine, synchronous, no encoding; refused with `ErrBridgeClosed` before Stage 5 completes and after `CloseCallbacks` | No |
| Plugin's own goroutines to its evaluator | An atomically swapped snapshot the evaluator reads and never writes | No |
| Config to reactor | The compiled chain inside `filterapi.Policy` through `PeerSettings` and reload | No |
| Set field to wire | Registered per-field handler writing `ModAccumulator` ops, applied by `buildModifiedPayload` | No |

### Integration Points
- `filterapi.Policy` and its in-process bindings: the compiled chain names each member's plugin and filter; `CallFilterUpdate` picks the typed slot when the plugin's connection holds an activated bridge with the slot set, and the batched IPC message otherwise (D-12). `BindInProcess` and the `filterapi.Register` family fold into this one mechanism (D-13).
- `DirectBridge` (`pkg/plugin/rpc/bridge.go`) and `PluginConn` (`internal/component/plugin/ipc/rpc.go`): one new typed slot beside `SetDeliverStructured`, installed by the SDK after Stage 5; one `engineOp`-style entry, never a second path for the op (`ai/doors/plugin-infra.md`).
- The re-judge route: a plugin whose data resolves late asks the peer to send the routes again with `request peer <peer> refresh <family>` (RFC 2918), as `bgp-filter-irr`'s `readmitter` does at HEAD (AC-32).
- `policy.CheckChain` at the `bgp import`/`bgp export` attachments: judges an external filter's declared reads and sets like a named instance's, and resolves a dynamic name through its declared pattern; `validateChainNames` loses its "validated by that plugin at runtime" skip (D-10).
- The `match` condition registry, which replaces the `matchConditions` literal (D-6): every condition, the nine library ones included, registers its key, read fields, source flag and compiled check, and the five D-6 conditions register from their own packages. `RouteMatch` holds the compiled checks of the registered conditions an instance names, never one field per condition.
- `migration.MustRegisterTransformation`: one transformation per D-6 condition package, rewriting its old filter type into `match` (AC-39).
- `registry.FilterTypesDischarging` and `validateLeakFilterObligations`: the transit-leak obligation is discharged by the `reject-asn` `match` condition's declaration (R-16).
- `buildModifiedPayload`, `attrModHandlersWithDefaults`, `filterapi.RegisterAttrModHandler`: unchanged apply engine.
- `ExtractASPathPrependOps`, `ExtractRemovePrivateASOps`, `subsetNLRI`, `mpSectionDelta`: kept as the wire-side halves of the named intents, driven without text (D-5).
- `attribute.RegisterJSONFormatter` and its registered encoders: the slow-path encoder's value writers (D-1).
- `internal/le/perf`: the throughput harness for both paths (FR-7).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Designed, unproven | every set field and named intent reaches the wire only through `buildModifiedPayload` (D-5); proven per child |
| No unintended coupling (components stay isolated) | Designed, unproven | the reactor names no filter plugin; plugins reach Ze through the contract or a registration; the policy library learns no BGP type (D-7) |
| No duplicated functionality (extends existing, does not recreate) | Designed, unproven | one vocabulary, one chain model, one apply engine; the event encoders reused (D-1); the text machinery deleted, not kept (AC-8) |
| Zero-copy preserved where applicable (refs, not copies) | Designed, unproven | the view returns borrowed slices valid only during the synchronous run (D-7); the slow path encodes only declared fields (FR-2) |
| Works with the plugin in another process | Designed, unproven | each filter has one evaluator on two transports (D-12), and the in-process-only `filterapi.Register` family merges into it (D-13); proven by AC-22, AC-31, AC-37 |
| Registration over hardcoding, outbound | Designed, unproven | the view's field readers and the set-field handlers register per field from the BGP side (D-7); each `match` condition, the five D-6 ones included, registers from its own package, and so does its migration transformation (D-6) |
| Registration over hardcoding, inbound | Designed, unproven | lists to delete or derive: `attrNameToCode`, `validateModifyDelta`, `textDeltaToModOps`' per-attribute arms, `FilterDecl.Attributes` consumers, and the `matchConditions` literal with `RouteMatch`'s per-condition fields and the fixed sequence in `Matches` (AC-38) |
| Works with the plugin in another process, for the five D-6 conditions | Designed, unproven | they are no longer plugins, so there is no forked variant to keep equal: the library evaluates them in the daemon (D-6), and `./le plugin boundary check` has no plugin left to judge for them (AC-40) |

## Requirements

The owner fixed these on 2026-10-31 (`## Disposition`). They are not open.

| ID | Requirement |
|----|-------------|
| FR-1 | Two paths, one vocabulary. A plugin running internal is called in process over a lazy wire view; a plugin running forked, in any language, is called out of process over the process protocol (as revised by FR-8). Both read and set fields named from the library's registered vocabulary, share the load-time read/set check and the chain model, and one chain may mix both. |
| FR-2 | Parse on demand on both paths. In process, the view decodes a field on first read. Out of process, a plugin declares at startup the fields it reads, and Ze decodes and sends only those per route; a filter reading only the prefix is never sent the AS path or communities. The raw UPDATE bytes go only to a plugin that declares it wants them. The set of fields a plugin may read is the set it declared, which is the set the load-time check uses. |
| FR-3 | The out-of-process contract is versioned and documented in the protocol pages (`docs/architecture/api/process-protocol.md`, `docs/plugin-development/`). The Go SDK is a convenience over it, never a second contract. A plugin can tell which contract version it speaks (negotiation, D-2). |
| FR-4 | The text `Update` format is replaced, not kept beside the new one (no layering). Third-party filter plugins written against the text format break, and that is accepted. |
| FR-5 | The in-tree BGP filter plugins migrate to the new contract (and, running internal, to the typed bridge slot) in the same body of work. |
| FR-6 | Batching several routes per exchange is the slow path's performance lever. |
| FR-7 | A throughput harness measures both paths. Thomas runs it on his Mac; no agent runs a perf measurement. |
| FR-8 | (owner, 2026-10-31, the plugin filter pattern) A filter is owned by its plugin, whatever process the plugin runs in. Running internal, the plugin's goroutines own its state and background work, its evaluator is called through the typed `DirectBridge` filter slot with the lazy wire view and no encoding, and its commands still reach it over its normal connection. Running forked, the same filter runs over the bus, batched (D-1 to D-11). The author writes the evaluator once; the SDK installs it as the typed slot when the plugin runs internal and as the `filter-update` handler when it runs forked. |
| FR-9 | (owner, 2026-10-31) The fast-path contract. The evaluator runs on reactor goroutines; it is non-blocking, bounded, and safe under concurrent calls; it never retains the borrowed view's slices after returning; it reads state from a snapshot its own goroutines swap atomically. The SDK enforces what it can: a per-call deadline in debug and test builds, and a race-tested conformance harness every fast-path filter must pass. |
| FR-10 | (owner, 2026-10-31) Unresolved data never blocks the chain. An input the plugin has not resolved yet takes the filter's on-error verdict; once background resolution completes, the plugin asks Ze to re-judge the affected routes through the existing mechanism, which at HEAD is ROUTE-REFRESH (`request peer <peer> refresh <family>`, RFC 2918), because Adj-RIB-In stores only accepted routes. |
| FR-11 | (owner, 2026-10-31) Lifecycle. The evaluator is callable only after the plugin reaches ready (Stage 5). A restart or reload swaps it atomically. If the plugin's goroutines die, the evaluator reports "unavailable" and on-error applies; it never answers from stale state. |

## Open Design Questions

**Resolved.** The owner answered Q-1 to Q-12 on 2026-10-31 by approving every
recommendation. Each answer is a row of `## Key Design Decisions` (D-1 to
D-12, D-N answering Q-N), with the alternatives it beat and the reasoning that
chose it, so the options table is deleted rather than kept beside them. No
design question is open.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every BGP field in the vocabulary can be decoded from `AttributesWire` on demand without copying the wire bytes | `AttributesWire.Get`, `GetRaw`, `ForEach` (`internal/core/bgp/attribute/wire.go`); `WireUpdate` lazy fields under `sync.Once` (`wireu/wire_update.go`) | the view allocates per read and the fast path regresses | a unit test per field reader with `testing.AllocsPerRun` | unvalidated |
| A-2 | Every plain set field maps onto existing `AttrModHandler`s (`genericAttrSetHandler`, `aspathHandler`, the community handlers, `mpReachNextHopHandler`) | `attrModHandlersWithDefaults`; `textDeltaToModOps` already emits ops per attribute | a field needs a new handler, registered per code | a test per settable field asserting the wire bytes | unvalidated |
| A-3 | Prepend and remove-private need reactor-side state (local AS, peer AS, ASN4 width) a plugin cannot know | `ExtractASPathPrependOps` takes `localAS`; `ExtractRemovePrivateASOps` takes `asn4` and `peerAS` | the plugin could compute them and the intents collapse to set fields | reading both signatures and their callers in `filter_ordered.go` | validated by reading at 544f8f930c |
| A-4 | BGP chains already run `policy.CheckChain` for named instances, so the fast path inherits a working load-time check | `validateChainNames` in `internal/component/bgp/config/peers.go`; attachments registered in `bgp/config/register.go` | the check must be wired first | reading at 544f8f930c | validated by reading |
| A-5 | `ribpolicy.Route` cannot carry BGP fields, so the lazy view is a new view kind | BGP fields registered with `RouteView{}` (`internal/component/policy/register.go`); route-policy decision that the Loc-RIB best carries no communities or AS path | BGP fields would be added to a core leaf type the Loc-RIB hot path copies by value | reading at 544f8f930c | validated by reading |
| A-6 | A JSON message with only the declared fields, batched, is no slower per route than today's text subject | FR-6; today a modified UPDATE pays render, two parses and a diff | the slow path regresses for small filters | the throughput harness on Thomas's Mac (FR-7) | unvalidated |
| A-7 | The functional runner and the QEMU guest can fork a Python 3 plugin | no `.py` plugin exists under `test/plugin` or `test/reload` today; Python scripts exist only under `test/exabgp-compat` | the non-Go AC needs another language the runner already has | a runner probe `.ci` forking `python3 -I` | unvalidated |
| A-8 | Non-CIDR families (EVPN, VPN, FlowSpec beyond its destination prefix, BGP-LS, ...) can keep today's rule: per-NLRI decisions need raw | `CIDRFamily` (`filtertext/cidr.go`); "Non-CIDR Families in the Filter Text Protocol" | the decoded NLRI field must cover family-specific encodings | `spec-filter-wire-1-contract`: under D-1 the `prefix` field takes the event's NLRI shapes (json-format.md "NLRI Formats") for CIDR families, and under D-4 a filter that decides on another family declares `raw`; the child confirms each registered non-CIDR family is reachable that way | unvalidated |
| A-9 | The BGP fields the view reads can register their readers from the BGP side while their names stay declared in `internal/component/policy/register.go` | D-7; the BGP fields there carry an empty `RouteView{}` today | the library would learn a BGP type, which D-7 forbids | `spec-filter-wire-2-view` reading `MustRegisterField` and registering one reader from `internal/component/bgp/` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Blast radius: `pkg/plugin/rpc`, the SDK, the reactor, nine plugins, `filtertext`, dry-run, two protocol pages, every filter test | compile and lint fanout when `rpc` changes | child specs land in dependency order; each lands green for its own packages; no compatibility window, because FR-4 forbids one |
| R-2 | A batch delays a route behind slower ones, or reorders per-source delivery | interop and `.ci` order assertions; cache ack FIFO | D-3: per-source FIFO holds across batches, a batch is bounded, and one `rpc.FilterUpdateTimeout` covers the whole batch (AC-12, AC-19) |
| R-3 | The lazy view keeps a pointer into a reused receive buffer past the chain run | race detector; `IsAsyncSafe` false | D-7: the view's values are borrowed slices valid only inside the synchronous run. A route queued for a batch (D-3) leaves that run, so the queue holds a copy or a retained cache entry, never the view (`bgp-engine.md`, "Received bytes are borrowed") |
| R-4 | A field the view decodes from a malformed attribute reads as zero and a filter accepts | RFC 7606 treat-as-withdraw tests | D-7: a malformed attribute fails the route; the chain applies the filter's `on-error` (`ai/rules/principles.md`, AC-28) |
| R-5 | `filter_modify` arithmetic (increment, decrement from the current value or the configured default) changes result when the current value arrives decoded rather than as text | existing `filter_modify` `.ci` and units | it declares the fields it reads; AC-10 runs every existing test unchanged |
| R-6 | Losing the text subject removes operator-readable dry-run output | `show policy test` `.ci` | D-11: a display-only renderer of the view and the set fields keeps the output; it is never a transport (AC-16) |
| R-7 | Route-policy phase 4 and this spec edit the same seam | both open | this spec is that phase's BGP half; route-policy's phase 4 step names it, and the two close together for AC-17 |
| R-8 | `filter_irr` blocks inside its handler on first IRR resolution (`firstResolveWait`, `rpc.FilterUpdateTimeout` minus one second); on a reactor goroutine (fast path) or in a batch (slow path) it would stall every route behind it | `filter-irr-fail.ci`, `clear-irr-cache*.ci` | Closed by the revised D-12 and FR-10: the wait is deleted, not moved. An unresolved set takes the on-error verdict for the announced prefixes, the `readmitter` holds them, and ROUTE-REFRESH re-judges them once the set resolves (AC-32; the IRR design in phase 4) |
| R-9 | Closed by the revised D-12 (owner, 2026-10-31). It asked how a daemon-side IRR evaluator could read state that lives in the forked `bgp-filter-irr` process. Under the revision there is no daemon-side IRR evaluator: IRR keeps its prefix lists, its resolution goroutines and `clear irr cache` in its own goroutines. Running internal, its evaluator is reached through the typed bridge slot and reads the snapshot its goroutines publish; running forked, the same evaluator answers `filter-update` over IPC | `test/plugin/clear-irr-cache-external.ci` (plugin forked) and `clear-irr-cache.ci` (internal) | `clear-irr-cache-external.ci` and the other forked IRR tests stay as slow-path coverage; AC-31 runs the same IRR verdicts on both transports |
| R-10 | D-1 reuses the event encoders, but `communities`, `large-communities` and `extended-communities` are registered by `bgp-filter-community` and `originator-id`, `cluster-list` by `bgp-rr`. A binary without them has no encoder, and a silently absent field would read as "attribute not present" | a build without either plugin; `attribute.GetJSONFormatter` answering nothing | a declared field with no registered encoder refuses the filter at Stage 1, naming the field and the missing plugin, never sends it empty (`ai/rules/principles.md`, AC-29) |
| R-11 | D-8 keeps one name per fact, so `metric` leaves the `bgp import` and `bgp export` carries list. The `match` type's `metric` condition (`policy/match.go`) then refuses at a BGP attachment | `TestCheckChainMetricRefusedAtBGP` | intended: the load error names `med` as the BGP field (AC-24). No `.ci`, reload or interop config at 5d9b21066b uses `match` with `metric` in a BGP chain (grep of `test/`), so AC-10 is unaffected |
| R-12 | The revised D-12 says a filter is owned by its plugin, whatever process it runs in. D-6 folds the types of `filter_aspath`, `filter_aspath_length`, `filter_community_match`, `filter_path_asn` and `filter_prefix` into `match` conditions, and `match` is the library's type (`policy.MatchFilterType`, registered in `internal/component/policy/register.go`, owned by `policy.PluginName`), evaluated by the library over `matchConditions` (`policy/match.go`). A condition evaluated inside a library `match` instance has no plugin transport: with `bgp-filter-prefix` forked, nothing in it would run over the bus | AC-31 and AC-22 for `prefix-list`; `TestBGPMatchConditions` | Closed by the owner on 2026-10-31, option (a), recorded in the revised D-6: a `match` condition is the library's and runs wherever the library runs. The five filters stop being plugins, so there is no plugin transport to follow and no forked variant: they are pure checks on route fields with no state of their own. AC-22 moves to a type that stays a plugin; AC-30 and AC-38 to AC-41 prove the conditions, the registry, the migration, the removal and the refusal. `spec-filter-wire-4-plugins` runs its D-6 half with the rest of phase 4 |
| R-16 | `bgp-filter-path-asn` is more than a check: its plugin wrapper also carries the `show ... reject-asn` commands (`register_command.go`, `command.go`, with shapes, columns and completion), its metrics (`metrics.go`, `ConfigureMetrics`), and the only `FilterObligations` declaration (`filterapi.ObligationTransitLeak`), which `validateLeakFilterObligations` reads through `registry.FilterTypesDischarging` to make a reject-asn check mandatory for a peer with an RFC 9234 role. Deleting the wrapper with nothing in its place would silently drop the commands and turn the obligation rule into one no filter can discharge | `test/` `.ci` files running `show ... reject-asn`; the transit-leak config refusals; `TestFilterTypeMappings` | No feature leaves with the wrapper. The `reject-asn` condition declares the transit-leak obligation through the condition registry, so the config rule still names no filter type. The commands and metrics keep one owner, a plugin (`ai/doors/system.md`: every RPC carries a YANG registration, and whatever has RPCs is a plugin), which `spec-filter-wire-4-plugins` names after reading `docs/architecture/command-ownership.md`; the command paths and output stay unchanged (AC-40) |
| R-17 | The five filter types own config today: each plugin ships the YANG of its named list under `policy` (`<plugin>/yang`), registers `ConfigRoots`, and parses and applies it in `config.go` and `config_tx.go`. `prefix-list` is also read at the Loc-RIB, redistribute and IGP attachments, and `match`'s own `prefix-list` condition already resolves a `prefix-list` instance by name (`prefixListFilterType`, `parseMatchPrefixList`). With no plugin, the list definitions need another owner for their schema, their config delivery and their transaction callbacks | `./le config claims`; the editor's completion of list names; `ze config validate` against boot (`test/plugin/config-validate-agrees-with-boot.ci`) | The list definitions keep their names and grammar under `policy`; each condition package carries its list's YANG and parser, and the library's `policy` plugin (`policy.PluginName`), which already owns the `policy` root, receives and applies them through the condition registry. Each condition's package and gate keep every attachment that accepts the list today, in every build flavour that has it today. `spec-filter-wire-4-plugins` proves it with `./le config claims` and the existing parse and reload `.ci` |
| R-13 | AC-35 asks that a third-party Go filter compiled into a build gets the fast path. At HEAD a compiled-in plugin registers through `registry.Register` (`internal/component/plugin/registry`), which a module outside this repository cannot import, and no `pkg/` registration of a compiled-in plugin exists (`grep '^func Register' pkg/` finds only `RegisterRouteInjector`, `RegisterBatchValidator`, `RegisterDNSResolver`). D-7 also places the view inside `internal/component/bgp/`, and the `StructuredEvent` precedent carries the message as `RawMessage any` that the consumer asserts to an internal type | building an out-of-tree module that links into ze | AC-35 is met by Go code compiled into the tree (a fork's own plugin directory, discovered by `./le repo update`), which is what "compiled in" means at HEAD; reaching ze from a separate module needs a public registration and a public view type, which is a `pkg/` contract change the owner decides. Reported, not designed here |
| R-14 | D-13 merges `filterapi.Register` into the plugin filter, but three of its rows read state only the reactor holds: `loop` reads `LocalAS`, `RouterID`, `ClusterID`, `AllowOwnAS` from `PeerFilterInfo` and is engine code, not a plugin; `bgp-gr`'s LLGR filters read `PeerFilterInfo.LLGRAdvertised`, which the reactor fills from the session's OPEN or the relay rail; `bgp-role` and `bgp-filter-community-relation` order themselves by stage and priority inside one ingress pass, ahead of the chain at `FilterStagePeerChain`, through `meta["src-role"]` | the D-13 table; `TestFilterTypeMappings`; the ingress stage order in `core-design.md` | D-13 names each row's outcome. A row needing reactor-only state receives it as declared fields of the view (as `PeerFilterInfo` already passes it) or stays engine code; the child proves each against its own `.ci` with the plugin forked |
| R-15 | At HEAD every bridge callback, `filter-update` included, queues on `callbackCh` and runs on the plugin's one `bridgeEventLoop` goroutine, so a handler is serialised for free. The typed slot runs on reactor goroutines, concurrently, as `DeliverStructured` does. An evaluator migrated without a snapshot races its own state | the race detector; the conformance harness (AC-36) | FR-9: each migrated evaluator reads only an atomically swapped snapshot; the harness runs every fast-path evaluator concurrently under `-race` and refuses one that writes shared state or blocks |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | BGP policy decisions: routes accepted that a filter should reject (leaks), or rejected that it should accept; malformed modified UPDATEs; every third-party text-format filter plugin (expected, FR-4) |
| How is it reverted? | Per child spec commit; the text machinery's deletion is the last child, so earlier children revert alone |
| Who else touches this path? | Route-policy phase 4 (same seam); any spec editing `reactor/filter_*.go`, `forward_build.go`, `filterapi`, the filter plugins, `internal/component/policy/match.go`, or `internal/component/config/migration` |
| Who outside the repository sees the D-6 change? | Every operator whose config names `as-path-list`, `as-path-length`, `community-match`, `reject-asn` or `prefix-list` in a chain: the config stops loading until `ze config migrate` rewrites it (AC-39, AC-41); every operator or script that names the five plugins (`bgp-filter-aspath`, `bgp-filter-aspath-length`, `bgp-filter-community-match`, `bgp-filter-path-asn`, `bgp-filter-prefix`) in `plugin { ... }` or `attach process` |

## Wiring Test (MANDATORY -- NOT deferrable)

Test names are planned; their files are created by the child that owns them.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `filter { import { filters [ <instance of a plugin running internal> ]; final accept; } }` on a peer, UPDATE received | → | typed `DirectBridge` filter slot, the plugin's evaluator over the lazy view, set fields to `ModAccumulator`, `buildModifiedPayload` | `test/plugin/filter-wire-inprocess-import.ci` |
| `filter { export { filters [ <plugin>:<filter> ]; final accept; } }` with an external plugin declaring one field | → | slow-path encoder sends that field only, reply sets applied | `test/plugin/filter-wire-external-declared.ci` |
| The same with a Python 3 plugin speaking the documented contract | → | slow path end to end, no Go SDK | `test/plugin/filter-wire-python-plugin.ci` |
| `show policy test` against a chain mixing both paths | → | dry-run over the view, trace per member | `test/plugin/filter-wire-dryrun.ci` |
| `filter { import { filters [ bgp-filter-irr:65001 ]; final accept; } }`, plugin internal, then forked | → | typed bridge slot, then batched message, same evaluator | `test/plugin/filter-wire-irr-two-transports.ci` |
| An IRR-filtered UPDATE before the first resolution | → | on-error verdict, `readmitter`, `request peer <peer> refresh <family>` | `test/plugin/filter-wire-irr-unresolved.ci` |
| An internal filter plugin stopped mid-flow | → | slot answers `ErrBridgeClosed`, on-error applies | `test/plugin/filter-wire-plugin-unavailable.ci` |
| `filter { import { filters [ <match instance with as-path, as-path-length, community, reject-asn and prefix-list conditions> ]; final accept; } }` on a peer, UPDATE received | → | condition registry, each D-6 condition's compiled check over the BGP view, in the daemon | `test/plugin/filter-wire-match-conditions.ci` |
| `ze config migrate` on a config whose chains name the five old filter types | → | the registered transformations, rewritten config, then the daemon loading it | `test/parse/filter-wire-migrate-simple-filters.ci` |
| A config whose chain names `prefix-list:NAME` loaded without migration | → | config load refusal naming the old reference, its `match` replacement and `ze config migrate` | `test/parse/filter-wire-old-filter-refused.ci` |

## Acceptance Criteria

The last column names the owner decision (`## Key Design Decisions`, D-N) or
requirement each row proves.

| AC ID | Input / Condition | Expected Behavior | Decided by |
|-------|-------------------|-------------------|------------|
| AC-1 | An in-process filter instance that reads only `prefix` runs on an UPDATE carrying AS_PATH, COMMUNITY and LARGE_COMMUNITY | a decode counter on the BGP view records the NLRI decode and zero attribute decodes; no allocation for the evaluation (`testing.AllocsPerRun` = 0) | D-7 |
| AC-2 | An external test plugin declares it reads `prefix` only | the message Ze sends carries the `prefix` key, no `as-path`, no community key and no `raw` key (asserted on the captured request) | D-1, D-2, D-4 |
| AC-3 | Two external filters in one chain, one declaring the `raw` field, one not | only the first receives the raw UPDATE | D-4 |
| AC-4 | A filter, in process and external in turn, sets `local-preference` 200 on an import route | the cached UPDATE carries LOCAL_PREF 200, and the two paths produce identical UPDATE bytes | D-1, D-5 |
| AC-5 | A chain at `bgp export` names a filter that sets a field the attachment does not allow, or reads one it does not carry | config load fails naming the filter, the field and the attachment; the same for an external filter's Stage 1 declared reads and sets | D-10 |
| AC-6 | An external reply sets a field the filter did not declare | that route alone is treated as a failed filter under its `on-error` (fail-closed by default); the other routes of the batch keep their own verdicts; the refusal is logged naming the filter and the field, and counted | D-9 |
| AC-7 | An external filter answers the named intents AS-path prepend 2, remove-private-AS, and an NLRI subset dropping one IPv4 and one IPv6 prefix | the receiving peer sees the path prepended twice with the local AS, private ASNs stripped segment-preserving with the peer AS and ASN4 width Ze holds, and only the kept prefixes in NLRI and MP_REACH_NLRI | D-5 |
| AC-8 | The text format is removed | `FilterUpdateInput.Update`, `FilterUpdateOutput.Update`, `AppendUpdateForFilter`, `AppendAttrsForFilter`, `parseFilterAttrs`, `applyFilterDelta`, `formatFilterAttrs`, `textDeltaToModOps`, `validateModifyDelta`, the text-driven half of `sectionNLRIDelta` and the `internal/component/bgp/filtertext` package no longer exist; the four text-protocol sections of `process-protocol.md` and the handler section of `docs/plugin-development/protocol.md` describe the new contract | FR-4 |
| AC-9 | A plugin whose `declare-registration` carries no protocol version (the text era) | Stage 1 refuses it with one error naming the plugin, saying it declared no protocol version, naming the version it must speak, and pointing at `docs/plugin-development/protocol.md`; it is the only refusal for that plugin, and the startup tier stops as for any plugin failure (`plugin-infra.md` trap) | D-2 |
| AC-10 | Every existing BGP policy `.ci` (`find test -name '*.ci'` naming a `bgp-filter-*` plugin) and every interop scenario whose config names a filter chain | pass with unchanged expectations; this is route-policy AC-17. A config that names one of the five D-6 filter types is first rewritten by `ze config migrate` (AC-39), and only its config changes: every expected verdict, UPDATE byte and output line stays. Each such rewrite is judged by an independent `ze-test-judge` agent before it lands (`ai/META-DECISIONS.md`, "Test changes") | all |
| AC-11 | A Python 3 test plugin, written against the protocol page only, implements a filter that rejects one prefix and sets `med` on the rest | the receiving peer gets only the accepted routes, each with the MED set; this is the slow path's end-to-end proof with no Go SDK in the plugin (in-tree plugins run forked prove the slow path through the SDK, AC-22 and AC-31) | D-1, D-2, D-12, A-7 |
| AC-12 | Ten routes from one source queued for one external filter, with the batch bound above ten | they go in one exchange; the reply carries ten verdicts in request order, each with its own sets; per-source order is unchanged on the wire. With the bound below ten, they split into consecutive batches and per-source order still holds across them | D-3 |
| AC-13 | One chain with an in-process member that sets `local-preference` and an external member that reads it | the external member is sent the value the first member set | D-1, D-12 |
| AC-14 | Each of the four in-tree filter plugins that stay plugins (`filter_modify`, `filter_remove_private_as`, `filter_family`, `filter_irr`), run internal | its filter is called through the typed bridge slot (as a `modify` instance or a dedicated type); no in-tree code calls `OnFilterUpdate` with a text handler; every one of their existing tests passes. The other five are `match` conditions with no plugin (AC-30, AC-40) | D-6, D-12 |
| AC-15 | An external filter's batch times out, errors, or a route's verdict answers teardown, on import and on export | `on-error` decides each route's outcome as today; teardown closes the session on import only | D-3 |
| AC-16 | `show policy test` on a chain of both kinds | reports the same decision and final as the live path, and a per-member trace drawn from the view and the set fields; every existing `test/plugin/policy-test-*.ci` passes with unchanged expectations | D-11 |
| AC-17 | The throughput harness | exists under `./le perf` for the in-process path and for the external path at several batch sizes, compiles, and prints its run command for Thomas; no agent records a perf number | FR-7 |
| AC-18 | A plugin declares a protocol version above the one Ze speaks | the Stage 1 answer names the version Ze will speak; the plugin then speaks that version and its filters run. A plugin declaring exactly Ze's version gets the same answer | D-2 |
| AC-19 | An external filter's batch fails as a whole: it times out under its one `rpc.FilterUpdateTimeout`, or the reply carries more or fewer verdicts than routes | the filter's `on-error` is applied to each route of that batch, one outcome per route; no verdict of a malformed reply is applied; the next batch is unaffected | D-3 |
| AC-20 | An external reply sets a declared field the attachment does not let a filter set, names a field the vocabulary does not register, or carries a value of the wrong shape | the route fails under `on-error`, logged and counted as in AC-6; nothing of that route's sets is applied | D-9 |
| AC-21 | A plugin declares a filter name pattern covering `<plugin>:<asn>` names, and a chain names one matching reference and one that matches no declared name or pattern | the matching reference is checked at load against the pattern's declared fields like a named instance; the other fails config load naming the reference and the plugin's declared names and patterns | D-10 |
| AC-22 | A peer chain names an in-tree type that stays a plugin (`family-filter` of `bgp-filter-family`), first with the plugin internal, then with it configured to run forked | internal: the route is judged through the typed bridge slot, no `filter-update` JSON is marshalled and no callback is queued on `callbackCh` (counted); forked: the route is judged by the forked process over the batched message (counted on the captured transport). The verdict and UPDATE bytes are equal in both runs. Revised with D-12: the earlier text required a forked plugin's type to be judged in the daemon | D-12 (revised) |
| AC-23 | Filters read `atomic-aggregate`, `aggregator`, `originator-id` and `cluster-list`, in process and external | each field is registered and carried by the `bgp import` and `bgp export` attachments; in process, reading one decodes only that attribute (decode counter); an external filter declaring one is sent it in the `attr` value shape and is sent none of the others | D-8 |
| AC-24 | A BGP chain names a `match` instance with a `metric` condition | config load fails naming the instance, the field `metric`, the attachment, and `med` as the BGP field for that fact; the same instance still loads at a Loc-RIB, redistribute or IGP attachment | D-8 |
| AC-25 | Filters answer `community` add and remove, `med` set, increment and decrement, and `local-preference` set, increment and decrement, in process and external | each produces the expected attribute bytes on the receiving peer, increment and decrement computed from the route's current value (or the configured default where `filter_modify` uses one today), and the two paths produce identical UPDATE bytes | D-5 |
| AC-26 | A filter answers the family-strip intent for one MP family of an UPDATE carrying two | the receiving peer sees the UPDATE without that family's MP_REACH_NLRI and MP_UNREACH_NLRI, the other family intact, and a body equal to what `filter_family`'s raw rewrite produces today | D-5 |
| AC-27 | An external filter declaring the `raw` field answers a whole-body raw rewrite | the rewrite passes `decodeFilterRawOverride`, stops later filters, still receives the final, and a malformed body fails the route under `on-error` | D-4 |
| AC-28 | An in-process member reads a field whose attribute is malformed on the wire | the route fails under the member's `on-error`; no zero or empty value reaches the evaluator | D-7 |
| AC-29 | An external filter declares `community`, `as-path` and `local-preference` | the message keys them by those field names (not the event keys `communities`), with the `attr` value shapes (a string list, a number list, a number); a binary that links no encoder for a declared field refuses the filter at Stage 1 naming the field | D-1 |
| AC-30 | Each of the five simple filters as a `match` condition: one `match` instance per condition (the AS-path list, the AS-path length, the community match, the reject-asn list, the prefix-list), in BGP import and export chains, and `prefix-list` also at the Loc-RIB, redistribute and IGP attachments that accept it today, fed the inputs of today's `.ci` and interop scenarios for `bgp-filter-aspath`, `bgp-filter-aspath-length`, `bgp-filter-community-match`, `bgp-filter-path-asn` and `bgp-filter-prefix` | every accept, reject and NLRI-subset verdict and every UPDATE byte equals what today's `.ci` and interop scenarios assert (`bgp-path-asn-leak-frr` among them); each condition is evaluated by the library in the daemon over the BGP view, with no `filter-update` call and no plugin process (counted); `reject-asn` still reads `PeerAS` and the direction it reads today | D-6 (revised) |
| AC-31 | The same `bgp-filter-irr` plugin and config, one run with the plugin internal and one with `plugin { external bgp-filter-irr { run "ze plugin bgp-filter-irr"; } }`, fed the same UPDATEs (in list, out of list, mixed, withdrawals, an MP family) | identical verdicts and identical UPDATE bytes on the receiving peer in both runs; the internal run takes the typed slot, the forked run the batched message (both counted) | D-12, FR-8 |
| AC-32 | An IRR-filtered UPDATE arrives before its ASN's first resolution completes, with the IRR mock answering after a delay longer than `rpc.FilterUpdateTimeout` | the chain answers within the per-call bound with no wait in the evaluator: the announced prefixes take the filter's on-error verdict (reject by default) and the withdrawals in the same UPDATE still apply; `show bgp irr` reports the ASN `pending`; once the mock answers, the `readmitter` sends `request peer <peer> refresh <family>` and the re-sent prefixes in the list are accepted | FR-10, D-12 |
| AC-33 | A filter plugin running internal stops (its process ends, or it is respawned) while routes arrive | while its slot answers `ErrBridgeClosed` each route takes the member's on-error verdict, logged naming the plugin and "unavailable"; no verdict is taken from the stopped plugin's state; after respawn reaches Stage 5 the new slot answers, and no route is judged by a mix of the two | FR-11 |
| AC-34 | One internal filter plugin's evaluator is swapped by a config reload that changes its data while UPDATEs flow | every route is judged wholly by the old snapshot or wholly by the new one (race-tested) | FR-9, FR-11 |
| AC-35 | A Go filter plugin that is not one of the nine, written against the SDK only and compiled into the tree | it gets the typed slot exactly as an in-tree filter does, and passes the conformance harness; the same plugin built as its own binary and forked gets the batched message (R-13 bounds what "compiled in" covers) | FR-8 |
| AC-36 | The conformance harness runs a fast-path evaluator that blocks (sleeps past the deadline), one that keeps a slice of the view after returning, and one that writes shared state without the snapshot | each is refused, naming the evaluator and the broken clause of the contract; a conforming evaluator passes under `-race` with concurrent calls; in debug and test builds a call past the per-call deadline fails the test | FR-9 |
| AC-37 | Each `filterapi.Register` row of `## Current Behavior` | is a plugin filter on the bridge path whose existing `.ci` passes with its plugin internal and with it forked, or is named in D-13 with the reactor state it needs and the test proving its behaviour unchanged; `filterapi.Register` and `BindInProcess` no longer exist as a second family | D-13 |
| AC-38 | A `match` condition registered from a package outside `internal/component/policy/match.go`, in a test | a `match` instance naming it compiles, `CheckChain` judges it by the fields it registered, `MatchFields` and `IsSourceConditionKey` answer for it, and it is evaluated with no edit to `match.go`; `matchConditions`, `RouteMatch`'s per-condition fields and the fixed sequence in `Matches` no longer exist; the nine library conditions pass their existing tests through the registry; a second registration of one key panics at `init()`; evaluation still allocates nothing (`testing.AllocsPerRun` = 0) | D-6 (revised), `ai/rules/principles.md` |
| AC-39 | `ze config migrate` on configs that name `as-path-list:NAME`, `as-path-length:NAME`, `community-match:NAME`, `reject-asn:NAME` and `prefix-list:NAME` (and their unqualified forms) as chain members, at bgp, group and peer level and, for `prefix-list`, at the Loc-RIB, redistribute and IGP attachments | each reference becomes a `match` instance whose one condition names the same list, in the same chain position; the list definitions keep their names and contents; chain order, `final` and inheritance are unchanged; a second run changes nothing; `ze config migrate` dry-run lists each rewrite; the migrated config loads and gives the verdicts AC-30 asserts | D-6 (revised) |
| AC-40 | The build after phase 4 | the packages `internal/component/bgp/plugins/filter_aspath`, `filter_aspath_length`, `filter_community_match`, `filter_path_asn` and `filter_prefix` and their `registry.Register` calls no longer exist; no plugin named `bgp-filter-aspath`, `bgp-filter-aspath-length`, `bgp-filter-community-match`, `bgp-filter-path-asn` or `bgp-filter-prefix` is registered (`ze plugin` list, the generated `plugin/all`, the catalog); no wrapper, alias or fallback keeps the old type running (`ai/rules/no-layering.md`); the `show ... reject-asn` commands answer with unchanged paths and output (`test/plugin/path-asn-show.ci`, `test/ui/completion-words-reject-asn-show.ci`), the reject-asn metrics keep their names, and a peer with an RFC 9234 role is still refused without a reject-asn check, discharged by the `reject-asn` condition (R-16) | D-6 (revised) |
| AC-41 | A config that names one of the five old filter types as a chain member, or one of the five removed plugin names, loaded without migration (daemon start, reload, `ze config validate`, the editor commit) | config load fails with one error per reference, naming the reference and where it sits, saying the filter type is now a `match` condition and which one, and giving the exact command `ze config migrate -o <new-file> <file>`, as the relocated `bgp { policy { ... } }` refusal does today (`relocatedHint`, `internal/component/config/relocated.go`); nothing loads partly; the same text on every interface (`ai/rules/user-facing-errors.md`) | D-6 (revised) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a prefix-list import chain on a peer | config → library compile → `match` instance with the `prefix-list` condition → library evaluation over the BGP view → cache | `test/plugin/filter-wire-match-conditions.ci` |
| 4 | Upgrades with a config whose chains name `prefix-list:NAME` and `reject-asn:NAME` | daemon start → refusal naming `ze config migrate` → `ze config migrate -o <new-file> <file>` → daemon loads the migrated config with the same verdicts | `test/parse/filter-wire-old-filter-refused.ci`, `test/parse/filter-wire-migrate-simple-filters.ci` |
| 2 | Writes a filter plugin in Python from the protocol page | Stage 1 declaration → slow path → reply sets → wire | `test/plugin/filter-wire-python-plugin.ci` |
| 3 | Runs `show policy test` | dry-run over the view | `test/plugin/filter-wire-dryrun.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWireViewDecodesOnDemand` | the BGP view package (`## Files to Create`) | AC-1 decode counter and zero allocations; AC-23 one decode per new field | |
| `TestWireViewMalformedAttributeErrors` | same | AC-28, R-4 | |
| `TestWireViewASPathBuiltOnce` | same | D-7: the reconstructed AS path is built once per run however many members read it | |
| `TestFilterMessageFieldShapes` | `internal/component/bgp/reactor/` | AC-29: keys are field names, values the `attr` shapes; a declared field with no encoder refused | |
| `TestDeclareRegistrationVersionAnswer` | `internal/component/plugin/server/` | AC-18 | |
| `TestFilterBatchFailureAppliesOnError` | `internal/component/bgp/reactor/` | AC-19 | |
| `TestUnsettableOrUnknownSetFailsRoute` | `internal/component/bgp/reactor/` | AC-20 | |
| `TestCheckChainExternalNamePattern` | `internal/component/bgp/config/` | AC-21 | |
| `TestInTreeTypeInProcessWhenForked` | `internal/component/bgp/reactor/` | AC-22 | |
| `TestCheckChainMetricRefusedAtBGP` | `internal/component/bgp/config/` | AC-24 | |
| `TestSetActionsToModOps` | `internal/component/bgp/reactor/` | AC-25, one case per field and action | |
| `TestFamilyStripIntent` | `internal/component/bgp/reactor/` | AC-26, compared against today's raw rewrite captured before deletion | |
| `TestRawRewriteTerminal` | `internal/component/bgp/reactor/` | AC-27 | |
| `TestBGPMatchConditions` | each D-6 condition package (one test per condition) | AC-30: every case of the old plugin's `match_test.go`, `aspath_length_test.go`, `filter_path_asn_test.go`, `rfc8955_flowspec_test.go` and `mp_blocks_test.go` gives the same verdict through the condition over the BGP view | |
| `TestMatchConditionRegistry` | `internal/component/policy/` | AC-38: a test-registered condition compiles, is field-checked and evaluated with no edit to `match.go`; duplicate key panics; zero allocations; registry snapshot restored in `t.Cleanup` | |
| `TestMigrateSimpleFilters` | each D-6 condition package, beside its registered transformation | AC-39: every reference form and attachment, chain position kept, idempotent second run, dry-run listing | |
| `TestOldFilterTypeRefused` | `internal/component/policy/` or `internal/component/bgp/config/`, where the chain is resolved | AC-41: the refusal text names the reference, the replacing condition and `ze config migrate -o <new-file> <file>` | |
| `TestTransitLeakDischargedByCondition` | `internal/component/bgp/config/` | AC-40, R-16: a peer with an RFC 9234 role and a `match` instance with the `reject-asn` condition loads; without one it is refused as today | |
| `TestFilterMessageDeclaredFieldsOnly` | `internal/component/bgp/reactor/` | AC-2, AC-3 | |
| `TestSetFieldToModOps` | `internal/component/bgp/reactor/` | AC-4, one case per settable field | |
| `TestUndeclaredSetFailsRoute` | `internal/component/bgp/reactor/` | AC-6 | |
| `TestTypedIntentsMatchTextEra` | `internal/component/bgp/reactor/` | AC-7, compared against today's outputs captured before deletion | |
| `TestCheckChainExternalDeclaration` | `internal/component/bgp/config/` | AC-5 | |
| `TestDeclareRegistrationRefusesTextContract` | `internal/component/plugin/server/` | AC-9 | |
| `TestFilterBatchOrderAndVerdicts` | `internal/component/bgp/reactor/` | AC-12 | |
| `TestMixedChainSetsVisible` | `internal/component/bgp/reactor/` | AC-13 | |
| `TestBridgeFilterSlotNoEncoding` | `pkg/plugin/rpc/` | AC-22: the typed slot is called on the caller's goroutine, marshals nothing, queues nothing on `callbackCh` | |
| `TestBridgeFilterSlotClosed` | `pkg/plugin/rpc/` | AC-33: before activation and after `CloseCallbacks` the slot answers `ErrBridgeClosed` | |
| `TestFilterUnavailableAppliesOnError` | `internal/component/bgp/reactor/` | AC-33: `ErrBridgeClosed` from the slot takes the member's on-error | |
| `TestSDKInstallsEvaluatorPerTransport` | `pkg/plugin/sdk/` | FR-8: one evaluator registration becomes the typed slot under `IsInternal()` and the `filter-update` handler otherwise | |
| `TestFastPathConformance` | the SDK's conformance harness package, named by `spec-filter-wire-3-chain` | AC-36: blocking, view-retaining and unsnapshotted evaluators refused; deadline in debug and test builds | |
| `TestIRRUnresolvedTakesOnError` | `internal/component/bgp/plugins/filter_irr/` | AC-32: no wait, announced prefixes take on-error, withdrawals kept, prefixes held | |
| `TestIRRSnapshotSwap` | `internal/component/bgp/plugins/filter_irr/` | AC-34: concurrent evaluation across a snapshot swap under `-race` | |
| `TestIRRPassesConformance` | `internal/component/bgp/plugins/filter_irr/` | AC-36 for IRR | |
| per-plugin evaluator and handler tests | `filter_modify`, `filter_remove_private_as`, `filter_family`, `filter_irr` | AC-14 | |

The five removed plugins' unit tests move with their checks into the condition
packages. Moving or rewriting a test is test maintenance, reported apart from
the product change, and each move is judged by an independent `ze-test-judge`
agent before it lands; the moved test proves at least what the old one did.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| batch size (D-3): routes per exchange | 1 to the bound, a constant beside `rpc.FilterUpdateTimeout` in `pkg/plugin/rpc` and stated on the protocol page; not operator-configurable. Its value is set and justified in `spec-filter-wire-3-chain` | the bound (one exchange) | 0 routes: no exchange is sent | bound + 1: two consecutive exchanges, order kept |
| AS-path prepend count (D-5) | 1..32, today's `filter_modify` YANG range (`ze-filter-modify.yang`, `as-path-prepend`) | 32 | 0: the intent is refused and the route fails under `on-error` | 33: refused the same way |
| contract version (D-2) | the version Ze speaks, 1 for the first versioned contract | 1 | absent or 0: the text era, refused at Stage 1 (AC-9) | 2 or more: Ze answers 1 (AC-18) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-wire-inprocess-import` | `test/plugin/` | AC-1, AC-4 through the daemon | |
| `filter-wire-external-declared` | `test/plugin/` | AC-2, AC-3 | |
| `filter-wire-python-plugin` | `test/plugin/` | AC-11 | |
| `filter-wire-text-contract-refused` | `test/plugin/` | AC-9 | |
| `filter-wire-dryrun` | `test/plugin/` | AC-16 | |
| `filter-wire-version-negotiated` | `test/plugin/` | AC-18 | |
| `filter-wire-batch` | `test/plugin/` | AC-12, AC-19 through the daemon | |
| `filter-wire-undeclared-set` | `test/plugin/` | AC-6, AC-20 | |
| `filter-wire-name-pattern` | `test/parse/` | AC-21 | |
| `filter-wire-intree-forked` | `test/plugin/` | AC-22 | |
| `filter-wire-new-fields` | `test/plugin/` | AC-23 | |
| `filter-wire-metric-at-bgp-refused` | `test/parse/` | AC-24 | |
| `filter-wire-set-actions` | `test/plugin/` | AC-25 | |
| `filter-wire-intents` | `test/plugin/` | AC-7, AC-26 | |
| `filter-wire-irr-two-transports` | `test/plugin/` | AC-31 | |
| `filter-wire-irr-unresolved` | `test/plugin/` | AC-32 | |
| `filter-wire-plugin-unavailable` | `test/plugin/` | AC-33 | |
| `filter-wire-thirdparty-compiled-in` | `test/plugin/` | AC-35, a test fixture filter plugin compiled into the tree, run internal then forked | |
| `filter-wire-inprocess-family-forked` | `test/plugin/` | AC-37, each merged `filterapi` filter with its plugin forked | |
| `filter-wire-match-conditions` | `test/plugin/` | AC-30: the five conditions in one `match` instance and singly, import and export, no plugin process started | |
| `filter-wire-migrate-simple-filters` | `test/parse/` | AC-39: `ze config migrate` output, then the migrated config loads | |
| `filter-wire-old-filter-refused` | `test/parse/` | AC-41: an unmigrated config refused with the migrate command | |
| `path-asn-show`, `completion-words-reject-asn-show` | `test/plugin/`, `test/ui/` | AC-40: the `show ... reject-asn` commands unchanged after the wrapper goes | exist |
| `clear-irr-cache-external`, `filter-irr-fail`, `filter-irr-readmit` and the other forked IRR tests | `test/plugin/` | kept unchanged as slow-path coverage (R-9) | exist |
| every existing BGP policy `.ci` | `test/plugin/`, `test/reload/`, `test/parse/` | AC-10, unchanged | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-policy-final-frr`, `bgp-policy-import-export-frr`, `bgp-policy-modify-mp-frr` | `test/interop/scenarios/` | FRR | AC-10: chain decisions and modified attributes, MP included, accepted by a real peer | exist; run owed after migration |
| `bgp-remove-private-as-frr`, `bgp-remove-private-as-as4path-frr`, `as-path-prepend-two-octet-peer` | `test/interop/scenarios/` | FRR | AC-7 intents on the wire | exist; run owed after migration |
| `bgp-path-asn-leak-frr`, `bgp-med-increment-gobgp`, `bgp-med-remove-configured-gobgp`, `bgp-attribute-default-localpref-gobgp` | `test/interop/scenarios/` | FRR, GoBGP | AC-10 for `filter_path_asn` and `filter_modify` | exist; run owed after migration |
| the remaining scenarios whose config names a filter chain (`grep -rlE "filters \[" test/interop/scenarios`) | `test/interop/scenarios/` | as each names | AC-10 | exist; run owed after migration |

Each changed behavior owes a recorded red: revert, rebuild the image, see the
scenario fail, restore (`ai/rules/interop-and-goal-validation.md`).

## Files to Modify

- `pkg/plugin/rpc/types.go` - `DeclareRegistrationInput` gains the plugin-level protocol version (D-2); `FilterDecl` declares reads, sets (raw among them) and an optional name pattern in vocabulary names (D-4, D-10); `FilterUpdateInput` and `FilterUpdateOutput` become a batch of routes keyed by field name, one verdict per route with its sets and intents (D-1, D-3, D-5); the batch bound beside `FilterUpdateTimeout`
- `pkg/plugin/rpc/bridge.go` - the typed `filter-update` slot beside `SetDeliverStructured`/`DeliverStructured`, with the same `beginDelivery`/`CloseCallbacks` fencing (D-12)
- `internal/component/plugin/ipc/rpc.go` - `SendFilterUpdate` takes the typed slot when the bridge holds it, the batched IPC message otherwise; one route per op, never a second path (D-12)
- `pkg/plugin/sdk/sdk_callbacks.go`, `pkg/plugin/sdk/sdk_types.go`, `pkg/plugin/sdk/sdk.go` - one evaluator registration, installed as the typed slot after Stage 5 when `IsInternal()`, and as the batched `filter-update` handler when forked; the debug and test build per-call deadline (FR-8, FR-9)
- `internal/component/plugin/server/server.go`, `internal/component/plugin/server/startup.go` - `CallFilterUpdate` (batch), `registrationFromRPC` filters, `FilterInfo`, the Stage 1 version answer and the text-era refusal (D-2)
- `internal/component/bgp/reactor/filter_chain.go`, `filter_ordered.go`, `filter_delta.go`, `filter_nlri_delta.go`, `filter_format.go`, `policy_dryrun.go`, `peer_initial_sync.go` - chain run over the view and the new message; text deleted
- `internal/component/bgp/reactor/forward_build.go`, `filter_delta_handlers.go` - only if a set field needs a handler (A-2)
- `internal/component/bgp/filterapi/policy.go`, `filterapi.go` - the compiled chain inside `Policy` names each member's plugin and filter, not its transport (D-12); `Register`, `IngressFilters`, `EgressFilters` and `BindInProcess` merge into the plugin filter (D-13)
- `internal/component/bgp/plugins/filter_community/`, `role/`, `gr/`, `rs/`, `internal/component/bgp/reactor/filter/` - the `filterapi.Register` rows move as D-13 names (R-14)
- `internal/component/bgp/config/register.go`, `peers.go` - the attachments carry `atomic-aggregate`, `aggregator`, `originator-id` and `cluster-list` and drop `metric` (D-8); `validateChainNames` checks external declarations and name patterns (D-10)
- `internal/component/policy/fields.go`, `register.go` - a reader seam the BGP side fills for its fields, which keep their names here (D-7, A-9)
- `internal/component/policy/match.go` - the condition set becomes a registry: `matchConditions`, `RouteMatch`'s per-condition fields and the fixed sequence in `Matches` are deleted, and the nine library conditions register through the registry like any other (D-6, AC-38)
- `internal/component/policy/` (the `policy` plugin, `policy.PluginName`) - receives and applies the list definitions the condition packages register under the `policy` root (R-17); refuses a chain member naming an old filter type with the migrate command (AC-41)
- `internal/component/bgp/plugins/filter_aspath/`, `filter_aspath_length/`, `filter_community_match/`, `filter_path_asn/`, `filter_prefix/` - deleted whole, `register.go` and its `registry.Register` call included, after their checks, list YANG, config parsers and tests have moved to the condition packages (D-6, AC-40, `ai/rules/no-layering.md`). `filter_path_asn`'s commands and metrics move to the plugin `spec-filter-wire-4-plugins` names (R-16)
- `internal/component/bgp/config/peers.go` - `validateLeakFilterObligations` reads the obligation the `reject-asn` condition declares (R-16)
- `internal/component/plugin/registry/registry.go` - `FilterObligations` and `FilterTypesDischarging` also answer for a `match` condition's declaration, or move to the condition registry (R-16)
- `internal/component/plugin/all/` - regenerated by `./le repo update` once the five packages are gone
- every `test/` `.ci` and interop config naming the five old filter types - rewritten by `ze config migrate`, expectations unchanged, each judged by `ze-test-judge` (AC-10)
- `internal/component/bgp/plugins/filter_modify/` - its `modify` type evaluates in process over the view, with set actions and the prepend intent (D-5, D-6)
- `internal/component/bgp/plugins/filter_remove_private_as/`, `filter_family/` - dedicated types with one evaluator for both transports (D-6, D-12)
- `internal/component/bgp/plugins/filter_irr/filter_irr.go`, `match.go`, `readmit.go`, `register.go` - the IRR design in phase 4: the snapshot, the evaluator over the view, `firstResolveWait` and the wait in `handleFilterUpdate` deleted, the name pattern (D-10)
- `internal/component/bgp/plugins/cmd/policy/handler.go` - dry-run output through the display-only renderer (D-11)
- `internal/test/fixture/plugin_fixture_filter_subject.go`, `plugin_fixture_filter_origin.go`, `plugin_fixture_12_filters.go`, `plugin_fixture_09_bgp.go` - fixtures that implement `OnFilterUpdate` today
- `internal/le/interoplab/bgp/helper.go`, `helper_srv6_identity.go` - interop helpers that implement `OnFilterUpdate` today
- `internal/le/perf/` - the throughput harness (FR-7)
- `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`, `docs/plugin-development/README.md`, `docs/architecture/core-design.md` ("Ingress Filter Pipeline", "Policy Filter Chain"), `docs/guide/bgp-policy.md`, `docs/architecture/api/architecture.md`, `ai/rules/plugins.md`

## Files to Create

- Child specs under `plan/post-release/engineering/`, written at the design gate, one per implementation phase below, with stems `spec-filter-wire-1-contract`, `spec-filter-wire-2-view`, `spec-filter-wire-3-chain`, `spec-filter-wire-4-plugins`, `spec-filter-wire-5-delete` (the split planned in `## Implementation Steps`; `/ze-implement` writes each child before its phase starts)
- The BGP view's package under `internal/component/bgp/` (D-7): inside the BGP engine and importable by the BGP filter plugins, as `filterapi` is, because a plugin never imports `reactor`. `spec-filter-wire-2-view` names it
- The display-only renderer of the view and the set fields, in the reactor's dry-run path (D-11)
- One package per D-6 `match` condition (AS-path list, AS-path length, community match, reject-asn, prefix-list), each registering from its own `init()` its condition, its list definition's YANG and parser, and its `migration.MustRegisterTransformation` entry (D-6, AC-30, AC-39). The four that read BGP-only fields sit under `internal/component/bgp/` and read through the BGP view (D-7); the `prefix-list` package and gate keep it at every attachment that accepts it today (R-17). `spec-filter-wire-4-plugins` names the paths after `./le arch tier check`
- `test/plugin/filter-wire-*.ci` - the functional tests above
- The Python 3 test plugin the functional runner forks, placed beside the other test plugins the runner starts (A-7)
- The fast-path conformance harness (FR-9, AC-36), under `pkg/plugin/sdk` so an evaluator author outside the BGP tree can run it; `spec-filter-wire-3-chain` names the package
- A compiled-in third-party-style test filter plugin for AC-35, beside the other test fixtures (`internal/test/fixture/`)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-plugin-callback.yang` `filter-update` input and output change (`internal/core/ipc/yang/`) |
| YANG validation constraints | No | no config leaf added: the batch bound is a constant, not operator-configurable (D-3) |
| YANG custom validators | No | the field check is `policy.CheckChain` |
| CLI commands/flags | Yes | `show policy test` keeps its output through the display-only renderer (D-11, AC-16) |
| CLI grammar (keyword before value) | No | no new command |
| Editor autocomplete | Yes | a BGP chain offers `match` instances, and no longer the five old filter types as members (`route-policy.md` says today "a BGP chain offers no `match`"); `test/ui/completion-words-reject-asn-show.ci` keeps the `show ... reject-asn` completion (AC-40) |
| YANG modules moved | Yes | the five lists' YANG leaves the plugin `yang/` directories for the condition packages, same names and grammar under `policy`; `./le yang glue update` regenerates the glue (R-17) |
| Config migration | Yes | one registered `ze config migrate` transformation per D-6 condition (AC-39) |
| Functional test for new RPC/API | Yes | `test/plugin/filter-wire-*.ci` |
| Pipe completeness | No | `show policy test` already pipes |
| Env var registration | No | none planned |
| Doctor check for runtime dependencies | No | no new runtime dependency |
| Prometheus counters/metrics | Yes | a counter of routes failed by a refused set (AC-6, AC-20), labelled by filter, and a batch size histogram for the slow path (D-3); names follow `docs/plugin-development/metrics.md` and are fixed in `spec-filter-wire-3-chain` |
| BGP family surface (new SAFI / capability / attribute) | No | no family, capability or attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the BGP policy `features/` entry: external filters in any language |
| 2 | Config syntax changed? | Yes | a chain names a `match` instance where it named one of the five old filter types, and the old member form is refused with `ze config migrate` (AC-39, AC-41): `docs/guide/route-policy.md` (the filter type table, the `match` condition table, the completion paragraph's "a BGP chain offers no `match`"), `docs/guide/bgp-policy.md`, `docs/architecture/config/syntax.md`, `docs/config-reference.md`, `docs/guide/configuration.md`, `docs/guide/config-reload.md`, and the `ze config migrate` entry in `docs/guide/command-reference.md` |
| 3 | CLI command added/changed? | No | `show policy test` output is kept (D-11); `docs/guide/command-reference.md` changes only if the trace gains a line |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` if it lists the callback |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (filter plugins: the five removed, the four kept), `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/architecture/plugin/plugin-relationships.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/route-policy.md` (the five as `match` conditions, the condition registry as the way a condition is added), `docs/guide/bgp-policy.md`, and the filter guides that name the five types: `docs/guide/bgp-peering.md`, `docs/guide/bgp-role.md` (the transit-leak obligation, R-16), `docs/guide/route-reflection.md`, `docs/guide/config-editor.md`; `docs/architecture/bgp/filter-path-asn.md` (the plugin becomes a condition, its commands' new owner) |
| 7 | Wire format changed? | No | BGP wire unchanged |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`, `ai/rules/plugins.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | behavior preserved (AC-10) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the runner gains a Python plugin |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: filter plugins in any language |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | five plugins leave the registry and the website catalog (AC-40); the `show ... reject-asn` commands change owner, not path (R-16): `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md`, `docs/architecture/api/commands.md`, `docs/plugin-development/metrics.md` (the reject-asn metrics' owner), `docs/features.md`, `docs/comparison.md`, `docs/DESIGN.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived per child by `./le spec citation anchors spec <child spec>`. Pages declared by `// Design:` headers of files listed here, each judged per child: `docs/architecture/api/ipc_protocol.md` (`pkg/plugin/rpc/types.go`: answer framing, affected if batching changes the message size), `docs/architecture/config/syntax.md` (`bgp/config/register.go`, `peers.go`: unaffected, since D-3 adds no config leaf), `docs/architecture/testing/interop.md` (`internal/le/interoplab/bgp/helper.go`, `helper_srv6_identity.go`: unaffected beyond the helpers' filter handler) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `OnFilterUpdate` and text-subject example in `docs/` |
| 18 | The plugin filter pattern (D-12, D-13) as the documented way to build a filter plugin | Yes | Written by the phase that lands it, never before (pages describe the code as it is). `ai/patterns/plugin.md`, "Route Filters": replace the two routes (`FilterTypes` with `OnFilterUpdate`, and `filterapi.Register`) by the one evaluator registration, the fast-path contract and the conformance harness (phases 3 and 4). `ai/doors/plugin-infra.md`: the typed filter slot in "Boundaries" beside "Every feature works over both transports", and the harness in "Gates" (phase 3). `ai/doors/bgp-plugins.md`: the hook table loses its `filterapi.Register` row and gains the typed filter slot as a fast path with its forked equivalent (phase 4). `ai/doors/system.md`: "Reach another unit through the boundary that survives a process split" lists the EventBus, DirectBridge and the command dispatcher; the phase adds that a typed bridge slot is how DirectBridge serves a per-route callback (phase 3). `ai/doors/bgp-engine.md`: "The engine reaches plugin features only through what they register" loses `filterapi.Register` and "A `filterapi` filter is in-process only" (phase 4). `docs/plugin-development/README.md` ("Architecture"), `protocol.md` ("Runtime Callbacks", the handler section) and `handlers.md`: the one evaluator, its two transports, the contract, ready and unavailable (phase 3). `docs/architecture/api/process-protocol.md`, "Runtime hot path": its `filter-update` row moves from the generic `SendCallback` to the typed slot (phase 3). `docs/architecture/bgp/filter-irr.md`: the snapshot, the removed first-resolution wait (its "The prefix-lists live in memory only" and "A changed as-set starts from no list" paragraphs describe the wait), and the name pattern (phase 4). `docs/architecture/core-design.md`, "Ingress Filter Pipeline": the stage-ordered pass without `filterapi.Register` (phase 4) |

## Implementation Steps

Each phase is one child spec; each follows TDD and ends with a self-critical
review. Order is by dependency. Wiring comes first.

1. **Phase 1: Contract and wiring** (`spec-filter-wire-1-contract`) - the message keyed by field name in the `attr` shapes (D-1), the plugin-level version and the Stage 1 answer and refusal (D-2), raw as a declarable field (D-4), the external reads, sets and name pattern checked by `CheckChain` (D-10); the wiring `.ci` written failing; the Python runner probe (A-7). AC-2, AC-3, AC-5, AC-9, AC-18, AC-21, AC-29.
2. **Phase 2: BGP view** (`spec-filter-wire-2-view`) - the view inside the BGP engine with readers registered from the BGP side, borrowed values, the AS path built once per run, malformed attributes failing the route (D-7); the four new fields and `metric` out of the BGP attachments (D-8); decode counting and zero-allocation tests. AC-1, AC-23, AC-24, AC-28.
3. **Phase 3: BGP chain on the library** (`spec-filter-wire-3-chain`) - BGP chains compile through the library at `bgp import`/`bgp export`; in-process and external members in one chain; set actions and named intents to `ModAccumulator` (D-5); batching with its bound, per-route verdicts and per-batch timeout (D-3); fail-closed refused sets (D-9); the display-only dry-run renderer (D-11); the typed `DirectBridge` filter slot, the SDK's one evaluator registration for both transports, "unavailable" on `ErrBridgeClosed`, and the fast-path conformance harness with its per-call deadline (D-12, FR-8 to FR-11). AC-4, AC-6, AC-7, AC-12, AC-13, AC-15, AC-16, AC-19, AC-20, AC-22, AC-25, AC-26, AC-27, AC-33, AC-34, AC-36. Closes route-policy AC-17 with phase 4.
4. **Phase 4: Plugin migration** (`spec-filter-wire-4-plugins`) - `filter_modify` to `modify`, `filter_remove_private_as`, `filter_family` and `filter_irr` as dedicated types, each with one evaluator on both transports (D-6, D-12); the `filterapi.Register` merge (D-13); fixtures and interop helpers; and the five simple filters to library `match` conditions (revised D-6), in this order:

   | Step | Task | ACs |
   |------|------|-----|
   | 4a | Turn the `match` condition set into a registry (`ai/rules/principles.md`, registration over enumeration): delete `matchConditions`, `RouteMatch`'s per-condition fields and the fixed sequence in `Matches`; register the nine library conditions through it; keep evaluation allocation-free | AC-38 |
   | 4b | One package per D-6 condition, registering from its own `init()` its condition (over the BGP view for the BGP-only fields, D-7), its list definition's YANG and parser applied by the `policy` plugin (R-17), and, for `reject-asn`, the transit-leak obligation (R-16) | AC-30 |
   | 4c | One `ze config migrate` transformation per condition package (`migration.MustRegisterTransformation`), and the load-time refusal of the old member form naming the migrate command | AC-39, AC-41 |
   | 4d | Rewrite every `test/` `.ci` and interop config naming the old types with `ze config migrate`, expectations unchanged, each rewrite judged by `ze-test-judge` | AC-10, AC-30 |
   | 4e | Move `filter_path_asn`'s `show ... reject-asn` commands and metrics to the plugin this phase names after reading `docs/architecture/command-ownership.md`, paths and output unchanged | AC-40 |
   | 4f | Delete the five plugin packages whole, with their `registry.Register` calls, and regenerate `plugin/all` with `./le repo update`; no wrapper, alias or fallback. Per `ai/rules/no-layering.md` each plugin wrapper is deleted before its condition is written in 4b, and the two land in one commit, so no commit carries both the plugin and the condition for one filter | AC-40 |

   AC-10, AC-11, AC-14, AC-30, AC-31, AC-32, AC-33, AC-34, AC-35, AC-37, AC-38, AC-39, AC-40, AC-41.

### The IRR filter under D-12 (owned by `spec-filter-wire-4-plugins`)

Read at HEAD from `internal/component/bgp/plugins/filter_irr/` and
`docs/architecture/bgp/filter-irr.md`. The plugin already owns its state and
its background work in its own goroutines; what changes is how its verdict is
reached and that it stops waiting.

**State and how it is resolved today:**

| State | Symbol | Resolved by |
|-------|--------|-------------|
| Settings: IRR server (default RADB), PeeringDB URL, source address, refresh interval, per peer the remote ASN, the `session irr as-set` leaf and whether its chain names `bgp-filter-irr` | `irrConfig`, `peerIRRConfig` (`config.go`, `parseIRRConfig`), from the `policy irr` container and the bgp root, delivered by `newIRRConfigTx` (`config_tx.go`) | config load and every reload, `handleConfigure` |
| One state per enrolled ASN: `configured` (the operator's as-set, the only query hint), `asSet` (last resolved set), `list` (`*irrPrefixList`), `lastOK`, `lastErr`, counts, `peerAddrs`, `firstDone`, `lookups` | `asnState`, held in `irrPlugin.byASN` under `irrPlugin.mu` | `refreshASNCtx`: `store.PrefixStore.Refresh` asks PeeringDB for the as-set when none is configured, queries the IRR server, keeps a per-family last-known-good in memory; `prefixListFromIRR` refuses a list over `maxPrefixEntries` whole |
| The prefix store, memory only, one for the plugin's life | `irrPlugin.prefixStore` (`store.PrefixStore`, `internal/component/resolve/irr/store`) | first configure creates it, later ones call `UseClients`; `forgetUnconfigured` purges entries the config no longer asks for |
| Prefixes rejected because no list authorized them yet | `readmitter.held` (`readmit.go`), capped at `heldPrefixMax` | `trackVerdict` after each verdict |

**The first-resolution path today:** a configure inside the startup handshake
starts nothing; `OnStarted` sets `started` and starts `initialResolve` and
`refreshLoop`; a reload configure after `started` starts them itself.
`initialResolve` runs `refreshASN` (bounded by `perASNRefreshTimeout`, 30 s)
for every enrolled ASN and closes each `firstDone`. An UPDATE arriving earlier
blocks in `handleFilterUpdate` on `firstDone` for up to `firstResolveWait`
(`rpc.FilterUpdateTimeout` minus one second); on expiry `judgeUnlisted` rejects
the announcements, keeps the withdrawals (`keepWithdrawals`), and
`trackVerdict` holds the prefixes. `refreshLoop` re-resolves every
`refresh-interval`; `update bgp irr all|asn|as-set` runs `refreshAllNow` or
`refreshASN`; the `readmitter` retries a failed or missing ASN every
`readmitRetryInterval`.

**Goroutines that own the state under D-12** (unchanged from HEAD, each a
long-lived worker per `ai/rules/goroutine-lifecycle.md`): `initialResolve`,
`refreshLoop` and the command handlers write `byASN` and the store; the
`readmitter.run` worker owns the held set and sends ROUTE-REFRESH. The
evaluator owns nothing and writes nothing.

**The snapshot it publishes:** one immutable value per publish, swapped
atomically, holding per enrolled ASN its prefix list (or "unresolved"), whether
the last lookup failed, and the set of enrolled ASNs. It is published at every
point that today writes `asnState.list` or `byASN` under `plug.mu`: the end of
`handleConfigure`, and the success and failure arms of `refreshASNCtx`. The
evaluator reads only the snapshot; `show bgp irr` and the `readmitter` keep
reading `byASN` under `mu` as today. A reload that changes an ASN's as-set
publishes that ASN as unresolved, never carrying the old set's list
(`carriedOverBy`).

**The evaluator:** given the view's `prefix` field (all sections, MP families
included) and the snapshot entry for the ASN its filter name carries, it
partitions the announced prefixes as `partitionUpdate` does, answers accept,
reject or the NLRI subset intent of D-5 (replacing `buildModifyDelta`'s text),
keeps withdrawals as `keepWithdrawals` does, and hands the partition to the
`readmitter` through a non-blocking call (today's `trackVerdict` path, whose
common case takes no lock: `heldCount` is atomic). An unrewritable block keeps
today's whole reject.

**An unresolved set (FR-10, AC-32):** no wait. The announced prefixes take the
filter's on-error verdict, reject by default (`FilterOnError` answers
`OnErrorReject` for a filter with no declared policy); the withdrawals of the
same UPDATE still apply, because RFC 4271 Section 9 says the withdrawn routes
"SHALL be removed from the Adj-RIB-In" (quoted at `keepWithdrawals`); the
prefixes are held. When `refreshASNCtx` publishes the list, it calls
`readmitter.notify` as today, and the worker asks the peer with `request peer
<peer> refresh <family>` (RFC 2918 Section 4, gated by `attach process
bgp-filter-irr { send [ refresh ] }` and negotiated Route Refresh). That is the
re-judge mechanism at HEAD: Adj-RIB-In replay cannot serve it, because the
import chain runs before `bgp-adj-rib-in`, which stores only accepted routes
(`readmit.go`, the `readmitter` comment). `firstResolveWait`, `firstDone`,
`signalFirstResolved`, `isClosed` and the wait in `handleFilterUpdate` are
deleted.

**What operators see while a set is unresolved:** `show bgp irr` reports the
ASN `"status":"pending"` (no list, no error) or `"status":"error"` with the
lookup error (`showIRR`, `command.go`); the log carries `irr filter reject`
with `reason no-prefix-list` per UPDATE, and the configure-time warnings when a
peer lacks `send [ refresh ]` or Route Refresh, which say the route stays
rejected until the peer sends it again. Both stay; AC-32 asserts the `pending`
row.

**`clear irr cache`:** it reaches the plugin as the `resolver-cache-clear`
callback (`p.OnResolverCacheClear(store.ResolverCaches(),
plug.clearResolverCache)`) and clears the IRR and PeeringDB clients' response
caches only (`(*PrefixStore).ClearResolverCache`: "The prefix entries the store
holds are the filter's durable copy, not a client cache, and stay"). It
therefore does not touch the snapshot and triggers no re-judge. The next
refresh (periodic, `update bgp irr ...`, or the `readmitter` retry) queries the
servers afresh and, if the list changes, publishes a new snapshot and wakes the
`readmitter`. Over both transports the command takes the plugin's normal
connection, as `clear-irr-cache.ci` (internal) and
`clear-irr-cache-external.ci` (forked) prove today; both stay.

**Its dynamic filter names (D-10):** today `bgp-filter-irr` declares no
`FilterDecl` and no `FilterTypes`; a chain names `bgp-filter-irr:<asn>`,
`extractASNFromFilter` takes the ASN from the trailing digits, and
`validateChainNames` skips the reference. Under D-10 it declares at Stage 1 one
filter name pattern covering `<asn>` (a decimal 32-bit ASN), direction import,
reading `prefix` and `family`, setting nothing but the NLRI subset intent, with
on-error reject; `CheckChain` judges every `bgp-filter-irr:<asn>` against it
(AC-21), and a reference whose suffix is not an ASN fails config load instead
of rejecting every route at run time with "cannot extract ASN".

**Current symbols that change:**

| Symbol | File | Change |
|--------|------|--------|
| `handleFilterUpdate`, `judgeUnlisted`, `judgeListed`, `keepWithdrawals` | `filter_irr.go` | become the one evaluator over the view and the snapshot, with no wait |
| `firstResolveWait`, `asnState.firstDone`, `closeOnce`, `signalFirstResolved`, `isClosed` | `filter_irr.go` | deleted |
| `initialResolve`, `refreshASNCtx`, `handleConfigure` | `filter_irr.go` | publish the snapshot where they write `list` or `byASN` today; the backstop loop that only closed `firstDone` goes |
| `runFilterIRR`'s `p.OnFilterUpdate` registration | `filter_irr.go` | the SDK's one evaluator registration (FR-8) |
| `extractASNFromFilter` | `filter_irr.go` | still reads the ASN at run time; the name pattern refuses a bad one at load |
| `extractNLRIField`, `nlriBlock`, `partitionUpdate`, `buildModifyDelta`, `evaluateUpdate` | `match.go` | the text parsing is replaced by the view's `prefix` field; `buildModifyDelta` by the NLRI subset intent; `evaluateUpdate` (no non-test caller, `plan/journal/production-function-only-tests-call.md`) is deleted |
| `trackVerdict`, `readmitter.track` | `filter_irr.go`, `readmit.go` | fed from the view's prefixes; `heldKey.family` keeps the family name the refresh command takes |
| `rpc.FilterUpdateTimeout` comment | `pkg/plugin/rpc/types.go` | the sentence "a plugin that waits inside its handler MUST bound that wait below this value" loses its one in-tree user; rewritten for the batch bound (D-3) |
| registration | `register.go`, `runFilterIRR` | the name-pattern declaration (D-10) in both `registry.Registration` and `sdk.Registration`, which must agree |
5. **Phase 5: Delete the text machinery** (`spec-filter-wire-5-delete`) - AC-8; the protocol pages rewritten in the same commit; every interop scenario in the table run with recorded reds.
6. **Throughput harness** (inside `spec-filter-wire-3-chain`, which introduces the batch bound it measures) - FR-7 and AC-17; the text-path benchmarks named in `## Relationship to Other Specs` give Thomas the before figure, and he runs it on his Mac.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the two paths produce identical UPDATE bytes for the same sets (AC-4) |
| Naming | JSON keys kebab-case; field names are the registered vocabulary names, never a second list |
| Data flow | a set field reaches the wire only through `buildModifiedPayload`; the reactor names no plugin |
| Rule: no-layering | no text field, text parser or compatibility window left (AC-8) |
| Rule: performance | the view reads borrowed bytes in place; no `fmt.Sprintf` on the chain path |
| Both transports | every plugin filter's verdict is the same internal and forked (AC-31); no plugin filter exists only in-process (AC-37); the five D-6 conditions are the library's, with no plugin left behind them (AC-40) |
| Registration | no `match` condition, list definition or migration is named in a central list (AC-38); `./le arch enumeration check` finds no literal of the condition keys |
| Fast-path contract | every fast-path evaluator passes the conformance harness (AC-36); none waits on unresolved data (AC-32) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| text format gone | `grep -rn "AppendUpdateForFilter\|applyFilterDelta\|parseFilterAttrs\|filtertext" internal pkg` returns nothing |
| declared fields only | `TestFilterMessageDeclaredFieldsOnly` |
| non-Go plugin | `test/plugin/filter-wire-python-plugin.ci` |
| behavior preserved | AC-10 runs |
| five simple filters are conditions, not plugins | `ls internal/component/bgp/plugins` shows no `filter_aspath`, `filter_aspath_length`, `filter_community_match`, `filter_path_asn` or `filter_prefix`; `grep -n "matchConditions" internal/component/policy` returns nothing; `TestMatchConditionRegistry`, `TestMigrateSimpleFilters`, `TestOldFilterTypeRefused` pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a plugin reply naming an unknown field, an unsettable field, a malformed value, or more verdicts than routes in a batch is refused, never applied partly |
| Fail-closed | every failure path applies `on-error`, default reject |
| Resource exhaustion | batch and message size bounded; a plugin cannot make Ze decode undeclared fields |

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

- External `<plugin>:<filter>` references are the one chain member the load-time check cannot see today; FR-2's rule that the declared set is both what is sent and what is checked closes that gap by construction (D-10).
- The text subject was never narrowed by `FilterDecl.Attributes` (`AppendUpdateForFilter` is always called with a nil list); parse on demand is a new guarantee, not a restored one.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

No open journal row names this spec at 544f8f930c.

## Key Design Decisions

Decided only. D-1 to D-11 answer Q-1 to Q-11; the owner approved each
recommendation on 2026-10-31. D-12 is the owner's revision of the same day,
which supersedes his answer to Q-12; D-13 is its no-layering consequence. D-6
was revised by the owner the same day, answering R-12 with option (a).

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1 (Q-1, owner, 2026-10-31): the slow-path message carries the declared fields as one JSON object keyed by registered field name, each value in the shape the UPDATE event `attr` map uses (`docs/architecture/api/json-format.md`), so the event encoders are reused. Shapes the `attr` table lacks are written on the protocol page | a per-field value table of its own; per-field typed JSON with type tags | one JSON style for events and filters, and a new field needs only its row; the other two meant a second contract to maintain or larger messages and more SDK code |
| D-2 (Q-2, owner, 2026-10-31): one plugin-level protocol version in `declare-registration`; Ze answers with the version it will speak, and a plugin declaring none (the text era) gets one clear startup refusal naming the version it must speak | a version per `filters[]` entry; a capability list | one negotiation serves every future contract change, not only filters; a capability list multiplies the combinations to test |
| D-3 (Q-3, owner, 2026-10-31): a batch holds what is queued for the filter at dispatch, up to a bound. Each route gets one verdict, in request order, with its own sets; per-source FIFO order holds across batches; one `rpc.FilterUpdateTimeout` covers a batch; a failed batch applies the filter's `on-error` to each of its routes | a fixed batch size; an operator-configurable size; a timeout scaled per route | no added latency when one route is queued, throughput when many are, no config leaf, and the cache-ack FIFO (`bgp-plugins.md` trap) and ingress order held |
| D-4 (Q-4, owner, 2026-10-31): raw is one more declarable field. Whole-body raw stays available, and so does the terminal whole-body rewrite answer, as `filter_family`'s rewrite needs | keep `FilterDecl.Raw` beside the field list; raw per attribute | one declaration mechanism for everything a plugin reads; per-attribute raw cannot carry a whole-body rewrite |
| D-5 (Q-5, owner, 2026-10-31): plain attributes are set fields with an action (set, add, remove, prepend, increment, decrement), covering communities, MED and local-preference. Named intents that Ze resolves: AS-path prepend (needs the local AS), remove-private-AS (needs the peer AS and 4-byte AS), NLRI subset per section, and family strip | every change a set field; raw rewrites for NLRI and family changes | a flat vocabulary for what a plugin can compute, and reactor-side resolution for what needs state the plugin structurally lacks; no raw needed to drop a prefix |
| D-6 (Q-6, owner, 2026-10-31; revised by the owner, 2026-10-31 (R-12), option (a)): the five simple filters, `filter_aspath`, `filter_aspath_length`, `filter_community_match`, `filter_path_asn` and `filter_prefix`, become route-policy library `match` conditions and stop being plugins. Each condition registers from its own package, so no central list names them; at HEAD `matchConditions` is a literal table, so the condition set first becomes a registry (phase 4a). The conditions are the library's and run wherever the library runs, never over a plugin transport. `ze config migrate` maps the old filter names and instances onto the equivalent `match` conditions, and a config still naming an old filter type is refused with that command. The five plugin wrappers are deleted, with no layering; `filter_path_asn`'s `FilterObligations` declaration (`filterapi.ObligationTransitLeak`) merges into its `reject-asn` condition, and its commands and metrics keep an owner (R-16). `filter_modify` becomes `modify`. `filter_remove_private_as` and `filter_family` stay dedicated types, and `filter_irr` stays a dedicated type for its dynamic names (D-10); the revised D-12 removes its blocking (FR-10). These four stay plugins with one evaluator on both transports (D-12) | a dedicated evaluator per plugin; some plugins external only; (b) a `match` condition owned by its plugin, following the plugin's transport, so each of the five keeps a forked variant | the five are pure checks on route fields and hold no state of their own, so a forked variant buys nothing; one condition model shared with the other attachments; fewest evaluators; registration over enumeration for the conditions and their migrations (AC-30, AC-38 to AC-41) |
| D-7 (Q-7, owner, 2026-10-31): a BGP view type inside the BGP engine, over `WireUpdate`/`AttributesWire`, with per-field readers registered from the BGP side, so the policy library stays protocol-neutral. Values are borrowed slices valid only during the synchronous chain run; the AS path is rebuilt once per run and cached; a malformed attribute fails the route | an optional BGP reader on `policy.RouteView`; a field-reader registry in `filterapi` | `internal/core/rib/policy` and `internal/component/policy` learn no BGP type; zero copy; no silent zero (`ai/rules/principles.md`) |
| D-8 (Q-8, owner, 2026-10-31): register `atomic-aggregate`, `aggregator`, `originator-id` and `cluster-list` as fields, and keep one name per fact: BGP uses `med`, not both `metric` and `med` | drop the four from what filters read; reach them only through raw; keep `metric` beside `med` | every value the text subject exposed stays readable on demand; two names for one fact is a second declaration (`ai/rules/principles.md`) |
| D-9 (Q-9, owner, 2026-10-31): a reply that sets an undeclared or unsettable field fails that route: fail-closed, counted and logged | drop the undeclared set with a warning and apply the rest; fail the whole batch | no silent partial application, and routes in one batch stay independent |
| D-10 (Q-10, owner, 2026-10-31): an external filter declares the fields it reads and sets, in vocabulary names, at Stage 1, and `CheckChain` judges it like a named instance. A plugin with dynamic filter names, such as `bgp-filter-irr:<asn>`, declares a name pattern | declare reads only and judge sets at run time; map `FilterDecl.Attributes` text names onto fields | the external reference stops skipping the load-time check, and the declared set is both what is sent and what is checked (FR-2) |
| D-11 (Q-11, owner, 2026-10-31): dry-run and trace use a display-only renderer of the view and the set fields. It is never a transport | structured JSON only, rendered by the command's output layer | `show policy test` output, which operators and tests read, is kept; the renderer cannot become a second text contract (FR-4) |
| D-12 (owner, 2026-10-31, supersedes the earlier D-12): a filter is owned by its plugin, whatever process the plugin runs in. A plugin running internal is called through a typed `DirectBridge` filter slot, shaped like `DeliverStructured`: the lazy wire view in, the verdict and sets back, synchronous on the reactor goroutine, no text and no JSON. A plugin running forked, in any language, answers the batched, versioned `filter-update` message (D-1 to D-11). The author writes the evaluator once and the SDK installs it on whichever transport the plugin runs on. The evaluator honours the fast-path contract (FR-9), never blocks on unresolved data (FR-10), and is callable only between Stage 5 and `CloseCallbacks`, answering "unavailable" otherwise (FR-11). The slow path is proven by in-tree plugins run forked and by the non-Go (Python) test plugin. The earlier D-12, "a compiled-in filter type always evaluates in process", whatever the plugin's placement, is withdrawn | the earlier D-12; a new SDK direct-call registration beside the bridge; in process only, with no forked equivalent | the bridge already is Ze's in-process transport of the protocol (1a5aa8029b, `DeliverStructured`), so the fast path needs one slot, not a second mechanism; the earlier D-12 put a forked plugin's evaluation in the daemon, away from the state it reads (the closed R-9) |
| D-13 (follows from D-12 and `ai/rules/no-layering.md`, 2026-10-31): the `filterapi.Register` family and the bridge path end as one filter mechanism. Each row of the `## Current Behavior` table becomes a plugin filter reached through the typed slot internal and the protocol forked, keeping its stage order, or is named with the reactor state that keeps it engine-side. Starting reading, which `spec-filter-wire-4-plugins` proves row by row: `bgp-filter-community`, `bgp-filter-community-relation` and `bgp-role` become plugin filters, with the stage and priority order and `meta["src-role"]` carried as declared fields; `bgp-gr`'s LLGR filters become plugin filters reading `LLGRAdvertised` and the stale mark as declared fields; `bgp-rs`'s `PreserveOpaqueAttributes` is a forwarding choice with no verdict, so it leaves the filter family for a plugin declaration; `loop` is engine code reading `LocalAS`, `RouterID`, `ClusterID` and `AllowOwnAS`, and stays engine-side as an RFC 4271 and RFC 4456 protocol check, not a plugin filter (R-14) | keep `filterapi.Register` beside the bridge path | two filter mechanisms, one of which silently vanishes when its plugin runs forked, is the layering the rule forbids and the boundary defect `ai/doors/system.md` names |
| This spec is route-policy phase 4's BGP half, on the shared library (Disposition, 2026-10-31) | a BGP-only mod-op and intent vocabulary in the SDK (the July design) | a second filter model beside the library would be undone by phase 4 |
| Two paths, one vocabulary (owner, 2026-10-31, FR-1) | in-process only; external only | plugins in any language are part of Ze's design, and a plugin running internal needs no encoding (D-12) |
| Parse on demand on both paths (owner, 2026-10-31, FR-2) | send every attribute, as the text subject does | Ze decodes and serialises nothing a filter did not ask for |
| The in-process fast path is a lazy view over `WireUpdate` (owner approved, 2026-10-31) | evaluate over a decoded struct | lazy over eager; zero copy |
| The contract is versioned, documented in the protocol pages, and the Go SDK is a convenience over it (owner, 2026-10-31, FR-3) | the SDK as the contract | a non-Go author reads the page, not Go types |
| Replace the text format; third-party text plugins break (owner, 2026-10-31, FR-4) | keep text beside the new message during migration | no layering; pre-release |
| Batching is the slow path's performance lever (owner, 2026-10-31, FR-6) | a typed DirectBridge slot as the slow path's lever | batching helps forked plugins, which a bridge never reaches; the typed slot is the internal plugin's transport under the revised D-12, never a substitute for batching on the forked path |
| The benchmark is a harness Thomas runs on his Mac (owner rule, `ai/META-DECISIONS.md`) | an agent-run benchmark gate (the July Phase 1) | perf is measured only on known hardware |
| BGP import stays the per-peer ingress chain at `FilterStagePeerChain`, and no new text representation is added (route-policy Key Design Decisions and R-6) | move BGP import to the Loc-RIB gate | BGP import must see communities and AS path before best path |
| Child specs live under `plan/post-release/engineering/` (`ai/META-DECISIONS.md`) | the `plan/` root (the July design) | plans live only in subfolders of `plan/` |

## Known Limitations

- Third-party filter plugins written against the text `Update` format stop working when this lands (FR-4, owner-accepted). The refusal at startup tells their authors which contract to speak (AC-9).
- A `match` instance with a `metric` condition no longer loads in a BGP chain, because BGP names that fact `med` only (D-8, AC-24). It still loads at the Loc-RIB, redistribute and IGP attachments.
- Storage of BGP routes rejected by import stays in `plan/post-release/features/spec-bgp-filtered-route-storage.md`.
- The re-judge of FR-10 covers what HEAD's `readmitter` covers: prefixes rejected because a list did not authorize them yet. An accepted route whose prefix a later refresh removes from the list is judged again only when the peer next sends it, as at HEAD; the 2026-10-17 row of `plan/journal/announced-state-never-replayed.md` records that no RFC obliges more. A re-judge on import needs Route Refresh negotiated and `attach process <plugin> { send [ refresh ] }`, as today.
- A Go filter plugin outside this repository cannot be compiled into ze at HEAD (R-13); AC-35's fast path is for Go code compiled into the tree.
- A config that names `as-path-list`, `as-path-length`, `community-match`, `reject-asn` or `prefix-list` as a chain member, or one of the five removed plugins, stops loading until `ze config migrate` rewrites it (revised D-6, AC-41). The list definitions themselves load unchanged.

## Relationship to Other Specs

| Spec | Relationship |
|------|--------------|
| `plan/post-release/features/spec-route-policy-0-umbrella.md` | This spec is its phase 4 BGP half. Its AC-17 is this spec's AC-10; its R-6 is resolved by this spec adding no text representation. Both stay open until AC-10 is green |
| `spec-perf-next-2-filter-delta-alloc` (closed 2026-09-05) | It cut allocations in `textDeltaToModOps`. That path is deleted here (AC-8); its benchmark `BenchmarkFilterModifyEgress` (`reactor/filter_delta_test.go`), with `BenchmarkAppendUpdateForFilter_Reused` and `BenchmarkAppendAttrsForFilter_Reused` (`reactor/filter_format_bench_test.go`), measure the text path and give Thomas the before figure for FR-7 before deletion |

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
- [ ] Every Open Design Question answered and moved to Key Design Decisions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-41 all demonstrated (across children)
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits. The full verification is Thomas's release check
- [ ] Feature code integrated (`internal/*`, `pkg/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features, each with a recorded red

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script

## Work Inherited From a Deferral Row

From `filter-wire-0-umbrella.md`, 2026-07-15, deferred by rib-arch-2 (Option
A, learned 1127): remove the text `Update` filter IPC carrier entirely,
rewrite the text-parsing filter plugins, and rework the modify-delta apply.
The binary `Raw` carrier it mentions shipped. At HEAD the item is FR-4 and
FR-5 of this spec, proven by AC-8, AC-14 and AC-30; it now covers nine
plugins, not seven, of which five become `match` conditions under the revised
D-6.

## Disposition

| Field | Value |
|-------|-------|
| Date | 2026-10-31 |
| Checked at | ca71630763 |
| Verdict | REWORK |
| Goal still holds | The external BGP filter chain is still a text round-trip: `PolicyFilterChain`, `applyFilterDelta`, `parseFilterAttrs`, `formatFilterAttrs`, `textDeltaToModOps`, `AppendUpdateForFilter`, `buildModifiedPayload` and the SDK `FilterUpdateHandler` all exist, and `FilterUpdateInput`/`FilterUpdateOutput` still carry `Update string` |
| Landed since the design (c2b684d57e, 2026-07-16) | None of AC-1..AC-8. 6019dd0675 replaced `extractLegacyNLRIOverride` with `sectionNLRIDelta` (`reactor/filter_nlri_delta.go`), adding `nlri add/del` and `mp <family> add/del` to the text protocol, so MP_REACH rewrite already works over text: the Known Limitation and A-5 are stale |
| Moved | `filterapi.FilterRef`/`FinalDisposition` are `policy.Ref`/`policy.Final` (`internal/component/policy/chain.go`, faf7c5c380); the filter registry is `policy.BuildRegistry`; the core-design pipeline section is now "Ingress Filter Pipeline" |
| Ground shifted | The route-policy library (`plan/post-release/features/spec-route-policy-0-umbrella.md`) owns the field vocabulary (`MustRegisterField`, `MustRegisterFilterType`, `MustRegisterAttachment`), set actions (`RouteResult` sets), in-process evaluators (`MustRegisterRouteEvaluator`) and the load-time read/set check (`policy.CheckChain`), and its phase 4 folds this spec in |
| Rule conflicts | Child specs listed at the `plan/` root, which `ai/META-DECISIONS.md` bans; Phase 1 has an agent run a benchmark, which only Thomas runs, on his Mac |

As written, this spec would give BGP a second filter model beside the library:
its own mod-op and intent vocabulary in the SDK, its own declared-attribute
check, and chains composed in the reactor from `filterapi.Policy`. Route-policy
phase 4 would then have to undo it. The goal (no text round-trip on the BGP
filter path, wire-native zero copy) is unchanged and still owed.

The smallest rework that fits main makes this spec route-policy phase 4's BGP
half. It deletes the text machinery without inventing a composition model. It
defines one lazy BGP route view over `WireUpdate`, which in-process library
evaluators read and the external filter IPC carries as the raw update plus SDK
field accessors. A filter's result is expressed as the library's registered set
fields, each turned into a `ModAccumulator` op by a registered handler, with
AS-path prepend and remove-private kept as typed intents. AC-7 becomes
`policy.CheckChain` at load plus a runtime check over the same field codes.
Current Behavior and AC-6 are rebuilt from `sectionNLRIDelta` and the `mp`
sections, child specs move under `plan/post-release/`, and Phase 1 becomes a
benchmark harness Thomas runs. The redesign runs through `/ze-spec`.

**Fixed requirement for the redesign (owner, 2026-10-31): two paths, one
vocabulary.** Plugins in any language are part of Ze's design
(`ai/doors/system.md`: a third-party plugin, in Go or any other language, is a
forked process speaking the process protocol). The lazy wire view above is
the in-process fast path only, for filter types compiled into the binary. The
out-of-process slow path stays language-neutral: the process protocol carries
the raw UPDATE bytes plus a decoded map of only the fields the plugin declared
at startup, and the plugin answers a verdict plus set fields named from the
same registered vocabulary, in JSON, which Ze applies through the same
handlers as the fast path. Both paths share the vocabulary, the load-time
read/set check, and the chain model, so one chain may mix in-process and
external members. The decoded-map and reply encoding is a versioned, documented
part of the plugin protocol, and the Go SDK is a convenience over it, never a
second contract. The text `Update` format is replaced, not kept beside it.
Batching several routes per exchange is the slow path's performance lever, and
the benchmark harness measures both paths.

**Parse on demand, on both paths (owner, 2026-10-31).** A filter receives only
the route information it asks for, and Ze decodes and serialises nothing more.
In process, the lazy view decodes a field on first read. Out of process, a
plugin declares at startup the fields it reads, and Ze decodes and sends only
those for each route, so a filter that reads only the prefix is never sent the
AS path or the communities. The raw UPDATE bytes go only to a plugin that
declares it wants them. The load-time check uses the same declaration, so the
set of fields a plugin may read is the set it is sent. The owner approved the
in-code fast path as proposed above.

**The consequence, stated outright:**

- Replacing the text `Update` filter IPC breaks third-party filter plugins
  written against the text format. The new contract is versioned and
  documented in the plugin protocol pages
  (`docs/architecture/api/process-protocol.md`, `docs/plugin-development/`),
  and the in-tree filter plugins migrate with it.
- The redesign must define what an external plugin sees as protocol version
  or capability negotiation, so a plugin can tell which contract it is
  speaking.
