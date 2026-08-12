# Spec: stale-spec-frame-check

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Report a spec whose claim about the state of the project is older than the code
that claim rests on.

`ai/rules/evidence.md` makes this a reader's duty, under "Claims About the State
of the Project". A document can state that something is unimplemented,
undecided, unreachable or out of scope. Such a statement is evidence of one
author's belief on one day. The reader must verify it against the tree first.

The rule states the duty. Nothing measures it. So a reader finds a stale frame
only by chance. The cost of a miss is a session of competent work. That work
answers a question the tree stopped asking.

**The signal is cheap and already present.** Every spec carries an `Updated`
date in its header table. Specs also cite the files they rest on. A cited file
can carry a commit newer than that date. When it does, and the spec also carries
a decaying claim, the claim is a candidate for a re-read.

**This is a report, never a block.** A spec can be older than its code and stay
correct. The check cannot know whether a change touched the claim. A block would
teach an author to bump `Updated` and read nothing. That gives a fresh date over
stale text, and it destroys the one honest signal the check holds.

**Measured motivation.** One session met five such claims on 2026-08-10:

- a design question recorded as open, which shipped code already answered
- a defect described as test-only, which was a live production fault
- a buffer-budget question, which an existing two-tier pool already answered
- an acceptance criterion premised on a config surface that does not exist
- a task naming two constants that live nowhere, and one wrong kernel value

A reader found each one at a producer. Each one had already redirected work.

## Sketch (not a design)

| Element | Note |
|---------|------|
| Input | every spec under `plan/` and `plan/future/` |
| Claim detection | one reviewable phrase list, held in one place |
| Citation extraction | reuse what `scripts/dev/spec-citation-check.py` parses |
| Staleness test | `git log -1 --format=%cI -- <cited path>` against `Updated` |
| Output | one row per spec: the claim, the phrase, the paths that moved |
| Severity | report only, exit 0 |

## Open questions

- Is the phrase list maintainable? A check nobody reads is worse than no check.
- Must the comparison read only the producer a claim NAMES? A spec that cites
  thirty files will always have one that moved.
- Do `plan/journal/` rows and `plan/deferrals/` shards deserve the same
  treatment? They carry the same claims and hold no `Updated` field.

## Why this is future work

No shipped behaviour depends on it. No defect reaches an operator through it.
It makes an expensive class of mistake visible earlier. It belongs after the
first release, per `plan/future/README.md`.
