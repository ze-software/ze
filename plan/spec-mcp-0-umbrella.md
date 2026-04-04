# Spec: mcp-0 -- MCP Feature Parity (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/mcp/handler.go` -- current MCP handler (6 tools)
4. `docs/guide/mcp/overview.md` -- MCP user guide
5. Child specs: `spec-mcp-1-*` through `spec-mcp-4-*`

## Task

The MCP server exposes 6 structured tools while the CLI has 60+ commands across BGP, config,
interface, resolution, schema, metrics, logging, cache, data store, and environment management.
The `ze_execute` escape hatch gives raw access to everything, but without typed parameters,
validation, or discoverability -- an AI agent has to guess command syntax.

Add 13 structured MCP tools (bringing the total to 19) to give AI agents typed, discoverable
access to every major CLI capability. Each tool follows the established pattern: parse JSON
params, validate inputs, build command string, dispatch.

### Gap Analysis

**Currently covered (6 tools):**

| Tool | CLI Equivalent |
|------|---------------|
| `ze_announce` | `peer * update text ... nlri add` |
| `ze_withdraw` | `peer * update text ... nlri del` |
| `ze_peers` | `peer list`, `show bgp peer` |
| `ze_peer_control` | `peer X teardown/pause/resume/flush` |
| `ze_execute` | Any command (unstructured) |
| `ze_commands` | `command-list` |

**Not covered (no structured tool):**

| Category | CLI Commands | AI Impact |
|----------|-------------|-----------|
| RIB queries | `rib status`, `rib routes`, `rib best status` | High -- most common operational task |
| RIB mutations | `rib clear in/out`, `rib inject`, `rib withdraw` | High -- route manipulation |
| Config read | `config dump/diff/validate/history/fmt/ls/cat` | High -- config awareness |
| Config write | `config set`, `config rollback` | High -- config management |
| Route refresh | `peer refresh/borr/eorr`, `peer clear soft` | Medium -- operational |
| Resolution | `resolve dns/cymru/peeringdb/irr` | High -- network intelligence |
| Interface | `interface show/create/delete`, addr/unit mgmt | Medium -- infrastructure |
| Schema | `schema list/methods/events/handlers/protocol` | Medium -- self-discovery |
| Metrics | `metrics values/list` | High -- observability |
| Logging | `log levels/set` | Medium -- debugging |
| Daemon control | `signal reload/stop/restart/status` | Medium -- lifecycle |
| Cache | `cache list/forward` | Low -- specialized |
| Data store | `data ls/cat/registered` | Low -- specialized |
| Environment | `env list/get/registered` | Low -- debugging |
| Monitoring | `monitor bgp`, `monitor event` | N/A -- streaming |
| Subscriptions | `subscribe/unsubscribe` | N/A -- streaming |

### Design Decisions

| Decision | Detail |
|----------|--------|
| 13 new tools, 4 phases | Phased by AI agent value: visibility first, then control, intelligence, completeness |
| One tool per domain | Group related actions under one tool with an `action` parameter when schemas overlap |
| Read/write split | Separate tools for queries vs mutations (ze_config vs ze_config_edit, ze_rib vs ze_rib_control) |
| Same dispatch pattern | All tools build command strings for the existing `CommandDispatcher` -- no backend changes |
| Streaming out of scope | `monitor` and `subscribe` require protocol-level streaming (SSE/long-poll), not request-response. Deferred to a future MCP transport spec |
| Interactive out of scope | `config edit` is an interactive session, not a single tool call. Use `ze_config_edit` for atomic set/rollback |
| handler.go stays one file | All tools share the same server struct and dispatch function. Split only if the file exceeds 600 lines (likely after phase 2) |
| Input validation | Same `noSpaces()` pattern for all string params that become command tokens. Enum validation for action params |

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| 13 new MCP tools | Typed JSON schemas, input validation, command building, dispatch |
| Tool definitions | JSON Schema for each tool's inputSchema (parameters, types, enums, descriptions) |
| Unit tests | Test each handler: valid input, missing required params, invalid enums, whitespace injection |
| Functional tests | `.ci` test per phase proving tools work through the MCP HTTP endpoint |
| Documentation | Update `docs/guide/mcp/overview.md` with new tool reference tables |
| AI help reference | Update `cmd/ze/help_ai.go` `printMCPTools()` if manually maintained |
| File splitting | Split `handler.go` if it exceeds 600 lines |

