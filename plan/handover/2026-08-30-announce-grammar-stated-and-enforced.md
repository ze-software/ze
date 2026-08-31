# Handover: announce-grammar-stated-and-enforced

Written 2026-08-30 by the session that ran out of token budget mid-Phase-5.
Nothing below is committed. Every file named here is live in the working tree.

## What this work is

`plan/spec-announce-grammar-stated-and-enforced.md`, status `in-progress`,
Phase 5/6. Read it first: its RESUME HERE block and its `→ Decision:` and
`→ Constraint:` annotations are the state, and this file only adds what happened
after they were written.

One problem with three faces. The command model STATED that the `announce
flowspec` action was optional while the handler REQUIRED it. The handler
under-enforced in the other direction, discarding a trailing token in silence.
Nothing proved either half, because no `.ci` exercised any of the seven
announce and withdraw handlers.

## The blocker, and it is the first thing to do

`./le commit create` REFUSES the commit:

    internal/component/bgp/plugins/cmd/announce/announce_test.go weakens
    TestParseTrailingOptsUnknownTokenStops and test/weakened.md has no row for it

The implementing agent DELETED that test. Its name says what it asserted: that
an unknown token STOPS the parse, which is precisely the silent-discard
behaviour this spec exists to remove. So the deletion is very likely correct and
the replacement is `TestAnnounceRefusesAnUnclaimedTrailingToken`. That reasoning
is UNVERIFIED: nobody has read the deleted test's body against the new one.

Do this, in order:

1. Read the deleted test in `git show HEAD:internal/component/bgp/plugins/cmd/announce/announce_test.go`, and read the new test beside it.
2. If the new test covers the same input with the opposite expectation, add the `test/weakened.md` row naming what left the suite and why the commit is correct without it, then add `file test/weakened.md` to the commit population.
3. If it does NOT, the old test was deleted rather than corrected. That is banned (`ai/rules/testing.md`); restore it and fix it instead.

## State by phase

| Phase | State | Evidence |
|-------|-------|----------|
| 1 probe | DONE | `internal/test/fixture/ui_fixture_cli_announce.go` exists. A-6 is answered: one fixture starts a daemon over ephemeral SSH AND a `ze-peer`, so AC-6 and AC-7 did not have to split |
| 2 renderer | edits present | `usage.go`, `ze-extensions.yang`, `ze-cli-announce-cmd.yang` |
| 3 completion, help | edits present | `completer.go`, `help.go`, and both test files |
| 4 parser | edits present | `announce.go`, `announce_test.go` |
| 5 functional coverage | PARTIAL | The agent's last words were "Both wire tests pass. Now Phase 5's forced red for the handler change." The FORCED RED was never run |
| 6 documentation | NOT STARTED | A live debt against `ai/rules/documentation.md` |

`internal/test/fixture/misc_fixture_vpp.go` is also modified. It holds
`startVPPPeer`, the helper the new fixture almost certainly reuses. Read that
diff before assuming it is incidental.

## Two debts that are NOT paid

**Phase 5's forced red.** The two wire tests PASS, and that is exactly the
condition under which a test's discrimination is unproven
(`ai/rules/interop-and-goal-validation.md`). Revert the handler change, rebuild
the artifact the test drives so the revert takes effect, confirm RED, restore,
confirm GREEN, record the red output. A test written against already-working
code has never had a red phase.

**AC-2's corpus diff, and it is ORDER-SENSITIVE and already compromised.**
AC-2 is the independent proof of AC-1: render every command's usage line before
and after, and only the announce flowspec entry may differ. The before-capture
was never taken, and the renderer edits are now in the working tree, so it
CANNOT be taken from here. Take it from the last commit that precedes the Phase
2 changes. The command is `ze help command --json`, which publishes a `usage`
string and a `grammar` token list per command (`commandEntry`,
`cmd/ze/help_command.go`). `./le docvalid usage-contract` walks the same tree and
is the second reading. A diff produced after the fact proves nothing, so do not
settle for one.

## Decisions already made, do not re-open

- The modifier shape is a NEW parent container carrying `ze:modifier "one-of"`, with `modifierChildren` recursing ONE level. Owner decision. The rejected alternative was a set key on the three sibling containers.
- The `.ci` coverage gate blind spot gets its OWN spec, AFTER this one. Owner decision. Its row is already written in `plan/journal/gate-excludes-part-of-its-population.md`.
- The nineteen match components are NOT regrouped. An alternation over nineteen names is not a readable line.
- The interop row is N-A because no accepted input encodes differently; only which inputs are ACCEPTED changes.

## Traps this session paid for

- `modifierChildren` must recurse ONLY into a node carrying the new modifier. Recursing into every group moves other commands' published lines (R-2).
- `ze-extensions.yang` states the occurrence set TWICE, in the `extension modifier` description and in a comment table, and both said "four occurrences". No check compares them.
- `../gh-pages/data/cli-commands.json` is stale INDEPENDENTLY of this work: it still publishes `announce <unicast|blackhole|flowspec> <args> ...`, the authored sentence the 2026-08-30 split deleted, while the live model publishes three announce commands. Phase 6's `./le site build` and `./le wiki-catalog update` clear it.
- `SignalPeerAPIReady` gaining a `Sender` parameter is reddening plugin lifecycle test files across the tree. That is ANOTHER session's in-flight change. It is not this work's and must not be carried (`ai/rules/principles.md`).
- Nothing the implementing agent reported was verified against source by the supervising session. Read the producer before relying on any claim in this file (`ai/rules/evidence.md`).
