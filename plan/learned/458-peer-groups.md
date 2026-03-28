# 458 -- Peer Groups

## Objective

Replace ExaBGP-style template/inherit config model with Junos-style peer-groups, giving peers 3-level inheritance (bgp globals, group defaults, peer overrides) and optional human-readable names usable as CLI selectors.

## Decisions

- **Groups, not templates:** Peers live inside named groups (`bgp { group X { peer Y { } } }`). Groups define shared defaults; peer values override. No glob auto-matching, no `inherit` directive. Standalone peers (directly under `bgp`) remain supported for simple configs.
- **3 YANG augment targets per plugin capability:** Each plugin YANG must augment standalone peer, peer-inside-group, AND group-level. Forgetting group-level means the capability can't be set as a group default.
- **Flat output from ResolveBGPTree:** The resolved map is still `{"peer": {"ip": {...}}}` -- the reactor never sees groups. Group name is injected as `"group-name"` key in each resolved peer map.
- **Plugins receive raw JSON (not resolved):** Config JSON delivered at Stage 2 has `group` containers intact. Each plugin must handle both `bgp.peer` and `bgp.group.<name>.peer` paths. Shared `configjson.ForEachPeer` helper centralizes this traversal.
- **Reserved peer names validated at config time:** Names like "list", "detail", "add" collide with `bgp peer <subcommand>` dispatch. A hardcoded set in resolve.go is verified against registered RPCs by `TestReservedPeerNamesSyncWithRPCs`.
- **Peer name validation is ASCII-only:** `unicode.IsLetter` was rejected because it accepts CJK/accented chars. Only `[a-zA-Z0-9_-]` allowed, max 255 chars, must not parse as IP.

## Patterns

- **configjson.ForEachPeer:** All plugins that read config JSON use this shared helper. It visits standalone peers (groupMap=nil) and grouped peers (groupMap=enclosing group). Each plugin checks peer-level first, group-level as fallback. Pattern: `peerCfg := parse(peerMap); groupCfg := parse(groupMap); use := groupCfg; if peerCfg != nil { use = peerCfg }`.
- **Deep merge for containers, override for leaves:** `deepMergeMaps` recurses into map values; scalar values overwrite. This means group `capability.GR` + peer `capability.hostname` = both present. Group `hold-time 180` + peer `hold-time 90` = 90 wins.
- **Migration produces groups from templates:** Ze-native migration converts `template.group X` to `bgp.group X`, moves peers with `inherit X` inside, assigns ungrouped peers to `default` group. ExaBGP migration groups peers by their template name.
- **Dynamic completion with TTL cache:** `fetchPeerSelectors` queries the daemon for peer names/IPs on tab completion. Results cached for 3 seconds to avoid per-keystroke queries.

## Gotchas

- **YANG `mandatory true` on `peer-as` and `local-address` had to be removed:** Groups can set these as defaults, so they're not mandatory per-peer in YANG. Go code validates instead.
- **Validator deep merge is only one level deep:** `mergeGroupDefaults` in validator.go merges containers at the first level (e.g., capability) but sub-containers (e.g., capability.graceful-restart) are replaced, not merged. This is acceptable because the runtime `deepMergeMaps` handles it correctly; the validator only misses validation of group values overridden by peer values (which wouldn't be used anyway).
- **Watchdog plugin needed both-levels processing, not fallback:** Initial fix used fallback-only (check group only if peer has no updates). Correct fix processes both layers: group updates first, then peer updates. Same route key at both levels logs a duplicate warning via `AddRoute`.
- **`hasTemplateBlock` belongs in detect.go:** Spec said detect.go but initial implementation put it in migrate.go. Detection functions should live with their peers for discoverability.

## Files

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- group list, standalone peer with name
- `internal/component/bgp/config/resolve.go` -- ResolveBGPTree rewritten for groups
- `internal/component/bgp/config/peers.go` -- route extraction from group+peer layers
- `internal/component/bgp/configjson/traverse.go` -- new: shared ForEachPeer for plugins
- `internal/component/bgp/reactor/peersettings.go` -- Name, GroupName fields
- `internal/component/plugin/server/command.go` -- isKnownPeerName dispatch
- `internal/component/plugin/server/rpc_register.go` -- PeerSubcommandKeywords
- `internal/component/bgp/plugins/{gr,hostname,llnh,role,softver,watchdog}/` -- group-aware config
- `internal/component/cli/validator.go` -- deep merge for group validation
- `internal/component/config/migration/migrate.go` -- template-to-group migration
- `internal/exabgp/migration/migrate.go` -- ExaBGP inherit-to-group migration
- `docs/architecture/config/syntax.md` -- rewritten for peer-groups
- 90+ config/test files updated from template to group syntax
