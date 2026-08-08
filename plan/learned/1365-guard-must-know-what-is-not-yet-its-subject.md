# A guard must know what is not yet its subject

**Date:** 2026-08-08
**Source:** repairing GitHub Actions run 31225029268

## The lesson

A guard keyed on a PATH protects everything under it, including the things that
are not yet what the guard exists to protect. `test/draft/` is the incubator: it
is gitignored, skipped by every repo-wide gate, and a file there claims no
evidence and proves no RFC obligation. Two guards did not know that.

`check_test_deletion` (`.claude/hooks/pretool-bash.py`) matched any `rm` naming
a `.ci`, and `c_test_weakening` (`.claude/hooks/pretool-writeedit.py`) ran its
weakening heuristic and its `RFC requirement:` branch over any test-shaped file.
Both fired on drafts. The draft workflow ends in exactly two moves, promote or
delete, so guarding the delete made the incubator the one directory an agent
could fill and never empty.

It cost real work twice in one session. An agent finished an investigation and
had to hand its scratch draft back to the operator to delete. Another could not
edit its own draft, because the draft carried an `RFC requirement:` tag it had
just written, and the guard treated that tag as proof behind a public
compliance claim. A tag inside a draft is worth nothing until the file is live.

## What to do

State the exemption in the rule, not only in the check, and pin BOTH directions
in fixtures. `ai/rules/testing.md` now says a draft is promoted or deleted and
never left, and names the two checks that skip it. The fixtures assert the three
draft cases pass AND that a live test still blocks, including a command naming a
draft and a live test together, which stays refused: the live one is the reason
the guard exists.

The general shape, worth applying to the next guard: an exemption must be
derived from what the guard PROTECTS, never from where the file sits. Ask what
the guard's subject actually is, then ask which paths hold things that are not
that subject yet.

## Files

- `.claude/hooks/pretool-bash.py`
- `.claude/hooks/pretool-writeedit.py`
- `ai/rules/points/testing/draft-a-functional-test-before-it-is-live-blocking/a-draft-is-promoted-or-deleted-never-left.md`
- `ai/rules/points/testing/manifest.md`
- `ai/rules/testing.md`
- `scripts/dev/hook-fixture-check.py`
