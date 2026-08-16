---
kind: table
level:
stage:
---
| Banned | Reality |
|--------|---------|
| "I will read the whole file so I have the full picture" | Grep for the symbol, then read its range |
| "One tool call at a time is safer" | Safety is dependency, not sequence: independent calls are safe in one message |
| "These two calls are related, so batch them" | Related is not independent. An Edit that consumes a Read in the same batch runs on content that was never returned |
| "My package is bigger than one agent, so I will drop the last acceptance criterion" | Scope is not yours to cut (`ai/rules/completion.md`). Report the size; the main thread re-cuts the packages |
| "Review is expensive, one lens will do" | Cost pressure never reduces the required review lenses |
| "Spawning an agent costs a round trip" | The round trip is the supervision (`ai/rules/planning.md`). Size the agent instead |
| "My context is nearly full, I will push through to the end" | Write the state file and hand off |
| "LSP is IDE navigation, grep is enough for me" | Grep matches strings; LSP resolves symbols |
| "The LSP schema loaded, so LSP works" | A loaded schema is not a running server. With `gopls` absent every call returns `ENOENT`. Verify the server once (`.claude/rules/session-start.md`) |
| "My ToolSearch came back empty, so I have no LSP here" | You have no LSP TOOL here. The capability is on PATH: run `gopls` from Bash |
| "Subagents never get LSP, so I will not try" | Which contexts carry the tool depends on the harness build and the machine, and both change. Issue the query, then fall back |
| "This phase needs findReferences, so it cannot be delegated" | `references` is answerable in a subagent by either route. Delegation is a cost decision, not a tooling one |
| "`Explore` is cheaper, so route the read-only phases to it" | It receives none of this repository's rules. Use `ze-read`, which keeps the preamble |
| "The `tools:` list is what costs, so cut tools until the agent is cheap" | The FIELD lowers the floor, not its length. Removing a tool the phase needs buys nothing and breaks the phase |
| "The name is in my tool list, so the agent I spawn will have it" | A `tools:` name the harness does not serve is dropped in SILENCE. Probe the agent and read back its own registry |
