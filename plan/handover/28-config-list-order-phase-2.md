# Handover: config list entries in operator order, start of Phase 2

Written 2026-10-28 by session ed626c22 at Thomas's request, at the end of Phase 1.

## Start here

1. Free space on `/home/thomas/.cache` first. It was at 100% (248 MB free of
   295 GB) after `./le scratch trim` and `./le scratch clean`; the space is in
   the shared `docker`, `containerd` and `go-build` trees, which only Thomas
   may clear. Every build and test fails with "no space left on device" until
   then.
2. `./le spec claim plan/release/fixes/spec-config-list-entries-operator-order.md`
   (Status `in-progress`, Phase 1/5), then `/ze-implement` on it.
3. Read the per-spec state file before any source:
   `tmp/session/2026-10-26-ed626c22-d6a4-4293-a5f7-287a2deb8cf2/state/session-state-config-list-entries-operator-order-ed626c22-d6a4-4293-a5f7-287a2deb8cf2.md`,
   section "Phase 1 continuation handoff". It holds the baseline reds, every
   A-N verdict and the Phase 2 first steps.

## What the spec delivers

Owner decision 2026-10-28 (the Junos model, recorded on Q-2 of
`plan/post-release/engineering/spec-ze-config-fmt.md`): the entries of every
list print in the order the tree recorded (operator order), never sorted by
key. `ordered-by user` lists already did. Decisions D-1 to D-4 are in the
spec's Key Design Decisions; D-4: for a list that is not `ordered-by user`, a
reorder-only difference is no change in compare, diff and prune.

## Committed in Phase 1

| Commit | Content |
|--------|---------|
| c129bc5bf9 | Wiring tests, red on purpose: `TestEveryListPrintsRecordedOrder`, `TestUnrecordedEntryPrintsLast`, `TestConfigFmtKeepsOperatorOrderedEntries`, `TestConfigDiffUserOrderedReorder`, `TestTreeDiffListFollowsWorkingOrder`, `TestWebCLIShowKeepsOperatorOrder`, `test/parse/config-fmt-operator-order.ci` |
| a14479ed6f | Set-form parser defects fixed (spec rows A-7 to A-9): `set route <k> ...` and `set vpls rd ...` were refused under InlineList and Flex parents; three `set vpls <k> ...` entries kept only the last; `delete vpls` did not reach entries. Tests in `setparser_flex_inline_test.go` |
| 3f15ef1fa1 | `.et` harness `pattern` key (Go regex) for `expect=viewport` and `expect=content`, documented in `docs/architecture/testing/ci-format.md` |

## Owed before Phase 2 code

| # | Item |
|---|------|
| 1 | `./le repo compiles check` after 3f15ef1fa1 (not run, disk full) |
| 2 | `test/draft/editor/show-operator-order.et` is red for the wrong reason: after two `set` steps the `show` viewport holds only peer zulu. Investigate (compare `show-full.et`), make it red only for sorting, promote it, commit. It runs through `./le --name order test editor --draft -a -v` |
| 3 | `./le test functional parse`: confirm `config-fmt-operator-order.ci` is red only for sorting. Another session's `internal/test/fixture/rfc8212_reload_rollback.go` broke the suite build earlier; report it, do not edit it |
| 4 | `./le test functional editor` |
| 5 | Suspected config loss, untested: a Flex multi-value prints as several `set vpls <v>` lines that `setFlexValue` reads with `tree.Set`, so the last line may win. Test it; if real, fix it in this spec |
| 6 | `./le verify changed` over c129bc5bf9..3f15ef1fa1, run by the main thread |

`config/cli` tests need every feature tag (`scratch/alltags.txt` in the session
folder); untagged, about 40 fail with "unknown path: bgp".

## Phase 2 first steps (from the spec)

1. `listBlockKeys` prints recorded entries first, then unrecorded ones sorted, for every list kind.
2. Every printer goes through it, and the inline sort copies are deleted: serialize, serialize_set, serialize_annotated, serialize_blame, for list, Flex and InlineList.
3. `Editor.loadTree` (`cli/editor_load.go`) uses `GetListOrdered`, since today it ranges `ListKeys` (A-3 broken).
4. Rewrite the `docs/architecture/config/syntax.md` order paragraph.
5. Rerun config, config/cli (tagged), cli/testing, the web terminal test, the parse and editor suites. Every changed expectation goes to `ze-test-judge`.

Phase 3 (diff surfaces, D-4) needs a tree-level `ordered-by user` comparison in
`ze config diff`, which today diffs only `ResolveBGPTree` output (A-5 broken).

## Related specs, ready, waiting on this one

| Spec | Waits for Thomas |
|------|------------------|
| `plan/release/fixes/spec-config-yang-definition-order.md` | D-5 (editor completion stays alphabetical), D-6 (top-level blocks from several modules ordered by module name). Riskiest point: one-line positional list entries read values by child position, so definition order changes what an existing line means |
| `plan/post-release/features/spec-config-list-entry-insert.md` | D-7 (web terminal `insert` takes the editor form), D-8 (`first`/`last` refused for lists) |
| `plan/post-release/engineering/spec-ze-config-fmt.md` | none; depends on both ordering specs |

## Other open items from session ed626c22

| Item | State |
|------|-------|
| StayRTR PR | https://github.com/bgp/stayrtr/pull/187, opened as Thomas. Commit is unsigned (built via the GitHub API). `test/interop/Dockerfile.stayrtr` keeps its local patch until upstream releases the fix |
| Stale RFC proofs owned by this session | RFC5082-3-2 (two records, `gtsm.go::peerTerms`) can only be re-recorded on the Mac (needs CAP_NET_ADMIN). RFC2866-4.1-1 waits for another session's `internal/plugins/dnsresolver/resolve.go` to build; then rerun `tmp/session/2026-10-26-ed626c22-d6a4-4293-a5f7-287a2deb8cf2/scratch/guest.sh <log>` |
| Push | Blocked on a scoped verify; the last one had no red from this session's commits, only other sessions' reds and load timeouts |
| Unjournaled functional reds under load | `bgp-route-refresh-enabled-over-ssh`, `commit-verify-reject`, `ddos-show`: not reproduced, not journaled |
| Owner decision still open | whether to add a hook refusing shell writes into the repo (agents bypassed the edit hooks with python and sed several times) |
