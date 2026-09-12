# Spec: ipsec-vpp-dataplane-readback

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol, cli |
| Depends | - |
| Phase | SCOPE approved; RESEARCH and DESIGN pending |
| Handoff | - |
| Owner | IKE component, VPP dataplane readback |
| Updated | 2026-09-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Implement VPP Security Association Database (SAD) and Security Policy Database
(SPD) enumeration behind `Dataplane.ListSAs` and `Dataplane.ListPolicies`.
The existing public inspection commands and their consumers must report what
VPP holds. Install-time bookkeeping cannot supply that answer.

This file captures the owner-approved scope. Research and design remain open,
and this spec authorizes no implementation. The owner permits the first release
to ship XFRM-only inspection, so this work belongs in the root `plan/` bucket.

## Ownership and Inherited Work

This spec owns the VPP readback feature inherited from the inspection spec's
2026-08-02 deferral row. The source was
`spec-ipsec-dataplane-inspection`, under
`Work Inherited From a Deferral Row`. The owner separated this feature from the
XFRM inspection closure and required a separately owned backlog item.

| Boundary | Ownership |
|----------|-----------|
| VPP SAD/SPD enumeration, record conversion, and readback consumers | This spec |
| XFRM inspection and its proof | The existing inspection work, documented in `docs/architecture/ike/ipsec-dataplane-inspection.md` |
| Operator selection of the IPsec backend | `plan/spec-ipsec-dataplane-selector.md` |
| Supplying the VPP policy interface and correcting the resulting rekey lifecycle | `plan/spec-ipsec-vpp-policy-interface.md` |

The related specs retain their scope. Research must establish their dependency
order before this feature can claim operator reachability. A private backend
selection in a proof does not complete the public selector feature.

## Required Reading

