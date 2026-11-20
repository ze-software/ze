# Handover: agent-economy (cut token and wall-time waste in the development system)

First written 2026-11-14 by the session that picked the set up after it
stalled, and brought up to date on 2026-11-15 by the same session. It replaces
`plan/handover/2026-11-11-agent-economy.md`, which a cleanup of closed
handovers removed on 2026-11-14 (935398f8df) while three children were still
open. Its content is in git history at that commit's parent.

Thomas's orders: review a proposed improvement programme; find why the
umbrella was not closed; "Add all safeguards and then update the specs so we
can pick it"; "We should differentiate the two caps"; "Next, next, next. Do the
work."; "keeping what is built"; "1 yes, 2 measured, 3 ok 6"; and on
2026-11-15, "you have a handover to continue". Nothing is pushed; a push needs
his order.

Read first: `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`,
"Where the set stands", then run `./le spec status set list`, which derives
the set's state, its next child and its handover. Everything this session
wrote is committed.

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | Children 1, 2, 6 | Closed 2026-11-11 | None |
| 2 | Child 3, spec-agent-economy-3-preamble | Closed 2026-11-20 (commit A 588464afb8) | None |
| 3 | Child 4, spec-agent-economy-4-supervision | Closed 2026-11-20 under the owner's scope cut: packages 1 to 4, 8, 10, 11 and AC-1 landed; packages 5 to 7 and the rest of 9 were dropped | None. `./le spec status set list` now prints where this set stands |
| 7 | Child 5, `spec-agent-economy-5-scoped-gates.md` | `skeleton` | Last, by owner decision D-5. Strongest-tier design |
| 8 | The umbrella's criteria | Measured over the two weeks after the last child closes | `./le ai tokens report from <date> days 14` prints them |

The probe brief for item 3, for the temporary agent in a fresh session:

> Before any tool call, say whether a listing of available skills is in your
> context and roughly how many entries it has, and list every block of text
> you were given at start with a rough size for each. Then read
> `ai/skills/ze-audit.md` and say whether you could follow it.

Two of the three probes are answered. A `ze-read` agent started without the
instruction files and without the decisions file, received the rule-set
section, and read exactly the range the hook named. An agent followed a skill
by reading its file. The skill listing was present, about 70 entries, the
largest block of an agent's start.

## What landed on 2026-11-15

| Commit | What |
|--------|------|
| 143da2e5c3 | Four sections of the decisions file bind only the agent doing that work; an agent's rule set went from 23,772 to 21,335 tokens |
| 81ace1d673 | One function decides what a spec set is and which file leads it |
| 3909e97f96 | Skill bodies: reasons moved to rationale pages, the review limit stated, one judge for several units |
| df9e35bd6b | The token report measures the umbrella's criteria and baseline rows over a window |
| ff4e834449 | A record of closed specs, and a set lists its closed children |
| 2c82fd85e3 | A claim is live, dead or cannot be told; leaked markers are reaped |
| 7fdf877380 | An agent spawn past 600k of context tells the main thread to hand off |
| 18600e8905 | `./le ai sync check` reports the rule payload each agent definition starts with; the seal refreshed for the four before it |

## What the measurements say

| Finding | Consequence |
|---------|-------------|
| An agent's rule set is 21,329 tokens; the four situational sections are 2,713 more, read only for their work | The budget of 12,000 set for it has no basis |
| Over 2026-11-01 to 2026-11-15 a subagent's first call carried a median 56.9k tokens | Those weeks are almost all before the lean start; judge the 35k target on the weeks after 2026-11-15 |
| 2.5% of editing agents reached the 100-call budget; the largest main session peaked at 967k | The umbrella's baselines have comparable figures now |
| The four skill bodies shrank by 0.8 to 1.9KB each, not the 6 to 20KB the spec guessed | Nearly all their text is instruction, and instructions stayed |
| The skill listing is the largest block of an agent's start | The last probe says whether it can go |

## Decisions taken (do not put them to Thomas again)

