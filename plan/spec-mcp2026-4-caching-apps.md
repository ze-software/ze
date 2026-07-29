# Spec: mcp2026-4-caching-apps

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | spec-mcp2026-1-stateless-core |
| Phase | 4/4 |
| Deferral shard | `plan/deferrals/mcp2026-4-caching-apps.md` |
| Updated | 2026-07-28 |

Parent: `plan/spec-mcp2026-0-umbrella.md`. Depends on Phase 1 only, so it may run
concurrently with Phase 3 if capacity allows.

## Task

Two additive conformance items that Phase 1 deliberately left out so the
transport cutover stayed atomic.

**1. Cacheable results (SEP-2549).**

`ttlMs` and `cacheScope` are **required** fields on results returned by
`tools/list`, `prompts/list`, `resources/list`, `resources/read` and
`resources/templates/list`, via a new `CacheableResult` interface, plus
`server/discover`. Ze emits neither today: `tools/list` returns a bare
`{"tools": ...}` (`streamable_tools.go:27`) and `resources/list` a bare
`{"resources": ...}` (`resources.go:140`).

| Field | Meaning |
|-------|---------|
| `ttlMs` | Freshness hint in milliseconds, letting clients cache and stop polling |
| `cacheScope` | `"public"` or `"private"`, controlling whether shared intermediaries may cache |

`cacheScope` is a security decision, not a formatting one. Ze's tool list is
generated from the command registry and is currently identical for every
principal, but the auth modes carry per-identity scopes (`BearerListEntry.Scopes`,
`streamable.go:100-104`), so if the tool surface is ever scope-filtered the
correct scope is `"private"`. Decide explicitly and record the reasoning; a
wrong `"public"` here would let an intermediary serve one principal's tool list
to another.

Also in this item: `tools/list` **SHOULD** return tools in a deterministic order,
to improve client caching and LLM prompt-cache hit rates. Confirm whether
`generateTools` (`tools.go`) is already deterministic; map iteration in
`groupCommands` would make it not.

**2. MCP Apps as an official extension (SEP-1865).**

Ze already ships MCP Apps in the pre-extension shape from the `2026-01-26`
draft: tool descriptors carry `_meta.ui.resourceUri` pointing at `ui://` assets
(`tools.go:378`, `:386`), served through `resources/list` and `resources/read`
from an embedded FS (`plan/learned/682-mcp-5-apps.md`). In `2026-07-28` Apps is
a first-class extension identified as `io.modelcontextprotocol/ui`, negotiated
through the `extensions` capability map with a settings object (the versioning
page shows `{"mimeTypes": ["text/html;profile=mcp-app"]}`).

What must change:
- Advertise `io.modelcontextprotocol/ui` in `server/discover` capabilities.
- Read the client's declared UI extension settings from per-request `clientCapabilities` instead of the initialize-time `clientResources` session bit (`session.go:57`), which Phase 1 deleted.
- Verify `_meta.ui.*` field names and the `ui://` scheme against the current extension specification. Ze's shape came from a draft and may have drifted.
- Reconcile the resources capability gate: today `resources/list` and `resources/read` are gated on the client having declared `capabilities.resources` (`resources.go:144`, `streamable_tools.go:38-41`). Under per-request capabilities that gate moves; decide whether resources stay gated at all, since the spec treats `resources` as a normal server capability rather than something requiring client opt-in.

**Open design questions:** (all three resolved during design, 2026-07-28; the
answers and their evidence are recorded immediately below and carried into Key
Design Decisions)

1. Where do `ttlMs` values come from? The tool list changes only on daemon config reload and UI resources are immutable, so long TTLs are honest. Umbrella A-6 assumes no new config surface is needed; validate that rather than assuming it.
2. Is a config reload supposed to invalidate a client's cache? Without `subscriptions/listen` (umbrella A-4) there is no push, so `ttlMs` is the only invalidation lever. A very long TTL plus a config reload means stale tools at the client. This is the one place where A-4 has a user-visible cost, and it should be stated in the docs rather than discovered.
3. Does the UI extension settings object change what Ze serves (for example a client declaring only certain `mimeTypes`)?

**Design resolution (2026-07-28).**

*Question 1: two compile-time constants, no config surface.* The correct TTL for
a surface is a function of that surface's mutability, which the code knows and
an operator does not. Ze has exactly two mutability classes, so it gets exactly
two constants.

| Mutability class | Surfaces | `ttlMs` | Why this class exists |
|------------------|----------|---------|-----------------------|
| Registry-derived, changes at runtime | `tools/list`, `server/discover` | `60000` (60 s) | The command list is re-read from the dispatcher on every call (`cmd/ze/hub/command_meta.go:87-95`), so it changes when a plugin registers or a config reload lands |
| Embedded asset, changes only on upgrade | `resources/list`, `resources/read` | `3600000` (1 h) | `cachedResources` is built once at construction and documented immutable (`internal/component/mcp/streamable.go:137`, `:195`); the bytes come from `//go:embed` (`internal/component/mcp/ui/embed.go`), so they are fixed for the binary's lifetime |

No YANG leaf and no env var. A-1 is confirmed: exposing these would let an
operator set a lifetime that contradicts the server's actual invalidation
behaviour (a one-hour tool-list TTL on a list that changes at reload) with no
way for Ze to detect or reject the contradiction, which is the failure
`ai/rules/exact-or-reject.md` exists to prevent. `ai/rules/config-surface.md`
also answers "no" to every YANG-config question in its decision table for these:
an operator would not change them during capacity planning, they need no diff or
rollback, and they do not belong in `show configuration`.

*Question 2: the reload window is bounded at 60 s and is a documented,
specification-sanctioned mode, not a gap.* The caching page states it directly:
"A server **MAY** provide `ttlMs` without advertising `listChanged: true` in its
capabilities. In this case, the client relies entirely on TTL-based freshness."
So Ze relying on TTL alone is conformant rather than a shortfall of umbrella A-4.
The residual cost is that for up to 60 s after a reload a client may still offer
a removed tool. Two things bound the damage. The window is a minute, not the
five minutes the specification's own example uses. And the failure is
self-correcting: the same page says clients "**MAY** re-fetch before the TTL
expires if they have reason to believe the data has changed (e.g., receiving an
unexpected error on a tool call indicating the method was not found or the
parameters were invalid)", which is exactly what a call to a removed tool
produces. This goes in `docs/guide/mcp/overview.md` as a Known Limitation with
the 60 s bound named.

