# Spec: bgp-rpki-ov-community-ebgp-boundary

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Security boundary defect in the BGP relay: RFC 8097 Section 2 requires dropping type 0x43/0x00 from eBGP peers by default (landed with unit proofs; needs functional/interop proof and journal closure) and stripping it on egress to eBGP peers by default unless explicitly configured to send across a trusted eBGP boundary. |
| RFC gaps | RFC8097-2-4, RFC8097-2-9, RFC8097-2-10 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| Scope and YANG override leaf | Implement full RFC 8097 boundary handling: default inbound eBGP drop (`RFC8097-2-4`), default egress eBGP strip (`SHOULD`), and an explicit operator YANG override leaf to accept/send across trusted eBGP boundaries (updated 2026-11-24). |

## Task

RFC 8097 Section 2 (`rfc/full/rfc8097.txt`) defines the BGP Prefix Origin Validation State extended community (non-transitive opaque type `0x43`, sub-type `0x00`) to propagate RPKI origin validation state inside an autonomous system, and specifies two boundary protections plus explicit operator overrides:

- Inbound (`RFC8097-2-4`, MUST): "By default, implementations MUST drop the origin validation state extended community if received from an External BGP (EBGP) peer, without processing it further."
- Outbound (`RFC8097-2-9`, SHOULD NOT): "Similarly, by default, an implementation SHOULD NOT send the community to EBGP peers."
- Overrides (`RFC8097-2-10`, SHOULD): "However, it SHOULD be possible to configure an implementation to send or accept the community when warranted." (Examples: adjacent ASes under common administration, or an IXP route server signaling validation state to clients.)

Commit `2167203bdc` already landed all five RFC 8097 MUST requirements — including the inbound eBGP default drop (`RFC8097-2-4`), its `session { accept-origin-validation-state }` YANG leaf (default `false`), and both-polarity unit tests with discrimination proofs (`rfc/short/rfc8097.md`). Two pieces remain to complete the owner's 2026-11-24 full-boundary decision and close the open journal row (`plan/journal/scoped-attribute-relayed-past-its-boundary.md`):

1. **Outbound eBGP default strip (`RFC8097-2-9`) and explicit send override (`RFC8097-2-10`):** Today, nothing strips `0x43/0x00` on egress to eBGP peers. A route learned from iBGP (or from an eBGP peer with `accept-origin-validation-state true`), originated via config/API `update text`, or tagged by an export filter still leaks `0x43/0x00` across eBGP boundaries on every egress rail. Add `session { send-origin-validation-state }` (boolean, default `false`) in `ze-bgp-conf.yang` beside `accept-origin-validation-state`, wire it through `PeerSettings` and `parsePeerFromTree` (reading the delivered string leaf via `configvalue.Bool` from `internal/core/configvalue`, never `.(bool)`), and strip `0x43/0x00` in place inside `Session.writeUpdateBody` / `normalizeTransmittedUpdateWith` whenever sending to an eBGP peer without `send-origin-validation-state true` (removing `EXTENDED_COMMUNITIES` entirely when `0x43/0x00` was its sole entry, preserving all sibling extended communities and header flags, and zeroing Reserved octets `[2..6]` on allowed iBGP / opted-in eBGP sends). Also align `Session.sessionIsEBGP()`'s fallback (when `s.negotiated == nil`) to call `s.settings.IsEBGP()` so RFC 7705 AS migration is consistent on both inbound and outbound.
2. **End-to-end functional and interop proof + documentation & journal closure:** Add a functional `.ci` test (`test/plugin/rpki-ov-community-ebgp-boundary.ci`) and a Docker interop scenario against GoBGP (`test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/`) exercising both inbound and outbound eBGP boundary behavior (default drop/strip vs explicit opt-in and iBGP pass-through), update architecture/RFC docs and published YANG schema fixtures, and close the `2026-10-30` open row in `plan/journal/scoped-attribute-relayed-past-its-boundary.md`.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, schema authority, buffer-first encoding
  → Constraint: Every YANG leaf is a contract for operators and API clients; encoding mutates only private per-session buffers, never shared received/cache slices.
- [ ] `ai/doors/bgp-engine.md` - BGP reactor, zero-copy forwarding invariants, egress normalization
  → Constraint: Zero-copy forwarding requires received bytes to stay immutable; final-wire per-session edits happen on the session's private `writeBuf` copy in `Session.writeUpdateBody`.
- [ ] `ai/doors/bgp-plugins.md` - BGP plugin boundaries (`filter_community`, `bgp-rib`, `bgp-rs`)
  → Constraint: Core reactor never imports plugin packages; community filter handlers run only on forward rails with mods, so universal egress enforcement belongs in `Session.writeUpdateBody`.
- [ ] `ai/doors/config-yang.md` - YANG schema registration, string coercion, reload parity
  → Constraint: Plain leaves arrive as strings; read booleans through `configvalue.Bool` (`internal/core/configvalue`), never `.(bool)`, and keep published YANG golden files synchronized.
- [ ] `ai/doors/le-tooling.md` - `.ci` functional harness and Docker interop lab rules
  → Constraint: Interop scenarios register in `internal/le/interoplab/bgp/` with explicit positive and negative controls and fail when the feature is reverted.
- [ ] `docs/architecture/wire/attributes.md` - Attribute header flags, `EXTENDED_COMMUNITIES`, `normalizeTransmittedUpdate`
  → Decision: Extend `normalizeTransmittedUpdate` in `session_egress_normalize.go` (single-pass private-buffer walk before `ownership.filter`, `ownership.record`, and `DirectionSent`) rather than duplicating strip logic across four egress rails.
- [ ] `docs/architecture/bgp/egress-attribute-rules.md` - Egress attribute prohibitions and proof matrix
  → Decision: Pair every absence assertion in unit, `.ci`, and interop tests with a surviving sibling community and an opted-in / iBGP positive control.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8097.md` - RFC 8097 Section 2 encoding, decoding, inbound drop, egress strip, and overrides
  → Constraint: Inbound eBGP drop (`RFC8097-2-4`) is MUST by default; outbound eBGP strip (`RFC8097-2-9`) is SHOULD NOT send by default; both directions SHOULD be independently configurable (`RFC8097-2-10`); Reserved octets `[2..6]` MUST be 0 on send (`RFC8097-2-1`).
- [ ] `rfc/short/rfc4360.md` - Extended community 8-octet layout and Type High `0x43` (non-transitive opaque)
  → Constraint: `EXTENDED_COMMUNITIES` (type code 16, Optional Transitive) carries N 8-octet entries (`length % 8 == 0`); stripping `0x43/0x00` preserves every other 8-octet community and `FlagPartial` (RFC 4271 Section 5) on surviving attributes.

**Key insights:**
- Commit `2167203bdc` already implemented all 5 RFC 8097 MUSTs (`RFC8097-2-1` through `RFC8097-2-5`) plus `session accept-origin-validation-state`, with full discrimination records and `enforced` audit verdicts.
- Every BGP UPDATE Ze transmits — ordinary forward, RS fast path, static/API/RIB origination, stale readvertise, and raw injection — funnels through `Session.writeUpdateBody` (`session_write.go:829`), which copies `body` into `s.writeBuf` and runs `normalizeTransmittedUpdate` BEFORE `ownership.filter`, `ownership.record`, socket write, and `DirectionSent` events.
- Stripping `0x43/0x00` inside `normalizeTransmittedUpdateWith` achieves zero heap allocations, zero mutation of shared fan-out/cache buffers, automatic Adj-RIB-Out and BMP/event consistency, and single-site coverage of every egress rail.

## Current Behavior

**Source files read:**
- [ ] `internal/core/bgp/attribute/origin_validation_state.go` - defines `ExtCommTypeNonTransOpaque` (`0x43`), `ExtCommSubTypeOriginValidation` (`0x00`), `IsOriginValidationStateExtCommunity`, `NewOriginValidationStateExtCommunity`, `DecodeOriginValidationStateExtCommunity`, `ResolveOriginValidationState`, `HasOriginValidationStateNonZeroReserved`, `ZeroOriginValidationStateReservedBytes`, `SanitizeOriginValidationStateAttr`, keyword parse/format.
- [ ] `internal/core/bgp/attribute/community.go` - `ExtendedCommunities.WriteTo` already zeroes Reserved octets `[2..6]` on `0x43/0x00` (`RFC8097-2-1`).
- [ ] `internal/component/bgp/reactor/session_origin_validation.go` - `Session.sanitizeOriginValidationState` (called from `session_read.go:538`) already drops `0x43/0x00` on inbound eBGP by default unless `s.settings.AcceptOriginValidationState` is true, ignores Reserved octets, keeps the greatest valid state, and discards/logs values > 2. Its fallback `sessionIsEBGP()` compares `s.settings.LocalAS != s.settings.PeerAS` when `s.negotiated == nil` rather than calling `s.settings.IsEBGP()`.
- [ ] `internal/component/bgp/reactor/session_write.go` - `Session.writeUpdateBody` copies `body` into `s.writeBuf` and calls `normalizeTransmittedUpdate(body, ownership.encoding)` at line 884, then runs `ownership.filter`, `ownership.record`, socket write, and `onMessageReceived(..., DirectionSent, ...)`. Nothing strips `0x43/0x00` on eBGP egress.
- [ ] `internal/component/bgp/reactor/session_egress_normalize.go` - `normalizeTransmittedUpdate` walks path attributes in place in `s.writeBuf`, clears unused flag bits, clears Prefix-SID Label-Index reserved/flags, sets MP_UNREACH withdrawal compatibility, and compacts duplicate `LARGE_COMMUNITY` entries in place. It does not touch `AttrExtCommunity` (code 16).
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - declares `accept-srv6-prefix-sid`, `propagate-srv6-prefix-sid`, and `accept-origin-validation-state` under `grouping peer-fields / container session` (lines 903-932), with no outbound `send-origin-validation-state` leaf.
- [ ] `internal/component/bgp/reactor/peer_settings.go` & `config.go` - declare and parse `AcceptOriginValidationState` (default `false`); no `SendOriginValidationState` field exists yet.
- [ ] `internal/component/bgp/reactor/reactor_api.go` & `peer_settings_apply.go` - `peerSettingsEqual` compares the whole `PeerSettings` struct via `reflect.DeepEqual`, making non-hot-swappable fields restart-scoped on reload automatically.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go` - zeroes dirty Reserved bytes on egress Set/Add/Keep for `AttrExtCommunity`, does not enforce eBGP boundary stripping.
- [ ] `internal/component/bgp/reactor/forward_prefix_sid.go` & `forward_local_pref.go` - single-predicate helpers (`prefixSIDAllowedTo`, `localPrefAllowedTo`) for per-rail attribute rules.

**Behavior to preserve:**
- All five already-landed RFC 8097 MUST behaviors (`RFC8097-2-1` through `RFC8097-2-5`), including inbound eBGP default drop and `session accept-origin-validation-state true` override, remain intact and passing with their discrimination records.
- Every extended community other than `0x43/0x00` (including Route Targets, Route Origins, FlowSpec actions, and other non-transitive opaque subtypes such as `0x43/0x01`) is forwarded and originated unchanged.
- Routes sent to iBGP peers carry `0x43/0x00` unchanged (with Reserved octets `[2..6]` zeroed).
- Zero-copy shared receive/cache/dedup buffers are never mutated by egress stripping; `FlagPartial` and `FlagExtLength` on surviving `EXTENDED_COMMUNITIES` attributes are preserved; malformed `EXTENDED_COMMUNITIES` lengths (`length % 8 != 0`) are left untouched by the normalizer.
- Existing configurations omitting `send-origin-validation-state` remain valid and inherit `false`.

