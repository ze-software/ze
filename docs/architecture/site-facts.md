# Published Facts Come From Committed Data

<!-- source: internal/le/site/facts/sitefacts.go -- derive, write, factsFile -->
<!-- source: internal/le/site/facts/staleness.go -- check, compare, render -->
<!-- source: internal/le/site/facts.go -- factsFromRepositoryFile -->
<!-- source: internal/le/verify/engine/stages.go -- the site facts check stage -->

The website publishes numbers about this repository: how many Go packages it
holds, how many file headers explain themselves, how many peers it is tested
against. Each number is a claim, and this page states where a claim like that
must come from.

## The defect this page exists to remove

Several sessions share one checkout of ze. A tool that counts the working tree
counts every session's work in progress, and the output says nothing about it.
Two builds one minute apart then publish two different numbers for one
repository, and a reader cannot tell which is the repository and which is a
half-written file on somebody's disk.

`plan/journal/concurrent-session-corruption.md` records the class. Seventeen
generation targets in this repository derive from the working tree. This is the
one that was fixed first, and the pattern below is written so the other sixteen
have a template.

## The pattern

A published number is derived ONCE, deliberately, by a person standing in the
tree they are about to commit. The result lands in a committed file. Everything
downstream reads that file and never counts for itself.

| Step | What it is here |
|------|-----------------|
| One derivation | `derive` (`internal/le/site/facts/sitefacts.go`) is the only counter. The action that WRITES and the action that CHECKS both call it, because two counters over one tree drift by construction |
| One committed file | `website/data/repo-facts.json`, named by `factsFile` |
| One reader | `factsFromRepositoryFile` (`internal/le/site/facts.go`) fills the published snapshot from that file and refuses a fact the file does not carry |
| One gate | `./le site facts check`, a stage of `./le verify` |
| One fix | `./le site facts update`, named in every report the gate prints |

The commands are `./le site facts update` and `./le site facts check`. The pair
is the shape `./le test-health update` and `./le test-health check` already had,
and the pair is the point: a generated file nobody gates goes stale in silence.

## Not every fact can be committed

A published fact belongs to one of five categories, and the file records the
category of each one so a reader can tell what kind of claim a number is.

| Category | What it is a claim about | Example | Committed? |
|----------|--------------------------|---------|------------|
| Committed data | a commit | `repo.go_packages` | Yes |
| Built binary | a binary somebody built | `cli_commands`, from `ze help command --json` | No. Recorded under the `live` key, with no value and with the command that answers it |
| Network | a remote service | `github_stars`, from api.github.com | No. The build carries the previously published value when the reach fails |
| Site-owned | the website's own tree | the blog article count | Not about this repository at all |
| Already gated | a committed artifact another gate owns | the RFC requirement counts | No copy. A second record of one fact is the drift this pattern removes |

The rule that sorts them: **move a fact when its derivation WALKS, and leave it
where it is when it READS one artifact that is itself committed and gated.**
`liveFacts` (`internal/le/site/facts/sitefacts.go`) names the two facts this
tool cannot derive, so a number a reader cannot find in the file reads as
uncommitted rather than as forgotten.

## Two properties make the gate answerable

**It judges a COMMIT, never the working tree.** A check that read the tree would
answer differently in two sessions of one checkout, which is the defect it
exists to catch rather than a way to look for it. `check`
(`internal/le/site/facts/staleness.go`) materializes HEAD in a throwaway
worktree with `git worktree add --detach` and derives there.

**It judges the PUBLISHED figure, not the exact count.** A count reaches a page
through a formatter that floors a magnitude to one tenth of its visible unit, so
3852 and 3899 publish one string. Exactness is also not something the commit
flow can deliver: `git ls-files` answers from the index, so a regeneration run
before the files are staged cannot count what that same commit adds, and a gate
demanding exact equality would go red on the commit that fixed it. `render`
(`staleness.go`) is that formatter, and it MUST move whenever the site's own
display rule moves.

## The dirty-tree warning

`warnUncommitted` (`internal/le/site/facts/actions.go`) names every Go file the
tree and the last commit disagree about, before the file is written. This is the
half that stops the tool becoming the defect it removes. Without it, a
regeneration records another session's uncommitted edit in a committed file, and
nothing in that file says where the number came from.

## Applying this to another generated file

1. Find the walk. A derivation that reads ONE committed artifact is already
   committed data and needs nothing.
2. Give the walk one home, called by both the writer and the checker.
3. Write the result to a committed file, with a category and a source sentence
   per fact, so a reader can re-derive a number by hand.
4. Register a `<area> update` action with `Writes: true` and a `<area> check`
   action without it (`internal/le/leaction`).
5. Warn on a dirty tree before writing.
6. Add the check as a stage of `./le verify`, so it runs where every other
   generated-file gate runs.
7. Make the check judge a commit, and compare what a reader sees rather than
   what the tool counted.

## Related

- `plan/journal/concurrent-session-corruption.md` -- the class, and its count
- `docs/contributing/running-commands.md` -- how a `./le` gate is run and read
- `ai/rules/repo-maintenance.md` -- a generated file and its source move in one commit
