### `ze-test web` 20 `test/web/commit-flow.wb` -- `expect html` fails in the full suite, passes alone

Observed 2026-07-28 during the plugin-suite flake clearance. Not caused by that
work, and not a product defect as far as this entry can show.

| Condition | Result |
|-----------|--------|
| `ze-test web 20` alone | PASS, 3/3 |
| `ze-test web -a` (full suite, 4-way browser concurrency) | FAIL, 3/3 runs |
| same, after `agent-browser close` | FAIL |
| **clean worktree at HEAD, none of this session's changes, `ze-test web -a`** | **FAIL, identically** |
| GitHub `verify` run 30343424675 | PASS 87/87 |

The failing step is line 23 of the test, `expect=html:not-contains=system host
web-host-1`, and the error is `html: exit status 1` -- the agent-browser `html`
command itself exiting non-zero, NOT a contains/not-contains mismatch. Every
earlier step passes, including the click that precedes it.

## Attribution: proven pre-existing, by baseline worktree

Not inferred. `git worktree add` at HEAD (25130b882, without this session's
uncommitted changes), `make test` in it, then `ze-test web -a`: the same test
fails the same way. So it is neither the RS/relay changes nor the draft-test
workflow. Worktree removed afterwards.

It also passed 87/87 on the GitHub runner in the last `verify` run, so it looks
host-specific rather than universal -- consistent with 40 occurrences of
`agent-browser: --ignore-https-errors ignored: daemon already running` in the
suite log, i.e. four concurrent browser tests sharing one daemon whose options
were fixed by whichever test started it first.

## Owner

UNOWNED. Nobody is currently clearing it.

Whoever picks it up: the suite runs 4 browser tests concurrently against a single
shared `agent-browser` daemon. Start by establishing whether `html` exit 1 is a
daemon-level contention (two tests driving one browser context) or a real page
state, e.g. by running `ze-test web -a -p 1` -- if it passes serially, the
contention is the mechanism and the fix is per-test browser isolation or a `web`
exclusive group, not a change to commit-flow.wb.

Do NOT read this entry as licence to leave the red standing: `ai/rules/git-safety.md`
("Do not let a red persist") means an unowned red hides the next real regression
under it.
