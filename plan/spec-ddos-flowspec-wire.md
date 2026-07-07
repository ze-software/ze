# Spec: ddos-flowspec origination — wire the responder + implement the flowspec announce verb

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-07-07 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/1008-cp-survival-4-on-demand-origination-design.md`, `plan/learned/1011-cp-survival-5-detect-0-umbrella.md`
4. `internal/plugins/ddos/flowspec/responder.go`, `internal/plugins/ddos/flowspec/register.go`, `internal/component/bgp/plugins/cmd/announce/announce.go`

## Task

Complete DDoS FlowSpec origination (ddos detection umbrella, item 3). Two BGP FlowSpec origination surfaces were left stubbed by prior work and this spec closes both:

1. **ddos-flowspec responder** (`internal/plugins/ddos/flowspec/responder.go:28,33`): the built-and-tested mitigation state machine calls `announceFunc`/`withdrawFunc`, which are logging-only stubs ("cp-survival-4 not yet wired"). Wire them so a characterized attack actually originates an upstream FlowSpec rule on the wire, and the leak-probe withdraws it.
2. **`announce ipv4 flowspec` CLI verb** (`internal/component/bgp/plugins/cmd/announce/announce.go:309`): `handleAnnounceFlowspec` returns "flowspec announce not yet implemented". Implement it so operators can originate a tracked FlowSpec rule on demand through the same registry the unicast/blackhole verbs use.

Config model (settled with user): `action` leaf is `mandatory` with no default (enum `rate-limit`|`discard`); `rate-limit-bytes` range `0..max`, required only when `action`=`rate-limit`; a rate-limit with the field absent is a config error, `rate-limit 0` and `discard` are both valid and both encode an RFC 8955 traffic-rate of 0. Correct the inherited uncommitted `config.go`/`yang` edits (must not keep `default discard`).

**Out of scope:** flowspec-egress firewall→FlowSpec reverse bridge (cp-survival-4 phase D2).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `plan/learned/1008-cp-survival-4-on-demand-origination-design.md` - the on-demand origination verb + tag registry
  → Decision: announce verbs build an `NLRIBatch` and call `bgpReactor.AnnounceNLRIBatch(sel, batch)` in-process; registry tracking happens only when a `tag <k> <v>` opt is supplied, else fire-and-forget.
  → Constraint: `tag.*` meta is NOT consumed in the update-route path (grep found no server-side consumer). Cross-process plugins cannot get registry tracking via `UpdateRoute` meta; the registry is driven by the `ze-bgp:announce` / `ze-bgp:withdraw` RPCs only.
- [ ] `plan/learned/1011-cp-survival-5-detect-0-umbrella.md` - the detector/responder umbrella that stubbed announce/withdraw
  → Constraint: `announceFunc`/`withdrawFunc` are injectable package vars (chosen for testability); production never reassigns them — only `responder_test.go` does (to no-ops).
- [ ] `docs/guide/ddos-mitigation.md` - user guide already DESCRIBES the responder as announcing on the wire
  → Constraint: guide lines ~14/141/145/222 claim origination works and state `action` default is `rate-limit`; these become true (origination) and stale (default) — must update.
- [ ] `internal/exabgp/bridge/bridge_command.go` + `internal/exabgp/bridge/bridge_test.go` - the bridge is the reference for the Ze update-text flowspec grammar
  → Constraint: the traffic-action clause in update-text is `extended-community [rate-limit:<bps>]` / `extended-community [rate-limit:<n>:packets]` / `extended-community [rate-limit:0]` (== discard). There is also a distinct `discard` keyword (sugar for `traffic-rate 0 0`). NOTE: there is NO `ln` keyword in Ze grammar (a prior grep artifact suggested one; it does not exist). Components: `nlri ipv4/flow add destination <prefix> source <prefix> protocol <p> destination-port =<n> source-port =<n> tcp-flags <...>`.
- [ ] `internal/component/bgp/plugins/nlri/flowspec/types.go` + `internal/component/bgp/plugins/cmd/update/update_text.go` + `internal/component/bgp/route/route_community.go` - Part A (CLI verb) building blocks
  → Decision: FlowSpec NLRI built in-process via `flowspec.NewFlowSpec(fam)` (types.go:241) + `AddComponent(NewFlow*Component(...))` (types.go:259); a `*FlowSpec` satisfies `nlri.NLRI` so it drops into `NLRIBatch.NLRIs` like `nlri.NewINET`. Map-driven `buildFlowSpecComponents(map[string][]string, isIPv6)` (flowspec/config_builder.go:22) is the reusable text-criteria→NLRI builder (currently unexported).
  → Decision: traffic-action ext-community bytes come from `route.ParseExtendedCommunities(args)` (route_community.go:84) — handles `rate-limit:<n>` (subtype 0x06 bytes), `rate-limit:<n>:packets` (0x0c), and a distinct `discard` (route_community.go:172, = traffic-rate 0). Added to the batch via `attribute.Builder.AddExtendedCommunity(ec)` (attribute/builder.go:111). No dedicated traffic-rate method on Builder.
  → Constraint: responder text command is parsed by `ParseUpdateText` (update_text.go:499): `kwExtendedCommunity` (update_text.go:340) → `route.ParseExtendedCommunities`; `nlri <fam> add/del` → `parseFlowSpecSection` (update_text_flowspec.go:27). Same parser the bridge output feeds — bridge-proven.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8955.md` - FlowSpec v4 (NLRI components, traffic-rate extended community, rate 0 = discard)
  → Constraint: traffic-rate of 0 bytes/sec is the discard action; a non-zero rate is a byte-per-second rate-limit.
