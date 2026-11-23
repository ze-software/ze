# Spec: explicit-plugin-and-attach-config

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Priority | P1 |
| Release reason | Configuration correctness and identity integrity of shipped behavior: replaces the ExaBGP-derived `plugin { }` and per-peer `attach process` grammar with the protocol-agnostic `bus { endpoint, flow, hub }` model, converts all repository configs/tests/docs cleanly with no hybrid layering, limits `ze start --plugin` to built-in plugins, and fixes aliased plugin state-key and doctor snapshot keying |
| RFC gaps | - |
| Depends | - |
| Phase | Design approved (2026-11-23); ready for implementation |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Bucket: `plan/release/fixes/`. Child 1 of 3 (split per R-8 on 2026-11-23):
- **Child 1 (this spec):** `bus { endpoint, flow, hub }` grammar cutover (pure bus, zero `flow` in `bgp {}`), generic message tags on `bus { flow }`, built-in-only `ze start --plugin`, `ze exabgp migrate` and `create bgp peer` cutover, parsed-tree conversion of all repository configs/tests/docs, and the owned alias-keying journal row plus 5 sibling alias-keying defects.
- **Child 2:** `plan/release/fixes/spec-bus-2-implied-display-and-grant-refusals.md` — `| implied` display (CLI, editor, Web Inspection & Workbench, API), generic plugin-declared implied flows & narrow-grant refusals, AIGP explicit trigger, and D-6 intrinsic flows on `show event delivery`.
- **Child 3 (P0):** `plan/release/fixes/spec-bus-3-external-command-gating.md` — D-8 external command gating (short subsystem tokens + `from cli` prefixes) closing the live `ReservedInternalPrefix` AAA bypass on `dispatch-command`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Replace `plugin { internal ...; external ...; hub ...; }` and BGP's `attach process <name> { ... }` (including inline `run`/`use` and the dead `content` container) with one protocol-agnostic `bus { endpoint ...; flow ...; hub ...; }` configuration block, keeping `bgp { }` pure BGP and fixing aliased plugin identity bugs across state and doctor surfaces.

### Owner rulings governing this spec (2026-11-16 through 2026-11-23)

| # | Decision | Status in Child 1 |
|---|----------|-------------------|
| D-4 | The config states an `endpoint` (who is on the bus) apart from a `flow` (what moves between two parties). `attach process` is replaced whole, its inline `run`/`use` and dead `content` block included. | Landed here |
| D-5 (reversed 2026-11-23) | **Pure protocol-agnostic bus + pure BGP.** Zero `flow [ ... ]` inside `bgp { }` and zero BGP-specific keywords (`peer`, `group`) inside `bus { }`. Built-in plugins subscribe to generic bus events in their own `register()` (when a plugin is absent, nothing listens). A `bus { flow }` applies to all events from `from` by default and supports optional protocol-agnostic message-tag filtering (for example filtering `best-change` on a secondary tag such as `source bgp`, or filtering BGP events on publisher-emitted tags like `group` or `peer`). | Landed here (reverses 2026-11-17 D-5) |
| D-7 / Q3 | `plugin { }` becomes `bus { }`, and `hub { server ...; client ...; }` moves under `bus { hub { ... } }`, renaming the YANG-discovered listener service from `"plugin-hub"` to `"bus-hub"`. Repository configs are converted directly (`ai/META-DECISIONS.md`, "Ze is pre-release"). | Landed here |
| Q1 (W-1, Option B) | Compiled-in plugins declare owned event namespaces and namespaced command types on `registry.Registration`; domain/core namespaces (`bgp`, `interface`, `subscriber`, `system-rib`, `config`, `mpls-fib`, `system-dns`, `redistribute`, `link-state`) register as built-in bus publishers. `events.ValidSendTypes` becomes namespaced per publisher. `flow { from ... }` accepts a built-in publisher or an aliased built-in endpoint, with an optional `namespace <ns>;` leaf when disambiguating a multi-namespace publisher (such as `l2tp` emitting `l2tp` and `subscriber`). | Landed here |
| Q2 (W-8 & W-4) | (1) `ze start --plugin` accepts **built-in plugins only** (refusing external program paths with an error pointing to `bus { endpoint <name> { external { run ...; } } }`; offline `ze bgp decode --plugin` and `ze plugin test --plugin` keep external decoders). (2) `bus { flow }` allows **built-in publishers only in `from`** (external endpoints appear in `to` only, eliminating W-4/A-2 and Stage 1 wire changes). | Landed here (narrows 2026-11-18 `yang-config-design.md` `--plugin` note) |
| D-6, D-10, Q4, Q5 | `\| implied` display, generic plugin-declared implied flows & narrow-grant refusals, AIGP explicit trigger, and D-6 intrinsic flows on `show event delivery`. | Homed in `plan/release/fixes/spec-bus-2-implied-display-and-grant-refusals.md` |
| D-8, Q6 | Full external command gating (short subsystem tokens + `from cli` prefixes) closing the `ReservedInternalPrefix` AAA bypass. | Homed in `plan/release/fixes/spec-bus-3-external-command-gating.md` |
| Owned journal row (2026-11-14) | Key compiled-in state-key grants (`statestore.PluginKeyReadable`/`Writable`/`PluginList`), doctor snapshots (`PluginDoctorState`), and live-owner guards (`runningOwnerError`) on the compiled registry name (guarded by `!proc.Config().RunsExternalProgram()`), and fix the 5 sibling alias-vs-registry-name bugs found in `bgp/server/events.go`, `plugin/server/startup.go`, and `plugin/server/startup_autoload.go`. | Landed here |

### Target `bus { }` configuration model

| Concept | What it states | Where it is written |
|---------|----------------|---------------------|
| Endpoint | One party on the bus under an operator-chosen name: an aliased built-in plugin run in the daemon, or an external program Ze starts | `bus { endpoint <name> { ... } }` |
| Flow | One named relationship from a built-in publisher to an endpoint: events delivered `from -> to`, commands `to` may send back to `from`, and optional generic message-tag filters | `bus { flow <name> { ... } }` |
| Hub | External plugin TLS hub listener (`server`) and managed-config `client` blocks moved from `plugin { hub }` | `bus { hub { server ...; client ...; } }` |

Fields of `bus { endpoint <name> { ... } }`:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string (key), `ze:validate "external-plugin-name"` | Endpoint name referenced by flows and diagnostics |
| `internal` | string (`ze:validate "internal-plugin-name"`) | Registry name of a built-in plugin to run in the daemon; mutually exclusive with `external` |
| `external` | container (`run`, `respawn`, `timeout`) | External program Ze forks; mutually exclusive with `internal` |
| `encoding` | enumeration `json` \| `text` (default `json`) | Wire encoding of events delivered to this endpoint (replaces `encoder`; per-attachment `content` is deleted as dead) |

Fields of `bus { flow <name> { ... } }`:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string (key) | Unique flow name |
| `from` | string | Built-in publisher (registered namespace/subsystem name or aliased built-in `endpoint` name). External endpoints are refused in `from` |
| `namespace` | string (optional) | Disambiguates the event namespace when `from` owns multiple namespaces (e.g. `l2tp` vs `subscriber`); refused if `from` does not own it |
| `to` | string | Receiving endpoint: either a declared `bus { endpoint }` name or a compiled-in plugin registry name |
| `event` | leaf-list of string | Event types delivered from `from` to `to`, validated against the resolved publisher/namespace's registered events (direction suffixes and `*` preserved) |
| `command` | leaf-list of string | Command capability tokens `to` may send back to `from`, validated against the resolved publisher's registered command types (and `*`) |
| `match` | list keyed by `tag` (`tag` string, `value` leaf-list of string) | Optional protocol-agnostic message-tag filter (omit = match all messages from `from`). A message matches when every listed `tag` matches one of its `value` entries (e.g. `match source { value [ bgp ]; }`, `match group { value [ customers ]; }`, `match peer { value [ 198.51.100.1 ]; }`) |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform boundaries, process boundary, registration over enumeration
  -> Constraint: `internal/component/plugin/` never imports `internal/component/bgp/`; `bus` carries zero BGP-specific concepts
- [ ] `ai/doors/config-yang.md` - YANG schema loading, first-pass roots, `validatedSections`, `ze:validate`, config text manipulation rules
  -> Constraint: config text changes go through a parsed `*config.Tree` (never regex text surgery); `validatedSections` timing trap for `bus` must be solved by `init()`-time event/command registration
- [ ] `ai/doors/plugin-infra.md` - `plugin/registry`, `plugin/server`, `plugin/process`, `HoldDeclaredPlugins`, Stage 1 checks
  -> Constraint: external programs are never spawned offline during `ze config validate`; compiled-in vs external distinction uses `!proc.Config().RunsExternalProgram()`
- [ ] `ai/doors/bgp-engine.md` - BGP reactor, `PeerSettings`, `DeliveryGraph` publication, `send_permission.go`
  -> Constraint: BGP reactor stays isolated; BGP emits standard `group`/`peer` tags onto its bus events generically without teaching `bus` about BGP
- [ ] `ai/doors/core.md` - `internal/core/events`, `internal/core/statestore`
  -> Constraint: `internal/core/` never imports upward into `internal/component/`
- [ ] `ai/doors/hub-engine.md` - `cmd/ze/hub` startup, `FirstPass`, managed server/client, listener discovery
  -> Constraint: `firstPassRoots` and `PluginOnlySchema` must switch from `plugin` to `bus` before full YANG parse
- [ ] `ai/doors/system-plugins.md` - `internal/exabgp/migration`, `internal/exabgp/bridge`, `internal/plugins/ddos/flowspec`
  -> Constraint: ExaBGP migration and `bridge_neighbor.go` (`create bgp peer`) must cut over in the same work
- [ ] `docs/architecture/api/architecture.md` - "Attaching a Process to a Peer", "Encoding", "Owner ruling: endpoints and flows on the bus"
  -> Constraint: departs from old `attach process` syntax and 2026-11-17 D-5 (`flow [ ... ]` in `bgp {}`); update page in the same work
- [ ] `docs/architecture/config/yang-config-design.md` - "Where a validator actually runs", `## Design decisions` (2026-11-18 `--plugin` row)
  -> Constraint: departs from 2026-11-18 note allowing arbitrary external `--plugin` on `ze start`; update page in the same work
- [ ] `docs/architecture/api/process-protocol.md` - event namespaces, persistent runtime state, plugin auto-loading
  -> Constraint: Stage 1 wire format stays unchanged; state-key grants remain keyed on compiled registry names

