---
name: ze-handoff
description: Write a self-contained handoff so a fresh session can continue this work, listing what is done, in progress and remaining, with concrete next edits. Use when ending a session with work left.
---

# Handoff

Generate a session handoff document for continuing work in a new session.

The output is a self-contained document that a fresh session can execute without re-reading the full codebase.

See also: `/ze-debrief` (current session state), `/ze-status` (cross-project attention view)

## Steps

1. **Read the spec:** Run `./le spec current`. If a spec is selected, read the spec file.
2. **Identify remaining work:** From the spec, conversation, and task list, enumerate:
   - What is done (committed or implemented)
   - What is in progress (uncommitted changes)
   - What is remaining (unfinished spec items, ACs without evidence)
   - What this spec will not do (its **Work Not Done** table, naming the spec that owns each item) and what this session skipped
3. **Identify files already handled:** List files that were read, understood, and don't need re-reading.
4. **Build the edit list:** For remaining work, produce concrete edits. Each edit must be:
   - File path with line range
   - Exact old text to replace (or "new section after line N")
   - Exact new text
   - Maximum 5 edits per handoff. If more remain, note "Phase 2 needed after these edits."
5. **Determine verification command:** What to run after the edits to confirm they work.
6. **Write and commit the handoff in `plan/handover/`** (`ai/META-DECISIONS.md`: a handover lives in `plan/handover/` and is committed in the turn it is written, never only in chat or a session directory). Name it `plan/handover/YYYY-MM-DD-<slug>.md` so concurrent sessions never collide on numeric prefixes, commit it through `./le commit create`, and present a concise link and summary using this format:

```
## Handoff

**Spec:** [name or "none"]
**Branch:** [branch name]
**Goal:** [1-2 sentences]

### Status
- Done: [bullet list]
- In progress: [bullet list]
- Remaining: [bullet list]
- Not done, and the spec that owns each item: [bullet list]

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

- Write only the handover file under `plan/handover/` and commit it via `./le commit create`; do NOT edit product code or unrelated files.
- Each edit must be self-contained -- no "update similarly", spell it out.
- Cite files and symbols (not bare line numbers) in prose; keep exact line numbers only inside fenced code blocks if quoting tool output (`ai/rules/writing.md`).
- Maximum 5 edits. Split into phases if more remain.
- Include what this spec will not do, so the next session knows what was intentionally left and which spec now owns it.

## Done when

Done when the handover file is written under `plan/handover/`, committed via `./le commit create`, and presented with its path, SHA, at most five self-contained edits, and a verification command.
