# Context Economy

**When:** spawning an agent, looking up a Go symbol, or deciding how much of a file to read
**Severity:** blocking

## Directives

**A Go symbol question MUST be answered by a symbol server before any whole-file read, and there are two routes in one order: `ToolSearch query="select:LSP"` first, then `gopls` from Bash when that answers empty.** `./le setup install` puts `gopls` on PATH, so every context reaches the capability whatever its tool registry holds, and "I have no LSP" selects the second route rather than ending the question.
**MUST locate the symbol with `gopls symbols <file>` and then ask about that position; MUST NOT guess a position, and MUST NOT read a whole file to FIND a symbol.** The operations, the position format and a worked example are `docs/contributing/navigating-the-code.md`. The `gopls mcp` server is not registered and MUST NOT be used: it holds one open file descriptor per file under the workspace root.

**A name in an agent's `tools:` field that the harness does not serve is dropped in SILENCE, so a `tools:` list MUST be probed by spawning the agent and asking it to enumerate its own registry.** A name in the MAIN thread's registry is no evidence the subagent gets it, and a definition carrying no `tools:` field inherits every schema in the registry. A new or edited definition takes effect only in the NEXT session: write it in `ai/agents/`, then run `./le ai skills-sync`.

**A generic agent such as `Explore` receives NONE of this repository's rules, so repository work MUST NOT be routed to one.** It buys tokens by making the agent ignorant of `ai/rules/evidence.md` and `ai/rules/rfc-compliance.md`. Every phase agent MUST carry a `subagent_type` from `ai/agents/`: `ze-read` for a phase that only reads, `ze-work` for one that edits.

- **A work package MUST be cut at decomposition to about 60 tool calls, and an editing agent that reaches 100 tool calls MUST stop, append its handoff to the per-spec state file, and report that the package needs a continuation.** Every API call re-feeds the whole context, a subagent starts at a 45k floor and grows by about 1.9k tokens a call, and past 250k each call reads more for one tool result than a successor's whole start costs. The continuation carries the SAME package into a fresh context: it is never a reduced scope, a stub, or a parked item (`ai/rules/completion.md`), and the handoff's four parts are in `/ze-implement`, "Phase handoff". The pretool hook counts a `ze-work` agent's Bash and Write/Edit calls and refuses the 101st, letting the calls that write the state file through (`bashCallBudget`, `internal/le/hookruntime/budget.go`).
- **The main thread MUST spawn the continuation from that handoff, and the successor MUST NOT re-derive what the handoff digests.** A digest says where to look, never what the tree holds now: the successor judges the current tree, using the digest to make that cheap.

- **A `<persisted-output>` file MUST NOT be read whole: read it with `grep` or `sed -n` for the lines the decision needs.** The harness writes that file when a command's output passes 33KB, and a whole read puts every one of those bytes into a context that is then re-fed on each later call.
- **A file MUST be read in ONE turn at the range the question needs, and MUST NOT be read in consecutive slices.** Each slice is a turn that re-feeds the whole context, so three `sed -n` slices of one file cost three contexts for one read. `gopls symbols` gives the range first; then one read of that range.
- **A rule, a page, or the spec that a brief names MUST be read once.** A phase agent re-reading a file the handoff already digests pays twice for one answer, and the digest tells it which lines to read when it needs more.
