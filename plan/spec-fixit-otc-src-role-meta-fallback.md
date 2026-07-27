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
| egress stamping block | `role/otc.go:562-584` | gated only on `mods != nil` and `destRemoteRole ∈ {customer, peer, rs-client}`. No advertisement gate of any kind |
| `isPayloadUnicast` | `role/otc.go:100-145` | scans for `mpReachAttrCode` (`= byte(14)`, `otc.go:93`) only, and its terminal `return true` at `:144` reads "no MP_REACH found, therefore IPv4 unicast" |
| `extractAttrsFromPayload` | `role/otc.go:210-225` | returns `payload[attrStart : attrStart+attrLen]`, a non-nil EMPTY slice when `attrLen == 0`, so no `attrs == nil` guard can fire on a pure withdrawal |
| MP_UNREACH awareness | (none) | `grep -rn -i 'unreach' internal/component/bgp/plugins/role/*.go` returns no code hit. Type 15 is not mentioned anywhere in the plugin |

So a pure IPv4 withdrawal (`00 04 18 0a 00 00 00 00`) passes `isPayloadUnicast`, reaches
`otc.go:563` with an empty attribute slice, `findOTC` reports `hasOTC == false`, and
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
`TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` (`otc_test.go:1120`) still
carries no `RFC requirement:` tag, so `ai/RFC-REQUIREMENTS.md` continues to credit
`RFC9234-5-4` to the weaker test; and `TestOTCEgressUnicastOnly` (`otc_test.go:1358`)
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
(`internal/component/bgp/reactor/forward_build.go:241-259`) writes unconsumed ops as new
attributes, and `otcAttrModHandler` emits the 7-byte attribute. Neither egress caller
(`reactor_api_forward.go:605`, `forward_rs.go:349`) has a withdrawal guard.

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
the MP_REACH branch, so the "unicast-only scoping" claim at `docs/features/rfc-status.md:25`
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

`internal/component/bgp/plugins/role/otc.go:396-403` reads `meta["src-role"]` and
treats a MISSING key as "no restriction". That is the zero-value trap in
`ai/rules/fail-closed-guards.md`: any caller without ingress metadata silently
skips an RFC 9234 Section 5 leak guard. The Adj-RIB-In relay path
(`RelayStoredRoute`) is one such caller today; it is not the only possible one.

The value is recoverable EXACTLY, not guessable: `OTCIngressFilter` (otc.go:311-317)
writes `meta["src-role"] = cfg.role` from `getFilterConfig(src.Address.String())`,
the same lookup `OTCEgressFilter` already performs into `srcCfg`. So the fix is a
config-derived fallback when meta lacks the key.

Split out of `plan/spec-fixit-bgp-egress-rail-divergence.md` (see
`plan/deferrals/fixit-bgp-egress-rail-divergence.md`) because that spec's ACs pass
without it -- OTC suppression on the replay path already works through the
wire-bytes rule -- and because landing it requires changing an RFC-tagged test.

**BLOCKING precondition:** `TestOTCEgressNoStampProvider`
(`internal/component/bgp/plugins/role/otc_test.go:995`) carries
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
- [ ] `ai/rules/fail-closed-guards.md` - the rule the original defect violates
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
- [ ] `internal/component/bgp/plugins/role/otc.go:485` - `OTCEgressFilter`: the unicast gate, the two `getFilterConfig` lookups, the wire-bytes suppression, the Gao-Rexford safety net, the export-set match, and the stamping block
  → Constraint: the stamping block (`:562-584`) has no advertisement gate. See OPEN BLOCKER.
