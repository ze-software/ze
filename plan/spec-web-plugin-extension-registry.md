# Spec: web-plugin-extension-registry

| Field | Value |
|-------|-------|
| Status | design |
| Scope | web, operator-workbench, plugin |
| Depends | existing workbench shell, `ze:related` tools, `WebRoute` registry |
| Phase | design |
| Handoff | define the three web modes, implement typed workbench contribution registry, then migrate BGP first |
| Updated | 2026-09-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Build the next web UI as one of three explicit web modes, not as a replacement
for the existing YANG-based interface. The web product must expose:

1. **CLI over HTTPS** - a CLI-like web terminal that replicates the Ze CLI
   interaction through the authenticated web server.
2. **YANG editor** - the current complete YANG-based editor, visually the same
   Finder-style interface available today.
3. **Operator workbench** - the new RouterOS/FortiOS-inspired table-first
   interface with top navigation, left navigation, contextual tools, and
   reduced change/verify friction.

The workbench must look and behave more like RouterOS WebFig / WinBox and
FortiOS: table-first pages, top navigation, left navigation for major sections,
and contextual operational tools that run from the page the engineer is already
using.

The specific architectural requirement is that the workbench must not become a
single monolithic blob. Each feature must be able to register the part of the UI
it owns. That registration can live in a separate `<feature>-web` module/package
or in the feature package itself, depending on build-tag, dependency, and
ownership constraints.

**User constraints captured 2026-09-11:**

- Keep the YANG-based interface. This work is about the alternative workbench,
  not deleting or hiding the generic YANG editor.
- Define the three modes above as first-class product surfaces. The current
  code has `/cli` and the YANG/Finder/workbench selector, but the design must
  no longer describe the web UI as a single mode plus a rollback.
- Keep `ze_web` as the base web-server selector. The three web modes get
  separate inclusion selectors: `ze_web_cli`, `ze_web_yang`, and
  `ze_web_workbench`.
- A feature-owned workbench contribution file must require both the workbench UI
  selector and the owning feature selector. For example, BGP workbench
  registration uses `//go:build ze_web && ze_web_workbench && ze_bgp`.
- The operator workflow is the point of innovation: make a change, check if it
  worked, and minimize page changes or other friction.
- Related tools must appear through the `ze:` metadata path so operational
  commands can run and return output in an overlay.
- The web UI must be modular. Different components/features register their
  own pages, nav entries, tables, tools, widgets, and monitors. The central web
  package composes them; it does not know every feature-specific page.

## Product Definition of Good

The high-level product/design quality bar is
[`plan/design/web-operator-workbench-definition-of-good.md`](design/web-operator-workbench-definition-of-good.md).
That document requires a separate implementation-ready spec for every substantial
workbench feature, page family, or cross-cutting objective before coding starts.

## External UI Research

### RouterOS / WebFig observations

Source: MikroTik WebFig documentation
<https://help.mikrotik.com/docs/spaces/ROS/pages/328131/WebFig>

- WebFig explicitly combines configuration, monitoring, and troubleshooting in
  one browser UI. That maps to Ze's desired engineer loop: change config, observe
  live state, and run diagnostics without leaving the surface.
- WebFig exposes a terminal from the top-right of the UI. Ze should keep the CLI
  reachable, but the more important workbench pattern is contextual tools: row
  actions and command overlays before forcing the engineer into a raw terminal.
- WebFig skins let an operator hide menus, rename menus, add notes, make items
  read-only, and add tab/separator structure. Ze should not clone the skin
  mechanism in this spec, but should learn from it: the workbench registry needs
  stable IDs and metadata so sections can later be shown, hidden, ordered, or
  renamed without editing a central switch.

Source: RouterOS BGP documentation
<https://help.mikrotik.com/docs/spaces/ROS/pages/328220/BGP>

- RouterOS separates BGP configuration and session monitoring into explicit
  menus: instance, connection, session, and template. Ze should keep the config
  table and live session state near each other, but not collapse them into a
  single generic schema tree.
- The BGP session view shows flags, remote/local session data, negotiated
  capabilities, counters, last notification, and uptime. Ze's BGP workbench page
  should make these operational values first-class table columns and detail tabs.

### FortiOS observations

Source: FortiOS GUI overview
<https://docs.fortinet.com/document/fortigate/7.4.0/administration-guide/130914/using-the-gui>

- FortiOS treats menus, tables, value entry, global search, and command palette
  as separate GUI concepts. Ze should keep those as separate registered
  contribution types, not a page-only registry.

Source: FortiOS table behavior
<https://docs.fortinet.com/document/fortigate/7.6.1/administration-guide/572053/tables>

- FortiOS tables support filter bars, column filters, filters from a cell's
  content, column settings, rearrangement, resizing, inline edits, copy/paste,
  and multiple simultaneous filters. Ze v1 should implement the parts that
  reduce network-operator friction immediately: server-side filtering,
  column visibility/order, row actions, copy/clone where the backend supports
  it, and lazy loading/pagination for large operational datasets.

Source: FortiOS policy list performance improvements
<https://docs.fortinet.com/document/fortigate/7.4.0/new-features/846522/improve-the-performance-of-the-gui-policy-list>

- FortiOS moved heavy policy pages toward loading only the data needed for the
  current view, editing in a pane rather than a page switch, row menus, insertion
  gutters, right-click actions, and multi-select. Ze should apply the same ideas
  to firewall rules, BGP policy, and large route tables.

Source: FortiOS command palette
<https://docs.fortinet.com/document/fortigate/7.6.5/administration-guide/861507>

- FortiOS supports keyboard entry for jumping to pages, opening monitors without
  changing the current page, running CLI diagnostics, searching within output,
  copying output, downloading output, and rerunning commands. Ze should implement
  this as a workbench command palette backed by registered pages and `ze:related`
  operational commands.

Source: FortiOS simultaneous packet captures
<https://docs.fortinet.com/document/fortigate/7.4.0/new-features/677254/run-simultaneous-packet-captures-and-use-the-command-palette>

- FortiOS can run multiple captures at once, minimize/dock them, and keep them
  running in the background. Ze's overlay should become a job dock, not a single
  modal that disappears when the user changes pages.

### Research decisions

