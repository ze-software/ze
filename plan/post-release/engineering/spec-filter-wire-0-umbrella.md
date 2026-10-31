# Spec: filter-wire-0-umbrella -- BGP policy filters without the text round-trip (route-policy phase 4, BGP half)

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P3 |
| Release reason | Release-independent filter wire 0 umbrella work; historical findings do not establish a current product defect. |
| Depends | Route-policy phases 1-3, landed (490618ef43). This builds on their library in `internal/component/policy/`: `policy.CompileRouteChain`, `policy.RouteChain` and `policy.CheckChain`; the field registry, `policy.MustRegisterField` with a `policy.RouteView` (`Read`, `Written`); `policy.MustRegisterRouteAttachment`; and `ribpolicy.Route` and `ribpolicy.Sets` (`internal/core/rib/policy`) |
| Phase | - |
| Updated | 2026-10-31 |

## Read this first

Status: **design**, body current at 544f8f930c (2026-10-31).

Every section from `## Task` through `## Work Inherited From a Deferral Row`
was rewritten against HEAD on 2026-10-31. It replaces the July 2026 design,
which `## Disposition` (kept at the end as the audit trail) judged REWORK. The
owner's fixed requirements in the Disposition are stated as requirements in
`## Requirements`; nothing in this body overrides them.

Only the design gate remains. The next step is `/ze-spec` on this file, in
the main thread: it answers the questions in `## Open Design Questions` with
the owner, one row at a time, records each answer under `## Key Design
Decisions`, and moves the spec to `ready`. Until then no row of that table is
decided, and no child spec exists.

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
| In-process fast path | A filter type compiled into the binary | A lazy view over the received `WireUpdate` | On first read, by the view, from the wire bytes |
| Out-of-process slow path | A plugin in any language, a forked process speaking the process protocol | A language-neutral, versioned message: the fields the plugin declared at startup, decoded; the raw UPDATE only when declared | Once per route, only for the fields declared |

Both paths answer a verdict plus set fields named from the same vocabulary,
and Ze turns every set field into the same UPDATE change through the same
attribute handlers. One chain may mix members of both paths. The text
`Update` format of the `filter-update` RPC is replaced, not kept beside the
new one, and the in-tree BGP filter plugins move with it.

**Scope.** BGP import and export policy chains (`filter { import|export { ...
} }` at bgp, group and peer level), the `filter-update` contract in
`pkg/plugin/rpc` and `pkg/plugin/sdk`, the reactor call sites, the dry-run
surface (`show policy test`), the nine in-tree BGP filter plugins, and the
protocol pages. Loc-RIB, redistribution and IGP chains are route-policy
phases 2 and 3 and stay untouched.

## Required Reading

<!-- NEVER tick [ ] to [x]. Annotations are the knowledge. -->

### Design doors
- [ ] `ai/doors/system.md` - the design every subsystem shares
  → Constraint: a third-party plugin "in Go or any other language" is a forked process speaking the process protocol; the slow path's contract is that protocol, never a Go type. A change under `pkg/` is a published contract change, made on purpose with its page.
  → Constraint: DirectBridge is "an in-process optimization of the same protocol, not a second protocol"; the in-process fast path for compiled-in types is therefore a registration (like `filterapi.Register`), and an in-tree plugin run forked must still work through the slow path.
- [ ] `ai/doors/bgp-engine.md` - reactor, wire, filter pipeline
  → Constraint: "A policy plugin" depends on the `filter-update` RPC (`FilterUpdateInput`, `FilterUpdateOutput`); changing it breaks code outside the repository, so the page edit lands in the same change.
  → Constraint: received bytes are borrowed (`RawMessage.IsAsyncSafe()` false); a lazy view over them must not escape the synchronous chain run, and a batch held across goroutines must copy or retain the cache entry first.
  → Constraint: the trap "a filter modify rewrote only the IPv4 body NLRI" (2026-10-17) is what `sectionNLRIDelta` fixed; its MP_REACH/MP_UNREACH behaviour must survive the redesign.
- [ ] `ai/doors/bgp-plugins.md` - the filter plugins
  → Constraint: in-tree plugins hold no privilege over a third-party one on the protocol; every in-tree filter must be reachable through the documented contract, and the in-process fast path is an extra registration, not a replacement of the contract.
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
  → Constraint: there is no protocol version field anywhere in the startup today (grep of the page and of `DeclareRegistrationInput`); version negotiation is new design (Q-2).
- [ ] `docs/architecture/api/architecture.md`, "Wire Bytes in Events", and `docs/architecture/api/json-format.md`
  → Decision: the UPDATE event already has a decoded JSON attribute map (`attr`) beside optional raw hex (`raw`); it is a candidate shape for the decoded-map encoding (Q-1), so a plugin author reads one JSON style for events and filters.