**Behavior to change:**
- Add `session > send-origin-validation-state` (boolean, default `false`) to `ze-bgp-conf.yang`, `PeerSettings.SendOriginValidationState`, and `parsePeerFromTree` (parsed via `configvalue.Bool` from `internal/core/configvalue`).
- On every UPDATE transmitted through `Session.writeUpdateBody`, when `!originValidationStateSendAllowedHeld()` (eBGP peer without `send-origin-validation-state true`), strip every `0x43/0x00` entry from `EXTENDED_COMMUNITIES` in place in `s.writeBuf`, removing the entire `EXTENDED_COMMUNITIES` attribute when no sibling extended community survives. When allowed (iBGP or eBGP with `send-origin-validation-state true`), zero Reserved octets `[2..6]` of any `0x43/0x00` entry in place.
- Align `Session.sessionIsEBGP()`'s fallback (when `s.negotiated == nil`) to call `s.settings.IsEBGP()` for RFC 7705 AS migration consistency.
- Add functional `.ci` and Docker GoBGP interop coverage for both inbound and outbound RFC 8097 boundary handling, update docs and published YANG goldens, and close the `2026-10-30` open journal row.

## Existing Mechanisms

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| RFC 8097 attribute codecs & helpers | `internal/core/bgp/attribute/origin_validation_state.go` (`IsOriginValidationStateExtCommunity`, `ZeroOriginValidationStateReservedBytes`, `SanitizeOriginValidationStateAttr`, `NewOriginValidationStateExtCommunity`) and `community.go` (`ExtendedCommunities.WriteTo`) | Encodes, parses, formats, and sanitizes `0x43/0x00` extended communities; enforces `RFC8097-2-1` Reserved=0 on typed send paths | Extends: reuses `ExtCommTypeNonTransOpaque`, `ExtCommSubTypeOriginValidation`, and `ZeroOriginValidationStateReservedBytes` on the final-wire egress normalizer |
| Inbound RFC 8097 receive sanitizer | `internal/component/bgp/reactor/session_origin_validation.go` (`Session.sanitizeOriginValidationState`, `Session.sessionIsEBGP`) wired at `session_read.go:538` | Drops `0x43/0x00` from eBGP peers by default (`RFC8097-2-4`) unless `AcceptOriginValidationState` is true; ignores Reserved (`RFC8097-2-2`); selects greatest valid state (`RFC8097-2-3`); discards and logs >2 (`RFC8097-2-5`) | Extends: aligns `sessionIsEBGP()` nil-negotiated fallback to `s.settings.IsEBGP()`, adds sibling egress predicate `originValidationStateSendAllowedTo` / `originValidationStateSendAllowedHeld`, and adds `.ci` and interop proofs covering inbound + outbound |
| Single-pass transmitted-UPDATE normalizer | `internal/component/bgp/reactor/session_egress_normalize.go` (`normalizeTransmittedUpdate`, `compactLargeCommunities`), called from `session_write.go:884` (`Session.writeUpdateBody`) | Walks path attributes in place on the session's private `writeBuf` copy before `ownership.filter`, `ownership.record`, socket write, and `DirectionSent` events; zeroes unused flags, clears Prefix-SID Label-Index fields, sets MP_UNREACH compatibility, and compacts duplicate `LARGE_COMMUNITY` values in place | Extends: adds `normalizeTransmittedUpdateWith(body, encoding, stripOriginValidationState)` to strip `0x43/0x00` entries (or remove sole `AttrExtCommunity`) in place on default eBGP egress and zero Reserved bytes `[2..6]` on allowed egress across every egress rail |
| Per-peer EBGP boundary YANG leaves & reload diff | `internal/component/bgp/yang/ze-bgp-conf.yang` (`grouping peer-fields / container session` leaves `accept-srv6-prefix-sid`, `propagate-srv6-prefix-sid`, `accept-origin-validation-state`), `reactor/config.go` (`parsePeerFromTree`), `reactor/reactor_api.go` (`peerSettingsEqual`), `reactor/peer_settings_apply.go` (`hotSwappableSettings`) | Declares boolean per-peer/per-group boundary opt-in leaves defaulting to `false`, parses existing leaves via `mapBool` (while `reactor/config_aigp.go` uses `internal/core/configvalue`), and treats non-hot-swappable `PeerSettings` fields as restart-scoped on reload | Extends: adds sibling `leaf send-origin-validation-state` (default `false`) and `PeerSettings.SendOriginValidationState bool`, parsed in `parsePeerFromTree` via `configvalue.Bool` (`internal/core/configvalue`) and verified in reload and dynamic-group parity tests |
| Community egress filter handler | `internal/component/bgp/plugins/filter_community/handler.go` (`extCommunityAttrModHandler`, `emitCommunityOp`, `emitKeptCommunityValue`) | Handles `send-community` suppression and policy community Add/Remove/Set on forward rails; zeroes dirty RFC 8097 Reserved bytes on emitted ops | Does not apply to the eBGP boundary strip: runs only on forward rails when `mods` has a code-16 op, whereas `normalizeTransmittedUpdateWith` in `writeUpdateBody` covers every egress rail (forwarded, RS fast path, originated, policy-added, raw) in one allocation-free pass |
| Per-rail attribute suppression (`applyFactsPrefixSID`, `applyFactsLocalPref`) | `internal/component/bgp/reactor/forward_prefix_sid.go`, `forward_local_pref.go`, `reactor_api_forward.go`, `forward_rs.go` | Strips whole attributes (code 40, code 5) across forward and origination rails | Does not apply directly: RFC 8097 strips only `0x43/0x00` members inside code 16 while retaining sibling extended communities; `normalizeTransmittedUpdateWith` already handles sub-attribute in-place compaction (`compactLargeCommunities`) once for every rail |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | always | Config is YANG-modeled, and `ConfigProvider` is its single authority | agrees | The new `send-origin-validation-state` override is declared as a standard YANG boolean leaf in `ze-bgp-conf.yang` under `grouping peer-fields / container session`, so config validation, editor completion, web UI, and `PeersFromTree` all derive from one schema. |
| `ai/doors/bgp-engine.md` | Traps | `ze-bgp-conf.yang`, `reactor/*`, `config/dynamic_group_parity_test.go` | Zero-copy needs the received bytes to survive. | agrees | Egress stripping runs in place inside `Session.writeUpdateBody` on the session's private `s.writeBuf` copy rather than mutating shared received, cache, or fan-out buffers, so zero-copy forwarding survives untouched. |
| `ai/doors/le-tooling.md` | Boundaries | `internal/le/site/testdata/*`, `internal/le/interoplab/bgp/*`, `test/*` | A bare `go test` omits the feature tags | agrees | All unit test verification commands in this spec use `./le test unit pkg path ...`, functional tests live in `test/plugin/*.ci`, and Docker interop scenarios register under `internal/le/interoplab/bgp/`. |
| `docs/architecture/core-design.md` | Executive Summary | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | `WireUpdate` - BGP UPDATE message as bytes | agrees | Outbound RFC 8097 boundary enforcement operates directly on the raw wire UPDATE bytes in `s.writeBuf` without introducing parsed struct copies on the forwarding path. |
| `docs/architecture/wire/attributes.md` | Attribute Flags | `session_origin_validation.go`, `session_egress_normalize.go`, `rfc8097_origin_validation_state_test.go`, modified page | `writeUpdateBody`, the writer every UPDATE funnels through, clears them in the session's private copy of the body, the raw diagnostic write included, in the same walk that removes LARGE_COMMUNITY repeats, clears the Prefix-SID Label-Index fields, and writes 0x800000 into the Compatibility field of every labeled-unicast and VPN withdrawal in MP_UNREACH_NLRI (RFC 8277 Section 2.4). | agrees | Extends this exact single-pass private-buffer walk (`normalizeTransmittedUpdateWith`) to strip `0x43/0x00` on default eBGP egress and zero Reserved octets `[2..6]` on allowed sends, and updates this page with matching source anchors. |
| `docs/architecture/bgp/egress-attribute-rules.md` | Proof | modified page | Each functional test carries its confining negative in the same run: one UPDATE, several destinations, opposite transforms, so neither half can pass by accident. | agrees | Both `test/plugin/rpki-ov-community-ebgp-boundary.ci` and `bgp-rpki-ov-ebgp-boundary-gobgp` pair default eBGP stripping with opted-in eBGP and iBGP retention plus a surviving sibling Route Target in the same run, and this page is updated with the RFC 8097 rule and proof row. |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | `session_write.go`, `bgp_test.go`, `check_special.go`, `check_rfc.go` | A scenario with no checker, or a nil checker, is an ERROR rather than a skipped test, so a fixture and its registry cannot silently disagree. | agrees | The new `bgp-rpki-ov-ebgp-boundary-gobgp` scenario directory carries only declarative inputs and registers its typed Go checker in `internal/le/interoplab/bgp/` alongside positive/negative controls and counterfactual verification. |
| `docs/architecture/buffer-architecture.md` | Executive Summary | `session_write.go` | One representation (bytes). Everything else is views/iterators. | agrees | `normalizeTransmittedUpdateWith` walks the attribute bytes in `s.writeBuf` directly and compacts non-`0x43/0x00` 8-octet entries in place with zero heap allocations or intermediate struct copies. |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | `ze-bgp-conf.yang` | YANG defines format. Extensions declare behavior. Implementation executes behavior. | agrees | `send-origin-validation-state` uses native YANG `type boolean; default false;` with `ze:help` in `ze-bgp-conf.yang`, parsed into `PeerSettings` by `parsePeerFromTree` and enforced by `Session.writeUpdateBody`. |
| `docs/architecture/api/update-syntax.md` | Overview | `ze-bgp-conf.yang`, `session_write.go` | - | silent | Covers `update text` command grammar and raw UPDATE API syntax; silent on RFC 8097 `0x43/0x00` eBGP egress stripping and `send-origin-validation-state`. |
| `docs/architecture/bgp/filter-path-asn.md` | The config | `ze-bgp-conf.yang` | - | silent | Covers the `reject-asn` BGP policy filter plugin; anchored only to `filter-policy` in `ze-bgp-conf.yang` and silent on RFC 8097 extended communities. |
| `docs/architecture/config/environment-block.md` | TL;DR | `ze-bgp-conf.yang` | - | silent | Covers the top-level `environment { }` config block; anchored only to `ze-bgp-conf.yang`'s environment augment and silent on BGP session extended-community leaves. |
| `docs/architecture/config/environment.md` | Overview | `ze-bgp-conf.yang` | - | silent | Covers environment variable precedence and reactor buffer env keys; silent on per-peer YANG `send-origin-validation-state`. |
| `docs/architecture/config/syntax.md` | What a reader receives | `ze-bgp-conf.yang`, `config.go` | Read all three through `internal/core/configvalue` (`Bool`, `Int`, `LeafList`, `ListEntries`) rather than asserting a type. The obligation is stated in the configuration rule. A `.(bool)` or `.(float64)` assertion on a delivered leaf never succeeds, and it fails the way an absent value fails. | agrees | Because plain YANG leaves arrive as strings (`"true"`/`"false"`), `parsePeerFromTree` reads `session > send-origin-validation-state` through `configvalue.Bool` (`internal/core/configvalue`), never a bare `.(bool)` type assertion, preventing silent discard of the operator's setting. |
| `docs/architecture/rib-transition.md` | Executive Summary | `ze-bgp-conf.yang` | - | silent | Covers the RIB transition and update-block watchdog; silent on RFC 8097 `0x43/0x00` eBGP boundary stripping. |
| `docs/architecture/api/architecture.md` | TL;DR (Read This First) | `peer_settings.go`, `session_write.go` | - | silent | Covers plugin bus transport, event delivery, and cache control; silent on RFC 8097 `0x43/0x00` eBGP stripping (post-strip bytes naturally flow to `DirectionSent`). |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | `peer_settings.go`, `session_write.go` | - | silent | Covers `UpdateBuilder`, batching, and FlowSpec next-hop origination; silent on RFC 8097 `0x43/0x00` eBGP stripping (which runs uniformly downstream in `writeUpdateBody`). |
| `docs/architecture/api/commands.md` | Overview | `config.go` | - | silent | Covers API command paths and pipe semantics; silent on RFC 8097 eBGP extended-community boundary handling. |
| `docs/architecture/meta/role.md` | Keys Set | `config.go` | - | silent | Covers RFC 9234 BGP Roles and OTC attribute metadata; silent on RFC 8097 Origin Validation State extended communities. |
| `docs/architecture/rfc-may-decisions.md` | RFC 4760 Section 7 - Non-Negotiated AFI/SAFI Handling | `config.go` | - | silent | Records optional RFC MAY behavior choices such as `ignore-mismatch`; silent on RFC 8097 Section 2 boundary requirements. |
| `docs/architecture/wire/capabilities.md` | Negotiated State | `config.go` | RFC 7705 Section 4.2 lets a renumbering router hold an internal session under its second AS, so equal AS numbers are the ordinary internal case and not the whole rule. The reactor answers both before it negotiates: `sessionPeerAS` takes the configured AS before the advertised one, `PeerSettings.isIBGPWith` decides `Internal`, and `PeerIdentity.IsIBGP` returns that verdict unchanged. | agrees | Aligns `Session.sessionIsEBGP()`'s nil-negotiated fallback from `s.settings.LocalAS != s.settings.PeerAS` to `s.settings.IsEBGP()` (the complement of `PeerSettings.isIBGPWith`) while using `!neg.Identity.IsIBGP()` (`!neg.Identity.Internal`) once negotiated, so both inbound and outbound RFC 8097 boundary checks obey RFC 7705 Section 4.2. |
| `docs/architecture/api/json-format.md` | Overview | `session_egress_normalize.go` | - | silent | Covers JSON serialization of BGP events and messages; silent on RFC 8097 eBGP egress stripping (existing `origin-validation-state:<state>` rendering is preserved). |
| `docs/architecture/wire/nlri.md` | Overview | `session_egress_normalize.go` | - | silent | Covers NLRI prefix and label wire encodings (anchored to `setWithdrawalCompatibility`); silent on `EXTENDED_COMMUNITIES` attribute 16. |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | `session_write.go` | - | silent | Covers Established-state timers and Send Hold Timer lifecycle; silent on RFC 8097 attribute stripping. |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | `session_write.go` | - | silent | Covers BGP FSM states and transitions; silent on extended community processing. |
| `docs/architecture/behavior/peer-lifecycle.md` | TL;DR | `session_write.go` | - | silent | Covers peer connection establishment, teardown, and write retirement; silent on RFC 8097 extended communities. |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | `session_write.go` | - | silent | Covers RS fast-path selection, worker pool, and policy epochs; silent on RFC 8097 eBGP egress stripping (which runs downstream in `writeUpdateBody`). |
| `docs/architecture/congestion-industry.md` | Universal Rule | `session_write.go` | - | silent | Covers BGP slow-peer congestion handling and write budgets; silent on extended community filtering. |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | `session_write.go` | - | silent | Covers RFC 8654 65535-octet message sizing and NOTIFICATION truncation; silent on RFC 8097 (stripping only shrinks UPDATEs). |
| `docs/architecture/forward-congestion-pool.md` | Invariant | `session_write.go` | - | silent | Covers per-peer forward worker queue and flush handoff; silent on RFC 8097 extended communities. |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | `session_write.go` | - | silent | Covers `bgp-rib` storage and best-path selection; silent on RFC 8097 eBGP egress stripping. |
| `docs/architecture/route-selection.md` | Overview | `session_write.go` | - | silent | Covers BGP best-path tie-breaking and AIGP origination; silent on RFC 8097 extended community stripping. |
| `docs/architecture/wire/messages.md` | TL;DR (Read This First) | `session_write.go` | - | silent | Covers BGP message header and top-level message types; silent on `EXTENDED_COMMUNITIES` subtype `0x43/0x00`. |
| `docs/architecture/wire/mp-nlri-ordering.md` | Key Insight | `session_write.go` | - | silent | Covers RFC 7606 Section 5.1 MP_REACH/MP_UNREACH ordering; silent on RFC 8097 extended communities (attribute order is unchanged). |
| `docs/architecture/wire/nlri-bgpls.md` | Wire Format Overview | `session_write.go` | - | silent | Covers BGP-LS NLRI wire encoding; silent on RFC 8097 extended communities. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now. | agrees | Pure BGP wire-attribute boundary handling touches no Linux kernel or VPP dataplane programming and does not alter kernel/VPP mismatch refusal. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each. | agrees | Does not alter kernel/VPP validation verdicts across start, commit, reload, or `ze config validate`. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation. | agrees | Touches no firewall YANG annotations or VPP firewall verifier logic. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it". | agrees | Introduces no data-plane selector or `ze:backend` annotation. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it." | agrees | Touches no next-hop rewriting surface; `session { send-origin-validation-state }` is a protocol trust-boundary opt-in (like `accept-origin-validation-state` and `propagate-srv6-prefix-sid`), not a next-hop knob. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs"). | agrees | Does not touch `policychain.Sets` or any `policy { modify }` next-hop authoring forms. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)"). | agrees | Leaves `policychain.Chain` built-in steps and `pkg/plugin/rpc.FilterInsertion` untouched. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self"). | agrees | Does not alter next-hop folding or raw-override provenance checking. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load"). | agrees | Does not change explicit IPv4/IPv6 next-hop validation or import-chain refusal rules. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes"). | agrees | Leaves next-hop preservation on locally originated routes untouched while stripping `0x43/0x00` uniformly on all eBGP egress. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?". | agrees | Adds no automatic filter-chain member; RFC 8097 eBGP stripping is final-wire attribute normalization in `Session.writeUpdateBody`, parallel to RFC 4271 flags and RFC 8092 dedup. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it. | agrees | Introduces no daemon startup refusal; `send-origin-validation-state` is a valid boolean leaf defaulting to `false` that cannot fail `ze start`. |
| `docs/architecture/core-design.md` | Design decisions | `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`, `session_write.go` | The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch. | agrees | Touches no kernel/VPP mismatch startup or commit handling. |
| `docs/architecture/api/architecture.md` | Design decisions | `peer_settings.go`, `session_write.go` | When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added". | agrees | Does not change `\| implied` rendering or insert implied filter lines. |
| `docs/architecture/api/architecture.md` | Design decisions | `peer_settings.go`, `session_write.go` | Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs | agrees | Adds no bus participant and leaves bus message registrations untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | `peer_settings.go`, `session_write.go` | A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody | agrees | Does not modify `bus` `send > to` compilation or delivery semantics. |
| `docs/architecture/api/architecture.md` | Design decisions | `peer_settings.go`, `session_write.go` | A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody | agrees | Does not modify `bus` `receive > from` filtering. |
| `docs/architecture/api/architecture.md` | Design decisions | `peer_settings.go`, `session_write.go` | A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message | agrees | Does not modify bus allow-list enforcement. |
| `docs/architecture/api/architecture.md` | Design decisions | `peer_settings.go`, `session_write.go` | What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines | agrees | Does not change participant/bus filter compilation or commit conflict refusal. |
| `docs/architecture/api/architecture.md` | Design decisions | `peer_settings.go`, `session_write.go` | The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself | agrees | Introduces no bus arbitration or exclusive-role rule. |
| `docs/architecture/api/architecture.md` | Design decisions | `peer_settings.go`, `session_write.go` | One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry | agrees | Does not alter bus effective-display rendering. |
| `docs/architecture/update-building.md` | Design decisions | `peer_settings.go`, `session_write.go` | Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family | agrees | Preserves `session { flowspec-redirect-next-hop }` untouched and follows the same explicit boolean per-session opt-in pattern (default `false`) for `session { send-origin-validation-state }`. |
| `docs/architecture/api/commands.md` | Design decisions | `config.go` | The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included | agrees | Adds no CLI command or subcommand and leaves command/pipe grammar untouched. |
| `docs/architecture/api/commands.md` | Design decisions | `config.go` | A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed." | agrees | Adds no pipe operator or command override. |
| `docs/architecture/api/commands.md` | Design decisions | `config.go` | A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does | agrees | Touches no pipe execution behavior. |
| `docs/architecture/testing/interop.md` | Design decisions | `session_write.go`, `bgp_test.go`, `check_special.go`, `check_rfc.go` | Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec) | agrees | The new pure-BGP `bgp-rpki-ov-ebgp-boundary-gobgp` scenario declares no kernel capabilities, matching the BGP interop suite's scoped kernel capability rule. |

