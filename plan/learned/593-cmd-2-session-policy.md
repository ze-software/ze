# 593 -- Session Policy Knobs

## Context

Ze's BGP peer configuration lacked four session-level knobs that every production deployment needs: send-community control (filter which community types are sent to a peer), default-originate (originate 0.0.0.0/0 or ::/0 to a peer), local-AS modifiers (no-prepend, replace-as for migration scenarios), and AS-override (replace peer's ASN in forwarded AS_PATH). Without these, operators cannot control community leaking, inject default routes, or perform ASN migration.

## Decisions

- All four features as YANG leaves in the existing `peer-fields` grouping, inheritable at bgp/group/peer levels. No new containers needed.
- Send-community as leaf-list enum {standard, large, extended, all, none} over a boolean, because operators need granular per-type control.
- `AttrModSuppress` action added to the attribute modifier registry for community stripping, over inline deletion in the forwarding loop. Last-action-wins semantics in `genericAttrSetHandler`.
- Default-originate per-family (inside `session/family`) over per-peer, because IPv4 and IPv6 default routes are independent decisions.
- Default-originate-filter uses `PolicyFilterChain` dry-run with fail-closed semantics: missing reactor, missing API, or malformed filter ref all suppress the default route.
- Local-AS modifiers wired into the forwarding path's AS_PATH prepend logic (`reactor_api_forward.go:488-494`). OPEN message already uses `settings.LocalAS` (the override), so no OPEN changes were needed.
- AS-override implemented as `rewriteASPathOverride` two-pass (scan for match, copy+replace), respecting ASN4 negotiation state.

## Consequences

- Operators can now control community leaking per peer per type.
- Default route origination is conditional on named policy filters (fail-closed).
- ASN migration scenarios (local-AS with no-prepend/replace-as) work for both OPEN and forwarding.
- The `AttrModSuppress` action extends the modifier registry pattern for any future "remove attribute" needs.
- Wire-level .ci tests for send-community and as-override are blocked by the eBGP forwarding infrastructure gap (same as community-strip.ci). Config parsing .ci and encode tests exist.

## Gotchas

- The spec claimed local-AS OPEN modifiers and default-originate-filter were unimplemented, but both were found fully working during the 2026-04-14 audit. The OPEN already used the override ASN via `settings.LocalAS`, and `defaultOriginateFilterAccepts` was integrated into `sendDefaultOriginateRoutes`.
- `rewriteASPathOverride` does a two-pass over AS_PATH bytes. First pass scans for presence of the target ASN; second pass copies with replacement. This avoids allocating a modified copy when no replacement is needed.
- Send-community `none` suppresses types 8 (standard), 16 (extended), and 32 (large). The `all` default means no suppression. Community types are identified by BGP attribute type code, not by name.

## Files

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- as-override, local-options, community/send, default-originate leaves
- `internal/component/bgp/reactor/peersettings.go` -- ASOverride, LocalASNoPrepend, LocalASReplaceAS, SendCommunity, DefaultOriginate, DefaultOriginateFilter
- `internal/component/bgp/reactor/config.go` -- parsePeerFromTree, parseFamiliesFromTree extraction
- `internal/component/bgp/reactor/reactor_api_forward.go` -- applySendCommunityFilter, applyASOverride, local-AS dual-prepend logic
- `internal/component/bgp/reactor/peer_initial_sync.go` -- sendDefaultOriginateRoutes, defaultOriginateFilterAccepts
- `internal/component/plugin/registry/registry_bgp_filter.go` -- AttrModSuppress
- `test/parse/session-policy-config.ci`, `test/encode/default-originate.ci`
