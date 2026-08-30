---
kind: directive
level: MUST
stage:
---
**A Go symbol question MUST be answered by a symbol server before any whole-file read, and there are two routes in one order: `ToolSearch query="select:LSP"` first, then `gopls` from Bash when that answers empty.** `./le setup` puts `gopls` on PATH, so every context reaches the capability whatever its tool registry holds, and "I have no LSP" selects the second route rather than ending the question.
**MUST locate the symbol with `gopls symbols <file>` and then ask about that position; MUST NOT guess a position, and MUST NOT read a whole file to FIND a symbol.** The operations, the position format and a worked example are `docs/contributing/navigating-the-code.md`. The `gopls mcp` server is not registered and MUST NOT be used: it holds one open file descriptor per file under the workspace root.