## Data Flow

### Entry Point
- **Inbound:** BGP UPDATE received over TCP carrying `EXTENDED_COMMUNITIES` (type code 16) with one or more `0x43/0x00` entries -> `Session.ReadAndProcess` -> `processValidatedMessage` -> `Session.sanitizeOriginValidationState` (already landed).
- **Outbound:** Any BGP UPDATE reaching `Session.writeUpdateBody` (general forward rail `forwardUpdateCore`, RS fast path `reactorForwardRS`, static/API/RIB origination `writeOrdinaryUpdateBody`, stale readvertise, export policy addition, or raw injection `SendRawMessage`) carrying `EXTENDED_COMMUNITIES` with one or more `0x43/0x00` entries.
- **Configuration:** Operator configures `bgp { peer|group ... { session { send-origin-validation-state true; } } }` (or omits it for default `false`).

### Transformation Path
1. **Config parse:** `ze-bgp-conf.yang` validates `session > send-origin-validation-state` as boolean (default `false`); `parsePeerFromTree` (`reactor/config.go`) reads the delivered string leaf via `configvalue.Bool` (`internal/core/configvalue`, never `.(bool)`) into `PeerSettings.SendOriginValidationState`.
2. **Inbound boundary (existing):** `Session.ReadAndProcess` -> `processValidatedMessage` (`session_read.go:538`) -> `Session.sanitizeOriginValidationState` -> `attribute.SanitizeOriginValidationStateAttr(values, s.sessionIsEBGP(), s.settings.AcceptOriginValidationState)`. Drops `0x43/0x00` on default eBGP without further processing or logging; retains/resolves on iBGP or opted-in eBGP.
3. **Outbound boundary (new):** Every egress rail delivers its UPDATE body to `Session.writeUpdateBody` (`session_write.go:829`), which copies `body` into `s.writeBuf` and computes `stripOVS := !s.originValidationStateSendAllowedHeld()` (true iff `s.sessionIsEBGPHeld() && !s.settings.SendOriginValidationState`).
4. **In-place normalization:** `normalizeTransmittedUpdateWith(body, ownership.encoding, stripOVS)` (`session_egress_normalize.go`) walks every path attribute in `s.writeBuf`:
   - When `code == attribute.AttrExtCommunity` and `length % 8 == 0`:
     - If `stripOVS` is `true` and any 8-octet entry has `entry[0] == 0x43 && entry[1] == 0x00`: compacts non-`0x43/0x00` entries in place (`compactWithoutOriginValidationState`). If `kept == 0`, removes the entire attribute (`header + length` bytes), shifts trailing attributes and NLRI left, updates `Total Path Attribute Length`, and stays at `pos` to inspect the next attribute. If `0 < kept < length`, updates the 1- or 2-byte attribute length header (preserving `FlagPartial` and `FlagExtLength`), shifts trailing bytes left, and updates `Total Path Attribute Length`.
     - If `stripOVS` is `false`: calls `attribute.ZeroOriginValidationStateReservedBytes(value)` in place so any surviving `0x43/0x00` carries zero Reserved octets `[2..6]`.
   - Returns the shortened body length `end`.
