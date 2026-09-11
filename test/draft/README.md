# `test/draft/` — functional tests under development

A `.ci` file here is **invisible to every suite and every repo-wide gate**. Write
and iterate on it as long as you like; it cannot redden `./le verify current mode full`, and
because the directory is gitignored it does not exist in CI at all. The one
tracked exception below is checked out by CI: it is invisible there because
every reader skips this directory, not because the file is absent.

This directory is tracked for this README and for that exception. Everything
else in it is ignored (`.gitignore`: `test/draft/*` plus the negations).

## The one tracked exception

It is not a test under development, and it is never promoted on the workflow
below. It is here for the property this directory already has, which is that no
gate reads it, and it states its own reason in its own header.

| File | What it is | Why it is not gated | What would gate it |
|------|-----------|--------------------|--------------------|
| `plugin/gr-vacuity-*.ci` | three DEMONSTRATIONS: two pass with Graceful Restart unreachable, and the third MUST fail | a demonstration that MUST fail reddens every sweep of the directory that holds it (`plan/journal/unwired-feature.md`, 2026-09-08) | nothing. A demonstration is never a regression test |

Run it with `ze-test bgp plugin --draft --pattern gr-vacuity`.

`l2tp/subscriber-reader-failing-socket.ci` was the second exception until
2026-09-11. It was KNOWN VACUOUS because `strace` was the only mechanism that
sustained a read error, and ptrace's own signal-trap overhead throttled the
traced daemon into the same range the fix produces, so the pre-fix build passed
every assertion in it. `ze-test fail-syscall` (`internal/test/failsyscall`)
replaced that stimulus with a seccomp filter, which has no tracer in the path,
and the scenario moved to `test/l2tp/` and is now gated. Its header carries the
whole finding, including the ptrace numbers.

A second file must not be added to this table on the strength of "it is not
ready". That is what the rest of this directory is for, and an ignored draft
costs no reader anything. A tracked exception earns its place by carrying a
finding that is worth more than the file costs, and by saying in its own header
why no gate reads it.

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
#    The verb is the suite's own: `bgp plugin`, but bare `ui`, `editor`, `web`.
#    internal/le/functional/suites.go carries the argv for each suite, and a
#    runner given a verb it does not know prints usage and EXITS 0.
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
| documentation wiring | `internal/le/doc/wiring/checks.go` |
| RFC evidence | `internal/le/rfc/carriers.go` |

## Two ways a draft run passes without running

**A wrong verb exits 0.** A runner given a suite verb it does not know prints its
usage and answers 0, and through `./le stress-repro` that reads as "not
reproduced", which is the word for green. Read the log for a `--- PASS` or
`VERIFY STEP` line naming your test before you believe a run. The defect is
recorded in `plan/journal/silent-fall-through.md` and the rule is
`ai/rules/commands.md`.

**A new compiled fixture needs the test runner rebuilt.** `fixture <name>`
resolves the name in the runner binary's own registry
(`internal/test/fixture`, `Register`), so a fixture added this session is absent
from a stale runner while the daemon under test is current. `./le functional
<suite>` rebuilds both.

## What a draft does NOT get

No accept-only, frame-length, documentation-wiring, or RFC-evidence check runs
until promotion. Promote early enough to run all four before review.
