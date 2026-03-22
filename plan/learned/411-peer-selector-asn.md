# 411 -- Peer Selector ASN + Dynamic Completion

## Objective

Add ASN-based peer selection (`as<N>` format) to CLI commands and dynamic peer selector tab-completion for `ze show`/`ze run`.

## Decisions

- `as<N>` format is case-insensitive (`as65001`, `AS65001`, `As65001` all work), consistent with `peer` keyword which is also case-insensitive
- ASN matching returns all peers with that ASN (glob behavior), not unique-only
- `ze completion peers` is a separate subcommand from `ze completion words` (static YANG tree vs daemon-dependent data)
- Shell scripts merge both completions at the `peer` selector position
- ASN dedup in completion output: first peer by sorted IP wins the `as<N>` description
- Resolution priority: exact key > bare IP+port > name/IP > ASN > glob (ASN between name and glob)

## Patterns

- Selector recognition (dispatcher) and resolution (reactor) are separate concerns in separate packages, using equivalent but independent parsing logic
- `filterPeersBySelector` in `plugins/cmd/peer/peer.go` is a third independent selector resolver operating on `PeerInfo` (API-level structs), not on reactor peers directly
- Shell completion scripts are Go string literals in `bash.go`, `zsh.go`, `fish.go`, `nushell.go` -- dynamic data requires calling back to `ze` at tab time
- Zsh arrays are 1-indexed (`${path_words[1]}`), bash arrays are 0-indexed (`${path_words[0]}`)

## Gotchas

- **Three independent selector resolvers**: `getMatchingPeersLocked` (reactor), `filterPeersBySelector` (peer command handlers), and `SoftClearPeer` (route refresh). Adding a new selector format requires updating all three. The deep review caught that `filterPeersBySelector` was missed -- it would have silently returned empty results for `peer as65001 list`. This is the "feature not wired" pattern again.
- `SoftClearPeer` uses `ipGlobMatch` directly and still does not support name or ASN selectors. Pre-existing but now more visible. Guarded at handler level by `netip.ParseAddr` check.
- Duplicated ASN parsing logic between dispatcher (`command.go`) and reactor (`reactor_api.go`) cannot be shared due to package boundaries. Both must be kept in sync.

## Files

- `internal/component/plugin/server/command.go` -- `looksLikeASNSelector()`, wired into `Dispatch()`
- `internal/component/bgp/reactor/reactor_api.go` -- ASN branch in `getMatchingPeersLocked()`
- `internal/component/bgp/plugins/cmd/peer/peer.go` -- ASN branch in `filterPeersBySelector()`
- `cmd/ze/completion/peers.go` -- `ze completion peers` subcommand
- `cmd/ze/completion/bash.go`, `zsh.go`, `fish.go`, `nushell.go` -- shell script peer selector integration
- `test/plugin/peer-selector-asn.ci` -- functional test
