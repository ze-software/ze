# Spec: bgp-tunnel-encapsulation-consumption

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | scope recorded; separate design approval required |
| Handoff | - |
| Updated | 2026-10-08 |

## Task

Own the absent RFC 9012 attribute-23-driven tunnel selection and encapsulation
consumer. Thomas selected **Separate owning spec** on 2026-10-08: record this
capability here, retain explicit blockers in the BGP acceptance work, and do not
implement it in that acceptance pass. This is an ownership record, not an
approved implementation design or a claim that the capability exists.

The release bucket is `plan/`: this is an absent feature, not permission to move
implemented receive defects out of the BGP repair. Existing configured tunnels,
labeled-NLRI forwarding and attribute-40 SRv6 processing are not this consumer.
No conformance credit follows from creating this spec.

## Work Inherited From BGP Acceptance

| Requirement or boundary | Ownership | Acceptance effect |
|-------------------------|-----------|-------------------|
| RFC9012-13-16 | Meaningless sub-TLVs must not influence attribute-driven tunnel consumption | Explicit unimplemented consumer gap, not weak opaque-carry proof; the BGP child retains the permanent audit identity for unimplemented rejudgment, and this spec owns real consumer proof |
| RFC9012-13-17 | Encapsulation-header construction must be unaffected by those meaningless sub-TLVs | Preserve the explicit implementation gap; opaque propagation is not header proof |
| RFC9012-13-21 | First-occurrence interpretation of duplicate non-endpoint single-instance sub-TLVs | Own the absent consumer rule restored from extraction site 13:7; historical 13-7 remains retired, and the receive-side endpoint exception stays in the BGP child |
| RFC9012-4.2-1 | Router's MAC Extended Community precedence when a consumed tunnel carries conflicting MAC information | Own the absent consumption boundary; preserve the full conflict context and correct the old assertion that opaque values cannot disagree |
| RFC9012 Section 3.7 consumer behavior | Tunnel Prefix-SID/SRGB/Label-Index application when forwarding packets | Own the absent attribute-23 consumer, not existing receive disregard or unrelated attribute-40 behavior |
| RFC9012 Section 15 tunnel traffic filtering | Traffic filtering by every participating router when Prefix-SID tunnels require it | Design must address actual packet filtering, not substitute filtering attribute 23 |

The broader tunnel-type and forwarding obligations must be walked against the
RFC during this spec's design. This table transfers the named ownership; it is
not a complete conformance inventory or an exclusion of other applicable rows.

## Scope Boundaries

| Boundary | Decision |
|----------|----------|
| Existing BGP receive processing | Remains in `plan/pre-release/spec-rfc-verdict-fix-bgp.md`, including RFC9012-13-10/12 transit-processing proof and 13-13/14 special-purpose endpoint rejection |
| Structural framing and endpoint count | Remain in the BGP child; this spec does not excuse malformed received updates |
| RFC 9830 SR Policy defects | Remain in `plan/immediate/spec-bgp-sr-policy-rfc-defects.md`; its AC-4/5 do not own general RFC 9012 header construction |
| Optional origin-AS ownership validation | No implementation authorization was given by the consumer ownership decision |
| Runtime configuration, supported tunnel types and backend | Require a separate design decision; no new command, tunnel mode or backend is selected here |
| Requirement accounting | Retain permanent identities, obligation levels and applicable gaps; no automatic enforced or not-applicable verdict |

## Required Reading

- [ ] `rfc/full/rfc9012.txt` Sections 3, 4, 6-9, 13 and 15.
  → Constraint: Section 13 says, "Sub-TLVs of this sort MUST be disregarded."
  The next sentence defines the consumer obligation: "That is, they MUST NOT
  affect the creation of the encapsulation header."
- [ ] `rfc/short/rfc9012.md` and its extraction, audit and discrimination records.
  → Constraint: the summary is an inventory to verify against the full RFC,
  not authority for a new support claim.
- [ ] `docs/architecture/wire/attributes.md`.
  → Constraint: existing receive validation and opaque propagation must remain
  separate from selection and header-construction claims.
