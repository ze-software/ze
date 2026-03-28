# 466 -- set-with

## Context

The `set bgp peer <ip> with` command used a hardcoded flat key-value parser (`asn 65001 local-as 65000`) that duplicated the YANG peer-fields schema in Go code. Adding a new config key required both a YANG change and a matching Go switch case. The parser also only supported a subset of peer config (no families, capabilities, or prefix limits), meaning dynamically-created peers were less capable than config-file peers.

## Decisions

- Chose YANG-schema-driven parsing over hardcoded switch/map. `config.ParseInlineArgs` walks the YANG tree to consume tokens, so any valid YANG peer-field is automatically accepted without Go code changes. Rejected maintaining a parallel Go map because it diverges from the schema.
- Chose `AddDynamicPeer` (direct `parsePeerFromTree` + `AddPeer`) over `ApplyConfigDiff` because the config diff path re-reads from disk in production, ignoring in-memory tree modifications. Deep review caught this.
- Chose generic `HandleNodeWith` with `NodePrepare` + `NodeApply` callbacks over a peer-specific handler. The pattern is reusable for `set bgp group <name> with` or any future `set X with` command.
- Removed `DynamicPeerConfig` struct entirely. The YANG-parsed `map[string]any` tree is the interchange format, same as the config pipeline uses. No intermediate struct needed.
- Dropped old flat keys (`asn`, `local-as`, `local-ip`) without aliases. Ze has no users, no backward compatibility needed.

## Consequences

- Adding a new peer config key now requires: (1) add YANG leaf, (2) handle in `parsePeerFromTree` if not already. No parser changes needed.
- `ParseInlineArgs` validates leaf values against YANG types at parse time (uint32 range, IP format, etc.), catching errors before they reach the reactor.
- `HandleNodeWith` lives in `plugin/server/`, making it available to any command handler package. New `set X with` commands are a one-liner handler + prepare callback.
- The `consumeOneField` function handles all schema node types: Leaf, Container, List, Flex. Freeform and InlineList return explicit errors.

## Gotchas

- `ApplyConfigDiff` in production re-reads from disk via `reloadFn`, ignoring the provided tree. This was the critical finding from deep review. Using it for dynamic peer creation silently fails.
- `GetConfigTree` returns the reactor's live map reference (not a copy). Mutating it outside locks causes data races. The initial design mutated it in `HandleNodeWith` before deep review caught the race.
- YANG `type boolean` fields accept `enable`/`disable`/`true`/`false` in the config pipeline. The inline parser stores the raw string; `parsePeerFromTree` handles the mapping.
- The `tokenizeLine` function silently accepts unterminated quotes (pre-existing issue, not introduced here).

## Files

- `internal/component/plugin/server/node_with.go` (new) -- generic HandleNodeWith, ParseInlineArgsForSchema, GetSchemaNode
- `internal/component/config/setparser_inline.go` (new) -- ParseInlineArgs, consumeOneField, validateLeafValue
- `internal/component/bgp/plugins/cmd/peer/peer.go` -- HandleBgpPeerWith + preparePeerTree (was HandleBgpPeerAdd with hardcoded parser)
- `internal/component/plugin/types.go` -- removed DynamicPeerConfig, added AddDynamicPeer(addr, tree) to interface
- `internal/component/bgp/reactor/reactor_peers.go` -- AddDynamicPeer uses parsePeerFromTree directly
- `docs/guide/command-reference.md` -- config key reference table
