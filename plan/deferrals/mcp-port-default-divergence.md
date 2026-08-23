# Deferrals: mcp-port-default-divergence

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-07 | review round 4 of the doctor-listener work | `internal/component/mcp/yang/ze-mcp-conf.yang` declares `refine port { default 8080; }` for `environment/mcp/server`, and the daemon never applies it: `extractMCPBlock` (`internal/component/config/loader_extract.go`) passes an EMPTY default port to `extractServerList`, so an mcp server entry that omits the port yields `Port ""` and `ExtractMCPConfig` starts no listener at all. Every sibling service (`environment/web`, `environment/gnmi`, `environment/api-server`) passes a real default instead, so mcp is the only one where the schema promises a port the daemon ignores. An operator who reads the YANG and omits the port gets no MCP listener | Closing it either way is operator-visible and is not this round's subject. Making the refine reach the daemon STARTS an MCP listener on 127.0.0.1:8080 for every `mcp { enabled true }` that has none today, and MCP is an agent-facing API with its own auth modes, so that is a decision about exposure rather than about defaults. Removing the refine instead retires a promise the schema has shipped. This round fixed what it owned: the message no longer claims mcp has no default port, it names the divergence (`config.MCPMissingPortAdvice`) | owner ruling 2026-08-23: apply the default in the daemon, so mcp agrees with the schema and with every sibling service | done |

Closed on 2026-08-23. Thomas ruled that the daemon applies the default rather
than the YANG retiring it. `extractMCPBlock` passes 8080 to `extractServerList`,
`RegisterListenerDefault("mcp", ...)` is back in
`internal/component/config/listener_defaults.go`, and the `listener/mcp` row moved
from `excluded` to `covered` in the doctor dependency inventory.

The refusal the round of 2026-08-07 made visible is gone with the divergence that
caused it. `MCPServersMissingPort` and `MCPMissingPortAdvice` are deleted, and
neither `ze config validate` nor `ze doctor` reports `config-mcp-invalid` for an
entry that names no port.

An upgrade therefore starts MCP on 127.0.0.1:8080 for a deployment whose mcp block
is enabled and names no port. That deployment already asked for an MCP server, so
nothing starts that the operator did not request, but the release notes owe the
sentence.