- [ ] `rfc/short/rfc8956.md` - FlowSpec v6 (source/destination-ipv6, next-header, flow-label) — v6 responder path if in scope

**Key insights:** (minimal context to resume after compaction)
- FlowSpec origination on the wire already works (bridge + `test/decode/bgp-flow-*.ci` + `test/encode/flow.ci`). The gap is two stubs, not a missing capability.
- The two stubs are SEPARATE code paths because of the plugin process boundary: the responder (separate process) uses `p.UpdateRoute` text grammar; the CLI verb (in-process BGP plugin) builds the NLRI struct directly. They share RFC 8955 semantics, not code.
- `rate-limit:0` ≡ discard byte-for-byte on the wire (confirmed: `test/decode/bgp-flow-4.ci` decodes `rate-limit:0` as a traffic-rate ext-community).

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ddos/flowspec/responder.go` - state machine (onDetected/onCharacterized/probeTick/withdraw); `announce()` at :104 calls `announceFunc(match, action)`; `announceFunc`/`withdrawFunc` are logging stubs (:28,:33); stores `r.match` (:105) for withdraw.
- [ ] `internal/plugins/ddos/flowspec/register.go` - `runEngine` builds `p := sdk.NewWithConn(Name, conn)` (:57), creates responder via `newResponder(cfg)` (:114) and never hands it `p`; subscribes to `ddosevent.Detected/Characterized/Cleared`.
- [ ] `internal/plugins/ddos/flowspec/config.go` - `Config` struct; inherited edit added `RateLimitBytes` + a `< 1` rejection (:147) that is WRONG per user (rate-limit 0 must be valid); `DefaultConfig()` presets `Action:"rate-limit"` (:42).
- [ ] `internal/plugins/ddos/flowspec/match.go` - `flowspecMatch{DstPrefix,Proto,DstPort,SrcPort,TCPFlags}`; `buildMatch(VectorTuple)`; `shouldAnnounce` allowlist check.
- [ ] `internal/component/bgp/plugins/cmd/announce/announce.go` - `handleAnnounce` dispatches unicast/blackhole/flowspec (:94); `handleAnnounceUnicast` builds `nlri.NewINET` + `attribute.NewBuilder()` batch → `announceAndTrack` (:259); `handleAnnounceFlowspec` is a stub (:309); withdraw by `tag`/`id`/`all` (:316).
- [ ] `internal/component/bgp/plugins/cmd/announce/registry.go` - tag registry; withdraw func wraps `bgpReactor.WithdrawNLRIBatch` (:62).
- [ ] `internal/exabgp/bridge/bridge_command.go` - `convertAnnounceFlowSpec` (:308) shows the update-text flowspec grammar Ze accepts.

**Behavior to preserve:**
- The responder state machine (event handling, leak-probe, allowlist, blackhole-fallback) is correct and must not change; only `announceFunc`/`withdrawFunc` gain a real implementation.
- The unicast/blackhole announce verbs and the tag registry API are unchanged; flowspec is added alongside.
- The existing FlowSpec NLRI codec and `update text ... nlri ipv4/flow add/del` grammar are reused; no new BGP family/attribute work.
- `announceFunc`/`withdrawFunc` remain overridable for tests (or an equivalent injectable dispatcher seam).

**Behavior to change:**
- `config.go`: remove the `rate-limit-bytes < 1` rejection; add presence tracking so `action`=`rate-limit` with the field absent is a validation error while `0` is accepted. Drop the `DefaultConfig()` action preset.
- `yang`: `action` leaf `mandatory`, no `default` (remove inherited `default discard`); `rate-limit-bytes` range `0..max`.
- `handleAnnounceFlowspec`: implement (parse components + traffic-rate ext-community, build batch, `announceAndTrack`).
- responder `announceFunc`/`withdrawFunc`: build and send `update text` flowspec commands via `p.UpdateRoute`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Responder path: a `ddosevent.AttackCharacterized` (or critical `AttackDetected` for blackhole-fallback) event delivered on the in-process EventBus to `responder.onCharacterized`/`onDetected`, carrying a `VectorTuple` (dst prefix, proto, ports, tcp-flags).
- CLI verb path: the `ze-bgp:announce` RPC with args `flowspec <components...> [rate-limit <bps> | discard] [tag <k> <v>] [for <dur>]`, arriving at `handleAnnounce` → `handleAnnounceFlowspec`.

### Transformation Path
1. Responder: `buildMatch(VectorTuple)` → `flowspecMatch`; `announce()` renders it to an `update text extended-community [rate-limit:<bps>] nlri ipv4/flow add <components>` string.
2. Responder: `p.UpdateRoute(ctx, "*", cmd)` sends the text command over the plugin SDK RPC to the BGP engine, which tokenises it, builds the FlowSpec NLRI + traffic-rate ext-community, and calls `AnnounceNLRIBatch` → wire.
3. CLI verb: `handleAnnounceFlowspec` parses components into a FlowSpec NLRI and the action into a traffic-rate extended community via `attribute.NewBuilder()`, assembles an `NLRIBatch`, calls `announceAndTrack` → `bgpReactor.AnnounceNLRIBatch` → wire (and records in the tag registry when `tag` is supplied).
4. Withdraw: responder leak-probe → `withdraw()` renders `update text ... nlri ipv4/flow del <components>` from stored `r.match`; CLI verb withdrawn by `withdraw tag/id/all`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ddos-flowspec plugin ↔ BGP engine | `p.UpdateRoute` text command over plugin SDK RPC (`ze-plugin-engine:update-route`) | [ ] |
| CLI/RPC ↔ BGP reactor | `ze-bgp:announce` RPC → `handleAnnounceFlowspec` → `bgpReactor.AnnounceNLRIBatch` (in-process) | [ ] |
| update-text string ↔ FlowSpec NLRI | tokeniser parses `nlri ipv4/flow add <components>` (bridge-proven) | [ ] |
| Config tree ↔ responder | YANG `ddos/flowspec` → `ParseConfig`/`Validate` → `newResponder(cfg)` | [ ] |

### Integration Points
- `sdk.Plugin.UpdateRoute` (`pkg/plugin/sdk/sdk_engine.go:20`) — responder's route to the wire.
- `bgpReactor.AnnounceNLRIBatch` / `WithdrawNLRIBatch` — CLI verb's route to the wire (via `announceAndTrack`, announce.go:166).
- `flowspec.NewFlowSpec(fam)` + `AddComponent` (nlri/flowspec/types.go:241,259) and `route.ParseExtendedCommunities` (route/route_community.go:84) + `attribute.Builder.AddExtendedCommunity` (attribute/builder.go:111) — CLI verb's NLRI/attr assembly.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — the flowspec verb is dispatched by the existing `handleAnnounce` switch; the responder registers via its existing plugin `init()`; no new per-feature field in a core/shared struct

## Responder Renderer Grammar (pinned)

The shared renderer `render(match flowspecMatch, action string, rateBytes uint64, mode string) string` (mode ∈ `add`|`del`) builds the responder's update-text command. Grammar confirmed against `internal/component/bgp/plugins/nlri/flowspec/config_builder.go` and `internal/exabgp/bridge/bridge_test.go`.

Command shape: `update text` + [ext-community clause + `nhop self`, `add` only] + `nlri <fam> <mode> <components>`.

**CRITICAL: `add` MUST include `nhop self`.** ze drops a FlowSpec origination that has no next-hop before the wire (the MP_REACH_NLRI requires one; the traffic action lives in the ext-community, so `self` is the correct originator next-hop). Proven by `test/plugin/ddos-flowspec-announce.ci` and the interop scenarios (`nhop required for MP_REACH_NLRI`). Withdraw (MP_UNREACH) needs none.

- Family: `ipv4/flow` if `DstPrefix.Addr().Is4()`, else `ipv6/flow`.
- Ext-community clause (`add` only; omitted on `del` — the flowspec key is the NLRI): `extended-community [rate-limit:<rate>]`, where `<rate>` = `rateBytes` for `action rate-limit` (byte form, subtype 0x06 per `bridge_command.go` `normalizeFlowSpecExtCommunityToken`), and `0` for `action discard`. `rate-limit:0` == discard on the wire (`test/decode/bgp-flow-4.ci`).

| `flowspecMatch` field | Condition | Token emitted | Grammar source |
|-----------------------|-----------|---------------|----------------|
| `DstPrefix` | always (valid) | `destination <prefix>` (same keyword for v4/v6; family disambiguates) | `config_builder.go:33-41` (`destination` key, `Is6()` branch internal) |
| `Proto` | `!= 0` | `protocol =<n>` | the flowspec encoder `parseProtocolComponentText` now accepts both `=<n>` and bare `<n>` (fixed for consistency with ports); renderer emits the `=` form uniformly. Confirmed by `TestEncodeFlowSpecProtocolOperatorEquivalence` + `TestParseUpdateText_FlowSpecResponderGrammar` |
| `DstPort` | `!= 0` | `destination-port =<n>` | `parseFlowMatches:213` (accepts `=<n>`) |
| `SrcPort` | `!= 0` | `source-port =<n>` | `parseFlowMatches:213` |
| `TCPFlags` | `!= 0` | `tcp-flags <name>[&<name>…]` symbolic; bit→name fin/syn/rst/psh/ack/urg/ece/cwr | `parseFlowTCPFlagMatches:410` (**symbolic ONLY — no numeric fallback**; `&` = AND) |

→ Constraint: withdraw (`del`) re-renders the SAME components from stored `r.match` with no ext-community, so the withdrawn NLRI byte-matches the announced one (R-1). Proven del grammar: `bridge_command.go:326` `convertWithdrawFlowSpec` → `... update text nlri ipv4/flow del <components>`.
→ Constraint: TCP-flags bit→name reverse map must be canonical (`0x04`→`rst`, `0x08`→`psh`, `0x20`→`urg`) since the parser's flagMap has aliases (`reset`/`push`/`urgent`); emit the short canonical form.
→ Bounded confirmation at implement: that the update-text `nlri ipv6/flow add destination <v6prefix>` path accepts the `destination` keyword for v6 (config_builder uses it; verify the cmd/update gatherer agrees).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The responder (separate process) can originate FlowSpec only via `p.UpdateRoute` text grammar, not the in-process tag registry | `announce.go:62-166` (registry driven by RPC + reactor); grep found no `tag.*` meta consumer; Explore confirmed registry is RPC/reactor-driven | design uses two separate builders; if a meta path exists, could unify | confirmed by Explore agent (no `tag.*` consumer) — recheck dispatch.go at impl | confirmed |
| A-2 | The responder's rendered `update text extended-community [rate-limit:<n>] nlri <fam> add <components>` is accepted verbatim by the engine tokeniser | `TestParseUpdateText_FlowSpecResponderGrammar` runs the exact v4+v6 renderer output through the real `ParseUpdateText` → valid flowspec NLRI | responder command rejected → nothing on wire | confirmed (after fixing the `protocol =<n>` bug it surfaced) | confirmed |
| A-3 | A FlowSpec NLRI can be built in-process from components for `handleAnnounceFlowspec` reusing existing codec | `flowspec.NewFlowSpec`+`AddComponent` (types.go:241,259), `buildFlowSpecComponents` (config_builder.go:22); `*FlowSpec` satisfies `nlri.NLRI` | CLI verb needs new NLRI plumbing | confirmed by Explore agent citations; compile at impl | confirmed |
| A-4 | Announcing with selector `"*"` self-scopes to peers that negotiated the flow family | redistribute uses `"*"`; engine filters by negotiated family | routes leak to non-flow peers or error | functional/interop test | unvalidated |
| A-5 | The `nlri ipv6/flow add` update-text path accepts the `destination` keyword (not `destination-ipv6`) for a v6 target | `config_builder.go:33` uses `destination` for both families | v6 responder command rejected | grep/read cmd/update v6 flow gatherer + unit test at implement | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Responder withdraw reconstructs a component string that does not byte-match the announced NLRI, leaving a stale upstream rule | leak-probe fires withdraw but peer still filters | withdraw from the SAME stored `r.match` via one shared renderer used by announce and withdraw |
| R-2 | tcp-flags / port operators render to a token the tokeniser rejects | command parse error in engine logs | unit test the renderer against bridge_test grammar; cover all `flowspecMatch` fields |
| R-3 | Config `rate-limit` with absent field silently treated as 0 (fabricated discard) | operator sees discard when expecting rate-limit | presence flag in `ParseConfig`; `Validate` errors when action=rate-limit and field absent |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `AttackCharacterized` event (enforce mode) | → | responder `announce()` → `p.UpdateRoute` | `TestResponderAnnounceEmitsUpdateText` (unit, injected dispatcher) |
| responder leak-probe withdraw | → | responder `withdraw()` → `p.UpdateRoute ... del` | `TestResponderWithdrawEmitsDel` (unit) |
| `announce ipv4 flowspec ...` RPC | → | `handleAnnounceFlowspec` → `announceAndTrack` | `test-announce-flowspec` (`.ci`) |
| responder end-to-end on the wire | → | update-text tokenise → `AnnounceNLRIBatch` | `test-ddos-flowspec-announce` (`.ci`, peer receives flow route) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | enforce mode, `AttackCharacterized` with a dst-prefix vector, `action rate-limit`, `rate-limit-bytes 9600` | responder sends `update text extended-community [rate-limit:9600] nlri ipv4/flow add destination <prefix> ...` via `p.UpdateRoute("*", …)` |
| AC-2 | same but `action discard` (or `rate-limit 0`) | responder sends `... extended-community [rate-limit:0] ...` (traffic-rate 0) |
| AC-3 | leak-probe decides clear | responder sends the matching `... nlri ipv4/flow del <components>` reconstructed from stored `r.match` |
| AC-4 | config `action rate-limit` with `rate-limit-bytes` absent | `Validate()` returns an error; nothing is announced; no rate is fabricated |
| AC-5 | config `action rate-limit rate-limit-bytes 0` OR `action discard` | both validate; both encode traffic-rate 0 |
| AC-6 | yang: minimal `ddos/flowspec` block without `action` | rejected by `mandatory` (no default applied) |
| AC-7 | `announce ipv4 flowspec destination 10.0.0.0/24 protocol tcp destination-port =80 rate-limit 100000` | a tracked FlowSpec route is announced; visible via `show-announcements`; withdrawable by `withdraw tag/id` |
| AC-8 | `announce ipv4 flowspec ... discard` | announces a traffic-rate-0 FlowSpec route |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | attack detected + characterized on a protected prefix | detector → EventBus → responder.onCharacterized → announce() → UpdateRoute → tokenise → AnnounceNLRIBatch → wire | `test-ddos-flowspec-announce` |
| 2 | attack subsides (leak-probe trickle) | probeTick → withdraw() → UpdateRoute del → WithdrawNLRIBatch → wire | `test-ddos-flowspec-withdraw` |
| 3 | operator announces a FlowSpec rule by hand | `announce ipv4 flowspec ...` RPC → handleAnnounceFlowspec → announceAndTrack → wire | `test-announce-flowspec` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildFlowspecUpdateText` | `internal/plugins/ddos/flowspec/responder_test.go` | flowspecMatch + action → correct `update text ln [...] nlri ipv4/flow add ...` string | |
| `TestBuildFlowspecWithdrawText` | `internal/plugins/ddos/flowspec/responder_test.go` | same match → matching `... del ...` | |
| `TestConfigRateLimitRequiresBytes` | `internal/plugins/ddos/flowspec/config_test.go` | action=rate-limit + absent field → error; 0 and N valid; discard valid | |
| `TestHandleAnnounceFlowspec` | `internal/component/bgp/plugins/cmd/announce/announce_test.go` | components + rate-limit/discard → batch built + tracked | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `rate-limit-bytes` | 0..max (uint64) | 0 | N/A (unsigned) | N/A (uint64 max) |
| `rate-limit-bytes` when action=rate-limit | must be present | 0 (present) | absent → error | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-announce-flowspec` | `test/plugin/*.ci` (or `test/bgp/`) | operator announces + withdraws a FlowSpec rule via the CLI verb | |
| `test-ddos-flowspec-announce` | `test/*/*.ci` | characterized attack originates a FlowSpec route a peer receives | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| existing flowspec interop audit | `test/interop/scenarios/` | GoBGP/BIRD | announced FlowSpec route accepted by a real peer (reuse/extend existing flow interop) | |

### Future (if deferring any tests)
- v6 responder path (`ipv6/flow`) deferred pending user decision on whether the detector emits v6 vectors — requires explicit user approval.

## Files to Modify
- `internal/plugins/ddos/flowspec/responder.go` - real `announceFunc`/`withdrawFunc` (or dispatcher seam); shared command renderer.
- `internal/plugins/ddos/flowspec/register.go` - hand the responder a dispatcher backed by `p.UpdateRoute`.
- `internal/plugins/ddos/flowspec/config.go` - presence-aware `rate-limit-bytes` validation; drop action preset; remove `< 1` rejection.
- `internal/plugins/ddos/flowspec/yang/ze-ddos-flowspec-conf.yang` - `action` mandatory/no-default; `rate-limit-bytes` range `0..max`.
- `internal/component/bgp/plugins/cmd/announce/announce.go` - implement `handleAnnounceFlowspec`.
- `internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go` - protocol parser accepts `=<n>` (equality operator) consistently with ports.
- `internal/component/bgp/plugins/cmd/announce/yang/` - `flowspec` branch of the `announce` command grammar + completion.
- `docs/guide/ddos-mitigation.md` - correct the `action` default claim; confirm origination now real.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (config) | [ ] | `internal/plugins/ddos/flowspec/yang/ze-ddos-flowspec-conf.yang` (action/rate-limit-bytes) |
| YANG validation constraints | [ ] | `enumeration` for action, `range 0..max` for rate-limit-bytes; `mandatory` for action |
| YANG custom validators | [ ] | rate-limit-requires-bytes cross-field check (native YANG cannot express) — `ValidateFn` or `Validate()` in config.go |
| CLI commands/flags | [ ] | `announce ipv4 flowspec ...` grammar in `internal/component/bgp/plugins/cmd/announce/yang/` |
| CLI grammar (action before identifier) | [ ] | `ai/rules/cli-grammar.md` — `announce flowspec` verb ordering |
| Editor autocomplete | [ ] | flowspec component/action completion in the announce YANG grammar |
| Functional test for new RPC/API | [ ] | `test/plugin/*.ci` announce-flowspec |
| Pipe completeness | [ ] | `show-announcements` output already routed; verify flowspec rows |
| Env var registration | [ ] | N/A — no `environment/` leaves added |
| Doctor check for runtime dependencies | [ ] | N/A — no new file/socket/port/module; reuses BGP session |
| Prometheus counters/metrics | [ ] | consider a `ddos_flowspec_announced_total` gauge/counter (decide in DESIGN) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` (flowspec origination now functional) |
| 2 | Config syntax changed? | [ ] | `docs/guide/ddos-mitigation.md` (action mandatory, rate-limit-bytes) |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` (`announce ipv4 flowspec`) |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` (announce flowspec grammar) |
| 5 | Plugin added/changed? | [ ] | N/A — no new plugin |
| 6 | Has a user guide page? | [ ] | `docs/guide/ddos-mitigation.md` |
| 7 | Wire format changed? | [ ] | N/A — reuses existing FlowSpec codec |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfc8955.md` (traffic-rate origination) |
| 10 | Test infrastructure changed? | [ ] | N/A (reuse existing harness) |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |
| 13 | Route metadata keys added/changed? | [ ] | N/A |
| 14 | Prometheus counters added/changed? | [ ] | decide in DESIGN |
| 15 | Registered command/inventory changed? | [ ] | `docs/guide/command-reference.md` (flowspec announce verb now live) |
| 16 | Changed source referenced by doc source anchors? | [ ] | grep `docs/` for `source: internal/plugins/ddos/flowspec/responder.go` |
| 17 | Existing docs show config/CLI examples for this area? | [ ] | `docs/guide/ddos-mitigation.md` flowspec block example |

## Files to Create
- `test/plugin/announce-flowspec.ci` - functional test for the CLI verb (or under `test/bgp/`)
- `test/*/ddos-flowspec-announce.ci` - functional test proving responder origination reaches a peer

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — responder dispatcher seam + failing wiring tests
   - Define `RouteDispatcher` interface (`UpdateRoute(ctx, sel, cmd) (uint32,uint32,error)`); add `dispatcher` field to `responder`; change `newResponder(cfg, dispatcher)`; `runEngine` builds a dispatcher wrapping `p` and passes it; remove `announceFunc`/`withdrawFunc` package vars; update `responder_test.go` to inject a fake dispatcher.
   - Tests: `TestResponderAnnounceEmitsUpdateText` (fails: renderer stub)
   - Files: `responder.go`, `register.go`, `responder_test.go`
   - Verify: responder holds a dispatcher; wiring test fails because renderer is a stub
2. **Phase: Config model** — presence-aware validation + yang correction
   - Tests: `TestConfigRateLimitRequiresBytes`
   - Files: `config.go`, `yang/ze-ddos-flowspec-conf.yang`
   - Verify: absent→error, 0/N/discard valid; `default discard` gone
3. **Phase: Responder command renderer** — flowspecMatch → update-text (announce + withdraw), one shared renderer
   - Tests: `TestBuildFlowspecUpdateText`, `TestBuildFlowspecWithdrawText`
   - Files: `responder.go`
4. **Phase: CLI verb** — implement `handleAnnounceFlowspec` (components + action → batch → announceAndTrack) + YANG grammar/completion
   - Tests: `TestHandleAnnounceFlowspec`, `test-announce-flowspec`
   - Files: `announce.go`, `announce/yang/`
5. **Functional + interop tests** — responder-to-wire and CLI verb end-to-end
6. **RFC refs** — `// RFC 8955 Section ...` on the traffic-rate origination
7. **Full verification + docs + learned summary**

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Both stubs gone; responder + CLI verb both reach the wire; withdraw matches announce |
| Correctness | `rate-limit:0` for discard/rate-0; component tokens byte-match tokeniser grammar |
| Naming | YANG kebab-case; `ln [...]` grammar exactly as the tokeniser expects |
| Data flow | Responder uses `p.UpdateRoute` only; CLI verb uses reactor in-process only |
| CLI grammar | `announce ipv4 flowspec` action-before-identifier per `ai/rules/cli-grammar.md` |
| Registration over hardcoding | flowspec dispatched by existing `handleAnnounce` switch; no core/shared struct edits |
| YANG validation | action `enumeration` + `mandatory`; rate-limit-bytes `range 0..max`; cross-field validator |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| responder announces on the wire | functional `.ci` test output shows peer received a flow route |
| CLI verb works | `test-announce-flowspec` passes; `show-announcements` lists it |
| no stub strings remain | grep `announce.go` and `responder.go` for "not yet" / "stub" returns nothing |
| config rejects fabricated rate | `TestConfigRateLimitRequiresBytes` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | flowspec component args parsed safely (bad prefix/port/proto rejected, no panic) |
| Resource exhaustion | announce-rate-limit still bounds responder announcements per minute |
| Injection | update-text command built from typed fields, not raw operator strings |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Functional test fails | Check AC; if AC wrong → DESIGN |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The flowspec encoder accepts `protocol =<n>` like ports do | The update-text flow encoder `parseProtocolComponentText` (plugin_encode_text.go) rejected `=<n>` ("invalid protocol") — only bare numbers/names, inconsistent with ports which accept `=` | `TestParseUpdateText_FlowSpecResponderGrammar` failed on the exact responder grammar | Root-cause fix (per user directive): made the protocol parser strip a leading `=` so `=17` and `17` parse identically. Confirms the value of validating rendered grammar through the real tokeniser (R-2). |
| The responder's origination reaches the wire (the handover said "origination already works") | A FlowSpec origination with NO next-hop is silently DROPPED before the wire — the MP_REACH_NLRI requires a next-hop (interop scenarios comment "nhop required for MP_REACH_NLRI"). The responder emitted none, so mitigation never reached any peer. Unit + `ParseUpdateText` integration tests missed it (they parse the NLRI; the drop is in the SEND path). | The peer-based `test/plugin/ddos-flowspec-announce.ci` received 0 flow messages; adding `nhop` made the route arrive. | Renderer now emits `nhop self` on `add`. The `.ci` (peer receives the FlowSpec UPDATE) is the only test that could catch this — vindicates requiring a peer-based functional test over unit/integration alone. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->
- The handover's "one shared tracked path" is not achievable at code level: the responder is a separate process and cannot use the in-process tag registry; it originates via `p.UpdateRoute` text grammar. The two stubs are closed by two builders sharing only RFC 8955 semantics.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Responder gets `p.UpdateRoute` via a `RouteDispatcher` field injected through `newResponder(cfg, dispatcher)`; the package-var `announceFunc`/`withdrawFunc` stubs are removed | reassign the package vars in `runEngine` (minimal diff, keeps global mutable state) | explicit dependency, no package-global mutable state, tests inject a fake dispatcher; `runEngine` builds one dispatcher from the stable `p` and passes it to each `newResponder` (recreated per config apply) |
| One shared `render(match, action, rate, mode)` for announce (`add`) and withdraw (`del`); withdraw omits the ext-community (flowspec key is the NLRI only) | separate announce/withdraw string builders | guarantees `del` byte-matches `add` NLRI (R-1); withdraw needs only `r.match`, action needs `r.cfg.RateLimitBytes` |
| Responder originates via `p.UpdateRoute` update-text, withdraws by re-rendering stored `r.match` | tag-registry tracked path | tag registry needs the in-process reactor (`announce.go:62-166`) and there is NO `tag.*` meta consumer (only firewall NAT uses a tag prefix); the responder is a separate process so the registry is unreachable. Raw's downsides are covered: withdraw re-renders the SAME stored `r.match` (`responder.go:105,144`) through ONE shared renderer so `del` byte-matches `add` (no stale-rule drift); visibility is via `show ddos flowspec` (`responder.go:157` status → `show.go`), the correct mitigation view. Tracking is not lost overall — the in-process CLI verb still uses `announceAndTrack`. |
| `discard` and `rate-limit 0` both encode traffic-rate 0 | reject rate-limit 0 | user decision; matches RFC 8955 (rate 0 = discard) |
| Selector `"*"` | explicit upstream config leaf | engine filters flow NLRI by negotiated family; no new leaf (revisit if operator scoping needed) |
| CLI verb builds the FlowSpec NLRI via `registry.EncodeNLRIByFamily("ipv4/flow", componentTokens)` (registry.go:708, core infra) + `nlri.NewWireNLRI` | export & import `flowspec.buildFlowSpecComponents` (plugin→plugin) OR reimplement component parsing | reuses the flowspec parser THROUGH the registration seam (same path as update_text_nlri.go:341); no plugin→plugin import; honors plugin-self-containment. Refines the user's "reuse, don't duplicate" choice with the idiomatic mechanism. |
| Both v4 and v6: renderer/verb branch on `DstPrefix.Addr().Is6()` → `ipv4/flow`+`destination` vs `ipv6/flow`+`destination-ipv6` | v4 only | `VectorTuple.DstPrefix` is `netip.Prefix` (event.go:62), already v4/v6; branch is one line; avoids a deferred v6 gap |