| Decision | Basis |
|----------|-------|
| Keep configuration and operational state together on domain pages | RouterOS WebFig explicitly covers configure/monitor/troubleshoot; FortiOS command palette can open monitors without changing page |
| Keep YANG as the canonical schema/debug/editor fallback | User correction and Ze's current architecture: generic YANG rendering is already the complete coverage path |
| Use tables as the primary list surface | RouterOS and FortiOS both present named/network objects as lists/tables; Ze's current workbench already started this pattern |
| Use contextual overlays/job dock for operational commands | FortiOS diagnostics and packet capture flows reduce page changes; Ze already has `ze:related` descriptors and `/tools/related/run` |
| Introduce typed contribution registration | Current workbench pages/nav are hardcoded in central files; user explicitly rejected a monolithic blob |

## Current Behavior

**Source files read:**

- `internal/component/web/webroute.go`
- `cmd/ze/hub/service_web.go`
- `internal/component/web/handler_workbench.go`
- `internal/component/web/workbench_pages.go`
- `internal/component/web/workbench_sections.go`
- `internal/component/web/ui_mode.go`
- `internal/component/web/cli.go`
- `internal/component/web/cli_terminal.go`
- `internal/component/web/handler_tools.go`
- `internal/component/web/related_resolver.go`
- `internal/component/plugin/all/all_ze_web.go`
- `internal/component/plugin/all/all_ze_radius_ze_l2tp.go`
- `internal/component/web/page_l2tp.go`
- `internal/component/web/page_l2tp_off.go`
- `internal/component/config/related.go`
- `docs/architecture/web-interface.md`
- `docs/architecture/web-components.md`
- `docs/architecture/web-workbench-pages.md`
- `plan/design/web-operator-workbench-definition-of-good.md`

### What already exists

- `internal/component/web/webroute.go` provides `RegisterWebRoute` and
  `RegisteredWebRoutes`. It lets in-tree web features register HTTP routes and
  portal entries without editing `cmd/ze/hub/service_web.go`.
- `cmd/ze/hub/service_web.go` iterates `RegisteredWebRoutes()` at startup,
  wraps routes by `WrapKind`, and registers portal menu entries when present.
- The current default `/show/` UI is the workbench; Finder remains available via
  `ze.web.ui-mode=finder` and the current `ze-ui-mode` cookie.
- `/cli` already exists as a full CLI terminal page. `cli_terminal.go` supports
  operational and configuration modes through `/cli/terminal`, returning a JSON
  envelope that the existing `cli.js` client renders into the terminal viewport.
- `ui_mode.go` currently models only Finder and Workbench as selectable
  `UIMode` values. `uiModeTokenCLI` exists only as an `ActiveUI` marker for the
  `/cli` page and is explicitly not a selectable UI mode today.
- The existing `ze_web` build tag gates web service registration and the web
  YANG module (`register_web.go`, `service_web.go`, `all_ze_web.go`). It is
  already the correct foundation gate for serving web surfaces.
- Combined build tags are already normal in this repo, for example
  `ze_core && ze_bgp` and `ze_l2tp && ze_radius`.
- L2TP workbench code currently gates direct L2TP imports with `ze_l2tp` and
  provides a `!ze_l2tp` disabled-page stub. The new registry should prefer not
  registering absent optional workbench pages over central disabled-page stubs.
- `handler_workbench.go` reuses the existing editor/session/YANG fragment path
  for generic pages and calls `renderPageContent` for purpose-built workbench
  pages.
- `workbench_sections.go` owns a central hardcoded two-level left navigation
  taxonomy.
- `workbench_pages.go` owns a central hardcoded dispatcher for purpose-built
  pages.
- `handler_tools.go` already implements `POST /tools/related/run`. The browser
  sends only `tool_id` and `context_path`; the server resolves a `ze:related`
  descriptor, substitutes placeholders against the user's working tree, validates
  the resolved command, dispatches through the normal command path, and renders
  an overlay.
- `related_resolver.go` validates placeholder resolution and command length.
- `config/related.go` parses `ze:related` YANG descriptors into typed
  `RelatedTool` metadata.

### Behavior to preserve

- The YANG/Finder UI remains available and must keep covering schema paths that
  have no purpose-built workbench page.
- The web CLI remains available as an authenticated HTTPS surface and must keep
  using the same editor/session/dispatcher/authorization path as the SSH CLI
  and web admin command surfaces.
- `ze_web` must remain sufficient to build the base web server, but mode source
  files should be split so CLI, YANG editor, and Workbench can be included or
  omitted by their own selectors.
- The current editor model remains: per-user working tree, pending changes,
  diff, commit, conflict handling, and reload hook.
- `ze:related` remains the source of truth for operational row/detail actions
  that are naturally tied to a schema node.
- Browser-submitted commands remain forbidden. Browser forms send IDs and
  context, while server-side metadata resolves the command.
- Web routes remain auth-wrapped by the hub; feature registries must not bypass
  authentication, same-origin checks, edit authorization, or audit.

### Behavior to change

- Replace the central workbench nav taxonomy with a typed contribution registry.
- Replace the central purpose-built page dispatcher with registered page
  contributions.
- Promote the product model from two `/show/` shells plus an incidental `/cli`
  page into three named modes: CLI, YANG editor, and Workbench. Implementation
  can still serve CLI at `/cli`, but the top navigation and docs must present it
  as a peer mode rather than a side link.
- Let feature-owned packages register pages, tables, dashboard widgets,
  row/detail tools, command-palette entries, and monitor panels without editing
  central web files.
- Upgrade related-tool overlays toward FortiOS-style operational output
  overlays/job dock: searchable, copyable, downloadable, rerunnable, and able to
  survive page navigation where the command is long-running.

## Problem Statement

The repo has crossed the line that `workbench_sections.go` warned about:
"v1 keeps the section taxonomy in one Go file" and if entries spread, "switching
to metadata first" is required. The workbench now has domain-specific pages for
BGP, interfaces, IP, firewall, system, services, VPN, tools, logs, dashboard,
and L2TP. Those are useful, but new work now requires editing central dispatch
and navigation code.

That is exactly the monolithic failure mode the user wants to avoid. It creates
three risks:

1. A feature cannot ship a coherent UI surface with the feature that owns the
   domain model.
