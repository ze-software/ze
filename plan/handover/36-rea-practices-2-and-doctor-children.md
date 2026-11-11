# Handover: rea-practices child 2, then the doctor area children

Written 2026-11-10 by the session that landed and closed
spec-operator-errors-name-the-fix (P1) and spec-doctor-coverage-1-audit-and-gate
(P2). Thomas's orders in that session: "Land P1 and P2", "Complete the work",
"Fix the issues found along the way and continue with P2", "close loose hands
and all that can be done". Nothing is pushed; a push needs his order.

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | `spec-rea-practices-2-error-inventory`, closed 2026-11-10 (f35639c2fe, 3e6af8384a) (code-derived user-visible error inventory, option A, harm pre-rank) | `design`; both dependencies closed; current-state text refreshed (aa8c59a0aa) | Apply the two decisions below, set `ready`, claim, implement |
| 2 | `spec-rea-practices-0-umbrella`, closed 2026-11-11 | both children closed (rows 1 and 3), then the umbrella | none |
| 3 | `spec-rea-practices-1-mcp-catalog-cost` | closed 2026-11-11 (b619cc8edd, 264db5ca6a): AC-7 lane round recorded; Codex 0.167.1 not yet supported (older MCP revision, by owner decision no compatibility path) | none |
| 4 | Doctor area children `plan/release/validation/spec-doctor-coverage-<n>-<area>.md`, n = 3..15, plus the existing child 2 (`home-cpe-batch`) | all `ready`, P2 release work, about 650 ACs | Child 12 (plugin-infra) first: every other child depends on its phases P0a (cross-cutting codes, transient-code registration, a raise carrying its fix) and P0b (the plugin-protocol report-bus route). One spec per session |

## Decisions already taken for rea-practices child 2

Both follow from the spec's own agreed design; record them in the spec, do not
put them to Thomas.

| Open point the refresh found | Decision |
|------------------------------|----------|
| A-5 partly broken: `ze doctor` findings (`diagnostic.Diagnostic` literals) and their `Fix` are judged by no gate; doctor-coverage reads only log lines, report raises and health rows | Option A gives this spec what neither existing check covers, and the "no second scanner" row forbids a new scanner: extend `internal/le/arch/doctorcoverage` to list `Diagnostic` literals and judge that each carries `Fix` |
| AC-4 has no open journal row left to prove (every row of `plan/journal/error-names-no-fix-command.md` is fixed) | Rework AC-4 into a positive criterion over a testdata fixture; an AC that judges nothing proves nothing |

## What landed (for orientation; read the commits, not this table)

| Area | Commits |
|------|---------|
| P1 fixes and closure | d131605aa4 reload fix line last, SDK lost-config no panic; 81eb233e20 error-wrap follows function parameters and verb prefixes; 605500cf11 operator-error sees `StatusError` answers (`plugin.OperatorErrorResponse`); 37c3a2abb1 argument refusals alike on every route; 41540585bb, dc5b80f1b6 closure |
| P2 code | fc96556110 doctor-coverage gate; ce58dddf53 runtime phase, `Diagnostic.Fix`, local doctor asks the daemon, recent events; beb3a7e136 gate reads covered codes from source declarations; 149497ce12 a plugin's runtime check runs once, through its callback |
| P2 audit | e820ef5cfe `plan/audits/doctor-problem-inventory.md` (+13 area files); 94f8de0b56 the 13 area children and 38 journal rows; c49b0b858a, 974cff10be closure |
| Security | 537b3a87f9, ad9fc4abe2 gNMI (and MCP half pair) refuse to serve on TLS material that cannot load, never plaintext; check `gnmi-tls-material` |
| Rules | 516a82640c rule points (operator error, doctor code per Warn/Error, plugin answers its own runtime check) and the review, implement and spec skills plus TEMPLATE rows |

## Known stale text, owned by the spec that will touch it

| File | Stale | Fix when |
|------|-------|----------|
| `spec-doctor-coverage-7-bgp-plugins.md` | names `collectPluginDoctorChecks` (now `collectDoctorChecksFrom`), and plans registry and Stage 1 declarations of one check to "agree": 149497ce12 refuses a check declared both ways and refuses a runtime `DoctorCheckDef` | before child 7 is claimed |
| `spec-doctor-coverage-4-api-surfaces.md` | the deprecated telemetry aliases (AC-21) are a compatibility path for a form no release shipped; the pre-release rule allows deleting them rather than reporting them | when child 4 is implemented |

## Not this work's, reported

- Another session's uncommitted edit to `internal/component/ike/engine/remote_cert_checks.go` puts `return nil` at the top of `checkIKEPurpose` and `checkPeerChainRevocation` (peer certificate EKU and revocation not checked). That session must restore both before committing.
- Reds from other sessions seen 2026-11-10: `internal/le` vocabulary tests (`reclaim worktrees`, `commit approval-check`); `internal/le/commit` `TestEmptyAttestationBlockRefusesAnyPath`, `TestAFailedPublicationKeepsTheUnchangedCopy`; `internal/test/fixture` `TestEveryDaemonStoppingFixtureDeclaresStopsDaemon`; `./le doc check all` on the `../wiki` command catalog; `./le spec citation` on `spec-oauth-login-0-umbrella`; `plugin/server` lint unparam at `startup_autoload.go`.

## Conventions that cost time

- Every test, build and lint runs in the main thread, one at a time; agents only read and edit, and hand back the commands.
- Packages under `internal/component/plugin/server` and `internal/component/doctor` need the full feature-tag set (`./le repo changed scope print both`, section `# tags`), or BGP registrations are missing and dozens of tests fail.
- A `.ci` needing uncommitted runner code: `./le --name session test ...`; when another session's mid-edit breaks that rebuild, `./le test ...` (HEAD-built `le`) still builds `ze` from the working tree.
- The doctor-coverage gate refuses a commit that adds a Warn/Error line naming no declared doctor code. Give the line its code; never reshape an existing line to dodge it.
- `plan/` rejects shell writes; a large generated file goes in through the Write tool, split per area if needed.
- A journal row is at most 600 characters; `./le spec journal check file <path>` before the commit.
- `le` panics at startup when an action Summary exceeds 60 characters; load the `le` root in a test after adding an action.
