---
kind: directive
level:
stage:
---
**A pinned tag does NOT make a line number safe. Measured 2026-08-03: four BIRD anchors in `docs/architecture/congestion-industry.md` pointed at unrelated code at the v3.2.0 tag they named, and a GoBGP anchor was off by six lines.** They were written against a different version and never re-read, so the citation was wrong from the day the dependency moved and nothing could detect it.