2. Build-tagged features can leave dead nav entries or empty pages behind.
3. The central web package becomes the dependency magnet for every operational
   feature, making compile-out and plugin removal tests harder.

## Scope

### In scope

- A typed workbench contribution registry in `internal/component/web`.
- A first-class three-mode web contract and selector: CLI over HTTPS, YANG
  editor/Finder, and Operator Workbench.
- A build-tag inclusion model where `ze_web` gates the base web server,
  `ze_web_cli` gates CLI-over-HTTPS, `ze_web_yang` gates the YANG editor, and
  `ze_web_workbench` gates the new operator workbench UI contributions.
- Feature registration for:
  - left-navigation sections and sub-pages,
  - purpose-built pages,
  - table definitions and server-side table state,
  - dashboard widgets,
  - monitor panels,
  - command-palette entries,
  - related operational tools and job overlays.
- Support for both module placement styles:
  - `internal/component/<feature>/web` or `internal/component/<feature>-web`
    when the UI has extra dependencies or build tags,
  - `internal/component/<feature>` when the UI is small and does not create an
    unwanted dependency cycle.
- Migration of BGP as the first proof that a feature can own its workbench UI.
- A per-feature/per-objective spec decomposition that lets a fresh agent
  implement each workbench slice without hidden context.
- Tests that prove a feature can add/remove UI without central workbench edits.

### Spec decomposition

The registry spec is the foundation spec. It does not authorize implementing
every workbench page in one pass. Each substantial feature or objective gets its
own implementation-ready spec before code changes start. That spec may depend on
underlying module work, but it must name the dependency and its readiness.

Minimum spec slices expected after the registry foundation:

| Slice | Spec name | Primary dependencies to declare |
|-------|-----------|----------------------------------|
| Workbench registry foundation | `plan/spec-web-plugin-extension-registry.md` | `internal/component/web`, `WebRoute`, editor/session/commit path |
| BGP workbench | `plan/spec-web-workbench-bgp.md` | BGP config YANG, BGP command handlers, live peer/session state APIs |
| Interface workbench | `plan/spec-web-workbench-interfaces.md` | iface config YANG, netlink/interface state, counters, packet capture tools |
| IP and routes workbench | `plan/spec-web-workbench-ip-routes.md` | address config, kernel/FIB route state, route table caps/pagination |
| Firewall and policy workbench | `plan/spec-web-workbench-firewall-policy.md` | firewall config, applied backend snapshot, counters/logs, rule insertion APIs |
| L2TP workbench | `plan/spec-web-workbench-l2tp.md` | L2TP config, session snapshot, disconnect/clear commands, CQM data |
| Services/system workbench | `plan/spec-web-workbench-system-services.md` | service YANG roots, users/authz, host resources, hardware inventory |
| Tools/logs/dashboard | `plan/spec-web-workbench-tools-logs-dashboard.md` | command dispatcher, event broker, health registry, log/event APIs |
| Command palette | `plan/spec-web-workbench-command-palette.md` | registry snapshot, command tree, safe diagnostics, config object indexing |
| Related job dock | `plan/spec-web-workbench-related-job-dock.md` | `ze:related`, command dispatcher, output caps, streaming/job lifecycle API |
| Table state | `plan/spec-web-workbench-table-state.md` | shared table component, query parsing, server-side sort/filter/page support |

Every slice spec must list:

- underlying modules and build tags;
- data sources and ownership boundaries;
- fallback behavior when a module is absent or not ready;
- accepted placeholders, if the UI can land before live module state exists;
- acceptance criteria and tests that prove the slice works end to end.

### Out of scope

- Deleting the YANG/Finder interface.
- Replacing YANG schema ownership with hand-written form schemas.
- Accepting raw command strings from the browser.
- Rewriting the whole workbench in a client-side framework.
- Arbitrary plugin-supplied HTML in the authenticated admin UI.
- A full skin/theme system. The registry should leave room for later operator
  customization, but this spec does not implement skins.

## Design Gate

### Option A: extend `WebRoute` only

Keep `RegisterWebRoute` as the single web extension point and ask each feature
to expose whole HTTP routes.

**Pros:** exists today; keeps routing decoupled from the hub.

**Cons:** does not solve left navigation, page dispatch, dashboard widgets,
table behavior, or command palette. It makes route ownership better but the
workbench itself remains central.

**Verdict:** keep `WebRoute`, but it is not enough.

### Option B: encode the workbench entirely in YANG extensions

Add YANG extensions for nav sections, table columns, widgets, and actions. The
workbench derives everything from schema metadata.

**Pros:** keeps one canonical source of configuration truth; naturally compiles
out with schema modules; good for `ze:related`, help, field hints, and generic
table annotations.

**Cons:** operational views often combine config, runtime state, command output,
and process-local snapshots. YANG can describe config and metadata, but not
every data source or renderer. Complex page composition becomes awkward in YANG
strings.

**Verdict:** use YANG metadata where it is the right authority (`ze:related`,
labels, help, field hints), but not as the only workbench registration path.

### Option C: typed workbench contribution registry

Add a registry beside `WebRoute`. Features register typed contribution structs:
nav items, pages, table providers, dashboard widgets, monitor panels, palette
entries, and job/tool definitions. The web shell sorts, validates, auth-filters,
and renders these contributions.

**Pros:** solves the monolith directly; keeps rendering server-side and typed;
preserves build-tag compile-out; allows `ze:related` to remain schema-driven;
lets BGP, iface, firewall, L2TP, and future plugins evolve their own operator UI
without central switches.

**Cons:** new API surface must be stable enough that feature packages can depend
on it; collision and ordering rules must be explicit; without discipline,
features could duplicate generic YANG editor behavior.

**Recommended:** Option C, with YANG metadata retained for schema-owned facts and
`WebRoute` retained for whole-route extensions.

## Architecture

### Build-tag inclusion model

The existing `ze_web` tag remains the base web service selector. It means
"compile the authenticated web server foundation": TLS/listeners, auth/session,
assets, renderer setup, editor/session integration, and shared route wiring.
The individual web modes then have their own selectors:

