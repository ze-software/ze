# Session Start

**Blocking:** Complete before any work.
Rationale: `ai/rationale/session-start.md`

## Checklist

```
[ ] 1. Load LSP tool (`ToolSearch query="select:LSP"`). This is the first action, unconditionally.
[ ] 2. Read `docs/contributing/ze-go-style.md`. Every session, before any code.
[ ] 3. Run `./le spec current` to see this session's claimed spec
[ ] 4. Read plan/<spec-name> (if a spec is claimed)
[ ] 5. Read per-spec session state (tmp/session/<YYYY-MM-DD>-<SID>/state/session-state-<spec-stem>-<SID>.md) if exists
[ ] 6. Check git status
[ ] 7. If user provides a handoff: complete Receiving a Handoff (below) before any plan
[ ] 8. Before the first search, grep, or agent: read the page that documents the
       surface (`ai/rules/documentation.md`, always-on in `ai/rules/CORE.md`)
[ ] 9. Start working
```

## Style Read (step 2)

Read `docs/contributing/ze-go-style.md` in full before writing any code, every
session, whatever the task looks like. This applies to ordinary edits too, not
only to a Go design decision, a review, or an argument about how Ze code is
written.

Nothing else reminds you: `ze-style` is an output style
(`.claude/output-styles/ze-style.md`), not a skill, and Go written through a
Bash heredoc never reaches the `pretool-writeedit` action.

## LSP Load (step 1) -- no-exceptions clause

**Load LSP before any other tool call, whatever the task looks like.** The first
`ToolSearch`, `Bash`, `Read`, `Edit` or other call in a new session is
`ToolSearch query="select:LSP"`. A task that looks shell-only, docs-only,
config-only or trivial is not an exception: the query costs one call. If your
first call was something else, load it now and proceed.

**Empty-result carve-out.** The requirement is to issue the query, not to load a
tool your harness does not expose. A "No matching deferred tools found" answer
(subagents on some builds get it) satisfies step 1: proceed, do not retry, and do not treat it as a skipped step.
The native `block-until-lsp` action (`./le ai hooks block-until-lsp`,
`internal/le/hookruntime/lifecycle.go`) lifts on the query text, not on a
successful load.

**An empty result routes you to the second way, it does not leave you without one.**
`gopls` is on PATH (`./le setup` installs it) and every context has Bash, so the
same server answers the same questions: `gopls symbols <file>` maps a file, and
`gopls definition|references <file>:<line>:<col>` answers about a symbol. The recipes
and their costs are in `ai/rules/context-economy.md`. Which contexts carry the tool
varies by harness build and by machine, so check rather than assume, and do not
read a whole file to hunt for a symbol on the strength of one empty query.

**A loaded schema is not a working server.** The tool talks to `gopls`; without that
binary every call returns `ENOENT: gopls` and the session silently falls back to
reading whole files. The gate cannot see this. So a context whose registry served
the tool verifies the server once per session, right after step 1:

```
command -v gopls || ./le setup
```

When it is missing, say so and install it (`./le setup` installs `gopls`, among
the rest; `./le setup --check` only reports). Once per session is the whole cost:
do not re-probe before each call. A context that fell back to the CLI needs no
separate probe: it calls `gopls` directly, so a missing binary announces itself on
the first call.

## Receiving a Handoff

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

**One spec per session, and the next spec in a fresh session.** The main thread
never compacts under a 1M window, and every call re-feeds all of it. The per-spec
state file is the handoff, so the next session starts from it, not from this one.

The Stop hook knows about this instruction and does not fight it.
The native `block-premature-stop` action (`./le ai hooks block-premature-stop`,
`hookStop` in `internal/le/hookruntime/lifecycle.go`) holds `what next` and `what
would you like` in a second phrase list, appended only when `openWork` is true.
`openWork` is set when the claimed spec's Status is still `in-progress`.

So the question above is permitted when no work remains. It is refused with exit 2
while a spec is open. The question is mandated behavior once the task is done. The
same words mid-spec are premature stopping (`ai/rules/completion.md`). Fixtures:
`stop-phrase-what-next-allowed-when-no-open-work` and
`stop-phrase-what-next-blocks-with-open-work`.
