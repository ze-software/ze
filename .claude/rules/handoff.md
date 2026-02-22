# Session Handoff

Rationale: New sessions waste tokens re-reading and re-analyzing. Give exact edits instead.

## When User Asks How to Continue

**BLOCKING:** Output **exact edits**, not design explanations.

| Include | Don't Include |
|---------|---------------|
| File path + line range for each edit | Architecture explanations |
| OLD text → NEW text (copy-pasteable) | Design decisions already made |
| "Don't re-read these files" list | File summaries |
| Final verification command | Research the new session can skip |

## Template

```
Do these edits in order. Don't re-read files unless stuck.

FILES ALREADY HANDLED (don't re-read): [list]

EDIT 1: [file:lines]
- Delete/Replace: [exact old text → new text]

EDIT 2: [file:lines]
- Delete/Replace: [exact old text → new text]

THEN: [test command with timeout]
```

## Rules

- Max 5 remaining edits per handoff. If more, split into phases.
- Each edit must be self-contained (no "update similarly" — spell it out).
- Include line numbers from current file state, not original.
