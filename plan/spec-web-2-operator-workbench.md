# Spec: web-2 -- RouterOS-Style Operator Workbench and Related Tool Overlays

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 8/8 (browser verification pending) |
| Updated | 2026-04-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `ai/rules/planning.md` - spec workflow and completion rules.
3. `ai/patterns/web-endpoint.md` - web handler, template, HTMX, and route conventions.
4. `docs/architecture/web-interface.md` and `docs/architecture/web-components.md` - current web UI architecture.
5. `docs/architecture/config/yang-config-design.md` - YANG extension and command-tree architecture.
6. `docs/architecture/api/commands.md` - command taxonomy, request identity, authz, and operational command contracts.
7. `internal/component/web/{fragment.go,handler.go,handler_admin.go,render.go,cli.go}` - current config/admin UI.
8. `internal/component/config/yang/{command.go,validator_registry.go}` and `internal/component/config/yang_schema.go` - current YANG metadata extraction patterns.
9. `internal/component/config/yang/modules/ze-extensions.yang` - existing `ze:` extension definitions.
10. `cmd/ze/hub/main.go` web startup route registration.

## Task

Create an experimental V2 web UI that becomes the active web UI in situ when development starts, while keeping an explicit rollback switch that restores the current Finder-first UI if the experiment fails. The V2 UI is a RouterOS/WebFig-inspired workbench:

- Persistent top navigation for identity, global actions, search, CLI, and pending changes.
- Persistent left navigation for main operator sections such as Dashboard, Interfaces, Routing, Policy, Firewall, Services, System, Tools, and Logs.
- Table-first presentation for named configuration data, especially BGP peers, groups, families, interfaces, policies, and firewall-like rule lists.
- Contextual tools attached to config nodes and table rows using Ze YANG metadata under the `ze:` extension namespace.
- Operational commands run from these related tools must render their output in an overlay, not replace the current config/table workspace.
- The work must preserve the existing CLI bar, per-user draft sessions, diff/commit flow, authentication, CSP posture, and HTMX/server-rendered architecture.

This spec is intentionally broader than a visual restyle. It defines the metadata, routing, execution, rendering, and test contract needed to make the UI behave like an operator workstation rather than a schema browser.

The central UX goal is the engineer's operational loop: make a change, check if it worked, and decide the next action without changing pages or losing context. Innovation in this spec means compressing that loop into one workspace, not adding decorative UI.

## Experiment Model

The V2 workbench is built behind an opt-in `ze.web.ui=workbench` switch through Phases 1-3, then becomes the default `/show` UI at the end of Phase 4 once the BGP change-and-verify loop ships and passes the Promotion Criteria. After the flip, the old Finder implementation remains reachable only via `ze.web.ui=finder` as an emergency rollback. There is no permanent dual UI: Phase 7 ends with one UI removed.

| Rule | Requirement |
|------|-------------|
| Phased default switch | `/` and `/show/<path>` render Finder by default through Phases 1-3. The default flips to V2 at the end of Phase 4, only when the BGP change-and-verify loop ships and passes its acceptance tests. Until that flip, V2 is reachable only via opt-in `ze.web.ui=workbench`. |
| Rollback after the flip | After the Phase 4 flip, `ze.web.ui=finder` restores Finder. The variable is read at hub startup; switching it requires restarting the hub. This is acceptable because rollback is an emergency control, not a runtime preference. |
| Single active UI per process | The hub serves exactly one UI mode per process. There is no per-request, per-user, or per-cookie switching. |
| Shared domain logic | V2 may have separate layout/templates, but it must reuse the same editor sessions, schema, dispatcher, auth, commit flow, and validation. |
| No permanent dual UI | The spec ends with a promotion decision: delete Finder if V2 succeeds, or delete V2 and restore the Finder default if it fails. |
| Measurable outcome | V2 acceptance is judged against the Promotion Criteria below, not subjective impressions. |
| Deletion plan | Once V2 is promoted, a follow-up cleanup must remove obsolete Finder templates, CSS, tests, route branches, and rollback mode. |

### UI Mode Contract

| Phase | Default `/show/<path>` | `ze.web.ui=workbench` | `ze.web.ui=finder` |
|-------|------------------------|-----------------------|--------------------|
| Phases 1-3 (V2 incomplete) | Finder | V2 workbench (incomplete; opt-in for development only) | Finder (explicit, identical to default) |
| Phase 4 onward (V2 reaches parity) | V2 workbench | V2 workbench (explicit, identical to default) | Finder (emergency rollback) |
| After promotion | V2 workbench (only UI; rollback removed) | (variable removed) | (variable removed) |

The flip from "Finder default" to "V2 default" is a single commit at the end of Phase 4, gated by the Promotion Criteria. The variable is read once at hub startup; switching it requires restarting the hub. Documentation should call `ze.web.ui` an experiment rollback control, not a long-term dual-interface mode.

### Promotion Criteria

V2 promotion is gated by `test/web/workbench-bgp-change-verify.wb`. Every criterion below must hold against the canonical BGP peer edit -> review -> commit -> verify scenario:

| Criterion | Threshold |
|-----------|-----------|
| Full-page navigations across the canonical loop | 0 |
| Selected peer row remains identifiable in DOM after every step | yes |
| User clicks for the canonical loop (open peer, edit one field, run peer detail, commit, rerun peer detail) | <= 6 |
| Pending-change marker visible after edit and before commit | yes |
| Tool overlay opens, closes, and reruns without an HTMX-boost full reload | yes |
| Authz failures render in the overlay error state, not as HTTP 500 | yes |

Failing any criterion blocks the Phase 4 default flip and blocks Phase 7 promotion. If V2 cannot meet these criteria, the experiment is rejected: the default stays at Finder and V2 is removed in cleanup.

## Operator Workflow Goal

The workbench is designed around a repeated operational loop.

| Step | Engineer Intent | V2 Behavior |
|------|-----------------|-------------|
| 1. Find object | Locate the peer, interface, policy, route, or service being changed. | Left navigation lands on a table. Search/filter narrows rows without leaving the table. |
| 2. Inspect current state | Understand configured and runtime state before touching anything. | Row detail drawer shows config fields. Related tools show runtime output as overlay/drawer. |
| 3. Make change | Edit the minimum fields needed. | Inline edits and detail drawer edits update the draft session and mark affected rows. |
| 4. Review impact | See what will change before commit. | Commit bar and row markers expose pending changes in the same workspace. |
| 5. Commit/apply | Apply the draft. | Commit remains one click away without navigating to a separate page. |
| 6. Verify | Check runtime state, logs, warnings, counters, and peer/session status. | Related tools rerun in-place; overlays can be rerun or pinned while the table remains visible. |
| 7. Decide next action | Keep, adjust, rollback, or investigate. | The same row provides edit, operational tools, diff, and CLI context. |

### Friction Targets

| Workflow | Target |
|----------|--------|
| Edit one BGP peer field and run peer detail | No full page navigation; table remains visible; detail/tool output appears beside or over the same row context. |
| Commit a peer change and verify session state | Commit action and peer detail/statistics are reachable from the same BGP peer workspace. |
| Investigate failed peer session | Peer row exposes configured fields, runtime detail, capabilities, statistics, warnings, errors, and CLI context without visiting `/admin`. |
| Compare before/after output | Related tool overlay supports rerun and, where cheap, pinning the previous result until dismissed. |
| Recover from bad change | Pending change marker, diff, discard, and relevant runtime checks stay visible in the same workspace. |

## Scope

### In Scope

| Area | Description |
|------|-------------|
| Workbench shell | Top bar plus left navigation, with the existing content area converted from Finder columns to section/table/detail layouts. |
| Phased V2 entry point | V2 is opt-in via `ze.web.ui=workbench` through Phases 1-3 and becomes the default for `/show` after the Phase 4 flip; Finder is the default through Phases 1-3 and the emergency rollback after the flip. |
| Rollback mode | A simple mode switch can restore Finder if the experiment fails or blocks testing. |
| Change-and-verify workflow | BGP peer workflow must support edit, review, commit, and runtime verification from one workspace. |
| Table-first lists | Named YANG lists render as tables by default, not only when they have `unique` constraints. |
| Related tool metadata | New `ze:related` YANG extension marks config nodes with context-relevant operational tools. |
| Tool command resolution | Related tool execution resolves server-side from trusted YANG metadata and current config context. Browser requests cannot submit arbitrary command strings for related tools. |
| Tool output overlay | Command results render in a dismissible overlay with command name, output, error state, copy/rerun affordance, and preserved underlying workspace. |
| BGP first coverage | BGP peer list/detail receives day-one related tools backed by existing commands: peer detail, capabilities, statistics, flush, teardown, BGP summary/health, warnings, and errors. Route refresh or soft clear are added only if corresponding commands exist or are added in this spec. |
| Admin tree cleanup | Web admin navigation stops using the static `BuildAdminCommandTree` map and derives its tree from the same YANG command tree used by CLI/dispatcher. |
| Documentation | Web guide and architecture docs updated to describe workbench navigation and related tools. |
| Tests | Unit, integration, and web browser tests cover metadata extraction, command resolution, authz behavior, overlay rendering, and the BGP user path. |

### Out of Scope

| Area | Reason |
|------|--------|
| Immediate deletion of the current UI | Finder remains in the codebase as the default through Phases 1-3 and as the rollback target after the Phase 4 flip; Phase 7 deletes whichever UI is rejected. There is no period where both UIs are normal parallel UIs. |
| Full visual redesign of every page in one patch | This work should land in phases; L2TP custom pages and portal pages can adapt after core components stabilize. |
| Client-side table engine | Preserve server-rendered HTML and HTMX patterns. Search/sort/filter may use server endpoints or simple form submissions first. |
| Arbitrary user-defined tool commands from config | Related tools are trusted schema metadata, not operator-editable config, to avoid command injection and authz confusion. |
| Replacing the CLI bar | The bottom CLI remains a first-class workflow and should become better synchronized with table/detail context. |
| Structured rendering for every command output | The initial related-tool implementation displays command output safely as text. Rich per-command renderers are future work. |
| Fleet/multi-router switching | Related to `spec-web-1-identity`, but not required for this feature. |

## Related Work

