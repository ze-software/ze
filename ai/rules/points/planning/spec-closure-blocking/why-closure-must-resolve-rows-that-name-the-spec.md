---
kind: note
level:
stage:
---
Why: closure DELETES the spec, and `deferral_unassigned_problems`
(`scripts/dev/commit_helper.py`) checks that every live row's destination exists on
disk. A row left pointing at a closed spec can therefore never be satisfied: it
dangles forever, is reported on every future commit (as a WARNING: that gate is
advisory and does not block), and the next reader cannot tell whether the work was
done or silently lost. Advisory is exactly why it persists: the six rows homed on
2026-08-03 had been reported on every commit for 17 days at no cost to anyone. The two rules collided precisely because
neither side was written down: "destination must exist" and "closure deletes the
spec" are both right, and closure is the side that must give.
