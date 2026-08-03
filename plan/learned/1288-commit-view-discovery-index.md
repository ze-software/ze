# 1288 -- The discovery-index gate judges the commit, not the working tree

## Context

`commit_helper.py`'s discovery-index gate ran each generator's `--check` against
the WORKING TREE and blocked when any index was stale. In a repo where several
agent sessions share one checkout, that measurement is confounded: a concurrent
session's uncommitted `plan/learned/NNN-*.md` makes `ai/LEARNED-FULL-INDEX.md`
look stale to everyone, and the gate's own remediation ("regenerate and include
the index") is then actively wrong -- it commits an index row pointing at a file
absent from HEAD, which is green locally and red for everyone else.

This was not theoretical. Commit `afb617952` hit it and had to take
`--stale-index-ok`, and HEAD at that moment already carried the damage from an
earlier commit that had followed the bad remediation: the committed index
referenced `plan/learned/1282-...md`, which was never committed.

The gate was wrong in BOTH directions, and only the first is obvious:

| Working tree | Commit | Old gate | Reality |
|--------------|--------|----------|---------|
| stale | coherent | blocks | false positive; a concurrent session's files |
| fresh | incoherent | passes | the index was regenerated WITH those files and now carries rows for paths absent from HEAD |

The second row is how `1282` got in. A first attempt at this fix handled only the
first row, and a probe on the real tree showed the second row still sailing
through -- committing an index with two foreign rows was allowed.

## Decisions

- **Judge the tree the commit PRODUCES**, `HEAD + adds - removes`, rather than
  the working tree. That is the only tree whose coherence the commit can be held
  responsible for, and it is exactly what CI checks out.
- **Materialize the view and run the REAL generators against it**, via a new
  `--root` option, instead of reimplementing their input-gathering inside the
  gate. A reimplementation would have been cheaper and would have drifted from
  the generators the moment either side changed.
- **`--root` lives in `discovery_sources.py`** (`root_from_argv`), the module
  that already owns the shared source-to-index map, so all three generators pick
  it up identically.
- **Verify the union of "working tree says stale" and "this commit touches it".**
  A fresh working tree says nothing about an index whose content the commit is
  changing, so the fed-set (via `indexes_fed_by`, already the shared source map)
  must be checked too. The old "fresh" branch only asked whether a fed index was
  OMITTED, never whether an INCLUDED one was right.
- **Cheap check first, commit view only when something is at stake.** The
  working-tree `--check` still runs first (~2s for all three) and the view is
  built only when an index is stale or this commit touches one, so a commit that
  feeds no index costs what it always did. Building the view is ~1.5s:
  `git archive HEAD ':(exclude)vendor'` (98 MB rather than 138 MB, and every
  generator skips `vendor` anyway).
- **Two distinct messages, because the two failures need different fixes.**
  "omitted" means run the generator and add the file; "included but wrong" means
  the file you added was generated from a tree holding sources this commit does
  not contain, and must be regenerated from HEAD plus your own files.
- **Fail closed.** If the view cannot be built, every index stays stale and the
  commit blocks. A gate that cannot evaluate must deny, not wave through
  (`ai/rules/evidence.md`).
- **Say something when downgrading.** When the working tree is stale but the
  commit view is coherent, the gate prints which index and why rather than
  passing silently, so the confounding is visible in the commit output.

## Consequences

- A session can commit while another session has uncommitted index-feeding
  files. Neither `--stale-index-ok` nor a cross-committed index row is needed
  for that case any more; the override stays for genuine owner decisions.
- `make ze-discovery-index-check` is deliberately unchanged: it checks the
  working tree, which is correct for CI (where the working tree IS the checkout)
  and for a developer regenerating locally. Only the commit gate needs the
  commit view. The consequence is that a shared checkout can show that target
  red while commits are legitimately allowed through -- the red belongs to the
  other session's uncommitted files.
- The generators are now runnable against any tree, which makes them testable:
  the fixtures build a throwaway git repo and point the real generator at it.

## Gotchas

- **The fixture must contain the generator.** `discovery_index_freshness` skips
  a generator that does not exist in the repo, so a fixture without
  `scripts/dev/learned_index.py` silently exercises nothing and reports
  "unknown". Copy the generator AND `discovery_sources.py` (its import) into the
  fixture, and keep the other two out to keep the fixture small.