| Selector | Meaning |
|----------|---------|
| `ze_web` | Base authenticated web server foundation |
| `ze_web_cli` | CLI over HTTPS mode (`/cli`, `/cli/terminal`, completion and terminal assets) |
| `ze_web_yang` | YANG editor mode, currently the Finder-style generic YANG interface |
| `ze_web_workbench` | New operator workbench mode and its contribution registry |
| `ze_<feature>` | Owning feature implementation, such as `ze_bgp` or `ze_l2tp` |

Feature-specific workbench files must require web foundation, workbench UI, and
the owning feature. Examples:

```go
// BGP workbench registration and BGP-owned workbench components.
//go:build ze_web && ze_web_workbench && ze_bgp

// L2TP workbench registration and L2TP-owned workbench components.
//go:build ze_web && ze_web_workbench && ze_l2tp

// Always-on base workbench shell/registry files, if split from current ze_web files.
//go:build ze_web && ze_web_workbench
```

If a future system page has a feature selector, for example `ze_system`, it uses
`ze_web && ze_web_workbench && ze_system`. If it is truly always-on, it uses
`ze_web && ze_web_workbench` only.

Build profiles that currently use `ze_web` and expect today's complete web UI
must explicitly include the desired mode selectors. A compatibility build can
include `ze_web ze_web_cli ze_web_yang ze_web_workbench`; an appliance that
wants only CLI over HTTPS can include `ze_web ze_web_cli`.

### Three web modes

The web interface has three product modes. They share authentication, session
identity, authorization, audit, and command dispatch, but they optimize for
different operator needs.

| Mode | Current route / code evidence | Purpose | Must preserve |
|------|-------------------------------|---------|---------------|
| CLI over HTTPS | `/cli`, `HandleCLIPageHTTP`, `HandleCLITerminalWithDispatchAuthorizerAndAudit`, `component_cli_terminal.templ` | Let an operator use the CLI from a browser when that is faster or required | Same command grammar, RBAC, audit, editor session, command dispatcher, and safe output handling as the rest of Ze |
| YANG editor | `/show/` with `UIModeFinder`, `HandleFragment`, Finder components | Complete schema-backed config editor and recovery/fallback view | Full YANG coverage, generic edit path, current visual behavior, `ze.web.ui-mode=finder` rollback |
| Operator workbench | `/show/` with `UIModeWorkbench`, `HandleWorkbench`, workbench components | Task-oriented tables, details, operational state, and contextual tools | YANG-backed writes, generic fallback, related-tool safety, feature-owned contribution registration |

The implementation may keep CLI on `/cli` instead of routing it through
`/show/`, but it must be first-class in the top navigation, mode names, docs,
and tests. `uiModeTokenCLI` should stop being only an internal layout marker if
the UI selector stores the active mode in a cookie or config value.

The YANG editor should be named as a YANG editor in user-facing copy even if the
implementation token remains `finder` for compatibility. Finder is the visual
presentation; YANG editor is the product mode.

### Core types

Add a workbench registry package inside `internal/component/web`, likely:

- `workbench_registry.go`
- `workbench_registry_test.go`
- `workbench_registry_reset_test.go` if tests need isolated global state

The first implementation can stay in package `web` to reuse existing unexported
view models. If dependency pressure grows, split only the registration structs
into a subpackage such as `internal/component/web/workbench`.

```go
type WorkbenchContribution struct {
    ID        string
    Owner     string
    BuildTags []string
    Nav       []WorkbenchNavContribution
    Pages     []WorkbenchPageContribution
    Tables    []WorkbenchTableContribution
    Widgets   []WorkbenchWidgetContribution
    Monitors  []WorkbenchMonitorContribution
    Palette   []WorkbenchPaletteContribution
}

type WorkbenchNavContribution struct {
    SectionID    string
    Section      string
    SectionIcon  string
    SectionOrder int
    PageID       string
    Label        string
    Path         string
    Order        int
    Capability   string
}

type WorkbenchPageContribution struct {
    ID          string
    PathPrefix  []string
    Exact       bool
    Order       int
    Capability  string
    Render      WorkbenchPageRenderer
}

type WorkbenchPageRequest struct {
    Request  *http.Request
    Renderer *Renderer
    Schema   *config.Schema
    Tree     *config.Tree
    Dispatch CommandDispatcher
    Broker   *EventBroker
    ReadOnly bool
    Username string
}

type WorkbenchPageRenderer func(WorkbenchPageRequest) (template.HTML, bool)
```

Exact names can change during implementation, but the contract must stay typed:
features return data/HTML through registered renderers and shared components,
not by injecting raw unreviewed markup into the shell.

### Contribution ownership

A feature may register UI in either shape:

1. **Feature-local registration**
   - Example: `internal/component/bgp/workbench_register.go`
   - Use when the page has no extra dependency that would create a cycle and
     the feature already imports the web package for route or metadata support.

2. **Feature web module/package**
   - Example: `internal/component/bgp/web/register.go` or
     `internal/component/bgp-web/register.go`
   - Use when the UI needs templ components, web-only dependencies, or build
     tags distinct from the protocol core.
   - The package is blank-imported only when `ze_web`, `ze_web_workbench`, and
     the feature build tag are enabled.

The registry must not care which shape is used. It only sees typed contributions
at init/startup.

### HTMX component composition

Registered feature UI must compose through server-rendered components and HTMX,
not through a monolithic client app.

- The shell owns the top bar, mode selector, left nav composition, workspace
  frame, pending-change indicator, error panel, and overlay/job dock targets.
- Feature contributions own the content fragment for their pages and the data
  builders behind those fragments.
- Feature pages must be renderable as full-page content and as HTMX fragments
  for in-place navigation.
- Mutation responses must keep using OOB swaps for commit bar, errors, path
  context, and other chrome that changes outside the primary target.
- Features may use existing shared components (`workbenchTable`, detail panels,
  forms, tool overlays) or add new one-concern templ components in their web
  package.
- No feature should construct HTML in Go strings. The existing markup guards
  should continue to reject that pattern.

### Registry lifecycle

- Feature packages call `web.RegisterWorkbenchContribution(...)` from `init()`.
- The hub builds the web server as it does today.
- `HandleWorkbench` receives an immutable registry snapshot at server start, or
  the renderer reads one immutable snapshot during request setup.
- Startup validates duplicate contribution IDs, duplicate page IDs, duplicate
  nav `(sectionID,pageID)` pairs, conflicting exact paths, invalid path
  segments, nil renderers, unsupported icon tokens, and invalid order/capability
  metadata.
