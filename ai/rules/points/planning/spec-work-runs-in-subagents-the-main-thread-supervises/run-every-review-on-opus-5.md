---
kind: directive
level:
stage:
---
**Review still runs on Opus 5, and that half is unchanged.** `review_gate.py record`
refuses to record a review performed off it, and `.claude/hooks/pretool-agent-skill.py`
refuses to spawn one. Those remain, because a review's worth depends on the
judgment behind it in a way that writing a test does not.
