# Spec: BCP 194 child 6 -- honour the RFC 7999 BLACKHOLE community

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | spec-bcp194-1-communities |
| Phase | 4/4 |
| Deferral shard | - |
| Updated | 2026-08-13 |

## Phase 4 complete 2026-08-13 (NOT closure)

Phases 1-3 landed in six commits. Phase 4 closes the three items they left, and
found one defect that made the whole feature inert on Linux.

| Owed | State |
|------|-------|
| Functional `.ci` | DONE. `test/plugin/bgp-blackhole-honor.ci` takes `option=linger:value=true`. `runMessageLoop` reaches `Peer.completed` from its `action=send` exhaustion site (`internal/test/peer/peer.go`), so linger holds the session open there and the two assertions read one steady state. 8/8 alone, 6/6 under `make ze-plugin-test`. The reported flake did NOT reproduce on this machine without linger either (6/6), so linger is justified by the teardown path in source, not by an observed red |
| Interop with FRR | DONE. `test/interop/scenarios/59-rfc7999-blackhole-frr/`. FRR and BIRD both announce 65535:666; `check.py` asserts `ip route show` inside the ze container. Three outcomes from one community: `blackhole 10.100.0.1`, `198.51.100.1 via` (uncovered), `10.200.0.1 via` (honor false). All three proven to discriminate by mutation, each rebuilt from source |
| Docs | DONE. `docs/guide/configuration.md` gains "Blackhole Honoring (RFC 7999)" with the coverage-vs-prefix-list distinction and a "Blackhole with origin validation" subsection naming `blackhole-exempt`. `docs/features/rfc-status.md` gains the RFC 7999 row. `rfc/not-enrolled.txt` records what enrolment still owes |

**The defect phase 4 found: no honored route could reach the Linux kernel.**
`buildRichRoute` (`internal/plugins/fib/kernel/nexthop_linux.go`) set
`route.Gw` from the change's next-hop whatever the route type was. Linux
`fib_create_info` rejects RTN_BLACKHOLE, RTN_UNREACHABLE and RTN_PROHIBIT that
carry a gateway, a device or multipath, and every BGP path resolves a next-hop.
Measured in an ephemeral netns: without the fix the kernel holds ZERO ze routes
for a blackhole change, with it exactly one RTN_BLACKHOLE and no gateway. No
test looked, because `test/plugin/fib-blackhole.ci` is a blind hold and the only
producer of a route type before phase 1 was the `fakefib` test plugin.

`rfc/enrolled.txt` is untouched, which the owner directed: enrolling RFC 7999
needs an extraction sign-off, and that is a separate step nobody has authorised.
The scenario carries NO `RFC requirement:` tag: an interop-tier tag is a
permanent `check_evidence_ratchet` commitment and is the owner's to make. It
WOULD serve RFC7999-3.3-1 (positive and negative), RFC7999-3.3-2 (positive and
negative) and RFC7999-4-1 (positive).

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/planning.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec is child 6 of the BCP 194 spec set. The umbrella is
`plan/spec-bcp194-0-umbrella.md`.

Goal: decide and implement what Ze does with a received route that carries the
RFC 7999 BLACKHOLE community (65535:666).

`parseCommunityAttr` in `internal/component/bgp/wireu/community.go` sets
`CommunityPolicy.RFC7999Blackhole`. No code reads that field. Ze recognizes the
community and takes no action on it.

Child 1 (`plan/spec-bcp194-1-communities.md`) owns the two pieces inside its own
ingress engine. Section 3.2 asks a receiver to add NO_ADVERTISE or NO_EXPORT and
keep the prefix inside the local AS. Child 1 also guarantees that its new
own-Global-Administrator scrub keeps every well-known community. This child owns
the rest: honoring the community.