- Validation failures should be logged as warnings for optional feature UI and
  fail tests. If a central in-tree contribution is malformed, tests must fail.

### Relationship to existing registries

| Existing surface | Keep / extend |
|------------------|---------------|
| `RegisterWebRoute` | Keep for whole HTTP route ownership and portal iframe entries |
| `RegisterPortalService` | Keep initially; later it can be backed by a portal contribution type |
| `/cli` terminal handlers | Keep as CLI-over-HTTPS mode; gate source files with `ze_web && ze_web_cli` where practical, and wire it into the mode selector rather than treating it as a workbench detail |
| `ui_mode.go` | Extend or wrap so the product has CLI, YANG editor, and Workbench names; keep `finder` token as compatibility alias; hide or refuse modes not compiled into the binary |
| build-tag generated imports | Add mode-scoped imports for workbench contribution packages, with feature-specific conjunctions such as `ze_web && ze_web_workbench && ze_bgp` |
| `ze:related` | Keep as schema-owned related command metadata |
| `fieldInputs` registry | Keep for generic YANG field rendering |
| `workbenchSections()` | Replace hardcoded definitions with a registry-backed builder |
| `renderPageContent()` | Replace central switch with registry dispatch, retaining generic YANG fallback |

### Navigation model

The workbench shell owns visual layout. Features register nav metadata: stable
section ID, section label and icon, section order, page ID, page label, page
path, page order, and optional capability/auth profile.

The shell composes all contributions into the left nav. It sorts by order, then
label, then ID for deterministic output. It computes selected/expanded state
from the current path. It hides pages the current user cannot access.

The old section set becomes seed data registered by built-in packages. BGP is
the first page family to move out of the central list.

The left nav belongs only to the Operator Workbench. The CLI mode has CLI-native
navigation and prompt context. The YANG editor keeps the Finder/YANG navigation
that exists today.

### Page dispatch model

Registered pages are matched by path:

- exact page match first,
- longest prefix match second,
- lower `Order` wins ties,
- if no registered page handles the path, fall back to the existing generic
  YANG detail renderer.

This is important for BGP:

- `/show/bgp/peer/` renders the registered BGP peers table.
- `/show/bgp/peer/<name>/` can fall through to the generic YANG editor unless
  the BGP contribution explicitly registers a richer detail page.

### Tables

The table contribution should not immediately replace every existing
`WorkbenchTableData` builder. Instead:

1. Keep `WorkbenchTableData` and `workbenchTable` as the shared render target.
2. Add metadata around it: table ID, default columns, sortable columns,
   filterable columns, row key, default sort, lazy/paginated data source flag,
   and row action providers.
3. Add a table state parser for query params: `q`, `sort`, `dir`,
   `filter.<column>`, `columns`, `page`, `limit`.

FortiOS-inspired requirements:

- every table has visible column headers even when empty,
- filter chips remain visible and removable,
- column visibility/order is per table and can be persisted later,
- large operational tables are capped or paginated server-side,
- row actions can open overlays without navigating,
- edit/create happens in a drawer or pane where the current table remains in
  view when feasible.

### Related tools and job dock

The current `POST /tools/related/run` handler is the right safety model. The
next step is presentation and lifecycle:

- Add overlay header actions: search within output, copy, download, rerun,
  pin/unpin, and close.
- Preserve multiple overlays at once.
- Add a job dock for commands whose dispatcher supports streaming or background
  operation.
- Keep the browser form shape: `tool_id`, `context_path`, optional
  `confirm=true`, optional `job_id` for polling/rerun. Never accept raw command
  text.
- Use `ze:related` for schema-tied operations; use registered palette/tool
  contributions for global tools that have no schema context.

Example BGP related tools remain schema-owned: peer detail, capabilities,
statistics, route refresh, soft reset, and teardown with confirmation.

### Command palette

Add a top-bar command palette inspired by FortiOS:

- `Cmd/Ctrl+P`: jump to page.
- `>` prefix: run registered action/tool.
- `/` prefix: run registered diagnostic command.
- `?` or no prefix: search configuration objects and pages.

The palette index is built from registered nav/page contributions, registered
palette contributions, `ze:related` descriptors visible for the current
page/selection, command tree entries that are safe diagnostics, and config
object labels derived from the user's working tree.

Command palette actions must dispatch through the same endpoint model as row
tools. The palette submits IDs, not raw commands.

### Dashboard and monitor panels

Dashboard widgets are registered separately from pages:

- summary widgets: BGP sessions, interface health, system resources,
  warnings/errors, recent events,
- monitor panels: live log, packet capture, interface counters, BGP session
  state, route table view.

Widgets must state their data source and refresh strategy: static config
snapshot, command dispatcher polling, HTMX fragment polling, existing
`EventBroker` SSE, or feature-specific stream.

No widget may read another feature's process-local state unless that feature
exports a typed read API or a command/monitor surface.

## Data Flow

### Mode selection

1. User opens the web UI.
2. The top navigation exposes CLI, YANG editor, and Workbench as peer modes.
3. CLI navigates to `/cli` and uses `/cli/terminal` for command execution.
4. YANG editor navigates to `/show/` with the YANG/Finder mode selected.
5. Workbench navigates to `/show/` with the Workbench mode selected.
6. The selected mode is stored in the existing switch-cookie/config mechanism or
   an explicit successor, with `finder` kept as a compatibility alias for the
   YANG editor.

### Entry point

`cmd/ze/hub/service_web.go` starts the web service and wires renderer, YANG
schema, committed config tree, per-user editor manager, command dispatcher,
authz/audit, event broker, and the workbench registry snapshot.

### Request flow

1. User requests `/show/<path>`.
2. Hub auth middleware validates the session.
3. `HandleWorkbench` selects the per-user working tree if one exists.
4. Workbench registry dispatch tries to render a registered page.
5. If no registered page handles the path, generic YANG detail renders.
6. Shell renders top bar, registry-backed left nav, page content, commit state,
   error panel, and overlay/job dock.

### Related tool flow

1. User clicks a row/detail/palette tool.
2. Browser posts `tool_id` and `context_path`.
3. Server validates auth/session/same-origin.
4. Server locates the descriptor from `ze:related` or a registered tool source.
5. Server resolves placeholders against the user's working tree.
6. Server dispatches through the standard command dispatcher.
7. Output appears in an overlay or job dock without changing the current page.

