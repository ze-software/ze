# Spec: BCP 194 child 6 -- honour the RFC 7999 BLACKHOLE community

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | done |
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

Both items this paragraph left open were closed on 2026-08-13, in the enrolment
step recorded below. `rfc/enrolled.txt` now carries `rfc7999`, on the extraction
sign-off it was waiting for. The scenario now carries four `RFC requirement:`
tags, serving RFC7999-3.3-1 and RFC7999-3.3-2 in both polarities at
`interop/nightly`. RFC7999-4-1 was left untagged on purpose: the scenario would
serve it positively, but it is SHOULD NOT and nothing gates it, so the tag would
buy a permanent `check_evidence_ratchet` commitment that no gate reads.

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

### Enrolment TAKEN 2026-08-13, on an owner ruling

All five steps are done and `rfc7999` is in `rfc/enrolled.txt`. The gate counts
171 enrolled RFCs and 2967 gated MUSTs, up 4, which are this RFC's four.

**Thomas ruled on RFC7999-3.1-2 twice on 2026-08-13, and the SECOND ruling
governs.** He first chose a `{not-applicable}` annotation, on the reading that
Section 3.1's obligation binds "the two networks" rather than the BGP speaker.
He then replaced that reading the same day: configuring the community on a peer
IS Ze's half of the agreement, so the obligation has a machine-checkable
predicate after all. **The annotation is withdrawn**, `agreedSelector`
(`internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go`) is the
gate, and both polarities are proven at unit tier over the command handler
(`1d2af98ab`). `rfc/short/rfc7999.md` now carries no annotation of any kind, and
the `rfc/enrolled.txt` row records the withdrawal. The paragraphs below were
written under the first ruling and are kept as the record of what was rejected.

Driving `evaluate()` in `scripts/dev/rfc_requirements.py` over the summary and
the live tag set, with `rfc7999` enrolled, returned one violation before the
annotation and zero after.

**Two items noted here rather than fixed, by owner direction.** The stale
`blackholePropagationGuard` comment was corrected in the same change and has its
journal row. The commit-helper gate has a row of its own at
`plan/journal/gate-fires-outside-its-population.md`: `spec_closure_stem` reads
any new journal row naming a spec as a closure signal, so it will refuse
ordinary mid-spec commits now that CLAUDE.md mandates a row per defect found.

### What the enrolment rests on (recorded 2026-08-13)

| Step | State |
|------|-------|
| Extraction sign-off | DONE. `rfc/extraction/rfc7999.json`, signed 2026-08-13. 14 sections and 4 sites, every one classified. 1 site excluded, the IETF Trust Legal Provisions boilerplate of the front matter, so the exclusion ratio is 0.25 over all sites and 0 over the three body sites. `make ze-rfc-check` exits 0, which is what makes the sign-off valid |
| Register | `prose`, not the `rfc2119` this spec predicted. The register is DERIVED and the derivation is right: `prose` is also derived when the source has FEWER MUST-level sites than the summary declares gated rows. RFC 7999 has 3 body sites for 4 gated rows, because Section 3.3's first sentence states one obligation with two bullets and the site scan sees the lead-in sentence alone. No `register-reason` is owed, and none is written |
| Public row | UPDATED, not added. The phase 4 row stood but carried a false claim; see below |
| Interop tags | DONE. `test/interop/scenarios/59-rfc7999-blackhole-frr/check.py` now tags RFC7999-3.3-1 and RFC7999-3.3-2 in both polarities, at `interop/nightly`. This is a permanent `check_evidence_ratchet` commitment: neither requirement may afterwards be proven by unit evidence alone. That is the right commitment, because the kernel-state assertion is the only thing that caught the phase 4 defect |
| Disposition | DONE. `rfc7999` left `rfc/not-enrolled.txt` and arrived in `rfc/enrolled.txt` in one change, which is what `check_summary_disposition` requires: deleting the row alone returns the stem to the undeclared state it refuses |
| The four MUSTs | ALL FOUR proven in both polarities, two of them at `interop/nightly` as well as unit. The fourth, RFC7999-3.1-2, was annotated for part of 2026-08-13 and is now tested: the annotation was withdrawn with `1d2af98ab`. `make ze-rfc-check` exits 0 and `rfc/short/rfc7999.md` carries no annotation |

**The phase 4 ledgers carried a false claim, and it is withdrawn.** Both
`docs/features/rfc-status.md` and `rfc/not-enrolled.txt` said RFC7999-3.1-2 has
no producer "because Ze originates no BLACKHOLE-tagged announcement". Ze does.
`handleAnnounceBlackhole`
(`internal/component/bgp/plugins/cmd/announce/announce.go`) builds a route
carrying `attribute.CommunityBlackhole` and sends it to the peers the command
selector names. It consults no record of any agreement, and no configuration
holds one. **That correction is what the ruling was made on**, so the annotation
states it rather than resting on the absent-code-path grounds every other
`{not-applicable}` in `rfc/short/` uses.

