# Context Economy

**When:** spawning an agent, batching tool calls, looking up a symbol, reading a source file, or running a shell command in a long session
**Severity:** blocking
**Related:** planning, completion, evidence

## Directives

Every figure here is reproducible from a named command, and there are two of them. A figure about this machine's session transcripts is printed by `make ze-token-economy`, which reads the local Claude Code transcript store. A figure about a FILE -- the `gopls` byte and token counts, the command timings -- comes from running the named command in this checkout; token counts there are characters / 3.6, the approximation `scripts/dev/token_economy.py` uses. Nothing here is estimated, and a figure that no command reproduces does not belong in this rule.

The store grows with every session, so the RATIOS are what the directives rest on and every absolute is a reading at its date. Reading of 2026-08-05: 40,062 API calls over 32 sessions and 472 subagents, 12.4B tokens of context fed of which 228M were distinct. Re-run the target for current numbers rather than trusting these.

- **Cost per API call is the context size at that call, so the bill is round trips times context and nothing else moves it much.** Every choice below lowers one of those two terms.
- **Trimming tool OUTPUT lowers neither term, so do not spend effort there.** Bash results average 436 tokens in the measured corpus; a shorter command output changes no number a session pays for.
- **Batch independent tool calls into ONE message. 85.1% of measured API calls carried exactly one tool call.** Two Reads, a Read and a Grep, or three Bash checks that do not consume each other's results belong in one message.
- **The precondition is INDEPENDENCE, not relatedness.** A Read and an Edit that consumes that Read's result are dependent and must never share a batch: the Edit is composed before the Read returns, so it lands on content nobody has seen. When in doubt about a dependency, split the batch.
- **LSP FIRST for any symbol question: before Grep, before Read.** Measured with `gopls documentSymbol` against reading the whole file: `internal/component/bgp/reactor/peer.go` 1,482 tokens against 13,475 (9.0x), `internal/component/ike/engine/fsm.go` 360 against 12,267 (34.0x), `internal/component/bgp/reactor/session_prefix.go` 626 against 6,498 (10.3x).
- **Reading a whole file to FIND a symbol is the anti-pattern.** The file is the container; the symbol is the question. LSP answers the question, and every other token in that file is paid for on every later call in the session.
- **Read a RANGE, not a whole file, above 500 lines. Resolve the symbol with LSP first, or with `grep -n` / `sed -n` where LSP is absent, then Read the range it names.** 54.2% of measured Read calls named a path already read within one SESSION, which is a main thread and every agent under it; 25.2% named one already read within one THREAD, which is a single context window. This directive rests on the session figure, because a path a later agent re-reads is paid for in full again.
- **Do not re-read what a digest already records.** `tmp/session/session-state-<spec-stem>-<SID>.md` holds the per-spec digest, and `_find_latest_state_for_spec` (`.claude/hooks/lib/state-file.sh`) resolves the newest one for a spec across sessions. Read the digest first, and re-read the source only when the digest lacks the detail your claim depends on.
- **A digest is not evidence.** When you are about to state what code does, read the producing function (`ai/rules/evidence.md`). This rule lowers the cost of reading; it never lowers the standard of proof.
- **Size a work package so ONE agent finishes it.** Measured implementation agents ran 144 API calls each at 294k mean context, the highest of any phase on both counts: cost inside one agent grows with turns, because its context grows with turns.
- **A package boundary is chosen at DECOMPOSITION, never at the moment an agent feels full.** An agent that finds its package too big REPORTS the size to the main thread, which re-cuts the packages. It never trims an acceptance criterion, parks a defect, or weakens a test to fit (`ai/rules/completion.md`).
- **Lower cost by SIZING agents, never by spawning fewer of them.** "Spawning an agent costs a round trip" is already banned reasoning (`ai/rules/planning.md`): the round trip IS the supervision.
- **A main thread that must keep reading has already lost the argument: write the per-spec state file and hand off** (`ai/rules/planning.md`, "Spec Work Runs in Subagents"). 49.5% of measured main-thread context was fed at calls already above 600k, against a 1M ceiling. That is the main-thread column of the main-against-subagent histogram (26.7% at 600k-800k plus 22.8% at 800k-1M). The all-calls histogram beside it says 18.2%, because the subagent calls dilute it.
- **Resolve the symbol. There are two routes and an ORDER: the LSP tool first, `gopls` through Bash when the tool is absent.** Try `ToolSearch query="select:LSP"`; when your registry carries the tool, use it. When it answers empty, run `gopls` (see "The gopls CLI" below) -- same server, same answers. Whether a given context carries the tool is a property of the harness build and the machine, never of this repository: do not write either state down as a standing fact, and do not assume one before you have checked.
- **Every context reaches the capability, because `ze-setup` puts `gopls` on PATH.** "I have no LSP" is never the end of a symbol question. It selects the second route.
- **The main thread resolving symbols and handing agents `file + symbol + line range` is an OPTIMISATION, not a precondition.** It pays one resolution instead of one per agent. An agent given a range READS it; an agent given a bare file name that resolves nothing HUNTS through it, and that hunt is the 54.2% session re-read above.
- **No phase is undelegatable for want of LSP.** "Every call site updated" and "every implementation of this interface handles the new case" are answerable in a subagent, by whichever of the two routes is live there. Size an agent on cost; tool availability does not decide what you may delegate.