**Out of Scope:**

| Area | Reason |
|------|--------|
| Streaming (monitor, subscribe) | Requires MCP transport extension (SSE or long-poll). Separate spec |
| Interactive config editing | `config edit` is a TUI session, not a tool call |
| New dispatcher commands | All tools wrap existing commands. No backend changes |
| MCP resources | Read-only browsable data (future enhancement, separate spec) |
| MCP prompts | Canned workflows (future enhancement, separate spec) |
| Authentication | Already local-only (127.0.0.1). Auth is a separate concern |

### Child Specs

| Phase | Spec | Tools | Depends |
|-------|------|-------|---------|
| 1 | `spec-mcp-1-visibility.md` | `ze_rib`, `ze_config`, `ze_schema`, `ze_metrics`, `ze_log` | - |
| 2 | `spec-mcp-2-control.md` | `ze_rib_control`, `ze_route_refresh`, `ze_config_edit` | mcp-1 |
| 3 | `spec-mcp-3-intelligence.md` | `ze_resolve`, `ze_interface` | - |
| 4 | `spec-mcp-4-completeness.md` | `ze_signal`, `ze_cache`, `ze_data`, `ze_env` | - |

Phases 1 and 2 are ordered (mutations depend on read tools for testing). Phases 3 and 4 are independent and can be implemented in any order after phase 1.

## Tool Definitions

### Phase 1 -- Operational Visibility

#### ze_rib

Query the BGP RIB for routes and status.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `status`, `routes`, or `best-status` |
| `scope` | string | No | Route scope: `received`, `sent`, or `best` (for `routes` action) |
| `peer` | string | No | Peer selector: address, name, or `*` |
| `family` | string | No | Address family filter (e.g. `ipv4/unicast`) |
| `prefix` | string | No | Prefix filter (e.g. `10.0.0.0/24`) |

Command mapping:

| Action + Scope | Command |
|---------------|---------|
| `status` | `rib status` |
| `routes` + `received` | `rib routes received [peer P] [family F] [prefix X]` |
| `routes` + `sent` | `rib routes sent [peer P] [family F] [prefix X]` |
| `routes` + `best` | `rib routes best [family F] [prefix X]` |
| `best-status` | `rib best status` |

#### ze_config

Read-only configuration operations.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `dump`, `diff`, `validate`, `history`, `fmt`, `ls`, or `cat` |
| `path` | string | No | Config path (for `cat`) or file path (for `validate`, `fmt`) |
| `rev1` | string | No | First revision (for `diff`) |
| `rev2` | string | No | Second revision (for `diff`) |
| `entries` | integer | No | Number of history entries to show |

Command mapping:

| Action | Command |
|--------|---------|
| `dump` | `show config dump` |
| `diff` | `show config diff [rev1] [rev2]` |
| `validate` | `validate config [path]` |
| `history` | `show config history [entries]` |
| `fmt` | `show config fmt [path]` |
| `ls` | `show config ls` |
| `cat` | `show config cat <path>` |

#### ze_schema

Schema discovery and introspection.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `list`, `methods`, `events`, `handlers`, or `protocol` |

Command mapping:

| Action | Command |
|--------|---------|
| `list` | `show schema list` |
| `methods` | `show schema methods` |
| `events` | `show schema events` |
| `handlers` | `show schema handlers` |
| `protocol` | `show schema protocol` |

#### ze_metrics

Prometheus metrics access.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `values` or `list` |

Command mapping:

| Action | Command |
|--------|---------|
| `values` | `metrics values` |
| `list` | `metrics list` |

#### ze_log

Log level management.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `levels` or `set` |
| `subsystem` | string | No | Subsystem name (for `set`) |
| `level` | string | No | Log level: `disabled`, `debug`, `info`, `warn`, `err` (for `set`) |

Command mapping:

