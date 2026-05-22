# 762 -- IXP Route Server: Dynamic Peers, RS-Client, Community Filtering

## Context

Ze had all the building blocks for an IXP route server (RS plugin, passive mode, 3-layer config inheritance, RFC 9234 roles) but required explicit per-peer configuration. IXPs with hundreds of peers need dynamic peer creation from prefix ranges, transparent AS-path forwarding (RFC 7947), and community-based selective forwarding (Euro-IX convention). The gap was "create peer at connection time" rather than "create peer at config time."

## Decisions

- Chose `ip dynamic` as an enum value in the existing `connection/remote/ip` union over a separate `dynamic-mode` leaf, keeping the YANG schema flat and the config syntax natural (`ip dynamic; range 185.1.69.0/24`).
- Chose longest-prefix-match for overlapping dynamic group ranges over first-match or reject-overlap, matching standard routing table semantics.
- Chose to bypass `parsePeerFromTree` for dynamic peers (it hard-rejects PeerAS=0 and empty IP) and build PeerSettings directly from the template via `buildDynamicGroupSettings`, over modifying parsePeerFromTree to accept optional fields.
- Chose inline `resolveFilterVars` in the reactor package over importing `bgp/config`, because the import cycle (reactor -> config -> reactor) is architecturally forbidden. The duplication is three `strings.ReplaceAll` calls.
- Chose `GlobalLocalAS` (router's real ASN) for community matching over `LocalAS` (per-peer override). The RS community scheme (`RS-ASN:peer-ASN`) must use the actual RS identity, not a per-peer local-as override.
- Chose to store original unresolved filters on the Peer struct (`dynImportFilters`/`dynExportFilters`) over re-reading from the template, so reconnections resolve from pristine originals even if the template was replaced by a config reload.
- Chose `clock.AfterFunc` for idle cleanup timers over a goroutine with select, avoiding nolint directives and matching the existing clock abstraction.
- Community parsing placed in `wireu/` (shared) over `rs/` (plugin-specific), because both the reactor fast path and the RS plugin slow path need it.

## Consequences

- `DynamicGroupConfig` on the reactor holds runtime state (`ActivePeers` atomic counter). On reload, `SetDynamicGroups` must transfer surviving peer counts to new group objects (new groups start at zero).
- `resolveDynamicPeerSettings` runs at every Established transition (not just first), so filter chains are re-resolved if the remote ASN changes on reconnection.
- RS-client peers skip AS-path prepending in TWO places: `forward_rs.go` (fast path) and `reactor_api_forward.go` (standard path). Both must be kept in sync.
- Community stripping via `AttrModRemove` on attribute code 8 reuses the existing egress attribute modification framework. No new wire rewriting code was needed.
- Standard communities can only encode 16-bit ASNs. For RS ASNs > 65535, community-based filtering requires LARGE_COMMUNITY. This is documented but not enforced at config validation time.

## Gotchas

- **Map modification during iteration**: `SetDynamicGroups` originally iterated `r.peers` while calling `removeDynamicPeer` which deletes from the same map. Go map iteration during modification is undefined. Fixed by collecting peers into a `toRemove` slice first.
- **ActivePeers counter goes negative on reload**: when `SetDynamicGroups` replaces group objects, `removeDynamicPeer` would decrement the NEW group's counter (which starts at zero) for peers created under the OLD group. Fixed by removing peers before replacing groups, then counting survivors onto new groups.
- **`PeersFromConfigTree` prunes inactive nodes**: the reload path must call `ResolveBGPTree` AFTER `PeersFromConfigTree` returns (post-pruning), not before. Otherwise dynamic groups inside `inactive` blocks would be loaded.
- **Import cycle**: reactor cannot import `bgp/config`. Variable resolution and community parsing had to be either inlined or placed in shared packages (`wireu/`).
- **`connect false` validation**: YANG default for `connect` is `true`. When the key is absent from the config map, it means `true`. The validation must check for absence, not just for `"true"`.

## Files

- `internal/component/bgp/reactor/reactor_dynamic.go` -- dynamic group matching, peer creation/lifecycle/cleanup
- `internal/component/bgp/reactor/peersettings.go` -- IsDynamic, RSClient fields
- `internal/component/bgp/reactor/reactor_connection.go` -- dynamic peer creation on unknown IP
- `internal/component/bgp/reactor/peer_run.go` -- variable resolution at Established
- `internal/component/bgp/reactor/forward_rs.go` -- RS-client AS-path skip, community filtering
- `internal/component/bgp/reactor/reactor_api_forward.go` -- same for standard path
- `internal/component/bgp/wireu/community.go` -- zero-copy community parsing
- `internal/component/bgp/config/resolve.go` -- dynamic group detection, DynamicGroupTemplate
- `internal/component/bgp/config/peers.go` -- DynamicGroupsFromTree, buildDynamicGroupSettings
- `internal/component/bgp/config/variables.go` -- variable substitution engine
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- dynamic enum, range, max-peers, rs-client