| File | Relationship |
|------|--------------|
| `plan/spec-web-1-identity.md` | Router identity display would improve the top bar but does not block this spec. |
| `plan/spec-l2tp-11-web.md` | Shows how a custom web page, chart feeds, and dispatch-backed actions can live under the web component. |
| `plan/spec-mcp-5-apps.md` | Uses YANG metadata to advertise UI resources for tools. **Hard prerequisite for Phase 2:** confirm whether `ze:related` and the MCP UI-resource extension can share one YANG metadata model or must remain separate. If shared, design `ze:related` so MCP can consume it. If separate, document the divergence reason in Design Decisions and own the maintenance cost. Two unaligned extensions for "operator tool metadata" is a maintenance trap and must not be entered silently. |
| `plan/learned/454-web-htmx-architecture.md` | Captures the HTMX/server-rendered architecture this work must preserve. |
| `plan/learned/471-web-4-admin-commands.md` | Introduced web operational command execution via `CommandDispatcher`. |
| `plan/learned/474-web-admin-finder.md` | Explains current admin Finder reuse and the limitation that results replace detail content. |
| `plan/learned/486-cli-nav-sync.md` | Defines the current CLI/web context synchronization and why list-level context needs care. |
| `plan/learned/395-yang-command-tree.md` | Defines `ze:command` and the YANG command-tree source of truth. |
| `plan/learned/525-mcp-auto-tools.md` | Establishes that command surfaces should derive from the command registry, not hardcoded per consumer. |

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/web-interface.md` - web server, auth, URL scheme, editor session, admin commands, looking glass separation.
  -> Constraint: Existing web UI is HTTPS-only, authenticated, and server-rendered; related tools must preserve the same security headers and auth flow.
  -> Decision: Related tool execution belongs inside `internal/component/web/`, not the public looking glass.
- [ ] `docs/architecture/web-components.md` - HTMX fragments, one template per visual concern, OOB swaps, list table, CLI bar.
  -> Constraint: New workbench components must be rendered by Go templates and updated with HTMX; no new client-side rendering framework.
  -> Decision: Tool output uses a new overlay fragment and OOB swap, matching diff/add/login overlay patterns.
- [ ] `docs/architecture/config/yang-config-design.md` - YANG modules and extension interpretation.
  -> Constraint: Ze-specific behavior belongs in `ze-extensions.yang`; implementation code interprets extensions but YANG remains the declaration layer.
  -> Decision: Related tools are declared as `ze:related` metadata on config schema nodes.
- [ ] `docs/architecture/api/commands.md` - command taxonomy, read/write semantics, request metadata, report bus.
  -> Constraint: Caller identity is injected by trusted transport wiring; browser requests must not supply identity fields.
  -> Decision: Tool execution reuses `CommandDispatcher(command, username, remoteAddr)`.
- [ ] `ai/patterns/web-endpoint.md` - web route/handler conventions.
  -> Constraint: New handlers must parse URL, validate paths, negotiate response format where applicable, set content type before writing, and use authenticated routes.
  -> Decision: Add a dedicated related-tool POST route instead of overloading `/admin/<path>`.
- [ ] `ai/rules/data-flow-tracing.md` - full data-flow trace required before implementation.
  -> Constraint: Tool metadata, command resolution, dispatch, and overlay rendering must be traced end to end.
- [ ] `ai/rules/spec-no-code.md` - spec must use prose and tables, not code snippets.
  -> Constraint: Implementation examples are described in tables/prose only.

### Source Files

- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - existing extension namespace.
  -> Constraint: Add extension documentation here; keep extension purpose narrow and user-visible.
- [ ] `internal/component/config/yang/command.go` - command tree builder and `ze:command` extraction.
  -> Constraint: Command tree already maps WireMethod to CLI paths; related tools should not duplicate command discovery logic.
- [ ] `internal/component/config/yang_schema.go` - config schema conversion and extension extraction.
  -> Constraint: Flat extension extraction is the established pattern for config-node metadata.
- [ ] `internal/component/config/schema.go` - schema node structs.
  -> Constraint: If related tools are needed at render time, `ContainerNode`, `ListNode`, and possibly `LeafNode` need metadata fields.
- [ ] `internal/component/web/fragment.go` - `FragmentData`, table/list construction, current Finder columns.
  -> Constraint: Current view data is config-path based; related tools must be built from the current path and editor working tree.
- [ ] `internal/component/web/handler_admin.go` - current operational command execution.
  -> Constraint: Existing admin execution accepts URL path as command string and replaces the detail panel; related tools need stricter command construction and overlay rendering.
- [ ] `internal/component/web/handler.go` - URL parsing and validation.
  -> Constraint: Add a top-level `tools` prefix or direct route registration with equivalent path validation.
- [ ] `internal/component/web/render.go` - template loading and renderer data model.
  -> Constraint: New templates must be included in the fragment template set.
- [ ] `internal/component/web/templates/component/detail.html` - detail priority chain.
  -> Constraint: Workbench detail/tool areas need explicit template branches rather than ad hoc HTML from handlers.
- [ ] `internal/component/web/templates/component/list_table.html` - existing editable table.
  -> Constraint: Extend or replace as a reusable table component; avoid one-off BGP table templates unless needed for operational columns.
- [ ] `internal/component/web/assets/style.css` - current layout, finder, table, modal, command result styles.
  -> Constraint: New workbench shell must use stable dimensions and avoid nested-card layouts.
- [ ] `internal/component/web/assets/cli.js` - only current custom JS, handles theme, overlays, rename/add behavior, CLI context.
  -> Constraint: Any new JS must be minimal, event-delegated, and only where HTMX cannot express the behavior.
- [ ] `cmd/ze/hub/main.go` - web route registration and dispatcher wiring.
  -> Constraint: Related tool routes must be registered through the same auth wrapper as config/admin/CLI.
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - BGP config nodes.
  -> Decision: Day-one `ze:related` annotations should start on `bgp`, `bgp/peer`, and `bgp/group/peer`.
- [ ] `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` - peer commands.
  -> Constraint: Peer lifecycle/introspection command paths already exist and must be reused.
- [ ] `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - show commands.
  -> Constraint: Global health/warning/error tools already exist as show commands.

### External UX References

- [ ] MikroTik WinBox documentation: `https://help.mikrotik.com/docs/spaces/ROS/pages/328129/WinBox?src=contextnavpagetreemode`
  -> Observed pattern: persistent left-side feature tree, top toolbar, tabular object windows, row filtering, and compact operational affordances.
  -> Decision: Borrow the operator model of section-first navigation and tables, but keep a browser-native single workspace.
- [ ] MikroTik WebFig documentation: `https://help.mikrotik.com/docs/spaces/ROS/pages/328131/WebFig?src=contextnavpagetreemode`
  -> Observed pattern: web UI retains RouterOS section hierarchy and exposes terminal/operational access alongside configuration.
  -> Decision: Preserve Ze's CLI bar and add related operational tools instead of making the browser a passive form editor.
- [ ] RouterOS BGP documentation: `https://help.mikrotik.com/docs/spaces/ROS/pages/328220/BGP`
  -> Observed pattern: BGP configuration and runtime inspection are organized around peers, sessions, filters, templates, and status-oriented command output.
  -> Decision: Make BGP peer tables the first proof point for row-level related tools.
- [ ] WinBox BGP screenshot examples from operator guide: `https://www.psychz.net/images/winbox-3.png`, `https://www.psychz.net/images/winbox-4.png`, `https://www.psychz.net/images/winbox-5.png`, `https://www.psychz.net/images/winbox-6.png`
  -> Observed pattern: named lists are dense tables with direct access to peer filters, actions, and status.
  -> Decision: Ze BGP peer/group/policy screens should feel like operational tables with contextual tools, not directory columns.
- [ ] RouterOS firewall filter documentation: `https://help.mikrotik.com/docs/spaces/ROS/pages/48660574/Filter?src=contextnavpagetreemode`
  -> Observed pattern: rule-oriented areas rely on ordered tables, counters, enable/disable actions, and diagnostic views.
  -> Decision: The table contract must support ordered rule lists later, even though BGP peers are the day-one implementation target.
  -> Constraint: Do not copy floating WinBox MDI windows into the web UI; a stable single-page workbench is better for browser ergonomics.

### RFC Summaries

- Not applicable. This is a web/UI and command metadata feature, not protocol behavior.

**Key insights:**
- Current web UI is technically strong but exposes the YANG tree as the primary navigation model. RouterOS-style operators expect task sections and tables first.
- The better interface is not just tables. It is the ability to change a configuration object and run the relevant operational checks without moving to another page.
- Existing command execution and authz can be reused, but related tools must avoid accepting arbitrary command text from the browser.
- Existing `ze:command` binds command nodes to WireMethod. Related tools should reference or validate against this command surface rather than hardcoding undocumented command names in templates.
- Table row tools need per-row context. For BGP peers, the row key may be a friendly name while operational peer selectors usually need `connection/remote/ip`; the metadata must support fallback from configured field value to list key.
- Overlay output must preserve the config workspace and draft state. Replacing `#detail` with command output is exactly what operators complained about implicitly when asking for related tools.

## Current Behavior

### Source Files Read

- [ ] `internal/component/web/templates/page/layout.html` - grid with breadcrumb, content area, notification bar, commit bar, CLI bar, diff/error panels.
- [ ] `internal/component/web/templates/component/finder.html` - current Finder column UI, selected item highlighting, list counts, add entry buttons.
- [ ] `internal/component/web/templates/component/oob_response.html` - full and HTMX content use `main-split`, Finder columns, and detail panel.
- [ ] `internal/component/web/templates/component/detail.html` - detail renders command result, command form, list table, fields, or hint.
- [ ] `internal/component/web/templates/component/list_table.html` - list table supports rename, key link, editable unique-field cells, delete, and add.
- [ ] `internal/component/web/templates/component/command_result.html` - command output renders as a result card with preformatted text and error class.
- [ ] `internal/component/web/fragment.go` - builds `FragmentData`, Finder columns, context heading, list table from `unique` fields, and field metadata.
- [ ] `internal/component/web/handler_admin.go` - admin tree is a static map; POST dispatches URL path as a command and replaces detail with command result.
- [ ] `internal/component/web/handler.go` - validates path characters and classifies `/show`, `/monitor`, `/config`, `/admin`, `/portal`, `/login`, and `/assets`.
- [ ] `internal/component/web/cli.go` - CLI bar handles edit/set/delete/show/top/up/commit/discard/who/help against editor sessions, not operational command dispatcher.
- [ ] `internal/component/config/yang/command.go` - builds a merged command tree from `-cmd.yang` modules and extracts `ze:command`.
- [ ] `internal/component/plugin/server/server.go` - server startup builds WireMethod-to-path mappings and registers RPC handlers with authz-aware CLI path context.
- [ ] `internal/component/plugin/server/command.go` - dispatcher extracts peer selectors, matches command prefixes, enforces authz, and executes handlers.
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - peer list/detail/capabilities/statistics/lifecycle handlers already filter by peer selector.
- [ ] `internal/component/config/yang_schema.go` - current config schema converter extracts simple extensions such as `ze:decorate`, `ze:backend`, `ze:hidden`, `ze:required`, and `ze:suggest`.
- [ ] `cmd/ze/hub/main.go` - web startup creates renderer, editor manager, CLI handlers, admin handlers, L2TP handlers, portal, and route registration under auth wrapper.