- [ ] `internal/component/bgp/plugins/role/otc.go:311` - `OTCIngressFilter`: writes `meta["src-role"] = cfg.role` at `:316` from `getFilterConfig(src.Address.String())` at `:312`, the same lookup the egress filter performs into `srcCfg` at `:491`
- [ ] `internal/component/bgp/plugins/role/otc.go:404` - `resolveSrcRole`: meta first, config fallback, `""` when neither
- [ ] `internal/component/bgp/plugins/role/otc.go:459` - `resolvePeerRole`: capability first, then the config complement via `peerRoleComplement` (`:426-432`)
- [ ] `internal/component/bgp/plugins/role/config.go` - `getFilterConfig` keying and the unusable-key rejection added by `f5dd2f040`
- [ ] `internal/component/bgp/plugins/role/role.go` - `setFilterRemoteRole` (the only writer of `filterRemoteRoles`) and the OPEN-time clear added by `f5dd2f040`
- [ ] `internal/component/bgp/plugins/role/metrics.go` - `recordDrop` and the four drop reasons added by `f5dd2f040`
- [ ] `internal/component/bgp/reactor/reactor_api_relay.go` - `RelayStoredRoute`, which builds the replayed update with `Meta: nil`: the caller shape the fallback exists for

**Behavior to preserve:** (unless user explicitly said to change)
- `OTCEgressFilter` / `OTCIngressFilter` signatures: they are `filterapi.EgressFilterFunc` / `IngressFilterFunc` and are registered, not called directly.
- The wire-bytes egress rule (`checkOTCEgress`, `otc.go:201-206`) stays unconditional and independent of source config: it is the rule that already made suppression work on the replay path.
- A peer with NO role config at all is not filtered. `""` means "unconfigured", never "unrestricted".
- `roleUnknown` stays an operator-selected export target in the export-set match (`config.go:36`), NOT an unanswered question. Reclassifying it there would silently retarget an operator knob.
- The RFC 9234 Section 5 unicast scoping.

**Behavior to change:** (only if user explicitly requested)
- The five defects enumerated in the Implementation Summary. Nothing else. The stamping-onto-withdrawals defect in the OPEN BLOCKER is IN scope and NOT yet fixed.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Ingress: a received BGP UPDATE's payload bytes, handed to `OTCIngressFilter` by the reactor's ingress filter chain before caching and dispatch, together with the shared `ingressMeta` map.
- Egress: the same payload replayed per destination peer, handed to `OTCEgressFilter` by `safeEgressFilter` from `reactor_api_forward.go:655` (the forward rail) and `forward_rs.go:349` (the route-server rail), plus `reactor_api_batch.go:1253` for the readvertise rail. The relay rail (`reactor_api_relay.go`) reaches the same filter with `Meta: nil`.
- Format at entry: raw RFC 4271 UPDATE payload -- `withdrawnLen(2) + withdrawn + attrLen(2) + attrs + nlri` -- plus a `filterapi.PeerFilterInfo` for source and destination and a `*filterapi.ModAccumulator` for egress attribute edits.

### Transformation Path
1. Ingress: `getFilterConfig(src)` yields `(cfg, capRole)`; `cfg.role` is copied into `meta["src-role"]` (`otc.go:315-317`).
2. Ingress: `resolvePeerRole(capRole, cfg)` decides what the source peer IS to us -- capability first, config complement second (`otc.go:327`, `:459-476`).
3. Ingress: the unicast gate (`isPayloadUnicast`) and `extractAttrsFromPayload` run; `checkOTCIngress` applies the three Section 5 ingress MUSTs and may return a stamp ASN (`otc.go:344`).
4. Ingress: a leak or malformed OTC is counted through `recordDrop` and rejected / treated-as-withdraw; a stamp rewrites the payload via `insertOTCInPayload`, and that MODIFIED payload replaces the original for caching and dispatch.
5. Egress: the unicast gate runs first, then `resolvePeerRole` for the destination, then the unconditional wire-bytes rule (`checkOTCEgress`).
6. Egress: the Gao-Rexford safety net calls `resolveSrcRole(meta, srcCfg)` -- meta if usable, else `srcCfg.role`, else `""` -- and suppresses toward a Provider/Peer/RS destination.
7. Egress: the operator export-set match runs (capability-only role), then the stamping block queues `mods.Op(otcAttrCode, AttrModSet, asn)`.
8. Build: `buildModifiedPayload` (`reactor/forward_build.go`) walks the source attributes and, at step 6, writes unconsumed ops as NEW attributes; `otcAttrModHandler` (`otc.go:598`) emits the 7-byte attribute. This is the step that puts a queued stamp on the wire, and the step the OPEN BLOCKER's withdrawal defect rides.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ role plugin (ingress) | `filterapi.IngressFilterFunc`; the returned payload REPLACES the original for caching and dispatch | Yes -- `OTCIngressFilter` returns `(true, modified)` at `otc.go:368` |
| Reactor ↔ role plugin (egress) | `filterapi.EgressFilterFunc` returning a bare `bool` plus a `*ModAccumulator`; there is no failure channel, so "could not evaluate" is indistinguishable from "policy said no" | Yes -- signature at `filterapi/filterapi.go`; recorded as a live deferral on `plan/spec-fixit-stored-route-relay-hardening.md` |
| Ingress filter ↔ egress filter | the shared `ingressMeta` map, key `src-role`. One map is shared by every in-process ingress filter, so any filter can clobber the key | Yes -- `resolveSrcRole` logs a WARN on an unusable value rather than failing silently (`otc.go:414`) |
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
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->
<!-- Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis) land HERE, not just in conversation. -->

