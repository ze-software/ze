# 532 -- MCP Umbrella (Closed)

## Context

The MCP umbrella spec planned 13 handcrafted tools in 4 phases (visibility, control, intelligence, completeness) to give AI agents typed access to CLI commands. The actual implementation took a fundamentally different approach: auto-generating all MCP tools from the command registry. This made the phased handcrafted plan obsolete.

## Decisions

- Closed umbrella without implementing child specs (mcp-1 through mcp-4), because auto-generation (learned/525) superseded the entire plan.
- No audit tables filled -- the spec's ACs targeted handcrafted tools that were never built. The auto-generation approach satisfies the same user need (typed MCP access to CLI commands) through a different mechanism.

## Consequences

- All future YANG commands are automatically MCP-accessible with zero MCP code changes.
- No further MCP tool specs are needed unless adding non-command features (resources, prompts, streaming).

## Gotchas

- The umbrella was written before the auto-generation idea emerged. Specs can become obsolete when a better approach is discovered during implementation.

## Files

- `plan/learned/525-mcp-auto-tools.md` -- the actual implementation summary
