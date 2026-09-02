# Context Economy

**When:** spawning an agent, looking up a Go symbol, or deciding how much of a file to read
**Severity:** blocking

## Directives

**A Go symbol question MUST be answered by a symbol server before any whole-file read, and there are two routes in one order: `ToolSearch query="select:LSP"` first, then `gopls` from Bash when that answers empty.** `./le setup install` puts `gopls` on PATH, so every context reaches the capability whatever its tool registry holds, and "I have no LSP" selects the second route rather than ending the question.
**MUST locate the symbol with `gopls symbols <file>` and then ask about that position; MUST NOT guess a position, and MUST NOT read a whole file to FIND a symbol.** The operations, the position format and a worked example are `docs/contributing/navigating-the-code.md`. The `gopls mcp` server is not registered and MUST NOT be used: it holds one open file descriptor per file under the workspace root.

**A name in an agent's `tools:` field that the harness does not serve is dropped in SILENCE, so a `tools:` list MUST be probed by spawning the agent and asking it to enumerate its own registry.** A name in the MAIN thread's registry is no evidence the subagent gets it, and a definition carrying no `tools:` field inherits every schema in the registry. A new or edited definition takes effect only in the NEXT session: write it in `ai/agents/`, then run `./le ai skills-sync`.

**A generic agent such as `Explore` receives NONE of this repository's rules, so repository work MUST NOT be routed to one.** It buys tokens by making the agent ignorant of `ai/rules/evidence.md` and `ai/rules/rfc-compliance.md`. Every phase agent MUST carry a `subagent_type` from `ai/agents/`: `ze-read` for a phase that only reads, `ze-work` for one that edits.
