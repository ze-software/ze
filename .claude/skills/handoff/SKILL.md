# Handoff

Generate a session handoff document for continuing work in a new session.

The output is a self-contained document that a fresh session can execute without re-reading the full codebase.

## Steps

1. **Read the spec:** Read `.claude/selected-spec`. If a spec is selected, read the spec file.
2. **Identify remaining work:** From the spec, conversation, and task list, enumerate:
   - What is done (committed or implemented)
   - What is in progress (uncommitted changes)
   - What is remaining (unfinished spec items, ACs without evidence)
   - What is deferred (from `plan/deferrals.md` and session skips)
3. **Identify files already handled:** List files that were read, understood, and don't need re-reading.
4. **Build the edit list:** For remaining work, produce concrete edits. Each edit must be:
   - File path with line range
   - Exact old text to replace (or "new section after line N")
   - Exact new text
   - Maximum 5 edits per handoff. If more remain, note "Phase 2 needed after these edits."
5. **Determine verification command:** What to run after the edits to confirm they work.
6. **Present the handoff** using this format:

```
## Handoff

**Spec:** [name or "none"]
**Branch:** [branch name]
**Goal:** [1-2 sentences]

### Status
- Done: [bullet list]
- In progress: [bullet list]
- Remaining: [bullet list]
- Deferred: [bullet list]

### Files Already Handled (don't re-read)
- [file list with one-line description each]

### Edits

EDIT 1: [file:lines]
- [Delete/Replace/Add]: [exact old text -> new text]

EDIT 2: [file:lines]
- [Delete/Replace/Add]: [exact old text -> new text]

...

### Then
[verification command with timeout]
```

## Rules

- Do NOT edit anything. Generate the document only.
- Each edit must be self-contained -- no "update similarly", spell it out.
- Line numbers from current file state, not original.
- Maximum 5 edits. Split into phases if more remain.
- Include deferred items so the next session knows what was intentionally skipped.
