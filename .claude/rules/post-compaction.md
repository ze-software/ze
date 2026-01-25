# Post-Compaction Recovery

**BLOCKING:** After context compaction, you MUST complete recovery before ANY other action.

## Detection

Compaction occurred if you see:
- "continued from a previous conversation"
- "context was compacted"
- You don't remember recent conversation details

## Recovery Steps (MANDATORY)

Complete IN ORDER before doing ANYTHING else:

```
[ ] 1. Read selected spec
      → cat .claude/selected-spec
      → Read docs/plan/<spec-name> if not empty

[ ] 2. Read spec's "Post-Compaction Recovery" section
      → Lists all docs/files you MUST re-read

[ ] 3. Read planning rules
      → .claude/rules/planning.md

[ ] 4. Read session state (if exists)
      → .claude/session-state.md
      → Contains decisions, progress, what was read

[ ] 5. Re-read architecture docs relevant to current work
      → Check spec's "Required Reading" section

[ ] 6. Check git status
      → What files are modified?
      → What's the current state?

[ ] 7. ONLY THEN continue work
```

## What You MUST NOT Do Post-Compaction

Until recovery is complete:
- ❌ Write any code
- ❌ Create any files
- ❌ Make design decisions
- ❌ Claim anything is "done"
- ❌ Propose changes

## Session State

After recovery, update `.claude/session-state.md` with:
- What you've re-read
- Current task status
- Key decisions from the spec

## Why This Matters

Without recovery:
- You'll redesign already-decided things
- You'll break working code
- You'll duplicate existing patterns
- You'll waste user's time
