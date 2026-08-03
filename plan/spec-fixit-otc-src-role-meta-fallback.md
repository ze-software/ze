# Spec: fixit-otc-src-role-meta-fallback

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/6 |
| Updated | 2026-07-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now), starting at the OPEN BLOCKER below
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9234.md` - the Section 5 ingress/egress procedures this spec enforces
4. `internal/component/bgp/plugins/role/otc.go` - the whole filter pair; and
   `internal/component/bgp/plugins/role/role.go`, `config.go`, `metrics.go` for the
   role-resolution and observability halves that landed later

## OPEN BLOCKER -- OTC is stamped onto WITHDRAWALS (round-4 review, 2026-07-27)

**This spec must NOT be closed until this is fixed.** Found by the fourth independent
review. Rounds 1, 2 and 3 each found a defect too; this is the most severe.

**Re-verified against the working tree 2026-07-27 (second pass, after `f5dd2f040`
landed): STILL PRESENT, and the reason it was not fixed no longer exists.** Producers
read, not inferred:

| Producer | `file:line` | What it does |
|----------|-------------|--------------|
| egress stamping block | `role/otc.go` | gated only on `mods != nil` and `destRemoteRole ∈ {customer, peer, rs-client}`. No advertisement gate of any kind |
| `isPayloadUnicast` | `role/otc.go` | scans for `mpReachAttrCode` (`= byte(14)`, `otc.go`) only, and its terminal `return true` at `:144` reads "no MP_REACH found, therefore IPv4 unicast" |
| `extractAttrsFromPayload` | `role/otc.go` | returns `payload[attrStart : attrStart+attrLen]`, a non-nil EMPTY slice when `attrLen == 0`, so no `attrs == nil` guard can fire on a pure withdrawal |
| MP_UNREACH awareness | (none) | `grep -rn -i 'unreach' internal/component/bgp/plugins/role/*.go` returns no code hit. Type 15 is not mentioned anywhere in the plugin |

So a pure IPv4 withdrawal (`00 04 18 0a 00 00 00 00`) passes `isPayloadUnicast`, reaches
`otc.go` with an empty attribute slice, `findOTC` reports `hasOTC == false`, and
`mods.Op(otcAttrCode, ...)` fires at `:573`. Same for a VPNv4 MP_UNREACH-only payload,
which the family gate cannot see.

**The tree-contention block is CLEARED.** The original note below said the fix was held
because a concurrent session held uncommitted edits to `otc.go`, `config.go`, `role.go`,
`register.go` and four untracked files. That session committed as `f5dd2f040`
("fix(bgp): close three RFC 9234 role-resolution holes"): `metrics.go` and
`metrics_test.go` are tracked, `recordDrop`/`dropLeak` resolve, and `git status` is clean
for the role package. **Nothing now blocks the fix.** It remains small and known: gate the
stamp on the payload actually advertising reachable NLRI, and teach `isPayloadUnicast`
about MP_UNREACH (type 15).

**Also still open from round 4, re-checked 2026-07-27:**
`TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` (`otc_test.go`) still
carries no `RFC requirement:` tag, so `ai/RFC-REQUIREMENTS.md` continues to credit
`RFC9234-5-4` to the weaker test; and `TestOTCEgressUnicastOnly` (`otc_test.go`)
still proves `RFC9234-5-10` only through the MP_REACH branch, so the "unicast-only
scoping" claim at `docs/features/rfc-status.md` is not backed for the withdrawal shape.

**Defect.** The egress stamping block (`otc.go`, the `mods != nil && destRemoteRole ∈
{customer, peer, rs-client}` branch) has no "is this an advertisement?" gate. RFC 9234 §5
egress rule 1 says "If a route **is to be advertised**"; a withdrawal is not a route. The
family gate cannot catch it either: `isPayloadUnicast` scans only for `mpReachAttrCode`
(type 14) and never inspects MP_UNREACH_NLRI (type 15), and its terminal `return true`
reads "no MP_REACH found, therefore IPv4 unicast". `extractAttrsFromPayload` returns a
non-nil empty slice when `attrLen == 0`, so the `attrs == nil` guard never fires. There is
zero MP_UNREACH awareness anywhere in the role plugin.

**Reproduced, not inferred.** Feeding the payload of
`TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig`:

| Input | Result |
|-------|--------|
| pure IPv4 withdrawal `00 04 18 0a 00 00 00 00` | `accept=true`, `mods.Len()==1` |
| VPNv4 MP_UNREACH `00 00 00 06 80 0f 03 00 01 80` (AFI 1 / SAFI 128) | `accept=true`, `mods.Len()==1` |

**It reaches the wire.** `buildModifiedPayload` step 6
(`internal/component/bgp/reactor/forward_build.go`) writes unconsumed ops as new
attributes, and `otcAttrModHandler` emits the 7-byte attribute. Neither egress caller
(`reactor_api_forward.go`, `forward_rs.go`) has a withdrawal guard.

**Four violations**, the last one interop-fatal: RFC 4271 §4.3 (no path attributes on a
withdraw-only UPDATE); RFC 7606 §3(d) (treat-as-withdraw); **RFC 7606 §5.2 -- a peer
receiving attributes other than MP_UNREACH with no reachable NLRI MUST use "session
reset"**; and RFC 9234 §5 / `RFC9234-5-10` (MUST NOT apply to other address families).

**Symmetric on ingress** (producer-read, not executed): `OTCIngressFilter` →
`isPayloadUnicast` → `checkOTCIngress` returns a stamp ASN for a withdrawal from a
Provider/Peer/RS → `insertOTCInPayload` rewrites it, and per the `IngressFilterFunc`
contract that modified payload replaces the original **for caching and dispatch**, so the
corrupted bytes are stored and later relayed.

**Pre-existing in shape, widened by `e0607d0f4`.** Removing the `srcCfg == nil` early
return (correct in itself: it was gating an RFC MUST) means the stamp now fires for iBGP
peers, RR clients and locally originated routes -- in a typical deployment, essentially
every withdrawal forwarded to any Customer/Peer/RS-Client.

**Also open from round 4:** `TestOTCEgressUnicastOnly` proves `RFC9234-5-10` only through
the MP_REACH branch, so the "unicast-only scoping" claim at `docs/features/rfc-status.md`
("No tracked gap") is not backed for the withdrawal shape; and
`TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` carries no `RFC requirement:` tag,
so `ai/RFC-REQUIREMENTS.md` still credits `RFC9234-5-4` to the weaker test.

**Why it is not fixed in this session.** A CONCURRENT session holds uncommitted edits to
`otc.go`, `config.go`, `role.go` and `register.go` plus four untracked files, and `otc.go`
now references `recordDrop`/`dropLeak` defined in their untracked `metrics.go`. Committing
`otc.go` would either cross-commit their work or land a tree that does not build. This is
a tree-contention block, verifiable with `git status`, not a scope decision. The fix is
small and known: gate the stamp on the payload actually advertising reachable NLRI, and
teach `isPayloadUnicast` about MP_UNREACH.

## Task

Close the missing-metadata gap in `OTCEgressFilter`'s Gao-Rexford safety net.

`internal/component/bgp/plugins/role/otc.go` reads `meta["src-role"]` and
treats a MISSING key as "no restriction". That is the zero-value trap in
`ai/rules/evidence.md`: any caller without ingress metadata silently
skips an RFC 9234 Section 5 leak guard. The Adj-RIB-In relay path
(`RelayStoredRoute`) is one such caller today; it is not the only possible one.

The value is recoverable EXACTLY, not guessable: `OTCIngressFilter` (otc.go)
writes `meta["src-role"] = cfg.role` from `getFilterConfig(src.Address.String())`,
the same lookup `OTCEgressFilter` already performs into `srcCfg`. So the fix is a
config-derived fallback when meta lacks the key.

Split out of `plan/spec-fixit-bgp-egress-rail-divergence.md` (see
`plan/deferrals/fixit-bgp-egress-rail-divergence.md`) because that spec's ACs pass
without it -- OTC suppression on the replay path already works through the
wire-bytes rule -- and because landing it requires changing an RFC-tagged test.

**BLOCKING precondition:** `TestOTCEgressNoStampProvider`
(`internal/component/bgp/plugins/role/otc_test.go`) carries
`RFC requirement: RFC9234-5-4 negative`. Its fixture asserts `accept == true` for a
Provider source to a Provider destination, which RFC 9234 Section 5 says must be
SUPPRESSED; it only reads as accept because meta is nil. Changing it needs explicit
user approval and an `// rfc-test-change-approved:` marker
(`ai/rules/testing.md`, RFC-Tagged Tests). Ask before writing any code.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/meta/role.md` - the route-metadata contract for `src-role`: who writes the key, who reads it, and what a missing key means
  → Decision: `src-role` is a CACHE of the source peer's `role { import ... }` config value, not an independent input. That is what makes a config-derived fallback an exact recovery rather than a guess.
  → Constraint: metadata keys are best-effort. No producer is guaranteed to have run before a consumer, so a consumer must not treat an absent key as a decision.
- [ ] `ai/rules/evidence.md` - the rule the original defect violates
  → Constraint: "On a miss, an unmapped input, an empty set, or an error, deny." A guard that neither denies nor speaks does not exist. The zero-value trap: an absent value must never select the permissive branch.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9234.md` - the OTC attribute, the Section 5 ingress and egress procedures, and the Section 4.2 role-resolution rule
  → Constraint: Section 5 egress rule 1 is conditioned on the DESTINATION only -- "If a route is to be advertised to a Customer, a Peer, or an RS-Client [...] and the OTC Attribute is not present, then [...] an OTC Attribute MUST be added". It is also conditioned on the route being ADVERTISED, which is the open blocker above.
  → Constraint: Section 5 egress rule 2 -- "If a route already contains the OTC Attribute, it MUST NOT be propagated to Providers, Peers, or RSes."
  → Constraint: Section 4.2 -- "The locally configured BGP Role is used for the procedures described in Section 5." Config is the prescribed input for a peer that sent no Role capability, not a consolation prize.
  → Constraint: Section 5 -- OTC procedures are scoped to AFI 1/2, SAFI 1. They MUST NOT be applied to other address families by default.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- One defect shape recurred five times in this spec's life: a value that can legitimately be empty was read as a decision. It appeared at `meta["src-role"]` (the original), at the destination role, at the ingress role, at the `srcCfg == nil` early return, and at the config key itself. Each fix closed one reader; the next review found the next.
- The RFC conditions the stamping rule on the DESTINATION alone. Every gate this spec removed was a source-side condition the RFC never stated.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/bgp/plugins/role/otc.go` - `OTCEgressFilter`: the unicast gate, the two `getFilterConfig` lookups, the wire-bytes suppression, the Gao-Rexford safety net, the export-set match, and the stamping block
  → Constraint: the stamping block has no advertisement gate. See OPEN BLOCKER.
- [ ] `internal/component/bgp/plugins/role/otc.go` - `OTCIngressFilter`: writes `meta["src-role"] = cfg.role` at `:316` from `getFilterConfig(src.Address.String())` at `:312`, the same lookup the egress filter performs into `srcCfg` at `:491`
- [ ] `internal/component/bgp/plugins/role/otc.go` - `resolveSrcRole`: meta first, config fallback, `""` when neither
- [ ] `internal/component/bgp/plugins/role/otc.go` - `resolvePeerRole`: capability first, then the config complement via `peerRoleComplement`
- [ ] `internal/component/bgp/plugins/role/config.go` - `getFilterConfig` keying and the unusable-key rejection added by `f5dd2f040`
- [ ] `internal/component/bgp/plugins/role/role.go` - `setFilterRemoteRole` (the only writer of `filterRemoteRoles`) and the OPEN-time clear added by `f5dd2f040`
- [ ] `internal/component/bgp/plugins/role/metrics.go` - `recordDrop` and the four drop reasons added by `f5dd2f040`
- [ ] `internal/component/bgp/reactor/reactor_api_relay.go` - `RelayStoredRoute`, which builds the replayed update with `Meta: nil`: the caller shape the fallback exists for

**Behavior to preserve:** (unless user explicitly said to change)
- `OTCEgressFilter` / `OTCIngressFilter` signatures: they are `filterapi.EgressFilterFunc` / `IngressFilterFunc` and are registered, not called directly.
- The wire-bytes egress rule (`checkOTCEgress`, `otc.go`) stays unconditional and independent of source config: it is the rule that already made suppression work on the replay path.
- A peer with NO role config at all is not filtered. `""` means "unconfigured", never "unrestricted".
- `roleUnknown` stays an operator-selected export target in the export-set match (`config.go`), NOT an unanswered question. Reclassifying it there would silently retarget an operator knob.
- The RFC 9234 Section 5 unicast scoping.

**Behavior to change:** (only if user explicitly requested)
- The five defects enumerated in the Implementation Summary. Nothing else. The stamping-onto-withdrawals defect in the OPEN BLOCKER is IN scope and NOT yet fixed.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Ingress: a received BGP UPDATE's payload bytes, handed to `OTCIngressFilter` by the reactor's ingress filter chain before caching and dispatch, together with the shared `ingressMeta` map.
- Egress: the same payload replayed per destination peer, handed to `OTCEgressFilter` by `safeEgressFilter` from `reactor_api_forward.go` (the forward rail) and `forward_rs.go` (the route-server rail), plus `reactor_api_batch.go` for the readvertise rail. The relay rail (`reactor_api_relay.go`) reaches the same filter with `Meta: nil`.
- Format at entry: raw RFC 4271 UPDATE payload -- `withdrawnLen(2) + withdrawn + attrLen(2) + attrs + nlri` -- plus a `filterapi.PeerFilterInfo` for source and destination and a `*filterapi.ModAccumulator` for egress attribute edits.

### Transformation Path
1. Ingress: `getFilterConfig(src)` yields `(cfg, capRole)`; `cfg.role` is copied into `meta["src-role"]` (`otc.go`).
2. Ingress: `resolvePeerRole(capRole, cfg)` decides what the source peer IS to us -- capability first, config complement second (`otc.go`, `:459-476`).
3. Ingress: the unicast gate (`isPayloadUnicast`) and `extractAttrsFromPayload` run; `checkOTCIngress` applies the three Section 5 ingress MUSTs and may return a stamp ASN (`otc.go`).
4. Ingress: a leak or malformed OTC is counted through `recordDrop` and rejected / treated-as-withdraw; a stamp rewrites the payload via `insertOTCInPayload`, and that MODIFIED payload replaces the original for caching and dispatch.
5. Egress: the unicast gate runs first, then `resolvePeerRole` for the destination, then the unconditional wire-bytes rule (`checkOTCEgress`).
6. Egress: the Gao-Rexford safety net calls `resolveSrcRole(meta, srcCfg)` -- meta if usable, else `srcCfg.role`, else `""` -- and suppresses toward a Provider/Peer/RS destination.
7. Egress: the operator export-set match runs (capability-only role), then the stamping block queues `mods.Op(otcAttrCode, AttrModSet, asn)`.
8. Build: `buildModifiedPayload` (`reactor/forward_build.go`) walks the source attributes and, at step 6, writes unconsumed ops as NEW attributes; `otcAttrModHandler` (`otc.go`) emits the 7-byte attribute. This is the step that puts a queued stamp on the wire, and the step the OPEN BLOCKER's withdrawal defect rides.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ role plugin (ingress) | `filterapi.IngressFilterFunc`; the returned payload REPLACES the original for caching and dispatch | Yes -- `OTCIngressFilter` returns `(true, modified)` at `otc.go` |
| Reactor ↔ role plugin (egress) | `filterapi.EgressFilterFunc` returning a bare `bool` plus a `*ModAccumulator`; there is no failure channel, so "could not evaluate" is indistinguishable from "policy said no" | Yes -- signature at `filterapi/filterapi.go`; recorded as a live deferral on `plan/spec-fixit-stored-route-relay-hardening.md` |
| Ingress filter ↔ egress filter | the shared `ingressMeta` map, key `src-role`. One map is shared by every in-process ingress filter, so any filter can clobber the key | Yes -- `resolveSrcRole` logs a WARN on an unusable value rather than failing silently (`otc.go`) |
| Filter ↔ wire build | `ModAccumulator` ops consumed by `otcAttrModHandler` during `buildModifiedPayload` | Yes -- registration asserted by `TestOTCAttrModHandlerRegisteredInRegistry` |
| Config ↔ filter | `getFilterConfig(addr.String())`: all three readers key by ADDRESS, so a name-keyed entry is unreachable | Yes -- rejection + WARN added by `f5dd2f040`, proven by `TestRoleConfigWithoutUsableRemoteIPIsRejected` |

### Integration Points
- `filterapi` ingress/egress registration (`role/register.go`) - the only way either filter is reached; the plugin is removable by deleting its blank import.
- `reactor/forward_build.go` `buildModifiedPayload` step 6 + `otcAttrModHandler` - how a queued OTC op becomes wire bytes.
- `reactor/reactor_api_relay.go` `RelayStoredRoute` - the `Meta: nil` caller the fallback exists for.
- `internal/core/metrics` via `role/metrics.go` `recordDrop` - the observability surface `f5dd2f040` added.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugins.md`)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->
<!-- Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis) land HERE, not just in conversation. -->

### Assumptions
<!-- Things believed true that the design depends on. Every row needs a validation method. -->
<!-- Status: unvalidated → confirmed | broken. A broken assumption also gets a Mistake Log "Wrong Assumptions" row. -->
<!-- No assumption may still be `unvalidated` at Pre-Commit Verification. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `meta["src-role"]` is recoverable EXACTLY from config, not guessed | `OTCIngressFilter` writes `meta["src-role"] = cfg.role` (`otc.go`) from `getFilterConfig(src.Address.String())`; `OTCEgressFilter` performs the same lookup into `srcCfg` | the fallback would be a heuristic, and a wrong suppression is a routing outage | read both producers | **confirmed, with a caveat** -- same map, same key, same field, but they agree at one INSTANT only: meta is captured at RECEIVE, config is read at FORWARD. A config reload between them makes them differ. The relay-time role is the safer one to gate on; recorded in the code comment at `otc.go` |
| A-2 | `RelayStoredRoute` is not the only caller that can lack meta | the fallback is at the READ, not at the caller | fixing the caller instead would leave the next one broken | design decision, not a grep | **confirmed by events** -- three further readers of the same zero-value shape were found by later review rounds (destination role, ingress role, `srcCfg == nil`), each in a different call path |
| A-3 | Landing the fix requires changing an RFC-tagged test | `TestOTCEgressNoStampProvider` carries `RFC requirement: RFC9234-5-4 negative`, and its fixture only reads as `accept` because meta is nil | the change would be blocked, or would land without approval | the pre-write hook | **confirmed** -- the hook blocked the edit until `// rfc-test-change-approved:` was present; Thomas approved 2026-07-27 |
| A-4 | An empty `filterRemoteRoles` value means "the peer advertised no role" | `setFilterRemoteRole` is the only writer (`role.go`) and its caller is guarded by `len(remoteRoles) > 0` | an unrecorded role would be read as a decision | read the writer and its guard | **broken, twice** -- it ALSO meant "we never recorded it" (validate-open RPC timeout, plugin conn not up, `setFilterState` nulling the map on `OnConfigure`), and it meant "a role recorded by a PREVIOUS session was never cleared" until `f5dd2f040` added the OPEN-time clear. The first half is homed on `plan/spec-fixit-stored-route-relay-hardening.md` |
| A-5 | `role` config reaches the filters however the peer is named | the config parser accepted a `role` block on any peer | a config block would be silently inert | `f5dd2f040`'s keying audit | **broken** -- all three `getFilterConfig` readers key by ADDRESS, so a name-keyed entry was unreachable and the whole `role` block was inert with no diagnostic. Fixed by rejecting unreachable keys with a WARN |

### Risks
<!-- Things that could go wrong even if all assumptions hold. From /ze-spec Failure Mode Analysis. -->
<!-- Surviving risks copy forward to the Executive Summary "Risks & observations" and the learned summary. -->
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The fallback introduces a FALSE suppression, dropping routes that used to be advertised | a peer's prefix count drops after upgrade; `ze_role_route_suppressions_total{reason="source-role"}` climbs where the operator did not expect it | the fallback only ever returns the SAME config field the ingress filter would have written; `""` (no config) still means no filtering. Proven by `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` and the full `make ze-test-bgp` regression |
| R-2 | Suppressions are invisible, so a config typo silently withdraws a peer's routes | none, by construction -- that WAS the risk | closed by `f5dd2f040`: four `recordDrop` reasons plus a one-shot WARN on the first drop (`metrics.go`), and the unreachable-key rejection now names the peer |
| R-3 | **A stamped withdrawal reaches a peer and triggers a session reset** | a peer sends NOTIFICATION on a withdraw-only UPDATE; RFC 7606 Section 5.2 says a receiver seeing attributes other than MP_UNREACH with no reachable NLRI MUST use "session reset" | **OPEN. Not mitigated.** This is the blocker at the top of the spec. Nothing in the tree gates the stamp on the payload advertising reachable NLRI |
| R-4 | Widening `resolvePeerRole` retargets the operator's `export unknown` knob | an operator's `role export unknown` set stops matching peers it used to match | deliberately scoped: `resolvePeerRole` feeds the RFC MUST gates only. The export-set match keeps the capability-only value, and the reason is recorded at `otc.go` |
| R-5 | A config reload between receive and forward makes meta and config disagree | none observable | accepted, not fixed: the relay-time role describes the relationship the route is being forwarded under, and is the safer of the two to gate a leak check on. Recorded at `otc.go` |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation — unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An egress filter invocation carrying no ingress metadata (the `RelayStoredRoute` shape, `reactor_api_relay.go` builds the replay with `Meta: nil`) | → | `resolveSrcRole` config fallback (`otc.go`), reached from the `destRemoteRole` branch at `otc.go` | `TestOTCEgressSuppressProviderLearnedWithoutMeta` (`otc_test.go`) |
| A peer that sent no Role capability, on INGRESS | → | `resolvePeerRole` config complement (`otc.go`), reached from `otc.go` | `TestOTCIngressStampsWhenPeerSentNoRoleCapability` (`otc_test.go`) |
| A peer that sent no Role capability, as the egress DESTINATION | → | `resolvePeerRole` from `otc.go` | `TestOTCEgressSuppressToProviderWithoutRoleCapability` (`otc_test.go`) |
| A route from a source with no `role { import ... }` container, advertised to a Customer | → | the stamping block at `otc.go`, no longer gated on `srcCfg != nil` | `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` (`otc_test.go`) |
| A `role` config block on a peer whose remote IP does not resolve | → | the unreachable-key rejection in `config.go` | `TestRoleConfigWithoutUsableRemoteIPIsRejected` (`config_keying_test.go`) |
| A peer that reconnects WITHOUT the Role capability it once advertised | → | the OPEN-time clear of `filterRemoteRoles` in `role.go` | `TestReconnectWithoutRoleCapabilityClearsStaleRole` (`session_role_test.go`) |
| Any suppression decision reaching an operator | → | `recordDrop` (`metrics.go`), called from all four suppression sites | `TestRoleDropsAreCounted` (`metrics_test.go`), `TestRoleFirstDropEmitsWarn` (`metrics_test.go`) |
| A daemon actually applying role policy end to end | → | the registered ingress/egress filter pair | `test/plugin/role-otc-*.ci` (existing suite, unchanged by this work) |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
<!-- Written retroactively 2026-07-27 from the Task section, which stated the
     required behavior precisely; the skeleton's placeholders were never
     filled before implementation. Each row is what the landed code and tests
     actually assert, not an aspiration. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `OTCEgressFilter` reached with `meta == nil` (the `RelayStoredRoute` shape), source configured `role customer` (source IS our Provider), destination remote role Provider | The route is SUPPRESSED. The Gao-Rexford guard evaluates using the role recovered from config rather than skipping |
| AC-2 | Same, but `meta["src-role"]` present and a valid string | The meta value is used unchanged; config is not consulted. Existing behavior is untouched |
| AC-3 | Same, but `meta["src-role"]` present and NOT a string (int, bool) | The fallback is taken. A malformed value is never MORE permissive than a missing one |
| AC-4 | `OTCEgressFilter` reached with no `src-role` in meta AND no role config for the source peer | No suppression. `""` correctly means "peer unconfigured", not "unrestricted"; an unconfigured peer was never filtered and still is not |
| AC-5 | A route legitimately transiting to a Provider (source configured `role provider`, i.e. source IS our Customer), `meta == nil`, destination Provider | ACCEPTED and NOT stamped. The fallback must not cause a false suppression, and the route must still reach the egress stamping block where a Provider destination correctly gets no OTC |

**AC-6..AC-10 added retroactively 2026-07-27, and they are the honest record of a scope
that grew.** AC-1..AC-5 describe the ONE defect the Task section names: the missing-meta
gap in the egress Gao-Rexford net, which landed in `c398e97f0`. Four further commits then
landed under this spec's name -- `276096afb`, `d373d9f40`, `e0607d0f4`, `f5dd2f040` --
each closing a different reader of the SAME zero-value shape, each found by an independent
review round rather than planned. Those commits are not covered by AC-1..AC-5 at all.
Leaving the AC table at five rows would have made the audit report "all ACs Done" over a
diff four times larger than the ACs describe, which is exactly the false-completion
`ai/rules/completion.md` forbids. The rows below state what those commits
actually assert, taken from the landed tests, not from the commit messages. The scope
growth itself is recorded in Deviations.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-6 | The egress DESTINATION peer sent no Role capability but has `role { import ... }` config | The Section 5 destination gates resolve from the config complement instead of reading the empty capability as "no role". A Provider-learned route toward that destination is still suppressed |
| AC-7 | The INGRESS source peer sent no Role capability but has `role { import ... }` config | All three Section 5 ingress MUSTs still run (leak from a Customer/RS-Client, the Peer ASN mismatch, and the stamp). A route without OTC from a configured Provider is stamped with the REMOTE peer's AS |
| AC-8 | A route is advertised to a Customer/Peer/RS-Client from a source with NO role config at all (iBGP, an RR client, a locally originated or API-injected route) | OTC is stamped with the local AS. RFC 9234 Section 5 conditions this rule on the DESTINATION only; no source-side gate may suppress it |
| AC-9 | A `role` block is configured on a peer whose remote IP does not resolve, so the entry would be keyed by name while every reader keys by address | The config is REJECTED with a WARN naming the peer, rather than silently stored and never read. A peer NAMED for its address still works |
| AC-10 | A peer that once advertised a Role capability reconnects without one; and separately, any suppression decision fires | The stale capability-learned role is CLEARED at the OPEN (not on session down, which carries no session identity); and every suppression is counted under a distinguishable reason with a one-shot WARN on the first drop, so a peer's advertisements can never be withdrawn invisibly |

**AC-11 code is MET; its functional/interop coverage is still owed.** The unit-level
behavior is implemented and mutation-verified; the `.ci` and interop rows below remain
outstanding, so this spec does not close on AC-11 alone.

| AC ID | Input / Condition | Expected Behavior | Status |
|-------|-------------------|-------------------|--------|
| AC-11 | `OTCEgressFilter` reached with a payload that advertises NO reachable NLRI: a pure IPv4 withdrawal, or an MP_UNREACH-only payload of any family, toward a Customer/Peer/RS-Client destination | NO OTC attribute is queued. RFC 9234 Section 5 egress rule 1 applies to a route that "is to be advertised"; a withdrawal is not a route. `isPayloadUnicast` must also recognise MP_UNREACH (type 15) so the family scoping holds for the withdrawal shape | **MET in code** (`otc.go` `payloadAdvertisesNLRI`, gating egress at `otc.go` and ingress at `otc.go`; MP_UNREACH family read at `otc.go`). See the evidence table below. **Functional `.ci` + interop coverage NOT MET** -- still owed |

**Why the gate is where it is, and why it must not be loosened.** Stamping a
non-advertising UPDATE is wire-visible damage three times over, and the third
reason is the one a future reader is most likely to miss:

1. RFC 4271 Section 4.3 -- a withdraw-only UPDATE "will not include path
   attributes or Network Layer Reachability Information", so a stamped
   withdrawal is a message the base spec says cannot exist.
2. RFC 7606 Section 5.2 -- an UPDATE with "path attributes other than
   MP_UNREACH_NLRI and no reachable NLRI" leaves a conforming receiver unable to
   trust that the NLRI parsed, so any later attribute error escalates to
   **session reset**. A neighbour withdrawing prefixes could drop our session.
3. **It destroys End-of-RIB markers.** RFC 7606 Section 5.2 names both RFC 4724
   EoR encodings: MP_UNREACH-only with no NLRI, and "a completely empty UPDATE
   message in the case of the legacy encoding". An added attribute stops either
   being an EoR at all, breaking graceful-restart convergence for the receiving
   peer. This is REACHABLE, not theoretical: the route-server fast path
   (`internal/component/bgp/reactor/reactor_notify.go`) admits a received
   UPDATE to the forward rails on `msgType == TypeUPDATE` alone, and an EoR is
   an UPDATE. Locally originated EoRs are safe only by a different route --
   `AnnounceEOR` calls `peer.SendUpdate` directly
   (`reactor_api_forward.go`), bypassing egress filters -- so this gate is
   the ONLY thing protecting a relayed one.

**AC-11 evidence** (`ai/rules/completion.md` Evidence Standards -- each row
names the assertion, not the suite):

| Shape | Test | Assertion |
|-------|------|-----------|
| Pure IPv4 withdrawal, egress to Customer | `TestOTCEgressNoStampOnPureWithdrawal` (`otc_test.go`) | `mods.Len() == 0` on a `withdrawnLen=4, attrLen=0` payload |
| MP_UNREACH-only (IPv6 unicast), egress to Customer | `TestOTCEgressNoStampOnMPUnreachOnly` (`otc_test.go`) | `mods.Len() == 0` |
| RFC 4724 legacy End-of-RIB (empty UPDATE), egress to Customer | `TestOTCEgressNoStampOnLegacyEndOfRIB` (`otc_test.go`) | `mods.Len() == 0` on `00 00 00 00` |
| Mixed withdraw + announce, egress to Customer (the false-negative direction) | `TestOTCEgressStampsMixedWithdrawAndAnnounce` (`otc_test.go`) | exactly one `AttrOp`, code 35, value `65000` -- the gate must NOT over-reach |
| Pure IPv4 withdrawal, ingress from Provider | `TestOTCIngressNoStampOnPureWithdrawal` (`otc_test.go`) | returned `modified` is `nil` (no `insertOTCInPayload` rewrite) |
| MP_UNREACH-only, ingress from Provider | `TestOTCIngressNoStampOnMPUnreachOnly` (`otc_test.go`) | returned `modified` is `nil` |
| Family scoping via MP_UNREACH | `TestIsPayloadUnicastMPUnreachFamily` (`otc_test.go`) | vpn/flow/evpn/multicast withdrawals -> `false`; v4/v6 unicast -> `true`; MP_REACH wins when both present |
| Family scoping end-to-end (discriminating) | `TestOTCNonUnicastWithdrawalSkipsOTCProcedures` (`otc_test.go`) | a VPNv4 withdrawal carrying OTC is NOT suppressed by egress rule 2, and an EVPN one is NOT rejected as a leak on ingress |
| Attributes present, no reachable NLRI (the strict form) | `TestOTCNotStampedWithoutReachableNLRI` (`otc_test.go`) | `payloadAdvertisesNLRI` is `false` for `(attrs, nil)` and `true` for `(attrs, nlri)`; egress queues no op and ingress returns `modified == nil` |

**Mutation verification** (`ai/rules/testing.md`), each reverted and the
package re-run green:

| Mutation | Reds -- and only these |
|----------|------------------------|
| stamp gate forced open (always stampable) | the five no-stamp tests |
| stamp gate forced shut | `TestOTCEgressStampsMixedWithdrawAndAnnounce` -- the gate is bounded in BOTH directions, so it cannot silently drop a mandatory stamp |
| `mpUnreachAttrCode` replaced with an unused code | `TestIsPayloadUnicastMPUnreachFamily` and both subtests of `TestOTCNonUnicastWithdrawalSkipsOTCProcedures` |
| gate loosened back to "any non-MP_UNREACH attribute counts" | all three subtests of `TestOTCNotStampedWithoutReachableNLRI`, and nothing else -- that test is what holds the strict form |

**RFC-tagged fixtures corrected (Thomas approved 2026-07-27).** Four tests built
`buildTestPayload(buildTestAttrs(0), nil)` -- attributes but no NLRI -- and asserted a
stamp, so they encoded the absent gate and were the only thing preventing the strict
form. Each now carries an NLRI; every assertion is byte-identical, and each change
records `// rfc-test-change-approved: 2026-07-27 ...` (auditable via
`grep -rn 'rfc-test-change-approved:'`).

| Test | Tag it carries |
|------|----------------|
| `TestOTCEgressStampMod` (`otc_test.go`) | RFC9234-5-4 positive, RFC9234-5-10 negative |
| `TestOTCEgressStampLocalASN` (`otc_test.go`) | RFC9234-5-9 positive |
| `TestOTCIngressFilter/stamp_from_provider` (`otc_test.go`) | RFC9234-5-3 positive |
| `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` (`otc_test.go`) | RFC9234-5-4 positive -- tagged during this work, so it became hook-protected mid-flight; same correction, same rationale |

Three further stamp tests carry no RFC tag and took the same fixture correction with no
authorisation needed: `TestOTCEgressStampRSClient`,
`TestOTCIngressStampsWhenPeerSentNoRoleCapability`, and
`TestOTCEgressStampFailClosedObservability`. `TestOTCEgressStampPeer` was left alone --
despite its name it asserts Gao-Rexford *suppression*, not a stamp, so the gate never
reaches it.

## End-to-End User Stories (MANDATORY for new features)

<!-- For each user-facing operation the feature enables, trace the full path.
     This section catches missing code that narrow ACs miss. ACs verify individual
     components work; user stories verify the full chain is connected.
     Every story must have a corresponding functional or wiring test. -->

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures `role import customer` on a Provider peer and expects that peer's routes never to reach another Provider, including after a session flap replays them from the Adj-RIB-In | peer-up -> `RelayStoredRoute` (`Meta: nil`) -> egress filter chain -> `OTCEgressFilter` -> `resolveSrcRole` config fallback -> suppress | `TestOTCEgressSuppressProviderLearnedWithoutMeta` (`otc_test.go`); the OTC egress behaviour itself is covered end-to-end by `test/plugin/role-otc-egress-filter.ci` |
| 2 | peers with an early-adopter neighbour that does not send the RFC 9234 Role capability, and expects role policy to work from local config | OPEN with no Role capability -> `resolvePeerRole` config complement -> Section 5 gates on both ingress and egress | `TestOTCIngressStampsWhenPeerSentNoRoleCapability` (`otc_test.go`), `TestOTCEgressSuppressToProviderWithoutRoleCapability` (`otc_test.go`) |
| 3 | advertises an iBGP-learned or locally originated route to a Customer and expects it to carry OTC so a downstream leak is catchable | forward rail -> `OTCEgressFilter` stamping block -> `mods.Op` -> `buildModifiedPayload` step 6 -> `otcAttrModHandler` -> wire | `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` (`otc_test.go`); wire-level in `test/plugin/role-otc-egress-stamp.ci` |
| 4 | mistypes a peer name in a `role` block and expects to be told, not to get silent no-op policy | config load -> `parseRoleContainer` -> keying check -> WARN + reject | `TestRoleConfigWithoutUsableRemoteIPIsRejected` (`config_keying_test.go`), `TestUnusableRoleConfigDoesNotShadowUsablePeers` (`config_keying_test.go`) |
| 5 | asks why a peer stopped receiving routes | any suppression -> `recordDrop` -> `ze_bgp_role_drops_total` by reason + one-shot WARN | `TestRoleDropsAreCounted` (`metrics_test.go`), `TestRoleFirstDropEmitsWarn` (`metrics_test.go`) |
| 6 | withdraws a prefix toward a Customer and expects a plain withdraw-only UPDATE on the wire | forward rail -> `OTCEgressFilter` -> **no advertisement gate** -> `mods.Op` -> a 7-byte OTC attribute on a withdraw-only UPDATE | **NONE. This path is BROKEN** -- see AC-11 and the OPEN BLOCKER |

<!-- If a path has a broken link (no implementation at some step), that is a spec gap.
     Add the missing component to ACs, Files to Create, and TDD Test Plan before proceeding. -->

## 🧪 TDD Test Plan

All paths below are `internal/component/bgp/plugins/role/`.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOTCEgressSuppressProviderLearnedWithoutMeta` | `otc_test.go` | AC-1 -- the config fallback fires with `meta == nil`. Carries `RFC requirement: RFC9234-3.1-1 positive` | Done |
| `TestOTCEgressMetaTakesPrecedenceOverConfig` | `otc_test.go` | AC-2 -- a usable meta value wins over config | Done |
| `TestOTCEgressMalformedMetaTakesConfigFallback` | `otc_test.go` | AC-3 -- a PRESENT but non-string value takes the fallback, over a source that HAS config so the branch is distinguishable | Done |
| `TestOTCEgressFilter/meta_wrong_type_not_suppressed` | `otc_test.go` | AC-3/AC-4 -- a non-string value over a source with NO config yields no suppression | Done |
| `TestOTCEgressNoStampProvider` | `otc_test.go` | AC-5 -- legitimate transit to a Provider is accepted and unstamped. Retains `RFC requirement: RFC9234-5-4 negative`; fixture changed under `// rfc-test-change-approved:` | Done |
| `TestOTCEgressSuppressToProviderWithoutRoleCapability` | `otc_test.go` | AC-6 -- destination-side config complement | Done |
| `TestOTCIngressStampsWhenPeerSentNoRoleCapability` | `otc_test.go` | AC-7 -- ingress-side config complement, asserting the stamp carries the REMOTE AS | Done |
| `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` | `otc_test.go` | AC-8 -- the stamp is destination-conditioned only | Done (untagged -- see OPEN BLOCKER) |
| `TestRoleConfigWithoutUsableRemoteIPIsRejected`, `TestRoleConfigNamedByAddressWithoutConnectionBlockIsKept`, `TestRoleCapabilityNotDeclaredForUnusablePeer`, `TestRoleConfigWithUsableRemoteIPStillKeyedByAddress`, `TestUnusableRoleConfigDoesNotShadowUsablePeers` | `config_keying_test.go,118,142,157,189` | AC-9 -- both edges of the keying rejection | Done |
| `TestReconnectWithoutRoleCapabilityClearsStaleRole`, `TestReconnectWithNewRoleCapabilityWinsOverPrevious`, `TestReconnectWithUnassignedRoleValueClearsStaleRole`, `TestClearedRoleIsKeyedLikeTheSetter`, `TestStaleRoleClearedEvenWhenOpenIsRejected`, `TestReconnectWithoutRoleIsObservable` | `session_role_test.go,91,125,151,179,206` | AC-10 (clearing half) | Done |
| `TestRoleDropsAreCounted`, `TestRoleAcceptedRouteIsNotCounted`, `TestRoleFirstDropEmitsWarn`, `TestRoleMetricsSafeBeforeConfigure` | `metrics_test.go,255,302,346` | AC-10 (observability half) | Done |
| (none) | - | **AC-11 -- no test asserts that a withdraw-only payload is NOT stamped** | **MISSING** |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| OTC attribute value (4-byte ASN) | 0..4294967295 | 4294967295 | N/A (unsigned) | N/A (`uint32` by type) -- covered by `TestOTCBoundaryASN` (`otc_test.go`) |
| OTC attribute length | MUST be exactly 4 (RFC 9234 Section 5) | 4 | 3 -> treat-as-withdraw | 5 -> treat-as-withdraw -- covered by `TestOTCBoundaryLength` (`otc_test.go`) |
| rewritten `attrLen` after an ingress stamp | 0..65535 | 65535 | N/A | overflow returns nil and the route is accepted unmodified (`insertOTCInPayload`, `otc.go`) |

This spec adds no new numeric input; the rows above are the pre-existing coverage the
change had to keep green.

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests — unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `role-otc-egress-filter` | `test/plugin/role-otc-egress-filter.ci` | a Provider-learned route is not advertised to a Provider | Done (pre-existing, kept green) |
| `role-otc-egress-stamp` | `test/plugin/role-otc-egress-stamp.ci` | a route advertised to a Customer carries OTC on the wire | Done (pre-existing, kept green) |
| `role-otc-ingress-reject` | `test/plugin/role-otc-ingress-reject.ci` | a leaked route from a Customer is rejected | Done (pre-existing) |
| `role-otc-unicast-scope` | `test/plugin/role-otc-unicast-scope.ci` | OTC is not applied to a non-unicast family | Done (pre-existing) -- proves the MP_REACH branch only; the withdrawal shape is the AC-11 gap |
| `role-otc-export-unknown` | `test/plugin/role-otc-export-unknown.ci` | `export unknown` still targets capability-less peers after the `resolvePeerRole` widening | Done (pre-existing, and the regression guard for R-4) |
| (none) | - | **a withdraw-only UPDATE toward a Customer leaves the wire without an OTC attribute** | **MISSING -- AC-11** |

No NEW `.ci` was written: this work changed decision logic inside filters the existing
`role-otc-*.ci` suite already drives end to end, and the one behaviour that would need a
new `.ci` (AC-11) is not implemented.

### Interop Tests (MANDATORY for protocol features)
<!-- REQUIRED when the spec adds/changes wire protocol behavior (BGP, IPsec, L2TP). -->
<!-- See ai/rules/interop-and-goal-validation.md for when interop is required. -->
<!-- Skip this section (with justification) only for non-protocol features. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (none for AC-1..AC-10) | - | - | The landed changes alter WHICH routes are advertised, not the wire encoding of any message a peer parses. A conforming peer cannot distinguish "ze suppressed this route because config said so" from "ze had no such route", so an interop scenario would be vacuous in the sense `ai/rules/interop-and-goal-validation.md` names: reverting the change would leave the peer's routing table identical | N/A with justification |
| withdraw-only UPDATE against a strict receiver | `test/interop/scenarios/` | FRR or BIRD | **REQUIRED for AC-11 and genuinely non-vacuous:** RFC 7606 Section 5.2 makes a conforming receiver session-reset on a withdraw-only UPDATE carrying a non-MP_UNREACH attribute, so reverting the fix produces an observable NOTIFICATION. This is the one part of this spec that interop can actually discriminate | **MISSING -- AC-11** |

### Future (if deferring any tests)
- Nothing is deferred. The missing AC-11 tests are not deferred work; they are part of an
  unfixed defect that blocks closure (`ai/rules/completion.md`: a reproducible defect has
  no recording path).

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
<!-- Check // Design: annotations on each file — if the change affects behavior
     described in the referenced architecture doc, include the doc here too -->
- `internal/component/bgp/plugins/role/otc.go` - `resolveSrcRole`, `resolvePeerRole`, both filters' gates, the stamping block, and the `recordDrop` call sites
- `internal/component/bgp/plugins/role/otc_test.go` - the new and adjusted unit tests
- `internal/component/bgp/plugins/role/config.go` - the unreachable-key rejection
- `internal/component/bgp/plugins/role/role.go` - the OPEN-time clear of capability-learned roles
- `internal/component/bgp/plugins/role/register.go` - metric registration wiring
- `internal/component/bgp/reactor/reactor_api_relay.go` - the comment documenting the gap this spec closed (`ai/rules/stale-comments.md`)
- `docs/architecture/meta/role.md` - the `src-role` metadata contract (the `// Design:` target of the changed filter code)
- `docs/plugin-development/metrics.md` - the new drop counter
- `ai/RFC-REQUIREMENTS.md` - regenerated; a new RFC-tagged test shifts the ledger

### BGP Family Checklist (if new SAFI / capability / attribute)

N/A. This spec adds no address family, SAFI, capability or path attribute. The OTC
attribute (type 35) and the Role capability (code 9) both predate it; every change here
is to the DECISION logic that gates when they are read, written or acted on. The
`ai/patterns/bgp-family.md` checklist has no applicable row, so it is removed rather than
left as twenty unanswered checkboxes.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | The `role { import ... }` / `role { export ... }` containers already exist in `internal/component/bgp/plugins/role/yang/`. No leaf added, renamed or retyped |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | No new leaf. The unreachable-key rejection (AC-9) is a resolution-time check in `config.go`, not a schema constraint: it depends on whether the peer's remote IP resolves, which the schema cannot express |
| CLI commands/flags | No | No command added or renamed |
| CLI grammar (action before identifier) | No | No command added |
| Editor autocomplete | No | No new leaf |
| Functional test for new RPC/API | No | No RPC or API added. The existing `test/plugin/role-otc-*.ci` suite drives the changed filters end to end |
| Pipe completeness | No | No new command output |
| Env var registration | No | No `environment/` leaf added |
| Doctor check for runtime dependencies | No | No file path, socket, port, module, binary or certificate introduced. The AC-9 rejection is a config-time WARN naming the peer, which is the right surface for a config mistake |
| Prometheus counters/metrics | **Yes -- done** | `internal/component/bgp/plugins/role/metrics.go`. Two counters: `ze_role_route_rejects_total` (ingress, route made ineligible) and `ze_role_route_suppressions_total` (egress, advertisement withheld), each labeled `reason`. Five bounded reason values -- `leak`, `malformed-otc`, `otc-present`, `source-role`, `export-set` -- so cardinality is five series per metric and never per-peer; peer identity goes in the log line. Every child is pre-resolved at build time (`metrics.go`) because `CounterVec.With` allocates a `[]string` per call and this is the forward path (`ai/rules/performance.md`), and pre-creating each child means the series exists at 0 from startup so a rate alert does not wait for it to appear. Documented at `docs/plugin-development/metrics.md` |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- Every No MUST be backed by a source-aware check, not a guess. At minimum, grep docs for source anchors pointing at changed files. -->
<!-- Any factual doc change MUST include or update a source-anchor HTML comment after the claim. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | No feature added. RFC 9234 role support was already listed; this work made existing gates fire in cases where they had silently not |
| 2 | Config syntax changed? | No | No leaf added, renamed or retyped. The AC-9 change REJECTS a config shape that used to be accepted-and-ignored, which is a diagnostic change, not a syntax change |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | `bgp-role`'s registration, name, families and capability set are untouched |
| 6 | Has a user guide page? | **Yes -- NOT updated. Gap.** | `docs/guide/bgp-role.md` exists and carries three source anchors into the role plugin. It describes when OTC is stamped and when routes are suppressed -- the exact behaviour five commits changed. No commit in this set touched it. Flagged for the reviewer; it is not blocking on its own, but it is a real omission |
| 7 | Wire format changed? | No | The OTC attribute encoding (`buildOTCAttr`, `otcAttrModHandler`) is byte-identical. WHEN it is emitted changed; HOW it is encoded did not |
| 8 | Plugin SDK/protocol changed? | No | `filterapi.IngressFilterFunc` / `EgressFilterFunc` signatures unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | **Yes -- partially, and one claim is now known-false** | `docs/features/rfc-status.md` still reads "No tracked gap in current source anchors" for RFC 9234 and credits "unicast-only (AFI 1/2, SAFI 1) OTC scoping". The OPEN BLOCKER shows that scoping does not hold for the withdrawal shape, so that row is not backed. It must be corrected as part of the AC-11 fix, not before it |
| 10 | Test infrastructure changed? | No | No new runner, format or fixture pattern; unit tests use the existing `setFilterState` / `setFilterRemoteRole` helpers |
| 11 | Affects daemon comparison? | No | No capability gained or lost relative to other daemons |
| 12 | Internal architecture changed? | No | No layer, boundary or registration mechanism moved |
| 13 | Route metadata keys added/changed? | **Yes -- done** | `docs/architecture/meta/role.md` updated by `276096afb`, `d373d9f40`, `e0607d0f4` and `f5dd2f040`; it now carries anchors for `resolvePeerRole`/`peerRoleComplement`, `resolveExport`, `extractPeerRoleConfigs`, `applyValidateOpen`/`clearFilterRemoteRole`/`filterKeyLocked` and `recordDrop`/`buildMetrics`. No key was added or renamed; what changed is the documented meaning of an ABSENT `src-role` |
| 14 | Prometheus counters added/changed? | **Yes -- done** | `docs/plugin-development/metrics.md`, anchored to `role/metrics.go` `recordDrop` and to both filters |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | None |
| 16 | Any changed source file is referenced by existing doc source anchors? | **Yes -- one stale, one gap** | `grep -rn 'source:.*plugins/role/' docs/` returns 18 anchors. `docs/architecture/route-selection.md` anchors `otc.go` for "RFC 9234 OTC validation" and was NOT revisited; `docs/guide/bgp-role.md` (row 6) likewise. The `meta/role.md` and `metrics.md` anchors were updated. Verified by grep, not assumed |
| 17 | Existing docs show config/CLI/API examples for this area? | **Yes -- unverified** | `docs/guide/bgp-role.md` shows `role { import ... }` / `role { export ... }` examples. The syntax is unchanged, so the examples still parse; whether their described EFFECT still matches after the `resolvePeerRole` widening was not re-checked. Part of the row-6 gap |

## Files to Create
- `internal/component/bgp/plugins/role/metrics.go` + `metrics_test.go` - the drop counters (landed in `f5dd2f040`)
- `internal/component/bgp/plugins/role/config_keying_test.go` - the unreachable-key rejection tests (landed in `f5dd2f040`)
- `internal/component/bgp/plugins/role/session_role_test.go` - the stale-role clearing tests (landed in `f5dd2f040`)
- **Still owed for AC-11:** a `.ci` under `test/plugin/` proving a withdraw-only UPDATE toward a Customer carries no OTC attribute, and an interop scenario under `test/interop/scenarios/` proving a conforming receiver does not session-reset on it

## Implementation Steps

<!-- Steps must map to /implement stages. Each step should be a concrete phase of work,
     not a generic process description. The review checklists below are what /implement
     stages 5, 9, and 10 check against — they MUST be filled with feature-specific items. -->

### /implement Stage Mapping

<!-- This table maps /implement stages to spec sections. Fill during design. -->
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan — check what exists |
| 3. Wiring phase | Wiring Test table — register entry points, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist below |
| 13. /ze-review gate | Review Gate section — run `/ze-review`; fix every BLOCKER/ISSUE; re-run until 0 BLOCKER/0 ISSUE (final review gate before closure) |
| 14. Present summary + close | Executive Summary Report; two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

<!-- List concrete phases of work. Each phase follows TDD: write test → fail → implement → pass.
     Phase 1 is ALWAYS wiring: create the entry point and a failing wiring test.
     Remaining phases fill in feature logic behind the wired skeleton.
     Phases should be ordered by dependency (e.g., schema before resolution, resolution before CLI). -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

Phases 1-5 are the record of what actually happened, one per commit. Phase 6 is the
remaining work. Phase 1 is not "wiring" in the template's sense: both filters were already
registered and reachable, so the wiring test for each phase is an existing entry point
driven with a NEW fixture that reaches the previously-unreachable branch.

1. **Phase: the egress meta fallback (AC-1..AC-5)** — `c398e97f0`
   - Tests: `TestOTCEgressSuppressProviderLearnedWithoutMeta`, `TestOTCEgressMalformedMetaTakesConfigFallback`, `TestOTCEgressMetaTakesPrecedenceOverConfig`; `TestOTCEgressNoStampProvider` re-fixtured under user approval
   - Files: `otc.go` (`resolveSrcRole`), `otc_test.go`, `reactor_api_relay.go` (comment), `ai/RFC-REQUIREMENTS.md`
   - Verify: mutation-verified -- deleting the fallback turns `TestOTCEgressSuppressProviderLearnedWithoutMeta` red
2. **Phase: the destination-side hole (AC-6)** — `276096afb`, found by review round 1
   - Tests: `TestOTCEgressSuppressToProviderWithoutRoleCapability`
   - Files: `otc.go` (`resolvePeerRole`, `peerRoleComplement`), `otc_test.go`, `docs/architecture/meta/role.md`
   - Verify: the same zero-value shape at the DESTINATION reader, two lines from the one just fixed
3. **Phase: the ingress hole (AC-7)** — `d373d9f40`, found by review round 2
   - Tests: `TestOTCIngressStampsWhenPeerSentNoRoleCapability`
   - Files: `otc.go` (`OTCIngressFilter`), `otc_test.go`, `docs/architecture/meta/role.md`
   - Verify: the THIRD reader of `getFilterConfig`, 155 lines above the two already swept. The existing subtest that covered the branch pinned the permissive outcome and could not detect it
4. **Phase: the source-side gate on an RFC destination-only rule (AC-8)** — `e0607d0f4`, found by review round 3
   - Tests: `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig`
   - Files: `otc.go` (removal of the `srcCfg == nil` early return), `otc_test.go`, `docs/architecture/meta/role.md`
   - Verify: this had ZERO coverage before -- making the stamp source-independent left the whole package green while the RFC ledger still reported `RFC9234-5-4` proven
5. **Phase: config keying, stale roles, observability (AC-9, AC-10)** — `f5dd2f040`
   - Tests: `config_keying_test.go` (5 tests), `session_role_test.go` (6 tests), `metrics_test.go` (4 tests)
   - Files: `config.go`, `role.go`, `metrics.go`, `register.go`, `otc.go` (`recordDrop` call sites), `docs/architecture/meta/role.md`, `docs/plugin-development/metrics.md`
   - Verify: the inert-config and stale-role defects are both invisible without a test that drives a RECONNECT or a name-keyed peer
6. **Phase: the advertisement gate (AC-11)** — **NOT STARTED. This is the closure blocker**
   - Tests: a unit test asserting `mods.Len() == 0` for a pure IPv4 withdrawal and for an MP_UNREACH-only payload toward a Customer; a `test/plugin/*.ci` proving the withdraw-only UPDATE leaves the wire clean; an interop scenario against FRR or BIRD
   - Files: `otc.go` (an advertisement gate before the stamping block at `:562`; MP_UNREACH awareness in `isPayloadUnicast`), `otc_test.go`, plus the `docs/features/rfc-status.md` correction and the missing `RFC requirement:` tag on `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig`
   - Verify: the interop scenario is the non-vacuous one -- reverting the fix makes a conforming receiver session-reset per RFC 7606 Section 5.2
7. **Functional tests** → the AC-11 `.ci` above. The existing `role-otc-*.ci` suite covers phases 1-5.
5. **RFC refs** → Add `// RFC NNNN Section X.Y` comments (protocol work only)
6. **Full verification** → `make ze-verify` (lint + all ze tests except fuzz)
7. **Complete spec** → Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`. TWO commits: commit A saves code + tests + spec + learned summary; commit B does `git rm` of the spec. BLOCKING: summary is part of commit A, not a follow-up.

### Critical Review Checklist (/implement stage 6)

<!-- MANDATORY: Fill with feature-specific checks. /implement uses this table
     to verify the implementation. Generic checks from rules/quality.md always apply;
     this table adds what's specific to THIS feature. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path (no broken links in the chain). Reference feature comparison: new feature has everything the reference has |
| Correctness | Every RFC 9234 Section 5 rule is gated on exactly the peer the RFC names -- ingress rules on the SOURCE, egress rule 1 on the DESTINATION, egress rule 2 on the destination and the wire bytes. Four of the five defects in this spec were an extra gate the RFC never stated |
| Enumerate every reader, not the one that failed | This spec's whole history is a single zero-value shape found one reader at a time. Before claiming a fix, grep EVERY caller of `getFilterConfig` and every read of `meta["src-role"]` and `filterRemoteRoles`, and state what an empty value means at each |
| Fail closed AND say something | Each empty-value branch either denies or logs. `resolveSrcRole` WARNs on an unusable meta value (`otc.go`); `resolvePeerRole` WARNs on a role with no complement (`otc.go`); the stamping block WARNs when `dest.LocalAS` is 0 (`otc.go`). A guard that neither denies nor speaks does not exist (`ai/rules/evidence.md`) |
| Naming | Metric names follow `ze_{scope}_{subject}_{event}_total` with scope `role`, not `bgprole` (`metrics.go`); label values are bounded lower-kebab strings |
| Data flow | The role decision stays entirely inside the role plugin. The reactor learns of a suppression only through the filter's `bool` and of a stamp only through the `ModAccumulator`; no role vocabulary leaks into `reactor/` |
| No allocation on the forward path | `recordDrop` indexes a pre-resolved counter array; it must not call `CounterVec.With` per UPDATE (`ai/rules/performance.md`) |
| CLI grammar | If CLI commands added: action before identifier per `ai/rules/cli.md` |
| Registration over hardcoding | New command/view/family/handler is registry-registered and core-discovered; no new per-feature field, switch case, or factory added to a core/shared struct (incl. the CLI `Model`). See `ai/rules/plugins.md` |
| Doctor checks | If runtime dependencies added: `ze doctor` check registered per `ai/rules/repo-maintenance.md` |
| YANG validation | If YANG leaves added: every leaf has max native constraints (`range`/`length`/`pattern`/`enum`). Bare `type string` is a red flag. Custom validator + `CompleteFn` where native is insufficient |
| Prometheus counters | If observable state exists: counters defined, registered, metric names listed |
| Rule: no-layering | The `srcCfg == nil` early return was DELETED, not bypassed. No compatibility branch preserves the old permissive behaviour |
| Rule: testing (RFC-tagged tests) | `TestOTCEgressNoStampProvider` keeps its `RFC requirement: RFC9234-5-4 negative` tag. Its FIXTURE changed; no assertion was weakened, and the change carries `// rfc-test-change-approved:` with the date and what Thomas approved |
| Rule: no-parking | AC-11 is a reproducible defect. It is recorded as an OPEN BLOCKER on a spec that stays open, NOT as a `plan/known-failures/` shard and NOT as a deferral |

### Deliverables Checklist (/implement stage 10)

<!-- MANDATORY: Every deliverable with a concrete verification method.
     /implement re-reads the spec and checks each item independently. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `resolveSrcRole` exists and is CALLED from the egress Gao-Rexford branch | `grep -n 'resolveSrcRole' internal/component/bgp/plugins/role/otc.go` -> definition and call site, not definition alone |
| `resolvePeerRole` is called at all THREE `getFilterConfig` readers | `grep -n 'resolvePeerRole\|getFilterConfig' internal/component/bgp/plugins/role/otc.go` |
| No source-side gate sits between the export check and the stamping block | read `otc.go`; the removed `if srcCfg == nil { return true }` must not be back in any form |
| Unreachable role config is rejected, not stored | `go test -run 'TestRoleConfig' ./internal/component/bgp/plugins/role/` |
| Stale capability roles are cleared at the OPEN | `go test -run 'TestReconnect|TestStaleRole|TestClearedRole' ./internal/component/bgp/plugins/role/` |
| Every suppression is counted | `grep -n 'recordDrop' internal/component/bgp/plugins/role/otc.go` -> one call per `return false` site |
| **AC-11: no OTC on a withdraw-only payload** | **FAILS today.** `grep -n 'mpUnreach\|unreach' internal/component/bgp/plugins/role/otc.go` returns nothing, and `otc.go` has no advertisement gate |

### Security Review Checklist (/implement stage 11)

<!-- MANDATORY: Feature-specific security concerns. /implement checks each item.
     Think about: untrusted input, injection, resource exhaustion, error leakage. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | Every offset arithmetic on the attacker-controlled payload is bounds-checked before indexing: `findOTC` (`otc.go`), `isPayloadUnicast`, `extractAttrsFromPayload`, `insertOTCInPayload`, `payloadToWithdrawal`. Each returns nil or breaks rather than slicing past the end |
| Integer overflow | `insertOTCInPayload` rejects a rewritten `attrLen > 65535` and accepts the route unmodified rather than truncating |
| Trusting a peer's assertion over local config | `resolvePeerRole` prefers the peer's CAPABILITY over local config when both exist. That is RFC 9234 Section 4.2's own precedence and is validated against config by `validateOpenRolePair`, so a peer cannot claim an arbitrary role and have it accepted -- but it is the one place a remote value outranks a local one, and it must stay bounded by that validation |
| Fail-open on a filter panic | `safeEgressFilter` recovers a panicking filter; a recovered panic must not be reported as "accept". Egress filters return a bare `bool` with no failure channel, which is a live gap homed on `plan/spec-fixit-stored-route-relay-hardening.md` |
| **Wire output a peer can weaponise** | **OPEN.** The stamped withdrawal (AC-11) puts a non-MP_UNREACH attribute on a withdraw-only UPDATE, which RFC 7606 Section 5.2 makes a conforming receiver treat with "session reset". A neighbour withdrawing prefixes can therefore drop our session with it |
| Metric cardinality | Labels are five compile-time constants; peer identity is deliberately kept out of the label set and put in the log line (`metrics.go`), so a peer cannot inflate the series count |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Fixing `meta["src-role"]` closed the missing-value hole | The SAME shape sat at four more readers: the egress destination role, the ingress source role, the `srcCfg == nil` early return, and the config key itself | four successive independent review rounds, each finding the next one | the spec took five commits instead of one. Every round found a real, wire-visible defect, so none of the rounds was wasted -- but rounds 2, 3 and 4 were all findable in round 1 by enumerating the readers instead of fixing the reported one |
| `TestOTCEgressNoStampProvider`'s `RFC9234-5-4 negative` tag meant the requirement was proven | The fixture only read as `accept` because meta was nil, i.e. it passed for the wrong reason, and it had no `LocalAS` so the stamp was refused by the `localASN > 0` guard rather than by the destination-role gate. The test passed with the destination gate deleted | reading the fixture while preparing to change it | an RFC-tagged test can be vacuous. The tag records intent, not proof; only mutation-verification distinguishes them |
| `ai/RFC-REQUIREMENTS.md` reporting `RFC9234-5-4` proven meant the MUST was enforced for every source | It was credited to a test whose SOURCE has role config, so the source-gated stamping bug (AC-8) was invisible to the ledger | review round 3 | the ledger proves a requirement is TESTED, not that the test covers every input class the requirement names |
| An empty `filterRemoteRoles` entry means "the peer advertised no role" | It also meant "we never recorded it" and "a previous session recorded it and nobody cleared it" | reviews plus the `f5dd2f040` audit | a stale role outranked config in `resolvePeerRole`, so a peer that dropped its Role capability kept the old one across a reconnect |
| A `role` block configured on a peer takes effect | If the peer's remote IP does not resolve, the entry is keyed by NAME while all three readers key by ADDRESS, so the whole block was inert with no diagnostic | the `f5dd2f040` keying audit | the config surface accepted input it silently ignored -- the operator-facing form of the same zero-value trap |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Fix the gap at the `RelayStoredRoute` caller by populating meta before the replay | The gap is at the READ. Any present or future egress caller without ingress metadata has it, and A-2 was confirmed by events: three more readers of the same shape turned up in different call paths | the fallback lives in `resolveSrcRole`, at the read |
| Return `""` from `resolveSrcRole` when meta holds a non-string value | A malformed input would then be MORE permissive than a missing one, which inverts the failure ordering | present-but-unusable takes the config fallback AND logs a WARN |
| Treat `roleUnknown` as the resolved destination role in the export-set match too, for consistency with the RFC gates | `unknown` is a documented operator token meaning "also send to peers with no role configured" (`config.go`). Reclassifying it there would silently retarget an operator knob -- policy, not conformance | `resolvePeerRole` feeds the RFC MUST gates only; the export match keeps the capability-only value, with the reason recorded at `otc.go` |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Fixing the reported reader of a zero-value trap instead of enumerating every reader | five times in this one spec | already covered by `ai/rules/evidence.md` and the Sibling Call-Site Audit in `ai/rules/architecture.md`. What is missing is not a rule but the habit of treating a fail-open finding as a CLASS: the first fix should grep every reader of the same producer and state what an empty value means at each | no new rule proposed. The Critical Review Checklist above carries an "Enumerate every reader, not the one that failed" row so the next reviewer of this area applies it |
| An RFC-tagged test that passes for the wrong reason | twice here (`TestOTCEgressNoStampProvider`, and `RFC9234-5-4` credited to a source-configured test) | `ai/rules/interop-and-goal-validation.md` already mandates proving the test discriminates. The gap is that `ze-rfc-check` counts a TAG, and a tag cannot be mutation-verified mechanically | flagged for the reviewer; no rule change proposed from a single spec |

## Design Insights

- **The RFC conditions egress rule 1 on the destination alone.** Every gate this spec
  removed was a source-side condition the RFC never stated. Reading the requirement text
  as a predicate ("if a route IS TO BE ADVERTISED to a Customer/Peer/RS-Client, and OTC is
  not present, then add it") names exactly two inputs: the destination's role, and whether
  OTC is already there. A third input in the code is a bug by construction. That reading
  is also what makes AC-11 obvious once stated: "is to be advertised" is the FIRST clause,
  and no code checks it.
- **`meta` and config are the same value at one instant, not the same value.** Calling the
  fallback an "exact recovery" is right about the field and wrong about the time. Meta is
  captured at RECEIVE; config is read at FORWARD. A reload between them makes them differ,
  and the forward-time value is the better one to gate a leak check on -- but a reader who
  takes "exact" literally will assume an identity that does not hold.
- **An operator knob and an RFC gate can read the same variable and must not share a
  resolution.** `roleUnknown` in the export set is an operator TARGET; an empty role at an
  RFC gate is an unanswered question. Widening one resolution for both would have silently
  changed what `export unknown` matches.

## Core Insight

A value that can be legitimately empty is not one bug, it is a bug at every reader. This
spec fixed the same shape five times because each round treated the reported symptom as
the defect. The producer here is `getFilterConfig` plus the shared `ingressMeta` map, and
the moment the first fail-open was found the right move was to enumerate every consumer
of both and state what empty means at each -- which would have surfaced all five in one
pass, and would have surfaced AC-11 too, since "no reachable NLRI" is the same question
asked of the payload instead of the config.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fix at the READ (`resolveSrcRole`), not at the `RelayStoredRoute` caller | populate meta in the relay path | the gap belongs to any caller without ingress metadata, present or future. Confirmed by events: three further readers of the same shape were found in other call paths |
| A present-but-unusable meta value takes the config fallback AND logs | return `""`; or trust the value | a malformed input must never be more permissive than a missing one, and a producer bug must not be silent -- one `ingressMeta` map is shared by every in-process ingress filter, so any filter can clobber the key |
| `resolvePeerRole` prefers the capability, then the config complement | config first; or capability only | RFC 9234 Section 4.2: "The locally configured BGP Role is used for the procedures described in Section 5", and Section 8 names the non-compliant remote as the expected early-adopter case. Capability-only was the zero-value trap; config-first would ignore what the peer actually negotiated |
| `resolvePeerRole` is scoped to the RFC MUST gates and NOT the export-set match | apply it everywhere for consistency | `unknown` is an operator-selected export target, not a missing answer. Consistency would have retargeted a knob |
| Reject an unreachable `role` config key with a WARN naming the peer | keep it and make the readers try both keys; or silently drop it | making readers try both keys spreads the keying question to three call sites and hides the operator's mistake. A peer NAMED for its address still works, because peers very often are |
| Clear a stale capability role at the OPEN, not on session down | clear on the state-down event | the structured state event carries no session identity, and for an in-process plugin the down arrives on a different loop, so a late down could delete a role a NEWER session had already written |
| Keep the spec OPEN rather than close it with AC-11 outstanding | close on AC-1..AC-10 and file AC-11 as a follow-up | AC-11 is a reproducible, wire-visible, interop-fatal defect in the code this spec is about. `ai/rules/completion.md`: a reproducible defect has no recording path, and "pre-existing in shape" says when it started, not whose it is |

## Known Limitations
- **Peers created from a dynamic group receive no per-peer role config at all.** The
  config extraction walks configured peers; a peer that only exists at runtime has no
  entry, so every role gate reads `""` for it. `bgp-rpki` has the identical limitation.
  Homed on `plan/spec-fixit-dynamic-group-peer-config.md`, which exists.
- **An egress filter has no failure channel.** `filterapi.EgressFilterFunc` returns a bare
  `bool`, so "could not evaluate" and "policy says no" are indistinguishable to the
  reactor. Closing it means changing a signature used by three plugins. Homed on
  `plan/spec-fixit-stored-route-relay-hardening.md`.
- **`meta` and config can disagree across a config reload.** Accepted, not fixed; the
  forward-time value is the safer one. Recorded at `otc.go`.
- **AC-11 is not a limitation, it is an unfixed defect.** Listed here only so a reader
  scanning this section does not conclude the spec's scope is settled: see the OPEN
  BLOCKER.

## RFC Documentation

The enforcing sites carry their requirement text inline: RFC 9234 Section 5's three
ingress rules above `checkOTCIngress` (`otc.go`), egress rule 2 above
`checkOTCEgress` (`otc.go`) and at its call site (`otc.go`), egress rule 1
above the stamping block (`otc.go`), the unicast scoping above both filters'
family gates (`otc.go`, `:334`, `:486`), the preserve-unchanged rule above
`otcAttrModHandler` (`otc.go`), and Section 4.2's "the locally configured BGP Role is
used" in `resolvePeerRole`'s doc (`otc.go`). The AC-11 fix must add the "is to be
advertised" clause of egress rule 1 above the new gate, and an RFC 7606 Section 5.2 note
explaining why a stamped withdrawal is interop-fatal rather than merely untidy.

## Implementation Summary

### What Was Implemented
Five commits, each closing a different reader of one zero-value shape.

- **`c398e97f0` -- the egress meta fallback (AC-1..AC-5).** `resolveSrcRole`
  (`otc.go`) returns the meta value when it is a usable string, else
  `srcCfg.role`, else `""`; it WARNs on a present-but-unusable value. Called from the
  `destRemoteRole` branch at `otc.go`. `reactor_api_relay.go`'s comment updated from
  "this gap is open" to naming the proving test. `ai/RFC-REQUIREMENTS.md` regenerated.
- **`276096afb` -- the destination-side hole (AC-6).** `resolvePeerRole`
  (`otc.go`) plus the `peerRoleComplement` table resolve what a peer
  IS to us from the config complement when it sent no Role capability. Wired at the
  destination reader (`otc.go`).
- **`d373d9f40` -- the ingress hole (AC-7).** The same resolution applied at the THIRD
  `getFilterConfig` reader (`otc.go`), 155 lines above the other two. Without it an
  empty role returned early and skipped all three Section 5 ingress MUSTs.
- **`e0607d0f4` -- the source gate on a destination-only rule (AC-8).** The
  `if srcCfg == nil { return true }` early return between the export check and the
  stamping block was DELETED. RFC 9234 Section 5 conditions the stamping rule on the
  destination alone, so every route from a config-less source (iBGP, RR client, local
  origination, API injection) had been reaching a Customer without OTC.
- **`f5dd2f040` -- keying, stale roles, observability (AC-9, AC-10).** Role config keyed
  by a name that no reader can look up is now rejected with a WARN naming the peer
  (`config.go`), while a peer NAMED for its address still works. Capability-learned roles
  are cleared at the OPEN rather than on session down (`role.go`), because the structured
  state event carries no session identity and a late down could delete a newer session's
  role. Every suppression is counted: `ze_role_route_rejects_total` and
  `ze_role_route_suppressions_total`, five bounded `reason` values, children pre-resolved
  so the forward path allocates nothing (`metrics.go`), with a one-shot WARN on the first
  drop.

**What was NOT implemented: AC-11.** See the OPEN BLOCKER.

### Bugs Found/Fixed
Every one was found by an independent review round, not by a failing test, and every one
was wire-visible.

- **Fail-open on the egress Gao-Rexford net when meta lacked `src-role`.** Fixed by
  `resolveSrcRole`; covered by `TestOTCEgressSuppressProviderLearnedWithoutMeta`.
- **Fail-open on the egress DESTINATION role for a peer that sent no Role capability.**
  Fixed by `resolvePeerRole`; covered by `TestOTCEgressSuppressToProviderWithoutRoleCapability`.
- **All three Section 5 INGRESS MUSTs skipped for a peer that sent no Role capability.**
  Fixed by the same resolution at the ingress reader; covered by
  `TestOTCIngressStampsWhenPeerSentNoRoleCapability`. The pre-existing subtest over that
  branch (`config_but_no_remote_role`) pinned the permissive outcome with a fixture whose
  complement is Customer, which no ingress rule bites -- it read as coverage while proving
  nothing.
- **OTC never stamped toward a Customer when the SOURCE had no role config.** Fixed by
  deleting the `srcCfg == nil` early return; covered by
  `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig`. This had zero coverage: making
  the stamp source-independent left the package green while the RFC ledger still reported
  `RFC9234-5-4` proven, by a test whose source DOES have role config.
- **A whole `role` config block silently inert when the peer's remote IP did not resolve.**
  Fixed by rejecting an unreachable key with a WARN; covered by `config_keying_test.go`.
- **A capability-learned role surviving a reconnect that carried no Role capability.**
  Fixed by clearing at the OPEN; covered by `session_role_test.go`.
- **Every suppression invisible at the default log level.** Fixed by `recordDrop` plus a
  first-drop WARN; covered by `metrics_test.go`.
- **OPEN, NOT FIXED: OTC stamped onto withdrawals.** Found by review round 4. No test
  covers it. See the OPEN BLOCKER.

### Documentation Updates
- `docs/architecture/meta/role.md` -- updated by four of the five commits. Anchors now
  cover `resolvePeerRole`/`peerRoleComplement`, `resolveExport`,
  `extractPeerRoleConfigs`, `applyValidateOpen`/`clearFilterRemoteRole`/`filterKeyLocked`
  (`:82`) and `recordDrop`/`buildMetrics`. No metadata KEY changed; what changed
  is the documented meaning of an ABSENT `src-role`.
- `docs/plugin-development/metrics.md` -- the drop counters, anchored to
  `role/metrics.go` `recordDrop` and to both filters.
- `ai/RFC-REQUIREMENTS.md` -- regenerated in four of the five commits; a new or moved
  RFC-tagged test shifts the ledger's `file:line` records and `ze-rfc-check` fails on a
  stale one.
- `ai/CODE-TO-DOCS.md` -- regenerated by `e0607d0f4`.
- **Not updated, and it should have been:** `docs/guide/bgp-role.md` (three anchors into
  the role plugin, describing exactly the stamping and suppression behaviour these commits
  changed) and `docs/architecture/route-selection.md` (anchors `otc.go` for "RFC 9234
  OTC validation"). Found by `grep -rn 'source:.*plugins/role/' docs/` on 2026-07-27.
- **Known-false claim left standing:** `docs/features/rfc-status.md` reads "No tracked
  gap in current source anchors" and credits "unicast-only (AFI 1/2, SAFI 1) OTC scoping".
  The OPEN BLOCKER disproves the scoping claim for the withdrawal shape. It must be
  corrected as part of the AC-11 fix; correcting it first would be recording a defect
  instead of fixing it.
- `make ze-doc-test` result: not run in this spec-fill pass, which touched no file under
  `docs/`.

### Deviations from Plan
- **Scope grew fourfold, by discovery rather than by decision.** The Task section names
  ONE defect and AC-1..AC-5 describe it. Four further commits landed under this spec's
  name, each closing a different reader of the same zero-value shape, each found by an
  independent review round. AC-6..AC-10 were written retroactively on 2026-07-27 so the
  audit describes the diff that exists rather than the diff that was planned. This is a
  deviation worth naming rather than absorbing: a spec whose ACs cover a fifth of its
  commits reports "all ACs Done" over work nobody specified.
- **"Recoverable EXACTLY" was qualified, not delivered as stated.** The Task section calls
  the config value an exact recovery of `meta["src-role"]`. It is the same FIELD of the
  same peer's config, but read at a different TIME, so a config reload between receive and
  forward makes them differ. Recorded at `otc.go` rather than silently accepted.
- **An RFC-tagged test's fixture was changed under explicit approval.**
  `TestOTCEgressNoStampProvider` kept its `RFC9234-5-4 negative` tag; its fixture was
  corrected twice (source role, then `LocalAS`) because without `LocalAS` the stamp is
  refused by the `localASN > 0` guard and the test passes with the destination-role gate
  deleted. Both changes ADD failure modes; no assertion was weakened. Marker and date at
  `otc_test.go`.
- **The spec is NOT closed.** AC-11 is open, so this is an in-progress spec with a filled
  audit, not a completed one awaiting bookkeeping.

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Close the missing-metadata gap in `OTCEgressFilter`'s Gao-Rexford safety net | Done | `role/otc.go` `resolveSrcRole`, CALLED at `otc.go` inside the `destRemoteRole` branch | Fixed at the READ, not at the `RelayStoredRoute` caller, because the gap fires for ANY caller lacking meta |
| Recover the value EXACTLY from config rather than guessing | Done | `resolveSrcRole` returns `srcCfg.role`, the field `OTCIngressFilter` copies into `meta["src-role"]` at `otc.go` | Changed: qualified in the code comment (`otc.go`). The two agree only at one INSTANT (meta captured at receive, config read at forward), so a config reload between them makes them differ; the relay-time role is the safer one to gate on |
| Requires changing an RFC-tagged test; ask before writing code | Done | Thomas approved 2026-07-27; `// rfc-test-change-approved:` marker at `otc_test.go` on `TestOTCEgressNoStampProvider` | Approval obtained BEFORE the change, per `ai/rules/testing.md`. Applied twice under the one approval (source role, then `LocalAS`); both ADD failure modes |
| **RFC 9234 Section 5 procedures apply to a peer that sent no Role capability** | Done | `resolvePeerRole` (`otc.go`), called at all THREE `getFilterConfig` readers -- ingress `:327`, egress destination `:498`; the egress source reader `:491` feeds `resolveSrcRole` instead | Requirement NOT in the Task section. Added by commits 2 and 3 after review found the same shape at readers the original fix did not touch |
| **The Section 5 stamping rule is conditioned on the DESTINATION only** | Done | the `srcCfg == nil` early return deleted; the stamping block at `otc.go` is now reached from every source | Requirement NOT in the Task section. Added by commit 4 |
| **Role config that no reader can look up is rejected, not silently stored** | Done | `role/config.go`, the unreachable-key rejection | Requirement NOT in the Task section. Added by commit 5 |
| **A capability-learned role does not outlive the session that taught it** | Done | `role/role.go`, cleared at the OPEN | Requirement NOT in the Task section. Added by commit 5. Cleared at the OPEN, not on session down: the structured state event carries no session identity, and for an in-process plugin the down arrives on a different loop, so a late down could delete a role a NEWER session had written |
| **No suppression is invisible to the operator** | Done | `role/metrics.go` `recordDrop`, one call per `return false` site (`otc.go,353,505,520,551`) | Requirement NOT in the Task section. Added by commit 5 |
| **OTC is applied only to routes being ADVERTISED** | **NOT MET** | no gate exists; `otc.go` tests only `mods != nil` and the destination role | RFC 9234 Section 5 egress rule 1 opens with "If a route is to be advertised". This is AC-11 and the closure blocker |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestOTCEgressSuppressProviderLearnedWithoutMeta` (`otc_test.go`), assertion: `assert.False(t, accept, ...)` with `meta == nil` | Mutation-verified: disabling the fallback turns it red |
| AC-2 | Done | `TestOTCEgressMetaTakesPrecedenceOverConfig` (`otc_test.go`), plus the `TestOTCEgressFilter/src_role_*` subtests (`otc_test.go`) | The subtests pass meta explicitly from a config-less source, so meta is the only signal there; the dedicated test is the one that discriminates meta from config when BOTH exist |
| AC-3 | Done | `TestOTCEgressMalformedMetaTakesConfigFallback` (`otc_test.go`) over a source that HAS config, and `TestOTCEgressFilter/meta_wrong_type_not_suppressed` (`otc_test.go`) over one that does not | The pair is deliberate. The subtest alone could not detect a `resolveSrcRole` that returned `""` on a malformed value, because its source has no config and the result is `""` either way; the test doc at `otc_test.go` records exactly that |
| AC-4 | Done | `TestOTCEgressFilter/meta_wrong_type_not_suppressed`; `10.0.0.99` has no `peerRoleConfig` entry | `resolveSrcRole` returns `""` when `srcCfg == nil` (`otc.go`); `""` matches none of `roleCustomer`/`rolePeer`/`roleRSClient` at `otc.go` |
| AC-5 | Done | `TestOTCEgressNoStampProvider` (`otc_test.go`), assertions `assert.True(t, accept)` and `assert.Equal(t, 0, mods.Len(), ...)` | Keeps its `RFC9234-5-4 negative` tag; the FIXTURE changed twice so the route actually reaches the stamp block. The `LocalAS: 65000` addition is load-bearing: without it the `localASN > 0` guard refuses the stamp and the test passes even with the destination-role gate deleted |
| AC-6 | Done | `TestOTCEgressSuppressToProviderWithoutRoleCapability` (`otc_test.go`) | Producing code: `resolvePeerRole` at `otc.go`, wired at `otc.go` |
| AC-7 | Done | `TestOTCIngressStampsWhenPeerSentNoRoleCapability` (`otc_test.go`), assertions `require.NotNil(t, modified)` and `assert.Equal(t, uint32(65060), asn)` | Asserts the stamp carries the REMOTE AS, not merely that a stamp happened. Producing code: `otc.go` |
| AC-8 | Done | `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` (`otc_test.go`), assertion `assert.Equal(t, 1, mods.Len(), ...)` from a source with NO config entry | Producing change: the deleted `srcCfg == nil` early return. **Untagged:** this test carries no `RFC requirement:` line, so `ai/RFC-REQUIREMENTS.md` still credits `RFC9234-5-4 positive` to `TestOTCEgressStampMod` (`otc_test.go`), whose source DOES have role config. Verified 2026-07-27 by `grep -n 'RFC requirement:' otc_test.go` -- the nearest tags are at `:1080` and `:1324`. Recorded in the OPEN BLOCKER |
| AC-9 | Done | `TestRoleConfigWithoutUsableRemoteIPIsRejected` (`config_keying_test.go`) for the reject, `TestRoleConfigNamedByAddressWithoutConnectionBlockIsKept` for the accept, `TestUnusableRoleConfigDoesNotShadowUsablePeers` for the blast radius | Both edges are tested, which is what makes the rejection safe: a peer named for its address still works |
| AC-10 | Done | Clearing: `TestReconnectWithoutRoleCapabilityClearsStaleRole` (`session_role_test.go`) and five siblings, including `TestStaleRoleClearedEvenWhenOpenIsRejected`. Counting: `TestRoleDropsAreCounted` (`metrics_test.go`), `TestRoleAcceptedRouteIsNotCounted`, `TestRoleFirstDropEmitsWarn`, `TestRoleMetricsSafeBeforeConfigure` | `TestRoleAcceptedRouteIsNotCounted` is the discriminating half: a counter that increments on every route would pass "drops are counted" while proving nothing |
| **AC-11** | **NOT MET -- no code, no test** | nothing | The closure blocker. `otc.go` has no advertisement gate and `isPayloadUnicast` (`otc.go`) has no MP_UNREACH awareness. Not Partial and not Skipped: those statuses mean the work was scoped down with approval. This is an unfixed defect on an open spec |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Suppression with no meta | Done | `otc_test.go` `TestOTCEgressSuppressProviderLearnedWithoutMeta` | New test; carries `RFC9234-3.1-1 positive` |
| Stamping scope preserved | Done | `otc_test.go` `TestOTCEgressNoStampProvider` | Fixture changed twice, RFC tag retained, no assertion weakened |
| Wrong-type meta ignored (no source config) | Done | `otc_test.go` `meta_wrong_type_not_suppressed` | Source changed to config-less |
| Wrong-type meta takes the fallback (with source config) | Done | `otc_test.go` `TestOTCEgressMalformedMetaTakesConfigFallback` | Added because the subtest above cannot distinguish the two implementations |
| Meta precedence over config | Done | `otc_test.go` `TestOTCEgressMetaTakesPrecedenceOverConfig` | |
| Destination-side complement | Done | `otc_test.go` | |
| Ingress-side complement | Done | `otc_test.go` | |
| Source-independent stamping | Done | `otc_test.go` | Untagged; see AC-8 |
| Config keying (5 tests) | Done | `config_keying_test.go,118,142,157,189` | |
| Stale-role clearing (6 tests) | Done | `session_role_test.go,91,125,151,179,206` | |
| Drop counters (4 tests) | Done | `metrics_test.go,255,302,346` | |
| **Withdraw-only payload is not stamped** | **MISSING** | - | AC-11. No unit test, no `.ci`, no interop scenario |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/role/otc.go` | Done | `resolveSrcRole` and `resolvePeerRole` added; three readers rewired; the `srcCfg == nil` early return deleted; five `recordDrop` call sites |
| `internal/component/bgp/plugins/role/otc_test.go` | Done | Six new tests, two fixtures adjusted under RFC-test approval |
| `internal/component/bgp/plugins/role/config.go` | Changed | Not in the original plan. Unreachable-key rejection (AC-9) |
| `internal/component/bgp/plugins/role/role.go` | Changed | Not in the original plan. OPEN-time clear of capability-learned roles (AC-10) |
| `internal/component/bgp/plugins/role/metrics.go` + `metrics_test.go` | Changed | Not in the original plan. New files: the drop counters (AC-10) |
| `internal/component/bgp/plugins/role/config_keying_test.go`, `session_role_test.go` | Changed | Not in the original plan. New test files for AC-9 and AC-10 |
| `internal/component/bgp/plugins/role/register.go` | Changed | Not in the original plan. Metric registration wiring |
| `internal/component/bgp/reactor/reactor_api_relay.go` | Changed | Not in the original plan. Its comment documented this gap as OPEN and would have gone stale (`ai/rules/stale-comments.md`) |
| `ai/RFC-REQUIREMENTS.md`, `ai/CODE-TO-DOCS.md` | Changed | Not in the original plan. Regenerated: a new or moved RFC-tagged test shifts the ledger and `ze-rfc-check` fails on a stale one |
| `docs/architecture/meta/role.md`, `docs/plugin-development/metrics.md` | Changed | Not in the original plan; required by `ai/rules/writing.md` once metadata semantics and counters changed |
| `docs/guide/bgp-role.md`, `docs/architecture/route-selection.md` | **NOT updated** | Both carry source anchors into the changed code and describe the changed behaviour. A real omission, found 2026-07-27 |

### Audit Summary
- **Total items:** 44 (9 Task requirements, 11 ACs, 12 TDD-plan tests, 12 file rows)
- **Done:** 41
- **Partial:** 0
- **Skipped:** 0
- **NOT MET:** 3 -- the "advertised only" requirement, AC-11, and its missing test. These
  are one defect counted at its three audit rows, not three separate gaps.
- **Changed:** 10 (the exactness qualification, plus nine file rows touched beyond the
  plan). None reduces scope; all record scope that GREW.
- **Documentation gap:** 2 doc files that carry anchors into the changed code were not
  revisited, and one `rfc-status.md` claim is known-false pending the AC-11 fix.

**Audit verdict: this spec CANNOT be closed.** Ten of eleven ACs are met with code and
tests, but AC-11 is an unfixed, reproducible, wire-visible defect in the exact function
this spec exists to correct, and `ai/rules/completion.md` makes "done with the
following outstanding" a contradiction rather than a status.

## Goal Validation (BLOCKING)

<!-- MANDATORY: Maps each stated goal to concrete proof it was achieved. -->
<!-- "Tests pass" is not sufficient. Each goal needs specific evidence. -->
<!-- See ai/rules/interop-and-goal-validation.md for required evidence types. -->
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| A caller without ingress metadata no longer skips the RFC 9234 Section 5 leak guard | Unit test, mutation-verified | `TestOTCEgressSuppressProviderLearnedWithoutMeta` passes with the fallback and FAILS without it (`go test -race -run TestOTCEgressSuppressProviderLearnedWithoutMeta ./internal/component/bgp/plugins/role/` exit 1 with the fallback disabled, exit 0 restored) |
| The recovered value is the config-derived one, not a guess | Code reading, not a test | `OTCIngressFilter` writes `meta["src-role"] = cfg.role` from `getFilterConfig(src.Address.String())`; `OTCEgressFilter` reads `srcCfg, _ := getFilterConfig(src.Address.String())` and the fallback returns `srcCfg.role`. Same map, same key, same field |
| No false suppression is introduced | Full-suite regression | `make ze-test-bgp` exit 0 (includes every `TestOTCEgress*` and the reactor forward/relay tests). Independently reviewed for wrongly-suppressing call sites -- see Review Gate |
| The RFC 9234-5-4 obligation stays PROVEN, not just green | RFC ledger + non-vacuity argument | `ai/RFC-REQUIREMENTS.md` still lists `RFC9234-5-4` with `otc_test.go` as its negative proof, and the fixture was changed rather than the assertion precisely so the test still reaches the stamping block; flipping the assertion would have made the tag hold for the wrong reason |
| The relay path specifically is covered | Code reading + comment | `reactor_api_relay.go` builds the replayed update with `Meta: nil`, which is exactly the shape the new test drives; its comment now records the gap as closed and names the proving test |
| RFC 9234 Section 5 gates are not skipped for a peer that sent no Role capability | Unit tests at all three readers | `TestOTCIngressStampsWhenPeerSentNoRoleCapability` (`otc_test.go`) asserts the ingress stamp carries the remote AS `65060`; `TestOTCEgressSuppressToProviderWithoutRoleCapability` (`otc_test.go`) asserts the egress suppression. Both re-run 2026-07-27: PASS. Non-vacuity: each drives a fixture with NO `setFilterRemoteRole` call, so the capability-only code path returns `""` and the test can only pass through the config complement |
| The Section 5 stamping MUST is not gated on the source | Unit test over a source with no config at all | `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` (`otc_test.go`): `mods.Len() == 1` with source `10.0.0.70` absent from `setFilterState`. Re-run 2026-07-27: PASS. **Evidence caveat, stated rather than glossed:** this test carries no `RFC requirement:` tag, so the ledger's `RFC9234-5-4 positive` claim still rests on `TestOTCEgressStampMod` (`otc_test.go`), whose source IS configured. The behaviour is proven; the LEDGER's proof of it is not the test that discriminates |
| A `role` block an operator writes actually takes effect, or says why not | Unit tests on both edges | `TestRoleConfigWithoutUsableRemoteIPIsRejected` (`config_keying_test.go`) and `TestRoleConfigNamedByAddressWithoutConnectionBlockIsKept`. Re-run 2026-07-27: PASS |
| A role learned from a capability does not outlive its session | Unit tests over a reconnect | six tests in `session_role_test.go`, including `TestStaleRoleClearedEvenWhenOpenIsRejected`. Re-run 2026-07-27: PASS |
| No suppression is invisible | Unit tests, including the discriminating negative | `TestRoleDropsAreCounted` (`metrics_test.go`) plus `TestRoleAcceptedRouteIsNotCounted`, which is what stops a counter that increments on every route from reading as proof. Re-run 2026-07-27: PASS |
| **OTC is applied only to routes being advertised** | **NONE** | **This goal is NOT achieved.** No test, no `.ci`, no interop scenario asserts it, and the producing code has no gate. `ai/rules/interop-and-goal-validation.md` forbids marking a goal row "N/A" or "blocked" to avoid the work; the honest entry is that the goal fails. The interop scenario for it would be genuinely non-vacuous, which is rare for this spec: reverting the fix makes a conforming receiver session-reset per RFC 7606 Section 5.2, so the peer's observable behaviour changes |

**Goal-validation verdict: incomplete.** Every goal the Task section states is achieved
and evidenced. The goals the spec ACQUIRED during implementation are achieved except the
last, and the last is the one with wire-visible, interop-fatal consequences.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->
<!-- Hook pre-commit-spec-audit.sh (exit 2) checks this section exists and is filled. -->

Re-verified 2026-07-27 by running the commands below, not by re-reading the audit. All
`go test` runs use the full default-on feature tags
(`ze_core` + every `ze_*` in `feature-gates.txt`), per `ai/rules/commands.md`: a bare
`go test` drops the tags and fabricates reds. Line numbers below are from the CURRENT
tree; the earlier version of this section cited pre-`f5dd2f040` numbers (`resolveSrcRole`
at `:392`, called at `:435`) which no longer resolve.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/role/otc.go` | Yes | `-rw-r--r-- 1 thomas staff 25K Jul 27 15:46` |
| `internal/component/bgp/plugins/role/otc_test.go` | Yes | `-rw-r--r-- 1 thomas staff 75K Jul 27 15:50` |
| `internal/component/bgp/plugins/role/config.go` | Yes | `-rw-r--r-- 1 thomas staff 9.7K Jul 27 15:52` |
| `internal/component/bgp/plugins/role/role.go` | Yes | `-rw-r--r-- 1 thomas staff 9.6K Jul 27 15:49` |
| `internal/component/bgp/plugins/role/metrics.go` | Yes | `-rw-r--r-- 1 thomas staff 6.8K Jul 27 15:53` (new file, `f5dd2f040`) |
| `internal/component/bgp/plugins/role/metrics_test.go` | Yes | `-rw-r--r-- 1 thomas staff 14K Jul 27 15:43` (new file) |
| `internal/component/bgp/plugins/role/config_keying_test.go` | Yes | `-rw-r--r-- 1 thomas staff 8.8K Jul 27 15:53` (new file) |
| `internal/component/bgp/plugins/role/session_role_test.go` | Yes | `-rw-r--r-- 1 thomas staff 9.8K Jul 27 15:48` (new file) |
| `test/plugin/role-otc-{egress-filter,egress-stamp,export-unknown,ingress-reject,unicast-scope}.ci` | Yes | all five present, 4.4K-6.4K, dated 2026-07-14 to 2026-07-25 |
| the AC-11 `.ci` and interop scenario | **No** | Not created. They are the missing proof for the open blocker, not an omission from a completed set |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Suppression fires with `meta == nil` | `grep -n 'resolveSrcRole' otc.go` -> defined at `:404`, CALLED at `:518` inside the `destRemoteRole` branch (not defined-only). `go test -race -run TestOTCEgressSuppressProviderLearnedWithoutMeta` -> `--- PASS` |
| AC-2 | Explicit meta still wins | `resolveSrcRole` returns the meta string before consulting `srcCfg` at `:417`. `go test -race -run TestOTCEgressMetaTakesPrecedenceOverConfig` -> `--- PASS` |
| AC-3 | Non-string meta takes the fallback | The `ok && role != ""` guard at `otc.go` fails for a non-string, so control falls to the `srcCfg` branch at `:417`, and `:414` WARNs on the way. `go test -race -run TestOTCEgressMalformedMetaTakesConfigFallback` -> `--- PASS` |
| AC-4 | No config -> no suppression | `resolveSrcRole` returns `""` at `otc.go` when `srcCfg == nil`; `""` matches none of `roleCustomer`/`rolePeer`/`roleRSClient` at `:519`. Covered by the `meta_wrong_type_not_suppressed` subtest |
| AC-5 | Legit transit still accepted and unstamped | `go test -race -run TestOTCEgressNoStampProvider` -> `--- PASS`. Assertion re-read, not inferred: `assert.Equal(t, 0, mods.Len(), "no mod should be written for Provider destination")` at `otc_test.go` |
| AC-6 | Destination role resolves from config when no capability | `grep -n 'resolvePeerRole' otc.go` -> defined `:459`, called `:327` and `:498`. `go test -race -run TestOTCEgressSuppressToProviderWithoutRoleCapability` -> `--- PASS` |
| AC-7 | Ingress MUSTs run for a capability-less peer | `go test -race -run TestOTCIngressStampsWhenPeerSentNoRoleCapability` -> `--- PASS`; assertion is `assert.Equal(t, uint32(65060), asn)`, i.e. the REMOTE AS, not merely "a stamp happened" |
| AC-8 | Stamping is destination-conditioned only | `go test -race -run TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` -> `--- PASS`. Read `otc.go` to confirm no `srcCfg == nil` early return remains between the export check and the stamping block: the only `srcCfg` test left is `if srcCfg != nil && len(srcCfg.resolvedExport) > 0` at `:538`, which guards the EXPORT match and falls through |
| AC-9 | Unreachable role config rejected | `go test -race -run 'TestRoleConfig|TestRoleCapabilityNotDeclared|TestUnusableRoleConfig'` -> `ok ... 1.413s` |
| AC-10 | Stale roles cleared; drops counted | `go test -race -run 'TestReconnect|TestClearedRole|TestStaleRole|TestRoleAccepted|TestRoleFirstDrop|TestRoleMetrics'` -> `ok ... 1.413s`. `grep -n 'recordDrop' otc.go` -> five call sites (`:348,353,505,520,551`), matching the five `return false` / reject paths |
| **AC-11** | **no OTC on a withdraw-only payload** | **FAILS.** `grep -rn -i 'unreach' internal/component/bgp/plugins/role/*.go` returns no code hit (only the `treat-as-withdraw` comments in `otc.go`). `otc.go` reads `if mods != nil && (destRemoteRole == roleCustomer || ...)` with no advertisement test. `otc.go` returns `true` for any payload with no MP_REACH, which includes every MP_UNREACH-only payload |
| Whole package | no regression | `go test -race -count=1 ./internal/component/bgp/plugins/role/` -> `ok github.com/ze-software/ze/internal/component/bgp/plugins/role 5.110s` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `RelayStoredRoute` -> `OTCEgressFilter` with `Meta: nil` | none | NO `.ci` for this path, stated rather than implied. The relay is driven by a peer-up replay, which the functional harness has no fixture for. Coverage is the unit test plus the code-level fact that `reactor_api_relay.go` sets `Meta: nil`. `ai/rules/testing.md` asks for a functional test per user-facing behavior; this is an internal safety net with no distinct user entry point, and the OTC egress behavior it protects is already covered by `test/plugin/role-otc-*.ci` |
| Ingress/egress role gates for a capability-less peer | `test/plugin/role-otc-export-unknown.ci` | Partially. That `.ci` exists to prove `export unknown` still targets capability-less peers after the `resolvePeerRole` widening -- it is the regression guard for R-4, not a positive proof of the widening itself. The widening's proof is the two unit tests. Stated rather than claimed as end-to-end |
| The egress stamp reaching the wire | `test/plugin/role-otc-egress-stamp.ci` | Yes -- byte-level: it asserts the OTC attribute in the UPDATE a Customer peer receives, which is the only thing that proves `mods.Op` -> `buildModifiedPayload` -> `otcAttrModHandler` actually emits bytes. The unit tests stop at `mods.Len()` |
| The unicast family gate | `test/plugin/role-otc-unicast-scope.ci` | Yes for the MP_REACH branch. **No for the withdrawal shape** -- which is the AC-11 gap, and is why the `docs/features/rfc-status.md` "unicast-only scoping" claim is not fully backed |
| **A withdraw-only UPDATE toward a Customer** | **none** | **MISSING.** AC-11 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 "the value is recoverable EXACTLY" (Task) | **Confirmed, with a caveat** | Same config field on both sides: `OTCIngressFilter` writes `cfg.role` into `meta["src-role"]` at `otc.go` from the lookup at `:312`; `OTCEgressFilter` performs the same lookup at `:491` and `resolveSrcRole` returns `srcCfg.role` at `:418`. Caveat recorded in code (`otc.go`) and in the audit: they agree at one instant, not across a config reload |
| A-2 "`RelayStoredRoute` is not the only caller that can lack meta" | **Confirmed by events** | Three further readers of the same shape were found in different call paths by review rounds 1-3, which is why the fix is at the read |
| A-3 "landing it requires changing an RFC-tagged test" (Task) | **Confirmed** | The pre-write hook blocked the edit until `// rfc-test-change-approved:` was present; Thomas approved 2026-07-27. Marker at `otc_test.go` |
| A-4 "an empty `filterRemoteRoles` value means the peer advertised no role" | **Broken** | It also meant "never recorded" and "recorded by a previous session and never cleared". The second half is fixed (`role.go`, cleared at the OPEN, proven by `session_role_test.go`); the first half is a live deferral homed on `plan/spec-fixit-stored-route-relay-hardening.md` |
| A-5 "`role` config reaches the filters however the peer is named" | **Broken** | `grep -n 'getFilterConfig' otc.go` -> all three readers key by `Address.String()`, so a name-keyed entry was unreachable. Fixed by rejecting unreachable keys; proven by `config_keying_test.go` |

No assumption is left `unvalidated`.

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/meta/role.md` reflects the changed `src-role` semantics and the new resolution helpers | `grep -rn 'source:.*plugins/role/' docs/` -> anchors at `meta/role.md,27,49,59,60,70,82,96`, covering `resolvePeerRole`/`peerRoleComplement`, `resolveExport`, `extractPeerRoleConfigs`, `applyValidateOpen`/`clearFilterRemoteRole`/`filterKeyLocked`, and `recordDrop`/`buildMetrics` -- i.e. every symbol the five commits added | Yes |
| `docs/plugin-development/metrics.md` documents the drop counters | anchors at `:341-342` to `role/metrics.go` `recordDrop` and to both filters; the prose above them explains why a filtering plugin needs metrics even when it holds no state | Yes |
| `ai/RFC-REQUIREMENTS.md` regenerated | `make ze-rfc-index` run in the implementing commits; the ledger gained `RFC9234-3.1-1` from `otc_test.go` and absorbed the `file:line` shifts. `grep -c 'RFC requirement:' otc_test.go` -> 21 tags | Yes |
| `reactor_api_relay.go` comment no longer stale | It previously described this gap as open; now records it closed and names the proving test | Yes |
| **`docs/guide/bgp-role.md` NOT updated** | It carries three anchors into the role plugin and describes when OTC is stamped and when routes are suppressed -- the exact behaviour five commits changed. No commit in this set touched it. Found by the grep above, not assumed | **No -- gap** |
| **`docs/architecture/route-selection.md` NOT revisited** | Anchors `otc.go` for "RFC 9234 OTC validation". Whether its claim is still accurate after the `resolvePeerRole` widening was not checked | **No -- gap** |
| **`docs/features/rfc-status.md` carries a claim now known to be false** | The row reads "No tracked gap in current source anchors" and credits "unicast-only (AFI 1/2, SAFI 1) OTC scoping". The OPEN BLOCKER shows the scoping does not hold for a withdraw-only payload | **No -- must be corrected WITH the AC-11 fix.** Correcting the doc first would be recording a defect instead of fixing it (`ai/rules/completion.md`) |
| `make ze-doc-test` | Not run: this spec-fill pass edited no file under `docs/`. It was run by the implementing commits that did | n/a for this pass |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
