# Bug Review Final Report

Generated: 2026-06-19
Parent spec: `plan/spec-bug-review-0-umbrella.md`
Final backlog spec: `plan/spec-bug-review-5-verification-and-fix-backlog.md`

## Summary

The bug review program completed the inventory pass, three area reviews, deduplication, and fix-spec creation. No production code, tests, generated files, or user documentation were changed. The deliverables are review reports and follow-up bugfix specs.

| Result | Count | IDs |
|--------|------:|-----|
| Accepted findings with fix specs | 10 | SYS-001, SYS-002, SYS-003, BENG-001, BENG-002, BENG-003, BENG-004, BENG-005, BPLUG-001, BPLUG-002 |
| Plausible findings not promoted | 4 | SYS-004, SYS-005, BPLUG-P1, BPLUG-P2 |
| Inventory observations, no production bug | 2 | INV-OBS-1, INV-OBS-2 |
| Child reports consumed | 4 | inventory, plugin/system, BGP engine, BGP plugins |

## Source Artifacts Loaded

| Artifact | Role | Status |
|----------|------|--------|
| `plan/review-bug-review-inventory.md` | generated import, directory, registry, and child assignment ledger | loaded |
| `plan/review-bug-review-plugin-engine-system.md` | child 2 review | loaded |
| `plan/review-bug-review-bgp-engine.md` | child 3 review | loaded |
| `plan/review-bug-review-bgp-plugins.md` | child 4 review | loaded |
| `plan/spec-bug-review-0-umbrella.md` | parent acceptance criteria | loaded |
| `plan/spec-bug-review-5-verification-and-fix-backlog.md` | final backlog acceptance criteria | loaded |

## Audit Tests

| Test | Status | Evidence |
|------|--------|----------|
| FinalReviewAllChildReportsLoaded | PASS | all four child report artifacts listed above were read and consumed |
| FinalReviewDedupHasRootCause | PASS | accepted findings are grouped by root cause and fix spec below |
| FinalReviewAcceptedFindingsHaveFixSpecs | PASS | every accepted finding maps to exactly one `plan/spec-bugfix-*.md` file |
| FinalReviewRejectedCandidatesHaveProof | PASS | plausible and rejected candidates are listed with source report proof |
| FinalReviewInventoryCoverageZeroMissing | PASS | inventory report records unassigned count 0 and exclusions with reasons |
| FixSpecRegressionPlanPresent | PASS | each fix spec contains acceptance criteria and a TDD or regression plan |

## Inventory Coverage Closure

| Inventory class | Child owner | Final status |
|-----------------|-------------|--------------|
| Generated import rows from `internal/component/plugin/all/all.go` | child 1, assigned to children 2 through 4 | PASS, 258 rows accounted, in-scope unassigned count 0 |
| Directory-only command providers | child 2 | PASS, command roots wired through `cmd/ze` and classified separately |
| Plugin engine, SDK, system plugins, non-BGP component plugins | child 2 | PASS, report produced SYS findings and cleared classes |
| BGP core engine and protocol core | child 3 | PASS, report produced BENG findings and cleared classes |
| BGP plugin packages, NLRI families, BGP command plugins | child 4 | PASS, report produced BPLUG findings and cleared classes |
| Final backlog and fix specs | child 5 | PASS, this report and bugfix specs created |

## Accepted Findings and Fix Specs

| Finding | Severity | Root cause | Fix spec | Notes |
|---------|----------|------------|----------|-------|
| SYS-001 | BLOCKER | plugin startup can mutate registries and then report success after a failed stage | `plan/spec-bugfix-sys-plugin-lifecycle-rollback.md` | paired with SYS-002 because both require exact-or-reject lifecycle cleanup |
| SYS-002 | ISSUE | reload failure cleanup passes plugin names to a config-root cleanup API | `plan/spec-bugfix-sys-plugin-lifecycle-rollback.md` | same lifecycle rollback spec |
| SYS-003 | ISSUE | DirectBridge callback panic does not send a result to the engine caller | `plan/spec-bugfix-sys-directbridge-panic.md` | isolated bridge panic handling spec |
| BENG-001 | BLOCKER | malformed known capabilities and truncated TLVs are ignored or accepted during OPEN negotiation | `plan/spec-bugfix-bgp-message-validation-before-delivery.md` | paired with BENG-003 as receive validation before delivery |
| BENG-003 | ISSUE | malformed ROUTE-REFRESH reaches callbacks before body validation | `plan/spec-bugfix-bgp-message-validation-before-delivery.md` | same validation-before-delivery spec |
| BENG-002 | BLOCKER | oversized forwarding splits source-context bytes before destination context conversion | `plan/spec-bugfix-bgp-forward-split-context.md` | protocol corruption risk, standalone spec |
| BENG-004 | ISSUE | late reactor startup failures can leak listeners/cache/subscriptions | `plan/spec-bugfix-bgp-reactor-startup-cleanup.md` | lifecycle cleanup spec |
| BENG-005 | ISSUE | IPv6 link-local next-hop-self builds a 32-byte slice on forwarding hot path | `plan/spec-bugfix-bgp-next-hop-alloc.md` | accepted as allocation-confirming fix spec because the code path is concrete |
| BPLUG-001 | ISSUE | NLRI encode/config parsers silently ignore unknown or dangling tokens | `plan/spec-bugfix-bgp-nlri-strictness.md` | strict parser spec across labeled, MUP, MVPN, and VPLS |
| BPLUG-002 | ISSUE | SR-Policy family lacks canonical route encoder registration | `plan/spec-bugfix-bgp-srpolicy-encode.md` | owner-package family-chain wiring spec |

