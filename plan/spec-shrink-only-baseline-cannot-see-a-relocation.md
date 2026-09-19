# Spec: a shrink-only baseline cannot see a relocation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Originally filed in `plan/future/` on 2026-08-19 as process tooling. That path
and the bucket rationale below describe the filing context.

## Task

Determine whether any currently checked relocation still needs debt-identity
handling. The original spec-relocation case below no longer establishes pending
implementation work: `citationExcludes` in `internal/le/doc/check/links.go`
derives a `spec-` exclusion for every bucket from `specpath.Dirs`, and
`sweepTracked` skips citation inspection for those paths. This follows the
record-preservation rule in `ai/rules/writing.md`.

The baseline still keys rows by citer and target. Before a broader change is
designed, identify a currently included non-spec population and reproduce the
relocation refusal there. The owner can retain that broader scope or consider
the original blocker resolved through normal review. Neither decision, nor
closure, is recorded here.

## Original Problem, 2026-08-19

A grandfathering baseline keyed on a file's PATH could not distinguish the
measured relocation from new debt.

`internal/le/doc/check/links.go` grandfathers dead citations as
`citer<TAB>target` pairs in `internal/le/`, and refuses
any pair that is new against HEAD. The rule is right: it stops a session
silencing fresh dead references by appending to the list. What it cannot see is
that a repointed citer is the same debt at a new address, so relocating a file
that carries N grandfathered rows reports N new pairs.

At filing, `plan/future/` was where a spec went when it stopped blocking the
release. Old specs carried grandfathered references to files that had never
been built, so moving those specs raised baseline-growth findings. This was
the motivating cost; it is no longer a current consequence for bucketed specs.

Measured 2026-08-19: relocating one spec reported 17 new baseline pairs and one
dead reference from a deferral shard that named its old path. The move was
reverted rather than forced.

## Decisions Retained for Any Broader Baseline Change

| Question | Why it is not obvious |
|----------|-----------------------|
| Key the baseline on something that survives a move, or teach the check to net a rename | A content hash of the citing line survives a move and a reflow; a path does not. Netting a rename needs the check to read two trees rather than one |
| Whether a relocation should carry its debt at all | Specs now preserve historical citations outside this check. Any broader proposal must name the currently checked population and explain why its debt should move |
| What updates live referrers | A live document can still need a reference update when its target moves. Historical record paths follow `ai/rules/writing.md` and must not be rewritten as current claims |

## Notes

The general practice, which holds with or without the fix: a baseline that
grandfathers debt should be keyed on the debt's identity, not on its current
address, or relocation and accumulation become the same event to it.
