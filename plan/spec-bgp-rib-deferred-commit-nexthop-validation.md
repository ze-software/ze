# Spec: rib CommitService refuses an announce with no next hop

| Field | Value |
|-------|-------|
| Status | verification |
| Scope | protocol |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | `plan/deferrals/fixit-commit-rail-nexthop-unvalidated.md` |
| Handoff | verify |
| Updated | 2026-08-23 |

<!-- Handoff `verify`: the implementation session commits its work, sets Status to
     `verification`, and stops. A later Opus 5 session reviews that commit and closes
     the spec. The implementation session does NOT append plan/TEMPLATE-CLOSURE.md. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The rib `CommitService` builds an MP_REACH_NLRI attribute without asking whether the
next hop can be encoded. `(*CommitService).buildMPReachNLRI`
(`internal/component/bgp/rib/commit.go`) returns the attribute with no check and no
error return. `(*CommitService).packAttributesWithASPath` writes it with
`attribute.WriteAttrTo`, which calls `WriteTo` and never `CheckedWriteTo`, so
`(*MPReachNLRI).ValidateNextHops` (`internal/core/bgp/attribute/mpnlri.go`) is never
reached on this rail.

The symptom is a malformed UPDATE. A zero `netip.Addr` has no wire form, so
`nextHopOctets` counts zero octets, `Len` and `WriteTo` agree at zero, the
size-versus-write invariant at the end of `packAttributesWithASPath` passes, and the
peer receives a Length of Next Hop Network Address octet of `0x00`. RFC 4760 Section 3
defines no such length, so the peer treats the UPDATE as malformed and the session is
reset (RFC 7606 Section 7.11).

The VPN branch fails differently and is equally wrong. `buildVPNMPReachNLRI` sizes the
next-hop field from `Is4()` while filling it from `AsSlice()`, so a zero `netip.Addr`
produces a 24-octet field of zeros: length `0x18`, an 8-octet Route Distinguisher of
zeros, and a 16-octet unspecified address. That is a syntactically well-formed field
naming no next router, which is the failure mode a peer cannot even diagnose.

The goal is one guard that fails closed on both branches, built from the validation
that already exists, with the refusal named by the error the sibling announce rails
already return.

## Required Reading

<!-- NEVER tick [ ] to [x]. -->

### Architecture Docs
- [ ] `docs/architecture/update-building.md` - how an UPDATE is assembled before it
  reaches a peer's write path
  → Constraint: the attribute block is sized and then written; a size query and a
    write must never come to different answers about the same attribute
  → Decision: the rib rail sizes with `attrSize`/`attrSizeWithContext` and writes with
    `attribute.WriteAttrTo`, so the checked write path is not available to it without
    a signature change to the whole packing function
