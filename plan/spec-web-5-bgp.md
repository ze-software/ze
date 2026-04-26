# Spec: web-5 -- BGP Pages for the Operator Workbench

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-web-3-foundation |
| Phase | 9/9 |
| Updated | 2026-04-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/design/web-interface-design.md` - page layouts, columns, empty states, workflows
4. `docs/architecture/web-interface.md` - web UI architecture
5. `docs/architecture/web-components.md` - HTMX fragment patterns
6. `internal/component/web/handler_workbench.go` - workbench handler
7. `internal/component/web/workbench_enrich.go` - table enrichment pipeline
8. `internal/component/web/handler_tools.go` - related tool overlay execution
9. `internal/component/web/related_resolver.go` - placeholder substitution
10. `internal/component/bgp/schema/ze-bgp-conf.yang` - BGP YANG config model
11. `internal/component/bgp/plugins/cmd/peer/peer.go` - peer command RPCs
12. `internal/component/bgp/plugins/cmd/peer/summary.go` - summary and capabilities RPCs

## Task

Build the BGP pages of the V2 operator workbench: Peers, Groups, Summary, Families, and Filters/Policy. BGP is Ze's primary use case. These pages must deliver the complete operator loop: find a peer, inspect its live state, edit config, commit, verify the change took effect, all without leaving the workspace.

The spec depends on spec-web-3-foundation for reusable components (table pattern, detail panel, empty states, toolbar, form overlays, confirmation dialogs, color-coded status rows). This spec focuses on BGP-specific page handlers, data integration, operational command plumbing, and the peer change-and-verify loop.

The existing workbench infrastructure (spec-web-2) already provides: the V2 shell layout, left navigation sections, the related tool overlay endpoint (`/tools/related/run`), the `ze:related` YANG extensions on peer lists, placeholder resolution against the working tree, pending-change markers, and the `enrichWorkbenchTable` pipeline. This spec builds on all of that.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `plan/design/web-interface-design.md` - master page layout reference
  -> Decision: Tables are the primary view; every table row has actions; status is visible inline
  -> Constraint: Color coding: green=Established, red=Idle/error, grey=Disabled, yellow=Active/Connect
  -> Constraint: Empty state pattern: column headers + message + Add button in body + in toolbar
  -> Constraint: Detail panel has tabs (Config, Status, Actions); operational tools show as overlays
- [ ] `docs/architecture/web-interface.md` - web UI architecture
  -> Constraint: Server-rendered Go templates + HTMX; no client-side SPA framework
  -> Constraint: CSP posture must be preserved; no inline scripts
- [ ] `docs/architecture/web-components.md` - HTMX fragment and OOB swap patterns
  -> Decision: Fragment-based partial updates via HX-Request header detection
  -> Constraint: HTMX partial requests return OOB response; full-page requests render workbench shell
- [ ] `docs/architecture/api/commands.md` - command taxonomy and dispatch
  -> Decision: Operational commands go through CommandDispatcher with same authz as CLI
  -> Constraint: Commands use `peer <selector> <verb>` syntax; selector is IP, name, or AS
- [ ] `docs/architecture/config/yang-config-design.md` - YANG model and config resolution
  -> Decision: Config follows YANG schema; editor sessions produce pending diffs
  -> Constraint: Changes are draft until commit; pending-change markers must be visible

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - BGP-4 specification: session states, hold timer, message formats
  -> Constraint: FSM states: Idle, Connect, Active, OpenSent, OpenConfirm, Established
- [ ] `rfc/short/rfc4486.md` - BGP cease notification subcodes
  -> Constraint: Last error display must show cease subcode text
- [ ] `rfc/short/rfc8203.md` - Shutdown communication
  -> Constraint: Teardown action should support optional shutdown message

**Key insights:**
- BGP peer lists exist at two YANG paths: `bgp/group/*/peer` (grouped) and `bgp/peer` (standalone). Both have `ze:related` tool annotations already wired by spec-web-2.
- Operational commands (detail, capabilities, statistics, flush, teardown) are registered as plugin RPCs via `pluginserver.RegisterRPCs` and dispatched through the command tree.
- The `enrichWorkbenchTable` pipeline already attaches row tools and pending-change markers to list table rows. The BGP pages need page-level handlers that route to the correct YANG paths and add BGP-specific presentation (status columns, color coding, state sorting).
- The `show bgp-health` command maps to wire method `ze-show:bgp-health`; `peer * detail` maps to `ze-bgp:peer-detail`.
- The summary RPC (`ze-bgp:summary`) accepts an optional AFI/SAFI argument and returns per-peer rows with state, uptime, prefixes, messages.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/component/web/handler_workbench.go` - serves /show/* in workbench mode; delegates to fragment data builder + enrichment
  -> Constraint: Full-page renders the workbench shell; HTMX requests return OOB fragments
  -> Constraint: Reuses `buildFragmentData` from Finder; only chrome differs
- [ ] `internal/component/web/workbench_enrich.go` - attaches row tools, table tools, pending-change markers to FragmentData.ListTable
  -> Constraint: Only enriches list views (path must end at a list node, not an entry)
  -> Constraint: `splitRelatedByPlacement` partitions tools into row/table subsets
- [ ] `internal/component/web/workbench_sections.go` - left navigation sections; "Routing" maps to `/show/bgp/`
  -> Constraint: Selection is driven by leading YANG path segment; `bgp` selects "Routing"
- [ ] `internal/component/web/handler_tools.go` - POST `/tools/related/run`; resolves tool id + context path, dispatches command, renders overlay
  -> Constraint: Output is ANSI-stripped, capped at 4 MiB, HTML-escaped
  -> Constraint: ToolOverlayData supports Result, Error, and Confirm states
- [ ] `internal/component/web/related_resolver.go` - substitutes `${path:...}` and `${path-inherit:...}` placeholders
  -> Constraint: Browser never sees raw commands; only tool id + context path are posted
- [ ] `internal/component/web/templates/component/list_table.html` - current list table template with row tools, pending markers, inline editable cells
  -> Constraint: Each row has rename button, link, inline inputs, row tool buttons, delete button
- [ ] `internal/component/web/templates/component/tool_overlay.html` - overlay fragment for command output
- [ ] `internal/component/web/fragment.go` - FragmentData model, FieldMeta, ListTableView
  -> Constraint: ListTableView has Rows, Columns, TableTools, FormURL fields
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - full BGP YANG model
  -> Constraint: `bgp/peer` list key is `name`; unique on `connection/remote/ip`
  -> Constraint: `bgp/group` list key is `name`; contains nested `peer` list
  -> Constraint: `ze:related` tools already defined on both standalone and grouped peer lists
  -> Constraint: Family list is per-peer at `session/family`; key is `name` (AFI/SAFI)
  -> Constraint: Filter chains are at `filter/import` and `filter/export` (leaf-lists)
  -> Constraint: Policy definitions are under `bgp/policy` (augmented by filter plugins)
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - RPCs: peer-list, peer-detail, peer-teardown, peer-pause, peer-resume, peer-flush, peer-history
  -> Constraint: Selector matching: IP address first, then peer name, then AS prefix ("asN")
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - RPCs: summary, peer-capabilities, peer-statistics
  -> Constraint: Summary accepts optional AFI/SAFI argument for family-scoped view
- [ ] `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` - command tree for peer operations
  -> Constraint: Commands registered under `peer/detail`, `peer/capabilities`, `peer/statistics`, `peer/teardown`, `peer/flush`

**Behavior to preserve:**
- The existing `enrichWorkbenchTable` pipeline must continue to work for all lists, not just BGP
- The Finder UI behavior must not change (V2 enrichment is workbench-only)
- The related tool overlay endpoint and resolution must remain generic (not BGP-specific)
- The `ze:related` annotations in the YANG schema are the single source of truth for which tools appear on which rows
- Editor session draft/commit/discard flow unchanged
- CSP headers, authentication middleware, and authz checks unchanged
- The `buildFragmentData` path continues to serve both Finder and workbench

**Behavior to change:**
- New BGP-specific page handlers that add status columns, color coding, and state-aware sorting
- New page handlers for Groups, Summary, Families, and Filters views
- New sub-navigation within the "Routing" section for BGP sub-pages
- Peer detail panel with tabbed Config/Status/Actions layout (extends existing detail fragment)
- BGP Health toolbar button as a table-level tool (already annotated in YANG as `bgp-health` with placement=global)
- Empty state messages specific to each BGP page

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Browser navigates to `/show/bgp/peer/` (peers table), `/show/bgp/group/` (groups), `/show/bgp/` (summary), `/show/bgp/*/peer/*/session/family/` (families), or `/show/bgp/policy/` (filters)
- Format at entry: HTTP GET with optional HX-Request header for HTMX partial

### Transformation Path
1. **Route dispatch** in `handler_workbench.go` matches `/show/*` prefix. The path is extracted and validated.
2. **BGP page detection**: new BGP page handlers check whether the path maps to a known BGP page (peers, groups, summary, families, filters). If so, they build page-specific data instead of the generic fragment.
3. **Schema + tree walk**: `walkSchema` and `walkTree` resolve the YANG path to a schema node and tree node. For list views, `lookupListNode` identifies it as a list.
4. **Fragment data build**: `buildFragmentData` produces the base FragmentData with fields, breadcrumbs, CLI context.
5. **BGP enrichment**: new BGP-specific enrichment adds status columns from operational data (peer state, uptime, prefixes, messages), color-codes rows by state, and sorts by state severity.
6. **Workbench enrichment**: existing `enrichWorkbenchTable` attaches row tools, table tools, and pending-change markers.
7. **Template render**: BGP page template renders the table with status columns, or the detail panel with tabs (Config/Status/Actions).
8. **HTMX response**: partial requests return OOB fragment; full-page requests render within workbench shell.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser -> Web handler | HTTP GET/POST with path + optional HX-Request header | [ ] |
| Web handler -> Config schema/tree | `walkSchema`, `walkTree`, `buildFragmentData` | [ ] |
| Web handler -> Plugin commands | `CommandDispatcher` via `HandleRelatedToolRun` for operational data | [ ] |
| Plugin command -> BGP reactor | `ctx.Reactor().Peers()` returns `[]plugin.PeerInfo` | [ ] |
| Web handler -> Editor session | `mgr.Tree(username)` for pending changes; `mgr.PendingChangePaths(username)` | [ ] |
| Template -> Browser | Server-rendered HTML with HTMX attributes for partial updates | [ ] |

### Integration Points
- `enrichWorkbenchTable` (existing) - extends with BGP-specific column injection and state coloring
- `HandleRelatedToolRun` (existing) - tool overlay execution for Detail, Capabilities, Statistics, Flush, Teardown
- `RelatedResolver` (existing) - resolves `${path:connection/remote/ip|key}` placeholders for peer tools
- `WorkbenchSections` (existing) - "Routing" section with sub-page links for BGP Peers, Groups, Summary, Families
- `buildFragmentData` (existing) - base data builder for config fields and list entries
- `CommandDispatcher` (existing) - dispatches `peer <selector> <verb>` and `show bgp-health`
- `EditorManager` (existing) - per-user draft sessions, pending-change paths, commit flow

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| GET `/show/bgp/peer/` | -> | `HandleBGPPeers` handler renders peer table with status columns | `TestBGPPeersPageRendersTable` |
| GET `/show/bgp/peer/` (HX-Request) | -> | HTMX partial returns peer table fragment with row tools | `TestBGPPeersHTMXPartial` |
| GET `/show/bgp/group/` | -> | `HandleBGPGroups` handler renders groups table | `TestBGPGroupsPageRendersTable` |
| GET `/show/bgp/` (summary route) | -> | `HandleBGPSummary` renders operational summary | `TestBGPSummaryPageRenders` |
| POST `/tools/related/run` with tool_id=peer-detail | -> | `HandleRelatedToolRun` dispatches `peer <ip> detail` | `test/web/workbench-bgp-peer-tools.wb` |
| POST `/tools/related/run` with tool_id=peer-teardown | -> | confirmation overlay shown, then dispatch on confirm | `test/web/workbench-bgp-peer-tools.wb` |
| GET `/show/bgp/peer/` after edit | -> | row shows pending-change marker | `test/web/workbench-bgp-pending-diff.wb` |
| Edit -> commit -> rerun peer detail | -> | change-and-verify loop completes without full-page nav | `test/web/workbench-bgp-change-verify.wb` |
| GET `/show/bgp/peer/` with no peers | -> | empty state message with Add Peer button | `TestBGPPeersEmptyState` |
| GET `/show/bgp/policy/` | -> | filters page renders filter chains | `TestBGPFiltersPageRendersTable` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Navigate to `/show/bgp/peer/` with configured peers | Table shows all peers with columns: Flags, Name, Remote IP, Remote AS, Local AS, Group, State, Uptime, Prefixes, TX/RX Messages, Last Error |
| AC-2 | Peer in Established state | Row has green status indicator (CSS class `row--state-established`) |
| AC-2b | Peer in Idle state or with error | Row has red status indicator (CSS class `row--state-idle` or `row--state-error`) |
| AC-2c | Peer disabled | Row has grey status indicator (CSS class `row--state-disabled`) |
| AC-2d | Peer in Active/Connect state | Row has yellow status indicator (CSS class `row--state-active`) |
| AC-3 | Click "Add Peer" button on peers page | Add Peer form overlay opens with required fields: name, remote IP, remote AS, local AS, address family |
| AC-4 | Click a peer row to open detail panel | Detail panel opens with three tabs: Config, Status, Actions |
| AC-5 | Edit a field in the Config tab | Field shows pending-change marker; row in table also shows pending marker |
| AC-6 | Open Status tab on peer detail | Tab shows live session state, uptime, negotiated capabilities, prefix counts, message counters, hold time, last notification, from operational command output |
| AC-7 | Click "Peer Detail" row tool button | Tool overlay opens with `peer <ip> detail` output; "Capabilities" and "Statistics" similarly open their respective overlays |
| AC-8 | Click "Flush" row tool button | Confirmation dialog appears; on confirm, soft reset executes via `peer <ip> flush` |
| AC-9 | Click "Teardown" row tool button | Danger-styled confirmation dialog appears; on confirm, hard close executes via `peer <ip> teardown` |
| AC-10 | Navigate to `/show/bgp/group/` | Table shows groups with columns: Name, Peer Count, Remote AS, Families, Description; row action "View Peers" filters peers table by group |
| AC-11 | Navigate to BGP summary page | Read-only table from `show bgp-health` output, sorted by state severity (problems first): Peer, Remote AS, State, Uptime, Prefixes, Messages In/Out, Last Error |
| AC-12 | Navigate to families view | Table shows address family config across peers/groups: Family, Mode, Max Prefixes, Warning Threshold, Teardown on Limit, Default Originate |
| AC-13 | Navigate to `/show/bgp/policy/` | Table shows filter chains: Name, Type, Used By, Rule Count; detail shows ordered rules with match conditions and actions |
| AC-14 | Edit peer field, run peer detail, commit, rerun peer detail | Complete loop works: pending marker appears after edit, tool output shows pre-commit state, commit clears marker, re-run tool shows new state. Zero full-page navigations. |
| AC-15 | Navigate to any BGP page with no data | Empty state shows page-specific message with Add button and contextual hint |
| AC-16 | Click "BGP Health" in peers page toolbar | Modal opens with `show bgp-health` command output |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBGPPeersPageRendersTable` | `internal/component/web/handler_bgp_test.go` | Peers table renders with all expected columns and row data | |
| `TestBGPPeersHTMXPartial` | `internal/component/web/handler_bgp_test.go` | HTMX request returns fragment, not full page | |
| `TestBGPPeersEmptyState` | `internal/component/web/handler_bgp_test.go` | Empty peers list renders empty state message with Add button | |
| `TestBGPPeersStateColorCoding` | `internal/component/web/handler_bgp_test.go` | Rows have correct CSS class per peer state (established/idle/active/disabled) | |
| `TestBGPPeersStateSorting` | `internal/component/web/handler_bgp_test.go` | Summary-mode sorting puts problems first: idle/error, then active/connect, then established | |
| `TestBGPGroupsPageRendersTable` | `internal/component/web/handler_bgp_test.go` | Groups table renders with Name, Peer Count, Remote AS, Families, Description | |
| `TestBGPGroupsEmptyState` | `internal/component/web/handler_bgp_test.go` | Empty groups renders "No peer groups configured" message | |
| `TestBGPGroupsViewPeersLink` | `internal/component/web/handler_bgp_test.go` | "View Peers" action links to peers page filtered by group name | |
| `TestBGPSummaryPageRenders` | `internal/component/web/handler_bgp_test.go` | Summary page renders operational data sorted by state severity | |
| `TestBGPSummaryAutoRefresh` | `internal/component/web/handler_bgp_test.go` | Summary page includes HX auto-refresh trigger | |
| `TestBGPFamiliesPageRenders` | `internal/component/web/handler_bgp_test.go` | Families table shows family config across peers | |
| `TestBGPFiltersPageRendersTable` | `internal/component/web/handler_bgp_test.go` | Filters page shows filter chains with rule count and used-by | |
| `TestBGPFiltersEmptyState` | `internal/component/web/handler_bgp_test.go` | Empty filters renders appropriate message | |
| `TestBGPPeerDetailTabs` | `internal/component/web/handler_bgp_test.go` | Peer detail renders Config/Status/Actions tabs | |
| `TestBGPPeerDetailConfigEditable` | `internal/component/web/handler_bgp_test.go` | Config tab fields are editable with HX-POST targets | |
| `TestBGPPeerDetailStatusReadOnly` | `internal/component/web/handler_bgp_test.go` | Status tab renders read-only operational data | |
| `TestBGPPeerDetailActionsTab` | `internal/component/web/handler_bgp_test.go` | Actions tab shows Flush, Teardown, Route Refresh buttons | |
| `TestBGPHealthToolbarButton` | `internal/component/web/handler_bgp_test.go` | BGP Health button appears in toolbar and dispatches `show bgp-health` | |
| `TestBGPSubNavigation` | `internal/component/web/handler_bgp_test.go` | Routing section in workbench nav shows sub-pages: Peers, Groups, Summary, Families | |
| `TestBGPPeersRowToolButtons` | `internal/component/web/handler_bgp_test.go` | Each peer row has Detail, Capabilities, Statistics, Flush, Teardown tool buttons | |
| `TestBGPPeersPendingChangeMarker` | `internal/component/web/handler_bgp_test.go` | Row with pending edit shows `row--pending` class and pending marker | |
| `TestBGPGroupDeleteBlockedWithPeers` | `internal/component/web/handler_bgp_test.go` | Delete action on group with peers returns blocking error | |
| `TestBGPPeersStatusColumnFromOperational` | `internal/component/web/handler_bgp_test.go` | State, Uptime, Prefixes columns come from operational data, not config | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Hold time (receive) | 0 or 3..65535 | 65535 | 1 (below 3, not 0) | 65536 |
| Hold time (send) | 0 or 480..65535 | 65535 | 1 (below 480, not 0) | 65536 |
| Connect retry | 0..65535 | 65535 | N/A | 65536 |
| Prefix maximum | 1..4294967295 | 4294967295 | 0 | N/A (uint32 max) |
| TTL max | 0..255 | 255 | N/A | 256 |
| Max paths (multipath) | 1..256 | 256 | 0 | 257 |
| Admin distance (eBGP/iBGP) | 1..255 | 255 | 0 | 256 |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `workbench-bgp-peer-table` | `test/web/workbench-bgp-peer-table.wb` | Navigate to BGP peers page, verify table renders with status columns, click row to see detail | |
| `workbench-bgp-peer-tools` | `test/web/workbench-bgp-peer-tools.wb` | Click Peer Detail, Capabilities, Statistics buttons; verify overlay renders; verify Flush shows confirm | |
| `workbench-bgp-change-verify` | `test/web/workbench-bgp-change-verify.wb` | Edit peer field, see pending marker, run peer detail, commit, rerun peer detail, verify new state | |
| `workbench-bgp-pending-diff` | `test/web/workbench-bgp-pending-diff.wb` | Edit peer, check pending marker visible, open diff, discard to clear | |
| `workbench-bgp-groups` | `test/web/workbench-bgp-groups.wb` | Navigate to groups, verify table, click View Peers, verify peer filter | |
| `workbench-bgp-summary` | `test/web/workbench-bgp-summary.wb` | Navigate to summary, verify read-only table sorted by state | |
| `workbench-bgp-empty-states` | `test/web/workbench-bgp-empty-states.wb` | Navigate to peers/groups/filters with no config, verify empty state messages | |
| `workbench-bgp-add-peer` | `test/web/workbench-bgp-add-peer.wb` | Click Add Peer, fill form, submit, verify peer appears in table | |
| `workbench-bgp-health-modal` | `test/web/workbench-bgp-health-modal.wb` | Click BGP Health button, verify modal with health output | |

### Future (if deferring any tests)
- FSM transition history display in Status tab (requires reactor event log, not yet exposed)
- Route Refresh action button (depends on capability negotiation state detection in web layer)
- Drag-to-reorder for filter rules (complex JS interaction, may need client-side library)

## Files to Modify
- `internal/component/web/handler_workbench.go` - add BGP page routing to detect `/show/bgp/peer/`, `/show/bgp/group/`, `/show/bgp/`, `/show/bgp/policy/` and delegate to BGP handlers
- `internal/component/web/workbench_sections.go` - add sub-navigation entries under "Routing" for Peers, Groups, Summary, Families
- `internal/component/web/workbench_enrich.go` - extend enrichment to inject operational status columns for BGP peer rows
- `internal/component/web/fragment.go` - extend FragmentData/ListTableView with status column types and state color classes
- `internal/component/web/render.go` - register new BGP templates
- `internal/component/web/handler_tools.go` - no changes expected (tool overlay endpoint is generic)
- `internal/component/web/related_resolver.go` - no changes expected (resolution is generic)
- `internal/component/web/templates/component/list_table.html` - extend table template with status column rendering and state CSS classes
- `internal/component/web/assets/` - add CSS for state color coding (established=green, idle=red, disabled=grey, active=yellow)
- `internal/component/bgp/schema/ze-bgp-conf.yang` - add `ze:related` for `show bgp-health` as table-level tool on peer list (if not already present as global tool)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No - existing RPCs sufficient | - |
| CLI commands/flags | [ ] No - existing commands sufficient | - |
| Editor autocomplete | [ ] No - YANG-driven (automatic) | - |
| Functional test for new RPC/API | [ ] No - new pages use existing RPCs | - |
| Web handler route registration | [x] Yes | `internal/component/web/handler_workbench.go` or `server.go` |
| Web template registration | [x] Yes | `internal/component/web/render.go` |
| CSS additions | [x] Yes | `internal/component/web/assets/` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] Yes | `docs/features.md` - add BGP pages section |
| 2 | Config syntax changed? | [ ] No | - |
| 3 | CLI command added/changed? | [ ] No | - |
| 4 | API/RPC added/changed? | [ ] No | - |
| 5 | Plugin added/changed? | [ ] No | - |
| 6 | Has a user guide page? | [x] Yes | `docs/guide/web-ui.md` - add BGP pages usage |
| 7 | Wire format changed? | [ ] No | - |
| 8 | Plugin SDK/protocol changed? | [ ] No | - |
| 9 | RFC behavior implemented? | [ ] No | - |
| 10 | Test infrastructure changed? | [ ] No | - |
| 11 | Affects daemon comparison? | [ ] No | - |
| 12 | Internal architecture changed? | [x] Yes | `docs/architecture/web-interface.md` - document BGP page handler pattern |

## Files to Create
- `internal/component/web/handler_bgp.go` - BGP page handlers (peers, groups, summary, families, filters)
- `internal/component/web/handler_bgp_test.go` - unit tests for all BGP page handlers
- `internal/component/web/templates/component/bgp_peers.html` - peers table template with status columns and color coding
- `internal/component/web/templates/component/bgp_groups.html` - groups table template
- `internal/component/web/templates/component/bgp_summary.html` - read-only summary table template
- `internal/component/web/templates/component/bgp_families.html` - families table template
- `internal/component/web/templates/component/bgp_filters.html` - filters table template
- `internal/component/web/templates/component/bgp_peer_detail.html` - peer detail panel with Config/Status/Actions tabs
- `internal/component/web/templates/component/bgp_empty.html` - BGP-specific empty state messages
- `test/web/workbench-bgp-peer-table.wb` - functional test for peer table rendering
- `test/web/workbench-bgp-groups.wb` - functional test for groups page
- `test/web/workbench-bgp-summary.wb` - functional test for summary page
- `test/web/workbench-bgp-empty-states.wb` - functional test for empty states
- `test/web/workbench-bgp-add-peer.wb` - functional test for add peer flow
- `test/web/workbench-bgp-health-modal.wb` - functional test for BGP health modal

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. /ze-review gate | Review Gate section -- run `/ze-review`; fix every BLOCKER/ISSUE; re-run until only NOTEs remain (BEFORE full verification) |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: BGP peers table page** -- core handler, columns, status integration, CSS, template
   - Tests: `TestBGPPeersPageRendersTable`, `TestBGPPeersHTMXPartial`, `TestBGPPeersEmptyState`, `TestBGPPeersStateColorCoding`, `TestBGPSubNavigation`
   - Files: `handler_bgp.go`, `handler_bgp_test.go`, `bgp_peers.html`, `bgp_empty.html`, `workbench_sections.go`, CSS
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Add Peer form and creation flow** -- form overlay, required field validation, peer creation via config set
   - Tests: `TestBGPPeersAddPeerForm` (in handler_bgp_test.go)
   - Files: `handler_bgp.go`, `bgp_peers.html`, add form template
   - Functional: `test/web/workbench-bgp-add-peer.wb`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Peer detail panel with tabs** -- Config/Status/Actions tab layout, status data integration
   - Tests: `TestBGPPeerDetailTabs`, `TestBGPPeerDetailConfigEditable`, `TestBGPPeerDetailStatusReadOnly`, `TestBGPPeerDetailActionsTab`
   - Files: `handler_bgp.go`, `bgp_peer_detail.html`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Row-level tool integration** -- Detail, Capabilities, Statistics overlays; Flush and Teardown with confirmation
   - Tests: `TestBGPPeersRowToolButtons`, `TestBGPPeersStatusColumnFromOperational`
   - Files: `handler_bgp.go` (operational data fetch), `workbench_enrich.go` (status columns)
   - Functional: `test/web/workbench-bgp-peer-tools.wb`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Change-and-verify loop** -- pending markers, commit from table, tool rerun after commit
   - Tests: `TestBGPPeersPendingChangeMarker`
   - Files: `handler_bgp.go`, `bgp_peers.html`, `workbench_enrich.go`
   - Functional: `test/web/workbench-bgp-change-verify.wb`, `test/web/workbench-bgp-pending-diff.wb`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Groups page** -- groups table, peer count, member management, View Peers filter link
   - Tests: `TestBGPGroupsPageRendersTable`, `TestBGPGroupsEmptyState`, `TestBGPGroupsViewPeersLink`, `TestBGPGroupDeleteBlockedWithPeers`
   - Files: `handler_bgp.go`, `bgp_groups.html`
   - Functional: `test/web/workbench-bgp-groups.wb`
   - Verify: tests fail -> implement -> tests pass

7. **Phase: Summary page and Families page** -- read-only operational summary, cross-peer family view
   - Tests: `TestBGPSummaryPageRenders`, `TestBGPSummaryAutoRefresh`, `TestBGPFamiliesPageRenders`, `TestBGPPeersStateSorting`
   - Files: `handler_bgp.go`, `bgp_summary.html`, `bgp_families.html`
   - Functional: `test/web/workbench-bgp-summary.wb`
   - Verify: tests fail -> implement -> tests pass

8. **Phase: Filters/Policy page** -- filter chain table, rule list, Used By resolution
   - Tests: `TestBGPFiltersPageRendersTable`, `TestBGPFiltersEmptyState`
   - Files: `handler_bgp.go`, `bgp_filters.html`
   - Functional: (covered by unit tests; filter YANG model is plugin-augmented so functional test scope depends on loaded plugins)
   - Verify: tests fail -> implement -> tests pass

9. **Phase: Integration and full verification** -- BGP Health modal, all empty states, cross-page links, end-to-end
   - Tests: `TestBGPHealthToolbarButton`
   - Functional: `test/web/workbench-bgp-health-modal.wb`, `test/web/workbench-bgp-empty-states.wb`, `test/web/workbench-bgp-peer-table.wb`
   - Verify: all tests pass, `make ze-verify`

10. **Functional tests** -> Create after feature works. Cover user-visible behavior.
11. **RFC refs** -> Add `// RFC NNNN Section X.Y` comments (protocol work only)
12. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)
13. **Complete spec** -> Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; all 16 ACs plus sub-criteria demonstrated |
| Correctness | State color coding matches design doc exactly (green=Established, red=Idle, grey=Disabled, yellow=Active); operational data comes from runtime, not config |
| Naming | CSS classes use `row--state-{state}` convention; handler functions use `HandleBGP*` prefix; templates use `bgp_*.html` naming |
| Data flow | Operational status (state, uptime, prefixes) comes from plugin RPCs, not YANG config tree; config data comes from tree |
| Rule: no-layering | No duplicate table rendering code between BGP pages and generic list_table; BGP pages extend, not replace |
| Rule: derive-not-hardcode | Peer states derived from reactor/plugin data, not hardcoded state list; family names from YANG type, not hardcoded enum |
| Rule: exact-or-reject | Add Peer form validates required fields (remote IP, remote AS, family) before submission |
| Template reuse | BGP templates compose from shared components (empty state, toolbar, table chrome) defined in spec-web-3-foundation |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| BGP peers page handler | `grep -r "HandleBGP" internal/component/web/handler_bgp.go` |
| Peers table template | `ls internal/component/web/templates/component/bgp_peers.html` |
| Groups table template | `ls internal/component/web/templates/component/bgp_groups.html` |
| Summary table template | `ls internal/component/web/templates/component/bgp_summary.html` |
| Families table template | `ls internal/component/web/templates/component/bgp_families.html` |
| Filters table template | `ls internal/component/web/templates/component/bgp_filters.html` |
| Peer detail template | `ls internal/component/web/templates/component/bgp_peer_detail.html` |
| State color CSS | `grep -r "row--state-established" internal/component/web/assets/` |
| Unit tests | `go test -run TestBGP -count=1 ./internal/component/web/` |
| Functional tests | `ls test/web/workbench-bgp-*.wb` (at least 9 files) |
| Sub-navigation | `grep -r "Peers\|Groups\|Summary\|Families" internal/component/web/workbench_sections.go` |
| Route registration | `grep -r "bgp/peer\|bgp/group" internal/component/web/handler_workbench.go internal/component/web/server.go` |
| Documentation | `grep -r "BGP" docs/features.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Path segments validated by `ValidatePathSegments`; peer name validated by YANG pattern `[a-zA-Z_][a-zA-Z0-9_.\-]*`; AS number validated as uint32 |
| Command injection | Tool commands are resolved server-side from YANG descriptors; browser sends only tool_id + context_path, never raw commands |
| Sensitive data | MD5 password field (`ze:sensitive`) must not appear in table columns or tool output; masked in Config tab |
| Authorization | All BGP page handlers sit behind the same auth middleware as existing workbench; tool dispatch goes through `CommandDispatcher` with authz |
| XSS prevention | Tool overlay output is HTML-escaped; state column values are template-rendered, not raw HTML injection |
| Denial of service | Operational data fetch (summary, detail) has timeout; tool overlay output capped at 4 MiB |
| CSRF | Tool execution uses POST with HTMX; relies on existing CSRF protection |
| Error leakage | Dispatcher errors render in overlay error state, not as raw stack traces; invalid paths redirect to error, not 500 |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| Operational data not available (no reactor) | Render table with config columns only; status columns show "N/A" or "--" |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem -> arch doc, process -> rules, knowledge -> memory.md -->

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

Relevant RFCs for BGP pages:
- RFC 4271 Section 8.2.2: FSM state names used in state column
- RFC 4486: Cease notification subcodes for Last Error column
- RFC 8203: Shutdown Communication message in Teardown action
- RFC 5082: TTL Security / GTSM displayed in peer detail
- RFC 4456 Section 7: Route reflector fields in peer detail Config tab

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

<!-- BLOCKING (rules/planning.md Completion Checklist step 7): -->
<!-- Run /ze-review BEFORE the final testing/verify step. Record the findings here. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns only NOTEs (or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block -- record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->
<!-- Hook pre-commit-spec-audit.sh (exit 2) checks this section exists and is filled. -->

### Files Exist (ls)
<!-- For EVERY file in "Files to Create": ls -la <path> -- paste output. -->
<!-- For EVERY .ci file in Wiring Test and Functional Tests: ls -la <path> -- paste output. -->
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
<!-- For EVERY AC-N: independently verify. Do NOT copy from audit -- re-check. -->
<!-- Acceptable evidence: test name + pass output, grep showing function call, ls showing file. -->
<!-- NOT acceptable: "already checked", "should work", reference to audit table above. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
<!-- For EVERY wiring test row: does the .ci test exist AND does it exercise the full path? -->
<!-- Read the .ci file content. Does it actually test what the wiring table claims? -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
