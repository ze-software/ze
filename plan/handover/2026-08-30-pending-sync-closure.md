# Handover 2026-08-30: fixit-peer-pending-sync-settles-too-early

For whoever picks this up, on any agent or any model. Read the spec at
`plan/spec-fixit-peer-pending-sync-settles-too-early.md` first; this page says
only what the spec cannot, which is where the work stopped and why.

## State in one line

The implementation is DONE and COMMITTED. Closure was in flight when the session
ended.

## The three commits that carry the work

| SHA | What it landed |
|-----|----------------|
| `fc9c8bcaa` | the flag split: `sendingInitialRoutes` gates queueing, `initialSyncEOROwed` gates the marker; the barrier moved after `drainAndCloseQueueGate`; `ProcessBinding.MayPushRoutes`; `bgp-rs` and `bgp-adj-rib-in` signal readiness |
| `267fb88e9` | the `raw` send word, `rawOrigin` gated on it, the YANG description, `test/plugin/initial-sync-barrier-raw.ci` |
| `01d0474da` | `validatePeerProcessCaps` reads `MayPushRoutes`; `TestRouteRefreshAcceptsARawOnlyProcess`; three stale code comments; two stale doc blocks; three `.ci` header corrections; `test/weakened.md` |

Both owner rulings of 2026-08-30 are recorded in the spec and are what the code
implements: a plugin-injected route belongs to this speaker's initial routing
update, and the send vocabulary gains a word for `raw`.

## What is left

Closure only, through `/ze-close` on the same spec path:

1. Append `plan/TEMPLATE-CLOSURE.md` and fill Implementation Summary, Audit,
   Deferrals Resolved, Pre-Commit Verification. The spec was written in an
   abbreviated fixit shape and never carried a Deliverables, Security or
   Documentation checklist; fill those from `ai/rules/planning.md` rather than
   stopping on their absence.
2. One `/ze-review` pass over the committed diff, then re-record the artifact
   with `./le spec session review record`. The one recorded on 2026-08-30 is
   hash-pinned to files that have since been committed, so it no longer matches.
3. One journal row, then commit A (spec + row) and commit B
   (`remove plan/spec-fixit-peer-pending-sync-settles-too-early.md`).
4. Account for `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md`.

## Evidence already gathered, so nobody pays for it twice

`ai/rules/pre-release.md` forbids re-running a check to reconfirm a result
already read. All of this was read on 2026-08-30:

- The five unit tests the TDD plan names pass 3 of 3 under `-race`.
- `initial-sync-barrier-raw`, `role-otc-rs-withdraw-eor`, `mup-ipv4-announce`,
  `ipv4-announce-withdraw`, `ipv6-announce-withdraw` and `plugin-nexthop` all
  pass against isolated binaries.
- Red phase proven twice. Deleting the `SendRaw` arm of `MayPushRoutes` makes
  `initial-sync-barrier-raw` fail with `out-of-order marker accepted in
  silence`. `TestRouteRefreshAcceptsARawOnlyProcess` fails against
  `MaySend(SendUpdate)`.

## Three reds that are NOT this work

Each already carries a journal row. Do not spend a session on them.

| Test | Row |
|------|-----|
| `TestDefaultOriginateAppendsLinkLocalWhenSection3Holds`, `TestSendAnnounceAppendsLinkLocalWhenSection3Holds` | `plan/journal/unwired-feature.md`, 2026-08-28 |
| `TestNoConfigFeedsSentUpdatesToAReceivedOnlyPlugin` | `plan/journal/concurrent-session-corruption.md`, 2026-08-29 |

`./le verify lint run` is red on 29 files belonging to other sessions.

## The trap waiting for the next session

A concurrent session was mid-refactor in this shared checkout when this one
ended, on the SAME package: 15 uncommitted files across
`internal/component/bgp/reactor/` and `internal/component/plugin/`, changing
`resetAPISync` to take a `[]string` and adding a `plugin.Sender` argument to
`SignalAPIReady` and `SignalPeerAPIReady`. The reactor TEST package did not
compile as a result.

That work is the OVERCOUNT half, and it is NOT this spec's. Its home is
`plan/spec-bgp-session-ready-contract.md`. An external plugin the barrier counts
that never dispatches `request peer <addr> plugin session ready` still costs the
peer the whole `apiSyncTimeout` of 2s. The four ze plugins all signal; the test
fixtures do not.

If the reactor test package is red when you arrive, read `git status` before you
read the failure: a red produced by another session's uncommitted refactor is
not evidence about this spec, and adapting those tests would be taking another
session's work.

## Verification debt

`plan/verification-debt/c863fb9f.md` and `bae6e1b4.md` hold open rows: no full
native verification covers these commits. That blocks a push, not a commit.
