---
name: ze-design-judge
description: Judge a spec's Design documents verdicts against the pages they cite, as an independent agent, before the design is presented to Thomas
---

# Judge a Spec's Design

A spec lists, in `## Design documents`, every design page its files link and
every decision those pages carry, each with a verdict: `agrees`, `departs` or
`silent`. Before the main session presents the design to Thomas, an agent that
did not write the spec judges those verdicts here and records the result
(`docs/contributing/spec-workflow.md`, "Design approval"). Thomas approves
only a design whose current text this judge approved.

**You did not write the spec.** The agent that wrote or revised its design
never runs this skill on it. If you authored any part of the spec's Task,
Acceptance Criteria, Design documents or Design presentation, say so and hand
it back.

See also: `/ze-spec` (writing the spec), `/ze-test-judge` (the same
independence for a test change), `docs/contributing/spec-workflow.md` (what
`ready` means).

## Input

| Item | Form |
|------|------|
| The spec | its path, `plan/<work-folder>/spec-<name>.md` |

You need nothing else: the spec carries the table, the derivation names what
it owes, and the pages hold what each row cites.

## Procedure

Every `./le` command you run starts with your identity line
(`ZE_AGENT_ID=...`): the judge action keys your read records on it and refuses
a process that has none.

1. Run `./le spec design documents spec <path>`. It lists every page and
   decision row the spec owes. A derived row the table lacks is a REFUSE.
2. Run `./le spec design check spec <path>`. Every problem it prints except
   the missing judge approval is a REFUSE: the table or the presentation is
   incomplete.
3. For every row of `## Design documents`, read the section it names with the
   Read tool, the whole section in one Read. A door is read whole. The judge
   action refuses a verdict while any row's section is unread, and prints the
   offset and limit of each.
4. Read the producing code where a row's truth depends on it: the function
   the design changes, not its doc comment (`ai/rules/evidence.md`).
5. Read `## Existing Mechanisms` against the pages: each mechanism the design
   extends is named there.
6. Decide.

## Verdict

**APPROVE** only when all of these hold, stated in one sentence:

- every `agrees` row holds against its section: the design does nothing its
  binding statement forbids;
- every `silent` row's page is silent on what the design changes;
- every departure the design makes from a page is a `departs` row, presented
  under `### Documents departed from` with its before and after;
- the Existing Mechanisms rows name what the design extends.

**REFUSE** otherwise, naming the row and what does not hold: the sentence the
design contradicts, the page that is not silent, the departure with no row.
"The design looks right" and the author's reasoning restated are not an
approval. A refusal stands: the author fixes the design, and you never ask
Thomas.

## Recording the verdict

You record it yourself, approve or refuse, with your verdict sentence as the
reason, one line:

```
./le spec design judge spec <path> verdict approve reason "<verdict sentence>"
./le spec design judge spec <path> verdict refuse reason "<the row and what does not hold>"
```

The record lands in `tmp/spec-design/judged/<stem>.md`, bound to the design
digest of the four sections Thomas approves, so any later edit of them owes a
new judgment. A later verdict replaces the record.

## Report

One row: the spec, APPROVE, REFUSE or HANDED BACK, the verdict sentence, and
the digest the judge action printed (or the reason, for a spec handed back).

## Done when

Done when the verdict is recorded through `./le spec design judge` and the
report gives the verdict sentence and the digest, or the spec is handed back
with the reason.