- [ ] `docs/guide/route-injection.md` - the user-facing page whose source anchor names
  `internal/component/bgp/rib/commit.go` for extended next-hop encoding
  → Constraint: the page claims an IPv6 next hop injected into the RIB is re-emitted
    as an RFC 5549 / RFC 8950 extended next hop. A valid next hop must encode exactly
    as it does today, or that claim goes stale

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4760.md` - MP_REACH_NLRI wire format and requirement RFC4760-3-2
  → Constraint: RFC4760-3-2 (MUST) says the encoding of the Next Hop must provide a
    way to determine its network-layer protocol. A length octet of `0x00` names no
    protocol, and neither does 24 octets of zeros
- [ ] `rfc/short/rfc7606.md` - revised error handling
  → Constraint: Section 7.11 makes a malformed MP_REACH_NLRI a session reset, not a
    treat-as-withdraw. The cost of emitting one is the whole session
- [ ] `rfc/short/rfc4364.md` - VPN next hop carries an 8-octet Route Distinguisher
  → Constraint: Section 4.3.4 requires RD(8) + address, so 12 octets for IPv4 and 24
    for IPv6. The RD is a prefix on a real address, never a substitute for one

**Key insights:** (minimal context to resume after compaction)
- `ValidateNextHops` already exists, already returns `attribute.ErrUnencodableNextHop`,
  and is already the refusal used by the two reactor announce rails
  (`reactor/reactor_api_batch.go`, `reactor/peer_rib_routes.go`). Nothing new is owed
  except the call and its propagation.
- `(*MPReachNLRI).WriteTo` and `nextHopOctets` in `internal/core/bgp/attribute/mpnlri.go`
  ALREADY write and count the RFC 4364 Route Distinguisher for SAFI 128. The private
  `vpnMPReachNLRI` type in `commit.go` is a second encoder of a wire form the core now
  encodes, and its comment ("MPReachNLRI.WriteTo() doesn't handle RD prefix") is false.
- The doc comment on `ValidateNextHops` (`mpnlri.go`, the paragraph beginning "One
  caller does NOT ask") describes this defect as live. It becomes false the moment the
  guard lands and must be rewritten in the same change (`ai/rules/stale-comments.md`).

## Current Behavior (MANDATORY)

**Source files read:** (read before this spec was written)
- [ ] `internal/component/bgp/rib/commit.go` - `Commit` sends routes to one peer.
  `buildGroupedUpdateTwoLevel` and `buildSingleUpdate` both call
  `packAttributesWithASPath`, which chooses `attribute.NextHop` when
  `useTraditionalNLRI` holds and `buildMPReachNLRI` otherwise.
  `buildMPReachNLRI` returns `attribute.NewMPReachNLRI(...)` for a standard family and
  `buildVPNMPReachNLRI` for SAFI 128. Neither validates. `packAttributesWithASPath`
  writes every attribute with `attribute.WriteAttrTo` and refuses only when the total
  written differs from the total predicted.
- [ ] `internal/core/bgp/attribute/mpnlri.go` - `nextHopOctets` counts
  `len(nh.AsSlice())` and adds `RDSize` for SAFI 128, so it returns 0 for the zero
  `netip.Addr`. `WriteTo` skips an address with no wire form and skips its RD with it,
  so size and write agree at 0. `ValidateNextHops` refuses any next hop failing
  `IsValid`, returning `ErrUnencodableNextHop` with the AFI and SAFI. `CheckedWriteTo`
  calls it; `WriteTo` does not.
- [ ] `internal/core/bgp/attribute/attribute.go` - `WriteAttrTo` writes the header and
  calls `attr.WriteTo`. It has no error return and performs no validation, which is
  why the rib rail cannot inherit the check from the write.
- [ ] `internal/component/bgp/rib/grouping.go` - `GroupByAttributesTwoLevel` stores
  `route.NextHop().AsSlice()` in `AttributeGroup.NextHop`, which is nil for the zero
  `netip.Addr`. `bytesToAddr` maps that nil back to the zero `netip.Addr`, so the
  invalid next hop survives grouping unchanged.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `SendRoutes` is the only
  non-test caller of `rib.NewCommitService`. It builds one `CommitService` per matching
  peer and calls `Commit` only when `len(routes) > 0`. An error from `Commit` makes it
  `continue` to the next peer.
- [ ] `internal/component/bgp/transaction/commit_manager.go` - `(*Transaction).QueueAnnounce`
  is the only writer of `Transaction.announces`, and it has no non-test caller.
  `Routes()` therefore returns an empty slice in production.
- [ ] `internal/component/bgp/plugins/cmd/commit/commit.go` - `handleNamedCommitEnd`
  reads `tx.Routes()` and returns "commit empty, nothing sent" when it is empty. It is
  the reachable user entry point for this rail, and today it cannot carry an announce.
- [ ] `internal/component/bgp/reactor/peer_rib_routes.go` - the sibling rail that DOES
  ask: it builds the same `MPReachNLRI`, calls `ValidateNextHops`, logs, and drops the
  route. Precedent for the shape of the refusal.

**Behavior to preserve:**
- Every valid announce encodes byte-for-byte as it does today, on every family. The
  wire bytes of a VPNv4 announce keep the RFC 4364 Section 4.3.4 form: length octet 12,
  eight zero octets of Route Distinguisher, then the four IPv4 octets.
- `Commit` keeps its signature, its `CommitServiceStats`, its `ErrNilContext` behavior,
  its two-level grouping, its paths-limit enforcement, and its EOR emission.
- `useTraditionalNLRI` keeps gating the legacy NEXT_HOP attribute on `nextHop.Is4()`,
  so an IPv4-unicast route with an unset next hop keeps landing in MP_REACH_NLRI.
- The existing tests in `internal/component/bgp/rib` stay green with no assertion
  weakened, `TestCommitService_VPNNextHopHasRD` and
  `TestCommitService_IPv4WithIPv6NextHop` (`commit_edge_test.go`) among them.
- No exported symbol of `internal/component/bgp/rib` changes signature, so no caller
  outside the package is touched.

**Behavior to change:**
- An announce whose next hop has no wire form is refused before any UPDATE carrying it
  is handed to the sender. `Commit` returns an error that wraps
  `attribute.ErrUnencodableNextHop`, on the standard branch and on the VPN branch.
- The private `vpnMPReachNLRI` type and `buildVPNMPReachNLRI` are deleted. The core
  `MPReachNLRI` encoder already writes and counts the RD for SAFI 128, so the second
  encoder is duplicated wire knowledge (`ai/rules/no-layering.md`).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `(*CommitService).Commit` in `internal/component/bgp/rib/commit.go`, called by
  `(*reactorAPIAdapter).SendRoutes` in `internal/component/bgp/reactor/reactor_api_batch.go`.
- Input at entry: a `[]*rib.Route`, each carrying an NLRI, a `netip.Addr` next hop, and
  path attributes. The next hop is invalid when the route was built without one.
- The user-facing entry above it is the `commit <name> end` CLI command
  (`internal/component/bgp/plugins/cmd/commit/commit.go`), which today can never deliver
  a non-empty route list. See Blast Radius.

### Transformation Path
1. `Commit` enforces the per-prefix paths limit, then branches on `groupUpdates`.
2. Grouped: `GroupByAttributesTwoLevel` (`grouping.go`) keys routes by family, next hop
   bytes and attributes; `buildGroupedUpdateTwoLevel` turns `AttributeGroup.NextHop`
   back into a `netip.Addr` with `bytesToAddr`. Ungrouped: `buildSingleUpdate` takes
   `route.NextHop()` directly.
3. `packAttributesWithASPath` picks the next-hop carrier: `attribute.NextHop` when
   `useTraditionalNLRI` holds, `buildMPReachNLRI` otherwise.
4. `buildMPReachNLRI` builds the attribute. This is where the guard goes.
5. `packAttributesWithASPath` sizes every attribute with `attrSize`, allocates, writes
   with `attribute.WriteAttrTo`, and refuses on a size-versus-write mismatch.
6. `Commit` hands the `*message.Update` to `UpdateSender.SendUpdate`, which is the peer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| rib → attribute (core) | `attribute.NewMPReachNLRI`, `ValidateNextHops`, `WriteAttrTo` | Yes: read `mpnlri.go` and `attribute.go` |
| reactor → rib | `rib.NewCommitService` and `Commit`, over the `UpdateSender` interface | Yes: `reactor_api_batch.go` SendRoutes is the only non-test construction |
| plugin → reactor | `bgptypes.BGPReactor.SendRoutes` from the `commit` command plugin | Yes: `internal/component/bgp/types/reactor.go` declares it, `plugins/cmd/commit/commit.go` calls it |
| rib → wire | none added: the guard runs before any byte is written | Yes: the check precedes the allocation in `packAttributesWithASPath` |

### Integration Points
- `(*MPReachNLRI).ValidateNextHops` (`internal/core/bgp/attribute/mpnlri.go`) - the one
  validation, reused, not re-implemented.
- `attribute.ErrUnencodableNextHop` (same file) - the sentinel a test asserts with
  `errors.Is`, already used by both reactor announce rails.
- `(*CommitService).packAttributesWithASPath` - already returns an error, so the
  propagation needs no new plumbing above `buildMPReachNLRI`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the guard sits in the rib rail's own attribute builder, using the core validator the other two rails call |
| No unintended coupling (components stay isolated) | Yes | rib already imports `internal/core/bgp/attribute`; no new import, no new dependency direction |
| No duplicated functionality (extends existing, does not recreate) | Yes | the validation is `ValidateNextHops`, unchanged; the change DELETES the duplicate VPN encoder rather than adding one |
| Zero-copy preserved where applicable (refs, not copies) | Yes | validation reads `NextHops.Slice()` and allocates nothing; deleting `vpnMPReachNLRI` removes one intermediate `value` allocation per VPN UPDATE |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | no command, view, family or handler is added. The change removes one switch on SAFI (`isVPNSAFI`) from the rib rail |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `(*MPReachNLRI).WriteTo` with SAFI 128 emits bytes identical to the deleted `vpnMPReachNLRI` for every next hop the rib rail can pass | `mpnlri.go` `nextHopOctets` adds `RDSize` for `SAFIVPN` and `WriteTo` writes 8 zero octets before each address; `commit.go` `buildVPNMPReachNLRI` writes the same fields in the same order | the VPN wire form changes and a peer rejects the UPDATE | `TestCommitVPNAnnounceCarriesTheRFC4364NextHop` pins the whole attribute value byte by byte, and `TestCommitService_VPNNextHopHasRD` stays green | confirmed 2026-08-23: the byte test passed against the UNCHANGED code in phase 1 and passes against the core encoder after the deletion, with no assertion edited; `TestCommitService_VPNNextHopHasRD` never went red |
| A-2 | `(*Transaction).QueueAnnounce` has no non-test caller, so no production path reaches `Commit` with routes today | grep over the tree: the only callers are `transaction/commit_manager_test.go`. `Peer.QueueAnnounce` and `OutgoingRIB.QueueAnnounce` are different methods on different types | the rail is live and the defect is shipping, which raises priority but changes no line of this spec | grep for `QueueAnnounce` at implementation time; record the result in the Implementation Summary | confirmed 2026-08-23: `gopls references` on `(*Transaction).QueueAnnounce` returns nine references, every one in `transaction/commit_manager_test.go`. The `peer.QueueAnnounce` call inside `(*reactorAPIAdapter).SendRoutes` is `(*Peer).QueueAnnounce` (`reactor/peer.go`), a different method on a different type |
| A-3 | `attribute.ErrUnencodableNextHop` survives the wrapping `Commit` applies (`build update: %w`), so `errors.Is` holds at the `Commit` boundary | `commit.go` wraps with `%w` in both branches | the test asserts on a string instead of a sentinel, which is weaker | the new tests assert with `errors.Is` from `Commit`, not from the helper | confirmed 2026-08-23: `assert.ErrorIs` holds on all four rows of `TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm`, each driving `(*CommitService).Commit` |
| A-4 | No caller outside `internal/component/bgp/rib` names `vpnMPReachNLRI` or `buildVPNMPReachNLRI` | both are unexported and grep finds references only inside `commit.go` | the deletion breaks a build | `make ze-unit-pkg-test PKG=./internal/component/bgp/rib` plus `make ze-lint-changed` | confirmed 2026-08-23: after the deletion `grep -rn "vpnMPReachNLRI\|buildVPNMPReachNLRI\|isVPNSAFI" internal/` returns nothing, the package is green, and the tree-wide lint reports no finding in any package this spec touches |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Deleting the VPN encoder changes VPN wire bytes | `TestCommitService_VPNNextHopHasRD` fails on the next-hop length or the RD octets | the byte-exact test in AC-3 is written FIRST and run against the unchanged code, so any divergence appears before the deletion |
| R-2 | The guard refuses a next hop that is actually encodable, and a valid announce stops going out | an existing wire test in `commit_wire_test.go` or `commit_edge_test.go` turns red | `ValidateNextHops` refuses only `!IsValid()`, which no parsed or configured address produces; the whole existing suite is the control |
| R-3 | The refusal is invisible in production because `SendRoutes` discards the error with `continue` | nothing in the log when a peer is skipped | the guard logs before returning, mirroring the size-mismatch branch already in `packAttributesWithASPath`. The reactor-side swallow is recorded under Known Limitations and is not fixed here |
| R-4 | Grouped mode already sent earlier UPDATEs before a later group fails, so the commit is partial | `CommitServiceStats.UpdatesSent` is non-zero on an error return | this is the pre-existing behavior of the size-mismatch refusal on the same function. The guard does not make it worse: it stops the malformed UPDATE, which is the whole obligation |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A wrong guard refuses valid announces on the `commit <name> end` path, or a wrong VPN encoder emits an MP_REACH a peer rejects with a NOTIFICATION (RFC 7606 Section 7.11). Nothing else uses `CommitService`. |
| How is it reverted? | Single commit revert. No config, no schema, no persisted state, no peer-visible change on any path reachable today. |
| Who else touches this path? | `internal/component/bgp/reactor/reactor_api_batch.go` (`SendRoutes`, the only non-test constructor) and `internal/component/bgp/plugins/cmd/commit/` (the CLI verbs above it). `plan/deferrals/fixit-attribute-length-from-family.md` and `plan/deferrals/fixit-mpreach-split-undercounts-rd.md` cover sibling size-versus-write defects in `internal/core/bgp/attribute`; neither touches `commit.go`. |
| Is the rail reachable in production today? | No, and the guard is written anyway. `(*Transaction).QueueAnnounce` has only test callers, so `tx.Routes()` is empty and `handleNamedCommitEnd` returns "commit empty, nothing sent" before `SendRoutes` is called with routes. A guard that fails closed while unreachable is exactly the guard that is correct the day the rail is wired (`ai/rules/evidence.md`). |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `(*CommitService).Commit` with grouping on and a route whose next hop is the zero `netip.Addr` | → | `buildMPReachNLRI` guard, propagated through `packAttributesWithASPath` | `TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm/grouped-ipv6` |
| `(*CommitService).Commit` with grouping off and the same route | → | same guard through `buildSingleUpdate` | `TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm/ungrouped-ipv6` |
| `(*CommitService).Commit` with a VPNv4 route (SAFI 128) whose next hop is the zero `netip.Addr` | → | same guard, VPN branch | `TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm/grouped-vpnv4` |
| `(*CommitService).Commit` with a VPNv4 route and a valid next hop | → | `buildMPReachNLRI` returning the core `MPReachNLRI` for SAFI 128 | `TestCommitVPNAnnounceCarriesTheRFC4364NextHop` |
| `(*CommitService).Commit` with an IPv4-unicast route and a valid IPv4 next hop | → | `useTraditionalNLRI` and the legacy NEXT_HOP attribute, untouched | `TestCommitService_IPv4_HasNextHop` (existing, `commit_wire_test.go`) |

<!-- No .ci row, and no new feature surface: this spec adds a guard, not a command.
     No production entry point reaches this rail with routes, so a .ci could only
     assert the empty-commit message and would pass with the guard deleted. See
     Functional Tests and Blast Radius. Every row above is a Go test in the
     package that owns the rail, driven from Commit and never from the helper. -->

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `Commit` is given an IPv6-unicast route whose next hop is the zero `netip.Addr`, with grouping on and with grouping off | `Commit` returns an error satisfying `errors.Is(err, attribute.ErrUnencodableNextHop)`, and the sender receives no UPDATE. No MP_REACH_NLRI with a next-hop length of 0 is produced. |
| AC-2 | `Commit` is given a VPNv4 route (SAFI 128) whose next hop is the zero `netip.Addr` | Same refusal and same sentinel. No MP_REACH_NLRI carrying a 24-octet all-zero next hop is produced. |
| AC-3 | `Commit` is given a VPNv4 route with next hop 10.0.0.1 | One UPDATE is sent, and its MP_REACH_NLRI value is AFI 1, SAFI 128, next-hop length 12, eight zero octets of Route Distinguisher, the four octets of 10.0.0.1, a zero Reserved octet, then the NLRI (RFC 4364 Section 4.3.4). Byte for byte what the current code emits. |
| AC-4 | `Commit` is given an IPv4-unicast route with a valid IPv4 next hop, an IPv6 route with a valid IPv6 next hop, an IPv4 route with an IPv6 next hop, and an EVPN route | Every existing assertion in `commit_test.go`, `commit_wire_test.go` and `commit_edge_test.go` holds unchanged, and every UPDATE is byte-identical to the current output. |
| AC-5 | The refused announce happens while a peer is established | The refusal is visible: one log record at Warn names the family and the invalid next hop before `Commit` returns. The malformed UPDATE never reaches `UpdateSender.SendUpdate`. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `commit <name> end` for a named commit holding a VPNv4 route with a valid next hop | commit plugin → `SendRoutes` → `cs.Commit` → `buildMPReachNLRI` → peer | `TestCommitVPNAnnounceCarriesTheRFC4364NextHop` |
| 2 | runs `commit <name> end` for a named commit holding a route with no next hop | commit plugin → `SendRoutes` → `cs.Commit` refuses, the peer is skipped, the session survives | `TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm` |

<!-- Story 1 and story 2 are not reachable end to end today: `(*Transaction).QueueAnnounce`
     has no non-test caller, so a named commit never holds a route (Blast Radius). The
     path above each row is the chain that runs the day it is wired; the tests drive the
     rail's own entry point, `(*CommitService).Commit`. -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm` | `internal/component/bgp/rib/commit_nexthop_test.go` (new) | AC-1, AC-2, AC-5. Table with four rows: grouped IPv6, ungrouped IPv6, grouped VPNv4, ungrouped VPNv4. Each drives `(*CommitService).Commit`, asserts `errors.Is(err, attribute.ErrUnencodableNextHop)`, and asserts the mock sender captured zero updates | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `TestCommitVPNAnnounceCarriesTheRFC4364NextHop` | `internal/component/bgp/rib/commit_nexthop_test.go` (new) | AC-3. Pins the complete MP_REACH_NLRI value of a VPNv4 announce field by field, so the deletion of `vpnMPReachNLRI` cannot change one octet | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `TestCommitService_VPNNextHopHasRD` | `internal/component/bgp/rib/commit_edge_test.go` (existing) | AC-3, AC-4. Must stay green with no assertion changed | |
