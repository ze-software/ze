# Session Start

**BLOCKING:** Complete before any work.
Rationale: `ai/rationale/session-start.md`

## Checklist

```
[ ] 1. Load LSP tool (`ToolSearch query="select:LSP"`). UNCONDITIONAL FIRST ACTION.
[ ] 2. Read `docs/contributing/ze-style.md`. EVERY session, before any code.
[ ] 3. Run `scripts/dev/spec-session.sh current` to see this session's claimed spec
[ ] 4. Read plan/<spec-name> (if a spec is claimed)
[ ] 5. Read per-spec session state (tmp/session/<YYYY-MM-DD>-<SID>/state/session-state-<spec-stem>-<SID>.md) if exists
[ ] 6. Check git status
[ ] 7. If user provides a handoff: complete Receiving a Handoff (below) BEFORE any plan
[ ] 8. Start working
```

## Style Read (step 2) -- owner directive, 2026-08-18

**BLOCKING, every session, whatever the task looks like.** Read
`docs/contributing/ze-style.md` in full before writing any code.

This REPLACES the older instruction in
`ai/rules/points/go-standards/directives/read-the-ze-style-guide-before-go-design-or-review.md`,
which read the guide only before a Go DESIGN decision, a review, or an argument
about how Ze code is written, and told you not to open it for an ordinary edit.
That gate was set to save context and it cost more than it saved: a session can
write Go all day, never meet one of those three triggers, and never learn that
Ze guards with early returns, splits a compound condition, or states an
invariant positively.

The failure was measured on 2026-08-18. One session wrote `dpd.go`,
`detector.go`, `command_registry.go` and `process.go` without opening the guide
once, and shipped `if d == nil || !d.awaitReply` and a three-fact error
condition. It had a route -- `ai/rules/TRIGGERS.md` lists `go-standards` under
"writing Go in Ze" -- and did not take it.

Two things hid the gap, and neither is a reason to rely on them:

- `ze-style` is an OUTPUT STYLE (`.claude/output-styles/ze-style.md`), not a
  skill, so it never appears in the skills listing an agent reads at startup.
- `c_pre_write_go` (`.claude/hooks/pretool-writeedit.py`) returns None unless
  the tool is `Write` or `Edit`. Go written through a Bash heredoc reaches it
  never, and auto mode tells agents to prefer Bash for file changes.

## LSP Load (step 1) -- no-exceptions clause

**BLOCKING. Load LSP before any other tool call, regardless of what the task looks like.**

The repo has been bitten by sessions that rationalized skipping this step. To close
the loophole: every one of the excuses below is **banned reasoning**. If you find
yourself thinking any of them, stop and call `ToolSearch query="select:LSP"` first.

| Banned excuse | Reality |
|---------------|---------|
| "The task is shell-only / Makefile-only" | Shell edits drive Go tests. Investigations branch. Load it. |
| "The task is docs / markdown-only" | Docs describe Go code. You may need to verify a symbol. Load it. |
| "The task is config / YAML-only" | Config references Go structs. Load it. |
| "It's a trivial one-file change" | Triviality is judged after reading, not before. Load it. |
| "LSP is for Go navigation and I won't navigate" | Predicting future tool use is the antipattern. Load it. |
| "The user will correct me if I need it" | They have. Repeatedly. That is the cost. Load it. |

Loading LSP is ~1 tool call and zero-cost if unused. Skipping it costs a round-trip
with the user every time you are wrong about what the task needs. The asymmetry is
not close.

**Empty-result carve-out.** The requirement is to ISSUE the query, not to succeed in
loading a tool your harness does not expose. Some contexts get "No matching deferred
tools found" back, subagents on some builds among them. Issuing the query and getting
that answer SATISFIES step 1 -- proceed, do not retry, do not treat it as a skipped
step. The gate agrees: `.claude/hooks/block-until-lsp.sh` lifts on the query text, not
on a successful load (by design -- a stuck session is the worse failure). The banned
excuses above are about SKIPPING the query; issuing it and getting nothing back is not
a skip.

**An empty result routes you to the second way, it does not leave you without one.**
`gopls` is on PATH (`make ze-dev-setup` installs it) and every context has Bash, so the
same server answers the same questions: `gopls symbols <file>` maps a file, and
`gopls definition|references <file>:<line>:<col>` answers about a symbol. The recipes
and their measured costs are in `ai/rules/context-economy.md`. Which contexts carry
the tool varies by harness build and by machine, so check rather than assume -- and
never read a whole file to hunt for a symbol on the strength of one empty query.

**A loaded schema is not a working server.** The tool talks to `gopls`; without that
binary every call returns `ENOENT: gopls` and the session silently falls back to
reading whole files. That is what happened on one of the two dev machines: the server
was absent there until 2026-08-05, and that machine's transcript store held 33
sessions with no LSP call in any of them (`make ze-token-economy-report` reads
`~/.claude/projects/`, so its counts are per-machine and say nothing about the other).
The gate could not see it, and by design will not: it lifts on the query text, because
a stuck session is the worse failure.

So a context whose registry DID serve the tool verifies the server ONCE per session,
right after step 1:

```
command -v gopls || make ze-dev-setup
```

When it is missing, SAY SO and install it (`make ze-dev-setup` installs `gopls`, among
the rest; `make ze-dev-setup CHECK=1` only reports). Working on without a server, having
seen it is absent, is the failure this paragraph exists to name. Once per session is
the whole cost: do not re-probe before each call. A context that fell back to the CLI
needs no separate probe -- it calls `gopls` directly, so a missing binary announces
itself on the first call instead of failing silently behind a tool.

**Mechanical rule:** the first `ToolSearch` / `Bash` / `Read` / `Edit` / anything
in a new session must be `ToolSearch query="select:LSP"`. If it is not, you have
violated this rule. Apologize, load it, proceed.

## Receiving a Handoff (BLOCKING)

When the user provides a handoff document (structured state from a previous session):

1. **Enumerate every outstanding item** from the handoff into a table. Every AC, every task, every blocked item, every mistake noted. No filtering, no editorializing, no forming opinions about what matters.
2. **Present the enumeration** to the user. This is verification that nothing was dropped.
3. **Only then** propose a plan or ask about priorities.

| Banned | Why |
|--------|-----|
| Skimming for themes | Drops specific items that don't fit the narrative |
| Forming a plan before enumerating | Plan filters out items that seem hard or unfamiliar |
| Summarizing categories instead of listing items | "Data infrastructure" hides 5 specific ACs |
| Proposing action before the user confirms completeness | Commits to a direction before scope is agreed |

**Mechanical check:** count the items in your enumeration. Count the items in the handoff. If they don't match, you missed something.

## Session Focus

Do not switch to a different line of work without confirming with the user first.
When the original task is done (e.g., spec closed), stop and ask "What next?" instead
of picking up other uncommitted work. "Continue what you were doing" means the stated
goal, not "find more things to do."

The Stop hook knows about this instruction and does not fight it.
`.claude/hooks/block-premature-stop.sh` holds `what next` and `what would you like`
in a second phrase list (`COMPLETION_PHRASES`, `:133-138`). It scans that list ONLY
when this session has a claimed spec whose Status is still `in-progress`. The flag
is `OPEN_WORK`, set at `:193` and assembled at `:225-228`.

So the question above is permitted when no work remains. It is refused with exit 2
while a spec is open. The question is mandated behavior once the task is done. The
same words mid-spec are premature stopping (`ai/rules/completion.md`). Fixtures:
`stop-phrase-what-next-allowed-when-no-open-work` and
`stop-phrase-what-next-blocks-with-open-work`.
