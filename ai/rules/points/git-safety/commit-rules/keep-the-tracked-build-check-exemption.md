---
kind: directive
level: MUST NOT
stage:
---
**Thomas ruled on this exemption on 2026-08-04: KEEP IT.** It was raised twice as
a narrowing of the fast path, because it adds about 45 seconds to a commit that
carried Go. It is settled, so you MUST NOT re-open it. The reasoning he accepted: the
check is not a rerun, since its input is a commit that did not exist until the
script ran, and it is the only thing that reads the population git holds. The
failure it prevents is unbounded where its cost is bounded and one-shot. HEAD was
unbuildable for 34 commits across more than a day (`eae57dfca`, 2026-08-03, to
`7abe8a07e`) precisely because the break was only discoverable at a full verify
that nobody in that window ran.
If scope is ambiguous, ask one narrow question; otherwise proceed.
