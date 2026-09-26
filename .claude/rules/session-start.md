# Session Start

**Blocking:** Complete before any work.
Rationale: `ai/rationale/session-start.md`

## Checklist

```
[ ] 1. Read `docs/contributing/ze-go-style.md`. Every session, before any code.
[ ] 2. Run `./le spec current` to see this session's claimed spec
[ ] 3. Read plan/<spec-name> (if a spec is claimed)
[ ] 4. Read per-spec session state (tmp/session/<YYYY-MM-DD>-<SID>/state/session-state-<spec-stem>-<SID>.md) if exists
[ ] 5. Check git status
[ ] 6. If user provides a handoff: complete Receiving a Handoff (below) before any plan
[ ] 7. Before the first search, grep, or agent: read the page that documents the
       surface (`ai/rules/documentation.md`, always-on in `ai/rules/CORE.md`)
[ ] 8. Start working
```

## Style Read (step 1)

Read `docs/contributing/ze-go-style.md` in full before writing any code, every
session, whatever the task looks like. This applies to ordinary edits too, not
only to a Go design decision, a review, or an argument about how Ze code is
written.

Nothing else reminds you: `ze-style` is an output style
(`.claude/output-styles/ze-style.md`), not a skill, and Go written through a
Bash heredoc never reaches the `pretool-writeedit` action.

## Symbols

Resolve a Go symbol with the LSP tool, or with `gopls` from Bash where the
registry serves no LSP tool. Never read a whole file to find a symbol. The
`gopls` recipes and their costs are in `ai/rules/context-economy.md`.

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
