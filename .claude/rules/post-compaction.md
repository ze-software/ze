# Post-Compaction Recovery / Resuming Spec Work

**BLOCKING:** After context compaction OR when resuming work on a spec, you MUST complete recovery before ANY other action.

## When This Applies

- Context was compacted ("continued from a previous conversation")
- New session started
- Resuming work on an existing spec
- Coming back to a spec after working on something else
- You don't remember recent conversation details

**Rule:** If you didn't read the source files in THIS session, you must read them again.

## Checkboxes Are Lies

**⚠️ Checkboxes in specs are MEANINGLESS unless you read the file THIS session.**

A `[x]` next to a file means you read it in a PREVIOUS session. You don't remember the content. The checkbox is a lie.

**You MUST re-read every file marked as read** when:
- Resuming work on a spec
- After context compaction
- Starting a new session
- Coming back to a spec after other work

Do not trust past checkboxes. Do not trust your memory. READ THE CODE.

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

[ ] 5. Re-read ALL source files in spec's "Current Behavior" section
      → You MUST know what the existing code does
      → Do NOT rely on spec description - READ THE CODE

[ ] 6. Re-read architecture docs in spec's "Required Reading" section
      → Ignore checkboxes - re-read everything
      → FAST-PATH: If entries have `→ Decision:` / `→ Constraint:` annotations,
        read those FIRST for immediate context, then re-read full docs

[ ] 7. Check git status
      → What files are modified?
      → What's the current state?

[ ] 8. ONLY THEN continue work
```

## Context Checkpoints (Fast-Path Recovery)

Specs written with the current template include `→ Decision:` and `→ Constraint:` annotations under Required Reading entries. These capture the key insight from each doc in one line:

```
- [ ] `docs/architecture/wire/attributes.md` - [why relevant]
  → Decision: Attributes use pool-based dedup with ref-counting
  → Constraint: Must use WriteHeaderTo(), not PackHeader()
```

**After compaction, read these annotations FIRST.** They restore critical context in seconds instead of minutes. Then re-read the full docs to fill in details.

If the spec lacks checkpoint annotations, you must read the full docs with no shortcut.

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
