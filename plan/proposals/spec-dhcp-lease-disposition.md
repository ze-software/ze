# Spec: dhcp-lease-disposition

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | First-release disposition of long/infinite DHCP leases remains unapproved; a proposal preserves the decision and requirement without declaring a defect or release prerequisite. |
| Depends | - |
| Phase | - |
| Updated | 2026-10-22 |

## Task

Own the explicitly unresolved lease disposition and full long/infinite lease
requirement split from `plan/post-release/features/spec-dhcp-pool-options.md`.
Arbitrary DHCPv4 options remain in that feature spec and do not depend on this
decision. This proposal is not an approval to implement or a decision that the
lease work can wait until after release.

The latest original amendment says `parseSubnet` and the YANG leaf restrict
`lease-time` to `60..604800`, while `buildReply` emits option 51 from that value.
RFC 2132 Section 9.2 gives an unsigned 32-bit duration and RFC 2131 Section 3.3
reserves `0xffffffff` for infinity. Wire width alone does not establish that a
server must offer every duration. The earlier August finding called the cap a
defect; that conclusion is not adopted by the amendment or by this split.

Record the explicit owner disposition before assigning implementation:

| Possible disposition | Required ownership outcome |
|----------------------|----------------------------|
| Treat the cap as a supported-configuration defect | Name a release-fix owner carrying AC-7/8, arithmetic/lifetime implementation and all wire/expiry proof |
| Retain the cap as a documented product limit and schedule long/infinite leases as expansion | Record the supported limit and its documentation owner; name a post-release feature owner carrying the full AC-7/8 requirement and proof |

Until that decision names the destination, this proposal remains the owner of
all lease obligations. Placement under proposals and P3 are not the decision,
not a readiness claim and not a waiver. The range cannot be widened merely by
editing a validator.

## Reciprocal ownership

| Original obligation | Owner |
|---------------------|-------|
| Lease disposition; AC-7/8; long/infinite lease paragraph and finite arithmetic/infinite lifetime amendment; user story 3; lease boundary and functional/controlled-clock proof; lease-specific RESEARCH/DESIGN and files | This spec |
| AC-1..AC-6, arbitrary options and option 26 consideration, option boundaries/tests/user stories 1/2, remaining option design and DHCPv6 option-shape coordination | `plan/post-release/features/spec-dhcp-pool-options.md` |

The feature sibling retains the complete original body as design history;
its lease rows no longer own or pre-approve the work. Both specs remain
`skeleton`. No runtime reproduction was performed for the original planning
correction or this ownership split.

## Required reading and preserved constraints

- Read the sibling's complete Task amendment before design, then the existing
  `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang`, `config.go`,
  `handler.go` and `lease.go` lease paths. No fresh protocol audit is claimed.
- `buildReply` computes T2 as `LeaseTimeSec * 7 / 8` in `uint32`; widening
  validators alone risks overflow. Preserve finite-duration arithmetic across
  the widened range if approved.
- `leaseTable.add` currently schedules expiry for every value. Infinity must
  have no finite expiry while preserving explicit release and replacement.
- Prove the current rejection and intended wire/lifetime behavior before
  changing the range. Keep the existing lower-bound decision until design;
  do not silently broaden it along with the upper bound.
- Preserve fixed options and finite leases within today's supported range.
  DHCPv6 options are outside this requirement, as in the source.

## Retained acceptance criteria

These are the original capability criteria, not approved release prerequisites.
They remain owned here pending disposition and must move intact to the named
implementation owner; choosing a documented limit does not erase them.

| ID | Input / Condition | Expected behavior |
|----|-------------------|-------------------|
| AC-7 | A configured finite lease longer than 604800 seconds, including values near the uint32 finite limit | Config accepts the duration, OFFER/ACK option 51 carries it exactly, T1/T2 do not overflow and remain consistent with the lease, and the binding expires at the configured finite duration |
| AC-8 | A configured lease of `0xffffffff` | Config accepts infinity, OFFER/ACK represents the infinite lease, and the server does not expire the binding on a finite timer; explicit release and replacement still work |

## Retained implementation and proof

| Surface | Inherited obligation |
|---------|----------------------|
| YANG/config validation | Lease range and infinity representation in `ze-dhcp-server-conf.yang` and `config.go`; preserve today's valid finite values and lower-bound decision |
| Reply builder | Option 51 and consistent non-overflowing T1/T2 in `handler.go` |
| Lease lifetime | Explicit infinity and finite expiry in `lease.go` |
| Boundary proof | 604801 first value beyond the old cap; finite values through 4294967294; 4294967295 infinity; reject 4294967296 |
| User story 3 | Config to option 51/renewal/rebinding values to client duration and correct binding retention/expiry |
| Functional proof | Planned `test/plugin/dhcp-long-and-infinite-leases.ci`, including old-cap rejection reproduction and wide T2 arithmetic |
| Controlled-clock proof | Finite expiry, no finite expiry for infinity, explicit release and replacement |

## Decision completion

The decision is complete only when its rationale, supported limit and any
associated documentation obligation are recorded, and a named spec owns each
remaining implementation/proof criterion with reciprocal references. A chosen
future owner must preserve the criteria rather than claim they were delivered.
No product code, tests, RFC artifacts or support claims change in this planning
reorganization.