**Key insights:**
- `internal/component/config/validate_sections.go` includes `sectionPlugin` in `validatedSections`, while `bgp` was excluded. Moving `event`/`command` under `bus` causes `ValidateCustomSections` to run inside `LoadConfig` *before* `PeersFromTree`/`NewServer` call `RegisterPluginSendTypes` — so event namespaces and command types MUST register at `init()` (or before `LoadConfig`).
- `ProcessBinding.Encoding` and `ProcessBinding.Format` (`attach > content`) are never read on any BGP delivery path (`delivery_graph.go` drops them; `bgp/server/events.go` reads only `proc.Encoding()`/`Format()`), and cause spurious reload peer bounces via `reflect.DeepEqual`. Deleting `content` loses zero behavior.
- `internal/exabgp/bridge/bridge_neighbor.go` (`convertNeighborCreate`) emits `create bgp peer ... attach exabgp-bridge` into `internal/component/bgp/plugins/cmd/peer/create.go` (`setAttach`), which writes directly into `attach.process`; both must cut over alongside `attach process` removal.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/yang/ze-plugin-conf.yang` - declares `container plugin` with `list internal`, `list external`, `container hub` (`server`, `client`)
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - declares `container attach` (`list process` with `receive`, `send`, inline `run`/`use`, dead `content`)
- [ ] `internal/component/config/first_pass.go`, `yang_schema.go`, `loader.go`, `loader_extract.go` - parse `plugin` in first pass, extract `internal`/`external`/`hub` via `tree.GetContainer("plugin")`, merge CLI `--plugin` via `MergeCliPlugins`
- [ ] `internal/component/config/validate_sections.go`, `validate_semantic.go`, `validators.go`, `retired.go`, `listener.go`, `graph.go`, `cli/cmd_validate.go` - validate `plugin`, discover `"plugin-hub"` listener, map `RetiredKeywordHint("process")` to `"attach process"`, gate hub validation on `tree.GetContainer("plugin") != nil`
- [ ] `internal/component/bgp/config/plugins.go`, `peers.go`, `redistribute_binding.go`, `flowspec_binding.go`, `filter_state_binding.go`, `dissemination.go`, `dissemination_config.go`, `resolve.go` - extract inline BGP plugins, resolve per-peer `attach process`, print `attach process` in refusals
- [ ] `internal/component/bgp/reactor/config.go`, `delivery_graph.go`, `send_permission.go` - `parseProcessBindingsFromTree`, `deliveryPeersFromSettings`, `Peer.maySend` / `filterPermittedPeers`
- [ ] `internal/component/plugin/server/delivery_graph.go`, `delivery_reconcile.go`, `dissemination_validate.go`, `startup_autoload.go`, `startup.go` - build BGP-only `DeliveryGraph`, print `attach process` in `DeliveryDisagreement.Fix()` and `disseminationAttachLine`, extract custom event/send auto-load types from `PeerSettings.ProcessBindings`
- [ ] `internal/component/plugin/server/dispatch_state.go`, `doctor_snapshot.go`, `internal/core/statestore/permissions.go`, `internal/plugins/ddos/flowspec/doctor.go`, `internal/component/config/storage/cli/data_rpc.go`, `internal/component/bgp/server/events.go` - key state grants (`PluginKeyReadable`/`Writable`/`PluginList`), `PluginDoctorState`, `runningOwnerError`, `RequiresPeerUpBarrier`, `configRefusalIsFatal`, `collectOrphanedDependencies`, and `appendHeldNeedProviders` on operator alias `proc.Name()` instead of compiled registry name
- [ ] `internal/exabgp/migration/migrate.go`, `migrate_serialize.go`, `internal/exabgp/bridge/bridge_neighbor.go`, `internal/component/bgp/plugins/cmd/peer/create.go` - emit `plugin { internal ... }`, `attach process`, and runtime `create bgp peer ... attach`
- [ ] `internal/core/events/events.go`, `internal/component/plugin/resolve.go`, `internal/component/plugin/registry/registry.go` - register 15+ event namespaces and flat global `ValidSendTypes`; no `EventNamespaces` field on `registry.Registration`; `ResolvePlugin`/`deriveName` allows arbitrary external paths on `--plugin`

**Behavior to preserve:**
- Every compiled-in and external plugin continues running internal or external (TLS connect-back) over the existing Stage 1–5 RPC protocol with zero wire breakage.
- Silence stays the default for external programs: an external endpoint with no matching `bus { flow }` receives no events and cannot send BGP updates.
- Built-in derived/implied behaviors (`wireRedistributeDelivery`, `requireRIBDelivery`, `grantFilterOwnersState`, `startup_autoload.go`) keep working identically during Child 1 (projected into the bus flow model; Child 2 replaces their internals with generic plugin declarations and `| implied`).
- All 196 existing `ze --plugin ze.<name> -` built-in invocations and offline `ze bgp decode --plugin` / `ze plugin test --plugin` continue working.
- Existing multi-peer routing semantics (including route server, route reflector, ordinary transit, and per-peer ExaBGP `feed <ip>` isolation) continue passing all functional and interop tests.

**Behavior to change:**
- Delete `container plugin` from `ze-plugin-conf.yang` and `container attach` from `ze-bgp-conf.yang` (no layering); replace with `container bus { list endpoint; list flow; container hub; }`.
- Delete `extractBGPInlinePlugins`, `attach > content`, and `create bgp peer ... attach` (replaced by bus flow / runtime binding without `attach process`).
- Add `EventNamespaces` / namespaced command type registration on `registry.Registration` and `internal/core/events`; validate `bus { flow }` `from`, optional `namespace`, `to`, `event`, `command`, and `match` tags at `LoadConfig` / `ze config validate`.
- Support protocol-agnostic message tags on `DeliveryGraph` / bus delivery (BGP attaches `peer` and `group` tags; `sysrib` / route events attach `source` tag; `bus` itself carries zero BGP-specific code).
- Restrict `ze start --plugin` to compiled-in built-in plugins only; refuse external program paths with a fix message pointing to `bus { endpoint <name> { external { run ...; } } }`.
- Move `hub` under `bus` (`bus-hub` listener service) and update `firstPassRoots`, `PluginOnlySchema`, `ExtractHubConfig`, `cmd_validate.go`, `graph.go`, managed/doctor checks, and `RetiredKeywordHint` (`plugin`, `attach`, `process`).
- Fix the owned journal row (`dispatch_state.go`, `doctor_snapshot.go`, `ddos/flowspec/doctor.go`, `data_rpc.go`) and all 5 sibling alias-keying defects (`RequiresPeerUpBarrier`, `configRefusalIsFatal`, `isPluginLoaded`, `collectOrphanedDependencies`, `appendHeldNeedProviders`), guarded by `!proc.Config().RunsExternalProgram()`.
- Convert every repository `.conf`, `.ci`, `.et`, interop scenario, fixture, Go test, doc, and website example (~1,752 files) via a parsed-tree converter in the same work.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Program & hub declaration | `internal/component/plugin/yang/ze-plugin-conf.yang` (`container plugin`); `internal/component/config/loader.go` (`ExtractPluginsFromTree`); `loader_extract.go` (`ExtractHubConfig`) | Declares `internal`, `external`, and `hub` under `plugin { }` | Replaces (D-4, D-7, Q3): unified into `bus { endpoint; flow; hub; }` with one `endpoint` list and `hub` moved to `bus-hub` |
| Per-peer attachment | `internal/component/bgp/yang/ze-bgp-conf.yang` (`container attach`); `internal/component/bgp/reactor/config.go` (`parseProcessBindingsFromTree`); `bgp/config/plugins.go` (`extractBGPInlinePlugins`) | Per-peer/group `attach process` with inline `run`/`use`, `receive`, `send`, and dead `content` | Replaces (D-4, D-5 reversal): deleted from `bgp { }`; replaced by `bus { flow }` with optional generic `match` tags |
| First-pass config extraction | `internal/component/config/first_pass.go` (`firstPassRoots`); `yang_schema.go` (`PluginOnlySchema`) | Parses `plugin` and `environment` before external Stage 1 YANG publish | Extends: switches `plugin` root to `bus` so external endpoints and `bus { hub }` still parse in Pass 1 |
| Event & send type registries | `internal/core/events/events.go` (`RegisterNamespace`, `RegisterSendType`, `ValidEvents`, `ValidSendTypes`) | Registers 15+ event namespaces and flat global `ValidSendTypes` | Extends (Q1): adds namespaced command-type registration and publisher-to-namespace ownership populated at `init()` so `validatedSections` runs cleanly inside `LoadConfig` |
| Delivery graph & send permission | `internal/component/plugin/server/delivery_graph.go` (`DeliveryGraph`); `internal/component/bgp/reactor/send_permission.go` (`Peer.maySend`, `filterPermittedPeers`) | Computes BGP peer receive/send grants from `PeerSettings.ProcessBindings` | Extends: populates delivery and send permissions from `bus { flow }` (including generic `match` tag evaluation) and updates refusal text to name `bus { flow }` |
| Process alias resolution | `internal/component/plugin/process/process.go` (`RegistrationName`); `internal/component/plugin/resolve.go` (`RegistryName`, `RegistryNames`) | Already separates configured alias (`Name()`) from compiled registry name (`RegistrationName()`) | Extends: uses `RegistrationName()` (guarded by `!RunsExternalProgram()`) in `dispatch_state.go`, `doctor_snapshot.go`, `ddos/flowspec/doctor.go`, `data_rpc.go`, and the 5 sibling alias-keying sites |
| Retired keyword hints | `internal/component/config/retired.go` (`retiredKeywords`, `RetiredKeywordHint`) | Maps retired config keywords (`process` -> `attach process`) to migration hints without accepting old syntax | Extends: updates `plugin`, `attach`, and `process` entries to point to `bus { endpoint; flow; hub; }` (`no-layering.md` compliant) |
| CLI `--plugin` flag | `internal/component/config/loader.go` (`MergeCliPlugins`); `internal/component/plugin/resolve.go` (`ResolvePlugin`) | Loads internal or arbitrary external programs via `ze start --plugin` | Extends (Q2): restricts `ze start --plugin` to built-in compiled plugins only, refusing external paths with the `bus { endpoint }` fix |
| ExaBGP migration & bridge | `internal/exabgp/migration/migrate.go`, `migrate_serialize.go`; `internal/exabgp/bridge/bridge_neighbor.go`; `internal/component/bgp/plugins/cmd/peer/create.go` | Emits `plugin { }`, `attach process`, and runtime `create bgp peer ... attach` | Extends: emits deduplicated `bus { endpoint; flow; }` (no `attach process` in `bgp {}`) and replaces `create bgp peer ... attach` cleanly |
| Config tree parser & corpus validator | `internal/component/config/serialize.go`, `serialize_set.go`; `internal/component/config/cli/validate_corpus_test.go` | Parses/serializes `*config.Tree` and validates every tracked `.conf`, `.ci`, `.tape`, `.wb` | Extends: drives a one-shot parsed-tree converter across all ~1,752 files (preserving `$PORT` placeholders and skipping secret re-encoding) |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "A process Ze forks from `plugin { external <name> { run ...; } }`, or one built on `pkg/plugin/sdk`" | departs | Replaces `plugin { external <name> { run ...; } }` with `bus { endpoint <name> { external { run ...; } } }`; door updated in the same work |
| `ai/doors/plugin-infra.md` | Who depends on it from outside | covering door for `internal/component/plugin/` | "`plugin { external <name> { run ...; } }` (`internal/component/plugin/yang/ze-plugin-conf.yang`)" | departs | Replaces `plugin { external }` with `bus { endpoint <name> { external { run ...; } } }`; door updated in the same work |
| `ai/doors/bgp-engine.md` | Who depends on it from outside | covering door for `internal/component/bgp/` | "Which process sees which peer is the peer's `attach process` block" | departs | Removes `attach process` from `bgp { }`; bus delivery and optional generic tag matching move to `bus { flow }`; door updated in the same work |
| `ai/doors/bgp-plugins.md` | Who depends on it from outside | covering door for `internal/component/bgp/plugins/cmd/peer/` | "and the alias form `internal rs { use bgp-rs }`" | departs | Replaces `internal rs { use bgp-rs }` with `bus { endpoint rs { internal bgp-rs; } }`; door updated in the same work |
| `ai/doors/core.md` | Boundaries | covering door for `internal/core/events/` | "Never import upward." | agrees | `internal/core/events` stays leaf-only; components register namespaces and command types into core from `init()` |
| `ai/doors/config-yang.md` | Boundaries | covering door for `internal/component/config/` | "Config text changes through a parsed tree or `set` lines only." | agrees | Repository-wide conversion of ~1,752 files uses a parsed `*config.Tree` converter rather than regex text surgery |
| `ai/doors/system-plugins.md` | Boundaries | covering door for `internal/exabgp/` and `internal/plugins/ddos/flowspec/` | "A plugin owns its whole surface." | agrees | `exabgp` migration/bridge and `ddos-flowspec` doctor stay inside their own packages and register via existing seams |
| `ai/doors/hub-engine.md` | Boundaries | covering door for `cmd/ze/` and `internal/component/managed/` | "The hub and the Engine carry no BGP knowledge." | agrees | Pure protocol-agnostic `bus { }` keeps hub, engine, and `bus` completely free of BGP concepts |
| `ai/doors/le-tooling.md` | Boundaries | covering door for `test/` `.ci` files | "Never import `internal/le` from product code" | agrees | Product config/plugin packages import nothing from `internal/le`; new tests live under `test/` |
| `docs/architecture/api/architecture.md` | Owner ruling: endpoints and flows on the bus | `// Design:` page of `plugin/registry`, `bgp/config`, `bgp/reactor`, `plugin/server`; modified page | "A BGP group or peer takes part in a flow by listing its name, and the list merges through groups as every list does." | departs | Reversed by Thomas on 2026-11-23: zero `flow [ ... ]` in `bgp { }` and zero BGP concepts in `bus { }`; `bus { flow }` applies to all events from `from` by default with optional generic `match` tag filters; page updated in the same work |
| `docs/architecture/config/syntax.md` | The first pass: plugin declarations before the full parse | `// Design:` page of `config/first_pass.go`, `yang_schema.go`, `retired.go`, `listener.go`, `graph.go`, `bgp/config/`; modified page | "`PluginOnlySchema` holds the roots `plugin` and `environment` and no other." | departs | `PluginOnlySchema` and `firstPassRoots` switch from `plugin` to `bus` (`bus { endpoint; flow; hub; }`), and `RetiredKeywordHint` points `plugin`, `attach`, and `process` to `bus`; page updated in the same work |
| `docs/architecture/config/yang-config-design.md` | Decided, not built: modules compiled at build time | `// Design:` page of `validate_sections.go`, `validators.go`; modified page | "The `--plugin` flag keeps starting the plugin it names" | departs | Narrowed by Thomas on 2026-11-23: `ze start --plugin` accepts compiled-in built-in plugins only and refuses external paths (pointing to `bus { endpoint }`); page updated in the same work |
| `docs/architecture/core-design.md` | 1. System Architecture | `// Design:` page of `bgp/reactor/`, `bgp/server/events.go`, `exabgp/migration/`; modified page | "Phase 2: explicit plugins (from config `plugin { }` block)." | departs | Phase 2 reads explicit endpoints from `bus { endpoint ... }` instead of `plugin { }`; page updated in the same work |
| `docs/architecture/core-design.md` | Design decisions | decision row on `docs/architecture/core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel while the interfaces run on VPP." | agrees | Bus config cutover does not alter VPP/kernel feature commit checks |
| `docs/architecture/api/process-protocol.md` | Event Delivery | `// Design:` page of `core/events/events.go`, `plugin/resolve.go`, `startup_autoload.go`, `startup.go`, `dispatch_state.go`; modified page | "the plugin subscribed to the type in that direction, and the peer's `attach process <name>` block grants it." | departs | Replaces `attach process <name>` with `bus { flow }` (and optional generic `match` tags) and corrects stale prose about Stage 1 vs config load order; page updated in the same work |
| `docs/architecture/api/commands.md` | Event Subscription Commands | `// Design:` page of `cmd/peer/create.go`, `plugin/server/startup.go`, `managed/cmd_fleet.go`; modified page | "the process subscribed to the event, and the peer's `attach process <name>` block grants that type in that direction." | departs | Replaces `attach process <name>` with `bus { flow }`; page updated in the same work |
| `docs/architecture/api/commands.md` | Design decisions | decision row on `docs/architecture/api/commands.md` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Child 1 adds no command arguments that belong in pipes (`create bgp peer` removes `attach`) |
| `docs/architecture/api/commands.md` | Design decisions | decision row on `docs/architecture/api/commands.md` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Child 1 adds no pipe operations or overrides (`\| implied` is in Child 2) |
| `docs/architecture/fleet-config.md` | Unified Named Blocks | `// Design:` page of `check_managed_listener.go`, `managed/client.go`, `cmd_fleet.go`, `managed_owner.go`; modified page | "All hub configuration uses explicit `server` and `client` keywords under `plugin { hub { } }`:" | departs | Moves `hub { server; client; }` under `bus { hub { } }` (renaming listener `"plugin-hub"` to `"bus-hub"`); page updated in the same work |
| `docs/architecture/hub-architecture.md` | Config File | `// Design:` page of `data_rpc.go`, `cmd/ze/hub/main.go`; modified page | "external acme {" | departs | Replaces top-level `plugin { external acme { ... } }` config layout with `bus { endpoint acme { external { ... } } }`; page updated in the same work |
| `docs/architecture/system-architecture.md` | Overview | `// Design:` page of `cmd/ze/ze_core_start.go`; modified page | "`plugin { external ... }` block" | departs | Hub mode external plugins are declared under `bus { endpoint <name> { external { ... } } }`; page updated in the same work |
| `docs/architecture/rib-transition.md` | Owner ruling: a learned route needs the recipient's export policy | anchored to `ze-bgp-conf.yang`; modified page | "The `bgp-rib` plugin is fed a peer's routes, and may send to a peer, only where that peer's `attach process` block grants it, as for any process" | departs | Removes per-peer `attach process`; `bgp-rib` receives/sends through `bus { flow }` (and export policy); page updated in the same work |
| `docs/architecture/bgp/structural-forwarding.md` | Receive validation and cache changes | anchored to `plugin/registry/registry.go`; modified page | "the source's `attach process <route server> { receive [ update-received ] }`" | departs | Replaces `attach process` references with `bus { flow }`; page updated in the same work |
| `docs/architecture/api/capability-contract.md` | Config Validation (✅ DONE) | anchored to `bgp/config/peers.go`; modified page | "configured: attach process logger, attach process monitor - none carry either word" | departs | Updates `validatePeerProcessCaps` refusal text and examples from `attach process` to `bus { flow }`; page updated in the same work |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `plugin/registry/registry.go`, `resolve.go`, `startup_autoload.go`, `startup.go`; modified page | "The fast path then relays a source only when the source's attach block feeds this plugin its UPDATEs (`receive [ update-received ]`), and reaches a destination only when the destination grants this plugin `send [ update ]`" | departs | Updates `RelaysOnRSFastPath` and `use` alias wording from `attach` block to `bus { flow }` and `bus { endpoint }`; page updated in the same work |
| `docs/architecture/doctor-and-health-checks.md` | Plugin registration and delivery checks | `// Design:` page of `doctor_snapshot.go`, `check_external.go`; modified page | "the `attach process` change (`DeliveryDisagreement.Fix`)" | departs | `DeliveryDisagreement.Fix` names `bus { flow }` instead of `attach process`, and `PluginDoctorState` carries `RegistrationName` and `External`; page updated in the same work |
| `docs/architecture/exabgp-bridge.md` | A neighbor command is one of three things | `// Design:` page of `bridge_neighbor.go`; modified page | "`attach exabgp-bridge`, once" | departs | Drops `attach` keyword from `create bgp peer` (pure bus delivers to `exabgp-bridge` via its `bus { flow }` and bridge `feed <ip>`); page updated in the same work |
| `docs/architecture/update-building.md` | Shared ownership admission | anchored to `exabgp/migration/migrate.go`; modified page | "when no `attach process` block grants the owner process the owning source's" | departs | Updates `attach process` reference to `bus { flow }`; page updated in the same work |
| `docs/architecture/update-building.md` | Design decisions | decision row on `docs/architecture/update-building.md` | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | ExaBGP migration preserves `session { flowspec-redirect-next-hop true; }` unchanged while emitting `bus { endpoint; flow; }` |
| `docs/guide/ddos-mitigation.md` | FlowSpec mode sensor blindness | `// Design:` page of `ddos/flowspec/doctor.go`; modified page | "A block of another name (`plugin { internal <name> { use ddos-flowspec } }`)" | departs | Replaces `plugin { internal ... { use ddos-flowspec } }` with `bus { endpoint <name> { internal ddos-flowspec; } }` and resolves the alias ambiguity by matching `PluginDoctorState.RegistrationName`; page updated in the same work |
| `docs/config-reference.md` | Plugin Loading | modified page | "Plugins are declared in the `plugin { }` block:" | departs | Replaces `plugin { }` and `attach process` syntax sections with `bus { endpoint; flow; hub; }`; page updated in the same work |
| `docs/exabgp/exabgp-migration.md` | Key Differences | modified page | "`attach process <name> { }`" | departs | ExaBGP migration emits `bus { endpoint; flow; }` and pure `bgp { }`; page updated in the same work |
| `docs/architecture/testing/ci-format.md` | Where the block goes | anchored to `cmd/ze/hub/main.go` | "The daemon `-` is recognized by POSITION, not by a list of verbs:" | agrees | Built-in `ze --plugin ze.<name> -` invocations continue using the exact same stdin-to-file routing |
| `docs/architecture/config/transaction-protocol.md` | Phase 1: Verify | anchored to `plugin/registry`, `loader_create.go`, `reactor_api.go`, `startup.go`; modified page | "`plugin { internal rpki { use bgp-rpki } }` names the process `rpki`, and its `pki` section still arrives" | departs | Replaces `plugin { internal ... { use ... } }` and `external ... { run ... }` syntax with `bus { endpoint <name> { internal ...; } }` and `bus { endpoint <name> { external { run ...; } } }` while preserving `RunsExternalProgram()` gating; page updated in the same work |
| `docs/architecture/plugin/component-boundaries.md` | The four components | anchored to `internal/core/events/events.go` | - | silent | Covers the four public `pkg/ze` interfaces; silent on `plugin { }` / `attach process` / `bus { }` config syntax |
| `docs/guide/flowspec-protected-router.md` | 2. Update the active stored config | `// Design:` page of `bgp/config/flowspec_binding.go`; modified page | "Execution mode comes from the effective plugin declaration: `use bgp-rib` runs in-process even inside an `external` stanza, while `run "ze plugin bgp-rib"` forks a subprocess." | departs | `bus { endpoint }` makes `internal` and `external` mutually exclusive (no `use` inside `external`), and replaces `send [ update ]` / per-peer attach wording with `bus { flow }`; page updated in the same work |
| `ai/rules/repo-maintenance.md` | Directives | `// Design:` page of `managed/doctor.go` | - | silent | Covers generated-file maintenance and doctor registration hygiene; silent on `bus` config syntax |
| `docs/architecture/api/update-syntax.md` | Raw Passthrough Commands | anchored to `ze-bgp-conf.yang`, `registry.go`; modified page | "The peer must attach the program with `send [ raw ]`; no other send type permits it." | departs | Replaces per-peer `attach` + `send [ raw ]` wording with `bus { flow ... { from bgp; command [ raw ]; } }`; page updated in the same work |
| `docs/architecture/bgp/filter-path-asn.md` | The config | anchored to `ze-bgp-conf.yang` | - | silent | Covers `reject-asn` BGP filter config; silent on `plugin { }` / `attach process` / `bus { }` |
| `docs/architecture/config/environment-block.md` | TL;DR | anchored to `ze-bgp-conf.yang` | - | silent | Covers `environment { }` config block; silent on `plugin { }` / `attach process` / `bus { }` |
| `docs/architecture/config/environment.md` | Overview | anchored to `ze-bgp-conf.yang` | - | silent | Covers env var precedence; silent on `plugin { }` / `attach process` / `bus { }` |
| `docs/architecture/cli/command-verbs.md` | The axis: does the command change the system | anchored to `ze-peer-cmd.yang` | - | silent | Covers CLI verb classification (`show`/`create`/`delete`); silent on `attach` or `bus` |
| `docs/architecture/decisions/001-pull-model-metrics.md` | The Question | anchored to `registry.go` | - | silent | Covers Prometheus pull-model metrics; silent on `bus` config or plugin aliasing |
| `docs/architecture/dns/geodns.md` | Config model | anchored to `registry.go` | - | silent | Covers GeoDNS configuration; silent on `bus` config or plugin aliasing |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `registry.go`, `startup_autoload.go`, `startup.go` | - | silent | Covers IKEv2 engine decisions; silent on `bus` config or plugin aliasing |
| `docs/architecture/wire/nlri.md` | TL;DR (Read This First) | anchored to `registry.go` | - | silent | Covers BGP NLRI wire encoding; silent on `bus` config or plugin aliasing |
| `docs/architecture/config/system-update.md` | The backend | anchored to `validators.go` | - | silent | Covers appliance system update backend; silent on `bus` config |
| `docs/architecture/isis/isis-4-component-config.md` | Decision: registration, never a switch | anchored to `validators.go` | - | silent | Covers IS-IS component config registration; silent on `bus` config |
| `docs/architecture/ospf/ospf-4-component-config.md` | Decisions | anchored to `validators.go` | - | silent | Covers OSPF component config; silent on `bus` config |
| `docs/architecture/resolve.md` | Structure | anchored to `validators.go` | - | silent | Covers DNS resolution helper; silent on `bus` config |
| `docs/architecture/dns/as112.md` | The response code says where the query name sits | anchored to `listener.go` | - | silent | Covers AS112 DNS responses; silent on `bus` config or `bus-hub` listener |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `peers.go`, `loader_create.go`, `peer_settings_apply.go` | - | silent | Covers deterministic config apply order; silent on `bus` syntax |
| `docs/architecture/wire/attributes.md` | TL;DR (Read This First) | anchored to `peers.go` | - | silent | Covers BGP path attribute wire encoding; silent on `bus` syntax |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `resolve.go`, `filter_state_binding.go`, `reactor/config.go`, `events.go`, `delivery_graph.go` | - | silent | Covers RFC 9234 route metadata keys; silent on `bus` config syntax |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `redistribute_binding.go`; modified page | "the message names the peer and the forwarder, and tells the operator to attach the RIB to the peer with `receive [ update-received ]` or to take the peer down around the change." | departs | Replaces per-peer attach wording and `source_policy_reload.go` refusal text (`receive [ update-received ]`) with `bus { flow }`; page updated in the same work |
| `docs/architecture/route-selection.md` | Overview | anchored to `redistribute_binding.go` | - | silent | Covers BGP best-path selection; silent on `bus` config syntax |
| `docs/architecture/rfc-may-decisions.md` | RFC 4760 Section 7 - Non-Negotiated AFI/SAFI Handling | anchored to `reactor/config.go` | - | silent | Covers BGP RFC MAY decisions; silent on `bus` config syntax |
| `docs/architecture/wire/capabilities.md` | TL;DR (Read This First) | anchored to `reactor/config.go` | - | silent | Covers BGP OPEN capability wire encoding; silent on `bus` config syntax |
| `docs/architecture/api/text-format.md` | Current Format | anchored to `reactor_api.go` | - | silent | Covers BGP text event payload format; silent on `bus` config syntax |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | anchored to `reactor_api.go`, `peer_derived_bindings.go` | - | silent | Covers BGP FSM state transitions; silent on `bus` config syntax |
| `docs/architecture/behavior/peer-lifecycle.md` | TL;DR | anchored to `reactor_api.go`, `peer_settings_apply.go`, `peer_derived_bindings.go`, `events.go` | - | silent | Covers BGP peer TCP/session lifecycle; silent on `bus` config syntax |
| `docs/architecture/wire/nlri-bgpls.md` | Consumer Decoding and Origination | anchored to `reactor_api.go`; modified page | "Collector peers negotiate the `bgp-ls` family and attach the plugin with `state` and `refresh` event delivery and UPDATE-send permission." | departs | Replaces per-peer `attach` wording for `bgp-ls-export` and `bgp-epe` with `bus { flow }`; page updated in the same work |
| `docs/architecture/api/json-format.md` | Overview | anchored to `peer_derived_bindings.go`, `events.go` | - | silent | Covers BGP JSON event envelope format; silent on `bus` config syntax |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 1: the backend loads on demand | anchored to `startup_autoload.go`, `startup.go` | - | silent | Covers firewall nft table ownership; silent on `bus` config syntax |
| `docs/architecture/ospf/bfd-client.md` | Decisions | anchored to `startup_autoload.go` | - | silent | Covers OSPF BFD client integration; silent on `bus` config syntax |
| `docs/architecture/plugin/feature-gates.md` | The inventory is `feature-gates.txt` | anchored to `startup_autoload.go` | - | silent | Covers `ze_<feature>` compile-out tags; silent on `bus` config syntax |
| `docs/architecture/static-routes.md` | A main-table route reaches the FIB through the Loc-RIB | anchored to `startup_autoload.go` | - | silent | Covers static route installation into Loc-RIB; silent on `bus` config syntax |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `startup.go` | - | silent | Covers IPC wire framing; silent on `bus` config syntax |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to `startup.go`, `cmd/ze/hub/main.go` | - | silent | Covers CLI completion producers; silent on `bus` config syntax |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | RFC obligations carried by this code | anchored to `startup.go` | - | silent | Covers PPPoE client RFC obligations; silent on `bus` config syntax |
| `docs/architecture/storage-backends.md` | The memory encoding | anchored to `data_rpc.go`, `main.go`, `ze_core_start.go` | - | silent | Covers ZeFS/memory storage backend mechanics; silent on `bus` config syntax |
| `docs/architecture/web-interface.md` | Source Files | anchored to `data_rpc.go`, `main.go` | - | silent | Covers Web UI routes and HTMX templates; silent on `bus` config syntax |
| `docs/architecture/zefs-format.md` | Netcapstring | anchored to `data_rpc.go` | - | silent | Covers ZeFS binary format; silent on `bus` config syntax |
| `docs/architecture/appliance/remote-operations.md` | Decisions | anchored to `cmd/ze/hub/main.go` | - | silent | Covers appliance remote operations; silent on `bus` config syntax |
| `docs/architecture/behavior/signals.md` | Supported Signals | anchored to `cmd/ze/hub/main.go` | - | silent | Covers SIGHUP/SIGTERM handling; silent on `bus` config syntax |
| `docs/architecture/diagnostics/crash-capture.md` | The decisions | anchored to `cmd/ze/hub/main.go` | - | silent | Covers crash capture dumps; silent on `bus` config syntax |
| `docs/architecture/pki/pki-store.md` | Decisions | anchored to `cmd/ze/hub/main.go` | - | silent | Covers PKI certificate store; silent on `bus` config syntax |
| `docs/architecture/pki/tls-listeners.md` | Decision: the leaf comes first, then the intermediates | anchored to `cmd/ze/hub/main.go` | - | silent | Covers TLS certificate chain ordering; silent on `bus` config syntax |
| `docs/architecture/appliance/device-config.md` | Legacy boot inbox | anchored to `cmd/ze/ze_core_start.go` | - | silent | Covers appliance legacy boot inbox; silent on `bus` config syntax |
| `docs/architecture/appliance/kernel-profiles.md` | Go owns the guarantee | anchored to `cmd/ze/ze_core_start.go` | - | silent | Covers appliance kernel profiles; silent on `bus` config syntax |
| `docs/architecture/fleet-interface.md` | The package | anchored to `cmd/ze/ze_core_start.go` | - | silent | Covers `pkg/fleet` wire envelopes; silent on `bus` config syntax |
| `docs/architecture/config/exabgp-syntax.md` | Static Routes | anchored to `exabgp/migration/migrate.go` | - | silent | Covers ExaBGP static route syntax; silent on `bus` config syntax |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config file / stdin / editor commit / SIGHUP reload (`bus { endpoint ...; flow ...; hub ...; }`, `bgp { ... }`), `ze start --plugin <builtin>`, `ze exabgp migrate`, and runtime plugin RPCs (`state-*`, `doctor` snapshot).

