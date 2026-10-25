# Learned: who started the run is a fact the process can read, and a reviewed list fails closed

`spec-verify-release-check-only` made the full verification a release check
only Thomas starts, and gave day-to-day work and a push one scoped run an agent
may start.

## What changed

- The change set is anchored on git: `ResolveAnchor`
  (`internal/le/repo/changed/anchor.go`) answers `merge-base(HEAD, @{u})`, else
  `origin/HEAD`, else refuses. `greenBaseline`, which anchored the scope on the
  last green full verify, is deleted, so one interrupted run no longer widens
  every later scoped run to `./...`.
- Every whole-tree verb declares `leaction.Action.WholeTree`, and `Area.admit`
  runs `human.RequireTerminal` (`internal/le/le/human`) before the body: a run
  whose stdin or stdout is not a terminal is refused, with no override. The
  evidence container gets `docker run -t`, so the inner run passes the gate.
- `./le verify changed` runs the `changed` population in a throwaway worktree
  and writes `tmp/ze-verify-changed.status`; `deriveDebt`
  (`internal/le/commit/owed.go`) admits a push on that record when its anchor
  is an ancestor-or-equal of the current upstream.
- A full run publishes `./...` and every tag, so a release check judges the
  whole tree. A shared stage is judged in part unless it declares
  `Stage.Whole`, so a partial green never replaces a full red.
- `TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify`
  (`internal/le/wholetree_corpus_test.go`) holds every mention of a whole-tree
  verb in the agent corpus against the reviewed ledger
  `internal/le/testdata/wholetree-mentions.txt`.

## What outlives the spec

1. **A gate decides on a fact the process can read at start.** Whether the
   machine will stay quiet for the next hour is a judgement no check at
   admission can make; whether a person at a terminal started the run is a
   property of the process's own streams, which a tool call cannot supply.
2. **Anchor on what no run can overwrite.** The certificate is rewritten by
   every run, red or green, so anything anchored on it inherits the last
   interruption. The upstream is what a push compares with, and it moves only
   when someone pushes.
3. **A classifier over English fails open; a reviewed ledger fails closed.**
   Two rounds of regexes over sentences each passed imperatives they were meant
   to refuse. Listing every mention verbatim, refusing an unlisted one and a
   stale one, moves the judgement to a reader and leaves the test only the part
   a test can do.
4. **Invert the default when the omission is the risk.** Declaring every scope
   reader failed open on the reader nobody declared. Making "judged in part"
   the default and declaring only `Whole` makes the same omission refuse.
