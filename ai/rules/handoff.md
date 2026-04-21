# Session Handoff

Rationale: New sessions waste tokens re-reading. Give exact edits, but lead
with the rationale so the user can verify the handoff matches the decisions
they believe were agreed.

## When User Asks How to Continue

**BLOCKING:** Start with a short rationale section, then output **exact edits**.
The rationale exists so the user can catch a misaligned handoff BEFORE the next
session blindly applies the edits. If the rationale and the edits disagree,
the user must be able to spot it from the handoff alone.

| Include | Exclude |
|---------|---------|
| Rationale: what was agreed and why (3-6 bullets) | Re-derivation of background research |
| Design decisions the edits encode | File summaries unrelated to the edits |
| File path + line range per edit | Speculative future work |
| OLD text -> NEW text (copy-pasteable) | Redundant restatement of the codebase |
| "Don't re-read these files" list | |
| Final verification command | |

The rationale is a verification checkpoint, not an essay. Each bullet should
name a decision (not a fact) and tie to one or more edits below.

## Template

```
RATIONALE (verify this matches what we agreed):
- Decision 1: [what + why] -> EDIT N
- Decision 2: [what + why] -> EDIT N
- Anything still open or assumed: [list]

If any bullet is wrong, STOP and fix the handoff before applying edits.

FILES ALREADY HANDLED (don't re-read): [list]

EDIT 1: [file:lines]
- Delete/Replace: [exact old text -> new text]

EDIT 2: [file:lines]
- Delete/Replace: [exact old text -> new text]

THEN: [test command with timeout]
```

## Rules

- Rationale bullets map to edits. An edit with no rationale bullet is suspect.
- Max 5 remaining edits per handoff. Split into phases if more.
- Each edit self-contained. No "update similarly", spell it out.
- Line numbers from current file state, not original.
- If a decision is assumed rather than agreed, mark it explicitly in the
  rationale so the user can correct it.
