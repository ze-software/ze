# Spec: a shrink-only baseline cannot see a relocation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Filed in `plan/future/` because it is process tooling, not a release defect: it
matches none of the five defect kinds in `plan/future/README.md`.

## Task

A grandfathering baseline keyed on a file's PATH cannot distinguish a
relocation from new debt, so moving a file reads as growth and a shrink-only
gate refuses it.

`internal/le/doccheck/links.go` grandfathers dead citations as
`citer<TAB>target` pairs in `internal/le/`, and refuses
any pair that is new against HEAD. The rule is right: it stops a session
silencing fresh dead references by appending to the list. What it cannot see is
that a repointed citer is the same debt at a new address, so relocating a file
that carries N grandfathered rows reports N new pairs.

The consequence is the part worth fixing. `plan/future/` is where a spec goes
when it stops blocking the release, and the specs most likely to go there are
the OLD ones, whose plans name files nobody built. Those are exactly the specs
carrying grandfathered rows. So the gate makes `plan/future/` hardest to reach
for the specs that most need it, and the only honest routes are to repair
citations to files nobody ever built, or to leave the spec where it is.

Measured 2026-08-19: relocating one spec reported 17 new baseline pairs and one
dead reference from a deferral shard that named its old path. The move was
reverted rather than forced.

## What a fix has to decide

| Question | Why it is not obvious |
|----------|-----------------------|
| Key the baseline on something that survives a move, or teach the check to net a rename | A content hash of the citing line survives a move and a reflow; a path does not. Netting a rename needs the check to read two trees rather than one |
| Whether a relocation should carry its debt at all | Moving a spec out of the release backlog and keeping its dead citations is arguably the honest outcome, and arguably the thing the shrink-only rule exists to prevent |
| What updates the referrers | A deferral shard naming the old path goes dead on the move, and nothing repoints it |

## Notes

The general practice, which holds with or without the fix: a baseline that
grandfathers debt should be keyed on the debt's identity, not on its current
address, or relocation and accumulation become the same event to it.