| `TestCommitService_IPv4WithIPv6NextHop` | `internal/component/bgp/rib/commit_edge_test.go` (existing) | AC-4. RFC 5549 extended next hop is unaffected | |
| `TestCommitService_IPv6_UsesMPReachNLRI`, `TestCommitService_EVPN_UsesMPReachNLRI`, `TestCommitService_IPv4_HasNextHop` | `internal/component/bgp/rib/commit_wire_test.go` (existing) | AC-4. The three families that reach `buildMPReachNLRI` or bypass it | |

Existing helpers to reuse, all in the same package: `mockUpdateSender` and
`testContext` (`commit_test.go`), `newIPv6NLRI` (`commit_test.go`), `newVPNv4NLRI`
(`commit_edge_test.go`), `NewRoute` (`route.go`).

The refusal test carries the tag `RFC requirement: RFC4760-3-2 negative` with a
sentence naming the producing function, matching the form already used in
`internal/core/bgp/attribute/mpnlri_nexthop_wire_test.go` and
`internal/component/bgp/reactor/peer_rib_routes_nexthop_test.go`. It is the third rail
proving the same requirement, so it adds proof and removes none
(`ai/rules/rfc-compliance.md`, the coverage and evidence ratchets).

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Length of Next Hop Network Address, standard family (RFC 4760 Section 3) | 4, 16 or 32 octets | 16 (IPv6 global) | 0 (the zero `netip.Addr`, refused by AC-1) | N/A: `netip.Addr` cannot exceed 16 octets, so no over-long value is constructible |
| Length of Next Hop Network Address, SAFI 128 (RFC 4364 Section 4.3.4) | 12 or 24 octets | 12 (RD + IPv4), 24 (RD + IPv6) | 8 (RD alone, which is what the zero `netip.Addr` would mean; refused by AC-2, and today mis-encoded as 24) | N/A: same reason |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Not applicable | - | A `.ci` drives the daemon through a user-reachable command, and no such command reaches this rail with routes: `(*Transaction).QueueAnnounce` has no non-test caller, so `commit <name> end` always finds an empty route list and returns "commit empty, nothing sent" without calling `SendRoutes` with routes. A `.ci` written today would assert the empty-commit message and would pass with the guard deleted, which is a vacuous test (`ai/rules/interop-and-goal-validation.md`). The tests above drive the rail's own entry point instead | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| None owed | - | - | No wire-visible change reaches a peer. On every input a peer can receive today the bytes are unchanged (AC-3, AC-4), and the only new behavior is the refusal of an UPDATE that no production path can currently produce. An interop scenario would exercise the unchanged path and pass with the guard reverted, which is the vacuity trap `ai/rules/interop-and-goal-validation.md` names. The peer-visible value of this change is the session reset that never happens | |

