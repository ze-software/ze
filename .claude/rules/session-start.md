# Session Start

**BLOCKING:** Complete before any work.
Rationale: `.claude/rationale/session-start.md`

## Checklist

```
[ ] 1. Load LSP tool (`select:LSP`) for Go code intelligence
[ ] 2. Read .claude/selected-spec
[ ] 3. Read plan/<spec-name> (if selected)
[ ] 4. Read per-spec session state (.claude/session-state-<spec-stem>.md) if exists
[ ] 5. Check git status
[ ] 6. If user provides a handoff: complete Receiving a Handoff (below) BEFORE any plan
[ ] 7. Start working
```

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