## Which LSP Operation Answers Which Question

| Question | LSP tool operation | `gopls` CLI, available everywhere | What comes back |
|----------|-----------|-----------------|-----------------|
| What is in this file? | `documentSymbol` | `gopls symbols <file>` | every symbol with its line range: the map you would otherwise read the whole file to build |
| What does this one symbol declare or say? | `goToDefinition`, then `hover` | `gopls definition <file>:<line>:<col>` | the declaration and its doc comment, not the file around it |
| Who calls this? | `findReferences` | `gopls references <file>:<line>:<col>` | every call site as file plus line. `grep` on a common name returns the comments and the string literals too |
| Who calls this, and from inside WHICH function? | `callHierarchy` | `gopls call_hierarchy <file>:<line>:<col>` | each caller's range AND the enclosing function that `references` leaves you to work out |
| Where does a name I can spell actually live? | `workspaceSymbol` | `gopls workspace_symbol <name>` | the file holding it, without guessing a directory |
| Does this file compile, and with what errors? | (diagnostics) | `gopls check <file>` | the type errors for that file. Silence and exit 0 mean clean |

## The gopls CLI: LSP From Any Context, Subagents Included

- **`gopls` is on PATH (`scripts/dev/dev-setup.py`, `REQUIRED_TOOLS`), so ANY context with Bash reaches the capability, whatever its tool registry holds.** This is the fall-back route of the two above, and it needs no session restart. A context whose `ToolSearch` came back empty runs the command instead. It never reads a whole file to hunt for a symbol, and it never reports back that it could not look.
- **The measured saving is the CLI's, not only the tool's.** `gopls symbols` output against the whole file: `internal/component/bgp/reactor/peer.go` 5,338 bytes against 48,513 (9.1x), `internal/component/ike/engine/fsm.go` 1,297 against 44,164 (34.1x), `internal/component/bgp/reactor/session_prefix.go` 2,254 against 23,395 (10.4x). One `definition` answered in 705 bytes, doc comment included.
- **The two-step recipe, and the only one you need: `gopls symbols <file>` prints `Name Kind <line>:<col>-<line>:<col>`, and that `<line>:<col>` is exactly what `definition`, `references` and `call_hierarchy` take.** Find the symbol in step one, ask about it in step two. Never guess a position.
- **Positions are 1-based and the column is the START of the identifier, never the start of the line.** A `func` declaration puts its name at column 6.
- **Every invocation starts a fresh server and loads the workspace, so budget seconds, not milliseconds.** Measured warm in this repository: `symbols` 3.3s, `workspace_symbol` 3.7s, `references` 4.1s, `definition` 6.5s. A 60s timeout is generous; do not paste a multi-minute one.
- **Batch them like any other Bash call.** Several independent `gopls` questions belong in ONE message, as with any independent calls.
- **`gopls mcp` serves the same workspace as an MCP server over stdio, and `.mcp.json` at the repo root registers it.** MCP servers are bound at SESSION START, so a session that was already running when that file changed does not carry them. The CLI above needs no restart and is the path that always works.

```
$ gopls symbols internal/component/bgp/config/resolve.go
ResolveBGPTree Function 43:6-43:20
$ gopls definition internal/component/bgp/config/resolve.go:43:6
.../resolve.go:43:6-43:20: defined here as func ResolveBGPTree(tree *config.Tree) (map[string]any, error)
ResolveBGPTree resolves peer-group inheritance and returns the bgp block as map[string]any.
$ gopls references internal/component/bgp/config/resolve.go:43:6
.../loader_create.go:274:28-42
.../peers.go:53:18-32
```

## What This Rule Never Targets

- **Review. 144 review agents were 15.4% of measured subagent context, and the fix/debug phase they prevent was 24.5%.** Cutting lenses, passes, or the model a reviewer runs on to save tokens costs more than it saves, and is banned by `ai/rules/planning.md` independently of this measurement. `make ze-token-economy` prints both figures, and labels its phase split a keyword heuristic over the spawn description: nothing in the transcript store records the phase an agent ran.
- **Gates. No check, test, or verification target is skipped to save a round trip** (`ai/rules/completion.md`, `ai/rules/git-safety.md`). A gate not run is not a saving; it is an unmeasured risk.
- **The rules themselves. A rule you did not read is a rule you did not follow.** `ai/rules/TRIGGERS.md` names every rule in one line each precisely so the read is targeted, not skipped.

## Banned Reasoning

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
