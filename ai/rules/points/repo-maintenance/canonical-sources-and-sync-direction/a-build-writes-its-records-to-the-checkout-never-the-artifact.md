---
kind: directive
level: MUST NOT
stage:
rationale: plan/journal/concurrent-session-corruption.md
---
**A build MUST NOT write its own bookkeeping into the artifact it publishes.** A record of what a run did belongs to the checkout that ran it. The artifact holds what a reader came for and nothing else, so a build MUST resolve a record path from the repository root and never from the output root, whatever `ZE_REPO_ROOT` names at the time.

**The tell MUST be checked by hand rather than waited for from a gate: a path a build writes is joined to `paths.Output`, or to a root a caller supplied, while the thing written is evidence about the RUN rather than content for a reader.** The failure is quiet in the only place that matters, because the record is correct, the build succeeds, and the file is served. `ZE_REPO_ROOT` makes the route reachable from every sibling checkout, so a path that is correct in the ze tree publishes tooling state the moment it is pointed at a published one.