### Behavior to Preserve

- Authentication and session cookie behavior.
- Basic Auth for JSON API requests.
- TLS certificate generation/persistence.
- Security headers and no-store cache headers.
- Per-user draft sessions and working tree visibility.
- Commit/diff/discard workflow and conflict detection.
- CLI bar and terminal mode.
- Existing `/show`, `/monitor`, `/config`, `/admin`, `/cli`, `/events`, `/portal`, and `/l2tp` endpoints.
- Existing JSON negotiation for view/admin routes.
- Current editor validation, `ze:decorate`, `ze:required`, `ze:suggest`, and unique constraint behavior.
- Command dispatch authz using username and remote address from trusted server context.

### Behavior to Change

- Primary config UI changes from Finder columns to a RouterOS-style workbench shell.
- Main navigation becomes section-first with persistent top and left navigation.
- Named lists render as tables by default, not only when a list has YANG `unique` constraints.
- List tables receive contextual tool areas and per-row related tool buttons.
- Detail views receive contextual toolbars for selected entries and containers.
- Related operational tools are declared with `ze:related` metadata on config schema nodes.
- Related tool execution uses a dedicated server-side resolver and result overlay.
- Admin command tree is derived from the YANG command tree rather than the static `BuildAdminCommandTree` map.

## Design Alternatives

| Approach | Description | Advantages | Problems |
|----------|-------------|------------|----------|
| A: Hardcode BGP buttons in web templates | Add peer detail/capabilities/statistics buttons directly in BGP table templates. | Fastest visible result; minimal metadata work. | Does not scale beyond BGP; duplicates command knowledge in templates; violates derive-not-hardcode; makes plugins unable to contribute tools. |
| B: Add `ze:related` metadata and overlay execution inside current UI first | Add metadata extraction, related tool data model, command resolver, and overlay endpoint while keeping Finder/table layout mostly intact. | Establishes safe reusable contract; tests can prove dispatch/authz/overlay before the larger shell rewrite; plugins can annotate nodes later. | Visual change is incremental; left/top nav still needs a later phase. |
| C: Build full RouterOS shell first, then wire tools | Replace layout/navigation/table components first, then attach tools. | Gives immediate visual direction and table-first model. | High blast radius; command overlay contract may be bolted on afterward; harder to test causality and preserve behavior. |
| D: Reuse `/admin` command tree as the related tools UI | Link from config rows to existing admin command pages. | Reuses existing handler. | Output replaces workspace; browser can still construct arbitrary `/admin` command paths; does not provide row-aware prefilling; feels disconnected from config context. |
| E: Build a V2 workbench experiment with a phased default switch | Build V2 behind opt-in `ze.web.ui=workbench` through Phases 1-3, flip the default to V2 after Phase 4 once the BGP change-and-verify loop passes the Promotion Criteria, then promote or reject in Phase 7. | Tests the workflow in situ once parity is reached; directly targets operator friction; avoids shipping a degraded default; avoids maintaining two visible UIs long-term. | Requires a reliable env-var-driven mode switch, measurable Promotion Criteria, and disciplined cleanup. |

### Recommendation

Use Approach E.

The workbench should be created as a V2 experiment with BGP peers as the first complete workflow. The related-tool contract remains load-bearing because it is security-sensitive and reusable, but it should be delivered through the V2 change-and-verify experience rather than bolted onto the old Finder UI first.

## Design Decisions

| ID | Decision | Reason |
|----|----------|--------|
| D1 | Related tools are declared in YANG metadata with a new `ze:related` extension. | Keeps config schema, web UI, CLI command tree, and plugin-contributed UI hints aligned. |
| D2 | The browser submits tool id plus config context, never raw command text, for related tool execution. | Prevents command injection and keeps authz semantics server-controlled. |
| D3 | Tool execution reuses `CommandDispatcher(command, username, remoteAddr)`. | Existing dispatch path already handles command resolution, authz, accounting identity, and handler execution. |
| D4 | Tool results render in an overlay, not in `#detail`. | Operators can inspect output while keeping their current table/form context and draft state visible. |
| D5 | Initial related-tool output rendering is safe preformatted text. | Avoids prematurely building per-command renderers; every command already has text output. |
| D6 | Admin tree should be YANG-derived. | The static admin map already drifted from the command-tree architecture; related tools need the same source of truth. |
| D7 | BGP peer tools resolve remote IP via `${path-inherit:connection/remote/ip\|key}`. For `bgp/peer`, this looks up the peer's own `connection/remote/ip` and falls back to the peer key. For `bgp/group/peer`, it looks up the peer's value first, walks to the parent group's `connection/remote/ip`, and finally falls back to the peer key. | Group peers commonly inherit connection settings from the group; without inheritance, a row whose IP is declared on the group would be unusable. |
| D8 | Mutating related tools require an explicit confirmation step in the overlay. | RouterOS-style tools include destructive actions; read/write authz still applies, but the UI should prevent accidental teardown/clear. |
| D9 | Workbench section navigation is metadata-assisted, not purely hardcoded. | Main sections are a UX taxonomy, but schema/plugins should be able to contribute entries without editing central templates. |
| D10 | V2 becomes the active `/show` UI at the end of Phase 4, after the BGP change-and-verify loop ships and passes the Promotion Criteria. Until then, V2 is reached only via `ze.web.ui=workbench`. | A default-active V2 is only meaningful once it has parity with Finder for the canonical workflow; flipping earlier ships an intentionally degraded UI to anyone running the hub. |
| D11 | V2 acceptance is judged by the Promotion Criteria table (zero full-page navigations, click-count ceiling, preserved workspace context), not by subjective impression or visual parity. | Quantitative criteria let Phase 7 resolve as a measurement, not a judgment call. |
| D12 | BGP peer screen is the first complete workflow, not only the first table. | A complete workflow proves find, inspect, edit, review, commit, verify, and decide in one place. |
| D13 | Related tool overlays support rerun and pinning in v1; pinning is required, not optional. | Friction Target 4 (compare before/after output) hard-depends on at least one pinned overlay. Pinning is DOM-only state -- each overlay is a sibling node with a unique id and an explicit close button -- so it adds no server-side state. |
| D14 | Finder is the default through Phases 1-3 and the rollback target after the Phase 4 flip. There is no time at which both UIs are reachable as normal routes from a single hub process. | A phased default keeps users on a working UI while V2 is incomplete; emergency rollback covers regressions discovered after the flip. |
| D15 | Promotion must remove Finder and rollback mode in a follow-up cleanup. | The V2 exception should not create a permanent dual-interface maintenance burden. |
| D16 | `ze:related` (this spec) and `ze:ui-resource`/`ze:ui-permissions`/`ze:ui-csp` (`spec-mcp-5-apps.md`) are kept as separate YANG extensions; they do not share a metadata model. Both live under the `ze:` namespace and use kebab-case names. The `ze-extensions.yang` description for each MUST cross-reference the other and call out that the divergence is intentional. | They annotate different schema sites (config nodes vs. command nodes), describe different artifacts (executable command templates vs. static asset URIs), have different cardinalities (many-per-node vs. one root + siblings), use different substitution grammars (placeholder-heavy vs. literal), and enforce different trust contracts (server-side trusted command construction vs. sandboxed iframe with declared CSP). MCP Apps is governed by the 2026-01-26 external schema and ze is a consumer that must mirror `_meta.ui.*` exactly; web-2's `ze:related` is internal and evolves independently. Forcing one model would either bloat both annotations with optional fields the other ignores or invent a discriminator that obscures the simple per-extension intent. The cross-reference requirement is the maintenance hook so a future contributor doesn't quietly try to merge them. (Resolves Q8.) |

## Related Tool Metadata Contract

### New Extension

| Extension | Applies To | Repeatable | Purpose |
|-----------|------------|------------|---------|
| `ze:related` | config containers, lists, leaves, and possibly command nodes in later phases | yes | Declares one operator tool related to the annotated config node. |

### Argument Wire Format

The `ze:related` argument is a single string carrying one descriptor. `goyang` returns extension arguments as opaque strings, so the descriptor format is defined here, not derived from YANG.

| Element | Rule |
|---------|------|
| Field separator | `;` |
| Key/value separator | `=` |
| Quoting | Values that contain `;`, `=`, leading/trailing whitespace, or `}` must be enclosed in double quotes. Values without those characters may be unquoted. |
| Escapes inside quoted values | `\"` is a literal `"`, `\\` is a literal backslash. Unknown escape sequences (`\x`, `\n`, etc.) are a parse error. |
| Whitespace around separators | Ignored. |
| Repetition | Multiple `ze:related` statements on the same node are allowed; one descriptor per statement. The argument string itself never carries multiple descriptors. |
| Unknown keys | Rejected at parse time. |

Placeholder substitution (`${...}`) inside the `command` value is parsed after the descriptor is decoded; placeholders are not subject to the quoting rules above and are tokenized by their own grammar (see Placeholder Sources).

### Descriptor Fields

The descriptor uses named fields so new fields can be added without changing old annotations.

| Field | Required | Purpose |
|-------|----------|---------|
| `id` | yes | Stable id used by the browser when requesting execution. Unique within the annotated node. |
| `label` | yes | Short button/menu label. |
| `command` | yes | User-facing command template. This is a Ze command, not a shell command. |
| `placement` | no | Where to display: `global`, `table`, `row`, `detail`, or `field`. Default is `detail`. |
| `presentation` | no | How to show output: `modal`, `drawer`, or `panel`. Default is `modal`. |
| `confirm` | no | Confirmation label/message for mutating or disruptive tools. Empty means no confirmation. |
| `requires` | no | Comma-separated placeholder names that must resolve before the tool is enabled. Use `\,` inside a quoted value to embed a literal comma. |
| `class` | no | Visual intent: `inspect`, `diagnose`, `refresh`, `danger`. Styling only; authz is not inferred from this. |
| `empty` | no | Behavior when a placeholder is missing: `disable`, `omit`, or `allow`. Default is `disable`. |

### Placeholder Sources

Placeholders are resolved against the user's working tree, not the committed tree. The base for relative paths is the schema node carrying the descriptor; for row context, the base is the row's config subtree.