- [ ] `docs/architecture/ike/ipsec-dataplane-inspection.md`
  → Constraint: Failed reads remain errors or unknown observations. Counter and drift consumers must retain destination-qualified identity and generation consistency.
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`
  → Constraint: Readback uses the existing dataplane abstraction. VPP interface binding and XFRM interface identity have different meanings.
- [ ] `plan/spec-ipsec-dataplane-selector.md` and `plan/spec-ipsec-vpp-policy-interface.md`
  → Constraint: This scope does not approve backend selection or policy-install changes owned by those specs.
- [ ] `ai/rules/planning.md`, `ai/rules/spec-no-code.md`, `ai/rules/writing.md`, and `plan/TEMPLATE.md`
  → Constraint: Research and design require separate approval before this skeleton becomes an implementation plan.

## Current Behavior

Source inspection on 2026-09-11 established the following. No VPP runtime proof
was performed for this scope capture.

| Producer | Verified behavior |
|----------|-------------------|
| `internal/component/ike/dataplane/vpp.go`, `vppBackend.ListSAs` | Returns a nil slice and an error wrapping `ErrNotSupported`. It does not enumerate the SAD. |
| `internal/component/ike/dataplane/vpp_policy.go`, `vppBackend.ListPolicies` | Returns a nil slice and an error wrapping `ErrNotSupported`. It does not enumerate SPD entries. |
| `vpp.go`, `firstFreeSadID` | Already sends generated `IpsecSaV3Dump` requests and reads `IpsecSaV3Details`. It uses only the SAD IDs to allocate an install ID. |
| `vpp_policy.go`, `spdIDs` | Already sends generated `IpsecSpdsDump` requests and reads SPD IDs from `IpsecSpdsDetails`. This enumerates databases, without producing `PolicyInfo` records. |
| `vpp.go`, `vppBackend`, `allocSadID` | Tracks local SA identity-to-SAD-ID assignments. A VPP SAD ID is distinct from an SPI. |
| `vpp_policy.go`, `InstallPolicy`, `ensureSPD` | Records installed entries and creates an SPD with interface bindings. These local records do not enumerate foreign or externally changed state. |
| `internal/component/ike/dataplane/dataplane.go`, `SAInfo`, `PolicyInfo`, `Dataplane` | Public records contain no key bytes. `ListSAs(0)` means all interface IDs, and `ListPolicies` promises every policy the dataplane holds. |

The backend imports generated `go.fd.io/govpp/binapi/ipsec` types under the
`ze_vpp` build tag. Generated declarations establish binding availability only,
without proving compatibility or semantics on the VPP version used for deployment.

### Public Consumers

| Producer | Verified read path and behavior |
|----------|---------------------------------|
| `internal/component/ike/cmd/show_dataplane.go`, `init`, `handleShowVPNIPsecDataplaneSA`, `handleShowVPNIPsecDataplanePolicy` | Registers the existing RPC handlers. They call `ListSAs(0)` or `ListPolicies` and return command errors on failed reads. |
| `show_dataplane.go`, `saInfoToMap`, `policyInfoToMap` | Maps records to public fields, including scalar counters and limits. A zero-valued field reaches the result as a value. |
| `internal/component/ike/engine/health_drift.go`, `driftSAD`, `ObserveDataplane` | Reads `ListSAs(0)` and rejects observations that overlap tracked dataplane changes. |
| `internal/component/ike/cmd/show_ipsec.go`, `readSADCounters`, `addChildCounters` | Joins counters by `SAIdentity`. Failed observations preserve engine information and emit null counters with `counters-known: false`. |
| `internal/component/ike/engine/metrics.go`, `publishDataplaneGauges` | Uses the shared observation and clears both dataplane gauge families when it fails. SA counts currently use the XFRM `if_id` label. |

## Requested Outcome

These requirements preserve the approved scope. They are inputs to the design
and its acceptance criteria, without claiming an approved design or test plan.

| ID | Condition | Required outcome |
|----|-----------|------------------|
| O-1 | VPP holds SAs and policies | The list methods enumerate VPP state, including entries Ze did not install. A successful empty result means the completed read found no matching entries. |
| O-2 | An operator uses `show vpn ipsec dataplane sa`, its `spi` selector, or `show vpn ipsec dataplane policy` | Existing public commands and structured output expose the VPP records through the shared handlers. Backend-only helper success is insufficient. |
| O-3 | Drift, health, SA counters, and dataplane metrics consume VPP readback | Each reports the measured state or an explicit unknown/error state. Missing records, failed observations, and measured zero remain distinct. |
| O-4 | A VPP reply contains key material | Only algorithm metadata and key lengths can enter public records. Key bytes must not reach command output, JSON, logs, error text, or diagnostic artifacts. |
| O-5 | VPP cannot supply a field or requested filter | Report unsupported or unknown semantics explicitly. Never invent zero counters, unlimited lifetimes, interface IDs, ownership, or timestamps. |
| O-6 | The VPP connection, dump, or required counter read fails | Preserve the failure. Never publish partial enumeration as a complete successful table or retain stale clean metrics. |
| O-7 | Foreign policies contain ranges or dispositions outside Ze's install subset | Preserve their meaning or return a truthful unsupported result. Never widen selectors, substitute an action, or silently omit entries. |
| O-8 | SAs change during rekey or external removal | Preserve the shared observation contract and identity-qualified joins. A policy referencing an absent SA remains visible independently of the SAD. |

XFRM output semantics and installation behavior must remain unchanged. The scope
adds no new command namespace and does not replace the IKE engine's belief view.

## Research Questions

| ID | Question | Evidence and research needed |
|----|----------|------------------------------|
| Q-1 | How can VPP enumeration distinguish unavailable fields from measured values throughout the public records and consumers? | `SAInfo` has scalar counters and limits without per-field availability. `IpsecSaV3Details` carries `StatIndex`, without byte or packet totals. Establish the deployed VPP statistics source and its identity/lifetime guarantees, then choose an explicit availability contract. |
| Q-2 | Which generated dump messages are supported by the deployed VPP, and which request values enumerate every entry? | Verify message versions, CRCs, dump completion, and filter semantics against VPP source and a running instance. Existing allocation helpers prove a reuse point, without proving full inspection. |
| Q-3 | How do all SPD entries and their interface bindings map into the shared policy record? | `IpsecSpdDump` takes `SpdID` and `SaID`. `IpsecSpdInterfaceDetails` carries `SpdIndex`. Establish ID-versus-index semantics and enumerate every SPD, including foreign and unbound databases. |
| Q-4 | How can arbitrary VPP address and port ranges be represented without changing their meaning? | `IpsecSpdEntry` uses inclusive ranges. `PolicyInfo` uses prefixes and masked ports. Determine exact representability and the required public result for other ranges. |
| Q-5 | What does each VPP SAD field mean in the shared vocabulary? | Determine algorithm names and key lengths, mode and direction, replay state versus window size, timestamp availability, and lifetime semantics. A same-named generated field is insufficient evidence. |
| Q-6 | How do VPP identities and bindings participate in filters, drift, ownership, and metrics? | Establish the meaning of nonzero `ListSAs(ifID)` and the existing `if_id` metric label. Never reuse `SadID` or `sw_if_index` as an XFRM interface ID. Prove counter joins survive ID reuse and restart. |
| Q-7 | How does enumeration behave under concurrent installs, deletes, disconnects, and incomplete multipart replies? | Study the shared channel and locking contract. Define a bounded read and failure semantics without publishing an inconsistent snapshot as clean. |
| Q-8 | What dependency order permits a public end-to-end proof? | Reconcile this readback scope with the selector and policy-interface specs. Identify the deployment fixture and independently observed VPP state used as the oracle. |

### Binding Evidence

| Source | Symbols and established facts |
|--------|-------------------------------|
| `vendor/go.fd.io/govpp/binapi/ipsec/ipsec.ba.go` | `IpsecSaV3Details` contains `Entry`, sequence fields, `ReplayWindow`, `SwIfIndex`, and `StatIndex`. `IpsecSpdDump`, `IpsecSpdDetails`, and the SPD-interface dump types exist. Their runtime behavior remains unverified here. |
| `vendor/go.fd.io/govpp/binapi/ipsec_types/ipsec_types.ba.go` | `IpsecSadEntryV3` contains encryption and integrity keys. `IpsecSpdEntry` contains local/remote address and port range endpoints. |

## Risks & Assumptions

| ID | Risk | Early signal | Required response |
|----|------|--------------|-------------------|
| R-1 | Successful enumeration turns absent counters into fabricated zero values | The dump succeeds without an independently verified counter source | Resolve Q-1 before design approval. Preserve unknown values through every consumer. |
| R-2 | A partial SPD read looks like the whole dataplane | Foreign or unbound entries disappear from the result | Compare against independently installed entries across multiple SPDs. |
| R-3 | Decoded VPP keys escape through generic serialization or error output | A generated SAD reply reaches a public mapper or logger | Restrict public mapping to named metadata fields and establish safe reply handling. |
| R-4 | An XFRM-shaped identity assigns counters or ownership to another VPP object | Same-SPI entries or reused VPP IDs collide | Define the complete VPP identity and prove joins against collisions and replacement. |

No runtime API compatibility, statistics availability, or approved representation
is assumed. Q-1 through Q-8 remain research obligations.

## Remaining Spec Phases

| Phase | Required artifact before approval |
|-------|-----------------------------------|
| RESEARCH | Verified VPP API and statistics semantics, complete data flow, dependency order, and every integration/documentation checklist row from `plan/TEMPLATE.md` |
| DESIGN | Alternatives, field availability and identity contracts, failure analysis, testable ACs, named wiring tests, and files to change |
| WRITE | Approved implementation steps and proof plan, including public commands against a populated VPP and unchanged XFRM behavior |

The eventual documentation inventory includes the two architecture pages above,
`docs/guide/ipsec.md`, and the CLI/metrics documentation affected by the chosen
availability contract. Runtime dependency and doctor requirements remain part of
research. No validator, formatter, linter, build, or test ran for this capture.