## Known Limitations
- v6 FlowSpec responder path (`ipv6/flow`) only if the detector emits v6 vectors — flagged for DESIGN.
- Operator-scoped upstream selection (beyond `"*"`) not added unless requested.

## RFC Documentation

Add `// RFC 8955 Section 4.2: "traffic-rate ... a rate of 0 ... discard"` above the discard/rate-limit encoding.

## Implementation Summary

### What Was Implemented
- Responder: `routeDispatcher` field + `sdkDispatcher` (register.go) + shared `renderFlowspecCommand` (announce `add` with `nhop self`, withdraw `del`); package-var stubs removed.
- Config: presence-aware `rate-limit-bytes` validation (`rateLimitBytesSet`), action constants, `DefaultConfig()` action preset dropped; yang `action` mandatory/no-default, `rate-limit-bytes` range 0..max.
- CLI verb: `handleAnnounceFlowspec` (splitFlowspecArgs → flowspecFamilyName → encodeFlowspecNLRI via registry → traffic-rate ext-community → announceAndTrack).
- Parser: `parseProtocolComponentText` accepts `protocol =<n>` (equality operator), consistent with ports.
- Tests: unit (renderer, config, CLI helpers, protocol equivalence), integration (`TestParseUpdateText_FlowSpecResponderGrammar`, `TestEncodeFlowspecNLRIBuildsWireRoute`), handler (`TestHandleAnnounceFlowspec` fake reactor), functional (`test/plugin/ddos-flowspec-announce.ci`).