*Question 3: yes, the settings object gates `_meta.ui`, and the fallback is to
omit rather than reject.* The negotiated shape is
`{"mimeTypes": ["text/html;profile=mcp-app"]}` (versioning page, Extension
Negotiation), and "Each extension specifies the schema of its settings object;
an empty object indicates support with no additional settings." Ze serves one
bundle format: HTML sniffed from the file extension (`resources.go:28-30`,
`:47-53`). The rule Ze implements:

| Client's declared `io.modelcontextprotocol/ui` settings | Ze emits `_meta.ui`? |
|---------------------------------------------------------|----------------------|
| Extension absent from `extensions` | No |
| `{}` (support, no settings) | Yes |
| Present with no `mimeTypes` key | Yes |
| `mimeTypes` containing a media type whose base type is `text/html` | Yes |
| `mimeTypes` present and containing no `text/html` base type | No |

Omission is the correct fallback because the versioning page allows exactly two:
"the supporting party **MUST** either revert to core protocol behavior or reject
the request with an appropriate error." A tool descriptor without `_meta.ui` is a
valid core descriptor, so omitting is reverting to core behaviour; rejecting a
whole `tools/list` because the host cannot render HTML panels would break every
non-UI client for no benefit. Matching is on the base media type with the
parameter stripped, because `;profile=mcp-app` is a media-type parameter and a
client declaring bare `text/html` is declaring a superset.

Umbrella AC covered: AC-8.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/mcp/overview.md` - resources and UI sections
  → Constraint: every `.go` file under `internal/component/mcp/` names this file in its `// Design:` line, so the result-envelope change and the extension advertisement must be written into it in the same work (`ai/rules/design-doc-references.md`).
- [ ] `plan/learned/682-mcp-5-apps.md` - why the current UI shape is as it is
  → Decision: the draft shape it records (`resourceUri`, `permissions`, `csp`, `ui://`) survives verbatim into the official extension, so the bundle and the YANG `ze:ui-resource` walker are untouched. Only negotiation changes.
- [ ] `ai/rules/derive-not-hardcode.md` - cache metadata derived from the registry, not hand-written per tool
  → Constraint: `ttlMs` is a property of a surface's mutability class, not of a tool, so it is two named constants applied by one helper, never a per-tool or per-resource literal.

### Protocol Specification (Scope: protocol)
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/server/utilities/caching` - `CacheableResult`, `ttlMs`, `cacheScope`
  → Constraint: "Servers MUST include caching hints on results with `resultType: "complete"`" for six operations, of which Ze implements four. "Servers **MUST** provide a `ttlMs` value that is `>= 0`."
  → Constraint: interim `resultType: "input_required"` results "are not cacheable and carry no caching hints", and MRTR retries carrying `inputResponses` or `requestState` "**MUST NOT** be cached". `tools/call` therefore never carries hints, in either result shape.
  → Decision: "A server **MAY** provide `ttlMs` without advertising `listChanged: true`" makes TTL-only freshness a sanctioned mode, which is what resolves design question 2 without reopening umbrella A-4.
- [ ] `https://modelcontextprotocol.io/extensions/apps/overview` - the `io.modelcontextprotocol/ui` extension
  → Decision: A-3 confirmed. The page names `_meta.ui.resourceUri` "pointing to a `ui://` resource", `_meta.ui.csp` "to control what external origins the app can load resources from", and `permissions` "to request additional capabilities". That is field-for-field what `tools.go:376-386` already emits, so no reshaping is needed.
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning#extension-negotiation` - settings objects and the fallback rule
  → Constraint: identifiers "**MUST** follow the `_meta` key naming rules, with a mandatory prefix"; the worked example is `io.modelcontextprotocol/ui` with `{"mimeTypes": ["text/html;profile=mcp-app"]}`.
  → Decision: the fallback is "revert to core protocol behavior **or** reject". Ze reverts, meaning it omits `_meta.ui` and still lists and serves the tool.
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/server/tools` - deterministic ordering SHOULD, and `x-mcp-header`
  → Constraint: "Servers **SHOULD** return tools in a deterministic order (i.e., the same ordering across requests when the underlying set of tools has not changed)." Also "Tool names **SHOULD** be unique within a server."
  → Decision: the page states the tool set "**MAY** vary by the authorization presented on the request", so a scope-filtered tool list is an anticipated, normal design. That is what makes R-1 a live concern rather than a hypothetical, and it decides `cacheScope` (see Key Design Decisions).
  → Constraint: `x-mcp-header` is an optional `inputSchema` annotation. Ze emits none and this spec adds none; it is named here only so a future reader does not mistake its absence for an omission.

**Key insights:** (minimal context to resume after compaction)
- A-1, A-2 and A-3 all confirmed. Two `ttlMs` constants (`60000` registry-derived, `3600000` embedded-asset), `cacheScope: "private"` everywhere, and Ze's existing `_meta.ui` shape is already the official one.
- R-1 is resolved by construction, not by vigilance: one unconditional `"private"` means no code path exists that could pick `"public"` wrongly.
- **R-3 was NOT already satisfied, contrary to the phase's opening assumption.** Group ordering is fine, but action names are provably NOT unique within a group, and the duplicate is a real defect that predates this spec. See A-4 and Design Insights.
- The cacheable-result MUST is conditional on implementing the operation: `prompts/list` and `resources/templates/list` are in the specification's list and in neither Ze's `runMethod` nor this spec. A method Ze does not implement returns an error, not a result, so it carries no hints and breaches nothing.
- `tools/call` must carry no hints at all, in either result shape. It is the one place where adding the fields "for consistency" would be a conformance error.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [x] `internal/component/mcp/tools.go` (855L) - descriptor generation. `groupCommands` (`:112-240`) buckets commands by first and first-two tokens, then `sort.Slice` by unique `prefix` (`:235-237`) and `sortActions` by `name` (`:242-246`). `_meta.ui` emitted at `:376-386` with `resourceUri`/`permissions`/`csp`. `handcraftedTools` is a fixed 2-element slice literal (`:833-855`); `handcraftedNames()` (`:788-793`) is a set used only for the `skipNames` lookup at `:262`, so its map iteration never reaches output.
- [x] `internal/component/mcp/resources.go` (166L) - `listResources()` (`:64-88`) walks the embed FS with `fs.WalkDir`, which is lexical and therefore ordered. `readResource` (`:111-134`) sniffs MIME by extension. Traversal guard at `:101-107`. Both handlers gated on `sess.ClientSupportsResources()` (`:137-138`, `:144-145`), returning `-32601`. Bare envelopes at `:140` and `:165`.
- [x] `internal/component/mcp/streamable_tools.go` (368L) - `runMethod` (`:22-45`) dispatch; `tools/list` returns `{"tools": s.allTools()}` (`:27`). `allTools` (`:49-64`) copies `handcraftedTools` then appends `generateTools(groupCommands(s.cfg.Commands()), handcraftedNames())`.
- [x] `internal/component/mcp/ui/embed.go` (6L) - `//go:embed bgp-peer` into `FS`. One bundle, immutable for the binary's lifetime.
- [x] `cmd/ze/hub/command_meta.go` (230L) - the neutral command source behind BOTH the MCP lister and the API lister. `commandMetaSource` (`:66-124`) unions `d.Commands()` (`:95-112`) with `d.Registry().All()` (`:115-120`) and does NOT deduplicate. This is where A-4 breaks.
- [x] `cmd/ze/hub/service_mcp.go` (:78-110) - `mcpCommandLister` converts index-for-index (`:84-85`) with no dedupe, so duplicates reach `groupCommands` unchanged.
- [x] `internal/component/plugin/server/command.go` (:494-527) - `RegisterWithOptions` always writes `d.commands[key]` but calls `AddBuiltin` only when `!opts.PluginProxy` (`:524-526`), deliberately so "the plugin's own registration to route the command to the process" can claim the same name (`:521-523`).
- [x] `internal/component/mcp/auth.go` (:100-102) - `Identity.HasScope` exists and has no production caller in the component; scope enforcement is admission-only at `jwt.go:256-259`.

