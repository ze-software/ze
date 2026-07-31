# Detail Budget

**When:** writing anything a reader acts on: a reply, a rule, a document, a commit body, a learned summary, or an agent report
**Severity:** advisory
**Related:** fix-dont-record, no-fabrication, rule-format, simplified-technical-english

## Directives

Write what changes the reader's next action. Write nothing else.

- **Detail is a cost the reader pays, not proof that you did the work.** A fact the reader can recover in seconds by opening the code is not written down.
- **Cite a location so the reader can NAVIGATE, never to show that you looked.** Verification is an action you take (read the producing function). The citation is a pointer for the reader, and it is a separate decision.
- **Name the file and the symbol: `session.go` `Session.Run`.** A line number is correct when the line IS the fact, or when a gate or generator pins it. Examples: a stack frame, a generated ledger row, a gate's own message, a `file:line -> sha` audit entry, a `ai/digests/` anchor that `make ze-digest-check` validates, a handoff edit range, and a `<!-- source: -->` anchor. Everywhere else the number rots at the next edit, and a reader who has the symbol never needs it.
- **One example for one point.** A second example earns its place only when it shows a DIFFERENT reading. A second instance of the same reading teaches nothing and costs every future session.
- **When a directive can be read two ways, write both readings and name the one that governs.** More examples hide an ambiguity. Naming the readings ends it.
- **Never make the same cut twice.** When a table and a paragraph draw the same distinction, keep the table and delete the paragraph.
- **State the obligation, name the gate, stop.** How a gate is implemented (flags, exit codes, guard order, retry bounds, byte offsets) belongs in the script and its fixtures. A rule that narrates its own enforcement code is a second, stale copy of that code.
- **Report the conclusion, not the search.** What you tried, in what order, and how long it took are yours. The reader needs the answer, the evidence that would overturn it, and what is still open.
- **Give a count plus the exceptions, not a row per item.** "12 call sites updated, 2 refused and are listed below" is complete. Twelve identical rows are not more complete.
- **Every line in a directive section enters EVERY session through `CONDENSED.md`.** Before you add one, ask whether it changes an action. When it does not, put it under `## Rationale` or `## Examples`. The digest drops both.
- **A pointer line points. It never summarises.** An entry in `ai/LEARNED-INDEX.md`, `ai/INDEX.md`, or any other index says what the target answers, then stops. Under 120 characters after the link. A reader who wants the content opens the target.

## Budgets

A record earns its length from what the reader must DO. Over budget means cut, never split into two documents.

| Artifact | Contains | Budget |
|----------|----------|--------|
| Reply to the user | what changed, what proves it, what is not done | under 15 lines, tables before prose |
| Subagent report to the main thread | the conclusion, the evidence that would overturn it, open questions | under 40 lines |
| Review finding | the claim, where it lives, how it fails | 3 lines |
| Commit subject | what changed, imperative | one line |
| Commit body | the defect, its cause, what the fix does | under 15 lines |
| Known-failure shard | the failing output, the repro command, the next step | under 20 lines |
| Learned summary | what the code cannot tell a future reader | 25 to 35 lines (`ai/rules/planning.md`) |
| Index or pointer line | what the target answers | under 120 characters after the link |
| Rule file | trigger, directives, one example for each | under 150 lines. Above that, move reference tables to `docs/` and link |

No gate measures these yet. They are the standard a review applies, and the number to quote when a document is over.

## Banned

| Banned | Why |
|--------|-----|
| Recounting dead ends, wrong hypotheses, or the order you tried things | The reader needs the answer, not the route to it |
| Any sentence about the difficulty or size of the work | It changes no action |
| Restating a fact in the next paragraph, or "as noted above" | Say it once, in the place the reader looks first |
| A line number for a claim the symbol name already locates | It rots at the next edit and forces a re-index |
| Pasting a whole file, table, or log when the answer is one row | Quote the row |
| A third example to settle an ambiguity two readings would settle | The ambiguity survives, now hidden |

## Rationale

Detail feels like rigor, so it grows without anyone deciding to add it. The cost is invisible at the moment of writing and paid on every read after it.

Two measurements, 2026-07-31. `CONDENSED.md` reached 99k tokens. `ai/INSTRUCTIONS.md` imports it into every session before any work starts. One table row in `hook-mapping.md` reached 1,327 tokens. It narrates a hook's guard order, its exit codes, and its line offsets. The script and its 35 fixtures already state all three.

The drift is measurable elsewhere too. Learned summaries averaged 27 lines in the first hundred and 93 lines in the last hundred. The stated budget is 25 to 35. Entries in `ai/LEARNED-INDEX.md` started at about 80 characters and now run to 2538. An index that exists to route a reader now repeats the summary it points at.

The citation rule has a second cost. Nine rules mint the `file:line` demand independently. Seven `ze-*` skills repeat it for each claim. A line number pinned in prose goes stale on the next edit of the file it points into. This is why `/ze-rfc-audit` must tell a real verdict change from "a pure `file:line` refresh from someone else's un-regenerated test edit".