5. **Post-normalization accounting & emit:** `ownership.filter`, `ownership.record`, `s.bufWriter.Write`, and `s.onMessageReceived(..., DirectionSent, ...)` all run on the normalized `body[:end]`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file / YANG ↔ Reactor | `ze-bgp-conf.yang` boolean leaf `send-origin-validation-state` -> `parsePeerFromTree` (`configvalue.Bool`) -> `PeerSettings.SendOriginValidationState` | No |
| Wire ↔ Adj-RIB-In (Inbound) | `Session.ReadAndProcess` -> `Session.sanitizeOriginValidationState` before cache/bus/RIB publication (already landed) | No |
| Egress rails ↔ Peer TCP socket (Outbound) | `Session.writeUpdateBody` private `s.writeBuf` copy -> `normalizeTransmittedUpdateWith` before `ownership.filter`, `ownership.record`, `bufWriter.Write`, and `DirectionSent` | No |
| Config reload ↔ Running peer | `peerSettingsEqual` (`reflect.DeepEqual`) detects `SendOriginValidationState` changes -> session restart | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/component/bgp/` and `internal/core/bgp/` via `git grep` for `writeUpdateBody`, `writeRawUpdateBody`, `writeOriginatedRawUpdate`, `SendRawMessage`, `bufWriter.Write`, `normalizeTransmittedUpdate`, `AttrExtCommunity`, `OriginValidation`, `accept-origin-validation-state`, and `propagate-srv6-prefix-sid`. Positive controls confirmed: every BGP UPDATE transmitted by Ze (`forwardUpdateCore`, `reactorForwardRS`, `writeOrdinaryUpdateBody`, `writeMessageHeld`, `SendRawMessage`) passes through `Session.writeUpdateBody` (`session_write.go:829`, the sole `bufWriter.Write` site for `TypeUPDATE`), and `ze-bgp-conf.yang` `grouping peer-fields / container session` is the single schema declaration for `accept-origin-validation-state` (read at `reactor/config.go:415` and published to `internal/le/site/testdata/published-configuration.md` and `published-yang-config-tree.json`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| BGP UPDATE `EXTENDED_COMMUNITIES` (type code 16) on eBGP egress | writer | `internal/component/bgp/reactor/session_write.go::Session.writeUpdateBody` + `session_egress_normalize.go::normalizeTransmittedUpdateWith` (fed by `forwardUpdateCore`, `reactorForwardRS`, `writeOrdinaryUpdateBody`, `writeOriginatedRawUpdate`) | Sole `TypeUPDATE` `bufWriter.Write` path (`session_write.go:884, 997`); currently leaves `AttrExtCommunity` untouched on egress | Change: strip `0x43/0x00` in place (or drop entire code-16 attribute when sole) when `!originValidationStateSendAllowedHeld()`, and zero Reserved octets `[2..6]` when allowed; proven by unit, `.ci`, and GoBGP interop tests |
| same | reader | External BGP peers outside repository (FRR, BIRD, GoBGP, any RFC 4360 / RFC 8097 speaker) | Standard RFC 4360 / RFC 8097 wire decoding of path attribute code 16 | Preserve: output stays valid RFC 4360 wire framing (with `Total Path Attribute Length`, attribute length, `FlagPartial`, and `FlagExtLength` preserved); proven against GoBGP in `bgp-rpki-ov-ebgp-boundary-gobgp` |
| same | reader | `internal/component/bgp/reactor/session_ownership.go::adjOutWrite.filter`/`record` and `Session.onMessageReceived` (`DirectionSent`) -> `internal/core/bgp/attribute/community.go::ParseExtendedCommunities` / `extcomm_decoded.go::AppendDecoded` | Invoked in `session_write.go:901, 989, 1051` after `normalizeTransmittedUpdate` | Preserve: runs on post-strip `body`, so Adj-RIB-Out duplicate suppression, BMP Post-Policy Adj-RIB-Out, and `DirectionSent` observers stay exact without code changes |
| YANG schema `ze-bgp-conf` (`peer-fields/session`) | writer | `internal/component/bgp/yang/ze-bgp-conf.yang` (`grouping peer-fields / container session / leaf send-origin-validation-state`) + `internal/le/site/testdata/published-configuration.md`, `published-yang-config-tree.json` | Sibling leaf `accept-origin-validation-state` declared at `ze-bgp-conf.yang:923` and mirrored in published site goldens | Change: add optional boolean leaf `send-origin-validation-state` defaulting to `false` and refresh published goldens |
| same | reader | `internal/component/bgp/reactor/config.go::parsePeerFromTree` | Reads sibling `accept-origin-validation-state` via `mapBool` at `config.go:415`; `reactor/config_aigp.go` already uses `internal/core/configvalue` | Change: parse `send-origin-validation-state` via `configvalue.Bool` (`internal/core/configvalue`) into `PeerSettings.SendOriginValidationState` |
| same | reader | `internal/component/config/yang` validator, CLI editor autocomplete, and external operator configs/API clients | Schema auto-loads `ze-bgp-conf` (`schema_test.go:108-150`); existing configs omit the leaf | Preserve: additive boolean leaf with `default false` keeps every existing config valid and automatically populates CLI/editor/API completion |

### Integration Points
- `Session.writeUpdateBody` (`internal/component/bgp/reactor/session_write.go:884`) — calls `normalizeTransmittedUpdateWith(body, ownership.encoding, !s.originValidationStateSendAllowedHeld())`.
- `parsePeerFromTree` (`internal/component/bgp/reactor/config.go:415`) — parses `session > send-origin-validation-state` via `configvalue.Bool` (`internal/core/configvalue`) into `PeerSettings.SendOriginValidationState`.
- `peerSettingsEqual` (`internal/component/bgp/reactor/reactor_api.go:1623`) — automatically compares `SendOriginValidationState` via whole-struct `reflect.DeepEqual` so reload triggers session restart.
- `internal/le/interoplab/bgp/check_special.go` & `bgp_test.go` — registers `bgp-rpki-ov-ebgp-boundary-gobgp` checker.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Config flows through YANG -> `parsePeerFromTree` -> `PeerSettings`; outbound wire enforcement runs in `Session.writeUpdateBody` -> `normalizeTransmittedUpdateWith`, the single final-wire path every UPDATE rail uses |
| No unintended coupling (components stay isolated) | Yes | `reactor` depends only on `internal/core/bgp/attribute` (existing dependency); no plugin package is imported by reactor |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends `normalizeTransmittedUpdate` (single-pass in-place compactor already used for RFC 8092) and `session_origin_validation.go` rather than duplicating strip logic across four egress rails |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Operates in place on the session's private `s.writeBuf` copy already made by `writeUpdateBody`, with zero heap allocation and zero mutation of shared received/cache/fan-out buffers |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, view, family, or handler is added; YANG schema registration in `internal/component/bgp/yang/` auto-publishes the new leaf to validator, editor, and API |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/config/`, `internal/component/cli/`, `internal/component/bgp/reactor/` for `accept-origin-validation-state` and `propagate-srv6-prefix-sid`; validator, editor completion, and `peerSettingsEqual` derive automatically from YANG and struct reflection; only `parsePeerFromTree` parses peer session leaves (same site as every other `PeerSettings` field) and interop scenario registers in `interoplab/bgp` checker map |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every BGP UPDATE transmitted by Ze (forwarded, RS fast path, originated, policy-modified, raw) passes through `Session.writeUpdateBody` | `git grep "bufWriter.Write" internal/component/bgp/reactor/` shows line 997 (`writeUpdateBody`) is the sole `TypeUPDATE` write site; `writeMessageHeld` and `SendRawMessage` both route `TypeUPDATE` into `writeOriginatedRawUpdate` -> `writeUpdateBody` | An egress rail could bypass `0x43/0x00` stripping | Unit tests driving ordinary forward, RS fast path, originated announce, and `SendRawMessage` plus functional `.ci` | confirmed |
| A-2 | Inbound eBGP drop (`RFC8097-2-4`) and all five RFC 8097 MUSTs are already implemented and audited in HEAD | `git show 2167203bdc`, `session_origin_validation.go`, `rfc/audit/rfc8097/RFC8097-2-*.json`, `rfc/short/rfc8097.md` | Inbound work would still need core implementation | Verified directly in HEAD source; end-to-end `.ci` and GoBGP interop test will prove live daemon behavior | confirmed |
| A-3 | GoBGP 4.11.0 (in `test/interop/Dockerfile.gobgp`) displays RFC 8097 `0x43/0x00` extended communities in `gobgp global rib` / `gobgp-paths` so a foreign daemon can observe both absence (default eBGP) and presence (opted-in eBGP) | `docs/architecture/testing/interop.md` ("Tested Daemons"): `gobgp-paths` prints Extended Communities via `String()` and `MarshalJSON` | Interop scenario would need transparent wire relay (like `bgp-local-pref-strip-gobgp`) to prove absence/presence if GoBGP strips `0x43/0x00` on eBGP receive by default | Verify GoBGP's eBGP receive handling for `0x43/0x00` during interop implementation; use transparent extended relay capture if GoBGP strips `0x43/0x00` on eBGP receive | unvalidated |
| A-4 | Removing the entire `EXTENDED_COMMUNITIES` attribute when `0x43/0x00` is its sole entry never creates an empty attribute or skips the following attribute in `normalizeTransmittedUpdateWith` | Inspection of `session_egress_normalize.go:61-112` loop mechanics (`pos` increment vs `continue`) | Trailing attributes could skip normalization or `Total Path Attribute Length` could be wrong | Unit tests with sole `0x43/0x00` followed by `LARGE_COMMUNITY` (with duplicates) and unused-flag-bit attributes | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Shared wire buffer corruption during multi-peer fan-out (default eBGP + iBGP/opted-in eBGP sharing one forwarded or batched body) | Opted-in eBGP or iBGP peer loses `0x43/0x00` when a default eBGP peer is in the same fan-out | `writeUpdateBody` copies `body` into `s.writeBuf` before calling `normalizeTransmittedUpdateWith`; add explicit multi-destination fan-out unit test and `.ci` test asserting both destinations simultaneously |
| R-2 | GoBGP interop vacuity trap: GoBGP itself may drop `0x43/0x00` on eBGP receive per RFC 8097 Section 2, which would make a RIB-only absence check pass even if Ze leaked `0x43/0x00` | Opted-in eBGP peer (`send-origin-validation-state true`) ALSO fails to show `0x43/0x00` in GoBGP's RIB, or reverting Ze's strip still passes | Pair default eBGP and opted-in eBGP in the same scenario AND/OR capture the real wire bytes through `extended_relay` (same proven pattern as `bgp-local-pref-strip-gobgp`), verifying counterfactual failure when Ze's strip is reverted |
| R-3 | Published YANG schema golden tests fail after adding `send-origin-validation-state` | `internal/le/site` golden test reports diff in `published-configuration.md` or `published-yang-config-tree.json` | Update both golden files in the same change set and verify via `internal/le/site` unit test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A bug in `normalizeTransmittedUpdateWith` could corrupt `Total Path Attribute Length` or trailing NLRI bytes on any UPDATE carrying `EXTENDED_COMMUNITIES` toward eBGP peers; or a missing guard could continue leaking `0x43/0x00` across eBGP boundaries |
| How is it reverted? | Single commit revert (`send-origin-validation-state` defaults to `false`; no on-disk state migration) |
| Who else touches this path? | `session_egress_normalize.go` also runs RFC 4271 unused-flag clearing, RFC 8669 Prefix-SID Label-Index clearing, RFC 8277 withdrawal compatibility, and RFC 8092 LARGE_COMMUNITY dedup; all existing tests for those four must stay green |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-bgp-conf.yang` (`session/send-origin-validation-state`) | → | `PeersFromTree` (`reactor/config.go::parsePeerFromTree`) -> `PeerSettings.SendOriginValidationState` | `TestSchemaSendOriginValidationStateDefaultsFalse` (`yang/schema_test.go`) & `TestRFC8097EgressEBGPStripsOriginValidationByDefault` (`reactor/rfc8097_origin_validation_state_test.go`) |
| `Session.writeUpdateBody` (`reactor/session_write.go:884`) | → | `Session.originValidationStateSendAllowedHeld` (`session_origin_validation.go`) -> `normalizeTransmittedUpdateWith` (`session_egress_normalize.go`) | `TestRFC8097EgressEBGPStripsOriginValidationByDefault` & `TestRFC8097EgressIBGPAndConfiguredEBGPRetainOriginValidation` (`reactor/rfc8097_origin_validation_state_test.go`) |
| Full `ze` daemon eBGP <-> iBGP relay | → | `Session.sanitizeOriginValidationState` (inbound) & `normalizeTransmittedUpdateWith` (outbound) | `test/plugin/rpki-ov-community-ebgp-boundary.ci` |
| Reload diff (`reconcilePeersJournaled`) | → | `peerSettingsEqual` (`reactor/reactor_api.go:1623`) | `TestPeerSettingsEqualDetectsEachSignificantField` (`reactor/peer_settings_reload_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An UPDATE carrying `EXTENDED_COMMUNITIES` with both a sibling community (`target:65000:9`) and one or more RFC 8097 `0x43/0x00` entries is sent to a default eBGP peer (`send-origin-validation-state` omitted or `false`) via forwarded, RS fast-path, originated, policy-added, or raw write paths | The transmitted wire UPDATE retains the route NLRI, all other path attributes, the preserved `FlagPartial` and `FlagExtLength` bits on code 16, and `target:65000:9` intact while stripping every `0x43/0x00` entry and updating attribute and total path attribute lengths to match | Non-`0x43/0x00` extended communities (such as `0x0002` Route Target, `0x43/0x01` non-RFC-8097 non-transitive opaque, and FlowSpec communities) survive intact because RFC 8097 Section 2 governs only type `0x43` sub-type `0x00` and general RFC 4360 non-transitive stripping is out of scope |
| AC-2 | An UPDATE whose `EXTENDED_COMMUNITIES` attribute contains only RFC 8097 `0x43/0x00` entries (single or multiple, with attributes both before and after code 16 including duplicate `LARGE_COMMUNITY` or dirty flag bits) is sent to a default eBGP peer | The transmitted wire UPDATE delivers the route NLRI and all remaining path attributes (properly normalized) with the entire `EXTENDED_COMMUNITIES` attribute omitted and `Total Path Attribute Length` reduced by the exact removed attribute span | All other path attributes and the route NLRI survive intact because stripping `0x43/0x00` must preserve the route advertisement and sibling attribute normalization |
| AC-3 | An UPDATE carrying `0x43/0x00` (states 0, 1, or 2, including non-zero Reserved octets `[2..6]`) is sent to an iBGP peer (default config) or to an eBGP peer configured with `session { send-origin-validation-state true; }` (peer, group, or dynamic listen-range group), including simultaneous fan-out alongside a default eBGP peer | The iBGP and opted-in eBGP peers receive the UPDATE with `0x43/0x00` intact, Reserved octets `[2..6]` zeroed, and validation state `[7]` preserved, while the sibling default eBGP peer receives the route stripped of `0x43/0x00` with zero shared-buffer mutation | `0x43/0x00` survives on iBGP (its primary RFC 8097 intra-AS scope) and on eBGP peers with explicit `send-origin-validation-state true` (the RFC 8097 Section 2 trusted-eBGP exception); non-`0x43/0x00` communities keep their bytes verbatim |
| AC-4 | `ze-bgp-conf.yang` declares `session > send-origin-validation-state` (boolean, default `false`) under `grouping peer-fields` beside `accept-origin-validation-state` | Schema validation accepts boolean `true`/`false` and rejects non-booleans at `bgp/group/session`, `bgp/group/peer/session`, and `bgp/peer/session`; `NewPeerSettings` and `PeersFromTree` default `SendOriginValidationState` to `false` and `parsePeerFromTree` reads the delivered string leaf via `configvalue.Bool` (`internal/core/configvalue`, never `.(bool)`); dynamic listen-range groups inherit both `accept-origin-validation-state` and `send-origin-validation-state` identically to static groups; changing `SendOriginValidationState` causes `peerSettingsEqual` to return `false` | Unchanged peer configs compare equal under `peerSettingsEqual` (avoiding gratuitous session bounce), and `accept-origin-validation-state` remains independent so inbound and outbound trust can be configured asymmetrically |
| AC-5 | Full `ze` daemon runs with default and opted-in eBGP peers plus an iBGP peer in both `test/plugin/rpki-ov-community-ebgp-boundary.ci` and Docker interop scenario `bgp-rpki-ov-ebgp-boundary-gobgp` | Inbound: default eBGP route carrying `target:65001:7` + `0x43/0x00` reaches the iBGP peer with `target:65001:7` intact and `0x43/0x00` removed, while opted-in eBGP (`accept-origin-validation-state true`) route reaches the iBGP peer with `0x43/0x00` intact. Outbound: iBGP route carrying `target:65000:9` + `0x43/0x00` reaches the default eBGP peer with `target:65000:9` intact and `0x43/0x00` removed, while reaching the opted-in eBGP peer (`send-origin-validation-state true`) with both `target:65000:9` and `0x43/0x00` intact | Sibling Route Target communities (`target:...`) survive on every leg as the positive delivery control; `0x43/0x00` survives only on iBGP hops and on eBGP hops carrying the matching directional opt-in leaf |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs Ze between an untrusted outside eBGP peer and an internal iBGP peer (default config) | eBGP wire -> `Session.ReadAndProcess` -> `sanitizeOriginValidationState` drops `0x43/0x00` -> Adj-RIB-In / RIB -> iBGP peer receives route with sibling communities intact and no forged `0x43/0x00` | `test/plugin/rpki-ov-community-ebgp-boundary.ci`, `bgp-rpki-ov-ebgp-boundary-gobgp` |
| 2 | Runs Ze between an internal iBGP peer (carrying `0x43/0x00`) and an outside eBGP peer (default config) | iBGP wire -> RIB/relay -> `Session.writeUpdateBody` -> `normalizeTransmittedUpdateWith` strips `0x43/0x00` -> eBGP peer receives route with sibling communities intact and no leaked `0x43/0x00` | `test/plugin/rpki-ov-community-ebgp-boundary.ci`, `bgp-rpki-ov-ebgp-boundary-gobgp` |
| 3 | Configures `session { accept-origin-validation-state true; }` and/or `session { send-origin-validation-state true; }` on a trusted eBGP peer (e.g., adjacent AS under common administration or IXP route server) | `ze-bgp-conf.yang` -> `parsePeerFromTree` -> `PeerSettings` -> `sanitizeOriginValidationState` (inbound) / `normalizeTransmittedUpdateWith` (outbound) preserves `0x43/0x00` with zeroed Reserved octets | `test/plugin/rpki-ov-community-ebgp-boundary.ci`, `bgp-rpki-ov-ebgp-boundary-gobgp`, `TestRFC8097EgressIBGPAndConfiguredEBGPRetainOriginValidation` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSchemaSendOriginValidationStateDefaultsFalse` | `internal/component/bgp/yang/schema_test.go` | AC-4: `session/send-origin-validation-state` is boolean, defaults to `false`, validates across group/group-peer/peer scopes | |
| `TestPeerSettingsEqualDetectsEachSignificantField` (`SendOriginValidationState` row) | `internal/component/bgp/reactor/peer_settings_reload_test.go` | AC-4: toggling `SendOriginValidationState` makes `peerSettingsEqual` return `false` | |
| `TestDynamicGroupParity` (`accept/send-origin-validation-state` leaves) | `internal/component/bgp/config/dynamic_group_parity_test.go` | AC-4: dynamic listen-range groups inherit both RFC 8097 leaves identically to static groups | |
| `TestRFC8097EgressEBGPStripsOriginValidationByDefault` | `internal/component/bgp/reactor/rfc8097_origin_validation_state_test.go` | AC-1, AC-2, AC-4: `NewPeerSettings` & `PeersFromTree` default `false`; default eBGP strips single/multiple `0x43/0x00` with sibling RT preserved, preserves `FlagPartial` and `FlagExtLength` (both 1-byte and 2-byte length headers), removes sole code-16 cleanly without skipping following `LARGE_COMMUNITY`/flags normalization, preserves `0x43/0x01`, leaves malformed length untouched, covers `writeUpdateBody` and `SendRawMessage`, and verifies `DirectionSent` observes stripped bytes | |
| `TestRFC8097EgressIBGPAndConfiguredEBGPRetainOriginValidation` | `internal/component/bgp/reactor/rfc8097_origin_validation_state_test.go` | AC-3: iBGP default and eBGP with `send-origin-validation-state true` retain `0x43/0x00` (states 0, 1, 2) with Reserved octets `[2..6]` zeroed; RFC 7705 AS-migration iBGP session retains `0x43/0x00`; shared input buffer fanned out to default eBGP + opted-in eBGP + iBGP leaves shared buffer unmutated | |
| Existing `TestRFC8097*` tests | `internal/component/bgp/reactor/rfc8097_origin_validation_state_test.go` | Preserves all 5 already-landed RFC 8097 MUST proofs (`RFC8097-2-1`..`RFC8097-2-5`) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | boolean leaf only (`true` / `false`); validation state octet boundary `0..2` vs `3..255` already tested in `TestRFC8097ValueAboveTwoDiscardedAndLogged` | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rpki-ov-community-ebgp-boundary` | `test/plugin/rpki-ov-community-ebgp-boundary.ci` | AC-5: real `ze` daemon proves default eBGP inbound drop + outbound strip, explicit `accept-origin-validation-state true` inbound acceptance, explicit `send-origin-validation-state true` outbound send, iBGP pass-through, and sibling Route Target preservation on every leg | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rpki-ov-ebgp-boundary-gobgp` | `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/` | GoBGP 4.11.0 (plus wire capture via `extended_relay` if GoBGP strips eBGP `0x43/0x00` before RIB storage) | AC-5: real wire interop proving Ze strips `0x43/0x00` toward default eBGP GoBGP while sending it to opted-in eBGP GoBGP (`send-origin-validation-state true`) and dropping untrusted eBGP `0x43/0x00` on inbound; verified red when Ze's strip is reverted | |

## Files to Modify
- `internal/component/bgp/yang/ze-bgp-conf.yang` - add `leaf send-origin-validation-state` (boolean, default `false`) under `grouping peer-fields / container session` beside `accept-origin-validation-state`
- `internal/component/bgp/yang/schema_test.go` - add `TestSchemaSendOriginValidationStateDefaultsFalse`
- `internal/component/bgp/reactor/peer_settings.go` - add `SendOriginValidationState bool` to `PeerSettings`
- `internal/component/bgp/reactor/config.go` - parse `session > send-origin-validation-state` via `configvalue.Bool` (`internal/core/configvalue`) in `parsePeerFromTree`
- `internal/component/bgp/reactor/session_origin_validation.go` - add `originValidationStateSendAllowedTo`, `Session.sessionIsEBGPHeld`, `Session.originValidationStateSendAllowedHeld`, and align `sessionIsEBGP()` fallback to `s.settings.IsEBGP()`
- `internal/component/bgp/reactor/session_egress_normalize.go` - add `normalizeTransmittedUpdateWith` and in-place `AttrExtCommunity` `0x43/0x00` stripping (plus Reserved-zeroing on allowed sends)
- `internal/component/bgp/reactor/session_write.go` - call `normalizeTransmittedUpdateWith(body, ownership.encoding, !s.originValidationStateSendAllowedHeld())` in `writeUpdateBody`
- `internal/component/bgp/reactor/peer_settings_reload_test.go` - add `SendOriginValidationState` row to `TestPeerSettingsEqualDetectsEachSignificantField`
- `internal/component/bgp/reactor/rfc8097_origin_validation_state_test.go` - add egress unit tests (`TestRFC8097EgressEBGPStripsOriginValidationByDefault`, `TestRFC8097EgressIBGPAndConfiguredEBGPRetainOriginValidation`)
- `internal/component/bgp/config/dynamic_group_parity_test.go` - add `accept-origin-validation-state true` and `send-origin-validation-state true` to `parityConfig`
- `internal/le/site/testdata/published-configuration.md` - refresh published YANG configuration golden with `send-origin-validation-state`
- `internal/le/site/testdata/published-yang-config-tree.json` - refresh published YANG config tree golden with `send-origin-validation-state`
- `internal/le/interoplab/bgp/bgp_test.go` - register `bgp-rpki-ov-ebgp-boundary-gobgp` scenario
- `internal/le/interoplab/bgp/check_special.go` - dispatch `bgp-rpki-ov-ebgp-boundary-gobgp` checker
- `internal/le/interoplab/bgp/check_rfc.go` - implement `checkRPKIOVEBGPBoundary` checker (or dedicated `check_rfc8097.go`)
- `docs/architecture/wire/attributes.md` - document RFC 8097 `0x43/0x00` boundary handling under Section 16 (`EXTENDED_COMMUNITY`) and update the `normalizeTransmittedUpdate` bullet with source anchors
- `docs/architecture/bgp/egress-attribute-rules.md` - document the RFC 8097 eBGP egress strip and `send-origin-validation-state` override with source anchors and proof row
- `rfc/short/rfc8097.md` - update `Meta` to record `RFC8097-2-9` egress strip and `RFC8097-2-10` `send-origin-validation-state` override
- `plan/journal/scoped-attribute-relayed-past-its-boundary.md` - close the `2026-10-30` open row

## Files to Create
- `test/plugin/rpki-ov-community-ebgp-boundary.ci` - end-to-end functional test for inbound and outbound RFC 8097 eBGP boundary handling
- `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/ze.conf` - Ze config for GoBGP interop scenario
- `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/gobgp.toml` - GoBGP peer config for interop scenario
- `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/inject.msg` - BGP route injector fixture for interop scenario
- `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/inject-args` - injector arguments for interop scenario
- `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/speaker-args` - speaker/relay arguments for interop scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang`: `leaf send-origin-validation-state` (boolean, default `false`) under `grouping peer-fields / container session` |
| YANG validation constraints | Yes | Native `type boolean; default false;` verified in `TestSchemaSendOriginValidationStateDefaultsFalse` |
| YANG custom validators | No | Native YANG `boolean` type enforces all constraints; no `ze:validate` needed |
| CLI commands/flags | No | No new CLI verb; config editor automatically exposes `set bgp peer <name> session send-origin-validation-state true` from YANG |
| CLI grammar (keyword before value) | N-A | No new CLI command added |
| Editor autocomplete | Yes | Automatic from YANG boolean leaf; verified via YANG schema resolution test |
| Functional test for new RPC/API | Yes | `test/plugin/rpki-ov-community-ebgp-boundary.ci` proves config + datapath end to end |
| Pipe completeness | N-A | No command output added or changed |
| Env var registration | No | Per-peer BGP policy leaf is YANG-only (matching `accept-origin-validation-state` and `propagate-srv6-prefix-sid`) |
| Doctor check for runtime dependencies | No | No external binary, socket, file path, or kernel feature required |
| Doctor code per problem class | No | Egress strip is normal RFC 8097 default behavior (silent on wire, matching LOCAL_PREF and Prefix-SID egress strips); inbound invalid >2 already uses registered `doctor-bgp-origin-validation-state-invalid` |
| Prometheus counters/metrics | No | No new metric required for routine eBGP community boundary strip |
| BGP family surface (new SAFI / capability / attribute) | No | Uses existing attribute code 16 (`EXTENDED_COMMUNITIES`); no new AFI/SAFI, capability, or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | `rpki` feature already covers RFC 8097; no maturity tier change |
| 2 | Config syntax changed? | Yes | `internal/le/site/testdata/published-configuration.md` and `published-yang-config-tree.json` (generated from YANG schema); `docs/architecture/wire/attributes.md` and `docs/architecture/bgp/egress-attribute-rules.md` |
| 3 | CLI command added/changed? | No | No CLI command added or changed |
| 4 | API/RPC added/changed? | No | No API or RPC added or changed |
| 5 | Plugin added/changed? | No | No plugin added or changed; change is in core BGP reactor |
| 6 | Has a user guide page? | No | `docs/guide/rpki.md` documents RTR/ROA/ASPA validation (`bgp-rpki`), not per-peer extended-community relay options (same as `accept-origin-validation-state`) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md` (Section 16 `EXTENDED_COMMUNITY` & `normalizeTransmittedUpdate` bullet) and `docs/architecture/bgp/egress-attribute-rules.md` |
| 8 | Plugin SDK/protocol changed? | No | No `pkg/` or process-protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8097.md` (`Meta` updated for `RFC8097-2-9` egress strip and `RFC8097-2-10` `send-origin-validation-state` override) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (unaffected unless new helper contract added; scenario uses existing BGP GoBGP/extended-relay lab infrastructure) |
| 11 | Affects daemon comparison? | No | No daemon comparison table row changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/attributes.md` and `docs/architecture/bgp/egress-attribute-rules.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata key added or changed |
| 14 | Prometheus counters added/changed? | No | No metric added or changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration inventory entry changed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked via `./le spec citation anchors spec plan/release/fixes/spec-bgp-rpki-ov-community-ebgp-boundary.md`: `docs/architecture/wire/attributes.md` and `docs/architecture/bgp/egress-attribute-rules.md` are updated; `docs/architecture/core-design.md` (declared by `// Design:` on `peer_settings.go`, `config.go`, `session_write.go`) and `docs/architecture/testing/interop.md` (declared by `check_special.go`, `check_rfc.go`) are unaffected because their existing design contracts and `## Design decisions` already hold without change |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No existing doc carries stale `send-origin-validation-state` syntax |