| Action | Command |
|--------|---------|
| `levels` | `log levels` |
| `set` | `log set <subsystem> <level>` |

### Phase 2 -- Operational Control

#### ze_rib_control

RIB mutation operations.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `clear-in`, `clear-out`, `inject`, or `withdraw` |
| `peer` | string | No | Peer selector (for `clear-in`, `clear-out`) |
| `family` | string | No | Address family |
| `prefix` | string | No | Prefix (for `inject`, `withdraw`) |
| `next-hop` | string | No | Next-hop IP (for `inject`) |
| `origin` | string | No | Origin attribute (for `inject`) |

Command mapping:

| Action | Command |
|--------|---------|
| `clear-in` | `rib clear in [peer P]` |
| `clear-out` | `rib clear out [peer P]` |
| `inject` | `rib inject <family> <prefix> [next-hop H] [origin O]` |
| `withdraw` | `rib withdraw <family> <prefix>` |

#### ze_route_refresh

BGP route refresh operations (RFC 2918).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `peer` | string | Yes | Peer selector |
| `action` | string | Yes | `refresh`, `borr`, `eorr`, or `soft-clear` |
| `family` | string | No | Address family (for `refresh`, `borr`, `eorr`) |

Command mapping:

| Action | Command |
|--------|---------|
| `refresh` | `peer <P> refresh [family]` |
| `borr` | `peer <P> borr [family]` |
| `eorr` | `peer <P> eorr [family]` |
| `soft-clear` | `peer <P> clear soft` |

#### ze_config_edit

Configuration modification operations.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `set` or `rollback` |
| `path` | string | No | Config path (for `set`) |
| `value` | string | No | Config value (for `set`) |
| `revision` | string | No | Revision to rollback to (for `rollback`) |

Command mapping:

| Action | Command |
|--------|---------|
| `set` | `config set <path> <value>` |
| `rollback` | `config rollback [revision]` |

### Phase 3 -- Network Intelligence

#### ze_resolve

Network resolution services.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `service` | string | Yes | `dns`, `cymru`, `peeringdb`, or `irr` |
| `operation` | string | Yes | Service-specific operation (see below) |
| `query` | string | Yes | Query value (domain, ASN, prefix, AS-SET name) |

DNS operations: `a`, `aaaa`, `txt`, `ptr`
Cymru operations: `asn-name`
PeeringDB operations: `prefix-count`, `as-set`
IRR operations: `expand`, `prefix`

Command mapping:

| Service + Operation | Command |
|--------------------|---------|
| `dns` + `a` | `resolve dns a <query>` |
| `dns` + `aaaa` | `resolve dns aaaa <query>` |
| `dns` + `txt` | `resolve dns txt <query>` |
| `dns` + `ptr` | `resolve dns ptr <query>` |
| `cymru` + `asn-name` | `resolve cymru asn-name <query>` |
| `peeringdb` + `prefix-count` | `resolve peeringdb prefix-count <query>` |
| `peeringdb` + `as-set` | `resolve peeringdb as-set <query>` |
| `irr` + `expand` | `resolve irr expand <query>` |
| `irr` + `prefix` | `resolve irr prefix <query>` |

#### ze_interface

OS network interface management.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `show`, `create-dummy`, `create-veth`, `delete`, `addr-add`, `addr-del`, `unit-add`, `unit-del` |
| `name` | string | No | Interface name |
| `peer-name` | string | No | Peer interface name (for `create-veth`) |
| `unit` | integer | No | Logical unit ID |
| `vlan-id` | integer | No | VLAN ID (for `unit-add`) |
| `address` | string | No | IP address in CIDR notation (for `addr-add`, `addr-del`) |

Command mapping:

| Action | Command |
|--------|---------|
| `show` | `interface show [name]` |
| `create-dummy` | `interface create dummy <name>` |
| `create-veth` | `interface create veth <name> <peer-name>` |
| `delete` | `interface delete <name>` |
| `addr-add` | `interface addr add <name> unit <unit> <address>` |
| `addr-del` | `interface addr del <name> unit <unit> <address>` |
| `unit-add` | `interface unit add <name> <unit> [vlan-id <vlan-id>]` |
| `unit-del` | `interface unit del <name> <unit>` |