| Placeholder | Source | Notes |
|-------------|--------|-------|
| `${key}` | Current list entry key | Available only for row/detail contexts under a list entry. |
| `${path:<relative-path>}` | Value at relative config path; base is the row subtree (row context) or the schema node (container/detail context) | Used for fields such as BGP remote IP or interface name. |
| `${path:<relative-path>\|key}` | Relative config value, falling back to the list key | Use only in row context. |
| `${path-inherit:<relative-path>\|key}` | Same as the previous, but if the value is missing on the row, walks one parent list entry and retries before falling back to the key. Required for `bgp/group/peer` so the group's `connection/remote/ip` is visible from the peer row. |
| `${current-path}` | Full current YANG path | Useful for diagnostics, not command selectors. |
| `${leaf}` | Current leaf name | Available only for field-level tools. |
| `${value}` | Current leaf value | Available only for field-level tools. |

#### Relative Path Grammar

| Rule | Detail |
|------|--------|
| Segments | `/`-separated YANG identifiers; no leading or trailing `/`. |
| Predicates | Not allowed in v1. Row context already binds the list entry; cross-list lookups are out of scope for v1. |
| Reserved characters in path text | `\|` separates fallback; `}` terminates the placeholder; `:` separates source from path. None of these may appear inside a path segment. |
| Depth | <= 16 segments (matches Boundary Tests). |

#### Resolved-Value Validation

Placeholder resolution must produce command tokens, not shell text. The initial related-tool implementation does not support free-text placeholders.

| Rule | Detail |
|------|--------|
| Allowed value characters | Letters, digits, `.`, `:`, `-`, `_`, `/`. IPv6 brackets `[` and `]` are allowed only when the resolved value matches a valid address literal. |
| Whitespace | Rejected. |
| Shell metacharacters | `;`, `&`, `|`, backtick, `$`, `(`, `)`, `<`, `>`, `\`, `"`, `'`, newline rejected. Command construction never reaches a shell, but rejecting these still blocks attempts at injection through downstream renderers. |
| Length | <= 256 chars per resolved placeholder; total resolved command length matches Boundary Tests. |

### Day-One BGP Related Tools

| Config Context | Tool ID | Placement | Command Intent | Selector Source | Presentation |
|----------------|---------|-----------|----------------|-----------------|--------------|
| `bgp` | `bgp-health` | global/detail | Show BGP peer health | none | modal |
| `bgp` | `bgp-warnings` | global/detail | Show active BGP warnings | none | modal |
| `bgp` | `bgp-errors` | global/detail | Show recent BGP errors | none | modal |
| `bgp/peer` | `peer-detail` | row/detail | Show runtime peer details | `${path:connection/remote/ip\|key}` | drawer |
| `bgp/peer` | `peer-capabilities` | row/detail | Show negotiated peer capabilities | `${path:connection/remote/ip\|key}` | modal |
| `bgp/peer` | `peer-statistics` | row/detail | Show peer update/message counters | `${path:connection/remote/ip\|key}` | modal |
| `bgp/peer` | `peer-flush` | row/detail | Wait for peer forward pool to drain | `${path:connection/remote/ip\|key}` | modal |
| `bgp/peer` | `peer-teardown` | row/detail | Teardown session | `${path:connection/remote/ip\|key}` | modal with confirmation |
| `bgp/group/peer` | same ids as `bgp/peer` | row/detail | Same as standalone peer | `${path-inherit:connection/remote/ip\|key}` (peer -> group -> key) | same |

### Future BGP Related Tools

| Tool | Condition |
|------|-----------|
| Route refresh | Add when a command tree entry and dispatcher handler exist for sending route refresh safely. |
| Soft clear | Add when command semantics are explicit: clear inbound, outbound, both, and whether routes are retained. |
| Policy impact preview | Add when the runtime path can report affected peers/routes for a draft policy change. |

### Metadata Validation Rules

| Rule | Expected Behavior |
|------|-------------------|
| Duplicate ids on the same node | Schema build fails with a clear error pointing to the offending node and id. |
| Unknown descriptor fields | Rejected at parse time. |
| Missing required field (`id`, `label`, `command`) | Related tool is invalid and must not render. |
| Placeholder syntax error | Related tool is invalid and must not render. |
| Placeholder cannot resolve for current row | Tool renders disabled if `empty=disable`; otherwise follows descriptor behavior. |
| Command template after placeholder substitution against a representative resolved value does not match a registered command in the YANG command tree | Schema build fails with a clear error. Validation runs once at load using a synthesized canonical row; this catches typos and renamed-command drift before any user clicks the tool. |
| Command template too long after substitution at request time | Request rejected before dispatch. |
| Command template contains unsupported token characters at request time | Request rejected before dispatch. |

## Workbench UI Contract

### Layout Regions

| Region | Purpose | Current Source | New Source |
|--------|---------|----------------|------------|
| Top bar | Identity, section breadcrumbs, global search, safe mode/commit, CLI toggle, user/session | `breadcrumb-bar` | new workbench header template, still includes breadcrumb data and commit state |
| Left navigation | Main sections and subsections | Finder first column | schema/metadata-assisted section model |
| Workspace toolbar | Current section title, table actions, related tools, filters | none | new table/header component |
| Main table | Named resource lists | `list_table` for unique lists only | new generic list table for named lists |
| Detail drawer/panel | Selected row full config and related tools | `detail` panel | detail drawer or right panel, still server-rendered |
| Tool overlay | Operational command output, one DOM node per overlay instance | none; admin replaces detail | overlays are appended to a `#tool-overlays` container with `hx-swap="beforeend"`. Each overlay node has a unique `id="overlay-<short-uuid>"` so close affordances and OOB swaps target one instance. Multiple overlays may coexist for pinning. |
| CLI bar | Command entry and completions | existing `cli_bar` | preserved |
| Commit bar | Pending changes review/discard | existing `commit_bar` | preserved, likely moved visually into top bar later |

The workbench targets a minimum viewport of 1280x800. Below 1280px wide, the left navigation collapses to a toggle button and the CLI bar may be hidden by user action. Below 1024px wide, the workbench shows an explicit "viewport too small" notice. Mobile and tablet ergonomics are not v1 acceptance targets.

### Operational Workspace Behavior

| Behavior | Requirement |
|----------|-------------|
| Row change marker | A row with draft changes shows a clear pending marker and exposes row-scoped diff/revert actions when practical. |
| Same-context verification | Related tools launched from a row preserve the selected row and table scroll/context. |
| Runtime status summary | The BGP peer table should reserve space for runtime summary fields such as session state, uptime, prefix counts, last error, or "unknown" when structured data is not yet available. |
| Tool rerun | Every related tool result can be rerun with the same resolved context without navigating away. |
| Tool pinning | At least one overlay/drawer result can remain visible while the operator edits or reruns another check, if the UI can do this without custom framework code. |
| CLI context handoff | The CLI bar path/context follows the selected object so an engineer can drop to CLI from the same workflow. |
| Verification after commit | After commit, the workspace should keep the same selected object and make the most relevant verification tools visually prominent. |

### Left Navigation Sections

| Section | Initial Contents | Source |
|---------|------------------|--------|
| Dashboard | health, warnings, errors, recent events | command tree plus current health endpoints |
| Interfaces | interface config lists and show traffic | iface schema and show traffic command |
| Routing | BGP, RIB, static, policy routing | config schema plus command tree |
| Policy | BGP policy/filter config | `bgp/policy`, future firewall policy |
| Firewall | firewall config once present | firewall schema |
| Services | SSH, web, telemetry, MCP, L2TP, looking glass | environment/service schema |
| System | users/authz, host inventory, daemon status | config and show host/system commands |
| Tools | ping, decode/encode, metrics query, capture | command tree |
| Logs | warnings, errors, event recent, log levels | command tree/report bus |

v1 ships a small ordered table in Go in a single file under `internal/component/web/`. `ze:nav-section` is deferred to a follow-up spec. The boundary is hard: if section grouping spreads to more than one Go file, stop and add the metadata extension before continuing.

### Table Behavior

| Feature | Phase | Behavior |
|---------|-------|----------|
| Default columns | 1 | Key plus required, unique, suggested, and decorated fields when available. |
| Inline edits | 1 | Preserve existing save-on-blur/enter/debounce behavior. |
| Row navigation | 1 | Key click opens detail view/drawer for full config. |
| Row tools | 1 | Tools with `placement=row` appear as compact actions. |
| Table tools | 1 | Tools with `placement=table` appear in toolbar. |
| Search/filter | 2 | Server-rendered filter form first; no client table framework. |
| Sorting | 2 | Server-side query params or simple stable sort in Go; must not mutate config order. |
| Column chooser | 3 | Per-user preference in session or zefs later; out of first patch unless cheap. |
| Operational columns | 3 | BGP state/prefixes/uptime merged from commands once command outputs have structured rendering. |

## Data Flow

### Entry Points

| Entry Point | Format | Purpose |
|-------------|--------|---------|
| `GET /show/<yang-path>` | HTML or JSON | Phases 1-3: renders Finder by default, V2 only with `ze.web.ui=workbench`. Phase 4 onward: renders V2 by default, Finder only with `ze.web.ui=finder`. |
| `GET /fragment/detail?path=<yang-path>` | HTML fragment | HTMX navigation update. |
| `POST /tools/related/run` | form | Execute one related tool for a config context. |
| `GET /admin/<command-path>` | HTML or JSON | Browse the general command tree. |
| `POST /admin/<command-path>` | form | Execute an explicit admin command path, existing behavior preserved. |
| `POST /cli` | form | Existing CLI bar integrated mode. |

V2 uses `/show/` so the workflow is tested in the real operator path: opt-in via `ze.web.ui=workbench` through Phases 1-3, default after the Phase 4 flip. `ze.web.ui=finder` switches `/show/` back to Finder. There must not be a permanent `/workbench/` side route; if a short-lived side route is needed as a development aid, it must be removed before the Phase 4 flip.

### Related Tool Transformation Path

1. Config YANG modules load with `ze:related` extension statements.
2. Config schema conversion extracts related descriptors into schema node metadata.
3. Web fragment builder walks current schema node and tree path.
4. Related tool builder selects descriptors whose placement matches the current region.
5. For list tables, row builder resolves descriptor placeholders against each row's config subtree.
6. Template renders tool buttons with trusted tool id and current YANG context path, not raw command.
7. Browser clicks a tool button; HTMX posts tool id and context path to related-tool route.
8. Handler authenticates via existing web middleware and gets username from request context.
9. Handler validates context path, finds the schema node, finds the matching `ze:related` descriptor, and resolves placeholders from the user's working tree.
10. Handler constructs a Ze command string from the trusted template.
11. Handler dispatches through `CommandDispatcher(command, username, remoteAddr)`.
12. Dispatcher performs existing command matching, peer selector extraction, authz, and handler execution.
13. Handler receives output/error and renders `tool_overlay` via the renderer.
14. HTMX swaps the overlay into the page while the workbench table/detail remains unchanged.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema -> config schema structs | `ze:related` extractor stores descriptors on nodes | unit tests in config package |
| Config schema -> web view data | `buildFragmentData`/table builders collect `RelatedTool` data | web unit tests |
| Browser -> web tool route | tool id + context path only | route tests reject raw command input |
| Web route -> command dispatcher | trusted command string plus username/remoteAddr | handler test captures command and identity |
| Dispatcher -> command handler | existing `CommandDispatcher` | integration test with BGP peer command |
| Command output -> overlay HTML | `ToolOverlayData` template | render tests and web browser test |

