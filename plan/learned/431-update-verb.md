# 431 -- Update Verb

## Context

BGP peer prefix update was buried at `ze bgp peer * prefix update` -- a verb hiding inside a noun tree. This made it hard to discover and inconsistent with the emerging verb-first CLI taxonomy. The goal was to promote `update` to a top-level verb (`ze update bgp peer * prefix`) and establish the pattern for all "refresh stale data" commands (PeeringDB, RPKI, IRR).

## Decisions

- Created `internal/component/cmd/update/` as a top-level verb package, following the pattern already established by `cmd/set/` and `cmd/del/`.
- YANG module named `ze-cli-update-cmd.yang` (not `ze-update-cmd.yang`) to follow the `ze-cli-*-cmd.yang` convention used by other verb packages.
- Handler code stayed in `plugins/cmd/peer/prefix_update.go` (proximity: the logic is BGP-peer specific). Only the RPC registration moved to `cmd/update/update.go`.
- Wire method changed from `ze-bgp:peer-prefix-update` to `ze-update:bgp-peer-prefix` -- the `ze-update:` prefix groups all update RPCs.
- No change to `reservedPeerNames` in resolve.go -- the spec originally planned to remove "prefix" but it was never in the reserved list.

## Consequences

- `update` is now a first-class verb in the CLI taxonomy alongside `show`, `set`, `del`.
- Future refresh commands (RPKI, IRR) add sibling containers under the `update` YANG module and RPCs with `ze-update:` prefix.
- The `extractPeerSelector` dispatcher already scans all positions for `peer <selector>`, so all verb-first paths work without further dispatcher changes.
- Design Insights section in the spec documents the full verb-first migration table for remaining commands.

## Gotchas

- The spec was written predicting RPC count 14->13, but parallel specs (set/del) moved other RPCs simultaneously, landing at count 11. Always check actual counts, not spec predictions.
- "prefix" was never a reserved peer name despite being a peer subcommand in the old YANG. The reserved list tracks dispatch-ambiguous names, and "prefix" was nested under a container, not directly under "peer".

## Files

- `internal/component/cmd/update/` -- new package (update.go, doc.go, update_test.go, schema/)
- `internal/component/bgp/plugins/cmd/peer/prefix_update.go` -- handler (unchanged, init() removed)
- `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` -- prefix/update block removed
- `test/plugin/api-peer-prefix-update.ci` -- dispatch path updated
- `docs/guide/command-reference.md`, `docs/features.md`, `docs/guide/configuration.md` -- updated
