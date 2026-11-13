# News fragments

One file per change that operators see, written by the agent that made the
change, at the moment it knows the facts. The weekly update and the release
notes start from these files. The site does not publish them: the changes
pages are built from `website/changes/posts/` only
(`changesSourceDirectory`, `internal/le/site/changes.go`), and the build
trims this directory from the artifact it deploys (`sourceOnlyDirectories`,
`internal/le/site/paths.go`).

`./le weekly report from <rev> to <rev>` reads them too: its `fix-fragments`
section lists the fragments a range adds, grouped by `found-by` with each
one's spec or journal row, and counts the ones with no `found-by` as one
total. A fragment that does not parse fails that section, naming its path.
<!-- source: internal/le/commit/weekly_source.go -- answerWeeklyFragments, weeklyAddedFragments -->

## A fragment is a claim

A fragment records what its author believed when the change landed. It is
never evidence. The weekly writer verifies every claim it uses at the
producing function before it enters a post (`ai/skills/ze-weekly-update.md`,
Phase 1), and a claim that fails is dropped. Write what you checked, not what
you hope.

## When one is owed

| Change | File name | Owed |
|--------|-----------|------|
| A spec closure | `<stem>.md`, using the canonical spec stem without the leading `spec-` or the `.md` suffix | Always, or `no-news "<reason>"` on the commit that removes the spec (`ai/skills/ze-close.md`, step 6) |
| A fix outside a spec | `<journal-class>-<slug>.md`, the journal class file's name and a short slug of the fix | When operators see the fix (`plan/journal/README.md`). No check enforces it |
<!-- source: internal/le/commit/newsfragment.go -- newsFragmentPath, newsFragmentOwed -->
<!-- source: internal/le/commit/review.go -- closedSpecStem -->
<!-- source: internal/le/spec/path/specpath.go -- Stem -->

A closure carrying both a fragment and `no-news` is refused. The `no-news`
reason becomes a `No-news:` trailer; never write that line by hand.

## Front matter

| Key | Required | Value |
|-----|----------|-------|
| `spec` or `journal` | one of the two | The canonical spec stem without `spec-` or `.md`, matching the fragment's filename stem, or the journal row the fix closes (class file and date) |
| `topic` | yes | A tag family from `website/data/topics.json`. Add a missing family there, in the same commit |
| `feature` | no | The feature id under `features/`, when the change belongs to one. Name it; do not restate its level or scope |
| `found-by` | no | For a fix: what surfaced the defect. One of `rfc-check`, `interop`, `test-judge`, `review`, `operator-report`, `upstream` |
<!-- source: internal/le/commit/newsfragment.go -- newsFragmentClosureProblem, newsMetadataProblems -->

The date is read from git, so a fragment carries none.

## The six headings

Each fragment's body carries six fixed headings, in a fixed order, written
`## <Heading>`, and nothing before the first one. Write
`none` under a heading whose fact is truly absent. `none` is refused under
Before and After, because a fragment exists to say what an operator sees
change.

| Heading | What it says |
|---------|--------------|
| Before | What an operator would have hit before the change |
| After | What they see now |
| Corrects | The earlier ze-news text this corrects, by date, or `none` |
| Exposure | For a security fix, which configurations were exposed and which were not, or `none` |
| Upstream | A bug found in another implementation or a library and reported or fixed upstream, or `none` |
| Asked by | Who outside the project asked for it (a GitHub issue, a Discord thread), credited by role, or `none` |

Write for an operator: no spec names, acceptance criteria, agent lessons or
repository vocabulary. Lessons go to the surface that governs them
(`ai/rules/planning.md`).

## The one declaration

`internal/le/commit/newsfragment.go` declares the heading names and their
order, the accepted `found-by` values, and the front matter keys.
`./le commit create` validates every fragment a commit carries against it, and
refuses a closure that carries neither a fragment written by the spec's own
work nor `no-news`. The tables above describe that declaration. Where they
differ, the Go file wins and this page is the defect.
