# 525 -- MCP Auto-Generated Tools

## Context

The MCP server exposed 6 handcrafted tools while the CLI had 60+ commands. An AI agent had to use `ze_execute` (untyped escape hatch) for most operations. Adding new YANG commands required manually coding MCP tool handlers. The user wanted feature parity with automatic updates when new commands are registered.

## Decisions

- Auto-generate all MCP tools from the command registry, over hardcoding per-tool handlers. Every registered YANG command and plugin command appears as a typed MCP tool at `tools/list` time without code changes.
- Group commands by prefix (depth-1 or depth-2), over one-tool-per-command. Produces ~15 tools with action enums instead of 60+ individual tools. Depth-2 grouping triggers when a prefix has 2+ subgroups (e.g. "show" splits into "show config", "show schema").
- Remove all handcrafted tools, over keeping them alongside auto-generated ones. The typed YANG param dispatch (`key value` pairs in command string) and the existing `peer update text` parser handle announce/withdraw. `system dispatch` replaces `ze_execute`.
- Bearer token auth via CLI/env/config, over no auth or full OAuth. Constant-time comparison, `ze:sensitive` on YANG leaf, `Secret: true` on env var (cleared from `/proc`).
- Register resolve (9 RPCs) and interface management (7 RPCs) in the dispatcher, over leaving them as CLI-only. Package-level `SetResolvers()` accessor for handlers (same pattern as `iface.SetBus()`).
- `dispatchGenerated` serializes typed YANG params as `key value` pairs, over generic `arguments` string only. Unknown JSON fields (not action/arguments/peer) become command tokens.

## Consequences

- New YANG commands are MCP-accessible with zero MCP code changes.
- YANG RPC input parameters (`LeafMeta`) automatically become typed JSON Schema properties in tool definitions.
- AI agents see tool schemas with action enums + typed params instead of guessing command syntax.
- `show config dump` through MCP/dispatcher has no handler yet -- it's YANG-defined but CLI-only. Config display commands need dispatcher handlers for full MCP access.
- The `mcp-announce.ci` functional test uses `@ze_announce` which no longer exists. Needs updating.

## Gotchas

- `handcraftedSkip` was initially empty -- generated tools duplicated handcrafted names in `tools/list`. Fixed by deriving skip set from `toolHandlers` keys.
- Multi-word actions (e.g. "best status" from "rib best status") were rejected by `noSpaces()` on the action field. Fixed by validating against server-defined enum instead.
- Community values in `toolAnnounce` had no `noSpaces()` check -- bracket syntax allowed NLRI injection. Fixed.
- `validActions != nil` guard was redundant and dangerous -- nil map lookup returns false. Removed.
- `show config dump` masking is CLI-only (in `cmd_dump.go`), not in `Serialize()`. Serialize must stay plaintext for round-tripping (`config fmt -w`, `config migrate`).
- Signal/daemon commands were already registered (`daemon shutdown/reload/status/quit`). No work needed.

## Files

| File | Purpose |
|------|---------|
| `internal/component/mcp/tools.go` | Auto-generation engine: CommandInfo, ParamInfo, groupCommands, generateTools, buildToolDef, dispatchGenerated |
| `internal/component/mcp/handler.go` | Handler(dispatch, commands, token), allTools, callTool, bearer auth, no handcrafted tools |
| `internal/component/mcp/tools_test.go` | 25 tests |
| `cmd/ze/hub/mcp.go` | commandLister with YANG param extraction (sync.Once), startMCPServer |
| `internal/component/mcp/schema/ze-mcp-conf.yang` | Token leaf with ze:sensitive |
| `internal/component/resolve/cmd/` | 9 resolve RPC handlers + YANG schema |
| `internal/component/iface/cmd/manage.go` | 7 interface management RPC handlers |
| `internal/component/iface/schema/ze-iface-api.yang` | Interface management typed parameters |