## Files to Modify
- `internal/component/bgp/rib/commit.go` - `buildMPReachNLRI` gains an error return and
  validates before returning; `buildVPNMPReachNLRI` and the `vpnMPReachNLRI` type are
  deleted; `isVPNSAFI` is deleted with its last caller; `packAttributesWithASPath`
  propagates the error.
- `internal/core/bgp/attribute/mpnlri.go` - comment only. The `ValidateNextHops` doc
  comment states that `(*CommitService).buildMPReachNLRI` does not ask and names the
  deferral shard. That becomes false; rewrite the paragraph to say all three assembling
  rails ask (`ai/rules/stale-comments.md`).

## Files to Create
- `internal/component/bgp/rib/commit_nexthop_test.go` - the refusal test and the VPN <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
  byte-exact test.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config surface changes |
| YANG validation constraints | No | no leaf added |
| YANG custom validators | No | no leaf added |
| CLI commands/flags | No | `commit <name> end` keeps its verbs and its output shape |
| CLI grammar (keyword before value) | N-A | no grammar touched |
| Editor autocomplete | N-A | no leaf added |
| Functional test for new RPC/API | N-A | no RPC or API added; see the Functional Tests row for why no `.ci` is owed |
| Pipe completeness | N-A | no command output produced or changed |
| Env var registration | No | no environment leaf added |
| Doctor check for runtime dependencies | No | no file path, socket, service, module, port, procfs entry, netlink use, binary or certificate is added |
| Prometheus counters/metrics | No | the refusal is a log record, not a counter. `CommitServiceStats` keeps its fields, so no metric contract moves |
| BGP family surface (new SAFI / capability / attribute) | No | no SAFI, capability or attribute is added. SAFI 128 encoding MOVES to the core encoder that already implements it, and AC-3 pins the bytes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a refusal on an unreachable rail is not a feature a user can invoke |
| 2 | Config syntax changed? | No | no config surface touched |
| 3 | CLI command added/changed? | No | `commit` keeps its verbs, its arguments and its responses |
| 4 | API/RPC added/changed? | No | `bgptypes.BGPReactor.SendRoutes` keeps its signature |
| 5 | Plugin added/changed? | No | `plugins/cmd/commit` is not edited |
| 6 | Has a user guide page? | No | `docs/guide/route-injection.md` anchors `commit.go`, and its claim is about a VALID extended next hop, which AC-4 keeps byte-identical |
| 7 | Wire format changed? | No | AC-3 and AC-4 pin every reachable encoding unchanged |
| 8 | Plugin SDK/protocol changed? | No | no SDK type touched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | no summary edit is owed: RFC4760-3-2 already carries positive and negative tags (`rfc/enrolled.txt`), and this adds a third tagged proof on a new rail. Run `make ze-rfc-check` and confirm no counter moves; do NOT edit `rfc/short/rfc4760.md` or `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | one new test file using existing package helpers |
| 11 | Affects daemon comparison? | No | no capability claim changes |
| 12 | Internal architecture changed? | No | `docs/architecture/update-building.md` describes the rail as it stays; the deletion removes a duplicate encoder the doc never named. The two design docs the changed files' `// Design:` headers name are unaffected: `docs/architecture/pool-architecture.md` (named by `internal/component/bgp/rib/commit.go`) documents RIB wire storage, and this change allocates one buffer fewer rather than changing where a buffer comes from; `docs/architecture/wire/attributes.md` (named by `internal/core/bgp/attribute/mpnlri.go`) documents the attribute wire forms, and the edit there is a doc comment with no encoding change |
| 13 | Route metadata keys added/changed? | No | no metadata key touched |
| 14 | Prometheus counters added/changed? | No | none added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/route-injection.md` carries `<!-- source: internal/component/bgp/rib/commit.go -- extended next-hop encoding -->`. Its claim is that an injected IPv6 next hop is re-emitted as an RFC 5549 / RFC 8950 extended next hop. That path is `useTraditionalNLRI` false plus a VALID next hop, unchanged by this spec, so the page needs no edit. Re-read the paragraph and confirm before answering No in the closure |
| 17 | Existing docs show config/CLI/API examples for this area? | No | the `route-injection.md` examples use `request bgp rib inject`, which does not reach `CommitService` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the entry point reaches the defect
   - Tests: write `TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm` with all four
     rows, and `TestCommitVPNAnnounceCarriesTheRFC4364NextHop`, before touching
     `commit.go`.
   - Files: `internal/component/bgp/rib/commit_nexthop_test.go`. <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
   - Verify: run `make ze-unit-pkg-test PKG=./internal/component/bgp/rib`. The refusal test
     MUST fail, and it MUST fail for the stated reason: `Commit` returns a nil error and
     the mock sender holds one UPDATE. Record what the captured MP_REACH shows in each
     row: next-hop length `0x00` on the IPv6 rows, next-hop length `0x18` followed by 24
     zero octets on the VPNv4 rows. The VPN byte test MUST pass against the UNCHANGED
     code: it is the control that pins the wire form before the encoder is deleted. If
     it fails here, stop and re-read `buildVPNMPReachNLRI`; A-1 is broken.
2. **Phase: Guard** -- validate before returning
   - Tests: the same two tests.
   - Files: `internal/component/bgp/rib/commit.go`.
   - Work: give `buildMPReachNLRI` an error return. Build the core
     `attribute.NewMPReachNLRI` for every family, call `ValidateNextHops`, and on error
     log once at Warn naming the family and the next hop, then return the error wrapped
     with enough context to name the rail. Propagate through
     `packAttributesWithASPath`, which already returns an error, and change no other
     signature. Nothing may escape `internal/component/bgp/rib`.
   - Verify: the refusal test passes, and every existing test in the package still
     passes.
3. **Phase: Delete the duplicate encoder** -- one encoder for SAFI 128
   - Tests: `TestCommitVPNAnnounceCarriesTheRFC4364NextHop`,
     `TestCommitService_VPNNextHopHasRD`.
   - Files: `internal/component/bgp/rib/commit.go`.
   - Work: delete `buildVPNMPReachNLRI`, the `vpnMPReachNLRI` type with all its methods,
     and `isVPNSAFI` once its last caller is gone. `buildMPReachNLRI` returns the core
     attribute for every family, because `(*MPReachNLRI).nextHopOctets` and `WriteTo`
     already carry the RFC 4364 Route Distinguisher for SAFI 128.
   - Verify: both VPN tests pass with no assertion edited. If any octet moves, revert
     this phase and report; the guard in phase 2 stands on its own.
4. **Phase: Comment truth** -- remove the stale claim
   - Files: `internal/core/bgp/attribute/mpnlri.go`.
   - Work: rewrite the `ValidateNextHops` paragraph that says one assembling caller does
     not ask, and drop the deferral pointer with it. Say what is then true: all three
     assembling rails call `ValidateNextHops` before contributing the attribute.
   - Verify: `make ze-lint-changed`, then `make ze-unit-pkg-test PKG=./internal/core/bgp/attribute`.
5. **Phase: Discrimination proof (BLOCKING before any completion claim)**
   - Work: revert ONLY the phase 2 guard in the working tree, re-run
     `make ze-unit-pkg-test PKG=./internal/component/bgp/rib`, and paste the RED output into
     the spec. Restore the guard and paste the GREEN output
     (`ai/rules/interop-and-goal-validation.md`, "Prove the test discriminates"). Red
     looks like: `TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm` fails on every
     row, reporting a nil error from `Commit` and one captured UPDATE.
   - Verify: the test cannot pass without the guard. If it can, the test asserts the
     wrong thing and must be fixed before anything else.
6. **Phase: Gates and handoff**
   - Verify: `make ze-unit-pkg-test PKG=./internal/component/bgp/rib`,
     `make ze-unit-pkg-test PKG=./internal/core/bgp/attribute`, `make ze-lint-changed`,
     `make ze-rfc-check`, and `make ze-precommit-verify` per `ai/rules/git-safety.md` (or the
     scoped evidence a shared checkout allows, attributed).
   - Then: commit, set `Status` to `verification`, and STOP. Handoff is `verify`: a
     later Opus 5 session reviews the commit and closes the spec.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 each have a named test and a file:line implementation |
| Feature completeness | Both branches are guarded: read `buildMPReachNLRI` and confirm no path returns an attribute without having called `ValidateNextHops` |
| Guard fails closed | The zero value produces an ERROR, never an empty-but-valid-looking attribute. No branch returns a nil error with an unvalidated next hop, and no branch swallows the validation error inside the rib package (`ai/rules/evidence.md`) |
| Correctness | The VPN wire bytes are unchanged: AFI, SAFI, length 12, eight RD zeros, four address octets, Reserved zero, NLRI |
| Naming | The error wraps `attribute.ErrUnencodableNextHop`; no new exported sentinel is added to `rib` |
| Data flow | The guard runs before the size pass in `packAttributesWithASPath`, so no buffer is allocated for an attribute that will be refused |
| Rule: `ai/rules/no-layering.md` | `vpnMPReachNLRI` is DELETED, not left beside the core encoder |
| Rule: `ai/rules/stale-comments.md` | The `ValidateNextHops` doc comment no longer says a caller skips the check |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The guard exists on both branches | `grep -n ValidateNextHops internal/component/bgp/rib/commit.go` returns a call in `buildMPReachNLRI` |
| The duplicate encoder is gone | `grep -rn "vpnMPReachNLRI\|buildVPNMPReachNLRI\|isVPNSAFI" internal/` returns nothing |
| The refusal is visible | `grep -n "slog" internal/component/bgp/rib/commit.go` shows the Warn record in the guard branch |
| The stale comment is gone | `grep -n "does NOT ask" internal/core/bgp/attribute/mpnlri.go` returns nothing |
| The tests exist and drive the entry point | `grep -n "cs.Commit\|\.Commit(" internal/component/bgp/rib/commit_nexthop_test.go` shows every case calling `Commit`, none calling `buildMPReachNLRI` |
| The package is green | `make ze-unit-pkg-test PKG=./internal/component/bgp/rib` |
| The RFC ledger did not move | `make ze-rfc-check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The next hop is the untrusted input here: it arrives from a route built elsewhere and is written straight to the wire. The guard must reject before the size pass, not after the write |
| Fail closed | A zero `netip.Addr` must never read as a valid answer. Both the standard and the VPN branch return an error; neither returns a zero-length or all-zero next hop |
| Resource exhaustion | None: the guard removes an allocation, it adds none |
| Error leakage | The error names the AFI, the SAFI and the family. It carries no peer address or credential |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| `TestCommitVPNAnnounceCarriesTheRFC4364NextHop` fails in phase 1 | A-1 is broken. STOP, do not delete the encoder, keep phase 2 only, and report |
| An existing test in the package fails after phase 2 | The guard is too strict. Re-read `ValidateNextHops`: it refuses only `!IsValid()` |
| Lint failure | Fix inline. If architectural → report |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Discrimination Proof (2026-08-23)

