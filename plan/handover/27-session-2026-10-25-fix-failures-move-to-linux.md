# 27 Session 2026-10-25 "fix-failures": state at the move to Linux

The session fixed the failures from the last full verify (24 October, commit
61e01bb) and from GitHub CI run 40125794710 and the evidence nightly, then
worked towards a green CI run. Its own work is all committed. Nothing is
pushed.

## STATE 2026-10-26, the Linux session that took this handover

The Mac commits were already on `origin/main` (89b9c2eedb). CI run
40238058974 at that commit failed five of six shards. Fixed here, committed
locally, not pushed:

| Red | Cause | Commit |
|-----|-------|--------|
| `config claims`, `TestConfigSchemaRootsClaimed`, `TestAuditReadsTheLiveRegistry`, gated table stale | The claim gate counted owned roots only; `pki` became a read root of ike (b71b99a761), which commit and reload still deliver | 3e17f1a6a1 |
| `site facts check` | Published counts drifted | 05eeb3cce8 |
| `update-text-sr-policy-addpath` | A withdrawal queued during initial sync went out as a bare MP_UNREACH_NLRI, the direct one with attributes: one command, two wire shapes by timing | a306bd35f2 |
| `error-panel-toggle.wb` | The runner clicked a sliding element; a field saved twice (blur, Enter and debounce) | d674241c52 |
| misspell in gnmi | UK spelling; the gNMI spec quote is kept and exempted | 832334aa95 |
| Six weakening-ledger tests | No git identity on the CI runner | eccfe5bcea |

Still red in that run and owned elsewhere: `le-checks-answers`,
`le-completion-shells`, `ze-help-ai-unchanged` and `TestTheCheckoutsOwnHeadIsClean`
(better-cli, `setup docker` rename in flight); `TestPolicyFinalReloadRetainedRoutes`
(RS fast path send permission, journal row 3d3c017f80); `modify-oversize-suppress`
(the known plugin-suite population flake, `suppression-decision-not-deterministic.md`).

Still owed:
- `./le test integration gtsm` under sudo needs the host to itself; it never was
  (load 25 to 126 all session).
- Interop for the queued-withdrawal change (wire-visible): `./le test integration interop`.
- A CI run after the next push confirms all of the above, and the web
  line-65 race, which shows only on a fast host.
- The RFC7947-x-5 gomu run is being run by the route-server session (started
  2026-10-25 22:58); this session's duplicate was stopped.
- `buildWithdrawNLRI` has no production caller left. Deleting it deletes its
  bounds test, which needs Thomas's approval.

## Read first on the Linux host

- `main` on the Mac is 68 commits ahead of `origin/main`, today's work and other
  sessions' together. A Linux checkout has none of them until they are pushed.
  `./le commit push` refuses until `./le verify changed` passes over them, and
  Thomas declined that run on the Mac. How the commits reach Linux is his call:
  the verify then the gated push, or another route he names.
- Other sessions (better-cli, better-location) still had uncommitted work in
  the Mac tree at the time of writing. It is theirs and is not in any commit
  of this session.

## Where each piece of open work lives

| Work | Owner and home |
|------|----------------|
| Route-server replacement spec: rest of Phase 3a, then Phases 4 to 6 | `plan/release/fixes/spec-bgp-rs-replacement-on-withdrawal.md`, in-progress. State and decisions in `plan/handover/12-rs-replacement-phase-3-remaining.md`, section "STATE 2026-10-25" |
| RFC7947-x-5 proof on the discriminating break | Handover 12: a gomu run over `session_ownership.go`, 869 mutants, about 4-7 hours at 2 workers. Thomas moved it to the Linux host; ask him when the host can carry it |
| Go runtime bug report (SIGQUIT dump hangs under `-race`) | `plan/handover/26-go-runtime-sigquit-race-report.md`, a dedicated session. The runtime patch is committed (c772c6ca1e) |
| Site package race run exceeding its timeout | journal `gate-verdict-depends-on-the-machine.md` (2026-10-25), owned by `spec-test-verdicts-independent-of-load` |
| Recovery builds each owed key's replacement alone | journal `bound-too-small-for-its-own-burst.md` (2026-10-25), owned by the route-server spec |
| Six doc-wiring claims under better-cli's edits, and `le-checks-answers` (needs the `setup docker host` rename) | better-cli (spec-le-cli-usability) |

## Owed proof that needs Linux or CI

- Done 2026-10-25 on Linux at 89b9c2eedb: the three needs-linux functional
  tests the route-server work reaches (`adj-rib-in-replay-rfc2545-next-hop`,
  `linklocal-only-multihop-withdraw`, `rfc2545-joint-subnet`) pass in the
  QEMU guest. The commits above were pushed before the move, so the Linux
  checkout already carries them.
- The nightly gtsm gate's timeout, raised from 120 s to 600 s (54f414612a):
  `./le test integration gtsm` under sudo, alone on the host.
- The nightly interop jobs' module-cache step (`./le setup appliance-modules`,
  5f9d306b1d): proven only by unit tests until a nightly runs.
- A GitHub CI run at the pushed commit is the full answer on what is still red
  across every session's work.

## Decisions taken this session (do not reopen)

- Recovery writes are packed into shared UPDATEs; a test capture is never
  widened to hide one UPDATE per key.
- A read pool at its byte budget makes the session read wait, unbounded and
  silent, as FRR's `bgp_process_reads` does. The hold timer still expires under
  RFC 4271 Section 8.2.2 Event 10, per Thomas's 2026-08-03 ruling recorded in
  `docs/architecture/congestion-industry.md`.
- The home page keeps "Plugin catalogue"; `.golangci.yml` exempts that word and
  the site plugin, concept and RFC-prose tables from `goconst`, each with its
  reason.
- `le-evidence-answers` runs the release-candidate check on a pseudo-terminal;
  the whole-tree terminal gate is unchanged.
- ExaBGP compatibility tests wait on "update sent" events (`wait_for_ack`
  with `announce=`/`withdraw=`), never on sleeps or a `done` ack.

## Retained files

The session scratch directory under `tmp/session/2026-10-25-85f8903f-.../scratch/`
keeps the standalone Go reproducer (`repro/`) for the handover 26 report, and
small logs. It is untracked and Mac-local; delete it once the Go report is
filed. Everything else this session created was removed.