## Design presentation

### What changes

Ze enforces both halves of the RFC 8097 Section 2 eBGP boundary by default and gives operators independent directional overrides on trusted eBGP sessions. Inbound eBGP dropping (`RFC8097-2-4`) and `session { accept-origin-validation-state }` (default `false`) are already in HEAD; this change adds the matching outbound default strip (`RFC8097-2-9`) and `session { send-origin-validation-state }` leaf (default `false`, `RFC8097-2-10`) in `ze-bgp-conf.yang`, strips `0x43/0x00` in place inside `Session.writeUpdateBody` / `normalizeTransmittedUpdateWith` across every egress rail (forwarded, RS fast path, originated, policy-added, and raw), adds end-to-end functional (`.ci`) and GoBGP Docker interop proofs for both directions, and closes the open journal row.

### Before and after

1. **Relaying or originating a route carrying `0x43/0x00` toward a default eBGP peer:**
   - **Before:** Ze transmits `EXTENDED_COMMUNITIES` carrying `origin-validation-state:valid` (`0x4300000000000000`) unchanged to the eBGP peer across every egress rail.
   - **After:** `Session.writeUpdateBody` strips every `0x43/0x00` entry in place in the session's private `writeBuf` before Adj-RIB-Out recording and socket write. Sibling extended communities (such as `target:65000:9`) survive with header flags preserved; if `0x43/0x00` was the sole extended community, the entire attribute is omitted and `Total Path Attribute Length` shrinks accordingly.

