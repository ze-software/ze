# Handover: agent-economy (cut token and wall-time waste in the development system)

Written 2026-11-14 by the session that picked the set up after it stalled.
It replaces `plan/handover/2026-11-11-agent-economy.md`, which a cleanup of
closed handovers removed on 2026-11-14 (935398f8df) while three children were
still open. Its content is in git history at that commit's parent.

Thomas's orders this session: review a proposed improvement programme; find
why the umbrella was not closed; "Add all safeguards and then update the specs
so we can pick it"; "We should differentiate the two caps"; "Next, next, next.
Do the work." Nothing is pushed; a push needs his order.

Read first: `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`,
"Where the set stands". This page says where each child stands and what to do
next.

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | Children 1, 2, 6 | Closed 2026-11-11 | None |
| 2 | Child 3, `spec-agent-economy-3-preamble.md` | `in-progress`. Phase 5 (style page) committed, 9d2636d728. Phases 1, 3 and 4 written, reviewed in five rounds and green, see "Uncommitted" below | Land the uncommitted work if it is still in the tree, then run the probe |
| 3 | Child 3, phase 2 (the decisions file in two parts) | Classified, not built: 11 of 135 bullets are main-session only, about 3KB of 45KB. The list is in the session state file named below | Build it with the headings `Rules for every agent` and `Rules for the main session`, the names the reader looks for. One commit, when no other session holds a hunk in `ai/META-DECISIONS.md`. Strongest-tier review of the placement |
| 4 | Child 3, the probe (A-1, A-3, A-4) | Not run. It needs a session started after `./le ai sync update` | Steps are in the spec's phase 1 and in the phase 1 agent's report in the session state file |
| 5 | Child 4, `spec-agent-economy-4-supervision.md` | `design`, written and committed (1a669fbe16). Not approved | Thomas owes two values, below. Then present the design to him whole and move it to `ready` |
| 6 | Child 5, `spec-agent-economy-5-scoped-gates.md` | `skeleton` | Last, by owner decision D-5. Strongest-tier design |
| 7 | The umbrella's criteria | Measured over the two weeks after the last child closes | Child 4's AC-7 builds the kept measure |

## Uncommitted when this was written

The commit of phases 1, 3 and 4 was prepared and refused by a change-set
check that parses the whole tree: another session's
`internal/le/site/homebody.go` did not parse. Nothing in the refusal concerns
this work. The file list is
`tmp/session/2026-11-14-180706/state/preamble-files.txt` and the message body
is `preamble-body.txt` beside it; if that scratch is gone, the files are those
`git status` shows under `internal/le/hookruntime`, `internal/le/ai`,
`internal/le/le/path`, `ai/agents`, `ai/rules`, `.claude/hooks`,
`docs/contributing/rule-authoring.md` and the child 3 spec.

Before committing, claim child 3 (`./le spec claim
spec-agent-economy-3-preamble.md`) so the commit carries its `Spec-work:`
line. The commit also carries one judged test change:
`Weakening-approved: rules.TestThePayloadRefusesToMeasureAFileThatIsNotThere`
(the verdict is recorded in the session's approval file).

Evidence at the time of writing, all run in the main checkout: the packages
`internal/le/hookruntime`, `internal/le/ai/rules`, `internal/le/ai/rules/ruleset`,
`internal/le/ai/sync`, `internal/le/ai/hooks`, `internal/le/le/path`,
`internal/le/job`, `internal/le/spec` and `internal/le/session` pass;
`./le --name session ai hooks unit` passes 30 of 30; the lint is clean on the
changed packages; `./le test weakened check` is clean. The shared `./le ai
hooks unit` reports `hook-source-drift` until the commit lands, because the
shared `le` is built from HEAD and carries the old seal.

Note for closure: 9d2636d728 was committed under the umbrella's claim, so its
`Spec-work:` line names the umbrella and child 3's closure review must be told
to cover it.

## What changed in the plan for child 3

| Finding | Consequence |
|---------|-------------|
| The harness caps a hook's added context at 10,000 characters | The start hook sends the hard bans and names three files to read. It cannot send the rule set. One extra turn per agent, the same tokens |
| Only 11 of 135 bullets of the decisions file are main-session only | The two-part split saves about 3KB, not the share the spec assumed |
| A rule point that states a prohibition cannot be hidden, and most points mix a prohibition with a procedure in one paragraph | Two points carry `applies`. The core is 45.0KB, from 45.5KB. A real saving needs about twenty points split at sentence boundaries, each reviewed, for an estimated 13KB |
| The style page split | The largest saving: 52KB to 16KB for every agent that edits Go. Landed |
| The skill listing (28KB) | Still an unverified assumption (A-3). The probe settles it |

AC-8 (core at most 24KB) and AC-13 (first turn at most 35k tokens) are not
reachable as written. Changing them is the owner's decision.

## Open with Thomas

| Question | Recommendation |
|----------|----------------|
| The `applies` mechanism: do the twenty point splits, stop at what is built, or remove it | Land what is built, do not split now, decide with the probe's measurement |
| Let whole topic sections of the decisions file be read only in their situation (website 5.2KB, test systems and labs 3.1KB, ExaBGP 1.1KB, developer tooling 0.3KB) | Yes. It departs from the approved two-part design, so it is his |
| Child 4: the open limit's starting value. Today's cap of 12 refuses every claim on the first day, since started sets add to the 10 specs in progress | Start at the count measured on the day it lands, lowered as specs close |
| Child 4: the active limit's value | 6 distinct specs under a live claim |

## Decisions taken (do not put them to Thomas again)

| Point | Decision | Why |
|-------|----------|-----|
| Two limits | An open limit (everything started and not closed) and an active limit (live claims). Recorded in `ai/META-DECISIONS.md`, "How agents work" | His words, 2026-11-14 |
| A set between children | Counts once in the open limit | Accepted 2026-11-14 |
| The stop refusal of child 4's AC-11 | No acknowledgement route | Accepted 2026-11-14 |
| Delivery of the agent rule set | The spec's own fallback for a false A-2 | The documented cap |
| The id check in `internal/le/spec/model.go` | Left without a length bound | It also validates spec names; the reviewer confirmed no refused id reaches a file name or a claim through it |
| Byte-identical rule files | The owner's condition is that no rule's wording changes; a split adds a blank line, so the check is a word-level comparison | Recorded in child 3's phase 3 |

## What every brief must say

- A subagent runs no test, build, lint, formatter or generator. It ends its
  report with the owed commands, and the main thread runs them.
- After an agent's change, run the tests of every package that imports or
  pins what it touched, not only the packages it names. Twice this session a
  change passed its own package and broke a neighbour nobody had run.
- A pinned count or a stored digest is updated by what was added, derived from
  the rows themselves, never copied from what the test now computes.
- A commit body written by a subagent goes through `body-file`.

## Session state

`tmp/session/2026-11-14-180706/state/session-state-agent-economy-0-umbrella-972de0b2-472a-4cf3-9820-6385b6ff38e0.md`
and the child 3 state file beside it hold the agents' handoffs. They are
untracked scratch and may be reaped; this page is the durable record.