Every run below is a real execution, not a cached verdict: each reports a duration,
and each mutation is to package source, which changes the `go test` cache key. Each
mutation was verified to have LANDED with a non-empty diff against a pristine copy
taken first, and each restore was a `cp` of that copy followed by a matching SHA-256.

**Phase 1, before the guard existed.** The refusal test failed on all four rows for
the stated reason, `Commit` returning a nil error, and the byte-exact VPN control
passed against the unchanged encoder. A throwaway probe captured what the rail put
on the wire:

```
ipv6:  mpreach value=00 02 01 00 00 20 20 01 0d b8
       AFI=0x0002 SAFI=0x01 NHLen=0x00 Reserved=0x00 NLRI=2001:db8::/32
vpnv4: mpreach value=00 01 80 18 00*24 00 18 c0 a8 01
       AFI=0x0001 SAFI=0x80 NHLen=0x18 then 24 zero octets
```

**Mutation A, the guard deleted from `buildMPReachNLRI`.**

```
--- FAIL: TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm/grouped-ipv6
--- FAIL: TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm/ungrouped-ipv6
--- FAIL: TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm/grouped-vpnv4
--- FAIL: TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm/ungrouped-vpnv4
--- FAIL: TestCommitRefusalOfAnUnencodableNextHopIsLogged
FAIL	github.com/ze-software/ze/internal/component/bgp/rib	0.578s
```

