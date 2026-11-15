# Handover 40: fetched data survives a restart

Spec: `plan/release/fixes/spec-fetched-data-survives-restart.md` (in-progress,
claimed by session bb434b3e, approved by Thomas 2026-11-15, authorised above the
WIP cap). Per-phase handoffs, file digests and every agent's owed commands:
`tmp/session/2026-11-15-023133/state/session-state-fetched-data-survives-restart-bb434b3e-5d37-4a2e-841c-a015ab81dd1b.md`.
Read that file before any source.

The session handed off because its context passed the 600k threshold, not
because the work is blocked.

## Landed

| Commit | Content |
|--------|---------|
| 683b848cd8 | The approved spec |
| 0f1dc02e4f | Phase 7: firewall-domain stores expires-at on every answer; a restart re-resolves a name only after its stored expiry; unreadable/unsaved doctor codes |
| 04dc7a7e26 | Phases 1+2: `irr` owner plugin skeleton (`system irr`), key registry with per-key value bound (dataset 12,000,000 bytes), compact binary dataset codec with SHA-256 digest, read-only and one-instance state grants |
| 57bde3372f | Phase 7 follow-up: an unsaved DNS answer is raised on the report bus until saved |

## In the working tree, not committed

Phases 3, 4 and 5 must land together in one commit: phase 3 converts and
removes the legacy `meta/irr/{name}` keys that firewall-irr reads until phase 5.

| Phase | State |
|-------|-------|
| 3 owner (`internal/plugins/irr/`) | Green: package tests, lint, a break of `dueOf` turned TestDueFromCheckedAt and TestRestartReadsStoredNoFetch red. Strengthened `TestUpdateFailurePreservesState` (discovery_test.go) passes; its discrimination break (comment out `o.retryAt[asSet] = now.Add(retryInterval)` in owner.go refreshSet) was NOT run yet because a phase-5 agent was editing owner.go |
| 4 BGP consumer (`internal/component/bgp/plugins/filter_irr/`, `store/consumer.go`) | Green with the two old files set aside: `command_test.go` and `cmd_irr_args_test.go` must go in the commit's `remove` list (judge-approved). Race tests pass; a break of the `st.configured != target.configured` guard turned the reload-race tests red. Lint clean |
| 5 firewall consumer (`internal/component/firewall/plugins/irr/`) | A continuation agent was running when the session ended: the judge's 10 refusals, a panic-recovery product fix in owner `run` and firewall `storeReader`, deletion of callerless `PrefixStore` persistence and `zefs.KeyIRRCache`, pages left wrong. Read its handoff in the state file |

## Test judgments

Approvals are recorded in `tmp/commit-weakening-approved-783f3a77.md` (42 rows
at hand-off, checked: no bogus row). That file is keyed to commit session
`783f3a77`; a new session may need `./le commit create session 783f3a77` to
reuse it, or the judges re-record.

| Unit | State |
|------|-------|
| filter_irr.TestUpdateASNFailurePreservesState | Round 4 judge was running on the strengthened replacement (asn-path retry and rowFailing added). If refused again, fix as it says |
| Phase 5 refusals (metrics, panic, table term, show empty/stale, clear absent, clear notation, doctor stale, ArgsOrSelector) | Being replaced by the phase-5 continuation; its replacements go to a judge before commit |
| All other phase 4/5 units | Approved and recorded |

An agent in phase 4 edited tests through a python script, bypassing the
weakening hook; every one of those changes has since been judged. Brief every
agent to edit tests with Edit/Write only.

## Remaining, in order

1. Read the phase-5 continuation's handoff; run its owed commands; send its
   replacement tests to one ze-test-judge.
2. Run the phase-3 retry discrimination break.
3. Regenerate nothing with `-update`: add only this spec's rows to
   `internal/component/plugin/all/testdata/*.snapshot` (other sessions' rows are
   pending there). Phase 4 removes the three `ze-bgp:update-irr-*` rows, phase 5
   the `ze-firewall:update-irr-*` and `clear-irr-*` rows.
4. Commit phases 3+4+5 together, with `remove` for the two filter_irr test
   files and every file the phase-5 continuation lists; use `hunks` for any file
   carrying another session's hunks (all.go, ze-plugin-engine.yang,
   process-protocol.md, functional-tests.md had foreign hunks earlier).
5. Phase 6: remove `policy irr { server; source-address; peeringdb-url;
   refresh-interval }` and `firewall irr { server; peeringdb-url;
   refresh-interval }`, convert every repository config, `.ci`, fixture and doc
   to `system irr` / `system peeringdb url` / `update irr` / `clear irr` in the
   same commit. Expected-red tests until then are listed in the phase-4 handoff
   item 10.
6. Promote `test/draft/plugin/irr-restart-keeps-bgp-list.ci` and
   `test/draft/plugin/firewall-domain-group-restart-keeps-answers.ci` after a
   green run; run `./le test qemu netns suites plugin` once for the firewall and
   DNS restart tests.
7. Spec bookkeeping in one agent that reads the doors and producer files the
   spec-edit hook demands: Phase marker, A-N verdicts (A-4 confirmed with the
   await-exit barrier, A-5 broken: mock needed --query-log), Mistake Log rows,
   Design Insights from the handoffs.
8. `/ze-close` in a fresh session.

## Findings outside this spec

| Finding | Owner |
|---------|-------|
| `./le repo update` stops at "no such action in web vendor: sync" | Another session's tooling change |
| `./le arch tier check` fails on `internal/le/interoplab/bgp/ze_ready.go` | Another session |
| `./le arch fs-persistence check` fails on ike and skills files | Other sessions |
| `TestNoIngressFilterGatesAPeerRouteOnRedistributionConfig` (bgp-gr needs policy, 44cb535338) | Pre-existing |
| `show doctor` can list a plugin runtime-check problem twice (runtime check and recent-events): `runtimeCheckCodes` in `internal/component/doctor/checks_recent_events.go` hides only in-tree checks. OSPF has the same | Doctor component; owed a journal row if not fixed here |
| A single `./le test bgp <suite> --pattern` run uses the launcher's cached `le`, which lacks fixtures added since its build: run with `bin/le-s-<sid>/le` after `./le --name session ...` rebuilds it | Test tooling |