- [ ] `docs/plugin-development/protocol.md`, "Runtime Callbacks (Engine to Plugin)" and "The Filter Text an `OnFilterUpdate` Handler Reads"
  → Constraint: the author-facing page promises `rpc.FilterUpdateTimeout` (5 s) per call and the on-error policy on expiry; batching (Q-3) must restate that bound per batch or per route.
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
- The BGP fields are registered with an empty `policy.RouteView{}` (`internal/component/policy/register.go`): `RouteView` reads a `ribpolicy.Route`, which holds no BGP attribute. The lazy wire view therefore needs its own read path (Q-7).
- Three BGP filter plugins already compile in-process route evaluators for the Loc-RIB (`filter_prefix`, `filter_family`, `filter_modify`, each `rib_evaluator.go`), but over `ribpolicy.Route`, not over an UPDATE; BGP chains still reach them by RPC.

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

**The nine in-tree filter plugins that answer `filter-update`** (all under
`internal/component/bgp/plugins/`):

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

**Behavior to change:**
- The `filter-update` message: text `Update` in and out is replaced by declared decoded fields, optional raw, and set fields from the vocabulary, under a negotiated contract version. Third-party text-format plugins stop working (owner-accepted).
- Compiled-in filter types evaluate in process over a lazy wire view, without an RPC.

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
2. The chain is compiled once at config load through the library, against the `bgp import` or `bgp export` attachment, members in order, each either an in-process evaluator or an external filter with its declared fields (Q-6, Q-10, Q-12).
3. In-process member: reads fields through the lazy view (Q-7); each field decodes on first read from the wire bytes, no copy.
4. External member: Ze decodes only the fields the filter declared at startup, adds raw only if declared (Q-4), encodes the versioned message (Q-1, Q-2), possibly batched with other routes (Q-3); the plugin answers a verdict and set fields named from the vocabulary.
5. Each member's sets apply to the view the next member reads, as `RouteChain.Evaluate` does for `ribpolicy.Route`.
6. After the chain, set fields and typed intents (Q-5) become `ModAccumulator` ops through registered per-field handlers; `buildModifiedPayload` writes the body.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to external filter plugin | The versioned `filter-update` message (JSON on the process protocol; over DirectBridge for an internal plugin, MuxConn over TLS for a forked one) | No |
| Reactor to in-process evaluator | A registered function over the lazy view, inside the reactor's goroutine, synchronous | No |
| Config to reactor | The compiled chain inside `filterapi.Policy` through `PeerSettings` and reload | No |
| Set field to wire | Registered per-field handler writing `ModAccumulator` ops, applied by `buildModifiedPayload` | No |

### Integration Points
- `filterapi.Policy` and its in-process bindings: the compiled chain replaces or extends what `BindInProcess` binds (Q-6, Q-12).
- `policy.CheckChain` at the `bgp import`/`bgp export` attachments: gains external filters' declared fields (Q-10).
- `buildModifiedPayload`, `attrModHandlersWithDefaults`, `filterapi.RegisterAttrModHandler`: unchanged apply engine.
- `ExtractASPathPrependOps`, `ExtractRemovePrivateASOps`, `subsetNLRI`, `mpSectionDelta`: kept as the wire-side halves of typed intents, driven without text (Q-5).
- `internal/le/perf`: the throughput harness for both paths (FR-7).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to fill at design: every set field reaches the wire only through `buildModifiedPayload` |
| No unintended coupling (components stay isolated) | No | to fill at design: the reactor names no filter plugin; plugins reach Ze only through the contract or a registration |
| No duplicated functionality (extends existing, does not recreate) | No | to fill at design: one vocabulary, one chain model, one apply engine; the text machinery deleted, not kept |
| Zero-copy preserved where applicable (refs, not copies) | No | to fill at design: the lazy view reads borrowed bytes in place; the slow path encodes only declared fields |
| Registration over hardcoding, outbound | No | to fill at design: the lazy view's field reads and the set-field handlers register per field |
| Registration over hardcoding, inbound | No | to fill at design: lists to search include `attrNameToCode`, `validateModifyDelta`, `textDeltaToModOps`' per-attribute arms, `FilterDecl.Attributes` consumers |

## Requirements

The owner fixed these on 2026-10-31 (`## Disposition`). They are not open.

