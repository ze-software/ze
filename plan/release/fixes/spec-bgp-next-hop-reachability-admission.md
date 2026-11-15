# Spec: bgp-next-hop-reachability-admission

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | An unresolved BGP next hop must not displace a usable route; changing interior reachability must update selection and installed forwarding state |
| RFC gaps | RFC4271-3.1-2, RFC4271-9.1.2.1-2 |
| Depends | - |
| Updated | 2026-11-15 |

## Task

Complete and review the next-hop admission work exposed by the RFC test
campaign. Retain received paths, exclude unresolved or prospectively
unresolvable next hops before Phase 2 ranking, and recover when interior
reachability changes. Internal and forked plugins must produce the same
selection and kernel result.

The owner narrowed the current campaign on 15 November: build the test
batches, fix straightforward defects, finish investigations already underway,
and retain complicated fixes for later spec review. A failing test remains
visible and is reported. This skeleton owns the remaining investigation and
fixture work; it does not approve an implementation design or claim release
readiness.

No existing spec was found for this mechanism. The proof-debt spec owns
specified old evidence rows, and the dropped-error BGP engine spec explicitly
excludes BGP plugins; neither owns this admission change.

## Authority and Required Reading

- `rfc/full/rfc4271.txt`, Section 9.1.2:
  "If the NEXT_HOP attribute of a BGP route depicts an address that is
  not resolvable, or if it would become unresolvable if the route was
  installed in the routing table, the BGP route MUST be excluded from
  the Phase 2 decision function."
  → Constraint: a currently resolvable next hop is insufficient if installing
  the candidate would make its own resolution recursive or unavailable.
- `docs/architecture/route-selection.md` and `ai/digests/rib.md`
  → Constraint: preserve ranking, received route retention, ADD-PATH identity,
  and reselection on interior changes.
- `docs/architecture/api/process-protocol.md`
  → Constraint: prospective metric proposals are independent, bounded values;
  the preview never installs them.
- `ai/doors/system.md`, `ai/doors/bgp-plugins.md`, `ai/doors/core.md`, and
  `ai/doors/plugin-infra.md`
  → Constraint: forked plugins reach the daemon through the SDK; diagnostic
  RPCs must not hold the route-selection or remote-cache locks.

## Existing Mechanisms

| Mechanism | Producer | Disposition |
|-----------|----------|-------------|
| Normal route admission and election | `internal/core/rib/locrib/prospective.go`, `PreviewSelected` | Reuse the ordinary admission/election to preview one contribution without mutation |
| Recursive next-hop resolution | `internal/core/rib/igpcost/igpcost.go`, `Resolve` and private `resolve` | One bounded resolver loop serves ordinary and concrete prospective views |
| Candidate gathering | `internal/component/bgp/plugins/rib/rib_next_hop.go`, `candidateNextHopResolved` | Check feasibility before ranking; preserve families without address next hops |
| Remote metric lifecycle | `internal/component/bgp/plugins/rib/rib_remote.go`, `runRemoteMetrics` | Reuse the worker for bounded metric RPCs, cache invalidation, reselection and coalesced issue delivery |
| Engine metric RPC | `pkg/plugin/sdk/sdk_engine.go`, `RouteMetricsProspective` | Preserve the same response and validation over both transports |

## Current Evidence

| Observation | Evidence and limit |
|-------------|--------------------|
| Eight new admission/lifecycle tests and the immediate-next-hop reselection test passed | `job-unit-pkg-130c510f.log` and `job-unit-pkg-a904e39b.log` under the campaign session; this proves the unit boundaries, not installed kernel forwarding |
| Unanswered prospective metric request returns within its bound | `job-bgp-next-hop-as0-boundaries-1662fc0c.log`, `TestProspectiveMetricUnansweredRequestIsBounded`, PASS |
| Full affected package run remains red | `job-rfc4271-next-hop-packages-125044b6.log`: 148 top-level and 124 nested failures; the other two affected core packages passed |
| Shared fixture constructors supply no admission Loc-RIB | `newTestRIBManager` in `internal/component/bgp/plugins/rib/rib_test.go` and `newTestRIBManagerWithBus` in `rfc4271_rib_bestchange_test.go`; addressed candidates are now excluded before the old assertions |
| Other fixtures supply an empty Loc-RIB | Receive, self-next-hop, AIGP, ADD-PATH, MED, configuration reselection, ECMP, blackhole, ASPA, validation and shutdown fixtures need individual topology review; this is not a claim that every failing assertion has the same cause |
| Prospective lookup allocated 320 bytes once per call | `job-rfc4271-next-hop-bench-7ba4b266.log`; a concrete-view refactor is written, but remeasurement is owed |
| Unbounded remote waits and missing operator reporting were found | Response deadlines, registered issue reporting and worker tests are written; their scoped execution is owed |
| No resolver-hook bypass defect established | `extractCandidate` calls the existing IGP distance hook before admission. Seed real reachability and rerun unchanged shutdown/locking assertions before changing their control seam |

Logs above live in
`tmp/session/2026-11-14-210034/scratch/`. Retain the package-failure log until
fixture review and reruns classify its failures. Retain the allocation log
until a same-toolchain comparison in one benchmark run supersedes it; timing
figures from a loaded host are not acceptance evidence.

## Remaining Work for Review

1. Audit each failing fixture. Seed explicit independent terminal host routes
   in the actual admission Loc-RIB, preserving table-size assertions and
   avoiding an always-resolved stub. Obtain independent test-judge approval
   before changing shared helpers or RFC-tagged tests.
2. Run unchanged ranking, ownership, allocation and concurrency assertions.
   Distinguish fixture failures from any further product defect at its producer.
3. Validate the concrete prospective resolver has no extra allocation relative
   to ordinary resolution in the same toolchain/architecture run.
4. Run the new issue lifecycle tests over socket RPC and DirectBridge, including
   the unanswered revision-poll deadline and report delivery outside both locks.
5. Run the internal and forked kernel drafts in `test/draft/plugin/`, named
   `rfc4271-next-hop-kernel.ci` and `rfc4271-next-hop-kernel-forked.ci`, then
   strengthen/promote them through the functional-test workflow. Prove usable
   fallback, withdrawal when none resolves, and restoration without a new UPDATE.
6. Record actual passing-baseline/failing-break proofs before adding or upgrading
   RFC claims. Keep the mandatory gap rows honest until whole-path evidence exists.

## Risks and Assumptions

| ID | Concern | Validation |
|----|---------|------------|
| A-1 | Common empty-topology setup explains many reds, but has not classified every failure | Review each failing unit and rerun after its approved topology correction |
| R-1 | A broad reachable default hides prospective recursion defects | Seed independent host reachability and retain explicit self/mutual-recursion negatives |
| R-2 | Reporting while selection holds a lock deadlocks the plugin | Worker tests must acquire both locks from report callbacks on both transports |
| R-3 | A unit preview passes while the daemon still installs an unresolved route | Assert actual kernel state in both daemon modes |

## Review Gate

Research is recorded. Design review, fixture approvals, kernel evidence and
closure remain outstanding. No full RFC support or completed batch is claimed.
