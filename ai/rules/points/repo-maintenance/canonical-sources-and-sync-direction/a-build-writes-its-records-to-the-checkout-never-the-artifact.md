---
kind: directive
level: MUST NOT
stage:
rationale: plan/journal/concurrent-session-corruption.md
---
**A build MUST NOT write its own bookkeeping into the artifact it publishes.** A record of what a run did belongs to the checkout that ran it. The artifact holds what a reader came for and nothing else, so a build MUST resolve a record path from the repository root and never from the output root, whatever `ZE_REPO_ROOT` names at the time.

The failure is quiet in the only place that matters: the record is correct, the build succeeds, and the file is served. Three instances landed in one day, from three different producers. `plan/verification-debt/c7beceff.md` is committed in the `gh-pages` tree and live on ze-software.net, where no route map explains it and no source produces it. `plan/verification-debt/d97ae77d.md` reached the wiki through `ZE_REPO_ROOT=../wiki ./le commit create`, into a repository that had no `plan/` directory at all. A producer record was heading for `data/site-producers.json` inside the published tree before review moved it to `tmp/site` under the repository.

The tell is the same each time, and it is worth checking by hand rather than waiting for a gate: a path a build writes is joined to `paths.Output`, or to a root a caller supplied, when the thing being written is evidence about the RUN rather than content for a reader. `ZE_REPO_ROOT` makes this reachable from every sibling checkout, so a route that is correct in the ze tree publishes tooling state the moment it is pointed at a published one.
