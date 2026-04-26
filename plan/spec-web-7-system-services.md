# Spec: web-7 -- System, Services, and L2TP Pages

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-web-3-foundation |
| Phase | 7/7 |
| Updated | 2026-04-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/design/web-interface-design.md` - page layouts and navigation structure
4. `ai/patterns/web-endpoint.md` - handler, template, HTMX, and route conventions
5. `docs/architecture/web-interface.md` - current web UI architecture
6. `internal/component/web/handler_workbench.go` - V2 workbench handler
7. `internal/component/web/handler_l2tp.go` - existing L2TP web handler (preserve)
8. `internal/component/cmd/show/system.go` - show system/* handlers
9. `internal/component/cmd/show/host.go` - show host/* handlers
10. `internal/component/cmd/show/show.go` - show verb RPC registration (l2tp-health, version, uptime)
11. YANG schemas listed in Current Behavior

## Task

Build the System, Services, and L2TP pages for the operator workbench V2 web UI. This spec covers 16 pages across three navigation sections defined in `plan/design/web-interface-design.md`:

- **System** (5 pages): Identity, Users, Resources, Host Hardware, Sysctl Profiles
- **Services** (7 pages): SSH, Web, Telemetry, TACACS, MCP, Looking Glass, API
- **L2TP** (3 pages): Sessions, Configuration, Health

Each page follows the design document's patterns exactly: tables for lists of objects, forms for singleton configuration, property lists for read-only operational data. Every page has proper empty states, toolbar actions, and row actions as defined in the design document.

The L2TP section must reuse and adapt the existing `handler_l2tp.go` implementation, which already provides session listing, detail, samples (JSON/CSV/SSE), and disconnect functionality.

-> Decision: Pages are rendered server-side with Go templates + HTMX, consistent with the V2 workbench architecture established in spec-web-2.
-> Decision: Configuration pages use the standard config/commit flow via `EditorManager` and per-user draft trees.
-> Decision: Operational pages (Resources, Host Hardware, L2TP Health) source data from existing show command RPCs, not by duplicating detection logic.
-> Constraint: All pages live behind the V2 workbench UI mode gate (`ze.web.ui=workbench` during Phases 1-3, default after Phase 4 promotion).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `plan/design/web-interface-design.md` - page layouts, navigation structure, empty state patterns, form patterns
  -> Decision: Tables are the primary view for lists; forms for singletons; property lists for operational data
  -> Constraint: Every table when empty shows column headers plus a specific empty message and Add button
- [ ] `docs/architecture/web-interface.md` - web server structure, rendering pipeline
  -> Constraint: All handlers follow the 6-step pattern from `ai/patterns/web-endpoint.md`
- [ ] `docs/architecture/web-components.md` - reusable component library
  -> Constraint: Reuse table, form, empty state, and toolbar components from spec-web-3-foundation
- [ ] `ai/patterns/web-endpoint.md` - handler pattern, URL routing, content negotiation
  -> Constraint: JSON and HTMX fragment responses alongside full page renders
- [ ] `ai/patterns/registration.md` - route registration pattern
  -> Constraint: New routes register via the standard web route registration

### RFC Summaries (MUST for protocol work)
- Not applicable (web UI feature, not protocol work)

**Key insights:** (summary of all checkpoint lines)
- Server-rendered Go templates + HTMX; no client-side framework
- Config pages reuse EditorManager for per-user draft sessions and the standard diff/commit flow
- Operational pages dispatch show commands via CommandDispatcher and render results
- Existing L2TP handler (`handler_l2tp.go`) provides session list, detail, samples, SSE streaming, and disconnect; these must be preserved and adapted to the workbench shell
- YANG schemas define the authoritative config structure for each service; the web forms must mirror their leaf/list topology

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/component/web/handler_workbench.go` - V2 workbench page handler, reuses fragment data builder
  -> Constraint: New pages must integrate into the workbench shell (WorkbenchData, WorkbenchSections)
- [ ] `internal/component/web/handler_l2tp.go` - L2TP list, detail, samples (JSON/CSV/SSE), disconnect handlers
  -> Constraint: Preserve all existing L2TP functionality; adapt to workbench layout
- [ ] `internal/component/web/render.go` - Renderer with RenderWorkbench, RenderFragment, RenderL2TPTemplate methods
  -> Constraint: L2TP already has its own template namespace (l2tp/list.html, l2tp/detail.html)
