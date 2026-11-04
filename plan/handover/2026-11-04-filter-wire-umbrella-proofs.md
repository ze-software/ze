# Filter-wire umbrella proof handoff

**Blocked only on A-6:** Thomas must run `./le perf filter measure` on his Mac and supply a comparable same-Mac text-era baseline. Neither measurement is available here. Unit tests, daemon behavior, load testing, harness compilation and source discrimination do not establish that relative-performance claim. No scope reduction, performance result or closure is implied.

## Handoff

**Spec:** `plan/post-release/engineering/spec-filter-wire-0-umbrella.md` — `blocked`, updated 2026-11-04.
**Branch:** `main`.
**Goal:** Finish the umbrella's promised behavioral proofs without weakening any acceptance criterion, then resume closure only after the owner supplies A-6 evidence.

### Status

- **Done, committed:**
  - `8869b196d201`: four new consumer-visible unit proofs, with scoped race results and independent judgment of the six community-header oracle corrections.
  - `d4539a9be2d3`: actual forked-IRR config-delivery fix and documentation. `runFilterIRR` now requests both `bgp` and `policy` in Stage 1; an external program cannot inherit the compiled registration's `ConfigReads`. Missing `policy irr` settings had rejected announcements the internal run admitted.
  - `f0b04c142890`: functional proof chunk and final spec bookkeeping; nine promoted tests, fixtures/test plugin, transport/admission metrics, relevant docs and only the two owned `ai/INDEX.md` hunks. Main reviewed the shared tracked diff and excluded foreign changes.
- **Done, observed:** the scoped unit/substitute selections pass; A-2/A-8 are confirmed; all nine tests are promoted and pass at normal paths; eight plugin proofs and the parse proof each completed 80 invocations under load; six source-overlay experiments fail at runtime assertions and all nine canonical consumers pass after restoration.
- **In progress:** no implementation changes or CPU jobs. The source and functional proof work is committed; only A-6's external evidence and subsequent closure remain.
- **Remaining:** A-6's owner-run Mac comparison, truthful evidence-driven spec update, then `/ze-close` with its independent Review Gate and ordinary closure obligations. No implementation edit is currently justified.
- **Not done / intentionally outside scope:** rejected-import-route storage remains owned by `plan/post-release/features/spec-bgp-filtered-route-storage.md`. R-13 still bounds compiled-in third-party Go filters to code linked into the tree; no public out-of-tree registration/view API was added. Existing IRR accepted-route refresh limitations remain the spec's Known Limitations, not newly waived acceptance criteria. No promised proof was dropped.

### Promoted functional proofs

| File | Proof |
|------|-------|
| `test/plugin/filter-wire-inprocess-import.ci` | Cached post-import LOCAL_PREF 200, exact receiver bytes, actual typed-slot versus encoded-route counts |
| `test/plugin/filter-wire-dryrun.ci` | Mixed internal/external trace, final decision and live UPDATE-byte agreement |
| `test/plugin/filter-wire-intree-forked.ci` | AC-22: same family evaluator internal/forked, identical verdicts/UPDATEs, both transports counted |
| `test/plugin/filter-wire-irr-two-transports.ci` | AC-31: in/out/mixed IPv4, withdrawal and mixed MP-family cases, identical five receiver frames and six judgments per transport |
| `test/plugin/filter-wire-batch.ci` | Real bounded batches, positional verdicts/sets, malformed-batch isolation/recovery and same-batch refused-set isolation |
| `test/plugin/filter-wire-undeclared-set.ci` | Atomic rollback of four refused-set classes, unchanged values under on-error accept, exact refusal count |
| `test/plugin/filter-wire-new-fields.ci` | Four D-8 fields on both transports and declared-only projection |
| `test/plugin/filter-wire-set-actions.ci` | Community add/remove and MED/LOCAL_PREF arithmetic, identical full receiver frames |
| `test/parse/filter-wire-metric-at-bgp-refused.ci` | Both BGP attachments refuse metric with actionable diagnostics; non-BGP control accepts it |

Post-promotion evidence is under `tmp/session/2026-11-04-111704/scratch/`:
- `job-fw-promoted-plugin-17a8d3b2.log`: eight new plugin proofs **plus existing `clear-irr-cache-external`**, 9/9 PASS.
- `job-fw-promoted-parse-379b61c3.log`: normal-path parse proof, 1/1 PASS.

### Evidence already established — do not rerun merely to reconfirm it

Older owned evidence remains under `tmp/session/2026-11-01-174538/scratch/`; that directory does not identify the current session's spec claim.