### Assumptions
<!-- Things believed true that the design depends on. Every row needs a validation method. -->
<!-- Status: unvalidated → confirmed | broken. A broken assumption also gets a Mistake Log "Wrong Assumptions" row. -->
<!-- No assumption may still be `unvalidated` at Pre-Commit Verification. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `meta["src-role"]` is recoverable EXACTLY from config, not guessed | `OTCIngressFilter` writes `meta["src-role"] = cfg.role` (`otc.go:315-317`) from `getFilterConfig(src.Address.String())` (`:312`); `OTCEgressFilter` performs the same lookup into `srcCfg` (`:491`) | the fallback would be a heuristic, and a wrong suppression is a routing outage | read both producers | **confirmed, with a caveat** -- same map, same key, same field, but they agree at one INSTANT only: meta is captured at RECEIVE, config is read at FORWARD. A config reload between them makes them differ. The relay-time role is the safer one to gate on; recorded in the code comment at `otc.go:383-390` |
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
| R-4 | Widening `resolvePeerRole` retargets the operator's `export unknown` knob | an operator's `role export unknown` set stops matching peers it used to match | deliberately scoped: `resolvePeerRole` feeds the RFC MUST gates only. The export-set match keeps the capability-only value, and the reason is recorded at `otc.go:453-458` |
| R-5 | A config reload between receive and forward makes meta and config disagree | none observable | accepted, not fixed: the relay-time role describes the relationship the route is being forwarded under, and is the safer of the two to gate a leak check on. Recorded at `otc.go:383-390` |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation — unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An egress filter invocation carrying no ingress metadata (the `RelayStoredRoute` shape, `reactor_api_relay.go` builds the replay with `Meta: nil`) | → | `resolveSrcRole` config fallback (`otc.go:404-421`), reached from the `destRemoteRole` branch at `otc.go:518` | `TestOTCEgressSuppressProviderLearnedWithoutMeta` (`otc_test.go:1081`) |
| A peer that sent no Role capability, on INGRESS | → | `resolvePeerRole` config complement (`otc.go:459-476`), reached from `otc.go:327` | `TestOTCIngressStampsWhenPeerSentNoRoleCapability` (`otc_test.go:1167`) |
| A peer that sent no Role capability, as the egress DESTINATION | → | `resolvePeerRole` from `otc.go:498` | `TestOTCEgressSuppressToProviderWithoutRoleCapability` (`otc_test.go:1270`) |
| A route from a source with no `role { import ... }` container, advertised to a Customer | → | the stamping block at `otc.go:562-584`, no longer gated on `srcCfg != nil` | `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` (`otc_test.go:1120`) |
| A `role` config block on a peer whose remote IP does not resolve | → | the unreachable-key rejection in `config.go` | `TestRoleConfigWithoutUsableRemoteIPIsRejected` (`config_keying_test.go:53`) |
| A peer that reconnects WITHOUT the Role capability it once advertised | → | the OPEN-time clear of `filterRemoteRoles` in `role.go` | `TestReconnectWithoutRoleCapabilityClearsStaleRole` (`session_role_test.go:52`) |
| Any suppression decision reaching an operator | → | `recordDrop` (`metrics.go`), called from all four suppression sites | `TestRoleDropsAreCounted` (`metrics_test.go:138`), `TestRoleFirstDropEmitsWarn` (`metrics_test.go:302`) |
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
`ai/rules/no-partial-completion.md` forbids. The rows below state what those commits
actually assert, taken from the landed tests, not from the commit messages. The scope
growth itself is recorded in Deviations.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-6 | The egress DESTINATION peer sent no Role capability but has `role { import ... }` config | The Section 5 destination gates resolve from the config complement instead of reading the empty capability as "no role". A Provider-learned route toward that destination is still suppressed |
| AC-7 | The INGRESS source peer sent no Role capability but has `role { import ... }` config | All three Section 5 ingress MUSTs still run (leak from a Customer/RS-Client, the Peer ASN mismatch, and the stamp). A route without OTC from a configured Provider is stamped with the REMOTE peer's AS |
| AC-8 | A route is advertised to a Customer/Peer/RS-Client from a source with NO role config at all (iBGP, an RR client, a locally originated or API-injected route) | OTC is stamped with the local AS. RFC 9234 Section 5 conditions this rule on the DESTINATION only; no source-side gate may suppress it |
| AC-9 | A `role` block is configured on a peer whose remote IP does not resolve, so the entry would be keyed by name while every reader keys by address | The config is REJECTED with a WARN naming the peer, rather than silently stored and never read. A peer NAMED for its address still works |
| AC-10 | A peer that once advertised a Role capability reconnects without one; and separately, any suppression decision fires | The stale capability-learned role is CLEARED at the OPEN (not on session down, which carries no session identity); and every suppression is counted under a distinguishable reason with a one-shot WARN on the first drop, so a peer's advertisements can never be withdrawn invisibly |