### Phase 4 -- Completeness

#### ze_signal

Daemon lifecycle control.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `reload`, `stop`, `restart`, `status` |
| `host` | string | No | Daemon host (default: from config) |
| `port` | integer | No | Daemon SSH port (default: from config) |

Command mapping:

| Action | Command |
|--------|---------|
| `reload` | `signal reload [--host H] [--port P]` |
| `stop` | `signal stop [--host H] [--port P]` |
| `restart` | `signal restart [--host H] [--port P]` |
| `status` | `signal status [--host H] [--port P]` |

#### ze_cache

BGP message cache operations.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `list` or `forward` |

Command mapping:

| Action | Command |
|--------|---------|
| `list` | `cache list` |
| `forward` | `cache forward` |

#### ze_data

Blob store inspection.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `ls`, `cat`, or `registered` |
| `key` | string | No | Entry key (for `cat`) or glob pattern (for `ls`) |

Command mapping:

| Action | Command |
|--------|---------|
| `ls` | `show data ls [key]` |
| `cat` | `show data cat <key>` |
| `registered` | `show data registered` |

#### ze_env

Environment variable inspection.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | `list`, `get`, or `registered` |
| `name` | string | No | Variable name (for `get`) |

Command mapping:

| Action | Command |
|--------|---------|
| `list` | `show env list` |
| `get` | `show env get <name>` |
| `registered` | `show env registered` |

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/mcp/handler.go` -- MCP HTTP handler with 6 tools
  -> Constraint: all tools build command strings and call `s.dispatch()`
- [ ] `internal/component/mcp/handler_test.go` -- existing test patterns
- [ ] `cmd/ze-test/mcp.go` -- MCP test client, `@tool_name` syntax for typed calls
- [ ] `test/plugin/mcp-announce.ci` -- functional test: announce via MCP, verify BGP UPDATE

**Behavior to preserve:**
- All 6 existing tools unchanged (ze_announce, ze_withdraw, ze_peers, ze_peer_control, ze_execute, ze_commands)
- JSON-RPC 2.0 protocol envelope (initialize, tools/list, tools/call)
- Content-Type validation (CSRF protection)
- 1 MB request body limit
- `noSpaces()` input validation pattern
- Error response format (`isError: true`, `"Error: "` prefix)
- Protocol version `2024-11-05`

**Behavior to change:**
- `tools/list` returns 19 tools instead of 6
- `handler.go` split into multiple files once it exceeds 600 lines
- `ze help --ai --mcp` output includes new tools

## Data Flow (MANDATORY)

### Entry Point
- HTTP POST with JSON-RPC body to MCP endpoint (127.0.0.1:port)
- Format: `{"jsonrpc":"2.0","id":N,"method":"tools/call","params":{"name":"ze_rib","arguments":{...}}}`

### Transformation Path
1. HTTP handler validates method, content-type, body size
2. JSON-RPC request unmarshaled into `request` struct
3. `tools/call` handler looks up tool name in `toolHandlers` map
4. Tool handler unmarshals `arguments` into tool-specific param struct
5. Handler validates required fields, enums, whitespace
6. Handler builds command string from validated params
7. `s.dispatch(command)` sends to reactor's `ExecuteCommand`
8. Text result wrapped in MCP content format and returned

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| HTTP -> MCP handler | JSON-RPC POST, Content-Type validated | [ ] |
| MCP handler -> dispatcher | Command string via `CommandDispatcher` func | [ ] |
| Dispatcher -> reactor | Same function reference passed at MCP server creation | [ ] |

### Integration Points
- `CommandDispatcher` -- existing interface, no changes needed
- `toolHandlers` map -- add new tool handlers
- `tools` slice -- add new tool JSON schema definitions
- `ze-test mcp` client -- already supports `@tool_name` syntax for any tool

### Architectural Verification
- [ ] No bypassed layers (all tools go through CommandDispatcher)
- [ ] No unintended coupling (MCP has no direct reactor access)
- [ ] No duplicated functionality (tools wrap existing CLI commands)
- [ ] Zero-copy preserved (not applicable -- JSON-RPC is text-based)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `@ze_rib {"action":"status"}` via MCP HTTP | -> | `toolRib` dispatches `rib status` | `test/plugin/mcp-rib.ci` |
| `@ze_config {"action":"dump"}` via MCP HTTP | -> | `toolConfig` dispatches `show config dump` | `test/plugin/mcp-config.ci` |
| `@ze_resolve {"service":"dns","operation":"a","query":"example.com"}` via MCP HTTP | -> | `toolResolve` dispatches `resolve dns a example.com` | `test/plugin/mcp-resolve.ci` |
| `@ze_metrics {"action":"list"}` via MCP HTTP | -> | `toolMetrics` dispatches `metrics list` | `test/plugin/mcp-metrics.ci` |

## Implementation Pattern

Every new tool follows the same pattern established by the existing 6 tools:

1. Define a struct for the tool's JSON parameters
2. Unmarshal `json.RawMessage` args into the struct
3. Validate required fields and enums
4. Call `noSpaces()` on all string params that become command tokens
5. Build the command string with `strings.Builder` or `fmt.Sprintf`
6. Call `s.run(command)` and return the result
7. Add the handler to `toolHandlers` map
8. Add the JSON schema to `tools` slice

Each handler is approximately 30-50 lines. No changes to the dispatcher, reactor, or any backend code.

## File Impact Estimate

| Phase | handler.go growth | Split needed? |
|-------|-------------------|---------------|
| Phase 1 (5 tools) | ~250 lines | Likely (total ~670) -- split tool handlers into `handler_tools.go` |
| Phase 2 (3 tools) | ~180 lines | Already split |
| Phase 3 (2 tools) | ~150 lines | No |
| Phase 4 (4 tools) | ~180 lines | Review at completion |

## Testing Strategy

| Level | What | How |
|-------|------|-----|
| Unit | Each handler: valid input, missing required, invalid enum, whitespace injection | `handler_test.go` table-driven tests |
| Integration | Tool dispatch through HTTP handler | `TestCallTool` with mock dispatcher |
| Functional | Tool works against running daemon | `.ci` test per phase with `ze-test mcp` |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/mcp/overview.md` -- current MCP documentation
  -> Constraint: tool naming uses `ze_` prefix, camelCase for MCP protocol fields