## Fix Spec Ledger

| Fix spec | Findings covered | Required regression proof |
|----------|------------------|---------------------------|
| `plan/spec-bugfix-sys-plugin-lifecycle-rollback.md` | SYS-001, SYS-002 | startup failure rollback, capability atomicity, reload auto-stop cleanup |
| `plan/spec-bugfix-sys-directbridge-panic.md` | SYS-003 | bridge callback panic returns prompt non-timeout error and cleanup closes bridge |
| `plan/spec-bugfix-bgp-message-validation-before-delivery.md` | BENG-001, BENG-003 | malformed capability OPEN rejected, malformed ROUTE-REFRESH not delivered before validation |
| `plan/spec-bugfix-bgp-forward-split-context.md` | BENG-002 | ADD-PATH and ASN4 context-mismatch split encodes for destination |
| `plan/spec-bugfix-bgp-reactor-startup-cleanup.md` | BENG-004 | late startup failure cleans listeners, context/cache, and safe Stop-after-failure |
| `plan/spec-bugfix-bgp-next-hop-alloc.md` | BENG-005 | IPv6 link-local next-hop-self byte equality and allocation proof |
| `plan/spec-bugfix-bgp-nlri-strictness.md` | BPLUG-001 | each affected parser rejects unknown and dangling tokens |
| `plan/spec-bugfix-bgp-srpolicy-encode.md` | BPLUG-002 | registry and canonical `ze bgp encode` SR-Policy IPv4/IPv6 tests |

## Plausible Findings Not Promoted

| Finding | Reason not promoted | Future route |
|---------|---------------------|--------------|
| SYS-004 | Source shows initial autoload downgrades dependency resolution errors to warnings, but no current in-scope plugin with a missing hard dependency was verified. | Promote only with a concrete registered dependency failure or build-tag omission. |
| SYS-005 | VPP DPDK parser itself drops unknown nested keys, but normal user config appears to pass through YANG validation that should reject typos first. | Revisit in a direct-parser API audit or VPP parser hardening pass. |
| BPLUG-P1 | BGP-LS lacks direct `InProcessNLRIDecoder`, but CLI decode has subprocess/direct fallback and no user-visible failing path was proven. | Promote if a command/API path requiring `registry.DecodeNLRIByFamily` fails end-to-end. |
| BPLUG-P2 | MVPN, RTC, and BGP-LS encode completeness needs a product decision that those families must support canonical encode today. | Decide encode support policy by family before writing fix specs. |

## Rejected or Cleared Candidate Classes

| Area | Cleared proof source | Disposition |
|------|----------------------|-------------|
| TFTP and image server traversal | child 2 report path validation reads | rejected, path cleaning and root checks present |
| Generic command root import gaps | inventory plus child 2 command-root evidence | rejected, directory-only roots are wired by `cmd/ze` imports |
| UPDATE delivery before RFC 7606 validation | child 3 receive matrix | rejected, UPDATE validation runs before callback |
| cache buffer release on normal eviction | child 3 rejected candidate R2 | rejected, original and EBGP buffers returned on eviction/delete |
| unlimited DirectBridge destinations | child 3 rejected candidate R3 | rejected, forward destination cap exists |
| FlowSpec unknown wire component default | child 4 rejected candidate H1 | rejected, invalid type errors out |
| BMP retaining zero-copy UPDATE bytes | child 4 zero-copy/lifetime proof | rejected, writes are synchronous and OPEN cache copies bytes |
| RS optional dependency on Adj-RIB-In | child 4 rejected candidate | rejected, optional dependency is declared and documented |

## Active Spec Overlap Routing

| Active spec | Review impact |
|-------------|---------------|
| `spec-exabgp-compat-sync.md` | No accepted finding directly changes ExaBGP compatibility. SR-Policy encode spec names the existing ExaBGP compatibility fixture as parity evidence only. |
| `spec-route-config-plugin-migration.md` | No accepted finding directly changes route config plugin migration. SYS lifecycle and NLRI parser specs may touch config reload or route config tests, but they are bugfix specs with their own acceptance criteria. |

## Risk Register

| Risk | Status | Mitigation |
|------|--------|------------|
| Review found defects but did not patch production code | intentional | follow-up bugfix specs created with tests and ACs |
| BENG-005 allocation may not escape under compiler analysis | captured | fix spec requires allocation proof before broader refactor |
| Broad plugin surface has OS-specific runtime behavior not executed on Darwin | captured | fix specs require targeted unit or QEMU/Linux tests when touching OS resources |
| Product policy for decode-only NLRI families remains undecided | captured | BPLUG-P2 not promoted without decision |

## Completion Against Parent Acceptance Criteria

| Parent AC | Status | Evidence |
|-----------|--------|----------|
| AC-U-1 inventory every plugin import row and adjacent registration surface | PASS | `plan/review-bug-review-inventory.md` |
| AC-U-2 child reviews cover plugin engine/system, BGP engine, and BGP plugins | PASS | three area reports |
| AC-U-3 no duplicated scope or orphaned plugin rows | PASS | inventory assignment count 0, child scope tables |
| AC-U-4 findings include evidence, impact, severity, and regression plan | PASS | child reports and fix specs |
| AC-U-5 accepted findings become implementation specs | PASS | fix spec ledger above |
| AC-U-6 rejected candidates have proof | PASS | plausible and rejected tables above |

## Final Status

The review program is complete at the artifact level: inventory, child reports, final report, and eight fix specs exist. Production bugs are not fixed here by design; each accepted finding now has a dedicated implementation spec with tests and acceptance criteria.