- **Unit proofs:** `job-fw-proof-unit-a4173714.log`, `job-fw-proof-reactor-292513d4.log`, `job-fw-proof-plain-sets-dfcb7879.log`, `job-fw-proof-contracts-a793e314.log`, `job-fw-proof-transport-cc04e2da.log`, `job-fw-proof-admission-88d62c28.log`. The spec distinguishes initial compile/oracle failures from the corrected named PASS results. A-2 covers eight plain fields/18 actions at whole-UPDATE bytes; A-8 covers 17 non-CIDR families in both MP sections under both verdicts.
- **Completed load:** `job-fw-stress-plugin-final-a0c0ae67.log` reports all 80 plugin invocations without reproduction. Full capture: `tmp/stress-repro/plugin-draft-pattern-filter-wire-p-2-161b42949d133f2ff6b75a5855d86d9000dba76ff34e8d9e1868ad6c5a23f9df-20261104-110120-4126909445.log`. Parse: `job-fw-stress-parse-7b438323.log`, full capture `tmp/stress-repro/parse-draft-pattern-filter-wire-metric-at-bgp-refused-p-1-a8b8c6f4be165baf30245d7281000a3613cbddd17a5993c2926acf1a9e77e2f1-20261104-102022-940363179.log`. Stress-repro's non-reproduction exit status is not a failed functional test; read the completed invocations and named results.
- **Discrimination:** `fw-discrimination/observed-discrimination.json` records six compiled, runtime-red overlays: aggregating ASN projection, metric refusal, one-route-only batch draining, undeclared LOCAL_PREF admission, missing forked-IRR policy config, and erased typed-slot verdict/edits. It carries exact arguments, source hashes and unchanged-source receipts. No canonical test assertion/source was mutated. Canonical restoration: `job-fw-restored-plugin-db0fbfc6.log` (8 PASS) and `job-fw-restored-parse-c6c946fd.log` (1 PASS).
- **Inherited D-16 replacement:** `test/reload/role-upstream-check.ci` passed on 2026-11-03 at CI checkout `1a2ccaeef8c0da4dc9f3747ce70f4aeecb84ddfa`: `ci-41685742426-shard1.log:7169`, checkout SHA at line 200. The current test and `role/upstream.go` were bounded unchanged against that revision. This is inherited dynamic-peer/`show policy test`/reload coverage, not one of the nine newly promoted proofs. Owner commit `655d07b59b` explicitly replaced the obsolete reject-asn load-time obligation and approved deleting its old test.
- **Committed build results:** `job-fw-proof-committed-63ed8ad7.log` covers all five flavors at `8869b196d201`; `job-fw-irr-committed-e6594260.log` covers all five at `d4539a9be2d3`. Final `tmp/session/2026-11-04-111704/scratch/job-fw-proof-committed-final-9b56138a.log` explicitly names HEAD `f0b04c142890` and reports distro, appliance, setup, host and installer all OK, followed by “every flavor of the committed tree compiles.” No `./le verify` was run.
- **Independent reviews:** `fw-proof-review-report.md`, `fw-proof-review-round2-report.md`, `fw-proof-review-round3-report.md`, `fw-proof-review-round4-fix-report.md`; final integration approval is `tmp/session/2026-11-04-111704/scratch/fw-proof-review-final-integration.md`. The final stale inherited-proof status was independently resolved. Test approvals, including the community header literals and the narrowly authorized post-proof administrative shutdown, are in `tmp/commit-weakening-approved-5112be1d.md`. These are scoped source/docs/test reviews, **not a CLEAN closure artifact or the closure Review Gate**.

The failed load attempts remain useful history, not success evidence. The IRR fixture now fences IPv4 receiver acknowledgments before releasing MP inputs and settles initial sync through existing quiescence before taking baselines. Its same five byte goldens remain. The batch proof authorizes Cease/Administrative Shutdown only after all checks, while retaining early/other notification failures and its final failure assertion. No timeout increase or assertion removal substitutes for those readiness fixes.

### Files already handled — start from these facts, not another audit

