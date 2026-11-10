# Handover: spec-tooling-one-off-defects waits on owner decisions (2026-11-10)

Spec: `plan/post-release/engineering/spec-tooling-one-off-defects.md`.
Evidence: `plan/post-release/engineering/tooling-one-off-evidence.md` (every
section dated 2026-11-10 was added in this session).
Session state with run directories:
`tmp/session/2026-11-09-222002/state/session-state-tooling-one-off-defects-03d43e18-c488-4511-8040-2650b2986bb3.md`.

## Done in this session

- The four interop checkers (#2 LOCAL_PREF, #8 typed NLRI, #14 End-of-RIB,
  #7 RFC 7606 mixed UPDATE) are proved, and P-12's four second breaks
  (MEDAcrossAS, MEDRemoval, RFC7999, NoExportBoundary).
- Roughly 140 carriers now have a recorded red and restored green across
  P-1 to P-41 (two batches of overlay walks, functional `.ci` walks through
  an overlaid `le`, lab walks, P-18 native-entry cancellation observers,
  browser carriers once headless Chrome proved usable).
- Defects found and fixed while proving: ze-peer AS_PATH on eBGP and iBGP,
  the BGP batch path's silent drop, git-ignored lint inputs left out of the
  fingerprint, peer-image observation lost behind the recording wrapper,
  `StopsDaemon` undeclared on 525 fixtures, the `/proc` ESRCH race, trim
  removing an active session's paths, the `hunks` route writing before
  validation, `le help ai` shadowing the `ai` area, l2tp-scale running ze on
  the operator's store, and the web-config commit bar (now asserted in place
  with the new `action=wait-html`).
- The owned journal table holds 8 open rows; the rest are closed or moved to
  "Rows found already fixed" with their fixing commits.

## Waiting on Thomas

| # | Decision | Unblocks |
| --- | --- | --- |
| 1 | Run `spec-inventory-fast-registry-checks.md` (approved "yes replace", `58e0684c8b`) in a fresh session | `TestAnswerAnswersTheInventory`, P-32 |
| 2 | P-31: which scanner is the one Go test-unit inventory (weakening's or the RFC gate's) | P-31, row helper-bypassed 2026-11-02 |
| 3 | P-26: whether a recorded historical lint finding plus a clean scoped lint counts as red and green (golangci reads disk; no overlay reaches it) | P-26 lint members, rows lint-contract 2026-10-27 x3 and gate-red 2026-08-30 |
| 4 | P-10: the 9p symlink check on the Mac (Linux is clean under both builders) | row container-cannot-see 2026-10-21 |
| 5 | P-17: a push for the GitHub CodeQL extraction proof | P-17 CodeQL member |
| 6 | P-18: root for l2tp-ppp and gokrazy-l2tp-ppp, and a deliberate VM-slot holder for docker-lab admission | P-18's last members |
| 7 | Delete the unowned, superseded untracked `internal/test/fixture/register_stops_daemon_test.go` | a local red |
| 8 | A one-minute in-place edit of the shared root `ze` launcher to record the launcher tests' red | row delegation 2026-09-07 |
| 9 | `pluginOwnershipGate`: a baseline or a per-edge design | row gate-red 2026-10-05 |

## After the decisions

Finish the gated proofs, rerun a strict audit of P-1 to P-41 against the
evidence file (only a recorded red plus restored green counts), close the last
rows, then `/ze-review` for the Review Gate and the two-commit closure.

## Noted, not this spec's

- `internal/component/web/testing/expect.go` says negative expectations do not
  poll; `checkExpectationRetry` retries them for 5s (stale comment).
- `internal/component/ike/engine/rfc7296_certreq_test.go` (another session's
  `83a985dce0`) fails `./le rfc check`'s file-naming rule.
- Observer scripts for P-18 live in the job scratch
  `/home/thomas/.claude/jobs/03d43e18/tmp/p18obs/` (`lib.sh` with a subreaper).

## Update, later on 2026-11-10

Decisions 3 and 8 were answered by the spec's own walk method for guards that
read their source from disk (an in-place edit, restored from a saved copy with
its hash checked): the three lint-contract rows, the 2026-08-30 site goconst
row and the 2026-09-07 launcher row are closed on in-place reds (evidence
sections "Fixed lint members proved red in place" and "The root launchers exec
their binary"). The browser was never broken: headless Chrome works here, so
P-13's capture, P-17's theme and P-19's web-config journey are proved, and the
2026-08-18 and 2026-08-24 rows are closed (the web-config commit bar is now
asserted in place with `action=wait-html`).

Still waiting on Thomas: decisions 1, 2, 4, 5, 6, 7 and 9. Owned rows still
open: helper-bypassed 2026-11-02 (2), container-cannot-see 2026-10-21 (4),
gate-red 2026-10-05 pluginOwnershipGate (9).
