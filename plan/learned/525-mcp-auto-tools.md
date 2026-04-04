# Learned: MCP Auto-Generated Tools + Bearer Auth

## What was built

Auto-generation of MCP tools from the command registry, plus bearer token
authentication. When a new YANG command or plugin command is registered, it
appears as an MCP tool automatically at `tools/list` time.

## Key decisions

| Decision | Rationale |
|----------|-----------|
| Auto-generate from registry, not hardcode | User wanted tools to self-update when features are added. 13 hardcoded tools would break on every new command. |
| Group by command prefix | "rib status" + "rib routes" become `ze_rib` with action enum. "show config dump" + "show config diff" become `ze_show_config`. Depth-2 grouping when a prefix has 2+ subgroups. |
| Keep 6 handcrafted tools | `ze_announce`, `ze_withdraw` need complex parameter schemas (origin, next-hop, communities, as-path). Auto-generation can't build those. `ze_peers`, `ze_peer_control`, `ze_execute`, `ze_commands` kept for ergonomics. |
| Filter by tool name, not prefix | `handcraftedNames()` derives from `toolHandlers` map keys. Prevents duplicate tool names in `tools/list`. |
| Bearer token, not OAuth | Localhost-only service. Token is pragmatic. Full OAuth deferred. |
| `ze:sensitive` on YANG token leaf | Prevents exposure in `show config dump`. Same pattern as SSH passwords and plugin tokens. |
| CLI > env > config precedence | Reordered code to match. Env var registered with `Secret: true` (cleared from `/proc`). `ze env` masks with `****`. |
| Constant-time token comparison | `crypto/subtle.ConstantTimeCompare`. Low practical risk on localhost but trivial to implement. |
| Action enum validated server-side | `validActions` map built from command group. Prevents injection of arbitrary tokens via the action field. |

## What the dispatcher can and cannot reach

The MCP `CommandDispatcher` is `reactor.ExecuteCommand`, which goes through
the YANG-based plugin server dispatcher. It handles all YANG-registered RPCs
and plugin commands.

**Reachable:** `rib *`, `show *`, `metrics *`, `log *`, `cache *`, `peer *`,
`monitor *`, `subscribe/unsubscribe`, `commit *`, `validate *`.

**Not reachable:** `resolve dns/cymru/peeringdb/irr`, `signal reload/stop`,
`interface create/delete/addr/unit` (only `show interface` is registered).
These are top-level CLI commands, not YANG RPCs. Need dispatcher registration
to appear as MCP tools.

## Security findings from deep review

| Finding | Resolution |
|---------|-----------|
| Token exposed in config dump | `ze:sensitive` annotation added. Serializer masks sensitive leaves. |
| Token exposed in `ze env` output | `currentValue()` checks `env.IsSecret()`, returns `****` |
| Token visible in `ps aux` via `--mcp-token` | Documented env var as preferred for production |
| Community values not validated in `toolAnnounce` | Added `noSpaces()` loop for community values |
| `validActions != nil` guard redundant | Removed. Nil map lookup returns false, correctly rejects. |
| Multi-word actions rejected by `noSpaces` | Removed `noSpaces` on action. Validated against enum instead. |

## Files

| File | Purpose |
|------|---------|
| `internal/component/mcp/tools.go` | Auto-generation: CommandInfo, CommandLister, groupCommands, generateTools, buildToolDef, dispatchGenerated |
| `internal/component/mcp/handler.go` | Handler signature (dispatch, commands, token), allTools, callTool routing, bearer auth |
| `internal/component/mcp/tools_test.go` | 30 tests |
| `cmd/ze/hub/mcp.go` | commandLister wiring, startMCPServer |
| `internal/component/mcp/schema/ze-mcp-conf.yang` | Token leaf with ze:sensitive |
| `cmd/ze-test/mcp.go` | --token flag for test client |

## Deferred

- Config dump serializer does not yet read `ze:sensitive` (cross-cutting, needs own spec)
- Typed parameter schemas from YANG RPC metadata (needs YANG loader at handler level)
- Commands not in dispatcher (resolve, signal, interface mgmt)
- MCP streaming (monitor, subscribe) -- needs transport extension
- `--mcp-token-file` for production token loading
