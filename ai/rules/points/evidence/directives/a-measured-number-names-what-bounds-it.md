---
kind: directive
level: MUST
stage:
---
**A number you measured is a claim about what ran, and before you report it, act on it, or pick a winner from it, you MUST name the resource that bounds it and rule out that it measured something else.** Ask why it is not twice as good: the answer is a core, a lock, the disk, the network, the load generator or a setting, read from a profile or system counters taken during a run, never guessed from the code. Then rule out the usual substitutes: errors or lost routes, work that never ran inside the timed region, a side left on default settings, noise larger than the gap, and a piece too small to move the end-to-end path. A number reported without its run count, its spread and its limiter MUST be labelled inconclusive. The procedure for a `./le perf` comparison is `docs/guide/benchmarking.md`, "Before you report a number".
