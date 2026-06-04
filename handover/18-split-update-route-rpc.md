# Handover: Split update-route RPC into route injection and peer actions

## Problem

`ze-plugin-engine:update-route` handles two unrelated concerns through one RPC:

1. **Route injection**: `peer <sel> announce/withdraw/update text ...`
2. **Peer lifecycle actions**: `peer <sel> teardown/pause/resume/flush/refresh/borr/eorr/clear soft`

The peer-prepend logic at `dispatch.go:147-151` stitches these together by guessing
whether a command is peer-scoped based on `HasCommandPrefix`. This coupling blocks
the verb-first migration for peer actions: moving `peer <sel> teardown` to
`request peer <sel> teardown` breaks the prepend heuristic.

## Current Flow

```
Plugin sends update-route RPC
  -> handleUpdateRouteRPC (dispatch.go:117)
    -> input.Command = "teardown" or "update text announce route ..."
    -> HasCommandPrefix(command)?
       yes -> pass through (top-level command like "cache ..." or "commit ...")
       no  -> prepend "peer <sel> " -> dispatch "peer upstream1 teardown"
```

Both route injection and peer actions flow through the same RPC, the same
prepend heuristic, and the same dispatch path.

## Proposed Split

### 1. Keep `update-route` for route injection only

Route injection commands always need `peer <sel>` prepended. The RPC name
already says "update-route". After the split, this RPC only handles:
- `announce route ...`
- `withdraw route ...`
- `update text ...`
- `update hex ...`

The prepend logic becomes unconditional: always prepend `peer <sel>`.

### 2. New `dispatch-command` for peer actions (or use existing)

`ze-plugin-engine:dispatch-command` already exists and dispatches arbitrary
commands through the YANG tree. Peer actions can use it directly:

```python
dispatch(api, 'request peer upstream1 teardown')
```

No special prepend logic needed. The command is the full dispatch key.

### 3. Migrate peer action YANG

Once `update-route` no longer routes peer actions, move the YANG containers:

| Current | Target |
|---------|--------|
| `peer <sel> teardown` | `request peer <sel> teardown` |
| `peer <sel> pause` | `request peer <sel> pause` |
| `peer <sel> resume` | `request peer <sel> resume` |
| `peer <sel> flush` | `request peer <sel> flush` |
| `peer <sel> plugin session ready` | `request peer <sel> plugin session ready` |
| `peer <sel> refresh` | `request peer <sel> refresh` |
| `peer <sel> borr` | `request peer <sel> borr` |
| `peer <sel> eorr` | `request peer <sel> eorr` |
| `peer <sel> clear soft` | `request peer <sel> clear soft` |

### 4. Clean up HasCommandPrefix

After the split, `HasCommandPrefix` in `update-route` becomes unnecessary.
The prepend is unconditional for route commands. The check can be removed
or simplified.

## Impact

- **ExaBGP compat**: ExaBGP text protocol uses `update-route` for both routes
  and actions. The bridge layer (`internal/component/bgp/cli/cmd_plugin.go` or
  similar) must route actions to `dispatch-command` instead.
- **Python SDK**: `ze_api.py` has `_send_update_route` which handles peer actions.
  After the split, peer actions should use `dispatch-command`.
- **Tests**: `.ci` tests that send peer actions via `update-route` must switch
  to `dispatch-command`.

## Files

- `internal/component/plugin/server/dispatch.go` (handleUpdateRouteRPC, HasCommandPrefix usage)
- `internal/component/plugin/server/command.go` (HasCommandPrefix)
- `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` (peer action containers)
- `internal/component/bgp/plugins/cmd/refresh/schema/ze-refresh-cmd.yang` (refresh containers)
- `test/scripts/ze_api.py` (_send_update_route)
- ExaBGP bridge code
- `.ci` tests using peer actions
