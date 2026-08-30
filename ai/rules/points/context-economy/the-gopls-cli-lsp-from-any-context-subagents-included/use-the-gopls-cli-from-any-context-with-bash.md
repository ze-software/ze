---
kind: directive
level: MUST
stage:
---
- **`gopls` is on PATH (`requiredTools`, `internal/le/setup/tools.go`), so ANY context with Bash reaches the capability, whatever its tool registry holds.** This is the fall-back route of the two above, and it needs no session restart. A context whose `ToolSearch` came back empty MUST run the command instead. It MUST NOT read a whole file to hunt for a symbol, and it MUST NOT report back that it could not look.
- **MUST find the symbol with `gopls symbols <file>` first, then ask about that position. MUST NOT guess a position.** The operations, the position format, the cost, and a worked example are in `docs/contributing/navigating-the-code.md`.
- **MUST batch `gopls` calls like any other Bash call.** Several independent questions belong in ONE message.
- **The `gopls mcp` server is not registered and MUST NOT be used.** It holds one open file descriptor per file under the workspace root. Use the LSP tool or the command line.
