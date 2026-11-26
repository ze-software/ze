# Spec: srv6-bestpath-resolvability -- SRv6 Service SID Resolvability Before Best-Path

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Unresolvable SRv6 Service SIDs must be excluded before BGP best-path selection, not declined after winning and losing the fallback. |
| Scope | protocol |
| RFC gaps | RFC9252-5-2 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| RFC 9252 Section 5 local-policy resolvability bypass (`MAY` clause) | Add an explicit YANG config option (per peer or per family) allowing local policy to bypass the SRv6 Service SID reachability check when an alternate steering mechanism (such as an SR Policy) reaches the egress PE; default remains strict resolvability enforcement before best-path. |

## Task

RFC 9252 Section 5 (`rfc/full/rfc9252.txt`):

> "Therefore, the ingress PE MUST perform a resolvability check for the SRv6
> Service SID before considering the received prefix for the BGP best path
> computation. The resolvability is evaluated as per [RFC4271]. If the SRv6 SID
> is reachable via more than one forwarding table, local policy is used to
> determine which table to use."

Section 6 repeats that sentence verbatim for EVPN over SRv6, so the obligation
covers every address family that can carry an SRv6 Service SID (IPv4/IPv6
unicast, IPv4/IPv6 VPN, and EVPN), not L3VPN alone.

`rfc/short/rfc9252.md` records `RFC9252-5-2` as a `{gap}`. Ze acts as an SRv6
ingress PE: `entrySRv6SID` / `srv6SIDFromResult` reconstructs the winning path's
Service SID (`internal/component/bgp/plugins/rib/rib_bestchange.go`) and the FIB
plugins install it as a kernel SEG6 encap route
(`internal/plugins/fib/kernel/nexthop_linux.go`) or a VPP SR steering policy
(`internal/plugins/fib/vpp/srv6.go`). Today `bgp-rib` already excludes paths
whose `NEXT_HOP` is unresolvable (RFC 4271 Section 9.1.2, via
`candidateNextHopResolved` in `rib_next_hop.go`) and paths whose Prefix-SID
attribute carries SRv6 Service TLVs with no valid SID (`isSRv6Ineligible`,
`RFC9252-5-1`), but it performs no reachability check on the reconstructed SRv6
Service SID itself.

### The operator-visible defect

The only SRv6 Service SID reachability check runs inside `sysrib` (`resolvedFibEntry`
-> `srv6SIDResolvable` in `internal/component/sysrib/sysrib.go`), AFTER both BGP
best-path selection and Loc-RIB cross-protocol arbitration have already chosen a
winner. When the winning BGP path's Service SID has no covering route in the
Loc-RIB, `resolvedFibEntry` returns `fibForbidden` and withholds the FIB write.

Two operator-visible defects follow:

| Consequence | Why it happens |
|-------------|----------------|
| The prefix is dark, with no fallback | Losing at `resolvedFibEntry` is not losing BGP best-path. A reachable runner-up BGP path never wins the prefix, and a static route at a worse administrative distance stays unprogrammed while the unprogrammable BGP winner holds the Loc-RIB entry |
| The dark path is re-advertised | BGP selected the unresolvable SRv6 path as best, so Ze propagates it to downstream peers and attracts traffic it cannot encapsulate |

Moving the SRv6 Service SID resolvability check into `bgp-rib`'s pre-ranking
forwarding admission (beside `candidateNextHopResolved`, using the same shared
`igpcost` / forked `route-metrics` resolution and coalescing reselection workers)
fixes both defects: an unresolvable Service SID costs the path its candidacy so a
reachable BGP alternative or static backup wins, and re-evaluation restores or
withdraws the retained Adj-RIB-In path automatically when locator reachability
changes.

### Local-policy bypass (RFC 9252 Section 5 MAY clause -- owner decision 2026-11-24)

RFC 9252 Section 5 also permits local policy to ignore the SRv6 Service SID
resolvability verdict when an alternate steering mechanism (such as an SR Policy)
reaches the egress PE. Per Thomas's 2026-11-24 ruling, this spec adds an explicit
YANG container `srv6-sid-resolvability` in `internal/component/bgp/plugins/rib/yang/ze-rib.yang`,
augmented at `/bgp:bgp`, `/bgp:bgp/bgp:group`, `/bgp:bgp/bgp:peer`, and
`/bgp:bgp/bgp:group/bgp:peer`, with:

- `ignore` (boolean, no YANG default so inheritance works across `bgp` -> `group` -> `peer`; absent at every level enforces strict resolvability, `false`).
- `family <name> { ignore <bool>; }` (list keyed by `zt:address-family`, mandatory boolean `ignore` leaf) for per-family overrides within any scope.

Precedence is strictly most-specific wins: peer family > peer > group family >
group > global `bgp` family > global `bgp` > strict default (`false`). Dynamic
peer groups resolve through `configjson.LookupPeerConfig(..., peerGroupName)`
just like `blackhole` and `flowspec-redirect-check`. Reload and rollback run
through `config_tx.go` and trigger `reselectAIGPRoutes` when the effective rules
change, so toggling the bypass re-evaluates retained Adj-RIB-In routes with no
BGP UPDATE required.

The bypass skips ONLY the SRv6 Service SID reachability check. It never bypasses
RFC 9252 Section 7 semantic SID validity (`isSRv6Ineligible`) and never bypasses
RFC 4271 Section 9.1.2 `NEXT_HOP` resolvability (`candidateNextHopResolved`).

### Relationship to RFC9252-5-1

| Requirement | Its sentence | Property tested | State today |
|-------------|--------------|-----------------|-------------|
| `RFC9252-5-1` | "The path having any such Prefix-SID attribute without any valid SRv6 SID information MUST be considered ineligible during the selection of the best path for the corresponding prefix" (`rfc/full/rfc9252.txt`, Section 7) | SEMANTIC validity of the SID information carried in the attribute | Implemented by `isSRv6Ineligible`, proven by `rfc9252_srv6_ineligible_test.go`, `rfc9252_srv6_admission_test.go`, and `rfc9252_srv6_transposition_test.go` |
| `RFC9252-5-2` | Section 5, quoted above (repeated in Section 6 for EVPN) | REACHABILITY of the reconstructed Service SID address, "evaluated as per [RFC4271]" | `{gap}` -- closed by this spec |

All existing `RFC9252-5-1` tests stay green and MUST NOT be edited: a path with a
semantically valid but unreachable SID passes `isSRv6Ineligible` (`RFC9252-5-1`)
and fails `candidateSRv6SIDResolved` (`RFC9252-5-2`).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/route-selection.md` - Phase 2 pre-ranking next-hop admission (`candidateNextHopResolved`), prospective CIDR preview, forked `route-metrics` RPC, `show bgp rib` visibility, and runtime doctor reporting
  → Decision: extend the existing pre-ranking forwarding admission path (`rib_next_hop.go`, `rib_commands.go`, `rib_manual.go`), `show bgp rib` row annotations (`rib_pipeline_bestfact.go`), and runtime doctor checks (`rib_next_hop_doctor.go`) instead of inventing a parallel seam or reselection worker
  → Constraint: forwarding admission runs only when `admission.requiresResolution()` is true (`candidateAdmissionForwarding`), preserving `candidateAdmissionRecovery` for source-owned Adj-RIB-Out recovery; forked mode MUST work identically over `route-metrics` and fail closed on lookup error
- [ ] `docs/architecture/plugin/rib-storage-design.md` - Adj-RIB-In retention, candidate gathering lock order (`shard.mu` -> `peerMu.RLock`), per-peer config extraction (`flowspec-redirect-check`, `blackhole`), and Prometheus metrics
  → Constraint: `candidateSRv6SIDResolved` runs under `peerMu.RLock`, so it MUST NOT take BGP write locks or re-enter `RIBManager`; `srv6ResolvabilityCfg` is published via `atomic.Pointer` for lock-free reads
- [ ] `docs/architecture/config/transaction-protocol.md` - plugin `ConfigTx` verify/apply/rollback lifecycle (`config_tx.go`)
  → Constraint: `parseSRv6ResolvabilityConfig` validates all values cleanly in verify without mutating live state; apply and rollback compare rules and call `reselectAIGPRoutes` when rules change
- [ ] `docs/architecture/doctor-and-health-checks.md` - Stage 1 `runtime`-phase doctor check declarations and `diagnostic.Register` metadata
  → Constraint: declare `bgp-rib-unresolved-srv6-sid` only in Stage 1 `sdk.Registration.DoctorChecks` (never as a `runtime`-phase `registry.DoctorCheckDef`), register `doctor-bgp-srv6-sid-unresolvable` in `doctor_codes.go`, and read bounded live state without taking `peerMu` or making engine RPCs
- [ ] `docs/features/srv6.md` - published SRv6 feature page and RFC 9252 compliance table
  → Decision: update Section 5 compliance row, data flow, and configuration docs once pre-best-path SRv6 SID resolvability and `srv6-sid-resolvability` land

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9252.md` - `RFC9252-5-1`, `RFC9252-5-2`, and `## Meta` Support rows
  → Constraint: remove `{gap}` on `RFC9252-5-2` only once both positive and negative tagged tests and the discrimination record exist
- [ ] `rfc/full/rfc9252.txt` Sections 5, 6, and 7 - authoritative MUST and MAY sentences
  → Constraint: Section 5 (L3) and Section 6 (EVPN) bind every family carrying an SRv6 Service SID; Section 5 MAY permits explicit local-policy bypass while keeping strict enforcement as the default
- [ ] `docs/contributing/rfc-conformance-gates.md` - obligations for new RFC tags
  → Constraint: new `RFC9252-5-2` tags require a discrimination record in `rfc/discrimination/rfc9252/RFC9252-5-2.json` produced by `./le rfc record` in the same change