### Integration Points

- `ze-extensions.yang` adds `ze:related`.
- `schema.go` gains a reusable related-tool metadata type or per-node fields.
- `yang_schema.go` parses `ze:related` on config nodes.
- `fragment.go` builds related tool data for current page, table, row, and detail contexts.
- `handler_tools.go` adds the related tool execution route.
- `render.go` parses the overlay and toolbar templates.
- `cmd/ze/hub/main.go` registers tool routes under the existing auth wrapper.
- `handler_admin.go` gains a YANG-derived command tree path, or a new helper replaces the static map.
- BGP config YANG receives initial `ze:related` annotations.

### Architectural Verification

- [ ] No bypassed layers: related tools use the same dispatcher and authz as CLI/admin/MCP.
- [ ] No unintended coupling: web does not import BGP command handlers; it only uses config schema metadata and command dispatch.
- [ ] No duplicated functionality: command discovery reuses YANG command tree; table/edit behavior reuses existing editor manager.
- [ ] No arbitrary command execution from browser-controlled fields.
- [ ] No plugin boundary violation: plugin-contributed command metadata remains in YANG/command registry.

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `zeconfig.YANGSchema()` loads BGP peer list annotations | -> | `ze:related` parser stores peer tool descriptors on `ListNode` | `TestYANGSchemaRelatedTools_BGPPeer` |
| `GET /show/bgp/peer/` with `ze.web.ui=workbench` (Phases 1-3) or with the default after the Phase 4 flip | -> | V2 renders peer table, row detail, pending change area, related tools, and CLI context in one workspace; buttons carry no raw command in client-submitted data | `TestBuildListTable_RelatedTools` and `test/web/workbench-bgp-change-verify.wb` |
| `GET /show/bgp/peer/` with `ze.web.ui=finder`, or with the default during Phases 1-3 | -> | Finder UI renders and V2 workbench markers are absent | `test/web/workbench-rollback-finder.wb` |
| Edit BGP peer field then run peer detail from same row | -> | draft marker appears, peer detail overlay opens, table context remains visible | `test/web/workbench-bgp-change-verify.wb` |
| `POST /tools/related/run` for `peer-detail` | -> | server resolves command from metadata and dispatches with username/remoteAddr | `TestHandleRelatedToolRun_DispatchesResolvedCommand` |
| `POST /tools/related/run` with unknown id | -> | handler rejects before dispatch | `TestHandleRelatedToolRun_UnknownTool` |
| `POST /tools/related/run` where required selector is missing | -> | overlay shows disabled/missing context error; no dispatch | `TestHandleRelatedToolRun_MissingRequiredValue` |
| `POST /tools/related/run` for `peer-teardown` without confirmation | -> | handler returns confirmation overlay and does not dispatch | `TestHandleRelatedToolRun_ConfirmRequired` |
| `POST /tools/related/run` for `peer-teardown` with confirmation, read-only user | -> | dispatcher/authz denies and overlay shows error | `TestHandleRelatedToolRun_AuthzDenied` or functional authz test |
| Admin page root | -> | command tree derived from YANG command tree, not static map | `TestBuildAdminCommandTree_FromYANG` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | User visits `/show/` after the Phase 4 default flip, or during Phases 1-3 with `ze.web.ui=workbench` | Page renders the workbench shell with top bar, left navigation, workspace area, commit bar, and CLI bar. |
| AC-1a | User starts the hub with `ze.web.ui=finder` after the flip, or with the default during Phases 1-3 | `/show/` renders the Finder UI and V2 workbench markers are absent. |
| AC-2 | User visits `/show/bgp/peer/` | BGP peers render as a table even if only key/required/suggest fields are available. |
| AC-3 | BGP peer table has configured peer with remote IP | Row shows related tool actions for detail, capabilities, statistics, flush, and teardown as applicable. |
| AC-4 | BGP peer table has peer without remote IP | Tools requiring a peer selector are disabled or use list key fallback when descriptor allows fallback. |
| AC-5 | User runs peer detail related tool | Command is resolved server-side, dispatched with authenticated username/remote address, and output appears in overlay. |
| AC-6 | User closes overlay | Underlying table/detail page state remains unchanged. |
| AC-7 | Related command fails | Overlay shows error styling and command output/error text; page does not navigate away. |
| AC-8 | User attempts to forge raw command in tool POST | Handler ignores/rejects raw command input and uses only tool id + schema metadata. |
| AC-9 | Read-only user runs read-only related tool | Tool succeeds if authz allows the command. |
| AC-10 | Read-only user runs mutating related tool | Dispatcher denies the command; overlay shows authorization error; command is not executed. |
| AC-11 | Admin command page root is opened | Admin command tree reflects registered YANG command modules, not the old static `peer/route/cache/system` map. |
| AC-12 | Existing config set/add/rename/delete flows | Still work and update commit bar as before. |
| AC-13 | Existing CLI bar navigation | Prompt, path bar, and hidden CLI context remain in sync after workbench navigation. |
| AC-14 | Browser has JavaScript disabled except HTMX | Core navigation and related tool execution still work through server-rendered forms/buttons where practical; non-essential conveniences may degrade. |
| AC-15 | CSP is active | No inline script/eval requirement is introduced by related tools or workbench shell. |
| AC-16 | `?format=json` on existing show/admin endpoints | Existing JSON responses remain compatible unless explicitly documented. |
| AC-17 | Engineer edits one BGP peer field in V2 | The row shows a pending change marker and the table remains visible. |
| AC-18 | Engineer runs peer detail after editing but before commit | The related tool opens in an overlay/drawer without navigating away, and the selected peer context remains active. |
| AC-19 | Engineer commits a BGP peer change | The same BGP peer workspace remains selected after commit, and relevant verification tools are still visible. |
| AC-20 | Engineer reruns peer detail/statistics after commit | The tool reruns for the same peer context and updates the overlay/drawer output. |
| AC-21 | Engineer opens diff/review from the BGP peer workspace | Diff/review is reachable without losing the selected peer context. |
| AC-22 | After the Phase 4 default flip, V2 is the default UI | Finder is reachable only through `ze.web.ui=finder`; the default route serves V2. |
| AC-23 | V2 is promoted in Phase 7 | A follow-up cleanup plan removes the old Finder-first UI path, rollback mode, and obsolete templates/tests. |
| AC-24 | V2 fails the Promotion Criteria | The default reverts to Finder and V2 workbench templates/handlers are removed without losing the current UI. |
| AC-25 | User pins overlay A, opens overlay B, closes overlay B | Overlay A remains visible and unchanged; overlay B's DOM node is removed without disturbing other overlays. |
| AC-26 | Tool output is between 128 KiB and 4 MiB | First 128 KiB renders inline; "Show full output" extends the same overlay with the remainder; no full-page navigation. |
| AC-27 | Tool output exceeds 4 MiB | Output is truncated server-side at 4 MiB; the overlay renders the first 128 KiB inline, surfaces a clear truncation notice, and offers "Show full output" against the buffered 4 MiB only. |
| AC-28 | Command output contains ANSI escape sequences | Sequences are stripped server-side before rendering; the overlay shows plain text. |

### Known v1 Limitations

