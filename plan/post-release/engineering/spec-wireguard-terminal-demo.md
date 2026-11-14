# Spec: wireguard terminal demo

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | A WireGuard recording and showcase chapter are optional presentation work, not proof of the shipped tunnel. |
| Scope | protocol \| tooling \| docs |
| Depends | Kernel prerequisite complete (`spec-appliance-kernel-vpn-modules`); see `docs/architecture/appliance/kernel-profiles.md`. Product runtime proof remains in `plan/release/validation/spec-wireguard-runtime-proof.md`. |
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
- AC-4: the nine recordings of `spec-terminal-demo-showcase` (`showcase`, `rpki`, `irr-filter`, `bfd-failover`, `ospf-adjacency`, `ipsec`, `vrrp-tracking`, `traffic-anomaly`, `commit-confirmed`) are re-rendered from current sources, `./le site terminal-demo check artifacts` reports them current, and the published showcase transcript reads `commit confirmed 15`.
- AC-5: the re-rendered recordings are reviewed by watching each one, and the vendor mapping on their cards is verified, not guessed, with the reviewer and the source of each mapping recorded.
- AC-6: a test in the privileged renderer container, with FRR installed, makes zebra start with its `zserv.api` socket withheld and asserts that no zebra process remains after `startFRR` (`internal/le/site/terminaldemo/scenarios_network.go`) returns its error.

Amended 2026-11-14 at the closure of `spec-terminal-demo-showcase`, which this
spec extends with a chapter and so re-renders. AC-4: that spec closed on its
validators with the published recordings stale; the owner said "we will not
re-record the terminal yet", which leaves the re-record wanted later. AC-5:
that spec's AC-17 asked for both properties, and it closed on the owner's
order "close terminal-demo-showcase", which stated neither, so both are owed
with the re-record. AC-6: `stopStartedFRR` landed in `8a1f6f5193` with no test.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Original Task 1,2,4; kernel/config/interop proof and evidence-based feature documentation. | `plan/release/validation/spec-wireguard-runtime-proof.md` |
| Original Task 3 only; demo image, standalone recording and showcase integration. | `plan/post-release/engineering/spec-wireguard-terminal-demo.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Original Task 1,2,4; kernel/config/interop proof and evidence-based feature documentation. | `plan/release/validation/spec-wireguard-runtime-proof.md` |

## Owned journal rows

This spec renders a new demo and a new showcase chapter, so it runs the
artifact check the row is about and fixes its answer in the same package.

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| error-names-no-fix-command.md | 2026-11-14 | `./le site terminal-demo check artifacts` stops at the first stale demo and names no render command |

## Provenance and retained evidence

Split from `plan/release/validation/spec-wireguard-runtime-proof.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/release/validation/spec-wireguard-runtime-proof.md`. This child claims no new test result or completion.
