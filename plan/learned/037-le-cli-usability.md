# Learned: le speaks one grammar, declared once, and every surface reads it

`spec-le-cli-usability` gave `le` typed parameters on its action table, help
pages, completion, `le help ai`, the terminal menu and prompting, exit codes
and a 36-verb vocabulary, all derived from that one table, and renamed 203
spellings with no alias. The reference pages are
`docs/contributing/running-commands.md` (verb vocabulary, exit codes,
completion install, launcher words) and `docs/architecture/core-design.md`
("Command engine and composition roots").

## What outlives the spec

1. **One table per area, and le refuses to start without it.** Every area
   declares its actions in `leaction`; help, completion, `le help ai`, the
   menu and the usage errors read that table (`leroot.Tree`,
   `requireEveryAreaHasATable`). A hand-rolled parser beside a table is the
   second declaration this spec deleted, 30 times.
2. **A renamed spelling is refused naming its successor, never aliased.** The
   rename map `internal/le/le/retired` serves the runtime refusal and the
   citation sweep `./le doc check retired-commands` from one declaration. The
   sweep also judges every cited `le` line through dispatch's own resolution
   (`leroot.JudgeCitation`, `List.AtVerb`), so the doc judge and the
   dispatcher cannot disagree.
3. **Exit codes have one meaning each:** 0 success, 1 found a problem, 2
   usage, 3 could not run. Hooks and pass-through areas keep their protocol's
   codes and say so in `le help ai exit-codes`.
4. **A bound written twice drifts.** Rounds 8 and 9 found test copies of the
   dispatcher's word bound; tests now read `CommandWordsMax`.
5. **Startup time is package init first.** The 100 ms completion budget
   (AC-37) failed at 111 to 208 ms, and the cause was not the code under test:
   go-runewidth v0.0.27 built a 2x0x110000 width table in `init()` on every
   `le` and `ze` start. `GODEBUG=inittrace=1` found it; v0.0.31 builds it on
   first use (5b6a4f5d8c). Measure init before optimising a hot path.
6. **A golden that counts linked plugins is per platform.** `ze help ai`
   lists three linux-only plugins, so its goldens are per GOOS
   (`internal/test/fixture/testdata/ze-help-ai-*-<GOOS>.golden`).
7. **Which runner walks which `.ci` directory is one resolver,**
   `testfunctional.CIRunnerWords`; the suite table alone is wrong for
   `test/install/` and `test/pppoe/`.

## What it cost

Ten review rounds, rounds 6 to 10 owner-authorised. Rounds 3 to 5 each found
a defect in the retired-successor walk (a panic on growing tails, a reachable
"did not settle" panic on typed argv, quadratic memory), now pinned by
`TestSuccessorIsNeverARetiredLine` and
`TestSuccessorOfALongTypedLineAllocatesLinearly`. Three planned tests were
found unwritten only at closure, because no gate reads a spec's TDD plan
against HEAD.