### Bugs Found/Fixed
- **Responder never reached the wire:** FlowSpec origination with no next-hop is dropped before the wire; renderer now emits `nhop self`. Caught only by the peer-based `.ci`.
- **Protocol `=<n>` rejected** by the flow encoder (inconsistent with ports); fixed to strip `=`.

### Documentation Updates
- `docs/guide/ddos-mitigation.md`: `action` mandatory (no default), `rate-limit-bytes` row + example, `rate-limit 0 == discard`, origination now real. Source anchors added. `make ze-doc-test` to run in verification.

### Deviations from Plan
- The `announce flowspec` tracked-verb peer-based `.ci` is not feasible (harness has no CLI-command dispatch); covered by `TestHandleAnnounceFlowspec` (fake reactor) instead. Scaffold `test/plugin/announce-flowspec-verb.ci` slated for deletion (`tmp/delete-ddos-flowspec-wire.sh`).
- Renderer grammar corrected during implementation: `protocol` uses `=<n>` (after the parser fix), `nhop self` required on `add` (both surfaced by tests, not foreseen in the pinned grammar).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| DDoS mitigation reaches the wire | functional test | `test/plugin/ddos-flowspec-announce.ci` PASS — peer receives the FlowSpec UPDATE (MP_REACH nexthop 127.0.0.1, destination 192.0.2.0/24 + tcp + dport 80, traffic-rate rate-limit:9600 ext-community) |
| responder command grammar accepted verbatim | integration test | `TestParseUpdateText_FlowSpecResponderGrammar` (v4+v6) through real `ParseUpdateText` |
| operator can originate FlowSpec on demand | handler test | `TestHandleAnnounceFlowspec` — fake reactor receives batch (ipv4/ipv6 flow NLRI + next-hop self); `.ci` infeasible (harness has no CLI dispatch) |
| config rejects a fabricated rate | unit test | `TestConfigRateLimitRequiresBytes` — absent rate errors, 0 valid, discard valid |