- [ ] `docs/architecture/plugin/rib-storage-design.md` and
  `docs/architecture/route-selection.md`.
  → Constraint: the forwarding consumer must follow the existing ownership and
  registration boundaries rather than introduce a second route inventory.
- [ ] `docs/architecture/testing/interop.md`.
  → Constraint: prove the resulting packets against another implementation and
  demonstrate a meaningful failure with the implementing behavior removed.

## Current Behavior

**Source read while recording ownership:**

| Producer | Observed boundary |
|----------|-------------------|
| `internal/component/bgp/reactor/session_tunnel_encap.go::applyTunnelEncap` and `tunnelTLVLayout` | Validate received framing/carriers and remove invalid endpoint TLVs; this is not tunnel forwarding |
| `internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange` | Publishes next hop, metrics, labeled-NLRI labels and SRv6 SID into the Loc-RIB; does not construct a header from attribute-23 sub-TLVs |
| `internal/component/bgp/plugins/rib/rib_bestchange.go::entrySRv6SID` | Reads the attribute-40 SRv6 source, not the RFC 9012 Tunnel Encapsulation consumer |

Preserve raw unknown-sub-TLV propagation where the RFC requires it, source and
route identity, withdrawal behavior, and existing configured forwarding paths.
No source changes are authorized by this ownership record.

## Data Flow

| Stage | Current or required boundary |
|-------|------------------------------|
| Entry | A real BGP peer supplies a route with Tunnel Encapsulation attribute 23 |
| Existing receive path | Session validation, retained route attributes and propagation |
| Absent consumer | Determine applicable tunnel semantics, feasibility and selection for the route |
| Required forwarding boundary | Deliver the selected, validated encapsulation parameters to the owning forwarding backend |
| Observable result | Packets use the selected tunnel and contain the required header fields; ignored sub-TLVs do not change those fields |

## Acceptance Criteria For The Separate Design

| AC ID | Condition | Required evidence |
|-------|-----------|-------------------|
| AC-1 | A supported attribute-driven tunnel is selected from a received route | A real operator/BGP entry point reaches forwarding, with explicit installed-state and packet-header assertions |
| AC-2 | RFC9012-13-16 and RFC9012-13-17: a meaningless sub-TLV is added to an otherwise usable tunnel | Header construction is unchanged, required opaque propagation remains intact, and removing the implementing guard makes the test fail |
| AC-3 | RFC9012-4.2-1: conflicting MAC sources are applicable to a supported tunnel | The exact RFC 9012 precedence reaches the resulting forwarding state and packet |
| AC-4 | Prefix-SID tunnel behavior is included in the approved design | Label/SRGB behavior and all-participating-router traffic filtering are proven at the owned forwarding boundary |
| AC-5 | A supported route is replaced or withdrawn, or its owner disappears | No stale tunnel state remains and unrelated routes/tunnels survive |
| AC-6 | This spec claims a transferred requirement is enforced | Native discrimination records, real-entry-point and interoperability evidence, and an independent whole-row judgment exist before the BGP blocker is removed |
| AC-7 | A supported tunnel carries a duplicate non-endpoint single-instance sub-TLV | Only its first occurrence influences consumption; required propagation remains intact, and the endpoint exception is not replaced by this generic rule |

## Risks And Design Gates

| Risk or unresolved choice | Required resolution |
|---------------------------|---------------------|
| Treating configured tunnels or attribute-40 forwarding as attribute-23 support | Trace the actual attribute-23 consumer and assert its output |
| Choosing tunnel types or configuration without owner agreement | Complete the separate research/design gates before implementation |
| Keeping an obsolete route or tunnel after source replacement | Specify ownership, replacement and withdrawal transitions before code |
| Hiding implemented receive defects behind this future consumer | Keep the BGP-child obligations named above in their existing scope |
| Counting ignored opaque bytes as header-construction proof | Require installed-state and packet-level observations plus a meaningful negative control |

## Review Gate

The implementation review has not started. Before this spec becomes `ready`,
complete the design-time integration, documentation, discovery and wiring-test
tables from `plan/TEMPLATE.md` for the owner-selected tunnel types and backend.
Do not treat this recorded scope as authorization to implement those choices.
