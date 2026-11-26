# Spec: bgp-ls-receiver-accept-and-limit-controls

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Required receiver controls and bounded storage belong to the shipped BGP-LS capability. |
| Scope | protocol |
| RFC gaps | RFC9552-8.2.3-1, RFC9552-8.2.3-3 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the RFC asks for.** RFC 9552 Section 8.2.3 states three receiver-side
configuration obligations, of which two bind a BGP-LS receiver. The first
(`RFC9552-8.2.3-1`, SHOULD): "An implementation SHOULD allow the operator to
specify neighbors to which Link-State NLRIs will be advertised and from which
Link-State NLRIs will be accepted." The second (`RFC9552-8.2.3-3`, SHOULD): "An
implementation SHOULD allow the operator to specify the maximum number of
Link-State NLRIs stored in a router's Routing Information Base (RIB)." Both rows
in `rfc/short/rfc9552.md` are unannotated and untested today.

**What exists at HEAD.**

- **BGP-LS producers and propagation exist.** Ze originates BGP-LS via
  `bgp-ls-export` (from native IS-IS and OSPF state) and `bgp-epe`, relays
  received BGP-LS opaquely via `bgp-rs`, and stores received BGP-LS in
  `bgp-rib`'s opaque `FamilyRIB`. Both halves of `RFC9552-8.2.3-1` ("advertised
  to" and "accepted from") therefore act on live paths.
- **Per-neighbor accept/advertise control already exists.** A peer whose config
  carries a `session > family` block negotiates `bgp-ls/bgp-ls` only when that
  block names it (RFC 4760 capability intersection). On receive,
  `validateUpdateFamilies` (`internal/component/bgp/reactor/session_validation.go`)
  refuses any unnegotiated family with NOTIFICATION 3/9 (`UPDATE Message Error`
  / `Optional Attribute Error`, RFC 4760 Section 7), or drops the UPDATE while
  keeping the session up when `mode ignore` or `ignore-mismatch enable` is set.
  On send, the final session writer (`session_ownership.go`) and batch/replay
  send paths withhold sections for families the destination did not negotiate.
  For one-directional control when BGP-LS is negotiated, `policy { family-filter }`
  (`action remove`) suppresses `bgp-ls/bgp-ls` in either the `import` chain
  (advertise only) or the `export` chain (accept only). What is missing is unit,
  functional `.ci`, and GoBGP interop proof, RFC tags with discrimination
  records, and documentation.
