# Spec: ipsec-vpp-dual-form-proof

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | protocol validation |
| Priority | P3 |
| Release reason | Real-VPP dual-form evidence gates optional operator-selectable VPP, not shipped XFRM. |
| Depends | plan/post-release/features/spec-ipsec-vpp-policy-interface.md |
| Phase | 5/5 (inherited sequence, not completion) |
| Updated | 2026-10-22 |

## Task and ownership

Own AC-5 split from `plan/release/validation/spec-ipsec-esp-dual-form-receive.md`,
including its VPP A-6/AC-6 assumption evidence, R-4 policy dependency, Phase 4
VPP capability handling and real-VPP harness extension. The parent retains
all XFRM evidence, Stories 1..3, throughput R-1, A-7 bookkeeping, durable
Design links and ledger regeneration. Its complete original design, test
proposals, provenance and dated findings remain the historical record;
VPP passages there are non-owning references to this child.

The production dataplane is XFRM. `spec-ipsec-dataplane-selector` records that
VPP is unavailable to operators and must not become selectable before the
policy-interface and real-ESP prerequisites hold. Its real-VPP proof
prerequisite now names this owner; installation and SA read-back do not clear it.

## Acceptance and proof

- AC-5: a real VPP receives bare and UDP-encapsulated ESP on the same inbound
  SA and decrypts both. Extend the current native `(*VPP).runIPsec` producer
  in `internal/le/deployment/vppevidencerun.go`, registered by `scenarioRuns`
  in `vppevidence.go`; the older `run_ipsec_evidence` name is provenance.
- Preserve the capability-error obligation: a backend measured unable to
  receive both forms must fail `ze config verify` with a named capability
  error. Refusal is not a substitute for the measurement or support.
- Resolve VPP A-6 with real evidence. The August source-reading claim was
  withdrawn by its own producer; neither the old confirmed cell nor the
  former not-applicable verdict proves reception. Record the result with
  the harness and actual same-SA decryption evidence.
- Retain the policy-interface prerequisite and its AC-1..AC-5 before exposing
  VPP to operators. R-4 records that `SPParams.IfIndex == 0` currently refuses
  IKE policies; removing that accidental guard cannot silently discharge
  the dual-form obligation.

## Decisions and scheduling preserved

The 2026-08-29 withdrawal leaves AC-5 unresolved. The 2026-09-05 ruling is
MEASURE IT, not a choice between measurement and refusal. The 2026-10-09
owner set-aside ("Set aside the 7 specs waiting on something else") and
owner defer-heavy-testing instruction remain effective. This child is blocked,
not newly commissioned or ready. The release-scope rules in `ai/META-DECISIONS.md` change release placement only.

The parent's original Phase 4 refusal-only test proposals and all checklist
references to VPP are governed by that September ruling. No RFC artifact,
verdict, evidence status or tagged-test authorization is changed by this split.
