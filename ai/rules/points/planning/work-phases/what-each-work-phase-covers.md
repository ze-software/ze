---
kind: directive
level: MUST
stage:
---
**Ze work has three phases and a piece of work MUST be classified by what it IS rather than by convenience: planning and design (research, spec writing, architecture decisions, RFC reading), implementation (code, tests, fixing failures, refactors, the doc edits that follow), and review and audit (the Review Gate, the implementation audit, spec closure).**
**At a boundary you MUST end the phase and hand off, and you MUST NOT carry it past because you are already here.** Fixes a review produces are implementation, so make them; the re-review that follows is a fresh pass, never the same context re-reading itself.