**AC-11 is OPEN and is the closure blocker.** Stated here so the AC table matches the
spec's own verdict rather than contradicting it:

| AC ID | Input / Condition | Expected Behavior | Status |
|-------|-------------------|-------------------|--------|
| AC-11 | `OTCEgressFilter` reached with a payload that advertises NO reachable NLRI: a pure IPv4 withdrawal, or an MP_UNREACH-only payload of any family, toward a Customer/Peer/RS-Client destination | NO OTC attribute is queued. RFC 9234 Section 5 egress rule 1 applies to a route that "is to be advertised"; a withdrawal is not a route. `isPayloadUnicast` must also recognise MP_UNREACH (type 15) so the family scoping holds for the withdrawal shape | **NOT MET** -- see OPEN BLOCKER. No advertisement gate exists at `otc.go:562`, and the plugin has no MP_UNREACH awareness |

## End-to-End User Stories (MANDATORY for new features)

<!-- For each user-facing operation the feature enables, trace the full path.
     This section catches missing code that narrow ACs miss. ACs verify individual
     components work; user stories verify the full chain is connected.
     Every story must have a corresponding functional or wiring test. -->

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures `role import customer` on a Provider peer and expects that peer's routes never to reach another Provider, including after a session flap replays them from the Adj-RIB-In | peer-up -> `RelayStoredRoute` (`Meta: nil`) -> egress filter chain -> `OTCEgressFilter` -> `resolveSrcRole` config fallback -> suppress | `TestOTCEgressSuppressProviderLearnedWithoutMeta` (`otc_test.go:1081`); the OTC egress behaviour itself is covered end-to-end by `test/plugin/role-otc-egress-filter.ci` |
| 2 | peers with an early-adopter neighbour that does not send the RFC 9234 Role capability, and expects role policy to work from local config | OPEN with no Role capability -> `resolvePeerRole` config complement -> Section 5 gates on both ingress and egress | `TestOTCIngressStampsWhenPeerSentNoRoleCapability` (`otc_test.go:1167`), `TestOTCEgressSuppressToProviderWithoutRoleCapability` (`otc_test.go:1270`) |
| 3 | advertises an iBGP-learned or locally originated route to a Customer and expects it to carry OTC so a downstream leak is catchable | forward rail -> `OTCEgressFilter` stamping block -> `mods.Op` -> `buildModifiedPayload` step 6 -> `otcAttrModHandler` -> wire | `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` (`otc_test.go:1120`); wire-level in `test/plugin/role-otc-egress-stamp.ci` |
| 4 | mistypes a peer name in a `role` block and expects to be told, not to get silent no-op policy | config load -> `parseRoleContainer` -> keying check -> WARN + reject | `TestRoleConfigWithoutUsableRemoteIPIsRejected` (`config_keying_test.go:53`), `TestUnusableRoleConfigDoesNotShadowUsablePeers` (`config_keying_test.go:189`) |
| 5 | asks why a peer stopped receiving routes | any suppression -> `recordDrop` -> `ze_bgp_role_drops_total` by reason + one-shot WARN | `TestRoleDropsAreCounted` (`metrics_test.go:138`), `TestRoleFirstDropEmitsWarn` (`metrics_test.go:302`) |
| 6 | withdraws a prefix toward a Customer and expects a plain withdraw-only UPDATE on the wire | forward rail -> `OTCEgressFilter` -> **no advertisement gate** -> `mods.Op` -> a 7-byte OTC attribute on a withdraw-only UPDATE | **NONE. This path is BROKEN** -- see AC-11 and the OPEN BLOCKER |

