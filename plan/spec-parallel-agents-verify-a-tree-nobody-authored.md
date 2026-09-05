# Spec: parallel agents verify a tree nobody authored

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Filed in `plan/future/` because it is process tooling, not a release defect: it
matches none of the five defect kinds in `plan/future/README.md`. It costs
accuracy in every session that runs more than one editing agent, so it is worth
doing, and it does not hold the first release.

## Task

Verification that reads a shared mutable workspace cannot bound what it owns.
Every red it meets is therefore attributed to somebody else, and the
attribution is a guess.

A session that runs several editing agents at once in one checkout gives each
agent a tree its siblings are mid-edit in. A working tree records no author, so
an agent cannot separate a sibling's half-finished refactor from a foreign
session's uncommitted work. The failure is not that the build breaks. The
failure is that the break is filed as someone else's, which converts a red the
session owns into a red it may override.

The same shape reaches the commit gate. A full `./le verify current mode full`
certifies the tree as it stands when the run FINISHES, so a run that spans a
sibling's edits certifies content its early stages never saw. That half is
already recorded at `plan/journal/reference-checked-claim-unchecked.md` and
`plan/journal/concurrent-session-corruption.md`; this spec is the agent-fan-out
half of the same condition.

The practice that holds without any tooling, and which the rules should state:
an agent that CHANGES files verifies against a workspace whose contents it
controls, and a red is never attributed to another session without evidence
naming the author.

## What a fix has to decide

| Question | Why it is not obvious |
|----------|-----------------------|
| Isolated worktree per editing agent, or serialised editing agents | A worktree costs setup time and disk per agent, and it splits the build cache. Serialising costs wall-clock and removes the reason to fan out at all |
| Who decides | `CLAUDE.md` forbids an agent from spawning a worktree agent on its own initiative, so a default of "isolate every editing agent" is an owner decision, not an implementer's |
| What a verification result should carry | A verdict that named the file set it covered, and the tree hash it read, would let a reader judge attribution instead of trusting it |
| Whether read-only agents need it | They do not change files, so they cost nothing here. Only the editing fan-out creates the condition |

## Notes

Do not treat this as a rule-writing task. `ai/rules/evidence.md` already
forbids the unevidenced claim, and restating it buys nothing. What is missing
is a workspace or a verdict shape that makes the claim checkable.
