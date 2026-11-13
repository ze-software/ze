# Current source patch refresh: all cuts UNAPPLIED

This directory refreshes the original 24 daemon cuts and supplements 25/26,
then includes new supplement 27 for latest producer originals. No product
source was mutated. No patch was applied, compiled or run. `outcomes.json`
therefore remains empty. Earlier directories retain their original bytes and
are historical proposals, not executed evidence.

`cuts.current.json` is the current source/patch manifest. Every row carries the
complete current source SHA256 and a patch against those exact bytes. All 27
patch old hunks and their line counts are checked using scratch-only text
analysis in `hunk-checks.json`; that check is not a compiler or test result.

Among the original 24 cuts, two producer files changed since current-v2:

- 08: `manager.go` changed during the nonwinner explicit-ECMP repair. The
  hidden-LPM producer and the original semantic cut remain unchanged; its
  hunk moves to current source and its hash guard is refreshed.
- 12: `route_view_worker.go` gained local replay reconciliation and replay
  admission changes. The old surrounding hunk no longer matches. The current
  patch still only clears `batch.ReplayID` after `replayBatches` and before
  `publish`. Local key consumption, pending requests, publication and retirement
  remain intact. The cut still preserves every Add while erasing replay identity.

The other original producer hashes and all their semantic edits are unchanged.
Supplemental 25/26 retain their semantic edits and current source hash guards.
All patches carry a proposed `// MUTATION-APPLIED` source marker. None of those
markers was put in product source by this preparation.

Use `carrier-commands.md` for the original 24 populations and required semantic
outcomes. Reuse the fresh-copy/apply/nonempty-diff/guarded-restore procedure in
`../execution-v3/source-cycle.md`, with each row's **execution-v4** patch and
current source hash. Save every actual attempt in a fresh directory; never
restore an earlier attempt's baseline. Read current guest-builder output before
using its returned binary environment. The old supplemental recipes remain in
`../execution-v3/supplemental.md`; their patches here are the refreshed copies.

Supplement 27 has its own bounded unit recipe in
`../execution-v3/27-unchanged-original-values.md`. It proves a unit regression
control added after the fix, and does not replace any paired daemon/kernel cut.
Its original proposal and source metadata also live in execution-v3, as requested.

Measured current results already read: main's five new unit roots passed in
`job-rp6-complete-explicit-ecmp-unit-green-79665ef0.log`, including both reader
placements and the originals control. The unchanged 100-iteration benchmark
reports winner and nonwinner each 0 B/op, 0 allocs/op after the fix, compared
with 560 B/op, 6 allocs/op and 224 B/op, 2 allocs/op respectively before it.
Those are allocation counts only; no timing or end-to-end performance claim is
made. Supplement 27 still owes actual semantic RED and guarded restored GREEN.