2. **Sending `0x43/0x00` across a trusted eBGP boundary (common administration or IXP route server):**
   - **Before:** No YANG leaf exists to control outbound `0x43/0x00` emission to eBGP peers.
   - **After:** Configuring `session { send-origin-validation-state true; }` on the peer or group (independently of `session { accept-origin-validation-state }`) preserves `0x43/0x00` on egress to that eBGP peer with its 5-octet Reserved field `[2..6]` zeroed, while iBGP peers continue to receive `0x43/0x00` by default and sibling default eBGP peers in the same fan-out still have it stripped.

3. **Receiving `0x43/0x00` from an eBGP peer (end-to-end daemon proof):**
   - **Before:** Unit tests prove `Session.sanitizeOriginValidationState`, but no `.ci` functional test or Docker interop scenario proves the full eBGP-to-iBGP relay and `plan/journal/scoped-attribute-relayed-past-its-boundary.md` stays open.
   - **After:** `test/plugin/rpki-ov-community-ebgp-boundary.ci` and `bgp-rpki-ov-ebgp-boundary-gobgp` prove both inbound and outbound defaults and overrides through the real daemon, and the open journal row is closed.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Enforce egress stripping per rail (`applyFacts*` on `forwardUpdateCore` and `reactorForwardRS` plus each static/API/RIB origination builder, like `applyFactsPrefixSID`) | Duplicates logic across four+ rails, risks rail drift (as happened historically with `LOCAL_PREF`), and still misses raw writes or export policy additions. `Session.writeUpdateBody` -> `normalizeTransmittedUpdateWith` already runs on every transmitted UPDATE's private `writeBuf` copy and already performs in-place sub-attribute compaction (`compactLargeCommunities`), giving single-site, zero-allocation coverage across all rails without mutating shared fan-out buffers. |
