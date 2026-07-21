# Rationale: Deferral Tracking

Deferrals are promises that rot. Without formal tracking in the sharded
`plan/deferrals/` directory (one file per source) with a destination spec, a
deferred item exists only as a paragraph in a learned summary that no future
session will read proactively.

The stale-deferrals mistake (redist-phase2) happened because a phase-N
spec was created from open deferrals without first checking whether the
deferred code had already been written by a different session. The spec
duplicated work that was already done, wasting an entire session.

Formal tracking prevents both loss and duplication:

- Loss: an untracked deferral is forgotten the moment the session ends.
  The next session sees the spec as closed and the feature as done.
  Nobody picks up the deferred item because nobody knows it exists.

- Duplication: a tracked deferral with a destination spec can be grepped
  before creating a new spec. If the code already exists, the deferral
  is closed. If not, the new spec inherits the deferral's context.

The verify-before-deferring rule exists because sessions rationalize
deferrals to avoid hard work. "Deferred to next spec" is the easiest
way to claim completion without doing the work. Requiring verification
that the deferral is genuine (not just avoidance) catches this at the
moment of temptation.