**Behavior to preserve:**
- Tool names, argument shapes and descriptor content generated from the YANG command registry.
- The `ui://` scheme, path-traversal prevention, and extension-based MIME sniffing (`resources.go`).
- Resource-not-found returning `-32602` (`resources.go:158-163`).
- The `_meta.ui` field names and the `ui://` scheme themselves: A-3 confirmed them unchanged, so `tools.go:376-386` and the YANG `ze:ui-resource` walker keep emitting exactly what they emit today. Only the negotiation around them changes.
- `handcraftedTools` first, generated tools after (`streamable_tools.go:60-63`). That order is part of the deterministic sequence AC-6 pins.

**Behavior to change:**
- Everything in the two items above, plus the command-list deduplication that AC-6 turns out to require (A-4).
- **Hidden plugin commands must stop reaching the tool list.** Found 2026-07-29 while landing the A-4 dedupe, and homed here because this phase already owns what `tools/list` contains. `CommandRegistry.All` returns every registration including hidden ones (`RegisteredCommand.Hidden`, `internal/component/plugin/server/command_registry.go:147`), and `Hidden` is referenced nowhere in the command-metadata path (`cmd/ze/hub/command_meta.go`, `cmd/ze/hub/api.go`, `internal/component/mcp/tools.go`). The registry's own `VisibleCommandEntries` skips them (`command_registry.go:445-447`) but feeds only the interactive completion tree (`cmd/ze/hub/main.go:972`, `cmd/ze/hub/session_factory.go:167`). So a command marked `Hidden: true` is suppressed from completion and help while still being advertised as an MCP tool and an API command, the opposite of what `ai/rules/cli-patterns.md` documents `Hidden` to mean. Decide during implementation whether the filter belongs in `buildCommandMeta`, fixing both surfaces at the shared source as the A-4 dedupe did, or whether either surface has a reason to keep seeing hidden commands. Deliberately NOT folded into the A-4 fix, which was scoped to the duplicate.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- HTTP POST `tools/list`, `resources/list`, `resources/read`, `server/discover`, each carrying per-request `clientCapabilities` including any declared `extensions`.

### Transformation Path
1. Phase 1 validation and per-request auth.
2. Descriptor or resource assembly from the registry / embedded FS. The registry path gains one step: the neutral command list is deduplicated by command name before grouping, first occurrence winning, because the dispatcher entry carries the YANG metadata and the plugin-registry entry does not.
3. Result envelope gains `ttlMs` and `cacheScope` alongside `resultType`, applied by one shared helper invoked by the four cacheable methods. The helper is deliberately NOT folded into the generic `ok` responder, because `tools/call` shares that responder and must carry no hints.
4. UI metadata emitted per the extension shape, gated on the client's declared UI extension settings; when the gate says no, the descriptor is emitted without `_meta.ui` and is otherwise identical.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Server ↔ shared intermediary | `cacheScope` decides whether a proxy may store the response. Ze answers `"private"` unconditionally, so no intermediary may share any MCP response across authorization contexts | Yes |
| Server ↔ client cache | `ttlMs` decides how long a stale result may be used: 60 s for registry-derived surfaces, 1 h for embedded assets | Yes |
| MCP ↔ embedded UI FS | `ui://` URI to `fs.ReadFile` after the traversal guard (`resources.go:101-107`, `:116`), unchanged | Yes |
| MCP ↔ neutral command source | `CommandLister` (`tools.go:84`) over `commandMetaSource` (`cmd/ze/hub/command_meta.go:66-124`); the union of dispatcher and plugin registry is deduplicated at the source, so the API lister (`cmd/ze/hub/api.go:272-297`) is fixed by the same change | Yes |
| MCP ↔ per-request client capabilities | The UI extension settings object is read from the `_meta` capabilities Phase 1 parses, never from session state | Yes |