- The umbrella spec: exact twelve-name unit reconciliation, current promoted paths, A-2/A-8 confirmation, A-6 blocker, failure history and proof receipts.
- `internal/component/bgp/reactor/filter_plain_sets_wire_test.go`, `internal/component/bgp/reactor/filter_raw_families_test.go`, `internal/component/bgp/reactor/filter_unavailable_policy_test.go`, `internal/component/bgp/plugins/filter_prefix/condition_locrib_parity_test.go`: the four new units.
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` `runFilterIRR`; `internal/component/plugin/server/startup.go` `registryRow`/`registryConfigReads`: config-delivery diagnosis and source fix. Current `docs/architecture/bgp/filter-irr.md` documents both roots.
- `internal/test/fixture/plugin_fixture_filter_wire_transports.go` `filterWireTransports`: actual cached forwarding, byte acknowledgments, quiescence and transport baselines.
- `internal/test/plugins/filterwirecases/`, the promoted `.ci` files and their fixture drivers: semantic behavioral cases, not wiring/source-text assertions.
- `internal/component/plugin/ipc/filter_slot.go`, `internal/component/plugin/ipc/filter_batch.go`, `internal/component/plugin/ipc/filter_batch_metrics.go`; `docs/plugin-development/metrics.md`: completed typed judgments, encoded batches and successful queue admissions are distinct counters.
- `internal/le/perf/filter.go` and `pkg/plugin/sdk/filter_bench_test.go`: the owner-only measurement harness and its limits, already traced below.

Re-read a file only when new evidence or a changed hash invalidates these facts. Existing foreign/shared-checkout edits are not this spec's work; no clean-working-tree claim is made.

### Edits

**Zero current implementation edits.** A-6 depends on unavailable owner/hardware evidence, not a guessed code fix. Do not prewrite a confirmed/broken outcome, invent numbers, replace the comparison with a compile check, or weaken the assumption. An exact evidence-driven spec replacement cannot truthfully be supplied before the measured result exists.

After Thomas supplies the result, record the observed outcome and artifact/revision references in the spec's **Blocker** and **A-6** rows. If it confirms A-6, resume `/ze-close` and its independent Review Gate; if it breaks A-6, diagnose the measured producer/design before changing code. Keep every original acceptance criterion. The open 2026-11-02 journal rows in `second-logger-detaches-runtime-level.md` and `full-disk-false-red.md` remain owned until closure rehomes them; do not rehome them merely because this proof handoff is committed.

### Then

**Thomas's Mac only:** with the repository Go toolchain and `CGO_ENABLED=0`, run `./le perf filter measure` and preserve its full output and source revision. `measureFilters` refuses a non-Darwin host; `filterArgs` selects SDK fast-slot and batches of 1/16/64 with `-benchmem -count=10`. The batch benchmark measures marshal/evaluate/unmarshal over supplied fields, not subprocess launch or the deleted text-subject work. A comparable same-Mac/toolchain/workload text-era before result is still required; normalize batch cost per route. Removed text rendering benchmark source existed before `2e5c2d012a`; the current similarly named modify benchmark is not that baseline. There are no configured SSH hosts for an agent to obtain the missing run. No performance measurement or number is supplied by this session.

**Agent verification now:** none. The scoped proofs above are exercised evidence; do not rerun them to reconfirm reported results and do not run `./le verify`. The final committed-tree compile result is recorded above; no CPU job remains to await. After an evidence-driven future edit, follow scoped verification rules and `/ze-close`; no full local verification is pre-authorized. The Mac measurement duration is owner-controlled, not an agent job to launch or time out here.

### Session ownership and retained resources

- Finishing session ID: `01a1a5be-a56f-734c-beaf-07dd6a1d224a`, historical identity only; a fresh session uses its own real harness ID and claim. The rebuildable native CLI cache `bin/le-s-01a1a5be/le` (167.9 MB) is retained for metadata and commit tooling. The normal hourly store trim removes it after a day without writes and once no process references it; it is not a pinned performance result or a custom comparison binary.
- Actual state path: `tmp/session/2026-11-04-111704/state/session-state-filter-wire-0-umbrella-01a1a5be-a56f-734c-beaf-07dd6a1d224a.md`.
- The old `010c656c-7d73-4e34-9923-47e8f6a7475d` marker belongs to another Home-CPE session. Never overwrite/release it. Preserve its other state, files and shared `bin/le-s-010c656c`; only the unique filter-wire receipts/state written earlier in that namespace are ours.
- Main identified and terminated only the three peers left by this session's canceled stress job, PIDs `2566970`, `2566995`, `2567036`, using the exact `ZE_RUN_JOB` identity ending `fw-stress-plugin-phased.2545700.2082545166.job`; its subsequent process check found none. No owned server remains to stop; do not repeat cleanup by broad process matching.
- Retain owned `fw-*` raw logs, scoped review reports, overlay originals/mutants and `observed-discrimination.json` until the A-6/closure audit has consumed the comparisons and durable citations. Their purpose is to distinguish real behavioral reds/restoration from compile or harness failures. Normal owned-artifact cleanup may remove them only after that audit no longer needs them. No large tree copies or custom comparison binaries are retained; do not delete another session's temporary files or shared tooling.
