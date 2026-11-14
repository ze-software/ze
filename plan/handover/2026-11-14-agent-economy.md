# Handover: agent-economy (cut token and wall-time waste in the development system)

Written 2026-11-14 by the session that picked the set up after it stalled.
It replaces `plan/handover/2026-11-11-agent-economy.md`, which a cleanup of
closed handovers removed on 2026-11-14 (935398f8df) while three children were
still open. Its content is in git history at that commit's parent.

Thomas's orders this session: review a proposed improvement programme; find
why the umbrella was not closed; "Add all safeguards and then update the specs
so we can pick it"; "We should differentiate the two caps"; "Next, next, next.
Do the work."; "keeping what is built"; "1 yes, 2 measured, 3 ok 6". Nothing
is pushed; a push needs his order.

Read first: `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`,
"Where the set stands". This page says where each child stands and what to do
next. Everything this session wrote is committed.

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | Children 1, 2, 6 | Closed 2026-11-11 | None |
| 2 | Child 3, `spec-agent-economy-3-preamble.md` | `in-progress`. Phases 1 to 5 implemented, reviewed and committed: 9d2636d728 (style page), a9258af3c0 (start hook, reader, `applies`, payload measure), dccb8730d9 (decisions file in two parts). The agent definitions are synced | The probe, then phase 6. See "Remaining before closure" in the spec |
| 3 | Child 3, the probe (A-1, A-3, A-4) | Not run. Agent definitions load at session start, so it needs a session started after 2026-11-14 23:50 | Spawn `ze-read` with the brief below and record the answers in the spec |
| 4 | Child 3, phase 6 (situation-scoped sections of the decisions file) | Not started. Owner approved 2026-11-14. In the spec as phase 6 with AC-15 | Implement, with a strongest-tier review that no bullet in the four sections binds an agent doing other work |
| 5 | Child 3, smaller items owed before closure | R-6: `./le ai sync check` does not report the agent payload per definition. The agent rule-set budget (12,000 tokens) and AC-13 (35k first turn) have no basis yet | Build R-6. Set both figures from the probe, and put the AC-13 target to Thomas with the measurement |
| 6 | Child 4, `spec-agent-economy-4-supervision.md` | `ready` (74fb90daa5). Design shown to Thomas, both limit values settled | Claim it and run `/ze-implement`. Its AC-7, the kept measure, is what the umbrella's criteria are measured with |
| 7 | Child 5, `spec-agent-economy-5-scoped-gates.md` | `skeleton` | Last, by owner decision D-5. Strongest-tier design |
| 8 | The umbrella's criteria | Measured over the two weeks after the last child closes | After child 4's AC-7 |

The probe brief, for `ze-read` in a fresh session:

> Before any tool call, say whether "Ze - Agent Instructions" and "Owner Meta
> Decisions" are in your context, quote the last line of the "Agent rule set"
> section of your start text, and give your context size in tokens. Then make
> the reads that section names and give your context size again.

For A-3 and A-4, a further fresh session with a temporary definition that has
no `Skill` in its tools, synced: compare the first-turn size and whether a
skill listing is present, then have it follow `ai/skills/ze-audit.md` by
reading the file.

One assumption is already answered by observation: a review agent spawned
after the hook landed reported that the line range the hook printed was the
text it received, so the hook's start text reaches an agent.

## What the session learned about the savings

| Finding | Consequence |
|---------|-------------|
| The harness caps a hook's added context at 10,000 characters | The start hook sends the hard bans and names what to read. One extra turn per agent, the same tokens |
| Only 11 of 135 bullets of the decisions file are main-session only | The two-part split saves about 4KB of 45.7KB |
| A rule point that states a prohibition cannot be hidden, and most points mix one with a procedure | Two points carry `applies`; the core is 45.0KB. The owner chose to keep the mechanism and not split points, and withdrew the 24KB target |
| The style page split | The largest saving: 52KB to 16KB for every agent that edits Go |
| What an agent loads in rules, measured by `./le ai rules report payload` | 23,772 tokens: bans 652, decisions 10,399, trigger index 1,461, core 11,258. Phase 6 takes about 2,500 off the decisions |
| The skill listing (28KB) | Still unverified (A-3). The probe settles it, and it is the largest lever left |

## Decisions taken (do not put them to Thomas again)

| Point | Decision |
|-------|----------|
| Two limits | An open limit (everything started and not closed) and an active limit (live claims). In `ai/META-DECISIONS.md`, "How agents work" |
| The open limit's starting value | The open count measured on the day it lands, lowered as specs close |
| The active limit | 6 distinct specs under a live claim |
| A set between children | Counts once in the open limit |
| The stop refusal of child 4's AC-11 | No acknowledgement route |
| The `applies` mechanism | Kept as landed. Points are not split. The 24KB core target is withdrawn |
| Sections of the decisions file read only in their situation | Yes, for website design, test systems and labs, ExaBGP compatibility and developer tooling. It departs from the two-part design of 2026-11-11, and he approved the departure |
| Delivery of the agent rule set | The spec's own fallback for a false A-2 |
| The name check in `internal/le/spec/model.go` | Left without a length bound: it also validates spec names |

## Open with Thomas

Nothing blocks the next session. One question is owed to him later, with its
evidence: the first-turn target (AC-13), once the probe has measured it.

## What every brief must say

- A subagent runs no test, build, lint, formatter or generator. It ends its
  report with the owed commands, and the main thread runs them.
- After an agent's change, run the tests of every package that imports or
  pins what it touched, not only the packages it names. Three times this
  session a change passed its own package and broke a neighbour nobody had
  run.
- A pinned count or a stored digest is updated by what was added, derived from
  the rows themselves, never copied from what the test now computes.
- A change to a file under the hook source seal needs an independent review
  that answers yes before `hookSourcesDigest` is refreshed
  (`.claude/hooks/README.md`). The candidate value is printed by
  `TestEveryStoredDigestMatchesTheTreeItRatchets`.
- The shared `./le` is built from HEAD. A working-tree change is run with
  `./le --name session <words>`, and after a commit the first call starts a
  detached build and still answers from the previous one.

## Notes for closure of child 3

- 9d2636d728 was committed under the umbrella's claim, so its `Spec-work:`
  line names the umbrella. The closure review must be told to cover it.
- a9258af3c0 carries `Weakening-approved:
  rules.TestThePayloadRefusesToMeasureAFileThatIsNotThere`.
- The review record is in the spec's `## Review Gate`: five rounds for phases
  1, 3 and 4 (7, 4, 2, 1 and 0 issues), one round for phase 2, one review of
  the style page split.
- Six lint findings in `internal/le/ai/hooks` are older than this work and in
  files it did not touch.
