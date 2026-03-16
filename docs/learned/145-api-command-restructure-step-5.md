# 145 — API Command Restructure Step 5: BGP Command Migration

## Objective

Move all BGP-related commands under the `bgp` namespace and update the dispatcher to parse `bgp peer <sel>` as the standard peer selector pattern.

## Decisions

- `bgp list` and `bgp show` (flat) were consolidated into `bgp peer * list` and `bgp peer * show` — user feedback found the flat forms redundant and inconsistent.
- `neighbor` prefix support removed entirely; only `bgp peer` is valid. No backward compatibility.
- `bgp raw` in the spec became `bgp peer <sel> raw` in implementation — raw messages require a destination peer, so a peer-scoped path is more correct.

## Patterns

- Dispatcher extracts peer selector from `bgp peer <sel> <command>` by detecting an IP or glob at tokens[2], then reconstructs the command as `bgp peer <command>` for registration lookup.

## Gotchas

- Several new handlers (`bgp daemon restart`, `bgp peer tcp reset`, `bgp peer tcp ttl`) were deferred because they require ReactorInterface additions (`RestartBGP`, `ResetTCP`, `SetPeerTTL`) that were not yet implemented. Plan showed them as new commands; implementation skipped them with explicit deferred list.

## Files

- `internal/component/plugin/command.go` — dispatcher updated for `bgp peer <sel>` prefix extraction
- `internal/component/plugin/handler.go`, `route.go`, `commit.go`, `raw.go`, `refresh.go` — handlers renamed and re-registered