### Integration Points
- `internal/component/mcp/discover.go` (Phase 1) - extension advertisement and its own cache fields
- `internal/component/config/yang/command.go` - the `ze:ui-resource` walker, unchanged: A-3 confirmed the extension shape did not move
- `cmd/ze/hub/command_meta.go` - the deduplication fix, shared with the API command lister
- `internal/test/cli/cmd_mcp.go` - asserts the new fields

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Cache hints are applied by one helper called from the four cacheable method handlers, not sprinkled into each envelope literal at `streamable_tools.go:27`, `resources.go:140` and `:165` |
| No unintended coupling | Yes | MCP gains no import of `cmd/ze/hub`; the deduplication lands in `command_meta.go`, which already owns the neutral list for both consumers. The UI gate reads the per-request capabilities Phase 1 already parses |
| No duplicated functionality | Yes | One cache-hint helper for four methods; one deduplication at the source rather than one per consumer |
| Zero-copy preserved where applicable | N-A | The MCP control plane assembles `map[string]any` and marshals it; it holds no pooled wire buffers, so `ai/rules/buffer-first.md` does not reach this path |
| Registration over hardcoding | Yes | Descriptors stay derived from the YANG command registry (`command_meta.go:66-124`); the extension identifier is one named constant shared by the advertisement and the gate; no plugin name or command spelling is added to MCP (`ai/rules/plugin-self-containment.md`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ttlMs` and `cacheScope` can be derived from static server knowledge with no new config | Umbrella A-6: the tool list changes only on config reload, UI resources are immutable | A YANG surface is needed for cache lifetimes | Design question 1 | confirmed: two compile-time constants, keyed to the two mutability classes evidenced at `cmd/ze/hub/command_meta.go:87-95` (re-read per call) and `internal/component/mcp/streamable.go:137`, `:195` plus `internal/component/mcp/ui/embed.go` (immutable). Umbrella A-6 is discharged |
| A-2 | Ze's tool surface is identical for every principal, so `cacheScope: "public"` would be safe | `allTools` (`streamable_tools.go:49-64`) takes no identity argument | `"public"` would leak one principal's tool list to another through an intermediary | Read `allTools` and every caller for an identity-dependent branch | confirmed as a fact, rejected as a basis for `"public"`. `CommandLister` is `func() []CommandInfo` (`tools.go:84`) and `allTools` (`streamable_tools.go:49-64`) take no identity; `Identity.Scopes` is set at `bearer.go:130`, `jwt.go:262` and `oauth.go:58` but its only reader, `Identity.HasScope` (`auth.go:100-102`), has no production caller, and scopes gate admission only (`jwt.go:256-259`). The tool list is identical per principal today. `"private"` is chosen anyway; see Key Design Decisions |
| A-3 | Ze's `_meta.ui` shape still matches the official extension | It was implemented against the `2026-01-26` draft (`plan/learned/682-mcp-5-apps.md`) | The UI metadata must be reshaped and the bundle may need changes | Read the current extension specification against `tools.go:370-390` | confirmed. The Apps overview names `_meta.ui.resourceUri` on a `ui://` resource, `_meta.ui.csp` for external origins, and `permissions` for extra capabilities; `tools.go:376-386` emits those three keys and no others. The extension's own specification directory is still `2026-01-26`, the draft Ze built against, so nothing drifted. The bundle and the YANG walker are untouched |
| A-4 | Action names are unique within a generated tool group, so sorting actions by `name` (`tools.go:242-246`) is a total order and the non-stable `sort.Slice` cannot reorder ties | The phase's opening research assumed this and asked for it to be checked | Ordering, the `action` enum and the action description are all non-deterministic across calls, and the enum is invalid JSON Schema | Trace `CommandInfo.Name` uniqueness from `groupCommands` back to its producer | **broken.** `RegisterWithOptions` skips `AddBuiltin` when `PluginProxy` is set (`internal/component/plugin/server/command.go:524-526`) precisely so a plugin may claim the same name (comment, `:521-523`); 106 production registrations set `PluginCommand` (for example `internal/plugins/isis/cmd_show.go:49` against constant `"show isis neighbor"` at `:35`, matching the YANG path `show isis neighbor` in `internal/plugins/isis/yang/ze-isis-cmd.yang:17-27`). `commandMetaSource` then unions both maps without dedupe (`cmd/ze/hub/command_meta.go:95-112`, `:115-120`) and `mcpCommandLister` converts index-for-index (`cmd/ze/hub/service_mcp.go:84-85`). The design was changed in response: deduplicate at the source, which restores uniqueness and with it the total order. **Fix landed ahead of this phase on 2026-07-29** at the owner's request, because the defect is live today and does not depend on the cutover: `buildCommandMeta` (`cmd/ze/hub/command_meta.go`) is now a pure function that dedupes on the lowercase name both sources key on, keeps the dispatcher entry, fills an empty dispatcher `Help` from the plugin `Description` rather than dropping it, and sorts by name. Covered by six tests in `cmd/ze/hub/command_meta_test.go`, mutation-verified (disabling the dedupe branch turns four of them red) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `cacheScope: "public"` is chosen by default and a future scope-filtered tool list leaks across principals | Review finds `"public"` with no reasoning recorded | **Resolved by construction.** `"private"` is emitted unconditionally by the single shared helper, so no code path exists that could choose `"public"`, and no future audit is owed when the tool list becomes scope-dependent. AC-5 asserts the absence of `"public"` across every surface, so reintroducing it fails the gate rather than passing review |
| R-2 | Long `ttlMs` plus no push invalidation leaves clients on a stale tool list after a config reload | An operator reloads config and the client keeps offering removed tools | Bounded at 60 s by the registry-derived constant, and self-correcting: a call to a removed tool returns an error, which the caching page names as a reason a client "MAY re-fetch before the TTL expires". Documented in `docs/guide/mcp/overview.md` as a Known Limitation with the bound named |
| R-3 | Tool ordering is non-deterministic through map iteration, silently defeating client caching | Two `tools/list` calls return different orders | **Live, and larger than ordering.** Group order is already safe: `groupCommands` builds from map iteration (`tools.go:146`, `:202`) but sorts on `prefix` (`:235-237`), and prefixes are map keys so the order is total. Actions are not safe, because A-4 is broken. Fix the duplicate at `cmd/ze/hub/command_meta.go`, which restores the total order on action names; then AC-6 and AC-13 hold the line. The upstream sources (`Dispatcher.Commands`, `internal/component/plugin/server/command.go:660-666`; `CommandRegistry.All`, `command_registry.go:422-431`) both range over maps and stay unordered, which is fine once the sort keys are unique |
| R-4 | Two distinct group prefixes collide into one tool name, breaching "Tool names SHOULD be unique within a server" | Two generated tools share a `name` | `toolName` (`tools.go:250-254`) maps both space and hyphen to underscore, so prefixes differing only in that separator would collide. Not verified to occur in the current command set, and this spec does not go looking; AC-13 simply asserts tool-name uniqueness alongside the enum check, so the collision cannot appear later without failing a gate |
| R-5 | The deduplication changes what the always-on API command lister returns, outside this phase's stated surface | `cmd/ze/hub/api.go` consumers see a shorter command list | Intended and correct: `apiCommandLister` (`cmd/ze/hub/api.go:272-297`) wraps the same source with the same index-for-index conversion, so it carries the same duplicates today. Fixing the source fixes both; fixing only MCP would leave a known-wrong API list. Named here so the review reads the API diff rather than being surprised by it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Clients cache stale tool lists, or an intermediary serves a cached response to the wrong principal (R-1). The deduplication additionally touches the always-on API command list (R-5). No dataplane impact |
| How is it reverted? | Single revert of the phase; the cache and extension fields are additive. The deduplication is separable and would revert with it, restoring the pre-existing duplicate rather than a new defect |
| Who else touches this path? | Phase 1 owns `server/discover`, which also carries cache fields. `cmd/ze/hub/command_meta.go` is shared with the API command lister |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Client POSTs `tools/list` | → | `allTools` → result envelope with `ttlMs` and `cacheScope` | `test/plugin/mcp-tools-list-cache-hints.ci` |
| Client POSTs `resources/read` on a `ui://` URI | → | `readResource` → content plus cache hints | `test/plugin/mcp-ui-resource-read.ci` |
| Client declaring the UI extension POSTs `server/discover` | → | extension advertisement | `test/plugin/mcp-discover-ui-extension.ci` |
| Client declaring NO UI extension POSTs `tools/list` | → | UI gate → descriptor emitted without `_meta.ui` | `test/plugin/mcp-ui-extension-fallback.ci` |
| Client declaring NO `capabilities.resources` POSTs `resources/list` | → | `resourcesList` with the client-opt-in gate removed | `test/plugin/mcp-resources-no-client-capability.ci` |
| Client POSTs `tools/list` twice against an unchanged daemon | → | deduplicated lister → `groupCommands` → identical bytes | `test/plugin/mcp-tools-list-deterministic-order.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `tools/list` | Result carries `ttlMs: 60000` and `cacheScope: "private"` |
| AC-2 | `resources/list` | Result carries `ttlMs: 3600000` and `cacheScope: "private"` |
| AC-3 | `resources/read` | Result carries `ttlMs: 3600000` and `cacheScope: "private"` |
| AC-4 | `server/discover` | Result carries `ttlMs: 60000` and `cacheScope: "private"` |
| AC-5 | Every cacheable result from every surface | `cacheScope` is `"private"`; no surface emits `"public"` |
| AC-6 | Two consecutive `tools/list` calls with unchanged registrations | Byte-identical `tools` array, including every `action` enum and every description string |
| AC-7 | `server/discover` | Advertises `extensions["io.modelcontextprotocol/ui"]` |
| AC-8 | Tool with a UI annotation, client declaring the UI extension with `{}` or with a `mimeTypes` list containing a `text/html` base type | Descriptor carries `_meta.ui` with `resourceUri` on the `ui://` scheme, plus `permissions` and `csp` when the YANG annotation supplies them |
| AC-9 | Client not declaring the UI extension at all | Descriptor omits `_meta.ui` entirely; the tool is still listed and still callable |
| AC-10 | `resources/read` on a path-traversal URI | Still refused (preserved from `plan/learned/682-mcp-5-apps.md`) |
| AC-11 | Client declaring the UI extension with a `mimeTypes` list containing no `text/html` base type | Descriptor omits `_meta.ui`; the request is not rejected |
| AC-12 | `resources/list` and `resources/read` from a client declaring no `capabilities.resources` | Both succeed; neither returns `-32601` |
| AC-13 | Any generated tool descriptor | No `action` enum contains a duplicate value, and no two tools in the result share a `name` |
| AC-14 | Every emitted `ttlMs` on every surface | An integer `>= 0`, never absent and never negative |
| AC-15 | `tools/call`, in both the `complete` and the `input_required` result shape | Carries neither `ttlMs` nor `cacheScope` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Lists tools twice and the client serves the second from cache | `tools/list` → `ttlMs` → client cache | `test/plugin/mcp-tools-list-cache-hints.ci` |
| 2 | Opens a tool's embedded UI panel | `tools/list` UI metadata → `resources/read` on `ui://` → HTML bundle | `test/plugin/mcp-ui-resource-read.ci` |
| 3 | Connects a host with no MCP Apps support and still uses every tool | `tools/list` without the UI extension → descriptors with no `_meta.ui` → `tools/call` | `test/plugin/mcp-ui-extension-fallback.ci` |
| 4 | Reloads config and sees the new command surface without restarting the client | reload → tool list changes → client TTL expires within 60 s → `tools/list` | `test/plugin/mcp-tools-list-deterministic-order.ci` plus the documented bound |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestListResultsCarryCacheHints` | `internal/component/mcp/streamable_test.go` | AC-1..AC-4, AC-14, table over the four implemented methods | |
| `TestCacheScopeIsAlwaysPrivate` | `internal/component/mcp/streamable_test.go` | AC-5, asserts `"public"` appears on no surface | |
| `TestToolsCallCarriesNoCacheHints` | `internal/component/mcp/streamable_test.go` | AC-15, both result shapes | |
| `TestToolOrderDeterministic` | `internal/component/mcp/tools_test.go` | AC-6, repeated generation from a shuffled lister, plus AC-13 uniqueness of enums and tool names | |
| `TestBuildCommandMeta_DedupesPluginProxiedCommand` | `cmd/ze/hub/command_meta_test.go` | A-4 fix: a name present in both the dispatcher and the plugin registry appears once, keeping the YANG-bearing entry | done 2026-07-29 |
| `TestBuildCommandMeta_DedupeIsCaseInsensitive` | `cmd/ze/hub/command_meta_test.go` | Dedupe matches on the lowercase key both sources store under | done 2026-07-29 |
| `TestBuildCommandMeta_PluginHelpFillsEmptyDispatcherHelp` | `cmd/ze/hub/command_meta_test.go` | An empty YANG description does not discard the plugin's help text | done 2026-07-29 |
| `TestBuildCommandMeta_KeepsPluginOnlyCommand` | `cmd/ze/hub/command_meta_test.go` | The dedupe does not over-match and shrink the tool surface | done 2026-07-29 |
| `TestBuildCommandMeta_OrderIsDeterministic` | `cmd/ze/hub/command_meta_test.go` | Same set in two input orders yields one output order (AC-6) | done 2026-07-29 |
| `TestBuildCommandMeta_UIResourceSurvivesDedupe` | `cmd/ze/hub/command_meta_test.go` | Parent-path UI annotation still attaches to the surviving entry | done 2026-07-29 |
| `TestUIExtensionAdvertised` | `internal/component/mcp/discover_test.go` | AC-7 | |
| `TestUIMetadataShape` | `internal/component/mcp/tools_test.go` | AC-8 against the current extension | |
| `TestUIMetadataGatedOnExtensionSettings` | `internal/component/mcp/tools_test.go` | AC-9 and AC-11, table over the five settings cases in the design resolution | |
| `TestResourcesUngatedOnClientCapability` | `internal/component/mcp/resources_test.go` | AC-12 | |
| `TestResourceTraversalStillRefused` | `internal/component/mcp/resources_test.go` | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ttlMs` protocol contract, any cacheable result | integer, `>= 0` ("Servers **MUST** provide a `ttlMs` value that is `>= 0`") | `0` (legal, but means immediately stale, so no Ze surface may emit it) | `-1` (breaches the MUST; assert no surface can emit a negative) | no maximum in the specification; Ze asserts `<= 3600000` so an edit cannot silently pin a client for longer than its longest declared class |
| `ttlMs` on registry-derived surfaces (`tools/list`, `server/discover`) | exact constant | `60000` | `59999` | `60001` |
| `ttlMs` on embedded-asset surfaces (`resources/list`, `resources/read`) | exact constant | `3600000` | `3599999` | `3600001` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mcp-tools-list-cache-hints` | `test/plugin/*.ci` | Tool list carries cache metadata | |
| `mcp-ui-resource-read` | `test/plugin/*.ci` | UI bundle fetched with cache metadata | |
| `mcp-discover-ui-extension` | `test/plugin/*.ci` | UI extension advertised | |
| `mcp-ui-extension-fallback` | `test/plugin/*.ci` | A host without MCP Apps still gets every tool, without `_meta.ui` | |
| `mcp-resources-no-client-capability` | `test/plugin/*.ci` | Resources served without a client opt-in declaration | |
| `mcp-tools-list-deterministic-order` | `test/plugin/*.ci` | Two `tools/list` calls return byte-identical tool arrays | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | No third-party MCP peer in-tree (umbrella Key Design Decisions). The substitute obligation stands: `internal/test/cli/cmd_mcp.go` asserts the cache fields and the extension shape from the specification text, not from Ze's emission | |

## Files to Modify
- `internal/component/mcp/tools.go` - UI metadata gate on the extension settings; the `_meta.ui` shape itself is unchanged (A-3)
- `internal/component/mcp/resources.go` - cache hints, removal of the client-opt-in capability gate
- `internal/component/mcp/streamable_tools.go` - list handler envelopes
- `internal/component/mcp/discover.go` - UI extension advertisement, own cache hints
- `cmd/ze/hub/command_meta.go` - deduplicate the dispatcher / plugin-registry union (A-4, R-3, R-5)
- `internal/test/cli/cmd_mcp.go` - assertions on the new fields
- `docs/architecture/mcp/overview.md`, `docs/guide/mcp/overview.md`, `ai/digests/mcp.md`

## Files to Create
- `test/plugin/*.ci` - the functional suite above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | A-1 confirmed: cache lifetimes are two compile-time constants keyed to mutability classes the code knows and an operator does not. No leaf added |
| YANG validation constraints | N-A | No new leaves |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | Yes | `internal/test/cli/cmd_mcp.go` asserts the new fields and declares the UI extension settings object |
| CLI grammar | N-A | No operator-facing command surface |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | Yes | `test/plugin/*.ci` |
| Pipe completeness | N-A | JSON-RPC over HTTP |
| Env var registration | No | A-1 confirmed: no env var, for the same reason no YANG leaf. A wrong value could not be detected or rejected by the server (`ai/rules/exact-or-reject.md`) |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module or certificate |
| Prometheus counters/metrics | No | The MCP component registers no metrics today: a grep for `metrics.` and `prometheus` over `internal/component/mcp/` excluding tests returns zero hits. This phase adds none |
| BGP family surface | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | No | A-1 confirmed: no YANG leaf and no env var, so no syntax changes |
| 3 | CLI command added/changed? | Yes | `docs/functional-tests.md` for the `ze-test mcp` driver flags |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` result shapes |
| 5 | Plugin added/changed? | No | MCP is a component |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/overview.md`, including the 60 s stale-tool-list window from design question 2 |
| 7 | Wire format changed? | Yes | `docs/architecture/mcp/overview.md` |
| 8 | Plugin SDK/protocol changed? | No | Untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/mcp-integration.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` MCP Apps row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/mcp/overview.md`, `ai/digests/mcp.md` |
| 13 | Route metadata keys added/changed? | N-A | Not routing |
| 14 | Prometheus counters added/changed? | No | None exist and none are added |
| 15 | Registered plugin/event/command/capability changed? | Yes | The UI capability moves into the `extensions` map |
| 16 | Changed source referenced by doc source anchors? | Yes | The overview anchors every MCP file; `cmd/ze/hub/command_meta.go` gains a behavior change worth an anchor check |
| 17 | Docs show config/CLI/API examples for this area? | Yes | Result-shape examples gain the cache fields |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - add cache fields to one list result and a failing wiring test asserting them.
2. **Phase: Cache hints** - the four implemented methods, through one shared helper that is not folded into the generic responder, so `tools/call` stays hint-free (AC-15).
3. ~~**Phase: Deterministic ordering** - deduplicate the command union at `cmd/ze/hub/command_meta.go`, then prove the total order holds (AC-6, AC-13). Confirm the API lister diff is the intended one (R-5).~~ **Landed early, 2026-07-29** (see A-4). The union is deduped and name-sorted in `buildCommandMeta`, with six tests. What remains for this phase is AC-13 (tool-name uniqueness after `toolName` collapses spaces and hyphens, R-4), which is a different key from the action names A-4 was about.
4. **Phase: UI extension** - advertise the identifier, read the per-request settings object, apply the five-case gate. The `_meta.ui` payload itself is untouched (A-3).
5. **Phase: Resources gate** - remove the client-opt-in gate now that capabilities are per-request and `resources` is a server capability.
6. **Phase: Consumers and docs** - test client assertions, guide, digest.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | `cacheScope` is justified, not defaulted (R-1), and `"public"` appears nowhere |
| Correctness | Tool order is stable across repeated generation, not merely stable within one call, and the deduplication is at the source rather than papered over in MCP |
| Correctness | `tools/call` carries no cache hints in either result shape (AC-15) |
| Naming | Cache fields use the spec's camelCase names verbatim; `_meta` extension keys use the reverse-DNS form |
| Data flow | Cache metadata is derived from a surface's mutability class, not hand-written per tool or per resource (`ai/rules/derive-not-hardcode.md`) |
| Rule: `ai/rules/no-fabrication.md` | The `_meta.ui` shape is checked against the current extension text, not assumed unchanged from the draft (A-3) |
| Rule: `ai/rules/diagnosis-before-fix.md` | The duplicate command is fixed at its owning layer, and the review reads the API command-list diff it also produces (R-5) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Cache fields on every implemented cacheable method | Table test over `tools/list`, `resources/list`, `resources/read`, `server/discover` |
| Deterministic order | `TestToolOrderDeterministic` fails when the deduplication is removed |
| No duplicate action enum values or tool names | Same test, asserted independently of ordering |
| UI extension advertised | `server/discover` response contains `io.modelcontextprotocol/ui` |
| UI fallback is omission, never rejection | `mcp-ui-extension-fallback.ci` lists and calls a UI-annotated tool with no extension declared |
| Full gate | `make ze-verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| `cacheScope` leak | Any `"public"` on any response. There should be none; the caching page warns that a `"public"` result from an authenticated endpoint "may be shared outside of the initial requests authorization context" |
| Stale authorization | A cached tool list outliving a revoked identity's access; bounded by the 60 s registry-derived TTL, and never shared across authorization contexts because every response is `"private"` |
| Not relying on `cacheScope` for access control | The caching page requires servers to "apply appropriate per-primitive access controls, and MUST NOT rely on `cacheScope` alone". Per-request authentication from Phase 1 remains the control; `"private"` is defence in depth, not the gate |
| Path traversal | Unchanged protections in `readResource` (`resources.go:101-107`) still hold after the capability gate is removed |
| Capability gate removal is not an authorization change | Confirm the removed gate was a client self-declaration, never an authorization check, and that per-request authentication still runs ahead of both resource handlers |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Wrong AC → DESIGN, correct AC → IMPLEMENT |
| The deduplication changes an API consumer's behavior beyond the duplicate | STOP. That is a wider blast radius than R-5 predicted; re-scope with the user |
| A spec MUST cannot be met as designed | STOP. Escalate per `ai/rules/rfc-compliance.md` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- **The cacheable-result MUST is conditional on implementing the operation.** The caching page lists six operations; Ze implements four of them. `prompts/list` has never been handled (`streamable_tools.go:22-45`, and umbrella Known Limitations) and `resources/templates/list` is not handled either. An unimplemented method returns a JSON-RPC error, and an error is not a result, so it carries no hints and breaches nothing. Recording this so a future reader does not read the six-item list as six obligations.
- **`tools/call` is the one place where adding the fields "for consistency" would be a conformance error.** It is absent from the cacheable list, `input_required` results "are not cacheable and carry no caching hints", and MRTR retries carrying `inputResponses` or `requestState` "**MUST NOT** be cached". This is why the cache helper is invoked per method rather than folded into the shared responder that `tools/call` also uses.
- **Staleness self-corrects for the tool list and does not for the UI bundle, which inverts the naive TTL intuition.** A stale tool list produces a failed call, and the caching page names exactly that error as a reason a client may re-fetch early. A stale UI bundle renders old HTML and produces no error at all. So the surface whose contents are "immutable" gets the *shorter* practical ceiling than pure immutability would suggest: one hour rather than a day, because the bytes are immutable for the process lifetime but not across an upgrade, and Ze has no push channel to say so (umbrella A-4).
- **Group ordering was already safe; action ordering was not, for a reason nothing in the MCP component could reveal.** `groupCommands` iterates maps (`tools.go:146`, `:202`) but sorts groups on `prefix` (`:235-237`), and prefixes are map keys, so that sort is total and the non-stable `sort.Slice` cannot bite. `sortActions` (`:242-246`) sorts on `name`, which is `full` minus a shared prefix, so it is total only if `full` is unique. It is not: the plugin-proxy mechanism (`internal/component/plugin/server/command.go:521-526`) deliberately allows one command name to live in both the dispatcher map and the plugin command registry, and `commandMetaSource` unions them without dedupe (`cmd/ze/hub/command_meta.go:95-120`). Reading only `internal/component/mcp/` makes the sort look total; the defect is two packages away.
- **The duplicate is worse than an ordering wobble.** The two entries carry different payloads: the dispatcher entry has YANG params, task-support, UI resource and selector flag (`command_meta.go:96-110`), the registry entry has only name and description (`:115-120`). So the duplicated action contributes two different help strings to `actionDescs` (`tools.go:295-301`) which are joined in tie order (`:308`), making the tool *description* vary between identical calls, and it contributes its name twice to `actionEnums` (`:293-296`), producing a JSON Schema `enum` with a repeated value. Both are invisible to a test that only compares array lengths.
- **`Identity.HasScope` is the exact hook a scope-filtered tool list would use, and it is currently unwired.** It exists at `internal/component/mcp/auth.go:100-102` with no production caller; scopes are enforced only as admission at `jwt.go:256-259`. The tools page explicitly contemplates the tool set varying "by the authorization presented on the request", so this is a designed-for future, not a hypothetical. That is what makes `"private"` the right answer today rather than a decision to revisit later.
- **Two gates that were previously conflated are now separate.** Serving `ui://` resources is ungated (they are non-secret embedded assets behind an authenticated endpoint), while emitting `_meta.ui` is gated on the UI extension. A non-UI client can therefore still fetch a `ui://` asset, which is harmless and avoids coupling a resource handler to an extension declaration.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `cacheScope: "private"` on every cacheable result, unconditionally | `"public"`, which A-2 confirms would be accurate today and which the caching page calls "appropriate for lists of tools, prompts, and resource templates when they are identical for all users" | The whole endpoint sits behind authentication, and the caching page warns that a `"public"` result from an authenticated endpoint "may be shared outside of the initial requests authorization context". `"public"` would buy one intermediary cache hit in exchange for a cross-principal leak that appears the moment anyone wires `Identity.HasScope` (`auth.go:100-102`) into the tool list, which the tools page explicitly anticipates. One unconditional value resolves R-1 by construction: there is no branch that could choose wrongly and no audit owed when the tool surface later becomes principal-dependent (`ai/rules/fail-closed-guards.md`). No MUST is narrowed by this: `"private"` is the strictly more restrictive value and the specification imposes no obligation to advertise `"public"` |
| Two `ttlMs` constants keyed to mutability class, not one value and not a config leaf | (a) a single value everywhere; (b) a YANG leaf or env var per `ai/rules/config-surface.md`'s default-to-config answer; (c) the specification's own 300000 example | The two classes have genuinely different lifetimes and evidence: registry-derived surfaces are re-read per call (`cmd/ze/hub/command_meta.go:87-95`), embedded assets are fixed at construction (`internal/component/mcp/streamable.go:137`, `:195`). A single value would be wrong for one of them. A config leaf is worse than absent: the right value is a function of internal mutability the operator cannot see, and a contradictory setting could not be detected or rejected (`ai/rules/exact-or-reject.md`). The specification's 300000 would put the reload window at five minutes, too long for a daemon whose command surface changes when a plugin is enabled |
| 60 s for registry-derived surfaces | 300000 as in the specification example; 0 to disable caching | 60 s is the bound this phase owes design question 2. It is long enough to stop an agent re-listing tools every turn and short enough that an operator sees a reload reflected within a minute with no push mechanism. `0` would satisfy the MUST while defeating the entire point of SEP-2549 |
| 3600000 for embedded-asset surfaces, rather than a day or more | 86400000 (24 h), justified by the assets being immutable for the process lifetime | Immutable for the *process* lifetime is not immutable across an upgrade, and a stale UI bundle produces no error for a client to recover from. A day of stale panels after an appliance update is a worse surprise than the handful of extra fetches an hour costs. 3600000 also stays far inside int32, so no client-side numeric handling is at risk |
| The UI extension fallback is omission, never rejection | Rejecting `tools/list` with an error when the client has not declared the extension | The versioning page permits exactly two fallbacks: "revert to core protocol behavior or reject the request with an appropriate error". A descriptor without `_meta.ui` is a valid core descriptor, so omission is the revert branch. Rejecting would break every non-Apps client for no gain |
| `mimeTypes` matching is on the base media type, treating bare `text/html` as compatible with `text/html;profile=mcp-app` | Exact string equality against `text/html;profile=mcp-app` | `;profile=mcp-app` is a media-type parameter, so a client declaring bare `text/html` is declaring a superset, and exact matching would refuse a host that can render Ze's bundle. Ze serves HTML sniffed by extension (`resources.go:28-30`, `:47-53`), so base-type matching is exactly the right granularity |
| Remove the client-opt-in gate on `resources/list` and `resources/read` entirely | Port the gate to per-request capabilities, keeping `-32601` when the client omits `capabilities.resources` | `resources` is a server capability advertised in `server/discover`; nothing in `2026-07-28` conditions the method's existence on a client declaration. Ported to per-request capabilities the gate would be worse than useless: a client that omitted the declaration on one request would get "method not found" for a method that exists, which is the per-connection variance the tools page forbids in spirit. It is not a security relaxation either, because the gate was a self-declaration, never an authorization check, and per-request authentication from Phase 1 still runs ahead of both handlers |
| Deduplicate the command union at `cmd/ze/hub/command_meta.go`, first occurrence winning | (a) dedupe inside `groupCommands` so MCP is self-sufficient; (b) make `sortActions` sort on `full` to force a total order; (c) accept the duplicate and assert only array length | The owning layer is the one that performs the union (`ai/rules/diagnosis-before-fix.md`); the API lister reads the same source and carries the same defect (R-5), so fixing there fixes both. (b) does not work: duplicates have identical `full`, so no key derived from `full` breaks the tie. (a) would leave the API wrong. First occurrence wins because `d.Commands()` is appended first (`command_meta.go:95`) and carries the YANG params, task-support and UI resource the registry entry lacks |

## Known Limitations

- Without `subscriptions/listen` (umbrella A-4), `ttlMs` is the only cache-invalidation lever, so a config reload leaves clients on a stale tool list for **up to 60 seconds**. This is explicitly sanctioned rather than a gap: the caching page states that a server "**MAY** provide `ttlMs` without advertising `listChanged: true`", in which case "the client relies entirely on TTL-based freshness". The failure is also self-correcting, because calling a removed tool returns an error and the same page allows clients to re-fetch early on exactly that signal. Documented in `docs/guide/mcp/overview.md` with the bound named, so an operator meets it in the guide rather than in production.
- A UI bundle replaced by a daemon upgrade may render from a client cache for **up to one hour**, and unlike a stale tool list this produces no error for the client to recover from. Bounding it further would mean content-addressed `ui://` URIs, which changes the resource-naming scheme and is out of scope for a conformance phase.
- `prompts/list` and `resources/templates/list` appear in the caching page's list of operations that must carry hints, and Ze implements neither. This is not a caching gap: an unimplemented method returns a JSON-RPC error rather than a result. Prompts remain out of scope for the same reason the umbrella records, and nothing here changes that.
- Tool-name collisions through `toolName` (`tools.go:250-254`), which maps both space and hyphen to underscore, are guarded by AC-13 but not eliminated. If two group prefixes ever differ only by that separator the gate fails and the naming is fixed then; this phase does not audit the current command set for such a pair (R-4).

## RFC Documentation (Scope: protocol)

Add `// MCP 2026-07-28 server/utilities/caching Section X: "<quoted requirement>"`
above the cache-field emission, and the equivalent extension citation above the
UI metadata and advertisement paths. At minimum quote: the operation list that
must carry hints, "Servers **MUST** provide a `ttlMs` value that is `>= 0`", the
`"private"` scope definition, the exclusion of `input_required` and MRTR-retry
results, the tools page's deterministic-order **SHOULD**, and the versioning
page's "revert to core protocol behavior or reject" fallback rule above the UI
gate.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved

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
- [ ] **Commit B:** `git rm plan/<spec>` only
