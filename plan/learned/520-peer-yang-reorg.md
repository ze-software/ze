# 520 -- Peer YANG Config Reorganization

## Context

The peer configuration in `ze-bgp-conf.yang` had a flat structure with ~20 leaves at peer level mixing transport, session, and operational concerns. This made the config hard to navigate and obscured which settings belonged together. The goal was to group related fields into logical containers before the first release locks the config schema.

## Decisions

- Split flat peer-fields into four containers: `connection` (transport: IPs, ports, MD5, TTL, link-local bool), `session` (BGP: ASN, router-id, families, capabilities, add-path, link-local IPv6), `behaviour` (operational knobs: group-updates, manual-eor, auto-flush), and `rib` (adj-rib-in/out moved from flat leaves to `rib > adj > in/out`).
- Split single `port` leaf into `connection > local > port` and `connection > remote > port` over keeping a single leaf, because local bind port and remote connection port are distinct concepts.
- Split `link-local` into boolean (`connection > link-local`) for TCP auto-discovery and IPv6 address (`session > link-local`) for MP_REACH_NLRI encoding (RFC 2545), over keeping a single overloaded leaf.
- Moved prefix enforcement (`teardown`, `idle-timeout`, `updated`) per-family into `session > family > prefix` over keeping peer-level, because different families may have different prefix limits.
- Renamed TTL leaves: `ttl-security` to `ttl > max`, `outgoing-ttl` to `ttl > set`, `incoming-ttl` to `ttl > min`.
- Built `ze config migrate` tool using the config engine for old-to-new format conversion over manual sed/awk scripts.
- No backwards compatibility -- ze has no releases, so old syntax is rejected by parser.

## Consequences

- All plugin YANG augments now target `bgp:session/bgp:capability` (three paths each: standalone peer, grouped peer, group). Any new plugin augmenting peer capabilities must use this path.
- Config deep-merge in `ResolveBGPTree` is container-name-agnostic, so the new nesting depth works without merge logic changes.
- ExaBGP migration (`internal/exabgp/migration/migrate.go`) targets the new paths. Future ExaBGP field mappings must use the nested structure.
- Every `.ci` test file with peer config was updated to the new syntax.
- The `ze config migrate` tool establishes the pattern for future config format migrations.

## Gotchas

- Plugin YANG files have three augment paths each (standalone peer, grouped peer, group) -- easy to update only one and miss the others.
- `parsePeerFromTree` extracts `connection`, `session`, and `behavior` containers from the tree at the top, then drills into each. Adding new peer fields requires knowing which container they belong to.
- The YANG uses `behavior` (American spelling) not `behaviour` (British) -- match the YANG leaf name in Go code.

## Files

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- restructured peer-fields grouping
- `internal/component/bgp/plugins/gr/schema/ze-graceful-restart.yang` -- updated augment paths
- `internal/component/bgp/plugins/hostname/schema/ze-hostname.yang` -- updated augment paths
- `internal/component/bgp/plugins/softver/schema/ze-softver.yang` -- updated augment paths
- `internal/component/bgp/plugins/llnh/schema/ze-link-local-nexthop.yang` -- updated augment paths
- `internal/component/bgp/config/resolve.go` -- updated merge targets
- `internal/component/bgp/reactor/config.go` -- parsePeerFromTree with new container paths
- `internal/exabgp/migration/migrate.go` -- updated field target paths
- `cmd/ze/config/cmd_migrate.go` -- migration tool CLI
