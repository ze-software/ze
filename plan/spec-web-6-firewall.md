# Spec: web-6 -- Firewall Pages for the Operator Workbench

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-web-3-foundation |
| Phase | 1/8 |
| Updated | 2026-04-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/design/web-interface-design.md` - page layouts (Firewall section)
4. `docs/architecture/web-interface.md` - web UI architecture
5. `docs/architecture/web-components.md` - web component patterns
6. `ai/patterns/web-endpoint.md` - handler, template, HTMX, route conventions
7. `internal/component/firewall/model.go` - Table, Chain, Term, Set types
8. `internal/component/firewall/accessor.go` - LastApplied(), GetBackend(), GetCounters()
9. `internal/component/firewall/schema/ze-firewall-conf.yang` - YANG model
10. `internal/component/web/handler_l2tp.go` - reference handler pattern
11. `internal/component/web/handler_workbench.go` - workbench handler
12. `internal/component/web/workbench_sections.go` - left nav (firewall section exists)

## Task

Build the Firewall pages of the operator workbench: Tables, Chains, Rules, Sets, and Connections. Each page is a purpose-built operator view (not a YANG tree browser) that renders nftables configuration and operational data as actionable tables following the design in `plan/design/web-interface-design.md`.

The pages allow operators to view and manage firewall tables, chains, ordered rules, and named sets through the web UI, and to inspect live conntrack entries. The implementation reuses the spec-web-3-foundation reusable components (table pattern, empty state pattern, toolbar, row actions, form pattern) and the existing firewall data model in `internal/component/firewall/`.

## Required Reading

### Architecture Docs
- [ ] `plan/design/web-interface-design.md` - Firewall section defines Tables, Chains, Rules, Sets, Connections page layouts
  -> Decision: Every page follows the table pattern with toolbar, row actions, and empty states
  -> Constraint: Tables are the primary view, not YANG tree browsers
- [ ] `docs/architecture/web-interface.md` - Web UI architecture, handler patterns, template rendering
  -> Constraint: Server-rendered Go templates + HTMX, no client-side frameworks
- [ ] `docs/architecture/web-components.md` - Reusable component patterns
  -> Decision: V2 workbench shell with left nav, breadcrumbs, workspace area
- [ ] `ai/patterns/web-endpoint.md` - Handler, template, HTMX, and route conventions
  -> Constraint: Routes registered in cmd/ze/hub/main.go, handlers in internal/component/web/
- [ ] `internal/component/firewall/schema/ze-firewall-conf.yang` - YANG model for firewall config
  -> Decision: Tables have family enum; chains have type/hook/priority/policy; terms are ordered-by user with from/then blocks; sets have typed elements with flags
  -> Constraint: Table names bare in config, kernel carries ze_ prefix. Chain priority range -400..400. Term names are the key (not numeric order).

### RFC Summaries (MUST for protocol work)
_Not applicable. This is a web UI spec, not protocol work._

**Key insights:**
- Firewall data model is in `internal/component/firewall/model.go`: Table, Chain, Term, Set, Flowtable structs with typed enums (TableFamily, ChainType, ChainHook, Policy, SetType, SetFlags)
- Applied state accessible via `firewall.LastApplied()` (atomic snapshot, immutable)
- Counter readback via `firewall.GetBackend().GetCounters(tableName)` returns ChainCounters with per-term packets/bytes
- L2TP handler (`handler_l2tp.go`) is the reference pattern: struct with Renderer+Dispatch, JSON content negotiation, template rendering
- The workbench left nav already has a "Firewall" section at key="firewall" pointing to `/show/firewall/`
- Terms use named keys (not numeric indices), but `ordered-by user` in YANG means the config order matters
- Conntrack data is operational (not in config tree); must come from a show command or telemetry

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` - Table, Chain, Term, Set, Flowtable types; TableFamily, ChainType, ChainHook, Policy, SetType, SetFlags enums; Match/Action interfaces with 15+ match types and 16+ action types
- [ ] `internal/component/firewall/accessor.go` - LastApplied() returns atomic snapshot of applied tables; StripZeTablePrefix() converts kernel names to bare names; ActiveBackendName() for backend detection
- [ ] `internal/component/firewall/backend.go` - Backend interface: Apply, ListTables, GetCounters, Close; backend registry with RegisterBackend/LoadBackend/GetBackend
- [ ] `internal/component/firewall/config.go` - Config-to-model parsing (resolves YANG tree to firewall.Table slices)
- [ ] `internal/component/firewall/engine.go` - Firewall engine: OnConfigure applies desired state
- [ ] `internal/component/cmd/show/firewall.go` - show firewall ruleset/group RPCs; joins applied state with kernel counters
- [ ] `internal/component/web/handler_l2tp.go` - Reference handler pattern: L2TPHandlers struct, HandleL2TPList/Detail, JSON content negotiation, template rendering
- [ ] `internal/component/web/handler_workbench.go` - Workbench handler serving /show/* in V2 mode
- [ ] `internal/component/web/workbench_sections.go` - Left nav sections; firewall section already registered at key="firewall", URL="/show/firewall/"
- [ ] `internal/component/web/render.go` - Renderer with RenderWorkbench, RenderL2TPTemplate; WorkbenchData embeds LayoutData
- [ ] `internal/component/web/server.go` - Route registration via HandleFunc; handlers registered in cmd/ze/hub/main.go
- [ ] `internal/component/firewall/schema/ze-firewall-conf.yang` - Full YANG schema defining table/chain/term/set/flowtable structure

**Behavior to preserve:**
- `firewall.LastApplied()` is read-only; web handlers must never mutate the returned slice
- `firewall.StripZeTablePrefix()` used for display (kernel names start with ze_)
- Existing show firewall RPC format in `show/firewall.go` for JSON API compatibility
- Workbench left nav "Firewall" section key and URL mapping
- JSON content negotiation pattern (Accept: application/json returns JSON, otherwise HTML)
- HTMX partial request pattern (HX-Request header triggers fragment response, not full page)
- Config editing flows through EditorManager (per-user draft trees), not direct tree mutation

**Behavior to change:**
- Currently /show/firewall/ renders the generic YANG tree browser. This spec adds purpose-built firewall pages.
- No dedicated firewall handler exists yet. New handler file(s) and templates are needed.
- No conntrack/connections page exists. This requires either a new show command or direct telemetry access.

## Data Flow (MANDATORY)

### Entry Point
- HTTP requests to `/firewall/tables`, `/firewall/chains`, `/firewall/rules`, `/firewall/sets`, `/firewall/connections`
- Data sources: `firewall.LastApplied()` for config state, `backend.GetCounters()` for counters, config tree for edit forms, conntrack show command for connections

### Transformation Path
1. **Request routing** in `cmd/ze/hub/main.go`: routes to firewall handler functions
2. **Data retrieval** in `handler_firewall.go`: calls `firewall.LastApplied()` for table/chain/term/set data, `GetBackend().GetCounters()` for counters, EditorManager for pending edits
3. **View model construction**: transforms firewall model types into template-friendly structs (e.g., flattening matches into a summary string, computing chain/set counts per table)
4. **Content negotiation**: JSON path returns structured data directly; HTML path renders through templates
5. **Template rendering**: Go templates produce HTML tables following the workbench table pattern
6. **HTMX response**: partial requests return fragment HTML for in-page updates; full requests render within workbench shell

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Web handler <-> Firewall model | `firewall.LastApplied()` returns immutable `[]Table` | [ ] |
| Web handler <-> Backend | `firewall.GetBackend().GetCounters(name)` for per-term counters | [ ] |
| Web handler <-> Config tree | EditorManager for user draft trees (add/edit/delete operations) | [ ] |
| Web handler <-> Show commands | Dispatch function for conntrack data | [ ] |
| Browser <-> Server | HTMX for partial updates, standard form POST for mutations | [ ] |

### Integration Points
- `firewall.LastApplied()` - read applied table state for display
- `firewall.GetBackend()` - access counter readback
- `firewall.StripZeTablePrefix()` - display bare table names
- `web.Renderer` - template rendering (new firewall templates)
- `web.CommandDispatcher` - dispatch show commands for conntrack data
- `web.EditorManager` - per-user config editing sessions
- `web.WorkbenchSections()` - firewall section in left nav (already exists)
- `cmd/ze/hub/main.go` - route registration

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| GET /firewall/tables | -> | FirewallHandlers.HandleFirewallTables() | `TestHandleFirewallTables_RendersHTML` |
| GET /firewall/tables (Accept: application/json) | -> | FirewallHandlers.HandleFirewallTables() | `TestHandleFirewallTables_JSON` |
| GET /firewall/chains?table=filter | -> | FirewallHandlers.HandleFirewallChains() | `TestHandleFirewallChains_FilterByTable` |
| GET /firewall/rules?table=filter&chain=input | -> | FirewallHandlers.HandleFirewallRules() | `TestHandleFirewallRules_OrderPreserved` |
| GET /firewall/sets | -> | FirewallHandlers.HandleFirewallSets() | `TestHandleFirewallSets_RendersHTML` |
| GET /firewall/connections | -> | FirewallHandlers.HandleFirewallConnections() | `TestHandleFirewallConnections_RendersHTML` |
| POST /firewall/tables (add) | -> | FirewallHandlers.HandleFirewallTableAdd() | `TestHandleFirewallTableAdd_CreatesTable` |
| POST /firewall/rules/{table}/{chain}/move | -> | FirewallHandlers.HandleFirewallRuleMove() | `TestHandleFirewallRuleMove_ReordersCorrectly` |
| POST /firewall/rules/{table}/{chain}/clone | -> | FirewallHandlers.HandleFirewallRuleClone() | `TestHandleFirewallRuleClone_DuplicatesRule` |
| POST /firewall/rules/{table}/{chain}/{term}/toggle | -> | FirewallHandlers.HandleFirewallRuleToggle() | `TestHandleFirewallRuleToggle_DisablesRule` |
| Hub route registration | -> | /firewall/* routes registered | `TestFirewallRoutesRegistered` |
| Workbench nav: Firewall > Tables clicked | -> | full page at /firewall/tables | `workbench-firewall-tables.wb` |
| Tables row: View Chains clicked | -> | navigates to /firewall/chains?table=X | `workbench-firewall-table-to-chains.wb` |
| Chains row: View Rules clicked | -> | navigates to /firewall/rules?table=X&chain=Y | `workbench-firewall-chain-to-rules.wb` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GET /firewall/tables with applied tables | Tables page renders nftables tables with Name, Family, chain count, set count columns |
| AC-2 | Add Table form submitted with name="filter" family="inet" | Creates a table in the user's draft config tree at firewall/table/filter with family inet |
| AC-3 | GET /firewall/chains with applied chains | Chains page renders chains with Table, Name, Type, Hook, Priority, Policy, Rule Count, Packet Count, Byte Count columns |
| AC-4 | Add Chain form submitted with table selector, name, type, hook, priority, policy | Creates a chain in the user's draft config tree under the selected table |
| AC-5 | GET /firewall/rules with applied rules | Rules page renders ordered rule table with #, Flags, Chain, Match summary, Action, Packets, Bytes, Comment columns |
| AC-6 | Add Rule form submitted with match conditions and action | Creates a term in the user's draft config tree under the selected chain, appended at end |
| AC-7 | Move Up/Down action on a rule in the middle of the list | Rule order changes; preceding/following rules shift accordingly; no other rules are modified |
| AC-8 | Clone action on a rule | A duplicate rule is created with a generated name (original-name-copy or original-name-N) immediately after the source rule |
| AC-9 | Enable/Disable toggle on a rule | Rule's deactivate state toggles; disabled rules show X flag in the table |
| AC-10 | GET /firewall/sets with applied sets | Sets page renders sets with Table, Name, Type, Flags, element count columns |
| AC-11 | View Elements on a set, then Add Element | Set element viewer shows current members; add element form appends to the set in the draft tree |
| AC-12 | GET /firewall/connections | Connections page shows conntrack entries with Protocol, Source, Destination, State, Timeout, Packets, Bytes columns (read-only, no Add/Edit/Delete) |
| AC-13 | All firewall pages with empty config | Each page shows its specific empty state message with an Add button (except Connections which shows "No active connections") |
| AC-14 | Navigate: Tables row "View Chains" -> Chains row "View Rules" | Context is maintained: chains page filtered to selected table, rules page filtered to selected table+chain; breadcrumbs reflect the path |
| AC-15 | GET /firewall/rules for a chain with counter-enabled terms and backend loaded | Rule hit counters (packets/bytes) display in the Packets and Bytes columns with numeric formatting |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHandleFirewallTables_RendersHTML` | `internal/component/web/handler_firewall_test.go` | Tables page returns 200 with table rows | |
| `TestHandleFirewallTables_JSON` | `internal/component/web/handler_firewall_test.go` | Tables endpoint returns JSON with table array | |
| `TestHandleFirewallTables_Empty` | `internal/component/web/handler_firewall_test.go` | Empty state message when no tables applied | |
| `TestHandleFirewallChains_RendersHTML` | `internal/component/web/handler_firewall_test.go` | Chains page returns 200 with chain rows | |
| `TestHandleFirewallChains_FilterByTable` | `internal/component/web/handler_firewall_test.go` | Table query param filters chains to one table | |
| `TestHandleFirewallChains_FilterByHook` | `internal/component/web/handler_firewall_test.go` | Hook query param filters chains by hook point | |
| `TestHandleFirewallChains_FilterByType` | `internal/component/web/handler_firewall_test.go` | Type query param filters chains by chain type | |
| `TestHandleFirewallChains_Empty` | `internal/component/web/handler_firewall_test.go` | Empty state message when no chains | |
| `TestHandleFirewallRules_RendersHTML` | `internal/component/web/handler_firewall_test.go` | Rules page returns 200 with ordered rule rows | |
| `TestHandleFirewallRules_OrderPreserved` | `internal/component/web/handler_firewall_test.go` | Rule # column matches configured order, not alphabetical | |
| `TestHandleFirewallRules_MatchSummary` | `internal/component/web/handler_firewall_test.go` | Match column shows human-readable summary of from block (e.g., "tcp dport 22 saddr 10.0.0.0/8") | |
| `TestHandleFirewallRules_Counters` | `internal/component/web/handler_firewall_test.go` | Packets and Bytes columns populated from backend counters | |
| `TestHandleFirewallRules_FilterByChain` | `internal/component/web/handler_firewall_test.go` | Chain query param filters rules to one chain | |
| `TestHandleFirewallRules_Empty` | `internal/component/web/handler_firewall_test.go` | Empty state with policy text when no rules in chain | |
| `TestHandleFirewallRuleMove_Up` | `internal/component/web/handler_firewall_test.go` | Move up shifts rule position by -1, adjusting neighbors | |
| `TestHandleFirewallRuleMove_Down` | `internal/component/web/handler_firewall_test.go` | Move down shifts rule position by +1 | |
| `TestHandleFirewallRuleMove_AtBoundary` | `internal/component/web/handler_firewall_test.go` | Move up on first rule or move down on last rule is a no-op (not an error) | |
| `TestHandleFirewallRuleClone_DuplicatesRule` | `internal/component/web/handler_firewall_test.go` | Clone creates new term with generated name after source | |
| `TestHandleFirewallRuleClone_NameConflict` | `internal/component/web/handler_firewall_test.go` | Clone with name conflict appends numeric suffix | |
| `TestHandleFirewallRuleToggle_DisablesRule` | `internal/component/web/handler_firewall_test.go` | Toggle sets deactivate on the term | |
| `TestHandleFirewallRuleToggle_EnablesRule` | `internal/component/web/handler_firewall_test.go` | Toggle on already-disabled term removes deactivate | |
| `TestHandleFirewallSets_RendersHTML` | `internal/component/web/handler_firewall_test.go` | Sets page returns 200 with set rows | |
| `TestHandleFirewallSets_ElementCount` | `internal/component/web/handler_firewall_test.go` | Elements column shows count of set members | |
| `TestHandleFirewallSets_ViewElements` | `internal/component/web/handler_firewall_test.go` | Element viewer endpoint returns set members | |
| `TestHandleFirewallSets_AddElement` | `internal/component/web/handler_firewall_test.go` | Add element appends to set in draft tree | |
| `TestHandleFirewallSets_RemoveElement` | `internal/component/web/handler_firewall_test.go` | Remove element deletes from set in draft tree | |
| `TestHandleFirewallSets_Empty` | `internal/component/web/handler_firewall_test.go` | Empty state message when no sets | |
| `TestHandleFirewallConnections_RendersHTML` | `internal/component/web/handler_firewall_test.go` | Connections page returns 200 with conntrack rows | |
| `TestHandleFirewallConnections_Search` | `internal/component/web/handler_firewall_test.go` | IP/port/protocol search filters connection entries | |
| `TestHandleFirewallConnections_NoBackend` | `internal/component/web/handler_firewall_test.go` | Graceful message when firewall backend unavailable | |
| `TestHandleFirewallTableAdd_CreatesTable` | `internal/component/web/handler_firewall_test.go` | POST creates table in draft config tree | |
| `TestHandleFirewallTableAdd_InvalidFamily` | `internal/component/web/handler_firewall_test.go` | Rejects invalid family with 400 | |
| `TestHandleFirewallTableAdd_DuplicateName` | `internal/component/web/handler_firewall_test.go` | Rejects duplicate table name with 409 | |
| `TestHandleFirewallTableDelete_Confirmation` | `internal/component/web/handler_firewall_test.go` | Delete requires confirmation; removes table and all children | |
| `TestHandleFirewallChainAdd_WithTableSelector` | `internal/component/web/handler_firewall_test.go` | POST creates chain under selected table in draft tree | |
| `TestHandleFirewallRuleAdd_AppendsTerm` | `internal/component/web/handler_firewall_test.go` | POST creates term at end of chain's term list | |
| `TestFirewallMatchSummary` | `internal/component/web/handler_firewall_test.go` | matchSummary() converts Match slice to human-readable string | |
| `TestFirewallActionSummary` | `internal/component/web/handler_firewall_test.go` | actionSummary() extracts terminal action name from Action slice | |
| `TestFirewallTableViewModel` | `internal/component/web/handler_firewall_test.go` | tableViewModel() computes chain count, set count per table | |
| `TestFirewallContextNavigation` | `internal/component/web/handler_firewall_test.go` | Query params table= and chain= properly scope data and set breadcrumbs | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Chain priority | -400 .. 400 | -400, 400 | -401 | 401 |
| Table name length | 1 .. 255 | 255 chars | 0 (empty) | 256 chars |
| Chain name length | 1 .. 255 | 255 chars | 0 (empty) | 256 chars |
| Term name length | 1 .. 255 | 255 chars | 0 (empty) | 256 chars |
| Set element timeout | 0 .. 4294967295 | max uint32 | N/A (0 valid) | 4294967296 |
| Port number (in match form) | 1 .. 65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `workbench-firewall-tables.wb` | `test/web/workbench-firewall-tables.wb` | Navigate to Firewall > Tables, see table list with columns | |
| `workbench-firewall-tables-empty.wb` | `test/web/workbench-firewall-tables-empty.wb` | Empty config shows "No firewall tables configured" message and Add button | |
| `workbench-firewall-table-to-chains.wb` | `test/web/workbench-firewall-table-to-chains.wb` | Click "View Chains" on table row, chains page filtered to that table | |
| `workbench-firewall-chain-to-rules.wb` | `test/web/workbench-firewall-chain-to-rules.wb` | Click "View Rules" on chain row, rules page filtered to that chain | |
| `workbench-firewall-rules-order.wb` | `test/web/workbench-firewall-rules-order.wb` | Rules display in configured order with correct # column | |
| `workbench-firewall-rule-move.wb` | `test/web/workbench-firewall-rule-move.wb` | Move Up/Down reorders rules and page refreshes with new order | |
| `workbench-firewall-rule-clone.wb` | `test/web/workbench-firewall-rule-clone.wb` | Clone creates duplicate rule appearing after original | |
| `workbench-firewall-sets.wb` | `test/web/workbench-firewall-sets.wb` | Sets page renders with element counts | |
| `workbench-firewall-connections.wb` | `test/web/workbench-firewall-connections.wb` | Connections page shows conntrack entries (read-only) | |
| `workbench-firewall-add-table.wb` | `test/web/workbench-firewall-add-table.wb` | Add Table form creates table in draft config | |

### Future (if deferring any tests)
- Conntrack search/filter functional tests: may require a running firewall backend with actual connections; defer to integration test phase if conntrack data is not available in unit test environment

## Files to Modify
- `internal/component/web/handler_firewall.go` - NEW: Firewall page handlers (Tables, Chains, Rules, Sets, Connections)
- `internal/component/web/handler_firewall_test.go` - NEW: Tests for all firewall handlers
- `internal/component/web/render.go` - Add RenderFirewallTemplate method (following RenderL2TPTemplate pattern)
- `cmd/ze/hub/main.go` - Register /firewall/* routes with auth wrapper
- `internal/component/web/workbench_sections.go` - Update firewall sub-pages in left nav if needed (Tables, Chains, Rules, Sets, Connections)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No - uses existing firewall model, no new RPCs | - |
| CLI commands/flags | [ ] No - web pages, not CLI | - |
| Editor autocomplete | [ ] No - YANG-driven, no YANG changes | - |
| Functional test for new RPC/API | [ ] No - functional tests are .wb web tests | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] Yes | `docs/features.md` - add firewall web management |
| 2 | Config syntax changed? | [ ] No | - |
| 3 | CLI command added/changed? | [ ] No | - |
| 4 | API/RPC added/changed? | [ ] No | - |
| 5 | Plugin added/changed? | [ ] No | - |
| 6 | Has a user guide page? | [x] Yes | `docs/guide/web-interface.md` - add firewall pages section |
| 7 | Wire format changed? | [ ] No | - |
| 8 | Plugin SDK/protocol changed? | [ ] No | - |
| 9 | RFC behavior implemented? | [ ] No | - |
| 10 | Test infrastructure changed? | [ ] No | - |
| 11 | Affects daemon comparison? | [ ] No | - |
| 12 | Internal architecture changed? | [ ] No | - |

## Files to Create
- `internal/component/web/handler_firewall.go` - FirewallHandlers struct and all handler methods
- `internal/component/web/handler_firewall_test.go` - Comprehensive unit tests
- `internal/component/web/templates/firewall/tables.html` - Tables page template
- `internal/component/web/templates/firewall/chains.html` - Chains page template
- `internal/component/web/templates/firewall/rules.html` - Rules page template
- `internal/component/web/templates/firewall/sets.html` - Sets page template
- `internal/component/web/templates/firewall/set_elements.html` - Set element viewer/editor template
- `internal/component/web/templates/firewall/connections.html` - Connections page template
- `test/web/workbench-firewall-tables.wb` - Tables page functional test
- `test/web/workbench-firewall-tables-empty.wb` - Empty tables functional test
- `test/web/workbench-firewall-table-to-chains.wb` - Table-to-chains navigation test
- `test/web/workbench-firewall-chain-to-rules.wb` - Chain-to-rules navigation test
- `test/web/workbench-firewall-rules-order.wb` - Rule ordering test
- `test/web/workbench-firewall-rule-move.wb` - Rule move up/down test
- `test/web/workbench-firewall-rule-clone.wb` - Rule clone test
- `test/web/workbench-firewall-sets.wb` - Sets page test
- `test/web/workbench-firewall-connections.wb` - Connections page test
- `test/web/workbench-firewall-add-table.wb` - Add table form test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan: check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. /ze-review gate | Review Gate section: run `/ze-review`; fix every BLOCKER/ISSUE; re-run until only NOTEs remain (BEFORE full verification) |
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

1. **Phase: Tables page** -- Handler, template, data model, and route registration for the Tables page
   - Tests: `TestHandleFirewallTables_RendersHTML`, `TestHandleFirewallTables_JSON`, `TestHandleFirewallTables_Empty`, `TestHandleFirewallTableAdd_CreatesTable`, `TestHandleFirewallTableAdd_InvalidFamily`, `TestHandleFirewallTableAdd_DuplicateName`, `TestHandleFirewallTableDelete_Confirmation`, `TestFirewallTableViewModel`
   - Files: `handler_firewall.go`, `handler_firewall_test.go`, `templates/firewall/tables.html`, `render.go`, `cmd/ze/hub/main.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Chains page** -- Chains handler with table/hook/type filtering
   - Tests: `TestHandleFirewallChains_RendersHTML`, `TestHandleFirewallChains_FilterByTable`, `TestHandleFirewallChains_FilterByHook`, `TestHandleFirewallChains_FilterByType`, `TestHandleFirewallChains_Empty`, `TestHandleFirewallChainAdd_WithTableSelector`
   - Files: `handler_firewall.go`, `handler_firewall_test.go`, `templates/firewall/chains.html`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Rules page** -- Ordered rule table with match summaries, action display, and counter integration
   - Tests: `TestHandleFirewallRules_RendersHTML`, `TestHandleFirewallRules_OrderPreserved`, `TestHandleFirewallRules_MatchSummary`, `TestHandleFirewallRules_Counters`, `TestHandleFirewallRules_FilterByChain`, `TestHandleFirewallRules_Empty`, `TestHandleFirewallRuleAdd_AppendsTerm`, `TestFirewallMatchSummary`, `TestFirewallActionSummary`
   - Files: `handler_firewall.go`, `handler_firewall_test.go`, `templates/firewall/rules.html`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Move, Clone, Toggle** -- Rule reordering, duplication, and enable/disable actions
   - Tests: `TestHandleFirewallRuleMove_Up`, `TestHandleFirewallRuleMove_Down`, `TestHandleFirewallRuleMove_AtBoundary`, `TestHandleFirewallRuleClone_DuplicatesRule`, `TestHandleFirewallRuleClone_NameConflict`, `TestHandleFirewallRuleToggle_DisablesRule`, `TestHandleFirewallRuleToggle_EnablesRule`
   - Files: `handler_firewall.go`, `handler_firewall_test.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Sets page** -- Sets handler with element management
   - Tests: `TestHandleFirewallSets_RendersHTML`, `TestHandleFirewallSets_ElementCount`, `TestHandleFirewallSets_ViewElements`, `TestHandleFirewallSets_AddElement`, `TestHandleFirewallSets_RemoveElement`, `TestHandleFirewallSets_Empty`
   - Files: `handler_firewall.go`, `handler_firewall_test.go`, `templates/firewall/sets.html`, `templates/firewall/set_elements.html`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Connections page** -- Read-only conntrack display with search
   - Tests: `TestHandleFirewallConnections_RendersHTML`, `TestHandleFirewallConnections_Search`, `TestHandleFirewallConnections_NoBackend`
   - Files: `handler_firewall.go`, `handler_firewall_test.go`, `templates/firewall/connections.html`
   - Verify: tests fail -> implement -> tests pass

7. **Phase: Cross-page navigation** -- Context-preserving navigation between Tables, Chains, and Rules; breadcrumbs; left nav sub-pages
   - Tests: `TestFirewallContextNavigation`, `TestFirewallRoutesRegistered`
   - Files: `handler_firewall.go`, `handler_firewall_test.go`, `workbench_sections.go`
   - Verify: tests fail -> implement -> tests pass

8. **Functional tests** -- Create after feature works. Cover user-visible behavior.
   - All `.wb` functional test files from the TDD plan
   - Verify: all functional tests pass

9. **Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)

10. **Complete spec** -- Fill audit tables, write learned summary to `plan/learned/NNN-web-6-firewall.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-15 has implementation with file:line |
| Correctness | Rule ordering preserved through all operations (move, clone, add); match summaries accurate for all 15+ match types |
| Naming | Route paths use kebab-case (`/firewall/tables`); template files use snake_case; Go functions use CamelCase; query params use lowercase |
| Data flow | Handlers read from LastApplied() and GetCounters() only; mutations go through EditorManager draft trees |
| Rule: no-layering | No duplication of firewall model parsing (use existing `internal/component/firewall/` types) |
| Rule: exact-or-reject | Invalid table/chain/term names rejected with clear error messages, not silently accepted |
| Rule: derive-not-hardcode | Family, hook, type, policy dropdown values derived from firewall model enum maps, not hardcoded in templates |
| Immutability | Handlers never mutate the `[]Table` slice returned by `LastApplied()` |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `handler_firewall.go` exists with all handlers | `ls -la internal/component/web/handler_firewall.go` |
| `handler_firewall_test.go` exists with all tests | `grep -c 'func Test' internal/component/web/handler_firewall_test.go` >= 35 |
| 6 firewall templates exist | `ls internal/component/web/templates/firewall/*.html` shows 6+ files |
| Routes registered in hub | `grep -c '/firewall/' cmd/ze/hub/main.go` >= 6 |
| Tables handler returns 200 | `go test -run TestHandleFirewallTables_RendersHTML ./internal/component/web/` |
| Rules order preserved | `go test -run TestHandleFirewallRules_OrderPreserved ./internal/component/web/` |
| Move up/down works | `go test -run TestHandleFirewallRuleMove ./internal/component/web/` |
| Clone works | `go test -run TestHandleFirewallRuleClone ./internal/component/web/` |
| 10 functional .wb tests exist | `ls test/web/workbench-firewall-*.wb` shows 10 files |
| All unit tests pass | `go test ./internal/component/web/ -run TestHandleFirewall` |
| Docs updated | `grep -l 'firewall' docs/features.md docs/guide/web-interface.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Table/chain/term names validated with ValidateName() before any tree operation; family/hook/type/policy validated against enum maps; port numbers validated 1-65535 |
| Path traversal | URL path parameters for table/chain/term names must not contain path separators or dot-dot sequences |
| XSS prevention | All template output properly escaped; match summaries and action names rendered through Go template escaping, not raw HTML |
| CSRF protection | All mutation endpoints (POST) require valid session; HTMX requests carry session cookie |
| Authorization | All firewall routes wrapped with authWrap() in hub registration |
| Resource exhaustion | Connections page limits result count (conntrack can be huge); use pagination or max-rows |
| Error leakage | Backend errors logged server-side; client sees generic "firewall backend unavailable" not stack traces |
| Injection | Form values for names, addresses, ports sanitized before passing to config tree; no shell command injection through dispatch |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

- Firewall terms use named keys (`ordered-by user` in YANG), not numeric positions. Move Up/Down must reorder the YANG list entries in the draft config tree, not change a numeric index. The display # column is derived from position in the list, not a stored field.
- Match summary generation needs to handle 15+ match types. A type switch in Go is the natural fit. The summary should read naturally (e.g., "tcp dport 22,80,443 saddr 10.0.0.0/8") rather than showing raw struct fields.
- Conntrack data does not live in the config tree. It must be sourced from a show command dispatch or a direct backend query. If no backend is loaded, the connections page should gracefully show "Firewall backend not configured" rather than an error.
- The firewall model uses `ze_` table name prefix in kernel but bare names in config. All display must use `StripZeTablePrefix()` for consistency with CLI.
- Dropdown values for family, hook, type, policy must be derived from the model's enum maps (familyNames, chainHookNames, chainTypeNames, policyNames) to satisfy derive-not-hardcode.

## RFC Documentation

_Not applicable. Firewall web UI is not protocol work._

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered, add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

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

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete, every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled, 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md`, no failures)

### Quality Gates (SHOULD pass, defer with user approval)
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

### Completion (BLOCKING, before ANY commit)
- [ ] Critical Review passes, all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-web-6-firewall.md`
- [ ] **Summary included in commit**, NEVER commit implementation without the completed summary. One commit = code + tests + summary.
