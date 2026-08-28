# `test/draft/` — functional tests under development

A `.ci` file here is **invisible to every suite and every repo-wide gate**. Write
and iterate on it as long as you like; it cannot redden `./le verify current mode full`, and
because the directory is gitignored it does not exist in CI at all.

This directory is tracked only for this README. Everything else in it is ignored
(`.gitignore`: `test/draft/` plus `!test/draft/README.md`).

## Why this exists

The suite runner discovers tests with a NON-recursive glob
(`internal/test/runner/record_parse.go` `Discover`: `filepath.Glob(dir + "/*.ci")`),
so a subdirectory is already invisible to it. The problem was everything else: six
gates walk `test/` RECURSIVELY, and each one would have seen a half-written draft.

A test in progress used to have nowhere to live. Writing it in `test/<suite>/`
meant every `./le verify current mode full` in the tree ran it — including runs by other sessions
working on unrelated things, who then had to decide whether the red was theirs.

## Layout

Mirror the real suite directory one level down:

```
test/plugin/eor-per-family.ci          <- the real test, every gate applies
test/draft/plugin/eor.ci    <- the draft, no gate applies
```

The suite name is the directory name the real suite uses (`plugin`, `encode`,
`reload`, `firewall`, `policy`, `ospf`, ...).

## Workflow

```
# 1. write it
$EDITOR test/draft/plugin/my-new-test.ci

# 2. run it (only drafts are discovered under --draft)
ze-test bgp plugin --draft -a
ze-test bgp plugin --draft --pattern my-new-test

# 3. prove it under load, still as a draft
./le stress-repro run suite "bgp plugin --draft" test 1 any-failure

# 4. promote when green: a plain move, no git plumbing needed
mv test/draft/plugin/my-new-test.ci test/plugin/my-new-test.ci

# 5. now it is a real test -- run the whole suite once before committing
ze-test bgp plugin -a
```

Replacing an existing test is the same move: draft alongside it under
`test/draft/<suite>/`, then `mv` over the original.

## The checks that skip this directory

Each recursive `.ci` reader explicitly skips `draft`. Add each new reader to
`TestDraftDirIsInvisibleToRepoChecks`
(`internal/test/runner/draft_dir_test.go`).

| Check | Producer |
|------|----------|
| accept-only lint | `internal/test/runner/accept_only.go` |
| BGP frame-length fixtures | `internal/test/runner/ci_fixture_test.go` |
| documentation wiring | `internal/le/docwiring/checks.go` |
| RFC evidence | `internal/le/rfc/carriers.go` |

## What a draft does NOT get

No accept-only, frame-length, documentation-wiring, or RFC-evidence check runs
until promotion. Promote early enough to run all four before review.