| RFC 7999 section | What this child must settle |
|------------------|-----------------------------|
| S3.3 | A bilateral peer honors BLACKHOLE under two conditions: the neighbor is authorized to advertise an equal or shorter covering prefix, and the receiver agreed to the community. The RFC states this as one MUST sentence with two bullets, and the summary captures it as two requirements because each is independently violable |
| S3.3 | A route server, and any multilateral topology, applies the same two conditions |
| S3.3 | Origin validation does not block a legitimate BLACKHOLE announcement, which interlocks with `plan/spec-bcp194-4-prefix.md` |
| S3.1 | Ignoring the community is conformant, so the owner decides whether Ze honors BLACKHOLE. Settle this before implementation |
| S4 | A vendor can offer a shorthand configuration keyword for the community. The RFC suggests the string `blackhole` |
| S6 | A receiver verifies a blackhole announcement by strict filtering, and filters it when verification fails |

Honoring discards traffic, so this child reaches the FIB and the firewall, not
only the BGP tables.

Owner decision, 2026-08-08: **RFC 7999 is enrolled, and this child does it.**
Enrolment lands here rather than earlier because it requires every MUST-level
requirement to be classified, and all four are in this child's scope. Child 1
touches RFC 7999 only at section 3.2, which is a SHOULD and is not gated.

| Enrolment step | Detail |
|----------------|--------|
| Extraction sign-off | `make ze-rfc-extract STEM=rfc7999`, then classify every derived site and section by hand at `rfc/extraction/rfc7999.json`. The generated skeleton always fails the check; only the walk makes it pass |
| Register | `rfc2119`. The register is derived and refuses a stronger claim than the source supports. RFC 7999 carries capitalised keywords, so no `register-reason` is owed |
| Disposition | The stem leaves `rfc/not-enrolled.txt` by arriving in `rfc/enrolled.txt`. Deleting the row alone returns it to the undeclared state, which the gate refuses |
| Public row | `docs/features/rfc-status.md` gains an RFC 7999 row in the same change |
| The four MUSTs | The section 3.1 Tx obligation, the two section 3.3 Rx conditions, and the section 3.3 origin-validation obligation. Each needs a tagged positive and negative pair, or an owner-authorised annotation |

Reach enrolment with the MUSTs already proven or already annotated. Six ratchets
judge the stem from that commit on, and a stem that enrols and then loses a
proof is what they refuse.

### Enrolment prepared 2026-08-13, NOT taken (one owner ruling is owed)

Four of the five enrolment steps are done. `rfc/enrolled.txt` is untouched,
because taking the fifth step today would land a red gate that the ratchets then
hold.

| Step | State |
|------|-------|
| Extraction sign-off | DONE. `rfc/extraction/rfc7999.json`, signed 2026-08-13. 14 sections and 4 sites, every one classified. 1 site excluded, the IETF Trust Legal Provisions boilerplate of the front matter, so the exclusion ratio is 0.25 over all sites and 0 over the three body sites. `make ze-rfc-check` exits 0, which is what makes the sign-off valid |
| Register | `prose`, not the `rfc2119` this spec predicted. The register is DERIVED and the derivation is right: `prose` is also derived when the source has FEWER MUST-level sites than the summary declares gated rows. RFC 7999 has 3 body sites for 4 gated rows, because Section 3.3's first sentence states one obligation with two bullets and the site scan sees the lead-in sentence alone. No `register-reason` is owed, and none is written |
| Public row | UPDATED, not added. The phase 4 row stood but carried a false claim; see below |
| Interop tags | DONE. `test/interop/scenarios/59-rfc7999-blackhole-frr/check.py` now tags RFC7999-3.3-1 and RFC7999-3.3-2 in both polarities, at `interop/nightly`. This is a permanent `check_evidence_ratchet` commitment: neither requirement may afterwards be proven by unit evidence alone. That is the right commitment, because the kernel-state assertion is the only thing that caught the phase 4 defect |
| Disposition | NOT taken. `rfc7999` stays in `rfc/not-enrolled.txt`, with its reason corrected |