- [ ] `internal/component/web/templates/l2tp/list.html` - existing L2TP session list template
- [ ] `internal/component/web/templates/l2tp/detail.html` - existing L2TP session detail template
- [ ] `internal/component/cmd/show/show.go` - RPC registration: ze-show:version, ze-show:uptime, ze-show:system-memory, ze-show:system-cpu, ze-show:system-date, ze-show:l2tp-health, ze-show:bgp-health
  -> Constraint: Resources page sources data from these existing RPCs
- [ ] `internal/component/cmd/show/system.go` - handleShowSystemMemory (Go MemStats + host hardware), handleShowSystemCPU (goroutines, GOMAXPROCS + host hardware), handleShowSystemDate (RFC3339, timezone)
  -> Constraint: Resources page aggregates data from system-memory, system-cpu, version, uptime, system-date RPCs
- [ ] `internal/component/cmd/show/host.go` - Generated host section handlers from host.SectionNames(); dispatches via host.DetectSection()
  -> Constraint: Host Hardware page calls show host/* RPCs (cpu, nic, storage, memory, thermal, dmi, kernel)
- [ ] `internal/component/host/inventory.go` - Inventory struct with CPU, NICs, DMI, Memory, Thermal, Storage, Kernel sections
  -> Constraint: Host Hardware page layout mirrors the Inventory struct sections
- [ ] `internal/component/config/system/schema/ze-system-conf.yang` - system container: host, domain, name-server, dns{}, peeringdb{}, archive{}
  -> Constraint: Identity page edits system.host (hostname); router-id comes from BGP config, not system YANG
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` - system.authentication.user[] (name, password, profile[], public-keys[]), environment.ssh (enabled, server[], host-key, idle-timeout, max-sessions)
  -> Constraint: Users page maps to system.authentication.user[]; SSH service page maps to environment.ssh
- [ ] `internal/component/web/schema/ze-web-conf.yang` - environment.web (enabled, server[], insecure)
  -> Constraint: Web service page maps to environment.web
- [ ] `internal/component/telemetry/schema/ze-telemetry-conf.yang` - telemetry.prometheus (enabled, server[], path, prefix, interval, collector[])
  -> Constraint: Telemetry service page maps to telemetry.prometheus
- [ ] `internal/component/tacacs/schema/ze-tacacs-conf.yang` - system.authentication.tacacs (server[], timeout, source-address, authorization, accounting), tacacs-profile[]
  -> Constraint: TACACS service page maps to system.authentication.tacacs
- [ ] `internal/component/mcp/schema/ze-mcp-conf.yang` - environment.mcp (enabled, bind-remote, auth-mode, token, identity[], oauth{}, tls{}, server[])
  -> Constraint: MCP service page maps to environment.mcp; sensitive fields (token, key) masked in display
- [ ] `internal/component/l2tp/schema/ze-l2tp-conf.yang` - l2tp (enabled, max-tunnels, max-sessions, shared-secret, hello-interval, cqm-enabled, max-logins, ...), environment.l2tp.server[]
  -> Constraint: L2TP config page maps to l2tp{} + environment.l2tp.server[]
- [ ] `internal/component/lg/schema/ze-lg-conf.yang` - environment.looking-glass (enabled, server[], tls)
  -> Constraint: Looking Glass service page maps to environment.looking-glass
- [ ] `internal/component/authz/schema/ze-authz-conf.yang` - system.authorization.profile[] (name, run{}, edit{})
  -> Constraint: Users page shows profile names from authz config; full profile editing is out of scope for this spec
- [ ] `internal/plugins/sysctl/schema/ze-sysctl-conf.yang` - sysctl.setting[], sysctl.profile[] (name, setting[])
  -> Constraint: Sysctl Profiles page maps to sysctl.profile[]; built-in profiles are read-only, custom profiles are editable

**Behavior to preserve:** (unless user explicitly said to change)
- L2TP session list, detail, samples (JSON/CSV/SSE), and disconnect functionality from `handler_l2tp.go`
- L2TP content negotiation (JSON vs HTML)
- L2TP disconnect command dispatch via CommandDispatcher with actor tracking
- L2TP input validation (session ID parsing, login extraction, reason length, cause code range)
- L2TP CQM SSE heartbeat pattern
- All existing show command RPC contracts and JSON output shapes
- Config diff/commit flow through EditorManager
- Per-user draft session isolation
- Web authentication and authorization
- CSP headers

**Behavior to change:** (only if user explicitly requested)
- L2TP pages render inside the workbench shell instead of standalone pages
- L2TP navigation moves from flat `/l2tp` to the workbench left nav under "L2TP" section
- System/Services pages are new additions (no existing behavior to change)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- **Config pages (Identity, Users, SSH, Web, Telemetry, TACACS, MCP, LG, API, L2TP Config, Sysctl):** User navigates to workbench page via left nav. Handler reads YANG schema + config tree to populate form fields.
- **Operational pages (Resources, Host Hardware, L2TP Health):** User navigates to workbench page. Handler dispatches show commands via CommandDispatcher and renders the JSON response.
- **L2TP Sessions:** User navigates to L2TP > Sessions. Handler calls `l2tp.LookupService().Snapshot()` directly (existing pattern from `handler_l2tp.go`).

### Transformation Path
1. **Navigation:** Left sidebar click -> HTMX partial request -> workbench handler routes to section-specific handler
2. **Config form read:** Handler walks YANG schema for the config path -> reads current values from config tree (or user's draft tree) -> renders form template
3. **Config form save:** POST with form values -> standard `/config/set/<path>` endpoint -> updates draft tree -> pending change marker appears
4. **Operational data read:** Handler dispatches show RPC via CommandDispatcher -> receives plugin.Response -> extracts .Data map -> renders property list or table template
5. **L2TP session data:** Handler calls `l2tp.LookupService().Snapshot()` -> iterates tunnels and sessions -> renders table template
6. **Auto-refresh (Resources, L2TP Health):** HTMX `hx-trigger="every 5s"` on the content container -> partial request -> re-renders the data section

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Web handler -> Config tree | EditorManager.Tree(username) for draft, config.Tree for committed | [ ] |
| Web handler -> Show RPCs | CommandDispatcher("show system/memory", username, remoteAddr) | [ ] |
| Web handler -> L2TP service | l2tp.LookupService().Snapshot() (direct Go call, not RPC) | [ ] |
| Web handler -> YANG schema | config.Schema walk for form field metadata | [ ] |
| Browser -> Server | HTMX partial requests (HX-Request header) | [ ] |
| Form POST -> Config | Standard /config/set/<path> POST via HTMX | [ ] |

### Integration Points
- `HandleWorkbench()` in `handler_workbench.go` - new pages route through the workbench shell
- `WorkbenchSections()` in workbench_sections.go - System/Services/L2TP sections already defined in nav taxonomy
- `L2TPHandlers` struct in `handler_l2tp.go` - reuse for L2TP Sessions and adapt for workbench
- `EditorManager` - per-user draft sessions for all config forms
- `CommandDispatcher` type - dispatch show commands for operational pages
- `Renderer.RenderFragment()` - render page content as HTMX fragments
- `Renderer.RenderL2TPTemplate()` - existing L2TP template rendering
- `host.SectionNames()` / `host.DetectSection()` - Host Hardware data source

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| GET /show/system/identity (workbench) | -> | System Identity form handler | `TestWorkbenchSystemIdentity` |
| GET /show/system/users (workbench) | -> | Users table handler | `TestWorkbenchSystemUsers` |
| GET /show/system/resources (workbench) | -> | Resources property list handler | `TestWorkbenchSystemResources` |
| GET /show/system/host-hardware (workbench) | -> | Host Hardware handler | `TestWorkbenchSystemHostHardware` |
| GET /show/system/sysctl-profiles (workbench) | -> | Sysctl Profiles table handler | `TestWorkbenchSystemSysctlProfiles` |
| GET /show/services/ssh (workbench) | -> | SSH config form handler | `TestWorkbenchServiceSSH` |
| GET /show/services/web (workbench) | -> | Web config form handler | `TestWorkbenchServiceWeb` |
| GET /show/services/telemetry (workbench) | -> | Telemetry config form handler | `TestWorkbenchServiceTelemetry` |
| GET /show/services/tacacs (workbench) | -> | TACACS config form handler | `TestWorkbenchServiceTACACS` |
| GET /show/services/mcp (workbench) | -> | MCP config form handler | `TestWorkbenchServiceMCP` |
| GET /show/services/looking-glass (workbench) | -> | Looking Glass config form handler | `TestWorkbenchServiceLookingGlass` |
| GET /show/services/api (workbench) | -> | API config form handler | `TestWorkbenchServiceAPI` |
| GET /show/l2tp/sessions (workbench) | -> | L2TP Sessions table (adapted from handler_l2tp.go) | `TestWorkbenchL2TPSessions` |
| GET /show/l2tp/configuration (workbench) | -> | L2TP Config form handler | `TestWorkbenchL2TPConfiguration` |
| GET /show/l2tp/health (workbench) | -> | L2TP Health table handler | `TestWorkbenchL2TPHealth` |
| POST /l2tp/{sid}/disconnect | -> | L2TPHandlers.HandleL2TPDisconnect() | `TestL2TPDisconnectPreserved` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Navigate to System > Identity | Form shows hostname and router-id fields populated from config; saving updates the draft tree |
| AC-2 | Navigate to System > Users | Table shows user list with columns: Username, Groups (profile names), Permissions (summary), Last Login |
| AC-3 | Click Add User on Users page | Form creates a new user entry under system.authentication.user[] with password and profile assignment |
| AC-4 | Navigate to System > Resources | Property list shows Uptime, Version, CPU Cores, CPU Load %, GOMAXPROCS, Goroutines, Memory Allocated, Memory Total, GC Runs, Current Time; auto-refreshes every 5 seconds |
| AC-5 | Navigate to System > Host Hardware | Page shows inventory tables organized by subsystem: CPU cores, NICs, Storage, Memory, Thermal sensors, DMI info |
| AC-6 | Navigate to System > Sysctl Profiles | Table shows profiles with Name, Applied To (interfaces/units), Parameters columns; View action shows all sysctl values; Edit works only for custom profiles |
| AC-7 | Navigate to any Service page (SSH, Web, Telemetry, TACACS, MCP, LG, API) | Config form renders with correct fields matching the YANG schema for that service |
| AC-8 | Edit a field on any Service page and save | Draft tree is updated via standard config/set flow; pending change marker appears |
| AC-9 | Service forms with list fields (SSH server[], TACACS server[], MCP identity[], MCP server[], L2TP server[]) | Add/remove rows work via HTMX; each row shows existing values |
| AC-10 | Navigate to L2TP > Sessions | Table shows active sessions with columns: Tunnel ID, Session ID, Peer, State, Uptime, TX/RX, Echo Loss % |
| AC-11 | Click Disconnect on an L2TP session row | Confirmation dialog appears; confirming dispatches the disconnect command via CommandDispatcher |
| AC-12 | Navigate to L2TP > Configuration | Form shows all L2TP settings (enabled, max-tunnels, max-sessions, shared-secret, hello-interval, cqm-enabled, max-logins) plus server endpoints table |
| AC-13 | Navigate to L2TP > Health | Table shows sessions sorted by echo loss (worst first) with columns: Session, Peer, Echo Loss %, Latency, State |
| AC-14 | Navigate to any page with no data | Appropriate empty state message is shown with the correct page-specific text |
| AC-15 | Existing L2TP /l2tp endpoints | All existing L2TP endpoints (/l2tp, /l2tp/{sid}, /l2tp/{login}/samples, /l2tp/{login}/samples.csv, /l2tp/{login}/samples/stream, /l2tp/{sid}/disconnect) continue to work |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWorkbenchSystemIdentity` | `internal/component/web/handler_system_test.go` | Identity form renders hostname and router-id fields | |
| `TestWorkbenchSystemIdentitySave` | `internal/component/web/handler_system_test.go` | Identity form save updates draft tree | |
| `TestWorkbenchSystemUsers` | `internal/component/web/handler_system_test.go` | Users table renders user list from config | |
| `TestWorkbenchSystemUsersEmpty` | `internal/component/web/handler_system_test.go` | Users table shows empty state message | |
| `TestWorkbenchSystemUsersAddUser` | `internal/component/web/handler_system_test.go` | Add User creates entry under system.authentication.user[] | |
| `TestWorkbenchSystemResources` | `internal/component/web/handler_system_test.go` | Resources property list populates from show RPCs | |
| `TestWorkbenchSystemResourcesAutoRefresh` | `internal/component/web/handler_system_test.go` | Resources HTMX fragment re-renders on polling trigger | |
| `TestWorkbenchSystemHostHardware` | `internal/component/web/handler_system_test.go` | Host Hardware renders subsystem sections from inventory | |
| `TestWorkbenchSystemHostHardwareUnsupported` | `internal/component/web/handler_system_test.go` | Host Hardware gracefully handles ErrUnsupported sections | |
| `TestWorkbenchSystemSysctlProfiles` | `internal/component/web/handler_system_test.go` | Sysctl Profiles table renders profile list | |
| `TestWorkbenchSystemSysctlProfilesView` | `internal/component/web/handler_system_test.go` | View action shows all sysctl values for a profile | |
| `TestWorkbenchServiceSSH` | `internal/component/web/handler_services_test.go` | SSH form renders enabled, server[], host-key, idle-timeout, max-sessions | |
| `TestWorkbenchServiceSSHSave` | `internal/component/web/handler_services_test.go` | SSH form save updates draft tree for environment.ssh | |
| `TestWorkbenchServiceSSHServerList` | `internal/component/web/handler_services_test.go` | SSH server[] add/remove renders correctly | |
| `TestWorkbenchServiceWeb` | `internal/component/web/handler_services_test.go` | Web form renders enabled, server[], insecure fields | |
| `TestWorkbenchServiceTelemetry` | `internal/component/web/handler_services_test.go` | Telemetry form renders prometheus section with collector[] | |
| `TestWorkbenchServiceTACACS` | `internal/component/web/handler_services_test.go` | TACACS form renders server[], timeout, source-address, authorization, accounting | |
| `TestWorkbenchServiceMCP` | `internal/component/web/handler_services_test.go` | MCP form renders auth-mode, identity[], oauth{}, tls{}, server[] | |
| `TestWorkbenchServiceMCPSensitiveFields` | `internal/component/web/handler_services_test.go` | MCP sensitive fields (token, key) are masked in display | |
| `TestWorkbenchServiceLookingGlass` | `internal/component/web/handler_services_test.go` | LG form renders enabled, server[], tls | |
| `TestWorkbenchServiceAPI` | `internal/component/web/handler_services_test.go` | API form renders listen address and TLS config | |
| `TestWorkbenchServiceFormListAddRemove` | `internal/component/web/handler_services_test.go` | Generic list field add/remove works for any service form | |
| `TestWorkbenchL2TPSessions` | `internal/component/web/handler_l2tp_workbench_test.go` | L2TP sessions table renders inside workbench shell | |
| `TestWorkbenchL2TPSessionsEmpty` | `internal/component/web/handler_l2tp_workbench_test.go` | L2TP sessions empty state shows "No active L2TP sessions." | |
| `TestWorkbenchL2TPConfiguration` | `internal/component/web/handler_l2tp_workbench_test.go` | L2TP config form renders all settings from YANG | |
| `TestWorkbenchL2TPConfigurationSave` | `internal/component/web/handler_l2tp_workbench_test.go` | L2TP config save updates draft tree | |
| `TestWorkbenchL2TPConfigurationServerList` | `internal/component/web/handler_l2tp_workbench_test.go` | L2TP server endpoint add/remove works | |
| `TestWorkbenchL2TPHealth` | `internal/component/web/handler_l2tp_workbench_test.go` | L2TP health table renders sorted by echo loss | |
| `TestWorkbenchL2TPHealthEmpty` | `internal/component/web/handler_l2tp_workbench_test.go` | L2TP health empty state when no sessions | |
| `TestL2TPDisconnectPreserved` | `internal/component/web/handler_l2tp_workbench_test.go` | Existing disconnect handler continues to work with confirmation flow | |
| `TestWorkbenchAllEmptyStates` | `internal/component/web/handler_empty_states_test.go` | Every page shows correct empty state text per design doc | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| L2TP max-tunnels | 0-65535 (uint16) | 65535 | N/A (0 means unlimited) | 65536 |
| L2TP max-sessions | 0-65535 (uint16) | 65535 | N/A (0 means unlimited) | 65536 |
| L2TP hello-interval | 1-3600 | 3600 | 0 | 3601 |
| L2TP max-logins | 1-1000000 | 1000000 | 0 | 1000001 |
| Telemetry interval | 1-60 | 60 | 0 | 61 |
| TACACS timeout | 1-300 | 300 | 0 | 301 |
| SSH idle-timeout | 0-max uint32 | 4294967295 | N/A | N/A |
| SSH max-sessions | 0-65535 (uint16) | 65535 | N/A | 65536 |
| TACACS port | 0-65535 (uint16) | 65535 | N/A | 65536 |
| DNS cache-size | 0-1000000 | 1000000 | N/A | 1000001 |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-web-system-identity` | `test/web/system-identity.ci` | Navigate to System > Identity, verify form fields, edit hostname, save | |
| `test-web-system-users` | `test/web/system-users.ci` | Navigate to System > Users, add a user, verify in table, delete | |
| `test-web-system-resources` | `test/web/system-resources.ci` | Navigate to System > Resources, verify all properties display, verify auto-refresh | |
| `test-web-system-host-hw` | `test/web/system-host-hardware.ci` | Navigate to System > Host Hardware, verify subsystem sections render | |
| `test-web-service-forms` | `test/web/service-forms.ci` | Navigate to each Service page, verify form renders, edit a field, save | |
| `test-web-l2tp-sessions` | `test/web/l2tp-sessions.ci` | Navigate to L2TP > Sessions, verify table renders (empty or with data) | |
| `test-web-l2tp-config` | `test/web/l2tp-config.ci` | Navigate to L2TP > Configuration, edit settings, save | |
| `test-web-l2tp-health` | `test/web/l2tp-health.ci` | Navigate to L2TP > Health, verify sorted by echo loss | |
| `test-web-l2tp-compat` | `test/web/l2tp-compat.ci` | Verify existing /l2tp endpoints still work alongside workbench pages | |

### Future (if deferring any tests)
- Sysctl profile "Applied To" cross-referencing (requires interface config correlation, may be deferred if interface config is not yet wired to sysctl profiles in the current codebase)

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/component/web/handler_system.go` - System section handlers (Identity, Users, Resources, Host Hardware, Sysctl Profiles)
- `internal/component/web/handler_services.go` - Services section handlers (SSH, Web, Telemetry, TACACS, MCP, LG, API)
- `internal/component/web/handler_l2tp.go` - Adapt existing L2TP handlers for workbench integration
- `internal/component/web/handler_workbench.go` - Route workbench section paths to new handlers
- `internal/component/web/render.go` - Register new template namespaces if needed
- `internal/component/web/workbench_sections.go` - Verify System/Services/L2TP sections are defined in nav taxonomy

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | Existing YANG schemas and show RPCs are sufficient |
| CLI commands/flags | [ ] No | No new CLI commands; web pages use existing show RPCs |
| Editor autocomplete | [ ] No | YANG-driven (no changes needed) |
| Functional test for new RPC/API | [ ] No | No new RPCs; functional tests cover web page rendering |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Yes | `docs/features.md` - add System/Services/L2TP workbench pages |
| 2 | Config syntax changed? | [ ] No | N/A |
| 3 | CLI command added/changed? | [ ] No | N/A |
| 4 | API/RPC added/changed? | [ ] No | N/A |
| 5 | Plugin added/changed? | [ ] No | N/A |
| 6 | Has a user guide page? | [ ] Yes | `docs/guide/web-interface.md` - document new workbench pages |
| 7 | Wire format changed? | [ ] No | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] No | N/A |
| 9 | RFC behavior implemented? | [ ] No | N/A |
| 10 | Test infrastructure changed? | [ ] No | N/A |
| 11 | Affects daemon comparison? | [ ] No | N/A |
| 12 | Internal architecture changed? | [ ] No | N/A |

## Files to Create
- `internal/component/web/handler_system.go` - System section page handlers
- `internal/component/web/handler_system_test.go` - System section handler tests
- `internal/component/web/handler_services.go` - Services section page handlers
- `internal/component/web/handler_services_test.go` - Services section handler tests
- `internal/component/web/handler_l2tp_workbench.go` - L2TP workbench adapter (bridges existing L2TPHandlers into workbench shell)
- `internal/component/web/handler_l2tp_workbench_test.go` - L2TP workbench adapter tests
- `internal/component/web/handler_empty_states_test.go` - Empty state verification tests
- `internal/component/web/templates/system/identity.html` - Identity form template
- `internal/component/web/templates/system/users.html` - Users table template
- `internal/component/web/templates/system/users_add.html` - Add User form template
- `internal/component/web/templates/system/resources.html` - Resources property list template
- `internal/component/web/templates/system/host_hardware.html` - Host Hardware inventory template
- `internal/component/web/templates/system/sysctl_profiles.html` - Sysctl Profiles table template
- `internal/component/web/templates/system/sysctl_profile_detail.html` - Sysctl Profile detail overlay
- `internal/component/web/templates/services/ssh.html` - SSH config form template
- `internal/component/web/templates/services/web.html` - Web config form template
- `internal/component/web/templates/services/telemetry.html` - Telemetry config form template
- `internal/component/web/templates/services/tacacs.html` - TACACS config form template
- `internal/component/web/templates/services/mcp.html` - MCP config form template
- `internal/component/web/templates/services/looking_glass.html` - Looking Glass config form template
- `internal/component/web/templates/services/api.html` - API config form template
- `internal/component/web/templates/l2tp/sessions_workbench.html` - L2TP Sessions table for workbench
- `internal/component/web/templates/l2tp/configuration.html` - L2TP Configuration form template
- `internal/component/web/templates/l2tp/health.html` - L2TP Health table template
- `test/web/system-identity.ci` - Functional test: System Identity
- `test/web/system-users.ci` - Functional test: System Users
- `test/web/system-resources.ci` - Functional test: System Resources
- `test/web/system-host-hardware.ci` - Functional test: System Host Hardware
- `test/web/service-forms.ci` - Functional test: Service config forms
- `test/web/l2tp-sessions.ci` - Functional test: L2TP Sessions
- `test/web/l2tp-config.ci` - Functional test: L2TP Configuration
- `test/web/l2tp-health.ci` - Functional test: L2TP Health
- `test/web/l2tp-compat.ci` - Functional test: L2TP backwards compatibility

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

1. **Phase 1: System Identity and Users pages** -- Config forms for hostname/router-id and user management
   - Tests: `TestWorkbenchSystemIdentity`, `TestWorkbenchSystemIdentitySave`, `TestWorkbenchSystemUsers`, `TestWorkbenchSystemUsersEmpty`, `TestWorkbenchSystemUsersAddUser`
   - Files: `handler_system.go`, `templates/system/identity.html`, `templates/system/users.html`, `templates/system/users_add.html`
   - Verify: tests fail -> implement -> tests pass

2. **Phase 2: System Resources and Host Hardware pages** -- Operational data rendering from show RPCs
   - Tests: `TestWorkbenchSystemResources`, `TestWorkbenchSystemResourcesAutoRefresh`, `TestWorkbenchSystemHostHardware`, `TestWorkbenchSystemHostHardwareUnsupported`
   - Files: `handler_system.go`, `templates/system/resources.html`, `templates/system/host_hardware.html`
   - Verify: tests fail -> implement -> tests pass

3. **Phase 3: Service configuration forms (SSH, Web, Telemetry)** -- Singleton config forms with list fields
   - Tests: `TestWorkbenchServiceSSH`, `TestWorkbenchServiceSSHSave`, `TestWorkbenchServiceSSHServerList`, `TestWorkbenchServiceWeb`, `TestWorkbenchServiceTelemetry`
   - Files: `handler_services.go`, `templates/services/ssh.html`, `templates/services/web.html`, `templates/services/telemetry.html`
   - Verify: tests fail -> implement -> tests pass

4. **Phase 4: Service configuration forms (TACACS, MCP, Looking Glass, API)** -- Remaining service forms including sensitive fields
   - Tests: `TestWorkbenchServiceTACACS`, `TestWorkbenchServiceMCP`, `TestWorkbenchServiceMCPSensitiveFields`, `TestWorkbenchServiceLookingGlass`, `TestWorkbenchServiceAPI`, `TestWorkbenchServiceFormListAddRemove`
   - Files: `handler_services.go`, `templates/services/tacacs.html`, `templates/services/mcp.html`, `templates/services/looking_glass.html`, `templates/services/api.html`
   - Verify: tests fail -> implement -> tests pass

5. **Phase 5: L2TP pages** -- Reuse/adapt existing handler_l2tp.go for workbench
   - Tests: `TestWorkbenchL2TPSessions`, `TestWorkbenchL2TPSessionsEmpty`, `TestWorkbenchL2TPConfiguration`, `TestWorkbenchL2TPConfigurationSave`, `TestWorkbenchL2TPConfigurationServerList`, `TestWorkbenchL2TPHealth`, `TestWorkbenchL2TPHealthEmpty`, `TestL2TPDisconnectPreserved`
   - Files: `handler_l2tp_workbench.go`, `handler_l2tp.go` (adapt), `templates/l2tp/sessions_workbench.html`, `templates/l2tp/configuration.html`, `templates/l2tp/health.html`
   - Verify: tests fail -> implement -> tests pass

6. **Phase 6: Sysctl Profiles page** -- Table of profiles with view/edit actions
   - Tests: `TestWorkbenchSystemSysctlProfiles`, `TestWorkbenchSystemSysctlProfilesView`
   - Files: `handler_system.go`, `templates/system/sysctl_profiles.html`, `templates/system/sysctl_profile_detail.html`
   - Verify: tests fail -> implement -> tests pass

7. **Phase 7: Integration, empty states, and full verification** -- Wire all pages into workbench nav, verify empty states, backwards compatibility
   - Tests: `TestWorkbenchAllEmptyStates`, all functional tests in `test/web/`
   - Files: `handler_workbench.go`, `workbench_sections.go`, all templates (empty state text)
   - Verify: `make ze-verify` (lint + all ze tests except fuzz)

8. **Functional tests** -> Create after feature works. Cover user-visible behavior.

9. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)

10. **Complete spec** -> Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-15 has implementation with file:line |
| Correctness | Config forms match YANG schema leaf/list topology exactly; operational pages match show RPC JSON shapes |
| Naming | Template files use snake_case; handler functions use CamelCase; URL paths use kebab-case |
| Data flow | Config saves go through EditorManager draft tree, not directly to committed tree; operational data comes from show RPCs, not duplicated logic |
| Rule: no-layering | L2TP workbench adapter wraps existing handlers, does not duplicate their logic |
| Rule: derive-not-hardcode | Service form fields derived from YANG schema, not hardcoded HTML |
| Rule: exact-or-reject | Numeric inputs validated against YANG range constraints; out-of-range values rejected with clear message |
| Sensitive fields | MCP token, TACACS key, L2TP shared-secret, TLS keys display masked values |
| Empty states | Every page has the correct empty state text from the design document |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| System Identity form handler and template | `grep -r 'SystemIdentity\|system/identity' internal/component/web/` |
| System Users table handler and template | `grep -r 'SystemUsers\|system/users' internal/component/web/` |
| System Resources handler and template | `grep -r 'SystemResources\|system/resources' internal/component/web/` |
| System Host Hardware handler and template | `grep -r 'SystemHostHardware\|host.hardware\|host-hardware' internal/component/web/` |
| System Sysctl Profiles handler and template | `grep -r 'SysctlProfiles\|sysctl.profiles\|sysctl-profiles' internal/component/web/` |
| SSH service form handler and template | `grep -r 'ServiceSSH\|services/ssh' internal/component/web/` |
| Web service form handler and template | `grep -r 'ServiceWeb\|services/web' internal/component/web/` |
| Telemetry service form handler and template | `grep -r 'ServiceTelemetry\|services/telemetry' internal/component/web/` |
| TACACS service form handler and template | `grep -r 'ServiceTACACS\|services/tacacs' internal/component/web/` |
| MCP service form handler and template | `grep -r 'ServiceMCP\|services/mcp' internal/component/web/` |
| Looking Glass service form handler and template | `grep -r 'ServiceLookingGlass\|services/looking-glass' internal/component/web/` |
| API service form handler and template | `grep -r 'ServiceAPI\|services/api' internal/component/web/` |
| L2TP Sessions workbench adapter | `grep -r 'L2TPSessions\|l2tp/sessions' internal/component/web/` |
| L2TP Configuration form handler and template | `grep -r 'L2TPConfiguration\|l2tp/configuration' internal/component/web/` |
| L2TP Health table handler and template | `grep -r 'L2TPHealth\|l2tp/health' internal/component/web/` |
| Existing L2TP endpoints still functional | `go test ./internal/component/web/ -run TestL2TPDisconnectPreserved` |
| All empty state messages present | `go test ./internal/component/web/ -run TestWorkbenchAllEmptyStates` |
| All unit tests pass | `go test ./internal/component/web/ -run TestWorkbench` |
| Feature docs updated | `grep -l 'System.*Services.*L2TP\|workbench.*system\|workbench.*services' docs/` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | All form inputs validated against YANG schema types and ranges before config tree mutation |
| Sensitive field masking | Passwords, tokens, shared secrets, TLS keys never appear in plaintext in HTML output |
| Authentication | All System/Services/L2TP workbench handlers require authenticated session (GetUsernameFromRequest check) |
| Authorization | Config mutations respect authz profile rules; user cannot edit config paths they lack permission for |
| CSRF protection | All POST endpoints (config save, L2TP disconnect, user add/delete) protected by session token |
| L2TP disconnect injection | Existing quoteForDispatch() prevents command injection via reason field; verify preserved |
| Path traversal | URL path segments validated by ValidatePathSegments before any file or config path operation |
| XSS prevention | All user-supplied values in templates escaped by Go's html/template engine; no raw HTML injection |
| Resource exhaustion | L2TP SSE streams have heartbeat timeout and context cancellation; Resources auto-refresh uses server-side HTMX poll, not open SSE |

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
| L2TP backwards compatibility broken | Re-read handler_l2tp.go; adapter must not alter existing handler signatures |
| Show RPC returns unexpected shape | Re-read system.go/host.go; adapt template to actual JSON structure |

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

Not applicable (web UI feature, not protocol work).

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

### Files Exist (ls)
<!-- For EVERY file in "Files to Create": ls -la <path> -- paste output. -->
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
<!-- For EVERY AC-N: independently verify. Do NOT copy from audit -- re-check. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
<!-- For EVERY wiring test row: does the .ci test exist AND does it exercise the full path? -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
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