- [ ] `docs/architecture/core-design.md` -- overall architecture
  -> Constraint: MCP wraps CommandDispatcher, no direct reactor access

### Source Files
- [ ] `internal/component/mcp/handler.go` -- current 6-tool implementation
  -> Constraint: established pattern for tool handlers, validation, dispatch
- [ ] `internal/component/mcp/handler_test.go` -- existing test patterns
- [ ] `cmd/ze-test/mcp.go` -- MCP test client (@tool_name syntax)
- [ ] `test/plugin/mcp-announce.ci` -- existing functional test pattern
- [ ] `cmd/ze/help_ai.go` -- AI help reference generation

**Key insights:**
- All MCP tools are thin command-string builders over the existing dispatcher
- No backend changes needed -- every CLI command is already dispatchable
- The `ze_execute` escape hatch already provides full access; structured tools add schema and validation
- Each tool handler is 30-50 lines following an established pattern

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `tools/list` after all phases | Returns 19 tools with complete JSON schemas |
| AC-2 | `@ze_rib {"action":"status"}` with running daemon | Returns RIB summary text |
| AC-3 | `@ze_rib {"action":"routes","scope":"received"}` | Returns received routes |
| AC-4 | `@ze_config {"action":"dump"}` | Returns current config text |
| AC-5 | `@ze_config {"action":"validate"}` | Returns validation result |
| AC-6 | `@ze_schema {"action":"methods"}` | Returns registered RPC methods |
| AC-7 | `@ze_metrics {"action":"values"}` | Returns Prometheus-format metrics |
| AC-8 | `@ze_log {"action":"set","subsystem":"bgp","level":"debug"}` | Changes log level, returns confirmation |
| AC-9 | `@ze_rib_control {"action":"clear-in"}` | Clears Adj-RIB-In, returns confirmation |
| AC-10 | `@ze_route_refresh {"peer":"*","action":"refresh"}` | Sends ROUTE-REFRESH to all peers |
| AC-11 | `@ze_config_edit {"action":"set","path":"bgp.peer.10.0.0.1.local-as","value":"65000"}` | Sets config value |
| AC-12 | `@ze_resolve {"service":"dns","operation":"a","query":"example.com"}` | Returns DNS A records |
| AC-13 | `@ze_interface {"action":"show"}` | Returns interface list |
| AC-14 | `@ze_signal {"action":"status"}` | Returns daemon status |
| AC-15 | `@ze_cache {"action":"list"}` | Returns cache contents |
| AC-16 | `@ze_data {"action":"registered"}` | Returns registered key patterns |
| AC-17 | `@ze_env {"action":"list"}` | Returns registered env vars |
| AC-18 | Any tool with whitespace in string param | Returns error, command not dispatched |
| AC-19 | Any tool with unknown action enum | Returns error with valid options listed |
| AC-20 | Existing tools unchanged | `ze_announce`, `ze_withdraw` etc. work identically |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestToolRib` | `internal/component/mcp/handler_test.go` | RIB tool: valid actions, filters, missing action | |
| `TestToolConfig` | `internal/component/mcp/handler_test.go` | Config tool: all 7 actions, path param | |
| `TestToolSchema` | `internal/component/mcp/handler_test.go` | Schema tool: all 5 actions | |
| `TestToolMetrics` | `internal/component/mcp/handler_test.go` | Metrics tool: values/list | |
| `TestToolLog` | `internal/component/mcp/handler_test.go` | Log tool: levels read, set with validation | |
| `TestToolRibControl` | `internal/component/mcp/handler_test.go` | RIB control: all 4 actions, required params | |
| `TestToolRouteRefresh` | `internal/component/mcp/handler_test.go` | Route refresh: all 4 actions, peer required | |
| `TestToolConfigEdit` | `internal/component/mcp/handler_test.go` | Config edit: set/rollback, required params | |
| `TestToolResolve` | `internal/component/mcp/handler_test.go` | Resolve: service+operation combinations | |
| `TestToolInterface` | `internal/component/mcp/handler_test.go` | Interface: all 8 actions, required params per action | |
| `TestToolSignal` | `internal/component/mcp/handler_test.go` | Signal: all 4 actions | |
| `TestToolCache` | `internal/component/mcp/handler_test.go` | Cache: list/forward | |
| `TestToolData` | `internal/component/mcp/handler_test.go` | Data: ls/cat/registered, key required for cat | |
| `TestToolEnv` | `internal/component/mcp/handler_test.go` | Env: list/get/registered, name required for get | |
| `TestToolListAll` | `internal/component/mcp/handler_test.go` | tools/list returns all 19 tools | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ze_config` `entries` | 1+ | 1 | 0 | N/A (int) |
| `ze_interface` `unit` | 0+ | 0 | -1 | N/A (int) |
| `ze_interface` `vlan-id` | 1-4094 | 4094 | 0 | 4095 |
| `ze_signal` `port` | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mcp-rib` | `test/plugin/mcp-rib.ci` | RIB status via MCP after peer established | |
| `mcp-config` | `test/plugin/mcp-config.ci` | Config dump via MCP | |
| `mcp-resolve` | `test/plugin/mcp-resolve.ci` | DNS resolution via MCP | |
| `mcp-tools-list` | `test/plugin/mcp-tools-list.ci` | tools/list returns 19 tools | |

### Future (if deferring any tests)
- Streaming/monitoring tests deferred -- requires MCP transport extension (user-approved scope exclusion)

## Files to Modify

- `internal/component/mcp/handler.go` -- add tool handlers and schemas (split when >600 lines)
- `internal/component/mcp/handler_test.go` -- unit tests for all new tools
- `docs/guide/mcp/overview.md` -- tool reference documentation
- `docs/features.md` -- update MCP tool count
- `cmd/ze/help_ai.go` -- update AI help output (if manually maintained)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A -- MCP tools wrap existing RPCs |
| CLI commands/flags | No | N/A -- no new CLI commands |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/plugin/mcp-rib.ci`, `mcp-config.ci`, `mcp-resolve.ci`, `mcp-tools-list.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- update MCP tool count |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | Yes | `docs/guide/mcp/overview.md` -- 13 new tool reference tables |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/overview.md` -- already exists, update |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |

