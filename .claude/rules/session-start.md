# Session Start

**BLOCKING:** Complete before any work.
Rationale: `ai/rationale/session-start.md`

## Checklist

```
[ ] 1. Load LSP tool (`ToolSearch query="select:LSP"`). UNCONDITIONAL FIRST ACTION.
[ ] 2. Read tmp/session/selected-spec
[ ] 3. Read plan/<spec-name> (if selected)
[ ] 4. Read per-spec session state (tmp/session/session-state-<spec-stem>-<SID>.md) if exists
[ ] 5. Check git status
[ ] 6. If user provides a handoff: complete Receiving a Handoff (below) BEFORE any plan
[ ] 7. Start working
```

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