| ID | Requirement |
|----|-------------|
| FR-1 | Two paths, one vocabulary. Compiled-in filter types run in process over a lazy wire view; plugins in any language run out of process over the process protocol. Both read and set fields named from the library's registered vocabulary, share the load-time read/set check and the chain model, and one chain may mix both. |
| FR-2 | Parse on demand on both paths. In process, the view decodes a field on first read. Out of process, a plugin declares at startup the fields it reads, and Ze decodes and sends only those per route; a filter reading only the prefix is never sent the AS path or communities. The raw UPDATE bytes go only to a plugin that declares it wants them. The set of fields a plugin may read is the set it declared, which is the set the load-time check uses. |
| FR-3 | The out-of-process contract is versioned and documented in the protocol pages (`docs/architecture/api/process-protocol.md`, `docs/plugin-development/`). The Go SDK is a convenience over it, never a second contract. A plugin can tell which contract version it speaks (negotiation, Q-2). |
| FR-4 | The text `Update` format is replaced, not kept beside the new one (no layering). Third-party filter plugins written against the text format break, and that is accepted. |
| FR-5 | The in-tree BGP filter plugins migrate to the new contract (and, where compiled in, to the fast path) in the same body of work. |
| FR-6 | Batching several routes per exchange is the slow path's performance lever. |
| FR-7 | A throughput harness measures both paths. Thomas runs it on his Mac; no agent runs a perf measurement. |

## Open Design Questions

None of these is answered. `/ze-spec` takes each to the owner (or answers it
from an RFC, an agreed design, or FRR and BIRD source, per
`ai/META-DECISIONS.md`) and records the answer in `## Key Design Decisions`.