**The phase 4 ledgers carried a false claim, and it is withdrawn.** Both
`docs/features/rfc-status.md` and `rfc/not-enrolled.txt` said RFC7999-3.1-2 has
no producer "because Ze originates no BLACKHOLE-tagged announcement". Ze does.
`handleAnnounceBlackhole`
(`internal/component/bgp/plugins/cmd/announce/announce.go`) builds a route
carrying `attribute.CommunityBlackhole` and sends it to the peers the command
selector names. It consults no record of any agreement, and no configuration
holds one.

**What blocks enrolment is exactly one requirement, and this is measured rather
than reasoned.** Driving `evaluate()` in `scripts/dev/rfc_requirements.py` over
the summary and the live tag set, with `rfc7999` added to the enrolled set,
returns one violation: `RFC7999-3.1-2 [MUST] has no test and no annotation`.

**The ruling that is owed.** RFC 7999 Section 3.1 states "use of the BLACKHOLE
community MUST be agreed upon by the two networks before advertising it". Its
actor is "the two networks", where Section 3.3's first sentence says "BGP
speakers", so the document draws the distinction deliberately. Two readings
follow and the owner picks one:

| Reading | What it costs |
|---------|---------------|
| The obligation binds an out-of-band agreement between operators, so no daemon can be held to it | An owner-authorised `{not-applicable}` annotation on RFC7999-3.1-2, and enrolment lands the same day |
| The obligation is met the way Section 3.3's second condition already is, by a per-peer leaf that RECORDS the agreement and a speaker that refuses to advertise without it | A Tx agreement leaf defaulting off, a gate in `handleAnnounceBlackhole`, and a tagged pair. The second advertising path is already covered: a received BLACKHOLE-tagged route is withheld from egress once the Section 3.2 guard adds NO_EXPORT or NO_ADVERTISE, because `Reactor.wellKnownAllowsEgress` (`internal/component/bgp/reactor/forward_wellknown.go`) suppresses such a route and RFC1997-Well-1, Well-2 and Well-3 each carry both polarities. So the work is bounded by the `announce` path |

The second reading is a spec, not a step. Neither is written here, because an
annotation that lowers what Ze owes is the owner's to authorise
(`ai/rules/rfc-compliance.md`).

## Design (settled 2026-08-13, re-read against HEAD)

**Both ends exist. The work is the middle.**

| Piece | State at HEAD | Work |
|-------|---------------|------|
| The community is read on ingress for a normal peer | **Exists.** `blackholePropagationGuard` (`internal/component/bgp/plugins/filter_community/blackhole.go`) is the production reader of `wireu.CommunityPolicy.RFC7999Blackhole` | consume it |
| A discard route reaches the kernel and VPP | **Exists.** `sysribevents.RouteTypeBlackhole` is consumed by `internal/plugins/fib/kernel/nexthop_linux.go` (`RTN_BLACKHOLE`) and `internal/plugins/fib/vpp/backend.go` (drop) | none |
| A route type travels from the BGP RIB to sysrib | **ABSENT, and this is the spec's real work** | see below |
| The Section 3.3 coverage test | **Exists.** `evaluatePrefix` (`internal/component/bgp/plugins/filter_irr/match.go`) already computes "covered by an equal or shorter prefix the neighbor may advertise", and `prefixListFromIRR` sets `le: 32` / `le: 128`, so a /32 blackhole inside an authorized block already passes | wire it |
| Per-peer honour leaf | absent | small |

**The carry-through is the new plumbing.** Verified at HEAD by `documentSymbol`: `ribevents.BestChangeEntry` has 14 fields and no route type. Neither has `locrib.Path` nor sysrib's `protocolRoute`. The only non-test code that sets `RouteType:` is the two FIB backends, reading it off an incoming change. So the FIB consumes a route type nothing upstream produces, and the sole producer in the tree is the test plugin `fakefib`, injecting at the sysrib event boundary. That is why `test/plugin/fib-blackhole.ci` proves the FIB half and nothing above it.

`BestChangeEntry` already carries `ProtocolType`, so a type field on that struct has precedent. A route type is a different axis and needs its own.