**Mutation B, `attribute.SAFI(fam.SAFI)` perturbed to `fam.SAFI+1`,** to prove the
byte-exact control discriminates rather than passing on shape alone:

```
--- FAIL: TestCommitVPNAnnounceCarriesTheRFC4364NextHop
--- FAIL: TestCommitService_VPNNextHopHasRD
expected: 00 01 80 0c 00*8 0a 00 00 01 00 18 c0 a8 01
actual  : 00 01 81 04 0a 00 00 01 00 18 c0 a8 01
FAIL	github.com/ze-software/ze/internal/component/bgp/rib	0.579s
```

The Route Distinguisher disappears with the SAFI, which is the octet the control
exists to hold. `TestCommitService_VPNNextHopHasRD` caught it too, so the pre-existing
test and the new one are not proving the same thing twice by accident.

**Restored, and green.**

```
ok  	github.com/ze-software/ze/internal/component/bgp/rib	1.965s
```

## Design Insights
- The size-versus-write invariant at the end of `packAttributesWithASPath` used to catch
  this defect by accident. Before `nextHopOctets` was derived from `AsSlice`, a zero
  `netip.Addr` made the prediction and the write differ by 16, so `Commit` refused with
  "BUG: attribute size mismatch". Making the two agree was correct and removed the
  accident. An invariant that catches a second class of defect by coincidence is not a
  guard for that class, and repairing the invariant is what exposes the missing guard.