- **The family-less peer fallback remains a named gap.** A peer with NO
  `family` block still advertises every plugin decode family via
  `offeredMultiprotocol` (`session_negotiate.go`), BGP-LS included. Fixing that
  cross-family fallback is owned by `plan/release/fixes/spec-bgp-one-off-defects.md`
  (journal `plan/journal/silent-fall-through.md`, 2026-08-16). Per owner decision
  (2026-10-13), `RFC9552-8.2.3-1` lands now with scoped evidence ("peers
  carrying an explicit `family` block") and names the no-family-block fallback
  as the gap against RFC 9552 Section 8.1.2's off-by-default.
- **The blocking counting defect and sibling role overlap are settled.**
  `forEachPrefixEntry` (`internal/component/bgp/reactor/session_prefix.go`)
  dispatches through `nlrisplit.Get` / `GetWithdraw`, so BGP-LS sections are
  counted by their own TLV framing (`RFC9552-8.2.6-1` carries no `{gap}`).
  Receive-path NLRI syntactic validation (`judgeBGPLSNLRISection`,
  `RetainWellFormedNLRI`) is landed, `RFC9552-8.2.6-2` is `{not-applicable}`,
  and sibling `plan/release/fixes/spec-bgp-ls-receiver-fault-management.md`
  dropped the redundant `consumer-facing` per-peer YANG role (owner decision,
  2026-11-24). Route D of `spec-plugin-ownership-imports` also landed
  (`internal/core/bgp/nlri/bgpls/`), and this spec touches nothing under
  `internal/component/bgp/plugins/nlri/ls/`.
- **No router-wide family ceiling exists.** Per-peer `prefix { maximum N; }`
  (`session_prefix.go`) bounds only one session at a time (`prefixCounts` dies
  with the `Session`). Nothing bounds total Link-State NLRIs held across all
  sessions on the router.

**What this spec delivers.**

1. **Prove, tag, and document per-neighbor accept/advertise controls
   (`RFC9552-8.2.3-1`).** No new BGP-LS-specific neighbor list config is added.
   Unit tests, two `.ci` functional tests, a GoBGP interop scenario,
   discrimination records, and docs prove that the per-peer/per-group `family`
   block controls bidirectional BGP-LS acceptance and advertisement, and
   `policy { family-filter }` (`action remove`) controls either direction
   independently.
2. **Add a router-wide per-family ceiling (`RFC9552-8.2.3-3`).** Add
   `bgp { prefix-limit { family <afi/safi> { maximum N; warning M; } } }` in
   `ze-bgp-conf.yang`, generic over every registered address family via
   `registered-address-family` (no `bgp-ls` literal in YANG or reactor). The
   reactor owns one atomic counter per ceiling family, fed by each session's
   installed route-identity set (`nlrisplit.GetPrefixKey` + ADD-PATH path ID),
   summing installed paths across all active sessions. Crossing the ceiling
   drops the crossing UPDATE without dispatching it and keeps every session up
   (no NOTIFICATION), rolls back the session's installed set, logs and raises
   operational reports, and updates Prometheus metrics. Reload updates the
   ceiling transactionally via `Globals` without resetting sessions or evicting
   already-installed NLRIs.

**Not in scope.** Section 8.2.3's producer-side SHOULDs other than the
advertise half of `8.2.3-1` (advertisement rate limit `8.2.3-2`, abstracted
topologies `8.2.3-4`, and 4096-byte BGP-LS UPDATE size limit `8.2.3-7`) stay
with `plan/post-release/features/spec-bgp-ls-origination-and-the-scheduled-marker.md`.
Fixing the no-`family`-block plugin fallback stays with
`plan/release/fixes/spec-bgp-one-off-defects.md`.

## Owner decisions (2026-10-13)

All seven design questions were answered by Thomas on 2026-10-13 ("LS follow
recommendations"; evidence: `research-drain-ospf-startup-bgpls.md`) and are
unfolded across this spec:

| Topic | Decision |
|-------|----------|
| Accept/advertise config surface (`RFC9552-8.2.3-1`) | Use the existing per-peer/per-group `family` block for bidirectional allow-listing and `policy { family-filter }` (`action remove`) for one-direction refusal; add no dedicated BGP-LS neighbor list. |
| Family-less peer fallback (`session_negotiate.go`) | Land `RFC9552-8.2.3-1` now with scoped evidence ("peers with an explicit `family` block"), naming the no-family-block fallback as a gap against RFC 9552 Section 8.1.2's off-by-default. |
| Action when router-wide ceiling is crossed | Drop the crossing UPDATE without dispatching it and keep every session established (no NOTIFICATION), because the crossing peer is not necessarily the one that filled the RIB. |
| Counting semantics and Graceful Restart | Count installed paths summed across active sessions; under-counting stale paths held in `bgp-rib` during a Graceful Restart window is an accepted Known Limitation. |
| Sibling fault-management spec overlap | Re-scope the sibling rather than building a redundant role (subsequently executed in `plan/release/fixes/spec-bgp-ls-receiver-fault-management.md` via the 2026-11-24 owner decision). |
| Interop peer daemon | Try GoBGP first (`test/interop/Dockerfile.gobgp`, pinned GoBGP v4.11.0; FRR originates BGP-LS only from 10.7). |
| Ceiling YANG placement and shape | Mirror the per-peer `prefix` container shape at `bgp` level (`bgp > prefix-limit > family <afi/safi> { maximum; warning; }`), citing the IETF BGP-LS YANG draft's `max-number` as the external name in the description. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/nlri-bgpls.md` - how a Link-State NLRI is framed, validated on receive, originated, and stored
  → Constraint: Ze both receives and produces BGP-LS (`bgp-ls-export`, `bgp-epe`, `bgp-rs`, `bgp-rib`), so both halves of `RFC9552-8.2.3-1` have live paths
  → Constraint: Section 8.2.2 receive validation (`judgeBGPLSNLRISection`, `RetainWellFormedNLRI`) runs in `enforceRFC7606` before `checkPrefixLimits`, so the ceiling counts only well-formed NLRIs
  → Decision: no file this spec creates or edits lives under `internal/component/bgp/plugins/nlri/ls/` or `internal/core/bgp/nlri/bgpls/`
- [ ] `docs/architecture/core-design.md` - BGP receive pipeline, policy filter chain, and Section 15 Operational Report Bus
  → Constraint: `filter_family` is generic over every registered family and both directions (`remove` in import/export, `tear-down` in import only); no BGP-LS-specific branch is added
  → Constraint: new BGP report bus codes follow Section 15's state-based Warning vs event-based Error contract and are documented in Section 15 and `docs/architecture/api/commands.md`
- [ ] `docs/architecture/behavior/fsm-established.md` - `processMessage` ordering and `checkPrefixLimits` rollback contract
  → Constraint: `validateUpdateFamilies` -> `receiveCeiling` -> `checkPrefixLimits` run before callback dispatch; if any family or ceiling refuses an UPDATE, `rollbackPrefixSets` restores every installed set and `EventUpdateMsg` restarts the HoldTimer on drop
- [ ] `docs/architecture/testing/interop.md` - BGP Docker interop suites, GoBGP `gobgp-paths`, declarative scenario directories, and typed Go checkers
  → Constraint: scenario directories hold only declarative inputs; assertions live in typed Go checkers under `internal/le/interoplab/bgp/`
  → Constraint: scenarios are named (never numbered) and require positive controls so absence assertions cannot pass vacuously
- [ ] `docs/plugin-development/metrics.md` - metric naming policy (`ze_{scope}_{subject}_{detail}`) and full inventory
  → Constraint: gauges carry no `_total` suffix; counters end in `_total`; variable dimensions (`family`, `peer`) are labels, never part of the metric name

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9552.md` - rows `RFC9552-8.2.3-1` and `RFC9552-8.2.3-3`
  → Constraint: both are SHOULD-level; proof owed is positive and negative tagged tests with `./le rfc record` discrimination records in `rfc/discrimination/rfc9552.json` (`ai/rules/rfc-compliance.md`)
  → Constraint: `RFC9552-8.2.6-1` is already tagged by `TestBGPLSPrefixCountCountsNLRIsNotPrefixBytes` and `TestBGPLSPrefixLimitActuallyFires` (`internal/component/bgp/reactor/rfc9552_prefix_limit_test.go`); neither test is edited by this spec

**RFC text, read at `rfc/full/rfc9552.txt`:**

| Section | Whole sentence quoted |
|---------|-----------------------|
| 8.2.3 | "An implementation SHOULD allow the operator to specify neighbors to which Link-State NLRIs will be advertised and from which Link-State NLRIs will be accepted." |
| 8.2.3 | "An implementation SHOULD allow the operator to specify the maximum number of Link-State NLRIs stored in a router's Routing Information Base (RIB)." |
| 8.2.6 | "An implementation MUST have the means to limit inbound updates." |
| 8.2.6 | "An operator MUST define an import policy to limit inbound updates as follows: * Drop all updates from peers that are only serving BGP-LS Consumers." |

**Key insights:**
- `forEachPrefixEntry` already dispatches through `nlrisplit`, and `session_ownership.go` already gates outbound sections on `negotiated.SupportsFamily(fam)`.
- `Globals` / `GlobalsFromTree` (`config.go`) + `applyGlobalsJournaled` (`reload_globals.go`) already handle transactional reload of BGP-level settings without resetting unchanged peers.
- `Session.Run` (`session.go`) and `Peer.runOnce` (`peer_run.go`) provide deterministic teardown defer sites where a session's installed ceiling counts can be released idempotently.

## Current Behavior (MANDATORY)

**Source files read (verified against HEAD):**
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `forEachPrefixEntry` and `countPrefixEntries` dispatch through `nlrisplit.Get` / `GetWithdraw`. `prefixCounts` is per `Session` (`newPrefixCounts`) and tracks `installed` sets (keyed by `nlrisplit.GetPrefixKey` + 4-octet ADD-PATH ID) only for families whose peer config sets `count installed` (or `retained` for `rs-client` offered limits). `checkPrefixLimits` -> `applyInstalledPrefixSections` -> `rollbackPrefixSets` enforces per-peer limits only; nothing counts across peers.
- [ ] `internal/component/bgp/reactor/session_validation.go` and `internal/component/bgp/reactor/session_read.go` - `validateUpdateFamilies` walks MP_REACH/MP_UNREACH and, for any family absent from `Negotiated().SupportsFamily`, drops the UPDATE (`IgnoreFamilyMismatch` or `mode ignore`) or returns `ErrFamilyNotNegotiated`, which `processMessage` (`session_read.go:548-572`) turns into NOTIFICATION 3/9 (`NotifyUpdateMessage` / `NotifyUpdateOptionalAttr`) and closes the connection. `checkPrefixLimits` runs at `session_read.go:593` before plugin dispatch; on `prefixDrop`, `EventUpdateMsg` restarts the HoldTimer and the message is not dispatched.
- [ ] `internal/component/bgp/reactor/session_ownership.go` and `internal/component/bgp/reactor/reactor_api_forward.go` - outbound sections are withheld for unnegotiated families by `negotiated.SupportsFamily(fam)` at `session_ownership.go:900` (covering relayed, forwarded, originated, and raw writes), and by `nc.Has(fam)` in `reactor_api_forward.go:143`, `reactor_api_batch.go:740,1342`, `selected_origin.go:49`, `relay_recovery.go:465`, and `peer_static_wire.go:113`.
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - `offeredMultiprotocol` advertises configured families when a peer carries a `family` block, and falls back to all plugin decode families (including BGP-LS) when no `family` block is configured (documented `KNOWN DEFECT`, owned by `plan/release/fixes/spec-bgp-one-off-defects.md`).
- [ ] `internal/component/bgp/plugins/filter_family/config.go` and `handler.go` - `parseFamilyFilters` parses `policy > family-filter` instances (forbidding `tear-down` in export chains); `evaluateFamilyFilter` / `judgeFamily` strips or suppresses matching families on either `import` or `export` chains.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `container bgp` has no `prefix-limit` container today; per-peer `session > list family` validates keys with `ze:validate "registered-address-family"` and holds `container prefix { maximum (uint32 range "1..max"); warning (uint32); teardown; count; idle-timeout; reconnect; }`.
- [ ] `internal/component/bgp/reactor/config.go`, `config_prefix.go`, `reload_globals.go`, `reactor_api.go`, and `internal/component/bgp/config/loader_create.go` - `Globals{RouterID, LocalAS}` is parsed by `GlobalsFromTree` at both startup (`treeGlobals`, `loader_create.go:44`) and reload (`reloadPeersFromTree`, `loader_create.go:340`; `loadPeersFullOrTree`, `reactor_api.go:1087`), and applied transactionally on reload by `applyGlobalsJournaled` (`reload_globals.go:22`, called from `reconcilePeersJournaled`, `reactor_api.go:1148`). `parsePrefixLimitFromFamily` (`config_prefix.go`) enforces `maximum > 0`, `warning < maximum`, and default `warning = maximum * 9 / 10`.
- [ ] `internal/component/bgp/config/resolve.go` - `ResolveBGPTree` keeps bgp-level containers at the top of the returned map; `PeersFromTree` ignores unknown top-level containers in `bgpDefaults`.
- [ ] `internal/component/bgp/reactor/session.go`, `peer_session_factory.go`, and `peer_run.go` - `Peer.newSessionWithEpoch` builds every peer-owned `Session` via `NewSession(p.settings)` and wires reactor references (`prefixMetrics`, `aigpReactor`); `Peer.runOnce`'s teardown defer calls `sess.clearReportedWarnings()` after `Session.Run` exits (`peer_run.go:558-560`), and `Session.Run` owns its own exit defers (`session.go:1476-1508`).
- [ ] `internal/component/bgp/reactor/reactor_metrics.go` - registers the eight per-peer `ze_bgp_prefix_*` metrics; no router-wide family ceiling metrics exist.
- [ ] `rfc/short/rfc9552.md` - `RFC9552-8.2.3-1` and `RFC9552-8.2.3-3` are unannotated and untested; `RFC9552-8.2.6-1` is proven with no `{gap}`; `RFC9552-8.2.6-2` is `{not-applicable}`.

**Behavior to preserve:**
- A peer with no `family` block keeps today's `offeredMultiprotocol` negotiation (owned by `plan/release/fixes/spec-bgp-one-off-defects.md`).
- A peer whose `family` block names `bgp-ls/bgp-ls` keeps accepting and propagating BGP-LS unchanged when no router ceiling is configured.
- Per-peer, per-family `prefix { maximum; warning; teardown; count; idle-timeout; reconnect; }` keeps its exact enforcement order, NOTIFICATION 6/1 encoding, report codes, and metrics.
- With no `bgp > prefix-limit` configured, no session allocates or maintains any extra installed set, and the UPDATE receive path pays zero extra work.
- Existing `NewSession` and `newPrefixCounts` signatures stay unchanged so existing RFC-tagged tests compile untouched.

**Behavior to change:**
- Prove, tag (with discrimination records), and document the existing per-peer `family` allow-list and per-direction `family-filter` controls for Link-State NLRIs (`RFC9552-8.2.3-1`).
- Add router-wide `bgp { prefix-limit { family <afi/safi> { maximum N; warning M; } } }` enforcement across all BGP sessions in the reactor (`RFC9552-8.2.3-3`), with transactional reload, operational report bus warnings/errors, Prometheus metrics, unit/functional tests, and GoBGP Docker interop scenarios.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-peer Multiprotocol capability negotiation and receive family validation | `internal/component/bgp/reactor/session_negotiate.go` (`offeredMultiprotocol`), `session_validation.go` (`validateUpdateFamilies`), `session_read.go` (`processMessage`) | Negotiates only families listed in `session > family` (when present) and refuses unnegotiated MP families on receive with NOTIFICATION 3/9 or `mode ignore` drop | Does not replace; proves, tags (`RFC9552-8.2.3-1`), and documents this existing mechanism as the inbound accept allow-list |
| Outbound negotiated-family write gate | `internal/component/bgp/reactor/session_ownership.go` (`adjOutWrite.writeSection`), `reactor_api_forward.go` (`announceEOR`), `reactor_api_batch.go` | Withholds outbound UPDATE sections and EoRs for families the destination session did not negotiate (`negotiated.SupportsFamily` / `nc.Has`) | Does not replace; proves and tags (`RFC9552-8.2.3-1`) this existing gate as the outbound advertise allow-list |
| Per-direction family policy filter | `internal/component/bgp/plugins/filter_family/config.go` (`parseFamilyFilters`), `handler.go` (`evaluateFamilyFilter`, `judgeFamily`) | Strips or suppresses a registered family on either `import` or `export` chains (`action remove`) or tears down on `import` (`action tear-down`) | Does not replace; proves (`AC-4`) and documents its use for asymmetric BGP-LS accept/advertise control |
| Per-session installed-set prefix accounting and rollback | `internal/component/bgp/reactor/session_prefix.go` (`prefixCounts`, `applyInstalledPrefixSections`, `prefixSetWalk.identity`, `rollbackPrefixSets`, `forEachPrefixEntry`) | Tracks per-session route-identity sets via `nlrisplit.Get` / `GetWithdraw` / `GetPrefixKey` + ADD-PATH ID and rolls back whole-UPDATE mutations on refusal | Extends: tracks ceiling families in installed mode on every session, reserves net new installed NLRIs against the reactor ceiling after per-peer checks pass, rolls back on refusal, and releases on withdrawal/teardown |
| Registered address-family YANG validation | `internal/component/bgp/yang/ze-bgp-conf.yang` (`ze:validate "registered-address-family"`), `internal/core/family` (`LookupFamily`) | Validates family list keys and drives CLI editor completion from the live family registry without hardcoding family names | Extends: reuses `zt:address-family` and `ze:validate "registered-address-family"` on `bgp > prefix-limit > family` |
| Transactional BGP global config load and reload | `internal/component/bgp/reactor/config.go` (`Globals`, `GlobalsFromTree`), `reload_globals.go` (`applyGlobalsJournaled`, `setGlobals`), `internal/component/bgp/config/loader_create.go` (`treeGlobals`, `reloadPeersFromTree`) | Parses `bgp`-level settings once for startup and reload and applies changes via the reload journal with rollback support | Extends: carries `PrefixLimits` inside `Globals` so startup, `VerifyConfig`, and reload update the reactor ceiling transactionally without resetting sessions |
| Operational report bus and BGP prefix metrics | `internal/core/report` (`RaiseWarning`, `ClearWarning`, `RaiseError`), `internal/component/bgp/reactor/session_prefix.go` (`reportCodePrefixThreshold`), `reactor_metrics.go` (`reactorMetrics`) | Publishes BGP prefix warnings/errors to `show warnings`, `show errors`, login banner, and Prometheus `ze_bgp_prefix_*` series | Extends: adds router-ceiling warning/error report codes and `ze_bgp_family_ceiling_*` metrics following the same naming and lifecycle conventions |
| Single session teardown cleanup | `internal/component/bgp/reactor/peer_run.go` (`runOnce` defer calling `clearReportedWarnings`), `session.go` (`Session.Run` defers) | Cleans up per-session timers, connections, and reported warnings once the read goroutine exits | Extends: releases the session's held ceiling counts idempotently on both `Session.Run` exit and `Peer.runOnce` teardown defer |
| BGP-LS receive-path syntactic validation | `internal/component/bgp/message/rfc7606_bgpls_nlri.go` (`judgeBGPLSNLRISection`, `RetainWellFormedNLRI`), `rfc7606_bgpls.go` (`validateBGPLSAttr`) | Validates BGP-LS NLRI and attribute syntax before `checkPrefixLimits` runs | Preserved unchanged; ensures the router ceiling counts only syntactically well-formed Link-State NLRIs |
| Docker BGP interop lab with GoBGP v4.11.0 | `internal/le/interoplab/bgp/` (`names.go`, `checkers.go`), `test/interop/Dockerfile.gobgp` (`gobgp-paths`) | Runs multi-daemon Docker BGP scenarios with declarative scenario directories and typed Go checkers | Extends: adds `bgp-ls-accept-by-family-gobgp` and `bgp-ls-rib-ceiling-gobgp` scenarios and checkers |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "The RFC, then ExaBGP API compatibility, then ExaBGP's behavior, in that order" | agrees | Implements RFC 9552 Section 8.2.3 receiver controls and YANG config without breaking any published `pkg/` API, process protocol, or ExaBGP compatibility contract. |
| `ai/doors/bgp-engine.md` | Boundaries | the door covering `internal/component/bgp/reactor/*.go`, `internal/component/bgp/config/loader_create.go`, and `internal/component/bgp/yang/ze-bgp-conf.yang` | "Never name a plugin package from the reactor." | agrees | The router-wide ceiling lives entirely inside the reactor and is keyed generically by registered `family.Family`; it never imports `bgp-rib`, `nlri/ls`, or any plugin package. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/le/interoplab/bgp/*.go` and `test/plugin/rfc9552-823-*.ci` | "A compiled Go fixture returns an error when an assertion fails" | agrees | New `.ci` tests live in `test/plugin/` and new GoBGP interop assertions live in typed Go checkers in `internal/le/interoplab/bgp/` that return errors on any failed check. |
| `docs/architecture/core-design.md` | 15. Operational Report Bus | `// Design:` page of `session_prefix.go`, `session.go`, `config.go`, `reactor_api.go`, `reactor.go`, `peer_run.go`, `reactor_metrics.go`; anchored to `ze-bgp-conf.yang`; modified by this spec | "Warnings are state-based (condition is currently problematic, may resolve). Errors are event-based (something already happened; no clear API)." | agrees | The router-ceiling warning threshold uses `RaiseWarning` / `ClearWarning` (state-based, cleared when count drops below `warning` or ceiling is removed), while ceiling refusal uses `RaiseError` (event-based per dropped UPDATE), documented in Section 15. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Pure userspace BGP reactor accounting and YANG config; adds no Linux-kernel-only feature and does not affect VPP/kernel backend validation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | No data-plane mismatch rule is touched; invalid `bgp > prefix-limit` entries (`warning >= maximum`, unregistered family) use one validator shared across validate, commit, startup, and reload. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Firewall VPP verification and `ze:backend` annotations are untouched; `bgp > prefix-limit` carries no `ze:backend` annotation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Introduces no data-plane selector or backend value. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Adds no session-level next-hop knob and leaves export-chain next-hop rewriting untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Leaves `policy { modify }` and `next-hop:*` chain references untouched; asymmetric BGP-LS filtering uses existing `policy { family-filter }`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Adds no filter chain step or `FilterInsertion` extension; the router ceiling enforces on ingress inside `checkPrefixLimits` before policy dispatch. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Export-chain next-hop folding and `wireOverride` provenance are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | `policychain.Sets.NextHop` and import/export next-hop validation are untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Originated vs forwarded next-hop preservation is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec inserts no automatic filter-chain member and leaves `filterapi.ImpliedFilter` untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `docs/architecture/core-design.md` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Adds no auxiliary listener or service startup failure path; invalid BGP `prefix-limit` config (`warning >= maximum`, unregistered family) continues to be refused on commit, reload (`VerifyConfig`), and `ze config validate`, while reload stays refused as a whole. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `docs/architecture/core-design.md` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Adds no kernel-only feature or VPP mismatch path; leaves daemon startup behavior untouched. |
| `docs/architecture/api/commands.md` | Login banner integration | anchored to `session_prefix.go`, `config.go`, `reactor_api.go`; modified by this spec | "The Ze CLI login banner reads from the same report bus (filtered by source `bgp`)." | agrees | New router-ceiling threshold warnings and refusal errors publish onto the `bgp` report bus source so they appear in `show warnings`, `show errors`, and the login banner, and are added to the Day-one BGP producers table. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 on `docs/architecture/api/commands.md` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds no CLI command or subcommand; ceiling reports use existing `show warnings` and `show errors` with standard pipes. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 on `docs/architecture/api/commands.md` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Adds or changes no command or pipe operator override. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/commands.md` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Adds or changes no pipe operation. |
| `docs/architecture/behavior/fsm-established.md` | Prefix-limit route identity | anchored to `session_prefix.go`, `session.go` | "If any family refuses the UPDATE, rollback restores every installed set before publication is skipped." | agrees | Router-ceiling reservation runs inside `applyInstalledPrefixSections` after the per-peer check; on over-ceiling refusal, the atomic reservation is undone and `rollbackPrefixSets` restores every installed set before skipping publication and restarting the HoldTimer via `EventUpdateMsg`. |
| `docs/architecture/plugin/rib-storage-design.md` | Adj-RIB-In route keys | anchored to `session_prefix.go` | "Every other family is split by its registered `nlrisplit` splitter and keyed by the family's own decoder, never by reading its octets as an address" | agrees | The router ceiling reuses `prefixSetWalk.identity` (`nlrisplit.Get` / `GetWithdraw` / `GetPrefixKey` + 4-octet ADD-PATH ID) and preserves `rs-client` `prefixCounts.retained` tracking. |
| `docs/architecture/buffer-architecture.md` | Why allocations on the UPDATE path cost | `// Design:` page of `session.go`; anchored to `session.go` | "bounded pools replace `make()`, and the read side stays lazy" | agrees | When no router ceiling is configured, the UPDATE receive path adds zero allocations and zero map lookups (`AC-9`); when configured, reservation adds one atomic operation per section on top of the existing installed-set path without copying UPDATE payloads. |
| `docs/architecture/api/architecture.md` | TL;DR (Read This First) | anchored to `session.go`, `reactor_api.go`, `reactor.go` | - | silent | Anchored for reactor-to-plugin event/API transport; this spec enforces the router ceiling inside the reactor before event dispatch and changes no plugin API or bus wiring. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec adds no implied config line or filter insertion. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Adds no bus participant; `bgp-ls-export`, `bgp-rs`, and `bgp-rib` keep their registered bus participant declarations unchanged. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Does not alter participant `send > to` resolution. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Does not alter participant `receive > from` resolution. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Does not alter bus message allow-lists; BGP-LS neighbor accept/advertise control operates at BGP protocol capability negotiation and policy filters. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Does not alter bus filter compilation or commit-time conflict checks. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Adds no bus arbitration or exclusive-role rule. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Does not alter bus default display. |
| `docs/architecture/behavior/fsm-active.md` | TL;DR | anchored to `session.go` | - | silent | Covers pre-OPEN Active state transitions; this spec acts on Established UPDATE accounting and teardown release only. |
| `docs/architecture/behavior/fsm-connect.md` | TL;DR | anchored to `session.go` | - | silent | Covers TCP Connect state transitions; unaffected by BGP-LS accept proofs or router-wide ceiling accounting. |
| `docs/architecture/behavior/fsm-idle.md` | TL;DR | anchored to `session.go` | - | silent | Covers Idle state entry and startup; unaffected because crossing the router ceiling keeps sessions Established and does not transition to Idle. |
| `docs/architecture/behavior/fsm-open-confirm.md` | TL;DR | anchored to `session.go` | - | silent | Covers OpenConfirm state transitions; unaffected by Established UPDATE ceiling checks. |
| `docs/architecture/behavior/fsm-open-sent.md` | TL;DR | anchored to `session.go` | - | silent | Covers OpenSent state transitions; capability intersection is unchanged and only tested/tagged for BGP-LS. |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | anchored to `session.go`, `reactor_api.go` | - | silent | Describes the BGP FSM state machine; this spec adds no FSM state or event and reuses `EventUpdateMsg` on drop. |
| `docs/architecture/behavior/peer-lifecycle.md` | TL;DR | `// Design:` page of `peer_session_factory.go`; anchored to `session.go`, `reactor_api.go`, `peer_run.go` | "This page covers what happens *around* it: the outer per-peer run loop that creates sessions, retries with exponential backoff, resolves inbound connection races, recovers from panics, and schedules connection retries." | agrees | `Peer.newSessionWithEpoch` wires the reactor's `routerCeiling` onto each newly created `Session`, and `Peer.runOnce`'s teardown defer releases that session's held ceiling counts idempotently without changing reconnect, collision, or backoff behavior. |
| `docs/architecture/congestion-industry.md` | Universal Rule | anchored to `session.go` | - | silent | Covers TCP socket tuning and write-queue congestion; this spec does not touch socket options or outbound congestion handling. |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | anchored to `session.go` | - | silent | Covers RFC 8654 65535-byte message sizing; this spec changes no message size limit or buffer sizing. |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to `session.go` | - | silent | Covers per-peer `EncodingContext` and `ContextID` zero-copy matching; unaffected by router ceiling accounting. |
| `docs/architecture/forward-congestion-pool.md` | Invariant | anchored to `session.go`, `reactor_metrics.go` | - | silent | Covers outbound forwarding buffer pools and overflow sizing; this spec adds no forward pool state or buffer changes. |
| `docs/architecture/memory/lifetime-contracts.md` | TL;DR | anchored to `session.go` | - | silent | Covers received read-buffer ownership and lifetime contracts; the ceiling stores no borrowed read-buffer slices past `checkPrefixLimits` (reusing `prefixSetWalk.identity` keys and clearing `prefixSetJournal` before return). |
| `docs/architecture/message-buffer-design.md` | Overview | anchored to `session.go` | - | silent | Covers message buffer layout and passthrough markers; unaffected by ceiling counting. |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to `session.go`, `ze-bgp-conf.yang` | - | silent | Covers migration of RIB ownership to plugins (`bgp-rib`); the ceiling stays in the reactor beside per-peer prefix limits and introduces no engine Loc-RIB. |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to `session.go` | - | silent | Covers outbound UPDATE wire building; this spec changes no outbound UPDATE wire encoding. |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/update-building.md` | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Does not touch FlowSpec next-hop emission, `flowspec-redirect-next-hop`, or `ze exabgp migrate`. |
| `docs/architecture/config/syntax.md` | Overview | `// Design:` page of `loader_create.go`; anchored to `config.go`, `loader_create.go`, `ze-bgp-conf.yang`; modified by this spec | "The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed." | agrees | `bgp > prefix-limit > family <afi/safi> { maximum; warning; }` uses standard YANG container/list/leaf nodes with no custom `ze:syntax` annotations and is documented in `syntax.md`. |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `config.go` | - | silent | Covers RFC 9234 BGP Role route metadata keys; this spec adds no route metadata and builds no BGP-LS role. |
| `docs/architecture/rfc-may-decisions.md` | RFC 4760 Section 7 - Non-Negotiated AFI/SAFI Handling | anchored to `config.go` | "The UPDATE is DROPPED and the session stays up, so no route of that AFI/SAFI reaches the RIB or the forward rails." | agrees | `AC-2` proves both strict-mode NOTIFICATION 3/9 refusal and `mode ignore` silent drop for unnegotiated `bgp-ls/bgp-ls` exactly as this decision specifies. |
| `docs/architecture/wire/capabilities.md` | 1. Multiprotocol Extensions (Code 1) | anchored to `config.go` | "One capability TLV per address family. Multiple families = multiple capabilities." | agrees | Per-peer `family` allow-listing relies on standard RFC 4760 Code 1 capability negotiation without changing OPEN capability encoding. |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `reactor_api.go` | - | silent | Covers plugin process startup and RPC wire framing; this spec adds no plugin RPC or process-protocol message. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/process-protocol.md` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no engine operation or plugin authorization rule. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 on `docs/architecture/api/process-protocol.md` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Over-ceiling UPDATE refusal happens on the session receive path (`checkPrefixLimits`) before event dispatch, leaving lossless engine-to-plugin event delivery untouched. |
| `docs/architecture/api/text-format.md` | Current Format | anchored to `reactor_api.go` | - | silent | Covers ExaBGP-compatible text API format; this spec changes no text command or event format. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `reactor_api.go`, `loader_create.go` | "All plugins that already applied undo via their journals." | agrees | Reload updates the reactor ceiling inside `applyGlobalsJournaled` (`reload_globals.go`), recording an undo closure that restores the prior ceiling map on rollback. |
| `docs/architecture/wire/nlri-bgpls.md` | Receive-Path NLRI Validation (RFC 9552 Section 8.2.2) | anchored to `reactor_api.go`; modified by this spec | "An NLRI Type this document does not define stays opaque and is propagated whole, because Section 5.2 requires it." | agrees | The router ceiling counts well-formed Link-State NLRIs (including opaque unknown NLRI types via `nlrisplit`) after Section 8.2.2 validation, without performing semantic inspection; a new "Receiver Controls (RFC 9552 Section 8.2.3)" section documents both controls. |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `loader_create.go` | - | silent | Covers multi-plugin config apply ordering; the router ceiling is owned solely by the BGP reactor and introduces no cross-plugin ordering dependency. |
| `docs/architecture/api/update-syntax.md` | Overview | anchored to `ze-bgp-conf.yang` | - | silent | Covers `update { ... }` route announcement syntax; unaffected by `bgp > prefix-limit`. |
| `docs/architecture/bgp/filter-path-asn.md` | The config | anchored to `ze-bgp-conf.yang` | - | silent | Covers `policy > path-asn` filter configuration; unaffected by `bgp > prefix-limit`. |
| `docs/architecture/config/environment-block.md` | TL;DR | anchored to `ze-bgp-conf.yang` | - | silent | Covers `environment { ... }` config block; this spec adds no environment variable. |
| `docs/architecture/config/environment.md` | Overview | anchored to `ze-bgp-conf.yang` | - | silent | Covers `ze.*` environment variable registry; the ceiling is YANG configuration, not an env var. |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | anchored to `ze-bgp-conf.yang` | "YANG defines format. Extensions declare behavior. Implementation executes behavior." | agrees | Declares `bgp > prefix-limit > family` in `ze-bgp-conf.yang` using `zt:address-family`, `ze:validate "registered-address-family"`, and `uint32 range "1..max"`, with `GlobalsFromTree` enforcing `warning < maximum`. |
| `docs/plugin-development/metrics.md` | Naming Policy | modified by this spec | "Every metric name is built from four parts in fixed order:" | agrees | Adds `ze_bgp_family_ceiling_count`, `ze_bgp_family_ceiling_maximum`, `ze_bgp_family_ceiling_warning_exceeded` (gauges, label `family`), and `ze_bgp_family_ceiling_refused_total` (counter, labels `peer, family`) to the inventory, obeying every naming and label rule. |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | `// Design:` page of `internal/le/interoplab/bgp/names.go` and `checkers.go`; modified by this spec | "A scenario directory carries only declarative inputs its runner reads" | agrees | `bgp-ls-accept-by-family-gobgp` and `bgp-ls-rib-ceiling-gobgp` hold only declarative configs/inputs under `test/interop/scenarios/`; all assertions live in typed Go checkers in `internal/le/interoplab/bgp/`. |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/testing/interop.md` | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | Both new BGP-LS GoBGP scenarios are pure userspace BGP tests requiring no extra kernel capabilities. |
| `docs/architecture/ospf/ospf-ext-16-ipsec-auth.md` | Decisions | anchored to `internal/le/interoplab/bgp/checkers.go` | - | silent | Anchored to `checkers.go` only for `ospfIPsecPeerSetup`; this spec adds BGP-LS scenario entries to `scenarioOperations` and does not touch OSPFv3 IPsec authentication. |
| `docs/architecture/api/capability-contract.md` | TL;DR | anchored to `reactor.go` | - | silent | Covers plugin-injected OPEN capabilities and BGP cache control; this spec adds no capability injection or cache contract change. |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `reactor.go` | - | silent | Covers plugin IPC transports and wire framing; this spec adds no plugin RPC or IPC message. |
| `docs/architecture/behavior/signals.md` | Supported Signals | anchored to `reactor.go` | - | silent | Covers POSIX signal handling (`SIGTERM`, `SIGHUP`, `SIGALRM`); reload reuses `applyGlobalsJournaled` without altering signal dispatch. |
| `docs/architecture/bgp/interface-event-reactions.md` | The decisions | anchored to `reactor.go` | - | silent | Covers reactor reactions to kernel interface and neighbor address changes; unaffected by router-wide prefix ceilings. |
| `docs/architecture/bgp/interface-event-reactions.md` | Design decisions | decision of 2026-11-22 on `docs/architecture/bgp/interface-event-reactions.md` | "Interface and neighbor change re-advertisement runs the first flip immediately, then coalesces bursts behind a per-peer cooldown with at most one queued trailing re-pass; BGP route flap dampening (RFC 2439) is a separate concept and, if ever implemented, must be an option off by default ("flap dampening should be an option and off by default as since the RFC it was found to not help. 1A is fine.")" | agrees | Does not alter interface/neighbor re-advertisement coalescing or introduce RFC 2439 route flap dampening. |
| `docs/architecture/overview.md` | 1. What is Ze? | anchored to `reactor.go` | - | silent | High-level system overview; unaffected by adding a BGP reactor ceiling option. |
| `docs/architecture/plugin/plugin-system.md` | Where each layer lives | anchored to `reactor.go`, `peer_run.go` | - | silent | Covers plugin registration, startup stages, and process boundary; the router ceiling lives inside the BGP reactor component and adds no plugin or cross-process call. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/plugin/plugin-system.md` | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | `bgp > prefix-limit` lives in the built-in `ze-bgp-conf.yang` schema and validates via `GlobalsFromTree` and `registered-address-family`, leaving external-program query-mode validation unchanged. |
| `docs/guide/config-reload.md` | What Can Change Live | `// Design:` page of `reload_globals.go` | "If reconciliation fails, static and dynamic reload transactions report the error and restore the previous settings" | agrees | Updating `bgp > prefix-limit` reloads live via `applyGlobalsJournaled` without resetting sessions or evicting routes, and restores the prior ceiling map on rollback. |
| `docs/architecture/bgp/protocol-event-capture.md` | Enabling it | anchored to `peer_run.go` | - | silent | Covers per-peer JSONL protocol event capture; unaffected by `Peer.runOnce`'s `releaseRouterCeiling` defer. |
| `docs/architecture/diagnostics/event-history-rings.md` | The decisions | anchored to `peer_run.go` | - | silent | Covers per-peer FSM history ring buffers; unaffected by router ceiling teardown release. |
| `docs/architecture/mrt.md` | Packages | anchored to `peer_run.go` | - | silent | Covers MRT archive/dump plugin and library; unaffected by router ceiling accounting. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `reactor_metrics.go` | "The Prometheus implementation should continue to use the existing push-model `metrics.Registry` path." | agrees | The four new `ze_bgp_family_ceiling_*` metrics register on `metrics.Registry` inside `reactorMetrics` (`reactor_metrics.go`) using the existing push-model gauges and counters, introducing no pull collector. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A BGP UPDATE carrying MP_REACH_NLRI or MP_UNREACH_NLRI with AFI 16388 arrives on an established session, in wire bytes.
- Operator configuration (`bgp > prefix-limit > family <afi/safi> { maximum N; warning M; }` and per-peer `session > family` / `policy > family-filter`) arrives as a YANG-modeled config tree at startup (`CreateReactorFromTree`) or reload (`VerifyConfig` / `ApplyConfigDiff` / `Reload`).

### Transformation Path
1. OPEN: the per-peer (or per-group) `session > family` list becomes RFC 4760 Multiprotocol capabilities (`offeredMultiprotocol`, `session_negotiate.go`); intersection with the peer's OPEN admits a neighbor to BGP-LS in both directions.
2. Receive family gate: `validateUpdateFamilies` (`session_validation.go`, called from `session_read.go:548`) refuses an UPDATE of a non-negotiated family with NOTIFICATION 3/9 (or drops it silently with `EventUpdateMsg` HoldTimer restart under `mode ignore` / `ignore-mismatch enable`).
3. Receive syntax validation: `enforceRFC7606` (`judgeBGPLSNLRISection`, `RetainWellFormedNLRI`, `validateBGPLSAttr`) validates framing and TLV syntax before prefix accounting so only well-formed Link-State NLRIs are counted.
4. Receive prefix accounting: `checkPrefixLimits` -> `applyInstalledPrefixSections` (`session_prefix.go`) checks the per-peer `prefix maximum` first (preserving its NOTIFICATION/drop semantics), then for any family covered by the reactor's `routerCeiling` atomically reserves the net new installed NLRI delta of the message across all sessions. If the atomic reservation exceeds the ceiling's `maximum`, it immediately undoes the reservation, rolls back the session's installed sets (`rollbackPrefixSets`), drops the UPDATE (keeping the session up and restarting the HoldTimer via `EventUpdateMsg`), logs, raises an operational error report, and increments `ze_bgp_family_ceiling_refused_total`.
5. Import/export policy and outbound send gate: `policy { family-filter }` (`action remove`) in a peer's `import` or `export` chain narrows an admitted BGP-LS peer to one direction; outbound writes pass `negotiated.SupportsFamily` (`session_ownership.go:900`) so non-negotiating peers receive no BGP-LS frames.
6. Storage and teardown: `bgp-rib` and `bgp-rs` receive only UPDATEs that survived step 4 and step 5. On withdrawal, the session releases the withdrawn count from the reactor ceiling; on session close (any close path), `Session.releaseRouterCeiling()` releases the session's remaining held count idempotently from both `Session.Run` exit defer and `Peer.runOnce` teardown defer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ reactor | per-peer `family` list (`parseFamiliesFromTree`, existing); router ceiling (`bgp > prefix-limit > family`) through `Globals` / `GlobalsFromTree` at load (`loader_create.go`) and reload (`reload_globals.go`, `reactor_api.go`) | Yes for `family` list and `Globals` load/reload wiring; new field parsing verified in Phase 1 |
| Session ↔ reactor-wide state | each session holds a pointer to the reactor's `routerCeiling` (wired in `Peer.newSessionWithEpoch`), reserving and releasing against one atomic counter per ceiling family | New (Phase 1/3) |
| Reactor ↔ plugins | unchanged: a refused UPDATE is dropped before callback dispatch, so `bgp-rib` and `bgp-rs` never see it | Yes (`checkPrefixLimits` runs before dispatch at `session_read.go:593`) |

### Changed-Wire Census

No BGP wire format, capability TLV, NOTIFICATION code/subcode, plugin RPC payload, or CLI/JSON response wire schema changes in this spec.

**Search evidence:** Inspected `internal/component/bgp/reactor/session_prefix.go`, `session_read.go`, `session_validation.go`, `session_ownership.go`, `config.go`, `reload_globals.go`, and `internal/component/bgp/yang/ze-bgp-conf.yang` at HEAD. All BGP-LS wire framing (`internal/core/bgp/nlri/bgpls/`), RFC 4760 Section 7 NOTIFICATION 3/9 (`session_read.go:558`), RFC 4486 NOTIFICATION 6/1 (`session_prefix.go:1045`), and plugin event payloads remain byte-for-byte identical. The only new external surface is the additive YANG config container `bgp > prefix-limit` and four Prometheus metrics, neither of which alters an existing wire format.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| BGP UPDATE / OPEN / NOTIFICATION wire bytes | writer & reader (unchanged) | `internal/component/bgp/reactor/session_read.go` (`processMessage`), `session_validation.go` (`validateUpdateFamilies`), `session_ownership.go` (`adjOutWrite.writeSection`) | Verified at HEAD: crossing the router ceiling drops the UPDATE with no NOTIFICATION; non-negotiated family refusal keeps existing RFC 4760 Section 7 NOTIFICATION 3/9 | Preserve unchanged; proved by unit tests, `.ci` suites, and GoBGP Docker interop |

### Integration Points
- `applyInstalledPrefixSections` and `rollbackPrefixSets` (`session_prefix.go`) - ceiling reservation runs after per-peer check passes; over-ceiling undoes the reservation and rolls back per-session sets; withdrawals release.
- `newPrefixCounts` and `Session.attachRouterCeiling` (`session_prefix.go`, `peer_session_factory.go`) - keeps `newPrefixCounts(settings)` and `NewSession(settings)` signatures unchanged; attaching a non-empty `routerCeiling` enables installed-set tracking for every ceiling family on that session (and handles families added at reload dynamically on next UPDATE).
- `Globals` / `GlobalsFromTree` (`config.go`), `treeGlobals` / `reloadPeersFromTree` (`loader_create.go`), and `applyGlobalsJournaled` / `setGlobals` (`reload_globals.go`) - carry and transactionally update the router ceiling map.
- `Session.Run` (`session.go`) and `Peer.runOnce` (`peer_run.go`) - idempotent `releaseRouterCeiling()` on session exit releases all held counts for that session.
- `reactorMetrics` (`reactor_metrics.go`) and `internal/core/report` (`session_prefix.go` / `router_ceiling.go`) - publish `ze_bgp_family_ceiling_*` metrics and `bgp/family-ceiling-threshold` (warning) + `bgp/family-ceiling-exceeded` (error) reports.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | ceiling sits beside the per-peer maximum inside `checkPrefixLimits`, after RFC 7606 validation and before plugin dispatch |
| No unintended coupling (components stay isolated) | Yes (design) | no plugin package is imported; the reactor owns the counter and reads only `family.Family` and `nlrisplit` |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | accept/advertise reuses negotiation and `family-filter`; the ceiling extends `prefixCounts` installed-set accounting and `Globals` reload |
| Zero-copy preserved where applicable (refs, not copies) | Yes (design) | reuses `prefixSetWalk.identity` keys and stack `prefixSection` array; zero work when no ceiling is configured |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes (design) | ceiling is keyed by any registered `family.Family` (`uint32` `familyKey`); no BGP-LS-specific field, switch case, or branch is added anywhere in the reactor or YANG |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes (design) | `bgp > prefix-limit > family` reuses `zt:address-family` and `ze:validate "registered-address-family"` (deriving validation and CLI completion from `internal/core/family`); no family list is hardcoded |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The reactor (not `bgp-rib`) is the sole owner that can enforce a router-wide Link-State ceiling before dispatch and forwarding | verified at HEAD: `bgp-rib` sees routes only after dispatch and does not gate `bgp-rs` forwarding; `checkPrefixLimits` runs before callback dispatch (`session_read.go:593`) | ceiling would leave RIB and forwarded routes disagreeing | read `session_prefix.go`, `session_read.go`, `familyrib_opaque.go` | confirmed |
| A-2 | `Globals` is re-read at reload and can update running reactor state transactionally without resetting peers | verified at HEAD: `reloadPeersFromTree` (`loader_create.go:340`) and `loadPeersFullOrTree` (`reactor_api.go:1087`) call `GlobalsFromTree`; `reconcilePeersJournaled` (`reactor_api.go:1148`) calls `applyGlobalsJournaled` (`reload_globals.go:22`) before diffing peers | reload would require peer restarts or fail to update live ceiling | read `loader_create.go`, `reactor_api.go`, `reload_globals.go`; pinned by `TestRouterCeilingLoweredAtReloadEvictsNothing` | confirmed |
| A-3 | The outbound path withholds any family a destination session did not negotiate, so the per-peer `family` block governs "advertised to" as well as "accepted from" | verified at HEAD: `session_ownership.go:900` checks `negotiated.SupportsFamily(fam)` on every outbound section (relayed, forwarded, originated, raw); `reactor_api_forward.go:143` and `reactor_api_batch.go:740,1342` check `nc.Has(fam)` | advertise half of `8.2.3-1` would need a new gate | read `session_ownership.go:900`, `reactor_api_forward.go:143`; discriminated by `rfc9552-823-advertise-by-family.ci` | confirmed |
| A-4 | GoBGP v4.11.0 (`test/interop/Dockerfile.gobgp`) can negotiate `ls` and serve as the third-party peer for both interop scenarios | `test/interop/Dockerfile.gobgp` already ships GoBGP v4.11.0 with `gobgp-paths` reading AFI 16388 / SAFI 71; whether GoBGP CLI/API injects BGP-LS NLRIs directly or needs a raw injector alongside GoBGP (as `bgp-sr-policy-malformed-receive-gobgp` does) is verified in Phase 4 | scenarios use GoBGP as third-party peer for capability/RIB state paired with raw BGP-LS injection if GoBGP CLI cannot originate LS | Phase 4: test GoBGP `ls` negotiation and injection in the Docker lab | unvalidated |
| A-5 | Every session's held ceiling counts can be released whole and idempotently on teardown at `Session.Run` exit and `Peer.runOnce` defer | verified at HEAD: `Session.Run` (`session.go:1476-1508`) runs exit defers on every read-loop exit (including panic containment), and `Peer.runOnce` (`peer_run.go:558-560`) runs teardown cleanup after `Session.Run` returns | a close path skipping release would leak the ceiling counter | Phase 3: idempotent release guard + `TestRouterCeilingReleasesOnSessionTeardown` covering each close-path class | confirmed (code path); tested in Phase 3 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Duplicate per-peer BGP-LS accept mechanism from sibling spec | resolved: `plan/release/fixes/spec-bgp-ls-receiver-fault-management.md` dropped the `consumer-facing` role (2026-11-24 owner decision) | this spec builds no role; `family` block + `family-filter` remain the sole controls |
| R-2 | Router-wide counter leaks on a session teardown path (NOTIFICATION, hold-timer, GR, BFD down, collision, admin stop, reload restart, panic) and permanently refuses later NLRIs | `ze_bgp_family_ceiling_count` stays above zero with all sessions down | idempotent `Session.releaseRouterCeiling()` in both `Session.Run` exit defer and `Peer.runOnce` teardown defer; `TestRouterCeilingReleasesOnSessionTeardown` tests every close-path class and asserts count returns to zero |
| R-3 | Two sessions race past the ceiling between check and increment | count exceeds `maximum` under concurrent UPDATEs | atomic CAS loop (`reserve`) checks `current + delta <= maximum` and commits in one atomic step; `TestRouterCeilingConcurrentReserve` runs under `-race` |
| R-4 | Tracking installed sets on every session for a ceiling family uses memory proportional to each session's held keys for that family | RSS growth after enabling a ceiling on a large feed | only families explicitly configured in `bgp > prefix-limit` pay (`AC-9`); cost is documented in YANG description and `docs/guide/configuration.md` |
| R-5 | Graceful Restart keeps stale routes in `bgp-rib` while the restarted `Session` starts with an empty installed set, under-counting during the restart window | ceiling count drops during GR restart while `bgp-rib` retains stale paths | accepted by owner decision (2026-10-13) as a documented Known Limitation |
| R-6 | Dropping the crossing UPDATE while keeping sessions up desynchronizes Ze's view from the peer that sent it | operator sees missing routes with session still Established | accepted by owner decision (2026-10-13), matching per-peer `teardown false`; operational error report names peer, family, count, and maximum, and `ze_bgp_family_ceiling_refused_total` increments |
| R-7 | Lowering `maximum` at reload below the current router-wide count evicts nothing, leaving count above the new bound until withdrawals or teardowns occur | `ze_bgp_family_ceiling_count > ze_bgp_family_ceiling_maximum` after reload | documented: ceiling refuses new NLRIs and never evicts (matching FRR `maximum-prefix` and per-peer limits); pinned by `AC-8` |
| R-8 | A peer with NO `family` block still advertises BGP-LS via `offeredMultiprotocol` | a peer without a `family` block negotiates and receives BGP-LS | accepted by owner decision (2026-10-13): `AC-1`..`AC-4` and `RFC9552-8.2.3-1` state the explicit-`family`-block precondition; the fallback fix belongs to `plan/release/fixes/spec-bgp-one-off-defects.md` |
| R-9 | Adding a ceiling family at reload while sessions are already Established leaves existing sessions without an installed set for already-held NLRIs | reload enables a new ceiling family on live sessions; pre-existing NLRIs from before reload were not in `prefixCounts.sets` | lazy set creation on first UPDATE of a ceiling family tracks all subsequent arrivals cleanly; documented in `docs/guide/configuration.md` that enabling a new ceiling family on an already-established session counts newly received NLRIs until session reset or refresh |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A leaked or racy router ceiling counter refuses BGP-LS (or any ceiling-configured family) from every peer while sessions remain Established, silently halting topology updates |
| How is it reverted? | Single commit revert; `bgp > prefix-limit` defaults to unset and the accept/advertise proof adds no runtime code |
| Who else touches this path? | `session_prefix.go` is the per-peer prefix-limit path for every family; `reload_globals.go` handles BGP global reloads; `plan/release/fixes/spec-bgp-ls-receiver-fault-management.md` touches only `.ci` / GoBGP interop for §8.2.2 (no file overlap on reactor code) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a peer whose `family` block omits `bgp-ls/bgp-ls` sends a Link-State NLRI | → | `validateUpdateFamilies` refusal (`session_validation.go`) and `processMessage` (`session_read.go`) | `TestRFC9552UnlistedNeighborsLinkStateNLRIIsNotAccepted` |
| Ze relays BGP-LS and one destination's `family` block omits `bgp-ls/bgp-ls` | → | `adjOutWrite.writeSection` negotiated-family gate (`session_ownership.go`) | `test/plugin/rfc9552-823-advertise-by-family.ci` <!-- doc-links: ignore (planned test this open spec has not built yet) --> |
| config `bgp { prefix-limit { family bgp-ls/bgp-ls { maximum N; } } }` | → | `GlobalsFromTree` → `CreateReactorFromTree` / `applyGlobalsJournaled` → `Peer.newSessionWithEpoch` → `Session.attachRouterCeiling` | `TestRouterCeilingConfigReachesEverySession` |
| router holds `maximum` installed Link-State NLRIs across two peers and one more arrives | → | `checkPrefixLimits` → `applyInstalledPrefixSections` → `routerCeiling.reserve` | `TestRFC9552RouterCeilingRefusesTheNextLinkStateNLRI` |
| operator runs live `ze` daemon with `bgp > prefix-limit` against two BGP-LS peers | → | full config-to-session-to-report/metric receive path | `test/plugin/rfc9552-823-rib-ceiling.ci` <!-- doc-links: ignore (planned test this open spec has not built yet) --> |

## Acceptance Criteria

"Peer with a `family` block" is a precondition of AC-1..AC-4 because of the no-family-block fallback owned by `plan/release/fixes/spec-bgp-one-off-defects.md` (R-8).

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A peer whose `family` block names `bgp-ls/bgp-ls` sends a valid Link-State NLRI | Ze negotiates `bgp-ls/bgp-ls`, accepts the UPDATE, delivers it to plugins/RIB, and retains the Link-State NLRI in the RIB (`TestRFC9552ListedNeighborsLinkStateNLRIIsAccepted`, `rfc9552-823-accept-by-family.ci`) | None: every valid Link-State NLRI from a listed peer is accepted under default policy |
| AC-2 | A peer whose `family` block omits `bgp-ls/bgp-ls` sends a Link-State NLRI | Ze omits `bgp-ls/bgp-ls` from OPEN negotiation; in strict mode Ze sends NOTIFICATION 3/9 (`UPDATE Message Error` / `Optional Attribute Error`, RFC 4760 Section 7) and closes the session; under `mode ignore` or `ignore-mismatch enable` Ze drops the UPDATE, restarts the HoldTimer, keeps the session Established, and dispatches zero Link-State NLRIs (`TestRFC9552UnlistedNeighborsLinkStateNLRIIsNotAccepted`, `rfc9552-823-accept-by-family.ci`) | Non-BGP-LS negotiated families (such as `ipv4/unicast`) remain operational under `mode ignore` because only the unnegotiated UPDATE is dropped |
| AC-3 | Ze holds BGP-LS (relayed or exported) and has two Established destinations, one whose `family` block names `bgp-ls/bgp-ls` and one whose omits it | Ze advertises the Link-State NLRI to the first destination and withholds AFI 16388 MP_REACH frames from the second while continuing to deliver its negotiated families (`rfc9552-823-advertise-by-family.ci`) | Negotiated non-BGP-LS families (such as `ipv4/unicast` control routes) continue to be advertised to the second destination to prove the session is alive and non-vacuous |
| AC-4 | A peer negotiates `bgp-ls/bgp-ls` and has `policy { family-filter }` (`action remove` on `bgp-ls/bgp-ls`) in its `import` chain only (and a mirror peer in its `export` chain only) | On the import-filtered peer, Ze advertises outbound BGP-LS to it while suppressing inbound BGP-LS from it; on the export-filtered peer, Ze accepts inbound BGP-LS into the RIB while suppressing outbound BGP-LS to it (`rfc9552-823-accept-by-family.ci`) | Other negotiated families on the same peers remain unaffected because `family-filter` strips or suppresses only `bgp-ls/bgp-ls` |
| AC-5 | `bgp { prefix-limit { family bgp-ls/bgp-ls { maximum N; warning M; } } }` is configured and active sessions together hold N installed Link-State NLRIs | Up to N installed NLRIs across peers are accepted and delivered (`TestRFC9552RouterCeilingAcceptsUpToTheMaximum`); crossing `warning` M raises `bgp/family-ceiling-threshold` and sets `ze_bgp_family_ceiling_warning_exceeded{family}` to 1; the next UPDATE carrying a new Link-State NLRI from any peer is dropped before dispatch, all sessions remain Established with zero NOTIFICATIONs, the session's installed set is rolled back, an error report (`bgp/family-ceiling-exceeded`) names peer, family, count, and maximum, and `ze_bgp_family_ceiling_refused_total{peer,family}` increments (`TestRFC9552RouterCeilingRefusesTheNextLinkStateNLRI`, `rfc9552-823-rib-ceiling.ci`) | Previously accepted N Link-State NLRIs remain stored and sessions remain Established per the 2026-10-13 owner decision |
| AC-6 | At router-wide ceiling N, a peer re-announces a Link-State NLRI it already holds, or withdraws a Link-State NLRI it holds | Re-announcement is accepted with net delta 0 and delivered; withdrawal is accepted, decrements the router-wide count to N-1 (clearing `bgp/family-ceiling-threshold` when below `warning`), and the next new Link-State NLRI from any peer is accepted (`TestRouterCeilingCountsInstalledNotOffered`) | Unmatched withdrawals for NLRIs the peer never held leave the set and router-wide count unchanged, matching `count installed` semantics |
| AC-7 | A session holding K ceiling-tracked NLRIs closes via any close path (NOTIFICATION sent/received, hold timer expiry, TCP EOF/FIN, BFD down, collision close, admin stop, reload restart, panic) | The router-wide count drops by exactly K and reaches 0 once all sessions holding entries are down, with repeated teardown calls remaining idempotent (`TestRouterCeilingReleasesOnSessionTeardown`) | Counts held by other still-Established sessions remain intact; during Graceful Restart, stale paths in `bgp-rib` are outside reactor accounting per Known Limitation R-5 |
| AC-8 | Config reload lowers `bgp > prefix-limit > family <f> > maximum` below the current router-wide count, or rolls back a failed reload | On committed reload, existing sessions stay Established, zero routes are evicted, `ze_bgp_family_ceiling_maximum{family}` reflects the new bound, and new NLRIs are refused until withdrawals or teardowns bring the count below the new maximum; on reload rollback, the prior ceiling is restored (`TestRouterCeilingLoweredAtReloadEvictsNothing`) | Already-installed NLRIs above the lowered bound survive until explicitly withdrawn or their session closes (R-7) |
| AC-9 | No `bgp > prefix-limit` is configured | No session allocates an installed set it did not already allocate for per-peer `count installed` / `rs-client` `retained`, and all existing prefix-limit tests pass unchanged (`TestNoRouterCeilingAddsNoInstalledSet`, `TestRouterCeilingConfigReachesEverySession`) | Sessions that explicitly configure per-peer `prefix { count installed; }` or `rs-client` retain their existing sets |
| AC-10 | Both a per-peer `prefix { maximum P; }` and a router-wide `bgp > prefix-limit { family <f> { maximum N; } }` apply on a session | The per-peer maximum runs first with its configured action (NOTIFICATION 6/1 teardown or per-peer drop); the router ceiling is checked only for messages the per-peer check accepted, and a per-peer refusal leaves the router ceiling count unchanged (`TestPerPeerMaximumIsCheckedBeforeRouterCeiling`) | None: strict precedence with rollback guarantees neither counter drifts on refusal |
| AC-11 | `bgp > prefix-limit` names an unregistered family, `maximum 0`, or `warning >= maximum` | Config validation, startup, and reload refuse the configuration with the `registered-address-family` / range / warning-below-maximum error (`TestRouterCeilingRefusesUnregisteredFamily`) | None: invalid ceiling configurations are never applied |
| AC-12 | Two Established sessions concurrently announce new Link-State NLRIs while the router-wide count is at N-1 | Under `-race`, the atomic reservation admits exactly one new NLRI (bringing count to N) and refuses the second without exceeding N or leaving leaked counts (`TestRouterCeilingConcurrentReserve`) | Which of the two racing peers wins the Nth slot is nondeterministic; total admitted count is strictly N |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Lists `bgp-ls/bgp-ls` only on designated collector/producer peers (and optionally adds `family-filter` for one-direction control) | config → OPEN Multiprotocol negotiation → receive `validateUpdateFamilies` / outbound `SupportsFamily` gate / `family-filter` | `rfc9552-823-accept-by-family.ci`, `rfc9552-823-advertise-by-family.ci`, `bgp-ls-accept-by-family-gobgp` |
| 2 | Configures `bgp { prefix-limit { family bgp-ls/bgp-ls { maximum N; warning M; } } }` to bound router-wide Link-State storage | config → `GlobalsFromTree` → reactor `routerCeiling` → `Session` reserve/release → report bus & Prometheus metrics | `rfc9552-823-rib-ceiling.ci`, `bgp-ls-rib-ceiling-gobgp` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC9552ListedNeighborsLinkStateNLRIIsAccepted` | `internal/component/bgp/reactor/rfc9552_accept_by_family_test.go` | AC-1, tag `RFC9552-8.2.3-1 positive` | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRFC9552UnlistedNeighborsLinkStateNLRIIsNotAccepted` | `internal/component/bgp/reactor/rfc9552_accept_by_family_test.go` | AC-2, tag `RFC9552-8.2.3-1 negative` | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingConfigReachesEverySession` | `internal/component/bgp/reactor/router_ceiling_test.go` | wiring, startup + reload propagation | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRFC9552RouterCeilingAcceptsUpToTheMaximum` | `internal/component/bgp/reactor/rfc9552_router_ceiling_test.go` | AC-5, tag `RFC9552-8.2.3-3 positive` | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRFC9552RouterCeilingRefusesTheNextLinkStateNLRI` | `internal/component/bgp/reactor/rfc9552_router_ceiling_test.go` | AC-5, tag `RFC9552-8.2.3-3 negative` | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingCountsInstalledNotOffered` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-6 (re-announce, withdraw, unmatched withdraw, warning raise/clear) | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingReleasesOnSessionTeardown` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-7, subtests per close-path class + idempotent double-release | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingLoweredAtReloadEvictsNothing` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-8 (lowered ceiling + reload rollback restoration) | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestNoRouterCeilingAddsNoInstalledSet` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-9 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestPerPeerMaximumIsCheckedBeforeRouterCeiling` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-10 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingRefusesUnregisteredFamily` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-11 (unregistered family, `maximum 0`, `warning >= maximum`) | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestRouterCeilingConcurrentReserve` | `internal/component/bgp/reactor/router_ceiling_test.go` | AC-12 (under `-race`) | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->

### RFC-tagged tests this spec changes (owner approval)

**None planned.** Every tag added is new, in a new test file. Existing tagged
tests on the touched path (`rfc9552_prefix_limit_test.go`,
`rfc4271_prefix_limit_cease_peer_test.go`, and tagged tests in
`session_prefix_test.go`) compile and pass untouched because `NewSession` and
`newPrefixCounts` retain their signatures. If implementation ever requires
editing an existing tagged test, stop and follow `test/rfc-changed.md`
(`ai/rules/testing.md`).

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `bgp > prefix-limit > family > maximum` | 1 - 4294967295 (`uint32`, `range "1..max"`) | 4294967295 | 0 (refused by YANG `range` and `GlobalsFromTree`) | 4294967296 (refused by `uint32`) |
| `bgp > prefix-limit > family > warning` | 0 - `maximum` - 1 (defaults to `maximum * 9 / 10` when omitted) | `maximum` - 1 | N/A (`uint32`) | `maximum` (refused by `GlobalsFromTree`) |
| Router-wide installed count at ceiling | N-1, N, N+1 | N accepted | N/A (clamped / invariant >= 0) | N+1 refused (UPDATE dropped, count stays N) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc9552-823-accept-by-family` | `test/plugin/rfc9552-823-accept-by-family.ci` | `le test peer` speakers against live `ze`: listed peer's BGP-LS reaches RIB (AC-1); unlisted peer is refused per AC-2 (both strict NOTIFICATION 3/9 and `mode ignore` drop); negotiated peer with `family-filter` `remove` on import receives BGP-LS and contributes none, and mirror on export contributes and receives none (AC-4) | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `rfc9552-823-advertise-by-family` | `test/plugin/rfc9552-823-advertise-by-family.ci` | live `ze` relays a Link-State NLRI plus an IPv4 unicast control route; destination naming `bgp-ls/bgp-ls` receives both, destination omitting `bgp-ls/bgp-ls` receives the IPv4 control route and zero AFI 16388 MP_REACH frames (AC-3) | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `rfc9552-823-rib-ceiling` | `test/plugin/rfc9552-823-rib-ceiling.ci` | live `ze` with `bgp { prefix-limit { family bgp-ls/bgp-ls { maximum 3; warning 2; } } }`; peer A sends 2, peer B sends 2; B's second is dropped while both sessions stay Established, RIB holds 3, `show warnings` / `show errors` and `ze_bgp_family_ceiling_*` metrics reflect the state; A withdraws 1 and B's resend is accepted (AC-5, AC-6) | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-ls-accept-by-family-gobgp` | `test/interop/scenarios/bgp-ls-accept-by-family-gobgp/` | GoBGP v4.11.0 (Docker) | GoBGP configured with `ls` negotiates BGP-LS with the Ze peer naming `bgp-ls/bgp-ls` and not with the Ze peer omitting it (read from GoBGP's neighbor state); Link-State NLRI reaches Ze's RIB only via the listed peer while IPv4 unicast control proves both sessions are up (A-4) | | <!-- doc-links: ignore (directory this open spec plans and has not created yet) -->
| `bgp-ls-rib-ceiling-gobgp` | `test/interop/scenarios/bgp-ls-rib-ceiling-gobgp/` | GoBGP v4.11.0 (Docker) | N+1 Link-State NLRIs delivered against Ze's `bgp > prefix-limit` ceiling N; session stays Established with zero NOTIFICATIONs on GoBGP's side, Ze holds N, and withdrawal + resend admits the replacement (A-4) | | <!-- doc-links: ignore (directory this open spec plans and has not created yet) -->

## Design

### Router-Wide Ceiling Mechanics (C3)

| Component | Behavior |
|-----------|----------|
| YANG schema (`ze-bgp-conf.yang`) | Under `container bgp`, add `container prefix-limit` containing `list family` keyed by `leaf name` (`type zt:address-family; ze:validate "registered-address-family";`), with mandatory `leaf maximum` (`type uint32 { range "1..max"; }`, description citing the IETF BGP-LS YANG draft's `max-number` as the external name) and optional `leaf warning` (`type uint32;`, default 90% of `maximum`, refused when `>= maximum`). |
| Config parsing & reload (`config.go`, `loader_create.go`, `reload_globals.go`, `reactor_api.go`) | `Globals` gains `PrefixLimits map[string]RouterPrefixLimit` (each holding `Maximum` and `Warning`). `GlobalsFromTree` parses and validates `bgp > prefix-limit > family`, refusing unregistered families, `maximum == 0`, and `warning >= maximum`. `CreateReactorFromTree` passes `globals.PrefixLimits` in `reactor.Config`; `applyGlobalsJournaled` updates `Reactor.routerCeiling` in place without resetting peers and records the previous map in the reload journal for rollback. |
| Reactor counter (`router_ceiling.go`) | `routerCeiling` holds per-`uint32` `familyKey` entries protected for config swaps (`sync.RWMutex` or atomic snapshot) with an `atomic.Int64` current count, `atomic.Uint32` maximum and warning, and `atomic.Bool` warned state. `reserve(fk, delta)` uses an atomic CAS loop: if `delta <= 0` it adds `delta` (clamped at 0); if `delta > 0` it succeeds only when `current + delta <= maximum`. |
| Session accounting (`session_prefix.go`, `session.go`, `peer_session_factory.go`, `peer_run.go`) | `Peer.newSessionWithEpoch` calls `session.attachRouterCeiling(p.reactor.routerCeiling)`. `checkPrefixLimits` enters `applyInstalledPrefixSections` whenever a session has per-peer `installed`/`retained` families OR `s.routerCeiling` has active limits. For each ceiling-tracked family in the UPDATE, after per-peer limits succeed, the session computes the net delta against its own `ceilingHeld[fk]` (per-session map of held count per ceiling family, accessed only on the read goroutine) and calls `reserve(fk, delta)`. On over-ceiling refusal, all reservations from this UPDATE are undone, `rollbackPrefixSets` restores every per-session set, `EventUpdateMsg` restarts the HoldTimer, `reportRouterCeilingExceeded` logs and raises `bgp/family-ceiling-exceeded` + increments `ze_bgp_family_ceiling_refused_total`, and the UPDATE is dropped with `notif == nil, drop == true`. On `Session.Run` exit defer and `Peer.runOnce` teardown defer, idempotent `releaseRouterCeiling()` subtracts each remaining `ceilingHeld[fk]` entry from `routerCeiling` and clears the map. |
| Observability (`reactor_metrics.go`, `router_ceiling.go`) | Four metrics obeying `docs/plugin-development/metrics.md`: `ze_bgp_family_ceiling_count{family}` (GaugeVec), `ze_bgp_family_ceiling_maximum{family}` (GaugeVec), `ze_bgp_family_ceiling_warning_exceeded{family}` (GaugeVec), and `ze_bgp_family_ceiling_refused_total{peer,family}` (CounterVec). Two report bus codes under source `bgp`: `family-ceiling-threshold` (Warning, subject `<family>`, raised at `count >= warning`, cleared when `count < warning` or ceiling removed) and `family-ceiling-exceeded` (Error, subject `<peer>/<family>`, detail `peer`, `family`, `count`, `maximum`). |

### Triple Challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | Accept/advertise control adds zero runtime code. The ceiling adds one small `routerCeiling` counter file (`router_ceiling.go`) and extends `applyInstalledPrefixSections` and `Globals` rather than building a second accounting or config pipeline. |
| Uniformity | Mirrors the per-peer `prefix` container (`maximum`, `warning`, `range "1..max"`, default 90% warning), reuses `registered-address-family`, `nlrisplit.GetPrefixKey`, `rollbackPrefixSets`, `applyGlobalsJournaled`, the `bgp` report bus, and `ze_bgp_*` metric conventions. |
| Performance | Zero cost when `bgp > prefix-limit` is unset (`AC-9`: nil/empty check). When set: one atomic CAS per ceiling-family section on the existing installed-set path, with no extra payload copies or locks on the hot receive path. |

### Strongest Concern

The counter-leak class (R-2): every session close path (NOTIFICATION sent/received, hold timer, TCP EOF, BFD down, collision, GR, admin stop, reload restart, panic) must release the session's held ceiling counts exactly once. Placing an idempotent `Session.releaseRouterCeiling()` in both `Session.Run`'s exit defer and `Peer.runOnce`'s teardown defer (guarded so a second call is a no-op) covers both peer-managed and standalone sessions, and `TestRouterCeilingReleasesOnSessionTeardown` verifies every close-path class returns the count to zero.

## Files to Modify
- `internal/component/bgp/yang/ze-bgp-conf.yang` - add `container prefix-limit` with `list family` (`name`, `maximum`, `warning`) under `container bgp`
- `internal/component/bgp/reactor/config.go` - extend `Globals` and `GlobalsFromTree` to parse and validate `bgp > prefix-limit > family`
- `internal/component/bgp/reactor/reactor.go` - add `PrefixLimits` to `reactor.Config` and `routerCeiling` field to `Reactor`
- `internal/component/bgp/reactor/reload_globals.go` - update `applyGlobalsJournaled`, `setGlobals`, and `globals` to reconcile `routerCeiling` transactionally on reload
- `internal/component/bgp/reactor/reactor_api.go` - ensure `VerifyConfig` and `reconcilePeersJournaled` validate and apply `Globals.PrefixLimits`
- `internal/component/bgp/config/loader_create.go` - pass `globals.PrefixLimits` into `reactor.Config` in `CreateReactorFromTree`
- `internal/component/bgp/reactor/peer_session_factory.go` - attach `p.reactor.routerCeiling` to each new `Session` in `newSessionWithEpoch`
- `internal/component/bgp/reactor/session_prefix.go` - extend `applyInstalledPrefixSections` and `checkPrefixLimits` for ceiling reservation, rollback, withdrawal release, and `Session.attachRouterCeiling`
- `internal/component/bgp/reactor/session.go` - hold `routerCeiling` on `Session` and call idempotent `releaseRouterCeiling` in `Session.Run` exit defer
- `internal/component/bgp/reactor/peer_run.go` - call idempotent `sess.releaseRouterCeiling()` in `Peer.runOnce` teardown defer beside `sess.clearReportedWarnings()`
- `internal/component/bgp/reactor/reactor_metrics.go` - register `ze_bgp_family_ceiling_count`, `ze_bgp_family_ceiling_maximum`, `ze_bgp_family_ceiling_warning_exceeded`, and `ze_bgp_family_ceiling_refused_total`
- `internal/le/interoplab/bgp/names.go` and `internal/le/interoplab/bgp/checkers.go` - register and implement typed Go checkers for `bgp-ls-accept-by-family-gobgp` and `bgp-ls-rib-ceiling-gobgp`
- `rfc/short/rfc9552.md` - annotate and link `RFC9552-8.2.3-1` (scoped evidence naming the no-family-block gap) and `RFC9552-8.2.3-3`
- `rfc/discrimination/rfc9552.json` - updated via `./le rfc record`, never hand-edited
- `docs/architecture/wire/nlri-bgpls.md` - add "Receiver Controls (RFC 9552 Section 8.2.3)" section
- `docs/architecture/core-design.md` - list `bgp/family-ceiling-threshold` and `bgp/family-ceiling-exceeded` in Section 15 Operational Report Bus
- `docs/architecture/api/commands.md` - add the two ceiling report codes to the Day-one BGP producers table
- `docs/architecture/config/syntax.md` - document `bgp { prefix-limit { family <afi/safi> { maximum; warning; } } }`
- `docs/architecture/testing/interop.md` - document `bgp-ls-accept-by-family-gobgp` and `bgp-ls-rib-ceiling-gobgp`
- `docs/plugin-development/metrics.md` - document the four `ze_bgp_family_ceiling_*` metrics
- `docs/guide/configuration.md` - document `bgp > prefix-limit` and per-peer BGP-LS accept/advertise configuration
- `docs/features/configuration.md` and `features/prefix-limit-counting-mode.md` - document router-wide `prefix-limit` beside per-peer prefix limits; regenerate `docs/features/rfc-status.md` via `./le rfc update`

## Files to Create
- `internal/component/bgp/reactor/router_ceiling.go` - reactor-owned per-family atomic ceiling counter, config update, report bus helpers, and metric updates <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/reactor/router_ceiling_test.go` - unit tests covering wiring, installed-vs-offered semantics, close-path release, reload/rollback, zero-overhead when unset, per-peer precedence, validation, and `-race` concurrency <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/reactor/rfc9552_router_ceiling_test.go` - `RFC9552-8.2.3-3` positive and negative tagged tests <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/reactor/rfc9552_accept_by_family_test.go` - `RFC9552-8.2.3-1` positive and negative tagged tests <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `test/plugin/rfc9552-823-accept-by-family.ci`, `test/plugin/rfc9552-823-advertise-by-family.ci`, `test/plugin/rfc9552-823-rib-ceiling.ci` - live daemon functional tests <!-- doc-links: ignore (files this open spec plans and has not created yet) -->
- `test/interop/scenarios/bgp-ls-accept-by-family-gobgp/`, `test/interop/scenarios/bgp-ls-rib-ceiling-gobgp/` - declarative GoBGP v4.11.0 Docker interop scenarios <!-- doc-links: ignore (directories this open spec plans and has not created yet) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang`: `bgp > prefix-limit > family` |
| YANG validation constraints | Yes | `uint32` `range "1..max"` on `maximum`, `mandatory true`; `zt:address-family` with `ze:validate "registered-address-family"` on `name` |
| YANG custom validators | Yes | reuses existing `registered-address-family` validator and `CompleteFn`; `GlobalsFromTree` validates `warning < maximum` |
| CLI commands/flags | No | configuration-only surface; reports appear in existing `show warnings` / `show errors` |
| CLI grammar (keyword before value) | N-A | no new CLI command |
| Editor autocomplete | Yes | automatic from YANG; family key completes from registered-family `CompleteFn` |
| Functional test for new RPC/API | Yes | three live-daemon `.ci` files in `test/plugin/` (`rfc9552-823-*.ci`) |
| Pipe completeness | N-A | no new command output |
| Env var registration | No | operator configuration in YANG, not an environment default |
| Doctor check for runtime dependencies | No | no new file, socket, port, kernel module, binary, or certificate |
| Doctor code per problem class | Yes | transient operational report bus codes `bgp/family-ceiling-threshold` (Warning, state-based, cleared on drop below warning) and `bgp/family-ceiling-exceeded` (Error, event-based, names peer, family, count, maximum, and fix pointing to `bgp prefix-limit family <f> maximum` or peer inspection); config validation refusals return `opererr`-style errors naming the offending value and fix (`ai/rules/user-facing-errors.md`) |
| Prometheus counters/metrics | Yes | `ze_bgp_family_ceiling_count{family}`, `ze_bgp_family_ceiling_maximum{family}`, `ze_bgp_family_ceiling_warning_exceeded{family}`, `ze_bgp_family_ceiling_refused_total{peer,family}` in `reactor_metrics.go` |
| BGP family surface (new SAFI / capability / attribute) | No | no new AFI/SAFI, capability, or path attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/prefix-limit-counting-mode.md` and `docs/features/configuration.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (Operational Report Bus Day-one BGP producers table) |
| 5 | Plugin added/changed? | No | reactor only |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` and `docs/architecture/wire/nlri-bgpls.md` |
| 7 | Wire format changed? | No | no wire byte Ze writes or reads changes |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9552.md`, `rfc/discrimination/rfc9552.json`, regenerated `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (new GoBGP BGP-LS scenarios) |
| 11 | Affects daemon comparison? | No | checked `docs/comparison.md`; no change required unless prefix-limit scopes table is updated during Phase 4 |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/core-design.md` (Section 15), `docs/architecture/api/commands.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked via `./le spec citation anchors spec plan/release/fixes/spec-bgp-ls-receiver-accept-and-limit-controls.md`. Declared `// Design:` pages of modified files: `docs/architecture/core-design.md` (modified: Section 15), `docs/architecture/config/syntax.md` (modified: `bgp > prefix-limit` syntax), `docs/architecture/testing/interop.md` (modified: new scenarios), `docs/architecture/buffer-architecture.md` (declared by `session.go`: **unaffected**, because no read/write buffer pool, ownership rule, or copy trigger changes), `docs/architecture/behavior/peer-lifecycle.md` (declared by `peer_session_factory.go`: **unaffected**, because session creation, reconnect backoff, collision, and FSM lifecycle are unchanged; `newSessionWithEpoch` only attaches the reactor ceiling pointer), and `docs/guide/config-reload.md` (declared by `reload_globals.go`: **unaffected** or updated in Phase 4 if global reload table lists `prefix-limit`). |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify `prefix { maximum }` examples in `docs/guide/configuration.md` remain valid alongside the new `bgp > prefix-limit` block |

### Discovery (`ai/rules/repo-maintenance.md`)
| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` rows for `bgp-ls` and prefix limits point to `docs/architecture/wire/nlri-bgpls.md` and `docs/guide/configuration.md` |
| What rule prevents regression | Four new RFC tags (`RFC9552-8.2.3-1` ±, `RFC9552-8.2.3-3` ±) with discrimination records verified by `./le rfc check` |
| What registry prevents drift | `ze:validate "registered-address-family"` validates family keys from `internal/core/family`, so no list of families is hardcoded |
| What verification proves it | Scoped reactor/config unit tests (including `-race`), three `.ci` tests in `test/plugin/`, and two GoBGP Docker interop scenarios |

## Design presentation

### What changes

This change proves and documents Ze's existing per-neighbor BGP-LS accept and advertise controls (`RFC9552-8.2.3-1`) and adds a router-wide per-family RIB ceiling (`RFC9552-8.2.3-3`). Operators continue to control which neighbors exchange BGP-LS through each peer's or group's `session > family` block (and `policy { family-filter }` with `action remove` for one-direction control), now backed by unit tests, live-daemon `.ci` proofs, a GoBGP interop test, and RFC discrimination records. To cap total Link-State NLRIs across all sessions, operators configure a new `bgp { prefix-limit { family <afi/safi> { maximum N; warning M; } } }` block (generic over any registered family, no BGP-LS hardcoding). The reactor maintains one atomic counter per configured ceiling family, fed from each session's installed route-identity set. When an inbound UPDATE would push the router-wide total past `maximum`, Ze drops that UPDATE before plugin dispatch, rolls back the session's installed set, keeps all BGP sessions Established (no NOTIFICATION), emits `bgp/family-ceiling-exceeded` on the operational report bus, and increments `ze_bgp_family_ceiling_refused_total`. Reloading `maximum` or `warning` updates live sessions transactionally without resetting peers or evicting already-installed routes, and session teardown releases every held entry idempotently.

### Before and after

1. **Per-neighbor BGP-LS accept/advertise (`RFC9552-8.2.3-1`):**
   - **Before:** The per-peer `session > family` allow-list and per-direction `policy { family-filter }` already functioned in code, but `RFC9552-8.2.3-1` was unannotated and unproven, with no dedicated unit, `.ci`, or GoBGP interop test.
   - **After:** `TestRFC9552ListedNeighborsLinkStateNLRIIsAccepted`, `TestRFC9552UnlistedNeighborsLinkStateNLRIIsNotAccepted`, `rfc9552-823-accept-by-family.ci`, `rfc9552-823-advertise-by-family.ci`, and `bgp-ls-accept-by-family-gobgp` prove both directions (plus asymmetric `family-filter` import/export removal), with discrimination records in `rfc/discrimination/rfc9552.json` and scoped evidence naming the no-`family`-block fallback gap.
2. **Router-wide Link-State RIB ceiling (`RFC9552-8.2.3-3`):**
   - **Before:** Operators could only set per-peer `family <f> { prefix { maximum N; } }`; 10 peers with `maximum 1000` each could still install 10,000 Link-State NLRIs with no router-wide cap.
   - **After:** Configuring `bgp { prefix-limit { family bgp-ls/bgp-ls { maximum 1000; warning 900; } } }` bounds total installed Link-State NLRIs summed across all active sessions to 1000. Crossing 900 raises `bgp/family-ceiling-threshold` (`show warnings`) and sets `ze_bgp_family_ceiling_warning_exceeded{family="bgp-ls/bgp-ls"} 1`; the 1001st new NLRI from any peer is dropped without tearing down the session, raises `bgp/family-ceiling-exceeded` (`show errors`) naming peer, family, count, and maximum, and increments `ze_bgp_family_ceiling_refused_total{peer,family}`. Withdrawals and session teardowns immediately free capacity.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Dedicated `bgp-ls { accept-from [...]; advertise-to [...]; }` neighbor list (`A2`) | Duplicates RFC 4760 Multiprotocol negotiation, allows two configs to disagree, and embeds BGP-LS-specific syntax in a generic path (`ai/rules/simplicity.md`). |
| Build a dedicated `consumer-facing` per-peer YANG role (`A3`) | Inbound-only and all-or-nothing; `RFC9552-8.2.6-2` is `{not-applicable}` and the sibling fault-management spec already dropped the role (2026-11-24 owner decision). |
| Tag per-peer `prefix { maximum N; count installed; }` alone as the answer to `8.2.3-3` (`C1`) | Bounds one neighbor's contribution, not a router's RIB across peers. |
| Enforce the ceiling inside `bgp-rib`'s `PeerRIB.Insert` (`C2`) | `bgp-rib` receives routes only after reactor dispatch and does not gate `bgp-rs` forwarding, leaving the RIB and relayed routes disagreeing. |
| Tear down the crossing peer with Cease/MaxPrefixes when the router ceiling is crossed | The peer sending the N+1st NLRI is not necessarily the peer that filled the RIB; Thomas decided on 2026-10-13 to drop the crossing UPDATE and keep all sessions Established. |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- config schema, `Globals`, reactor ceiling plumbing, and session attachment
   - Tests: `TestRouterCeilingConfigReachesEverySession`, `TestNoRouterCeilingAddsNoInstalledSet`, `TestRouterCeilingRefusesUnregisteredFamily`
   - Files: `ze-bgp-conf.yang`, `config.go`, `reactor.go`, `reload_globals.go`, `reactor_api.go`, `loader_create.go`, `peer_session_factory.go`, `router_ceiling.go` (initial stub that accepts every reservation), `reactor_metrics.go`
   - Verify: config loads through YANG + `GlobalsFromTree` + `CreateReactorFromTree` + `applyGlobalsJournaled` and reaches every new session; `TestRFC9552RouterCeilingRefusesTheNextLinkStateNLRI` fails because the stub never refuses.
2. **Phase: Accept/advertise proof (`RFC9552-8.2.3-1`)** -- unit tests, functional `.ci` tests, and discrimination records
   - Tests: `TestRFC9552ListedNeighborsLinkStateNLRIIsAccepted`, `TestRFC9552UnlistedNeighborsLinkStateNLRIIsNotAccepted`, `rfc9552-823-accept-by-family.ci`, `rfc9552-823-advertise-by-family.ci`
   - Files: `rfc9552_accept_by_family_test.go`, `test/plugin/rfc9552-823-accept-by-family.ci`, `test/plugin/rfc9552-823-advertise-by-family.ci`, `rfc/short/rfc9552.md`, `rfc/discrimination/rfc9552.json`
   - Verify: record red discrimination with `./le rfc record` (temporarily breaking `validateUpdateFamilies` refusal and outbound `SupportsFamily` gate).
3. **Phase: Router-wide ceiling enforcement (`RFC9552-8.2.3-3`)** -- atomic reserve, rollback, idempotent teardown release, reload, reports, metrics
   - Tests: `TestRFC9552RouterCeilingAcceptsUpToTheMaximum`, `TestRFC9552RouterCeilingRefusesTheNextLinkStateNLRI`, `TestRouterCeilingCountsInstalledNotOffered`, `TestRouterCeilingReleasesOnSessionTeardown`, `TestRouterCeilingLoweredAtReloadEvictsNothing`, `TestPerPeerMaximumIsCheckedBeforeRouterCeiling`, `TestRouterCeilingConcurrentReserve`, `rfc9552-823-rib-ceiling.ci`
   - Files: `router_ceiling.go`, `session_prefix.go`, `session.go`, `peer_run.go`, `router_ceiling_test.go`, `rfc9552_router_ceiling_test.go`, `test/plugin/rfc9552-823-rib-ceiling.ci`, `rfc/short/rfc9552.md`, `rfc/discrimination/rfc9552.json`
   - Verify: enumerate and test every `Session` close-path class in `TestRouterCeilingReleasesOnSessionTeardown`; run `TestRouterCeilingConcurrentReserve` under `-race`; record red discrimination with `./le rfc record`.
4. **Phase: GoBGP interop and documentation**
   - Tests: `bgp-ls-accept-by-family-gobgp`, `bgp-ls-rib-ceiling-gobgp`
   - Files: `test/interop/scenarios/bgp-ls-accept-by-family-gobgp/`, `test/interop/scenarios/bgp-ls-rib-ceiling-gobgp/`, `internal/le/interoplab/bgp/names.go`, `internal/le/interoplab/bgp/checkers.go`, `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/core-design.md`, `docs/architecture/api/commands.md`, `docs/architecture/config/syntax.md`, `docs/architecture/testing/interop.md`, `docs/plugin-development/metrics.md`, `docs/guide/configuration.md`, `docs/features/configuration.md`, `features/prefix-limit-counting-mode.md`, regenerated `docs/features/rfc-status.md`
   - Verify: settle A-4 in the Docker lab; drive each interop scenario red against a reverted producer (`ai/rules/interop-and-goal-validation.md`); run `./le doc check all` and `./le rfc check`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-12 has an implementation or verified existing producer at `file:line` |
| Correctness | Ceiling reserves only the net new installed NLRIs of a message (re-announcements and unmatched withdrawals consume zero) |
| Correctness | Reserve and upper-bound compare execute in one atomic CAS loop; an over-ceiling failure undoes all partial reservations from that UPDATE before `rollbackPrefixSets` |
| Correctness | Every `Session` close path releases held counts idempotently; count returns to zero with all sessions down |
| Correctness | Per-peer maximum runs first with its exact action (NOTIFICATION 6/1 or drop) unchanged; per-peer refusal leaves router ceiling untouched |
| Naming | Zero `bgp-ls` literals in the reactor or `bgp > prefix-limit` YANG; metrics follow `ze_bgp_family_ceiling_*` |
| Data flow | Ceiling decision runs in `checkPrefixLimits` before plugin dispatch; no plugin package is imported by the reactor |
| Operator errors and doctor codes | Config validation errors name the offending family/value and exact fix; report bus raises `bgp/family-ceiling-threshold` (Warning) and `bgp/family-ceiling-exceeded` (Error) with peer, family, count, and maximum |
| Rule: `ai/rules/rfc-compliance.md` | All four tags (`RFC9552-8.2.3-1` ±, `RFC9552-8.2.3-3` ±) have `./le rfc record` discrimination records; `8.2.3-1` states the explicit-`family`-block precondition and names the no-family-block gap |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `bgp > prefix-limit` YANG schema | grep `internal/component/bgp/yang/ze-bgp-conf.yang`; `ze config validate` on valid and invalid configs |
| Scoped reactor & config unit tests (`-race`) | `./le test unit pkg path ./internal/component/bgp/reactor/ ./internal/component/bgp/config/` |
| Four RFC 9552 Section 8.2.3 tags + discrimination records | `./le rfc check`; inspect `rfc/discrimination/rfc9552.json` |
| Three live-daemon `.ci` functional tests | `./le test functional plugin` (or targeted runs for `rfc9552-823-accept-by-family`, `rfc9552-823-advertise-by-family`, `rfc9552-823-rib-ceiling`) |
| Two GoBGP Docker interop scenarios | `./le test integration interop` on `bgp-ls-accept-by-family-gobgp` and `bgp-ls-rib-ceiling-gobgp` |
| Docs, anchors, and tier boundaries clean | `./le doc check all`; `./le arch tier check`; `./le plugin boundary check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | The router ceiling bounds total Link-State NLRIs across all sessions; per-session installed-set memory cost (R-4) is bounded by the ceiling and incurred only for configured ceiling families |
| Fail open vs fail closed | A leaked counter fails closed (refuses new NLRIs) and is prevented by idempotent release on every close path (AC-7); GR stale-path under-counting (R-5) is explicitly documented |
| Input validation | Family key validated via `registered-address-family`, `maximum` via `uint32 range "1..max"`, and `warning < maximum` enforced in `GlobalsFromTree` before startup or reload apply |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| A tagged existing test must change | STOP; owner approval through `test/rfc-changed.md` |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Known Limitations
- Section 8.2.3's producer-side SHOULDs other than the advertise half of `8.2.3-1` (rate limit `8.2.3-2`, abstracted topologies `8.2.3-4`, 4096-byte UPDATE size `8.2.3-7`) stay with `plan/post-release/features/spec-bgp-ls-origination-and-the-scheduled-marker.md`.
- A peer with NO `family` block still advertises all plugin decode families including BGP-LS (`offeredMultiprotocol`, `session_negotiate.go`; R-8); fixing that cross-family fallback is owned by `plan/release/fixes/spec-bgp-one-off-defects.md` (journal `plan/journal/silent-fall-through.md`, 2026-08-16).
- During a Graceful Restart window, stale routes retained in `bgp-rib` while the restarted `Session` starts with an empty installed set are not counted in the reactor ceiling (R-5, accepted by owner decision 2026-10-13).
- Enabling a new ceiling family via config reload on already-Established sessions tracks NLRIs received after the reload until those sessions reset or refresh (R-9).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-peer/per-group `family` block + `family-filter` is Ze's accept/advertise list | Dedicated BGP-LS neighbor list; sibling's `consumer-facing` role | One declaration per fact; already per peer and per group; covers both directions; avoids BGP-LS-specific config in generic paths (owner decision 2026-10-13) |
| Router ceiling lives in the reactor (`routerCeiling`), fed by per-session installed sets summing across sessions | Per-peer maximum alone; enforcement inside `bgp-rib` `PeerRIB.Insert` | Only the reactor sees every session before dispatch and gates both `bgp-rib` and `bgp-rs` forwarding; installed-set cardinality matches what each session holds (owner decision 2026-10-13) |
| Crossing the router ceiling drops the UPDATE and keeps all sessions Established | Tear down the crossing peer with Cease/MaxPrefixes | The crossing peer is not necessarily the peer that filled the RIB (owner decision 2026-10-13) |
| Ceiling config is `bgp > prefix-limit > family <afi/safi> { maximum; warning; }`, generic per registered family | BGP-LS-only leaf; `bgp > family > rib-maximum` | Mirrors per-peer `prefix` container shape, cites IETF `max-number` in description, and uses `registered-address-family` so any family can be capped with zero new code (owner decision 2026-10-13) |

## RFC Documentation (Scope: protocol)

The ceiling's refusal check in `router_ceiling.go` / `session_prefix.go` carries `// RFC 9552 Section 8.2.3: "An implementation SHOULD allow the operator to specify the maximum number of Link-State NLRIs stored in a router's Routing Information Base (RIB)."` above the enforcing statement, and the test files carry the four `RFC requirement:` tags (`RFC9552-8.2.3-1` positive/negative, `RFC9552-8.2.3-3` positive/negative).

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 9552 §8.2.3 (`RFC9552-8.2.3-1`) | Peers with an explicit `session > family` block negotiate `bgp-ls/bgp-ls` only when listed; unnegotiated inbound BGP-LS triggers NOTIFICATION 3/9 (`validateUpdateFamilies`) and unnegotiated destinations receive no AFI 16388 MP_REACH (`session_ownership.go`). Gap: peers with no `family` block still advertise plugin families (`plan/release/fixes/spec-bgp-one-off-defects.md`). | `family { mode ignore; }` / `ignore-mismatch enable` (silent drop keeping session up); `policy { family-filter }` `action remove` on `import` or `export` for one-direction control | `TestRFC9552ListedNeighborsLinkStateNLRIIsAccepted`, `TestRFC9552UnlistedNeighborsLinkStateNLRIIsNotAccepted`, `rfc9552-823-accept-by-family.ci`, `rfc9552-823-advertise-by-family.ci`, `bgp-ls-accept-by-family-gobgp` | `rfc9552-823-accept-by-family.ci` (`mode ignore` and `family-filter` import/export) |
| RFC 9552 §8.2.3 (`RFC9552-8.2.3-3`) | Default `bgp > prefix-limit` is unset (unbounded, zero overhead); when configured for `bgp-ls/bgp-ls`, total installed Link-State NLRIs summed across all active sessions are capped at `maximum`, dropping crossing UPDATEs without session teardown | Generic over any registered address family; per-peer `prefix { maximum; teardown; count; }` preserved unchanged and evaluated first | `TestRFC9552RouterCeilingAcceptsUpToTheMaximum`, `TestRFC9552RouterCeilingRefusesTheNextLinkStateNLRI`, `rfc9552-823-rib-ceiling.ci`, `bgp-ls-rib-ceiling-gobgp` | `TestNoRouterCeilingAddsNoInstalledSet`, `TestPerPeerMaximumIsCheckedBeforeRouterCeiling`, `TestRouterCeilingLoweredAtReloadEvictsNothing` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | - |
| `./le spec review check` | not run |
| Rounds | 0 |
| Reviewer lenses used | - |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior, Existing Mechanisms, Design documents, Data Flow (with Changed-Wire Census), and Design presentation sections completed
- [ ] AC-N rows carry positive testable assertions and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist answered Yes/No/N-A on every row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/component/bgp/reactor/ ./internal/component/bgp/config/`, `./le test functional plugin` (covering `rfc9552-823-*.ci`), `./le test integration interop` (covering `bgp-ls-accept-by-family-gobgp` and `bgp-ls-rib-ceiling-gobgp`), `./le rfc check`, `./le arch tier check`, `./le plugin boundary check`, `./le doc check all`, and whatever `./le repo changed scope print both` reaches. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding (outbound and inbound)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is homed in its own spec, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `./le spec review record`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-ls-receiver-accept-and-limit-controls.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
