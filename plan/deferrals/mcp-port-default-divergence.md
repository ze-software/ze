# Deferrals: mcp-port-default-divergence

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-07 | review round 4 of the doctor-listener work | `internal/component/mcp/yang/ze-mcp-conf.yang` declares `refine port { default 8080; }` for `environment/mcp/server`, and the daemon never applies it: `extractMCPBlock` (`internal/component/config/loader_extract.go`) passes an EMPTY default port to `extractServerList`, so an mcp server entry that omits the port yields `Port ""` and `ExtractMCPConfig` starts no listener at all. Every sibling service (`environment/web`, `environment/gnmi`, `environment/api-server`) passes a real default instead, so mcp is the only one where the schema promises a port the daemon ignores. An operator who reads the YANG and omits the port gets no MCP listener | Closing it either way is operator-visible and is not this round's subject. Making the refine reach the daemon STARTS an MCP listener on 127.0.0.1:8080 for every `mcp { enabled true }` that has none today, and MCP is an agent-facing API with its own auth modes, so that is a decision about exposure rather than about defaults. Removing the refine instead retires a promise the schema has shipped. This round fixed what it owned: the message no longer claims mcp has no default port, it names the divergence (`config.MCPMissingPortAdvice`) | needs an owner ruling; no spec owns `environment/mcp` defaults today | open |

The round that found this made the refusal visible rather than silent:
`ExtractMCPConfig` now requires EVERY server entry to name a port (it tested only
`Servers[0]`, so a second entry reached the binder as `"<ip>:"` and the kernel
chose the port), and `MCPServersMissingPort` reports it through
`config-mcp-invalid` on both `ze config validate` and `ze doctor`. So an operator
who hits the divergence is now told what to write. What remains open is only which
side of the divergence should move.

Whoever takes it should decide both halves together: if the refine is made to
reach the daemon, `RegisterListenerDefault("mcp", "127.0.0.1", "8080")` belongs
back in `internal/component/config/listener_defaults.go` and the `listener/mcp`
row moves from `excluded` to `covered` in the doctor dependency inventory, because
the empty-list config would then start a listener that needs probing.
