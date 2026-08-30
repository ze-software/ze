---
kind: directive
level: MUST
stage:
---
- **You MUST write what changes the reader's next action, and nothing else.** Detail is a cost the reader pays, not proof that you did the work, so a fact the reader can recover in seconds by opening the code MUST NOT be written down.
- **You MUST cite a location so the reader can NAVIGATE, never to show that you looked, and you MUST name the file and the symbol: `session.go` `Session.Run`.** A line number is correct only when the line IS the fact, or when a gate or a generator pins it: a stack frame, a generated ledger row, a gate's own message, a `file:line -> sha` audit entry, an `ai/digests/` anchor, a handoff edit range, a `<!-- source: -->` anchor. Everywhere else it rots at the next edit.
- **The artifact budgets, the banned detail patterns, and how to ask a question in plain English are in `docs/contributing/writing-style.md`.** Over budget means CUT, never split into two documents.
