# 430 -- Peer Local/Remote Config Restructuring

## Context

BGP peer configuration used flat keys (`peer-as`, `local-as`, `local-address`) at peer level, and peers were keyed by IP address. This made tab-completion unhelpful (peer names invisible), semantics unclear (local vs remote not grouped), and the global `local-as` a flat leaf. The goal was to restructure into nested `local`/`remote` containers and key peers by human-readable name.

## Decisions

- Chose YANG containers (`container local`, `container remote`) over flat renamed leaves, because containers give editor autocomplete grouping for free.
- Peer list key changed from `address` (IP) to `name` (string), with IP moved to `remote > ip` (mandatory). Chose mandatory `remote > ip` over optional because every peer needs an address.
- Global `local-as` restructured to `local > as` container at bgp level, matching the per-peer pattern.
- Old flat keys (`peer-as`, `local-as`, `local-address`) fully removed over keeping them as aliases, per no-layering rule.
- AC-5 (unique `remote > ip` across peers) deferred because goyang does not support YANG `unique` constraint validation.

## Consequences

- All 300+ `.ci` functional tests updated to the new nested format in the same change.
- Plugin YANG augments on `/bgp:bgp/bgp:peer` remain valid since key type change is transparent to augments.
- `parsePeerFromTree` reads nested maps via `mapMap` helper, making future container additions straightforward.
- PeerSettings struct fields unchanged -- only the config-to-struct mapping changed.
- ExaBGP migration (`ze bgp config migrate`) must produce the new nested format.

## Gotchas

- Two separate `parsePeersFromTree` functions exist (config.go and reactor_api.go) -- both needed updating for the nested structure.
- `reactor_api.go` version is a test-fallback only; production uses the full config pipeline via `reloadFunc`.
- `local > ip` supports `"auto"` value for automatic interface detection -- this had to be preserved in the nested structure.

## Files

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- YANG restructuring
- `internal/component/bgp/reactor/config.go` -- parsePeerFromTree, PeersFromTree
- `internal/component/bgp/reactor/config_test.go` -- unit tests for nested structure
- `internal/component/bgp/reactor/reactor_api.go` -- API path parser
- `test/parse/local-remote-basic.ci` -- dedicated functional test
- 300+ `.ci` files updated for new config format