### Boundaries crossed

| Boundary | How | Constraint |
|----------|-----|------------|
| Browser to web server | HTMX request | IDs and context only; no raw commands |
| Web to config editor | `EditorManager.Tree(username)` | per-user draft tree, not another user's tree |
| Web to command runtime | `CommandDispatcher` | same authz/accounting path as CLI/admin |
| Feature to web shell | typed contribution structs | no central page/nav switch edits |
| Schema to workbench | `ze:related`, labels, help, decorators | schema metadata remains authoritative |

## Acceptance Criteria

### Mode criteria

| Mode AC ID | Input / Condition | Expected Behavior |
|------------|-------------------|-------------------|
| MODE-1 | Operator selects CLI mode | Browser opens the authenticated web CLI page; commands execute through `/cli/terminal` with the same editor, RBAC, audit, dispatcher, and output safety as today |
| MODE-2 | Operator selects YANG editor mode | Browser opens the current Finder/YANG editor; all generic schema-backed config paths remain editable and visually match the current interface |
| MODE-3 | Operator selects Workbench mode | Browser opens the new table-first workbench; registered feature pages render through the shell and generic YANG fallback remains available |
| MODE-4 | `ze.web.ui-mode=finder` is set | Startup/default mode selects the YANG editor rollback, not the workbench |
| MODE-5 | A stale old `ze-ui` cookie exists | It does not force the operator into the old mode; only the current mode selector mechanism is honored |
| MODE-6 | Binary is built with `ze_web ze_web_cli` only | CLI over HTTPS works; YANG editor and Workbench are not exposed as selectable modes |
| MODE-7 | Binary is built with `ze_web ze_web_yang` only | YANG editor works; CLI and Workbench are not exposed as selectable modes |
| MODE-8 | Binary is built with `ze_web ze_web_workbench` but without a feature tag such as `ze_bgp` | The Workbench shell exists, but that feature's nav entries, pages, widgets, and tools are absent |

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze.web.ui-mode=finder` is set | The YANG/Finder UI still renders and can edit schema-backed config |
| AC-2 | Workbench is active and a path has no registered page | Existing generic YANG workbench detail renders |
| AC-3 | BGP registers its workbench contribution from a feature-owned package | BGP nav entries and BGP pages appear without editing `workbench_sections.go` or `workbench_pages.go` |
| AC-4 | A build without a feature tag excludes that feature's web registration package | The feature's nav entries, pages, widgets, and tools are absent |
| AC-4a | A feature-owned workbench file is added | Its build tag requires `ze_web && ze_web_workbench && ze_<feature>` unless the feature is always-on |
| AC-5 | Two contributions register the same ID | Registry validation reports a deterministic duplicate-ID error |
| AC-6 | Two page contributions claim the same exact path | Registry validation reports a deterministic route conflict unless one is explicitly lower-priority fallback |
| AC-7 | A registered page returns `handled=false` | Dispatch continues to the next match or generic YANG fallback |
| AC-8 | A read-only user opens a page with edit controls | Mutating controls are hidden or disabled; mutation routes still enforce 403 |
| AC-9 | A row tool is clicked | Browser posts only `tool_id` and `context_path`; the server resolves the command |
| AC-10 | A tool has a missing required placeholder | The tool is disabled or omitted according to its `empty` policy |
| AC-11 | A diagnostic command returns large output | Overlay caps/truncates according to existing safety limits and offers download/copy |
| AC-12 | Multiple tools are run from the same page | Multiple overlays/job entries can remain visible and independently close/rerun |
| AC-13 | Command palette searches pages | Registered nav/page entries appear with stable labels and URLs |
| AC-14 | Command palette runs a diagnostic | It uses registered IDs and the same dispatcher/authz path as related tools |
| AC-15 | A table has zero rows | Column headers and Add/Filter controls remain visible |
| AC-16 | A large route/firewall/policy table is opened | Data is capped or paginated server-side; the browser is not asked to render unbounded rows |
| AC-17 | A feature page is removed | Its registered nav/page/palette/widget entries disappear and unrelated pages still work |
| AC-18 | A plugin/feature registers a whole HTTP route | Existing `RegisterWebRoute` behavior continues to work |
| AC-19 | A workbench feature/page family is proposed for implementation | A dedicated spec exists and names its underlying module/API/YANG/build-tag dependencies before code changes start |

## TDD Test Plan

### Unit tests

| Test | File | Validates |
|------|------|-----------|
| `TestRegisterWorkbenchContributionCopiesInput` | `internal/component/web/workbench_registry_test.go` | registry snapshot cannot be mutated through caller slices |
| `TestWorkbenchRegistryRejectsDuplicateContributionID` | `internal/component/web/workbench_registry_test.go` | AC-5 |
| `TestWorkbenchRegistryRejectsDuplicatePageID` | `internal/component/web/workbench_registry_test.go` | page identity is stable |
| `TestWorkbenchRegistryRejectsConflictingExactPath` | `internal/component/web/workbench_registry_test.go` | AC-6 |
| `TestWorkbenchRegistrySortsNavDeterministically` | `internal/component/web/workbench_registry_test.go` | stable left nav ordering |
| `TestWorkbenchSectionsComeFromRegistry` | `internal/component/web/workbench_sections_test.go` | central taxonomy no longer owns feature entries |
| `TestWorkbenchDispatchFallsBackToYANG` | `internal/component/web/handler_workbench_test.go` | AC-2 |
| `TestWorkbenchDispatchUsesRegisteredPage` | `internal/component/web/handler_workbench_test.go` | registered page handles matching path |
| `TestBGPWorkbenchContributionRegistersNavAndPages` | BGP web registration test | AC-3 |
| `TestReadOnlyRegistryPageHidesMutations` | `internal/component/web/handler_workbench_test.go` | AC-8 |
| `TestWebModeSelectorNamesThreeModes` | `internal/component/web/ui_mode_test.go` or successor | MODE-1 through MODE-3 |
| `TestCLIModeIsFirstClassButServedAtCLI` | `internal/component/web/cli_test.go` | MODE-1 |
| `TestYANGEditorModeKeepsFinderCompatibilityToken` | `internal/component/web/ui_mode_test.go` | MODE-2, MODE-4 |
| `TestWebModeSelectorsGateUnavailableModes` | build-tag test, likely `cmd/ze/hub` | MODE-6, MODE-7, MODE-8 |
| `TestFeatureWorkbenchFilesRequireFeatureAndWorkbenchTags` | generated import/build-tag test | AC-4a |
| `TestCommandPaletteIndexesRegisteredPages` | `internal/component/web/palette_test.go` | AC-13 |
| `TestRelatedToolOverlayRerunUsesServerDescriptor` | `internal/component/web/handler_tools_test.go` | AC-9 and rerun safety |

### Golden / markup tests

| Test | File | Validates |
|------|------|-----------|
| workbench nav golden | `internal/component/web/testdata/golden/...` | BGP/feature nav rendered from registry |
| command palette golden | new golden fixture | palette groups pages/actions/diagnostics |
| job dock golden | new golden fixture | multiple tool outputs render as separate dock entries |

### Functional tests

| Test | Location | Scenario |
|------|----------|----------|
| `web-workbench-yang-fallback` | `test/web/*.ci` | open a schema path with no registered page and edit through generic YANG |
| `web-mode-cli` | `test/web/*.ci` | open CLI mode and run a read-only operational command through HTTPS |
| `web-mode-yang-editor` | `test/web/*.ci` | select YANG editor mode and verify the current Finder visual path still edits config |
| `web-mode-workbench` | `test/web/*.ci` | select Workbench mode and verify registry-backed nav/page rendering |
| `web-mode-selector-build-tags` | build-tag web test | selected mode tags determine which modes are selectable |
| `web-workbench-bgp-registered` | `test/web/*.ci` | BGP contribution appears and page renders when `ze_web && ze_web_workbench && ze_bgp` are present |
| `web-workbench-related-overlay` | existing/new web test | run BGP related detail tool and see overlay output |
| `web-workbench-feature-removal` | build-tag test | compile without feature and verify nav/page absent |

## Files to Modify

Likely implementation files:

- `internal/component/web/workbench_registry.go`
- `internal/component/web/workbench_registry_test.go`
- `internal/component/web/ui_mode.go`
- `internal/component/web/ui_mode_test.go`
- `internal/component/web/cli.go`
- `internal/component/web/cli_terminal.go`
- `internal/component/web/component_cli_terminal.templ`
- `internal/component/plugin/all/*` generated build-tag imports, including new `ze_web_workbench` conjunctions
- build-tag guard tests under `cmd/ze/hub` or the generator-owned package
- `internal/component/web/workbench_sections.go`
- `internal/component/web/workbench_sections_test.go`
- `internal/component/web/workbench_pages.go`
- `internal/component/web/handler_workbench.go`
- `internal/component/web/handler_workbench_test.go`
- `internal/component/web/component_workbench_nav.templ`
- `internal/component/web/component_workbench_topbar.templ`
- `internal/component/web/component_tool_overlay.templ`
- `internal/component/web/component_tool_overlay_templ.go`
- `internal/component/web/handler_tools.go`
- `internal/component/web/handler_tools_test.go`
- `internal/component/web/page_bgp_*.go` during migration
- `internal/component/bgp/web/register.go` or `internal/component/bgp/workbench_register.go`
- `internal/component/plugin/all/*` if a new build-tag-gated blank import is required
- `cmd/ze/hub/service_web.go` to pass a registry snapshot if request-time global reads are avoided
- `docs/architecture/web-interface.md`
- `docs/architecture/web-components.md`
- `docs/architecture/web-workbench-pages.md`

## Implementation Steps

0. **Spec slicing gate** - before implementing any non-foundation page
   family, create or update the dedicated slice spec and record module/API/YANG
   dependencies and readiness.
1. **Mode contract** - update naming, docs, top navigation, and tests so CLI,
   YANG editor, and Workbench are peer modes. Preserve existing `/cli` serving
   and `finder` compatibility while making the product semantics explicit.
2. **Mode build selectors** - introduce `ze_web_cli`, `ze_web_yang`, and
   `ze_web_workbench` as mode selectors layered on `ze_web`. Update generated
   import rules and tests so feature-owned workbench files require
   `ze_web && ze_web_workbench && ze_<feature>`.
3. **Registry skeleton** - add typed contribution structs, register/snapshot
   helpers, reset-for-test helpers, and validation for duplicate IDs, invalid
   paths, nil renderers, and exact path conflicts.
4. **Navigation migration** - teach `workbenchSections` to build from the
   registry snapshot, register existing central nav as built-in contributions,
   and keep current labels/URLs identical for the first migration.
5. **Page dispatch migration** - replace `renderPageContent` central switch with
   registry dispatch while preserving generic YANG fallback.
6. **BGP proof** - move BGP nav/page registration into a BGP-owned web
   registration package or BGP package file. BGP registers peers, groups,
   families, summary, and policy entries.
7. **Related overlay improvements** - add copy/download/rerun/search controls,
   stable overlay/job IDs, and no client-supplied command strings.
8. **Command palette** - build palette index from registry snapshot and safe
   diagnostics, add keyboard/topbar entry, and submit action IDs rather than raw
   commands.
9. **FortiOS-style table state** - add server-side table state parsing and apply
   to BGP peers first, then firewall/routing tables.
10. **Docs** - update architecture docs with the contribution model and module
   placement rules, and document that the YANG interface remains the canonical
   fallback.

## Failure Modes and Mitigations

| Risk | Early signal | Mitigation |
|------|--------------|------------|
| Registry becomes another monolith | A new feature requires editing central nav/page switches | AC-3 blocks closure; migrate at least BGP before declaring success |
| CLI remains treated as a side page | Top navigation says Finder/Workbench only and `/cli` is hidden behind a small link | MODE-1 blocks closure; CLI is a peer mode even if it remains served at `/cli` |
| `ze_web` accidentally pulls in every mode or every feature contribution | A `ze_web`-only build imports CLI, YANG, Workbench, BGP, L2TP, etc. workbench code | MODE-6 through MODE-8 and AC-4a require explicit mode selectors and feature-tag conjunctions |
| Feature implementation starts from vague design prose | A coding agent cannot name the module APIs, YANG nodes, live state source, or build tags it depends on | AC-19 blocks implementation until a feature/objective spec exists |
| Feature packages import too much web code | Dependency cycles or feature core importing templ-heavy packages | Allow separate `<feature>/web` or `<feature>-web` packages |
| UI diverges from YANG config truth | Hand-written forms accept fields not in schema | Forms must write through existing config handlers/editor manager |
| Operational commands bypass authz | A page handler calls feature internals directly for mutation/clear commands | Commands run through `CommandDispatcher`; direct reads only for exported read-only snapshots |
| Plugin-supplied markup causes XSS/CSP holes | Registry accepts raw HTML strings from external plugin data | In-tree renderers use templ; out-of-process plugins expose data/route surfaces, not arbitrary authenticated shell HTML |
| Large tables freeze browsers | Route/firewall tables render thousands of rows | Server-side caps/pagination required before migrating large operational tables |
| Finder rollback breaks | Workbench code changes generic fragments globally | AC-1 and AC-2 require YANG/Finder and generic fallback tests |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Workbench is alternative UI, not replacement | Delete/disable Finder once workbench starts | User explicitly wants to keep YANG; generic YANG remains essential for full coverage and recovery |
| Three web modes are product-level peers | Treat CLI as a small workbench link and Finder as only rollback plumbing | User explicitly asked to define CLI over HTTPS, current YANG edition, and the new interface as modes |
| Separate mode selectors under `ze_web` | Keep one `ze_web` tag for all web UI, or use only a generic `ze_web_ui` tag | `ze_web` already means web service; explicit `ze_web_cli`, `ze_web_yang`, and `ze_web_workbench` let builds include only the desired modes, while `ze_web_workbench` is the approved selector for the new interface |
| Typed registry rather than central switches | Keep `workbench_sections.go` and `workbench_pages.go` as permanent owners | The current code already documents this as a temporary v1 boundary; user requires components to register their own UI |
| Allow feature-local and `<feature>-web` ownership | Force all UI into central web package or all into separate modules | Some features need web-only dependencies/build tags; others only need a small registration file |
| Keep `ze:related` for contextual operations | Hardcode row actions in page builders | YANG schema knows the config context and server-side resolver already prevents raw browser commands |
| Use FortiOS command palette/job overlay ideas | Keep only modal command results | Palette and docked overlays directly support "make change, check result" with fewer page changes |
| Migrate BGP first | Migrate every page in one pass | BGP exercises config tables, operational state, related tools, and domain-specific nav without touching every feature at once |
| Require per-feature/objective specs | Let agents implement directly from the high-level design | The design defines quality; specs define exact module dependencies, files, tests, and fallback behavior needed for safe implementation |

## Known Limitations

- This spec does not define a full out-of-process plugin UI protocol. It defines
  the in-process typed registry and keeps whole-route plugin extension via
  `WebRoute`.
- First table-state work is query-param based. Per-user persistence can follow
  once the interaction is proven.
- The first job dock can use completed command output and polling. Streaming
  long-running jobs may need a later command/job API if the current dispatcher
  cannot represent job lifecycle cleanly.
- The command palette must start with registered pages/actions and safe
  diagnostics. Free-form CLI execution from the palette is out of scope.

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Original skeleton only specified "web route registry" and did not cover the operator-workflow innovation, YANG fallback, FortiOS ideas, or component-owned workbench pages | this spec before 2026-09-11 rewrite | Rewritten into a full design spec |
| 2 | ISSUE | Existing code already has `WebRoute`, `ze:related`, and workbench pages; the missing part is not route registration but workbench contribution registration | `internal/component/web/webroute.go`, `workbench_sections.go`, `workbench_pages.go` | Scope narrowed to typed workbench contributions |
| 3 | ISSUE | A central left nav and central page dispatcher would recreate the monolith the user rejected | `workbench_sections.go`, `workbench_pages.go` | AC-3 requires BGP migration without central edits |

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

This is a design/spec update. Implementation verification is intentionally not
complete yet.

### Files Exist

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/web/webroute.go` | yes | read during research |
| `internal/component/web/workbench_sections.go` | yes | read during research |
| `internal/component/web/workbench_pages.go` | yes | read during research |
| `internal/component/web/handler_tools.go` | yes | read during research |
| `internal/component/web/ui_mode.go` | yes | read during research |
| `internal/component/web/cli_terminal.go` | yes | read during research |
| `cmd/ze/hub/service_web.go` | yes | read during research |
| `internal/component/plugin/all/all_ze_web.go` | yes | read during research |
| `internal/component/plugin/all/all_ze_radius_ze_l2tp.go` | yes | read during research |
| `internal/component/web/page_l2tp.go` and `page_l2tp_off.go` | yes | read during research |
| `internal/component/config/related.go` | yes | read during research |

### AC Verified

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Finder exists as rollback | `ui_mode.go` supports `finder`, docs mention `ze.web.ui-mode=finder` |
| AC-9 | Related tools use IDs/context | `handler_tools.go` accepts `tool_id` and `context_path` and resolves server-side |
| MODE-1 | CLI over HTTPS already exists as a route but is not yet first-class in `UIMode` | `/cli` is wired in `cmd/ze/hub/service_web.go`; `uiModeTokenCLI` is only an `ActiveUI` marker today |
| MODE-6..8 / AC-4a | `ze_web` already exists and combined feature tags are normal | `register_web.go` and `service_web.go` use `ze_web`; examples include `ze_core && ze_bgp` and `ze_l2tp && ze_radius` |

## Checklist

### Goal Gates

- [ ] Mode selectors `ze_web_cli`, `ze_web_yang`, and `ze_web_workbench` implemented and tested
- [ ] Feature-owned workbench files require `ze_web && ze_web_workbench && ze_<feature>`
- [ ] Registry API implemented
- [ ] BGP moved to feature-owned workbench contribution
- [ ] Generic YANG fallback preserved
- [ ] Finder rollback preserved
- [ ] Related overlay upgraded with copy/download/rerun/search controls
- [ ] Command palette implemented with registered pages/actions
- [ ] Table state implemented for at least BGP peers
- [ ] Build-tag removal test proves feature UI disappears
- [ ] Docs updated
- [ ] Standard verification run passes