| Single bidirectional YANG leaf (e.g., `origin-validation-state true`) instead of separate `accept-origin-validation-state` and `send-origin-validation-state` | `accept-origin-validation-state` is already shipped and published in `ze-bgp-conf.yang` (commit `2167203bdc`), and real deployments are asymmetric: an IXP route server sends `0x43/0x00` to clients without accepting forged states from them, while a route-server client accepts it without re-advertising it to unrelated eBGP peers (RFC 8097 Section 2). Separate directional leaves also mirror `accept-srv6-prefix-sid` / `propagate-srv6-prefix-sid` immediately above in `ze-bgp-conf.yang`. |
| Name the new leaf `propagate-origin-validation-state` instead of `send-origin-validation-state` | RFC 8097 Section 2 uses the exact verb pair "send or accept" (and Thomas's 2026-11-24 decision uses "accept/send"), and the leaf governs both locally originated/policy-tagged announcements and relayed routes rather than propagation alone. |
| Strip all non-transitive extended communities (`TypeHigh & 0x40 != 0`) at eBGP egress under RFC 4360 Section 6 | Wider scope than this defect fix; risks breaking non-RFC-8097 non-transitive opaque or FlowSpec extended communities that operators expect to cross eBGP peers unless explicitly scoped and tested. |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add `send-origin-validation-state` YANG leaf, `PeerSettings.SendOriginValidationState`, `parsePeerFromTree` wiring (via `configvalue.Bool` from `internal/core/configvalue`), `originValidationStateSendAllowedHeld` stub/predicate, and write failing wiring/unit tests.
   - Tests: `TestSchemaSendOriginValidationStateDefaultsFalse`, `TestPeerSettingsEqualDetectsEachSignificantField` (`SendOriginValidationState`), `TestRFC8097EgressEBGPStripsOriginValidationByDefault`, `TestRFC8097EgressIBGPAndConfiguredEBGPRetainOriginValidation`
   - Files: `internal/component/bgp/yang/ze-bgp-conf.yang`, `internal/component/bgp/yang/schema_test.go`, `internal/component/bgp/reactor/peer_settings.go`, `internal/component/bgp/reactor/config.go`, `internal/component/bgp/reactor/session_origin_validation.go`, `internal/component/bgp/reactor/peer_settings_reload_test.go`, `internal/component/bgp/config/dynamic_group_parity_test.go`
   - Verify: YANG schema, config parsing, reload diff, and dynamic-group parity tests pass; egress strip tests fail red because `normalizeTransmittedUpdateWith` does not strip `0x43/0x00` yet.
2. **Phase: Egress strip & normalization** -- implement `normalizeTransmittedUpdateWith` in `session_egress_normalize.go`, wire it in `session_write.go:884`, and align `sessionIsEBGP()` fallback in `session_origin_validation.go`.
   - Tests: `TestRFC8097EgressEBGPStripsOriginValidationByDefault`, `TestRFC8097EgressIBGPAndConfiguredEBGPRetainOriginValidation`, and all existing `rfc4271_attr_flags_egress_test.go`, `rfc8092_large_community_egress_test.go`, `rfc8277_withdrawal_compatibility_test.go`, `rfc8669_label_index_family_test.go` tests.
   - Files: `internal/component/bgp/reactor/session_egress_normalize.go`, `internal/component/bgp/reactor/session_write.go`, `internal/component/bgp/reactor/session_origin_validation.go`, `internal/component/bgp/reactor/rfc8097_origin_validation_state_test.go`
   - Verify: all reactor unit tests pass with `-race` and zero allocations in the non-strip path; `./le go lint run scope "./internal/component/bgp/..."` passes.
3. **Phase: Functional `.ci`, GoBGP interop, docs & journal closure** -- author `test/plugin/rpki-ov-community-ebgp-boundary.ci` (drafted outside `test/` and promoted per `/ze-test`), author `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/` + checker (proving red when Ze's strip is reverted), regenerate published YANG goldens, update `docs/architecture/wire/attributes.md`, `docs/architecture/bgp/egress-attribute-rules.md`, `rfc/short/rfc8097.md`, and close the `2026-10-30` row in `plan/journal/scoped-attribute-relayed-past-its-boundary.md`.
   - Tests: `./le test plugin rpki-ov-community-ebgp-boundary`, `INTEROP_SCENARIO=bgp-rpki-ov-ebgp-boundary-gobgp ./le test integration interop`, `./le rfc check`, `./le doc check all`, `./le config claims`, `./le test unit pkg path ./internal/le/site/`
   - Verify: functional test, GoBGP interop test, RFC check, doc check, and published-schema golden tests all pass.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-5 has an implementation at file:line and a passing test |
| Correctness | Only type `0x43` sub-type `0x00` is dropped/stripped, only on eBGP peers without the matching directional opt-in leaf (`accept-origin-validation-state` inbound, `send-origin-validation-state` outbound); sibling extended communities, `FlagPartial`, `FlagExtLength`, trailing attributes, and NLRI survive intact |
| Loop & framing safety | When `kept == 0` removes the entire `AttrExtCommunity` attribute, `pos` does not advance past the immediately following attribute, `Total Path Attribute Length` shrinks by `header + length`, and malformed `length % 8 != 0` attributes are left untouched |
| Buffer isolation | `normalizeTransmittedUpdateWith` mutates only `s.writeBuf`; shared fan-out/cache/dedup buffers are never touched |
| Interop vacuity | `bgp-rpki-ov-ebgp-boundary-gobgp` fails when Ze's egress strip is reverted (not masked by GoBGP stripping `0x43/0x00` on eBGP receive) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG leaf `send-origin-validation-state` (default `false`) + published goldens | `./le test unit pkg path ./internal/component/bgp/yang/ ./internal/le/site/` and `./le config claims` |
| Egress eBGP default strip & opt-in preservation + reload/dynamic parity | `./le test unit pkg path ./internal/component/bgp/reactor/ ./internal/component/bgp/config/` |
| End-to-end functional test | `./le test plugin rpki-ov-community-ebgp-boundary` |
| GoBGP interop test (with counterfactual red verified) | `INTEROP_SCENARIO=bgp-rpki-ov-ebgp-boundary-gobgp ./le test integration interop` |
| RFC 8097 conformance & docs anchors | `./le rfc check` and `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Inbound spoofing defense | An untrusted eBGP peer cannot inject RFC 8097 origin validation state into our AS by default (`AcceptOriginValidationState` defaults `false`, dropped before RIB/policy/logging) |
| Outbound information leakage | Internal RPKI origin validation state extended communities do not leak to untrusted eBGP peers by default (`SendOriginValidationState` defaults `false`, stripped at the final `writeUpdateBody` chokepoint across every rail, including raw/policy-injected routes) |
| Fail-closed reload | Changing either `accept-origin-validation-state` or `send-origin-validation-state` restarts the peer via `peerSettingsEqual` so stale boundary permissions cannot linger |

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

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `scoped-attribute-relayed-past-its-boundary.md` | 2026-10-30 | EBGP-received type 0x43 origin validation state community relayed to IBGP peers |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Strip `0x43/0x00` in `Session.writeUpdateBody` -> `normalizeTransmittedUpdateWith` | Per-rail `applyFacts*` across forward/RS/origination builders | Single zero-allocation site operating on `s.writeBuf` covers every egress rail, preserves shared buffers, and keeps Adj-RIB-Out and `DirectionSent` exact |
| Separate boolean leaf `session > send-origin-validation-state` (default `false`) beside `accept-origin-validation-state` | Single bidirectional leaf; `propagate-origin-validation-state` | `accept-origin-validation-state` is already shipped; IXP route servers and clients require asymmetric inbound/outbound trust; matches RFC 8097 Section 2 ("send or accept") and owner decision ("accept/send") |

## Known Limitations
- RFC 8097 automatic generation/attachment from `bgp-rpki`'s local ROA validation state (`RFC8097-2-6`) and deriving local RPKI validation state from a received `0x43/0x00` community (`RFC8097-2-7`) are optional SHOULD features in `rfc/short/rfc8097.md`, outside this boundary-enforcement fix.
- General RFC 4360 Section 6 stripping of non-transitive extended communities other than `0x43/0x00` is intentionally outside this fix to avoid breaking unrelated opaque or FlowSpec extended communities.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

Treat the RFC as the required baseline, not a ceiling on explicit operator
control. Assess compliance on the default configuration path. Preserve explicit
options for more specific behavior and extensions, including behavior that differs from
the RFC. Their availability does not make a compliant default path fail.
Name the configuration covered by each claim and test both paths. RFC work does
not remove existing features to improve a conformance figure.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 8097 Section 2 (`RFC8097-2-4`) | `session accept-origin-validation-state` defaults `false`; `Session.sanitizeOriginValidationState` drops every `0x43/0x00` received from an eBGP peer without processing or logging | `session accept-origin-validation-state true` accepts and processes `0x43/0x00` from a trusted eBGP peer; iBGP always accepts | `TestRFC8097EBGPDropsOriginValidationByDefault`, `test/plugin/rpki-ov-community-ebgp-boundary.ci`, `bgp-rpki-ov-ebgp-boundary-gobgp` | `TestRFC8097EBGPDropsOriginValidationByDefault`, `test/plugin/rpki-ov-community-ebgp-boundary.ci`, `bgp-rpki-ov-ebgp-boundary-gobgp` |
| RFC 8097 Section 2 (`RFC8097-2-9`, `RFC8097-2-10`) | `session send-origin-validation-state` defaults `false`; `Session.writeUpdateBody` / `normalizeTransmittedUpdateWith` strips every `0x43/0x00` on egress to an eBGP peer (removing code 16 when sole) | `session send-origin-validation-state true` sends `0x43/0x00` (with Reserved `[2..6]` zeroed) to a trusted eBGP peer; iBGP always sends when present | `TestRFC8097EgressEBGPStripsOriginValidationByDefault`, `test/plugin/rpki-ov-community-ebgp-boundary.ci`, `bgp-rpki-ov-ebgp-boundary-gobgp` | `TestRFC8097EgressIBGPAndConfiguredEBGPRetainOriginValidation`, `test/plugin/rpki-ov-community-ebgp-boundary.ci`, `bgp-rpki-ov-ebgp-boundary-gobgp` |
| RFC 8097 Section 2 (`RFC8097-2-1`) | Reserved octets `[2..6]` zeroed on every transmitted `0x43/0x00` (including raw writes via `normalizeTransmittedUpdateWith`) | Non-`0x43/0x00` opaque extended communities (`0x43/0x01`, `0x43/0x0b`) keep all payload octets verbatim | `TestRFC8097ReservedFieldZeroOnSend`, `TestRFC8097EgressIBGPAndConfiguredEBGPRetainOriginValidation` | `TestRFC8097ReservedFieldZeroOnSend`, `TestRFC8097EgressEBGPStripsOriginValidationByDefault` |

## Implementation Summary

### What Was Implemented
- `internal/component/bgp/yang/ze-bgp-conf.yang`: added `leaf send-origin-validation-state` (`boolean`, default `false`) under `grouping peer-fields / container session` beside `accept-origin-validation-state`; refreshed `internal/le/site/testdata/published-configuration.md` and `published-yang-config-tree.json`.
- `internal/component/bgp/reactor/peer_settings.go` & `config.go`: added `SendOriginValidationState bool` to `PeerSettings`, parsed `session > send-origin-validation-state` via `configvalue.Bool` (`internal/core/configvalue`), verified restart-scoped reload in `peer_settings_reload_test.go`, and added dynamic listen-range group parity in `internal/component/bgp/config/dynamic_group_parity_test.go`.
- `internal/component/bgp/reactor/session_origin_validation.go`: added `originValidationStateSendAllowedTo`, `sessionIsEBGPFrom` (aligning fallback to `s.settings.IsEBGP()` for RFC 7705), `sessionIsEBGPHeld`, and `originValidationStateSendAllowedHeld` (safe under `writeMu` with zero lock-order inversion).
- `internal/component/bgp/reactor/session_egress_normalize.go` & `session_write.go`: added `normalizeTransmittedUpdateWith` and zero-alloc in-place `compactWithoutOriginValidationState` wired in `Session.writeUpdateBody` across all egress rails; strips `0x43/0x00` for default eBGP (omitting sole code 16 without skipping trailing attributes, preserving sibling RTs, `0x43/0x01`, `FlagPartial`, and `FlagExtLength`) and zeroes Reserved `[2..6]` on allowed sends without mutating shared fan-out buffers.
- `internal/component/bgp/reactor/rfc8097_origin_validation_state_test.go`: added `TestRFC8097EgressEBGPStripsOriginValidationByDefault` and `TestRFC8097EgressIBGPAndConfiguredEBGPRetainOriginValidation` (including zero-alloc `testing.AllocsPerRun` proof); re-sealed shifted `rfc/audit/rfc8097/RFC8097-2-{2,3,4,5}.json`.
- `test/plugin/rpki-ov-community-ebgp-boundary.ci` + `internal/test/fixture/plugin_fixture_09*.go`: 5-peer end-to-end daemon test covering inbound and outbound defaults and independent overrides (80/80 `stress-repro` pass under load, promoted live).
- `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/` + `internal/le/interoplab/bgp/check_rfc8097*.go`, `register_speaker.go`, `check_special.go`, `bgp_test.go`: GoBGP + 2 raw-wire eBGP speaker oracles (`rfc8097-ebgp-strip`, `rfc8097-ebgp-optin`) Docker interop scenario (`1 passed, 0 failed`).
- `docs/architecture/wire/attributes.md`, `docs/architecture/bgp/egress-attribute-rules.md`, `rfc/short/rfc8097.md`, `plan/journal/scoped-attribute-relayed-past-its-boundary.md`: documented RFC 8097 `0x43/0x00` boundary handling and closed the `2026-10-30` journal row.

### Bugs Found/Fixed
- Round 1 review caught `_ = json.Unmarshal(entry.Value, &ovState)` in `check_rfc8097.go` (could silently leave `ovState == 0`) and multi-ID `# RFC requirement:` tag syntax in `.ci` -> fixed with explicit error return + negative unit test (`"value":"valid"`) and single-ID tags; Round 2 CLEAN.
- Scoped lint caught 3 minor items (`normalizeTransmittedUpdate` unused `int` return, `rfc8097BGPConfigTreeWithSend` unused param + unchecked type assertions, 5 `stringConcatSimplify` in `check_rfc8097_test.go`) -> fixed cleanly.

### Documentation Updates
- `docs/architecture/wire/attributes.md` (`Attribute Flags`, Section 16 `EXTENDED_COMMUNITY`): updated with source anchors (`ste check: OK`).
- `docs/architecture/bgp/egress-attribute-rules.md`: added RFC 8097 eBGP boundary section and `## Proof` row (`ste check: OK`).
- `rfc/short/rfc8097.md`: updated `Meta` for `RFC8097-2-9` and `RFC8097-2-10` (`ste check: OK`).
- `internal/le/site/testdata/published-configuration.md` & `published-yang-config-tree.json`: updated with `send-origin-validation-state` (`TestTheConfiguration` PASS).

### Deviations from Plan
- Used two raw-wire native eBGP speaker oracles (`rfc8097-ebgp-strip` and `rfc8097-ebgp-optin`) alongside iBGP GoBGP (`ExtcommsJSON`) in `bgp-rpki-ov-ebgp-boundary-gobgp` to eliminate the GoBGP eBGP receive-strip vacuity trap completely.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | First `inject.msg` waited on an ordered `10.97.3.0/24` barrier and used off-subnet `198.51.100.1` NEXT_HOP | `ze-peer`'s `10.97.2.0/24` raced ahead on export, and one-hop eBGP rejects off-subnet NEXT_HOP per RFC 4271 §6.3 | Live Docker interop run | Set `export { final reject; }` on `ebgp-default-in`, `option=linger:value=true` in `inject.msg`, and base-network NEXT_HOP `172.30.0.9` (`AC1E0009`); scenario passed `1 passed, 0 failed` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| YANG `session > send-origin-validation-state` (default `false`) | Done | `internal/component/bgp/yang/ze-bgp-conf.yang:934-943` | Tested in `schema_test.go` + `internal/le/site/` |
| `PeerSettings.SendOriginValidationState` + `configvalue.Bool` + reload + dynamic parity | Done | `peer_settings.go`, `config.go`, `peer_settings_reload_test.go`, `dynamic_group_parity_test.go` | Independent of `accept-origin-validation-state` |
| Egress eBGP strip across all rails in `Session.writeUpdateBody` / `normalizeTransmittedUpdateWith` | Done | `session_origin_validation.go`, `session_egress_normalize.go`, `session_write.go` | Zero-alloc compaction; Reserved `[2..6]` zeroed on allowed sends |
| End-to-end `.ci` functional test | Done | `test/plugin/rpki-ov-community-ebgp-boundary.ci`, `plugin_fixture_09*.go` | 80/80 `stress-repro`, live PASS (`1.4s`) |
| GoBGP + raw-wire speaker Docker interop | Done | `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/`, `check_rfc8097*.go` | `1 passed, 0 failed` (`6m34s`) |
| Docs, RFC summary, re-sealed audits, journal closure | Done | `attributes.md`, `egress-attribute-rules.md`, `rfc8097.md`, `RFC8097-2-{2..5}.json`, `scoped-attribute-relayed-past-its-boundary.md` | All checked clean |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC8097EgressEBGPStripsOriginValidationByDefault`, `rpki-ov-community-ebgp-boundary.ci`, `bgp-rpki-ov-ebgp-boundary-gobgp` | Mixed eBGP strip across all rails preserving sibling RTs, `0x43/0x01`, flags, NLRI |
| AC-2 | Done | `TestRFC8097EgressEBGPStripsOriginValidationByDefault` | Sole `0x43/0x00` removes code 16 cleanly without skipping trailing attrs |
| AC-3 | Done | `TestRFC8097EgressIBGPAndConfiguredEBGPRetainOriginValidation`, `rpki-ov-community-ebgp-boundary.ci`, `bgp-rpki-ov-ebgp-boundary-gobgp` | iBGP & opted-in eBGP retain with Reserved `[2..6]` zeroed; fan-out buffer unmutated |
| AC-4 | Done | `TestSchemaSendOriginValidationStateDefaultsFalse`, `TestPeerSettingsEqualDetectsEachSignificantField`, `TestDynamicGroupParity`, `TestTheConfiguration` | YANG, `configvalue.Bool`, dynamic group parity, reload diff, site goldens all green |
| AC-5 | Done | `test/plugin/rpki-ov-community-ebgp-boundary.ci` (80/80 stress + live PASS), `bgp-rpki-ov-ebgp-boundary-gobgp` (`1 passed, 0 failed`) | Full daemon inbound + outbound defaults & overrides proven |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSchemaSendOriginValidationStateDefaultsFalse` | Done | `internal/component/bgp/yang/schema_test.go` | PASS |
| `TestPeerSettingsEqualDetectsEachSignificantField` | Done | `internal/component/bgp/reactor/peer_settings_reload_test.go` | PASS (20x `-race`) |
| `TestDynamicGroupParity` | Done | `internal/component/bgp/config/dynamic_group_parity_test.go` | PASS |
| `TestRFC8097EgressEBGPStripsOriginValidationByDefault` & `TestRFC8097EgressIBGPAndConfiguredEBGPRetainOriginValidation` | Done | `internal/component/bgp/reactor/rfc8097_origin_validation_state_test.go` | PASS (20x `-race`, 0 allocs) |
| `TestRequireGoBGPRPKIOVPathsPolarity` & `TestRFC8097WireOraclesAndReportPolarity` | Done | `internal/le/interoplab/bgp/check_rfc8097_test.go` | PASS |
| `rpki-ov-community-ebgp-boundary` | Done | `test/plugin/rpki-ov-community-ebgp-boundary.ci` | 80/80 stress + `1.4s` live PASS |
| `bgp-rpki-ov-ebgp-boundary-gobgp` | Done | `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/` | `1 passed, 0 failed` (`6m34s`) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All 19 modified + 10 created files | Done | All present and verified |

### Audit Summary
- **Total items:** 24
- **Done:** 24
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Default eBGP strips `0x43/0x00` on egress across all rails while preserving sibling communities and NLRI, and independent `send-origin-validation-state` opt-in preserves `0x43/0x00` with zeroed Reserved octets | Unit (20x `-race`), functional `.ci` (80/80 stress + live), and real GoBGP + wire-speaker Docker interop | `go test -race -count=20 -run 'TestRFC8097\|TestPeerSettingsEqualDetectsEachSignificantField' ./internal/component/bgp/reactor/` (`ok 11.043s`); `./le test bgp plugin --pattern rpki-ov-community-ebgp-boundary` (`PASS 1.4s`, 80/80 `stress-repro`); `INTEROP_SCENARIO=bgp-rpki-ov-ebgp-boundary-gobgp ./le test integration interop` (`1 passed, 0 failed`, `6m34s`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | All in-scope work completed | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-rpki-ov-community-ebgp-boundary-c6c30003-3333-4000-8000-000000000003.md` |
| `./le spec review check` | `review_gate: OK` |
| Rounds | 2 |
| Reviewer lenses used | lock ordering (`s.mu`->`s.writeMu`), zero-alloc compaction, YANG/config/reload/dynamic parity, RFC 8097 audit/discrimination freshness, `.ci` hygiene, GoBGP/raw-wire oracle non-vacuity, docs & journal |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `RFC8097-2-4` tags in `.ci` staled sealed MUST audit verdict and created unproven gated-MUST tag entries | `test/plugin/rpki-ov-community-ebgp-boundary.ci` | Converted inbound `RFC8097-2-4` comments to plain prose, kept clean single-ID `RFC8097-2-9`/`2-10` tags, re-sealed shifted `RFC8097-2-{2..5}.json` via `./le rfc reseal stem rfc8097` |
| 2 | ISSUE | `_ = json.Unmarshal(entry.Value, &ovState)` discarded error and risked vacuous `0` pass | `internal/le/interoplab/bgp/check_rfc8097.go:174` | Checked and returned unmarshal error + added negative unit test in `check_rfc8097_test.go` |
| 3 | ISSUE | Multi-ID tag lines dropped secondary IDs in `parseTagRest` | `test/plugin/rpki-ov-community-ebgp-boundary.ci:16,25` | Split to one requirement ID + polarity per line; Round 2 CLEAN |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/rpki-ov-community-ebgp-boundary.ci` | Yes | `-rw-rw-r-- 1 thomas thomas 10792 Nov 28 00:18 test/plugin/rpki-ov-community-ebgp-boundary.ci` |
| `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/*` | Yes | 7 files (`gobgp.toml`, `inject-args`, `inject.msg`, `speaker2-args`, `speaker-args`, `ze-peer.conf`, `ze.conf`) |
| `internal/le/interoplab/bgp/check_rfc8097.go` & `_test.go` | Yes | `14743B` & `6505B` |
| `internal/component/bgp/reactor/session_{origin_validation,egress_normalize,write}.go` | Yes | All present |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-3 | Egress strip, sole-attr removal, iBGP & opted-in retention, zeroed Reserved, 0 allocs | `go test -race -count=20 ./internal/component/bgp/reactor/` (`ok 11.043s`) |
| AC-4 | YANG leaf, `configvalue.Bool`, dynamic parity, reload, site goldens | `./le test unit pkg path ./internal/component/bgp/yang/ path ./internal/component/bgp/config/ path ./internal/le/site/` (`ok`), `./le config claims` (`OK`), `./le config coercion check` (`OK`) |
| AC-5 | Live `.ci` + GoBGP + wire-speaker Docker interop | `./le test bgp plugin --pattern rpki-ov-community-ebgp-boundary` (`PASS 1.4s`, 80/80 stress); `INTEROP_SCENARIO=bgp-rpki-ov-ebgp-boundary-gobgp ./le test integration interop` (`1 passed, 0 failed`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze-bgp-conf.yang` -> `PeersFromTree` -> `Session.ReadAndProcess` & `Session.writeUpdateBody` | `test/plugin/rpki-ov-community-ebgp-boundary.ci` + `test/interop/scenarios/bgp-rpki-ov-ebgp-boundary-gobgp/` | Yes (both PASS) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `Session.writeUpdateBody` -> `normalizeTransmittedUpdateWith` strips `0x43/0x00` on private `s.writeBuf` across all rails with zero shared-buffer mutation |
| A-2 | confirmed | `publishNegotiation` writes `s.negotiated` under both `s.mu` and `s.writeMu`; `sessionIsEBGPHeld()` reads safely under `writeMu` (`20x -race` clean) |
| A-3 | confirmed | `TestRFC8097WireOraclesAndReportPolarity` + `bgp-rpki-ov-ebgp-boundary-gobgp` native speakers inspect raw wire bytes directly without GoBGP eBGP receive-strip masking |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/wire/attributes.md` & `docs/architecture/bgp/egress-attribute-rules.md` | `session_origin_validation.go`, `session_egress_normalize.go`, `session_write.go` | Yes (`ste check: OK`) |
| `rfc/short/rfc8097.md` & `rfc/audit/rfc8097/RFC8097-2-{2..5}.json` | Re-sealed (`0 refused`) | Yes |
| `plan/journal/scoped-attribute-relayed-past-its-boundary.md` | Row length 511 chars (`<= 600`) | Yes |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [x] Metadata table present, with a valid Status, Depends, Phase and Updated
- [x] `ai/INDEX.md` keyword table checked
- [x] An `rfc/short/` summary exists for every RFC referenced
- [x] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [x] No code snippets
- [x] Files to Modify names feature code, not only tests
- [x] Current Behavior and Data Flow sections completed
- [x] AC-N rows carry testable assertions
- [x] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [x] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [x] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [x] AC-1..AC-5 all demonstrated
- [x] Every user story has a working path and a passing test
- [x] Wiring Test table complete: every row a concrete test name, none deferred
- [x] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [x] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [x] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [x] Architectural Verification table filled, including registration over hardcoding
- [x] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [x] Every A-N confirmed or broken, none `unvalidated`
- [x] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [x] Tests written
- [x] Tests FAIL (paste output)
- [x] Tests PASS (paste output)
- [x] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [x] Functional `.ci` tests for end-to-end behavior
- [x] Interop tests for protocol features (or N-A with a reason)

### Closure
- [x] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [x] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [x] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [x] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [x] **Commit B:** `remove plan/release/fixes/spec-bgp-rpki-ov-community-ebgp-boundary.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