**Owner ruling, 2026-08-12: honouring is a per-peer option, defaulting off.** RFC 7999 Section 4 states the same default. Section 3.3 requires agreement "on the particular BGP session", so a daemon-wide switch would be non-conformant.

### The trap that decides whether this feature works at all

`(*ROACache).Validate` (`internal/component/bgp/plugins/rpki/validate.go`) applies `prefixLen <= entry.MaxLength` per RFC 6811. **A /32 blackhole under a ROA with maxLength 24 is `Invalid`**, so an operator running invalid-reject drops it before any honour path sees it.

That is Section 3.3's fourth requirement in live code: "An operator MUST ensure that origin validation techniques do not inadvertently block legitimate announcements carrying the BLACKHOLE community." **The design MUST answer it.** Ze's only lever today is the per-peer `originInvalidAction` override, which is all-or-nothing for the session. Shipping without addressing this gives the operators most likely to want the feature a switch that silently does nothing.

### The route server

`RSBlackhole` is Ze's own `RS:0` suppress convention and is **unrelated** to RFC 7999. `StripControlCommunities` strips only `0:X` and `RS:X`, so `65535:666` already passes a route server untouched, correct per RFC 7947. Section 3.3's multilateral paragraph needs the same per-client machinery as the bilateral case, once that exists.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/meta/README.md` - route metadata is ingress-to-egress only
  → Constraint: the `meta map[string]any` seam never reaches the RIB store, so a
    honour verdict computed in an ingress filter cannot travel to the FIB. The
    verdict is therefore computed where the FIB candidate is built.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7999.md` - the four MUSTs and the Section 4 default
  → Constraint: RFC7999-3.3-1 asks for COVERAGE by an equal or shorter
    authorized prefix, which is NOT the same test as prefix-list membership
    under `ge`/`le`. A list saying `192.0.2.0/24 le 24` authorizes the /24, and
    the /32 blackhole inside it is covered while failing the `le` bound.
  → Constraint: RFC7999-4-1 makes OFF the default, so no configuration keeps
    today's behavior.

