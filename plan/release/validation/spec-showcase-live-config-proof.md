# Spec: showcase live config proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P2 |
| Release reason | Supported live commits must work regardless of an optional showcase; retain the repaired IRR path as proof, not a presumed current defect. |
| Scope | tooling \| docs |
| Depends | `spec-session-editor-file-mode-parity` (closed 2026-10-10); native amd64/KVM and arm64/HVF kernel VPN proof completed under `spec-appliance-kernel-vpn-modules`. See `docs/architecture/testing/qemu-integration.md`, “VPN facilities without module loading”; live configuration proof remains this spec's work. |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Own the existing IRR commit acceptance and the product-correctness portion of AC-2: configuring each supported topic on a running daemon enables it through the actual entry point. Use the original closed/repaired evidence first (including D-5), and retain already-named session-editor and BGP live-binding owners rather than duplicate their work. A newly reproduced live-commit defect is release work here, never a reason to postpone correctness with the recording.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|---|---|---|
| AC-1 | A running daemon with `bgp-filter-irr` loaded; the operator commits a changed IRR as-set or peer import filter (and, under A-1, a first-time IRR block) | The commit applies: a prefix outside the new set is refused at ingress and absent from Adj-RIB-In, an in-set prefix is accepted. A config the filter cannot apply fails the commit with the reason; the commit never reports success while the filter keeps its old config |

- AC-2: for each supported topic named by original AC-2, a functional test demonstrates absence before its live commit and the promised product effect after it, or cites existing equivalent evidence.
- AC-3: D-1..D-9 are dispositioned against current owners/evidence; a historical observation is not asserted current without that evidence. Recording-only D-9 remains with the presentation parent.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Recordings, publication, shared fragments/validators and demonstration AC-2 presentation plus AC-3..17; preserve existing moved-out13/14 owners. | `plan/post-release/engineering/spec-terminal-demo-showcase.md` |
| Original AC-1 and AC-2 supported-product behavior; release evidence reconciliation for diagnosis findings, not recording production. | `plan/release/validation/spec-showcase-live-config-proof.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Recordings, publication, shared fragments/validators and demonstration AC-2 presentation plus AC-3..17; preserve existing moved-out13/14 owners. | `plan/post-release/engineering/spec-terminal-demo-showcase.md` |

## Provenance and retained evidence

Split from `plan/post-release/engineering/spec-terminal-demo-showcase.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/engineering/spec-terminal-demo-showcase.md`. This child claims no new test result or completion.