## Files to Create

- `internal/component/mcp/handler_tools.go` -- tool handlers split from handler.go (phase 1)
- `internal/component/mcp/handler_tools_test.go` -- tests for new tools (if test file split needed)
- `test/plugin/mcp-rib.ci` -- functional test for RIB tools
- `test/plugin/mcp-config.ci` -- functional test for config tools
- `test/plugin/mcp-resolve.ci` -- functional test for resolve tools
- `test/plugin/mcp-tools-list.ci` -- functional test for tools/list completeness

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + child spec for current phase |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase 1: Operational Visibility** -- `ze_rib`, `ze_config`, `ze_schema`, `ze_metrics`, `ze_log`
   - Tests: `TestToolRib`, `TestToolConfig`, `TestToolSchema`, `TestToolMetrics`, `TestToolLog`
   - Files: `handler.go` (split to `handler_tools.go`), `handler_test.go`
   - Verify: tests fail -> implement -> tests pass
   - Functional: `test/plugin/mcp-rib.ci`, `test/plugin/mcp-config.ci`

2. **Phase 2: Operational Control** -- `ze_rib_control`, `ze_route_refresh`, `ze_config_edit`
   - Tests: `TestToolRibControl`, `TestToolRouteRefresh`, `TestToolConfigEdit`
   - Files: `handler_tools.go`, `handler_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase 3: Network Intelligence** -- `ze_resolve`, `ze_interface`
   - Tests: `TestToolResolve`, `TestToolInterface`
   - Files: `handler_tools.go`, `handler_test.go`
   - Verify: tests fail -> implement -> tests pass
   - Functional: `test/plugin/mcp-resolve.ci`

4. **Phase 4: Completeness** -- `ze_signal`, `ze_cache`, `ze_data`, `ze_env`
   - Tests: `TestToolSignal`, `TestToolCache`, `TestToolData`, `TestToolEnv`, `TestToolListAll`
   - Files: `handler_tools.go`, `handler_test.go`
   - Verify: tests fail -> implement -> tests pass
   - Functional: `test/plugin/mcp-tools-list.ci`

5. **Documentation** -- update `docs/guide/mcp/overview.md`, `docs/features.md`

6. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a tool handler and test |
| Correctness | Command strings match actual CLI dispatch syntax |
| Naming | Tool names use `ze_` prefix, params use kebab-case JSON keys |
| Data flow | All tools go through `s.dispatch()`, no direct reactor calls |
| Rule: no-layering | No duplicate tool names or overlapping functionality |
| Rule: input validation | Every string param calls `noSpaces()`, every enum validated |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| 13 new tool handlers | grep `toolHandlers` in handler files |
| 13 new tool schemas in `tools` slice | grep `"name":` in handler files, count 19 |
| Unit tests for all 13 tools | grep `TestTool` in test files |
| Functional .ci tests | `ls test/plugin/mcp-*.ci` |
| Updated MCP docs | grep new tool names in `docs/guide/mcp/overview.md` |
| Updated features.md | grep "MCP" or tool count in `docs/features.md` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | All string params validated with `noSpaces()` to prevent command injection |
| Enum validation | All `action`/`service`/`operation` params checked against allow-list |
| Required params | Missing required params return error before dispatch |
| Body size | 1 MB limit preserved |
| Binding | Still 127.0.0.1 only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check command string syntax against actual CLI |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

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

## RFC Documentation

Not applicable -- MCP is not an RFC-defined protocol. Tool definitions follow the MCP specification at modelcontextprotocol.io.

## Implementation Summary

### What Was Implemented
- (to be filled per phase)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- [ ] AC-1..AC-20 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
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
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-mcp-0-umbrella.md`
- [ ] Summary included in commit
