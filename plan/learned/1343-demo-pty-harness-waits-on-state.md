# 1343 -- The Demo PTY Harness Waits On State, Not On A Deadline

## Context

On 2026-08-04 `make ze-terminal-demos` stopped on its first red demo. Thomas was
therefore unable to update the website. A per-demo sweep of all 18 validators
found `zefs-config` and `commit-confirmed` red. Both reds were the same defect in
the shared driver rather than in the product.

`demos/terminal/pty-session.py` typed each `--command`, then waited `--delay`
seconds, then typed the next one. The CLI answers asynchronously.
`Model.executeCommand` (`internal/component/cli/model_commands.go`) returns a
`tea.Cmd` closure. `dispatchCommand` therefore runs on a Bubble Tea goroutine,
and its `commandResultMsg` lands when it finishes.

Measured on an idle host, the editor's commit answers in 0.13s to 0.76s. The
validators allowed 1.0s (`zefs-config`, the default) and 2.0s
(`commit-confirmed`). Under the load of a full sweep the answer missed that
window. Every later command then entered the wrong state. `confirm` reached
`cmdConfirm` (`internal/component/cli/model_load.go`) while
`m.confirmTimerActive` was still false, and got `no pending commit to confirm`.

The product is correct. Replaying both command sequences byte for byte on an
unloaded host reaches `Session committed: 1 change(s) applied and reloaded`,
`Connection to 127.0.0.1 closed.` and
`Configuration confirmed and saved permanently.`

## Decisions

- A `@wait <regex>` directive, over raising `--delay`. A larger number is the
  same defect with a different constant: it still fails on a busier machine and
  it lengthens every validator run.
- `@wait` joins the existing `--command` list rather than becoming its own
  option. `--command` uses `action="append"`, and argparse does not preserve
  order ACROSS options. A separate `--expect` therefore cannot say WHERE in the
  sequence it belongs. The list is also where `@escape` and `@sleep` already
  live.
- Correctness comes from a `window` offset, not from skipping the delay. The
  wait searches `captured[window:]`, where `window` is the moment the directive
  BEFORE it started. The `handing_off` branch that skips the fixed delay is
  speed only. The first review round found the first design, which relied on the
  skip alone: see the first Gotcha.
- `parse_args` refuses three shapes rather than accepting them quietly. A
  trailing `@wait` or `@sleep` drops the tail, because the final command's
  output is read until the connection closes and both return earlier. An
  unknown `@word` is a misspelt directive, and it used to be typed into the CLI
  verbatim. A `@sleep` or `@wait` with no argument is incomplete.
- `@sleep` stays where the wait is the assertion. `commit-confirmed` still sleeps
  7 seconds after `confirm`, because the point is to observe that the confirm
  window passes WITHOUT a rollback. There is no state to wait for. The absence
  over time is the evidence.

## Consequences

- Both validators pass with `--delay 0.05` as well as at the delay they ship
  with (2.0 and the 1.0 default). That is the discrimination proof: the waits
  carry the sequence. It is an experiment rather than the shipped configuration,
  so it does not re-run from the tree. Re-run it by hand with `sed` over a copy
  of the validator whenever the driver changes.
- Editing `pty-session.py` changes `source_digest` for EVERY demo, because it is
  in `render.py`'s `SHARED_SOURCE_PATHS`. Any change to it forces a full
  re-render before `--check` passes again.
- Commands still sequenced by the fixed delay do have asynchronous answers, and
  the delay is what covers them. A command whose SUCCESSOR depends on that
  answer needs a `@wait`, not a bigger delay.

## Gotchas

- **A `@wait` that starts its own search cannot see an answer that already
  arrived.** `read_until` scans only what IT reads. An answer consumed by a
  preceding `read_for(delay)` or `@sleep` leaves the wait to time out on a
  screen that already said what it waits for. Skipping the delay before a wait
  hides this for the write path and does NOTHING for `@sleep`, which reads and
  then hands over. The `seen=` window is the fix that covers both.
- **The captured stream is a full-screen TUI redraw, so reading it as a
  transcript misleads.** In the first failure log the `Committed.` banner looked
  as though it arrived two seconds late, which pointed at a slow commit. Direct
  measurement with a timestamped driver showed 0.13s. Frames overwrite each
  other and the ANSI stripping splices the fragments, so ORDER in the capture is
  evidence and LATENCY in the capture is not.
- **A regex directive must be escaped for the regex, not only for the shell.**
  `@wait Quit?` matches `Qui` followed by an optional `t`. The validator passes
  `'@wait Quit\?'`.
- **Wait for a token the ANSWER owns, never one the screen already holds.** The
  editor viewport shows the config tree, and `router-id 192.0.2.1` is in it. A
  `@wait router-id` after `run show bgp summary` can therefore return on a config
  repaint rather than on the summary. `peers-established` is a key
  of the summary result (`internal/component/bgp/plugins/cmd/peer/summary.go`).
  It appears nowhere in the configuration, so only the answer satisfies it.
  After the Escape that LEAVES the summary, the same reasoning inverts.
  `router-id` is then the token the answer owns, because the config view is what
  repaints.
- **Bubble Tea repaints only what changed, so a wait on unchanged text never
  matches.** `\[operational\]` was the tightening two reviewers asked for, and it
  timed out. The header line is redrawn from the changed column onward, so the
  stream carries `operational]` and never the opening bracket. The token must be
  both unique AND newly painted. `operational\]` satisfies both.
- **The out-of-order hazard is wider than the refusal it produced here.**
  `Model.executeCommand` (`internal/component/cli/model_commands.go`) has a VALUE
  receiver. `cmdConfirm` therefore read a snapshot taken before
  `confirmTimerActive` was set, and `no pending commit to confirm` is a correct
  answer to a stale read.
  `Model.editor` is a `*Editor` shared by every snapshot, `Editor` carries no
  mutex, and nothing gates a second Enter while a command is in flight. Two
  commands pasted as a block therefore run two mutating goroutines over one
  editor and its files. That is a live defect and it is not fixed here.

## Files

- `demos/terminal/pty-session.py`: the `@wait` directive, the `seen=`/`window`
  search boundary, the three `parse_args` refusals, and a `poll` helper that
  never hands `select` a negative timeout
- `demos/terminal/zefs-config/validate.sh`: waits on `peers-established`,
  `router-id`, `Session committed` and `operational]`
- `demos/terminal/commit-confirmed/validate.sh`: waits on `Confirm within`,
  `automatically rolled back`, `confirmed and saved permanently`, `operational]`
  and `Quit?`