### Transformation Path
1. **Init-time registration:** Every component and plugin registers its event namespaces (`events.RegisterNamespace`), namespaced command types (`events.RegisterCommandType`), and `Registration.EventNamespaces` / `Registration.CommandTypes` at `init()`.
2. **Pass 1 (`config.FirstPass`):** `PluginOnlySchema` parses `bus` (extracting external `endpoint` entries for early Stage 1 schema publish and `bus { hub }` for managed mode) and `environment`. `MergeCliPlugins` admits built-in `--plugin` names only and refuses external paths.
3. **Full parse & validation (`config.LoadConfig` / `ze config validate`):** `ze-plugin-conf.yang` (`container bus`) and `ze-bgp-conf.yang` (no `container attach`) parse into `*config.Tree`. `ValidateCustomSections` validates `bus` (including `from` built-in-only check, optional `namespace` ownership check, `to` endpoint/built-in check, `event` and `command` membership against init-populated registries, and `match` tag syntax).
4. **Bus flow projection & tag matching (`plugin/server` & `bgp/config`):** `ExtractBusFromTree` produces `BusConfig{Endpoints, Flows, Hub}`. `DeliveryGraph` indexes `Flows` per `(publisher, namespace, event/command, match tags)` without any BGP imports. BGP emits generic tags (`peer`, `group`) on peer events and evaluates send permission via `DeliveryGraph` / `Peer.maySend` using the peer's tags. Custom event/send auto-load in `startup_autoload.go` reads `BusConfig.Flows`.
5. **Process identity & state/doctor keying:** `process.Process` carries `Name()` (operator endpoint alias), `RegistrationName()` (compiled registry name), and `RunsExternalProgram()`. State-key checks (`dispatch_state.go`), `PluginDoctorState` (`doctor_snapshot.go`), `runningOwnerError` (`data_rpc.go`), `RequiresPeerUpBarrier`, `configRefusalIsFatal`, `isPluginLoaded`, `collectOrphanedDependencies`, and `appendHeldNeedProviders` key on `RegistrationName()` when `!RunsExternalProgram()`.
6. **ExaBGP migration (`internal/exabgp/migration`):** Emits `bus { endpoint ...; flow ...; }` (deduplicating identical `(from, to, event, command, match)` flows) and pure `bgp { }` (relying on `exabgp { bridge { process { feed <peerIP> } } }` for per-peer ExaBGP script selection).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `internal/core/events` <-> `internal/component/config` | Init-populated publisher/namespace/command registries read by `bus` validators | No |
| `internal/component/config` <-> `internal/component/plugin/server` | `BusConfig{Endpoints, Flows, Hub}` value struct; zero BGP imports in `config` or `plugin` | No |
| `internal/component/bgp` <-> `internal/component/plugin/server` | BGP attaches generic `peer`/`group` tags on bus events and queries flow delivery/send grants through `DeliveryGraph` | No |
| Engine <-> External plugin process | Unchanged TLS Stage 1–5 RPC protocol; external endpoints cannot appear in `from` and cannot claim compiled-in state keys | No |

