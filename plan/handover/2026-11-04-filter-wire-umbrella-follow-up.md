**Superseded: the spec closure landed in `54c5e8af0c`.**

# Filter-wire umbrella: follow-up after session 384a3761 stood down

**Spec:** `spec-filter-wire-0-umbrella`, unclaimed by
this session. Another session (its commits call it "UmbrellaCloser") was closing it
on 2026-11-04 and has uncommitted closure sections in the spec file.

## First: find out whether the other closer is still running

1. Run `./le spec status | grep filter-wire-0-umbrella` and `git log -5 --format='%h %ad %s'
   --grep 'Spec-work: filter-wire-0-umbrella'`. Its last commit seen here was 19237e47d8
   (2026-11-04 21:52).
2. Look for a session directory under `tmp/session/` still being written whose state file
   names `filter-wire-0-umbrella` (rule: `ai/rules/git-safety.md`, deferring to a holder).
3. **If it is still running, do not touch the spec or its findings.** Report and stop.
   **If it is gone**, ask Thomas before taking over its uncommitted spec edits: the file holds
   its closure sections (findings UClose-1 to UClose-14) and this session's attempt-3 edits
   (Status line, the second-logger row removed from Owned journal rows, the Blocker table).

## What the closure still needs (whoever owns it)

| Item | State | Evidence |
|------|-------|----------|
| Nine regressions the other closer found | open, they block a CLEAN Review Gate | named in the body of 19237e47d8: `TestFilterOwnerAliasDelivery`, `TestPolicyWithdrawalFollowUsesOriginalDecision`, `TestRemovalDownRetainsRecipientsAcrossConcurrentPublication`, `TestDefaultConditionSetsGateWithoutMutatingDefault`, `TestStaleWriterDoesNotReenterExport`, `TestUpstreamInheritanceUsesConfiguredRole`, route-event burst, route-server receipt, inactive-map |
| Review Gate scope | Thomas chose option B on 2026-11-04: review only the code since the last independent per-phase reviews (after f0b04c1428), not the full net diff. That narrows the review; it does not clear the known findings above | this handover; record it verbatim in the spec's Review Gate |
| Review rounds already run under option B | round 1 (one ISSUE, fixed by 21b7b99c43), round 2 (0 BLOCKER, 0 ISSUE, style pass) | `tmp/session/2026-11-01-384a3761-c376-4523-a82d-0c61fcd10814/scratch/close4-review-findings.md` |
| A-6 | confirmed on Thomas's Mac | `plan/records/filter-wire-0-a6-mac.bench.txt`, the spec's "A-6 matched slow-path benchmarks" |
| Lint `./le go lint run scope ./internal/component/plugin/ipc/...` and `./internal/component/bgp/reactor/...` | not run: other sessions' edits keep invalidating the plan, and another session's reactor file did not parse | name as a red in commit A if still so |
| Closure commits A and B, `./le spec citation`, `./le spec release` | not done | `/ze-close` |

Session state with attempts 3 to 6:
`tmp/session/2026-11-01-384a3761-c376-4523-a82d-0c61fcd10814/state/session-state-filter-wire-0-umbrella-384a3761-c376-4523-a82d-0c61fcd10814.md`.

## Traps already met

- The spec-edit hook (`internal/le/hookruntime/designevidence.go` `designSubjects`) requires
  every closing agent to read the nine doors and every existing source the spec names under
  Current Behavior and Files to Modify (about 34) before its first spec edit. Do the reads
  in one batch first. b1d4cf2f94 and 1df2a04ba0 (owner-approved) fixed its two refusals of
  deleted and package-relative producers.
- `test/perf/filter-a6/textera_bench_test.go.txt` must never be edited: its sha256 is in the
  Mac provenance record. The link check exempts it (801424a35c).
- An independent judge approved the 21b7b99c43 change to
  `ipc.TestFilterSlotCountsOnlyCompletedJudgments`; trailer, if a gate asks:
  `Weakening-approved: ipc.TestFilterSlotCountsOnlyCompletedJudgments`.

## Not the closure's, recorded elsewhere

- `plan/post-release/features/spec-route-policy-0-umbrella.md` Current Behavior still names
  `PolicyFilterChain` and `filter_registry.go`, deleted by this work; its own work corrects it.
- `./le ai hooks unit` fails its hook-source-drift row: the stored digest predates 3475223c4b.
  The tooling session owns it; do not re-seal without reviewing those hook changes.
- `internal/component/bgp/reactor/filter_format.go` is not text-era residue: the
  `show policy test` dry-run display uses it.

## For Thomas

- Nothing is pushed: `main` was 70 commits ahead of origin. A push needs
  `./le commit push "<authorisation>"` from his terminal.
- Session 384a3761's scratch (`tmp/session/2026-11-01-384a3761-c376-4523-a82d-0c61fcd10814/`,
  5.3 GB) holds the evidence behind the spec's proof records. Delete it once the umbrella is
  closed.
