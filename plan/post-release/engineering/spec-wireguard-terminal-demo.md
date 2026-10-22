# Spec: wireguard terminal demo

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | A WireGuard recording and showcase chapter are optional presentation work, not proof of the shipped tunnel. |
| Scope | protocol \| tooling \| docs |
| Depends | `plan/release/delivery/spec-appliance-kernel-vpn-modules.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Produce the separate WireGuard terminal demo and then the showcase chapter requested by original Task item 3. Consume the release runtime proof; a recording is not a replacement for the functional and interop evidence. Include wireguard-tools in the demo image if the chosen far end needs it.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: a dedicated WireGuard demo id shows the configured tunnel and working traffic.
- AC-2: the showcase includes a WireGuard chapter after runtime proof lands.
- AC-3: the demo image provides its chosen peer tooling and the published recording agrees with the proven feature level.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Original Task 1,2,4; kernel/config/interop proof and evidence-based feature documentation. | `plan/release/validation/spec-wireguard-runtime-proof.md` |
| Original Task 3 only; demo image, standalone recording and showcase integration. | `plan/post-release/engineering/spec-wireguard-terminal-demo.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Original Task 1,2,4; kernel/config/interop proof and evidence-based feature documentation. | `plan/release/validation/spec-wireguard-runtime-proof.md` |

## Provenance and retained evidence

Split from `plan/release/validation/spec-wireguard-runtime-proof.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/release/validation/spec-wireguard-runtime-proof.md`. This child claims no new test result or completion.