**Key insights:**
- Both ends of the chain exist; the missing middle is a route type travelling
  from the BGP RIB to sysrib. See the Design section above.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/bgp/ribevents/ribevents.go` - `BestChangeEntry`, 14 fields, no route type
- [ ] `internal/core/rib/locrib/candidate.go` - `Path`, carry-through fields excluded from `key()` and included in `Equal`
- [ ] `internal/component/sysrib/sysrib.go` - `protocolRoute`, `recomputeBest`, `changeToBatch`, eight `outgoingChange` literals
- [ ] `internal/component/sysrib/events/events.go` - `RouteType` and `RouteTypeBlackhole`, consumed by both FIB backends
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `checkBestPathChange` writes BOTH rails: `mirrorToLocRIB` and the `bestChangeEntry`
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - `buildDecisions` rejects an Invalid route when the peer action is reject
- [ ] `internal/component/bgp/plugins/rpki/validate.go` - `(*ROACache).Validate` returns Invalid on `prefixLen > MaxLength`
- [ ] `internal/component/bgp/plugins/filter_prefix/match.go` - `evaluatePrefix` enforces `ge`/`le`; coverage needs its own predicate
- [ ] `internal/component/bgp/configjson/traverse.go` - `ForEachPeer` and `PeerRemoteIP`, the correct peer-by-IP reader

**Behavior to preserve:**
- A peer with no blackhole configuration installs an ordinary unicast route, exactly as today.
- `RSBlackhole` (`RS:0`) stays Ze's own route-server suppress convention, untouched.
- `StripControlCommunities` keeps passing `65535:666` through a route server.

**Behavior to change:**
- A peer that opts in installs a discard route for a covered BLACKHOLE-tagged prefix.
- A BLACKHOLE-tagged route from an opted-in peer is not dropped by origin
  validation when the only reason it is Invalid is that the prefix is longer
  than a covering VRP's maxLength.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire bytes: a received BGP UPDATE whose COMMUNITIES attribute (type 8) carries
  0xFFFF029A, from a peer whose config sets `blackhole honor true`.
- Config: `bgp { peer X { blackhole { honor; authorized-prefix-list NAME } } }`.

### Transformation Path
1. `rpki.buildDecisions` keeps the route when Invalid is length-only and the route is a candidate blackhole.
2. `rib.checkBestPathChange` selects the best path and asks `blackholeRouteType` for a route type.
3. That route type rides BOTH rails out of the RIB: `locrib.Path.RouteType` and `ribevents.BestChangeEntry.RouteType`.
4. `sysrib.changeToBatch` (Loc-RIB rail) and `sysrib.processEvent` (event-bus rail) copy it into `protocolRoute.routeType`.
5. `sysrib.recomputeBest` copies the winner's route type into `sysribevents.BestChangeEntry.RouteType`.
6. `fib/kernel.routeTypeToLinux` maps it to `RTN_BLACKHOLE`; `fib/vpp.routeTypeToVPP` maps it to a drop path.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BGP RIB plugin ↔ sysrib | `ribevents.BestChangeEntry.RouteType`, json `route-type` | Yes |
| BGP RIB plugin ↔ sysrib (in-process) | `locrib.Path.RouteType` | Yes |
| sysrib ↔ FIB plugin | `sysribevents.BestChangeEntry.RouteType`, json `route-type` | Yes |

### Integration Points
- `internal/core/rib/routetype` - the single definition of the type, aliased by `sysribevents` so every existing consumer keeps compiling.
- `internal/component/bgp/plugins/rib` - reads the per-peer config from the `bgp` subtree it already receives.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the route type travels the same two rails every other FIB field travels |
| No unintended coupling (components stay isolated) | Yes | RPKI and the RIB each read the same peer leaf from the config subtree they already receive; neither calls the other |
| No duplicated functionality (extends existing, does not recreate) | Yes | `RouteType` is MOVED to core and aliased, never redefined |
| Zero-copy preserved where applicable (refs, not copies) | Yes | one `uint8` per best-path change, on the cold path |
| Registration over hardcoding | Yes | the leaf is YANG in the owning plugin, read through `configjson.ForEachPeer` |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ribevents.BestChangeEntry` carries no route type, so the FIB consumes a field nothing upstream produces | `internal/core/bgp/ribevents/ribevents.go` read at HEAD | the carry-through is already there and only the honour path is owed | read the struct | confirmed |
| A-2 | The BGP RIB plugin receives the whole `bgp` subtree, peers included | `rib.go` `WantsConfig: []string{"bgp"}` | the honour config needs another host plugin | read the registration | confirmed |
| A-3 | `PeerRemoteIP` is the correct peer-by-IP reader for a plugin config map | `configjson/traverse.go` doc comment, naming RPKI, watchdog and role as the precedent | peers key by name and the RIB cannot match on address | read the function | confirmed |
| A-4 | Moving `RouteType` to core and aliasing it in `sysribevents` keeps every existing consumer compiling | Go type aliases; six non-test consumers found by grep | the move becomes a rename across the FIB backends | `make ze-test-pkg` on the FIB packages | confirmed |
| A-5 | `locrib.Path.Equal` is what makes a carry-through change re-program the FIB, and `key()` is what must not see it | `candidate.go` comments on `Labels` and `BackupNextHop` | a route-type-only change is deduped away and never reaches the kernel | `TestPathEqualRouteType` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Honouring installs a discard route for a prefix the peer was not authorized for | AC-3 fails | the coverage predicate runs before the stamp, and its absence is the OFF state |
| R-2 | The RPKI carve-out weakens origin validation more widely than RFC 7999 Section 3.3 asks | AC-6 fails | the carve-out requires all three of: peer opted in, route carries BLACKHOLE, and a covering VRP naming the origin AS |
| R-3 | A route-type-only change is suppressed by the same-best short circuit and never reaches the kernel | a blackhole applied to an already-installed prefix does not appear in `ip route` | `Path.Equal` includes the route type, and the RIB's own short circuit is measured against it |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [live sessions dropped / routes mis-encoded / config rejected / nothing user-visible] |
| How is it reverted? | [single commit revert / needs config migration / not revertible once peers see it] |
| Who else touches this path? | [other plugins, components, or specs working the same files] |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by .claude/hooks/validate-spec.sh, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `bgp { peer X { blackhole { honor; authorized-prefix-list L } } }` | → | `parseBlackholeConfig` in the RIB plugin | `TestParseBlackholeConfigPeerLevel` |
| A received UPDATE carrying 65535:666 from an opted-in peer | → | `(*RIBManager).blackholeRouteType` | `TestBlackholeRouteTypeStampedOnBestPath` |
| The stamped route type leaving the BGP RIB | → | `sysrib.changeToBatch` and `sysrib.processEvent` | `TestSysribCarriesRouteTypeFromLocRIB`, `TestSysribCarriesRouteTypeFromEventBus` |
| The stamped route type reaching the kernel | → | `fib/kernel.routeTypeToLinux` | `test/plugin/bgp-blackhole-honor.ci` |
| An Invalid-by-length blackhole under origin validation | → | `rpki.buildDecisions` | `TestBlackholeSurvivesLengthOnlyInvalid` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer with no blackhole configuration sends 192.0.2.1/32 carrying 65535:666 | An ordinary unicast route is installed. Nothing is discarded (RFC7999-4-1) |
| AC-2 | A peer with `blackhole honor true` and an authorized list holding 192.0.2.0/24 sends 192.0.2.1/32 carrying 65535:666 | A discard route for 192.0.2.1/32 reaches the FIB (RFC7999-3.3-2) |
| AC-3 | The same peer sends 198.51.100.1/32 carrying 65535:666, outside every authorized entry | An ordinary unicast route is installed. No discard route (RFC7999-3.3-1) |
| AC-4 | The same peer sends 192.0.2.1/32 with no BLACKHOLE community | An ordinary unicast route is installed |
| AC-5 | An opted-in peer sends a covered /32 carrying BLACKHOLE, and a covering VRP names its origin AS with maxLength 24, under `origin-invalid-action reject` | The route is not rejected by origin validation and the discard route is installed (RFC7999-3.3-4) |
| AC-6 | An opted-in peer sends a /32 carrying BLACKHOLE whose origin AS matches no covering VRP, under `origin-invalid-action reject` | The route is rejected, as any Invalid route is |
| AC-7 | A route type set on a best path in the BGP RIB | It arrives at the FIB unchanged on both the Loc-RIB rail and the event-bus rail |
| AC-8 | An opted-in peer withdraws the blackhole prefix | The discard route is removed from the FIB |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [for example "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-blackhole-honor` | `test/plugin/bgp-blackhole-honor.ci` | An opted-in peer's tagged /32 inside its authorized block shows `route-type: blackhole` in `show rib`; a tagged /32 outside it shows no route type at all | done, 8/8 alone and 6/6 under the suite |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `59-rfc7999-blackhole-frr` | `test/interop/scenarios/59-rfc7999-blackhole-frr/` | FRR 10.3.1 and BIRD | Ze discards a real FRR announcement in the Linux FIB, and forwards the two that fail one RFC 7999 Section 3.3 condition each. `check.py` reads `ip route show` in the ze container | done. Discrimination proven three ways: disabling the honor path reddens only the positive; bypassing the coverage test reddens only the uncovered negative; ignoring the `honor` leaf reddens only the un-agreed negative |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/...` - [feature changes]

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves. Dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for `source: <changed-file>` and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: the entry point exists and is reachable. The wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [feature-specific, for example "merge order correct", "error messages name the offending value"] |
| Naming | [feature-specific, for example "JSON keys kebab-case", "YANG leaf matches env var leaf"] |
| Data flow | [feature-specific, for example "resolution in X only, reactor unaware of Y"] |
| Rule: [relevant rule] | [what to check] |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
