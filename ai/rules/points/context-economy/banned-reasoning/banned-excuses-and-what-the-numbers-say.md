---
kind: table
level:
stage:
---
| Banned | Reality |
|--------|---------|
| "I will read the whole file so I have the full picture" | 54.2% of Reads named a path already read in this session. Grep for the symbol, then read its range |
| "One tool call at a time is safer" | 85.1% of measured calls carried exactly one. Safety is dependency, not sequence: independent calls are safe in one message |
| "These two calls are related, so batch them" | Related is not independent. An Edit that consumes a Read in the same batch runs on content that was never returned |
| "My package is bigger than one agent, so I will drop the last acceptance criterion" | Scope is not yours to cut (`ai/rules/completion.md`). Report the size; the main thread re-cuts the packages |
| "Review is expensive, one lens will do" | Review is 15.4% of subagent context; the phase it prevents is 24.5%. Cost pressure never applies here |
| "Spawning an agent costs a round trip" | The round trip is the supervision (`ai/rules/planning.md`). Size the agent instead |
| "My context is nearly full, I will push through to the end" | 49.5% of main-thread context was fed above 600k against a 1M ceiling. Write the state file and hand off |
| "LSP is IDE navigation, grep is enough for me" | `documentSymbol` cost 360 tokens where the file cost 12,267 (34.0x). Grep matches strings; LSP resolves symbols |
| "The LSP schema loaded, so LSP works" | A loaded schema is not a running server. With `gopls` absent every call returns `ENOENT`. Verify the server once (`.claude/rules/session-start.md`) |
| "My ToolSearch came back empty, so I have no LSP here" | You have no LSP TOOL here. The capability is on PATH: run `gopls` from Bash. `gopls symbols` cost 1,297 bytes where the file cost 44,164 (34.1x) |
| "Subagents never get LSP, so I will not try" | Which contexts carry the tool depends on the harness build and the machine, and both change. Issue the query, then fall back |
| "This phase needs findReferences, so it cannot be delegated" | `references` is answerable in a subagent by either route. Delegation is a cost decision, not a tooling one |