**Key insights:**
- Since this spec was first drafted, HEAD landed the full RFC 4271 Section 9.1.2 pre-ranking next-hop resolvability infrastructure: `internal/core/rib/igpcost` (`Resolve`, `Lookup`, `ResolveProspective`), `RIBManager.nextHopResolved` / `igpDistance` (`rib_next_hop.go`, `rib_remote.go`), engine `route-metrics` RPC (`dispatch_route_metrics.go`), coalescing internal Loc-RIB `OnChange` + forked revision-polling reselection workers (`runAIGPSelection`, `runRemoteMetrics`, `reselectAIGPRoutes` in `rib_aigp.go` and `rib_remote.go`), `show bgp rib` ineligibility visibility (`rib_pipeline_bestfact.go`), and bounded runtime doctor reporting (`rib_next_hop_doctor.go`).
- Calling `nextHopResolved` with the reconstructed merged Service SID from `entrySRv6SID` gives SRv6 SID resolvability internal AND forked support, recursive 8-hop LPM resolution, discard/self-loop rejection, CIDR prospective self-capture detection, and automatic Loc-RIB / config-reload reselection without duplicating any seam or worker.
- `ze-rib.yang` already establishes the exact four-scope BGP augmentation pattern (`bgp`, `group`, `peer`, `group/peer`) without YANG defaults on inherited booleans (`flowspec-redirect-check-fields`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `gatherPrefixCandidatesForLocked` (CIDR) and `gatherKeyCandidatesForLocked` (non-CIDR) call `candidateAdmitted` (`SourcePolicyEligible`, `isSRv6Ineligible`, `isSelfNextHop`), extract each candidate, and when `admission.requiresResolution()` is true call `candidateNextHopResolved`; neither checks SRv6 Service SID reachability
- [ ] `internal/component/bgp/plugins/rib/rib_manual.go` - `appendManualCandidatesLocked` runs the same `candidateAdmitted` + `candidateNextHopResolved` gate for manually injected BGP routes
- [ ] `internal/component/bgp/plugins/rib/rib_next_hop.go` - `candidateNextHopResolved`, `nextHopResolved`, and `nextHopResolvedCached` evaluate RFC 4271 Section 9.1.2 resolvability via `igpcost.ResolveProspective` / `remote.prospectiveDistance` (CIDR) and `r.igpDistance` (non-CIDR), failing closed when no covering route resolves or when a forked `route-metrics` RPC fails
- [ ] `internal/component/bgp/plugins/rib/rib_remote.go` - `igpDistance` resolves via `igpcost.Lookup` in-process and via `route-metrics` RPC when forked; `runRemoteMetrics` polls the engine revision every 1s, invalidates caches, and triggers `reselectAIGPRoutes`
- [ ] `internal/component/bgp/plugins/rib/rib_aigp.go` - `runAIGPSelection` subscribes to `loc.OnChange` (coalescing via size-1 channel) and calls `reselectAIGPRoutes`, which walks every retained route in `r.bgpPeers` and `r.ribInPool[bgpProtocolID]` (CIDR and non-CIDR, admitted and excluded) outside locks and calls `checkBestPathChange` + `publishBestChanges`
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `isSRv6Ineligible` enforces `RFC9252-5-1` semantic SID validity; `entrySRv6SID` / `srv6SIDFromResult` reconstructs the full merged SRv6 Service SID (applying `nlrisplit.TranspositionLabel` + `pool.ApplyTransposition` for VPN families); `checkRouteBestChange` passes the winning SID to Loc-RIB (`mirrorToLocRIB`) and `best-change` events
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go`, `rib_pipeline_bestfact.go` - `markNextHopFacts` and `writeNextHopIneligible` annotate received non-best `show bgp rib` rows whose next hop fails forwarding admission with `"next-hop-resolvable": false` and `"ineligible-reason": "next-hop-unresolvable"`, using cached lookups fail-closed during a forked `route-metrics` outage
- [ ] `internal/component/bgp/plugins/rib/rib_next_hop_doctor.go`, `doctor_codes.go` - `unresolvedNextHopTracker` (bounded to 4096 entries + overflow filter) backs runtime doctor check `bgp-rib-unresolved-next-hop` (`doctor-bgp-next-hop-unresolvable`)
- [ ] `internal/component/bgp/plugins/rib/rib.go`, `config_tx.go`, `rib_flowspec_redirect.go`, `rib_blackhole_config.go`, `yang/ze-rib.yang` - `RIBManager` runs in-process with `locrib.Default()` or forked with `setupRemoteRIB()`, registers Stage 1 `DoctorChecks` and Prometheus metrics, and loads per-peer/group/global YANG augmentations (`blackhole`, `flowspec-redirect-check`) through `ConfigTx` with `reselectAIGPRoutes` on selection-rule changes
- [ ] `internal/core/rib/igpcost/igpcost.go`, `prospective.go` - bounded 8-hop recursive LPM resolution over `locrib.RIB`, rejecting missing covers, discard routes, and self-loops, plus prospective CIDR overlay resolution
- [ ] `internal/component/plugin/server/dispatch_route_metrics.go` - engine `route-metrics` RPC answering `igpcost.Resolve` / `igpcost.ResolveProspective` for forked `bgp-rib`
- [ ] `internal/component/sysrib/sysrib.go`, `nhresolver.go`, `sysrib_srv6_test.go` - `resolvedFibEntry` checks `srv6SIDResolvable` (`nhResolver.Resolve`) at FIB install time and returns `fibForbidden` for unreachable SIDs (already covered by 6 tests in `sysrib_srv6_test.go`); its comment still references this spec's old path `plan/immediate/spec-srv6-bestpath-resolvability.md`
- [ ] `internal/core/bgp/nlri/nlrisplit/transposition.go` - `TranspositionLabelWidth` returns 20 for VPNv4/VPNv6, 24 for EVPN, 0 otherwise; `TranspositionLabel` extracts the 20-bit VPN label (EVPN label transposition is tracked separately in `plan/release/fixes/spec-srv6-evpn-label-width.md`)
- [ ] `internal/component/bgp/plugins/rib/rfc9252_srv6_ineligible_test.go`, `rfc9252_srv6_admission_test.go`, `rfc9252_srv6_transposition_test.go` - existing `RFC9252-5-1` and transposition tests (must remain untouched and green)
- [ ] `docs/features/srv6.md` - Section 5 row accurately states "Partial: sysrib blocks FIB installation without a resolvable SID; BGP pre-selection filtering is not implemented"
- [ ] `test/plugin/fib-srv6-kernel.ci`, `test/interop/scenarios/bgp-srv6-frr/ze.conf` - existing SRv6 kernel FIB functional test and FRR interop scenario

**Behavior to preserve:**
- `isSRv6Ineligible` and all existing `RFC9252-5-1` tests (`rfc9252_srv6_ineligible_test.go`, `rfc9252_srv6_admission_test.go`, `rfc9252_srv6_transposition_test.go`), untouched.
- `sysrib`'s `resolvedFibEntry` / `srv6SIDResolvable` FIB-time guard and `sysrib_srv6_test.go`, unchanged in behavior as defense-in-depth.
- `candidateAdmissionRecovery` (`requiresResolution() == false`) continuing to omit local forwarding resolvability checks for source-owned Adj-RIB-Out recovery.
- RFC 4271 Section 9.1.2 `NEXT_HOP` resolvability admission, `show bgp rib` `next-hop-unresolvable` annotations, and `bgp-rib-unresolved-next-hop` doctor reporting.
- Existing behavior for routes carrying no SRv6 Service SID (zero overhead fast path when `!HasOtherAttrs()`).

**Behavior to change:**
- In forwarding admission (`candidateAdmissionForwarding`), a route whose reconstructed merged SRv6 Service SID has no resolving path in the Loc-RIB (evaluated via the same RFC 4271 resolution engine as `NEXT_HOP`, including CIDR prospective self-capture and forked `route-metrics` RPC) is excluded from best-path candidacy unless explicitly bypassed by local policy.
- Operators can configure `srv6-sid-resolvability { ignore true|false; family <name> { ignore true|false; } }` at `bgp`, `group`, `peer`, or `group/peer` scope to bypass the SRv6 Service SID reachability check per peer or per family (strict enforcement remains the default); config reload and rollback re-evaluate retained routes immediately.
- Routes excluded by an unresolvable SRv6 Service SID remain retained in Adj-RIB-In, re-enter selection automatically when locator reachability changes, appear in `show bgp rib` with `"srv6-sid-resolvable": false` and `"ineligible-reason": "srv6-sid-unresolvable"`, are counted in `ze_rib_srv6_sid_unresolvable_exclusions_total`, and are surfaced by runtime doctor check `bgp-rib-unresolved-srv6-sid` (`doctor-bgp-srv6-sid-unresolvable`).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Core recursive reachability & prospective resolver | `internal/core/rib/igpcost/igpcost.go` (`Resolve`, `Lookup`), `prospective.go` (`ResolveProspective`) | Performs bounded 8-hop recursive LPM resolution over `locrib.RIB`, rejecting missing covers, discard routes, self-loops, and CIDR prospective self-capture | Extends (reuses directly): `candidateSRv6SIDResolved` delegates SID reachability to `nextHopResolved`, which calls `igpcost.ResolveProspective` (CIDR) and `igpcost.Lookup` (non-CIDR) with no second leaf seam |
| Pre-ranking forwarding admission & forked metric RPC | `internal/component/bgp/plugins/rib/rib_next_hop.go` (`candidateNextHopResolved`, `nextHopResolved`, `nextHopResolvedCached`), `rib_remote.go` (`igpDistance`, `remoteRIB`), `internal/component/plugin/server/dispatch_route_metrics.go` (`opRouteMetrics`) | Excludes unresolvable `NEXT_HOP` candidates before Phase 2 ranking in both internal and forked `bgp-rib` placements, failing closed on lookup errors | Extends: adds `candidateSRv6SIDResolved` and cached show helpers beside `candidateNextHopResolved` in `rib_next_hop.go`, `rib_commands.go`, and `rib_manual.go` |
| Loc-RIB change & forked revision reselection workers | `internal/component/bgp/plugins/rib/rib_aigp.go` (`runAIGPSelection`, `reselectAIGPRoutes`), `rib_remote.go` (`runRemoteMetrics`) | Coalesces Loc-RIB `OnChange` notifications (internal) and polls engine routing revisions (forked) to re-run `checkBestPathChange` across every retained Adj-RIB-In route (including excluded routes) outside all locks | Extends (reuses directly): no second `OnChange` subscriber or custom dependency index is built; `reselectAIGPRoutes` already re-evaluates retained CIDR and non-CIDR SRv6 routes on every reachability or config change |
| Merged SRv6 SID extraction & Section 7 semantic filter | `internal/component/bgp/plugins/rib/rib_bestchange.go` (`isSRv6Ineligible`, `entrySRv6SID`, `srv6SIDFromResult`) | Enforces `RFC9252-5-1` semantic SID validity and merges transposed VPN label bits back into the 128-bit Service SID | Extends (reuses directly): `candidateSRv6SIDResolved` calls `entrySRv6SID` so reachability always checks the merged Service SID while `isSRv6Ineligible` stays unchanged |
| Per-peer/group/global RIB YANG config & transaction reload | `internal/component/bgp/plugins/rib/yang/ze-rib.yang` (`flowspec-redirect-check-fields`, `blackhole-honor-fields`), `config_tx.go` (`parseRIBConfigSections`, `installConfig`), `internal/component/bgp/configjson` (`ForEachPeer`, `LookupPeerConfig`) | Augments `/bgp:bgp`, `/bgp:bgp/bgp:group`, `/bgp:bgp/bgp:peer`, `/bgp:bgp/bgp:group/bgp:peer` without YANG defaults on inherited booleans, parses in `ConfigTx` verify, and reselects via `reselectAIGPRoutes` on apply/rollback | Extends: adds `srv6-sid-resolvability-fields` (`ignore` leaf and `family` list) following the same four-scope augmentation, dynamic-group lookup, and `ConfigTx` reload reselection pattern |
| Excluded-route visibility (`show bgp rib` + runtime doctor) | `internal/component/bgp/plugins/rib/rib_pipeline_bestfact.go` (`markNextHopFacts`, `writeNextHopIneligible`), `rib_next_hop_doctor.go` (`unresolvedNextHopTracker`, `doctorCheck`), `doctor_codes.go` | Annotates excluded received routes in `show bgp rib` and exposes bounded live exclusion counts in Stage 1 runtime doctor checks | Extends: adds `"srv6-sid-resolvable": false` / `"ineligible-reason": "srv6-sid-unresolvable"` row annotations and Stage 1 runtime check `bgp-rib-unresolved-srv6-sid` (`doctor-bgp-srv6-sid-unresolvable`) |
| Sysrib FIB-time SRv6 SID guard | `internal/component/sysrib/sysrib.go` (`resolvedFibEntry`, `srv6SIDResolvable`), `nhresolver.go` (`Resolve`) | Blocks FIB installation (`fibForbidden`) when a winning route's SRv6 Service SID does not resolve in Loc-RIB | Preserves as defense-in-depth (updates stale comment referencing this spec only); continues preventing broken SEG6 kernel/VPP installs even when local policy bypasses BGP pre-selection resolvability |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | `candidateSRv6SIDResolved` delegates to `nextHopResolved`, which uses `igpcost` in-process and the registered `route-metrics` RPC in forked mode, and `bgp-rib-unresolved-srv6-sid` runs via Stage 1 `DoctorChecks` / `OnDoctorCheck` on both transports. |
| `docs/architecture/plugin/rib-storage-design.md` | Mandatory FlowSpec authorization | `// Design:` of `rib_commands.go`, `rib_manual.go`, `rib_pipeline.go`, `rib_pipeline_bestfact.go`, `rib.go`; modified by this spec | "`flowspec-redirect-check { enabled false; }` at the bgp, group or peer level turns the check off for a session. The peer's value beats the group's, and the group's beats the bgp level's. When no level states the leaf, the check runs." | agrees | `srv6-sid-resolvability` follows the exact same four-scope `ze-rib.yang` augmentation and precedence pattern (extended with per-family overrides) with strict enforcement when omitted at every level. |
| `docs/architecture/api/commands.md` | Overview | `// Design:` of `rib_pipeline.go`; anchored to `rib_commands.go`, `rib_pipeline.go` | "" | silent | No command path, verb, or pipe operator is added or changed; `show bgp rib` keeps its existing verb-first command path and pipe pipeline while adding optional JSON fields on excluded received rows. |
| `docs/architecture/route-selection.md` | Phase 2: Best-Path Selection (RFC 4271 Section 9.1.2) | `// Design:` of `rib_next_hop.go`, `rib_next_hop_doctor.go`; anchored to `rib_commands.go`, `rib_next_hop.go`, `rib_pipeline.go`, `rib_pipeline_bestfact.go`, `rib_next_hop_doctor.go`, `rib.go`; modified by this spec | "Before ranking, the RIB excludes an address next hop that cannot resolve. For CIDR routes it also resolves against the routing table that would result from installing that candidate." | agrees | Extends this exact pre-ranking forwarding admission path (`candidateSRv6SIDResolved` beside `candidateNextHopResolved`), reuses `nextHopResolved` / `ResolveProspective` / `route-metrics`, preserves `candidateAdmissionRecovery`, and mirrors `show bgp rib` and runtime doctor visibility. |
| `docs/architecture/update-building.md` | Three Paths for UPDATE Messages | anchored to `rib_commands.go` | "" | silent | Covers BGP UPDATE wire construction, buffer pools, and forwarding paths; this spec modifies no UPDATE wire encoding or buffer-pool lifecycle. |
| `ai/doors/bgp-plugins.md` | Boundaries | door covering `internal/component/bgp/plugins/rib/*` | "A doctor check takes one route, never both." | agrees | Declares `bgp-rib-unresolved-srv6-sid` exclusively in Stage 1 `sdk.Registration.DoctorChecks` answered by `OnDoctorCheck` (never as a `runtime`-phase `registry.DoctorCheckDef`), and reaches the engine only via `route-metrics` RPC in forked mode. |
| `docs/architecture/api/birdwatcher-compat.md` | 3. Endpoints | anchored to `rib_pipeline.go` | "" | silent | Specifies the Looking Glass HTTP/JSON API for Alice-LG; silent on `bgp-rib` SRv6 Service SID candidacy filtering, and additive exclusion fields on non-best received rows leave birdwatcher transforms unaffected. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | `// Design:` of `doctor_codes.go`; anchored to `rib_next_hop_doctor.go`, `doctor_codes.go`, `rib.go` | "Each check runs once per answer, through one route. A plugin's runtime check reads state that lives in the plugin's own process, so its one route is the doctor callback, which reaches a forked plugin as well as an internal one." | agrees | `bgp-rib-unresolved-srv6-sid` is declared only in Stage 1 `DoctorChecks`, answered by `doctorCheck` from a bounded in-memory tracker without taking `peerMu` or making engine RPCs, and registers `doctor-bgp-srv6-sid-unresolvable` in `doctor_codes.go`. |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `rib.go` | "" | silent | Defines plugin IPC framing, 5-stage startup, and engine RPC schemas; this spec changes no RPC message or wire framing, reusing `route-metrics` and Stage 1 `DoctorChecks` unchanged. |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to `rib.go` | "" | silent | Covers BGP FSM Established state transitions, keepalives, and timers; silent on `bgp-rib` best-path candidate resolvability. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Consequences | anchored to `rib.go`, `sysrib.go` | "New Ze Prometheus metrics should follow the registry-backed push model unless this decision is explicitly reopened." | agrees | Registers `ze_rib_srv6_sid_unresolvable_exclusions_total` through `metrics.Registry` in `SetMetricsRegistry` (`rib.go`) following the push-model registry and `ze_<subsystem>_<thing>_<unit>` naming convention. |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to `rib.go`, `config_tx.go` | "**API = Policy** - RIB storage, best-path selection, route refresh, GR state" | agrees | Places RFC 9252 Section 5/6 SRv6 Service SID pre-selection resolvability and local-policy bypass in the `bgp-rib` plugin rather than the BGP reactor engine. |
| `docs/architecture/rib/forward-handle.md` | The decisions | anchored to `rib.go` | "" | silent | Specifies zero-copy `ForwardHandle` refcounting and fast-path state tracking on `Change.Forward`; silent on pre-ranking SRv6 Service SID resolvability. |
| `docs/architecture/config/transaction-protocol.md` | Phase 1: Verify | `// Design:` of `config_tx.go` | "Verification is non-destructive: no state changes, no side effects." | agrees | `parseSRv6ResolvabilityConfig` validates `srv6-sid-resolvability` cleanly into a candidate `ribConfig` during verify with zero mutation; apply and rollback install atomically and re-run `reselectAIGPRoutes` only when rules change. |
| `docs/architecture/core-design.md` | System RIB | `// Design:` of `sysrib.go`; anchored to `sysrib.go` | "The `rib` plugin consumes the daemon Loc-RIB's authoritative selection. The Loc-RIB applies import policy and selects across protocols once; the consumer resolves next hops and builds forwarding changes from those selected values." | agrees | Keeps `sysrib`'s authoritative selection consumption and `resolvedFibEntry` FIB-time SRv6 SID guard unchanged as defense-in-depth, updating only the stale comment referencing this spec. |
| `docs/architecture/api/text-format.md` | Current Format | anchored to `sysrib.go` | "" | silent | Specifies BGP text event formatting; silent on SRv6 Service SID pre-selection resolvability and `sysrib` comments. |
| `docs/architecture/rib/unified-locrib.md` | Owner requirement: internal and external system RIB | anchored to `sysrib.go` | "Both execution modes must preserve distances, next-hop resolution, ECMP, candidate inspection, configuration changes, FIB output, and recovery." | agrees | Preserves `sysrib`'s internal and external `route_view` forwarding behavior unchanged and ensures `bgp-rib` itself works identically in both internal and forked placements. |
| `docs/architecture/wire/nlri-evpn.md` | Common Header | anchored to `sysrib.go` | "" | silent | Specifies RFC 7432 EVPN NLRI wire framing; this spec changes no EVPN wire codec and checks reachability on already-decoded `entrySRv6SID` addresses. |
| `ai/doors/forwarding.md` | Where it sits | door covering `internal/component/sysrib/sysrib.go` | "Protocols insert paths into the shared Loc-RIB, which applies import policy and selects the system route." | agrees | Excluding unresolvable SRv6 BGP paths before best-path prevents unprogrammable BGP winners from entering Loc-RIB and shadowing lower-AD static backups, while preserving `sysrib`'s `resolvedFibEntry` guard. |
| `ai/doors/le-tooling.md` | Boundaries | door covering `test/plugin/srv6-bestpath-*.ci` | "**A `.ci` that needs a Linux kernel feature carries `option=needs-linux`** and runs in the QEMU VM" | agrees | `test/plugin/srv6-bestpath-static-wins.ci` programs real Linux kernel FIB routes and carries `option=needs-linux:caps=net-admin` so it executes in the privileged QEMU guest. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds no new subcommand or selector word; operators inspect excluded SRv6 routes with existing pipe operations (`show bgp rib \| match srv6-sid-unresolvable`). |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Adds no new pipe override and preserves `show bgp rib`'s existing pipe data shape while adding optional exclusion fields to ineligible received rows. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Changes no pipe operation behavior; `show bgp rib` produces its received rows and pipes continue to filter or format that answer unchanged. |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Does not touch FlowSpec MP_REACH next-hop emission or `flowspec-redirect-next-hop`; follows the same principle of explicit opt-in (`srv6-sid-resolvability { ignore true; }`, strict default). |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no new engine operation and modifies no plugin authorization profile; reuses the already-declared `route-metrics` operation. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Does not touch engine-to-plugin event delivery or queue overflow handling; Loc-RIB reselection runs inside `bgp-rib`'s existing coalescing `runAIGPSelection` worker. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Control-plane `bgp-rib` SRv6 Service SID resolvability and `srv6-sid-resolvability` config are data-plane agnostic (apply equally to Linux kernel and VPP FIB setups) and do not relax any kernel-on-VPP check. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Adds no backend-specific mismatch and preserves unified validation across start, commit, reload, and `ze config validate` (subject to the 2026-11-26 start-time supersession below). |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Touches no firewall YANG annotations or VPP firewall verifiers. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Introduces no data-plane selector and keeps existing `fib kernel` / `fib vpp` naming untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Adds no egress next-hop rewrite knob; `srv6-sid-resolvability` controls ingress Phase 2 best-path candidacy in `bgp-rib` per Thomas's 2026-11-24 ruling, not egress next-hop rewriting. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Touches no export filter chain or `policychain.Sets` next-hop intent. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Touches no compiled `policychain.Chain` steps or `FilterInsertion` registrations. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Does not touch export chain next-hop folding or `wireOverride` provenance. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Does not touch `policy { modify }` next-hop parsing or import/export chain validation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Does not touch egress `next-hop:auto` or locally originated next-hop preservation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Inserts no automatic filter-chain member and leaves the implied-filter seam untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Adds no startup exit condition; an invalid `srv6-sid-resolvability` edit is refused at config verify/reload while running sessions keep their current selection, and runtime SRv6 SID exclusions report non-fatal warnings under `doctor-bgp-srv6-sid-unresolvable`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Control-plane SRv6 Service SID resolvability works on both Linux kernel and VPP setups and does not alter kernel-on-VPP start or validation rules. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- **BGP UPDATE**: wire bytes carrying a Prefix-SID attribute (code 40) with SRv6 L3 Service TLV (type 5) or SRv6 L2 Service TLV (type 6), received from any peer in any negotiated address family (IPv4/IPv6 unicast, IPv4/IPv6 VPN, EVPN), interned into `pool.OtherAttrs` on the peer's Adj-RIB-In entry.
- **Loc-RIB routing change**: `locrib.Change` (Add/Update/Remove) for a covering IPv6 locator prefix (internal mode via `loc.OnChange` in `runAIGPSelection`) or engine routing revision advance (forked mode via 1s `RouteMetrics` poll in `runRemoteMetrics`).
- **Configuration commit / reload / rollback**: `bgp` config subtree delivered through `ConfigTx` carrying `srv6-sid-resolvability { ignore ...; family <name> { ignore ...; } }` at `bgp`, `group`, `peer`, or `group/peer` scope.

### Transformation Path
1. UPDATE is parsed and validated by RFC 7606 / EBGP `accept-srv6-prefix-sid`; the Prefix-SID bytes are interned into `pool.OtherAttrs` and stored in the peer's Adj-RIB-In (`r.bgpPeers[owner]`).
2. `checkRouteBestChange` acquires the `bestPrev` shard lock, then `r.peerMu.RLock()`, and calls `gatherPrefixCandidatesLocked` (CIDR) or `gatherKeyCandidatesLocked` (non-CIDR), both with `candidateAdmissionForwarding`.
3. For each stored path, `candidateAdmitted` enforces `SourcePolicyEligible`, `isSRv6Ineligible` (`RFC9252-5-1` semantic validity), and `isSelfNextHop` (RFC 4271 Section 5.1.3).
4. After `extractCandidate`, forwarding admission (`requiresResolution() == true`) checks `candidateNextHopResolved` (RFC 4271 Section 9.1.2 `NEXT_HOP`), then `candidateSRv6SIDResolved` (`RFC9252-5-2`):
   - Fast-path returns true when `!entry.GetBundle().HasOtherAttrs()`.
   - Reconstructs the merged 128-bit Service SID via `entrySRv6SID` (`srv6SIDFromResult` + VPN transposition label merge). Returns true if no valid Service SID is present.
   - Checks `srv6SIDResolvabilityIgnored(owner, peer, fam)` against the atomically stored `srv6ResolvabilityCfg` (most-specific scope wins: peer family > peer > group family > group > global family > global > false). If ignored, admits the candidate.
   - Evaluates reachability via `nextHopResolved(fam, prefix, sid, isEBGP, c.MED, c.AIGP, c.HasAIGP)` (prospective CIDR preview or `igpDistance` in internal/forked mode).
   - If unresolved: drops the candidate before Phase 2 ranking, increments `ze_rib_srv6_sid_unresolvable_exclusions_total`, and records the exclusion in the bounded `unresolvedSRv6SIDs` tracker for runtime doctor reporting.
5. `SelectMultipath` / `SelectBest` runs over surviving candidates only. The winner is mirrored to Loc-RIB (CIDR) and published to `best-change` and downstream peers (`advertise` / `selectedDispatch`). An excluded SRv6 path stays retained in Adj-RIB-In and is never selected or advertised.
6. When a covering IPv6 locator prefix is added, modified, or removed in the Loc-RIB (or when `srv6-sid-resolvability` changes on config commit/rollback), the existing worker (`runAIGPSelection` / `runRemoteMetrics` / `installConfig`) invokes `reselectAIGPRoutes`, which re-runs step 2 for every retained Adj-RIB-In route (admitted or excluded) outside all locks.
7. `sysrib`'s `resolvedFibEntry` keeps its existing `srv6SIDResolvable` guard as defense-in-depth before FIB installation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BGP RIB (in-process) ↔ Loc-RIB | Reads via `igpcost.ResolveProspective` / `igpcost.Lookup`; change notifications via existing `loc.OnChange` in `runAIGPSelection` | No |
| BGP RIB (forked) ↔ Engine | Reachability via existing `route-metrics` RPC (`opRouteMetrics`); revision invalidation via existing `runRemoteMetrics` | No |
| Config transaction ↔ BGP RIB | `ConfigTx` verify/apply/rollback over `bgp` config section (`config_tx.go`) | No |
| BGP RIB ↔ Operator (`show bgp rib`, `show doctor`, Prometheus) | JSON row annotations (`srv6-sid-resolvable`, `ineligible-reason`), Stage 1 runtime doctor check `bgp-rib-unresolved-srv6-sid`, counter `ze_rib_srv6_sid_unresolvable_exclusions_total` | No |
| BGP RIB → Peers / Loc-RIB / sysrib | Winning selection only (`checkRouteBestChange` -> `mirrorToLocRIB` / `commitReceivedAdvertise` / `publishBestChanges`) | No |

### Changed-Wire Census

No BGP protocol wire format, plugin RPC wire schema (`pkg/plugin/rpc`), or internal event payload changes in this spec: Prefix-SID attribute 40 framing, `route-metrics` RPC payloads, and `(bgp-rib, best-change)` events remain byte-identical. Two operator-facing structured surfaces are extended additively: (1) `show bgp rib` received JSON rows gain optional fields `"srv6-sid-resolvable": false` and `"ineligible-reason": "srv6-sid-unresolvable"` on routes excluded for an unresolvable SRv6 Service SID (omitted for all other routes, matching the additive shape of `"next-hop-resolvable"` / `"ineligible-reason"`), and (2) the BGP YANG configuration schema gains the optional `srv6-sid-resolvability` container.

**Search evidence:** Searched `internal/`, `docs/`, and `test/` for `ineligible-reason`, `next-hop-resolvable`, and `flowspec-redirect-check` (positive controls: `internal/component/bgp/plugins/rib/rib_pipeline_bestfact.go:27-29`, `rib_next_hop_visibility_test.go:87-90`, `test/plugin/rib-unresolved-next-hop-visibility.ci:79-88`, and `internal/component/bgp/plugins/rib/yang/ze-rib.yang:170-213`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `show bgp rib` received JSON row | writer | `internal/component/bgp/plugins/rib/rib_pipeline.go` (`serializeRouteItem`) and `rib_pipeline_bestfact.go` (`writeNextHopIneligible`, new `writeSRv6SIDIneligible`) | `grep` confirmed `rib_pipeline_bestfact.go:27-29,148-154` is the sole writer of `next-hop-resolvable` and `ineligible-reason` | Additive change: emit `"srv6-sid-resolvable": false` and `"ineligible-reason": "srv6-sid-unresolvable"` only when a received row fails SRv6 SID resolvability; preserve all existing keys unchanged |
| `show bgp rib` received JSON row | reader | CLI/API/Web/MCP/Looking-Glass consumers and pipe filters (`show bgp rib \| match ...`), plus functional tests (`test/plugin/rib-unresolved-next-hop-visibility.ci`) | `grep` confirmed no reader enumerates a closed set of `ineligible-reason` strings; existing tests match `"next-hop-unresolvable"` and verify omission on resolvable rows | Preserve: resolvable and non-SRv6 rows omit both keys, so all existing readers and tests remain compatible |
| BGP YANG config (`ze-rib.yang`) | writer | Operators, CLI editor, web UI, config files (`bgp { srv6-sid-resolvability { ... } }`) | `grep` for `flowspec-redirect-check` confirmed YANG schema in `internal/component/bgp/plugins/rib/yang/ze-rib.yang` generates editor/web/site schema automatically via `./le yang glue update` | Additive change: absent container defaults to strict enforcement; existing configs remain 100% valid with no migration |
| BGP YANG config (`ze-rib.yang`) | reader | `internal/component/bgp/plugins/rib/config_tx.go` (`parseRIBConfigSections`) -> new `rib_srv6_resolvability_config.go` (`parseSRv6ResolvabilityConfig`) | `config_tx.go:91-131` reads `configRootBGP` through `configjson.ParseBGPSubtree` and `configjson.ForEachPeer` | Change: add parser and `installConfig` comparison + `reselectAIGPRoutes` trigger |

### Integration Points
- `gatherPrefixCandidatesForLocked` and `gatherKeyCandidatesForLocked` (`internal/component/bgp/plugins/rib/rib_commands.go`) - call `candidateSRv6SIDResolved` beside `candidateNextHopResolved` under `requiresResolution`.
- `appendManualCandidatesLocked` (`internal/component/bgp/plugins/rib/rib_manual.go`) - call `candidateSRv6SIDResolved` beside `candidateNextHopResolved`.
- `nextHopResolved` / `nextHopResolvedCached` (`internal/component/bgp/plugins/rib/rib_next_hop.go`) - shared RFC 4271 resolution engine reused for Service SIDs in internal and forked modes.
- `entrySRv6SID` / `srv6SIDFromResult` (`internal/component/bgp/plugins/rib/rib_bestchange.go`) - reconstructs the merged Service SID (including VPN transposition).
- `runAIGPSelection` / `reselectAIGPRoutes` (`internal/component/bgp/plugins/rib/rib_aigp.go`) and `runRemoteMetrics` (`rib_remote.go`) - existing Loc-RIB / revision reselection workers that automatically re-evaluate retained SRv6 routes.
- `markNextHopFacts` (`internal/component/bgp/plugins/rib/rib_pipeline_bestfact.go`) and `serializeRouteItem` (`rib_pipeline.go`) - `show bgp rib` operator visibility.
- `doctorCheck` (`internal/component/bgp/plugins/rib/rib_next_hop_doctor.go`), `doctor_codes.go`, and `runRIBPlugin` (`rib.go`) - Stage 1 runtime doctor check `bgp-rib-unresolved-srv6-sid`.
- `parseRIBConfigSections` / `installConfig` (`internal/component/bgp/plugins/rib/config_tx.go`) and `yang/ze-rib.yang` - local-policy bypass configuration and reload/rollback reselection.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Pending implementation |
| No unintended coupling (components stay isolated) | No | Pending implementation (`./le arch tier check`, `./le plugin boundary check`) |
| No duplicated functionality (extends existing, does not recreate) | No | Pending implementation (reuses `igpcost`, `nextHopResolved`, `runAIGPSelection`, `runRemoteMetrics`, `entrySRv6SID`, `ConfigTx`) |
| Zero-copy preserved where applicable (refs, not copies) | No | Pending implementation (fast-path skips `!HasOtherAttrs()`; non-CIDR reuses stack-backed `nlriBuf`) |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | Pending implementation (YANG augments via `ze-rib.yang`, doctor check via Stage 1 `DoctorChecks`, code via `diagnostic.Register`, family names validated via registered `parseFamily`) |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | Pending implementation (`./le arch enumeration check`, `./le config claims`, `./le yang glue check`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `igpcost.Resolve` / `igpcost.ResolveProspective` (and engine `opRouteMetrics` in forked mode) enforce the exact reachability semantics RFC 9252 Section 5 ("evaluated as per [RFC4271]") requires for IPv6 Service SIDs and agree with `sysrib`'s `nhResolver.Resolve` on missing covers, discard routes, connected/IGP/recursive-static/BGP chains, self-loops, and 8-hop depth limit | `internal/core/rib/igpcost/igpcost.go` (`resolve`), `prospective.go` (`ResolveProspective`), `internal/component/sysrib/nhresolver.go` (`Resolve`), `internal/component/plugin/server/dispatch_route_metrics.go` (`opRouteMetrics`) | `bgp-rib` and `sysrib` could disagree on whether a Service SID resolves, either admitting a route the FIB refuses or excluding a valid route | Unit test comparing `candidateSRv6SIDResolved` and `sysrib.srv6SIDResolvable` over the same Loc-RIB cases (direct, recursive, discard, loop, depth 8 vs 9, missing) | unvalidated |
| A-2 | The locator bits of a partial SID can differ from the merged SID, so the check must run on `entrySRv6SID` (`srv6SIDFromResult`) output rather than `pool.ExtractSRv6SIDFull` | `rfc/full/rfc9252.txt` Section 3.2.1 bounds transposition offset+length only against LBL+LNL+FL+AL, so the window can overlap locator bits | Checking the partial SID would resolve the wrong IPv6 prefix for a transposed VPN route and admit or reject the wrong path | Unit test `TestResolvabilityUsesMergedTransposedSID` with a transposition window overlapping the locator | unvalidated |
| A-3 | `runAIGPSelection` (internal `loc.OnChange`) and `runRemoteMetrics` (forked 1s revision poll) re-evaluate EVERY retained Adj-RIB-In route -- including non-CIDR VPN/EVPN routes and routes currently excluded from `bestPrev` -- whenever an IPv6 locator route is added, modified, or removed in the Loc-RIB | `internal/component/bgp/plugins/rib/rib_aigp.go:60-98` (`reselectAIGPRoutes` iterates `r.bgpPeers` and `r.ribInPool[bgpProtocolID]` directly, not `bestPrev`) and `rib_remote.go:130-146` | An excluded SRv6 route would fail to recover when its locator route appears | Unit tests `TestLocRIBChangeReevaluatesSRv6Prefix` and `TestLocRIBRemovalReevaluatesSRv6Prefix`, plus forked `route-metrics` recovery test | unvalidated |
| A-4 | For an IPv4 unicast route carrying an IPv6 SRv6 Service SID, `igpcost.ResolveProspective` checks the IPv6 SID against the unmodified IPv6 Loc-RIB without interference from the IPv4 overlay, while for an IPv6 unicast route carrying an IPv6 SRv6 Service SID it also catches prospective self-capture | `internal/core/rib/igpcost/prospective.go:42-49` (`if fam != view.fam { return path, prefix, found }`) | IPv4-over-SRv6 routes could be mishandled by the prospective overlay | Unit tests covering both IPv4 unicast over SRv6 and IPv6 unicast self-capturing SRv6 SID | unvalidated |
| A-5 | Existing `RFC9252-5-1` tests (`rfc9252_srv6_ineligible_test.go`, `rfc9252_srv6_admission_test.go`, `rfc9252_srv6_transposition_test.go`) continue passing untouched because they test `isSRv6Ineligible`, `candidateAdmitted`, and `entrySRv6SID` directly, which remain unchanged | Read all three test files on HEAD | The 5-1 proofs would break or need rewording | Run scoped `go test -race -tags ze_bgp ./internal/component/bgp/plugins/rib/` before and after | unvalidated |
| A-6 | Non-transposed EVPN SRv6 Service SIDs (`TransposLen == 0`) reconstruct cleanly via `entrySRv6SID` -> `srv6SIDFromResult`, so Section 6 EVPN resolvability works immediately while EVPN label transposition remains separately owned by `plan/release/fixes/spec-srv6-evpn-label-width.md` | `internal/component/bgp/plugins/rib/rib_bestchange.go:1542-1555` (`if !result.HasTranspos { return result.SID }`) and `internal/core/bgp/nlri/nlrisplit/transposition.go:35-73` | EVPN paths would bypass `RFC9252-5-2` | Unit test `TestEVPNUnresolvableSRv6SIDIneligible` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Lock-order inversion or re-entrancy when Loc-RIB changes trigger SRv6 re-evaluation | Deadlock under concurrent UPDATE and static-route churn; `-race` timeout | Reuses existing `runAIGPSelection` (`OnChange` sends to size-1 channel only, zero locks) and `reselectAIGPRoutes` (snapshots keys under `peerMu.RLock`, releases before `checkBestPathChange`); `unresolvedSRv6SIDs` uses a leaf mutex like `unresolvedNextHopTracker` |
| R-2 | Unbounded memory growth in the runtime doctor exclusion tracker under a full VPN table with unreachable locators | High heap growth when a PE locator flaps | Strictly cap `unresolvedSRv6SIDs` at `maxTrackedUnresolvedNextHops` (4096) with the same fixed 4096-bit deduplication overflow filter and per-owner / withdrawal cleanup used by `unresolvedNextHopTracker` |
| R-3 | Forked `bgp-rib` fails silently or blocks `show bgp rib` during an engine `route-metrics` outage | `show bgp rib` hangs or admits unresolvable SRv6 routes in forked mode | Reuse `nextHopResolved` (5s timeout, fail-closed, coalesced `doctor-bgp-next-hop-metrics`) on admission and `nextHopResolvedCached` (zero RPC) in `show bgp rib` during `metricFailureActive()` |
| R-4 | Operator sets `srv6-sid-resolvability { ignore true; }` expecting local Linux/VPP SEG6 installation without a covering locator route | Route wins BGP best-path and is advertised to peers, while `sysrib` keeps withholding single-SID SEG6 encap because Ze has no SR Policy data-plane steering yet | Document clearly in YANG `description`, `docs/features/srv6.md`, and `docs/guide/configuration.md` that `ignore true` bypasses BGP best-path exclusion (for RR/transit propagation or alternate steering) while `sysrib` keeps its single-SID FIB guard as defense-in-depth |
| R-5 | Per-peer / per-family inheritance mis-resolves dynamic peer groups or fails to reselect on config reload/rollback | Dynamic group peer ignores group rule, or toggling `ignore` leaves stale best path until next UPDATE | Reuse `configjson.ForEachPeer`, `configjson.KeyFor`, `configjson.LookupPeerConfig(..., r.peerGroupName(owner))`, and include `!sameSRv6ResolvabilityRules` in `config_tx.go`'s `rulesChanged` check so both commit and rollback call `reselectAIGPRoutes` |
| R-6 | Excluding an unresolvable SRv6 path withdraws a route a peer previously received | Interop scenario observes a withdraw when locator disappears | That is the required RFC 9252 Section 5 behavior; the FRR interop scenario asserts both suppression while unreachable and advertisement once the locator resolves |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Best-path selection for every family carrying an SRv6 Service SID (IPv4/IPv6 unicast, VPNv4/VPNv6, EVPN). A false "not resolvable" withdraws working SRv6 routes from the FIB and from downstream peers; a mis-parsed `srv6-sid-resolvability` rule could either reject valid configs or fail to enforce strict default resolvability |
| How is it reverted? | Single commit revert. The new `srv6-sid-resolvability` YANG container is optional and defaults to strict behavior when absent, so no config migration or state format change is needed |
| Who else touches this path? | `plan/release/fixes/spec-srv6-evpn-label-width.md`, `plan/release/fixes/spec-vpp-srv6-service-policy.md`, and `plan/post-release/features/spec-srv6-labeled-unicast.md` touch SRv6 extraction/FIB paths; none conflict with `rib_next_hop.go` or `ze-rib.yang` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE carrying Prefix-SID SRv6 Service TLV whose locator has no covering route | → | `gatherPrefixCandidatesForLocked` / `gatherKeyCandidatesForLocked` -> `candidateSRv6SIDResolved` -> `nextHopResolved` | `TestGatherCandidatesSkipsUnresolvableSRv6SID` |
| Covering IPv6 locator route added/removed in Loc-RIB | → | `runAIGPSelection` (`loc.OnChange`) -> `reselectAIGPRoutes` -> `checkBestPathChange` | `TestLocRIBChangeReevaluatesSRv6Prefix` |
| Forked `bgp-rib` resolving SRv6 Service SID over engine `route-metrics` RPC | → | `candidateSRv6SIDResolved` -> `remoteRIB.igpDistance` / `prospectiveDistance` + `runRemoteMetrics` | `TestForkedRIBSRv6SIDResolvability` |
| Operator configures `bgp { srv6-sid-resolvability { ignore true; family ... } }` and reloads/rolls back | → | `config_tx.go` (`parseRIBConfigSections` -> `parseSRv6ResolvabilityConfig` -> `installConfig` -> `reselectAIGPRoutes`) -> `srv6SIDResolvabilityIgnored` | `TestSRv6ResolvabilityBypassConfigAndReload` |
| Real daemon receives unresolvable SRv6 BGP route alongside static backup and programs kernel FIB | → | Full chain (BGP peer -> `bgp-rib` exclusion -> Loc-RIB static wins -> `sysrib` -> `fib-kernel`) | `test/plugin/srv6-bestpath-static-wins.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A received BGP path carries a valid SRv6 Service SID whose 128-bit address has no resolving route in the Loc-RIB (or resolves only via a discard route, self-loop, or chain > 8 hops) | `candidateSRv6SIDResolved` excludes the path before Phase 2 best-path ranking, keeps the path retained in Adj-RIB-In, increments `ze_rib_srv6_sid_unresolvable_exclusions_total`, annotates it in `show bgp rib` with `"srv6-sid-resolvable": false` and `"ineligible-reason": "srv6-sid-unresolvable"`, and surfaces it via runtime doctor check `bgp-rib-unresolved-srv6-sid` (`doctor-bgp-srv6-sid-unresolvable`) | Paths carrying no SRv6 Service SID, or paths whose peer/family explicitly configured `srv6-sid-resolvability { ignore true; }`, remain candidates (local policy per RFC 9252 §5 MAY clause) |
| AC-2 | A prefix has an unresolvable SRv6 path with higher `LOCAL_PREF` (200) and a second reachable path (non-SRv6 or resolvable SRv6) with worse `LOCAL_PREF` (100) | The reachable runner-up wins BGP best-path, is installed in Loc-RIB, and is advertised to downstream peers; the excluded SRv6 path is not advertised | None for the excluded path; only the reachable runner-up wins and is advertised |
| AC-3 | A covering IPv6 locator route for a previously unresolvable retained SRv6 Service SID is inserted into the Loc-RIB (with no new BGP UPDATE) | `runAIGPSelection` (internal) or `runRemoteMetrics` (forked) triggers `reselectAIGPRoutes`; the retained SRv6 path becomes eligible, wins best-path if preferred, clears its `show bgp rib` ineligibility keys and doctor entry, and is published to Loc-RIB and peers | None; recovery must succeed from full exclusion (no prior `bestPrev` record) without any BGP UPDATE |
| AC-4 | The covering IPv6 locator route for a winning SRv6 Service SID is removed from the Loc-RIB (with no new BGP UPDATE) | `runAIGPSelection` / `runRemoteMetrics` re-evaluates the prefix; the SRv6 path loses best-path, the next-best reachable path takes over (or the prefix is withdrawn from Loc-RIB and peers if none remains), and the SRv6 path stays retained in Adj-RIB-In marked `srv6-sid-unresolvable` | The path stays retained in Adj-RIB-In so later locator restoration recovers it (AC-3); only best-path candidacy and selection are withdrawn |
| AC-5 | A received BGP path carries a valid SRv6 Service SID whose address resolves in the Loc-RIB (directly connected, IGP, static, or recursive chain <= 8 hops) | The path is admitted to best-path selection, wins normally, omits `srv6-sid-resolvable` and `ineligible-reason` in `show bgp rib`, and installs with its reconstructed Service SID | None; resolvable SRv6 paths behave identically to normal eligible paths |
| AC-6 | `bgp-rib` runs in forked subprocess mode (`process separate`) with `route-metrics` RPC; covering route is absent, then added, then engine lookup fails | Forked `bgp-rib` resolves the Service SID via `route-metrics`, excludes while absent, promotes on revision change after locator insertion, fails closed (`Resolved: false`) on RPC error, and serves `show bgp rib` from cache during an active outage without blocking | Cached resolvable entries during an outage stay readable fail-closed via `entrySRv6SIDResolvedCached`, matching `entryNextHopResolvedCached` |
| AC-7 | Real daemon with `fib-kernel` receives a BGP path carrying an unresolvable SRv6 Service SID and also holds a static backup route for the same prefix at a worse administrative distance | Because the SRv6 BGP path is excluded before best-path, it never enters the Loc-RIB; the static backup wins Loc-RIB arbitration and is programmed into the Linux kernel FIB (prefix is reachable, not dark) | None; the static backup must be programmed in the kernel routing table |
| AC-8 | A VPNv4 or VPNv6 path carries a transposed SRv6 Service SID whose transposition window overlaps the locator bits (so partial SID and merged SID fall under different IPv6 prefixes) | `candidateSRv6SIDResolved` resolves the MERGED SID reconstructed by `entrySRv6SID` / `srv6SIDFromResult`, admitting the path iff the merged SID has a covering route (even when the partial SID does or does not) | None; the partial untransposed SID must never decide resolvability |
| AC-9 | A path carries no Prefix-SID attribute, OR carries SRv6 Service TLVs with no semantically valid SID (RFC 9252 Section 7) | Behavior is unchanged: the non-SRv6 path skips the SRv6 check with zero extra work; the semantically invalid SRv6 path stays excluded by `isSRv6Ineligible` (`RFC9252-5-1`) even when `srv6-sid-resolvability { ignore true; }` is configured | Non-SRv6 paths remain candidates subject only to standard RFC 4271 checks; semantically invalid SRv6 paths have zero permitted survivors |
| AC-10 | An EVPN path (`l2vpn/evpn`) carries a valid non-transposed SRv6 Service SID (`TransposLen == 0`) whose address does not resolve in the Loc-RIB | `gatherKeyCandidatesForLocked` excludes the EVPN path before best-path selection (RFC 9252 Section 6), and admits it once a covering IPv6 route resolves the SID | Paths under peers/families with explicit `srv6-sid-resolvability { ignore true; }` remain permitted per AC-11 |
| AC-11 | Operator configures `srv6-sid-resolvability` (`ignore true|false` and/or `family <name> { ignore true|false }`) at `bgp`, `group` (including dynamic group), `peer`, or `group/peer` scope, commits, reloads, and rolls back | Most-specific scope wins (`peer family` > `peer` > `group family` > `group` > `bgp family` > `bgp` > strict default `false`). When resolved `ignore` is `true`, an unreachable SRv6 Service SID is admitted to BGP best-path and propagated (while `NEXT_HOP` resolvability and `RFC9252-5-1` semantic validity remain enforced); invalid booleans or unregistered family names are refused at verify/validate with actionable errors; commit and rollback trigger `reselectAIGPRoutes` immediately | `sysrib`'s `resolvedFibEntry` FIB guard remains active as defense-in-depth when no alternate data-plane steering backend exists; only BGP pre-selection resolvability is bypassed |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Receives a VPN/unicast route from a PE whose SRv6 locator is unreachable, alongside a reachable fallback peer | wire -> Adj-RIB-In -> `candidateSRv6SIDResolved` excludes SRv6 path -> runner-up wins -> Loc-RIB -> advertised to peers; `show bgp rib` and `show doctor` explain why the SRv6 path was excluded | `test/plugin/srv6-bestpath-unresolvable.ci` |
| 2 | Commits the missing IPv6 locator route (and later withdraws it), expecting automatic failover without BGP flap | static/IGP route -> Loc-RIB `OnChange` -> `runAIGPSelection` -> `reselectAIGPRoutes` -> `checkBestPathChange` -> Loc-RIB & peers | `test/plugin/srv6-bestpath-resolvable.ci` |
| 3 | Keeps a static backup for a prefix also advertised with an unreachable SRv6 Service SID | static config -> Loc-RIB; BGP SRv6 path excluded before best-path -> static wins arbitration -> `sysrib` -> `fib-kernel` installs static kernel route | `test/plugin/srv6-bestpath-static-wins.ci` |
| 4 | Configures `srv6-sid-resolvability { ignore true; }` (or per-family `family ipv4/mpls-vpn { ignore true; }`) for a peer steered via alternate local policy, then toggles it back | YANG validate -> `ConfigTx` verify/apply -> `parseSRv6ResolvabilityConfig` -> `installConfig` -> `reselectAIGPRoutes` -> candidate admitted/excluded on commit and rollback | `test/plugin/srv6-bestpath-bypass-config.ci` |
| 5 | Peers with FRR advertising SRv6 L3VPN whose locator is unreachable, then brings the locator up | FRR UPDATE -> Ze excludes from best-path and sends no re-advertisement -> locator becomes reachable -> Ze selects and advertises to FRR | `bgp-srv6-resolvability-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGatherCandidatesSkipsUnresolvableSRv6SID` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | AC-1 (`RFC9252-5-2` negative): unresolvable Service SID excluded before Phase 2 ranking, retained in Adj-RIB-In, counter incremented | planned |
| `TestGatherCandidatesAdmitsResolvableSRv6SID` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | AC-5 (`RFC9252-5-2` positive): resolvable Service SID admitted and wins best-path | planned |
| `TestUnresolvableSRv6LosesToNonSRv6Path` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | AC-2: higher-LOCAL_PREF unresolvable SRv6 path loses to reachable lower-LOCAL_PREF runner-up; runner-up enters Loc-RIB and `advertise` | planned |
| `TestLocRIBChangeReevaluatesSRv6Prefix` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | AC-3: adding covering IPv6 locator route re-evaluates retained excluded SRv6 path via `runAIGPSelection` without new UPDATE | planned |
| `TestLocRIBRemovalReevaluatesSRv6Prefix` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | AC-4: removing covering IPv6 locator route withdraws winning SRv6 path or fails over to runner-up without new UPDATE | planned |
| `TestSRv6ResolvabilityProspectiveSelfCaptureAndCrossFamily` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | A-4, AC-1, AC-5: IPv6 unicast route whose prefix captures its own Service SID is excluded; IPv4 unicast over IPv6 SRv6 SID resolves cleanly against IPv6 Loc-RIB | planned |
| `TestResolvabilityUsesMergedTransposedSID` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | AC-8, A-2: VPNv4/VPNv6 transposition overlapping locator checks the merged SID, not the partial wire SID | planned |
| `TestEVPNUnresolvableSRv6SIDIneligible` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | AC-10, A-6: EVPN path with non-transposed unreachable Service SID is excluded and recovers when locator resolves | planned |
| `TestSRv6ResolvabilityRecursionDepthAndDiscard` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | A-1 & numeric boundary: 8-hop recursive chain resolves, 9-hop chain and discard/self-loop covers fail; agrees with `sysrib.srv6SIDResolvable` | planned |
| `TestForkedRIBSRv6SIDResolvability` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | AC-6, R-3: forked `route-metrics` RPC resolves, fails closed on RPC error, recovers on revision change, and serves `show bgp rib` from cache during outage | planned |
| `TestSRv6ResolvabilityBypassConfigAndReload` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | AC-11, AC-9, R-5: precedence (`peer family` > `peer` > `group family` > `group` > `bgp family` > `bgp` > strict default), dynamic groups, string/bool coercion, invalid value/family refusal, reload & rollback reselection, and `RFC9252-5-1` / `NEXT_HOP` never bypassed | planned |
| `TestSRv6UnresolvableVisibilityShowAndDoctor` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` | AC-1, R-2: `show bgp rib` emits `"srv6-sid-resolvable": false` + `"ineligible-reason": "srv6-sid-unresolvable"` (omits when resolvable or bypassed), `bgp-rib-unresolved-srv6-sid` doctor check reports samples and enforces 4096 cap + purge on withdraw/peer-down | planned |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Recursion depth of IPv6 locator resolution chain (`igpcost.resolve`) | 1-8 hops | 8 hops resolves (`Resolved: true`, candidate admitted) | N/A (0 hops = direct cover = 1 lookup) | 9 hops resolves to `Resolved: false` (candidate excluded) |
| Runtime doctor SRv6 exclusion tracker capacity (`maxTrackedUnresolvedNextHops`) | 1-4096 tracked entries | 4096 individual samples | N/A (0 entries = no doctor diagnostic) | 4097+ entries spill cleanly into bounded overflow counter without growing map |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `srv6-bestpath-unresolvable` | `test/plugin/srv6-bestpath-unresolvable.ci` | AC-1, AC-2: route with unreachable locator does not win, runner-up wins and is advertised, `show bgp rib` and `show doctor` report the exclusion | planned |
| `srv6-bestpath-resolvable` | `test/plugin/srv6-bestpath-resolvable.ci` | AC-3, AC-4: adding and removing covering IPv6 locator route promotes and demotes the SRv6 path with no BGP UPDATE | planned |
| `srv6-bestpath-static-wins` | `test/plugin/srv6-bestpath-static-wins.ci` | AC-7 (`option=needs-linux:caps=net-admin`): static backup route is programmed in the Linux kernel FIB when competing BGP path has an unreachable SRv6 Service SID | planned |
| `srv6-bestpath-bypass-config` | `test/plugin/srv6-bestpath-bypass-config.ci` | AC-11: per-peer and per-family `srv6-sid-resolvability { ignore true; }` bypasses BGP exclusion, and reload/rollback re-evaluates retained routes | planned |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-srv6-resolvability-frr` | `test/interop/scenarios/bgp-srv6-resolvability-frr` | FRR | FRR advertises an SRv6 L3VPN route whose locator Ze cannot reach; Ze excludes it from best-path and does not re-advertise it; once the locator becomes reachable, Ze selects and advertises it (sibling of `bgp-srv6-frr`) | planned |

## Files to Modify
- `internal/component/bgp/plugins/rib/rib_commands.go` - call `candidateSRv6SIDResolved` beside `candidateNextHopResolved` in `gatherPrefixCandidatesForLocked` and `gatherKeyCandidatesForLocked`, record SRv6 exclusions into `unresolvedSRv6SIDs`, and increment the exclusion counter
- `internal/component/bgp/plugins/rib/rib_next_hop.go` - add `candidateSRv6SIDResolved`, `entrySRv6SIDResolvedWithClass`, and `entrySRv6SIDResolvedCached` delegating merged-SID reachability to `nextHopResolved` / `nextHopResolvedCached` and honoring `srv6SIDResolvabilityIgnored`
- `internal/component/bgp/plugins/rib/rib_manual.go` - call `candidateSRv6SIDResolved` in `appendManualCandidatesLocked` beside `candidateNextHopResolved`
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - carry raw NLRI bytes on received items for non-CIDR SRv6 transposition reconstruction in `markNextHopFacts` and emit SRv6 ineligibility keys in `serializeRouteItem`
- `internal/component/bgp/plugins/rib/rib_pipeline_bestfact.go` - extend `markNextHopFacts` with `entrySRv6SIDResolvedForShow` and add `writeSRv6SIDIneligible` (`"srv6-sid-resolvable": false`, `"ineligible-reason": "srv6-sid-unresolvable"`)
- `internal/component/bgp/plugins/rib/rib_next_hop_doctor.go` - track SRv6 Service SID exclusions in a bounded tracker and answer Stage 1 runtime doctor check `bgp-rib-unresolved-srv6-sid`
- `internal/component/bgp/plugins/rib/doctor_codes.go` - register `doctor-bgp-srv6-sid-unresolvable` diagnostic code metadata
- `internal/component/bgp/plugins/rib/rib.go` - add `srv6ResolvabilityCfg` atomic pointer and `unresolvedSRv6SIDs` tracker on `RIBManager`, declare `bgp-rib-unresolved-srv6-sid` in Stage 1 `DoctorChecks`, and register `ze_rib_srv6_sid_unresolvable_exclusions_total` in `SetMetricsRegistry`
- `internal/component/bgp/plugins/rib/config_tx.go` - parse `srv6-sid-resolvability` in `parseRIBConfigSections` and detect rule changes in `installConfig` to call `reselectAIGPRoutes` on commit and rollback
- `internal/component/bgp/plugins/rib/yang/ze-rib.yang` - add `srv6-sid-resolvability-fields` grouping (`ignore` leaf and `family` list keyed by `zt:address-family`) augmented onto `/bgp:bgp`, `/bgp:bgp/bgp:group`, `/bgp:bgp/bgp:peer`, and `/bgp:bgp/bgp:group/bgp:peer` (regenerate `embed.go` and `register.go` via `./le yang glue update`)
- `internal/component/sysrib/sysrib.go` - update the stale `resolvedFibEntry` comment referencing this spec's old path to describe the FIB-time check as defense-in-depth (guard behavior unchanged)
- `docs/architecture/route-selection.md` - document SRv6 Service SID pre-selection resolvability, local-policy bypass, `show bgp rib` visibility, and runtime doctor reporting beside RFC 4271 next-hop admission
- `docs/architecture/plugin/rib-storage-design.md` - document `srv6-sid-resolvability` config extraction, reload reselection, and the SRv6 exclusion counter
- `docs/features/srv6.md` - update the Section 5 compliance row, data flow, and configuration reference for pre-best-path SRv6 Service SID resolvability and `srv6-sid-resolvability`
- `docs/guide/configuration.md` - document the `srv6-sid-resolvability` container (`ignore` and `family`) under BGP configuration
- `rfc/short/rfc9252.md` - remove `{gap}` on `RFC9252-5-2` (regenerate `rfc/requirements/rfc9252.md` and `docs/features/rfc-status.md` via `./le rfc update`)

## Files to Create
- `internal/component/bgp/plugins/rib/rib_srv6_resolvability_config.go` - parser, precedence resolver (`peer family` > `peer` > `group family` > `group` > `bgp family` > `bgp` > strict default), and dynamic-group lookup for `srv6-sid-resolvability`
- `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` - unit tests for AC-1..AC-11 carrying the `RFC9252-5-2` positive and negative tags
- `test/plugin/srv6-bestpath-unresolvable.ci` - functional test for AC-1, AC-2, and operator visibility
- `test/plugin/srv6-bestpath-resolvable.ci` - functional test for AC-3 and AC-4 (locator add/remove recovery without UPDATE)
- `test/plugin/srv6-bestpath-static-wins.ci` - Linux kernel FIB functional test (`option=needs-linux:caps=net-admin`) for AC-7
- `test/plugin/srv6-bestpath-bypass-config.ci` - functional test for AC-11 (per-peer/per-family bypass, reload, and rollback)
- `test/interop/scenarios/bgp-srv6-resolvability-frr/ze.conf` - FRR interop scenario config (alongside `frr.conf` and registration in `internal/le/interoplab/bgp/`)
- `rfc/discrimination/rfc9252/RFC9252-5-2.json` - discrimination record written by `./le rfc record` (not by hand)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/rib/yang/ze-rib.yang` (`srv6-sid-resolvability-fields` augmented onto `/bgp:bgp`, `/bgp:bgp/bgp:group`, `/bgp:bgp/bgp:peer`, `/bgp:bgp/bgp:group/bgp:peer`); regenerate glue via `./le yang glue update` |
| YANG validation constraints | Yes | `ignore` uses native `type boolean`; `family/name` uses `type zt:address-family` and `family/ignore` uses `type boolean` with `mandatory true` |
| YANG custom validators | No | Native YANG types plus `parseSRv6ResolvabilityConfig` family-registry lookup cover all constraints without a new `ze:validate` hook |
| CLI commands/flags | No | Observable through existing `show bgp rib` (`\| match srv6-sid-unresolvable`) and `show doctor`; no new command verb |
| CLI grammar (keyword before value) | N-A | No new CLI command |
| Editor autocomplete | Yes | Automatic from YANG schema (`boolean` and `zt:address-family`) once `./le yang glue update` regenerates `ze-rib.yang` glue |
| Functional test for new RPC/API | Yes | `test/plugin/srv6-bestpath-unresolvable.ci`, `test/plugin/srv6-bestpath-resolvable.ci`, `test/plugin/srv6-bestpath-static-wins.ci`, `test/plugin/srv6-bestpath-bypass-config.ci` |
| Pipe completeness | Yes | `show bgp rib` already runs through the RIB pipeline; new JSON row keys (`srv6-sid-resolvable`, `ineligible-reason`) work automatically with `\| match`, `\| json`, `\| count` |
| Env var registration | N-A | No new environment variable |
| Doctor check for runtime dependencies | No | No new file path, socket, kernel module, or binary; reuses existing in-process Loc-RIB and forked `route-metrics` RPC |
| Doctor code per problem class | Yes | Runtime-phase Stage 1 check `bgp-rib-unresolved-srv6-sid` (`doctor-bgp-srv6-sid-unresolvable`, warning, fix `ze show bgp rib \| match srv6-sid-unresolvable`), registered in `internal/component/bgp/plugins/rib/doctor_codes.go` and declared in `rib.go` `DoctorChecks` |
| Prometheus counters/metrics | Yes | `ze_rib_srv6_sid_unresolvable_exclusions_total` (CounterVec, label `family`) registered in `SetMetricsRegistry` (`internal/component/bgp/plugins/rib/rib.go`) |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new SAFI, capability, or attribute; changes pre-ranking candidacy for existing families carrying Prefix-SID SRv6 Service TLVs |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | `docs/features.md` is generated from `features/*.md`; SRv6 overlay behavior is documented in `docs/features/srv6.md` (row 6), and no `features/*.md` maturity level changes until real-path runs are recorded |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- document `srv6-sid-resolvability { ignore; family <name> { ignore; } }` under BGP configuration (regenerate `docs/config-reference.md` if needed) |
| 3 | CLI command added/changed? | No | No new CLI command; `show bgp rib` gains additive JSON keys on excluded SRv6 rows |
| 4 | API/RPC added/changed? | No | No new RPC; `route-metrics` is unchanged |
| 5 | Plugin added/changed? | No | No plugin added or removed; `bgp-rib` gains behavior documented under rows 2, 6, 12, and 14 |
| 6 | Has a user guide page? | Yes | `docs/features/srv6.md` -- update Section 5 row from Partial to Implemented, update data flow diagram, and document `srv6-sid-resolvability` local-policy bypass (noting `sysrib` defense-in-depth) |
| 7 | Wire format changed? | No | No BGP wire encoding changes; only best-path candidacy changes |
| 8 | Plugin SDK/protocol changed? | No | Reuses existing `route-metrics` RPC and Stage 1 `DoctorChecks` without changing `pkg/plugin` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9252.md` (`RFC9252-5-2` loses `{gap}`); `rfc/requirements/rfc9252.md` and `docs/features/rfc-status.md` regenerate via `./le rfc update` |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` and interop harnesses with new test cases only |
| 11 | Affects daemon comparison? | No | Verified by `grep -i srv6 docs/comparison.md` (zero hits): `docs/comparison.md` carries no SRv6 Service SID resolvability row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/route-selection.md` and `docs/architecture/plugin/rib-storage-design.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata key added |
| 14 | Prometheus counters added/changed? | Yes | `ze_rib_srv6_sid_unresolvable_exclusions_total` documented in `docs/architecture/plugin/rib-storage-design.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No new plugin, event type, send type, command, or capability; Stage 1 runtime doctor check `bgp-rib-unresolved-srv6-sid` is declared by existing `bgp-rib` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED via `./le spec citation anchors`: modified declared docs are `docs/architecture/route-selection.md` and `docs/architecture/plugin/rib-storage-design.md`; declared docs unaffected are `docs/architecture/api/commands.md` (declared by `rib_pipeline.go`; no command path or pipe operator changes), `docs/architecture/core-design.md` (declared by `sysrib.go`; comment-only cleanup in `resolvedFibEntry`, behavior unchanged), `docs/architecture/doctor-and-health-checks.md` (declared by `doctor_codes.go`; runtime doctor contract unchanged, `bgp-rib-unresolved-srv6-sid` follows the existing Stage 1 `DoctorChecks` route), and `docs/architecture/config/transaction-protocol.md` (declared by `config_tx.go`; verify/apply/rollback protocol unchanged). Advisory anchored docs (`docs/architecture.md`, `docs/architecture/api/birdwatcher-compat.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/text-format.md`, `docs/architecture/behavior/fsm-established.md`, `docs/architecture/decisions/001-pull-model-metrics.md`, `docs/architecture/rib-transition.md`, `docs/architecture/rib/forward-handle.md`, `docs/architecture/rib/unified-locrib.md`, `docs/architecture/update-building.md`, `docs/architecture/wire/nlri-evpn.md`, `docs/config-reference.md`, `docs/features/api-commands.md`, `docs/guide/bmp.md`, `docs/guide/cli.md`, `docs/guide/command-reference.md`, `docs/guide/graceful-restart.md`, `docs/guide/looking-glass.md`, `docs/guide/monitoring.md`, `docs/guide/plugins.md`, `docs/guide/route-injection.md`, `docs/guide/vpp.md`, `docs/plugin-development/metrics.md`) remain accurate; `docs/config-reference.md` regenerates from `ze-rib.yang` if tracked |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/srv6.md` and `docs/guide/configuration.md` examples verified against `ze-rib.yang` |

## Design presentation

### What changes

`bgp-rib` enforces RFC 9252 Section 5 (and Section 6 for EVPN) SRv6 Service SID
resolvability before Phase 2 BGP best-path selection by extending its existing
RFC 4271 next-hop admission path (`candidateNextHopResolved` / `nextHopResolved`)
rather than building a second reachability seam or Loc-RIB worker. Every
candidate carrying a valid SRv6 Service SID has its merged SID (including VPN
label transposition) checked against the Loc-RIB (via `igpcost` in-process and
the `route-metrics` RPC when forked); unresolvable paths lose candidacy while
staying retained in Adj-RIB-In, re-entering or leaving selection automatically
when locator routes change. Operators gain an explicit per-peer / per-family
YANG local-policy bypass (`srv6-sid-resolvability { ignore true; }` and
`family <name> { ignore true; }` at `bgp`, `group`, `peer`, or `group/peer`
scope, strict default `false`), plus `show bgp rib` (`srv6-sid-unresolvable`),
`show doctor` (`doctor-bgp-srv6-sid-unresolvable`), and Prometheus visibility.

### Before and after

1. **Unresolvable SRv6 BGP path with a reachable fallback BGP peer or static backup:**
   - Before: BGP selects the unresolvable SRv6 path as best, re-advertises it to downstream peers, and installs it into Loc-RIB; `sysrib` then withholds the FIB write, leaving the prefix dark and blocking the runner-up BGP path or static backup.
   - After: `candidateSRv6SIDResolved` excludes the unresolvable SRv6 path before Phase 2 ranking. The reachable runner-up BGP path wins and is advertised, or the static backup wins Loc-RIB arbitration and is programmed into the kernel FIB.

2. **Locator route added or withdrawn later (no BGP UPDATE):**
   - Before: If the SRv6 path won initially, the FIB stayed dark until the locator appeared; if a non-SRv6 path had higher preference or another SRv6 path failed, BGP never reconsidered candidacy on locator reachability changes.
   - After: The existing Loc-RIB `OnChange` worker (`runAIGPSelection`) and forked revision poller (`runRemoteMetrics`) re-run `reselectAIGPRoutes` over all retained Adj-RIB-In paths, automatically promoting the retained SRv6 path when its locator appears and failing over when its locator disappears.

3. **Operator inspecting why an SRv6 route is not selected (`show bgp rib` and `show doctor`):**
   - Before: No pre-selection SRv6 exclusion existed in `bgp-rib`, so `show bgp rib` showed the unprogrammable route as `"best": true` while the kernel FIB lacked the prefix.
   - After: `show bgp rib | match srv6-sid-unresolvable` displays `"srv6-sid-resolvable": false` and `"ineligible-reason": "srv6-sid-unresolvable"` on the retained non-best row, and `ze show doctor` reports warning `doctor-bgp-srv6-sid-unresolvable` with the fix command.

4. **Local-policy bypass for alternate steering (RFC 9252 Section 5 MAY clause):**
   - Before: No configuration option existed to bypass SRv6 Service SID resolvability.
   - After: Setting `bgp { peer pe1 { srv6-sid-resolvability { ignore true; } } }` or `bgp { group vpn { srv6-sid-resolvability { family ipv4/mpls-vpn { ignore true; } } } }` admits the peer's/family's routes to BGP best-path even without a local Loc-RIB route to the SID; reloading or rolling back immediately re-evaluates retained routes. Default remains strict enforcement.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Add a new leaf seam `internal/core/rib/reachable` published from `sysrib.SetLocRIB` and a second `OnChange` worker + SID dependency index in `rib.go` | Duplicates existing HEAD infrastructure: `sysrib.SetLocRIB` no longer exists in production (sysrib is a `route_view` consumer), `internal/core/rib/igpcost` + `rib_next_hop.go` + `route-metrics` RPC already provide internal and forked recursive resolution, and `runAIGPSelection` / `runRemoteMetrics` already re-evaluate all retained Adj-RIB-In routes safely outside locks |
| Check the partial wire SID from `pool.ExtractSRv6SIDFull` instead of the merged SID from `entrySRv6SID` / `srv6SIDFromResult` | RFC 9252 Section 3.2.1 allows a transposition window to overlap locator bits, so the partial SID can match a different IPv6 prefix than the actual Service SID the FIB installs |
| Remove `sysrib`'s `resolvedFibEntry` SRv6 check as redundant | Removing a working guard violates `ai/rules/planning.md`; keeping it provides defense-in-depth and prevents broken single-SID SEG6 kernel/VPP installs when an operator enables `srv6-sid-resolvability { ignore true; }` |
| Make unset reachability permissive in `bgp-rib` | Conflicts with HEAD's fail-closed `docs/architecture/route-selection.md` design: in production `bgp-rib` always wires either `locrib.Default()` or `forkRIB` (`route-metrics`), and lookup errors must fail closed and raise `doctor-bgp-next-hop-metrics` |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG schema, config parser, doctor code/check declaration, and candidate gate skeleton
   - Tests: `TestSRv6ResolvabilityBypassConfigAndReload`, `TestGatherCandidatesSkipsUnresolvableSRv6SID`
   - Files: `internal/component/bgp/plugins/rib/yang/ze-rib.yang` (plus `./le yang glue update`), `internal/component/bgp/plugins/rib/rib_srv6_resolvability_config.go`, `internal/component/bgp/plugins/rib/config_tx.go`, `internal/component/bgp/plugins/rib/doctor_codes.go`, `internal/component/bgp/plugins/rib/rib.go`, `internal/component/bgp/plugins/rib/rib_next_hop.go`, `internal/component/bgp/plugins/rib/rib_commands.go`, `internal/component/bgp/plugins/rib/rib_manual.go`
   - Verify: tests fail first while `candidateSRv6SIDResolved` is stubbed, then YANG glue and config transaction wire cleanly and wiring tests begin exercising the real path
2. **Phase: Pre-selection SRv6 SID resolvability and reload reselection** -- merged-SID extraction, RFC 4271 resolution (CIDR prospective + non-CIDR + forked `route-metrics`), and local-policy bypass precedence
   - Tests: `TestGatherCandidatesSkipsUnresolvableSRv6SID`, `TestGatherCandidatesAdmitsResolvableSRv6SID`, `TestUnresolvableSRv6LosesToNonSRv6Path`, `TestLocRIBChangeReevaluatesSRv6Prefix`, `TestLocRIBRemovalReevaluatesSRv6Prefix`, `TestSRv6ResolvabilityProspectiveSelfCaptureAndCrossFamily`, `TestResolvabilityUsesMergedTransposedSID`, `TestEVPNUnresolvableSRv6SIDIneligible`, `TestSRv6ResolvabilityRecursionDepthAndDiscard`, `TestForkedRIBSRv6SIDResolvability`, `TestSRv6ResolvabilityBypassConfigAndReload`
   - Files: `internal/component/bgp/plugins/rib/rib_next_hop.go`, `internal/component/bgp/plugins/rib/rib_commands.go`, `internal/component/bgp/plugins/rib/rib_manual.go`, `internal/component/bgp/plugins/rib/rib_srv6_resolvability_config.go`, `internal/component/bgp/plugins/rib/config_tx.go`, `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go`
   - Verify: all existing `RFC9252-5-1` tests pass untouched; scoped unit tests pass under `-race -tags ze_bgp`; enforcing code carries the RFC 9252 Section 5/6 quote comment
3. **Phase: Operator visibility (`show bgp rib`, runtime doctor, metrics) and `sysrib` comment cleanup**
   - Tests: `TestSRv6UnresolvableVisibilityShowAndDoctor`
   - Files: `internal/component/bgp/plugins/rib/rib_pipeline.go`, `internal/component/bgp/plugins/rib/rib_pipeline_bestfact.go`, `internal/component/bgp/plugins/rib/rib_next_hop_doctor.go`, `internal/component/bgp/plugins/rib/doctor_codes.go`, `internal/component/bgp/plugins/rib/rib.go`, `internal/component/sysrib/sysrib.go`
   - Verify: `show bgp rib` emits `"srv6-sid-resolvable": false` and `"ineligible-reason": "srv6-sid-unresolvable"` on excluded rows (and omits on resolvable/bypassed/non-SRv6 rows); runtime doctor check `bgp-rib-unresolved-srv6-sid` works in internal and forked modes with 4096-entry cap
4. **Phase: Functional and interop proof**
   - Tests: `test/plugin/srv6-bestpath-unresolvable.ci`, `test/plugin/srv6-bestpath-resolvable.ci`, `test/plugin/srv6-bestpath-static-wins.ci`, `test/plugin/srv6-bestpath-bypass-config.ci`, and interop scenario `bgp-srv6-resolvability-frr`
   - Files: `test/plugin/srv6-bestpath-*.ci`, `test/interop/scenarios/bgp-srv6-resolvability-frr/`
   - Verify: prove each functional and interop test goes RED when the SRv6 resolvability filter is reverted, then GREEN when restored (`ai/rules/interop-and-goal-validation.md`)
5. **Phase: RFC ledger, discrimination record, and documentation**
   - Tests: `./le rfc check`, `./le doc check all`, `./le config claims`, `./le config coercion check`, `./le yang glue check`, `./le plugin boundary check`, `./le plugin declarations check`, `./le arch tier check`
   - Files: `rfc/short/rfc9252.md`, `rfc/discrimination/rfc9252/RFC9252-5-2.json` (via `./le rfc record`), regenerated `rfc/requirements/rfc9252.md` and `docs/features/rfc-status.md` (via `./le rfc update`), `docs/architecture/route-selection.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/features/srv6.md`, `docs/guide/configuration.md`
   - Verify: `./le rfc record` produces `rfc/discrimination/rfc9252/RFC9252-5-2.json` and `./le rfc check` passes cleanly

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-11 has an implementation at file:line and a named test; AC-7 is proven against the real Linux kernel FIB |
| Feature completeness | Recovery (AC-3) works from full exclusion (no `bestPrev` entry) in both internal and forked `bgp-rib` placements; reload and rollback of `srv6-sid-resolvability` immediately reselect retained routes |
| Correctness | The address checked is always the merged SID from `entrySRv6SID` (`srv6SIDFromResult`); EVPN (`l2vpn/evpn`), VPNv4/VPNv6, and IPv4/IPv6 unicast are all covered; `srv6-sid-resolvability { ignore true; }` never bypasses `RFC9252-5-1` semantic validity or RFC 4271 `NEXT_HOP` resolvability |
| Naming | YANG container `srv6-sid-resolvability`, leaf `ignore`, list `family` (`name`, `ignore`); JSON keys `srv6-sid-resolvable` and `ineligible-reason` (`srv6-sid-unresolvable`); doctor check `bgp-rib-unresolved-srv6-sid` / code `doctor-bgp-srv6-sid-unresolvable`; metric `ze_rib_srv6_sid_unresolvable_exclusions_total` |
| Data flow | No lock-order inversion: `runAIGPSelection` enqueues on size-1 channel with no BGP lock, `reselectAIGPRoutes` snapshots under `peerMu.RLock` and elects unlocked, `unresolvedSRv6SIDs` uses a leaf mutex |
| Operator errors and doctor codes | Invalid boolean or unknown family in `srv6-sid-resolvability` returns an actionable `opererr` naming the offending value and fix; `doctor-bgp-srv6-sid-unresolvable` is registered in `doctor_codes.go` and declared in Stage 1 `DoctorChecks` (`rib.go`) |
| Rule: `ai/rules/rfc-compliance.md` | `RFC9252-5-2` tagged tests state only what the test body proves; discrimination record `rfc/discrimination/rfc9252/RFC9252-5-2.json` lands in the same commit via `./le rfc record` |
| Rule: `ai/rules/config.md` | `parseSRv6ResolvabilityConfig` coerces both JSON `bool` and string `"true"`/`"false"` and refuses any other value (`./le config coercion check`) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Pre-selection SRv6 SID resolvability wired in all forwarding candidate gathers | `grep -rn "candidateSRv6SIDResolved" internal/component/bgp/plugins/rib/ --include='*.go'` shows `rib_commands.go` (CIDR + non-CIDR) and `rib_manual.go` |
| YANG local-policy bypass option compiled and claimed | `./le yang glue check`, `./le config claims`, and `./le config coercion check` pass |
| `RFC9252-5-2` is no longer a gap and both polarities are tagged | `grep -n "RFC9252-5-2" rfc/short/rfc9252.md rfc/requirements/rfc9252.md` shows no `{gap}`; `grep -rn "RFC requirement: RFC9252-5-2" internal/` returns positive and negative tags |
| Discrimination record exists and passes RFC gate | `./le rfc check` passes with `rfc/discrimination/rfc9252/RFC9252-5-2.json` present |
| No existing `RFC9252-5-1` test was edited | `git diff --stat internal/component/bgp/plugins/rib/rfc9252_srv6_ineligible_test.go internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go internal/component/bgp/plugins/rib/rfc9252_srv6_transposition_test.go` is empty |
| Interop scenario discovered by name | `./le test integration` lists `bgp-srv6-resolvability-frr` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Service SID bytes come from untrusted peers; `candidateSRv6SIDResolved` extends no wire parser and reads only the already-validated `entrySRv6SID` address; `parseSRv6ResolvabilityConfig` rejects malformed booleans and unregistered family names |
| Resource exhaustion | A peer advertising many routes with unreachable SIDs cannot grow runtime doctor tracking past `maxTrackedUnresolvedNextHops` (4096) + fixed 512-byte overflow deduplication filter; forked `remoteRIB` caches are bounded and cleared on revision change |
| Denial by a neighbor | Peers cannot force unbounded re-evaluation: Loc-RIB `OnChange` bursts coalesce onto a size-1 channel in `runAIGPSelection`, and forked mode polls once per second in `runRemoteMetrics` |
| Fail-closed admission | Missing covering route, discard route, self-loop, >8-hop chain, or failed forked `route-metrics` RPC all answer `Resolved: false` (excluded); bypass requires explicit operator config (`srv6-sid-resolvability { ignore true; }`) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read producer source in Current Behavior; if misunderstood -> RESEARCH |
| Lint or gate failure (`./le arch tier check`, `./le plugin boundary check`, `./le config coercion check`, `./le yang glue check`) | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check the AC: wrong AC -> DESIGN, correct AC -> IMPLEMENT |
| Deadlock in re-evaluation or doctor path | Check lock ordering (`shard.mu` -> `peerMu.RLock` -> leaf mutex; never acquire `peerMu` inside `OnChange` or `doctorCheck`) |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Checking HEAD revealed that the core reachability oracle (`internal/core/rib/igpcost`), internal/forked `nextHopResolved` admission (`rib_next_hop.go`, `rib_remote.go`, `dispatch_route_metrics.go`), coalescing Loc-RIB/revision reselection (`runAIGPSelection`, `reselectAIGPRoutes`, `runRemoteMetrics`), `show bgp rib` ineligibility visibility (`rib_pipeline_bestfact.go`), and bounded runtime doctor checks (`rib_next_hop_doctor.go`) all already exist. Extending them directly avoids a redundant seam, avoids a second `OnChange` worker, and brings forked subprocess support and prospective CIDR self-capture for free.
- RFC 9252's "before best path computation" is satisfied by excluding the candidate in `gatherPrefixCandidatesForLocked` / `gatherKeyCandidatesForLocked` (allowing reachable runner-ups or lower-AD static routes to win), not by withholding the FIB write after selection.
- Earlier notes about `sysrib` lacking SRv6 tests, `docs/features/srv6.md` over-claiming Section 5, and `rfc/short/rfc9252.md` mis-numbering `RFC9252-5-1` as Section 5 have all been resolved on HEAD (`sysrib_srv6_test.go` landed 2026-09-09; `RFC9252-5-1` corrected to §7 on 2026-09-21; `docs/features/srv6.md` updated to Partial).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reuse `igpcost` + `RIBManager.nextHopResolved` + `runAIGPSelection` / `runRemoteMetrics` for SRv6 Service SID resolvability | (a) Create a new leaf seam `internal/core/rib/reachable` published from `sysrib` and a second `OnChange` worker + SID dependency index in `rib.go`; (b) move `nhResolver` into `internal/core/` | `sysrib.SetLocRIB` no longer exists in production; HEAD already provides bounded 8-hop recursive LPM resolution in `igpcost`, forked `route-metrics` RPC in `rib_remote.go`, prospective CIDR self-capture in `igpcost.ResolveProspective`, and coalescing reselection over all retained Adj-RIB-In routes in `rib_aigp.go`. Extending the existing mechanism avoids duplication and guarantees internal/forked parity |
| Check the merged SID returned by `entrySRv6SID` (`srv6SIDFromResult`) | Check the partial SID from `pool.ExtractSRv6SIDFull` | RFC 9252 Section 3.2.1 permits a transposition window overlapping locator bits, so the partial SID can match a different IPv6 prefix than the Service SID actually installed in the FIB (A-2) |
| Model the owner-decided local-policy bypass in `internal/component/bgp/plugins/rib/yang/ze-rib.yang` as `srv6-sid-resolvability { ignore; family <name> { ignore; } }` augmented across `bgp`, `group`, `peer`, and `group/peer` with no YANG default on `ignore` | (a) Single global switch; (b) leaf under `ze-bgp-conf.yang` with `default false` | Matches Thomas's 2026-11-24 decision ("per peer or per family; default remains strict"), follows `ze-rib.yang`'s established `flowspec-redirect-check-fields` inheritance pattern (no YANG default so peer inherits group and group inherits global), keeps `bgp-rib` owning its own policy config, and supports dynamic peer groups via `configjson.LookupPeerConfig` |
| Keep `sysrib`'s `resolvedFibEntry` SRv6 SID guard unchanged as defense-in-depth | Remove `sysrib`'s guard once `bgp-rib` filters before best-path | Preserves defense-in-depth (`ai/rules/planning.md`) and prevents installing an unresolvable single-SID SEG6 encap route into Linux/VPP when an operator configures `srv6-sid-resolvability { ignore true; }` before SR Policy steering exists |
| Surface excluded SRv6 routes in `show bgp rib` (`srv6-sid-resolvable`, `ineligible-reason`) and Stage 1 runtime doctor check `bgp-rib-unresolved-srv6-sid` | Silent exclusion or log-only reporting | Matches HEAD's `next-hop-unresolvable` operator visibility contract in `docs/architecture/route-selection.md` so operators can diagnose excluded SRv6 routes in both internal and forked placements |

## Known Limitations

- "If the SRv6 SID is reachable via more than one forwarding table, local policy is used to determine which table to use" (RFC 9252 Section 5) has no trigger in Ze today: there is one Loc-RIB and no VRF forwarding table plumbing (`mirrorToLocRIB` in `rib_bestchange.go`). It becomes reachable when Ze adds VRF forwarding tables.
- EVPN label-field transposition (`TransposLen > 0` on `l2vpn/evpn`) is tracked separately in `plan/release/fixes/spec-srv6-evpn-label-width.md` (`RFC9252-6.2-1`). This spec enforces RFC 9252 Section 6 resolvability on every EVPN SRv6 Service SID that `entrySRv6SID` reconstructs (including all non-transposed EVPN SIDs) and will automatically cover transposed EVPN SIDs once `nlrisplit.TranspositionLabel` implements EVPN extraction.
- When `srv6-sid-resolvability { ignore true; }` bypasses the BGP pre-selection check, BGP selects and propagates the route even without a covering Loc-RIB route for the Service SID, while `sysrib`'s `resolvedFibEntry` continues withholding single-SID SEG6 FIB programming because Ze does not yet implement SR Policy data-plane steering (`docs/features/srv6.md`).

## RFC Documentation (Scope: protocol)

The filter in `candidateSRv6SIDResolved` carries a comment naming RFC 9252 Section 5 and quoting `"Therefore, the ingress PE MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix for the BGP best path computation. The resolvability is evaluated as per [RFC4271]."` and noting that Section 6 repeats it for EVPN and Section 5 permits local-policy bypass.

| Polarity | Tag line | Location |
|----------|----------|----------|
| Negative | `RFC requirement: RFC9252-5-2 negative -- a path whose reconstructed SRv6 Service SID has no resolving route in the Loc-RIB is excluded from best-path candidate selection` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` |
| Positive | `RFC requirement: RFC9252-5-2 positive -- a path whose reconstructed SRv6 Service SID resolves in the Loc-RIB is admitted to best-path candidate selection` | `internal/component/bgp/plugins/rib/rfc9252_srv6_resolvability_test.go` |

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 9252 Sections 5 and 6 (`RFC9252-5-2`) | `srv6-sid-resolvability` omitted (strict default `ignore false`): every BGP path carrying a valid SRv6 Service SID MUST resolve its reconstructed SID in the Loc-RIB (as per RFC 4271) before entering Phase 2 best-path selection; unresolvable paths are excluded while retained in Adj-RIB-In | `bgp { srv6-sid-resolvability { ignore true\|false; family <name> { ignore true\|false; } } }` (and `group`, `peer`, `group/peer` scopes) bypasses SRv6 Service SID reachability per peer or per family (RFC 9252 Section 5 MAY clause) while still enforcing `RFC9252-5-1` semantic SID validity and RFC 4271 `NEXT_HOP` resolvability | `TestGatherCandidatesSkipsUnresolvableSRv6SID`, `TestGatherCandidatesAdmitsResolvableSRv6SID`, `TestUnresolvableSRv6LosesToNonSRv6Path`, `TestLocRIBChangeReevaluatesSRv6Prefix`, `TestLocRIBRemovalReevaluatesSRv6Prefix`, `TestResolvabilityUsesMergedTransposedSID`, `TestEVPNUnresolvableSRv6SIDIneligible`, `test/plugin/srv6-bestpath-unresolvable.ci`, `test/plugin/srv6-bestpath-resolvable.ci`, `test/plugin/srv6-bestpath-static-wins.ci`, `bgp-srv6-resolvability-frr` | `TestSRv6ResolvabilityBypassConfigAndReload`, `test/plugin/srv6-bestpath-bypass-config.ci` |

Both `RFC9252-5-2` tags are new, so implementation owes a discrimination record written by `./le rfc record` at `rfc/discrimination/rfc9252/RFC9252-5-2.json` in the same commit.

## Review Gate

Pending implementation; populated by `/ze-review` and `/ze-close`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, RFC gaps, Depends, Phase, and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior, Existing Mechanisms, Design documents, Data Flow (with Changed-Wire Census), and Design presentation sections completed
- [ ] AC-N rows carry positive observable assertions and non-empty Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar", "Doctor check", "Doctor code", and "YANG schema" accurately

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests (`./le test unit pkg path ./internal/component/bgp/plugins/rib/`, `./le test unit pkg path ./internal/component/sysrib/`), the gates (`./le yang glue check`, `./le config claims`, `./le config coercion check`, `./le plugin boundary check`, `./le plugin declarations check`, `./le arch tier check`, `./le rfc check`, `./le doc check all`), and the functional/interop suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding (outbound and inbound)
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] The revert-rebuild-RED walk recorded for each functional and interop test
- [ ] `./le rfc record` has written `rfc/discrimination/rfc9252/RFC9252-5-2.json`

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `./le spec review record`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-srv6-bestpath-resolvability.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