- Three assembling rails now call the same `ValidateNextHops`. The count is the finding:
  a validation reachable only through `CheckedWriteTo` is reachable by no rail that
  needs an `EncodingContext`, which is every rail that encodes AS_PATH.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Validate inside `buildMPReachNLRI` and give it an error return | Route the whole rail through `CheckedWriteTo` | `CheckedWriteTo` takes no `*bgpctx.EncodingContext`, and `packAttributesWithASPath` needs one for the AS_PATH ASN width (RFC 6793 Section 4.1). Routing through it would four-octet-encode AS_PATH toward a two-octet peer. The same reasoning is already recorded on `CheckedWriteTo` in `mpnlri.go` |
| Delete `vpnMPReachNLRI` and use the core encoder for SAFI 128 | Keep it and add a second `IsValid` check inside `buildVPNMPReachNLRI` | The second check would be a second statement of one rule (`ai/rules/no-layering.md`), and the type's own comment ("MPReachNLRI.WriteTo() doesn't handle RD prefix") is no longer true: `nextHopOctets` adds `RDSize` and `WriteTo` writes the eight zero octets for `SAFIVPN`. Deleting it leaves one encoder, one validator, and one place a future RD change lands |
| Wrap `attribute.ErrUnencodableNextHop` instead of adding a rib sentinel | Add `rib.ErrUnencodableNextHop` | The sentinel already exists and both reactor rails already match on it. A second sentinel means callers must know which rail refused, which no caller needs |
| Refuse the whole `Commit` rather than skipping the route | Drop the offending route and send the rest, as `peer_rib_routes.go` does | `Commit` has no per-route reporting channel and its caller counts announced routes from the input slice, so a silent drop would report routes as announced that were not. The reactor rail that drops has a logging path built for it; this one does not |