| Limitation | Reason |
|------------|--------|
| Mutating tools render visibly for read-only users; clicks fail through dispatcher denial. | Hiding requires exposing authz prediction to the render path. v1 prefers consistent button visibility plus dispatcher-enforced denial; a later phase may add prediction. (See Q4.) |
| `.wb` runner asserts on DOM swap behavior; if the runner cannot distinguish HTMX swap from full navigation, navigation-count Promotion Criteria fall back to manual review for that run. | The runner contract is owned by `test/web/`; this spec depends on it but does not extend it. |
| `ze.web.ui` is read once at hub startup; flipping it requires a restart. | A runtime toggle would require dual-rendering infrastructure; deliberately out of scope. |
| `r.RemoteAddr` reaches the dispatcher unfiltered. Behind a reverse proxy this is the proxy's IP, not the operator's. | Consistent with the existing `/cli` and `/admin` handlers; trusted-header parsing belongs to a hub-wide change, not to this spec. |
| `lookupRelative` only walks container-then-leaf paths. Day-one BGP descriptors all match this shape. | A descriptor that traverses a list mid-path (e.g. `policy/<key>/community`) silently returns "not found"; documented and deferred until a real use case appears. |
| `bin/ze-test web` runner has no per-test env-var support. | The `.wb` files in `test/web/workbench-*.wb` document the intended workbench-mode runs; the operator exports `ZE_WEB_UI=workbench` before invoking the runner. Extending the runner is a separate change owned by `test/web/`. |
| CSRF defense is Origin/Referer matching, not a synchronizer token. | SameSite=Strict on the session cookie is the primary gate; `checkSameOrigin` in `handler_tools.go` is the spec-recommended belt-and-braces check for destructive related tools. A full CSRF token would require session integration that no other endpoint uses today. |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRelatedExtension_ParseDescriptor` | `internal/component/config/yang_schema_test.go` or new related metadata test | Parses valid `ze:related` descriptor into id/label/command/placement/presentation/confirm fields. | |
| `TestRelatedExtension_WireFormat_QuotedValues` | same | Quoted values containing `;`, `=`, `}`, and whitespace round-trip; `\"` and `\\` escapes decode correctly; unknown escape sequences are a parse error. | |
| `TestRelatedExtension_WireFormat_RejectsUnknownKey` | same | Unknown descriptor keys are rejected at parse time. | |
| `TestRelatedExtension_RejectsInvalidDescriptor` | same | Duplicate/missing required fields are rejected or omitted deterministically. | |
| `TestRelatedExtension_RejectsCommandNotInTree` | same | Descriptor whose command template (after canonical placeholder substitution) does not match a registered YANG command tree entry fails schema build with a clear error. | |
| `TestYANGSchemaRelatedTools_BGPPeer` | `internal/component/config/yang_schema_test.go` | BGP peer list carries expected related tool descriptors. | |
| `TestRelatedToolResolve_PeerSelectorFromRemoteIP` | `internal/component/web/handler_tools_test.go` | Resolves peer selector from `connection/remote/ip`. | |
| `TestRelatedToolResolve_FallbackToKey` | same | Fallback from missing remote IP to list key works only when descriptor allows it. | |
| `TestRelatedToolResolve_PathInheritFromGroup` | same | `${path-inherit:connection/remote/ip\|key}` on `bgp/group/peer` walks to the group when the peer omits the field, and falls back to the peer key when both are missing. | |
| `TestRelatedToolResolve_RejectsUnsafeValue` | same | Unsafe placeholder values (whitespace, shell metacharacters, over-length) are rejected before dispatch. | |
| `TestRelatedToolResolve_PathDepthExceeded` | same | Relative path > 16 segments is rejected at parse time. | |
| `TestBuildListTable_RelatedTools` | `internal/component/web/fragment_test.go` | Row/table data includes expected related tools and disabled state. | |
| `TestHandleRelatedToolRun_DispatchesResolvedCommand` | `internal/component/web/handler_tools_test.go` | Handler dispatches expected command with username and remote address. | |
| `TestHandleRelatedToolRun_DoesNotTrustCommandFormValue` | same | Form-supplied command text does not affect dispatched command. | |
| `TestHandleRelatedToolRun_UnknownTool` | same | Unknown tool id returns 404/400 and does not dispatch. | |
| `TestHandleRelatedToolRun_ConfirmRequired` | same | Mutating tool with confirm metadata returns confirmation overlay first. | |
| `TestHandleRelatedToolRun_StripsANSI` | same | Output containing ANSI escape sequences is stripped before being placed in the overlay payload. | |
| `TestHandleRelatedToolRun_TruncatesAtBufferLimit` | same | Output beyond 4 MiB is truncated server-side; truncation flag is set on the response. | |
| `TestToolOverlay_RenderSuccess` | `internal/component/web/render_test.go` | Success overlay includes title, command name, output, close affordance, and a unique DOM id. | |
| `TestToolOverlay_RenderError` | same | Error overlay includes error class and safe escaped output. | |
| `TestToolOverlay_ShowFullOutput` | same | Output between 128 KiB and the buffer cap renders an inline preview plus a "Show full output" affordance that expands without re-dispatching. | |
| `TestToolOverlay_TruncationNotice` | same | Output beyond the buffer cap renders the truncation notice. | |
| `TestToolOverlay_MultipleInstancesUniqueIDs` | same | Two simultaneous overlays receive distinct DOM ids; close on one leaves the other intact. | |
| `TestUIMode_DefaultsToFinder` | `internal/component/web/ui_mode_test.go` | Default mode (no env var set) selects Finder. (Updated when Phase 4 flips the default; test asserts both pre-flip and post-flip behavior under the corresponding code state.) | |
| `TestUIMode_OptInWorkbench` | same | `ze.web.ui=workbench` selects V2. | |
| `TestUIMode_RollbackFinder` | same | `ze.web.ui=finder` selects Finder. | |
| `TestBuildAdminCommandTree_FromYANG` | `internal/component/web/handler_admin_test.go` | Admin tree is built from YANG command tree and includes representative show/peer/rib/system commands. | |
| `TestWorkbenchSectionModel_BGP` | `internal/component/web/fragment_test.go` or new workbench test | `bgp` appears under Routing/BGP in left nav. | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Tool id length | 1-64 chars | 64 chars | empty | 65 chars |
| Tool label length | 1-48 chars | 48 chars | empty | 49 chars |
| Related command template length | 1-512 chars before substitution | 512 chars | empty | 513 chars |
| Resolved command length | 1-4096 chars | 4096 chars | empty | 4097 chars |
| Resolved placeholder value length | 1-256 chars | 256 chars | empty | 257 chars |
| Placeholder relative path depth | 0-16 segments | 16 | N/A | 17 |
| Overlay inline display | 0-128 KiB | 128 KiB | N/A | first 128 KiB rendered inline; "Show full output" loads the rest into the same overlay |
| Overlay full-output buffer | 0-4 MiB | 4 MiB | N/A | output beyond 4 MiB is truncated server-side with a clear notice in the overlay |

### Functional / Browser Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-workbench-shell` | `test/web/*.wb` | Login/open root; top bar and left nav render. | |
| `web-bgp-peer-table` | `test/web/*.wb` | Open BGP peers; table contains configured peer and required columns. | |
| `web-workbench-bgp-change-verify` | `test/web/*.wb` | Edit BGP peer field, see pending marker, run peer detail, commit, rerun peer detail without page navigation. Asserts every Promotion Criterion. | |
| `web-workbench-bgp-pending-diff` | `test/web/*.wb` | Edit BGP peer field and open diff/review from same workspace. | |
| `web-bgp-peer-tools` | `test/web/*.wb` | Click peer detail tool; overlay opens with output; table remains visible after close. | |
| `web-workbench-rollback-finder` | `test/web/*.wb` | Hub started with `ze.web.ui=finder`: `/show` renders Finder; V2 workbench markers are absent. Same scenario also runs with no env var set during Phases 1-3 to assert the default is Finder. | |
| `web-workbench-default-flip` | `test/web/*.wb` | Added in Phase 4: with no env var set after the default flip, `/show` renders V2; with `ze.web.ui=finder` it renders Finder. | |
| `web-related-tool-authz` | `test/web/*.wb` or functional web test | Read-only user sees mutating tool denied through overlay. | |
| `web-admin-yang-tree` | `test/web/*.wb` | Admin root shows YANG-derived commands rather than static subset. | |
| `web-tool-overlay-multi` | `test/web/*.wb` | Pin overlay A (peer detail), open overlay B (peer statistics), close overlay B; overlay A remains visible and unchanged. Covers AC-25. | |
| `web-tool-overlay-large-output` | `test/web/*.wb` | Run a tool whose output exceeds 128 KiB but is under 4 MiB; verify inline preview, "Show full output" expansion within the same overlay, no second dispatch. Covers AC-26. | |
| `web-tool-overlay-truncated-output` | `test/web/*.wb` | Run a tool whose output exceeds 4 MiB; verify truncation notice is rendered and the buffered prefix is shown. Covers AC-27. | |
| `web-tool-overlay-ansi` | `test/web/*.wb` or render test | Tool output containing ANSI sequences renders as plain text in the overlay. Covers AC-28. | |

### Future Tests

- Rich structured rendering for command-specific JSON output is deferred until a command-output renderer registry exists.
- Column chooser persistence is deferred unless Phase 2 adds per-user UI preferences.

## Files to Modify