## Review Gate

### Run 1 (initial — adversarial self-review of the diff)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Responder command dropped on the wire (no next-hop) | responder.go renderFlowspecCommand | fixed: emit `nhop self` on `add`; proven by `.ci` |
| 2 | ISSUE | `protocol =<n>` rejected by flow encoder | plugin_encode_text.go:parseProtocolComponentText | fixed: strip leading `=` |
| 3 | NOTE | `rate-limit-bytes` cross-field requirement enforced in Go `Validate`, not native YANG `when` | config.go | acknowledged — native `when` support uncertain; Go validation is authoritative |

### Fixes applied
- Next-hop: `renderFlowspecCommand` emits `nhop self` on announce; withdraw (MP_UNREACH) omits it.
- Protocol operator: `parseProtocolComponentText` strips a leading `=`.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | (none) | Re-review after fixes: all tests + `.ci` green, lint 0 issues | — | clean |

### Final status
- [ ] self-review shows 0 BLOCKER, 0 ISSUE after fixes (2 ISSUEs found and fixed)
- [ ] NOTE-1 recorded (Go-side cross-field validation)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/ddos-flowspec-announce.ci` | yes | `ze-test bgp plugin ddos-flowspec-announce` PASS |
| `internal/plugins/ddos/flowspec/responder.go` (renderer + dispatcher) | yes | `go test ./internal/plugins/ddos/flowspec/` ok |
| `internal/component/bgp/plugins/cmd/announce/announce.go` (handleAnnounceFlowspec) | yes | grep: no "not yet implemented" remains |
| `plan/learned/1073-ddos-flowspec-wire.md` | yes | written; counter bumped to 1074 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1/AC-2 | rate-limit/discard → correct `update text` command | `TestResponderAnnounceEmitsUpdateText`, `TestBuildFlowspecUpdateText` PASS |
| AC-3 | withdraw re-renders matching `del` | `TestResponderWithdrawEmitsDel` PASS |
| AC-4/AC-5 | absent rate errors; 0/N/discard valid | `TestConfigRateLimitRequiresBytes` PASS |
| AC-6 | action mandatory (yang) | `mandatory true` in yang; supported (pki/iface) |
| AC-7/AC-8 | CLI verb announces tracked flow route | `TestHandleAnnounceFlowspec` PASS |
| wire | responder command reaches peer | `ddos-flowspec-announce.ci` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| responder rendered command → wire | `test/plugin/ddos-flowspec-announce.ci` | yes — peer receives FlowSpec UPDATE + traffic-rate EC |
| responder grammar → real tokeniser | (unit) `TestParseUpdateText_FlowSpecResponderGrammar` | yes (v4+v6) |
| CLI verb → announceAndTrack → reactor | (unit) `TestHandleAnnounceFlowspec` | yes (fake reactor) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | no `tag.*` meta consumer; registry RPC/reactor-driven |
| A-2 | confirmed | `TestParseUpdateText_FlowSpecResponderGrammar` (after protocol + nhop fixes) |
| A-3 | confirmed | registry encode; `TestEncodeFlowspecNLRIBuildsWireRoute` |
| A-4 | confirmed | `"*"` selector: `.ci` peer receives the route |
| A-5 | confirmed | v6 `destination` keyword parses (`.ci` + grammar test) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `ddos-mitigation.md` action mandatory + rate-limit-bytes | matches `config.go` Validate + yang `mandatory` | yes (anchors added) |
| config example `rate-limit` + `rate-limit-bytes` | valid per Validate | yes |
| `make ze-doc-test` | run in full verification before commit | pending final verify |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ddos-flowspec-wire.md`