## Known Limitations
- `(*reactorAPIAdapter).SendRoutes` (`internal/component/bgp/reactor/reactor_api_batch.go`)
  discards the error from `Commit` with `continue`, so the refusal reaches no user even
  when the rail is live. The Warn record in AC-5 is what makes it visible. Changing
  `SendRoutes` is outside this spec: it is reactor behavior, it is not needed for the
  wire to be correct, and it would take the change past the one package this spec names.
- Whether the set of size-versus-write mismatch sites is CLOSED stays open. It is the
  question `plan/deferrals/fixit-commit-rail-nexthop-unvalidated.md` records, and the
  sibling shards `fixit-attribute-length-from-family.md` and
  `fixit-mpreach-split-undercounts-rd.md` hold the known members. This spec closes one
  rail; it does not audit the class.
- The rail has no non-test caller today, so no `.ci` and no interop scenario can prove
  the guard end to end. Wiring `(*Transaction).QueueAnnounce` to a command is separate
  work with its own value and its own risks, and it is not attempted here.

## RFC Documentation (Scope: protocol)

The guard carries `// RFC 4760 Section 3: "Network Address of Next Hop"` above it,
naming why a next hop with no wire form cannot be encoded, and
`// RFC 7606 Section 7.11` naming what a peer does with the malformed attribute. The
VPN path carries `// RFC 4364 Section 4.3.4` where the Route Distinguisher is now
written, which is `(*MPReachNLRI).WriteTo` in `internal/core/bgp/attribute/mpnlri.go`
and already documented there. The refusal test carries the
`RFC requirement: RFC4760-3-2 negative` tag.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The new test was shown RED with the guard reverted, and the output is pasted
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`), not test-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: `plan/deferrals/fixit-commit-rail-nexthop-unvalidated.md`
      has no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior, or the recorded reason none is owed
- [ ] Interop tests for protocol features, or the recorded reason none is owed

### Closure
- [ ] Implementation session: commit, set Status to `verification`, STOP
- [ ] Verification session (Opus 5): review the commit against every AC
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
