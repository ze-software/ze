---
kind: directive
level:
stage:
---
- **Detail is a cost the reader pays, not proof that you did the work.** A fact the reader can recover in seconds by opening the code is not written down.
- **Cite a location so the reader can NAVIGATE, never to show that you looked.** Verification is an action you take (read the producing function). The citation is a pointer for the reader, and it is a separate decision.
- **Name the file and the symbol: `session.go` `Session.Run`.** A line number is correct when the line IS the fact, or when a gate or generator pins it. Examples: a stack frame, a generated ledger row, a gate's own message, a `file:line -> sha` audit entry, a `ai/digests/` anchor that `make ze-digest-check` validates, a handoff edit range, and a `<!-- source: -->` anchor. Everywhere else the number rots at the next edit, and a reader who has the symbol never needs it.
- **One example for one point.** A second example earns its place only when it shows a DIFFERENT reading. A second instance of the same reading teaches nothing and costs every future session.
- **When a directive can be read two ways, write both readings and name the one that governs.** More examples hide an ambiguity. Naming the readings ends it.
- **Never make the same cut twice.** When a table and a paragraph draw the same distinction, keep the table and delete the paragraph.
- **State the obligation, name the gate, stop.** How a gate is implemented (flags, exit codes, guard order, retry bounds, byte offsets) belongs in the script and its fixtures. A rule that narrates its own enforcement code is a second, stale copy of that code.
- **Report the conclusion, not the search.** What you tried, in what order, and how long it took are yours. The reader needs the answer, the evidence that would overturn it, and what is still open.
- **Give a count plus the exceptions, not a row per item.** "12 call sites updated, 2 refused and are listed below" is complete. Twelve identical rows are not more complete.
- **A directive line in an always-on rule enters EVERY session through `CORE.md`, and every rule's `**When:**` line enters it through `TRIGGERS.md`.** Before you add one, ask whether it changes an action. When it does not, put it under `## Rationale` or `## Examples`. The digest drops both.
- **A pointer line points. It never summarises.** An entry in `ai/INDEX.md` or any other index says what the target answers, then stops. Under 120 characters after the link. A reader who wants the content opens the target.