| File | Purpose |
|------|---------|
| `internal/component/config/yang/modules/ze-extensions.yang` | Add `ze:related` extension documentation. |
| `internal/component/config/environment.go` | Register temporary `ze.web.ui` rollback env var. |
| `internal/component/config/schema.go` | Add related tool metadata storage to schema nodes or shared metadata wrapper. |
| `internal/component/config/yang_schema.go` | Extract `ze:related` descriptors from YANG entries. |
| `internal/component/config/yang_schema_test.go` | Test parsing and BGP schema metadata. |
| `internal/component/web/fragment.go` | Add related tool data to `FragmentData`, list rows, detail views, and workbench section model. |
| `internal/component/web/handler_admin.go` | Replace static command tree with YANG-derived tree or helper injection. |
| `internal/component/web/handler_admin_test.go` | Update tests for YANG-derived admin tree and preserve execution behavior. |
| `internal/component/web/render.go` | Parse/render new workbench, toolbar, related tool, and overlay templates. |
| `internal/component/web/templates/page/layout.html` | Adapt shell regions for top/left workbench layout while preserving commit/CLI bars. |
| `internal/component/web/templates/component/oob_response.html` | Update HTMX response to include workbench regions and OOB overlay target. |
| `internal/component/web/templates/component/detail.html` | Include detail-level related tools and workbench detail behavior. |
| `internal/component/web/templates/component/list_table.html` | Generalize named-list tables and row tool actions. |
| `internal/component/web/templates/component/breadcrumb.html` | Integrate with top bar if breadcrumb is retained there. |
| `internal/component/web/templates/component/cli_bar.html` | Preserve path/context behavior under new layout. |
| `internal/component/web/assets/style.css` | Workbench shell, left nav, table toolbar, row tools, overlay styles. |
| `internal/component/web/assets/cli.js` | Minimal delegated overlay close/rerun/copy behavior if HTMX alone is insufficient. |
| `internal/component/bgp/schema/ze-bgp-conf.yang` | Add initial `ze:related` annotations for BGP and peer lists. |
| `cmd/ze/hub/main.go` | Select workbench vs Finder route mode at startup; register related tool route under auth wrapper and pass dispatch/schema/editor manager as needed. |
| `docs/architecture/web-interface.md` | Document workbench shell and related tool route. |
| `docs/architecture/web-components.md` | Document new components and overlay flow. |
| `docs/features/web-interface.md` | Update feature summary from Finder navigation to workbench/table/tool model. |
| `docs/guide/web-interface.md` | User-facing guide for related tools, overlays, and new navigation. |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/web/handler_workbench.go` | Experimental V2 workbench handler and route-specific data assembly. |
| `internal/component/web/handler_workbench_test.go` | V2 `/show` routing and workflow tests. |
| `internal/component/web/ui_mode.go` | UI mode parsing/selection. Decision: separate file (not embedded in `cmd/ze/hub/main.go`) so `ui_mode_test.go` can exercise default/opt-in/rollback without spinning up the whole hub. |
| `internal/component/web/ui_mode_test.go` | Workbench-default and rollback-to-Finder mode tests. |
| `internal/component/web/handler_tools.go` | Related tool endpoint, resolver, and overlay response builder. |
| `internal/component/web/handler_tools_test.go` | Focused tests for security and command resolution. |
| `internal/component/web/templates/page/workbench.html` | V2 workbench shell if a separate page template is clearer than branching inside `layout.html`. |
| `internal/component/web/templates/component/tool_overlay.html` | Result/confirmation overlay fragment. |
| `internal/component/web/templates/component/related_tools.html` | Toolbar and row action rendering. |
| `internal/component/web/templates/component/workbench_nav.html` | Left navigation and section rendering, if kept separate. |
| `internal/component/web/templates/component/workbench_table.html` | Generic named-list table if `list_table.html` becomes too narrow. |

## Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema extension | yes | `internal/component/config/yang/modules/ze-extensions.yang` |
| Env var registration | yes | `internal/component/config/environment.go` for the temporary `ze.web.ui` mode selector (opt-in during Phases 1-3, rollback after the Phase 4 default flip; removed in Phase 7) |
| Config schema conversion | yes | `internal/component/config/schema.go`, `yang_schema.go` |
| CLI command tree | yes | `internal/component/config/yang/command.go` reused; no new CLI command required |
| Web UI mode selection | yes | `cmd/ze/hub/main.go`, `internal/component/web/handler_workbench.go`, existing Finder handler |
| Web tool route | yes | `cmd/ze/hub/main.go`, `internal/component/web/handler_tools.go` |
| Authz | yes | Existing dispatcher authz; tests must prove username/remoteAddr flow |
| Editor autocomplete | no | No new editor command syntax |
| Functional test for new web endpoint | yes | `test/web/*.wb` or equivalent web functional tests |
| Documentation | yes | Web architecture, guide, feature summary |

## Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, `docs/features/web-interface.md` |
| 2 | Config syntax changed? | No | Rollback uses temporary env var `ze.web.ui`; no YANG config syntax is added. |
| 3 | CLI command added/changed? | No | Existing commands reused |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/web-interface.md` for `/tools/related/run` |
| 5 | Plugin added/changed? | No | Plugin-contributed metadata supported later through existing YANG registration |
| 6 | Has a user guide page? | Yes | `docs/guide/web-interface.md` |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | Maybe | Only if `.wb` runner needs new assertions; update `docs/functional-tests.md` if so |
| 11 | Affects daemon comparison? | Maybe | `docs/comparison.md` if web UI feature table mentions UI capabilities |
| 12 | Internal architecture changed? | Yes | `docs/architecture/web-components.md`, `docs/architecture/config/yang-config-design.md` if `ze:related` is documented there |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, Current Behavior, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section below |
| 5. Full verification | `make ze-verify` or split approved unit targets if timeout |
| 6. Critical review | Critical Review Checklist |
| 7. Fix issues | Fix every critical review issue |
| 8. Re-verify | Re-run targeted and full verification |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run verification after security fixes |
| 13. Present summary | Executive Summary Report per planning rules |

### Implementation Phases

Each phase follows TDD: write failing tests first, implement, then pass. Each phase ends with a self-critical review before moving on.

1. **Phase 1: V2 workbench reachable behind opt-in flag**
   - Wire the `ze.web.ui` selector at hub startup. Default is `finder`; `workbench` enables V2.
   - When `ze.web.ui=workbench`, render the top bar, left navigation, BGP peer table, row detail area, commit bar, and CLI bar from shared web/editor/session state.
   - Finder remains the default `/show` UI. No user sees V2 unless they opt in.
   - Tests: `/show` renders Finder by default, `ze.web.ui=workbench` renders the workbench shell, `ze.web.ui=finder` is identical to the default during this phase, BGP peer table appears under the workbench, CLI context is present.
   - Files: `handler_workbench.go`, workbench templates, `render.go`, `cmd/ze/hub/main.go`, `ui_mode.go` (separate file for testability), CSS, workbench tests.

2. **Phase 2: Related metadata model**
   - **Hard prerequisite (Q8):** before writing the `ze:related` extension, confirm with `plan/spec-mcp-5-apps.md` whether one shared metadata model can serve both web related-tools and MCP UI resources. Resolve in writing in Design Decisions before any code lands. Phase cannot start otherwise.
   - Add `ze:related` extension declaration following the resolution above.
   - Add descriptor parser per the Argument Wire Format rules (semicolon/equals, quoted-value escapes, unknown-key rejection) and metadata storage on schema nodes.
   - Implement command-template validation against the YANG command tree at schema-load time using a synthesized canonical row; broken or renamed commands fail the schema build, not the user click.
   - Tests: `TestRelatedExtension_ParseDescriptor`, `TestRelatedExtension_WireFormat_QuotedValues`, `TestRelatedExtension_WireFormat_RejectsUnknownKey`, `TestRelatedExtension_RejectsInvalidDescriptor`, `TestRelatedExtension_RejectsCommandNotInTree`, `TestYANGSchemaRelatedTools_BGPPeer`.
   - Files: `ze-extensions.yang`, `schema.go`, `yang_schema.go`, config tests, BGP config YANG annotations.

3. **Phase 3: Related tool resolver, secure run endpoint, and overlay**
   - Add resolver from tool id + context path to trusted command invocation.
   - Add POST route that accepts tool id, context path, and (for confirm-required tools) a confirmation flag. No raw command field. Rerun is handled by re-POSTing the same id+context.
   - Add tiered output handling: render the first 128 KiB inline, expose the rest up to 4 MiB via "Show full output" inside the same overlay, and truncate beyond 4 MiB with a clear notice.
   - Strip ANSI escape sequences and other terminal control characters from command output before overlay rendering.
   - Apply HTML escaping at every render path (labels, output, error text).
   - Add overlay/drawer fragment with result, error, confirmation, close, rerun, and copy affordances. Each overlay instance has a unique DOM id; pinning is supported by leaving previous overlays attached.
   - Tests: handler/resolver security tests, render success/error/confirmation tests, multi-overlay pinning test, output-truncation tests.
   - Files: `handler_tools.go`, `handler_tools_test.go`, overlay templates, renderer, CSS, `cmd/ze/hub/main.go`.

4. **Phase 4: BGP change-and-verify loop and default flip**
   - Surface BGP row related tools inside the V2 table and detail drawer.
   - Preserve selected row and table context during edit, diff/review, commit, tool execution, and tool rerun.
   - Mark rows with pending draft changes.
   - Keep verification tools visible after commit.
   - **Default flip (gated by Promotion Criteria)**: once `web-workbench-bgp-change-verify` and the Promotion Criteria all pass, change the `ze.web.ui` default in `environment.go` from `finder` to `workbench`. Update Phase 1 default tests accordingly. If any criterion fails, do not flip; route to Phase 7 rejection branch.
   - Tests: `web-workbench-bgp-change-verify`, `web-workbench-bgp-pending-diff`, row marker unit tests, tool rerun test, default-flip test that asserts the new default and the rollback path still serves Finder.
   - Files: workbench handler/data model, templates, CSS, CLI/commit OOB integration as needed, `environment.go` for the default flip.

5. **Phase 5: Table-first named lists and section navigation**
   - Generalize V2 tables beyond BGP peers.
   - Default columns derive from key, required, unique, suggested, decorated, and runtime-summary metadata when available.
   - Add workbench section mapping for Dashboard, Interfaces, Routing, Policy, Services, System, Tools, and Logs.
   - Tests: named list without unique renders table; BGP peer table still works; workbench section model tests.
   - Files: list table builder, workbench nav, templates, tests.

6. **Phase 6: YANG-derived admin tree**
   - Replace or deprecate static `BuildAdminCommandTree`.
   - Build admin navigation from the merged YANG command tree.
   - Preserve existing admin execution endpoint.
   - Tests: admin tree from YANG, execution unchanged, JSON unchanged as far as documented.
   - Files: `handler_admin.go`, tests, possibly helper under `web` or `config/yang`.

7. **Phase 7: Promotion decision and cleanup plan**
   - Re-run the Promotion Criteria table against the live workbench. The decision is the conjunction of those criteria, not a judgment call.
   - If every criterion passes: write the follow-up cleanup scope to remove Finder-first UI files, route branches, rollback mode, the `ze.web.ui` env var, and obsolete tests.
   - If any criterion fails: revert the `environment.go` default to `finder`, remove V2 templates/handlers/tests, and document the rejection rationale in `plan/learned/`.
   - Tests: promotion/rejection is documentation and route-state driven; default mode must be exactly one active UI.

   ### Phase 7 readiness checklist (operator action)

   Phase 7 cannot land from a code-only session because the decision is gated on running `bin/ze-test web` against a hub launched in workbench mode. Before invoking the promote-or-reject branch:

   1. Build the hub: `make ze` (or `go build -o bin/ze ./cmd/ze`).
   2. Set `ZE_WEB_UI=workbench` for the test invocation so the hub serves V2.
   3. Run the canonical Phase 4 acceptance test: `bin/ze-test web -p workbench-bgp-change-verify`.
   4. Run the supporting workbench tests: `workbench-shell`, `workbench-bgp-peer-tools`, `workbench-bgp-pending-diff`, `workbench-tool-overlay-multi`.
   5. Run the rollback test: `bin/ze-test web -p workbench-rollback-finder` (default env, no `ZE_WEB_UI`).
   6. Read every Promotion Criteria threshold in this spec and confirm each holds.

   Any failure routes to the **Rejection branch** below. Otherwise route to the **Promotion branch**.

   ### Promotion branch (every Promotion Criterion passes)

   Cleanup scope, in commit order:

   1. Flip `internal/component/config/environment.go` `ze.web.ui` `Default` from `"finder"` to `"workbench"`. Remove the `PHASE 4 DEFAULT FLIP (PENDING)` comment.
   2. Update `TestUIMode_DefaultsToFinder` to assert the workbench default; rename the test if helpful.
   3. Remove the legacy Finder UI: `internal/component/web/handler.go` `RegisterRoutes` Finder branches, `internal/component/web/templates/page/layout.html` (Finder shell), Finder-only fragments (`finder.html`, `finder_oob.html`, `sidebar.html` if no longer used), Finder CSS sections in `assets/style.css`.
   4. Remove `HandleFragment` once `/show/`, `/monitor/`, and `/fragment/detail` are served by the workbench handler exclusively. Delete `handler.go`'s Finder dispatch branch in `cmd/ze/hub/main.go`.
   5. Remove the `ze.web.ui` env var registration from `internal/component/config/environment.go` and `GetUIMode`/`UIMode`/`ParseUIMode` from `internal/component/web/ui_mode.go` plus its tests.
   6. Remove `BuildAdminCommandTree` (the legacy static map) once no caller falls back to it; collapse the `cmd/ze/hub/main.go` admin tree branch to call `AdminTreeFromYANG` directly.
   7. Drop the rollback `.wb` test (`workbench-rollback-finder.wb`); rename `workbench-*.wb` files to drop the `workbench-` prefix.
   8. Update docs: replace "Finder columns" / "workbench" duality language with single-UI prose in `docs/architecture/web-interface.md`, `docs/guide/web-interface.md`, `docs/features/web-interface.md`.
   9. Write `plan/learned/<n>-web-2-promotion.md` summarising the Promotion Criteria results, the deliverables, and any deferred follow-ups.

   ### Rejection branch (any Promotion Criterion fails)

   1. Revert the Phase 4 default flip if it was committed (or, if the flip never landed, leave `environment.go` at `Default: "finder"`).
   2. Remove the workbench code added by this spec: `handler_workbench.go`, `handler_workbench_test.go`, `ui_mode.go`, `ui_mode_test.go`, `workbench_sections.go`, `workbench_render_test.go`, `workbench_enrich.go`, `workbench_enrich_test.go`, `handler_tools.go`, `handler_tools_test.go`, `related_resolver.go`, `related_resolver_test.go`, the workbench templates, the workbench CSS section, and the `/tools/related/run` route in `cmd/ze/hub/main.go`.
   3. Keep the `ze:related` YANG extension and parser if they are useful elsewhere; otherwise revert `ze-extensions.yang`, `internal/component/config/related.go`, the schema-node `Related` fields, and the BGP YANG annotations.
   4. Remove `AdminTreeFromYANG` only if no other caller uses it; otherwise keep it as a Finder bonus.
   5. Drop the `.wb` test files added by Phase 4.
   6. Write `plan/learned/<n>-web-2-rejection.md` with the failing Promotion Criterion(s), the operator-observed reason, and the design conclusion that motivates retiring V2.

8. **Phase 8: Documentation and final verification**
   - Update docs and feature summaries.
   - Run targeted tests and standard verification.
   - Perform critical/security review and fill implementation audit when implementation is complete.

## Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation and tests; BGP peer path works from browser to dispatcher to overlay. |
| Correctness | Tool command resolution uses schema metadata and current working tree, not stale committed tree. |
| Security | Browser cannot submit raw commands; unsafe placeholders rejected; output escaped; authz enforced by dispatcher. |
| UI preservation | Commit bar, CLI bar, config editing, and login/session behavior still work. |
| Naming | New YANG fields and JSON/form names are kebab-case where exposed; Go names are clear and narrow. |
| Data flow | No web import of BGP command packages; all operational execution goes through dispatcher. |
| Rule: derive-not-hardcode | Admin tree derived from YANG; related tools declared in schema metadata, not template conditionals. |
| Rule: V2 experiment exception | Through Phases 1-3, V2 is opt-in via `ze.web.ui=workbench` and Finder is the default; the Phase 4 default flip is gated by the Promotion Criteria; after promotion, only one UI exists; no shared hidden hybrid page; rejection path also documented. |
| Rule: no-layering after promotion | Finder-specific primary UI code removed or isolated once workbench shell is promoted; no duplicate nav systems left active unintentionally. |
| Test quality | Tests prove wiring from web endpoint, not just parser helpers. |
| Workflow quality | BGP edit -> review -> commit -> verify is tested from the browser and does not require page navigation. |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `ze:related` extension declared | Search `ze-extensions.yang` for extension and description. |
| MCP alignment (Q8) resolved before Phase 2 | Design Decisions table contains a documented resolution; commit message references it. |
| Argument wire format parser | Unit tests cover quoted values, escapes, unknown-key rejection. |
| Command-tree validation at schema load | Unit test proves a descriptor referencing a non-existent command fails the schema build. |
| Related metadata parsed | Run config/YANG unit tests and inspect BGP peer schema test. |
| Related tool endpoint registered | Search `cmd/ze/hub/main.go` for related tool route under auth wrapper. |
| Browser cannot submit command text | Handler test attempts forged command and captured dispatch proves it was ignored. |
| BGP peer row tools render | Web unit test and browser test verify row actions. |
| `${path-inherit}` works on `bgp/group/peer` | Resolver unit test exercises the peer -> group -> key fallback. |
| Tool output overlay renders | Render test and browser test verify overlay open/close. |
| Multiple overlays coexist with unique DOM ids | `web-tool-overlay-multi` browser test passes; render test asserts unique ids. |
| Tiered output handling | Render and browser tests cover inline preview, "Show full output" expansion, and >4 MiB truncation notice. |
| ANSI sequences stripped from output | Unit test feeds ANSI-escaped output and asserts plain text in the overlay payload. |
| Admin tree is YANG-derived | Unit test proves representative command from YANG appears without static map entry. |
| Workbench shell renders | Browser test verifies top bar + left nav + workspace. |
| BGP change-and-verify workflow works | Browser test edits a peer, sees pending marker, commits, and reruns peer detail from the same workspace. |
| Promotion Criteria measured | `web-workbench-bgp-change-verify` asserts every Promotion Criteria threshold; CI surfaces the metrics on failure. |
| Phase 4 default flip | `environment.go` default value flipped from `finder` to `workbench`; `web-workbench-default-flip` browser test passes. |
| V2 experiment is bounded | Phase 7 outcome (promote or reject) is documented in `plan/learned/` with one UI removed. |
| Existing config edit flow preserved | Existing web config tests pass. |
| Docs updated | Documentation checklist rows with Yes have file diffs. |

## Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Command injection | Browser request has no command field or ignores it; only trusted metadata creates command strings. |
| Placeholder injection | Placeholder values are token-validated before command construction. |
| Auth bypass | Tool route is registered under `authWrap`; handler requires username. |
| Authz bypass | Handler dispatches through `CommandDispatcher`, not direct handler calls. |
| CSRF posture | Existing SameSite cookie helps; mutating related tools should require POST and confirmation. Consider whether additional CSRF token is needed for destructive tools. |
| XSS | Command output and labels are HTML-escaped by templates; descriptors from trusted YANG still escape. |
| Output normalization | ANSI escape sequences and other terminal control characters are stripped server-side before rendering, regardless of source command. |
| Output exhaustion | Inline display capped at 128 KiB; full output buffered to 4 MiB max; beyond that, server truncates with a clear notice. The "Show full output" affordance never makes a second dispatch call. |
| Path traversal | Context path validated with `ValidatePathSegments`; placeholders can only read within config tree. |
| Sensitive data | Overlay should not expose sensitive config values through placeholders; do not add tools that print secrets. |
| Click accidents | Tools with `confirm` metadata require explicit confirmation before dispatch. |

## Failure Routing

| Failure | Route To |
|---------|----------|
| Descriptor grammar proves brittle | Revisit design before implementing more annotations; prefer explicit parser with strict tests. |
| YANG parser cannot carry extension argument as needed | Switch to simpler descriptor format or separate extension fields before writing UI code. |
| MCP alignment (Q8) cannot be resolved | Block Phase 2. Either land a shared metadata model in `plan/spec-mcp-5-apps.md` first, or document the divergence in Design Decisions with the maintenance plan, before any `ze:related` code lands. |
| Command-tree validation rejects existing real commands at schema-load | Investigate command tree completeness first; do not weaken validation to bypass. If the command tree is genuinely missing a command, fix the registration. |
| `${path-inherit}` walks more than one parent on `bgp/group/peer` | Out of scope: cap inheritance at one parent walk in v1. Add deeper inheritance only with a new placeholder source and explicit tests. |
| Command selector resolution fails for BGP peer commands | Re-read dispatcher peer selector extraction; adjust descriptor command templates rather than special-casing BGP in web. |
| Overlay response breaks HTMX/OOB behavior | Re-read web-components OOB rules and existing diff/add overlay patterns. Verify per-instance overlay ids do not collide. |
| Multiple overlays leak DOM when one is closed | Audit close handler: it must remove only its own `#overlay-<id>` node, not the parent `#tool-overlays` container. |
| Output truncation drops the "Show full output" affordance | Re-test with output between 128 KiB and 4 MiB; the affordance must always render in that range. |
| ANSI strip removes legitimate output content | Tighten the strip rule to control sequences only (CSI/OSC/ESC); printable text including UTF-8 must pass through unchanged. |
| Promotion Criteria fail in Phase 4 | Do not flip the default. Route to Phase 7 rejection branch: revert default to Finder, remove V2 templates/handlers/tests, document rationale in `plan/learned/`. |
| Workbench shell breaks CLI context | Re-read `plan/learned/486-cli-nav-sync.md` and fix OOB path/prompt/context swaps. |
| V2 routing starts sharing too much layout code with Finder UI | Split V2 page/template concerns; share only domain data builders and existing editor/session/dispatcher services. |
| BGP change-and-verify requires page navigation | Stop and redesign the workspace interaction before adding more sections. |
| Authz denial appears as generic 500 | Route error through overlay error state; preserve status code if tests expect it. |
| Three fix attempts fail | Stop, document all three attempts, ask the user for the next design decision. |

## Open Questions

| ID | Question | Default Recommendation |
|----|----------|------------------------|
| Q1 | Should the first implementation include `ze:nav-section`, or start with a small Go section taxonomy? | Start with a small taxonomy only if it is one file and schema-derived availability; add `ze:nav-section` if mappings spread. |
| Q2 | Should related tool descriptor reference user-facing command templates or WireMethod plus argument template? | Resolved: user-facing command templates. Templates are validated at schema-load time against the YANG command tree using a synthesized canonical row (see Metadata Validation Rules); broken or renamed commands fail the build, not at click time. |
| Q3 | Should tool overlays support structured JSON rendering immediately? | No. Render safe text first; add renderer registry later. |
| Q4 | Should mutating related tools be hidden for read-only users before click? | Resolved: no for v1. See Known v1 Limitations. v2 may add authz prediction. |
| Q5 | Should the tool overlay be modal or side drawer by default? | Modal by default; drawer for peer detail where comparing with row/table is useful. |
| Q6 | What V2 entry point should be used? | Resolved: `/show/`. Phases 1-3 require `ze.web.ui=workbench` to opt in; Phase 4 flips the default. |
| Q7 | What is the first acceptance workflow? | BGP peer edit -> diff/review -> commit -> peer detail/statistics rerun without page navigation. Quantified by the Promotion Criteria. |
| Q8 | Can `ze:related` and the MCP UI-resource extension share one metadata model? | Resolved (D16): no. The two extensions annotate different schema sites, describe different artifacts (executable command templates vs. static asset URIs), and enforce different trust contracts. They remain as separate `ze:` YANG extensions with cross-references in `ze-extensions.yang` descriptions. |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| None yet | - | - | - |

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Hardcoded BGP-only tool buttons | Would duplicate command knowledge in templates and not scale to plugins | `ze:related` metadata |
| Reusing `/admin` pages for related tools | Replaces workspace and accepts URL-shaped command path | Dedicated related tool route |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| None yet | - | - | - |

## Design Insights

- The related tool feature should be treated as a secure command-construction problem first and a UI convenience second.
- Existing `CommandDispatcher` already has the identity/authz properties needed for web-triggered operational commands.
- Admin command navigation is still static despite the YANG command-tree work. This spec should fix that because related tools need command-tree parity.
- BGP peer selectors are subtle: command strings can include peer selector tokens that the dispatcher strips before matching the registered command. Tests must cover the exact command template used by related tools.
- The web UI is a deliberate exception to no-layering while V2 is evaluated, structured as a phased default switch: Finder is the default through Phases 1-3, V2 becomes the default after the Phase 4 Promotion Criteria pass, and the rejected UI is deleted in Phase 7. There is no period of permanent dual UI.
- The innovative part is the compressed operator loop: edit, review, commit, and verify from the same object workspace.
