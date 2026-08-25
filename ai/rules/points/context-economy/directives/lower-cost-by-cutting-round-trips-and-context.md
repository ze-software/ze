---
kind: directive
level: MUST
stage:
---
- **Cost per API call is the context size at that call, so the bill is round trips times context and nothing else moves it much.** Every choice below lowers one of those two terms.
- **Trimming tool OUTPUT lowers neither term, so MUST NOT spend effort there.** A shorter command output changes no number a session pays for.
- **MUST batch independent tool calls into ONE message.** Two Reads, a Read and a Grep, or three Bash checks that do not consume each other's results belong in one message.
- **The precondition is INDEPENDENCE, not relatedness.** A Read and an Edit that consumes that Read's result are dependent and MUST NOT share a batch: the Edit is composed before the Read returns, so it lands on content nobody has seen. When in doubt about a dependency, MUST split the batch.
- **MUST use LSP FIRST for any symbol question: before Grep, before Read.**
- **Reading a whole file to FIND a symbol is the anti-pattern.** The file is the container; the symbol is the question. LSP answers the question, and every other token in that file is paid for on every later call in the session.
- **MUST read a RANGE, not a whole file, above 500 lines. MUST resolve the symbol with LSP first, or with `grep -n` / `sed -n` where LSP is absent, then read the range it names.**
- **MUST NOT re-read what a digest already records.** `tmp/session/<YYYY-MM-DD>-<SID>/state/session-state-<spec-stem>-<SID>.md` holds the per-spec digest, and `_find_latest_state_for_spec` (`.claude/hooks/lib/state-file.sh`) resolves the newest one for a spec across sessions. MUST read the digest first, and MAY re-read the source only when the digest lacks the detail your claim depends on.
- **A digest is not evidence.** When you are about to state what code does, MUST read the producing function (`ai/rules/evidence.md`). This rule lowers the cost of reading; it never lowers the standard of proof.
- **MUST size a work package so ONE agent finishes it.** Cost inside one agent grows with its turns because its context grows with its turns.
- **A package boundary MUST be chosen at DECOMPOSITION; it MUST NOT be chosen at the moment an agent feels full.** An agent that finds its package too big MUST REPORT the size to the main thread, which re-cuts the packages. It MUST NOT trim an acceptance criterion, park a defect, or weaken a test to fit (`ai/rules/completion.md`).
- **MUST lower cost by SIZING agents; MUST NOT lower it by spawning fewer of them.** "Spawning an agent costs a round trip" is already banned reasoning (`ai/rules/planning.md`): the round trip IS the supervision.
- **A main thread that has to keep reading has already lost the argument: MUST write the per-spec state file and hand off** (`ai/rules/planning.md`, "Spec Work Runs in Subagents").
- **MUST resolve the symbol. There are two routes and an ORDER: the LSP tool first, `gopls` through Bash when the tool is absent.** MUST try `ToolSearch query="select:LSP"`; when your registry carries the tool, MUST use it. When it answers empty, MUST run `gopls` (see "The gopls CLI" below) -- same server, same answers. Whether a given context carries the tool is a property of the harness build and the machine, never of this repository: MUST NOT write either state down as a standing fact, and MUST NOT assume one before you have checked.
- **Every context reaches the capability for Go, because `./le setup` puts `gopls` on PATH.** "I have no LSP" is never the end of a Go symbol question. It selects the second route. Python has no equivalent CLI route.
- **The main thread resolving symbols and handing agents `file + symbol + line range` is an OPTIMISATION, not a precondition.** It pays one resolution instead of one per agent. An agent given a range READS it; an agent given a bare file name that resolves nothing HUNTS through it.
- **No phase is undelegatable for want of LSP.** "Every call site updated" and "every implementation of this interface handles the new case" are answerable in a subagent, by whichever of the two routes is live there. MUST size an agent on cost; tool availability does not decide what you can delegate.