### Changed-Wire Census

**Search evidence:** Searched `pkg/plugin/rpc/`, `internal/core/ipc/yang/`, and `pkg/fleet/` for `attach`, `plugin`, `DisseminationDecl`, `DeclareRegistrationInput`, `EventEnvelope`, and `ConfigFetchResponse` (positive controls: `DeclareRegistrationInput` in `pkg/plugin/rpc/types.go`, `list dissemination` in `internal/core/ipc/yang/ze-plugin-engine.yang`, `ConfigFetchResponse` in `pkg/fleet/envelope.go`). **No plugin RPC or fleet wire format changes in Child 1**: because Q2 restricts `from` to built-in publishers, Stage 1 `DeclareRegistrationInput` needs no `EventTypes`/`SendTypes` fields; event envelopes, subscribe RPCs, and `pkg/fleet/` opaque base64 config envelopes are unchanged. The only operator/CLI contract surface that changes is the YANG configuration grammar (`bus` replacing `plugin` + `attach`), the `create bgp peer` CLI command (`ze-peer-cmd.yang`), and the `show listeners` service name (`plugin-hub` -> `bus-hub`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Ze YANG config grammar (`bus` replacing `plugin` + `attach`) | writer | `internal/exabgp/migration/migrate_serialize.go` (`SerializeTree`), `internal/component/config/serialize.go` (`Serialize`), `internal/component/config/cli/cmd_fmt.go`, ~1,752 tracked repo configs/tests/docs | Verified in Phase 1 census: 248 Go, 1,197 `.ci`, 2 `.et`, 148 interop, 74 other test, 55 docs, 9 website, 11 `ai/`+`etc/` | Change in same work via parsed-tree converter (`ai/META-DECISIONS.md`, "Ze is pre-release"); `retired.go` emits migration hints |
| Ze YANG config grammar (`bus` replacing `plugin` + `attach`) | reader | `internal/component/config/first_pass.go` (`FirstPass`), `loader.go` (`ExtractPluginsFromTree`), `loader_extract.go` (`ExtractHubConfig`), `internal/component/bgp/reactor/config.go` (`parseProcessBindingsFromTree`), `internal/component/plugin/doctor/check_external.go`, external gNMI/REST/CLI operators | Verified at each producer in Phase 1 | Change in same work (`no-layering.md`: delete `plugin` & `attach`, implement `bus`) |
| `create bgp peer` CLI command (`ze-peer-cmd.yang`) | writer | `internal/exabgp/bridge/bridge_neighbor.go` (`convertNeighborCreate`), operators/tests | Verified: `convertNeighborCreate` emits `create bgp peer <ip> ... attach exabgp-bridge` | Change in same work: drop `attach` keyword (pure bus delivers to `exabgp-bridge` via its `bus { flow }` + bridge `feed <ip>`) |
| `create bgp peer` CLI command (`ze-peer-cmd.yang`) | reader | `internal/component/bgp/plugins/cmd/peer/create.go` (`peerCreateKeywords`, `setAttach`), `ze-peer-cmd.yang` | Verified: `setAttach` writes directly to `attach.process.<name>` | Change in same work: remove `attach` keyword and `setAttach` |
| Listener service name (`show listeners`, `config graph`) | writer | `internal/component/config/listener.go` (`DiscoverListenerServices`, `buildListenerService`), `internal/component/config/graph.go` (`listenerSectionName`) | Verified: derives `<top>-<child>` = `"plugin-hub"` -> `"bus-hub"` | Change in same work: update `graph.go` and tests matching `"plugin-hub"` |
| Listener service name (`show listeners`, `config graph`) | reader | `internal/component/config/cli/` (`show listeners`), `internal/component/managed/doctor.go`, `internal/component/plugin/doctor/check_managed_listener.go`, API/CLI consumers | Verified in Phase 1 W-9 audit | Change in same work: update diagnostics and test expectations to `"bus-hub"` / `bus hub` |

### Integration Points
- `internal/component/config/first_pass.go` (`FirstPass`, `firstPassRoots`) - switches `"plugin"` to `"bus"` so early external Stage 1 and `bus { hub }` extraction run before full parse.
- `internal/component/config/validate_sections.go` (`validatedSections`) - replaces `sectionPlugin` with `sectionBus` after moving event/command registration to `init()`.
- `internal/component/plugin/server/delivery_graph.go` (`DeliveryGraph`) - indexes `bus { flow }` entries with generic tag matching; feeds BGP delivery, `send_permission.go`, and `delivery_reconcile.go`.
- `internal/component/plugin/server/dispatch_state.go` & `doctor_snapshot.go` - keys compiled-in state access and doctor snapshots on `RegistrationName()` (guarded by `!RunsExternalProgram()`).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | All configs parse through `config.FirstPass` + `config.LoadConfig` into `BusConfig` -> `DeliveryGraph`; no second config path or hybrid grammar |
| No unintended coupling (components stay isolated) | Yes | `bus` carries zero BGP keywords (`peer`/`group`); `config` and `plugin` import nothing from `bgp`; `internal/core/events` stays leaf-only |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends `internal/core/events`, `DeliveryGraph`, `process.RegistrationName()`, `RetiredKeywordHint`, and `config.Tree` serialization |
| Zero-copy preserved where applicable (refs, not copies) | Yes | BGP UPDATE wire buffers and `RawMessage` lifetime are untouched; tag lookup uses pre-resolved slice/IDs on peer setup rather than per-packet allocations |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Plugins register `EventNamespaces` and `CommandTypes` from `init()`; `bus` validates `from`, `namespace`, `event`, and `command` from those registries with zero per-plugin switches |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Audited `firstPassRoots`, `validatedSections`, `retiredKeywords`, `listenerSectionName`, `PluginOnlySchema`, `peerCreateKeywords`; `bus` replaces `plugin` in the config-root infrastructure lists and all publisher/event/command vocabularies derive from `events` and `registry` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Dropping `attach > content` (`ProcessBinding.Encoding`/`Format`) removes no working feature | Phase 1 W-10 trace: `delivery_graph.go` (`addBinding`) drops both fields; `bgp/server/events.go` reads only `proc.Encoding()`/`Format()` | A consumer outside `bgp/server` depends on per-attachment encoding | Repo-wide grep for `ProcessBinding` readers completed in Phase 1 | validated |
| A-2 | Existing multi-peer BGP tests and interop scenarios that attached different plugins per peer can be expressed via pure `bus { flow }` (default all-peers, or optional generic `match` tag on `peer`/`group`) without putting `flow` in `bgp {}` | Phase 1 census of `test/interop/` and `test/**/*.ci` plus Thomas's 2026-11-23 pure-bus + generic-tag ruling | A scenario requires per-peer semantics that `match` tags cannot represent | Parsed-tree conversion dry-run + scoped `parse`, `plugin`, `reload`, `encode`, `exabgp` functional tests | unvalidated |
| A-3 | All event namespaces and command types referenced by `bus { flow }` can register at `init()` (before `LoadConfig`) so `validatedSections` validates `bus` cleanly | Phase 1 check of `validate_sections.go` and `internal/core/events/events.go` | A plugin registers event/send types only at runtime after `LoadConfig` | Unit test verifying `ValidateCustomSections` on `bus` before reactor creation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `validate_sections.go` timing trap causes `LoadConfig` to reject valid `event`/`command` tokens before `RegisterPluginSendTypes` runs | `TestRepositoryConfigsValidate` or `test/parse/*.ci` fails on `update`/`refresh`/`update-rpki` | Move `RegisterPluginEventTypes` and `RegisterPluginSendTypes` (and namespaced command registration) into `init()` / lazy once-init before `ValidateCustomSections` |
| R-2 | Parsed-tree converter corrupts `$PORT` placeholders, re-encodes plaintext test secrets (`$9$`), or mangles `refused=` negative `.ci` blocks | Diff review or `TestRepositoryConfigsValidate` / `test/parse/*.ci` failures | Substitute `$PORT` sentinels around parse/serialize, bypass `EncodeSecrets` in converter, and triage `refused=` blocks explicitly |
| R-3 | `hub` validation silently skipped or `"bus-hub"` listener edge dropped in `graph.go` / `cmd_validate.go` | Managed/hub `.ci` or `TestConfigGraph` tests fail | Update `firstPassRoots`, `PluginOnlySchema`, `cmd_validate.go` guard, `graph.go` `listenerSectionName`, and all 12 diagnostic strings together |
| R-4 | External plugin named after a compiled-in plugin (e.g. `endpoint ddos-flowspec { external { run ./evil; } }`) tries to claim compiled-in state-key grants | Unit test of external plugin calling `state-get`/`state-put` | Guard every `RegistrationName()` lookup in `dispatch_state.go` and `doctor_snapshot.go` with `!proc.Config().RunsExternalProgram()` |
| R-5 | Scale of ~1,752-file repo conversion creates merge friction with concurrent sessions in shared `main` checkout | Dirty files or compile conflicts during conversion | Land core `bus` engine/YANG + parsed-tree converter cleanly and run conversion in a focused pass, committing coherent compiling chunks per `ai/rules/git-safety.md` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Config files fail to parse/validate, BGP plugins receive no events or lose send grants, managed hub listeners fail to bind, or aliased plugins fail state access |
| How is it reverted? | Git revert of the cutover commits; pre-release `main` has no external deployed users |
| Who else touches this path? | Child 2 (`spec-bus-2-implied-display-and-grant-refusals.md`), Child 3 (`spec-bus-3-external-command-gating.md`), and any concurrent spec adding new `.ci` tests with `plugin {` or `attach process` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` and `ze start` of a `bus { endpoint; flow; }` config with pure `bgp { }` | -> | `config.FirstPass` -> `ExtractBusFromTree` -> `DeliveryGraph` -> BGP event delivery & send permission | `test/plugin/bus-endpoint-flow-delivery.ci` (new) |
| `ze start --plugin /path/to/program` | -> | `config.MergeCliPlugins` built-in-only guard | `test/parse/bus-cli-plugin-builtin-only.ci` (new) |
| Aliased internal plugin (`bus { endpoint my-ddos { internal ddos-flowspec; } }`) accessing state & `show doctor` | -> | `dispatch_state.go` (`stateInput`) + `doctor_snapshot.go` (`PluginDoctorState`) | `TestAliasedInternalPluginStateAndDoctor` (`internal/component/plugin/server/dispatch_state_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Config declares `bus { endpoint collector { external { run ...; } encoding json; } flow watch { from bgp; to collector; event [ update-received state ]; } }` with pure `bgp { }` (zero `attach process`, zero `flow` in `bgp {}`) | Config passes `ze config validate` and boots; `collector` receives `update-received` and `state` for all BGP peers and is refused if it sends `update` | Unlisted event/command types stay blocked (silence by default for external endpoints) |
| AC-2 | Config declares a `bus { flow }` with optional generic `match` tag (e.g. `match group { value [ customers ]; }` or `match source { value [ bgp ]; }`) | Only messages carrying matching tag key/value are delivered (and BGP peer sends are restricted to matching peers); omitting `match` delivers all messages from `from` | Messages whose tags match every listed tag key pass; non-matching messages are dropped |
| AC-3 | Config declares `bus { flow }` with multi-namespace publisher `from l2tp; namespace subscriber; to watcher; event [ session-up ];` vs an invalid `namespace`, unknown `from`/`to`, external endpoint in `from`, or unregistered `event`/`command` | Valid flow loads and delivers; invalid `from`, external `from`, wrong `namespace`, unknown `to`, or unregistered `event`/`command` is refused at `ze config validate`, editor commit, and `ze start`, naming the flow, offending token, and accepted values | Built-in publishers and declared endpoints are admitted as specified |
| AC-4 | Config moves `hub { server ...; client ...; }` under `bus { hub { ... } }` | Managed hub listener binds and validates, `show listeners` reports service `"bus-hub"`, `config graph` links `bus` to `"bus-hub"`, and managed doctor checks pass | None: old `plugin { hub }` is removed |
| AC-5 | Config uses retired `plugin { }`, `attach process`, or `process` under `bgp` | Rejected at parse with `RetiredKeywordHint` pointing to `bus { endpoint; flow; hub; }` (zero hybrid acceptance) | None (`no-layering.md`) |
| AC-6 | Operator invokes `ze start --plugin ze.bgp-bmp <cfg>` vs `ze start --plugin /tmp/ext.py <cfg>` | Built-in `--plugin ze.bgp-bmp` loads and runs; external path `/tmp/ext.py` is refused at start with a message naming `bus { endpoint <name> { external { run ...; } } }` | Offline `ze bgp decode --plugin /path` and `ze plugin test --plugin /path` keep accepting external decoders |
| AC-7 | ExaBGP config with `process`, `api { processes [...] }`, and dynamic `create neighbor ... api` run through `ze exabgp migrate` and `exabgp-bridge` | Migration emits deduplicated `bus { endpoint; flow; }` and pure `bgp { }` (zero `plugin { }` or `attach process`); migrated config loads and feeds/announces routes; dynamic `create bgp peer` works without `attach` | `exabgp { bridge { process { encoder; feed; } } }` stays intact for per-peer ExaBGP script filtering |
| AC-8 | Aliased built-in endpoint `bus { endpoint my-ddos { internal ddos-flowspec; } }` runs, reads/writes its state key, and is checked by `show doctor` and `ze data rm`; external endpoint `bus { endpoint ddos-flowspec { external { run ...; } } }` attempts the same state key | Aliased internal plugin succeeds on state get/put/list, reports healthy in `show doctor`, and blocks `ze data rm` while running; external endpoint named `ddos-flowspec` is refused state access (cannot spoof compiled-in grants). Sibling alias sites (`RequiresPeerUpBarrier`, `configRefusalIsFatal`, `isPluginLoaded`, `collectOrphanedDependencies`, `appendHeldNeedProviders`) resolve via `RegistrationName()` | None: external endpoints never receive compiled-in state grants |
| AC-9 | Every tracked `.conf`, `.ci`, `.et`, interop scenario, fixture, doc, and website example in the repository | Zero active configs/tests/docs retain `plugin {` or `attach process` (excluding historical `plan/` records); `TestRepositoryConfigsValidate` and scoped functional suites (`parse`, `plugin`, `reload`, `encode`, `exabgp`, `editor`) pass | Historical entries under `plan/journal/`, `plan/learned/`, `plan/handover/`, and `plan/records/` may quote the old syntax as history |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes a pure `bgp { }` config and wires an external collector in `bus { endpoint; flow; }` (with and without `match` tags), validates, and runs | `ze config validate` -> `FirstPass` -> `LoadConfig` -> `ExtractBusFromTree` -> `DeliveryGraph` -> BGP event delivery & send gate | `test/plugin/bus-endpoint-flow-delivery.ci` |
| 2 | Runs an aliased built-in plugin (`bus { endpoint my-ddos { internal ddos-flowspec; } }`) and checks `show doctor` | `ExtractBusFromTree` -> `process.NewProcess` (`RegistrationName`) -> `dispatch_state.go` + `doctor_snapshot.go` + `ddos/flowspec/doctor.go` | `TestAliasedInternalPluginStateAndDoctor` |
| 3 | Migrates an ExaBGP config with processes and runs it on Ze | `ze exabgp migrate` -> `SerializeTree` (`bus { endpoint; flow; }`) -> `ze start` -> `exabgp-bridge` | `test/exabgp/` suite |
| 4 | Configures managed hub under `bus { hub { server; client; } }` | `FirstPass` -> `ExtractHubConfig` -> `startManagedServer` -> `show listeners` (`bus-hub`) | `test/managed/` suite |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBusFlowValidationBuiltInFromAndNamespace` | `internal/component/config/bus_validate_test.go` | AC-3: built-in `from`, optional `namespace` disambiguation, external `from` refused, unregistered `event`/`command` refused before reactor creation (`validatedSections` timing) | |
| `TestBusFlowTagMatching` | `internal/component/plugin/server/delivery_graph_test.go` | AC-2: omit `match` delivers all; `match` filters by generic key/value tags | |
| `TestRetiredPluginAndAttachHints` | `internal/component/config/retired_test.go` | AC-5: `plugin`, `attach`, `process` refused with `bus` migration hint | |
| `TestMergeCliPluginsBuiltInOnly` | `internal/component/config/loader_test.go` | AC-6: built-in `--plugin` accepted; external path refused with `bus { endpoint }` fix | |
| `TestAliasedInternalPluginStateAndDoctor` | `internal/component/plugin/server/dispatch_state_test.go` | AC-8: aliased internal plugin gets state keys + doctor healthy + `runningOwnerError` protection; external plugin with matching name refused | |
| `TestAliasedPluginLifecycleHelpers` | `internal/component/plugin/server/startup_alias_test.go` | AC-8: `RequiresPeerUpBarrier`, `configRefusalIsFatal`, `isPluginLoaded`, `collectOrphanedDependencies`, `appendHeldNeedProviders` use `RegistrationName()` | |
| `TestExaBGPMigrateEmitsBusEndpointsAndFlows` | `internal/exabgp/migration/migrate_bus_test.go` | AC-7: deduplicated `bus { endpoint; flow; }` and pure `bgp { }` | |
| `TestRepositoryConfigsValidate` | `internal/component/config/cli/validate_corpus_test.go` | AC-9: every tracked repo config validates under `bus` schema | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `bus > endpoint > external > timeout` | Existing YANG range preserved from `plugin > external > timeout` | Unchanged | Unchanged | Unchanged |
| `bus > hub > server/client > port` | Existing `1..65535` preserved from `plugin > hub` | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bus-endpoint-flow-delivery` | `test/plugin/bus-endpoint-flow-delivery.ci` | AC-1, AC-2: pure `bgp {}` + `bus { endpoint; flow; }` with and without `match` tags | |
| `bus-cli-plugin-builtin-only` | `test/parse/bus-cli-plugin-builtin-only.ci` | AC-6: built-in `--plugin` works; external `--plugin` refused | |
| `bus-flow-validation-refusals` | `test/parse/bus-flow-validation-refusals.ci` | AC-3, AC-5: invalid `from`/`namespace`/`to`/`event`/`command` and retired `plugin`/`attach` refused | |

### Interop Tests (Scope: protocol)
N-A: No BGP/IGP wire format changes (`Scope: config`). Existing 148 interop scenario configs under `test/interop/` are converted to `bus { }` syntax and remain valid.

## Files to Modify
- `internal/component/plugin/yang/ze-plugin-conf.yang` - replace `container plugin` with `container bus` (`endpoint`, `flow`, `hub`)
- `internal/component/bgp/yang/ze-bgp-conf.yang` - remove `container attach` (including `content`)
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang`, `internal/component/bgp/plugins/cmd/peer/create.go` - remove `attach` keyword and `setAttach`
- `internal/core/events/events.go` - namespaced command type registry (`RegisterCommandType`, `ValidCommandTypes`) and publisher/namespace lookup
- `internal/component/plugin/registry/registry.go` - add `EventNamespaces` and `CommandTypes` fields on `Registration`
- `internal/component/plugin/resolve.go` - register plugin event namespaces & command types at `init()`; restrict startup CLI plugin resolution to built-in plugins
- `internal/component/config/first_pass.go`, `yang_schema.go`, `yang/join.go`, `yang/publish.go`, `loader.go`, `loader_extract.go` - switch `plugin` root to `bus`, extract `BusConfig{Endpoints, Flows, Hub}`, restrict `MergeCliPlugins` to built-in plugins
- `internal/component/config/validate_sections.go`, `validate_semantic.go`, `validators.go`, `retired.go`, `listener.go`, `graph.go`, `cli/cmd_validate.go` - validate `bus` section, `bus-hub` listener & graph edge, update `RetiredKeywordHint`
- `internal/component/bgp/config/plugins.go`, `peers.go`, `resolve.go`, `redistribute_binding.go`, `flowspec_binding.go`, `filter_state_binding.go`, `dissemination.go`, `dissemination_config.go`, `loader_create.go`, `register.go` - remove `extractBGPInlinePlugins` and `attach process` parsing; project bus flows onto peers and custom event/send auto-load
- `internal/component/bgp/reactor/config.go`, `delivery_graph.go`, `send_permission.go`, `reactor_api.go`, `peer_settings_apply.go`, `peer_derived_bindings.go`, `source_policy_reload.go` - remove `ProcessBinding.Encoding`/`Format` and `parseProcessBindingsFromTree`; emit generic `peer`/`group` tags, evaluate send permissions from bus flows, and update `source_policy_reload.go` attach refusal text to `bus { flow }`
- `internal/component/bgp/server/events.go` - fix `RequiresPeerUpBarrier` alias lookup; pass generic peer/group tags on bus delivery
- `internal/component/plugin/server/delivery_graph.go`, `delivery_reconcile.go`, `dissemination_validate.go`, `startup_autoload.go`, `startup.go` - generic tag-aware `DeliveryGraph` fed by `BusConfig.Flows`; fix `configRefusalIsFatal`, `isPluginLoaded`, `collectOrphanedDependencies`, `appendHeldNeedProviders` alias bugs; update refusal/fix texts from `attach process` to `bus { flow }`
- `internal/component/plugin/server/dispatch_state.go`, `doctor_snapshot.go` - key state access on `RegistrationName()` (guarded by `!RunsExternalProgram()`); add `RegistrationName` and `External` to `PluginDoctorState`
- `internal/plugins/ddos/flowspec/doctor.go`, `internal/component/config/storage/cli/data_rpc.go` - match `PluginDoctorState.RegistrationName` (non-external) and report configured alias in restart fixes
- `internal/component/plugin/doctor/check_external.go`, `check_managed_listener.go`, `internal/component/managed/client.go`, `cmd_fleet.go`, `doctor.go`, `cmd/ze/hub/main.go`, `managed_owner.go`, `cmd/ze/ze_core_start.go` - read `bus` instead of `plugin`; update `"plugin hub"` diagnostics to `"bus hub"` / `"bus-hub"`
- `internal/exabgp/migration/migrate.go`, `migrate_serialize.go`, `internal/exabgp/bridge/bridge_neighbor.go` - emit deduplicated `bus { endpoint; flow; }` and pure `bgp { }`; remove `attach` from `convertNeighborCreate`
- `docs/architecture/api/architecture.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/config/syntax.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/core-design.md`, `docs/architecture/fleet-config.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/system-architecture.md`, `docs/architecture/rib-transition.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/api/capability-contract.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/exabgp-bridge.md`, `docs/architecture/update-building.md`, `docs/guide/ddos-mitigation.md`, `docs/guide/flowspec-protected-router.md`, `docs/guide/plugins.md`, `docs/guide/configuration.md`, `docs/config-reference.md`, `docs/exabgp/exabgp-migration.md`, `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `website/` - update all departed docs, doors, and examples to `bus { endpoint; flow; hub; }` and record the 2026-11-23 rulings
- Tracked `.conf`, `.ci`, `.et`, interop scenarios, fixtures, and unit tests (~1,752 files) - converted via parsed-tree converter

## Files to Create
- `internal/component/config/bus_validate_test.go` - unit tests for `bus` flow validation (AC-3)
- `internal/component/plugin/server/startup_alias_test.go` - unit tests for sibling alias-keying fixes (AC-8)
- `internal/exabgp/migration/migrate_bus_test.go` - unit tests for ExaBGP migration to `bus` (AC-7)
- `test/plugin/bus-endpoint-flow-delivery.ci` - end-to-end pure-bus + tag-matching functional test (AC-1, AC-2)
- `test/parse/bus-cli-plugin-builtin-only.ci` - `--plugin` built-in-only functional test (AC-6)
- `test/parse/bus-flow-validation-refusals.ci` - invalid flow and retired keyword refusal test (AC-3, AC-5)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/plugin/yang/ze-plugin-conf.yang` (`bus`), `internal/component/bgp/yang/ze-bgp-conf.yang` (remove `attach`), `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` (remove `attach`); regenerate glue via `./le yang glue update` |
| YANG validation constraints | Yes | `enumeration` on `encoding`, `must`/mutual exclusion on `internal` vs `external`, `range` on `timeout` and `hub` ports |
| YANG custom validators | Yes | `ze:validate` for `external-plugin-name`, `internal-plugin-name`, `bus-flow-from`, `bus-flow-namespace`, `bus-flow-to`, `receive-event-type`, `send-message-type` (all reading `init()`-populated registries) |
| CLI commands/flags | Yes | `cmd/ze/ze_core_dispatch.go`, `ze_core_start.go` (restrict `--plugin` to built-in plugins); `cmd/peer/create.go` (drop `attach`) |
| CLI grammar (keyword before value) | Yes | `bus { endpoint <name> ...; flow <name> ...; }` and `create bgp peer` obey R1–R9 (`./le cli grammar`) |
| Editor autocomplete | Yes | Automatic for YANG enums/containers; `CompleteFn` for `internal`, `from`, `namespace`, `to`, `event`, `command` |
| Functional test for new RPC/API | Yes | `test/plugin/bus-endpoint-flow-delivery.ci`, `test/parse/bus-cli-plugin-builtin-only.ci`, `test/parse/bus-flow-validation-refusals.ci` |
| Pipe completeness | N-A | No new operational command added here (`\| implied` is in Child 2) |
| Env var registration | N-A | No new env vars introduced (`ZE_PLUGIN_*` wire env vars unchanged) |
| Doctor check for runtime dependencies | Yes | Updates `internal/component/plugin/doctor/check_external.go`, `check_managed_listener.go`, `internal/plugins/ddos/flowspec/doctor.go`, `internal/component/managed/doctor.go` to read `bus` and `PluginDoctorState.RegistrationName` |
| Doctor code per problem class | Yes | Existing doctor codes preserved (`doctor-plugin-delivery-disagreement`, managed listener, ddos-flowspec); fix texts updated to `bus` syntax |
| Prometheus counters/metrics | N-A | No new metrics in Child 1 (external command refusal counter is in Child 3) |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP family, capability, or attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/` pages referencing `attach process` or `plugin {` (regenerate `docs/features.md`) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md`, `docs/config-reference.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`ze start --plugin` built-in-only, `create bgp peer`) |
| 4 | API/RPC added/changed? | No | Plugin RPC wire format unchanged; verified against `pkg/plugin/rpc/` and `ze-plugin-engine.yang` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/plugin-development/` |
| 6 | Has a user guide page? | Yes | `docs/guide/plugins.md`, `docs/guide/route-reflection.md`, `docs/guide/fleet-config.md` |
| 7 | Wire format changed? | No | BGP/IGP/plugin RPC wire formats unchanged |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` (fix stale Stage 1 order prose; keep wire docs accurate) |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Config syntax and internal identity fix only |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (update `attach process` / `plugin {` references) |
| 11 | Affects daemon comparison? | No | Checked `docs/comparison.md`: no `attach process` syntax dependency |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/core-design.md`, `docs/architecture/exabgp-bridge.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata keys added or changed |
| 14 | Prometheus counters added/changed? | No | No counters added in Child 1 |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md` (update `plugin {` examples if present) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: `docs/architecture/api/commands.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/fleet-config.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/system-architecture.md`, `docs/guide/ddos-mitigation.md`, and `docs/guide/flowspec-protected-router.md` are updated in `Files to Modify`; `ai/rules/repo-maintenance.md` (declared by `managed/doctor.go`) is unaffected because it does not mention `plugin { }` or `attach process` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | All 55 `docs/` files, 9 `website/` files, and 7 `ai/` files from Phase 1 census |

## Design presentation

### What changes

Configuration separates who sits on the event bus from BGP protocol configuration. The `plugin { internal ...; external ...; hub ...; }` block and BGP's `attach process` block (including inline `run`/`use` and the dead `content` block) are removed and replaced by a single protocol-agnostic `bus { endpoint ...; flow ...; hub ...; }` block. `bgp { }` contains only BGP settings (zero `attach process` and zero `flow`), built-in plugins subscribe to generic bus events via `register()`, and `bus { flow }` defines `from <built-in publisher> -> to <endpoint>` relationships with optional `namespace` disambiguation and optional protocol-agnostic `match` tag filters (for example `source bgp` on `best-change`, or `group`/`peer` tags emitted by BGP). External endpoints may appear only in `to`, `ze start --plugin` accepts built-in plugins only (refusing external program paths with a fix naming `bus { endpoint }`), `bus { hub }` renames the hub listener service from `"plugin-hub"` to `"bus-hub"`, aliased internal plugins resolve state-key grants, doctor snapshots, and lifecycle helpers through their compiled registry name (guarded against external spoofing), and every repository config, test, ExaBGP migration output, and doc page is converted in the same work. Implied-config display (`| implied`) + generic narrow-grant refusals move to Child 2 (`spec-bus-2-implied-display-and-grant-refusals.md`), and full external command gating (closing the live `ReservedInternalPrefix` AAA bypass) moves to Child 3 (`spec-bus-3-external-command-gating.md`).

### Before and after

1. **Declaring an external collector and wiring BGP events (pure `bgp {}`, zero `attach process`):**

Before:
```
plugin {
    external collector { run ./collector.py; encoder json; }
}
bgp {
    group transit {
        attach process collector { receive [ update-received state ]; }
        peer 198.51.100.1 { remote-as 65001; }
    }
}
```

After (all peers by default, or scoped with generic message tags inside `bus {}`):
```
bus {
    endpoint collector { external { run ./collector.py; } encoding json; }
    flow watch {
        from bgp;
        to collector;
        match group { value [ transit ]; }
        event [ update-received state ];
    }
}
bgp {
    group transit {
        peer 198.51.100.1 { remote-as 65001; }
    }
}
```

2. **Moving `hub` under `bus`:**

Before:
```
plugin {
    hub { server { listen 0.0.0.0:9443; } }
}
```
(`show listeners` reports `plugin-hub`.)

After:
```
bus {
    hub { server { listen 0.0.0.0:9443; } }
}
```
(`show listeners` reports `bus-hub`.)

3. **External program passed to `ze start --plugin`:**

Before: `ze start --plugin /opt/watcher.py ze.conf` started `/opt/watcher.py` as plugin `"watcher.py"` (illegal dot in name, invisible to `ze config validate`).

After: `ze start --plugin /opt/watcher.py ze.conf` is refused with an error telling the operator to declare `bus { endpoint <name> { external { run /opt/watcher.py; } } }`; `ze start --plugin ze.bgp-bmp ze.conf` continues working for built-in plugins.

4. **Aliased built-in plugin accessing persistent state and doctor:**

Before: `plugin { internal my-ddos { use ddos-flowspec; } }` failed every state read/write (`plugin my-ddos holds no grant...`) and `show doctor` reported `ddos-flowspec` as not running.

After: `bus { endpoint my-ddos { internal ddos-flowspec; } }` authorizes state keys and doctor snapshots via compiled registry name `ddos-flowspec` (while an external endpoint named `ddos-flowspec` is refused).

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep `flow [ ... ]` inside `bgp { group; peer; }` (2026-11-17 D-5) | Leaks bus wiring into `bgp {}`; reversed by Thomas on 2026-11-23 in favor of pure `bgp {}` and pure protocol-agnostic `bus {}` |
| Put BGP-specific `group`/`peer` leaves directly on `bus { flow }` | Leaks BGP concepts into the protocol-agnostic bus; replaced by optional generic `match <tag> { value [ ... ]; }` filtering |
| Keep arbitrary external paths on `ze start --plugin` (or add `--plugin name=/path` to `ze config validate`) | Breaks `config-validate-agrees-with-boot` and produces illegal dot-containing names; built-in-only `--plugin` keeps all 196 built-in test lines working cleanly |
| Allow external plugins in `bus { flow { from ... } }` | Requires Stage 1 `EventTypes`/`SendTypes` wire changes, makes offline validation incomplete, and opens external event spoofing |
| Ship one monolithic spec covering grammar cutover, `\| implied` display, narrow-grant refusals, and D-8 external command gating | Hits R-8 across 10 doors and ~1,752 files; splitting into 3 child specs lets the cutover land first, followed independently by Child 2 and P0 Child 3 |

### Documents departed from

1. `ai/doors/system.md` ("Ze is a platform, and other people's code runs against it"): `"A process Ze forks from plugin { external <name> { run ...; } }, or one built on pkg/plugin/sdk"` -> updated to `bus { endpoint <name> { external { run ...; } } }`.
2. `ai/doors/plugin-infra.md` ("Who depends on it from outside"): `"plugin { external <name> { run ...; } }"` -> updated to `bus { endpoint <name> { external { run ...; } } }`.
3. `ai/doors/bgp-engine.md` ("Who depends on it from outside"): `"Which process sees which peer is the peer's attach process block"` -> updated to `bus { flow ... }` with optional generic `match` tags.
4. `ai/doors/bgp-plugins.md` ("Who depends on it from outside"): `"and the alias form internal rs { use bgp-rs }"` -> updated to `bus { endpoint rs { internal bgp-rs; } }`.
5. `docs/architecture/api/architecture.md` ("Owner ruling: endpoints and flows on the bus"): `"A BGP group or peer takes part in a flow by listing its name, and the list merges through groups as every list does."` -> updated to record Thomas's 2026-11-23 pure-bus ruling (zero `flow [ ... ]` in `bgp { }`, optional generic `match` tags in `bus { flow }`, built-in-only `from`, and built-in-only `ze start --plugin`).
6. `docs/architecture/config/syntax.md` ("The first pass: plugin declarations before the full parse"): `"PluginOnlySchema holds the roots plugin and environment and no other."` -> updated to `bus` and `environment`, `bus hub`, and updated `RetiredKeywordHint` rows.
7. `docs/architecture/config/yang-config-design.md` ("Decided, not built: modules compiled at build time"): `"The --plugin flag keeps starting the plugin it names"` -> updated to state that `ze start --plugin` accepts compiled-in built-in plugins only (external programs live in `bus { endpoint }`).
8. `docs/architecture/core-design.md` ("1. System Architecture"): `"Phase 2: explicit plugins (from config plugin { } block)."` -> updated to `bus { endpoint }` block.
9. `docs/architecture/api/process-protocol.md` ("Event Delivery"): `"the plugin subscribed to the type in that direction, and the peer's attach process <name> block grants it."` -> updated to `bus { flow }` (and fixes stale Stage 1 vs load-order prose).
10. `docs/architecture/api/commands.md` ("Event Subscription Commands"): `"the process subscribed to the event, and the peer's attach process <name> block grants that type in that direction."` -> updated to `bus { flow }`.
11. `docs/architecture/fleet-config.md` ("Unified Named Blocks"): `"All hub configuration uses explicit server and client keywords under plugin { hub { } }:"` -> updated to `bus { hub { } }` and listener `"bus-hub"`.
12. `docs/architecture/hub-architecture.md` ("Config File"): `"external acme {"` under `plugin {` -> updated to `bus { endpoint acme { external { ... } } }`.
13. `docs/architecture/system-architecture.md` ("Overview"): `"plugin { external ... } block"` -> updated to `bus { endpoint <name> { external { ... } } }`.
14. `docs/architecture/rib-transition.md` ("Owner ruling: a learned route needs the recipient's export policy"): `"The bgp-rib plugin is fed a peer's routes, and may send to a peer, only where that peer's attach process block grants it, as for any process"` -> updated to `bus { flow }` (with export policy governing per-peer advertisement).
15. `docs/architecture/bgp/structural-forwarding.md` ("Receive validation and cache changes"): `"the source's attach process <route server> { receive [ update-received ] }"` -> updated to `bus { flow }`.
16. `docs/architecture/api/capability-contract.md` ("Config Validation (✅ DONE)"): `"configured: attach process logger, attach process monitor - none carry either word"` -> updated to `bus { flow }` wording.
17. `docs/architecture/plugin/plugin-system.md` ("Registration fields"): `"The fast path then relays a source only when the source's attach block feeds this plugin its UPDATEs (receive [ update-received ]), and reaches a destination only when the destination grants this plugin send [ update ]"` -> updated to `bus { flow }` and `bus { endpoint }`.
18. `docs/architecture/doctor-and-health-checks.md` ("Plugin registration and delivery checks"): `"the attach process change (DeliveryDisagreement.Fix)"` -> updated to `bus { flow }` and `PluginDoctorState.RegistrationName`.
19. `docs/architecture/exabgp-bridge.md` ("A neighbor command is one of three things"): `"attach exabgp-bridge, once"` -> updated to drop `attach` from `create bgp peer` (delivered via `bus { flow }` + bridge `feed <ip>`).
20. `docs/architecture/update-building.md` ("Shared ownership admission"): `"when no attach process block grants the owner process the owning source's"` -> updated to `bus { flow }`.
21. `docs/guide/ddos-mitigation.md` ("FlowSpec mode sensor blindness"): `"A block of another name (plugin { internal <name> { use ddos-flowspec } })"` -> updated to `bus { endpoint <name> { internal ddos-flowspec; } }` with alias resolution via `PluginDoctorState.RegistrationName`.
22. `docs/config-reference.md` ("Plugin Loading"): `"Plugins are declared in the plugin { } block:"` -> updated to `bus { endpoint; flow; hub; }`.
23. `docs/exabgp/exabgp-migration.md` ("Key Differences"): `"attach process <name> { }"` -> updated to `bus { endpoint; flow; }` and pure `bgp { }`.
24. `docs/architecture/config/transaction-protocol.md` ("Phase 1: Verify"): `"plugin { internal rpki { use bgp-rpki } } names the process rpki, and its pki section still arrives"` -> updated to `bus { endpoint rpki { internal bgp-rpki; } }` and `bus { endpoint bgp-rpki { external { run "/opt/x"; } } }`.
25. `docs/guide/flowspec-protected-router.md` ("2. Update the active stored config"): `"Execution mode comes from the effective plugin declaration: use bgp-rib runs in-process even inside an external stanza, while run "ze plugin bgp-rib" forks a subprocess."` -> updated to mutually exclusive `internal` vs `external` under `bus { endpoint }` and `bus { flow }` permission wording.
26. `docs/architecture/api/update-syntax.md` ("Raw Passthrough Commands"): `"The peer must attach the program with send [ raw ]; no other send type permits it."` -> updated to `bus { flow ... { from bgp; command [ raw ]; } }`.
27. `docs/architecture/plugin/rib-storage-design.md` ("Overview"): `"the message names the peer and the forwarder, and tells the operator to attach the RIB to the peer with receive [ update-received ] or to take the peer down around the change."` -> updated to `bus { flow }` wording alongside `source_policy_reload.go`.
28. `docs/architecture/wire/nlri-bgpls.md` ("Consumer Decoding and Origination"): `"Collector peers negotiate the bgp-ls family and attach the plugin with state and refresh event delivery and UPDATE-send permission."` -> updated to `bus { flow }` wording for `bgp-ls-export` and `bgp-epe`.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Split per R-8 into 3 child specs | (a) Split into Child 1 (bus cutover + conversion + alias fixes), Child 2 (`\| implied` + generic refusals + D-6), Child 3 (P0 D-8 command gating); (b) 1 monolithic spec | Decided 2026-11-23: **(a)** 3-child split |
| C-2 | Bus scoping model (replacing D-5) | (a) Pure bus + optional generic message-tag filtering (`match <tag>`) with zero `flow` in `bgp {}` and zero BGP keywords in `bus {}`; (b) `flow [...]` in `bgp {}`; (c) BGP `group`/`peer` in `bus {}` | Decided 2026-11-23: **(a)** pure bus + optional generic message tags |
| C-3 | Namespace resolution in `bus { flow }` (W-1) | (a) Built-in publishers + aliased endpoints without `namespace` leaf; (b) Same + optional `namespace <ns>;` leaf to disambiguate multi-namespace publishers; (c) Namespace-only `from` | Decided 2026-11-23: **(b)** |
| C-4 | `--plugin` on `ze start` (W-8) & `from` direction (W-4) | (a) Built-in plugins only on `ze start --plugin`, and built-ins only in `bus { flow { from ... } }`; (b) Allow external `--plugin name=/path` and external `from` | Decided 2026-11-23: **(a)** built-in only for both |
| C-5 | `hub` placement (W-9) | (a) `bus { hub { ... } }` renaming listener `plugin-hub` -> `bus-hub`; (b) Top-level `hub { }` | Decided 2026-11-23: **(a)** `bus { hub }` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add failing functional tests (`test/plugin/bus-endpoint-flow-delivery.ci`, `test/parse/bus-cli-plugin-builtin-only.ci`, `test/parse/bus-flow-validation-refusals.ci`) and failing unit tests (`TestAliasedInternalPluginStateAndDoctor`, `TestBusFlowValidationBuiltInFromAndNamespace`).
2. **Phase: Init-time namespace & command registries + alias identity fixes** -- add `EventNamespaces`/`CommandTypes` to `registry.Registration` and `internal/core/events` (fixing `validate_sections.go` timing trap); fix `dispatch_state.go`, `doctor_snapshot.go`, `ddos/flowspec/doctor.go`, `data_rpc.go`, and the 5 sibling alias-keying bugs.
3. **Phase: `bus` YANG schema, loader, first-pass, validation & `--plugin` guard** -- delete `container plugin` and `container attach`, add `container bus` (`endpoint`, `flow`, `hub`), regenerate YANG glue, update `FirstPass`, `PluginOnlySchema`, `ExtractBusFromTree`, `ExtractHubConfig`, `validatedSections`, `RetiredKeywordHint`, `listener.go`/`graph.go` (`bus-hub`), `MergeCliPlugins` built-in guard, and `cmd/peer/create.go`.
4. **Phase: Tag-aware `DeliveryGraph`, BGP projection & ExaBGP migration** -- wire `BusConfig.Flows` + generic `match` tag filtering into `DeliveryGraph` and BGP event/send paths (deleting dead `ProcessBinding.Encoding`/`Format` and `extractBGPInlinePlugins`); update `internal/exabgp/migration/` and `bridge_neighbor.go`.
5. **Phase: Parsed-tree conversion & documentation** -- run parsed-tree converter across all ~1,752 tracked `.conf`, `.ci`, `.et`, interop, fixture, Go test, doc, and website files; update architecture/guide pages and `## Design decisions` rows; verify `TestRepositoryConfigsValidate` and scoped functional suites (`parse`, `plugin`, `reload`, `encode`, `exabgp`, `editor`, `managed`).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-9 has an implementation and passing test |
| No layering | Zero acceptance of `plugin {` or `attach process` remains; only `RetiredKeywordHint` names them |
| Pure bus isolation | `internal/component/plugin/` and `internal/component/config/` carry zero BGP imports and zero `peer`/`group` keywords in `bus` YANG |
| Alias security boundary | External endpoint named after a compiled-in plugin is refused state-key access (`!proc.Config().RunsExternalProgram()`) |
| Init-time validation order | `ValidateCustomSections` on `bus` passes inside `LoadConfig` before any reactor or plugin server exists |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line naming `bus` syntax; all `"plugin hub"` diagnostics updated to `"bus hub"` / `"bus-hub"` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero active `plugin {` or `attach process` outside `plan/` history and `retired.go` | `git grep -E 'plugin \{|attach process' test/ docs/ etc/ examples/` |
| All tracked configs validate | `./le test unit pkg path ./internal/component/config/cli/ run TestRepositoryConfigsValidate` |
| Scoped unit & functional suites green | `./le test unit pkg` on changed packages + `./le test functional parse`, `plugin`, `reload`, `encode`, `exabgp`, `editor`, `managed` |
| YANG glue & tier/grammar/doc gates green | `./le yang glue check`, `./le arch tier check`, `./le cli grammar`, `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| External endpoint state-key spoofing | `dispatch_state.go` and `doctor_snapshot.go` MUST check `!proc.Config().RunsExternalProgram()` before resolving `RegistrationName()` |
| External endpoint in `bus { flow { from } }` | Refused at parse/validation so external programs cannot spoof built-in event publishers |
| External binary via `ze start --plugin` | Refused before spawn, pointing operator to `bus { endpoint }` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Converter breaks `$PORT` or `refused=` block | Fix converter sentinel handling or triage the specific `refused=` test |
| Multi-peer test fails after `attach process` removal | Verify whether the test needs a `match` tag on `bus { flow }` or BGP export policy |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- Removing `attach > content` (`ProcessBinding.Encoding`/`Format`) also eliminates spurious peer session bounces caused by `reflect.DeepEqual` in `peerSettingsEqual` over dead leaves.
- Keeping `from` built-in-only eliminates Stage 1 wire churn (`W-4`/`A-2`) and makes `bus { flow }` 100% offline-validatable.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-keyed-on-a-field-its-consumer-ignores.md` | 2026-11-14 | `statestore.PluginKeyReadable`/`Writable`/`PluginList` (`plugin/server/dispatch_state.go`), `PluginDoctorState.Name` (`doctor_snapshot.go`), `ddos/flowspec/doctor.go`, and `data_rpc.go` (`runningOwnerError`) key on configured process alias instead of compiled registry name |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Pure protocol-agnostic `bus { endpoint, flow, hub }` + optional generic `match` tags; zero `flow` in `bgp {}` (reverses D-5, 2026-11-23) | `flow [...]` in `bgp {}` (old D-5); BGP `group`/`peer` leaves in `bus {}` | Keeps `bgp {}` pure BGP and `bus {}` protocol-agnostic while supporting optional secondary-field filtering (`source`, `group`, `peer`, etc.) |
| Built-in publishers + optional `namespace` leaf on `flow` (Q1 Option B, 2026-11-23) | No `namespace` leaf; namespace-only `from` | Handles multi-namespace publishers (like `l2tp`/`subscriber`) cleanly while letting `from` accept aliased built-in endpoints |
| Built-in-only `ze start --plugin` and built-in-only `from` in `bus { flow }` (Q2, 2026-11-23) | `--plugin name=/path` + offline validate flag; external-to-external flows | Keeps all 196 built-in `--plugin` tests intact, guarantees `config-validate-agrees-with-boot`, and eliminates Stage 1 wire changes |
| Move `hub` under `bus` (`bus-hub` listener, Q3 Option A, 2026-11-23) | Top-level `hub {}` | Follows D-7 and keeps external plugin TLS hub under one `bus` root |
| Split into 3 child specs per R-8 (2026-11-23) | Single monolithic spec | Lets the `bus` grammar cutover land cleanly first, unblocking Child 2 (`\| implied` + refusals) and P0 Child 3 (`D-8` AAA bypass fix) |

## Known Limitations
- Implied configuration display (`| implied`), generic plugin-declared narrow-grant refusals, AIGP explicit trigger, and D-6 intrinsic flows on `show event delivery` are owned by `plan/release/fixes/spec-bus-2-implied-display-and-grant-refusals.md`.
- Full external command permission gating (D-8) and closing the `ReservedInternalPrefix` AAA bypass on `dispatch-command` are owned by `plan/release/fixes/spec-bus-3-external-command-gating.md`.
- External-to-external bus flows are deliberately excluded (Q2, 2026-11-23).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: scoped package tests + `./le test functional parse`, `plugin`, `reload`, `encode`, `exabgp`, `editor`, `managed`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-user behavior
- [ ] Interop tests: N-A (no wire protocol change; existing interop configs converted)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-explicit-plugin-and-attach-config.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
