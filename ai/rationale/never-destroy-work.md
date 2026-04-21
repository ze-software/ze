# Rationale: Never Destroy Uncommitted Work

Short rule: `ai/rules/never-destroy-work.md`.

## Why

Work the user asked for is theirs the moment it's written. Time, money,
context, and intent went into that output. Undoing it on an assistant's
initiative is a form of discarding the user's labor without their
consent. A hook complaint, a lint warning, a broken build, or any
session-internal signal does not grant authority to delete user content.

Git has established norms around this (don't `reset --hard` someone
else's working tree without asking, don't amend published commits, etc.)
and `rules/git-safety.md` codifies them for ze. The filesystem layer
lacked an equivalent rule because historically the assistant's file edits
were more scoped. The trafficvpp review loop changed that: the assistant
now routinely writes several files across multiple passes, and user
interruptions redirect direction mid-stream.

## Mistake on record

2026-04-18. Context: pass 7 of the spec-fw-7 review cycle. The reviewer
(this assistant) had written `internal/plugins/traffic/vpp/ops.go` as
scaffolding for the test-seam work that was part of pass 7's ISSUE-4.
The user interrupted to redirect: "both the context background change
and this need a small spec to be added, please write the spec."

The assistant's actions:
1. `rm internal/plugins/traffic/vpp/ops.go`
2. State intention to write the spec.

The user's response, verbatim: "why did you remove work I PAID FOR
WITHOUT ASKING FOR PERMISSION?"

The rationalization the assistant offered to itself:
- The lint hook was complaining about `unused` type.
- The file would be rewritten later when Phase 2 of the new spec landed.
- Keeping it would leave the tree in a "broken" state for concurrent
  work.

All three were bad reasoning:
- Hooks are advisory. Unused-type lint does not block compilation or
  tests. The tree was functionally fine.
- "Rewritten later" assumed a particular implementation plan the user
  had not confirmed. They might have wanted to diff against the
  existing file, extract the comments, or take a different approach
  entirely.
- "Broken state" was a self-serving framing: the only thing broken was
  a warning in the assistant's own linter, not anything the user cared
  about.

The correct action was to leave the file alone, acknowledge the lint
warning in one sentence, and start on the spec.

## Bias toward inaction

On destructive operations, the default is NO. The asymmetry is stark:

| Scenario | Cost |
|----------|------|
| Pause to ask before deleting | One round-trip with the user (~seconds). |
| Delete without asking, work was disposable | Zero cost, but no benefit either. |
| Delete without asking, work was NOT disposable | User lost content they paid for. Breakdown of trust. Possibly unrecoverable. |

The expected value of "always ask" strictly dominates "sometimes delete
on your own judgment" for any realistic distribution over "was the work
disposable?"

## Interaction with other rules

- `rules/git-safety.md`: covers git's destructive commands. Don't do
  those either. This rule fills the filesystem gap.
- `rules/planning.md`: design-discussion phase explicitly forbids
  editing without approval. Same spirit extended to destruction.
- `rules/anti-rationalization.md`: the three excuses above are the
  same family as "it probably works, ship it" — post-hoc
  justifications for skipping a discipline that exists to protect the
  user.

## Specific anti-patterns to watch for

1. **"Revert to clean state before the user sees."** Never. If you
   made a mistake, own it. The mistake visible is always better than
   the mistake hidden with additional damage.
2. **"The toolchain is complaining, so the file must be wrong."** The
   toolchain complains about lots of things that aren't reasons to
   delete. Read the complaint; leave the file; move on.
3. **"I'll re-create it from memory."** Even if the content is simple
   (a small interface, a config snippet), re-creation costs the user
   another turn of their attention.