**The reading that was NOT chosen, kept because it is the shape of any future
work here.** Section 3.1's obligation could instead be met the way Section 3.3's
second condition already is: a per-peer leaf that RECORDS the agreement, and a
speaker that refuses to advertise without it. That would be a Tx agreement leaf
defaulting off, a gate in `handleAnnounceBlackhole`, and a tagged pair. The
second advertising path needs nothing: a received BLACKHOLE-tagged route is
withheld from egress once the Section 3.2 guard adds NO_EXPORT or NO_ADVERTISE,
because `Reactor.wellKnownAllowsEgress`
(`internal/component/bgp/reactor/forward_wellknown.go`) suppresses such a route,
and RFC1997-Well-1, Well-2 and Well-3 each carry both polarities. Thomas chose
the annotation over this on 2026-08-13. Reopening it needs a fresh ruling, not a
citation of this paragraph.

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
| What breaks if this is wrong? | Traffic is discarded for a prefix nobody authorized, which is a denial of reachability RFC 7999 Section 6 names. The opposite error is inert and visible: the honoring path returns 0 and an ordinary unicast route is installed, which is what every deployment without a `blackhole` container gets |
| How is it reverted? | Single commit revert of the config surface. Nothing persists: the route type is recomputed per best-path change, and removing the `blackhole` container returns the peer to an ordinary unicast install on the next change |
| Who else touches this path? | `spec-bcp194-1-communities` owns the RFC 7999 Section 3.2 propagation guard in `filter_community`. `spec-bcp194-4-prefix` owns the prefix-limit surface the same `rib` plugin reads. The `rpki` plugin's decision path is shared with origin validation |

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
| 1 | Receives a BLACKHOLE-tagged prefix from a peer that agreed the community and is authorized for a covering block | wire -> `parseCommunityAttr` -> `checkBestPathChange` -> `blackholeRouteTypeForBest` -> `locrib.Path.RouteType` and `ribevents.BestChangeEntry.RouteType` -> sysrib -> `routeTypeToLinux` -> `RTN_BLACKHOLE` | `test/interop/scenarios/59-rfc7999-blackhole-frr` (kernel `ip route show`), `test/plugin/bgp-blackhole-honor.ci` (`show rib`) |
| 2 | Runs `announce blackhole 198.51.100.1/32` and reaches only the sessions that agreed the community | CLI -> `handleAnnounceBlackhole` -> `agreedSelector` -> per-peer fan-out | `TestAnnounceBlackholeReachesAnAgreedPeer`, with `TestAnnounceBlackholeIsWithheldFromAPeerThatDidNotAgree` and `TestAnnounceBlackholeRefusedWhenNoPeerAgreed` as the negatives |
| 3 | Runs origin validation with `reject` and still receives a legitimate /32 blackhole under a maxLength-24 VRP | wire -> `rpki.buildDecisions` -> `invalidByLengthOnly` + `carriesAgreedBlackhole` -> route kept | `TestBlackholeSurvivesLengthOnlyInvalid`, with `TestBlackholeDoesNotSurviveAWrongOrigin` as the negative |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseBlackholeConfigPeerLevel` and 8 siblings | `internal/component/bgp/plugins/rib/rib_blackhole_config_test.go` | the leaves parse, accumulate down bgp/group/peer, and REFUSE a bad community, a bad prefix, a bare address and a peer with no remote IP | done |
| `TestCoveredByAuthorizedPrefix`, `...EmptyListAuthorizesNothing`, `...IsFamilyScoped` | `internal/component/bgp/plugins/rib/rib_blackhole_test.go` | the RFC7999-3.3-1 coverage predicate, its closed state and its family scoping | done |
| `TestBlackholeRouteTypeDecision` | same | the three-condition decision table | done |
| `TestBlackholeRouteTypeStampedOnBestPath` and 6 siblings | `internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go` | AC-1..AC-4, AC-8: the stamp on the real best-path path, and its absence for each of the three conditions | done |
| `TestBlackholeSurvivesLengthOnlyInvalid` and 6 siblings | `internal/component/bgp/plugins/rpki/blackhole_decision_test.go` | AC-5, AC-6: the origin-validation carve-out and its four refusals | done |
| `TestInvalidByLengthOnly*` (5) | `internal/component/bgp/plugins/rpki/blackhole_test.go` | the predicate alone: wrong origin, non-Invalid states, `OriginNone`, unparseable prefix | done |
| `TestCarriesAgreedBlackhole*` (6) | `internal/component/bgp/plugins/rpki/blackhole_agreement_test.go` | the RPKI side reads the session's OWN community set, and fails closed | done |
| `TestAnnounceBlackhole*` / `TestAnnounceUnicast*` (12) | `internal/component/bgp/plugins/cmd/announce/blackhole_agreement_test.go` | RFC7999-3.1-2: both origination verbs narrow the fan-out to the agreed sessions, and refuse when none agreed | done |
| `TestPathEqualRouteType`, `TestPathKeyIgnoresRouteType`, `TestPathRouteTypeDefaultsUnset` | `internal/core/rib/locrib/candidate_routetype_test.go` | A-5: `Equal` sees the route type and `key()` does not, so a type-only change reprograms the FIB | done |
| `TestSysribCarriesRouteTypeFrom{LocRIB,EventBus}` and 4 siblings | `internal/component/sysrib/sysrib_routetype_test.go` | AC-7: both rails, replay, the type-only emit, and the `show rib` rendering | done |
| `TestBestChangeEntryRouteType{RoundTrip,OmittedWhenUnset}` | `internal/core/bgp/ribevents/ribevents_routetype_test.go` | the JSON boundary carries `route-type` and omits it when unset | done |
| `TestValuesMatchLinuxRTN`, `TestZeroIsUnset`, `TestDiscards`, `TestString` | `internal/core/rib/routetype/routetype_test.go` | the moved core type, its zero value and its discard set | done |
| `TestBuildRichRouteDiscardCarriesNoNextHop`, `TestBuildRichRouteUnicastKeepsNextHop` | `internal/plugins/fib/kernel/nexthop_discard_linux_test.go` | the phase-4 defect: a discard route names no gateway, device, multipath or encap, and a unicast route is unchanged | done |
| `TestNetlinkIntegration_BlackholeRouteWithNextHop` | `internal/plugins/fib/kernel/richroute_discard_integration_linux_test.go` | the same, read back from a real kernel in an ephemeral netns | done |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Announced length against a /24 authorization (RFC7999-3.3-1's "equal or shorter") | the authorization must be no more specific than the announcement | `192.0.2.0/24` announced, equal to the authorization, is covered | `192.0.0.0/16` announced is NOT covered: it is shorter, so the /24 covers it rather than the reverse (`TestCoveredByAuthorizedPrefix`) | `192.0.2.128/25` and `192.0.2.1/32` are covered at any greater length; `192.0.3.1/32`, one bit outside the block, is not |
| `routetype.Type` | 0..255, values mirroring Linux `RTN_*` | `RTN_PROHIBIT` | 0 is Unset and never a discard (`TestZeroIsUnset`) | an unmapped value renders as unset (`TestString`) |
| COMMUNITIES value scan | 4-octet steps | a trailing partial community is ignored (`Carries`) | N/A | N/A |

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
- `internal/component/bgp/plugins/rib/rib.go`, `rib_bestchange.go` - read the per-peer config, ask for the route type on each best-path change
- `internal/component/bgp/plugins/rib/yang/ze-rib.yang` - the `blackhole` container
- `internal/component/bgp/plugins/rpki/rpki.go`, `rpki_config.go`, `origin_tracker.go`, `yang/ze-rpki.yang` - the `blackhole-exempt` leaf and the decision carve-out
- `internal/component/bgp/plugins/cmd/announce/announce.go` - both origination verbs go through `agreedSelector`
- `internal/component/bgp/plugins/filter_community/blackhole.go` - the Section 3.2 guard reads the same shared config
- `internal/component/bgp/plugins/filter_modify/*`, `filter_community_match/match.go`, `internal/component/bgp/filtertext/community.go` - the generic `match` condition on a modify action, which this spec needed and which is not blackhole-specific
- `internal/core/bgp/ribevents/ribevents.go`, `internal/core/rib/locrib/candidate.go` - `RouteType` on both rails
- `internal/component/sysrib/sysrib.go`, `events/events.go` - carry and re-emit it; `RouteType` moved to core and aliased
- `internal/plugins/fib/kernel/nexthop_linux.go` - a discard route names no next-hop
- `internal/core/selector/selector.go`, `internal/component/plugin/server/command.go` - `selector.Addrs`, and the peer list the gate reads
- `docs/guide/configuration.md`, `docs/features/rfc-status.md` - the operator surface and the public claim
- `rfc/short/rfc7999.md`, `rfc/enrolled.txt`, `rfc/not-enrolled.txt` - enrolment

## Files to Create
- `internal/core/rib/routetype/routetype.go` - the single definition of the type
- `internal/component/bgp/blackholecfg/blackholecfg.go` - the one reader of the `blackhole` container, shared by the three deciders
- `internal/component/bgp/plugins/rib/rib_blackhole.go`, `rib_blackhole_config.go` - the honoring decision
- `internal/component/bgp/plugins/rpki/blackhole.go` - the origin-validation carve-out
- `internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go` - the send-side gate
- `rfc/extraction/rfc7999.json` - the hand-classified sign-off enrolment requires
- `test/plugin/bgp-blackhole-honor.ci` - the functional test
- `test/interop/scenarios/59-rfc7999-blackhole-frr/` - the FRR and BIRD scenario

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/rib/yang/ze-rib.yang` (`blackhole-honor-fields`), `plugins/rpki/yang/ze-rpki.yang` (`blackhole-exempt`), `plugins/filter_modify/yang/ze-filter-modify.yang` (`match`) |
| YANG validation constraints | Yes | The leaf-lists are `string` because a community accepts two spellings and a prefix accepts both families. The value is validated at parse and REFUSED with the level, the peer and the offending value named (`blackholecfg.parseLevel`), rather than dropped |
| YANG custom validators | No | Native types plus the parse-time refusal above cover it. No `ze:validate` was added |
| CLI commands/flags | No | `announce blackhole` and `announce unicast` already existed. This spec added a gate inside them, not a verb |
| CLI grammar (keyword before value) | N-A | No new command surface |
| Editor autocomplete | N-A | Both leaves are free-form leaf-lists with no enumeration |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-blackhole-honor.ci` |
| Pipe completeness | N-A | `show rib` already routes through the pipe layer; this spec added one field to its output |
| Env var registration | N-A | No `environment/` leaf |
| Doctor check for runtime dependencies | No | No new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink call, binary or certificate. The netlink route program already existed; this spec changed which attributes one message carries |
| Prometheus counters/metrics | No | No metric added. The operator-visible surface is `show rib`'s `route-type` field and the configure log line `blackholeHonorPeerCount` produces |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new AFI/SAFI, capability or attribute. RFC 7999 registers a COMMUNITIES value, which the codec already parsed |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/configuration.md` "Blackhole Honoring (RFC 7999)" carries the whole surface. `docs/features.md` already claims community filters generically and states nothing this change makes false |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`: the `blackhole { communities; prefixes; }` container, `blackhole-exempt`, and the `modify` `match` container |
| 3 | CLI command added/changed? | No | No verb added. `announce blackhole` gained a refusal, documented under "Announcing a blackhole to a peer" in `docs/guide/configuration.md` |
| 4 | API/RPC added/changed? | No | No new RPC. `show rib` gained one optional JSON field, `route-type`, omitted when unset |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` "Route Attribute Modifier": the unconditional claim was FALSE after the `match` container landed. Corrected, with a `match.go -- matchCond, matches` anchor |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md`, sections "Blackhole Honoring (RFC 7999)", "Announcing a blackhole to a peer", "A blackhole on a community Ze does not read" |
| 7 | Wire format changed? | No | Nothing Ze emits changed shape. The COMMUNITIES codec is untouched; this spec reads a registered value |
| 8 | Plugin SDK/protocol changed? | No | `route-type` travels the existing `BestChangeEntry` JSON on both rails, as an added optional field |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7999.md` (4 gated MUSTs, both polarities), `rfc/enrolled.txt`, `rfc/extraction/rfc7999.json`, and the RFC 7999 row of `docs/features/rfc-status.md` with producing-function anchors |
| 10 | Test infrastructure changed? | No | The `.ci` and the interop scenario use the existing runners. No new option, runner or make target |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` names `bgp-filter-modify` for attribute modification and RTBH is not one of its rows. No claim there became false |
| 12 | Internal architecture changed? | Yes | The route type is a new field on both RIB-to-sysrib rails. Recorded in this spec's Data Flow section; `docs/architecture/plugin/rib-storage-design.md` is named by the `// Design:` annotations of every new file |
| 13 | Route metadata keys added/changed? | No | The `meta map[string]any` seam is untouched. `docs/architecture/meta/README.md` says it never reaches the RIB store, which is why the verdict travels as a struct field instead |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No new plugin, event type, send type, command or capability. The inventory pages are unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/plugins.md` anchors `filter_modify.go -- handleFilterUpdate` and `modify.go -- buildDelta, buildDynamicDelta`; `docs/guide/configuration.md` anchors `config.go -- parseModifyDefs`. Both pages claimed the modifier is unconditional and both are corrected above. The `rib`, `rpki`, `sysrib`, `locrib` and `fib/kernel` changes are additive and break no anchored claim |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `modify` example in `docs/guide/configuration.md` was verified against `ze-filter-modify.yang` and now names the three `match` leaf-lists it actually declares |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase 1: the carry-through (landed, `6130c0aaf`)** -- `routetype` in core, `RouteType` on `locrib.Path` and `ribevents.BestChangeEntry`, carried by sysrib on both rails.
   - Tests: `TestPathEqualRouteType`, `TestPathKeyIgnoresRouteType`, `TestSysribCarriesRouteTypeFrom{LocRIB,EventBus}`, `TestBestChangeEntryRouteTypeRoundTrip`
   - Verify: AC-7. A type-only change reprograms the FIB rather than being deduped away
2. **Phase 2: the honoring decision (landed, `9a2ec776b`, `393d4a421`, `822edda2f`)** -- the per-peer config, the coverage predicate, the stamp on the best path, the RPKI carve-out, and `show rib` rendering.
   - Tests: the `rib` and `rpki` blackhole suites
   - Verify: AC-1 to AC-6, AC-8
3. **Phase 3: proof (landed, `3b67e13ce`, `4bc823a7b`, `2ad55236f`)** -- the `.ci`, the FRR and BIRD interop scenario, the requirement tags, and the Linux discard-route defect the scenario found.
   - Verify: the kernel holds one `RTN_BLACKHOLE` and no gateway
4. **Phase 4: enrolment (landed, `586570f95`, `4be9f0594`, `854d246ee`, `d9c724e38`, `1d2af98ab`)** -- the extraction sign-off, `rfc/enrolled.txt`, the operator's own community, the generic `match` condition, and the Section 3.1 send-side gate that replaced the withdrawn annotation.
   - Verify: `make ze-rfc-check` exits 0 with 4 gated MUSTs for `rfc7999`, none annotated

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and a named test |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Coverage is containment plus "authorized no more specific than announced", never `ge`/`le` membership. All three RFC 7999 Section 3.3 conditions are tested in exactly one place each, so no second copy can drift |
| Naming | JSON key `route-type` is kebab-case and omitted when unset. The YANG leaf-lists are plural (`communities`, `prefixes`), matching the leaf-list naming rule |
| Data flow | The verdict is computed where the FIB candidate is built, because the `meta` seam never reaches the RIB store. It rides both rails as one `uint8` |
| Rule: `ai/rules/evidence.md` (fail closed) | Every unreadable input answers "do not honor": an empty community set, an empty authorization set, an absent peer entry, an unparseable prefix, a missing attribute and a trailing partial community |
| Rule: `ai/rules/simplicity.md` | One shared reader (`blackholecfg`) rather than three walks. The Linux next-hop suppression sits at the one place the netlink message is built, rather than clearing the next-hop upstream where VPP and `show rib` both want it |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| The per-peer `blackhole` container reaches the honoring path | `make ze-test-pkg PKG=./internal/component/bgp/blackholecfg` and `PKG=./internal/component/bgp/plugins/rib` |
| A honored route reaches the Linux FIB as a discard route | `test/interop/scenarios/59-rfc7999-blackhole-frr/check.py` asserts `blackhole 10.100.0.1` in `ip route show` inside the ze container |
| The same route type is visible to an operator | `test/plugin/bgp-blackhole-honor.ci`, `show rib` renders `"route-type":"blackhole"` |
| Origin validation does not block a legitimate blackhole | `make ze-test-pkg PKG=./internal/component/bgp/plugins/rpki` |
| Ze advertises BLACKHOLE only to sessions that agreed it | `make ze-test-pkg PKG=./internal/component/bgp/plugins/cmd/announce` |
| RFC 7999 is enrolled with every gated MUST proven | `make ze-rfc-check` exits 0 and `grep rfc7999 rfc/enrolled.txt` returns the row |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | Every configured value is parsed and REFUSED on error, naming the level, the peer and the offending value. A dropped entry would read as an in-force agreement that does nothing (`blackholecfg.parseLevel`, `blackholecfg.Parse`) |
| Untrusted input from the wire | The COMMUNITIES scan steps 4 octets at a time, so a value straddling two adjacent communities never matches. A trailing partial community is ignored (`blackholecfg.Carries`) |
| Authorization that could fail open | Three independent conditions gate a discard, and each answers false on absent or unreadable input. An empty authorization set authorizes nothing, which is the closed state RFC 7999 Section 6 requires |
| Denial of service | The feature DISCARDS traffic, so a wrong honor is a denial of reachability. The coverage predicate is what bounds it, and it is per session: a prefix announced by two peers is honored only when the peer that WON best-path selection is authorized for it |
| Weakening an existing guard | The RPKI carve-out is narrower than "skip validation": it needs a covering VRP naming the origin AS, an Invalid caused by length alone, the route carrying an agreed community, and the operator's `blackhole-exempt` on that session. A wrong origin stays Invalid |
| Resource exhaustion | One atomic load and one map miss per best-path change on a deployment that does not use the feature. No wire scan runs for a peer that stated no rule |

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

- **A feature can be correct at every layer and inert at the boundary.** The
  community was read, the decision was right, the route type rode both rails, and
  the Linux kernel then held nothing, because `fib_create_info` refuses a discard
  route carrying a gateway. Every unit test was green. Only an assertion on
  KERNEL state saw it, which is why the interop scenario reads `ip route show`
  rather than a Ze table.
- **A test plugin that produces a value nothing else produces hides the missing
  producer.** `fakefib` injected a route type at the sysrib boundary, so
  `test/plugin/fib-blackhole.ci` proved the FIB half of a chain whose upper half
  did not exist.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The agreed COMMUNITY LIST is the agreement, with no separate boolean | An `honor true` leaf beside the list | A peer that named no community agreed to nothing, so one list answers Section 3.3's receive condition and Section 3.1's send condition. A boolean beside it can disagree with it |
| Coverage walks the authorized set directly | Reuse `filter_prefix.evaluatePrefix` with its `ge`/`le` bounds | RFC 7999 asks whether a shorter authorized prefix CONTAINS the announcement. A `192.0.2.0/24` entry with no `le` bound rejects the /32 inside it, which is the opposite answer |
| Suppress the next-hop where the netlink message is built | Clear the next-hop upstream, in the RIB or sysrib | Upstream clearing destroys data VPP and `show rib` both want, and spreads a Linux constraint into two protocol-tier packages |
| A shared `blackholecfg` package | A walk in each of the three deciders | Three answers about one session that can drift is the failure the `filtertext` package was extracted to prevent |
| Section 3.1 is met by a send-side gate | The `{not-applicable}` annotation held until 2026-08-13 | Configuring the community on a peer IS Ze's half of the agreement, so the obligation has a machine-checkable predicate. The owner replaced the annotation with the gate the same day |
| A generic `match` container on `modify` | A blackhole-specific modifier | A filter chain is a pipe where a reject DROPS the route, so an earlier match filter cannot express "modify these and pass the rest untouched". The condition belongs on the modifier, and nothing about it is blackhole-specific |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- The authorization Section 3.3 turns on is operator-supplied. `blackhole
  prefixes` is a configured leaf-list, not a view derived from IRR or RPKI.
  Deriving it is `plan/future/spec-blackhole-authorization-from-irr.md`.
- The honoring machinery is bilateral. A route server does not apply the two
  Section 3.3 conditions to its clients. RFC7999-3.3-3 states that as a SHOULD,
  is not gated, and is disclosed in the RFC 7999 row of
  `docs/features/rfc-status.md`.
- RFC7999-6-1's strict filtering, and RFC7999-3.1-3 and RFC7999-3.1-5 on the
  sender, carry no test. None is gated, so none is ratcheted, and each is named
  in the same public row.

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

## Implementation Summary

### What Was Implemented

- A route type travels from the BGP RIB to the FIB on both rails. `routetype.Type`
  is defined once in `internal/core/rib/routetype` and aliased by `sysribevents`,
  so every existing consumer kept compiling. It rides `locrib.Path.RouteType` and
  `ribevents.BestChangeEntry.RouteType`, and `Path.Equal` sees it while `key()`
  does not, so a type-only change reprograms the FIB instead of being deduped.
- One shared reader of the per-peer `blackhole` container,
  `internal/component/bgp/blackholecfg`. Three deciders consume it: the honoring
  path, origin validation, and the origination path.
- The honoring decision, `blackholeRouteType` and `coveredByAuthorized`
  (`internal/component/bgp/plugins/rib/rib_blackhole.go`). All three RFC 7999
  Section 3.3 conditions are tested in exactly one place each, and the expensive
  one (the wire scan) runs last.
- The origin-validation carve-out, `invalidByLengthOnly` and
  `carriesAgreedBlackhole` (`internal/component/bgp/plugins/rpki/blackhole.go`),
  reached through the `blackhole-exempt` leaf.
- The send-side gate, `agreedSelector`
  (`internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go`). Both
  `announce blackhole` and `announce unicast community 65535:666` narrow the
  fan-out to the sessions that named the community, and refuse when none did.
- A generic `match` container on the `modify` filter
  (`internal/component/bgp/plugins/filter_modify/match.go`). Nothing about it is
  blackhole-specific; this spec needed it and it shipped as its own artifact.
- RFC 7999 enrolled: `rfc/extraction/rfc7999.json` (14 sections, 4 sites, 1
  excluded, register `prose`), `rfc/enrolled.txt`, and the public row.

### Bugs Found/Fixed

- **No honored route could reach the Linux kernel.** `buildRichRoute`
  (`internal/plugins/fib/kernel/nexthop_linux.go`) set `route.Gw` from the
  change's next-hop whatever the route type was, and `fib_create_info` rejects
  `RTN_BLACKHOLE`, `RTN_UNREACHABLE` and `RTN_PROHIBIT` that carry a gateway, a
  device or multipath. Every BGP path resolves a next-hop, so the whole netlink
  message was refused and nothing was programmed. Fixed by returning early for
  `routetype.Discards()` at the one place the message is built. Covered by
  `TestBuildRichRouteDiscardCarriesNoNextHop` and, in a real kernel,
  `TestNetlinkIntegration_BlackholeRouteWithNextHop`.
- **The Section 3.1 annotation rested on a false premise.** Both
  `docs/features/rfc-status.md` and `rfc/not-enrolled.txt` said RFC7999-3.1-2 had
  no producer because Ze originates no BLACKHOLE-tagged announcement. Ze does,
  through `handleAnnounceBlackhole`. The claim was withdrawn and the requirement
  is now gated by `agreedSelector` with both polarities tested.
- **`docs/guide/plugins.md` claimed `bgp-filter-modify` applies its operations
  unconditionally**, which the `match` container made false. Corrected at closure,
  together with the same claim in `docs/guide/configuration.md`.

### Documentation Updates

- `docs/guide/configuration.md`: "Blackhole Honoring (RFC 7999)", "Announcing a
  blackhole to a peer", "A blackhole on a community Ze does not read", and the
  Route Attribute Modifier `match` container. Anchors name
  `ze-rib.yang -- blackhole-honor-fields`, `blackholecfg.go -- Rule, Parse,
  Carries, Agreed`, `rib_blackhole.go -- coveredByAuthorized, blackholeRouteType`,
  `blackhole_agreement.go -- agreedSelector`, `ze-rpki.yang -- blackhole-exempt`,
  `blackhole.go -- invalidByLengthOnly, carriesAgreedBlackhole` and
  `match.go -- matchCond, matches`.
- `docs/guide/plugins.md`: the Route Attribute Modifier section, with a
  `match.go -- matchCond, matches` anchor added.
- `docs/features/rfc-status.md`: the RFC 7999 row, its coverage text and its
  Remaining cell.
- `make ze-doc-test` and `make ze-validate` both exit 0.

### Deviations from Plan

- The extraction register came out `prose`, not the `rfc2119` this spec
  predicted. The register is DERIVED, and `prose` is also derived when the source
  has fewer MUST-level SITES than the summary declares gated rows. Section 3.3
  states one obligation with two bullets and the site scan sees the lead-in
  sentence alone: 3 body sites for 4 gated rows. No `register-reason` is owed and
  none is written.
- RFC7999-3.1-2 was annotated `{not-applicable}` and then tested instead, on a
  second owner ruling the same day. The spec's "Enrolment TAKEN" section records
  both rulings and says which governs.
- The `match` container on `modify` was not in the original design. It was needed
  to express the honoring condition an operator states, and it shipped as a
  generic artifact rather than a blackhole-specific one.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The Linux next-hop was to be cleared upstream, in the RIB or sysrib, so the FIB would receive a discard route with no gateway | That destroys data the VPP backend and `show rib` both want, and spreads a Linux netlink constraint into two protocol-tier packages | Reading the two consumers of `RichRoute` before editing | The suppression sits at the one place the netlink message is built, scoped by `routetype.Discards()` |
| assumption | The published ledgers said Ze originates no BLACKHOLE-tagged announcement, which is what the Section 3.1 annotation rested on | `handleAnnounceBlackhole` builds a route carrying `attribute.CommunityBlackhole` and sends it to the peers the selector names | Reading the producer before writing the annotation's justification | The claim was withdrawn from both ledgers, the owner re-ruled, and the requirement is now tested rather than annotated |
| escalation | A test plugin was the only producer of a route type in the tree, so `test/plugin/fib-blackhole.ci` proved the FIB half of a chain whose upper half did not exist | A fixture that produces a value no production code produces makes the consumer's test vacuous about the feature | The phase-4 kernel measurement | Recorded as a Design Insight. The interop scenario now asserts KERNEL state, which is the only level at which the defect was visible |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| S3.3: honor under both conditions | Done | `blackholeRouteType`, `coveredByAuthorized` (`rib_blackhole.go`) | RFC7999-3.3-1 and RFC7999-3.3-2, both polarities, unit and `interop/nightly` |
| S3.3: a route server applies the same conditions | Changed | not implemented | RFC7999-3.3-3 is a SHOULD and is not gated. Disclosed in the RFC 7999 row of `docs/features/rfc-status.md` and in Known Limitations |
| S3.3: origin validation does not block a legitimate blackhole | Done | `invalidByLengthOnly`, `carriesAgreedBlackhole` (`rpki/blackhole.go`) | RFC7999-3.3-4, both polarities |
| S3.1: the owner decides whether Ze honors BLACKHOLE | Done | owner decision 2026-08-08, recorded in Task | Enrolled, and this child does it |
| S3.1: agreement before advertising | Done | `agreedSelector` (`announce/blackhole_agreement.go`) | RFC7999-3.1-2, both polarities. The earlier annotation is withdrawn |
| S4: the shorthand keyword `blackhole` | Done | `attribute.ParseCommunity`, reached from `blackholecfg.parseLevel` | Both spellings resolve to one value. RFC7999-4-1's default is proven by `TestBlackholeNotStampedWithoutAgreement` |
| S6: strict filtering, and filter when verification fails | Changed | the coverage predicate is the verification | RFC7999-6-1 is not gated. The authorization it turns on is operator-supplied, which Known Limitations states |
| Enrolment with every MUST proven or annotated | Done | `rfc/enrolled.txt`, `rfc/extraction/rfc7999.json` | 4 gated MUSTs, zero annotations. `make ze-rfc-check` exits 0 |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestBlackholeNotStampedWithoutAgreement` | A peer with no `blackhole` container never reaches the map; RFC7999-4-1 positive |
| AC-2 | Done | `TestBlackholeRouteTypeStampedOnBestPath`; `test/plugin/bgp-blackhole-honor.ci`; scenario 59 asserts `blackhole 10.100.0.1` in the kernel | RFC7999-3.3-2 positive at both tiers |
| AC-3 | Done | `TestBlackholeNotStampedOutsideAuthorization`; the same `.ci` asserts 192.0.2.1/32 carries no route type; scenario 59's uncovered outcome | RFC7999-3.3-1 negative at both tiers |
| AC-4 | Done | `TestBlackholeNotStampedWithoutTheCommunity` | An untagged route from an opted-in peer installs ordinarily |
| AC-5 | Done | `TestBlackholeSurvivesLengthOnlyInvalid` | RFC7999-3.3-4 positive |
| AC-6 | Done | `TestBlackholeDoesNotSurviveAWrongOrigin`, `TestBlackholeExemptionPreservesTheInvalidState` | RFC7999-3.3-4 negative |
| AC-7 | Done | `TestSysribCarriesRouteTypeFromLocRIB`, `TestSysribCarriesRouteTypeFromEventBus`, `TestSysribEmitsOnRouteTypeOnlyChange`, `TestSysribReplayCarriesRouteType` | Both rails plus replay |
| AC-8 | Done | `TestBlackholeRemovedWhenPrefixWithdrawn`, `TestBlackholeClearedWhenCommunityRemoved` | Withdrawal and community removal both clear it |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestParseBlackholeConfigPeerLevel` and 8 siblings | Done | `rib/rib_blackhole_config_test.go` | Includes four refusal cases |
| `TestCoveredByAuthorized` group and `TestBlackholeRouteTypeDecision` | Done | `rib/rib_blackhole_test.go` | |
| `TestBlackholeRouteTypeStampedOnBestPath` and 6 siblings | Done | `rib/rib_blackhole_wiring_test.go` | Carries the RFC requirement tags |
| `TestBlackholeSurvivesLengthOnlyInvalid` and 6 siblings | Done | `rpki/blackhole_decision_test.go` | |
| `TestInvalidByLengthOnly` group (5) and `TestRPKICarriesBlackhole` group (2) | Done | `rpki/blackhole_test.go` | |
| `TestCarriesAgreedBlackhole` group (6) and the two parse tests | Done | `rpki/blackhole_agreement_test.go` | |
| `TestAnnounceBlackhole` and `TestAnnounceUnicast` groups (12) | Done | `announce/blackhole_agreement_test.go` | RFC7999-3.1-2 both polarities |
| `TestPathEqualRouteType` and 2 siblings | Done | `locrib/candidate_routetype_test.go` | |
| `TestSysribCarriesRouteType` group and 4 siblings | Done | `sysrib/sysrib_routetype_test.go` | |
| `TestBestChangeEntryRouteType` group (2) | Done | `ribevents/ribevents_routetype_test.go` | |
| `TestValuesMatchLinuxRTN` and 3 siblings | Done | `routetype/routetype_test.go` | |
| `TestBuildRichRouteDiscardCarriesNoNextHop`, `TestBuildRichRouteUnicastKeepsNextHop`, `TestNetlinkIntegration_BlackholeRouteWithNextHop` | Done | `fib/kernel/` | The netns test is the only level the defect was visible at |
| `bgp-blackhole-honor` | Done | `test/plugin/bgp-blackhole-honor.ci` | PASS under `make ze-plugin-test` on 2026-08-13 |
| `59-rfc7999-blackhole-frr` | Done | `test/interop/scenarios/59-rfc7999-blackhole-frr/` | Three outcomes, each proven to discriminate by mutation |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/core/rib/routetype/routetype.go` | Done | Created; aliased by `sysribevents` |
| `internal/component/bgp/blackholecfg/blackholecfg.go` | Done | Created; the one reader, shared by three deciders |
| `internal/component/bgp/plugins/rib/rib_blackhole.go` and `rib_blackhole_config.go` | Done | Created |
| `internal/component/bgp/plugins/rpki/blackhole.go` | Done | Created |
| `internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go` | Done | Created |
| `internal/component/bgp/plugins/filter_modify/match.go` | Changed | Not in the original file list; the generic `match` artifact this spec needed |
| `internal/plugins/fib/kernel/nexthop_linux.go` | Changed | Not in the original file list; the phase-4 defect |
| `rfc/extraction/rfc7999.json`, `rfc/enrolled.txt`, `rfc/short/rfc7999.md` | Done | Enrolment |
| `test/plugin/bgp-blackhole-honor.ci` and `test/interop/scenarios/59-rfc7999-blackhole-frr/` | Done | Created |

### Audit Summary

- **Total items:** 8 requirements, 8 acceptance criteria, 14 test groups, 9 file groups
- **Done:** 6 requirements, 8 ACs, 14 test groups, 7 file groups
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 requirements (RFC7999-3.3-3 and RFC7999-6-1, both ungated SHOULD-level, publicly disclosed rather than implemented) and 2 file groups (the `match` container and the kernel fix, both added), each recorded in Deviations or Known Limitations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Ze honors a received BLACKHOLE route under RFC 7999 Section 3.3's two conditions | interop | `test/interop/scenarios/59-rfc7999-blackhole-frr/check.py` asserts `blackhole 10.100.0.1` in `ip route show` inside the ze container, from a real FRR 10.3.1 announcement. Discrimination proven three ways: disabling the honor path reddens only the positive, bypassing the coverage test reddens only the uncovered negative, ignoring the `honor` leaf reddens only the un-agreed negative |
| Honoring is off by default (Section 4) | functional | `test/plugin/bgp-blackhole-honor.ci`: 192.0.2.1/32 carries the same community from the same session and reaches the system RIB with NO route type. Unit: `TestBlackholeNotStampedWithoutAgreement` |
| The verdict is per BGP session, not daemon-wide | interop | The same scenario runs BIRD on a session that named 65001:666 alone. It announces a COVERED prefix and the kernel forwards it, so the negative isolates the session agreement rather than an absent config block |
| Origin validation does not block a legitimate blackhole | functional, at unit tier over the decision path | `make ze-test-pkg PKG=./internal/component/bgp/plugins/rpki` green. `TestBlackholeSurvivesLengthOnlyInvalid` positive, `TestBlackholeDoesNotSurviveAWrongOrigin` negative |
| Ze advertises BLACKHOLE only where the two networks agreed (Section 3.1) | functional, at unit tier over the command handler | `TestAnnounceBlackholeReachesAnAgreedPeer`, `TestAnnounceBlackholeIsWithheldFromAPeerThatDidNotAgree`, `TestAnnounceUnicastWithBlackholeCommunityMeetsTheSameGate` |
| A discard route reaches the Linux kernel | integration | `TestNetlinkIntegration_BlackholeRouteWithNextHop` programs a real kernel in an ephemeral netns and reads back exactly one `RTN_BLACKHOLE` with no gateway. With the guard removed the netns holds zero ze routes |
| RFC 7999 is enrolled with no gap and no annotation | gate output | `make ze-rfc-check` exits 0: 2967 gated MUSTs across 171 enrolled RFCs, `interop/nightly` evidence 26. A grep for `{gap}` and `{not-applicable}` over `rfc/short/rfc7999.md` returns nothing |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| No shard. The metadata `Deferral shard` cell is `-` and no file `plan/deferrals/bcp194-6-blackhole.md` was ever created | done | A grep for `bcp194-6-blackhole` over `plan/deferrals/` returns nothing, as both Source and Destination. No other shard names this spec as the home of any live row |
| The IRR-derived authorization this spec did not build | deferred | `plan/future/spec-blackhole-authorization-from-irr.md`, created 2026-08-13 alongside `plan/future/spec-irr-filtering-both-directions.md`. It is a `plan/future/` spec, not a live deferral row, so it homes the work without holding this spec open |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | written by `review_gate.py record` under `tmp/review/`, hash-pinned over the 24 code and test files of this spec |
| `review_gate.py check` | clean |
| Rounds | 1. An independent closure session, not the implementing one, read the producing function behind every claim rather than a diff summary, and found no BLOCKER and no ISSUE in the product |
| Reviewer lenses used | fail-closed guards and authorization (every input that could answer "honor" wrongly), RFC conformance against `rfc/short/rfc7999.md` and the enrolled row, wire-value handling (the 4-octet community scan), Linux netlink constraints, config parse refusal against silent drop, and doc-claim staleness against changed source anchors |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `bgp-filter-modify` was documented as applying its operations unconditionally, which the `match` container made false. The claim carries source anchors pointing at the changed files, so checklist item 16 owns it | `docs/guide/plugins.md` and `docs/guide/configuration.md`, both "Route Attribute Modifier" sections | Both rewritten to state the `match` container, its three leaf-lists and the pass-through-unchanged semantics, with a `match.go -- matchCond, matches` anchor added to each |
| 2 | ISSUE | Three specs cited `plan/spec-bcp194-6-blackhole.md`, which commit B removes, and one unrelated spec already cited a spec closed earlier without clearing its citers. `make ze-spec-citation-check` exited 2 | `plan/spec-bcp194-0-umbrella.md`, `plan/spec-bcp194-1-communities.md` (2 sites), `plan/spec-hub-deferred-api-auth-independent-of-ssh-block.md` (3 sites) | Each citation restated with the bare stem and the durable document that replaced the spec. `make ze-spec-citation-check` now exits 0 |
| 3 | NOTE | The spec's own "Enrolment TAKEN" section said Thomas chose the `{not-applicable}` annotation for RFC7999-3.1-2. He replaced that ruling the same day and the annotation is withdrawn | this spec, "Enrolment TAKEN 2026-08-13" | The section now records both rulings and says which governs. A record defect, fixed in one edit, and it earned no extra round |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/rib/routetype/routetype.go` | Yes | `ls internal/core/rib/routetype/` lists `routetype.go` and `routetype_test.go` |
| `internal/component/bgp/blackholecfg/blackholecfg.go` | Yes | `ls` lists it beside `blackholecfg_test.go` |
| `internal/component/bgp/plugins/rib/rib_blackhole.go`, `rib_blackhole_config.go` | Yes | a glob over `rib/*blackhole*` lists both plus three test files |
| `internal/component/bgp/plugins/rpki/blackhole.go` | Yes | a glob over `rpki/*blackhole*` lists it plus three test files |
| `internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go` | Yes | `ls` lists it beside `blackhole_agreement_test.go` |
| `internal/component/bgp/plugins/filter_modify/match.go` | Yes | read in full at closure |
| `test/plugin/bgp-blackhole-honor.ci` | Yes | `ls -la` reports 5517 bytes |
| `test/interop/scenarios/59-rfc7999-blackhole-frr/` | Yes | `ls -la` lists `bird.conf`, `check.py`, `frr.conf`, `ze.conf` |
| `rfc/extraction/rfc7999.json` | Yes | `make ze-rfc-check` reports it signed off at register `prose` |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | An unconfigured peer installs an ordinary route | `TestBlackholeNotStampedWithoutAgreement` found in `rib_blackhole_wiring_test.go`; `make ze-test-pkg PKG=./internal/component/bgp/plugins/rib` exits 0 |
| AC-2 | A covered tagged prefix becomes a discard route | Same package green, plus `bgp-blackhole-honor` PASS in `make ze-plugin-test` on 2026-08-13 |
| AC-3 | An uncovered tagged prefix installs ordinarily | The same `.ci` asserts the 192.0.2.1/32 object closes right after `priority`, so it carries no `route-type` |
| AC-4 | An untagged prefix installs ordinarily | `TestBlackholeNotStampedWithoutTheCommunity` present and green |
| AC-5 | An Invalid-by-length blackhole survives origin validation | `make ze-test-pkg PKG=./internal/component/bgp/plugins/rpki` exits 0 |
| AC-6 | A wrong origin stays Invalid | Same run; `TestBlackholeDoesNotSurviveAWrongOrigin` present |
| AC-7 | The route type reaches the FIB on both rails | `make ze-test-pkg PKG=./internal/component/sysrib` and `PKG=./internal/core/rib/locrib` both exit 0 |
| AC-8 | A withdrawal removes the discard route | `TestBlackholeRemovedWhenPrefixWithdrawn` present and green in the `rib` package run |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `bgp { peer X { blackhole { communities; prefixes } } }` | `test/plugin/bgp-blackhole-honor.ci` | Yes. The config block states `communities [ blackhole 65001:666 ]` and `prefixes 10.0.0.0/24`, and the third UPDATE proves the operator's own value fires, which a hardcoded 65535:666 could not |
| A received UPDATE carrying 65535:666 from an opted-in peer | `test/plugin/bgp-blackhole-honor.ci` | Yes. The hex of the first UPDATE carries the COMMUNITY attribute `C00804FFFF029A` and NLRI `200A000001` |
| The stamped route type leaving the BGP RIB | `test/plugin/bgp-blackhole-honor.ci` | Yes. `show rib` is the system RIB view, reached only after sysrib arbitration and next-hop resolution, and the `.ci` injects a covering route so both prefixes resolve |
| The stamped route type reaching the kernel | `test/interop/scenarios/59-rfc7999-blackhole-frr/check.py` | Yes. `check.py` runs `ip route show` inside the ze container and asserts `blackhole 10.100.0.1`, so the assertion is kernel state rather than a Ze table |
| An Invalid-by-length blackhole under origin validation | none, proven at unit tier | Yes, over `rpki.buildDecisions`. No `.ci` exists for this row and none is claimed: the RFC requirement tags for RFC7999-3.3-4 name the unit tests |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `ribevents.BestChangeEntry` had no route type at HEAD; the field was added by this spec and `TestBestChangeEntryRouteTypeRoundTrip` pins its JSON |
| A-2 | confirmed | The RIB plugin's `WantsConfig` names `bgp`, which delivers the whole subtree, and `parseBlackholeConfig` reads peers straight out of it |
| A-3 | confirmed | `configjson.PeerRemoteIP` is the reader `blackholecfg.Parse` uses, and a peer with no usable remote IP is REFUSED rather than skipped |
| A-4 | confirmed | `routetype.Type` moved to core and aliased; every FIB and sysrib package compiles and its tests pass |
| A-5 | confirmed | `TestPathEqualRouteType` and `TestPathKeyIgnoresRouteType` both green, so a type-only change reprograms rather than dedupes |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| The `blackhole` container's two leaf-lists and their meaning | `ze-rib.yang` grouping `blackhole-honor-fields`, and `blackholecfg.Parse` accumulates both down bgp, group and peer | Yes |
| `prefixes` alone resolves the community to the well-known value | `Rule.applyDefaultCommunity` returns unchanged when `Communities` is non-empty OR `Authorized` is empty, so only the prefixes-alone case is filled in | Yes |
| A stated `communities` list is taken exactly, with the well-known value never unioned in | The same function; `test/plugin/bgp-blackhole-honor.ci` proves the operator's own value fires and the interop BIRD leg proves a stated list excludes the well-known value | Yes |
| Coverage is not prefix-list membership | `coveredByAuthorized` walks the set and tests containment plus an authorized length no greater than the announced length, with no `ge` or `le` bound | Yes |
| The `modify` filter applies conditionally when a `match` container is stated | `matchCond.matches` returns true for an empty condition and ORs the listed values otherwise; the YANG declares `community`, `large-community` and `extended-community` | Yes, and the previous unconditional claim was corrected on both pages |
| The RFC 7999 public row's "no tracked gap" | A grep for `{gap}` and `{not-applicable}` over `rfc/short/rfc7999.md` returns nothing, and `make ze-rfc-check` exits 0 | Yes |
| No documentation category was answered No without a check | Rows 3, 4, 7, 8, 10, 11, 13, 14 and 15 each name the surface that would have changed and why it did not | Yes |

## Core Insight

A feature can be correct at every layer and still do nothing, and the layer that
proves it is the one outside the process. Ze read the community, took the right
decision, carried the verdict on both rails, and the Linux kernel held no route,
because a discard route may not name a next-hop and every BGP path resolves one.
Every unit test was green throughout. What found it was an assertion on kernel
state, and what had hidden it was a test plugin that produced a route type no
production code produced, which made the FIB's own test vacuous about the chain
above it.