<!-- If a path has a broken link (no implementation at some step), that is a spec gap.
     Add the missing component to ACs, Files to Create, and TDD Test Plan before proceeding. -->

## 🧪 TDD Test Plan

All paths below are `internal/component/bgp/plugins/role/`.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOTCEgressSuppressProviderLearnedWithoutMeta` | `otc_test.go:1081` | AC-1 -- the config fallback fires with `meta == nil`. Carries `RFC requirement: RFC9234-3.1-1 positive` | Done |
| `TestOTCEgressMetaTakesPrecedenceOverConfig` | `otc_test.go:1232` | AC-2 -- a usable meta value wins over config | Done |
| `TestOTCEgressMalformedMetaTakesConfigFallback` | `otc_test.go:1201` | AC-3 -- a PRESENT but non-string value takes the fallback, over a source that HAS config so the branch is distinguishable | Done |
| `TestOTCEgressFilter/meta_wrong_type_not_suppressed` | `otc_test.go:633` | AC-3/AC-4 -- a non-string value over a source with NO config yields no suppression | Done |
| `TestOTCEgressNoStampProvider` | `otc_test.go:1036` | AC-5 -- legitimate transit to a Provider is accepted and unstamped. Retains `RFC requirement: RFC9234-5-4 negative`; fixture changed under `// rfc-test-change-approved:` | Done |
| `TestOTCEgressSuppressToProviderWithoutRoleCapability` | `otc_test.go:1270` | AC-6 -- destination-side config complement | Done |
| `TestOTCIngressStampsWhenPeerSentNoRoleCapability` | `otc_test.go:1167` | AC-7 -- ingress-side config complement, asserting the stamp carries the REMOTE AS | Done |
| `TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig` | `otc_test.go:1120` | AC-8 -- the stamp is destination-conditioned only | Done (untagged -- see OPEN BLOCKER) |
| `TestRoleConfigWithoutUsableRemoteIPIsRejected`, `TestRoleConfigNamedByAddressWithoutConnectionBlockIsKept`, `TestRoleCapabilityNotDeclaredForUnusablePeer`, `TestRoleConfigWithUsableRemoteIPStillKeyedByAddress`, `TestUnusableRoleConfigDoesNotShadowUsablePeers` | `config_keying_test.go:53,118,142,157,189` | AC-9 -- both edges of the keying rejection | Done |
| `TestReconnectWithoutRoleCapabilityClearsStaleRole`, `TestReconnectWithNewRoleCapabilityWinsOverPrevious`, `TestReconnectWithUnassignedRoleValueClearsStaleRole`, `TestClearedRoleIsKeyedLikeTheSetter`, `TestStaleRoleClearedEvenWhenOpenIsRejected`, `TestReconnectWithoutRoleIsObservable` | `session_role_test.go:52,91,125,151,179,206` | AC-10 (clearing half) | Done |
| `TestRoleDropsAreCounted`, `TestRoleAcceptedRouteIsNotCounted`, `TestRoleFirstDropEmitsWarn`, `TestRoleMetricsSafeBeforeConfigure` | `metrics_test.go:138,255,302,346` | AC-10 (observability half) | Done |
| (none) | - | **AC-11 -- no test asserts that a withdraw-only payload is NOT stamped** | **MISSING** |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| OTC attribute value (4-byte ASN) | 0..4294967295 | 4294967295 | N/A (unsigned) | N/A (`uint32` by type) -- covered by `TestOTCBoundaryASN` (`otc_test.go:732`) |
| OTC attribute length | MUST be exactly 4 (RFC 9234 Section 5) | 4 | 3 -> treat-as-withdraw | 5 -> treat-as-withdraw -- covered by `TestOTCBoundaryLength` (`otc_test.go:744`) |
| rewritten `attrLen` after an ingress stamp | 0..65535 | 65535 | N/A | overflow returns nil and the route is accepted unmodified (`insertOTCInPayload`, `otc.go:249-251`) |

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
  unfixed defect that blocks closure (`ai/rules/no-parking.md`: a reproducible defect has
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
| Prometheus counters/metrics | **Yes -- done** | `internal/component/bgp/plugins/role/metrics.go`. Two counters: `ze_role_route_rejects_total` (ingress, route made ineligible) and `ze_role_route_suppressions_total` (egress, advertisement withheld), each labeled `reason`. Five bounded reason values -- `leak`, `malformed-otc`, `otc-present`, `source-role`, `export-set` -- so cardinality is five series per metric and never per-peer; peer identity goes in the log line. Every child is pre-resolved at build time (`metrics.go:88-98`) because `CounterVec.With` allocates a `[]string` per call and this is the forward path (`ai/rules/no-sprintf-alloc.md`), and pre-creating each child means the series exists at 0 from startup so a rate alert does not wait for it to appear. Documented at `docs/plugin-development/metrics.md:341-342` |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- Every No MUST be backed by a source-aware check, not a guess. At minimum, grep docs for source anchors pointing at changed files. -->
<!-- Any factual doc change MUST include or update a source-anchor HTML comment after the claim. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] | `rfc/short/rfcNNNN.md` (summary) and `docs/features/rfc-status.md` (status ledger row with source anchors) |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | [ ] | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | [ ] | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md`, relevant guide |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Grep `docs/` for `source: <changed-file>` and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | Verify examples against YANG/parser/handler and update stale syntax |

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

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

1. **Phase: Wiring (MANDATORY FIRST)** — register entry points, write failing wiring tests
   - Tests: [wiring test names from Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: entry point exists and is reachable; wiring test fails because feature logic is a stub
2. **Phase: [name]** — [what to implement]
   - Tests: [test names from TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses
3. **Phase: [name]** — [what to implement]
   - Tests: [test names from TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test passes
4. **Functional tests** → Create after feature works. Cover user-visible behavior.
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
| Correctness | [feature-specific: e.g., "merge order correct", "error messages accurate"] |
| Naming | [feature-specific: e.g., "JSON keys use kebab-case", "YANG uses kebab-case"] |
| Data flow | [feature-specific: e.g., "resolution in X only, reactor unaware of Y"] |
| CLI grammar | If CLI commands added: action before identifier per `ai/rules/cli-grammar.md` |
| Registration over hardcoding | New command/view/family/handler is registry-registered and core-discovered; no new per-feature field, switch case, or factory added to a core/shared struct (incl. the CLI `Model`). See `ai/rules/plugin-self-containment.md` |
| Doctor checks | If runtime dependencies added: `ze doctor` check registered per `ai/rules/doctor-checks.md` |
| YANG validation | If YANG leaves added: every leaf has max native constraints (`range`/`length`/`pattern`/`enum`). Bare `type string` is a red flag. Custom validator + `CompleteFn` where native is insufficient |
| Prometheus counters | If observable state exists: counters defined, registered, metric names listed |
| Rule: no-layering | [if replacing something: "old code fully deleted"] |
| Rule: [other relevant rule] | [what to check] |

### Deliverables Checklist (/implement stage 10)

<!-- MANDATORY: Every deliverable with a concrete verification method.
     /implement re-reads the spec and checks each item independently. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command to verify] |

### Security Review Checklist (/implement stage 11)

<!-- MANDATORY: Feature-specific security concerns. /implement checks each item.
     Think about: untrusted input, injection, resource exhaustion, error leakage. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |
| [other concern] | [what to check] |

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

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem → arch doc, process → rules, knowledge → memory.md -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->
<!-- Not all specs have one. Delete this section if nothing qualifies. -->
<!-- Source for learned summary Decisions section (METHODOLOGY.md extraction step 2). -->

## Key Design Decisions
<!-- Record each significant design choice as it is made. -->
<!-- Format: "Chose X over Y because Z." Include rejected alternatives. -->
<!-- Source for learned summary Decisions section (METHODOLOGY.md extraction step 2). -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries and constraints accepted. -->
<!-- Source for learned summary Consequences section (METHODOLOGY.md extraction step 3). -->
- [What was deliberately not done and why]

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered — add test for each]

### Documentation Updates
- [Docs updated, with source anchors named, or "None" with grep evidence]
- [If docs were changed: `make ze-doc-test` result]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Close the missing-metadata gap in `OTCEgressFilter`'s Gao-Rexford safety net | Done | `role/otc.go` `resolveSrcRole`, called from the `destRemoteRole` branch of `OTCEgressFilter` | Fixed at the READ, not at the `RelayStoredRoute` caller, because the gap fires for ANY caller lacking meta |
| Recover the value EXACTLY from config rather than guessing | Done | `resolveSrcRole` returns `srcCfg.role`, the field `OTCIngressFilter` copies into `meta["src-role"]` | Changed: qualified in the code comment. The two agree only at one INSTANT (meta captured at receive, config read at forward), so a config reload between them makes them differ; the relay-time role is the safer one to gate on |
| Requires changing an RFC-tagged test; ask before writing code | Done | Thomas approved 2026-07-27; `// rfc-test-change-approved:` marker on `TestOTCEgressNoStampProvider` | Approval obtained BEFORE the change, per `ai/rules/testing.md` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestOTCEgressSuppressProviderLearnedWithoutMeta` | Mutation-verified: disabling the fallback turns it red |
| AC-2 | Done | `TestOTCEgressFilter/src_role_*` subtests | Unchanged by this work; they pass meta explicitly from a config-less source, so meta is the only signal |
| AC-3 | Done | `TestOTCEgressFilter/meta_wrong_type_not_suppressed` | Moved to a config-less source so it isolates the type check |
| AC-4 | Done | Same subtest (10.0.0.99 has no `peerRoleConfig`) | `resolveSrcRole` returns `""` when `srcCfg == nil` |
| AC-5 | Done | `TestOTCEgressNoStampProvider` | Keeps its `RFC9234-5-4 negative` tag; fixture changed so the route still reaches the stamp block |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Suppression with no meta | Done | `otc_test.go` `TestOTCEgressSuppressProviderLearnedWithoutMeta` | New test |
| Stamping scope preserved | Done | `otc_test.go` `TestOTCEgressNoStampProvider` | Fixture changed, RFC tag retained |
| Wrong-type meta ignored | Done | `otc_test.go` `meta_wrong_type_not_suppressed` | Source changed to config-less |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/role/otc.go` | Done | `resolveSrcRole` added; egress branch calls it |
| `internal/component/bgp/plugins/role/otc_test.go` | Done | One new test, two fixtures adjusted |
| `internal/component/bgp/reactor/reactor_api_relay.go` | Changed | Not in the original plan. Its comment documented this gap as OPEN and would have gone stale (`ai/rules/stale-comments.md`) |
| `ai/RFC-REQUIREMENTS.md` | Changed | Not in the original plan. Regenerated because a new RFC-tagged test shifts the ledger and `ze-rfc-check` fails on a stale one |

### Audit Summary
- **Total items:** 15 (3 requirements, 5 ACs, 3 tests, 4 files)
- **Done:** 15
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (the exactness qualification, plus two files touched beyond the plan) -- all recorded above; none reduces scope

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

Re-verified 2026-07-27 by running the commands below, not by re-reading the audit.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/role/otc.go` | Yes | `ls -la` -> 19K |
| `internal/component/bgp/plugins/role/otc_test.go` | Yes | `ls -la` -> 65K |
| (no "Files to Create") | n/a | This change adds no new file; it modifies two existing ones |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Suppression fires with `meta == nil` | `grep -n resolveSrcRole otc.go` -> defined :392, CALLED at :435 inside the `destRemoteRole` branch. `go test -race -run TestOTCEgressSuppressProviderLearnedWithoutMeta` exit 0; same run with the fallback disabled exit 1 |
| AC-2 | Explicit meta still wins | `resolveSrcRole` (:392) returns the meta string before consulting `srcCfg`; `make ze-test-bgp` exit 0 with the pre-existing `src_role_*` subtests unchanged |
| AC-3 | Non-string meta takes the fallback | The `ok && role != ""` guard at :393 fails for a non-string, so control reaches the `srcCfg` branch. `meta_wrong_type_not_suppressed` passes |
| AC-4 | No config -> no suppression | `resolveSrcRole` returns `""` when `srcCfg == nil`; `""` matches none of `roleCustomer/rolePeer/roleRSClient` at the call site |
| AC-5 | Legit transit still accepted and unstamped | `grep -c 'func TestOTCEgressNoStampProvider'` -> present; passes in `make ze-test-bgp` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `RelayStoredRoute` -> `OTCEgressFilter` with `Meta: nil` | none | NO `.ci` for this path, stated rather than implied. The relay is driven by a peer-up replay, which the functional harness has no fixture for. Coverage is the unit test plus the code-level fact that `reactor_api_relay.go` sets `Meta: nil`. `ai/rules/functional-test-gate.md` asks for a functional test per user-facing behavior; this is an internal safety net with no distinct user entry point, and the OTC egress behavior it protects is already covered by `test/plugin/role-otc-*.ci` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| "the value is recoverable EXACTLY" (Task) | Confirmed, with a caveat | Same config field on both sides (verified by reading `OTCIngressFilter` and the egress `getFilterConfig` call). Caveat recorded in code and in the audit: they agree at one instant, not across a config reload |
| "landing it requires changing an RFC-tagged test" (Task) | Confirmed | The pre-write hook blocked the edit until `// rfc-test-change-approved:` was present; Thomas approved 2026-07-27 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user-facing doc change | The fix restores an RFC 9234 guarantee already documented as implemented; it changes no config surface, CLI output or JSON shape. `git show c398e97f0 --stat` touches only `role/`, `reactor_api_relay.go` (comment), the RFC ledger and a deferral shard | Yes |
| `ai/RFC-REQUIREMENTS.md` regenerated | `make ze-rfc-index`; diff shows `RFC9234-3.1-1` gaining the new test and line shifts confined to `otc_test.go` | Yes |
| `reactor_api_relay.go` comment no longer stale | It previously described this gap as open; now records it closed and names the proving test | Yes |

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
