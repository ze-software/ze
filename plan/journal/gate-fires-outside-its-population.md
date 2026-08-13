# gate-fires-outside-its-population

The mirror of `gate-excludes-part-of-its-population`. A gate's trigger predicate
matches a population WIDER than the one it judges. So it refuses work it was
never meant to see. The usual cause is a policy change elsewhere that made an
old proxy signal common.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-13 | - | commit helper, review gate | `spec_closure_stem` (`scripts/dev/commit_helper.py`) treats a NEW journal row that names a spec as a spec-closure signal. `review_gate_problems` then refuses the commit until an INDEPENDENT review artifact exists. That proxy was sound when a journal row appeared only at closure. CLAUDE.md now mandates one row for every defect an agent finds outside the work in hand. So an ordinary mid-spec commit that carries a journal row is read as a closure of a spec that stays open. The author is told to run a review phase they are not in. Met during RFC 7999 enrolment work, which closed nothing. The two other closure signals are precise, and neither fired: a new `plan/learned/` summary, or `git rm` of the spec file | worked around, not fixed. The row's Spec cell was set to `-`, and the spec named in the row prose instead. That contradicts `plan/journal/README.md`, where `-` means the defect was found OUTSIDE a spec. So the workaround costs the attribution the column exists for. The next agent who meets a defect mid-spec will reach for it again. A real fix reads the spec's own Status, or requires a closure signal a mid-spec row cannot produce |