- **A passing new test proves nothing until it is mutated.** Disabling the
  commit-view check made `commit-gate-index-foreign-staleness-passes` fail with
  the exact old error text, which is what makes it evidence
  (`ai/rules/testing.md`).
- **`git archive` accepts `:(exclude)` pathspecs**, which is what keeps the view
  cheap. Excluding `vendor` is safe only because every generator already skips
  it; adding a generator that reads `vendor` would silently break the view.
- **The generator-free rule had to survive.** `scripts/dev/commit_helper_test.py`
  already carried `TestDiscoveryIndexProblems`, whose two cases run against a
  minimal repo with NO generators in it. Replacing the old fed/omitted check with
  the commit view broke `test_unrelated_dirty_index_passes`, because "generator
  absent" was being treated as "cannot judge -> stay stale". The right reading is
  the one `discovery_index_freshness` already documents: a generator that cannot
  run yields unknown and never blocks. So the cheap fed/omitted rule stays (it is
  all a minimal checkout can apply) and the commit view is layered on top, skipping
  any index whose generator is not present.
- **Fail-closed applies to a check that broke, not to a check that does not exist.**
  Conflating the two turns a gate into a wall in every minimal repo.
- **`indexes_fed_by` under-approximates PACKAGE-MAP, so it must not gate what gets
  VERIFIED.** `package_map.build` keys its rows on DIRECTORY existence
  (`scripts/dev/package_map.py`), while `indexes_fed_by`
  (`scripts/dev/discovery_sources.py`) recognizes a PACKAGE-MAP source only
  by a `// Package` header or a `register.go` filename. A new
  `internal/x/thing.go` carrying only `// Design:` therefore adds a PACKAGE-MAP row
  while feeding DOCS-TO-CODE alone. The first version of this fix narrowed the
  commit-view check to `stale | fed` and let exactly that commit through. `fed` is
  fine for deciding the "you omitted the index" message; it is not fine for
  deciding what to check. The view is already built, so check everything: 1.9s
  instead of 1.5s.
- **The view must run the COMMIT's generator, not the working tree's.** Running
  `repo / gen` judged a commit with a concurrent session's uncommitted generator
  edits, which is the same cross-session contamination this whole change removes
  for data, reintroduced through code. `dest / gen` is the tree under test, and
  `add_paths` already overwrites it when the commit changes a generator.
- **A per-PID scratch directory is a fail-open.** `tar -x` overwrites archived
  paths but never removes extras, so a directory leaked by an earlier killed run
  with the same PID merges into the view and can make an incoherent index look
  coherent. `tempfile.mkdtemp` is the fix; `discovery_index_head_status` already
  did it that way in the same file.
- **A fail-closed handler is worthless if its most likely trigger happens outside
  the `try`.** `mkdtemp` sat above the `try:`, so the one failure it exists to
  absorb -- a `tmp/` left root-owned by `sudo make ze-netns-test`, ENOSPC, a
  read-only FS -- escaped as an uncaught traceback instead of degrading.
- **One generator in the fixture made two different fixes indistinguishable.**
  With only `learned_index.py` seeded, "verify every index" and "verify the ones
  this commit feeds" produce identical results, so the fail-open fix had zero
  coverage and reverting it left all fixtures green. The discriminating case needs
  a SECOND generator and a commit that drifts an index it does not visibly feed.
- **Documentation drifts from a gate faster than the gate drifts from its rule.**
  The `repo-maintenance.md` row asserted three properties the code had stopped having
  within one session: a `<pid>` path (now random), "the view runs only when the
  working tree reports stale" (it runs whenever an index source is touched), and
  "fails CLOSED" (it now falls back to the working-tree verdict and says so). A
  reader trusting any of the three would have reasoned from fiction.

## Files

- `scripts/dev/discovery_sources.py` -- `root_from_argv`
- `scripts/dev/learned_index.py`, `package_map.py`, `docs_to_code.py` -- honour `--root`
- `scripts/dev/commit_helper.py` -- `build_commit_view`, `stale_in_commit_view`, two-tier `discovery_index_problems`
- `scripts/dev/hook-fixture-check.py` -- two commit-gate fixtures
- `ai/rules/repo-maintenance.md` -- the gate's row