| Point | Decision |
|-------|----------|
| Two limits | An open limit (everything started and not closed) and an active limit (live claims). In `ai/META-DECISIONS.md`, "How agents work" |
| The open limit's starting value | The open count measured on the day it lands, lowered as specs close |
| The active limit | 6 distinct specs under a live claim |
| A set between children | Counts once in the open limit |
| A claim that only closes a spec | Outside the open limit, because it starts nothing; the setting that lifted the cap for one claim goes away in package 5 |
| The `applies` mechanism | Kept as landed. Points are not split. The 24KB core target is withdrawn |
| Sections of the decisions file read only in their situation | Done, for website design, test systems and labs, ExaBGP compatibility and developer tooling |
| What forms a spec set | The spec files alone. The record of closed specs only adds closed children to a set the files form |
| When the reaper may call a session dead | Only on proof, and never when anything of it was written within a day; with no Claude process visible it refuses to judge |

## Open with Thomas

| Question | What each answer changes |
|----------|--------------------------|
| The stop refusal for a half-done spec (child 4, AC-11). The harness fires its stop event at the end of every turn, so as specified it refuses a session that only pauses to ask a question | Keep it a warning (recommended: the stalled-set report and the undeletable handover cover the failure that happened); refuse once then pass; or refuse always, which costs a committed handover before every mid-spec question |
| The notices at 60 and 80 calls for an editing agent (child 4, AC-3). Written as a patch, not applied: it needs two hunks in `internal/le/hookruntime/runtime.go`, the permission layer refused that edit to the agent, and the session did not apply a refused edit on an agent's request | Say yes and a session applies it with a review. The patch was left in the session scratch (`ac3-budget-notice.patch`, `ac3-docs.patch` under `tmp/session/2026-11-14-180706/scratch/`), which the reaper removes once the session is gone; it is about forty lines and can be rewritten from AC-3 |
| The budget for an agent's rule set (child 3, AC-12) | 12,000 was picked without a basis; the measured set is 21,329 tokens |
| The first-turn target (child 3, AC-13), 35k | Set it once the report over the weeks after 2026-11-15 gives the figure |

## What every brief must say

- A subagent runs no test, build, lint, formatter or generator. It ends its
  report with the owed commands, and the main thread runs them.
- After an agent's change, run the tests of every package that imports or
  pins what it touched, not only the packages it names.
- A pinned count or a stored digest is updated by what was added, derived from
  the rows themselves, never copied from what the test now computes.
- A change to a file under the hook source seal needs an independent review
  that answers yes before `hookSourcesDigest` is refreshed
  (`.claude/hooks/README.md`). The candidate value is printed by
  `TestEveryStoredDigestMatchesTheTreeItRatchets`. Several sealed changes in
  flight share one digest: land them in a series and refresh it in the last.
- A test change the audit reads as a weakening (a moved test, a new test with
  a skip, a fixture aged) goes to an independent judge, and the verdict is
  recorded with `./le test weakened approve` before the edit. The approval is
  pinned to the file's content: a later edit to the same file makes it stale
  and it is recorded again.
- The lint scope is a path: `./le go lint run scope "./internal/le/<pkg>/"`.
- The shared `./le` is built from HEAD. A working-tree change is run with
  `./le --name session <words>`.
- An editing agent has 100 calls. A fix round after a long first round goes to
  a fresh agent.

## Notes for closure

- Child 3: 9d2636d728 was committed under the umbrella's claim, so its
  `Spec-work:` line names the umbrella; a9258af3c0 carries
  `Weakening-approved: rules.TestThePayloadRefusesToMeasureAFileThatIsNotThere`.
  The review record is in the spec's `## Review Gate` and in its phase 6 and
  R-6 entries.
- Child 4: `spec-le-session-clean-preserves-active-harnesses.md` owns the same
  surface as package 3 and is partly met by it; amend it when package 3's
  work is closed.
- Reds seen that are not this work's: `TestNumberParseAllowlistMatchesThisTree`
  in `internal/le/repo` (a VPP evidence file missing from an allowlist), two
  `./le cli grammar` violations on `arch error-inventory`, and older lint
  findings in `internal/le/commit` and `internal/le/ai/hooks`.