| ID | Question | Options | What each answer changes |
|----|----------|---------|--------------------------|
| Q-1 | How are the declared fields encoded in the slow-path message? | (a) A JSON object keyed by registered field name (`as-path`, `community`, ...), values in the shapes `docs/architecture/api/json-format.md` uses for the UPDATE event `attr` map. (b) A JSON object whose value shapes are defined field by field in a new table on the protocol page. (c) Per-field typed JSON with explicit type tags. | (a) one JSON style across events and filters, the event formatter's encoders are reused, and field names come from the registry, so a new field needs no page edit beyond its row; ties the filter contract to the event format's evolution. (b) a filter contract independent of events, more page to maintain. (c) self-describing, larger messages, more SDK code. Every choice decides how NLRI (a list per section and family, with path id and label stack handling) is represented. |
| Q-2 | How do Ze and a plugin agree on the contract version at startup? | (a) A version field in each `filters[]` entry of `declare-registration`; Ze refuses an unknown or text-era version at Stage 1. (b) One plugin-level protocol version in `declare-registration`, Ze answers the version it will speak. (c) A capability list (`filter-fields`, `filter-batch`, `filter-raw`) rather than a number. | (a) per-filter granularity, simplest refusal. (b) one negotiation for every future contract change, not only filters. (c) features evolve independently, more combinations to test. Each decides the refusal message a text-era plugin sees (`ai/rules/user-facing-errors.md`) and what AC-12 asserts. |
| Q-3 | What are a batch's size and semantics? | Size: (a) fixed maximum, (b) whatever is queued for that filter at dispatch, up to a bound, (c) operator-configurable. Verdicts: one per route, in request order, each with its own sets. Ordering: per-source FIFO preserved across batches. Timeout: per batch of `rpc.FilterUpdateTimeout`, or scaled per route. Failure: on-error applied to every route of a failed batch. | Size and dispatch decide latency versus throughput and whether a goroutine accumulates routes (a long-lived worker per filter, `ai/rules/goroutine-lifecycle.md`). Ordering decides whether the cache ack FIFO (`bgp-plugins.md` trap) and ingress order hold. The timeout rule decides what `docs/plugin-development/protocol.md` promises. Batching across UPDATEs also decides whether borrowed receive buffers must be copied or cache-retained first (`bgp-engine.md`). |
| Q-4 | How is raw opted into and carried? | (a) Keep `FilterDecl.Raw`; raw stays base64 in JSON. (b) Raw as one more declarable field, carried the same way. (c) Raw carried per attribute (only the declared attributes' wire bytes) rather than the whole body. | (a) no declaration change. (b) one declaration mechanism for everything a plugin reads. (c) smallest messages for raw users, but `filter_family`'s whole-body rewrite and `filter_remove_private_as`' AS4_PATH read need checking against it. Also decides whether a raw full-body rewrite answer survives (today terminal, `decodeFilterRawOverride`). |
| Q-5 | Which changes are typed intents beyond plain set fields? | Candidates: AS-path prepend (needs local AS), remove-private-AS (needs peer AS and ASN4 width), NLRI subset per section (the equivalent of `sectionNLRIDelta`), family strip (what `filter_family` does by raw rewrite), community add and remove (list edits versus a whole-list set), arithmetic (`filter_modify` increment and decrement). Options per candidate: (a) a set field with an action (set, add, remove, prepend), (b) a named intent the reactor resolves, (c) a raw rewrite. | Decides the reply vocabulary, which reactor helpers stay (`ExtractASPathPrependOps`, `ExtractRemovePrivateASOps`, `subsetNLRI`, `mpSectionDelta`), and whether a plugin can express an NLRI change without raw. A set-with-action keeps the vocabulary flat; named intents keep AS context reactor-side, which the plugin structurally lacks. |
| Q-6 | How does each of the nine plugins map onto the library? | Per plugin: (a) becomes, or is served by, `match`/`modify` instances, (b) stays a dedicated filter type with its own evaluator, (c) stays an external filter only. Candidates by reading: `filter_aspath`, `filter_aspath_length`, `filter_community_match`, `filter_path_asn` read only; `filter_prefix`, `filter_irr` subset NLRI; `filter_modify`, `filter_remove_private_as` set attributes; `filter_family` strips or tears down. | Decides how many evaluators are written, whether `filter_irr` (named by ASN, blocking on first resolution) can be in process at all, and what operators type (filter type names are operator-visible and preserved by AC-10). |
| Q-7 | What is the lazy wire view, and what are its zero-copy and pool constraints? | (a) A BGP view type in the BGP engine over `WireUpdate` and `AttributesWire`, with per-field readers registered from the BGP side (a second view kind beside `RouteView`). (b) Extend `policy.RouteView` with an optional BGP reader over an interface the BGP engine implements. (c) A new field-reader registry in `filterapi`. Constraints to settle: decoded values returned as borrowed slices or iterators (no allocation), validity only during the synchronous chain run, the reconstructed AS path computed once and cached for the run, behavior on a malformed attribute. | (a) keeps `internal/core/rib/policy` free of BGP and the library protocol-neutral. (b) one view abstraction, but the library learns a BGP interface. (c) closest to the apply engine, furthest from the vocabulary. Also decides what AC-1 counts and how the slow path's encoder reuses the same readers. |
| Q-8 | Which attributes does the BGP attachment carry and let a filter set? | Today's lists (`internal/component/bgp/config/register.go`) carry 14 fields and allow 9 sets. The text protocol also renders `atomic-aggregate`, `aggregator`, `originator-id` and `cluster-list`, which the vocabulary does not register. Options: (a) register those four as fields, (b) drop them from what filters can read, (c) reach them only through raw. Also: whether `metric` and `med` stay two names for one BGP fact. | (a) keeps every value the text subject exposes reachable on-demand. (b) is a contract narrowing a plugin author may depend on. (c) forces raw for them. Decides the field rows in the protocol page and what AC-4 and AC-7 can name. |
| Q-9 | How are declared fields enforced at run time? | (a) Ze sends only declared fields, so reads are enforced by construction; replies setting an undeclared or unsettable field fail the route (fail-closed, counted, logged), as `validateModifyDelta` does today. (b) Undeclared sets are dropped with a warning and the rest applied. (c) Undeclared sets fail the whole batch. | (a) keeps today's AC-13 behavior. (b) is a silent partial application, which `ai/rules/principles.md` refuses unless named. (c) couples routes in a batch. Decides AC-6 and the counter. |
| Q-10 | How does an external `<plugin>:<filter>` join the load-time check? | (a) Its Stage 1 declaration names the fields it reads and sets in vocabulary names, and `CheckChain` judges it like a named instance. (b) It declares reads only; sets are judged at run time (Q-9). (c) Map today's `FilterDecl.Attributes` names onto fields (route-policy A-5). | (a) closes the gap where external refs skip `CheckChain` today (`validateChainNames`), and makes FR-2's "the set it may read is the set it is sent" a load-time fact. (b) leaves set refusals to run time. (c) keeps the old declaration, which conflicts with FR-4 if it carries text-era names. A plugin whose filter names are dynamic (`bgp-filter-irr:<asn>`) needs a declaration that covers a family of names. |
| Q-11 | What do dry-run and trace show without the text subject? | (a) A display-only renderer of the view and the set fields, used only by `show policy test` and logs. (b) Structured JSON only (fields and sets), rendered by the command's output layer. | (a) keeps today's operator output. (b) changes `show policy test` output, which operators and tests read. Either way the renderer is not a transport (FR-4). |
| Q-12 | Where does a compiled-in filter type evaluate when its plugin is configured to run forked? | (a) Always in process when the type is compiled in, as Loc-RIB route evaluators do (route-policy: "the gate never calls a plugin process"). (b) In process only when the plugin runs internal, otherwise over the slow path. | (a) one evaluation path per type in a given binary and no RPC for in-tree types, but the plugin's process placement no longer decides where its filter runs. (b) keeps placement meaningful and exercises the slow path for in-tree types, at an RPC cost. Decides AC-14 and whether the in-tree plugins are the slow path's test corpus. |

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
| A-8 | Non-CIDR families (EVPN, VPN, FlowSpec beyond its destination prefix, BGP-LS, ...) can keep today's rule: per-NLRI decisions need raw | `CIDRFamily` (`filtertext/cidr.go`); "Non-CIDR Families in the Filter Text Protocol" | the decoded NLRI field must cover family-specific encodings | design of Q-1 and Q-4 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Blast radius: `pkg/plugin/rpc`, the SDK, the reactor, nine plugins, `filtertext`, dry-run, two protocol pages, every filter test | compile and lint fanout when `rpc` changes | child specs land in dependency order; each lands green for its own packages; no compatibility window, because FR-4 forbids one |
| R-2 | A batch delays a route behind slower ones, or reorders per-source delivery | interop and `.ci` order assertions; cache ack FIFO | Q-3 fixes per-source FIFO as a requirement of any answer; a bound per batch |
| R-3 | The lazy view keeps a pointer into a reused receive buffer past the chain run | race detector; `IsAsyncSafe` false | the view is valid only inside the synchronous run; a batch retains or copies before handing off (Q-3, Q-7) |
| R-4 | A field the view decodes from a malformed attribute reads as zero and a filter accepts | RFC 7606 treat-as-withdraw tests | readers answer an error; the chain applies the filter's `on-error` (`ai/rules/principles.md`) |
| R-5 | `filter_modify` arithmetic (increment, decrement from the current value or the configured default) changes result when the current value arrives decoded rather than as text | existing `filter_modify` `.ci` and units | it declares the fields it reads; AC-10 runs every existing test unchanged |
| R-6 | Losing the text subject removes operator-readable dry-run output | `show policy test` `.ci` | Q-11 decides the display surface before the text is deleted |
| R-7 | Route-policy phase 4 and this spec edit the same seam | both open | this spec is that phase's BGP half; route-policy's phase 4 step names it, and the two close together for AC-17 |
| R-8 | `filter_irr` blocks inside its handler on first IRR resolution; in a batch it would block every route | `filter-irr-fail.ci`, `clear-irr-cache*.ci` | Q-3 and Q-12 decide whether the wait is per batch or per route |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | BGP policy decisions: routes accepted that a filter should reject (leaks), or rejected that it should accept; malformed modified UPDATEs; every third-party text-format filter plugin (expected, FR-4) |
| How is it reverted? | Per child spec commit; the text machinery's deletion is the last child, so earlier children revert alone |
| Who else touches this path? | Route-policy phase 4 (same seam); any spec editing `reactor/filter_*.go`, `forward_build.go`, `filterapi`, or the filter plugins |

## Wiring Test (MANDATORY -- NOT deferrable)

Test names are planned; their files are created by the child that owns them.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `filter { import { filters [ <compiled-in instance> ]; final accept; } }` on a peer, UPDATE received | → | in-process evaluator over the lazy view, set fields to `ModAccumulator`, `buildModifiedPayload` | `test/plugin/filter-wire-inprocess-import.ci` |
| `filter { export { filters [ <plugin>:<filter> ]; final accept; } }` with an external plugin declaring one field | → | slow-path encoder sends that field only, reply sets applied | `test/plugin/filter-wire-external-declared.ci` |
| The same with a Python 3 plugin speaking the documented contract | → | slow path end to end, no Go SDK | `test/plugin/filter-wire-python-plugin.ci` |
| `show policy test` against a chain mixing both paths | → | dry-run over the view, trace per member | `test/plugin/filter-wire-dryrun.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Depends on |
|-------|-------------------|-------------------|------------|
| AC-1 | An in-process filter instance that reads only `prefix` runs on an UPDATE carrying AS_PATH, COMMUNITY and LARGE_COMMUNITY | a decode counter on the view records the NLRI decode and zero attribute decodes; no allocation for the evaluation (`testing.AllocsPerRun` = 0) | Q-7 |
| AC-2 | An external test plugin declares it reads `prefix` only | the message Ze sends carries the prefix, no `as-path`, no community field and no raw bytes (asserted on the captured request) | Q-1, Q-2 |
| AC-3 | Two external filters in one chain, one declaring raw, one not | only the first receives the raw UPDATE | Q-4 |
| AC-4 | A filter, in process and external in turn, sets `local-preference` 200 on an import route | the cached UPDATE carries LOCAL_PREF 200, and the two paths produce identical UPDATE bytes | Q-1, Q-8 |
| AC-5 | A chain at `bgp export` names a filter that sets a field the attachment does not allow, or reads one it does not carry | config load fails naming the filter, the field and the attachment; the same for an external filter's declared fields | Q-10 |
| AC-6 | An external reply sets a field the filter did not declare | the route is treated as a failed filter under its `on-error` (fail-closed by default), and the event is logged and counted | Q-9 |
| AC-7 | An external filter answers AS-path prepend 2, remove-private-AS, and an NLRI subset dropping one IPv4 and one IPv6 prefix | the receiving peer sees the path prepended twice with the local AS, private ASNs stripped segment-preserving, and only the kept prefixes in NLRI and MP_REACH_NLRI | Q-5 |
| AC-8 | The text format is removed | `FilterUpdateInput.Update`, `FilterUpdateOutput.Update`, `AppendUpdateForFilter`, `AppendAttrsForFilter`, `parseFilterAttrs`, `applyFilterDelta`, `formatFilterAttrs`, `textDeltaToModOps`, `validateModifyDelta`, the text-driven half of `sectionNLRIDelta` and the `internal/component/bgp/filtertext` package no longer exist; the four text-protocol sections of `process-protocol.md` and the handler section of `docs/plugin-development/protocol.md` describe the new contract | FR-4 |
| AC-9 | A plugin started with the text-era contract, or an unknown contract version | Stage 1 refuses it with a message naming the version it declared, the version Ze speaks, and the page to read | Q-2 |
| AC-10 | Every existing BGP policy `.ci` (`find test -name '*.ci'` naming a `bgp-filter-*` plugin) and every interop scenario whose config names a filter chain | pass with unchanged expectations; this is route-policy AC-17 | all |
| AC-11 | A Python 3 test plugin, written against the protocol page only, implements a filter that rejects one prefix and sets `med` on the rest | the receiving peer gets only the accepted routes, each with the MED set | Q-1, Q-2, A-7 |
| AC-12 | Ten routes queued for one external filter | they go in one exchange (or as many as Q-3 bounds), each route gets its own verdict and sets, and per-source order is unchanged on the wire | Q-3 |
| AC-13 | One chain with an in-process member that sets `local-preference` and an external member that reads it | the external member is sent the value the first member set | Q-1, Q-12 |
| AC-14 | Each of the nine in-tree filter plugins | answers through the new contract or the fast path; no in-tree code calls `OnFilterUpdate` with a text handler; every one of their existing tests passes | Q-6 |
| AC-15 | An external filter times out, errors, or answers teardown on import and on export | `on-error` decides the outcome as today; teardown closes the session on import only | Q-3 |
| AC-16 | `show policy test` on a chain of both kinds | reports the same decision and final as the live path, and a per-member trace | Q-11 |
| AC-17 | The throughput harness | exists under `./le perf` for the in-process path and for the external path at several batch sizes, compiles, and prints its run command for Thomas; no agent records a perf number | FR-7 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a prefix-list import chain on a peer | config → library compile → in-process evaluator → cache | `test/plugin/filter-wire-inprocess-import.ci` |
| 2 | Writes a filter plugin in Python from the protocol page | Stage 1 declaration → slow path → reply sets → wire | `test/plugin/filter-wire-python-plugin.ci` |
| 3 | Runs `show policy test` | dry-run over the view | `test/plugin/filter-wire-dryrun.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWireViewDecodesOnDemand` | `internal/component/bgp/reactor/` (view package per Q-7) | AC-1 decode counter and zero allocations | |
| `TestWireViewMalformedAttributeErrors` | same | R-4 | |
| `TestFilterMessageDeclaredFieldsOnly` | `internal/component/bgp/reactor/` | AC-2, AC-3 | |
| `TestSetFieldToModOps` | `internal/component/bgp/reactor/` | AC-4, one case per settable field | |
| `TestUndeclaredSetFailsRoute` | `internal/component/bgp/reactor/` | AC-6 | |
| `TestTypedIntentsMatchTextEra` | `internal/component/bgp/reactor/` | AC-7, compared against today's outputs captured before deletion | |
| `TestCheckChainExternalDeclaration` | `internal/component/bgp/config/` | AC-5 | |
| `TestDeclareRegistrationRefusesTextContract` | `internal/component/plugin/server/` | AC-9 | |
| `TestFilterBatchOrderAndVerdicts` | `internal/component/bgp/reactor/` | AC-12 | |
| `TestMixedChainSetsVisible` | `internal/component/bgp/reactor/` | AC-13 | |
| per-plugin evaluator and handler tests | each plugin under `internal/component/bgp/plugins/filter_*` | AC-14 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| batch size (Q-3) | to be fixed at design | | | |
| AS-path prepend count (Q-5) | to be fixed at design, today's `filter_modify` range | | | |
| contract version (Q-2) | to be fixed at design | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-wire-inprocess-import` | `test/plugin/` | AC-1, AC-4 through the daemon | |
| `filter-wire-external-declared` | `test/plugin/` | AC-2, AC-3 | |
| `filter-wire-python-plugin` | `test/plugin/` | AC-11 | |
| `filter-wire-text-contract-refused` | `test/plugin/` | AC-9 | |
| `filter-wire-dryrun` | `test/plugin/` | AC-16 | |
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

- `pkg/plugin/rpc/types.go` - `FilterDecl`, `FilterUpdateInput`, `FilterUpdateOutput`, the contract version (Q-1, Q-2, Q-4)
- `pkg/plugin/sdk/sdk_callbacks.go`, `pkg/plugin/sdk/sdk_types.go` - the handler and its typed accessors over the new message
- `internal/component/plugin/server/server.go`, `internal/component/plugin/server/startup.go` - `CallFilterUpdate` (batch), `registrationFromRPC` filters, `FilterInfo`
- `internal/component/bgp/reactor/filter_chain.go`, `filter_ordered.go`, `filter_delta.go`, `filter_nlri_delta.go`, `filter_format.go`, `policy_dryrun.go`, `peer_initial_sync.go` - chain run over the view and the new message; text deleted
- `internal/component/bgp/reactor/forward_build.go`, `filter_delta_handlers.go` - only if a set field needs a handler (A-2)
- `internal/component/bgp/filterapi/policy.go`, `filterapi.go` - the compiled chain inside `Policy` (Q-6, Q-12)
- `internal/component/bgp/config/register.go`, `peers.go` - attachment field lists (Q-8), external declarations in `validateChainNames` (Q-10)
- `internal/component/policy/fields.go`, `register.go` - the BGP view's field readers (Q-7) and any field Q-8 adds
- `internal/component/bgp/plugins/filter_aspath/`, `filter_aspath_length/`, `filter_community_match/`, `filter_path_asn/`, `filter_family/`, `filter_prefix/`, `filter_irr/`, `filter_modify/`, `filter_remove_private_as/` - migration (Q-6)
- `internal/component/bgp/plugins/cmd/policy/handler.go` - dry-run output (Q-11)
- `internal/test/fixture/plugin_fixture_filter_subject.go`, `plugin_fixture_filter_origin.go`, `plugin_fixture_12_filters.go`, `plugin_fixture_09_bgp.go` - fixtures that implement `OnFilterUpdate` today
- `internal/le/interoplab/bgp/helper.go`, `helper_srv6_identity.go` - interop helpers that implement `OnFilterUpdate` today
- `internal/le/perf/` - the throughput harness (FR-7)
- `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`, `docs/plugin-development/README.md`, `docs/architecture/core-design.md` ("Ingress Filter Pipeline", "Policy Filter Chain"), `docs/guide/bgp-policy.md`, `docs/architecture/api/architecture.md`, `ai/rules/plugins.md`

## Files to Create

- Child specs under `plan/post-release/engineering/`, written at the design gate, one per implementation phase below, with stems `spec-filter-wire-1-contract`, `spec-filter-wire-2-view`, `spec-filter-wire-3-chain`, `spec-filter-wire-4-plugins`, `spec-filter-wire-5-delete` (names provisional until `/ze-spec` fixes the split)
- The lazy view's package, placed by Q-7
- `test/plugin/filter-wire-*.ci` - the functional tests above
- The Python 3 test plugin the functional runner forks, placed beside the other test plugins the runner starts (A-7)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-plugin-callback.yang` `filter-update` input and output change (`internal/core/ipc/yang/`) |
| YANG validation constraints | No | no config leaf added unless Q-3 makes batch size configurable |
| YANG custom validators | No | the field check is `policy.CheckChain` |
| CLI commands/flags | Yes | `show policy test` output (Q-11) |
| CLI grammar (keyword before value) | No | no new command |
| Editor autocomplete | No | filter names unchanged |
| Functional test for new RPC/API | Yes | `test/plugin/filter-wire-*.ci` |
| Pipe completeness | No | `show policy test` already pipes |
| Env var registration | No | none planned |
| Doctor check for runtime dependencies | No | no new runtime dependency |
| Prometheus counters/metrics | Yes | a counter for AC-6 refusals and a batch size histogram if Q-3 needs one; names fixed at design |
| BGP family surface (new SAFI / capability / attribute) | No | no family, capability or attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the BGP policy `features/` entry: external filters in any language |
| 2 | Config syntax changed? | No | chains unchanged |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` if `show policy test` output changes (Q-11) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` if it lists the callback |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (filter plugins) |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-policy.md` |
| 7 | Wire format changed? | No | BGP wire unchanged |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`, `ai/rules/plugins.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | behavior preserved (AC-10) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the runner gains a Python plugin |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: filter plugins in any language |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | plugin set unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived per child by `./le spec citation anchors spec <child spec>`. Pages declared by `// Design:` headers of files listed here, each judged per child: `docs/architecture/api/ipc_protocol.md` (`pkg/plugin/rpc/types.go`: answer framing, affected if batching changes the message size), `docs/architecture/config/syntax.md` (`bgp/config/register.go`, `peers.go`: unaffected unless Q-3 adds a config leaf), `docs/architecture/testing/interop.md` (`internal/le/interoplab/bgp/helper.go`, `helper_srv6_identity.go`: unaffected beyond the helpers' filter handler) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `OnFilterUpdate` and text-subject example in `docs/` |

## Implementation Steps

Each phase is one child spec; each follows TDD and ends with a self-critical
review. Order is by dependency. Wiring comes first.

1. **Phase 1: Contract and wiring** (`spec-filter-wire-1-contract`) - the versioned message, Stage 1 negotiation and refusal, the external field declaration; the four wiring `.ci` written failing; the Python runner probe (A-7). Answers built on Q-1, Q-2, Q-4, Q-10.
2. **Phase 2: Lazy view** (`spec-filter-wire-2-view`) - the view, per-field readers registered, decode counting, zero-allocation tests; Q-7, Q-8.
3. **Phase 3: BGP chain on the library** (`spec-filter-wire-3-chain`) - BGP chains compile through the library at `bgp import`/`bgp export`; in-process and external members in one chain; set fields and typed intents to `ModAccumulator`; batching; dry-run; Q-3, Q-5, Q-9, Q-11, Q-12. Closes route-policy AC-17 with phase 4.
4. **Phase 4: Plugin migration** (`spec-filter-wire-4-plugins`) - the nine plugins, fixtures and interop helpers; Q-6.
5. **Phase 5: Delete the text machinery** (`spec-filter-wire-5-delete`) - AC-8; the protocol pages rewritten in the same commit; every interop scenario in the table run with recorded reds.
6. **Throughput harness** (inside phase 3 or its own child, decided at design) - FR-7 and AC-17; Thomas runs it on his Mac.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the two paths produce identical UPDATE bytes for the same sets (AC-4) |
| Naming | JSON keys kebab-case; field names are the registered vocabulary names, never a second list |
| Data flow | a set field reaches the wire only through `buildModifiedPayload`; the reactor names no plugin |
| Rule: no-layering | no text field, text parser or compatibility window left (AC-8) |
| Rule: performance | the view reads borrowed bytes in place; no `fmt.Sprintf` on the chain path |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| text format gone | `grep -rn "AppendUpdateForFilter\|applyFilterDelta\|parseFilterAttrs\|filtertext" internal pkg` returns nothing |
| declared fields only | `TestFilterMessageDeclaredFieldsOnly` |
| non-Go plugin | `test/plugin/filter-wire-python-plugin.ci` |
| behavior preserved | AC-10 runs |

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

- External `<plugin>:<filter>` references are the one chain member the load-time check cannot see today; FR-2's rule that the declared set is both what is sent and what is checked closes that gap by construction (Q-10).
- The text subject was never narrowed by `FilterDecl.Attributes` (`AppendUpdateForFilter` is always called with a nil list); parse on demand is a new guarantee, not a restored one.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

No open journal row names this spec at 544f8f930c.

## Key Design Decisions

Decided only. Open choices are in `## Open Design Questions`.

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| This spec is route-policy phase 4's BGP half, on the shared library (Disposition, 2026-10-31) | a BGP-only mod-op and intent vocabulary in the SDK (the July design) | a second filter model beside the library would be undone by phase 4 |
| Two paths, one vocabulary (owner, 2026-10-31, FR-1) | in-process only; external only | plugins in any language are part of Ze's design, and compiled-in types need no RPC |
| Parse on demand on both paths (owner, 2026-10-31, FR-2) | send every attribute, as the text subject does | Ze decodes and serialises nothing a filter did not ask for |
| The in-process fast path is a lazy view over `WireUpdate` (owner approved, 2026-10-31) | evaluate over a decoded struct | lazy over eager; zero copy |
| The contract is versioned, documented in the protocol pages, and the Go SDK is a convenience over it (owner, 2026-10-31, FR-3) | the SDK as the contract | a non-Go author reads the page, not Go types |
| Replace the text format; third-party text plugins break (owner, 2026-10-31, FR-4) | keep text beside the new message during migration | no layering; pre-release |
| Batching is the slow path's performance lever (owner, 2026-10-31, FR-6) | a typed DirectBridge slot | batching helps forked plugins, which a bridge never reaches |
| The benchmark is a harness Thomas runs on his Mac (owner rule, `ai/META-DECISIONS.md`) | an agent-run benchmark gate (the July Phase 1) | perf is measured only on known hardware |
| BGP import stays the per-peer ingress chain at `FilterStagePeerChain`, and no new text representation is added (route-policy Key Design Decisions and R-6) | move BGP import to the Loc-RIB gate | BGP import must see communities and AS path before best path |
| Child specs live under `plan/post-release/engineering/` (`ai/META-DECISIONS.md`) | the `plan/` root (the July design) | plans live only in subfolders of `plan/` |

## Known Limitations

- Third-party filter plugins written against the text `Update` format stop working when this lands (FR-4, owner-accepted). The refusal at startup tells their authors which contract to speak (AC-9).
- Storage of BGP routes rejected by import stays in `plan/post-release/features/spec-bgp-filtered-route-storage.md`.

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
- [ ] AC-1..AC-17 all demonstrated (across children)
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
FR-5 of this spec, proven by AC-8 and AC-14; it now covers nine plugins, not
seven.

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
